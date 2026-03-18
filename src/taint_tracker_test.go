// taint_tracker_test.go — 污染追踪引擎测试
// lobster-guard v20.1
package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestTaintTracker 创建测试用 TaintTracker（SQLite in-memory）
func newTestTaintTracker(t *testing.T, cfg TaintConfig) (*TaintTracker, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if !cfg.Enabled {
		cfg.Enabled = true
	}
	if cfg.Action == "" {
		cfg.Action = "block"
	}
	if cfg.TTLMinutes <= 0 {
		cfg.TTLMinutes = 30
	}
	tt := NewTaintTracker(db, cfg)
	return tt, db
}

// containsStr 辅助函数：检查 slice 是否包含某个字符串
func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// ============================================================
// 1. TestTaintMarkPhone — 手机号标记 PII
// ============================================================
func TestTaintMarkPhone(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-phone-001", "请把钱转到13812345678这个号码", "inbound")
	if entry == nil {
		t.Fatal("应检测到手机号并标记污染")
	}
	if !containsStr(entry.Labels, TaintPII) {
		t.Errorf("标签应包含 PII-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "phone_cn") {
		t.Errorf("SourceDetail 应包含 phone_cn, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 2. TestTaintMarkIDCard — 身份证标记 PII
// ============================================================
func TestTaintMarkIDCard(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-idcard-001", "我的身份证号是110101199001011234", "inbound")
	if entry == nil {
		t.Fatal("应检测到身份证号并标记污染")
	}
	if !containsStr(entry.Labels, TaintPII) {
		t.Errorf("标签应包含 PII-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "id_card_cn") {
		t.Errorf("SourceDetail 应包含 id_card_cn, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 3. TestTaintMarkBankCard — 银行卡标记 PII
// ============================================================
func TestTaintMarkBankCard(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-bank-001", "银行卡号 6222021234567890123", "inbound")
	if entry == nil {
		t.Fatal("应检测到银行卡号并标记污染")
	}
	if !containsStr(entry.Labels, TaintPII) {
		t.Errorf("标签应包含 PII-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "bank_card") {
		t.Errorf("SourceDetail 应包含 bank_card, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 4. TestTaintMarkEmail — 邮箱标记 PII
// ============================================================
func TestTaintMarkEmail(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-email-001", "发送到 zhangzhuo@qianxin.com", "inbound")
	if entry == nil {
		t.Fatal("应检测到邮箱并标记污染")
	}
	if !containsStr(entry.Labels, TaintPII) {
		t.Errorf("标签应包含 PII-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "email") {
		t.Errorf("SourceDetail 应包含 email, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 5. TestTaintMarkCredential — 凭据标记 CREDENTIAL（私钥）
// ============================================================
func TestTaintMarkCredential(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-cred-001", "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBg...", "inbound")
	if entry == nil {
		t.Fatal("应检测到私钥并标记污染")
	}
	if !containsStr(entry.Labels, TaintCredential) {
		t.Errorf("标签应包含 CREDENTIAL-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "private_key") {
		t.Errorf("SourceDetail 应包含 private_key, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 6. TestTaintMarkAPIKey — API Key 标记 CREDENTIAL
// ============================================================
func TestTaintMarkAPIKey(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-apikey-001", "我的 key 是 sk-abcdefghijklmnopqrstuvwxyz12345", "inbound")
	if entry == nil {
		t.Fatal("应检测到 API Key 并标记污染")
	}
	if !containsStr(entry.Labels, TaintCredential) {
		t.Errorf("标签应包含 CREDENTIAL-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "api_key") {
		t.Errorf("SourceDetail 应包含 api_key, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 7. TestTaintMarkPassword — 密码标记 CREDENTIAL
// ============================================================
func TestTaintMarkPassword(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-pwd-001", "数据库 password=MyS3cr3tPass!", "inbound")
	if entry == nil {
		t.Fatal("应检测到密码泄漏并标记污染")
	}
	if !containsStr(entry.Labels, TaintCredential) {
		t.Errorf("标签应包含 CREDENTIAL-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "password_leak") {
		t.Errorf("SourceDetail 应包含 password_leak, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 8. TestTaintMarkClean — 无 PII 文本不标记
// ============================================================
func TestTaintMarkClean(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-clean-001", "今天天气真好，我们去公园散步吧", "inbound")
	if entry != nil {
		t.Errorf("干净文本不应标记污染, got labels: %v", entry.Labels)
	}
}

// ============================================================
// 9. TestTaintPropagate — 传播记录
// ============================================================
func TestTaintPropagate(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-prop-001"
	tt.MarkTainted(traceID, "密码 password=secret123", "inbound")

	// 传播到 llm_request
	tt.Propagate(traceID, "llm_request", "user message forwarded to LLM")

	// 传播到 llm_response
	tt.Propagate(traceID, "llm_response", "LLM response received")

	entry := tt.GetTaint(traceID)
	if entry == nil {
		t.Fatal("应能获取到污染条目")
	}

	// 应该有至少 3 条传播记录（初始标记 + 2 次传播）
	if len(entry.Propagations) < 3 {
		t.Errorf("应有至少 3 条传播记录, got %d", len(entry.Propagations))
	}

	// 验证阶段
	stages := make(map[string]bool)
	for _, p := range entry.Propagations {
		stages[p.Stage] = true
	}
	if !stages["inbound"] {
		t.Error("应有 inbound 阶段")
	}
	if !stages["llm_request"] {
		t.Error("应有 llm_request 阶段")
	}
	if !stages["llm_response"] {
		t.Error("应有 llm_response 阶段")
	}
}

// ============================================================
// 10. TestTaintCheckOutboundBlock — 污染出站阻断
// ============================================================
func TestTaintCheckOutboundBlock(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "block"})
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-block-001"
	tt.MarkTainted(traceID, "发送到 13912345678", "inbound")

	decision := tt.CheckOutbound(traceID)
	if !decision.Tainted {
		t.Error("应检测到污染")
	}
	if decision.Action != "block" {
		t.Errorf("action 应为 block, got: %s", decision.Action)
	}
	if len(decision.Labels) == 0 {
		t.Error("labels 不应为空")
	}
}

// ============================================================
// 11. TestTaintCheckOutboundClean — 无污染出站放行
// ============================================================
func TestTaintCheckOutboundClean(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	decision := tt.CheckOutbound("trace-clean-999")
	if decision.Tainted {
		t.Error("无污染的 trace 不应被标记为 tainted")
	}
	if decision.Action != "pass" {
		t.Errorf("action 应为 pass, got: %s", decision.Action)
	}
}

// ============================================================
// 12. TestTaintMultipleLabels — 多标签（PII+CREDENTIAL同时）
// ============================================================
func TestTaintMultipleLabels(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	// 文本同时包含手机号（PII）和密码（CREDENTIAL）
	entry := tt.MarkTainted("trace-multi-001",
		"联系 13812345678，密码 password=admin123",
		"inbound")
	if entry == nil {
		t.Fatal("应检测到 PII 并标记")
	}
	hasPII := containsStr(entry.Labels, TaintPII)
	hasCred := containsStr(entry.Labels, TaintCredential)
	if !hasPII || !hasCred {
		t.Errorf("应同时包含 PII-TAINTED 和 CREDENTIAL-TAINTED, got: %v", entry.Labels)
	}
	// SourceDetail 应包含两种类型
	if !strings.Contains(entry.SourceDetail, "phone_cn") {
		t.Errorf("SourceDetail 应包含 phone_cn, got: %s", entry.SourceDetail)
	}
	if !strings.Contains(entry.SourceDetail, "password_leak") {
		t.Errorf("SourceDetail 应包含 password_leak, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 13. TestTaintTTLExpiry — 过期清理
// ============================================================
func TestTaintTTLExpiry(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{TTLMinutes: 1})
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-ttl-001"
	entry := tt.MarkTainted(traceID, "电话 13812345678", "inbound")
	if entry == nil {
		t.Fatal("应标记污染")
	}

	// 手动设置时间戳为过去（超过 TTL）
	tt.mu.Lock()
	if e, ok := tt.active[traceID]; ok {
		e.Timestamp = time.Now().Add(-2 * time.Minute)
	}
	tt.mu.Unlock()

	// 执行清理
	tt.CleanupNow()

	// 内存中应已删除
	tt.mu.RLock()
	_, exists := tt.active[traceID]
	tt.mu.RUnlock()
	if exists {
		t.Error("过期条目应从内存中清理")
	}

	// 出站检查应返回 pass（不在内存缓存中了）
	decision := tt.CheckOutbound(traceID)
	if decision.Tainted {
		t.Error("过期条目不应被视为 tainted")
	}
}

// ============================================================
// 14. TestTaintStats — 统计
// ============================================================
func TestTaintStats(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "block"})
	defer db.Close()
	defer tt.Stop()

	// 标记几条
	tt.MarkTainted("trace-stats-001", "手机 13812345678", "inbound")
	tt.MarkTainted("trace-stats-002", "密码 password=xxx", "inbound")
	tt.MarkTainted("trace-stats-003", "干净文本无敏感信息", "inbound") // 不会被标记

	// 执行一次出站检查触发 block 统计
	tt.CheckOutbound("trace-stats-001")

	stats := tt.Stats()
	if !stats["enabled"].(bool) {
		t.Error("enabled 应为 true")
	}
	if stats["total_marked"].(int64) != 2 {
		t.Errorf("total_marked 应为 2, got: %v", stats["total_marked"])
	}
	if stats["active_count"].(int) < 2 {
		t.Errorf("active_count 应 >= 2, got: %v", stats["active_count"])
	}
	if stats["total_blocked"].(int64) < 1 {
		t.Errorf("total_blocked 应 >= 1, got: %v", stats["total_blocked"])
	}
	labelDist, ok := stats["label_distribution"].(map[string]int)
	if !ok {
		t.Fatal("label_distribution 类型不正确")
	}
	if labelDist[TaintPII] < 1 {
		t.Error("PII-TAINTED 分布应 >= 1")
	}
}

// ============================================================
// 15. TestTaintScanAPI — 扫描测试 API
// ============================================================
func TestTaintScanAPI(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	api := &ManagementAPI{taintTracker: tt}

	// 测试有 PII 的文本
	body := `{"text":"请联系 13812345678 或 zhangsan@example.com"}`
	req := httptest.NewRequest("POST", "/api/v1/taint/scan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	api.handleTaintScan(rr, req)

	if rr.Code != 200 {
		t.Fatalf("应返回 200, got: %d", rr.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)

	if !result["tainted"].(bool) {
		t.Error("tainted 应为 true")
	}
	matches, ok := result["matches"].([]interface{})
	if !ok || len(matches) < 2 {
		t.Errorf("matches 应包含 >= 2 个匹配, got: %v", result["matches"])
	}

	// 测试干净文本
	body2 := `{"text":"今天天气很好"}`
	req2 := httptest.NewRequest("POST", "/api/v1/taint/scan", strings.NewReader(body2))
	rr2 := httptest.NewRecorder()
	api.handleTaintScan(rr2, req2)

	var result2 map[string]interface{}
	json.NewDecoder(rr2.Body).Decode(&result2)
	if result2["tainted"].(bool) {
		t.Error("干净文本 tainted 应为 false")
	}
}

// ============================================================
// 16. TestTaintCheckOutboundWarn — 污染出站告警（非阻断）
// ============================================================
func TestTaintCheckOutboundWarn(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "warn"})
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-warn-001"
	tt.MarkTainted(traceID, "银行卡 6222021234567890123", "inbound")

	decision := tt.CheckOutbound(traceID)
	if !decision.Tainted {
		t.Error("应检测到污染")
	}
	if decision.Action != "warn" {
		t.Errorf("action 应为 warn, got: %s", decision.Action)
	}
}

// ============================================================
// 17. TestTaintConfigUpdate — 配置更新
// ============================================================
func TestTaintConfigUpdate(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "block", TTLMinutes: 30})
	defer db.Close()
	defer tt.Stop()

	// 更新配置
	tt.UpdateConfig(TaintConfig{Action: "warn", TTLMinutes: 60, Enabled: true})

	cfg := tt.GetConfig()
	if cfg.Action != "warn" {
		t.Errorf("action 应为 warn, got: %s", cfg.Action)
	}
	if cfg.TTLMinutes != 60 {
		t.Errorf("TTLMinutes 应为 60, got: %d", cfg.TTLMinutes)
	}
}

// ============================================================
// 18. TestTaintSSN — 美国 SSN 标记 PII
// ============================================================
func TestTaintSSN(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-ssn-001", "My SSN is 123-45-6789", "inbound")
	if entry == nil {
		t.Fatal("应检测到 SSN 并标记污染")
	}
	if !containsStr(entry.Labels, TaintPII) {
		t.Errorf("标签应包含 PII-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "ssn_us") {
		t.Errorf("SourceDetail 应包含 ssn_us, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 19. TestTaintJWT — JWT Token 标记 CREDENTIAL
// ============================================================
func TestTaintJWT(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	entry := tt.MarkTainted("trace-jwt-001",
		"Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"inbound")
	if entry == nil {
		t.Fatal("应检测到 JWT 并标记污染")
	}
	if !containsStr(entry.Labels, TaintCredential) {
		t.Errorf("标签应包含 CREDENTIAL-TAINTED, got: %v", entry.Labels)
	}
	if !strings.Contains(entry.SourceDetail, "jwt_token") {
		t.Errorf("SourceDetail 应包含 jwt_token, got: %s", entry.SourceDetail)
	}
}

// ============================================================
// 20. TestTaintListAndGet — 列表和查询
// ============================================================
func TestTaintListAndGet(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	tt.MarkTainted("trace-list-001", "手机 13812345678", "inbound")
	tt.MarkTainted("trace-list-002", "密码 password=abc", "inbound")

	// 列表
	entries := tt.ListTainted(10)
	if len(entries) != 2 {
		t.Errorf("应有 2 个活跃条目, got: %d", len(entries))
	}

	// 查询特定
	e := tt.GetTaint("trace-list-001")
	if e == nil {
		t.Fatal("应能获取到 trace-list-001")
	}
	if e.Source != "inbound" {
		t.Errorf("source 应为 inbound, got: %s", e.Source)
	}

	// 查询不存在的
	e2 := tt.GetTaint("trace-nonexist")
	if e2 != nil {
		t.Error("不存在的 trace 应返回 nil")
	}
}

// ============================================================
// 21. TestTaintStatsAPI — 统计 API
// ============================================================
func TestTaintStatsAPI(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	tt.MarkTainted("trace-api-001", "手机 13812345678", "inbound")

	api := &ManagementAPI{taintTracker: tt}
	req := httptest.NewRequest("GET", "/api/v1/taint/stats", nil)
	rr := httptest.NewRecorder()
	api.handleTaintStats(rr, req)

	if rr.Code != 200 {
		t.Fatalf("应返回 200, got: %d", rr.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)

	if !result["enabled"].(bool) {
		t.Error("enabled 应为 true")
	}
}

// ============================================================
// 22. TestTaintActiveAPI — 活跃列表 API
// ============================================================
func TestTaintActiveAPI(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	tt.MarkTainted("trace-active-001", "电话 13812345678", "inbound")
	tt.MarkTainted("trace-active-002", "密码 pwd=secret", "inbound")

	api := &ManagementAPI{taintTracker: tt}
	req := httptest.NewRequest("GET", "/api/v1/taint/active", nil)
	rr := httptest.NewRecorder()
	api.handleTaintActive(rr, req)

	if rr.Code != 200 {
		t.Fatalf("应返回 200, got: %d", rr.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&result)

	total := int(result["total"].(float64))
	if total != 2 {
		t.Errorf("total 应为 2, got: %d", total)
	}
}

// ============================================================
// 23. TestTaintTraceAPI — 特定 trace 查询 API
// ============================================================
func TestTaintTraceAPI(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{})
	defer db.Close()
	defer tt.Stop()

	tt.MarkTainted("trace-detail-001", "邮箱 test@example.com", "inbound")

	api := &ManagementAPI{taintTracker: tt}
	req := httptest.NewRequest("GET", "/api/v1/taint/trace/trace-detail-001", nil)
	rr := httptest.NewRecorder()
	api.handleTaintTrace(rr, req)

	if rr.Code != 200 {
		t.Fatalf("应返回 200, got: %d", rr.Code)
	}

	var entry TaintEntry
	json.NewDecoder(rr.Body).Decode(&entry)
	if entry.TraceID != "trace-detail-001" {
		t.Errorf("trace_id 应为 trace-detail-001, got: %s", entry.TraceID)
	}
}

// ============================================================
// 24. TestTaintConfigAPI — 配置 API（GET + PUT）
// ============================================================
func TestTaintConfigAPI(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "block", TTLMinutes: 30})
	defer db.Close()
	defer tt.Stop()

	api := &ManagementAPI{taintTracker: tt}

	// GET config
	req := httptest.NewRequest("GET", "/api/v1/taint/config", nil)
	rr := httptest.NewRecorder()
	api.handleTaintConfigGet(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET config 应返回 200, got: %d", rr.Code)
	}

	// PUT config
	body := `{"enabled":true,"action":"warn","ttl_minutes":60}`
	req2 := httptest.NewRequest("PUT", "/api/v1/taint/config", strings.NewReader(body))
	rr2 := httptest.NewRecorder()
	api.handleTaintConfigUpdate(rr2, req2)
	if rr2.Code != 200 {
		t.Fatalf("PUT config 应返回 200, got: %d", rr2.Code)
	}

	// 验证更新
	cfg := tt.GetConfig()
	if cfg.Action != "warn" {
		t.Errorf("更新后 action 应为 warn, got: %s", cfg.Action)
	}
}

// ============================================================
// 25. TestTaintFullPropagationChain — 完整传播链 (inbound→llm→outbound)
// ============================================================
func TestTaintFullPropagationChain(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Action: "block"})
	defer db.Close()
	defer tt.Stop()

	traceID := "trace-chain-001"

	// 1. 入站标记
	entry := tt.MarkTainted(traceID, "用户手机 13912345678", "inbound")
	if entry == nil {
		t.Fatal("入站标记失败")
	}

	// 2. LLM 请求传播
	tt.Propagate(traceID, "llm_request", "user message forwarded to LLM")

	// 3. LLM 响应传播
	tt.Propagate(traceID, "llm_response", "LLM response received")

	// 4. 出站检查（CheckOutbound 内部也会调用 Propagate）
	decision := tt.CheckOutbound(traceID)
	if !decision.Tainted {
		t.Error("完整链路应检测到污染")
	}
	if decision.Action != "block" {
		t.Errorf("应阻断, got: %s", decision.Action)
	}

	// 验证传播历史
	entry = tt.GetTaint(traceID)
	if entry == nil {
		t.Fatal("应能获取到条目")
	}
	// 至少 4 条：初始标记 + llm_request + llm_response + outbound
	if len(entry.Propagations) < 4 {
		t.Errorf("应有至少 4 条传播记录, got: %d", len(entry.Propagations))
	}

	// 验证所有阶段都有
	stageMap := make(map[string]bool)
	for _, p := range entry.Propagations {
		stageMap[p.Stage] = true
	}
	for _, expected := range []string{"inbound", "llm_request", "llm_response", "outbound"} {
		if !stageMap[expected] {
			t.Errorf("传播链应包含阶段 %s", expected)
		}
	}
}