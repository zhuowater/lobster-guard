// gateway_ws_client.go — 持久化 WSS 连接到 OpenClaw Gateway
// 复刻 OpenClaw Control UI 的完整 WSS RPC 协议
// lobster-guard v29.0
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ============================================================
// WSS RPC 帧定义（与 OpenClaw Gateway 协议完全一致）
// ============================================================

// gwRPCFrame 通用帧
type gwRPCFrame struct {
	Type string `json:"type"` // "req" | "res" | "event"
}

// gwReqFrame 请求帧
type gwReqFrame struct {
	Type   string      `json:"type"`   // "req"
	ID     string      `json:"id"`     // UUID
	Method string      `json:"method"` // RPC 方法名
	Params interface{} `json:"params,omitempty"`
}

// gwResFrame 响应帧
type gwResFrame struct {
	Type    string                 `json:"type"` // "res"
	ID      string                 `json:"id"`
	OK      bool                   `json:"ok"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Error   *gwRPCError            `json:"error,omitempty"`
}

// gwRPCError 错误体
type gwRPCError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// gwEventFrame 事件帧
type gwEventFrame struct {
	Type    string                 `json:"type"` // "event"
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Seq     *int64                 `json:"seq,omitempty"`
}

// gwConnectParams connect RPC 参数（复刻 Control UI）
type gwConnectParams struct {
	MinProtocol int                    `json:"minProtocol"`
	MaxProtocol int                    `json:"maxProtocol"`
	Client      gwClientInfo           `json:"client"`
	Role        string                 `json:"role"`
	Scopes      []string               `json:"scopes"`
	Caps        []string               `json:"caps"`
	Auth        *gwAuth                `json:"auth,omitempty"`
	UserAgent   string                 `json:"userAgent,omitempty"`
	Locale      string                 `json:"locale,omitempty"`
}

type gwClientInfo struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	Platform   string `json:"platform"`
	Mode       string `json:"mode"`
	InstanceID string `json:"instanceId,omitempty"`
}

type gwAuth struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

// ============================================================
// 连接状态
// ============================================================

type gwConnState int32

const (
	gwStateDisconnected gwConnState = 0
	gwStateConnecting   gwConnState = 1
	gwStateConnected    gwConnState = 2
	gwStateClosed       gwConnState = 3
)

func (s gwConnState) String() string {
	switch s {
	case gwStateDisconnected:
		return "disconnected"
	case gwStateConnecting:
		return "connecting"
	case gwStateConnected:
		return "connected"
	case gwStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ============================================================
// Gateway WSS Client — 持久化连接
// ============================================================

// GatewayWSClient 持久化 WSS 连接到一个 OpenClaw Gateway
type GatewayWSClient struct {
	// 配置（不可变）
	upstreamID string
	address    string
	port       int
	token      string
	origin     string // Gateway controlUi 的 HTTPS origin（匹配 allowedOrigins）
	instanceID string

	// 连接状态
	state    int32 // atomic gwConnState
	conn     *websocket.Conn
	connMu   sync.Mutex
	writeMu  sync.Mutex // 保护 conn.WriteMessage（gorilla/websocket 不支持并发写）
	closed   int32 // atomic bool

	// 请求-响应配对
	pending   map[string]chan *gwResFrame
	pendingMu sync.Mutex

	// Hello payload 快照
	hello   map[string]interface{}
	helloMu sync.RWMutex

	// 事件回调
	onEvent func(upstreamID string, event *gwEventFrame)

	// 重连控制
	backoffMs   int64
	reconnectMu sync.Mutex

	// 统计
	totalRequests   int64
	totalErrors     int64
	lastConnectedAt int64 // unix ms
	lastErrorMsg    string
	lastErrorAt     int64
	statsMu         sync.RWMutex
}

// GatewayWSClientConfig 创建配置
type GatewayWSClientConfig struct {
	UpstreamID string
	Address    string
	Port       int
	Token      string
	Origin     string // HTTPS origin for controlUi allowedOrigins
	OnEvent    func(upstreamID string, event *gwEventFrame)
}

// NewGatewayWSClient 创建并启动客户端
func NewGatewayWSClient(cfg GatewayWSClientConfig) *GatewayWSClient {
	c := &GatewayWSClient{
		upstreamID: cfg.UpstreamID,
		address:    cfg.Address,
		port:       cfg.Port,
		token:      cfg.Token,
		origin:     cfg.Origin,
		instanceID: uuid.New().String(),
		pending:    make(map[string]chan *gwResFrame),
		onEvent:    cfg.OnEvent,
		backoffMs:  800,
	}
	go c.connectLoop()
	return c
}

// ============================================================
// 公开方法
// ============================================================

// Request 发送 RPC 请求，等待响应，带超时
func (c *GatewayWSClient) Request(method string, params interface{}, timeout time.Duration) (map[string]interface{}, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if gwConnState(atomic.LoadInt32(&c.state)) != gwStateConnected {
		return nil, fmt.Errorf("not connected (state=%s)", gwConnState(atomic.LoadInt32(&c.state)))
	}

	reqID := fmt.Sprintf("lg-%s", uuid.New().String()[:8])
	frame := gwReqFrame{
		Type:   "req",
		ID:     reqID,
		Method: method,
		Params: params,
	}

	ch := make(chan *gwResFrame, 1)
	c.pendingMu.Lock()
	c.pending[reqID] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, reqID)
		c.pendingMu.Unlock()
	}()

	atomic.AddInt64(&c.totalRequests, 1)

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("no active connection")
	}

	data, _ := json.Marshal(frame)
	c.writeMu.Lock()
	writeErr := conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	if writeErr != nil {
		c.recordError("write failed: " + writeErr.Error())
		return nil, fmt.Errorf("write failed: %w", writeErr)
	}

	select {
	case res := <-ch:
		if res == nil {
			return nil, fmt.Errorf("connection closed while waiting")
		}
		if !res.OK {
			msg := "rpc failed"
			if res.Error != nil && res.Error.Message != "" {
				msg = res.Error.Message
			}
			return nil, fmt.Errorf("%s", msg)
		}
		return res.Payload, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout after %s", timeout)
	}
}

// Close 关闭客户端（不再重连）
func (c *GatewayWSClient) Close() {
	atomic.StoreInt32(&c.closed, 1)
	atomic.StoreInt32(&c.state, int32(gwStateClosed))
	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()
	c.flushPending()
}

// UpdateToken 热更新 token（下次重连生效）
func (c *GatewayWSClient) UpdateToken(token string) {
	c.connMu.Lock()
	c.token = token
	c.connMu.Unlock()
}

// IsConnected 是否已连接
func (c *GatewayWSClient) IsConnected() bool {
	return gwConnState(atomic.LoadInt32(&c.state)) == gwStateConnected
}

// State 返回连接状态字符串
func (c *GatewayWSClient) State() string {
	return gwConnState(atomic.LoadInt32(&c.state)).String()
}

// Hello 返回最近的 hello snapshot
func (c *GatewayWSClient) Hello() map[string]interface{} {
	c.helloMu.RLock()
	defer c.helloMu.RUnlock()
	return c.hello
}

// Stats 返回客户端统计
func (c *GatewayWSClient) Stats() map[string]interface{} {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	return map[string]interface{}{
		"upstream_id":      c.upstreamID,
		"state":            c.State(),
		"total_requests":   atomic.LoadInt64(&c.totalRequests),
		"total_errors":     atomic.LoadInt64(&c.totalErrors),
		"last_connected_at": c.lastConnectedAt,
		"last_error":       c.lastErrorMsg,
		"last_error_at":    c.lastErrorAt,
	}
}

// ============================================================
// 连接循环（内部）
// ============================================================

func (c *GatewayWSClient) connectLoop() {
	for {
		if atomic.LoadInt32(&c.closed) != 0 {
			return
		}
		c.doConnect()
		if atomic.LoadInt32(&c.closed) != 0 {
			return
		}
		// 重连退避
		backoff := atomic.LoadInt64(&c.backoffMs)
		time.Sleep(time.Duration(backoff) * time.Millisecond)
		newBackoff := backoff * 17 / 10
		if newBackoff > 15000 {
			newBackoff = 15000
		}
		atomic.StoreInt64(&c.backoffMs, newBackoff)
	}
}

func (c *GatewayWSClient) doConnect() {
	atomic.StoreInt32(&c.state, int32(gwStateConnecting))

	c.connMu.Lock()
	token := c.token
	c.connMu.Unlock()

	wsURL := fmt.Sprintf("ws://%s:%d/", c.address, c.port)

	headers := http.Header{}
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	// Gateway controlUi 要求 HTTPS（secure context）
	// 本地连接时使用 https:// origin 匹配 allowedOrigins 白名单
	if c.origin != "" {
		headers.Set("Origin", c.origin)
	} else {
		headers.Set("Origin", fmt.Sprintf("https://%s", c.address))
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		c.recordError(fmt.Sprintf("dial failed (HTTP %d): %v", status, err))
		atomic.StoreInt32(&c.state, int32(gwStateDisconnected))
		return
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	log.Printf("[GW-WSS][%s] WebSocket 连接已建立 -> %s", c.upstreamID, wsURL)

	// 读循环，处理 challenge -> connect -> hello -> events/responses
	c.readLoop(conn, token)

	// 读循环退出意味着连接断开
	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.connMu.Unlock()
	conn.Close()
	c.flushPending()
	atomic.StoreInt32(&c.state, int32(gwStateDisconnected))
	log.Printf("[GW-WSS][%s] 连接断开，将重连", c.upstreamID)
}

func (c *GatewayWSClient) readLoop(conn *websocket.Conn, token string) {
	connectSent := false
	// connect 响应通过专用 channel 异步接收（避免 readLoop 死锁）
	connectCh := make(chan *gwResFrame, 1)
	var connectReqID string

	connectTimer := time.AfterFunc(2*time.Second, func() {
		// 如果 2 秒内没收到 challenge，直接发 connect
		if !connectSent {
			connectSent = true
			connectReqID = c.sendConnectAsync(conn, token, "")
		}
	})
	defer connectTimer.Stop()

	// connect 超时器
	connectTimeout := time.AfterFunc(15*time.Second, func() {
		if gwConnState(atomic.LoadInt32(&c.state)) != gwStateConnected {
			c.recordError("connect timeout (15s)")
			conn.Close()
		}
	})
	defer connectTimeout.Stop()

	for {
		if atomic.LoadInt32(&c.closed) != 0 {
			return
		}

		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if atomic.LoadInt32(&c.closed) == 0 {
				c.recordError("read: " + err.Error())
			}
			return
		}

		// 解析帧类型
		var probe gwRPCFrame
		if json.Unmarshal(data, &probe) != nil {
			continue
		}

		switch probe.Type {
		case "event":
			var evt gwEventFrame
			if json.Unmarshal(data, &evt) != nil {
				continue
			}

			// connect.challenge → 发 connect（异步，不阻塞 readLoop）
			if evt.Event == "connect.challenge" {
				connectTimer.Stop()
				nonce := ""
				if evt.Payload != nil {
					if n, ok := evt.Payload["nonce"].(string); ok {
						nonce = n
					}
				}
				if !connectSent {
					connectSent = true
					connectReqID = c.sendConnectAsync(conn, token, nonce)
				}
				continue
			}

			// 转发事件给回调
			if c.onEvent != nil {
				c.onEvent(c.upstreamID, &evt)
			}

		case "res":
			var res gwResFrame
			if json.Unmarshal(data, &res) != nil {
				continue
			}

			// connect 响应特殊处理
			if connectReqID != "" && res.ID == connectReqID {
				connectTimeout.Stop()
				select {
				case connectCh <- &res:
				default:
				}
				// 处理 connect 结果
				c.handleConnectResponse(&res, connectCh)
				continue
			}

			c.handleResponse(&res)
		}
	}
}

// sendConnectAsync 发送 connect RPC 帧（不阻塞，响应由 readLoop 处理）
func (c *GatewayWSClient) sendConnectAsync(conn *websocket.Conn, token, nonce string) string {
	params := gwConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client: gwClientInfo{
			ID:         "openclaw-control-ui", // 必须匹配 Gateway 白名单
			Version:    "v29.0-lobster-guard",
			Platform:   "linux",
			Mode:       "webchat",             // Gateway 允许的 mode
			InstanceID: c.instanceID,
		},
		Role:   "operator",
		Scopes: []string{"operator.admin", "operator.approvals", "operator.pairing"},
		Caps:   []string{},
		Auth: &gwAuth{
			Token: token,
		},
		Locale: "en",
	}

	reqID := fmt.Sprintf("lg-connect-%s", uuid.New().String()[:8])
	frame := gwReqFrame{
		Type:   "req",
		ID:     reqID,
		Method: "connect",
		Params: params,
	}

	data, _ := json.Marshal(frame)
	c.writeMu.Lock()
	writeErr := conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()
	if writeErr != nil {
		c.recordError("connect write: " + writeErr.Error())
		return ""
	}

	log.Printf("[GW-WSS][%s] 发送 connect 请求 (id=%s)", c.upstreamID, reqID)
	return reqID
}

// handleConnectResponse 处理 connect 响应（在 readLoop 中内联调用）
func (c *GatewayWSClient) handleConnectResponse(res *gwResFrame, _ chan *gwResFrame) {
	if !res.OK {
		msg := "connect failed"
		if res.Error != nil && res.Error.Message != "" {
			msg = res.Error.Message
		}
		c.recordError("connect rejected: " + msg)
		return
	}

	// 成功！保存 hello snapshot
	c.helloMu.Lock()
	c.hello = res.Payload
	c.helloMu.Unlock()

	atomic.StoreInt32(&c.state, int32(gwStateConnected))
	atomic.StoreInt64(&c.backoffMs, 800) // 重置退避
	c.statsMu.Lock()
	c.lastConnectedAt = time.Now().UnixMilli()
	c.statsMu.Unlock()

	log.Printf("[GW-WSS][%s] ✅ 已连接并认证成功 (hello payload: %d keys)", c.upstreamID, len(res.Payload))
}

// handleResponse 分发响应到 pending channel
func (c *GatewayWSClient) handleResponse(res *gwResFrame) {
	c.pendingMu.Lock()
	ch, ok := c.pending[res.ID]
	if ok {
		delete(c.pending, res.ID)
	}
	c.pendingMu.Unlock()

	if ok && ch != nil {
		select {
		case ch <- res:
		default:
		}
	}
}

// flushPending 清空所有待处理请求
func (c *GatewayWSClient) flushPending() {
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// recordError 记录错误
func (c *GatewayWSClient) recordError(msg string) {
	atomic.AddInt64(&c.totalErrors, 1)
	c.statsMu.Lock()
	c.lastErrorMsg = msg
	c.lastErrorAt = time.Now().UnixMilli()
	c.statsMu.Unlock()
	if !strings.Contains(msg, "use of closed") {
		log.Printf("[GW-WSS][%s] ⚠ %s", c.upstreamID, msg)
	}
}

// ============================================================
// Gateway WSS Client Manager — 管理多上游连接池
// ============================================================

// GatewayWSManager 管理所有上游 WSS 连接
type GatewayWSManager struct {
	clients       map[string]*GatewayWSClient // key: upstream ID
	mu            sync.RWMutex
	defaultOrigin string // v29.0: 全局默认 Origin，从 config 读取
	onEvent       func(upstreamID string, event *gwEventFrame)
}

// NewGatewayWSManager 创建连接管理器
func NewGatewayWSManager(onEvent func(string, *gwEventFrame), defaultOrigin string) *GatewayWSManager {
	if defaultOrigin == "" {
		defaultOrigin = "http://localhost"
	}
	return &GatewayWSManager{
		clients:       make(map[string]*GatewayWSClient),
		defaultOrigin: defaultOrigin,
		onEvent:       onEvent,
	}
}

// EnsureClient 确保为指定上游创建 WSS 客户端
// 如果已存在且 token 没变则复用，否则重建
func (m *GatewayWSManager) EnsureClient(up *Upstream) *GatewayWSClient {
	if up.GatewayToken == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.clients[up.ID]
	if ok {
		// token 没变且没关闭，复用
		if gwConnState(atomic.LoadInt32(&existing.state)) != gwStateClosed {
			existing.connMu.Lock()
			sameToken := existing.token == up.GatewayToken
			existing.connMu.Unlock()
			if sameToken {
				return existing
			}
		}
		// token 变了或已关闭，先关旧的
		existing.Close()
	}

	client := NewGatewayWSClient(GatewayWSClientConfig{
		UpstreamID: up.ID,
		Address:    up.Address,
		Port:       up.Port,
		Token:      up.GatewayToken,
		Origin:     m.resolveOrigin(up.GatewayOrigin),
		OnEvent:    m.onEvent,
	})
	m.clients[up.ID] = client
	return client
}

// resolveOrigin 返回上游 origin，fallback 到全局默认
func (m *GatewayWSManager) resolveOrigin(upstreamOrigin string) string {
	if upstreamOrigin != "" {
		return upstreamOrigin
	}
	return m.defaultOrigin
}

// GetClient 获取已有客户端（不创建）
func (m *GatewayWSManager) GetClient(upstreamID string) *GatewayWSClient {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[upstreamID]
}

// RemoveClient 移除并关闭客户端
func (m *GatewayWSManager) RemoveClient(upstreamID string) {
	m.mu.Lock()
	c, ok := m.clients[upstreamID]
	if ok {
		delete(m.clients, upstreamID)
	}
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
}

// CloseAll 关闭所有连接
func (m *GatewayWSManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.clients {
		c.Close()
		delete(m.clients, id)
	}
}

// Stats 返回所有客户端的统计
func (m *GatewayWSManager) Stats() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(m.clients))
	for _, c := range m.clients {
		out = append(out, c.Stats())
	}
	return out
}

// ============================================================
// 便捷 RPC 方法 — 封装常用 Gateway 操作
// ============================================================

// GWStatus 获取 Gateway 状态
func (c *GatewayWSClient) GWStatus() (map[string]interface{}, error) {
	return c.Request("status", map[string]interface{}{}, 10*time.Second)
}

// GWHealth 获取 Gateway 健康状态
func (c *GatewayWSClient) GWHealth() (map[string]interface{}, error) {
	return c.Request("health", map[string]interface{}{}, 10*time.Second)
}

// GWSessionsList 获取 sessions 列表
func (c *GatewayWSClient) GWSessionsList(opts map[string]interface{}) (map[string]interface{}, error) {
	if opts == nil {
		opts = map[string]interface{}{
			"includeGlobal":  true,
			"includeUnknown": true,
			"limit":          500,
		}
	}
	return c.Request("sessions.list", opts, 15*time.Second)
}

// GWSessionsUsage 获取 sessions usage
func (c *GatewayWSClient) GWSessionsUsage(opts map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("sessions.usage", opts, 15*time.Second)
}

// GWUsageCost 获取 usage cost
func (c *GatewayWSClient) GWUsageCost(opts map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("usage.cost", opts, 15*time.Second)
}

// GWChatHistory 获取聊天历史
func (c *GatewayWSClient) GWChatHistory(sessionKey string, limit int) (map[string]interface{}, error) {
	return c.Request("chat.history", map[string]interface{}{
		"sessionKey": sessionKey,
		"limit":      limit,
	}, 15*time.Second)
}

// GWAgentsList 获取 agents 列表
func (c *GatewayWSClient) GWAgentsList() (map[string]interface{}, error) {
	return c.Request("agents.list", map[string]interface{}{}, 10*time.Second)
}

// GWAgentIdentity 获取 agent identity
func (c *GatewayWSClient) GWAgentIdentity(agentID string) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	if agentID != "" {
		params["agentId"] = agentID
	}
	return c.Request("agent.identity.get", params, 10*time.Second)
}

// GWCronStatus 获取 cron 状态
func (c *GatewayWSClient) GWCronStatus() (map[string]interface{}, error) {
	return c.Request("cron.status", map[string]interface{}{}, 10*time.Second)
}

// GWCronList 获取 cron 任务列表
func (c *GatewayWSClient) GWCronList() (map[string]interface{}, error) {
	return c.Request("cron.list", map[string]interface{}{"includeDisabled": true}, 10*time.Second)
}

// GWCronRun 触发运行 cron 任务
func (c *GatewayWSClient) GWCronRun(jobID string) (map[string]interface{}, error) {
	return c.Request("cron.run", map[string]interface{}{"id": jobID, "mode": "force"}, 15*time.Second)
}

// GWCronRuns 获取 cron 运行历史
func (c *GatewayWSClient) GWCronRuns(jobID string, limit int) (map[string]interface{}, error) {
	return c.Request("cron.runs", map[string]interface{}{"id": jobID, "limit": limit}, 10*time.Second)
}

// GWSkillsStatus 获取 skills 状态
func (c *GatewayWSClient) GWSkillsStatus(agentID string) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	if agentID != "" {
		params["agentId"] = agentID
	}
	return c.Request("skills.status", params, 10*time.Second)
}

// GWNodeList 获取 nodes 列表
func (c *GatewayWSClient) GWNodeList() (map[string]interface{}, error) {
	return c.Request("node.list", map[string]interface{}{}, 10*time.Second)
}

// GWLogsTail 获取日志尾部
func (c *GatewayWSClient) GWLogsTail(cursor interface{}, limit int) (map[string]interface{}, error) {
	params := map[string]interface{}{"limit": limit}
	if cursor != nil {
		params["cursor"] = cursor
	}
	return c.Request("logs.tail", params, 10*time.Second)
}

// GWModelsList 获取模型列表
func (c *GatewayWSClient) GWModelsList() (map[string]interface{}, error) {
	return c.Request("models.list", map[string]interface{}{}, 10*time.Second)
}

// GWChannelsStatus 获取 channels 状态
func (c *GatewayWSClient) GWChannelsStatus(probe bool) (map[string]interface{}, error) {
	return c.Request("channels.status", map[string]interface{}{
		"probe":     probe,
		"timeoutMs": 8000,
	}, 15*time.Second)
}

// GWSystemPresence 获取系统存在
func (c *GatewayWSClient) GWSystemPresence() (map[string]interface{}, error) {
	return c.Request("system-presence", map[string]interface{}{}, 10*time.Second)
}

// GWConfigGet 获取配置
func (c *GatewayWSClient) GWConfigGet() (map[string]interface{}, error) {
	return c.Request("config.get", map[string]interface{}{}, 10*time.Second)
}

// GWConfigSchema 获取配置 schema
func (c *GatewayWSClient) GWConfigSchema() (map[string]interface{}, error) {
	return c.Request("config.schema", map[string]interface{}{}, 10*time.Second)
}

// GWLastHeartbeat 获取最后心跳
func (c *GatewayWSClient) GWLastHeartbeat() (map[string]interface{}, error) {
	return c.Request("last-heartbeat", map[string]interface{}{}, 10*time.Second)
}

// GWDevicePairList 获取设备配对列表
func (c *GatewayWSClient) GWDevicePairList() (map[string]interface{}, error) {
	return c.Request("device.pair.list", map[string]interface{}{}, 10*time.Second)
}

// GWSessionsPatch 修改 session
func (c *GatewayWSClient) GWSessionsPatch(sessionKey string, patch map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{"key": sessionKey}
	for k, v := range patch {
		params[k] = v
	}
	return c.Request("sessions.patch", params, 10*time.Second)
}

// GWSessionsDelete 删除 session
func (c *GatewayWSClient) GWSessionsDelete(sessionKey string) (map[string]interface{}, error) {
	return c.Request("sessions.delete", map[string]interface{}{
		"key":              sessionKey,
		"deleteTranscript": true,
	}, 10*time.Second)
}

// GWAgentFiles 获取 agent 文件列表
func (c *GatewayWSClient) GWAgentFiles(agentID string) (map[string]interface{}, error) {
	return c.Request("agents.files.list", map[string]interface{}{"agentId": agentID}, 10*time.Second)
}

// GWAgentFileGet 获取 agent 单个文件内容
func (c *GatewayWSClient) GWAgentFileGet(agentID, name string) (map[string]interface{}, error) {
	return c.Request("agents.files.get", map[string]interface{}{
		"agentId": agentID,
		"name":    name,
	}, 10*time.Second)
}

// GWAgentFileSet 设置 agent 文件内容
func (c *GatewayWSClient) GWAgentFileSet(agentID, name, content string) (map[string]interface{}, error) {
	return c.Request("agents.files.set", map[string]interface{}{
		"agentId": agentID,
		"name":    name,
		"content": content,
	}, 10*time.Second)
}

// ==================== P0: Session 管理 ====================

// GWSessionsPreview 获取 session 预览（含最后几条消息摘要）
func (c *GatewayWSClient) GWSessionsPreview(sessionKey string) (map[string]interface{}, error) {
	return c.Request("sessions.preview", map[string]interface{}{
		"key": sessionKey,
	}, 10*time.Second)
}

// GWSessionsReset 重置 session（清空历史但保留 session）
func (c *GatewayWSClient) GWSessionsReset(sessionKey string) (map[string]interface{}, error) {
	return c.Request("sessions.reset", map[string]interface{}{
		"key": sessionKey,
	}, 10*time.Second)
}

// GWSessionsCompact 压缩 session 上下文
func (c *GatewayWSClient) GWSessionsCompact(sessionKey string) (map[string]interface{}, error) {
	return c.Request("sessions.compact", map[string]interface{}{
		"key": sessionKey,
	}, 15*time.Second)
}

// ==================== P0: Chat 操作 ====================

// GWChatSend 向 session 发送消息（触发 agent 回复）
func (c *GatewayWSClient) GWChatSend(sessionKey, message string) (map[string]interface{}, error) {
	return c.Request("chat.send", map[string]interface{}{
		"sessionKey": sessionKey,
		"message":    message,
	}, 30*time.Second)
}

// GWChatAbort 中止正在生成的回复
func (c *GatewayWSClient) GWChatAbort(sessionKey string) (map[string]interface{}, error) {
	return c.Request("chat.abort", map[string]interface{}{
		"sessionKey": sessionKey,
	}, 10*time.Second)
}

// ==================== P0: Cron CRUD ====================

// GWCronAdd 创建 cron 任务
func (c *GatewayWSClient) GWCronAdd(job map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("cron.add", job, 10*time.Second)
}

// GWCronUpdate 更新 cron 任务
func (c *GatewayWSClient) GWCronUpdate(jobID string, patch map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{"id": jobID}
	for k, v := range patch {
		params[k] = v
	}
	return c.Request("cron.update", params, 10*time.Second)
}

// GWCronRemove 删除 cron 任务
func (c *GatewayWSClient) GWCronRemove(jobID string) (map[string]interface{}, error) {
	return c.Request("cron.remove", map[string]interface{}{"id": jobID}, 10*time.Second)
}

// ==================== P1: Agent 生命周期 ====================

// GWAgentsCreate 创建 agent
func (c *GatewayWSClient) GWAgentsCreate(params map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("agents.create", params, 15*time.Second)
}

// GWAgentsUpdate 更新 agent
func (c *GatewayWSClient) GWAgentsUpdate(agentID string, patch map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{"agentId": agentID}
	for k, v := range patch {
		params[k] = v
	}
	return c.Request("agents.update", params, 10*time.Second)
}

// GWAgentsDelete 删除 agent
func (c *GatewayWSClient) GWAgentsDelete(agentID string) (map[string]interface{}, error) {
	return c.Request("agents.delete", map[string]interface{}{"agentId": agentID}, 10*time.Second)
}

// ==================== P1: Config 修改 ====================

// GWConfigPatch 部分修改 Gateway 配置（合并）
func (c *GatewayWSClient) GWConfigPatch(patch map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("config.patch", patch, 15*time.Second)
}

// GWConfigApply 完整覆盖 Gateway 配置
func (c *GatewayWSClient) GWConfigApply(raw string) (map[string]interface{}, error) {
	return c.Request("config.apply", map[string]interface{}{"raw": raw}, 15*time.Second)
}

// ==================== P1: Skills 管理 ====================

// GWSkillsBins 获取可用的 skill 仓库
func (c *GatewayWSClient) GWSkillsBins(agentID string) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	if agentID != "" {
		params["agentId"] = agentID
	}
	return c.Request("skills.bins", params, 10*time.Second)
}

// GWSkillsInstall 安装技能
func (c *GatewayWSClient) GWSkillsInstall(params map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("skills.install", params, 30*time.Second)
}

// GWSkillsUpdate 更新技能
func (c *GatewayWSClient) GWSkillsUpdate(params map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("skills.update", params, 30*time.Second)
}

// ==================== P1: 心跳管理 ====================

// GWSetHeartbeats 设置心跳配置
func (c *GatewayWSClient) GWSetHeartbeats(params map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("set-heartbeats", params, 10*time.Second)
}

// GWWake 唤醒 agent
func (c *GatewayWSClient) GWWake(params map[string]interface{}) (map[string]interface{}, error) {
	return c.Request("wake", params, 10*time.Second)
}

// ==================== P1: 设备/节点配对 ====================

// GWDevicePairApprove 批准设备配对
func (c *GatewayWSClient) GWDevicePairApprove(requestID string) (map[string]interface{}, error) {
	return c.Request("device.pair.approve", map[string]interface{}{"requestId": requestID}, 10*time.Second)
}

// GWDevicePairReject 拒绝设备配对
func (c *GatewayWSClient) GWDevicePairReject(requestID string) (map[string]interface{}, error) {
	return c.Request("device.pair.reject", map[string]interface{}{"requestId": requestID}, 10*time.Second)
}

// GWDeviceTokenRotate 轮换设备 token
func (c *GatewayWSClient) GWDeviceTokenRotate(deviceID string) (map[string]interface{}, error) {
	return c.Request("device.token.rotate", map[string]interface{}{"deviceId": deviceID}, 10*time.Second)
}

// GWDeviceTokenRevoke 吊销设备 token
func (c *GatewayWSClient) GWDeviceTokenRevoke(deviceID string) (map[string]interface{}, error) {
	return c.Request("device.token.revoke", map[string]interface{}{"deviceId": deviceID}, 10*time.Second)
}

// GWNodePairList 获取节点配对列表
func (c *GatewayWSClient) GWNodePairList() (map[string]interface{}, error) {
	return c.Request("node.pair.list", map[string]interface{}{}, 10*time.Second)
}

// GWNodePairApprove 批准节点配对
func (c *GatewayWSClient) GWNodePairApprove(requestID string) (map[string]interface{}, error) {
	return c.Request("node.pair.approve", map[string]interface{}{"requestId": requestID}, 10*time.Second)
}

// GWNodePairReject 拒绝节点配对
func (c *GatewayWSClient) GWNodePairReject(requestID string) (map[string]interface{}, error) {
	return c.Request("node.pair.reject", map[string]interface{}{"requestId": requestID}, 10*time.Second)
}

// GWNodeDescribe 获取节点详情
func (c *GatewayWSClient) GWNodeDescribe(nodeID string) (map[string]interface{}, error) {
	return c.Request("node.describe", map[string]interface{}{"node": nodeID}, 10*time.Second)
}

// GWNodeRename 重命名节点
func (c *GatewayWSClient) GWNodeRename(nodeID, name string) (map[string]interface{}, error) {
	return c.Request("node.rename", map[string]interface{}{"node": nodeID, "name": name}, 10*time.Second)
}

// ==================== P1: 系统事件 ====================

// GWSystemEvent 注入系统事件到 session
func (c *GatewayWSClient) GWSystemEvent(sessionKey, text string) (map[string]interface{}, error) {
	return c.Request("system-event", map[string]interface{}{
		"sessionKey": sessionKey,
		"text":       text,
	}, 10*time.Second)
}