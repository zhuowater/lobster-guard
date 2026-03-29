// llm_proxy.go — LLMProxy: LLM 侧透明反向代理（SSE streaming 支持）
// lobster-guard v10.0
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// LLMProxy — LLM 侧透明反向代理
type LLMProxy struct {
	cfg        LLMProxyConfig
	mainCfg    *Config // reference to main config for engine enabled checks
	auditor    *LLMAuditor
	ruleEngine *LLMRuleEngine
	httpServer *http.Server
	client     *http.Client
	// v10.1 Canary Token
	canaryMu    sync.RWMutex
	canaryToken string
	// v17.3 IM↔LLM 会话关联
	sessionCorrelator *SessionCorrelator
	// v18.0 执行信封
	envelopeMgr *EnvelopeManager
	// v18.1 事件总线
	eventBus *EventBus
	// v19.1 语义检测引擎
	semanticDetector *SemanticDetector
	// v20.0 工具策略引擎
	toolPolicy *ToolPolicyEngine
	// v23.0 路径级策略引擎
	pathPolicyEngine *PathPolicyEngine
	// v20.1 污染追踪引擎
	taintTracker *TaintTracker
	// v20.2 污染链逆转引擎
	reversalEngine *TaintReversalEngine
	// v20.3 LLM 响应缓存
	llmCache *LLMCache
	// v20.4 API Gateway
	apiGateway *APIGateway
	// v18.3 奇点蜜罐引擎
	singularityEngine *SingularityEngine
	// v24.0 反事实验证引擎
	cfVerifier *CounterfactualVerifier
	// v25.0 执行计划编译器
	planCompiler      *PlanCompiler
	capabilityEngine  *CapabilityEngine
	deviationDetector *DeviationDetector
	// v26.0 信息流控制引擎
	ifcEngine *IFCEngine
	// v26.1 隔离LLM
	ifcQuarantine *IFCQuarantine
	// v26.3 审计日志写入(治理引擎事件)
	auditLogger *AuditLogger
	// v27.0 API Key 身份管理
	apiKeyMgr *APIKeyManager
	tenantMgr *TenantManager
}

// NewLLMProxy 创建 LLM 代理
func NewLLMProxy(cfg LLMProxyConfig, auditor *LLMAuditor, ruleEngine *LLMRuleEngine) *LLMProxy {
	if cfg.Listen == "" {
		cfg.Listen = ":8445"
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 300
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 10 * 1024 * 1024 // 10MB
	}

	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		// 不自动解压 — 让客户端看到原始响应
		DisableCompression: true,
	}

	lp := &LLMProxy{
		cfg:        cfg,
		auditor:    auditor,
		ruleEngine: ruleEngine,
		client: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
			// 不跟随重定向
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	// v10.1: 初始化 Canary Token
	lp.initCanaryToken()

	lp.httpServer = &http.Server{
		Addr:         cfg.Listen,
		Handler:      lp,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: time.Duration(cfg.TimeoutSec+10) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return lp
}

// Start 启动 HTTP server
// SetAPIKeyManager 设置 API Key 管理器（v27.0）
func (lp *LLMProxy) SetAPIKeyManager(akm *APIKeyManager) { lp.apiKeyMgr = akm }

// SetTenantManager 设置租户管理器（v27.0）
func (lp *LLMProxy) SetTenantManager(tm *TenantManager) { lp.tenantMgr = tm }

func (lp *LLMProxy) Start() error {
	if err := lp.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop 优雅关闭
func (lp *LLMProxy) Stop() error {
	if lp.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lp.httpServer.Shutdown(ctx)
}

// matchTarget 根据请求路径匹配目标
func (lp *LLMProxy) matchTarget(r *http.Request) *LLMTargetConfig {
	if len(lp.cfg.Targets) == 0 {
		return nil
	}
	if len(lp.cfg.Targets) == 1 {
		return &lp.cfg.Targets[0]
	}
	// 按 path_prefix 匹配
	for i := range lp.cfg.Targets {
		t := &lp.cfg.Targets[i]
		if t.PathPrefix != "" && strings.HasPrefix(r.URL.Path, t.PathPrefix) {
			return t
		}
	}
	// 默认第一个
	return &lp.cfg.Targets[0]
}

// ServeHTTP 实现 http.Handler
func (lp *LLMProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// panic recovery
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[LLM代理] PANIC: %v\n%s", rv, debug.Stack())
			http.Error(w, "Internal Server Error", 500)
		}
	}()

	start := time.Now()
	// Prefer trace ID from upstream (InboundProxy sets X-Trace-Id),
	// fallback to generating a new one for direct LLM calls
	traceID := r.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = r.Header.Get("X-Trace-ID")
	}
	if traceID == "" {
		traceID = GenerateTraceID()
	}

	// 匹配上游
	target := lp.matchTarget(r)
	if target == nil {
		http.Error(w, `{"error":"no upstream target configured"}`, 502)
		return
	}

	// 读取请求体
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(io.LimitReader(r.Body, lp.cfg.MaxBodyBytes))
		r.Body.Close()
	}

	// 提取 model（用于审计上下文）
	model := ParseAnthropicRequest(bodyBytes)

	// v17.3: 尝试关联 IM session（通过内容指纹匹配）
	var sessionLink *SessionLink
	if lp.sessionCorrelator != nil && len(bodyBytes) > 0 {
		sessionLink = lp.sessionCorrelator.MatchLLMRequest(bodyBytes, traceID)
		if sessionLink != nil {
			logSessionLink(traceID, sessionLink)
		}
	}

	// v10.1: Canary Token 注入
	var activeCanaryToken string
	if lp.cfg.Security.CanaryToken.Enabled {
		bodyBytes, activeCanaryToken = lp.injectCanaryToken(bodyBytes)
	}

	// v28.0: 提前解析租户 ID（供规则引擎使用租户专属规则）
	earlyTenantID := r.Header.Get("X-Tenant-Id")
	if earlyTenantID == "" && lp.apiKeyMgr != nil {
		if authH := r.Header.Get("Authorization"); authH != "" && strings.HasPrefix(authH, "Bearer sk-") {
			if ke, err := lp.apiKeyMgr.Resolve(authH); err == nil {
				earlyTenantID = ke.TenantID
			}
		}
	}
	if earlyTenantID == "" {
		earlyTenantID = "default"
	}

	// v10.0: 请求侧规则检测（v28.0: 使用租户感知版本）
	var llmReqDecision string
	var llmReqRules []string
	if lp.ruleEngine != nil && len(bodyBytes) > 0 {
		reqMatches := lp.ruleEngine.CheckRequestWithTenant(string(bodyBytes), earlyTenantID)
		if len(reqMatches) > 0 {
			action, topMatch := HighestPriorityAction(reqMatches)
			llmReqDecision = action
			for _, m := range reqMatches {
				llmReqRules = append(llmReqRules, m.RuleName)
			}
			// v31.1: LLM auto-review — block 前检查是否需要 LLM 二审
			if action == "block" && lp.ruleEngine.autoReviewMgr != nil && len(llmReqRules) > 0 {
				allInReview := true
				for _, rule := range llmReqRules {
					lp.ruleEngine.autoReviewMgr.RecordBlock(rule)
					if !lp.ruleEngine.autoReviewMgr.IsInReview(rule) {
						allInReview = false
					}
				}
				if allInReview {
					action = lp.ruleEngine.autoReviewMgr.ReviewWithLLM(llmReqRules[0], string(bodyBytes))
					llmReqDecision = action
					log.Printf("[LLM规则] auto-review: %s → %s rule=%s", "block", action, llmReqRules[0])
				}
			}
			switch action {
			case "block":
				log.Printf("[LLM规则] 请求被阻断: rule=%s category=%s pattern=%q",
					topMatch.RuleID, topMatch.Category, topMatch.Pattern)
				// v18.0: 执行信封（block 也要记录）
				if lp.envelopeMgr != nil {
					lp.envelopeMgr.Seal(traceID, "llm_request", string(bodyBytes), "block", llmReqRules, "")
				}
				// v18.1: 事件总线
				if lp.eventBus != nil {
					lp.eventBus.Emit(&SecurityEvent{
						Type: "llm_block", Severity: "high", Domain: "llm",
						TraceID: traceID,
						Summary: fmt.Sprintf("LLM 请求阻断: %s (%s)", topMatch.RuleName, topMatch.Category),
						Details: map[string]interface{}{"rule_id": topMatch.RuleID, "category": topMatch.Category, "rules": llmReqRules},
					})
				}
				// v18.3: 奇点蜜罐暴露 — LLM block 时注入蜜罐内容
				if lp.singularityEngine != nil {
					if shouldExpose, tpl := lp.singularityEngine.ShouldExpose("llm", traceID); shouldExpose && tpl != nil {
						if lp.auditor != nil {
							lp.auditor.LogSingularityExpose(traceID, "llm", tpl.Name, tpl.Level)
						}
						if lp.envelopeMgr != nil {
							lp.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_llm_" + tpl.Name}, "")
						}
						log.Printf("[LLM代理] 🔮 奇点暴露 template=%s level=%d trace_id=%s", tpl.Name, tpl.Level, traceID)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(200)
						fmt.Fprintf(w, `%s`, tpl.Content)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(403)
				fmt.Fprintf(w, `{"error":"Request blocked by LLM security rule: %s","rule_id":"%s","category":"%s"}`,
					topMatch.RuleName, topMatch.RuleID, topMatch.Category)
				return
			case "warn":
				log.Printf("[LLM规则] 请求告警: rule=%s category=%s pattern=%q",
					topMatch.RuleID, topMatch.Category, topMatch.Pattern)
				// v18.1: 事件总线
				if lp.eventBus != nil {
					lp.eventBus.Emit(&SecurityEvent{
						Type: "llm_block", Severity: "medium", Domain: "llm",
						TraceID: traceID,
						Summary: fmt.Sprintf("LLM 请求告警: %s (%s)", topMatch.RuleName, topMatch.Category),
						Details: map[string]interface{}{"rule_id": topMatch.RuleID, "category": topMatch.Category, "action": "warn"},
					})
				}
				// v18.3: 奇点蜜罐暴露 — LLM warn 时注入蜜罐内容
				if lp.singularityEngine != nil {
					if shouldExpose, tpl := lp.singularityEngine.ShouldExpose("llm", traceID); shouldExpose && tpl != nil {
						if lp.auditor != nil {
							lp.auditor.LogSingularityExpose(traceID, "llm", tpl.Name, tpl.Level)
						}
						if lp.envelopeMgr != nil {
							lp.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_llm_" + tpl.Name}, "")
						}
						log.Printf("[LLM代理] 🔮 奇点暴露(warn) template=%s level=%d trace_id=%s", tpl.Name, tpl.Level, traceID)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(200)
						fmt.Fprintf(w, `%s`, tpl.Content)
						return
					}
				}
			case "log":
				log.Printf("[LLM规则] 请求日志: rule=%s category=%s",
					topMatch.RuleID, topMatch.Category)
			}
		}
	}
	// v20.1: LLM 请求侧污染传播（使用关联的 IM trace_id 以匹配入站标记）
	taintTraceID := traceID
	if sessionLink != nil && sessionLink.IMTraceID != "" {
		taintTraceID = sessionLink.IMTraceID
	}
	if lp.taintTracker != nil {
		lp.taintTracker.Propagate(taintTraceID, "llm_request",
			fmt.Sprintf("user message forwarded to LLM (llm_trace=%s)", traceID))
	}

	// v18.0: 执行信封 — 请求侧（非 block 的也要记录）
	if lp.envelopeMgr != nil {
		decision := llmReqDecision
		if decision == "" {
			decision = "pass"
		}
		lp.envelopeMgr.Seal(traceID, "llm_request", string(bodyBytes), decision, llmReqRules, "")
	}

	// v14.0 + v27.0: 从请求头或 API Key 提取租户 ID
	tenantID := r.Header.Get("X-Tenant-Id")
	// v27.0: 尝试从 Authorization header 的 API Key 解析身份
	if lp.apiKeyMgr != nil {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer sk-") {
			if keyEntry, err := lp.apiKeyMgr.Resolve(authHeader); err == nil {
				if tenantID == "" {
					tenantID = keyEntry.TenantID
				}
				// 填充 sender_id（如果原来为空）
				if sessionLink == nil || sessionLink.SenderID == "" {
					r.Header.Set("X-Sender-Id", keyEntry.UserID)
				}
				// pending 状态处理：允许通行但标记 header
				if keyEntry.Status == "pending" {
					w.Header().Set("X-API-Key-Status", "pending")
					log.Printf("[LLM代理] API Key 待绑定: prefix=%s (允许通行)", keyEntry.KeyPrefix)
				}
				// 配额检查（pending key 的 quota 默认为 0，不限）
				if !lp.apiKeyMgr.CheckQuota(keyEntry.ID) {
					log.Printf("[LLM代理] API Key 配额已用完: user=%s key_prefix=%s", keyEntry.UserID, keyEntry.KeyPrefix)
					http.Error(w, `{"error":"API key daily quota exceeded"}`, 429)
					return
				}
				lp.apiKeyMgr.IncrUsage(keyEntry.ID)
			}
		}
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// v20.3: LLM 响应缓存 — 请求侧查找
	var cacheQuery string
	if lp.llmCache != nil && lp.llmCache.config.Enabled {
		cacheQuery = extractUserQuery(bodyBytes)
		if cacheQuery != "" {
			if cachedEntry, hit := lp.llmCache.Lookup(cacheQuery, model, tenantID); hit {
				log.Printf("[LLMCache] 缓存命中: query_hash=%s tenant=%s hits=%d", cachedEntry.QueryHash[:8], tenantID, cachedEntry.HitCount)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("X-Cache-Key", cachedEntry.Key)
				w.WriteHeader(200)
				w.Write([]byte(cachedEntry.Response))
				return
			}
		}
	}

	// 构建上游请求（strip_prefix 模式下去掉 path_prefix 再转发）
	requestPath := r.URL.RequestURI()
	if target.StripPrefix && target.PathPrefix != "" && strings.HasPrefix(requestPath, target.PathPrefix) {
		stripped := strings.TrimPrefix(requestPath, target.PathPrefix)
		if !strings.HasPrefix(stripped, "/") {
			stripped = "/" + stripped
		}
		requestPath = stripped
	}

	// v26.2: PII 隐藏 — 向上游发送前替换高机密字段
	if lp.ifcEngine != nil && lp.ifcEngine.config.HidingEnabled {
		hideResult := lp.ifcEngine.HideContent(traceID, string(bodyBytes), lp.ifcEngine.config.HidingThreshold)
		if hideResult != nil && hideResult.HiddenCount > 0 {
			log.Printf("[IFC-Hiding] 隐藏了 %d 个字段 trace=%s", hideResult.HiddenCount, traceID)
			bodyBytes = []byte(hideResult.Redacted)
		}
	}

	// v28.0i: Fides Selective Hide — scan tool messages in request, hide content that would raise context label
	if lp.ifcEngine != nil && lp.ifcEngine.config.Enabled {
		bodyBytes = lp.applySelectiveHide(traceID, bodyBytes)
	}

	upstreamURL := strings.TrimRight(target.Upstream, "/") + requestPath
	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[LLM代理] 创建上游请求失败: %v", err)
		http.Error(w, `{"error":"failed to create upstream request"}`, 500)
		return
	}

	// 复制 headers
	for key, values := range r.Header {
		for _, v := range values {
			upReq.Header.Add(key, v)
		}
	}
	upReq.Header.Set("X-Trace-ID", traceID)
	upReq.ContentLength = int64(len(bodyBytes))

	// 发送上游请求
	resp, err := lp.client.Do(upReq)
	if err != nil {
		log.Printf("[LLM代理] 上游请求失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"upstream request failed: %v"}`, err), 502)
		return
	}
	defer resp.Body.Close()

	// 审计上下文
	auditCtx := &LLMAuditContext{
		TraceID:      traceID,
		StartTime:    start,
		Model:        model,
		ReqBody:      bodyBytes,
		CanaryToken:  activeCanaryToken,
		TenantID:     tenantID,
	}

	// v17.3: 填充 IM 会话关联信息
	if sessionLink != nil {
		auditCtx.IMTraceID = sessionLink.IMTraceID
		auditCtx.SenderID = sessionLink.SenderID
		auditCtx.SessionID = sessionLink.SessionID
	}

	// 复制响应 headers
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	// 检测是否是 SSE 流式响应
	contentType := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(contentType, "text/event-stream")

	if isSSE {
		// SSE 流式处理
		w.WriteHeader(resp.StatusCode)
		lp.handleSSEResponse(w, resp, auditCtx, taintTraceID)
	} else {
		// 非流式：读取完整响应
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[LLM代理] 读取上游响应失败: %v", err)
			w.WriteHeader(502)
			return
		}

		// v10.0: 响应侧规则检测（v28.0: 使用租户感知版本）
		var llmRespDecision string
		var llmRespRules []string
		if lp.ruleEngine != nil && len(respBody) > 0 {
			respMatches := lp.ruleEngine.CheckResponseWithTenant(string(respBody), auditCtx.TenantID)
			if len(respMatches) > 0 {
				action, topMatch := HighestPriorityAction(respMatches)
				llmRespDecision = action
				for _, m := range respMatches {
					llmRespRules = append(llmRespRules, m.RuleName)
				}
				// v31.1: LLM 响应 auto-review
				if action == "block" && lp.ruleEngine.autoReviewMgr != nil && len(llmRespRules) > 0 {
					allInReview := true
					for _, rule := range llmRespRules {
						lp.ruleEngine.autoReviewMgr.RecordBlock(rule)
						if !lp.ruleEngine.autoReviewMgr.IsInReview(rule) {
							allInReview = false
						}
					}
					if allInReview {
						action = lp.ruleEngine.autoReviewMgr.ReviewWithLLM(llmRespRules[0], string(respBody))
						llmRespDecision = action
						log.Printf("[LLM规则] 响应 auto-review: %s → %s rule=%s", "block", action, llmRespRules[0])
					}
				}
				switch action {
				case "block":
					log.Printf("[LLM规则] 响应被阻断: rule=%s category=%s",
						topMatch.RuleID, topMatch.Category)
					// v18.0: 执行信封
					if lp.envelopeMgr != nil {
						lp.envelopeMgr.Seal(traceID, "llm_response", string(respBody), "block", llmRespRules, "")
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(403)
					fmt.Fprintf(w, `{"error":"Response blocked by LLM security rule: %s","rule_id":"%s","category":"%s"}`,
						topMatch.RuleName, topMatch.RuleID, topMatch.Category)
					go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
					return
				case "rewrite":
					newBody := lp.ruleEngine.ApplyRewrite(string(respBody), respMatches)
					respBody = []byte(newBody)
					// 更新 Content-Length
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
					log.Printf("[LLM规则] 响应已改写: rule=%s category=%s",
						topMatch.RuleID, topMatch.Category)
				case "warn":
					log.Printf("[LLM规则] 响应告警: rule=%s category=%s",
						topMatch.RuleID, topMatch.Category)
				case "log":
					log.Printf("[LLM规则] 响应日志: rule=%s category=%s",
						topMatch.RuleID, topMatch.Category)
				}
			}
		}
		// v20.0: 工具策略引擎 — 非流式响应中的 tool_calls 检测
		if lp.toolPolicy != nil && lp.toolPolicy.config.Enabled {
			info := ParseAnthropicResponse(respBody)
			if info != nil && info.HasToolUse {
				for i, tcName := range info.ToolNames {
					tcArgs := ""
					if i < len(info.ToolInputs) {
						tcArgs = info.ToolInputs[i]
					}
					// v27.0: 租户工具黑名单检查
					if lp.tenantMgr != nil {
						tcfg := lp.tenantMgr.GetConfig(tenantID)
						if tcfg != nil && isToolBlacklisted(tcName, tcfg.ToolBlacklist) {
							log.Printf("[ToolPolicy] 租户黑名单拦截: tool=%s tenant=%s trace=%s", tcName, tenantID, traceID)
							continue
						}
					}
					tpEvent := lp.toolPolicy.Evaluate(tcName, tcArgs, traceID, tenantID)
					// v23.0: 路径策略引擎 — 注册 tool_call 步骤并评估
					if lp.pathPolicyEngine != nil && lp.isEngineEnabled("path_policy") {
						lp.pathPolicyEngine.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: tcName, Details: tcArgs})
						ppDec := lp.pathPolicyEngine.Evaluate(traceID, tcName)
						if actionSev(ppDec.Decision) > actionSev(tpEvent.Decision) {
							tpEvent.Decision = ppDec.Decision
							tpEvent.RuleHit = ppDec.RuleName
							tpEvent.RiskLevel = "high"
						}
					}
					if tpEvent.Decision == "block" {
						log.Printf("[ToolPolicy] 工具调用被阻断: tool=%s rule=%s trace=%s", tcName, tpEvent.RuleHit, traceID)
						if lp.eventBus != nil {
							lp.eventBus.Emit(&SecurityEvent{
								Type: "tool_block", Severity: "high", Domain: "llm",
								TraceID: traceID,
								Summary: fmt.Sprintf("工具调用阻断: %s (%s)", tcName, tpEvent.RuleHit),
								Details: map[string]interface{}{"tool_name": tcName, "rule_hit": tpEvent.RuleHit, "risk_level": tpEvent.RiskLevel},
							})
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(403)
						fmt.Fprintf(w, `{"error":"Tool call blocked by policy: %s","tool":"%s","rule":"%s"}`,
							tpEvent.RuleHit, tcName, tpEvent.RuleHit)
						go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
						return
					} else if tpEvent.Decision == "warn" {
						log.Printf("[ToolPolicy] 工具调用告警: tool=%s rule=%s trace=%s", tcName, tpEvent.RuleHit, traceID)
					}
					// v24.0: 反事实验证 — 在 tool_call 被 ToolPolicy 评估后
					if lp.cfVerifier != nil && lp.isEngineEnabled("counterfactual") && lp.cfVerifier.ShouldVerify(tcName, tcArgs, traceID, tpEvent.RiskScoreNum()) {
						cfUpstream := upstreamURL
						cfAuth := r.Header.Get("Authorization")
						cfCfg := lp.cfVerifier.GetConfig()
						// v24.1: 传入 senderID 用于攻击者画像联动
						cfSenderID := auditCtx.SenderID
						if cfCfg.Mode == "sync" {
							cfResult := lp.cfVerifier.Verify(r.Context(), bodyBytes, tcName, tcArgs, cfUpstream, cfAuth, cfSenderID)
							if cfResult != nil && cfResult.Decision == "block" {
								log.Printf("[Counterfactual] 反事实验证阻断: tool=%s verdict=%s attribution=%.2f trace=%s",
									tcName, cfResult.Verdict, cfResult.AttributionScore, traceID)
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(403)
								fmt.Fprintf(w, `{"error":"Tool call blocked by counterfactual verification","tool":"%s","verdict":"%s","attribution_score":%.2f}`,
									tcName, cfResult.Verdict, cfResult.AttributionScore)
								go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
								return
							}
						} else {
							// async 模式: 后台验证，不阻塞
							go func(body []byte, tn, ta, url, auth, sid string) {
								lp.cfVerifier.Verify(context.Background(), body, tn, ta, url, auth, sid)
							}(bodyBytes, tcName, tcArgs, cfUpstream, cfAuth, cfSenderID)
						}
					}
				}
			}
		}

		// v25.0-v25.2: 执行计划验证 + Capability + 偏差检测
		if lp.toolPolicy != nil && lp.toolPolicy.config.Enabled {
			info25 := ParseAnthropicResponse(respBody)
			if info25 != nil && info25.HasToolUse {
				for i25, tcName25 := range info25.ToolNames {
					tcArgs25 := ""
					if i25 < len(info25.ToolInputs) { tcArgs25 = info25.ToolInputs[i25] }

					// v25.0: PlanCompiler — 比对 tool_call vs 计划模板
					if lp.planCompiler != nil && lp.isEngineEnabled("plan_compiler") {
						planEval := lp.planCompiler.EvaluateToolCall(traceID, tcName25, tcArgs25)
						if planEval != nil && planEval.Violation != nil {
							log.Printf("[PlanCompiler] 计划偏离: tool=%s violation=%s severity=%s decision=%s trace=%s",
								tcName25, planEval.Violation.Description, planEval.Violation.Severity, planEval.Decision, traceID)
							if planEval.Decision == "block" {
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(403)
								fmt.Fprintf(w, `{"error":"Tool call blocked by plan compiler","tool":"%s","violation":"%s"}`,
									tcName25, planEval.Violation.Description)
								go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
								return
							}
						}
					}

					// v25.1+v28.0g: CapabilityEngine — 数据级权限检查 + CaMeL source propagation
					if lp.capabilityEngine != nil && lp.isEngineEnabled("capability") {
						toolDataID := fmt.Sprintf("tool-%s-%d", tcName25, i25)
						lp.capabilityEngine.RegisterToolResult(traceID, tcName25, toolDataID)

						// CaMeL: propagate sources from prior tool results in this trace
						// If previous tool outputs feed into this tool, the lineage carries forward
						if i25 > 0 {
							var parentIDs []string
							for j := 0; j < i25; j++ {
								parentIDs = append(parentIDs, fmt.Sprintf("tool-%s-%d", info25.ToolNames[j], j))
							}
							lp.capabilityEngine.PropagateData(traceID, toolDataID, "tool:"+tcName25, parentIDs)
						}

						capEval := lp.capabilityEngine.EvaluateWithProvenance(traceID, toolDataID, "execute", tcName25)
						if capEval != nil && (capEval.Decision == "deny" || capEval.Decision == "warn") {
							log.Printf("[Capability] %s: tool=%s reason=%s trace=%s", capEval.Decision, tcName25, capEval.Reason, traceID)
							if capEval.Decision == "deny" && lp.eventBus != nil {
								lp.eventBus.Emit(&SecurityEvent{
									Type: "capability_deny", Severity: "high", Domain: "llm", TraceID: traceID,
									Summary: fmt.Sprintf("Capability denied: %s (%s)", tcName25, capEval.Reason),
								})
							}
							if capEval.Decision == "warn" && lp.eventBus != nil {
								lp.eventBus.Emit(&SecurityEvent{
									Type: "capability_warn", Severity: "medium", Domain: "llm", TraceID: traceID,
									Summary: fmt.Sprintf("Capability warn (untrusted lineage): %s (%s)", tcName25, capEval.Reason),
								})
							}
							// v26.3: 写入审计日志
							if lp.auditLogger != nil {
								lp.auditLogger.LogWithTrace("outbound", auditCtx.SenderID, capEval.Decision,
									fmt.Sprintf("[Capability] %s: %s", capEval.Decision, capEval.Reason),
									fmt.Sprintf("tool_call: %s", tcName25), "", 0, "", "", traceID)
							}
						}
					}

					// v25.2: DeviationDetector — 综合偏差检测
					if lp.deviationDetector != nil && lp.isEngineEnabled("deviation") {
						devResult := lp.deviationDetector.Detect(traceID, tcName25, tcArgs25)
						if devResult.HasDeviation {
							log.Printf("[Deviation] 检测到偏差: tool=%s type=%s severity=%s decision=%s trace=%s",
								tcName25, devResult.Deviation.Type, devResult.Deviation.Severity, devResult.Decision, traceID)
							// v26.3: 写入审计日志
							if lp.auditLogger != nil {
								devAction := "warn"
								if devResult.Decision == "block" {
									devAction = "block"
								}
								lp.auditLogger.LogWithTrace("outbound", auditCtx.SenderID, devAction,
									fmt.Sprintf("[Deviation] %s: %s", devResult.Deviation.Type, devResult.Reason),
									fmt.Sprintf("tool_call: %s", tcName25), "", 0, "", "", traceID)
							}
							// v28: 如果修复成功，替换工具名和参数
							if devResult.Repaired {
								if devResult.RepairedTool != "" {
									log.Printf("[Deviation] 自动修复: %s → %s trace=%s", tcName25, devResult.RepairedTool, traceID)
									tcName25 = devResult.RepairedTool
								}
								if devResult.RepairedArgs != "" {
									tcArgs25 = devResult.RepairedArgs
								}
								// 修复后继续执行，不 block
							} else if devResult.Decision == "block" {
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(403)
								fmt.Fprintf(w, `{"error":"Tool call blocked by deviation detector","tool":"%s","reason":"%s"}`,
									tcName25, devResult.Reason)
								go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
								return
							}
						}
					}

					// v26.0: IFC 信息流控制 — tool_call 安全检查
					if lp.ifcEngine != nil && lp.ifcEngine.config.Enabled {
						// 注册 tool result 变量
						toolSource := "tool:" + tcName25
						toolVar := lp.ifcEngine.RegisterVariable(traceID, "tool_result_"+tcName25, toolSource, tcArgs25)
						// 获取该 trace 的所有变量 ID 作为输入
						if toolVar != nil {
							allVars := lp.ifcEngine.GetVariables(traceID)
							var varIDs []string
							for _, v := range allVars {
								varIDs = append(varIDs, v.ID)
							}
							// 检查 tool call 是否违反 IFC 规则
							ifcDecision := lp.ifcEngine.CheckToolCall(traceID, tcName25, varIDs)
							if ifcDecision != nil && !ifcDecision.Allowed && ifcDecision.Decision == "block" {
								log.Printf("[IFC] 信息流违规阻断: tool=%s type=%s trace=%s", tcName25, ifcDecision.Violation.Type, traceID)
								// v26.3: 写入审计日志
								if lp.auditLogger != nil {
									lp.auditLogger.LogWithTrace("outbound", auditCtx.SenderID, "block",
										fmt.Sprintf("[IFC] %s violation: %s", ifcDecision.Violation.Type, ifcDecision.Reason),
										fmt.Sprintf("tool_call: %s", tcName25), "", 0, "", "", traceID)
								}
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(403)
								fmt.Fprintf(w, `{"error":"Tool call blocked by IFC: %s","tool":"%s","type":"%s"}`,
									ifcDecision.Reason, tcName25, ifcDecision.Violation.Type)
								go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
								return
							} else if ifcDecision != nil && ifcDecision.Decision == "warn" {
								log.Printf("[IFC] 信息流告警: tool=%s reason=%s trace=%s", tcName25, ifcDecision.Reason, traceID)
								// v26.3: 写入审计日志
								if lp.auditLogger != nil {
									lp.auditLogger.LogWithTrace("outbound", auditCtx.SenderID, "warn",
										fmt.Sprintf("[IFC] %s: %s", ifcDecision.Violation.Type, ifcDecision.Reason),
										fmt.Sprintf("tool_call: %s", tcName25), "", 0, "", "", traceID)
								}
							}

							// v26.1: IFC 隔离路由
							if lp.ifcQuarantine != nil && lp.ifcEngine.config.QuarantineEnabled {
								if lp.ifcQuarantine.ShouldRoute(traceID, varIDs) {
									quarantineURL, sessionID, qErr := lp.ifcQuarantine.Route(traceID, varIDs)
									if qErr == nil && quarantineURL != "" {
										log.Printf("[IFC-Quarantine] 被污染数据路由到隔离LLM: trace=%s upstream=%s session=%s", traceID, quarantineURL, sessionID)
									}
								}
							}

							// v26.2: DOE 数据过度暴露检测
							fields := extractFieldNames(tcArgs25)
							if len(fields) > 0 {
								doeResult := lp.ifcEngine.DetectDOE(traceID, tcName25, fields, nil)
								if doeResult != nil && doeResult.Severity == "critical" {
									log.Printf("[IFC-DOE] 严重数据过度暴露: tool=%s excess=%v trace=%s", tcName25, doeResult.ExcessFields, traceID)
								} else if doeResult != nil && doeResult.Severity == "warning" {
									log.Printf("[IFC-DOE] 数据过度暴露告警: tool=%s excess=%v trace=%s", tcName25, doeResult.ExcessFields, traceID)
								}
							}
						}
					}
				}
			}
		}

		// v20.1: LLM 响应侧污染传播（使用关联的 IM trace_id）
		if lp.taintTracker != nil {
			lp.taintTracker.Propagate(taintTraceID, "llm_response",
				fmt.Sprintf("LLM response received (llm_trace=%s)", traceID))
		}

		// v20.2: 污染链逆转 — 对被污染的 LLM 响应自动注入缓解提示
		if lp.reversalEngine != nil && len(respBody) > 0 {
			reversed, record := lp.reversalEngine.Reverse(taintTraceID, string(respBody))
			if record != nil {
				respBody = []byte(reversed)
				log.Printf("[LLM代理] 🔄 污染逆转 trace=%s taint_trace=%s mode=%s template=%s",
					traceID, taintTraceID, record.Mode, record.TemplateID)
				// 更新 Content-Length
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
			}
		}

		// v18.0: 执行信封 — 响应侧（非 block 的也要记录）
		if lp.envelopeMgr != nil {
			decision := llmRespDecision
			if decision == "" {
				decision = "pass"
			}
			lp.envelopeMgr.Seal(traceID, "llm_response", string(respBody), decision, llmRespRules, "")
		}

		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)

		// v20.3: LLM 响应缓存 — 存储（非流式，仅 200 成功响应）
		if lp.llmCache != nil && cacheQuery != "" && resp.StatusCode == 200 {
			// 判断是否被污染
			tainted := false
			if lp.taintTracker != nil {
				te := lp.taintTracker.GetTaint(traceID)
				if te != nil && len(te.Labels) > 0 {
					tainted = true
				}
			}
			go lp.llmCache.Store(cacheQuery, string(respBody), model, tenantID, tainted)
		}

		// 异步审计
		go lp.auditor.ProcessResponse(auditCtx, resp.StatusCode, respBody)
	}
}

// handleSSEResponse 处理 SSE 流式响应
func (lp *LLMProxy) handleSSEResponse(w http.ResponseWriter, resp *http.Response, auditCtx *LLMAuditContext, taintTraceID string) {
	flusher, hasFlusher := w.(http.Flusher)
	scanner := bufio.NewScanner(resp.Body)
	// 增大缓冲区以处理大的 SSE 事件
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var eventBuf bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()
		// 立刻转发给客户端
		fmt.Fprintf(w, "%s\n", line)
		if hasFlusher {
			flusher.Flush()
		}
		// 同时记录到审计缓冲
		eventBuf.WriteString(line + "\n")
	}

	// 流结束，异步解析完整的审计数据
	eventData := make([]byte, eventBuf.Len())
	copy(eventData, eventBuf.Bytes())

	// v10.0: SSE 流式响应的规则检测（v28.0: 使用租户感知版本，仅 log/warn，数据已推送给客户端）
	if lp.ruleEngine != nil && len(eventData) > 0 {
		sseTenantID := auditCtx.TenantID
		go func() {
			respMatches := lp.ruleEngine.CheckResponseWithTenant(string(eventData), sseTenantID)
			if len(respMatches) > 0 {
				action, topMatch := HighestPriorityAction(respMatches)
				if topMatch != nil {
					log.Printf("[LLM规则] SSE 响应检测: rule=%s category=%s action=%s (流式模式仅记录)",
						topMatch.RuleID, topMatch.Category, topMatch.Action)
				}
				// v18.0: 执行信封 — SSE 响应侧
				if lp.envelopeMgr != nil {
					var rules []string
					for _, m := range respMatches {
						rules = append(rules, m.RuleName)
					}
					lp.envelopeMgr.Seal(auditCtx.TraceID, "llm_response", string(eventData), action, rules, "")
				}
			} else if lp.envelopeMgr != nil {
				// 无规则匹配时也记录 pass 信封
				lp.envelopeMgr.Seal(auditCtx.TraceID, "llm_response", string(eventData), "pass", nil, "")
			}
		}()
	} else if lp.envelopeMgr != nil && len(eventData) > 0 {
		// 规则引擎不存在但信封管理器存在
		go func() {
			lp.envelopeMgr.Seal(auditCtx.TraceID, "llm_response", string(eventData), "pass", nil, "")
		}()
	}

	// v20.0: 工具策略引擎 — SSE 流式响应中的 tool_calls 评估（异步，仅 log/warn）
	if lp.toolPolicy != nil && lp.toolPolicy.config.Enabled {
		go func() {
			defer func() { recover() }()
			info := ParseSSEEvents(eventData)
			if info != nil && info.HasToolUse {
				for i, tcName := range info.ToolNames {
					tcArgs := ""
					if i < len(info.ToolInputs) {
						tcArgs = info.ToolInputs[i]
					}
					// v27.0: 租户工具黑名单检查（SSE）
					if lp.tenantMgr != nil {
						tcfg := lp.tenantMgr.GetConfig(auditCtx.TenantID)
						if tcfg != nil && isToolBlacklisted(tcName, tcfg.ToolBlacklist) {
							log.Printf("[ToolPolicy] SSE 租户黑名单拦截: tool=%s tenant=%s trace=%s", tcName, auditCtx.TenantID, auditCtx.TraceID)
							continue
						}
					}
					tpEvent := lp.toolPolicy.Evaluate(tcName, tcArgs, auditCtx.TraceID, auditCtx.TenantID)
					// v23.0: 路径策略引擎 — SSE 模式下也注册步骤
					if lp.pathPolicyEngine != nil {
						lp.pathPolicyEngine.RegisterStep(auditCtx.TraceID, PathStep{Stage: "tool_call", Action: tcName, Details: tcArgs})
						ppDec := lp.pathPolicyEngine.Evaluate(auditCtx.TraceID, tcName)
						if actionSev(ppDec.Decision) > actionSev(tpEvent.Decision) {
							tpEvent.Decision = ppDec.Decision
							tpEvent.RuleHit = ppDec.RuleName
						}
					}
					if tpEvent.Decision == "block" || tpEvent.Decision == "warn" {
						log.Printf("[ToolPolicy] SSE 工具调用 %s: tool=%s rule=%s trace=%s (流式模式仅记录)",
							tpEvent.Decision, tcName, tpEvent.RuleHit, auditCtx.TraceID)
					}
					// v25.0: PlanCompiler — SSE 模式下也评估 tool_call
					if lp.planCompiler != nil && lp.isEngineEnabled("plan_compiler") {
						planEval := lp.planCompiler.EvaluateToolCall(auditCtx.TraceID, tcName, tcArgs)
						if planEval != nil && planEval.Violation != nil {
							log.Printf("[PlanCompiler] SSE 计划偏离: tool=%s severity=%s trace=%s (流式仅记录)",
								tcName, planEval.Violation.Severity, auditCtx.TraceID)
						}
					}
					// v25.1+v28.0g: CapabilityEngine — SSE 模式下注册 tool 结果 + source propagation
					if lp.capabilityEngine != nil && lp.isEngineEnabled("capability") {
						sseDataID := fmt.Sprintf("sse-tool-%s-%d", tcName, i)
						lp.capabilityEngine.RegisterToolResult(auditCtx.TraceID, tcName, sseDataID)
						// CaMeL: propagate sources from prior tools in this SSE batch
						if i > 0 {
							var parentIDs []string
							for j := 0; j < i; j++ {
								parentIDs = append(parentIDs, fmt.Sprintf("sse-tool-%s-%d", info.ToolNames[j], j))
							}
							lp.capabilityEngine.PropagateData(auditCtx.TraceID, sseDataID, "tool:"+tcName, parentIDs)
						}
						capEval := lp.capabilityEngine.EvaluateWithProvenance(auditCtx.TraceID, sseDataID, "execute", tcName)
						if capEval != nil && (capEval.Decision == "deny" || capEval.Decision == "warn") {
							log.Printf("[Capability] SSE %s: tool=%s reason=%s trace=%s (流式仅记录)", capEval.Decision, tcName, capEval.Reason, auditCtx.TraceID)
						}
					}
					// v25.2: DeviationDetector — SSE 模式下检测偏差
					if lp.deviationDetector != nil && lp.isEngineEnabled("deviation") {
						devResult := lp.deviationDetector.Detect(auditCtx.TraceID, tcName, tcArgs)
						if devResult.HasDeviation {
							repairNote := ""
							if devResult.Repaired {
								repairNote = fmt.Sprintf(" (建议修复: %s→%s)", tcName, devResult.RepairedTool)
							}
							log.Printf("[Deviation] SSE 偏差: tool=%s severity=%s%s trace=%s (流式仅记录)",
								tcName, devResult.Deviation.Severity, repairNote, auditCtx.TraceID)
						}
					}
				}
			}
		}()
	}

	// v20.2: SSE 流式响应侧 taint 传播 + 逆转
	if lp.taintTracker != nil && taintTraceID != "" {
		lp.taintTracker.Propagate(taintTraceID, "llm_response",
			fmt.Sprintf("SSE stream completed (llm_trace=%s)", auditCtx.TraceID))
	}
	if lp.reversalEngine != nil && len(eventData) > 0 && taintTraceID != "" {
		originalStr := string(eventData)
		reversed, record := lp.reversalEngine.Reverse(taintTraceID, originalStr)
		if record != nil {
			// 提取追加的缓解提示（reversed = original + template content）
			reversalContent := strings.TrimPrefix(reversed, originalStr)
			if reversalContent == "" {
				// hard 模式下 reversed 替换了原始内容
				reversalContent = reversed
			}
			reversalContent = strings.TrimSpace(reversalContent)
			if reversalContent != "" {
				sseEvent := fmt.Sprintf("event: lobster_guard_taint_reversal\ndata: %s\n\n",
					strings.ReplaceAll(reversalContent, "\n", "\ndata: "))
				fmt.Fprint(w, sseEvent)
				if hasFlusher {
					flusher.Flush()
				}
			}
			log.Printf("[LLM代理] 🔄 SSE 污染逆转 trace=%s taint_trace=%s mode=%s template=%s",
				auditCtx.TraceID, taintTraceID, record.Mode, record.TemplateID)
		}
	}

	go lp.auditor.ProcessSSEBuffer(auditCtx, eventData)
}

// ============================================================
// v10.1 Canary Token — Prompt 泄露检测
// ============================================================

// generateCanaryToken 生成随机 Canary Token
func generateCanaryToken() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("<!-- LG-CANARY-%x -->", b)
}

// initCanaryToken 初始化 canary token（从配置读取或自动生成）
func (lp *LLMProxy) initCanaryToken() {
	cfg := &lp.cfg.Security.CanaryToken
	// 默认启用
	if cfg.AlertAction == "" {
		cfg.AlertAction = "warn"
	}
	if cfg.Token == "" {
		cfg.Token = generateCanaryToken()
		cfg.Enabled = true
		log.Printf("[Canary] 自动生成 Canary Token: %s", cfg.Token)
	}
	lp.canaryToken = cfg.Token
}

// GetCanaryToken 返回当前的 canary token（并发安全）
func (lp *LLMProxy) GetCanaryToken() string {
	lp.canaryMu.RLock()
	defer lp.canaryMu.RUnlock()
	return lp.canaryToken
}

// RotateCanaryToken 轮换 canary token（并发安全）
func (lp *LLMProxy) RotateCanaryToken() string {
	lp.canaryMu.Lock()
	defer lp.canaryMu.Unlock()
	newToken := generateCanaryToken()
	lp.canaryToken = newToken
	lp.cfg.Security.CanaryToken.Token = newToken
	log.Printf("[Canary] Token 已轮换: %s", newToken)
	return newToken
}

// injectCanaryToken 在请求 JSON 的 system prompt 末尾注入 canary token
func (lp *LLMProxy) injectCanaryToken(body []byte) ([]byte, string) {
	token := lp.GetCanaryToken()
	if token == "" {
		return body, ""
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body, ""
	}

	modified := false

	// Anthropic 格式: "system" 字段（string 或 array）
	if sys, ok := req["system"]; ok {
		switch v := sys.(type) {
		case string:
			req["system"] = v + "\n" + token
			modified = true
		case []interface{}:
			// system 是 content block 数组
			for i, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if t, _ := m["type"].(string); t == "text" {
						if text, ok := m["text"].(string); ok {
							m["text"] = text + "\n" + token
							v[i] = m
							modified = true
							break
						}
					}
				}
			}
			req["system"] = v
		}
	}

	// OpenAI 格式: messages 中 role=system 的内容
	if !modified {
		if msgs, ok := req["messages"].([]interface{}); ok {
			for i, msg := range msgs {
				if m, ok := msg.(map[string]interface{}); ok {
					if role, _ := m["role"].(string); role == "system" {
						if content, ok := m["content"].(string); ok {
							m["content"] = content + "\n" + token
							msgs[i] = m
							modified = true
							break
						}
					}
				}
			}
		}
	}

	if !modified {
		return body, ""
	}

	newBody, err := json.Marshal(req)
	if err != nil {
		return body, ""
	}
	return newBody, token
}

// checkCanaryLeak 检查响应中是否包含 canary token
func (lp *LLMProxy) checkCanaryLeak(responseBody string, canaryToken string) bool {
	if canaryToken == "" {
		return false
	}
	return strings.Contains(responseBody, canaryToken)
}

// isToolBlacklisted 检查工具是否在黑名单中（v27.0 租户策略闭环）
func isToolBlacklisted(toolName, blacklistCSV string) bool {
	if blacklistCSV == "" || toolName == "" {
		return false
	}
	for _, bl := range strings.Split(blacklistCSV, ",") {
		bl = strings.TrimSpace(bl)
		if bl != "" && bl == toolName {
			return true
		}
	}
	return false
}

// ============================================================
// Fides Selective Hide — scan messages for tool results, replace high-label content
// ============================================================
// Algorithm 7 HIDE: for each node in tool result, if label ⋢ context label,
// store in variable and replace with reference.

// isEngineEnabled checks if a v23-v26 engine is enabled via mainCfg
func (lp *LLMProxy) isEngineEnabled(engine string) bool {
	if lp.mainCfg == nil {
		return true // if no mainCfg reference, default to enabled
	}
	switch engine {
	case "path_policy":
		return lp.mainCfg.PathPolicy.Enabled
	case "counterfactual":
		return lp.mainCfg.Counterfactual.Enabled
	case "plan_compiler":
		return lp.mainCfg.PlanCompiler.Enabled
	case "capability":
		return lp.mainCfg.Capability.Enabled
	case "deviation":
		return lp.mainCfg.Deviation.Enabled
	case "ifc":
		return lp.mainCfg.IFC.Enabled
	default:
		return true
	}
}

func (lp *LLMProxy) applySelectiveHide(traceID string, body []byte) []byte {
	if lp.ifcEngine == nil {
		return body
	}

	// Parse messages array from request body
	var reqObj map[string]interface{}
	if err := json.Unmarshal(body, &reqObj); err != nil {
		return body
	}
	messages, ok := reqObj["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return body
	}

	// Compute current context label: join of all messages' labels seen so far
	// For simplicity, we track context as the join of all previous tool result labels
	contextLabel := IFCLabel{
		Confidentiality: lp.ifcEngine.config.DefaultConf,
		Integrity:       lp.ifcEngine.config.DefaultInteg,
	}

	// First pass: determine the "safe" context label from system+user messages
	// System and user messages are trusted (⊥ in Fides)
	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok { continue }
		role, _ := m["role"].(string)
		if role == "system" || role == "user" {
			// These don't raise context — they are trusted/public
			continue
		}
	}

	modified := false
	hiddenCount := 0

	// Second pass: check tool messages and selectively hide
	for i, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok { continue }
		role, _ := m["role"].(string)
		if role != "tool" { continue }

		content, _ := m["content"].(string)
		if content == "" { continue }

		// Infer tool name from context (previous assistant message's tool_call)
		toolName := lp.inferToolNameFromMessages(messages, i)
		if toolName == "" {
			toolName = "unknown_tool"
		}

		hideResult := lp.ifcEngine.SelectiveHide(traceID, toolName, content, contextLabel)
		if hideResult.Hidden {
			m["content"] = hideResult.Modified
			messages[i] = m
			hiddenCount++
			modified = true
		} else {
			// This tool result is safe — join its label into context
			if hideResult.OrigLabel.Confidentiality > contextLabel.Confidentiality {
				contextLabel.Confidentiality = hideResult.OrigLabel.Confidentiality
			}
			if hideResult.OrigLabel.Integrity < contextLabel.Integrity {
				contextLabel.Integrity = hideResult.OrigLabel.Integrity
			}
		}
	}

	if !modified {
		return body
	}

	log.Printf("[IFC:SelectiveHide] trace=%s hidden=%d tool results to preserve context label", traceID, hiddenCount)

	reqObj["messages"] = messages
	newBody, err := json.Marshal(reqObj)
	if err != nil {
		return body
	}
	return newBody
}

// inferToolNameFromMessages — look backward from a tool message to find the tool_call that produced it
func (lp *LLMProxy) inferToolNameFromMessages(messages []interface{}, toolMsgIdx int) string {
	// Tool message usually has a "tool_call_id" matching an assistant message's tool_call
	toolMsg, ok := messages[toolMsgIdx].(map[string]interface{})
	if !ok { return "" }
	toolCallID, _ := toolMsg["tool_call_id"].(string)

	// Walk backwards to find matching assistant tool_call
	for i := toolMsgIdx - 1; i >= 0; i-- {
		m, ok := messages[i].(map[string]interface{})
		if !ok { continue }
		role, _ := m["role"].(string)
		if role != "assistant" { continue }

		toolCalls, ok := m["tool_calls"].([]interface{})
		if !ok { continue }
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]interface{})
			if !ok { continue }
			id, _ := tcMap["id"].(string)
			if id == toolCallID || toolCallID == "" {
				fn, ok := tcMap["function"].(map[string]interface{})
				if ok {
					name, _ := fn["name"].(string)
					return name
				}
			}
		}
	}
	return ""
}
