package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSettingsGet_DTOSections(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	initial := `inbound_listen: ":18443"
outbound_listen: ":18444"
management_listen: ":9090"
openclaw_upstream: "http://127.0.0.1:18790"
lanxin_upstream: "https://example.com"
default_gateway_origin: "http://localhost"
log_level: "debug"
log_format: "json"
inbound_detect_enabled: true
outbound_audit_enabled: false
detect_timeout_ms: 321
rate_limit:
  global_rps: 123
  global_burst: 456
  per_sender_rps: 7
  per_sender_burst: 8
session_idle_timeout_min: 66
session_fp_window_sec: 777
alert_webhook: "https://webhook"
alert_format: "generic"
alert_min_interval: 99
db_path: "/tmp/audit.db"
heartbeat_interval_sec: 12
route_default_policy: "round-robin"
audit_retention_days: 45
ws_idle_timeout: 901
backup_auto_interval: 6
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

	assertJSONPathString(t, got, "basic.inbound_listen", ":18443")
	assertJSONPathString(t, got, "basic.outbound_listen", ":18444")
	assertJSONPathString(t, got, "basic.management_listen", ":9090")
	assertJSONPathString(t, got, "basic.log_level", "debug")
	assertJSONPathString(t, got, "basic.log_format", "json")
	assertJSONPathBool(t, got, "security.inbound_detect_enabled", true)
	assertJSONPathBool(t, got, "security.outbound_audit_enabled", false)
	assertJSONPathInt(t, got, "security.detect_timeout_ms", 321)
	assertJSONPathInt(t, got, "rate_limit.global_burst", 456)
	assertJSONPathInt(t, got, "rate_limit.per_sender_burst", 8)
	assertJSONPathInt(t, got, "session.session_idle_timeout_min", 66)
	assertJSONPathInt(t, got, "session.session_fp_window_sec", 777)
	assertJSONPathString(t, got, "alerts.alert_webhook", "https://webhook")
	assertJSONPathString(t, got, "alerts.alert_format", "generic")
	assertJSONPathInt(t, got, "alerts.alert_min_interval", 99)
	assertJSONPathString(t, got, "advanced.db_path", "/tmp/audit.db")
	assertJSONPathInt(t, got, "advanced.heartbeat_interval_sec", 12)
	assertJSONPathString(t, got, "advanced.route_default_policy", "round-robin")
	assertJSONPathInt(t, got, "advanced.audit_retention_days", 45)
	assertJSONPathInt(t, got, "advanced.ws_idle_timeout", 901)
	assertJSONPathInt(t, got, "advanced.backup_auto_interval", 6)
}

func digJSONPath(raw map[string]interface{}, path string) (interface{}, bool) {
	cur := interface{}(raw)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i != len(path) && path[i] != '.' {
			continue
		}
		part := path[start:i]
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
		start = i + 1
	}
	return cur, true
}

func assertJSONPathString(t *testing.T, raw map[string]interface{}, path, want string) {
	t.Helper()
	v, ok := digJSONPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	got, ok := v.(string)
	if !ok {
		t.Fatalf("path %s should be string, got %#v", path, v)
	}
	if got != want {
		t.Fatalf("path %s=%q want %q", path, got, want)
	}
}

func assertJSONPathBool(t *testing.T, raw map[string]interface{}, path string, want bool) {
	t.Helper()
	v, ok := digJSONPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	got, ok := v.(bool)
	if !ok {
		t.Fatalf("path %s should be bool, got %#v", path, v)
	}
	if got != want {
		t.Fatalf("path %s=%v want %v", path, got, want)
	}
}

func assertJSONPathInt(t *testing.T, raw map[string]interface{}, path string, want int) {
	t.Helper()
	v, ok := digJSONPath(raw, path)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	got, ok := v.(float64)
	if !ok {
		t.Fatalf("path %s should be numeric, got %#v", path, v)
	}
	if int(got) != want {
		t.Fatalf("path %s=%v want %d", path, got, want)
	}
}
