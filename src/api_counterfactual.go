// api_counterfactual.go — 反事实验证引擎 API 端点
// lobster-guard v24.0
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleCFStats GET /api/v1/counterfactual/stats — 验证统计
func (api *ManagementAPI) handleCFStats(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled": false,
			"stats":   CFStats{},
		})
		return
	}
	stats := api.cfVerifier.GetStats()
	cfg := api.cfVerifier.GetConfig()
	jsonResponse(w, 200, map[string]interface{}{
		"enabled": cfg.Enabled,
		"mode":    cfg.Mode,
		"stats":   stats,
	})
}

// handleCFVerifications GET /api/v1/counterfactual/verifications — 验证记录列表
func (api *ManagementAPI) handleCFVerifications(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"verifications": []interface{}{},
			"total":         0,
		})
		return
	}
	traceID := r.URL.Query().Get("trace_id")
	verdict := r.URL.Query().Get("verdict")
	since := r.URL.Query().Get("since")
	if since != "" && len(since) <= 3 {
		since = parseSinceParam(since)
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	results := api.cfVerifier.QueryVerifications(traceID, verdict, since, limit)
	if results == nil {
		results = []CFVerification{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"verifications": results,
		"total":         len(results),
	})
}

// handleCFVerificationGet GET /api/v1/counterfactual/verifications/:id — 单条详情
func (api *ManagementAPI) handleCFVerificationGet(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 404, map[string]string{"error": "counterfactual verifier not enabled"})
		return
	}
	// 从 path 提取 id: /api/v1/counterfactual/verifications/{id}
	id := r.URL.Path[len("/api/v1/counterfactual/verifications/"):]
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	vf := api.cfVerifier.GetVerification(id)
	if vf == nil {
		jsonResponse(w, 404, map[string]string{"error": "verification not found"})
		return
	}
	jsonResponse(w, 200, vf)
}

// handleCFConfigGet GET /api/v1/counterfactual/config — 获取配置
func (api *ManagementAPI) handleCFConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, defaultCFConfig)
		return
	}
	jsonResponse(w, 200, api.cfVerifier.GetConfig())
}

// handleCFConfigUpdate PUT /api/v1/counterfactual/config — 更新配置
func (api *ManagementAPI) handleCFConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 400, map[string]string{"error": "counterfactual verifier not initialized"})
		return
	}
	var cfg CFConfig
	if json.NewDecoder(r.Body).Decode(&cfg) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.cfVerifier.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"config": api.cfVerifier.GetConfig(),
	})
}

// handleCFCacheGet GET /api/v1/counterfactual/cache — 缓存状态
func (api *ManagementAPI) handleCFCacheGet(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"total_entries": 0,
			"valid_entries": 0,
		})
		return
	}
	jsonResponse(w, 200, api.cfVerifier.GetCacheStats())
}

// handleCFCacheClear DELETE /api/v1/counterfactual/cache — 清除缓存
func (api *ManagementAPI) handleCFCacheClear(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{"status": "no_cache", "cleared": 0})
		return
	}
	n := api.cfVerifier.ClearCache()
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "cleared",
		"cleared": n,
	})
}
