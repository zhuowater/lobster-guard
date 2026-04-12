package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionReplayTimelineIncludesSourceClassifier(t *testing.T) {
	tmpDB := t.TempDir() + "/session_replay_source.db"
	db, err := initDB(tmpDB)
	if err != nil {
		t.Fatalf("initDB failed: %v", err)
	}
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil {
		t.Fatalf("NewAuditLogger failed: %v", err)
	}
	defer logger.Close()

	replay := NewSessionReplayEngine(db)
	llmAuditor := NewLLMAuditor(db, LLMAuditConfig{LogToolInput: true, LogToolResult: true, MaxPreviewLen: 512}, &LLMProxyConfig{})

	traceID := "trace-session-source"
	ts := time.Now().UTC().Format(time.RFC3339)
	logger.LogWithTrace("inbound", "user-session", "warn", "source replay", "fetch internal docs", "", 12, "up-1", "app-1", traceID)
	logger.Flush()

	callID, err := llmAuditor.RecordCallWithTenant(ts, traceID, "gpt-4o", 100, 80, 180, 42, 200, true, 1, "", false, false, "req", "resp", "default")
	if err != nil {
		t.Fatalf("RecordCallWithTenant failed: %v", err)
	}
	if _, err := db.Exec(`UPDATE llm_calls SET session_id='sess-source-1', im_trace_id=? WHERE id=?`, traceID, callID); err != nil {
		t.Fatalf("update llm_calls session_id failed: %v", err)
	}
	if err := llmAuditor.RecordToolCallWithSource(callID, ts, "web_fetch", `{"url":"https://docs.example.com/runbook"}`, `{"ok":true}`, &SourceDescriptor{SourceKey: "docs.example.com", BaseTool: "web_fetch", URL: "https://docs.example.com/runbook", Host: "docs.example.com", Category: "internal_api"}); err != nil {
		t.Fatalf("RecordToolCallWithSource failed: %v", err)
	}

	timeline, err := replay.GetTimeline(traceID)
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if len(timeline.Summary.SourceCategories) != 1 || timeline.Summary.SourceCategories[0] != "internal_api" {
		t.Fatalf("expected internal_api summary, got %+v", timeline.Summary.SourceCategories)
	}
	if len(timeline.Summary.SourceKeys) != 1 || timeline.Summary.SourceKeys[0] != "docs.example.com" {
		t.Fatalf("expected docs.example.com source key, got %+v", timeline.Summary.SourceKeys)
	}

	found := false
	for _, ev := range timeline.Events {
		if ev.Type == "tool_call" {
			found = true
			if ev.SourceCategory != "internal_api" {
				t.Fatalf("expected tool source_category internal_api, got %q", ev.SourceCategory)
			}
			if ev.SourceKey != "docs.example.com" {
				t.Fatalf("expected tool source_key docs.example.com, got %q", ev.SourceKey)
			}
			if !strings.Contains(ev.SourceDescriptorJSON, "docs.example.com") {
				t.Fatalf("expected descriptor json to contain docs.example.com, got %q", ev.SourceDescriptorJSON)
			}
		}
	}
	if !found {
		t.Fatal("expected at least one tool_call event")
	}
}

func TestSessionReplayListAPI_SourceCategoryFilter(t *testing.T) {
	tmpDB := t.TempDir() + "/session_replay_source_api.db"
	db, err := initDB(tmpDB)
	if err != nil {
		t.Fatalf("initDB failed: %v", err)
	}
	defer db.Close()

	logger, err := NewAuditLogger(db)
	if err != nil {
		t.Fatalf("NewAuditLogger failed: %v", err)
	}
	defer logger.Close()

	replay := NewSessionReplayEngine(db)
	llmAuditor := NewLLMAuditor(db, LLMAuditConfig{LogToolInput: true, LogToolResult: true, MaxPreviewLen: 512}, &LLMProxyConfig{})

	traceID := "trace-session-filter"
	ts := time.Now().UTC().Format(time.RFC3339)
	logger.LogWithTrace("inbound", "user-api", "pass", "", "fetch metadata", "", 8, "up-1", "app-1", traceID)
	logger.Flush()

	callID, err := llmAuditor.RecordCallWithTenant(ts, traceID, "gpt-4o", 10, 20, 30, 11, 200, true, 1, "", false, false, "req", "resp", "default")
	if err != nil {
		t.Fatalf("RecordCallWithTenant failed: %v", err)
	}
	if _, err := db.Exec(`UPDATE llm_calls SET session_id='sess-filter-1', im_trace_id=? WHERE id=?`, traceID, callID); err != nil {
		t.Fatalf("update llm_calls session_id failed: %v", err)
	}
	if err := llmAuditor.RecordToolCallWithSource(callID, ts, "http_get", `{"url":"http://169.254.169.254/latest/meta-data"}`, `{"ok":true}`, &SourceDescriptor{SourceKey: "169.254.169.254", BaseTool: "http_get", URL: "http://169.254.169.254/latest/meta-data", Host: "169.254.169.254", Category: "metadata_service"}); err != nil {
		t.Fatalf("RecordToolCallWithSource failed: %v", err)
	}

	api := &ManagementAPI{logger: logger, sessionReplayEng: replay, cfg: &Config{}}
	req := httptest.NewRequest("GET", "/api/v1/sessions/replay?source_category=metadata_service", nil)
	w := httptest.NewRecorder()
	api.handleSessionReplayList(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if result["source_category"] != "metadata_service" {
		t.Fatalf("expected echoed source_category metadata_service, got %v", result["source_category"])
	}
	sessions, ok := result["sessions"].([]interface{})
	if !ok || len(sessions) != 1 {
		t.Fatalf("expected exactly 1 session, got %#v", result["sessions"])
	}
}
