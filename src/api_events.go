// api_events.go — 事件总线管理 API
// lobster-guard v18.1
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// handleEventsList GET /api/v1/events/list — 事件列表
func (api *ManagementAPI) handleEventsList(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 200, map[string]interface{}{"events": []interface{}{}, "total": 0, "enabled": false})
		return
	}
	eventType := r.URL.Query().Get("type")
	severity := r.URL.Query().Get("severity")
	since := r.URL.Query().Get("since")
	// 支持 since 简写: "1h" / "24h" / "7d" / "30d"
	if since != "" && !strings.Contains(since, "T") {
		since = parseSinceParam(since)
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := api.eventBus.QueryEvents(eventType, severity, since, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if events == nil {
		events = []map[string]interface{}{}
	}
	jsonResponse(w, 200, map[string]interface{}{"events": events, "total": len(events)})
}

// handleEventsStats GET /api/v1/events/stats — 事件统计
func (api *ManagementAPI) handleEventsStats(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 200, map[string]interface{}{"enabled": false})
		return
	}
	stats := api.eventBus.GetStats()
	jsonResponse(w, 200, map[string]interface{}{
		"enabled":           true,
		"total_events":      stats.TotalEvents,
		"total_delivered":   stats.TotalDelivered,
		"total_failed":      stats.TotalFailed,
		"total_chains_fired": stats.TotalChainsFired,
		"total_dropped":     stats.TotalDropped,
		"targets_count":     len(api.eventBus.ListTargets()),
		"chains_count":      len(api.eventBus.ListChains()),
	})
}

// handleEventsTargetList GET /api/v1/events/targets — 推送目标列表
func (api *ManagementAPI) handleEventsTargetList(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 200, map[string]interface{}{"targets": []interface{}{}, "total": 0})
		return
	}
	targets := api.eventBus.ListTargets()
	jsonResponse(w, 200, map[string]interface{}{"targets": targets, "total": len(targets)})
}

// handleEventsTargetCreate POST /api/v1/events/targets — 添加推送目标
func (api *ManagementAPI) handleEventsTargetCreate(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 400, map[string]string{"error": "event bus not enabled"})
		return
	}
	var target WebhookTarget
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	if target.URL == "" {
		jsonResponse(w, 400, map[string]string{"error": "url required"})
		return
	}
	if err := api.eventBus.AddTarget(target); err != nil {
		jsonResponse(w, 409, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "added", "target": target})
}

// handleEventsTargetUpdate PUT /api/v1/events/targets/:id — 更新推送目标
func (api *ManagementAPI) handleEventsTargetUpdate(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 400, map[string]string{"error": "event bus not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/events/targets/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "target id required"})
		return
	}
	var target WebhookTarget
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	target.ID = id
	if err := api.eventBus.UpdateTarget(target); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "target": target})
}

// handleEventsTargetDelete DELETE /api/v1/events/targets/:id — 删除推送目标
func (api *ManagementAPI) handleEventsTargetDelete(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 400, map[string]string{"error": "event bus not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/events/targets/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "target id required"})
		return
	}
	if err := api.eventBus.DeleteTarget(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// handleEventsTest POST /api/v1/events/test — 发送测试事件
func (api *ManagementAPI) handleEventsTest(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 400, map[string]string{"error": "event bus not enabled"})
		return
	}
	testEvent := &SecurityEvent{
		ID:        GenerateTraceID(),
		Timestamp: time.Now().UTC(),
		Type:      "test",
		Severity:  "info",
		Domain:    "system",
		Summary:   "事件总线连通性测试",
		Details: map[string]interface{}{
			"source": "api_test",
		},
	}
	api.eventBus.Emit(testEvent)
	jsonResponse(w, 200, map[string]interface{}{
		"status":   "emitted",
		"event_id": testEvent.ID,
	})
}

// handleEventsDeliveries GET /api/v1/events/deliveries — 投递记录
func (api *ManagementAPI) handleEventsDeliveries(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 200, map[string]interface{}{"deliveries": []interface{}{}, "total": 0})
		return
	}
	eventID := r.URL.Query().Get("event_id")
	targetID := r.URL.Query().Get("target_id")
	status := r.URL.Query().Get("status")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	deliveries, err := api.eventBus.QueryDeliveries(eventID, targetID, status, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if deliveries == nil {
		deliveries = []map[string]interface{}{}
	}
	jsonResponse(w, 200, map[string]interface{}{"deliveries": deliveries, "total": len(deliveries)})
}

// handleEventsChains GET /api/v1/events/chains — 动作链列表
func (api *ManagementAPI) handleEventsChains(w http.ResponseWriter, r *http.Request) {
	if api.eventBus == nil {
		jsonResponse(w, 200, map[string]interface{}{"chains": []interface{}{}, "total": 0})
		return
	}
	chains := api.eventBus.ListChains()
	jsonResponse(w, 200, map[string]interface{}{"chains": chains, "total": len(chains)})
}
