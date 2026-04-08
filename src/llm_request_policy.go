package main

import (
	"fmt"
	"log"
	"net/http"
)

type llmRequestPolicyEvaluation struct {
	Decision  string
	RuleNames []string
	TopMatch  LLMRuleMatch
	HasMatch  bool
	Blocked   bool
}

// evaluateLLMRequestPolicy centralizes request-side rule evaluation and any immediate block response.
func (lp *LLMProxy) evaluateLLMRequestPolicy(w http.ResponseWriter, traceID string, bodyBytes []byte, tenantID string) llmRequestPolicyEvaluation {
	result := llmRequestPolicyEvaluation{}
	if lp.ruleEngine == nil || len(bodyBytes) == 0 {
		return result
	}

	reqMatches := lp.ruleEngine.CheckRequestWithTenant(string(bodyBytes), tenantID)
	if len(reqMatches) == 0 {
		return result
	}

	action, topMatch := HighestPriorityAction(reqMatches)
	result.Decision = action
	result.RuleNames = collectLLMRuleNames(reqMatches)
	if topMatch != nil {
		result.TopMatch = *topMatch
		result.HasMatch = true
	}

	if action == "block" && lp.ruleEngine.autoReviewMgr != nil && len(result.RuleNames) > 0 {
		allInReview := true
		for _, rule := range result.RuleNames {
			lp.ruleEngine.autoReviewMgr.RecordBlock(rule)
			if !lp.ruleEngine.autoReviewMgr.IsInReview(rule) {
				allInReview = false
			}
		}
		if allInReview {
			action = lp.ruleEngine.autoReviewMgr.ReviewWithLLM(result.RuleNames[0], string(bodyBytes))
			result.Decision = action
			log.Printf("[LLM规则] auto-review: %s → %s rule=%s", "block", action, result.RuleNames[0])
		}
	}

	if !result.HasMatch {
		return result
	}

	switch action {
	case "block":
		log.Printf("[LLM规则] 请求被阻断: rule=%s category=%s pattern=%q",
			result.TopMatch.RuleID, result.TopMatch.Category, result.TopMatch.Pattern)
		if lp.envelopeMgr != nil {
			lp.envelopeMgr.Seal(traceID, "llm_request", string(bodyBytes), "block", result.RuleNames, "")
		}
		if lp.eventBus != nil {
			lp.eventBus.Emit(&SecurityEvent{
				Type: "llm_block", Severity: "high", Domain: "llm",
				TraceID: traceID,
				Summary: fmt.Sprintf("LLM 请求阻断: %s (%s)", result.TopMatch.RuleName, result.TopMatch.Category),
				Details: map[string]interface{}{"rule_id": result.TopMatch.RuleID, "category": result.TopMatch.Category, "rules": result.RuleNames},
			})
		}
		if lp.singularityEngine != nil {
			if shouldExpose, tpl := lp.singularityEngine.ShouldExpose("llm", traceID); shouldExpose && tpl != nil {
				if lp.auditor != nil {
					lp.auditor.LogSingularityExpose(traceID, "llm", tpl.Name, tpl.Level)
				}
				if lp.envelopeMgr != nil {
					lp.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_llm_" + tpl.Name}, "")
				}
				log.Printf("[LLM代理] 🔮 奇点暴露 template=%s level=%d trace_id=%s", tpl.Name, tpl.Level, traceID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				fmt.Fprintf(w, `%s`, tpl.Content)
				result.Blocked = true
				return result
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprintf(w, `{"error":"Request blocked by LLM security rule: %s","rule_id":"%s","category":"%s"}`,
			result.TopMatch.RuleName, result.TopMatch.RuleID, result.TopMatch.Category)
		result.Blocked = true
	case "warn":
		log.Printf("[LLM规则] 请求告警: rule=%s category=%s pattern=%q",
			result.TopMatch.RuleID, result.TopMatch.Category, result.TopMatch.Pattern)
		if lp.eventBus != nil {
			lp.eventBus.Emit(&SecurityEvent{
				Type: "llm_block", Severity: "medium", Domain: "llm",
				TraceID: traceID,
				Summary: fmt.Sprintf("LLM 请求告警: %s (%s)", result.TopMatch.RuleName, result.TopMatch.Category),
				Details: map[string]interface{}{"rule_id": result.TopMatch.RuleID, "category": result.TopMatch.Category, "action": "warn"},
			})
		}
		if lp.singularityEngine != nil {
			if shouldExpose, tpl := lp.singularityEngine.ShouldExpose("llm", traceID); shouldExpose && tpl != nil {
				if lp.auditor != nil {
					lp.auditor.LogSingularityExpose(traceID, "llm", tpl.Name, tpl.Level)
				}
				if lp.envelopeMgr != nil {
					lp.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_llm_" + tpl.Name}, "")
				}
				log.Printf("[LLM代理] 🔮 奇点暴露(warn) template=%s level=%d trace_id=%s", tpl.Name, tpl.Level, traceID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				fmt.Fprintf(w, `%s`, tpl.Content)
				result.Blocked = true
				return result
			}
		}
	case "log":
		log.Printf("[LLM规则] 请求日志: rule=%s category=%s", result.TopMatch.RuleID, result.TopMatch.Category)
	}

	return result
}