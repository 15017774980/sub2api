package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 本文件实现 DeepSeek 上游响应的伪装中间件，把流式 / 非流式 / 错误响应中
// 暴露上游身份的字段在出口处改写为对外协议族（Anthropic）口径，
// 避免泄露真实上游给终端用户。
//
// 关键不变量（违反即破坏 DeepSeek 多轮对话）：
//  1. content_block_start/delta/stop 内的 thinking/text/tool_use 结构 严禁触碰
//     — DeepSeek anthropic 兼容 endpoint 要求 thinking_block 原样回传，破坏即 400
//  2. 仅改写顶层 model 字段（message.model 或 message_start.message.model）
//  3. 错误体使用白名单重写：检出泄露字串即整体替换为不透明 api_error，不保留 raw message
//  4. admin 角色全程透传不改写（由调用侧通过 isAdmin 参数 / context 控制）
//  5. 仅 deepseek 平台触发：调用侧必须先校验 account.Platform == PlatformDeepSeek

const deepseekDisguiseContextKey = "deepseek_disguise"

// disguiseOpaqueErrorBody 是错误体白名单重写后的固定 payload，
// 与 anthropic 官方错误格式对齐，不保留任何上游真实错误信息。
var disguiseOpaqueErrorBody = []byte(`{"type":"error","error":{"type":"api_error","message":"Upstream channel returned an error"}}`)

// deepseekDisguiseContext 存活于单次请求 gin.Context 中。
// 仅当 account.Platform==PlatformDeepSeek 且非 admin 时由调用侧 set。
//
// SyntheticMessageID 在 SetDeepSeekDisguiseContext 时一次性生成，整次请求共享：
// 用于改写非流式 message.id 和 SSE message_start.message.id，对外稳定为 anthropic 风格
// (msg_ + 24 hex)。usage_log 的 request_id 也会复用这个值 (见 resolveUsageBillingRequestID)
// 保证响应里的 id 和 user 看 usage 历史时的 id 自洽。
type deepseekDisguiseContext struct {
	OriginalModel      string
	SyntheticMessageID string
}

// SetDeepSeekDisguiseContext 在 gin.Context 上记录伪装状态。
// 仅在 account 为 deepseek 平台、且当前 APIKey 关联 user 非 admin 时生效。
// 其他场景静默 no-op，保持透传。
func SetDeepSeekDisguiseContext(c *gin.Context, account *Account, originalModel string) {
	if c == nil || account == nil {
		return
	}
	if account.Platform != PlatformDeepSeek {
		return
	}
	if isAdminAPIKeyContext(c) {
		return
	}
	syntheticID := synthesizeAnthropicMessageID()
	c.Set(deepseekDisguiseContextKey, &deepseekDisguiseContext{
		OriginalModel:      originalModel,
		SyntheticMessageID: syntheticID,
	})

	// 同时把合成 id 注入 c.Request.Context()，让下游 billing 链路（resolveUsageBillingRequestID）
	// 优先用它写 usage_log.request_id，确保响应里的 id 与 user 看到的 usage 记录自洽。
	if c.Request != nil {
		ctx := context.WithValue(c.Request.Context(), ctxkey.DisguisedMessageID, syntheticID)
		c.Request = c.Request.WithContext(ctx)
	}
}

// synthesizeAnthropicMessageID 生成 "msg_" + 24 hex 字符的 id，
// 模仿 Anthropic 官方 message id 格式（msg_01XXX...）。
// 使用 crypto/rand；极小概率失败时退化为伪随机但仍合法的 id。
func synthesizeAnthropicMessageID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// fail-safe：crypto/rand 在现代系统几乎不会失败；用 hex 编码 0 兜底也不会让 id 变得异常
		return "msg_000000000000000000000000"
	}
	return "msg_" + hex.EncodeToString(raw[:])
}

func getDeepSeekDisguiseContext(c *gin.Context) *deepseekDisguiseContext {
	if c == nil {
		return nil
	}
	v, ok := c.Get(deepseekDisguiseContextKey)
	if !ok {
		return nil
	}
	cfg, _ := v.(*deepseekDisguiseContext)
	return cfg
}

func isAdminAPIKeyContext(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get("api_key")
	if !ok {
		return false
	}
	apiKey, _ := v.(*APIKey)
	return apiKey != nil && apiKey.User != nil && apiKey.User.IsAdmin()
}

// DisguiseStreamLine 改写一条 SSE 行。
//   - isAdmin=true 直接返回原 line（admin 透传）
//   - 仅对 data: {...} 行的 message_start / error 事件做改写，其它事件类型严格 passthrough
//   - 严禁触碰 content_block_* 事件
//   - syntheticMessageID 非空时，message_start.message.id 改写为该值（与非流式 id 一致）
func DisguiseStreamLine(line []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
	if isAdmin || len(line) == 0 {
		return line
	}
	if !bytes.HasPrefix(line, []byte("data:")) {
		return line
	}
	payload := bytes.TrimLeft(line[len("data:"):], " \t")
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return line
	}
	if payload[0] != '{' {
		return line
	}
	if !gjson.ValidBytes(payload) {
		return line
	}

	eventType := gjson.GetBytes(payload, "type").String()
	var (
		newPayload []byte
		modified   bool
	)
	switch eventType {
	case "message_start":
		newPayload, modified = rewriteMessageStart(payload, originalModel, syntheticMessageID)
	case "error":
		newPayload, modified = rewriteErrorPayload(payload)
	default:
		return line
	}
	if !modified {
		return line
	}

	out := make([]byte, 0, len("data: ")+len(newPayload))
	out = append(out, "data: "...)
	out = append(out, newPayload...)
	return out
}

// DisguiseJSONResponse 改写非流式 JSON 响应体。
// 处理两类 payload：
//   - {"type":"message","id":"<UUID>","model":"deepseek-...","content":[...]}
//     → 改写顶层 model 为 originalModel，id 为 syntheticMessageID
//   - {"type":"error","error":{...}}
//     → 检出泄露则白名单重写
func DisguiseJSONResponse(body []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
	if isAdmin || len(body) == 0 {
		return body
	}
	if !gjson.ValidBytes(body) {
		return body
	}
	parsed := gjson.ParseBytes(body)
	switch parsed.Get("type").String() {
	case "message":
		out := body
		modelField := parsed.Get("model").String()
		if looksLikeDeepSeekModel(modelField) && originalModel != "" {
			if next, err := sjson.SetBytes(out, "model", originalModel); err == nil {
				out = next
			}
		}
		// id 字段与 model 是否泄露无关，无脑覆盖：deepseek 的 id 永远是 UUID 格式，
		// 不可能与 anthropic 的 msg_xxx 格式重合，且 syntheticMessageID 由 disguise context 唯一持有。
		if syntheticMessageID != "" {
			if next, err := sjson.SetBytes(out, "id", syntheticMessageID); err == nil {
				out = next
			}
		}
		return out
	case "error":
		if errorBodyContainsLeak(parsed.Get("error.message").String()) {
			return cloneBytes(disguiseOpaqueErrorBody)
		}
		return body
	default:
		return body
	}
}

// DisguiseErrorBody 改写直接透传的上游错误体（如 handleErrorResponse 400 透传分支）。
// 既扫 error.message 也扫整体 body，命中任一即整体白名单重写。
// 非 JSON 输入也兼容处理。
func DisguiseErrorBody(body []byte, isAdmin bool) []byte {
	if isAdmin || len(body) == 0 {
		return body
	}
	if gjson.ValidBytes(body) {
		if errorBodyContainsLeak(gjson.GetBytes(body, "error.message").String()) {
			return cloneBytes(disguiseOpaqueErrorBody)
		}
	}
	if rawBodyContainsLeak(body) {
		return cloneBytes(disguiseOpaqueErrorBody)
	}
	return body
}

// MaybeDisguiseStreamLine 从 gin.Context 读取伪装状态并按需改写。
// 不在 deepseek 伪装会话中时直接返回 line。
func MaybeDisguiseStreamLine(c *gin.Context, line []byte) []byte {
	cfg := getDeepSeekDisguiseContext(c)
	if cfg == nil {
		return line
	}
	return DisguiseStreamLine(line, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// MaybeDisguiseJSONResponse 同上，针对非流式 JSON 响应体。
func MaybeDisguiseJSONResponse(c *gin.Context, body []byte) []byte {
	cfg := getDeepSeekDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseJSONResponse(body, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// DisguisedSyntheticMessageID 返回当前请求的合成 message id（如启用了 deepseek 伪装），
// 否则返回空。供 usage 计费链路写入 request_id，保证响应里的 id 与 usage_log 自洽。
func DisguisedSyntheticMessageID(c *gin.Context) string {
	cfg := getDeepSeekDisguiseContext(c)
	if cfg == nil {
		return ""
	}
	return cfg.SyntheticMessageID
}

// MaybeDisguiseErrorBody 同上，针对透传的上游错误体。
func MaybeDisguiseErrorBody(c *gin.Context, body []byte) []byte {
	cfg := getDeepSeekDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseErrorBody(body, false)
}

// rewriteMessageStart 改写 SSE 的 message_start 事件 payload。
//   - 若 message.model 是 deepseek 风格且 originalModel 非空：改写 message.model
//   - 若 syntheticMessageID 非空：改写 message.id
//
// 任一改写命中即返回 modified=true。
func rewriteMessageStart(payload []byte, originalModel, syntheticMessageID string) ([]byte, bool) {
	out := payload
	modified := false

	if originalModel != "" {
		model := gjson.GetBytes(out, "message.model").String()
		if looksLikeDeepSeekModel(model) {
			if next, err := sjson.SetBytes(out, "message.model", originalModel); err == nil {
				out = next
				modified = true
			}
		}
	}
	if syntheticMessageID != "" {
		if next, err := sjson.SetBytes(out, "message.id", syntheticMessageID); err == nil {
			out = next
			modified = true
		}
	}
	return out, modified
}

func rewriteErrorPayload(payload []byte) ([]byte, bool) {
	if errorBodyContainsLeak(gjson.GetBytes(payload, "error.message").String()) {
		return cloneBytes(disguiseOpaqueErrorBody), true
	}
	return payload, false
}

func looksLikeDeepSeekModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "deepseek")
}

// deepseekLeakKeywords 是 DeepSeek 路径错误体里需要拦截的暴露词。
//   - "deepseek"        实测命中（tools/deepseek_verify case 01 错误体含
//                       "The supported API model names are deepseek-v4-pro or deepseek-v4-flash..."）
//   - "unknown variant" 防御性兜底，对接其它 anthropic-兼容上游可能用到的常见错误模式，
//                       当前 DeepSeek 实测不命中，保留无害
var deepseekLeakKeywords = []string{"deepseek", "unknown variant"}

func errorBodyContainsLeak(msg string) bool {
	return messageContainsAnyKeyword(msg, deepseekLeakKeywords)
}

func rawBodyContainsLeak(body []byte) bool {
	return bodyContainsAnyKeyword(body, deepseekLeakKeywords)
}

func cloneBytes(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
