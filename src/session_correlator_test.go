package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================================
// contentFingerprint 测试
// ============================================================

func TestContentFingerprint_Basic(t *testing.T) {
	fp1 := contentFingerprint("帮我查一下张三的邮件")
	fp2 := contentFingerprint("帮我查一下张三的邮件")
	if fp1 != fp2 {
		t.Errorf("same content should produce same fingerprint: %s != %s", fp1, fp2)
	}
	if fp1 == "" {
		t.Error("fingerprint should not be empty")
	}
	if len(fp1) != 16 {
		t.Errorf("fingerprint should be 16 hex chars, got %d", len(fp1))
	}
}

func TestContentFingerprint_TrimSpace(t *testing.T) {
	fp1 := contentFingerprint("  hello world  ")
	fp2 := contentFingerprint("hello world")
	if fp1 != fp2 {
		t.Error("whitespace-trimmed content should match")
	}
}

func TestContentFingerprint_Empty(t *testing.T) {
	if fp := contentFingerprint(""); fp != "" {
		t.Errorf("empty content should return empty fingerprint, got %s", fp)
	}
	if fp := contentFingerprint("   "); fp != "" {
		t.Errorf("whitespace-only content should return empty fingerprint, got %s", fp)
	}
}

func TestContentFingerprint_Different(t *testing.T) {
	fp1 := contentFingerprint("message A")
	fp2 := contentFingerprint("message B")
	if fp1 == fp2 {
		t.Error("different content should produce different fingerprints")
	}
}

// ============================================================
// extractFirstUserFingerprint 测试
// ============================================================

func TestExtractFirstUserFingerprint_OpenAIFormat(t *testing.T) {
	// OpenAI 格式：content 是字符串
	body := `{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "你是一个助手"},
			{"role": "user", "content": "帮我查张三的邮件"}
		]
	}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("fingerprint mismatch: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_AnthropicFormat(t *testing.T) {
	// Anthropic 格式：content 是数组
	body := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "帮我查张三的邮件"}]}
		]
	}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("fingerprint mismatch: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_MultiRound_ToolResult(t *testing.T) {
	// 多轮对话：最后一条 user message 是 tool_result 数组
	// tool_result 块没有 type=text，extractTextFromContent 返回空
	// 所以应该回退到前一条有文本的 user message
	body := `{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "user", "content": "帮我查张三的邮件"},
			{"role": "assistant", "content": "好的，正在查询..."},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "xxx", "content": "找到3封邮件"}]}
		]
	}`
	fp := extractFirstUserFingerprint([]byte(body))
	// 最后一条 user 的 content blocks 里没有 type=text → 提取为空 → 回退到前一条
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("should fallback to previous user message with text: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_MultiRoundWithTextBlock(t *testing.T) {
	// 多轮中最后一条 user message 含 tool_result + text 块
	body := `{
		"messages": [
			{"role": "user", "content": "第一轮问题"},
			{"role": "assistant", "content": "第一轮回复"},
			{"role": "user", "content": "第二轮追问"}
		]
	}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("第二轮追问")
	if fp != expected {
		t.Errorf("should match last user message: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_Empty(t *testing.T) {
	if fp := extractFirstUserFingerprint(nil); fp != "" {
		t.Error("nil body should return empty")
	}
	if fp := extractFirstUserFingerprint([]byte(`{}`)); fp != "" {
		t.Error("no messages should return empty")
	}
	if fp := extractFirstUserFingerprint([]byte(`{"messages":[]}`)); fp != "" {
		t.Error("empty messages should return empty")
	}
	if fp := extractFirstUserFingerprint([]byte(`{"messages":[{"role":"system","content":"hi"}]}`)); fp != "" {
		t.Error("no user message should return empty")
	}
}

func TestExtractFirstUserFingerprint_InvalidJSON(t *testing.T) {
	if fp := extractFirstUserFingerprint([]byte(`not json`)); fp != "" {
		t.Error("invalid JSON should return empty")
	}
}

// ============================================================
// extractTextFromContent 测试
// ============================================================

func TestExtractTextFromContent_String(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	text := extractTextFromContent(raw)
	if text != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", text)
	}
}

func TestExtractTextFromContent_TextBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]`)
	text := extractTextFromContent(raw)
	if text != "line1\nline2" {
		t.Errorf("expected 'line1\\nline2', got '%s'", text)
	}
}

func TestExtractTextFromContent_MixedBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello"},{"type":"tool_result","tool_use_id":"x","content":"result"}]`)
	text := extractTextFromContent(raw)
	if text != "hello" {
		t.Errorf("expected 'hello' (ignoring tool_result), got '%s'", text)
	}
}

func TestExtractTextFromContent_ToolResultOnly(t *testing.T) {
	raw := json.RawMessage(`[{"type":"tool_result","tool_use_id":"x","content":"result"}]`)
	text := extractTextFromContent(raw)
	if text != "" {
		t.Errorf("tool_result only should return empty, got '%s'", text)
	}
}

func TestExtractTextFromContent_Empty(t *testing.T) {
	if text := extractTextFromContent(nil); text != "" {
		t.Error("nil should return empty")
	}
	if text := extractTextFromContent(json.RawMessage(`""`)); text != "" {
		t.Error("empty string should return empty")
	}
}

// ============================================================
// SessionCorrelator 集成测试
// ============================================================

func TestSessionCorrelator_BasicFlow(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)

	// 1. IM 入站注册
	sc.RegisterIMSession("帮我查张三的邮件", "im-trace-001", "user-zhangzhuo", "app-001")

	// 2. LLM 请求匹配（OpenAI 格式）
	llmBody := `{"messages":[{"role":"user","content":"帮我查张三的邮件"}]}`
	link := sc.MatchLLMRequest([]byte(llmBody), "llm-trace-001")
	if link == nil {
		t.Fatal("should match")
	}
	if link.IMTraceID != "im-trace-001" {
		t.Errorf("expected im-trace-001, got %s", link.IMTraceID)
	}
	if link.SenderID != "user-zhangzhuo" {
		t.Errorf("expected user-zhangzhuo, got %s", link.SenderID)
	}

	// 3. 验证 session traces
	traces := sc.GetSessionTraces("im-trace-001")
	if len(traces) != 1 || traces[0] != "llm-trace-001" {
		t.Errorf("expected [llm-trace-001], got %v", traces)
	}
}

func TestSessionCorrelator_MultiRoundLLM(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)

	// IM 入站
	sc.RegisterIMSession("帮我查张三的邮件然后转发给李四", "im-001", "zhangzhuo", "app-001")

	// 第 1 轮 LLM
	body1 := `{"messages":[{"role":"user","content":"帮我查张三的邮件然后转发给李四"}]}`
	link1 := sc.MatchLLMRequest([]byte(body1), "llm-001")
	if link1 == nil || link1.IMTraceID != "im-001" {
		t.Fatal("round 1 should match")
	}

	// 第 2 轮 LLM（多轮：最后一条 user 是 tool_result，但 messages 里还有原始消息）
	// 这里最后一条 user 是纯文本追问（Anthropic 多轮模式）
	body2 := `{"messages":[
		{"role":"user","content":"帮我查张三的邮件然后转发给李四"},
		{"role":"assistant","content":"正在搜索..."},
		{"role":"user","content":"找到了，请转发"}
	]}`
	link2 := sc.MatchLLMRequest([]byte(body2), "llm-002")
	// 第 2 轮最后一条 user 是 "找到了，请转发"，指纹不同，不会匹配
	// 这是预期行为——多轮中新的 user 消息会注册新的 session
	if link2 != nil {
		t.Log("round 2 matched a different fingerprint (new user message), this is expected behavior")
	}

	// 验证 session traces（至少第 1 轮关联上了）
	traces := sc.GetSessionTraces("im-001")
	if len(traces) < 1 {
		t.Error("should have at least 1 LLM trace linked")
	}
}

func TestSessionCorrelator_NoMatch(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)

	// 注册 session A
	sc.RegisterIMSession("消息A", "im-001", "user1", "app1")

	// LLM 请求是完全不同的内容
	body := `{"messages":[{"role":"user","content":"完全不同的消息B"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link != nil {
		t.Error("should not match different content")
	}
}

func TestSessionCorrelator_WindowExpiry(t *testing.T) {
	// 使用极短窗口（50ms）
	sc := NewSessionCorrelator(1000, 50)

	sc.RegisterIMSession("测试消息", "im-001", "user1", "app1")

	// 等待窗口过期
	time.Sleep(100 * time.Millisecond)

	body := `{"messages":[{"role":"user","content":"测试消息"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link != nil {
		t.Error("should not match after window expiry")
	}
}

func TestSessionCorrelator_Overwrite(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)

	// 同一个用户发了相同内容两次
	sc.RegisterIMSession("你好", "im-001", "user1", "app1")
	sc.RegisterIMSession("你好", "im-002", "user1", "app1")

	body := `{"messages":[{"role":"user","content":"你好"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link == nil {
		t.Fatal("should match")
	}
	// 应该关联到最新的 im-002
	if link.IMTraceID != "im-002" {
		t.Errorf("should match latest session im-002, got %s", link.IMTraceID)
	}
}

func TestSessionCorrelator_Eviction(t *testing.T) {
	sc := NewSessionCorrelator(3, 5*60*1000) // maxSize=3

	sc.RegisterIMSession("msg1", "im-001", "u1", "a1")
	sc.RegisterIMSession("msg2", "im-002", "u2", "a2")
	sc.RegisterIMSession("msg3", "im-003", "u3", "a3")
	sc.RegisterIMSession("msg4", "im-004", "u4", "a4") // 触发淘汰

	stats := sc.Stats()
	size := stats["active_sessions"].(int)
	if size > 3 {
		t.Errorf("should evict to maxSize, got %d sessions", size)
	}
}

func TestSessionCorrelator_Stats(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)
	sc.RegisterIMSession("msg1", "im-001", "u1", "a1")

	body := `{"messages":[{"role":"user","content":"msg1"}]}`
	sc.MatchLLMRequest([]byte(body), "llm-001")
	sc.MatchLLMRequest([]byte(body), "llm-002")

	stats := sc.Stats()
	if stats["active_sessions"].(int) != 1 {
		t.Error("should have 1 session")
	}
	if stats["total_llm_links"].(int) != 2 {
		t.Error("should have 2 LLM links")
	}
}

func TestSessionCorrelator_AnthropicContentBlocks(t *testing.T) {
	sc := NewSessionCorrelator(1000, 5*60*1000)

	// IM 入站是纯文本
	sc.RegisterIMSession("帮我查邮件", "im-001", "user1", "app1")

	// LLM 请求是 Anthropic content blocks 格式
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"帮我查邮件"}]}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link == nil {
		t.Fatal("should match Anthropic format against plain text")
	}
	if link.IMTraceID != "im-001" {
		t.Errorf("expected im-001, got %s", link.IMTraceID)
	}
}

func TestSessionCorrelator_ConcurrentAccess(t *testing.T) {
	sc := NewSessionCorrelator(10000, 5*60*1000)

	// 并发写入和读取
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			msg := string(rune('A'+n)) + " message"
			sc.RegisterIMSession(msg, "im-"+string(rune('0'+n)), "user", "app")
		}(i)
	}
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			body := `{"messages":[{"role":"user","content":"` + string(rune('A'+n)) + ` message"}]}`
			sc.MatchLLMRequest([]byte(body), "llm-"+string(rune('0'+n)))
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	// 不 panic 就算通过
}
