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
	// 从当前位置摘出
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node == tc.tail {
		tc.tail = node.prev
	}
	// 插到头部
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
		// 更新已有节点
		node.traceID = traceID
		node.ts = time.Now()
		tc.moveToFront(node)
		return
	}
	// 新建节点
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
	// 淘汰最旧的（O(1)）
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
	// 5 分钟窗口
	if time.Since(node.ts) > 5*time.Minute {
		return ""
	}
	tc.moveToFront(node) // 读也算访问
	return node.traceID
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
	// v18.3 奇点蜜罐引擎
	singularityEngine *SingularityEngine
	// v23.0 路径级策略引擎
	pathPolicyEngine *PathPolicyEngine
	// v25.0 执行计划编译器
	planCompiler      *PlanCompiler
	capabilityEngine  *CapabilityEngine
	deviationDetector *DeviationDetector
	// v26.0 信息流控制引擎
	ifcEngine *IFCEngine
	// v27.0 租户识别
	tenantMgr *TenantManager
	apiKeyMgr *APIKeyManager
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

// SetTenantManager 设置租户管理器（v27.0）
func (ip *InboundProxy) SetTenantManager(tm *TenantManager) { ip.tenantMgr = tm }

// SetAPIKeyManager 设置 API Key 管理器（v27.0）
func (ip *InboundProxy) SetAPIKeyManager(akm *APIKeyManager) { ip.apiKeyMgr = akm }

// resolveTenantID 根据 senderID 和 appID 解析真实租户 ID（v27.0）
func (ip *InboundProxy) resolveTenantID(senderID, appID string) string {
	if ip.tenantMgr != nil {
		return ip.tenantMgr.ResolveTenant(senderID, appID)
	}
	return "default"
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
			upstreamID = ip.resolveUpstream(senderID, appID, "[桥接路由]")
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

		// v23.0+v23.1: 路径级策略 — 注册入站步骤（带 sender 先验风险注入）
		if ip.pathPolicyEngine != nil {
			ip.pathPolicyEngine.RegisterStepWithSender(bridgeTraceID, senderID, PathStep{
				Stage: "inbound", Action: "inbound_message",
				Details: truncate(msgText, 100),
			})
		}

		// v20.1: 入站污染标记
		if ip.taintTracker != nil {
			taintEntry := ip.taintTracker.MarkTainted(bridgeTraceID, msgText, "inbound")
			if taintEntry != nil {
				log.Printf("[桥接入站] 🏷️ 污染标记 sender=%s trace=%s labels=%v", senderID, bridgeTraceID, taintEntry.Labels)
				// v23.0: 同步污染标签到路径上下文（驱动 cumulative 规则）
				if ip.pathPolicyEngine != nil {
					for _, label := range taintEntry.Labels {
						ip.pathPolicyEngine.AddTaintLabel(bridgeTraceID, label)
						ip.pathPolicyEngine.RegisterStep(bridgeTraceID, PathStep{
							Stage: "taint", Action: taintActionForLabel(label), Details: label,
						})
					}
				}
			}
		}

		// v25.0: Bridge 模式 — 编译执行计划
		if ip.planCompiler != nil && msgText != "" {
			plan := ip.planCompiler.CompileIntent(bridgeTraceID, msgText)
			if plan != nil {
				log.Printf("[桥接入站] 📋 执行计划已编译 trace=%s plan=%s steps=%d", bridgeTraceID, plan.ID, plan.TotalSteps)
			}
		}
		// v25.1: Bridge 模式 — 初始化 Capability 上下文
		if ip.capabilityEngine != nil && senderID != "" {
			userCaps := []CapLabel{
				{Name: "read", Source: "user_input", Level: "read", Granted: true},
				{Name: "write", Source: "user_input", Level: "write", Granted: true},
				{Name: "execute", Source: "user_input", Level: "execute", Granted: true},
			}
			ip.capabilityEngine.InitContext(bridgeTraceID, senderID, userCaps)
		}

		// v26.0: IFC 入站标签 (Bridge)
		if ip.ifcEngine != nil && ip.ifcEngine.config.Enabled {
			ip.ifcEngine.RegisterVariable(bridgeTraceID, "user_input", "user_input", msgText)
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

		// v23.0: 路径级策略 — 评估入站消息
		if ip.pathPolicyEngine != nil {
			ppDecision := ip.pathPolicyEngine.Evaluate(bridgeTraceID, "inbound_message")
			if actionSev(ppDecision.Decision) > actionSev(act) {
				act = ppDecision.Decision
				reason = reason + ",path_policy:" + ppDecision.Reason
			}
		}

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
			// D-009: 桥接模式 block 明确标记消息被丢弃（无法通知用户）
			// TODO: 后续版本通过蓝信 API 给发送者发送拦截通知
			log.Printf("[桥接入站] ⚠️ 消息已拦截并丢弃（桥接模式无法通知用户）sender=%s reasons=%v", senderID, detectResult.Reasons)
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
			// v18.3: 奇点蜜罐暴露（桥接模式）
			if ip.singularityEngine != nil {
				if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", bridgeTraceID); shouldExpose && tpl != nil {
					ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, bridgeTraceID)
					log.Printf("[桥接入站] 🔮 奇点暴露 sender=%s template=%s level=%d", senderID, tpl.Name, tpl.Level)
				}
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
						TenantID:      ip.resolveTenantID(senderID, appID),
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
					// v31.0: 蜜罐→攻击链事件发布
					if ip.eventBus != nil {
						ip.eventBus.Emit(&SecurityEvent{
							Type:     "honeypot_trigger",
							Severity: "high",
							SenderID: senderID,
							Summary: fmt.Sprintf("蜜罐触发: template=%s watermark=%s trigger_type=%s", tpl.Name, watermark, tpl.TriggerType),
							Details: map[string]interface{}{"template": tpl.Name, "watermark": watermark, "trigger_type": tpl.TriggerType},
							Domain: "inbound",
						})
					}
					log.Printf("[桥接入站] 🍯 蜜罐触发 sender=%s template=%s watermark=%s", senderID, tpl.Name, watermark)
					return // 不转发给上游，蜜罐已介入
				}
			}
			// v18.3: 奇点蜜罐暴露（桥接 warn 模式）
			if ip.singularityEngine != nil {
				if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", bridgeTraceID); shouldExpose && tpl != nil {
					ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, bridgeTraceID)
					log.Printf("[桥接入站] 🔮 奇点暴露(warn) sender=%s template=%s level=%d", senderID, tpl.Name, tpl.Level)
					return // 蜜罐已介入，不转发
				}
			}
		}

		// D-003: 获取上游地址（包含 path_prefix + 健康降级）
		var targetURL string
		func() {
			ip.pool.mu.RLock()
			defer ip.pool.mu.RUnlock()
			if upstreamID != "" {
				if up, ok := ip.pool.upstreams[upstreamID]; ok {
					targetURL = fmt.Sprintf("http://%s:%d%s", up.Address, up.Port, up.PathPrefix)
				}
			}
			if targetURL == "" {
				// 降级：使用 SelectUpstream 而非随机遍历 map
				var fallbackUID string
				for id, up := range ip.pool.upstreams {
					if up.Healthy {
						targetURL = fmt.Sprintf("http://%s:%d%s", up.Address, up.Port, up.PathPrefix)
						fallbackUID = id
						break
					}
				}
				if targetURL == "" {
					// 所有都不健康，failopen 取第一个
					for id, up := range ip.pool.upstreams {
						targetURL = fmt.Sprintf("http://%s:%d%s", up.Address, up.Port, up.PathPrefix)
						fallbackUID = id
						break
					}
				}
				// 降级后更新路由表
				if fallbackUID != "" && fallbackUID != upstreamID && senderID != "" {
					log.Printf("[桥接路由] ⚠️ 上游 %s 不可用，降级到 %s sender=%s", upstreamID, fallbackUID, senderID)
					if upstreamID != "" {
						ip.routes.Bind(senderID, appID, fallbackUID)
						ip.pool.TransferUserCount(upstreamID, fallbackUID)
					}
					upstreamID = fallbackUID
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

// resolveUpstream 统一路由决策：策略优先 → 亲和兜底 → 负载均衡
//
// 优先级模型（策略路由是权威的）：
//   1. 有策略规则能匹配 → 用策略结果（即使亲和绑定不同，也迁移）
//   2. 策略匹配不到 → 走亲和路由（已有绑定就用，故障就转移）
//   3. 都没有 → 负载均衡分配 + 异步纠偏
//
// 这保证管理员配的策略规则始终生效，不会被历史亲和绑定架空。
func (ip *InboundProxy) resolveUpstream(senderID, appID, logPrefix string) string {
	currentUID, hasBind := ip.routes.Lookup(senderID, appID)

	// ── 第一优先级：策略路由（权威） ──
	if ip.policyEng != nil && ip.userCache != nil {
		// 尝试获取用户信息（缓存命中立即返回，否则同步等最多 1.5s）
		info, _ := ip.userCache.GetOrFetchWithTimeout(senderID, 1500*time.Millisecond)
		if info != nil {
			ip.routes.UpdateUserInfo(senderID, info.Name, info.Email, info.Department)
			if pUID, ok := ip.policyEng.Match(info, appID); ok && pUID != "" {
				if ip.pool.IsHealthy(pUID) {
					if hasBind && currentUID == pUID {
						// 策略结果和亲和绑定一致，直接走
						return pUID
					}
					if hasBind && currentUID != pUID {
						// 策略结果与亲和绑定不一致 → 迁移到策略指定的上游
						if AtomicMigrate(ip.routes, ip.pool, senderID, appID, currentUID, pUID) {
							log.Printf("%s 策略路由覆盖亲和 sender=%s app=%s: %s -> %s (dept=%s email=%s)",
								logPrefix, senderID, appID, currentUID, pUID, info.Department, info.Email)
						} else {
							// CAS 失败（被并发修改），强制绑定
							ip.routes.Bind(senderID, appID, pUID)
							log.Printf("%s 策略路由强制绑定 sender=%s app=%s -> %s", logPrefix, senderID, appID, pUID)
						}
						return pUID
					}
					// 新用户，直接按策略绑定
					ip.routes.Bind(senderID, appID, pUID)
					ip.pool.IncrUserCount(pUID, 1)
					log.Printf("%s 策略匹配绑定 sender=%s app=%s -> %s (dept=%s email=%s)",
						logPrefix, senderID, appID, pUID, info.Department, info.Email)
					return pUID
				}
				// D-001: 策略匹配到但上游不健康 → 明确警告 + 审计日志
				log.Printf("%s ⚠️ 策略上游 %s 不健康，降级 sender=%s app=%s (dept=%s email=%s)",
					logPrefix, pUID, senderID, appID, info.Department, info.Email)
				if ip.logger != nil {
					ip.logger.Log("inbound", senderID, "policy_degraded",
						fmt.Sprintf("policy_upstream=%s unhealthy, degraded", pUID),
						"", "", 0, pUID, appID)
				}
				if ip.realtime != nil {
					ip.realtime.RecordEvent("inbound", senderID, "policy_degraded",
						fmt.Sprintf("策略上游 %s 不健康", pUID), "")
				}
			}
		} else if !hasBind {
			// 用户信息获取失败，但 default 策略不需要用户信息
			if pUID, ok := ip.policyEng.Match(nil, appID); ok && pUID != "" && ip.pool.IsHealthy(pUID) {
				ip.routes.Bind(senderID, appID, pUID)
				ip.pool.IncrUserCount(pUID, 1)
				log.Printf("%s default策略绑定(无用户信息) sender=%s app=%s -> %s", logPrefix, senderID, appID, pUID)
				return pUID
			}
			// 也没有 default 策略 → 降级到负载均衡，后面异步纠偏
		}
	}

	// ── 第二优先级：亲和路由（策略未匹配时的兜底） ──
	if hasBind {
		if ip.pool.IsHealthy(currentUID) {
			// 异步刷新用户信息（下次请求时策略可能就能匹配了）
			ip.asyncRefreshUserInfo(senderID, appID, currentUID, logPrefix)
			return currentUID
		}
		// 故障转移
		newUID := ip.pool.SelectUpstream(ip.policy)
		if newUID != "" && newUID != currentUID {
			if ip.routes.Migrate(senderID, appID, currentUID, newUID) {
				ip.pool.TransferUserCount(currentUID, newUID)
				log.Printf("%s 故障转移 sender=%s app=%s: %s -> %s", logPrefix, senderID, appID, currentUID, newUID)
			}
			return newUID
		}
		return currentUID // failopen
	}

	// ── 第三优先级：负载均衡（新用户，无策略匹配） ──
	upstreamID := ip.pool.SelectUpstream(ip.policy)
	if upstreamID != "" {
		ip.routes.Bind(senderID, appID, upstreamID)
		ip.pool.IncrUserCount(upstreamID, 1)
		log.Printf("%s 新用户绑定 sender=%s app=%s -> %s", logPrefix, senderID, appID, upstreamID)
		// 异步纠偏：后台获取用户信息，下次请求时策略生效
		ip.asyncPolicyCorrection(senderID, appID, upstreamID, logPrefix)
	}
	return upstreamID
}

// asyncRefreshUserInfo 异步刷新用户信息（仅更新缓存和 display_name，不改路由）
func (ip *InboundProxy) asyncRefreshUserInfo(senderID, appID, currentUID, logPrefix string) {
	if ip.userCache == nil {
		return
	}
	go func(sid, aID, curUID string) {
		defer func() { recover() }()
		info, err := ip.userCache.GetOrFetch(sid)
		if err != nil || info == nil {
			return
		}
		ip.routes.UpdateUserInfo(sid, info.Name, info.Email, info.Department)
		// 纠偏：如果策略匹配到不同上游，原子迁移
		if ip.policyEng != nil {
			if pUID, ok := ip.policyEng.Match(info, aID); ok && pUID != "" && ip.pool.IsHealthy(pUID) && pUID != curUID {
				if AtomicMigrate(ip.routes, ip.pool, sid, aID, curUID, pUID) {
					log.Printf("%s 策略纠偏 sender=%s app=%s: %s -> %s (dept=%s email=%s)",
						logPrefix, sid, aID, curUID, pUID, info.Department, info.Email)
				}
			}
		}
	}(senderID, appID, currentUID)
}

// asyncPolicyCorrection 异步策略纠偏：负载均衡分配后，后台获取用户信息并纠偏
func (ip *InboundProxy) asyncPolicyCorrection(senderID, appID, assignedUID, logPrefix string) {
	if ip.userCache == nil || ip.policyEng == nil {
		return
	}
	go func(sid, aID, curUID string) {
		defer func() { recover() }()
		info, err := ip.userCache.GetOrFetch(sid)
		if err != nil || info == nil {
			return
		}
		ip.routes.UpdateUserInfo(sid, info.Name, info.Email, info.Department)
		if pUID, ok := ip.policyEng.Match(info, aID); ok && pUID != "" && ip.pool.IsHealthy(pUID) && pUID != curUID {
			if AtomicMigrate(ip.routes, ip.pool, sid, aID, curUID, pUID) {
				log.Printf("%s 异步策略纠偏 sender=%s app=%s: %s -> %s (dept=%s email=%s)",
					logPrefix, sid, aID, curUID, pUID, info.Department, info.Email)
			}
		}
	}(senderID, appID, assignedUID)
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

	// 路由决策（统一方法，修复 Race #1/#2/#3/#5）
	var upstreamID string
	if senderID != "" {
		upstreamID = ip.resolveUpstream(senderID, appID, "[路由]")
	}

	// v34.0: 固定返回内容 — 命中策略且配置了 fixed_response 时直接短路
	if ip.policyEng != nil {
		var matchedPolicy *RoutePolicyConfig
		if ip.userCache != nil {
			info, _ := ip.userCache.GetOrFetchWithTimeout(senderID, 500*time.Millisecond)
			matchedPolicy, _ = ip.policyEng.MatchFull(info, appID)
		} else {
			matchedPolicy, _ = ip.policyEng.MatchFull(nil, appID)
		}
		if matchedPolicy != nil && matchedPolicy.FixedResponse != nil && matchedPolicy.FixedResponse.Enabled {
			fr := matchedPolicy.FixedResponse
			for k, v := range fr.Headers {
				w.Header().Set(k, v)
			}
			ct := fr.ContentType
			if ct == "" {
				ct = "application/json"
			}
			w.Header().Set("Content-Type", ct)
			if traceID != "" {
				w.Header().Set("X-Trace-ID", traceID)
			}
			w.Header().Set("X-Lobster-Guard", "fixed-response")
			sc := fr.StatusCode
			if sc == 0 {
				sc = 200
			}
			w.WriteHeader(sc)
			w.Write([]byte(fr.Body))
			// 审计日志
			if ip.logger != nil {
				ip.logger.Log("inbound", senderID, "fixed_response",
					fmt.Sprintf("status=%d content_type=%s", sc, ct),
					truncate(msgText, 200), "", 0, "fixed_response", appID)
			}
			if ip.metrics != nil {
				latMs := float64(time.Since(start).Microseconds()) / 1000.0
				ip.metrics.RecordRequest("inbound", "fixed_response", ip.channel.Name(), latMs)
			}
			log.Printf("[路由] 🎯 固定返回 sender=%s app=%s status=%d", senderID, appID, sc)
			// v34.0: IM 主动回复 — 蓝信场景下将 body 作为消息推送给发送者
			if fr.Body != "" && senderID != "" && ip.cfg != nil && ip.cfg.LanxinAppID != "" {
				go ip.sendLanxinFixedReply(senderID, fr.Body)
			}
			return
		}
	}

	// 获取代理
	var proxy *httputil.ReverseProxy
	if upstreamID != "" {
		proxy = ip.pool.GetProxy(upstreamID)
	}
	// D-002: 当 GetProxy 返回 nil 需要降级时，更新路由表和计数
	if proxy == nil {
		var fallbackUID string
		proxy, fallbackUID = ip.pool.GetAnyHealthyProxy()
		if fallbackUID != "" && fallbackUID != upstreamID && senderID != "" {
			log.Printf("[路由] ⚠️ 上游 %s proxy不可用，降级到 %s sender=%s", upstreamID, fallbackUID, senderID)
			if upstreamID != "" {
				ip.routes.Bind(senderID, appID, fallbackUID)
				ip.pool.TransferUserCount(upstreamID, fallbackUID)
			}
			upstreamID = fallbackUID // 确保审计日志记录实际上游
		}
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
			// v23.0: 同步污染标签到路径上下文（驱动 cumulative 规则）
			if ip.pathPolicyEngine != nil {
				for _, label := range taintEntry.Labels {
					ip.pathPolicyEngine.AddTaintLabel(traceID, label)
					ip.pathPolicyEngine.RegisterStep(traceID, PathStep{
						Stage: "taint", Action: taintActionForLabel(label), Details: label,
					})
				}
			}
		}
	}

	// v25.0: 编译执行计划 — 从用户 query 提取意图并生成允许的执行模板
	if ip.planCompiler != nil && msgText != "" {
		plan := ip.planCompiler.CompileIntent(traceID, msgText)
		if plan != nil {
			log.Printf("[入站] 📋 执行计划已编译 trace=%s plan=%s steps=%d", traceID, plan.ID, plan.TotalSteps)
		}
	}

	// v25.1: 初始化 Capability 上下文 — 用户输入拥有完整权限
	if ip.capabilityEngine != nil && senderID != "" {
		userCaps := []CapLabel{
			{Name: "read", Source: "user_input", Level: "read", Granted: true},
			{Name: "write", Source: "user_input", Level: "write", Granted: true},
			{Name: "execute", Source: "user_input", Level: "execute", Granted: true},
		}
		ctx := ip.capabilityEngine.InitContext(traceID, senderID, userCaps)
		if ctx != nil {
			log.Printf("[入站] 🔑 Capability 上下文已初始化 trace=%s user=%s caps=%d", traceID, senderID, len(userCaps))
		}
	}

	// v26.0: IFC 入站标签 — 注册用户输入变量
	if ip.ifcEngine != nil && ip.ifcEngine.config.Enabled {
		v := ip.ifcEngine.RegisterVariable(traceID, "user_input", "user_input", msgText)
		if v != nil {
			log.Printf("[入站] 🏷️ IFC 变量已注册 trace=%s name=user_input conf=%s integ=%s", traceID, v.Label.Confidentiality, v.Label.Integrity)
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
		// v18.3: 奇点蜜罐暴露 — block 时注入蜜罐内容
		if ip.singularityEngine != nil {
			if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", traceID); shouldExpose && tpl != nil {
				ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, traceID)
				if ip.envelopeMgr != nil {
					ip.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_im_" + tpl.Name}, senderID)
				}
				log.Printf("[入站] 🔮 奇点暴露 sender=%s template=%s level=%d trace_id=%s", senderID, tpl.Name, tpl.Level, traceID)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Trace-ID", traceID)
				w.WriteHeader(200)
				w.Write([]byte(fmt.Sprintf(`{"errcode":0,"errmsg":"ok","singularity_response":%q}`, tpl.Content)))
				return
			}
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
					TenantID:      ip.resolveTenantID(senderID, appID),
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
				// v31.0: 蜜罐→攻击链事件发布
				if ip.eventBus != nil {
					ip.eventBus.Emit(&SecurityEvent{
						Type:     "honeypot_trigger",
						Severity: "high",
						SenderID: senderID,
						Summary:  fmt.Sprintf("蜜罐触发: template=%s watermark=%s trigger_type=%s", tpl.Name, watermark, tpl.TriggerType),
						Details:  map[string]interface{}{"template": tpl.Name, "watermark": watermark, "trigger_type": tpl.TriggerType},
						Domain:   "inbound",
					})
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
		// v18.3: 奇点蜜罐暴露 — warn 时也可注入蜜罐内容（蜜罐未触发时）
		if ip.singularityEngine != nil {
			if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", traceID); shouldExpose && tpl != nil {
				ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, traceID)
				if ip.envelopeMgr != nil {
					ip.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_im_" + tpl.Name}, senderID)
				}
				log.Printf("[入站] 🔮 奇点暴露(warn) sender=%s template=%s level=%d trace_id=%s", senderID, tpl.Name, tpl.Level, traceID)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Trace-ID", traceID)
				w.WriteHeader(200)
				w.Write([]byte(fmt.Sprintf(`{"errcode":0,"errmsg":"ok","singularity_response":%q}`, tpl.Content)))
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
	// v27.0: 根据租户配置获取排除规则
	tenantID := ip.resolveTenantID(senderID, appID)
	var excludeRules []string
	if ip.tenantMgr != nil {
		cfg := ip.tenantMgr.GetConfig(tenantID)
		if cfg != nil && cfg.DisabledRules != "" {
			for _, r := range strings.Split(cfg.DisabledRules, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					excludeRules = append(excludeRules, r)
				}
			}
		}
	}

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
		// v27.0: 排除租户禁用的规则
		if len(excludeRules) > 0 {
			dr = filterExcludedRules(dr, excludeRules)
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
		// v30.0: 追加全局启用的行业模板规则检测
		globalTplResult := ip.engine.DetectGlobalTemplates(msgText)
		if globalTplResult.Action != "pass" {
			dr = mergeDetectResults(dr, globalTplResult)
		}
		// v27.1: 追加租户专属入站规则检测
		if tenantID != "" && tenantID != "default" {
			tenantResult := ip.engine.DetectTenantRules(tenantID, msgText)
			if tenantResult.Action != "pass" {
				dr = mergeDetectResults(dr, tenantResult)
			}
		}
		return dr
	}
	// 回退: 直接调用引擎（带排除）
	result := ip.engine.DetectWithExclusions(msgText, appID, excludeRules)
	// v30.0: 追加全局启用的行业模板规则检测
	globalTplResult := ip.engine.DetectGlobalTemplates(msgText)
	if globalTplResult.Action != "pass" {
		result = mergeDetectResults(result, globalTplResult)
	}
	// v27.1: 追加租户专属入站规则检测
	if tenantID != "" && tenantID != "default" {
		tenantResult := ip.engine.DetectTenantRules(tenantID, msgText)
		if tenantResult.Action != "pass" {
			result = mergeDetectResults(result, tenantResult)
		}
	}
	return result
}

// filterExcludedRules 从检测结果中移除被租户禁用的规则（v27.0）
func filterExcludedRules(dr DetectResult, excludeRules []string) DetectResult {
	if len(excludeRules) == 0 {
		return dr
	}
	excSet := make(map[string]bool, len(excludeRules))
	for _, r := range excludeRules {
		excSet[r] = true
	}
	var filteredRules []string
	var filteredReasons []string
	for _, r := range dr.MatchedRules {
		if !excSet[r] {
			filteredRules = append(filteredRules, r)
		}
	}
	for _, r := range dr.Reasons {
		if !excSet[r] {
			filteredReasons = append(filteredReasons, r)
		}
	}
	if len(filteredRules) == 0 {
		dr.Action = "pass"
		dr.Reasons = nil
		dr.MatchedRules = nil
		dr.Message = ""
		return dr
	}
	dr.MatchedRules = filteredRules
	dr.Reasons = filteredReasons
	return dr
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
	// v20.2 污染链逆转引擎
	reversalEngine *TaintReversalEngine
	// D-005: 出站代理路由表引用（路由感知）
	routes *RouteTable
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
			pv := text; if rs := []rune(pv); len(rs) > 2000 { pv = string(rs[:2000]) + "..." }
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
			pv := text; if rs := []rune(pv); len(rs) > 2000 { pv = string(rs[:2000]) + "..." }
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

	// D-005: 获取来源容器 ID — 优先 header，其次通过 traceCorrelator 反查入站 sender → 路由表查 upstream
	upstreamID := r.Header.Get("X-Upstream-Id")
	if upstreamID == "" && op.routes != nil && recipient != "" {
		if uid, ok := op.routes.Lookup(recipient, outAppID); ok {
			upstreamID = uid
		}
	}

	pv := text; if rs := []rune(pv); len(rs) > 2000 { pv = string(rs[:2000]) + "..." }

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


// taintActionForLabel 将污染标签映射到风险权重 action 名
func taintActionForLabel(label string) string {
	switch label {
	case "CREDENTIAL-TAINTED":
		return "credential_detected"
	default:
		return "pii_detected"
	}
}

// sendLanxinFixedReply v34.0: 蓝信固定返回 IM 主动回复
// 通过蓝信 API 将固定返回内容作为消息发送给 IM 发送者
func (ip *InboundProxy) sendLanxinFixedReply(senderID, text string) {
	cfg := ip.cfg
	upstream := cfg.LanxinUpstream
	if upstream == "" {
		upstream = "https://apigw.lx.qianxin.com"
	}

	// 1. 获取 app_token
	tokenURL := upstream + "/v1/apptoken/create?grant_type=client_credential&appid=" +
		url.QueryEscape(cfg.LanxinAppID) + "&secret=" + url.QueryEscape(cfg.LanxinAppSecret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		log.Printf("[固定返回] 获取蓝信 token 失败: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		ErrCode  int    `json:"errCode"`
		ErrMsg   string `json:"errMsg"`
		AppToken string `json:"appToken"`
	}
	if json.Unmarshal(body, &tokenResp) != nil || tokenResp.AppToken == "" {
		log.Printf("[固定返回] 蓝信 token 响应异常: %s", string(body))
		return
	}

	// 2. 发送消息
	msgPayload := map[string]interface{}{
		"userIdList": []string{senderID},
		"msgData": map[string]interface{}{
			"msgType": "text",
			"text": map[string]string{
				"content": text,
			},
		},
	}
	msgBody, _ := json.Marshal(msgPayload)
	msgURL := upstream + "/v1/bot/messages/create"
	req, _ := http.NewRequest("POST", msgURL, bytes.NewReader(msgBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenResp.AppToken)
	client := &http.Client{Timeout: 10 * time.Second}
	msgResp, err := client.Do(req)
	if err != nil {
		log.Printf("[固定返回] 发送蓝信消息失败: %v", err)
		return
	}
	defer msgResp.Body.Close()
	msgRespBody, _ := io.ReadAll(msgResp.Body)
	log.Printf("[固定返回] 蓝信回复完成 sender=%s status=%d resp=%s", senderID, msgResp.StatusCode, truncate(string(msgRespBody), 200))
}
