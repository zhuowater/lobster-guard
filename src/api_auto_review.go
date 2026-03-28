// api_auto_review.go — AC 智能分级 API handlers
// v31.0: 自动复核管理 API
package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleAutoReviewStatus GET /api/v1/auto-review/status
func (api *ManagementAPI) handleAutoReviewStatus(w http.ResponseWriter, r *http.Request) {
	if api.autoReviewMgr == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled": false,
			"rules":   []interface{}{},
		})
		return
	}
	config := api.autoReviewMgr.GetConfig()
	rules := api.autoReviewMgr.GetReviewRules()
	if rules == nil {
		rules = []AutoReviewStatus{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"enabled": config.Enabled,
		"config":  config,
		"rules":   rules,
	})
}

// handleAutoReviewConfig POST /api/v1/auto-review/config
func (api *ManagementAPI) handleAutoReviewConfig(w http.ResponseWriter, r *http.Request) {
	if api.autoReviewMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "auto-review not initialized"})
		return
	}
	var cfg RuleAutoReviewConfig
	if json.NewDecoder(r.Body).Decode(&cfg) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	api.autoReviewMgr.UpdateConfig(cfg)
	// 同步到全局配置
	api.cfg.AutoReview = cfg
	jsonResponse(w, 200, map[string]interface{}{
		"status": "ok",
		"config": api.autoReviewMgr.GetConfig(),
	})
}

// handleAutoReviewStats GET /api/v1/auto-review/stats
func (api *ManagementAPI) handleAutoReviewStats(w http.ResponseWriter, r *http.Request) {
	if api.autoReviewMgr == nil {
		jsonResponse(w, 200, AutoReviewStats{})
		return
	}
	jsonResponse(w, 200, api.autoReviewMgr.GetStats())
}

// handleAutoReviewSetReview POST /api/v1/auto-review/rules/:name/review
func (api *ManagementAPI) handleAutoReviewSetReview(w http.ResponseWriter, r *http.Request) {
	if api.autoReviewMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "auto-review not initialized"})
		return
	}
	// Extract rule name from path: /api/v1/auto-review/rules/{name}/review
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v1/auto-review/rules/")
	name := strings.TrimSuffix(path, "/review")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "rule name required"})
		return
	}
	api.autoReviewMgr.SetManualReview(name)
	jsonResponse(w, 200, map[string]string{"status": "ok", "rule": name, "mode": "review"})
}

// handleAutoReviewRestore POST /api/v1/auto-review/rules/:name/restore
func (api *ManagementAPI) handleAutoReviewRestore(w http.ResponseWriter, r *http.Request) {
	if api.autoReviewMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "auto-review not initialized"})
		return
	}
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v1/auto-review/rules/")
	name := strings.TrimSuffix(path, "/restore")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "rule name required"})
		return
	}
	api.autoReviewMgr.RestoreFromReview(name)
	jsonResponse(w, 200, map[string]string{"status": "ok", "rule": name, "mode": "block"})
}
