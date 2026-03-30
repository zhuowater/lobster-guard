// api_envelope.go — 执行信封管理 API（v18.0）
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
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

// handleEnvelopeList GET /api/v1/envelopes/list?trace_id=&limit= — 列出信封（trace_id 可选）
func (api *ManagementAPI) handleEnvelopeList(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	traceID := r.URL.Query().Get("trace_id")

	if traceID != "" {
		// 按 trace_id 筛选
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
	} else {
		// 无筛选，返回最近信封
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}
		envelopes, err := api.envelopeMgr.ListRecent(limit)
		if err != nil {
			jsonResponse(w, 500, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"envelopes": envelopes,
			"total":     len(envelopes),
		})
	}
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

// handleEnvelopeProof GET /api/v1/envelopes/proof/:id — 返回 Merkle Proof
func (api *ManagementAPI) handleEnvelopeProof(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	envelopeID := strings.TrimPrefix(r.URL.Path, "/api/v1/envelopes/proof/")
	if envelopeID == "" {
		jsonResponse(w, 400, map[string]string{"error": "envelope id required"})
		return
	}

	proof, err := api.envelopeMgr.GenerateProof(envelopeID)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, proof)
}

// handleEnvelopeBatchList GET /api/v1/envelopes/batches — 批次列表
func (api *ManagementAPI) handleEnvelopeBatchList(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	batches, err := api.envelopeMgr.ListBatches(100)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"batches": batches,
		"total":   len(batches),
	})
}

// handleEnvelopeBatchDetail GET /api/v1/envelopes/batch/:id — 批次详情 + 验证
func (api *ManagementAPI) handleEnvelopeBatchDetail(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	batchID := strings.TrimPrefix(r.URL.Path, "/api/v1/envelopes/batch/")
	if batchID == "" {
		jsonResponse(w, 400, map[string]string{"error": "batch id required"})
		return
	}

	result, err := api.envelopeMgr.VerifyBatch(batchID)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, result)
}

// handleEnvelopeConfigUpdate PUT /api/v1/envelopes/config — 配置开关+密钥（热更新）

// handleEnvelopeRangeVerify POST /api/v1/envelopes/verify — 按时间范围批量验证 Merkle 批次
func (api *ManagementAPI) handleEnvelopeRangeVerify(w http.ResponseWriter, r *http.Request) {
	if api.envelopeMgr == nil {
		jsonResponse(w, 404, map[string]string{"error": "envelope not enabled"})
		return
	}
	var req struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	var start, end time.Time
	var err error
	if strings.TrimSpace(req.Start) != "" {
		start, err = time.Parse(time.RFC3339, req.Start)
		if err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid start time"})
			return
		}
	}
	if strings.TrimSpace(req.End) != "" {
		end, err = time.Parse(time.RFC3339, req.End)
		if err != nil {
			jsonResponse(w, 400, map[string]string{"error": "invalid end time"})
			return
		}
	}
	batches, err := api.envelopeMgr.ListBatches(500)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	type item struct {
		BatchID        string    `json:"batch_id"`
		CreatedAt      time.Time `json:"created_at"`
		Valid          bool      `json:"valid"`
		LeafCount      int       `json:"leaf_count"`
		FailureReasons []string  `json:"failure_reasons,omitempty"`
	}
	results := make([]item, 0)
	passed := 0
	failed := 0
	for _, batch := range batches {
		if !start.IsZero() && batch.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && batch.CreatedAt.After(end) {
			continue
		}
		v, err := api.envelopeMgr.VerifyBatch(batch.ID)
		entry := item{BatchID: batch.ID, CreatedAt: batch.CreatedAt, LeafCount: batch.LeafCount}
		if err != nil {
			entry.Valid = false
			entry.FailureReasons = []string{err.Error()}
			failed++
		} else {
			entry.Valid = v.Valid
			entry.LeafCount = v.LeafCount
			entry.FailureReasons = v.FailureReasons
			if v.Valid {
				passed++
			} else {
				failed++
			}
		}
		results = append(results, entry)
	}
	jsonResponse(w, 200, map[string]interface{}{
		"start":         req.Start,
		"end":           req.End,
		"total_batches": len(results),
		"passed":        passed,
		"failed":        failed,
		"results":       results,
	})
}

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
