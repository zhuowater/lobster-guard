// realtime.go — 实时监控指标（v5.0 环形缓冲区）
// 内存中维护最近 60 秒逐秒统计，用于 Dashboard 实时监控大屏
package main

import (
	"sync"
	"time"
)

const realtimeSlots = 60

// RealtimeSlot 一秒的统计数据
type RealtimeSlot struct {
	Timestamp  int64 `json:"ts"`           // Unix 秒
	InboundN   int64 `json:"inbound"`      // 入站请求数
	OutboundN  int64 `json:"outbound"`     // 出站请求数
	BlockN     int64 `json:"block"`        // 拦截数
	WarnN      int64 `json:"warn"`         // 告警数
	LatencySum int64 `json:"latency_sum"`  // 延迟累计（微秒）
	LatencyN   int64 `json:"latency_n"`    // 延迟计数
}

// RealtimeEvent 攻击实时事件
type RealtimeEvent struct {
	Time      string `json:"time"`
	Direction string `json:"direction"`
	SenderID  string `json:"sender_id"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	TraceID   string `json:"trace_id"`
}

// RealtimeMetrics 实时监控指标收集器（环形缓冲区）
type RealtimeMetrics struct {
	mu     sync.Mutex
	slots  [realtimeSlots]RealtimeSlot
	events []RealtimeEvent // 最近 20 条 block/warn 事件
}

// NewRealtimeMetrics 创建实时指标收集器
func NewRealtimeMetrics() *RealtimeMetrics {
	return &RealtimeMetrics{
		events: make([]RealtimeEvent, 0, 20),
	}
}

// getSlot 获取当前秒对应的槽位（若时间戳不同则重置）
func (rm *RealtimeMetrics) getSlot() *RealtimeSlot {
	now := time.Now().Unix()
	idx := int(now % realtimeSlots)
	slot := &rm.slots[idx]
	if slot.Timestamp != now {
		// 新的一秒，重置槽位
		*slot = RealtimeSlot{Timestamp: now}
	}
	return slot
}

// RecordInbound 记录入站请求
func (rm *RealtimeMetrics) RecordInbound(action string, latencyUs int64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	slot := rm.getSlot()
	slot.InboundN++
	slot.LatencySum += latencyUs
	slot.LatencyN++
	switch action {
	case "block":
		slot.BlockN++
	case "warn":
		slot.WarnN++
	}
}

// RecordOutbound 记录出站请求
func (rm *RealtimeMetrics) RecordOutbound(action string, latencyUs int64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	slot := rm.getSlot()
	slot.OutboundN++
	slot.LatencySum += latencyUs
	slot.LatencyN++
	switch action {
	case "block":
		slot.BlockN++
	case "warn":
		slot.WarnN++
	}
}

// RecordEvent 记录 block/warn 事件
func (rm *RealtimeMetrics) RecordEvent(direction, senderID, action, reason, traceID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	evt := RealtimeEvent{
		Time:      time.Now().UTC().Format(time.RFC3339),
		Direction: direction,
		SenderID:  senderID,
		Action:    action,
		Reason:    reason,
		TraceID:   traceID,
	}
	rm.events = append(rm.events, evt)
	if len(rm.events) > 20 {
		rm.events = rm.events[len(rm.events)-20:]
	}
}

// Snapshot 返回最近 60 秒的统计快照
func (rm *RealtimeMetrics) Snapshot() RealtimeSnapshot {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now().Unix()
	slots := make([]RealtimeSlot, 0, realtimeSlots)
	var totalReq, totalBlock, totalLatencySum, totalLatencyN int64

	for i := realtimeSlots - 1; i >= 0; i-- {
		ts := now - int64(i)
		idx := int(ts % realtimeSlots)
		slot := rm.slots[idx]
		if slot.Timestamp != ts {
			// 该秒无数据
			slot = RealtimeSlot{Timestamp: ts}
		}
		slots = append(slots, slot)
		totalReq += slot.InboundN + slot.OutboundN
		totalBlock += slot.BlockN
		totalLatencySum += slot.LatencySum
		totalLatencyN += slot.LatencyN
	}

	var avgLatencyMs float64
	if totalLatencyN > 0 {
		avgLatencyMs = float64(totalLatencySum) / float64(totalLatencyN) / 1000.0
	}

	var blockRate float64
	if totalReq > 0 {
		blockRate = float64(totalBlock) / float64(totalReq) * 100.0
	}

	// 复制事件列表
	events := make([]RealtimeEvent, len(rm.events))
	copy(events, rm.events)

	return RealtimeSnapshot{
		Slots:        slots,
		Events:       events,
		TotalReq:     totalReq,
		TotalBlock:   totalBlock,
		BlockRate:    blockRate,
		AvgLatencyMs: avgLatencyMs,
	}
}

// RealtimeSnapshot 实时指标快照
type RealtimeSnapshot struct {
	Slots        []RealtimeSlot  `json:"slots"`
	Events       []RealtimeEvent `json:"events"`
	TotalReq     int64           `json:"total_requests"`
	TotalBlock   int64           `json:"total_blocks"`
	BlockRate    float64         `json:"block_rate"`
	AvgLatencyMs float64        `json:"avg_latency_ms"`
}
