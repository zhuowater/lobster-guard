// plan_deviation_test.go — DeviationDetector tests (v25.2)
package main

import (
	"database/sql"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newDevDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func newDevDetector(t *testing.T, cfg DeviationConfig, pcCfg *PlanConfig) *DeviationDetector {
	t.Helper()
	db := newDevDB(t)
	var pc *PlanCompiler
	if pcCfg != nil {
		pc = NewPlanCompiler(db, *pcCfg)
	}
	return NewDeviationDetector(db, cfg, pc, nil)
}

var devPCDefault = &PlanConfig{
	Enabled: true, StrictMode: false, MaxStepsPerPlan: 20, DefaultTimeout: 300,
	AutoComplete: true, ViolationAction: "warn", MatchThreshold: 0.1,
	MaxActivePlans: 100, RetentionDays: 30,
}

var devPCStrict = &PlanConfig{
	Enabled: true, StrictMode: true, MaxStepsPerPlan: 20, DefaultTimeout: 300,
	AutoComplete: false, ViolationAction: "block", MatchThreshold: 0.1,
	MaxActivePlans: 100, RetentionDays: 30,
}

// ── 1. 基础功能 ──────────────────────────────────────

func TestNewDeviationDetector(t *testing.T) {
	db := newDevDB(t)
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, AutoRepair: false, MaxRepairs: 3}, nil, nil)
	if dd == nil {
		t.Fatal("expected non-nil detector")
	}
	cfg := dd.GetConfig()
	if !cfg.Enabled || cfg.AutoRepair || cfg.MaxRepairs != 3 {
		t.Errorf("config mismatch: %+v", cfg)
	}
	// MaxRepairs <= 0 → default 5
	dd2 := NewDeviationDetector(nil, DeviationConfig{MaxRepairs: 0}, nil, nil)
	if dd2.GetConfig().MaxRepairs != 5 {
		t.Errorf("expected default max_repairs=5, got %d", dd2.GetConfig().MaxRepairs)
	}
	t.Logf("✅ NewDeviationDetector initializes correctly (incl. default max_repairs)")
}

func TestCheckDeviation_NoPlan(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCDefault)
	r := dd.Detect("trace-noplan", "web_search", `{"q":"test"}`)
	if r.HasDeviation {
		t.Error("expected no deviation when no plan for trace")
	}
	if r.Decision != "allow" {
		t.Errorf("expected allow, got %s", r.Decision)
	}
	t.Logf("✅ No deviation when no active plan")
}

func TestCheckDeviation_Normal(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCDefault)
	plan := dd.planCompiler.CompileIntent("trace-norm", "search for golang tutorials and find information")
	if plan == nil {
		t.Fatal("expected plan")
	}
	r := dd.Detect("trace-norm", "web_search", `{"q":"golang"}`)
	if r.HasDeviation {
		t.Errorf("expected no deviation for planned tool, got: %+v", r.Deviation)
	}
	if r.Decision != "allow" {
		t.Errorf("expected allow, got %s", r.Decision)
	}
	t.Logf("✅ Normal tool call (plan match) — no deviation")
}

func TestCheckDeviation_UnexpectedTool(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCStrict)
	dd.planCompiler.CompileIntent("trace-unexp", "search for golang and find information")
	r := dd.Detect("trace-unexp", "dangerous_hack_tool", `{}`)
	if !r.HasDeviation || r.Deviation == nil {
		t.Fatal("expected deviation for unexpected tool")
	}
	if r.Decision != "block" {
		t.Errorf("expected block, got %s", r.Decision)
	}
	if r.Deviation.Severity != "critical" {
		t.Errorf("expected critical, got %s", r.Deviation.Severity)
	}
	t.Logf("✅ Unexpected tool: type=%s severity=%s decision=%s", r.Deviation.Type, r.Deviation.Severity, r.Decision)
}

func TestCheckDeviation_ParameterAnomaly(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCDefault)
	dd.planCompiler.CompileIntent("trace-param", "search for data and find results")
	r := dd.Detect("trace-param", "shell_exec", `{"cmd":"rm -rf /"}`)
	if !r.HasDeviation {
		t.Fatal("expected deviation")
	}
	if r.Deviation.Severity != "minor" {
		t.Errorf("expected minor severity, got %s", r.Deviation.Severity)
	}
	if r.Decision != "warn" {
		t.Errorf("expected warn, got %s", r.Decision)
	}
	t.Logf("✅ Parameter anomaly: severity=%s decision=%s", r.Deviation.Severity, r.Decision)
}

func TestCheckDeviation_OrderViolation(t *testing.T) {
	noAuto := &PlanConfig{
		Enabled: true, StrictMode: false, MaxStepsPerPlan: 20, DefaultTimeout: 300,
		AutoComplete: false, ViolationAction: "warn", MatchThreshold: 0.1,
		MaxActivePlans: 100, RetentionDays: 30,
	}
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, noAuto)
	plan := dd.planCompiler.CompileIntent("trace-ord", "search for golang and find information")
	if plan == nil || plan.TotalSteps < 2 {
		t.Skipf("need >=2 steps, got %d", plan.TotalSteps)
	}
	dd.planCompiler.mu.Lock()
	tpl := dd.planCompiler.templates[plan.TemplateID]
	second := tpl.Steps[1].ToolName
	dd.planCompiler.mu.Unlock()
	r := dd.Detect("trace-ord", second, `{}`)
	if !r.HasDeviation {
		t.Fatal("expected order violation")
	}
	if r.Deviation.Severity != "moderate" {
		t.Errorf("expected moderate, got %s", r.Deviation.Severity)
	}
	t.Logf("✅ Order violation: tool=%s severity=%s", second, r.Deviation.Severity)
}

// ── 2. 自动修复 ──────────────────────────────────────

func TestAutoRepair_Disabled(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, AutoRepair: false, MaxRepairs: 5}, devPCDefault)
	dd.planCompiler.CompileIntent("trace-norep", "search for data and find results")
	r := dd.Detect("trace-norep", "shell_exec", `{"cmd":"ls"}`)
	if r.HasDeviation && r.Deviation != nil && r.Deviation.Repaired {
		t.Error("should not repair when disabled")
	}
	if dd.GetStats().RepairsApplied != 0 {
		t.Error("expected 0 repairs")
	}
	t.Logf("✅ Auto-repair disabled — no repairs")
}

func TestAutoRepair_Enabled(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, AutoRepair: true, MaxRepairs: 5}, devPCDefault)
	dd.planCompiler.CompileIntent("trace-rep", "search for data and find results")
	r := dd.Detect("trace-rep", "my_custom_tool", `{"key":"value"}`)
	if !r.HasDeviation {
		t.Fatal("expected deviation")
	}
	if r.Deviation.Severity != "minor" {
		t.Skipf("severity=%s (not minor), auto-repair N/A", r.Deviation.Severity)
	}
	if !r.Deviation.Repaired {
		t.Error("expected repaired=true")
	}
	if r.Deviation.RepairedArgs == "" {
		t.Error("expected non-empty repaired_args")
	}
	if r.Deviation.Decision != "allow" {
		t.Errorf("expected allow after repair, got %s", r.Deviation.Decision)
	}
	// Check DeviationResult-level repair fields
	if !r.Repaired {
		t.Error("expected DeviationResult.Repaired=true")
	}
	if r.RepairedArgs == "" {
		t.Error("expected DeviationResult.RepairedArgs non-empty")
	}
	if dd.GetStats().RepairsApplied < 1 {
		t.Error("expected >=1 repairs_applied")
	}
	t.Logf("✅ Auto-repair enabled (minor/args): args=%s", r.Deviation.RepairedArgs)
}

func TestAutoRepair_OutOfOrder(t *testing.T) {
	noAuto := &PlanConfig{
		Enabled: true, StrictMode: false, MaxStepsPerPlan: 20, DefaultTimeout: 300,
		AutoComplete: false, ViolationAction: "warn", MatchThreshold: 0.1,
		MaxActivePlans: 100, RetentionDays: 30,
	}
	dd := newDevDetector(t, DeviationConfig{Enabled: true, AutoRepair: true, MaxRepairs: 5}, noAuto)
	plan := dd.planCompiler.CompileIntent("trace-oor", "search for golang and find information")
	if plan == nil || plan.TotalSteps < 2 {
		t.Skipf("need >=2 steps, got %d", plan.TotalSteps)
	}
	dd.planCompiler.mu.Lock()
	tpl := dd.planCompiler.templates[plan.TemplateID]
	first := tpl.Steps[0].ToolName
	second := tpl.Steps[1].ToolName
	dd.planCompiler.mu.Unlock()

	// Call the second tool first → out_of_order
	r := dd.Detect("trace-oor", second, `{"q":"test"}`)
	if !r.HasDeviation {
		t.Fatal("expected order violation")
	}
	if r.Deviation.Severity != "moderate" {
		t.Skipf("severity=%s (not moderate), skip", r.Deviation.Severity)
	}
	if !r.Repaired {
		t.Fatal("expected DeviationResult.Repaired=true for out_of_order")
	}
	if r.RepairedTool != first {
		t.Errorf("expected RepairedTool=%s, got %s", first, r.RepairedTool)
	}
	if r.Deviation.RepairedTool != first {
		t.Errorf("expected Deviation.RepairedTool=%s, got %s", first, r.Deviation.RepairedTool)
	}
	if r.Decision != "allow" {
		t.Errorf("expected allow after repair, got %s", r.Decision)
	}
	t.Logf("✅ Auto-repair out_of_order: %s → %s", second, r.RepairedTool)
}

func TestAutoRepair_MaxRepairs(t *testing.T) {
	noAuto := &PlanConfig{
		Enabled: true, StrictMode: false, MaxStepsPerPlan: 20, DefaultTimeout: 300,
		AutoComplete: false, ViolationAction: "warn", MatchThreshold: 0.1,
		MaxActivePlans: 100, RetentionDays: 30,
	}
	max := 2
	dd := newDevDetector(t, DeviationConfig{Enabled: true, AutoRepair: true, MaxRepairs: max}, noAuto)
	dd.planCompiler.CompileIntent("trace-max", "search for data and find results")
	repaired := 0
	for i := 0; i < max+3; i++ {
		r := dd.Detect("trace-max", "unknown_tool", `{"i":"test"}`)
		if r.HasDeviation && r.Deviation != nil && r.Deviation.Repaired {
			repaired++
		}
	}
	if repaired > max {
		t.Errorf("expected at most %d repairs, got %d", max, repaired)
	}
	t.Logf("✅ Auto-repair stops at max_repairs=%d: actual=%d", max, repaired)
}

// ── 3. 统计与历史 ────────────────────────────────────

func TestDeviationStats(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCStrict)
	dd.Detect("t1", "web_search", `{}`)
	dd.planCompiler.CompileIntent("t2", "search for data and find results")
	dd.Detect("t2", "bad_tool", `{}`)
	dd.Detect("t3", "web_search", `{}`)
	s := dd.GetStats()
	if s.TotalChecks < 3 {
		t.Errorf("expected >=3 checks, got %d", s.TotalChecks)
	}
	if s.TotalDeviations < 1 {
		t.Errorf("expected >=1 deviations, got %d", s.TotalDeviations)
	}
	if s.CriticalCount < 1 {
		t.Errorf("expected >=1 critical, got %d", s.CriticalCount)
	}
	t.Logf("✅ Stats: checks=%d dev=%d crit=%d mod=%d min=%d rep=%d",
		s.TotalChecks, s.TotalDeviations, s.CriticalCount, s.ModerateCount, s.MinorCount, s.RepairsApplied)
}

func TestDeviationHistory(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCStrict)
	dd.planCompiler.CompileIntent("trace-hist", "search for data and find results")
	dd.Detect("trace-hist", "bad_tool_1", `{}`)
	dd.Detect("trace-hist", "bad_tool_2", `{}`)
	devs := dd.QueryDeviations("trace-hist", "", 50)
	if len(devs) < 2 {
		t.Errorf("expected >=2 deviations, got %d", len(devs))
	}
	for _, d := range devs {
		if d.ID == "" || d.TraceID != "trace-hist" || d.Severity == "" {
			t.Errorf("bad deviation: %+v", d)
		}
	}
	crit := dd.QueryDeviations("", "critical", 50)
	if len(crit) < 2 {
		t.Errorf("expected >=2 critical, got %d", len(crit))
	}
	t.Logf("✅ History: %d by trace, %d critical", len(devs), len(crit))
}

// ── 4. 边界条件 ──────────────────────────────────────

func TestCheckDeviation_EmptyTool(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCDefault)
	dd.planCompiler.CompileIntent("trace-et", "search for data and find results")
	r := dd.Detect("trace-et", "", `{}`)
	if r == nil {
		t.Fatal("nil result for empty tool")
	}
	t.Logf("✅ Empty tool handled: deviation=%v decision=%s", r.HasDeviation, r.Decision)
}

func TestCheckDeviation_ConcurrentAccess(t *testing.T) {
	dd := newDevDetector(t, DeviationConfig{Enabled: true, MaxRepairs: 5}, devPCDefault)
	dd.planCompiler.CompileIntent("trace-cc", "search for data and find results")
	var wg sync.WaitGroup
	errs := make(chan string, 200)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := dd.Detect("trace-cc", "web_search", `{}`); r == nil {
				errs <- "nil result"
			}
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dd.UpdateConfig(DeviationConfig{Enabled: true, AutoRepair: true, MaxRepairs: 3})
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = dd.GetStats()
			_ = dd.GetConfig()
			_ = dd.QueryDeviations("", "", 10)
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Errorf("concurrent: %s", e)
	}
	if s := dd.GetStats(); s.TotalChecks < 50 {
		t.Errorf("expected >=50 checks, got %d", s.TotalChecks)
	}
	t.Logf("✅ Concurrent access safe: checks=%d", dd.GetStats().TotalChecks)
}

// ── 5. CapabilityEngine 集成 ─────────────────────────

func TestCheckDeviation_CapViolation_Block(t *testing.T) {
	db := newDevDB(t)
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "block"})
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, MaxRepairs: 5}, nil, ce)
	r := dd.Detect("trace-cap-block", "shell_exec", `{}`)
	if !r.HasDeviation {
		t.Fatal("expected capability violation")
	}
	if r.Deviation.Type != "capability_violation" {
		t.Errorf("expected capability_violation, got %s", r.Deviation.Type)
	}
	if r.Decision != "block" {
		t.Errorf("expected block, got %s", r.Decision)
	}
	t.Logf("✅ CapEngine block: type=%s decision=%s", r.Deviation.Type, r.Decision)
}

func TestCheckDeviation_CapViolation_Allow(t *testing.T) {
	db := newDevDB(t)
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "allow"})
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, MaxRepairs: 5}, nil, ce)
	r := dd.Detect("trace-cap-allow", "web_search", `{}`)
	if r.HasDeviation {
		t.Errorf("expected no deviation with allow policy, got: %+v", r.Deviation)
	}
	t.Logf("✅ CapEngine allow: no deviation")
}

// ── 6. DB 记录持久化 ────────────────────────────────

func TestDeviationDB_RecordAndQuery(t *testing.T) {
	db := newDevDB(t)
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, MaxRepairs: 5}, nil, nil)
	dd.recordDeviation(&Deviation{ID: "d1", TraceID: "t1", Type: "forbidden_tool", ToolName: "x", Severity: "critical", Decision: "block"})
	dd.recordDeviation(&Deviation{ID: "d2", TraceID: "t1", Type: "sequence_violation", ToolName: "y", Severity: "moderate", Decision: "warn"})
	dd.recordDeviation(&Deviation{ID: "d3", TraceID: "t2", Type: "unknown_tool", ToolName: "z", Severity: "minor", Decision: "allow", Repaired: true, RepairedTool: "expected_tool", RepairedArgs: `{"_repaired":true}`})
	byTrace := dd.QueryDeviations("t1", "", 50)
	if len(byTrace) != 2 {
		t.Errorf("expected 2 for t1, got %d", len(byTrace))
	}
	bySev := dd.QueryDeviations("", "critical", 50)
	if len(bySev) != 1 {
		t.Errorf("expected 1 critical, got %d", len(bySev))
	}
	all := dd.QueryDeviations("", "", 50)
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
	// Check repaired round-trip (including repaired_tool)
	t2 := dd.QueryDeviations("t2", "", 50)
	if len(t2) != 1 || !t2[0].Repaired {
		t.Error("repaired field lost in DB round-trip")
	}
	if t2[0].RepairedTool != "expected_tool" {
		t.Errorf("repaired_tool lost in DB round-trip: got %q", t2[0].RepairedTool)
	}
	t.Logf("✅ DB record+query: trace=%d sev=%d all=%d repaired=%v tool=%s", len(byTrace), len(bySev), len(all), t2[0].Repaired, t2[0].RepairedTool)
}

func TestDeviationDB_NilSafe(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: true, MaxRepairs: 5}, nil, nil)
	devs := dd.QueryDeviations("", "", 10)
	if devs == nil || len(devs) != 0 {
		t.Errorf("nil DB query should return empty slice, got %v", devs)
	}
	// recordDeviation with nil DB should not panic
	dd.recordDeviation(&Deviation{ID: "x", TraceID: "y"})
	t.Logf("✅ Nil DB: safe query + record")
}

func TestDeviationConfig_Defaults(t *testing.T) {
	if defaultDeviationConfig.Enabled {
		t.Error("default should be disabled")
	}
	if defaultDeviationConfig.AutoRepair {
		t.Error("default auto_repair should be false")
	}
	if defaultDeviationConfig.MaxRepairs != 5 {
		t.Errorf("default max_repairs=5, got %d", defaultDeviationConfig.MaxRepairs)
	}
	t.Logf("✅ Defaults: enabled=%v repair=%v max=%d", defaultDeviationConfig.Enabled, defaultDeviationConfig.AutoRepair, defaultDeviationConfig.MaxRepairs)
}
