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
