// detect.go — RuleEngine（AC 自动机+正则）、OutboundRuleEngine、PII 检测、规则绑定
// lobster-guard v4.0 代码拆分 / v28.0 入站模板 CRUD
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 入站规则引擎（v3.5 支持热更新）
// ============================================================

type RuleLevel int
const (
	LevelHigh   RuleLevel = iota
	LevelMedium
	LevelLow
)
type Rule struct { Name string; Level RuleLevel; Category string; Priority int; Message string; Group string; ShadowMode bool; Enabled bool }

// isEnabled 解析 *bool 指针，nil 默认为 true（旧配置兼容）
func isEnabled(p *bool) bool {
	if p == nil { return true }
	return *p
}

// RuleVersion 规则版本信息
type RuleVersion struct {
	Version      int       `json:"version"`
	LoadedAt     time.Time `json:"loaded_at"`
	Source       string    `json:"source"`        // "default" / "config" / "file:/path/to/rules.yaml"
	RuleCount    int       `json:"rule_count"`
	PatternCount int       `json:"pattern_count"`
}

// InboundRuleSummary 入站规则摘要（用于 API 返回）
type InboundRuleSummary struct {
	Name          string `json:"name"`
	DisplayName   string `json:"display_name,omitempty"` // 中文显示名
	PatternsCount int    `json:"patterns_count"`
	Action        string `json:"action"`
	Category      string `json:"category"`
	Priority      int    `json:"priority"`
	Message       string `json:"message,omitempty"`
	Type          string `json:"type,omitempty"`  // v3.11 规则类型
	Group         string `json:"group,omitempty"` // v3.11 规则分组
	ShadowMode    bool   `json:"shadow_mode"`     // 影子模式
	Enabled       bool   `json:"enabled"`         // 启用状态
}

// RegexRule v3.11 正则规则（独立于 AC 自动机）
type RegexRule struct {
	Name       string
	Pattern    *regexp.Regexp
	Level      RuleLevel
	Category   string
	Priority   int
	Message    string
	Group      string
	ShadowMode bool
	Enabled    bool
}

type RuleEngine struct {
	mu               sync.RWMutex
	ac               *AhoCorasick
	rules            []Rule
	piiRe            []*regexp.Regexp
	piiNames         []string
	compositeKeyword *AhoCorasick
	version          RuleVersion
	ruleConfigs      []InboundRuleConfig // 保存原始配置用于 API 展示
	regexRules       []RegexRule         // v3.11 正则规则列表
	ruleBindings     []RuleBindingConfig // v3.11 规则绑定配置
	// v27.1 租户专属入站规则
	tenantRules    map[string][]InboundRuleConfig // tenantID -> extra rules
	tenantAC       map[string]*AhoCorasick        // tenantID -> compiled AC for tenant rules
	tenantRuleList map[string][]Rule              // tenantID -> Rule metadata
	tenantRegex    map[string][]RegexRule          // tenantID -> regex rules
	tenantDB       *sql.DB                        // v27.2: 持久化存储
	// v31.0 AC 智能分级
	autoReviewMgr  *AutoReviewManager
	// v30.0 全局启用的行业模板规则
	globalTemplateAC    *AhoCorasick // 全局模板 AC 自动机
	globalTemplateRules []Rule       // 全局模板 Rule 列表
	globalTemplateRegex []RegexRule  // 全局模板正则规则
}

// actionToLevel 将 action 字符串映射到 RuleLevel
func actionToLevel(action string) RuleLevel {
	switch action {
	case "block":
		return LevelHigh
	case "warn":
		return LevelMedium
	case "log":
		return LevelLow
	default:
		return LevelHigh
	}
}

// buildACFromConfigs 从 InboundRuleConfig 列表构建 AC 自动机和 Rule 列表
// v3.11: 只处理 keyword 类型规则（type 为空或 "keyword"），跳过 regex 类型
func buildACFromConfigs(configs []InboundRuleConfig) (*AhoCorasick, []Rule) {
	var patterns []string
	var rules []Rule
	for _, cfg := range configs {
		// v3.11: 跳过 regex 类型规则
		if cfg.Type == "regex" {
			continue
		}
		level := actionToLevel(cfg.Action)
		for _, p := range cfg.Patterns {
			patterns = append(patterns, p)
			rules = append(rules, Rule{Name: cfg.Name, Level: level, Category: cfg.Category, Priority: cfg.Priority, Message: cfg.Message, Group: cfg.Group, ShadowMode: cfg.ShadowMode, Enabled: isEnabled(cfg.Enabled)})
		}
	}
	if len(patterns) == 0 {
		return NewAhoCorasick([]string{}), nil
	}
	return NewAhoCorasick(patterns), rules
}

// buildRegexRules 从 InboundRuleConfig 列表构建正则规则（v3.11）
// 只处理 type == "regex" 的规则
func buildRegexRules(configs []InboundRuleConfig) []RegexRule {
	var regexRules []RegexRule
	for _, cfg := range configs {
		if cfg.Type != "regex" {
			continue
		}
		level := actionToLevel(cfg.Action)
		for _, p := range cfg.Patterns {
			compiled, err := regexp.Compile(p)
			if err != nil {
				log.Printf("[入站规则] 正则编译失败 rule=%s pattern=%q: %v（跳过）", cfg.Name, p, err)
				continue
			}
			regexRules = append(regexRules, RegexRule{
				Name:       cfg.Name,
				Pattern:    compiled,
				Level:      level,
				Category:   cfg.Category,
				Priority:   cfg.Priority,
				Message:    cfg.Message,
				Group:      cfg.Group,
				ShadowMode: cfg.ShadowMode,
				Enabled:    isEnabled(cfg.Enabled),
			})
		}
	}
	return regexRules
}

// countPatterns 统计规则配置中的 pattern 总数
func countPatterns(configs []InboundRuleConfig) int {
	n := 0
	for _, c := range configs {
		n += len(c.Patterns)
	}
	return n
}

// defaultPIIPatterns 返回默认的 PII 正则模式
func defaultPIIPatterns() ([]*regexp.Regexp, []string) {
	return []*regexp.Regexp{
		regexp.MustCompile(`\d{17}[\dXx]`),
		regexp.MustCompile(`(?:^|\D)1[3-9]\d{9}(?:\D|$)`),
		regexp.MustCompile(`(?:^|\D)\d{16,19}(?:\D|$)`),
	}, []string{"身份证号", "手机号", "银行卡号"}
}

// buildPIIPatterns 从配置构建 PII 正则模式（v3.11 可配置化）
func buildPIIPatterns(configs []OutboundPIIPatternConfig) ([]*regexp.Regexp, []string) {
	if len(configs) == 0 {
		return defaultPIIPatterns()
	}
	var piiRe []*regexp.Regexp
	var piiNames []string
	for _, c := range configs {
		compiled, err := regexp.Compile(c.Pattern)
		if err != nil {
			log.Printf("[出站PII] 正则编译失败 name=%s pattern=%q: %v（跳过）", c.Name, c.Pattern, err)
			continue
		}
		piiRe = append(piiRe, compiled)
		piiNames = append(piiNames, c.Name)
	}
	if len(piiRe) == 0 {
		// 配置了但全部编译失败，回退到默认
		log.Printf("[出站PII] 所有自定义模式编译失败，回退到默认模式")
		return defaultPIIPatterns()
	}
	return piiRe, piiNames
}

// NewRuleEngine 使用默认硬编码规则创建 RuleEngine（向后兼容）
func NewRuleEngine() *RuleEngine {
	defaultConfigs := getDefaultInboundRules()
	ac, rules := buildACFromConfigs(defaultConfigs)
	regexRules := buildRegexRules(defaultConfigs)
	patternCount := countPatterns(defaultConfigs)
	piiRe, piiNames := defaultPIIPatterns()
	return &RuleEngine{
		ac: ac, rules: rules,
		piiRe:            piiRe,
		piiNames:         piiNames,
		compositeKeyword: NewAhoCorasick([]string{"没有限制", "不受约束"}),
		version: RuleVersion{
			Version: 1, LoadedAt: time.Now(), Source: "default",
			RuleCount: len(defaultConfigs), PatternCount: patternCount,
		},
		ruleConfigs: defaultConfigs,
		regexRules:  regexRules,
	}
}

// NewRuleEngineFromConfig 从配置构建 RuleEngine
func NewRuleEngineFromConfig(configs []InboundRuleConfig, source string) *RuleEngine {
	ac, rules := buildACFromConfigs(configs)
	regexRules := buildRegexRules(configs)
	patternCount := countPatterns(configs)
	piiRe, piiNames := defaultPIIPatterns()
	return &RuleEngine{
		ac: ac, rules: rules,
		piiRe:            piiRe,
		piiNames:         piiNames,
		compositeKeyword: NewAhoCorasick([]string{"没有限制", "不受约束"}),
		version: RuleVersion{
			Version: 1, LoadedAt: time.Now(), Source: source,
			RuleCount: len(configs), PatternCount: patternCount,
		},
		ruleConfigs: configs,
		regexRules:  regexRules,
	}
}

// NewRuleEngineWithPII 从配置构建 RuleEngine，支持自定义 PII 模式（v3.11）
func NewRuleEngineWithPII(configs []InboundRuleConfig, source string, piiPatterns []OutboundPIIPatternConfig, bindings []RuleBindingConfig) *RuleEngine {
	ac, rules := buildACFromConfigs(configs)
	regexRules := buildRegexRules(configs)
	patternCount := countPatterns(configs)
	piiRe, piiNames := buildPIIPatterns(piiPatterns)
	return &RuleEngine{
		ac: ac, rules: rules,
		piiRe:            piiRe,
		piiNames:         piiNames,
		compositeKeyword: NewAhoCorasick([]string{"没有限制", "不受约束"}),
		version: RuleVersion{
			Version: 1, LoadedAt: time.Now(), Source: source,
			RuleCount: len(configs), PatternCount: patternCount,
		},
		ruleConfigs:  configs,
		regexRules:   regexRules,
		ruleBindings: bindings,
	}
}

// Reload 热更新入站规则（并发安全）
func (re *RuleEngine) Reload(configs []InboundRuleConfig, source string) {
	ac, rules := buildACFromConfigs(configs)
	regexRules := buildRegexRules(configs)
	patternCount := countPatterns(configs)
	re.mu.Lock()
	re.ac = ac
	re.rules = rules
	re.regexRules = regexRules
	re.ruleConfigs = configs
	re.version.Version++
	re.version.LoadedAt = time.Now()
	re.version.Source = source
	re.version.RuleCount = len(configs)
	re.version.PatternCount = patternCount
	re.mu.Unlock()
	log.Printf("[入站规则] 热更新完成 v%d，加载 %d 条规则 %d 个 pattern (source=%s, regex_rules=%d)",
		re.version.Version, len(configs), patternCount, source, len(regexRules))
}

// ReloadWithBindings 热更新入站规则和绑定配置（v3.11）
func (re *RuleEngine) ReloadWithBindings(configs []InboundRuleConfig, source string, bindings []RuleBindingConfig) {
	ac, rules := buildACFromConfigs(configs)
	regexRules := buildRegexRules(configs)
	patternCount := countPatterns(configs)
	re.mu.Lock()
	re.ac = ac
	re.rules = rules
	re.regexRules = regexRules
	re.ruleConfigs = configs
	re.ruleBindings = bindings
	re.version.Version++
	re.version.LoadedAt = time.Now()
	re.version.Source = source
	re.version.RuleCount = len(configs)
	re.version.PatternCount = patternCount
	re.mu.Unlock()
	log.Printf("[入站规则] 热更新完成 v%d，加载 %d 条规则 %d 个 pattern (source=%s, regex_rules=%d, bindings=%d)",
		re.version.Version, len(configs), patternCount, source, len(regexRules), len(bindings))
}

// SetRuleBindings 设置规则绑定配置（v3.11）
func (re *RuleEngine) SetRuleBindings(bindings []RuleBindingConfig) {
	re.mu.Lock()
	re.ruleBindings = bindings
	re.mu.Unlock()
}

// GetRuleBindings 获取规则绑定配置（v3.11）
func (re *RuleEngine) GetRuleBindings() []RuleBindingConfig {
	re.mu.RLock()
	defer re.mu.RUnlock()
	cp := make([]RuleBindingConfig, len(re.ruleBindings))
	copy(cp, re.ruleBindings)
	return cp
}

// GetApplicableGroups 根据 app_id 获取适用的规则组列表（v3.11）
// 返回 nil 表示所有规则都适用（无绑定配置或未匹配）
func (re *RuleEngine) GetApplicableGroups(appID string) []string {
	re.mu.RLock()
	bindings := re.ruleBindings
	re.mu.RUnlock()

	if len(bindings) == 0 {
		return nil // 没有绑定配置，所有规则生效
	}

	// 精确匹配
	for _, b := range bindings {
		if b.AppID == appID {
			return b.Groups
		}
	}
	// 通配符匹配
	for _, b := range bindings {
		if b.AppID == "*" {
			return b.Groups
		}
	}
	return nil // 无匹配，所有规则生效
}

// isRuleApplicable 判断规则是否适用于当前 app_id（v3.11）
func isRuleApplicable(ruleGroup string, applicableGroups []string) bool {
	if applicableGroups == nil {
		return true // 无绑定配置，所有规则生效
	}
	if ruleGroup == "" {
		return true // 未分组的规则始终生效
	}
	for _, g := range applicableGroups {
		if g == ruleGroup {
			return true
		}
	}
	return false
}

// GetRulesForAppID 获取某个 app_id 适用的规则列表（v3.11，用于 API 测试）
func (re *RuleEngine) GetRulesForAppID(appID string) []InboundRuleSummary {
	re.mu.RLock()
	defer re.mu.RUnlock()
	groups := re.GetApplicableGroups(appID)
	var summaries []InboundRuleSummary
	for _, cfg := range re.ruleConfigs {
		if isRuleApplicable(cfg.Group, groups) {
			ruleType := cfg.Type
			if ruleType == "" {
				ruleType = "keyword"
			}
			summaries = append(summaries, InboundRuleSummary{
				Name: cfg.Name, PatternsCount: len(cfg.Patterns),
				Action: cfg.Action, Category: cfg.Category,
				Priority: cfg.Priority, Message: cfg.Message,
				Type: ruleType, Group: cfg.Group,
			})
		}
	}
	return summaries
}

// Version 返回当前规则版本信息（并发安全）
func (re *RuleEngine) Version() RuleVersion {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.version
}

// ListRules 返回当前入站规则摘要列表
func (re *RuleEngine) ListRules() []InboundRuleSummary {
	re.mu.RLock()
	defer re.mu.RUnlock()
	summaries := make([]InboundRuleSummary, len(re.ruleConfigs))
	for i, cfg := range re.ruleConfigs {
		ruleType := cfg.Type
		if ruleType == "" {
			ruleType = "keyword"
		}
		enabled := isEnabled(cfg.Enabled)
		summaries[i] = InboundRuleSummary{
			Name: cfg.Name, DisplayName: cfg.DisplayName,
			PatternsCount: len(cfg.Patterns),
			Action: cfg.Action, Category: cfg.Category,
			Priority: cfg.Priority, Message: cfg.Message,
			Type: ruleType, Group: cfg.Group,
			ShadowMode: cfg.ShadowMode, Enabled: enabled,
		}
	}
	return summaries
}

type DetectResult struct { Action string; Reasons []string; PIIs []string; Message string; MatchedRules []string; ShadowReasons []string }

// actionWeight returns a numeric weight for action precedence (higher = more severe)
// Used when multiple rules have the same priority: block > warn > log
func actionWeight(action string) int {
	switch action {
	case "block":
		return 3
	case "warn":
		return 2
	case "log":
		return 1
	default:
		return 0
	}
}

// levelToAction converts RuleLevel back to action string
func levelToAction(level RuleLevel) string {
	switch level {
	case LevelHigh:
		return "block"
	case LevelMedium:
		return "warn"
	case LevelLow:
		return "log"
	default:
		return "block"
	}
}

func (re *RuleEngine) Detect(text string) DetectResult {
	return re.DetectWithAppID(text, "")
}

// DetectWithAppID 入站检测（v3.11 支持 app_id 规则绑定）
func (re *RuleEngine) DetectWithAppID(text, appID string) DetectResult {
	r := DetectResult{Action: "pass"}
	if text == "" { return r }
	re.mu.RLock()
	ac := re.ac
	rules := re.rules
	compositeKeyword := re.compositeKeyword
	regexRules := re.regexRules
	re.mu.RUnlock()

	// v3.11: 获取适用的规则组
	applicableGroups := re.GetApplicableGroups(appID)

	// Collect all matched rules (deduplicate by rule name, keep highest priority match)
	type matchedRule struct {
		Name       string
		Level      RuleLevel
		Priority   int
		Message    string
		Action     string
		ShadowMode bool
	}
	matchesByName := make(map[string]*matchedRule)

	for _, idx := range ac.Search(text) {
		if idx < 0 || idx >= len(rules) { continue }
		rule := rules[idx]
		// 跳过禁用的规则
		if !rule.Enabled { continue }
		// v3.11: 检查规则组是否适用
		if !isRuleApplicable(rule.Group, applicableGroups) {
			continue
		}
		action := levelToAction(rule.Level)
		if existing, ok := matchesByName[rule.Name]; ok {
			// Same rule name (different pattern), keep if higher priority or higher action weight
			if rule.Priority > existing.Priority ||
				(rule.Priority == existing.Priority && actionWeight(action) > actionWeight(existing.Action)) {
				existing.Level = rule.Level
				existing.Priority = rule.Priority
				existing.Message = rule.Message
				existing.Action = action
			}
		} else {
			matchesByName[rule.Name] = &matchedRule{
				Name: rule.Name, Level: rule.Level,
				Priority: rule.Priority, Message: rule.Message,
				Action: action, ShadowMode: rule.ShadowMode,
			}
		}
	}

	// v3.11: 正则规则匹配（在 AC 自动机之后）
	for _, rr := range regexRules {
		// 跳过禁用的规则
		if !rr.Enabled { continue }
		// 检查规则组是否适用
		if !isRuleApplicable(rr.Group, applicableGroups) {
			continue
		}
		// 正则匹配超时保护（100ms）
		matched := false
		done := make(chan bool, 1)
		go func() {
			defer func() {
				if rv := recover(); rv != nil {
					done <- false
				}
			}()
			done <- rr.Pattern.MatchString(text)
		}()
		select {
		case matched = <-done:
		case <-time.After(100 * time.Millisecond):
			log.Printf("[入站规则] 正则匹配超时 rule=%s（100ms），跳过", rr.Name)
			continue
		}
		if !matched {
			continue
		}
		action := levelToAction(rr.Level)
		if existing, ok := matchesByName[rr.Name]; ok {
			if rr.Priority > existing.Priority ||
				(rr.Priority == existing.Priority && actionWeight(action) > actionWeight(existing.Action)) {
				existing.Level = rr.Level
				existing.Priority = rr.Priority
				existing.Message = rr.Message
				existing.Action = action
			}
		} else {
			matchesByName[rr.Name] = &matchedRule{
				Name: rr.Name, Level: rr.Level,
				Priority: rr.Priority, Message: rr.Message,
				Action: action, ShadowMode: rr.ShadowMode,
			}
		}
	}

	// Composite check
	if strings.Contains(strings.ToLower(text), "你现在是") && len(compositeKeyword.Search(text)) > 0 {
		matchesByName["prompt_injection_composite_cn"] = &matchedRule{
			Name: "prompt_injection_composite_cn", Level: LevelHigh,
			Priority: 0, Action: "block",
		}
	}

	// Determine final action based on priority
	if len(matchesByName) > 0 {
		// Collect all matches into a slice
		var matches []*matchedRule
		for _, m := range matchesByName {
			matches = append(matches, m)
		}

		// Sort: highest priority first, then by action weight (block > warn > log)
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Priority != matches[j].Priority {
				return matches[i].Priority > matches[j].Priority
			}
			return actionWeight(matches[i].Action) > actionWeight(matches[j].Action)
		})

		// 分离影子模式和正常规则
		var normalMatches []*matchedRule
		var shadowMatches []*matchedRule
		for _, m := range matches {
			if m.ShadowMode {
				shadowMatches = append(shadowMatches, m)
			} else {
				normalMatches = append(normalMatches, m)
			}
		}

		// 影子模式命中记录到 ShadowReasons（只记录不拦截）
		for _, m := range shadowMatches {
			r.ShadowReasons = append(r.ShadowReasons, m.Name)
		}

		// 正常规则决定最终 action
		if len(normalMatches) > 0 {
			winner := normalMatches[0]
			r.Action = winner.Action
			r.Message = winner.Message
		}

		// Collect all matched rule names as reasons
		for _, m := range normalMatches {
			r.Reasons = append(r.Reasons, m.Name)
			r.MatchedRules = append(r.MatchedRules, m.Name)
		}
	}

	// v31.0: AC 智能分级 — block 前检查 auto-review
	if r.Action == "block" && re.autoReviewMgr != nil && len(r.MatchedRules) > 0 {
		// 记录所有命中规则到滑动窗口 + 检查是否全部处于 review 状态
		allInReview := true
		for _, rule := range r.MatchedRules {
			re.autoReviewMgr.RecordBlock(rule)
			if !re.autoReviewMgr.IsInReview(rule) {
				allInReview = false
			}
		}
		if allInReview {
			// 所有命中规则均已降级 → 触发 LLM 复核
			r.Action = re.autoReviewMgr.ReviewWithLLM(r.MatchedRules[0], text)
		}
	}

	// PII detection — v31.1: PII 规则已合并到 InboundRuleConfig (type=regex, group=pii)
	// 统一规则体系的 regex 检测已在上方执行，此处仅做兼容性 PIIs 字段填充
	for _, rule := range r.MatchedRules {
		if strings.HasPrefix(rule, "pii_") {
			r.PIIs = append(r.PIIs, rule)
		}
	}
	return r
}

// GetRuleConfigs 返回当前入站规则的原始配置列表（v6.3 CRUD 用）
func (re *RuleEngine) GetRuleConfigs() []InboundRuleConfig {
	re.mu.RLock()
	defer re.mu.RUnlock()
	cp := make([]InboundRuleConfig, len(re.ruleConfigs))
	for i, c := range re.ruleConfigs {
		rc := InboundRuleConfig{
			Name:        c.Name,
			DisplayName: c.DisplayName,
			Patterns:    make([]string, len(c.Patterns)),
			Action:      c.Action,
			Category:    c.Category,
			Priority:    c.Priority,
			Message:     c.Message,
			Type:        c.Type,
			Group:       c.Group,
			ShadowMode:  c.ShadowMode,
			Enabled:     c.Enabled,
		}
		// 如果没有 DisplayName，从全局映射表查找（兼容旧配置）
		if rc.DisplayName == "" {
			rc.DisplayName = inboundRuleDisplayNames[rc.Name]
		}
		copy(rc.Patterns, c.Patterns)
		cp[i] = rc
	}
	return cp
}

// inboundRuleDisplayNames 入站规则中文显示名映射表（兼容旧配置中没有 display_name 的规则）
var inboundRuleDisplayNames = map[string]string{
	"prompt_injection_en":          "提示注入（英文）",
	"prompt_injection_identity":    "身份伪造注入",
	"prompt_injection_jailbreak":   "越狱攻击",
	"credential_theft":             "凭据窃取",
	"data_exfiltration":            "数据外泄",
	"prompt_injection_system":      "系统提示词窃取",
	"code_injection":               "代码注入",
	"destructive_commands":         "破坏性命令",
	"prompt_injection_cn":          "提示注入（中文）",
	"prompt_injection_system_cn":   "系统提示词窃取（中文）",
	"roleplay_cn":                  "角色扮演诱导（中文）",
	"roleplay_en":                  "角色扮演诱导（英文）",
	"prompt_injection_bypass":      "安全绕过",
	"prompt_injection_cn_extra":    "提示注入（中文增强）",
	"prompt_injection_role_inject": "角色注入",
	"base64_injection":             "Base64 混淆注入",
	"sensitive_keywords":           "敏感关键词",
	"copyright_violation":          "版权侵犯",
	"cross_border_data":            "跨境数据传输",
	"confidential_document":        "机密文件",
	// 旧版规则名映射（142 等旧部署兼容）
	"custom_block":                    "自定义拦截",
	"regex_base64_injection":          "Base64 注入（正则）",
	"prompt_injection_ignore":         "提示注入（忽略指令）",
	"prompt_injection_dan":            "DAN 越狱攻击",
	"prompt_injection_role":           "角色注入",
	"prompt_injection_cn_roleplay":    "角色扮演注入（中文）",
	"regex_role_injection":            "角色注入（正则）",
}

// ListPIIPatterns 返回当前 PII 模式列表（v3.11 API 展示用）
func (re *RuleEngine) ListPIIPatterns() []map[string]string {
	re.mu.RLock()
	defer re.mu.RUnlock()
	patterns := make([]map[string]string, len(re.piiNames))
	for i, name := range re.piiNames {
		p := ""
		if i < len(re.piiRe) {
			p = re.piiRe[i].String()
		}
		patterns[i] = map[string]string{"name": name, "pattern": p}
	}
	return patterns
}

func (re *RuleEngine) DetectPII(text string) []string {
	var piis []string
	for i, p := range re.piiRe {
		if p.MatchString(text) { piis = append(piis, re.piiNames[i]) }
	}
	return piis
}

// DetectWithExclusions 入站检测，排除指定规则名（v27.0 租户策略闭环）
func (re *RuleEngine) DetectWithExclusions(text, appID string, excludeRules []string) DetectResult {
	if len(excludeRules) == 0 {
		return re.DetectWithAppID(text, appID)
	}
	result := re.DetectWithAppID(text, appID)
	if result.Action == "pass" {
		return result
	}
	// 过滤掉被排除的规则
	excSet := make(map[string]bool, len(excludeRules))
	for _, r := range excludeRules {
		excSet[r] = true
	}
	var filteredReasons []string
	var filteredRules []string
	for _, r := range result.MatchedRules {
		if !excSet[r] {
			filteredRules = append(filteredRules, r)
		}
	}
	for _, r := range result.Reasons {
		if !excSet[r] {
			filteredReasons = append(filteredReasons, r)
		}
	}
	// 如果所有规则都被排除，降级为 pass
	if len(filteredRules) == 0 {
		result.Action = "pass"
		result.Reasons = nil
		result.MatchedRules = nil
		result.Message = ""
		return result
	}
	result.MatchedRules = filteredRules
	result.Reasons = filteredReasons
	return result
}

// ============================================================
// v27.1 租户专属入站规则（行业模板绑定）
// ============================================================

// SetTenantRules 设置租户专属入站规则并编译 AC 自动机
// SetTenantDB 设置持久化 DB 并加载已保存的租户规则
func (re *RuleEngine) SetTenantDB(db *sql.DB) {
	re.tenantDB = db
	if db == nil {
		return
	}
	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_inbound_rules (
		tenant_id TEXT NOT NULL,
		rules_json TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (tenant_id)
	)`)
	// 从 DB 重建内存
	re.loadTenantRulesFromDB()
}

// loadTenantRulesFromDB 启动时从 DB 重建租户入站规则
func (re *RuleEngine) loadTenantRulesFromDB() {
	if re.tenantDB == nil {
		return
	}
	rows, err := re.tenantDB.Query(`SELECT tenant_id, rules_json FROM tenant_inbound_rules`)
	if err != nil {
		log.Printf("[入站规则] 加载持久化租户规则失败: %v", err)
		return
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var tid, rulesJSON string
		if rows.Scan(&tid, &rulesJSON) != nil {
			continue
		}
		var rules []InboundRuleConfig
		if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
			continue
		}
		if len(rules) == 0 {
			continue
		}
		// 重建 AC 自动机（不走 SetTenantRules 避免重复写 DB）
		ac, ruleList := buildACFromConfigs(rules)
		regexRules := buildRegexRules(rules)
		re.mu.Lock()
		if re.tenantRules == nil {
			re.tenantRules = make(map[string][]InboundRuleConfig)
			re.tenantAC = make(map[string]*AhoCorasick)
			re.tenantRuleList = make(map[string][]Rule)
			re.tenantRegex = make(map[string][]RegexRule)
		}
		re.tenantRules[tid] = rules
		re.tenantAC[tid] = ac
		re.tenantRuleList[tid] = ruleList
		re.tenantRegex[tid] = regexRules
		re.mu.Unlock()
		count++
	}
	if count > 0 {
		log.Printf("[入站规则] ✅ 从 DB 恢复了 %d 个租户的入站规则", count)
	}
}

// persistTenantRules 持久化租户入站规则到 DB
func (re *RuleEngine) persistTenantRules(tenantID string, rules []InboundRuleConfig) {
	if re.tenantDB == nil {
		return
	}
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		log.Printf("[入站规则] 序列化失败: %v", err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	re.tenantDB.Exec(`INSERT OR REPLACE INTO tenant_inbound_rules (tenant_id, rules_json, updated_at) VALUES (?,?,?)`,
		tenantID, string(rulesJSON), now)
}

// removePersistTenantRules 从 DB 删除租户入站规则
func (re *RuleEngine) removePersistTenantRules(tenantID string) {
	if re.tenantDB == nil {
		return
	}
	re.tenantDB.Exec(`DELETE FROM tenant_inbound_rules WHERE tenant_id=?`, tenantID)
}

func (re *RuleEngine) SetTenantRules(tenantID string, rules []InboundRuleConfig) {
	ac, ruleList := buildACFromConfigs(rules)
	regexRules := buildRegexRules(rules)
	re.mu.Lock()
	if re.tenantRules == nil {
		re.tenantRules = make(map[string][]InboundRuleConfig)
		re.tenantAC = make(map[string]*AhoCorasick)
		re.tenantRuleList = make(map[string][]Rule)
		re.tenantRegex = make(map[string][]RegexRule)
	}
	re.tenantRules[tenantID] = rules
	re.tenantAC[tenantID] = ac
	re.tenantRuleList[tenantID] = ruleList
	re.tenantRegex[tenantID] = regexRules
	re.mu.Unlock()
	// v27.2: 持久化到 DB
	re.persistTenantRules(tenantID, rules)
	log.Printf("[入站规则] 设置租户 %s 专属规则: %d 条规则, %d 个 pattern, %d 条正则",
		tenantID, len(rules), countPatterns(rules), len(regexRules))
}

// RemoveTenantRules 移除租户专属入站规则
func (re *RuleEngine) RemoveTenantRules(tenantID string) {
	re.mu.Lock()
	delete(re.tenantRules, tenantID)
	delete(re.tenantAC, tenantID)
	delete(re.tenantRuleList, tenantID)
	delete(re.tenantRegex, tenantID)
	re.mu.Unlock()
	// v27.2: 从 DB 删除
	re.removePersistTenantRules(tenantID)
	log.Printf("[入站规则] 移除租户 %s 专属规则", tenantID)
}

// GetTenantRules 获取租户专属入站规则配置
func (re *RuleEngine) GetTenantRules(tenantID string) []InboundRuleConfig {
	re.mu.RLock()
	defer re.mu.RUnlock()
	rules := re.tenantRules[tenantID]
	if rules == nil {
		return nil
	}
	cp := make([]InboundRuleConfig, len(rules))
	for i, c := range rules {
		rc := InboundRuleConfig{
			Name:        c.Name,
			DisplayName: c.DisplayName,
			Patterns:    make([]string, len(c.Patterns)),
			Action:      c.Action,
			Category:    c.Category,
			Priority:    c.Priority,
			Message:     c.Message,
			Type:        c.Type,
			Group:       c.Group,
			ShadowMode:  c.ShadowMode,
			Enabled:     c.Enabled,
		}
		if rc.DisplayName == "" {
			rc.DisplayName = inboundRuleDisplayNames[rc.Name]
		}
		copy(rc.Patterns, c.Patterns)
		cp[i] = rc
	}
	return cp
}

// ============================================================
// v28.0 入站规则模板 CRUD（DB 持久化）
// ============================================================

// SetInboundTemplateDB 设置入站规则模板 DB 并加载内置模板
// 如果 RuleEngine 已有 tenantDB 可复用
func (re *RuleEngine) SetInboundTemplateDB(db *sql.DB) {
	if db == nil {
		return
	}
	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS inbound_rule_templates (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		category TEXT,
		rules_json TEXT NOT NULL,
		built_in INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	// v30.0: 给已有表加 enabled 列（ALTER TABLE 幂等）
	db.Exec(`ALTER TABLE inbound_rule_templates ADD COLUMN enabled INTEGER DEFAULT 0`)
	// 加载内置模板
	re.loadBuiltinInboundTemplates(db)
}

// loadBuiltinInboundTemplates 启动时加载内置入站模板到 DB
func (re *RuleEngine) loadBuiltinInboundTemplates(db *sql.DB) {
	if db == nil {
		return
	}
	builtins := getDefaultInboundTemplates()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, tpl := range builtins {
		rulesJSON, err := json.Marshal(tpl.Rules)
		if err != nil {
			continue
		}
		db.Exec(`INSERT OR IGNORE INTO inbound_rule_templates (id, name, description, category, rules_json, built_in, created_at, updated_at)
			VALUES (?,?,?,?,?,1,?,?)`,
			tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, now)
		// 同步更新
		db.Exec(`UPDATE inbound_rule_templates SET name=?, description=?, category=?, rules_json=?, updated_at=?
			WHERE id=? AND built_in=1`,
			tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, tpl.ID)
	}
	log.Printf("[入站规则] 加载 %d 个内置入站规则模板", len(builtins))
}

// ListInboundTemplates 返回所有入站规则行业模板
func (re *RuleEngine) ListInboundTemplates() []InboundRuleTemplate {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()

	if db == nil {
		return getDefaultInboundTemplates()
	}

	rows, err := db.Query(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM inbound_rule_templates ORDER BY id`)
	if err != nil {
		return getDefaultInboundTemplates()
	}
	defer rows.Close()

	var templates []InboundRuleTemplate
	for rows.Next() {
		var tpl InboundRuleTemplate
		var rulesJSON string
		var builtIn, enabled int
		if rows.Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.Category, &rulesJSON, &builtIn, &enabled) != nil {
			continue
		}
		tpl.BuiltIn = builtIn == 1
		tpl.Enabled = enabled == 1
		if json.Unmarshal([]byte(rulesJSON), &tpl.Rules) != nil {
			continue
		}
		templates = append(templates, tpl)
	}
	if len(templates) == 0 {
		return getDefaultInboundTemplates()
	}
	return templates
}

// GetInboundTemplate 获取单个入站规则模板
func (re *RuleEngine) GetInboundTemplate(id string) *InboundRuleTemplate {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()

	if db == nil {
		// 无 DB 回退到内存
		for _, tpl := range getDefaultInboundTemplates() {
			if tpl.ID == id {
				return &tpl
			}
		}
		return nil
	}

	var tpl InboundRuleTemplate
	var rulesJSON string
	var builtIn, enabled int
	err := db.QueryRow(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM inbound_rule_templates WHERE id=?`, id).
		Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.Category, &rulesJSON, &builtIn, &enabled)
	if err != nil {
		return nil
	}
	tpl.BuiltIn = builtIn == 1
	tpl.Enabled = enabled == 1
	if json.Unmarshal([]byte(rulesJSON), &tpl.Rules) != nil {
		return nil
	}
	return &tpl
}

// EnableInboundTemplate 启用/禁用入站模板全局开关（v30.0）
func (re *RuleEngine) EnableInboundTemplate(id string, enabled bool) error {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	val := 0
	if enabled {
		val = 1
	}
	result, err := db.Exec(`UPDATE inbound_rule_templates SET enabled=?, updated_at=? WHERE id=?`,
		val, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	// 重建全局模板 AC 自动机
	re.rebuildGlobalTemplateAC()
	return nil
}

// GetEnabledInboundTemplateRules 返回所有全局启用模板的规则合集（v30.0）
func (re *RuleEngine) GetEnabledInboundTemplateRules() []InboundRuleConfig {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()
	if db == nil {
		return nil
	}
	var allRules []InboundRuleConfig
	// v31.0: 从旧表 + 统一行业模板表 union 读取
	for _, query := range []string{
		`SELECT rules_json FROM inbound_rule_templates WHERE enabled=1`,
		`SELECT inbound_rules_json FROM industry_templates WHERE enabled=1 AND inbound_rules_json != '' AND inbound_rules_json != '[]' AND inbound_rules_json != 'null'`,
	} {
		rows, err := db.Query(query)
		if err != nil {
			continue
		}
		for rows.Next() {
			var rulesJSON string
			if rows.Scan(&rulesJSON) != nil {
				continue
			}
			var rules []InboundRuleConfig
			if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
				continue
			}
			allRules = append(allRules, rules...)
		}
		rows.Close()
	}
	return allRules
}

// rebuildGlobalTemplateAC 重建全局模板 AC 自动机缓存（v30.0）
func (re *RuleEngine) rebuildGlobalTemplateAC() {
	rules := re.GetEnabledInboundTemplateRules()
	if len(rules) == 0 {
		re.mu.Lock()
		re.globalTemplateAC = nil
		re.globalTemplateRules = nil
		re.globalTemplateRegex = nil
		re.mu.Unlock()
		log.Printf("[入站规则] 全局模板: 0 条规则（已清空）")
		return
	}
	ac, ruleList := buildACFromConfigs(rules)
	regexRules := buildRegexRules(rules)
	re.mu.Lock()
	re.globalTemplateAC = ac
	re.globalTemplateRules = ruleList
	re.globalTemplateRegex = regexRules
	re.mu.Unlock()
	log.Printf("[入站规则] 全局模板: %d 条规则, AC 已重建", len(rules))
}

// DetectGlobalTemplates 全局模板规则检测（v30.0）
func (re *RuleEngine) DetectGlobalTemplates(text string) DetectResult {
	r := DetectResult{Action: "pass"}
	if text == "" {
		return r
	}
	re.mu.RLock()
	ac := re.globalTemplateAC
	rules := re.globalTemplateRules
	regexRules := re.globalTemplateRegex
	re.mu.RUnlock()
	if ac == nil && len(regexRules) == 0 {
		return r
	}
	type matchedRule struct {
		Name     string
		Level    RuleLevel
		Priority int
		Message  string
		Action   string
	}
	matchesByName := make(map[string]*matchedRule)
	// AC 匹配
	if ac != nil && rules != nil {
		for _, idx := range ac.Search(text) {
			if idx < 0 || idx >= len(rules) {
				continue
			}
			rule := rules[idx]
			action := levelToAction(rule.Level)
			if existing, ok := matchesByName[rule.Name]; ok {
				if rule.Priority > existing.Priority ||
					(rule.Priority == existing.Priority && actionWeight(action) > actionWeight(existing.Action)) {
					existing.Level = rule.Level
					existing.Priority = rule.Priority
					existing.Message = rule.Message
					existing.Action = action
				}
			} else {
				matchesByName[rule.Name] = &matchedRule{Name: rule.Name, Level: rule.Level, Priority: rule.Priority, Message: rule.Message, Action: action}
			}
		}
	}
	// 正则匹配
	for _, rr := range regexRules {
		if !rr.Enabled {
			continue
		}
		if rr.Pattern.MatchString(text) {
			action := levelToAction(rr.Level)
			if existing, ok := matchesByName[rr.Name]; ok {
				if rr.Priority > existing.Priority {
					existing.Level = rr.Level
					existing.Priority = rr.Priority
					existing.Message = rr.Message
					existing.Action = action
				}
			} else {
				matchesByName[rr.Name] = &matchedRule{Name: rr.Name, Level: rr.Level, Priority: rr.Priority, Message: rr.Message, Action: action}
			}
		}
	}
	// 转换为 DetectResult
	for _, m := range matchesByName {
		r.MatchedRules = append(r.MatchedRules, m.Name)
		if m.Message != "" {
			r.Reasons = append(r.Reasons, m.Message)
		}
		if actionWeight(m.Action) > actionWeight(r.Action) {
			r.Action = m.Action
			r.Message = m.Message
		}
	}
	return r
}

// InitGlobalTemplateAC 启动时初始化全局模板 AC（v30.0）
func (re *RuleEngine) InitGlobalTemplateAC() {
	re.rebuildGlobalTemplateAC()
}

// CreateInboundTemplate 创建自定义入站规则模板
func (re *RuleEngine) CreateInboundTemplate(tpl InboundRuleTemplate) error {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	if tpl.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}
	existing := re.GetInboundTemplate(tpl.ID)
	if existing != nil {
		return fmt.Errorf("模板 %q 已存在", tpl.ID)
	}

	rulesJSON, err := json.Marshal(tpl.Rules)
	if err != nil {
		return fmt.Errorf("序列化规则失败: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	builtIn := 0
	if tpl.BuiltIn {
		builtIn = 1
	}
	_, err = db.Exec(`INSERT INTO inbound_rule_templates (id, name, description, category, rules_json, built_in, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), builtIn, now, now)
	if err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}
	return nil
}

// UpdateInboundTemplate 更新入站规则模板
func (re *RuleEngine) UpdateInboundTemplate(id string, tpl InboundRuleTemplate) error {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	existing := re.GetInboundTemplate(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}

	rulesJSON, err := json.Marshal(tpl.Rules)
	if err != nil {
		return fmt.Errorf("序列化规则失败: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`UPDATE inbound_rule_templates SET name=?, description=?, category=?, rules_json=?, updated_at=? WHERE id=?`,
		tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, id)
	if err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}
	return nil
}

// DeleteInboundTemplate 删除入站规则模板（内置模板不可删）
func (re *RuleEngine) DeleteInboundTemplate(id string) error {
	re.mu.RLock()
	db := re.tenantDB
	re.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	existing := re.GetInboundTemplate(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	if existing.BuiltIn {
		return fmt.Errorf("内置模板 %q 不可删除", id)
	}
	_, err := db.Exec(`DELETE FROM inbound_rule_templates WHERE id=? AND built_in=0`, id)
	if err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}
	return nil
}

// DetectTenantRules 对指定租户的专属规则进行检测
func (re *RuleEngine) DetectTenantRules(tenantID, text string) DetectResult {
	r := DetectResult{Action: "pass"}
	if text == "" || tenantID == "" {
		return r
	}
	re.mu.RLock()
	ac := re.tenantAC[tenantID]
	rules := re.tenantRuleList[tenantID]
	regexRules := re.tenantRegex[tenantID]
	re.mu.RUnlock()

	if ac == nil && len(regexRules) == 0 {
		return r
	}

	type matchedRule struct {
		Name     string
		Level    RuleLevel
		Priority int
		Message  string
		Action   string
	}
	matchesByName := make(map[string]*matchedRule)

	// AC 自动机匹配
	if ac != nil && rules != nil {
		for _, idx := range ac.Search(text) {
			if idx < 0 || idx >= len(rules) {
				continue
			}
			rule := rules[idx]
			action := levelToAction(rule.Level)
			if existing, ok := matchesByName[rule.Name]; ok {
				if rule.Priority > existing.Priority ||
					(rule.Priority == existing.Priority && actionWeight(action) > actionWeight(existing.Action)) {
					existing.Level = rule.Level
					existing.Priority = rule.Priority
					existing.Message = rule.Message
					existing.Action = action
				}
			} else {
				matchesByName[rule.Name] = &matchedRule{
					Name: rule.Name, Level: rule.Level,
					Priority: rule.Priority, Message: rule.Message,
					Action: action,
				}
			}
		}
	}

	// 正则规则匹配
	for _, rr := range regexRules {
		matched := false
		done := make(chan bool, 1)
		go func() {
			defer func() {
				if rv := recover(); rv != nil {
					done <- false
				}
			}()
			done <- rr.Pattern.MatchString(text)
		}()
		select {
		case matched = <-done:
		case <-time.After(100 * time.Millisecond):
			log.Printf("[入站规则] 租户 %s 正则匹配超时 rule=%s（100ms），跳过", tenantID, rr.Name)
			continue
		}
		if !matched {
			continue
		}
		action := levelToAction(rr.Level)
		if existing, ok := matchesByName[rr.Name]; ok {
			if rr.Priority > existing.Priority ||
				(rr.Priority == existing.Priority && actionWeight(action) > actionWeight(existing.Action)) {
				existing.Level = rr.Level
				existing.Priority = rr.Priority
				existing.Message = rr.Message
				existing.Action = action
			}
		} else {
			matchesByName[rr.Name] = &matchedRule{
				Name: rr.Name, Level: rr.Level,
				Priority: rr.Priority, Message: rr.Message,
				Action: action,
			}
		}
	}

	if len(matchesByName) > 0 {
		var matches []*matchedRule
		for _, m := range matchesByName {
			matches = append(matches, m)
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Priority != matches[j].Priority {
				return matches[i].Priority > matches[j].Priority
			}
			return actionWeight(matches[i].Action) > actionWeight(matches[j].Action)
		})
		winner := matches[0]
		r.Action = winner.Action
		r.Message = winner.Message
		for _, m := range matches {
			r.Reasons = append(r.Reasons, m.Name)
			r.MatchedRules = append(r.MatchedRules, m.Name)
		}
	}
	return r
}

// mergeDetectResults 合并两个检测结果，取更严格的 action
func mergeDetectResults(base, extra DetectResult) DetectResult {
	if extra.Action == "pass" {
		return base
	}
	if base.Action == "pass" {
		return extra
	}
	// 合并: 取更严格的 action
	if actionWeight(extra.Action) > actionWeight(base.Action) {
		base.Action = extra.Action
	}
	if extra.Message != "" && base.Message == "" {
		base.Message = extra.Message
	}
	base.Reasons = append(base.Reasons, extra.Reasons...)
	base.MatchedRules = append(base.MatchedRules, extra.MatchedRules...)
	base.PIIs = append(base.PIIs, extra.PIIs...)
	return base
}

// ============================================================
// 出站规则引擎 v2.0（block/warn/log）
// ============================================================

type OutboundRule struct {
	Name        string
	DisplayName string
	Regexps     []*regexp.Regexp
	Action      string
	Priority    int
	Message     string
	ShadowMode  bool
	Enabled     bool
}

type OutboundRuleEngine struct {
	mu                  sync.RWMutex
	rules               []OutboundRule
	globalTemplateRules []OutboundRule
}

func NewOutboundRuleEngine(configs []OutboundRuleConfig) *OutboundRuleEngine {
	// v18: 合并用户配置 + 内置默认规则（去重：用户配置同名规则覆盖默认）
	merged := mergeOutboundDefaults(configs)
	return &OutboundRuleEngine{rules: compileOutboundRules(merged)}
}

// getDefaultOutboundRules 内置出站规则（PII + 凭据 + 恶意命令）
func getDefaultOutboundRules() []OutboundRuleConfig {
	return []OutboundRuleConfig{
		// 默认规则 priority=0，用户自定义规则 priority>0 时自动优先
		{Name: "pii_id_card", DisplayName: "身份证号泄露", Patterns: []string{`\d{17}[\dXx]`}, Action: "warn", Priority: 0, Message: "检测到身份证号"},
		{Name: "pii_phone", DisplayName: "手机号泄露", Patterns: []string{`(?:^|\D)1[3-9]\d{9}(?:\D|$)`}, Action: "warn", Priority: 0, Message: "检测到手机号"},
		{Name: "pii_bank_card", DisplayName: "银行卡号泄露", Patterns: []string{`(?:^|\D)(62|4[0-9]|5[1-5])\d{14,17}(?:\D|$)`}, Action: "warn", Priority: 0, Message: "检测到银行卡号"},
		{Name: "credential_password", DisplayName: "密码/密钥泄露", Patterns: []string{`(?i)(password|passwd|secret_key)\s*[:=]\s*\S+`}, Action: "block", Priority: 0, Message: "检测到密码/密钥泄露"},
		{Name: "credential_api_key", DisplayName: "API Key 泄露", Patterns: []string{`(?i)(sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}|AKIA[0-9A-Z]{16})`}, Action: "block", Priority: 0, Message: "检测到API Key泄露"},
		{Name: "malicious_command", DisplayName: "恶意命令注入", Patterns: []string{`(?i)rm\s+-rf\s+/`, `(?i)curl\s+.{0,50}\|\s*bash`, `(?i)wget\s+.{0,50}\|\s*bash`}, Action: "block", Priority: 0, Message: "检测到恶意命令"},
	}
}

// mergeOutboundDefaults 合并默认规则和用户配置（用户同名规则覆盖默认）
func mergeOutboundDefaults(userConfigs []OutboundRuleConfig) []OutboundRuleConfig {
	defaults := getDefaultOutboundRules()
	userNames := make(map[string]bool)
	for _, c := range userConfigs {
		userNames[c.Name] = true
	}
	var merged []OutboundRuleConfig
	// 先放用户规则
	merged = append(merged, userConfigs...)
	// 再放未被覆盖的默认规则
	for _, d := range defaults {
		if !userNames[d.Name] {
			merged = append(merged, d)
		}
	}
	return merged
}

func compileOutboundRules(configs []OutboundRuleConfig) []OutboundRule {
	var rules []OutboundRule
	for _, c := range configs {
		rule := OutboundRule{Name: c.Name, DisplayName: c.DisplayName, Action: c.Action, Priority: c.Priority, Message: c.Message, ShadowMode: c.ShadowMode, Enabled: isEnabled(c.Enabled)}
		if rule.Action == "" { rule.Action = "log" }
		var patterns []string
		if c.Pattern != "" { patterns = append(patterns, c.Pattern) }
		patterns = append(patterns, c.Patterns...)
		for _, p := range patterns {
			compiled, err := regexp.Compile(p)
			if err != nil {
				log.Printf("[出站规则] 编译正则失败 rule=%s: %v", c.Name, err)
				continue
			}
			rule.Regexps = append(rule.Regexps, compiled)
		}
		if len(rule.Regexps) > 0 { rules = append(rules, rule) }
	}
	return rules
}

func (ore *OutboundRuleEngine) Reload(configs []OutboundRuleConfig) {
	newRules := compileOutboundRules(configs)
	ore.mu.Lock(); ore.rules = newRules; ore.mu.Unlock()
	log.Printf("[出站规则] 热更新完成，加载 %d 条规则", len(newRules))
}

func (ore *OutboundRuleEngine) RebuildGlobalTemplateRules(configs []OutboundRuleConfig) {
	compiled := compileOutboundRules(configs)
	ore.mu.Lock()
	ore.globalTemplateRules = compiled
	ore.mu.Unlock()
	log.Printf("[出站规则] 全局模板规则已重建: %d 条", len(compiled))
}

func (ore *OutboundRuleEngine) InitGlobalTemplateRules(db *sql.DB) {
	if db == nil {
		ore.RebuildGlobalTemplateRules(nil)
		return
	}
	rows, err := db.Query(`SELECT outbound_rules_json FROM industry_templates WHERE enabled=1`)
	if err != nil {
		ore.RebuildGlobalTemplateRules(nil)
		return
	}
	defer rows.Close()
	var allRules []OutboundRuleConfig
	for rows.Next() {
		var rulesJSON string
		if rows.Scan(&rulesJSON) != nil {
			continue
		}
		var rules []OutboundRuleConfig
		if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
			continue
		}
		allRules = append(allRules, rules...)
	}
	ore.RebuildGlobalTemplateRules(allRules)
}

// GetRuleConfigs 返回当前规则的配置表示（用于 CRUD 和持久化）
func (ore *OutboundRuleEngine) GetRuleConfigs() []OutboundRuleConfig {
	ore.mu.RLock()
	defer ore.mu.RUnlock()
	configs := make([]OutboundRuleConfig, len(ore.rules))
	for i, rule := range ore.rules {
		var patterns []string
		for _, re := range rule.Regexps {
			patterns = append(patterns, re.String())
		}
		enabled := rule.Enabled
		configs[i] = OutboundRuleConfig{
			Name:        rule.Name,
			DisplayName: rule.DisplayName,
			Patterns:    patterns,
			Action:      rule.Action,
			Priority:    rule.Priority,
			Message:     rule.Message,
			ShadowMode:  rule.ShadowMode,
			Enabled:     &enabled,
		}
	}
	return configs
}

// AddRule 添加一条出站规则（内存更新，不涉及持久化）
func (ore *OutboundRuleEngine) AddRule(cfg OutboundRuleConfig) error {
	ore.mu.Lock()
	defer ore.mu.Unlock()
	for _, r := range ore.rules {
		if r.Name == cfg.Name {
			return fmt.Errorf("规则 '%s' 已存在", cfg.Name)
		}
	}
	compiled := compileOutboundRules([]OutboundRuleConfig{cfg})
	if len(compiled) == 0 {
		return fmt.Errorf("规则 '%s' 没有有效的正则模式", cfg.Name)
	}
	ore.rules = append(ore.rules, compiled[0])
	log.Printf("[出站规则] 添加规则: %s (action=%s, patterns=%d)", cfg.Name, cfg.Action, len(compiled[0].Regexps))
	return nil
}

// UpdateRule 更新一条出站规则（内存更新，不涉及持久化）
func (ore *OutboundRuleEngine) UpdateRule(cfg OutboundRuleConfig) error {
	ore.mu.Lock()
	defer ore.mu.Unlock()
	for i, r := range ore.rules {
		if r.Name == cfg.Name {
			compiled := compileOutboundRules([]OutboundRuleConfig{cfg})
			if len(compiled) == 0 {
				return fmt.Errorf("规则 '%s' 没有有效的正则模式", cfg.Name)
			}
			ore.rules[i] = compiled[0]
			log.Printf("[出站规则] 更新规则: %s", cfg.Name)
			return nil
		}
	}
	return fmt.Errorf("规则 '%s' 不存在", cfg.Name)
}

// DeleteRule 删除一条出站规则（内存更新，不涉及持久化）
func (ore *OutboundRuleEngine) DeleteRule(name string) error {
	ore.mu.Lock()
	defer ore.mu.Unlock()
	for i, r := range ore.rules {
		if r.Name == name {
			ore.rules = append(ore.rules[:i], ore.rules[i+1:]...)
			log.Printf("[出站规则] 删除规则: %s", name)
			return nil
		}
	}
	return fmt.Errorf("规则 '%s' 不存在", name)
}

type OutboundDetectResult struct {
	Action        string
	RuleName      string
	Reason        string
	Message       string   // v3.6 自定义拦截提示
	ShadowReasons []string // 影子模式命中的规则名
}

func (ore *OutboundRuleEngine) Detect(text string) OutboundDetectResult {
	ore.mu.RLock(); defer ore.mu.RUnlock()
	result := OutboundDetectResult{Action: "pass"}
	if text == "" { return result }

	// v3.6: collect all matching rules and pick the one with highest priority
	type matchedOutbound struct {
		Action     string
		RuleName   string
		Reason     string
		Message    string
		Priority   int
		ShadowMode bool
	}
	var matches []matchedOutbound
	allRules := make([]OutboundRule, 0, len(ore.rules)+len(ore.globalTemplateRules))
	allRules = append(allRules, ore.rules...)
	allRules = append(allRules, ore.globalTemplateRules...)

	for _, rule := range allRules {
		// 跳过禁用的规则
		if !rule.Enabled { continue }
		for _, compiled := range rule.Regexps {
			if compiled.MatchString(text) {
				matches = append(matches, matchedOutbound{
					Action:     rule.Action,
					RuleName:   rule.Name,
					Reason:     "outbound_" + rule.Action + ":" + rule.Name,
					Message:    rule.Message,
					Priority:   rule.Priority,
					ShadowMode: rule.ShadowMode,
				})
				break // one match per rule is enough
			}
		}
	}

	if len(matches) == 0 {
		return result
	}

	// Sort by priority desc, then by action weight desc
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return actionWeight(matches[i].Action) > actionWeight(matches[j].Action)
	})

	// 分离影子模式和正常规则
	var normalMatches []matchedOutbound
	var shadowMatches []matchedOutbound
	for _, m := range matches {
		if m.ShadowMode {
			shadowMatches = append(shadowMatches, m)
		} else {
			normalMatches = append(normalMatches, m)
		}
	}

	// 影子模式只记录
	for _, m := range shadowMatches {
		result.ShadowReasons = append(result.ShadowReasons, m.RuleName)
	}

	// 正常规则决定最终 action
	if len(normalMatches) > 0 {
		winner := normalMatches[0]
		result.Action = winner.Action
		result.RuleName = winner.RuleName
		result.Reason = winner.Reason
		result.Message = winner.Message
	}

	return result
}

// ============================================================
// 规则命中率统计（v3.6）
// ============================================================

// RuleHitDetail 单条规则命中详情
type RuleHitDetail struct {
	Name    string `json:"name"`
	Hits    int64  `json:"hits"`
	LastHit string `json:"last_hit,omitempty"`
	Group   string `json:"group,omitempty"` // v3.11 规则分组标签
}

// RuleHitStats 规则命中率统计（线程安全）
type RuleHitStats struct {
	mu       sync.RWMutex
	hits     map[string]int64     // key: rule_name, value: hit count
	lastHit  map[string]time.Time // key: rule_name, value: last hit time
	groups   map[string]string    // key: rule_name, value: group (v3.11)
}

func NewRuleHitStats() *RuleHitStats {
	return &RuleHitStats{
		hits:    make(map[string]int64),
		lastHit: make(map[string]time.Time),
		groups:  make(map[string]string),
	}
}

func (rhs *RuleHitStats) Record(ruleName string) {
	rhs.mu.Lock()
	rhs.hits[ruleName]++
	rhs.lastHit[ruleName] = time.Now()
	rhs.mu.Unlock()
}

// RecordWithGroup 记录命中并关联分组（v3.11）
func (rhs *RuleHitStats) RecordWithGroup(ruleName, group string) {
	rhs.mu.Lock()
	rhs.hits[ruleName]++
	rhs.lastHit[ruleName] = time.Now()
	if group != "" {
		rhs.groups[ruleName] = group
	}
	rhs.mu.Unlock()
}

// SetRuleGroup 设置规则的分组信息（v3.11）
func (rhs *RuleHitStats) SetRuleGroup(ruleName, group string) {
	rhs.mu.Lock()
	rhs.groups[ruleName] = group
	rhs.mu.Unlock()
}

func (rhs *RuleHitStats) Get() map[string]int64 {
	rhs.mu.RLock()
	defer rhs.mu.RUnlock()
	cp := make(map[string]int64, len(rhs.hits))
	for k, v := range rhs.hits {
		cp[k] = v
	}
	return cp
}

func (rhs *RuleHitStats) GetDetails() []RuleHitDetail {
	rhs.mu.RLock()
	defer rhs.mu.RUnlock()
	details := make([]RuleHitDetail, 0, len(rhs.hits))
	for name, count := range rhs.hits {
		d := RuleHitDetail{Name: name, Hits: count}
		if t, ok := rhs.lastHit[name]; ok {
			d.LastHit = t.UTC().Format(time.RFC3339)
		}
		if g, ok := rhs.groups[name]; ok {
			d.Group = g
		}
		details = append(details, d)
	}
	// Sort by hits descending
	sort.Slice(details, func(i, j int) bool {
		return details[i].Hits > details[j].Hits
	})
	return details
}

// GetDetailsByGroup 按分组筛选规则命中详情（v3.11）
func (rhs *RuleHitStats) GetDetailsByGroup(group string) []RuleHitDetail {
	rhs.mu.RLock()
	defer rhs.mu.RUnlock()
	details := make([]RuleHitDetail, 0)
	for name, count := range rhs.hits {
		g := rhs.groups[name]
		if g != group {
			continue
		}
		d := RuleHitDetail{Name: name, Hits: count, Group: g}
		if t, ok := rhs.lastHit[name]; ok {
			d.LastHit = t.UTC().Format(time.RFC3339)
		}
		details = append(details, d)
	}
	sort.Slice(details, func(i, j int) bool {
		return details[i].Hits > details[j].Hits
	})
	return details
}

func (rhs *RuleHitStats) TotalHits() int64 {
	rhs.mu.RLock()
	defer rhs.mu.RUnlock()
	var total int64
	for _, v := range rhs.hits {
		total += v
	}
	return total
}

func (rhs *RuleHitStats) Reset() {
	rhs.mu.Lock()
	rhs.hits = make(map[string]int64)
	rhs.lastHit = make(map[string]time.Time)
	rhs.groups = make(map[string]string)
	rhs.mu.Unlock()
}

