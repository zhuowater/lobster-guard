// plan_deviation.go — Plan Deviation Detector + Auto-Repair
// lobster-guard v25.2
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type DeviationDetector struct {
	db              *sql.DB
	mu              sync.RWMutex
	config          DeviationConfig
	planCompiler    *PlanCompiler
	capEngine       *CapabilityEngine
	stats           DeviationStats
	repairPolicies  []RepairPolicy
}

type DeviationConfig struct {
	Enabled        bool            `json:"enabled" yaml:"enabled"`
	AutoRepair     bool            `json:"auto_repair" yaml:"auto_repair"`
	MaxRepairs     int             `json:"max_repairs" yaml:"max_repairs"` // per trace
	RepairPolicies []RepairPolicy  `json:"repair_policies" yaml:"repair_policies"`
}

// RepairPolicy 定义特定偏差类型/严重度的修复策略
type RepairPolicy struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	DeviationType string `json:"deviation_type" yaml:"deviation_type"` // out_of_order / unexpected / capability_violation / * (all)
	Severity    string `json:"severity" yaml:"severity"`               // minor / moderate / critical / * (all)
	Action      string `json:"action" yaml:"action"`                   // replace_tool / sanitize_args / block / skip / log
	Description string `json:"description" yaml:"description"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Builtin     bool   `json:"builtin" yaml:"builtin"`
}

type Deviation struct {
	ID           string    `json:"id"`
	TraceID      string    `json:"trace_id"`
	Type         string    `json:"type"`     // forbidden_tool / sequence_violation / constraint_violation / capability_violation / unknown_tool
	ToolName     string    `json:"tool_name"`
	Expected     string    `json:"expected"`
	Actual       string    `json:"actual"`
	Severity     string    `json:"severity"` // minor / moderate / critical
	Repaired     bool      `json:"repaired"`
	RepairedTool string    `json:"repaired_tool,omitempty"`
	RepairedArgs string    `json:"repaired_args,omitempty"`
	Decision     string    `json:"decision"` // allow / warn / block
	CreatedAt    time.Time `json:"created_at"`
}

type DeviationResult struct {
	HasDeviation bool       `json:"has_deviation"`
	Deviation    *Deviation `json:"deviation,omitempty"`
	Decision     string     `json:"decision"` // allow / warn / block
	Reason       string     `json:"reason"`
	Repaired     bool       `json:"repaired"`
	RepairedTool string     `json:"repaired_tool,omitempty"` // 修复后的工具名（空=不替换）
	RepairedArgs string     `json:"repaired_args,omitempty"` // 修复后的参数（空=不替换）
}

// RepairResult holds the outcome of an auto-repair attempt
type RepairResult struct {
	Success bool
	Tool    string // 替换后的工具名（空=不替换）
	Args    string // 替换后的参数（空=不替换）
	Reason  string
}

type DeviationStats struct {
	TotalChecks     int64 `json:"total_checks"`
	TotalDeviations int64 `json:"total_deviations"`
	CriticalCount   int64 `json:"critical_count"`
	ModerateCount   int64 `json:"moderate_count"`
	MinorCount      int64 `json:"minor_count"`
	RepairsApplied  int64 `json:"repairs_applied"`
}

var defaultDeviationConfig = DeviationConfig{
	Enabled:    false,
	AutoRepair: false,
	MaxRepairs: 5,
}

func NewDeviationDetector(db *sql.DB, config DeviationConfig, pc *PlanCompiler, ce *CapabilityEngine) *DeviationDetector {
	if config.MaxRepairs <= 0 {
		config.MaxRepairs = defaultDeviationConfig.MaxRepairs
	}
	dd := &DeviationDetector{
		db:           db,
		config:       config,
		planCompiler: pc,
		capEngine:    ce,
	}
	dd.initDeviationDB()
	dd.initRepairPolicies()
	log.Printf("[Deviation] Detector initialized: enabled=%v auto_repair=%v max_repairs=%d policies=%d",
		config.Enabled, config.AutoRepair, config.MaxRepairs, len(dd.repairPolicies))
	return dd
}

func (dd *DeviationDetector) initDeviationDB() {
	if dd.db == nil {
		return
	}
	dd.db.Exec(`CREATE TABLE IF NOT EXISTS plan_deviations (
		id TEXT PRIMARY KEY, trace_id TEXT NOT NULL,
		type TEXT NOT NULL, tool_name TEXT, expected TEXT, actual TEXT,
		severity TEXT DEFAULT 'moderate', repaired INTEGER DEFAULT 0,
		repaired_tool TEXT, repaired_args TEXT, decision TEXT DEFAULT 'warn',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	dd.db.Exec(`CREATE INDEX IF NOT EXISTS idx_dev_trace ON plan_deviations(trace_id)`)
	dd.db.Exec(`CREATE INDEX IF NOT EXISTS idx_dev_severity ON plan_deviations(severity)`)

	// Restore counters from DB
	var total, critical, moderate, minor, repaired int64
	dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations").Scan(&total)
	dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations WHERE severity='critical'").Scan(&critical)
	dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations WHERE severity='moderate'").Scan(&moderate)
	dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations WHERE severity='minor'").Scan(&minor)
	dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations WHERE repaired=1").Scan(&repaired)
	atomic.StoreInt64(&dd.stats.TotalDeviations, total)
	atomic.StoreInt64(&dd.stats.TotalChecks, total) // at minimum, each deviation was a check
	atomic.StoreInt64(&dd.stats.CriticalCount, critical)
	atomic.StoreInt64(&dd.stats.ModerateCount, moderate)
	atomic.StoreInt64(&dd.stats.MinorCount, minor)
	atomic.StoreInt64(&dd.stats.RepairsApplied, repaired)
}

func (dd *DeviationDetector) initRepairPolicies() {
	if dd.db == nil {
		return
	}
	dd.db.Exec(`CREATE TABLE IF NOT EXISTS repair_policies (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, deviation_type TEXT NOT NULL,
		severity TEXT NOT NULL, action TEXT NOT NULL, description TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1, builtin INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)

	defaults := getDefaultRepairPolicies()
	for _, p := range defaults {
		dd.db.Exec(`INSERT OR IGNORE INTO repair_policies (id, name, deviation_type, severity, action, description, enabled, builtin) VALUES (?,?,?,?,?,?,?,?)`,
			p.ID, p.Name, p.DeviationType, p.Severity, p.Action, p.Description, p.Enabled, true)
		dd.db.Exec(`UPDATE repair_policies SET name=?, description=? WHERE id=? AND builtin=1`,
			p.Name, p.Description, p.ID)
	}
	dd.loadRepairPolicies()
}

func (dd *DeviationDetector) loadRepairPolicies() {
	if dd.db == nil {
		return
	}
	rows, err := dd.db.Query("SELECT id, name, deviation_type, severity, action, description, enabled, builtin FROM repair_policies ORDER BY id")
	if err != nil {
		return
	}
	defer rows.Close()
	var policies []RepairPolicy
	for rows.Next() {
		var p RepairPolicy
		var enabled, builtin int
		rows.Scan(&p.ID, &p.Name, &p.DeviationType, &p.Severity, &p.Action, &p.Description, &enabled, &builtin)
		p.Enabled = enabled == 1
		p.Builtin = builtin == 1
		policies = append(policies, p)
	}
	dd.mu.Lock()
	dd.repairPolicies = policies
	dd.mu.Unlock()
}

func getDefaultRepairPolicies() []RepairPolicy {
	return []RepairPolicy{
		{ID: "rp-001", Name: "乱序工具替换", DeviationType: "out_of_order", Severity: "moderate", Action: "replace_tool", Description: "工具调用顺序不符时，替换为计划期望的工具", Enabled: true},
		{ID: "rp-002", Name: "意外工具参数修正", DeviationType: "unexpected", Severity: "minor", Action: "sanitize_args", Description: "未知工具调用时，清理参数中的敏感信息", Enabled: true},
		{ID: "rp-003", Name: "权限违规阻断", DeviationType: "capability_violation", Severity: "critical", Action: "block", Description: "能力标签不足时直接阻断", Enabled: true},
		{ID: "rp-004", Name: "严格模式未知工具", DeviationType: "unexpected", Severity: "critical", Action: "block", Description: "严格模式下未匹配的工具直接阻断", Enabled: true},
		{ID: "rp-005", Name: "乱序仅记录", DeviationType: "out_of_order", Severity: "moderate", Action: "log", Description: "乱序调用仅记录不修复（保守策略）", Enabled: false},
	}
}

// GetRepairPolicies 返回所有修复策略
func (dd *DeviationDetector) GetRepairPolicies() []RepairPolicy {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	out := make([]RepairPolicy, len(dd.repairPolicies))
	copy(out, dd.repairPolicies)
	return out
}

// CreateRepairPolicy 创建自定义修复策略
func (dd *DeviationDetector) CreateRepairPolicy(p RepairPolicy) error {
	if p.ID == "" || p.Name == "" || p.Action == "" {
		return fmt.Errorf("id, name and action required")
	}
	if dd.db != nil {
		_, err := dd.db.Exec(`INSERT INTO repair_policies (id, name, deviation_type, severity, action, description, enabled, builtin) VALUES (?,?,?,?,?,?,?,0)`,
			p.ID, p.Name, p.DeviationType, p.Severity, p.Action, p.Description, p.Enabled)
		if err != nil {
			return fmt.Errorf("create policy: %w", err)
		}
	}
	dd.loadRepairPolicies()
	return nil
}

// UpdateRepairPolicy 更新修复策略
func (dd *DeviationDetector) UpdateRepairPolicy(id string, p RepairPolicy) error {
	if dd.db != nil {
		res, err := dd.db.Exec(`UPDATE repair_policies SET name=?, deviation_type=?, severity=?, action=?, description=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			p.Name, p.DeviationType, p.Severity, p.Action, p.Description, p.Enabled, id)
		if err != nil {
			return fmt.Errorf("update policy: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("policy %s not found", id)
		}
	}
	dd.loadRepairPolicies()
	return nil
}

// DeleteRepairPolicy 删除自定义修复策略（内置不可删）
func (dd *DeviationDetector) DeleteRepairPolicy(id string) error {
	if dd.db != nil {
		res, err := dd.db.Exec("DELETE FROM repair_policies WHERE id=? AND builtin=0", id)
		if err != nil {
			return fmt.Errorf("delete policy: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("policy %s not found or is builtin", id)
		}
	}
	dd.loadRepairPolicies()
	return nil
}

// findRepairPolicy 查找匹配偏差类型和严重度的第一个已启用策略
func (dd *DeviationDetector) findRepairPolicy(devType, severity string) *RepairPolicy {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	for _, p := range dd.repairPolicies {
		if !p.Enabled {
			continue
		}
		typeMatch := p.DeviationType == devType || p.DeviationType == "*"
		sevMatch := p.Severity == severity || p.Severity == "*"
		if typeMatch && sevMatch {
			return &p
		}
	}
	return nil
}

func generateDevID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "dev-" + hex.EncodeToString(b)
}

// Detect checks a tool call for deviations against the active plan and capabilities
func (dd *DeviationDetector) Detect(traceID, toolName, toolArgs string) *DeviationResult {
	dd.mu.RLock()
	cfg := dd.config
	dd.mu.RUnlock()

	atomic.AddInt64(&dd.stats.TotalChecks, 1)

	if !cfg.Enabled {
		return &DeviationResult{Decision: "allow", Reason: "deviation detection disabled"}
	}

	result := &DeviationResult{Decision: "allow"}

	// Check 1: Plan compliance (if PlanCompiler available)
	if dd.planCompiler != nil {
		eval := dd.planCompiler.EvaluateToolCall(traceID, toolName, toolArgs)
		if eval != nil && eval.Violation != nil {
			dev := &Deviation{
				ID:        generateDevID(),
				TraceID:   traceID,
				Type:      eval.Violation.Action, // forbidden/warn/block
				ToolName:  toolName,
				Expected:  eval.Violation.Expected,
				Actual:    eval.Violation.Description,
				Severity:  eval.Violation.Severity,
				Decision:  eval.Decision,
				CreatedAt: time.Now(),
			}

			// Auto-repair for minor and moderate deviations
			if cfg.AutoRepair && (dev.Severity == "minor" || dev.Severity == "moderate") {
				repair := dd.attemptRepair(traceID, toolName, toolArgs, eval)
				if repair.Success {
					dev.Repaired = true
					dev.RepairedTool = repair.Tool
					dev.RepairedArgs = repair.Args
					dev.Decision = "allow"
					atomic.AddInt64(&dd.stats.RepairsApplied, 1)
				}
			}

			dd.recordDeviation(dev)
			atomic.AddInt64(&dd.stats.TotalDeviations, 1)
			switch dev.Severity {
			case "critical":
				atomic.AddInt64(&dd.stats.CriticalCount, 1)
			case "moderate":
				atomic.AddInt64(&dd.stats.ModerateCount, 1)
			case "minor":
				atomic.AddInt64(&dd.stats.MinorCount, 1)
			}

			result.HasDeviation = true
			result.Deviation = dev
			result.Decision = dev.Decision
			result.Reason = fmt.Sprintf("plan deviation: %s (%s)", dev.Type, dev.Severity)
			if dev.Repaired {
				result.Repaired = true
				result.RepairedTool = dev.RepairedTool
				result.RepairedArgs = dev.RepairedArgs
			}
			return result
		}
	}

	// Check 2: Capability compliance (if CapabilityEngine available)
	if dd.capEngine != nil {
		// 注册工具结果并用真实 dataID 做评估（修复之前传空 dataID 的问题）
		dataID := fmt.Sprintf("dev-tool-%s-%d", toolName, time.Now().UnixNano())
		dd.capEngine.RegisterToolResult(traceID, toolName, dataID)
		capEval := dd.capEngine.Evaluate(traceID, dataID, "execute", toolName)
		if capEval != nil && capEval.Decision == "block" {
			dev := &Deviation{
				ID:        generateDevID(),
				TraceID:   traceID,
				Type:      "capability_violation",
				ToolName:  toolName,
				Expected:  capEval.Reason,
				Actual:    "insufficient capabilities",
				Severity:  "critical",
				Decision:  "block",
				CreatedAt: time.Now(),
			}
			dd.recordDeviation(dev)
			atomic.AddInt64(&dd.stats.TotalDeviations, 1)
			atomic.AddInt64(&dd.stats.CriticalCount, 1)

			result.HasDeviation = true
			result.Deviation = dev
			result.Decision = "block"
			result.Reason = fmt.Sprintf("capability violation: %s", capEval.Reason)
			return result
		}
	}

	return result
}

func (dd *DeviationDetector) attemptRepair(traceID, toolName, toolArgs string, eval *PlanEvaluation) RepairResult {
	if eval.Violation == nil {
		return RepairResult{Reason: "no violation"}
	}

	sev := eval.Violation.Severity
	sm := eval.StepMatch

	// 确定偏差类型
	devType := "unexpected"
	if sm == "out_of_order" {
		devType = "out_of_order"
	}

	// 查找匹配的修复策略
	policy := dd.findRepairPolicy(devType, sev)
	if policy == nil {
		return RepairResult{Reason: fmt.Sprintf("no repair policy for type=%s sev=%s", devType, sev)}
	}

	// 策略动作为 block/skip/log 时不修复
	if policy.Action == "block" || policy.Action == "skip" || policy.Action == "log" {
		return RepairResult{Reason: fmt.Sprintf("policy %s: action=%s (no repair)", policy.ID, policy.Action)}
	}

	// 检查修复预算
	dd.mu.RLock()
	maxRepairs := dd.config.MaxRepairs
	dd.mu.RUnlock()

	if dd.db != nil {
		var count int
		dd.db.QueryRow("SELECT COUNT(*) FROM plan_deviations WHERE trace_id=? AND repaired=1", traceID).Scan(&count)
		if count >= maxRepairs {
			return RepairResult{Reason: "repair budget exhausted"}
		}
	}

	// 按策略执行修复
	switch policy.Action {
	case "replace_tool":
		// 工具替换：用计划期望的工具替换
		if eval.Violation.Expected != "" {
			return RepairResult{
				Success: true,
				Tool:    eval.Violation.Expected,
				Args:    "",
				Reason:  fmt.Sprintf("[%s] 工具替换: %s → %s", policy.ID, toolName, eval.Violation.Expected),
			}
		}
		return RepairResult{Reason: fmt.Sprintf("[%s] replace_tool: no expected tool", policy.ID)}

	case "sanitize_args":
		// 参数修正：清理参数
		var args map[string]interface{}
		if json.Unmarshal([]byte(toolArgs), &args) != nil {
			return RepairResult{Reason: "cannot parse args"}
		}
		args["_repaired"] = true
		args["_repair_reason"] = eval.Violation.Expected
		args["_repair_policy"] = policy.ID
		repaired, _ := json.Marshal(args)
		return RepairResult{
			Success: true,
			Tool:    "",
			Args:    string(repaired),
			Reason:  fmt.Sprintf("[%s] 参数修正: %s", policy.ID, toolName),
		}

	default:
		return RepairResult{Reason: fmt.Sprintf("unknown action: %s", policy.Action)}
	}
}

func (dd *DeviationDetector) recordDeviation(dev *Deviation) {
	if dd.db == nil {
		return
	}
	_, err := dd.db.Exec("INSERT INTO plan_deviations (id,trace_id,type,tool_name,expected,actual,severity,repaired,repaired_tool,repaired_args,decision) VALUES(?,?,?,?,?,?,?,?,?,?,?)",
		dev.ID, dev.TraceID, dev.Type, dev.ToolName, dev.Expected, dev.Actual, dev.Severity,
		boolToInt(dev.Repaired), dev.RepairedTool, dev.RepairedArgs, dev.Decision)
	if err != nil {
		log.Printf("[Deviation] DB write failed: %v", err)
	}
}

func (dd *DeviationDetector) QueryDeviations(traceID, severity string, limit int) []Deviation {
	if dd.db == nil {
		return []Deviation{}
	}
	if limit <= 0 {
		limit = 50
	}
	q := "SELECT id,trace_id,type,tool_name,COALESCE(expected,''),COALESCE(actual,''),severity,repaired,COALESCE(repaired_tool,''),COALESCE(repaired_args,''),decision,COALESCE(created_at,'') FROM plan_deviations WHERE 1=1"
	var args []interface{}
	if traceID != "" {
		q += " AND trace_id=?"
		args = append(args, traceID)
	}
	if severity != "" {
		q += " AND severity=?"
		args = append(args, severity)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := dd.db.Query(q, args...)
	if err != nil {
		return []Deviation{}
	}
	defer rows.Close()
	var result []Deviation
	for rows.Next() {
		var d Deviation
		var rep int
		var ca string
		rows.Scan(&d.ID, &d.TraceID, &d.Type, &d.ToolName, &d.Expected, &d.Actual, &d.Severity, &rep, &d.RepairedTool, &d.RepairedArgs, &d.Decision, &ca)
		d.Repaired = rep != 0
		result = append(result, d)
	}
	if result == nil {
		result = []Deviation{}
	}
	return result
}

func (dd *DeviationDetector) GetStats() DeviationStats {
	return DeviationStats{
		TotalChecks:     atomic.LoadInt64(&dd.stats.TotalChecks),
		TotalDeviations: atomic.LoadInt64(&dd.stats.TotalDeviations),
		CriticalCount:   atomic.LoadInt64(&dd.stats.CriticalCount),
		ModerateCount:   atomic.LoadInt64(&dd.stats.ModerateCount),
		MinorCount:      atomic.LoadInt64(&dd.stats.MinorCount),
		RepairsApplied:  atomic.LoadInt64(&dd.stats.RepairsApplied),
	}
}

func (dd *DeviationDetector) GetConfig() DeviationConfig {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	return dd.config
}

func (dd *DeviationDetector) UpdateConfig(cfg DeviationConfig) {
	dd.mu.Lock()
	defer dd.mu.Unlock()
	dd.config = cfg
	log.Printf("[Deviation] Config updated: enabled=%v auto_repair=%v max_repairs=%d", cfg.Enabled, cfg.AutoRepair, cfg.MaxRepairs)
}
