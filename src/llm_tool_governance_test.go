package main

import "testing"

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