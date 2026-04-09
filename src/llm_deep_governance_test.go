package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestEvaluatePlanForTool_BlockWritesResponse(t *testing.T) {
	db := openTestSQLite(t)
	pc := NewPlanCompiler(db, PlanConfig{Enabled: true, StrictMode: true})
	if plan := pc.CompileIntent("trace-1", "send email to alice"); plan == nil {
		t.Fatal("expected active plan")
	}
	lp := &LLMProxy{planCompiler: pc}
	rr := httptest.NewRecorder()
	blocked := lp.evaluatePlanForTool(rr, llmDeepGovernanceContext{TraceID: "trace-1"}, "shell_exec", `{}`)
	if !blocked {
		t.Fatal("expected plan compiler block")
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "plan compiler") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}
