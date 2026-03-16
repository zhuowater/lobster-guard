// bridge.go — Bridge Mode、BridgeConnector 接口、飞书/钉钉 Bridge 实现
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================
// FeishuBridge — 飞书长连接桥接
// ============================================================

type FeishuBridge struct {
	appID     string
	appSecret string
	conn      *websocket.Conn
	status    BridgeStatus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	plugin    *FeishuPlugin
}

func (fb *FeishuBridge) Name() string { return "feishu-bridge" }

func (fb *FeishuBridge) Status() BridgeStatus {
	fb.mu.RLock()
	defer fb.mu.RUnlock()
	return fb.status
}

func (fb *FeishuBridge) getTenantAccessToken() (string, error) {
	body, _ := json.Marshal(map[string]string{
		"app_id":     fb.appID,
		"app_secret": fb.appSecret,
	})
	resp, err := http.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("获取 tenant_access_token 失败: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 token 响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("获取 token 失败: code=%d msg=%s", result.Code, result.Msg)
	}
	return result.TenantAccessToken, nil
}

func (fb *FeishuBridge) connect(token string) (*websocket.Conn, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial("wss://open.feishu.cn/callback/ws/endpoint", header)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	return conn, nil
}

func (fb *FeishuBridge) Start(ctx context.Context, onMessage func(msg InboundMessage)) error {
	fb.ctx, fb.cancel = context.WithCancel(ctx)
	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-fb.ctx.Done():
			return fb.ctx.Err()
		default:
		}

		// 获取 token
		token, err := fb.getTenantAccessToken()
		if err != nil {
			log.Printf("[飞书桥接] 获取 token 失败: %v, %v 后重试", err, backoff)
			fb.mu.Lock()
			fb.status.LastError = err.Error()
			fb.status.Connected = false
			fb.mu.Unlock()
			select {
			case <-fb.ctx.Done():
				return fb.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// 建立连接
		conn, err := fb.connect(token)
		if err != nil {
			log.Printf("[飞书桥接] 连接失败: %v, %v 后重试", err, backoff)
			fb.mu.Lock()
			fb.status.LastError = err.Error()
			fb.status.Connected = false
			fb.mu.Unlock()
			select {
			case <-fb.ctx.Done():
				return fb.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		fb.mu.Lock()
		fb.conn = conn
		fb.status.Connected = true
		fb.status.ConnectedAt = time.Now()
		fb.status.LastError = ""
		fb.mu.Unlock()
		backoff = time.Second // 重置退避
		log.Printf("[飞书桥接] WebSocket 连接成功")

		// 设置 ping/pong
		conn.SetPongHandler(func(appData string) error {
			return nil
		})
		conn.SetPingHandler(func(appData string) error {
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		})

		// Token 刷新定时器 (每 100 分钟刷新一次，token 有效期 2 小时)
		tokenRefreshTicker := time.NewTicker(100 * time.Minute)

		// 读取消息循环
		connClosed := make(chan struct{})
		go func() {
			defer close(connClosed)
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						log.Printf("[飞书桥接] 读取消息错误: %v", err)
						fb.mu.Lock()
						fb.status.LastError = err.Error()
						fb.mu.Unlock()
					}
					return
				}

				fb.mu.Lock()
				fb.status.LastMessage = time.Now()
				fb.status.MessageCount++
				fb.mu.Unlock()

				// 解析飞书事件
				var event map[string]interface{}
				if json.Unmarshal(message, &event) != nil {
					continue
				}

				// 发送确认
				if header, ok := event["header"].(map[string]interface{}); ok {
					if eventID, ok := header["event_id"].(string); ok && eventID != "" {
						ack, _ := json.Marshal(map[string]interface{}{
							"headers": map[string]string{"X-Request-Id": eventID},
						})
						conn.WriteMessage(websocket.TextMessage, ack)
					}
				}

				// 解析为 InboundMessage（复用 FeishuPlugin 的解析逻辑）
				msg, err := fb.plugin.ParseInbound(message)
				if err != nil {
					log.Printf("[飞书桥接] 解析消息失败: %v", err)
					continue
				}

				// URL Verification 在桥接模式不需要处理
				if msg.EventType == "url_verification" {
					continue
				}

				onMessage(msg)
			}
		}()

		// 等待连接断开或 context 取消
		select {
		case <-fb.ctx.Done():
			tokenRefreshTicker.Stop()
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			conn.Close()
			return fb.ctx.Err()
		case <-connClosed:
			tokenRefreshTicker.Stop()
			fb.mu.Lock()
			fb.status.Connected = false
			fb.status.Reconnects++
			fb.mu.Unlock()
			log.Printf("[飞书桥接] 连接断开，%v 后重连 (第 %d 次)", backoff, fb.status.Reconnects)
			select {
			case <-fb.ctx.Done():
				return fb.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		case <-tokenRefreshTicker.C:
			// Token 即将过期，关闭当前连接以触发重连（使用新 token）
			tokenRefreshTicker.Stop()
			log.Printf("[飞书桥接] Token 刷新，重建连接")
			conn.Close()
			<-connClosed
			fb.mu.Lock()
			fb.status.Connected = false
			fb.status.Reconnects++
			fb.mu.Unlock()
		}
	}
}

func (fb *FeishuBridge) Stop() error {
	if fb.cancel != nil {
		fb.cancel()
	}
	fb.mu.Lock()
	defer fb.mu.Unlock()
	if fb.conn != nil {
		fb.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		fb.conn.Close()
		fb.conn = nil
	}
	fb.status.Connected = false
	return nil
}


// ============================================================
// DingtalkBridge — 钉钉长连接桥接
// ============================================================

type DingtalkBridge struct {
	clientID     string
	clientSecret string
	conn         *websocket.Conn
	status       BridgeStatus
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	plugin       *DingtalkPlugin
}

func (db *DingtalkBridge) Name() string { return "dingtalk-bridge" }

func (db *DingtalkBridge) Status() BridgeStatus {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.status
}

func (db *DingtalkBridge) getConnectionTicket() (endpoint, ticket string, err error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"clientId":     db.clientID,
		"clientSecret": db.clientSecret,
	})
	req, err := http.NewRequest("POST", "https://api.dingtalk.com/v1.0/gateway/connections/open",
		bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("获取连接票据失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("解析票据响应失败: %w", err)
	}
	if result.Endpoint == "" || result.Ticket == "" {
		return "", "", fmt.Errorf("票据响应为空")
	}
	return result.Endpoint, result.Ticket, nil
}

func (db *DingtalkBridge) connect(endpoint, ticket string) (*websocket.Conn, error) {
	wsURL := endpoint + "?ticket=" + url.QueryEscape(ticket)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	return conn, nil
}

func (db *DingtalkBridge) Start(ctx context.Context, onMessage func(msg InboundMessage)) error {
	db.ctx, db.cancel = context.WithCancel(ctx)
	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-db.ctx.Done():
			return db.ctx.Err()
		default:
		}

		// 获取票据
		endpoint, ticket, err := db.getConnectionTicket()
		if err != nil {
			log.Printf("[钉钉桥接] 获取票据失败: %v, %v 后重试", err, backoff)
			db.mu.Lock()
			db.status.LastError = err.Error()
			db.status.Connected = false
			db.mu.Unlock()
			select {
			case <-db.ctx.Done():
				return db.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// 建立连接
		conn, err := db.connect(endpoint, ticket)
		if err != nil {
			log.Printf("[钉钉桥接] 连接失败: %v, %v 后重试", err, backoff)
			db.mu.Lock()
			db.status.LastError = err.Error()
			db.status.Connected = false
			db.mu.Unlock()
			select {
			case <-db.ctx.Done():
				return db.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		db.mu.Lock()
		db.conn = conn
		db.status.Connected = true
		db.status.ConnectedAt = time.Now()
		db.status.LastError = ""
		db.mu.Unlock()
		backoff = time.Second // 重置退避
		log.Printf("[钉钉桥接] WebSocket 连接成功")

		// 设置 ping/pong
		conn.SetPingHandler(func(appData string) error {
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		})

		// 读取消息循环
		connClosed := make(chan struct{})
		go func() {
			defer close(connClosed)
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						log.Printf("[钉钉桥接] 读取消息错误: %v", err)
						db.mu.Lock()
						db.status.LastError = err.Error()
						db.mu.Unlock()
					}
					return
				}

				// 解析钉钉 Stream 消息
				var streamMsg struct {
					SpecVersion string                 `json:"specVersion"`
					Type        string                 `json:"type"`
					Headers     map[string]string      `json:"headers"`
					Data        string                 `json:"data"`
				}
				if json.Unmarshal(message, &streamMsg) != nil {
					continue
				}

				// 系统心跳
				if streamMsg.Type == "SYSTEM" {
					if topic, ok := streamMsg.Headers["topic"]; ok && topic == "/ping" {
						pong, _ := json.Marshal(map[string]interface{}{
							"code":    200,
							"headers": streamMsg.Headers,
							"message": "pong",
							"data":    streamMsg.Data,
						})
						conn.WriteMessage(websocket.TextMessage, pong)
						continue
					}
				}

				// 回调消息
				if streamMsg.Type == "CALLBACK" {
					db.mu.Lock()
					db.status.LastMessage = time.Now()
					db.status.MessageCount++
					db.mu.Unlock()

					// 发送确认
					ack, _ := json.Marshal(map[string]interface{}{
						"response": map[string]interface{}{
							"statusCode": 200,
							"headers":    map[string]string{},
							"body":       "",
						},
					})
					conn.WriteMessage(websocket.TextMessage, ack)

					// 解析 data JSON
					var dataBody []byte
					if streamMsg.Data != "" {
						dataBody = []byte(streamMsg.Data)
					} else {
						continue
					}

					// 使用 DingtalkPlugin 解析消息
					msg, err := db.plugin.ParseInbound(dataBody)
					if err != nil {
						log.Printf("[钉钉桥接] 解析消息失败: %v", err)
						continue
					}

					onMessage(msg)
				}
			}
		}()

		// 等待连接断开或 context 取消
		select {
		case <-db.ctx.Done():
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			conn.Close()
			return db.ctx.Err()
		case <-connClosed:
			db.mu.Lock()
			db.status.Connected = false
			db.status.Reconnects++
			db.mu.Unlock()
			log.Printf("[钉钉桥接] 连接断开，%v 后重连 (第 %d 次)", backoff, db.status.Reconnects)
			select {
			case <-db.ctx.Done():
				return db.ctx.Err()
			case <-time.After(backoff):
			}
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (db *DingtalkBridge) Stop() error {
	if db.cancel != nil {
		db.cancel()
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.conn != nil {
		db.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		db.conn.Close()
		db.conn = nil
	}
	db.status.Connected = false
	return nil
}

