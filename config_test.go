// config_test.go — Config 加载、验证测试
// lobster-guard v4.0 代码拆分
package main

import (
	"strings"
	"testing"
)

func TestValidateConfig_PortConflict(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8080",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "端口冲突") {
			found = true
		}
	}
	if !found {
		t.Error("expected port conflict error")
	}
}

func TestValidateConfig_InvalidChannel(t *testing.T) {
	cfg := &Config{
		Channel:          "invalid",
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "无效") {
			found = true
		}
	}
	if !found {
		t.Error("expected invalid channel error")
	}
}

func TestValidateConfig_InvalidMode(t *testing.T) {
	cfg := &Config{
		Mode:             "invalid",
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "mode") {
			found = true
		}
	}
	if !found {
		t.Error("expected invalid mode error")
	}
}

func TestValidateConfig_BridgeNeedsCredentials(t *testing.T) {
	cfg := &Config{
		Channel:          "feishu",
		Mode:             "bridge",
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "bridge") && strings.Contains(e, "feishu_app_id") {
			found = true
		}
	}
	if !found {
		t.Error("expected bridge credentials error")
	}
}

func TestValidateConfig_EmptyUpstream(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "static_upstreams") {
			found = true
		}
	}
	if !found {
		t.Error("expected empty upstream error")
	}
}

func TestValidateConfig_InvalidRegex(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
		InboundRules: []InboundRuleConfig{
			{Name: "test", Type: "regex", Patterns: []string{"[invalid"}, Action: "block"},
		},
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "正则编译失败") {
			found = true
		}
	}
	if !found {
		t.Error("expected regex compile error")
	}
}

func TestValidateConfig_InvalidPII(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
		OutboundPIIPatterns: []OutboundPIIPatternConfig{
			{Name: "test", Pattern: "[invalid"},
		},
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "PII") && strings.Contains(e, "正则编译失败") {
			found = true
		}
	}
	if !found {
		t.Error("expected PII regex compile error")
	}
}

func TestValidateConfig_NegativeRateLimit(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
		RateLimit:        RateLimiterConfig{GlobalRPS: -1},
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "global_rps") {
			found = true
		}
	}
	if !found {
		t.Error("expected negative rate limit error")
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	cfg := &Config{
		Channel:          "lanxin",
		Mode:             "webhook",
		InboundListen:    ":8080",
		OutboundListen:   ":8081",
		ManagementListen: ":9090",
		StaticUpstreams:  []StaticUpstreamConfig{{ID: "up1", Address: "127.0.0.1", Port: 8080}},
		ManagementToken:  "test",
	}
	errs := validateConfig(cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}
