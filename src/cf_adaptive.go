// cf_adaptive.go — AdaptiveStrategy: 自适应验证策略引擎 + 验证成本控制
// lobster-guard v24.2
// 基于验证预算、优先级模式、效果反馈的自适应决策，控制反事实验证的触发频率和成本
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// AdaptiveConfig 自适应策略配置
type AdaptiveConfig struct {
	Enabled             bool    `json:"enabled" yaml:"enabled"`
	MonthlyBudgetUSD    float64 `json:"monthly_budget_usd" yaml:"monthly_budget_usd"`         // 月预算（美元）
	CostPerVerification float64 `json:"cost_per_verification" yaml:"cost_per_verification"`     // 每次验证的预估成本
	PriorityMode        string  `json:"priority_mode" yaml:"priority_mode"`                     // "risk_score" / "tool_severity" / "hybrid"
	MinRiskForSync      float64 `json:"min_risk_for_sync" yaml:"min_risk_for_sync"`             // 同步验证的最低风险分
	FeedbackEnabled     bool    `json:"feedback_enabled" yaml:"feedback_enabled"`                // 是否接受人类反馈
}

// CostTracker 成本追踪器
type CostTracker struct {
	mu            sync.RWMutex
	MonthlyUsed   float64     `json:"monthly_used"`
	MonthlyBudget float64     `json:"monthly_budget"`
	DailyHistory  []DailyCost `json:"daily_history"` // 最近30天
	CurrentMonth  string      `json:"current_month"`  // "2026-03"
}

// DailyCost 每日成本记录
type DailyCost struct {
	Date           string  `json:"date"`
	Verifications  int     `json:"verifications"`
	CostUSD        float64 `json:"cost_usd"`
	BlockedCount   int     `json:"blocked_count"`
	FalsePositives int     `json:"false_positives"`
}

// PendingVerification 待处理验证
type PendingVerification struct {
	ToolName   string
	ToolArgs   string
	TraceID    string
	RiskScore  float64
	Priority   float64 // 综合优先级分数
	EnqueuedAt time.Time
}

// EffectTracker 效果追踪器
type EffectTracker struct {
	mu           sync.RWMutex
	TotalChecked int64   `json:"total_checked"`
	TruePositive int64   `json:"true_positive"`  // 正确阻断
	FalsePositive int64  `json:"false_positive"` // 误报
	TrueNegative int64   `json:"true_negative"`  // 正确放行
	FalseNegative int64  `json:"false_negative"` // 漏报
	Accuracy     float64 `json:"accuracy"`
	Precision    float64 `json:"precision"`
	Recall       float64 `json:"recall"`
	F1Score      float64 `json:"f1_score"`
}

// CostSummary 成本摘要
type CostSummary struct {
	MonthlyUsed      float64     `json:"monthly_used"`
	MonthlyBudget    float64     `json:"monthly_budget"`
	UsagePct         float64     `json:"usage_pct"`
	DailyHistory     []DailyCost `json:"daily_history"`
	CurrentMonth     string      `json:"current_month"`
	PredictedTotal   float64     `json:"predicted_total"`
	RemainingBudget  float64     `json:"remaining_budget"`
	AvgDailyCost     float64     `json:"avg_daily_cost"`
	TotalVerifications int       `json:"total_verifications"`
}

// AdaptiveStrategy 自适应验证策略引擎
type AdaptiveStrategy struct {
	mu            sync.RWMutex
	db            *sql.DB
	config        AdaptiveConfig
	costTracker   *CostTracker
	priorityQueue []PendingVerification
	effectTracker *EffectTracker
	pathPolicy    *PathPolicyEngine
}

var defaultAdaptiveConfig = AdaptiveConfig{
	Enabled:             false,
	MonthlyBudgetUSD:    100.0,
	CostPerVerification: 0.05,
	PriorityMode:        "hybrid",
	MinRiskForSync:      80.0,
	FeedbackEnabled:     true,
}

// toolSeverityMap 工具危险等级权重 (0-1)
var toolSeverityMap = map[string]float64{
	"shell_exec":      1.0,
	"execute_command":  1.0,
	"run_command":      1.0,
	"send_email":       0.8,
	"send_message":     0.7,
	"file_write":       0.7,
	"write_file":       0.7,
	"create_file":      0.6,
	"http_request":     0.8,
	"fetch_url":        0.7,
	"database_query":   0.6,
	"sql_query":        0.6,
	"delete_file":      0.9,
	"remove_file":      0.9,
}

// NewAdaptiveStrategy 初始化自适应验证策略引擎
func NewAdaptiveStrategy(db *sql.DB, config AdaptiveConfig, pathPolicy *PathPolicyEngine) *AdaptiveStrategy {
	if config.MonthlyBudgetUSD <= 0 {
		config.MonthlyBudgetUSD = defaultAdaptiveConfig.MonthlyBudgetUSD
	}
	if config.CostPerVerification <= 0 {
		config.CostPerVerification = defaultAdaptiveConfig.CostPerVerification
	}
	if config.PriorityMode == "" {
		config.PriorityMode = defaultAdaptiveConfig.PriorityMode
	}
	if config.MinRiskForSync <= 0 {
		config.MinRiskForSync = defaultAdaptiveConfig.MinRiskForSync
	}

	now := time.Now()
	currentMonth := now.Format("2006-01")

	as := &AdaptiveStrategy{
		db:     db,
		config: config,
		costTracker: &CostTracker{
			MonthlyBudget: config.MonthlyBudgetUSD,
			CurrentMonth:  currentMonth,
			DailyHistory:  make([]DailyCost, 0),
		},
		priorityQueue: make([]PendingVerification, 0),
		effectTracker: &EffectTracker{},
		pathPolicy:    pathPolicy,
	}

	if db != nil {
		as.initDB()
		as.loadFromDB()
	}

	log.Printf("[AdaptiveStrategy] 初始化: enabled=%v budget=%.2f$/mo cost=%.4f$/verify mode=%s sync_threshold=%.0f feedback=%v",
		config.Enabled, config.MonthlyBudgetUSD, config.CostPerVerification, config.PriorityMode, config.MinRiskForSync, config.FeedbackEnabled)

	return as
}

func (as *AdaptiveStrategy) initDB() {
	if as.db == nil {
		return
	}
	as.db.Exec(`CREATE TABLE IF NOT EXISTS cf_costs (
		date TEXT PRIMARY KEY,
		verifications INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		blocked_count INTEGER NOT NULL DEFAULT 0,
		false_positives INTEGER NOT NULL DEFAULT 0,
		month TEXT NOT NULL DEFAULT ''
	)`)
	as.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cf_costs_month ON cf_costs(month)`)

	as.db.Exec(`CREATE TABLE IF NOT EXISTS cf_feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		verification_id TEXT NOT NULL,
		was_correct INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	as.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cf_feedback_vid ON cf_feedback(verification_id)`)
}

func (as *AdaptiveStrategy) loadFromDB() {
	if as.db == nil {
		return
	}

	as.costTracker.mu.Lock()
	defer as.costTracker.mu.Unlock()

	currentMonth := as.costTracker.CurrentMonth

	// 加载当月成本
	row := as.db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cf_costs WHERE month = ?`, currentMonth)
	row.Scan(&as.costTracker.MonthlyUsed)

	// 加载最近30天每日历史
	rows, err := as.db.Query(`SELECT date, verifications, cost_usd, blocked_count, false_positives FROM cf_costs ORDER BY date DESC LIMIT 30`)
	if err != nil {
		return
	}
	defer rows.Close()
	var history []DailyCost
	for rows.Next() {
		var dc DailyCost
		if rows.Scan(&dc.Date, &dc.Verifications, &dc.CostUSD, &dc.BlockedCount, &dc.FalsePositives) == nil {
			history = append(history, dc)
		}
	}
	// 反转为时间正序
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}
	as.costTracker.DailyHistory = history

	// 加载效果数据
	as.loadEffectFromDB()
}

func (as *AdaptiveStrategy) loadEffectFromDB() {
	if as.db == nil {
		return
	}
	as.effectTracker.mu.Lock()
	defer as.effectTracker.mu.Unlock()

	// 从 cf_feedback 统计效果
	var tp, fp, tn, fn int64
	// TruePositive: verdict=INJECTION_DRIVEN AND was_correct=1
	as.db.QueryRow(`SELECT COUNT(*) FROM cf_feedback f JOIN cf_verifications v ON f.verification_id = v.id WHERE v.verdict = 'INJECTION_DRIVEN' AND f.was_correct = 1`).Scan(&tp)
	// FalsePositive: verdict=INJECTION_DRIVEN AND was_correct=0
	as.db.QueryRow(`SELECT COUNT(*) FROM cf_feedback f JOIN cf_verifications v ON f.verification_id = v.id WHERE v.verdict = 'INJECTION_DRIVEN' AND f.was_correct = 0`).Scan(&fp)
	// TrueNegative: verdict=USER_DRIVEN AND was_correct=1
	as.db.QueryRow(`SELECT COUNT(*) FROM cf_feedback f JOIN cf_verifications v ON f.verification_id = v.id WHERE v.verdict = 'USER_DRIVEN' AND f.was_correct = 1`).Scan(&tn)
	// FalseNegative: verdict=USER_DRIVEN AND was_correct=0
	as.db.QueryRow(`SELECT COUNT(*) FROM cf_feedback f JOIN cf_verifications v ON f.verification_id = v.id WHERE v.verdict = 'USER_DRIVEN' AND f.was_correct = 0`).Scan(&fn)

	as.effectTracker.TruePositive = tp
	as.effectTracker.FalsePositive = fp
	as.effectTracker.TrueNegative = tn
	as.effectTracker.FalseNegative = fn
	as.effectTracker.TotalChecked = tp + fp + tn + fn
	as.effectTracker.recalculate()
}

// recalculate 重新计算效果指标
func (et *EffectTracker) recalculate() {
	total := et.TruePositive + et.FalsePositive + et.TrueNegative + et.FalseNegative
	if total == 0 {
		et.Accuracy = 0
		et.Precision = 0
		et.Recall = 0
		et.F1Score = 0
		return
	}
	et.TotalChecked = total
	et.Accuracy = float64(et.TruePositive+et.TrueNegative) / float64(total)

	if et.TruePositive+et.FalsePositive > 0 {
		et.Precision = float64(et.TruePositive) / float64(et.TruePositive+et.FalsePositive)
	} else {
		et.Precision = 0
	}

	if et.TruePositive+et.FalseNegative > 0 {
		et.Recall = float64(et.TruePositive) / float64(et.TruePositive+et.FalseNegative)
	} else {
		et.Recall = 0
	}

	if et.Precision+et.Recall > 0 {
		et.F1Score = 2 * (et.Precision * et.Recall) / (et.Precision + et.Recall)
	} else {
		et.F1Score = 0
	}
}

// ShouldVerifyAdaptive 自适应判断是否应执行验证，返回 (是否验证, 原因/模式)
// 返回的第二个值: "sync", "async", 或 "" (跳过)
func (as *AdaptiveStrategy) ShouldVerifyAdaptive(toolName string, riskScore float64) (bool, string) {
	as.mu.RLock()
	cfg := as.config
	as.mu.RUnlock()

	if !cfg.Enabled {
		return false, ""
	}

	// 1. 检查月预算
	as.costTracker.mu.RLock()
	monthlyUsed := as.costTracker.MonthlyUsed
	as.costTracker.mu.RUnlock()

	if monthlyUsed+cfg.CostPerVerification > cfg.MonthlyBudgetUSD {
		return false, ""
	}

	// 2. 计算优先级
	priority := as.calculatePriority(toolName, riskScore, cfg.PriorityMode)

	// 低优先级（< 0.3）跳过
	if priority < 0.3 {
		return false, ""
	}

	// 3. 决定同步/异步
	if riskScore >= cfg.MinRiskForSync {
		return true, "sync"
	}

	// 中等优先级（0.3-0.7）用异步
	return true, "async"
}

// calculatePriority 按优先级模式计算综合优先级 (0-1)
func (as *AdaptiveStrategy) calculatePriority(toolName string, riskScore float64, mode string) float64 {
	switch mode {
	case "risk_score":
		return riskScore / 100.0
	case "tool_severity":
		return as.getToolSeverity(toolName)
	case "hybrid":
		rs := riskScore / 100.0
		ts := as.getToolSeverity(toolName)
		return 0.6*rs + 0.4*ts
	default:
		return riskScore / 100.0
	}
}

// getToolSeverity 获取工具的危险等级权重 (0-1)
func (as *AdaptiveStrategy) getToolSeverity(toolName string) float64 {
	if sev, ok := toolSeverityMap[toolName]; ok {
		return sev
	}
	return 0.3 // 未知工具默认中低
}

// RecordVerificationCost 记录一次验证的成本
func (as *AdaptiveStrategy) RecordVerificationCost(blocked bool) {
	as.mu.RLock()
	costPer := as.config.CostPerVerification
	as.mu.RUnlock()

	today := time.Now().Format("2006-01-02")
	currentMonth := time.Now().Format("2006-01")

	as.costTracker.mu.Lock()

	// 月份切换检测
	if as.costTracker.CurrentMonth != currentMonth {
		as.costTracker.CurrentMonth = currentMonth
		as.costTracker.MonthlyUsed = 0
	}

	as.costTracker.MonthlyUsed += costPer

	// 更新每日历史
	found := false
	for i := range as.costTracker.DailyHistory {
		if as.costTracker.DailyHistory[i].Date == today {
			as.costTracker.DailyHistory[i].Verifications++
			as.costTracker.DailyHistory[i].CostUSD += costPer
			if blocked {
				as.costTracker.DailyHistory[i].BlockedCount++
			}
			found = true
			break
		}
	}
	if !found {
		dc := DailyCost{
			Date:          today,
			Verifications: 1,
			CostUSD:       costPer,
		}
		if blocked {
			dc.BlockedCount = 1
		}
		as.costTracker.DailyHistory = append(as.costTracker.DailyHistory, dc)
		// 保留最近30天
		if len(as.costTracker.DailyHistory) > 30 {
			as.costTracker.DailyHistory = as.costTracker.DailyHistory[len(as.costTracker.DailyHistory)-30:]
		}
	}
	as.costTracker.mu.Unlock()

	// 持久化到数据库
	if as.db != nil {
		blockedInt := 0
		if blocked {
			blockedInt = 1
		}
		as.db.Exec(`INSERT INTO cf_costs (date, verifications, cost_usd, blocked_count, false_positives, month)
			VALUES (?, 1, ?, ?, 0, ?)
			ON CONFLICT(date) DO UPDATE SET
				verifications = verifications + 1,
				cost_usd = cost_usd + ?,
				blocked_count = blocked_count + ?`,
			today, costPer, blockedInt, currentMonth, costPer, blockedInt)
	}
}

// RecordResult 记录验证结果用于效果追踪（从 CFVerification 推断）
func (as *AdaptiveStrategy) RecordResult(vf *CFVerification) {
	if vf == nil {
		return
	}
	blocked := vf.Verdict == "INJECTION_DRIVEN"
	as.RecordVerificationCost(blocked)
}

// RecordFeedback 记录人类反馈
func (as *AdaptiveStrategy) RecordFeedback(verificationID string, wasCorrect bool) error {
	as.mu.RLock()
	feedbackEnabled := as.config.FeedbackEnabled
	as.mu.RUnlock()

	if !feedbackEnabled {
		return fmt.Errorf("feedback is disabled")
	}

	wasCorrectInt := 0
	if wasCorrect {
		wasCorrectInt = 1
	}

	if as.db != nil {
		_, err := as.db.Exec(`INSERT INTO cf_feedback (verification_id, was_correct) VALUES (?, ?)`,
			verificationID, wasCorrectInt)
		if err != nil {
			return fmt.Errorf("save feedback: %w", err)
		}

		// 如果反馈为"误报"，更新当日的 false_positives 计数
		if !wasCorrect {
			today := time.Now().Format("2006-01-02")
			as.db.Exec(`UPDATE cf_costs SET false_positives = false_positives + 1 WHERE date = ?`, today)

			as.costTracker.mu.Lock()
			for i := range as.costTracker.DailyHistory {
				if as.costTracker.DailyHistory[i].Date == today {
					as.costTracker.DailyHistory[i].FalsePositives++
					break
				}
			}
			as.costTracker.mu.Unlock()
		}
	}

	// 更新效果指标
	as.updateEffectFromFeedback(verificationID, wasCorrect)

	return nil
}

func (as *AdaptiveStrategy) updateEffectFromFeedback(verificationID string, wasCorrect bool) {
	if as.db == nil {
		return
	}

	// 查询这条验证的判定
	var verdict string
	err := as.db.QueryRow(`SELECT verdict FROM cf_verifications WHERE id = ?`, verificationID).Scan(&verdict)
	if err != nil {
		return
	}

	as.effectTracker.mu.Lock()
	defer as.effectTracker.mu.Unlock()

	switch {
	case verdict == "INJECTION_DRIVEN" && wasCorrect:
		as.effectTracker.TruePositive++
	case verdict == "INJECTION_DRIVEN" && !wasCorrect:
		as.effectTracker.FalsePositive++
	case verdict == "USER_DRIVEN" && wasCorrect:
		as.effectTracker.TrueNegative++
	case verdict == "USER_DRIVEN" && !wasCorrect:
		as.effectTracker.FalseNegative++
	}
	as.effectTracker.recalculate()
}

// GetCostSummary 获取成本摘要
func (as *AdaptiveStrategy) GetCostSummary() CostSummary {
	as.costTracker.mu.RLock()
	defer as.costTracker.mu.RUnlock()

	predicted := as.predictMonthlyCostLocked()
	remaining := as.costTracker.MonthlyBudget - as.costTracker.MonthlyUsed
	if remaining < 0 {
		remaining = 0
	}

	usagePct := 0.0
	if as.costTracker.MonthlyBudget > 0 {
		usagePct = as.costTracker.MonthlyUsed / as.costTracker.MonthlyBudget * 100
	}

	// 计算总验证数和平均日成本
	totalVerifications := 0
	totalCost := 0.0
	daysWithData := 0
	historyCopy := make([]DailyCost, len(as.costTracker.DailyHistory))
	copy(historyCopy, as.costTracker.DailyHistory)
	for _, dc := range historyCopy {
		totalVerifications += dc.Verifications
		totalCost += dc.CostUSD
		if dc.Verifications > 0 {
			daysWithData++
		}
	}
	avgDailyCost := 0.0
	if daysWithData > 0 {
		avgDailyCost = totalCost / float64(daysWithData)
	}

	return CostSummary{
		MonthlyUsed:        as.costTracker.MonthlyUsed,
		MonthlyBudget:      as.costTracker.MonthlyBudget,
		UsagePct:           usagePct,
		DailyHistory:       historyCopy,
		CurrentMonth:       as.costTracker.CurrentMonth,
		PredictedTotal:     predicted,
		RemainingBudget:    remaining,
		AvgDailyCost:       avgDailyCost,
		TotalVerifications: totalVerifications,
	}
}

// GetEffectMetrics 获取效果指标（返回副本）
func (as *AdaptiveStrategy) GetEffectMetrics() EffectTracker {
	as.effectTracker.mu.RLock()
	defer as.effectTracker.mu.RUnlock()
	return EffectTracker{
		TotalChecked:  as.effectTracker.TotalChecked,
		TruePositive:  as.effectTracker.TruePositive,
		FalsePositive: as.effectTracker.FalsePositive,
		TrueNegative:  as.effectTracker.TrueNegative,
		FalseNegative: as.effectTracker.FalseNegative,
		Accuracy:      as.effectTracker.Accuracy,
		Precision:     as.effectTracker.Precision,
		Recall:        as.effectTracker.Recall,
		F1Score:       as.effectTracker.F1Score,
	}
}

// PredictMonthlyCost 预测当月总成本
func (as *AdaptiveStrategy) PredictMonthlyCost() float64 {
	as.costTracker.mu.RLock()
	defer as.costTracker.mu.RUnlock()
	return as.predictMonthlyCostLocked()
}

func (as *AdaptiveStrategy) predictMonthlyCostLocked() float64 {
	now := time.Now()
	dayOfMonth := now.Day()
	if dayOfMonth <= 0 {
		dayOfMonth = 1
	}
	// 当月总天数
	year, month, _ := now.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, now.Location()).Day()

	if dayOfMonth == 0 {
		return 0
	}
	dailyRate := as.costTracker.MonthlyUsed / float64(dayOfMonth)
	return dailyRate * float64(daysInMonth)
}

// GetConfig 获取当前配置
func (as *AdaptiveStrategy) GetConfig() AdaptiveConfig {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.config
}

// UpdateConfig 运行时更新配置
func (as *AdaptiveStrategy) UpdateConfig(cfg AdaptiveConfig) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if cfg.MonthlyBudgetUSD > 0 {
		as.config.MonthlyBudgetUSD = cfg.MonthlyBudgetUSD
		as.costTracker.mu.Lock()
		as.costTracker.MonthlyBudget = cfg.MonthlyBudgetUSD
		as.costTracker.mu.Unlock()
	}
	if cfg.CostPerVerification > 0 {
		as.config.CostPerVerification = cfg.CostPerVerification
	}
	if cfg.PriorityMode == "risk_score" || cfg.PriorityMode == "tool_severity" || cfg.PriorityMode == "hybrid" {
		as.config.PriorityMode = cfg.PriorityMode
	}
	if cfg.MinRiskForSync > 0 {
		as.config.MinRiskForSync = cfg.MinRiskForSync
	}
	as.config.Enabled = cfg.Enabled
	as.config.FeedbackEnabled = cfg.FeedbackEnabled

	log.Printf("[AdaptiveStrategy] 配置更新: enabled=%v budget=%.2f cost=%.4f mode=%s sync=%.0f feedback=%v",
		as.config.Enabled, as.config.MonthlyBudgetUSD, as.config.CostPerVerification,
		as.config.PriorityMode, as.config.MinRiskForSync, as.config.FeedbackEnabled)
}

// ResetMonth 月份重置（用于测试或手动触发）
func (as *AdaptiveStrategy) ResetMonth(newMonth string) {
	as.costTracker.mu.Lock()
	defer as.costTracker.mu.Unlock()
	as.costTracker.CurrentMonth = newMonth
	as.costTracker.MonthlyUsed = 0
}

// SetEffectCounters 直接设置效果计数器（用于测试）
func (as *AdaptiveStrategy) SetEffectCounters(tp, fp, tn, fn int64) {
	as.effectTracker.mu.Lock()
	defer as.effectTracker.mu.Unlock()
	as.effectTracker.TruePositive = tp
	as.effectTracker.FalsePositive = fp
	as.effectTracker.TrueNegative = tn
	as.effectTracker.FalseNegative = fn
	as.effectTracker.recalculate()
}

// roundFloat 辅助函数: 保留指定位数小数
func roundFloat(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}
