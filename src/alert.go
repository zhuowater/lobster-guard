// alert.go — AlertNotifier、告警推送
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// v3.10 告警通知器
// ============================================================

// AlertNotifier 异步发送 block 事件告警到 webhook
type AlertNotifier struct {
	webhookURL  string
	format      string // "generic" 或 "lanxin"
	minInterval time.Duration
	lastAlert   time.Time
	mu          sync.Mutex
	metrics     *MetricsCollector
	alertsTotal int64
	httpClient  *http.Client
}

// NewAlertNotifier 创建告警通知器
func NewAlertNotifier(webhookURL, format string, minIntervalSec int, metrics *MetricsCollector) *AlertNotifier {
	if minIntervalSec <= 0 { minIntervalSec = 60 }
	if format == "" { format = "generic" }
	return &AlertNotifier{
		webhookURL:  webhookURL,
		format:      format,
		minInterval: time.Duration(minIntervalSec) * time.Second,
		metrics:     metrics,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// AlertEvent block 事件数据
type AlertEvent struct {
	Event          string `json:"event"`
	Direction      string `json:"direction"`
	SenderID       string `json:"sender_id"`
	Rule           string `json:"rule"`
	ContentPreview string `json:"content_preview"`
	Timestamp      string `json:"timestamp"`
	AppID          string `json:"app_id"`
}

// Notify 异步发送 block 告警
func (an *AlertNotifier) Notify(direction, senderID, rule, content, appID string) {
	an.mu.Lock()
	now := time.Now()
	if now.Sub(an.lastAlert) < an.minInterval {
		an.mu.Unlock()
		return
	}
	an.lastAlert = now
	an.mu.Unlock()

	atomic.AddInt64(&an.alertsTotal, 1)

	// 记录 metrics
	if an.metrics != nil {
		an.metrics.RecordAlert()
	}

	// 内容预览：前 50 字符
	preview := content
	if rs := []rune(preview); len(rs) > 50 { preview = string(rs[:50]) + "..." }

	event := AlertEvent{
		Event:          "block",
		Direction:      direction,
		SenderID:       senderID,
		Rule:           rule,
		ContentPreview: preview,
		Timestamp:      now.UTC().Format(time.RFC3339),
		AppID:          appID,
	}

	// 异步发送
	go func() {
		defer func() { recover() }()

		var body []byte
		var err error

		if an.format == "lanxin" {
			// 蓝信机器人 webhook 格式
			text := fmt.Sprintf("🚨 [龙虾卫士告警]\n方向: %s\n发送者: %s\n规则: %s\n内容: %s\n时间: %s\nBot: %s",
				event.Direction, event.SenderID, event.Rule, event.ContentPreview, event.Timestamp, event.AppID)
			lanxinMsg := map[string]interface{}{
				"msgType": "text",
				"msgData": map[string]string{"text": text},
			}
			body, err = json.Marshal(lanxinMsg)
		} else {
			// 通用 webhook 格式
			body, err = json.Marshal(event)
		}
		if err != nil {
			log.Printf("[告警] 序列化告警消息失败: %v", err)
			return
		}

		resp, err := an.httpClient.Post(an.webhookURL, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("[告警] 发送告警失败: %v", err)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)

		if resp.StatusCode >= 300 {
			log.Printf("[告警] 告警 webhook 返回非成功状态码: %d", resp.StatusCode)
		} else {
			log.Printf("[告警] 已发送 block 告警: direction=%s sender=%s rule=%s", event.Direction, event.SenderID, event.Rule)
		}
	}()
}

// TotalAlerts 返回告警总数
func (an *AlertNotifier) TotalAlerts() int64 {
	return atomic.LoadInt64(&an.alertsTotal)
}

// ============================================================
// 辅助函数
// ============================================================

func truncate(s string, maxRunes int) string {
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	return string(rs[:maxRunes]) + "..."
}

