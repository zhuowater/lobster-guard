// api_path_policy.go — Path Policy API endpoints
// lobster-guard v23.0
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
