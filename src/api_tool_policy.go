// api_tool_policy.go — Tool Policy 管理 API
// lobster-guard v20.0
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleToolPolicyStats GET /api/v1/tools/stats
func (api *ManagementAPI) handleToolPolicyStats(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":      false,
			"total_events": 0,
			"total_rules":  0,
		})
		return
	}
	stats := api.toolPolicy.Stats()
	jsonResponse(w, 200, stats)
}

// handleToolPolicyEvents GET /api/v1/tools/events
func (api *ManagementAPI) handleToolPolicyEvents(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{"events": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	toolName := q.Get("tool")
	decision := q.Get("decision")
	risk := q.Get("risk")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	events, total, err := api.toolPolicy.QueryEvents(toolName, decision, risk, limit, offset)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if events == nil {
		events = []map[string]interface{}{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// handleToolPolicyRulesList GET /api/v1/tools/rules
func (api *ManagementAPI) handleToolPolicyRulesList(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{"rules": []interface{}{}, "total": 0})
		return
	}
	rules := api.toolPolicy.ListRules()
	jsonResponse(w, 200, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// handleToolPolicyRulesCreate POST /api/v1/tools/rules
func (api *ManagementAPI) handleToolPolicyRulesCreate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var rule ToolPolicyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.toolPolicy.AddRule(rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"rule":   rule,
	})
}

// handleToolPolicyRulesUpdate PUT /api/v1/tools/rules/:id
func (api *ManagementAPI) handleToolPolicyRulesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	// Extract ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing rule id"})
		return
	}
	ruleID := parts[len(parts)-1]

	var rule ToolPolicyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	rule.ID = ruleID
	if err := api.toolPolicy.UpdateRule(rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"rule":   rule,
	})
}

// handleToolPolicyRulesDelete DELETE /api/v1/tools/rules/:id
func (api *ManagementAPI) handleToolPolicyRulesDelete(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing rule id"})
		return
	}
	ruleID := parts[len(parts)-1]
	if err := api.toolPolicy.RemoveRule(ruleID); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "deleted",
		"id":     ruleID,
	})
}

// handleToolPolicyConfigGet GET /api/v1/tools/config
func (api *ManagementAPI) handleToolPolicyConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":          false,
			"default_action":   "allow",
			"max_calls_per_min": 60,
		})
		return
	}
	cfg := api.toolPolicy.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleToolPolicyConfigUpdate PUT /api/v1/tools/config
func (api *ManagementAPI) handleToolPolicyConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var cfg ToolPolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	api.toolPolicy.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"config": cfg,
	})
}

// handleToolPolicyEvaluate POST /api/v1/tools/evaluate
func (api *ManagementAPI) handleToolPolicyEvaluate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var req struct {
		ToolName  string `json:"tool_name"`
		Arguments string `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	event := api.toolPolicy.Evaluate(req.ToolName, req.Arguments, "test-eval", "test")
	jsonResponse(w, 200, event)
}
