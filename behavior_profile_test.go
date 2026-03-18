package main

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupBehaviorTestDB 创建测试用的内存数据库
func setupBehaviorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// 创建 audit_log 表
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		direction TEXT,
		sender_id TEXT,
		action TEXT,
		reason TEXT,
		content_preview TEXT,
		full_request_hash TEXT,
		latency_ms REAL,
		upstream_id TEXT,
		app_id TEXT,
		trace_id TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)

	// 创建 llm_calls 表
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		trace_id TEXT,
		model TEXT,
		request_tokens INTEGER,
		response_tokens INTEGER,
		total_tokens INTEGER,
		latency_ms REAL,
		status_code INTEGER,
		has_tool_use INTEGER DEFAULT 0,
		tool_count INTEGER DEFAULT 0,
		error_message TEXT,
		canary_leaked INTEGER DEFAULT 0,
		budget_exceeded INTEGER DEFAULT 0,
		budget_violations TEXT,
		prompt_hash TEXT DEFAULT '',
		tenant_id TEXT DEFAULT 'default'
	)`)

	// 创建 llm_tool_calls 表
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER REFERENCES llm_calls(id),
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_input_preview TEXT,
		tool_result_preview TEXT,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0,
		flag_reason TEXT,
		tenant_id TEXT DEFAULT 'default'
	)`)

	return db
}

// seedTestAgent 注入测试 agent 数据
func seedTestAgent(db *sql.DB, agentID string, traceCount int, tools []string, dayOffset int) {
	now := time.Now().UTC()
	for i := 0; i < traceCount; i++ {
		traceID := fmt.Sprintf("test-trace-%s-%04d", agentID, i)
		ts := now.Add(-time.Duration(dayOffset*24+i) * time.Hour)
		tsStr := ts.Format(time.RFC3339)

		// audit_log
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, full_request_hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,'',?,?,?,?)`,
			tsStr, "inbound", agentID, "pass", "", "test content", 50.0, "upstream-1", "app-test", traceID)

		// llm_calls
		result, _ := db.Exec(`INSERT INTO llm_calls (timestamp, trace_id, model, request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, error_message) VALUES (?,?,?,?,?,?,?,200,1,?,?)`,
			tsStr, traceID, "test-model", 1000, 500, 1500, 800.0, len(tools), "")

		callID, _ := result.LastInsertId()
		for j, tool := range tools {
			toolTs := ts.Add(time.Duration(100*(j+1)) * time.Millisecond).Format(time.RFC3339Nano)
			db.Exec(`INSERT INTO llm_tool_calls (llm_call_id, timestamp, tool_name, tool_input_preview, tool_result_preview, risk_level, flagged, flag_reason) VALUES (?,?,?,?,?,?,?,?)`,
				callID, toolTs, tool, `{"p":"v"}`, `{"r":"ok"}`, "low", 0, "")
		}
	}
}

// ============================================================
// 测试
// ============================================================

func TestNewBehaviorProfileEngine(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)
	if eng == nil {
		t.Fatal("expected non-nil engine")
	}
	if eng.db == nil {
		t.Fatal("expected non-nil db")
	}

	// 验证 behavior_anomalies 表存在
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='behavior_anomalies'`).Scan(&name)
	if err != nil {
		t.Fatalf("behavior_anomalies table not created: %v", err)
	}
}

func TestBuildProfileEmpty(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)
	profile, err := eng.BuildProfile("nonexistent-agent", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.TotalRequests != 0 {
		t.Errorf("expected 0 requests, got %d", profile.TotalRequests)
	}
	if profile.RiskLevel != "normal" {
		t.Errorf("expected normal risk, got %s", profile.RiskLevel)
	}
}

func TestBuildProfileEmptyAgentID(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)
	_, err := eng.BuildProfile("", "default")
	if err == nil {
		t.Fatal("expected error for empty agent_id")
	}
}

func TestBuildProfileWithData(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	seedTestAgent(db, "agent-alpha", 10, []string{"web_search", "read_file", "summarize"}, 0)

	eng := NewBehaviorProfileEngine(db)
	profile, err := eng.BuildProfile("agent-alpha", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.TotalRequests != 10 {
		t.Errorf("expected 10 requests, got %d", profile.TotalRequests)
	}
	if profile.AgentID != "agent-alpha" {
		t.Errorf("expected agent-alpha, got %s", profile.AgentID)
	}
	if len(profile.TypicalTools) == 0 {
		t.Error("expected non-empty typical tools")
	}
	if profile.AvgTokensPerReq <= 0 {
		t.Error("expected positive avg tokens")
	}
}

func TestCalcSequenceRiskScore(t *testing.T) {
	tests := []struct {
		name     string
		seq      []string
		minScore float64
		maxScore float64
	}{
		{"empty", []string{}, 0, 0},
		{"read_only", []string{"read_file", "summarize"}, 0, 5},
		{"with_exec", []string{"exec"}, 35, 45},
		{"data_exfil", []string{"read_file", "curl", "send_email"}, 50, 100},
		{"full_chain", []string{"read_file", "exec", "send_message"}, 60, 100},
		{"pure_search", []string{"web_search", "summarize"}, 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalcSequenceRiskScore(tt.seq)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("CalcSequenceRiskScore(%v) = %.1f, want [%.1f, %.1f]", tt.seq, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestCalcSequenceRiskScoreCap(t *testing.T) {
	// 极端序列：所有高危工具
	seq := []string{"exec", "shell", "bash", "rm", "curl", "send_email"}
	score := CalcSequenceRiskScore(seq)
	if score > 100 {
		t.Errorf("score should be capped at 100, got %.1f", score)
	}
}

func TestExtractPatterns(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	// 注入相同序列的多个 trace
	seedTestAgent(db, "agent-pattern", 5, []string{"search", "read_file", "summarize"}, 0)

	eng := NewBehaviorProfileEngine(db)
	patterns := eng.extractPatterns("agent-pattern", "default")

	if len(patterns) == 0 {
		t.Fatal("expected at least 1 pattern")
	}

	// 所有 trace 都用同样的工具序列
	p := patterns[0]
	if p.Count != 5 {
		t.Errorf("expected pattern count 5, got %d", p.Count)
	}
	if len(p.Sequence) != 3 {
		t.Errorf("expected sequence length 3, got %d", len(p.Sequence))
	}
}

func TestDetectAnomaliesEmptyAgent(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)
	_, err := eng.DetectAnomalies("", "default")
	if err == nil {
		t.Fatal("expected error for empty agent_id")
	}
}

func TestDetectNewTools(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	// 基线: 3天前的数据，只用 search 和 read_file
	seedTestAgent(db, "agent-newtool", 5, []string{"search", "read_file"}, 3)

	// 最近: 加入新工具 exec
	now := time.Now().UTC()
	traceID := "test-trace-agent-newtool-new"
	tsStr := now.Add(-1 * time.Hour).Format(time.RFC3339)
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, full_request_hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,'',?,?,?,?)`,
		tsStr, "inbound", "agent-newtool", "pass", "", "test", 50.0, "upstream-1", "app-test", traceID)
	result, _ := db.Exec(`INSERT INTO llm_calls (timestamp, trace_id, model, request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, error_message) VALUES (?,?,?,?,?,?,?,200,1,1,'')`,
		tsStr, traceID, "test-model", 1000, 500, 1500, 800.0)
	callID, _ := result.LastInsertId()
	db.Exec(`INSERT INTO llm_tool_calls (llm_call_id, timestamp, tool_name, tool_input_preview, tool_result_preview, risk_level, flagged, flag_reason) VALUES (?,?,?,?,?,?,?,?)`,
		callID, tsStr, "exec", `{"cmd":"ls"}`, `ok`, "critical", 1, "高危工具")

	eng := NewBehaviorProfileEngine(db)
	anomalies := eng.detectNewTools("agent-newtool", now.Add(-24*time.Hour).Format(time.RFC3339), now.AddDate(0, 0, -7).Format(time.RFC3339), "default", now)

	found := false
	for _, a := range anomalies {
		if a.Type == "new_tool" && strings.Contains(a.Description, "exec") {
			found = true
			if a.Severity != "high" {
				t.Errorf("exec should be high severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected new_tool anomaly for exec")
	}
}

func TestListProfiles(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	seedTestAgent(db, "agent-a", 5, []string{"search"}, 0)
	seedTestAgent(db, "agent-b", 3, []string{"read_file"}, 0)

	eng := NewBehaviorProfileEngine(db)
	profiles, err := eng.ListProfiles("all")
	if err != nil {
		t.Fatalf("ListProfiles error: %v", err)
	}
	if len(profiles) < 2 {
		t.Errorf("expected at least 2 profiles, got %d", len(profiles))
	}
}

func TestListAnomalies(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)

	// 插入测试突变记录
	now := time.Now().UTC()
	db.Exec(`INSERT INTO behavior_anomalies (id, timestamp, agent_id, tenant_id, type, severity, description, details, trace_id) VALUES (?,?,?,?,?,?,?,?,?)`,
		"test-ba-001", now.Format(time.RFC3339), "agent-test", "default", "new_tool", "high", "test anomaly", "{}", "")
	db.Exec(`INSERT INTO behavior_anomalies (id, timestamp, agent_id, tenant_id, type, severity, description, details, trace_id) VALUES (?,?,?,?,?,?,?,?,?)`,
		"test-ba-002", now.Format(time.RFC3339), "agent-test", "default", "volume_spike", "medium", "test volume", "{}", "")

	anomalies, err := eng.ListAnomalies("default", "", 50)
	if err != nil {
		t.Fatalf("ListAnomalies error: %v", err)
	}
	if len(anomalies) != 2 {
		t.Errorf("expected 2 anomalies, got %d", len(anomalies))
	}

	// 按严重度过滤
	highOnly, _ := eng.ListAnomalies("default", "high", 50)
	if len(highOnly) != 1 {
		t.Errorf("expected 1 high anomaly, got %d", len(highOnly))
	}
}

func TestListAllPatterns(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	seedTestAgent(db, "agent-p1", 3, []string{"search", "read_file"}, 0)
	seedTestAgent(db, "agent-p2", 2, []string{"search", "read_file"}, 0)

	eng := NewBehaviorProfileEngine(db)
	patterns, err := eng.ListAllPatterns("all")
	if err != nil {
		t.Fatalf("ListAllPatterns error: %v", err)
	}
	if len(patterns) == 0 {
		t.Error("expected at least 1 pattern")
	}
	// 两个 agent 用相同序列，合并后 count 应该是 5
	if patterns[0].Count != 5 {
		t.Errorf("expected merged count 5, got %d", patterns[0].Count)
	}
}

func TestScanAndPersist(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	seedTestAgent(db, "agent-persist", 5, []string{"search"}, 0)

	eng := NewBehaviorProfileEngine(db)
	profile, err := eng.ScanAndPersist("agent-persist", "default")
	if err != nil {
		t.Fatalf("ScanAndPersist error: %v", err)
	}
	if profile.AgentID != "agent-persist" {
		t.Errorf("expected agent-persist, got %s", profile.AgentID)
	}
}

func TestSeedBehaviorDemoData(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	eng := NewBehaviorProfileEngine(db)
	profiles, anomalies, patterns := eng.SeedBehaviorDemoData(db)

	if profiles != 5 {
		t.Errorf("expected 5 profiles, got %d", profiles)
	}
	if anomalies < 8 {
		t.Errorf("expected at least 8 anomalies, got %d", anomalies)
	}
	if patterns < 15 {
		t.Errorf("expected at least 15 patterns, got %d", patterns)
	}

	// 验证数据库中有记录
	var aCount int
	db.QueryRow(`SELECT COUNT(*) FROM behavior_anomalies`).Scan(&aCount)
	if aCount < 8 {
		t.Errorf("expected at least 8 behavior_anomalies rows, got %d", aCount)
	}
}

func TestIsHighRiskToolName(t *testing.T) {
	if !isHighRiskToolName("exec") {
		t.Error("exec should be high risk")
	}
	if !isHighRiskToolName("SHELL") {
		t.Error("SHELL should be high risk (case insensitive)")
	}
	if isHighRiskToolName("read_file") {
		t.Error("read_file should NOT be high risk")
	}
	if isHighRiskToolName("web_search") {
		t.Error("web_search should NOT be high risk")
	}
}

func TestGetPeakHours(t *testing.T) {
	db := setupBehaviorTestDB(t)
	defer db.Close()

	seedTestAgent(db, "agent-hours", 10, []string{"search"}, 0)

	eng := NewBehaviorProfileEngine(db)
	hours := eng.getPeakHours("agent-hours")

	if len(hours) == 0 {
		t.Error("expected non-empty peak hours")
	}
	// 验证已排序
	for i := 1; i < len(hours); i++ {
		if hours[i] < hours[i-1] {
			t.Error("peak hours should be sorted")
		}
	}
}
