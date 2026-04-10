// confirm_store.go — 人工确认等待队列（v37.0 action: confirm）
package main

import (
	"log"
	"sync"
	"time"
)

// PendingConfirm 待人工确认的入站请求
type PendingConfirm struct {
	SenderID      string
	AppID         string
	MsgText       string    // 原始消息文本（用于日志）
	Body          []byte    // 原始 HTTP request body（用于重放）
	ReqPath       string    // 原始请求路径
	TraceID       string
	UpstreamID    string
	RuleName      string
	TimeoutAction string    // "block" | "pass"，空则用全局配置
	DefaultAction string    // "confirm" | "cancel" | "" (继续等待)
	ExpiresAt     time.Time
	cancelCh      chan struct{} // 关闭此 channel 取消超时 goroutine
}

// ConfirmStore 入站确认等待队列（每个 senderID 最多保留最新一条待确认）
type ConfirmStore struct {
	mu      sync.Mutex
	pending map[string]*PendingConfirm
	proxy   *InboundProxy
}

func NewConfirmStore() *ConfirmStore {
	return &ConfirmStore{pending: make(map[string]*PendingConfirm)}
}

// Add 添加（或替换）待确认记录，并启动超时 goroutine
func (cs *ConfirmStore) Add(pc *PendingConfirm) {
	cancelCh := make(chan struct{})
	pc.cancelCh = cancelCh

	cs.mu.Lock()
	if old, ok := cs.pending[pc.SenderID]; ok {
		close(old.cancelCh) // 取消旧超时
	}
	cs.pending[pc.SenderID] = pc
	cs.mu.Unlock()

	go func() {
		timer := time.NewTimer(time.Until(pc.ExpiresAt))
		defer timer.Stop()
		select {
		case <-timer.C:
			cs.onTimeout(pc)
		case <-cancelCh:
		}
	}()
}

// Pop 取出并删除 senderID 的待确认记录（Y/N 处理时使用）
func (cs *ConfirmStore) Pop(senderID string) *PendingConfirm {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	pc, ok := cs.pending[senderID]
	if !ok {
		return nil
	}
	delete(cs.pending, senderID)
	close(pc.cancelCh) // 取消超时 goroutine
	return pc
}

// Has 检查 senderID 是否有待确认记录
func (cs *ConfirmStore) Has(senderID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	_, ok := cs.pending[senderID]
	return ok
}

// peekDefaultAction 返回待确认记录的 DefaultAction（不移除记录）
func (cs *ConfirmStore) peekDefaultAction(senderID string) string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if pc, ok := cs.pending[senderID]; ok {
		return pc.DefaultAction
	}
	return ""
}

// onTimeout 处理超时（在独立 goroutine 中调用）
func (cs *ConfirmStore) onTimeout(pc *PendingConfirm) {
	cs.mu.Lock()
	cur, ok := cs.pending[pc.SenderID]
	if !ok || cur != pc {
		cs.mu.Unlock()
		return // 已被 Pop（用户已回复）
	}
	delete(cs.pending, pc.SenderID)
	cs.mu.Unlock()

	cfg := cs.proxy.cfg.HumanConfirm
	action := pc.TimeoutAction
	if action == "" {
		action = cfg.TimeoutAction
	}
	if action == "" {
		action = "block"
	}
	log.Printf("[确认] ⏰ 超时 sender=%s trace=%s action=%s rule=%s", pc.SenderID, pc.TraceID, action, pc.RuleName)

	msg := cfg.TimeoutMsg
	if msg == "" {
		msg = "⏰ 确认超时，操作已取消"
	}
	go cs.proxy.sendFixedReplyViaOutbound(pc.SenderID, msg)

	if action == "pass" {
		go cs.proxy.replayConfirmedRequest(pc)
	}
}

