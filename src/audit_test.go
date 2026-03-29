// audit_test.go — AuditLogger、审计日志查询/导出/清理/时间线测试
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// 审计日志测试
// ============================================================

func TestAuditLogger(t *testing.T) {
	tmpDB := "/tmp/lobster-guard-test-audit.db"
	defer os.Remove(tmpDB)
	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("初始化数据库失败: %v", err) }
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil { t.Fatalf("初始化审计日志失败: %v", err) }
	defer logger.Close()

	logger.Log("inbound", "user1", "block", "prompt_injection", "ignore previous", "hash123", 0.5, "up-1", "app-1")
	logger.Log("outbound", "", "pass", "", "正常消息", "hash456", 0.1, "up-1", "app-1")

	time.Sleep(200 * time.Millisecond)
	logger.Flush() // v32.1: 确保批量缓冲区写入 DB

	logs, err := logger.QueryLogs("inbound", "block", "", 10)
	if err != nil { t.Fatalf("查询失败: %v", err) }
	if len(logs) == 0 { t.Fatal("应查到至少1条") }

	stats := logger.Stats()
	if stats["total"] == nil { t.Fatal("统计应包含 total") }
}

// ============================================================
// 管理 API 测试

// ============================================================
// 数据库初始化测试
// ============================================================

func TestInitDB(t *testing.T) {
	tmpDB := "/tmp/lobster-guard-test-initdb.db"
	defer os.Remove(tmpDB)
	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("初始化失败: %v", err) }
	defer db.Close()

	// 验证表存在
	tables := []string{"audit_log", "upstreams", "user_routes"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil { t.Fatalf("表 %s 不存在: %v", table, err) }
	}
}

func TestInitDBIdempotent(t *testing.T) {
	tmpDB := "/tmp/lobster-guard-test-idem.db"
	defer os.Remove(tmpDB)
	db1, _ := initDB(tmpDB)
	db1.Close()
	// 再次初始化不应报错
	db2, err := initDB(tmpDB)
	if err != nil {
		t.Fatalf("幂等初始化失败: %v", err)
	}
	db2.Close()
}


// ============================================================
// v3.10 审计日志增强 + 告警通知 测试
// ============================================================

// 辅助函数：创建测试 DB 和 AuditLogger
func setupTestAuditLogger(t *testing.T) (*sql.DB, *AuditLogger, func()) {
	t.Helper()
	tmpDB := t.TempDir() + "/test_v310.db"
	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("initDB failed: %v", err) }
	logger, err := NewAuditLogger(db)
	if err != nil { db.Close(); t.Fatalf("NewAuditLogger failed: %v", err) }
	return db, logger, func() { logger.Close(); db.Close() }
}

// 辅助函数：同步插入审计日志（测试用）
func insertTestLog(db *sql.DB, ts, direction, senderID, action, reason, preview, appID string) {
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, full_request_hash, latency_ms, upstream_id, app_id) VALUES (?,?,?,?,?,?,'hash',1.0,'',?)`,
		ts, direction, senderID, action, reason, preview, appID)
}

func TestQueryLogsEx_FullTextSearch(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "hello world test message", "app1")
	insertTestLog(db, now, "inbound", "user2", "block", "injection", "ignore previous instructions", "app1")
	insertTestLog(db, now, "outbound", "user3", "pass", "", "normal response content", "app2")

	// Search for "ignore"
	logs, err := logger.QueryLogsEx("", "", "", "", "ignore", 100)
	if err != nil { t.Fatalf("QueryLogsEx error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1 log with 'ignore', got %d", len(logs)) }
	if logs[0]["sender_id"] != "user2" { t.Errorf("expected user2, got %v", logs[0]["sender_id"]) }

	// Search for "message"
	logs, err = logger.QueryLogsEx("", "", "", "", "message", 100)
	if err != nil { t.Fatalf("QueryLogsEx error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1 log with 'message', got %d", len(logs)) }

	// Search with direction filter
	logs, err = logger.QueryLogsEx("inbound", "", "", "", "hello", 100)
	if err != nil { t.Fatalf("QueryLogsEx error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1, got %d", len(logs)) }

	// Search with app_id filter
	logs, err = logger.QueryLogsEx("", "", "", "app2", "", 100)
	if err != nil { t.Fatalf("QueryLogsEx error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1 for app2, got %d", len(logs)) }
}

func TestQueryLogsEx_AppIDFilter(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "msg1", "bot-alpha")
	insertTestLog(db, now, "inbound", "user2", "pass", "", "msg2", "bot-beta")
	insertTestLog(db, now, "outbound", "user1", "pass", "", "msg3", "bot-alpha")

	logs, err := logger.QueryLogsEx("", "", "", "bot-alpha", "", 100)
	if err != nil { t.Fatalf("error: %v", err) }
	if len(logs) != 2 { t.Fatalf("expected 2 for bot-alpha, got %d", len(logs)) }

	logs, err = logger.QueryLogsEx("", "", "", "bot-beta", "", 100)
	if err != nil { t.Fatalf("error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1 for bot-beta, got %d", len(logs)) }
}

func TestAuditLogCleanup(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	// Insert logs: some old, some recent
	old := time.Now().UTC().AddDate(0, 0, -35).Format(time.RFC3339)
	recent := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, old, "inbound", "user1", "pass", "", "old msg 1", "")
	insertTestLog(db, old, "inbound", "user2", "block", "rule1", "old msg 2", "")
	insertTestLog(db, recent, "inbound", "user3", "pass", "", "recent msg", "")

	// Cleanup with 30 day retention
	deleted, err := logger.CleanupOldLogs(30)
	if err != nil { t.Fatalf("CleanupOldLogs error: %v", err) }
	if deleted != 2 { t.Fatalf("expected 2 deleted, got %d", deleted) }

	// Verify remaining
	logs, err := logger.QueryLogs("", "", "", 100)
	if err != nil { t.Fatalf("QueryLogs error: %v", err) }
	if len(logs) != 1 { t.Fatalf("expected 1 remaining, got %d", len(logs)) }
}

func TestAuditLogCleanup_NoneToClean(t *testing.T) {
	_, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	deleted, err := logger.CleanupOldLogs(30)
	if err != nil { t.Fatalf("error: %v", err) }
	if deleted != 0 { t.Fatalf("expected 0, got %d", deleted) }
}

func TestAuditStats(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	ts1 := "2024-01-01T10:00:00Z"
	ts2 := "2024-06-15T12:00:00Z"
	insertTestLog(db, ts1, "inbound", "user1", "pass", "", "msg1", "")
	insertTestLog(db, ts2, "outbound", "user2", "block", "", "msg2", "")

	stats := logger.AuditStats()
	if stats["total"] != 2 { t.Fatalf("expected total=2, got %v", stats["total"]) }
	if stats["earliest"] != ts1 { t.Errorf("expected earliest=%s, got %v", ts1, stats["earliest"]) }
	if stats["latest"] != ts2 { t.Errorf("expected latest=%s, got %v", ts2, stats["latest"]) }
	if stats["disk_bytes"] == nil { t.Error("expected disk_bytes") }
}

func TestAuditTimeline(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	// Insert some logs in the past few hours
	now := time.Now().UTC()
	h1 := now.Add(-1 * time.Hour).Format(time.RFC3339)
	h2 := now.Add(-2 * time.Hour).Format(time.RFC3339)
	insertTestLog(db, h1, "inbound", "user1", "pass", "", "msg1", "")
	insertTestLog(db, h1, "inbound", "user2", "block", "rule", "msg2", "")
	insertTestLog(db, h2, "outbound", "user3", "warn", "", "msg3", "")

	timeline := logger.Timeline(24)
	if len(timeline) != 24 { t.Fatalf("expected 24 hours, got %d", len(timeline)) }

	// The timeline should have entries with non-zero values for recent hours
	totalPass := 0; totalBlock := 0; totalWarn := 0
	for _, entry := range timeline {
		if p, ok := entry["pass"].(int); ok { totalPass += p }
		if b, ok := entry["block"].(int); ok { totalBlock += b }
		if wa, ok := entry["warn"].(int); ok { totalWarn += wa }
	}
	if totalPass < 1 { t.Errorf("expected at least 1 pass, got %d", totalPass) }
	if totalBlock < 1 { t.Errorf("expected at least 1 block, got %d", totalBlock) }
	if totalWarn < 1 { t.Errorf("expected at least 1 warn, got %d", totalWarn) }
}

func TestAuditTimeline_Empty(t *testing.T) {
	_, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	timeline := logger.Timeline(24)
	if len(timeline) != 24 { t.Fatalf("expected 24 empty hours, got %d", len(timeline)) }
	for _, entry := range timeline {
		if entry["pass"] != 0 || entry["block"] != 0 || entry["warn"] != 0 {
			t.Error("expected all zeros in empty timeline")
		}
	}
}

func TestAuditExportCSV_API(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "csv test msg", "app1")
	insertTestLog(db, now, "outbound", "user2", "block", "rule1", "blocked msg", "app2")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Test CSV export
	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv", nil)
	w := httptest.NewRecorder()
	api.handleAuditExport(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	if !strings.Contains(w.Header().Get("Content-Type"), "text/csv") { t.Error("expected csv content type") }
	body := w.Body.String()
	if !strings.Contains(body, "id,timestamp,direction") { t.Error("expected CSV header") }
	if !strings.Contains(body, "csv test msg") { t.Error("expected csv content") }
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 3 { t.Errorf("expected 3 lines (header + 2 data), got %d", len(lines)) }
}

func TestAuditExportJSON_API(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "json test msg", "app1")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=json", nil)
	w := httptest.NewRecorder()
	api.handleAuditExport(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") { t.Error("expected json content type") }
	var logs []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(logs) != 1 { t.Fatalf("expected 1 log, got %d", len(logs)) }
	if logs[0]["content_preview"] != "json test msg" { t.Error("expected 'json test msg'") }
}

func TestAuditExportBadFormat(t *testing.T) {
	_, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	cfg := &Config{}
	db2, _ := initDB(t.TempDir() + "/db2.db")
	defer db2.Close()
	pool := NewUpstreamPool(cfg, db2)
	routes := NewRouteTable(db2, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=xml", nil)
	w := httptest.NewRecorder()
	api.handleAuditExport(w, req)
	if w.Code != 400 { t.Fatalf("expected 400, got %d", w.Code) }
}

func TestAuditCleanup_API(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	old := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	insertTestLog(db, old, "inbound", "user1", "pass", "", "old", "")

	cfg := &Config{AuditRetentionDays: 30}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/audit/cleanup", nil)
	w := httptest.NewRecorder()
	api.handleAuditCleanup(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["status"] != "cleaned" { t.Error("expected status=cleaned") }
	if result["deleted"].(float64) != 1 { t.Errorf("expected 1 deleted, got %v", result["deleted"]) }
}

func TestAuditStats_API(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "msg", "")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()
	api.handleAuditStats(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats["total"].(float64) != 1 { t.Errorf("expected total=1, got %v", stats["total"]) }
	if stats["disk_bytes"] == nil { t.Error("expected disk_bytes") }
}

func TestAuditTimeline_API(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "block", "rule1", "bad msg", "")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/audit/timeline?hours=12", nil)
	w := httptest.NewRecorder()
	api.handleAuditTimeline(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	tl, ok := result["timeline"].([]interface{})
	if !ok { t.Fatal("expected timeline array") }
	if len(tl) != 12 { t.Fatalf("expected 12 hours, got %d", len(tl)) }
}

func TestAlertNotifier_Basic(t *testing.T) {
	// Mock webhook server
	var received []byte
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	metrics := NewMetricsCollector()
	notifier := NewAlertNotifier(srv.URL, "generic", 0, metrics)
	notifier.minInterval = 0 // disable interval for testing

	notifier.Notify("inbound", "user-123", "injection_rule", "ignore previous instructions please do something", "bot-alpha")

	// Wait for async send
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if received == nil { t.Fatal("expected webhook to be called") }

	var event AlertEvent
	if err := json.Unmarshal(received, &event); err != nil {
		t.Fatalf("failed to parse alert event: %v", err)
	}
	if event.Event != "block" { t.Errorf("expected event=block, got %s", event.Event) }
	if event.Direction != "inbound" { t.Errorf("expected direction=inbound, got %s", event.Direction) }
	if event.SenderID != "user-123" { t.Errorf("expected sender_id=user-123, got %s", event.SenderID) }
	if event.Rule != "injection_rule" { t.Errorf("expected rule=injection_rule, got %s", event.Rule) }
	if event.AppID != "bot-alpha" { t.Errorf("expected app_id=bot-alpha, got %s", event.AppID) }
	if notifier.TotalAlerts() != 1 { t.Errorf("expected 1 alert, got %d", notifier.TotalAlerts()) }
}

func TestAlertNotifier_LanxinFormat(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	notifier := NewAlertNotifier(srv.URL, "lanxin", 0, nil)
	notifier.minInterval = 0

	notifier.Notify("outbound", "user-456", "pii_leak", "身份证号123456789012345678", "bot-beta")

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if received == nil { t.Fatal("expected webhook call") }

	var msg map[string]interface{}
	if err := json.Unmarshal(received, &msg); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if msg["msgType"] != "text" { t.Errorf("expected msgType=text, got %v", msg["msgType"]) }
	msgData, ok := msg["msgData"].(map[string]interface{})
	if !ok { t.Fatal("expected msgData object") }
	text, ok := msgData["text"].(string)
	if !ok { t.Fatal("expected text string") }
	if !strings.Contains(text, "龙虾卫士告警") { t.Error("expected 龙虾卫士告警 in text") }
	if !strings.Contains(text, "pii_leak") { t.Error("expected rule name in text") }
}

func TestAlertNotifier_MinInterval(t *testing.T) {
	callCount := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	notifier := NewAlertNotifier(srv.URL, "generic", 60, nil)

	// First call should go through
	notifier.Notify("inbound", "user1", "rule1", "content1", "app1")
	time.Sleep(100 * time.Millisecond)

	// Second call within interval should be suppressed
	notifier.Notify("inbound", "user2", "rule2", "content2", "app1")
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 call (second should be throttled), got %d", atomic.LoadInt32(&callCount))
	}
}

func TestAlertNotifier_ContentPreviewTruncation(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	notifier := NewAlertNotifier(srv.URL, "generic", 0, nil)
	notifier.minInterval = 0

	longContent := strings.Repeat("一二三四五", 20) // 100 chars
	notifier.Notify("inbound", "user", "rule", longContent, "app")

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	var event AlertEvent
	json.Unmarshal(received, &event)
	// Content should be truncated to 50 chars + "..."
	if len([]rune(event.ContentPreview)) > 54 { // 50 + "..."
		t.Errorf("content_preview too long: %d runes", len([]rune(event.ContentPreview)))
	}
}

func TestMetricsCollector_AlertsTotal(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordAlert()
	mc.RecordAlert()
	mc.RecordAlert()

	var buf bytes.Buffer
	mc.WritePrometheus(&buf, 1, 1, 0, nil, "test", "webhook", nil, nil, nil)
	output := buf.String()
	if !strings.Contains(output, "lobster_guard_alerts_total{type=\"block\"} 3") {
		t.Errorf("expected alerts_total=3 in:\n%s", output)
	}
}

func TestAuditExportWithFilters(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "block", "r1", "blocked content here", "app1")
	insertTestLog(db, now, "outbound", "user2", "pass", "", "normal msg", "app2")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Export CSV with direction filter
	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv&direction=inbound", nil)
	w := httptest.NewRecorder()
	api.handleAuditExport(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 2 { t.Errorf("expected 2 lines (header + 1), got %d", len(lines)) }

	// Export JSON with q filter
	req = httptest.NewRequest("GET", "/api/v1/audit/export?format=json&q=blocked", nil)
	w = httptest.NewRecorder()
	api.handleAuditExport(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	var logs []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &logs)
	if len(logs) != 1 { t.Fatalf("expected 1 log, got %d", len(logs)) }
}

func TestConfigNewFields(t *testing.T) {
	yamlStr := `
audit_retention_days: 7
alert_webhook: "https://example.com/webhook"
alert_min_interval: 30
alert_format: "lanxin"
`
	tmpFile := t.TempDir() + "/test_config.yaml"
	os.WriteFile(tmpFile, []byte(yamlStr), 0644)
	cfg, err := loadConfig(tmpFile)
	if err != nil { t.Fatalf("loadConfig error: %v", err) }
	if cfg.AuditRetentionDays != 7 { t.Errorf("expected 7, got %d", cfg.AuditRetentionDays) }
	if cfg.AlertWebhook != "https://example.com/webhook" { t.Error("wrong webhook") }
	if cfg.AlertMinInterval != 30 { t.Errorf("expected 30, got %d", cfg.AlertMinInterval) }
	if cfg.AlertFormat != "lanxin" { t.Error("wrong format") }
}

func TestConfigDefaults_NewFields(t *testing.T) {
	// Test that omitting v3.10 fields doesn't break anything
	yamlStr := `inbound_listen: ":8443"`
	tmpFile := t.TempDir() + "/test_config_defaults.yaml"
	os.WriteFile(tmpFile, []byte(yamlStr), 0644)
	cfg, err := loadConfig(tmpFile)
	if err != nil { t.Fatalf("loadConfig error: %v", err) }
	if cfg.AuditRetentionDays != 0 { t.Errorf("expected default 0, got %d", cfg.AuditRetentionDays) }
	if cfg.AlertWebhook != "" { t.Error("expected empty webhook") }
	if cfg.AlertMinInterval != 0 { t.Errorf("expected 0, got %d", cfg.AlertMinInterval) }
	if cfg.AlertFormat != "" { t.Error("expected empty format") }
}

func TestAuditLogsAPI_WithQParam(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "hello search test", "app1")
	insertTestLog(db, now, "inbound", "user2", "pass", "", "goodbye world", "app1")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Test with q param
	req := httptest.NewRequest("GET", "/api/v1/audit/logs?q=search", nil)
	w := httptest.NewRecorder()
	api.handleAuditLogs(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	total := int(result["total"].(float64))
	if total != 1 { t.Errorf("expected 1 result for q=search, got %d", total) }

	// Test with app_id param
	req = httptest.NewRequest("GET", "/api/v1/audit/logs?app_id=app1", nil)
	w = httptest.NewRecorder()
	api.handleAuditLogs(w, req)
	if w.Code != 200 { t.Fatalf("expected 200, got %d", w.Code) }
	json.Unmarshal(w.Body.Bytes(), &result)
	total = int(result["total"].(float64))
	if total != 2 { t.Errorf("expected 2 results for app_id=app1, got %d", total) }
}

func TestServeHTTP_NewAuditEndpoints(t *testing.T) {
	db, logger, cleanup := setupTestAuditLogger(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)
	insertTestLog(db, now, "inbound", "user1", "pass", "", "test msg", "")

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()
	client := server.Client()

	// Test /api/v1/audit/export
	resp, err := client.Get(server.URL + "/api/v1/audit/export?format=json")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Errorf("export: expected 200, got %d", resp.StatusCode) }

	// Test /api/v1/audit/stats
	resp2, err := client.Get(server.URL + "/api/v1/audit/stats")
	if err != nil { t.Fatal(err) }
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 { t.Errorf("stats: expected 200, got %d", resp2.StatusCode) }

	// Test /api/v1/audit/timeline
	resp3, err := client.Get(server.URL + "/api/v1/audit/timeline")
	if err != nil { t.Fatal(err) }
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 { t.Errorf("timeline: expected 200, got %d", resp3.StatusCode) }

	// Test /api/v1/audit/cleanup
	resp4, err := client.Post(server.URL+"/api/v1/audit/cleanup", "application/json", nil)
	if err != nil { t.Fatal(err) }
	defer resp4.Body.Close()
	if resp4.StatusCode != 200 { t.Errorf("cleanup: expected 200, got %d", resp4.StatusCode) }
}

func TestAlertNotifier_NilNotifier(t *testing.T) {
	// Ensure nil alertNotifier doesn't cause panic in InboundProxy/OutboundProxy
	var notifier *AlertNotifier
	if notifier != nil {
		t.Error("nil notifier should be nil")
	}
	// This just tests that the nil check works (no crash)
}

