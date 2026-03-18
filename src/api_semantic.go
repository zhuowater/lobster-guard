// api_semantic.go — 语义检测引擎管理 API（v19.1）
package main

import (
	"encoding/json"
	"net/http"
)

// ============================================================
// 语义检测引擎 API
// ============================================================

// handleSemanticStats GET /api/v1/semantic/stats — 检测统计
func (api *ManagementAPI) handleSemanticStats(w http.ResponseWriter, r *http.Request) {
	if api.semanticDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "semantic detector not enabled"})
		return
	}
	stats := api.semanticDetector.Stats()
	jsonResponse(w, 200, stats)
}

// handleSemanticConfigGet GET /api/v1/semantic/config — 获取配置
func (api *ManagementAPI) handleSemanticConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.semanticDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "semantic detector not enabled"})
		return
	}
	cfg := api.semanticDetector.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleSemanticConfigPut PUT /api/v1/semantic/config — 更新配置
func (api *ManagementAPI) handleSemanticConfigPut(w http.ResponseWriter, r *http.Request) {
	if api.semanticDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "semantic detector not enabled"})
		return
	}
	var cfg SemanticConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.semanticDetector.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleSemanticAnalyze POST /api/v1/semantic/analyze — 测试分析
func (api *ManagementAPI) handleSemanticAnalyze(w http.ResponseWriter, r *http.Request) {
	if api.semanticDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "semantic detector not enabled"})
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		jsonResponse(w, 400, map[string]string{"error": "text field required"})
		return
	}
	result := api.semanticDetector.Analyze(req.Text)
	jsonResponse(w, 200, result)
}

// handleSemanticPatterns GET /api/v1/semantic/patterns — 查看攻击模式库
func (api *ManagementAPI) handleSemanticPatterns(w http.ResponseWriter, r *http.Request) {
	if api.semanticDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "semantic detector not enabled"})
		return
	}
	patterns := api.semanticDetector.ListPatterns()
	jsonResponse(w, 200, map[string]interface{}{
		"patterns": patterns,
		"total":    len(patterns),
	})
}
