package main

import "testing"

func TestApplyDeviationRepair_NoResult(t *testing.T) {
	lp := &LLMProxy{}
	tool, args := lp.applyDeviationRepair("trace-1", "tool-a", "{}", nil)
	if tool != "tool-a" || args != "{}" {
		t.Fatalf("unexpected repair result: %s %s", tool, args)
	}
}

func TestEvaluateDeviationForTool_NoDetectorNoop(t *testing.T) {
	lp := &LLMProxy{}
	tool, args, blocked := lp.evaluateDeviationForTool(nil, llmDeviationGovernanceContext{}, "tool-a", "{}")
	if blocked || tool != "tool-a" || args != "{}" {
		t.Fatalf("unexpected deviation result: %s %s %v", tool, args, blocked)
	}
}