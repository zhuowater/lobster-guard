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

// ToolSemanticRule 可配置语义分类规则
type ToolSemanticRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ToolPattern string   `json:"tool_pattern"`
	ParamKeys   []string `json:"param_keys"`
	MatchType   string   `json:"match_type"`
	Pattern     string   `json:"pattern"`
	Class       string   `json:"class"`
	Action      string   `json:"action"`
	RiskLevel   string   `json:"risk_level"`
	Enabled     bool     `json:"enabled"`
	Priority    int      `json:"priority"`
}

// ToolContextPolicy 可配置上下文链路策略
type ToolContextPolicy struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SourceClasses []string `json:"source_classes"`
	TargetClasses []string `json:"target_classes"`
	TargetTools   []string `json:"target_tools"`
	Action        string   `json:"action"`
	RiskLevel     string   `json:"risk_level"`
	Enabled       bool     `json:"enabled"`
	Priority      int      `json:"priority"`
	WindowSize    int      `json:"window_size"`
}

// ToolCallEvent 工具调用事件
type ToolCallEvent struct {
	ID             string                 `json:"id"`
	TraceID        string                 `json:"trace_id"`
	Timestamp      time.Time              `json:"timestamp"`
	ToolName       string                 `json:"tool_name"`
	Arguments      map[string]interface{} `json:"arguments"`
	SemanticClass  string                 `json:"semantic_class,omitempty"`
	ContextSignals []string               `json:"context_signals,omitempty"`
	Decision       string                 `json:"decision"`
	RuleHit        string                 `json:"rule_hit"`
	RiskLevel      string                 `json:"risk_level"`
	TenantID       string                 `json:"tenant_id"`
}

type toolSemanticAssessment struct {
	Class     string
	Decision  string
	RuleHit   string
	RiskLevel string
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
	db              *sql.DB
	mu              sync.RWMutex
	config          ToolPolicyConfig
	rules           []ToolPolicyRule
	semanticRules   []ToolSemanticRule
	contextPolicies []ToolContextPolicy
	rateMu          sync.Mutex
	rateWindows     map[string][]time.Time
	regexCache      map[string]*regexp.Regexp
}

// defaultToolPolicyRules 内置默认规则（18 条）
var defaultToolPolicyRules = []ToolPolicyRule{
	{ID: "tp-001", Name: "block_shell_exec", ToolPattern: "*shell*", Action: "block", Reason: "Shell 执行类工具", Enabled: true, Priority: 1},
	{ID: "tp-002", Name: "block_code_exec", ToolPattern: "*execute*code*", Action: "block", Reason: "代码执行类工具", Enabled: true, Priority: 1},
	{ID: "tp-003", Name: "block_eval", ToolPattern: "*eval*", Action: "block", Reason: "Eval 类工具", Enabled: true, Priority: 1},
	{ID: "tp-004", Name: "block_file_write", ToolPattern: "*write*file*", Action: "block", Reason: "文件写入工具", Enabled: true, Priority: 2},
	{ID: "tp-005", Name: "block_db_drop", ToolPattern: "*database*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(DROP|TRUNCATE|DELETE\s+FROM)`, Action: "block"}}, Action: "warn", Reason: "数据库破坏性操作", Enabled: true, Priority: 1},
	{ID: "tp-006", Name: "warn_command_build_test", ToolPattern: "*command*", ParamRules: []ParamRule{{ParamName: "command", Pattern: `(?i)^\s*(python\s+-m\s+pytest|pytest|go\s+test\b|go\s+build\b|npm\s+(run\s+)?(build|test)\b|make\s+(test|build)\b|cargo\s+(test|build)\b)`, Action: "warn"}, {ParamName: "cmd", Pattern: `(?i)^\s*(python\s+-m\s+pytest|pytest|go\s+test\b|go\s+build\b|npm\s+(run\s+)?(build|test)\b|make\s+(test|build)\b|cargo\s+(test|build)\b)`, Action: "warn"}}, Action: "allow", Reason: "开发执行命令", Enabled: true, Priority: 5},
	{ID: "tp-007", Name: "warn_command_system_mutation", ToolPattern: "*command*", ParamRules: []ParamRule{{ParamName: "command", Pattern: `(?i)(systemctl\b|service\b|crontab\b|chmod\b|chown\b|sed\s+-i\b|kubectl\s+(apply|delete|patch)\b|docker\s+(exec|run|rm|stop|restart)\b|mount\b|umount\b)`, Action: "warn"}, {ParamName: "cmd", Pattern: `(?i)(systemctl\b|service\b|crontab\b|chmod\b|chown\b|sed\s+-i\b|kubectl\s+(apply|delete|patch)\b|docker\s+(exec|run|rm|stop|restart)\b|mount\b|umount\b)`, Action: "warn"}}, Action: "allow", Reason: "系统修改命令", Enabled: true, Priority: 3},
	{ID: "tp-008", Name: "block_system_cmd", ToolPattern: "*system*command*", Action: "block", Reason: "系统命令执行工具", Enabled: true, Priority: 1},
	{ID: "tp-009", Name: "block_sudo", ToolPattern: "*sudo*", Action: "block", Reason: "提权类工具", Enabled: true, Priority: 1},
	{ID: "tp-010", Name: "warn_file_read", ToolPattern: "*read*file*", Action: "warn", Reason: "文件读取工具", Enabled: true, Priority: 5},
	{ID: "tp-011", Name: "warn_http_request", ToolPattern: "*http*", Action: "warn", Reason: "HTTP 请求工具", Enabled: true, Priority: 5},
	{ID: "tp-012", Name: "warn_email_send", ToolPattern: "*send*email*", Action: "warn", Reason: "邮件发送工具", Enabled: true, Priority: 5},
	{ID: "tp-013", Name: "warn_network_tool", ToolPattern: "*network*", Action: "warn", Reason: "网络操作工具", Enabled: true, Priority: 5},
	{ID: "tp-014", Name: "block_sensitive_path", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(/etc/passwd|/etc/shadow|~/\.ssh/|\.env|\.git/config)`, Action: "block"}}, Action: "allow", Reason: "敏感路径访问", Enabled: true, Priority: 3},
	{ID: "tp-015", Name: "block_credential_in_param", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(password|secret|api.?key|token|credential)`, Action: "warn"}}, Action: "allow", Reason: "参数含凭据关键词", Enabled: true, Priority: 4},
	{ID: "tp-016", Name: "warn_large_query", ToolPattern: "*query*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(SELECT\s+\*|LIMIT\s+\d{4,})`, Action: "warn"}}, Action: "allow", Reason: "大范围数据查询", Enabled: true, Priority: 5},
	{ID: "tp-017", Name: "block_curl_pipe_bash", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(curl\s+.*\|\s*(ba)?sh|wget\s+.*\|\s*(ba)?sh|base64\s+-d\s*\|\s*(ba)?sh)`, Action: "block"}}, Action: "allow", Reason: "远程代码执行模式", Enabled: true, Priority: 2},
	{ID: "tp-018", Name: "warn_sql_injection", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(UNION\s+SELECT|OR\s+1\s*=\s*1|'\s*OR\s*'|--\s*$)`, Action: "warn"}}, Action: "allow", Reason: "疑似 SQL 注入", Enabled: true, Priority: 3},
	{ID: "tp-019", Name: "block_rm_rf", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(rm\s+-rf\s+/|rm\s+-rf\s+~|mkfs\s|dd\s+if=)`, Action: "block"}}, Action: "allow", Reason: "破坏性系统命令", Enabled: true, Priority: 1},
	{ID: "tp-020", Name: "block_reverse_shell", ToolPattern: "*", ParamRules: []ParamRule{{ParamName: "*", Pattern: `(?i)(bash\s+-i\s+>&\s*/dev/tcp|nc\s+-e\s+/bin|python.*socket.*connect)`, Action: "block"}}, Action: "allow", Reason: "反弹 Shell 模式", Enabled: true, Priority: 1},
}

var defaultToolSemanticRules = []ToolSemanticRule{
	{ID: "ts-001", Name: "semantic_command_introspection", ToolPattern: "*command*", ParamKeys: []string{"command", "cmd"}, MatchType: "regex", Pattern: `^(pwd|whoami|uname(\s+-[a-z]+)?|id|ls(\s|$)|git\s+status\b|python\s+--version\b|go\s+version\b)`, Class: "command:introspection", Action: "allow", RiskLevel: "low", Enabled: true, Priority: 10},
	{ID: "ts-002", Name: "semantic_command_build_test", ToolPattern: "*command*", ParamKeys: []string{"command", "cmd"}, MatchType: "regex", Pattern: `^(python\s+-m\s+pytest|pytest|go\s+test\b|go\s+build\b|npm\s+(run\s+)?(build|test)\b|make\s+(test|build)\b|cargo\s+(test|build)\b)`, Class: "command:build_test", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 20},
	{ID: "ts-003", Name: "semantic_command_system_mutation", ToolPattern: "*command*", ParamKeys: []string{"command", "cmd"}, MatchType: "regex", Pattern: `(systemctl\b|service\b|crontab\b|chmod\b|chown\b|sed\s+-i\b|kubectl\s+(apply|delete|patch)\b|docker\s+(exec|run|rm|stop|restart)\b|mount\b|umount\b)`, Class: "command:system_mutation", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 30},
	{ID: "ts-004", Name: "semantic_sensitive_path", ToolPattern: "*", ParamKeys: []string{"path", "file_path", "filepath"}, MatchType: "regex", Pattern: `(/etc/passwd|/etc/shadow|/root/\.ssh|/home/[^/]+/\.ssh|\.env\b|\.git/config)`, Class: "path:sensitive", Action: "block", RiskLevel: "high", Enabled: true, Priority: 10},
	{ID: "ts-005", Name: "semantic_metadata_url", ToolPattern: "*", ParamKeys: []string{"url", "uri", "endpoint"}, MatchType: "regex", Pattern: `(169\.254\.169\.254|metadata\.google\.internal|100\.100\.100\.200)`, Class: "url:metadata_service", Action: "block", RiskLevel: "high", Enabled: true, Priority: 10},
	{ID: "ts-006", Name: "semantic_internal_url", ToolPattern: "*", ParamKeys: []string{"url", "uri", "endpoint"}, MatchType: "regex", Pattern: `(?i)https?://(localhost|127\.|10\.|192\.168\.|172\.(1[6-9]|2[0-9]|3[0-1])\.)`, Class: "url:internal_control_plane", Action: "block", RiskLevel: "high", Enabled: true, Priority: 20},
	{ID: "ts-007", Name: "semantic_external_url", ToolPattern: "*", ParamKeys: []string{"url", "uri", "endpoint"}, MatchType: "exists", Pattern: `.+`, Class: "url:external", Action: "allow", RiskLevel: "low", Enabled: true, Priority: 200},
	{ID: "ts-008", Name: "semantic_destructive_query", ToolPattern: "*", ParamKeys: []string{"query", "sql", "statement"}, MatchType: "regex", Pattern: `\b(drop|truncate|alter)\b|\bdelete\s+from\b`, Class: "query:destructive", Action: "block", RiskLevel: "high", Enabled: true, Priority: 10},
	{ID: "ts-009", Name: "semantic_large_extract_query", ToolPattern: "*", ParamKeys: []string{"query", "sql", "statement"}, MatchType: "regex", Pattern: `select\s+\*|limit\s+\d{4,}`, Class: "query:large_extract", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 20},
	{ID: "ts-010", Name: "semantic_read_query", ToolPattern: "*", ParamKeys: []string{"query", "sql", "statement"}, MatchType: "exists", Pattern: `.+`, Class: "query:read", Action: "allow", RiskLevel: "low", Enabled: true, Priority: 200},
	{ID: "ts-011", Name: "semantic_credential_exfiltration", ToolPattern: "*send*", ParamKeys: []string{"content", "body", "message", "text"}, MatchType: "regex", Pattern: `(password|secret|api[_ -]?key|token|credential)|sk-[a-z0-9-]{6,}`, Class: "message:credential_exfiltration", Action: "block", RiskLevel: "high", Enabled: true, Priority: 10},
	{ID: "ts-012", Name: "semantic_external_message", ToolPattern: "*send*", ParamKeys: []string{"content", "body", "message", "text"}, MatchType: "exists", Pattern: `.+`, Class: "message:external_send", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 200},
	{ID: "ts-013", Name: "semantic_external_message_email", ToolPattern: "*email*", ParamKeys: []string{"content", "body", "message", "text"}, MatchType: "exists", Pattern: `.+`, Class: "message:external_send", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 200},
	{ID: "ts-014", Name: "semantic_external_message_webhook", ToolPattern: "*webhook*", ParamKeys: []string{"content", "body", "message", "text", "url"}, MatchType: "exists", Pattern: `.+`, Class: "message:external_send", Action: "warn", RiskLevel: "medium", Enabled: true, Priority: 200},
}

var defaultToolContextPolicies = []ToolContextPolicy{
	{ID: "tc-001", Name: "context_sensitive_read_then_egress", SourceClasses: []string{"path:sensitive", "message:credential_exfiltration"}, TargetClasses: []string{"url:external", "message:external_send", "url:internal_control_plane", "url:metadata_service"}, Action: "block", RiskLevel: "high", Enabled: true, Priority: 10, WindowSize: 12},
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
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_events_decision_ts ON tool_call_events(decision, timestamp)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_events_tenant_id ON tool_call_events(tenant_id)`)
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
	e.db.Exec(`CREATE TABLE IF NOT EXISTS tool_semantic_rules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		tool_pattern TEXT NOT NULL,
		param_keys_json TEXT DEFAULT '[]',
		match_type TEXT DEFAULT 'regex',
		pattern TEXT DEFAULT '',
		class TEXT NOT NULL,
		action TEXT DEFAULT 'allow',
		risk_level TEXT DEFAULT 'low',
		enabled INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 100
	)`)
	e.db.Exec(`CREATE TABLE IF NOT EXISTS tool_context_policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		source_classes_json TEXT DEFAULT '[]',
		target_classes_json TEXT DEFAULT '[]',
		target_tools_json TEXT DEFAULT '[]',
		action TEXT NOT NULL,
		risk_level TEXT DEFAULT 'medium',
		enabled INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 100,
		window_size INTEGER DEFAULT 12
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
	semanticRules, err := e.loadSemanticRulesFromDB()
	if err != nil || len(semanticRules) == 0 {
		for _, r := range defaultToolSemanticRules {
			e.saveSemanticRuleToDB(r)
		}
		e.semanticRules = make([]ToolSemanticRule, len(defaultToolSemanticRules))
		copy(e.semanticRules, defaultToolSemanticRules)
	} else {
		e.semanticRules = semanticRules
	}
	contextPolicies, err := e.loadContextPoliciesFromDB()
	if err != nil || len(contextPolicies) == 0 {
		for _, r := range defaultToolContextPolicies {
			e.saveContextPolicyToDB(r)
		}
		e.contextPolicies = make([]ToolContextPolicy, len(defaultToolContextPolicies))
		copy(e.contextPolicies, defaultToolContextPolicies)
	} else {
		e.contextPolicies = contextPolicies
	}
	sort.Slice(e.rules, func(i, j int) bool { return e.rules[i].Priority < e.rules[j].Priority })
	sort.Slice(e.semanticRules, func(i, j int) bool { return e.semanticRules[i].Priority < e.semanticRules[j].Priority })
	sort.Slice(e.contextPolicies, func(i, j int) bool { return e.contextPolicies[i].Priority < e.contextPolicies[j].Priority })
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

func (e *ToolPolicyEngine) loadSemanticRulesFromDB() ([]ToolSemanticRule, error) {
	rows, err := e.db.Query(`SELECT id, name, tool_pattern, param_keys_json, match_type, pattern, class, action, risk_level, enabled, priority FROM tool_semantic_rules ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []ToolSemanticRule
	for rows.Next() {
		var r ToolSemanticRule
		var keysJSON string
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.ToolPattern, &keysJSON, &r.MatchType, &r.Pattern, &r.Class, &r.Action, &r.RiskLevel, &enabled, &r.Priority); err != nil {
			continue
		}
		r.Enabled = enabled != 0
		_ = json.Unmarshal([]byte(keysJSON), &r.ParamKeys)
		rules = append(rules, r)
	}
	return rules, nil
}

func (e *ToolPolicyEngine) saveSemanticRuleToDB(r ToolSemanticRule) error {
	keysJSON, _ := json.Marshal(r.ParamKeys)
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := e.db.Exec(`INSERT OR REPLACE INTO tool_semantic_rules (id, name, tool_pattern, param_keys_json, match_type, pattern, class, action, risk_level, enabled, priority) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.Name, r.ToolPattern, string(keysJSON), r.MatchType, r.Pattern, r.Class, r.Action, r.RiskLevel, enabled, r.Priority)
	return err
}

func (e *ToolPolicyEngine) loadContextPoliciesFromDB() ([]ToolContextPolicy, error) {
	rows, err := e.db.Query(`SELECT id, name, source_classes_json, target_classes_json, target_tools_json, action, risk_level, enabled, priority, window_size FROM tool_context_policies ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var policies []ToolContextPolicy
	for rows.Next() {
		var p ToolContextPolicy
		var srcJSON, tgtJSON, toolsJSON string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &srcJSON, &tgtJSON, &toolsJSON, &p.Action, &p.RiskLevel, &enabled, &p.Priority, &p.WindowSize); err != nil {
			continue
		}
		p.Enabled = enabled != 0
		_ = json.Unmarshal([]byte(srcJSON), &p.SourceClasses)
		_ = json.Unmarshal([]byte(tgtJSON), &p.TargetClasses)
		_ = json.Unmarshal([]byte(toolsJSON), &p.TargetTools)
		policies = append(policies, p)
	}
	return policies, nil
}

func (e *ToolPolicyEngine) saveContextPolicyToDB(p ToolContextPolicy) error {
	srcJSON, _ := json.Marshal(p.SourceClasses)
	tgtJSON, _ := json.Marshal(p.TargetClasses)
	toolsJSON, _ := json.Marshal(p.TargetTools)
	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	_, err := e.db.Exec(`INSERT OR REPLACE INTO tool_context_policies (id, name, source_classes_json, target_classes_json, target_tools_json, action, risk_level, enabled, priority, window_size) VALUES (?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Name, string(srcJSON), string(tgtJSON), string(toolsJSON), p.Action, p.RiskLevel, enabled, p.Priority, p.WindowSize)
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
	for _, rule := range e.semanticRules {
		if rule.MatchType == "regex" && rule.Pattern != "" {
			if _, exists := e.regexCache[rule.Pattern]; !exists {
				if re, err := regexp.Compile(rule.Pattern); err == nil {
					e.regexCache[rule.Pattern] = re
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
	semantic := e.assessToolSemantics(toolName, args)
	event.SemanticClass = semantic.Class

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

	if semantic.Decision != "" && toolPolicyActionSeverity(semantic.Decision) > toolPolicyActionSeverity(highestDecision) {
		highestDecision = semantic.Decision
		highestRuleHit = semantic.RuleHit
		highestRiskLevel = semantic.RiskLevel
	}
	if highestDecision != "" {
		event.Decision = highestDecision
		event.RuleHit = highestRuleHit
		event.RiskLevel = highestRiskLevel
	}

	contextDecision, contextRule, contextRisk, contextSignals := e.evaluateContextPolicy(traceID, toolName, args, event.SemanticClass)
	if toolPolicyActionSeverity(contextDecision) > toolPolicyActionSeverity(event.Decision) {
		event.Decision = contextDecision
		event.RuleHit = contextRule
		event.RiskLevel = contextRisk
	}
	if len(contextSignals) > 0 {
		event.ContextSignals = contextSignals
	}
	attachToolPolicyMetadata(event)

	e.recordEvent(event)
	return event
}

func (e *ToolPolicyEngine) assessToolSemantics(toolName string, args map[string]interface{}) toolSemanticAssessment {
	if len(args) == 0 {
		return toolSemanticAssessment{}
	}
	e.mu.RLock()
	rules := make([]ToolSemanticRule, len(e.semanticRules))
	copy(rules, e.semanticRules)
	e.mu.RUnlock()
	best := toolSemanticAssessment{}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.ToolPattern != "" && !wildcardMatch(rule.ToolPattern, strings.ToLower(toolName)) {
			continue
		}
		if !e.semanticRuleMatches(rule, args) {
			continue
		}
		if toolPolicyActionSeverity(rule.Action) > toolPolicyActionSeverity(best.Decision) || (toolPolicyActionSeverity(rule.Action) == toolPolicyActionSeverity(best.Decision) && best.RuleHit == "") {
			best = toolSemanticAssessment{Class: rule.Class, Decision: rule.Action, RuleHit: rule.Name, RiskLevel: defaultSemanticRisk(rule)}
		}
	}
	return best
}

func (e *ToolPolicyEngine) evaluateContextPolicy(traceID, toolName string, args map[string]interface{}, semanticClass string) (string, string, string, []string) {
	if traceID == "" || !isEgressLikeTool(toolName, semanticClass) {
		return "", "", "", nil
	}
	e.mu.RLock()
	policies := make([]ToolContextPolicy, len(e.contextPolicies))
	copy(policies, e.contextPolicies)
	e.mu.RUnlock()
	bestDecision, bestRule, bestRisk := "", "", ""
	var bestSignals []string
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		window := policy.WindowSize
		if window <= 0 {
			window = 12
		}
		rows, err := e.db.Query(`SELECT arguments_json FROM tool_call_events WHERE trace_id=? ORDER BY timestamp DESC LIMIT ?`, traceID, window)
		if err != nil {
			continue
		}
		var signals []string
		matched := false
		for rows.Next() {
			var argsJSON string
			if rows.Scan(&argsJSON) != nil {
				continue
			}
			var decoded map[string]interface{}
			if json.Unmarshal([]byte(argsJSON), &decoded) != nil {
				continue
			}
			meta, _ := decoded["_tool_policy_meta"].(map[string]interface{})
			prevClass, _ := meta["semantic_class"].(string)
			if stringInSlice(prevClass, policy.SourceClasses) {
				signals = append(signals, "source:"+prevClass)
				matched = true
			}
		}
		rows.Close()
		if !matched {
			continue
		}
		if len(policy.TargetClasses) > 0 && !stringInSlice(semanticClass, policy.TargetClasses) {
			continue
		}
		if len(policy.TargetTools) > 0 && !matchesAnyToolPattern(strings.ToLower(toolName), policy.TargetTools) {
			continue
		}
		if toolPolicyActionSeverity(policy.Action) > toolPolicyActionSeverity(bestDecision) {
			bestDecision, bestRule, bestRisk = policy.Action, policy.Name, defaultContextRisk(policy)
			bestSignals = uniqueStrings(signals)
		}
	}
	return bestDecision, bestRule, bestRisk, bestSignals
}

func attachToolPolicyMetadata(event *ToolCallEvent) {
	if event == nil {
		return
	}
	if event.Arguments == nil {
		event.Arguments = map[string]interface{}{}
	}
	meta := map[string]interface{}{}
	if event.SemanticClass != "" {
		meta["semantic_class"] = event.SemanticClass
	}
	if len(event.ContextSignals) > 0 {
		meta["context_signals"] = event.ContextSignals
	}
	if len(meta) > 0 {
		event.Arguments["_tool_policy_meta"] = meta
	}
}

func (e *ToolPolicyEngine) semanticRuleMatches(rule ToolSemanticRule, args map[string]interface{}) bool {
	if len(rule.ParamKeys) == 0 {
		return rule.MatchType == "always"
	}
	for _, key := range rule.ParamKeys {
		if v, ok := args[key]; ok && semanticRuleMatchesValue(rule, fmt.Sprintf("%v", v)) {
			return true
		}
	}
	return false
}

func semanticRuleMatchesValue(rule ToolSemanticRule, value string) bool {
	switch strings.ToLower(rule.MatchType) {
	case "always":
		return true
	case "exists":
		return strings.TrimSpace(value) != ""
	default:
		re, err := regexp.Compile(`(?i)` + rule.Pattern)
		return err == nil && re.MatchString(value)
	}
}

func defaultSemanticRisk(rule ToolSemanticRule) string {
	if rule.RiskLevel != "" {
		return rule.RiskLevel
	}
	return classifyRiskFromAction(rule.Action, rule.Priority)
}

func defaultContextRisk(policy ToolContextPolicy) string {
	if policy.RiskLevel != "" {
		return policy.RiskLevel
	}
	return classifyRiskFromAction(policy.Action, policy.Priority)
}

func stringInSlice(target string, values []string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func matchesAnyToolPattern(tool string, patterns []string) bool {
	for _, pattern := range patterns {
		if wildcardMatch(pattern, tool) {
			return true
		}
	}
	return false
}

func matchesAnyRegex(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		re, err := regexp.Compile(`(?i)` + pattern)
		if err == nil && re.MatchString(value) {
			return true
		}
	}
	return false
}

func isEgressLikeTool(toolName, semanticClass string) bool {
	lower := strings.ToLower(toolName)
	if strings.HasPrefix(semanticClass, "url:") || strings.HasPrefix(semanticClass, "message:") {
		return true
	}
	return matchesAnyRegex(lower, `http`, `network`, `send`, `email`, `message`, `webhook`)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
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
				if re.MatchString(strVal) && toolPolicyActionSeverity(pr.Action) > toolPolicyActionSeverity(highestAction) {
					highestAction = pr.Action
				}
			}
		} else if v, ok := args[pr.ParamName]; ok {
			strVal := fmt.Sprintf("%v", v)
			if re.MatchString(strVal) && toolPolicyActionSeverity(pr.Action) > toolPolicyActionSeverity(highestAction) {
				highestAction = pr.Action
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
		event.ID, event.TraceID, event.Timestamp.Format(time.RFC3339), event.ToolName, string(argsJSON), event.Decision, event.RuleHit, event.RiskLevel, event.TenantID)
	if err != nil {
		log.Printf("[ToolPolicy] 记录事件失败: %v", err)
	}
}

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
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
	sort.Slice(e.rules, func(i, j int) bool { return e.rules[i].Priority < e.rules[j].Priority })
	e.compileRegexCache()
	return nil
}

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
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == rule.ID {
			e.rules[i] = rule
			break
		}
	}
	sort.Slice(e.rules, func(i, j int) bool { return e.rules[i].Priority < e.rules[j].Priority })
	e.compileRegexCache()
	return nil
}

func (e *ToolPolicyEngine) RemoveRule(id string) error {
	_, err := e.db.Exec(`DELETE FROM tool_policy_rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			break
		}
	}
	return nil
}

func (e *ToolPolicyEngine) ListRules() []ToolPolicyRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ToolPolicyRule, len(e.rules))
	copy(result, e.rules)
	return result
}

func (e *ToolPolicyEngine) AddSemanticRule(rule ToolSemanticRule) error {
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("ts-custom-%d", time.Now().UnixNano())
	}
	if rule.Name == "" {
		return fmt.Errorf("semantic rule name is required")
	}
	if rule.ToolPattern == "" {
		rule.ToolPattern = "*"
	}
	if rule.MatchType == "" {
		rule.MatchType = "regex"
	}
	if rule.Class == "" {
		return fmt.Errorf("semantic class is required")
	}
	if rule.Action == "" {
		rule.Action = "allow"
	}
	if strings.EqualFold(rule.MatchType, "regex") {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern %q: %v", rule.Pattern, err)
		}
	}
	if err := e.saveSemanticRuleToDB(rule); err != nil {
		return fmt.Errorf("save semantic rule failed: %v", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.semanticRules = append(e.semanticRules, rule)
	sort.Slice(e.semanticRules, func(i, j int) bool { return e.semanticRules[i].Priority < e.semanticRules[j].Priority })
	e.compileRegexCache()
	return nil
}

func (e *ToolPolicyEngine) UpdateSemanticRule(rule ToolSemanticRule) error {
	if rule.ID == "" {
		return fmt.Errorf("semantic rule id is required")
	}
	if rule.MatchType == "" {
		rule.MatchType = "regex"
	}
	if strings.EqualFold(rule.MatchType, "regex") {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern %q: %v", rule.Pattern, err)
		}
	}
	if err := e.saveSemanticRuleToDB(rule); err != nil {
		return fmt.Errorf("save semantic rule failed: %v", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.semanticRules {
		if r.ID == rule.ID {
			e.semanticRules[i] = rule
			break
		}
	}
	sort.Slice(e.semanticRules, func(i, j int) bool { return e.semanticRules[i].Priority < e.semanticRules[j].Priority })
	e.compileRegexCache()
	return nil
}

func (e *ToolPolicyEngine) RemoveSemanticRule(id string) error {
	_, err := e.db.Exec(`DELETE FROM tool_semantic_rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, r := range e.semanticRules {
		if r.ID == id {
			e.semanticRules = append(e.semanticRules[:i], e.semanticRules[i+1:]...)
			break
		}
	}
	e.compileRegexCache()
	return nil
}

func (e *ToolPolicyEngine) ListSemanticRules() []ToolSemanticRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ToolSemanticRule, len(e.semanticRules))
	copy(result, e.semanticRules)
	return result
}

func (e *ToolPolicyEngine) AddContextPolicy(policy ToolContextPolicy) error {
	if policy.ID == "" {
		policy.ID = fmt.Sprintf("tc-custom-%d", time.Now().UnixNano())
	}
	if policy.Name == "" {
		return fmt.Errorf("context policy name is required")
	}
	if policy.Action == "" {
		policy.Action = "warn"
	}
	if policy.WindowSize <= 0 {
		policy.WindowSize = 12
	}
	if err := e.saveContextPolicyToDB(policy); err != nil {
		return fmt.Errorf("save context policy failed: %v", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.contextPolicies = append(e.contextPolicies, policy)
	sort.Slice(e.contextPolicies, func(i, j int) bool { return e.contextPolicies[i].Priority < e.contextPolicies[j].Priority })
	return nil
}

func (e *ToolPolicyEngine) UpdateContextPolicy(policy ToolContextPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("context policy id is required")
	}
	if policy.WindowSize <= 0 {
		policy.WindowSize = 12
	}
	if err := e.saveContextPolicyToDB(policy); err != nil {
		return fmt.Errorf("save context policy failed: %v", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, p := range e.contextPolicies {
		if p.ID == policy.ID {
			e.contextPolicies[i] = policy
			break
		}
	}
	sort.Slice(e.contextPolicies, func(i, j int) bool { return e.contextPolicies[i].Priority < e.contextPolicies[j].Priority })
	return nil
}

func (e *ToolPolicyEngine) RemoveContextPolicy(id string) error {
	_, err := e.db.Exec(`DELETE FROM tool_context_policies WHERE id=?`, id)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, p := range e.contextPolicies {
		if p.ID == id {
			e.contextPolicies = append(e.contextPolicies[:i], e.contextPolicies[i+1:]...)
			break
		}
	}
	return nil
}

func (e *ToolPolicyEngine) ListContextPolicies() []ToolContextPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ToolContextPolicy, len(e.contextPolicies))
	copy(result, e.contextPolicies)
	return result
}

func (e *ToolPolicyEngine) GetConfig() ToolPolicyConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// ClassifyToolRiskByName 仅凭工具名返回风险等级（不写 DB、不评估参数）。
// 只检查无参数条件的策略规则（block→critical, warn→high）；
// 语义规则依赖参数内容，工具名阶段不参与评估。
func (e *ToolPolicyEngine) ClassifyToolRiskByName(toolName string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	name := strings.ToLower(toolName)
	levelRank := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}
	best := "low"
	promote := func(lvl string) {
		if levelRank[lvl] > levelRank[best] {
			best = lvl
		}
	}
	for _, r := range e.rules {
		if !r.Enabled || len(r.ParamRules) > 0 {
			continue // 有参数条件的规则需要实际参数，工具名阶段跳过
		}
		if !wildcardMatch(r.ToolPattern, name) {
			continue
		}
		switch r.Action {
		case "block":
			promote("critical")
		case "warn":
			promote("high")
		}
	}
	return best
}
func (e *ToolPolicyEngine) UpdateConfig(cfg ToolPolicyConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = cfg
}

func (e *ToolPolicyEngine) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_events":         0,
		"by_decision":          map[string]int{"allow": 0, "warn": 0, "block": 0},
		"by_risk":              map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
		"top_tools":            []map[string]interface{}{},
		"top_rules":            []map[string]interface{}{},
		"blocked_24h":          0,
		"warned_24h":           0,
		"total_rules":          len(e.ListRules()),
		"semantic_rule_count":  len(e.ListSemanticRules()),
		"context_policy_count": len(e.ListContextPolicies()),
		"config":               e.GetConfig(),
	}
	var total int
	e.db.QueryRow(`SELECT COUNT(*) FROM tool_call_events`).Scan(&total)
	stats["total_events"] = total
	if rows, err := e.db.Query(`SELECT decision, COUNT(*) FROM tool_call_events GROUP BY decision`); err == nil {
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
	if rows, err := e.db.Query(`SELECT risk_level, COUNT(*) FROM tool_call_events GROUP BY risk_level`); err == nil {
		defer rows.Close()
		byRisk := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
		for rows.Next() {
			var r string
			var c int
			if rows.Scan(&r, &c) == nil {
				byRisk[r] = c
			}
		}
		stats["by_risk"] = byRisk
	}
	if rows, err := e.db.Query(`SELECT tool_name, COUNT(*) as cnt FROM tool_call_events GROUP BY tool_name ORDER BY cnt DESC LIMIT 10`); err == nil {
		defer rows.Close()
		var topTools []map[string]interface{}
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
				topTools = append(topTools, map[string]interface{}{"tool_name": name, "count": cnt})
			}
		}
		if topTools != nil {
			stats["top_tools"] = topTools
		}
	}
	if rows, err := e.db.Query(`SELECT rule_hit, COUNT(*) as cnt FROM tool_call_events WHERE rule_hit != '' GROUP BY rule_hit ORDER BY cnt DESC LIMIT 10`); err == nil {
		defer rows.Close()
		var topRules []map[string]interface{}
		for rows.Next() {
			var name string
			var cnt int
			if rows.Scan(&name, &cnt) == nil {
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

func (e *ToolPolicyEngine) QueryEvents(toolName, decision, risk, semanticClass, contextSignal string, limit, offset int) ([]map[string]interface{}, int, error) {
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
	if semanticClass != "" {
		where += " AND arguments_json LIKE ?"
		args = append(args, "%\"semantic_class\":\""+semanticClass+"\"%")
	}
	if contextSignal != "" {
		where += " AND arguments_json LIKE ?"
		args = append(args, "%"+contextSignal+"%")
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
		record := map[string]interface{}{"id": id, "trace_id": traceID, "timestamp": ts, "tool_name": tool, "arguments": argsJSON, "decision": dec, "rule_hit": ruleHit, "risk_level": riskLvl, "tenant_id": tenant}
		var decoded map[string]interface{}
		if json.Unmarshal([]byte(argsJSON), &decoded) == nil {
			if meta, ok := decoded["_tool_policy_meta"].(map[string]interface{}); ok {
				record["semantic_class"] = meta["semantic_class"]
				record["context_signals"] = meta["context_signals"]
			}
		}
		records = append(records, record)
	}
	return records, total, nil
}

func wildcardMatch(pattern, name string) bool {
	pattern = strings.ToLower(pattern)
	name = strings.ToLower(name)
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
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
			return false
		}
		pos += idx + len(part)
	}
	if parts[len(parts)-1] != "" && pos != len(name) {
		return false
	}
	return true
}

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

func classifyRiskFromAction(action string, priority int) string {
	switch action {
	case "block":
		if priority <= 1 {
			return "critical"
		}
		return "high"
	case "warn":
		return "medium"
	default:
		return "low"
	}
}
