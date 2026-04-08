package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type llmSSETailRewriteResult struct {
	Decision      string
	RuleNames     []string
	TopMatch      LLMRuleMatch
	HasMatch      bool
	Content       string
	RewriteEvent  string
	ShouldRewrite bool
}

// evaluateLLMSSETailRewrite centralizes end-of-stream rewrite/block evaluation for SSE responses.
func (lp *LLMProxy) evaluateLLMSSETailRewrite(fullContent string, tenantID string) llmSSETailRewriteResult {
	result := llmSSETailRewriteResult{Content: fullContent}
	if lp.ruleEngine == nil || fullContent == "" {
		return result
	}

	respMatches := lp.ruleEngine.CheckResponseWithTenant(fullContent, tenantID)
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

	if !result.HasMatch {
		return result
	}

	switch action {
	case "rewrite":
		newContent := lp.ruleEngine.ApplyRewrite(fullContent, respMatches)
		if newContent != fullContent {
			result.Content = newContent
			result.ShouldRewrite = true
			result.RewriteEvent = buildSSEJSONEvent("security_rewrite", map[string]interface{}{
				"content":      newContent,
				"rewritten_by": result.TopMatch.RuleName,
			})
			log.Printf("[LLM规则] SSE 尾部改写: rule=%s category=%s len=%d→%d",
				result.TopMatch.RuleID, result.TopMatch.Category, len(fullContent), len(newContent))
		}
	case "block", "warn":
		log.Printf("[LLM规则] SSE 流结束检测: rule=%s action=%s 累积 %d 字节",
			result.TopMatch.RuleID, action, len(fullContent))
	}

	return result
}

func buildSSEJSONEvent(eventName string, payload interface{}) string {
	b, _ := json.Marshal(payload)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, b)
}

func buildSSETextEvent(eventName string, text string) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, strings.ReplaceAll(text, "\n", "\ndata: "))
}