package service

import (
	"bytes"
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 本文件实现 Kimi (Moonshot) 上游响应的伪装中间件，把流式 / 非流式 / 错误响应中
// 暴露上游身份的字段在出口处改写为对外协议族（Anthropic）口径，
// 避免泄露真实上游给终端用户。
//
// 走 anthropic-passthrough 路径（同协议，与 DeepSeek 完全对称）。
//
// 关键不变量：
//  1. content_block_* 内的 thinking/text/tool_use 严禁触碰
//  2. 仅改写顶层 model 与 message_start.message.id
//  3. 错误体白名单重写：检出泄露字串即整体替换为不透明 api_error
//  4. admin 角色全程透传不改写
//  5. 仅 Kimi 平台触发：调用侧必须先校验 account.Platform == PlatformKimi

const kimiDisguiseContextKey = "kimi_disguise"

// kimiLeakKeywords 是 Kimi 路径错误体里需要拦截的暴露词。
//   - "kimi" / "moonshot"            — 品牌字串
//   - "k2-" / "k2." / "kimi-"        — 模型前缀
var kimiLeakKeywords = []string{"kimi", "moonshot"}

type kimiDisguiseContext struct {
	OriginalModel      string
	SyntheticMessageID string
}

// SetKimiDisguiseContext 在 gin.Context 上记录伪装状态。
// 仅在 account 为 Kimi 平台、且当前 APIKey 关联 user 非 admin 时生效。
func SetKimiDisguiseContext(c *gin.Context, account *Account, originalModel string) {
	if c == nil || account == nil {
		return
	}
	if account.Platform != PlatformKimi {
		return
	}
	if isAdminAPIKeyContext(c) {
		return
	}
	syntheticID := synthesizeAnthropicMessageID()
	c.Set(kimiDisguiseContextKey, &kimiDisguiseContext{
		OriginalModel:      originalModel,
		SyntheticMessageID: syntheticID,
	})

	if c.Request != nil {
		ctx := context.WithValue(c.Request.Context(), ctxkey.DisguisedMessageID, syntheticID)
		c.Request = c.Request.WithContext(ctx)
	}
}

func getKimiDisguiseContext(c *gin.Context) *kimiDisguiseContext {
	if c == nil {
		return nil
	}
	v, ok := c.Get(kimiDisguiseContextKey)
	if !ok {
		return nil
	}
	cfg, _ := v.(*kimiDisguiseContext)
	return cfg
}

// DisguiseKimiStreamLine 改写 Kimi 路径的一条 SSE 行（已是 anthropic 格式，因为是 anthropic-passthrough）。
//   - isAdmin=true 直接返回
//   - 仅对 message_start / error 事件改写
//   - content_block_* 严禁触碰
func DisguiseKimiStreamLine(line []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
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
		newPayload, modified = rewriteKimiMessageStart(payload, originalModel, syntheticMessageID)
	case "error":
		newPayload, modified = rewriteKimiErrorPayload(payload)
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

// DisguiseKimiJSONResponse 改写非流式 JSON 响应体（顶层 model + id；error 白名单）。
func DisguiseKimiJSONResponse(body []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
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
		if looksLikeKimiModel(modelField) && originalModel != "" {
			if next, err := sjson.SetBytes(out, "model", originalModel); err == nil {
				out = next
			}
		}
		if syntheticMessageID != "" {
			if next, err := sjson.SetBytes(out, "id", syntheticMessageID); err == nil {
				out = next
			}
		}
		return out
	case "error":
		if messageContainsAnyKeyword(parsed.Get("error.message").String(), kimiLeakKeywords) {
			return cloneBytes(commonOpaqueErrorBody)
		}
		return body
	default:
		return body
	}
}

// DisguiseKimiErrorBody 改写直接透传的上游错误体。
func DisguiseKimiErrorBody(body []byte, isAdmin bool) []byte {
	if isAdmin || len(body) == 0 {
		return body
	}
	if gjson.ValidBytes(body) {
		if messageContainsAnyKeyword(gjson.GetBytes(body, "error.message").String(), kimiLeakKeywords) {
			return cloneBytes(commonOpaqueErrorBody)
		}
	}
	if bodyContainsAnyKeyword(body, kimiLeakKeywords) {
		return cloneBytes(commonOpaqueErrorBody)
	}
	return body
}

// MaybeDisguiseKimiStreamLine 从 gin.Context 读伪装状态并按需改写。
func MaybeDisguiseKimiStreamLine(c *gin.Context, line []byte) []byte {
	cfg := getKimiDisguiseContext(c)
	if cfg == nil {
		return line
	}
	return DisguiseKimiStreamLine(line, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// MaybeDisguiseKimiJSONResponse 同上，针对非流式 JSON 响应体。
func MaybeDisguiseKimiJSONResponse(c *gin.Context, body []byte) []byte {
	cfg := getKimiDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseKimiJSONResponse(body, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// MaybeDisguiseKimiErrorBody 同上，针对透传的上游错误体。
func MaybeDisguiseKimiErrorBody(c *gin.Context, body []byte) []byte {
	cfg := getKimiDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseKimiErrorBody(body, false)
}

// KimiDisguisedSyntheticMessageID 返回当前请求的合成 message id（如启用了 Kimi 伪装）。
func KimiDisguisedSyntheticMessageID(c *gin.Context) string {
	cfg := getKimiDisguiseContext(c)
	if cfg == nil {
		return ""
	}
	return cfg.SyntheticMessageID
}

func rewriteKimiMessageStart(payload []byte, originalModel, syntheticMessageID string) ([]byte, bool) {
	out := payload
	modified := false

	if originalModel != "" {
		model := gjson.GetBytes(out, "message.model").String()
		if looksLikeKimiModel(model) {
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

func rewriteKimiErrorPayload(payload []byte) ([]byte, bool) {
	if messageContainsAnyKeyword(gjson.GetBytes(payload, "error.message").String(), kimiLeakKeywords) {
		return cloneBytes(commonOpaqueErrorBody), true
	}
	return payload, false
}

func looksLikeKimiModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.HasPrefix(lower, "kimi") || strings.HasPrefix(lower, "moonshot")
}
