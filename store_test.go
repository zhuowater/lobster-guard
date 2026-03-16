// store_test.go — Store 接口、SQLiteStore、备份/恢复、健康检查、优雅关闭测试（v4.2）
package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// Store 辅助函数
// ============================================================

func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()
	tmpDB := filepath.Join(t.TempDir(), "test_store.db")
	db, err := initDB(tmpDB)
	if err != nil {
		t.Fatalf("initDB failed: %v", err)
	}
	store := NewSQLiteStore(db, tmpDB)
	return store, func() { store.Close() }
}

// ============================================================
// 1. Store.Ping 测试
// ============================================================

func TestSQLiteStore_Ping(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if err := store.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestSQLiteStore_Ping_ClosedDB(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "closed.db")
	db, _ := initDB(tmpDB)
	store := NewSQLiteStore(db, tmpDB)
	store.Close()

	err := store.Ping()
	if err == nil {
		t.Fatal("expected error for closed db")
	}
}

// ============================================================
// 2. Store.LogAudit + QueryAuditLogs 测试
// ============================================================

func TestSQLiteStore_LogAndQueryAudit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	entry := &AuditEntry{
		Direction:       "inbound",
		SenderID:        "user1",
		Action:          "block",
		Reason:          "injection",
		ContentPreview:  "ignore previous instructions",
		LatencyMs:       1.5,
		UpstreamID:      "up-1",
		AppID:           "app-1",
	}
	if err := store.LogAudit(entry); err != nil {
		t.Fatalf("LogAudit failed: %v", err)
	}

	results, err := store.QueryAuditLogs(AuditFilter{Direction: "inbound", Limit: 10})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "block" {
		t.Errorf("expected action=block, got %s", results[0].Action)
	}
	if results[0].SenderID != "user1" {
		t.Errorf("expected sender_id=user1, got %s", results[0].SenderID)
	}
}

func TestSQLiteStore_QueryAuditLogs_Filters(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.LogAudit(&AuditEntry{Direction: "inbound", SenderID: "u1", Action: "pass", ContentPreview: "hello world", AppID: "app1"})
	store.LogAudit(&AuditEntry{Direction: "outbound", SenderID: "u2", Action: "block", ContentPreview: "dangerous content", AppID: "app2"})
	store.LogAudit(&AuditEntry{Direction: "inbound", SenderID: "u3", Action: "warn", ContentPreview: "suspicious message", AppID: "app1"})

	// Filter by direction
	r, _ := store.QueryAuditLogs(AuditFilter{Direction: "inbound", Limit: 100})
	if len(r) != 2 {
		t.Errorf("expected 2 inbound, got %d", len(r))
	}

	// Filter by action
	r, _ = store.QueryAuditLogs(AuditFilter{Action: "block", Limit: 100})
	if len(r) != 1 {
		t.Errorf("expected 1 block, got %d", len(r))
	}

	// Filter by app_id
	r, _ = store.QueryAuditLogs(AuditFilter{AppID: "app1", Limit: 100})
	if len(r) != 2 {
		t.Errorf("expected 2 for app1, got %d", len(r))
	}

	// Full text search
	r, _ = store.QueryAuditLogs(AuditFilter{Query: "dangerous", Limit: 100})
	if len(r) != 1 {
		t.Errorf("expected 1 for q=dangerous, got %d", len(r))
	}
}

// ============================================================
// 3. Store.CleanupAuditLogs 测试
// ============================================================

func TestSQLiteStore_CleanupAuditLogs(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	old := time.Now().UTC().AddDate(0, 0, -35).Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	store.LogAudit(&AuditEntry{Timestamp: old, Direction: "inbound", SenderID: "u1", Action: "pass"})
	store.LogAudit(&AuditEntry{Timestamp: old, Direction: "inbound", SenderID: "u2", Action: "block"})
	store.LogAudit(&AuditEntry{Timestamp: now, Direction: "inbound", SenderID: "u3", Action: "pass"})

	deleted, err := store.CleanupAuditLogs(30)
	if err != nil {
		t.Fatalf("CleanupAuditLogs failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
}

// ============================================================
// 4. Store.AuditStats 测试
// ============================================================

func TestSQLiteStore_AuditStats(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.LogAudit(&AuditEntry{Direction: "inbound", Action: "pass"})
	store.LogAudit(&AuditEntry{Direction: "outbound", Action: "block"})

	stats, err := store.AuditStats()
	if err != nil {
		t.Fatalf("AuditStats failed: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("expected total=2, got %d", stats.Total)
	}
	if stats.DiskBytes <= 0 {
		t.Error("expected positive disk_bytes")
	}
}

// ============================================================
// 5. Store.AuditTimeline 测试
// ============================================================

func TestSQLiteStore_AuditTimeline(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	recent := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	store.LogAudit(&AuditEntry{Timestamp: recent, Direction: "inbound", Action: "pass"})
	store.LogAudit(&AuditEntry{Timestamp: recent, Direction: "inbound", Action: "block"})

	timeline, err := store.AuditTimeline(24)
	if err != nil {
		t.Fatalf("AuditTimeline failed: %v", err)
	}
	if len(timeline) != 24 {
		t.Fatalf("expected 24 buckets, got %d", len(timeline))
	}
}

// ============================================================
// 6. Store 路由操作测试
// ============================================================

func TestSQLiteStore_RouteOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	if err := store.SaveRoute("sender1", "app1", "upstream1"); err != nil {
		t.Fatalf("SaveRoute failed: %v", err)
	}
	if err := store.SaveRoute("sender2", "app1", "upstream2"); err != nil {
		t.Fatalf("SaveRoute failed: %v", err)
	}

	routes, err := store.LoadRoutes()
	if err != nil {
		t.Fatalf("LoadRoutes failed: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	if err := store.DeleteRoute("sender1", "app1"); err != nil {
		t.Fatalf("DeleteRoute failed: %v", err)
	}
	routes, _ = store.LoadRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route after delete, got %d", len(routes))
	}
}

func TestSQLiteStore_SaveRouteWithMeta(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.SaveRouteWithMeta("s1", "a1", "up1", "安全部", "张三", "zhangsan@example.com")
	routes, _ := store.LoadRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].Department != "安全部" {
		t.Errorf("expected department=安全部, got %s", routes[0].Department)
	}
}

func TestSQLiteStore_ListRoutesByApp(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.SaveRoute("s1", "app-a", "up1")
	store.SaveRoute("s2", "app-b", "up1")
	store.SaveRoute("s3", "app-a", "up2")

	routes, _ := store.ListRoutesByApp("app-a")
	if len(routes) != 2 {
		t.Errorf("expected 2 routes for app-a, got %d", len(routes))
	}
}

func TestSQLiteStore_MigrateRoute(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.SaveRoute("s1", "a1", "up1")
	store.MigrateRoute("s1", "a1", "up2")

	routes, _ := store.LoadRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].UpstreamID != "up2" {
		t.Errorf("expected upstream=up2 after migration, got %s", routes[0].UpstreamID)
	}
}

// ============================================================
// 7. Store 用户信息缓存测试
// ============================================================

func TestSQLiteStore_UserInfo(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	info := &UserInfo{
		SenderID:   "sender1",
		Name:       "Test User",
		Email:      "test@example.com",
		Department: "Engineering",
		FetchedAt:  time.Now(),
	}
	if err := store.SaveUserInfo(info); err != nil {
		t.Fatalf("SaveUserInfo failed: %v", err)
	}

	got, err := store.GetUserInfo("sender1")
	if err != nil {
		t.Fatalf("GetUserInfo failed: %v", err)
	}
	if got.Name != "Test User" {
		t.Errorf("expected name=Test User, got %s", got.Name)
	}

	list, err := store.ListUserInfo("Engineering", "")
	if err != nil {
		t.Fatalf("ListUserInfo failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 user, got %d", len(list))
	}
}

// ============================================================
// 8. Store 上游管理测试
// ============================================================

func TestSQLiteStore_UpstreamOperations(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	up := &Upstream{
		ID:            "up-1",
		Address:       "127.0.0.1",
		Port:          18790,
		Healthy:       true,
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
		Tags:          map[string]string{"type": "test"},
		Load:          map[string]interface{}{"cpu": 0.5},
	}
	if err := store.SaveUpstream(up); err != nil {
		t.Fatalf("SaveUpstream failed: %v", err)
	}

	loaded, err := store.LoadUpstreams()
	if err != nil {
		t.Fatalf("LoadUpstreams failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(loaded))
	}
	if loaded[0].Port != 18790 {
		t.Errorf("expected port=18790, got %d", loaded[0].Port)
	}

	store.DeleteUpstream("up-1")
	loaded, _ = store.LoadUpstreams()
	if len(loaded) != 0 {
		t.Errorf("expected 0 upstreams after delete, got %d", len(loaded))
	}
}

// ============================================================
// 9. 备份和恢复测试
// ============================================================

func TestSQLiteStore_Backup(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.LogAudit(&AuditEntry{Direction: "inbound", Action: "pass", ContentPreview: "backup test"})
	store.SaveRoute("s1", "a1", "up1")

	backupDir := filepath.Join(t.TempDir(), "backups")
	path, size, err := store.Backup(backupDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty backup path")
	}
	if size <= 0 {
		t.Fatal("expected positive backup size")
	}
	if !strings.HasPrefix(filepath.Base(path), "lobster-guard-") {
		t.Errorf("unexpected backup filename: %s", filepath.Base(path))
	}
}

func TestSQLiteStore_BackupAndRestore(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.LogAudit(&AuditEntry{Direction: "inbound", Action: "block", ContentPreview: "restore test"})
	store.SaveRoute("s1", "a1", "up1")

	backupDir := filepath.Join(t.TempDir(), "backups")
	backupPath, _, err := store.Backup(backupDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	if err := RestoreFromBackup(backupPath, restoredPath); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	db2, err := initDB(restoredPath)
	if err != nil {
		t.Fatalf("initDB on restored failed: %v", err)
	}
	defer db2.Close()
	store2 := NewSQLiteStore(db2, restoredPath)
	defer store2.Close()

	routes, _ := store2.LoadRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route in restored db, got %d", len(routes))
	}
}

func TestListBackups(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "backups")
	os.MkdirAll(backupDir, 0755)

	os.WriteFile(filepath.Join(backupDir, "lobster-guard-20260101-120000.db"), []byte("data1"), 0644)
	os.WriteFile(filepath.Join(backupDir, "lobster-guard-20260102-120000.db"), []byte("data22"), 0644)
	os.WriteFile(filepath.Join(backupDir, "unrelated.txt"), []byte("not a backup"), 0644)

	backups, err := ListBackups(backupDir)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}
	if backups[0].Name != "lobster-guard-20260102-120000.db" {
		t.Errorf("expected newest first, got %s", backups[0].Name)
	}
}

func TestCleanupOldBackups(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "backups")
	os.MkdirAll(backupDir, 0755)

	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(backupDir, "lobster-guard-2026010"+string(rune('1'+i))+"-120000.db"), []byte("data"), 0644)
	}

	deleted, err := CleanupOldBackups(backupDir, 3)
	if err != nil {
		t.Fatalf("CleanupOldBackups failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining, _ := ListBackups(backupDir)
	if len(remaining) != 3 {
		t.Errorf("expected 3 remaining, got %d", len(remaining))
	}
}

func TestDeleteBackup(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "backups")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "lobster-guard-20260101-120000.db"), []byte("data"), 0644)

	if err := DeleteBackup(backupDir, "lobster-guard-20260101-120000.db"); err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "lobster-guard-20260101-120000.db")); !os.IsNotExist(err) {
		t.Error("backup file should be deleted")
	}
}

func TestDeleteBackup_PathTraversal(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "backups")
	err := DeleteBackup(backupDir, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestListBackups_NonexistentDir(t *testing.T) {
	backups, err := ListBackups("/nonexistent/dir")
	if err != nil {
		t.Fatalf("should not error for non-existent dir: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected empty list, got %d", len(backups))
	}
}

// ============================================================
// 10. 健康检查增强测试
// ============================================================

func TestPerformHealthChecks_AllOK(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	cfg := &Config{
		StaticUpstreams: []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
	}
	db := store.RawDB()
	pool := NewUpstreamPool(cfg, db)

	result := PerformHealthChecks(store, pool, store.path)
	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %s (checks: db=%s up=%s disk=%s mem=%s gr=%s)",
			result.Status,
			result.Checks["database"].Status,
			result.Checks["upstream"].Status,
			result.Checks["disk"].Status,
			result.Checks["memory"].Status,
			result.Checks["goroutines"].Status)
	}
	if result.Checks["database"].Status != "ok" {
		t.Errorf("expected database=ok, got %s", result.Checks["database"].Status)
	}
	if result.Checks["memory"].Status != "ok" {
		t.Errorf("expected memory=ok, got %s", result.Checks["memory"].Status)
	}
	if result.Checks["goroutines"].Status != "ok" {
		t.Errorf("expected goroutines=ok, got %s", result.Checks["goroutines"].Status)
	}
}

func TestPerformHealthChecks_NilStore(t *testing.T) {
	cfg := &Config{}
	db, _ := initDB(filepath.Join(t.TempDir(), "nil.db"))
	defer db.Close()
	pool := NewUpstreamPool(cfg, db)

	result := PerformHealthChecks(nil, pool, "/tmp")
	if result.Checks["database"].Status != "ok" {
		t.Errorf("nil store should be ok (skip), got %s", result.Checks["database"].Status)
	}
}

func TestCheckUpstreams_Empty(t *testing.T) {
	cfg := &Config{}
	db, _ := initDB(filepath.Join(t.TempDir(), "empty.db"))
	defer db.Close()
	pool := NewUpstreamPool(cfg, db)

	item := checkUpstreams(pool)
	if item.Total != 0 {
		t.Errorf("expected 0 total, got %d", item.Total)
	}
	if item.Status != "warning" {
		t.Errorf("expected warning for empty pool, got %s", item.Status)
	}
}

func TestCheckMemory(t *testing.T) {
	item := checkMemory()
	if item.Status != "ok" {
		t.Errorf("expected ok for test memory usage, got %s", item.Status)
	}
	if item.AllocMB <= 0 {
		t.Error("expected positive alloc_mb")
	}
}

func TestCheckGoroutines(t *testing.T) {
	item := checkGoroutines()
	if item.Status != "ok" {
		t.Errorf("expected ok for goroutines, got %s", item.Status)
	}
	if item.Count <= 0 {
		t.Error("expected positive goroutine count")
	}
}

func TestCheckDisk(t *testing.T) {
	item := checkDisk("/tmp")
	if item.UsedPercent < 0 || item.UsedPercent > 100 {
		t.Errorf("disk usage out of range: %f", item.UsedPercent)
	}
}

// ============================================================
// 11. 优雅关闭测试
// ============================================================

func TestShutdownManager_IsShuttingDown(t *testing.T) {
	cfg := &Config{ShutdownTimeout: 5}
	sm := NewShutdownManager(cfg)

	if sm.IsShuttingDown() {
		t.Error("should not be shutting down initially")
	}

	go sm.Shutdown()
	time.Sleep(50 * time.Millisecond)

	if !sm.IsShuttingDown() {
		t.Error("should be shutting down after Shutdown()")
	}
}

func TestShutdownManager_DefaultTimeout(t *testing.T) {
	cfg := &Config{}
	sm := NewShutdownManager(cfg)
	if sm.shutdownTimeout != 30*time.Second {
		t.Errorf("expected 30s default timeout, got %v", sm.shutdownTimeout)
	}
}

func TestShutdownManager_CustomTimeout(t *testing.T) {
	cfg := &Config{ShutdownTimeout: 60}
	sm := NewShutdownManager(cfg)
	if sm.shutdownTimeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", sm.shutdownTimeout)
	}
}

// ============================================================
// 12. 健康检查 API（关闭中）测试
// ============================================================

func TestHealthzDuringShutdown(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	cfg := &Config{ShutdownTimeout: 2}
	db := store.RawDB()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)
	shutdownMgr := NewShutdownManager(cfg)

	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, store, shutdownMgr, nil)

	// Normal healthz
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	api.handleHealthz(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Trigger shutdown
	go shutdownMgr.Shutdown()
	time.Sleep(50 * time.Millisecond)

	// Shutting down healthz
	req2 := httptest.NewRequest("GET", "/healthz", nil)
	w2 := httptest.NewRecorder()
	api.handleHealthz(w2, req2)
	if w2.Code != 503 {
		t.Fatalf("expected 503 during shutdown, got %d", w2.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp["status"] != "shutting_down" {
		t.Errorf("expected status=shutting_down, got %v", resp["status"])
	}
}

// ============================================================
// 13. 备份 API 测试
// ============================================================

func TestBackupAPI_Create(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	backupDir := filepath.Join(t.TempDir(), "api_backups")
	cfg := &Config{BackupDir: backupDir, BackupMaxCount: 5}
	db := store.RawDB()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)

	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, store, nil, nil)

	req := httptest.NewRequest("POST", "/api/v1/backup", nil)
	w := httptest.NewRecorder()
	api.handleCreateBackup(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "created" {
		t.Errorf("expected status=created, got %v", resp["status"])
	}
	if resp["path"] == nil || resp["path"] == "" {
		t.Error("expected non-empty path")
	}
}

func TestBackupAPI_ListAndDelete(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	backupDir := filepath.Join(t.TempDir(), "api_backups")
	cfg := &Config{BackupDir: backupDir}
	db := store.RawDB()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)

	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, store, nil, nil)

	// Create backup
	req := httptest.NewRequest("POST", "/api/v1/backup", nil)
	w := httptest.NewRecorder()
	api.handleCreateBackup(w, req)
	if w.Code != 200 {
		t.Fatalf("create failed: %d", w.Code)
	}

	// List
	req2 := httptest.NewRequest("GET", "/api/v1/backups", nil)
	w2 := httptest.NewRecorder()
	api.handleListBackups(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("list failed: %d", w2.Code)
	}
	var listResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	total := int(listResp["total"].(float64))
	if total != 1 {
		t.Fatalf("expected 1 backup, got %d", total)
	}

	backups := listResp["backups"].([]interface{})
	backupName := backups[0].(map[string]interface{})["name"].(string)

	// Delete
	req3 := httptest.NewRequest("DELETE", "/api/v1/backups/"+backupName, nil)
	w3 := httptest.NewRecorder()
	api.handleDeleteBackup(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("delete failed: %d", w3.Code)
	}

	// Verify deleted
	req4 := httptest.NewRequest("GET", "/api/v1/backups", nil)
	w4 := httptest.NewRecorder()
	api.handleListBackups(w4, req4)
	var listResp2 map[string]interface{}
	json.Unmarshal(w4.Body.Bytes(), &listResp2)
	if int(listResp2["total"].(float64)) != 0 {
		t.Errorf("expected 0 backups after delete, got %v", listResp2["total"])
	}
}

// ============================================================
// 14. Store 接口兼容性测试
// ============================================================

func TestSQLiteStore_ImplementsStoreInterface(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Verify SQLiteStore implements Store interface
	var s Store = store
	if s == nil {
		t.Fatal("SQLiteStore should implement Store")
	}
}

// ============================================================
// 15. 新增配置字段测试
// ============================================================

func TestConfig_V42Fields(t *testing.T) {
	yamlStr := `
shutdown_timeout: 60
backup_dir: "/data/backups"
backup_max_count: 20
backup_auto_interval: 6
`
	tmpFile := filepath.Join(t.TempDir(), "test_config_v42.yaml")
	os.WriteFile(tmpFile, []byte(yamlStr), 0644)
	cfg, err := loadConfig(tmpFile)
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.ShutdownTimeout != 60 {
		t.Errorf("expected shutdown_timeout=60, got %d", cfg.ShutdownTimeout)
	}
	if cfg.BackupDir != "/data/backups" {
		t.Errorf("expected backup_dir=/data/backups, got %s", cfg.BackupDir)
	}
	if cfg.BackupMaxCount != 20 {
		t.Errorf("expected backup_max_count=20, got %d", cfg.BackupMaxCount)
	}
	if cfg.BackupAutoInterval != 6 {
		t.Errorf("expected backup_auto_interval=6, got %d", cfg.BackupAutoInterval)
	}
}

func TestConfig_V42Defaults(t *testing.T) {
	yamlStr := `inbound_listen: ":8443"`
	tmpFile := filepath.Join(t.TempDir(), "test_config_v42_defaults.yaml")
	os.WriteFile(tmpFile, []byte(yamlStr), 0644)
	cfg, err := loadConfig(tmpFile)
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.ShutdownTimeout != 0 {
		t.Errorf("expected default 0, got %d", cfg.ShutdownTimeout)
	}
	if cfg.BackupDir != "" {
		t.Errorf("expected empty, got %s", cfg.BackupDir)
	}
	if cfg.BackupMaxCount != 0 {
		t.Errorf("expected 0, got %d", cfg.BackupMaxCount)
	}
	if cfg.BackupAutoInterval != 0 {
		t.Errorf("expected 0, got %d", cfg.BackupAutoInterval)
	}
}

// ============================================================
// 16. 健康检查响应格式测试
// ============================================================

func TestHealthzResponse_Format(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	cfg := &Config{}
	db := store.RawDB()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)

	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, store, nil, nil)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	api.handleHealthz(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Verify checks field exists with all dimensions
	checks, ok := resp["checks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected checks object in response")
	}
	for _, dim := range []string{"database", "upstream", "disk", "memory", "goroutines"} {
		if checks[dim] == nil {
			t.Errorf("expected check dimension: %s", dim)
		}
	}

	// Verify version
	if resp["version"] == nil {
		t.Error("expected version in response")
	}

	// Verify uptime
	if resp["uptime"] == nil {
		t.Error("expected uptime in response")
	}
}

// ============================================================
// 17. RawDB 兼容性测试
// ============================================================

func TestSQLiteStore_RawDB(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	db := store.RawDB()
	if db == nil {
		t.Fatal("RawDB should not be nil")
	}

	// Should be able to query through raw DB
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count)
	if err != nil {
		t.Fatalf("RawDB query failed: %v", err)
	}
}
