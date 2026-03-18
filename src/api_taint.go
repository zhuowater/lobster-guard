// api_taint.go — 污染追踪管理 API
// lobster-guard v20.1
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleTaintStats GET /api/v1/taint/stats
func (api *ManagementAPI) handleTaintStats(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":      false,
			"active_count": 0,
			"total_marked": 0,
		})
		return
	}
	stats := api.taintTracker.Stats()
	jsonResponse(w, 200, stats)
}

// handleTaintActive GET /api/v1/taint/active
func (api *ManagementAPI) handleTaintActive(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 200, map[string]interface{}{"entries": []interface{}{}, "total": 0})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	entries := api.taintTracker.ListTainted(limit)
	if entries == nil {
		entries = []TaintEntry{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleTaintTrace GET /api/v1/taint/trace/:id
func (api *ManagementAPI) handleTaintTrace(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "taint tracker not enabled"})
		return
	}
	// 提取 trace_id from path: /api/v1/taint/trace/xxxx
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 6 {
		jsonResponse(w, 400, map[string]string{"error": "missing trace_id"})
		return
	}
	traceID := parts[5]

	entry := api.taintTracker.GetTaint(traceID)
	if entry == nil {
		jsonResponse(w, 404, map[string]string{"error": "trace not found"})
		return
	}
	jsonResponse(w, 200, entry)
}

// handleTaintConfigGet GET /api/v1/taint/config
func (api *ManagementAPI) handleTaintConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 200, TaintConfig{})
		return
	}
	cfg := api.taintTracker.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleTaintConfigUpdate PUT /api/v1/taint/config
func (api *ManagementAPI) handleTaintConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 400, map[string]string{"error": "taint tracker not initialized"})
		return
	}
	var req TaintConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	// 验证 action
	if req.Action != "" && req.Action != "block" && req.Action != "warn" && req.Action != "log" {
		jsonResponse(w, 400, map[string]string{"error": "action must be block/warn/log"})
		return
	}
	api.taintTracker.UpdateConfig(req)
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleTaintScan POST /api/v1/taint/scan
func (api *ManagementAPI) handleTaintScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Text == "" {
		jsonResponse(w, 400, map[string]string{"error": "text is required"})
		return
	}

	matchedNames, labels := ScanPII(req.Text)
	jsonResponse(w, 200, map[string]interface{}{
		"text":     req.Text,
		"tainted":  len(labels) > 0,
		"matches":  matchedNames,
		"labels":   labels,
		"patterns": len(piiPatterns),
	})
}
