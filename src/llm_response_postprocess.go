package main

import "fmt"

func (lp *LLMProxy) applyLLMResponseTaint(traceID, taintTraceID string) {
	if lp.taintTracker == nil {
		return
	}
	lp.taintTracker.Propagate(taintTraceID, "llm_response",
		fmt.Sprintf("LLM response received (llm_trace=%s)", traceID))
}

func (lp *LLMProxy) applyLLMResponseReversal(traceID, taintTraceID string, respBody []byte) ([]byte, bool) {
	if lp.reversalEngine == nil || len(respBody) == 0 {
		return respBody, false
	}
	reversed, record := lp.reversalEngine.Reverse(taintTraceID, string(respBody))
	if record == nil {
		return respBody, false
	}
	return []byte(reversed), true
}

func (lp *LLMProxy) storeLLMCacheEntry(cacheQuery string, respBody []byte, model, tenantID, traceID string, statusCode int) {
	if lp.llmCache == nil || cacheQuery == "" || statusCode != 200 {
		return
	}
	tainted := false
	if lp.taintTracker != nil {
		te := lp.taintTracker.GetTaint(traceID)
		if te != nil && len(te.Labels) > 0 {
			tainted = true
		}
	}
	go lp.llmCache.Store(cacheQuery, string(respBody), model, tenantID, tainted)
}