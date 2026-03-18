// api_v10_test.go — v10.0~v11.2 新增 API handler 测试
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

// setupV10API 创建包含 v10~v11 新模块的测试 API
func setupV10API(t *testing.T) (*ManagementAPI, *sql.DB, func()) {
	t.Helper()
	tmpDB := "/tmp/lobster-guard-test-v10-" + fmt.Sprintf("%d", time.Now().UnixNano()) + ".db"
	db, err := initDB(tmpDB)
	if err != nil {
		t.Fatal(err)
	}
	// 确保 llm_calls 有扩展列
	db.Exec(`ALTER TABLE llm_calls ADD COLUMN canary_leaked INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE llm_calls ADD COLUMN budget_exceeded INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE llm_calls ADD COLUMN budget_violations TEXT`)

	cfg := &Config{
		StaticUpstreams:       []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		ManagementToken:       "test-token",
		RegistrationToken:     "reg-token",
		HeartbeatIntervalSec:  10,
		HeartbeatTimeoutCount: 3,
	}

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	outEngine := NewOutboundRuleEngine(nil)
	engine := NewRuleEngine()
	channel := NewGenericPlugin("", "")
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, nil, nil, nil, nil, nil)

	llmRuleEngine := NewLLMRuleEngine(defaultLLMRules)
	healthScoreEng := NewHealthScoreEngine(db)
	owaspMatrixEng := NewOWASPMatrixEngine(db, llmRuleEngine)
	strictMode := NewStrictModeManager(engine, llmRuleEngine)
	notificationEng := NewNotificationEngine(db)
	anomalyDetector := NewAnomalyDetector(db)
	userProfileEng := NewUserProfileEngine(db)

	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	api.llmRuleEngine = llmRuleEngine
	api.healthScoreEng = healthScoreEng
	api.owaspMatrixEng = owaspMatrixEng
	api.strictMode = strictMode
	api.notificationEng = notificationEng
	api.anomalyDetector = anomalyDetector
	api.userProfileEng = userProfileEng

	cleanup := func() {
		logger.Close()
		db.Close()
		os.Remove(tmpDB)
	}
	return api, db, cleanup
}

// doRequest 辅助函数
func doRequest(api *ManagementAPI, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func TestAPI_HealthScore(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/health/score", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/health/score 期望200，实际 %d: %s", rec.Code, rec.Body.String())
	}

	var resp HealthScoreResult
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Score < 0 || resp.Score > 100 {
		t.Errorf("分数应在[0,100]，实际 %d", resp.Score)
	}
	if resp.Level == "" {
		t.Error("等级不应为空")
	}
	if len(resp.Trend) != 7 {
		t.Errorf("趋势应有7天，实际 %d", len(resp.Trend))
	}
}

func TestAPI_OWASPMatrix(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/llm/owasp-matrix", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/llm/owasp-matrix 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 10 {
		t.Errorf("OWASP矩阵应有10项，实际 %v", total)
	}
	items, _ := resp["items"].([]interface{})
	if len(items) != 10 {
		t.Errorf("items 应有10项，实际 %d", len(items))
	}
}

func TestAPI_StrictModeGetSet(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	// GET — 默认关闭
	rec := doRequest(api, "GET", "/api/v1/system/strict-mode", "")
	if rec.Code != 200 {
		t.Fatalf("GET strict-mode 期望200，实际 %d", rec.Code)
	}
	var getResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &getResp)
	if getResp["enabled"] != false {
		t.Error("默认应为 disabled")
	}

	// POST — 开启
	rec = doRequest(api, "POST", "/api/v1/system/strict-mode", `{"enabled":true}`)
	if rec.Code != 200 {
		t.Fatalf("POST strict-mode 期望200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var setResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &setResp)
	if setResp["enabled"] != true {
		t.Error("设置后应为 enabled")
	}

	// GET — 确认已开启
	rec = doRequest(api, "GET", "/api/v1/system/strict-mode", "")
	json.Unmarshal(rec.Body.Bytes(), &getResp)
	if getResp["enabled"] != true {
		t.Error("二次确认应为 enabled")
	}

	// POST — 关闭
	rec = doRequest(api, "POST", "/api/v1/system/strict-mode", `{"enabled":false}`)
	if rec.Code != 200 {
		t.Fatalf("POST strict-mode off 期望200，实际 %d", rec.Code)
	}
}

func TestAPI_Notifications(t *testing.T) {
	api, db, cleanup := setupV10API(t)
	defer cleanup()

	// 插入一些通知数据
	now := time.Now().UTC()
	ts := now.Add(-1 * time.Hour).Format(time.RFC3339)
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'block', 'injection', '')`, ts)
	db.Exec(`INSERT INTO llm_tool_calls (timestamp, tool_name, risk_level, flagged, flag_reason) VALUES (?, 'exec', 'critical', 1, 'canary_leaked')`, ts)

	rec := doRequest(api, "GET", "/api/v1/notifications", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/notifications 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total < 2 {
		t.Errorf("应有至少2条通知，实际 %v", total)
	}
}

func TestAPI_AnomalyStatus(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/anomaly/status", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/anomaly/status 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["enabled"] != true {
		t.Error("异常检测应为 enabled")
	}
}

func TestAPI_AnomalyBaselines(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	// 注入 demo baselines
	api.anomalyDetector.InjectDemoBaselines()

	rec := doRequest(api, "GET", "/api/v1/anomaly/baselines", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/anomaly/baselines 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 6 {
		t.Errorf("Demo注入后应有6个基线，实际 %v", total)
	}
}

func TestAPI_AnomalyAlerts(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	api.anomalyDetector.InjectDemoAlerts()

	rec := doRequest(api, "GET", "/api/v1/anomaly/alerts", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/anomaly/alerts 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total < 6 {
		t.Errorf("Demo注入后应有>=6条告警，实际 %v", total)
	}
}

func TestAPI_AnomalyMetric(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	api.anomalyDetector.InjectDemoBaselines()

	rec := doRequest(api, "GET", "/api/v1/anomaly/metric/im_requests_per_hour", "")
	if rec.Code != 200 {
		t.Fatalf("GET anomaly metric 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["metric_name"] != "im_requests_per_hour" {
		t.Errorf("metric_name 期望 im_requests_per_hour，实际 %v", resp["metric_name"])
	}
	if resp["baseline"] == nil {
		t.Error("Demo注入后应有基线")
	}
}

func TestAPI_UserRiskTop(t *testing.T) {
	api, db, cleanup := setupV10API(t)
	defer cleanup()

	now := time.Now().UTC()
	for _, uid := range []string{"user-a", "user-b"} {
		for i := 0; i < 10; i++ {
			action := "pass"
			if i < 5 {
				action = "block"
			}
			ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', ?, ?, '', '')`, ts, uid, action)
		}
	}

	rec := doRequest(api, "GET", "/api/v1/users/risk-top", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/users/risk-top 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 2 {
		t.Errorf("应有2个用户，实际 %v", total)
	}
}

func TestAPI_UserRiskStats(t *testing.T) {
	api, db, cleanup := setupV10API(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'pass', '', '')`, ts)
	}

	rec := doRequest(api, "GET", "/api/v1/users/risk-stats", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/users/risk-stats 期望200，实际 %d", rec.Code)
	}

	var resp UserRiskStats
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.TotalUsers != 1 {
		t.Errorf("总用户数应为1，实际 %d", resp.TotalUsers)
	}
}

func TestAPI_UserTimeline(t *testing.T) {
	api, db, cleanup := setupV10API(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-tl', 'pass', '', 'hello')`, ts)
	}

	rec := doRequest(api, "GET", "/api/v1/users/timeline/user-tl", "")
	if rec.Code != 200 {
		t.Fatalf("GET timeline 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != 3 {
		t.Errorf("应有3条时间线事件，实际 %v", total)
	}
}

func TestAPI_UserRiskProfile(t *testing.T) {
	api, db, cleanup := setupV10API(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		action := "pass"
		if i < 3 {
			action = "block"
		}
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-risk', ?, '', '')`, ts, action)
	}

	rec := doRequest(api, "GET", "/api/v1/users/risk/user-risk", "")
	if rec.Code != 200 {
		t.Fatalf("GET risk profile 期望200，实际 %d: %s", rec.Code, rec.Body.String())
	}

	var resp UserRiskProfile
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.UserID != "user-risk" {
		t.Errorf("UserID 应为 user-risk，实际 %s", resp.UserID)
	}
	if resp.TotalRequests != 10 {
		t.Errorf("TotalRequests 应为10，实际 %d", resp.TotalRequests)
	}
}

func TestAPI_UserRiskProfile_NotFound(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/users/risk/nonexistent-user", "")
	if rec.Code != 404 {
		t.Errorf("不存在的用户应返回404，实际 %d", rec.Code)
	}
}

func TestAPI_DemoSeed(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "POST", "/api/v1/demo/seed", "")
	if rec.Code != 200 {
		t.Fatalf("POST /api/v1/demo/seed 期望200，实际 %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Error("demo seed 应返回 ok=true")
	}
	count, _ := resp["count"].(float64)
	if count < 200 {
		t.Errorf("应注入至少200条数据，实际 %v", count)
	}

	// 注入后验证 health score 有数据
	rec2 := doRequest(api, "GET", "/api/v1/health/score", "")
	if rec2.Code != 200 {
		t.Fatalf("demo seed 后 health score 应返回200，实际 %d", rec2.Code)
	}
}

func TestAPI_LLMRulesList(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/llm/rules", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/llm/rules 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total != float64(len(defaultLLMRules)) {
		t.Errorf("规则数应为 %d，实际 %v", len(defaultLLMRules), total)
	}
}

func TestAPI_LLMRulesHits(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	rec := doRequest(api, "GET", "/api/v1/llm/rules/hits", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/llm/rules/hits 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["hits"] == nil {
		t.Error("应返回 hits 字段")
	}
}

func TestAPI_NotificationsWithAnomalyAlerts(t *testing.T) {
	api, _, cleanup := setupV10API(t)
	defer cleanup()

	// 注入异常告警
	api.anomalyDetector.InjectDemoAlerts()

	rec := doRequest(api, "GET", "/api/v1/notifications", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/v1/notifications 期望200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	total, _ := resp["total"].(float64)
	if total < 6 {
		t.Errorf("异常告警注入后通知应>=6，实际 %v", total)
	}

	// 验证包含 anomaly 类型
	items, _ := resp["notifications"].([]interface{})
	hasAnomaly := false
	for _, item := range items {
		m, _ := item.(map[string]interface{})
		if m["type"] == "anomaly" {
			hasAnomaly = true
		}
	}
	if !hasAnomaly {
		t.Error("通知中应包含 anomaly 类型")
	}
}
