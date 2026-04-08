package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestCopyHeaders_PreservesMultipleValues(t *testing.T) {
	src := make(map[string][]string)
	src["X-Test"] = []string{"a", "b"}
	dst := make(map[string][]string)
	copyHeaders(dst, src)
	if len(dst["X-Test"]) != 2 || dst["X-Test"][0] != "a" || dst["X-Test"][1] != "b" {
		t.Fatalf("unexpected copied headers: %#v", dst)
	}
}

func TestBuildLLMUpstreamRequest_CopiesHeadersAndTraceID(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", nil)
	r.Header.Add("Authorization", "Bearer token")
	r.Header.Add("X-Custom", "abc")

	upReq, err := buildLLMUpstreamRequest(r, "https://example.com/v1/messages", []byte(`{"x":1}`), "trace-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upReq.Header.Get("Authorization") != "Bearer token" {
		t.Fatalf("authorization header missing: %#v", upReq.Header)
	}
	if upReq.Header.Get("X-Trace-ID") != "trace-123" {
		t.Fatalf("trace id not overridden: %#v", upReq.Header)
	}
	if upReq.ContentLength != int64(len([]byte(`{"x":1}`))) {
		t.Fatalf("unexpected content length: %d", upReq.ContentLength)
	}
}

func TestBuildLLMAuditContext_PropagatesSessionLink(t *testing.T) {
	start := time.Unix(100, 0)
	ctx := buildLLMAuditContext(start, "trace-1", "gpt-test", []byte("req"), "canary", "tenant-a", &SessionLink{
		IMTraceID: "im-trace",
		SenderID:  "user-1",
		SessionID: "session-1",
	})
	if ctx.TraceID != "trace-1" || ctx.TenantID != "tenant-a" || ctx.IMTraceID != "im-trace" {
		t.Fatalf("unexpected audit context: %#v", ctx)
	}
	if !ctx.StartTime.Equal(start) {
		t.Fatalf("unexpected start time: %v", ctx.StartTime)
	}
}