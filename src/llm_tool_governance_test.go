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