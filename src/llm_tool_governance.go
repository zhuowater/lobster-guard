package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

type llmToolGovernanceContext struct {
	TraceID     string
	TenantID    string
	UpstreamURL string
	AuthHeader  string
	SenderID    string
	ReqBody     []byte
	RespBody    []byte
	StatusCode  int
	AuditCtx    *LLMAuditContext
}

type llmToolGovernanceResult struct {
	Blocked bool
	Tool    string
	Reason  string
}

func (lp *LLMProxy) evaluateToolPolicyForResponseTool(ctx llmToolGovernanceContext, tcName, tcArgs string) (*ToolCallEvent, bool) {
	if lp.tenantMgr != nil {
		tcfg := lp.tenantMgr.GetConfig(ctx.TenantID)
		if tcfg != nil && isToolBlacklisted(tcName, tcfg.ToolBlacklist) {
			log.Printf("[ToolPolicy] 租户黑名单拦截: tool=%s tenant=%s trace=%s", tcName, ctx.TenantID, ctx.TraceID)
			return nil, false
		}
	}
	tpEvent := lp.toolPolicy.Evaluate(tcName, tcArgs, ctx.TraceID, ctx.TenantID)
	if lp.pathPolicyEngine != nil && lp.isEngineEnabled("path_policy") {
		lp.pathPolicyEngine.RegisterStep(ctx.TraceID, PathStep{Stage: "tool_call", Action: tcName, Details: tcArgs})
		ppDec := lp.pathPolicyEngine.Evaluate(ctx.TraceID, tcName)
		if actionSev(ppDec.Decision) > actionSev(tpEvent.Decision) {
			tpEvent.Decision = ppDec.Decision
			tpEvent.RuleHit = ppDec.RuleName
			tpEvent.RiskLevel = "high"
		}
	}
	return tpEvent, true
}

func (lp *LLMProxy) enforceToolPolicyDecision(w http.ResponseWriter, ctx llmToolGovernanceContext, tcName string, tpEvent *ToolCallEvent) llmToolGovernanceResult {
	if tpEvent == nil {
		return llmToolGovernanceResult{}
	}
	if tpEvent.Decision == "block" {
		log.Printf("[ToolPolicy] 工具调用被阻断: tool=%s rule=%s trace=%s", tcName, tpEvent.RuleHit, ctx.TraceID)
		if lp.eventBus != nil {
			lp.eventBus.Emit(&SecurityEvent{
				Type: "tool_block", Severity: "high", Domain: "llm",
				TraceID: ctx.TraceID,
				Summary: fmt.Sprintf("工具调用阻断: %s (%s)", tcName, tpEvent.RuleHit),
				Details: map[string]interface{}{"tool_name": tcName, "rule_hit": tpEvent.RuleHit, "risk_level": tpEvent.RiskLevel},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprintf(w, `{"error":"Tool call blocked by policy: %s","tool":"%s","rule":"%s"}`,
			tpEvent.RuleHit, tcName, tpEvent.RuleHit)
		lp.auditLLMResponse(ctx.AuditCtx, ctx.StatusCode, ctx.RespBody)
		return llmToolGovernanceResult{Blocked: true, Tool: tcName, Reason: tpEvent.RuleHit}
	}
	if tpEvent.Decision == "warn" {
		log.Printf("[ToolPolicy] 工具调用告警: tool=%s rule=%s trace=%s", tcName, tpEvent.RuleHit, ctx.TraceID)
	}
	return llmToolGovernanceResult{}
}

func (lp *LLMProxy) maybeRunCounterfactualForTool(w http.ResponseWriter, ctx llmToolGovernanceContext, tcName, tcArgs string, tpEvent *ToolCallEvent) llmToolGovernanceResult {
	if tpEvent == nil || lp.cfVerifier == nil || !lp.isEngineEnabled("counterfactual") || !lp.cfVerifier.ShouldVerify(tcName, tcArgs, ctx.TraceID, tpEvent.RiskScoreNum()) {
		return llmToolGovernanceResult{}
	}
	cfCfg := lp.cfVerifier.GetConfig()
	if cfCfg.Mode == "sync" {
		cfResult := lp.cfVerifier.Verify(context.Background(), ctx.ReqBody, tcName, tcArgs, ctx.UpstreamURL, ctx.AuthHeader, ctx.SenderID)
		if cfResult != nil && cfResult.Decision == "block" {
			log.Printf("[Counterfactual] 反事实验证阻断: tool=%s verdict=%s attribution=%.2f trace=%s",
				tcName, cfResult.Verdict, cfResult.AttributionScore, ctx.TraceID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			fmt.Fprintf(w, `{"error":"Tool call blocked by counterfactual verification","tool":"%s","verdict":"%s","attribution_score":%.2f}`,
				tcName, cfResult.Verdict, cfResult.AttributionScore)
			lp.auditLLMResponse(ctx.AuditCtx, ctx.StatusCode, ctx.RespBody)
			return llmToolGovernanceResult{Blocked: true, Tool: tcName, Reason: cfResult.Verdict}
		}
		return llmToolGovernanceResult{}
	}
	go func() {
		lp.cfVerifier.Verify(context.Background(), ctx.ReqBody, tcName, tcArgs, ctx.UpstreamURL, ctx.AuthHeader, ctx.SenderID)
	}()
	return llmToolGovernanceResult{}
}