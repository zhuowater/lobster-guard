package main

import (
	"strings"
	"testing"
)

func TestCollectLLMRuleNames_PreservesMatchOrder(t *testing.T) {
	matches := []LLMRuleMatch{{RuleName: "rule-a"}, {RuleName: "rule-b"}}
	got := collectLLMRuleNames(matches)
	if len(got) != 2 || got[0] != "rule-a" || got[1] != "rule-b" {
		t.Fatalf("unexpected rule names: %#v", got)
	}
}

func TestEvaluateLLMResponseRules_Rewrite(t *testing.T) {
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "resp-rewrite-1", Name: "mask-secret", Category: "pii_leak", Direction: "response",
		Type: "keyword", Patterns: []string{"secret-token"}, Action: "rewrite", RewriteTo: "[REDACTED]", Enabled: true, Priority: 10,
	}})
	lp := &LLMProxy{ruleEngine: engine}

	eval := lp.evaluateLLMResponseRules([]byte(`{"content":"secret-token leaked"}`), "default")
	if eval.Decision != "rewrite" {
		t.Fatalf("expected rewrite decision, got %q", eval.Decision)
	}
	if !eval.HasMatch || eval.TopMatch.RuleName != "mask-secret" {
		t.Fatalf("unexpected top match: %#v", eval.TopMatch)
	}
	if !strings.Contains(string(eval.Body), "[REDACTED]") {
		t.Fatalf("expected rewritten body, got %q", string(eval.Body))
	}
}

func TestEvaluateLLMResponseRules_Block(t *testing.T) {
	engine := NewLLMRuleEngine([]LLMRule{{
		ID: "resp-block-1", Name: "block-malware", Category: "sensitive_topic", Direction: "response",
		Type: "keyword", Patterns: []string{"drop database"}, Action: "block", Enabled: true, Priority: 20,
	}})
	lp := &LLMProxy{ruleEngine: engine}
	body := []byte(`{"content":"drop database now"}`)

	eval := lp.evaluateLLMResponseRules(body, "default")
	if eval.Decision != "block" {
		t.Fatalf("expected block decision, got %q", eval.Decision)
	}
	if string(eval.Body) != string(body) {
		t.Fatalf("block should not rewrite body, got %q", string(eval.Body))
	}
	if len(eval.RuleNames) != 1 || eval.RuleNames[0] != "block-malware" {
		t.Fatalf("unexpected rule names: %#v", eval.RuleNames)
	}
}