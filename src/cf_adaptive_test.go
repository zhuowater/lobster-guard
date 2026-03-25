// cf_adaptive_test.go — AdaptiveStrategy 测试
// lobster-guard v24.2
package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupAdaptiveTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// 创建 cf_verifications 表（AdaptiveStrategy 的 feedback 查询需要）
	db.Exec(`CREATE TABLE IF NOT EXISTS cf_verifications (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_args TEXT,
		original_messages TEXT,
		control_messages TEXT,
		original_result TEXT,
		control_result TEXT,
		survived INTEGER NOT NULL DEFAULT 0,
		attribution_score REAL NOT NULL DEFAULT 0,
		causal_driver TEXT,
		verdict TEXT NOT NULL,
		decision TEXT NOT NULL DEFAULT 'allow',
		latency_ms INTEGER,
		cached INTEGER NOT NULL DEFAULT 0,
		tenant_id TEXT NOT NULL DEFAULT 'default',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	return db
}

func insertTestVerification(db *sql.DB, id, verdict string) {
	db.Exec(`INSERT INTO cf_verifications (id, trace_id, tool_name, verdict, decision, tenant_id)
		VALUES (?, 'trace-1', 'shell_exec', ?, 'block', 'default')`, id, verdict)
}

// TestAdaptive_Basic 基础功能测试
func TestAdaptive_Basic(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
		FeedbackEnabled:     true,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)
	if as == nil {
		t.Fatal("NewAdaptiveStrategy returned nil")
	}

	// 高风险应该验证
	shouldVerify, mode := as.ShouldVerifyAdaptive("shell_exec", 90)
	if !shouldVerify {
		t.Error("expected ShouldVerifyAdaptive to return true for high risk")
	}
	if mode != "sync" {
		t.Errorf("expected sync mode for risk=90, got %s", mode)
	}

	// 低风险应该跳过
	shouldVerify, _ = as.ShouldVerifyAdaptive("read_file", 10)
	if shouldVerify {
		t.Error("expected ShouldVerifyAdaptive to return false for low risk")
	}
}

// TestAdaptive_BudgetExhausted 预算耗尽测试
func TestAdaptive_BudgetExhausted(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    0.10, // 很小的预算
		CostPerVerification: 0.05,
		PriorityMode:        "risk_score",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 使用两次预算
	as.RecordVerificationCost(false)
	as.RecordVerificationCost(false)

	// 预算已满，应该不验证
	shouldVerify, _ := as.ShouldVerifyAdaptive("shell_exec", 95)
	if shouldVerify {
		t.Error("expected false when budget exhausted")
	}
}

// TestAdaptive_PriorityRiskScore risk_score 模式测试
func TestAdaptive_PriorityRiskScore(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "risk_score",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// risk_score 模式: 优先级 = riskScore / 100
	// riskScore=20 → priority=0.2 < 0.3 → 跳过
	shouldVerify, _ := as.ShouldVerifyAdaptive("shell_exec", 20)
	if shouldVerify {
		t.Error("expected false for risk=20 in risk_score mode (priority < 0.3)")
	}

	// riskScore=50 → priority=0.5 >= 0.3 → 验证（async because 50 < 80）
	shouldVerify, mode := as.ShouldVerifyAdaptive("shell_exec", 50)
	if !shouldVerify {
		t.Error("expected true for risk=50")
	}
	if mode != "async" {
		t.Errorf("expected async mode for risk=50, got %s", mode)
	}
}

// TestAdaptive_PriorityToolSeverity tool_severity 模式测试
func TestAdaptive_PriorityToolSeverity(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "tool_severity",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// shell_exec severity=1.0 → priority=1.0 >= 0.3 → 验证
	shouldVerify, _ := as.ShouldVerifyAdaptive("shell_exec", 10) // riskScore 不影响
	if !shouldVerify {
		t.Error("expected true for shell_exec in tool_severity mode")
	}

	// unknown_tool severity=0.3 → priority=0.3 >= 0.3 → 验证
	shouldVerify, _ = as.ShouldVerifyAdaptive("unknown_tool", 10)
	if !shouldVerify {
		t.Error("expected true for unknown_tool with severity=0.3")
	}
}

// TestAdaptive_PriorityHybrid hybrid 模式测试
func TestAdaptive_PriorityHybrid(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// hybrid: 0.6 * (riskScore/100) + 0.4 * toolSeverity
	// riskScore=10, unknown_tool(0.3) → 0.6*0.1 + 0.4*0.3 = 0.06 + 0.12 = 0.18 < 0.3 → 跳过
	shouldVerify, _ := as.ShouldVerifyAdaptive("unknown_tool", 10)
	if shouldVerify {
		t.Error("expected false for hybrid: low risk + low severity")
	}

	// riskScore=50, shell_exec(1.0) → 0.6*0.5 + 0.4*1.0 = 0.3 + 0.4 = 0.7 >= 0.3 → 验证
	shouldVerify, _ = as.ShouldVerifyAdaptive("shell_exec", 50)
	if !shouldVerify {
		t.Error("expected true for hybrid: medium risk + high severity")
	}
}

// TestAdaptive_SyncVsAsync 同步/异步决策测试
func TestAdaptive_SyncVsAsync(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "risk_score",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// risk=90 >= 80 → sync
	_, mode := as.ShouldVerifyAdaptive("shell_exec", 90)
	if mode != "sync" {
		t.Errorf("expected sync for risk=90, got %s", mode)
	}

	// risk=80 >= 80 → sync
	_, mode = as.ShouldVerifyAdaptive("shell_exec", 80)
	if mode != "sync" {
		t.Errorf("expected sync for risk=80, got %s", mode)
	}

	// risk=50 < 80 → async
	_, mode = as.ShouldVerifyAdaptive("shell_exec", 50)
	if mode != "async" {
		t.Errorf("expected async for risk=50, got %s", mode)
	}
}

// TestAdaptive_CostTracking 成本追踪测试
func TestAdaptive_CostTracking(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.10,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 记录3次验证
	as.RecordVerificationCost(true)
	as.RecordVerificationCost(false)
	as.RecordVerificationCost(true)

	summary := as.GetCostSummary()
	expectedCost := 0.30
	if diff := summary.MonthlyUsed - expectedCost; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected monthly_used=%.2f, got %.2f", expectedCost, summary.MonthlyUsed)
	}
	if summary.MonthlyBudget != 100.0 {
		t.Errorf("expected monthly_budget=100, got %.2f", summary.MonthlyBudget)
	}
}

// TestAdaptive_DailyHistory 每日历史测试
func TestAdaptive_DailyHistory(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 记录几次
	as.RecordVerificationCost(true)
	as.RecordVerificationCost(false)
	as.RecordVerificationCost(true)

	summary := as.GetCostSummary()
	today := time.Now().Format("2006-01-02")

	found := false
	for _, dc := range summary.DailyHistory {
		if dc.Date == today {
			found = true
			if dc.Verifications != 3 {
				t.Errorf("expected 3 verifications today, got %d", dc.Verifications)
			}
			if dc.BlockedCount != 2 {
				t.Errorf("expected 2 blocked today, got %d", dc.BlockedCount)
			}
		}
	}
	if !found {
		t.Error("today's record not found in daily history")
	}
}

// TestAdaptive_EffectTracker 效果追踪测试
func TestAdaptive_EffectTracker(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 直接设置效果计数器
	as.SetEffectCounters(8, 2, 7, 3)

	metrics := as.GetEffectMetrics()
	if metrics.TotalChecked != 20 {
		t.Errorf("expected total=20, got %d", metrics.TotalChecked)
	}
	if metrics.TruePositive != 8 {
		t.Errorf("expected TP=8, got %d", metrics.TruePositive)
	}
	if metrics.FalsePositive != 2 {
		t.Errorf("expected FP=2, got %d", metrics.FalsePositive)
	}

	// Accuracy = (TP+TN)/(TP+FP+TN+FN) = (8+7)/20 = 0.75
	expectedAcc := 0.75
	if diff := metrics.Accuracy - expectedAcc; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected accuracy=%.2f, got %.4f", expectedAcc, metrics.Accuracy)
	}
}

// TestAdaptive_RecordFeedback 人类反馈记录测试
func TestAdaptive_RecordFeedback(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
		FeedbackEnabled:     true,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 插入一条测试验证记录
	insertTestVerification(db, "cf-test-001", "INJECTION_DRIVEN")

	// 记录反馈: 正确阻断
	err := as.RecordFeedback("cf-test-001", true)
	if err != nil {
		t.Fatalf("RecordFeedback failed: %v", err)
	}

	// 检查效果指标
	metrics := as.GetEffectMetrics()
	if metrics.TruePositive != 1 {
		t.Errorf("expected TP=1, got %d", metrics.TruePositive)
	}

	// 插入另一条验证并标记为误报
	insertTestVerification(db, "cf-test-002", "INJECTION_DRIVEN")
	err = as.RecordFeedback("cf-test-002", false)
	if err != nil {
		t.Fatalf("RecordFeedback failed: %v", err)
	}

	metrics = as.GetEffectMetrics()
	if metrics.FalsePositive != 1 {
		t.Errorf("expected FP=1, got %d", metrics.FalsePositive)
	}
}

// TestAdaptive_Accuracy 准确率计算测试
func TestAdaptive_Accuracy(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := defaultAdaptiveConfig
	cfg.Enabled = true
	as := NewAdaptiveStrategy(db, cfg, nil)

	// TP=90, FP=10, TN=85, FN=15 → total=200
	as.SetEffectCounters(90, 10, 85, 15)
	m := as.GetEffectMetrics()

	// Accuracy = (90+85)/200 = 0.875
	if diff := m.Accuracy - 0.875; diff > 0.001 || diff < -0.001 {
		t.Errorf("accuracy: expected 0.875, got %.4f", m.Accuracy)
	}
	// Precision = 90/(90+10) = 0.9
	if diff := m.Precision - 0.9; diff > 0.001 || diff < -0.001 {
		t.Errorf("precision: expected 0.9, got %.4f", m.Precision)
	}
	// Recall = 90/(90+15) = 0.857
	if diff := m.Recall - 0.857; diff > 0.01 || diff < -0.01 {
		t.Errorf("recall: expected ~0.857, got %.4f", m.Recall)
	}
	// F1 = 2*P*R/(P+R) = 2*0.9*0.857/(0.9+0.857) ≈ 0.878
	if diff := m.F1Score - 0.878; diff > 0.01 || diff < -0.01 {
		t.Errorf("f1: expected ~0.878, got %.4f", m.F1Score)
	}
}

// TestAdaptive_PredictMonthlyCost 月成本预测测试
func TestAdaptive_PredictMonthlyCost(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.10,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 记录一些成本
	for i := 0; i < 10; i++ {
		as.RecordVerificationCost(false)
	}

	predicted := as.PredictMonthlyCost()
	// 预测值应大于等于当前使用量（1.0）
	if predicted < 1.0 {
		t.Errorf("predicted cost %.2f should be >= 1.0", predicted)
	}
	// 预测值应大于0
	if predicted <= 0 {
		t.Error("predicted cost should be positive")
	}
}

// TestAdaptive_DBPersistence 数据库持久化测试
func TestAdaptive_DBPersistence(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
		FeedbackEnabled:     true,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 记录成本
	as.RecordVerificationCost(true)
	as.RecordVerificationCost(false)

	// 验证 cf_costs 表
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM cf_costs`).Scan(&count)
	if count != 1 { // 同一天只有一行
		t.Errorf("expected 1 row in cf_costs, got %d", count)
	}

	var verifications int
	var costUSD float64
	today := time.Now().Format("2006-01-02")
	db.QueryRow(`SELECT verifications, cost_usd FROM cf_costs WHERE date = ?`, today).Scan(&verifications, &costUSD)
	if verifications != 2 {
		t.Errorf("expected 2 verifications in DB, got %d", verifications)
	}
	if diff := costUSD - 0.10; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected cost_usd=0.10, got %.4f", costUSD)
	}

	// 测试 feedback 持久化
	insertTestVerification(db, "cf-persist-1", "INJECTION_DRIVEN")
	as.RecordFeedback("cf-persist-1", true)

	var fbCount int
	db.QueryRow(`SELECT COUNT(*) FROM cf_feedback`).Scan(&fbCount)
	if fbCount != 1 {
		t.Errorf("expected 1 feedback row, got %d", fbCount)
	}
}

// TestAdaptive_ConfigUpdate 配置动态更新测试
func TestAdaptive_ConfigUpdate(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             false,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "risk_score",
		MinRiskForSync:      80.0,
		FeedbackEnabled:     true,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// disabled → 不验证
	shouldVerify, _ := as.ShouldVerifyAdaptive("shell_exec", 95)
	if shouldVerify {
		t.Error("expected false when disabled")
	}

	// 启用并更新配置
	as.UpdateConfig(AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    200.0,
		CostPerVerification: 0.10,
		PriorityMode:        "hybrid",
		MinRiskForSync:      70.0,
		FeedbackEnabled:     false,
	})

	newCfg := as.GetConfig()
	if !newCfg.Enabled {
		t.Error("expected enabled after update")
	}
	if newCfg.MonthlyBudgetUSD != 200.0 {
		t.Errorf("expected budget=200, got %.2f", newCfg.MonthlyBudgetUSD)
	}
	if newCfg.PriorityMode != "hybrid" {
		t.Errorf("expected mode=hybrid, got %s", newCfg.PriorityMode)
	}
	if newCfg.MinRiskForSync != 70.0 {
		t.Errorf("expected min_risk_for_sync=70, got %.1f", newCfg.MinRiskForSync)
	}

	// 现在应该验证了
	shouldVerify, _ = as.ShouldVerifyAdaptive("shell_exec", 95)
	if !shouldVerify {
		t.Error("expected true after enabling")
	}

	// 反馈应该被禁用
	insertTestVerification(db, "cf-update-1", "INJECTION_DRIVEN")
	err := as.RecordFeedback("cf-update-1", true)
	if err == nil {
		t.Error("expected error when feedback is disabled")
	}
}

// TestAdaptive_MonthReset 月份重置测试
func TestAdaptive_MonthReset(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    1.0, // 非常低的预算
		CostPerVerification: 0.50,
		PriorityMode:        "risk_score",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	// 使用两次，预算耗尽
	as.RecordVerificationCost(false)
	as.RecordVerificationCost(false)

	shouldVerify, _ := as.ShouldVerifyAdaptive("shell_exec", 95)
	if shouldVerify {
		t.Error("expected false when budget exhausted")
	}

	// 重置月份
	as.ResetMonth("2026-04")

	// 应该可以再验证了
	shouldVerify, _ = as.ShouldVerifyAdaptive("shell_exec", 95)
	if !shouldVerify {
		t.Error("expected true after month reset")
	}

	summary := as.GetCostSummary()
	if summary.MonthlyUsed != 0 {
		t.Errorf("expected monthly_used=0 after reset, got %.2f", summary.MonthlyUsed)
	}
}

// TestAdaptive_DisabledNoVerify 禁用状态不验证
func TestAdaptive_DisabledNoVerify(t *testing.T) {
	cfg := AdaptiveConfig{
		Enabled:             false,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(nil, cfg, nil)

	shouldVerify, mode := as.ShouldVerifyAdaptive("shell_exec", 100)
	if shouldVerify {
		t.Error("expected false when disabled")
	}
	if mode != "" {
		t.Errorf("expected empty mode when disabled, got %s", mode)
	}
}

// TestAdaptive_NilDB 无数据库运行
func TestAdaptive_NilDB(t *testing.T) {
	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    100.0,
		CostPerVerification: 0.05,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
		FeedbackEnabled:     true,
	}
	as := NewAdaptiveStrategy(nil, cfg, nil)

	shouldVerify, mode := as.ShouldVerifyAdaptive("shell_exec", 90)
	if !shouldVerify {
		t.Error("expected true even without DB")
	}
	if mode != "sync" {
		t.Errorf("expected sync, got %s", mode)
	}

	// RecordVerificationCost 不应 panic
	as.RecordVerificationCost(true)
	as.RecordVerificationCost(false)

	summary := as.GetCostSummary()
	if summary.MonthlyUsed < 0.099 {
		t.Errorf("expected ~0.10 monthly used, got %.4f", summary.MonthlyUsed)
	}
}

// TestAdaptive_RecordResultIntegration 完整的 RecordResult 流程测试
func TestAdaptive_RecordResultIntegration(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := defaultAdaptiveConfig
	cfg.Enabled = true
	as := NewAdaptiveStrategy(db, cfg, nil)

	vf := &CFVerification{
		ID:       "cf-int-001",
		ToolName: "shell_exec",
		Verdict:  "INJECTION_DRIVEN",
	}
	as.RecordResult(vf)

	summary := as.GetCostSummary()
	if summary.MonthlyUsed < 0.04 {
		t.Error("expected cost to be recorded")
	}

	// nil 不 panic
	as.RecordResult(nil)
}

// TestAdaptive_CostSummaryFields 验证 CostSummary 所有字段
func TestAdaptive_CostSummaryFields(t *testing.T) {
	db := setupAdaptiveTestDB(t)
	defer db.Close()

	cfg := AdaptiveConfig{
		Enabled:             true,
		MonthlyBudgetUSD:    50.0,
		CostPerVerification: 0.10,
		PriorityMode:        "hybrid",
		MinRiskForSync:      80.0,
	}
	as := NewAdaptiveStrategy(db, cfg, nil)

	for i := 0; i < 5; i++ {
		as.RecordVerificationCost(i%2 == 0)
	}

	s := as.GetCostSummary()

	if s.MonthlyBudget != 50.0 {
		t.Errorf("budget mismatch: got %.2f", s.MonthlyBudget)
	}
	if s.RemainingBudget < 0 {
		t.Error("remaining budget should not be negative")
	}
	if s.UsagePct <= 0 {
		t.Error("usage pct should be positive")
	}
	if s.TotalVerifications != 5 {
		t.Errorf("expected 5 total verifications, got %d", s.TotalVerifications)
	}
	if s.PredictedTotal <= 0 {
		t.Error("predicted total should be positive")
	}
	if s.CurrentMonth == "" {
		t.Error("current month should not be empty")
	}
}
