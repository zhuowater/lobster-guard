package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildSSEJSONEvent(t *testing.T) {
	event := buildSSEJSONEvent("security_rewrite", map[string]interface{}{"content": "patched"})
	if !strings.Contains(event, "event: security_rewrite") {
		t.Fatalf("missing event header: %q", event)
	}
	if !strings.Contains(event, `"content":"patched"`) {
		t.Fatalf("missing json payload: %q", event)
	}
}

func TestBuildSSETextEvent_Multiline(t *testing.T) {
	event := buildSSETextEvent("lobster_guard_taint_reversal", "line1\nline2")
	if !strings.Contains(event, "data: line1\ndata: line2") {
		t.Fatalf("expected multiline SSE encoding, got %q", event)
	}
}

func TestEvaluateLLMSSETailRewrite_Rewrite(t *testing.T) {
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "sse-rewrite-1", Name: "rewrite-secret", Category: "pii_leak", Direction: "response",
		Type: "keyword", Patterns: []string{"secret-token"}, Action: "rewrite", RewriteTo: "[MASKED]", Enabled: true, Priority: 10,
	}})
	lp := &LLMProxy{ruleEngine: engine}

	result := lp.evaluateLLMSSETailRewrite("hello secret-token", "default")
	if result.Decision != "rewrite" || !result.ShouldRewrite {
		t.Fatalf("expected rewrite result, got %#v", result)
	}
	if !strings.Contains(result.Content, "[MASKED]") {
		t.Fatalf("expected rewritten content, got %q", result.Content)
	}
	if !strings.Contains(result.RewriteEvent, "event: security_rewrite") {
		t.Fatalf("expected rewrite event, got %q", result.RewriteEvent)
	}
}

func TestEvaluateLLMSSETailRewrite_Block(t *testing.T) {
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "sse-block-1", Name: "block-secret", Category: "sensitive_topic", Direction: "response",
		Type: "keyword", Patterns: []string{"drop database"}, Action: "block", Enabled: true, Priority: 10,
	}})
	lp := &LLMProxy{ruleEngine: engine}

	result := lp.evaluateLLMSSETailRewrite("please drop database", "default")
	if result.Decision != "block" || !result.HasMatch {
		t.Fatalf("expected block result, got %#v", result)
	}
	if result.ShouldRewrite {
		t.Fatalf("block result should not rewrite: %#v", result)
	}
}

func TestHandleSSEResponse_RewriteEventPrecedesDone(t *testing.T) {
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "sse-rewrite-order", Name: "rewrite-secret", Category: "pii_leak", Direction: "response",
		Type: "keyword", Patterns: []string{"secret-token"}, Action: "rewrite", RewriteTo: "[MASKED]", Enabled: true, Priority: 10,
	}})
	lp := &LLMProxy{ruleEngine: engine}
	sseBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hello "}}]}`,
		`data: {"choices":[{"delta":{"content":"secret-token"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")
	resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(sseBody))}
	rr := httptest.NewRecorder()
	lp.handleSSEResponse(rr, resp, &LLMAuditContext{TraceID: "trace-sse-order", TenantID: "default"}, "")

	body := rr.Body.String()
	rewriteIdx := strings.Index(body, "event: security_rewrite")
	doneIdx := strings.Index(body, "data: [DONE]")
	if rewriteIdx == -1 || doneIdx == -1 {
		t.Fatalf("expected rewrite event and done marker, got %q", body)
	}
	if rewriteIdx > doneIdx {
		t.Fatalf("expected rewrite event before done marker, got %q", body)
	}
}
