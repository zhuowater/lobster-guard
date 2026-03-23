// gateway_monitor.go — v22.0 上游 OpenClaw Gateway 监控中心
// 代理龙虾卫士的管理 API 去访问上游 OpenClaw Gateway 实例
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Gateway 代理客户端
// ============================================================

var gatewayHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	},
}

// gatewayProxyRequest 向上游 Gateway 发起 HTTP 请求
// 返回 (status_code, response_body, error)
func gatewayProxyRequest(address string, port int, pathPrefix, gatewayToken, apiPath string) (int, []byte, error) {
	// 构造 URL
	baseURL := fmt.Sprintf("http://%s:%d", address, port)
	if pathPrefix != "" {
		baseURL += "/" + strings.Trim(pathPrefix, "/")
	}
	fullURL := baseURL + apiPath

	// 尝试 query parameter 认证（OpenClaw Gateway 支持）
	if gatewayToken != "" {
		sep := "?"
		if strings.Contains(fullURL, "?") {
			sep = "&"
		}
		fullURL += sep + "authToken=" + gatewayToken
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 同时设置 Authorization header（双重认证尝试）
	if gatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+gatewayToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := gatewayHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 限制 1MB
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return resp.StatusCode, body, nil
}

// ============================================================
// API Handlers
// ============================================================

// handleGatewayTokenPut PUT /api/v1/upstreams/{id}/gateway-token
func (api *ManagementAPI) handleGatewayTokenPut(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway-token")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	if err := api.pool.SetGatewayToken(id, req.Token); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[Gateway监控] 更新上游 %s 的 Gateway Token", id)
	jsonResponse(w, 200, map[string]interface{}{
		"status":     "updated",
		"id":         id,
		"configured": req.Token != "",
	})
}

// handleGatewayTokenDelete DELETE /api/v1/upstreams/{id}/gateway-token
func (api *ManagementAPI) handleGatewayTokenDelete(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway-token")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	if err := api.pool.SetGatewayToken(id, ""); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[Gateway监控] 清除上游 %s 的 Gateway Token", id)
	jsonResponse(w, 200, map[string]interface{}{
		"status":     "cleared",
		"id":         id,
		"configured": false,
	})
}

// handleGatewayTokenStatus GET /api/v1/upstreams/{id}/gateway-token/status
func (api *ManagementAPI) handleGatewayTokenStatus(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway-token/status")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	configured := up.GatewayToken != ""
	status := "unknown"
	if !configured {
		status = "not_configured"
	}

	jsonResponse(w, 200, map[string]interface{}{
		"configured": configured,
		"status":     status,
	})
}

// handleGatewayPing GET /api/v1/upstreams/{id}/gateway/ping
func (api *ManagementAPI) handleGatewayPing(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/ping")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token",
		})
		return
	}

	start := time.Now()
	statusCode, _, err := gatewayProxyRequest(up.Address, up.Port, up.PathPrefix, up.GatewayToken, "/api/v1/sessions/list")
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonResponse(w, 200, map[string]interface{}{
			"reachable":     false,
			"authenticated": false,
			"latency_ms":    latency,
			"error":         "unreachable",
			"message":       fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if statusCode == 401 || statusCode == 403 {
		jsonResponse(w, 200, map[string]interface{}{
			"reachable":     true,
			"authenticated": false,
			"latency_ms":    latency,
			"error":         "authentication_failed",
			"message":       "Gateway Token 无效或已过期",
		})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"reachable":     true,
		"authenticated": statusCode >= 200 && statusCode < 400,
		"latency_ms":    latency,
		"status_code":   statusCode,
	})
}

// handleGatewaySessions GET /api/v1/upstreams/{id}/gateway/sessions
func (api *ManagementAPI) handleGatewaySessions(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/sessions")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token",
		})
		return
	}

	statusCode, body, err := gatewayProxyRequest(up.Address, up.Port, up.PathPrefix, up.GatewayToken, "/api/v1/sessions/list")
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error":   "unreachable",
			"message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if statusCode == 401 || statusCode == 403 {
		jsonResponse(w, 401, map[string]interface{}{
			"error":   "authentication_failed",
			"message": "Gateway Token 无效或已过期",
		})
		return
	}

	// 直接转发响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// handleGatewayCron GET /api/v1/upstreams/{id}/gateway/cron
func (api *ManagementAPI) handleGatewayCron(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token",
		})
		return
	}

	statusCode, body, err := gatewayProxyRequest(up.Address, up.Port, up.PathPrefix, up.GatewayToken, "/api/v1/cron/list")
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error":   "unreachable",
			"message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if statusCode == 401 || statusCode == 403 {
		jsonResponse(w, 401, map[string]interface{}{
			"error":   "authentication_failed",
			"message": "Gateway Token 无效或已过期",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// handleGatewayStatus GET /api/v1/upstreams/{id}/gateway/status
func (api *ManagementAPI) handleGatewayStatus(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/status")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token",
		})
		return
	}

	statusCode, body, err := gatewayProxyRequest(up.Address, up.Port, up.PathPrefix, up.GatewayToken, "/status")
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error":   "unreachable",
			"message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if statusCode == 401 || statusCode == 403 {
		jsonResponse(w, 401, map[string]interface{}{
			"error":   "authentication_failed",
			"message": "Gateway Token 无效或已过期",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

// handleGatewayOverview GET /api/v1/upstreams/gateway/overview
func (api *ManagementAPI) handleGatewayOverview(w http.ResponseWriter, r *http.Request) {
	upstreams := api.pool.ListUpstreams()

	type upstreamResult struct {
		ID              string      `json:"id"`
		Address         string      `json:"address"`
		Port            int         `json:"port"`
		Healthy         bool        `json:"healthy"`
		TokenConfigured bool        `json:"token_configured"`
		GatewayStatus   string      `json:"gateway_status"` // connected / not_configured / auth_failed / unreachable / error
		LatencyMs       int64       `json:"latency_ms"`
		Sessions        interface{} `json:"sessions,omitempty"`
		SessionCount    int         `json:"session_count"`
		ActiveSessions  int         `json:"active_sessions"`
		CronCount       int         `json:"cron_count"`
		Error           string      `json:"error,omitempty"`
	}

	var (
		results []upstreamResult
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, up := range upstreams {
		wg.Add(1)
		go func(u Upstream) {
			defer wg.Done()
			result := upstreamResult{
				ID:              u.ID,
				Address:         u.Address,
				Port:            u.Port,
				Healthy:         u.Healthy,
				TokenConfigured: u.GatewayToken != "",
			}

			if u.GatewayToken == "" {
				result.GatewayStatus = "not_configured"
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// Ping test
			start := time.Now()
			statusCode, body, err := gatewayProxyRequest(u.Address, u.Port, u.PathPrefix, u.GatewayToken, "/api/v1/sessions/list")
			result.LatencyMs = time.Since(start).Milliseconds()

			if err != nil {
				result.GatewayStatus = "unreachable"
				result.Error = err.Error()
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			if statusCode == 401 || statusCode == 403 {
				result.GatewayStatus = "auth_failed"
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			if statusCode >= 200 && statusCode < 400 {
				result.GatewayStatus = "connected"
				// 解析 sessions 数据
				var sessionsResp interface{}
				if json.Unmarshal(body, &sessionsResp) == nil {
					result.Sessions = sessionsResp
					// 尝试统计会话数
					if m, ok := sessionsResp.(map[string]interface{}); ok {
						if sessions, ok := m["sessions"].([]interface{}); ok {
							result.SessionCount = len(sessions)
							for _, s := range sessions {
								if sm, ok := s.(map[string]interface{}); ok {
									if state, ok := sm["state"].(string); ok {
										if state == "running" || state == "active" || state == "busy" || state == "working" {
											result.ActiveSessions++
										}
									}
								}
							}
						}
					}
				}
			} else {
				result.GatewayStatus = "error"
				result.Error = fmt.Sprintf("HTTP %d", statusCode)
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(up)
	}

	wg.Wait()

	// 聚合统计
	totalUpstreams := len(results)
	onlineCount := 0
	offlineCount := 0
	tokenConfigured := 0
	tokenNotConfigured := 0
	totalSessions := 0
	activeSessions := 0

	for _, r := range results {
		if r.GatewayStatus == "connected" {
			onlineCount++
		} else {
			offlineCount++
		}
		if r.TokenConfigured {
			tokenConfigured++
		} else {
			tokenNotConfigured++
		}
		totalSessions += r.SessionCount
		activeSessions += r.ActiveSessions
	}

	jsonResponse(w, 200, map[string]interface{}{
		"upstreams":          results,
		"total":              totalUpstreams,
		"online":             onlineCount,
		"offline":            offlineCount,
		"token_configured":   tokenConfigured,
		"token_not_configured": tokenNotConfigured,
		"total_sessions":     totalSessions,
		"active_sessions":    activeSessions,
	})
}

// ============================================================
// 路由辅助函数
// ============================================================

// extractUpstreamIDFromGatewayPath 从 /api/v1/upstreams/{id}/xxx 中提取 id
func extractUpstreamIDFromGatewayPath(urlPath, suffix string) string {
	// 去掉 prefix 和 suffix
	prefix := "/api/v1/upstreams/"
	if !strings.HasPrefix(urlPath, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(urlPath, prefix)
	if !strings.HasSuffix(rest, suffix) {
		return ""
	}
	id := strings.TrimSuffix(rest, suffix)
	return id
}
