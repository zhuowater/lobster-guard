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
	TraceID        string   `json:"trace_id"`
	StartTime      string   `json:"start_time"`
	EndTime        string   `json:"end_time"`
	DurationMs     float64  `json:"duration_ms"`
	SenderID       string   `json:"sender_id"`
	Model          string   `json:"model"`
	IMEvents       int      `json:"im_events"`
	LLMCalls       int      `json:"llm_calls"`
	ToolCalls      int      `json:"tool_calls"`
	TotalTokens    int      `json:"total_tokens"`
	RiskLevel      string   `json:"risk_level"`
	Blocked        bool     `json:"blocked"`
	CanaryLeaked   bool     `json:"canary_leaked"`
	BudgetExceeded bool     `json:"budget_exceeded"`
	FlaggedTools   int      `json:"flagged_tools"`
	Tags           []string `json:"tags"`
}

// SessionTimeline 完整时间线（详情用）
type SessionTimeline struct {
	TraceID string          `json:"trace_id"`
	Events  []TimelineEvent `json:"events"`
	Summary SessionSummary  `json:"summary"`
}

// TimelineEvent 时间线上的一个事件
type TimelineEvent struct {
	ID         int64   `json:"id"`
	Timestamp  string  `json:"timestamp"`
	Type       string  `json:"type"`
	Direction  string  `json:"direction,omitempty"`
	Action     string  `json:"action,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Content    string  `json:"content,omitempty"`
	SenderID   string  `json:"sender_id,omitempty"`
	Model      string  `json:"model,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
	LatencyMs  float64 `json:"latency_ms,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	HasToolUse bool    `json:"has_tool_use,omitempty"`
	ToolCount  int     `json:"tool_count,omitempty"`
	ErrorMsg   string  `json:"error_message,omitempty"`
	CanaryLeak bool    `json:"canary_leaked,omitempty"`
	BudgetOver bool    `json:"budget_exceeded,omitempty"`
	ToolName   string  `json:"tool_name,omitempty"`
	ToolInput  string  `json:"tool_input,omitempty"`
	ToolResult string  `json:"tool_result,omitempty"`
	RiskLevel  string  `json:"risk_level,omitempty"`
	Flagged    bool    `json:"flagged,omitempty"`
	FlagReason string  `json:"flag_reason,omitempty"`
	TagText    string  `json:"tag_text,omitempty"`
	TagAuthor  string  `json:"tag_author,omitempty"`
	TagID      int64   `json:"tag_id,omitempty"`
}

// GetTimeline 获取某个 trace_id 的完整时间线
func (e *SessionReplayEngine) GetTimeline(traceID string) (*SessionTimeline, error) {
	if traceID == "" {
		return nil, fmt.Errorf("trace_id is required")
	}

	var events []TimelineEvent

	// 1. 从 audit_log 查 IM 事件
	rows, err := e.db.Query(`SELECT id, timestamp, direction, COALESCE(sender_id,''), action, COALESCE(reason,''), COALESCE(content_preview,''), COALESCE(latency_ms,0)
		FROM audit_log WHERE trace_id=? ORDER BY timestamp ASC`, traceID)
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

	// 2. 从 llm_calls 查 LLM 调用
	llmRows, err := e.db.Query(`SELECT id, timestamp, COALESCE(model,''), COALESCE(request_tokens,0), COALESCE(response_tokens,0), COALESCE(total_tokens,0), COALESCE(latency_ms,0), COALESCE(status_code,0), COALESCE(has_tool_use,0), COALESCE(tool_count,0), COALESCE(error_message,''), COALESCE(canary_leaked,0), COALESCE(budget_exceeded,0)
		FROM llm_calls WHERE trace_id=? ORDER BY timestamp ASC`, traceID)
	if err == nil {
		defer llmRows.Close()
		for llmRows.Next() {
			var ev TimelineEvent
			var hasToolUse, canaryLeaked, budgetExceeded int
			if llmRows.Scan(&ev.ID, &ev.Timestamp, &ev.Model, &ev.Tokens, &ev.StatusCode, &ev.Tokens, &ev.LatencyMs, &ev.StatusCode, &hasToolUse, &ev.ToolCount, &ev.ErrorMsg, &canaryLeaked, &budgetExceeded) == nil {
				ev.Type = "llm_call"
				ev.HasToolUse = hasToolUse != 0
				ev.CanaryLeak = canaryLeaked != 0
				ev.BudgetOver = budgetExceeded != 0
				events = append(events, ev)
			}
		}
	}

	// Collect llm_call IDs for this trace
	var llmCallIDs []int64
	idRows, err := e.db.Query(`SELECT id FROM llm_calls WHERE trace_id=?`, traceID)
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
		toolQuery := fmt.Sprintf(`SELECT id, timestamp, tool_name, COALESCE(tool_input_preview,''), COALESCE(tool_result_preview,''), COALESCE(risk_level,'low'), COALESCE(flagged,0), COALESCE(flag_reason,'')
			FROM llm_tool_calls WHERE llm_call_id IN (%s) ORDER BY timestamp ASC`, strings.Join(placeholders, ","))
		toolRows, err := e.db.Query(toolQuery, args...)
		if err == nil {
			defer toolRows.Close()
			for toolRows.Next() {
				var ev TimelineEvent
				var flagged int
				if toolRows.Scan(&ev.ID, &ev.Timestamp, &ev.ToolName, &ev.ToolInput, &ev.ToolResult, &ev.RiskLevel, &flagged, &ev.FlagReason) == nil {
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
		TraceID:   traceID,
		RiskLevel: "low",
		Tags:      []string{},
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

// ListSessions 列出会话（分页）
func (e *SessionReplayEngine) ListSessions(from, to, senderID, riskLevel, q string, limit, offset int) ([]SessionSummary, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	// 策略：先从 llm_calls 按 trace_id 分组获取会话列表，再联查 audit_log
	// 使用 UNION 合并两个来源的 trace_id
	whereClause := "WHERE trace_id IS NOT NULL AND trace_id != ''"
	var args []interface{}
	if from != "" {
		whereClause += " AND timestamp >= ?"
		args = append(args, from)
	}
	if to != "" {
		whereClause += " AND timestamp <= ?"
		args = append(args, to)
	}

	// 获取所有唯一 trace_id（合并两个来源）
	traceQuery := fmt.Sprintf(`
		SELECT DISTINCT trace_id, MIN(timestamp) as first_ts FROM (
			SELECT trace_id, timestamp FROM llm_calls %s
			UNION ALL
			SELECT trace_id, timestamp FROM audit_log %s
		) combined GROUP BY trace_id ORDER BY first_ts DESC
	`, whereClause, whereClause)

	allArgs := append(args, args...)

	rows, err := e.db.Query(traceQuery, allArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	var allTraceIDs []string
	for rows.Next() {
		var tid, ts string
		if rows.Scan(&tid, &ts) == nil && tid != "" {
			allTraceIDs = append(allTraceIDs, tid)
		}
	}

	// 对每个 trace_id 快速构建摘要
	var summaries []SessionSummary
	for _, tid := range allTraceIDs {
		sum := e.quickSummary(tid)
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
			q = strings.ToLower(q)
			if !strings.Contains(strings.ToLower(sum.TraceID), q) &&
				!strings.Contains(strings.ToLower(sum.SenderID), q) &&
				!strings.Contains(strings.ToLower(sum.Model), q) {
				// 还需在内容中搜索
				found := false
				var contentSample string
				e.db.QueryRow(`SELECT COALESCE(content_preview,'') FROM audit_log WHERE trace_id=? LIMIT 1`, tid).Scan(&contentSample)
				if strings.Contains(strings.ToLower(contentSample), q) {
					found = true
				}
				if !found {
					var toolName string
					// 搜索工具名
					idRow := e.db.QueryRow(`SELECT id FROM llm_calls WHERE trace_id=? LIMIT 1`, tid)
					var callID int64
					if idRow.Scan(&callID) == nil {
						e.db.QueryRow(`SELECT COALESCE(tool_name,'') FROM llm_tool_calls WHERE llm_call_id=? LIMIT 1`, callID).Scan(&toolName)
						if strings.Contains(strings.ToLower(toolName), q) {
							found = true
						}
					}
				}
				if !found {
					continue
				}
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
func (e *SessionReplayEngine) ListSessionsTenant(from, to, senderID, riskLevel, q, tenantID string, limit, offset int) ([]SessionSummary, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	tClause, tArgs := TenantFilter(tenantID)

	whereClause := "WHERE trace_id IS NOT NULL AND trace_id != ''" + tClause
	var args []interface{}
	args = append(args, tArgs...)
	if from != "" {
		whereClause += " AND timestamp >= ?"
		args = append(args, from)
	}
	if to != "" {
		whereClause += " AND timestamp <= ?"
		args = append(args, to)
	}

	// Get unique trace IDs from both sources
	traceQuery := fmt.Sprintf(`
		SELECT DISTINCT trace_id, MIN(timestamp) as first_ts FROM (
			SELECT trace_id, timestamp FROM llm_calls %s
			UNION ALL
			SELECT trace_id, timestamp FROM audit_log %s
		) combined GROUP BY trace_id ORDER BY first_ts DESC
	`, whereClause, whereClause)

	allArgs := append(args, args...)

	rows, err := e.db.Query(traceQuery, allArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query traces: %w", err)
	}
	defer rows.Close()

	var allTraceIDs []string
	for rows.Next() {
		var tid, ts string
		if rows.Scan(&tid, &ts) == nil && tid != "" {
			allTraceIDs = append(allTraceIDs, tid)
		}
	}

	var summaries []SessionSummary
	for _, tid := range allTraceIDs {
		sum := e.quickSummary(tid)
		if sum == nil {
			continue
		}
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
			q = strings.ToLower(q)
			if !strings.Contains(strings.ToLower(sum.TraceID), q) &&
				!strings.Contains(strings.ToLower(sum.SenderID), q) &&
				!strings.Contains(strings.ToLower(sum.Model), q) {
				continue
			}
		}
		summaries = append(summaries, *sum)
	}

	total := len(summaries)
	if offset >= len(summaries) {
		return []SessionSummary{}, total, nil
	}
	end := offset + limit
	if end > len(summaries) {
		end = len(summaries)
	}
	return summaries[offset:end], total, nil
}

// quickSummary 快速构建某个 trace_id 的摘要（不查标签以加速列表查询）
func (e *SessionReplayEngine) quickSummary(traceID string) *SessionSummary {
	s := &SessionSummary{
		TraceID:   traceID,
		RiskLevel: "low",
		Tags:      []string{},
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
		}
	}
	s.ToolCalls = toolCount
	s.FlaggedTools = flaggedCount

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

	return s
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
