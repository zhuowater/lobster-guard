package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupCFTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func newTestCFVerifier(t *testing.T) (*CounterfactualVerifier, *sql.DB) {
	db := setupCFTestDB(t)
	cfg := CFConfig{
		Enabled:       true,
		Mode:          "sync",
		MaxPerHour:    100,
		RiskThreshold: 50,
		CacheTTLSec:   300,
		TimeoutSec:    10,
		FuzzyMatch:    true,
	}
	v := NewCounterfactualVerifier(db, cfg, nil)
	return v, db
}

// TestCFVerifier_Basic 初始化+配置
func TestCFVerifier_Basic(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	if v == nil {
		t.Fatal("verifier should not be nil")
	}
	cfg := v.GetConfig()
	if !cfg.Enabled {
		t.Error("should be enabled")
	}
	if cfg.Mode != "sync" {
		t.Errorf("mode should be sync, got %s", cfg.Mode)
	}
	if cfg.MaxPerHour != 100 {
		t.Errorf("max_per_hour should be 100, got %d", cfg.MaxPerHour)
	}
}

// TestCFVerifier_ShouldVerify_HighRiskTool
func TestCFVerifier_ShouldVerify_HighRiskTool(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	if !v.ShouldVerify("shell_exec", `{"cmd":"ls"}`, "trace-1", 0) {
		t.Error("should verify high-risk tool shell_exec even with low risk score")
	}
	if !v.ShouldVerify("send_email", `{"to":"a@b.com"}`, "trace-2", 0) {
		t.Error("should verify high-risk tool send_email")
	}
	if !v.ShouldVerify("database_query", `{"sql":"SELECT 1"}`, "trace-3", 0) {
		t.Error("should verify high-risk tool database_query")
	}
}

// TestCFVerifier_ShouldVerify_LowRiskTool
func TestCFVerifier_ShouldVerify_LowRiskTool(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	if v.ShouldVerify("read_file", `{"path":"/tmp/a"}`, "trace-1", 10) {
		t.Error("should NOT verify low-risk tool with score below threshold")
	}
	if v.ShouldVerify("web_search", `{"q":"hello"}`, "trace-2", 30) {
		t.Error("should NOT verify web_search with score 30 < threshold 50")
	}
}

// TestCFVerifier_ShouldVerify_RiskThreshold
func TestCFVerifier_ShouldVerify_RiskThreshold(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// Non-high-risk tool but above threshold
	if !v.ShouldVerify("custom_tool", `{}`, "trace-1", 80) {
		t.Error("should verify custom_tool when risk score 80 > threshold 50")
	}
	if v.ShouldVerify("custom_tool", `{}`, "trace-2", 30) {
		t.Error("should NOT verify custom_tool when risk score 30 < threshold 50")
	}
}

// TestCFVerifier_ShouldVerify_BudgetExhausted
func TestCFVerifier_ShouldVerify_BudgetExhausted(t *testing.T) {
	db := setupCFTestDB(t)
	defer db.Close()

	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 3,
		RiskThreshold: 50, CacheTTLSec: 300, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, nil)

	// Consume all budget
	v.consumeBudget()
	v.consumeBudget()
	v.consumeBudget()

	if v.ShouldVerify("shell_exec", `{}`, "trace-1", 100) {
		t.Error("should NOT verify when budget exhausted")
	}
}

// TestCFVerifier_BuildControlMessages
func TestCFVerifier_BuildControlMessages(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	messages := []CFMessage{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "What is the weather?"},
		{Role: "assistant", Content: "Let me check..."},
		{Role: "tool", Content: "The weather is sunny", ToolCallID: "tc-1", Name: "get_weather"},
		{Role: "user", Content: "Thanks!"},
	}

	control := v.BuildControlMessages(messages)
	// Should remove the tool message
	for _, msg := range control {
		if msg.Role == "tool" {
			t.Error("control messages should not contain tool role")
		}
	}
	if len(control) != 4 { // system + user + assistant + user
		t.Errorf("expected 4 control messages, got %d", len(control))
	}
}

// TestCFVerifier_BuildControlMessages_KeepUserMessages
func TestCFVerifier_BuildControlMessages_KeepUserMessages(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	messages := []CFMessage{
		{Role: "user", Content: "Hello"},
		{Role: "user", Content: "How are you?"},
		{Role: "function", Content: "external data", Name: "fn1"},
	}

	control := v.BuildControlMessages(messages)
	userCount := 0
	for _, msg := range control {
		if msg.Role == "user" {
			userCount++
		}
		if msg.Role == "function" {
			t.Error("function messages should be removed")
		}
	}
	if userCount != 2 {
		t.Errorf("expected 2 user messages, got %d", userCount)
	}
}

// TestCFVerifier_BuildControlMessages_KeepSystem
func TestCFVerifier_BuildControlMessages_KeepSystem(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	messages := []CFMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
	}
	control := v.BuildControlMessages(messages)
	if len(control) != 2 {
		t.Errorf("expected 2, got %d", len(control))
	}
	if control[0].Role != "system" {
		t.Error("first message should be system")
	}
}

// TestCFVerifier_BuildControlMessages_AnthropicToolResult
func TestCFVerifier_BuildControlMessages_AnthropicToolResult(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// Anthropic format: user message with tool_result block
	messages := []CFMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "User input"},
			map[string]interface{}{"type": "tool_result", "tool_use_id": "tu-1", "content": "external data"},
		}},
	}

	control := v.BuildControlMessages(messages)
	if len(control) != 2 {
		t.Errorf("expected 2 messages, got %d", len(control))
	}
	// The user message should have the tool_result block removed
	blocks, ok := control[1].Content.([]interface{})
	if !ok {
		t.Fatal("user content should be array")
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 block (tool_result removed), got %d", len(blocks))
	}
	bm, ok := blocks[0].(map[string]interface{})
	if !ok {
		t.Fatal("block should be map")
	}
	if bm["type"] != "text" {
		t.Error("remaining block should be text type")
	}
}

// TestCFVerifier_CompareResults_SameToolSameArgs
func TestCFVerifier_CompareResults_SameToolSameArgs(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// Anthropic format response with same tool call
	resp := `{"content":[{"type":"tool_use","name":"shell_exec","input":{"cmd":"ls"}}]}`
	survived, attribution := v.CompareResults("shell_exec", `{"cmd":"ls"}`, []byte(resp), true)
	if !survived {
		t.Error("should survive when same tool+args")
	}
	if attribution != 0.0 {
		t.Errorf("attribution should be 0.0, got %.2f", attribution)
	}
}

// TestCFVerifier_CompareResults_SameToolDiffArgs
func TestCFVerifier_CompareResults_SameToolDiffArgs(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	resp := `{"content":[{"type":"tool_use","name":"shell_exec","input":{"cmd":"pwd"}}]}`

	// Fuzzy match mode
	survived, attribution := v.CompareResults("shell_exec", `{"cmd":"ls"}`, []byte(resp), true)
	if !survived {
		t.Error("fuzzy mode: should survive with same tool name")
	}
	if attribution != 0.3 {
		t.Errorf("fuzzy mode: attribution should be 0.3, got %.2f", attribution)
	}

	// Strict mode
	survived, attribution = v.CompareResults("shell_exec", `{"cmd":"ls"}`, []byte(resp), false)
	if survived {
		t.Error("strict mode: should NOT survive with different args")
	}
	if attribution != 0.7 {
		t.Errorf("strict mode: attribution should be 0.7, got %.2f", attribution)
	}
}

// TestCFVerifier_CompareResults_DiffTool
func TestCFVerifier_CompareResults_DiffTool(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	resp := `{"content":[{"type":"tool_use","name":"read_file","input":{"path":"/tmp/a"}}]}`
	survived, attribution := v.CompareResults("shell_exec", `{"cmd":"ls"}`, []byte(resp), true)
	if survived {
		t.Error("should NOT survive with different tool")
	}
	if attribution != 0.9 {
		t.Errorf("attribution should be 0.9, got %.2f", attribution)
	}
}

// TestCFVerifier_CompareResults_NoToolCall
func TestCFVerifier_CompareResults_NoToolCall(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	resp := `{"content":[{"type":"text","text":"I cannot do that."}]}`
	survived, attribution := v.CompareResults("shell_exec", `{"cmd":"rm -rf /"}`, []byte(resp), true)
	if survived {
		t.Error("should NOT survive when control has no tool call")
	}
	if attribution != 1.0 {
		t.Errorf("attribution should be 1.0, got %.2f", attribution)
	}
}

// TestCFVerifier_Verify_Sync 用 httptest.Server 模拟 LLM
func TestCFVerifier_Verify_Sync(t *testing.T) {
	// Mock LLM that returns the same tool call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "tool_use",
					"name": "shell_exec",
					"input": map[string]interface{}{
						"cmd": "ls -la",
					},
				},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 300, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"Run ls"}]}`
	result := v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"ls -la"}`, server.URL, "")

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Verdict != "USER_DRIVEN" {
		t.Errorf("verdict should be USER_DRIVEN, got %s", result.Verdict)
	}
	if result.AttributionScore != 0.0 {
		t.Errorf("attribution should be 0.0, got %.2f", result.AttributionScore)
	}
	if result.Decision != "allow" {
		t.Errorf("decision should be allow, got %s", result.Decision)
	}
}

// TestCFVerifier_Verify_Async
func TestCFVerifier_Verify_Async(t *testing.T) {
	// Mock LLM that returns no tool call (injection detected)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "I cannot do that."},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "async", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 300, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"Run rm -rf"}]}`
	result := v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"rm -rf /"}`, server.URL, "")

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Verdict != "INJECTION_DRIVEN" {
		t.Errorf("verdict should be INJECTION_DRIVEN, got %s", result.Verdict)
	}
	if result.AttributionScore != 1.0 {
		t.Errorf("attribution should be 1.0, got %.2f", result.AttributionScore)
	}
	if result.Decision != "block" {
		t.Errorf("decision should be block, got %s", result.Decision)
	}
}

// TestCFVerifier_Cache
func TestCFVerifier_Cache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "tool_use", "name": "shell_exec", "input": map[string]interface{}{"cmd": "ls"}},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 300, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"Run ls"}]}`
	// First call: fresh
	r1 := v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"ls"}`, server.URL, "")
	if r1.Cached {
		t.Error("first call should not be cached")
	}

	// Second call: should be cached
	r2 := v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"ls"}`, server.URL, "")
	if !r2.Cached {
		t.Error("second call should be cached")
	}
	if r2.Verdict != r1.Verdict {
		t.Errorf("cached verdict mismatch: %s vs %s", r2.Verdict, r1.Verdict)
	}
}

// TestCFVerifier_DBPersistence
func TestCFVerifier_DBPersistence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "No tool call"},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 1, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"Delete all"}]}`
	result := v.Verify(context.Background(), []byte(reqBody), "delete_file", `{"path":"/important"}`, server.URL, "")

	// Query from DB
	found := v.GetVerification(result.ID)
	if found == nil {
		t.Fatal("should find verification in DB")
	}
	if found.ToolName != "delete_file" {
		t.Errorf("tool_name mismatch: %s", found.ToolName)
	}
	if found.Verdict != "INJECTION_DRIVEN" {
		t.Errorf("verdict mismatch: %s", found.Verdict)
	}
}

// TestCFVerifier_Stats
func TestCFVerifier_Stats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "tool_use", "name": "shell_exec", "input": map[string]interface{}{"cmd": "ls"}},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 1, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"ls"}]}`
	v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"ls"}`, server.URL, "")
	time.Sleep(10 * time.Millisecond) // let cache expire

	stats := v.GetStats()
	if stats.TotalVerifications < 1 {
		t.Errorf("total should be >= 1, got %d", stats.TotalVerifications)
	}
	if stats.HourlyBudget != 100 {
		t.Errorf("budget should be 100, got %d", stats.HourlyBudget)
	}
}

// TestCFVerifier_QueryVerifications
func TestCFVerifier_QueryVerifications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "No"},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 1, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"test"}]}`
	v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"test1"}`, server.URL, "")
	time.Sleep(10 * time.Millisecond) // let cache expire
	v.Verify(context.Background(), []byte(reqBody), "send_email", `{"to":"a@b.com"}`, server.URL, "")

	results := v.QueryVerifications("", "INJECTION_DRIVEN", "", 10)
	if len(results) < 2 {
		t.Errorf("expected at least 2 INJECTION_DRIVEN, got %d", len(results))
	}
	// Query by empty verdict
	all := v.QueryVerifications("", "", "", 100)
	if len(all) < 2 {
		t.Errorf("expected at least 2 total, got %d", len(all))
	}
}

// TestCFVerifier_Concurrency
func TestCFVerifier_Concurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "tool_use", "name": "shell_exec", "input": map[string]interface{}{"cmd": "ls"}},
			},
		})
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 1000,
		RiskThreshold: 50, CacheTTLSec: 1, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reqBody := fmt.Sprintf(`{"model":"test","messages":[{"role":"user","content":"test %d"}]}`, idx)
			result := v.Verify(context.Background(), []byte(reqBody), "shell_exec",
				fmt.Sprintf(`{"cmd":"ls-%d"}`, idx), server.URL, "")
			if result == nil {
				t.Errorf("result %d should not be nil", idx)
			}
		}(i)
	}
	wg.Wait()

	stats := v.GetStats()
	if stats.TotalVerifications < 20 {
		t.Errorf("expected >= 20 verifications, got %d", stats.TotalVerifications)
	}
}

// TestCFVerifier_ConfigUpdate
func TestCFVerifier_ConfigUpdate(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// Update config
	newCfg := CFConfig{
		Enabled:       false,
		Mode:          "async",
		MaxPerHour:    200,
		RiskThreshold: 75,
		CacheTTLSec:   600,
		TimeoutSec:    20,
		FuzzyMatch:    false,
	}
	v.UpdateConfig(newCfg)

	cfg := v.GetConfig()
	if cfg.Enabled {
		t.Error("should be disabled")
	}
	if cfg.Mode != "async" {
		t.Errorf("mode should be async, got %s", cfg.Mode)
	}
	if cfg.MaxPerHour != 200 {
		t.Errorf("max_per_hour should be 200, got %d", cfg.MaxPerHour)
	}
	if cfg.RiskThreshold != 75 {
		t.Errorf("threshold should be 75, got %.0f", cfg.RiskThreshold)
	}
	if cfg.FuzzyMatch {
		t.Error("fuzzy_match should be false")
	}

	// Verify disabled means ShouldVerify returns false
	if v.ShouldVerify("shell_exec", `{}`, "t", 100) {
		t.Error("should NOT verify when disabled")
	}
}

// TestCFVerifier_CacheClearing
func TestCFVerifier_CacheClearing(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// Manually add cache entries
	v.mu.Lock()
	v.cache["key1"] = &CFCacheEntry{Survived: true, Verdict: "USER_DRIVEN", CachedAt: time.Now()}
	v.cache["key2"] = &CFCacheEntry{Survived: false, Verdict: "INJECTION_DRIVEN", CachedAt: time.Now()}
	v.mu.Unlock()

	stats := v.GetCacheStats()
	if stats["total_entries"].(int) != 2 {
		t.Errorf("expected 2 cache entries, got %v", stats["total_entries"])
	}

	n := v.ClearCache()
	if n != 2 {
		t.Errorf("expected 2 cleared, got %d", n)
	}

	stats = v.GetCacheStats()
	if stats["total_entries"].(int) != 0 {
		t.Error("cache should be empty after clearing")
	}
}

// TestCFVerifier_OpenAIFormat tests CompareResults with OpenAI-style response
func TestCFVerifier_OpenAIFormat(t *testing.T) {
	v, db := newTestCFVerifier(t)
	defer db.Close()

	// OpenAI format response
	resp := `{"choices":[{"message":{"tool_calls":[{"function":{"name":"shell_exec","arguments":"{\"cmd\":\"ls\"}"}}]}}]}`
	survived, attribution := v.CompareResults("shell_exec", `{"cmd":"ls"}`, []byte(resp), true)
	if !survived {
		t.Error("should survive with OpenAI format same tool+args")
	}
	if attribution != 0.0 {
		t.Errorf("attribution should be 0.0, got %.2f", attribution)
	}
}

// TestCFVerifier_VerifyControlFailure tests behavior when control request fails
func TestCFVerifier_VerifyControlFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	db := setupCFTestDB(t)
	defer db.Close()
	cfg := CFConfig{
		Enabled: true, Mode: "sync", MaxPerHour: 100,
		RiskThreshold: 50, CacheTTLSec: 300, TimeoutSec: 10, FuzzyMatch: true,
	}
	v := NewCounterfactualVerifier(db, cfg, server.Client())

	reqBody := `{"model":"test","messages":[{"role":"user","content":"test"}]}`
	result := v.Verify(context.Background(), []byte(reqBody), "shell_exec", `{"cmd":"ls"}`, server.URL, "")
	if result == nil {
		t.Fatal("result should not be nil even on failure")
	}
	if result.Verdict != "INCONCLUSIVE" {
		t.Errorf("verdict should be INCONCLUSIVE on failure, got %s", result.Verdict)
	}
	if result.Decision != "allow" {
		t.Errorf("decision should be allow on inconclusive (fail-open), got %s", result.Decision)
	}
}
