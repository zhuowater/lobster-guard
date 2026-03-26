// ifc_test.go — IFC engine tests (v26.0 + v26.1 + v26.2)
package main

import (
	"database/sql"
	"sort"
	"strings"
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

// ============================================================
// v26.1: Quarantine Tests
// ============================================================

func TestIFCQuarantineShouldRoute(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, QuarantineEnabled: true, QuarantineUpstream: "http://quarantine:8080"})
	q := NewIFCQuarantine(e, nil)

	traceID := "trace-qsr-1"

	// Register a TAINT variable (from web_fetch)
	v1 := e.RegisterVariable(traceID, "tainted_data", "tool:web_fetch", "untrusted data")

	// ShouldRoute should be true for TAINT
	if !q.ShouldRoute(traceID, []string{v1.ID}) {
		t.Error("expected ShouldRoute=true for TAINT variable")
	}

	// Register a HIGH integrity variable
	e.mu.Lock()
	e.sourceRules["safe_src"] = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	e.mu.Unlock()
	v2 := e.RegisterVariable(traceID, "safe_data", "safe_src", "trusted data")

	// ShouldRoute should be false for HIGH
	if q.ShouldRoute(traceID, []string{v2.ID}) {
		t.Error("expected ShouldRoute=false for HIGH integrity variable")
	}

	// Mixed: one TAINT + one HIGH → should route (any TAINT triggers)
	if !q.ShouldRoute(traceID, []string{v1.ID, v2.ID}) {
		t.Error("expected ShouldRoute=true when any input is TAINT")
	}
}

func TestIFCQuarantineRoute(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, QuarantineEnabled: true, QuarantineUpstream: "http://quarantine-llm:8080"})
	q := NewIFCQuarantine(e, nil)

	traceID := "trace-qr-1"

	// Register TAINT variable
	v1 := e.RegisterVariable(traceID, "tainted_data", "tool:web_fetch", "untrusted")

	// Route
	upstreamURL, sessionID, err := q.Route(traceID, []string{v1.ID})
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if upstreamURL != "http://quarantine-llm:8080" {
		t.Errorf("expected upstream=http://quarantine-llm:8080, got %s", upstreamURL)
	}
	if sessionID == "" {
		t.Error("expected non-empty sessionID")
	}

	// Verify session exists
	sessions := q.GetSessions(10)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != "processing" {
		t.Errorf("expected status=processing, got %s", sessions[0].Status)
	}

	// CompleteSession → verify depurified variable has integ=MEDIUM
	outputVar := q.CompleteSession(traceID, sessionID, "cleaned output")
	if outputVar == nil {
		t.Fatal("CompleteSession returned nil")
	}
	if outputVar.Label.Integrity != IntegMedium {
		t.Errorf("expected depurified integ=MEDIUM, got %s", outputVar.Label.Integrity)
	}
	if outputVar.Source != "quarantine" {
		t.Errorf("expected source=quarantine, got %s", outputVar.Source)
	}

	// Verify session is completed
	sessions = q.GetSessions(10)
	found := false
	for _, s := range sessions {
		if s.SessionID == sessionID && s.Status == "completed" {
			found = true
			if s.OutputVar != outputVar.ID {
				t.Errorf("expected output_var=%s, got %s", outputVar.ID, s.OutputVar)
			}
		}
	}
	if !found {
		t.Error("session not found as completed")
	}
}

func TestIFCQuarantineStats(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, QuarantineEnabled: true, QuarantineUpstream: "http://quarantine:8080"})
	q := NewIFCQuarantine(e, nil)

	traceID := "trace-qs-1"
	v := e.RegisterVariable(traceID, "data", "tool:web_fetch", "untrusted")

	// Route 3 times
	_, sid1, _ := q.Route(traceID, []string{v.ID})
	_, sid2, _ := q.Route(traceID, []string{v.ID})
	_, sid3, _ := q.Route(traceID, []string{v.ID})

	stats := q.GetStats()
	if stats.TotalRouted != 3 {
		t.Errorf("expected TotalRouted=3, got %d", stats.TotalRouted)
	}
	if stats.ActiveSessions != 3 {
		t.Errorf("expected ActiveSessions=3, got %d", stats.ActiveSessions)
	}

	// Complete 2 sessions
	q.CompleteSession(traceID, sid1, "output1")
	q.CompleteSession(traceID, sid2, "output2")

	// Fail 1 session
	q.FailSession(sid3)

	stats = q.GetStats()
	if stats.TotalDepurified != 2 {
		t.Errorf("expected TotalDepurified=2, got %d", stats.TotalDepurified)
	}
	if stats.TotalFailed != 1 {
		t.Errorf("expected TotalFailed=1, got %d", stats.TotalFailed)
	}
	if stats.ActiveSessions != 0 {
		t.Errorf("expected ActiveSessions=0, got %d", stats.ActiveSessions)
	}
}

// ============================================================
// v26.2: Hiding + DOE Proxy Tests
// ============================================================

func TestIFCHidingProxy(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true, HidingEnabled: true, HidingThreshold: ConfConfidential})

	// Text with PII
	text := "用户张三手机号13900001234，身份证号码320101199001012345，请处理"
	result := e.HideContent("trace-hiding-proxy", text, ConfConfidential)

	if result.HiddenCount == 0 {
		t.Error("expected hidden count > 0")
	}
	if strings.Contains(result.Redacted, "13900001234") {
		t.Error("expected phone number to be redacted")
	}
	if strings.Contains(result.Redacted, "320101199001012345") {
		t.Error("expected ID card to be redacted")
	}
	if !strings.Contains(result.Redacted, "[REDACTED:conf=CONFIDENTIAL]") {
		t.Error("expected [REDACTED:conf=CONFIDENTIAL] in output")
	}
	if len(result.HiddenFields) == 0 {
		t.Error("expected hidden field names")
	}
}

func TestIFCDOEProxy(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()

	e := NewIFCEngine(db, IFCConfig{Enabled: true})

	// With plan template defining required fields
	template := &PlanTemplate{
		Steps: []PlanStep{
			{ToolName: "send_email", AllowedArgs: []string{"to", "subject"}},
		},
	}

	// Expose excess fields
	result := e.DetectDOE("trace-doe-proxy", "send_email", []string{"to", "subject", "password", "secret_key", "cc", "bcc"}, template)

	if result.Severity != "critical" {
		t.Errorf("expected severity=critical, got %s", result.Severity)
	}
	if len(result.ExcessFields) != 4 {
		t.Errorf("expected 4 excess fields, got %d: %v", len(result.ExcessFields), result.ExcessFields)
	}

	// Exact fields → no excess
	result2 := e.DetectDOE("trace-doe-proxy-2", "send_email", []string{"to", "subject"}, template)
	if result2.Severity != "info" {
		t.Errorf("expected severity=info for exact fields, got %s", result2.Severity)
	}

	// Warning level (1-3 excess)
	result3 := e.DetectDOE("trace-doe-proxy-3", "send_email", []string{"to", "subject", "cc"}, template)
	if result3.Severity != "warning" {
		t.Errorf("expected severity=warning, got %s", result3.Severity)
	}
}

func TestExtractFieldNames(t *testing.T) {
	// Valid JSON
	keys := extractFieldNames(`{"name":"张三","phone":"13800138000","age":30}`)
	sort.Strings(keys)
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
	expected := []string{"age", "name", "phone"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("key[%d] expected %s, got %s", i, expected[i], k)
		}
	}

	// Invalid JSON → nil
	keys2 := extractFieldNames("not json")
	if keys2 != nil {
		t.Errorf("expected nil for invalid JSON, got %v", keys2)
	}

	// Empty object
	keys3 := extractFieldNames("{}")
	if len(keys3) != 0 {
		t.Errorf("expected 0 keys for empty object, got %d", len(keys3))
	}

	// Array (not object) → nil
	keys4 := extractFieldNames(`[1,2,3]`)
	if keys4 != nil {
		t.Errorf("expected nil for array JSON, got %v", keys4)
	}
}

// ============================================================
// Fides-aligned IFC Tests (v28.0h)
// ============================================================

func TestIFC_PropagateWithTool(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()
	e := NewIFCEngine(db, IFCConfig{
		Enabled: true,
		SourceRules: []IFCSourceRule{
			{Source: "tool:web_fetch", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegTaint}},
			{Source: "user_input", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegHigh}},
		},
	})
	traceID := "test-fides-prop"

	// Register a trusted user input variable
	userVar := e.RegisterVariable(traceID, "user_query", "user_input", "hello")

	// Propagate WITHOUT tool label — should inherit from user_input
	v1 := e.Propagate(traceID, "output1", []string{userVar.ID})
	if v1.Label.Integrity != IntegHigh {
		t.Errorf("without tool: expected integrity HIGH, got %v", v1.Label.Integrity)
	}

	// Propagate WITH web_fetch tool label — should downgrade integrity to TAINT
	v2 := e.PropagateWithTool(traceID, "output2", "web_fetch", []string{userVar.ID})
	if v2.Label.Integrity != IntegTaint {
		t.Errorf("with web_fetch tool: expected integrity TAINT, got %v", v2.Label.Integrity)
	}
	if v2.Label.Confidentiality != ConfInternal {
		t.Errorf("expected conf INTERNAL, got %v", v2.Label.Confidentiality)
	}
}

func TestIFC_CheckToolCallFides_PTvsPF(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()
	e := NewIFCEngine(db, IFCConfig{
		Enabled:         true,
		ViolationAction: "warn",
		ToolRequirements: []IFCToolRequirement{
			{Tool: "send_email", RequiredInteg: IntegMedium, MaxConf: ConfInternal},
		},
	})
	traceID := "test-fides-ptpf"

	// Register a low-integrity, low-confidentiality variable
	v1 := e.RegisterVariable(traceID, "web_data", "tool:web_fetch", "untrusted")
	e.mu.Lock()
	e.variables[traceID][v1.ID].Label = IFCLabel{Confidentiality: ConfPublic, Integrity: IntegTaint}
	e.mu.Unlock()

	// Case 1: High-integrity context + low-conf args → P-T pass, P-F pass
	highCtx := &IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	dec1 := e.CheckToolCallFides(traceID, "send_email", []string{v1.ID}, highCtx)
	if !dec1.Allowed {
		t.Errorf("Fides: high-integrity context should allow, got: %s (%s)", dec1.Decision, dec1.Reason)
	}

	// Case 2: Low-integrity context → P-T fail
	lowCtx := &IFCLabel{Confidentiality: ConfPublic, Integrity: IntegLow}
	dec2 := e.CheckToolCallFides(traceID, "send_email", []string{v1.ID}, lowCtx)
	if dec2.Allowed {
		t.Errorf("Fides: low-integrity context should block")
	}

	// Case 3: Backward compat (nil context) → falls back to args (TAINT < MEDIUM → fail)
	dec3 := e.CheckToolCallFides(traceID, "send_email", []string{v1.ID}, nil)
	if dec3.Allowed {
		t.Errorf("backward-compat: tainted args should fail P-T")
	}
}

func TestIFC_FidesExplicitSecrecy(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()
	e := NewIFCEngine(db, IFCConfig{
		Enabled:         true,
		ViolationAction: "block",
		ToolRequirements: []IFCToolRequirement{
			{Tool: "send_external", RequiredInteg: IntegHigh, MaxConf: ConfPublic},
		},
	})
	traceID := "test-fides-secrecy"

	// Register a SECRET variable
	v1 := e.RegisterVariable(traceID, "secret_doc", "system_prompt", "top secret")
	e.mu.Lock()
	e.variables[traceID][v1.ID].Label = IFCLabel{Confidentiality: ConfSecret, Integrity: IntegHigh}
	e.mu.Unlock()

	// Trying to send SECRET data through send_external (MaxConf=PUBLIC) → P-F violation
	trustedCtx := &IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}
	dec := e.CheckToolCallFides(traceID, "send_external", []string{v1.ID}, trustedCtx)
	if dec.Allowed {
		t.Error("P-F should block: SECRET data → PUBLIC-only tool")
	}
}

func TestIFC_SelectiveHide(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()
	e := NewIFCEngine(db, IFCConfig{
		Enabled:     true,
		DefaultConf: ConfPublic,
		DefaultInteg: IntegHigh,
		SourceRules: []IFCSourceRule{
			{Source: "tool:web_fetch", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegTaint}},
			{Source: "tool:read_secret", Label: IFCLabel{Confidentiality: ConfSecret, Integrity: IntegHigh}},
			{Source: "tool:calculator", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegHigh}},
		},
	})
	traceID := "test-selective-hide"
	contextLabel := IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium}

	// Case 1: calculator (PUBLIC, HIGH) — within context bounds → no hiding
	r1 := e.SelectiveHide(traceID, "calculator", "42", contextLabel)
	if r1.Hidden {
		t.Error("calculator should NOT be hidden — label within context")
	}
	if r1.Modified != "42" {
		t.Errorf("content should be unchanged, got: %s", r1.Modified)
	}

	// Case 2: web_fetch (PUBLIC, TAINT) — would lower integrity (TAINT < MEDIUM) → HIDE
	r2 := e.SelectiveHide(traceID, "web_fetch", "untrusted web content here", contextLabel)
	if !r2.Hidden {
		t.Error("web_fetch should be hidden — integrity TAINT < context MEDIUM")
	}
	if len(r2.VarIDs) != 1 {
		t.Errorf("expected 1 var, got %d", len(r2.VarIDs))
	}
	if !strings.Contains(r2.Modified, "IFC_VAR:") {
		t.Errorf("modified should contain IFC_VAR placeholder, got: %s", r2.Modified)
	}
	if !strings.Contains(r2.Reason, "integ") {
		t.Errorf("reason should mention integ, got: %s", r2.Reason)
	}

	// Case 3: read_secret (SECRET, HIGH) — would raise conf (SECRET > INTERNAL) → HIDE
	r3 := e.SelectiveHide(traceID, "read_secret", "password=hunter2", contextLabel)
	if !r3.Hidden {
		t.Error("read_secret should be hidden — conf SECRET > context INTERNAL")
	}
	if !strings.Contains(r3.Reason, "conf") {
		t.Errorf("reason should mention conf, got: %s", r3.Reason)
	}

	// Verify content is stored and expandable
	content, label, ok := e.ExpandVariable(traceID, r3.VarIDs[0])
	if !ok {
		t.Error("ExpandVariable should find the hidden content")
	}
	if content != "password=hunter2" {
		t.Errorf("expanded content mismatch: got %s", content)
	}
	if label.Confidentiality != ConfSecret {
		t.Errorf("expanded label conf should be SECRET, got %v", label.Confidentiality)
	}
}

func TestIFC_SelectiveHide_NoSourceRule(t *testing.T) {
	db := newTestIFCDB(t)
	defer db.Close()
	e := NewIFCEngine(db, IFCConfig{
		Enabled:      true,
		DefaultConf:  ConfPublic,
		DefaultInteg: IntegMedium,
	})
	traceID := "test-sh-default"
	ctx := IFCLabel{Confidentiality: ConfPublic, Integrity: IntegMedium}

	// Unknown tool → uses defaults (PUBLIC, MEDIUM) → same as context → no hiding
	r := e.SelectiveHide(traceID, "unknown_tool", "some data", ctx)
	if r.Hidden {
		t.Error("unknown tool with default label matching context should not hide")
	}
}
