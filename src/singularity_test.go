// singularity_test.go — 奇点蜜罐 + 奇点预算测试（v18.3）
package main

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// newTestSingularityEngine 创建测试用奇点引擎
func newTestSingularityEngine(t *testing.T) (*SingularityEngine, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// 创建依赖表
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, preview TEXT, hash TEXT, latency_ms REAL,
		upstream_id TEXT, app_id TEXT, trace_id TEXT, tenant_id TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_templates (
		id TEXT PRIMARY KEY, name TEXT, trigger_type TEXT, trigger_pattern TEXT,
		response_type TEXT, response_template TEXT, watermark_prefix TEXT,
		enabled INTEGER DEFAULT 1, tenant_id TEXT, created_at TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_triggers (
		id TEXT PRIMARY KEY, timestamp TEXT, tenant_id TEXT, sender_id TEXT,
		template_id TEXT, template_name TEXT, trigger_type TEXT, original_input TEXT,
		fake_response TEXT, watermark TEXT, detonated INTEGER DEFAULT 0,
		detonated_at TEXT, trace_id TEXT
	)`)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       3,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 1,
	}
	engine := NewSingularityEngine(db, honeypot, nil, cfg)
	return engine, db
}

// newTestSingularityWithEnvelope 带信封的奇点引擎
func newTestSingularityWithEnvelope(t *testing.T) (*SingularityEngine, *EnvelopeManager, *sql.DB) {
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
	db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_templates (
		id TEXT PRIMARY KEY, name TEXT, trigger_type TEXT, trigger_pattern TEXT,
		response_type TEXT, response_template TEXT, watermark_prefix TEXT,
		enabled INTEGER DEFAULT 1, tenant_id TEXT, created_at TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_triggers (
		id TEXT PRIMARY KEY, timestamp TEXT, tenant_id TEXT, sender_id TEXT,
		template_id TEXT, template_name TEXT, trigger_type TEXT, original_input TEXT,
		fake_response TEXT, watermark TEXT, detonated INTEGER DEFAULT 0,
		detonated_at TEXT, trace_id TEXT
	)`)

	em := NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")
	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       3,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 1,
	}
	engine := NewSingularityEngine(db, honeypot, em, cfg)
	return engine, em, db
}

// 1. TestSingularityExposureTemplates — 模板获取
func TestSingularityExposureTemplates(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)

	// 获取 IM 通道 Level 3 模板
	templates := engine.GetExposureTemplates("im", 3)
	if len(templates) == 0 {
		t.Fatal("expected templates for im/level=3")
	}
	// Level 3 应该包含 level 1, 2, 3 的模板
	for _, tpl := range templates {
		if tpl.Channel != "im" {
			t.Errorf("expected channel=im, got %q", tpl.Channel)
		}
		if tpl.Level > 3 {
			t.Errorf("expected level <= 3, got %d", tpl.Level)
		}
	}

	// 获取 LLM 通道 Level 1 模板
	llmTemplates := engine.GetExposureTemplates("llm", 1)
	for _, tpl := range llmTemplates {
		if tpl.Level > 1 {
			t.Errorf("expected level <= 1, got %d", tpl.Level)
		}
	}

	// Level 0 不返回模板
	zeroTemplates := engine.GetExposureTemplates("im", 0)
	if len(zeroTemplates) != 0 {
		t.Errorf("expected 0 templates for level=0, got %d", len(zeroTemplates))
	}

	// 所有模板总数 >= 10
	all := engine.GetAllTemplates()
	if len(all) < 10 {
		t.Errorf("expected at least 10 templates, got %d", len(all))
	}

	// 确保覆盖 3 个通道
	channels := map[string]bool{}
	for _, tpl := range all {
		channels[tpl.Channel] = true
	}
	for _, ch := range []string{"im", "llm", "toolcall"} {
		if !channels[ch] {
			t.Errorf("missing templates for channel %q", ch)
		}
	}

	// 确保覆盖多个等级
	levels := map[int]bool{}
	for _, tpl := range all {
		levels[tpl.Level] = true
	}
	if len(levels) < 3 {
		t.Errorf("expected at least 3 different levels, got %d", len(levels))
	}
}

// 2. TestSingularityShouldExpose — 暴露判断
func TestSingularityShouldExpose(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)

	// IM 通道 Level=3, 应该暴露
	shouldExpose, tpl := engine.ShouldExpose("im", "trace-001")
	if !shouldExpose {
		t.Error("expected exposure for im channel (level=3)")
	}
	if tpl == nil {
		t.Fatal("template should not be nil")
	}
	if tpl.Channel != "im" {
		t.Errorf("expected channel=im, got %q", tpl.Channel)
	}

	// 确定性选择：同一 traceID 应返回相同模板
	_, tpl2 := engine.ShouldExpose("im", "trace-001")
	if tpl2.Name != tpl.Name {
		t.Errorf("deterministic select failed: %q != %q", tpl2.Name, tpl.Name)
	}

	// 禁用后不暴露
	engine.UpdateConfig(SingularityConfig{Enabled: false})
	shouldExpose, _ = engine.ShouldExpose("im", "trace-002")
	if shouldExpose {
		t.Error("should not expose when disabled")
	}

	// 重新启用，Level=0 不暴露
	engine.UpdateConfig(SingularityConfig{Enabled: true, IMExposureLevel: 0})
	shouldExpose, _ = engine.ShouldExpose("im", "trace-003")
	if shouldExpose {
		t.Error("should not expose at level=0")
	}
}

// 3. TestSingularityRecommendPlacement — 推荐+证明
func TestSingularityRecommendPlacement(t *testing.T) {
	engine, db := newTestSingularityEngine(t)

	// 插入测试数据
	for i := 0; i < 100; i++ {
		action := "pass"
		if i < 20 {
			action = "block"
		}
		if i >= 20 && i < 25 {
			action = "warn"
		}
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, preview, hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			"2024-01-01T00:00:00Z", "inbound", "sender", action, "", "", "", 1.0, "", "", "")
	}
	for i := 0; i < 50; i++ {
		action := "pass"
		if i < 15 {
			action = "block"
		}
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, preview, hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			"2024-01-01T00:00:00Z", "llm_request", "sender", action, "", "", "", 1.0, "", "", "")
	}

	proof := engine.RecommendPlacement()
	if proof == nil {
		t.Fatal("proof should not be nil")
	}
	if proof.RecommendedChannel == "" {
		t.Error("recommended channel should not be empty")
	}
	if proof.IMTrafficVolume == 0 {
		t.Error("IM traffic volume should not be 0")
	}
	if proof.Reason == "" {
		t.Error("reason should not be empty")
	}

	// JSON 序列化检查
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	var decoded PlacementProof
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
}

// 4. TestSingularitySetLevel — 设置等级
func TestSingularitySetLevel(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)

	// 正常设置
	if err := engine.SetExposureLevel("im", 5); err != nil {
		t.Fatalf("SetExposureLevel failed: %v", err)
	}
	cfg := engine.GetConfig()
	if cfg.IMExposureLevel != 5 {
		t.Errorf("IMExposureLevel = %d, want 5", cfg.IMExposureLevel)
	}

	if err := engine.SetExposureLevel("llm", 0); err != nil {
		t.Fatalf("SetExposureLevel failed: %v", err)
	}
	cfg = engine.GetConfig()
	if cfg.LLMExposureLevel != 0 {
		t.Errorf("LLMExposureLevel = %d, want 0", cfg.LLMExposureLevel)
	}

	// 边界检查
	if err := engine.SetExposureLevel("im", 6); err == nil {
		t.Error("expected error for level > 5")
	}
	if err := engine.SetExposureLevel("im", -1); err == nil {
		t.Error("expected error for level < 0")
	}
	if err := engine.SetExposureLevel("unknown_channel", 3); err == nil {
		t.Error("expected error for unknown channel")
	}
}

// 5. TestSingularityBudgetCalculation — 预算计算
func TestSingularityBudgetCalculation(t *testing.T) {
	engine, db := newTestSingularityEngine(t)

	// 插入入站数据
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, preview, hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		"2024-01-01T00:00:00Z", "inbound", "sender", "pass", "", "", "", 1.0, "", "", "")

	budget := engine.GetBudget()
	if budget == nil {
		t.Fatal("budget should not be nil")
	}

	// 基本字段检查
	if budget.TotalChannels < 1 {
		t.Errorf("TotalChannels = %d, want >= 1", budget.TotalChannels)
	}
	if budget.TotalEngines < 1 {
		t.Errorf("TotalEngines = %d, want >= 1", budget.TotalEngines)
	}
	if budget.MinSingularities < 2 {
		t.Errorf("MinSingularities = %d, want >= 2", budget.MinSingularities)
	}

	// 当前分配: IM=3 + LLM=2 + ToolCall=1 = 6
	expectedAllocated := 3 + 2 + 1
	if budget.AllocatedIM+budget.AllocatedLLM+budget.AllocatedToolCall != expectedAllocated {
		t.Errorf("total allocated = %d, want %d", budget.AllocatedIM+budget.AllocatedLLM+budget.AllocatedToolCall, expectedAllocated)
	}
}

// 6. TestSingularityBudgetOverspend — 超支告警
func TestSingularityBudgetOverspend(t *testing.T) {
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

	// 插入多通道数据以提高 coverage gaps
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason) VALUES (?,?,?,?,?)`,
		"2024-01-01T00:00:00Z", "inbound", "s", "pass", "")
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason) VALUES (?,?,?,?,?)`,
		"2024-01-01T00:00:00Z", "outbound", "s", "pass", "")
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason) VALUES (?,?,?,?,?)`,
		"2024-01-01T00:00:00Z", "llm_request", "s", "pass", "")

	// 所有通道暴露等级 = 0 → allocated = 0 < MinSingularities
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       0,
		LLMExposureLevel:      0,
		ToolCallExposureLevel: 0,
	}

	budget := CalculateBudget(db, cfg)
	if !budget.OverBudget {
		t.Error("expected OverBudget=true when allocated=0")
	}
	if budget.OverBudgetWarning == "" {
		t.Error("expected non-empty OverBudgetWarning")
	}
}

// 7. TestSingularityBudgetMinimum — 拓扑下限
func TestSingularityBudgetMinimum(t *testing.T) {
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

	cfg := SingularityConfig{
		Enabled:          true,
		IMExposureLevel:  1,
		LLMExposureLevel: 1,
	}

	budget := CalculateBudget(db, cfg)
	// 拓扑下限至少为 2
	if budget.MinSingularities < 2 {
		t.Errorf("MinSingularities = %d, want >= 2", budget.MinSingularities)
	}
}

// 8. TestSingularityPlacementProof — 放置证明完整性
func TestSingularityPlacementProof(t *testing.T) {
	engine, em, db := newTestSingularityWithEnvelope(t)

	// 插入测试数据
	for i := 0; i < 50; i++ {
		action := "pass"
		if i < 10 {
			action = "block"
		}
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, preview, hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			"2024-01-01T00:00:00Z", "inbound", "sender", action, "", "", "", 1.0, "", "", "")
	}

	proof := engine.RecommendPlacement()
	if proof == nil {
		t.Fatal("proof should not be nil")
	}

	// 验证帕累托最优
	if !proof.ParetoOptimal {
		t.Error("expected ParetoOptimal=true")
	}

	// 验证推荐通道
	validChannels := map[string]bool{"im": true, "llm": true, "toolcall": true}
	if !validChannels[proof.RecommendedChannel] {
		t.Errorf("invalid recommended channel: %q", proof.RecommendedChannel)
	}

	// 验证推荐等级
	if proof.RecommendedLevel < 0 || proof.RecommendedLevel > 5 {
		t.Errorf("recommended level out of range: %d", proof.RecommendedLevel)
	}

	// 验证信封已生成
	stats := em.Stats()
	total := 0
	if v, ok := stats["total"].(int); ok {
		total = v
	} else if v, ok := stats["total"].(int64); ok {
		total = int(v)
	}
	if total < 1 {
		t.Errorf("expected at least 1 envelope from placement, got %d", total)
	}
}

// 9. TestSingularityConfig — 配置CRUD
func TestSingularityConfig(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)

	// 读取初始配置
	cfg := engine.GetConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if cfg.IMExposureLevel != 3 {
		t.Errorf("IMExposureLevel = %d, want 3", cfg.IMExposureLevel)
	}

	// 更新配置
	engine.UpdateConfig(SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       5,
		LLMExposureLevel:      4,
		ToolCallExposureLevel: 3,
	})

	cfg = engine.GetConfig()
	if cfg.IMExposureLevel != 5 {
		t.Errorf("updated IMExposureLevel = %d, want 5", cfg.IMExposureLevel)
	}
	if cfg.LLMExposureLevel != 4 {
		t.Errorf("updated LLMExposureLevel = %d, want 4", cfg.LLMExposureLevel)
	}
	if cfg.ToolCallExposureLevel != 3 {
		t.Errorf("updated ToolCallExposureLevel = %d, want 3", cfg.ToolCallExposureLevel)
	}

	// SetExposureLevel 单独更新
	engine.SetExposureLevel("llm", 1)
	cfg = engine.GetConfig()
	if cfg.LLMExposureLevel != 1 {
		t.Errorf("after SetExposureLevel: LLMExposureLevel = %d, want 1", cfg.LLMExposureLevel)
	}
}

// 10. TestSingularityHistory — 历史记录
func TestSingularityHistory(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)

	// 设置几个等级（每次设置会记录历史）
	engine.SetExposureLevel("im", 4)
	engine.SetExposureLevel("llm", 3)
	engine.SetExposureLevel("toolcall", 2)

	history := engine.GetHistory(10)
	if len(history) < 3 {
		t.Errorf("expected at least 3 history entries, got %d", len(history))
	}

	// 检查最近记录
	for _, h := range history {
		if h.ID == "" {
			t.Error("history ID should not be empty")
		}
		if h.Channel == "" {
			t.Error("history Channel should not be empty")
		}
		if h.Timestamp == "" {
			t.Error("history Timestamp should not be empty")
		}
		if h.Action != "set_level" && h.Action != "expose" && h.Action != "recommend" {
			t.Errorf("unexpected action: %q", h.Action)
		}
	}
}

// 11. TestSingularityTemplateCount — 模板数量 >= 10
func TestSingularityTemplateCount(t *testing.T) {
	if len(defaultExposureTemplates) < 10 {
		t.Errorf("expected at least 10 default templates, got %d", len(defaultExposureTemplates))
	}

	// 检查通道覆盖
	channelCount := map[string]int{}
	levelCount := map[int]int{}
	for _, tpl := range defaultExposureTemplates {
		channelCount[tpl.Channel]++
		levelCount[tpl.Level]++
		if tpl.Name == "" {
			t.Error("template Name should not be empty")
		}
		if tpl.Content == "" {
			t.Error("template Content should not be empty")
		}
		if tpl.Description == "" {
			t.Error("template Description should not be empty")
		}
	}

	// 每个通道至少 3 个模板
	for _, ch := range []string{"im", "llm", "toolcall"} {
		if channelCount[ch] < 3 {
			t.Errorf("channel %q has %d templates, want >= 3", ch, channelCount[ch])
		}
	}

	// 至少覆盖 3 个不同等级
	if len(levelCount) < 3 {
		t.Errorf("expected at least 3 different levels, got %d", len(levelCount))
	}
}

// 12. TestSingularityBudgetJSON — 预算 JSON 序列化
func TestSingularityBudgetJSON(t *testing.T) {
	engine, _ := newTestSingularityEngine(t)
	budget := engine.GetBudget()

	data, err := json.Marshal(budget)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	var decoded SingularityBudget
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if decoded.MinSingularities != budget.MinSingularities {
		t.Errorf("round-trip MinSingularities = %d, want %d", decoded.MinSingularities, budget.MinSingularities)
	}
	if decoded.AllocatedIM != budget.AllocatedIM {
		t.Errorf("round-trip AllocatedIM = %d, want %d", decoded.AllocatedIM, budget.AllocatedIM)
	}
}
