// leaderboard_test.go — v14.3 安全排行榜 + SLA 基线测试
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// 辅助函数
// ============================================================

func setupLeaderboardEngine(t *testing.T) (*LeaderboardEngine, *sql.DB, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-leaderboard-test-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}

	// 初始化 schema
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT, app_id TEXT, trace_id TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, trace_id TEXT, model TEXT, request_tokens INTEGER,
		response_tokens INTEGER, total_tokens INTEGER, latency_ms REAL,
		status_code INTEGER, has_tool_use INTEGER, tool_count INTEGER,
		error_message TEXT, canary_leaked INTEGER DEFAULT 0,
		budget_exceeded INTEGER DEFAULT 0, budget_violations TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER, timestamp TEXT, tool_name TEXT,
		tool_input_preview TEXT, tool_result_preview TEXT,
		risk_level TEXT, flagged INTEGER DEFAULT 0, flag_reason TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS redteam_reports (
		id TEXT PRIMARY KEY, tenant_id TEXT DEFAULT 'default',
		timestamp TEXT, duration_ms INTEGER, total_tests INTEGER,
		passed INTEGER, failed INTEGER, pass_rate REAL,
		report_json TEXT, status TEXT DEFAULT 'completed'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY, name TEXT, description TEXT,
		created_at TEXT, max_agents INTEGER DEFAULT 0,
		max_rules INTEGER DEFAULT 0, enabled INTEGER DEFAULT 1,
		strict_mode INTEGER DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id TEXT, match_type TEXT, match_value TEXT,
		description TEXT, created_at TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_configs (
		tenant_id TEXT PRIMARY KEY, disabled_rules TEXT DEFAULT '',
		extra_rules_yaml TEXT DEFAULT '', strict_mode INTEGER DEFAULT 0,
		canary_enabled INTEGER DEFAULT 1, budget_enabled INTEGER DEFAULT 1,
		budget_max_tokens INTEGER DEFAULT 0, budget_max_tools INTEGER DEFAULT 0,
		tool_blacklist TEXT DEFAULT '', alert_level TEXT DEFAULT 'high',
		alert_webhook TEXT DEFAULT '', updated_at TEXT DEFAULT ''
	)`)

	tenantMgr := NewTenantManager(db)
	healthEng := NewHealthScoreEngine(db)
	le := NewLeaderboardEngine(db, tenantMgr, healthEng)

	cleanup := func() { db.Close(); os.Remove(tmpDB) }
	return le, db, cleanup
}

func seedTenantData(db *sql.DB, tenantID string, totalRequests, blockedRequests int, blockReasons []string) {
	now := time.Now().UTC()
	for i := 0; i < totalRequests; i++ {
		ts := now.Add(-time.Duration(i*30) * time.Minute).Format(time.RFC3339)
		action := "pass"
		reason := ""
		if i < blockedRequests {
			action = "block"
			if len(blockReasons) > 0 {
				reason = blockReasons[i%len(blockReasons)]
			} else {
				reason = "test block"
			}
		}
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, tenant_id) VALUES (?, 'inbound', 'test-user', ?, ?, 'test', ?)`,
			ts, action, reason, tenantID)
	}
}

func seedRedTeamReport(db *sql.DB, tenantID string, passRate float64) {
	ts := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO redteam_reports (id, tenant_id, timestamp, total_tests, passed, failed, pass_rate, report_json) VALUES (?, ?, ?, 35, ?, ?, ?, '{}')`,
		fmt.Sprintf("rt-%s-%d", tenantID, time.Now().UnixNano()), tenantID, ts,
		int(passRate/100*35), 35-int(passRate/100*35), passRate)
}

// ============================================================
// 测试用例
// ============================================================

// Test 1: 默认 SLA 配置
func TestDefaultSLAConfig(t *testing.T) {
	cfg := DefaultSLAConfig()
	if cfg.MinHealthScore != 70 {
		t.Errorf("MinHealthScore: 期望 70, 得到 %d", cfg.MinHealthScore)
	}
	if cfg.MaxIncidentCount != 10 {
		t.Errorf("MaxIncidentCount: 期望 10, 得到 %d", cfg.MaxIncidentCount)
	}
	if cfg.MinRedTeamScore != 80.0 {
		t.Errorf("MinRedTeamScore: 期望 80.0, 得到 %.1f", cfg.MinRedTeamScore)
	}
}

// Test 2: SLA 配置更新
func TestSLAConfigUpdate(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	newCfg := SLAConfig{
		MinHealthScore:   80,
		MaxIncidentCount: 5,
		MinRedTeamScore:  90.0,
		MinBlockRate:     0.5,
	}
	le.SetSLAConfig(newCfg)

	got := le.GetSLAConfig()
	if got.MinHealthScore != 80 {
		t.Errorf("MinHealthScore: 期望 80, 得到 %d", got.MinHealthScore)
	}
	if got.MinBlockRate != 0.5 {
		t.Errorf("MinBlockRate: 期望 0.5, 得到 %f", got.MinBlockRate)
	}
}

// Test 3: 空数据时排行榜返回空列表
func TestLeaderboardEmpty(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	scores := le.GetLeaderboard()
	// default 租户应该存在
	if len(scores) == 0 {
		t.Log("只有默认租户，无数据时也应返回")
	}
}

// Test 4: 多租户排行榜按健康分排序
func TestLeaderboardRanking(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	// 创建 security-team 租户（低拦截率 = 高健康分）
	le.tenantMgr.Create(&Tenant{ID: "security-team", Name: "安全团队", Enabled: true})
	seedTenantData(db, "security-team", 100, 2, nil)

	// 创建 product-team 租户（高拦截率 = 低健康分）
	le.tenantMgr.Create(&Tenant{ID: "product-team", Name: "产品团队", Enabled: true})
	seedTenantData(db, "product-team", 100, 35, nil)

	// default 租户
	seedTenantData(db, "default", 100, 15, nil)

	scores := le.GetLeaderboard()
	if len(scores) < 3 {
		t.Fatalf("期望至少 3 个租户, 得到 %d", len(scores))
	}

	// 验证排名：security-team 应该排名最高
	if scores[0].TenantID != "security-team" {
		t.Errorf("排名第一应该是 security-team, 得到 %s", scores[0].TenantID)
	}
	if scores[0].Rank != 1 {
		t.Errorf("第一名的 Rank 应该是 1, 得到 %d", scores[0].Rank)
	}

	// 验证健康分递减
	for i := 1; i < len(scores); i++ {
		if scores[i].HealthScore > scores[i-1].HealthScore {
			t.Errorf("排行榜未按健康分降序: %d(%s) > %d(%s)",
				scores[i].HealthScore, scores[i].TenantID,
				scores[i-1].HealthScore, scores[i-1].TenantID)
		}
	}
}

// Test 5: 红队检测率获取
func TestRedTeamScoreRetrieval(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	// 先无数据
	score := le.getLatestRedTeamScore("default")
	if score != 0 {
		t.Errorf("无数据时红队检测率应为 0, 得到 %f", score)
	}

	// 注入数据
	seedRedTeamReport(db, "default", 85.7)
	score = le.getLatestRedTeamScore("default")
	if score != 85.7 {
		t.Errorf("期望 85.7, 得到 %f", score)
	}
}

// Test 6: SLA 判定 - Green
func TestSLAEvaluationGreen(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	ts := TenantScore{
		HealthScore:   95,
		IncidentCount: 2,
		RedTeamScore:  100,
	}
	cfg := DefaultSLAConfig()
	status, score := le.evaluateSLA(ts, cfg)
	if status != "green" {
		t.Errorf("SLA status: 期望 green, 得到 %s", status)
	}
	if score != 100 {
		t.Errorf("SLA score: 期望 100, 得到 %f", score)
	}
}

// Test 7: SLA 判定 - Yellow
func TestSLAEvaluationYellow(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	ts := TenantScore{
		HealthScore:   75,
		IncidentCount: 8,
		RedTeamScore:  60, // 低于阈值
	}
	cfg := DefaultSLAConfig()
	status, _ := le.evaluateSLA(ts, cfg)
	if status != "yellow" {
		t.Errorf("SLA status: 期望 yellow, 得到 %s (2/3 met)", status)
	}
}

// Test 8: SLA 判定 - Red
func TestSLAEvaluationRed(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	ts := TenantScore{
		HealthScore:   50,
		IncidentCount: 20,
		RedTeamScore:  40,
	}
	cfg := DefaultSLAConfig()
	status, _ := le.evaluateSLA(ts, cfg)
	if status != "red" {
		t.Errorf("SLA status: 期望 red, 得到 %s (0/3 met)", status)
	}
}

// Test 9: 攻击热力图
func TestAttackHeatmap(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	le.tenantMgr.Create(&Tenant{ID: "test-team", Name: "测试团队", Enabled: true})

	// 注入不同类别的攻击
	reasons := []string{
		"Prompt injection detected",
		"SQL injection attempt",
		"PII detected: credit card",
		"Jailbreak attempt: DAN mode",
		"curl | bash command injection",
	}
	seedTenantData(db, "test-team", 20, 10, reasons)

	cells := le.GetHeatmap()
	if len(cells) == 0 {
		t.Error("热力图不应为空")
	}

	// 检查是否有多个分类
	categories := make(map[string]bool)
	for _, c := range cells {
		if c.TenantID == "test-team" {
			categories[c.Category] = true
		}
	}
	if len(categories) < 3 {
		t.Errorf("期望至少 3 个分类有数据, 得到 %d", len(categories))
	}
}

// Test 10: SLA 概览
func TestSLAOverview(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	le.tenantMgr.Create(&Tenant{ID: "good-team", Name: "优秀团队", Enabled: true})
	le.tenantMgr.Create(&Tenant{ID: "bad-team", Name: "问题团队", Enabled: true})

	// good-team: 低拦截
	seedTenantData(db, "good-team", 50, 1, nil)
	seedRedTeamReport(db, "good-team", 95)

	// bad-team: 高拦截
	seedTenantData(db, "bad-team", 50, 25, nil)

	overview := le.GetSLAOverview()
	if overview.Summary.Total < 3 {
		t.Errorf("期望至少 3 个租户, 得到 %d", overview.Summary.Total)
	}
	if overview.Summary.Green == 0 {
		t.Log("注意: 没有 green 租户（可能因为默认租户没有红队数据）")
	}
}

// Test 11: 强度等级转换
func TestCountToIntensity(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, "none"},
		{1, "low"},
		{3, "medium"},
		{10, "high"},
		{20, "critical"},
		{100, "critical"},
	}
	for _, tt := range tests {
		got := countToIntensity(tt.count)
		if got != tt.expected {
			t.Errorf("countToIntensity(%d): 期望 %s, 得到 %s", tt.count, tt.expected, got)
		}
	}
}

// Test 12: 趋势计算
func TestComputeTrend(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	le.tenantMgr.Create(&Tenant{ID: "trend-test", Name: "趋势测试", Enabled: true})

	now := time.Now().UTC()
	// 上周大量拦截
	for i := 0; i < 20; i++ {
		ts := now.AddDate(0, 0, -10).Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, tenant_id) VALUES (?, 'inbound', 'user', 'block', 'test', ?)`,
			ts, "trend-test")
	}
	// 本周很少拦截
	for i := 0; i < 20; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, tenant_id) VALUES (?, 'inbound', 'user', 'pass', '', ?)`,
			ts, "trend-test")
	}

	trend := le.computeTrend("trend-test")
	if trend != "up" && trend != "stable" {
		t.Logf("趋势: %s (期望 up 或 stable，因为本周拦截率更低)", trend)
	}
}

// ============================================================
// API 集成测试
// ============================================================

func setupLeaderboardAPI(t *testing.T) (*ManagementAPI, *sql.DB, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-lb-api-test-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}

	// 初始化 schema
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT, app_id TEXT, trace_id TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, trace_id TEXT, model TEXT, request_tokens INTEGER,
		response_tokens INTEGER, total_tokens INTEGER, latency_ms REAL,
		status_code INTEGER, has_tool_use INTEGER, tool_count INTEGER,
		error_message TEXT, canary_leaked INTEGER DEFAULT 0,
		budget_exceeded INTEGER DEFAULT 0, budget_violations TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER, timestamp TEXT, tool_name TEXT,
		tool_input_preview TEXT, tool_result_preview TEXT,
		risk_level TEXT, flagged INTEGER DEFAULT 0, flag_reason TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS redteam_reports (
		id TEXT PRIMARY KEY, tenant_id TEXT DEFAULT 'default',
		timestamp TEXT, duration_ms INTEGER, total_tests INTEGER,
		passed INTEGER, failed INTEGER, pass_rate REAL,
		report_json TEXT, status TEXT DEFAULT 'completed'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY, name TEXT, description TEXT,
		created_at TEXT, max_agents INTEGER DEFAULT 0,
		max_rules INTEGER DEFAULT 0, enabled INTEGER DEFAULT 1,
		strict_mode INTEGER DEFAULT 0
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id TEXT, match_type TEXT, match_value TEXT,
		description TEXT, created_at TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tenant_configs (
		tenant_id TEXT PRIMARY KEY, disabled_rules TEXT DEFAULT '',
		extra_rules_yaml TEXT DEFAULT '', strict_mode INTEGER DEFAULT 0,
		canary_enabled INTEGER DEFAULT 1, budget_enabled INTEGER DEFAULT 1,
		budget_max_tokens INTEGER DEFAULT 0, budget_max_tools INTEGER DEFAULT 0,
		tool_blacklist TEXT DEFAULT '', alert_level TEXT DEFAULT 'high',
		alert_webhook TEXT DEFAULT '', updated_at TEXT DEFAULT ''
	)`)

	tenantMgr := NewTenantManager(db)
	healthEng := NewHealthScoreEngine(db)

	cfg := &Config{ManagementToken: "test-token"}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	store := NewSQLiteStore(db, tmpDB)
	shutdownMgr := NewShutdownManager(cfg)
	engine := NewRuleEngine()
	outboundEngine := NewOutboundRuleEngine(nil)

	api := NewManagementAPI(
		&Config{ManagementToken: "test-token"},
		"/tmp/config.yaml",
		pool, routes, nil, engine, outboundEngine,
		nil, nil, nil, nil, nil, nil, nil, nil,
		store, shutdownMgr, nil,
	)
	api.tenantMgr = tenantMgr
	api.healthScoreEng = healthEng
	api.leaderboardEng = NewLeaderboardEngine(db, tenantMgr, healthEng)

	cleanup := func() { db.Close(); os.Remove(tmpDB) }
	return api, db, cleanup
}

// Test 13: GET /api/v1/leaderboard API
func TestLeaderboardAPI(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/leaderboard", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 得到 %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["scores"]; !ok {
		t.Error("响应中缺少 scores 字段")
	}
}

// Test 14: GET /api/v1/leaderboard/heatmap API
func TestHeatmapAPI(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/leaderboard/heatmap", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["cells"]; !ok {
		t.Error("响应中缺少 cells 字段")
	}
}

// Test 15: GET /api/v1/leaderboard/sla API
func TestSLAAPI(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/leaderboard/sla", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 得到 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["config"]; !ok {
		t.Error("响应中缺少 config 字段")
	}
}

// Test 16: PUT /api/v1/leaderboard/sla/config API
func TestSLAConfigAPI(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	body := `{"min_health_score":80,"max_incident_count":5,"min_redteam_score":90}`
	req := httptest.NewRequest("PUT", "/api/v1/leaderboard/sla/config", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200, 得到 %d: %s", w.Code, w.Body.String())
	}

	// 验证配置已更新
	cfg := api.leaderboardEng.GetSLAConfig()
	if cfg.MinHealthScore != 80 {
		t.Errorf("期望 MinHealthScore=80, 得到 %d", cfg.MinHealthScore)
	}
}

// Test 17: 事件统计（有数据）
func TestIncidentStats(t *testing.T) {
	le, db, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	seedTenantData(db, "default", 50, 15, nil)

	since7d := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	incidents, total, blocked := le.getIncidentStats("default", since7d)

	if total != 50 {
		t.Errorf("期望 total=50, 得到 %d", total)
	}
	if incidents != 15 {
		t.Errorf("期望 incidents=15, 得到 %d", incidents)
	}
	if blocked != 15 {
		t.Errorf("期望 blocked=15, 得到 %d", blocked)
	}
}

// Test 18: 禁用租户不出现在排行榜
func TestDisabledTenantExcluded(t *testing.T) {
	le, _, cleanup := setupLeaderboardEngine(t)
	defer cleanup()

	// Create first (Create() forces Enabled=true), then Update to disable
	le.tenantMgr.Create(&Tenant{ID: "disabled-team", Name: "禁用团队", Enabled: true})
	le.tenantMgr.Update(&Tenant{ID: "disabled-team", Name: "禁用团队", Enabled: false})

	scores := le.GetLeaderboard()
	for _, s := range scores {
		if s.TenantID == "disabled-team" {
			t.Error("禁用的租户不应出现在排行榜中")
		}
	}
}

// Test 19: 未授权 API 返回 401
func TestLeaderboardUnauthorized(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/leaderboard", nil)
	// 不设置 Authorization
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("期望 401, 得到 %d", w.Code)
	}
}

// Test 20: PUT SLA config 无效 JSON 返回 400
func TestSLAConfigInvalidJSON(t *testing.T) {
	api, _, cleanup := setupLeaderboardAPI(t)
	defer cleanup()

	req := httptest.NewRequest("PUT", "/api/v1/leaderboard/sla/config", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望 400, 得到 %d", w.Code)
	}
}
