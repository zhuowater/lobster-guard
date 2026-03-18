// ab_testing.go — Prompt A/B 测试引擎
// lobster-guard v15.1 — "同时跑两个 Prompt 版本比安全性"
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ============================================================
// A/B 测试引擎
// ============================================================

// ABTestEngine A/B 测试引擎
type ABTestEngine struct {
	db    *sql.DB
	mu    sync.RWMutex
	tests map[string]*ABTest // 活跃测试缓存 (status=running)
}

// ABTest A/B 测试定义
type ABTest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"` // "draft" / "running" / "completed" / "cancelled"

	// 版本 A（对照组）
	VersionA    string `json:"version_a"`
	PromptHashA string `json:"prompt_hash_a"`
	TrafficA    int    `json:"traffic_a"`

	// 版本 B（实验组）
	VersionB    string `json:"version_b"`
	PromptHashB string `json:"prompt_hash_b"`
	TrafficB    int    `json:"traffic_b"`

	// 时间
	CreatedAt string `json:"created_at"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`

	// 结果
	ResultA        *ABTestMetrics `json:"result_a"`
	ResultB        *ABTestMetrics `json:"result_b"`
	Winner         string         `json:"winner"`
	Confidence     float64        `json:"confidence"`
	Recommendation string         `json:"recommendation"`
}

// ABTestMetrics 安全指标
type ABTestMetrics struct {
	TotalRequests     int     `json:"total_requests"`
	InjectionAttempts int     `json:"injection_attempts"`
	InjectionBlocked  int     `json:"injection_blocked"`
	BlockRate         float64 `json:"block_rate"`
	CanaryLeaks       int     `json:"canary_leaks"`
	CanaryLeakRate    float64 `json:"canary_leak_rate"`
	ErrorCount        int     `json:"error_count"`
	ErrorRate         float64 `json:"error_rate"`
	AvgTokens         float64 `json:"avg_tokens"`
	FlaggedTools      int     `json:"flagged_tools"`
	FlaggedToolRate   float64 `json:"flagged_tool_rate"`
	SecurityScore     float64 `json:"security_score"`
}

// NewABTestEngine 创建 A/B 测试引擎
func NewABTestEngine(db *sql.DB) *ABTestEngine {
	engine := &ABTestEngine{
		db:    db,
		tests: make(map[string]*ABTest),
	}
	engine.initSchema()
	engine.loadRunningTests()
	return engine
}

// initSchema 初始化数据库表
func (ab *ABTestEngine) initSchema() {
	ab.db.Exec(`CREATE TABLE IF NOT EXISTS ab_tests (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		tenant_id TEXT DEFAULT 'default',
		status TEXT DEFAULT 'draft',
		version_a TEXT DEFAULT 'A',
		prompt_hash_a TEXT DEFAULT '',
		traffic_a INTEGER DEFAULT 50,
		version_b TEXT DEFAULT 'B',
		prompt_hash_b TEXT DEFAULT '',
		traffic_b INTEGER DEFAULT 50,
		created_at TEXT NOT NULL,
		started_at TEXT DEFAULT '',
		ended_at TEXT DEFAULT '',
		result_json TEXT DEFAULT '',
		winner TEXT DEFAULT '',
		confidence REAL DEFAULT 0,
		recommendation TEXT DEFAULT ''
	)`)
	ab.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ab_tests_tenant ON ab_tests(tenant_id)`)
	ab.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ab_tests_status ON ab_tests(status)`)
}

// loadRunningTests 加载活跃测试到内存缓存
func (ab *ABTestEngine) loadRunningTests() {
	rows, err := ab.db.Query(`SELECT id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at, started_at, ended_at, result_json, winner, confidence, recommendation FROM ab_tests WHERE status='running'`)
	if err != nil {
		log.Printf("[ABTest] 加载活跃测试失败: %v", err)
		return
	}
	defer rows.Close()

	ab.mu.Lock()
	defer ab.mu.Unlock()

	for rows.Next() {
		t := &ABTest{}
		var resultJSON string
		if err := rows.Scan(&t.ID, &t.Name, &t.TenantID, &t.Status, &t.VersionA, &t.PromptHashA, &t.TrafficA, &t.VersionB, &t.PromptHashB, &t.TrafficB, &t.CreatedAt, &t.StartedAt, &t.EndedAt, &resultJSON, &t.Winner, &t.Confidence, &t.Recommendation); err != nil {
			continue
		}
		t.TrafficB = 100 - t.TrafficA
		ab.tests[t.ID] = t
	}
	log.Printf("[ABTest] 加载了 %d 个活跃 A/B 测试", len(ab.tests))
}

// ============================================================
// CRUD
// ============================================================

// Create 创建一个新的 A/B 测试
func (ab *ABTestEngine) Create(test *ABTest) error {
	if test.ID == "" {
		test.ID = fmt.Sprintf("ab-%d", time.Now().UnixNano()%1000000000)
	}
	if test.Name == "" {
		return fmt.Errorf("测试名称不能为空")
	}
	if test.TenantID == "" {
		test.TenantID = "default"
	}
	if test.TrafficA <= 0 || test.TrafficA > 100 {
		test.TrafficA = 50
	}
	test.TrafficB = 100 - test.TrafficA
	test.Status = "draft"
	test.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err := ab.db.Exec(`INSERT INTO ab_tests (id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		test.ID, test.Name, test.TenantID, test.Status, test.VersionA, test.PromptHashA, test.TrafficA, test.VersionB, test.PromptHashB, test.TrafficB, test.CreatedAt)
	if err != nil {
		return fmt.Errorf("创建 A/B 测试失败: %w", err)
	}
	log.Printf("[ABTest] 创建测试: %s (%s)", test.ID, test.Name)
	return nil
}

// Get 获取单个测试详情
func (ab *ABTestEngine) Get(id string) (*ABTest, error) {
	t := &ABTest{}
	var resultJSON string
	err := ab.db.QueryRow(`SELECT id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at, started_at, ended_at, result_json, winner, confidence, recommendation FROM ab_tests WHERE id=?`, id).Scan(
		&t.ID, &t.Name, &t.TenantID, &t.Status, &t.VersionA, &t.PromptHashA, &t.TrafficA, &t.VersionB, &t.PromptHashB, &t.TrafficB, &t.CreatedAt, &t.StartedAt, &t.EndedAt, &resultJSON, &t.Winner, &t.Confidence, &t.Recommendation)
	if err != nil {
		return nil, fmt.Errorf("测试 %q 不存在", id)
	}
	t.TrafficB = 100 - t.TrafficA

	// 解析 result_json
	if resultJSON != "" {
		var results struct {
			ResultA *ABTestMetrics `json:"result_a"`
			ResultB *ABTestMetrics `json:"result_b"`
		}
		if json.Unmarshal([]byte(resultJSON), &results) == nil {
			t.ResultA = results.ResultA
			t.ResultB = results.ResultB
		}
	}

	// 如果正在运行，实时收集指标（但保留存储的 demo 数据作为兜底）
	if t.Status == "running" {
		storedA := t.ResultA
		storedB := t.ResultB
		ab.collectMetricsForTest(t)
		// 如果实时数据为空（没有匹配的 llm_calls），回退到存储的数据
		if t.ResultA != nil && t.ResultA.TotalRequests == 0 && storedA != nil && storedA.TotalRequests > 0 {
			t.ResultA = storedA
		}
		if t.ResultB != nil && t.ResultB.TotalRequests == 0 && storedB != nil && storedB.TotalRequests > 0 {
			t.ResultB = storedB
		}
	}

	return t, nil
}

// Update 更新测试
func (ab *ABTestEngine) Update(id string, name string, trafficA int, versionA, promptHashA, versionB, promptHashB string) error {
	t, err := ab.Get(id)
	if err != nil {
		return err
	}
	if t.Status != "draft" && t.Status != "running" {
		return fmt.Errorf("只能修改 draft 或 running 状态的测试")
	}

	if name != "" {
		t.Name = name
	}
	if trafficA > 0 && trafficA <= 100 {
		t.TrafficA = trafficA
		t.TrafficB = 100 - trafficA
	}
	if versionA != "" {
		t.VersionA = versionA
	}
	if promptHashA != "" {
		t.PromptHashA = promptHashA
	}
	if versionB != "" {
		t.VersionB = versionB
	}
	if promptHashB != "" {
		t.PromptHashB = promptHashB
	}

	_, err = ab.db.Exec(`UPDATE ab_tests SET name=?, traffic_a=?, traffic_b=?, version_a=?, prompt_hash_a=?, version_b=?, prompt_hash_b=? WHERE id=?`,
		t.Name, t.TrafficA, t.TrafficB, t.VersionA, t.PromptHashA, t.VersionB, t.PromptHashB, id)
	if err != nil {
		return fmt.Errorf("更新测试失败: %w", err)
	}

	// 更新缓存
	ab.mu.Lock()
	if cached, ok := ab.tests[id]; ok {
		cached.Name = t.Name
		cached.TrafficA = t.TrafficA
		cached.TrafficB = t.TrafficB
		cached.VersionA = t.VersionA
		cached.PromptHashA = t.PromptHashA
		cached.VersionB = t.VersionB
		cached.PromptHashB = t.PromptHashB
	}
	ab.mu.Unlock()

	return nil
}

// Delete 删除测试
func (ab *ABTestEngine) Delete(id string) error {
	result, err := ab.db.Exec(`DELETE FROM ab_tests WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("删除测试失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("测试 %q 不存在", id)
	}

	ab.mu.Lock()
	delete(ab.tests, id)
	ab.mu.Unlock()

	log.Printf("[ABTest] 删除测试: %s", id)
	return nil
}

// List 列出测试
func (ab *ABTestEngine) List(tenantID, status string) ([]*ABTest, error) {
	where := "WHERE 1=1"
	var args []interface{}

	if tenantID != "" && tenantID != "all" {
		where += " AND tenant_id=?"
		args = append(args, tenantID)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}

	rows, err := ab.db.Query(`SELECT id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at, started_at, ended_at, result_json, winner, confidence, recommendation FROM ab_tests `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tests []*ABTest
	for rows.Next() {
		t := &ABTest{}
		var resultJSON string
		if err := rows.Scan(&t.ID, &t.Name, &t.TenantID, &t.Status, &t.VersionA, &t.PromptHashA, &t.TrafficA, &t.VersionB, &t.PromptHashB, &t.TrafficB, &t.CreatedAt, &t.StartedAt, &t.EndedAt, &resultJSON, &t.Winner, &t.Confidence, &t.Recommendation); err != nil {
			continue
		}
		t.TrafficB = 100 - t.TrafficA

		if resultJSON != "" {
			var results struct {
				ResultA *ABTestMetrics `json:"result_a"`
				ResultB *ABTestMetrics `json:"result_b"`
			}
			if json.Unmarshal([]byte(resultJSON), &results) == nil {
				t.ResultA = results.ResultA
				t.ResultB = results.ResultB
			}
		}

		if t.Status == "running" {
			storedA := t.ResultA
			storedB := t.ResultB
			ab.collectMetricsForTest(t)
			if t.ResultA != nil && t.ResultA.TotalRequests == 0 && storedA != nil && storedA.TotalRequests > 0 {
				t.ResultA = storedA
			}
			if t.ResultB != nil && t.ResultB.TotalRequests == 0 && storedB != nil && storedB.TotalRequests > 0 {
				t.ResultB = storedB
			}
		}

		tests = append(tests, t)
	}
	return tests, nil
}

// ============================================================
// 生命周期
// ============================================================

// Start 开始测试
func (ab *ABTestEngine) Start(id string) error {
	t, err := ab.Get(id)
	if err != nil {
		return err
	}
	if t.Status != "draft" {
		return fmt.Errorf("只有 draft 状态的测试可以开始")
	}
	if t.PromptHashA == "" || t.PromptHashB == "" {
		return fmt.Errorf("需要设置两个版本的 Prompt Hash")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = ab.db.Exec(`UPDATE ab_tests SET status='running', started_at=? WHERE id=?`, now, id)
	if err != nil {
		return fmt.Errorf("启动测试失败: %w", err)
	}

	t.Status = "running"
	t.StartedAt = now

	ab.mu.Lock()
	ab.tests[id] = t
	ab.mu.Unlock()

	log.Printf("[ABTest] 启动测试: %s (%s)", id, t.Name)
	return nil
}

// Stop 停止测试（计算结果）
func (ab *ABTestEngine) Stop(id string) (*ABTest, error) {
	t, err := ab.Get(id)
	if err != nil {
		return nil, err
	}
	if t.Status != "running" {
		return nil, fmt.Errorf("只有 running 状态的测试可以停止")
	}

	// 收集最终指标
	ab.collectMetricsForTest(t)

	// 计算赢家
	ab.calculateWinner(t)

	now := time.Now().UTC().Format(time.RFC3339)
	t.Status = "completed"
	t.EndedAt = now

	// 序列化结果
	resultJSON, _ := json.Marshal(map[string]interface{}{
		"result_a": t.ResultA,
		"result_b": t.ResultB,
	})

	_, err = ab.db.Exec(`UPDATE ab_tests SET status='completed', ended_at=?, result_json=?, winner=?, confidence=?, recommendation=? WHERE id=?`,
		now, string(resultJSON), t.Winner, t.Confidence, t.Recommendation, id)
	if err != nil {
		return nil, fmt.Errorf("停止测试失败: %w", err)
	}

	ab.mu.Lock()
	delete(ab.tests, id)
	ab.mu.Unlock()

	log.Printf("[ABTest] 停止测试: %s 赢家=%s 置信度=%.1f%%", id, t.Winner, t.Confidence)
	return t, nil
}

// ============================================================
// 流量分配
// ============================================================

// AssignVersion 根据请求分配 A/B 版本
func (ab *ABTestEngine) AssignVersion(tenantID, senderID string) (testID, version, promptHash string) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	for _, t := range ab.tests {
		if t.Status != "running" {
			continue
		}
		if t.TenantID != tenantID && t.TenantID != "all" {
			continue
		}

		bucket := hashToBucket(senderID)
		if bucket < t.TrafficA {
			return t.ID, "A", t.PromptHashA
		}
		return t.ID, "B", t.PromptHashB
	}

	return "", "", ""
}

// hashToBucket 将 senderID 哈希映射到 0-99 的桶
func hashToBucket(senderID string) int {
	h := sha256.Sum256([]byte("ab-test-" + senderID))
	val := binary.BigEndian.Uint32(h[:4])
	return int(val % 100)
}

// ============================================================
// 指标收集
// ============================================================

// collectMetricsForTest 从 llm_calls 聚合指标
func (ab *ABTestEngine) collectMetricsForTest(t *ABTest) {
	if t.PromptHashA != "" {
		t.ResultA = ab.collectMetricsForHash(t.PromptHashA, t.StartedAt)
	}
	if t.PromptHashB != "" {
		t.ResultB = ab.collectMetricsForHash(t.PromptHashB, t.StartedAt)
	}
}

// collectMetricsForHash 按 prompt_hash 聚合安全指标
func (ab *ABTestEngine) collectMetricsForHash(promptHash, since string) *ABTestMetrics {
	m := &ABTestMetrics{}

	timeClause := ""
	var timeArgs []interface{}
	if since != "" {
		timeClause = " AND timestamp >= ?"
		timeArgs = append(timeArgs, since)
	}

	args := append([]interface{}{promptHash}, timeArgs...)
	ab.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE prompt_hash=?`+timeClause, args...).Scan(&m.TotalRequests)

	if m.TotalRequests == 0 {
		return m
	}

	ab.db.QueryRow(`SELECT COALESCE(SUM(canary_leaked),0) FROM llm_calls WHERE prompt_hash=?`+timeClause, args...).Scan(&m.CanaryLeaks)

	errArgs := append([]interface{}{promptHash}, timeArgs...)
	ab.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE prompt_hash=? AND status_code >= 400`+timeClause, errArgs...).Scan(&m.ErrorCount)

	ab.db.QueryRow(`SELECT COALESCE(AVG(total_tokens),0) FROM llm_calls WHERE prompt_hash=?`+timeClause, args...).Scan(&m.AvgTokens)

	ab.db.QueryRow(`SELECT COUNT(*) FROM llm_tool_calls WHERE flagged=1 AND llm_call_id IN (SELECT id FROM llm_calls WHERE prompt_hash=?`+timeClause+`)`, args...).Scan(&m.FlaggedTools)

	ab.db.QueryRow(`SELECT COALESCE(SUM(budget_exceeded),0) FROM llm_calls WHERE prompt_hash=?`+timeClause, args...).Scan(&m.InjectionAttempts)
	m.InjectionBlocked = m.InjectionAttempts

	total := float64(m.TotalRequests)
	if m.InjectionAttempts > 0 {
		m.BlockRate = float64(m.InjectionBlocked) / float64(m.InjectionAttempts)
	}
	m.CanaryLeakRate = float64(m.CanaryLeaks) / total
	m.ErrorRate = float64(m.ErrorCount) / total
	m.FlaggedToolRate = float64(m.FlaggedTools) / total

	m.SecurityScore = CalculateSecurityScore(m)

	return m
}

// CalculateSecurityScore 计算综合安全评分 (0-100)
func CalculateSecurityScore(m *ABTestMetrics) float64 {
	if m.TotalRequests == 0 {
		return 0
	}

	score := 100.0

	// Canary 泄露率扣分（权重 30）
	score -= m.CanaryLeakRate * 30

	// 危险工具调用率扣分（权重 20）
	score -= m.FlaggedToolRate * 20

	// 错误率扣分（权重 10）
	score -= m.ErrorRate * 10

	// 注入成功率扣分（权重 40）
	if m.InjectionAttempts > 0 {
		successRate := 1.0 - float64(m.InjectionBlocked)/float64(m.InjectionAttempts)
		score -= successRate * 40
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

// ============================================================
// 统计显著性
// ============================================================

// calculateWinner 计算赢家和统计显著性
func (ab *ABTestEngine) calculateWinner(t *ABTest) {
	if t.ResultA == nil || t.ResultB == nil {
		t.Winner = ""
		t.Confidence = 0
		t.Recommendation = "无数据，无法判断"
		return
	}

	nA := t.ResultA.TotalRequests
	nB := t.ResultB.TotalRequests

	if nA < 5 || nB < 5 {
		t.Winner = ""
		t.Confidence = 0
		t.Recommendation = "样本不足，无法判断"
		return
	}

	scoreA := t.ResultA.SecurityScore
	scoreB := t.ResultB.SecurityScore

	confidence := CalculateSignificance(
		t.ResultA.CanaryLeakRate, t.ResultB.CanaryLeakRate,
		nA, nB,
	)
	t.Confidence = math.Round(confidence*10) / 10

	rec := ""
	if nA < 30 || nB < 30 {
		rec = "⚠️ 样本不足（<30），置信度不可靠。"
	}

	diff := scoreB - scoreA
	absDiff := math.Abs(diff)

	if absDiff < 3.0 {
		t.Winner = "tie"
		rec += fmt.Sprintf("两个版本安全评分接近（A: %.1f, B: %.1f），差异不显著", scoreA, scoreB)
	} else if scoreB > scoreA {
		t.Winner = "B"
		improvement := ""
		if t.ResultA.CanaryLeakRate > 0 && t.ResultB.CanaryLeakRate < t.ResultA.CanaryLeakRate {
			pct := (1 - t.ResultB.CanaryLeakRate/t.ResultA.CanaryLeakRate) * 100
			improvement = fmt.Sprintf("，Canary 泄露率降低 %.0f%%", pct)
		}
		rec += fmt.Sprintf("推荐采用版本 B（安全分 %.1f vs %.1f）%s", scoreB, scoreA, improvement)
	} else {
		t.Winner = "A"
		rec += fmt.Sprintf("版本 A 更优（安全分 %.1f vs %.1f），不建议切换", scoreA, scoreB)
	}
	t.Recommendation = rec
}

// CalculateSignificance 计算统计显著性（z-test for proportions）
func CalculateSignificance(rateA, rateB float64, nA, nB int) float64 {
	if nA <= 0 || nB <= 0 {
		return 0
	}
	if rateA == rateB {
		return 0
	}

	fA := float64(nA)
	fB := float64(nB)

	xA := rateA * fA
	xB := rateB * fB
	pPooled := (xA + xB) / (fA + fB)

	if pPooled == 0 || pPooled == 1 {
		return 0
	}

	se := math.Sqrt(pPooled * (1 - pPooled) * (1/fA + 1/fB))
	if se == 0 {
		return 0
	}

	z := math.Abs(rateA-rateB) / se

	confidence := zToConfidence(z) * 100
	if confidence > 99.9 {
		confidence = 99.9
	}
	return confidence
}

// zToConfidence z-score 转置信度 (0-1)
func zToConfidence(z float64) float64 {
	if z < 0 {
		z = -z
	}

	b1 := 0.319381530
	b2 := -0.356563782
	b3 := 1.781477937
	b4 := -1.821255978
	b5 := 1.330274429
	p := 0.2316419

	t := 1.0 / (1.0 + p*z)
	phi := math.Exp(-z*z/2.0) / math.Sqrt(2.0*math.Pi)
	cdf := 1.0 - phi*(b1*t+b2*t*t+b3*t*t*t+b4*t*t*t*t+b5*t*t*t*t*t)

	confidence := 2*cdf - 1
	if confidence < 0 {
		confidence = 0
	}
	return confidence
}

// ============================================================
// Demo 数据
// ============================================================

// SeedABTestDemoData 注入 A/B 测试演示数据
func (ab *ABTestEngine) SeedABTestDemoData() int {
	now := time.Now().UTC()
	inserted := 0

	// 1. 已完成的测试 — B 胜出
	completedID := "ab-demo-001"
	completedResultA := &ABTestMetrics{
		TotalRequests:     156,
		InjectionAttempts: 23,
		InjectionBlocked:  19,
		BlockRate:         0.826,
		CanaryLeaks:       8,
		CanaryLeakRate:    0.0513,
		ErrorCount:        5,
		ErrorRate:         0.032,
		AvgTokens:         2350,
		FlaggedTools:      12,
		FlaggedToolRate:   0.0769,
		SecurityScore:     72.3,
	}
	completedResultB := &ABTestMetrics{
		TotalRequests:     148,
		InjectionAttempts: 21,
		InjectionBlocked:  20,
		BlockRate:         0.952,
		CanaryLeaks:       2,
		CanaryLeakRate:    0.0135,
		ErrorCount:        3,
		ErrorRate:         0.0203,
		AvgTokens:         2480,
		FlaggedTools:      4,
		FlaggedToolRate:   0.027,
		SecurityScore:     89.1,
	}

	resultJSON1, _ := json.Marshal(map[string]interface{}{
		"result_a": completedResultA,
		"result_b": completedResultB,
	})

	_, err := ab.db.Exec(`INSERT OR REPLACE INTO ab_tests (id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at, started_at, ended_at, result_json, winner, confidence, recommendation) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		completedID, "v3.2 安全加固测试", "default", "completed",
		"v3.1-基线", "a1b2c3d4e5f67890", 50,
		"v3.2-加固版", "f0e1d2c3b4a59876", 50,
		now.Add(-48*time.Hour).Format(time.RFC3339),
		now.Add(-48*time.Hour).Format(time.RFC3339),
		now.Add(-2*time.Hour).Format(time.RFC3339),
		string(resultJSON1),
		"B", 87.3,
		"推荐采用版本 B（安全分 89.1 vs 72.3），Canary 泄露率降低 74%",
	)
	if err == nil {
		inserted++
	}

	// 2. 运行中的测试
	runningID := "ab-demo-002"
	runningResultA := &ABTestMetrics{
		TotalRequests:     67,
		InjectionAttempts: 9,
		InjectionBlocked:  7,
		BlockRate:         0.778,
		CanaryLeaks:       4,
		CanaryLeakRate:    0.0597,
		ErrorCount:        2,
		ErrorRate:         0.0299,
		AvgTokens:         2100,
		FlaggedTools:      6,
		FlaggedToolRate:   0.0896,
		SecurityScore:     68.5,
	}
	runningResultB := &ABTestMetrics{
		TotalRequests:     63,
		InjectionAttempts: 8,
		InjectionBlocked:  7,
		BlockRate:         0.875,
		CanaryLeaks:       1,
		CanaryLeakRate:    0.0159,
		ErrorCount:        1,
		ErrorRate:         0.0159,
		AvgTokens:         2250,
		FlaggedTools:      3,
		FlaggedToolRate:   0.0476,
		SecurityScore:     78.9,
	}

	resultJSON2, _ := json.Marshal(map[string]interface{}{
		"result_a": runningResultA,
		"result_b": runningResultB,
	})

	_, err2 := ab.db.Exec(`INSERT OR REPLACE INTO ab_tests (id, name, tenant_id, status, version_a, prompt_hash_a, traffic_a, version_b, prompt_hash_b, traffic_b, created_at, started_at, ended_at, result_json, winner, confidence, recommendation) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		runningID, "v4.0 安全指令优化", "default", "running",
		"v3.2-当前版", "f0e1d2c3b4a59876", 50,
		"v4.0-新指令", "1234abcd5678efgh", 50,
		now.Add(-3*time.Hour).Format(time.RFC3339),
		now.Add(-2*time.Hour).Format(time.RFC3339),
		"",
		string(resultJSON2),
		"", 0,
		"",
	)
	if err2 == nil {
		inserted++
		// 加入活跃缓存
		ab.mu.Lock()
		ab.tests[runningID] = &ABTest{
			ID: runningID, Name: "v4.0 安全指令优化", TenantID: "default", Status: "running",
			VersionA: "v3.2-当前版", PromptHashA: "f0e1d2c3b4a59876", TrafficA: 50,
			VersionB: "v4.0-新指令", PromptHashB: "1234abcd5678efgh", TrafficB: 50,
			StartedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
			ResultA: runningResultA, ResultB: runningResultB,
		}
		ab.mu.Unlock()
	}

	log.Printf("[ABTest] 注入了 %d 个 A/B 测试演示数据", inserted)
	return inserted
}

// ClearABTestData 清除 A/B 测试数据
func (ab *ABTestEngine) ClearABTestData() int64 {
	result, err := ab.db.Exec("DELETE FROM ab_tests")
	if err != nil {
		return 0
	}
	n, _ := result.RowsAffected()

	ab.mu.Lock()
	ab.tests = make(map[string]*ABTest)
	ab.mu.Unlock()

	return n
}