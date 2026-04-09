package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestEvaluateIFCForTool_BlockWritesResponse(t *testing.T) {
	db := openTestSQLite(t)
	eng := NewIFCEngine(db, IFCConfig{Enabled: true, DefaultConf: ConfPublic, DefaultInteg: IntegLow, ViolationAction: "block"})
	lp := &LLMProxy{ifcEngine: eng}
	rr := httptest.NewRecorder()
	blocked := lp.evaluateIFCForTool(rr, llmIFCGovernanceContext{TraceID: "trace-1"}, "send_email", `{"body":"hello"}`)
	if !blocked {
		t.Fatal("expected IFC block")
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Tool call blocked by IFC") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
