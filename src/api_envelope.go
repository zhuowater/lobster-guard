// api_envelope.go — 执行信封管理 API（v18.0）
package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleEnvelopeVerify GET /api/v1/envelopes/verify/:id — 验证单个信封
func (api *ManagementAPI) handleEnvelopeVerify(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	envelopeID := strings.TrimPrefix(r.URL.Path, "/api/v1/envelopes/verify/")
	if envelopeID == "" {
		jsonResponse(w, 400, map[string]string{"error": "envelope id required"})
		return
	}

	result, err := api.envelopeMgr.Verify(envelopeID)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, result)
}

// handleEnvelopeChainVerify GET /api/v1/envelopes/chain/:trace — 验证信封链完整性
func (api *ManagementAPI) handleEnvelopeChainVerify(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/envelopes/chain/")
	if traceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id required"})
		return
	}

	result, err := api.envelopeMgr.VerifyChain(traceID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, result)
}

// handleEnvelopeList GET /api/v1/envelopes/list?trace_id= — 按 trace_id 列出信封
func (api *ManagementAPI) handleEnvelopeList(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	traceID := r.URL.Query().Get("trace_id")
	if traceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id parameter required"})
		return
	}

	envelopes, err := api.envelopeMgr.ListByTrace(traceID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"envelopes": envelopes,
		"total":     len(envelopes),
		"trace_id":  traceID,
	})
}

// handleEnvelopeStats GET /api/v1/envelopes/stats — 信封统计
func (api *ManagementAPI) handleEnvelopeStats(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled": false,
			"total":   0,
		})
		return
	}
	stats := api.envelopeMgr.Stats()
	stats["enabled"] = true
	jsonResponse(w, 200, stats)
}

// handleEnvelopeConfigUpdate PUT /api/v1/envelopes/config — 配置开关+密钥（热更新）
func (api *ManagementAPI) handleEnvelopeConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled   *bool  `json:"enabled"`
		SecretKey string `json:"secret_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Enabled != nil {
		api.cfg.EnvelopeEnabled = *req.Enabled
	}
	if req.SecretKey != "" {
		api.cfg.EnvelopeSecretKey = req.SecretKey
	}

	// 热更新：如果启用且有密钥，确保管理器存在
	if api.cfg.EnvelopeEnabled && api.cfg.EnvelopeSecretKey != "" {
		if api.envelopeMgr == nil {
			api.envelopeMgr = NewEnvelopeManager(api.logger.DB(), api.cfg.EnvelopeSecretKey)
		} else {
			api.envelopeMgr.UpdateSecretKey(api.cfg.EnvelopeSecretKey)
		}
	}

	// 如果禁用，设为 nil（但不删除数据）
	if req.Enabled != nil && !*req.Enabled {
		api.envelopeMgr = nil
	}

	jsonResponse(w, 200, map[string]interface{}{
		"status":  "ok",
		"enabled": api.cfg.EnvelopeEnabled,
	})
}
