package main

import (
	"net/http"
	"strings"
)

// resolveLLMTraceID returns the inbound trace ID if present, otherwise generates one.
func resolveLLMTraceIDHeader(xTraceId string, xTraceID string) string {
	if xTraceId != "" {
		return xTraceId
	}
	if xTraceID != "" {
		return xTraceID
	}
	return GenerateTraceID()
}

func resolveLLMTraceID(r *http.Request) string {
	return resolveLLMTraceIDHeader(r.Header.Get("X-Trace-Id"), r.Header.Get("X-Trace-ID"))
}

// resolveTaintTraceID prefers the correlated IM trace when available.
func resolveTaintTraceID(traceID string, sessionLink *SessionLink) string {
	if sessionLink != nil && sessionLink.IMTraceID != "" {
		return sessionLink.IMTraceID
	}
	return traceID
}

// buildLLMUpstreamRequestPath normalizes prefix-stripping behavior for upstream forwarding.
func buildLLMUpstreamRequestPath(requestURI string, target *LLMTargetConfig) string {
	if target == nil {
		return requestURI
	}
	if target.StripPrefix && target.PathPrefix != "" && strings.HasPrefix(requestURI, target.PathPrefix) {
		stripped := strings.TrimPrefix(requestURI, target.PathPrefix)
		if !strings.HasPrefix(stripped, "/") {
			stripped = "/" + stripped
		}
		return stripped
	}
	return requestURI
}
