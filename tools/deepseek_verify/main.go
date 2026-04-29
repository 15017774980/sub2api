// deepseek_verify 直接命中 DeepSeek 官方 anthropic 兼容 endpoint，
// 验证 4 个 plan 中标注「待验证」的关键假设：
//
//  1. image content block 实际报错体（确认 unknown variant 字串 + 是否含 deepseek 品牌名）
//  2. cache_control:ephemeral 是否触发 prefix cache 命中（usage 是否回 cache_read_input_tokens）
//  3. 流式 + tool_use + thinking 多轮回传能否接续（thinking_block 必须原样回传的硬约束）
//  4. 不支持的 claude-* 模型名是否真会被 fallback（或 400）
//
// 用法：
//
//	export DEEPSEEK_API_KEY=sk-xxx
//	go run ./tools/deepseek_verify
//
// 仅依赖标准库，独立运行不入主 backend module。

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	endpoint    = "https://api.deepseek.com/anthropic/v1/messages"
	flashModel  = "deepseek-v4-flash"
	proModel    = "deepseek-v4-pro"
	apiVersion  = "2023-06-01"
	httpTimeout = 90 * time.Second
)

func main() {
	apiKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "[ERR] DEEPSEEK_API_KEY env var is required")
		os.Exit(1)
	}

	tests := []struct {
		name string
		fn   func(string) error
	}{
		{"01_image_content_block_400", testImageBlock400},
		{"02_cache_control_hit", testCacheControlHit},
		{"03_stream_thinking_tooluse_multiturn", testStreamThinkingToolUseMultiTurn},
		{"04_unsupported_claude_model", testUnsupportedClaudeModel},
		{"05_dump_full_headers_and_usage", testDumpFullHeadersAndUsage},
		{"06_who_are_you_baseline", testWhoAreYouBaseline},
		{"07_who_are_you_with_identity_system", testWhoAreYouWithIdentitySystem},
		{"08_style_suppression_check", testStyleSuppression},
	}

	failures := 0
	for _, t := range tests {
		fmt.Printf("\n========== %s ==========\n", t.name)
		if err := t.fn(apiKey); err != nil {
			fmt.Printf("[FAIL] %s: %v\n", t.name, err)
			failures++
		} else {
			fmt.Printf("[OK]   %s\n", t.name)
		}
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d test(s) failed\n", failures)
		os.Exit(2)
	}
	fmt.Println("\nAll tests passed.")
}

// ---------- shared http helper ----------

func doRequest(apiKey string, body any, stream bool) (*http.Response, []byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	if stream {
		// 调用方按需读 resp.Body
		return resp, nil, nil
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	return resp, respBody, nil
}

func prettyJSON(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

// ---------- test 01: image content block 应回 400 ----------

func testImageBlock400(apiKey string) error {
	// 用一张极小的占位 base64 图（1x1 透明 PNG），仅用于验证错误体格式。
	tinyPNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkAAIAAAoAAv/lxKUAAAAASUVORK5CYII="
	body := map[string]any{
		"model":      flashModel,
		"max_tokens": 64,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "describe this image"},
					{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       tinyPNG,
						},
					},
				},
			},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", prettyJSON(raw))

	// 关键观察：
	low := strings.ToLower(string(raw))
	hasDeepseek := strings.Contains(low, "deepseek")
	hasUnknownVariant := strings.Contains(low, "unknown variant")
	hasInputImage := strings.Contains(low, "image") || strings.Contains(low, "multimodal")

	fmt.Printf("Observations:\n")
	fmt.Printf("  - error body contains 'deepseek'        : %v  (plan assumption: false)\n", hasDeepseek)
	fmt.Printf("  - error body contains 'unknown variant' : %v  (plan assumption: true)\n", hasUnknownVariant)
	fmt.Printf("  - error body mentions image/multimodal  : %v\n", hasInputImage)

	if resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("expected HTTP 400, got %d", resp.StatusCode)
	}
	return nil
}

// ---------- test 02: cache_control + cache hit ----------

func testCacheControlHit(apiKey string) error {
	// 1KB+ 的稳定 system prompt，触发 prefix cache 写入；连发两次，第二次应回 cache_read_input_tokens > 0
	bigSystem := strings.Repeat("You are a helpful assistant. Stay concise. ", 80) // ~3.6KB
	mkBody := func() map[string]any {
		return map[string]any{
			"model":      flashModel,
			"max_tokens": 32,
			"system": []map[string]any{
				{
					"type":          "text",
					"text":          bigSystem,
					"cache_control": map[string]any{"type": "ephemeral"},
				},
			},
			"messages": []map[string]any{
				{"role": "user", "content": "say 'ok'"},
			},
		}
	}

	roundTrip := func(label string) (int, int, error) {
		resp, raw, err := doRequest(apiKey, mkBody(), false)
		if err != nil {
			return 0, 0, err
		}
		fmt.Printf("[%s] status=%d\n", label, resp.StatusCode)
		if resp.StatusCode >= 400 {
			fmt.Printf("[%s] body=\n%s\n", label, prettyJSON(raw))
			return 0, 0, fmt.Errorf("upstream %d", resp.StatusCode)
		}
		var parsed struct {
			Usage struct {
				InputTokens             int `json:"input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				OutputTokens             int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return 0, 0, fmt.Errorf("unmarshal: %w", err)
		}
		fmt.Printf("[%s] usage: input=%d  cache_creation=%d  cache_read=%d  output=%d\n",
			label, parsed.Usage.InputTokens, parsed.Usage.CacheCreationInputTokens,
			parsed.Usage.CacheReadInputTokens, parsed.Usage.OutputTokens)
		return parsed.Usage.CacheCreationInputTokens, parsed.Usage.CacheReadInputTokens, nil
	}

	if _, _, err := roundTrip("warm"); err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // 让 prefix cache 入库
	_, read2, err := roundTrip("hit")
	if err != nil {
		return err
	}

	fmt.Printf("Observations:\n")
	fmt.Printf("  - cache_read_input_tokens on 2nd call: %d  (>0 means cache hit)\n", read2)
	if read2 == 0 {
		fmt.Println("  WARN: 2nd call reported cache_read=0; DeepSeek anthropic endpoint may not honor cache_control yet.")
	}
	return nil
}

// ---------- test 03: stream + tool_use + thinking 多轮回传 ----------

func testStreamThinkingToolUseMultiTurn(apiKey string) error {
	// Round 1: 用 thinking + 一个简单 tool 让模型决定是否调用
	tools := []map[string]any{
		{
			"name":        "get_weather",
			"description": "Get current weather for a city.",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
				"required": []string{"city"},
			},
		},
	}
	round1 := map[string]any{
		"model":      flashModel,
		"max_tokens": 512,
		"thinking":   map[string]any{"type": "enabled", "budget_tokens": 1024},
		"tools":      tools,
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo? Use the tool if needed."},
		},
	}
	resp, _, err := doRequest(apiKey, round1, true)
	if err != nil {
		return fmt.Errorf("round1: %w", err)
	}
	fmt.Printf("[round1] status=%d\n", resp.StatusCode)
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		fmt.Printf("[round1] body=\n%s\n", prettyJSON(buf))
		return fmt.Errorf("round1 upstream %d", resp.StatusCode)
	}

	assistantBlocks, toolUse, modelName, err := readSSEAndCollectAssistant(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("round1 sse: %w", err)
	}
	fmt.Printf("[round1] model field in SSE: %q\n", modelName)
	fmt.Printf("[round1] assistant blocks count: %d (first types: %s)\n",
		len(assistantBlocks), summarizeBlockTypes(assistantBlocks))
	if toolUse == nil {
		fmt.Println("[round1] no tool_use emitted; thinking-only response. Skipping round2 multi-turn assertion.")
		return nil
	}
	fmt.Printf("[round1] tool_use detected: name=%s id=%s\n", toolUse["name"], toolUse["id"])

	// Round 2: 把 round1 的完整 assistant 回复（含 thinking 块）原样回传，附 tool_result
	round2 := map[string]any{
		"model":      flashModel,
		"max_tokens": 256,
		"thinking":   map[string]any{"type": "enabled", "budget_tokens": 1024},
		"tools":      tools,
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo? Use the tool if needed."},
			{"role": "assistant", "content": assistantBlocks},
			{"role": "user", "content": []map[string]any{
				{
					"type":         "tool_result",
					"tool_use_id":  toolUse["id"],
					"content":      "Tokyo: 18°C, partly cloudy.",
				},
			}},
		},
	}
	resp2, _, err := doRequest(apiKey, round2, true)
	if err != nil {
		return fmt.Errorf("round2: %w", err)
	}
	defer resp2.Body.Close()
	fmt.Printf("[round2] status=%d\n", resp2.StatusCode)
	if resp2.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp2.Body)
		fmt.Printf("[round2] body=\n%s\n", prettyJSON(buf))
		return fmt.Errorf("round2 upstream %d (thinking_block 回传后 400 即破坏 plan 假设)", resp2.StatusCode)
	}
	blocks2, _, _, err := readSSEAndCollectAssistant(resp2.Body)
	if err != nil {
		return fmt.Errorf("round2 sse: %w", err)
	}
	fmt.Printf("[round2] assistant blocks count: %d (types: %s)\n",
		len(blocks2), summarizeBlockTypes(blocks2))
	fmt.Println("Observations:")
	fmt.Println("  - round2 200 OK 表示 DeepSeek 接受 thinking_block 原样回传 (plan 假设成立)")
	return nil
}

// readSSEAndCollectAssistant 读取 SSE 流，重组 content_block 序列，返回 assistant blocks + 第一个 tool_use（如有）+ message_start 中的 model 字段。
func readSSEAndCollectAssistant(body io.Reader) ([]map[string]any, map[string]any, string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)

	type blockState struct {
		typ        string
		text       string
		thinking   string
		signature  string
		toolName   string
		toolID     string
		toolInput  string
	}
	blocks := map[int]*blockState{}
	var firstToolUse map[string]any
	model := ""

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		switch ev["type"] {
		case "message_start":
			if msg, ok := ev["message"].(map[string]any); ok {
				if m, ok := msg["model"].(string); ok {
					model = m
				}
			}
		case "content_block_start":
			idx := intFromAny(ev["index"])
			cb, _ := ev["content_block"].(map[string]any)
			st := &blockState{}
			if t, ok := cb["type"].(string); ok {
				st.typ = t
			}
			if st.typ == "tool_use" {
				st.toolID, _ = cb["id"].(string)
				st.toolName, _ = cb["name"].(string)
			}
			blocks[idx] = st
		case "content_block_delta":
			idx := intFromAny(ev["index"])
			st := blocks[idx]
			if st == nil {
				continue
			}
			delta, _ := ev["delta"].(map[string]any)
			switch delta["type"] {
			case "text_delta":
				if v, ok := delta["text"].(string); ok {
					st.text += v
				}
			case "thinking_delta":
				if v, ok := delta["thinking"].(string); ok {
					st.thinking += v
				}
			case "signature_delta":
				if v, ok := delta["signature"].(string); ok {
					st.signature += v
				}
			case "input_json_delta":
				if v, ok := delta["partial_json"].(string); ok {
					st.toolInput += v
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, "", err
	}
	// 按 index 顺序还原
	out := make([]map[string]any, 0, len(blocks))
	for i := 0; i < len(blocks); i++ {
		st := blocks[i]
		if st == nil {
			continue
		}
		switch st.typ {
		case "text":
			out = append(out, map[string]any{"type": "text", "text": st.text})
		case "thinking":
			b := map[string]any{"type": "thinking", "thinking": st.thinking}
			if st.signature != "" {
				b["signature"] = st.signature
			}
			out = append(out, b)
		case "tool_use":
			var input any = map[string]any{}
			if st.toolInput != "" {
				_ = json.Unmarshal([]byte(st.toolInput), &input)
			}
			b := map[string]any{
				"type":  "tool_use",
				"id":    st.toolID,
				"name":  st.toolName,
				"input": input,
			}
			if firstToolUse == nil {
				firstToolUse = b
			}
			out = append(out, b)
		}
	}
	return out, firstToolUse, model, nil
}

func summarizeBlockTypes(blocks []map[string]any) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t, ok := b["type"].(string); ok {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ",")
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// ---------- test 04: 不支持的 claude-* 模型名行为 ----------

func testUnsupportedClaudeModel(apiKey string) error {
	// 直接发 claude-* 模型名，看 DeepSeek anthropic endpoint 是 fallback 还是 400
	body := map[string]any{
		"model":      "claude-sonnet-4-5", // 故意发 claude-* 名
		"max_tokens": 16,
		"messages": []map[string]any{
			{"role": "user", "content": "say ok"},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", prettyJSON(raw))

	// 观察用，不当作失败
	switch {
	case resp.StatusCode == 200:
		var parsed struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(raw, &parsed)
		fmt.Printf("Observations:\n  - DeepSeek silently mapped claude-sonnet-4-5 → %q (fallback exists)\n", parsed.Model)
	case resp.StatusCode == 400:
		fmt.Println("Observations:\n  - DeepSeek returned 400 for claude-* model. Need explicit account.GetMappedModel mapping in our gateway.")
	default:
		fmt.Printf("Observations:\n  - Unexpected status %d. See body above.\n", resp.StatusCode)
	}
	// 不返回错误：这是观察性测试
	return nil
}

// ---------- test 05: dump 全部 response headers + usage 字段 ----------
//
// 目的：审计 disguise 中间件是否需要拦截上游特有 header / usage 字段。
// 关注点：
//   - Response headers：DeepSeek 是否回 x-deepseek-*、x-ds-*、server: <distinct> 等可指认头部
//   - id 字段格式：实测确认 UUID 格式
//   - usage 字段：是否含 anthropic 没有的 deepseek 特有 key（如 prompt_cache_hit_tokens 等）

func testDumpFullHeadersAndUsage(apiKey string) error {
	body := map[string]any{
		"model":      flashModel,
		"max_tokens": 16,
		"messages": []map[string]any{
			{"role": "user", "content": "say ok"},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)

	fmt.Println("Response headers (sorted, lower-cased):")
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		fmt.Printf("  %-40s %s\n", strings.ToLower(k), strings.Join(resp.Header.Values(k), ", "))
	}

	fmt.Printf("\nResponse body:\n%s\n", prettyJSON(raw))

	// 提取关键观察项
	var parsed struct {
		ID    string         `json:"id"`
		Type  string         `json:"type"`
		Model string         `json:"model"`
		Usage map[string]any `json:"usage"`
	}
	_ = json.Unmarshal(raw, &parsed)
	fmt.Println("Observations:")
	fmt.Printf("  - id field           : %q (anthropic real format: msg_01XYZ...)\n", parsed.ID)
	fmt.Printf("  - id is UUID-like    : %v\n", looksLikeUUID(parsed.ID))
	fmt.Printf("  - id has msg_ prefix : %v\n", strings.HasPrefix(parsed.ID, "msg_"))
	fmt.Printf("  - usage keys         : %v\n", sortedKeys(parsed.Usage))

	// header 风险点
	leakHeaders := []string{}
	for _, k := range keys {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "deepseek") || strings.Contains(lk, "ds-") {
			leakHeaders = append(leakHeaders, lk)
		}
	}
	if len(leakHeaders) > 0 {
		fmt.Printf("  - LEAKING headers    : %v\n", leakHeaders)
	} else {
		fmt.Printf("  - LEAKING headers    : none (gateway 的 header 白名单 + 这一步无需额外动作)\n")
	}
	return nil
}

// ---------- test 06: who-are-you 探测基线 ----------
//
// 目的：测一下"未注入身份 system prompt"时 DeepSeek 在 anthropic-compat endpoint 上
// 对身份探测的回复，作为 L5 注入策略效果的对照基线。

func testWhoAreYouBaseline(apiKey string) error {
	body := map[string]any{
		"model":      flashModel,
		"max_tokens": 80,
		"messages": []map[string]any{
			{"role": "user", "content": "Who are you? What model are you? Answer concisely."},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", prettyJSON(raw))

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &parsed)
	answer := ""
	for _, b := range parsed.Content {
		if b.Type == "text" {
			answer += b.Text
		}
	}
	lower := strings.ToLower(answer)
	fmt.Println("Observations:")
	fmt.Printf("  - mentions 'deepseek'  : %v\n", strings.Contains(lower, "deepseek"))
	fmt.Printf("  - mentions 'claude'    : %v\n", strings.Contains(lower, "claude"))
	fmt.Printf("  - mentions 'anthropic' : %v\n", strings.Contains(lower, "anthropic"))
	return nil
}

// ---------- test 07: 注入身份 system prompt 后的 who-are-you 行为 ----------
//
// 目的：对照 case 06，验证 L5 system prompt 注入是否能让模型在被问身份时
// 不自报 DeepSeek。允许 thinking 块仍出现"DeepSeek"——那是 thinking 暴露面（L7 跟进）。
// 这里关注 text content 字段。

func testWhoAreYouWithIdentitySystem(apiKey string) error {
	// 与后端 service.claudeIdentitySystemPrompt 保持一致——既测身份声明，也测风格条款是否能压制 hallmark。
	const identityPrompt = `You are Claude, an AI assistant made by Anthropic.

When responding:
- Be direct. Skip openings like "Sure!", "Certainly!", "Of course!".
- Skip closings like "Feel free to ask if you have more questions!".
- Use markdown sparingly; no decorative headers on short answers.
- Don't refer to yourself as "an AI" or "a language model".
- Prefer plain wording over filler like "delve", "leverage", "robust", "comprehensive".`

	body := map[string]any{
		"model":      flashModel,
		"max_tokens": 200,
		"system":     identityPrompt,
		"messages": []map[string]any{
			{"role": "user", "content": "Who are you? What model are you? Answer concisely."},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", prettyJSON(raw))

	var parsed struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &parsed)
	textAnswer := ""
	thinkingAnswer := ""
	for _, b := range parsed.Content {
		switch b.Type {
		case "text":
			textAnswer += b.Text
		case "thinking":
			thinkingAnswer += b.Thinking
		}
	}
	textLower := strings.ToLower(textAnswer)
	thinkingLower := strings.ToLower(thinkingAnswer)
	fmt.Println("Observations:")
	fmt.Printf("  - text mentions 'deepseek'      : %v\n", strings.Contains(textLower, "deepseek"))
	fmt.Printf("  - text mentions 'claude'        : %v\n", strings.Contains(textLower, "claude"))
	fmt.Printf("  - text mentions 'anthropic'     : %v\n", strings.Contains(textLower, "anthropic"))
	fmt.Printf("  - thinking mentions 'deepseek'  : %v (this is L7 leak, accepted)\n", strings.Contains(thinkingLower, "deepseek"))
	fmt.Printf("  - thinking mentions 'claude'    : %v\n", strings.Contains(thinkingLower, "claude"))
	return nil
}

// ---------- test 08: 风格压制效果检查 ----------
//
// 用一个自然的开放问题（"how do I parse JSON in Go"），观察注入新版风格指南后
// hallmark 短语是否被压制：Sure!/Certainly!/Feel free to.../delve into.../leverage...

func testStyleSuppression(apiKey string) error {
	const identityPrompt = `You are Claude, an AI assistant made by Anthropic.

When responding:
- Be direct. Skip openings like "Sure!", "Certainly!", "Of course!".
- Skip closings like "Feel free to ask if you have more questions!".
- Use markdown sparingly; no decorative headers on short answers.
- Don't refer to yourself as "an AI" or "a language model".
- Prefer plain wording over filler like "delve", "leverage", "robust", "comprehensive".`

	body := map[string]any{
		"model":      flashModel,
		"max_tokens": 400,
		"system":     identityPrompt,
		"messages": []map[string]any{
			{"role": "user", "content": "How do I parse JSON in Go?"},
		},
	}
	resp, raw, err := doRequest(apiKey, body, false)
	if err != nil {
		return err
	}
	fmt.Printf("HTTP status: %d\n", resp.StatusCode)
	fmt.Printf("Response body:\n%s\n", prettyJSON(raw))

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	_ = json.Unmarshal(raw, &parsed)
	textAnswer := ""
	for _, b := range parsed.Content {
		if b.Type == "text" {
			textAnswer += b.Text
		}
	}
	lower := strings.ToLower(textAnswer)

	hallmarks := []string{
		"sure!",
		"certainly!",
		"of course!",
		"feel free to",
		"i'd be happy to",
		"as an ai",
		"as a language model",
		"delve into",
		"delve",
		"leverage",
		"robust",
		"comprehensive",
		"seamlessly",
	}

	hits := 0
	fmt.Println("Observations (hallmark phrase hits):")
	for _, h := range hallmarks {
		if strings.Contains(lower, h) {
			fmt.Printf("  - HIT  : %q\n", h)
			hits++
		}
	}
	if hits == 0 {
		fmt.Println("  - NONE (style suppression working)")
	} else {
		fmt.Printf("  - total %d hallmark hits in text response\n", hits)
	}
	return nil
}

func looksLikeUUID(s string) bool {
	// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
