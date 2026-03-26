// plan_compiler.go - PlanCompiler: execution plan compiler (CaMeL)
// lobster-guard v25.0
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PlanConfig configures the plan compiler
type PlanConfig struct {
	Enabled         bool    `json:"enabled" yaml:"enabled"`
	StrictMode      bool    `json:"strict_mode" yaml:"strict_mode"`
	MaxStepsPerPlan int     `json:"max_steps_per_plan" yaml:"max_steps_per_plan"`
	DefaultTimeout  int     `json:"default_timeout_sec" yaml:"default_timeout_sec"`
	AutoComplete    bool    `json:"auto_complete" yaml:"auto_complete"`
	ViolationAction string  `json:"violation_action" yaml:"violation_action"`
	MatchThreshold  float64 `json:"match_threshold" yaml:"match_threshold"`
	MaxActivePlans  int     `json:"max_active_plans" yaml:"max_active_plans"`
	RetentionDays   int     `json:"retention_days" yaml:"retention_days"`
}

// PlanTemplate defines a reusable execution plan template
type PlanTemplate struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	Keywords    []string   `json:"keywords"`
	Steps       []PlanStep `json:"steps"`
	Enabled     bool       `json:"enabled"`
	Priority    int        `json:"priority"`
	Builtin     bool       `json:"builtin"`
	CreatedAt   string     `json:"created_at,omitempty"`
	UpdatedAt   string     `json:"updated_at,omitempty"`
}

// PlanStep defines one step in a plan template
type PlanStep struct {
	Order       int      `json:"order"`
	ToolName    string   `json:"tool_name"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	AllowedArgs []string `json:"allowed_args,omitempty"`
	MaxRetries  int      `json:"max_retries"`
	TimeoutSec  int      `json:"timeout_sec"`
}

// ActivePlan tracks a running plan execution
type ActivePlan struct {
	ID            string          `json:"id"`
	TraceID       string          `json:"trace_id"`
	TemplateID    string          `json:"template_id"`
	TemplateName  string          `json:"template_name"`
	UserQuery     string          `json:"user_query"`
	Status        string          `json:"status"`
	CurrentStep   int             `json:"current_step"`
	TotalSteps    int             `json:"total_steps"`
	ExecutedSteps []ExecutedStep  `json:"executed_steps"`
	Violations    []PlanViolation `json:"violations"`
	Score         float64         `json:"score"`
	StartedAt     string          `json:"started_at"`
	CompletedAt   string          `json:"completed_at,omitempty"`
}

// ExecutedStep records a tool call within a plan
type ExecutedStep struct {
	Order     int    `json:"order"`
	ToolName  string `json:"tool_name"`
	ToolArgs  string `json:"tool_args"`
	Result    string `json:"result"`
	Timestamp string `json:"timestamp"`
	LatencyMs int64  `json:"latency_ms"`
}

// PlanViolation records a deviation from the plan
type PlanViolation struct {
	ID          string `json:"id"`
	TraceID     string `json:"trace_id"`
	PlanID      string `json:"plan_id"`
	StepOrder   int    `json:"step_order"`
	ToolName    string `json:"tool_name"`
	Expected    string `json:"expected"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Timestamp   string `json:"timestamp"`
}

// PlanEvaluation is the result of evaluating a tool call against a plan
type PlanEvaluation struct {
	Allowed    bool           `json:"allowed"`
	Decision   string         `json:"decision"`
	ToolName   string         `json:"tool_name"`
	StepMatch  string         `json:"step_match"`
	Reason     string         `json:"reason"`
	Violation  *PlanViolation `json:"violation,omitempty"`
	PlanID     string         `json:"plan_id"`
	PlanStatus string         `json:"plan_status"`
}

// PlanStats aggregates plan execution statistics
type PlanStats struct {
	TotalPlans        int64                    `json:"total_plans"`
	ActivePlans       int64                    `json:"active_plans"`
	CompletedPlans    int64                    `json:"completed_plans"`
	ViolatedPlans     int64                    `json:"violated_plans"`
	TotalViolations   int64                    `json:"total_violations"`
	TotalEvaluations  int64                    `json:"total_evaluations"`
	AvgScore          float64                  `json:"avg_score"`
	TemplateCount     int                      `json:"template_count"`
	TopTemplates      []map[string]interface{} `json:"top_templates"`
	RecentViolations  []PlanViolation          `json:"recent_violations"`
	CategoryBreakdown map[string]int64         `json:"category_breakdown"`
}

// PlanCompiler is the core engine
type PlanCompiler struct {
	db               *sql.DB
	config           PlanConfig
	mu               sync.RWMutex
	templates        map[string]*PlanTemplate
	activePlans      map[string]*ActivePlan
	totalEvaluations int64
}

func planGenID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func planBoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// NewPlanCompiler creates and initializes a PlanCompiler
func NewPlanCompiler(db *sql.DB, config PlanConfig) *PlanCompiler {
	if config.MaxStepsPerPlan <= 0 {
		config.MaxStepsPerPlan = 20
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 300
	}
	if config.ViolationAction == "" {
		config.ViolationAction = "warn"
	}
	if config.MatchThreshold <= 0 {
		config.MatchThreshold = 0.3
	}
	if config.MaxActivePlans <= 0 {
		config.MaxActivePlans = 1000
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = 30
	}
	pc := &PlanCompiler{
		db: db, config: config,
		templates:   make(map[string]*PlanTemplate),
		activePlans: make(map[string]*ActivePlan),
	}
	pc.initTables()
	pc.loadBuiltinTemplates()
	pc.loadTemplatesFromDB()
	return pc
}

func (pc *PlanCompiler) initTables() {
	stmts := []string{
		"CREATE TABLE IF NOT EXISTS plan_templates (id TEXT PRIMARY KEY, name TEXT NOT NULL, category TEXT DEFAULT '', description TEXT DEFAULT '', keywords TEXT DEFAULT '[]', steps TEXT DEFAULT '[]', enabled INTEGER DEFAULT 1, priority INTEGER DEFAULT 0, builtin INTEGER DEFAULT 0, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE IF NOT EXISTS plan_executions (id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, template_id TEXT DEFAULT '', template_name TEXT DEFAULT '', user_query TEXT DEFAULT '', status TEXT DEFAULT 'active', current_step INTEGER DEFAULT 0, total_steps INTEGER DEFAULT 0, executed_steps TEXT DEFAULT '[]', violations TEXT DEFAULT '[]', score REAL DEFAULT 0, started_at TEXT DEFAULT '', completed_at TEXT DEFAULT '')",
		"CREATE TABLE IF NOT EXISTS plan_violations_log (id TEXT PRIMARY KEY, trace_id TEXT DEFAULT '', plan_id TEXT DEFAULT '', step_order INTEGER DEFAULT 0, tool_name TEXT DEFAULT '', expected TEXT DEFAULT '', severity TEXT DEFAULT 'minor', description TEXT DEFAULT '', action TEXT DEFAULT 'logged', timestamp TEXT DEFAULT '')",
		"CREATE INDEX IF NOT EXISTS idx_pe_trace ON plan_executions(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_pe_status ON plan_executions(status)",
		"CREATE INDEX IF NOT EXISTS idx_pv_trace ON plan_violations_log(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_pv_plan ON plan_violations_log(plan_id)",
	}
	for _, s := range stmts {
		if _, err := pc.db.Exec(s); err != nil {
			log.Printf("[PlanCompiler] init: %v", err)
		}
	}
}

func (pc *PlanCompiler) loadBuiltinTemplates() {
	builtins := getBuiltinPlanTemplates()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range builtins {
		t := &builtins[i]
		t.ID = "builtin_" + t.Name
		t.Enabled = true
		t.Builtin = true
		t.Priority = 100 - i
		t.CreatedAt = now
		t.UpdatedAt = now
		pc.templates[t.ID] = t
		kw, _ := json.Marshal(t.Keywords)
		st, _ := json.Marshal(t.Steps)
		pc.db.Exec("INSERT OR IGNORE INTO plan_templates (id,name,category,description,keywords,steps,enabled,priority,builtin,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,1,?,?)",
			t.ID, t.Name, t.Category, t.Description, string(kw), string(st), planBoolToInt(t.Enabled), t.Priority, now, now)
	}
}

func getBuiltinPlanTemplates() []PlanTemplate {
	return []PlanTemplate{
		{Name: "search_and_summarize", Category: "query", Description: "搜索并总结", Keywords: []string{"search", "find", "lookup", "query", "what is", "tell me about", "搜索", "查找", "帮我找", "搜一下", "查询", "是什么"}, Steps: []PlanStep{{Order: 1, ToolName: "web_search", Required: true}, {Order: 2, ToolName: "summarize", Required: true}}},
		{Name: "lookup_contact", Category: "query", Description: "查找联系人", Keywords: []string{"contact", "phone", "email address", "find person", "联系人", "电话", "邮箱", "找人", "通讯录"}, Steps: []PlanStep{{Order: 1, ToolName: "contacts_search", Required: true}}},
		{Name: "check_weather", Category: "query", Description: "查询天气", Keywords: []string{"weather", "temperature", "forecast", "rain", "天气", "气温", "预报", "下雨", "温度", "查天气"}, Steps: []PlanStep{{Order: 1, ToolName: "weather_api", Required: true}}},
		{Name: "database_readonly", Category: "query", Description: "只读数据库查询", Keywords: []string{"select", "query database", "count", "list records", "查数据库", "查表", "统计", "记录"}, Steps: []PlanStep{{Order: 1, ToolName: "db_query", Required: true}}},
		{Name: "read_email", Category: "email", Description: "查看邮件", Keywords: []string{"read email", "check inbox", "show emails", "unread", "看邮件", "查邮件", "收件箱", "未读", "邮箱"}, Steps: []PlanStep{{Order: 1, ToolName: "email_read", Required: true}}},
		{Name: "send_email_simple", Category: "email", Description: "发送邮件", Keywords: []string{"send email", "compose email", "write email", "mail to", "发邮件", "写邮件", "发送邮件", "邮件发给"}, Steps: []PlanStep{{Order: 1, ToolName: "email_compose", Required: true}, {Order: 2, ToolName: "email_send", Required: true}}},
		{Name: "reply_email", Category: "email", Description: "回复邮件", Keywords: []string{"reply", "respond to email", "answer email", "回复邮件", "回邮件", "答复"}, Steps: []PlanStep{{Order: 1, ToolName: "email_read", Required: true}, {Order: 2, ToolName: "email_compose", Required: true}, {Order: 3, ToolName: "email_send", Required: true}}},
		{Name: "read_file", Category: "file", Description: "读取文件", Keywords: []string{"read file", "open file", "show file", "cat", "view", "看文件", "打开文件", "读取", "查看文件"}, Steps: []PlanStep{{Order: 1, ToolName: "file_read", Required: true}}},
		{Name: "write_report", Category: "file", Description: "撰写报告", Keywords: []string{"write report", "generate report", "create document", "save report", "写报告", "生成报告", "创建文档", "保存报告"}, Steps: []PlanStep{{Order: 1, ToolName: "data_gather", Required: true}, {Order: 2, ToolName: "file_write", Required: true}}},
		{Name: "file_convert", Category: "file", Description: "文件格式转换", Keywords: []string{"convert file", "transform", "export as", "change format", "转换文件", "格式转换", "导出", "转格式"}, Steps: []PlanStep{{Order: 1, ToolName: "file_read", Required: true}, {Order: 2, ToolName: "file_convert", Required: true}, {Order: 3, ToolName: "file_write", Required: true}}},
		{Name: "code_review", Category: "code", Description: "代码审查", Keywords: []string{"review code", "check code", "code quality", "lint", "代码审查", "代码检查", "代码质量", "审代码"}, Steps: []PlanStep{{Order: 1, ToolName: "file_read", Required: true}, {Order: 2, ToolName: "code_analyze", Required: true}}},
		{Name: "run_test", Category: "code", Description: "运行测试", Keywords: []string{"run test", "execute test", "test suite", "unit test", "跑测试", "运行测试", "单元测试", "执行测试"}, Steps: []PlanStep{{Order: 1, ToolName: "shell_exec", Required: true}}},
		{Name: "build_project", Category: "code", Description: "构建项目", Keywords: []string{"build", "compile", "make", "npm build", "go build", "编译", "构建", "打包"}, Steps: []PlanStep{{Order: 1, ToolName: "shell_exec", Required: true}}},
		{Name: "fetch_webpage", Category: "web", Description: "抓取网页", Keywords: []string{"fetch page", "scrape", "get url", "browse", "open url", "抓取网页", "爬取", "打开网址", "访问链接"}, Steps: []PlanStep{{Order: 1, ToolName: "web_fetch", Required: true}}},
		{Name: "api_call", Category: "web", Description: "调用API接口", Keywords: []string{"api call", "http request", "rest api", "curl", "调接口", "调API", "发请求", "HTTP请求"}, Steps: []PlanStep{{Order: 1, ToolName: "http_request", Required: true}}},
		{Name: "download_file", Category: "web", Description: "下载文件", Keywords: []string{"download", "save from url", "fetch file", "下载", "保存文件", "下载文件"}, Steps: []PlanStep{{Order: 1, ToolName: "web_fetch", Required: true}, {Order: 2, ToolName: "file_write", Required: true}}},
		{Name: "create_calendar", Category: "admin", Description: "创建日程", Keywords: []string{"calendar", "schedule", "meeting", "event", "appointment", "日历", "日程", "会议", "排期", "预约"}, Steps: []PlanStep{{Order: 1, ToolName: "calendar_create", Required: true}}},
		{Name: "manage_task", Category: "admin", Description: "管理任务", Keywords: []string{"task", "todo", "ticket", "create task", "assign", "任务", "待办", "工单", "创建任务", "分配"}, Steps: []PlanStep{{Order: 1, ToolName: "task_manage", Required: true}}},
		{Name: "system_status", Category: "admin", Description: "检查系统状态", Keywords: []string{"status", "health", "monitor", "uptime", "disk", "memory", "状态", "健康检查", "监控", "运行时间", "磁盘", "内存"}, Steps: []PlanStep{{Order: 1, ToolName: "system_info", Required: true}}},
		{Name: "deploy_service", Category: "admin", Description: "部署服务", Keywords: []string{"deploy", "release", "rollout", "publish", "部署", "发布", "上线", "发版"}, Steps: []PlanStep{{Order: 1, ToolName: "build_check", Required: true}, {Order: 2, ToolName: "deploy_exec", Required: true}, {Order: 3, ToolName: "health_check", Required: true}}},
	}
}

func (pc *PlanCompiler) loadTemplatesFromDB() {
	rows, err := pc.db.Query("SELECT id,name,category,description,keywords,steps,enabled,priority,builtin,created_at,updated_at FROM plan_templates WHERE builtin=0")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t PlanTemplate
		var kwJ, stJ string
		var eI, bI int
		if err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Description, &kwJ, &stJ, &eI, &t.Priority, &bI, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		t.Enabled = eI == 1
		t.Builtin = bI == 1
		json.Unmarshal([]byte(kwJ), &t.Keywords)
		json.Unmarshal([]byte(stJ), &t.Steps)
		pc.templates[t.ID] = &t
	}
}

// CompileIntent matches user query to a plan template
func (pc *PlanCompiler) CompileIntent(traceID, userQuery string) *ActivePlan {
	if !pc.config.Enabled {
		return nil
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if plan, ok := pc.activePlans[traceID]; ok {
		return plan
	}
	bestTpl, bestScore := pc.matchTemplate(userQuery)
	if bestTpl == nil || bestScore < pc.config.MatchThreshold {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	plan := &ActivePlan{
		ID: planGenID(), TraceID: traceID, TemplateID: bestTpl.ID,
		TemplateName: bestTpl.Name, UserQuery: userQuery, Status: "active",
		CurrentStep: 0, TotalSteps: len(bestTpl.Steps),
		ExecutedSteps: []ExecutedStep{}, Violations: []PlanViolation{},
		Score: bestScore, StartedAt: now,
	}
	if len(pc.activePlans) >= pc.config.MaxActivePlans {
		pc.evictOldestPlan()
	}
	pc.activePlans[traceID] = plan
	esJ, _ := json.Marshal(plan.ExecutedSteps)
	vJ, _ := json.Marshal(plan.Violations)
	pc.db.Exec("INSERT INTO plan_executions (id,trace_id,template_id,template_name,user_query,status,current_step,total_steps,executed_steps,violations,score,started_at,completed_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		plan.ID, plan.TraceID, plan.TemplateID, plan.TemplateName, plan.UserQuery, plan.Status,
		plan.CurrentStep, plan.TotalSteps, string(esJ), string(vJ), plan.Score, plan.StartedAt, "")
	return plan
}

func (pc *PlanCompiler) evictOldestPlan() {
	var oldK, oldT string
	for k, p := range pc.activePlans {
		if oldK == "" || p.StartedAt < oldT {
			oldK = k
			oldT = p.StartedAt
		}
	}
	if oldK != "" {
		if p := pc.activePlans[oldK]; p != nil {
			p.Status = "evicted"
			p.CompletedAt = time.Now().UTC().Format(time.RFC3339)
			pc.persistPlan(p)
		}
		delete(pc.activePlans, oldK)
	}
}

func (pc *PlanCompiler) matchTemplate(query string) (*PlanTemplate, float64) {
	qLow := strings.ToLower(query)
	var best *PlanTemplate
	var bestScore float64
	for _, tpl := range pc.templates {
		if !tpl.Enabled {
			continue
		}
		s := pc.scoreTemplate(tpl, qLow)
		if s > bestScore {
			bestScore = s
			best = tpl
		}
	}
	return best, bestScore
}

func (pc *PlanCompiler) scoreTemplate(tpl *PlanTemplate, qLow string) float64 {
	if len(tpl.Keywords) == 0 {
		return 0
	}
	matched := 0
	maxKWLen := 0
	for _, kw := range tpl.Keywords {
		kwLow := strings.ToLower(kw)
		if keywordMatches(qLow, kwLow) {
			matched++
			if len(kwLow) > maxKWLen {
				maxKWLen = len(kwLow)
			}
		}
	}
	if matched == 0 {
		return 0
	}
	// Hybrid scoring: first match gives 0.3 (meets default threshold),
	// each additional match adds diminishing bonus.
	// Longer keyword matches get a specificity bonus to prefer
	// "天气"→check_weather over "查一下"→search_and_summarize.
	score := 0.3 + float64(matched-1)*0.15
	// Specificity bonus: longer matched keyword = more specific intent
	score += float64(maxKWLen) * 0.005
	score += float64(tpl.Priority) * 0.001
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// keywordMatches checks if keyword matches the query.
// First tries exact substring, then bag-of-words for multi-word keywords
// (e.g. "read file" matches "read the config file").
func keywordMatches(qLow, kwLow string) bool {
	if strings.Contains(qLow, kwLow) {
		return true
	}
	// Bag-of-words fallback for multi-word keywords
	words := strings.Fields(kwLow)
	if len(words) <= 1 {
		return false
	}
	for _, w := range words {
		if !strings.Contains(qLow, w) {
			return false
		}
	}
	return true
}

// EvaluateToolCall checks if a tool call conforms to the active plan
func (pc *PlanCompiler) EvaluateToolCall(traceID, toolName, toolArgs string) *PlanEvaluation {
	atomic.AddInt64(&pc.totalEvaluations, 1)
	if !pc.config.Enabled {
		return &PlanEvaluation{Allowed: true, Decision: "allow", ToolName: toolName, StepMatch: "none", Reason: "disabled"}
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	plan, exists := pc.activePlans[traceID]
	if !exists {
		return &PlanEvaluation{Allowed: true, Decision: "allow", ToolName: toolName, StepMatch: "none", Reason: "no active plan"}
	}
	if plan.Status != "active" {
		return &PlanEvaluation{Allowed: true, Decision: "allow", ToolName: toolName, StepMatch: "none", PlanID: plan.ID, PlanStatus: plan.Status}
	}
	tpl := pc.templates[plan.TemplateID]
	if tpl == nil || plan.CurrentStep >= len(tpl.Steps) {
		return &PlanEvaluation{Allowed: true, Decision: "allow", ToolName: toolName, StepMatch: "none", PlanID: plan.ID, PlanStatus: plan.Status, Reason: "beyond steps"}
	}
	exp := tpl.Steps[plan.CurrentStep]
	now := time.Now().UTC().Format(time.RFC3339)
	sm := "none"
	if toolName == exp.ToolName {
		sm = "exact"
	} else if pc.isPartialMatch(toolName, exp.ToolName) {
		sm = "partial"
	} else {
		for i := plan.CurrentStep + 1; i < len(tpl.Steps); i++ {
			if toolName == tpl.Steps[i].ToolName {
				sm = "out_of_order"
				break
			}
		}
	}
	plan.ExecutedSteps = append(plan.ExecutedSteps, ExecutedStep{
		Order: plan.CurrentStep, ToolName: toolName, ToolArgs: toolArgs, Result: sm, Timestamp: now,
	})
	dec, reason := "allow", ""
	var viol *PlanViolation
	switch sm {
	case "exact":
		plan.CurrentStep++
		reason = fmt.Sprintf("tool %s matches step %d", toolName, plan.CurrentStep)
	case "partial":
		plan.CurrentStep++
		reason = fmt.Sprintf("tool %s partial match %s", toolName, exp.ToolName)
	case "out_of_order":
		viol = &PlanViolation{
			ID: planGenID(), TraceID: traceID, PlanID: plan.ID, StepOrder: plan.CurrentStep,
			ToolName: toolName, Expected: exp.ToolName, Severity: "moderate",
			Description: fmt.Sprintf("out of order: got %s expected %s step %d", toolName, exp.ToolName, plan.CurrentStep+1),
			Action: pc.config.ViolationAction, Timestamp: now,
		}
		dec = pc.config.ViolationAction
		reason = viol.Description
		plan.Violations = append(plan.Violations, *viol)
		pc.persistViolation(viol)
	default:
		sev := "minor"
		if pc.config.StrictMode {
			sev = "critical"
		}
		viol = &PlanViolation{
			ID: planGenID(), TraceID: traceID, PlanID: plan.ID, StepOrder: plan.CurrentStep,
			ToolName: toolName, Expected: exp.ToolName, Severity: sev,
			Description: fmt.Sprintf("unexpected %s expected %s step %d", toolName, exp.ToolName, plan.CurrentStep+1),
			Timestamp: now,
		}
		if pc.config.StrictMode {
			dec = "block"
			viol.Action = "blocked"
		} else {
			dec = pc.config.ViolationAction
			viol.Action = pc.config.ViolationAction
		}
		reason = viol.Description
		plan.Violations = append(plan.Violations, *viol)
		pc.persistViolation(viol)
	}
	if plan.CurrentStep >= plan.TotalSteps && pc.config.AutoComplete {
		plan.Status = "completed"
		plan.CompletedAt = now
	}
	pc.persistPlan(plan)
	return &PlanEvaluation{
		Allowed: dec != "block", Decision: dec, ToolName: toolName,
		StepMatch: sm, Reason: reason, Violation: viol,
		PlanID: plan.ID, PlanStatus: plan.Status,
	}
}

func (pc *PlanCompiler) isPartialMatch(actual, expected string) bool {
	parts := strings.Split(expected, "_")
	return len(parts) > 0 && strings.HasPrefix(actual, parts[0])
}

// CompletePlan marks a plan as completed
func (pc *PlanCompiler) CompletePlan(traceID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	p := pc.activePlans[traceID]
	if p == nil {
		return
	}
	p.Status = "completed"
	p.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	pc.persistPlan(p)
}

// GetActivePlan returns the active plan for a trace
func (pc *PlanCompiler) GetActivePlan(traceID string) *ActivePlan {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.activePlans[traceID]
}

func (pc *PlanCompiler) persistPlan(plan *ActivePlan) {
	esJ, _ := json.Marshal(plan.ExecutedSteps)
	vJ, _ := json.Marshal(plan.Violations)
	pc.db.Exec("UPDATE plan_executions SET status=?,current_step=?,executed_steps=?,violations=?,score=?,completed_at=? WHERE id=?",
		plan.Status, plan.CurrentStep, string(esJ), string(vJ), plan.Score, plan.CompletedAt, plan.ID)
}

func (pc *PlanCompiler) persistViolation(v *PlanViolation) {
	pc.db.Exec("INSERT INTO plan_violations_log (id,trace_id,plan_id,step_order,tool_name,expected,severity,description,action,timestamp) VALUES (?,?,?,?,?,?,?,?,?,?)",
		v.ID, v.TraceID, v.PlanID, v.StepOrder, v.ToolName, v.Expected, v.Severity, v.Description, v.Action, v.Timestamp)
}

// AddTemplate creates a new custom template
func (pc *PlanCompiler) AddTemplate(tpl PlanTemplate) (*PlanTemplate, error) {
	if tpl.Name == "" {
		return nil, fmt.Errorf("name required")
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	tpl.ID = planGenID()
	tpl.Builtin = false
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	if tpl.Steps == nil {
		tpl.Steps = []PlanStep{}
	}
	if tpl.Keywords == nil {
		tpl.Keywords = []string{}
	}
	kw, _ := json.Marshal(tpl.Keywords)
	st, _ := json.Marshal(tpl.Steps)
	if _, err := pc.db.Exec("INSERT INTO plan_templates (id,name,category,description,keywords,steps,enabled,priority,builtin,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,0,?,?)",
		tpl.ID, tpl.Name, tpl.Category, tpl.Description, string(kw), string(st), planBoolToInt(tpl.Enabled), tpl.Priority, now, now); err != nil {
		return nil, err
	}
	pc.templates[tpl.ID] = &tpl
	return &tpl, nil
}

// UpdateTemplate updates an existing template
func (pc *PlanCompiler) UpdateTemplate(id string, upd PlanTemplate) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	t, ok := pc.templates[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if upd.Name != "" {
		t.Name = upd.Name
	}
	if upd.Category != "" {
		t.Category = upd.Category
	}
	if upd.Description != "" {
		t.Description = upd.Description
	}
	if upd.Keywords != nil {
		t.Keywords = upd.Keywords
	}
	if upd.Steps != nil {
		t.Steps = upd.Steps
	}
	t.Enabled = upd.Enabled
	t.Priority = upd.Priority
	t.UpdatedAt = now
	kw, _ := json.Marshal(t.Keywords)
	st, _ := json.Marshal(t.Steps)
	_, err := pc.db.Exec("UPDATE plan_templates SET name=?,category=?,description=?,keywords=?,steps=?,enabled=?,priority=?,updated_at=? WHERE id=?",
		t.Name, t.Category, t.Description, string(kw), string(st), planBoolToInt(t.Enabled), t.Priority, now, id)
	return err
}

// DeleteTemplate removes a custom template
func (pc *PlanCompiler) DeleteTemplate(id string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	t, ok := pc.templates[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	if t.Builtin {
		return fmt.Errorf("cannot delete builtin")
	}
	pc.db.Exec("DELETE FROM plan_templates WHERE id=?", id)
	delete(pc.templates, id)
	return nil
}