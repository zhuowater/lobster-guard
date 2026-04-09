package main

import "testing"

func TestCollectIFCVarIDs_Empty(t *testing.T) {
	if ids := collectIFCVarIDs(nil); len(ids) != 0 {
		t.Fatalf("expected empty ids, got %#v", ids)
	}
}

func TestEvaluateIFCForTool_NoEngineNoop(t *testing.T) {
	lp := &LLMProxy{}
	blocked := lp.evaluateIFCForTool(nil, llmIFCGovernanceContext{}, "tool-a", "{}")
	if blocked {
		t.Fatal("expected no block without ifc engine")
	}
}