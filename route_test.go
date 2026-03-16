// route_test.go — RouteTable、UpstreamPool、UserInfoCache、RoutePolicyEngine 测试
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

// ============================================================
// 路由表测试
// ============================================================

func TestRouteTable(t *testing.T) {
	rt := NewRouteTable(nil, false)

	_, found := rt.Lookup("user1", "")
	if found {
		t.Fatal("空表不应找到路由")
	}

	rt.Bind("user1", "", "upstream-a")
	uid, found := rt.Lookup("user1", "")
	if !found || uid != "upstream-a" {
		t.Fatalf("绑定后查找失败: found=%v uid=%s", found, uid)
	}
	if rt.Count() != 1 {
		t.Fatalf("期望1条路由，实际 %d", rt.Count())
	}
	if rt.CountByUpstream("upstream-a") != 1 {
		t.Fatal("上游计数错误")
	}

	ok := rt.Migrate("user1", "", "upstream-a", "upstream-b")
	if !ok {
		t.Fatal("迁移应成功")
	}
	uid, _ = rt.Lookup("user1", "")
	if uid != "upstream-b" {
		t.Fatalf("迁移后应指向 upstream-b，实际 %s", uid)
	}

	ok = rt.Migrate("user1", "", "upstream-a", "upstream-c")
	if ok {
		t.Fatal("来源不匹配不应成功")
	}

	rt.Unbind("user1", "")
	_, found = rt.Lookup("user1", "")
	if found {
		t.Fatal("解绑后不应找到")
	}
}

func TestRouteTableListRoutes(t *testing.T) {
	rt := NewRouteTable(nil, false)
	rt.Bind("u1", "", "up-a")
	rt.Bind("u2", "", "up-b")
	rt.Bind("u3", "", "up-a")
	routes := rt.ListRoutes()
	if len(routes) != 3 {
		t.Fatalf("期望3条，实际 %d", len(routes))
	}
}

// ============================================================
// v3.8 多 Bot 亲和路由测试
// ============================================================

func TestRouteTableCompoundKey(t *testing.T) {
	rt := NewRouteTable(nil, false)

	// 同一用户绑定到不同 Bot 的不同上游
	rt.Bind("user1", "app-alpha", "upstream-a")
	rt.Bind("user1", "app-beta", "upstream-b")

	uid, found := rt.Lookup("user1", "app-alpha")
	if !found || uid != "upstream-a" {
		t.Fatalf("(user1, app-alpha) 应指向 upstream-a，实际 found=%v uid=%s", found, uid)
	}

	uid, found = rt.Lookup("user1", "app-beta")
	if !found || uid != "upstream-b" {
		t.Fatalf("(user1, app-beta) 应指向 upstream-b，实际 found=%v uid=%s", found, uid)
	}

	if rt.Count() != 2 {
		t.Fatalf("期望2条路由，实际 %d", rt.Count())
	}

	// 解绑其中一个
	rt.Unbind("user1", "app-alpha")
	_, found = rt.Lookup("user1", "app-alpha")
	if found {
		t.Fatal("解绑后 (user1, app-alpha) 不应找到")
	}

	// 另一个仍在
	uid, found = rt.Lookup("user1", "app-beta")
	if !found || uid != "upstream-b" {
		t.Fatal("解绑 alpha 不应影响 beta")
	}
}

func TestRouteTableFallback(t *testing.T) {
	rt := NewRouteTable(nil, false)

	// 绑定 (user1, "") 作为默认路由
	rt.Bind("user1", "", "upstream-default")

	// 精确匹配 app-alpha 没有，应 fallback 到 ""
	uid, found := rt.Lookup("user1", "app-alpha")
	if !found || uid != "upstream-default" {
		t.Fatalf("fallback 应返回 upstream-default，实际 found=%v uid=%s", found, uid)
	}

	// 绑定精确路由后，精确匹配优先
	rt.Bind("user1", "app-alpha", "upstream-alpha")
	uid, found = rt.Lookup("user1", "app-alpha")
	if !found || uid != "upstream-alpha" {
		t.Fatalf("精确匹配应优先，实际 uid=%s", uid)
	}

	// 其他 appID 仍 fallback
	uid, found = rt.Lookup("user1", "app-beta")
	if !found || uid != "upstream-default" {
		t.Fatalf("app-beta 应 fallback 到 upstream-default，实际 uid=%s", uid)
	}

	// appID 为空直接匹配 ""
	uid, found = rt.Lookup("user1", "")
	if !found || uid != "upstream-default" {
		t.Fatalf("appID 空应匹配 upstream-default，实际 uid=%s", uid)
	}
}

func TestRouteTableBatchBind(t *testing.T) {
	rt := NewRouteTable(nil, false)

	entries := []RouteEntry{
		{SenderID: "user-001", AppID: "app-alpha", UpstreamID: "upstream-a", Department: "安全研究院", DisplayName: "张三"},
		{SenderID: "user-002", AppID: "app-alpha", UpstreamID: "upstream-a", Department: "安全研究院", DisplayName: "李四"},
		{SenderID: "user-003", AppID: "app-beta", UpstreamID: "upstream-b", Department: "产品中心", DisplayName: "王五"},
	}
	rt.BindBatch(entries)

	if rt.Count() != 3 {
		t.Fatalf("期望3条路由，实际 %d", rt.Count())
	}

	uid, found := rt.Lookup("user-001", "app-alpha")
	if !found || uid != "upstream-a" {
		t.Fatalf("user-001 应绑定到 upstream-a, found=%v uid=%s", found, uid)
	}

	uid, found = rt.Lookup("user-003", "app-beta")
	if !found || uid != "upstream-b" {
		t.Fatalf("user-003 应绑定到 upstream-b, found=%v uid=%s", found, uid)
	}
}

func TestRouteTableMigration(t *testing.T) {
	rt := NewRouteTable(nil, false)

	rt.Bind("user1", "app-alpha", "upstream-a")

	// 迁移，保留 appID
	ok := rt.Migrate("user1", "app-alpha", "upstream-a", "upstream-b")
	if !ok {
		t.Fatal("迁移应成功")
	}

	uid, found := rt.Lookup("user1", "app-alpha")
	if !found || uid != "upstream-b" {
		t.Fatalf("迁移后应指向 upstream-b，实际 uid=%s", uid)
	}

	// 来源不匹配
	ok = rt.Migrate("user1", "app-alpha", "upstream-a", "upstream-c")
	if ok {
		t.Fatal("来源不匹配不应成功")
	}
}

func TestRouteLookupByApp(t *testing.T) {
	rt := NewRouteTable(nil, false)

	rt.Bind("user1", "app-alpha", "upstream-a")
	rt.Bind("user2", "app-alpha", "upstream-b")
	rt.Bind("user3", "app-beta", "upstream-a")

	alphaRoutes := rt.ListByApp("app-alpha")
	if len(alphaRoutes) != 2 {
		t.Fatalf("app-alpha 应有2条路由，实际 %d", len(alphaRoutes))
	}

	betaRoutes := rt.ListByApp("app-beta")
	if len(betaRoutes) != 1 {
		t.Fatalf("app-beta 应有1条路由，实际 %d", len(betaRoutes))
	}

	if rt.CountByApp("app-alpha") != 2 {
		t.Fatalf("CountByApp(app-alpha) 应为2，实际 %d", rt.CountByApp("app-alpha"))
	}
}

func TestRouteStats(t *testing.T) {
	rt := NewRouteTable(nil, false)

	rt.Bind("user1", "app-alpha", "upstream-a")
	rt.Bind("user2", "app-alpha", "upstream-a")
	rt.Bind("user3", "app-beta", "upstream-b")
	rt.Bind("user1", "app-beta", "upstream-b")

	stats := rt.Stats()
	if stats.TotalRoutes != 4 {
		t.Fatalf("TotalRoutes 期望4，实际 %d", stats.TotalRoutes)
	}
	if stats.TotalUsers != 3 {
		t.Fatalf("TotalUsers 期望3，实际 %d", stats.TotalUsers)
	}
	if stats.TotalApps != 2 {
		t.Fatalf("TotalApps 期望2，实际 %d", stats.TotalApps)
	}
	if stats.ByUpstream["upstream-a"] != 2 {
		t.Fatalf("ByUpstream[upstream-a] 期望2，实际 %d", stats.ByUpstream["upstream-a"])
	}
	if stats.ByApp["app-alpha"] != 2 {
		t.Fatalf("ByApp[app-alpha] 期望2，实际 %d", stats.ByApp["app-alpha"])
	}
}

// ============================================================
// 上游池测试
// ============================================================

func TestUpstreamPoolSelect(t *testing.T) {
	cfg := &Config{
		StaticUpstreams:       []StaticUpstreamConfig{
			{ID: "up-a", Address: "127.0.0.1", Port: 18790},
			{ID: "up-b", Address: "127.0.0.1", Port: 18791},
		},
		HeartbeatIntervalSec:  10,
		HeartbeatTimeoutCount: 3,
	}
	pool := NewUpstreamPool(cfg, nil)

	id := pool.SelectUpstream("least-users")
	if id == "" {
		t.Fatal("应该选出一个上游")
	}
	id1 := pool.SelectUpstream("round-robin")
	id2 := pool.SelectUpstream("round-robin")
	if id1 == "" || id2 == "" {
		t.Fatal("round-robin 应选出上游")
	}
}

func TestUpstreamPoolRegisterDeregister(t *testing.T) {
	cfg := &Config{HeartbeatIntervalSec: 10, HeartbeatTimeoutCount: 3}
	pool := NewUpstreamPool(cfg, nil)

	pool.Register("test-1", "10.0.0.1", 18790, map[string]string{"env": "test"})
	found := false
	for _, up := range pool.ListUpstreams() {
		if up.ID == "test-1" { found = true }
	}
	if !found {
		t.Fatal("注册后应能查到")
	}

	_, err := pool.Heartbeat("test-1", map[string]interface{}{"cpu": 50.0})
	if err != nil {
		t.Fatalf("心跳失败: %v", err)
	}

	pool.Deregister("test-1")
	for _, up := range pool.ListUpstreams() {
		if up.ID == "test-1" { t.Fatal("注销后不应存在") }
	}
}

func TestUpstreamPoolGetAnyHealthy(t *testing.T) {
	cfg := &Config{
		StaticUpstreams:       []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		HeartbeatIntervalSec:  10,
		HeartbeatTimeoutCount: 3,
	}
	pool := NewUpstreamPool(cfg, nil)
	proxy, id := pool.GetAnyHealthyProxy()
	if proxy == nil || id == "" {
		t.Fatal("应返回健康代理")
	}
}


// ============================================================

var _ = xml.Unmarshal
var _ = http.StatusOK
var _ = url.QueryEscape

// ============================================================
// v3.9 测试: UserInfoCache + RoutePolicyEngine + Management API
// ============================================================

// mockUserProvider 测试用的 UserInfoProvider
type mockUserProvider struct {
	users map[string]*UserInfo
	calls int
	mu    sync.Mutex
}

func (m *mockUserProvider) FetchUserInfo(senderID string) (*UserInfo, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if info, ok := m.users[senderID]; ok {
		return info, nil
	}
	return nil, nil
}

func (m *mockUserProvider) NeedsCredentials() []string {
	return []string{"mock_key"}
}

func (m *mockUserProvider) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// TestUserInfoCache_GetOrFetch 测试用户信息缓存基本功能
func TestUserInfoCache_GetOrFetch(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
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
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_email ON user_info_cache(email)`)

	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"user-001": {Name: "张三", Email: "zhangsan@example.com", Department: "安全研究院"},
			"user-002": {Name: "李四", Email: "lisi@example.com", Department: "产品中心"},
		},
	}

	cache := NewUserInfoCache(db, provider, 1*time.Hour)

	// 第一次获取 — 应调 API
	info, err := cache.GetOrFetch("user-001")
	if err != nil {
		t.Fatalf("GetOrFetch failed: %v", err)
	}
	if info == nil || info.Name != "张三" || info.Email != "zhangsan@example.com" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if provider.getCalls() != 1 {
		t.Fatalf("expected 1 API call, got %d", provider.getCalls())
	}

	// 第二次获取 — 应走内存缓存
	info2, err := cache.GetOrFetch("user-001")
	if err != nil || info2 == nil || info2.Name != "张三" {
		t.Fatalf("second fetch failed: %v, %+v", err, info2)
	}
	if provider.getCalls() != 1 {
		t.Fatalf("expected still 1 API call (cached), got %d", provider.getCalls())
	}

	// 获取不存在的用户
	info3, err := cache.GetOrFetch("user-999")
	if err != nil {
		t.Fatalf("GetOrFetch unknown user failed: %v", err)
	}
	if info3 != nil {
		t.Fatalf("expected nil for unknown user, got %+v", info3)
	}

	// 空 sender_id
	info4, err := cache.GetOrFetch("")
	if err != nil || info4 != nil {
		t.Fatalf("empty sender should return nil,nil: %v, %+v", err, info4)
	}
}

// TestUserInfoCache_ListAll 测试列出所有用户
func TestUserInfoCache_ListAll(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
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

	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"u1": {Name: "A", Email: "a@sec.com", Department: "安全"},
			"u2": {Name: "B", Email: "b@dev.com", Department: "开发"},
			"u3": {Name: "C", Email: "c@sec.com", Department: "安全"},
		},
	}
	cache := NewUserInfoCache(db, provider, 1*time.Hour)

	// Fetch all users
	cache.GetOrFetch("u1")
	cache.GetOrFetch("u2")
	cache.GetOrFetch("u3")

	// List all
	all := cache.ListAll("", "")
	if len(all) != 3 {
		t.Fatalf("expected 3 users, got %d", len(all))
	}

	// Filter by department
	secUsers := cache.ListAll("安全", "")
	if len(secUsers) != 2 {
		t.Fatalf("expected 2 security users, got %d", len(secUsers))
	}

	// Filter by email
	emailUsers := cache.ListAll("", "sec.com")
	if len(emailUsers) != 2 {
		t.Fatalf("expected 2 sec.com users, got %d", len(emailUsers))
	}
}

// TestUserInfoCache_Refresh 测试强制刷新
func TestUserInfoCache_Refresh(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS user_info_cache (
		sender_id TEXT PRIMARY KEY, name TEXT, email TEXT, department TEXT, avatar TEXT, mobile TEXT DEFAULT '',
		fetched_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`)

	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"u1": {Name: "Original", Email: "orig@test.com", Department: "Dept1"},
		},
	}
	cache := NewUserInfoCache(db, provider, 1*time.Hour)
	cache.GetOrFetch("u1")

	// Change provider data
	provider.mu.Lock()
	provider.users["u1"] = &UserInfo{Name: "Updated", Email: "updated@test.com", Department: "Dept2"}
	provider.mu.Unlock()

	// Normal fetch should still return cached
	info, _ := cache.GetOrFetch("u1")
	if info.Name != "Original" {
		t.Fatalf("expected cached name, got %s", info.Name)
	}

	// Force refresh
	refreshed, err := cache.Refresh("u1")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if refreshed.Name != "Updated" || refreshed.Email != "updated@test.com" {
		t.Fatalf("refresh didn't update: %+v", refreshed)
	}
}

// TestUserInfoCache_NilProvider 测试无 provider 的降级
func TestUserInfoCache_NilProvider(t *testing.T) {
	cache := NewUserInfoCache(nil, nil, 1*time.Hour)
	info, err := cache.GetOrFetch("user-001")
	if err != nil || info != nil {
		t.Fatalf("nil provider should return nil,nil: %v, %+v", err, info)
	}
}

// TestRoutePolicyEngine_Match 测试策略匹配
func TestRoutePolicyEngine_Match(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Email: "vip@example.com"}, UpstreamID: "upstream-vip"},
		{Match: RoutePolicyMatch{Department: "安全研究院"}, UpstreamID: "upstream-security"},
		{Match: RoutePolicyMatch{EmailSuffix: "@dev.example.com"}, UpstreamID: "upstream-dev"},
		{Match: RoutePolicyMatch{AppID: "bot-alpha", Department: "产品中心"}, UpstreamID: "upstream-product"},
		{Match: RoutePolicyMatch{AppID: "bot-public"}, UpstreamID: "upstream-public"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: "upstream-default"},
	}
	engine := NewRoutePolicyEngine(policies)

	tests := []struct {
		name       string
		info       *UserInfo
		appID      string
		wantUID    string
		wantMatch  bool
	}{
		{
			name:      "exact email match",
			info:      &UserInfo{Email: "vip@example.com"},
			wantUID:   "upstream-vip",
			wantMatch: true,
		},
		{
			name:      "department match",
			info:      &UserInfo{Email: "someone@test.com", Department: "安全研究院"},
			wantUID:   "upstream-security",
			wantMatch: true,
		},
		{
			name:      "email suffix match",
			info:      &UserInfo{Email: "alice@dev.example.com"},
			wantUID:   "upstream-dev",
			wantMatch: true,
		},
		{
			name:      "app_id + department combo",
			info:      &UserInfo{Email: "bob@other.com", Department: "产品中心"},
			appID:     "bot-alpha",
			wantUID:   "upstream-product",
			wantMatch: true,
		},
		{
			name:      "app_id + department combo - wrong app",
			info:      &UserInfo{Email: "bob@other.com", Department: "产品中心"},
			appID:     "bot-wrong",
			wantUID:   "upstream-default",
			wantMatch: true, // falls through to default
		},
		{
			name:      "app_id only match",
			info:      &UserInfo{Email: "anyone@test.com", Department: "任意部门"},
			appID:     "bot-public",
			wantUID:   "upstream-public",
			wantMatch: true,
		},
		{
			name:      "default match",
			info:      &UserInfo{Email: "nobody@none.com", Department: "未知"},
			wantUID:   "upstream-default",
			wantMatch: true,
		},
		{
			name:      "nil info",
			info:      nil,
			wantUID:   "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, matched := engine.Match(tt.info, tt.appID)
			if matched != tt.wantMatch {
				t.Errorf("Match() matched = %v, want %v", matched, tt.wantMatch)
			}
			if uid != tt.wantUID {
				t.Errorf("Match() uid = %q, want %q", uid, tt.wantUID)
			}
		})
	}
}

// TestRoutePolicyEngine_MatchPriority 测试策略优先级（从上到下）
func TestRoutePolicyEngine_MatchPriority(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Email: "special@example.com"}, UpstreamID: "first"},
		{Match: RoutePolicyMatch{Department: "安全"}, UpstreamID: "second"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: "last"},
	}
	engine := NewRoutePolicyEngine(policies)

	// User matches both email and department — should match first rule
	info := &UserInfo{Email: "special@example.com", Department: "安全"}
	uid, matched := engine.Match(info, "")
	if !matched || uid != "first" {
		t.Errorf("expected first match, got %q matched=%v", uid, matched)
	}
}

// TestRoutePolicyEngine_AppIDCrossBot 测试同一用户访问不同 Bot 命中不同策略
func TestRoutePolicyEngine_AppIDCrossBot(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{AppID: "bot-alpha"}, UpstreamID: "upstream-alpha"},
		{Match: RoutePolicyMatch{AppID: "bot-beta"}, UpstreamID: "upstream-beta"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: "upstream-default"},
	}
	engine := NewRoutePolicyEngine(policies)

	info := &UserInfo{Email: "user@example.com", Department: "通用"}

	uid1, _ := engine.Match(info, "bot-alpha")
	uid2, _ := engine.Match(info, "bot-beta")
	uid3, _ := engine.Match(info, "bot-gamma")

	if uid1 != "upstream-alpha" {
		t.Errorf("bot-alpha: got %q, want upstream-alpha", uid1)
	}
	if uid2 != "upstream-beta" {
		t.Errorf("bot-beta: got %q, want upstream-beta", uid2)
	}
	if uid3 != "upstream-default" {
		t.Errorf("bot-gamma: got %q, want upstream-default", uid3)
	}
}

// TestRoutePolicyEngine_TestMatch 测试 TestMatch
func TestRoutePolicyEngine_TestMatch(t *testing.T) {
	policies := []RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "安全"}, UpstreamID: "sec"},
		{Match: RoutePolicyMatch{Default: true}, UpstreamID: "def"},
	}
	engine := NewRoutePolicyEngine(policies)

	idx, policy, matched := engine.TestMatch(&UserInfo{Department: "安全"}, "")
	if !matched || idx != 0 || policy.UpstreamID != "sec" {
		t.Errorf("TestMatch failed: idx=%d, matched=%v, policy=%+v", idx, matched, policy)
	}

	idx2, _, matched2 := engine.TestMatch(&UserInfo{Department: "其他"}, "")
	if !matched2 || idx2 != 1 {
		t.Errorf("TestMatch default failed: idx=%d, matched=%v", idx2, matched2)
	}
}

// TestRoutePolicyEngine_Empty 测试空策略
func TestRoutePolicyEngine_Empty(t *testing.T) {
	engine := NewRoutePolicyEngine(nil)
	uid, matched := engine.Match(&UserInfo{Email: "test@test.com"}, "")
	if matched || uid != "" {
		t.Errorf("empty engine should not match, got %q matched=%v", uid, matched)
	}
}

// TestCreateUserInfoProvider 测试 provider 工厂函数
func TestCreateUserInfoProvider(t *testing.T) {
	// Lanxin with credentials
	cfg := &Config{Channel: "lanxin", LanxinAppID: "app1", LanxinAppSecret: "secret1", LanxinUpstream: "https://example.com"}
	p := createUserInfoProvider(cfg)
	if p == nil {
		t.Fatal("expected lanxin provider")
	}
	if _, ok := p.(*LanxinUserProvider); !ok {
		t.Fatalf("expected *LanxinUserProvider, got %T", p)
	}

	// Lanxin without credentials
	cfg2 := &Config{Channel: "lanxin"}
	if createUserInfoProvider(cfg2) != nil {
		t.Fatal("expected nil provider without credentials")
	}

	// Feishu
	cfg3 := &Config{Channel: "feishu", FeishuAppID: "cli_xxx", FeishuAppSecret: "sec"}
	p3 := createUserInfoProvider(cfg3)
	if _, ok := p3.(*FeishuUserProvider); !ok {
		t.Fatalf("expected *FeishuUserProvider, got %T", p3)
	}

	// DingTalk
	cfg4 := &Config{Channel: "dingtalk", DingtalkClientID: "cid", DingtalkClientSecret: "csec"}
	p4 := createUserInfoProvider(cfg4)
	if _, ok := p4.(*DingTalkUserProvider); !ok {
		t.Fatalf("expected *DingTalkUserProvider, got %T", p4)
	}

	// WeCom
	cfg5 := &Config{Channel: "wecom", WecomCorpId: "wk123", WecomCorpSecret: "wsec"}
	p5 := createUserInfoProvider(cfg5)
	if _, ok := p5.(*WeComUserProvider); !ok {
		t.Fatalf("expected *WeComUserProvider, got %T", p5)
	}

	// Generic
	cfg6 := &Config{Channel: "generic"}
	if createUserInfoProvider(cfg6) != nil {
		t.Fatal("generic should return nil provider")
	}
}

// TestUserInfoManagementAPI 测试 v3.9 Management API 端点
func TestUserInfoManagementAPI(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Create tables
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, direction TEXT, sender_id TEXT,
		action TEXT, reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT DEFAULT '', app_id TEXT DEFAULT '', trace_id TEXT DEFAULT ''
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (
		id TEXT PRIMARY KEY, address TEXT, port INTEGER, healthy INTEGER DEFAULT 1,
		registered_at TEXT, last_heartbeat TEXT, tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
		sender_id TEXT NOT NULL, app_id TEXT NOT NULL DEFAULT '', upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '', display_name TEXT DEFAULT '', email TEXT DEFAULT '',
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (sender_id, app_id)
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_info_cache (
		sender_id TEXT PRIMARY KEY, name TEXT DEFAULT '', email TEXT DEFAULT '',
		department TEXT DEFAULT '', avatar TEXT DEFAULT '', mobile TEXT DEFAULT '',
		fetched_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`)

	cfg := &Config{
		InboundListen: ":0", OutboundListen: ":0", ManagementListen: ":0",
		OpenClawUpstream: "http://localhost:18790", LanxinUpstream: "https://example.com",
		DBPath: ":memory:", RouteDefaultPolicy: "least-users",
		StaticUpstreams: []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		RoutePolicies: []RoutePolicyConfig{
			{Match: RoutePolicyMatch{Department: "安全"}, UpstreamID: "up-sec"},
			{Match: RoutePolicyMatch{Default: true}, UpstreamID: "up-default"},
		},
	}
	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"s1": {Name: "Alice", Email: "alice@sec.com", Department: "安全"},
			"s2": {Name: "Bob", Email: "bob@dev.com", Department: "开发"},
		},
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)
	userCache := NewUserInfoCache(db, provider, 1*time.Hour)
	policyEng := NewRoutePolicyEngine(cfg.RoutePolicies)

	gp := NewGenericPlugin("X-Sender-Id", "content")
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, nil, nil, userCache, policyEng)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, gp, nil, nil, userCache, policyEng, nil, nil, nil, nil, nil)

	// Pre-fetch users
	userCache.GetOrFetch("s1")
	userCache.GetOrFetch("s2")

	// Test GET /api/v1/users
	t.Run("list_users", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		total := int(resp["total"].(float64))
		if total != 2 {
			t.Fatalf("expected 2 users, got %d", total)
		}
	})

	// Test GET /api/v1/users?department=安全
	t.Run("list_users_by_department", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users?department="+url.QueryEscape("安全"), nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		total := int(resp["total"].(float64))
		if total != 1 {
			t.Fatalf("expected 1 security user, got %d", total)
		}
	})

	// Test GET /api/v1/users/:sender_id
	t.Run("get_user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users/s1", nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var info UserInfo
		json.Unmarshal(w.Body.Bytes(), &info)
		if info.Name != "Alice" || info.Email != "alice@sec.com" {
			t.Fatalf("unexpected user info: %+v", info)
		}
	})

	// Test GET /api/v1/users/:sender_id (not found)
	t.Run("get_user_not_found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users/unknown", nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	// Test POST /api/v1/users/:sender_id/refresh
	t.Run("refresh_user", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/users/s1/refresh", nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test GET /api/v1/route-policies
	t.Run("list_policies", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/route-policies", nil)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		total := int(resp["total"].(float64))
		if total != 2 {
			t.Fatalf("expected 2 policies, got %d", total)
		}
	})

	// Test POST /api/v1/route-policies/test
	t.Run("test_policy_match", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"sender_id":  "s1",
			"email":      "alice@sec.com",
			"department": "安全",
		})
		req := httptest.NewRequest("POST", "/api/v1/route-policies/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if !resp["matched"].(bool) {
			t.Fatal("expected matched=true")
		}
		if resp["upstream_id"].(string) != "up-sec" {
			t.Fatalf("expected up-sec, got %s", resp["upstream_id"])
		}
	})

	// Test policy test with fallback to default
	t.Run("test_policy_match_default", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"department": "未知部门",
		})
		req := httptest.NewRequest("POST", "/api/v1/route-policies/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["upstream_id"].(string) != "up-default" {
			t.Fatalf("expected up-default, got %s", resp["upstream_id"])
		}
	})
}

// TestRouteTable_UpdateUserInfo 测试 UpdateUserInfo
func TestRouteTable_UpdateUserInfo(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
		sender_id TEXT NOT NULL, app_id TEXT NOT NULL DEFAULT '', upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '', display_name TEXT DEFAULT '', email TEXT DEFAULT '',
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (sender_id, app_id)
	)`)

	rt := NewRouteTable(db, true)

	// Bind a user
	rt.Bind("user-001", "bot-alpha", "up-1")

	// Update info
	rt.UpdateUserInfo("user-001", "张三", "zhangsan@example.com", "安全部")

	// Verify via DB query
	var name, email, dept string
	err = db.QueryRow(`SELECT display_name, email, department FROM user_routes WHERE sender_id='user-001'`).Scan(&name, &email, &dept)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if name != "张三" || email != "zhangsan@example.com" || dept != "安全部" {
		t.Fatalf("unexpected: name=%q email=%q dept=%q", name, email, dept)
	}
}

// TestRoutePolicyConfig_YAML 测试策略配置 YAML 解析
func TestRoutePolicyConfig_YAML(t *testing.T) {
	yamlData := `
route_policies:
  - match:
      department: "安全研究院"
    upstream_id: "openclaw-security"
  - match:
      email_suffix: "@security.qianxin.com"
    upstream_id: "openclaw-security"
  - match:
      email: "zhangzhuo@qianxin.com"
    upstream_id: "openclaw-vip"
  - match:
      app_id: "alpha-3588352-9076736"
      department: "产品中心"
    upstream_id: "openclaw-product"
  - match:
      app_id: "gamma-3588352-7654321"
    upstream_id: "openclaw-public"
  - match:
      default: true
    upstream_id: ""
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}
	if len(cfg.RoutePolicies) != 6 {
		t.Fatalf("expected 6 policies, got %d", len(cfg.RoutePolicies))
	}
	if cfg.RoutePolicies[0].Match.Department != "安全研究院" {
		t.Fatalf("first policy department mismatch: %+v", cfg.RoutePolicies[0])
	}
	if cfg.RoutePolicies[3].Match.AppID != "alpha-3588352-9076736" {
		t.Fatalf("fourth policy app_id mismatch: %+v", cfg.RoutePolicies[3])
	}
	if cfg.RoutePolicies[4].Match.AppID != "gamma-3588352-7654321" {
		t.Fatalf("fifth policy app_id mismatch: %+v", cfg.RoutePolicies[4])
	}
	if !cfg.RoutePolicies[5].Match.Default {
		t.Fatalf("last policy should be default")
	}
}

// TestUserInfoManagementAPI_NilCache 测试无缓存时 API 的降级
func TestUserInfoManagementAPI_NilCache(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, direction TEXT, sender_id TEXT,
		action TEXT, reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT DEFAULT '', app_id TEXT DEFAULT '', trace_id TEXT DEFAULT ''
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (
		id TEXT PRIMARY KEY, address TEXT, port INTEGER, healthy INTEGER DEFAULT 1,
		registered_at TEXT, last_heartbeat TEXT, tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
		sender_id TEXT NOT NULL, app_id TEXT NOT NULL DEFAULT '', upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '', display_name TEXT DEFAULT '', email TEXT DEFAULT '',
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY (sender_id, app_id)
	)`)

	cfg := &Config{
		InboundListen: ":0", OutboundListen: ":0", ManagementListen: ":0",
		OpenClawUpstream: "http://localhost:18790", LanxinUpstream: "https://example.com",
		DBPath: ":memory:", RouteDefaultPolicy: "least-users",
		StaticUpstreams: []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)

	gp := NewGenericPlugin("X-Sender-Id", "content")
	// nil userCache and nil policyEng — should degrade gracefully
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, gp, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// GET /api/v1/users should return empty with message
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] == nil {
		t.Fatal("expected message about not configured")
	}

	// GET /api/v1/route-policies should return empty
	req2 := httptest.NewRequest("GET", "/api/v1/route-policies", nil)
	w2 := httptest.NewRecorder()
	api.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	// POST /api/v1/users/refresh-all should return error
	req3 := httptest.NewRequest("POST", "/api/v1/users/refresh-all", nil)
	w3 := httptest.NewRecorder()
	api.ServeHTTP(w3, req3)
	if w3.Code != 400 {
		t.Fatalf("expected 400, got %d", w3.Code)
	}
}

// TestNeedsCredentials 测试各 provider 的 NeedsCredentials
func TestNeedsCredentials(t *testing.T) {
	lp := NewLanxinUserProvider("a", "b", "https://example.com")
	if len(lp.NeedsCredentials()) != 2 || lp.NeedsCredentials()[0] != "lanxin_app_id" {
		t.Errorf("lanxin needs: %v", lp.NeedsCredentials())
	}
	fp := NewFeishuUserProvider("a", "b")
	if len(fp.NeedsCredentials()) != 2 {
		t.Errorf("feishu needs: %v", fp.NeedsCredentials())
	}
	dp := NewDingTalkUserProvider("a", "b")
	if len(dp.NeedsCredentials()) != 2 {
		t.Errorf("dingtalk needs: %v", dp.NeedsCredentials())
	}
	wp := NewWeComUserProvider("a", "b")
	if len(wp.NeedsCredentials()) != 2 {
		t.Errorf("wecom needs: %v", wp.NeedsCredentials())
	}
}

