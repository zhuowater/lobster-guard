package main

import "encoding/json"

type ConfigSettingsResponse struct {
	Basic         ConfigSettingsBasicDTO      `json:"basic"`
	Security      ConfigSettingsSecurityDTO   `json:"security"`
	RateLimit     ConfigSettingsRateLimitDTO  `json:"rate_limit"`
	Session       ConfigSettingsSessionDTO    `json:"session"`
	Alerts        ConfigSettingsAlertsDTO     `json:"alerts"`
	Advanced      ConfigSettingsAdvancedDTO   `json:"advanced"`
	EngineToggles ConfigSettingsEngineToggles `json:"engine_toggles"`
}

type ConfigSettingsBasicDTO struct {
	InboundListen        string `json:"inbound_listen"`
	OutboundListen       string `json:"outbound_listen"`
	ManagementListen     string `json:"management_listen"`
	OpenClawUpstream     string `json:"openclaw_upstream"`
	LanxinUpstream       string `json:"lanxin_upstream"`
	DefaultGatewayOrigin string `json:"default_gateway_origin"`
	LogLevel             string `json:"log_level"`
	LogFormat            string `json:"log_format"`
}

type ConfigSettingsSecurityDTO struct {
	InboundDetectEnabled bool `json:"inbound_detect_enabled"`
	OutboundAuditEnabled bool `json:"outbound_audit_enabled"`
	DetectTimeoutMS      int  `json:"detect_timeout_ms"`
}

type ConfigSettingsRateLimitDTO struct {
	GlobalRPS      float64 `json:"global_rps"`
	GlobalBurst    int     `json:"global_burst"`
	PerSenderRPS   float64 `json:"per_sender_rps"`
	PerSenderBurst int     `json:"per_sender_burst"`
}

type ConfigSettingsSessionDTO struct {
	SessionIdleTimeoutMin int `json:"session_idle_timeout_min"`
	SessionFPWindowSec    int `json:"session_fp_window_sec"`
}

type ConfigSettingsAlertsDTO struct {
	AlertWebhook     string `json:"alert_webhook"`
	AlertFormat      string `json:"alert_format"`
	AlertMinInterval int    `json:"alert_min_interval"`
}

type ConfigSettingsAdvancedDTO struct {
	DBPath               string `json:"db_path"`
	HeartbeatIntervalSec int    `json:"heartbeat_interval_sec"`
	RouteDefaultPolicy   string `json:"route_default_policy"`
	AuditRetentionDays   int    `json:"audit_retention_days"`
	WSIdleTimeout        int    `json:"ws_idle_timeout"`
	BackupAutoInterval   int    `json:"backup_auto_interval"`
}

type ConfigSettingsEngineToggles map[string]bool

func buildConfigSettingsDTO(cfg *Config) ConfigSettingsResponse {
	return ConfigSettingsResponse{
		Basic: ConfigSettingsBasicDTO{
			InboundListen:        cfg.InboundListen,
			OutboundListen:       cfg.OutboundListen,
			ManagementListen:     cfg.ManagementListen,
			OpenClawUpstream:     cfg.OpenClawUpstream,
			LanxinUpstream:       cfg.LanxinUpstream,
			DefaultGatewayOrigin: cfg.DefaultGatewayOrigin,
			LogLevel:             cfg.LogLevel,
			LogFormat:            cfg.LogFormat,
		},
		Security: ConfigSettingsSecurityDTO{
			InboundDetectEnabled: cfg.InboundDetectEnabled,
			OutboundAuditEnabled: cfg.OutboundAuditEnabled,
			DetectTimeoutMS:      cfg.DetectTimeoutMs,
		},
		RateLimit: ConfigSettingsRateLimitDTO{
			GlobalRPS:      cfg.RateLimit.GlobalRPS,
			GlobalBurst:    cfg.RateLimit.GlobalBurst,
			PerSenderRPS:   cfg.RateLimit.PerSenderRPS,
			PerSenderBurst: cfg.RateLimit.PerSenderBurst,
		},
		Session: ConfigSettingsSessionDTO{
			SessionIdleTimeoutMin: cfg.SessionIdleTimeoutMin,
			SessionFPWindowSec:    cfg.SessionFPWindowSec,
		},
		Alerts: ConfigSettingsAlertsDTO{
			AlertWebhook:     cfg.AlertWebhook,
			AlertFormat:      cfg.AlertFormat,
			AlertMinInterval: cfg.AlertMinInterval,
		},
		Advanced: ConfigSettingsAdvancedDTO{
			DBPath:               cfg.DBPath,
			HeartbeatIntervalSec: cfg.HeartbeatIntervalSec,
			RouteDefaultPolicy:   cfg.RouteDefaultPolicy,
			AuditRetentionDays:   cfg.AuditRetentionDays,
			WSIdleTimeout:        cfg.WSIdleTimeout,
			BackupAutoInterval:   cfg.BackupAutoInterval,
		},
		EngineToggles: buildConfigSettingsEngineToggles(cfg),
	}
}

func buildConfigSettingsEngineToggles(cfg *Config) ConfigSettingsEngineToggles {
	return ConfigSettingsEngineToggles{
		"engine_inbound_detect": cfg.InboundDetectEnabled,
		"engine_session_detect": cfg.SessionDetectEnabled,
		"engine_llm_detect":     cfg.LLMDetectEnabled,
		"engine_semantic":       cfg.SemanticDetector.Enabled,
		"engine_honeypot":       cfg.Honeypot.Enabled,
		"engine_honeypot_deep":  cfg.HoneypotDeep.Enabled,
		"engine_singularity":    cfg.Singularity.Enabled,
		"engine_ifc":            cfg.IFC.Enabled,
		"engine_ifc_quarantine": cfg.IFC.QuarantineEnabled,
		"engine_ifc_hiding":     cfg.IFC.HidingEnabled,
		"engine_path_policy":    cfg.PathPolicy.Enabled,
		"engine_tool_policy":    cfg.ToolPolicy.Enabled,
		"engine_plan_compiler":  cfg.PlanCompiler.Enabled,
		"engine_capability":     cfg.Capability.Enabled,
		"engine_deviation":      cfg.Deviation.Enabled,
		"engine_counterfactual": cfg.Counterfactual.Enabled,
		"engine_envelope":       cfg.EnvelopeEnabled,
		"engine_evolution":      cfg.EvolutionEnabled,
		"engine_adaptive":       cfg.AdaptiveDecision.Enabled,
		"engine_taint_tracker":  cfg.TaintTracker.Enabled,
		"engine_taint_reversal": cfg.TaintReversal.Enabled,
		"engine_event_bus":      cfg.EventBus.Enabled,
		"engine_human_confirm":  cfg.HumanConfirm.Enabled,
	}
}

// buildConfigSettingsResponse 返回合并响应：兼容旧 PascalCase 字段（如 RateLimit、InboundListen）
// + 新 snake_case DTO 字段（如 rate_limit、basic、engine_toggles）。
// 两套字段并存是有意为之，供前端平滑迁移；待 UI 完全切换到 DTO 字段后可移除 legacy 部分。
func buildConfigSettingsResponse(cfg *Config) (map[string]interface{}, error) {
	legacy, err := structToMap(cfg)
	if err != nil {
		return nil, err
	}
	dto, err := structToMap(buildConfigSettingsDTO(cfg))
	if err != nil {
		return nil, err
	}
	for key, value := range dto {
		legacy[key] = value
	}
	return legacy, nil
}

func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
