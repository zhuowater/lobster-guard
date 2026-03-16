// logger_test.go — Logger 和 trace_id 及 realtime 测试（v5.0）
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ============================================================
// 1. Logger 测试
// ============================================================

func TestLogger_TextFormat(t *testing.T) {
	l := NewLogger("text", nil)
	if l.Format() != "text" {
		t.Fatalf("expected format 'text', got %q", l.Format())
	}
	// text mode uses log.Printf, just ensure no panic
	l.Info("system", "启动成功")
	l.Warn("inbound", "告警事件", "sender_id", "user1")
	l.Error("audit", "数据库错误", "err", "connection refused")
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("json", &buf)
	if l.Format() != "json" {
		t.Fatalf("expected format 'json', got %q", l.Format())
	}

	l.Info("inbound", "请求通过", "sender_id", "user-001", "app_id", "bot-1", "action", "pass", "latency_ms", 5)

	output := buf.String()
	if output == "" {
		t.Fatal("JSON logger produced no output")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v, output=%q", err, output)
	}

	// 检查必要字段
	if entry["level"] != "info" {
		t.Errorf("expected level 'info', got %v", entry["level"])
	}
	if entry["module"] != "inbound" {
		t.Errorf("expected module 'inbound', got %v", entry["module"])
	}
	if entry["msg"] != "请求通过" {
		t.Errorf("expected msg '请求通过', got %v", entry["msg"])
	}
	if entry["sender_id"] != "user-001" {
		t.Errorf("expected sender_id 'user-001', got %v", entry["sender_id"])
	}
	if entry["time"] == nil {
		t.Error("expected time field")
	}
}

func TestLogger_JSONWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("json", &buf)

	l.Warn("outbound", "PII 检测", "rule", "pii_id_card")

	var entry map[string]interface{}
	json.Unmarshal(buf.Bytes(), &entry)
	if entry["level"] != "warn" {
		t.Errorf("expected level 'warn', got %v", entry["level"])
	}
}

func TestLogger_JSONErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("json", &buf)

	l.Error("system", "致命错误", "code", 500)

	var entry map[string]interface{}
	json.Unmarshal(buf.Bytes(), &entry)
	if entry["level"] != "error" {
		t.Errorf("expected level 'error', got %v", entry["level"])
	}
	if entry["code"].(float64) != 500 {
		t.Errorf("expected code 500, got %v", entry["code"])
	}
}

func TestLogger_DefaultFormat(t *testing.T) {
	l := NewLogger("invalid", nil)
	if l.Format() != "text" {
		t.Errorf("expected default 'text' for invalid format, got %q", l.Format())
	}
}

func TestLogger_EmptyFormat(t *testing.T) {
	l := NewLogger("", nil)
	if l.Format() != "text" {
		t.Errorf("expected default 'text' for empty format, got %q", l.Format())
	}
}

func TestInitAppLogger(t *testing.T) {
	InitAppLogger("json")
	l := GetAppLogger()
	if l.Format() != "json" {
		t.Errorf("expected json format, got %q", l.Format())
	}
	InitAppLogger("text") // restore
}

func TestGetAppLogger_DefaultInit(t *testing.T) {
	oldLogger := appLogger
	appLogger = nil
	defer func() { appLogger = oldLogger }()

	l := GetAppLogger()
	if l == nil {
		t.Fatal("GetAppLogger should not return nil")
	}
	if l.Format() != "text" {
		t.Errorf("default logger should be text, got %q", l.Format())
	}
}

func TestLogLevel_String(t *testing.T) {
	if LogLevelInfo.String() != "info" { t.Error("info") }
	if LogLevelWarn.String() != "warn" { t.Error("warn") }
	if LogLevelError.String() != "error" { t.Error("error") }
}

// ============================================================
// 2. TraceID 测试
// ============================================================

func TestGenerateTraceID_Length(t *testing.T) {
	id := GenerateTraceID()
	if len(id) != 16 {
		t.Errorf("expected trace_id length 16, got %d: %q", len(id), id)
	}
}

func TestGenerateTraceID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := GenerateTraceID()
		if seen[id] {
			t.Fatalf("duplicate trace_id at iteration %d: %s", i, id)
		}
		seen[id] = true
	}
}

func TestGenerateTraceID_HexChars(t *testing.T) {
	id := GenerateTraceID()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("trace_id contains non-hex char: %c in %s", c, id)
		}
	}
}

// ============================================================
// 3. RealtimeMetrics 测试
// ============================================================

func TestRealtimeMetrics_RecordInbound(t *testing.T) {
	rm := NewRealtimeMetrics()
	rm.RecordInbound("pass", 1000)
	rm.RecordInbound("block", 2000)
	rm.RecordInbound("warn", 3000)

	snap := rm.Snapshot()
	if snap.TotalReq != 3 {
		t.Errorf("expected 3 total requests, got %d", snap.TotalReq)
	}
	if snap.TotalBlock != 1 { // only "block" counts as block
		t.Errorf("expected 1 total block, got %d", snap.TotalBlock)
	}
}

func TestRealtimeMetrics_RecordOutbound(t *testing.T) {
	rm := NewRealtimeMetrics()
	rm.RecordOutbound("pass", 500)
	rm.RecordOutbound("block", 1500)

	snap := rm.Snapshot()
	if snap.TotalReq != 2 {
		t.Errorf("expected 2 total requests, got %d", snap.TotalReq)
	}
}

func TestRealtimeMetrics_Events(t *testing.T) {
	rm := NewRealtimeMetrics()
	for i := 0; i < 25; i++ {
		rm.RecordEvent("inbound", "user-1", "block", "injection", "trace-123")
	}

	snap := rm.Snapshot()
	if len(snap.Events) != 20 {
		t.Errorf("expected max 20 events, got %d", len(snap.Events))
	}
	// 验证最后一条
	last := snap.Events[len(snap.Events)-1]
	if last.SenderID != "user-1" {
		t.Errorf("expected sender_id 'user-1', got %q", last.SenderID)
	}
}

func TestRealtimeMetrics_SnapshotSlots(t *testing.T) {
	rm := NewRealtimeMetrics()
	snap := rm.Snapshot()
	if len(snap.Slots) != realtimeSlots {
		t.Errorf("expected %d slots, got %d", realtimeSlots, len(snap.Slots))
	}
}

func TestRealtimeMetrics_BlockRate(t *testing.T) {
	rm := NewRealtimeMetrics()
	rm.RecordInbound("pass", 1000)
	rm.RecordInbound("pass", 1000)
	rm.RecordInbound("block", 1000)
	rm.RecordInbound("block", 1000)

	snap := rm.Snapshot()
	// 2 blocks out of 4 = 50%
	if snap.BlockRate < 49 || snap.BlockRate > 51 {
		t.Errorf("expected ~50%% block rate, got %.2f%%", snap.BlockRate)
	}
}

func TestRealtimeMetrics_AvgLatency(t *testing.T) {
	rm := NewRealtimeMetrics()
	// 1000us = 1ms, 3000us = 3ms => avg = 2ms
	rm.RecordInbound("pass", 1000)
	rm.RecordInbound("pass", 3000)

	snap := rm.Snapshot()
	if snap.AvgLatencyMs < 1.9 || snap.AvgLatencyMs > 2.1 {
		t.Errorf("expected ~2ms avg latency, got %.2fms", snap.AvgLatencyMs)
	}
}

func TestRealtimeMetrics_SlotReset(t *testing.T) {
	rm := NewRealtimeMetrics()
	// Record and verify data exists for current second
	rm.RecordInbound("pass", 100)
	snap := rm.Snapshot()

	// The last slot should have data
	lastSlot := snap.Slots[len(snap.Slots)-1]
	if lastSlot.InboundN != 1 {
		t.Errorf("expected 1 inbound request in last slot, got %d", lastSlot.InboundN)
	}
}

// ============================================================
// 4. Config log_format 验证测试
// ============================================================

func TestConfig_LogFormat_Validation(t *testing.T) {
	cfg := &Config{
		InboundListen:    ":8443",
		OutboundListen:   ":8444",
		ManagementListen: ":9090",
		DBPath:           "/tmp/test.db",
		LogFormat:        "xml", // invalid
	}
	errs := validateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "log_format") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected log_format validation error for 'xml'")
	}
}

func TestConfig_LogFormat_Valid(t *testing.T) {
	for _, fmt := range []string{"text", "json", ""} {
		cfg := &Config{
			InboundListen:    ":8443",
			OutboundListen:   ":8444",
			ManagementListen: ":9090",
			DBPath:           "/tmp/test.db",
			LogFormat:        fmt,
		}
		errs := validateConfig(cfg)
		for _, e := range errs {
			if strings.Contains(e, "log_format") {
				t.Errorf("unexpected log_format validation error for %q: %s", fmt, e)
			}
		}
	}
}

// ============================================================
// 5. traceResponseWriter 测试
// ============================================================

func TestTraceResponseWriter_SetsHeader(t *testing.T) {
	// 使用 httptest 的 ResponseRecorder 不适合这里，因为它不会调用我们的 WriteHeader
	// 直接测试 traceResponseWriter 的逻辑
	inner := &mockResponseWriter{headers: make(map[string][]string)}
	tw := &traceResponseWriter{ResponseWriter: inner, traceID: "abc123def456"}

	tw.Write([]byte("hello"))
	if inner.headers["X-Trace-Id"] == nil || inner.headers["X-Trace-Id"][0] != "abc123def456" {
		t.Errorf("expected X-Trace-ID header, got %v", inner.headers)
	}
}

func TestTraceResponseWriter_WriteHeader(t *testing.T) {
	inner := &mockResponseWriter{headers: make(map[string][]string)}
	tw := &traceResponseWriter{ResponseWriter: inner, traceID: "trace999"}

	tw.WriteHeader(201)
	if inner.headers["X-Trace-Id"] == nil || inner.headers["X-Trace-Id"][0] != "trace999" {
		t.Errorf("expected X-Trace-ID header on WriteHeader, got %v", inner.headers)
	}
	if inner.statusCode != 201 {
		t.Errorf("expected status 201, got %d", inner.statusCode)
	}
}

// mockResponseWriter for testing
type mockResponseWriter struct {
	headers    map[string][]string
	statusCode int
	body       bytes.Buffer
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(map[string][]string)
	}
	return http.Header(m.headers)
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

// ============================================================
// 6. 审计日志 trace_id 测试
// ============================================================

func TestAuditLogger_TraceID(t *testing.T) {
	tmpDB := t.TempDir() + "/trace-test.db"
	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("initDB: %v", err) }
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil { t.Fatalf("NewAuditLogger: %v", err) }
	defer logger.Close()

	traceID := "abcdef1234567890"
	logger.LogWithTrace("inbound", "user1", "block", "injection", "test content", "hash", 5.0, "up-1", "app-1", traceID)
	time.Sleep(100 * time.Millisecond) // wait for async write

	// Query with trace_id filter
	logs, err := logger.QueryLogsExTrace("", "", "", "", "", traceID, 10)
	if err != nil { t.Fatalf("QueryLogsExTrace: %v", err) }
	if len(logs) != 1 {
		t.Fatalf("expected 1 log with trace_id, got %d", len(logs))
	}
	if logs[0]["trace_id"] != traceID {
		t.Errorf("expected trace_id %q, got %v", traceID, logs[0]["trace_id"])
	}
}

func TestAuditLogger_TraceID_EmptyFilter(t *testing.T) {
	tmpDB := t.TempDir() + "/trace-filter-test.db"
	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("initDB: %v", err) }
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil { t.Fatalf("NewAuditLogger: %v", err) }
	defer logger.Close()

	logger.LogWithTrace("inbound", "user1", "pass", "", "content1", "hash1", 1.0, "", "", "trace-aaa")
	logger.LogWithTrace("inbound", "user2", "pass", "", "content2", "hash2", 2.0, "", "", "trace-bbb")
	time.Sleep(100 * time.Millisecond)

	// Query without trace_id filter should return both
	logs, err := logger.QueryLogsExTrace("", "", "", "", "", "", 10)
	if err != nil { t.Fatalf("QueryLogsExTrace: %v", err) }
	if len(logs) < 2 {
		t.Errorf("expected at least 2 logs, got %d", len(logs))
	}
}

// ============================================================
// 7. 审计日志归档测试
// ============================================================

func TestAuditLogger_Archive(t *testing.T) {
	tmpDB := t.TempDir() + "/archive-test.db"
	archiveDir := t.TempDir()

	db, err := initDB(tmpDB)
	if err != nil { t.Fatalf("initDB: %v", err) }
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil { t.Fatalf("NewAuditLogger: %v", err) }
	defer logger.Close()

	// Insert old logs (40 days ago)
	oldTime := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339Nano)
	for i := 0; i < 5; i++ {
		db.Exec(`INSERT INTO audit_log (timestamp,direction,sender_id,action,reason,content_preview,full_request_hash,latency_ms,upstream_id,app_id,trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			oldTime, "inbound", "user1", "pass", "", "old content", "hash", 1.0, "", "", "trace-old")
	}

	// Insert recent logs
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < 3; i++ {
		db.Exec(`INSERT INTO audit_log (timestamp,direction,sender_id,action,reason,content_preview,full_request_hash,latency_ms,upstream_id,app_id,trace_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			now, "inbound", "user2", "pass", "", "new content", "hash", 1.0, "", "", "trace-new")
	}

	// Archive (retention = 30 days)
	path, deleted, err := logger.ArchiveLogs(30, archiveDir)
	if err != nil { t.Fatalf("ArchiveLogs: %v", err) }
	if path == "" { t.Fatal("expected archive path") }
	if deleted != 5 { t.Errorf("expected 5 deleted, got %d", deleted) }

	// Verify archive file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("archive file does not exist: %s", path)
	}

	// Verify remaining logs
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 remaining logs, got %d", count)
	}
}

func TestListArchives(t *testing.T) {
	dir := t.TempDir()

	// Create some archive files
	os.WriteFile(dir+"/audit-2026-01-01.json.gz", []byte("test"), 0644)
	os.WriteFile(dir+"/audit-2026-01-02.json.gz", []byte("test"), 0644)
	os.WriteFile(dir+"/not-an-archive.txt", []byte("test"), 0644)

	archives, err := ListArchives(dir)
	if err != nil { t.Fatalf("ListArchives: %v", err) }
	if len(archives) != 2 {
		t.Errorf("expected 2 archives, got %d", len(archives))
	}
}

func TestListArchives_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	archives, err := ListArchives(dir)
	if err != nil { t.Fatalf("ListArchives: %v", err) }
	if len(archives) != 0 {
		t.Errorf("expected 0 archives, got %d", len(archives))
	}
}

func TestListArchives_NonexistentDir(t *testing.T) {
	archives, err := ListArchives("/nonexistent/path/should/not/exist")
	if err != nil { t.Fatalf("ListArchives should not error for non-existent dir: %v", err) }
	if len(archives) != 0 {
		t.Errorf("expected 0 archives, got %d", len(archives))
	}
}
