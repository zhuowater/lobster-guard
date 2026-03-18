// session_correlator.go — IM↔LLM 会话级 trace 关联（v17.3）
//
// 双层关联机制：
//
//   Layer 1 — 活跃会话（appID + senderID + 滚动时间窗口）
//     同一个 appID+senderID 在 idleTimeout 内的连续 IM 消息归入同一个 session。
//     超过 idleTimeout 没有新消息 → 自动切新 session。
//     每条 IM 消息都记录自己的 trace_id，同一 session 内的所有 IM trace 共享 session_id。
//
//   Layer 2 — 内容指纹（LLM 请求 messages ↔ IM 消息指纹）
//     LLM 请求体的 messages 中最后一条 user content 指纹匹配到已知 IM 消息 → 精确关联。
//     如果指纹未命中，退化到 Layer 1：取该 appID+senderID 最近活跃 session。
//
// 数据流：
//   IM入站 "帮我查张三邮件" (sender=张卓, app=app1, trace=im-001)
//     → RegisterIMSession → session_id=sess-001, 记录指纹
//   IM入站 "转发给李四" (sender=张卓, app=app1, trace=im-002, 30秒后)
//     → RegisterIMSession → 同一个 sess-001（30秒 < idleTimeout）
//   LLM第1轮 messages含 "帮我查张三邮件" (trace=llm-001)
//     → MatchLLMRequest → Layer2 指纹命中 → sess-001, im_trace=im-001
//   LLM第2轮 messages含 "转发给李四" (trace=llm-002)
//     → MatchLLMRequest → Layer2 指纹命中 → sess-001, im_trace=im-002
//   LLM第3轮 messages 指纹不命中
//     → Layer1 退化匹配 → sess-001（最近活跃 session of 张卓@app1）
//
//   --- 1小时空档 ---
//
//   IM入站 "今天天气" (sender=张卓, app=app1, trace=im-010)
//     → RegisterIMSession → 新 session_id=sess-002（超过 idleTimeout）
//
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// SessionCorrelator 将 IM 入站消息与 LLM 请求通过双层机制关联
type SessionCorrelator struct {
	mu            sync.RWMutex
	// Layer 1: appID:senderID → 活跃 session
	activeSessions map[string]*activeSession
	// Layer 2: content fingerprint → IM trace info
	fingerprints   map[string]*fingerprintEntry
	// 配置
	idleTimeoutMs  int64 // 活跃会话空闲超时（毫秒），默认 1 小时
	fpWindowMs     int64 // 指纹匹配窗口（毫秒），默认 5 分钟
	maxSessions    int
	maxFingerprints int
}

// activeSession 一个用户的连续会话
type activeSession struct {
	sessionID  string    // 会话唯一 ID
	appID      string
	senderID   string
	lastActive time.Time // 最后活跃时间（每条 IM 消息刷新）
	imTraces   []string  // 该 session 内所有 IM trace_id
	llmTraces  []string  // 该 session 关联的所有 LLM trace_id
	latestIMTrace string // 最近一条 IM trace_id
}

// fingerprintEntry Layer 2 指纹条目
type fingerprintEntry struct {
	imTraceID  string
	sessionID  string
	senderID   string
	appID      string
	ts         time.Time
}

// SessionLink 关联结果
type SessionLink struct {
	SessionID string // 会话 ID
	IMTraceID string // 最相关的 IM trace_id
	SenderID  string
	AppID     string
	Method    string // "fingerprint" 或 "active_session"
}

func NewSessionCorrelator(maxSessions int, idleTimeoutMs int64) *SessionCorrelator {
	if maxSessions <= 0 {
		maxSessions = 50000
	}
	if idleTimeoutMs <= 0 {
		idleTimeoutMs = 60 * 60 * 1000 // 默认 1 小时
	}
	return &SessionCorrelator{
		activeSessions:  make(map[string]*activeSession),
		fingerprints:    make(map[string]*fingerprintEntry),
		idleTimeoutMs:   idleTimeoutMs,
		fpWindowMs:      5 * 60 * 1000, // 指纹窗口固定 5 分钟
		maxSessions:     maxSessions,
		maxFingerprints: maxSessions * 3, // 每个 session 平均 3 条消息
	}
}

// sessionKey 生成 Layer 1 的 key
func sessionKey(appID, senderID string) string {
	return appID + ":" + senderID
}

// generateSessionID 生成随机 session ID
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("sess-%s", hex.EncodeToString(b))
}

// RegisterIMSession IM 入站时注册
// 同一 appID+senderID 在 idleTimeout 内 → 归入同一 session
// 超时 → 创建新 session
func (sc *SessionCorrelator) RegisterIMSession(msgText, imTraceID, senderID, appID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	key := sessionKey(appID, senderID)

	// Layer 1: 查找或创建活跃 session
	sess, exists := sc.activeSessions[key]
	if exists && now.Sub(sess.lastActive).Milliseconds() <= sc.idleTimeoutMs {
		// 同一 session，刷新活跃时间
		sess.lastActive = now
		sess.imTraces = append(sess.imTraces, imTraceID)
		sess.latestIMTrace = imTraceID
	} else {
		// 新 session
		sess = &activeSession{
			sessionID:     generateSessionID(),
			appID:         appID,
			senderID:      senderID,
			lastActive:    now,
			imTraces:      []string{imTraceID},
			latestIMTrace: imTraceID,
		}
		sc.activeSessions[key] = sess
	}

	// Layer 2: 注册内容指纹
	fp := contentFingerprint(msgText)
	if fp != "" {
		sc.fingerprints[fp] = &fingerprintEntry{
			imTraceID: imTraceID,
			sessionID: sess.sessionID,
			senderID:  senderID,
			appID:     appID,
			ts:        now,
		}
	}

	// 淘汰
	sc.evictLocked(now)
}

// MatchLLMRequest LLM 请求时匹配会话
// 优先 Layer 2 指纹匹配，未命中退化到 Layer 1 活跃 session
func (sc *SessionCorrelator) MatchLLMRequest(reqBody []byte, llmTraceID string) *SessionLink {
	fp := extractFirstUserFingerprint(reqBody)

	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()

	// Layer 2: 指纹精确匹配
	if fp != "" {
		if entry, ok := sc.fingerprints[fp]; ok {
			if now.Sub(entry.ts).Milliseconds() <= sc.fpWindowMs {
				// 找到对应的活跃 session，记录 LLM trace
				key := sessionKey(entry.appID, entry.senderID)
				if sess, ok := sc.activeSessions[key]; ok {
					sess.llmTraces = append(sess.llmTraces, llmTraceID)
				}
				return &SessionLink{
					SessionID: entry.sessionID,
					IMTraceID: entry.imTraceID,
					SenderID:  entry.senderID,
					AppID:     entry.appID,
					Method:    "fingerprint",
				}
			}
			// 过期，删除
			delete(sc.fingerprints, fp)
		}
	}

	// Layer 1 退化：尝试从 LLM 请求体中提取所有 user messages 的指纹
	// 匹配任意一条 → 找到 session
	allFPs := extractAllUserFingerprints(reqBody)
	for _, afp := range allFPs {
		if afp == fp {
			continue // 已经试过了
		}
		if entry, ok := sc.fingerprints[afp]; ok {
			if now.Sub(entry.ts).Milliseconds() <= sc.fpWindowMs {
				key := sessionKey(entry.appID, entry.senderID)
				if sess, ok := sc.activeSessions[key]; ok {
					sess.llmTraces = append(sess.llmTraces, llmTraceID)
				}
				return &SessionLink{
					SessionID: entry.sessionID,
					IMTraceID: entry.imTraceID,
					SenderID:  entry.senderID,
					AppID:     entry.appID,
					Method:    "fingerprint",
				}
			}
		}
	}

	// Layer 1 最终退化：找最近活跃的 session（在 idleTimeout 内）
	// 用 LLM 请求的来源信息（如果有 tenant_id 等）缩小范围
	// 当前简化版：遍历所有活跃 session，找 idleTimeout 内最近活跃的
	var bestSess *activeSession
	for _, sess := range sc.activeSessions {
		if now.Sub(sess.lastActive).Milliseconds() > sc.idleTimeoutMs {
			continue
		}
		if bestSess == nil || sess.lastActive.After(bestSess.lastActive) {
			bestSess = sess
		}
	}

	if bestSess != nil {
		bestSess.llmTraces = append(bestSess.llmTraces, llmTraceID)
		return &SessionLink{
			SessionID: bestSess.sessionID,
			IMTraceID: bestSess.latestIMTrace,
			SenderID:  bestSess.senderID,
			AppID:     bestSess.appID,
			Method:    "active_session",
		}
	}

	return nil
}

// GetSessionTraces 查询某个 session 关联的所有 traces
func (sc *SessionCorrelator) GetSessionTraces(sessionID string) (imTraces, llmTraces []string) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, sess := range sc.activeSessions {
		if sess.sessionID == sessionID {
			im := make([]string, len(sess.imTraces))
			copy(im, sess.imTraces)
			llm := make([]string, len(sess.llmTraces))
			copy(llm, sess.llmTraces)
			return im, llm
		}
	}
	return nil, nil
}

// GetSessionByIMTrace 通过 IM trace 查找 session
func (sc *SessionCorrelator) GetSessionByIMTrace(imTraceID string) *SessionLink {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	for _, sess := range sc.activeSessions {
		for _, t := range sess.imTraces {
			if t == imTraceID {
				return &SessionLink{
					SessionID: sess.sessionID,
					IMTraceID: imTraceID,
					SenderID:  sess.senderID,
					AppID:     sess.appID,
				}
			}
		}
	}
	return nil
}

// Stats 返回当前关联器状态
func (sc *SessionCorrelator) Stats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	totalIMTraces := 0
	totalLLMTraces := 0
	for _, s := range sc.activeSessions {
		totalIMTraces += len(s.imTraces)
		totalLLMTraces += len(s.llmTraces)
	}

	return map[string]interface{}{
		"active_sessions":   len(sc.activeSessions),
		"fingerprints":      len(sc.fingerprints),
		"total_im_traces":   totalIMTraces,
		"total_llm_traces":  totalLLMTraces,
		"idle_timeout_ms":   sc.idleTimeoutMs,
		"fp_window_ms":      sc.fpWindowMs,
	}
}

func (sc *SessionCorrelator) evictLocked(now time.Time) {
	// 清过期 session
	for k, sess := range sc.activeSessions {
		if now.Sub(sess.lastActive).Milliseconds() > sc.idleTimeoutMs*2 {
			delete(sc.activeSessions, k)
		}
	}
	// 清过期指纹
	for k, fp := range sc.fingerprints {
		if now.Sub(fp.ts).Milliseconds() > sc.fpWindowMs*2 {
			delete(sc.fingerprints, k)
		}
	}
	// 超量淘汰 session
	for len(sc.activeSessions) > sc.maxSessions {
		var oldest string
		var oldestTs time.Time
		for k, v := range sc.activeSessions {
			if oldest == "" || v.lastActive.Before(oldestTs) {
				oldest = k
				oldestTs = v.lastActive
			}
		}
		if oldest != "" {
			delete(sc.activeSessions, oldest)
		}
	}
}

// ============================================================
// 指纹提取
// ============================================================

// contentFingerprint 对消息内容生成指纹
func contentFingerprint(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:8])
}

// extractFirstUserFingerprint 从 LLM 请求体中提取最后一条有文本的 user message 指纹
func extractFirstUserFingerprint(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	// 从最后往前找第一条有文本的 user message
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role != "user" {
			continue
		}
		text := extractTextFromContent(msg.Content)
		if text != "" {
			return contentFingerprint(text)
		}
	}
	return ""
}

// extractAllUserFingerprints 提取所有 user messages 的指纹
func extractAllUserFingerprints(body []byte) []string {
	if len(body) == 0 {
		return nil
	}

	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	var fps []string
	seen := make(map[string]bool)
	for _, msg := range req.Messages {
		if msg.Role != "user" {
			continue
		}
		text := extractTextFromContent(msg.Content)
		if text == "" {
			continue
		}
		fp := contentFingerprint(text)
		if fp != "" && !seen[fp] {
			fps = append(fps, fp)
			seen[fp] = true
		}
	}
	return fps
}

// extractTextFromContent 从 content 字段提取文本
func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return strings.TrimSpace(str)
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
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
// 日志
// ============================================================

func logSessionLink(llmTraceID string, link *SessionLink) {
	if link != nil {
		log.Printf("[SessionCorrelator] LLM %s → session %s (im=%s, sender=%s, method=%s)",
			llmTraceID, link.SessionID, link.IMTraceID, link.SenderID, link.Method)
	}
}
