// path_policy.go — PathPolicyEngine: path-level policy engine
// lobster-guard v23.0
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

type PathStep struct {
	Timestamp time.Time `json:"timestamp"`
	Stage     string    `json:"stage"`
	Action    string    `json:"action"`
	RiskDelta float64   `json:"risk_delta"`
	Details   string    `json:"details"`
}

type PathContext struct {
	TraceID     string     `json:"trace_id"`
	SessionID   string     `json:"session_id"`
	Steps       []PathStep `json:"steps"`
	TaintLabels []string   `json:"taint_labels"`
	ToolHistory []string   `json:"tool_history"`
	RiskScore   float64    `json:"risk_score"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUpdated time.Time  `json:"last_updated"`
	TenantID    string     `json:"tenant_id"`
}

type PathPolicyRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RuleType    string `json:"rule_type"`
	Conditions  string `json:"conditions"`
	Action      string `json:"action"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	Description string `json:"description"`
	TenantID    string `json:"tenant_id"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type PathDecision struct {
	Decision  string  `json:"decision"`
	RuleID    string  `json:"rule_id"`
	RuleName  string  `json:"rule_name"`
	Reason    string  `json:"reason"`
	RiskScore float64 `json:"risk_score"`
}

type SequenceCondition struct {
	After     string `json:"after"`
	Before    string `json:"before"`
	WindowSec int    `json:"window_sec"`
}

type CumulativeCondition struct {
	Label     string `json:"label"`
	Threshold int    `json:"threshold"`
}

type DegradationCondition struct {
	RiskThreshold float64 `json:"risk_threshold"`
	DegradeTo     string  `json:"degrade_to"`
}

type PathPolicyStats struct {
	ActiveContexts int     `json:"active_contexts"`
	TotalRules     int     `json:"total_rules"`
	EnabledRules   int     `json:"enabled_rules"`
	TotalEvents    int64   `json:"total_events"`
	BlockCount     int64   `json:"block_count"`
	WarnCount      int64   `json:"warn_count"`
	AvgRiskScore   float64 `json:"avg_risk_score"`
}

var defaultRiskWeights = map[string]float64{
	"inbound_message": 0, "web_fetch": 10, "file_read": 5,
	"database_query": 8, "shell_exec": 30, "send_email": 15,
	"http_request": 10, "pii_detected": 20, "credential_detected": 40,
	"honeypot_triggered": 50, "rule_violation": 25,
}

var defaultPathPolicies = []PathPolicyRule{
	{ID: "pp-001", Name: "web_fetch_then_send_email", RuleType: "sequence",
		Conditions: `{"after":"web_fetch","before":"send_email","window_sec":30}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "读取外部网页后30秒内禁止发送邮件"},
	{ID: "pp-002", Name: "web_fetch_then_shell", RuleType: "sequence",
		Conditions: `{"after":"web_fetch","before":"shell_exec","window_sec":60}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "读取外部网页后60秒内禁止执行Shell命令"},
	{ID: "pp-003", Name: "file_read_then_http", RuleType: "sequence",
		Conditions: `{"after":"file_read","before":"http_request","window_sec":30}`,
		Action: "warn", Enabled: true, Priority: 20, TenantID: "default",
		Description: "读取本地文件后30秒内发起HTTP请求时告警"},
	{ID: "pp-004", Name: "pii_exposure_limit", RuleType: "cumulative",
		Conditions: `{"label":"PII-TAINTED","threshold":3}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "单次会话内个人信息暴露次数超过3次时阻断"},
	{ID: "pp-005", Name: "credential_any", RuleType: "cumulative",
		Conditions: `{"label":"CREDENTIAL-TAINTED","threshold":1}`,
		Action: "block", Enabled: true, Priority: 5, TenantID: "default",
		Description: "检测到任何凭据泄露时立即阻断"},
	{ID: "pp-006", Name: "risk_warn_threshold", RuleType: "degradation",
		Conditions: `{"risk_threshold":60,"degrade_to":"warn"}`,
		Action: "warn", Enabled: true, Priority: 30, TenantID: "default",
		Description: "风险评分超过60时降级为告警模式"},
	{ID: "pp-007", Name: "risk_block_threshold", RuleType: "degradation",
		Conditions: `{"risk_threshold":80,"degrade_to":"block"}`,
		Action: "block", Enabled: true, Priority: 20, TenantID: "default",
		Description: "风险评分超过80时阻断所有操作"},
	{ID: "pp-008", Name: "risk_isolate_threshold", RuleType: "degradation",
		Conditions: `{"risk_threshold":95,"degrade_to":"isolate"}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "风险评分超过95时隔离当前会话"},

	// v23.2: AI Act 合规策略模板
	{ID: "pp-009", Name: "ai_act_data_minimization", RuleType: "cumulative",
		Conditions: `{"label":"PII-TAINTED","threshold":5}`,
		Action: "block", Enabled: false, Priority: 15, TenantID: "default",
		Description: "[AI法案] 数据最小化：单次会话个人信息暴露超5个字段时阻断"},
	{ID: "pp-010", Name: "ai_act_high_risk_shell", RuleType: "sequence",
		Conditions: `{"after":"database_query","before":"shell_exec","window_sec":120}`,
		Action: "block", Enabled: false, Priority: 10, TenantID: "default",
		Description: "[AI法案] 高风险AI：数据库查询后120秒内禁止执行Shell"},
	{ID: "pp-011", Name: "ai_act_exfiltration_chain", RuleType: "sequence",
		Conditions: `{"after":"file_read","before":"send_email","window_sec":60}`,
		Action: "block", Enabled: false, Priority: 10, TenantID: "default",
		Description: "[AI法案] 防数据外泄：读取文件后60秒内禁止发送邮件"},
	{ID: "pp-012", Name: "ai_act_credential_zero_tolerance", RuleType: "cumulative",
		Conditions: `{"label":"CREDENTIAL-TAINTED","threshold":1}`,
		Action: "block", Enabled: false, Priority: 5, TenantID: "default",
		Description: "[AI法案] 零容忍：检测到任何凭据泄露立即阻断"},
	{ID: "pp-013", Name: "ai_act_risk_human_review", RuleType: "degradation",
		Conditions: `{"risk_threshold":70,"degrade_to":"warn"}`,
		Action: "warn", Enabled: false, Priority: 25, TenantID: "default",
		Description: "[AI法案] 人工审核：风险评分超过70时要求人工复核"},
	// v27.0: 芯片设计关键词累积检测
	{ID: "pp-014", Name: "chip_design_ip_leak", RuleType: "cumulative",
		Conditions: `{"label":"chip_ip","threshold":2}`,
		Action: "block", Enabled: false, Priority: 5, TenantID: "default",
		Description: "芯片设计IP关键词（指令集/微架构代号/制程节点）出现≥2次时阻断"},
	// v27.0: 芯片代码泄露序列检测（先读取设计文件，再发送到外部）
	{ID: "pp-015", Name: "chip_rtl_exfiltration", RuleType: "sequence",
		Conditions: `{"before":"http_send","after":"file_read","window_sec":300}`,
		Action: "block", Enabled: false, Priority: 5, TenantID: "default",
		Description: "读取文件后5分钟内禁止HTTP外发（防RTL/HDL设计文件泄露）"},
	// v29.0: 全局新增 PathPolicy 规则
	{ID: "pp-016", Name: "database_to_email", RuleType: "sequence",
		Conditions: `{"after":"database_query","before":"send_email","window_sec":60}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "防止数据库内容通过邮件外发"},
	{ID: "pp-017", Name: "multi_tool_data_aggregation", RuleType: "cumulative",
		Conditions: `{"label":"data_aggregation","threshold":5}`,
		Action: "warn", Enabled: true, Priority: 15, TenantID: "default",
		Description: "单次会话内大量数据聚合行为（database_query/file_read/web_fetch/search_email/query_api 累计≥5次）"},
	{ID: "pp-018", Name: "shell_after_file_write", RuleType: "sequence",
		Conditions: `{"after":"file_write","before":"shell_exec","window_sec":30}`,
		Action: "block", Enabled: true, Priority: 10, TenantID: "default",
		Description: "写入文件后执行shell，防代码注入"},
}

// PolicyTemplate 策略模板（v23.2 CRUD）
type PolicyTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`    // compliance / security / industry / custom
	RuleIDs     []string `json:"rule_ids"`
	Enabled     bool     `json:"enabled"`
	BuiltIn     bool     `json:"built_in"`    // 内置模板不可删除
	TenantID    string   `json:"tenant_id"`   // v27.0: 归属租户（空=全局）
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

type PathPolicyEngine struct {
	db             *sql.DB
	mu             sync.RWMutex
	contexts       map[string]*PathContext
	rules          []PathPolicyRule
	templates      []PolicyTemplate
	riskWeights    map[string]float64
	halfLifeSec    float64
	evictAfter     time.Duration
	userProfileEng *UserProfileEngine // v23.1: 攻击者画像联动
}

func NewPathPolicyEngine(db *sql.DB) *PathPolicyEngine {
	e := &PathPolicyEngine{db: db, contexts: make(map[string]*PathContext),
		riskWeights: make(map[string]float64), halfLifeSec: 300, evictAfter: 2 * time.Hour}
	for k, v := range defaultRiskWeights { e.riskWeights[k] = v }
	e.initSchema(); e.loadRules(); e.loadTemplates()
	go e.evictionLoop()
	log.Printf("[PathPolicy] Engine initialized (%d rules, %d templates, %d weights)", len(e.rules), len(e.templates), len(e.riskWeights))
	return e
}

func (e *PathPolicyEngine) initSchema() {
	if e.db == nil { return }
	e.db.Exec(`CREATE TABLE IF NOT EXISTS path_policies (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, rule_type TEXT NOT NULL,
		conditions TEXT NOT NULL, action TEXT NOT NULL DEFAULT 'warn',
		enabled INTEGER NOT NULL DEFAULT 1, priority INTEGER NOT NULL DEFAULT 50,
		description TEXT, tenant_id TEXT NOT NULL DEFAULT 'default',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	e.db.Exec(`CREATE TABLE IF NOT EXISTS path_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT, trace_id TEXT NOT NULL,
		session_id TEXT, rule_id TEXT, rule_name TEXT, risk_score REAL,
		decision TEXT NOT NULL, reason TEXT, path_length INTEGER,
		tenant_id TEXT NOT NULL DEFAULT 'default', created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_path_events_trace ON path_events(trace_id)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_path_events_tenant ON path_events(tenant_id)`)
	// v23.2 CRUD: 策略模板表
	e.db.Exec(`CREATE TABLE IF NOT EXISTS policy_templates (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT,
		category TEXT NOT NULL DEFAULT 'custom', rule_ids TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1, built_in INTEGER NOT NULL DEFAULT 0,
		tenant_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	// v27.0: 补加 tenant_id 列（已有表忽略 duplicate column 错误）
	e.db.Exec(`ALTER TABLE policy_templates ADD COLUMN tenant_id TEXT NOT NULL DEFAULT ''`)
}

func (e *PathPolicyEngine) loadRules() {
	if e.db == nil { e.rules = append([]PathPolicyRule{}, defaultPathPolicies...); return }
	// 始终用 INSERT OR IGNORE 确保新增的默认规则被补入（不覆盖已有配置）
	// 同时用 UPDATE 同步内置规则的描述（中文化等变更）
	for _, r := range defaultPathPolicies {
		e.db.Exec("INSERT OR IGNORE INTO path_policies (id,name,rule_type,conditions,action,enabled,priority,description,tenant_id) VALUES (?,?,?,?,?,?,?,?,?)",
			r.ID, r.Name, r.RuleType, r.Conditions, r.Action, boolToInt(r.Enabled), r.Priority, r.Description, r.TenantID)
		// 同步内置规则描述（仅更新 pp-0xx 原始规则，不动租户绑定的副本）
		e.db.Exec("UPDATE path_policies SET description=? WHERE id=?", r.Description, r.ID)
	}
	rows, err := e.db.Query("SELECT id,name,rule_type,conditions,action,enabled,priority,COALESCE(description,''),COALESCE(tenant_id,'default'),COALESCE(created_at,''),COALESCE(updated_at,'') FROM path_policies ORDER BY priority ASC, id ASC")
	if err != nil { e.rules = append([]PathPolicyRule{}, defaultPathPolicies...); return }
	defer rows.Close()
	var rules []PathPolicyRule
	for rows.Next() {
		var r PathPolicyRule; var en int
		if rows.Scan(&r.ID, &r.Name, &r.RuleType, &r.Conditions, &r.Action, &en, &r.Priority, &r.Description, &r.TenantID, &r.CreatedAt, &r.UpdatedAt) != nil { continue }
		r.Enabled = en != 0; rules = append(rules, r)
	}
	if len(rules) == 0 { rules = append([]PathPolicyRule{}, defaultPathPolicies...) }
	e.rules = rules
}


func (e *PathPolicyEngine) RegisterStep(traceID string, step PathStep) {
	if traceID == "" { return }
	e.mu.Lock(); defer e.mu.Unlock()
	ctx, ok := e.contexts[traceID]
	if !ok {
		ctx = &PathContext{TraceID: traceID, Steps: make([]PathStep, 0, 16),
			TaintLabels: make([]string, 0), ToolHistory: make([]string, 0),
			CreatedAt: time.Now(), LastUpdated: time.Now(), TenantID: "default"}
		e.contexts[traceID] = ctx
	}
	if step.Timestamp.IsZero() { step.Timestamp = time.Now() }
	if step.RiskDelta == 0 { if w, ok := e.riskWeights[step.Action]; ok { step.RiskDelta = w } }
	ctx.Steps = append(ctx.Steps, step)
	ctx.LastUpdated = time.Now()
	if step.Stage == "tool_call" && step.Action != "" { ctx.ToolHistory = append(ctx.ToolHistory, step.Action) }
	ctx.RiskScore = e.decayScore(ctx) + step.RiskDelta
	if ctx.RiskScore < 0 { ctx.RiskScore = 0 }
}

func (e *PathPolicyEngine) GetContext(traceID string) *PathContext {
	e.mu.RLock(); defer e.mu.RUnlock()
	ctx, ok := e.contexts[traceID]; if !ok { return nil }
	cp := *ctx
	cp.Steps = append([]PathStep{}, ctx.Steps...)
	cp.TaintLabels = append([]string{}, ctx.TaintLabels...)
	cp.ToolHistory = append([]string{}, ctx.ToolHistory...)
	cp.RiskScore = e.decayScore(ctx)
	return &cp
}

func (e *PathPolicyEngine) UpdateRiskScore(traceID string, delta float64) {
	e.mu.Lock(); defer e.mu.Unlock()
	ctx, ok := e.contexts[traceID]; if !ok { return }
	ctx.RiskScore = e.decayScore(ctx) + delta
	if ctx.RiskScore < 0 { ctx.RiskScore = 0 }
	ctx.LastUpdated = time.Now()
}

func (e *PathPolicyEngine) AddTaintLabel(traceID, label string) {
	e.mu.Lock(); defer e.mu.Unlock()
	ctx, ok := e.contexts[traceID]; if !ok { return }
	for _, l := range ctx.TaintLabels { if l == label { return } }
	ctx.TaintLabels = append(ctx.TaintLabels, label)
}

func (e *PathPolicyEngine) SetSessionID(traceID, sid string) {
	e.mu.Lock(); defer e.mu.Unlock()
	if ctx, ok := e.contexts[traceID]; ok { ctx.SessionID = sid }
}
func (e *PathPolicyEngine) SetTenantID(traceID, tid string) {
	e.mu.Lock(); defer e.mu.Unlock()
	if ctx, ok := e.contexts[traceID]; ok { ctx.TenantID = tid }
}

func (e *PathPolicyEngine) ListContexts() []PathContext {
	e.mu.RLock(); defer e.mu.RUnlock()
	r := make([]PathContext, 0, len(e.contexts))
	for _, ctx := range e.contexts { c := *ctx; c.RiskScore = e.decayScore(ctx); r = append(r, c) }
	return r
}
func (e *PathPolicyEngine) ContextCount() int { e.mu.RLock(); defer e.mu.RUnlock(); return len(e.contexts) }

func (e *PathPolicyEngine) decayScore(ctx *PathContext) float64 {
	if ctx.RiskScore <= 0 { return 0 }
	el := time.Since(ctx.LastUpdated).Seconds()
	if el <= 0 { return ctx.RiskScore }
	s := ctx.RiskScore * math.Pow(2, -el/e.halfLifeSec)
	if s < 0.01 { return 0 }
	return s
}

// v23.1: 攻击者画像联动
func (e *PathPolicyEngine) SetUserProfileEngine(upe *UserProfileEngine) { e.mu.Lock(); defer e.mu.Unlock(); e.userProfileEng = upe }

// v23.1: 查询用户历史风险分，作为路径上下文的先验起始分
// 用户画像 RiskScore 0-100，映射到路径起始分 0-30（不超过 warn 阈值）
func (e *PathPolicyEngine) userPriorScore(senderID string) float64 {
	if e.userProfileEng == nil || senderID == "" { return 0 }
	profile, err := e.userProfileEng.GetUserProfile(senderID)
	if err != nil || profile == nil { return 0 }
	// 映射: userRiskScore 0-100 → pathPrior 0-30
	// 线性映射，截断在 30（不让先验就超过 warn 阈值）
	prior := float64(profile.RiskScore) * 0.3
	if prior > 30 { prior = 30 }
	return prior
}

// RegisterStepWithSender 带 sender 信息的步骤注册（v23.1: 新会话时注入先验风险分）
func (e *PathPolicyEngine) RegisterStepWithSender(traceID, senderID string, step PathStep) {
	if traceID == "" { return }
	e.mu.Lock()
	_, isNew := e.contexts[traceID]
	e.mu.Unlock()
	// 先注册步骤（会自动创建 context）
	e.RegisterStep(traceID, step)
	// 如果是新会话，注入用户先验风险分
	if !isNew {
		prior := e.userPriorScore(senderID)
		if prior > 0 {
			e.UpdateRiskScore(traceID, prior)
			log.Printf("[PathPolicy] v23.1 先验风险注入: sender=%s prior=%.1f", senderID, prior)
		}
	}
}

func (e *PathPolicyEngine) SetHalfLife(sec float64)              { e.mu.Lock(); defer e.mu.Unlock(); if sec > 0 { e.halfLifeSec = sec } }
func (e *PathPolicyEngine) SetRiskWeight(a string, w float64)    { e.mu.Lock(); defer e.mu.Unlock(); e.riskWeights[a] = w }
func (e *PathPolicyEngine) GetRiskWeight(a string) float64       { e.mu.RLock(); defer e.mu.RUnlock(); return e.riskWeights[a] }

func (e *PathPolicyEngine) Evaluate(traceID, proposed string) PathDecision {
	e.mu.RLock()
	ctx, ok := e.contexts[traceID]
	rules := append([]PathPolicyRule{}, e.rules...)
	score := float64(0)
	tenantID := "default"
	if ok && ctx != nil { score = e.decayScore(ctx); tenantID = ctx.TenantID }
	if tenantID == "" { tenantID = "default" }
	e.mu.RUnlock()
	def := PathDecision{Decision: "allow", RiskScore: score}
	if !ok || ctx == nil { return def }
	var best *PathDecision
	for _, rule := range rules {
		if !rule.Enabled { continue }
		// v27.0: 租户过滤 — 只执行属于当前租户或 default 的规则
		if rule.TenantID != "" && rule.TenantID != "default" && rule.TenantID != tenantID { continue }
		var d *PathDecision
		switch rule.RuleType {
		case "sequence":   d = e.evalSeq(ctx, rule, proposed)
		case "cumulative": d = e.evalCum(ctx, rule)
		case "degradation":d = e.evalDeg(ctx, rule, score)
		}
		if d != nil { d.RiskScore = score; if best == nil || actionSev(d.Decision) > actionSev(best.Decision) { best = d } }
	}
	if best != nil { e.logEvt(traceID, ctx, best); return *best }
	return def
}

func (e *PathPolicyEngine) evalSeq(ctx *PathContext, rule PathPolicyRule, proposed string) *PathDecision {
	var c SequenceCondition
	if json.Unmarshal([]byte(rule.Conditions), &c) != nil || proposed != c.Before { return nil }
	now := time.Now(); w := time.Duration(c.WindowSec) * time.Second
	for i := len(ctx.Steps) - 1; i >= 0; i-- {
		if ctx.Steps[i].Action == c.After && now.Sub(ctx.Steps[i].Timestamp) <= w {
			return &PathDecision{Decision: rule.Action, RuleID: rule.ID, RuleName: rule.Name,
				Reason: fmt.Sprintf("sequence violation: %s after %s within %ds", c.Before, c.After, c.WindowSec)}
		}
	}
	return nil
}

func (e *PathPolicyEngine) evalCum(ctx *PathContext, rule PathPolicyRule) *PathDecision {
	var c CumulativeCondition
	if json.Unmarshal([]byte(rule.Conditions), &c) != nil { return nil }
	n := 0
	for _, l := range ctx.TaintLabels { if l == c.Label { n++ } }
	for _, s := range ctx.Steps { if s.Details == c.Label { n++ } }
	if n >= c.Threshold {
		return &PathDecision{Decision: rule.Action, RuleID: rule.ID, RuleName: rule.Name,
			Reason: fmt.Sprintf("cumulative: %s count %d >= %d", c.Label, n, c.Threshold)}
	}
	return nil
}

func (e *PathPolicyEngine) evalDeg(ctx *PathContext, rule PathPolicyRule, score float64) *PathDecision {
	var c DegradationCondition
	if json.Unmarshal([]byte(rule.Conditions), &c) != nil { return nil }
	if score > c.RiskThreshold {
		d := rule.Action; if c.DegradeTo == "isolate" { d = "isolate" }
		return &PathDecision{Decision: d, RuleID: rule.ID, RuleName: rule.Name,
			Reason: fmt.Sprintf("degradation: %.1f > %.1f => %s", score, c.RiskThreshold, c.DegradeTo)}
	}
	return nil
}

func actionSev(a string) int {
	switch a { case "log": return 1; case "warn": return 2; case "block": return 3; case "isolate": return 4 }; return 0
}

func (e *PathPolicyEngine) logEvt(traceID string, ctx *PathContext, d *PathDecision) {
	if e.db == nil { return }
	tid, sid, pl := "default", "", 0
	if ctx != nil { tid = ctx.TenantID; sid = ctx.SessionID; pl = len(ctx.Steps) }
	if tid == "" { tid = "default" }
	e.db.Exec("INSERT INTO path_events (trace_id,session_id,rule_id,rule_name,risk_score,decision,reason,path_length,tenant_id) VALUES (?,?,?,?,?,?,?,?,?)",
		traceID, sid, d.RuleID, d.RuleName, d.RiskScore, d.Decision, d.Reason, pl, tid)
}

func (e *PathPolicyEngine) QueryEvents(traceID, since, tenantID string, limit int) ([]map[string]interface{}, error) {
	if e.db == nil { return nil, nil }
	if limit <= 0 { limit = 100 }
	q := "SELECT id,trace_id,COALESCE(session_id,''),COALESCE(rule_id,''),COALESCE(rule_name,''),COALESCE(risk_score,0),decision,COALESCE(reason,''),COALESCE(path_length,0),COALESCE(tenant_id,'default'),COALESCE(created_at,'') FROM path_events WHERE 1=1"
	var a []interface{}
	if traceID != "" { q += " AND trace_id=?"; a = append(a, traceID) }
	if since != "" { q += " AND created_at>=?"; a = append(a, since) }
	if tenantID != "" { q += " AND tenant_id=?"; a = append(a, tenantID) }
	q += " ORDER BY id DESC LIMIT ?"; a = append(a, limit)
	rows, err := e.db.Query(q, a...); if err != nil { return nil, err }
	defer rows.Close()
	var res []map[string]interface{}
	for rows.Next() {
		var id int64; var ti, si, ri, rn, dc, re, tn, ca string; var rs float64; var pl int
		if rows.Scan(&id, &ti, &si, &ri, &rn, &rs, &dc, &re, &pl, &tn, &ca) != nil { continue }
		res = append(res, map[string]interface{}{"id": id, "trace_id": ti, "session_id": si,
			"rule_id": ri, "rule_name": rn, "risk_score": rs, "decision": dc,
			"reason": re, "path_length": pl, "tenant_id": tn, "created_at": ca})
	}
	return res, nil
}

func (e *PathPolicyEngine) ListRules() []PathPolicyRule {
	e.mu.RLock(); defer e.mu.RUnlock(); return append([]PathPolicyRule{}, e.rules...)
}
func (e *PathPolicyEngine) ListRulesByTenant(tid string) []PathPolicyRule {
	e.mu.RLock(); defer e.mu.RUnlock()
	var r []PathPolicyRule
	for _, ru := range e.rules { if ru.TenantID == tid || tid == "" { r = append(r, ru) } }
	return r
}
func (e *PathPolicyEngine) GetRule(id string) *PathPolicyRule {
	e.mu.RLock(); defer e.mu.RUnlock()
	for _, r := range e.rules { if r.ID == id { c := r; return &c } }; return nil
}

func (e *PathPolicyEngine) AddRule(rule PathPolicyRule) error {
	if rule.ID == "" || rule.Name == "" { return fmt.Errorf("id and name required") }
	if rule.RuleType == "" { return fmt.Errorf("rule_type required") }
	if rule.TenantID == "" { rule.TenantID = "default" }
	e.mu.Lock(); defer e.mu.Unlock()
	for _, r := range e.rules { if r.ID == rule.ID { return fmt.Errorf("rule %q exists", rule.ID) } }
	if e.db != nil {
		if _, err := e.db.Exec("INSERT INTO path_policies (id,name,rule_type,conditions,action,enabled,priority,description,tenant_id) VALUES (?,?,?,?,?,?,?,?,?)",
			rule.ID, rule.Name, rule.RuleType, rule.Conditions, rule.Action, boolToInt(rule.Enabled), rule.Priority, rule.Description, rule.TenantID); err != nil { return err }
	}
	e.rules = append(e.rules, rule)
	return nil
}

func (e *PathPolicyEngine) UpdateRule(rule PathPolicyRule) error {
	if rule.ID == "" { return fmt.Errorf("id required") }
	e.mu.Lock(); defer e.mu.Unlock()
	idx := -1
	for i, r := range e.rules { if r.ID == rule.ID { idx = i; break } }
	if idx < 0 { return fmt.Errorf("rule %q not found", rule.ID) }
	if rule.Name != "" { e.rules[idx].Name = rule.Name }
	if rule.RuleType != "" { e.rules[idx].RuleType = rule.RuleType }
	if rule.Conditions != "" { e.rules[idx].Conditions = rule.Conditions }
	if rule.Action != "" { e.rules[idx].Action = rule.Action }
	e.rules[idx].Enabled = rule.Enabled
	if rule.Priority > 0 { e.rules[idx].Priority = rule.Priority }
	if rule.Description != "" { e.rules[idx].Description = rule.Description }
	if e.db != nil {
		e.db.Exec("UPDATE path_policies SET name=?,rule_type=?,conditions=?,action=?,enabled=?,priority=?,description=?,updated_at=CURRENT_TIMESTAMP WHERE id=?",
			e.rules[idx].Name, e.rules[idx].RuleType, e.rules[idx].Conditions, e.rules[idx].Action, boolToInt(e.rules[idx].Enabled), e.rules[idx].Priority, e.rules[idx].Description, rule.ID)
	}
	return nil
}

func (e *PathPolicyEngine) DeleteRule(id string) error {
	e.mu.Lock(); defer e.mu.Unlock()
	n := make([]PathPolicyRule, 0, len(e.rules)); found := false
	for _, r := range e.rules { if r.ID == id { found = true; continue }; n = append(n, r) }
	if !found { return fmt.Errorf("rule %q not found", id) }
	e.rules = n
	if e.db != nil { e.db.Exec("DELETE FROM path_policies WHERE id=?", id) }
	return nil
}

func (e *PathPolicyEngine) SetRuleEnabled(id string, en bool) error {
	e.mu.Lock(); defer e.mu.Unlock()
	for i, r := range e.rules {
		if r.ID == id {
			e.rules[i].Enabled = en
			if e.db != nil { e.db.Exec("UPDATE path_policies SET enabled=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", boolToInt(en), id) }
			return nil
		}
	}
	return fmt.Errorf("rule %q not found", id)
}

func (e *PathPolicyEngine) Stats() PathPolicyStats {
	e.mu.RLock()
	ac := len(e.contexts); tr := len(e.rules); er := 0
	for _, r := range e.rules { if r.Enabled { er++ } }
	ts := float64(0)
	for _, ctx := range e.contexts { ts += e.decayScore(ctx) }
	avg := float64(0); if ac > 0 { avg = ts / float64(ac) }
	e.mu.RUnlock()
	var te, bc, wc int64
	if e.db != nil {
		e.db.QueryRow("SELECT COUNT(*) FROM path_events").Scan(&te)
		e.db.QueryRow("SELECT COUNT(*) FROM path_events WHERE decision='block'").Scan(&bc)
		e.db.QueryRow("SELECT COUNT(*) FROM path_events WHERE decision='warn'").Scan(&wc)
	}
	return PathPolicyStats{ActiveContexts: ac, TotalRules: tr, EnabledRules: er,
		TotalEvents: te, BlockCount: bc, WarnCount: wc, AvgRiskScore: avg}
}

func (e *PathPolicyEngine) evictionLoop() {
	t := time.NewTicker(5 * time.Minute); defer t.Stop()
	for range t.C {
		e.mu.Lock()
		now := time.Now()
		for k, ctx := range e.contexts {
			if now.Sub(ctx.LastUpdated) > e.evictAfter { delete(e.contexts, k) }
		}
		e.mu.Unlock()
	}
}

func (e *PathPolicyEngine) EvictExpired() int {
	e.mu.Lock(); defer e.mu.Unlock()
	now := time.Now(); evicted := 0
	for k, ctx := range e.contexts {
		if now.Sub(ctx.LastUpdated) > e.evictAfter { delete(e.contexts, k); evicted++ }
	}
	return evicted
}

func (e *PathPolicyEngine) SetEvictAfter(d time.Duration) {
	e.mu.Lock(); defer e.mu.Unlock(); e.evictAfter = d
}

// ============================================================
// v23.2 CRUD: 策略模板管理
// ============================================================

// 8 个内置模板，覆盖合规/安全/行业/运维场景
var defaultTemplates = []PolicyTemplate{
	{ID: "tpl-ai-act", Name: "AI法案合规", Category: "compliance",
		Description: "EU AI法案：数据最小化、人工审核、凭据零容忍、防数据外泄、高风险系统管控",
		RuleIDs: []string{"pp-009", "pp-010", "pp-011", "pp-012", "pp-013"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-strict", Name: "严格安全", Category: "security",
		Description: "最高安全等级：启用全部核心规则，激进风险阈值",
		RuleIDs: []string{"pp-001", "pp-002", "pp-003", "pp-004", "pp-005", "pp-006", "pp-007", "pp-008"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-monitor", Name: "仅监控", Category: "security",
		Description: "只观察不阻断——记录所有违规，适用于初始部署或审计期",
		RuleIDs: []string{"pp-006"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-finance", Name: "银行/支付", Category: "industry",
		Description: "对标SOX/PCI-DSS：银行/支付行业凭据零容忍、严格个人信息限制、阻断数据库到外部的数据流",
		RuleIDs: []string{"pp-004", "pp-005", "pp-010", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-healthcare", Name: "医疗健康 / HIPAA", Category: "industry",
		Description: "对标HIPAA：激进个人信息保护（阈值2）、低风险分即人工审核、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-012", "pp-013"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-devops", Name: "DevOps / CI-CD", Category: "industry",
		Description: "保护CI/CD流水线：外部网页后禁止Shell、防凭据泄露、监控文件到HTTP链路",
		RuleIDs: []string{"pp-001", "pp-002", "pp-003", "pp-005", "pp-007"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-zero-trust", Name: "零信任Agent", Category: "security",
		Description: "默认不信任：启用全部序列规则和累积规则，风险60即降级",
		RuleIDs: []string{"pp-001", "pp-002", "pp-003", "pp-004", "pp-005", "pp-006", "pp-007", "pp-008", "pp-009", "pp-010", "pp-011", "pp-012", "pp-013"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-minimal", Name: "最小防护", Category: "security",
		Description: "仅基本防护：阻断凭据泄露和外部数据读取后的Shell执行",
		RuleIDs: []string{"pp-002", "pp-005"}, Enabled: true, BuiltIn: true},
	// v27.0: 芯片行业模板
	{ID: "tpl-semiconductor", Name: "芯片/半导体行业", Category: "industry",
		Description: "保护芯片设计IP：指令集名称、微架构代号、RTL/HDL代码、代工/制程信息、EDA工具配置",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-012", "pp-014", "pp-015"}, Enabled: true, BuiltIn: true},
	// v29.0: 8个新行业 PathPolicy 模板
	{ID: "tpl-legal", Name: "法律行业", Category: "industry",
		Description: "法律行业策略：PII保护、凭据零容忍、防数据外泄、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-education", Name: "教育行业", Category: "industry",
		Description: "教育行业策略：学生PII保护、数据最小化、凭据零容忍",
		RuleIDs: []string{"pp-004", "pp-009", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-government", Name: "政务/政府", Category: "industry",
		Description: "政务行业策略：PII保护、凭据零容忍、数据最小化、防外泄、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-energy", Name: "能源/电力", Category: "industry",
		Description: "能源行业策略：防网页到邮件外泄、防网页到Shell、凭据零容忍、风险阻断、风险隔离",
		RuleIDs: []string{"pp-001", "pp-002", "pp-005", "pp-007", "pp-008"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-automotive", Name: "汽车/自动驾驶", Category: "industry",
		Description: "汽车行业策略：防网页到Shell、凭据零容忍、风险阻断、防文件到邮件外泄",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-ecommerce", Name: "电商/零售", Category: "industry",
		Description: "电商行业策略：防文件到HTTP外发、PII保护、防文件到邮件外泄",
		RuleIDs: []string{"pp-003", "pp-004", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-hr", Name: "人力资源", Category: "industry",
		Description: "人力资源策略：PII保护、凭据零容忍、数据最小化、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-insurance", Name: "保险行业", Category: "industry",
		Description: "保险行业策略：PII保护、凭据零容忍、防文件到邮件外泄、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	// ========== 第二批行业 PathPolicy 模板（28个） ==========
	{ID: "tpl-securities", Name: "证券/投行", Category: "industry",
		Description: "证券/投行策略：PII保护、凭据零容忍、防数据外泄、防数据库到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-016"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-fund", Name: "基金/资管", Category: "industry",
		Description: "基金/资管策略：PII保护、凭据零容忍、防文件到邮件、数据最小化",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-pharma", Name: "制药/生物科技", Category: "industry",
		Description: "制药策略：PII保护、凭据零容忍、数据最小化、防数据外泄",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-robotics", Name: "机器人/自动化", Category: "industry",
		Description: "机器人策略：防网页到Shell、凭据零容忍、风险阻断",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-consumer-electronics", Name: "消费电子/家电", Category: "industry",
		Description: "消费电子策略：防文件到HTTP、PII保护、防文件到邮件",
		RuleIDs: []string{"pp-003", "pp-004", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-heavy-industry", Name: "重工/装备制造", Category: "industry",
		Description: "重工策略：防网页到Shell、凭据零容忍、风险阻断、防文件到邮件",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-civil-aviation", Name: "民航", Category: "industry",
		Description: "民航策略：防网页到Shell、凭据零容忍、风险阻断、风险隔离",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-008"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-railway", Name: "铁路/高铁", Category: "industry",
		Description: "铁路策略：防网页到Shell、凭据零容忍、风险阻断、风险隔离",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-008"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-metro", Name: "城市轨道/地铁", Category: "industry",
		Description: "地铁策略：防网页到Shell、凭据零容忍、风险阻断",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-maritime", Name: "航运/港口", Category: "industry",
		Description: "航运策略：PII保护、凭据零容忍、防文件到邮件、防数据库到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-016"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-gaming", Name: "游戏", Category: "industry",
		Description: "游戏策略：防网页到Shell、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-002", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-advertising", Name: "广告/营销", Category: "industry",
		Description: "广告策略：PII保护、防文件到HTTP、数据聚合监控",
		RuleIDs: []string{"pp-003", "pp-004", "pp-017"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-social-media", Name: "社交平台", Category: "industry",
		Description: "社交策略：PII保护、凭据零容忍、数据最小化、防文件到邮件、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-live-streaming", Name: "短视频/直播", Category: "industry",
		Description: "直播策略：PII保护、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-saas-cloud", Name: "SaaS/云服务", Category: "industry",
		Description: "SaaS策略：凭据零容忍、防网页到Shell、风险阻断、数据聚合监控、写后执行阻断",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-017", "pp-018"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-search-engine", Name: "搜索引擎", Category: "industry",
		Description: "搜索引擎策略：凭据零容忍、防网页到Shell、防文件到邮件",
		RuleIDs: []string{"pp-002", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-local-services", Name: "外卖/本地生活", Category: "industry",
		Description: "本地生活策略：PII保护、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-cybersecurity", Name: "网络安全", Category: "industry",
		Description: "网络安全策略：凭据零容忍、防网页到Shell、风险阻断、风险隔离、写后执行阻断",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-008", "pp-018"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-media-news", Name: "传媒/新闻", Category: "industry",
		Description: "传媒策略：PII保护、凭据零容忍、防文件到邮件、凭据锁定",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-012"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-publishing", Name: "出版/版权", Category: "industry",
		Description: "出版策略：PII保护、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-telecom", Name: "电信/运营商", Category: "industry",
		Description: "电信策略：凭据零容忍、防网页到Shell、风险阻断、风险隔离、防数据库到邮件",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-008", "pp-016"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-logistics", Name: "物流/供应链", Category: "industry",
		Description: "物流策略：PII保护、凭据零容忍、防文件到邮件、防数据库到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-016"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-real-estate", Name: "房地产/物业", Category: "industry",
		Description: "房地产策略：PII保护、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-agriculture", Name: "农业/食品", Category: "industry",
		Description: "农业策略：PII保护、凭据零容忍、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-aerospace", Name: "航空航天", Category: "industry",
		Description: "航空航天策略：凭据零容忍、防网页到Shell、风险阻断、风险隔离、防RTL外泄",
		RuleIDs: []string{"pp-002", "pp-005", "pp-007", "pp-008", "pp-015"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-mining", Name: "矿业/资源", Category: "industry",
		Description: "矿业策略：凭据零容忍、防文件到邮件、防数据库到邮件、数据聚合监控",
		RuleIDs: []string{"pp-005", "pp-011", "pp-016", "pp-017"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-construction", Name: "建筑/工程", Category: "industry",
		Description: "建筑策略：PII保护、凭据零容忍、防文件到邮件、防数据库到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-011", "pp-016"}, Enabled: true, BuiltIn: true},
	{ID: "tpl-hospitality", Name: "酒店/旅游", Category: "industry",
		Description: "酒店策略：PII保护、凭据零容忍、数据最小化、防文件到邮件",
		RuleIDs: []string{"pp-004", "pp-005", "pp-009", "pp-011"}, Enabled: true, BuiltIn: true},
}

func (e *PathPolicyEngine) loadTemplates() {
	if e.db == nil {
		e.templates = append([]PolicyTemplate{}, defaultTemplates...)
		return
	}
	// 补入内置模板，同步名称和描述
	for _, t := range defaultTemplates {
		rids, _ := json.Marshal(t.RuleIDs)
		e.db.Exec("INSERT OR IGNORE INTO policy_templates (id,name,description,category,rule_ids,enabled,built_in,tenant_id) VALUES (?,?,?,?,?,?,?,?)",
			t.ID, t.Name, t.Description, t.Category, string(rids), boolToInt(t.Enabled), boolToInt(t.BuiltIn), t.TenantID)
		// 同步内置模板的名称和描述
		e.db.Exec("UPDATE policy_templates SET name=?, description=? WHERE id=? AND built_in=1", t.Name, t.Description, t.ID)
	}
	rows, err := e.db.Query("SELECT id,name,COALESCE(description,''),COALESCE(category,'custom'),rule_ids,enabled,built_in,COALESCE(tenant_id,''),COALESCE(created_at,''),COALESCE(updated_at,'') FROM policy_templates ORDER BY built_in DESC, id ASC")
	if err != nil {
		e.templates = append([]PolicyTemplate{}, defaultTemplates...)
		return
	}
	defer rows.Close()
	var templates []PolicyTemplate
	for rows.Next() {
		var t PolicyTemplate
		var ridsJSON string
		var en, bi int
		if rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &ridsJSON, &en, &bi, &t.TenantID, &t.CreatedAt, &t.UpdatedAt) != nil {
			continue
		}
		t.Enabled = en != 0
		t.BuiltIn = bi != 0
		json.Unmarshal([]byte(ridsJSON), &t.RuleIDs)
		if t.RuleIDs == nil { t.RuleIDs = []string{} }
		templates = append(templates, t)
	}
	if len(templates) == 0 {
		templates = append([]PolicyTemplate{}, defaultTemplates...)
	}
	e.templates = templates
}

func (e *PathPolicyEngine) ListTemplates() []PolicyTemplate {
	e.mu.RLock(); defer e.mu.RUnlock()
	return append([]PolicyTemplate{}, e.templates...)
}

func (e *PathPolicyEngine) GetTemplate(id string) *PolicyTemplate {
	e.mu.RLock(); defer e.mu.RUnlock()
	for _, t := range e.templates {
		if t.ID == id { c := t; return &c }
	}
	return nil
}

func (e *PathPolicyEngine) AddTemplate(t PolicyTemplate) error {
	if t.ID == "" || t.Name == "" { return fmt.Errorf("id and name required") }
	if t.Category == "" { t.Category = "custom" }
	if t.RuleIDs == nil { t.RuleIDs = []string{} }
	e.mu.Lock(); defer e.mu.Unlock()
	for _, ex := range e.templates {
		if ex.ID == t.ID { return fmt.Errorf("template %q already exists", t.ID) }
	}
	if e.db != nil {
		rids, _ := json.Marshal(t.RuleIDs)
		if _, err := e.db.Exec("INSERT INTO policy_templates (id,name,description,category,rule_ids,enabled,built_in,tenant_id) VALUES (?,?,?,?,?,?,?,?)",
			t.ID, t.Name, t.Description, t.Category, string(rids), boolToInt(t.Enabled), 0, t.TenantID); err != nil {
			return err
		}
	}
	e.templates = append(e.templates, t)
	return nil
}

func (e *PathPolicyEngine) UpdateTemplate(t PolicyTemplate) error {
	if t.ID == "" { return fmt.Errorf("id required") }
	e.mu.Lock(); defer e.mu.Unlock()
	idx := -1
	for i, ex := range e.templates {
		if ex.ID == t.ID { idx = i; break }
	}
	if idx < 0 { return fmt.Errorf("template %q not found", t.ID) }
	if t.Name != "" { e.templates[idx].Name = t.Name }
	if t.Description != "" { e.templates[idx].Description = t.Description }
	if t.Category != "" { e.templates[idx].Category = t.Category }
	if t.RuleIDs != nil { e.templates[idx].RuleIDs = t.RuleIDs }
	e.templates[idx].Enabled = t.Enabled
	if e.db != nil {
		rids, _ := json.Marshal(e.templates[idx].RuleIDs)
		e.db.Exec("UPDATE policy_templates SET name=?,description=?,category=?,rule_ids=?,enabled=?,updated_at=CURRENT_TIMESTAMP WHERE id=?",
			e.templates[idx].Name, e.templates[idx].Description, e.templates[idx].Category, string(rids), boolToInt(e.templates[idx].Enabled), t.ID)
	}
	return nil
}

func (e *PathPolicyEngine) DeleteTemplate(id string) error {
	e.mu.Lock(); defer e.mu.Unlock()
	n := make([]PolicyTemplate, 0, len(e.templates))
	found := false
	for _, t := range e.templates {
		if t.ID == id {
			if t.BuiltIn { return fmt.Errorf("cannot delete built-in template %q", id) }
			found = true
			continue
		}
		n = append(n, t)
	}
	if !found { return fmt.Errorf("template %q not found", id) }
	e.templates = n
	if e.db != nil { e.db.Exec("DELETE FROM policy_templates WHERE id=? AND built_in=0", id) }
	return nil
}

func (e *PathPolicyEngine) SetTemplateEnabled(id string, en bool) error {
	e.mu.Lock(); defer e.mu.Unlock()
	for i, t := range e.templates {
		if t.ID == id {
			e.templates[i].Enabled = en
			if e.db != nil { e.db.Exec("UPDATE policy_templates SET enabled=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", boolToInt(en), id) }
			return nil
		}
	}
	return fmt.Errorf("template %q not found", id)
}

// ActivateTemplate 激活模板中所有规则
func (e *PathPolicyEngine) ActivateTemplate(id string) (int, error) {
	t := e.GetTemplate(id)
	if t == nil { return 0, fmt.Errorf("template %q not found", id) }
	activated := 0
	for _, rid := range t.RuleIDs {
		if err := e.SetRuleEnabled(rid, true); err == nil { activated++ }
	}
	return activated, nil
}

// DeactivateTemplate 停用模板中所有规则
func (e *PathPolicyEngine) DeactivateTemplate(id string) (int, error) {
	t := e.GetTemplate(id)
	if t == nil { return 0, fmt.Errorf("template %q not found", id) }
	deactivated := 0
	for _, rid := range t.RuleIDs {
		if err := e.SetRuleEnabled(rid, false); err == nil { deactivated++ }
	}
	return deactivated, nil
}

// BindTemplateToTenant 将模板绑定到租户：复制模板中的规则，TenantID 改为目标租户（v27.0）
func (e *PathPolicyEngine) BindTemplateToTenant(templateID, tenantID string) (int, error) {
	t := e.GetTemplate(templateID)
	if t == nil {
		return 0, fmt.Errorf("template %q not found", templateID)
	}
	if tenantID == "" {
		return 0, fmt.Errorf("tenant_id 不能为空")
	}
	bound := 0
	for _, rid := range t.RuleIDs {
		src := e.GetRule(rid)
		if src == nil {
			continue
		}
		// 生成新 ID：原ID + "-" + 租户ID
		newID := rid + "-" + tenantID
		// 检查是否已存在
		if existing := e.GetRule(newID); existing != nil {
			bound++ // 已绑定过
			continue
		}
		newRule := PathPolicyRule{
			ID:          newID,
			Name:        src.Name,
			RuleType:    src.RuleType,
			Conditions:  src.Conditions,
			Action:      src.Action,
			Enabled:     true,
			Priority:    src.Priority,
			Description: src.Description + " [bound to " + tenantID + "]",
			TenantID:    tenantID,
		}
		if err := e.AddRule(newRule); err == nil {
			bound++
		}
	}
	log.Printf("[PathPolicy] 模板 %s 绑定到租户 %s: %d/%d 规则", templateID, tenantID, bound, len(t.RuleIDs))
	return bound, nil
}

// ListRulesForTenant 返回租户可见的所有规则（租户专属 + default）
func (e *PathPolicyEngine) ListRulesForTenant(tenantID string) []PathPolicyRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []PathPolicyRule
	for _, r := range e.rules {
		if r.TenantID == tenantID || r.TenantID == "default" || r.TenantID == "" {
			result = append(result, r)
		}
	}
	return result
}
