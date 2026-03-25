// api_cf_adaptive.go — 自适应验证策略 API 端点
// lobster-guard v24.2
package main

import (
	"encoding/json"
	"net/http"
)

// handleCFAdaptiveCost GET /api/v1/counterfactual/cost — 成本摘要 + 每日趋势
func (api *ManagementAPI) handleCFAdaptiveCost(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveStrategy == nil {
		jsonResponse(w, 200, CostSummary{})
		return
	}
	summary := api.adaptiveStrategy.GetCostSummary()
	jsonResponse(w, 200, summary)
}

// handleCFAdaptiveEffectiveness GET /api/v1/counterfactual/effectiveness — 效果指标
func (api *ManagementAPI) handleCFAdaptiveEffectiveness(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveStrategy == nil {
		jsonResponse(w, 200, EffectTracker{})
		return
	}
	metrics := api.adaptiveStrategy.GetEffectMetrics()
	jsonResponse(w, 200, metrics)
}

// handleCFAdaptiveFeedback POST /api/v1/counterfactual/feedback — 提交人类反馈
func (api *ManagementAPI) handleCFAdaptiveFeedback(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveStrategy == nil {
		jsonResponse(w, 400, map[string]string{"error": "adaptive strategy not initialized"})
		return
	}

	var req struct {
		VerificationID string `json:"verification_id"`
		WasCorrect     bool   `json:"was_correct"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.VerificationID == "" {
		jsonResponse(w, 400, map[string]string{"error": "verification_id required"})
		return
	}

	if err := api.adaptiveStrategy.RecordFeedback(req.VerificationID, req.WasCorrect); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"status":          "recorded",
		"verification_id": req.VerificationID,
		"was_correct":     req.WasCorrect,
	})
}

// handleCFAdaptiveConfigGet GET /api/v1/counterfactual/adaptive-config — 获取自适应配置
func (api *ManagementAPI) handleCFAdaptiveConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveStrategy == nil {
		jsonResponse(w, 200, defaultAdaptiveConfig)
		return
	}
	jsonResponse(w, 200, api.adaptiveStrategy.GetConfig())
}

// handleCFAdaptiveConfigUpdate PUT /api/v1/counterfactual/adaptive-config — 更新自适应配置
func (api *ManagementAPI) handleCFAdaptiveConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.adaptiveStrategy == nil {
		jsonResponse(w, 400, map[string]string{"error": "adaptive strategy not initialized"})
		return
	}

	var cfg AdaptiveConfig
	if json.NewDecoder(r.Body).Decode(&cfg) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	api.adaptiveStrategy.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"config": api.adaptiveStrategy.GetConfig(),
	})
}
