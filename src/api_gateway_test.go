// api_gateway_test.go — API Gateway 测试
// lobster-guard v20.4
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// newTestGateway 创建测试用 API Gateway
func newTestGateway(t *testing.T) (*APIGateway, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cfg := APIGatewayConfig{
		Enabled:       true,
		JWTSecret:     "test-secret-key-for-gateway",
		JWTEnabled:    true,
		APIKeyEnabled: true,
		APIKeys:       []string{"key-abc-123", "key-xyz-789"},
	}
	gw := NewAPIGateway(db, cfg)
	return gw, db
}

// ============================================================
// 1. TestGatewayJWTGenerate — JWT 生成
// ============================================================
func TestGatewayJWTGenerate(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	claims := GWJWTClaims{
		TenantID: "tenant-001",
		Role:     "admin",
		Sub:      "user@example.com",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := gw.GenerateGWJWT(claims)
	if err != nil {
		t.Fatalf("GenerateGWJWT failed: %v", err)
	}

	// JWT 应该有 3 段
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	// Header 应该包含 HS256
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if !strings.Contains(string(headerBytes), "HS256") {
		t.Error("header should contain HS256")
	}

	// Payload 应该包含 tenant_id
	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !strings.Contains(string(payloadBytes), "tenant-001") {
		t.Error("payload should contain tenant_id")
	}

	t.Logf("Generated JWT: %s...%s", token[:20], token[len(token)-20:])
}

// ============================================================
// 2. TestGatewayJWTValidate — JWT 验证
// ============================================================
func TestGatewayJWTValidate(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	original := GWJWTClaims{
		TenantID: "tenant-002",
		Role:     "user",
		Sub:      "bob@example.com",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
		Iat:      time.Now().Unix(),
	}

	token, err := gw.GenerateGWJWT(original)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := gw.ValidateGWJWT(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.TenantID != "tenant-002" {
		t.Errorf("tenant_id = %q, want tenant-002", claims.TenantID)
	}
	if claims.Role != "user" {
		t.Errorf("role = %q, want user", claims.Role)
	}
	if claims.Sub != "bob@example.com" {
		t.Errorf("sub = %q, want bob@example.com", claims.Sub)
	}
}

// ============================================================
// 3. TestGatewayJWTExpired — 过期 JWT 拒绝
// ============================================================
func TestGatewayJWTExpired(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	claims := GWJWTClaims{
		TenantID: "tenant-003",
		Role:     "admin",
		Sub:      "expired-user",
		Exp:      time.Now().Add(-1 * time.Hour).Unix(), // 已过期
		Iat:      time.Now().Add(-2 * time.Hour).Unix(),
	}

	token, err := gw.GenerateGWJWT(claims)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = gw.ValidateGWJWT(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should contain 'expired', got: %v", err)
	}
}

// ============================================================
// 4. TestGatewayJWTInvalidSig — 签名错误拒绝
// ============================================================
func TestGatewayJWTInvalidSig(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	claims := GWJWTClaims{
		TenantID: "tenant-004",
		Role:     "admin",
		Sub:      "user4",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := gw.GenerateGWJWT(claims)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// 篡改 signature: 用不同的 key 重新签名
	parts := strings.SplitN(token, ".", 3)
	signingInput := parts[0] + "." + parts[1]
	wrongKey := []byte("wrong-secret-key")
	mac := hmac.New(sha256.New, wrongKey)
	mac.Write([]byte(signingInput))
	wrongSig := base64URLEncode(mac.Sum(nil))
	tamperedToken := signingInput + "." + wrongSig

	_, err = gw.ValidateGWJWT(tamperedToken)
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("error should mention signature, got: %v", err)
	}
}

// ============================================================
// 5. TestGatewayAPIKeyAuth — API Key 认证
// ============================================================
func TestGatewayAPIKeyAuth(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 通过 header
	r1 := httptest.NewRequest("GET", "/api/test", nil)
	r1.Header.Set("X-API-Key", "key-abc-123")
	claims, err := gw.AuthenticateRequest(r1)
	if err != nil {
		t.Fatalf("API key via header should succeed: %v", err)
	}
	if claims.TenantID != "apikey" {
		t.Errorf("tenant = %q, want apikey", claims.TenantID)
	}

	// 通过 query parameter
	r2 := httptest.NewRequest("GET", "/api/test?api_key=key-xyz-789", nil)
	claims2, err := gw.AuthenticateRequest(r2)
	if err != nil {
		t.Fatalf("API key via query should succeed: %v", err)
	}
	if claims2.TenantID != "apikey" {
		t.Errorf("tenant = %q, want apikey", claims2.TenantID)
	}

	// 无效的 API key
	r3 := httptest.NewRequest("GET", "/api/test", nil)
	r3.Header.Set("X-API-Key", "invalid-key")
	_, err = gw.AuthenticateRequest(r3)
	if err == nil {
		t.Fatal("invalid API key should fail")
	}
}

// ============================================================
// 6. TestGatewayNoAuth — 无认证路由
// ============================================================
func TestGatewayNoAuth(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	route := &GatewayRoute{
		Auth: "none",
	}

	r := httptest.NewRequest("GET", "/public/test", nil)
	claims, err := gw.AuthenticateForRoute(r, route)
	if err != nil {
		t.Fatalf("no auth should succeed: %v", err)
	}
	if claims.TenantID != "anonymous" {
		t.Errorf("tenant = %q, want anonymous", claims.TenantID)
	}
}

// ============================================================
// 7. TestGatewayRouteMatch — 路由匹配
// ============================================================
func TestGatewayRouteMatch(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 添加路由
	gw.AddRoute(GatewayRoute{
		ID: "r1", Name: "API Route", PathPattern: "/api/",
		UpstreamURL: "http://api:8080", Methods: []string{"GET", "POST"},
		Enabled: true, Priority: 10,
	})
	gw.AddRoute(GatewayRoute{
		ID: "r2", Name: "Web Route", PathPattern: "/web/",
		UpstreamURL: "http://web:3000", Methods: []string{"GET"},
		Enabled: true, Priority: 5,
	})

	// 匹配 /api/users GET
	route := gw.MatchRoute("/api/users", "GET")
	if route == nil {
		t.Fatal("should match /api/ route")
	}
	if route.ID != "r1" {
		t.Errorf("route id = %q, want r1", route.ID)
	}

	// 匹配 /web/index GET
	route2 := gw.MatchRoute("/web/index", "GET")
	if route2 == nil {
		t.Fatal("should match /web/ route")
	}
	if route2.ID != "r2" {
		t.Errorf("route id = %q, want r2", route2.ID)
	}

	// 不匹配 /web/ POST（方法不允许）
	route3 := gw.MatchRoute("/web/index", "POST")
	if route3 != nil {
		t.Error("/web/ POST should not match")
	}

	// 不匹配 /other/
	route4 := gw.MatchRoute("/other/path", "GET")
	if route4 != nil {
		t.Error("/other/ should not match any route")
	}
}

// ============================================================
// 8. TestGatewayRoutePriority — 路由优先级
// ============================================================
func TestGatewayRoutePriority(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 低优先级通用路由
	gw.AddRoute(GatewayRoute{
		ID: "general", Name: "General", PathPattern: "/api/",
		UpstreamURL: "http://general:8080", Methods: []string{"GET", "POST"},
		Enabled: true, Priority: 5,
	})
	// 高优先级特定路由
	gw.AddRoute(GatewayRoute{
		ID: "specific", Name: "Specific", PathPattern: "/api/v2/",
		UpstreamURL: "http://v2:8080", Methods: []string{"GET", "POST"},
		Enabled: true, Priority: 20,
	})

	// /api/v2/users 应该匹配高优先级路由
	route := gw.MatchRoute("/api/v2/users", "GET")
	if route == nil {
		t.Fatal("should match a route")
	}
	if route.ID != "specific" {
		t.Errorf("should match specific route, got %q", route.ID)
	}

	// /api/v1/users 应该匹配通用路由
	route2 := gw.MatchRoute("/api/v1/users", "GET")
	if route2 == nil {
		t.Fatal("should match general route")
	}
	if route2.ID != "general" {
		t.Errorf("should match general route, got %q", route2.ID)
	}
}

// ============================================================
// 9. TestGatewayTransformHeaders — 请求头转换
// ============================================================
func TestGatewayTransformHeaders(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	route := &GatewayRoute{
		AddHeaders: map[string]string{
			"X-Custom-Header": "custom-value",
			"X-Service":       "gateway",
		},
		RemoveHeaders: []string{"X-Secret-Key", "X-Internal"},
	}

	claims := &GWJWTClaims{
		TenantID: "tenant-t1",
	}

	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("X-Secret-Key", "should-be-removed")
	r.Header.Set("X-Internal", "also-removed")
	r.Header.Set("X-Keep", "keep-this")

	gw.TransformRequest(r, route, claims)

	// 检查注入的 header
	if r.Header.Get("X-Custom-Header") != "custom-value" {
		t.Error("X-Custom-Header should be set")
	}
	if r.Header.Get("X-Service") != "gateway" {
		t.Error("X-Service should be set")
	}

	// 检查删除的 header
	if r.Header.Get("X-Secret-Key") != "" {
		t.Error("X-Secret-Key should be removed")
	}
	if r.Header.Get("X-Internal") != "" {
		t.Error("X-Internal should be removed")
	}

	// 检查保留的 header
	if r.Header.Get("X-Keep") != "keep-this" {
		t.Error("X-Keep should be preserved")
	}

	// 检查 tenant ID
	if r.Header.Get("X-Tenant-ID") != "tenant-t1" {
		t.Errorf("X-Tenant-ID = %q, want tenant-t1", r.Header.Get("X-Tenant-ID"))
	}

	// 检查 request ID
	if r.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID should be set")
	}
}

// ============================================================
// 10. TestGatewayCanaryRouting — 灰度路由
// ============================================================
func TestGatewayCanaryRouting(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	route := &GatewayRoute{
		UpstreamURL:    "http://main:8080",
		CanaryPercent:  50,
		CanaryUpstream: "http://canary:8080",
	}

	mainCount := 0
	canaryCount := 0
	iterations := 1000

	for i := 0; i < iterations; i++ {
		upstream := gw.SelectUpstream(route)
		if upstream == "http://main:8080" {
			mainCount++
		} else if upstream == "http://canary:8080" {
			canaryCount++
		}
	}

	// 50% 灰度，允许 ±15% 的误差
	ratio := float64(canaryCount) / float64(iterations)
	if ratio < 0.35 || ratio > 0.65 {
		t.Errorf("canary ratio = %.2f, expected ~0.50 (main=%d, canary=%d)", ratio, mainCount, canaryCount)
	}

	// 0% 灰度全走主线
	route2 := &GatewayRoute{
		UpstreamURL:    "http://main:8080",
		CanaryPercent:  0,
		CanaryUpstream: "http://canary:8080",
	}
	for i := 0; i < 100; i++ {
		if gw.SelectUpstream(route2) != "http://main:8080" {
			t.Fatal("0% canary should always return main upstream")
		}
	}

	// 100% 灰度全走 canary
	route3 := &GatewayRoute{
		UpstreamURL:    "http://main:8080",
		CanaryPercent:  100,
		CanaryUpstream: "http://canary:8080",
	}
	for i := 0; i < 100; i++ {
		if gw.SelectUpstream(route3) != "http://canary:8080" {
			t.Fatal("100% canary should always return canary upstream")
		}
	}
}

// ============================================================
// 11. TestGatewayRouteAdd — 添加路由
// ============================================================
func TestGatewayRouteAdd(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	route := GatewayRoute{
		Name:        "Test Route",
		PathPattern: "/test/",
		UpstreamURL: "http://test:8080",
		Methods:     []string{"GET", "POST", "PUT"},
		Auth:        "jwt",
		Enabled:     true,
		Priority:    15,
	}

	err := gw.AddRoute(route)
	if err != nil {
		t.Fatalf("AddRoute failed: %v", err)
	}

	routes := gw.ListRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	r := routes[0]
	if r.Name != "Test Route" {
		t.Errorf("name = %q, want 'Test Route'", r.Name)
	}
	if r.PathPattern != "/test/" {
		t.Errorf("path = %q, want '/test/'", r.PathPattern)
	}
	if r.UpstreamURL != "http://test:8080" {
		t.Errorf("upstream = %q, want 'http://test:8080'", r.UpstreamURL)
	}
	if r.Auth != "jwt" {
		t.Errorf("auth = %q, want 'jwt'", r.Auth)
	}
	if r.Priority != 15 {
		t.Errorf("priority = %d, want 15", r.Priority)
	}
	if r.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

// ============================================================
// 12. TestGatewayRouteUpdate — 更新路由
// ============================================================
func TestGatewayRouteUpdate(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	gw.AddRoute(GatewayRoute{
		ID: "update-test", Name: "Original", PathPattern: "/orig/",
		UpstreamURL: "http://orig:8080", Enabled: true, Priority: 10,
	})

	// 更新
	err := gw.UpdateRoute("update-test", GatewayRoute{
		Name: "Updated", PathPattern: "/updated/",
		UpstreamURL: "http://updated:9090", Enabled: true, Priority: 20,
		Auth: "apikey",
	})
	if err != nil {
		t.Fatalf("UpdateRoute failed: %v", err)
	}

	routes := gw.ListRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	r := routes[0]
	if r.Name != "Updated" {
		t.Errorf("name = %q, want 'Updated'", r.Name)
	}
	if r.PathPattern != "/updated/" {
		t.Errorf("path = %q, want '/updated/'", r.PathPattern)
	}
	if r.Priority != 20 {
		t.Errorf("priority = %d, want 20", r.Priority)
	}

	// 更新不存在的路由
	err = gw.UpdateRoute("nonexistent", GatewayRoute{Name: "X"})
	if err == nil {
		t.Error("update nonexistent should fail")
	}
}

// ============================================================
// 13. TestGatewayRouteRemove — 删除路由
// ============================================================
func TestGatewayRouteRemove(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	gw.AddRoute(GatewayRoute{
		ID: "to-delete", Name: "Delete Me", PathPattern: "/del/",
		UpstreamURL: "http://del:8080", Enabled: true, Priority: 10,
	})

	if len(gw.ListRoutes()) != 1 {
		t.Fatal("should have 1 route")
	}

	err := gw.RemoveRoute("to-delete")
	if err != nil {
		t.Fatalf("RemoveRoute failed: %v", err)
	}

	if len(gw.ListRoutes()) != 0 {
		t.Error("should have 0 routes after removal")
	}

	// 删除不存在的路由
	err = gw.RemoveRoute("nonexistent")
	if err == nil {
		t.Error("remove nonexistent should fail")
	}
}

// ============================================================
// 14. TestGatewayStats — 统计
// ============================================================
func TestGatewayStats(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 记录一些请求
	gw.RecordRequest("route-1", "GET", "http://upstream1:8080", 10.5)
	gw.RecordRequest("route-1", "GET", "http://upstream1:8080", 20.5)
	gw.RecordRequest("route-2", "POST", "http://upstream2:8080", 30.0)
	gw.RecordRequest("route-1", "POST", "http://upstream1:8080", 15.0)

	stats := gw.GetStats()

	if stats.TotalRequests != 4 {
		t.Errorf("total = %d, want 4", stats.TotalRequests)
	}

	if stats.RouteHits["route-1"] != 3 {
		t.Errorf("route-1 hits = %d, want 3", stats.RouteHits["route-1"])
	}
	if stats.RouteHits["route-2"] != 1 {
		t.Errorf("route-2 hits = %d, want 1", stats.RouteHits["route-2"])
	}

	if stats.MethodBreakdown["GET"] != 2 {
		t.Errorf("GET count = %d, want 2", stats.MethodBreakdown["GET"])
	}
	if stats.MethodBreakdown["POST"] != 2 {
		t.Errorf("POST count = %d, want 2", stats.MethodBreakdown["POST"])
	}

	// 上游平均延迟
	avgLatency := stats.UpstreamLatency["http://upstream1:8080"]
	expected := (10.5 + 20.5 + 15.0) / 3.0
	if avgLatency < expected-0.1 || avgLatency > expected+0.1 {
		t.Errorf("upstream1 avg latency = %.2f, want ~%.2f", avgLatency, expected)
	}
}

// ============================================================
// 15. TestGatewayConfig — 配置更新
// ============================================================
func TestGatewayConfig(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 初始配置
	cfg := gw.GetConfig()
	if !cfg.Enabled {
		t.Error("should be enabled")
	}
	if !cfg.JWTEnabled {
		t.Error("JWT should be enabled")
	}
	if !cfg.APIKeyEnabled {
		t.Error("API key should be enabled")
	}

	// 更新配置
	newCfg := APIGatewayConfig{
		Enabled:       true,
		JWTSecret:     "new-secret",
		JWTEnabled:    false,
		APIKeyEnabled: true,
		APIKeys:       []string{"new-key-1"},
	}
	gw.UpdateConfig(newCfg)

	updated := gw.GetConfig()
	if updated.JWTEnabled {
		t.Error("JWT should be disabled after update")
	}
	if updated.JWTSecret != "new-secret" {
		t.Errorf("secret = %q, want 'new-secret'", updated.JWTSecret)
	}
	if len(updated.APIKeys) != 1 || updated.APIKeys[0] != "new-key-1" {
		t.Error("API keys should be updated")
	}

	// 验证新密钥生效
	claims := GWJWTClaims{
		TenantID: "test", Role: "admin", Sub: "test",
		Exp: time.Now().Add(1 * time.Hour).Unix(),
	}
	token, err := gw.GenerateGWJWT(claims)
	if err != nil {
		t.Fatalf("generate with new secret: %v", err)
	}
	_, err = gw.ValidateGWJWT(token)
	if err != nil {
		t.Fatalf("validate with new secret: %v", err)
	}
}

// ============================================================
// 16. TestGatewayLog — 日志记录与查询
// ============================================================
func TestGatewayLog(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	gw.LogRequest(GatewayLogEntry{
		Path: "/api/v1/users", Method: "GET", RouteID: "r1",
		TenantID: "t1", Upstream: "http://up:8080", StatusCode: 200,
		LatencyMs: 15.5, AuthResult: "jwt_ok",
	})
	gw.LogRequest(GatewayLogEntry{
		Path: "/api/v1/orders", Method: "POST", RouteID: "r2",
		TenantID: "t2", Upstream: "http://up:8080", StatusCode: 201,
		LatencyMs: 25.0, AuthResult: "apikey_ok",
	})
	gw.LogRequest(GatewayLogEntry{
		Path: "/api/v1/users", Method: "GET", RouteID: "r1",
		TenantID: "t1", Upstream: "http://up:8080", StatusCode: 200,
		LatencyMs: 12.0, AuthResult: "jwt_ok",
	})

	// 查全部
	all := gw.QueryLog(10, "", "")
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}

	// 按路由过滤
	r1Logs := gw.QueryLog(10, "r1", "")
	if len(r1Logs) != 2 {
		t.Errorf("expected 2 r1 entries, got %d", len(r1Logs))
	}

	// 按租户过滤
	t2Logs := gw.QueryLog(10, "", "t2")
	if len(t2Logs) != 1 {
		t.Errorf("expected 1 t2 entry, got %d", len(t2Logs))
	}
}

// ============================================================
// 17. TestGatewayMgmtAPIRoutes — 管理 API 路由 CRUD
// ============================================================
func TestGatewayMgmtAPIRoutes(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	api := &ManagementAPI{
		apiGateway: gw,
	}

	// 添加路由
	body := `{"name":"Test","path_pattern":"/test/","upstream_url":"http://test:8080","enabled":true,"priority":10}`
	r := httptest.NewRequest("POST", "/api/v1/gateway/routes", strings.NewReader(body))
	w := httptest.NewRecorder()
	api.handleGatewayRouteAdd(w, r)
	if w.Code != 200 {
		t.Errorf("add route: status = %d, body = %s", w.Code, w.Body.String())
	}

	// 路由列表
	r2 := httptest.NewRequest("GET", "/api/v1/gateway/routes", nil)
	w2 := httptest.NewRecorder()
	api.handleGatewayRouteList(w2, r2)
	if w2.Code != 200 {
		t.Errorf("list routes: status = %d", w2.Code)
	}
	var listResp struct {
		Routes []GatewayRoute `json:"routes"`
		Total  int            `json:"total"`
	}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	if listResp.Total != 1 {
		t.Errorf("total = %d, want 1", listResp.Total)
	}

	// 统计
	r3 := httptest.NewRequest("GET", "/api/v1/gateway/stats", nil)
	w3 := httptest.NewRecorder()
	api.handleGatewayStats(w3, r3)
	if w3.Code != 200 {
		t.Errorf("stats: status = %d", w3.Code)
	}
}

// ============================================================
// 18. TestGatewayMgmtAPIToken — 管理 API Token 生成验证
// ============================================================
func TestGatewayMgmtAPIToken(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	api := &ManagementAPI{
		apiGateway: gw,
	}

	// 生成 token
	tokenBody := `{"tenant_id":"acme-corp","role":"admin","expire_hours":24}`
	r := httptest.NewRequest("POST", "/api/v1/gateway/token", strings.NewReader(tokenBody))
	w := httptest.NewRecorder()
	api.handleGatewayTokenGenerate(w, r)
	if w.Code != 200 {
		t.Fatalf("generate token: status = %d, body = %s", w.Code, w.Body.String())
	}

	var tokenResp struct {
		Token    string      `json:"token"`
		Claims   GWJWTClaims `json:"claims"`
		ExpireAt int64       `json:"expire_at"`
	}
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	if tokenResp.Token == "" {
		t.Fatal("token should not be empty")
	}
	if tokenResp.Claims.TenantID != "acme-corp" {
		t.Errorf("tenant = %q, want acme-corp", tokenResp.Claims.TenantID)
	}

	// 验证 token
	validateBody := `{"token":"` + tokenResp.Token + `"}`
	r2 := httptest.NewRequest("POST", "/api/v1/gateway/validate", strings.NewReader(validateBody))
	w2 := httptest.NewRecorder()
	api.handleGatewayTokenValidate(w2, r2)
	if w2.Code != 200 {
		t.Fatalf("validate token: status = %d, body = %s", w2.Code, w2.Body.String())
	}

	var validateResp struct {
		Valid  bool        `json:"valid"`
		Claims GWJWTClaims `json:"claims"`
	}
	json.Unmarshal(w2.Body.Bytes(), &validateResp)
	if !validateResp.Valid {
		t.Error("token should be valid")
	}
}

// ============================================================
// 19. TestGatewayAuthenticateRequestJWT — 完整的 JWT 认证请求
// ============================================================
func TestGatewayAuthenticateRequestJWT(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	// 生成 token
	claims := GWJWTClaims{
		TenantID: "my-tenant",
		Role:     "admin",
		Sub:      "admin@example.com",
		Exp:      time.Now().Add(1 * time.Hour).Unix(),
	}
	token, _ := gw.GenerateGWJWT(claims)

	// 使用 Bearer token 认证
	r := httptest.NewRequest("GET", "/api/test", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	result, err := gw.AuthenticateRequest(r)
	if err != nil {
		t.Fatalf("auth should succeed: %v", err)
	}
	if result.TenantID != "my-tenant" {
		t.Errorf("tenant = %q, want my-tenant", result.TenantID)
	}
	if result.Role != "admin" {
		t.Errorf("role = %q, want admin", result.Role)
	}
}

// ============================================================
// 20. TestGatewayDisabledRoute — 禁用路由不匹配
// ============================================================
func TestGatewayDisabledRoute(t *testing.T) {
	gw, db := newTestGateway(t)
	defer db.Close()

	gw.AddRoute(GatewayRoute{
		ID: "disabled", Name: "Disabled", PathPattern: "/disabled/",
		UpstreamURL: "http://disabled:8080", Methods: []string{"GET"},
		Enabled: false, Priority: 100,
	})
	gw.AddRoute(GatewayRoute{
		ID: "enabled", Name: "Enabled", PathPattern: "/disabled/",
		UpstreamURL: "http://enabled:8080", Methods: []string{"GET"},
		Enabled: true, Priority: 5,
	})

	route := gw.MatchRoute("/disabled/test", "GET")
	if route == nil {
		t.Fatal("should match enabled route")
	}
	if route.ID != "enabled" {
		t.Errorf("should match enabled route, got %q", route.ID)
	}
}

// ============================================================
// 21. TestGatewaySortRoutesByPriority — 路由排序
// ============================================================
func TestGatewaySortRoutesByPriority(t *testing.T) {
	routes := []GatewayRoute{
		{ID: "low", Priority: 1},
		{ID: "high", Priority: 100},
		{ID: "mid", Priority: 50},
	}
	sortRoutesByPriority(routes)
	if routes[0].ID != "high" {
		t.Errorf("first should be high, got %q", routes[0].ID)
	}
	if routes[1].ID != "mid" {
		t.Errorf("second should be mid, got %q", routes[1].ID)
	}
	if routes[2].ID != "low" {
		t.Errorf("third should be low, got %q", routes[2].ID)
	}
}

// suppress unused import warning
var _ = http.StatusOK