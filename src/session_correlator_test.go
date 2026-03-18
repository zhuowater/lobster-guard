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
	body := `{"messages":[{"role":"system","content":"你是助手"},{"role":"user","content":"帮我查张三的邮件"}]}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_AnthropicFormat(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"帮我查张三的邮件"}]}]}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_ToolResultFallback(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":"帮我查张三的邮件"},
		{"role":"assistant","content":"正在查询..."},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"结果"}]}
	]}`
	fp := extractFirstUserFingerprint([]byte(body))
	// 最后一条 user 是 tool_result（无 text block），回退到前一条
	expected := contentFingerprint("帮我查张三的邮件")
	if fp != expected {
		t.Errorf("should fallback: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_LastUserMessage(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":"第一轮"},
		{"role":"assistant","content":"回复"},
		{"role":"user","content":"第二轮"}
	]}`
	fp := extractFirstUserFingerprint([]byte(body))
	expected := contentFingerprint("第二轮")
	if fp != expected {
		t.Errorf("should match last user: got %s, want %s", fp, expected)
	}
}

func TestExtractFirstUserFingerprint_Empty(t *testing.T) {
	cases := [][]byte{nil, []byte(`{}`), []byte(`{"messages":[]}`), []byte(`{"messages":[{"role":"system","content":"hi"}]}`), []byte(`not json`)}
	for _, c := range cases {
		if fp := extractFirstUserFingerprint(c); fp != "" {
			t.Errorf("should return empty for %q, got %s", c, fp)
		}
	}
}

// ============================================================
// extractAllUserFingerprints 测试
// ============================================================

func TestExtractAllUserFingerprints(t *testing.T) {
	body := `{"messages":[
		{"role":"user","content":"msg1"},
		{"role":"assistant","content":"reply"},
		{"role":"user","content":"msg2"},
		{"role":"user","content":"msg2"}
	]}`
	fps := extractAllUserFingerprints([]byte(body))
	if len(fps) != 2 {
		t.Errorf("expected 2 unique fps, got %d", len(fps))
	}
}

// ============================================================
// extractTextFromContent 测试
// ============================================================

func TestExtractTextFromContent_String(t *testing.T) {
	text := extractTextFromContent(json.RawMessage(`"hello world"`))
	if text != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", text)
	}
}

func TestExtractTextFromContent_TextBlocks(t *testing.T) {
	text := extractTextFromContent(json.RawMessage(`[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]`))
	if text != "line1\nline2" {
		t.Errorf("expected 'line1\\nline2', got '%s'", text)
	}
}

func TestExtractTextFromContent_MixedBlocks(t *testing.T) {
	text := extractTextFromContent(json.RawMessage(`[{"type":"text","text":"hello"},{"type":"tool_result","tool_use_id":"x"}]`))
	if text != "hello" {
		t.Errorf("expected 'hello', got '%s'", text)
	}
}

func TestExtractTextFromContent_Empty(t *testing.T) {
	if text := extractTextFromContent(nil); text != "" {
		t.Error("nil should return empty")
	}
}

// ============================================================
// SessionCorrelator — Layer 1 活跃会话
// ============================================================

func TestSession_SameUserContinuous(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000) // 1小时 idle

	// 同一用户连续发消息 → 同一 session
	sc.RegisterIMSession("msg1", "im-001", "zhangzhuo", "app1")
	sc.RegisterIMSession("msg2", "im-002", "zhangzhuo", "app1")
	sc.RegisterIMSession("msg3", "im-003", "zhangzhuo", "app1")

	// 通过 im-001 和 im-003 查到的应该是同一个 session
	link1 := sc.GetSessionByIMTrace("im-001")
	link3 := sc.GetSessionByIMTrace("im-003")
	if link1 == nil || link3 == nil {
		t.Fatal("both should find session")
	}
	if link1.SessionID != link3.SessionID {
		t.Errorf("same session expected: %s vs %s", link1.SessionID, link3.SessionID)
	}
}

func TestSession_IdleTimeoutNewSession(t *testing.T) {
	sc := NewSessionCorrelator(1000, 100) // 100ms idle timeout

	sc.RegisterIMSession("msg1", "im-001", "zhangzhuo", "app1")

	// 记住第一个 session 的 ID
	link1Before := sc.GetSessionByIMTrace("im-001")
	if link1Before == nil {
		t.Fatal("should find session for im-001 immediately")
	}
	firstSessionID := link1Before.SessionID

	time.Sleep(200 * time.Millisecond) // 超过 idle timeout (100ms)

	sc.RegisterIMSession("msg2", "im-002", "zhangzhuo", "app1")

	link2 := sc.GetSessionByIMTrace("im-002")
	if link2 == nil {
		t.Fatal("should find session for im-002")
	}
	if firstSessionID == link2.SessionID {
		t.Error("should be different sessions after idle timeout")
	}
}

func TestSession_DifferentUsers(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("hello", "im-001", "userA", "app1")
	sc.RegisterIMSession("hello", "im-002", "userB", "app1")

	link1 := sc.GetSessionByIMTrace("im-001")
	link2 := sc.GetSessionByIMTrace("im-002")
	if link1 == nil || link2 == nil {
		t.Fatal("both should find session")
	}
	if link1.SessionID == link2.SessionID {
		t.Error("different users should have different sessions")
	}
}

func TestSession_DifferentApps(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("hello", "im-001", "zhangzhuo", "app1")
	sc.RegisterIMSession("hello", "im-002", "zhangzhuo", "app2")

	link1 := sc.GetSessionByIMTrace("im-001")
	link2 := sc.GetSessionByIMTrace("im-002")
	if link1 == nil || link2 == nil {
		t.Fatal("both should find session")
	}
	if link1.SessionID == link2.SessionID {
		t.Error("different apps should have different sessions")
	}
}

// ============================================================
// SessionCorrelator — Layer 2 指纹匹配
// ============================================================

func TestMatch_Layer2_Fingerprint(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("帮我查张三邮件", "im-001", "zhangzhuo", "app1")

	body := `{"messages":[{"role":"user","content":"帮我查张三邮件"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link == nil {
		t.Fatal("should match")
	}
	if link.Method != "fingerprint" {
		t.Errorf("expected fingerprint method, got %s", link.Method)
	}
	if link.IMTraceID != "im-001" {
		t.Errorf("expected im-001, got %s", link.IMTraceID)
	}
}

func TestMatch_Layer2_MultipleIMMessages(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	// 同一 session 两条消息
	sc.RegisterIMSession("帮我查张三邮件", "im-001", "zhangzhuo", "app1")
	sc.RegisterIMSession("转发给李四", "im-002", "zhangzhuo", "app1")

	// LLM 请求 messages 包含两条 user message（多轮累积）
	body := `{"messages":[
		{"role":"user","content":"帮我查张三邮件"},
		{"role":"assistant","content":"找到了"},
		{"role":"user","content":"转发给李四"}
	]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link == nil {
		t.Fatal("should match")
	}
	if link.Method != "fingerprint" {
		t.Errorf("expected fingerprint, got %s", link.Method)
	}
	// 应该匹配到最后一条 "转发给李四" → im-002
	if link.IMTraceID != "im-002" {
		t.Errorf("expected im-002 (last user msg), got %s", link.IMTraceID)
	}
}

func TestMatch_Layer2_FallbackToOlderFingerprint(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("帮我查张三邮件", "im-001", "zhangzhuo", "app1")

	// LLM 第 2 轮：最后一条 user 是 tool_result（无 text），但历史里有原始消息
	body := `{"messages":[
		{"role":"user","content":"帮我查张三邮件"},
		{"role":"assistant","content":"调用工具..."},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"data"}]}
	]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-002")
	if link == nil {
		t.Fatal("should match via allFPs fallback")
	}
	if link.IMTraceID != "im-001" {
		t.Errorf("expected im-001, got %s", link.IMTraceID)
	}
}

func TestMatch_Layer1_ActiveSession_Fallback(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	// 注册 session，但用一条指纹不会匹配的消息
	sc.RegisterIMSession("你好世界", "im-001", "zhangzhuo", "app1")

	// LLM 请求的 user message 完全不同
	body := `{"messages":[{"role":"user","content":"完全不一样的内容，不可能指纹匹配"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")

	// 应该退化到 Layer 1 最近活跃 session
	if link == nil {
		t.Fatal("should fallback to active session")
	}
	if link.Method != "active_session" {
		t.Errorf("expected active_session method, got %s", link.Method)
	}
	if link.SenderID != "zhangzhuo" {
		t.Errorf("expected zhangzhuo, got %s", link.SenderID)
	}
}

func TestMatch_FingerprintWindowExpiry(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)
	sc.fpWindowMs = 50 // 50ms 指纹窗口

	sc.RegisterIMSession("test", "im-001", "user1", "app1")

	time.Sleep(100 * time.Millisecond) // 等指纹过期

	body := `{"messages":[{"role":"user","content":"test"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")

	// 指纹过期了，但 session 还活跃（idleTimeout=1h），应该退化到 Layer 1
	if link == nil {
		t.Fatal("should fallback to active session")
	}
	if link.Method != "active_session" {
		t.Errorf("expected active_session after fp expiry, got %s", link.Method)
	}
}

func TestMatch_NoSession(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	body := `{"messages":[{"role":"user","content":"no one registered"}]}`
	link := sc.MatchLLMRequest([]byte(body), "llm-001")
	if link != nil {
		t.Error("should return nil when no sessions exist")
	}
}

// ============================================================
// GetSessionTraces 测试
// ============================================================

func TestGetSessionTraces(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("msg1", "im-001", "zhangzhuo", "app1")
	sc.RegisterIMSession("msg2", "im-002", "zhangzhuo", "app1")

	body1 := `{"messages":[{"role":"user","content":"msg1"}]}`
	body2 := `{"messages":[{"role":"user","content":"msg2"}]}`
	link1 := sc.MatchLLMRequest([]byte(body1), "llm-001")
	sc.MatchLLMRequest([]byte(body2), "llm-002")

	if link1 == nil {
		t.Fatal("should match")
	}

	imTraces, llmTraces := sc.GetSessionTraces(link1.SessionID)
	if len(imTraces) != 2 {
		t.Errorf("expected 2 IM traces, got %d", len(imTraces))
	}
	if len(llmTraces) != 2 {
		t.Errorf("expected 2 LLM traces, got %d", len(llmTraces))
	}
}

// ============================================================
// Stats 测试
// ============================================================

func TestStats(t *testing.T) {
	sc := NewSessionCorrelator(1000, 3600000)

	sc.RegisterIMSession("msg1", "im-001", "user1", "app1")
	sc.RegisterIMSession("msg2", "im-002", "user2", "app1")

	body := `{"messages":[{"role":"user","content":"msg1"}]}`
	sc.MatchLLMRequest([]byte(body), "llm-001")

	stats := sc.Stats()
	if stats["active_sessions"].(int) != 2 {
		t.Errorf("expected 2 sessions, got %v", stats["active_sessions"])
	}
	if stats["fingerprints"].(int) < 1 {
		t.Error("should have fingerprints")
	}
}

// ============================================================
// Eviction 测试
// ============================================================

func TestEviction_MaxSessions(t *testing.T) {
	sc := NewSessionCorrelator(3, 3600000)

	sc.RegisterIMSession("a", "im-1", "u1", "app1")
	sc.RegisterIMSession("b", "im-2", "u2", "app1")
	sc.RegisterIMSession("c", "im-3", "u3", "app1")
	sc.RegisterIMSession("d", "im-4", "u4", "app1") // 触发淘汰

	stats := sc.Stats()
	if stats["active_sessions"].(int) > 3 {
		t.Errorf("should evict to maxSize, got %d", stats["active_sessions"])
	}
}

// ============================================================
// 并发安全
// ============================================================

func TestConcurrency(t *testing.T) {
	sc := NewSessionCorrelator(10000, 3600000)

	done := make(chan bool, 40)
	for i := 0; i < 20; i++ {
		go func(n int) {
			defer func() { done <- true }()
			msg := string(rune('A'+n)) + " msg"
			sc.RegisterIMSession(msg, "im-"+string(rune('0'+n)), "user", "app")
		}(i)
	}
	for i := 0; i < 20; i++ {
		go func(n int) {
			defer func() { done <- true }()
			body := `{"messages":[{"role":"user","content":"` + string(rune('A'+n)) + ` msg"}]}`
			sc.MatchLLMRequest([]byte(body), "llm-"+string(rune('0'+n)))
		}(i)
	}
	for i := 0; i < 40; i++ {
		<-done
	}
}
