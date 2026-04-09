package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigPersistence_PatchSectionPreservesUnrelatedFields(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	initial := `inbound_listen: ":8443"
llm_proxy:
  enabled: false
  listen: ":8445"
other_section:
  keep_me: true
  nested:
    value: 42
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewConfigPersistence(&sync.Mutex{}, cfgPath)
	if err := p.PatchSection("llm_proxy", map[string]interface{}{"enabled": true}); err != nil {
		t.Fatalf("PatchSection failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	assertConfigPathBool(t, raw, "llm_proxy.enabled", true)
	assertConfigPathString(t, raw, "llm_proxy.listen", ":8445")
	assertConfigPathBool(t, raw, "other_section.keep_me", true)
	assertConfigPathInt(t, raw, "other_section.nested.value", 42)
	assertConfigPathString(t, raw, "inbound_listen", ":8443")
}

func TestConfigPersistence_ReplaceSectionAndSyncConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `llm_proxy:
  enabled: false
  listen: ":8445"
other: 1
`
	confdCfg := `llm_proxy:
  enabled: false
  listen: ":9445"
something_else: true
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "llm.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewConfigPersistence(&sync.Mutex{}, cfgPath)
	newSection := map[string]interface{}{
		"enabled": true,
		"listen":  ":18445",
		"audit": map[string]interface{}{
			"log_tool_input": true,
		},
	}
	if err := p.ReplaceSectionAndSyncConfD("llm_proxy", newSection); err != nil {
		t.Fatalf("ReplaceSectionAndSyncConfD failed: %v", err)
	}

	mainRaw := mustReadYAML(t, cfgPath)
	confRaw := mustReadYAML(t, filepath.Join(confDir, "llm.yaml"))
	assertConfigPathBool(t, mainRaw, "llm_proxy.enabled", true)
	assertConfigPathString(t, mainRaw, "llm_proxy.listen", ":18445")
	assertConfigPathBool(t, mainRaw, "llm_proxy.audit.log_tool_input", true)
	assertConfigPathBool(t, confRaw, "llm_proxy.enabled", true)
	assertConfigPathString(t, confRaw, "llm_proxy.listen", ":18445")
	assertConfigPathBool(t, confRaw, "llm_proxy.audit.log_tool_input", true)
	assertConfigPathBool(t, confRaw, "something_else", true)
}

func TestSaveLLMConfig_SyncsConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `llm_proxy:
  enabled: false
  listen: ":8445"
`
	confdCfg := `llm_proxy:
  enabled: false
  listen: ":9445"
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "llm.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LLMProxy.Enabled = true
	cfg.LLMProxy.Listen = ":18445"
	cfg.LLMProxy.Security.ScanPIIInResponse = true
	cfg.LLMProxy.AuditConfig.LogToolInput = true
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath}

	if err := api.saveLLMConfig(); err != nil {
		t.Fatalf("saveLLMConfig failed: %v", err)
	}

	mainRaw := mustReadYAML(t, cfgPath)
	confRaw := mustReadYAML(t, filepath.Join(confDir, "llm.yaml"))
	assertConfigPathBool(t, mainRaw, "llm_proxy.enabled", true)
	assertConfigPathString(t, mainRaw, "llm_proxy.listen", ":18445")
	assertConfigPathBool(t, mainRaw, "llm_proxy.security.scan_pii_in_response", true)
	assertConfigPathBool(t, confRaw, "llm_proxy.enabled", true)
	assertConfigPathString(t, confRaw, "llm_proxy.listen", ":18445")
	assertConfigPathBool(t, confRaw, "llm_proxy.security.scan_pii_in_response", true)
}

func TestSaveRoutePolicies_SyncsConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-a"
`
	confdCfg := `route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-old"
other: true
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "routing.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfgPath: cfgPath}
	policies := []RoutePolicyConfig{{
		Match:      RoutePolicyMatch{AppID: "bot-a"},
		UpstreamID: "up-new",
	}}

	if err := api.saveRoutePolicies(policies); err != nil {
		t.Fatalf("saveRoutePolicies failed: %v", err)
	}

	mainRaw := mustReadYAML(t, cfgPath)
	confRaw := mustReadYAML(t, filepath.Join(confDir, "routing.yaml"))
	assertRoutePolicyUpstreamID(t, mainRaw, "bot-a", "up-new")
	assertRoutePolicyUpstreamID(t, confRaw, "bot-a", "up-new")
	assertConfigPathBool(t, confRaw, "other", true)
}

func TestSaveRoutePolicies_SyncsCustomRelativeConfDir(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "modules")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `conf_dir: "modules"
route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-a"
`
	confdCfg := `route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-old"
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "routing.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfgPath: cfgPath}
	policies := []RoutePolicyConfig{{Match: RoutePolicyMatch{AppID: "bot-a"}, UpstreamID: "up-new"}}

	if err := api.saveRoutePolicies(policies); err != nil {
		t.Fatalf("saveRoutePolicies failed: %v", err)
	}

	confRaw := mustReadYAML(t, filepath.Join(confDir, "routing.yaml"))
	assertRoutePolicyUpstreamID(t, confRaw, "bot-a", "up-new")
}

func TestSaveRoutePolicies_SyncsAbsoluteConfDir(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(t.TempDir(), "absolute-modules")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := "conf_dir: \"" + confDir + "\"\nroute_policies:\n  - match:\n      app_id: \"bot-a\"\n    upstream_id: \"up-a\"\n"
	confdCfg := `route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-old"
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "routing.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfgPath: cfgPath}
	policies := []RoutePolicyConfig{{Match: RoutePolicyMatch{AppID: "bot-a"}, UpstreamID: "up-new"}}

	if err := api.saveRoutePolicies(policies); err != nil {
		t.Fatalf("saveRoutePolicies failed: %v", err)
	}

	confRaw := mustReadYAML(t, filepath.Join(confDir, "routing.yaml"))
	assertRoutePolicyUpstreamID(t, confRaw, "bot-a", "up-new")
}

func TestPersistOutboundRules_SyncsConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `outbound_rules:
  - name: "phone"
    pattern: "\\d{11}"
    action: "block"
`
	confdCfg := `outbound_rules:
  - name: "phone"
    pattern: "old"
    action: "warn"
keep_me: true
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "rules.yaml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath}
	enabled := true
	configs := []OutboundRuleConfig{{
		Name:    "phone",
		Pattern: "new",
		Action:  "redact",
		Enabled: &enabled,
	}}

	if err := api.persistOutboundRules(configs); err != nil {
		t.Fatalf("persistOutboundRules failed: %v", err)
	}

	mainRaw := mustReadYAML(t, cfgPath)
	confRaw := mustReadYAML(t, filepath.Join(confDir, "rules.yaml"))
	assertOutboundRuleAction(t, mainRaw, "phone", "redact")
	assertOutboundRuleAction(t, confRaw, "phone", "redact")
	assertOutboundRulePattern(t, confRaw, "phone", "new")
	assertConfigPathBool(t, confRaw, "keep_me", true)
}

func TestPersistOutboundRules_SyncsYmlConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `outbound_rules:
  - name: "phone"
    pattern: "\\d{11}"
    action: "block"
`
	confdCfg := `outbound_rules:
  - name: "phone"
    pattern: "old"
    action: "warn"
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "rules.yml"), []byte(confdCfg), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath}
	configs := []OutboundRuleConfig{{Name: "phone", Pattern: "new", Action: "warn"}}

	if err := api.persistOutboundRules(configs); err != nil {
		t.Fatalf("persistOutboundRules failed: %v", err)
	}

	confRaw := mustReadYAML(t, filepath.Join(confDir, "rules.yml"))
	assertOutboundRuleAction(t, confRaw, "phone", "warn")
	assertOutboundRulePattern(t, confRaw, "phone", "new")
}

func TestReplaceSectionAndSyncConfD_FailsBeforeMainWriteOnBadConfD(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	confDir := filepath.Join(tmp, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainCfg := `route_policies:
  - match:
      app_id: "bot-a"
    upstream_id: "up-a"
`
	if err := os.WriteFile(cfgPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "routing.yaml"), []byte("route_policies: ["), 0644); err != nil {
		t.Fatal(err)
	}
	p := NewConfigPersistence(&sync.Mutex{}, cfgPath)
	newSection := []interface{}{map[string]interface{}{
		"match": map[string]interface{}{"app_id": "bot-a"},
		"upstream_id": "up-new",
	}}

	if err := p.ReplaceSectionAndSyncConfD("route_policies", newSection); err == nil {
		t.Fatal("expected error for invalid conf.d yaml")
	}

	mainRaw := mustReadYAML(t, cfgPath)
	assertRoutePolicyUpstreamID(t, mainRaw, "bot-a", "up-a")
}

func assertRoutePolicyUpstreamID(t *testing.T, raw map[string]interface{}, appID, want string) {
	t.Helper()
	value, ok := raw["route_policies"]
	if !ok {
		t.Fatalf("missing route_policies")
	}
	policies, ok := value.([]interface{})
	if !ok {
		t.Fatalf("route_policies should be slice, got %#v", value)
	}
	for _, item := range policies {
		policy, ok := item.(map[string]interface{})
		if !ok {
			if generic, ok := item.(map[interface{}]interface{}); ok {
				policy = normalizeStringMap(generic)
			} else {
				continue
			}
		}
		match := normalizeStringMap(policy["match"])
		if gotAppID, _ := match["app_id"].(string); gotAppID == appID {
			got, _ := policy["upstream_id"].(string)
			if got != want {
				t.Fatalf("route policy app_id=%s upstream_id=%q want %q", appID, got, want)
			}
			return
		}
	}
	t.Fatalf("route policy app_id=%s not found", appID)
}

func assertOutboundRuleAction(t *testing.T, raw map[string]interface{}, name, wantAction string) {
	t.Helper()
	value, ok := raw["outbound_rules"]
	if !ok {
		t.Fatalf("missing outbound_rules")
	}
	rules, ok := value.([]interface{})
	if !ok {
		t.Fatalf("outbound_rules should be slice, got %#v", value)
	}
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			if generic, ok := item.(map[interface{}]interface{}); ok {
				rule = normalizeStringMap(generic)
			} else {
				continue
			}
		}
		if gotName, _ := rule["name"].(string); gotName == name {
			gotAction, _ := rule["action"].(string)
			if gotAction != wantAction {
				t.Fatalf("outbound rule %s action=%q want %q", name, gotAction, wantAction)
			}
			return
		}
	}
	t.Fatalf("outbound rule %s not found", name)
}

func assertOutboundRulePattern(t *testing.T, raw map[string]interface{}, name, wantPattern string) {
	t.Helper()
	value, ok := raw["outbound_rules"]
	if !ok {
		t.Fatalf("missing outbound_rules")
	}
	rules, ok := value.([]interface{})
	if !ok {
		t.Fatalf("outbound_rules should be slice, got %#v", value)
	}
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			if generic, ok := item.(map[interface{}]interface{}); ok {
				rule = normalizeStringMap(generic)
			} else {
				continue
			}
		}
		if gotName, _ := rule["name"].(string); gotName == name {
			gotPattern, _ := rule["pattern"].(string)
			if gotPattern != wantPattern {
				t.Fatalf("outbound rule %s pattern=%q want %q", name, gotPattern, wantPattern)
			}
			return
		}
	}
	t.Fatalf("outbound rule %s not found", name)
}

func mustReadYAML(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func digConfigPath(raw map[string]interface{}, path string) (interface{}, bool) {
	cur := interface{}(raw)
	parts := splitPath(path)
	for _, part := range parts {
		switch m := cur.(type) {
		case map[string]interface{}:
			var ok bool
			cur, ok = m[part]
			if !ok {
				return nil, false
			}
		case map[interface{}]interface{}:
			var ok bool
			cur, ok = m[part]
			if !ok {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return cur, true
}

func splitPath(path string) []string {
	parts := []string{}
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	return parts
}

func assertConfigPathBool(t *testing.T, raw map[string]interface{}, path string, want bool) {
	t.Helper()
	v, ok := digConfigPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	got, ok := v.(bool)
	if !ok {
		t.Fatalf("path %s should be bool, got %#v", path, v)
	}
	if got != want {
		t.Fatalf("path %s = %v want %v", path, got, want)
	}
}

func assertConfigPathString(t *testing.T, raw map[string]interface{}, path string, want string) {
	t.Helper()
	v, ok := digConfigPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	got, ok := v.(string)
	if !ok {
		t.Fatalf("path %s should be string, got %#v", path, v)
	}
	if got != want {
		t.Fatalf("path %s = %q want %q", path, got, want)
	}
}

func assertConfigPathInt(t *testing.T, raw map[string]interface{}, path string, want int) {
	t.Helper()
	v, ok := digConfigPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	switch got := v.(type) {
	case int:
		if got != want {
			t.Fatalf("path %s = %d want %d", path, got, want)
		}
	case int64:
		if int(got) != want {
			t.Fatalf("path %s = %d want %d", path, got, want)
		}
	case float64:
		if int(got) != want {
			t.Fatalf("path %s = %v want %d", path, got, want)
		}
	default:
		t.Fatalf("path %s should be numeric, got %#v", path, v)
	}
}
