//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------- DisguiseStreamLine ----------

func TestDisguiseStreamLine_AdminPassthrough(t *testing.T) {
	line := []byte(`data: {"type":"message_start","message":{"id":"msg_x","type":"message","role":"assistant","model":"deepseek-v3.2","content":[],"usage":{"input_tokens":1}}}`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", true)
	require.Equal(t, string(line), string(got), "admin must passthrough verbatim")
}

func TestDisguiseStreamLine_RewritesMessageStartModel(t *testing.T) {
	line := []byte(`data: {"type":"message_start","message":{"id":"msg_x","type":"message","role":"assistant","model":"deepseek-v3.2","content":[],"usage":{"input_tokens":1}}}`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
	require.Contains(t, string(got), `"model":"claude-sonnet-4-5"`)
	require.NotContains(t, strings.ToLower(string(got)), "deepseek")
	require.True(t, strings.HasPrefix(string(got), "data: "))
}

func TestDisguiseStreamLine_DoesNotTouchContentBlocks(t *testing.T) {
	// 严守不变量：content_block_* 事件 NEVER 改写（thinking 块原样回传是 DeepSeek 多轮硬要求）
	cases := [][]byte{
		[]byte(`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`),
		[]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"思考中..."}}`),
		[]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`),
		[]byte(`data: {"type":"content_block_stop","index":0}`),
		[]byte(`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"fn","input":{}}}`),
		[]byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`),
		[]byte(`data: {"type":"message_stop"}`),
		[]byte(`data: {"type":"ping"}`),
	}
	for _, line := range cases {
		got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
		require.Equal(t, string(line), string(got), "line %q must passthrough", line)
	}
}

func TestDisguiseStreamLine_ErrorWhitelistRewrite(t *testing.T) {
	cases := [][]byte{
		[]byte(`data: {"type":"error","error":{"type":"invalid_request_error","message":"unknown variant ` + "`image`" + `, expected one of ..."}}`),
		[]byte(`data: {"type":"error","error":{"type":"api_error","message":"deepseek upstream returned 500"}}`),
		[]byte(`data: {"type":"error","error":{"type":"upstream","message":"DeepSeek 内部错误"}}`),
	}
	for _, line := range cases {
		got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
		require.Contains(t, string(got), `"type":"api_error"`)
		require.Contains(t, string(got), "Upstream channel returned an error")
		require.NotContains(t, strings.ToLower(string(got)), "deepseek")
		require.NotContains(t, strings.ToLower(string(got)), "unknown variant")
	}
}

func TestDisguiseStreamLine_ErrorBenignPassthrough(t *testing.T) {
	// 错误体不含泄露字串：保持原样（避免误改其它平台的合法错误）
	line := []byte(`data: {"type":"error","error":{"type":"rate_limit_error","message":"please retry"}}`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
	require.Equal(t, string(line), string(got))
}

func TestDisguiseStreamLine_NonDataLines(t *testing.T) {
	// event: / 空行 / [DONE] 等非 payload 行直接 passthrough
	cases := [][]byte{
		[]byte(`event: message_start`),
		[]byte(``),
		[]byte(`data: [DONE]`),
		[]byte(`: comment`),
	}
	for _, line := range cases {
		got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
		require.Equal(t, string(line), string(got))
	}
}

func TestDisguiseStreamLine_InvalidJSONPassthrough(t *testing.T) {
	line := []byte(`data: {not valid json`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
	require.Equal(t, string(line), string(got))
}

func TestDisguiseStreamLine_OriginalModelEmpty_NoRewrite(t *testing.T) {
	line := []byte(`data: {"type":"message_start","message":{"model":"deepseek-v3.2"}}`)
	got := DisguiseStreamLine(line, "", "", false)
	require.Equal(t, string(line), string(got))
}

// ---------- DisguiseJSONResponse ----------

func TestDisguiseJSONResponse_AdminPassthrough(t *testing.T) {
	body := []byte(`{"type":"message","model":"deepseek-v3.2","content":[]}`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "", true)
	require.Equal(t, string(body), string(got))
}

func TestDisguiseJSONResponse_RewritesMessageModel(t *testing.T) {
	body := []byte(`{"id":"msg_x","type":"message","role":"assistant","model":"deepseek-v3.2","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":3,"output_tokens":2}}`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "", false)
	require.Contains(t, string(got), `"model":"claude-sonnet-4-5"`)
	require.NotContains(t, strings.ToLower(string(got)), "deepseek")
	// 关键不变量：content 数组结构应保持
	require.Contains(t, string(got), `"text":"hi"`)
}

func TestDisguiseJSONResponse_ErrorWhitelistRewrite(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages.0.content.0.type: unknown variant ` + "`image`" + `"}}`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "", false)
	require.Contains(t, string(got), `"type":"api_error"`)
	require.Contains(t, string(got), "Upstream channel returned an error")
	require.NotContains(t, strings.ToLower(string(got)), "unknown variant")
}

func TestDisguiseJSONResponse_NonDeepSeekModelNotChanged(t *testing.T) {
	body := []byte(`{"type":"message","model":"claude-sonnet-4-5","content":[]}`)
	got := DisguiseJSONResponse(body, "claude-haiku-4-5", "", false)
	require.Equal(t, string(body), string(got))
}

func TestDisguiseJSONResponse_InvalidJSONPassthrough(t *testing.T) {
	body := []byte(`not json`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "", false)
	require.Equal(t, string(body), string(got))
}

// ---------- DisguiseErrorBody ----------

func TestDisguiseErrorBody_DetectsLeakInErrorMessage(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"unknown variant ` + "`image`" + `, expected one of [text, tool_use, tool_result, thinking]"}}`)
	got := DisguiseErrorBody(body, false)
	require.Contains(t, string(got), `"type":"api_error"`)
	require.NotContains(t, strings.ToLower(string(got)), "unknown variant")
}

func TestDisguiseErrorBody_DetectsLeakInRawBody(t *testing.T) {
	// 即使 error.message 不直接泄露，但 body 其它字段含 deepseek 也要重写
	body := []byte(`{"upstream":"deepseek","error":{"message":"internal"}}`)
	got := DisguiseErrorBody(body, false)
	require.Contains(t, string(got), `"type":"api_error"`)
	require.NotContains(t, strings.ToLower(string(got)), "deepseek")
}

func TestDisguiseErrorBody_NonJSONWithLeak(t *testing.T) {
	body := []byte(`<html>deepseek-server: 500</html>`)
	got := DisguiseErrorBody(body, false)
	require.Contains(t, string(got), `"type":"api_error"`)
}

func TestDisguiseErrorBody_BenignPassthrough(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"please retry"}}`)
	got := DisguiseErrorBody(body, false)
	require.Equal(t, string(body), string(got))
}

func TestDisguiseErrorBody_AdminPassthrough(t *testing.T) {
	body := []byte(`{"error":{"message":"deepseek failed"}}`)
	got := DisguiseErrorBody(body, true)
	require.Equal(t, string(body), string(got))
}

// ---------- Maybe* helpers via gin.Context ----------

func TestMaybeDisguise_NoContext_Passthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	line := []byte(`data: {"type":"message_start","message":{"model":"deepseek-v3.2"}}`)
	require.Equal(t, string(line), string(MaybeDisguiseStreamLine(c, line)))

	body := []byte(`{"type":"message","model":"deepseek-v3.2"}`)
	require.Equal(t, string(body), string(MaybeDisguiseJSONResponse(c, body)))

	errBody := []byte(`{"type":"error","error":{"message":"deepseek down"}}`)
	require.Equal(t, string(errBody), string(MaybeDisguiseErrorBody(c, errBody)))
}

// ---------- L1: id 字段改写 ----------

func TestDisguiseStreamLine_RewritesMessageStartID(t *testing.T) {
	line := []byte(`data: {"type":"message_start","message":{"id":"c98f3cb9-73b4-431f-a690-d2183c3ffb10","type":"message","role":"assistant","model":"deepseek-v4-flash","content":[],"usage":{"input_tokens":1}}}`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "msg_aabbccddeeff00112233445566778899", false)
	require.Contains(t, string(got), `"id":"msg_aabbccddeeff00112233445566778899"`)
	require.Contains(t, string(got), `"model":"claude-sonnet-4-5"`)
	require.NotContains(t, string(got), "c98f3cb9-73b4-431f-a690-d2183c3ffb10")
	require.NotContains(t, strings.ToLower(string(got)), "deepseek")
}

func TestDisguiseStreamLine_SyntheticIDEmpty_NoIDRewrite(t *testing.T) {
	line := []byte(`data: {"type":"message_start","message":{"id":"c98f3cb9-73b4-431f-a690-d2183c3ffb10","model":"deepseek-v4-flash"}}`)
	got := DisguiseStreamLine(line, "claude-sonnet-4-5", "", false)
	// model 仍改写
	require.Contains(t, string(got), `"model":"claude-sonnet-4-5"`)
	// id 保留原样（passthrough deepseek UUID）—— 这是显式契约：仅在调用方提供 syntheticID 时才改 id
	require.Contains(t, string(got), `"id":"c98f3cb9-73b4-431f-a690-d2183c3ffb10"`)
}

func TestDisguiseJSONResponse_RewritesIDAndModel(t *testing.T) {
	body := []byte(`{"id":"c98f3cb9-73b4-431f-a690-d2183c3ffb10","type":"message","role":"assistant","model":"deepseek-v4-flash","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":3,"output_tokens":2}}`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "msg_aabbccddeeff00112233445566778899", false)
	require.Contains(t, string(got), `"id":"msg_aabbccddeeff00112233445566778899"`)
	require.Contains(t, string(got), `"model":"claude-sonnet-4-5"`)
	require.NotContains(t, string(got), "c98f3cb9-73b4-431f-a690-d2183c3ffb10")
	require.NotContains(t, strings.ToLower(string(got)), "deepseek")
	// content 不变
	require.Contains(t, string(got), `"text":"hi"`)
}

func TestDisguiseJSONResponse_OnlyIDRewriteWhenModelAlreadyClaude(t *testing.T) {
	// 上游意外回了 claude 模型名 + UUID id（极少数 fallback 状态），仍要改 id
	body := []byte(`{"id":"c98f3cb9-73b4-431f-a690-d2183c3ffb10","type":"message","model":"claude-sonnet-4-5","content":[]}`)
	got := DisguiseJSONResponse(body, "claude-sonnet-4-5", "msg_xxxxxxxxxxxxxxxxxxxxxxxx", false)
	require.Contains(t, string(got), `"id":"msg_xxxxxxxxxxxxxxxxxxxxxxxx"`)
	require.NotContains(t, string(got), "c98f3cb9-73b4-431f-a690-d2183c3ffb10")
}

func TestSynthesizeAnthropicMessageID_FormatAndUniqueness(t *testing.T) {
	id1 := synthesizeAnthropicMessageID()
	id2 := synthesizeAnthropicMessageID()
	require.NotEqual(t, id1, id2, "two synthesized IDs must differ")
	require.True(t, strings.HasPrefix(id1, "msg_"))
	require.Equal(t, len("msg_")+24, len(id1), "msg_ + 12 bytes hex = 28 chars")
	// hex only after msg_
	for _, c := range id1[4:] {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		require.True(t, isHex, "id1 contains non-hex char: %c", c)
	}
}

func TestSetDeepSeekDisguiseContext_PopulatesSyntheticID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	SetDeepSeekDisguiseContext(c, &Account{Platform: PlatformDeepSeek}, "claude-sonnet-4-5")
	cfg := getDeepSeekDisguiseContext(c)
	require.NotNil(t, cfg)
	require.True(t, strings.HasPrefix(cfg.SyntheticMessageID, "msg_"))
	require.Equal(t, cfg.SyntheticMessageID, DisguisedSyntheticMessageID(c))
}

func TestSetDeepSeekDisguiseContext_OnlyDeepSeekUserActivates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Case A: non-deepseek account → no activation
	c, _ := gin.CreateTestContext(nil)
	SetDeepSeekDisguiseContext(c, &Account{Platform: PlatformAnthropic}, "claude-sonnet-4-5")
	require.Nil(t, getDeepSeekDisguiseContext(c))

	// Case B: deepseek account, no api_key in context → activates (non-admin default)
	c, _ = gin.CreateTestContext(nil)
	SetDeepSeekDisguiseContext(c, &Account{Platform: PlatformDeepSeek}, "claude-sonnet-4-5")
	cfg := getDeepSeekDisguiseContext(c)
	require.NotNil(t, cfg)
	require.Equal(t, "claude-sonnet-4-5", cfg.OriginalModel)

	// Case C: deepseek account, admin api_key → no activation
	c, _ = gin.CreateTestContext(nil)
	c.Set("api_key", &APIKey{User: &User{Role: RoleAdmin}})
	SetDeepSeekDisguiseContext(c, &Account{Platform: PlatformDeepSeek}, "claude-sonnet-4-5")
	require.Nil(t, getDeepSeekDisguiseContext(c))

	// Case D: deepseek account, non-admin api_key → activates
	c, _ = gin.CreateTestContext(nil)
	c.Set("api_key", &APIKey{User: &User{Role: RoleUser}})
	SetDeepSeekDisguiseContext(c, &Account{Platform: PlatformDeepSeek}, "claude-sonnet-4-5")
	require.NotNil(t, getDeepSeekDisguiseContext(c))
}
