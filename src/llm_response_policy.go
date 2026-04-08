package main

import "log"

// llmResponseRuleEvaluation captures the outcome of non-stream response rule checks.
type llmResponseRuleEvaluation struct {
	Decision  string
	RuleNames []string
	TopMatch  LLMRuleMatch
	HasMatch  bool
	Body      []byte
}

func collectLLMRuleNames(matches []LLMRuleMatch) []string {
	rules := make([]string, 0, len(matches))
	for _, m := range matches {
		rules = append(rules, m.RuleName)
	}
	return rules
}

// evaluateLLMResponseRules centralizes non-stream response-side LLM rule evaluation.
func (lp *LLMProxy) evaluateLLMResponseRules(respBody []byte, tenantID string) llmResponseRuleEvaluation {
	result := llmResponseRuleEvaluation{Body: respBody}
	if lp.ruleEngine == nil || len(respBody) == 0 {
		return result
	}

	respMatches := lp.ruleEngine.CheckResponseWithTenant(string(respBody), tenantID)
	if len(respMatches) == 0 {
		return result
	}

	action, topMatch := HighestPriorityAction(respMatches)
	result.Decision = action
	result.RuleNames = collectLLMRuleNames(respMatches)
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
			action = lp.ruleEngine.autoReviewMgr.ReviewWithLLM(result.RuleNames[0], string(respBody))
			result.Decision = action
			log.Printf("[LLM规则] 响应 auto-review: %s → %s rule=%s", "block", action, result.RuleNames[0])
		}
	}

	if !result.HasMatch {
		return result
	}

	switch action {
	case "block":
		log.Printf("[LLM规则] 响应被阻断: rule=%s category=%s", result.TopMatch.RuleID, result.TopMatch.Category)
	case "rewrite":
		newBody := lp.ruleEngine.ApplyRewrite(string(respBody), respMatches)
		result.Body = []byte(newBody)
		log.Printf("[LLM规则] 响应已改写: rule=%s category=%s len=%d→%d",
			result.TopMatch.RuleID, result.TopMatch.Category, len(respBody), len(result.Body))
	case "warn":
		log.Printf("[LLM规则] 响应告警: rule=%s category=%s", result.TopMatch.RuleID, result.TopMatch.Category)
	case "log":
		log.Printf("[LLM规则] 响应日志: rule=%s category=%s", result.TopMatch.RuleID, result.TopMatch.Category)
	}

	return result
}