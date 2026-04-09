package main

import (
	"testing"
	"time"
)

func TestApplyLLMResponseReversal_NoEngine(t *testing.T) {
	lp := &LLMProxy{}
	body := []byte("hello")
	got, changed := lp.applyLLMResponseReversal("trace-1", "taint-1", body)
	if changed {
		t.Fatal("expected no reversal without engine")
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected body: %q", string(got))
	}
}

func TestStoreLLMCacheEntry_NoCacheIsSafe(t *testing.T) {
	lp := &LLMProxy{}
	lp.storeLLMCacheEntry("q", []byte("resp"), "model-a", "tenant-a", "trace-a", 200)
}

func TestApplyLLMResponseTaint_NoTrackerIsSafe(t *testing.T) {
	lp := &LLMProxy{}
	lp.applyLLMResponseTaint("trace-1", "taint-1")
}

func TestApplyLLMResponseTaint_AppendsPropagation(t *testing.T) {
	tt, db := newTestTaintTracker(t, TaintConfig{Enabled: true, Action: "warn", TTLMinutes: 30})
	defer db.Close()
	defer tt.Stop()
	if entry := tt.MarkTainted("trace-1", "手机号 13812345678", "inbound"); entry == nil {
		t.Fatal("expected initial taint")
	}
	lp := &LLMProxy{taintTracker: tt}
	lp.applyLLMResponseTaint("trace-1", "trace-1")

	entry := tt.GetTaint("trace-1")
	if entry == nil {
		t.Fatal("expected taint entry")
	}
	if len(entry.Propagations) < 2 {
		t.Fatalf("expected llm_response propagation, got %#v", entry.Propagations)
	}
	last := entry.Propagations[len(entry.Propagations)-1]
	if last.Stage != "llm_response" {
		t.Fatalf("expected llm_response stage, got %#v", last)
	}
}

func TestApplyLLMResponseReversal_UsesEngineWhenTainted(t *testing.T) {
	engine, tt, db := newTestReversalEngine(t, TaintReversalConfig{Enabled: true, Mode: "soft"}, nil)
	defer db.Close()
	defer tt.Stop()
	if entry := tt.MarkTainted("trace-reverse", "手机号 13812345678", "inbound"); entry == nil {
		t.Fatal("expected taint entry")
	}
	lp := &LLMProxy{reversalEngine: engine}
	got, changed := lp.applyLLMResponseReversal("trace-reverse", "trace-reverse", []byte("original"))
	if !changed {
		t.Fatal("expected reversal to change response")
	}
	if string(got) == "original" {
		t.Fatalf("expected reversed content, got %q", string(got))
	}
}

func TestStoreLLMCacheEntry_SkipsTaintedResponse(t *testing.T) {
	tt, tdb := newTestTaintTracker(t, TaintConfig{Enabled: true, Action: "warn", TTLMinutes: 30})
	defer tdb.Close()
	defer tt.Stop()
	cache, cdb := setupTestCache(t, LLMCacheConfig{Enabled: true, TenantIsolation: true, SkipTainted: true})
	defer cdb.Close()
	if entry := tt.MarkTainted("trace-cache", "手机号 13812345678", "inbound"); entry == nil {
		t.Fatal("expected taint entry")
	}
	lp := &LLMProxy{taintTracker: tt, llmCache: cache}
	lp.storeLLMCacheEntry("query", []byte("resp"), "gpt-4", "tenant-a", "trace-cache", 200)
	time.Sleep(20 * time.Millisecond)

	if _, hit := cache.Lookup("query", "gpt-4", "tenant-a"); hit {
		t.Fatal("tainted response should not be cached")
	}
}
