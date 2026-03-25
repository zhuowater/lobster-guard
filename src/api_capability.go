// api_capability.go - Capability system management API (v25.1)
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleCapMappingsList GET /api/v1/capabilities/mappings
func (api *ManagementAPI) handleCapMappingsList(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"mappings": []interface{}{}, "total": 0})
		return
	}
	mappings := api.capabilityEngine.ListToolMappings()
	jsonResponse(w, 200, map[string]interface{}{"mappings": mappings, "total": len(mappings)})
}

// handleCapMappingsUpdate PUT /api/v1/capabilities/mappings/:tool
func (api *ManagementAPI) handleCapMappingsUpdate(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 503, map[string]string{"error": "capability engine not enabled"})
		return
	}
	var m CapToolMapping
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	toolName := strings.TrimPrefix(r.URL.Path, "/api/v1/capabilities/mappings/")
	if toolName != "" {
		m.ToolName = toolName
	}
	if err := api.capabilityEngine.UpdateToolMapping(m); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleCapMappingsDelete DELETE /api/v1/capabilities/mappings/:tool
func (api *ManagementAPI) handleCapMappingsDelete(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 503, map[string]string{"error": "capability engine not enabled"})
		return
	}
	toolName := strings.TrimPrefix(r.URL.Path, "/api/v1/capabilities/mappings/")
	if err := api.capabilityEngine.DeleteToolMapping(toolName); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted"})
}

// handleCapContexts GET /api/v1/capabilities/contexts
func (api *ManagementAPI) handleCapContexts(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"contexts": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	ctxs, total := api.capabilityEngine.QueryContexts(status, limit, offset)
	jsonResponse(w, 200, map[string]interface{}{"contexts": ctxs, "total": total, "limit": limit, "offset": offset})
}

// handleCapEvaluations GET /api/v1/capabilities/evaluations
func (api *ManagementAPI) handleCapEvaluations(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"evaluations": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	traceID := q.Get("trace_id")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	evals, total := api.capabilityEngine.QueryEvaluations(traceID, limit, offset)
	jsonResponse(w, 200, map[string]interface{}{"evaluations": evals, "total": total})
}

// handleCapStats GET /api/v1/capabilities/stats
func (api *ManagementAPI) handleCapStats(w http.ResponseWriter, r *http.Request) {
	if api.capabilityEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"total_contexts": 0, "tool_mapping_count": 0})
		return
	}
	stats := api.capabilityEngine.GetStats()
	jsonResponse(w, 200, stats)
}
