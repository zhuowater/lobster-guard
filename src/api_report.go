package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleReportGenerate(w http.ResponseWriter, r *http.Request) {
	if api.reportEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "report engine not initialized"})
		return
	}
	var body struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	var rt ReportType
	switch body.Type {
	case "daily":
		rt = ReportDaily
	case "weekly":
		rt = ReportWeekly
	case "monthly":
		rt = ReportMonthly
	default:
		jsonResponse(w, 400, map[string]string{"error": "type must be daily, weekly, or monthly"})
		return
	}
	meta, err := api.reportEngine.Generate(rt)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, meta)
}

// handleReportList GET /api/v1/reports — 报告列表
func (api *ManagementAPI) handleReportList(w http.ResponseWriter, r *http.Request) {
	if api.reportEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"reports": []interface{}{}})
		return
	}
	typ := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	reports, err := api.reportEngine.ListReports(typ, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if reports == nil {
		reports = []ReportMeta{}
	}
	jsonResponse(w, 200, map[string]interface{}{"reports": reports})
}

// handleReportGet GET /api/v1/reports/:id — 获取报告元数据
func (api *ManagementAPI) handleReportGet(w http.ResponseWriter, r *http.Request) {
	if api.reportEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "not found"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/reports/")
	meta, err := api.reportEngine.GetReport(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "report not found"})
		return
	}
	jsonResponse(w, 200, meta)
}

// handleReportDownload GET /api/v1/reports/:id/download — 下载报告 HTML
func (api *ManagementAPI) handleReportDownload(w http.ResponseWriter, r *http.Request) {
	if api.reportEngine == nil {
		http.Error(w, "not found", 404)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/reports/")
	id = strings.TrimSuffix(id, "/download")
	meta, err := api.reportEngine.GetReport(id)
	if err != nil {
		http.Error(w, "report not found", 404)
		return
	}
	data, err := os.ReadFile(meta.FilePath)
	if err != nil {
		http.Error(w, "report file not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s.html\"", meta.ID))
	w.Write(data)
}

// handleReportDelete DELETE /api/v1/reports/:id — 删除报告
func (api *ManagementAPI) handleReportDelete(w http.ResponseWriter, r *http.Request) {
	if api.reportEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "not found"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/reports/")
	if err := api.reportEngine.DeleteReport(id); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": true})
}

// handleLLMExport GET /api/v1/llm/export — 导出 LLM 审计数据（CSV/JSON）
func (api *ManagementAPI) handleRedTeamRun(w http.ResponseWriter, r *http.Request) {
	if api.redTeamEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "red team engine not available"})
		return
	}
	var req struct {
		TenantID string `json:"tenant_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.TenantID == "" {
		req.TenantID = "default"
	}

	report, err := api.redTeamEngine.RunAttack(req.TenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, report)
}

// handleRedTeamReportList GET /api/v1/redteam/reports — 报告列表
func (api *ManagementAPI) handleRedTeamReportList(w http.ResponseWriter, r *http.Request) {
	if api.redTeamEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "red team engine not available"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	reports, err := api.redTeamEngine.ListReports(tenantID, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"reports": reports, "total": len(reports)})
}

// handleRedTeamReportGet GET /api/v1/redteam/reports/:id — 报告详情
func (api *ManagementAPI) handleRedTeamReportGet(w http.ResponseWriter, r *http.Request) {
	if api.redTeamEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "red team engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/redteam/reports/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "report id required"})
		return
	}

	report, err := api.redTeamEngine.GetReport(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, report)
}

// handleRedTeamReportDelete DELETE /api/v1/redteam/reports/:id — 删除报告
func (api *ManagementAPI) handleRedTeamReportDelete(w http.ResponseWriter, r *http.Request) {
	if api.redTeamEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "red team engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/redteam/reports/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "report id required"})
		return
	}

	if err := api.redTeamEngine.DeleteReport(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// handleRedTeamVectors GET /api/v1/redteam/vectors — 攻击向量库
func (api *ManagementAPI) handleRedTeamVectors(w http.ResponseWriter, r *http.Request) {
	if api.redTeamEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "red team engine not available"})
		return
	}
	vectors := api.redTeamEngine.GetAttackVectors()
	jsonResponse(w, 200, map[string]interface{}{"vectors": vectors, "total": len(vectors)})
}

// ============================================================
// v14.3 排行榜 + SLA API handlers
// ============================================================

// handleLeaderboard GET /api/v1/leaderboard — 安全排行榜
func (api *ManagementAPI) handleABTestList(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"tests": []interface{}{}, "total": 0})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	status := r.URL.Query().Get("status")
	tests, err := api.abTestEngine.List(tenantID, status)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if tests == nil {
		tests = []*ABTest{}
	}
	jsonResponse(w, 200, map[string]interface{}{"tests": tests, "total": len(tests)})
}

// handleABTestCreate POST /api/v1/ab-tests — 创建测试
func (api *ManagementAPI) handleABTestCreate(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	var req ABTest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	if err := api.abTestEngine.Create(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, req)
}

// handleABTestGet GET /api/v1/ab-tests/:id — 测试详情
func (api *ManagementAPI) handleABTestGet(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/ab-tests/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	test, err := api.abTestEngine.Get(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, test)
}

// handleABTestUpdate PUT /api/v1/ab-tests/:id — 更新测试
func (api *ManagementAPI) handleABTestUpdate(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/ab-tests/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		TrafficA    int    `json:"traffic_a"`
		VersionA    string `json:"version_a"`
		PromptHashA string `json:"prompt_hash_a"`
		VersionB    string `json:"version_b"`
		PromptHashB string `json:"prompt_hash_b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := api.abTestEngine.Update(id, req.Name, req.TrafficA, req.VersionA, req.PromptHashA, req.VersionB, req.PromptHashB); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	test, _ := api.abTestEngine.Get(id)
	jsonResponse(w, 200, test)
}

// handleABTestStart POST /api/v1/ab-tests/:id/start — 开始测试
func (api *ManagementAPI) handleABTestStart(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ab-tests/")
	id := strings.TrimSuffix(path, "/start")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	if err := api.abTestEngine.Start(id); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	test, _ := api.abTestEngine.Get(id)
	jsonResponse(w, 200, map[string]interface{}{"status": "started", "test": test})
}

// handleABTestStop POST /api/v1/ab-tests/:id/stop — 停止测试
func (api *ManagementAPI) handleABTestStop(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/ab-tests/")
	id := strings.TrimSuffix(path, "/stop")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	test, err := api.abTestEngine.Stop(id)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "completed", "test": test})
}

// handleABTestDelete DELETE /api/v1/ab-tests/:id — 删除测试
func (api *ManagementAPI) handleABTestDelete(w http.ResponseWriter, r *http.Request) {
	if api.abTestEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "A/B test engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/ab-tests/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	if err := api.abTestEngine.Delete(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// ============================================================
// v16.1 攻击链分析 API
// ============================================================

// handleAttackChainList GET /api/v1/attack-chains
