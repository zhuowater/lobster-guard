// ifc_test.go — IFC engine tests (v26.0)
package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestIFCDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestIFCLabelPropagation(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "warn"})

	traceID := "trace-prop-1"

	// Register variables with explicit labels via source rules
	e.mu.Lock()
	e.sourceRules["src_public_high"] = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	e.sourceRules["src_secret_taint"] = IFCLabel{Confidentiality: ConfSecret, Integrity: IntegTaint}
	e.mu.Unlock()

	a := e.RegisterVariable(traceID, "var_a", "src_public_high", "data a")
	b := e.RegisterVariable(traceID, "var_b", "src_secret_taint", "data b")

	// Propagate: conf=max(PUBLIC,SECRET)=SECRET, integ=min(HIGH,TAINT)=TAINT
	c := e.Propagate(traceID, "var_c", []string{a.ID, b.ID})

	if c.Label.Confidentiality != ConfSecret {
		t.Errorf("expected conf=SECRET, got %s", c.Label.Confidentiality)
	}
	if c.Label.Integrity != IntegTaint {
		t.Errorf("expected integ=TAINT, got %s", c.Label.Integrity)
	}
	if len(c.Parents) != 2 {
		t.Errorf("expected 2 parents, got %d", len(c.Parents))
	}
}

func TestIFCConfidentialityViolation(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "warn"})

	traceID := "trace-conf-viol"

	// Register a SECRET variable
	e.mu.Lock()
	e.sourceRules["secret_src"] = IFCLabel{Confidentiality: ConfSecret, Integrity: IntegHigh}
	e.mu.Unlock()

	v := e.RegisterVariable(traceID, "secret_data", "secret_src", "top secret info")

	// send_email max_conf=INTERNAL → SECRET > INTERNAL → violation
	decision := e.CheckToolCall(traceID, "send_email", []string{v.ID})

	if decision.Allowed {
		t.Error("expected decision to NOT be allowed (warn)")
	}
	if decision.Decision != "warn" {
		t.Errorf("expected decision=warn, got %s", decision.Decision)
	}
	if decision.Violation == nil {
		t.Fatal("expected violation, got nil")
	}
	if decision.Violation.Type != "confidentiality" {
		t.Errorf("expected violation type=confidentiality, got %s", decision.Violation.Type)
	}
}

func TestIFCIntegrityViolation(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "block"})

	traceID := "trace-integ-viol"

	// Register a TAINT variable (e.g., from web_fetch)
	v := e.RegisterVariable(traceID, "web_data", "tool:web_fetch", "untrusted data")

	// shell_exec required_integ=HIGH → TAINT < HIGH → violation
	decision := e.CheckToolCall(traceID, "shell_exec", []string{v.ID})

	if decision.Allowed {
		t.Error("expected decision to NOT be allowed (block)")
	}
	if decision.Decision != "block" {
		t.Errorf("expected decision=block, got %s", decision.Decision)
	}
	if decision.Violation == nil {
		t.Fatal("expected violation, got nil")
	}
	if decision.Violation.Type != "integrity" {
		t.Errorf("expected violation type=integrity, got %s", decision.Violation.Type)
	}
}

func TestIFCAllowedToolCall(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "block"})

	traceID := "trace-allowed"

	// Register a PUBLIC + HIGH variable
	e.mu.Lock()
	e.sourceRules["safe_src"] = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	e.mu.Unlock()

	v := e.RegisterVariable(traceID, "safe_data", "safe_src", "public data")

	// web_fetch required_integ=LOW, max_conf=PUBLIC → PUBLIC<=PUBLIC && HIGH>=LOW → allowed
	decision := e.CheckToolCall(traceID, "web_fetch", []string{v.ID})

	if !decision.Allowed {
		t.Errorf("expected allowed=true, got false: %s", decision.Reason)
	}
	if decision.Decision != "allow" {
		t.Errorf("expected decision=allow, got %s", decision.Decision)
	}
}

func TestIFCMixedInputs(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "warn"})

	traceID := "trace-mixed"

	// Multiple inputs with different labels
	e.mu.Lock()
	e.sourceRules["src_public_high"] = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	e.sourceRules["src_internal_medium"] = IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium}
	e.sourceRules["src_confidential_low"] = IFCLabel{Confidentiality: ConfConfidential, Integrity: IntegLow}
	e.mu.Unlock()

	v1 := e.RegisterVariable(traceID, "v1", "src_public_high", "a")
	v2 := e.RegisterVariable(traceID, "v2", "src_internal_medium", "b")
	v3 := e.RegisterVariable(traceID, "v3", "src_confidential_low", "c")

	// Aggregated: conf=max(0,1,2)=CONFIDENTIAL, integ=min(3,2,1)=LOW
	decision := e.CheckToolCall(traceID, "file_write", []string{v1.ID, v2.ID, v3.ID})
	// file_write: required_integ=MEDIUM, max_conf=CONFIDENTIAL
	// integ=LOW < MEDIUM → integrity violation
	if decision.Violation == nil {
		t.Fatal("expected violation for mixed inputs")
	}
	if decision.Violation.Type != "integrity" {
		t.Errorf("expected integrity violation, got %s", decision.Violation.Type)
	}
}

func TestIFCSourceRuleCRUD(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	// Add
	err := e.AddSourceRule(IFCSourceRule{Source: "custom_src", Label: IFCLabel{Confidentiality: ConfSecret, Integrity: IntegHigh}})
	if err != nil {
		t.Fatalf("AddSourceRule failed: %v", err)
	}

	// Duplicate add should fail
	err = e.AddSourceRule(IFCSourceRule{Source: "custom_src", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegLow}})
	if err == nil {
		t.Error("expected error on duplicate add")
	}

	// Update
	err = e.UpdateSourceRule("custom_src", IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium})
	if err != nil {
		t.Fatalf("UpdateSourceRule failed: %v", err)
	}

	// Verify update
	rules := e.ListSourceRules()
	found := false
	for _, r := range rules {
		if r.Source == "custom_src" {
			if r.Label.Confidentiality != ConfInternal {
				t.Errorf("expected INTERNAL, got %s", r.Label.Confidentiality)
			}
			found = true
		}
	}
	if !found {
		t.Error("custom_src not found after update")
	}

	// Delete
	err = e.DeleteSourceRule("custom_src")
	if err != nil {
		t.Fatalf("DeleteSourceRule failed: %v", err)
	}

	// Delete non-existent should fail
	err = e.DeleteSourceRule("non_existent")
	if err == nil {
		t.Error("expected error on deleting non-existent rule")
	}
}

func TestIFCToolRequirementCRUD(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	// Add
	err := e.AddToolRequirement(IFCToolRequirement{Tool: "custom_tool", RequiredInteg: IntegHigh, MaxConf: ConfSecret})
	if err != nil {
		t.Fatalf("AddToolRequirement failed: %v", err)
	}

	// Duplicate add should fail
	err = e.AddToolRequirement(IFCToolRequirement{Tool: "custom_tool", RequiredInteg: IntegLow, MaxConf: ConfPublic})
	if err == nil {
		t.Error("expected error on duplicate add")
	}

	// Update
	err = e.UpdateToolRequirement("custom_tool", IFCToolRequirement{RequiredInteg: IntegMedium, MaxConf: ConfInternal})
	if err != nil {
		t.Fatalf("UpdateToolRequirement failed: %v", err)
	}

	// Verify
	reqs := e.ListToolRequirements()
	found := false
	for _, r := range reqs {
		if r.Tool == "custom_tool" {
			if r.RequiredInteg != IntegMedium {
				t.Errorf("expected MEDIUM, got %s", r.RequiredInteg)
			}
			found = true
		}
	}
	if !found {
		t.Error("custom_tool not found after update")
	}

	// Delete
	err = e.DeleteToolRequirement("custom_tool")
	if err != nil {
		t.Fatalf("DeleteToolRequirement failed: %v", err)
	}

	err = e.DeleteToolRequirement("non_existent")
	if err == nil {
		t.Error("expected error on deleting non-existent tool")
	}
}

func TestIFCHideContent(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, HidingEnabled: true, HidingThreshold: ConfConfidential})

	// Text with phone number and ID card
	text := "张三的手机号是13800138000，身份证号320106199001011234"
	result := e.HideContent("trace-hide", text, ConfConfidential)

	if result.HiddenCount == 0 {
		t.Error("expected some hidden fields")
	}
	if result.Redacted == text {
		t.Error("expected redacted text to differ from original")
	}
	if len(result.HiddenFields) == 0 {
		t.Error("expected hidden field names")
	}
}

func TestIFCDOEDetection(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	template := &PlanTemplate{
		Steps: []PlanStep{
			{ToolName: "send_email", AllowedArgs: []string{"to", "subject", "body"}},
		},
	}

	// Expose more fields than needed
	result := e.DetectDOE("trace-doe", "send_email", []string{"to", "subject", "body", "cc", "bcc", "password", "secret_key"}, template)

	if len(result.ExcessFields) != 4 {
		t.Errorf("expected 4 excess fields, got %d: %v", len(result.ExcessFields), result.ExcessFields)
	}
	if result.Severity != "critical" {
		t.Errorf("expected severity=critical, got %s", result.Severity)
	}
}

func TestIFCShouldQuarantine(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	// Quarantine enabled
	e := NewIFCEngine(db, IFCConfig{Enabled: true, QuarantineEnabled: true})

	traceID := "trace-quarantine"

	// Register a TAINT variable
	v := e.RegisterVariable(traceID, "tainted_data", "tool:web_fetch", "untrusted")

	if !e.ShouldQuarantine(traceID, []string{v.ID}) {
		t.Error("expected quarantine=true for TAINT variable")
	}

	// Non-tainted variable
	e.mu.Lock()
	e.sourceRules["safe"] = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	e.mu.Unlock()

	v2 := e.RegisterVariable(traceID, "safe_data", "safe", "trusted")

	if e.ShouldQuarantine(traceID, []string{v2.ID}) {
		t.Error("expected quarantine=false for HIGH integrity variable")
	}
}

func TestIFCStatsFromDB(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	// Create engine, register variables and create violations
	e1 := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "block"})

	traceID := "trace-stats"
	e1.RegisterVariable(traceID, "v1", "tool:web_fetch", "data1")
	v2 := e1.RegisterVariable(traceID, "v2", "tool:web_fetch", "data2")

	// Create a violation
	e1.CheckToolCall(traceID, "shell_exec", []string{v2.ID})

	// Create a NEW engine from the same DB — stats should restore
	e2 := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "block"})

	stats := e2.GetStats()
	if stats.TotalVariables < 2 {
		t.Errorf("expected TotalVariables >= 2, got %d", stats.TotalVariables)
	}
	if stats.TotalViolations < 1 {
		t.Errorf("expected TotalViolations >= 1, got %d", stats.TotalViolations)
	}
}

func TestIFCVariableChain(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	traceID := "trace-chain"

	a := e.RegisterVariable(traceID, "a", "user_input", "data a")
	b := e.Propagate(traceID, "b", []string{a.ID})
	c := e.Propagate(traceID, "c", []string{b.ID})

	// Verify parents chain
	if len(b.Parents) != 1 || b.Parents[0] != a.ID {
		t.Errorf("b.Parents expected [%s], got %v", a.ID, b.Parents)
	}
	if len(c.Parents) != 1 || c.Parents[0] != b.ID {
		t.Errorf("c.Parents expected [%s], got %v", b.ID, c.Parents)
	}
}

func TestIFCDecisionActions(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	// Test block action
	e1 := NewIFCEngine(db, IFCConfig{Enabled: true, ViolationAction: "block"})
	traceID := "trace-actions-1"
	v1 := e1.RegisterVariable(traceID, "web_data", "tool:web_fetch", "untrusted")
	d1 := e1.CheckToolCall(traceID, "shell_exec", []string{v1.ID})

	if d1.Decision != "block" {
		t.Errorf("expected decision=block, got %s", d1.Decision)
	}
	if d1.Allowed {
		t.Error("expected allowed=false for block")
	}

	// Test warn action with fresh DB
	db2 := newTestIFCDB(t)
	defer db2.Close()
	e2 := NewIFCEngine(db2, IFCConfig{Enabled: true, ViolationAction: "warn"})
	traceID2 := "trace-actions-2"
	v2 := e2.RegisterVariable(traceID2, "web_data", "tool:web_fetch", "untrusted")
	d2 := e2.CheckToolCall(traceID2, "shell_exec", []string{v2.ID})

	if d2.Decision != "warn" {
		t.Errorf("expected decision=warn, got %s", d2.Decision)
	}
	// warn means not allowed (by our convention)
	if d2.Allowed {
		t.Error("expected allowed=false for warn")
	}
}

func TestIFCDefaultSourceRules(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	// Empty config → should load 6 default source rules
	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	rules := e.ListSourceRules()
	if len(rules) != 6 {
		t.Errorf("expected 6 default source rules, got %d", len(rules))
		for _, r := range rules {
			t.Logf("  %s → conf=%s integ=%s", r.Source, r.Label.Confidentiality, r.Label.Integrity)
		}
	}
}

func TestIFCDefaultToolRequirements(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	// Empty config → should load 5 default tool requirements
	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	reqs := e.ListToolRequirements()
	if len(reqs) != 5 {
		t.Errorf("expected 5 default tool requirements, got %d", len(reqs))
		for _, r := range reqs {
			t.Logf("  %s → required_integ=%s max_conf=%s", r.Tool, r.RequiredInteg, r.MaxConf)
		}
	}
}
