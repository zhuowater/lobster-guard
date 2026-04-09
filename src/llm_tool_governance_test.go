package main

import (
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
