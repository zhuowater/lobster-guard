package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupDevTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil { t.Fatal(err) }
	return db
}

func TestDeviation_Basic(t *testing.T) {
	db := setupDevTestDB(t)
	defer db.Close()
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true, MaxRepairs: 5}, nil, nil)
	if dd == nil { t.Fatal("detector should not be nil") }
}

func TestDeviation_Disabled(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: false}, nil, nil)
	r := dd.Detect("trace-1", "shell_exec", "{}")
	if r.Decision != "allow" { t.Errorf("expected allow when disabled, got %s", r.Decision) }
}

func TestDeviation_NoPlanNoCap(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: true}, nil, nil)
	r := dd.Detect("trace-1", "shell_exec", "{}")
	if r.HasDeviation { t.Error("no deviation expected without plan or cap engine") }
}

func TestDeviation_WithPlanCompiler(t *testing.T) {
	db := setupDevTestDB(t)
	defer db.Close()
	pc := NewPlanCompiler(db, PlanConfig{Enabled: true, ViolationAction: "block", StrictMode: true})
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true}, pc, nil)

	// No plan compiled → strict mode in plan compiler may block
	r := dd.Detect("trace-1", "unknown_tool", "{}")
	// Just check it doesn't panic
	_ = r
}

func TestDeviation_Stats(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: true}, nil, nil)
	dd.Detect("t1", "a", "{}")
	dd.Detect("t2", "b", "{}")
	s := dd.GetStats()
	if s.TotalChecks != 2 { t.Errorf("expected 2 checks, got %d", s.TotalChecks) }
}

func TestDeviation_Config(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: true, AutoRepair: false, MaxRepairs: 3}, nil, nil)
	cfg := dd.GetConfig()
	if !cfg.Enabled { t.Error("should be enabled") }
	if cfg.AutoRepair { t.Error("auto_repair should be false") }
	dd.UpdateConfig(DeviationConfig{Enabled: false, AutoRepair: true, MaxRepairs: 10})
	cfg2 := dd.GetConfig()
	if cfg2.Enabled { t.Error("should be disabled after update") }
	if !cfg2.AutoRepair { t.Error("auto_repair should be true after update") }
}

func TestDeviation_QueryEmpty(t *testing.T) {
	db := setupDevTestDB(t)
	defer db.Close()
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true}, nil, nil)
	devs := dd.QueryDeviations("", "", 10)
	if len(devs) != 0 { t.Errorf("expected 0 deviations, got %d", len(devs)) }
}

func TestDeviation_RecordAndQuery(t *testing.T) {
	db := setupDevTestDB(t)
	defer db.Close()
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true}, nil, nil)
	dd.recordDeviation(&Deviation{
		ID: "dev-test1", TraceID: "trace-1", Type: "forbidden_tool",
		ToolName: "shell_exec", Severity: "critical", Decision: "block",
	})
	devs := dd.QueryDeviations("trace-1", "", 10)
	if len(devs) != 1 { t.Fatalf("expected 1 deviation, got %d", len(devs)) }
	if devs[0].Type != "forbidden_tool" { t.Error("wrong type") }
}

func TestDeviation_QueryBySeverity(t *testing.T) {
	db := setupDevTestDB(t)
	defer db.Close()
	dd := NewDeviationDetector(db, DeviationConfig{Enabled: true}, nil, nil)
	dd.recordDeviation(&Deviation{ID: "d1", TraceID: "t1", Severity: "critical", Decision: "block"})
	dd.recordDeviation(&Deviation{ID: "d2", TraceID: "t2", Severity: "minor", Decision: "allow"})
	critical := dd.QueryDeviations("", "critical", 10)
	if len(critical) != 1 { t.Errorf("expected 1 critical, got %d", len(critical)) }
}

func TestDeviation_NilDB(t *testing.T) {
	dd := NewDeviationDetector(nil, DeviationConfig{Enabled: true}, nil, nil)
	devs := dd.QueryDeviations("", "", 10)
	if len(devs) != 0 { t.Errorf("expected empty with nil db") }
}

func TestDeviation_DefaultConfig(t *testing.T) {
	if defaultDeviationConfig.MaxRepairs != 5 { t.Error("default max_repairs should be 5") }
	if defaultDeviationConfig.Enabled { t.Error("default should be disabled") }
}
