package main

import (
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type SourceDescriptor struct {
	SourceKey       string     `json:"source_key"`
	BaseTool        string     `json:"base_tool"`
	URL             string     `json:"url,omitempty"`
	Host            string     `json:"host,omitempty"`
	Path            string     `json:"path,omitempty"`
	Method          string     `json:"method,omitempty"`
	Category        string     `json:"category"`
	Confidentiality IFCLevel   `json:"confidentiality"`
	Integrity       IntegLevel `json:"integrity"`
	TrustScore      float64    `json:"trust_score"`
	AuthType        string     `json:"auth_type,omitempty"`
	PrivateNetwork  bool       `json:"private_network"`
	Suspicious      bool       `json:"suspicious"`
	Tags            []string   `json:"tags,omitempty"`
	Evidence        []string   `json:"evidence,omitempty"`
}

type ToolSourceRule struct {
	Name            string     `json:"name" yaml:"name"`
	ToolPattern     string     `json:"tool_pattern,omitempty" yaml:"tool_pattern,omitempty"`
	HostPattern     string     `json:"host_pattern,omitempty" yaml:"host_pattern,omitempty"`
	PathPattern     string     `json:"path_pattern,omitempty" yaml:"path_pattern,omitempty"`
	MethodPattern   string     `json:"method_pattern,omitempty" yaml:"method_pattern,omitempty"`
	AuthTypePattern string     `json:"auth_type_pattern,omitempty" yaml:"auth_type_pattern,omitempty"`
	Category        string     `json:"category" yaml:"category"`
	Confidentiality IFCLevel   `json:"confidentiality" yaml:"confidentiality"`
	Integrity       IntegLevel `json:"integrity" yaml:"integrity"`
	TrustScore      float64    `json:"trust_score" yaml:"trust_score"`
	PrivateNetwork  *bool      `json:"private_network,omitempty" yaml:"private_network,omitempty"`
	Suspicious      *bool      `json:"suspicious,omitempty" yaml:"suspicious,omitempty"`
	Tags            []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type ToolSourceClassifierConfig struct {
	Rules []ToolSourceRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

type ToolSourceClassifier struct {
	config ToolSourceClassifierConfig
}

var (
	defaultToolSourceClassifierMu     sync.RWMutex
	defaultToolSourceClassifierConfig ToolSourceClassifierConfig
)

func NewToolSourceClassifier() *ToolSourceClassifier {
	defaultToolSourceClassifierMu.RLock()
	cfg := defaultToolSourceClassifierConfig
	defaultToolSourceClassifierMu.RUnlock()
	return &ToolSourceClassifier{config: cfg}
}

func NewToolSourceClassifierWithConfig(config ToolSourceClassifierConfig) *ToolSourceClassifier {
	return &ToolSourceClassifier{config: config}
}

func SetDefaultToolSourceClassifierConfig(config ToolSourceClassifierConfig) {
	defaultToolSourceClassifierMu.Lock()
	defaultToolSourceClassifierConfig = config
	defaultToolSourceClassifierMu.Unlock()
}

func LoadToolSourceClassifierConfigYAML(raw string) (ToolSourceClassifierConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return ToolSourceClassifierConfig{}, nil
	}
	var cfg ToolSourceClassifierConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return ToolSourceClassifierConfig{}, err
	}
	return cfg, nil
}

func MergeToolSourceClassifierConfigs(primary, fallback ToolSourceClassifierConfig) ToolSourceClassifierConfig {
	merged := ToolSourceClassifierConfig{Rules: append([]ToolSourceRule{}, primary.Rules...)}
	merged.Rules = append(merged.Rules, fallback.Rules...)
	return merged
}

func classifyToolSourceForTenant(tenantMgr *TenantManager, tenantID, toolName, toolArgs string) *SourceDescriptor {
	desc, _ := explainToolSourceForTenant(tenantMgr, tenantID, toolName, toolArgs)
	return desc
}

func explainToolSourceForTenant(tenantMgr *TenantManager, tenantID, toolName, toolArgs string) (*SourceDescriptor, *ToolSourceRule) {
	classifier := NewToolSourceClassifier()
	if tenantMgr == nil || tenantID == "" || tenantID == "default" {
		return classifier.Explain(toolName, toolArgs)
	}
	tcfg := tenantMgr.GetConfig(tenantID)
	if tcfg == nil || strings.TrimSpace(tcfg.SourceClassifierYAML) == "" {
		return classifier.Explain(toolName, toolArgs)
	}
	override, err := LoadToolSourceClassifierConfigYAML(tcfg.SourceClassifierYAML)
	if err != nil {
		return classifier.Explain(toolName, toolArgs)
	}
	merged := MergeToolSourceClassifierConfigs(override, defaultToolSourceClassifierConfig)
	return NewToolSourceClassifierWithConfig(merged).Explain(toolName, toolArgs)
}

func (c *ToolSourceClassifier) Classify(toolName, toolArgs string) *SourceDescriptor {
	desc, _ := c.Explain(toolName, toolArgs)
	return desc
}

func (c *ToolSourceClassifier) Explain(toolName, toolArgs string) (*SourceDescriptor, *ToolSourceRule) {
	desc := &SourceDescriptor{
		SourceKey:       "tool:" + toolName,
		BaseTool:        toolName,
		Category:        "unknown",
		Confidentiality: ConfInternal,
		Integrity:       IntegLow,
		TrustScore:      0.3,
		Tags:            []string{},
		Evidence:        []string{},
	}

	meta := parseToolArgsMeta(toolArgs)
	if meta.Method != "" {
		desc.Method = meta.Method
	}
	desc.AuthType = meta.AuthType
	if meta.URL == "" {
		return desc, nil
	}

	desc.URL = meta.URL
	parsed, err := url.Parse(meta.URL)
	if err != nil {
		desc.Evidence = append(desc.Evidence, "invalid_url")
		return desc, nil
	}

	host := strings.ToLower(parsed.Hostname())
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	desc.Host = host
	desc.Path = path
	desc.PrivateNetwork = isPrivateOrLocalHost(host)
	if desc.PrivateNetwork {
		desc.Tags = append(desc.Tags, "private_network")
	}
	if meta.AuthType != "" {
		desc.Tags = append(desc.Tags, "auth:"+meta.AuthType)
	}
	if matchedRule, applied := c.applyConfigRuleDetailed(toolName, desc); applied {
		desc.SourceKey = buildSourceKey(toolName, desc)
		return desc, matchedRule
	}

	switch {
	case isMetadataService(host, path):
		desc.Category = "metadata_service"
		desc.Confidentiality = ConfSecret
		desc.Integrity = IntegLow
		desc.TrustScore = 0.3
		desc.Suspicious = true
		desc.Tags = append(desc.Tags, "metadata_service")
		desc.Evidence = append(desc.Evidence, "metadata_host_or_path")
	case isPublicWebHost(host) && meta.AuthType == "none":
		desc.Category = "public_web"
		desc.Confidentiality = ConfPublic
		desc.Integrity = IntegTaint
		desc.TrustScore = 0.25
		desc.Evidence = append(desc.Evidence, "public_web_host")
	case desc.PrivateNetwork:
		desc.Category = "internal_api"
		desc.Confidentiality = ConfConfidential
		desc.Integrity = IntegLow
		desc.TrustScore = 0.6
		desc.Evidence = append(desc.Evidence, "private_network_host")
	case looksLikeAPIHost(host) || strings.Contains(path, "/api/") || meta.AuthType != "none":
		desc.Category = "external_api"
		desc.Confidentiality = ConfInternal
		desc.Integrity = IntegLow
		desc.TrustScore = 0.5
		desc.Evidence = append(desc.Evidence, "api_or_authenticated_endpoint")
	default:
		desc.Category = "unknown_url"
		desc.Confidentiality = ConfInternal
		desc.Integrity = IntegLow
		desc.TrustScore = 0.3
		desc.Evidence = append(desc.Evidence, "url_present_but_unclassified")
	}

	desc.SourceKey = buildSourceKey(toolName, desc)
	return desc, nil
}

type toolArgsMeta struct {
	URL      string
	Method   string
	AuthType string
}

func parseToolArgsMeta(toolArgs string) toolArgsMeta {
	meta := toolArgsMeta{AuthType: "none"}
	var m map[string]interface{}
	if json.Unmarshal([]byte(toolArgs), &m) != nil {
		return meta
	}
	meta.URL = findFirstURL(m)
	meta.Method = strings.ToUpper(findStringField(m, "method", "http_method"))
	meta.AuthType = detectAuthType(m)
	return meta
}

func findFirstURL(v interface{}) string {
	switch vv := v.(type) {
	case map[string]interface{}:
		for _, key := range []string{"url", "uri", "endpoint", "api_url", "base_url", "resource", "link", "href", "target", "webhook", "callback_url"} {
			if s, ok := vv[key].(string); ok && looksLikeURL(s) {
				return s
			}
		}
		for _, child := range vv {
			if s := findFirstURL(child); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, child := range vv {
			if s := findFirstURL(child); s != "" {
				return s
			}
		}
	case string:
		if looksLikeURL(vv) {
			return vv
		}
	}
	return ""
}

func detectAuthType(m map[string]interface{}) string {
	if headers, ok := m["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			key := strings.ToLower(k)
			val, _ := v.(string)
			if key == "authorization" {
				l := strings.ToLower(val)
				switch {
				case strings.HasPrefix(l, "bearer "):
					return "bearer"
				case strings.HasPrefix(l, "basic "):
					return "basic"
				default:
					return "authorization"
				}
			}
			if strings.Contains(key, "api-key") || strings.Contains(key, "apikey") || strings.Contains(key, "x-api-key") {
				return "api_key"
			}
		}
	}
	for _, key := range []string{"api_key", "apikey", "token", "access_token"} {
		if s, ok := m[key].(string); ok && s != "" {
			if strings.Contains(key, "api") {
				return "api_key"
			}
			return "token"
		}
	}
	return "none"
}

func findStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func (c *ToolSourceClassifier) applyConfigRule(toolName string, desc *SourceDescriptor) bool {
	_, applied := c.applyConfigRuleDetailed(toolName, desc)
	return applied
}

func (c *ToolSourceClassifier) applyConfigRuleDetailed(toolName string, desc *SourceDescriptor) (*ToolSourceRule, bool) {
	for _, rule := range c.config.Rules {
		if !toolSourceRuleMatches(rule, toolName, desc) {
			continue
		}
		if rule.Category != "" {
			desc.Category = rule.Category
		}
		desc.Confidentiality = rule.Confidentiality
		desc.Integrity = rule.Integrity
		if rule.TrustScore > 0 {
			desc.TrustScore = rule.TrustScore
		}
		if rule.PrivateNetwork != nil {
			desc.PrivateNetwork = *rule.PrivateNetwork
		}
		if rule.Suspicious != nil {
			desc.Suspicious = *rule.Suspicious
		}
		if len(rule.Tags) > 0 {
			desc.Tags = append(desc.Tags, rule.Tags...)
		}
		desc.Evidence = append(desc.Evidence, "config_rule:"+rule.Name)
		matched := rule
		return &matched, true
	}
	return nil, false
}

func toolSourceRuleMatches(rule ToolSourceRule, toolName string, desc *SourceDescriptor) bool {
	if rule.ToolPattern != "" && !regexMatches(rule.ToolPattern, toolName) {
		return false
	}
	if rule.HostPattern != "" && !regexMatches(rule.HostPattern, desc.Host) {
		return false
	}
	if rule.PathPattern != "" && !regexMatches(rule.PathPattern, desc.Path) {
		return false
	}
	if rule.MethodPattern != "" && !regexMatches(rule.MethodPattern, desc.Method) {
		return false
	}
	if rule.AuthTypePattern != "" && !regexMatches(rule.AuthTypePattern, desc.AuthType) {
		return false
	}
	return rule.ToolPattern != "" || rule.HostPattern != "" || rule.PathPattern != "" || rule.MethodPattern != "" || rule.AuthTypePattern != ""
}

func regexMatches(pattern, value string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func looksLikeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func isPublicWebHost(host string) bool {
	if host == "" {
		return false
	}
	publicHints := []string{"docs.", "developer.", "wikipedia.org", "python.org", "mozilla.org", "golang.org", "readthedocs.io", "medium.com", "github.com"}
	for _, hint := range publicHints {
		if strings.HasPrefix(host, hint) || strings.Contains(host, hint) {
			return true
		}
	}
	return false
}

func looksLikeAPIHost(host string) bool {
	return strings.HasPrefix(host, "api.") || strings.Contains(host, ".api.")
}

func isPrivateOrLocalHost(host string) bool {
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".corp") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".svc.cluster.local")
	}
	privateCIDRs := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "169.254.0.0/16"}
	for _, cidr := range privateCIDRs {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func isMetadataService(host, path string) bool {
	if host == "169.254.169.254" {
		return true
	}
	pathLower := strings.ToLower(path)
	return strings.Contains(pathLower, "meta-data") || strings.Contains(pathLower, "/metadata") || strings.Contains(pathLower, "security-credentials")
}

func buildSourceKey(tool string, desc *SourceDescriptor) string {
	if desc == nil || desc.Category == "" {
		return "tool:" + tool
	}
	switch desc.Category {
	case "public_web", "metadata_service", "internal_api", "external_api", "unknown", "unknown_url":
		if desc.Host != "" && (desc.Category == "internal_api" || desc.Category == "external_api") {
			return "tool:" + tool + ":" + desc.Category + ":" + desc.Host
		}
		return "tool:" + tool + ":" + desc.Category
	default:
		return "tool:" + tool
	}
}
