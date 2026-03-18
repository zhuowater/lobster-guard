// api_gateway_mgmt.go — API Gateway 管理 API 路由处理器
// lobster-guard v20.4
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================================
// 管理 API Handlers
// ============================================================

// handleGatewayStats GET /api/v1/gateway/stats — 网关统计
func (api *ManagementAPI) handleGatewayStats(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}
	stats := api.apiGateway.GetStats()
	jsonResponse(w, 200, stats)
}

// handleGatewayRouteList GET /api/v1/gateway/routes — 路由列表
func (api *ManagementAPI) handleGatewayRouteList(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}
	routes := api.apiGateway.ListRoutes()
	if routes == nil {
		routes = []GatewayRoute{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"routes": routes,
		"total":  len(routes),
	})
}

// handleGatewayRouteAdd POST /api/v1/gateway/routes — 添加路由
func (api *ManagementAPI) handleGatewayRouteAdd(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	var route GatewayRoute
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if route.Name == "" || route.PathPattern == "" || route.UpstreamURL == "" {
		jsonResponse(w, 400, map[string]string{"error": "name, path_pattern, and upstream_url are required"})
		return
	}

	if err := api.apiGateway.AddRoute(route); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]string{"status": "ok", "id": route.ID})
}

// handleGatewayRouteUpdate PUT /api/v1/gateway/routes/:id — 更新路由
func (api *ManagementAPI) handleGatewayRouteUpdate(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/gateway/routes/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "route id required"})
		return
	}

	var route GatewayRoute
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if err := api.apiGateway.UpdateRoute(id, route); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleGatewayRouteDelete DELETE /api/v1/gateway/routes/:id — 删除路由
func (api *ManagementAPI) handleGatewayRouteDelete(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/gateway/routes/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "route id required"})
		return
	}

	if err := api.apiGateway.RemoveRoute(id); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleGatewayConfigGet GET /api/v1/gateway/config — 获取配置
func (api *ManagementAPI) handleGatewayConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}
	cfg := api.apiGateway.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleGatewayConfigUpdate PUT /api/v1/gateway/config — 更新配置
func (api *ManagementAPI) handleGatewayConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	var cfg APIGatewayConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	api.apiGateway.UpdateConfig(cfg)
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleGatewayTokenGenerate POST /api/v1/gateway/token — 生成 JWT token
func (api *ManagementAPI) handleGatewayTokenGenerate(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	var req struct {
		TenantID    string `json:"tenant_id"`
		Role        string `json:"role"`
		ExpireHours int    `json:"expire_hours"`
		Sub         string `json:"sub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.TenantID == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant_id required"})
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	if req.ExpireHours <= 0 {
		req.ExpireHours = 24
	}
	if req.Sub == "" {
		req.Sub = req.TenantID
	}

	now := time.Now().Unix()
	claims := GWJWTClaims{
		TenantID: req.TenantID,
		Role:     req.Role,
		Sub:      req.Sub,
		Iat:      now,
		Exp:      now + int64(req.ExpireHours)*3600,
	}

	token, err := api.apiGateway.GenerateGWJWT(claims)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"token":     token,
		"claims":    claims,
		"expire_at": claims.Exp,
	})
}

// handleGatewayTokenValidate POST /api/v1/gateway/validate — 验证 JWT token
func (api *ManagementAPI) handleGatewayTokenValidate(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Token == "" {
		jsonResponse(w, 400, map[string]string{"error": "token required"})
		return
	}

	claims, err := api.apiGateway.ValidateGWJWT(req.Token)
	if err != nil {
		jsonResponse(w, 401, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"valid":  true,
		"claims": claims,
	})
}

// handleGatewayLog GET /api/v1/gateway/log — 网关日志
func (api *ManagementAPI) handleGatewayLog(w http.ResponseWriter, r *http.Request) {
	if api.apiGateway == nil {
		jsonResponse(w, 404, map[string]string{"error": "API gateway not enabled"})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	routeID := r.URL.Query().Get("route")
	tenantID := r.URL.Query().Get("tenant")

	entries := api.apiGateway.QueryLog(limit, routeID, tenantID)
	if entries == nil {
		entries = []GatewayLogEntry{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}
