// ab_testing_test.go — A/B 测试引擎单元测试
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupABTestDB 创建测试用内存数据库
func setupABTestDB(t *testing.T) (*sql.DB, *ABTestEngine) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	// 创建 llm_calls 和 llm_tool_calls 表（ABTestEngine 的指标聚合需要）
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, trace_id TEXT, model TEXT,
		request_tokens INTEGER, response_tokens INTEGER, total_tokens INTEGER,
		latency_ms REAL, status_code INTEGER DEFAULT 200,
		has_tool_use INTEGER DEFAULT 0, tool_count INTEGER DEFAULT 0,
		error_message TEXT DEFAULT '',
		canary_leaked INTEGER DEFAULT 0,
		budget_exceeded INTEGER DEFAULT 0,
		budget_violations TEXT DEFAULT '',
		prompt_hash TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER, timestamp TEXT,
		tool_name TEXT, tool_input_preview TEXT, tool_result_preview TEXT,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0, flag_reason TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)

	engine := NewABTestEngine(db)
	return db, engine
}

// TestABTestCreate 测试创建 A/B 测试
func TestABTestCreate(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{
		Name:        "测试创建",
		VersionA:    "v1",
		PromptHashA: "hash-a",
		VersionB:    "v2",
		PromptHashB: "hash-b",
		TrafficA:    60,
	}
	err := engine.Create(test)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if test.ID == "" {
		t.Error("ID 不应为空")
	}
	if test.Status != "draft" {
		t.Errorf("状态应为 draft，实际 %s", test.Status)
	}
	if test.TrafficB != 40 {
		t.Errorf("TrafficB 应为 40，实际 %d", test.TrafficB)
	}
	if test.TenantID != "default" {
		t.Errorf("TenantID 应为 default，实际 %s", test.TenantID)
	}
}

// TestABTestCreateEmptyName 测试创建名称为空的测试
func TestABTestCreateEmptyName(t *testing.T) {
	_, engine := setupABTestDB(t)
	err := engine.Create(&ABTest{})
	if err == nil {
		t.Error("名称为空应返回错误")
	}
}

// TestABTestGet 测试获取测试详情
func TestABTestGet(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "test-get-1", Name: "获取测试", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb", TrafficA: 50}
	engine.Create(test)

	got, err := engine.Get("test-get-1")
	if err != nil {
		t.Fatalf("获取失败: %v", err)
	}
	if got.Name != "获取测试" {
		t.Errorf("名称不匹配: %s", got.Name)
	}
	if got.TrafficB != 50 {
		t.Errorf("TrafficB 应为 50，实际 %d", got.TrafficB)
	}
}

// TestABTestGetNotFound 测试获取不存在的测试
func TestABTestGetNotFound(t *testing.T) {
	_, engine := setupABTestDB(t)
	_, err := engine.Get("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

// TestABTestUpdate 测试更新测试
func TestABTestUpdate(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "test-upd-1", Name: "原始名称", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb", TrafficA: 50}
	engine.Create(test)

	err := engine.Update("test-upd-1", "新名称", 70, "", "", "", "")
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	got, _ := engine.Get("test-upd-1")
	if got.Name != "新名称" {
		t.Errorf("名称未更新: %s", got.Name)
	}
	if got.TrafficA != 70 {
		t.Errorf("TrafficA 应为 70，实际 %d", got.TrafficA)
	}
	if got.TrafficB != 30 {
		t.Errorf("TrafficB 应为 30，实际 %d", got.TrafficB)
	}
}

// TestABTestDelete 测试删除测试
func TestABTestDelete(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "test-del-1", Name: "删除测试", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb"}
	engine.Create(test)

	err := engine.Delete("test-del-1")
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = engine.Get("test-del-1")
	if err == nil {
		t.Error("删除后应获取不到")
	}
}

// TestABTestDeleteNotFound 测试删除不存在的测试
func TestABTestDeleteNotFound(t *testing.T) {
	_, engine := setupABTestDB(t)
	err := engine.Delete("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

// TestABTestList 测试列表
func TestABTestList(t *testing.T) {
	_, engine := setupABTestDB(t)

	engine.Create(&ABTest{ID: "t1", Name: "测试1", TenantID: "default", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb"})
	engine.Create(&ABTest{ID: "t2", Name: "测试2", TenantID: "default", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb"})
	engine.Create(&ABTest{ID: "t3", Name: "测试3", TenantID: "other", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb"})

	// 全部
	all, err := engine.List("all", "")
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("应有 3 个测试，实际 %d", len(all))
	}

	// 按租户
	defaultTests, _ := engine.List("default", "")
	if len(defaultTests) != 2 {
		t.Errorf("default 租户应有 2 个测试，实际 %d", len(defaultTests))
	}

	// 按状态
	drafts, _ := engine.List("all", "draft")
	if len(drafts) != 3 {
		t.Errorf("draft 状态应有 3 个，实际 %d", len(drafts))
	}
}

// TestABTestStartStop 测试开始和停止生命周期
func TestABTestStartStop(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "test-lifecycle", Name: "生命周期测试", VersionA: "A", PromptHashA: "hash-a", VersionB: "B", PromptHashB: "hash-b"}
	engine.Create(test)

	// 开始
	err := engine.Start("test-lifecycle")
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	got, _ := engine.Get("test-lifecycle")
	if got.Status != "running" {
		t.Errorf("状态应为 running，实际 %s", got.Status)
	}
	if got.StartedAt == "" {
		t.Error("StartedAt 不应为空")
	}

	// 不能重复开始
	err = engine.Start("test-lifecycle")
	if err == nil {
		t.Error("运行中不应能再次启动")
	}

	// 停止
	result, err := engine.Stop("test-lifecycle")
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("状态应为 completed，实际 %s", result.Status)
	}
	if result.EndedAt == "" {
		t.Error("EndedAt 不应为空")
	}
}

// TestABTestStartWithoutHash 测试没有 hash 时不能开始
func TestABTestStartWithoutHash(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "no-hash", Name: "无 Hash", VersionA: "A", PromptHashA: "", VersionB: "B", PromptHashB: ""}
	engine.Create(test)

	err := engine.Start("no-hash")
	if err == nil {
		t.Error("没有 hash 应不能启动")
	}
}

// TestAssignVersionConsistency 测试用户粘性（同一 senderID 始终分到同一组）
func TestAssignVersionConsistency(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "assign-test", Name: "分配测试", TenantID: "default", VersionA: "A", PromptHashA: "hash-a", VersionB: "B", PromptHashB: "hash-b", TrafficA: 50}
	engine.Create(test)
	engine.Start("assign-test")

	// 同一用户多次分配应一致
	for i := 0; i < 100; i++ {
		senderID := "user-consistency-test"
		_, v1, _ := engine.AssignVersion("default", senderID)
		_, v2, _ := engine.AssignVersion("default", senderID)
		if v1 != v2 {
			t.Fatalf("第 %d 次: 同一用户分配不一致: %s vs %s", i, v1, v2)
		}
	}
}

// TestAssignVersionDistribution 测试流量分配比例
func TestAssignVersionDistribution(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "dist-test", Name: "分配比例", TenantID: "default", VersionA: "A", PromptHashA: "hash-a", VersionB: "B", PromptHashB: "hash-b", TrafficA: 50}
	engine.Create(test)
	engine.Start("dist-test")

	countA := 0
	countB := 0
	total := 1000

	for i := 0; i < total; i++ {
		senderID := fmt.Sprintf("user-%d", i)
		_, version, _ := engine.AssignVersion("default", senderID)
		if version == "A" {
			countA++
		} else {
			countB++
		}
	}

	// 50/50 比例应该在 40-60% 范围内（允许哈希不均匀）
	pctA := float64(countA) / float64(total) * 100
	if pctA < 35 || pctA > 65 {
		t.Errorf("A 的比例 %.1f%% 偏离 50%% 太远（A=%d, B=%d）", pctA, countA, countB)
	}
}

// TestAssignVersionNoTest 测试没有活跃测试时返回空
func TestAssignVersionNoTest(t *testing.T) {
	_, engine := setupABTestDB(t)

	testID, version, hash := engine.AssignVersion("default", "user-1")
	if testID != "" || version != "" || hash != "" {
		t.Error("没有活跃测试应返回空")
	}
}

// TestAssignVersionWrongTenant 测试不同租户不会分配
func TestAssignVersionWrongTenant(t *testing.T) {
	_, engine := setupABTestDB(t)

	test := &ABTest{ID: "tenant-test", Name: "租户测试", TenantID: "team-a", VersionA: "A", PromptHashA: "ha", VersionB: "B", PromptHashB: "hb"}
	engine.Create(test)
	engine.Start("tenant-test")

	testID, _, _ := engine.AssignVersion("team-b", "user-1")
	if testID != "" {
		t.Error("不同租户不应分配")
	}

	testID, _, _ = engine.AssignVersion("team-a", "user-1")
	if testID == "" {
		t.Error("相同租户应分配")
	}
}

// TestCalculateSecurityScore 测试安全评分计算
func TestCalculateSecurityScore(t *testing.T) {
	// 完美分
	perfect := &ABTestMetrics{
		TotalRequests: 100,
		CanaryLeakRate: 0,
		FlaggedToolRate: 0,
		ErrorRate: 0,
		InjectionAttempts: 10,
		InjectionBlocked: 10,
	}
	score := CalculateSecurityScore(perfect)
	if score != 100.0 {
		t.Errorf("完美指标应得 100 分，实际 %.1f", score)
	}

	// 零请求
	zero := &ABTestMetrics{TotalRequests: 0}
	score = CalculateSecurityScore(zero)
	if score != 0 {
		t.Errorf("零请求应得 0 分，实际 %.1f", score)
	}

	// 有问题的指标
	bad := &ABTestMetrics{
		TotalRequests: 100,
		CanaryLeakRate: 0.1,    // 10% 泄露
		FlaggedToolRate: 0.05,  // 5% 危险工具
		ErrorRate: 0.02,        // 2% 错误
		InjectionAttempts: 20,
		InjectionBlocked: 15,   // 25% 未拦截
	}
	score = CalculateSecurityScore(bad)
	// 100 - 0.1*30 - 0.05*20 - 0.02*10 - 0.25*40 = 100 - 3 - 1 - 0.2 - 10 = 85.8
	expected := 85.8
	if math.Abs(score-expected) > 0.2 {
		t.Errorf("安全评分计算错误: 期望 ~%.1f，实际 %.1f", expected, score)
	}
}

// TestCalculateSignificance 测试统计显著性计算
func TestCalculateSignificance(t *testing.T) {
	// 相同比率 → 0
	c := CalculateSignificance(0.5, 0.5, 100, 100)
	if c != 0 {
		t.Errorf("相同比率应为 0，实际 %.1f", c)
	}

	// 零样本 → 0
	c = CalculateSignificance(0.5, 0.3, 0, 0)
	if c != 0 {
		t.Errorf("零样本应为 0，实际 %.1f", c)
	}

	// 明显差异，大样本 → 高置信度
	c = CalculateSignificance(0.5, 0.1, 1000, 1000)
	if c < 95 {
		t.Errorf("大样本显著差异应有高置信度，实际 %.1f%%", c)
	}

	// 微小差异 → 较低置信度
	c = CalculateSignificance(0.50, 0.49, 100, 100)
	if c > 50 {
		t.Errorf("微小差异不应有高置信度，实际 %.1f%%", c)
	}
}

// TestHashToBucket 测试哈希桶分配
func TestHashToBucket(t *testing.T) {
	// 确定性
	b1 := hashToBucket("user-1")
	b2 := hashToBucket("user-1")
	if b1 != b2 {
		t.Error("相同输入应返回相同桶")
	}

	// 范围
	for i := 0; i < 1000; i++ {
		b := hashToBucket(fmt.Sprintf("user-%d", i))
		if b < 0 || b >= 100 {
			t.Fatalf("桶 %d 超出范围 [0,100)", b)
		}
	}
}

// TestDemoData 测试演示数据注入
func TestDemoData(t *testing.T) {
	_, engine := setupABTestDB(t)
	n := engine.SeedABTestDemoData()
	if n != 2 {
		t.Errorf("应注入 2 个演示测试，实际 %d", n)
	}

	tests, _ := engine.List("all", "")
	if len(tests) != 2 {
		t.Errorf("应有 2 个测试，实际 %d", len(tests))
	}

	// 验证已完成测试
	completed, err := engine.Get("ab-demo-001")
	if err != nil {
		t.Fatalf("获取已完成测试失败: %v", err)
	}
	if completed.Status != "completed" {
		t.Errorf("状态应为 completed，实际 %s", completed.Status)
	}
	if completed.Winner != "B" {
		t.Errorf("赢家应为 B，实际 %s", completed.Winner)
	}

	// 验证运行中测试
	running, err := engine.Get("ab-demo-002")
	if err != nil {
		t.Fatalf("获取运行中测试失败: %v", err)
	}
	if running.Status != "running" {
		t.Errorf("状态应为 running，实际 %s", running.Status)
	}
}

// TestClearData 测试清除数据
func TestClearData(t *testing.T) {
	_, engine := setupABTestDB(t)
	engine.SeedABTestDemoData()

	n := engine.ClearABTestData()
	if n != 2 {
		t.Errorf("应清除 2 条，实际 %d", n)
	}

	tests, _ := engine.List("all", "")
	if len(tests) != 0 {
		t.Errorf("清除后应为空，实际 %d", len(tests))
	}
}

// TestABTestAPIIntegration 测试 API 集成
func TestABTestAPIIntegration(t *testing.T) {
	db, engine := setupABTestDB(t)

	// 需要创建 audit_log 表（ManagementAPI 的一些初始化可能需要）
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT,
		action TEXT, reason TEXT, content_preview TEXT,
		full_request_hash TEXT, latency_ms REAL,
		upstream_id TEXT, app_id TEXT, trace_id TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)

	// 测试 API handler：创建
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/ab-tests" {
			var req ABTest
			if err := decodeJSON(r, &req); err != nil {
				jsonResponse(w, 400, map[string]string{"error": err.Error()})
				return
			}
			if err := engine.Create(&req); err != nil {
				jsonResponse(w, 500, map[string]string{"error": err.Error()})
				return
			}
			jsonResponse(w, 200, req)
		} else if r.Method == "GET" && r.URL.Path == "/api/v1/ab-tests" {
			tests, _ := engine.List("default", "")
			if tests == nil {
				tests = []*ABTest{}
			}
			jsonResponse(w, 200, map[string]interface{}{"tests": tests, "total": len(tests)})
		}
	}

	// 创建测试
	body := `{"name":"API测试","version_a":"A","prompt_hash_a":"ha","version_b":"B","prompt_hash_b":"hb","traffic_a":50}`
	req := httptest.NewRequest("POST", "/api/v1/ab-tests", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != 200 {
		t.Errorf("创建 API 应返回 200，实际 %d: %s", w.Code, w.Body.String())
	}

	// 列表
	req = httptest.NewRequest("GET", "/api/v1/ab-tests", nil)
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != 200 {
		t.Errorf("列表 API 应返回 200，实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "API测试") {
		t.Error("列表应包含刚创建的测试")
	}
}

// decodeJSON 辅助函数
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
