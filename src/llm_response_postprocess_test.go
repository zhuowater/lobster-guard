package main

import "testing"

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