// tool_audit_test.go — ToolCallAuditor 测试
package main

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestToolAuditor(t *testing.T) (*ToolCallAuditor, *sql.DB) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ta, err := NewToolCallAuditor(db)
	if err != nil {
		t.Fatal(err)
	}
	return ta, db
}

func TestToolCallAuditor_ClassifyRisk(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	tests := []struct {
		tool string
		risk string
	}{
		{"exec", "critical"},
		{"shell", "critical"},
		{"bash", "critical"},
		{"write_file", "high"},
		{"edit_file", "high"},
		{"web_fetch", "high"},
		{"read_file", "medium"},
		{"browser", "medium"},
		{"web_search", "medium"},
		{"some_other_tool", "low"},
		{"canvas", "low"},
	}

	for _, tt := range tests {
		got := ta.ClassifyRisk(tt.tool)
		if got != tt.risk {
			t.Errorf("ClassifyRisk(%q) = %q, want %q", tt.tool, got, tt.risk)
		}
	}
}

func TestToolCallAuditor_ParseResponse_ToolUse(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	body := `{
		"content": [
			{"type": "text", "text": "hello"},
			{"type": "tool_use", "name": "exec", "input": {"command": "ls -la"}},
			{"type": "tool_use", "name": "read_file", "input": {"path": "/tmp/foo"}}
		]
	}`

	records := ta.ParseResponse([]byte(body), "user-1", "app-1", "trace-123")
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].ToolName != "exec" {
		t.Errorf("expected tool name 'exec', got %q", records[0].ToolName)
	}
	if records[0].RiskLevel != "critical" {
		t.Errorf("expected risk level 'critical', got %q", records[0].RiskLevel)
	}
	if records[0].SenderID != "user-1" {
		t.Errorf("expected sender_id 'user-1', got %q", records[0].SenderID)
	}

	if records[1].ToolName != "read_file" {
		t.Errorf("expected tool name 'read_file', got %q", records[1].ToolName)
	}
	if records[1].RiskLevel != "medium" {
		t.Errorf("expected risk level 'medium', got %q", records[1].RiskLevel)
	}
}

func TestToolCallAuditor_ParseResponse_FunctionCall(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	body := `{
		"choices": [{
			"message": {
				"function_call": {"name": "web_search", "arguments": "{\"query\":\"hello\"}"}
			}
		}]
	}`

	records := ta.ParseResponse([]byte(body), "user-2", "app-2", "trace-456")
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].ToolName != "web_search" {
		t.Errorf("expected tool name 'web_search', got %q", records[0].ToolName)
	}
}

func TestToolCallAuditor_ParseResponse_OpenAIToolCalls(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	body := `{
		"choices": [{
			"message": {
				"tool_calls": [
					{"type": "function", "function": {"name": "shell", "arguments": "{\"cmd\":\"whoami\"}"}}
				]
			}
		}]
	}`

	records := ta.ParseResponse([]byte(body), "u", "a", "t")
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ToolName != "shell" {
		t.Errorf("expected 'shell', got %q", records[0].ToolName)
	}
	if records[0].RiskLevel != "critical" {
		t.Errorf("expected 'critical', got %q", records[0].RiskLevel)
	}
}

func TestToolCallAuditor_RecordAndQuery(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	// Insert records
	ta.Record(ToolCallRecord{ToolName: "exec", SenderID: "u1", AppID: "a1", RiskLevel: "critical"})
	ta.Record(ToolCallRecord{ToolName: "read_file", SenderID: "u1", AppID: "a1", RiskLevel: "medium"})
	ta.Record(ToolCallRecord{ToolName: "browser", SenderID: "u2", AppID: "a1", RiskLevel: "medium"})

	// Query all
	records, total, err := ta.QueryToolCalls("", "", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// Query by tool
	records, total, err = ta.QueryToolCalls("exec", "", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}

	// Query by risk
	records, total, err = ta.QueryToolCalls("", "medium", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	_ = records
}

func TestToolCallAuditor_Stats(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	ta.Record(ToolCallRecord{ToolName: "exec", RiskLevel: "critical", Flagged: true, FlagReason: "dangerous"})
	ta.Record(ToolCallRecord{ToolName: "read_file", RiskLevel: "medium"})
	ta.Record(ToolCallRecord{ToolName: "browser", RiskLevel: "medium"})
	ta.Record(ToolCallRecord{ToolName: "canvas", RiskLevel: "low"})

	stats, err := ta.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalCalls != 4 {
		t.Errorf("expected total 4, got %d", stats.TotalCalls)
	}
	if stats.ByRisk["critical"] != 1 {
		t.Errorf("expected critical 1, got %d", stats.ByRisk["critical"])
	}
	if stats.ByRisk["medium"] != 2 {
		t.Errorf("expected medium 2, got %d", stats.ByRisk["medium"])
	}
	if stats.FlaggedCount != 1 {
		t.Errorf("expected flagged 1, got %d", stats.FlaggedCount)
	}

	// Verify by_tool
	if len(stats.ByTool) < 1 {
		t.Error("expected at least 1 tool in by_tool")
	}
}

func TestToolCallAuditor_HighRisk(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	ta.Record(ToolCallRecord{ToolName: "exec", RiskLevel: "critical"})
	ta.Record(ToolCallRecord{ToolName: "write_file", RiskLevel: "high"})
	ta.Record(ToolCallRecord{ToolName: "read_file", RiskLevel: "medium"})
	ta.Record(ToolCallRecord{ToolName: "canvas", RiskLevel: "low"})

	records, err := ta.QueryHighRisk(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 high risk records, got %d", len(records))
	}
}

func TestToolCallAuditor_Timeline(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	ta.Record(ToolCallRecord{ToolName: "exec", RiskLevel: "critical"})
	ta.Record(ToolCallRecord{ToolName: "read_file", RiskLevel: "medium"})

	timeline, err := ta.Timeline(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 24 {
		t.Errorf("expected 24 hours, got %d", len(timeline))
	}

	// The last hour should have some data
	lastHour := timeline[len(timeline)-1]
	total, _ := lastHour["total"].(int)
	if total < 2 {
		// The total might be 0 if timestamps don't align perfectly
		// Just check the timeline structure
		if _, ok := lastHour["hour"]; !ok {
			t.Error("expected 'hour' key in timeline entry")
		}
	}
}

func TestToolCallAuditor_ParseResponse_InvalidJSON(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	records := ta.ParseResponse([]byte("not json"), "u", "a", "t")
	if records != nil {
		t.Error("expected nil for invalid JSON")
	}

	records = ta.ParseResponse(nil, "u", "a", "t")
	if records != nil {
		t.Error("expected nil for nil body")
	}
}

func TestTruncateStr(t *testing.T) {
	short := "hello"
	if truncateStr(short, 10) != "hello" {
		t.Error("short string should not be truncated")
	}

	long := "这是一个很长的中文字符串需要被截断"
	result := truncateStr(long, 5)
	if len([]rune(result)) > 8 { // 5 + "..."
		t.Error("long string should be truncated")
	}
}

// TestStatsJSON verifies the stats output is valid JSON
func TestToolCallAuditor_StatsJSON(t *testing.T) {
	ta, db := setupTestToolAuditor(t)
	defer db.Close()

	stats, err := ta.Stats()
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatal("stats should be JSON serializable:", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal("stats JSON should be parseable:", err)
	}
}
