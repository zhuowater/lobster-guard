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
