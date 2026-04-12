// capability_test.go - Tests for CapabilityEngine
// lobster-guard v25.1
package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupCapTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestCapabilityNewEngine(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	if ce == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(ce.toolMappings) < 15 {
		t.Errorf("expected 15+ tool mappings, got %d", len(ce.toolMappings))
	}
}

func TestCapabilityInitContext(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ctx := ce.InitContext("trace-1", "user-1", nil)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.TraceID != "trace-1" {
		t.Errorf("expected trace-1, got %s", ctx.TraceID)
	}
	if ctx.Status != "active" {
		t.Errorf("expected active status, got %s", ctx.Status)
	}
	if len(ctx.UserCaps) != 3 {
		t.Errorf("expected 3 user caps, got %d", len(ctx.UserCaps))
	}
	if len(ctx.DataItems) != 1 {
		t.Errorf("expected 1 data item, got %d", len(ctx.DataItems))
	}
}

func TestCapabilityInitContextCustomCaps(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	caps := []CapLabel{
		{Name: "read", Source: "user_input", Level: "read", Granted: true},
	}
	ctx := ce.InitContext("trace-2", "user-2", caps)
	if len(ctx.UserCaps) != 1 {
		t.Errorf("expected 1 user cap, got %d", len(ctx.UserCaps))
	}
}

func TestCapabilityRegisterToolResult(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-3", "user-1", nil)
	tr := ce.RegisterToolResult("trace-3", "web_search", "data-1")
	if tr == nil {
		t.Fatal("expected non-nil tool result")
	}
	if tr.ToolName != "web_search" {
		t.Errorf("expected web_search, got %s", tr.ToolName)
	}
	if len(tr.RawCaps) == 0 || tr.RawCaps[0].Granted {
		t.Error("tool result raw caps should be zero/not granted")
	}
	if len(tr.MappedCaps) == 0 {
		t.Error("expected mapped caps from tool mapping")
	}
}

func TestCapabilityRegisterToolResultWithSourceDescriptor(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-src-1", "user-1", nil)

	desc := &SourceDescriptor{
		SourceKey:       "tool:http_request:metadata_service",
		Category:        "metadata_service",
		Host:            "169.254.169.254",
		Confidentiality: ConfSecret,
		Integrity:       IntegLow,
		AuthType:        "none",
		PrivateNetwork:  true,
		TrustScore:      0.05,
	}
	tr := ce.RegisterToolResultWithSource("trace-src-1", "http_request", "data-meta-1", desc)
	if tr == nil {
		t.Fatal("expected non-nil tool result")
	}
	ctx := ce.GetContext("trace-src-1")
	if ctx == nil {
		t.Fatal("expected context")
	}
	item := ctx.DataItems["data-meta-1"]
	if item == nil {
		t.Fatal("expected data item")
	}
	if item.SourceDescriptor == nil || item.SourceDescriptor.Category != "metadata_service" {
		t.Fatalf("expected source descriptor to be persisted, got %#v", item.SourceDescriptor)
	}
	if !containsString(item.Sources, "tool:http_request:metadata_service") {
		t.Fatalf("expected source key in provenance sources, got %#v", item.Sources)
	}
	if item.TrustScore > 0.1 {
		t.Fatalf("expected source-aware trust downgrade, got %.2f", item.TrustScore)
	}
}

func TestCapabilityRegisterToolResultNoContext(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	tr := ce.RegisterToolResult("trace-none", "web_search", "data-1")
	if tr != nil {
		t.Error("expected nil for unknown trace")
	}
}

func TestCapabilityEvaluateUserInput(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ctx := ce.InitContext("trace-4", "user-1", nil)
	var userDataID string
	for k := range ctx.DataItems {
		userDataID = k
		break
	}
	eval := ce.Evaluate("trace-4", userDataID, "read", "web_search")
	if eval.Decision != "allow" {
		t.Errorf("expected allow for user input, got %s", eval.Decision)
	}
}

func TestCapabilityEvaluateToolResultDenied(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-5", "user-1", nil)
	ce.RegisterToolResult("trace-5", "web_search", "data-ws-1")
	eval := ce.Evaluate("trace-5", "data-ws-1", "admin", "web_search")
	if eval.Decision != "deny" {
		t.Errorf("expected deny for admin on web_search, got %s", eval.Decision)
	}
}

func TestCapabilityEvaluateToolResultAllowed(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	cfg := defaultCapConfig
	cfg.TrustThreshold = 0.1
	ce := NewCapabilityEngine(db, cfg)
	ce.InitContext("trace-6", "user-1", nil)
	ce.RegisterToolResult("trace-6", "query_db", "data-qdb-1")
	eval := ce.Evaluate("trace-6", "data-qdb-1", "read", "query_db")
	if eval.Decision != "allow" {
		t.Errorf("expected allow for read on query_db, got %s (reason: %s)", eval.Decision, eval.Reason)
	}
}

func TestCapabilityEvaluateLowTrust(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	cfg := defaultCapConfig
	cfg.TrustThreshold = 0.9
	ce := NewCapabilityEngine(db, cfg)
	ce.InitContext("trace-7", "user-1", nil)
	ce.RegisterToolResult("trace-7", "web_fetch", "data-wf-1")
	eval := ce.Evaluate("trace-7", "data-wf-1", "read", "web_fetch")
	if eval.Decision != "warn" {
		t.Errorf("expected warn for low trust, got %s (reason: %s)", eval.Decision, eval.Reason)
	}
}

func TestCapabilityEvaluateNoContext(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	eval := ce.Evaluate("trace-none", "data-1", "read", "web_search")
	if eval.Decision != "warn" {
		t.Errorf("expected warn (default), got %s", eval.Decision)
	}
}

func TestCapabilityEvaluateUntracked(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-8", "user-1", nil)
	eval := ce.Evaluate("trace-8", "unknown-data", "read", "web_search")
	if eval.Decision != "allow" {
		t.Errorf("expected allow for untracked data, got %s", eval.Decision)
	}
}

func TestCapabilityLLMSummaryIntersection(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ctx := ce.InitContext("trace-9", "user-1", nil)
	var userDataID string
	for k := range ctx.DataItems {
		userDataID = k
		break
	}
	ce.RegisterToolResult("trace-9", "web_search", "tool-data-1")
	ce.RegisterLLMSummary("trace-9", "summary-1", []string{userDataID, "tool-data-1"})
	// intersection: user_input has read/write/execute, tool_result has none granted
	// So intersection should be empty -> deny
	eval := ce.Evaluate("trace-9", "summary-1", "read", "web_search")
	if eval.Decision != "deny" {
		t.Errorf("expected deny for summary with zero-cap tool data intersection, got %s", eval.Decision)
	}
}

func TestCapabilityCompleteContext(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-10", "user-1", nil)
	ce.CompleteContext("trace-10")
	ctx := ce.GetContext("trace-10")
	if ctx == nil {
		t.Fatal("expected context")
	}
	if ctx.Status != "completed" {
		t.Errorf("expected completed, got %s", ctx.Status)
	}
}

func TestCapabilityGetContext(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-11", "user-1", nil)
	ctx := ce.GetContext("trace-11")
	if ctx == nil {
		t.Fatal("expected context")
	}
	if ctx.TraceID != "trace-11" {
		t.Errorf("expected trace-11, got %s", ctx.TraceID)
	}
}

func TestCapabilityListToolMappings(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	mappings := ce.ListToolMappings()
	if len(mappings) < 15 {
		t.Errorf("expected 15+ mappings, got %d", len(mappings))
	}
}

func TestCapabilityUpdateToolMapping(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	err := ce.UpdateToolMapping(CapToolMapping{
		ToolName:     "custom_tool",
		Category:     "custom",
		DefaultLevel: "read",
		AllowedCaps:  []string{"read"},
		DeniedCaps:   []string{"admin"},
		TrustFactor:  0.7,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	m := ce.GetToolMapping("custom_tool")
	if m == nil {
		t.Fatal("expected mapping")
	}
	if m.TrustFactor != 0.7 {
		t.Errorf("expected trust 0.7, got %f", m.TrustFactor)
	}
}

func TestCapabilityDeleteToolMapping(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	err := ce.DeleteToolMapping("web_search")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	m := ce.GetToolMapping("web_search")
	if m != nil {
		t.Error("expected nil after delete")
	}
}

func TestCapabilityGetStats(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-s1", "user-1", nil)
	ce.RegisterToolResult("trace-s1", "web_search", "data-s1")
	ce.Evaluate("trace-s1", "data-s1", "admin", "web_search")
	stats := ce.GetStats()
	if stats.TotalContexts == 0 {
		t.Error("expected > 0 contexts")
	}
	if stats.TotalEvaluations == 0 {
		t.Error("expected > 0 evaluations")
	}
	if stats.ToolMappingCount == 0 {
		t.Error("expected > 0 tool mappings")
	}
}

func TestCapabilityQueryContexts(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-qc1", "user-1", nil)
	ce.InitContext("trace-qc2", "user-2", nil)
	ctxs, total := ce.QueryContexts("", 10, 0)
	if total < 2 {
		t.Errorf("expected at least 2, got %d", total)
	}
	if len(ctxs) < 2 {
		t.Errorf("expected at least 2 ctxs, got %d", len(ctxs))
	}
}

func TestCapabilityQueryEvaluations(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-qe1", "user-1", nil)
	ce.RegisterToolResult("trace-qe1", "web_search", "data-qe1")
	ce.Evaluate("trace-qe1", "data-qe1", "read", "web_search")
	evals, total := ce.QueryEvaluations("trace-qe1", 10, 0)
	if total < 1 {
		t.Errorf("expected at least 1, got %d", total)
	}
	if len(evals) < 1 {
		t.Errorf("expected at least 1 eval, got %d", len(evals))
	}
}

func TestCapabilityConfigUpdate(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.UpdateConfig(CapConfig{DefaultPolicy: "deny", TrustThreshold: 0.8, Enabled: true})
	cfg := ce.GetConfig()
	if cfg.DefaultPolicy != "deny" {
		t.Errorf("expected deny, got %s", cfg.DefaultPolicy)
	}
	if cfg.TrustThreshold != 0.8 {
		t.Errorf("expected 0.8, got %f", cfg.TrustThreshold)
	}
}

func TestCapabilityViolationStatus(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-vs1", "user-1", nil)
	ce.RegisterToolResult("trace-vs1", "admin_action", "data-admin-1")
	ce.Evaluate("trace-vs1", "data-admin-1", "admin", "admin_action")
	ctx := ce.GetContext("trace-vs1")
	if ctx.Status != "violated" {
		t.Errorf("expected violated after deny, got %s", ctx.Status)
	}
}

// ============================================================
// CaMeL Source Propagation + Label Propagation Tests
// ============================================================

func TestCapability_SourcePropagation(t *testing.T) {
	db := setupCapTestDB(t); defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "deny", TrustThreshold: 0.5, EnforceIntersect: true})
	traceID := "test-propagation-" + capGenID()

	// 1. Init context — user input is trusted
	ce.InitContext(traceID, "user1", nil)

	// 2. Register tool results
	ce.RegisterToolResult(traceID, "web_fetch", "data-web-1")
	ce.RegisterToolResult(traceID, "database_query", "data-db-1")

	// 3. Propagate: llm_summary depends on web_fetch + database_query
	item := ce.PropagateData(traceID, "data-summary-1", "llm_summary", []string{"data-web-1", "data-db-1"})
	if item == nil { t.Fatal("PropagateData returned nil") }

	// Check sources are union of parents
	sourceSet := map[string]bool{}
	for _, s := range item.Sources { sourceSet[s] = true }
	if !sourceSet["tool:web_fetch"] { t.Error("missing source tool:web_fetch") }
	if !sourceSet["tool:database_query"] { t.Error("missing source tool:database_query") }
	if !sourceSet["llm_summary"] { t.Error("missing own source llm_summary") }

	// Check parentIDs
	if len(item.ParentIDs) != 2 { t.Errorf("expected 2 parents, got %d", len(item.ParentIDs)) }

	// 4. Chain propagation: another tool depends on summary
	item2 := ce.PropagateData(traceID, "data-tool2-1", "tool:send_email", []string{"data-summary-1"})
	if item2 == nil { t.Fatal("second PropagateData returned nil") }
	sourceSet2 := map[string]bool{}
	for _, s := range item2.Sources { sourceSet2[s] = true }
	// Should recursively include web_fetch and database_query sources
	if !sourceSet2["tool:web_fetch"] { t.Error("chain: missing source tool:web_fetch") }
	if !sourceSet2["tool:database_query"] { t.Error("chain: missing source tool:database_query") }
	if !sourceSet2["tool:send_email"] { t.Error("chain: missing own source tool:send_email") }
}

func TestCapability_IsTrusted(t *testing.T) {
	db := setupCapTestDB(t); defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true})

	// Trusted sources
	if !ce.capIsTrusted([]string{"user_input"}) { t.Error("user_input should be trusted") }
	if !ce.capIsTrusted([]string{"user_input", "system", "assistant"}) { t.Error("user+system+assistant should be trusted") }

	// Untrusted sources
	if ce.capIsTrusted([]string{"tool:web_fetch"}) { t.Error("tool:web_fetch should be untrusted") }
	if ce.capIsTrusted([]string{"user_input", "tool:web_fetch"}) { t.Error("mixed with tool should be untrusted") }

	// Empty
	if ce.capIsTrusted([]string{}) { t.Error("empty sources should be untrusted") }
}

func TestCapability_EvaluateWithProvenance(t *testing.T) {
	db := setupCapTestDB(t); defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "deny", TrustThreshold: 0.1, EnforceIntersect: true})
	traceID := "test-provenance-" + capGenID()

	ce.InitContext(traceID, "user1", nil)

	// Register a tool result from untrusted source
	ce.RegisterToolResult(traceID, "web_fetch", "data-web-1")

	// Propagate: summary depends on web_fetch result
	ce.PropagateData(traceID, "data-summary", "llm_summary", []string{"data-web-1"})

	// Now a send_email tool wants to use that summary — should get "warn" due to untrusted lineage
	ce.RegisterToolResult(traceID, "send_email", "data-email-1")
	ce.PropagateData(traceID, "data-email-1", "tool:send_email", []string{"data-summary"})

	// Add tool mapping so basic eval passes
	ce.mu.Lock()
	ce.toolMappings["send_email"] = &CapToolMapping{
		ToolName: "send_email", AllowedCaps: []string{"execute"}, TrustFactor: 0.9,
	}
	ce.mu.Unlock()

	eval := ce.EvaluateWithProvenance(traceID, "data-email-1", "execute", "send_email")
	if eval == nil { t.Fatal("eval returned nil") }
	// Should warn because lineage includes tool:web_fetch (untrusted)
	if eval.Decision != "warn" {
		t.Errorf("expected 'warn' for untrusted lineage, got '%s' reason='%s'", eval.Decision, eval.Reason)
	}
	if !strings.Contains(eval.Reason, "untrusted sources") {
		t.Errorf("reason should mention untrusted sources, got: %s", eval.Reason)
	}
}

func TestCapability_EvaluateWithProvenance_SourceDescriptorReason(t *testing.T) {
	db := setupCapTestDB(t)
	defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "deny", TrustThreshold: 0.01, EnforceIntersect: true})
	traceID := "test-provenance-source-" + capGenID()
	ce.InitContext(traceID, "user1", nil)

	ce.mu.Lock()
	ce.toolMappings["send_email"] = &CapToolMapping{ToolName: "send_email", AllowedCaps: []string{"execute"}, TrustFactor: 0.9}
	ce.mu.Unlock()

	desc := &SourceDescriptor{
		SourceKey:       "tool:http_request:metadata_service",
		Category:        "metadata_service",
		Host:            "169.254.169.254",
		Confidentiality: ConfSecret,
		Integrity:       IntegLow,
		TrustScore:      0.05,
	}
	ce.RegisterToolResultWithSource(traceID, "http_request", "data-meta-1", desc)
	ce.PropagateData(traceID, "data-email-meta", "tool:send_email", []string{"data-meta-1"})

	eval := ce.EvaluateWithProvenance(traceID, "data-email-meta", "execute", "send_email")
	if eval == nil {
		t.Fatal("expected eval")
	}
	if eval.Decision != "warn" {
		t.Fatalf("expected warn for metadata-derived lineage, got %s (%s)", eval.Decision, eval.Reason)
	}
	if !strings.Contains(eval.Reason, "metadata_service") {
		t.Fatalf("expected reason to mention metadata_service, got %s", eval.Reason)
	}
}

func TestCapability_TrustScorePropagation(t *testing.T) {
	db := setupCapTestDB(t); defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, TrustThreshold: 0.5})
	traceID := "test-trust-" + capGenID()

	ce.InitContext(traceID, "user1", nil)

	// web_fetch has 0 trust
	ce.RegisterToolResult(traceID, "web_fetch", "data-web-1")
	// database has high trust
	ce.mu.Lock()
	ce.toolMappings["database_query"] = &CapToolMapping{ToolName: "database_query", TrustFactor: 0.95}
	ce.mu.Unlock()
	ce.RegisterToolResult(traceID, "database_query", "data-db-1")

	// Propagate — should take min trust
	item := ce.PropagateData(traceID, "data-merged", "llm_summary", []string{"data-web-1", "data-db-1"})
	if item == nil { t.Fatal("PropagateData nil") }
	if item.TrustScore >= 0.5 { t.Errorf("expected trust < 0.5 (min of web_fetch), got %.2f", item.TrustScore) }
}

func TestCapability_LabelIntersection(t *testing.T) {
	db := setupCapTestDB(t); defer db.Close()
	ce := NewCapabilityEngine(db, CapConfig{Enabled: true, EnforceIntersect: true})
	traceID := "test-intersect-" + capGenID()

	ctx := ce.InitContext(traceID, "user1", []CapLabel{
		{Name: "read", Level: "read", Granted: true},
		{Name: "write", Level: "write", Granted: true},
		{Name: "execute", Level: "execute", Granted: true},
	})

	// Manually set different labels on two items
	ce.mu.Lock()
	ctx.DataItems["item-a"] = &CapDataItem{
		DataID: "item-a", Source: "tool_result:safe_tool",
		Sources: []string{"tool:safe_tool"},
		Labels: []CapLabel{
			{Name: "read", Level: "read", Granted: true},
			{Name: "write", Level: "write", Granted: true},
		},
		TrustScore: 0.8,
	}
	ctx.DataItems["item-b"] = &CapDataItem{
		DataID: "item-b", Source: "tool_result:risky_tool",
		Sources: []string{"tool:risky_tool"},
		Labels: []CapLabel{
			{Name: "read", Level: "read", Granted: true},
			// No write — intersection should lose write
		},
		TrustScore: 0.3,
	}
	ce.mu.Unlock()

	// Propagate — intersection should only have "read"
	item := ce.PropagateData(traceID, "merged", "llm_summary", []string{"item-a", "item-b"})
	if item == nil { t.Fatal("nil") }
	hasRead, hasWrite := false, false
	for _, l := range item.Labels {
		if l.Level == "read" { hasRead = true }
		if l.Level == "write" { hasWrite = true }
	}
	if !hasRead { t.Error("intersection should include 'read'") }
	if hasWrite { t.Error("intersection should NOT include 'write' (item-b lacks it)") }

	// Trust should be min = 0.3
	if item.TrustScore != 0.3 { t.Errorf("expected trust 0.3, got %.2f", item.TrustScore) }

	// Sources should be union
	srcSet := map[string]bool{}
	for _, s := range item.Sources { srcSet[s] = true }
	if !srcSet["tool:safe_tool"] || !srcSet["tool:risky_tool"] { t.Error("sources union missing") }
}
