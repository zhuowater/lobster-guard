// detect.go — RuleEngine（AC 自动机+正则）、OutboundRuleEngine、PII 检测、规则绑定
// lobster-guard v4.0 代码拆分
package main

import (
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
type Rule struct { Name string; Level RuleLevel; Category string; Priority int; Message string; Group string }

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
	PatternsCount int    `json:"patterns_count"`
	Action        string `json:"action"`
	Category      string `json:"category"`
	Priority      int    `json:"priority"`
	Message       string `json:"message,omitempty"`
	Type          string `json:"type,omitempty"`  // v3.11 规则类型
	Group         string `json:"group,omitempty"` // v3.11 规则分组
}

// RegexRule v3.11 正则规则（独立于 AC 自动机）
type RegexRule struct {
	Name     string
	Pattern  *regexp.Regexp
	Level    RuleLevel
	Category string
	Priority int
	Message  string
	Group    string
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
			rules = append(rules, Rule{Name: cfg.Name, Level: level, Category: cfg.Category, Priority: cfg.Priority, Message: cfg.Message, Group: cfg.Group})
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
				Name:     cfg.Name,
				Pattern:  compiled,
				Level:    level,
				Category: cfg.Category,
				Priority: cfg.Priority,
				Message:  cfg.Message,
				Group:    cfg.Group,
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
		summaries[i] = InboundRuleSummary{
			Name: cfg.Name, PatternsCount: len(cfg.Patterns),
			Action: cfg.Action, Category: cfg.Category,
			Priority: cfg.Priority, Message: cfg.Message,
			Type: ruleType, Group: cfg.Group,
		}
	}
	return summaries
}

type DetectResult struct { Action string; Reasons []string; PIIs []string; Message string; MatchedRules []string }

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
		Name     string
		Level    RuleLevel
		Priority int
		Message  string
		Action   string
	}
	matchesByName := make(map[string]*matchedRule)

	for _, idx := range ac.Search(text) {
		if idx < 0 || idx >= len(rules) { continue }
		rule := rules[idx]
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
				Action: action,
			}
		}
	}

	// v3.11: 正则规则匹配（在 AC 自动机之后）
	for _, rr := range regexRules {
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
				Action: action,
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

		// The winning rule determines the action
		winner := matches[0]
		r.Action = winner.Action
		r.Message = winner.Message

		// Collect all matched rule names as reasons
		for _, m := range matches {
			r.Reasons = append(r.Reasons, m.Name)
			r.MatchedRules = append(r.MatchedRules, m.Name)
		}
	}

	// PII detection
	for i, pat := range re.piiRe {
		if pat.MatchString(text) { r.PIIs = append(r.PIIs, re.piiNames[i]) }
	}
	if len(r.PIIs) > 0 && r.Action == "pass" {
		r.Action = "warn"; r.Reasons = append(r.Reasons, "pii_detected")
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
			Name:     c.Name,
			Patterns: make([]string, len(c.Patterns)),
			Action:   c.Action,
			Category: c.Category,
			Priority: c.Priority,
			Message:  c.Message,
			Type:     c.Type,
			Group:    c.Group,
		}
		copy(rc.Patterns, c.Patterns)
		cp[i] = rc
	}
	return cp
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

// ============================================================
// 出站规则引擎 v2.0（block/warn/log）
// ============================================================

type OutboundRule struct {
	Name     string
	Regexps  []*regexp.Regexp
	Action   string
	Priority int
	Message  string
}

type OutboundRuleEngine struct {
	mu    sync.RWMutex
	rules []OutboundRule
}

func NewOutboundRuleEngine(configs []OutboundRuleConfig) *OutboundRuleEngine {
	return &OutboundRuleEngine{rules: compileOutboundRules(configs)}
}

func compileOutboundRules(configs []OutboundRuleConfig) []OutboundRule {
	var rules []OutboundRule
	for _, c := range configs {
		rule := OutboundRule{Name: c.Name, Action: c.Action, Priority: c.Priority, Message: c.Message}
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
		configs[i] = OutboundRuleConfig{
			Name:     rule.Name,
			Patterns: patterns,
			Action:   rule.Action,
			Priority: rule.Priority,
			Message:  rule.Message,
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
	Action   string
	RuleName string
	Reason   string
	Message  string // v3.6 自定义拦截提示
}

func (ore *OutboundRuleEngine) Detect(text string) OutboundDetectResult {
	ore.mu.RLock(); defer ore.mu.RUnlock()
	result := OutboundDetectResult{Action: "pass"}
	if text == "" { return result }

	// v3.6: collect all matching rules and pick the one with highest priority
	type matchedOutbound struct {
		Action   string
		RuleName string
		Reason   string
		Message  string
		Priority int
	}
	var matches []matchedOutbound

	for _, rule := range ore.rules {
		for _, compiled := range rule.Regexps {
			if compiled.MatchString(text) {
				matches = append(matches, matchedOutbound{
					Action:   rule.Action,
					RuleName: rule.Name,
					Reason:   "outbound_" + rule.Action + ":" + rule.Name,
					Message:  rule.Message,
					Priority: rule.Priority,
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

	winner := matches[0]
	return OutboundDetectResult{
		Action:   winner.Action,
		RuleName: winner.RuleName,
		Reason:   winner.Reason,
		Message:  winner.Message,
	}
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

