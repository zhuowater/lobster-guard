// session_replay.go — 会话回放引擎：通过 trace_id 串联 IM 审计 + LLM 调用 + 工具调用
// lobster-guard v13.0
package main

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ============================================================
// 会话回放引擎
// ============================================================

// SessionReplayEngine 会话回放引擎
type SessionReplayEngine struct {
	db *sql.DB
}

// NewSessionReplayEngine 创建会话回放引擎
func NewSessionReplayEngine(db *sql.DB) *SessionReplayEngine {
	// 确保 session_tags 表存在
	db.Exec(`CREATE TABLE IF NOT EXISTS session_tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id TEXT NOT NULL,
		event_type TEXT,
		event_id INTEGER,
		tag_text TEXT NOT NULL,
		author TEXT DEFAULT '',
		created_at TEXT NOT NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_session_tags_trace ON session_tags(trace_id)`)

	return &SessionReplayEngine{db: db}
}

// SessionSummary 会话摘要（列表用）
type SessionSummary struct {
	TraceID          string   `json:"trace_id"`
	SessionID        string   `json:"session_id,omitempty"`
	StartTime        string   `json:"start_time"`
	EndTime          string   `json:"end_time"`
	DurationMs       float64  `json:"duration_ms"`
	SenderID         string   `json:"sender_id"`
	DisplayName      string   `json:"display_name,omitempty"`
	Department       string   `json:"department,omitempty"`
	Model            string   `json:"model"`
	IMEvents         int      `json:"im_events"`
	LLMCalls         int      `json:"llm_calls"`
	ToolCalls        int      `json:"tool_calls"`
	TotalTokens      int      `json:"total_tokens"`
	RiskLevel        string   `json:"risk_level"`
	Blocked          bool     `json:"blocked"`
	CanaryLeaked     bool     `json:"canary_leaked"`
	BudgetExceeded   bool     `json:"budget_exceeded"`
	FlaggedTools     int      `json:"flagged_tools"`
	SourceCategories []string `json:"source_categories,omitempty"`
	SourceKeys       []string `json:"source_keys,omitempty"`
	Tags             []string `json:"tags"`
}

// SessionTimeline 完整时间线（详情用）
type SessionTimeline struct {
	TraceID string          `json:"trace_id"`
	Events  []TimelineEvent `json:"events"`
	Summary SessionSummary  `json:"summary"`
}

// TimelineEvent 时间线上的一个事件
type TimelineEvent struct {
	ID                   int64   `json:"id"`
	Timestamp            string  `json:"timestamp"`
	Type                 string  `json:"type"`
	Direction            string  `json:"direction,omitempty"`
	Action               string  `json:"action,omitempty"`
	Reason               string  `json:"reason,omitempty"`
	Content              string  `json:"content,omitempty"`
	SenderID             string  `json:"sender_id,omitempty"`
	Model                string  `json:"model,omitempty"`
	Tokens               int     `json:"tokens,omitempty"`
	LatencyMs            float64 `json:"latency_ms,omitempty"`
	StatusCode           int     `json:"status_code,omitempty"`
	HasToolUse           bool    `json:"has_tool_use,omitempty"`
	ToolCount            int     `json:"tool_count,omitempty"`
	ErrorMsg             string  `json:"error_message,omitempty"`
	CanaryLeak           bool    `json:"canary_leaked,omitempty"`
	BudgetOver           bool    `json:"budget_exceeded,omitempty"`
	RequestPreview       string  `json:"request_preview,omitempty"`
	ResponsePreview      string  `json:"response_preview,omitempty"`
	ToolName             string  `json:"tool_name,omitempty"`
	ToolInput            string  `json:"tool_input,omitempty"`
	ToolResult           string  `json:"tool_result,omitempty"`
	RiskLevel            string  `json:"risk_level,omitempty"`
	Flagged              bool    `json:"flagged,omitempty"`
	FlagReason           string  `json:"flag_reason,omitempty"`
	SourceCategory       string  `json:"source_category,omitempty"`
	SourceKey            string  `json:"source_key,omitempty"`
	SourceDescriptorJSON string  `json:"source_descriptor_json,omitempty"`
	TagText              string  `json:"tag_text,omitempty"`
	TagAuthor            string  `json:"tag_author,omitempty"`
	TagID                int64   `json:"tag_id,omitempty"`
}

// GetTimeline 获取某个 trace_id 的完整时间线
// 支持 IM↔LLM 双向关联：
//   - 如果 traceID 是 IM trace → 查关联的 LLM calls（via im_trace_id）
//   - 如果 traceID 是 LLM trace → 查关联的 IM events（via im_trace_id 反查）
func (e *SessionReplayEngine) GetTimeline(traceID string) (*SessionTimeline, error) {
	if traceID == "" {
		return nil, fmt.Errorf("trace_id is required")
	}

	var events []TimelineEvent

	// 0. 双向关联：收集所有相关的 trace_id
	imTraceIDs := []string{}  // 用于查 audit_log
	llmTraceIDs := []string{} // 用于查 llm_calls

	// 如果传入的是 session_id（以 sess- 开头），直接用 session_id 聚合
	var sessionID string
	if strings.HasPrefix(traceID, "sess-") || strings.HasPrefix(traceID, "session-") {
		sessionID = traceID
	} else {
		imTraceIDs = append(imTraceIDs, traceID)
		llmTraceIDs = append(llmTraceIDs, traceID)

		// 如果 traceID 是 LLM trace，查它的 im_trace_id
		var linkedIMTrace string
		e.db.QueryRow(`SELECT COALESCE(im_trace_id,'') FROM llm_calls WHERE trace_id=? LIMIT 1`, traceID).Scan(&linkedIMTrace)
		if linkedIMTrace != "" && linkedIMTrace != traceID {
			imTraceIDs = append(imTraceIDs, linkedIMTrace)
		}

		// 如果 traceID 是 IM trace，查所有关联的 LLM trace_id
		linkedLLMRows, err := e.db.Query(`SELECT DISTINCT trace_id FROM llm_calls WHERE im_trace_id=?`, traceID)
		if err == nil {
			defer linkedLLMRows.Close()
			for linkedLLMRows.Next() {
				var lt string
				if linkedLLMRows.Scan(&lt) == nil && lt != traceID {
					llmTraceIDs = append(llmTraceIDs, lt)
				}
			}
		}

		// 路径1: traceID 是 LLM trace → llm_calls.trace_id 匹配
		e.db.QueryRow(`SELECT COALESCE(session_id,'') FROM llm_calls WHERE trace_id=? AND session_id != '' LIMIT 1`, traceID).Scan(&sessionID)
		// 路径2: traceID 是 IM trace → llm_calls.im_trace_id 匹配（修复多轮会话详情只显示第一轮的 bug）
		if sessionID == "" {
			e.db.QueryRow(`SELECT COALESCE(session_id,'') FROM llm_calls WHERE im_trace_id=? AND session_id != '' LIMIT 1`, traceID).Scan(&sessionID)
		}
		// 路径3: linkedIMTrace 非空时再试
		if sessionID == "" && linkedIMTrace != "" {
			e.db.QueryRow(`SELECT COALESCE(session_id,'') FROM llm_calls WHERE im_trace_id=? AND session_id != '' LIMIT 1`, linkedIMTrace).Scan(&sessionID)
		}
	}
	if sessionID != "" {
		// 拉同 session 下所有 IM trace 和 LLM trace
		sessIMRows, err := e.db.Query(`SELECT DISTINCT im_trace_id FROM llm_calls WHERE session_id=? AND im_trace_id != ''`, sessionID)
		if err == nil {
			defer sessIMRows.Close()
			for sessIMRows.Next() {
				var it string
				if sessIMRows.Scan(&it) == nil {
					found := false
					for _, x := range imTraceIDs {
						if x == it {
							found = true
							break
						}
					}
					if !found {
						imTraceIDs = append(imTraceIDs, it)
					}
				}
			}
		}
		sessLLMRows, err := e.db.Query(`SELECT DISTINCT trace_id FROM llm_calls WHERE session_id=?`, sessionID)
		if err == nil {
			defer sessLLMRows.Close()
			for sessLLMRows.Next() {
				var lt string
				if sessLLMRows.Scan(&lt) == nil {
					found := false
					for _, x := range llmTraceIDs {
						if x == lt {
							found = true
							break
						}
					}
					if !found {
						llmTraceIDs = append(llmTraceIDs, lt)
					}
				}
			}
		}
	}

	// 1. 从 audit_log 查 IM 事件（所有关联的 IM trace_id）
	if len(imTraceIDs) == 0 {
		imTraceIDs = append(imTraceIDs, "__none__") // 占位符，不会匹配任何记录
	}
	imPlaceholders := make([]string, len(imTraceIDs))
	imArgs := make([]interface{}, len(imTraceIDs))
	for i, id := range imTraceIDs {
		imPlaceholders[i] = "?"
		imArgs[i] = id
	}
	rows, err := e.db.Query(fmt.Sprintf(`SELECT id, timestamp, direction, COALESCE(sender_id,''), action, COALESCE(reason,''), COALESCE(content_preview,''), COALESCE(latency_ms,0)
		FROM audit_log WHERE trace_id IN (%s) ORDER BY timestamp ASC`, strings.Join(imPlaceholders, ",")), imArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ev TimelineEvent
			var latMs float64
			if rows.Scan(&ev.ID, &ev.Timestamp, &ev.Direction, &ev.SenderID, &ev.Action, &ev.Reason, &ev.Content, &latMs) == nil {
				if ev.Direction == "inbound" {
					ev.Type = "im_inbound"
				} else {
					ev.Type = "im_outbound"
				}
				ev.LatencyMs = latMs
				events = append(events, ev)
			}
		}
	}

	// 2. 从 llm_calls 查 LLM 调用（所有关联的 LLM trace_id + im_trace_id 匹配）
	if len(llmTraceIDs) == 0 {
		llmTraceIDs = append(llmTraceIDs, "__none__")
	}
	llmPlaceholders := make([]string, len(llmTraceIDs))
	llmArgs := make([]interface{}, len(llmTraceIDs))
	for i, id := range llmTraceIDs {
		llmPlaceholders[i] = "?"
		llmArgs[i] = id
	}
	// 也通过 im_trace_id 关联查（双向）
	imPlaceholders2 := make([]string, len(imTraceIDs))
	for i, id := range imTraceIDs {
		imPlaceholders2[i] = "?"
		llmArgs = append(llmArgs, id)
	}
	llmRows, err := e.db.Query(fmt.Sprintf(`SELECT DISTINCT id, timestamp, COALESCE(model,''), COALESCE(request_tokens,0), COALESCE(response_tokens,0), COALESCE(total_tokens,0), COALESCE(latency_ms,0), COALESCE(status_code,0), COALESCE(has_tool_use,0), COALESCE(tool_count,0), COALESCE(error_message,''), COALESCE(canary_leaked,0), COALESCE(budget_exceeded,0), COALESCE(request_preview,''), COALESCE(response_preview,'')
		FROM llm_calls WHERE trace_id IN (%s) OR im_trace_id IN (%s) ORDER BY timestamp ASC`,
		strings.Join(llmPlaceholders, ","), strings.Join(imPlaceholders2, ",")), llmArgs...)
	if err == nil {
		defer llmRows.Close()
		for llmRows.Next() {
			var ev TimelineEvent
			var hasToolUse, canaryLeaked, budgetExceeded int
			if llmRows.Scan(&ev.ID, &ev.Timestamp, &ev.Model, &ev.Tokens, &ev.StatusCode, &ev.Tokens, &ev.LatencyMs, &ev.StatusCode, &hasToolUse, &ev.ToolCount, &ev.ErrorMsg, &canaryLeaked, &budgetExceeded, &ev.RequestPreview, &ev.ResponsePreview) == nil {
				ev.Type = "llm_call"
				ev.HasToolUse = hasToolUse != 0
				ev.CanaryLeak = canaryLeaked != 0
				ev.BudgetOver = budgetExceeded != 0
				events = append(events, ev)
			}
		}
	}

	// Collect llm_call IDs for this trace (using same dual-query as above)
	var llmCallIDs []int64
	idRows, err := e.db.Query(fmt.Sprintf(`SELECT DISTINCT id FROM llm_calls WHERE trace_id IN (%s) OR im_trace_id IN (%s)`,
		strings.Join(llmPlaceholders, ","), strings.Join(imPlaceholders2, ",")), llmArgs...)
	if err == nil {
		defer idRows.Close()
		for idRows.Next() {
			var id int64
			if idRows.Scan(&id) == nil {
				llmCallIDs = append(llmCallIDs, id)
			}
		}
	}

	// 3. 从 llm_tool_calls 查工具调用
	if len(llmCallIDs) > 0 {
		placeholders := make([]string, len(llmCallIDs))
		args := make([]interface{}, len(llmCallIDs))
		for i, id := range llmCallIDs {
			placeholders[i] = "?"
			args[i] = id
		}
		toolQuery := fmt.Sprintf(`SELECT id, timestamp, tool_name, COALESCE(tool_input_preview,''), COALESCE(tool_result_preview,''), COALESCE(risk_level,'low'), COALESCE(flagged,0), COALESCE(flag_reason,''), COALESCE(source_category,''), COALESCE(source_key,''), COALESCE(source_descriptor_json,'')
			FROM llm_tool_calls WHERE llm_call_id IN (%s) ORDER BY timestamp ASC`, strings.Join(placeholders, ","))
		toolRows, err := e.db.Query(toolQuery, args...)
		if err == nil {
			defer toolRows.Close()
			for toolRows.Next() {
				var ev TimelineEvent
				var flagged int
				if toolRows.Scan(&ev.ID, &ev.Timestamp, &ev.ToolName, &ev.ToolInput, &ev.ToolResult, &ev.RiskLevel, &flagged, &ev.FlagReason, &ev.SourceCategory, &ev.SourceKey, &ev.SourceDescriptorJSON) == nil {
					ev.Type = "tool_call"
					ev.Flagged = flagged != 0
					events = append(events, ev)
				}
			}
		}
	}

	// 4. 从 session_tags 查标签
	tagRows, err := e.db.Query(`SELECT id, COALESCE(event_type,''), COALESCE(event_id,0), tag_text, COALESCE(author,''), created_at
		FROM session_tags WHERE trace_id=? ORDER BY created_at ASC`, traceID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var ev TimelineEvent
			var eventType string
			var eventID int64
			if tagRows.Scan(&ev.TagID, &eventType, &eventID, &ev.TagText, &ev.TagAuthor, &ev.Timestamp) == nil {
				ev.Type = "tag"
				ev.ID = ev.TagID
				events = append(events, ev)
			}
		}
	}

	// 5. 按时间排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	// 6. 计算 summary
	summary := e.buildSummary(traceID, events)

	return &SessionTimeline{
		TraceID: traceID,
		Events:  events,
		Summary: summary,
	}, nil
}

// buildSummary 从事件列表计算摘要
func (e *SessionReplayEngine) buildSummary(traceID string, events []TimelineEvent) SessionSummary {
	s := SessionSummary{
		TraceID:          traceID,
		RiskLevel:        "low",
		Tags:             []string{},
		SourceCategories: []string{},
		SourceKeys:       []string{},
	}

	var startTime, endTime time.Time
	for _, ev := range events {
		t, err := time.Parse(time.RFC3339, ev.Timestamp)
		if err != nil {
			t, err = time.Parse(time.RFC3339Nano, ev.Timestamp)
		}
		if err == nil {
			if startTime.IsZero() || t.Before(startTime) {
				startTime = t
			}
			if endTime.IsZero() || t.After(endTime) {
				endTime = t
			}
		}

		switch ev.Type {
		case "im_inbound", "im_outbound":
			s.IMEvents++
			if ev.SenderID != "" && s.SenderID == "" {
				s.SenderID = ev.SenderID
			}
			if ev.Action == "block" {
				s.Blocked = true
			}
		case "llm_call":
			s.LLMCalls++
			if ev.Model != "" && s.Model == "" {
				s.Model = ev.Model
			}
			s.TotalTokens += ev.Tokens
			if ev.CanaryLeak {
				s.CanaryLeaked = true
			}
			if ev.BudgetOver {
				s.BudgetExceeded = true
			}
		case "tool_call":
			s.ToolCalls++
			if ev.Flagged {
				s.FlaggedTools++
			}
			if ev.SourceCategory != "" && !containsReplayString(s.SourceCategories, ev.SourceCategory) {
				s.SourceCategories = append(s.SourceCategories, ev.SourceCategory)
			}
			if ev.SourceKey != "" && !containsReplayString(s.SourceKeys, ev.SourceKey) {
				s.SourceKeys = append(s.SourceKeys, ev.SourceKey)
			}
		case "tag":
			s.Tags = append(s.Tags, ev.TagText)
		}
	}

	if !startTime.IsZero() {
		s.StartTime = startTime.Format(time.RFC3339Nano)
	}
	if !endTime.IsZero() {
		s.EndTime = endTime.Format(time.RFC3339Nano)
	}
	if !startTime.IsZero() && !endTime.IsZero() {
		s.DurationMs = float64(endTime.Sub(startTime).Milliseconds())
	}

	// 计算风险等级
	s.RiskLevel = calcRiskLevel(s)

	return s
}

// calcRiskLevel 根据会话摘要计算风险等级
func calcRiskLevel(s SessionSummary) string {
	if s.CanaryLeaked || s.Blocked {
		return "critical"
	}
	if s.BudgetExceeded || s.FlaggedTools >= 2 {
		return "high"
	}
	if s.FlaggedTools >= 1 {
		return "medium"
	}
	return "low"
}

func containsReplayString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// ListSessions 列出会话（分页）
func (e *SessionReplayEngine) ListSessions(from, to, senderID, riskLevel, q string, limit, offset int) ([]SessionSummary, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	// v22.6: 按 session_id 聚合，一个会话只显示一条
	// 策略：
	//   1. 从 llm_calls 获取所有 session_id（有关联的）+ 孤立的 trace_id（无 session_id 的）
	//   2. 从 audit_log 获取孤立的 IM trace_id（不在任何 session 中的）
	//   3. 每个 session/trace 构建聚合摘要

	var whereTime string
	var timeArgs []interface{}
	if from != "" {
		whereTime += " AND timestamp >= ?"
		timeArgs = append(timeArgs, from)
	}
	if to != "" {
		whereTime += " AND timestamp <= ?"
		timeArgs = append(timeArgs, to)
	}

	type sessionEntry struct {
		key       string // session_id 或 trace_id（孤立的）
		isSession bool   // true=session_id聚合, false=单trace_id
		firstTs   string
	}

	var entries []sessionEntry
	seen := map[string]bool{}

	// Step 1: 从 llm_calls 获取 session_id 聚合
	sessRows, err := e.db.Query(`SELECT COALESCE(session_id,''), MIN(timestamp) as first_ts
		FROM llm_calls WHERE trace_id IS NOT NULL AND trace_id != '' `+whereTime+`
		GROUP BY CASE WHEN session_id != '' THEN session_id ELSE trace_id END
		ORDER BY first_ts DESC`, timeArgs...)
	if err == nil {
		defer sessRows.Close()
		for sessRows.Next() {
			var sid, ts string
			if sessRows.Scan(&sid, &ts) == nil && sid != "" {
				if !seen[sid] {
					seen[sid] = true
					entries = append(entries, sessionEntry{key: sid, isSession: sid != "" && strings.HasPrefix(sid, "sess-"), firstTs: ts})
				}
			}
		}
	}

	// Step 2: 从 audit_log 获取孤立的 IM trace（不在任何 llm_calls.im_trace_id 中的）
	imRows, err := e.db.Query(`SELECT trace_id, MIN(timestamp) as first_ts
		FROM audit_log WHERE trace_id IS NOT NULL AND trace_id != '' `+whereTime+`
		GROUP BY trace_id ORDER BY first_ts DESC`, timeArgs...)
	if err == nil {
		defer imRows.Close()
		for imRows.Next() {
			var tid, ts string
			if imRows.Scan(&tid, &ts) == nil && tid != "" {
				// 检查是否已被某个 session 包含
				var linkedSession string
				e.db.QueryRow(`SELECT COALESCE(session_id,'') FROM llm_calls WHERE im_trace_id=? AND session_id != '' LIMIT 1`, tid).Scan(&linkedSession)
				if linkedSession != "" && seen[linkedSession] {
					continue // 已在某个 session 聚合中
				}
				// 也检查是否有直接的 LLM trace 匹配
				var directLLM int
				e.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE im_trace_id=?`, tid).Scan(&directLLM)
				if directLLM > 0 {
					continue // 有关联的 LLM，已在 session 聚合中
				}
				if !seen[tid] {
					seen[tid] = true
					entries = append(entries, sessionEntry{key: tid, isSession: false, firstTs: ts})
				}
			}
		}
	}

	// 按 firstTs 降序排列
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].firstTs > entries[j].firstTs
	})

	// Step 3: 为每个 entry 构建摘要
	var summaries []SessionSummary
	for _, ent := range entries {
		var sum *SessionSummary
		if ent.isSession {
			sum = e.sessionSummary(ent.key)
		} else {
			sum = e.quickSummary(ent.key)
		}
		if sum == nil {
			continue
		}

		// 过滤条件
		if senderID != "" && sum.SenderID != senderID {
			continue
		}
		if riskLevel != "" && riskLevel != "all" {
			if riskLevel == "high" && sum.RiskLevel != "high" && sum.RiskLevel != "critical" {
				continue
			}
			if riskLevel == "critical" && sum.RiskLevel != "critical" {
				continue
			}
		}
		if q != "" {
			ql := strings.ToLower(q)
			if !strings.Contains(strings.ToLower(sum.TraceID), ql) &&
				!strings.Contains(strings.ToLower(sum.SenderID), ql) &&
				!strings.Contains(strings.ToLower(sum.DisplayName), ql) &&
				!strings.Contains(strings.ToLower(sum.Department), ql) &&
				!strings.Contains(strings.ToLower(sum.Model), ql) {
				continue
			}
		}

		summaries = append(summaries, *sum)
	}

	total := len(summaries)

	// 分页
	if offset >= len(summaries) {
		return []SessionSummary{}, total, nil
	}
	end := offset + limit
	if end > len(summaries) {
		end = len(summaries)
	}
	page := summaries[offset:end]

	return page, total, nil
}

// ListSessionsTenant v14.0: 租户感知的会话列表
func (e *SessionReplayEngine) ListSessionsTenant(from, to, senderID, riskLevel, q, sourceCategory, tenantID string, limit, offset int) ([]SessionSummary, int, error) {
	// 目前租户仍委托给通用查询；sourceCategory 在聚合后做过滤即可
	sessions, total, err := e.ListSessions(from, to, senderID, riskLevel, q, limit, offset)
	if err != nil || sourceCategory == "" {
		return sessions, total, err
	}
	filtered := make([]SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		if containsReplayString(s.SourceCategories, sourceCategory) {
			filtered = append(filtered, s)
		}
	}
	return filtered, len(filtered), nil
}

// sessionSummary 按 session_id 聚合摘要（合并所有关联的 IM trace 和 LLM trace）
func (e *SessionReplayEngine) sessionSummary(sessionID string) *SessionSummary {
	// 用第一个 IM trace 作为展示用的 trace_id
	var representativeTrace string

	// 收集该 session 下所有 IM trace_id
	var imTraceIDs []string
	imTraceRows, err := e.db.Query(`SELECT DISTINCT im_trace_id FROM llm_calls WHERE session_id=? AND im_trace_id != ''`, sessionID)
	if err == nil {
		defer imTraceRows.Close()
		for imTraceRows.Next() {
			var it string
			if imTraceRows.Scan(&it) == nil {
				imTraceIDs = append(imTraceIDs, it)
			}
		}
	}
	if len(imTraceIDs) > 0 {
		representativeTrace = imTraceIDs[0]
	}

	// 如果没有 IM trace，用第一个 LLM trace
	if representativeTrace == "" {
		e.db.QueryRow(`SELECT trace_id FROM llm_calls WHERE session_id=? ORDER BY timestamp ASC LIMIT 1`, sessionID).Scan(&representativeTrace)
	}
	if representativeTrace == "" {
		representativeTrace = sessionID
	}

	s := &SessionSummary{
		TraceID:          representativeTrace,
		SessionID:        sessionID,
		RiskLevel:        "low",
		Tags:             []string{},
		SourceCategories: []string{},
		SourceKeys:       []string{},
	}

	// 聚合 IM 事件（从所有关联的 im_trace_id）
	if len(imTraceIDs) > 0 {
		phs := make([]string, len(imTraceIDs))
		args := make([]interface{}, len(imTraceIDs))
		for i, id := range imTraceIDs {
			phs[i] = "?"
			args[i] = id
		}
		var imCount int
		var senderID sql.NullString
		var blocked int
		var firstTs, lastTs sql.NullString
		e.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*), COALESCE(MIN(sender_id),''), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0), MIN(timestamp), MAX(timestamp) FROM audit_log WHERE trace_id IN (%s)`, strings.Join(phs, ",")), args...).Scan(&imCount, &senderID, &blocked, &firstTs, &lastTs)
		s.IMEvents = imCount
		if senderID.Valid {
			s.SenderID = senderID.String
		}
		s.Blocked = blocked > 0
		if firstTs.Valid {
			if t, err := time.Parse(time.RFC3339Nano, firstTs.String); err == nil {
				s.StartTime = firstTs.String
				_ = t
			} else if t, err := time.Parse(time.RFC3339, firstTs.String); err == nil {
				s.StartTime = firstTs.String
				_ = t
			}
		}
	}

	// 聚合 LLM 调用（该 session 下所有）
	var llmCount int
	var model sql.NullString
	var totalTokens int
	var canaryLeaked, budgetExceeded int
	var llmFirstTs, llmLastTs sql.NullString
	e.db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(model),''), COALESCE(SUM(total_tokens),0), COALESCE(SUM(canary_leaked),0), COALESCE(SUM(budget_exceeded),0), MIN(timestamp), MAX(timestamp) FROM llm_calls WHERE session_id=?`, sessionID).Scan(&llmCount, &model, &totalTokens, &canaryLeaked, &budgetExceeded, &llmFirstTs, &llmLastTs)
	s.LLMCalls = llmCount
	if model.Valid {
		s.Model = model.String
	}
	s.TotalTokens = totalTokens
	s.CanaryLeaked = canaryLeaked > 0
	s.BudgetExceeded = budgetExceeded > 0

	if s.IMEvents == 0 && llmCount == 0 {
		return nil
	}

	// 计算持续时间
	var startTime, endTime time.Time
	for _, tsStr := range []sql.NullString{llmFirstTs, llmLastTs} {
		if tsStr.Valid {
			if t, err := time.Parse(time.RFC3339Nano, tsStr.String); err == nil {
				if startTime.IsZero() || t.Before(startTime) {
					startTime = t
				}
				if endTime.IsZero() || t.After(endTime) {
					endTime = t
				}
			} else if t, err := time.Parse(time.RFC3339, tsStr.String); err == nil {
				if startTime.IsZero() || t.Before(startTime) {
					startTime = t
				}
				if endTime.IsZero() || t.After(endTime) {
					endTime = t
				}
			}
		}
	}
	if !startTime.IsZero() && !endTime.IsZero() {
		s.DurationMs = float64(endTime.Sub(startTime).Milliseconds())
	}

	// 工具调用
	var toolCount, flaggedCount int
	sourceCats := map[string]bool{}
	sourceKeys := map[string]bool{}
	idRows, err := e.db.Query(`SELECT id FROM llm_calls WHERE session_id=?`, sessionID)
	if err == nil {
		var ids []interface{}
		var phs []string
		for idRows.Next() {
			var id int64
			if idRows.Scan(&id) == nil {
				ids = append(ids, id)
				phs = append(phs, "?")
			}
		}
		idRows.Close()
		if len(ids) > 0 {
			e.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN flagged=1 THEN 1 ELSE 0 END),0) FROM llm_tool_calls WHERE llm_call_id IN (%s)`, strings.Join(phs, ",")), ids...).Scan(&toolCount, &flaggedCount)
			rows, err := e.db.Query(fmt.Sprintf(`SELECT DISTINCT COALESCE(source_category,''), COALESCE(source_key,'') FROM llm_tool_calls WHERE llm_call_id IN (%s)`, strings.Join(phs, ",")), ids...)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var cat, key string
					if rows.Scan(&cat, &key) == nil {
						if cat != "" {
							sourceCats[cat] = true
						}
						if key != "" {
							sourceKeys[key] = true
						}
					}
				}
			}
		}
	}
	s.ToolCalls = toolCount
	s.FlaggedTools = flaggedCount
	for cat := range sourceCats {
		s.SourceCategories = append(s.SourceCategories, cat)
	}
	for key := range sourceKeys {
		s.SourceKeys = append(s.SourceKeys, key)
	}
	sort.Strings(s.SourceCategories)
	sort.Strings(s.SourceKeys)

	// 风险等级
	if s.CanaryLeaked {
		s.RiskLevel = "critical"
	} else if s.Blocked || s.FlaggedTools > 0 {
		s.RiskLevel = "high"
	} else if s.BudgetExceeded || flaggedCount > 0 {
		s.RiskLevel = "medium"
	}

	e.enrichUserInfo(s)
	return s
}

// quickSummary 快速构建某个 trace_id 的摘要（不查标签以加速列表查询）
func (e *SessionReplayEngine) quickSummary(traceID string) *SessionSummary {
	s := &SessionSummary{
		TraceID:          traceID,
		RiskLevel:        "low",
		Tags:             []string{},
		SourceCategories: []string{},
		SourceKeys:       []string{},
	}

	// 从 audit_log 获取 IM 信息
	var imCount int
	var senderID, firstIMTs, lastIMTs sql.NullString
	var blocked int
	e.db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(sender_id),''), MIN(timestamp), MAX(timestamp), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0) FROM audit_log WHERE trace_id=?`, traceID).Scan(&imCount, &senderID, &firstIMTs, &lastIMTs, &blocked)
	s.IMEvents = imCount
	if senderID.Valid {
		s.SenderID = senderID.String
	}
	s.Blocked = blocked > 0

	// 从 llm_calls 获取 LLM 信息
	var llmCount int
	var model sql.NullString
	var totalTokens int
	var firstLLMTs, lastLLMTs sql.NullString
	var canaryLeaked, budgetExceeded int
	e.db.QueryRow(`SELECT COUNT(*), COALESCE(MIN(model),''), COALESCE(SUM(total_tokens),0), MIN(timestamp), MAX(timestamp), COALESCE(SUM(canary_leaked),0), COALESCE(SUM(budget_exceeded),0) FROM llm_calls WHERE trace_id=?`, traceID).Scan(&llmCount, &model, &totalTokens, &firstLLMTs, &lastLLMTs, &canaryLeaked, &budgetExceeded)
	s.LLMCalls = llmCount
	if model.Valid {
		s.Model = model.String
	}
	s.TotalTokens = totalTokens
	s.CanaryLeaked = canaryLeaked > 0
	s.BudgetExceeded = budgetExceeded > 0

	// 跳过完全空的 trace
	if imCount == 0 && llmCount == 0 {
		return nil
	}

	// 从 llm_tool_calls 获取工具信息
	var toolCount, flaggedCount int
	sourceCats := map[string]bool{}
	sourceKeys := map[string]bool{}
	// Get all llm_call IDs for this trace
	idRows, err := e.db.Query(`SELECT id FROM llm_calls WHERE trace_id=?`, traceID)
	if err == nil {
		var ids []interface{}
		var phs []string
		for idRows.Next() {
			var id int64
			if idRows.Scan(&id) == nil {
				ids = append(ids, id)
				phs = append(phs, "?")
			}
		}
		idRows.Close()
		if len(ids) > 0 {
			e.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN flagged=1 THEN 1 ELSE 0 END),0) FROM llm_tool_calls WHERE llm_call_id IN (%s)`, strings.Join(phs, ",")), ids...).Scan(&toolCount, &flaggedCount)
			rows, err := e.db.Query(fmt.Sprintf(`SELECT DISTINCT COALESCE(source_category,''), COALESCE(source_key,'') FROM llm_tool_calls WHERE llm_call_id IN (%s)`, strings.Join(phs, ",")), ids...)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var cat, key string
					if rows.Scan(&cat, &key) == nil {
						if cat != "" {
							sourceCats[cat] = true
						}
						if key != "" {
							sourceKeys[key] = true
						}
					}
				}
			}
		}
	}
	s.ToolCalls = toolCount
	s.FlaggedTools = flaggedCount
	for cat := range sourceCats {
		s.SourceCategories = append(s.SourceCategories, cat)
	}
	for key := range sourceKeys {
		s.SourceKeys = append(s.SourceKeys, key)
	}
	sort.Strings(s.SourceCategories)
	sort.Strings(s.SourceKeys)

	// 计算时间
	timestamps := []string{}
	if firstIMTs.Valid && firstIMTs.String != "" {
		timestamps = append(timestamps, firstIMTs.String)
	}
	if lastIMTs.Valid && lastIMTs.String != "" {
		timestamps = append(timestamps, lastIMTs.String)
	}
	if firstLLMTs.Valid && firstLLMTs.String != "" {
		timestamps = append(timestamps, firstLLMTs.String)
	}
	if lastLLMTs.Valid && lastLLMTs.String != "" {
		timestamps = append(timestamps, lastLLMTs.String)
	}
	if len(timestamps) > 0 {
		sort.Strings(timestamps)
		s.StartTime = timestamps[0]
		s.EndTime = timestamps[len(timestamps)-1]
		t1, err1 := time.Parse(time.RFC3339, s.StartTime)
		if err1 != nil {
			t1, err1 = time.Parse(time.RFC3339Nano, s.StartTime)
		}
		t2, err2 := time.Parse(time.RFC3339, s.EndTime)
		if err2 != nil {
			t2, err2 = time.Parse(time.RFC3339Nano, s.EndTime)
		}
		if err1 == nil && err2 == nil {
			s.DurationMs = math.Round(float64(t2.Sub(t1).Milliseconds())*10) / 10
		}
	}

	// 获取标签
	tagRows, err := e.db.Query(`SELECT tag_text FROM session_tags WHERE trace_id=?`, traceID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag string
			if tagRows.Scan(&tag) == nil {
				s.Tags = append(s.Tags, tag)
			}
		}
	}

	// 风险等级
	s.RiskLevel = calcRiskLevel(*s)

	e.enrichUserInfo(s)
	return s
}

// enrichUserInfo 从 user_routes 查询 display_name 和 department
func (e *SessionReplayEngine) enrichUserInfo(s *SessionSummary) {
	if s.SenderID == "" {
		return
	}
	var displayName, department sql.NullString
	e.db.QueryRow(`SELECT MAX(display_name), MAX(department) FROM user_routes WHERE sender_id=?`, s.SenderID).Scan(&displayName, &department)
	if displayName.Valid && displayName.String != "" {
		s.DisplayName = displayName.String
	}
	if department.Valid && department.String != "" {
		s.Department = department.String
	}
}

// AddTag 添加标签
func (e *SessionReplayEngine) AddTag(traceID, eventType string, eventID int, tagText, author string) (int64, error) {
	if traceID == "" || tagText == "" {
		return 0, fmt.Errorf("trace_id and tag_text required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := e.db.Exec(`INSERT INTO session_tags (trace_id, event_type, event_id, tag_text, author, created_at) VALUES (?,?,?,?,?,?)`,
		traceID, eventType, eventID, tagText, author, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// DeleteTag 删除标签
func (e *SessionReplayEngine) DeleteTag(tagID int) error {
	result, err := e.db.Exec(`DELETE FROM session_tags WHERE id=?`, tagID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}

// SearchSessions 搜索会话（全文搜索）
func (e *SessionReplayEngine) SearchSessions(query, from, to string, limit int) ([]SessionSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	sessions, _, err := e.ListSessions(from, to, "", "", query, limit, 0)
	return sessions, err
}
