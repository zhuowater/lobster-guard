// path_policy_test.go — PathPolicyEngine tests
// lobster-guard v23.0
package main

import (
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestPathPolicyDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL")
	if err != nil { t.Fatal(err) }
	// create the tenant boolToInt helper table is not needed, but ensure DB works
	return db
}

func TestPathPolicyEngine_Basic(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	if e == nil { t.Fatal("engine nil") }
	if len(e.ListRules()) < 8 { t.Errorf("expected >= 8 default rules, got %d", len(e.ListRules())) }
	e.RegisterStep("trace-1", PathStep{Stage: "inbound", Action: "inbound_message"})
	ctx := e.GetContext("trace-1")
	if ctx == nil { t.Fatal("context nil") }
	if ctx.TraceID != "trace-1" { t.Errorf("trace_id=%s", ctx.TraceID) }
	if len(ctx.Steps) != 1 { t.Errorf("steps=%d", len(ctx.Steps)) }
}

func TestPathPolicyEngine_SequenceRule(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "seq-test-1"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now()})
	// evaluate send_email right after web_fetch — should be blocked by pp-001
	d := e.Evaluate(traceID, "send_email")
	if d.Decision != "block" { t.Errorf("expected block, got %s", d.Decision) }
	if d.RuleID != "pp-001" { t.Errorf("expected pp-001, got %s", d.RuleID) }
}

func TestPathPolicyEngine_SequenceRuleExpired(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "seq-test-expired"
	// Add a step 60 seconds ago
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now().Add(-60 * time.Second)})
	// pp-001 has 30s window, so should allow
	d := e.Evaluate(traceID, "send_email")
	if d.Decision != "allow" { t.Errorf("expected allow (expired window), got %s", d.Decision) }
}

func TestPathPolicyEngine_CumulativeRule(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "cum-test-1"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	// Add PII taint labels — pp-004 threshold is 3
	e.AddTaintLabel(traceID, "PII-TAINTED")
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "database_query", Details: "PII-TAINTED"})
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "database_query", Details: "PII-TAINTED"})
	d := e.Evaluate(traceID, "any_action")
	if d.Decision != "block" { t.Errorf("expected block (PII >= 3), got %s", d.Decision) }
	if d.RuleID != "pp-004" { t.Errorf("expected pp-004, got %s", d.RuleID) }
}

func TestPathPolicyEngine_DegradationRule(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "deg-test-1"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	// Push risk score above 80 (pp-007 threshold)
	e.UpdateRiskScore(traceID, 85)
	d := e.Evaluate(traceID, "any_action")
	if d.Decision != "block" { t.Errorf("expected block (risk > 80), got %s (%s)", d.Decision, d.Reason) }
}

func TestPathPolicyEngine_RiskScoreDecay(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	e.SetHalfLife(1) // 1 second half-life for fast testing
	traceID := "decay-test"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "shell_exec"}) // +30
	ctx := e.GetContext(traceID)
	if ctx == nil { t.Fatal("nil ctx") }
	initialScore := ctx.RiskScore
	if initialScore < 25 { t.Errorf("expected score >= 25, got %.1f", initialScore) }
	time.Sleep(2 * time.Second) // wait for decay
	ctx2 := e.GetContext(traceID)
	if ctx2.RiskScore >= initialScore { t.Errorf("score should have decayed: %.1f >= %.1f", ctx2.RiskScore, initialScore) }
}

func TestPathPolicyEngine_RiskScoreThresholds(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	// Test warn threshold (>60)
	t1 := "threshold-warn"
	e.RegisterStep(t1, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.UpdateRiskScore(t1, 65)
	d1 := e.Evaluate(t1, "test")
	if d1.Decision != "warn" && d1.Decision != "block" { t.Errorf("expected warn or block at score 65, got %s", d1.Decision) }

	// Test block threshold (>80)
	t2 := "threshold-block"
	e.RegisterStep(t2, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.UpdateRiskScore(t2, 85)
	d2 := e.Evaluate(t2, "test")
	if d2.Decision != "block" { t.Errorf("expected block at score 85, got %s", d2.Decision) }

	// Test isolate threshold (>95)
	t3 := "threshold-isolate"
	e.RegisterStep(t3, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.UpdateRiskScore(t3, 96)
	d3 := e.Evaluate(t3, "test")
	if d3.Decision != "isolate" && d3.Decision != "block" { t.Errorf("expected isolate/block at score 96, got %s", d3.Decision) }
}

func TestPathPolicyEngine_MultipleRules(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "multi-test"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now()})
	// Push risk above 80 AND trigger sequence rule
	e.UpdateRiskScore(traceID, 85)
	d := e.Evaluate(traceID, "send_email")
	// Both sequence (block) and degradation (block) should trigger — strictest wins
	if d.Decision != "block" && d.Decision != "isolate" { t.Errorf("expected block/isolate, got %s", d.Decision) }
}

func TestPathPolicyEngine_RuleCRUD(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	initial := len(e.ListRules())

	// Add
	err := e.AddRule(PathPolicyRule{ID: "test-001", Name: "test_rule", RuleType: "sequence",
		Conditions: `{"after":"a","before":"b","window_sec":10}`, Action: "warn", Enabled: true, Priority: 50})
	if err != nil { t.Fatalf("add: %v", err) }
	if len(e.ListRules()) != initial+1 { t.Errorf("expected %d rules, got %d", initial+1, len(e.ListRules())) }

	// Duplicate
	err = e.AddRule(PathPolicyRule{ID: "test-001", Name: "dup", RuleType: "sequence", Conditions: "{}"})
	if err == nil { t.Error("expected duplicate error") }

	// Update
	err = e.UpdateRule(PathPolicyRule{ID: "test-001", Name: "updated_name", Action: "block"})
	if err != nil { t.Fatalf("update: %v", err) }
	r := e.GetRule("test-001")
	if r == nil { t.Fatal("rule not found") }
	if r.Name != "updated_name" { t.Errorf("name=%s", r.Name) }
	if r.Action != "block" { t.Errorf("action=%s", r.Action) }

	// Delete
	err = e.DeleteRule("test-001")
	if err != nil { t.Fatalf("delete: %v", err) }
	if len(e.ListRules()) != initial { t.Errorf("expected %d, got %d", initial, len(e.ListRules())) }

	// Delete nonexistent
	err = e.DeleteRule("nonexistent")
	if err == nil { t.Error("expected error") }
}

func TestPathPolicyEngine_SessionIntegration(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "session-int-1"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.SetSessionID(traceID, "sess-123")
	ctx := e.GetContext(traceID)
	if ctx.SessionID != "sess-123" { t.Errorf("session_id=%s", ctx.SessionID) }
}

func TestPathPolicyEngine_TaintIntegration(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "taint-int-1"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.AddTaintLabel(traceID, "PII-TAINTED")
	e.AddTaintLabel(traceID, "PII-TAINTED") // duplicate, should not add
	ctx := e.GetContext(traceID)
	if len(ctx.TaintLabels) != 1 { t.Errorf("taint_labels=%d", len(ctx.TaintLabels)) }
	if ctx.TaintLabels[0] != "PII-TAINTED" { t.Errorf("label=%s", ctx.TaintLabels[0]) }
}

func TestPathPolicyEngine_DefaultRules(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	rules := e.ListRules()
	if len(rules) < 8 { t.Errorf("expected >= 8 default rules, got %d", len(rules)) }
	// Check specific rule IDs exist
	ids := map[string]bool{}
	for _, r := range rules { ids[r.ID] = true }
	for _, id := range []string{"pp-001", "pp-002", "pp-003", "pp-004", "pp-005", "pp-006", "pp-007", "pp-008"} {
		if !ids[id] { t.Errorf("missing default rule %s", id) }
	}
}

func TestPathPolicyEngine_RuleDisable(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	// Disable pp-001
	err := e.SetRuleEnabled("pp-001", false)
	if err != nil { t.Fatalf("disable: %v", err) }
	r := e.GetRule("pp-001")
	if r.Enabled { t.Error("should be disabled") }

	// Sequence should now allow
	traceID := "disable-test"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now()})
	d := e.Evaluate(traceID, "send_email")
	if d.Decision == "block" && d.RuleID == "pp-001" { t.Error("disabled rule should not trigger") }

	// Re-enable
	e.SetRuleEnabled("pp-001", true)
}

func TestPathPolicyEngine_Tenant(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	// Add tenant-specific rule
	e.AddRule(PathPolicyRule{ID: "t-001", Name: "tenant_rule", RuleType: "degradation",
		Conditions: `{"risk_threshold":50,"degrade_to":"warn"}`, Action: "warn", Enabled: true, Priority: 50, TenantID: "tenant-a"})

	all := e.ListRules()
	tenantA := e.ListRulesByTenant("tenant-a")
	if len(tenantA) != 1 { t.Errorf("expected 1 tenant-a rule, got %d", len(tenantA)) }
	if len(all) <= len(tenantA) { t.Error("all rules should be more than tenant-a rules") }

	// Set tenant on context
	traceID := "tenant-ctx"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.SetTenantID(traceID, "tenant-a")
	ctx := e.GetContext(traceID)
	if ctx.TenantID != "tenant-a" { t.Errorf("tenant=%s", ctx.TenantID) }
}

func TestPathPolicyEngine_Concurrency(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tid := "concurrent-" + string(rune('A'+idx%26))
			e.RegisterStep(tid, PathStep{Stage: "tool_call", Action: "web_fetch"})
			e.Evaluate(tid, "send_email")
			e.GetContext(tid)
			e.UpdateRiskScore(tid, 5)
		}(i)
	}
	wg.Wait()
	// no panic = pass
	if e.ContextCount() == 0 { t.Error("expected some contexts") }
}

func TestPathPolicyEngine_Eviction(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	e.SetEvictAfter(1 * time.Millisecond)
	e.RegisterStep("evict-1", PathStep{Stage: "inbound", Action: "inbound_message"})
	time.Sleep(10 * time.Millisecond)
	n := e.EvictExpired()
	if n != 1 { t.Errorf("expected 1 evicted, got %d", n) }
	if e.GetContext("evict-1") != nil { t.Error("context should have been evicted") }
}

func TestPathPolicyEngine_Stats(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	e.RegisterStep("stat-1", PathStep{Stage: "inbound", Action: "inbound_message"})
	e.RegisterStep("stat-2", PathStep{Stage: "tool_call", Action: "web_fetch"})
	stats := e.Stats()
	if stats.ActiveContexts != 2 { t.Errorf("active=%d", stats.ActiveContexts) }
	if stats.TotalRules < 8 { t.Errorf("rules=%d", stats.TotalRules) }
	if stats.EnabledRules < 8 { t.Errorf("enabled=%d", stats.EnabledRules) }
}

func TestPathPolicyEngine_EmptyPath(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	// Evaluate on nonexistent trace
	d := e.Evaluate("nonexistent", "any_action")
	if d.Decision != "allow" { t.Errorf("expected allow, got %s", d.Decision) }

	// Register with empty trace — should be no-op
	e.RegisterStep("", PathStep{Stage: "inbound", Action: "test"})
	if e.ContextCount() != 0 { t.Error("empty trace should not create context") }
}

func TestPathPolicyEngine_RiskWeights(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	// Default weight for shell_exec = 30
	w := e.GetRiskWeight("shell_exec")
	if w != 30 { t.Errorf("expected 30, got %.1f", w) }

	// Custom weight
	e.SetRiskWeight("custom_action", 99)
	if e.GetRiskWeight("custom_action") != 99 { t.Error("custom weight not set") }

	// RegisterStep with custom action should use custom weight
	e.RegisterStep("weight-test", PathStep{Stage: "tool_call", Action: "custom_action"})
	ctx := e.GetContext("weight-test")
	if ctx.RiskScore < 90 { t.Errorf("expected score >= 90, got %.1f", ctx.RiskScore) }
}

func TestPathPolicyEngine_DBPersistence(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	// First engine
	e1 := NewPathPolicyEngine(db)
	e1.AddRule(PathPolicyRule{ID: "persist-1", Name: "persistent_rule", RuleType: "sequence",
		Conditions: `{"after":"a","before":"b","window_sec":10}`, Action: "block", Enabled: true, Priority: 50})
	count1 := len(e1.ListRules())

	// Second engine from same DB — should recover rules
	e2 := NewPathPolicyEngine(db)
	count2 := len(e2.ListRules())
	if count2 != count1 { t.Errorf("persistence: expected %d rules, got %d", count1, count2) }
	r := e2.GetRule("persist-1")
	if r == nil { t.Fatal("persisted rule not found") }
	if r.Name != "persistent_rule" { t.Errorf("name=%s", r.Name) }
}

func TestPathPolicyEngine_EventLogging(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "event-log-test"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now()})
	d := e.Evaluate(traceID, "send_email")
	if d.Decision != "block" { t.Fatalf("expected block, got %s", d.Decision) }

	events, err := e.QueryEvents(traceID, "", "", 10)
	if err != nil { t.Fatal(err) }
	if len(events) == 0 { t.Error("expected at least 1 event") }
	if events[0]["trace_id"] != traceID { t.Errorf("trace_id=%v", events[0]["trace_id"]) }
	if events[0]["decision"] != "block" { t.Errorf("decision=%v", events[0]["decision"]) }
}

func TestPathPolicyEngine_CredentialCumulative(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "cred-cum-test"
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
	e.AddTaintLabel(traceID, "CREDENTIAL-TAINTED")
	d := e.Evaluate(traceID, "any")
	if d.Decision != "block" { t.Errorf("expected block for credential, got %s", d.Decision) }
	if d.RuleID != "pp-005" { t.Errorf("expected pp-005, got %s", d.RuleID) }
}

func TestPathPolicyEngine_ToolHistory(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "tool-hist"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch"})
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "file_read"})
	e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"}) // not tool_call
	ctx := e.GetContext(traceID)
	if len(ctx.ToolHistory) != 2 { t.Errorf("expected 2 tools, got %d", len(ctx.ToolHistory)) }
}

func TestPathPolicyEngine_JSONConditions(t *testing.T) {
	// Test that conditions unmarshal correctly
	var sc SequenceCondition
	json.Unmarshal([]byte(`{"after":"web_fetch","before":"send_email","window_sec":30}`), &sc)
	if sc.After != "web_fetch" || sc.Before != "send_email" || sc.WindowSec != 30 {
		t.Errorf("sequence: %+v", sc)
	}
	var cc CumulativeCondition
	json.Unmarshal([]byte(`{"label":"PII-TAINTED","threshold":3}`), &cc)
	if cc.Label != "PII-TAINTED" || cc.Threshold != 3 { t.Errorf("cumulative: %+v", cc) }
	var dc DegradationCondition
	json.Unmarshal([]byte(`{"risk_threshold":60,"degrade_to":"warn"}`), &dc)
	if dc.RiskThreshold != 60 || dc.DegradeTo != "warn" { t.Errorf("degradation: %+v", dc) }
}

func TestPathPolicyEngine_ShellSequence(t *testing.T) {
	db := setupTestPathPolicyDB(t)
	defer db.Close()
	e := NewPathPolicyEngine(db)
	traceID := "shell-seq"
	e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "web_fetch", Timestamp: time.Now()})
	d := e.Evaluate(traceID, "shell_exec")
	if d.Decision != "block" { t.Errorf("expected block (pp-002), got %s", d.Decision) }
}
