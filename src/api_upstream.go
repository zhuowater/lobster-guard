package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func (api *ManagementAPI) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string            `json:"id"`
		Address      string            `json:"address"`
		Port         int               `json:"port"`
		Tags         map[string]string `json:"tags"`
		PathPrefix   string            `json:"path_prefix"`
		GatewayToken string            `json:"gateway_token"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := api.pool.Register(req.ID, req.Address, req.Port, req.Tags, req.PathPrefix); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// 设置 gateway_token（如果提供）
	if req.GatewayToken != "" {
		api.pool.SetGatewayToken(req.ID, req.GatewayToken)
	}
	// BUG-005 fix: force metrics gauge update after registration
	if api.metrics != nil {
		api.metrics.RecordUpstreamChange()
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "registered",
		"heartbeat_interval": fmt.Sprintf("%ds", api.cfg.HeartbeatIntervalSec),
		"heartbeat_path": "/api/v1/heartbeat",
	})
}

func (api *ManagementAPI) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string                 `json:"id"`
		Load map[string]interface{} `json:"load"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	userCount, err := api.pool.Heartbeat(req.ID, req.Load)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	// BUG-005 fix: force metrics gauge update after heartbeat
	if api.metrics != nil {
		api.metrics.RecordUpstreamChange()
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "ok", "user_count": userCount})
}

func (api *ManagementAPI) handleDeregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	api.pool.Deregister(req.ID)
	// BUG-005 fix: force metrics gauge update after deregistration
	if api.metrics != nil {
		api.metrics.RecordUpstreamChange()
	}
	jsonResponse(w, 200, map[string]string{"status": "deregistered"})
}

func (api *ManagementAPI) handleListUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams := api.pool.ListUpstreams()
	totalUsers := 0
	healthyCount := 0
	list := []map[string]interface{}{}
	for _, up := range upstreams {
		totalUsers += up.UserCount
		if up.Healthy { healthyCount++ }
		list = append(list, map[string]interface{}{
			"id": up.ID, "address": up.Address, "port": up.Port,
			"healthy": up.Healthy, "user_count": up.UserCount, "static": up.Static,
			"path_prefix": up.PathPrefix,
			"last_heartbeat": up.LastHeartbeat.Format(time.RFC3339),
			"tags": up.Tags, "load": up.Load,
			"gateway_token_configured": up.GatewayToken != "",
		})
	}
	jsonResponse(w, 200, map[string]interface{}{
		"upstreams": list, "total": len(upstreams),
		"healthy": healthyCount, "total_users": totalUsers,
	})
}

// ============================================================
// v21.0 上游 CRUD API
// ============================================================

// handleCreateUpstream POST /api/v1/upstreams — 创建上游（RESTful 等价于 register）
func (api *ManagementAPI) handleCreateUpstream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string            `json:"id"`
		Address      string            `json:"address"`
		Port         int               `json:"port"`
		Tags         map[string]string `json:"tags"`
		PathPrefix   string            `json:"path_prefix"`
		GatewayToken string            `json:"gateway_token"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" || req.Address == "" || req.Port <= 0 {
		jsonResponse(w, 400, map[string]string{"error": "id, address, port 均为必填"})
		return
	}
	if err := api.pool.Register(req.ID, req.Address, req.Port, req.Tags, req.PathPrefix); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// 设置 gateway_token（如果提供）
	if req.GatewayToken != "" {
		api.pool.SetGatewayToken(req.ID, req.GatewayToken)
	}
	log.Printf("[上游CRUD] 创建上游: %s -> %s:%d", req.ID, req.Address, req.Port)
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "created",
		"id":      req.ID,
		"address": req.Address,
		"port":    req.Port,
	})
}

// handleGetUpstream GET /api/v1/upstreams/{id} — 获取单个上游详情
func (api *ManagementAPI) handleGetUpstream(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"id": up.ID, "address": up.Address, "port": up.Port,
		"healthy": up.Healthy, "user_count": up.UserCount, "static": up.Static,
		"registered_at":  up.RegisteredAt.Format(time.RFC3339),
		"last_heartbeat": up.LastHeartbeat.Format(time.RFC3339),
		"tags": up.Tags, "load": up.Load,
		"gateway_token_configured": up.GatewayToken != "",
	})
}

// handleUpdateUpstream PUT /api/v1/upstreams/{id} — 更新上游
func (api *ManagementAPI) handleUpdateUpstream(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var req struct {
		Address      string            `json:"address"`
		Port         int               `json:"port"`
		Tags         map[string]string `json:"tags"`
		PathPrefix   string            `json:"path_prefix"`
		GatewayToken *string           `json:"gateway_token"` // 指针类型，区分未传和空字符串
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if err := api.pool.Update(id, req.Address, req.Port, req.Tags, req.PathPrefix); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	// 更新 gateway_token（如果提供）
	if req.GatewayToken != nil {
		api.pool.SetGatewayToken(id, *req.GatewayToken)
	}
	log.Printf("[上游CRUD] 更新上游: %s", id)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"id":     id,
	})
}

// handleDeleteUpstream DELETE /api/v1/upstreams/{id} — 删除上游
func (api *ManagementAPI) handleDeleteUpstream(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	// 静态上游不可删除
	if up.Static {
		jsonResponse(w, 403, map[string]string{"error": "静态上游不可删除，请修改配置文件"})
		return
	}

	// K8s 发现的上游可以删除，但给出提示
	warnings := []string{}
	if up.Tags != nil && up.Tags["source"] == "k8s" {
		warnings = append(warnings, "此上游由 K8s 自动管理，手动删除后可能会被重新发现")
	}

	// R2-003: 删除上游前清理孤儿路由
	orphanedRoutes := api.routes.CountByUpstream(id)
	if orphanedRoutes > 0 {
		// 清理绑定到该上游的所有路由
		routes := api.routes.ListRoutes()
		for _, r := range routes {
			if r.UpstreamID == id {
				api.routes.Unbind(r.SenderID, r.AppID)
			}
		}
		warnings = append(warnings, fmt.Sprintf("%d routes were bound to this upstream and have been unbound", orphanedRoutes))
		log.Printf("[上游CRUD] 清理了 %d 条孤儿路由 (upstream=%s)", orphanedRoutes, id)
	}

	api.pool.ForceDeregister(id)
	log.Printf("[上游CRUD] 删除上游: %s", id)

	resp := map[string]interface{}{
		"status": "deleted",
		"id":     id,
	}
	if orphanedRoutes > 0 {
		resp["orphaned_routes"] = orphanedRoutes
	}
	if len(warnings) > 0 {
		resp["warning"] = strings.Join(warnings, "; ")
	}
	jsonResponse(w, 200, resp)
}

// handleUpstreamHealthCheck POST /api/v1/upstreams/{id}/health-check — 手动触发健康检查
func (api *ManagementAPI) handleUpstreamHealthCheck(w http.ResponseWriter, r *http.Request) {
	// 从 URL 提取 id: /api/v1/upstreams/{id}/health-check
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	id := strings.TrimSuffix(path, "/health-check")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	// 执行 HTTP 健康检查
	addr := fmt.Sprintf("http://%s:%d/healthz", up.Address, up.Port)
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(addr)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	result := map[string]interface{}{
		"id":         id,
		"address":    addr,
		"latency_ms": latencyMs,
	}

	if err != nil {
		result["healthy"] = false
		result["error"] = err.Error()
	} else {
		resp.Body.Close()
		result["healthy"] = resp.StatusCode >= 200 && resp.StatusCode < 400
		result["status_code"] = resp.StatusCode
	}

	jsonResponse(w, 200, result)
}

// ============================================================
// v21.0 K8s 发现状态 API
// ============================================================

// handleDiscoveryStatus GET /api/v1/discovery/status — 返回 K8s 发现状态

// ============================================================
// v33.0 上游安全画像 API
// ============================================================

// handleUpstreamSecurityProfile GET /api/v1/upstreams/{id}/security-profile
func (api *ManagementAPI) handleUpstreamSecurityProfile(w http.ResponseWriter, r *http.Request) {
	if api.upstreamProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "upstream profile engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/upstreams/")
	id = strings.TrimSuffix(id, "/security-profile")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "upstream id required"})
		return
	}
	profile, err := api.upstreamProfileEng.BuildProfile(id)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, profile)
}

// handleUpstreamProfileList GET /api/v1/upstream-profiles
func (api *ManagementAPI) handleUpstreamProfileList(w http.ResponseWriter, r *http.Request) {
	if api.upstreamProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "upstream profile engine not available"})
		return
	}
	var ids []string
	api.pool.mu.RLock()
	for id := range api.pool.upstreams {
		ids = append(ids, id)
	}
	api.pool.mu.RUnlock()

	profiles := api.upstreamProfileEng.ListProfiles(ids)
	if profiles == nil {
		profiles = []UpstreamSecurityProfile{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"profiles": profiles,
		"total":    len(profiles),
	})
}
