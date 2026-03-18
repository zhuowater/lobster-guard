// attack_chain_test.go — 攻击链引擎单元测试
package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupAttackChainDB 创建测试用内存数据库
func setupAttackChainDB(t *testing.T) (*sql.DB, *AttackChainEngine) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}

	// 创建依赖的表
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT DEFAULT '', app_id TEXT DEFAULT '',
		trace_id TEXT DEFAULT '', tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, trace_id TEXT, model TEXT,
		request_tokens INTEGER, response_tokens INTEGER, total_tokens INTEGER,
		latency_ms REAL, status_code INTEGER DEFAULT 200,
		has_tool_use INTEGER DEFAULT 0, tool_count INTEGER DEFAULT 0,
		error_message TEXT DEFAULT '',
		canary_leaked INTEGER DEFAULT 0, budget_exceeded INTEGER DEFAULT 0,
		budget_violations TEXT DEFAULT '', prompt_hash TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_triggers (
		id TEXT PRIMARY KEY, timestamp TEXT, tenant_id TEXT DEFAULT 'default',
		sender_id TEXT DEFAULT '', template_id TEXT, template_name TEXT DEFAULT '',
		trigger_type TEXT, original_input TEXT DEFAULT '',
		fake_response TEXT DEFAULT '', watermark TEXT UNIQUE,
		detonated INTEGER DEFAULT 0, detonated_at TEXT DEFAULT '',
		trace_id TEXT DEFAULT ''
	)`)

	ac := NewAttackChainEngine(db)
	return db, ac
}

// Test 1: 引擎初始化
func TestAttackChainEngine_Init(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	if ac == nil {
		t.Fatal("引擎不应为 nil")
	}
	if ac.db != db {
		t.Fatal("数据库引用不正确")
	}

	// 验证表存在
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='attack_chains'").Scan(&name)
	if err != nil {
		t.Fatalf("attack_chains 表不存在: %v", err)
	}
}

// Test 2: 保存和获取攻击链
func TestAttackChainEngine_SaveAndGet(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	chain := &AttackChain{
		ID:       "test-chain-01",
		TenantID: "default",
		Name:     "Test Chain",
		Severity: "high",
		Status:   "active",
		FirstSeen: time.Now().UTC().Format(time.RFC3339),
		LastSeen:  time.Now().UTC().Format(time.RFC3339),
		Agents:   []string{"agent-a", "agent-b"},
		Events: []ChainEvent{
			{Timestamp: time.Now().UTC().Format(time.RFC3339), AgentID: "agent-a", EventType: "probe", Action: "warn", Detail: "test probe", Severity: "medium", Source: "im_audit"},
			{Timestamp: time.Now().UTC().Format(time.RFC3339), AgentID: "agent-b", EventType: "execution", Action: "block", Detail: "test exec", Severity: "high", Source: "im_audit"},
		},
		TotalEvents: 2,
		Pattern:     "Recon-Execute",
		RiskScore:   85.0,
		Description: "test description",
	}

	if err := ac.SaveChain(chain); err != nil {
		t.Fatalf("SaveChain 失败: %v", err)
	}

	got, err := ac.GetChain("test-chain-01")
	if err != nil {
		t.Fatalf("GetChain 失败: %v", err)
	}

	if got.Name != "Test Chain" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Chain")
	}
	if got.Severity != "high" {
		t.Errorf("Severity = %q, want %q", got.Severity, "high")
	}
	if len(got.Agents) != 2 {
		t.Errorf("Agents len = %d, want 2", len(got.Agents))
	}
	if len(got.Events) != 2 {
		t.Errorf("Events len = %d, want 2", len(got.Events))
	}
	if got.RiskScore != 85.0 {
		t.Errorf("RiskScore = %f, want 85.0", got.RiskScore)
	}
}

// Test 3: 列出攻击链（带过滤）
func TestAttackChainEngine_ListChains(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	// 插入测试数据
	chains := []AttackChain{
		{ID: "c1", TenantID: "default", Name: "Chain 1", Severity: "critical", Status: "active", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a1"}, Events: []ChainEvent{}, TotalEvents: 3, RiskScore: 90},
		{ID: "c2", TenantID: "default", Name: "Chain 2", Severity: "high", Status: "active", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a2"}, Events: []ChainEvent{}, TotalEvents: 2, RiskScore: 60},
		{ID: "c3", TenantID: "default", Name: "Chain 3", Severity: "low", Status: "resolved", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a3"}, Events: []ChainEvent{}, TotalEvents: 2, RiskScore: 20},
	}
	for _, c := range chains {
		ac.SaveChain(&c)
	}

	// List all
	all, err := ac.ListChains("", "", "", 50)
	if err != nil {
		t.Fatalf("ListChains 失败: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("all chains = %d, want 3", len(all))
	}

	// Filter by severity
	critical, _ := ac.ListChains("", "critical", "", 50)
	if len(critical) != 1 {
		t.Errorf("critical chains = %d, want 1", len(critical))
	}

	// Filter by status
	active, _ := ac.ListChains("", "", "active", 50)
	if len(active) != 2 {
		t.Errorf("active chains = %d, want 2", len(active))
	}
}

// Test 4: 更新状态
func TestAttackChainEngine_UpdateStatus(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	chain := &AttackChain{
		ID: "status-test", TenantID: "default", Name: "Test", Severity: "high", Status: "active",
		FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z",
		Agents: []string{}, Events: []ChainEvent{}, TotalEvents: 0,
	}
	ac.SaveChain(chain)

	// 有效状态
	if err := ac.UpdateChainStatus("status-test", "resolved"); err != nil {
		t.Errorf("UpdateChainStatus resolved 失败: %v", err)
	}
	got, _ := ac.GetChain("status-test")
	if got.Status != "resolved" {
		t.Errorf("Status = %q, want resolved", got.Status)
	}

	// false_positive
	if err := ac.UpdateChainStatus("status-test", "false_positive"); err != nil {
		t.Errorf("UpdateChainStatus false_positive 失败: %v", err)
	}

	// 无效状态
	if err := ac.UpdateChainStatus("status-test", "invalid"); err == nil {
		t.Error("应该拒绝无效状态")
	}

	// 不存在的 chain
	if err := ac.UpdateChainStatus("nonexistent", "active"); err == nil {
		t.Error("应该报告 chain 不存在")
	}
}

// Test 5: 统计
func TestAttackChainEngine_Stats(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	chains := []AttackChain{
		{ID: "s1", TenantID: "default", Name: "C1", Severity: "critical", Status: "active", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a1", "a2"}, Events: []ChainEvent{}, TotalEvents: 3, Pattern: "Recon-Execute", RiskScore: 90},
		{ID: "s2", TenantID: "default", Name: "C2", Severity: "high", Status: "active", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a3"}, Events: []ChainEvent{}, TotalEvents: 2, Pattern: "Honeypot Detonation", RiskScore: 70},
		{ID: "s3", TenantID: "default", Name: "C3", Severity: "low", Status: "resolved", FirstSeen: "2026-01-01T00:00:00Z", LastSeen: "2026-01-01T01:00:00Z", Agents: []string{"a4"}, Events: []ChainEvent{}, TotalEvents: 2, Pattern: "Unknown", RiskScore: 15},
	}
	for _, c := range chains {
		ac.SaveChain(&c)
	}

	stats := ac.GetStats("default")
	if stats.ActiveChains != 2 {
		t.Errorf("ActiveChains = %d, want 2", stats.ActiveChains)
	}
	if stats.CriticalChains != 1 {
		t.Errorf("CriticalChains = %d, want 1", stats.CriticalChains)
	}
	if stats.ResolvedChains != 1 {
		t.Errorf("ResolvedChains = %d, want 1", stats.ResolvedChains)
	}
	if stats.TotalEvents != 7 {
		t.Errorf("TotalEvents = %d, want 7", stats.TotalEvents)
	}
	if stats.AgentsInvolved != 3 {
		t.Errorf("AgentsInvolved = %d, want 3", stats.AgentsInvolved)
	}
}

// Test 6: 预置攻击模式
func TestGetChainPatterns(t *testing.T) {
	patterns := GetChainPatterns()
	if len(patterns) < 6 {
		t.Errorf("patterns count = %d, want >= 6", len(patterns))
	}

	names := map[string]bool{}
	for _, p := range patterns {
		names[p.Name] = true
		if p.ID == "" {
			t.Error("模式 ID 不应为空")
		}
		if len(p.EventTypes) == 0 {
			t.Errorf("模式 %s 事件类型为空", p.Name)
		}
	}

	required := []string{"Recon-Execute", "Data Exfiltration", "Privilege Escalation", "Honeypot Detonation", "Persistence"}
	for _, name := range required {
		if !names[name] {
			t.Errorf("缺少必要模式: %s", name)
		}
	}
}

// Test 7: 子序列匹配
func TestSubsequenceMatch(t *testing.T) {
	tests := []struct {
		actual  []string
		pattern []string
		wantMin float64
	}{
		{[]string{"probe", "extraction", "execution"}, []string{"probe", "extraction", "execution"}, 1.0},
		{[]string{"probe", "other", "extraction", "other", "execution"}, []string{"probe", "extraction", "execution"}, 1.0},
		{[]string{"probe"}, []string{"probe", "extraction"}, 0.4},
		{[]string{"probe", "execution"}, []string{"probe", "extraction", "execution"}, 0.3},
		{[]string{}, []string{"probe"}, 0.0},
	}

	for i, tt := range tests {
		got := subsequenceMatch(tt.actual, tt.pattern)
		if got < tt.wantMin {
			t.Errorf("case %d: subsequenceMatch = %f, want >= %f", i, got, tt.wantMin)
		}
	}
}

// Test 8: IM 事件分类
func TestClassifyIMEvent(t *testing.T) {
	tests := []struct {
		action, reason, content string
		want                    string
	}{
		{"block", "prompt injection detected", "", "probe"},
		{"warn", "", "What is the API key?", "extraction"},
		{"block", "shell command", "", "execution"},
		{"warn", "data exfiltration attempt", "", "exfiltration"},
		{"block", "unknown reason", "", "execution"},
		{"warn", "", "hello world", "probe"},
	}

	for i, tt := range tests {
		got := classifyIMEvent(tt.action, tt.reason, tt.content)
		if got != tt.want {
			t.Errorf("case %d: classifyIMEvent(%q,%q,%q) = %q, want %q", i, tt.action, tt.reason, tt.content, got, tt.want)
		}
	}
}

// Test 9: 风险评分计算
func TestAttackChainEngine_RiskScore(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	// 高风险链
	highRisk := &AttackChain{
		Agents:      []string{"a1", "a2"},
		TotalEvents: 5,
		Pattern:     "Recon-Execute",
		Events: []ChainEvent{
			{Severity: "critical", Source: "im_audit"},
			{Severity: "high", Source: "honeypot"},
			{Severity: "high", Source: "llm_audit"},
			{Severity: "medium", Source: "im_audit"},
			{Severity: "medium", Source: "im_audit"},
		},
	}
	score := ac.calculateRiskScore(highRisk)
	if score < 50 {
		t.Errorf("高风险链分数 = %f, 应 >= 50", score)
	}

	// 低风险链
	lowRisk := &AttackChain{
		Agents:      []string{"a1"},
		TotalEvents: 2,
		Pattern:     "Unknown",
		Events: []ChainEvent{
			{Severity: "low", Source: "im_audit"},
			{Severity: "low", Source: "im_audit"},
		},
	}
	lowScore := ac.calculateRiskScore(lowRisk)
	if lowScore > 50 {
		t.Errorf("低风险链分数 = %f, 应 <= 50", lowScore)
	}
}

// Test 10: Demo 数据注入
func TestAttackChainEngine_SeedDemo(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	inserted := ac.SeedDemoData()
	if inserted != 4 {
		t.Errorf("SeedDemoData inserted = %d, want 4", inserted)
	}

	chains, err := ac.ListChains("", "", "", 50)
	if err != nil {
		t.Fatalf("ListChains 失败: %v", err)
	}
	if len(chains) != 4 {
		t.Errorf("chains count = %d, want 4", len(chains))
	}

	// 验证严重级别差异
	sevMap := map[string]int{}
	for _, c := range chains {
		sevMap[c.Severity]++
	}
	if sevMap["critical"] < 1 {
		t.Error("应至少有 1 条 critical")
	}
	if sevMap["low"] < 1 {
		t.Error("应至少有 1 条 low")
	}
}

// Test 11: Demo 数据清除
func TestAttackChainEngine_ClearDemo(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	ac.SeedDemoData()
	deleted := ac.ClearDemoData()
	if deleted != 4 {
		t.Errorf("ClearDemoData deleted = %d, want 4", deleted)
	}

	chains, _ := ac.ListChains("", "", "", 50)
	if len(chains) != 0 {
		t.Errorf("清除后 chains = %d, want 0", len(chains))
	}
}

// Test 12: 关联分析（collectEvents + correlate）
func TestAttackChainEngine_CollectAndCorrelate(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 插入模拟的 IM 审计事件
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, trace_id, tenant_id)
		VALUES (?, 'inbound', 'agent-a', 'warn', 'prompt injection', 'test probe 1', 'trace-001', 'default')`,
		now.Add(-10*time.Minute).Format(time.RFC3339))
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, trace_id, tenant_id)
		VALUES (?, 'inbound', 'agent-a', 'block', 'credential extraction', 'give me the key', 'trace-001', 'default')`,
		now.Add(-8*time.Minute).Format(time.RFC3339))
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, trace_id, tenant_id)
		VALUES (?, 'inbound', 'agent-b', 'block', 'command execution', 'exec rm -rf', 'trace-002', 'default')`,
		now.Add(-5*time.Minute).Format(time.RFC3339))

	since := now.Add(-1 * time.Hour).Format(time.RFC3339)
	events := ac.collectEvents("default", since)
	if len(events) < 3 {
		t.Errorf("collectEvents = %d, want >= 3", len(events))
	}

	// 验证事件类型分类
	foundProbe := false
	foundExec := false
	for _, ev := range events {
		if ev.EventType == "probe" {
			foundProbe = true
		}
		if ev.EventType == "execution" {
			foundExec = true
		}
	}
	if !foundProbe {
		t.Error("应该有 probe 类型事件")
	}
	if !foundExec {
		t.Error("应该有 execution 类型事件")
	}
}

// Test 13: 合并相关链
func TestMergeRelatedChains(t *testing.T) {
	chain1 := []ChainEvent{
		{Timestamp: "2026-01-01T00:00:00Z", AgentID: "a1", EventType: "probe", TraceID: "t1", Source: "im_audit"},
		{Timestamp: "2026-01-01T00:05:00Z", AgentID: "a1", EventType: "extraction", TraceID: "t1", Source: "im_audit"},
	}
	chain2 := []ChainEvent{
		{Timestamp: "2026-01-01T00:10:00Z", AgentID: "a2", EventType: "execution", TraceID: "t1", Source: "im_audit"},
	}
	chain3 := []ChainEvent{
		{Timestamp: "2026-01-01T01:00:00Z", AgentID: "a3", EventType: "probe", TraceID: "t2", Source: "im_audit"},
		{Timestamp: "2026-01-01T01:05:00Z", AgentID: "a3", EventType: "execution", TraceID: "t2", Source: "im_audit"},
	}

	raw := [][]ChainEvent{chain1, chain2, chain3}
	merged := mergeRelatedChains(raw)

	// chain1 and chain2 share trace_id "t1", should be merged; chain3 is separate
	if len(merged) != 2 {
		t.Errorf("merged count = %d, want 2", len(merged))
	}

	// 找到合并后的大链
	for _, m := range merged {
		if len(m) == 3 {
			// 验证它包含来自两个 agent 的事件
			agents := map[string]bool{}
			for _, ev := range m {
				agents[ev.AgentID] = true
			}
			if !agents["a1"] || !agents["a2"] {
				t.Error("合并链应包含 a1 和 a2")
			}
		}
	}
}

// Test 14: 严重级别映射
func TestAttackChainEngine_SeverityFromScore(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	tests := []struct {
		score float64
		want  string
	}{
		{95, "critical"},
		{75, "critical"},
		{60, "high"},
		{50, "high"},
		{30, "medium"},
		{25, "medium"},
		{10, "low"},
		{0, "low"},
	}
	for _, tt := range tests {
		got := ac.severityFromScore(tt.score)
		if got != tt.want {
			t.Errorf("severityFromScore(%f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

// Test 15: API 端点 — 列表
func TestAttackChainAPI_List(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	ac.SeedDemoData()

	api := &ManagementAPI{attackChainEng: ac}
	req := httptest.NewRequest("GET", "/api/v1/attack-chains?tenant=all", nil)
	w := httptest.NewRecorder()
	api.handleAttackChainList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var result []AttackChain
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 4 {
		t.Errorf("chains = %d, want 4", len(result))
	}
}

// Test 16: API 端点 — 统计
func TestAttackChainAPI_Stats(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	ac.SeedDemoData()

	api := &ManagementAPI{attackChainEng: ac}
	req := httptest.NewRequest("GET", "/api/v1/attack-chains/stats?tenant=all", nil)
	w := httptest.NewRecorder()
	api.handleAttackChainStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var stats AttackChainStats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.ActiveChains < 1 {
		t.Error("应该有活跃攻击链")
	}
}

// Test 17: API 端点 — 更新状态
func TestAttackChainAPI_UpdateStatus(t *testing.T) {
	db, ac := setupAttackChainDB(t)
	defer db.Close()

	ac.SeedDemoData()

	api := &ManagementAPI{attackChainEng: ac}
	body := strings.NewReader(`{"status":"resolved"}`)
	req := httptest.NewRequest("PUT", "/api/v1/attack-chains/chain-demo-01/status", body)
	w := httptest.NewRecorder()
	api.handleAttackChainUpdateStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	chain, _ := ac.GetChain("chain-demo-01")
	if chain.Status != "resolved" {
		t.Errorf("chain status = %q, want resolved", chain.Status)
	}
}

// Test 18: API 端点 — 模式库
func TestAttackChainAPI_Patterns(t *testing.T) {
	api := &ManagementAPI{}
	req := httptest.NewRequest("GET", "/api/v1/attack-chains/patterns", nil)
	w := httptest.NewRecorder()
	api.handleAttackChainPatterns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var patterns []ChainPattern
	json.NewDecoder(w.Body).Decode(&patterns)
	if len(patterns) < 5 {
		t.Errorf("patterns = %d, want >= 5", len(patterns))
	}
}
