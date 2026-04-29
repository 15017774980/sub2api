//go:build unit

package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------- SetOpenAIDisguiseContext ----------

func TestSetOpenAIDisguiseContext_OnlyOpenAIUserActivates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// non-openai → no activation
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformAnthropic}, "claude-sonnet-4-5")
	require.Nil(t, getOpenAIDisguiseContext(c))

	// openai + no api_key in context → activates (non-admin default)
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")
	cfg := getOpenAIDisguiseContext(c)
	require.NotNil(t, cfg)
	require.Equal(t, "claude-sonnet-4-5", cfg.OriginalModel)
	require.True(t, strings.HasPrefix(cfg.SyntheticMessageID, "msg_"))
	require.Equal(t, len("msg_")+24, len(cfg.SyntheticMessageID))

	// openai + admin api_key → no activation
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{User: &User{Role: RoleAdmin}})
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")
	require.Nil(t, getOpenAIDisguiseContext(c))

	// openai + non-admin api_key → activates
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{User: &User{Role: RoleUser}})
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")
	require.NotNil(t, getOpenAIDisguiseContext(c))
}

// ---------- MaybeRewriteAnthropicResponseID ----------

func TestMaybeRewriteAnthropicResponseID_RewritesWhenContextActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")

	resp := &apicompat.AnthropicResponse{
		ID:    "resp_abc123def456",
		Type:  "message",
		Model: "claude-sonnet-4-5",
	}
	MaybeRewriteAnthropicResponseID(c, resp)
	require.True(t, strings.HasPrefix(resp.ID, "msg_"))
	require.NotEqual(t, "resp_abc123def456", resp.ID)
}

func TestMaybeRewriteAnthropicResponseID_NoContextNoChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	// 没 set context
	resp := &apicompat.AnthropicResponse{ID: "resp_abc"}
	MaybeRewriteAnthropicResponseID(c, resp)
	require.Equal(t, "resp_abc", resp.ID)
}

func TestMaybeRewriteAnthropicResponseID_NilResponseSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")
	require.NotPanics(t, func() {
		MaybeRewriteAnthropicResponseID(c, nil)
	})
}

// ---------- DisguiseAnthropicSSELineForOpenAI ----------

func TestDisguiseAnthropicSSELineForOpenAI_RewritesMessageStartID(t *testing.T) {
	line := `data: {"type":"message_start","message":{"id":"resp_abc123def456","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[]}}` + "\n\n"
	got := DisguiseAnthropicSSELineForOpenAI(line, "msg_aabbccddeeff00112233445566778899", false)
	require.Contains(t, got, `"id":"msg_aabbccddeeff00112233445566778899"`)
	require.NotContains(t, got, "resp_abc123def456")
	require.True(t, strings.HasPrefix(got, "data: "))
}

func TestDisguiseAnthropicSSELineForOpenAI_AdminPassthrough(t *testing.T) {
	line := `data: {"type":"message_start","message":{"id":"resp_abc"}}`
	got := DisguiseAnthropicSSELineForOpenAI(line, "msg_xx", true)
	require.Equal(t, line, got)
}

func TestDisguiseAnthropicSSELineForOpenAI_OnlyMessageStartIsRewritten(t *testing.T) {
	cases := []string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`data: {"type":"content_block_stop","index":0}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		`data: {"type":"message_stop"}`,
		`data: {"type":"ping"}`,
	}
	for _, line := range cases {
		got := DisguiseAnthropicSSELineForOpenAI(line, "msg_xx", false)
		require.Equal(t, line, got, "line %q must passthrough", line)
	}
}

func TestDisguiseAnthropicSSELineForOpenAI_NonDataLines(t *testing.T) {
	cases := []string{
		"event: message_start",
		"",
		"data: [DONE]",
		": comment",
	}
	for _, line := range cases {
		got := DisguiseAnthropicSSELineForOpenAI(line, "msg_xx", false)
		require.Equal(t, line, got)
	}
}

func TestDisguiseAnthropicSSELineForOpenAI_EmptySyntheticID_NoRewrite(t *testing.T) {
	line := `data: {"type":"message_start","message":{"id":"resp_xxx"}}`
	got := DisguiseAnthropicSSELineForOpenAI(line, "", false)
	require.Equal(t, line, got)
}

func TestDisguiseAnthropicSSELineForOpenAI_InvalidJSONPassthrough(t *testing.T) {
	line := `data: {not valid json`
	got := DisguiseAnthropicSSELineForOpenAI(line, "msg_xx", false)
	require.Equal(t, line, got)
}

// ---------- DisguiseOpenAIErrorBody ----------

func TestDisguiseOpenAIErrorBody_DetectsLeakInErrorMessage(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"error":{"message":"OpenAI rate limit exceeded","type":"rate_limit"}}`),
		[]byte(`{"error":{"message":"chatcmpl-abc was not found"}}`),
		[]byte(`{"error":{"message":"resp_xxx invalid"}}`),
		[]byte(`{"error":{"message":"ChatGPT internal error"}}`),
		[]byte(`{"error":{"message":"codex backend error"}}`),
	}
	for _, body := range cases {
		got := DisguiseOpenAIErrorBody(body, false)
		require.Contains(t, string(got), `"type":"api_error"`)
		require.Contains(t, string(got), "Upstream channel returned an error")
		lower := strings.ToLower(string(got))
		require.NotContains(t, lower, "openai")
		require.NotContains(t, lower, "chatgpt")
		require.NotContains(t, lower, "chatcmpl")
		require.NotContains(t, lower, "codex")
	}
}

func TestDisguiseOpenAIErrorBody_DetectsLeakInRawBody(t *testing.T) {
	body := []byte(`{"unrelated":"chatgpt-server: 500"}`)
	got := DisguiseOpenAIErrorBody(body, false)
	require.Contains(t, string(got), `"type":"api_error"`)
	require.NotContains(t, strings.ToLower(string(got)), "chatgpt")
}

func TestDisguiseOpenAIErrorBody_BenignPassthrough(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"please retry"}}`)
	got := DisguiseOpenAIErrorBody(body, false)
	require.Equal(t, string(body), string(got))
}

func TestDisguiseOpenAIErrorBody_AdminPassthrough(t *testing.T) {
	body := []byte(`{"error":{"message":"openai failed"}}`)
	got := DisguiseOpenAIErrorBody(body, true)
	require.Equal(t, string(body), string(got))
}

// ---------- MaybeDisguiseOpenAIErrorMessage ----------

func TestMaybeDisguiseOpenAIErrorMessage_RewritesLeakWhenContextActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")

	cases := []string{
		"OpenAI rate limit exceeded",
		"chatcmpl-abc was not found",
		"ChatGPT internal error",
		"codex backend error",
		"fp_a1b2c3d4 cache miss",
	}
	for _, msg := range cases {
		got := MaybeDisguiseOpenAIErrorMessage(c, msg)
		require.Equal(t, "Upstream channel returned an error", got, "msg %q must be replaced", msg)
	}
}

func TestMaybeDisguiseOpenAIErrorMessage_BenignKeptAsIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")

	require.Equal(t, "please retry", MaybeDisguiseOpenAIErrorMessage(c, "please retry"))
	require.Equal(t, "rate limit exceeded", MaybeDisguiseOpenAIErrorMessage(c, "rate limit exceeded"))
	require.Equal(t, "", MaybeDisguiseOpenAIErrorMessage(c, ""))
}

func TestMaybeDisguiseOpenAIErrorMessage_NoContextPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	// 没 set OpenAI context (chat completions 路径)
	require.Equal(t, "OpenAI failed", MaybeDisguiseOpenAIErrorMessage(c, "OpenAI failed"))
}

// ---------- MaybeInjectClaudeIdentityForOpenAI ----------

func TestMaybeInjectClaudeIdentityForOpenAI_OnlyClaudeModelsTriggerInjection(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantInject bool
	}{
		{"claude-sonnet", `{"model":"claude-sonnet-4-5","messages":[]}`, true},
		{"claude-opus-thinking", `{"model":"claude-opus-4-6-thinking","messages":[]}`, true},
		{"Claude case-insensitive", `{"model":"Claude-3","messages":[]}`, true},
		{"gpt-5.4", `{"model":"gpt-5.4","messages":[]}`, false},
		{"gpt-5-codex", `{"model":"gpt-5-codex","messages":[]}`, false},
		{"o3-mini", `{"model":"o3-mini","messages":[]}`, false},
		{"empty model", `{"model":"","messages":[]}`, false},
		{"no model field", `{"messages":[]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := MaybeInjectClaudeIdentityForOpenAI([]byte(tc.body))
			if tc.wantInject {
				require.NotEqual(t, tc.body, string(out), "must inject for claude-* model")
				require.Contains(t, string(out), "You are Claude")
			} else {
				require.Equal(t, tc.body, string(out), "must not inject for non-claude model")
			}
		})
	}
}

// ---------- ctxkey.DisguisedMessageID 注入 ----------

func TestSetOpenAIDisguiseContext_PopulatesCtxKeyForBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	SetOpenAIDisguiseContext(c, &Account{Platform: PlatformOpenAI}, "claude-sonnet-4-5")
	cfg := getOpenAIDisguiseContext(c)
	require.NotNil(t, cfg)

	// resolveUsageBillingRequestID 取的是 c.Request.Context() 里的 DisguisedMessageID
	got := OpenAIDisguisedSyntheticMessageID(c)
	require.Equal(t, cfg.SyntheticMessageID, got)
}
