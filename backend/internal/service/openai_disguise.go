package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 本文件实现 OpenAI 上游响应在 ForwardAsAnthropic（Anthropic 协议入站 → OpenAI 上游）
// 路径上的伪装中间件。把响应中暴露 OpenAI 身份的字段在出口处改写为 Anthropic 口径，
// 避免泄露真实上游给终端用户。
//
// 关键不变量（违反即破坏 GPT 多轮对话或泄露身份）：
//  1. content_block_start/delta/stop 内的 thinking/text/tool_use 结构 严禁触碰
//     — apicompat 协议转换层已将 reasoning/output_text/function_call 转成 anthropic 结构，
//     这些块由协议转换层产生，本中间件不再触碰
//  2. 仅改写顶层 message.id（非流式）和 message_start.message.id（流式）
//  3. 错误体使用白名单重写：检出泄露字串即整体替换为不透明 api_error，不保留 raw message
//  4. admin 角色全程透传不改写（由调用侧通过 isAdmin 参数 / context 控制）
//  5. 仅 OpenAI 平台触发：调用侧必须先校验 account.Platform == PlatformOpenAI

const openaiDisguiseContextKey = "openai_disguise"

// disguiseOpaqueErrorBodyOpenAI 是 OpenAI 路径错误体白名单重写后的固定 payload，
// 与 anthropic 官方错误格式对齐，不保留任何上游真实错误信息。
var disguiseOpaqueErrorBodyOpenAI = []byte(`{"type":"error","error":{"type":"api_error","message":"Upstream channel returned an error"}}`)

// openaiDisguiseContext 存活于单次请求 gin.Context 中。
// 仅当 account.Platform==PlatformOpenAI 且非 admin 时由调用侧 set。
//
// SyntheticMessageID 在 SetOpenAIDisguiseContext 时一次性生成，整次请求共享：
// 用于改写非流式 message.id 和 SSE message_start.message.id，对外稳定为 anthropic 风格
// (msg_ + 24 hex)。usage_log 的 request_id 也会复用这个值（见 resolveUsageBillingRequestID）
// 保证响应里的 id 和 user 看 usage 历史时的 id 自洽。
type openaiDisguiseContext struct {
	OriginalModel      string
	SyntheticMessageID string
}

// SetOpenAIDisguiseContext 在 gin.Context 上记录伪装状态。
// 仅在 account 为 OpenAI 平台、且当前 APIKey 关联 user 非 admin 时生效。
// 其他场景静默 no-op，保持透传。
func SetOpenAIDisguiseContext(c *gin.Context, account *Account, originalModel string) {
	if c == nil || account == nil {
		return
	}
	if account.Platform != PlatformOpenAI {
		return
	}
	if isAdminAPIKeyContext(c) {
		return
	}
	syntheticID := synthesizeAnthropicMessageID()
	c.Set(openaiDisguiseContextKey, &openaiDisguiseContext{
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

func getOpenAIDisguiseContext(c *gin.Context) *openaiDisguiseContext {
	if c == nil {
		return nil
	}
	v, ok := c.Get(openaiDisguiseContextKey)
	if !ok {
		return nil
	}
	cfg, _ := v.(*openaiDisguiseContext)
	return cfg
}

// OpenAIDisguisedSyntheticMessageID 返回当前请求的合成 message id（如启用了 OpenAI 伪装），
// 否则返回空。
func OpenAIDisguisedSyntheticMessageID(c *gin.Context) string {
	cfg := getOpenAIDisguiseContext(c)
	if cfg == nil {
		return ""
	}
	return cfg.SyntheticMessageID
}

// MaybeInjectClaudeIdentityForOpenAI 在 ForwardAsAnthropic 入口按需注入 Claude 身份提示。
//
// 只在入站 model 看起来像 claude-*（用户明确想要 Claude 行为，但实际打到 OpenAI 上游）
// 时才注入；对 gpt-*/o*-* 等 OpenAI 协议直发场景透传——那些请求用户显式想要 GPT
// 行为，注入"You are Claude" 反而会扰乱 instructions 模板和模型自我认知。
func MaybeInjectClaudeIdentityForOpenAI(body []byte) []byte {
	if !looksLikeClaudeRequestedModel(gjson.GetBytes(body, "model").String()) {
		return body
	}
	return InjectClaudeIdentitySystem(body)
}

func looksLikeClaudeRequestedModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "claude")
}

// MaybeRewriteAnthropicResponseID 改写非流式 anthropic 响应对象的 id 字段。
// 不在 OpenAI 伪装会话中时直接返回。
//
// 适用场景：handleAnthropicBufferedStreamingResponse 在 ResponsesToAnthropic 转换之后，
// 在 c.JSON() 输出之前调用。
func MaybeRewriteAnthropicResponseID(c *gin.Context, resp *apicompat.AnthropicResponse) {
	if resp == nil {
		return
	}
	cfg := getOpenAIDisguiseContext(c)
	if cfg == nil || cfg.SyntheticMessageID == "" {
		return
	}
	resp.ID = cfg.SyntheticMessageID
}

// DisguiseAnthropicSSELineForOpenAI 改写 OpenAI 路径的 SSE 行（已转成 anthropic 格式）。
//   - isAdmin=true 直接返回原 line
//   - 仅对 message_start 事件改 message.id
//   - 严禁触碰 content_block_* 事件
func DisguiseAnthropicSSELineForOpenAI(line string, syntheticMessageID string, isAdmin bool) string {
	if isAdmin || syntheticMessageID == "" || len(line) == 0 {
		return line
	}
	// SSE 帧可能跨多行（event: + data:），逐 data 行处理。其它行原样保留。
	if !strings.Contains(line, "data:") {
		return line
	}

	// 简化策略：仅当行为单一的 "data: {json}" 形态时尝试改写；
	// 复合帧（event: msg\ndata: {...}\n\n）由调用侧切分为单 data 行处理。
	dataIdx := strings.Index(line, "data:")
	prefix := line[:dataIdx+len("data:")]
	rest := strings.TrimLeft(line[dataIdx+len("data:"):], " \t")
	if len(rest) == 0 || strings.HasPrefix(rest, "[DONE]") || rest[0] != '{' {
		return line
	}
	// rest 可能含尾部 \n\n 或其它 SSE 终止符，先剥离
	jsonEnd := strings.Index(rest, "\n")
	jsonPart := rest
	tail := ""
	if jsonEnd >= 0 {
		jsonPart = rest[:jsonEnd]
		tail = rest[jsonEnd:]
	}

	if !gjson.Valid(jsonPart) {
		return line
	}
	if gjson.Get(jsonPart, "type").String() != "message_start" {
		return line
	}
	rewritten, err := sjson.Set(jsonPart, "message.id", syntheticMessageID)
	if err != nil {
		return line
	}
	return prefix + " " + rewritten + tail
}

// MaybeDisguiseAnthropicSSELineForOpenAI 从 gin.Context 读伪装状态并按需改写 SSE 行。
func MaybeDisguiseAnthropicSSELineForOpenAI(c *gin.Context, line string) string {
	cfg := getOpenAIDisguiseContext(c)
	if cfg == nil {
		return line
	}
	return DisguiseAnthropicSSELineForOpenAI(line, cfg.SyntheticMessageID, false)
}

// DisguiseOpenAIErrorBody 检测错误体里是否含暴露 OpenAI 身份的关键词，命中即整体白名单重写。
// 既扫 error.message 也扫整体 body，命中任一即整体白名单重写。
//
// 关键词来源：
//   - "openai" / "chatgpt" / "codex"     OpenAI 错误体常见品牌字串
//   - "chatcmpl" / "resp_"               OpenAI 响应 id 前缀，错误体里若提到也是泄露
//   - "fp_"                              OpenAI system_fingerprint 前缀
//   - "gpt-"                             模型名前缀（注意：anthropic 错误也可能含 gpt 字面，
//                                       匹配但触发概率低；通过 isOpenAILeak 的多项 OR 提高准确度）
func DisguiseOpenAIErrorBody(body []byte, isAdmin bool) []byte {
	if isAdmin || len(body) == 0 {
		return body
	}
	if gjson.ValidBytes(body) {
		if openaiErrorBodyContainsLeak(gjson.GetBytes(body, "error.message").String()) {
			return cloneBytes(disguiseOpaqueErrorBodyOpenAI)
		}
	}
	if openaiRawBodyContainsLeak(body) {
		return cloneBytes(disguiseOpaqueErrorBodyOpenAI)
	}
	return body
}

// MaybeDisguiseOpenAIErrorBody 同上，从 gin.Context 读取 admin 状态。
func MaybeDisguiseOpenAIErrorBody(c *gin.Context, body []byte) []byte {
	cfg := getOpenAIDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseOpenAIErrorBody(body, false)
}

// MaybeDisguiseOpenAIErrorMessage 检查 string 形式的 upstream error message。
// 命中 leak 即整体替换为通用文案，不破坏现有错误流程的 status code / errType。
//
// 适用：handleCompatErrorResponse 在调用 writeError 之前，对 upstreamMsg 做一遍清洗。
// 仅在 OpenAI disguise context 启用时生效（ForwardAsAnthropic 路径），
// chat completions 等纯 OpenAI 协议路径无 context 即 no-op。
func MaybeDisguiseOpenAIErrorMessage(c *gin.Context, msg string) string {
	cfg := getOpenAIDisguiseContext(c)
	if cfg == nil {
		return msg
	}
	if openaiErrorBodyContainsLeak(msg) {
		return "Upstream channel returned an error"
	}
	return msg
}

// openaiLeakKeywords 是 OpenAI 路径错误体里需要拦截的暴露词。
//   - "openai" / "chatgpt" / "codex" — OpenAI 错误体常见品牌字串
//   - "chatcmpl" / "resp_"           — OpenAI 响应 id 前缀，错误体里若提到也是泄露
//   - "fp_"                          — OpenAI system_fingerprint 前缀
var openaiLeakKeywords = []string{"openai", "chatgpt", "codex", "chatcmpl", "resp_", "fp_"}

func openaiErrorBodyContainsLeak(msg string) bool {
	return messageContainsAnyKeyword(msg, openaiLeakKeywords)
}

func openaiRawBodyContainsLeak(body []byte) bool {
	return bodyContainsAnyKeyword(body, openaiLeakKeywords)
}
