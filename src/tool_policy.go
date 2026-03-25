// tool_policy.go — ToolPolicyEngine: LLM tool_calls 策略引擎
// lobster-guard v20.0
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

// ToolPolicyConfig 工具策略配置
type ToolPolicyConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	DefaultAction  string `yaml:"default_action" json:"default_action"`
	MaxCallsPerMin int    `yaml:"max_calls_per_min" json:"max_calls_per_min"`
}

// ToolPolicyRule 工具策略规则
type ToolPolicyRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ToolPattern string      `json:"tool_pattern"`
	ParamRules  []ParamRule `json:"param_rules"`
	Action      string      `json:"action"`
	Reason      string      `json:"reason"`
	Enabled     bool        `json:"enabled"`
	Priority    int         `json:"priority"`
}

// ParamRule 参数检测规则
type ParamRule struct {
	ParamName string `json:"param_name"`
	Pattern   string `json:"pattern"`
	Action    string `json:"action"`
}

// ToolCallEvent 工具调用事件
type ToolCallEvent struct {
	ID        string                 `json:"id"`
	TraceID   string                 `json:"trace_id"`
	Timestamp time.Time              `json:"timestamp"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Decision  string                 `json:"decision"`
	RuleHit   string                 `json:"rule_hit"`
	RiskLevel string                 `json:"risk_level"`
	TenantID  string                 `json:"tenant_id"`
}

// RiskScoreNum 将风险等级转为数值
func (e *ToolCallEvent) RiskScoreNum() float64 {
	switch e.RiskLevel {
	case "critical":
		return 100
	case "high":
		return 80
	case "medium":
		return 50
	case "low":
		return 20
	default:
		return 0
	}
}

// ToolPolicyEngine 工具调用策略引擎
type ToolPolicyEngine struct {
	db          *sql.DB
	mu          sync.RWMutex
	config      ToolPolicyConfig
	rules       []ToolPolicyRule
	rateMu      sync.Mutex
	rateWindows map[string][]time.Time
	regexCache  map[string]*regexp.Regexp
}

// defaultToolPolicyRules 内置默认规则（18 条）
var defaultToolPolicyRules = []ToolPolicyRule{
	{ID: "tp-001", Name: "block_shell_exec", ToolPattern: "*shell*", Action: "block", Reason: "Shell 执行类工具", Enabled: true, Priority: 1},
	{ID: "tp-002", Name: "block_code_exec", ToolPattern: "*execute*code*", Action: "block", Reason: "代码执行类工具", Enabled: true, Priority: 1},
	{ID: "tp-003", Name: "block_eval", ToolPattern: "*eval*", Action: "block", Reason: "Eval 类工具", Enabled: true, Priority: 1},
	{ID: "tp-004", Name: "block_file_write", ToolPattern: "*write*file*", Action: "block", Reason: "文件写入工具", Enabled: true, Priority: 2},
	{ID: "tp-005", Name: "block_db_drop", ToolPattern: "*database*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(DROP|TRUNCATE|DELETE\s+FROM)`, Action: "block"}}, Action: "warn", Reason: "数据库破坏性操作", Enabled: true, Priority: 1},
	{ID: "tp-006", Name: "block_system_cmd", ToolPattern: "*command*", Action: "block", Reason: "系统命令执行工具", Enabled: true, Priority: 1},
	{ID: "tp-007", Name: "block_sudo", ToolPattern: "*sudo*", Action: "block", Reason: "提权类工具", Enabled: true, Priority: 1},
	{ID: "tp-008", Name: "warn_file_read", ToolPattern: "*read*file*", Action: "warn", Reason: "文件读取工具", Enabled: true, Priority: 5},
	{ID: "tp-009", Name: "warn_http_request", ToolPattern: "*http*", Action: "warn", Reason: "HTTP 请求工具", Enabled: true, Priority: 5},
	{ID: "tp-010", Name: "warn_email_send", ToolPattern: "*send*email*", Action: "warn", Reason: "邮件发送工具", Enabled: true, Priority: 5},
	{ID: "tp-011", Name: "warn_network_tool", ToolPattern: "*network*", Action: "warn", Reason: "网络操作工具", Enabled: true, Priority: 5},
	{ID: "tp-012", Name: "block_sensitive_path", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(/etc/passwd|/etc/shadow|~/\.ssh/|\.env|\.git/config)`, Action: "block"}}, Action: "allow", Reason: "敏感路径访问", Enabled: true, Priority: 3},
	{ID: "tp-013", Name: "block_credential_in_param", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(password|secret|api.?key|token|credential)`, Action: "warn"}}, Action: "allow", Reason: "参数含凭据关键词", Enabled: true, Priority: 4},
	{ID: "tp-014", Name: "warn_large_query", ToolPattern: "*query*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(SELECT\s+\*|LIMIT\s+\d{4,})`, Action: "warn"}}, Action: "allow", Reason: "大范围数据查询", Enabled: true, Priority: 5},
	{ID: "tp-015", Name: "block_curl_pipe_bash", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(curl\s+.*\|\s*(ba)?sh|wget\s+.*\|\s*(ba)?sh|base64\s+-d\s*\|\s*(ba)?sh)`, Action: "block"}}, Action: "allow", Reason: "远程代码执行模式", Enabled: true, Priority: 2},
	{ID: "tp-016", Name: "warn_sql_injection", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(UNION\s+SELECT|OR\s+1\s*=\s*1|'\s*OR\s*'|--\s*$)`, Action: "warn"}}, Action: "allow", Reason: "疑似 SQL 注入", Enabled: true, Priority: 3},
	{ID: "tp-017", Name: "block_rm_rf", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(rm\s+-rf\s+/|rm\s+-rf\s+~|mkfs\s|dd\s+if=)`, Action: "block"}}, Action: "allow", Reason: "破坏性系统命令", Enabled: true, Priority: 1},
	{ID: "tp-018", Name: "block_reverse_shell", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(bash\s+-i\s+>&\s*/dev/tcp|nc\s+-e\s+/bin|python.*socket.*connect)`, Action: "block"}}, Action: "allow", Reason: "反弹 Shell 模式", Enabled: true, Priority: 1},
}

// NewToolPolicyEngine 创建工具策略引擎
func NewToolPolicyEngine(db *sql.DB, cfg ToolPolicyConfig) *ToolPolicyEngine {
	if cfg.DefaultAction == "" {
		cfg.DefaultAction = "allow"
	}
	if cfg.MaxCallsPerMin <= 0 {
		cfg.MaxCallsPerMin = 60
	}
	engine := &ToolPolicyEngine{
		db:          db,
		config:      cfg,
		rateWindows: make(map[string][]time.Time),
		regexCache:  make(map[string]*regexp.Regexp),
	}
	engine.initDB()
	engine.loadRules()
	return engine
}

func (e *ToolPolicyEngine) initDB() {
	e.db.Exec(`CREATE TABLE IF NOT EXISTS tool_call_events (
		id TEXT PRIMARY KEY,
		trace_id TEXT DEFAULT '',
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		arguments_json TEXT DEFAULT '{}',
		decision TEXT NOT NULL,
		rule_hit TEXT DEFAULT '',
		risk_level TEXT DEFAULT 'low',
		tenant_id TEXT DEFAULT ''
	)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_events_ts ON tool_call_events(timestamp)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_events_tool ON tool_call_events(tool_name)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_events_decision ON tool_call_events(decision)`)
	e.db.Exec(`CREATE TABLE IF NOT EXISTS tool_policy_rules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		tool_pattern TEXT NOT NULL,
		param_rules_json TEXT DEFAULT '[]',
		action TEXT NOT NULL,
		reason TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 10
	)`)
}

func (e *ToolPolicyEngine) loadRules() {
	e.mu.Lock()
	defer e.mu.Unlock()
	rules, err := e.loadRulesFromDB()
	if err != nil || len(rules) == 0 {
		for _, r := range defaultToolPolicyRules {
			e.saveRuleToDB(r)
		}
		e.rules = make([]ToolPolicyRule, len(defaultToolPolicyRules))
		copy(e.rules, defaultToolPolicyRules)
	} else {
		e.rules = rules
	}
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority < e.rules[j].Priority
	})
	e.compileRegexCache()
}

func (e *ToolPolicyEngine) loadRulesFromDB() ([]ToolPolicyRule, error) {
	rows, err := e.db.Query(`SELECT id, name, tool_pattern, param_rules_json, action, reason, enabled, priority FROM tool_policy_rules ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []ToolPolicyRule
	for rows.Next() {
		var r ToolPolicyRule
		var paramJSON string
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.ToolPattern, &paramJSON, &r.Action, &r.Reason, &enabled, &r.Priority); err != nil {
			continue
		}
		r.Enabled = enabled != 0
		json.Unmarshal([]byte(paramJSON), &r.ParamRules)
		rules = append(rules, r)
	}
	return rules, nil
}

func (e *ToolPolicyEngine) saveRuleToDB(r ToolPolicyRule) error {
	paramJSON, _ := json.Marshal(r.ParamRules)
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := e.db.Exec(`INSERT OR REPLACE INTO tool_policy_rules (id, name, tool_pattern, param_rules_json, action, reason, enabled, priority) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.Name, r.ToolPattern, string(paramJSON), r.Action, r.Reason, enabled, r.Priority)
	return err
}

func (e *ToolPolicyEngine) compileRegexCache() {
	e.regexCache = make(map[string]*regexp.Regexp)
	for _, rule := range e.rules {
		for _, pr := range rule.ParamRules {
			if pr.Pattern != "" {
				if _, exists := e.regexCache[pr.Pattern]; !exists {
					if re, err := regexp.Compile(pr.Pattern); err == nil {
						e.regexCache[pr.Pattern] = re
					}
				}
			}
		}
	}
}

func (e *ToolPolicyEngine) getCompiledRegex(pattern string) *regexp.Regexp {
	if re, ok := e.regexCache[pattern]; ok {
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	e.regexCache[pattern] = re
	return re
}

// Evaluate 评估工具调用，返回决策事件
func (e *ToolPolicyEngine) Evaluate(toolName string, arguments string, traceID string, tenantID string) *ToolCallEvent {
	now := time.Now().UTC()
	event := &ToolCallEvent{
		ID:        fmt.Sprintf("tce-%s-%d", GenerateTraceID(), now.UnixNano()%100000),
		TraceID:   traceID,
		Timestamp: now,
		ToolName:  toolName,
		Decision:  e.config.DefaultAction,
		RiskLevel: "low",
		TenantID:  tenantID,
	}

	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			args = map[string]interface{}{"_raw": arguments}
		}
	}
	event.Arguments = args

	e.mu.RLock()
	rules := make([]ToolPolicyRule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	// 频率限制检查
	if e.checkRateLimit(tenantID) {
		event.Decision = "block"
		event.RuleHit = "rate_limit"
		event.RiskLevel = "high"
		e.recordEvent(event)
		return event
	}

	highestDecision := ""
	highestRuleHit := ""
	highestRiskLevel := "low"

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if !wildcardMatch(rule.ToolPattern, strings.ToLower(toolName)) {
			continue
		}

		paramDecision := e.evaluateParamRules(rule.ParamRules, args)
		if paramDecision != "" {
			if toolPolicyActionSeverity(paramDecision) > toolPolicyActionSeverity(highestDecision) {
				highestDecision = paramDecision
				highestRuleHit = rule.Name
				highestRiskLevel = classifyRiskFromAction(paramDecision, rule.Priority)
			}
			continue
		}

		if rule.Action != "allow" && toolPolicyActionSeverity(rule.Action) > toolPolicyActionSeverity(highestDecision) {
			highestDecision = rule.Action
			highestRuleHit = rule.Name
			highestRiskLevel = classifyRiskFromAction(rule.Action, rule.Priority)
		}
	}

	if highestDecision != "" {
		event.Decision = highestDecision
		event.RuleHit = highestRuleHit
		event.RiskLevel = highestRiskLevel
	}

	e.recordEvent(event)
	return event
}

func (e *ToolPolicyEngine) evaluateParamRules(paramRules []ParamRule, args map[string]interface{}) string {
	if len(paramRules) == 0 || args == nil {
		return ""
	}
	highestAction := ""
	for _, pr := range paramRules {
		re := e.getCompiledRegex(pr.Pattern)
		if re == nil {
			continue
		}
		if pr.ParamName == "*" {
			for _, v := range args {
				strVal := fmt.Sprintf("%v", v)
				if re.MatchString(strVal) {
					if toolPolicyActionSeverity(pr.Action) > toolPolicyActionSeverity(highestAction) {
						highestAction = pr.Action
					}
				}
			}
		} else {
			if v, ok := args[pr.ParamName]; ok {
				strVal := fmt.Sprintf("%v", v)
				if re.MatchString(strVal) {
					if toolPolicyActionSeverity(pr.Action) > toolPolicyActionSeverity(highestAction) {
						highestAction = pr.Action
					}
				}
			}
		}
	}
	return highestAction
}

func (e *ToolPolicyEngine) checkRateLimit(tenantID string) bool {
	if e.config.MaxCallsPerMin <= 0 {
		return false
	}
	e.rateMu.Lock()
	defer e.rateMu.Unlock()
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)
	key := tenantID
	if key == "" {
		key = "default"
	}
	timestamps := e.rateWindows[key]
	validStart := 0
	for i, ts := range timestamps {
		if ts.After(windowStart) {
			validStart = i
			break
		}
		if i == len(timestamps)-1 {
			validStart = len(timestamps)
		}
	}
	if len(timestamps) > 0 {
		timestamps = timestamps[validStart:]
	}
	if len(timestamps) >= e.config.MaxCallsPerMin {
		e.rateWindows[key] = timestamps
		return true
	}
	timestamps = append(timestamps, now)
	e.rateWindows[key] = timestamps
	return false
}

func (e *ToolPolicyEngine) recordEvent(event *ToolCallEvent) {
	argsJSON, _ := json.Marshal(event.Arguments)
	_, err := e.db.Exec(`INSERT INTO tool_call_events (id, trace_id, timestamp, tool_name, arguments_json, decision, rule_hit, risk_level, tenant_id) VALUES (?,?,?,?,?,?,?,?,?)`,
		event.ID, event.TraceID, event.Timestamp.Format(time.RFC3339), event.ToolName,
		string(argsJSON), event.Decision, event.RuleHit, event.RiskLevel, event.TenantID)
	if err != nil {
		log.Printf("[ToolPolicy] 记录事件失败: %v", err)
	}
}

// AddRule 添加规则
func (e *ToolPolicyEngine) AddRule(rule ToolPolicyRule) error {
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("tp-custom-%d", time.Now().UnixNano())
	}
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if rule.ToolPattern == "" {
		return fmt.Errorf("tool_pattern is required")
	}
	if rule.Action == "" {
		rule.Action = "warn"
	}
	for _, pr := range rule.ParamRules {
		if pr.Pattern != "" {
			if _, err := regexp.Compile(pr.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern %q: %v", pr.Pattern, err)
			}
		}
	}
	if err := e.saveRuleToDB(rule); err != nil {
		return fmt.Errorf("save rule failed: %v", err)
	}
	e.mu.Lock()
	e.rules = append(e.rules, rule)
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority < e.rules[j].Priority
	})
	e.compileRegexCache()
	e.mu.Unlock()
	return nil
}

// UpdateRule 更新规则
func (e *ToolPolicyEngine) UpdateRule(rule ToolPolicyRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule id is required")
	}
	for _, pr := range rule.ParamRules {
		if pr.Pattern != "" {
			if _, err := regexp.Compile(pr.Pattern); err != nil {
				return fmt.Errorf("invalid regex pattern %q: %v", pr.Pattern, err)
			}
		}
	}
	if err := e.saveRuleToDB(rule); err != nil {
		return fmt.Errorf("save rule failed: %v", err)
	}
	e.mu.Lock()
	for i, r := range e.rules {
		if r.ID == rule.ID {
			e.rules[i] = rule
			break
		}
	}
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority < e.rules[j].Priority
	})
	e.compileRegexCache()
	e.mu.Unlock()
	return nil
}

// RemoveRule 删除规则
func (e *ToolPolicyEngine) RemoveRule(id string) error {
	_, err := e.db.Exec(`DELETE FROM tool_policy_rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	e.mu.Lock()
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			break
		}
	}
	e.mu.Unlock()
	return nil
}

// ListRules 列出规则
func (e *ToolPolicyEngine) ListRules() []ToolPolicyRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ToolPolicyRule, len(e.rules))
	copy(result, e.rules)
	return result
}

// GetConfig 获取配置
func (e *ToolPolicyEngine) GetConfig() ToolPolicyConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// UpdateConfig 更新配置
func (e *ToolPolicyEngine) UpdateConfig(cfg ToolPolicyConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = cfg
}

// Stats 统计信息
func (e *ToolPolicyEngine) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_events": 0,
		"by_decision":  map[string]int{"allow": 0, "warn": 0, "block": 0},
		"by_risk":      map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
		"top_tools":    []map[string]interface{}{},
		"top_rules":    []map[string]interface{}{},
		"blocked_24h":  0,
		"warned_24h":   0,
		"total_rules":  len(e.ListRules()),
		"config":       e.GetConfig(),
	}

	var total int
	e.db.QueryRow(`SELECT COUNT(*) FROM tool_call_events`).Scan(&total)
	stats["total_events"] = total

	rows, err := e.db.Query(`SELECT decision, COUNT(*) FROM tool_call_events GROUP BY decision`)
	if err == nil {
		defer rows.Close()
		byDecision := map[string]int{"allow": 0, "warn": 0, "block": 0}
		for rows.Next() {
			var d string
			var c int
			if rows.Scan(&d, &c) == nil {
				byDecision[d] = c
			}
		}
		stats["by_decision"] = byDecision
	}

	rows2, err := e.db.Query(`SELECT risk_level, COUNT(*) FROM tool_call_events GROUP BY risk_level`)
	if err == nil {
		defer rows2.Close()
		byRisk := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
		for rows2.Next() {
			var r string
			var c int
			if rows2.Scan(&r, &c) == nil {
				byRisk[r] = c
			}
		}
		stats["by_risk"] = byRisk
	}

	rows3, err := e.db.Query(`SELECT tool_name, COUNT(*) as cnt FROM tool_call_events GROUP BY tool_name ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer rows3.Close()
		var topTools []map[string]interface{}
		for rows3.Next() {
			var name string
			var cnt int
			if rows3.Scan(&name, &cnt) == nil {
				topTools = append(topTools, map[string]interface{}{"tool_name": name, "count": cnt})
			}
		}
		if topTools != nil {
			stats["top_tools"] = topTools
		}
	}

	rows4, err := e.db.Query(`SELECT rule_hit, COUNT(*) as cnt FROM tool_call_events WHERE rule_hit != '' GROUP BY rule_hit ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer rows4.Close()
		var topRules []map[string]interface{}
		for rows4.Next() {
			var name string
			var cnt int
			if rows4.Scan(&name, &cnt) == nil {
				topRules = append(topRules, map[string]interface{}{"rule": name, "count": cnt})
			}
		}
		if topRules != nil {
			stats["top_rules"] = topRules
		}
	}

	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	var blocked24h, warned24h int
	e.db.QueryRow(`SELECT COUNT(*) FROM tool_call_events WHERE decision='block' AND timestamp>=?`, since24h).Scan(&blocked24h)
	e.db.QueryRow(`SELECT COUNT(*) FROM tool_call_events WHERE decision='warn' AND timestamp>=?`, since24h).Scan(&warned24h)
	stats["blocked_24h"] = blocked24h
	stats["warned_24h"] = warned24h

	return stats
}

// QueryEvents 查询事件列表
func (e *ToolPolicyEngine) QueryEvents(toolName, decision, risk string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	if toolName != "" {
		where += " AND tool_name=?"
		args = append(args, toolName)
	}
	if decision != "" {
		where += " AND decision=?"
		args = append(args, decision)
	}
	if risk != "" {
		where += " AND risk_level=?"
		args = append(args, risk)
	}
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	e.db.QueryRow("SELECT COUNT(*) FROM tool_call_events "+where, countArgs...).Scan(&total)
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	query := "SELECT id, trace_id, timestamp, tool_name, arguments_json, decision, rule_hit, risk_level, tenant_id FROM tool_call_events " + where + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var records []map[string]interface{}
	for rows.Next() {
		var id, traceID, ts, tool, argsJSON, dec, ruleHit, riskLvl, tenant string
		if rows.Scan(&id, &traceID, &ts, &tool, &argsJSON, &dec, &ruleHit, &riskLvl, &tenant) != nil {
			continue
		}
		records = append(records, map[string]interface{}{
			"id": id, "trace_id": traceID, "timestamp": ts, "tool_name": tool,
			"arguments": argsJSON, "decision": dec, "rule_hit": ruleHit,
			"risk_level": riskLvl, "tenant_id": tenant,
		})
	}
	return records, total, nil
}

// ============================================================
// 辅助函数
// ============================================================

// wildcardMatch 支持 * 通配符的字符串匹配
func wildcardMatch(pattern, name string) bool {
	pattern = strings.ToLower(pattern)
	name = strings.ToLower(name)
	if pattern == "*" {
		return true
	}
	// 将通配符模式转换为简单匹配
	// 支持 *xxx*, xxx*, *xxx, x*y 等
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	// 检查首段是否匹配开头
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(name[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			// 第一段不是 * 开头，必须匹配开头
			return false
		}
		pos += idx + len(part)
	}
	// 如果模式不以 * 结尾，则 name 必须在此结束
	if parts[len(parts)-1] != "" && pos != len(name) {
		return false
	}
	return true
}

// toolPolicyActionSeverity 返回动作的严重程度（用于比较）
func toolPolicyActionSeverity(action string) int {
	switch action {
	case "block":
		return 3
	case "warn":
		return 2
	case "allow":
		return 1
	default:
		return 0
	}
}

// classifyRiskFromAction 根据动作和优先级推断风险等级
func classifyRiskFromAction(action string, priority int) string {
	switch action {
	case "block":
		if priority <= 1 {
			return "critical"
		}
		return "high"
	case "warn":
		if priority <= 3 {
			return "medium"
		}
		return "medium"
	default:
		return "low"
	}
}