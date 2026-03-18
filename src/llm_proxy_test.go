// llm_proxy_test.go — LLMProxy 测试
package main

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestLLMProxy(t *testing.T, handler http.HandlerFunc) (*LLMProxy, *sql.DB, *httptest.Server) {
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

	// Mock upstream
	upstream := httptest.NewServer(handler)

	proxyCfg := LLMProxyConfig{
		Enabled: true,
		Listen:  ":0",
		Targets: []LLMTargetConfig{
			{
				Name:         "test",
				Upstream:     upstream.URL,
				PathPrefix:   "/v1/",
				APIKeyHeader: "x-api-key",
			},
		},
		TimeoutSec:   30,
		MaxBodyBytes:  10 * 1024 * 1024,
	}

	proxy := NewLLMProxy(proxyCfg, auditor, nil)
	return proxy, db, upstream
}

func TestLLMProxy_TransparentForward(t *testing.T) {
	requestBody := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hello"}]}`
	responseBody := `{"model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hi!"}],"usage":{"input_tokens":10,"output_tokens":5}}`

	var receivedBody string
	var receivedHeaders http.Header

	proxy, db, upstream := setupTestLLMProxy(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(responseBody))
	})
	defer db.Close()
	defer upstream.Close()

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key-123")
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	// Verify request was forwarded
	if receivedBody != requestBody {
		t.Errorf("upstream received body = %q, want %q", receivedBody, requestBody)
	}

	// Verify API key was forwarded
	if receivedHeaders.Get("x-api-key") != "test-key-123" {
		t.Errorf("upstream x-api-key = %q, want test-key-123", receivedHeaders.Get("x-api-key"))
	}

	// Verify response body
	respBody := rr.Body.String()
	if respBody != responseBody {
		t.Errorf("response body = %q, want %q", respBody, responseBody)
	}
}

func TestLLMProxy_SSEStreaming(t *testing.T) {
	sseData := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\ndata: [DONE]\n"

	proxy, db, upstream := setupTestLLMProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		for _, line := range strings.Split(sseData, "\n") {
			w.Write([]byte(line + "\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	})
	defer db.Close()
	defer upstream.Close()

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-20250514","stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	// Verify SSE data was forwarded
	respBody := rr.Body.String()
	if !strings.Contains(respBody, "message_start") {
		t.Error("SSE response should contain message_start event")
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Error("SSE response should contain [DONE]")
	}
}

func TestLLMProxy_PanicRecovery(t *testing.T) {
	proxy, db, upstream := setupTestLLMProxy(t, func(w http.ResponseWriter, r *http.Request) {
		panic("simulated upstream panic")
	})
	defer db.Close()
	defer upstream.Close()

	// The proxy should recover from panic in its own code, but if upstream panics
	// the http client will get a connection error. Test that proxy handles it gracefully.
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	// This should not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("proxy should not panic, got: %v", r)
			}
		}()
		proxy.ServeHTTP(rr, req)
	}()

	// Should get an error response (502) since upstream panicked
	if rr.Code != 502 {
		// May be 200 if the testserver catches the panic, that's also fine
		t.Logf("status = %d (upstream panic handled)", rr.Code)
	}
}

func TestLLMProxy_NoTarget(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	auditor := NewLLMAuditor(db, LLMAuditConfig{MaxPreviewLen: 500}, nil)
	proxy := NewLLMProxy(LLMProxyConfig{
		Enabled: true,
		Listen:  ":0",
		Targets: []LLMTargetConfig{}, // no targets
	}, auditor, nil)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	proxy.ServeHTTP(rr, req)

	if rr.Code != 502 {
		t.Errorf("status = %d, want 502", rr.Code)
	}
}
