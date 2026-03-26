// api_ifc.go — IFC (Information Flow Control) API handlers (v26.0)
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// handleIFC dispatches all /api/v1/ifc/* routes
// Returns true if the request was handled
func (api *ManagementAPI) handleIFC(w http.ResponseWriter, r *http.Request, path, method string) bool {
	if !strings.HasPrefix(path, "/api/v1/ifc") {
		return false
	}

	// GET /api/v1/ifc/config
	if path == "/api/v1/ifc/config" && method == "GET" {
		jsonResponse(w, 200, api.ifcEngine.GetConfig())
		return true
	}

	// PUT /api/v1/ifc/config
	if path == "/api/v1/ifc/config" && method == "PUT" {
		var cfg IFCConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		api.ifcEngine.UpdateConfig(cfg)
		jsonResponse(w, 200, map[string]interface{}{"status": "updated", "config": cfg})
		return true
	}

	// GET /api/v1/ifc/source-rules
	if path == "/api/v1/ifc/source-rules" && method == "GET" {
		rules := api.ifcEngine.ListSourceRules()
		if rules == nil {
			rules = []IFCSourceRule{}
		}
		jsonResponse(w, 200, map[string]interface{}{"rules": rules, "total": len(rules)})
		return true
	}

	// POST /api/v1/ifc/source-rules
	if path == "/api/v1/ifc/source-rules" && method == "POST" {
		var rule IFCSourceRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		if err := api.ifcEngine.AddSourceRule(rule); err != nil {
			jsonResponse(w, 409, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 201, map[string]interface{}{"status": "created", "rule": rule})
		return true
	}

	// PUT /api/v1/ifc/source-rules/:source
	if strings.HasPrefix(path, "/api/v1/ifc/source-rules/") && method == "PUT" {
		source := strings.TrimPrefix(path, "/api/v1/ifc/source-rules/")
		var label IFCLabel
		if err := json.NewDecoder(r.Body).Decode(&label); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		if err := api.ifcEngine.UpdateSourceRule(source, label); err != nil {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 200, map[string]interface{}{"status": "updated", "source": source, "label": label})
		return true
	}

	// DELETE /api/v1/ifc/source-rules/:source
	if strings.HasPrefix(path, "/api/v1/ifc/source-rules/") && method == "DELETE" {
		source := strings.TrimPrefix(path, "/api/v1/ifc/source-rules/")
		if err := api.ifcEngine.DeleteSourceRule(source); err != nil {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "source": source})
		return true
	}

	// GET /api/v1/ifc/tool-requirements
	if path == "/api/v1/ifc/tool-requirements" && method == "GET" {
		reqs := api.ifcEngine.ListToolRequirements()
		if reqs == nil {
			reqs = []IFCToolRequirement{}
		}
		jsonResponse(w, 200, map[string]interface{}{"requirements": reqs, "total": len(reqs)})
		return true
	}

	// POST /api/v1/ifc/tool-requirements
	if path == "/api/v1/ifc/tool-requirements" && method == "POST" {
		var req IFCToolRequirement
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		if err := api.ifcEngine.AddToolRequirement(req); err != nil {
			jsonResponse(w, 409, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 201, map[string]interface{}{"status": "created", "requirement": req})
		return true
	}

	// PUT /api/v1/ifc/tool-requirements/:tool
	if strings.HasPrefix(path, "/api/v1/ifc/tool-requirements/") && method == "PUT" {
		tool := strings.TrimPrefix(path, "/api/v1/ifc/tool-requirements/")
		var req IFCToolRequirement
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		if err := api.ifcEngine.UpdateToolRequirement(tool, req); err != nil {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 200, map[string]interface{}{"status": "updated", "tool": tool})
		return true
	}

	// DELETE /api/v1/ifc/tool-requirements/:tool
	if strings.HasPrefix(path, "/api/v1/ifc/tool-requirements/") && method == "DELETE" {
		tool := strings.TrimPrefix(path, "/api/v1/ifc/tool-requirements/")
		if err := api.ifcEngine.DeleteToolRequirement(tool); err != nil {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
			return true
		}
		jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "tool": tool})
		return true
	}

	// GET /api/v1/ifc/variables?trace_id=xxx&limit=100
	if path == "/api/v1/ifc/variables" && method == "GET" {
		traceID := r.URL.Query().Get("trace_id")
		if traceID != "" {
			vars := api.ifcEngine.GetVariables(traceID)
			if vars == nil {
				vars = []IFCVariable{}
			}
			jsonResponse(w, 200, map[string]interface{}{"variables": vars, "total": len(vars)})
		} else {
			// 无 trace_id 时返回所有变量（带 limit）
			limit := 100
			if l := r.URL.Query().Get("limit"); l != "" {
				fmt.Sscanf(l, "%d", &limit)
			}
			vars := api.ifcEngine.GetAllVariables(limit)
			jsonResponse(w, 200, map[string]interface{}{"variables": vars, "total": len(vars)})
		}
		return true
	}

	// GET /api/v1/ifc/violations?limit=50
	if path == "/api/v1/ifc/violations" && method == "GET" {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}
		viols := api.ifcEngine.GetViolations(limit)
		if viols == nil {
			viols = []IFCViolation{}
		}
		jsonResponse(w, 200, map[string]interface{}{"violations": viols, "total": len(viols)})
		return true
	}

	// GET /api/v1/ifc/stats
	if path == "/api/v1/ifc/stats" && method == "GET" {
		jsonResponse(w, 200, api.ifcEngine.GetStats())
		return true
	}

	// POST /api/v1/ifc/check
	if path == "/api/v1/ifc/check" && method == "POST" {
		var req struct {
			TraceID     string   `json:"trace_id"`
			Tool        string   `json:"tool"`
			InputVarIDs []string `json:"input_var_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		decision := api.ifcEngine.CheckToolCall(req.TraceID, req.Tool, req.InputVarIDs)
		jsonResponse(w, 200, decision)
		return true
	}

	// POST /api/v1/ifc/register
	if path == "/api/v1/ifc/register" && method == "POST" {
		var req struct {
			TraceID string `json:"trace_id"`
			Name    string `json:"name"`
			Source  string `json:"source"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		v := api.ifcEngine.RegisterVariable(req.TraceID, req.Name, req.Source, req.Content)
		jsonResponse(w, 201, v)
		return true
	}

	// POST /api/v1/ifc/propagate
	if path == "/api/v1/ifc/propagate" && method == "POST" {
		var req struct {
			TraceID    string   `json:"trace_id"`
			OutputName string   `json:"output_name"`
			InputVarIDs []string `json:"input_var_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		v := api.ifcEngine.Propagate(req.TraceID, req.OutputName, req.InputVarIDs)
		jsonResponse(w, 201, v)
		return true
	}

	// POST /api/v1/ifc/hide
	if path == "/api/v1/ifc/hide" && method == "POST" {
		var req struct {
			TraceID   string   `json:"trace_id"`
			Content   string   `json:"content"`
			Threshold IFCLevel `json:"threshold"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		result := api.ifcEngine.HideContent(req.TraceID, req.Content, req.Threshold)
		jsonResponse(w, 200, result)
		return true
	}

	// POST /api/v1/ifc/doe
	if path == "/api/v1/ifc/doe" && method == "POST" {
		var req struct {
			TraceID string   `json:"trace_id"`
			Tool    string   `json:"tool"`
			Fields  []string `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
			return true
		}
		result := api.ifcEngine.DetectDOE(req.TraceID, req.Tool, req.Fields, nil)
		jsonResponse(w, 200, result)
		return true
	}

	return false
}
