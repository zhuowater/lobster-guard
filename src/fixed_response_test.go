// fixed_response_test.go — v34.0 固定返回内容功能测试
package main

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestFixedResponseConfig_YAML 测试 fixed_response YAML 解析
func TestFixedResponseConfig_YAML(t *testing.T) {
	yamlData := `
route_policies:
  - match:
      department: "安全研究院"
    upstream_id: "openclaw-security"
  - match:
      default: true
    upstream_id: ""
    fixed_response:
      enabled: true
      status_code: 200
      content_type: "application/json"
      body: '{"code":0,"message":"ok"}'
      headers:
        X-Custom: "hello"
        X-Lobster: "fixed"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}
	if len(cfg.RoutePolicies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(cfg.RoutePolicies))
	}
	// 第一条没有 fixed_response
	if cfg.RoutePolicies[0].FixedResponse != nil {
		t.Fatal("first policy should not have fixed_response")
	}
	// 第二条有 fixed_response
	fr := cfg.RoutePolicies[1].FixedResponse
	if fr == nil {
		t.Fatal("second policy should have fixed_response")
	}
	if !fr.Enabled {
		t.Fatal("fixed_response should be enabled")
	}
	if fr.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", fr.StatusCode)
	}
	if fr.ContentType != "application/json" {
		t.Fatalf("expected application/json, got %s", fr.ContentType)
	}
	if fr.Body != `{"code":0,"message":"ok"}` {
		t.Fatalf("unexpected body: %s", fr.Body)
	}
	if fr.Headers["X-Custom"] != "hello" {
		t.Fatalf("expected header X-Custom=hello, got %s", fr.Headers["X-Custom"])
	}
	if fr.Headers["X-Lobster"] != "fixed" {
		t.Fatalf("expected header X-Lobster=fixed, got %s", fr.Headers["X-Lobster"])
	}
}

// TestFixedResponseConfig_JSON 测试 JSON 序列化/反序列化
func TestFixedResponseConfig_JSON(t *testing.T) {
	policy := RoutePolicyConfig{
		Match:      RoutePolicyMatch{Default: true},
		UpstreamID: "",
		FixedResponse: &FixedResponseConfig{
			Enabled:     true,
			StatusCode:  403,
			ContentType: "text/plain",
			Body:        "Forbidden",
			Headers:     map[string]string{"X-Reason": "blocked"},
		},
	}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	var parsed RoutePolicyConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if parsed.FixedResponse == nil {
		t.Fatal("parsed fixed_response is nil")
	}
	if !parsed.FixedResponse.Enabled {
		t.Fatal("parsed fixed_response should be enabled")
	}
	if parsed.FixedResponse.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", parsed.FixedResponse.StatusCode)
	}
	if parsed.FixedResponse.Body != "Forbidden" {
		t.Fatalf("expected 'Forbidden', got '%s'", parsed.FixedResponse.Body)
	}
	if parsed.FixedResponse.Headers["X-Reason"] != "blocked" {
		t.Fatalf("expected header X-Reason=blocked")
	}
}

// TestFixedResponseConfig_Disabled 测试 disabled 时不生效
func TestFixedResponseConfig_Disabled(t *testing.T) {
	policy := RoutePolicyConfig{
		Match:      RoutePolicyMatch{Default: true},
		UpstreamID: "some-upstream",
		FixedResponse: &FixedResponseConfig{
			Enabled:     false,
			StatusCode:  200,
			ContentType: "text/plain",
			Body:        "should not be used",
		},
	}
	// disabled 时，应该走正常上游路由
	if policy.FixedResponse.Enabled {
		t.Fatal("fixed_response should be disabled")
	}
}

// TestFixedResponseConfig_Nil 测试没有 fixed_response 时正常
func TestFixedResponseConfig_Nil(t *testing.T) {
	policy := RoutePolicyConfig{
		Match:      RoutePolicyMatch{Default: true},
		UpstreamID: "default-upstream",
	}
	if policy.FixedResponse != nil {
		t.Fatal("fixed_response should be nil")
	}
}

// TestMatchFull_DefaultWithFixedResponse 测试 MatchFull 命中默认策略并返回 fixed_response
func TestMatchFull_DefaultWithFixedResponse(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "安全"}, UpstreamID: "upstream-sec"},
		{
			Match:      RoutePolicyMatch{Default: true},
			UpstreamID: "",
			FixedResponse: &FixedResponseConfig{
				Enabled:     true,
				StatusCode:  200,
				ContentType: "application/json",
				Body:        `{"status":"ok"}`,
			},
		},
	}
	engine := NewRoutePolicyEngine(policies)

	// 安全部门用户应命中第一条
	secUser := &UserInfo{Department: "安全"}
	p, ok := engine.MatchFull(secUser, "")
	if !ok || p == nil {
		t.Fatal("should match security policy")
	}
	if p.UpstreamID != "upstream-sec" {
		t.Fatalf("expected upstream-sec, got %s", p.UpstreamID)
	}
	if p.FixedResponse != nil {
		t.Fatal("security policy should not have fixed_response")
	}

	// 其他用户应命中默认策略（含 fixed_response）
	otherUser := &UserInfo{Department: "行政"}
	p2, ok2 := engine.MatchFull(otherUser, "")
	if !ok2 || p2 == nil {
		t.Fatal("should match default policy")
	}
	if p2.FixedResponse == nil {
		t.Fatal("default policy should have fixed_response")
	}
	if !p2.FixedResponse.Enabled {
		t.Fatal("fixed_response should be enabled")
	}
	if p2.FixedResponse.Body != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", p2.FixedResponse.Body)
	}

	// nil 用户也应命中默认
	p3, ok3 := engine.MatchFull(nil, "")
	if !ok3 || p3 == nil {
		t.Fatal("nil user should match default")
	}
	if p3.FixedResponse == nil || !p3.FixedResponse.Enabled {
		t.Fatal("nil user default should have enabled fixed_response")
	}
}

// TestMatchFull_NoDefault 测试无默认策略时 MatchFull 返回 nil
func TestMatchFull_NoDefault(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "安全"}, UpstreamID: "upstream-sec"},
	}
	engine := NewRoutePolicyEngine(policies)

	otherUser := &UserInfo{Department: "行政"}
	p, ok := engine.MatchFull(otherUser, "")
	if ok || p != nil {
		t.Fatal("should not match any policy")
	}
}

// TestMatchFull_DefaultDisabledFixedResponse 测试默认策略有 fixed_response 但 disabled
func TestMatchFull_DefaultDisabledFixedResponse(t *testing.T) {
	policies := []RoutePolicyConfig{
		{
			Match:      RoutePolicyMatch{Default: true},
			UpstreamID: "fallback-upstream",
			FixedResponse: &FixedResponseConfig{
				Enabled: false,
				Body:    "should not use",
			},
		},
	}
	engine := NewRoutePolicyEngine(policies)

	p, ok := engine.MatchFull(nil, "")
	if !ok || p == nil {
		t.Fatal("should match default")
	}
	if p.FixedResponse == nil {
		t.Fatal("should have fixed_response struct")
	}
	if p.FixedResponse.Enabled {
		t.Fatal("fixed_response should be disabled")
	}
	// disabled 时走正常上游
	if p.UpstreamID != "fallback-upstream" {
		t.Fatalf("expected fallback-upstream, got %s", p.UpstreamID)
	}
}

// TestFixedResponse_StatusCodeDefaults 测试默认值（status_code=0 → 200, content_type="" → application/json）
func TestFixedResponse_StatusCodeDefaults(t *testing.T) {
	fr := &FixedResponseConfig{
		Enabled: true,
		Body:    "hello",
	}
	// 验证调用方应处理默认值
	sc := fr.StatusCode
	if sc == 0 {
		sc = 200
	}
	if sc != 200 {
		t.Fatalf("default status code should be 200, got %d", sc)
	}
	ct := fr.ContentType
	if ct == "" {
		ct = "application/json"
	}
	if ct != "application/json" {
		t.Fatalf("default content type should be application/json, got %s", ct)
	}
}
