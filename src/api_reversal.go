// api_reversal.go — 污染链逆转管理 API
// lobster-guard v20.2
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleReversalStats GET /api/v1/reversal/stats
func (api *ManagementAPI) handleReversalStats(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":         false,
			"total_reversals": 0,
		})
		return
	}
	stats := api.reversalEngine.Stats()
	jsonResponse(w, 200, stats)
}

// handleReversalRecords GET /api/v1/reversal/records
func (api *ManagementAPI) handleReversalRecords(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"records": []interface{}{}, "total": 0})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	records := api.reversalEngine.ListRecords(limit)
	jsonResponse(w, 200, map[string]interface{}{
		"records": records,
		"total":   len(records),
	})
}

// handleReversalTemplates GET /api/v1/reversal/templates
func (api *ManagementAPI) handleReversalTemplates(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"templates": []interface{}{}, "total": 0})
		return
	}
	templates := api.reversalEngine.GetTemplates()
	jsonResponse(w, 200, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// handleReversalTemplatesAdd POST /api/v1/reversal/templates
func (api *ManagementAPI) handleReversalTemplatesAdd(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "reversal engine not enabled"})
		return
	}
	var tmpl ReversalTemplate
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if err := api.reversalEngine.AddTemplate(tmpl); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "added", "id": tmpl.ID})
}

// handleReversalConfigGet GET /api/v1/reversal/config
func (api *ManagementAPI) handleReversalConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 200, TaintReversalConfig{})
		return
	}
	cfg := api.reversalEngine.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleReversalConfigUpdate PUT /api/v1/reversal/config
func (api *ManagementAPI) handleReversalConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "reversal engine not initialized"})
		return
	}
	var req TaintReversalConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	// 验证 mode
	if req.Mode != "" && req.Mode != "soft" && req.Mode != "hard" && req.Mode != "stealth" {
		jsonResponse(w, 400, map[string]string{"error": "mode must be soft/hard/stealth"})
		return
	}
	api.reversalEngine.UpdateConfig(req)
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleReversalTest POST /api/v1/reversal/test
func (api *ManagementAPI) handleReversalTest(w http.ResponseWriter, r *http.Request) {
	if api.reversalEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "reversal engine not enabled"})
		return
	}
	var req struct {
		TraceID  string `json:"trace_id"`
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TraceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id is required"})
		return
	}
	if req.Response == "" {
		jsonResponse(w, 400, map[string]string{"error": "response is required"})
		return
	}

	reversed, record := api.reversalEngine.Reverse(req.TraceID, req.Response)
	if record == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"reversed":  false,
			"trace_id":  req.TraceID,
			"original":  req.Response,
			"result":    reversed,
		})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"reversed":    true,
		"trace_id":    record.TraceID,
		"template_id": record.TemplateID,
		"mode":        record.Mode,
		"original":    req.Response,
		"result":      reversed,
		"record":      record,
	})
}
