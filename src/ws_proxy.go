// ws_proxy.go — WebSocket 消息流代理（v4.1）
// 客户端 → 龙虾卫士 → 上游 Agent 的 WebSocket 代理
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================
// WebSocket 代理管理器
// ============================================================

// WSProxyManager WebSocket 代理管理器
type WSProxyManager struct {
	mu             sync.RWMutex
	connections    map[string]*WSConnection // connID -> connection
	connCounter    int64                    // 连接 ID 计数器
	activeSlots    int64                    // 当前占用的连接槽位（含握手中）
	engine         *RuleEngine
	outboundEngine *OutboundRuleEngine
	logger         *AuditLogger
	metrics        *MetricsCollector
	pool           *UpstreamPool
	routes         *RouteTable
	ruleHits       *RuleHitStats
	cfg            *Config

	// 指标
	totalConnections int64 // 总连接数（累计）
}

// WSConnection 单个 WebSocket 连接状态
type WSConnection struct {
	ID            string    `json:"id"`
	SenderID      string    `json:"sender_id"`
	AppID         string    `json:"app_id"`
	UpstreamID    string    `json:"upstream_id"`
	UpstreamAddr  string    `json:"upstream_addr"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	InboundMsgs   int64     `json:"inbound_msgs"`
	OutboundMsgs  int64     `json:"outbound_msgs"`
	InboundBytes  int64     `json:"inbound_bytes"`
	OutboundBytes int64     `json:"outbound_bytes"`
	Path          string    `json:"path"`

	mu         sync.Mutex
	cancel     context.CancelFunc
	clientConn *websocket.Conn
	upConn     *websocket.Conn
	closed     int32 // atomic
}

// WSConnectionInfo API 返回的连接信息
type WSConnectionInfo struct {
	ID            string `json:"id"`
	SenderID      string `json:"sender_id"`
	AppID         string `json:"app_id"`
	UpstreamID    string `json:"upstream_id"`
	UpstreamAddr  string `json:"upstream_addr"`
	ConnectedAt   string `json:"connected_at"`
	Duration      string `json:"duration"`
	DurationSec   int64  `json:"duration_sec"`
	LastMessageAt string `json:"last_message_at"`
	InboundMsgs   int64  `json:"inbound_msgs"`
	OutboundMsgs  int64  `json:"outbound_msgs"`
	InboundBytes  int64  `json:"inbound_bytes"`
	OutboundBytes int64  `json:"outbound_bytes"`
	Path          string `json:"path"`
}

// NewWSProxyManager 创建 WebSocket 代理管理器
func NewWSProxyManager(cfg *Config, engine *RuleEngine, outboundEngine *OutboundRuleEngine, logger *AuditLogger, metrics *MetricsCollector, pool *UpstreamPool, routes *RouteTable, ruleHits *RuleHitStats) *WSProxyManager {
	// 用配置的白名单覆盖全局 wsUpgrader
	wsUpgrader = newWSUpgrader(cfg.WSAllowedOrigins)
	return &WSProxyManager{
		connections:    make(map[string]*WSConnection),
		engine:         engine,
		outboundEngine: outboundEngine,
		logger:         logger,
		metrics:        metrics,
		pool:           pool,
		routes:         routes,
		ruleHits:       ruleHits,
		cfg:            cfg,
	}
}

// ActiveCount 返回当前活跃连接数
func (wm *WSProxyManager) ActiveCount() int {
	return int(atomic.LoadInt64(&wm.activeSlots))
}

// TotalCount 返回历史总连接数
func (wm *WSProxyManager) TotalCount() int64 {
	return atomic.LoadInt64(&wm.totalConnections)
}

// ListConnections 返回当前活跃连接列表
func (wm *WSProxyManager) ListConnections() []WSConnectionInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	list := make([]WSConnectionInfo, 0, len(wm.connections))
	now := time.Now()
	for _, c := range wm.connections {
		dur := now.Sub(c.ConnectedAt)
		lastMsg := ""
		if !c.LastMessageAt.IsZero() {
			lastMsg = c.LastMessageAt.Format(time.RFC3339)
		}
		list = append(list, WSConnectionInfo{
			ID:            c.ID,
			SenderID:      c.SenderID,
			AppID:         c.AppID,
			UpstreamID:    c.UpstreamID,
			UpstreamAddr:  c.UpstreamAddr,
			ConnectedAt:   c.ConnectedAt.Format(time.RFC3339),
			Duration:      dur.String(),
			DurationSec:   int64(dur.Seconds()),
			LastMessageAt: lastMsg,
			InboundMsgs:   atomic.LoadInt64(&c.InboundMsgs),
			OutboundMsgs:  atomic.LoadInt64(&c.OutboundMsgs),
			InboundBytes:  atomic.LoadInt64(&c.InboundBytes),
			OutboundBytes: atomic.LoadInt64(&c.OutboundBytes),
			Path:          c.Path,
		})
	}
	return list
}

// addConnection 注册连接
func (wm *WSProxyManager) addConnection(conn *WSConnection) {
	wm.mu.Lock()
	wm.connections[conn.ID] = conn
	wm.mu.Unlock()
	atomic.AddInt64(&wm.totalConnections, 1)
}

func (wm *WSProxyManager) tryAcquireSlot(maxConn int) bool {
	for {
		current := atomic.LoadInt64(&wm.activeSlots)
		if int(current) >= maxConn {
			return false
		}
		if atomic.CompareAndSwapInt64(&wm.activeSlots, current, current+1) {
			return true
		}
	}
}

func (wm *WSProxyManager) releaseSlot() {
	for {
		current := atomic.LoadInt64(&wm.activeSlots)
		if current <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&wm.activeSlots, current, current-1) {
			return
		}
	}
}

// removeConnection 移除连接
func (wm *WSProxyManager) removeConnection(connID string) {
	wm.mu.Lock()
	delete(wm.connections, connID)
	wm.mu.Unlock()
}

// getWSMode 获取 WebSocket 模式
func (wm *WSProxyManager) getWSMode() string {
	mode := wm.cfg.WSMode
	if mode == "" {
		mode = "inspect"
	}
	return mode
}

// getWSIdleTimeout 获取空闲超时时间
func (wm *WSProxyManager) getWSIdleTimeout() time.Duration {
	t := wm.cfg.WSIdleTimeout
	if t <= 0 {
		t = 300
	}
	return time.Duration(t) * time.Second
}

// getWSMaxDuration 获取最大连接时长
func (wm *WSProxyManager) getWSMaxDuration() time.Duration {
	t := wm.cfg.WSMaxDuration
	if t <= 0 {
		t = 3600
	}
	return time.Duration(t) * time.Second
}

// getWSMaxConnections 获取最大并发连接数
func (wm *WSProxyManager) getWSMaxConnections() int {
	m := wm.cfg.WSMaxConnections
	if m <= 0 {
		m = 100
	}
	return m
}

// ============================================================
// WebSocket Upgrade 处理
// ============================================================

// newWSUpgrader 创建带 Origin 白名单校验的 WebSocket Upgrader
func newWSUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if len(allowedOrigins) == 0 {
				return true // 未配置白名单时允许全部（向后兼容）
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // 非浏览器请求无 Origin 头
			}
			for _, allowed := range allowedOrigins {
				if allowed == "*" || strings.EqualFold(origin, allowed) {
					return true
				}
			}
			log.Printf("[WebSocket] 拒绝 Origin: %s（不在白名单中）", origin)
			return false
		},
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
}

var wsUpgrader = newWSUpgrader(nil) // 默认允许全部，由 WSProxyManager.Init 覆盖

// IsWebSocketUpgrade 检测请求是否是 WebSocket Upgrade
func IsWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// HandleWebSocket 处理 WebSocket 代理请求
func (wm *WSProxyManager) HandleWebSocket(w http.ResponseWriter, r *http.Request, senderID, appID string) {
	// 并发连接限制检查
	maxConn := wm.getWSMaxConnections()
	if !wm.tryAcquireSlot(maxConn) {
		log.Printf("[WebSocket] 并发连接数已达上限 %d，拒绝新连接 sender=%s", maxConn, senderID)
		http.Error(w, "Service Unavailable: too many WebSocket connections", 503)
		return
	}
	slotHeld := true
	defer func() {
		if slotHeld {
			wm.releaseSlot()
		}
	}()

	// 路由决策：确定上游
	upstreamID, upstreamAddr := wm.resolveUpstream(senderID, appID)
	if upstreamAddr == "" {
		log.Printf("[WebSocket] 无可用上游 sender=%s app=%s", senderID, appID)
		http.Error(w, "Bad Gateway: no upstream available", 502)
		return
	}

	// 构建上游 WebSocket URL
	upstreamWSURL := wm.buildUpstreamWSURL(upstreamAddr, r)

	// 先与上游建立 WebSocket 连接
	upConn, _, err := websocket.DefaultDialer.Dial(upstreamWSURL, nil)
	if err != nil {
		log.Printf("[WebSocket] 连接上游失败 upstream=%s url=%s: %v", upstreamID, upstreamWSURL, err)
		http.Error(w, "Bad Gateway: upstream WebSocket connection failed", 502)
		return
	}

	// Upgrade 客户端连接
	clientConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade 客户端失败: %v", err)
		upConn.Close()
		return
	}

	// 生成连接 ID
	connID := fmt.Sprintf("ws-%d", atomic.AddInt64(&wm.connCounter, 1))

	// 创建连接上下文
	ctx, cancel := context.WithCancel(context.Background())

	conn := &WSConnection{
		ID:           connID,
		SenderID:     senderID,
		AppID:        appID,
		UpstreamID:   upstreamID,
		UpstreamAddr: upstreamAddr,
		ConnectedAt:  time.Now(),
		Path:         r.URL.Path,
		cancel:       cancel,
		clientConn:   clientConn,
		upConn:       upConn,
	}

	wm.addConnection(conn)
	slotHeld = false

	// 记录指标
	if wm.metrics != nil {
		wm.metrics.RecordWSConnect()
	}

	log.Printf("[WebSocket] 新连接 id=%s sender=%s app=%s upstream=%s path=%s mode=%s",
		connID, senderID, appID, upstreamID, r.URL.Path, wm.getWSMode())

	// 审计日志
	wm.logger.Log("inbound", senderID, "ws_connect", "websocket_upgrade", fmt.Sprintf("path=%s upstream=%s", r.URL.Path, upstreamID), "", 0, upstreamID, appID)

	// 启动双向转发
	go wm.proxyLoop(ctx, conn)
}

// resolveUpstream 确定上游地址
func (wm *WSProxyManager) resolveUpstream(senderID, appID string) (string, string) {
	var upstreamID string

	if senderID != "" {
		uid, found := wm.routes.Lookup(senderID, appID)
		if found && wm.pool.IsHealthy(uid) {
			upstreamID = uid
		}
	}

	if upstreamID == "" {
		upstreamID = wm.pool.SelectUpstream(wm.cfg.RouteDefaultPolicy)
	}

	if upstreamID == "" {
		return "", ""
	}

	// 获取上游地址
	wm.pool.mu.RLock()
	defer wm.pool.mu.RUnlock()
	if up, ok := wm.pool.upstreams[upstreamID]; ok {
		return upstreamID, fmt.Sprintf("%s:%d", up.Address, up.Port)
	}
	return "", ""
}

// buildUpstreamWSURL 构建上游 WebSocket URL
func (wm *WSProxyManager) buildUpstreamWSURL(upstreamAddr string, r *http.Request) string {
	u := &url.URL{
		Scheme:   "ws",
		Host:     upstreamAddr,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	return u.String()
}

// ============================================================
// 双向代理循环
// ============================================================

func (wm *WSProxyManager) proxyLoop(ctx context.Context, conn *WSConnection) {
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[WebSocket] proxyLoop panic id=%s: %v", conn.ID, rv)
		}
		wm.closeConnection(conn, websocket.CloseNormalClosure, "connection ended")
	}()

	idleTimeout := wm.getWSIdleTimeout()
	maxDuration := wm.getWSMaxDuration()
	mode := wm.getWSMode()

	// 最大连接时长定时器
	maxDurTimer := time.NewTimer(maxDuration)
	defer maxDurTimer.Stop()

	// 空闲超时定时器
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

	// Ping/Pong 心跳
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// 设置 Pong handler
	lastPong := time.Now()
	var pongMu sync.Mutex
	conn.clientConn.SetPongHandler(func(string) error {
		pongMu.Lock()
		lastPong = time.Now()
		pongMu.Unlock()
		return nil
	})
	conn.upConn.SetPongHandler(func(string) error {
		pongMu.Lock()
		lastPong = time.Now()
		pongMu.Unlock()
		return nil
	})

	// 客户端 → 上游 goroutine
	clientDone := make(chan error, 1)
	go func() {
		defer func() { recover() }()
		clientDone <- wm.forwardMessages(ctx, conn, conn.clientConn, conn.upConn, "inbound", mode, idleTimer, idleTimeout)
	}()

	// 上游 → 客户端 goroutine
	upstreamDone := make(chan error, 1)
	go func() {
		defer func() { recover() }()
		upstreamDone <- wm.forwardMessages(ctx, conn, conn.upConn, conn.clientConn, "outbound", mode, idleTimer, idleTimeout)
	}()

	// 主循环：等待各种退出条件
	for {
		select {
		case <-ctx.Done():
			return

		case err := <-clientDone:
			if err != nil {
				log.Printf("[WebSocket] 客户端断开 id=%s: %v", conn.ID, err)
			}
			return

		case err := <-upstreamDone:
			if err != nil {
				log.Printf("[WebSocket] 上游断开 id=%s: %v", conn.ID, err)
			}
			return

		case <-maxDurTimer.C:
			log.Printf("[WebSocket] 最大连接时长到期 id=%s (%v)", conn.ID, maxDuration)
			wm.closeConnection(conn, websocket.CloseNormalClosure, "max duration exceeded")
			return

		case <-idleTimer.C:
			log.Printf("[WebSocket] 空闲超时 id=%s (%v)", conn.ID, idleTimeout)
			wm.closeConnection(conn, websocket.CloseNormalClosure, "idle timeout")
			return

		case <-pingTicker.C:
			// 检查 Pong 超时
			pongMu.Lock()
			pongAge := time.Since(lastPong)
			pongMu.Unlock()
			if pongAge > 60*time.Second {
				log.Printf("[WebSocket] Pong 超时 id=%s (%v)", conn.ID, pongAge)
				return
			}
			// 发送 Ping
			conn.mu.Lock()
			if conn.clientConn != nil {
				conn.clientConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
			if conn.upConn != nil {
				conn.upConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
			conn.mu.Unlock()
		}
	}
}

// forwardMessages 单向转发消息
func (wm *WSProxyManager) forwardMessages(ctx context.Context, conn *WSConnection, src, dst *websocket.Conn, direction, mode string, idleTimer *time.Timer, idleTimeout time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 设置读取超时（略长于空闲超时+心跳间隔）
		src.SetReadDeadline(time.Now().Add(idleTimeout + 60*time.Second))

		msgType, data, err := src.ReadMessage()
		if err != nil {
			return err
		}

		// 重置空闲定时器
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(idleTimeout)

		// 更新时间戳
		now := time.Now()
		conn.mu.Lock()
		conn.LastMessageAt = now
		conn.mu.Unlock()

		// 记录字节数
		dataLen := int64(len(data))
		if direction == "inbound" {
			atomic.AddInt64(&conn.InboundMsgs, 1)
			atomic.AddInt64(&conn.InboundBytes, dataLen)
		} else {
			atomic.AddInt64(&conn.OutboundMsgs, 1)
			atomic.AddInt64(&conn.OutboundBytes, dataLen)
		}

		// 指标
		if wm.metrics != nil {
			wm.metrics.RecordWSMessage(direction, "pass", dataLen)
		}

		// 透传模式：直接转发所有帧
		if mode == "passthrough" {
			conn.mu.Lock()
			writeErr := dst.WriteMessage(msgType, data)
			conn.mu.Unlock()
			if writeErr != nil {
				return writeErr
			}
			continue
		}

		// inspect 模式：只检测 TextMessage
		if msgType != websocket.TextMessage {
			// 二进制帧直接透传
			conn.mu.Lock()
			writeErr := dst.WriteMessage(msgType, data)
			conn.mu.Unlock()
			if writeErr != nil {
				return writeErr
			}
			continue
		}

		// 文本帧检测
		text := string(data)
		action, writeData := wm.inspectMessage(conn, text, direction)

		// 更新指标中的 action
		if wm.metrics != nil && action != "pass" {
			// 减去之前记录的 pass，加上实际 action
			wm.metrics.RecordWSMessage(direction, action, 0)
		}

		switch action {
		case "block":
			if direction == "inbound" {
				// 入站 block → 关闭连接
				log.Printf("[WebSocket] 入站拦截 id=%s sender=%s", conn.ID, conn.SenderID)
				wm.closeConnection(conn, websocket.ClosePolicyViolation, "message blocked by security policy")
				return fmt.Errorf("inbound message blocked")
			}
			// 出站 block → 替换内容
			log.Printf("[WebSocket] 出站过滤 id=%s sender=%s", conn.ID, conn.SenderID)
			writeData = []byte("[内容已过滤]")

		case "warn":
			// warn → 记审计日志但转发原消息
			log.Printf("[WebSocket] 告警放行 id=%s direction=%s sender=%s", conn.ID, direction, conn.SenderID)
		}

		conn.mu.Lock()
		writeErr := dst.WriteMessage(websocket.TextMessage, writeData)
		conn.mu.Unlock()
		if writeErr != nil {
			return writeErr
		}
	}
}

// inspectMessage 检测单条消息，返回 action 和（可能修改的）数据
func (wm *WSProxyManager) inspectMessage(conn *WSConnection, text, direction string) (string, []byte) {
	if text == "" {
		return "pass", []byte(text)
	}

	var action string
	var reasons []string

	if direction == "inbound" {
		// 使用入站规则引擎
		result := wm.engine.DetectWithAppID(text, conn.AppID)
		action = result.Action
		reasons = result.Reasons

		// 规则命中统计
		if wm.ruleHits != nil && len(result.MatchedRules) > 0 {
			for _, ruleName := range result.MatchedRules {
				wm.ruleHits.Record(ruleName)
			}
		}
	} else {
		// 使用出站规则引擎
		result := wm.outboundEngine.Detect(text)
		action = result.Action
		if result.RuleName != "" {
			reasons = []string{result.RuleName}
		}

		// 规则命中统计
		if wm.ruleHits != nil && result.RuleName != "" {
			wm.ruleHits.Record(result.RuleName)
		}
	}

	if action == "" {
		action = "pass"
	}

	// 审计日志
	preview := text
	if rs := []rune(preview); len(rs) > 200 {
		preview = string(rs[:200]) + "..."
	}
	reason := strings.Join(reasons, ",")
	wm.logger.Log(direction, conn.SenderID, "ws_"+action, reason, preview, "", 0, conn.UpstreamID, conn.AppID)

	return action, []byte(text)
}

// closeConnection 优雅关闭连接
func (wm *WSProxyManager) closeConnection(conn *WSConnection, closeCode int, reason string) {
	if !atomic.CompareAndSwapInt32(&conn.closed, 0, 1) {
		return // 已关闭
	}

	// 取消上下文
	if conn.cancel != nil {
		conn.cancel()
	}

	// 发送 Close 帧
	closeMsg := websocket.FormatCloseMessage(closeCode, reason)
	deadline := time.Now().Add(5 * time.Second)

	conn.mu.Lock()
	if conn.clientConn != nil {
		conn.clientConn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
	}
	if conn.upConn != nil {
		conn.upConn.WriteControl(websocket.CloseMessage, closeMsg, deadline)
	}
	conn.mu.Unlock()

	// 等一下再强制关闭（给 Close 帧传输时间）
	time.AfterFunc(5*time.Second, func() {
		conn.mu.Lock()
		if conn.clientConn != nil {
			conn.clientConn.Close()
		}
		if conn.upConn != nil {
			conn.upConn.Close()
		}
		conn.mu.Unlock()
	})

	// 立即也关闭（读取循环会退出）
	conn.mu.Lock()
	if conn.clientConn != nil {
		conn.clientConn.Close()
	}
	if conn.upConn != nil {
		conn.upConn.Close()
	}
	conn.mu.Unlock()

	// 移除连接
	wm.removeConnection(conn.ID)
	wm.releaseSlot()

	// 指标
	if wm.metrics != nil {
		wm.metrics.RecordWSDisconnect()
	}

	duration := time.Since(conn.ConnectedAt)
	log.Printf("[WebSocket] 连接关闭 id=%s sender=%s duration=%s inbound_msgs=%d outbound_msgs=%d",
		conn.ID, conn.SenderID, duration, atomic.LoadInt64(&conn.InboundMsgs), atomic.LoadInt64(&conn.OutboundMsgs))

	// 审计日志
	wm.logger.Log("inbound", conn.SenderID, "ws_disconnect", reason,
		fmt.Sprintf("duration=%s inbound=%d outbound=%d", duration, conn.InboundMsgs, conn.OutboundMsgs),
		"", 0, conn.UpstreamID, conn.AppID)
}

// ============================================================
// WebSocket 连接状态 API
// ============================================================

// CloseAll 关闭所有活跃 WebSocket 连接（v4.2 优雅关闭用）
func (wm *WSProxyManager) CloseAll() {
	wm.mu.Lock()
	conns := make([]*WSConnection, 0, len(wm.connections))
	for _, conn := range wm.connections {
		conns = append(conns, conn)
	}
	wm.mu.Unlock()

	for _, conn := range conns {
		wm.closeConnection(conn, websocket.CloseGoingAway, "server shutting down")
	}
	log.Printf("[WebSocket] 已关闭 %d 个活跃连接", len(conns))
}

// HandleWSConnectionsAPI 处理 GET /api/v1/ws/connections
func (wm *WSProxyManager) HandleWSConnectionsAPI(w http.ResponseWriter, r *http.Request) {
	conns := wm.ListConnections()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connections":  conns,
		"active":       wm.ActiveCount(),
		"total":        wm.TotalCount(),
		"max":          wm.getWSMaxConnections(),
		"mode":         wm.getWSMode(),
		"idle_timeout": wm.cfg.WSIdleTimeout,
		"max_duration": wm.cfg.WSMaxDuration,
	})
}
