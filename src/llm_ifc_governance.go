package main

import (
	"fmt"
	"log"
	"net/http"
)

type llmIFCGovernanceContext struct {
	TraceID    string
	SenderID   string
	RespBody   []byte
	StatusCode int
	AuditCtx   *LLMAuditContext
}

func collectIFCVarIDs(vars []IFCVariable) []string {
	ids := make([]string, 0, len(vars))
	for _, v := range vars {
		ids = append(ids, v.ID)
	}
	return ids
}

func (lp *LLMProxy) evaluateIFCForTool(w http.ResponseWriter, ctx llmIFCGovernanceContext, tool, args string) bool {
	if lp.ifcEngine == nil || !lp.ifcEngine.config.Enabled {
		return false
	}
	toolSource := "tool:" + tool
	toolVar := lp.ifcEngine.RegisterVariable(ctx.TraceID, "tool_result_"+tool, toolSource, args)
	if toolVar == nil {
		return false
	}
	varIDs := collectIFCVarIDs(lp.ifcEngine.GetVariables(ctx.TraceID))
	ifcDecision := lp.ifcEngine.CheckToolCall(ctx.TraceID, tool, varIDs)
	if ifcDecision != nil && !ifcDecision.Allowed && ifcDecision.Decision == "block" {
		log.Printf("[IFC] 信息流违规阻断: tool=%s type=%s trace=%s", tool, ifcDecision.Violation.Type, ctx.TraceID)
		if lp.auditLogger != nil {
			lp.auditLogger.LogWithTrace("outbound", ctx.SenderID, "block",
				fmt.Sprintf("[IFC] %s violation: %s", ifcDecision.Violation.Type, ifcDecision.Reason),
				fmt.Sprintf("tool_call: %s", tool), "", 0, "", "", ctx.TraceID)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprintf(w, `{"error":"Tool call blocked by IFC: %s","tool":"%s","type":"%s"}`,
			ifcDecision.Reason, tool, ifcDecision.Violation.Type)
		lp.auditLLMResponse(ctx.AuditCtx, ctx.StatusCode, ctx.RespBody)
		return true
	}
	if ifcDecision != nil && ifcDecision.Decision == "warn" {
		log.Printf("[IFC] 信息流告警: tool=%s reason=%s trace=%s", tool, ifcDecision.Reason, ctx.TraceID)
		if lp.auditLogger != nil {
			lp.auditLogger.LogWithTrace("outbound", ctx.SenderID, "warn",
				fmt.Sprintf("[IFC] %s: %s", ifcDecision.Violation.Type, ifcDecision.Reason),
				fmt.Sprintf("tool_call: %s", tool), "", 0, "", "", ctx.TraceID)
		}
	}
	if lp.ifcQuarantine != nil && lp.ifcEngine.config.QuarantineEnabled && lp.ifcQuarantine.ShouldRoute(ctx.TraceID, varIDs) {
		quarantineURL, sessionID, qErr := lp.ifcQuarantine.Route(ctx.TraceID, varIDs)
		if qErr == nil && quarantineURL != "" {
			log.Printf("[IFC-Quarantine] 被污染数据路由到隔离LLM: trace=%s upstream=%s session=%s", ctx.TraceID, quarantineURL, sessionID)
		}
	}
	fields := extractFieldNames(args)
	if len(fields) > 0 {
		doeResult := lp.ifcEngine.DetectDOE(ctx.TraceID, tool, fields, nil)
		if doeResult != nil && doeResult.Severity == "critical" {
			log.Printf("[IFC-DOE] 严重数据过度暴露: tool=%s excess=%v trace=%s", tool, doeResult.ExcessFields, ctx.TraceID)
		} else if doeResult != nil && doeResult.Severity == "warning" {
			log.Printf("[IFC-DOE] 数据过度暴露告警: tool=%s excess=%v trace=%s", tool, doeResult.ExcessFields, ctx.TraceID)
		}
	}
	return false
}