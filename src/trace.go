// trace.go — 请求追踪：trace_id 生成 + 入站↔出站 Trace 关联缓存 + traceResponseWriter
// v5.0 可观测性 | v18 Trace 关联
package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// GenerateTraceID 生成 16 字符的 hex trace_id（8 字节随机数）
func GenerateTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// ============================================================
// v18: Trace 关联缓存 — 入站 senderID→traceID 映射，出站按 recipient 反查
// ============================================================

// TraceCorrelator 维护 sender→最近 trace_id 的映射（O(1) LRU 淘汰）
type TraceCorrelator struct {
	mu      sync.Mutex
	entries map[string]*traceNode
	head    *traceNode // 最新
	tail    *traceNode // 最旧
	maxSize int
	size    int
}

type traceNode struct {
	key     string // senderID
	traceID string
	ts      time.Time
	prev    *traceNode
	next    *traceNode
}

func NewTraceCorrelator(maxSize int) *TraceCorrelator {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &TraceCorrelator{entries: make(map[string]*traceNode, maxSize), maxSize: maxSize}
}

// moveToFront 将节点移到链表头（最新位置）
func (tc *TraceCorrelator) moveToFront(node *traceNode) {
	if node == tc.head {
		return
	}
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node == tc.tail {
		tc.tail = node.prev
	}
	node.prev = nil
	node.next = tc.head
	if tc.head != nil {
		tc.head.prev = node
	}
	tc.head = node
	if tc.tail == nil {
		tc.tail = node
	}
}

// removeTail 删除链表尾（最旧）
func (tc *TraceCorrelator) removeTail() {
	if tc.tail == nil {
		return
	}
	old := tc.tail
	delete(tc.entries, old.key)
	tc.tail = old.prev
	if tc.tail != nil {
		tc.tail.next = nil
	} else {
		tc.head = nil
	}
	tc.size--
}

// Set 入站时记录 sender→trace 映射（O(1)）
func (tc *TraceCorrelator) Set(senderID, traceID string) {
	if senderID == "" || traceID == "" {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if node, ok := tc.entries[senderID]; ok {
		node.traceID = traceID
		node.ts = time.Now()
		tc.moveToFront(node)
		return
	}
	node := &traceNode{key: senderID, traceID: traceID, ts: time.Now()}
	tc.entries[senderID] = node
	node.next = tc.head
	if tc.head != nil {
		tc.head.prev = node
	}
	tc.head = node
	if tc.tail == nil {
		tc.tail = node
	}
	tc.size++
	for tc.size > tc.maxSize {
		tc.removeTail()
	}
}

// Get 出站时按 recipient 查找入站 trace_id（5分钟内有效）
func (tc *TraceCorrelator) Get(recipientID string) string {
	if recipientID == "" {
		return ""
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	node, ok := tc.entries[recipientID]
	if !ok {
		return ""
	}
	if time.Since(node.ts) > 5*time.Minute {
		return ""
	}
	tc.moveToFront(node)
	return node.traceID
}

// ============================================================
// traceResponseWriter — 在响应中自动添加 X-Trace-ID（v5.0）
// ============================================================

type traceResponseWriter struct {
	http.ResponseWriter
	traceID       string
	headerWritten bool
}

func (tw *traceResponseWriter) WriteHeader(statusCode int) {
	if !tw.headerWritten {
		tw.ResponseWriter.Header().Set("X-Trace-ID", tw.traceID)
		tw.headerWritten = true
	}
	tw.ResponseWriter.WriteHeader(statusCode)
}

func (tw *traceResponseWriter) Write(b []byte) (int, error) {
	if !tw.headerWritten {
		tw.ResponseWriter.Header().Set("X-Trace-ID", tw.traceID)
		tw.headerWritten = true
	}
	return tw.ResponseWriter.Write(b)
}
