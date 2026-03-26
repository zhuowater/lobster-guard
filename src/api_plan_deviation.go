// api_plan_deviation.go — Deviation detection API (v25.2)
package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (api *ManagementAPI) handleDeviationsList(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"deviations": []interface{}{}, "total": 0})
		return
	}
	q := r.URL.Query()
	devs := api.deviationDetector.QueryDeviations(q.Get("trace_id"), q.Get("severity"), 50)
	jsonResponse(w, 200, map[string]interface{}{"deviations": devs, "total": len(devs)})
}

func (api *ManagementAPI) handleDeviationsStats(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 200, DeviationStats{})
		return
	}
	jsonResponse(w, 200, api.deviationDetector.GetStats())
}

func (api *ManagementAPI) handleDeviationsConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 200, defaultDeviationConfig)
		return
	}
	jsonResponse(w, 200, api.deviationDetector.GetConfig())
}

func (api *ManagementAPI) handleDeviationsConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 503, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	var cfg DeviationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	api.deviationDetector.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "config": cfg})
}

func (api *ManagementAPI) handleDeviationsDetail(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/deviations/")
	devs := api.deviationDetector.QueryDeviations(traceID, "", 100)
	jsonResponse(w, 200, map[string]interface{}{"trace_id": traceID, "deviations": devs, "total": len(devs)})
}

// === Repair Policies CRUD ===

func (api *ManagementAPI) handleRepairPoliciesList(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"policies": []interface{}{}, "total": 0})
		return
	}
	policies := api.deviationDetector.GetRepairPolicies()
	jsonResponse(w, 200, map[string]interface{}{"policies": policies, "total": len(policies)})
}

func (api *ManagementAPI) handleRepairPoliciesCreate(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 503, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	var p RepairPolicy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.deviationDetector.CreateRepairPolicy(p); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "created", "id": p.ID})
}

func (api *ManagementAPI) handleRepairPoliciesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 503, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/deviations/repair-policies/")
	var p RepairPolicy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := api.deviationDetector.UpdateRepairPolicy(id, p); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "updated", "id": id})
}

func (api *ManagementAPI) handleRepairPoliciesDelete(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 503, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/deviations/repair-policies/")
	if err := api.deviationDetector.DeleteRepairPolicy(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted", "id": id})
}

// handleDeviationsCheck POST /api/v1/deviations/check — 手动偏差检测
func (api *ManagementAPI) handleDeviationsCheck(w http.ResponseWriter, r *http.Request) {
	if api.deviationDetector == nil {
		jsonResponse(w, 503, map[string]string{"error": "deviation detector not enabled"})
		return
	}
	var req struct {
		TraceID string `json:"trace_id"`
		Tool    string `json:"tool"`
		Args    string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	result := api.deviationDetector.Detect(req.TraceID, req.Tool, req.Args)
	jsonResponse(w, 200, result)
}
