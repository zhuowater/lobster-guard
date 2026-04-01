// gateway_monitor.go — v29.0 上游 OpenClaw Gateway 监控中心
// 优先通过持久化 WSS RPC 连接（复刻 Control UI 协议），fallback 到 tools/invoke
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

	"github.com/gorilla/websocket"
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
	OK     bool               `json:"ok"`
	Result *toolsInvokeResult `json:"result,omitempty"`
	Error  *toolsInvokeError  `json:"error,omitempty"`
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

// Gateway RPC frames (WebSocket control plane)
type gatewayRPCRequest struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

type gatewayRPCResponse struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id"`
	OK      bool                   `json:"ok"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Error   map[string]interface{} `json:"error,omitempty"`
}

func gatewayRPCRequestCall(address string, port int, gatewayToken string, method string, params map[string]interface{}) (map[string]interface{}, int64, error) {
	if gatewayToken == "" {
		return nil, 0, fmt.Errorf("AUTH_FAILED")
	}
	wsURL := fmt.Sprintf("ws://%s:%d/", address, port)
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+gatewayToken)
	start := time.Now()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if resp != nil && resp.StatusCode == 401 {
			return nil, latency, fmt.Errorf("AUTH_FAILED")
		}
		return nil, latency, fmt.Errorf("ws dial failed: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	requestID := fmt.Sprintf("lgm-%d", time.Now().UnixNano())
	frame := gatewayRPCRequest{Type: "req", ID: requestID, Method: method, Params: params}
	if err := conn.WriteJSON(frame); err != nil {
		return nil, latency, fmt.Errorf("ws write failed: %w", err)
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, latency, fmt.Errorf("ws read failed: %w", err)
		}
		var probe map[string]interface{}
		if json.Unmarshal(data, &probe) != nil {
			continue
		}
		if probe["type"] == "event" {
			continue
		}
		var res gatewayRPCResponse
		if err := json.Unmarshal(data, &res); err != nil {
			continue
		}
		if res.Type != "res" || res.ID != requestID {
			continue
		}
		if !res.OK {
			if msg, ok := res.Error["message"].(string); ok && msg != "" {
				return nil, latency, fmt.Errorf(msg)
			}
			return nil, latency, fmt.Errorf("rpc method failed: %s", method)
		}
		return res.Payload, latency, nil
	}
}

func gatewaySessionsList(address string, port int, gatewayToken string) ([]map[string]interface{}, int64, error) {
	payload, latency, err := gatewayRPCRequestCall(address, port, gatewayToken, "sessions.list", map[string]interface{}{
		"includeGlobal":        true,
		"includeUnknown":       true,
		"includeDerivedTitles": true,
		"includeLastMessage":   true,
		"limit":                500,
	})
	if err != nil {
		return nil, latency, err
	}
	raw, _ := payload["sessions"].([]interface{})
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, latency, nil
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
// WSS Client 辅助方法（v29.0）
// ============================================================

// getWSClient 获取或创建上游的 WSS 客户端
func (api *ManagementAPI) getWSClient(up *Upstream) *GatewayWSClient {
	if api.gwManager == nil || up.GatewayToken == "" {
		return nil
	}
	return api.gwManager.EnsureClient(up)
}

// rpcOrError 调用 WSS RPC，如果连接不可用返回 nil + error
func (api *ManagementAPI) rpcCall(up *Upstream, method string, params interface{}) (map[string]interface{}, error) {
	client := api.getWSClient(up)
	if client == nil {
		return nil, fmt.Errorf("WSS_NOT_AVAILABLE")
	}
	if !client.IsConnected() {
		return nil, fmt.Errorf("WSS_NOT_CONNECTED")
	}
	return client.Request(method, params, 15*time.Second)
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
// v29.0: 优先 WSS RPC health，fallback tools/invoke
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

	// v29.0: 优先 WSS RPC
	start := time.Now()
	payload, err := api.rpcCall(up, "health", map[string]interface{}{})
	latency := time.Since(start).Milliseconds()
	if err == nil {
		// WSS 连接正常
		client := api.getWSClient(up)
		result := map[string]interface{}{
			"reachable":     true,
			"authenticated": true,
			"api_ok":        true,
			"latency_ms":    latency,
			"source":        "wss_rpc",
			"wss_state":     "connected",
		}
		if client != nil {
			result["wss_stats"] = client.Stats()
		}
		for k, v := range payload {
			result[k] = v
		}
		jsonResponse(w, 200, result)
		return
	}

	// Fallback: tools/invoke
	resp, latency2, invokeErr := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
	)
	if invokeErr != nil {
		errStr := invokeErr.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 200, map[string]interface{}{
				"reachable": true, "authenticated": false,
				"latency_ms": latency2, "error": "authentication_failed",
				"message": "Gateway Token 无效或已过期", "source": "tools_invoke_fallback",
			})
			return
		}
		jsonResponse(w, 200, map[string]interface{}{
			"reachable": false, "authenticated": false,
			"latency_ms": latency2, "error": "unreachable",
			"message": fmt.Sprintf("WSS: %v; HTTP fallback: %v", err, invokeErr), "source": "tools_invoke_fallback",
		})
		return
	}
	if !resp.OK {
		errMsg := "API 返回失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 200, map[string]interface{}{
			"reachable": true, "authenticated": true, "api_ok": false,
			"latency_ms": latency2, "error": "api_error", "message": errMsg, "source": "tools_invoke_fallback",
		})
		return
	}
	statusText := ""
	if resp.Result != nil && len(resp.Result.Content) > 0 { statusText = resp.Result.Content[0].Text }
	jsonResponse(w, 200, map[string]interface{}{
		"reachable": true, "authenticated": true, "api_ok": true,
		"latency_ms": latency2, "status_text": statusText, "source": "tools_invoke_fallback",
	})
}

// handleGatewaySessions GET /api/v1/upstreams/{id}/gateway/sessions
// v29.0: 优先持久化 WSS RPC，fallback tools/invoke
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
			"error": "gateway_token_not_configured", "message": "请先配置 Gateway Token",
		})
		return
	}

	// v29.0: 优先 WSS RPC sessions.list
	payload, err := api.rpcCall(up, "sessions.list", map[string]interface{}{
		"includeGlobal": true, "includeUnknown": true, "limit": 500,
	})
	if err == nil {
		raw, _ := payload["sessions"].([]interface{})
		sessions := interfaceSliceToMaps(raw)
		byAgent := make(map[string]int)
		for _, s := range sessions {
			if agentID, ok := s["agentId"].(string); ok && agentID != "" {
				byAgent[agentID]++
			}
		}
		jsonResponse(w, 200, map[string]interface{}{
			"sessions": sessions, "count": len(sessions),
			"by_agent": byAgent, "source": "wss_rpc",
		})
		return
	}

	// Fallback: tools/invoke
	resp, _, invokeErr := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "sessions_list", Args: map[string]interface{}{}},
	)
	if invokeErr != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("WSS: %v; fallback: %v", err, invokeErr),
		})
		return
	}
	if !resp.OK {
		errMsg := "sessions_list 调用失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}
	sessions := extractSessionsFromResponse(resp)
	jsonResponse(w, 200, map[string]interface{}{
		"sessions": sessions, "count": len(sessions),
		"source": "tools_invoke_fallback", "warning": "WSS 不可用，已降级到 tools/invoke",
	})
}

// handleGatewayCron GET /api/v1/upstreams/{id}/gateway/cron
// v29.0: 优先 WSS RPC cron.list
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
			"error": "gateway_token_not_configured", "message": "请先配置 Gateway Token",
		})
		return
	}

	// v29.0: 优先 WSS RPC
	payload, err := api.rpcCall(up, "cron.list", map[string]interface{}{"includeDisabled": true})
	if err == nil {
		raw, _ := payload["jobs"].([]interface{})
		jobs := interfaceSliceToMaps(raw)
		jsonResponse(w, 200, map[string]interface{}{
			"jobs": jobs, "count": len(jobs), "source": "wss_rpc",
		})
		return
	}

	// Fallback: tools/invoke
	resp, _, invokeErr := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "cron", Action: "list", Args: map[string]interface{}{}},
	)
	if invokeErr != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("WSS: %v; fallback: %v", err, invokeErr),
		})
		return
	}
	if !resp.OK {
		errMsg := "cron list 调用失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}
	jobs := extractCronJobsFromResponse(resp)
	jsonResponse(w, 200, map[string]interface{}{
		"jobs": jobs, "count": len(jobs), "source": "tools_invoke_fallback",
	})
}

// handleGatewayStatus GET /api/v1/upstreams/{id}/gateway/status
// v29.0: 优先 WSS RPC status + health 聚合
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
			"error": "gateway_token_not_configured", "message": "请先配置 Gateway Token",
		})
		return
	}

	// v29.0: 优先 WSS RPC — 并发获取 status + health
	client := api.getWSClient(up)
	if client != nil && client.IsConnected() {
		var (
			statusPayload, healthPayload map[string]interface{}
			statusErr, healthErr         error
			wg                           sync.WaitGroup
		)
		wg.Add(2)
		start := time.Now()
		go func() { defer wg.Done(); statusPayload, statusErr = client.GWStatus() }()
		go func() { defer wg.Done(); healthPayload, healthErr = client.GWHealth() }()
		wg.Wait()
		latency := time.Since(start).Milliseconds()

		if statusErr == nil {
			result := map[string]interface{}{
				"latency_ms": latency, "source": "wss_rpc", "wss_state": "connected",
			}
			for k, v := range statusPayload { result["status_"+k] = v }
			if healthErr == nil {
				for k, v := range healthPayload { result["health_"+k] = v }
			}
			result["wss_stats"] = client.Stats()
			jsonResponse(w, 200, result)
			return
		}
	}

	// Fallback: tools/invoke session_status
	resp, latency, err := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
	)
	if err != nil {
		errStr := err.Error()
		if errStr == "AUTH_FAILED" {
			jsonResponse(w, 502, map[string]interface{}{"error": "upstream_auth_failed", "message": "Gateway Token 无效或已过期"})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "unreachable", "message": fmt.Sprintf("无法连接到 Gateway: %v", err)})
		return
	}
	if !resp.OK {
		errMsg := "session_status 调用失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}
	statusText := ""
	if resp.Result != nil && len(resp.Result.Content) > 0 { statusText = resp.Result.Content[0].Text }
	result := map[string]interface{}{"status_text": statusText, "latency_ms": latency, "source": "tools_invoke_fallback"}
	if resp.Result != nil && resp.Result.Details != nil {
		for k, v := range resp.Result.Details { result[k] = v }
	}
	jsonResponse(w, 200, result)
}

// handleGatewayAgents GET /api/v1/upstreams/{id}/gateway/agents
// v29.0: 优先直接扫描 > WSS RPC agents.list > tools/invoke
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

	// 优先使用直接扫描（同机部署场景）
	if up.OpenClawConfigPath != "" {
		scan := directScanOpenClaw(up.OpenClawConfigPath)
		agents, _ := directScanToOverviewData(scan)
		result := map[string]interface{}{"agents": agents, "count": len(agents), "source": "direct_scan"}
		if scan.Error != "" { result["warning"] = scan.Error }
		jsonResponse(w, 200, result)
		return
	}

	if up.GatewayToken == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"error": "gateway_token_not_configured", "message": "请先配置 Gateway Token",
		})
		return
	}

	// v29.0: WSS RPC agents.list（完整 agent 列表）
	payload, err := api.rpcCall(up, "agents.list", map[string]interface{}{})
	if err == nil {
		raw, _ := payload["agents"].([]interface{})
		agents := interfaceSliceToMaps(raw)
		jsonResponse(w, 200, map[string]interface{}{
			"agents": agents, "count": len(agents), "source": "wss_rpc",
		})
		return
	}

	// Fallback: tools/invoke
	resp, _, invokeErr := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "agents_list", Args: map[string]interface{}{}},
	)
	if invokeErr != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("WSS: %v; fallback: %v", err, invokeErr),
		})
		return
	}
	if !resp.OK {
		errMsg := "agents_list 调用失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}
	agents := extractAgentsFromResponse(resp)
	jsonResponse(w, 200, map[string]interface{}{
		"agents": agents, "count": len(agents), "source": "tools_invoke_fallback",
		"warning": "agents_list 返回的是 spawn 权限列表，不是完整 agent 列表",
	})
}

// gwOverviewResult v29.0 overview 结果结构体（包级别，供多方法共享）
type gwOverviewResult struct {
	ID              string                   `json:"id"`
	Address         string                   `json:"address"`
	Port            int                      `json:"port"`
	Healthy         bool                     `json:"healthy"`
	TokenConfigured bool                     `json:"token_configured"`
	GatewayStatus   string                   `json:"gateway_status"`
	WSSState        string                   `json:"wss_state,omitempty"`
	Source          string                   `json:"source,omitempty"`
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

// handleGatewayOverview GET /api/v1/upstreams/gateway/overview
// v29.0: 全面改用持久化 WSS RPC，fallback tools/invoke
func (api *ManagementAPI) handleGatewayOverview(w http.ResponseWriter, r *http.Request) {
	upstreams := api.pool.ListUpstreams()

	var (
		results []gwOverviewResult
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, up := range upstreams {
		wg.Add(1)
		go func(u Upstream) {
			defer wg.Done()
			result := gwOverviewResult{
				ID: u.ID, Address: u.Address, Port: u.Port, Healthy: u.Healthy,
				TokenConfigured: u.GatewayToken != "",
			}

			if u.GatewayToken == "" {
				result.GatewayStatus = "not_configured"
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// v29.0: 优先 WSS RPC 持久连接
			client := api.getWSClient(&u)
			if client != nil && client.IsConnected() {
				result.WSSState = "connected"
				result.Source = "wss_rpc"
				api.overviewViaWSS(client, &u, &result)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			// Fallback: tools/invoke（WSS 未就绪时）
			result.WSSState = "disconnected"
			if client != nil {
				result.WSSState = client.State()
			}
			result.Source = "tools_invoke_fallback"
			api.overviewViaToolsInvoke(&u, &result)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(up)
	}

	wg.Wait()

	// 聚合统计
	totalUpstreams := len(results)
	var onlineCount, offlineCount, tokenConfigured, tokenNotConfigured int
	var totalSessions, activeSessions, totalAgents, totalCronJobs int

	for _, r := range results {
		if r.GatewayStatus == "connected" { onlineCount++ } else { offlineCount++ }
		if r.TokenConfigured { tokenConfigured++ } else { tokenNotConfigured++ }
		totalSessions += r.SessionCount
		activeSessions += r.ActiveSessions
		totalAgents += r.AgentCount
		totalCronJobs += r.CronCount
	}

	jsonResponse(w, 200, map[string]interface{}{
		"upstreams": results, "total": totalUpstreams,
		"online": onlineCount, "offline": offlineCount,
		"token_configured": tokenConfigured, "token_not_configured": tokenNotConfigured,
		"total_sessions": totalSessions, "active_sessions": activeSessions,
		"total_agents": totalAgents, "total_cron_jobs": totalCronJobs,
	})
}

// overviewViaWSS 通过持久化 WSS 连接并发获取 overview 数据
func (api *ManagementAPI) overviewViaWSS(client *GatewayWSClient, u *Upstream, result *gwOverviewResult) {
	var (
		statusPayload, sessPayload, agentsPayload, cronPayload map[string]interface{}
		statusErr, sessErr, agentsErr, cronErr                  error
		innerWg                                                 sync.WaitGroup
	)

	innerWg.Add(4)
	start := time.Now()

	go func() {
		defer innerWg.Done()
		statusPayload, statusErr = client.GWStatus()
	}()
	go func() {
		defer innerWg.Done()
		sessPayload, sessErr = client.GWSessionsList(nil)
	}()
	go func() {
		defer innerWg.Done()
		agentsPayload, agentsErr = client.GWAgentsList()
	}()
	go func() {
		defer innerWg.Done()
		cronPayload, cronErr = client.GWCronList()
	}()

	innerWg.Wait()
	result.LatencyMs = time.Since(start).Milliseconds()

	// Status
	if statusErr != nil {
		result.GatewayStatus = "error"
		result.Error = statusErr.Error()
		// 仍尝试填充 sessions 数据
	} else {
		result.GatewayStatus = "connected"
		// 提取 status text（如有）
		if txt, ok := statusPayload["statusText"].(string); ok {
			result.StatusText = txt
		}
	}

	// Sessions
	if sessErr == nil && sessPayload != nil {
		raw, _ := sessPayload["sessions"].([]interface{})
		sessions := interfaceSliceToMaps(raw)
		result.Sessions = sessions
		result.SessionCount = len(sessions)
		now := time.Now().UnixMilli()
		for _, s := range sessions {
			if state, ok := s["state"].(string); ok && isActiveSessionState(state) {
				result.ActiveSessions++
				continue
			}
			if updatedAt := extractTimestampMs(s, "updatedAt", "updated_at"); updatedAt > 0 {
				if now-updatedAt < 30*60*1000 { result.ActiveSessions++ }
			}
		}
	}

	// Agents（直接扫描优先，WSS RPC 次之）
	if u.OpenClawConfigPath != "" {
		scan := directScanOpenClaw(u.OpenClawConfigPath)
		agents, _ := directScanToOverviewData(scan)
		result.Agents = agents
		result.AgentCount = len(agents)
	} else if agentsErr == nil && agentsPayload != nil {
		raw, _ := agentsPayload["agents"].([]interface{})
		agents := interfaceSliceToMaps(raw)
		result.Agents = agents
		result.AgentCount = len(agents)
	}

	// Cron
	if cronErr == nil && cronPayload != nil {
		raw, _ := cronPayload["jobs"].([]interface{})
		result.CronCount = len(raw)
	}
}

// overviewViaToolsInvoke fallback: 走 tools/invoke（WSS 未就绪时）
func (api *ManagementAPI) overviewViaToolsInvoke(u *Upstream, result *gwOverviewResult) {
	var (
		statusResp *toolsInvokeResponse
		statusErr  error
		latency    int64
		innerWg    sync.WaitGroup
	)

	var sessPayload map[string]interface{}
	var sessErr error
	var agentsResp, cronResp *toolsInvokeResponse

	innerWg.Add(4)

	go func() {
		defer innerWg.Done()
		statusResp, latency, statusErr = gatewayToolsInvoke(
			u.Address, u.Port, u.PathPrefix, u.GatewayToken,
			toolsInvokeRequest{Tool: "session_status", Args: map[string]interface{}{}},
		)
	}()
	go func() {
		defer innerWg.Done()
		// 尝试一次性 WSS RPC sessions.list（旧方式）
		var rpcSess []map[string]interface{}
		rpcSess, _, sessErr = gatewaySessionsList(u.Address, u.Port, u.GatewayToken)
		if sessErr == nil {
			sessPayload = map[string]interface{}{"sessions": func() []interface{} {
				out := make([]interface{}, len(rpcSess))
				for i, s := range rpcSess { out[i] = s }
				return out
			}()}
		}
	}()
	go func() {
		defer innerWg.Done()
		agentsResp, _, _ = gatewayToolsInvoke(
			u.Address, u.Port, u.PathPrefix, u.GatewayToken,
			toolsInvokeRequest{Tool: "agents_list", Args: map[string]interface{}{}},
		)
	}()
	go func() {
		defer innerWg.Done()
		cronResp, _, _ = gatewayToolsInvoke(
			u.Address, u.Port, u.PathPrefix, u.GatewayToken,
			toolsInvokeRequest{Tool: "cron", Action: "list", Args: map[string]interface{}{}},
		)
	}()

	innerWg.Wait()
	result.LatencyMs = latency

	if statusErr != nil {
		errStr := statusErr.Error()
		if errStr == "AUTH_FAILED" {
			result.GatewayStatus = "auth_failed"
		} else {
			result.GatewayStatus = "unreachable"
			result.Error = errStr
		}
	} else if statusResp != nil && !statusResp.OK {
		result.GatewayStatus = "error"
		if statusResp.Error != nil { result.Error = statusResp.Error.Message }
	} else {
		result.GatewayStatus = "connected"
		if statusResp != nil && statusResp.Result != nil && len(statusResp.Result.Content) > 0 {
			result.StatusText = statusResp.Result.Content[0].Text
		}
	}

	// Sessions
	if sessErr == nil && sessPayload != nil {
		raw, _ := sessPayload["sessions"].([]interface{})
		sessions := interfaceSliceToMaps(raw)
		result.Sessions = sessions
		result.SessionCount = len(sessions)
		now := time.Now().UnixMilli()
		for _, s := range sessions {
			if state, ok := s["state"].(string); ok && isActiveSessionState(state) {
				result.ActiveSessions++
				continue
			}
			if updatedAt := extractTimestampMs(s, "updatedAt", "updated_at"); updatedAt > 0 {
				if now-updatedAt < 30*60*1000 { result.ActiveSessions++ }
			}
		}
	}

	// Agents
	if u.OpenClawConfigPath != "" {
		scan := directScanOpenClaw(u.OpenClawConfigPath)
		agents, _ := directScanToOverviewData(scan)
		result.Agents = agents
		result.AgentCount = len(agents)
	} else if agentsResp != nil && agentsResp.OK {
		agents := extractAgentsFromResponse(agentsResp)
		result.Agents = agents
		result.AgentCount = len(agents)
	}

	// Cron
	if cronResp != nil && cronResp.OK {
		jobs := extractCronJobsFromResponse(cronResp)
		result.CronCount = len(jobs)
	}
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

// extractTimestampMs 从 map 中提取毫秒级时间戳，支持多种字段名和类型
func extractTimestampMs(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			return int64(t)
		case int64:
			return t
		case json.Number:
			if n, err := t.Int64(); err == nil {
				return n
			}
		}
	}
	return 0
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
// v29.0: 优先 WSS RPC chat.history
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
			"error": "gateway_token_not_configured", "message": "请先配置 Gateway Token",
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
		if n, err := fmt.Sscanf(l, "%d", &limit); err != nil || n != 1 { limit = 30 }
	}
	if limit < 1 { limit = 1 }
	if limit > 100 { limit = 100 }

	// v29.0: 优先 WSS RPC chat.history
	payload, err := api.rpcCall(up, "chat.history", map[string]interface{}{
		"sessionKey": sessionKey, "limit": limit,
	})
	if err == nil {
		raw, _ := payload["messages"].([]interface{})
		messages := interfaceSliceToMaps(raw)
		jsonResponse(w, 200, map[string]interface{}{
			"messages": messages, "count": len(messages), "source": "wss_rpc",
		})
		return
	}

	// Fallback: tools/invoke
	resp, _, invokeErr := gatewayToolsInvoke(
		up.Address, up.Port, up.PathPrefix, up.GatewayToken,
		toolsInvokeRequest{Tool: "sessions_history", Args: map[string]interface{}{
			"sessionKey": sessionKey, "limit": limit, "includeTools": false,
		}},
	)
	if invokeErr != nil {
		jsonResponse(w, 502, map[string]interface{}{
			"error": "unreachable", "message": fmt.Sprintf("WSS: %v; fallback: %v", err, invokeErr),
		})
		return
	}
	if !resp.OK {
		errMsg := "sessions_history 调用失败"
		if resp.Error != nil { errMsg = resp.Error.Message }
		jsonResponse(w, 502, map[string]interface{}{"error": "api_error", "message": errMsg})
		return
	}
	messages := extractMessagesFromResponse(resp)
	jsonResponse(w, 200, map[string]interface{}{
		"messages": messages, "count": len(messages), "source": "tools_invoke_fallback",
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
// 通过 WSS RPC 获取上游 OpenClaw 实例的 skill 列表，聚合所有 agent
func (api *ManagementAPI) handleGatewaySkills(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}

	up, ok := api.pool.GetUpstream(id)
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "upstream not found"})
		return
	}

	// 优先通过 WSS RPC 获取
	if result, err := api.fetchSkillsViaRPC(up); err == nil {
		jsonResponse(w, 200, result)
		return
	}

	// WSS 不可用时返回空（不再扫描本地文件系统，因为生产环境龙虾卫士和 OpenClaw 不在同一台机器）
	jsonResponse(w, 200, map[string]interface{}{
		"skills":  nil,
		"count":   0,
		"summary": map[string]int{"global": 0, "user": 0, "workspace": 0},
		"source":  "unavailable",
		"warning": "WSS RPC 不可用，无法获取 skills",
	})
}

// fetchSkillsViaRPC 通过 WSS RPC 聚合所有 agent 的 skills
func (api *ManagementAPI) fetchSkillsViaRPC(up *Upstream) (map[string]interface{}, error) {
	// 1. 获取所有 agent
	agentsResult, err := api.rpcCall(up, "agents.list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	// 提取 agent ID 列表
	var agentIDs []string
	if agents, ok := agentsResult["agents"]; ok {
		if agentList, ok := agents.([]interface{}); ok {
			for _, a := range agentList {
				if agentMap, ok := a.(map[string]interface{}); ok {
					if aid, ok := agentMap["id"].(string); ok && aid != "" {
						agentIDs = append(agentIDs, aid)
					}
				}
			}
		}
	}

	// 如果拿不到 agent 列表，尝试不传 agentId（使用默认 agent）
	if len(agentIDs) == 0 {
		result, err := api.rpcCall(up, "skills.status", map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		result["source"] = "wss_rpc"
		return result, nil
	}

	// 2. 聚合每个 agent 的 skills
	seen := make(map[string]bool) // 按 category+name 去重
	var allSkills []interface{}
	globalCount, userCount, workspaceCount := 0, 0, 0

	for _, agentID := range agentIDs {
		result, err := api.rpcCall(up, "skills.status", map[string]interface{}{
			"agentId": agentID,
		})
		if err != nil {
			continue // 跳过失败的 agent
		}

		skills, ok := result["skills"]
		if !ok || skills == nil {
			continue
		}
		skillList, ok := skills.([]interface{})
		if !ok {
			continue
		}

		for _, s := range skillList {
			sm, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := sm["name"].(string)

			// RPC 返回 "source" 字段，前端需要 "category"
			// 映射: openclaw-core/openclaw-extra → global, user → user, workspace → workspace
			source, _ := sm["source"].(string)
			category, _ := sm["category"].(string)
			if category == "" {
				switch {
				case source == "openclaw-core" || source == "openclaw-extra":
					category = "global"
				case source == "user":
					category = "user"
				case strings.HasPrefix(source, "workspace"):
					category = "workspace:" + agentID
				default:
					category = "workspace:" + agentID
				}
				sm["category"] = category
			}

			// has_skill_md: RPC 返回 filePath，有值就说明有 SKILL.md
			if _, exists := sm["has_skill_md"]; !exists {
				if fp, _ := sm["filePath"].(string); fp != "" {
					sm["has_skill_md"] = true
				}
			}

			dedup := category + ":" + name
			if seen[dedup] {
				continue
			}
			seen[dedup] = true

			// 标记来源 agent
			sm["agentId"] = agentID
			allSkills = append(allSkills, sm)

			// 统计
			switch {
			case category == "global":
				globalCount++
			case category == "user":
				userCount++
			default:
				workspaceCount++
			}
		}
	}

	return map[string]interface{}{
		"skills": allSkills,
		"count":  len(allSkills),
		"summary": map[string]int{
			"global":    globalCount,
			"user":      userCount,
			"workspace": workspaceCount,
		},
		"source":   "wss_rpc",
		"agentIds": agentIDs,
	}, nil
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
			"key":              s.Key,
			"agent_id":         s.AgentID,
			"session_id":       s.SessionID,
			"last_modified_at": s.LastModifiedAt,
			"size_bytes":       s.SizeBytes,
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

// ============================================================
// v29.0 新增 API Handlers — 全部走 WSS RPC
// ============================================================

// handleGatewayWSSStatus GET /api/v1/gateway/wss/status
// 返回所有 WSS 连接的状态
func (api *ManagementAPI) handleGatewayWSSStatus(w http.ResponseWriter, r *http.Request) {
	if api.gwManager == nil {
		jsonResponse(w, 200, map[string]interface{}{"clients": []interface{}{}, "count": 0})
		return
	}
	stats := api.gwManager.Stats()
	jsonResponse(w, 200, map[string]interface{}{
		"clients": stats,
		"count":   len(stats),
	})
}

// handleGatewayModels GET /api/v1/upstreams/{id}/gateway/models
func (api *ManagementAPI) handleGatewayModels(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/models")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "models.list", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayChannels GET /api/v1/upstreams/{id}/gateway/channels
func (api *ManagementAPI) handleGatewayChannels(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/channels")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	probe := r.URL.Query().Get("probe") == "true"
	payload, err := api.rpcCall(up, "channels.status", map[string]interface{}{
		"probe": probe, "timeoutMs": 8000,
	})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayNodes GET /api/v1/upstreams/{id}/gateway/nodes
func (api *ManagementAPI) handleGatewayNodes(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/nodes")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "node.list", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayLogs GET /api/v1/upstreams/{id}/gateway/logs?limit=50
func (api *ManagementAPI) handleGatewayLogs(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/logs")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit < 1 { limit = 1 }
	if limit > 500 { limit = 500 }

	payload, err := api.rpcCall(up, "logs.tail", map[string]interface{}{"limit": limit})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayConfig GET /api/v1/upstreams/{id}/gateway/config
func (api *ManagementAPI) handleGatewayConfig(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/config")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "config.get", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayUsage GET /api/v1/upstreams/{id}/gateway/usage
func (api *ManagementAPI) handleGatewayUsage(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/usage")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 200, map[string]interface{}{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "usage.cost", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P0: Session 管理 ====================

// handleGatewaySessionPatch PATCH /api/v1/upstreams/{id}/gateway/session
func (api *ManagementAPI) handleGatewaySessionPatch(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/session")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	if _, ok := body["key"]; !ok {
		jsonResponse(w, 400, map[string]string{"error": "missing_key"})
		return
	}

	payload, err := api.rpcCall(up, "sessions.patch", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySessionDelete DELETE /api/v1/upstreams/{id}/gateway/session?key=xxx
func (api *ManagementAPI) handleGatewaySessionDelete(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/session")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	sessionKey := r.URL.Query().Get("key")
	if sessionKey == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_key"})
		return
	}

	payload, err := api.rpcCall(up, "sessions.delete", map[string]interface{}{
		"key":              sessionKey,
		"deleteTranscript": true,
	})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySessionReset POST /api/v1/upstreams/{id}/gateway/session/reset
func (api *ManagementAPI) handleGatewaySessionReset(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/session/reset")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	key, _ := body["key"].(string)
	if key == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_key"})
		return
	}

	payload, err := api.rpcCall(up, "sessions.reset", map[string]interface{}{"key": key})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySessionCompact POST /api/v1/upstreams/{id}/gateway/session/compact
func (api *ManagementAPI) handleGatewaySessionCompact(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/session/compact")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	key, _ := body["key"].(string)
	if key == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_key"})
		return
	}

	payload, err := api.rpcCall(up, "sessions.compact", map[string]interface{}{"key": key})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P0: Chat 操作 ====================

// handleGatewayChatSend POST /api/v1/upstreams/{id}/gateway/chat/send
func (api *ManagementAPI) handleGatewayChatSend(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/chat/send")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	sessionKey, _ := body["sessionKey"].(string)
	message, _ := body["message"].(string)
	if sessionKey == "" || message == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_sessionKey_or_message"})
		return
	}

	payload, err := api.rpcCall(up, "chat.send", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayChatAbort POST /api/v1/upstreams/{id}/gateway/chat/abort
func (api *ManagementAPI) handleGatewayChatAbort(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/chat/abort")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	sessionKey, _ := body["sessionKey"].(string)
	if sessionKey == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_sessionKey"})
		return
	}

	payload, err := api.rpcCall(up, "chat.abort", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P0: Cron CRUD ====================

// handleGatewayCronAdd POST /api/v1/upstreams/{id}/gateway/cron/add
func (api *ManagementAPI) handleGatewayCronAdd(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron/add")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "cron.add", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayCronUpdate PUT /api/v1/upstreams/{id}/gateway/cron/update
func (api *ManagementAPI) handleGatewayCronUpdate(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron/update")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	if _, ok := body["id"]; !ok {
		jsonResponse(w, 400, map[string]string{"error": "missing_id"})
		return
	}

	payload, err := api.rpcCall(up, "cron.update", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayCronRemove DELETE /api/v1/upstreams/{id}/gateway/cron/remove?id=xxx
func (api *ManagementAPI) handleGatewayCronRemove(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron/remove")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		// 也尝试从 body 读
		var body map[string]interface{}
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			jobID, _ = body["id"].(string)
		}
	}
	if jobID == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_id"})
		return
	}

	payload, err := api.rpcCall(up, "cron.remove", map[string]interface{}{"id": jobID})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayCronRun POST /api/v1/upstreams/{id}/gateway/cron/run
func (api *ManagementAPI) handleGatewayCronRun(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron/run")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	jobID, _ := body["id"].(string)
	if jobID == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_id"})
		return
	}
	if _, ok := body["mode"]; !ok {
		body["mode"] = "force"
	}

	payload, err := api.rpcCall(up, "cron.run", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayCronRuns GET /api/v1/upstreams/{id}/gateway/cron/runs?id=xxx&limit=10
func (api *ManagementAPI) handleGatewayCronRuns(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/cron/runs")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_id"})
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	payload, err := api.rpcCall(up, "cron.runs", map[string]interface{}{"id": jobID, "limit": limit})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: Agent 生命周期 ====================

// handleGatewayAgentCreate POST /api/v1/upstreams/{id}/gateway/agents/create
func (api *ManagementAPI) handleGatewayAgentCreate(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/create")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "agents.create", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayAgentUpdate PUT /api/v1/upstreams/{id}/gateway/agents/update
func (api *ManagementAPI) handleGatewayAgentUpdate(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/update")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "agents.update", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayAgentDelete DELETE /api/v1/upstreams/{id}/gateway/agents/delete?agentId=xxx
func (api *ManagementAPI) handleGatewayAgentDelete(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/delete")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	agentID := r.URL.Query().Get("agentId")
	if agentID == "" {
		var body map[string]interface{}
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			agentID, _ = body["agentId"].(string)
		}
	}
	if agentID == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_agentId"})
		return
	}

	payload, err := api.rpcCall(up, "agents.delete", map[string]interface{}{"agentId": agentID})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayAgentFiles GET /api/v1/upstreams/{id}/gateway/agents/files?agentId=xxx
func (api *ManagementAPI) handleGatewayAgentFiles(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/files")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	agentID := r.URL.Query().Get("agentId")
	params := map[string]interface{}{}
	if agentID != "" {
		params["agentId"] = agentID
	}

	payload, err := api.rpcCall(up, "agents.files.list", params)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayAgentFileGet GET /api/v1/upstreams/{id}/gateway/agents/file?agentId=xxx&name=SOUL.md
func (api *ManagementAPI) handleGatewayAgentFileGet(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/file")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	agentID := r.URL.Query().Get("agentId")
	name := r.URL.Query().Get("name")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_name"})
		return
	}

	params := map[string]interface{}{"name": name}
	if agentID != "" {
		params["agentId"] = agentID
	}

	payload, err := api.rpcCall(up, "agents.files.get", params)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayAgentFileSet PUT /api/v1/upstreams/{id}/gateway/agents/file
func (api *ManagementAPI) handleGatewayAgentFileSet(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/agents/file")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	name, _ := body["name"].(string)
	content, _ := body["content"].(string)
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_name"})
		return
	}

	payload, err := api.rpcCall(up, "agents.files.set", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	_ = content
	jsonResponse(w, 200, payload)
}

// ==================== P1: Config 修改 ====================

// handleGatewayConfigPatch PATCH /api/v1/upstreams/{id}/gateway/config
func (api *ManagementAPI) handleGatewayConfigPatch(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/config")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "config.patch", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayConfigSchema GET /api/v1/upstreams/{id}/gateway/config/schema
func (api *ManagementAPI) handleGatewayConfigSchema(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/config/schema")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "config.schema", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: Skills 管理 ====================

// handleGatewaySkillsBins GET /api/v1/upstreams/{id}/gateway/skills/bins
func (api *ManagementAPI) handleGatewaySkillsBins(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills/bins")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	params := map[string]interface{}{}
	if agentID := r.URL.Query().Get("agentId"); agentID != "" {
		params["agentId"] = agentID
	}

	payload, err := api.rpcCall(up, "skills.bins", params)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySkillsInstall POST /api/v1/upstreams/{id}/gateway/skills/install
func (api *ManagementAPI) handleGatewaySkillsInstall(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills/install")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "skills.install", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySkillsUpdate POST /api/v1/upstreams/{id}/gateway/skills/update
func (api *ManagementAPI) handleGatewaySkillsUpdate(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills/update")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "skills.update", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: 心跳管理 ====================

// handleGatewayHeartbeat GET /api/v1/upstreams/{id}/gateway/heartbeat
func (api *ManagementAPI) handleGatewayHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/heartbeat")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "last-heartbeat", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewaySetHeartbeats PUT /api/v1/upstreams/{id}/gateway/heartbeat
func (api *ManagementAPI) handleGatewaySetHeartbeats(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/heartbeat")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "set-heartbeats", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayWake POST /api/v1/upstreams/{id}/gateway/wake
func (api *ManagementAPI) handleGatewayWake(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/wake")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "wake", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: 设备配对 ====================

// handleGatewayDevicePairs GET /api/v1/upstreams/{id}/gateway/devices
func (api *ManagementAPI) handleGatewayDevicePairs(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/devices")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "device.pair.list", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayDevicePairAction POST /api/v1/upstreams/{id}/gateway/devices/approve or /reject
func (api *ManagementAPI) handleGatewayDevicePairAction(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var method string
	var suffix string
	if strings.HasSuffix(path, "/approve") {
		method = "device.pair.approve"
		suffix = "/gateway/devices/approve"
	} else if strings.HasSuffix(path, "/reject") {
		method = "device.pair.reject"
		suffix = "/gateway/devices/reject"
	} else {
		jsonResponse(w, 400, map[string]string{"error": "unknown_action"})
		return
	}

	id := extractUpstreamIDFromGatewayPath(path, suffix)
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, method, body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: 节点管理 ====================

// handleGatewayNodePairs GET /api/v1/upstreams/{id}/gateway/node-pairs
func (api *ManagementAPI) handleGatewayNodePairs(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/node-pairs")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	payload, err := api.rpcCall(up, "node.pair.list", map[string]interface{}{})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayNodePairAction POST /api/v1/upstreams/{id}/gateway/node-pairs/approve or /reject
func (api *ManagementAPI) handleGatewayNodePairAction(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	var method string
	var suffix string
	if strings.HasSuffix(path, "/approve") {
		method = "node.pair.approve"
		suffix = "/gateway/node-pairs/approve"
	} else if strings.HasSuffix(path, "/reject") {
		method = "node.pair.reject"
		suffix = "/gateway/node-pairs/reject"
	} else {
		jsonResponse(w, 400, map[string]string{"error": "unknown_action"})
		return
	}

	id := extractUpstreamIDFromGatewayPath(path, suffix)
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, method, body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayNodeDescribe GET /api/v1/upstreams/{id}/gateway/nodes/describe?node=xxx
func (api *ManagementAPI) handleGatewayNodeDescribe(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/nodes/describe")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	nodeID := r.URL.Query().Get("node")
	if nodeID == "" {
		jsonResponse(w, 400, map[string]string{"error": "missing_node"})
		return
	}

	payload, err := api.rpcCall(up, "node.describe", map[string]interface{}{"node": nodeID})
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// handleGatewayNodeRename POST /api/v1/upstreams/{id}/gateway/nodes/rename
func (api *ManagementAPI) handleGatewayNodeRename(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/nodes/rename")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "node.rename", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ==================== P1: 系统事件 ====================

// handleGatewaySystemEvent POST /api/v1/upstreams/{id}/gateway/system-event
func (api *ManagementAPI) handleGatewaySystemEvent(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/system-event")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid_json"})
		return
	}

	payload, err := api.rpcCall(up, "system-event", body)
	if err != nil {
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

// ===== v29.0 P2: 执行审批 / Gateway 控制 / 记忆 / Skill 卸载 =====

func (api *ManagementAPI) handleGatewayExecApprovals(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/exec-approvals")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	payload, err := api.rpcCall(up, "exec.approvals.list", map[string]interface{}{})
	if err != nil {
		// Gateway 不支持此方法时优雅降级为空列表
		if strings.Contains(err.Error(), "unknown method") {
			jsonResponse(w, 200, map[string]interface{}{"items": []interface{}{}, "source": "not_supported"})
			return
		}
		jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()})
		return
	}
	jsonResponse(w, 200, payload)
}

func (api *ManagementAPI) handleGatewayExecApprovalAction(w http.ResponseWriter, r *http.Request, approve bool) {
	suffix := "/gateway/exec-approvals/approve"
	if !approve { suffix = "/gateway/exec-approvals/reject" }
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, suffix)
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { jsonResponse(w, 400, map[string]string{"error": "invalid_json"}); return }
	method := "exec.approvals.approve"
	if !approve { method = "exec.approvals.reject" }
	payload, err := api.rpcCall(up, method, body)
	if err != nil { jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()}); return }
	jsonResponse(w, 200, payload)
}

func (api *ManagementAPI) handleGatewayRestart(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/restart")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	if body == nil { body = map[string]interface{}{} }
	payload, err := api.rpcCall(up, "gateway.restart", body)
	if err != nil { jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()}); return }
	jsonResponse(w, 200, payload)
}

func (api *ManagementAPI) handleGatewayUpdate(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/update")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	if body == nil { body = map[string]interface{}{} }
	payload, err := api.rpcCall(up, "gateway.update", body)
	if err != nil { jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()}); return }
	jsonResponse(w, 200, payload)
}

func (api *ManagementAPI) handleGatewayMemorySearch(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/memory/search")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { jsonResponse(w, 400, map[string]string{"error": "invalid_json"}); return }
	payload, err := api.rpcCall(up, "memory.search", body)
	if err != nil { jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()}); return }
	jsonResponse(w, 200, payload)
}

func (api *ManagementAPI) handleGatewaySkillUninstall(w http.ResponseWriter, r *http.Request) {
	id := extractUpstreamIDFromGatewayPath(r.URL.Path, "/gateway/skills/uninstall")
	up, ok := api.pool.GetUpstream(id)
	if !ok { jsonResponse(w, 404, map[string]string{"error": "upstream not found"}); return }
	if up.GatewayToken == "" { jsonResponse(w, 400, map[string]string{"error": "gateway_token_not_configured"}); return }
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { jsonResponse(w, 400, map[string]string{"error": "invalid_json"}); return }
	payload, err := api.rpcCall(up, "skills.uninstall", body)
	if err != nil { jsonResponse(w, 502, map[string]interface{}{"error": "rpc_failed", "message": err.Error()}); return }
	jsonResponse(w, 200, payload)
}
