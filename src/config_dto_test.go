package main

import "testing"

func TestBuildConfigSettingsResponse_ProvidesStableDTOSections(t *testing.T) {
	cfg := &Config{
		InboundListen:        ":8443",
		OutboundListen:       ":8444",
		ManagementListen:     ":9090",
		OpenClawUpstream:     "http://openclaw:8080",
		LanxinUpstream:       "http://lanxin:19090",
		DefaultGatewayOrigin: "http://localhost",
		LogLevel:             "debug",
		LogFormat:            "json",
		InboundDetectEnabled: true,
		OutboundAuditEnabled: false,
		DetectTimeoutMs:      250,
		RateLimit: RateLimiterConfig{
			GlobalRPS:      12,
			GlobalBurst:    24,
			PerSenderRPS:   3,
			PerSenderBurst: 6,
		},
		SessionIdleTimeoutMin: 45,
		SessionFPWindowSec:    180,
		AlertWebhook:          "https://alerts.example.com/hook",
		AlertFormat:           "generic",
		AlertMinInterval:      120,
		DBPath:                "/tmp/lobster.db",
		HeartbeatIntervalSec:  9,
		RouteDefaultPolicy:    "round-robin",
		AuditRetentionDays:    14,
		WSIdleTimeout:         90,
		BackupAutoInterval:    6,
		SessionDetectEnabled:  true,
		LLMDetectEnabled:      true,
		EnvelopeEnabled:       true,
		EvolutionEnabled:      false,
		SemanticDetector:      SemanticConfig{Enabled: true},
		HoneypotDeep:          HoneypotDeepConfig{Enabled: false},
		Singularity:           SingularityConfig{Enabled: true},
		IFC:                   IFCConfig{Enabled: true, QuarantineEnabled: true, HidingEnabled: false},
		PathPolicy:            PathPolicyConfig{Enabled: true},
		ToolPolicy:            ToolPolicyConfig{Enabled: false},
		PlanCompiler:          PlanConfig{Enabled: true},
		Capability:            CapConfig{Enabled: false},
		Deviation:             DeviationConfig{Enabled: true},
		Counterfactual:        CFConfig{Enabled: true},
		AdaptiveDecision:      AdaptiveDecisionConfig{Enabled: true},
		TaintTracker:          TaintConfig{Enabled: true},
		TaintReversal:         TaintReversalConfig{Enabled: false},
		EventBus:              EventBusConfig{Enabled: true},
	}

	got, err := buildConfigSettingsResponse(cfg)
	if err != nil {
		t.Fatalf("buildConfigSettingsResponse error: %v", err)
	}

	for _, key := range []string{"basic", "security", "rate_limit", "session", "alerts", "advanced", "engine_toggles"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected top-level key %q in response", key)
		}
	}

	basic := requireObject(t, got, "basic")
	assertEqual(t, basic["inbound_listen"], ":8443", "basic.inbound_listen")
	assertEqual(t, basic["outbound_listen"], ":8444", "basic.outbound_listen")
	assertEqual(t, basic["management_listen"], ":9090", "basic.management_listen")
	assertEqual(t, basic["log_level"], "debug", "basic.log_level")
	assertEqual(t, basic["log_format"], "json", "basic.log_format")

	security := requireObject(t, got, "security")
	assertEqual(t, security["inbound_detect_enabled"], true, "security.inbound_detect_enabled")
	assertEqual(t, security["outbound_audit_enabled"], false, "security.outbound_audit_enabled")
	assertEqual(t, security["detect_timeout_ms"], float64(250), "security.detect_timeout_ms")

	rateLimit := requireObject(t, got, "rate_limit")
	assertEqual(t, rateLimit["global_rps"], float64(12), "rate_limit.global_rps")
	assertEqual(t, rateLimit["global_burst"], float64(24), "rate_limit.global_burst")
	assertEqual(t, rateLimit["per_sender_rps"], float64(3), "rate_limit.per_sender_rps")
	assertEqual(t, rateLimit["per_sender_burst"], float64(6), "rate_limit.per_sender_burst")

	session := requireObject(t, got, "session")
	assertEqual(t, session["session_idle_timeout_min"], float64(45), "session.session_idle_timeout_min")
	assertEqual(t, session["session_fp_window_sec"], float64(180), "session.session_fp_window_sec")

	alerts := requireObject(t, got, "alerts")
	assertEqual(t, alerts["alert_webhook"], "https://alerts.example.com/hook", "alerts.alert_webhook")
	assertEqual(t, alerts["alert_format"], "generic", "alerts.alert_format")
	assertEqual(t, alerts["alert_min_interval"], float64(120), "alerts.alert_min_interval")

	advanced := requireObject(t, got, "advanced")
	assertEqual(t, advanced["db_path"], "/tmp/lobster.db", "advanced.db_path")
	assertEqual(t, advanced["heartbeat_interval_sec"], float64(9), "advanced.heartbeat_interval_sec")
	assertEqual(t, advanced["route_default_policy"], "round-robin", "advanced.route_default_policy")
	assertEqual(t, advanced["audit_retention_days"], float64(14), "advanced.audit_retention_days")
	assertEqual(t, advanced["ws_idle_timeout"], float64(90), "advanced.ws_idle_timeout")
	assertEqual(t, advanced["backup_auto_interval"], float64(6), "advanced.backup_auto_interval")

	engines := requireObject(t, got, "engine_toggles")
	assertEqual(t, engines["engine_inbound_detect"], true, "engine_toggles.engine_inbound_detect")
	assertEqual(t, engines["engine_tool_policy"], false, "engine_toggles.engine_tool_policy")
	assertEqual(t, engines["engine_event_bus"], true, "engine_toggles.engine_event_bus")
}

func TestBuildConfigSettingsResponse_PreservesLegacyFieldsForCompatibility(t *testing.T) {
	cfg := &Config{
		InboundListen:        ":7001",
		ManagementListen:     ":9091",
		InboundDetectEnabled: false,
		RateLimit:            RateLimiterConfig{GlobalRPS: 99},
		TaintReversal:        TaintReversalConfig{Enabled: true, RequestMode: "hard", ResponseMode: "soft"},
	}

	got, err := buildConfigSettingsResponse(cfg)
	if err != nil {
		t.Fatalf("buildConfigSettingsResponse error: %v", err)
	}

	assertEqual(t, got["InboundListen"], ":7001", "legacy InboundListen")
	assertEqual(t, got["ManagementListen"], ":9091", "legacy ManagementListen")
	assertEqual(t, got["InboundDetectEnabled"], false, "legacy InboundDetectEnabled")

	rl, ok := got["RateLimit"].(map[string]interface{})
	if !ok {
		t.Fatalf("legacy RateLimit should remain object, got %#v", got["RateLimit"])
	}
	assertEqual(t, rl["GlobalRPS"], float64(99), "legacy RateLimit.GlobalRPS")

	tr, ok := got["TaintReversal"].(map[string]interface{})
	if !ok {
		t.Fatalf("legacy TaintReversal should remain object, got %#v", got["TaintReversal"])
	}
	assertEqual(t, tr["request_mode"], "hard", "legacy TaintReversal.request_mode")
	assertEqual(t, tr["response_mode"], "soft", "legacy TaintReversal.response_mode")
}

func requireObject(t *testing.T, root map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	v, ok := root[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("key %q should be object, got %#v", key, v)
	}
	return obj
}

func assertEqual(t *testing.T, got, want interface{}, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}
