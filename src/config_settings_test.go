package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigSettingsGet_EngineTogglesContract(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	initial := `inbound_detect_enabled: true
session_detect_enabled: false
llm_detect_enabled: true
envelope_enabled: false
evolution_enabled: true
honeypot:
  enabled: false
semantic_detector:
  enabled: false
honeypot_deep:
  enabled: true
singularity:
  enabled: false
ifc:
  enabled: true
  quarantine_enabled: false
  hiding_enabled: true
path_policy:
  enabled: false
tool_policy:
  enabled: true
plan_compiler:
  enabled: false
capability:
  enabled: true
deviation:
  enabled: false
counterfactual:
  enabled: true
adaptive_decision:
  enabled: false
taint_tracker:
  enabled: true
taint_reversal:
  enabled: false
event_bus:
  enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/config/settings", nil)
	api.handleConfigSettingsGet(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET code=%d body=%s", rr.Code, rr.Body.String())
	}

	var got map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	engineTogglesRaw, ok := got["engine_toggles"]
	if !ok {
		t.Fatalf("expected engine_toggles in response, got keys: %#v", got)
	}
	engineToggles, ok := engineTogglesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("engine_toggles should be object, got %#v", engineTogglesRaw)
	}

	expected := map[string]bool{
		"engine_inbound_detect": true,
		"engine_session_detect": false,
		"engine_llm_detect": true,
		"engine_semantic": false,
		"engine_honeypot": false,
		"engine_honeypot_deep": true,
		"engine_singularity": false,
		"engine_ifc": true,
		"engine_ifc_quarantine": false,
		"engine_ifc_hiding": true,
		"engine_path_policy": false,
		"engine_tool_policy": true,
		"engine_plan_compiler": false,
		"engine_capability": true,
		"engine_deviation": false,
		"engine_counterfactual": true,
		"engine_envelope": false,
		"engine_evolution": true,
		"engine_adaptive": false,
		"engine_taint_tracker": true,
		"engine_taint_reversal": false,
		"engine_event_bus": true,
	}
	for key, want := range expected {
		v, ok := engineToggles[key]
		if !ok {
			t.Fatalf("missing engine_toggles[%s]", key)
		}
		gotBool, ok := v.(bool)
		if !ok {
			t.Fatalf("engine_toggles[%s] should be bool, got %#v", key, v)
		}
		if gotBool != want {
			t.Fatalf("engine_toggles[%s]=%v want %v", key, gotBool, want)
		}
	}
}

func TestConfigSettingsUpdate_PersistsEngineTogglesToYAML(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	initial := `inbound_detect_enabled: true
session_detect_enabled: true
llm_detect_enabled: true
envelope_enabled: true
evolution_enabled: true
honeypot:
  enabled: true
semantic_detector:
  enabled: true
honeypot_deep:
  enabled: true
singularity:
  enabled: true
ifc:
  enabled: true
  quarantine_enabled: true
  hiding_enabled: true
path_policy:
  enabled: true
tool_policy:
  enabled: true
plan_compiler:
  enabled: true
capability:
  enabled: true
deviation:
  enabled: true
counterfactual:
  enabled: true
adaptive_decision:
  enabled: true
taint_tracker:
  enabled: true
taint_reversal:
  enabled: true
event_bus:
  enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath}

	update := map[string]bool{
		"engine_inbound_detect": false,
		"engine_session_detect": false,
		"engine_llm_detect": false,
		"engine_semantic": false,
		"engine_honeypot": false,
		"engine_honeypot_deep": false,
		"engine_singularity": false,
		"engine_ifc": false,
		"engine_ifc_quarantine": false,
		"engine_ifc_hiding": false,
		"engine_path_policy": false,
		"engine_tool_policy": false,
		"engine_plan_compiler": false,
		"engine_capability": false,
		"engine_deviation": false,
		"engine_counterfactual": false,
		"engine_envelope": false,
		"engine_evolution": false,
		"engine_adaptive": false,
		"engine_taint_tracker": false,
		"engine_taint_reversal": false,
		"engine_event_bus": false,
	}
	body, _ := json.Marshal(update)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/config/settings", bytes.NewReader(body))
	api.handleConfigSettingsUpdate(rr, req)
	if rr.Code != 200 {
		t.Fatalf("PUT code=%d body=%s", rr.Code, rr.Body.String())
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	assertPathBool(t, raw, "inbound_detect_enabled", false)
	assertPathBool(t, raw, "session_detect_enabled", false)
	assertPathBool(t, raw, "llm_detect_enabled", false)
	assertPathBool(t, raw, "envelope_enabled", false)
	assertPathBool(t, raw, "evolution_enabled", false)
	assertPathBool(t, raw, "honeypot.enabled", false)
	assertPathBool(t, raw, "semantic_detector.enabled", false)
	assertPathBool(t, raw, "honeypot_deep.enabled", false)
	assertPathBool(t, raw, "singularity.enabled", false)
	assertPathBool(t, raw, "ifc.enabled", false)
	assertPathBool(t, raw, "ifc.quarantine_enabled", false)
	assertPathBool(t, raw, "ifc.hiding_enabled", false)
	assertPathBool(t, raw, "path_policy.enabled", false)
	assertPathBool(t, raw, "tool_policy.enabled", false)
	assertPathBool(t, raw, "plan_compiler.enabled", false)
	assertPathBool(t, raw, "capability.enabled", false)
	assertPathBool(t, raw, "deviation.enabled", false)
	assertPathBool(t, raw, "counterfactual.enabled", false)
	assertPathBool(t, raw, "adaptive_decision.enabled", false)
	assertPathBool(t, raw, "taint_tracker.enabled", false)
	assertPathBool(t, raw, "taint_reversal.enabled", false)
	assertPathBool(t, raw, "event_bus.enabled", false)
}

func TestConfigSettingsUpdate_UpdatesHoneypotRuntimeState(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	initial := `honeypot:
  enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	hp := &HoneypotEngine{enabled: true}
	api := &ManagementAPI{cfg: cfg, cfgPath: cfgPath, honeypotEngine: hp}

	body, _ := json.Marshal(map[string]bool{"engine_honeypot": false})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/config/settings", bytes.NewReader(body))
	api.handleConfigSettingsUpdate(rr, req)
	if rr.Code != 200 {
		t.Fatalf("PUT code=%d body=%s", rr.Code, rr.Body.String())
	}
	if api.honeypotEngine.IsEnabled() {
		t.Fatal("expected honeypot engine runtime state to be disabled")
	}
}

func assertPathBool(t *testing.T, raw map[string]interface{}, path string, want bool) {
	t.Helper()
	v, ok := digYAML(raw, path)
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

func digYAML(raw map[string]interface{}, path string) (interface{}, bool) {
	cur := interface{}(raw)
	for _, part := range bytes.Split([]byte(path), []byte(".")) {
		switch m := cur.(type) {
		case map[string]interface{}:
			var ok bool
			cur, ok = m[string(part)]
			if !ok {
				return nil, false
			}
		case map[interface{}]interface{}:
			var ok bool
			cur, ok = m[string(part)]
			if !ok {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return cur, true
}
