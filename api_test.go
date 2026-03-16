// api_test.go — ManagementAPI、所有 HTTP handler 测试
// lobster-guard v4.0 代码拆分
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// 管理 API 测试
// ============================================================

func setupMgmtAPI(t *testing.T) (*ManagementAPI, func()) {
	t.Helper()
	tmpDB := "/tmp/lobster-guard-test-mgmt-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".db"
	cfg := &Config{
		StaticUpstreams:       []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		ManagementToken:       "mgmt-token",
		RegistrationToken:     "reg-token",
		HeartbeatIntervalSec:  10,
		HeartbeatTimeoutCount: 3,
		RoutePersist:          false,
	}
	db, _ := initDB(tmpDB)
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	outEngine := NewOutboundRuleEngine(nil)
	engine := NewRuleEngine()
	channel := NewGenericPlugin("", "")
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil)
	cleanup := func() { logger.Close(); db.Close(); os.Remove(tmpDB) }
	return api, cleanup
}

func TestManagementAPIHealthz(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("healthz 期望 200，实际 %d", rec.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Fatalf("status 期望 healthy，实际 %v", resp["status"])
	}
}

func TestManagementAPIAuth(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 无 token
	req := httptest.NewRequest("GET", "/api/v1/upstreams", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 401 { t.Fatalf("无 token 期望 401，实际 %d", rec.Code) }

	// 有正确 token
	req = httptest.NewRequest("GET", "/api/v1/upstreams", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("有 token 期望 200，实际 %d", rec.Code) }
}

func TestManagementAPIRegisterFlow(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 注册
	body := `{"id":"claw-1","address":"10.0.0.1","port":18790,"tags":{"env":"test"}}`
	req := httptest.NewRequest("POST", "/api/v1/register", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer reg-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("注册期望 200，实际 %d body=%s", rec.Code, rec.Body.String()) }

	// 心跳
	body = `{"id":"claw-1","load":{"cpu":30}}`
	req = httptest.NewRequest("POST", "/api/v1/heartbeat", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer reg-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("心跳期望 200，实际 %d", rec.Code) }

	// 注销
	body = `{"id":"claw-1"}`
	req = httptest.NewRequest("POST", "/api/v1/deregister", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer reg-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("注销期望 200，实际 %d", rec.Code) }
}

func TestManagementAPIRoutes(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 绑定路由
	body := `{"sender_id":"user-1","upstream_id":"up-1"}`
	req := httptest.NewRequest("POST", "/api/v1/routes/bind", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("绑定期望 200，实际 %d", rec.Code) }

	// 查询路由
	req = httptest.NewRequest("GET", "/api/v1/routes", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("查询期望 200，实际 %d", rec.Code) }

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 1 {
		t.Fatalf("期望1条路由，实际 %v", resp["total"])
	}
}

func TestManagementAPIStats(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("stats 期望 200，实际 %d", rec.Code) }

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["version"] != AppVersion {
		t.Fatalf("version 期望 %s，实际 %v", AppVersion, resp["version"])
	}
}

// ============================================================
// v3.8 API 测试
// ============================================================

func TestAPIBatchBind(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 批量绑定（按条目列表）
	body := `{
		"app_id": "app-alpha",
		"upstream_id": "up-1",
		"entries": [
			{"sender_id": "user-001", "display_name": "张三", "department": "安全研究院"},
			{"sender_id": "user-002", "display_name": "李四", "department": "安全研究院"}
		]
	}`
	req := httptest.NewRequest("POST", "/api/v1/routes/batch-bind", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("batch-bind 期望 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if int(resp["count"].(float64)) != 2 {
		t.Fatalf("期望绑定2条，实际 %v", resp["count"])
	}

	// 验证路由
	req = httptest.NewRequest("GET", "/api/v1/routes?app_id=app-alpha", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("查询期望 200，实际 %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 2 {
		t.Fatalf("app-alpha 期望2条路由，实际 %v", resp["total"])
	}
}

func TestAPIRouteStats(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 先绑定一些路由
	for _, body := range []string{
		`{"sender_id":"u1","app_id":"app-a","upstream_id":"up-1"}`,
		`{"sender_id":"u2","app_id":"app-a","upstream_id":"up-1"}`,
		`{"sender_id":"u3","app_id":"app-b","upstream_id":"up-1"}`,
	} {
		req := httptest.NewRequest("POST", "/api/v1/routes/bind", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer mgmt-token")
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("绑定失败 %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 获取统计
	req := httptest.NewRequest("GET", "/api/v1/routes/stats", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("stats 期望 200，实际 %d", rec.Code)
	}

	var stats RouteStats
	json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.TotalRoutes != 3 {
		t.Fatalf("TotalRoutes 期望3，实际 %d", stats.TotalRoutes)
	}
	if stats.TotalUsers != 3 {
		t.Fatalf("TotalUsers 期望3，实际 %d", stats.TotalUsers)
	}
}

func TestAPIUnbindRoute(t *testing.T) {
	api, cleanup := setupMgmtAPI(t)
	defer cleanup()

	// 绑定
	body := `{"sender_id":"user-1","app_id":"app-x","upstream_id":"up-1"}`
	req := httptest.NewRequest("POST", "/api/v1/routes/bind", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("绑定期望 200，实际 %d", rec.Code) }

	// 解绑
	body = `{"sender_id":"user-1","app_id":"app-x"}`
	req = httptest.NewRequest("POST", "/api/v1/routes/unbind", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Fatalf("解绑期望 200，实际 %d", rec.Code) }

	// 验证已解绑
	req = httptest.NewRequest("GET", "/api/v1/routes", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 0 {
		t.Fatalf("解绑后期望0条路由，实际 %v", resp["total"])
	}
}

func TestDBMigration(t *testing.T) {
	tmpDB := "/tmp/lobster-guard-test-migration-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".db"
	defer os.Remove(tmpDB)

	// 创建旧 schema 的数据库
	db, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil { t.Fatal(err) }

	// 创建旧表（只有 sender_id 主键）
	_, err = db.Exec(`CREATE TABLE user_routes (
		sender_id TEXT PRIMARY KEY,
		upstream_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil { t.Fatal(err) }

	// 插入旧数据
	now := time.Now().Format(time.RFC3339)
	db.Exec(`INSERT INTO user_routes (sender_id, upstream_id, created_at, updated_at) VALUES(?,?,?,?)`,
		"old-user-1", "upstream-a", now, now)
	db.Exec(`INSERT INTO user_routes (sender_id, upstream_id, created_at, updated_at) VALUES(?,?,?,?)`,
		"old-user-2", "upstream-b", now, now)
	db.Close()

	// 重新打开，触发迁移
	db2, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil { t.Fatal(err) }
	defer db2.Close()

	migrateUserRoutes(db2)

	// 验证新 schema
	var cnt int
	err = db2.QueryRow(`SELECT COUNT(*) FROM user_routes`).Scan(&cnt)
	if err != nil { t.Fatal(err) }
	if cnt != 2 {
		t.Fatalf("迁移后应有2条数据，实际 %d", cnt)
	}

	// 验证 app_id 列存在且默认为空
	var appID string
	err = db2.QueryRow(`SELECT app_id FROM user_routes WHERE sender_id='old-user-1'`).Scan(&appID)
	if err != nil { t.Fatal(err) }
	if appID != "" {
		t.Fatalf("旧数据的 app_id 应为空，实际 %q", appID)
	}

	// 验证复合主键可用（同 sender_id 不同 app_id）
	db2.Exec(`INSERT INTO user_routes (sender_id, app_id, upstream_id, department, display_name, created_at, updated_at) VALUES(?,?,?,'','',?,?)`,
		"old-user-1", "new-app", "upstream-c", now, now)
	err = db2.QueryRow(`SELECT COUNT(*) FROM user_routes WHERE sender_id='old-user-1'`).Scan(&cnt)
	if err != nil { t.Fatal(err) }
	if cnt != 2 {
		t.Fatalf("复合主键应允许同 sender_id 不同 app_id，实际 %d", cnt)
	}

	// 验证 RouteTable 加载
	rt := NewRouteTable(db2, true)
	if rt.Count() != 3 {
		t.Fatalf("RouteTable 应加载3条路由，实际 %d", rt.Count())
	}

	uid, found := rt.Lookup("old-user-1", "")
	if !found || uid != "upstream-a" {
		t.Fatalf("旧数据路由查找失败: found=%v uid=%s", found, uid)
	}

	uid, found = rt.Lookup("old-user-1", "new-app")
	if !found || uid != "upstream-c" {
		t.Fatalf("新数据路由查找失败: found=%v uid=%s", found, uid)
	}
}

func TestInboundRoutingWithAppID(t *testing.T) {
	// 测试入站路由使用复合键
	rt := NewRouteTable(nil, false)

	// 模拟两个 Bot 的用户路由
	rt.Bind("sender-001", "bot-alpha", "upstream-a")
	rt.Bind("sender-001", "bot-beta", "upstream-b")

	// 查找 bot-alpha 的路由
	uid, found := rt.Lookup("sender-001", "bot-alpha")
	if !found || uid != "upstream-a" {
		t.Fatalf("bot-alpha 路由查找失败: found=%v uid=%s", found, uid)
	}

	// 查找 bot-beta 的路由
	uid, found = rt.Lookup("sender-001", "bot-beta")
	if !found || uid != "upstream-b" {
		t.Fatalf("bot-beta 路由查找失败: found=%v uid=%s", found, uid)
	}

	// 迁移 bot-alpha 的路由
	ok := rt.Migrate("sender-001", "bot-alpha", "upstream-a", "upstream-c")
	if !ok {
		t.Fatal("迁移应成功")
	}

	// bot-alpha 已迁移
	uid, found = rt.Lookup("sender-001", "bot-alpha")
	if !found || uid != "upstream-c" {
		t.Fatalf("迁移后 bot-alpha 应指向 upstream-c，实际 uid=%s", uid)
	}

	// bot-beta 不受影响
	uid, found = rt.Lookup("sender-001", "bot-beta")
	if !found || uid != "upstream-b" {
		t.Fatalf("bot-beta 路由不应受影响，实际 uid=%s", uid)
	}
}

func TestRouteTablePersistWithDB(t *testing.T) {
	tmpDB := "/tmp/lobster-guard-test-persist-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".db"
	defer os.Remove(tmpDB)

	db, err := initDB(tmpDB)
	if err != nil { t.Fatal(err) }

	// 绑定一些路由
	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app-alpha", "upstream-a")
	rt.BindWithMeta("user2", "app-alpha", "upstream-a", "安全研究院", "张三")

	// 重新加载
	rt2 := NewRouteTable(db, true)
	if rt2.Count() != 2 {
		t.Fatalf("从 DB 恢复应有2条路由，实际 %d", rt2.Count())
	}

	uid, found := rt2.Lookup("user1", "app-alpha")
	if !found || uid != "upstream-a" {
		t.Fatalf("恢复后路由查找失败: found=%v uid=%s", found, uid)
	}

	// 验证 ListRoutes 包含完整信息
	entries := rt2.ListRoutes()
	foundMeta := false
	for _, e := range entries {
		if e.SenderID == "user2" && e.Department == "安全研究院" && e.DisplayName == "张三" {
			foundMeta = true
		}
	}
	if !foundMeta {
		t.Fatal("ListRoutes 应包含部门和显示名信息")
	}

	db.Close()
}



// ============================================================
// v3.11 API 端点测试
// ============================================================

func TestAPIRuleBindings(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	cfg := &Config{
		RuleBindings: []RuleBindingConfig{
			{AppID: "bot-1", Groups: []string{"jailbreak"}},
			{AppID: "*", Groups: []string{"injection"}},
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(
		[]InboundRuleConfig{
			{Name: "j1", Patterns: []string{"jailbreak"}, Action: "block", Group: "jailbreak"},
			{Name: "i1", Patterns: []string{"inject"}, Action: "block", Group: "injection"},
		},
		"test", nil, cfg.RuleBindings,
	)
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()

	// GET /api/v1/rule-bindings
	resp, err := server.Client().Get(server.URL + "/api/v1/rule-bindings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	bindings, ok := result["bindings"].([]interface{})
	if !ok || len(bindings) != 2 {
		t.Errorf("expected 2 bindings, got %v", result["bindings"])
	}
}

func TestAPIRuleBindingsTest(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	cfg := &Config{
		RuleBindings: []RuleBindingConfig{
			{AppID: "bot-1", Groups: []string{"jailbreak", "injection"}},
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(
		[]InboundRuleConfig{
			{Name: "j1", Patterns: []string{"jailbreak"}, Action: "block", Group: "jailbreak"},
			{Name: "i1", Patterns: []string{"inject"}, Action: "block", Group: "injection"},
			{Name: "p1", Patterns: []string{"pii"}, Action: "warn", Group: "pii"},
		},
		"test", nil, cfg.RuleBindings,
	)
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()

	// POST /api/v1/rule-bindings/test
	body := `{"app_id":"bot-1"}`
	resp, err := server.Client().Post(server.URL+"/api/v1/rule-bindings/test", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	rulesCount, _ := result["rules_count"].(float64)
	if rulesCount != 2 {
		t.Errorf("bot-1 should have 2 applicable rules, got %v", rulesCount)
	}
}

func TestAPIRuleHitsWithGroup(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	ruleHits := NewRuleHitStats()
	ruleHits.RecordWithGroup("rule_a", "jailbreak")
	ruleHits.RecordWithGroup("rule_b", "injection")
	ruleHits.RecordWithGroup("rule_c", "jailbreak")

	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, ruleHits, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, ruleHits, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()

	// GET /api/v1/rules/hits?group=jailbreak
	resp, err := server.Client().Get(server.URL + "/api/v1/rules/hits?group=jailbreak")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var details []RuleHitDetail
	json.NewDecoder(resp.Body).Decode(&details)
	if len(details) != 2 {
		t.Errorf("expected 2 jailbreak hits, got %d", len(details))
	}
}

func TestAPIOutboundRulesWithPII(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	cfg := &Config{
		OutboundPIIPatterns: []OutboundPIIPatternConfig{
			{Name: "邮箱", Pattern: `test@example`},
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(nil, "test", cfg.OutboundPIIPatterns, nil)
	engine.Reload([]InboundRuleConfig{}, "test")
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/api/v1/outbound-rules")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	piiPatterns, ok := result["pii_patterns"].([]interface{})
	if !ok || len(piiPatterns) != 1 {
		t.Errorf("expected 1 pii pattern in response, got %v", result["pii_patterns"])
	}
}

func TestAPIInboundRulesGroupCounts(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	configs := []InboundRuleConfig{
		{Name: "r1", Patterns: []string{"a"}, Action: "block", Group: "jailbreak"},
		{Name: "r2", Patterns: []string{"b"}, Action: "block", Group: "jailbreak"},
		{Name: "r3", Patterns: []string{"c"}, Action: "block", Group: "injection"},
		{Name: "r4", Patterns: []string{"d"}, Action: "warn"},
	}
	engine := NewRuleEngineFromConfig(configs, "test")
	outEngine := NewOutboundRuleEngine(nil)
	inbound := NewInboundProxy(cfg, nil, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil)

	server := httptest.NewServer(api)
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/api/v1/inbound-rules")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	groupCounts, ok := result["group_counts"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected group_counts in response, got %v", result)
	}
	jailbreakCount, _ := groupCounts["jailbreak"].(float64)
	if jailbreakCount != 2 {
		t.Errorf("expected 2 jailbreak rules, got %v", jailbreakCount)
	}
	injectionCount, _ := groupCounts["injection"].(float64)
	if injectionCount != 1 {
		t.Errorf("expected 1 injection rule, got %v", injectionCount)
	}
}

func TestConfigYAMLRuleBindings(t *testing.T) {
	yamlData := `
rule_bindings:
  - app_id: "bot-1"
    groups: ["jailbreak", "injection"]
  - app_id: "*"
    groups: ["jailbreak"]
outbound_pii_patterns:
  - name: "邮箱"
    pattern: "[a-zA-Z0-9]+@[a-zA-Z0-9]+\\.[a-zA-Z]{2,}"
inbound_rules:
  - name: "test_regex"
    patterns: ["test\\d+"]
    action: "block"
    type: "regex"
    group: "injection"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(yamlData)
	tmpFile.Close()

	cfg, err := loadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if len(cfg.RuleBindings) != 2 {
		t.Errorf("expected 2 rule bindings, got %d", len(cfg.RuleBindings))
	}
	if cfg.RuleBindings[0].AppID != "bot-1" {
		t.Errorf("expected app_id 'bot-1', got '%s'", cfg.RuleBindings[0].AppID)
	}
	if len(cfg.OutboundPIIPatterns) != 1 {
		t.Errorf("expected 1 pii pattern, got %d", len(cfg.OutboundPIIPatterns))
	}
	if len(cfg.InboundRules) != 1 {
		t.Errorf("expected 1 inbound rule, got %d", len(cfg.InboundRules))
	}
	if cfg.InboundRules[0].Type != "regex" {
		t.Errorf("expected type 'regex', got '%s'", cfg.InboundRules[0].Type)
	}
	if cfg.InboundRules[0].Group != "injection" {
		t.Errorf("expected group 'injection', got '%s'", cfg.InboundRules[0].Group)
	}
}

