// proxy.go — InboundProxy、OutboundProxy
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// ============================================================
// v18: Trace 关联缓存 — 入站 senderID→traceID 映射，出站按 recipient 反查
// ============================================================

// TraceCorrelator 维护 sender→最近 trace_id 的映射（LRU 淘汰）
type TraceCorrelator struct {
	mu      sync.RWMutex
	entries map[string]traceEntry
	maxSize int
}

type traceEntry struct {
	traceID string
	ts      time.Time
}

func NewTraceCorrelator(maxSize int) *TraceCorrelator {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &TraceCorrelator{entries: make(map[string]traceEntry), maxSize: maxSize}
}

// Set 入站时记录 sender→trace 映射
func (tc *TraceCorrelator) Set(senderID, traceID string) {
	if senderID == "" || traceID == "" {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.entries[senderID] = traceEntry{traceID: traceID, ts: time.Now()}
	// 简单淘汰：超过 maxSize 时删最老的
	if len(tc.entries) > tc.maxSize {
		var oldest string
		var oldestTs time.Time
		for k, v := range tc.entries {
			if oldest == "" || v.ts.Before(oldestTs) {
				oldest = k
				oldestTs = v.ts
			}
		}
		if oldest != "" {
			delete(tc.entries, oldest)
		}
	}
}

// Get 出站时按 recipient 查找入站 trace_id（5分钟内有效）
func (tc *TraceCorrelator) Get(recipientID string) string {
	if recipientID == "" {
		return ""
	}
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	e, ok := tc.entries[recipientID]
	if !ok {
		return ""
	}
	// 5 分钟窗口
	if time.Since(e.ts) > 5*time.Minute {
		return ""
	}
	return e.traceID
}

// ============================================================
// 入站代理 v2.0
// ============================================================

type InboundProxy struct {
	channel    ChannelPlugin
	engine     *RuleEngine
	logger     *AuditLogger
	pool       *UpstreamPool
	routes     *RouteTable
	enabled    bool
	timeout    time.Duration
	whitelist  map[string]bool
	policy     string
	mode       string          // "webhook" | "bridge"
	bridge     BridgeConnector // bridge 模式下非 nil
	cfg        *Config
	limiter    *RateLimiter    // v3.3 限流器，nil 表示不限流
	metrics    *MetricsCollector // v3.4 指标采集器
	ruleHits   *RuleHitStats   // v3.6 规则命中统计
	userCache  *UserInfoCache  // v3.9 用户信息缓存
	policyEng  *RoutePolicyEngine // v3.9 路由策略引擎
	alertNotifier *AlertNotifier // v3.10 告警通知器
	wsProxy    *WSProxyManager // v4.1 WebSocket 代理管理器
	realtime   *RealtimeMetrics // v5.0 实时监控
	slog       *Logger          // v5.0 结构化日志
	traceCorrelator    *TraceCorrelator    // v18 出站 trace 关联
	sessionCorrelator  *SessionCorrelator  // v17.3 IM↔LLM 会话关联
	// v5.1 智能检测
	sessionDetector *SessionDetector
	llmDetector     *LLMDetector
	detectCache     *DetectCache
	pipeline        *DetectPipeline
	// v15.0 蜜罐引擎
	honeypot *HoneypotEngine
	// v18.0 执行信封
	envelopeMgr *EnvelopeManager
	// v18.1 事件总线
	eventBus *EventBus
	// v18.3 自适应决策
	adaptiveEngine *AdaptiveDecisionEngine
	// v19.1 语义检测引擎
	semanticDetector *SemanticDetector
	// v19.2 蜜罐深度交互引擎
	honeypotDeep *HoneypotDeepEngine
	// v20.1 污染追踪引擎
	taintTracker *TaintTracker
}

func NewInboundProxy(cfg *Config, channel ChannelPlugin, engine *RuleEngine, logger *AuditLogger, pool *UpstreamPool, routes *RouteTable, metrics *MetricsCollector, ruleHits *RuleHitStats, userCache *UserInfoCache, policyEng *RoutePolicyEngine, honeypot *HoneypotEngine) *InboundProxy {
	wl := make(map[string]bool)
	for _, id := range cfg.Whitelist { wl[id] = true }
	mode := cfg.Mode
	if mode == "" { mode = "webhook" }
	var limiter *RateLimiter
	if cfg.RateLimit.GlobalRPS > 0 || cfg.RateLimit.PerSenderRPS > 0 {
		limiter = NewRateLimiter(cfg.RateLimit)
	}
	return &InboundProxy{
		channel: channel, engine: engine, logger: logger, pool: pool, routes: routes,
		enabled: cfg.InboundDetectEnabled, timeout: time.Duration(cfg.DetectTimeoutMs) * time.Millisecond,
		whitelist: wl, policy: cfg.RouteDefaultPolicy, mode: mode, cfg: cfg, limiter: limiter,
		metrics: metrics, ruleHits: ruleHits, userCache: userCache, policyEng: policyEng,
		honeypot: honeypot,
	}
}

func (ip *InboundProxy) startBridge(ctx context.Context) error {
	bridge, err := ip.channel.NewBridgeConnector(ip.cfg)
	if err != nil {
		return err
	}
	ip.bridge = bridge

	go bridge.Start(ctx, func(msg InboundMessage) {
		start := time.Now()
		senderID := msg.SenderID
		msgText := msg.Text
		appID := msg.AppID
		rh := fmt.Sprintf("%x", sha256.Sum256(msg.Raw))
		bridgeTraceID := GenerateTraceID()
		// v18: 记录 sender→trace 映射，供出站关联
		if ip.traceCorrelator != nil {
			ip.traceCorrelator.Set(senderID, bridgeTraceID)
		}

		// 路由决策
		var upstreamID string
		if senderID != "" {
			uid, found := ip.routes.Lookup(senderID, appID)
			if found {
				if ip.pool.IsHealthy(uid) {
					upstreamID = uid
				} else {
					newUID := ip.pool.SelectUpstream(ip.policy)
					if newUID != "" && newUID != uid {
						ip.pool.IncrUserCount(uid, -1)
						ip.pool.IncrUserCount(newUID, 1)
						ip.routes.Migrate(senderID, appID, uid, newUID)
						upstreamID = newUID
						log.Printf("[桥接路由] 故障转移 sender=%s app=%s: %s -> %s", senderID, appID, uid, newUID)
					} else {
						upstreamID = uid
					}
				}
			} else {
				// v3.9: 先尝试策略匹配
				policyMatched := false
				if ip.policyEng != nil && ip.userCache != nil {
					if info := ip.userCache.GetCached(senderID); info != nil {
						if pUID, ok := ip.policyEng.Match(info, appID); ok && pUID != "" {
							if ip.pool.IsHealthy(pUID) {
								upstreamID = pUID
								ip.routes.Bind(senderID, appID, upstreamID)
								ip.pool.IncrUserCount(upstreamID, 1)
								policyMatched = true
								log.Printf("[桥接路由] 策略匹配绑定 sender=%s app=%s -> %s (email=%s dept=%s)", senderID, appID, upstreamID, info.Email, info.Department)
							}
						}
					}
				}
				if !policyMatched {
					upstreamID = ip.pool.SelectUpstream(ip.policy)
					if upstreamID != "" {
						ip.routes.Bind(senderID, appID, upstreamID)
						ip.pool.IncrUserCount(upstreamID, 1)
						log.Printf("[桥接路由] 新用户绑定 sender=%s app=%s -> %s", senderID, appID, upstreamID)
					}
				}
			}
		}

		// v3.9: 异步获取用户信息
		if senderID != "" && ip.userCache != nil {
			go func(sid, aID string) {
				defer func() { recover() }()
				info, err := ip.userCache.GetOrFetch(sid)
				if err == nil && info != nil {
					ip.routes.UpdateUserInfo(sid, info.Name, info.Email, info.Department)
					// 如果还没通过策略匹配路由，尝试策略匹配
					if ip.policyEng != nil {
						if _, found := ip.routes.Lookup(sid, aID); !found {
							if pUID, ok := ip.policyEng.Match(info, aID); ok && pUID != "" && ip.pool.IsHealthy(pUID) {
								ip.routes.Bind(sid, aID, pUID)
								ip.pool.IncrUserCount(pUID, 1)
								log.Printf("[桥接路由] 异步策略匹配绑定 sender=%s -> %s", sid, pUID)
							}
						}
					}
				}
			}(senderID, appID)
		}

		// 限流检查（安检之前）
		if ip.limiter != nil {
			allowed, reason := ip.limiter.Allow(msg.SenderID)
			if !allowed {
				if ip.metrics != nil {
					ip.metrics.RecordRateLimit(false)
					ip.metrics.RecordRequest("inbound", "rate_limited", ip.channel.Name(), 0)
				}
				ip.logger.Log("inbound", msg.SenderID, "rate_limited", reason, truncate(msg.Text, 200), rh, 0, "", msg.AppID)
				return // 丢弃消息
			}
			if ip.metrics != nil {
				ip.metrics.RecordRateLimit(true)
			}
		}

		// 白名单检查
		skipDetect := !ip.enabled || ip.whitelist[senderID] || msgText == ""

		// 安检（v5.1: 使用 Pipeline 统一编排 keyword→regex→pii→session→llm）
		var detectResult DetectResult
		if !skipDetect {
			ch := make(chan DetectResult, 1)
			go func() {
				defer func() {
					if rv := recover(); rv != nil {
						ch <- DetectResult{Action: "pass"}
					}
				}()
				ch <- ip.runPipelineDetect(msgText, appID, senderID, bridgeTraceID)
			}()
			select {
			case detectResult = <-ch:
			case <-time.After(ip.timeout):
				detectResult = DetectResult{Action: "pass", Reasons: []string{"timeout"}}
			}
		}

		// v20.1: 入站污染标记
		if ip.taintTracker != nil {
			taintEntry := ip.taintTracker.MarkTainted(bridgeTraceID, msgText, "inbound")
			if taintEntry != nil {
				log.Printf("[桥接入站] 🏷️ 污染标记 sender=%s trace=%s labels=%v", senderID, bridgeTraceID, taintEntry.Labels)
			}
		}

		// 审计日志
		latMs := float64(time.Since(start).Microseconds()) / 1000.0
		reason := strings.Join(detectResult.Reasons, ",")
		if len(detectResult.PIIs) > 0 {
			if reason != "" {
				reason += ","
			}
			reason += "pii:" + strings.Join(detectResult.PIIs, "+")
		}
		act := detectResult.Action
		if act == "" {
			act = "pass"
		}
		ip.logger.LogWithTrace("inbound", senderID, act, reason, msgText, rh, latMs, upstreamID, appID, bridgeTraceID)

		// v18.0: 执行信封
		if ip.envelopeMgr != nil {
			ip.envelopeMgr.Seal(bridgeTraceID, "inbound", msgText, act, detectResult.MatchedRules, senderID)
		}

		// 指标采集
		if ip.metrics != nil {
			ip.metrics.RecordRequest("inbound", act, ip.channel.Name(), latMs)
		}

		// v5.0 实时监控
		if ip.realtime != nil {
			ip.realtime.RecordInbound(act, time.Since(start).Microseconds())
			if act == "block" || act == "warn" {
				ip.realtime.RecordEvent("inbound", senderID, act, reason, bridgeTraceID)
			}
		}

		// v3.6 规则命中统计
		if ip.ruleHits != nil && len(detectResult.MatchedRules) > 0 {
			for _, ruleName := range detectResult.MatchedRules {
				ip.ruleHits.Record(ruleName)
			}
		}

		// 拦截
		if detectResult.Action == "block" {
			log.Printf("[桥接入站] 拦截 sender=%s reasons=%v", senderID, detectResult.Reasons)
			// v3.10 告警通知
			if ip.alertNotifier != nil {
				rule := strings.Join(detectResult.MatchedRules, ",")
				ip.alertNotifier.Notify("inbound", senderID, rule, msgText, appID)
			}
			// v18.1: 事件总线
			if ip.eventBus != nil {
				ip.eventBus.Emit(&SecurityEvent{
					Type: "inbound_block", Severity: "high", Domain: "inbound",
					TraceID: bridgeTraceID, SenderID: senderID,
					Summary: fmt.Sprintf("入站拦截: %s", strings.Join(detectResult.Reasons, "; ")),
					Details: map[string]interface{}{"rules": detectResult.MatchedRules, "app_id": appID},
				})
			}
			return
		}
		if detectResult.Action == "warn" {
			log.Printf("[桥接入站] 告警放行 sender=%s reasons=%v", senderID, detectResult.Reasons)
			// v18.1: 事件总线
			if ip.eventBus != nil {
				ip.eventBus.Emit(&SecurityEvent{
					Type: "inbound_block", Severity: "medium", Domain: "inbound",
					TraceID: bridgeTraceID, SenderID: senderID,
					Summary: fmt.Sprintf("入站告警: %s", strings.Join(detectResult.Reasons, "; ")),
					Details: map[string]interface{}{"rules": detectResult.MatchedRules, "action": "warn", "app_id": appID},
				})
			}
			// v15.0: 蜜罐触发检查
			if ip.honeypot != nil {
				tpl, watermark := ip.honeypot.ShouldTrigger(msgText, senderID, "")
				if tpl != nil {
					fakeResp := ip.honeypot.GenerateFakeResponse(tpl, watermark)
					ip.honeypot.RecordTrigger(&HoneypotTrigger{
						TenantID:      "default",
						SenderID:      senderID,
						TemplateID:    tpl.ID,
						TemplateName:  tpl.Name,
						TriggerType:   tpl.TriggerType,
						OriginalInput: msgText,
						FakeResponse:  fakeResp,
						Watermark:     watermark,
						TraceID:       bridgeTraceID,
					})
					ip.logger.LogWithTrace("inbound", senderID, "honeypot", "honeypot_triggered:"+tpl.Name, msgText, rh, latMs, upstreamID, appID, bridgeTraceID)
					// v19.2: 蜜罐深度交互记录
					if ip.honeypotDeep != nil {
						ip.honeypotDeep.RecordInteraction(senderID, tpl.TriggerType, "im", msgText)
					}
					log.Printf("[桥接入站] 🍯 蜜罐触发 sender=%s template=%s watermark=%s", senderID, tpl.Name, watermark)
					return // 不转发给上游，蜜罐已介入
				}
			}
		}

		// 获取上游地址
		var targetURL string
		func() {
			ip.pool.mu.RLock()
			defer ip.pool.mu.RUnlock()
			if upstreamID != "" {
				if up, ok := ip.pool.upstreams[upstreamID]; ok {
					targetURL = fmt.Sprintf("http://%s:%d", up.Address, up.Port)
				}
			}
			if targetURL == "" {
				for _, up := range ip.pool.upstreams {
					targetURL = fmt.Sprintf("http://%s:%d", up.Address, up.Port)
					break
				}
			}
		}()

		if targetURL == "" {
			log.Printf("[桥接入站] 无可用上游，丢弃消息 sender=%s", senderID)
			return
		}

		// 构建 HTTP POST 转发
		// v5.0: 转发请求，携带 X-Trace-ID
		fwdReq, err := http.NewRequest("POST", targetURL, bytes.NewReader(msg.Raw))
		if err != nil {
			log.Printf("[桥接入站] 创建转发请求失败: %v", err)
			return
		}
		fwdReq.Header.Set("Content-Type", "application/json")
		fwdReq.Header.Set("X-Trace-ID", bridgeTraceID)
		httpResp, err := http.DefaultClient.Do(fwdReq)
		if err != nil {
			log.Printf("[桥接入站] 转发失败: %v", err)
			return
		}
		defer httpResp.Body.Close()
		io.Copy(io.Discard, httpResp.Body)
	})

	return nil
}

func (ip *InboundProxy) handleWecomVerify(w http.ResponseWriter, r *http.Request, wp *WecomPlugin) {
	q := r.URL.Query()
	msgSignature := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if msgSignature == "" || timestamp == "" || nonce == "" || echostr == "" {
		http.Error(w, "Bad Request: missing parameters", 400)
		return
	}

	plainEchoStr, err := wp.VerifyURL(msgSignature, timestamp, nonce, echostr)
	if err != nil {
		log.Printf("[企微验证] 验证失败: %v", err)
		http.Error(w, "Forbidden: verification failed", 403)
		return
	}

	log.Printf("[企微验证] GET 验证成功，返回明文 echostr")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(200)
	w.Write([]byte(plainEchoStr))
}

func (ip *InboundProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// panic recovery
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[PANIC] InboundProxy: %v\n%s", rv, debug.Stack())
			http.Error(w, "Internal Server Error", 500)
		}
	}()

	start := time.Now()

	// v5.0: 生成 trace_id
	traceID := GenerateTraceID()
	// v4.1: WebSocket Upgrade 检测
	if IsWebSocketUpgrade(r) && ip.wsProxy != nil {
		// 从 query 或 header 提取 sender_id / app_id
		senderID := r.URL.Query().Get("sender_id")
		if senderID == "" {
			senderID = r.Header.Get("X-Sender-Id")
		}
		appID := r.URL.Query().Get("app_id")
		if appID == "" {
			appID = r.Header.Get("X-App-Id")
		}
		ip.wsProxy.HandleWebSocket(w, r, senderID, appID)
		return
	}

	// 企微 GET 验证回调
	if r.Method == "GET" {
		if wp, ok := ip.channel.(*WecomPlugin); ok {
			ip.handleWecomVerify(w, r, wp)
			return
		}
		// 非企微通道的 GET 请求，转发到上游
		proxy, _ := ip.pool.GetAnyHealthyProxy()
		if proxy != nil {
			proxy.ServeHTTP(w, r)
		} else {
			w.WriteHeader(502)
			w.Write([]byte(`{"errcode":502,"errmsg":"no upstream"}`))
		}
		return
	}

	if r.Method != http.MethodPost {
		// 非POST直接转发到任意健康上游
		proxy, _ := ip.pool.GetAnyHealthyProxy()
		if proxy != nil { proxy.ServeHTTP(w, r) } else {
			w.WriteHeader(502); w.Write([]byte(`{"errcode":502,"errmsg":"no upstream"}`))
		}
		return
	}

	// 入站超时保护：整个入站处理不超过 30 秒
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)

	body, err := io.ReadAll(r.Body); r.Body.Close()
	if err != nil {
		proxy, _ := ip.pool.GetAnyHealthyProxy()
		if proxy != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			proxy.ServeHTTP(w, r)
		}
		return
	}
	rh := fmt.Sprintf("%x", sha256.Sum256(body))

	// 使用通道插件解析入站消息
	var msgText, senderID, eventType, appID string
	var decryptOK bool
	var isVerify bool
	func() {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("[入站] ParseInbound panic: %v", rv)
			}
		}()
		// 优先使用 RequestAwareParser（支持从 URL query 提取参数）
		var msg InboundMessage
		var err error
		if rap, ok := ip.channel.(RequestAwareParser); ok {
			msg, err = rap.ParseInboundRequest(body, r)
		} else {
			msg, err = ip.channel.ParseInbound(body)
		}
		if err != nil {
			log.Printf("[入站] 解析失败: %v，fail-open", err)
			return
		}
		// URL Verification / echostr 验证特殊处理（飞书等）
		if msg.IsVerify && msg.VerifyReply != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(msg.VerifyReply)
			isVerify = true
			log.Printf("[入站] URL Verification 处理完成")
			return
		}
		// 兼容旧逻辑：飞书 URL Verification
		if msg.EventType == "url_verification" && msg.Raw != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(msg.Raw)
			isVerify = true
			return
		}
		msgText = msg.Text
		senderID = msg.SenderID
		eventType = msg.EventType
		appID = msg.AppID
		decryptOK = true
	}()

	// 如果是验证请求，已在闭包中直接响应，不再继续
	if isVerify {
		return
	}

	// v18: 记录 sender→trace 映射，供出站关联
	if ip.traceCorrelator != nil && senderID != "" {
		ip.traceCorrelator.Set(senderID, traceID)
	}

	// v17.3: 注册 IM→LLM 会话关联（内容指纹 → IM trace_id）
	if ip.sessionCorrelator != nil && msgText != "" {
		ip.sessionCorrelator.RegisterIMSession(msgText, traceID, senderID, appID)
	}

	// 限流检查（安检之前）
	if ip.limiter != nil {
		allowed, reason := ip.limiter.Allow(senderID)
		if !allowed {
			if ip.metrics != nil {
				ip.metrics.RecordRateLimit(false)
				ip.metrics.RecordRequest("inbound", "rate_limited", ip.channel.Name(), 0)
			}
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Trace-ID", traceID)
			w.WriteHeader(429)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errcode": 429,
				"errmsg":  "rate limited",
				"detail":  reason,
			})
			ip.logger.Log("inbound", senderID, "rate_limited", reason, truncate(msgText, 200), rh, 0, "", appID)
			return
		}
		if ip.metrics != nil {
			ip.metrics.RecordRateLimit(true)
		}
	}

	// 路由决策
	var upstreamID string
	if senderID != "" {
		uid, found := ip.routes.Lookup(senderID, appID)
		if found {
			if ip.pool.IsHealthy(uid) {
				upstreamID = uid
			} else {
				// 故障转移：选择新的健康上游
				newUID := ip.pool.SelectUpstream(ip.policy)
				if newUID != "" && newUID != uid {
					ip.pool.IncrUserCount(uid, -1)
					ip.pool.IncrUserCount(newUID, 1)
					ip.routes.Migrate(senderID, appID, uid, newUID)
					upstreamID = newUID
					log.Printf("[路由] 故障转移 sender=%s app=%s: %s -> %s", senderID, appID, uid, newUID)
				} else {
					upstreamID = uid // failopen: 仍尝试原上游
				}
			}
		} else {
			// v3.9: 先尝试策略匹配
			policyMatched := false
			if ip.policyEng != nil && ip.userCache != nil {
				if info := ip.userCache.GetCached(senderID); info != nil {
					if pUID, ok := ip.policyEng.Match(info, appID); ok && pUID != "" {
						if ip.pool.IsHealthy(pUID) {
							upstreamID = pUID
							ip.routes.Bind(senderID, appID, upstreamID)
							ip.pool.IncrUserCount(upstreamID, 1)
							policyMatched = true
							log.Printf("[路由] 策略匹配绑定 sender=%s app=%s -> %s (email=%s dept=%s)", senderID, appID, upstreamID, info.Email, info.Department)
						}
					}
				}
			}
			if !policyMatched {
				// 新用户分配
				upstreamID = ip.pool.SelectUpstream(ip.policy)
				if upstreamID != "" {
					ip.routes.Bind(senderID, appID, upstreamID)
					ip.pool.IncrUserCount(upstreamID, 1)
					log.Printf("[路由] 新用户绑定 sender=%s app=%s -> %s", senderID, appID, upstreamID)
				}
			}
		}
	}

	// v3.9: 异步获取用户信息
	if senderID != "" && ip.userCache != nil {
		go func(sid, aID string) {
			defer func() { recover() }()
			info, err := ip.userCache.GetOrFetch(sid)
			if err == nil && info != nil {
				ip.routes.UpdateUserInfo(sid, info.Name, info.Email, info.Department)
				// 如果还没通过策略匹配路由，尝试策略匹配
				if ip.policyEng != nil {
					if _, found := ip.routes.Lookup(sid, aID); !found {
						if pUID, ok := ip.policyEng.Match(info, aID); ok && pUID != "" && ip.pool.IsHealthy(pUID) {
							ip.routes.Bind(sid, aID, pUID)
							ip.pool.IncrUserCount(pUID, 1)
							log.Printf("[路由] 异步策略匹配绑定 sender=%s -> %s", sid, pUID)
						}
					}
				}
			}
		}(senderID, appID)
	}

	// 获取代理
	var proxy *httputil.ReverseProxy
	if upstreamID != "" {
		proxy = ip.pool.GetProxy(upstreamID)
	}
	if proxy == nil {
		proxy, upstreamID = ip.pool.GetAnyHealthyProxy()
	}
	if proxy == nil {
		w.WriteHeader(502)
		w.Write([]byte(`{"errcode":502,"errmsg":"no upstream available"}`))
		return
	}

	// 检测（白名单跳过）（v5.1: 使用 Pipeline 统一编排 keyword→regex→pii→session→llm）
	skipDetect := !ip.enabled || ip.whitelist[senderID] || !decryptOK || msgText == ""
	var detectResult DetectResult
	if !skipDetect {
		ch := make(chan DetectResult, 1)
		go func() {
			defer func() { if rv := recover(); rv != nil { ch <- DetectResult{Action: "pass"} } }()
			ch <- ip.runPipelineDetect(msgText, appID, senderID, traceID)
		}()
		select {
		case detectResult = <-ch:
		case <-time.After(ip.timeout):
			detectResult = DetectResult{Action: "pass", Reasons: []string{"timeout"}}
		}
	}

	// 构建审计信息
	latMs := float64(time.Since(start).Microseconds()) / 1000.0
	reason := strings.Join(detectResult.Reasons, ",")
	if len(detectResult.PIIs) > 0 {
		if reason != "" { reason += "," }
		reason += "pii:" + strings.Join(detectResult.PIIs, "+")
	}
	act := detectResult.Action; if act == "" { act = "pass" }
	_ = eventType

	// v18.3: 自适应决策 — 基于贝叶斯误伤率分析可能降级 block→warn
	if ip.adaptiveEngine != nil && act == "block" {
		newAction, proof := ip.adaptiveEngine.ShouldDowngrade(senderID, act)
		if newAction != act {
			act = newAction
			reason = fmt.Sprintf("adaptive_downgrade: P(FP)=%.3f [%.3f,%.3f]", proof.PosteriorMean, proof.PosteriorLower, proof.PosteriorUpper)
		}
	}

	// v20.1: 入站污染标记
	if ip.taintTracker != nil {
		taintEntry := ip.taintTracker.MarkTainted(traceID, msgText, "inbound")
		if taintEntry != nil {
			log.Printf("[入站] 🏷️ 污染标记 sender=%s trace=%s labels=%v", senderID, traceID, taintEntry.Labels)
		}
	}

	ip.logger.LogWithTrace("inbound", senderID, act, reason, msgText, rh, latMs, upstreamID, appID, traceID)

	// v18.0: 执行信封
	if ip.envelopeMgr != nil {
		ip.envelopeMgr.Seal(traceID, "inbound", msgText, act, detectResult.MatchedRules, senderID)
	}

	// 指标采集
	if ip.metrics != nil {
		ip.metrics.RecordRequest("inbound", act, ip.channel.Name(), latMs)
	}

	// v5.0 实时监控
	if ip.realtime != nil {
		ip.realtime.RecordInbound(act, time.Since(start).Microseconds())
		if act == "block" || act == "warn" {
			ip.realtime.RecordEvent("inbound", senderID, act, reason, traceID)
		}
	}

	// v3.6 规则命中统计
	if ip.ruleHits != nil && len(detectResult.MatchedRules) > 0 {
		for _, ruleName := range detectResult.MatchedRules {
			ip.ruleHits.Record(ruleName)
		}
	}

	// 执行决策
	if detectResult.Action == "block" {
		if ip.slog != nil {
			ip.slog.Warn("inbound", "请求拦截", "sender_id", senderID, "action", "block", "reason", reason, "trace_id", traceID)
		} else {
			log.Printf("[入站] 拦截 sender=%s reasons=%v trace_id=%s", senderID, detectResult.Reasons, traceID)
		}
		// v3.10 告警通知
		if ip.alertNotifier != nil {
			rule := strings.Join(detectResult.MatchedRules, ",")
			ip.alertNotifier.Notify("inbound", senderID, rule, msgText, appID)
		}
		// v18.1: 事件总线
		if ip.eventBus != nil {
			ip.eventBus.Emit(&SecurityEvent{
				Type: "inbound_block", Severity: "high", Domain: "inbound",
				TraceID: traceID, SenderID: senderID,
				Summary: fmt.Sprintf("入站拦截: %s", strings.Join(detectResult.Reasons, "; ")),
				Details: map[string]interface{}{"rules": detectResult.MatchedRules, "app_id": appID},
			})
		}
		code, respBody := ip.channel.BlockResponseWithMessage(detectResult.Message)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-ID", traceID)
		w.WriteHeader(code)
		w.Write(respBody)
		return
	}
	if detectResult.Action == "warn" {
		if ip.slog != nil {
			ip.slog.Warn("inbound", "告警放行", "sender_id", senderID, "action", "warn", "reason", reason, "trace_id", traceID)
		} else {
			log.Printf("[入站] 告警放行 sender=%s reasons=%v trace_id=%s", senderID, detectResult.Reasons, traceID)
		}
		// v18.1: 事件总线
		if ip.eventBus != nil {
			ip.eventBus.Emit(&SecurityEvent{
				Type: "inbound_block", Severity: "medium", Domain: "inbound",
				TraceID: traceID, SenderID: senderID,
				Summary: fmt.Sprintf("入站告警: %s", strings.Join(detectResult.Reasons, "; ")),
				Details: map[string]interface{}{"rules": detectResult.MatchedRules, "action": "warn", "app_id": appID},
			})
		}
		// v15.0: 蜜罐触发检查
		if ip.honeypot != nil {
			tpl, watermark := ip.honeypot.ShouldTrigger(msgText, senderID, "")
			if tpl != nil {
				fakeResp := ip.honeypot.GenerateFakeResponse(tpl, watermark)
				ip.honeypot.RecordTrigger(&HoneypotTrigger{
					TenantID:      "default",
					SenderID:      senderID,
					TemplateID:    tpl.ID,
					TemplateName:  tpl.Name,
					TriggerType:   tpl.TriggerType,
					OriginalInput: msgText,
					FakeResponse:  fakeResp,
					Watermark:     watermark,
					TraceID:       traceID,
				})
				ip.logger.LogWithTrace("inbound", senderID, "honeypot", "honeypot_triggered:"+tpl.Name, msgText, rh, latMs, upstreamID, appID, traceID)
				// v19.2: 蜜罐深度交互记录
				if ip.honeypotDeep != nil {
					ip.honeypotDeep.RecordInteraction(senderID, tpl.TriggerType, "im", msgText)
				}
				log.Printf("[入站] 🍯 蜜罐触发 sender=%s template=%s watermark=%s trace_id=%s", senderID, tpl.Name, watermark, traceID)
				// 返回蜜罐假响应而不是转发给上游
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Trace-ID", traceID)
				w.WriteHeader(200)
				w.Write([]byte(fmt.Sprintf(`{"errcode":0,"errmsg":"ok","honeypot_response":%q}`, fakeResp)))
				return
			}
		}
	}

	// v5.0: 设置 X-Trace-ID header 传递给上游
	r.Header.Set("X-Trace-ID", traceID)
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	// v5.0: 包装 ResponseWriter 以在响应中添加 X-Trace-ID
	tw := &traceResponseWriter{ResponseWriter: w, traceID: traceID, headerWritten: false}
	proxy.ServeHTTP(tw, r)
}

// ============================================================
// Pipeline 检测辅助方法
// ============================================================

// runPipelineDetect 使用 Pipeline 进行检测，回退到 engine.DetectWithAppID
// 返回兼容的 DetectResult 以减少对现有代码的侵入
func (ip *InboundProxy) runPipelineDetect(msgText, appID, senderID, traceID string) DetectResult {
	if ip.pipeline != nil {
		ctx := &DetectContext{
			Text:     msgText,
			SenderID: senderID,
			AppID:    appID,
			TraceID:  traceID,
		}
		pResult := ip.pipeline.Execute(ctx)
		// 转换 PipelineResult → DetectResult
		dr := DetectResult{
			Action:       pResult.FinalAction,
			MatchedRules: pResult.MatchedRules,
			PIIs:         pResult.PIIs,
			Message:      pResult.FinalMessage,
		}
		if dr.Action == "" {
			dr.Action = "pass"
		}
		// 收集 reasons
		for _, sr := range pResult.StageResults {
			if sr.Detail != "" && sr.Action != "pass" {
				dr.Reasons = append(dr.Reasons, sr.Detail)
			}
		}
		if pResult.FinalRule != "" && len(dr.Reasons) == 0 {
			dr.Reasons = []string{pResult.FinalRule}
		}
		// 日志: 各阶段耗时
		if ip.slog != nil {
			for _, sr := range pResult.StageResults {
				if sr.Action != "pass" {
					ip.slog.Info("pipeline", "阶段命中",
						"stage", sr.StageName, "action", sr.Action,
						"rule", sr.RuleName, "duration_us", sr.Duration.Microseconds())
				}
			}
		}
		return dr
	}
	// 回退: 直接调用引擎
	return ip.engine.DetectWithAppID(msgText, appID)
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

// ============================================================
// 出站代理 v3.0
// ============================================================

type OutboundProxy struct {
	channel        ChannelPlugin
	inboundEngine  *RuleEngine
	outboundEngine *OutboundRuleEngine
	logger         *AuditLogger
	proxy          *httputil.ReverseProxy
	enabled        bool
	metrics        *MetricsCollector // v3.4 指标采集器
	ruleHits       *RuleHitStats     // v3.6 规则命中统计
	alertNotifier  *AlertNotifier    // v3.10 告警通知器
	realtime       *RealtimeMetrics  // v5.0 实时监控
	// v15.0 蜜罐引擎
	honeypot *HoneypotEngine
	// v18 出站 trace 关联
	traceCorrelator *TraceCorrelator
	// v18.0 执行信封
	envelopeMgr *EnvelopeManager
	// v18.1 事件总线
	eventBus *EventBus
	// v20.1 污染追踪引擎
	taintTracker *TaintTracker
}

func NewOutboundProxy(cfg *Config, channel ChannelPlugin, inboundEngine *RuleEngine, outboundEngine *OutboundRuleEngine, logger *AuditLogger, metrics *MetricsCollector, ruleHits *RuleHitStats, honeypot *HoneypotEngine) (*OutboundProxy, error) {
	up, err := url.Parse(cfg.LanxinUpstream)
	if err != nil { return nil, err }
	p := httputil.NewSingleHostReverseProxy(up)
	p.Transport = &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns: 50, MaxIdleConnsPerHost: 50, IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	od := p.Director
	p.Director = func(r *http.Request) { od(r); r.Host = up.Host }
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		log.Printf("[出站] 转发错误: %v", e)
		w.WriteHeader(502)
		w.Write([]byte(`{"errcode":502,"errmsg":"lanxin api unavailable"}`))
	}
	return &OutboundProxy{
		channel: channel, inboundEngine: inboundEngine, outboundEngine: outboundEngine,
		logger: logger, proxy: p, enabled: cfg.OutboundAuditEnabled,
		metrics: metrics, ruleHits: ruleHits, honeypot: honeypot,
	}, nil
}

func (op *OutboundProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// panic recovery
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[PANIC] OutboundProxy: %v\n%s", rv, debug.Stack())
			http.Error(w, "Internal Server Error", 500)
		}
	}()

	start := time.Now()
	if !op.enabled || !op.channel.ShouldAuditOutbound(r.URL.Path) {
		op.proxy.ServeHTTP(w, r)
		return
	}

	// v18: 出站 trace_id — 优先从关联缓存查（实现入站↔出站关联），其次从请求头，最后自动生成
	var outTraceID string
	// 先提取 recipient，再查关联缓存
	// recipient 在后面才提取，这里先用 header
	outTraceID = r.Header.Get("X-Trace-ID")

	// 出站 body 大小限制：最大 10MB，防止 OOM
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)); r.Body.Close()
	if err != nil { op.proxy.ServeHTTP(w, r); return }
	rh := fmt.Sprintf("%x", sha256.Sum256(body))

	// 使用通道插件提取出站消息文本
	var text string
	var recipient string
	var outAppID string
	func() {
		defer func() { recover() }()
		t, ok := op.channel.ExtractOutbound(r.URL.Path, body)
		if ok { text = t }
		// 提取接收者（蓝信: userIdList/groupId）
		type recipientExtractor interface {
			ExtractOutboundRecipient([]byte) string
		}
		if re, ok := op.channel.(recipientExtractor); ok {
			recipient = re.ExtractOutboundRecipient(body)
		}
		// 提取 appId
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			if a, ok := m["appId"].(string); ok { outAppID = a }
		}
	}()

	// v18: 出站 trace 关联 — 用 recipient 查入站时记录的 trace_id
	if outTraceID == "" && op.traceCorrelator != nil && recipient != "" {
		outTraceID = op.traceCorrelator.Get(recipient)
	}
	if outTraceID == "" {
		outTraceID = GenerateTraceID()
	}

	// v15.0: 蜜罐引爆检测 — 检查出站内容中是否包含蜜罐水印
	if op.honeypot != nil && text != "" {
		detonatedWatermarks := op.honeypot.CheckDetonation(text)
		if len(detonatedWatermarks) > 0 {
			latMs := float64(time.Since(start).Microseconds()) / 1000.0
			upstreamID := r.Header.Get("X-Upstream-Id")
			detonationReason := "honeypot_detonation:" + strings.Join(detonatedWatermarks, ",")
			pv := text; if rs := []rune(pv); len(rs) > 500 { pv = string(rs[:500]) + "..." }
			op.logger.LogWithTrace("outbound", recipient, "honeypot_detonation", detonationReason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
			log.Printf("[出站] 💣 蜜罐引爆检测 path=%s watermarks=%v", r.URL.Path, detonatedWatermarks)
			// v18.1: 事件总线
			if op.eventBus != nil {
				op.eventBus.Emit(&SecurityEvent{
					Type: "honeypot_triggered", Severity: "critical", Domain: "outbound",
					TraceID: outTraceID, SenderID: recipient,
					Summary: fmt.Sprintf("蜜罐引爆: 水印 %v 出现在出站内容中", detonatedWatermarks),
					Details: map[string]interface{}{"watermarks": detonatedWatermarks},
				})
			}
			if op.realtime != nil {
				op.realtime.RecordOutbound("honeypot_detonation", time.Since(start).Microseconds())
				op.realtime.RecordEvent("outbound", recipient, "honeypot_detonation", detonationReason, outTraceID)
			}
			// 阻断包含蜜罐水印的出站消息
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			w.Write([]byte(`{"errcode":403,"errmsg":"honeypot detonation detected","detail":"outbound message contains tracked watermark"}`))
			return
		}
	}

	// v20.1: 出站污染追踪检查（血统级阻断）
	if op.taintTracker != nil && outTraceID != "" {
		taintDecision := op.taintTracker.CheckOutbound(outTraceID)
		if taintDecision.Tainted {
			latMs := float64(time.Since(start).Microseconds()) / 1000.0
			upstreamID := r.Header.Get("X-Upstream-Id")
			taintReason := fmt.Sprintf("taint_%s: labels=%v %s", taintDecision.Action, taintDecision.Labels, taintDecision.Reason)
			pv := text; if rs := []rune(pv); len(rs) > 500 { pv = string(rs[:500]) + "..." }
			op.logger.LogWithTrace("outbound", recipient, "taint_"+taintDecision.Action, taintReason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
			if taintDecision.Action == "block" {
				log.Printf("[出站] 🔒 污染阻断 trace=%s labels=%v", outTraceID, taintDecision.Labels)
				if op.eventBus != nil {
					op.eventBus.Emit(&SecurityEvent{
						Type: "taint_block", Severity: "high", Domain: "outbound",
						TraceID: outTraceID, SenderID: recipient,
						Summary: fmt.Sprintf("污染阻断: %v", taintDecision.Labels),
						Details: map[string]interface{}{"labels": taintDecision.Labels, "reason": taintDecision.Reason},
					})
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(403)
				w.Write([]byte(fmt.Sprintf(`{"errcode":403,"errmsg":"tainted data blocked","labels":%q,"trace_id":%q}`,
					strings.Join(taintDecision.Labels, ","), outTraceID)))
				return
			}
			if taintDecision.Action == "warn" {
				log.Printf("[出站] ⚠️ 污染告警放行 trace=%s labels=%v", outTraceID, taintDecision.Labels)
			}
		}
	}

	// 出站规则检测
	result := op.outboundEngine.Detect(text)
	latMs := float64(time.Since(start).Microseconds()) / 1000.0

	// 获取来源容器 ID（从 X-Upstream-Id header 或来源 IP）
	upstreamID := r.Header.Get("X-Upstream-Id")

	pv := text; if rs := []rune(pv); len(rs) > 500 { pv = string(rs[:500]) + "..." }

	// v3.6 规则命中统计
	if op.ruleHits != nil && result.RuleName != "" {
		op.ruleHits.Record(result.RuleName)
	}

	// v18.0: 执行信封
	if op.envelopeMgr != nil {
		var envRules []string
		if result.RuleName != "" {
			envRules = []string{result.RuleName}
		}
		op.envelopeMgr.Seal(outTraceID, "outbound", text, result.Action, envRules, "")
	}

	switch result.Action {
	case "block":
		log.Printf("[出站] 拦截 path=%s rule=%s", r.URL.Path, result.RuleName)
		op.logger.LogWithTrace("outbound", recipient, "block", result.Reason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "block", op.channel.Name(), latMs)
		}
		if op.realtime != nil {
			op.realtime.RecordOutbound("block", time.Since(start).Microseconds())
			op.realtime.RecordEvent("outbound", recipient, "block", result.Reason, outTraceID)
		}
		// v3.10 告警通知
		if op.alertNotifier != nil {
			op.alertNotifier.Notify("outbound", recipient, result.RuleName, text, outAppID)
		}
		// v18.1: 事件总线
		if op.eventBus != nil {
			op.eventBus.Emit(&SecurityEvent{
				Type: "outbound_block", Severity: "high", Domain: "outbound",
				TraceID: outTraceID, SenderID: recipient,
				Summary: fmt.Sprintf("出站拦截: %s", result.Reason),
				Details: map[string]interface{}{"rule": result.RuleName, "app_id": outAppID},
			})
		}
		code, respBody := op.channel.OutboundBlockResponseWithMessage(result.Reason, result.RuleName, result.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(respBody)
		return
	case "warn":
		log.Printf("[出站] 告警放行 path=%s rule=%s", r.URL.Path, result.RuleName)
		op.logger.LogWithTrace("outbound", recipient, "warn", result.Reason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "warn", op.channel.Name(), latMs)
		}
		if op.realtime != nil {
			op.realtime.RecordOutbound("warn", time.Since(start).Microseconds())
			op.realtime.RecordEvent("outbound", recipient, "warn", result.Reason, outTraceID)
		}
		// v18.1: 事件总线
		if op.eventBus != nil {
			op.eventBus.Emit(&SecurityEvent{
				Type: "outbound_block", Severity: "medium", Domain: "outbound",
				TraceID: outTraceID, SenderID: recipient,
				Summary: fmt.Sprintf("出站告警: %s", result.Reason),
				Details: map[string]interface{}{"rule": result.RuleName, "action": "warn", "app_id": outAppID},
			})
		}
	case "log":
		op.logger.LogWithTrace("outbound", recipient, "log", result.Reason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "log", op.channel.Name(), latMs)
		}
		if op.realtime != nil {
			op.realtime.RecordOutbound("log", time.Since(start).Microseconds())
		}
	default:
		// v1.0 兼容：PII 检测
		piis := op.inboundEngine.DetectPII(text)
		action, reason := "pass", ""
		if len(piis) > 0 {
			action = "pii_detected"; reason = "outbound_pii:" + strings.Join(piis, "+")
			log.Printf("[出站] PII path=%s piis=%v", r.URL.Path, piis)
		}
		op.logger.LogWithTrace("outbound", recipient, action, reason, pv, rh, latMs, upstreamID, outAppID, outTraceID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", action, op.channel.Name(), latMs)
		}
		if op.realtime != nil {
			op.realtime.RecordOutbound(action, time.Since(start).Microseconds())
		}
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	op.proxy.ServeHTTP(w, r)
}

