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
	"time"
)

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
}

func NewInboundProxy(cfg *Config, channel ChannelPlugin, engine *RuleEngine, logger *AuditLogger, pool *UpstreamPool, routes *RouteTable, metrics *MetricsCollector, ruleHits *RuleHitStats, userCache *UserInfoCache, policyEng *RoutePolicyEngine) *InboundProxy {
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

		// 安检
		var detectResult DetectResult
		if !skipDetect {
			ch := make(chan DetectResult, 1)
			go func() {
				defer func() {
					if rv := recover(); rv != nil {
						ch <- DetectResult{Action: "pass"}
					}
				}()
				ch <- ip.engine.DetectWithAppID(msgText, appID)
			}()
			select {
			case detectResult = <-ch:
			case <-time.After(ip.timeout):
				detectResult = DetectResult{Action: "pass", Reasons: []string{"timeout"}}
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
		ip.logger.Log("inbound", senderID, act, reason, msgText, rh, latMs, upstreamID, appID)

		// 指标采集
		if ip.metrics != nil {
			ip.metrics.RecordRequest("inbound", act, ip.channel.Name(), latMs)
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
			return
		}
		if detectResult.Action == "warn" {
			log.Printf("[桥接入站] 告警放行 sender=%s reasons=%v", senderID, detectResult.Reasons)
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
		httpResp, err := http.Post(targetURL, "application/json", bytes.NewReader(msg.Raw))
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

	// 检测（白名单跳过）
	skipDetect := !ip.enabled || ip.whitelist[senderID] || !decryptOK || msgText == ""
	var detectResult DetectResult
	if !skipDetect {
		ch := make(chan DetectResult, 1)
		go func() {
			defer func() { if rv := recover(); rv != nil { ch <- DetectResult{Action: "pass"} } }()
			ch <- ip.engine.DetectWithAppID(msgText, appID)
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
	ip.logger.Log("inbound", senderID, act, reason, msgText, rh, latMs, upstreamID, appID)

	// 指标采集
	if ip.metrics != nil {
		ip.metrics.RecordRequest("inbound", act, ip.channel.Name(), latMs)
	}

	// v3.6 规则命中统计
	if ip.ruleHits != nil && len(detectResult.MatchedRules) > 0 {
		for _, ruleName := range detectResult.MatchedRules {
			ip.ruleHits.Record(ruleName)
		}
	}

	// 执行决策
	if detectResult.Action == "block" {
		log.Printf("[入站] 拦截 sender=%s reasons=%v", senderID, detectResult.Reasons)
		// v3.10 告警通知
		if ip.alertNotifier != nil {
			rule := strings.Join(detectResult.MatchedRules, ",")
			ip.alertNotifier.Notify("inbound", senderID, rule, msgText, appID)
		}
		code, respBody := ip.channel.BlockResponseWithMessage(detectResult.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(respBody)
		return
	}
	if detectResult.Action == "warn" {
		log.Printf("[入站] 告警放行 sender=%s reasons=%v", senderID, detectResult.Reasons)
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	proxy.ServeHTTP(w, r)
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
}

func NewOutboundProxy(cfg *Config, channel ChannelPlugin, inboundEngine *RuleEngine, outboundEngine *OutboundRuleEngine, logger *AuditLogger, metrics *MetricsCollector, ruleHits *RuleHitStats) (*OutboundProxy, error) {
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
		metrics: metrics, ruleHits: ruleHits,
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

	switch result.Action {
	case "block":
		log.Printf("[出站] 拦截 path=%s rule=%s", r.URL.Path, result.RuleName)
		op.logger.Log("outbound", recipient, "block", result.Reason, pv, rh, latMs, upstreamID, outAppID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "block", op.channel.Name(), latMs)
		}
		// v3.10 告警通知
		if op.alertNotifier != nil {
			op.alertNotifier.Notify("outbound", recipient, result.RuleName, text, outAppID)
		}
		code, respBody := op.channel.OutboundBlockResponseWithMessage(result.Reason, result.RuleName, result.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(respBody)
		return
	case "warn":
		log.Printf("[出站] 告警放行 path=%s rule=%s", r.URL.Path, result.RuleName)
		op.logger.Log("outbound", recipient, "warn", result.Reason, pv, rh, latMs, upstreamID, outAppID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "warn", op.channel.Name(), latMs)
		}
	case "log":
		op.logger.Log("outbound", recipient, "log", result.Reason, pv, rh, latMs, upstreamID, outAppID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", "log", op.channel.Name(), latMs)
		}
	default:
		// v1.0 兼容：PII 检测
		piis := op.inboundEngine.DetectPII(text)
		action, reason := "pass", ""
		if len(piis) > 0 {
			action = "pii_detected"; reason = "outbound_pii:" + strings.Join(piis, "+")
			log.Printf("[出站] PII path=%s piis=%v", r.URL.Path, piis)
		}
		op.logger.Log("outbound", recipient, action, reason, pv, rh, latMs, upstreamID, outAppID)
		if op.metrics != nil {
			op.metrics.RecordRequest("outbound", action, op.channel.Name(), latMs)
		}
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	op.proxy.ServeHTTP(w, r)
}

