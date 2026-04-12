// llm_audit_test.go — LLMAuditor 测试
package main

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestLLMAuditor(t *testing.T) (*LLMAuditor, *sql.DB) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cfg := LLMAuditConfig{
		LogToolInput:  true,
		LogToolResult: true,
		MaxPreviewLen: 500,
	}
	auditor := NewLLMAuditor(db, cfg, nil)
	return auditor, db
}

func TestParseAnthropicResponse_WithToolUse(t *testing.T) {
	body := `{
		"model": "claude-sonnet-4-20250514",
		"content": [
			{"type": "text", "text": "Let me check..."},
			{"type": "tool_use", "name": "exec", "input": {"command": "ls -la"}},
			{"type": "tool_use", "name": "read_file", "input": {"path": "/tmp/foo"}}
		],
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`

	info := ParseAnthropicResponse([]byte(body))
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", info.Model)
	}
	if !info.HasToolUse {
		t.Error("expected HasToolUse = true")
	}
	if info.ToolCount != 2 {
		t.Errorf("ToolCount = %d, want 2", info.ToolCount)
	}
	if len(info.ToolNames) != 2 || info.ToolNames[0] != "exec" || info.ToolNames[1] != "read_file" {
		t.Errorf("ToolNames = %v, want [exec, read_file]", info.ToolNames)
	}
	if info.InputTokens != 100 || info.OutputTokens != 50 {
		t.Errorf("tokens = %d/%d, want 100/50", info.InputTokens, info.OutputTokens)
	}
}

func TestParseAnthropicResponse_NoToolUse(t *testing.T) {
	body := `{
		"model": "claude-sonnet-4-20250514",
		"content": [
			{"type": "text", "text": "Hello world!"}
		],
		"usage": {"input_tokens": 200, "output_tokens": 100}
	}`

	info := ParseAnthropicResponse([]byte(body))
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.HasToolUse {
		t.Error("expected HasToolUse = false")
	}
	if info.ToolCount != 0 {
		t.Errorf("ToolCount = %d, want 0", info.ToolCount)
	}
	if info.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", info.TotalTokens)
	}
}

func TestParseAnthropicResponse_OpenAIStyleToolCallsAndUsageFallback(t *testing.T) {
	body := `{
		"model": "gpt-4.1",
		"choices": [{
			"message": {
				"tool_calls": [{
					"function": {"name": "execute_code", "arguments": "{\"code\":\"print(1)\"}"}
				}]
			}
		}],
		"usage": {"prompt_tokens": 123, "completion_tokens": 45, "total_tokens": 168}
	}`

	info := ParseAnthropicResponse([]byte(body))
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Model != "gpt-4.1" {
		t.Fatalf("model = %q, want gpt-4.1", info.Model)
	}
	if !info.HasToolUse || info.ToolCount != 1 {
		t.Fatalf("expected one tool call, got %+v", info)
	}
	if len(info.ToolNames) != 1 || info.ToolNames[0] != "execute_code" {
		t.Fatalf("unexpected tool names: %v", info.ToolNames)
	}
	if len(info.ToolInputs) != 1 || info.ToolInputs[0] == "" {
		t.Fatalf("unexpected tool inputs: %v", info.ToolInputs)
	}
	if info.InputTokens != 123 || info.OutputTokens != 45 || info.TotalTokens != 168 {
		t.Fatalf("unexpected token counts: in=%d out=%d total=%d", info.InputTokens, info.OutputTokens, info.TotalTokens)
	}
}

func TestParseSSEEvents(t *testing.T) {
	events := `event: message_start
data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":500}}}

event: content_block_start
data: {"type":"content_block_start","content_block":{"type":"tool_use","name":"exec"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"\"ls\"}"}}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":200}}

data: [DONE]
`

	info := ParseSSEEvents([]byte(events))
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", info.Model)
	}
	if !info.HasToolUse {
		t.Error("expected HasToolUse = true")
	}
	if info.ToolCount != 1 {
		t.Errorf("ToolCount = %d, want 1", info.ToolCount)
	}
	if len(info.ToolNames) != 1 || info.ToolNames[0] != "exec" {
		t.Errorf("ToolNames = %v, want [exec]", info.ToolNames)
	}
	if info.InputTokens != 500 || info.OutputTokens != 200 {
		t.Errorf("tokens = %d/%d, want 500/200", info.InputTokens, info.OutputTokens)
	}
}

func TestClassifyToolRisk(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	tests := []struct {
		tool string
		risk string
	}{
		{"exec", "critical"},
		{"shell", "critical"},
		{"bash", "critical"},
		{"run_command", "critical"},
		{"write_file", "high"},
		{"edit_file", "high"},
		{"web_fetch", "high"},
		{"send_email", "high"},
		{"read_file", "medium"},
		{"browser", "medium"},
		{"web_search", "medium"},
		{"some_unknown_tool", "low"},
		{"canvas", "low"},
	}

	for _, tt := range tests {
		got := auditor.ClassifyToolRisk(tt.tool)
		if got != tt.risk {
			t.Errorf("ClassifyToolRisk(%q) = %q, want %q", tt.tool, got, tt.risk)
		}
	}
}

func TestLLMAuditor_RecordAndQuery(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	// Record a call
	callID, err := auditor.RecordCall("2026-01-01T00:00:00Z", "trace-1", "claude-sonnet-4-20250514", 100, 50, 150, 1000.0, 200, true, 2, "", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if callID <= 0 {
		t.Errorf("expected positive callID, got %d", callID)
	}

	// Record tool calls
	auditor.RecordToolCall(callID, "2026-01-01T00:00:00Z", "exec", `{"command":"ls"}`, "")
	auditor.RecordToolCall(callID, "2026-01-01T00:00:00Z", "read_file", `{"path":"/tmp"}`, "")

	// Query calls
	records, total, err := auditor.QueryCalls("", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total calls = %d, want 1", total)
	}
	if len(records) != 1 {
		t.Errorf("records = %d, want 1", len(records))
	}

	// Query tool calls
	toolRecords, toolTotal, err := auditor.QueryToolCalls("", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if toolTotal != 2 {
		t.Errorf("total tool calls = %d, want 2", toolTotal)
	}
	if len(toolRecords) != 2 {
		t.Errorf("tool records = %d, want 2", len(toolRecords))
	}

	// Query by tool name
	toolRecords, toolTotal, err = auditor.QueryToolCalls("exec", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if toolTotal != 1 {
		t.Errorf("exec total = %d, want 1", toolTotal)
	}
}

func TestLLMAuditor_RecordToolCallWithSourcePersistsDescriptor(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	callID, err := auditor.RecordCall("2026-01-01T00:00:00Z", "trace-src", "claude-sonnet-4-20250514", 100, 50, 150, 1000.0, 200, true, 1, "", false, false, "")
	if err != nil {
		t.Fatal(err)
	}

	sourceDesc := &SourceDescriptor{
		SourceKey:       "tool:web_fetch:public_web:docs.python.org",
		BaseTool:        "web_fetch",
		Host:            "docs.python.org",
		Path:            "/3/library/json.html",
		Category:        "public_web",
		Confidentiality: ConfPublic,
		Integrity:       IntegTaint,
		TrustScore:      0.25,
		AuthType:        "none",
	}
	if err := auditor.RecordToolCallWithSource(callID, "2026-01-01T00:00:00Z", "web_fetch", `{"url":"https://docs.python.org/3/library/json.html"}`, "", sourceDesc); err != nil {
		t.Fatal(err)
	}

	toolRecords, toolTotal, err := auditor.QueryToolCalls("web_fetch", "", "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if toolTotal != 1 || len(toolRecords) != 1 {
		t.Fatalf("unexpected tool records: total=%d len=%d", toolTotal, len(toolRecords))
	}
	if toolRecords[0]["source_category"] != "public_web" {
		t.Fatalf("unexpected source category: %#v", toolRecords[0])
	}
	if toolRecords[0]["source_key"] != sourceDesc.SourceKey {
		t.Fatalf("unexpected source key: %#v", toolRecords[0])
	}
	if toolRecords[0]["source_descriptor_json"] == "" {
		t.Fatalf("expected descriptor json, got %#v", toolRecords[0])
	}
}

func TestLLMAuditor_Stats(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	// Seed some data
	callID1, _ := auditor.RecordCall("2026-01-01T00:00:00Z", "t1", "claude-sonnet-4-20250514", 100, 50, 150, 1000, 200, true, 2, "", false, false, "")
	auditor.RecordToolCallWithSource(callID1, "2026-01-01T00:00:00Z", "exec", "", "", &SourceDescriptor{SourceKey: "tool:exec:internal_api", Category: "internal_api"})
	auditor.RecordToolCallWithSource(callID1, "2026-01-01T00:00:00Z", "read_file", "", "", &SourceDescriptor{SourceKey: "tool:read_file:public_web", Category: "public_web"})

	callID2, _ := auditor.RecordCall("2026-01-01T01:00:00Z", "t2", "gpt-4", 200, 100, 300, 2000, 200, false, 0, "", false, false, "")
	_ = callID2

	stats, err := auditor.ToolStats()
	if err != nil {
		t.Fatal(err)
	}

	total, ok := stats["total"].(int)
	if !ok || total != 2 {
		t.Errorf("total = %v, want 2", stats["total"])
	}

	bySource, ok := stats["by_source_category"].([]map[string]interface{})
	if !ok {
		t.Fatalf("by_source_category type = %T, want []map[string]interface{}", stats["by_source_category"])
	}
	if len(bySource) != 2 {
		t.Fatalf("by_source_category len = %d, want 2", len(bySource))
	}

	// Verify JSON serializable
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatal("stats should be JSON serializable:", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal("stats JSON should be parseable:", err)
	}
}

func TestLLMAuditor_Timeline(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	callID, _ := auditor.RecordCall("2026-01-01T00:00:00Z", "t1", "claude-sonnet-4-20250514", 100, 50, 150, 1000, 200, true, 1, "", false, false, "")
	auditor.RecordToolCall(callID, "2026-01-01T00:00:00Z", "exec", "", "")

	timeline, err := auditor.ToolTimeline(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 24 {
		t.Errorf("expected 24 hours, got %d", len(timeline))
	}
	// Check structure
	for _, entry := range timeline {
		if _, ok := entry["hour"]; !ok {
			t.Error("missing 'hour' in timeline entry")
		}
		if _, ok := entry["total"]; !ok {
			t.Error("missing 'total' in timeline entry")
		}
	}
}

func TestLLMAuditor_Overview(t *testing.T) {
	auditor, db := setupTestLLMAuditor(t)
	defer db.Close()

	// Seed data
	auditor.RecordCall("2026-01-01T00:00:00Z", "t1", "claude-sonnet-4-20250514", 100, 50, 150, 1000, 200, false, 0, "", false, false, "")
	auditor.RecordCall("2026-01-01T01:00:00Z", "t2", "claude-sonnet-4-20250514", 200, 100, 300, 2000, 500, false, 0, "server error", false, false, "")

	overview, err := auditor.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if overview["total_calls"] != 2 {
		t.Errorf("total_calls = %v, want 2", overview["total_calls"])
	}

	// JSON serializable
	data, err := json.Marshal(overview)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
}

func TestParseAnthropicResponse_OpenAIFormat(t *testing.T) {
	body := `{
		"choices": [{
			"message": {
				"tool_calls": [
					{"type": "function", "function": {"name": "web_search", "arguments": "{\"query\":\"hello\"}"}}
				]
			}
		}]
	}`

	info := ParseAnthropicResponse([]byte(body))
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if !info.HasToolUse {
		t.Error("expected HasToolUse = true")
	}
	if info.ToolCount != 1 {
		t.Errorf("ToolCount = %d, want 1", info.ToolCount)
	}
	if len(info.ToolNames) != 1 || info.ToolNames[0] != "web_search" {
		t.Errorf("ToolNames = %v, want [web_search]", info.ToolNames)
	}
}
