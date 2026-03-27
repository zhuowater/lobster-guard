// gateway_monitor.go — v22.1 上游 OpenClaw Gateway 监控中心
// 通过 POST /tools/invoke 调用上游 OpenClaw Gateway 的官方 HTTP API
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// skillInfo 表示一个 skill 的基本信息
type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Workspace   string `json:"workspace,omitempty"`
	HasSkillMD  bool   `json:"has_skill_md"`
}

// ============================================================
// Gateway Tools Invoke 客户端
// ============================================================

var gatewayHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	},
}

// toolsInvokeRequest 是 POST /tools/invoke 的请求体
type toolsInvokeRequest struct {
	Tool   string                 `json:"tool"`
	Action string                 `json:"action,omitempty"`
	Args   map[string]interface{} `json:"args,omitempty"`
}

// toolsInvokeResponse 是 /tools/invoke 的响应体
type toolsInvokeResponse struct {
	OK     bool                   `json:"ok"`
	Result *toolsInvokeResult     `json:"result,omitempty"`
	Error  *toolsInvokeError      `json:"error,omitempty"`
}

type toolsInvokeResult struct {
	Content []toolsContent         `json:"content,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type toolsContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolsInvokeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// gatewayToolsInvoke 通过 POST /tools/invoke 调用上游 Gateway 的 tool
// 返回解析后的 details 对象和原始文本
func gatewayToolsInvoke(address string, port int, pathPrefix, gatewayToken string, req toolsInvokeRequest) (*toolsInvokeResponse, int64, error) {
	// 构造 URL — 管理接口忽略 pathPrefix，直接请求根路径
	// pathPrefix 仅用于消息转发路由（蓝信/飞书回调等），/tools/invoke 始终在根路径
	baseURL := fmt.Sprintf("http://%s:%d", address, port)
	fullURL := baseURL + "/tools/invoke"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if gatewayToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+gatewayToken)
	}

	start := time.Now()
	resp, err := gatewayHTTPClient.Do(httpReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB
	if err != nil {
		return nil, latency, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查是否是 HTML (SPA fallback) — 防止误判
	if isHTMLResponse(respBody) {
		return nil, latency, fmt.Errorf("上游返回 HTML 而非 JSON，可能是 SPA fallback (HTTP %d)", resp.StatusCode)
	}

	// HTTP 层错误
	if resp.StatusCode == 401 {
		return nil, latency, fmt.Errorf("AUTH_FAILED")
	}
	if resp.StatusCode == 404 {
		return nil, latency, fmt.Errorf("TOOL_NOT_FOUND: %s", req.Tool)
	}
	if resp.StatusCode != 200 {
		return nil, latency, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result toolsInvokeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, latency, fmt.Errorf("解析 JSON 失败: %w (body: %s)", err, truncateStr(string(respBody), 200))
	}

	return &result, latency, nil
}

// isHTMLResponse 检测响应是否是 HTML（防止 SPA fallback 误判）
func isHTMLResponse(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	return bytes.HasPrefix(trimmed, []byte("<!")) || bytes.HasPrefix(trimmed, []byte("<html")) || bytes.HasPrefix(trimmed, []byte("<HTML"))
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
// 通过调用 session_status 验证连接 + 认证 + API 可用性
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

	// 用 session_status 做健康检查 — 轻量且能验证认证
	resp, latency, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
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
			"reachable":     false,
			"authenticated": false,
			"latency_ms":    latency,
			"error":         "unreachable",
			"message":       fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "API 返回失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 200, map[string]interface{}{
			"reachable":     true,
			"authenticated": true,
			"api_ok":        false,
			"latency_ms":    latency,
			"error":         "api_error",
			"message":       errMsg,
		})
		return
	}

	// 提取状态文本
	statusText := ""
	if resp.Result != nil && len(resp.Result.Content) > 0 {
		statusText = resp.Result.Content[0].Text
	}

	jsonResponse(w, 200, map[string]interface{}{
		"reachable":     true,
		"authenticated": true,
		"api_ok":        true,
		"latency_ms":    latency,
		"status_text":   statusText,
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

	// 优先使用直接扫描（绕过 tools/invoke 的 visibility 限制）
	if up.OpenClawConfigPath != "" {
		scan := directScanOpenClaw(up.OpenClawConfigPath)
		_, sessions := directScanToOverviewData(scan)

		// 按 agent 分组统计
		byAgent := make(map[string]int)
		for _, s := range scan.Sessions {
			byAgent[s.AgentID]++
		}

		jsonResponse(w, 200, map[string]interface{}{
			"sessions":  sessions,
			"count":     len(sessions),
			"by_agent":  byAgent,
			"source":    "direct_scan",
		})
		return
	}

	// Fallback: tools/invoke（受 visibility 限制，可能只能看到部分 sessions）
	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token 或 openclaw_config_path",
		})
		return
	}

	resp, _, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "sessions_list", Args: map[string]interface{}{}},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{
				"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期",
			})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "sessions_list 调用失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}

	// 从 details 提取 sessions，从 content[0].text 解析 JSON 作为 fallback
	sessions := extractSessionsFromResponse(resp)

	jsonResponse(w, 200, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
		"source":   "tools_invoke",
		"warning":  "受 tools.sessions.visibility 限制，可能只显示部分 sessions。配置 openclaw_config_path 可获取完整列表。",
	})
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

	resp, _, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "cron", Action: "list", Args: map[string]interface{}{}},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{
				"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期",
			})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "cron list 调用失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}

	jobs := extractCronJobsFromResponse(resp)

	jsonResponse(w, 200, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
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

	resp, latency, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{
				"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期",
			})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "session_status 调用失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}

	// 提取状态文本和 details
	statusText := ""
	if resp.Result != nil && len(resp.Result.Content) > 0 {
		statusText = resp.Result.Content[0].Text
	}

	result := map[string]interface{}{
		"status_text": statusText,
		"latency_ms":  latency,
	}

	// 合并 details 字段到结果
	if resp.Result != nil && resp.Result.Details != nil {
		for k, v := range resp.Result.Details {
			result[k] = v
		}
	}

	jsonResponse(w, 200, result)
}

// handleGatewayAgents GET /api/v1/upstreams/{id}/gateway/agents
func (api *ManagementAPI) handleGatewayAgents(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	// 优先使用直接扫描
	if up.OpenClawConfigPath != "" {
		scan := directScanOpenClaw(up.OpenClawConfigPath)
		agents, _ := directScanToOverviewData(scan)

		result := map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
			"source": "direct_scan",
		}
		if scan.Error != "" {
			result["warning"] = scan.Error
		}
		jsonResponse(w, 200, result)
		return
	}

	// Fallback: tools/invoke（agents_list 返回的是 spawn 权限列表，不是完整 agent 列表）
	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error":   "gateway_token_not_configured",
			"message": "请先配置 Gateway Token 或 openclaw_config_path",
		})
		return
	}

	resp, _, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "agents_list", Args: map[string]interface{}{}},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{
				"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期",
			})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "agents_list 调用失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}

	agents := extractAgentsFromResponse(resp)

	jsonResponse(w, 200, map[string]interface{}{
		"agents":  agents,
		"count":   len(agents),
		"source":  "tools_invoke",
		"warning": "agents_list 返回的是当前 session 的 spawn 权限列表，不是完整 agent 注册列表。配置 openclaw_config_path 可获取完整列表。",
	})
}

// handleGatewayOverview GET /api/v1/upstreams/gateway/overview
func (api *ManagementAPI) handleGatewayOverview(w http.ResponseWriter, r *http.Request) {
	upstreams := api.pool.ListUpstreams()

	type upstreamResult struct {
		ID              string                   `json:"id"`
		Address         string                   `json:"address"`
		Port            int                      `json:"port"`
		Healthy         bool                     `json:"healthy"`
		TokenConfigured bool                     `json:"token_configured"`
		GatewayStatus   string                   `json:"gateway_status"`
		LatencyMs       int64                    `json:"latency_ms"`
		Sessions        []map[string]interface{} `json:"sessions,omitempty"`
		SessionCount    int                      `json:"session_count"`
		ActiveSessions  int                      `json:"active_sessions"`
		CronCount       int                      `json:"cron_count"`
		Agents          []map[string]interface{} `json:"agents,omitempty"`
		AgentCount      int                      `json:"agent_count"`
		StatusText      string                   `json:"status_text,omitempty"`
		Error           string                   `json:"error,omitempty"`
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

			// 如果配置了 openclaw_config_path，优先直接扫描 agents/sessions
			var directScan *openclawDirectScanResult
			if u.OpenClawConfigPath != "" {
				directScan = directScanOpenClaw(u.OpenClawConfigPath)
			}

			// 并行获取 status + cron（始终走 tools/invoke）
			// sessions 和 agents：如果有 directScan 则用直接扫描结果，否则走 tools/invoke
			var (
				sessResp   *toolsInvokeResponse
				statusResp *toolsInvokeResponse
				agentsResp *toolsInvokeResponse
				cronResp   *toolsInvokeResponse
				sessErr    error
				statusErr  error
				latency    int64
				innerWg    sync.WaitGroup
			)

			invokeCount := 2 // status + cron 始终走 tools/invoke
			if directScan == nil {
				invokeCount = 4 // 没有直接扫描，sessions + agents 也走 tools/invoke
			}
			innerWg.Add(invokeCount)

			// sessions_list（仅在没有直接扫描时）
			if directScan == nil {
				go func() {
					defer innerWg.Done()
					sessResp, _, sessErr = gatewayToolsInvoke(
						u.Address, u.Port, u.PathPrefix, u.GatewayToken,
						toolsInvokeRequest{Tool: "sessions_list", Args: map[string]interface{}{}},
					)
				}()
			}

			// session_status (ping)
			go func() {
				defer innerWg.Done()
				statusResp, latency, statusErr = gatewayToolsInvoke(
					u.Address, u.Port, u.PathPrefix, u.GatewayToken,
					toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
				)
			}()

			// agents_list（仅在没有直接扫描时）
			if directScan == nil {
				go func() {
					defer innerWg.Done()
					agentsResp, _, _ = gatewayToolsInvoke(
						u.Address, u.Port, u.PathPrefix, u.GatewayToken,
						toolsInvokeRequest{Tool: "agents_list", Args: map[string]interface{}{}},
					)
				}()
			}

			// cron list
			go func() {
				defer innerWg.Done()
				cronResp, _, _ = gatewayToolsInvoke(
					u.Address, u.Port, u.PathPrefix, u.GatewayToken,
					toolsInvokeRequest{Tool: "cron", Action: "list", Args: map[string]interface{}{}},
				)
			}()

			innerWg.Wait()

			result.LatencyMs = latency

			// 判断连接状态
			if statusErr != nil {
				errStr := statusErr.Error()
				if errStr == "AUTH_FAILED" {
					result.GatewayStatus = "auth_failed"
				} else {
					result.GatewayStatus = "unreachable"
					result.Error = errStr
				}
				// 即使 tools/invoke 不通，直接扫描的数据仍然有效
				if directScan != nil && directScan.Error == "" {
					agents, sessions := directScanToOverviewData(directScan)
					result.Agents = agents
					result.AgentCount = len(agents)
					result.Sessions = sessions
					result.SessionCount = len(sessions)
					result.GatewayStatus = "partial" // 有文件系统数据但 API 不通
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			if statusResp != nil && !statusResp.OK {
				result.GatewayStatus = "error"
				if statusResp.Error != nil {
					result.Error = statusResp.Error.Message
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// 连接成功
			result.GatewayStatus = "connected"

			// 提取 status_text
			if statusResp != nil && statusResp.Result != nil && len(statusResp.Result.Content) > 0 {
				result.StatusText = statusResp.Result.Content[0].Text
			}

			// 提取 sessions 和 agents
			if directScan != nil && directScan.Error == "" {
				// 使用直接扫描结果（完整数据）
				agents, sessions := directScanToOverviewData(directScan)
				result.Agents = agents
				result.AgentCount = len(agents)
				result.Sessions = sessions
				result.SessionCount = len(sessions)
				// 用最近 30 分钟内有活动的 session 算活跃
				now := time.Now().UnixMilli()
				for _, s := range directScan.Sessions {
					if now-s.LastModifiedAt < 30*60*1000 {
						result.ActiveSessions++
					}
				}
			} else {
				// Fallback: tools/invoke 结果
				if sessErr == nil && sessResp != nil && sessResp.OK {
					sessions := extractSessionsFromResponse(sessResp)
					result.Sessions = sessions
					result.SessionCount = len(sessions)
					for _, s := range sessions {
						if state, ok := s["state"].(string); ok {
							if isActiveSessionState(state) {
								result.ActiveSessions++
							}
						}
					}
				}
				if agentsResp != nil && agentsResp.OK {
					agents := extractAgentsFromResponse(agentsResp)
					result.Agents = agents
					result.AgentCount = len(agents)
				}
			}

			// 提取 cron
			if cronResp != nil && cronResp.OK {
				jobs := extractCronJobsFromResponse(cronResp)
				result.CronCount = len(jobs)
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
	totalAgents := 0
	totalCronJobs := 0

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
		totalAgents += r.AgentCount
		totalCronJobs += r.CronCount
	}

	jsonResponse(w, 200, map[string]interface{}{
		"upstreams":            results,
		"total":                totalUpstreams,
		"online":               onlineCount,
		"offline":              offlineCount,
		"token_configured":     tokenConfigured,
		"token_not_configured": tokenNotConfigured,
		"total_sessions":       totalSessions,
		"active_sessions":      activeSessions,
		"total_agents":         totalAgents,
		"total_cron_jobs":      totalCronJobs,
	})
}

// ============================================================
// 数据提取辅助函数
// ============================================================

// extractSessionsFromResponse 从 tools/invoke 响应中提取 sessions 列表
func extractSessionsFromResponse(resp *toolsInvokeResponse) []map[string]interface{} {
	if resp == nil || resp.Result == nil {
		return nil
	}

	// 优先从 details.sessions 提取
	if resp.Result.Details != nil {
		if sessions, ok := resp.Result.Details["sessions"]; ok {
			if arr, ok := sessions.([]interface{}); ok {
				return interfaceSliceToMaps(arr)
			}
		}
	}

	// Fallback: 从 content[0].text 解析 JSON
	if len(resp.Result.Content) > 0 {
		text := resp.Result.Content[0].Text
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(text), &parsed) == nil {
			if sessions, ok := parsed["sessions"]; ok {
				if arr, ok := sessions.([]interface{}); ok {
					return interfaceSliceToMaps(arr)
				}
			}
		}
	}

	return nil
}

// extractCronJobsFromResponse 从 tools/invoke 响应中提取 cron jobs 列表
func extractCronJobsFromResponse(resp *toolsInvokeResponse) []map[string]interface{} {
	if resp == nil || resp.Result == nil {
		return nil
	}

	// 优先从 details.jobs 提取
	if resp.Result.Details != nil {
		if jobs, ok := resp.Result.Details["jobs"]; ok {
			if arr, ok := jobs.([]interface{}); ok {
				return interfaceSliceToMaps(arr)
			}
		}
	}

	// Fallback: 从 content[0].text 解析
	if len(resp.Result.Content) > 0 {
		text := resp.Result.Content[0].Text
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(text), &parsed) == nil {
			if jobs, ok := parsed["jobs"]; ok {
				if arr, ok := jobs.([]interface{}); ok {
					return interfaceSliceToMaps(arr)
				}
			}
		}
	}

	return nil
}

// extractAgentsFromResponse 从 tools/invoke 响应中提取 agents 列表
func extractAgentsFromResponse(resp *toolsInvokeResponse) []map[string]interface{} {
	if resp == nil || resp.Result == nil {
		return nil
	}

	// 优先从 details.agents 提取
	if resp.Result.Details != nil {
		if agents, ok := resp.Result.Details["agents"]; ok {
			if arr, ok := agents.([]interface{}); ok {
				return interfaceSliceToMaps(arr)
			}
		}
	}

	// Fallback: 从 content[0].text 解析
	if len(resp.Result.Content) > 0 {
		text := resp.Result.Content[0].Text
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(text), &parsed) == nil {
			if agents, ok := parsed["agents"]; ok {
				if arr, ok := agents.([]interface{}); ok {
					return interfaceSliceToMaps(arr)
				}
			}
		}
	}

	return nil
}

// interfaceSliceToMaps 将 []interface{} 转为 []map[string]interface{}
func interfaceSliceToMaps(arr []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// isActiveSessionState 判断会话是否处于活跃状态
func isActiveSessionState(state string) bool {
	switch strings.ToLower(state) {
	case "running", "active", "busy", "working", "in_progress", "processing", "streaming", "thinking", "executing":
		return true
	}
	return false
}

// ============================================================
// 路由辅助函数
// ============================================================

// extractUpstreamIDFromGatewayPath 从 /api/v1/upstreams/{id}/xxx 中提取 id
func extractUpstreamIDFromGatewayPath(urlPath, suffix string) string {
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

// ============================================================
// Session History API
// ============================================================

// handleGatewaySessionHistory GET /api/v1/upstreams/{id}/gateway/session-history?sessionKey=xxx&limit=20
func (api *ManagementAPI) handleGatewaySessionHistory(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/session-history")
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

	sessionKey := r.URL.Query().Get("sessionKey")
	if sessionKey == "" {
		jsonResponse(w, 400, map[string]string{"error": "sessionKey query parameter required"})
		return
	}

	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); err != nil || n != 1 {
			limit = 30
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	resp, _, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{
			Tool: "sessions_history",
			Args: map[string]interface{}{
				"sessionKey":   sessionKey,
				"limit":        limit,
				"includeTools": false,
			},
		},
	)

	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{
				"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期",
			})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err),
		})
		return
	}

	if !resp.OK {
		errMsg := "sessions_history 调用失败"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}

	// 从 details 提取 messages
	messages := extractMessagesFromResponse(resp)

	jsonResponse(w, 200, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

// extractMessagesFromResponse 从 tools/invoke 响应中提取 messages 列表
func extractMessagesFromResponse(resp *toolsInvokeResponse) []map[string]interface{} {
	if resp == nil || resp.Result == nil {
		return nil
	}

	// 优先从 details.messages 提取
	if resp.Result.Details != nil {
		if messages, ok := resp.Result.Details["messages"]; ok {
			if arr, ok := messages.([]interface{}); ok {
				return interfaceSliceToMaps(arr)
			}
		}
	}

	// Fallback: 从 content[0].text 解析 JSON
	if len(resp.Result.Content) > 0 {
		text := resp.Result.Content[0].Text
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(text), &parsed) == nil {
			if messages, ok := parsed["messages"]; ok {
				if arr, ok := messages.([]interface{}); ok {
					return interfaceSliceToMaps(arr)
				}
			}
		}
	}

	return nil
}

// ============================================================
// Skill 扫描 API
// ============================================================

// handleGatewaySkills GET /api/v1/upstreams/{id}/gateway/skills
// 扫描上游 OpenClaw 实例的 skill 目录
func (api *ManagementAPI) handleGatewaySkills(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	_, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	// OpenClaw skill 目录约定
	scanDirs := []struct {
		Path     string
		Category string
	}{
		{"/root/openclaw/skills", "global"},
		{"/root/.openclaw/skills", "user"},
	}

	// 扫描 workspace skills
	workspaceDirs, _ := filepath.Glob("/root/.openclaw/workspace-*/skills/*")
	for _, wd := range workspaceDirs {
		if info, err := os.Stat(wd); err == nil && info.IsDir() {
			// 检查是否有 SKILL.md
			if _, err := os.Stat(filepath.Join(wd, "SKILL.md")); err == nil {
				// 提取 workspace ID
				parts := strings.Split(wd, "/")
				wsName := ""
				for _, p := range parts {
					if strings.HasPrefix(p, "workspace-") {
						wsName = strings.TrimPrefix(p, "workspace-")
						break
					}
				}
				scanDirs = append(scanDirs, struct {
					Path     string
					Category string
				}{filepath.Dir(wd), "workspace:" + wsName})
			}
		}
	}

	seen := make(map[string]bool)
	var skills []skillInfo

	for _, sd := range scanDirs {
		entries, err := os.ReadDir(sd.Path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "." || name == ".." || name == "node_modules" {
				continue
			}

			// 去重 key
			dedup := sd.Category + ":" + name
			if seen[dedup] {
				continue
			}
			seen[dedup] = true

			skillDir := filepath.Join(sd.Path, name)
			skillMDPath := filepath.Join(skillDir, "SKILL.md")
			hasSkillMD := false
			desc := ""

			if data, err := os.ReadFile(skillMDPath); err == nil {
				hasSkillMD = true
				desc = extractSkillDescription(string(data))
			}

			ws := ""
			if strings.HasPrefix(sd.Category, "workspace:") {
				ws = strings.TrimPrefix(sd.Category, "workspace:")
			}

			skills = append(skills, skillInfo{
				Name:        name,
				Description: desc,
				Category:    sd.Category,
				Workspace:   ws,
				HasSkillMD:  hasSkillMD,
			})
		}
	}

	// 按类别排序：global → user → workspace
	sort.Slice(skills, func(i, j int) bool {
		ci := categoryOrder(skills[i].Category)
		cj := categoryOrder(skills[j].Category)
		if ci != cj {
			return ci < cj
		}
		return skills[i].Name < skills[j].Name
	})

	jsonResponse(w, 200, map[string]interface{}{
		"skills": skills,
		"count":  len(skills),
		"summary": map[string]int{
			"global":    countByCategory(skills, "global"),
			"user":      countByCategory(skills, "user"),
			"workspace": countByCategoryPrefix(skills, "workspace:"),
		},
	})
}

// extractSkillDescription 从 SKILL.md 提取 description 字段
func extractSkillDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "description:") {
			desc := strings.TrimSpace(trimmed[len("description:"):])
			desc = strings.Trim(desc, "\"'")
			if len(desc) > 120 {
				desc = desc[:120] + "..."
			}
			return desc
		}
	}
	// fallback: 取第一个非空非标题行
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "---") && !strings.HasPrefix(trimmed, "name:") {
			if len(trimmed) > 120 {
				trimmed = trimmed[:120] + "..."
			}
			return trimmed
		}
	}
	return ""
}

func categoryOrder(cat string) int {
	if cat == "global" {
		return 0
	}
	if cat == "user" {
		return 1
	}
	return 2
}

func countByCategory(skills []skillInfo, cat string) int {
	n := 0
	for _, s := range skills {
		if s.Category == cat {
			n++
		}
	}
	return n
}

func countByCategoryPrefix(skills []skillInfo, prefix string) int {
	n := 0
	for _, s := range skills {
		if strings.HasPrefix(s.Category, prefix) {
			n++
		}
	}
	return n
}

// ============================================================
// OpenClaw 直接扫描 — 绕过 tools/invoke 的 visibility 限制
// 当 openclaw_config_path 已配置时，直接读取 openclaw.json + 扫描 sessions 目录
// ============================================================

// openclawAgentInfo 从 openclaw.json 解析的 agent 信息
type openclawAgentInfo struct {
	ID        string `json:"id"`
	Workspace string `json:"workspace,omitempty"`
	AgentDir  string `json:"agent_dir,omitempty"`
}

// openclawSessionInfo 从文件系统扫描的 session 信息
type openclawSessionInfo struct {
	Key            string `json:"key"`
	AgentID        string `json:"agent_id"`
	SessionID      string `json:"session_id"`
	LastModifiedAt int64  `json:"last_modified_at"` // unix ms
	SizeBytes      int64  `json:"size_bytes"`
}

// openclawDirectScanResult 直接扫描结果（聚合 agents + sessions）
type openclawDirectScanResult struct {
	Agents   []openclawAgentInfo   `json:"agents"`
	Sessions []openclawSessionInfo `json:"sessions"`
	Source   string                `json:"source"` // "direct_scan" | "tools_invoke"
	Error    string                `json:"error,omitempty"`
}

// scanOpenClawConfig 读取 openclaw.json 提取 agents.list
func scanOpenClawConfig(configPath string) ([]openclawAgentInfo, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取 openclaw.json 失败: %w", err)
	}

	var parsed struct {
		Agents struct {
			List []struct {
				ID        string `json:"id"`
				Workspace string `json:"workspace"`
				AgentDir  string `json:"agentDir"`
			} `json:"list"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("解析 openclaw.json 失败: %w", err)
	}

	agents := make([]openclawAgentInfo, 0, len(parsed.Agents.List))
	for _, a := range parsed.Agents.List {
		agents = append(agents, openclawAgentInfo{
			ID:        a.ID,
			Workspace: a.Workspace,
			AgentDir:  a.AgentDir,
		})
	}
	return agents, nil
}

// scanOpenClawSessions 扫描 OpenClaw 的 sessions 目录
// 约定路径: ~/.openclaw/agents/{agentId-lowercase}/sessions/*.jsonl
func scanOpenClawSessions(configPath string) ([]openclawSessionInfo, error) {
	// 从 configPath 推导 .openclaw 根目录
	// configPath 一般是 /root/.openclaw/openclaw.json
	openclawDir := filepath.Dir(configPath)
	agentsBaseDir := filepath.Join(openclawDir, "agents")

	entries, err := os.ReadDir(agentsBaseDir)
	if err != nil {
		return nil, fmt.Errorf("读取 agents 目录失败: %w", err)
	}

	var sessions []openclawSessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		sessionsDir := filepath.Join(agentsBaseDir, agentID, "sessions")
		sessEntries, err := os.ReadDir(sessionsDir)
		if err != nil {
			continue // 该 agent 没有 sessions 目录
		}
		for _, se := range sessEntries {
			if se.IsDir() || !strings.HasSuffix(se.Name(), ".jsonl") {
				continue
			}
			info, err := se.Info()
			if err != nil {
				continue
			}
			sessionID := strings.TrimSuffix(se.Name(), ".jsonl")
			sessions = append(sessions, openclawSessionInfo{
				Key:            fmt.Sprintf("agent:%s:session:%s", agentID, sessionID),
				AgentID:        agentID,
				SessionID:      sessionID,
				LastModifiedAt: info.ModTime().UnixMilli(),
				SizeBytes:      info.Size(),
			})
		}
	}

	// 按最后修改时间降序排列
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModifiedAt > sessions[j].LastModifiedAt
	})

	return sessions, nil
}

// directScanOpenClaw 完整直接扫描：agents + sessions
func directScanOpenClaw(configPath string) *openclawDirectScanResult {
	result := &openclawDirectScanResult{Source: "direct_scan"}

	agents, err := scanOpenClawConfig(configPath)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Agents = agents

	sessions, err := scanOpenClawSessions(configPath)
	if err != nil {
		result.Error = fmt.Sprintf("agents ok (%d), sessions error: %s", len(agents), err.Error())
		return result
	}
	result.Sessions = sessions
	return result
}

// directScanToOverviewData 将直接扫描结果转为 overview API 需要的 agents/sessions 格式
func directScanToOverviewData(scan *openclawDirectScanResult) (agents []map[string]interface{}, sessions []map[string]interface{}) {
	agents = make([]map[string]interface{}, 0, len(scan.Agents))
	for _, a := range scan.Agents {
		agent := map[string]interface{}{
			"id":         a.ID,
			"configured": true,
		}
		if a.Workspace != "" {
			agent["workspace"] = a.Workspace
		}
		agents = append(agents, agent)
	}

	sessions = make([]map[string]interface{}, 0, len(scan.Sessions))

	// 按 agent 分组统计
	agentSessionCount := make(map[string]int)
	for _, s := range scan.Sessions {
		agentSessionCount[s.AgentID]++
		sess := map[string]interface{}{
			"key":             s.Key,
			"agent_id":        s.AgentID,
			"session_id":      s.SessionID,
			"last_modified_at": s.LastModifiedAt,
			"size_bytes":      s.SizeBytes,
		}
		sessions = append(sessions, sess)
	}

	// 回填 agent 的 session_count
	for i, a := range agents {
		agentIDLower := strings.ToLower(a["id"].(string))
		agents[i]["session_count"] = agentSessionCount[agentIDLower]
	}

	return agents, sessions
}