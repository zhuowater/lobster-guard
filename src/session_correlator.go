// session_correlator.go — IM↔LLM 会话级 trace 关联（v17.3 hotfix）
//
// 解决的问题：
//   InboundProxy 和 LLMProxy 各自生成独立的 trace_id，导致
//   IM 入站、LLM 多轮调用、IM 出站之间的审计数据无法关联。
//
// 方案：
//   1. IM 入站时：提取用户消息内容指纹 → 注册 session（fingerprint→im_trace_id）
//   2. LLM 请求时：提取 messages 中首条 user content 指纹 → 匹配 session → 获得 im_trace_id
//   3. 同一 session 内的多轮 LLM 调用共享同一个 session_trace_id
//   4. 极短时间窗口（默认 5 分钟）+ 内容指纹双重匹配，减少误关联
//
// 数据流：
//   IM入站(trace=im-001, fingerprint=abc)
//     → SessionCorrelator.RegisterIMSession("abc", "im-001", senderID, appID)
//   LLM第1轮(trace=llm-001, 请求体含 user message fingerprint=abc)
//     → SessionCorrelator.MatchLLMRequest("abc", remoteAddr) → "im-001"
//   LLM第2轮(trace=llm-002, 请求体仍含首条 user message fingerprint=abc)
//     → SessionCorrelator.MatchLLMRequest("abc", remoteAddr) → "im-001"
//   IM出站(trace=im-001)
//     → 已有 TraceCorrelator 关联
//
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"
)

// SessionCorrelator 将 IM 入站消息与 LLM 请求通过内容指纹关联
type SessionCorrelator struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry // fingerprint → session
	maxSize  int
	windowMs int64 // 毫秒，匹配时间窗口
}

type sessionEntry struct {
	imTraceID  string
	senderID   string
	appID      string
	ts         time.Time
	llmTraces  []string // 关联到此 session 的所有 LLM trace_id
}

// SessionLink 表示一次关联结果
type SessionLink struct {
	IMTraceID string // IM 侧的 trace_id
	SenderID  string
	AppID     string
}

func NewSessionCorrelator(maxSize int, windowMs int64) *SessionCorrelator {
	if maxSize <= 0 {
		maxSize = 50000
	}
	if windowMs <= 0 {
		windowMs = 5 * 60 * 1000 // 默认 5 分钟
	}
	return &SessionCorrelator{
		sessions: make(map[string]*sessionEntry),
		maxSize:  maxSize,
		windowMs: windowMs,
	}
}

// RegisterIMSession IM 入站时注册会话指纹
// 调用方：InboundProxy.ServeHTTP，在检测到非空消息文本后调用
func (sc *SessionCorrelator) RegisterIMSession(msgText, imTraceID, senderID, appID string) {
	fp := contentFingerprint(msgText)
	if fp == "" {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.sessions[fp] = &sessionEntry{
		imTraceID: imTraceID,
		senderID:  senderID,
		appID:     appID,
		ts:        time.Now(),
		llmTraces: nil,
	}

	// 淘汰过期和超量
	sc.evictLocked()
}

// MatchLLMRequest LLM 请求时匹配会话
// 从 LLM 请求体的 messages 中提取首条 user content 指纹，与已注册的 IM 会话匹配
// 返回关联的 IM trace 信息，未匹配返回 nil
func (sc *SessionCorrelator) MatchLLMRequest(reqBody []byte, llmTraceID string) *SessionLink {
	fp := extractFirstUserFingerprint(reqBody)
	if fp == "" {
		return nil
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	entry, ok := sc.sessions[fp]
	if !ok {
		return nil
	}

	// 检查时间窗口
	if time.Since(entry.ts).Milliseconds() > sc.windowMs {
		delete(sc.sessions, fp)
		return nil
	}

	// 记录此 LLM trace 关联到此 session
	entry.llmTraces = append(entry.llmTraces, llmTraceID)

	return &SessionLink{
		IMTraceID: entry.imTraceID,
		SenderID:  entry.senderID,
		AppID:     entry.appID,
	}
}

// GetSessionTraces 查询某个 IM trace 关联了哪些 LLM traces
func (sc *SessionCorrelator) GetSessionTraces(imTraceID string) []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, entry := range sc.sessions {
		if entry.imTraceID == imTraceID {
			result := make([]string, len(entry.llmTraces))
			copy(result, entry.llmTraces)
			return result
		}
	}
	return nil
}

// Stats 返回当前关联器状态
func (sc *SessionCorrelator) Stats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	totalLinks := 0
	for _, e := range sc.sessions {
		totalLinks += len(e.llmTraces)
	}

	return map[string]interface{}{
		"active_sessions": len(sc.sessions),
		"total_llm_links": totalLinks,
		"max_size":        sc.maxSize,
		"window_ms":       sc.windowMs,
	}
}

func (sc *SessionCorrelator) evictLocked() {
	// 先清过期的
	now := time.Now()
	for k, v := range sc.sessions {
		if now.Sub(v.ts).Milliseconds() > sc.windowMs {
			delete(sc.sessions, k)
		}
	}
	// 仍超量则删最老的
	for len(sc.sessions) > sc.maxSize {
		var oldest string
		var oldestTs time.Time
		for k, v := range sc.sessions {
			if oldest == "" || v.ts.Before(oldestTs) {
				oldest = k
				oldestTs = v.ts
			}
		}
		if oldest != "" {
			delete(sc.sessions, oldest)
		}
	}
}

// ============================================================
// 指纹提取
// ============================================================

// contentFingerprint 对消息内容生成指纹
// 去除首尾空白，取 SHA256 前 16 字符
func contentFingerprint(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:8])
}

// extractFirstUserFingerprint 从 LLM 请求体中提取首条 user message 的内容指纹
// 支持 Anthropic 和 OpenAI 格式
func extractFirstUserFingerprint(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 解析 JSON
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	// 找最后一条 role=user 的消息（最新的用户输入）
	// 注意：多轮对话中可能有多条 user message
	// 我们取最后一条，因为它是触发本次 LLM 调用的直接消息
	var lastUserContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role != "user" {
			continue
		}

		// content 可能是字符串或数组（Anthropic content blocks）
		text := extractTextFromContent(msg.Content)
		if text != "" {
			lastUserContent = text
			break
		}
	}

	if lastUserContent == "" {
		return ""
	}

	return contentFingerprint(lastUserContent)
}

// extractTextFromContent 从 content 字段提取文本
// 支持：
//   - 纯字符串: "hello"
//   - Anthropic content blocks: [{"type":"text","text":"hello"}, {"type":"tool_result",...}]
//   - OpenAI content parts: [{"type":"text","text":"hello"}]
func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str)
	}

	// 尝试解析为数组
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		// 只提取 type=text 的块，忽略 tool_result 等
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, strings.TrimSpace(b.Text))
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// ============================================================
// 辅助：日志
// ============================================================

func logSessionLink(llmTraceID string, link *SessionLink) {
	if link != nil {
		log.Printf("[SessionCorrelator] LLM trace %s → IM trace %s (sender=%s, app=%s)",
			llmTraceID, link.IMTraceID, link.SenderID, link.AppID)
	}
}
