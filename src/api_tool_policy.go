// api_tool_policy.go — Tool Policy 管理 API
// lobster-guard v20.0
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func mergeToolPolicyRulePatch(existing ToolPolicyRule, patch map[string]json.RawMessage) (ToolPolicyRule, error) {
	merged := existing
	for key, raw := range patch {
		switch key {
		case "id":
			// ignore path/body id overrides
		case "name":
			if err := json.Unmarshal(raw, &merged.Name); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid name: %w", err)
			}
		case "tool_pattern":
			if err := json.Unmarshal(raw, &merged.ToolPattern); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid tool_pattern: %w", err)
			}
		case "param_rules":
			if err := json.Unmarshal(raw, &merged.ParamRules); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid param_rules: %w", err)
			}
		case "action":
			if err := json.Unmarshal(raw, &merged.Action); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid action: %w", err)
			}
		case "reason":
			if err := json.Unmarshal(raw, &merged.Reason); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid reason: %w", err)
			}
		case "enabled":
			if err := json.Unmarshal(raw, &merged.Enabled); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid enabled: %w", err)
			}
		case "priority":
			if err := json.Unmarshal(raw, &merged.Priority); err != nil {
				return ToolPolicyRule{}, fmt.Errorf("invalid priority: %w", err)
			}
		}
	}
	return merged, nil
}

func findToolPolicyRuleByID(rules []ToolPolicyRule, id string) (ToolPolicyRule, bool) {
	for _, rule := range rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return ToolPolicyRule{}, false
}

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
	semanticClass := q.Get("semantic_class")
	contextSignal := q.Get("context_signal")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	events, total, err := api.toolPolicy.QueryEvents(toolName, decision, risk, semanticClass, contextSignal, limit, offset)
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

	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	existing, ok := findToolPolicyRuleByID(api.toolPolicy.ListRules(), ruleID)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "rule not found"})
		return
	}
	rule, err := mergeToolPolicyRulePatch(existing, patch)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
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
			"enabled":           false,
			"default_action":    "allow",
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

// handleToolSemanticRulesList GET /api/v1/tools/semantic-rules
func (api *ManagementAPI) handleToolSemanticRulesList(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{"rules": []interface{}{}, "total": 0})
		return
	}
	rules := api.toolPolicy.ListSemanticRules()
	jsonResponse(w, 200, map[string]interface{}{"rules": rules, "total": len(rules)})
}

// handleToolSemanticRulesCreate POST /api/v1/tools/semantic-rules
func (api *ManagementAPI) handleToolSemanticRulesCreate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var rule ToolSemanticRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.toolPolicy.AddSemanticRule(rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "rule": rule})
}

// handleToolSemanticRulesUpdate PUT /api/v1/tools/semantic-rules/:id
func (api *ManagementAPI) handleToolSemanticRulesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing rule id"})
		return
	}
	var rule ToolSemanticRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	rule.ID = parts[len(parts)-1]
	if err := api.toolPolicy.UpdateSemanticRule(rule); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "rule": rule})
}

// handleToolSemanticRulesDelete DELETE /api/v1/tools/semantic-rules/:id
func (api *ManagementAPI) handleToolSemanticRulesDelete(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing rule id"})
		return
	}
	if err := api.toolPolicy.RemoveSemanticRule(parts[len(parts)-1]); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": parts[len(parts)-1]})
}

// handleToolContextPoliciesList GET /api/v1/tools/context-policies
func (api *ManagementAPI) handleToolContextPoliciesList(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 200, map[string]interface{}{"policies": []interface{}{}, "total": 0})
		return
	}
	policies := api.toolPolicy.ListContextPolicies()
	jsonResponse(w, 200, map[string]interface{}{"policies": policies, "total": len(policies)})
}

// handleToolContextPoliciesCreate POST /api/v1/tools/context-policies
func (api *ManagementAPI) handleToolContextPoliciesCreate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var policy ToolContextPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.toolPolicy.AddContextPolicy(policy); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "policy": policy})
}

// handleToolContextPoliciesUpdate PUT /api/v1/tools/context-policies/:id
func (api *ManagementAPI) handleToolContextPoliciesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing policy id"})
		return
	}
	var policy ToolContextPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	policy.ID = parts[len(parts)-1]
	if err := api.toolPolicy.UpdateContextPolicy(policy); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "policy": policy})
}

// handleToolContextPoliciesDelete DELETE /api/v1/tools/context-policies/:id
func (api *ManagementAPI) handleToolContextPoliciesDelete(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonResponse(w, 400, map[string]string{"error": "missing policy id"})
		return
	}
	if err := api.toolPolicy.RemoveContextPolicy(parts[len(parts)-1]); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": parts[len(parts)-1]})
}

// handleToolPolicyEvaluate POST /api/v1/tools/evaluate
func (api *ManagementAPI) handleToolPolicyEvaluate(w http.ResponseWriter, r *http.Request) {
	if api.toolPolicy == nil {
		jsonResponse(w, 400, map[string]string{"error": "tool policy not enabled"})
		return
	}
	var req struct {
		ToolName   string          `json:"tool_name"`
		Arguments  json.RawMessage `json:"arguments"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	argBytes := req.Arguments
	if len(argBytes) == 0 {
		argBytes = req.Parameters
	}
	argText := ""
	if len(argBytes) > 0 {
		if len(argBytes) > 0 && argBytes[0] == '"' {
			_ = json.Unmarshal(argBytes, &argText)
		} else {
			argText = string(argBytes)
		}
	}
	event := api.toolPolicy.Evaluate(req.ToolName, argText, "test-eval", "test")
	jsonResponse(w, 200, event)
}
