package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (api *ManagementAPI) handleIndustryTemplateList(w http.ResponseWriter, r *http.Request) {
	templates := api.listIndustryTemplates()
	type templateMeta struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		Category      string `json:"category"`
		InboundCount  int    `json:"inbound_rule_count"`
		LLMCount      int    `json:"llm_rule_count"`
		OutboundCount int    `json:"outbound_rule_count"`
		Enabled       bool   `json:"enabled"`
		BuiltIn       bool   `json:"built_in"`
	}
	result := make([]templateMeta, 0, len(templates))
	for _, tpl := range templates {
		result = append(result, templateMeta{
			ID: tpl.ID, Name: tpl.Name, Description: tpl.Description, Category: tpl.Category,
			InboundCount: len(tpl.InboundRules), LLMCount: len(tpl.LLMRules), OutboundCount: len(tpl.OutboundRules),
			Enabled: tpl.Enabled, BuiltIn: tpl.BuiltIn,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{"templates": result, "total": len(result)})
}

func (api *ManagementAPI) handleIndustryTemplateGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/industry-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	id = strings.TrimSuffix(id, "/enable")
	tpl := api.getIndustryTemplate(id)
	if tpl == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("template %q not found", id)})
		return
	}
	jsonResponse(w, 200, tpl)
}

func (api *ManagementAPI) handleIndustryTemplateEnable(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/industry-templates/")
	id = strings.TrimSuffix(id, "/enable")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var req struct{ Enabled bool `json:"enabled"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.industryTemplateStore().setEnabled(id, req.Enabled); err != nil {
		if strings.Contains(err.Error(), "不存在") || strings.Contains(err.Error(), "not found") {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
		} else {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
		}
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": map[bool]string{true: "enabled", false: "disabled"}[req.Enabled], "id": normalizeIndustryTemplateID(id), "enabled": req.Enabled})
}

func (api *ManagementAPI) handleIndustryTemplateCreate(w http.ResponseWriter, r *http.Request) {
	var tpl IndustryTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	tpl.BuiltIn = false
	if err := api.industryTemplateStore().create(tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 201, map[string]interface{}{"status": "created", "template": tpl})
}

func (api *ManagementAPI) handleIndustryTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/industry-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	var tpl IndustryTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	if err := api.industryTemplateStore().update(id, tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": normalizeIndustryTemplateID(id)})
}

func (api *ManagementAPI) handleIndustryTemplateDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/industry-templates/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "template id required"})
		return
	}
	if err := api.industryTemplateStore().delete(id); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.syncIndustryTemplateEngines()
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": normalizeIndustryTemplateID(id)})
}
