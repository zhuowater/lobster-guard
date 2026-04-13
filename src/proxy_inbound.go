// proxy_inbound.go — InboundProxy 入站处理子函数（从 ServeHTTP 拆分）
// P1 重构：保持行为不变，提升可读性和可测试性
package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// v37.0 人工确认动作 (action: confirm)
// ============================================================

// handleConfirmAction 处理 confirm 动作：ACK IM 平台，将消息挂起等待人工确认
func (ip *InboundProxy) handleConfirmAction(w http.ResponseWriter, senderID, appID, msgText, traceID, rh, upstreamID string, detectResult DetectResult, body []byte, reqPath string, latMs float64) {
	cfg := ip.cfg.HumanConfirm
	if ip.slog != nil {
		ip.slog.Warn("inbound", "等待确认", "sender_id", senderID, "rule", strings.Join(detectResult.MatchedRules, ","), "trace_id", traceID)
	} else {
		log.Printf("[确认] 挂起请求 sender=%s rules=%v trace_id=%s", senderID, detectResult.MatchedRules, traceID)
	}

	// 从规则配置中查找 per-rule 设置
	timeoutAction := ""
	defaultAction := ""
	ruleName := ""
	if len(detectResult.MatchedRules) > 0 && ip.engine != nil {
		ruleName = detectResult.MatchedRules[0]
		for _, rc := range ip.engine.GetRuleConfigs() {
			if rc.Name == ruleName {
				timeoutAction = rc.TimeoutAction
				defaultAction = rc.DefaultAction
				break
			}
		}
	}

	// 超时时长
	timeoutSec := cfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 60
	}

	// 构建待确认记录
	pc := &PendingConfirm{
		SenderID:      senderID,
		AppID:         appID,
		MsgText:       msgText,
		Body:          body,
		ReqPath:       reqPath,
		TraceID:       traceID,
		UpstreamID:    upstreamID,
		RuleName:      ruleName,
		TimeoutAction: timeoutAction,
		DefaultAction: defaultAction,
		ExpiresAt:     time.Now().Add(time.Duration(timeoutSec) * time.Second),
	}
	ip.confirmStore.Add(pc)

	// 发送确认提示
	confirmMsg := cfg.ConfirmMsg
	if confirmMsg == "" {
		confirmMsg = fmt.Sprintf("⚠️ 您的消息触发了安全规则（%s），请回复 Y 放行或 N 取消（%d 秒内有效）",
			ruleName, timeoutSec)
	}
	go ip.sendFixedReplyViaOutbound(senderID, confirmMsg)

	// 告警通知
	if ip.alertNotifier != nil {
		ip.alertNotifier.Notify("inbound", senderID, ruleName, msgText, appID)
	}

	// 事件总线
	if ip.eventBus != nil {
		ip.eventBus.Emit(&SecurityEvent{
			Type: "inbound_confirm", Severity: "medium", Domain: "inbound",
			TraceID: traceID, SenderID: senderID,
			Summary: fmt.Sprintf("人工确认等待: %s", strings.Join(detectResult.Reasons, "; ")),
			Details: map[string]interface{}{"rules": detectResult.MatchedRules, "app_id": appID, "timeout_sec": timeoutSec},
		})
	}

	// ACK IM 平台
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", traceID)
	w.WriteHeader(200)
	w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
}

// isConfirmKeyword 检查消息是否为确认关键词（精确匹配，忽略首尾空格）
func isConfirmKeyword(msg string, keywords []string) bool {
	trimmed := strings.TrimSpace(msg)
	for _, kw := range keywords {
		if trimmed == kw {
			return true
		}
	}
	return false
}

// handleConfirmReply 拦截待确认 senderID 的 Y/N 回复（ServeHTTP 早期拦截）
// 返回 true 表示已处理（消息被消费），false 表示继续正常流程
func (ip *InboundProxy) handleConfirmReply(w http.ResponseWriter, senderID, msgText, traceID string, start time.Time) bool {
	cfg := ip.cfg.HumanConfirm
	confirmKW := cfg.ConfirmKeywords
	if len(confirmKW) == 0 {
		confirmKW = []string{"Y", "y", "是", "继续"}
	}
	cancelKW := cfg.CancelKeywords
	if len(cancelKW) == 0 {
		cancelKW = []string{"N", "n", "否", "取消"}
	}

	isYes := isConfirmKeyword(msgText, confirmKW)
	isNo := isConfirmKeyword(msgText, cancelKW)

	if !isYes && !isNo {
		// 非 Y/N：检查 pending 的 default_action
		pc := ip.confirmStore.peekDefaultAction(senderID)
		if pc == "" {
			return false // 继续等待，当前消息正常处理
		}
		if pc == "confirm" {
			isYes = true
		} else if pc == "cancel" {
			isNo = true
		} else {
			return false // 继续等待
		}
	}

	pending := ip.confirmStore.Pop(senderID)
	if pending == nil {
		// 已被超时处理（onTimeout 先于本函数抢到锁），静默 ACK 避免 Y/N 被再次转发上游
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		return true
	}

	if isYes {
		log.Printf("[确认] ✅ 用户确认放行 sender=%s trace=%s rule=%s", senderID, pending.TraceID, pending.RuleName)
		msg := cfg.ConfirmedMsg
		if msg == "" {
			msg = "✅ 已放行，正在处理您的请求"
		}
		go ip.sendFixedReplyViaOutbound(senderID, msg)
		go ip.replayConfirmedRequest(pending)
	} else {
		log.Printf("[确认] 🚫 用户取消请求 sender=%s trace=%s rule=%s", senderID, pending.TraceID, pending.RuleName)
		msg := cfg.CancelledMsg
		if msg == "" {
			msg = "🚫 已取消，请求被拒绝"
		}
		go ip.sendFixedReplyViaOutbound(senderID, msg)
	}

	// ACK 当前 Y/N 消息（不转发给上游）
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", traceID)
	w.WriteHeader(200)
	w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	return true
}

// inboundParsedMsg 入站消息解析结果
type inboundParsedMsg struct {
	Text      string
	SenderID  string
	EventType string
	AppID     string
	DecryptOK bool
}

// parseInboundMessage 使用通道插件解析入站消息（解密+提取字段）
// 返回 isVerify=true 表示验证请求已在函数内直接响应
func (ip *InboundProxy) parseInboundMessage(w http.ResponseWriter, body []byte, r *http.Request) (msg inboundParsedMsg, isVerify bool) {
	defer func() {
		if rv := recover(); rv != nil {
			log.Printf("[入站] ParseInbound panic: %v", rv)
		}
	}()

	var parsed InboundMessage
	var err error
	if rap, ok := ip.channel.(RequestAwareParser); ok {
		parsed, err = rap.ParseInboundRequest(body, r)
	} else {
		parsed, err = ip.channel.ParseInbound(body)
	}
	if err != nil {
		log.Printf("[入站] 解析失败: %v，fail-open", err)
		return msg, false
	}

	// URL Verification 特殊处理（飞书等）
	if parsed.IsVerify && parsed.VerifyReply != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(parsed.VerifyReply)
		return msg, true
	}
	if parsed.EventType == "url_verification" && parsed.Raw != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(parsed.Raw)
		return msg, true
	}

	msg.Text = parsed.Text
	msg.SenderID = parsed.SenderID
	msg.EventType = parsed.EventType
	msg.AppID = parsed.AppID
	msg.DecryptOK = true
	return msg, false
}

// handleFixedResponse 处理策略路由固定返回（v34.0）
// 返回 true 表示已处理（短路），false 表示继续正常流程
func (ip *InboundProxy) handleFixedResponse(w http.ResponseWriter, senderID, appID, msgText, traceID string, start time.Time) bool {
	if ip.policyEng == nil {
		return false
	}

	var matchedPolicy *RoutePolicyConfig
	if ip.userCache != nil {
		info, _ := ip.userCache.GetOrFetchWithTimeout(senderID, 500*time.Millisecond)
		matchedPolicy, _ = ip.policyEng.MatchFull(info, appID)
	} else {
		matchedPolicy, _ = ip.policyEng.MatchFull(nil, appID)
	}

	if matchedPolicy == nil || matchedPolicy.FixedResponse == nil || !matchedPolicy.FixedResponse.Enabled {
		return false
	}

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
	// IM 主动回复
	if fr.Body != "" && senderID != "" && ip.cfg != nil {
		go ip.sendFixedReplyViaOutbound(senderID, fr.Body)
	}
	return true
}

// applyInboundEnrichment 入站安全上下文增强（污染标记/计划编译/Capability/IFC）
func (ip *InboundProxy) applyInboundEnrichment(traceID, senderID, appID, msgText string) {
	// v20.1: 入站污染标记
	if ip.taintTracker != nil {
		taintEntry := ip.taintTracker.MarkTainted(traceID, msgText, "inbound")
		if taintEntry != nil {
			log.Printf("[入站] 🏷️ 污染标记 sender=%s trace=%s labels=%v", senderID, traceID, taintEntry.Labels)
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

	// v25.0: 编译执行计划
	if ip.planCompiler != nil && msgText != "" {
		plan := ip.planCompiler.CompileIntent(traceID, msgText)
		if plan != nil {
			log.Printf("[入站] 📋 执行计划已编译 trace=%s plan=%s steps=%d", traceID, plan.ID, plan.TotalSteps)
		}
	}

	// v25.1: 初始化 Capability 上下文
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

	// v26.0: IFC 入站标签
	if ip.ifcEngine != nil && ip.ifcEngine.config.Enabled {
		v := ip.ifcEngine.RegisterVariable(traceID, "user_input", "user_input", msgText)
		if v != nil {
			log.Printf("[入站] 🏷️ IFC 变量已注册 trace=%s name=user_input conf=%s integ=%s", traceID, v.Label.Confidentiality, v.Label.Integrity)
		}
	}
}

// handleBlockAction 处理入站拦截决策（含奇点蜜罐暴露）
// 返回 true 表示已响应（短路）
func (ip *InboundProxy) handleBlockAction(w http.ResponseWriter, senderID, appID, msgText, traceID, rh, upstreamID string, detectResult DetectResult, reason string, latMs float64) bool {
	if ip.slog != nil {
		ip.slog.Warn("inbound", "请求拦截", "sender_id", senderID, "action", "block", "reason", reason, "trace_id", traceID)
	} else {
		log.Printf("[入站] 拦截 sender=%s reasons=%v trace_id=%s", senderID, detectResult.Reasons, traceID)
	}
	// 告警通知
	if ip.alertNotifier != nil {
		rule := strings.Join(detectResult.MatchedRules, ",")
		ip.alertNotifier.Notify("inbound", senderID, rule, msgText, appID)
	}
	// 事件总线
	if ip.eventBus != nil {
		ip.eventBus.Emit(&SecurityEvent{
			Type: "inbound_block", Severity: "high", Domain: "inbound",
			TraceID: traceID, SenderID: senderID,
			Summary: fmt.Sprintf("入站拦截: %s", strings.Join(detectResult.Reasons, "; ")),
			Details: map[string]interface{}{"rules": detectResult.MatchedRules, "app_id": appID},
		})
	}
	// 奇点蜜罐暴露
	if ip.singularityEngine != nil {
		if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", traceID); shouldExpose && tpl != nil {
			ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, traceID)
			if ip.envelopeMgr != nil {
				ip.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_im_" + tpl.Name}, senderID)
			}
			log.Printf("[入站] 🔮 奇点暴露 sender=%s template=%s level=%d trace_id=%s", senderID, tpl.Name, tpl.Level, traceID)
			// 蜜罐内容通过出站 IM 通道推送给用户，ACK 正常返回
			go ip.sendFixedReplyViaOutbound(senderID, tpl.Content)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Trace-ID", traceID)
			w.WriteHeader(200)
			w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
			return true
		}
	}
	code, respBody := ip.channel.BlockResponseWithMessage(detectResult.Message)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", traceID)
	w.WriteHeader(code)
	w.Write(respBody)
	return true
}

// handleWarnAction 处理入站告警决策（含蜜罐触发和奇点暴露）
// 返回 true 表示已响应（短路，蜜罐/奇点触发），false 表示继续转发
func (ip *InboundProxy) handleWarnAction(w http.ResponseWriter, senderID, appID, msgText, traceID, rh, upstreamID string, detectResult DetectResult, reason string, latMs float64) bool {
	if ip.slog != nil {
		ip.slog.Warn("inbound", "告警放行", "sender_id", senderID, "action", "warn", "reason", reason, "trace_id", traceID)
	} else {
		log.Printf("[入站] 告警放行 sender=%s reasons=%v trace_id=%s", senderID, detectResult.Reasons, traceID)
	}
	// 事件总线
	if ip.eventBus != nil {
		ip.eventBus.Emit(&SecurityEvent{
			Type: "inbound_block", Severity: "medium", Domain: "inbound",
			TraceID: traceID, SenderID: senderID,
			Summary: fmt.Sprintf("入站告警: %s", strings.Join(detectResult.Reasons, "; ")),
			Details: map[string]interface{}{"rules": detectResult.MatchedRules, "action": "warn", "app_id": appID},
		})
	}
	// 蜜罐触发检查
	if ip.honeypot != nil {
		tpl, watermark := ip.honeypot.ShouldTrigger(msgText, senderID, "")
		if tpl != nil {
			fakeResp := ip.honeypot.GenerateFakeResponse(tpl, watermark)
			ip.honeypot.RecordTrigger(&HoneypotTrigger{
				TenantID: ip.resolveTenantID(senderID, appID), SenderID: senderID,
				TemplateID: tpl.ID, TemplateName: tpl.Name, TriggerType: tpl.TriggerType,
				OriginalInput: msgText, FakeResponse: fakeResp, Watermark: watermark, TraceID: traceID,
			})
			ip.logger.LogWithTrace("inbound", senderID, "honeypot", "honeypot_triggered:"+tpl.Name, msgText, rh, latMs, upstreamID, appID, traceID)
			if ip.honeypotDeep != nil {
				ip.honeypotDeep.RecordInteraction(senderID, tpl.TriggerType, "im", msgText)
			}
			if ip.eventBus != nil {
				ip.eventBus.Emit(&SecurityEvent{
					Type: "honeypot_trigger", Severity: "high", SenderID: senderID,
					Summary: fmt.Sprintf("蜜罐触发: template=%s watermark=%s trigger_type=%s", tpl.Name, watermark, tpl.TriggerType),
					Details: map[string]interface{}{"template": tpl.Name, "watermark": watermark, "trigger_type": tpl.TriggerType},
					Domain:  "inbound",
				})
			}
			log.Printf("[入站] 🍯 蜜罐触发 sender=%s template=%s watermark=%s trace_id=%s", senderID, tpl.Name, watermark, traceID)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Trace-ID", traceID)
			w.WriteHeader(200)
			w.Write([]byte(fmt.Sprintf(`{"errcode":0,"errmsg":"ok","honeypot_response":%q}`, fakeResp)))
			return true
		}
	}
	// 奇点蜜罐暴露（warn 时也可注入）
	if ip.singularityEngine != nil {
		if shouldExpose, tpl := ip.singularityEngine.ShouldExpose("im", traceID); shouldExpose && tpl != nil {
			ip.logger.LogWithTrace("inbound", senderID, "singularity_expose", fmt.Sprintf("channel=im,level=%d,template=%s", tpl.Level, tpl.Name), msgText, rh, latMs, upstreamID, appID, traceID)
			if ip.envelopeMgr != nil {
				ip.envelopeMgr.Seal(traceID, "singularity_expose", tpl.Content, "expose", []string{"singularity_im_" + tpl.Name}, senderID)
			}
			log.Printf("[入站] 🔮 奇点暴露(warn) sender=%s template=%s level=%d trace_id=%s", senderID, tpl.Name, tpl.Level, traceID)
			// 蜜罐内容通过出站 IM 通道推送给用户，ACK 正常返回
			go ip.sendFixedReplyViaOutbound(senderID, tpl.Content)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Trace-ID", traceID)
			w.WriteHeader(200)
			w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
			return true
		}
	}
	return false
}

// recordInboundObservability 统一记录审计/指标/信封/实时监控/规则命中
func (ip *InboundProxy) recordInboundObservability(senderID, appID, traceID, msgText, rh, upstreamID string, act, reason string, detectResult DetectResult, start time.Time) {
	latMs := float64(time.Since(start).Microseconds()) / 1000.0

	ip.logger.LogWithTrace("inbound", senderID, act, reason, msgText, rh, latMs, upstreamID, appID, traceID)

	// 执行信封
	if ip.envelopeMgr != nil {
		ip.envelopeMgr.Seal(traceID, "inbound", msgText, act, detectResult.MatchedRules, senderID)
	}

	// 指标采集
	if ip.metrics != nil {
		ip.metrics.RecordRequest("inbound", act, ip.channel.Name(), latMs)
	}

	// 实时监控
	if ip.realtime != nil {
		ip.realtime.RecordInbound(act, time.Since(start).Microseconds())
		if act == "block" || act == "warn" {
			ip.realtime.RecordEvent("inbound", senderID, act, reason, traceID)
		}
	}

	// 规则命中统计
	if ip.ruleHits != nil && len(detectResult.MatchedRules) > 0 {
		for _, ruleName := range detectResult.MatchedRules {
			ip.ruleHits.Record(ruleName)
		}
	}
}

// ============================================================
// v37.0 确认重放辅助
// ============================================================

type discardResponseWriter struct {
	header http.Header
	code   int
}

func newDiscardResponseWriter() *discardResponseWriter {
	return &discardResponseWriter{header: make(http.Header), code: 200}
}
func (d *discardResponseWriter) Header() http.Header          { return d.header }
func (d *discardResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardResponseWriter) WriteHeader(code int)        { d.code = code }

// replayConfirmedRequest 将待确认请求异步重放到上游（用户确认 Y 后调用）
func (ip *InboundProxy) replayConfirmedRequest(pc *PendingConfirm) {
	upstreamID := pc.UpstreamID
	proxy := ip.pool.GetProxy(upstreamID)
	if proxy == nil {
		var fallback string
		proxy, fallback = ip.pool.GetAnyHealthyProxy()
		if proxy == nil {
			log.Printf("[确认] 重放失败：无可用上游 sender=%s trace=%s", pc.SenderID, pc.TraceID)
			return
		}
		upstreamID = fallback
	}

	path := pc.ReqPath
	if path == "" {
		path = "/"
	}
	req, err := http.NewRequest("POST", path, bytes.NewReader(pc.Body))
	if err != nil {
		log.Printf("[确认] 重放请求创建失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-ID", pc.TraceID+"-confirmed")
	req.Header.Set("X-Human-Confirmed", "true")
	req.ContentLength = int64(len(pc.Body))
	req.URL.Path = path

	dw := newDiscardResponseWriter()
	proxy.ServeHTTP(dw, req)
	log.Printf("[确认] ✅ 重放完成 sender=%s trace=%s upstream=%s status=%d",
		pc.SenderID, pc.TraceID, upstreamID, dw.code)
}
