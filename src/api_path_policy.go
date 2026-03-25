// api_path_policy.go — Path Policy API endpoints
// lobster-guard v23.0 + v23.1 (risk-gauge)
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (api *ManagementAPI) handlePathPolicyList(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"rules": []interface{}{}, "total": 0})
		return
	}
	tenant := r.URL.Query().Get("tenant")
	var rules []PathPolicyRule
	if tenant != "" {
		rules = api.pathPolicyEngine.ListRulesByTenant(tenant)
	} else {
		rules = api.pathPolicyEngine.ListRules()
	}
	if rules == nil { rules = []PathPolicyRule{} }
	jsonResponse(w, 200, map[string]interface{}{"rules": rules, "total": len(rules)})
}

func (api *ManagementAPI) handlePathPolicyCreate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	var rule PathPolicyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if rule.ID == "" || rule.Name == "" || rule.RuleType == "" {
		jsonResponse(w, 400, map[string]string{"error": "id, name, rule_type required"})
		return
	}
	if rule.Action == "" { rule.Action = "warn" }
	if err := api.pathPolicyEngine.AddRule(rule); err != nil {
		jsonResponse(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "rule": rule})
}

func (api *ManagementAPI) handlePathPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/path-policies/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var rule PathPolicyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	rule.ID = id
	if err := api.pathPolicyEngine.UpdateRule(rule); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": id})
}

func (api *ManagementAPI) handlePathPolicyDelete(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/path-policies/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	if err := api.pathPolicyEngine.DeleteRule(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

func (api *ManagementAPI) handlePathPolicyEvents(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"events": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	traceID := q.Get("trace_id")
	since := q.Get("since")
	if since != "" && !strings.Contains(since, "T") { since = parseSinceParam(since) }
	tenant := q.Get("tenant")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 { limit = 100 }
	events, err := api.pathPolicyEngine.QueryEvents(traceID, since, tenant, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if events == nil { events = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"events": events, "total": len(events)})
}

func (api *ManagementAPI) handlePathPolicyContexts(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"contexts": []interface{}{}, "total": 0})
		return
	}
	contexts := api.pathPolicyEngine.ListContexts()
	if contexts == nil { contexts = []PathContext{} }
	jsonResponse(w, 200, map[string]interface{}{"contexts": contexts, "total": len(contexts)})
}

func (api *ManagementAPI) handlePathPolicyContextDetail(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 404, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/path-policies/contexts/")
	if traceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id required"})
		return
	}
	ctx := api.pathPolicyEngine.GetContext(traceID)
	if ctx == nil {
		jsonResponse(w, 404, map[string]string{"error": "context not found"})
		return
	}
	jsonResponse(w, 200, ctx)
}

func (api *ManagementAPI) handlePathPolicyStats(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, PathPolicyStats{})
		return
	}
	jsonResponse(w, 200, api.pathPolicyEngine.Stats())
}

// v23.1: GET /api/v1/path-policies/risk-gauge — 实时风险仪表数据
// 返回每个活跃路径的 trace_id/session_id/risk_score/step_count/taint_count
// 用于 Dashboard 飞行高度表实时刷新
func (api *ManagementAPI) handlePathPolicyRiskGauge(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"gauges": []interface{}{}, "total": 0})
		return
	}
	contexts := api.pathPolicyEngine.ListContexts()
	gauges := make([]map[string]interface{}, 0, len(contexts))
	for _, ctx := range contexts {
		gauges = append(gauges, map[string]interface{}{
			"trace_id":    ctx.TraceID,
			"session_id":  ctx.SessionID,
			"tenant_id":   ctx.TenantID,
			"risk_score":  ctx.RiskScore,
			"step_count":  len(ctx.Steps),
			"tool_count":  len(ctx.ToolHistory),
			"taint_count": len(ctx.TaintLabels),
			"taint_labels": ctx.TaintLabels,
			"last_action": lastAction(ctx.Steps),
			"age_sec":     int(time.Since(ctx.CreatedAt).Seconds()),
		})
	}
	// 按风险分降序
	sortGauges(gauges)
	jsonResponse(w, 200, map[string]interface{}{"gauges": gauges, "total": len(gauges)})
}

func lastAction(steps []PathStep) string {
	if len(steps) == 0 { return "" }
	return steps[len(steps)-1].Action
}

func sortGauges(gs []map[string]interface{}) {
	for i := 0; i < len(gs); i++ {
		for j := i + 1; j < len(gs); j++ {
			si, _ := gs[i]["risk_score"].(float64)
			sj, _ := gs[j]["risk_score"].(float64)
			if sj > si { gs[i], gs[j] = gs[j], gs[i] }
		}
	}
}

// v23.2 CRUD: GET /api/v1/path-policies/templates — 模板列表
func (api *ManagementAPI) handlePathPolicyTemplates(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"templates": []interface{}{}, "total": 0})
		return
	}
	templates := api.pathPolicyEngine.ListTemplates()
	if templates == nil { templates = []PolicyTemplate{} }
	jsonResponse(w, 200, map[string]interface{}{"templates": templates, "total": len(templates)})
}

// POST /api/v1/path-policies/templates — 创建模板
func (api *ManagementAPI) handlePathPolicyTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	var t PolicyTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if t.ID == "" || t.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "id and name required"})
		return
	}
	if err := api.pathPolicyEngine.AddTemplate(t); err != nil {
		jsonResponse(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "template": t})
}

// PUT /api/v1/path-policies/templates/:id — 更新模板
func (api *ManagementAPI) handlePathPolicyTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := extractTemplateID(r.URL.Path)
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var t PolicyTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	t.ID = id
	if err := api.pathPolicyEngine.UpdateTemplate(t); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": id})
}

// DELETE /api/v1/path-policies/templates/:id — 删除模板（内置模板不可删）
func (api *ManagementAPI) handlePathPolicyTemplateDelete(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := extractTemplateID(r.URL.Path)
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	if err := api.pathPolicyEngine.DeleteTemplate(id); err != nil {
		status := 404
		if strings.Contains(err.Error(), "built-in") { status = 403 }
		jsonResponse(w, status, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// POST /api/v1/path-policies/templates/:id/activate — 激活模板中的规则
func (api *ManagementAPI) handlePathPolicyTemplateActivate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/path-policies/templates/")
	id = strings.TrimSuffix(id, "/activate")
	n, err := api.pathPolicyEngine.ActivateTemplate(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "activated", "template": id, "rules_enabled": n})
}

// POST /api/v1/path-policies/templates/:id/deactivate — 停用模板中的规则
func (api *ManagementAPI) handlePathPolicyTemplateDeactivate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/path-policies/templates/")
	id = strings.TrimSuffix(id, "/deactivate")
	n, err := api.pathPolicyEngine.DeactivateTemplate(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deactivated", "template": id, "rules_disabled": n})
}

func extractTemplateID(path string) string {
	s := strings.TrimPrefix(path, "/api/v1/path-policies/templates/")
	if i := strings.Index(s, "/"); i >= 0 { s = s[:i] }
	return s
}
