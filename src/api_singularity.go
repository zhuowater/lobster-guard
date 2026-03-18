// api_singularity.go — 自适应决策 + 奇点蜜罐管理 API（v18.3）
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ============================================================
// 自适应决策 API
// ============================================================

// handleAdaptiveStats GET /api/v1/adaptive/stats — 统计
func (api *ManagementAPI) handleAdaptiveStats(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "adaptive decision engine not enabled"})
		return
	}
	stats := api.adaptiveEngine.GetStats()
	jsonResponse(w, 200, stats)
}

// handleAdaptiveProof GET /api/v1/adaptive/proof/:user_id — 指定用户的贝叶斯证明
func (api *ManagementAPI) handleAdaptiveProof(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "adaptive decision engine not enabled"})
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/api/v1/adaptive/proof/")
	if userID == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id required"})
		return
	}
	proof := api.adaptiveEngine.GetProof(userID)
	jsonResponse(w, 200, proof)
}

// handleAdaptiveFeedback POST /api/v1/adaptive/feedback — 记录人工反馈
func (api *ManagementAPI) handleAdaptiveFeedback(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "adaptive decision engine not enabled"})
		return
	}
	var req struct {
		UserID          string `json:"user_id"`
		Action          string `json:"action"`
		WasFalsePositive bool  `json:"was_false_positive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.UserID == "" || req.Action == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id and action required"})
		return
	}
	if err := api.adaptiveEngine.RecordOutcome(req.UserID, req.Action, req.WasFalsePositive); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleAdaptiveConfigGet GET /api/v1/adaptive/config — 获取配置
func (api *ManagementAPI) handleAdaptiveConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "adaptive decision engine not enabled"})
		return
	}
	cfg := api.adaptiveEngine.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleAdaptiveConfigPut PUT /api/v1/adaptive/config — 更新配置
func (api *ManagementAPI) handleAdaptiveConfigPut(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "adaptive decision engine not enabled"})
		return
	}
	var cfg AdaptiveDecisionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.adaptiveEngine.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// ============================================================
// 奇点蜜罐 API
// ============================================================

// handleSingularityConfigGet GET /api/v1/singularity/config — 获取配置
func (api *ManagementAPI) handleSingularityConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	cfg := api.singularityEngine.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleSingularityConfigPut PUT /api/v1/singularity/config — 设置各通道暴露等级
func (api *ManagementAPI) handleSingularityConfigPut(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	var cfg SingularityConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.singularityEngine.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleSingularityTemplates GET /api/v1/singularity/templates — 暴露模板列表
func (api *ManagementAPI) handleSingularityTemplates(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	channel := r.URL.Query().Get("channel")
	levelStr := r.URL.Query().Get("level")

	if channel != "" && levelStr != "" {
		level, _ := strconv.Atoi(levelStr)
		templates := api.singularityEngine.GetExposureTemplates(channel, level)
		jsonResponse(w, 200, templates)
		return
	}

	templates := api.singularityEngine.GetAllTemplates()
	jsonResponse(w, 200, templates)
}

// handleSingularityRecommend GET /api/v1/singularity/recommend — 推荐最优放置
func (api *ManagementAPI) handleSingularityRecommend(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	proof := api.singularityEngine.RecommendPlacement()
	jsonResponse(w, 200, proof)
}

// handleSingularityBudget GET /api/v1/singularity/budget — 奇点预算
func (api *ManagementAPI) handleSingularityBudget(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	budget := api.singularityEngine.GetBudget()
	jsonResponse(w, 200, budget)
}

// handleSingularityHistory GET /api/v1/singularity/history — 历史记录
func (api *ManagementAPI) handleSingularityHistory(w http.ResponseWriter, r *http.Request) {
	if api.singularityEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "singularity engine not enabled"})
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	history := api.singularityEngine.GetHistory(limit)
	if history == nil {
		history = []SingularityHistoryEntry{}
	}
	jsonResponse(w, 200, history)
}
