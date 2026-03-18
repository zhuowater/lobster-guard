// taint_reversal_test.go — 污染链逆转引擎测试
// lobster-guard v20.2
package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestReversalEngine 创建测试用逆转引擎（SQLite in-memory）
func newTestReversalEngine(t *testing.T, cfg TaintReversalConfig, taintCfg *TaintConfig) (*TaintReversalEngine, *TaintTracker, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	// 限制为单连接，避免 in-memory SQLite 多连接导致表不可见
	db.SetMaxOpenConns(1)

	// 默认 taint config
	tc := TaintConfig{Enabled: true, Action: "warn", TTLMinutes: 30}
	if taintCfg != nil {
		tc = *taintCfg
	}
	tt := NewTaintTracker(db, tc)

	// 默认 reversal config
	if !cfg.Enabled {
		cfg.Enabled = true
	}
	if cfg.Mode == "" {
		cfg.Mode = "soft"
	}

	// 创建执行信封管理器（测试用）
	db.Exec(`CREATE TABLE IF NOT EXISTS execution_envelopes (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		domain TEXT NOT NULL,
		request_hash TEXT NOT NULL,
		decision TEXT NOT NULL,
		rules_json TEXT NOT NULL,
		sender_id TEXT DEFAULT '',
		nonce TEXT NOT NULL,
		prev_hash TEXT DEFAULT '',
		content_hash TEXT NOT NULL,
		signature TEXT NOT NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_envelopes_trace ON execution_envelopes(trace_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_envelopes_ts ON execution_envelopes(timestamp)`)
	em := NewEnvelopeManager(db, "test-secret-key")

	engine := NewTaintReversalEngine(db, tt, em, cfg)
	return engine, tt, db
}

// ============================================================
// 测试用例 (>= 12 个)
// ============================================================

// 1. TestReversalSoftMode — 软逆转追加
func TestReversalSoftMode(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-soft-001"
	tt.MarkTainted(traceID, "用户手机号是 13800138000", "inbound")

	original := "这是一个普通的响应内容"
	reversed, record := engine.Reverse(traceID, original)

	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if record.Mode != "soft" {
		t.Errorf("期望 mode=soft, 得到 %s", record.Mode)
	}
	if !strings.HasPrefix(reversed, original) {
		t.Error("soft 模式应保留原始内容")
	}
	if len(reversed) <= len(original) {
		t.Error("soft 模式应追加逆转提示")
	}
	if !strings.Contains(reversed, "[安全提示]") && !strings.Contains(reversed, "[数据声明]") {
		t.Error("soft 模式应包含安全提示")
	}
	if record.OriginalLen != len(original) {
		t.Errorf("OriginalLen 不正确: %d vs %d", record.OriginalLen, len(original))
	}
	if record.ReversedLen != len(reversed) {
		t.Errorf("ReversedLen 不正确: %d vs %d", record.ReversedLen, len(reversed))
	}
}

// 2. TestReversalHardMode — 硬替换
func TestReversalHardMode(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "hard"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-hard-001"
	tt.MarkTainted(traceID, "用户手机号是 13800138000", "inbound")

	original := "这是包含敏感信息的响应"
	reversed, record := engine.Reverse(traceID, original)

	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if record.Mode != "hard" {
		t.Errorf("期望 mode=hard, 得到 %s", record.Mode)
	}
	if strings.Contains(reversed, original) {
		t.Error("hard 模式不应保留原始内容")
	}
	if !strings.Contains(reversed, "安全网关") && !strings.Contains(reversed, "拦截") && !strings.Contains(reversed, "替换") {
		t.Error("hard 模式应包含安全拦截提示")
	}
}

// 3. TestReversalStealthMode — 隐写标记
func TestReversalStealthMode(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "stealth"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-stealth-001"
	tt.MarkTainted(traceID, "用户手机号是 13800138000", "inbound")

	original := "这是一个普通的响应"
	reversed, record := engine.Reverse(traceID, original)

	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if record.Mode != "stealth" {
		t.Errorf("期望 mode=stealth, 得到 %s", record.Mode)
	}
	if !strings.HasPrefix(reversed, original) {
		t.Error("stealth 模式应保留原始内容")
	}
	if !strings.Contains(reversed, "\u200B") || !strings.Contains(reversed, "\u200C") {
		t.Error("stealth 模式应包含零宽字符标记")
	}
	if !strings.Contains(reversed, "[TAINT:") {
		t.Error("stealth 模式应包含 TAINT 标记")
	}
}

// 4. TestReversalPIITemplate — PII 模板匹配
func TestReversalPIITemplate(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-pii-001"
	tt.MarkTainted(traceID, "身份证号 110101199001011234", "inbound")

	_, record := engine.Reverse(traceID, "响应内容")

	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if !strings.HasPrefix(record.TemplateID, "pii-") {
		t.Errorf("期望 PII 模板, 得到 template_id=%s", record.TemplateID)
	}
	found := false
	for _, label := range record.TaintLabels {
		if label == "PII-TAINTED" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("标签应包含 PII-TAINTED, 得到 %v", record.TaintLabels)
	}
}

// 5. TestReversalCredTemplate — 凭据模板匹配
func TestReversalCredTemplate(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-cred-001"
	tt.MarkTainted(traceID, "密钥是 sk-abcdefghijklmnopqrstuvwxyz1234567890", "inbound")

	_, record := engine.Reverse(traceID, "响应内容")

	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if !strings.HasPrefix(record.TemplateID, "cred-") {
		t.Errorf("期望凭据模板, 得到 template_id=%s", record.TemplateID)
	}
}

// 6. TestReversalNoTaint — 无污染不逆转
func TestReversalNoTaint(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	original := "这是一个干净的响应"
	reversed, record := engine.Reverse("trace-clean-001", original)

	if record != nil {
		t.Error("无污染不应生成逆转记录")
	}
	if reversed != original {
		t.Error("无污染应返回原始内容")
	}
}

// 7. TestReversalRecord — 记录完整性
func TestReversalRecord(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-record-001"
	tt.MarkTainted(traceID, "电话 13912345678", "inbound")

	original := "响应内容 ABCDEF"
	reversed, record := engine.Reverse(traceID, original)

	if record == nil {
		t.Fatal("应返回逆转记录")
	}

	if record.ID == "" {
		t.Error("记录 ID 不应为空")
	}
	if record.TraceID != traceID {
		t.Errorf("TraceID 不匹配: %s vs %s", record.TraceID, traceID)
	}
	if record.Timestamp.IsZero() {
		t.Error("Timestamp 不应为零值")
	}
	if len(record.TaintLabels) == 0 {
		t.Error("TaintLabels 不应为空")
	}
	if record.TemplateID == "" {
		t.Error("TemplateID 不应为空")
	}
	if record.Mode != "soft" {
		t.Errorf("Mode 不正确: %s", record.Mode)
	}
	if record.OriginalLen != len(original) {
		t.Errorf("OriginalLen 不正确: %d", record.OriginalLen)
	}
	if record.ReversedLen != len(reversed) {
		t.Errorf("ReversedLen 不正确: %d", record.ReversedLen)
	}
	if !record.Effective {
		t.Error("Effective 应为 true")
	}

	// 等待异步持久化
	time.Sleep(100 * time.Millisecond)

	records := engine.ListRecords(10)
	if len(records) == 0 {
		t.Error("数据库中应有记录")
	}
}

// 8. TestReversalEnvelope — 执行信封生成
func TestReversalEnvelope(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-envelope-001"
	tt.MarkTainted(traceID, "密码 password: abc123", "inbound")

	_, record := engine.Reverse(traceID, "响应")

	if record == nil {
		t.Fatal("应返回逆转记录")
	}

	// 等写入
	time.Sleep(100 * time.Millisecond)

	envelopes, err := engine.envelopeMgr.ListByTrace(traceID)
	if err != nil {
		t.Fatalf("查询信封失败: %v", err)
	}

	found := false
	for _, env := range envelopes {
		if env.Domain == "taint_reversal" {
			found = true
			if env.Decision != "reversal_soft" {
				t.Errorf("信封 decision 不正确: %s", env.Decision)
			}
			hasTemplate := false
			for _, r := range env.Rules {
				if strings.HasPrefix(r, "cred-") || strings.HasPrefix(r, "pii-") || r == "taint_reversal" {
					hasTemplate = true
					break
				}
			}
			if !hasTemplate {
				t.Errorf("信封 rules 应包含模板 ID, 得到 %v", env.Rules)
			}
			break
		}
	}
	if !found {
		t.Error("应生成 taint_reversal 域的执行信封")
	}
}

// 9. TestReversalCustomTemplate — 自定义模板
func TestReversalCustomTemplate(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	err := engine.AddTemplate(ReversalTemplate{
		ID:          "custom-pii-1",
		Name:        "自定义PII逆转",
		TaintLabels: []string{"PII-TAINTED"},
		Mode:        "soft",
		Content:     "\n\n[自定义安全提示] 此内容已由自定义逆转模板处理。",
		Description: "测试自定义模板",
		Priority:    100,
	})
	if err != nil {
		t.Fatalf("添加模板失败: %v", err)
	}

	traceID := "trace-custom-001"
	tt.MarkTainted(traceID, "手机 13800138000", "inbound")

	reversed, record := engine.Reverse(traceID, "响应")
	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if record.TemplateID != "custom-pii-1" {
		t.Errorf("应使用自定义模板, 得到 %s", record.TemplateID)
	}
	if !strings.Contains(reversed, "自定义安全提示") {
		t.Error("应包含自定义模板内容")
	}
}

// 10. TestReversalStats — 统计
func TestReversalStats(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	stats := engine.Stats()
	if stats["total_reversals"].(int64) != 0 {
		t.Error("初始应为 0 逆转")
	}
	if stats["enabled"].(bool) != true {
		t.Error("应已启用")
	}

	tt.MarkTainted("trace-stats-001", "手机 13900139001", "inbound")
	engine.Reverse("trace-stats-001", "响应1")

	tt.MarkTainted("trace-stats-002", "手机 13900139002", "inbound")
	engine.Reverse("trace-stats-002", "响应2")

	stats = engine.Stats()
	if stats["total_reversals"].(int64) != 2 {
		t.Errorf("期望 2 次逆转, 得到 %d", stats["total_reversals"].(int64))
	}
	if stats["total_soft"].(int64) != 2 {
		t.Errorf("期望 2 次 soft, 得到 %d", stats["total_soft"].(int64))
	}
	if stats["template_count"].(int) < 10 {
		t.Errorf("模板数应 >= 10, 得到 %d", stats["template_count"].(int))
	}
}

// 11. TestReversalMultiLabel — 多标签优先级
func TestReversalMultiLabel(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-multi-001"
	tt.MarkTainted(traceID, "手机 13800138000 密钥 sk-abcdefghijklmnopqrstuvwxyz1234567890", "inbound")

	entry := tt.GetTaint(traceID)
	if entry == nil {
		t.Fatal("应有污染条目")
	}
	if len(entry.Labels) < 2 {
		t.Fatalf("应有多个标签, 得到 %v", entry.Labels)
	}

	_, record := engine.Reverse(traceID, "多标签响应")
	if record == nil {
		t.Fatal("应返回逆转记录")
	}
	if record.TemplateID == "" {
		t.Error("应选中某个模板")
	}
	if len(record.TaintLabels) < 2 {
		t.Errorf("记录应包含多个标签, 得到 %v", record.TaintLabels)
	}
}

// 12. TestReversalConfigSwitch — 模式切换
func TestReversalConfigSwitch(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID1 := "trace-switch-001"
	tt.MarkTainted(traceID1, "手机 13800138001", "inbound")

	original := "响应内容"
	reversed1, record1 := engine.Reverse(traceID1, original)
	if record1 == nil || record1.Mode != "soft" {
		t.Error("应为 soft 模式")
	}
	if !strings.HasPrefix(reversed1, original) {
		t.Error("soft 模式应保留原始内容")
	}

	// 切换到 hard 模式
	engine.UpdateConfig(TaintReversalConfig{Enabled: true, Mode: "hard"})
	cfg := engine.GetConfig()
	if cfg.Mode != "hard" {
		t.Errorf("模式应已切换为 hard, 得到 %s", cfg.Mode)
	}

	traceID2 := "trace-switch-002"
	tt.MarkTainted(traceID2, "手机 13800138002", "inbound")

	reversed2, record2 := engine.Reverse(traceID2, original)
	if record2 == nil || record2.Mode != "hard" {
		t.Error("应为 hard 模式")
	}
	if strings.Contains(reversed2, original) {
		t.Error("hard 模式不应保留原始内容")
	}

	// 切换到 stealth
	engine.UpdateConfig(TaintReversalConfig{Enabled: true, Mode: "stealth"})

	traceID3 := "trace-switch-003"
	tt.MarkTainted(traceID3, "手机 13800138003", "inbound")

	reversed3, record3 := engine.Reverse(traceID3, original)
	if record3 == nil || record3.Mode != "stealth" {
		t.Error("应为 stealth 模式")
	}
	if !strings.HasPrefix(reversed3, original) {
		t.Error("stealth 模式应保留原始内容")
	}
	if !strings.Contains(reversed3, "\u200B") {
		t.Error("stealth 模式应包含零宽字符")
	}
}

// 13. TestReversalDisabled — 禁用状态
func TestReversalDisabled(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: false, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	engine.UpdateConfig(TaintReversalConfig{Enabled: false})

	traceID := "trace-disabled-001"
	tt.MarkTainted(traceID, "手机 13800138000", "inbound")

	original := "响应内容"
	reversed, record := engine.Reverse(traceID, original)

	if record != nil {
		t.Error("禁用状态不应逆转")
	}
	if reversed != original {
		t.Error("禁用状态应返回原始内容")
	}
}

// 14. TestReversalTemplateCount — 内置模板数量 >= 10
func TestReversalTemplateCount(t *testing.T) {
	if len(defaultReversalTemplates) < 10 {
		t.Errorf("内置模板数应 >= 10, 得到 %d", len(defaultReversalTemplates))
	}

	modes := map[string]int{}
	for _, tmpl := range defaultReversalTemplates {
		modes[tmpl.Mode]++
	}
	for _, mode := range []string{"soft", "hard", "stealth"} {
		if modes[mode] == 0 {
			t.Errorf("模式 %s 没有对应的模板", mode)
		}
	}
}

// 15. TestReversalAddTemplateValidation — 模板验证
func TestReversalAddTemplateValidation(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	if err := engine.AddTemplate(ReversalTemplate{}); err == nil {
		t.Error("空 ID 应返回错误")
	}
	if err := engine.AddTemplate(ReversalTemplate{ID: "x"}); err == nil {
		t.Error("空 name 应返回错误")
	}
	if err := engine.AddTemplate(ReversalTemplate{ID: "x", Name: "x"}); err == nil {
		t.Error("空标签应返回错误")
	}
	if err := engine.AddTemplate(ReversalTemplate{ID: "x", Name: "x", TaintLabels: []string{"PII-TAINTED"}, Mode: "invalid"}); err == nil {
		t.Error("无效 mode 应返回错误")
	}
	if err := engine.AddTemplate(ReversalTemplate{ID: "x", Name: "x", TaintLabels: []string{"PII-TAINTED"}, Mode: "soft"}); err == nil {
		t.Error("空 content 应返回错误")
	}
	if err := engine.AddTemplate(ReversalTemplate{
		ID: "valid-1", Name: "Valid", TaintLabels: []string{"PII-TAINTED"}, Mode: "soft", Content: "test",
	}); err != nil {
		t.Errorf("有效模板不应出错: %v", err)
	}
}

// 16. TestReversalAPIStats — API: GET /api/v1/reversal/stats
func TestReversalAPIStats(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	api := &ManagementAPI{reversalEngine: engine}

	req := httptest.NewRequest("GET", "/api/v1/reversal/stats", nil)
	w := httptest.NewRecorder()
	api.handleReversalStats(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["enabled"] != true {
		t.Error("enabled 应为 true")
	}
}

// 17. TestReversalAPITest — API: POST /api/v1/reversal/test
func TestReversalAPITest(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-api-test-001"
	tt.MarkTainted(traceID, "手机 13800138000", "inbound")

	api := &ManagementAPI{reversalEngine: engine}

	body := `{"trace_id":"trace-api-test-001","response":"测试响应"}`
	req := httptest.NewRequest("POST", "/api/v1/reversal/test", strings.NewReader(body))
	w := httptest.NewRecorder()
	api.handleReversalTest(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["reversed"] != true {
		t.Error("应返回 reversed=true")
	}
	if resp["template_id"] == nil || resp["template_id"] == "" {
		t.Error("应返回 template_id")
	}
}

// 18. TestReversalAPIConfigUpdate — API: PUT /api/v1/reversal/config
func TestReversalAPIConfigUpdate(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	api := &ManagementAPI{reversalEngine: engine}

	body := `{"enabled":true,"mode":"hard"}`
	req := httptest.NewRequest("PUT", "/api/v1/reversal/config", strings.NewReader(body))
	w := httptest.NewRecorder()
	api.handleReversalConfigUpdate(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, 得到 %d", w.Code)
	}

	cfg := engine.GetConfig()
	if cfg.Mode != "hard" {
		t.Errorf("模式应为 hard, 得到 %s", cfg.Mode)
	}

	body2 := `{"enabled":true,"mode":"invalid"}`
	req2 := httptest.NewRequest("PUT", "/api/v1/reversal/config", strings.NewReader(body2))
	w2 := httptest.NewRecorder()
	api.handleReversalConfigUpdate(w2, req2)

	if w2.Code != 400 {
		t.Errorf("无效 mode 应返回 400, 得到 %d", w2.Code)
	}
}

// 19. TestReversalAPITemplates — API: GET/POST templates
func TestReversalAPITemplates(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	api := &ManagementAPI{reversalEngine: engine}

	// GET
	req := httptest.NewRequest("GET", "/api/v1/reversal/templates", nil)
	w := httptest.NewRecorder()
	api.handleReversalTemplates(w, req)

	if w.Code != 200 {
		t.Errorf("期望 200, 得到 %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total < 10 {
		t.Errorf("模板数应 >= 10, 得到 %d", total)
	}

	// POST
	addBody := `{"id":"api-test-1","name":"API测试模板","taint_labels":["PII-TAINTED"],"mode":"soft","content":"[API测试]","description":"test"}`
	req2 := httptest.NewRequest("POST", "/api/v1/reversal/templates", strings.NewReader(addBody))
	w2 := httptest.NewRecorder()
	api.handleReversalTemplatesAdd(w2, req2)

	if w2.Code != 200 {
		t.Errorf("期望 200, 得到 %d", w2.Code)
	}

	// 验证新增成功
	templates := engine.GetTemplates()
	found := false
	for _, tmpl := range templates {
		if tmpl.ID == "api-test-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应能通过 API 添加模板")
	}
}

// 20. TestReversalEmptyTraceID — 空 trace_id
func TestReversalEmptyTraceID(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()

	original := "响应"
	reversed, record := engine.Reverse("", original)

	if record != nil {
		t.Error("空 trace_id 不应逆转")
	}
	if reversed != original {
		t.Error("空 trace_id 应返回原始内容")
	}
}
