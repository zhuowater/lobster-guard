// config_security_test.go — 配置安全加固测试
// lobster-guard v18.2
package main

import (
	"strings"
	"testing"
)

// TestMaskSensitiveConfig_HidesSecrets 验证脱敏导出隐藏敏感字段
func TestMaskSensitiveConfig_HidesSecrets(t *testing.T) {
	cfg := &Config{
		ManagementToken:     "super-secret-token-12345678",
		CallbackKey:         "my-callback-key-base64==",
		CallbackSignToken:   "my-sign-token-abc123",
		ConfigEncryptionKey: "my-encryption-key-long-enough",
		InboundListen:       ":8443",
		OutboundListen:      ":8444",
		ManagementListen:    ":9090",
		DBPath:              "/var/lib/lobster-guard/audit.db",
	}
	cfg.Auth = AuthConfig{
		Enabled:   true,
		JWTSecret: "jwt-secret-32-characters-long!!!!!",
	}

	masked := MaskSensitiveConfig(cfg)

	// 检查敏感字段被脱敏
	checkMasked := func(key, original string) {
		t.Helper()
		val, ok := masked[key]
		if !ok {
			t.Errorf("脱敏结果缺少字段 %s", key)
			return
		}
		strVal, ok := val.(string)
		if !ok {
			return // 非字符串可以跳过
		}
		if strVal == original && original != "" {
			t.Errorf("字段 %s 未被脱敏，原始值: %s", key, original)
		}
		if original != "" && !strings.Contains(strVal, "***") {
			t.Errorf("字段 %s 脱敏格式不正确: %s", key, strVal)
		}
	}

	checkMasked("management_token", "super-secret-token-12345678")
	checkMasked("callbackKey", "my-callback-key-base64==")
	checkMasked("callbackSignToken", "my-sign-token-abc123")
	checkMasked("config_encryption_key", "my-encryption-key-long-enough")

	// 检查非敏感字段未被脱敏
	if v, ok := masked["inbound_listen"].(string); !ok || v != ":8443" {
		t.Errorf("非敏感字段 inbound_listen 被意外修改: %v", masked["inbound_listen"])
	}
	if v, ok := masked["db_path"].(string); !ok || v != "/var/lib/lobster-guard/audit.db" {
		t.Errorf("非敏感字段 db_path 被意外修改: %v", masked["db_path"])
	}

	// 检查嵌套结构中的敏感字段
	authMap, ok := masked["auth"].(map[string]interface{})
	if !ok {
		t.Fatal("auth 字段未返回为 map")
	}
	jwtVal, ok := authMap["jwt_secret"].(string)
	if !ok {
		t.Fatal("auth.jwt_secret 未返回为 string")
	}
	if !strings.Contains(jwtVal, "***") {
		t.Errorf("auth.jwt_secret 未被脱敏: %s", jwtVal)
	}
}

// TestMaskSensitiveConfig_EmptyValues 验证空值不会触发脱敏
func TestMaskSensitiveConfig_EmptyValues(t *testing.T) {
	cfg := &Config{}
	masked := MaskSensitiveConfig(cfg)

	// 空的 token 字段应为空字符串
	if val, ok := masked["management_token"].(string); ok && val != "" {
		t.Errorf("空的 management_token 应返回空字符串，实际: %s", val)
	}
}

// TestMaskSensitiveConfig_NilConfig 验证 nil 配置不会 panic
func TestMaskSensitiveConfig_NilConfig(t *testing.T) {
	result := MaskSensitiveConfig(nil)
	if len(result) != 0 {
		t.Errorf("nil 配置应返回空 map，实际长度: %d", len(result))
	}
}

// TestValidateConfigSecurity_AllGood 验证合法配置无警告
func TestValidateConfigSecurity_AllGood(t *testing.T) {
	cfg := &Config{
		ManagementToken:  "this-is-a-long-enough-token-16ch",
		InboundListen:    ":8443",
		OutboundListen:   ":8444",
		ManagementListen: ":9090",
		DBPath:           "/var/lib/lobster-guard/audit.db",
		Auth: AuthConfig{
			Enabled:         true,
			JWTSecret:       "this-jwt-secret-must-be-32-chars!!",
			DefaultPassword: "strong-password-123",
		},
	}

	issues := ValidateConfigSecurity(cfg)
	if len(issues) != 0 {
		t.Errorf("合法配置不应有问题，但发现 %d 个: %v", len(issues), issues)
	}
}

// TestValidateConfigSecurity_WeakSecrets 验证弱密钥被检出
func TestValidateConfigSecurity_WeakSecrets(t *testing.T) {
	cfg := &Config{
		ManagementToken: "short", // < 16 chars
		InboundListen:   ":8443",
		OutboundListen:  ":8444",
		DBPath:          "/var/lib/lobster-guard/audit.db",
		Auth: AuthConfig{
			Enabled:         true,
			JWTSecret:       "too-short",       // < 32 chars
			DefaultPassword: "weak",             // < 8 chars
		},
	}

	issues := ValidateConfigSecurity(cfg)
	if len(issues) == 0 {
		t.Fatal("弱密钥配置应检出问题")
	}

	// 验证包含预期的问题
	found := map[string]bool{
		"management_token": false,
		"jwt_secret":       false,
		"default_password": false,
	}
	for _, issue := range issues {
		lower := strings.ToLower(issue)
		if strings.Contains(lower, "management_token") {
			found["management_token"] = true
		}
		if strings.Contains(lower, "jwt_secret") {
			found["jwt_secret"] = true
		}
		if strings.Contains(lower, "default_password") || strings.Contains(lower, "password") {
			found["default_password"] = true
		}
	}
	for key, found := range found {
		if !found {
			t.Errorf("应检出 %s 的问题", key)
		}
	}
}

// TestValidateConfigSecurity_PortConflict 验证端口冲突被检出
func TestValidateConfigSecurity_PortConflict(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":9090",
		OutboundListen:   ":8444",
		ManagementListen: ":9090", // 与 InboundListen 冲突
		DBPath:           "/var/lib/lobster-guard/audit.db",
	}

	issues := ValidateConfigSecurity(cfg)
	foundConflict := false
	for _, issue := range issues {
		if strings.Contains(issue, "端口冲突") {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Errorf("应检出端口冲突，实际问题: %v", issues)
	}
}

// TestValidateConfigSecurity_EnvelopeNoKey 验证信封启用但无密钥被检出
func TestValidateConfigSecurity_EnvelopeNoKey(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8443",
		OutboundListen:   ":8444",
		ManagementListen: ":9090",
		DBPath:           "/var/lib/lobster-guard/audit.db",
		EnvelopeEnabled:  true,
		EnvelopeSecretKey: "",
	}

	issues := ValidateConfigSecurity(cfg)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "envelope_secret_key") || strings.Contains(issue, "信封") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("应检出信封密钥缺失，实际问题: %v", issues)
	}
}

// TestValidateConfigSecurity_NilConfig 验证 nil 配置
func TestValidateConfigSecurity_NilConfig(t *testing.T) {
	issues := ValidateConfigSecurity(nil)
	if len(issues) != 1 || issues[0] != "配置为 nil" {
		t.Errorf("nil 配置应返回 '配置为 nil'，实际: %v", issues)
	}
}

// TestValidateConfigSecurity_DefaultPlaceholder 验证默认占位符被检出
func TestValidateConfigSecurity_DefaultPlaceholder(t *testing.T) {
	cfg := &Config{
		CallbackKey:       "YOUR_CALLBACK_KEY_BASE64",
		CallbackSignToken: "YOUR_CALLBACK_SIGN_TOKEN",
		InboundListen:     ":8443",
		OutboundListen:    ":8444",
		ManagementListen:  ":9090",
		DBPath:            "/var/lib/lobster-guard/audit.db",
	}

	issues := ValidateConfigSecurity(cfg)
	foundCallback := false
	foundSign := false
	for _, issue := range issues {
		if strings.Contains(issue, "callbackKey") {
			foundCallback = true
		}
		if strings.Contains(issue, "callbackSignToken") {
			foundSign = true
		}
	}
	if !foundCallback {
		t.Error("应检出 callbackKey 使用默认值")
	}
	if !foundSign {
		t.Error("应检出 callbackSignToken 使用默认值")
	}
}

// TestIsSensitiveField 验证敏感字段判断
func TestIsSensitiveField(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{"management_token", true},
		{"jwt_secret", true},
		{"default_password", true},
		{"api_key", true},
		{"config_encryption_key", true},
		{"inbound_listen", false},
		{"db_path", false},
		{"log_level", false},
		{"channel", false},
	}
	for _, tc := range cases {
		got := isSensitiveField(tc.name)
		if got != tc.expected {
			t.Errorf("isSensitiveField(%q) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

// TestMaskValue 验证脱敏值格式
func TestMaskValue(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "***"},
		{"abcd", "***"},
		{"abcde", "ab***de"},
		{"super-secret-token", "su***en"},
	}
	for _, tc := range cases {
		got := maskValue(tc.input)
		if got != tc.expected {
			t.Errorf("maskValue(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// TestValidateConfigSecurity_LLMProxy 验证 LLM 代理配置校验
func TestValidateConfigSecurity_LLMProxy(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8443",
		OutboundListen:   ":8444",
		ManagementListen: ":9090",
		DBPath:           "/var/lib/lobster-guard/audit.db",
		LLMProxy: LLMProxyConfig{
			Enabled: true,
			Listen:  ":8445",
			Targets: []LLMTargetConfig{}, // 空 targets
		},
	}

	issues := ValidateConfigSecurity(cfg)
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "target") || strings.Contains(issue, "LLM") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("应检出 LLM 代理无 target，实际问题: %v", issues)
	}
}
