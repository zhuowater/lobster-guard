// config_security.go — 配置安全加固：脱敏导出、配置校验
// lobster-guard v18.2 工程化基础
package main

import (
	"fmt"
	"reflect"
	"strings"
)

// sensitiveFieldNames 敏感字段关键词列表（不区分大小写匹配）
var sensitiveFieldNames = []string{
	"secret", "password", "token", "api_key", "apikey",
	"aes_key", "encrypt_key", "encryption_key",
	"callbackkey", "callbacksigntoken",
}

// isSensitiveField 判断字段名是否为敏感字段
func isSensitiveField(name string) bool {
	lower := strings.ToLower(name)
	for _, kw := range sensitiveFieldNames {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// maskValue 对敏感值进行脱敏处理
func maskValue(val string) string {
	if val == "" {
		return ""
	}
	if len(val) <= 4 {
		return "***"
	}
	return val[:2] + "***" + val[len(val)-2:]
}

// MaskSensitiveConfig 脱敏导出配置，隐藏 secret/password/token 等字段
// 返回 map[string]interface{} 结构的配置镜像
func MaskSensitiveConfig(cfg *Config) map[string]interface{} {
	if cfg == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{})
	v := reflect.ValueOf(*cfg)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// 获取 yaml tag 作为 key
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			yamlTag = field.Name
		}
		// 去掉 tag 中的选项
		if idx := strings.Index(yamlTag, ","); idx != -1 {
			yamlTag = yamlTag[:idx]
		}

		// 如果是敏感字段且为字符串类型，脱敏处理
		if isSensitiveField(yamlTag) && fv.Kind() == reflect.String {
			result[yamlTag] = maskValue(fv.String())
			continue
		}

		// 对嵌套结构进行递归脱敏
		switch fv.Kind() {
		case reflect.Struct:
			result[yamlTag] = maskStructFields(fv)
		case reflect.Ptr:
			if fv.IsNil() {
				result[yamlTag] = nil
			} else {
				result[yamlTag] = fv.Elem().Interface()
			}
		default:
			result[yamlTag] = fv.Interface()
		}
	}
	return result
}

// maskStructFields 递归脱敏嵌套结构体
func maskStructFields(v reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			yamlTag = field.Name
		}
		if idx := strings.Index(yamlTag, ","); idx != -1 {
			yamlTag = yamlTag[:idx]
		}

		if isSensitiveField(yamlTag) && fv.Kind() == reflect.String {
			result[yamlTag] = maskValue(fv.String())
			continue
		}

		switch fv.Kind() {
		case reflect.Struct:
			result[yamlTag] = maskStructFields(fv)
		default:
			result[yamlTag] = fv.Interface()
		}
	}
	return result
}

// ValidateConfigSecurity 配置安全校验（检查 secret 长度、端口冲突、必填项等）
// 返回问题列表，空列表表示通过
func ValidateConfigSecurity(cfg *Config) []string {
	if cfg == nil {
		return []string{"配置为 nil"}
	}
	var issues []string

	// 1. management_token 安全性检查
	if cfg.ManagementToken == "" {
		issues = append(issues, "management_token 未配置，管理 API 无认证保护")
	} else if len(cfg.ManagementToken) < 16 {
		issues = append(issues, fmt.Sprintf("management_token 长度不足（当前 %d 字符，建议至少 16 字符）", len(cfg.ManagementToken)))
	}

	// 2. JWT secret 检查（如果启用了认证）
	if cfg.Auth.Enabled {
		if cfg.Auth.JWTSecret == "" {
			issues = append(issues, "认证已启用但 jwt_secret 未配置")
		} else if len(cfg.Auth.JWTSecret) < 32 {
			issues = append(issues, fmt.Sprintf("jwt_secret 长度不足（当前 %d 字符，建议至少 32 字符）", len(cfg.Auth.JWTSecret)))
		}
		if cfg.Auth.DefaultPassword == "" {
			issues = append(issues, "认证已启用但 default_password 未配置")
		} else if len(cfg.Auth.DefaultPassword) < 8 {
			issues = append(issues, "default_password 过短，建议至少 8 字符")
		}
	}

	// 3. 端口冲突检查（增强版）
	portMap := map[string]string{}
	portChecks := [][2]string{
		{cfg.InboundListen, "inbound_listen"},
		{cfg.OutboundListen, "outbound_listen"},
		{cfg.ManagementListen, "management_listen"},
	}
	if cfg.LLMProxy.Enabled && cfg.LLMProxy.Listen != "" {
		portChecks = append(portChecks, [2]string{cfg.LLMProxy.Listen, "llm_proxy.listen"})
	}
	for _, pair := range portChecks {
		addr, name := pair[0], pair[1]
		if addr == "" {
			continue
		}
		if prev, ok := portMap[addr]; ok {
			issues = append(issues, fmt.Sprintf("端口冲突: %s 和 %s 使用了相同的地址 %s", prev, name, addr))
		}
		portMap[addr] = name
	}

	// 4. 信封签名密钥检查
	if cfg.EnvelopeEnabled && cfg.EnvelopeSecretKey == "" {
		issues = append(issues, "执行信封已启用但 envelope_secret_key 未配置")
	}

	// 5. LLM 代理安全检查
	if cfg.LLMProxy.Enabled {
		if len(cfg.LLMProxy.Targets) == 0 {
			issues = append(issues, "LLM 代理已启用但未配置任何 target")
		}
	}

	// 6. 配置加密密钥检查
	if cfg.ConfigEncryptionKey != "" && len(cfg.ConfigEncryptionKey) < 16 {
		issues = append(issues, fmt.Sprintf("config_encryption_key 长度不足（当前 %d 字符，建议至少 16 字符）", len(cfg.ConfigEncryptionKey)))
	}

	// 7. 回调密钥安全检查
	if cfg.CallbackKey != "" && cfg.CallbackKey == "YOUR_CALLBACK_KEY_BASE64" {
		issues = append(issues, "callbackKey 使用了示例默认值，请替换为实际密钥")
	}
	if cfg.CallbackSignToken != "" && cfg.CallbackSignToken == "YOUR_CALLBACK_SIGN_TOKEN" {
		issues = append(issues, "callbackSignToken 使用了示例默认值，请替换为实际密钥")
	}

	// 8. 数据库路径检查
	if cfg.DBPath == "" {
		issues = append(issues, "db_path 未配置")
	}

	return issues
}
