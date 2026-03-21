// api_taint.go — 污染追踪管理 API
// lobster-guard v20.1
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
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

// handleTaintCleanup POST /api/v1/taint/cleanup — 批量清理过期标记
func (api *ManagementAPI) handleTaintCleanup(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 400, map[string]string{"error": "taint tracker not initialized"})
		return
	}
	api.taintTracker.CleanupNow()
	stats := api.taintTracker.Stats()
	jsonResponse(w, 200, map[string]interface{}{
		"status":       "cleaned",
		"active_count": stats["active_count"],
	})
}

// handleTaintEntryDelete DELETE /api/v1/taint/entry/:trace_id — 删除单条污染标记
func (api *ManagementAPI) handleTaintEntryDelete(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 400, map[string]string{"error": "taint tracker not initialized"})
		return
	}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 6 {
		jsonResponse(w, 400, map[string]string{"error": "missing trace_id"})
		return
	}
	traceID := parts[5]
	api.taintTracker.DeleteEntry(traceID)
	jsonResponse(w, 200, map[string]interface{}{
		"status":   "deleted",
		"trace_id": traceID,
	})
}

// handleTaintInject POST /api/v1/taint/inject — 注入污染标记（测试用）
func (api *ManagementAPI) handleTaintInject(w http.ResponseWriter, r *http.Request) {
	if api.taintTracker == nil {
		jsonResponse(w, 400, map[string]string{"error": "taint tracker not initialized"})
		return
	}
	var req struct {
		Labels []string `json:"labels"`
		Source string   `json:"source"`
		Detail string   `json:"detail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.Labels) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "labels required"})
		return
	}
	traceID := fmt.Sprintf("manual-%d", time.Now().UnixNano())
	api.taintTracker.InjectManual(traceID, req.Labels, req.Source, req.Detail)
	jsonResponse(w, 200, map[string]interface{}{
		"status":   "injected",
		"trace_id": traceID,
	})
}
