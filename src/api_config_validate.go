package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type configValidateRequest struct {
	YAML string `json:"yaml"`
}

type configFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func detectConfigLevel(cfg *Config, raw map[string]interface{}) string {
	if cfg == nil {
		return "L0"
	}
	level := "L0"
	if cfg.Channel != "" || cfg.CallbackKey != "" || cfg.CallbackSignToken != "" || cfg.ConfigEncryptionKey != "" || cfg.InboundDetectEnabled || cfg.OutboundAuditEnabled {
		level = "L1"
	}
	if cfg.LLMProxy.Enabled || cfg.IFC.Enabled || cfg.PlanCompiler.Enabled || cfg.Capability.Enabled || cfg.Deviation.Enabled || cfg.Counterfactual.Enabled || false {
		return "L2"
	}
	for _, k := range []string{"llm_proxy", "ifc", "camel", "tenants", "api_keys", "plan_compiler", "capability", "deviation", "counterfactual"} {
		if _, ok := raw[k]; ok {
			return "L2"
		}
	}
	return level
}

func validateConfigTemplate(cfg *Config, raw map[string]interface{}) (missing []string, invalid []configFieldError) {
	if cfg == nil {
		return []string{"yaml"}, []configFieldError{{Field: "yaml", Message: "配置为空"}}
	}

	if strings.TrimSpace(cfg.InboundListen) == "" {
		missing = append(missing, "listen_inbound")
	}
	if strings.TrimSpace(cfg.OutboundListen) == "" {
		missing = append(missing, "listen_outbound")
	}
	if cfg.ManagementListen == "" {
		missing = append(missing, "management_port")
	}
	if strings.TrimSpace(cfg.ManagementToken) == "" {
		missing = append(missing, "management_token")
	}
	if len(cfg.StaticUpstreams) == 0 {
		missing = append(missing, "upstreams")
	}

	portish := regexp.MustCompile(`^:?[0-9]{1,5}$`)
	if cfg.InboundListen != "" && !portish.MatchString(cfg.InboundListen) {
		invalid = append(invalid, configFieldError{Field: "listen_inbound", Message: "应为 :18443 或 18443 格式"})
	}
	if cfg.OutboundListen != "" && !portish.MatchString(cfg.OutboundListen) {
		invalid = append(invalid, configFieldError{Field: "listen_outbound", Message: "应为 :18444 或 18444 格式"})
	}
	if cfg.ManagementListen != "" && !portish.MatchString(cfg.ManagementListen) {
		invalid = append(invalid, configFieldError{Field: "management_port", Message: "应为 :9090 或 9090 格式"})
	}
	if cfg.ManagementToken != "" && len(cfg.ManagementToken) < 8 {
		invalid = append(invalid, configFieldError{Field: "management_token", Message: "长度至少 8 字符"})
	}
	if len(cfg.StaticUpstreams) > 0 {
		for i, up := range cfg.StaticUpstreams {
			prefix := fmt.Sprintf("upstreams[%d]", i)
			if strings.TrimSpace(up.ID) == "" {
				invalid = append(invalid, configFieldError{Field: prefix + ".id", Message: "不能为空"})
			}
			if strings.TrimSpace(up.Address) == "" {
				invalid = append(invalid, configFieldError{Field: prefix + ".address", Message: "不能为空"})
			}
			if up.Port <= 0 || up.Port > 65535 {
				invalid = append(invalid, configFieldError{Field: prefix + ".port", Message: "端口必须在 1-65535 之间"})
			}
		}
	}
	if cfg.Channel != "" {
		switch cfg.Channel {
		case "lanxin", "feishu", "dingtalk", "wecom", "generic":
		default:
			invalid = append(invalid, configFieldError{Field: "channel_type", Message: "必须是 lanxin/feishu/dingtalk/wecom/generic"})
		}
	}
	if _, ok := raw["encryption_key"]; ok && strings.TrimSpace(cfg.ConfigEncryptionKey) == "" {
		invalid = append(invalid, configFieldError{Field: "encryption_key", Message: "不能为空"})
	}
	return
}

func normalizeWizardAliases(raw map[string]interface{}) {
	if v, ok := raw["listen_inbound"]; ok {
		raw["inbound_listen"] = v
	}
	if v, ok := raw["listen_outbound"]; ok {
		raw["outbound_listen"] = v
	}
	if v, ok := raw["management_port"]; ok {
		switch vv := v.(type) {
		case int, int64, float64:
			raw["management_listen"] = fmt.Sprintf(":%v", trimFloat(fmt.Sprintf("%v", vv)))
		case string:
			if strings.HasPrefix(vv, ":") {
				raw["management_listen"] = vv
			} else {
				raw["management_listen"] = ":" + vv
			}
		}
	}
	if v, ok := raw["channel_type"]; ok {
		raw["channel"] = v
	}
	if v, ok := raw["upstreams"]; ok {
		raw["static_upstreams"] = v
	}
}

func trimFloat(s string) string {
	return strings.TrimSuffix(strings.TrimSuffix(s, ".0"), ".")
}

func (api *ManagementAPI) handleConfigValidatePost(w http.ResponseWriter, r *http.Request) {
	var req configValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		jsonResponse(w, 400, map[string]string{"error": "yaml is required"})
		return
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &raw); err != nil {
		jsonResponse(w, 200, map[string]interface{}{
			"valid":            false,
			"parse_error":      err.Error(),
			"missing_required": []string{},
			"invalid_fields":   []configFieldError{{Field: "yaml", Message: err.Error()}},
			"level":            "L0",
		})
		return
	}
	normalizeWizardAliases(raw)
	buf, _ := yaml.Marshal(raw)
	cfg := &Config{}
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		jsonResponse(w, 200, map[string]interface{}{
			"valid":            false,
			"parse_error":      err.Error(),
			"missing_required": []string{},
			"invalid_fields":   []configFieldError{{Field: "yaml", Message: err.Error()}},
			"level":            "L0",
		})
		return
	}

	missing, invalid := validateConfigTemplate(cfg, raw)
	baseIssues := validateConfig(cfg)
	securityIssues := ValidateConfigSecurity(cfg)
	issues := append([]string{}, baseIssues...)
	issues = append(issues, securityIssues...)

	jsonResponse(w, 200, map[string]interface{}{
		"valid":            len(missing) == 0 && len(invalid) == 0 && len(issues) == 0,
		"missing_required": missing,
		"invalid_fields":   invalid,
		"level":            detectConfigLevel(cfg, raw),
		"issues":           issues,
	})
}
