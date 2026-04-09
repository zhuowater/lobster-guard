package main

import (
	"fmt"
	"log"
	"net/http"
)

type llmDeepGovernanceContext struct {
	TraceID    string
	SenderID   string
	RespBody   []byte
	StatusCode int
	AuditCtx   *LLMAuditContext
}

func (lp *LLMProxy) blockToolByReason(w http.ResponseWriter, ctx llmDeepGovernanceContext, tool, reason, message string) bool {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(403)
	fmt.Fprintf(w, message, tool, reason)
	lp.auditLLMResponse(ctx.AuditCtx, ctx.StatusCode, ctx.RespBody)
	return true
}

func (lp *LLMProxy) evaluatePlanForTool(w http.ResponseWriter, ctx llmDeepGovernanceContext, tool, args string) bool {
	if lp.planCompiler == nil || !lp.isEngineEnabled("plan_compiler") {
		return false
	}
	planEval := lp.planCompiler.EvaluateToolCall(ctx.TraceID, tool, args)
	if planEval == nil || planEval.Violation == nil {
		return false
	}
	log.Printf("[PlanCompiler] 计划偏离: tool=%s violation=%s severity=%s decision=%s trace=%s",
		tool, planEval.Violation.Description, planEval.Violation.Severity, planEval.Decision, ctx.TraceID)
	if planEval.Decision == "block" {
		return lp.blockToolByReason(w, ctx, tool, planEval.Violation.Description, `{"error":"Tool call blocked by plan compiler","tool":"%s","violation":"%s"}`)
	}
	return false
}

func (lp *LLMProxy) evaluateCapabilityForTool(ctx llmDeepGovernanceContext, tool string, index int, priorToolNames []string) {
	if lp.capabilityEngine == nil || !lp.isEngineEnabled("capability") {
		return
	}
	toolDataID := fmt.Sprintf("tool-%s-%d", tool, index)
	lp.capabilityEngine.RegisterToolResult(ctx.TraceID, tool, toolDataID)
	if index > 0 {
		var parentIDs []string
		for j := 0; j < index; j++ {
			parentIDs = append(parentIDs, fmt.Sprintf("tool-%s-%d", priorToolNames[j], j))
		}
		lp.capabilityEngine.PropagateData(ctx.TraceID, toolDataID, "tool:"+tool, parentIDs)
	}
	capEval := lp.capabilityEngine.EvaluateWithProvenance(ctx.TraceID, toolDataID, "execute", tool)
	if capEval == nil || (capEval.Decision != "deny" && capEval.Decision != "warn") {
		return
	}
	log.Printf("[Capability] %s: tool=%s reason=%s trace=%s", capEval.Decision, tool, capEval.Reason, ctx.TraceID)
	if lp.eventBus != nil {
		eventType := "capability_warn"
		severity := "medium"
		summary := fmt.Sprintf("Capability warn (untrusted lineage): %s (%s)", tool, capEval.Reason)
		if capEval.Decision == "deny" {
			eventType = "capability_deny"
			severity = "high"
			summary = fmt.Sprintf("Capability denied: %s (%s)", tool, capEval.Reason)
		}
		lp.eventBus.Emit(&SecurityEvent{Type: eventType, Severity: severity, Domain: "llm", TraceID: ctx.TraceID, Summary: summary})
	}
	if lp.auditLogger != nil {
		lp.auditLogger.LogWithTrace("outbound", ctx.SenderID, capEval.Decision,
			fmt.Sprintf("[Capability] %s: %s", capEval.Decision, capEval.Reason),
			fmt.Sprintf("tool_call: %s", tool), "", 0, "", "", ctx.TraceID)
	}
}