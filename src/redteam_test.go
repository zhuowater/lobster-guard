// redteam_test.go — v14.2 Red Team Autopilot 测试
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
// 辅助函数
// ============================================================

func setupRedTeamEngine(t *testing.T) (*RedTeamEngine, *sql.DB, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-redteam-test-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	engine := NewRuleEngine()
	rt := NewRedTeamEngine(db, engine)
	cleanup := func() { db.Close(); os.Remove(tmpDB) }
	return rt, db, cleanup
}

func setupMgmtAPIWithRedTeam(t *testing.T) (*ManagementAPI, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-redteam-api-test-%d.db", time.Now().UnixNano())
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
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, nil, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// 注入 RedTeamEngine
	rt := NewRedTeamEngine(db, engine)
	api.redTeamEngine = rt

	cleanup := func() { logger.Close(); db.Close(); os.Remove(tmpDB) }
	return api, cleanup
}

// ============================================================
// 攻击向量测试
// ============================================================

func TestGetAttackVectors(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	vectors := rt.GetAttackVectors()
	if len(vectors) < 33 {
		t.Fatalf("期望至少 33 个攻击向量，实际 %d", len(vectors))
	}
}

func TestAttackVectorCategories(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	vectors := rt.GetAttackVectors()
	categories := make(map[string]int)
	for _, v := range vectors {
		categories[v.Category]++
	}

	expected := []string{
		"prompt_injection",
		"insecure_output",
		"sensitive_info",
		"insecure_plugin",
		"overreliance",
		"model_dos",
	}
	for _, cat := range expected {
		if categories[cat] == 0 {
			t.Fatalf("缺少 OWASP 分类: %s", cat)
		}
	}
	if len(categories) < 6 {
		t.Fatalf("期望至少 6 个分类，实际 %d", len(categories))
	}
}

func TestAttackVectorFields(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	vectors := rt.GetAttackVectors()
	ids := make(map[string]bool)
	for _, v := range vectors {
		if v.ID == "" {
			t.Fatal("攻击向量 ID 不能为空")
		}
		if ids[v.ID] {
			t.Fatalf("重复的攻击向量 ID: %s", v.ID)
		}
		ids[v.ID] = true

		if v.Category == "" {
			t.Fatalf("向量 %s 缺少 Category", v.ID)
		}
		if v.Name == "" {
			t.Fatalf("向量 %s 缺少 Name", v.ID)
		}
		if v.Payload == "" {
			t.Fatalf("向量 %s 缺少 Payload", v.ID)
		}
		if v.Severity == "" {
			t.Fatalf("向量 %s 缺少 Severity", v.ID)
		}
		if v.ExpectedAction == "" {
			t.Fatalf("向量 %s 缺少 ExpectedAction", v.ID)
		}
		// Severity 必须是有效值
		validSeverity := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
		if !validSeverity[v.Severity] {
			t.Fatalf("向量 %s 无效 Severity: %s", v.ID, v.Severity)
		}
		// ExpectedAction 必须是有效值
		validAction := map[string]bool{"pass": true, "warn": true, "block": true}
		if !validAction[v.ExpectedAction] {
			t.Fatalf("向量 %s 无效 ExpectedAction: %s", v.ID, v.ExpectedAction)
		}
	}
}

// ============================================================
// 红队执行测试
// ============================================================

func TestRunRedTeamDefault(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, err := rt.RunAttack("default")
	if err != nil {
		t.Fatalf("运行红队测试失败: %v", err)
	}
	if report == nil {
		t.Fatal("报告不应该为 nil")
	}
	if report.TotalTests == 0 {
		t.Fatal("测试数不应该为 0")
	}
	if report.ID == "" {
		t.Fatal("报告 ID 不应该为空")
	}
	if report.TenantID != "default" {
		t.Fatalf("期望 tenant_id=default，实际 %s", report.TenantID)
	}
}

func TestRunRedTeamResults(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("test-tenant")
	if len(report.Results) != report.TotalTests {
		t.Fatalf("Results 数量 %d != TotalTests %d", len(report.Results), report.TotalTests)
	}

	for _, r := range report.Results {
		if r.VectorID == "" {
			t.Fatal("结果缺少 VectorID")
		}
		if r.Action == "" {
			t.Fatal("结果缺少 Action")
		}
		if r.Expected == "" {
			t.Fatal("结果缺少 Expected")
		}
	}
}

func TestRunRedTeamPassRate(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")
	if report.Passed+report.Failed != report.TotalTests {
		t.Fatalf("Passed(%d) + Failed(%d) != TotalTests(%d)",
			report.Passed, report.Failed, report.TotalTests)
	}

	expectedRate := float64(report.Passed) / float64(report.TotalTests) * 100
	if fmt.Sprintf("%.2f", report.PassRate) != fmt.Sprintf("%.2f", expectedRate) {
		t.Fatalf("检测率计算错误: 期望 %.2f%%，实际 %.2f%%", expectedRate, report.PassRate)
	}
}

func TestRunRedTeamCategoryStats(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")
	if len(report.CategoryStats) == 0 {
		t.Fatal("分类统计不应该为空")
	}

	totalFromStats := 0
	for _, stat := range report.CategoryStats {
		if stat.Category == "" {
			t.Fatal("分类统计缺少 Category")
		}
		if stat.Total == 0 {
			t.Fatalf("分类 %s 总数不应该为 0", stat.Category)
		}
		if stat.Passed+stat.Failed != stat.Total {
			t.Fatalf("分类 %s: Passed(%d)+Failed(%d)!=Total(%d)",
				stat.Category, stat.Passed, stat.Failed, stat.Total)
		}
		totalFromStats += stat.Total
	}
	if totalFromStats != report.TotalTests {
		t.Fatalf("分类统计总数 %d != TotalTests %d", totalFromStats, report.TotalTests)
	}
}

func TestRunRedTeamVulnerabilities(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")
	// 漏洞数应该等于 Failed 数
	if len(report.Vulnerabilities) != report.Failed {
		t.Fatalf("漏洞数 %d != Failed %d", len(report.Vulnerabilities), report.Failed)
	}

	for _, v := range report.Vulnerabilities {
		if v.VectorID == "" {
			t.Fatal("漏洞缺少 VectorID")
		}
		if v.Severity == "" {
			t.Fatal("漏洞缺少 Severity")
		}
		if v.Suggestion == "" {
			t.Fatalf("漏洞 %s 缺少修复建议", v.VectorID)
		}
	}
}

func TestRunRedTeamRecommendations(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")
	if len(report.Recommendations) == 0 {
		t.Fatal("建议列表不应该为空")
	}
}

func TestRunRedTeamEmptyTenantDefaultsToDefault(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("")
	if report.TenantID != "default" {
		t.Fatalf("空 tenant 应该默认为 default，实际 %s", report.TenantID)
	}
}

func TestRunRedTeamNilEngine(t *testing.T) {
	tmpDB := fmt.Sprintf("/tmp/lobster-rt-nil-%d.db", time.Now().UnixNano())
	db, _ := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL")
	defer func() { db.Close(); os.Remove(tmpDB) }()

	rt := NewRedTeamEngine(db, nil)
	_, err := rt.RunAttack("default")
	if err == nil {
		t.Fatal("nil engine 应该返回错误")
	}
}

// ============================================================
// 报告持久化测试
// ============================================================

func TestSaveAndLoadReport(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")

	// 从数据库加载
	loaded, err := rt.GetReport(report.ID)
	if err != nil {
		t.Fatalf("加载报告失败: %v", err)
	}
	if loaded.ID != report.ID {
		t.Fatalf("ID 不匹配: %s != %s", loaded.ID, report.ID)
	}
	if loaded.TotalTests != report.TotalTests {
		t.Fatalf("TotalTests 不匹配: %d != %d", loaded.TotalTests, report.TotalTests)
	}
}

func TestListReports(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	// 运行两次
	rt.RunAttack("tenant-a")
	rt.RunAttack("tenant-b")

	// 列出全部
	reports, err := rt.ListReports("", 10)
	if err != nil {
		t.Fatalf("列出报告失败: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("期望 2 份报告，实际 %d", len(reports))
	}

	// 按租户过滤
	reports, _ = rt.ListReports("tenant-a", 10)
	if len(reports) != 1 {
		t.Fatalf("tenant-a 应该有 1 份报告，实际 %d", len(reports))
	}
}

func TestDeleteReport(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	report, _ := rt.RunAttack("default")

	err := rt.DeleteReport(report.ID)
	if err != nil {
		t.Fatalf("删除报告失败: %v", err)
	}

	_, err = rt.GetReport(report.ID)
	if err == nil {
		t.Fatal("删除后应该查找不到报告")
	}
}

func TestDeleteReportNotFound(t *testing.T) {
	rt, _, cleanup := setupRedTeamEngine(t)
	defer cleanup()

	err := rt.DeleteReport("non-existent-id")
	if err == nil {
		t.Fatal("删除不存在的报告应该失败")
	}
}

// ============================================================
// API 集成测试
// ============================================================

func TestRedTeamRunAPI(t *testing.T) {
	api, cleanup := setupMgmtAPIWithRedTeam(t)
	defer cleanup()

	body := `{"tenant_id":"default"}`
	req := httptest.NewRequest("POST", "/api/v1/redteam/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("红队 API 期望 200，实际 %d，body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["total_tests"] == nil {
		t.Fatal("响应应该包含 total_tests")
	}
}

func TestRedTeamVectorsAPI(t *testing.T) {
	api, cleanup := setupMgmtAPIWithRedTeam(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/redteam/vectors", nil)
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("向量 API 期望 200，实际 %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	vectors := resp["vectors"].([]interface{})
	if len(vectors) < 33 {
		t.Fatalf("期望至少 33 个向量，实际 %d", len(vectors))
	}
}

func TestRedTeamReportsListAPI(t *testing.T) {
	api, cleanup := setupMgmtAPIWithRedTeam(t)
	defer cleanup()

	// 先运行一次
	body := `{"tenant_id":"default"}`
	req := httptest.NewRequest("POST", "/api/v1/redteam/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mgmt-token")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	// 列出报告
	req2 := httptest.NewRequest("GET", "/api/v1/redteam/reports", nil)
	req2.Header.Set("Authorization", "Bearer mgmt-token")
	rec2 := httptest.NewRecorder()
	api.ServeHTTP(rec2, req2)

	if rec2.Code != 200 {
		t.Fatalf("报告列表 API 期望 200，实际 %d", rec2.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec2.Body.Bytes(), &resp)
	reports := resp["reports"].([]interface{})
	if len(reports) < 1 {
		t.Fatalf("应该至少有 1 份报告，实际 %d", len(reports))
	}
}

// ============================================================
// 辅助函数测试
// ============================================================

func TestRtContainsAny(t *testing.T) {
	if !rtContainsAny("Hello World", "hello") {
		t.Fatal("应该不区分大小写匹配 hello")
	}
	if !rtContainsAny("忽略所有指令", "忽略") {
		t.Fatal("应该匹配中文子串")
	}
	if rtContainsAny("safe text", "danger", "attack") {
		t.Fatal("不应该匹配不存在的子串")
	}
}

func TestRtToLower(t *testing.T) {
	if rtToLower("HELLO") != "hello" {
		t.Fatalf("期望 hello，实际 %s", rtToLower("HELLO"))
	}
	if rtToLower("HeLLo WoRLd") != "hello world" {
		t.Fatalf("期望 hello world，实际 %s", rtToLower("HeLLo WoRLd"))
	}
}

func TestRtRepeat(t *testing.T) {
	result := rtRepeat("AB", 3)
	if result != "ABABAB" {
		t.Fatalf("期望 ABABAB，实际 %s", result)
	}
}
