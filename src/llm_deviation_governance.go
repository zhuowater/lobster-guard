package main

import (
	"fmt"
	"log"
	"net/http"
)

type llmDeviationGovernanceContext struct {
	TraceID    string
	SenderID   string
	RespBody   []byte
	StatusCode int
	AuditCtx   *LLMAuditContext
}

func (lp *LLMProxy) applyDeviationRepair(traceID, tool, args string, devResult *DeviationResult) (string, string) {
	if devResult == nil || !devResult.Repaired {
		return tool, args
	}
	if devResult.RepairedTool != "" {
		log.Printf("[Deviation] 自动修复: %s → %s trace=%s", tool, devResult.RepairedTool, traceID)
		tool = devResult.RepairedTool
	}
	if devResult.RepairedArgs != "" {
		args = devResult.RepairedArgs
	}
	return tool, args
}

func (lp *LLMProxy) evaluateDeviationForTool(w http.ResponseWriter, ctx llmDeviationGovernanceContext, tool, args string) (string, string, bool) {
	if lp.deviationDetector == nil || !lp.isEngineEnabled("deviation") {
		return tool, args, false
	}
	devResult := lp.deviationDetector.Detect(ctx.TraceID, tool, args)
	if !devResult.HasDeviation {
		return tool, args, false
	}
	log.Printf("[Deviation] 检测到偏差: tool=%s type=%s severity=%s decision=%s trace=%s",
		tool, devResult.Deviation.Type, devResult.Deviation.Severity, devResult.Decision, ctx.TraceID)
	if lp.auditLogger != nil {
		devAction := "warn"
		if devResult.Decision == "block" {
			devAction = "block"
		}
		lp.auditLogger.LogWithTrace("outbound", ctx.SenderID, devAction,
			fmt.Sprintf("[Deviation] %s: %s", devResult.Deviation.Type, devResult.Reason),
			fmt.Sprintf("tool_call: %s", tool), "", 0, "", "", ctx.TraceID)
	}
	if devResult.Repaired {
		tool, args = lp.applyDeviationRepair(ctx.TraceID, tool, args, devResult)
		return tool, args, false
	}
	if devResult.Decision == "block" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprintf(w, `{"error":"Tool call blocked by deviation detector","tool":"%s","reason":"%s"}`,
			tool, devResult.Reason)
		lp.auditLLMResponse(ctx.AuditCtx, ctx.StatusCode, ctx.RespBody)
		return tool, args, true
	}
	return tool, args, false
}