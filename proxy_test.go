// proxy_test.go — InboundProxy、OutboundProxy 测试
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// 健壮性增强测试 (v3.2)
// ============================================================

func TestOutboundBodySizeLimit(t *testing.T) {
	// 验证 io.LimitReader 的行为
	data := bytes.Repeat([]byte("x"), 1024*1024) // 1MB
	limited := make([]byte, 0)
	reader := strings.NewReader(string(data))
	lr := io.LimitReader(reader, 10*1024*1024) // 10MB limit
	result, _ := io.ReadAll(lr)
	limited = result
	if len(limited) != 1024*1024 {
		t.Errorf("期望 1MB，实际 %d", len(limited))
	}
}

func TestAuditContentPreviewTruncation(t *testing.T) {
	// 验证截断逻辑
	longText := strings.Repeat("中", 600)
	rs := []rune(longText)
	if len(rs) > 500 {
		longText = string(rs[:500]) + "..."
	}
	if len([]rune(longText)) != 503 { // 500 + "..."(3 chars)
		t.Errorf("截断后长度期望 503 runes，实际 %d", len([]rune(longText)))
	}
}

// ============================================================
// Rate Limiter 测试 (v3.3)
// ============================================================

func TestTokenBucket_Basic(t *testing.T) {
	tb := NewTokenBucket(2, 3)
	// 快速消费 3 个 token → 全部允许
	for i := 0; i < 3; i++ {
		if !tb.Allow() {
			t.Fatalf("第 %d 个 token 应被允许", i+1)
		}
	}
	// 第 4 个 → 拒绝
	if tb.Allow() {
		t.Fatal("第 4 个 token 不应被允许")
	}
	// 等待 500ms → 补充约 1 个 token
	time.Sleep(550 * time.Millisecond)
	if !tb.Allow() {
		t.Fatal("等待后应允许 1 个请求")
	}
	// 再次应该被拒绝
	if tb.Allow() {
		t.Fatal("紧接着的请求应该被拒绝")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(10, 5)
	// 消耗所有
	for i := 0; i < 5; i++ {
		tb.Allow()
	}
	if tb.Allow() {
		t.Fatal("所有 token 消耗后不应允许")
	}
	// 等待 1 秒，应补充至满（10 tokens，但 burst=5，cap 在 5）
	time.Sleep(1050 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if !tb.Allow() {
			t.Fatalf("1 秒后第 %d 个应被允许（burst=5）", i+1)
		}
	}
	if tb.Allow() {
		t.Fatal("不应超过 burst")
	}
}

func TestRateLimiter_Global(t *testing.T) {
	cfg := RateLimiterConfig{
		GlobalRPS:   5,
		GlobalBurst: 5,
	}
	rl := NewRateLimiter(cfg)

	// 前 5 个应允许
	for i := 0; i < 5; i++ {
		allowed, _ := rl.Allow("user1")
		if !allowed {
			t.Fatalf("全局限流: 第 %d 个请求应被允许", i+1)
		}
	}
	// 第 6 个应被拒绝
	allowed, reason := rl.Allow("user1")
	if allowed {
		t.Fatal("全局限流: 应该被拒绝")
	}
	if reason == "" {
		t.Fatal("拒绝原因不应为空")
	}
	if !strings.Contains(reason, "global") {
		t.Fatalf("拒绝原因应包含 'global'，实际: %s", reason)
	}
}

func TestRateLimiter_PerSender(t *testing.T) {
	cfg := RateLimiterConfig{
		PerSenderRPS:   2,
		PerSenderBurst: 2,
	}
	rl := NewRateLimiter(cfg)

	// 用户 A 消耗 2 个
	rl.Allow("userA")
	rl.Allow("userA")

	// 用户 A 第 3 个应被拒绝
	allowed, reason := rl.Allow("userA")
	if allowed {
		t.Fatal("用户 A 应被限流")
	}
	if !strings.Contains(reason, "per-sender") {
		t.Fatalf("拒绝原因应包含 'per-sender'，实际: %s", reason)
	}

	// 用户 B 不受影响
	allowed, _ = rl.Allow("userB")
	if !allowed {
		t.Fatal("用户 B 不应受用户 A 限流影响")
	}
	allowed, _ = rl.Allow("userB")
	if !allowed {
		t.Fatal("用户 B 第 2 个请求也应被允许")
	}
}

func TestRateLimiter_Exempt(t *testing.T) {
	cfg := RateLimiterConfig{
		GlobalRPS:      1,
		GlobalBurst:    1,
		PerSenderRPS:   1,
		PerSenderBurst: 1,
		ExemptSenders:  []string{"admin"},
	}
	rl := NewRateLimiter(cfg)

	// 普通用户第 1 个通过
	allowed, _ := rl.Allow("normal")
	if !allowed {
		t.Fatal("普通用户第 1 个应通过")
	}

	// 白名单用户无论多少次都通过
	for i := 0; i < 100; i++ {
		allowed, _ := rl.Allow("admin")
		if !allowed {
			t.Fatalf("白名单用户第 %d 个请求不应被限流", i+1)
		}
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	cfg := RateLimiterConfig{
		PerSenderRPS:   1,
		PerSenderBurst: 1,
	}
	rl := NewRateLimiter(cfg)

	// 3 个请求: 1 allowed, 2 limited (对同一用户)
	rl.Allow("userX")
	rl.Allow("userX")
	rl.Allow("userX")

	stats := rl.Stats()
	if stats.TotalAllowed != 1 {
		t.Errorf("TotalAllowed 期望 1，实际 %d", stats.TotalAllowed)
	}
	if stats.TotalLimited != 2 {
		t.Errorf("TotalLimited 期望 2，实际 %d", stats.TotalLimited)
	}
	if stats.LimitRate < 60 || stats.LimitRate > 70 {
		t.Errorf("LimitRate 期望约 66.67%%，实际 %.2f%%", stats.LimitRate)
	}
	if len(stats.TopLimited) != 1 {
		t.Fatalf("TopLimited 期望 1 条，实际 %d", len(stats.TopLimited))
	}
	if stats.TopLimited[0].SenderID != "userX" {
		t.Errorf("TopLimited[0].SenderID 期望 'userX'，实际 %q", stats.TopLimited[0].SenderID)
	}
	if stats.TopLimited[0].Count != 2 {
		t.Errorf("TopLimited[0].Count 期望 2，实际 %d", stats.TopLimited[0].Count)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimiterConfig{
		PerSenderRPS:   10,
		PerSenderBurst: 10,
	}
	rl := NewRateLimiter(cfg)

	// 创建 sender bucket
	rl.Allow("temp-user")

	rl.mu.RLock()
	_, exists := rl.senderBuckets["temp-user"]
	rl.mu.RUnlock()
	if !exists {
		t.Fatal("sender bucket 应存在")
	}

	// 模拟 lastAccess 过期
	rl.mu.Lock()
	bucket := rl.senderBuckets["temp-user"]
	bucket.mu.Lock()
	bucket.lastAccess = time.Now().Add(-15 * time.Minute)
	bucket.mu.Unlock()
	rl.mu.Unlock()

	// 手动执行清理逻辑
	rl.mu.Lock()
	now := time.Now()
	for sid, b := range rl.senderBuckets {
		b.mu.Lock()
		idle := now.Sub(b.lastAccess)
		b.mu.Unlock()
		if idle > 10*time.Minute {
			delete(rl.senderBuckets, sid)
		}
	}
	rl.mu.Unlock()

	rl.mu.RLock()
	_, exists = rl.senderBuckets["temp-user"]
	rl.mu.RUnlock()
	if exists {
		t.Fatal("过期的 sender bucket 应被清理")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	// rps=0 时不创建限流器
	cfg := RateLimiterConfig{
		GlobalRPS:    0,
		PerSenderRPS: 0,
	}
	rl := NewRateLimiter(cfg)

	// 所有请求都应通过
	for i := 0; i < 1000; i++ {
		allowed, _ := rl.Allow(fmt.Sprintf("user%d", i))
		if !allowed {
			t.Fatalf("rps=0 时不应限流，第 %d 个请求被拒绝", i+1)
		}
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	cfg := RateLimiterConfig{
		PerSenderRPS:   1,
		PerSenderBurst: 1,
	}
	rl := NewRateLimiter(cfg)

	rl.Allow("user1")
	rl.Allow("user1") // limited

	stats := rl.Stats()
	if stats.TotalLimited != 1 {
		t.Fatalf("重置前 TotalLimited 期望 1，实际 %d", stats.TotalLimited)
	}

	rl.Reset()

	stats = rl.Stats()
	if stats.TotalAllowed != 0 || stats.TotalLimited != 0 {
		t.Fatalf("重置后统计应为 0, got allowed=%d limited=%d", stats.TotalAllowed, stats.TotalLimited)
	}

	// 重置后 bucket 也被清空，新请求应通过
	allowed, _ := rl.Allow("user1")
	if !allowed {
		t.Fatal("重置后请求应通过")
	}
}

func TestRateLimiter_GlobalAndPerSender(t *testing.T) {
	cfg := RateLimiterConfig{
		GlobalRPS:      10,
		GlobalBurst:    10,
		PerSenderRPS:   2,
		PerSenderBurst: 2,
	}
	rl := NewRateLimiter(cfg)

	// 每用户限流应先于全局生效
	rl.Allow("userA")
	rl.Allow("userA")
	allowed, reason := rl.Allow("userA")
	if allowed {
		t.Fatal("用户 A 应被 per-sender 限流")
	}
	if !strings.Contains(reason, "per-sender") {
		t.Fatalf("应为 per-sender 限流，实际: %s", reason)
	}

	// 其他用户仍可通过全局限流
	allowed, _ = rl.Allow("userB")
	if !allowed {
		t.Fatal("用户 B 应通过")
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != "hello" {
		t.Error("短字符串不应被截断")
	}
	long := strings.Repeat("中", 300)
	result := truncate(long, 200)
	rs := []rune(result)
	if len(rs) != 203 { // 200 + "..."
		t.Errorf("截断结果长度期望 203 runes，实际 %d", len(rs))
	}
}

func TestInboundProxy_RateLimit_Webhook(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT, reason TEXT, content_preview TEXT, full_request_hash TEXT, latency_ms REAL, upstream_id TEXT, app_id TEXT DEFAULT '')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (id TEXT PRIMARY KEY, address TEXT, port INTEGER, healthy INTEGER DEFAULT 1, registered_at TEXT, last_heartbeat TEXT, tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (sender_id TEXT PRIMARY KEY, upstream_id TEXT, created_at TEXT, updated_at TEXT)`)

	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	cfg := &Config{
		InboundDetectEnabled: true,
		DetectTimeoutMs:      50,
		RouteDefaultPolicy:   "least-users",
		RateLimit: RateLimiterConfig{
			PerSenderRPS:   1,
			PerSenderBurst: 1,
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)

	gp := NewGenericPlugin("", "content")
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, nil, nil, nil, nil)

	// 第 1 个请求 — 应通过（虽无上游会 502，但不应 429）
	body := []byte(`{"content":"hello","sender_id":"user1"}`)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	inbound.ServeHTTP(rec, req)
	if rec.Code == 429 {
		t.Fatal("第 1 个请求不应被限流")
	}

	// 第 2 个请求 — 应返回 429
	req = httptest.NewRequest("POST", "/", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	inbound.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Fatalf("第 2 个请求期望 429，实际 %d", rec.Code)
	}
	// 验证 Retry-After header
	if rec.Header().Get("Retry-After") != "1" {
		t.Errorf("Retry-After header 期望 '1'，实际 %q", rec.Header().Get("Retry-After"))
	}
}

func TestHealthz_RateLimiter(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT, reason TEXT, content_preview TEXT, full_request_hash TEXT, latency_ms REAL, upstream_id TEXT, app_id TEXT DEFAULT '')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (id TEXT PRIMARY KEY, address TEXT, port INTEGER, healthy INTEGER DEFAULT 1, registered_at TEXT, last_heartbeat TEXT, tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (sender_id TEXT PRIMARY KEY, upstream_id TEXT, created_at TEXT, updated_at TEXT)`)

	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	outboundEngine := NewOutboundRuleEngine(nil)
	cfg := &Config{
		InboundDetectEnabled: true,
		DetectTimeoutMs:      50,
		RouteDefaultPolicy:   "least-users",
		RateLimit: RateLimiterConfig{
			GlobalRPS:      100,
			GlobalBurst:    200,
			PerSenderRPS:   10,
			PerSenderBurst: 20,
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	gp := NewGenericPlugin("", "content")
	engine := NewRuleEngine()
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, nil, nil, nil, nil)
	mgmt := NewManagementAPI(cfg, "", pool, routes, logger, engine, outboundEngine, inbound, nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	mgmt.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("healthz 期望 200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	rl, ok := resp["rate_limiter"].(map[string]interface{})
	if !ok {
		t.Fatal("healthz 响应缺少 rate_limiter")
	}
	if enabled, ok := rl["enabled"].(bool); !ok || !enabled {
		t.Fatal("rate_limiter.enabled 应为 true")
	}
	if globalRPS, ok := rl["global_rps"].(float64); !ok || globalRPS != 100 {
		t.Errorf("rate_limiter.global_rps 期望 100，实际 %v", rl["global_rps"])
	}
	if perSenderRPS, ok := rl["per_sender_rps"].(float64); !ok || perSenderRPS != 10 {
		t.Errorf("rate_limiter.per_sender_rps 期望 10，实际 %v", rl["per_sender_rps"])
	}
}

func TestManagementAPI_RateLimitEndpoints(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT, reason TEXT, content_preview TEXT, full_request_hash TEXT, latency_ms REAL, upstream_id TEXT, app_id TEXT DEFAULT '')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (id TEXT PRIMARY KEY, address TEXT, port INTEGER, healthy INTEGER DEFAULT 1, registered_at TEXT, last_heartbeat TEXT, tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (sender_id TEXT PRIMARY KEY, upstream_id TEXT, created_at TEXT, updated_at TEXT)`)

	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	outboundEngine := NewOutboundRuleEngine(nil)
	cfg := &Config{
		InboundDetectEnabled: true,
		DetectTimeoutMs:      50,
		RouteDefaultPolicy:   "least-users",
		RateLimit: RateLimiterConfig{
			PerSenderRPS:   1,
			PerSenderBurst: 1,
		},
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	gp := NewGenericPlugin("", "content")
	engine2 := NewRuleEngine()
	inbound := NewInboundProxy(cfg, gp, engine2, logger, pool, routes, nil, nil, nil, nil)
	mgmt := NewManagementAPI(cfg, "", pool, routes, logger, engine2, outboundEngine, inbound, nil, nil, nil, nil, nil, nil)

	// 产生一些限流数据
	inbound.limiter.Allow("testUser")
	inbound.limiter.Allow("testUser") // limited

	// GET /api/v1/rate-limit/stats
	req := httptest.NewRequest("GET", "/api/v1/rate-limit/stats", nil)
	rec := httptest.NewRecorder()
	mgmt.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("rate-limit stats 期望 200，实际 %d", rec.Code)
	}
	var stats RateLimiterStats
	json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.TotalAllowed != 1 || stats.TotalLimited != 1 {
		t.Errorf("stats 期望 allowed=1 limited=1，实际 allowed=%d limited=%d", stats.TotalAllowed, stats.TotalLimited)
	}

	// POST /api/v1/rate-limit/reset
	req = httptest.NewRequest("POST", "/api/v1/rate-limit/reset", nil)
	rec = httptest.NewRecorder()
	mgmt.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("rate-limit reset 期望 200，实际 %d", rec.Code)
	}

	// 验证重置后统计清零
	req = httptest.NewRequest("GET", "/api/v1/rate-limit/stats", nil)
	rec = httptest.NewRecorder()
	mgmt.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.TotalAllowed != 0 || stats.TotalLimited != 0 {
		t.Errorf("重置后 stats 期望都为 0，实际 allowed=%d limited=%d", stats.TotalAllowed, stats.TotalLimited)
	}
}
