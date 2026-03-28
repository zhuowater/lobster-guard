package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func (api *ManagementAPI) handleSessionRisks(w http.ResponseWriter, r *http.Request) {
	if api.sessionDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"sessions": []interface{}{},
			"total":    0,
			"enabled":  false,
		})
		return
	}
	sessions := api.sessionDetector.ListHighRiskSessions()
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"sessions": sessions,
		"total":    len(sessions),
		"enabled":  true,
	})
}

// handleSessionRisksReset POST /api/v1/sessions/risks/reset — 重置某用户的风险积分
func (api *ManagementAPI) handleSessionRisksReset(w http.ResponseWriter, r *http.Request) {
	if api.sessionDetector == nil {
		jsonResponse(w, 400, map[string]string{"error": "session detector not enabled"})
		return
	}
	var req struct {
		SenderID string `json:"sender_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SenderID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id required"})
		return
	}
	if api.sessionDetector.ResetRisk(req.SenderID) {
		jsonResponse(w, 200, map[string]string{"status": "reset", "sender_id": req.SenderID})
	} else {
		jsonResponse(w, 404, map[string]string{"error": "session not found"})
	}
}

// ============================================================
// v6.3 规则 CRUD + 导入导出 API
// ============================================================

// persistInboundRules 将规则持久化到文件（如果配置了 inbound_rules_file）
func (api *ManagementAPI) handleSessionReplayList(w http.ResponseWriter, r *http.Request) {
	if api.sessionReplayEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "session replay engine not available"})
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	senderID := r.URL.Query().Get("sender_id")
	risk := r.URL.Query().Get("risk")
	q := r.URL.Query().Get("q")
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	// 支持 since 简写
	if from != "" && !strings.Contains(from, "T") {
		from = parseSinceParam(from)
	}
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	sessions, total, err := api.sessionReplayEng.ListSessionsTenant(from, to, senderID, risk, q, tenantID, limit, offset)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if sessions == nil {
		sessions = []SessionSummary{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"tenant":   tenantID,
	})
}

// handleSessionReplayDetail GET /api/v1/sessions/replay/:traceId — 完整时间线
func (api *ManagementAPI) handleSessionReplayDetail(w http.ResponseWriter, r *http.Request) {
	if api.sessionReplayEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "session replay engine not available"})
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/replay/")
	// 剔除可能的子路径
	if idx := strings.Index(traceID, "/"); idx >= 0 {
		traceID = traceID[:idx]
	}
	if traceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id required"})
		return
	}
	timeline, err := api.sessionReplayEng.GetTimeline(traceID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if len(timeline.Events) == 0 {
		jsonResponse(w, 404, map[string]string{"error": "session not found"})
		return
	}
	jsonResponse(w, 200, timeline)
}

// handleSessionReplayAddTag POST /api/v1/sessions/replay/:traceId/tags — 添加标签
func (api *ManagementAPI) handleSessionReplayAddTag(w http.ResponseWriter, r *http.Request) {
	if api.sessionReplayEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "session replay engine not available"})
		return
	}
	// 解析 traceId: /api/v1/sessions/replay/{traceId}/tags
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/replay/")
	traceID := strings.TrimSuffix(path, "/tags")
	if traceID == "" {
		jsonResponse(w, 400, map[string]string{"error": "trace_id required"})
		return
	}
	var req struct {
		Text      string `json:"text"`
		EventType string `json:"event_type"`
		EventID   int    `json:"event_id"`
		Author    string `json:"author"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Text == "" {
		jsonResponse(w, 400, map[string]string{"error": "text is required"})
		return
	}
	tagID, err := api.sessionReplayEng.AddTag(traceID, req.EventType, req.EventID, req.Text, req.Author)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"id":     tagID,
	})
}

// handleSessionReplayDeleteTag DELETE /api/v1/sessions/replay/tags/:id — 删除标签
func (api *ManagementAPI) handleSessionReplayDeleteTag(w http.ResponseWriter, r *http.Request) {
	if api.sessionReplayEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "session replay engine not available"})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/replay/tags/")
	tagID, err := strconv.Atoi(idStr)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid tag id"})
		return
	}
	if err := api.sessionReplayEng.DeleteTag(tagID); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": tagID})
}

// handleAnomalyBaselines GET /api/v1/anomaly/baselines — 所有指标的基线状态（v14.0: 租户感知不变，全局）
func (api *ManagementAPI) handleSessionCorrelatorStats(w http.ResponseWriter, r *http.Request) {
	if api.sessionCorrelator == nil {
		jsonResponse(w, 200, map[string]interface{}{"enabled": false})
		return
	}
	stats := api.sessionCorrelator.Stats()
	stats["enabled"] = true
	jsonResponse(w, 200, stats)
}

// handleSessionCorrelatorConfig GET /api/v1/session-correlator/config — 返回当前配置
func (api *ManagementAPI) handleSessionCorrelatorConfig(w http.ResponseWriter, r *http.Request) {
	idleMin := 60
	if api.cfg.SessionIdleTimeoutMin > 0 {
		idleMin = api.cfg.SessionIdleTimeoutMin
	}
	fpSec := 300
	if api.cfg.SessionFPWindowSec > 0 {
		fpSec = api.cfg.SessionFPWindowSec
	}
	jsonResponse(w, 200, map[string]interface{}{
		"session_idle_timeout_min": idleMin,
		"session_fp_window_sec":    fpSec,
	})
}

// handleSessionCorrelatorConfigUpdate PUT /api/v1/session-correlator/config — 热更新配置
func (api *ManagementAPI) handleSessionCorrelatorConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionIdleTimeoutMin *int `json:"session_idle_timeout_min"`
		SessionFPWindowSec    *int `json:"session_fp_window_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	updated := false

	if req.SessionIdleTimeoutMin != nil {
		v := *req.SessionIdleTimeoutMin
		if v < 1 || v > 1440 {
			jsonResponse(w, 400, map[string]string{"error": "session_idle_timeout_min must be 1-1440"})
			return
		}
		api.cfg.SessionIdleTimeoutMin = v
		if api.sessionCorrelator != nil {
			api.sessionCorrelator.mu.Lock()
			api.sessionCorrelator.idleTimeoutMs = int64(v) * 60 * 1000
			api.sessionCorrelator.mu.Unlock()
		}
		updated = true
	}

	if req.SessionFPWindowSec != nil {
		v := *req.SessionFPWindowSec
		if v < 10 || v > 3600 {
			jsonResponse(w, 400, map[string]string{"error": "session_fp_window_sec must be 10-3600"})
			return
		}
		api.cfg.SessionFPWindowSec = v
		if api.sessionCorrelator != nil {
			api.sessionCorrelator.mu.Lock()
			api.sessionCorrelator.fpWindowMs = int64(v) * 1000
			api.sessionCorrelator.mu.Unlock()
		}
		updated = true
	}

	if !updated {
		jsonResponse(w, 400, map[string]string{"error": "no fields to update"})
		return
	}

	// 写回 config.yaml
	if err := api.saveSessionCorrelatorConfig(); err != nil {
		log.Printf("[会话关联] 配置写回失败: %v", err)
	}

	jsonResponse(w, 200, map[string]interface{}{
		"ok":                       true,
		"session_idle_timeout_min": api.cfg.SessionIdleTimeoutMin,
		"session_fp_window_sec":    api.cfg.SessionFPWindowSec,
	})
}

// saveSessionCorrelatorConfig 将 session correlator 配置写回 config.yaml
func (api *ManagementAPI) saveSessionCorrelatorConfig() error {
	data, err := os.ReadFile(api.cfgPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	if api.cfg.SessionIdleTimeoutMin > 0 {
		raw["session_idle_timeout_min"] = api.cfg.SessionIdleTimeoutMin
	}
	if api.cfg.SessionFPWindowSec > 0 {
		raw["session_fp_window_sec"] = api.cfg.SessionFPWindowSec
	}
	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(api.cfgPath, out, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	log.Printf("[会话关联] 配置已保存: idle=%dmin, fp=%ds", api.cfg.SessionIdleTimeoutMin, api.cfg.SessionFPWindowSec)
	return nil
}

// ============================================================
// v27.0: API Key 管理 API
// ============================================================

// handleAPIKeyList GET /api/v1/apikeys — API Key 列表
