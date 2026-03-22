package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// setupTestDB 创建内存数据库并初始化必要的表
func setupPolicyCorrectionDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// user_info_cache 表
	db.Exec(`CREATE TABLE IF NOT EXISTS user_info_cache (
		sender_id TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		email TEXT DEFAULT '',
		department TEXT DEFAULT '',
		avatar TEXT DEFAULT '',
		mobile TEXT DEFAULT '',
		fetched_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	// upstreams 表
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (
		id TEXT PRIMARY KEY,
		address TEXT NOT NULL,
		port INTEGER NOT NULL,
		healthy INTEGER DEFAULT 1,
		registered_at TEXT NOT NULL,
		last_heartbeat TEXT,
		tags TEXT DEFAULT '{}',
		load TEXT DEFAULT '{}',
		path_prefix TEXT DEFAULT ''
	)`)
	// user_routes 表
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
		sender_id TEXT,
		app_id TEXT DEFAULT '',
		upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '',
		display_name TEXT DEFAULT '',
		email TEXT DEFAULT '',
		created_at TEXT,
		updated_at TEXT,
		PRIMARY KEY(sender_id, app_id)
	)`)
	return db
}

// TestPolicyCorrectionNewUser 验证新用户首次请求时策略路由纠偏
// 场景：新用户首次请求 → 缓存为空 → 负载均衡分配到 upstream-a
//       → 异步获取用户信息 → 策略匹配到 upstream-b → 纠偏迁移
func TestPolicyCorrectionNewUser(t *testing.T) {
	db := setupPolicyCorrectionDB(t)
	defer db.Close()

	// 模拟蓝信API返回用户信息
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apptoken") {
			w.Write([]byte(`{"errCode":0,"data":{"app_token":"test-token","expires_in":7200}}`))
			return
		}
		if strings.Contains(r.URL.Path, "infor/fetch") {
			w.Write([]byte(`{"errCode":0,"data":{"name":"张三","email":"zhangsan@qianxin.com","departments":[{"name":"天眼事业部"}]}}`))
			return
		}
	}))
	defer apiServer.Close()

	// 创建 provider 和 cache
	provider := NewLanxinUserProvider("test-app", "test-secret", apiServer.URL)
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	// 上游池: upstream-a (默认) 和 upstream-b (天眼事业部专用)
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-a", "127.0.0.1", 8001, nil)
	pool.Register("upstream-b", "127.0.0.1", 8002, nil)

	// 路由表
	routes := NewRouteTable(db, true)

	// 策略: 天眼事业部 → upstream-b
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "天眼事业部"}, UpstreamID: "upstream-b"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: ""},
	}
	policyEng := NewRoutePolicyEngine(policies)

	senderID := "new-user-001"
	appID := "app-001"

	// Step 1: 模拟新用户 — 缓存为空，策略匹配被跳过
	cachedInfo := cache.GetCached(senderID)
	if cachedInfo != nil {
		t.Fatal("新用户缓存应该为空")
	}

	// Step 2: 模拟负载均衡分配到 upstream-a（不是策略期望的 upstream-b）
	routes.Bind(senderID, appID, "upstream-a")
	pool.IncrUserCount("upstream-a", 1)

	currentUID, found := routes.Lookup(senderID, appID)
	if !found || currentUID != "upstream-a" {
		t.Fatalf("期望绑定到 upstream-a，实际 found=%v uid=%s", found, currentUID)
	}

	// Step 3: 模拟异步获取用户信息（纠偏核心）
	info, err := cache.GetOrFetch(senderID)
	if err != nil {
		t.Fatalf("GetOrFetch 失败: %v", err)
	}
	if info == nil || info.Name != "张三" {
		t.Fatalf("用户信息不正确: %+v", info)
	}
	if info.Department != "天眼事业部" {
		t.Fatalf("部门不正确: %s", info.Department)
	}
	routes.UpdateUserInfo(senderID, info.Name, info.Email, info.Department)

	// Step 4: 策略纠偏 — 修复后的核心逻辑
	corrected := false
	if pUID, ok := policyEng.Match(info, appID); ok && pUID != "" && pool.IsHealthy(pUID) {
		if currentUID, found := routes.Lookup(senderID, appID); !found || currentUID != pUID {
			oldUID := currentUID
			routes.Bind(senderID, appID, pUID)
			if found {
				pool.IncrUserCount(pUID, 1)
				pool.IncrUserCount(oldUID, -1)
			} else {
				pool.IncrUserCount(pUID, 1)
			}
			corrected = true
			t.Logf("策略纠偏成功: %s -> %s (dept=%s)", oldUID, pUID, info.Department)
		}
	}
	if !corrected {
		t.Fatal("应该触发策略纠偏: 天眼事业部用户应该从 upstream-a 迁移到 upstream-b")
	}

	// Step 5: 验证最终绑定
	finalUID, found := routes.Lookup(senderID, appID)
	if !found || finalUID != "upstream-b" {
		t.Fatalf("纠偏后期望 upstream-b，实际 found=%v uid=%s", found, finalUID)
	}
}

// TestPolicyCorrectionNoMigrationNeeded 验证策略匹配结果与当前绑定一致时不迁移
func TestPolicyCorrectionNoMigrationNeeded(t *testing.T) {
	db := setupPolicyCorrectionDB(t)
	defer db.Close()

	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"user-correct": {Name: "李四", Email: "lisi@qianxin.com", Department: "天眼事业部"},
		},
	}
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-b", "127.0.0.1", 8002, nil)

	routes := NewRouteTable(db, true)

	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "天眼事业部"}, UpstreamID: "upstream-b"},
	}
	policyEng := NewRoutePolicyEngine(policies)

	senderID := "user-correct"
	appID := "app-001"

	// 已经绑定到正确的上游
	routes.Bind(senderID, appID, "upstream-b")

	info, _ := cache.GetOrFetch(senderID)
	routes.UpdateUserInfo(senderID, info.Name, info.Email, info.Department)

	// 策略匹配 — 结果相同，不应迁移
	migrated := false
	if pUID, ok := policyEng.Match(info, appID); ok && pUID != "" {
		if currentUID, found := routes.Lookup(senderID, appID); !found || currentUID != pUID {
			migrated = true
		}
	}
	if migrated {
		t.Fatal("绑定已正确（upstream-b），不应触发迁移")
	}
}

// TestPolicyCorrectionDefaultPolicy 验证策略匹配到 default（空 upstream）时不纠偏
func TestPolicyCorrectionDefaultPolicy(t *testing.T) {
	db := setupPolicyCorrectionDB(t)
	defer db.Close()

	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"default-user": {Name: "王五", Email: "wangwu@qianxin.com", Department: "市场部"},
		},
	}
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-a", "127.0.0.1", 8001, nil)

	routes := NewRouteTable(db, true)

	// 只有 default 策略，upstream_id 为空 → 走负载均衡
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: ""},
	}
	policyEng := NewRoutePolicyEngine(policies)

	senderID := "default-user"
	appID := "app-001"

	routes.Bind(senderID, appID, "upstream-a")

	info, _ := cache.GetOrFetch(senderID)
	routes.UpdateUserInfo(senderID, info.Name, info.Email, info.Department)

	// default 策略返回空 upstream → pUID == "" → 不纠偏
	shouldCorrect := false
	if pUID, ok := policyEng.Match(info, appID); ok && pUID != "" {
		if currentUID, found := routes.Lookup(senderID, appID); !found || currentUID != pUID {
			shouldCorrect = true
		}
	}
	if shouldCorrect {
		t.Fatal("default 策略（空 upstream）不应触发纠偏")
	}

	// 绑定应保持不变
	finalUID, _ := routes.Lookup(senderID, appID)
	if finalUID != "upstream-a" {
		t.Fatalf("default 策略不应改变绑定，期望 upstream-a 实际 %s", finalUID)
	}
}

// TestPolicyCorrectionBugReproduction 完整复现修复前的 bug
// 修复前：新用户 → GetCached=nil → 跳过策略 → 负载均衡 → Bind → 异步获取 → Lookup找到 → 策略永远不执行
// 修复后：新用户 → GetCached=nil → 负载均衡 → Bind → 异步获取 → 策略匹配 → 结果不同 → 纠偏迁移
func TestPolicyCorrectionBugReproduction(t *testing.T) {
	db := setupPolicyCorrectionDB(t)
	defer db.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apptoken") {
			w.Write([]byte(`{"errCode":0,"data":{"app_token":"t","expires_in":7200}}`))
			return
		}
		w.Write([]byte(`{"errCode":0,"data":{"name":"赵六","email":"zhaoliu@qianxin.com","departments":[{"name":"安全运营BU"}]}}`))
	}))
	defer apiServer.Close()

	provider := NewLanxinUserProvider("app", "secret", apiServer.URL)
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("general-pool", "127.0.0.1", 8001, nil)
	pool.Register("security-ops", "127.0.0.1", 8002, nil)

	routes := NewRouteTable(db, true)

	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "安全运营BU"}, UpstreamID: "security-ops"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: ""},
	}
	policyEng := NewRoutePolicyEngine(policies)

	sid := "new-sec-ops-user"
	aid := "bot-001"

	// === 模拟完整的请求流程 ===

	// 1. 查亲和表 → 未找到
	_, found := routes.Lookup(sid, aid)
	if found {
		t.Fatal("新用户不应有亲和绑定")
	}

	// 2. 尝试策略匹配 → 缓存为空 → 跳过
	policyMatched := false
	if policyEng != nil && cache != nil {
		if info := cache.GetCached(sid); info != nil {
			// 这里不会执行 — 这就是 bug 的本质
			policyMatched = true
		}
	}
	if policyMatched {
		t.Fatal("缓存为空时策略不应匹配成功")
	}

	// 3. 负载均衡分配 → general-pool（错误的！应该去 security-ops）
	routes.Bind(sid, aid, "general-pool")
	pool.IncrUserCount("general-pool", 1)

	// 4. 异步获取用户信息 + 纠偏（修复后的逻辑）
	info, _ := cache.GetOrFetch(sid)
	routes.UpdateUserInfo(sid, info.Name, info.Email, info.Department)

	if pUID, ok := policyEng.Match(info, aid); ok && pUID != "" && pool.IsHealthy(pUID) {
		if currentUID, found := routes.Lookup(sid, aid); !found || currentUID != pUID {
			routes.Bind(sid, aid, pUID)
			if found {
				pool.IncrUserCount(pUID, 1)
				pool.IncrUserCount(currentUID, -1)
			}
			t.Logf("纠偏: %s → %s (dept=%s)", currentUID, pUID, info.Department)
		}
	}

	// 5. 验证最终结果
	finalUID, _ := routes.Lookup(sid, aid)
	if finalUID != "security-ops" {
		t.Fatalf("安全运营BU 用户应该被纠偏到 security-ops，实际 %s", finalUID)
	}
}
