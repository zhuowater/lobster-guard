// adaptive_decision_test.go — 自适应决策引擎测试（v18.3）
package main

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// newTestAdaptiveEngine 创建测试用自适应决策引擎
func newTestAdaptiveEngine(t *testing.T) (*AdaptiveDecisionEngine, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// 创建 audit_log 表（依赖）
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, preview TEXT, hash TEXT, latency_ms REAL,
		upstream_id TEXT, app_id TEXT, trace_id TEXT, tenant_id TEXT
	)`)

	cfg := AdaptiveDecisionConfig{
		Enabled:         true,
		LookbackDays:    30,
		MinSamples:      5, // 降低阈值方便测试
		FPThreshold:     0.5,
		ConfidenceLevel: 0.95,
	}

	engine := NewAdaptiveDecisionEngine(db, nil, cfg)
	return engine, db
}

// newTestAdaptiveWithEnvelope 带信封管理器的引擎
func newTestAdaptiveWithEnvelope(t *testing.T) (*AdaptiveDecisionEngine, *EnvelopeManager) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, preview TEXT, hash TEXT, latency_ms REAL,
		upstream_id TEXT, app_id TEXT, trace_id TEXT, tenant_id TEXT
	)`)

	em := NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")

	cfg := AdaptiveDecisionConfig{
		Enabled:         true,
		LookbackDays:    30,
		MinSamples:      5,
		FPThreshold:     0.5,
		ConfidenceLevel: 0.95,
	}

	engine := NewAdaptiveDecisionEngine(db, em, cfg)
	return engine, em
}

// 1. TestAdaptiveShouldDowngrade — 高误伤用户降级
func TestAdaptiveShouldDowngrade(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 录入高误伤历史：10次 block 中有 8 次误伤
	for i := 0; i < 10; i++ {
		fp := i < 8 // 前 8 次是误伤
		engine.RecordOutcome("user-high-fp", "block", fp)
	}

	action, proof := engine.ShouldDowngrade("user-high-fp", "block")
	if action != "warn" {
		t.Errorf("expected action='warn' for high FP user, got %q", action)
	}
	if proof == nil {
		t.Fatal("proof should not be nil")
	}
	if proof.Decision != "downgrade_to_warn" {
		t.Errorf("expected decision='downgrade_to_warn', got %q", proof.Decision)
	}
	if proof.PosteriorMean <= 0.5 {
		t.Errorf("expected PosteriorMean > 0.5 for high FP, got %.3f", proof.PosteriorMean)
	}
}

// 2. TestAdaptiveKeepBlock — 低误伤用户保持block
func TestAdaptiveKeepBlock(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 录入低误伤历史：10次 block 中有 1 次误伤
	for i := 0; i < 10; i++ {
		fp := i == 0 // 只有 1 次误伤
		engine.RecordOutcome("user-low-fp", "block", fp)
	}

	action, proof := engine.ShouldDowngrade("user-low-fp", "block")
	if action != "block" {
		t.Errorf("expected action='block' for low FP user, got %q", action)
	}
	if proof == nil {
		t.Fatal("proof should not be nil")
	}
	if proof.Decision != "keep_block" {
		t.Errorf("expected decision='keep_block', got %q", proof.Decision)
	}
	if proof.PosteriorMean >= 0.5 {
		t.Errorf("expected PosteriorMean < 0.5 for low FP, got %.3f", proof.PosteriorMean)
	}
}

// 3. TestAdaptiveMinSamples — 样本不足不干预
func TestAdaptiveMinSamples(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 只录入 3 次（低于 MinSamples=5）
	for i := 0; i < 3; i++ {
		engine.RecordOutcome("user-few", "block", true) // 全是误伤，但样本不足
	}

	action, proof := engine.ShouldDowngrade("user-few", "block")
	if action != "block" {
		t.Errorf("expected action='block' (insufficient samples), got %q", action)
	}
	if proof == nil {
		t.Fatal("proof should not be nil")
	}
	if proof.Decision != "keep_block" {
		t.Errorf("expected decision='keep_block', got %q", proof.Decision)
	}
}

// 4. TestAdaptiveBayesianProof — 证明字段完整性
func TestAdaptiveBayesianProof(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-proof", "block", i < 6) // 6/10 误伤
	}

	proof := engine.GetProof("user-proof")
	if proof == nil {
		t.Fatal("proof should not be nil")
	}

	// 检查所有字段
	if proof.PriorAlpha != 1.0 {
		t.Errorf("PriorAlpha = %.1f, want 1.0", proof.PriorAlpha)
	}
	if proof.PriorBeta != 1.0 {
		t.Errorf("PriorBeta = %.1f, want 1.0", proof.PriorBeta)
	}
	if proof.ObservedFP != 6 {
		t.Errorf("ObservedFP = %d, want 6", proof.ObservedFP)
	}
	if proof.ObservedTotal != 10 {
		t.Errorf("ObservedTotal = %d, want 10", proof.ObservedTotal)
	}
	// 后验均值 = (1+6)/(1+6+1+4) = 7/12 ≈ 0.583
	if proof.PosteriorMean < 0.5 || proof.PosteriorMean > 0.7 {
		t.Errorf("PosteriorMean = %.3f, expected ~0.583", proof.PosteriorMean)
	}
	if proof.PosteriorLower >= proof.PosteriorMean {
		t.Errorf("PosteriorLower (%.3f) should be < PosteriorMean (%.3f)", proof.PosteriorLower, proof.PosteriorMean)
	}
	if proof.PosteriorUpper <= proof.PosteriorMean {
		t.Errorf("PosteriorUpper (%.3f) should be > PosteriorMean (%.3f)", proof.PosteriorUpper, proof.PosteriorMean)
	}
	if proof.Decision == "" {
		t.Error("Decision should not be empty")
	}
	if proof.Reason == "" {
		t.Error("Reason should not be empty")
	}

	// 检查 JSON 序列化完整性
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	var decoded BayesianProof
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if decoded.ObservedFP != proof.ObservedFP {
		t.Errorf("round-trip ObservedFP = %d, want %d", decoded.ObservedFP, proof.ObservedFP)
	}
}

// 5. TestAdaptiveRecordOutcome — 反馈记录
func TestAdaptiveRecordOutcome(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 记录若干结果
	if err := engine.RecordOutcome("user-record", "block", true); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}
	if err := engine.RecordOutcome("user-record", "block", false); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}
	if err := engine.RecordOutcome("user-record", "warn", false); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	// 检查内存缓存
	h := engine.GetHistory("user-record")
	if h == nil {
		t.Fatal("history should not be nil")
	}
	if h.TotalBlocks != 2 {
		t.Errorf("TotalBlocks = %d, want 2", h.TotalBlocks)
	}
	if h.FalseBlocks != 1 {
		t.Errorf("FalseBlocks = %d, want 1", h.FalseBlocks)
	}
	if h.TotalWarns != 1 {
		t.Errorf("TotalWarns = %d, want 1", h.TotalWarns)
	}
}

// 6. TestAdaptiveConfig — 配置热更新
func TestAdaptiveConfig(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 初始配置
	cfg := engine.GetConfig()
	if cfg.MinSamples != 5 {
		t.Errorf("initial MinSamples = %d, want 5", cfg.MinSamples)
	}
	if cfg.FPThreshold != 0.5 {
		t.Errorf("initial FPThreshold = %.1f, want 0.5", cfg.FPThreshold)
	}

	// 更新配置
	engine.UpdateConfig(AdaptiveDecisionConfig{
		MinSamples:  20,
		FPThreshold: 0.3,
	})

	cfg = engine.GetConfig()
	if cfg.MinSamples != 20 {
		t.Errorf("updated MinSamples = %d, want 20", cfg.MinSamples)
	}
	if cfg.FPThreshold != 0.3 {
		t.Errorf("updated FPThreshold = %.1f, want 0.3", cfg.FPThreshold)
	}

	// 更新后，之前够 5 个样本的用户不再被降级（因为 MinSamples 提高了）
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-cfg", "block", true)
	}
	action, _ := engine.ShouldDowngrade("user-cfg", "block")
	// 10 < 20，样本不足
	if action != "block" {
		t.Errorf("expected block (samples < new MinSamples), got %q", action)
	}
}

// 7. TestAdaptiveEnvelopeIntegration — 信封包含证明
func TestAdaptiveEnvelopeIntegration(t *testing.T) {
	engine, em := newTestAdaptiveWithEnvelope(t)

	// 录入高误伤用户
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-env", "block", true) // 全是误伤
	}

	// 触发降级
	action, proof := engine.ShouldDowngrade("user-env", "block")
	if action != "warn" {
		t.Errorf("expected warn, got %q", action)
	}
	if proof == nil {
		t.Fatal("proof should not be nil")
	}

	// 检查信封是否生成
	stats := em.Stats()
	total, ok := stats["total"].(int)
	if !ok {
		// 尝试 int64
		if t64, ok := stats["total"].(int64); ok {
			total = int(t64)
		}
	}
	if total < 1 {
		t.Errorf("expected at least 1 envelope, got %d", total)
	}
}

// 8. TestAdaptiveMultipleUsers — 多用户独立决策
func TestAdaptiveMultipleUsers(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	// 用户 A: 高误伤
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-A", "block", true)
	}
	// 用户 B: 低误伤
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-B", "block", false)
	}
	// 用户 C: 无记录
	// 用户 D: 适中误伤
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-D", "block", i < 3)
	}

	// A: 应降级
	actionA, _ := engine.ShouldDowngrade("user-A", "block")
	if actionA != "warn" {
		t.Errorf("user-A: expected warn, got %q", actionA)
	}

	// B: 应保持
	actionB, _ := engine.ShouldDowngrade("user-B", "block")
	if actionB != "block" {
		t.Errorf("user-B: expected block, got %q", actionB)
	}

	// C: 无记录，应保持
	actionC, _ := engine.ShouldDowngrade("user-C", "block")
	if actionC != "block" {
		t.Errorf("user-C: expected block, got %q", actionC)
	}

	// D: 3/10 误伤 → P(FP) ≈ 4/12 = 0.333 < 0.5 → 保持
	actionD, _ := engine.ShouldDowngrade("user-D", "block")
	if actionD != "block" {
		t.Errorf("user-D: expected block, got %q", actionD)
	}

	// 非 block action 不干预
	actionPass, _ := engine.ShouldDowngrade("user-A", "warn")
	if actionPass != "warn" {
		t.Errorf("non-block action: expected warn, got %q", actionPass)
	}
}

// 9. TestAdaptiveNoDataUser — 无数据用户的证明
func TestAdaptiveNoDataUser(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	proof := engine.GetProof("nonexistent-user")
	if proof == nil {
		t.Fatal("proof should not be nil for nonexistent user")
	}
	if proof.Decision != "no_data" {
		t.Errorf("expected decision='no_data', got %q", proof.Decision)
	}
	if proof.ObservedFP != 0 || proof.ObservedTotal != 0 {
		t.Errorf("expected zero observations, got FP=%d Total=%d", proof.ObservedFP, proof.ObservedTotal)
	}
}

// 10. TestAdaptiveStats — 统计数据
func TestAdaptiveStats(t *testing.T) {
	engine, _ := newTestAdaptiveEngine(t)

	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-stats-1", "block", i < 8)
	}
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("user-stats-2", "block", i < 2)
	}

	stats := engine.GetStats()
	if stats.TotalUsers != 2 {
		t.Errorf("TotalUsers = %d, want 2", stats.TotalUsers)
	}
	if stats.AvgFPRate <= 0 {
		t.Errorf("AvgFPRate should be > 0, got %.3f", stats.AvgFPRate)
	}
}
