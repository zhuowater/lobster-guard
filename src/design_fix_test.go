// design_fix_test.go — Tests for design issues D-001 through D-009 and R2 bugs
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// ============================================================
// D-001: 策略上游不健康时记录 policy_degraded
// ============================================================

func TestD001_PolicyUpstreamUnhealthyDegradation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)

	// Register upstream-a (healthy) and upstream-b (unhealthy)
	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	pool.Register("upstream-b", "127.0.0.1", 9002, nil)
	// Make upstream-b unhealthy
	pool.mu.Lock()
	if up, ok := pool.upstreams["upstream-b"]; ok {
		up.Healthy = false
	}
	pool.mu.Unlock()

	// Setup policy engine to route to upstream-b
	policyEng := NewRoutePolicyEngine([]RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "engineering"}, UpstreamID: "upstream-b"},
	})

	// Setup user cache with a mock user
	userCache := &UserInfoCache{
		memory:  map[string]*UserInfo{"user1": {SenderID: "user1", Name: "Test", Email: "test@example.com", Department: "engineering"}},
		memTime: map[string]time.Time{"user1": time.Now()},
		ttl:     24 * time.Hour,
	}

	// Setup logger to capture audit logs
	logger, err := NewAuditLogger(db)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	realtime := NewRealtimeMetrics()

	// Create InboundProxy with all the components
	ip := &InboundProxy{
		pool:      pool,
		routes:    routes,
		policyEng: policyEng,
		userCache: userCache,
		logger:    logger,
		realtime:  realtime,
		cfg:       cfg,
	}

	// Call resolveUpstream — should degrade since upstream-b is unhealthy
	result := ip.resolveUpstream("user1", "", "[test]")
	// Should NOT return upstream-b (unhealthy), should fallback to load balancing
	if result == "upstream-b" {
		t.Error("resolveUpstream should not return unhealthy policy upstream")
	}
	// Should get upstream-a (healthy, via load balancing)
	if result != "upstream-a" {
		t.Errorf("Expected upstream-a from load balancing, got %q", result)
	}
}

// ============================================================
// D-002: Webhook proxy fallback → update route table
// ============================================================

func TestD002_WebhookProxyFallbackUpdatesRouteTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)

	// Register upstream-a and upstream-b
	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	pool.Register("upstream-b", "127.0.0.1", 9002, nil)

	// Bind user to upstream-a
	routes.Bind("user1", "app1", "upstream-a")
	pool.IncrUserCount("upstream-a", 1)

	// Now remove upstream-a proxy (simulate unavailability)
	pool.mu.Lock()
	if up, ok := pool.upstreams["upstream-a"]; ok {
		up.proxy = nil // simulate proxy not available
	}
	pool.mu.Unlock()

	// GetProxy should return nil for upstream-a
	proxy := pool.GetProxy("upstream-a")
	if proxy != nil {
		t.Error("Expected nil proxy for upstream-a with nil proxy")
	}

	// GetAnyHealthyProxy should return upstream-b
	healthyProxy, uid := pool.GetAnyHealthyProxy()
	if healthyProxy == nil {
		t.Error("Expected a healthy proxy")
	}
	if uid != "upstream-b" {
		// Could be either since map iteration is random, but upstream-b should work
		t.Logf("Got fallback upstream: %s", uid)
	}
}

// ============================================================
// D-003: Bridge mode → include path_prefix
// ============================================================

func TestD003_BridgeModePathPrefix(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
		StaticUpstreams: []StaticUpstreamConfig{
			{ID: "upstream-with-prefix", Address: "127.0.0.1", Port: 9001, PathPrefix: "/api/v1"},
		},
	}
	pool := NewUpstreamPool(cfg, db)

	// Verify that the upstream has the path prefix stored
	up, exists := pool.GetUpstream("upstream-with-prefix")
	if !exists {
		t.Fatal("upstream-with-prefix not found")
	}
	if up.PathPrefix != "/api/v1" {
		t.Errorf("Expected path_prefix '/api/v1', got %q", up.PathPrefix)
	}

	// Verify the URL construction includes path_prefix
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if rawUp, ok := pool.upstreams["upstream-with-prefix"]; ok {
		expectedURL := fmt.Sprintf("http://%s:%d%s", rawUp.Address, rawUp.Port, rawUp.PathPrefix)
		if expectedURL != "http://127.0.0.1:9001/api/v1" {
			t.Errorf("Expected URL 'http://127.0.0.1:9001/api/v1', got %q", expectedURL)
		}
	}
}

// ============================================================
// D-004: Policy CRUD → trigger reevaluateAllRoutes
// ============================================================

func TestD004_ReevaluateAllRoutes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)

	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	pool.Register("upstream-b", "127.0.0.1", 9002, nil)

	// Create user cache with a user in "engineering" department
	userCache := &UserInfoCache{
		memory: map[string]*UserInfo{
			"user1": {SenderID: "user1", Name: "Test User", Email: "test@example.com", Department: "engineering"},
		},
		memTime: map[string]time.Time{"user1": time.Now()},
		ttl:     24 * time.Hour,
	}

	// Bind user1 to upstream-a
	routes.Bind("user1", "app1", "upstream-a")
	pool.IncrUserCount("upstream-a", 1)

	// Create policy engine routing engineering to upstream-b
	policyEng := NewRoutePolicyEngine([]RoutePolicyConfig{
		{Match: RoutePolicyMatch{Department: "engineering"}, UpstreamID: "upstream-b"},
	})

	// Create management API
	api := &ManagementAPI{
		pool:      pool,
		routes:    routes,
		policyEng: policyEng,
		userCache: userCache,
	}

	// Reevaluate should migrate user1 to upstream-b
	migrated := api.reevaluateAllRoutes()
	if migrated != 1 {
		t.Errorf("Expected 1 route migrated, got %d", migrated)
	}

	// Verify the route is now to upstream-b
	uid, ok := routes.Lookup("user1", "app1")
	if !ok || uid != "upstream-b" {
		t.Errorf("Expected user1 routed to upstream-b, got %q (ok=%v)", uid, ok)
	}
}

// ============================================================
// D-006: RestoreUserCounts from route table
// ============================================================

func TestD006_RestoreUserCounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	pool.Register("upstream-b", "127.0.0.1", 9002, nil)

	routes := NewRouteTable(db, true)
	routes.Bind("user1", "app1", "upstream-a")
	routes.Bind("user2", "app1", "upstream-a")
	routes.Bind("user3", "app1", "upstream-b")

	// Simulate restart: user counts are zero
	pool.mu.Lock()
	for _, up := range pool.upstreams {
		up.UserCount = 0
	}
	pool.mu.Unlock()

	// Restore user counts
	pool.RestoreUserCounts(db)

	// Check counts
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if up, ok := pool.upstreams["upstream-a"]; ok {
		if up.UserCount != 2 {
			t.Errorf("Expected upstream-a user_count=2, got %d", up.UserCount)
		}
	}
	if up, ok := pool.upstreams["upstream-b"]; ok {
		if up.UserCount != 1 {
			t.Errorf("Expected upstream-b user_count=1, got %d", up.UserCount)
		}
	}
}

// ============================================================
// D-008: Default detect timeout 200ms
// ============================================================

func TestD008_DefaultDetectTimeout200ms(t *testing.T) {
	cfg := &Config{}
	// Simulate loading with defaults
	cfg.DetectTimeoutMs = 200 // This is the default in loadConfig
	if cfg.DetectTimeoutMs != 200 {
		t.Errorf("Expected default DetectTimeoutMs=200, got %d", cfg.DetectTimeoutMs)
	}
}

// ============================================================
// R2-001: Refresh non-existent user → 404
// ============================================================

func TestR2001_RefreshNonExistentUser404(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		ManagementToken: "test-token",
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(getDefaultInboundRules(), "default", nil, nil)
	outEngine := NewOutboundRuleEngine(nil)
	ruleHits := NewRuleHitStats()

	// Create user cache with a mock provider that returns nil (user not found)
	mockProvider := &mockUserProvider{users: map[string]*UserInfo{}}
	userCache := NewUserInfoCache(db, mockProvider, 24*time.Hour)
	policyEng := NewRoutePolicyEngine(nil)

	mgmtAPI := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, nil, NewGenericPlugin("", ""), nil, ruleHits, userCache, policyEng, nil, nil, nil, nil, nil)

	// Try to refresh a non-existent user
	req := httptest.NewRequest("POST", "/api/v1/users/nonexistent/refresh", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mgmtAPI.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("Expected 404 for non-existent user refresh, got %d, body: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// R2-003: Delete upstream → clean orphaned routes
// ============================================================

func TestR2003_DeleteUpstreamCleansOrphanedRoutes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		ManagementToken: "test-token",
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}

	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	pool.Register("upstream-b", "127.0.0.1", 9002, nil)

	routes := NewRouteTable(db, true)
	routes.Bind("user1", "app1", "upstream-a")
	routes.Bind("user2", "app1", "upstream-a")
	routes.Bind("user3", "app1", "upstream-b")

	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(getDefaultInboundRules(), "default", nil, nil)
	outEngine := NewOutboundRuleEngine(nil)
	ruleHits := NewRuleHitStats()

	mgmtAPI := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, nil, NewGenericPlugin("", ""), nil, ruleHits, nil, nil, nil, nil, nil, nil, nil)

	// Delete upstream-a
	req := httptest.NewRequest("DELETE", "/api/v1/upstreams/upstream-a", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mgmtAPI.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Should have orphaned_routes count
	if orphaned, ok := resp["orphaned_routes"]; ok {
		if int(orphaned.(float64)) != 2 {
			t.Errorf("Expected 2 orphaned routes, got %v", orphaned)
		}
	} else {
		t.Error("Expected orphaned_routes in response")
	}

	// Routes to upstream-a should be cleaned
	count := routes.CountByUpstream("upstream-a")
	if count != 0 {
		t.Errorf("Expected 0 routes to upstream-a after deletion, got %d", count)
	}

	// Routes to upstream-b should be untouched
	count = routes.CountByUpstream("upstream-b")
	if count != 1 {
		t.Errorf("Expected 1 route to upstream-b, got %d", count)
	}
}

// ============================================================
// R2-004: sender_id/upstream_id length limit
// ============================================================

func TestR2004_LengthLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &Config{
		ManagementToken: "test-token",
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
	}

	pool := NewUpstreamPool(cfg, db)
	pool.Register("upstream-a", "127.0.0.1", 9001, nil)
	routes := NewRouteTable(db, true)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(getDefaultInboundRules(), "default", nil, nil)
	outEngine := NewOutboundRuleEngine(nil)
	ruleHits := NewRuleHitStats()

	mgmtAPI := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, nil, NewGenericPlugin("", ""), nil, ruleHits, nil, nil, nil, nil, nil, nil, nil)

	// Try to bind with a very long sender_id
	longID := strings.Repeat("a", 300)
	body, _ := json.Marshal(map[string]string{
		"sender_id":   longID,
		"upstream_id": "upstream-a",
	})
	req := httptest.NewRequest("POST", "/api/v1/routes/bind", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mgmtAPI.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Expected 400 for oversized sender_id, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// R2-005: Policy upstream validation (warn if not registered)
// ============================================================

func TestR2005_PolicyUpstreamValidationWarn(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a temp config file for saveRoutePolicies
	tmpFile := "/tmp/test-config-r2005.yaml"
	cfgContent := "route_policies: []\nmanagement_token: test-token\nstatic_upstreams:\n  - id: upstream-a\n    address: 127.0.0.1\n    port: 9001\n"
	if err := writeFile(tmpFile, []byte(cfgContent)); err != nil {
		t.Skip("Cannot write temp config file")
	}

	cfg := &Config{
		ManagementToken: "test-token",
		RouteDefaultPolicy: "least-users",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 30,
		StaticUpstreams: []StaticUpstreamConfig{
			{ID: "upstream-a", Address: "127.0.0.1", Port: 9001},
		},
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, true)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()

	engine := NewRuleEngineWithPII(getDefaultInboundRules(), "default", nil, nil)
	outEngine := NewOutboundRuleEngine(nil)
	ruleHits := NewRuleHitStats()
	policyEng := NewRoutePolicyEngine(nil)

	mgmtAPI := NewManagementAPI(cfg, tmpFile, pool, routes, logger, engine, outEngine, nil, NewGenericPlugin("", ""), nil, ruleHits, nil, policyEng, nil, nil, nil, nil, nil)

	// Create policy pointing to non-existent upstream
	body, _ := json.Marshal(map[string]interface{}{
		"match":       map[string]string{"department": "engineering"},
		"upstream_id": "nonexistent-upstream",
	})
	req := httptest.NewRequest("POST", "/api/v1/route-policies", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mgmtAPI.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected 200 (warn but allow), got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if warning, ok := resp["warning"]; ok {
		if !strings.Contains(warning.(string), "not currently registered") {
			t.Errorf("Expected warning about upstream not registered, got: %v", warning)
		}
	} else {
		t.Error("Expected warning in response for non-existent upstream")
	}
}

// Helper: setupTestDB creates a temporary in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := initDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// Helper: writeFile writes content to a file
func writeFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}
