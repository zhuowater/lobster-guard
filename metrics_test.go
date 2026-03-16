// metrics_test.go — MetricsCollector、RateLimiter 测试
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// Prometheus Metrics 测试（v3.4）
// ============================================================

func TestMetricsCollector_RecordRequest(t *testing.T) {
	mc := NewMetricsCollector()

	// 记录几个请求
	mc.RecordRequest("inbound", "pass", "lanxin", 5.0)
	mc.RecordRequest("inbound", "pass", "lanxin", 10.0)
	mc.RecordRequest("inbound", "block", "lanxin", 3.0)
	mc.RecordRequest("outbound", "pass", "lanxin", 8.0)

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.requestsTotal["inbound:pass:lanxin"] != 2 {
		t.Fatalf("inbound:pass:lanxin expected 2, got %d", mc.requestsTotal["inbound:pass:lanxin"])
	}
	if mc.requestsTotal["inbound:block:lanxin"] != 1 {
		t.Fatalf("inbound:block:lanxin expected 1, got %d", mc.requestsTotal["inbound:block:lanxin"])
	}
	if mc.requestsTotal["outbound:pass:lanxin"] != 1 {
		t.Fatalf("outbound:pass:lanxin expected 1, got %d", mc.requestsTotal["outbound:pass:lanxin"])
	}
}

func TestMetricsCollector_Histogram(t *testing.T) {
	mc := NewMetricsCollector()

	// 记录不同延迟的请求
	mc.RecordRequest("inbound", "pass", "test", 0.5)   // <= 1ms bucket
	mc.RecordRequest("inbound", "pass", "test", 3.0)   // <= 5ms bucket
	mc.RecordRequest("inbound", "pass", "test", 7.0)   // <= 10ms bucket
	mc.RecordRequest("inbound", "pass", "test", 30.0)  // <= 50ms bucket
	mc.RecordRequest("inbound", "pass", "test", 200.0) // <= 250ms bucket
	mc.RecordRequest("inbound", "pass", "test", 2000.0) // > 1000ms (only in +Inf)

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	h := mc.latencyBuckets["inbound"]
	if h == nil {
		t.Fatal("inbound histogram should exist")
	}
	if h.count != 6 {
		t.Fatalf("expected count 6, got %d", h.count)
	}

	// Verify cumulative bucket counts
	// buckets: 1, 5, 10, 25, 50, 100, 250, 500, 1000
	// 0.5 -> [1:1, 5:1, 10:1, 25:1, 50:1, 100:1, 250:1, 500:1, 1000:1]
	// 3.0 -> [5:1, 10:1, 25:1, 50:1, 100:1, 250:1, 500:1, 1000:1]
	// 7.0 -> [10:1, 25:1, 50:1, 100:1, 250:1, 500:1, 1000:1]
	// 30.0 -> [50:1, 100:1, 250:1, 500:1, 1000:1]
	// 200.0 -> [250:1, 500:1, 1000:1]
	// 2000.0 -> (none)
	// Per-bucket (non-cumulative): [1, 1, 1, 0, 1, 0, 1, 0, 0]
	expected := []int64{1, 1, 1, 0, 1, 0, 1, 0, 0}
	for i, e := range expected {
		if h.counts[i] != e {
			t.Errorf("bucket %d (le=%.0f): expected %d, got %d", i, h.buckets[i], e, h.counts[i])
		}
	}

	// sum should be 0.5+3+7+30+200+2000 = 2240.5
	expectedSum := 2240.5
	if h.sum < expectedSum-0.01 || h.sum > expectedSum+0.01 {
		t.Errorf("expected sum %.2f, got %.2f", expectedSum, h.sum)
	}
}

func TestMetricsCollector_RateLimit(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordRateLimit(true)
	mc.RecordRateLimit(true)
	mc.RecordRateLimit(false)

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.rateLimitAllowed != 2 {
		t.Fatalf("expected allowed 2, got %d", mc.rateLimitAllowed)
	}
	if mc.rateLimitDenied != 1 {
		t.Fatalf("expected denied 1, got %d", mc.rateLimitDenied)
	}
}

func TestMetricsCollector_WritePrometheus(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordRequest("inbound", "pass", "lanxin", 5.0)
	mc.RecordRequest("inbound", "block", "lanxin", 15.0)
	mc.RecordRequest("outbound", "pass", "lanxin", 3.0)
	mc.RecordRateLimit(true)
	mc.RecordRateLimit(false)

	var buf bytes.Buffer
	mc.WritePrometheus(&buf, 4, 3, 15, nil, "lanxin", "webhook", nil, nil, nil)

	output := buf.String()

	// 验证包含 HELP 和 TYPE
	checks := []string{
		"# HELP lobster_guard_requests_total",
		"# TYPE lobster_guard_requests_total counter",
		`lobster_guard_requests_total{direction="inbound",action="pass",channel="lanxin"} 1`,
		`lobster_guard_requests_total{direction="inbound",action="block",channel="lanxin"} 1`,
		`lobster_guard_requests_total{direction="outbound",action="pass",channel="lanxin"} 1`,
		"# HELP lobster_guard_request_duration_ms",
		"# TYPE lobster_guard_request_duration_ms histogram",
		`lobster_guard_request_duration_ms_bucket{direction="inbound",le="1"}`,
		`lobster_guard_request_duration_ms_bucket{direction="inbound",le="+Inf"} 2`,
		`lobster_guard_request_duration_ms_sum{direction="inbound"}`,
		`lobster_guard_request_duration_ms_count{direction="inbound"} 2`,
		"lobster_guard_upstreams_total 4",
		"lobster_guard_upstreams_healthy 3",
		"lobster_guard_routes_total 15",
		"lobster_guard_bridge_connected 0",
		`lobster_guard_rate_limit_total{decision="allowed"} 1`,
		`lobster_guard_rate_limit_total{decision="denied"} 1`,
		"lobster_guard_uptime_seconds",
		`lobster_guard_info{version="`+AppVersion+`",channel="lanxin",mode="webhook"} 1`,
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing: %s", check)
		}
	}
}

func TestMetricsCollector_WritePrometheus_WithBridge(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordBridgeReconnect()
	mc.RecordBridgeReconnect()
	mc.RecordBridgeMessage()

	bs := &BridgeStatus{Connected: true}

	var buf bytes.Buffer
	mc.WritePrometheus(&buf, 2, 2, 5, bs, "feishu", "bridge", nil, nil, nil)

	output := buf.String()

	if !strings.Contains(output, "lobster_guard_bridge_connected 1") {
		t.Error("bridge connected should be 1")
	}
	if !strings.Contains(output, "lobster_guard_bridge_reconnects_total 2") {
		t.Error("bridge reconnects should be 2")
	}
	if !strings.Contains(output, "lobster_guard_bridge_messages_total 1") {
		t.Error("bridge messages should be 1")
	}
	if !strings.Contains(output, `lobster_guard_info{version="`+AppVersion+`",channel="feishu",mode="bridge"} 1`) {
		t.Error("info metric should have feishu and bridge")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	// 创建临时数据库
	tmpDB, err := os.CreateTemp("", "metrics-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpDB.Name())
	tmpDB.Close()

	db, err := initDB(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg := &Config{
		InboundListen:        ":8443",
		OutboundListen:       ":8444",
		OpenClawUpstream:     "http://localhost:18790",
		LanxinUpstream:       "https://apigw.lx.qianxin.com",
		DBPath:               tmpDB.Name(),
		InboundDetectEnabled: true,
		OutboundAuditEnabled: true,
		ManagementListen:     ":9090",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 3,
		RouteDefaultPolicy:   "least-users",
		RoutePersist:         false,
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	outEngine := NewOutboundRuleEngine(nil)
	gp := NewGenericPlugin("", "")
	metrics := NewMetricsCollector()

	engine := NewRuleEngine()
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, metrics, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, gp, metrics, nil, nil, nil, nil, nil, nil, nil, nil)

	// 记录一些指标
	metrics.RecordRequest("inbound", "pass", "generic", 5.0)
	metrics.RecordRequest("outbound", "block", "generic", 10.0)

	// 发送 /metrics 请求
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain Content-Type, got %s", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "lobster_guard_requests_total") {
		t.Error("should contain lobster_guard_requests_total")
	}
	if !strings.Contains(body, "lobster_guard_info") {
		t.Error("should contain lobster_guard_info")
	}
	if !strings.Contains(body, `direction="inbound"`) {
		t.Error("should contain inbound direction")
	}
	if !strings.Contains(body, `direction="outbound"`) {
		t.Error("should contain outbound direction")
	}
}

func TestMetricsEndpoint_Disabled(t *testing.T) {
	// 创建临时数据库
	tmpDB, err := os.CreateTemp("", "metrics-disabled-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpDB.Name())
	tmpDB.Close()

	db, err := initDB(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg := &Config{
		InboundListen:        ":8443",
		OutboundListen:       ":8444",
		OpenClawUpstream:     "http://localhost:18790",
		LanxinUpstream:       "https://apigw.lx.qianxin.com",
		DBPath:               tmpDB.Name(),
		ManagementListen:     ":9090",
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 3,
		RouteDefaultPolicy:   "least-users",
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	outEngine := NewOutboundRuleEngine(nil)
	gp := NewGenericPlugin("", "")

	engine := NewRuleEngine()
	inbound := NewInboundProxy(cfg, gp, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, gp, nil, nil, nil, nil, nil, nil, nil, nil, nil) // metrics=nil

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Fatalf("expected 404 when metrics disabled, got %d", rec.Code)
	}
}

func TestUpstreamPool_Count(t *testing.T) {
	cfg := &Config{
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 3,
	}
	pool := NewUpstreamPool(cfg, nil)
	pool.Register("test-1", "127.0.0.1", 8001, nil)
	pool.Register("test-2", "127.0.0.1", 8002, nil)

	total, healthy := pool.Count()
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if healthy != 2 {
		t.Fatalf("expected healthy 2, got %d", healthy)
	}

	// Mark one as unhealthy
	pool.mu.Lock()
	pool.upstreams["test-2"].Healthy = false
	pool.mu.Unlock()

	total, healthy = pool.Count()
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if healthy != 1 {
		t.Fatalf("expected healthy 1, got %d", healthy)
	}
}

func TestConfigMetricsEnabled(t *testing.T) {
	// Default (nil) -> enabled
	cfg := &Config{}
	if !cfg.IsMetricsEnabled() {
		t.Fatal("default should be enabled")
	}

	// Explicitly enabled
	enabled := true
	cfg.MetricsEnabled = &enabled
	if !cfg.IsMetricsEnabled() {
		t.Fatal("explicit true should be enabled")
	}

	// Explicitly disabled
	disabled := false
	cfg.MetricsEnabled = &disabled
	if cfg.IsMetricsEnabled() {
		t.Fatal("explicit false should be disabled")
	}
}

func TestLatencyHistogram(t *testing.T) {
	h := NewLatencyHistogram()

	h.Observe(0.5)
	h.Observe(3.0)
	h.Observe(50.0)
	h.Observe(500.0)
	h.Observe(1500.0)

	if h.count != 5 {
		t.Fatalf("expected count 5, got %d", h.count)
	}

	expectedSum := 0.5 + 3.0 + 50.0 + 500.0 + 1500.0
	if h.sum < expectedSum-0.01 || h.sum > expectedSum+0.01 {
		t.Errorf("expected sum %.2f, got %.2f", expectedSum, h.sum)
	}

	// buckets: 1, 5, 10, 25, 50, 100, 250, 500, 1000
	// 0.5 -> le=1: +1
	// 3.0 -> le=5: +1
	// 50.0 -> le=50: +1
	// 500.0 -> le=500: +1
	// 1500.0 -> none
	expectedCounts := []int64{1, 1, 0, 0, 1, 0, 0, 1, 0}
	for i, e := range expectedCounts {
		if h.counts[i] != e {
			t.Errorf("bucket %d (le=%.0f): expected %d, got %d", i, h.buckets[i], e, h.counts[i])
		}
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{1.0, "1"},
		{5.0, "5"},
		{10.0, "10"},
		{0.5, "0.5"},
		{100.0, "100"},
		{1000.0, "1000"},
	}
	for _, tt := range tests {
		got := formatFloat(tt.input)
		if got != tt.expected {
			t.Errorf("formatFloat(%f) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

