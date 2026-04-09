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

func TestCollectIFCVarIDs_PreservesIDs(t *testing.T) {
	ids := collectIFCVarIDs([]IFCVariable{{ID: "a"}, {ID: "b"}})
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}