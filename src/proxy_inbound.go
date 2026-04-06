// proxy_inbound.go — InboundProxy 入站处理子函数（从 ServeHTTP 拆分）
// P1 重构：保持行为不变，提升可读性和可测试性
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

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
