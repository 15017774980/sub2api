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

// 本文件实现 Xiaomi MiMo 上游响应的伪装中间件。走 anthropic-passthrough 路径。
// 关键不变量与 deepseek/kimi 完全一致。

const mimoDisguiseContextKey = "mimo_disguise"

// mimoLeakKeywords 是 MiMo 路径错误体里需要拦截的暴露词。
//   - "mimo" / "xiaomi"            — 品牌字串
//   - "v2-pro" / "v2-omni"         — 模型变体
var mimoLeakKeywords = []string{"mimo", "xiaomi"}

type mimoDisguiseContext struct {
	OriginalModel      string
	SyntheticMessageID string
}

// SetMiMoDisguiseContext 在 gin.Context 上记录伪装状态。
func SetMiMoDisguiseContext(c *gin.Context, account *Account, originalModel string) {
	if c == nil || account == nil {
		return
	}
	if account.Platform != PlatformMiMo {
		return
	}
	if isAdminAPIKeyContext(c) {
		return
	}
	syntheticID := synthesizeAnthropicMessageID()
	c.Set(mimoDisguiseContextKey, &mimoDisguiseContext{
		OriginalModel:      originalModel,
		SyntheticMessageID: syntheticID,
	})

	if c.Request != nil {
		ctx := context.WithValue(c.Request.Context(), ctxkey.DisguisedMessageID, syntheticID)
		c.Request = c.Request.WithContext(ctx)
	}
}

func getMiMoDisguiseContext(c *gin.Context) *mimoDisguiseContext {
	if c == nil {
		return nil
	}
	v, ok := c.Get(mimoDisguiseContextKey)
	if !ok {
		return nil
	}
	cfg, _ := v.(*mimoDisguiseContext)
	return cfg
}

// DisguiseMiMoStreamLine 改写 MiMo 路径的一条 SSE 行（已是 anthropic 格式）。
func DisguiseMiMoStreamLine(line []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
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
		newPayload, modified = rewriteMiMoMessageStart(payload, originalModel, syntheticMessageID)
	case "error":
		newPayload, modified = rewriteMiMoErrorPayload(payload)
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

// DisguiseMiMoJSONResponse 改写非流式 JSON 响应体（顶层 model + id；error 白名单）。
func DisguiseMiMoJSONResponse(body []byte, originalModel, syntheticMessageID string, isAdmin bool) []byte {
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
		if looksLikeMiMoModel(modelField) && originalModel != "" {
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
		if messageContainsAnyKeyword(parsed.Get("error.message").String(), mimoLeakKeywords) {
			return cloneBytes(commonOpaqueErrorBody)
		}
		return body
	default:
		return body
	}
}

// DisguiseMiMoErrorBody 改写直接透传的上游错误体。
func DisguiseMiMoErrorBody(body []byte, isAdmin bool) []byte {
	if isAdmin || len(body) == 0 {
		return body
	}
	if gjson.ValidBytes(body) {
		if messageContainsAnyKeyword(gjson.GetBytes(body, "error.message").String(), mimoLeakKeywords) {
			return cloneBytes(commonOpaqueErrorBody)
		}
	}
	if bodyContainsAnyKeyword(body, mimoLeakKeywords) {
		return cloneBytes(commonOpaqueErrorBody)
	}
	return body
}

// MaybeDisguiseMiMoStreamLine 从 gin.Context 读伪装状态并按需改写。
func MaybeDisguiseMiMoStreamLine(c *gin.Context, line []byte) []byte {
	cfg := getMiMoDisguiseContext(c)
	if cfg == nil {
		return line
	}
	return DisguiseMiMoStreamLine(line, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// MaybeDisguiseMiMoJSONResponse 同上，针对非流式 JSON 响应体。
func MaybeDisguiseMiMoJSONResponse(c *gin.Context, body []byte) []byte {
	cfg := getMiMoDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseMiMoJSONResponse(body, cfg.OriginalModel, cfg.SyntheticMessageID, false)
}

// MaybeDisguiseMiMoErrorBody 同上，针对透传的上游错误体。
func MaybeDisguiseMiMoErrorBody(c *gin.Context, body []byte) []byte {
	cfg := getMiMoDisguiseContext(c)
	if cfg == nil {
		return body
	}
	return DisguiseMiMoErrorBody(body, false)
}

// MiMoDisguisedSyntheticMessageID 返回当前请求的合成 message id。
func MiMoDisguisedSyntheticMessageID(c *gin.Context) string {
	cfg := getMiMoDisguiseContext(c)
	if cfg == nil {
		return ""
	}
	return cfg.SyntheticMessageID
}

func rewriteMiMoMessageStart(payload []byte, originalModel, syntheticMessageID string) ([]byte, bool) {
	out := payload
	modified := false

	if originalModel != "" {
		model := gjson.GetBytes(out, "message.model").String()
		if looksLikeMiMoModel(model) {
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

func rewriteMiMoErrorPayload(payload []byte) ([]byte, bool) {
	if messageContainsAnyKeyword(gjson.GetBytes(payload, "error.message").String(), mimoLeakKeywords) {
		return cloneBytes(commonOpaqueErrorBody), true
	}
	return payload, false
}

func looksLikeMiMoModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.HasPrefix(lower, "mimo")
}

// mimoModelSupportsImage 判断 upstream MiMo 模型是否原生支持 image input。
// 当前仅 *-omni 系列支持（mimo-v2-omni 等）；其它（mimo-v2-pro / mimo-v2-flash）text+code only。
func mimoModelSupportsImage(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "omni")
}
