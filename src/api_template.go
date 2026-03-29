package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (api *ManagementAPI) handleListRuleTemplates(w http.ResponseWriter, r *http.Request) {
	templates := api.inboundEngine.ListInboundTemplates()
	type templateMeta struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		RuleCount int    `json:"rule_count"`
		Category  string `json:"category"`
		Enabled   bool   `json:"enabled"`
		BuiltIn   bool   `json:"built_in"`
	}
	var result []templateMeta
	for _, tpl := range templates {
		result = append(result, templateMeta{
			ID: tpl.ID, Name: tpl.Name, RuleCount: len(tpl.Rules),
			Category: tpl.Category, Enabled: tpl.Enabled, BuiltIn: tpl.BuiltIn,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{
		"templates": result,
		"total":     len(result),
	})
}

// handleSessionRisks GET /api/v1/sessions/risks — 列出高风险会话
func (api *ManagementAPI) handleRuleTemplateDetail(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "name parameter required"})
		return
	}
	tpl := api.inboundEngine.GetInboundTemplate(name)
	if tpl == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("模板 %q 不存在", name)})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"name":    tpl.Name,
		"id":      tpl.ID,
		"rules":   tpl.Rules,
		"total":   len(tpl.Rules),
		"enabled": tpl.Enabled,
	})
}

// ============================================================
// Demo data seed/clear API
// ============================================================

// handleDemoSeed POST /api/v1/demo/seed — 注入模拟审计数据
func (api *ManagementAPI) handleInboundTemplateList(w http.ResponseWriter, r *http.Request) {
	industryTemplates := api.listIndustryTemplates()
	templates := make([]InboundRuleTemplate, 0, len(industryTemplates))
	for _, tpl := range industryTemplates {
		templates = append(templates, InboundRuleTemplate{
			ID:          strings.Replace(tpl.ID, "tpl-", "tpl-inbound-", 1),
			Name:        tpl.Name,
			Description: tpl.Description,
			Category:    tpl.Category,
			Rules:       tpl.InboundRules,
			BuiltIn:     tpl.BuiltIn,
			Enabled:     tpl.Enabled,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"templates": templates, "total": len(templates)})
}

// handleInboundTemplateGet GET /api/v1/inbound-templates/:id — 获取单个模板详情
func (api *ManagementAPI) handleInboundTemplateGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/inbound-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	tpl := api.getIndustryTemplate(id)
	if tpl == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("template %q not found", id)})
		return
	}
	jsonResponse(w, 200, InboundRuleTemplate{
		ID:          strings.Replace(tpl.ID, "tpl-", "tpl-inbound-", 1),
		Name:        tpl.Name,
		Description: tpl.Description,
		Category:    tpl.Category,
		Rules:       tpl.InboundRules,
		BuiltIn:     tpl.BuiltIn,
		Enabled:     tpl.Enabled,
	})
}

// handleTenantBindInboundTemplate POST /api/v1/tenants/:tid/bind-inbound-template — 绑定入站模板到租户
func (api *ManagementAPI) handleInboundTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var tpl InboundRuleTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	if tpl.ID == "" || tpl.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "id 和 name 不能为空"})
		return
	}
	store := api.industryTemplateStore()
	if err := store.create(IndustryTemplate{ID: tpl.ID, Name: tpl.Name, Description: tpl.Description, Category: tpl.Category, InboundRules: tpl.Rules, BuiltIn: false}); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 201, map[string]interface{}{"status": "created", "template": tpl})
}

// handleInboundTemplateUpdate PUT /api/v1/inbound-templates/:id — 更新入站规则模板
func (api *ManagementAPI) handleInboundTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/inbound-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var tpl InboundRuleTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	store := api.industryTemplateStore()
	if err := store.update(id, IndustryTemplate{ID: id, Name: tpl.Name, Description: tpl.Description, Category: tpl.Category, InboundRules: tpl.Rules}); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": id})
}

// handleInboundTemplateDelete DELETE /api/v1/inbound-templates/:id — 删除入站规则模板（内置不可删）
func (api *ManagementAPI) handleInboundTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/inbound-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	store := api.industryTemplateStore()
	if err := store.delete(id); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// handleInboundTemplateEnable POST /api/v1/inbound-templates/:id/enable — 启用/禁用入站模板全局开关（v30.0）
func (api *ManagementAPI) handleInboundTemplateEnable(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/inbound-templates/")
	id := strings.TrimSuffix(trimmed, "/enable")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	store := api.industryTemplateStore()
	if err := store.setEnabled(id, req.Enabled); err != nil {
		if strings.Contains(err.Error(), "不存在") || strings.Contains(err.Error(), "not found") {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
		} else {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
		}
		return
	}
	api.syncIndustryTemplateEngines()
	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}
	jsonResponse(w, 200, map[string]interface{}{"status": action, "id": id, "enabled": req.Enabled})
}

// ============================================================
// v28.0 LLM 规则模板 CRUD API
// ============================================================

// handleLLMTemplateList GET /api/v1/llm/templates — 列出所有 LLM 规则模板
func (api *ManagementAPI) handleLLMTemplateList(w http.ResponseWriter, r *http.Request) {
	industryTemplates := api.listIndustryTemplates()
	templates := make([]LLMRuleTemplate, 0, len(industryTemplates))
	for _, tpl := range industryTemplates {
		templates = append(templates, LLMRuleTemplate{
			ID:          strings.Replace(tpl.ID, "tpl-", "tpl-llm-", 1),
			Name:        tpl.Name,
			Description: tpl.Description,
			Category:    tpl.Category,
			Rules:       tpl.LLMRules,
			BuiltIn:     tpl.BuiltIn,
			Enabled:     tpl.Enabled,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"templates": templates, "total": len(templates)})
}

// handleLLMTemplateGet GET /api/v1/llm/templates/:id — 获取单个 LLM 规则模板
func (api *ManagementAPI) handleLLMTemplateGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	tpl := api.getIndustryTemplate(id)
	if tpl == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("LLM 模板 %q 不存在", id)})
		return
	}
	jsonResponse(w, 200, LLMRuleTemplate{
		ID:          strings.Replace(tpl.ID, "tpl-", "tpl-llm-", 1),
		Name:        tpl.Name,
		Description: tpl.Description,
		Category:    tpl.Category,
		Rules:       tpl.LLMRules,
		BuiltIn:     tpl.BuiltIn,
		Enabled:     tpl.Enabled,
	})
}

// handleLLMTemplateCreate POST /api/v1/llm/templates — 创建自定义 LLM 规则模板
func (api *ManagementAPI) handleLLMTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var tpl LLMRuleTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	if tpl.ID == "" || tpl.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "id 和 name 不能为空"})
		return
	}
	store := api.industryTemplateStore()
	if err := store.create(IndustryTemplate{ID: tpl.ID, Name: tpl.Name, Description: tpl.Description, Category: tpl.Category, LLMRules: tpl.Rules, BuiltIn: false}); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 201, map[string]interface{}{"status": "created", "template": tpl})
}

// handleLLMTemplateUpdate PUT /api/v1/llm/templates/:id — 更新 LLM 规则模板
func (api *ManagementAPI) handleLLMTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var tpl LLMRuleTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	store := api.industryTemplateStore()
	if err := store.update(id, IndustryTemplate{ID: id, Name: tpl.Name, Description: tpl.Description, Category: tpl.Category, LLMRules: tpl.Rules}); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": id})
}

// handleLLMTemplateDelete DELETE /api/v1/llm/templates/:id — 删除 LLM 规则模板（内置不可删）
func (api *ManagementAPI) handleLLMTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	store := api.industryTemplateStore()
	if err := store.delete(id); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// handleLLMTemplateEnable POST /api/v1/llm/templates/:id/enable — 启用/禁用 LLM 模板全局开关（v30.0）
func (api *ManagementAPI) handleLLMTemplateEnable(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/templates/")
	id := strings.TrimSuffix(trimmed, "/enable")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	store := api.industryTemplateStore()
	if err := store.setEnabled(id, req.Enabled); err != nil {
		if strings.Contains(err.Error(), "不存在") || strings.Contains(err.Error(), "not found") {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
		} else {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
		}
		return
	}
	api.syncIndustryTemplateEngines()
	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}
	jsonResponse(w, 200, map[string]interface{}{"status": action, "id": id, "enabled": req.Enabled})
}

// ============================================================
// v28.0 租户 LLM 规则绑定 API
// ============================================================

// handleTenantBindLLMTemplate POST /api/v1/tenants/:tid/bind-llm-template — 绑定 LLM 规则模板到租户
