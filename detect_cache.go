// detect_cache.go — 检测结果缓存 DetectCache (LRU)（v5.1 智能检测）
// 对相同内容短时间内不重复全链路检测
// 使用 sync.Mutex + map + doubly linked list 实现简单 LRU
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// LRU 双向链表节点
// ============================================================

type cacheEntry struct {
	key       string       // SHA-256 前 16 字节 hex
	result    DetectResult // 缓存的检测结果
	expiresAt time.Time    // 过期时间
	prev      *cacheEntry
	next      *cacheEntry
}

// ============================================================
// DetectCache
// ============================================================

// DetectCache 检测结果缓存（LRU 淘汰策略）
type DetectCache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	items    map[string]*cacheEntry
	head     *cacheEntry // 最近使用
	tail     *cacheEntry // 最久未使用

	// Prometheus 计数器
	hits   int64
	misses int64
}

// NewDetectCache 创建检测结果缓存
// capacity: 最大缓存条目数
// ttl: 缓存过期时间
func NewDetectCache(capacity int, ttl time.Duration) *DetectCache {
	if capacity <= 0 {
		capacity = 1000
	}
	if ttl <= 0 {
		ttl = 300 * time.Second
	}
	return &DetectCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*cacheEntry),
	}
}

// cacheKey 生成缓存键：消息内容的 SHA-256 前 16 字节 hex（32 字符）
func cacheKey(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:16])
}

// Get 查询缓存
// 返回缓存的 DetectResult 和是否命中
func (dc *DetectCache) Get(text string) (DetectResult, bool) {
	key := cacheKey(text)

	dc.mu.Lock()
	defer dc.mu.Unlock()

	entry, ok := dc.items[key]
	if !ok {
		atomic.AddInt64(&dc.misses, 1)
		return DetectResult{}, false
	}

	// 检查是否过期
	if time.Now().After(entry.expiresAt) {
		// 已过期，移除
		dc.removeEntry(entry)
		delete(dc.items, key)
		atomic.AddInt64(&dc.misses, 1)
		return DetectResult{}, false
	}

	// 命中：移到链表头部
	dc.moveToHead(entry)
	atomic.AddInt64(&dc.hits, 1)
	return entry.result, true
}

// Put 存入缓存
// 只缓存 pass 和 warn 结果（block 不缓存，确保规则更新后立即生效）
func (dc *DetectCache) Put(text string, result DetectResult) {
	// block 结果不缓存
	if result.Action == "block" {
		return
	}

	key := cacheKey(text)

	dc.mu.Lock()
	defer dc.mu.Unlock()

	// 如果已存在，更新
	if entry, ok := dc.items[key]; ok {
		entry.result = result
		entry.expiresAt = time.Now().Add(dc.ttl)
		dc.moveToHead(entry)
		return
	}

	// 新建条目
	entry := &cacheEntry{
		key:       key,
		result:    result,
		expiresAt: time.Now().Add(dc.ttl),
	}

	// 添加到链表头部
	dc.addToHead(entry)
	dc.items[key] = entry

	// 超过容量，淘汰尾部
	if len(dc.items) > dc.capacity {
		dc.evict()
	}
}

// Stats 返回缓存统计
func (dc *DetectCache) Stats() (hits, misses int64, size int) {
	dc.mu.Lock()
	size = len(dc.items)
	dc.mu.Unlock()
	hits = atomic.LoadInt64(&dc.hits)
	misses = atomic.LoadInt64(&dc.misses)
	return
}

// Clear 清空缓存
func (dc *DetectCache) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.items = make(map[string]*cacheEntry)
	dc.head = nil
	dc.tail = nil
}

// ============================================================
// 内部链表操作
// ============================================================

// addToHead 将节点添加到链表头部
func (dc *DetectCache) addToHead(entry *cacheEntry) {
	entry.prev = nil
	entry.next = dc.head
	if dc.head != nil {
		dc.head.prev = entry
	}
	dc.head = entry
	if dc.tail == nil {
		dc.tail = entry
	}
}

// removeEntry 从链表中移除节点
func (dc *DetectCache) removeEntry(entry *cacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		dc.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		dc.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

// moveToHead 将节点移到链表头部
func (dc *DetectCache) moveToHead(entry *cacheEntry) {
	if dc.head == entry {
		return
	}
	dc.removeEntry(entry)
	dc.addToHead(entry)
}

// evict 淘汰尾部（最久未使用）
func (dc *DetectCache) evict() {
	if dc.tail == nil {
		return
	}
	entry := dc.tail
	dc.removeEntry(entry)
	delete(dc.items, entry.key)
}
