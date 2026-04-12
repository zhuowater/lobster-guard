package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagementAPI_SourceClassifierGetAndUpdate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("management_token: test\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cfg := &Config{ManagementToken: "test"}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath, managementToken: "test"}

	putBody := map[string]interface{}{
		"rules": []map[string]interface{}{{
			"name": "corp-control-plane",
			"host_pattern": `^control\.corp\.example$`,
			"category": "internal_control_plane",
			"confidentiality": 3,
			"integrity": 3,
			"trust_score": 0.91,
		}},
	}
	buf, _ := json.Marshal(putBody)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/source-classifier", bytes.NewReader(buf))
	putReq.Header.Set("Authorization", "Bearer test")
	putRec := httptest.NewRecorder()
	api.ServeHTTP(putRec, putReq)
	if putRec.Code != 200 {
		t.Fatalf("expected 200 on PUT, got %d body=%s", putRec.Code, putRec.Body.String())
	}
	if len(api.cfg.SourceClassifier.Rules) != 1 {
		t.Fatalf("expected config to be updated, got %#v", api.cfg.SourceClassifier)
	}
	persisted, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	if !strings.Contains(string(persisted), "source_classifier:") || !strings.Contains(string(persisted), "corp-control-plane") {
		t.Fatalf("expected source_classifier to be persisted, got:\n%s", string(persisted))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/source-classifier", nil)
	getReq.Header.Set("Authorization", "Bearer test")
	getRec := httptest.NewRecorder()
	api.ServeHTTP(getRec, getReq)
	if getRec.Code != 200 {
		t.Fatalf("expected 200 on GET, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var resp struct {
		Config ToolSourceClassifierConfig `json:"config"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Config.Rules) != 1 || resp.Config.Rules[0].Category != "internal_control_plane" {
		t.Fatalf("unexpected response config: %#v", resp.Config)
	}
}

func TestClassifyToolSourceForTenant_OverrideWins(t *testing.T) {
	db := openTestSQLite(t)
	tm := NewTenantManager(db)
	if err := tm.Create(&Tenant{ID: "tenant-a", Name: "Tenant A", Enabled: true}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := tm.UpdateConfig(&TenantConfig{
		TenantID: "tenant-a",
		SourceClassifierYAML: "rules:\n  - name: tenant-docs-override\n    host_pattern: '^docs\\.python\\.org$'\n    category: tenant_docs\n    confidentiality: 2\n    integrity: 2\n    trust_score: 0.77\n",
	}); err != nil {
		t.Fatalf("update tenant config: %v", err)
	}
	SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{Rules: []ToolSourceRule{{
		Name: "global-docs-default", HostPattern: `^docs\\.python\\.org$`, Category: "public_web_docs", Confidentiality: ConfPublic, Integrity: IntegTaint, TrustScore: 0.2,
	}}})
	defer SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{})

	desc := classifyToolSourceForTenant(tm, "tenant-a", "web_fetch", `{"url":"https://docs.python.org/3/library/json.html"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "tenant_docs" {
		t.Fatalf("expected tenant override category, got %#v", desc)
	}
	if desc.Confidentiality != ConfConfidential || desc.Integrity != IntegMedium {
		t.Fatalf("expected tenant override labels, got conf=%v integ=%v", desc.Confidentiality, desc.Integrity)
	}
}

func TestManagementAPI_TenantConfigRoundTripIncludesSourceClassifierYAML(t *testing.T) {
	db := openTestSQLite(t)
	tm := NewTenantManager(db)
	if err := tm.Create(&Tenant{ID: "tenant-a", Name: "Tenant A", Enabled: true}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	api := &ManagementAPI{tenantMgr: tm, managementToken: "test"}

	body := bytes.NewBufferString(`{"source_classifier_yaml":"rules:\n  - name: tenant-docs\n    host_pattern: '^docs\\.python\\.org$'\n    category: tenant_docs\n","alert_level":"medium"}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/tenants/tenant-a/config", body)
	putReq.Header.Set("Authorization", "Bearer test")
	putRec := httptest.NewRecorder()
	api.ServeHTTP(putRec, putReq)
	if putRec.Code != 200 {
		t.Fatalf("expected 200 on PUT, got %d body=%s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/tenant-a/config", nil)
	getReq.Header.Set("Authorization", "Bearer test")
	getRec := httptest.NewRecorder()
	api.ServeHTTP(getRec, getReq)
	if getRec.Code != 200 {
		t.Fatalf("expected 200 on GET, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var resp struct {
		Config TenantConfig `json:"config"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(resp.Config.SourceClassifierYAML, "tenant-docs") {
		t.Fatalf("expected source classifier yaml in tenant config, got %#v", resp.Config)
	}
}

func TestManagementAPI_SourceClassifierExplainUsesTenantOverrideAndReturnsDecisions(t *testing.T) {
	db := openTestSQLite(t)
	tm := NewTenantManager(db)
	if err := tm.Create(&Tenant{ID: "tenant-a", Name: "Tenant A", Enabled: true}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := tm.UpdateConfig(&TenantConfig{
		TenantID: "tenant-a",
		SourceClassifierYAML: "rules:\n  - name: tenant-docs-override\n    host_pattern: '^docs\\.python\\.org$'\n    category: tenant_docs\n    confidentiality: 2\n    integrity: 2\n    trust_score: 0.77\n",
	}); err != nil {
		t.Fatalf("update tenant config: %v", err)
	}
	cfg := &Config{ManagementToken: "test", SourceClassifier: ToolSourceClassifierConfig{Rules: []ToolSourceRule{{
		Name: "global-docs-default", HostPattern: `^docs\\.python\\.org$`, Category: "public_web_docs", Confidentiality: ConfPublic, Integrity: IntegTaint, TrustScore: 0.2,
	}}}}
	applySourceClassifierConfig(cfg)
	defer SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{})

	api := &ManagementAPI{
		cfg:              cfg,
		managementToken:  "test",
		tenantMgr:        tm,
		pathPolicyEngine: NewPathPolicyEngine(nil),
		capabilityEngine: NewCapabilityEngine(db, CapConfig{Enabled: true, DefaultPolicy: "warn", TrustThreshold: 0.5}),
	}

	body := map[string]interface{}{
		"tenant_id":        "tenant-a",
		"tool_name":        "web_fetch",
		"tool_args":        map[string]interface{}{"url": "https://docs.python.org/3/library/json.html"},
		"proposed_action":  "shell_exec",
		"capability_action": "write",
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/source-classifier/explain", bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 on POST explain, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		TenantOverrideActive bool              `json:"tenant_override_active"`
		GlobalDescriptor     *SourceDescriptor `json:"global_descriptor"`
		EffectiveDescriptor  *SourceDescriptor `json:"effective_descriptor"`
		GlobalRule           map[string]any    `json:"global_rule"`
		EffectiveRule        map[string]any    `json:"effective_rule"`
		PathDecision         *PathDecision     `json:"path_decision"`
		CapabilityEvaluation *CapEvaluation    `json:"capability_evaluation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.TenantOverrideActive {
		t.Fatal("expected tenant override to be active")
	}
	if resp.GlobalDescriptor == nil || resp.GlobalDescriptor.Category != "public_web" {
		t.Fatalf("unexpected global descriptor: %#v", resp.GlobalDescriptor)
	}
	if resp.EffectiveDescriptor == nil || resp.EffectiveDescriptor.Category != "tenant_docs" {
		t.Fatalf("unexpected effective descriptor: %#v", resp.EffectiveDescriptor)
	}
	if resp.GlobalRule == nil || resp.GlobalRule["matched"] != false {
		t.Fatalf("expected no matched global config rule, got %#v", resp.GlobalRule)
	}
	if resp.EffectiveRule == nil || resp.EffectiveRule["name"] != "tenant-docs-override" {
		t.Fatalf("expected matched effective tenant rule, got %#v", resp.EffectiveRule)
	}
	if resp.PathDecision == nil || resp.PathDecision.Decision != "block" {
		t.Fatalf("expected path decision block, got %#v", resp.PathDecision)
	}
	if resp.CapabilityEvaluation == nil || resp.CapabilityEvaluation.Decision != "deny" {
		t.Fatalf("expected capability deny, got %#v", resp.CapabilityEvaluation)
	}
}
