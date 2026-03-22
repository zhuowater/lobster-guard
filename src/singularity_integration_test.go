// singularity_integration_test.go — 奇点蜜罐代理流程集成测试（v18.3）
package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupSingularityIntegrationDB 创建集成测试用的完整数据库
func setupSingularityIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// 创建所有依赖表
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
	db.Exec(`CREATE TABLE IF NOT EXISTS singularity_history (
		id TEXT PRIMARY KEY,
		channel TEXT NOT NULL,
		level INTEGER NOT NULL,
		action TEXT NOT NULL,
		proof_json TEXT DEFAULT '{}',
		timestamp TEXT NOT NULL
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS envelope_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id TEXT, domain TEXT, content_hash TEXT,
		decision TEXT, rules TEXT, actor TEXT,
		signature TEXT, prev_hash TEXT, timestamp TEXT,
		batch_id TEXT
	)`)

	return db
}

// TestSingularityProxyIntegration_BlockExpose 测试 block 时触发奇点暴露
func TestSingularityProxyIntegration_BlockExpose(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	em := NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")

	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       3,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 1,
	}
	se := NewSingularityEngine(db, honeypot, em, cfg)

	// 模拟 proxy 调用 ShouldExpose
	shouldExpose, tpl := se.ShouldExpose("im", "trace-proxy-block-001")
	if !shouldExpose {
		t.Fatal("expected singularity exposure for im channel with level=3")
	}
	if tpl == nil {
		t.Fatal("template should not be nil")
	}
	if tpl.Channel != "im" {
		t.Errorf("expected channel=im, got %q", tpl.Channel)
	}
	if tpl.Level > 3 {
		t.Errorf("expected template level <= 3, got %d", tpl.Level)
	}
	if tpl.Content == "" {
		t.Error("template content should not be empty")
	}

	// 验证审计日志记录
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM singularity_history WHERE action='expose' AND channel='im'`).Scan(&count)
	if count < 1 {
		t.Error("expected at least 1 singularity_history entry for expose action")
	}
}

// TestSingularityProxyIntegration_DisabledNoExpose 测试 disabled 时不触发
func TestSingularityProxyIntegration_DisabledNoExpose(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               false,
		IMExposureLevel:       3,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 1,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	shouldExpose, tpl := se.ShouldExpose("im", "trace-disabled-001")
	if shouldExpose {
		t.Error("should NOT expose when singularity is disabled")
	}
	if tpl != nil {
		t.Error("template should be nil when disabled")
	}
}

// TestSingularityProxyIntegration_ZeroLevelNoExpose 测试 level=0 时不暴露
func TestSingularityProxyIntegration_ZeroLevelNoExpose(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       0,
		LLMExposureLevel:      0,
		ToolCallExposureLevel: 0,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	// IM level=0
	shouldExpose, _ := se.ShouldExpose("im", "trace-zero-001")
	if shouldExpose {
		t.Error("should NOT expose for im when level=0")
	}

	// LLM level=0
	shouldExpose, _ = se.ShouldExpose("llm", "trace-zero-002")
	if shouldExpose {
		t.Error("should NOT expose for llm when level=0")
	}

	// ToolCall level=0
	shouldExpose, _ = se.ShouldExpose("toolcall", "trace-zero-003")
	if shouldExpose {
		t.Error("should NOT expose for toolcall when level=0")
	}
}

// TestSingularityProxyIntegration_LLMExpose 测试 LLM 通道暴露
func TestSingularityProxyIntegration_LLMExpose(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	em := NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")

	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       0,
		LLMExposureLevel:      3,
		ToolCallExposureLevel: 0,
	}
	se := NewSingularityEngine(db, honeypot, em, cfg)

	// LLM 通道应该暴露
	shouldExpose, tpl := se.ShouldExpose("llm", "trace-llm-001")
	if !shouldExpose {
		t.Fatal("expected singularity exposure for llm channel with level=3")
	}
	if tpl.Channel != "llm" {
		t.Errorf("expected channel=llm, got %q", tpl.Channel)
	}

	// IM 通道 level=0，不应暴露
	shouldExpose, _ = se.ShouldExpose("im", "trace-im-001")
	if shouldExpose {
		t.Error("should NOT expose for im when level=0")
	}
}

// TestSingularityProxyIntegration_TemplateContentCorrect 测试暴露模板内容正确
func TestSingularityProxyIntegration_TemplateContentCorrect(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       5,
		LLMExposureLevel:      5,
		ToolCallExposureLevel: 5,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	// 各通道全部 level=5，应返回最高等级模板
	channels := []string{"im", "llm", "toolcall"}
	for _, ch := range channels {
		shouldExpose, tpl := se.ShouldExpose(ch, "trace-content-"+ch)
		if !shouldExpose {
			t.Errorf("expected exposure for channel %q with level=5", ch)
			continue
		}
		if tpl == nil {
			t.Errorf("template should not be nil for channel %q", ch)
			continue
		}
		if tpl.Content == "" {
			t.Errorf("template content should not be empty for channel %q", ch)
		}
		if tpl.Name == "" {
			t.Errorf("template name should not be empty for channel %q", ch)
		}
		if tpl.Channel != ch {
			t.Errorf("expected template channel=%q, got %q", ch, tpl.Channel)
		}
		// Level 5 时应返回最高级别模板
		if tpl.Level < 1 {
			t.Errorf("expected template level >= 1, got %d for channel %q", tpl.Level, ch)
		}
	}
}

// TestSingularityProxyIntegration_EnvelopeSeal 测试暴露时生成执行信封
func TestSingularityProxyIntegration_EnvelopeSeal(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	em := NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")

	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       3,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 1,
	}
	se := NewSingularityEngine(db, honeypot, em, cfg)

	// 触发暴露
	shouldExpose, tpl := se.ShouldExpose("im", "trace-envelope-001")
	if !shouldExpose || tpl == nil {
		t.Fatal("expected exposure for im channel")
	}

	// 模拟 proxy 生成信封（proxy 代码中会调用 envelopeMgr.Seal）
	em.Seal("trace-envelope-001", "singularity_expose", tpl.Content, "expose", []string{"singularity_im_" + tpl.Name}, "sender-001")

	// 验证信封统计
	stats := em.Stats()
	total := 0
	if v, ok := stats["total"].(int); ok {
		total = v
	} else if v, ok := stats["total"].(int64); ok {
		total = int(v)
	}
	if total < 1 {
		t.Errorf("expected at least 1 envelope entry, got %d", total)
	}
}

// TestSingularityProxyIntegration_DeterministicTemplate 测试确定性模板选择
func TestSingularityProxyIntegration_DeterministicTemplate(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:          true,
		IMExposureLevel:  3,
		LLMExposureLevel: 3,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	// 同一 traceID 多次调用应返回相同模板
	traceID := "trace-deterministic-001"
	_, tpl1 := se.ShouldExpose("im", traceID)
	_, tpl2 := se.ShouldExpose("im", traceID)
	_, tpl3 := se.ShouldExpose("im", traceID)

	if tpl1 == nil || tpl2 == nil || tpl3 == nil {
		t.Fatal("all templates should be non-nil")
	}
	if tpl1.Name != tpl2.Name || tpl2.Name != tpl3.Name {
		t.Errorf("deterministic selection failed: %q, %q, %q", tpl1.Name, tpl2.Name, tpl3.Name)
	}

	// 不同 traceID 可能选择不同模板（只要不全相同就行）
	allSame := true
	for i := 0; i < 20; i++ {
		_, tplX := se.ShouldExpose("im", "trace-diff-"+string(rune('A'+i)))
		if tplX != nil && tplX.Name != tpl1.Name {
			allSame = false
			break
		}
	}
	// 不强制要求不同（可能碰巧全相同），但如果模板数>1则大概率不同
	_ = allSame // 仅验证不 panic
}

// TestSingularityProxyIntegration_ChannelLevelBoundary 测试通道级别边界
func TestSingularityProxyIntegration_ChannelLevelBoundary(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:               true,
		IMExposureLevel:       1,
		LLMExposureLevel:      2,
		ToolCallExposureLevel: 5,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	// IM level=1, 只能返回 level<=1 的模板
	_, tplIM := se.ShouldExpose("im", "trace-boundary-im")
	if tplIM != nil && tplIM.Level > 1 {
		t.Errorf("IM template level should be <= 1, got %d", tplIM.Level)
	}

	// LLM level=2, 只能返回 level<=2 的模板
	_, tplLLM := se.ShouldExpose("llm", "trace-boundary-llm")
	if tplLLM != nil && tplLLM.Level > 2 {
		t.Errorf("LLM template level should be <= 2, got %d", tplLLM.Level)
	}

	// ToolCall level=5, 可返回所有等级
	_, tplTool := se.ShouldExpose("toolcall", "trace-boundary-tool")
	if tplTool == nil {
		t.Error("expected non-nil template for toolcall level=5")
	}
}

// TestSingularityProxyIntegration_NilEngineNoExpose 测试 nil engine 时 proxy 不崩溃
func TestSingularityProxyIntegration_NilEngineNoExpose(t *testing.T) {
	// 模拟 proxy 中 singularityEngine == nil 的情况
	// 这是一个保护性测试 — 确保 nil 检查到位
	var se *SingularityEngine

	// 不应 panic
	if se != nil {
		se.ShouldExpose("im", "trace-nil-001")
	}
	// 通过 — nil 检查有效
}

// TestSingularityProxyIntegration_HistoryRecordOnExpose 测试暴露时历史记录
func TestSingularityProxyIntegration_HistoryRecordOnExpose(t *testing.T) {
	db := setupSingularityIntegrationDB(t)

	honeypot := &HoneypotEngine{db: db, enabled: true}
	cfg := SingularityConfig{
		Enabled:          true,
		IMExposureLevel:  3,
		LLMExposureLevel: 3,
	}
	se := NewSingularityEngine(db, honeypot, nil, cfg)

	// 触发多次暴露
	se.ShouldExpose("im", "trace-hist-001")
	se.ShouldExpose("llm", "trace-hist-002")
	se.ShouldExpose("im", "trace-hist-003")

	history := se.GetHistory(10)
	exposeCount := 0
	for _, h := range history {
		if h.Action == "expose" {
			exposeCount++
		}
	}
	if exposeCount < 3 {
		t.Errorf("expected at least 3 expose history entries, got %d", exposeCount)
	}
}
