// plan_compiler_test.go — PlanCompiler tests (v25.0)
package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestPlanCompiler(t *testing.T) (*PlanCompiler, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	cfg := PlanConfig{
		Enabled:         true,
		StrictMode:      false,
		MaxStepsPerPlan: 20,
		DefaultTimeout:  300,
		AutoComplete:    true,
		ViolationAction: "warn",
		MatchThreshold:  0.1,
		MaxActivePlans:  100,
		RetentionDays:   30,
	}
	pc := NewPlanCompiler(db, cfg)
	return pc, db
}

func TestPlanCompilerInit(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	templates := pc.ListTemplates()
	if len(templates) < 20 {
		t.Errorf("expected >=20 builtin templates, got %d", len(templates))
	}
}

func TestPlanCompilerBuiltinCategories(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	cats := map[string]int{}
	for _, tpl := range pc.ListTemplates() {
		cats[tpl.Category]++
	}
	expectedCats := []string{"query", "email", "file", "code", "web", "admin"}
	for _, c := range expectedCats {
		if cats[c] == 0 {
			t.Errorf("expected category %s to have templates", c)
		}
	}
}

func TestPlanCompilerCompileIntentMatch(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	plan := pc.CompileIntent("trace-001", "please search for golang tutorials and find information")
	if plan == nil {
		t.Fatal("expected plan to be created")
	}
	if plan.Status != "active" {
		t.Errorf("expected active, got %s", plan.Status)
	}
	if plan.TotalSteps == 0 {
		t.Error("expected steps > 0")
	}
}

func TestPlanCompilerCompileIntentNoMatch(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.config.MatchThreshold = 0.99
	plan := pc.CompileIntent("trace-nomatch", "xyzzy random gibberish")
	if plan != nil {
		t.Error("expected nil plan for non-matching query")
	}
}

func TestPlanCompilerCompileIntentDuplicate(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	p1 := pc.CompileIntent("trace-dup", "search for something")
	p2 := pc.CompileIntent("trace-dup", "search for something else")
	if p1 == nil || p2 == nil {
		t.Fatal("both should return plan")
	}
	if p1.ID != p2.ID {
		t.Error("duplicate compile should return same plan")
	}
}

func TestPlanCompilerEvaluateToolCallExact(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-eval", "search for data and find results")
	eval := pc.EvaluateToolCall("trace-eval", "web_search", `{"q":"data"}`)
	if eval == nil {
		t.Fatal("expected evaluation")
	}
	if eval.StepMatch != "exact" {
		t.Errorf("expected exact match, got %s", eval.StepMatch)
	}
	if !eval.Allowed {
		t.Error("expected allowed")
	}
}

func TestPlanCompilerEvaluateToolCallUnexpected(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-unexp", "search for something and find")
	eval := pc.EvaluateToolCall("trace-unexp", "dangerous_tool", `{}`)
	if eval == nil {
		t.Fatal("expected evaluation")
	}
	if eval.StepMatch != "none" {
		t.Errorf("expected none match, got %s", eval.StepMatch)
	}
	if eval.Violation == nil {
		t.Error("expected violation")
	}
}

func TestPlanCompilerEvaluateToolCallNoActivePlan(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	eval := pc.EvaluateToolCall("trace-none", "web_search", `{}`)
	if !eval.Allowed {
		t.Error("expected allowed when no plan")
	}
}

func TestPlanCompilerStrictMode(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.config.StrictMode = true
	pc.CompileIntent("trace-strict", "search for golang and find tutorials")
	eval := pc.EvaluateToolCall("trace-strict", "unexpected_tool", `{}`)
	if eval.Decision != "block" {
		t.Errorf("expected block in strict mode, got %s", eval.Decision)
	}
	if eval.Violation == nil || eval.Violation.Severity != "critical" {
		t.Error("expected critical violation")
	}
}

func TestPlanCompilerAutoComplete(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	// Use check_weather (1 step)
	pc.CompileIntent("trace-ac", "what is the weather today forecast")
	eval := pc.EvaluateToolCall("trace-ac", "weather_api", `{}`)
	if eval.PlanStatus != "completed" {
		t.Errorf("expected completed, got %s", eval.PlanStatus)
	}
}

func TestPlanCompilerCompletePlan(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-complete", "search for data and find")
	pc.CompletePlan("trace-complete")
	plan := pc.GetPlan("trace-complete")
	if plan == nil || plan.Status != "completed" {
		t.Error("expected completed plan")
	}
}

func TestPlanCompilerAddTemplate(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	tpl, err := pc.AddTemplate(PlanTemplate{
		Name:     "custom_test",
		Category: "custom",
		Keywords: []string{"custom", "test"},
		Steps:    []PlanStep{{Order: 1, ToolName: "custom_tool", Required: true}},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("add template: %v", err)
	}
	if tpl.ID == "" {
		t.Error("expected ID")
	}
	got := pc.GetTemplate(tpl.ID)
	if got == nil || got.Name != "custom_test" {
		t.Error("expected to find template")
	}
}

func TestPlanCompilerAddTemplateNoName(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	_, err := pc.AddTemplate(PlanTemplate{Category: "test"})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestPlanCompilerUpdateTemplate(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	tpl, _ := pc.AddTemplate(PlanTemplate{Name: "upd_test", Category: "test", Enabled: true})
	err := pc.UpdateTemplate(tpl.ID, PlanTemplate{Name: "upd_test_v2", Enabled: true, Priority: 50})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got := pc.GetTemplate(tpl.ID)
	if got.Name != "upd_test_v2" || got.Priority != 50 {
		t.Error("update not applied")
	}
}

func TestPlanCompilerDeleteTemplate(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	tpl, _ := pc.AddTemplate(PlanTemplate{Name: "del_test", Category: "test", Enabled: true})
	err := pc.DeleteTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if pc.GetTemplate(tpl.ID) != nil {
		t.Error("expected template to be deleted")
	}
}

func TestPlanCompilerDeleteNotFound(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	err := pc.DeleteTemplate("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestPlanCompilerQueryPlans(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-q1", "search for golang and find")
	pc.CompileIntent("trace-q2", "check weather forecast")
	plans, total := pc.QueryPlans("", 10, 0)
	if total < 2 {
		t.Errorf("expected >= 2 plans, got %d", total)
	}
	if len(plans) < 2 {
		t.Errorf("expected >= 2 plans in result")
	}
}

func TestPlanCompilerGetStats(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-st1", "search for golang and find")
	pc.CompileIntent("trace-st2", "check weather forecast")
	stats := pc.GetStats()
	if stats.TotalPlans < 2 {
		t.Errorf("expected >= 2 total, got %d", stats.TotalPlans)
	}
	if stats.TemplateCount < 20 {
		t.Errorf("expected >= 20 templates, got %d", stats.TemplateCount)
	}
}

func TestPlanCompilerQueryViolations(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-viol", "search for data and find")
	pc.EvaluateToolCall("trace-viol", "bad_tool", `{}`)
	viols, total := pc.QueryViolations("trace-viol", 10, 0)
	if total == 0 {
		t.Error("expected violations")
	}
	if len(viols) == 0 {
		t.Error("expected violation records")
	}
}

func TestPlanCompilerDisabled(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.config.Enabled = false
	plan := pc.CompileIntent("trace-dis", "search for data")
	if plan != nil {
		t.Error("expected nil when disabled")
	}
	eval := pc.EvaluateToolCall("trace-dis", "tool", "{}")
	if !eval.Allowed {
		t.Error("expected allowed when disabled")
	}
}

func TestPlanCompilerGetConfig(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	cfg := pc.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}
}

func TestPlanCompilerUpdateConfig(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.UpdateConfig(PlanConfig{Enabled: false, ViolationAction: "block"})
	cfg := pc.GetConfig()
	if cfg.Enabled {
		t.Error("expected disabled after update")
	}
	if cfg.ViolationAction != "block" {
		t.Errorf("expected block, got %s", cfg.ViolationAction)
	}
}

func TestPlanCompilerGetPlanFromDB(t *testing.T) {
	pc, _ := newTestPlanCompiler(t)
	pc.CompileIntent("trace-dbget", "search for golang and find")
	// Remove from memory cache to force DB lookup
	pc.mu.Lock()
	delete(pc.activePlans, "trace-dbget")
	pc.mu.Unlock()
	plan := pc.GetPlan("trace-dbget")
	if plan == nil {
		t.Fatal("expected plan from DB")
	}
	if plan.TraceID != "trace-dbget" {
		t.Errorf("expected trace-dbget, got %s", plan.TraceID)
	}
}
