// api_counterfactual.go — 反事实验证引擎 API 端点
// lobster-guard v24.0
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
		cfg := defaultCFConfig
		// 返回默认高风险工具列表
		tools := make([]string, 0, len(defaultHighRiskTools))
		for k := range defaultHighRiskTools {
			tools = append(tools, k)
		}
		cfg.HighRiskTools = tools
		jsonResponse(w, 200, cfg)
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
	// 如果请求中包含 high_risk_tools 字段，批量设置
	if cfg.HighRiskTools != nil {
		api.cfVerifier.SetHighRiskTools(cfg.HighRiskTools)
	}
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

// ============================================================
// v24.1 归因报告 API
// ============================================================

// handleCFReports GET /api/v1/counterfactual/reports — 归因报告列表
func (api *ManagementAPI) handleCFReports(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"reports": []interface{}{},
			"total":   0,
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
	results := api.cfVerifier.QueryAttributionReports(traceID, verdict, since, limit)
	if results == nil {
		results = []AttributionReport{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"reports": results,
		"total":   len(results),
	})
}

// handleCFReportGet GET /api/v1/counterfactual/reports/:id — 归因报告详情
func (api *ManagementAPI) handleCFReportGet(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 404, map[string]string{"error": "counterfactual verifier not enabled"})
		return
	}
	id := r.URL.Path[len("/api/v1/counterfactual/reports/"):]
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	report := api.cfVerifier.GetAttributionReport(id)
	if report == nil {
		jsonResponse(w, 404, map[string]string{"error": "report not found"})
		return
	}
	jsonResponse(w, 200, report)
}

// handleCFTimeline GET /api/v1/counterfactual/timeline — 因果归因时间线
func (api *ManagementAPI) handleCFTimeline(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"events": []interface{}{},
			"total":  0,
		})
		return
	}
	since := r.URL.Query().Get("since")
	if since != "" && len(since) <= 3 {
		since = parseSinceParam(since)
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events := api.cfVerifier.QueryTimeline(since, limit)
	if events == nil {
		events = []CFTimelineEvent{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

// ============================================================
// 高风险工具 CRUD API
// ============================================================

// handleCFHighRiskToolsList GET /api/v1/counterfactual/high-risk-tools — 列出高风险工具
func (api *ManagementAPI) handleCFHighRiskToolsList(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		// 返回默认列表
		tools := make([]string, 0, len(defaultHighRiskTools))
		for k := range defaultHighRiskTools {
			tools = append(tools, k)
		}
		jsonResponse(w, 200, map[string]interface{}{
			"tools": tools,
			"total": len(tools),
		})
		return
	}
	tools := api.cfVerifier.GetHighRiskTools()
	jsonResponse(w, 200, map[string]interface{}{
		"tools": tools,
		"total": len(tools),
	})
}

// handleCFHighRiskToolsAdd POST /api/v1/counterfactual/high-risk-tools — 添加高风险工具
func (api *ManagementAPI) handleCFHighRiskToolsAdd(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 400, map[string]string{"error": "counterfactual verifier not initialized"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		jsonResponse(w, 400, map[string]string{"error": "name is required"})
		return
	}
	name := strings.TrimSpace(req.Name)
	api.cfVerifier.AddHighRiskTool(name)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "added",
		"name":   name,
		"tools":  api.cfVerifier.GetHighRiskTools(),
	})
}

// handleCFHighRiskToolsDelete DELETE /api/v1/counterfactual/high-risk-tools/:name — 删除高风险工具
func (api *ManagementAPI) handleCFHighRiskToolsDelete(w http.ResponseWriter, r *http.Request) {
	if api.cfVerifier == nil {
		jsonResponse(w, 400, map[string]string{"error": "counterfactual verifier not initialized"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/counterfactual/high-risk-tools/")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "tool name required"})
		return
	}
	api.cfVerifier.RemoveHighRiskTool(name)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "removed",
		"name":   name,
		"tools":  api.cfVerifier.GetHighRiskTools(),
	})
}
