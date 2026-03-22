// metrics.go — MetricsCollector、RuleHitStats、RateLimiter
// lobster-guard v4.0 代码拆分
package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// Rate Limiter（v3.3 令牌桶限流）
// ============================================================

type RateLimiterConfig struct {
	GlobalRPS      float64  `yaml:"global_rps"`
	GlobalBurst    int      `yaml:"global_burst"`
	PerSenderRPS   float64  `yaml:"per_sender_rps"`
	PerSenderBurst int      `yaml:"per_sender_burst"`
	ExemptSenders  []string `yaml:"exempt_senders"`
}

type TokenBucket struct {
	rate       float64
	burst      int
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
	mu         sync.Mutex
}

func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastRefill: time.Now(),
		lastAccess: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	tb.lastAccess = now

	// 补充 token
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.lastRefill = now

	// 尝试消费
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

type RateLimiterStats struct {
	TotalAllowed int64            `json:"total_allowed"`
	TotalLimited int64            `json:"total_limited"`
	LimitRate    float64          `json:"limit_rate_percent"`
	TopLimited   []SenderLimitInfo `json:"top_limited"`
}

type SenderLimitInfo struct {
	SenderID string `json:"sender_id"`
	Count    int64  `json:"count"`
}

type RateLimiter struct {
	cfg           RateLimiterConfig
	globalBucket  *TokenBucket
	senderBuckets map[string]*TokenBucket
	exemptSet     map[string]bool
	mu            sync.RWMutex

	totalAllowed  int64
	totalLimited  int64
	senderLimited map[string]int64
}

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		cfg:           cfg,
		senderBuckets: make(map[string]*TokenBucket),
		exemptSet:     make(map[string]bool),
		senderLimited: make(map[string]int64),
	}
	for _, s := range cfg.ExemptSenders {
		rl.exemptSet[s] = true
	}
	if cfg.GlobalRPS > 0 {
		burst := cfg.GlobalBurst
		if burst <= 0 {
			burst = int(cfg.GlobalRPS)
		}
		rl.globalBucket = NewTokenBucket(cfg.GlobalRPS, burst)
	}
	return rl
}

func (rl *RateLimiter) Allow(senderID string) (bool, string) {
	// 白名单豁免
	if rl.exemptSet[senderID] {
		atomic.AddInt64(&rl.totalAllowed, 1)
		return true, ""
	}

	// 全局限流检查
	if rl.globalBucket != nil {
		if !rl.globalBucket.Allow() {
			atomic.AddInt64(&rl.totalLimited, 1)
			rl.mu.Lock()
			rl.senderLimited[senderID]++
			rl.mu.Unlock()
			return false, "global rate limit exceeded"
		}
	}

	// 按发送者限流检查
	if rl.cfg.PerSenderRPS > 0 && senderID != "" {
		rl.mu.RLock()
		bucket, exists := rl.senderBuckets[senderID]
		rl.mu.RUnlock()

		if !exists {
			burst := rl.cfg.PerSenderBurst
			if burst <= 0 {
				burst = int(rl.cfg.PerSenderRPS)
			}
			bucket = NewTokenBucket(rl.cfg.PerSenderRPS, burst)
			rl.mu.Lock()
			// double check after acquiring write lock
			if existing, ok := rl.senderBuckets[senderID]; ok {
				bucket = existing
			} else {
				rl.senderBuckets[senderID] = bucket
			}
			rl.mu.Unlock()
		}

		if !bucket.Allow() {
			atomic.AddInt64(&rl.totalLimited, 1)
			rl.mu.Lock()
			rl.senderLimited[senderID]++
			rl.mu.Unlock()
			return false, fmt.Sprintf("per-sender rate limit exceeded (sender=%s)", senderID)
		}
	}

	atomic.AddInt64(&rl.totalAllowed, 1)
	return true, ""
}

func (rl *RateLimiter) Stats() RateLimiterStats {
	allowed := atomic.LoadInt64(&rl.totalAllowed)
	limited := atomic.LoadInt64(&rl.totalLimited)
	total := allowed + limited
	var limitRate float64
	if total > 0 {
		limitRate = float64(limited) / float64(total) * 100.0
	}

	rl.mu.RLock()
	// 构建 top limited
	type kv struct {
		key string
		val int64
	}
	var sorted []kv
	for k, v := range rl.senderLimited {
		sorted = append(sorted, kv{k, v})
	}
	rl.mu.RUnlock()

	sort.Slice(sorted, func(i, j int) bool { return sorted[i].val > sorted[j].val })
	topN := 10
	if len(sorted) < topN {
		topN = len(sorted)
	}
	topLimited := make([]SenderLimitInfo, topN)
	for i := 0; i < topN; i++ {
		topLimited[i] = SenderLimitInfo{SenderID: sorted[i].key, Count: sorted[i].val}
	}

	return RateLimiterStats{
		TotalAllowed: allowed,
		TotalLimited: limited,
		LimitRate:    limitRate,
		TopLimited:   topLimited,
	}
}

func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.senderBuckets = make(map[string]*TokenBucket)
	rl.senderLimited = make(map[string]int64)
	atomic.StoreInt64(&rl.totalAllowed, 0)
	atomic.StoreInt64(&rl.totalLimited, 0)
	if rl.cfg.GlobalRPS > 0 {
		burst := rl.cfg.GlobalBurst
		if burst <= 0 {
			burst = int(rl.cfg.GlobalRPS)
		}
		rl.globalBucket = NewTokenBucket(rl.cfg.GlobalRPS, burst)
	}
}

func (rl *RateLimiter) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for sid, bucket := range rl.senderBuckets {
				bucket.mu.Lock()
				idle := now.Sub(bucket.lastAccess)
				bucket.mu.Unlock()
				if idle > 10*time.Minute {
					delete(rl.senderBuckets, sid)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// ============================================================
// Prometheus Metrics（v3.4 指标导出）
// ============================================================

type LatencyHistogram struct {
	buckets []float64 // bucket boundaries in ms: 1, 5, 10, 25, 50, 100, 250, 500, 1000
	counts  []int64   // count for each bucket
	sum     float64   // total latency sum
	count   int64     // total count
}

func NewLatencyHistogram() *LatencyHistogram {
	buckets := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000}
	return &LatencyHistogram{
		buckets: buckets,
		counts:  make([]int64, len(buckets)),
	}
}

func (h *LatencyHistogram) Observe(valueMs float64) {
	h.sum += valueMs
	h.count++
	for i, b := range h.buckets {
		if valueMs <= b {
			h.counts[i]++
			return
		}
	}
	// value exceeds all bucket boundaries, only counted in +Inf
}

type MetricsCollector struct {
	mu sync.RWMutex

	// 请求计数器 (by direction, action, channel)
	requestsTotal map[string]int64 // key: "direction:action:channel"

	// 请求延迟直方图桶
	latencyBuckets map[string]*LatencyHistogram // key: "direction"

	// Bridge 状态
	bridgeReconnects int64
	bridgeMessages   int64

	// 限流
	rateLimitAllowed int64
	rateLimitDenied  int64

	// v3.10 告警
	alertsTotal int64

	// v4.1 WebSocket 指标
	wsConnectionsTotal  int64            // 累计 WebSocket 连接总数
	wsConnectionsActive int64            // 当前活跃连接数
	wsMessagesTotal     map[string]int64 // key: "direction:action" → count
	wsMessageBytes      map[string]int64 // key: "direction" → bytes

	// 系统
	startTime time.Time
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestsTotal:  make(map[string]int64),
		latencyBuckets: make(map[string]*LatencyHistogram),
		wsMessagesTotal: make(map[string]int64),
		wsMessageBytes:  make(map[string]int64),
		startTime:      time.Now(),
	}
}

func (mc *MetricsCollector) RecordRequest(direction, action, channel string, latencyMs float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	key := direction + ":" + action + ":" + channel
	mc.requestsTotal[key]++
	h, ok := mc.latencyBuckets[direction]
	if !ok {
		h = NewLatencyHistogram()
		mc.latencyBuckets[direction] = h
	}
	h.Observe(latencyMs)
}

func (mc *MetricsCollector) RecordRateLimit(allowed bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if allowed {
		mc.rateLimitAllowed++
	} else {
		mc.rateLimitDenied++
	}
}

func (mc *MetricsCollector) RecordBridgeReconnect() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.bridgeReconnects++
}

func (mc *MetricsCollector) RecordBridgeMessage() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.bridgeMessages++
}

// RecordAlert 记录告警事件（v3.10）
func (mc *MetricsCollector) RecordAlert() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.alertsTotal++
}

// RecordWSConnect 记录 WebSocket 新连接（v4.1）
func (mc *MetricsCollector) RecordWSConnect() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.wsConnectionsTotal++
	mc.wsConnectionsActive++
}

// RecordWSDisconnect 记录 WebSocket 连接断开（v4.1）
func (mc *MetricsCollector) RecordWSDisconnect() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.wsConnectionsActive--
	if mc.wsConnectionsActive < 0 {
		mc.wsConnectionsActive = 0
	}
}

// RecordUpstreamChange 记录上游变更事件（BUG-005 fix: force metrics refresh）
func (mc *MetricsCollector) RecordUpstreamChange() {
	// This is a no-op signal — the actual gauge values are computed dynamically
	// in WritePrometheus via pool.Count(). This method exists as an extension
	// point for future cached gauge implementations.
}

// RecordWSMessage 记录 WebSocket 消息（v4.1）
func (mc *MetricsCollector) RecordWSMessage(direction, action string, bytes int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	key := direction + ":" + action
	mc.wsMessagesTotal[key]++
	if bytes > 0 {
		mc.wsMessageBytes[direction] += bytes
	}
}

// GetWSMetrics 获取 WebSocket 指标（v4.1）
func (mc *MetricsCollector) GetWSMetrics() (total int64, active int64) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.wsConnectionsTotal, mc.wsConnectionsActive
}

func (mc *MetricsCollector) WritePrometheus(w io.Writer, upstreamsTotal, upstreamsHealthy, routesTotal int, bridgeStatus *BridgeStatus, channelName, mode string, ruleHits *RuleHitStats, inboundEngine *RuleEngine, outboundEngine *OutboundRuleEngine, extraWriters ...func(io.Writer)) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// lobster_guard_requests_total
	fmt.Fprintln(w, "# HELP lobster_guard_requests_total Total number of requests processed")
	fmt.Fprintln(w, "# TYPE lobster_guard_requests_total counter")
	// Sort keys for deterministic output
	reqKeys := make([]string, 0, len(mc.requestsTotal))
	for k := range mc.requestsTotal {
		reqKeys = append(reqKeys, k)
	}
	sort.Strings(reqKeys)
	for _, key := range reqKeys {
		parts := strings.SplitN(key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		fmt.Fprintf(w, "lobster_guard_requests_total{direction=%q,action=%q,channel=%q} %d\n",
			parts[0], parts[1], parts[2], mc.requestsTotal[key])
	}

	// lobster_guard_request_duration_ms (histogram)
	fmt.Fprintln(w, "# HELP lobster_guard_request_duration_ms Request processing duration in milliseconds")
	fmt.Fprintln(w, "# TYPE lobster_guard_request_duration_ms histogram")
	histKeys := make([]string, 0, len(mc.latencyBuckets))
	for k := range mc.latencyBuckets {
		histKeys = append(histKeys, k)
	}
	sort.Strings(histKeys)
	for _, dir := range histKeys {
		h := mc.latencyBuckets[dir]
		var cumulative int64
		for i, b := range h.buckets {
			cumulative += h.counts[i]
			fmt.Fprintf(w, "lobster_guard_request_duration_ms_bucket{direction=%q,le=\"%s\"} %d\n",
				dir, formatFloat(b), cumulative)
		}
		fmt.Fprintf(w, "lobster_guard_request_duration_ms_bucket{direction=%q,le=\"+Inf\"} %d\n",
			dir, h.count)
		fmt.Fprintf(w, "lobster_guard_request_duration_ms_sum{direction=%q} %.2f\n", dir, h.sum)
		fmt.Fprintf(w, "lobster_guard_request_duration_ms_count{direction=%q} %d\n", dir, h.count)
	}

	// lobster_guard_upstreams_total
	fmt.Fprintln(w, "# HELP lobster_guard_upstreams_total Total number of registered upstreams")
	fmt.Fprintln(w, "# TYPE lobster_guard_upstreams_total gauge")
	fmt.Fprintf(w, "lobster_guard_upstreams_total %d\n", upstreamsTotal)

	// lobster_guard_upstreams_healthy
	fmt.Fprintln(w, "# HELP lobster_guard_upstreams_healthy Number of healthy upstreams")
	fmt.Fprintln(w, "# TYPE lobster_guard_upstreams_healthy gauge")
	fmt.Fprintf(w, "lobster_guard_upstreams_healthy %d\n", upstreamsHealthy)

	// lobster_guard_routes_total
	fmt.Fprintln(w, "# HELP lobster_guard_routes_total Number of active user-upstream route bindings")
	fmt.Fprintln(w, "# TYPE lobster_guard_routes_total gauge")
	fmt.Fprintf(w, "lobster_guard_routes_total %d\n", routesTotal)

	// lobster_guard_bridge_connected
	bridgeConnected := 0
	if bridgeStatus != nil && bridgeStatus.Connected {
		bridgeConnected = 1
	}
	fmt.Fprintln(w, "# HELP lobster_guard_bridge_connected Whether bridge mode is connected (1=yes, 0=no)")
	fmt.Fprintln(w, "# TYPE lobster_guard_bridge_connected gauge")
	fmt.Fprintf(w, "lobster_guard_bridge_connected %d\n", bridgeConnected)

	// lobster_guard_bridge_reconnects_total
	fmt.Fprintln(w, "# HELP lobster_guard_bridge_reconnects_total Total bridge reconnection attempts")
	fmt.Fprintln(w, "# TYPE lobster_guard_bridge_reconnects_total counter")
	fmt.Fprintf(w, "lobster_guard_bridge_reconnects_total %d\n", mc.bridgeReconnects)

	// lobster_guard_bridge_messages_total
	fmt.Fprintln(w, "# HELP lobster_guard_bridge_messages_total Total messages received via bridge")
	fmt.Fprintln(w, "# TYPE lobster_guard_bridge_messages_total counter")
	fmt.Fprintf(w, "lobster_guard_bridge_messages_total %d\n", mc.bridgeMessages)

	// lobster_guard_rate_limit_total
	fmt.Fprintln(w, "# HELP lobster_guard_rate_limit_total Rate limit decisions")
	fmt.Fprintln(w, "# TYPE lobster_guard_rate_limit_total counter")
	fmt.Fprintf(w, "lobster_guard_rate_limit_total{decision=\"allowed\"} %d\n", mc.rateLimitAllowed)
	fmt.Fprintf(w, "lobster_guard_rate_limit_total{decision=\"denied\"} %d\n", mc.rateLimitDenied)

	// lobster_guard_uptime_seconds
	uptime := time.Since(mc.startTime).Seconds()
	fmt.Fprintln(w, "# HELP lobster_guard_uptime_seconds Time since lobster-guard started")
	fmt.Fprintln(w, "# TYPE lobster_guard_uptime_seconds gauge")
	fmt.Fprintf(w, "lobster_guard_uptime_seconds %.1f\n", uptime)

	// lobster_guard_info
	fmt.Fprintln(w, "# HELP lobster_guard_info Build and configuration info")
	fmt.Fprintln(w, "# TYPE lobster_guard_info gauge")
	fmt.Fprintf(w, "lobster_guard_info{version=%q,channel=%q,mode=%q} 1\n", AppVersion, channelName, mode)

	// v3.6 lobster_guard_rule_hits_total
	if ruleHits != nil {
		fmt.Fprintln(w, "# HELP lobster_guard_rule_hits_total Rule hit count by rule name and action")
		fmt.Fprintln(w, "# TYPE lobster_guard_rule_hits_total counter")

		hits := ruleHits.Get()

		// Build a map of rule name -> action and direction
		type ruleInfo struct {
			action    string
			direction string
		}
		ruleInfoMap := make(map[string]ruleInfo)

		// Inbound rules
		if inboundEngine != nil {
			inboundEngine.mu.RLock()
			for _, cfg := range inboundEngine.ruleConfigs {
				action := cfg.Action
				if action == "" {
					action = "block"
				}
				ruleInfoMap[cfg.Name] = ruleInfo{action: action, direction: "inbound"}
			}
			inboundEngine.mu.RUnlock()
		}

		// Outbound rules
		if outboundEngine != nil {
			outboundEngine.mu.RLock()
			for _, rule := range outboundEngine.rules {
				ruleInfoMap[rule.Name] = ruleInfo{action: rule.Action, direction: "outbound"}
			}
			outboundEngine.mu.RUnlock()
		}

		// Sort keys for deterministic output
		hitKeys := make([]string, 0, len(hits))
		for k := range hits {
			hitKeys = append(hitKeys, k)
		}
		sort.Strings(hitKeys)

		for _, name := range hitKeys {
			count := hits[name]
			info, ok := ruleInfoMap[name]
			if !ok {
				info = ruleInfo{action: "unknown", direction: "unknown"}
			}
			fmt.Fprintf(w, "lobster_guard_rule_hits_total{rule=%q,action=%q,direction=%q} %d\n",
				name, info.action, info.direction, count)
		}
	}

	// v3.10 lobster_guard_alerts_total
	fmt.Fprintln(w, "# HELP lobster_guard_alerts_total Total alert notifications sent")
	fmt.Fprintln(w, "# TYPE lobster_guard_alerts_total counter")
	fmt.Fprintf(w, "lobster_guard_alerts_total{type=\"block\"} %d\n", mc.alertsTotal)

	// v4.1 WebSocket 指标
	fmt.Fprintln(w, "# HELP lobster_guard_ws_connections_total Total WebSocket connections (cumulative)")
	fmt.Fprintln(w, "# TYPE lobster_guard_ws_connections_total counter")
	fmt.Fprintf(w, "lobster_guard_ws_connections_total %d\n", mc.wsConnectionsTotal)

	fmt.Fprintln(w, "# HELP lobster_guard_ws_connections_active Current active WebSocket connections")
	fmt.Fprintln(w, "# TYPE lobster_guard_ws_connections_active gauge")
	fmt.Fprintf(w, "lobster_guard_ws_connections_active %d\n", mc.wsConnectionsActive)

	fmt.Fprintln(w, "# HELP lobster_guard_ws_messages_total WebSocket messages by direction and action")
	fmt.Fprintln(w, "# TYPE lobster_guard_ws_messages_total counter")
	wsMsgKeys := make([]string, 0, len(mc.wsMessagesTotal))
	for k := range mc.wsMessagesTotal {
		wsMsgKeys = append(wsMsgKeys, k)
	}
	sort.Strings(wsMsgKeys)
	for _, key := range wsMsgKeys {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Fprintf(w, "lobster_guard_ws_messages_total{direction=%q,action=%q} %d\n",
			parts[0], parts[1], mc.wsMessagesTotal[key])
	}

	fmt.Fprintln(w, "# HELP lobster_guard_ws_message_bytes_total WebSocket message bytes by direction")
	fmt.Fprintln(w, "# TYPE lobster_guard_ws_message_bytes_total counter")
	wsBytesKeys := make([]string, 0, len(mc.wsMessageBytes))
	for k := range mc.wsMessageBytes {
		wsBytesKeys = append(wsBytesKeys, k)
	}
	sort.Strings(wsBytesKeys)
	for _, dir := range wsBytesKeys {
		fmt.Fprintf(w, "lobster_guard_ws_message_bytes_total{direction=%q} %d\n", dir, mc.wsMessageBytes[dir])
	}

	// v5.1: 额外指标写入器（session risk, llm detect, detect cache 等）
	for _, writer := range extraWriters {
		writer(w)
	}
}

// formatFloat formats a float for Prometheus le labels (integer-like floats without decimal)
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

