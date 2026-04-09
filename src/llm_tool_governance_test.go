package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnforceToolPolicyDecision_NilEventNoop(t *testing.T) {
	lp := &LLMProxy{}
	result := lp.enforceToolPolicyDecision(nil, llmToolGovernanceContext{}, "tool-a", nil)
	if result.Blocked {
		t.Fatalf("expected noop result, got %#v", result)
	}
}

func TestMaybeRunCounterfactualForTool_NoVerifierNoop(t *testing.T) {
	lp := &LLMProxy{}
	result := lp.maybeRunCounterfactualForTool(nil, llmToolGovernanceContext{}, "tool-a", "{}", &ToolCallEvent{})
	if result.Blocked {
		t.Fatalf("expected noop result, got %#v", result)
	}
}

func TestEnforceToolPolicyDecision_BlockWritesResponse(t *testing.T) {
	lp := &LLMProxy{}
	rr := httptest.NewRecorder()
	result := lp.enforceToolPolicyDecision(rr, llmToolGovernanceContext{StatusCode: 200}, "execute_code", &ToolCallEvent{Decision: "block", RuleHit: "tp-001", RiskLevel: "high"})
	if !result.Blocked {
		t.Fatalf("expected blocked result, got %#v", result)
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Tool call blocked by policy") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestEvaluateToolPolicyForResponseTool_PathPolicyEscalatesToBlock(t *testing.T) {
	db := openTestSQLite(t)
	lp := &LLMProxy{
		toolPolicy:       NewToolPolicyEngine(db, ToolPolicyConfig{Enabled: true, DefaultAction: "allow"}),
		pathPolicyEngine: NewPathPolicyEngine(db),
	}
	ctx := llmToolGovernanceContext{TraceID: "trace-1", TenantID: "default"}
	lp.pathPolicyEngine.RegisterStep(ctx.TraceID, PathStep{Stage: "tool_call", Action: "web_fetch"})

	event, ok := lp.evaluateToolPolicyForResponseTool(ctx, "send_email", `{}`)
	if !ok {
		t.Fatal("expected tool policy evaluation to succeed")
	}
	if event.Decision != "block" {
		t.Fatalf("expected path policy block override, got %#v", event)
	}
	if event.RuleHit != "web_fetch_then_send_email" {
		t.Fatalf("expected path policy rule hit, got %#v", event)
	}
}

func TestMaybeRunCounterfactualForTool_SyncBlockWritesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cf-1","choices":[{"message":{"role":"assistant","content":"safe"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	verifier := NewCounterfactualVerifier(openTestSQLite(t), CFConfig{Enabled: true, Mode: "sync", RiskThreshold: 50, TimeoutSec: 5}, nil)
	lp := &LLMProxy{cfVerifier: verifier, mainCfg: &Config{Counterfactual: CFConfig{Enabled: true}}}
	rr := httptest.NewRecorder()
	ctx := llmToolGovernanceContext{
		TraceID:     "trace-cf",
		ReqBody:     []byte(`{"messages":[{"role":"user","content":"send it"}]}`),
		UpstreamURL: srv.URL,
	}
	result := lp.maybeRunCounterfactualForTool(rr, ctx, "send_email", `{"to":"a@example.com"}`, &ToolCallEvent{RiskLevel: "high"})
	if !result.Blocked {
		t.Fatalf("expected counterfactual block, got %#v", result)
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "counterfactual verification") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
