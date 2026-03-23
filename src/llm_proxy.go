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
	traceID := GenerateTraceID()

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

	// v10.0: 请求侧规则检测
	var llmReqDecision string
	var llmReqRules []string
	if lp.ruleEngine != nil && len(bodyBytes) > 0 {
		reqMatches := lp.ruleEngine.CheckRequest(string(bodyBytes))
		if len(reqMatches) > 0 {
			action, topMatch := HighestPriorityAction(reqMatches)
			llmReqDecision = action
			for _, m := range reqMatches {
				llmReqRules = append(llmReqRules, m.RuleName)
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

	// v14.0: 从请求头提取租户 ID（提前到缓存查找前）
	tenantID := r.Header.Get("X-Tenant-Id")
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

		// v10.0: 响应侧规则检测
		var llmRespDecision string
		var llmRespRules []string
		if lp.ruleEngine != nil && len(respBody) > 0 {
			respMatches := lp.ruleEngine.CheckResponse(string(respBody))
			if len(respMatches) > 0 {
				action, topMatch := HighestPriorityAction(respMatches)
				llmRespDecision = action
				for _, m := range respMatches {
					llmRespRules = append(llmRespRules, m.RuleName)
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
					tpEvent := lp.toolPolicy.Evaluate(tcName, tcArgs, traceID, tenantID)
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

	// v10.0: SSE 流式响应的规则检测（仅 log/warn，数据已推送给客户端）
	if lp.ruleEngine != nil && len(eventData) > 0 {
		go func() {
			respMatches := lp.ruleEngine.CheckResponse(string(eventData))
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
					tpEvent := lp.toolPolicy.Evaluate(tcName, tcArgs, auditCtx.TraceID, auditCtx.TenantID)
					if tpEvent.Decision == "block" || tpEvent.Decision == "warn" {
						log.Printf("[ToolPolicy] SSE 工具调用 %s: tool=%s rule=%s trace=%s (流式模式仅记录)",
							tpEvent.Decision, tcName, tpEvent.RuleHit, auditCtx.TraceID)
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
