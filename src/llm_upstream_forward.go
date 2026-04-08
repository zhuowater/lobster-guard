package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}

func buildLLMUpstreamRequest(r *http.Request, upstreamURL string, bodyBytes []byte, traceID string) (*http.Request, error) {
	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	copyHeaders(upReq.Header, r.Header)
	upReq.Header.Set("X-Trace-ID", traceID)
	upReq.ContentLength = int64(len(bodyBytes))
	return upReq, nil
}

func (lp *LLMProxy) forwardLLMUpstream(r *http.Request, upstreamURL string, bodyBytes []byte, traceID string) (*http.Response, error) {
	upReq, err := buildLLMUpstreamRequest(r, upstreamURL, bodyBytes, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}
	resp, err := lp.client.Do(upReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	return resp, nil
}

func buildLLMAuditContext(start time.Time, traceID, model string, reqBody []byte, canaryToken, tenantID string, sessionLink *SessionLink) *LLMAuditContext {
	auditCtx := &LLMAuditContext{
		TraceID:     traceID,
		StartTime:   start,
		Model:       model,
		ReqBody:     reqBody,
		CanaryToken: canaryToken,
		TenantID:    tenantID,
	}
	if sessionLink != nil {
		auditCtx.IMTraceID = sessionLink.IMTraceID
		auditCtx.SenderID = sessionLink.SenderID
		auditCtx.SessionID = sessionLink.SessionID
	}
	return auditCtx
}