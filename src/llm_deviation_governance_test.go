package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestApplyDeviationRepair_UsesRepairedToolAndArgs(t *testing.T) {
	lp := &LLMProxy{}
	tool, args := lp.applyDeviationRepair("trace-1", "old-tool", `{"a":1}`, &DeviationResult{Repaired: true, RepairedTool: "new-tool", RepairedArgs: `{"b":2}`})
	if tool != "new-tool" || args != `{"b":2}` {
		t.Fatalf("unexpected repaired result: %s %s", tool, args)
	}
}

func TestEvaluateDeviationForTool_BlockWritesResponse(t *testing.T) {
	db := openTestSQLite(t)
	pc := NewPlanCompiler(db, PlanConfig{Enabled: true, StrictMode: true})
	if plan := pc.CompileIntent("trace-1", "send email to alice"); plan == nil {
		t.Fatal("expected active plan")
	}
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true}, pc, nil)
	lp := &LLMProxy{deviationDetector: dd}
	rr := httptest.NewRecorder()
	tool, args, blocked := lp.evaluateDeviationForTool(rr, llmDeviationGovernanceContext{TraceID: "trace-1"}, "shell_exec", `{}`)
	if !blocked {
		t.Fatal("expected deviation block")
	}
	if tool != "shell_exec" || args != `{}` {
		t.Fatalf("unexpected returned tool/args: %s %s", tool, args)
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "deviation detector") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
