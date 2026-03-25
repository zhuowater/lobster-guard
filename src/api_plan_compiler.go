// api_plan_compiler.go — Plan Compiler management API (v25.0)
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handlePlanTemplatesList GET /api/v1/plan/templates
func (api *ManagementAPI) handlePlanTemplatesList(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 200, map[string]interface{}{"templates": []interface{}{}, "total": 0})
		return
	}
	templates := api.planCompiler.ListTemplates()
	jsonResponse(w, 200, map[string]interface{}{"templates": templates, "total": len(templates)})
}

// handlePlanTemplatesCreate POST /api/v1/plan/templates
func (api *ManagementAPI) handlePlanTemplatesCreate(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 503, map[string]string{"error": "plan compiler not enabled"})
		return
	}
	var tpl PlanTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result, err := api.planCompiler.AddTemplate(tpl)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 201, result)
}

// handlePlanTemplatesUpdate PUT /api/v1/plan/templates/:id
func (api *ManagementAPI) handlePlanTemplatesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 503, map[string]string{"error": "plan compiler not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/templates/")
	var tpl PlanTemplate
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := api.planCompiler.UpdateTemplate(id, tpl); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handlePlanTemplatesDelete DELETE /api/v1/plan/templates/:id
func (api *ManagementAPI) handlePlanTemplatesDelete(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 503, map[string]string{"error": "plan compiler not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/templates/")
	if err := api.planCompiler.DeleteTemplate(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted"})
}

// handlePlanActive GET /api/v1/plan/active
func (api *ManagementAPI) handlePlanActive(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 200, map[string]interface{}{"plans": []interface{}{}, "total": 0})
		return
	}
	plans, total := api.planCompiler.QueryPlans("active", 100, 0)
	jsonResponse(w, 200, map[string]interface{}{"plans": plans, "total": total})
}

// handlePlanHistory GET /api/v1/plan/history
func (api *ManagementAPI) handlePlanHistory(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 200, map[string]interface{}{"plans": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	plans, total := api.planCompiler.QueryPlans(status, limit, offset)
	jsonResponse(w, 200, map[string]interface{}{"plans": plans, "total": total, "limit": limit, "offset": offset})
}

// handlePlanViolations GET /api/v1/plan/violations
func (api *ManagementAPI) handlePlanViolations(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 200, map[string]interface{}{"violations": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	traceID := q.Get("trace_id")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	viols, total := api.planCompiler.QueryViolations(traceID, limit, offset)
	jsonResponse(w, 200, map[string]interface{}{"violations": viols, "total": total})
}

// handlePlanStats GET /api/v1/plan/stats
func (api *ManagementAPI) handlePlanStats(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 200, map[string]interface{}{"total_plans": 0, "template_count": 0})
		return
	}
	stats := api.planCompiler.GetStats()
	jsonResponse(w, 200, stats)
}

// handlePlanGet GET /api/v1/plans/:traceId (catch-all after other plan routes)
func (api *ManagementAPI) handlePlanGet(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 404, map[string]string{"error": "not found"})
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/")
	plan := api.planCompiler.GetPlan(traceID)
	if plan == nil {
		jsonResponse(w, 404, map[string]string{"error": "plan not found"})
		return
	}
	jsonResponse(w, 200, plan)
}

// handlePlanConfigUpdate PUT /api/v1/plans/config
func (api *ManagementAPI) handlePlanConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.planCompiler == nil {
		jsonResponse(w, 503, map[string]string{"error": "plan compiler not enabled"})
		return
	}
	var cfg PlanConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	api.planCompiler.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}
