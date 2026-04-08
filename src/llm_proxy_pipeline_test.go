package main

import (
	"net/http/httptest"
	"testing"
)

func TestResolveLLMTraceIDHeader_PrefersPrimaryValue(t *testing.T) {
	got := resolveLLMTraceIDHeader("trace-lower", "trace-upper")
	if got != "trace-lower" {
		t.Fatalf("expected primary trace header value, got %q", got)
	}
}

func TestResolveLLMTraceID_FallsBackToLegacyHeader(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", nil)
	r.Header.Set("X-Trace-ID", "trace-upper")

	got := resolveLLMTraceID(r)
	if got != "trace-upper" {
		t.Fatalf("expected legacy trace header fallback, got %q", got)
	}
}

func TestResolveLLMTraceID_GeneratesWhenMissing(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/messages", nil)

	got := resolveLLMTraceID(r)
	if got == "" {
		t.Fatal("expected generated trace id")
	}
}

func TestResolveTaintTraceID_PrefersIMTrace(t *testing.T) {
	link := &SessionLink{IMTraceID: "im-trace-1"}
	if got := resolveTaintTraceID("llm-trace-1", link); got != "im-trace-1" {
		t.Fatalf("expected IM trace ID, got %q", got)
	}
}

func TestResolveTaintTraceID_FallsBackToLLMTrace(t *testing.T) {
	if got := resolveTaintTraceID("llm-trace-1", nil); got != "llm-trace-1" {
		t.Fatalf("expected LLM trace fallback, got %q", got)
	}
}

func TestBuildLLMUpstreamRequestPath_StripsConfiguredPrefix(t *testing.T) {
	target := &LLMTargetConfig{PathPrefix: "/anthropic", StripPrefix: true}
	if got := buildLLMUpstreamRequestPath("/anthropic/v1/messages?stream=true", target); got != "/v1/messages?stream=true" {
		t.Fatalf("unexpected stripped request path: %q", got)
	}
}

func TestBuildLLMUpstreamRequestPath_PreservesOriginalPathWithoutStrip(t *testing.T) {
	target := &LLMTargetConfig{PathPrefix: "/anthropic", StripPrefix: false}
	if got := buildLLMUpstreamRequestPath("/anthropic/v1/messages", target); got != "/anthropic/v1/messages" {
		t.Fatalf("unexpected request path: %q", got)
	}
}
