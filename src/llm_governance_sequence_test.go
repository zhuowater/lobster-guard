package main

import (
	"net/http/httptest"
	"testing"
)

func TestDeepGovernanceSequence_DeviationRepairFeedsIFC(t *testing.T) {
	db := openTestSQLite(t)
	pc := NewPlanCompiler(db, PlanConfig{Enabled: true})
	if plan := pc.CompileIntent("trace-seq", "write report for alice"); plan == nil {
		t.Fatal("expected active plan")
	}
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, AutoRepair: true}, pc, nil)
	ifc := NewIFCEngine(db, IFCConfig{Enabled: true, DefaultConf: ConfPublic, DefaultInteg: IntegLow, ViolationAction: "block"})
	lp := &LLMProxy{deviationDetector: dd, ifcEngine: ifc}
	args := `{"path":"/tmp/out.txt","content":"hello"}`

	origBlocked := lp.evaluateIFCForTool(httptest.NewRecorder(), llmIFCGovernanceContext{TraceID: "trace-seq-orig"}, "file_write", args)
	if !origBlocked {
		t.Fatal("expected original file_write to be blocked by IFC")
	}

	repairedTool, repairedArgs, blockedByDeviation := lp.evaluateDeviationForTool(httptest.NewRecorder(), llmDeviationGovernanceContext{TraceID: "trace-seq"}, "file_write", args)
	if blockedByDeviation {
		t.Fatal("expected deviation stage to repair, not block")
	}
	if repairedTool != "data_gather" {
		t.Fatalf("expected repaired tool data_gather, got %q", repairedTool)
	}
	if repairedArgs != args {
		t.Fatalf("expected args preserved after tool repair, got %q", repairedArgs)
	}

	repairedBlocked := lp.evaluateIFCForTool(httptest.NewRecorder(), llmIFCGovernanceContext{TraceID: "trace-seq"}, repairedTool, repairedArgs)
	if repairedBlocked {
		t.Fatalf("expected repaired tool %q to pass IFC", repairedTool)
	}
}
