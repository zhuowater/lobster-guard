package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ParsesSourceClassifierRules(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := `
inbound_listen: ":8080"
outbound_listen: ":8081"
management_listen: ":9090"
management_token: "test"
static_upstreams:
  - id: up1
    address: 127.0.0.1
    port: 8080
source_classifier:
  rules:
    - name: corp-control-plane
      host_pattern: '^control\.corp\.example$'
      path_pattern: '/v[0-9]+/admin/'
      category: internal_control_plane
      confidentiality: 3
      integrity: 3
      trust_score: 0.92
      tags: [control_plane, corp_override]
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.SourceClassifier.Rules) != 1 {
		t.Fatalf("expected 1 source classifier rule, got %d", len(cfg.SourceClassifier.Rules))
	}
	rule := cfg.SourceClassifier.Rules[0]
	if rule.Category != "internal_control_plane" || rule.TrustScore != 0.92 {
		t.Fatalf("unexpected parsed rule: %#v", rule)
	}
}

func TestValidateConfig_InvalidSourceClassifierRegex(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
		SourceClassifier: ToolSourceClassifierConfig{
			Rules: []ToolSourceRule{{Name: "bad-host", HostPattern: "[invalid", Category: "internal_api", Confidentiality: ConfInternal, Integrity: IntegLow}},
		},
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if e != "" && containsConfigErrFragment(e, "source_classifier") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected source_classifier regex validation error, got %v", errs)
	}
}

func TestApplySourceClassifierConfig_SetsDefaultClassifier(t *testing.T) {
	SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{})
	t.Cleanup(func() { SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{}) })
	cfg := &Config{SourceClassifier: ToolSourceClassifierConfig{Rules: []ToolSourceRule{{
		Name:            "docs-override",
		HostPattern:     `^docs\.python\.org$`,
		Category:        "internal_knowledge_portal",
		Confidentiality: ConfInternal,
		Integrity:       IntegMedium,
		TrustScore:      0.8,
	}}}}
	applySourceClassifierConfig(cfg)
	desc := NewToolSourceClassifier().Classify("web_fetch", `{"url":"https://docs.python.org/3/library/json.html"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "internal_knowledge_portal" {
		t.Fatalf("expected injected default classifier config to override category, got %q", desc.Category)
	}
}

func containsConfigErrFragment(s, frag string) bool {
	return len(s) > 0 && len(frag) > 0 && (stringContains(s, frag))
}

func stringContains(s, frag string) bool {
	return len(frag) == 0 || (len(s) >= len(frag) && (func() bool {
		for i := 0; i+len(frag) <= len(s); i++ {
			if s[i:i+len(frag)] == frag {
				return true
			}
		}
		return false
	})())
}
