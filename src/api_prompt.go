package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (api *ManagementAPI) handlePromptsList(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 200, map[string]interface{}{"versions": []interface{}{}, "total": 0})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	versions := api.promptTracker.ListVersionsTenant(tenantID)
	if versions == nil {
		versions = []PromptVersion{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"versions": versions,
		"total":    len(versions),
		"tenant":   tenantID,
	})
}

// handlePromptsCurrent GET /api/v1/prompts/current — 当前活跃版本
func (api *ManagementAPI) handlePromptsCurrent(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "prompt tracker not available"})
		return
	}
	current := api.promptTracker.GetCurrent()
	if current == nil {
		jsonResponse(w, 404, map[string]string{"error": "no prompt version tracked yet"})
		return
	}
	jsonResponse(w, 200, current)
}

// handlePromptsGet GET /api/v1/prompts/:hash — 单个版本详情（含安全指标）
func (api *ManagementAPI) handlePromptsGet(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "prompt tracker not available"})
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts/")
	if hash == "" {
		jsonResponse(w, 400, map[string]string{"error": "hash required"})
		return
	}
	version := api.promptTracker.GetVersion(hash)
	if version == nil {
		jsonResponse(w, 404, map[string]string{"error": "version not found"})
		return
	}
	jsonResponse(w, 200, version)
}

// handlePromptsDiff GET /api/v1/prompts/:hash/diff — 与前一版本的 diff + 指标对比
func (api *ManagementAPI) handlePromptsDiff(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "prompt tracker not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts/")
	hash := strings.TrimSuffix(path, "/diff")
	if hash == "" {
		jsonResponse(w, 400, map[string]string{"error": "hash required"})
		return
	}
	diff := api.promptTracker.GetDiff(hash)
	if diff == nil {
		jsonResponse(w, 404, map[string]string{"error": "version not found"})
		return
	}
	jsonResponse(w, 200, diff)
}

// handlePromptsTag POST /api/v1/prompts/:hash/tag — 给版本打标签
func (api *ManagementAPI) handlePromptsTag(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "prompt tracker not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts/")
	hash := strings.TrimSuffix(path, "/tag")
	if hash == "" {
		jsonResponse(w, 400, map[string]string{"error": "hash required"})
		return
	}
	var body struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tag == "" {
		jsonResponse(w, 400, map[string]string{"error": "tag required"})
		return
	}
	err := api.promptTracker.SetTag(hash, body.Tag)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "ok", "hash": hash, "tag": body.Tag})
}

// handlePromptsRollback POST /api/v1/prompts/:hash/rollback — 回滚到指定版本
func (api *ManagementAPI) handlePromptsRollback(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 404, map[string]string{"error": "prompt tracker not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/prompts/")
	hash := strings.TrimSuffix(path, "/rollback")
	if hash == "" {
		jsonResponse(w, 400, map[string]string{"error": "hash required"})
		return
	}
	err := api.promptTracker.Rollback(hash)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "ok", "hash": hash, "message": "rolled back successfully"})
}

// handlePromptsStats GET /api/v1/prompts/stats — Prompt 版本统计
func (api *ManagementAPI) handlePromptsStats(w http.ResponseWriter, r *http.Request) {
	if api.promptTracker == nil {
		jsonResponse(w, 200, map[string]interface{}{"total": 0, "active": 0, "avg_tokens": 0, "last_change": ""})
		return
	}
	stats := api.promptTracker.GetStats()
	jsonResponse(w, 200, stats)
}

// ============================================================
// v14.0 租户管理 API handlers
// ============================================================

// handleTenantList GET /api/v1/tenants — 租户列表（含概要统计）
