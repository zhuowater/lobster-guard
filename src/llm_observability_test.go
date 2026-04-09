package main

import "testing"

func TestNormalizeLLMDecision_DefaultsToPass(t *testing.T) {
	if got := normalizeLLMDecision(""); got != "pass" {
		t.Fatalf("expected pass, got %q", got)
	}
	if got := normalizeLLMDecision("block"); got != "block" {
		t.Fatalf("expected block, got %q", got)
	}
}

func TestSealLLMResponseEnvelope_NoEnvelopeManagerIsSafe(t *testing.T) {
	lp := &LLMProxy{}
	lp.sealLLMResponseEnvelope("trace-1", llmResponseRecord{Decision: "", Body: []byte("ok")})
}

func TestAuditHelpers_NoAuditorIsSafe(t *testing.T) {
	lp := &LLMProxy{}
	ctx := &LLMAuditContext{TraceID: "trace-1"}
	lp.auditLLMResponse(ctx, 200, []byte("ok"))
	lp.auditSSEBuffer(ctx, []byte("data: [DONE]"))
}