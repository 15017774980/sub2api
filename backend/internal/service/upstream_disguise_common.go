package service

import (
	"bytes"
	"strings"

	"github.com/gin-gonic/gin"
)

// MaybeDisguiseAnthropicPassthroughSSELine 是 anthropic-passthrough 路径所有平台 SSE 行改写的统一入口。
// 多个平台的 Maybe* 函数顺序调用——每个函数在没有自己 context 时 no-op，叠加调用安全。
// 加新平台时在这里追加一行即可，不需要动 gateway_service.go 的调用点。
func MaybeDisguiseAnthropicPassthroughSSELine(c *gin.Context, line []byte) []byte {
	line = MaybeDisguiseStreamLine(c, line)     // DeepSeek
	line = MaybeDisguiseKimiStreamLine(c, line) // Kimi
	line = MaybeDisguiseMiMoStreamLine(c, line) // MiMo
	return line
}

// MaybeDisguiseAnthropicPassthroughJSONResponse 同上，针对非流式 JSON 响应体。
func MaybeDisguiseAnthropicPassthroughJSONResponse(c *gin.Context, body []byte) []byte {
	body = MaybeDisguiseJSONResponse(c, body)     // DeepSeek
	body = MaybeDisguiseKimiJSONResponse(c, body) // Kimi
	body = MaybeDisguiseMiMoJSONResponse(c, body) // MiMo
	return body
}

// MaybeDisguiseAnthropicPassthroughErrorBody 同上，针对透传的上游错误体。
func MaybeDisguiseAnthropicPassthroughErrorBody(c *gin.Context, body []byte) []byte {
	body = MaybeDisguiseErrorBody(c, body)     // DeepSeek
	body = MaybeDisguiseKimiErrorBody(c, body) // Kimi
	body = MaybeDisguiseMiMoErrorBody(c, body) // MiMo
	return body
}

// 本文件提供多上游伪装中间件共用的 building blocks。
// 各平台 disguise 文件（deepseek_disguise.go / openai_disguise.go / kimi_disguise.go / ...）
// 注册自己的 leak 关键词列表，调用通用函数做检测。
//
// 通用 OpaqueErrorBody / synthesizeAnthropicMessageID / cloneBytes / isAdminAPIKeyContext
// 仍在 deepseek_disguise.go 内（service 包内可直接复用，无需重新导出）。

// commonOpaqueErrorBody 与 anthropic 官方错误格式对齐，所有平台 leak 命中后统一替换为该 payload。
// 与 deepseek_disguise.go 的 disguiseOpaqueErrorBody / openai_disguise.go 的 disguiseOpaqueErrorBodyOpenAI
// 内容一致，未来可逐步收敛指向同一变量。
var commonOpaqueErrorBody = []byte(`{"type":"error","error":{"type":"api_error","message":"Upstream channel returned an error"}}`)

// messageContainsAnyKeyword 检测一个 string-form 错误 message 是否含给定关键词列表中的任一项（不区分大小写）。
// keywords 必须为小写。
func messageContainsAnyKeyword(msg string, keywords []string) bool {
	if msg == "" || len(keywords) == 0 {
		return false
	}
	lower := strings.ToLower(msg)
	for _, w := range keywords {
		if w == "" {
			continue
		}
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// bodyContainsAnyKeyword 检测 body bytes 是否含给定关键词列表中的任一项（不区分大小写）。
// keywords 必须为小写。
func bodyContainsAnyKeyword(body []byte, keywords []string) bool {
	if len(body) == 0 || len(keywords) == 0 {
		return false
	}
	lower := bytes.ToLower(body)
	for _, w := range keywords {
		if w == "" {
			continue
		}
		if bytes.Contains(lower, []byte(w)) {
			return true
		}
	}
	return false
}
