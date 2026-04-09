package main

import (
	"testing"
	"time"
)

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

func TestSealLLMResponseEnvelope_NormalizesEmptyDecisionToPass(t *testing.T) {
	em := newTestEnvelopeManager(t)
	lp := &LLMProxy{envelopeMgr: em}
	lp.sealLLMResponseEnvelope("trace-pass", llmResponseRecord{Decision: "", Body: []byte("hello")})

	var decision string
	if err := em.db.QueryRow(`SELECT decision FROM execution_envelopes WHERE trace_id = ? ORDER BY rowid DESC LIMIT 1`, "trace-pass").Scan(&decision); err != nil {
		t.Fatalf("query envelope: %v", err)
	}
	if decision != "pass" {
		t.Fatalf("expected normalized pass decision, got %q", decision)
	}
}

func TestSealSSEEnvelope_RuleMatchSealsBlockDecision(t *testing.T) {
	em := newTestEnvelopeManager(t)
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "sse-block-obs", Name: "block-secret", Category: "sensitive_topic", Direction: "response",
		Type: "keyword", Patterns: []string{"drop database"}, Action: "block", Enabled: true, Priority: 10,
	}})
	lp := &LLMProxy{envelopeMgr: em, ruleEngine: engine}
	ctx := &LLMAuditContext{TraceID: "trace-sse", TenantID: "default"}
	lp.sealSSEEnvelope(ctx, []byte("data: please drop database\n\n"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var decision string
		if err := em.db.QueryRow(`SELECT decision FROM execution_envelopes WHERE trace_id = ? ORDER BY rowid DESC LIMIT 1`, "trace-sse").Scan(&decision); err == nil {
			if decision != "block" {
				t.Fatalf("expected block decision, got %q", decision)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for SSE envelope")
}
