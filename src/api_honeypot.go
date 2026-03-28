package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleCanaryStatus(w http.ResponseWriter, r *http.Request) {
	cfg := api.cfg.LLMProxy.Security.CanaryToken
	result := map[string]interface{}{
		"enabled":      cfg.Enabled,
		"auto_rotate":  cfg.AutoRotate,
		"alert_action": cfg.AlertAction,
	}
	// 脱敏显示 token
	if cfg.Token != "" {
		if len(cfg.Token) > 20 {
			result["token"] = cfg.Token[:20] + "..."
		} else {
			result["token"] = cfg.Token
		}
	}
	// 查询泄露统计
	if api.llmAuditor != nil {
		canaryStats := api.llmAuditor.CanaryStatus()
		result["leak_count"] = canaryStats["leak_count"]
		result["last_leak"] = canaryStats["last_leak"]
	} else {
		result["leak_count"] = 0
		result["last_leak"] = ""
	}
	jsonResponse(w, 200, result)
}

// handleCanaryRotate POST /api/v1/llm/canary/rotate — 手动轮换 Token
func (api *ManagementAPI) handleCanaryRotate(w http.ResponseWriter, r *http.Request) {
	if api.llmProxy == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM proxy not enabled"})
		return
	}
	newToken := api.llmProxy.RotateCanaryToken()
	api.cfg.LLMProxy.Security.CanaryToken.Token = newToken
	// 写回配置文件
	if err := api.saveLLMConfig(); err != nil {
		log.Printf("[Canary] 持久化新 token 失败: %v", err)
	}
	// 脱敏
	display := newToken
	if len(display) > 20 {
		display = display[:20] + "..."
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "rotated",
		"token":  display,
	})
}

// handleCanaryLeaks GET /api/v1/llm/canary/leaks — 泄露事件列表
func (api *ManagementAPI) handleCanaryLeaks(w http.ResponseWriter, r *http.Request) {
	if api.llmAuditor == nil {
		jsonResponse(w, 200, map[string]interface{}{"records": []interface{}{}, "total": 0})
		return
	}
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil { limit = n }
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil { offset = n }
	}
	records, total, err := api.llmAuditor.QueryCanaryLeaks(limit, offset)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if records == nil { records = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"records": records, "total": total})
}

// ============================================================
// v10.1 Response Budget API
// ============================================================

// handleBudgetStatus GET /api/v1/llm/budget/status — Budget 配置和统计
func (api *ManagementAPI) handleHoneypotTemplateList(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		tenantID = "all"
	}
	templates, err := api.honeypotEngine.ListTemplates(tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, templates)
}

func (api *ManagementAPI) handleHoneypotTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	var tpl HoneypotTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	if err := api.honeypotEngine.CreateTemplate(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 201, tpl)
}

func (api *ManagementAPI) handleHoneypotTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/honeypot/templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing template id"})
		return
	}
	var tpl HoneypotTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	tpl.ID = id
	if err := api.honeypotEngine.UpdateTemplate(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, tpl)
}

func (api *ManagementAPI) handleHoneypotTemplateDelete(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/honeypot/templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing template id"})
		return
	}
	if err := api.honeypotEngine.DeleteTemplate(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted"})
}

func (api *ManagementAPI) handleHoneypotTriggerList(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		tenantID = "all"
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	triggers, err := api.honeypotEngine.ListTriggers(tenantID, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, triggers)
}

func (api *ManagementAPI) handleHoneypotTriggerGet(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/honeypot/triggers/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing trigger id"})
		return
	}
	trigger, err := api.honeypotEngine.GetTrigger(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "trigger not found"})
		return
	}
	jsonResponse(w, 200, trigger)
}

func (api *ManagementAPI) handleHoneypotStats(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		tenantID = "all"
	}
	stats := api.honeypotEngine.GetStats(tenantID)
	jsonResponse(w, 200, stats)
}

func (api *ManagementAPI) handleHoneypotTest(w http.ResponseWriter, r *http.Request) {
	if api.honeypotEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "honeypot engine not available"})
		return
	}
	var req struct {
		Text     string `json:"text"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Text == "" {
		jsonResponse(w, 400, map[string]string{"error": "text is required"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = "all"
	}
	result := api.honeypotEngine.TestHoneypot(req.Text, req.TenantID)
	jsonResponse(w, 200, result)
}


// ============================================================
// v15.1 A/B 测试 API handlers
// ============================================================

// handleABTestList GET /api/v1/ab-tests — 测试列表
