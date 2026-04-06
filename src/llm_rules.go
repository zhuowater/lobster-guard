// llm_rules.go — LLM 侧规则引擎（AC 自动机 + 正则 + 影子模式 + 行业模板 + 租户绑定）
// lobster-guard v10.0 / v28.0
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// LLM 规则模型
// ============================================================

// LLMRule 一条 LLM 侧规则
type LLMRule struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Category    string   `yaml:"category" json:"category"`       // prompt_injection / pii_leak / sensitive_topic / token_abuse / custom
	Direction   string   `yaml:"direction" json:"direction"`     // request / response / both
	Type        string   `yaml:"type" json:"type"`               // keyword / regex
	Patterns    []string `yaml:"patterns" json:"patterns"`
	Action      string   `yaml:"action" json:"action"`           // log / warn / block / rewrite
	RewriteTo   string   `yaml:"rewrite_to" json:"rewrite_to"`   // 仅 action=rewrite 时有效
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	Priority    int      `yaml:"priority" json:"priority"`
	Severity    string   `yaml:"severity" json:"severity,omitempty"`   // high / medium / low
	ShadowMode  bool     `yaml:"shadow_mode" json:"shadow_mode"` // 影子模式：只记录不执行
}

// LLMRuleMatch 规则匹配结果
type LLMRuleMatch struct {
	RuleID      string `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	Category    string `json:"category"`
	Action      string `json:"action"`
	Pattern     string `json:"pattern"`
	MatchedText string `json:"matched_text"`
	ShadowMode  bool   `json:"shadow_mode"`
	Priority    int    `json:"priority"`
	RewriteTo   string `json:"rewrite_to,omitempty"`
}

// LLMRuleHit 命中统计
type LLMRuleHit struct {
	Count      int64     `json:"count"`
	LastHit    time.Time `json:"last_hit"`
	ShadowHits int64    `json:"shadow_hits"` // 影子模式下的命中
}

// LLMRuleTemplate LLM 规则行业模板（v28.0）
type LLMRuleTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // industry / security / compliance
	Rules       []LLMRule `json:"rules"`
	BuiltIn     bool      `json:"built_in"`
	Enabled     bool      `json:"enabled"` // v30.0: 全局开关，启用后对所有流量生效
}

// compiledLLMRegexRule 编译后的正则规则
type compiledLLMRegexRule struct {
	ruleID    string
	ruleName  string
	category  string
	action    string
	rewriteTo string
	pattern   *regexp.Regexp
	rawPattern string
	priority  int
	shadowMode bool
}

// ============================================================
// LLM 规则引擎
// ============================================================

// LLMRuleEngine LLM 侧规则引擎
type LLMRuleEngine struct {
	mu        sync.RWMutex
	rules     []LLMRule
	// keyword 规则用 AC 自动机
	reqAC     *AhoCorasick // 请求方向
	respAC    *AhoCorasick // 响应方向
	// AC 自动机 pattern → rule 映射
	reqACRules  []llmACEntry
	respACRules []llmACEntry
	// regex 规则
	reqRegex  []*compiledLLMRegexRule
	respRegex []*compiledLLMRegexRule
	// 命中统计
	hits map[string]*LLMRuleHit
	// Issue #7 fix: 持久化支持
	db *sql.DB
	// v28.0 租户专属 LLM 规则
	tenantRules     map[string][]LLMRule              // tenantID -> rules
	tenantReqAC     map[string]*AhoCorasick           // 编译后的请求方向 AC
	tenantRespAC    map[string]*AhoCorasick           // 编译后的响应方向 AC
	tenantReqACRules  map[string][]llmACEntry
	tenantRespACRules map[string][]llmACEntry
	tenantReqRegex  map[string][]*compiledLLMRegexRule
	tenantRespRegex map[string][]*compiledLLMRegexRule
	tenantDB        *sql.DB                           // 租户规则持久化
	// v28.0 LLM 规则模板 DB
	templateDB *sql.DB
	// v31.1: LLM auto-review（复用入站的 AutoReviewManager）
	autoReviewMgr *AutoReviewManager
	// v30.0 全局启用的行业模板规则
	globalTemplateRules  []LLMRule
	globalTplReqAC       *AhoCorasick
	globalTplRespAC      *AhoCorasick
	globalTplReqACRules  []llmACEntry
	globalTplRespACRules []llmACEntry
	globalTplReqRegex    []compiledLLMRegexRule
	globalTplRespRegex   []compiledLLMRegexRule
}

// llmACEntry AC 自动机中每个 pattern 对应的规则信息
type llmACEntry struct {
	ruleID     string
	ruleName   string
	category   string
	action     string
	rewriteTo  string
	pattern    string
	priority   int
	shadowMode bool
}

// defaultLLMRules 内置默认规则集
var defaultLLMRules = []LLMRule{
	// 提示词注入（请求方向）
	{ID: "llm-pi-001", Name: "提取系统提示词", Category: "prompt_injection", Direction: "request", Type: "keyword",
		Patterns: []string{"reveal your system prompt", "show me your instructions", "what are your rules", "ignore previous instructions", "disregard above"},
		Action: "warn", Enabled: true, Priority: 10},

	{ID: "llm-pi-002", Name: "越狱攻击", Category: "prompt_injection", Direction: "request", Type: "keyword",
		Patterns: []string{"DAN mode", "developer mode", "no restrictions", "pretend you have no guidelines", "act as an unrestricted"},
		Action: "block", Enabled: true, Priority: 10},

	{ID: "llm-pi-003", Name: "提示词注入(正则)", Category: "prompt_injection", Direction: "request", Type: "regex",
		Patterns: []string{`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions|rules|guidelines)`, `(?i)(you\s+are|act\s+as)\s+.{0,30}(unrestricted|unfiltered|without\s+rules)`},
		Action: "warn", Enabled: true, Priority: 5},

	// 个人信息泄露（响应方向）
	{ID: "llm-pii-001", Name: "响应中检测到信用卡号", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{`\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13})\b`},
		Action: "rewrite", RewriteTo: "[已脱敏-信用卡]", Enabled: true, Priority: 20},

	{ID: "llm-pii-002", Name: "响应中检测到社保号", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{`\b\d{3}-\d{2}-\d{4}\b`},
		Action: "rewrite", RewriteTo: "[已脱敏-社保号]", Enabled: true, Priority: 20},

	{ID: "llm-pii-003", Name: "响应中检测到API密钥", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{`(?i)(sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}|AKIA[0-9A-Z]{16})`},
		Action: "rewrite", RewriteTo: "[已脱敏-密钥]", Enabled: true, Priority: 25},

	// 中国个人信息模式
	{ID: "llm-pii-004", Name: "响应中检测到身份证号", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{`(?:\D|^)([1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx])(?:\D|$)`},
		Action: "warn", Enabled: true, Severity: "high", Priority: 22},
	{ID: "llm-pii-005", Name: "响应中检测到手机号", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{`(?:\D|^)(1[3-9]\d{9})(?:\D|$)`},
		Action: "warn", Enabled: true, Severity: "medium", Priority: 21},

	// 敏感话题（双向）
	{ID: "llm-st-001", Name: "敏感话题检测", Category: "sensitive_topic", Direction: "both", Type: "keyword",
		Patterns: []string{"how to make a bomb", "synthesize drugs", "create malware", "hack into"},
		Action: "block", Enabled: true, Priority: 15},

	// Token滥用（请求方向，检测超长重复内容）
	{ID: "llm-ta-001", Name: "超长重复字符攻击", Category: "token_abuse", Direction: "request", Type: "regex",
		Patterns: []string{`(?i)(AAAA{100,}|\.{100,}|={100,}|\s{200,})`},
		Action: "warn", Enabled: true, Priority: 5},

	// 响应方向 — 系统提示词泄露检测
	{ID: "llm-resp-001", Name: "系统提示词泄露", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{
			`(?i)my\s+system\s+prompt\s+is`,
			`(?i)my\s+instructions?\s+(are|is)`,
			`(?i)here\s+(is|are)\s+my\s+(system\s+)?prompt`,
			`(?i)i\s+was\s+instructed\s+to`,
		},
		Action: "warn", Enabled: true, Priority: 15},

	// 响应方向 — 恶意代码/命令注入检测
	{ID: "llm-resp-002", Name: "响应中检测到恶意代码", Category: "sensitive_topic", Direction: "response", Type: "regex",
		Patterns: []string{
			`(?i)os\.system\s*\(\s*['\"].*rm\s+-rf`,
			`(?i)subprocess\.call\s*\(\s*\[.*curl.*bash`,
			`(?i)exec\s*\(\s*['\"].*wget.*\|.*sh`,
			`(?i)\beval\s*\(\s*['\"].*fetch\(`,
		},
		Action: "block", Enabled: true, Priority: 20},

	// 响应方向 — 凭据/密钥泄露
	{ID: "llm-resp-003", Name: "响应中检测到凭据泄露", Category: "pii_leak", Direction: "response", Type: "regex",
		Patterns: []string{
			`(?i)(database|db)\s+password\s+(is|=)\s*\S+`,
			`(?i)api[_ ]key\s+(is|=)\s*\S+`,
			`(?i)(access|auth)[_ ]token\s+(is|=)\s*\S+`,
		},
		Action: "block", Enabled: true, Priority: 20},
}

// mergeLLMRuleDefaults 合并用户配置与默认规则（用户同 ID 规则覆盖默认）
func mergeLLMRuleDefaults(userRules []LLMRule) []LLMRule {
	if len(userRules) == 0 {
		return defaultLLMRules
	}
	userIDs := make(map[string]bool)
	for _, r := range userRules {
		userIDs[r.ID] = true
	}
	merged := make([]LLMRule, len(userRules))
	copy(merged, userRules)
	for _, d := range defaultLLMRules {
		if !userIDs[d.ID] {
			merged = append(merged, d)
		}
	}
	return merged
}

// NewLLMRuleEngine 创建 LLM 规则引擎
func NewLLMRuleEngine(rules []LLMRule) *LLMRuleEngine {
	e := &LLMRuleEngine{
		hits: make(map[string]*LLMRuleHit),
	}
	e.buildIndex(rules)
	return e
}

// SetDB 设置数据库引用，启用命中计数持久化
func (e *LLMRuleEngine) SetDB(db *sql.DB) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.db = db

	// 创建表
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_rule_hits (
		rule_id TEXT PRIMARY KEY,
		count INTEGER DEFAULT 0,
		shadow_hits INTEGER DEFAULT 0,
		last_hit TEXT
	)`)

	// 从 DB 恢复命中计数
	rows, err := db.Query(`SELECT rule_id, count, shadow_hits, last_hit FROM llm_rule_hits`)
	if err != nil {
		log.Printf("[LLM规则] 命中计数恢复失败: %v", err)
		return
	}
	defer rows.Close()
	restored := 0
	for rows.Next() {
		var ruleID, lastHitStr string
		var count, shadowHits int64
		if rows.Scan(&ruleID, &count, &shadowHits, &lastHitStr) != nil {
			continue
		}
		hit, ok := e.hits[ruleID]
		if !ok {
			hit = &LLMRuleHit{}
			e.hits[ruleID] = hit
		}
		hit.Count = count
		hit.ShadowHits = shadowHits
		if t, err := time.Parse(time.RFC3339, lastHitStr); err == nil {
			hit.LastHit = t
		}
		restored++
	}
	if restored > 0 {
		log.Printf("[LLM规则] 恢复命中计数: %d 条规则", restored)
	}
}

// buildIndex 从规则列表构建内部索引（AC 自动机 + 正则）
func (e *LLMRuleEngine) buildIndex(rules []LLMRule) {
	e.rules = make([]LLMRule, len(rules))
	copy(e.rules, rules)

	var reqPatterns, respPatterns []string
	var reqEntries, respEntries []llmACEntry
	var reqRegex, respRegex []*compiledLLMRegexRule

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		entry := llmACEntry{
			ruleID:     rule.ID,
			ruleName:   rule.Name,
			category:   rule.Category,
			action:     rule.Action,
			rewriteTo:  rule.RewriteTo,
			priority:   rule.Priority,
			shadowMode: rule.ShadowMode,
		}

		isReq := rule.Direction == "request" || rule.Direction == "both"
		isResp := rule.Direction == "response" || rule.Direction == "both"

		if rule.Type == "regex" {
			for _, p := range rule.Patterns {
				compiled, err := regexp.Compile(p)
				if err != nil {
					log.Printf("[LLM规则] 正则编译失败 rule=%s pattern=%q: %v（跳过）", rule.ID, p, err)
					continue
				}
				cr := &compiledLLMRegexRule{
					ruleID:     rule.ID,
					ruleName:   rule.Name,
					category:   rule.Category,
					action:     rule.Action,
					rewriteTo:  rule.RewriteTo,
					pattern:    compiled,
					rawPattern: p,
					priority:   rule.Priority,
					shadowMode: rule.ShadowMode,
				}
				if isReq {
					reqRegex = append(reqRegex, cr)
				}
				if isResp {
					respRegex = append(respRegex, cr)
				}
			}
		} else {
			// keyword 类型：加入 AC 自动机
			for _, p := range rule.Patterns {
				e := entry
				e.pattern = p
				if isReq {
					reqPatterns = append(reqPatterns, p)
					reqEntries = append(reqEntries, e)
				}
				if isResp {
					respPatterns = append(respPatterns, p)
					respEntries = append(respEntries, e)
				}
			}
		}
	}

	e.reqAC = NewAhoCorasick(reqPatterns)
	e.reqACRules = reqEntries
	e.respAC = NewAhoCorasick(respPatterns)
	e.respACRules = respEntries
	e.reqRegex = reqRegex
	e.respRegex = respRegex

	// 初始化命中统计（保留现有计数）
	for _, rule := range rules {
		if _, ok := e.hits[rule.ID]; !ok {
			e.hits[rule.ID] = &LLMRuleHit{}
		}
	}

	log.Printf("[LLM规则] buildIndex 完成: 总规则%d, req_ac=%d req_regex=%d resp_ac=%d resp_regex=%d",
		len(rules), len(reqEntries), len(reqRegex), len(respEntries), len(respRegex))
}

// UpdateRules 热更新规则（并发安全）
func (e *LLMRuleEngine) UpdateRules(rules []LLMRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.buildIndex(rules)
	log.Printf("[LLM规则] 热更新完成，共 %d 条规则", len(rules))
}

// GetRules 返回当前规则列表副本
func (e *LLMRuleEngine) GetRules() []LLMRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cp := make([]LLMRule, len(e.rules))
	copy(cp, e.rules)
	return cp
}

// GetHits 返回命中统计副本
func (e *LLMRuleEngine) GetHits() map[string]*LLMRuleHit {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cp := make(map[string]*LLMRuleHit, len(e.hits))
	for k, v := range e.hits {
		cp[k] = &LLMRuleHit{
			Count:      v.Count,
			LastHit:    v.LastHit,
			ShadowHits: v.ShadowHits,
		}
	}
	return cp
}

// recordHit 记录命中（内部调用，需要持有写锁）
func (e *LLMRuleEngine) recordHit(ruleID string, shadow bool) {
	hit, ok := e.hits[ruleID]
	if !ok {
		hit = &LLMRuleHit{}
		e.hits[ruleID] = hit
	}
	if shadow {
		hit.ShadowHits++
	} else {
		hit.Count++
	}
	hit.LastHit = time.Now()

	// Issue #7 fix: 异步持久化到 DB
	if e.db != nil {
		go func(rid string, c, s int64, t time.Time) {
			e.db.Exec(`INSERT INTO llm_rule_hits (rule_id, count, shadow_hits, last_hit)
				VALUES (?, ?, ?, ?) ON CONFLICT(rule_id) DO UPDATE SET
				count=?, shadow_hits=?, last_hit=?`,
				rid, c, s, t.Format(time.RFC3339), c, s, t.Format(time.RFC3339))
		}(ruleID, hit.Count, hit.ShadowHits, hit.LastHit)
	}
}

// CheckRequest 检测请求内容，返回匹配结果
func (e *LLMRuleEngine) CheckRequest(content string) []LLMRuleMatch {
	e.mu.RLock()
	reqAC := e.reqAC
	reqACRules := e.reqACRules
	reqRegex := e.reqRegex
	e.mu.RUnlock()

	var matches []LLMRuleMatch
	seen := make(map[string]bool) // 去重：同一规则只记录一次

	// AC 自动机匹配
	for _, idx := range reqAC.Search(content) {
		if idx < 0 || idx >= len(reqACRules) {
			continue
		}
		entry := reqACRules[idx]
		if seen[entry.ruleID] {
			continue
		}
		seen[entry.ruleID] = true
		matches = append(matches, LLMRuleMatch{
			RuleID:      entry.ruleID,
			RuleName:    entry.ruleName,
			Category:    entry.category,
			Action:      entry.action,
			Pattern:     entry.pattern,
			MatchedText: entry.pattern, // keyword 匹配的文本就是 pattern 本身
			ShadowMode:  entry.shadowMode,
			Priority:    entry.priority,
			RewriteTo:   entry.rewriteTo,
		})
		// 记录命中
		e.mu.Lock()
		e.recordHit(entry.ruleID, entry.shadowMode)
		e.mu.Unlock()
	}

	// 正则匹配
	for _, cr := range reqRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID:      cr.ruleID,
			RuleName:    cr.ruleName,
			Category:    cr.category,
			Action:      cr.action,
			Pattern:     cr.rawPattern,
			MatchedText: matchedText,
			ShadowMode:  cr.shadowMode,
			Priority:    cr.priority,
			RewriteTo:   cr.rewriteTo,
		})
		e.mu.Lock()
		e.recordHit(cr.ruleID, cr.shadowMode)
		e.mu.Unlock()
	}

	return matches
}

// CheckResponse 检测响应内容，返回匹配结果
func (e *LLMRuleEngine) CheckResponse(content string) []LLMRuleMatch {
	e.mu.RLock()
	respAC := e.respAC
	respACRules := e.respACRules
	respRegex := e.respRegex
	e.mu.RUnlock()

	var matches []LLMRuleMatch
	seen := make(map[string]bool)

	// AC 自动机匹配
	for _, idx := range respAC.Search(content) {
		if idx < 0 || idx >= len(respACRules) {
			continue
		}
		entry := respACRules[idx]
		if seen[entry.ruleID] {
			continue
		}
		seen[entry.ruleID] = true
		matches = append(matches, LLMRuleMatch{
			RuleID:      entry.ruleID,
			RuleName:    entry.ruleName,
			Category:    entry.category,
			Action:      entry.action,
			Pattern:     entry.pattern,
			MatchedText: entry.pattern,
			ShadowMode:  entry.shadowMode,
			Priority:    entry.priority,
			RewriteTo:   entry.rewriteTo,
		})
		e.mu.Lock()
		e.recordHit(entry.ruleID, entry.shadowMode)
		e.mu.Unlock()
	}

	// 正则匹配
	for _, cr := range respRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID:      cr.ruleID,
			RuleName:    cr.ruleName,
			Category:    cr.category,
			Action:      cr.action,
			Pattern:     cr.rawPattern,
			MatchedText: matchedText,
			ShadowMode:  cr.shadowMode,
			Priority:    cr.priority,
			RewriteTo:   cr.rewriteTo,
		})
		e.mu.Lock()
		e.recordHit(cr.ruleID, cr.shadowMode)
		e.mu.Unlock()
	}

	return matches
}

// HasRewriteRuleForResponse 检查是否存在针对响应侧的 rewrite 规则（用于决定 SSE 是否走缓冲模式）
func (e *LLMRuleEngine) HasRewriteRuleForResponse() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, r := range e.rules {
		if r.Enabled && !r.ShadowMode && r.Action == "rewrite" &&
			(r.Direction == "response" || r.Direction == "both" || r.Direction == "") {
			return true
		}
	}
	return false
}

// ApplyRewrite 对内容应用 rewrite 规则，返回修改后的内容
func (e *LLMRuleEngine) ApplyRewrite(content string, matches []LLMRuleMatch) string {
	e.mu.RLock()
	respRegex := e.respRegex
	e.mu.RUnlock()

	result := content
	for _, m := range matches {
		if m.Action != "rewrite" || m.ShadowMode || m.RewriteTo == "" {
			continue
		}
		// 对 regex 规则使用正则替换
		for _, cr := range respRegex {
			if cr.ruleID == m.RuleID {
				result = cr.pattern.ReplaceAllString(result, m.RewriteTo)
			}
		}
		// 对 keyword 规则使用字符串替换（忽略大小写）
		if !strings.Contains(m.Pattern, "(") { // 简单判断非正则
			result = replaceAllInsensitive(result, m.Pattern, m.RewriteTo)
		}
	}
	return result
}

// replaceAllInsensitive 忽略大小写替换所有出现
func replaceAllInsensitive(s, old, replacement string) string {
	lower := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	var result strings.Builder
	i := 0
	for {
		idx := strings.Index(lower[i:], lowerOld)
		if idx == -1 {
			result.WriteString(s[i:])
			break
		}
		result.WriteString(s[i : i+idx])
		result.WriteString(replacement)
		i += idx + len(old)
	}
	return result.String()
}

// HighestPriorityAction 从匹配结果中找到最高优先级的非影子模式动作
func HighestPriorityAction(matches []LLMRuleMatch) (action string, match *LLMRuleMatch) {
	for i := range matches {
		m := &matches[i]
		if m.ShadowMode {
			continue
		}
		if match == nil || m.Priority > match.Priority ||
			(m.Priority == match.Priority && actionWeight(m.Action) > actionWeight(action)) {
			action = m.Action
			match = m
		}
	}
	return
}

// ============================================================
// v28.0 LLM 规则行业模板
// ============================================================

// getDefaultLLMTemplates 返回内置 LLM 规则行业模板
func getDefaultLLMTemplates() []LLMRuleTemplate {
	return []LLMRuleTemplate{
		// ---- 芯片行业 ----
		{
			ID:          "tpl-llm-semiconductor",
			Name:        "芯片行业 LLM 规则",
			Description: "芯片/半导体行业专属 LLM 规则，覆盖请求侧 IP 查询拦截和响应侧 IP 泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-semi-req-001", Name: "芯片IP查询拦截", Description: "检测请求中查询芯片IP的关键词",
					Category: "ip_protection", Direction: "request", Type: "keyword",
					Patterns: []string{
						"RTL代码", "帮我查RTL", "导出GDSII", "发送晶圆数据", "Verilog源码",
						"IP核设计", "芯片版图", "光罩数据", "流片信息", "EDA配置",
						"RTL source", "export GDSII", "send wafer data", "chip layout",
						"tape-out data", "netlist export", "HDL source code", "foundry process data",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-semi-req-002", Name: "芯片出口管制查询拦截", Description: "检测请求中涉及出口管制的查询",
					Category: "export_control", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查EAR管制", "ITAR清单", "出口管制名单", "实体清单查询", "瓦森纳协议",
						"export control list", "EAR classification", "ITAR restricted", "entity list lookup",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-semi-resp-001", Name: "响应中芯片IP泄露检测", Description: "检测响应中泄露芯片IP的内容",
					Category: "ip_protection", Direction: "response", Type: "keyword",
					Patterns: []string{
						"ISA指令集", "制程节点", "EDA配置文件", "PDK参数", "标准单元库",
						"ISA instruction set", "process node", "EDA configuration", "PDK parameter",
						"standard cell library", "design rule", "SPICE model",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-semi-resp-002", Name: "响应中芯片敏感数据泄露(正则)", Description: "正则检测响应中的制程节点、GDSII文件名等",
					Category: "ip_protection", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)\b\d+\s*nm\s+(process|制程|工艺)`,
						`(?i)\.gds(ii)?\b`,
						`(?i)\b(tsmc|smic|samsung)\s+(foundry|代工)`,
					},
					Action: "warn", Enabled: true, Priority: 12,
				},
			},
		},
		// ---- 银行/支付 ----
		{
			ID:          "tpl-llm-financial",
			Name:        "银行/支付 LLM 规则",
			Description: "银行/支付行业专属 LLM 规则，覆盖请求侧敏感查询拦截和响应侧数据泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-fin-req-001", Name: "金融敏感查询拦截", Description: "检测请求中查询金融数据的关键词",
					Category: "financial_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查账户余额", "导出交易流水", "查询征信", "查贷款审批", "导出信用报告",
						"查基金净值", "导出持仓数据", "查SWIFT代码",
						"query account balance", "export transaction", "check credit score",
						"export portfolio", "lookup SWIFT code", "routing number query",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-fin-req-002", Name: "内幕交易相关查询拦截", Description: "检测请求中涉及内幕交易或操纵市场的查询",
					Category: "compliance", Direction: "request", Type: "keyword",
					Patterns: []string{
						"内幕交易", "操纵市场", "抢先交易", "利用未公开信息",
						"insider trading", "market manipulation", "front running", "MNPI",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-fin-resp-001", Name: "响应中金融数据泄露检测", Description: "检测响应中泄露金融数据的内容",
					Category: "financial_data", Direction: "response", Type: "keyword",
					Patterns: []string{
						"账户余额", "交易流水", "征信报告", "授信额度", "信用评分",
						"SWIFT代码", "清算行号",
						"account balance", "transaction history", "credit report",
						"credit score", "SWIFT code", "routing number",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-fin-resp-002", Name: "响应中银行账号泄露(正则)", Description: "正则检测响应中的银行账号和卡号",
					Category: "financial_data", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)\b(SWIFT|BIC)\s*[:：]\s*[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?\b`,
						`(?i)\b(account|账号)\s*[:：]\s*\d{10,20}\b`,
					},
					Action: "warn", Enabled: true, Priority: 18,
				},
			},
		},
		// ---- 医疗行业 ----
		{
			ID:          "tpl-llm-healthcare",
			Name:        "医疗行业 LLM 规则",
			Description: "医疗行业专属 LLM 规则，覆盖请求侧患者数据查询拦截和响应侧 PHI 泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-health-req-001", Name: "患者数据查询拦截", Description: "检测请求中查询患者数据的关键词",
					Category: "phi", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查病历", "导出处方", "查诊断报告", "查化验单", "导出患者数据",
						"查手术记录", "导出医嘱", "查影像报告", "查出院小结",
						"query patient record", "export prescription", "lookup diagnosis",
						"export medical history", "patient data export", "lab result query",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-health-req-002", Name: "管制药品查询拦截", Description: "检测请求中涉及管制药品的查询",
					Category: "drug_safety", Direction: "request", Type: "keyword",
					Patterns: []string{
						"开具管制药品", "麻醉药品处方", "精神药品剂量", "管制物质",
						"prescribe controlled substance", "narcotic prescription", "schedule II drug",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-health-resp-001", Name: "响应中 PHI 泄露检测", Description: "检测响应中泄露患者健康信息(PHI)的内容",
					Category: "phi", Direction: "response", Type: "keyword",
					Patterns: []string{
						"患者姓名", "诊断结果", "处方详情", "化验结果", "手术记录",
						"出院小结", "影像报告", "医嘱内容",
						"patient name", "diagnosis result", "prescription detail",
						"lab result", "surgical record", "discharge summary",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-health-resp-002", Name: "响应中 PHI 敏感模式(正则)", Description: "正则检测响应中患者姓名+诊断、处方详情等组合",
					Category: "phi", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(patient|患者)\s*[:：]\s*\S+.{0,20}(diagnosis|诊断|condition)`,
						`(?i)(prescription|处方)\s*[:：].{0,50}(mg|ml|片|粒|支)`,
						`(?i)HIPAA\s+(violation|breach|违规)`,
					},
					Action: "warn", Enabled: true, Priority: 18,
				},
			},
		},
		// ---- AI合规 ----
		{
			ID:          "tpl-llm-compliance",
			Name:        "AI 合规 LLM 规则",
			Description: "AI 法规合规 LLM 规则，覆盖请求侧违规指令拦截和响应侧歧视/操纵内容检测",
			Category:    "compliance",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-comp-req-001", Name: "AI 法规违规请求拦截", Description: "检测请求中违反 AI 法规的指令",
					Category: "ai_act", Direction: "request", Type: "keyword",
					Patterns: []string{
						"社会信用评分", "实时生物识别", "潜意识操纵", "大规模监控",
						"social credit scoring", "real-time biometric", "subliminal manipulation",
						"mass surveillance", "emotion recognition surveillance",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-comp-req-002", Name: "高风险 AI 应用请求检测", Description: "检测请求中涉及高风险 AI 应用的内容",
					Category: "ai_act", Direction: "request", Type: "keyword",
					Patterns: []string{
						"自动化决策", "信用评分模型", "招聘AI筛选", "预测性执法",
						"automated decision making", "credit scoring model", "recruitment AI screening",
						"predictive policing", "judicial AI",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-comp-resp-001", Name: "响应中歧视性内容检测", Description: "检测响应中包含歧视性或操纵性内容",
					Category: "ai_act", Direction: "response", Type: "keyword",
					Patterns: []string{
						"种族歧视", "性别歧视", "年龄歧视", "残疾歧视",
						"racial discrimination", "gender bias", "age discrimination",
						"disability discrimination",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-comp-resp-002", Name: "响应中操纵性内容检测(正则)", Description: "正则检测响应中的操纵性和误导性内容",
					Category: "ai_act", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(you\s+must|你必须).{0,30}(obey|服从|comply\s+unconditionally)`,
						`(?i)(manipulat|操纵|误导).{0,20}(user|用户|consumer|消费者)`,
						`(?i)(deepfake|深度伪造).{0,20}(generate|生成|create|创建)`,
					},
					Action: "block", Enabled: true, Priority: 20,
				},
			},
		},
		// v29.0: 8个新行业 LLM 模板
		// ---- 法律行业 ----
		{
			ID:          "tpl-llm-legal",
			Name:        "法律行业 LLM 规则",
			Description: "法律行业专属 LLM 规则，覆盖请求侧案件信息查询拦截和响应侧当事人信息泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-legal-req-001", Name: "案件细节查询拦截", Description: "检测请求中查询案件细节的关键词",
					Category: "legal_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询案件详情", "调取案卷", "案件当事人", "查证据材料", "案件卷宗",
						"query case detail", "retrieve case file", "case party information",
						"evidence material", "litigation record", "查诉讼记录",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-legal-req-002", Name: "调取证据链拦截", Description: "检测请求中调取证据链或物证的操作",
					Category: "legal_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"调取证据链", "导出物证", "提取电子证据", "证据保全",
						"retrieve evidence chain", "export physical evidence", "extract digital evidence",
						"evidence preservation", "chain of custody export",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-legal-resp-001", Name: "案件当事人信息泄露检测", Description: "检测响应中泄露案件当事人身份信息",
					Category: "legal_data", Direction: "response", Type: "keyword",
					Patterns: []string{
						"原告姓名", "被告姓名", "当事人身份", "代理律师", "案件编号",
						"plaintiff name", "defendant name", "party identity",
						"attorney of record", "case number", "docket number",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-legal-resp-002", Name: "法律意见未免责声明检测(正则)", Description: "正则检测响应中法律意见缺少免责声明",
					Category: "legal_data", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(legal\s+opinion|法律意见|legal\s+advice|法律建议).{0,200}(?!disclaimer|免责|not\s+constitute)`,
						`(?i)(我(们)?认为|our\s+opinion\s+is).{0,100}(合法|lawful|有效|valid)`,
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
			},
		},
		// ---- 教育行业 ----
		{
			ID:          "tpl-llm-education",
			Name:        "教育行业 LLM 规则",
			Description: "教育行业专属 LLM 规则，覆盖请求侧学生数据查询拦截和响应侧学生 PII / 考试答案泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-edu-req-001", Name: "查询学生成绩拦截", Description: "检测请求中查询学生成绩或学籍的关键词",
					Category: "student_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询学生成绩", "导出成绩单", "学籍查询", "查学生档案", "查FERPA记录",
						"query student grade", "export transcript", "student record lookup",
						"FERPA record query", "enrollment query", "查学生学分",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-edu-req-002", Name: "代写论文/考试作弊拦截", Description: "检测请求中代写论文或协助考试作弊",
					Category: "academic_integrity", Direction: "request", Type: "keyword",
					Patterns: []string{
						"帮我写论文", "代写毕业论文", "考试答案", "帮我作弊", "替考",
						"write my essay", "ghost write thesis", "exam answers for me",
						"help me cheat", "take my exam", "代做作业",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-edu-resp-001", Name: "学生 PII 泄露检测", Description: "检测响应中泄露学生个人身份信息",
					Category: "student_data", Direction: "response", Type: "keyword",
					Patterns: []string{
						"学生姓名", "学号", "家长联系方式", "学生住址", "学生身份证",
						"student name", "student ID", "parent contact", "student address",
						"student SSN", "guardian phone", "学生成绩单",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-edu-resp-002", Name: "考试答案泄露检测(正则)", Description: "正则检测响应中泄露的考试答案内容",
					Category: "exam_security", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(answer\s+key|标准答案|correct\s+answer|参考答案)\s*[:：]`,
						`(?i)(第[一二三四五六七八九十\d]+题|question\s+\d+)\s*[:：].{0,20}(答案|answer)\s*[:：]`,
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
			},
		},
		// ---- 政务/政府 ----
		{
			ID:          "tpl-llm-government",
			Name:        "政务/政府 LLM 规则",
			Description: "政务行业专属 LLM 规则，覆盖请求侧涉密文件查询拦截和响应侧涉密内容泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-gov-req-001", Name: "查询涉密文件拦截", Description: "检测请求中查询涉密文件的操作",
					Category: "classified", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询机密文件", "调取秘密文件", "导出涉密数据", "查内部文件",
						"query classified document", "retrieve secret file", "export confidential data",
						"access restricted document", "查国家秘密",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-gov-req-002", Name: "政策内部讨论检测", Description: "检测请求中涉及政策内部讨论的内容",
					Category: "policy_draft", Direction: "request", Type: "keyword",
					Patterns: []string{
						"政策内部讨论", "内部征求意见", "政策草案内容", "领导批示",
						"internal policy discussion", "internal consultation draft",
						"policy deliberation", "leadership directive", "内部会议纪要",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-gov-resp-001", Name: "涉密内容泄露检测", Description: "检测响应中泄露涉密内容",
					Category: "classified", Direction: "response", Type: "keyword",
					Patterns: []string{
						"机密", "秘密", "内部", "国家秘密", "涉密信息",
						"classified", "confidential", "restricted", "state secret",
						"top secret", "for official use only",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-gov-resp-002", Name: "公文格式内容泄露检测(正则)", Description: "正则检测响应中的公文格式内容",
					Category: "gov_document", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(国发|国办发|部发)\s*[〔\[]\s*\d{4}\s*[〕\]]\s*\d+\s*号`,
						`(?i)(document\s+no|dispatch\s+no|文号)\s*[:：]\s*\S+`,
					},
					Action: "warn", Enabled: true, Priority: 18,
				},
			},
		},
		// ---- 能源/电力 ----
		{
			ID:          "tpl-llm-energy",
			Name:        "能源/电力 LLM 规则",
			Description: "能源行业专属 LLM 规则，覆盖请求侧工控命令拦截和响应侧 SCADA / 电网数据泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-energy-req-001", Name: "下发工控命令拦截", Description: "检测请求中下发 SCADA/工控命令的操作",
					Category: "scada_security", Direction: "request", Type: "keyword",
					Patterns: []string{
						"下发SCADA命令", "执行工控指令", "修改PLC参数", "远程控制RTU",
						"send SCADA command", "execute ICS command", "modify PLC parameter",
						"remote control RTU", "issue control command", "写入工控寄存器",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-energy-req-002", Name: "查询电网拓扑检测", Description: "检测请求中查询电网拓扑或调度数据的操作",
					Category: "grid_security", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询电网拓扑", "导出调度数据", "电网结构图", "变电站配置",
						"query grid topology", "export dispatch data", "power grid structure",
						"substation configuration", "grid map export", "查电力调度",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-energy-resp-001", Name: "SCADA配置泄露检测", Description: "检测响应中泄露 SCADA 配置信息",
					Category: "scada_security", Direction: "response", Type: "keyword",
					Patterns: []string{
						"SCADA配置", "PLC地址", "RTU配置", "工控密码", "DCS参数",
						"SCADA configuration", "PLC address", "RTU config", "ICS credential",
						"DCS parameter", "HMI configuration",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-energy-resp-002", Name: "电网拓扑数据泄露检测(正则)", Description: "正则检测响应中的电网拓扑和调度数据",
					Category: "grid_security", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(substation|变电站|switch\s*yard)\s*[:：]\s*\S+.{0,30}(kV|千伏|MW|兆瓦)`,
						`(?i)(grid\s+topology|电网拓扑|power\s+flow)\s*[:：]`,
					},
					Action: "warn", Enabled: true, Priority: 18,
				},
			},
		},
		// ---- 汽车/自动驾驶 ----
		{
			ID:          "tpl-llm-automotive",
			Name:        "汽车/自动驾驶 LLM 规则",
			Description: "汽车行业专属 LLM 规则，覆盖请求侧 ECU 参数修改拦截和响应侧车辆数据泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-auto-req-001", Name: "修改ECU参数拦截", Description: "检测请求中修改 ECU 参数的操作",
					Category: "ecu_security", Direction: "request", Type: "keyword",
					Patterns: []string{
						"修改ECU参数", "刷写ECU", "ECU标定", "篡改固件",
						"modify ECU parameter", "flash ECU", "ECU calibration",
						"tamper firmware", "reprogram ECU", "写入ECU配置",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-auto-req-002", Name: "查询车辆轨迹检测", Description: "检测请求中查询车辆行驶轨迹的操作",
					Category: "vehicle_tracking", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询车辆轨迹", "导出行驶记录", "车辆定位", "追踪车辆位置",
						"query vehicle trajectory", "export driving record", "vehicle location",
						"track vehicle position", "GPS history export", "查行车记录",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-auto-resp-001", Name: "ECU配置泄露检测", Description: "检测响应中泄露 ECU 配置数据",
					Category: "ecu_security", Direction: "response", Type: "keyword",
					Patterns: []string{
						"ECU标定参数", "ECU固件版本", "CAN总线配置", "UDS诊断密钥",
						"ECU calibration data", "ECU firmware version", "CAN bus config",
						"UDS diagnostic key", "OBD parameter", "ECU密钥",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-auto-resp-002", Name: "车辆轨迹数据泄露检测(正则)", Description: "正则检测响应中的车辆轨迹和 VIN 数据",
					Category: "vehicle_tracking", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(VIN|车架号)\s*[:：]\s*[A-HJ-NPR-Z0-9]{17}`,
						`(?i)(trajectory|轨迹|GPS\s+log)\s*[:：].{0,30}(\d+\.\d+\s*,\s*\d+\.\d+)`,
					},
					Action: "warn", Enabled: true, Priority: 18,
				},
			},
		},
		// ---- 电商/零售 ----
		{
			ID:          "tpl-llm-ecommerce",
			Name:        "电商/零售 LLM 规则",
			Description: "电商行业专属 LLM 规则，覆盖请求侧价格操纵拦截和响应侧定价策略泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-ecom-req-001", Name: "批量修改价格拦截", Description: "检测请求中批量修改价格的操作",
					Category: "price_manipulation", Direction: "request", Type: "keyword",
					Patterns: []string{
						"批量修改价格", "全店调价", "价格批量更新", "修改SKU价格",
						"bulk price update", "mass price change", "update all prices",
						"modify SKU price", "batch pricing", "批量改价",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-ecom-req-002", Name: "爬取竞品检测", Description: "检测请求中爬取竞品数据的操作",
					Category: "competitive_intel", Direction: "request", Type: "keyword",
					Patterns: []string{
						"爬取竞品", "抓取对手价格", "竞品数据采集", "爬虫抓取商品",
						"scrape competitor", "crawl competitor price", "competitor data collection",
						"scrape product listing", "price intelligence crawl", "采集竞品信息",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-ecom-resp-001", Name: "定价策略泄露检测", Description: "检测响应中泄露定价策略信息",
					Category: "price_manipulation", Direction: "response", Type: "keyword",
					Patterns: []string{
						"定价策略", "价格算法", "动态定价模型", "成本加成率", "利润率公式",
						"pricing strategy", "pricing algorithm", "dynamic pricing model",
						"cost markup rate", "margin formula", "价格决策逻辑",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-ecom-resp-002", Name: "用户购买行为泄露检测(正则)", Description: "正则检测响应中的用户购买行为数据",
					Category: "user_profiling", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(purchase\s+history|购买记录|order\s+history|订单历史)\s*[:：]`,
						`(?i)(user\s+id|用户ID)\s*[:：]\s*\S+.{0,30}(购买|bought|ordered|下单)`,
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
			},
		},
		// ---- 人力资源 ----
		{
			ID:          "tpl-llm-hr",
			Name:        "人力资源 LLM 规则",
			Description: "人力资源行业专属 LLM 规则，覆盖请求侧薪酬查询拦截和响应侧薪酬/绩效数据泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-hr-req-001", Name: "查询薪酬拦截", Description: "检测请求中查询薪酬数据的操作",
					Category: "salary_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询薪酬", "导出工资", "查员工薪资", "薪酬明细",
						"query salary", "export payroll", "employee compensation",
						"salary detail", "wage export", "查薪资报表",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-hr-req-002", Name: "批量导出员工数据拦截", Description: "检测请求中批量导出员工数据的操作",
					Category: "employee_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"批量导出员工", "导出人事数据", "全员信息导出", "员工名册导出",
						"bulk export employee", "export HR data", "export all staff",
						"employee roster export", "personnel data dump", "批量下载员工信息",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-hr-resp-001", Name: "薪酬信息泄露检测", Description: "检测响应中泄露薪酬信息",
					Category: "salary_data", Direction: "response", Type: "keyword",
					Patterns: []string{
						"基本工资", "绩效奖金", "年终奖", "薪资结构", "社保缴纳",
						"base salary", "performance bonus", "annual bonus",
						"compensation structure", "social insurance", "股票期权",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-hr-resp-002", Name: "绩效评语泄露检测(正则)", Description: "正则检测响应中的绩效评语和评级数据",
					Category: "performance_data", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(performance\s+rating|绩效评级|评估等级)\s*[:：]\s*(A|B|C|D|优秀|良好|合格|不合格)`,
						`(?i)(员工|employee)\s*[:：]?\s*\S+.{0,30}(绩效|performance).{0,20}(评语|comment|评价|review)`,
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
			},
		},
		// ---- 保险行业 ----
		{
			ID:          "tpl-llm-insurance",
			Name:        "保险行业 LLM 规则",
			Description: "保险行业专属 LLM 规则，覆盖请求侧理赔/精算数据查询拦截和响应侧保单/精算参数泄露检测",
			Category:    "industry",
			BuiltIn:     true,
			Rules: []LLMRule{
				{
					ID: "tpl-ins-req-001", Name: "查询理赔记录检测", Description: "检测请求中查询理赔记录的操作",
					Category: "claim_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"查询理赔记录", "导出理赔数据", "理赔明细", "出险记录查询",
						"query claim record", "export claim data", "claim detail",
						"incident record query", "loss report query", "查理赔流水",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-ins-req-002", Name: "导出精算数据拦截", Description: "检测请求中导出精算模型或数据的操作",
					Category: "actuarial_data", Direction: "request", Type: "keyword",
					Patterns: []string{
						"导出精算数据", "精算模型参数", "导出费率表", "精算假设导出",
						"export actuarial data", "actuarial model parameter", "export rate table",
						"actuarial assumption export", "risk model export", "查精算模型",
					},
					Action: "block", Enabled: true, Priority: 20,
				},
				{
					ID: "tpl-ins-resp-001", Name: "保单持有人信息泄露检测", Description: "检测响应中泄露保单持有人身份信息",
					Category: "policy_data", Direction: "response", Type: "keyword",
					Patterns: []string{
						"投保人姓名", "被保险人", "保单号", "受益人信息", "保额",
						"policyholder name", "insured person", "policy number",
						"beneficiary information", "sum insured", "保险金额",
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
				{
					ID: "tpl-ins-resp-002", Name: "精算参数泄露检测(正则)", Description: "正则检测响应中的精算参数和费率数据",
					Category: "actuarial_data", Direction: "response", Type: "regex",
					Patterns: []string{
						`(?i)(mortality\s+rate|死亡率|loss\s+ratio|赔付率)\s*[:：]\s*\d+[\.\d]*%`,
						`(?i)(premium\s+rate|费率|actuarial\s+factor|精算因子)\s*[:：]\s*[\d\.]+`,
					},
					Action: "warn", Enabled: true, Priority: 15,
				},
			},
		},
		// ========== 第二批行业 LLM 模板（28个） ==========
		// ---- 证券/投行 ----
		{ID: "tpl-llm-securities", Name: "证券/投行 LLM 规则", Description: "证券/投行行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-securities-req-001", Name: "查询未公开研报拦截", Description: "检测请求中查询未公开研报", Category: "research_data", Direction: "request", Type: "keyword", Patterns: []string{"未公开研报", "研报草稿", "投行项目", "路演材料", "draft research", "unpublished report", "underwriting", "roadshow material"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-securities-req-002", Name: "导出IPO定价拦截", Description: "检测请求中导出IPO定价", Category: "ipo_data", Direction: "request", Type: "keyword", Patterns: []string{"IPO定价", "配售方案", "保荐材料", "IPO pricing", "share allocation"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-securities-resp-001", Name: "研报未公开内容泄露", Description: "检测响应中泄露未公开研报", Category: "research_data", Direction: "response", Type: "keyword", Patterns: []string{"研报草稿", "投行项目", "路演材料", "保荐材料", "draft research", "unpublished report", "roadshow material"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-securities-resp-002", Name: "持仓数据泄露(正则)", Description: "正则检测持仓和交易策略", Category: "position_data", Direction: "response", Type: "regex", Patterns: []string{`持仓.*股|shares.*position`, `(?i)(trading\s+strategy|交易策略|锁定期|lock-up)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 基金/资管 ----
		{ID: "tpl-llm-fund", Name: "基金/资管 LLM 规则", Description: "基金/资管行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-fund-req-001", Name: "查询基金持仓拦截", Description: "检测请求中查询基金持仓", Category: "portfolio_data", Direction: "request", Type: "keyword", Patterns: []string{"基金持仓", "投资组合", "资产配置", "fund holding", "portfolio allocation", "asset allocation"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-fund-req-002", Name: "导出风控参数拦截", Description: "检测请求中导出风控参数", Category: "risk_model", Direction: "request", Type: "keyword", Patterns: []string{"风控模型", "清盘线", "预警线", "风险敞口", "risk model", "liquidation line", "risk exposure"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-fund-resp-001", Name: "投资组合泄露检测", Description: "检测响应中泄露投资组合", Category: "portfolio_data", Direction: "response", Type: "keyword", Patterns: []string{"基金持仓", "资产配置", "回撤数据", "夏普比率", "fund holding", "asset allocation", "drawdown", "Sharpe ratio"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-fund-resp-002", Name: "净值预测泄露(正则)", Description: "正则检测净值预测和alpha模型", Category: "fund_data", Direction: "response", Type: "regex", Patterns: []string{`(?i)NAV|净值.*预测|alpha.*模型`, `(?i)(Sharpe|夏普)\s*(ratio|比率)\s*[:：]\s*[\d\.]+`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 制药/生物科技 ----
		{ID: "tpl-llm-pharma", Name: "制药/生物科技 LLM 规则", Description: "制药/生物科技行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-pharma-req-001", Name: "查询药物分子式拦截", Description: "检测请求中查询药物分子式", Category: "drug_formula", Direction: "request", Type: "keyword", Patterns: []string{"药物配方", "分子式", "合成路线", "原料药工艺", "drug formula", "molecular structure", "synthesis route", "API process"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-pharma-req-002", Name: "导出临床数据拦截", Description: "检测请求中导出临床数据", Category: "clinical_data", Direction: "request", Type: "keyword", Patterns: []string{"临床试验", "受试者数据", "IND申请", "clinical trial", "subject data", "IND filing"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-pharma-resp-001", Name: "药物配方泄露检测", Description: "检测响应中泄露药物配方", Category: "drug_formula", Direction: "response", Type: "keyword", Patterns: []string{"药物配方", "合成路线", "原料药工艺", "drug formula", "synthesis route", "API process"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-pharma-resp-002", Name: "临床试验结果泄露(正则)", Description: "正则检测临床试验数据", Category: "clinical_data", Direction: "response", Type: "regex", Patterns: []string{`Phase [I-IV]|期临床|CRO|受试者`, `(?i)(bioequivalence|生物等效性|GMP\s+record|批生产记录)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 机器人/自动化 ----
		{ID: "tpl-llm-robotics", Name: "机器人/自动化 LLM 规则", Description: "机器人/自动化行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-robotics-req-001", Name: "查询控制算法拦截", Description: "检测请求中查询控制算法", Category: "control_algorithm", Direction: "request", Type: "keyword", Patterns: []string{"运动控制算法", "逆运动学", "轨迹规划", "SLAM算法", "motion control algorithm", "inverse kinematics", "trajectory planning", "SLAM algorithm"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-robotics-req-002", Name: "修改安全区域拦截", Description: "检测请求中修改安全区域", Category: "safety_config", Direction: "request", Type: "keyword", Patterns: []string{"安全区域", "协作区域", "力控参数", "safety zone", "collaborative zone", "force control"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-robotics-resp-001", Name: "控制算法泄露检测", Description: "检测响应中泄露控制算法", Category: "control_algorithm", Direction: "response", Type: "keyword", Patterns: []string{"运动控制算法", "逆运动学", "轨迹规划", "SLAM算法", "motion control algorithm", "inverse kinematics", "trajectory planning"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-robotics-resp-002", Name: "机器人参数泄露(正则)", Description: "正则检测PID参数和力矩数据", Category: "sensor_data", Direction: "response", Type: "regex", Patterns: []string{`PID.*参数|轨迹规划|joint.*torque`, `(?i)(servo\s+parameter|伺服参数|sensor\s+fusion|传感器融合)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 消费电子/家电 ----
		{ID: "tpl-llm-consumer-electronics", Name: "消费电子/家电 LLM 规则", Description: "消费电子/家电行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-consumer-electronics-req-001", Name: "导出BOM清单拦截", Description: "检测请求中导出BOM", Category: "bom_data", Direction: "request", Type: "keyword", Patterns: []string{"产品BOM", "物料清单", "成本结构", "product BOM", "bill of materials", "cost structure"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-consumer-electronics-req-002", Name: "查询供应商报价拦截", Description: "检测请求中查询供应商报价", Category: "supplier_data", Direction: "request", Type: "keyword", Patterns: []string{"供应商报价", "开模费用", "supplier quotation", "tooling cost"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-consumer-electronics-resp-001", Name: "BOM成本泄露检测", Description: "检测响应中泄露BOM成本", Category: "bom_data", Direction: "response", Type: "keyword", Patterns: []string{"产品BOM", "物料清单", "成本结构", "product BOM", "bill of materials", "cost structure"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-consumer-electronics-resp-002", Name: "模具参数泄露(正则)", Description: "正则检测BOM和模具参数", Category: "mold_data", Direction: "response", Type: "regex", Patterns: []string{`BOM.*cost|模具.*参数|供应商.*价格`, `(?i)(mold\s+parameter|tooling\s+cost|certification\s+data)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 重工/装备制造 ----
		{ID: "tpl-llm-heavy-industry", Name: "重工/装备制造 LLM 规则", Description: "重工/装备制造行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-heavy-industry-req-001", Name: "查询设备参数拦截", Description: "检测请求中查询设备参数", Category: "special_equipment", Direction: "request", Type: "keyword", Patterns: []string{"特种设备", "起重机参数", "锅炉参数", "special equipment", "crane parameter", "boiler parameter"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-heavy-industry-req-002", Name: "导出工艺规程拦截", Description: "检测请求中导出工艺规程", Category: "welding_data", Direction: "request", Type: "keyword", Patterns: []string{"焊接工艺", "WPS", "焊接规程", "压力容器", "welding procedure", "pressure vessel"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-heavy-industry-resp-001", Name: "工艺参数泄露检测", Description: "检测响应中泄露工艺参数", Category: "welding_data", Direction: "response", Type: "keyword", Patterns: []string{"焊接工艺", "WPS", "压力容器", "管道设计", "welding procedure", "pressure vessel", "piping design"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-heavy-industry-resp-002", Name: "特种设备泄露(正则)", Description: "正则检测焊接和特种设备数据", Category: "special_equipment", Direction: "response", Type: "regex", Patterns: []string{`焊接.*工艺|WPS|压力.*容器|特种设备`, `(?i)(heat\s+treatment|热处理|NDT|无损检测)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 民航 ----
		{ID: "tpl-llm-civil-aviation", Name: "民航 LLM 规则", Description: "民航行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-civil-aviation-req-001", Name: "查询飞控配置拦截", Description: "检测请求中查询飞控配置", Category: "flight_control", Direction: "request", Type: "keyword", Patterns: []string{"飞控参数", "FMS配置", "适航证", "flight control parameter", "FMS configuration", "airworthiness"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-civil-aviation-req-002", Name: "导出旅客数据拦截", Description: "检测请求中导出旅客数据", Category: "pnr_data", Direction: "request", Type: "keyword", Patterns: []string{"旅客记录", "PNR", "航线收益", "passenger record", "route yield"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-civil-aviation-resp-001", Name: "飞控参数泄露检测", Description: "检测响应中泄露飞控参数", Category: "flight_control", Direction: "response", Type: "keyword", Patterns: []string{"飞控参数", "FMS配置", "航路点", "适航证", "flight control parameter", "FMS configuration", "waypoint"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-civil-aviation-resp-002", Name: "航线运营泄露(正则)", Description: "正则检测FMS/NOTAM/ACARS", Category: "pnr_data", Direction: "response", Type: "regex", Patterns: []string{`FMS|NOTAM|ACARS|MEL`, `(?i)(load\s+factor|客座率|route\s+yield|航线收益)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 铁路/高铁 ----
		{ID: "tpl-llm-railway", Name: "铁路/高铁 LLM 规则", Description: "铁路/高铁行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-railway-req-001", Name: "查询信号配置拦截", Description: "检测请求中查询信号配置", Category: "signal_system", Direction: "request", Type: "keyword", Patterns: []string{"CTCS", "列控系统", "ATP参数", "应答器", "train control system", "ATP parameter", "balise"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-railway-req-002", Name: "修改线路参数拦截", Description: "检测请求中修改线路参数", Category: "dispatch_data", Direction: "request", Type: "keyword", Patterns: []string{"运行图", "调度命令", "线路限速", "timetable", "dispatch command", "speed restriction"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-railway-resp-001", Name: "信号系统泄露检测", Description: "检测响应中泄露信号系统", Category: "signal_system", Direction: "response", Type: "keyword", Patterns: []string{"CTCS", "列控系统", "ATP参数", "应答器", "轨道电路", "train control system", "ATP parameter", "balise", "track circuit"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-railway-resp-002", Name: "运行图泄露(正则)", Description: "正则检测CTCS/运行图数据", Category: "dispatch_data", Direction: "response", Type: "regex", Patterns: []string{`CTCS|列控|ATP|应答器|运行图`, `(?i)(block\s+section|闭塞分区|interlocking|联锁)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 城市轨道/地铁 ----
		{ID: "tpl-llm-metro", Name: "城市轨道/地铁 LLM 规则", Description: "城市轨道/地铁行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-metro-req-001", Name: "修改CBTC配置拦截", Description: "检测请求中修改CBTC配置", Category: "cbtc_data", Direction: "request", Type: "keyword", Patterns: []string{"CBTC", "ATO参数", "列车自动运行", "ATO parameter", "train automation"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-metro-req-002", Name: "查询运营数据拦截", Description: "检测请求中查询运营数据", Category: "passenger_flow", Direction: "request", Type: "keyword", Patterns: []string{"客流预测", "客流调度", "行车间隔", "passenger flow forecast", "headway"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-metro-resp-001", Name: "CBTC数据泄露检测", Description: "检测响应中泄露CBTC", Category: "cbtc_data", Direction: "response", Type: "keyword", Patterns: []string{"CBTC", "ATO参数", "列车自动运行", "ATO parameter", "train automation"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-metro-resp-002", Name: "客流调度泄露(正则)", Description: "正则检测CBTC/客流数据", Category: "passenger_flow", Direction: "response", Type: "regex", Patterns: []string{`CBTC|屏蔽门|ATO|客流.*预测`, `(?i)(headway|行车间隔|turnaround|折返)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 航运/港口 ----
		{ID: "tpl-llm-maritime", Name: "航运/港口 LLM 规则", Description: "航运/港口行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-maritime-req-001", Name: "查询船舶位置拦截", Description: "检测请求中查询船舶位置", Category: "ais_data", Direction: "request", Type: "keyword", Patterns: []string{"AIS数据", "船舶位置", "MMSI", "IMO编号", "AIS data", "vessel position"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-maritime-req-002", Name: "导出港口数据拦截", Description: "检测请求中导出港口数据", Category: "port_data", Direction: "request", Type: "keyword", Patterns: []string{"港口调度", "泊位分配", "集装箱追踪", "port schedule", "berth allocation", "container tracking"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-maritime-resp-001", Name: "船舶数据泄露检测", Description: "检测响应中泄露船舶数据", Category: "ais_data", Direction: "response", Type: "keyword", Patterns: []string{"AIS数据", "船舶位置", "MMSI", "IMO编号", "AIS data", "vessel position"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-maritime-resp-002", Name: "港口调度泄露(正则)", Description: "正则检测AIS/泊位数据", Category: "port_data", Direction: "response", Type: "regex", Patterns: []string{`AIS|MMSI|IMO.*number|泊位.*分配`, `(?i)(bill\s+of\s+lading|提单|customs\s+declaration|报关)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 游戏 ----
		{ID: "tpl-llm-gaming", Name: "游戏行业 LLM 规则", Description: "游戏行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-gaming-req-001", Name: "查询反外挂规则拦截", Description: "检测请求中查询反外挂", Category: "anticheat", Direction: "request", Type: "keyword", Patterns: []string{"反外挂策略", "外挂检测", "游戏源码", "anti-cheat strategy", "cheat detection", "game source code"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-gaming-req-002", Name: "修改内购价格拦截", Description: "检测请求中修改内购价格", Category: "iap_data", Direction: "request", Type: "keyword", Patterns: []string{"内购定价", "充值比例", "掉落概率", "抽卡概率", "in-app purchase pricing", "drop rate", "gacha rate"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-gaming-resp-001", Name: "反外挂策略泄露", Description: "检测响应中泄露反外挂策略", Category: "anticheat", Direction: "response", Type: "keyword", Patterns: []string{"反外挂策略", "外挂检测", "游戏源码", "服务器架构", "anti-cheat strategy", "cheat detection", "game source code", "server architecture"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-gaming-resp-002", Name: "游戏经济泄露(正则)", Description: "正则检测外挂和充值数据", Category: "iap_data", Direction: "response", Type: "regex", Patterns: []string{`外挂.*检测|anti-cheat|充值.*比例|drop.*rate`, `(?i)(gacha\s+rate|抽卡概率|virtual\s+item|虚拟道具)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 广告/营销 ----
		{ID: "tpl-llm-advertising", Name: "广告/营销 LLM 规则", Description: "广告/营销行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-advertising-req-001", Name: "导出用户画像拦截", Description: "检测请求中导出用户画像", Category: "user_tag", Direction: "request", Type: "keyword", Patterns: []string{"用户标签", "人群包", "DMP数据", "user tag", "audience segment", "DMP data"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-advertising-req-002", Name: "查询竞品投放拦截", Description: "检测请求中查询竞品投放", Category: "competitor_data", Direction: "request", Type: "keyword", Patterns: []string{"竞品监控", "ROI数据", "转化漏斗", "competitor monitoring", "ROI data", "conversion funnel"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-advertising-resp-001", Name: "投放策略泄露检测", Description: "检测响应中泄露投放策略", Category: "ad_strategy", Direction: "response", Type: "keyword", Patterns: []string{"投放策略", "出价策略", "千次展示成本", "media plan", "bidding strategy", "CPM"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-advertising-resp-002", Name: "DMP数据泄露(正则)", Description: "正则检测DMP/ROAS数据", Category: "user_tag", Direction: "response", Type: "regex", Patterns: []string{`DMP|人群包|ROAS|CPA.*bid|conversion.*rate`, `(?i)(audience\s+segment|creative\s+library|广告素材)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 社交平台 ----
		{ID: "tpl-llm-social-media", Name: "社交平台 LLM 规则", Description: "社交平台行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-social-media-req-001", Name: "导出用户关系拦截", Description: "检测请求中导出用户关系链", Category: "social_graph", Direction: "request", Type: "keyword", Patterns: []string{"用户关系链", "好友列表", "社交图谱", "social graph", "friend list"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-social-media-req-002", Name: "查询推荐策略拦截", Description: "检测请求中查询推荐策略", Category: "rec_algorithm", Direction: "request", Type: "keyword", Patterns: []string{"推荐算法", "内容审核策略", "信息流排序", "recommendation algorithm", "content moderation policy", "feed ranking"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-social-media-resp-001", Name: "关系链泄露检测", Description: "检测响应中泄露关系链", Category: "social_graph", Direction: "response", Type: "keyword", Patterns: []string{"用户关系链", "好友列表", "社交图谱", "用户行为日志", "social graph", "friend list", "user behavior log"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-social-media-resp-002", Name: "推荐算法泄露(正则)", Description: "正则检测推荐算法数据", Category: "rec_algorithm", Direction: "response", Type: "regex", Patterns: []string{`推荐.*算法|social.*graph|关系链|feed.*rank`, `(?i)(content\s+moderation|内容审核|report\s+data|举报数据)`}, Action: "block", Enabled: true, Priority: 18},
		}},
		// ---- 短视频/直播 ----
		{ID: "tpl-llm-live-streaming", Name: "短视频/直播 LLM 规则", Description: "短视频/直播行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-live-streaming-req-001", Name: "查询主播收入拦截", Description: "检测请求中查询主播收入", Category: "streamer_revenue", Direction: "request", Type: "keyword", Patterns: []string{"主播收入", "打赏分成", "礼物分成比例", "streamer revenue", "gift sharing ratio"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-live-streaming-req-002", Name: "导出分发规则拦截", Description: "检测请求中导出分发规则", Category: "traffic_rule", Direction: "request", Type: "keyword", Patterns: []string{"流量分发规则", "直播间权重", "推流地址", "直播源码", "traffic distribution rule", "live room weight", "streaming address", "source code"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-live-streaming-resp-001", Name: "主播收入泄露检测", Description: "检测响应中泄露主播收入", Category: "streamer_revenue", Direction: "response", Type: "keyword", Patterns: []string{"主播收入", "打赏分成", "礼物分成比例", "streamer revenue", "gift sharing ratio"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-live-streaming-resp-002", Name: "流量规则泄露(正则)", Description: "正则检测流量分发和MCN数据", Category: "traffic_rule", Direction: "response", Type: "regex", Patterns: []string{`流量.*分发|主播.*分成|MCN.*合约|打赏.*比例`, `(?i)(commission\s+rate|带货佣金|moderation\s+policy|审核策略)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- SaaS/云服务 ----
		{ID: "tpl-llm-saas-cloud", Name: "SaaS/云服务 LLM 规则", Description: "SaaS/云服务行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-saas-cloud-req-001", Name: "查询客户数据拦截", Description: "检测请求中查询客户数据", Category: "customer_data", Direction: "request", Type: "keyword", Patterns: []string{"客户数据隔离", "客户续约率", "客户流失率", "customer data isolation", "customer retention", "churn rate"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-saas-cloud-req-002", Name: "导出租户配置拦截", Description: "检测请求中导出租户配置", Category: "tenant_config", Direction: "request", Type: "keyword", Patterns: []string{"多租户配置", "部署架构", "multi-tenant config", "deployment architecture"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-saas-cloud-resp-001", Name: "客户数据泄露检测", Description: "检测响应中泄露客户数据", Category: "customer_data", Direction: "response", Type: "keyword", Patterns: []string{"客户数据隔离", "客户续约率", "客户流失率", "customer data isolation", "customer retention", "churn rate"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-saas-cloud-resp-002", Name: "云配置泄露(正则)", Description: "正则检测租户配置和SLA数据", Category: "tenant_config", Direction: "response", Type: "regex", Patterns: []string{`tenant.*config|SLA.*breach|客户.*数据|access.*key`, `(?i)(ARR|MRR|deployment\s+architecture|部署架构)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 搜索引擎 ----
		{ID: "tpl-llm-search-engine", Name: "搜索引擎 LLM 规则", Description: "搜索引擎行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-search-engine-req-001", Name: "查询排名规则拦截", Description: "检测请求中查询排名规则", Category: "ranking_algo", Direction: "request", Type: "keyword", Patterns: []string{"搜索排名算法", "排名因子", "索引策略", "ranking algorithm", "ranking factor", "indexing strategy"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-search-engine-req-002", Name: "导出搜索日志拦截", Description: "检测请求中导出搜索日志", Category: "search_log", Direction: "request", Type: "keyword", Patterns: []string{"搜索日志", "用户搜索词", "搜索意图", "search log", "search query", "search intent"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-search-engine-resp-001", Name: "排名算法泄露检测", Description: "检测响应中泄露排名算法", Category: "ranking_algo", Direction: "response", Type: "keyword", Patterns: []string{"搜索排名算法", "排名因子", "索引策略", "ranking algorithm", "ranking factor", "indexing strategy"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-search-engine-resp-002", Name: "搜索数据泄露(正则)", Description: "正则检测排名和爬虫数据", Category: "search_log", Direction: "response", Type: "regex", Patterns: []string{`排名.*算法|PageRank|索引.*策略|crawl.*policy`, `(?i)(quality\s+score|质量得分|search\s+intent|搜索意图)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 外卖/本地生活 ----
		{ID: "tpl-llm-local-services", Name: "外卖/本地生活 LLM 规则", Description: "外卖/本地生活行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-local-services-req-001", Name: "导出骑手数据拦截", Description: "检测请求中导出骑手数据", Category: "rider_data", Direction: "request", Type: "keyword", Patterns: []string{"骑手轨迹", "配送路径", "运力调度", "rider trajectory", "delivery route", "capacity scheduling"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-local-services-req-002", Name: "查询评分算法拦截", Description: "检测请求中查询评分算法", Category: "merchant_algo", Direction: "request", Type: "keyword", Patterns: []string{"商户评分算法", "佣金比例", "抽成比例", "merchant scoring algorithm", "commission rate"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-local-services-resp-001", Name: "商户数据泄露检测", Description: "检测响应中泄露商户数据", Category: "merchant_algo", Direction: "response", Type: "keyword", Patterns: []string{"商户评分算法", "佣金比例", "商户流水", "merchant scoring algorithm", "commission rate", "merchant revenue"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-local-services-resp-002", Name: "配送数据泄露(正则)", Description: "正则检测骑手和佣金数据", Category: "rider_data", Direction: "response", Type: "regex", Patterns: []string{`骑手.*轨迹|商户.*评分|配送.*路径|佣金.*比例`, `(?i)(surge\s+pricing|高峰定价|promotion\s+strategy|满减策略)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 网络安全 ----
		{ID: "tpl-llm-cybersecurity", Name: "网络安全 LLM 规则", Description: "网络安全行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-cybersecurity-req-001", Name: "查询未公开漏洞拦截", Description: "检测请求中查询未公开漏洞", Category: "vulnerability", Direction: "request", Type: "keyword", Patterns: []string{"0day漏洞", "未公开漏洞", "漏洞利用代码", "PoC代码", "zero-day", "undisclosed vulnerability", "exploit code", "PoC code"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-cybersecurity-req-002", Name: "导出渗透报告拦截", Description: "检测请求中导出渗透报告", Category: "security_report", Direction: "request", Type: "keyword", Patterns: []string{"渗透报告", "客户资产", "安全审计报告", "penetration report", "client asset", "security audit report"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-cybersecurity-resp-001", Name: "漏洞详情泄露检测", Description: "检测响应中泄露漏洞详情", Category: "vulnerability", Direction: "response", Type: "keyword", Patterns: []string{"0day漏洞", "未公开漏洞", "漏洞利用代码", "PoC代码", "zero-day", "undisclosed vulnerability", "exploit code", "PoC code"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-cybersecurity-resp-002", Name: "客户资产泄露(正则)", Description: "正则检测CVE和渗透报告", Category: "security_report", Direction: "response", Type: "regex", Patterns: []string{`CVE-\d{4}-\d+|0day|exploit.*code|PoC|渗透.*报告`, `(?i)(incident\s+response|应急响应|attack\s+toolchain|攻击工具链)`}, Action: "block", Enabled: true, Priority: 18},
		}},
		// ---- 传媒/新闻 ----
		{ID: "tpl-llm-media-news", Name: "传媒/新闻 LLM 规则", Description: "传媒/新闻行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-media-news-req-001", Name: "查询未发布内容拦截", Description: "检测请求中查询未发布内容", Category: "unpublished", Direction: "request", Type: "keyword", Patterns: []string{"未发布稿件", "发稿计划", "新闻素材", "unpublished article", "publication schedule", "news material"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-media-news-req-002", Name: "暴露信息源拦截", Description: "检测请求中暴露信息源", Category: "news_source", Direction: "request", Type: "keyword", Patterns: []string{"信息源", "匿名线人", "anonymous source", "confidential informant"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-media-news-resp-001", Name: "未发布内容泄露检测", Description: "检测响应中泄露未发布内容", Category: "unpublished", Direction: "response", Type: "keyword", Patterns: []string{"未发布稿件", "发稿计划", "新闻素材", "unpublished article", "publication schedule", "news material"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-media-news-resp-002", Name: "信息源泄露(正则)", Description: "正则检测信息源和独家新闻", Category: "news_source", Direction: "response", Type: "regex", Patterns: []string{`信息源|anonymous.*source|独家|exclusive.*story`, `(?i)(editorial\s+strategy|采编策略|public\s+opinion|舆论引导)`}, Action: "block", Enabled: true, Priority: 18},
		}},
		// ---- 出版/版权 ----
		{ID: "tpl-llm-publishing", Name: "出版/版权 LLM 规则", Description: "出版/版权行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-publishing-req-001", Name: "导出手稿内容拦截", Description: "检测请求中导出手稿", Category: "manuscript", Direction: "request", Type: "keyword", Patterns: []string{"未出版手稿", "unpublished manuscript"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-publishing-req-002", Name: "查询版税明细拦截", Description: "检测请求中查询版税", Category: "royalty_data", Direction: "request", Type: "keyword", Patterns: []string{"版税数据", "稿费标准", "royalty data", "author fee"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-publishing-resp-001", Name: "手稿内容泄露检测", Description: "检测响应中泄露手稿", Category: "manuscript", Direction: "response", Type: "keyword", Patterns: []string{"未出版手稿", "DRM配置", "数字版权", "unpublished manuscript", "DRM configuration", "digital rights"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-publishing-resp-002", Name: "版税数据泄露(正则)", Description: "正则检测版税和ISBN数据", Category: "royalty_data", Direction: "response", Type: "regex", Patterns: []string{`版税|royalty.*rate|ISBN.*\d{13}|稿费`, `(?i)(print\s+run|印数|distribution\s+channel|发行渠道)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 电信/运营商 ----
		{ID: "tpl-llm-telecom", Name: "电信/运营商 LLM 规则", Description: "电信/运营商行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-telecom-req-001", Name: "查询通话记录拦截", Description: "检测请求中查询通话记录", Category: "cdr_data", Direction: "request", Type: "keyword", Patterns: []string{"通话记录", "CDR数据", "信令数据", "call detail record", "CDR data", "signaling data"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-telecom-req-002", Name: "导出基站数据拦截", Description: "检测请求中导出基站数据", Category: "network_data", Direction: "request", Type: "keyword", Patterns: []string{"基站位置", "核心网配置", "监听接口", "base station location", "core network config", "lawful interception"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-telecom-resp-001", Name: "CDR数据泄露检测", Description: "检测响应中泄露CDR", Category: "cdr_data", Direction: "response", Type: "keyword", Patterns: []string{"通话记录", "CDR数据", "信令数据", "call detail record", "CDR data", "signaling data"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-telecom-resp-002", Name: "网络拓扑泄露(正则)", Description: "正则检测CDR/基站/IMSI", Category: "network_data", Direction: "response", Type: "regex", Patterns: []string{`CDR|基站.*位置|IMSI|IMEI|核心网`, `(?i)(lawful\s+interception|监听|DPI\s+data|subscriber\s+plan)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 物流/供应链 ----
		{ID: "tpl-llm-logistics", Name: "物流/供应链 LLM 规则", Description: "物流/供应链行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-logistics-req-001", Name: "导出客户地址拦截", Description: "检测请求中导出客户地址", Category: "address_data", Direction: "request", Type: "keyword", Patterns: []string{"客户收货地址", "customer shipping address"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-logistics-req-002", Name: "查询供应商报价拦截", Description: "检测请求中查询供应商报价", Category: "supplier_data", Direction: "request", Type: "keyword", Patterns: []string{"供应商报价", "物流路由", "运费协议", "supplier quotation", "logistics route", "freight agreement"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-logistics-resp-001", Name: "物流路由泄露检测", Description: "检测响应中泄露物流路由", Category: "supplier_data", Direction: "response", Type: "keyword", Patterns: []string{"供应商报价", "物流路由", "运费协议", "供应链金融", "supplier quotation", "logistics route", "freight agreement", "supply chain finance"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-logistics-resp-002", Name: "供应链泄露(正则)", Description: "正则检测仓储和供应链数据", Category: "address_data", Direction: "response", Type: "regex", Patterns: []string{`仓储.*布局|供应商.*报价|物流.*路由|库存.*数据`, `(?i)(warehouse\s+layout|storage\s+planning|inventory\s+data)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 房地产/物业 ----
		{ID: "tpl-llm-real-estate", Name: "房地产/物业 LLM 规则", Description: "房地产/物业行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-real-estate-req-001", Name: "导出业主数据拦截", Description: "检测请求中导出业主数据", Category: "owner_data", Direction: "request", Type: "keyword", Patterns: []string{"业主信息", "业主名单", "owner information", "owner list"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-real-estate-req-002", Name: "查询成交底价拦截", Description: "检测请求中查询成交底价", Category: "price_data", Direction: "request", Type: "keyword", Patterns: []string{"成交底价", "楼盘均价", "transaction floor price", "average price"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-real-estate-resp-001", Name: "业主信息泄露检测", Description: "检测响应中泄露业主信息", Category: "owner_data", Direction: "response", Type: "keyword", Patterns: []string{"业主信息", "业主名单", "物业费", "按揭数据", "owner information", "owner list", "property fee", "mortgage data"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-real-estate-resp-002", Name: "交易数据泄露(正则)", Description: "正则检测业主和房价数据", Category: "price_data", Direction: "response", Type: "regex", Patterns: []string{`业主.*信息|成交.*底价|楼盘.*均价|物业.*费`, `(?i)(mortgage\s+data|按揭|purchase\s+contract|购房合同)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 农业/食品 ----
		{ID: "tpl-llm-agriculture", Name: "农业/食品 LLM 规则", Description: "农业/食品行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-agriculture-req-001", Name: "查询种子配方拦截", Description: "检测请求中查询种子配方", Category: "seed_data", Direction: "request", Type: "keyword", Patterns: []string{"种子专利", "转基因数据", "育种记录", "seed patent", "GMO data", "breeding record"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-agriculture-req-002", Name: "导出溯源数据拦截", Description: "检测请求中导出溯源数据", Category: "trace_data", Direction: "request", Type: "keyword", Patterns: []string{"溯源数据", "产地证明", "有机认证", "traceability data", "origin certificate", "organic certification"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-agriculture-resp-001", Name: "种子专利泄露检测", Description: "检测响应中泄露种子专利", Category: "seed_data", Direction: "response", Type: "keyword", Patterns: []string{"种子专利", "转基因数据", "育种记录", "农药配方", "seed patent", "GMO data", "breeding record", "pesticide formula"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-agriculture-resp-002", Name: "食品检测泄露(正则)", Description: "正则检测种子和农药数据", Category: "trace_data", Direction: "response", Type: "regex", Patterns: []string{`种子.*专利|转基因|农药.*配方|食品.*检测`, `(?i)(pesticide\s+residue|农药残留|feed\s+formula|饲料配方)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 航空航天 ----
		{ID: "tpl-llm-aerospace", Name: "航空航天 LLM 规则", Description: "航空航天行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-aerospace-req-001", Name: "查询卫星轨道拦截", Description: "检测请求中查询卫星轨道", Category: "satellite_data", Direction: "request", Type: "keyword", Patterns: []string{"卫星参数", "轨道数据", "TLE", "遥测数据", "satellite parameter", "orbital data", "TLE", "telemetry data"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-aerospace-req-002", Name: "导出飞控代码拦截", Description: "检测请求中导出飞控代码", Category: "rocket_data", Direction: "request", Type: "keyword", Patterns: []string{"ITAR管制", "星载软件", "火箭参数", "ITAR controlled", "onboard software", "rocket parameter"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-aerospace-resp-001", Name: "卫星参数泄露检测", Description: "检测响应中泄露卫星参数", Category: "satellite_data", Direction: "response", Type: "keyword", Patterns: []string{"卫星参数", "轨道数据", "遥测数据", "遥控指令", "satellite parameter", "orbital data", "telemetry data", "telecommand"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-aerospace-resp-002", Name: "航天数据泄露(正则)", Description: "正则检测ITAR/TLE/遥测数据", Category: "rocket_data", Direction: "response", Type: "regex", Patterns: []string{`ITAR|TLE|轨道.*参数|遥测|星载`, `(?i)(launch\s+window|发射窗口|tracking\s+frequency|测控频段)`}, Action: "block", Enabled: true, Priority: 18},
		}},
		// ---- 矿业/资源 ----
		{ID: "tpl-llm-mining", Name: "矿业/资源 LLM 规则", Description: "矿业/资源行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-mining-req-001", Name: "查询矿藏数据拦截", Description: "检测请求中查询矿藏数据", Category: "exploration_data", Direction: "request", Type: "keyword", Patterns: []string{"勘探数据", "矿藏储量", "品位数据", "exploration data", "mineral reserves", "ore grade"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-mining-req-002", Name: "导出勘探报告拦截", Description: "检测请求中导出勘探报告", Category: "mining_rights", Direction: "request", Type: "keyword", Patterns: []string{"采矿权", "探矿权", "地质报告", "mining rights", "prospecting rights", "geological report"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-mining-resp-001", Name: "勘探数据泄露检测", Description: "检测响应中泄露勘探数据", Category: "exploration_data", Direction: "response", Type: "keyword", Patterns: []string{"勘探数据", "矿藏储量", "品位数据", "exploration data", "mineral reserves", "ore grade"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-mining-resp-002", Name: "储量数据泄露(正则)", Description: "正则检测勘探和品位数据", Category: "mining_rights", Direction: "response", Type: "regex", Patterns: []string{`勘探.*数据|矿藏.*储量|品位|exploration.*data`, `(?i)(tailings|尾矿|beneficiation|选矿|ore\s+composition|矿石成分)`}, Action: "block", Enabled: true, Priority: 18},
		}},
		// ---- 建筑/工程 ----
		{ID: "tpl-llm-construction", Name: "建筑/工程 LLM 规则", Description: "建筑/工程行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-construction-req-001", Name: "查询招标底价拦截", Description: "检测请求中查询招标底价", Category: "bid_data", Direction: "request", Type: "keyword", Patterns: []string{"招标底价", "投标报价", "工程造价", "bid floor price", "tender price", "project cost"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-construction-req-002", Name: "导出设计图纸拦截", Description: "检测请求中导出设计图纸", Category: "design_data", Direction: "request", Type: "keyword", Patterns: []string{"设计图纸", "结构计算书", "施工方案", "design drawing", "structural calculation", "construction plan"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-construction-resp-001", Name: "设计数据泄露检测", Description: "检测响应中泄露设计数据", Category: "design_data", Direction: "response", Type: "keyword", Patterns: []string{"设计图纸", "结构计算书", "施工方案", "design drawing", "structural calculation", "construction plan"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-construction-resp-002", Name: "投标数据泄露(正则)", Description: "正则检测招标和BIM数据", Category: "bid_data", Direction: "response", Type: "regex", Patterns: []string{`招标.*底价|投标.*报价|工程.*造价|BIM`, `(?i)(change\s+order|变更单|final\s+account|结算审计)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
		// ---- 酒店/旅游 ----
		{ID: "tpl-llm-hospitality", Name: "酒店/旅游 LLM 规则", Description: "酒店/旅游行业专属 LLM 规则", Category: "industry", BuiltIn: true, Rules: []LLMRule{
			{ID: "tpl-hospitality-req-001", Name: "查询旅客信息拦截", Description: "检测请求中查询旅客信息", Category: "guest_data", Direction: "request", Type: "keyword", Patterns: []string{"旅客信息", "VIP客户", "客户偏好", "会员数据", "guest information", "VIP customer", "customer preference", "loyalty data"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-hospitality-req-002", Name: "导出定价策略拦截", Description: "检测请求中导出定价策略", Category: "pricing_data", Direction: "request", Type: "keyword", Patterns: []string{"定价策略", "收益管理", "房价策略", "pricing strategy", "revenue management", "rate strategy"}, Action: "block", Enabled: true, Priority: 20},
			{ID: "tpl-hospitality-resp-001", Name: "旅客信息泄露检测", Description: "检测响应中泄露旅客信息", Category: "guest_data", Direction: "response", Type: "keyword", Patterns: []string{"旅客信息", "VIP客户", "客户偏好", "会员数据", "guest information", "VIP customer", "customer preference", "loyalty data"}, Action: "warn", Enabled: true, Priority: 15},
			{ID: "tpl-hospitality-resp-002", Name: "收益数据泄露(正则)", Description: "正则检测RevPAR和入住率数据", Category: "pricing_data", Direction: "response", Type: "regex", Patterns: []string{`RevPAR|入住率|occupancy.*rate|ADR`, `(?i)(revenue\s+management|收益管理|rate\s+strategy|房价策略)`}, Action: "warn", Enabled: true, Priority: 15},
		}},
	}
}

// SetTenantDB 设置租户 LLM 规则持久化 DB 并加载已保存的租户规则
func (e *LLMRuleEngine) SetTenantDB(db *sql.DB) {
	if db == nil {
		return
	}
	e.mu.Lock()
	e.tenantDB = db
	e.mu.Unlock()

	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_llm_rules (
		tenant_id TEXT PRIMARY KEY,
		rules_json TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)

	// 从 DB 重建内存
	e.loadTenantLLMRulesFromDB()
}

// loadTenantLLMRulesFromDB 启动时从 DB 重建租户 LLM 规则
func (e *LLMRuleEngine) loadTenantLLMRulesFromDB() {
	e.mu.RLock()
	db := e.tenantDB
	e.mu.RUnlock()
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT tenant_id, rules_json FROM tenant_llm_rules`)
	if err != nil {
		log.Printf("[LLM规则] 加载持久化租户规则失败: %v", err)
		return
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var tid, rulesJSON string
		if rows.Scan(&tid, &rulesJSON) != nil {
			continue
		}
		var rules []LLMRule
		if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
			continue
		}
		if len(rules) == 0 {
			continue
		}
		// 编译规则（不走 SetTenantLLMRules 避免重复写 DB）
		reqAC, reqACRules, respAC, respACRules, reqRegex, respRegex := compileLLMRulesForTenant(rules)
		e.mu.Lock()
		e.initTenantMaps()
		e.tenantRules[tid] = rules
		e.tenantReqAC[tid] = reqAC
		e.tenantRespAC[tid] = respAC
		e.tenantReqACRules[tid] = reqACRules
		e.tenantRespACRules[tid] = respACRules
		e.tenantReqRegex[tid] = reqRegex
		e.tenantRespRegex[tid] = respRegex
		e.mu.Unlock()
		count++
	}
	if count > 0 {
		log.Printf("[LLM规则] ✅ 从 DB 恢复了 %d 个租户的 LLM 规则", count)
	}
}

// initTenantMaps 初始化租户 map（需持有写锁）
func (e *LLMRuleEngine) initTenantMaps() {
	if e.tenantRules == nil {
		e.tenantRules = make(map[string][]LLMRule)
		e.tenantReqAC = make(map[string]*AhoCorasick)
		e.tenantRespAC = make(map[string]*AhoCorasick)
		e.tenantReqACRules = make(map[string][]llmACEntry)
		e.tenantRespACRules = make(map[string][]llmACEntry)
		e.tenantReqRegex = make(map[string][]*compiledLLMRegexRule)
		e.tenantRespRegex = make(map[string][]*compiledLLMRegexRule)
	}
}

// compileLLMRulesForTenant 编译 LLM 规则为 AC 自动机 + 正则（不修改引擎状态）
func compileLLMRulesForTenant(rules []LLMRule) (
	reqAC *AhoCorasick, reqACRules []llmACEntry,
	respAC *AhoCorasick, respACRules []llmACEntry,
	reqRegex []*compiledLLMRegexRule, respRegex []*compiledLLMRegexRule,
) {
	var reqPatterns, respPatterns []string

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		entry := llmACEntry{
			ruleID:     rule.ID,
			ruleName:   rule.Name,
			category:   rule.Category,
			action:     rule.Action,
			rewriteTo:  rule.RewriteTo,
			priority:   rule.Priority,
			shadowMode: rule.ShadowMode,
		}
		isReq := rule.Direction == "request" || rule.Direction == "both"
		isResp := rule.Direction == "response" || rule.Direction == "both"

		if rule.Type == "regex" {
			for _, p := range rule.Patterns {
				compiled, err := regexp.Compile(p)
				if err != nil {
					continue
				}
				cr := &compiledLLMRegexRule{
					ruleID: rule.ID, ruleName: rule.Name, category: rule.Category,
					action: rule.Action, rewriteTo: rule.RewriteTo, pattern: compiled,
					rawPattern: p, priority: rule.Priority, shadowMode: rule.ShadowMode,
				}
				if isReq {
					reqRegex = append(reqRegex, cr)
				}
				if isResp {
					respRegex = append(respRegex, cr)
				}
			}
		} else {
			for _, p := range rule.Patterns {
				e := entry
				e.pattern = p
				if isReq {
					reqPatterns = append(reqPatterns, p)
					reqACRules = append(reqACRules, e)
				}
				if isResp {
					respPatterns = append(respPatterns, p)
					respACRules = append(respACRules, e)
				}
			}
		}
	}
	reqAC = NewAhoCorasick(reqPatterns)
	respAC = NewAhoCorasick(respPatterns)
	return
}

// SetTenantLLMRules 设置租户专属 LLM 规则并编译 AC + 持久化
func (e *LLMRuleEngine) SetTenantLLMRules(tenantID string, rules []LLMRule) {
	reqAC, reqACRules, respAC, respACRules, reqRegex, respRegex := compileLLMRulesForTenant(rules)
	e.mu.Lock()
	e.initTenantMaps()
	e.tenantRules[tenantID] = rules
	e.tenantReqAC[tenantID] = reqAC
	e.tenantRespAC[tenantID] = respAC
	e.tenantReqACRules[tenantID] = reqACRules
	e.tenantRespACRules[tenantID] = respACRules
	e.tenantReqRegex[tenantID] = reqRegex
	e.tenantRespRegex[tenantID] = respRegex
	db := e.tenantDB
	e.mu.Unlock()

	// 持久化
	e.persistTenantLLMRules(db, tenantID, rules)
	log.Printf("[LLM规则] 设置租户 %s 专属LLM规则: %d 条", tenantID, len(rules))
}

// RemoveTenantLLMRules 移除租户专属 LLM 规则 + 持久化删除
func (e *LLMRuleEngine) RemoveTenantLLMRules(tenantID string) {
	e.mu.Lock()
	delete(e.tenantRules, tenantID)
	delete(e.tenantReqAC, tenantID)
	delete(e.tenantRespAC, tenantID)
	delete(e.tenantReqACRules, tenantID)
	delete(e.tenantRespACRules, tenantID)
	delete(e.tenantReqRegex, tenantID)
	delete(e.tenantRespRegex, tenantID)
	db := e.tenantDB
	e.mu.Unlock()

	if db != nil {
		db.Exec(`DELETE FROM tenant_llm_rules WHERE tenant_id=?`, tenantID)
	}
	log.Printf("[LLM规则] 移除租户 %s 专属LLM规则", tenantID)
}

// GetTenantLLMRules 获取租户专属 LLM 规则
func (e *LLMRuleEngine) GetTenantLLMRules(tenantID string) []LLMRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := e.tenantRules[tenantID]
	if rules == nil {
		return nil
	}
	cp := make([]LLMRule, len(rules))
	copy(cp, rules)
	return cp
}

// persistTenantLLMRules 持久化租户 LLM 规则到 DB
func (e *LLMRuleEngine) persistTenantLLMRules(db *sql.DB, tenantID string, rules []LLMRule) {
	if db == nil {
		return
	}
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		log.Printf("[LLM规则] 序列化失败: %v", err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT OR REPLACE INTO tenant_llm_rules (tenant_id, rules_json, updated_at) VALUES (?,?,?)`,
		tenantID, string(rulesJSON), now)
}

// CheckRequestWithTenant 全局 + 租户 LLM 规则合并检测（请求方向）
func (e *LLMRuleEngine) CheckRequestWithTenant(content, tenantID string) []LLMRuleMatch {
	// 全局规则检测
	matches := e.CheckRequest(content)

	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m.RuleID] = true
	}

	// v30.0: 全局启用的行业模板规则检测
	e.mu.RLock()
	gTplReqAC := e.globalTplReqAC
	gTplReqACRules := e.globalTplReqACRules
	gTplReqRegex := e.globalTplReqRegex
	e.mu.RUnlock()
	if gTplReqAC != nil {
		for _, idx := range gTplReqAC.Search(content) {
			if idx < 0 || idx >= len(gTplReqACRules) {
				continue
			}
			entry := gTplReqACRules[idx]
			if seen[entry.ruleID] {
				continue
			}
			seen[entry.ruleID] = true
			matches = append(matches, LLMRuleMatch{
				RuleID: entry.ruleID, RuleName: entry.ruleName, Category: entry.category,
				Action: entry.action, Pattern: entry.pattern, MatchedText: entry.pattern,
				ShadowMode: entry.shadowMode, Priority: entry.priority, RewriteTo: entry.rewriteTo,
			})
		}
	}
	for _, cr := range gTplReqRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID: cr.ruleID, RuleName: cr.ruleName, Category: cr.category,
			Action: cr.action, Pattern: cr.rawPattern, MatchedText: matchedText,
			ShadowMode: cr.shadowMode, Priority: cr.priority, RewriteTo: cr.rewriteTo,
		})
	}

	if tenantID == "" {
		return matches
	}

	// 租户规则检测
	e.mu.RLock()
	reqAC := e.tenantReqAC[tenantID]
	reqACRules := e.tenantReqACRules[tenantID]
	reqRegex := e.tenantReqRegex[tenantID]
	e.mu.RUnlock()

	if reqAC == nil && len(reqRegex) == 0 {
		return matches
	}

	// AC 自动机匹配
	if reqAC != nil {
		for _, idx := range reqAC.Search(content) {
			if idx < 0 || idx >= len(reqACRules) {
				continue
			}
			entry := reqACRules[idx]
			if seen[entry.ruleID] {
				continue
			}
			seen[entry.ruleID] = true
			matches = append(matches, LLMRuleMatch{
				RuleID: entry.ruleID, RuleName: entry.ruleName, Category: entry.category,
				Action: entry.action, Pattern: entry.pattern, MatchedText: entry.pattern,
				ShadowMode: entry.shadowMode, Priority: entry.priority, RewriteTo: entry.rewriteTo,
			})
		}
	}

	// 正则匹配
	for _, cr := range reqRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID: cr.ruleID, RuleName: cr.ruleName, Category: cr.category,
			Action: cr.action, Pattern: cr.rawPattern, MatchedText: matchedText,
			ShadowMode: cr.shadowMode, Priority: cr.priority, RewriteTo: cr.rewriteTo,
		})
	}

	return matches
}

// CheckResponseWithTenant 全局 + 全局模板 + 租户 LLM 规则合并检测（响应方向）
func (e *LLMRuleEngine) CheckResponseWithTenant(content, tenantID string) []LLMRuleMatch {
	// 全局规则检测
	matches := e.CheckResponse(content)

	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m.RuleID] = true
	}

	// v30.0: 全局启用的行业模板规则检测
	e.mu.RLock()
	gTplRespAC := e.globalTplRespAC
	gTplRespACRules := e.globalTplRespACRules
	gTplRespRegex := e.globalTplRespRegex
	e.mu.RUnlock()
	if gTplRespAC != nil {
		for _, idx := range gTplRespAC.Search(content) {
			if idx < 0 || idx >= len(gTplRespACRules) {
				continue
			}
			entry := gTplRespACRules[idx]
			if seen[entry.ruleID] {
				continue
			}
			seen[entry.ruleID] = true
			matches = append(matches, LLMRuleMatch{
				RuleID: entry.ruleID, RuleName: entry.ruleName, Category: entry.category,
				Action: entry.action, Pattern: entry.pattern, MatchedText: entry.pattern,
				ShadowMode: entry.shadowMode, Priority: entry.priority, RewriteTo: entry.rewriteTo,
			})
		}
	}
	for _, cr := range gTplRespRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID: cr.ruleID, RuleName: cr.ruleName, Category: cr.category,
			Action: cr.action, Pattern: cr.rawPattern, MatchedText: matchedText,
			ShadowMode: cr.shadowMode, Priority: cr.priority, RewriteTo: cr.rewriteTo,
		})
	}

	if tenantID == "" {
		return matches
	}

	// 租户规则检测
	e.mu.RLock()
	respAC := e.tenantRespAC[tenantID]
	respACRules := e.tenantRespACRules[tenantID]
	respRegex := e.tenantRespRegex[tenantID]
	e.mu.RUnlock()

	if respAC == nil && len(respRegex) == 0 {
		return matches
	}

	// AC 自动机匹配
	if respAC != nil {
		for _, idx := range respAC.Search(content) {
			if idx < 0 || idx >= len(respACRules) {
				continue
			}
			entry := respACRules[idx]
			if seen[entry.ruleID] {
				continue
			}
			seen[entry.ruleID] = true
			matches = append(matches, LLMRuleMatch{
				RuleID: entry.ruleID, RuleName: entry.ruleName, Category: entry.category,
				Action: entry.action, Pattern: entry.pattern, MatchedText: entry.pattern,
				ShadowMode: entry.shadowMode, Priority: entry.priority, RewriteTo: entry.rewriteTo,
			})
		}
	}

	// 正则匹配
	for _, cr := range respRegex {
		if seen[cr.ruleID] {
			continue
		}
		loc := cr.pattern.FindStringIndex(content)
		if loc == nil {
			continue
		}
		seen[cr.ruleID] = true
		matchedText := content[loc[0]:loc[1]]
		if len(matchedText) > 100 {
			matchedText = matchedText[:100] + "..."
		}
		matches = append(matches, LLMRuleMatch{
			RuleID: cr.ruleID, RuleName: cr.ruleName, Category: cr.category,
			Action: cr.action, Pattern: cr.rawPattern, MatchedText: matchedText,
			ShadowMode: cr.shadowMode, Priority: cr.priority, RewriteTo: cr.rewriteTo,
		})
	}

	return matches
}

// ============================================================
// v28.0 LLM 规则模板 CRUD（DB 持久化）
// ============================================================

// SetTemplateDB 设置 LLM 规则模板 DB 并加载内置模板
func (e *LLMRuleEngine) SetTemplateDB(db *sql.DB) {
	if db == nil {
		return
	}
	e.mu.Lock()
	e.templateDB = db
	e.mu.Unlock()

	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_rule_templates (
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
	db.Exec(`ALTER TABLE llm_rule_templates ADD COLUMN enabled INTEGER DEFAULT 0`)

	// 加载内置模板
	e.loadBuiltinLLMTemplates(db)
}

// loadBuiltinLLMTemplates 启动时 INSERT OR IGNORE 内置模板 + UPDATE 同步描述
func (e *LLMRuleEngine) loadBuiltinLLMTemplates(db *sql.DB) {
	if db == nil {
		return
	}
	builtins := getDefaultLLMTemplates()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, tpl := range builtins {
		rulesJSON, err := json.Marshal(tpl.Rules)
		if err != nil {
			continue
		}
		// INSERT OR IGNORE + 随后 UPDATE 同步描述和规则
		db.Exec(`INSERT OR IGNORE INTO llm_rule_templates (id, name, description, category, rules_json, built_in, created_at, updated_at)
			VALUES (?,?,?,?,?,1,?,?)`,
			tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, now)
		// 同步更新内置模板的描述和规则（代码变更时自动同步）
		db.Exec(`UPDATE llm_rule_templates SET name=?, description=?, category=?, rules_json=?, updated_at=?
			WHERE id=? AND built_in=1`,
			tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, tpl.ID)
	}
	log.Printf("[LLM规则] 加载 %d 个内置 LLM 规则模板", len(builtins))
}

// ListLLMTemplates 列出所有 LLM 规则模板
func (e *LLMRuleEngine) ListLLMTemplates() []LLMRuleTemplate {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()

	if db == nil {
		// 无 DB 回退到内存
		return getDefaultLLMTemplates()
	}

	rows, err := db.Query(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM llm_rule_templates ORDER BY id`)
	if err != nil {
		log.Printf("[LLM规则] 查询模板失败: %v", err)
		return getDefaultLLMTemplates()
	}
	defer rows.Close()

	var templates []LLMRuleTemplate
	for rows.Next() {
		var tpl LLMRuleTemplate
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
		return getDefaultLLMTemplates()
	}
	return templates
}

// GetLLMTemplate 获取单个 LLM 规则模板
func (e *LLMRuleEngine) GetLLMTemplate(id string) *LLMRuleTemplate {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()

	if db == nil {
		// 无 DB 回退到内存
		for _, tpl := range getDefaultLLMTemplates() {
			if tpl.ID == id {
				return &tpl
			}
		}
		return nil
	}

	var tpl LLMRuleTemplate
	var rulesJSON string
	var builtIn, enabled int
	err := db.QueryRow(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM llm_rule_templates WHERE id=?`, id).
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

// EnableLLMTemplate 启用/禁用 LLM 模板全局开关（v30.0）
func (e *LLMRuleEngine) EnableLLMTemplate(id string, enabled bool) error {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	val := 0
	if enabled {
		val = 1
	}
	result, err := db.Exec(`UPDATE llm_rule_templates SET enabled=?, updated_at=? WHERE id=?`,
		val, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("LLM 模板 %q 不存在", id)
	}
	// 重建全局 LLM 模板规则
	e.rebuildGlobalLLMTemplateRules()
	return nil
}

// rebuildGlobalLLMTemplateRules 重建全局 LLM 模板规则缓存（v30.0）
func (e *LLMRuleEngine) rebuildGlobalLLMTemplateRules() {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()
	if db == nil {
		return
	}
	var allRules []LLMRule
	// v31.0: 从旧表 + 统一行业模板表 union 读取
	for _, query := range []string{
		`SELECT rules_json FROM llm_rule_templates WHERE enabled=1`,
		`SELECT llm_rules_json FROM industry_templates WHERE enabled=1 AND llm_rules_json != '' AND llm_rules_json != '[]' AND llm_rules_json != 'null'`,
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
			var rules []LLMRule
			if json.Unmarshal([]byte(rulesJSON), &rules) != nil {
				continue
			}
			allRules = append(allRules, rules...)
		}
		rows.Close()
	}
	// 编译为全局模板检测规则（复用 SetTenantLLMRules 的编译逻辑）
	e.mu.Lock()
	e.globalTemplateRules = allRules
	e.mu.Unlock()
	// 重新编译 AC 和正则
	e.compileGlobalTemplateRules(allRules)
	log.Printf("[LLM规则] 全局模板: %d 条规则已重建", len(allRules))
}

// compileGlobalTemplateRules 编译全局模板规则到 AC 和正则（v30.0）
func (e *LLMRuleEngine) compileGlobalTemplateRules(rules []LLMRule) {
	var reqKeywords, respKeywords, bothKeywords []string
	type acEntry struct {
		ruleID, ruleName, category, action, pattern, rewriteTo string
		shadowMode                                              bool
		priority                                                int
	}
	var reqACEntries, respACEntries []acEntry
	var reqRegex, respRegex []compiledLLMRegexRule

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		for _, p := range rule.Patterns {
			entry := acEntry{ruleID: rule.ID, ruleName: rule.Name, category: rule.Category, action: rule.Action, pattern: p, rewriteTo: rule.RewriteTo, shadowMode: rule.ShadowMode, priority: rule.Priority}
			switch rule.Type {
			case "regex":
				compiled, err := regexp.Compile(p)
				if err != nil {
					continue
				}
				cr := compiledLLMRegexRule{ruleID: rule.ID, ruleName: rule.Name, category: rule.Category, action: rule.Action, rawPattern: p, pattern: compiled, shadowMode: rule.ShadowMode, priority: rule.Priority, rewriteTo: rule.RewriteTo}
				switch rule.Direction {
				case "request":
					reqRegex = append(reqRegex, cr)
				case "response":
					respRegex = append(respRegex, cr)
				default:
					reqRegex = append(reqRegex, cr)
					respRegex = append(respRegex, cr)
				}
			default: // keyword
				switch rule.Direction {
				case "request":
					reqKeywords = append(reqKeywords, p)
					reqACEntries = append(reqACEntries, entry)
				case "response":
					respKeywords = append(respKeywords, p)
					respACEntries = append(respACEntries, entry)
				default:
					bothKeywords = append(bothKeywords, p)
					reqACEntries = append(reqACEntries, entry)
					respACEntries = append(respACEntries, entry)
				}
			}
		}
	}

	// 构建 AC 自动机
	allReqKW := append(reqKeywords, bothKeywords...)
	allRespKW := append(respKeywords, bothKeywords...)

	e.mu.Lock()
	if len(allReqKW) > 0 {
		e.globalTplReqAC = NewAhoCorasick(allReqKW)
		e.globalTplReqACRules = make([]llmACEntry, len(reqACEntries))
		for i, entry := range reqACEntries {
			e.globalTplReqACRules[i] = llmACEntry{ruleID: entry.ruleID, ruleName: entry.ruleName, category: entry.category, action: entry.action, pattern: entry.pattern, rewriteTo: entry.rewriteTo, shadowMode: entry.shadowMode, priority: entry.priority}
		}
	} else {
		e.globalTplReqAC = nil
		e.globalTplReqACRules = nil
	}
	if len(allRespKW) > 0 {
		e.globalTplRespAC = NewAhoCorasick(allRespKW)
		e.globalTplRespACRules = make([]llmACEntry, len(respACEntries))
		for i, entry := range respACEntries {
			e.globalTplRespACRules[i] = llmACEntry{ruleID: entry.ruleID, ruleName: entry.ruleName, category: entry.category, action: entry.action, pattern: entry.pattern, rewriteTo: entry.rewriteTo, shadowMode: entry.shadowMode, priority: entry.priority}
		}
	} else {
		e.globalTplRespAC = nil
		e.globalTplRespACRules = nil
	}
	e.globalTplReqRegex = reqRegex
	e.globalTplRespRegex = respRegex
	e.mu.Unlock()
}

// InitGlobalLLMTemplateRules 启动时初始化全局 LLM 模板规则（v30.0）
func (e *LLMRuleEngine) InitGlobalLLMTemplateRules() {
	e.rebuildGlobalLLMTemplateRules()
}

// CreateLLMTemplate 创建自定义 LLM 规则模板
func (e *LLMRuleEngine) CreateLLMTemplate(tpl LLMRuleTemplate) error {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}
	if tpl.ID == "" {
		return fmt.Errorf("模板 ID 不能为空")
	}

	// 检查是否已存在
	existing := e.GetLLMTemplate(tpl.ID)
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
	_, err = db.Exec(`INSERT INTO llm_rule_templates (id, name, description, category, rules_json, built_in, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), builtIn, now, now)
	if err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}
	return nil
}

// UpdateLLMTemplate 更新 LLM 规则模板
func (e *LLMRuleEngine) UpdateLLMTemplate(id string, tpl LLMRuleTemplate) error {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}

	existing := e.GetLLMTemplate(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}

	rulesJSON, err := json.Marshal(tpl.Rules)
	if err != nil {
		return fmt.Errorf("序列化规则失败: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`UPDATE llm_rule_templates SET name=?, description=?, category=?, rules_json=?, updated_at=? WHERE id=?`,
		tpl.Name, tpl.Description, tpl.Category, string(rulesJSON), now, id)
	if err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}
	return nil
}

// DeleteLLMTemplate 删除 LLM 规则模板（内置模板不可删）
func (e *LLMRuleEngine) DeleteLLMTemplate(id string) error {
	e.mu.RLock()
	db := e.templateDB
	e.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("模板 DB 未初始化")
	}

	existing := e.GetLLMTemplate(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	if existing.BuiltIn {
		return fmt.Errorf("内置模板 %q 不可删除", id)
	}

	_, err := db.Exec(`DELETE FROM llm_rule_templates WHERE id=? AND built_in=0`, id)
	if err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}
	return nil
}
