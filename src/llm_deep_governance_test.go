package main

import "testing"

func TestEvaluatePlanForTool_NoPlanCompilerNoop(t *testing.T) {
	lp := &LLMProxy{}
	blocked := lp.evaluatePlanForTool(nil, llmDeepGovernanceContext{}, "tool-a", "{}")
	if blocked {
		t.Fatal("expected no block without plan compiler")
	}
}

func TestEvaluateCapabilityForTool_NoCapabilityNoop(t *testing.T) {
	lp := &LLMProxy{}
	lp.evaluateCapabilityForTool(llmDeepGovernanceContext{TraceID: "trace-1"}, "tool-a", 0, nil)
}