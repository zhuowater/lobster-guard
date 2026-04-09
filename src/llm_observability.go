package main

type llmResponseRecord struct {
	Decision string
	Rules    []string
	Body     []byte
}

func normalizeLLMDecision(decision string) string {
	if decision == "" {
		return "pass"
	}
	return decision
}

func (lp *LLMProxy) sealLLMResponseEnvelope(traceID string, record llmResponseRecord) {
	if lp.envelopeMgr == nil {
		return
	}
	lp.envelopeMgr.Seal(traceID, "llm_response", string(record.Body), normalizeLLMDecision(record.Decision), record.Rules, "")
}

func (lp *LLMProxy) auditLLMResponse(auditCtx *LLMAuditContext, statusCode int, body []byte) {
	if lp.auditor == nil {
		return
	}
	go lp.auditor.ProcessResponse(auditCtx, statusCode, body)
}

func (lp *LLMProxy) sealSSEEnvelope(auditCtx *LLMAuditContext, eventData []byte) {
	if lp.envelopeMgr == nil || len(eventData) == 0 {
		return
	}
	go func() {
		if lp.ruleEngine != nil {
			respMatches := lp.ruleEngine.CheckResponseWithTenant(string(eventData), auditCtx.TenantID)
			if len(respMatches) > 0 {
				action, _ := HighestPriorityAction(respMatches)
				lp.envelopeMgr.Seal(auditCtx.TraceID, "llm_response", string(eventData), action, collectLLMRuleNames(respMatches), "")
				return
			}
		}
		lp.envelopeMgr.Seal(auditCtx.TraceID, "llm_response", string(eventData), "pass", nil, "")
	}()
}

func (lp *LLMProxy) auditSSEBuffer(auditCtx *LLMAuditContext, eventData []byte) {
	if lp.auditor == nil {
		return
	}
	go lp.auditor.ProcessSSEBuffer(auditCtx, eventData)
}