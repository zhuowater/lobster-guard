// tool_audit.go — ToolCallAuditor: 解析出站响应中的 tool_call，审计工具调用
// lobster-guard v9.0
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ToolCallRecord 一次工具调用记录
type ToolCallRecord struct {
	ID            int     `json:"id,omitempty"`
	Timestamp     string  `json:"timestamp"`
	TraceID       string  `json:"trace_id"`
	SenderID      string  `json:"sender_id"`
	AppID         string  `json:"app_id"`
	ToolName      string  `json:"tool_name"`
	InputPreview  string  `json:"tool_input_preview"`
	ResultPreview string  `json:"tool_result_preview"`
	DurationMs    float64 `json:"duration_ms"`
	RiskLevel     string  `json:"risk_level"`
	Flagged       bool    `json:"flagged"`
	FlagReason    string  `json:"flag_reason,omitempty"`
}

// ToolCallAuditor 工具调用审计器
type ToolCallAuditor struct {
	db           *sql.DB
	criticalRisk map[string]bool
	highRisk     map[string]bool
	mediumRisk   map[string]bool
}

// NewToolCallAuditor 创建工具调用审计器
func NewToolCallAuditor(db *sql.DB) (*ToolCallAuditor, error) {
	// 创建 tool_calls 表
	schema := `
	CREATE TABLE IF NOT EXISTS tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		trace_id TEXT,
		sender_id TEXT,
		app_id TEXT,
		tool_name TEXT NOT NULL,
		tool_input_preview TEXT,
		tool_result_preview TEXT,
		duration_ms REAL,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0,
		flag_reason TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_tool_calls_ts ON tool_calls(timestamp);
	CREATE INDEX IF NOT EXISTS idx_tool_calls_tool ON tool_calls(tool_name);
	CREATE INDEX IF NOT EXISTS idx_tool_calls_risk ON tool_calls(risk_level);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("创建 tool_calls 表失败: %w", err)
	}

	return &ToolCallAuditor{
		db: db,
		criticalRisk: map[string]bool{
			"exec": true, "shell": true, "bash": true,
			"run_command": true, "execute_command": true,
		},
		highRisk: map[string]bool{
			"write_file": true, "edit_file": true, "delete_file": true,
			"write": true, "edit": true,
			"http_request": true, "curl": true, "web_fetch": true,
			"send_email": true, "send_message": true, "message": true,
		},
		mediumRisk: map[string]bool{
			"read_file": true, "read": true, "list_directory": true,
			"web_search": true, "browser": true,
		},
	}, nil
}

// ClassifyRisk 根据工具名返回风险等级
func (ta *ToolCallAuditor) ClassifyRisk(toolName string) string {
	name := strings.ToLower(toolName)
	if ta.criticalRisk[name] {
		return "critical"
	}
	if ta.highRisk[name] {
		return "high"
	}
	if ta.mediumRisk[name] {
		return "medium"
	}
	return "low"
}

// Record 写入一条工具调用记录
func (ta *ToolCallAuditor) Record(rec ToolCallRecord) {
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if rec.RiskLevel == "" {
		rec.RiskLevel = ta.ClassifyRisk(rec.ToolName)
	}
	flagged := 0
	if rec.Flagged {
		flagged = 1
	}
	_, err := ta.db.Exec(`INSERT INTO tool_calls
		(timestamp, trace_id, sender_id, app_id, tool_name, tool_input_preview, tool_result_preview, duration_ms, risk_level, flagged, flag_reason)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		rec.Timestamp, rec.TraceID, rec.SenderID, rec.AppID,
		rec.ToolName, truncateStr(rec.InputPreview, 200), truncateStr(rec.ResultPreview, 200),
		rec.DurationMs, rec.RiskLevel, flagged, rec.FlagReason,
	)
	if err != nil {
		log.Printf("[ToolAudit] 写入 tool_call 失败: %v", err)
	}
}

// ParseResponse 从出站响应 body 中解析 tool_call 记录
func (ta *ToolCallAuditor) ParseResponse(body []byte, senderID, appID, traceID string) []ToolCallRecord {
	if len(body) == 0 {
		return nil
	}

	var records []ToolCallRecord

	// 尝试解析为 JSON
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	// 递归查找 tool_use 模式
	ta.extractToolCalls(raw, senderID, appID, traceID, &records)

	return records
}

// extractToolCalls 递归遍历 JSON 结构，提取工具调用记录
func (ta *ToolCallAuditor) extractToolCalls(v interface{}, senderID, appID, traceID string, records *[]ToolCallRecord) {
	switch val := v.(type) {
	case map[string]interface{}:
		// 检查是否是 tool_use 对象
		if ta.isToolUseObject(val) {
			rec := ta.extractFromToolUse(val, senderID, appID, traceID)
			if rec != nil {
				*records = append(*records, *rec)
			}
		}

		// 检查是否包含 function_call 字段
		if fc, ok := val["function_call"]; ok {
			if fcMap, ok := fc.(map[string]interface{}); ok {
				rec := ta.extractFromFunctionCall(fcMap, senderID, appID, traceID)
				if rec != nil {
					*records = append(*records, *rec)
				}
			}
		}

		// 检查 tool_calls 数组字段
		if tc, ok := val["tool_calls"]; ok {
			if arr, ok := tc.([]interface{}); ok {
				for _, item := range arr {
					ta.extractToolCalls(item, senderID, appID, traceID, records)
				}
			}
		}

		// 检查 tools 数组字段
		if tc, ok := val["tools"]; ok {
			if arr, ok := tc.([]interface{}); ok {
				for _, item := range arr {
					ta.extractToolCalls(item, senderID, appID, traceID, records)
				}
			}
		}

		// 递归遍历所有值
		for k, child := range val {
			// 跳过已处理的特殊字段
			if k == "tool_calls" || k == "tools" || k == "function_call" {
				continue
			}
			ta.extractToolCalls(child, senderID, appID, traceID, records)
		}

	case []interface{}:
		for _, item := range val {
			ta.extractToolCalls(item, senderID, appID, traceID, records)
		}
	}
}

// isToolUseObject 判断对象是否是 tool_use 模式
func (ta *ToolCallAuditor) isToolUseObject(obj map[string]interface{}) bool {
	// 模式1: {"type":"tool_use", "name":"xxx", ...}
	if t, ok := obj["type"]; ok {
		if ts, ok := t.(string); ok && ts == "tool_use" {
			if _, ok := obj["name"]; ok {
				return true
			}
		}
	}
	// 模式2: {"type":"function", "function":{"name":"xxx"}}
	if t, ok := obj["type"]; ok {
		if ts, ok := t.(string); ok && ts == "function" {
			if fn, ok := obj["function"]; ok {
				if fnMap, ok := fn.(map[string]interface{}); ok {
					if _, ok := fnMap["name"]; ok {
						return true
					}
				}
			}
		}
	}
	return false
}

// extractFromToolUse 从 tool_use 对象提取记录
func (ta *ToolCallAuditor) extractFromToolUse(obj map[string]interface{}, senderID, appID, traceID string) *ToolCallRecord {
	var toolName string
	var inputPreview string

	// 模式1: {"type":"tool_use", "name":"xxx", "input":{...}}
	if name, ok := obj["name"]; ok {
		if nameStr, ok := name.(string); ok {
			toolName = nameStr
		}
	}

	// 模式2: {"type":"function", "function":{"name":"xxx", "arguments":"..."}}
	if toolName == "" {
		if fn, ok := obj["function"]; ok {
			if fnMap, ok := fn.(map[string]interface{}); ok {
				if name, ok := fnMap["name"]; ok {
					if nameStr, ok := name.(string); ok {
						toolName = nameStr
					}
				}
				if args, ok := fnMap["arguments"]; ok {
					inputPreview = fmt.Sprintf("%v", args)
				}
			}
		}
	}

	if toolName == "" {
		return nil
	}

	// 提取 input
	if inputPreview == "" {
		if input, ok := obj["input"]; ok {
			b, err := json.Marshal(input)
			if err == nil {
				inputPreview = string(b)
			}
		}
	}

	rec := &ToolCallRecord{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TraceID:      traceID,
		SenderID:     senderID,
		AppID:        appID,
		ToolName:     toolName,
		InputPreview: truncateStr(inputPreview, 200),
		RiskLevel:    ta.ClassifyRisk(toolName),
	}

	return rec
}

// extractFromFunctionCall 从 function_call 对象提取记录
func (ta *ToolCallAuditor) extractFromFunctionCall(obj map[string]interface{}, senderID, appID, traceID string) *ToolCallRecord {
	var toolName string
	var inputPreview string

	if name, ok := obj["name"]; ok {
		if nameStr, ok := name.(string); ok {
			toolName = nameStr
		}
	}
	if toolName == "" {
		return nil
	}

	if args, ok := obj["arguments"]; ok {
		inputPreview = fmt.Sprintf("%v", args)
	}

	rec := &ToolCallRecord{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TraceID:      traceID,
		SenderID:     senderID,
		AppID:        appID,
		ToolName:     toolName,
		InputPreview: truncateStr(inputPreview, 200),
		RiskLevel:    ta.ClassifyRisk(toolName),
	}

	return rec
}

// truncateStr 截断字符串到指定最大字符数
func truncateStr(s string, maxChars int) string {
	rs := []rune(s)
	if len(rs) > maxChars {
		return string(rs[:maxChars]) + "..."
	}
	return s
}

// ============================================================
// 查询 API
// ============================================================

// QueryToolCalls 查询工具调用列表（分页+筛选）
func (ta *ToolCallAuditor) QueryToolCalls(tool, risk, sender, from, to string, limit, offset int) ([]ToolCallRecord, int, error) {
	where := "WHERE 1=1"
	var args []interface{}

	if tool != "" {
		where += " AND tool_name=?"
		args = append(args, tool)
	}
	if risk != "" {
		risks := strings.Split(risk, ",")
		placeholders := make([]string, len(risks))
		for i, r := range risks {
			placeholders[i] = "?"
			args = append(args, strings.TrimSpace(r))
		}
		where += " AND risk_level IN (" + strings.Join(placeholders, ",") + ")"
	}
	if sender != "" {
		where += " AND sender_id=?"
		args = append(args, sender)
	}
	if from != "" {
		where += " AND timestamp>=?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND timestamp<=?"
		args = append(args, to)
	}

	// Total count
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := ta.db.QueryRow("SELECT COUNT(*) FROM tool_calls "+where, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Query with pagination
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	query := "SELECT id, timestamp, trace_id, sender_id, app_id, tool_name, tool_input_preview, tool_result_preview, duration_ms, risk_level, flagged, COALESCE(flag_reason,'') FROM tool_calls " + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := ta.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []ToolCallRecord
	for rows.Next() {
		var rec ToolCallRecord
		var flagged int
		if err := rows.Scan(&rec.ID, &rec.Timestamp, &rec.TraceID, &rec.SenderID, &rec.AppID, &rec.ToolName, &rec.InputPreview, &rec.ResultPreview, &rec.DurationMs, &rec.RiskLevel, &flagged, &rec.FlagReason); err != nil {
			continue
		}
		rec.Flagged = flagged != 0
		records = append(records, rec)
	}
	return records, total, nil
}

// ToolCallStats 工具调用统计
type ToolCallStats struct {
	TotalCalls   int                      `json:"total_calls"`
	ByTool       []map[string]interface{} `json:"by_tool"`
	ByRisk       map[string]int           `json:"by_risk"`
	HighRisk24h  int                      `json:"high_risk_24h"`
	FlaggedCount int                      `json:"flagged_count"`
}

// Stats 返回工具调用统计
func (ta *ToolCallAuditor) Stats() (*ToolCallStats, error) {
	stats := &ToolCallStats{
		ByRisk: map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
	}

	// Total
	ta.db.QueryRow("SELECT COUNT(*) FROM tool_calls").Scan(&stats.TotalCalls)

	// By tool
	rows, err := ta.db.Query("SELECT tool_name, COUNT(*) as cnt FROM tool_calls GROUP BY tool_name ORDER BY cnt DESC LIMIT 20")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				stats.ByTool = append(stats.ByTool, map[string]interface{}{"name": name, "count": count})
			}
		}
	}

	// By risk
	rows2, err := ta.db.Query("SELECT risk_level, COUNT(*) FROM tool_calls GROUP BY risk_level")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var level string
			var count int
			if rows2.Scan(&level, &count) == nil {
				stats.ByRisk[level] = count
			}
		}
	}

	// High risk 24h
	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	ta.db.QueryRow("SELECT COUNT(*) FROM tool_calls WHERE risk_level IN ('high','critical') AND timestamp>=?", since24h).Scan(&stats.HighRisk24h)

	// Flagged
	ta.db.QueryRow("SELECT COUNT(*) FROM tool_calls WHERE flagged=1").Scan(&stats.FlaggedCount)

	if stats.ByTool == nil {
		stats.ByTool = []map[string]interface{}{}
	}

	return stats, nil
}

// QueryHighRisk 查询高危调用
func (ta *ToolCallAuditor) QueryHighRisk(limit int) ([]ToolCallRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := ta.db.Query(`SELECT id, timestamp, trace_id, sender_id, app_id, tool_name, tool_input_preview, tool_result_preview, duration_ms, risk_level, flagged, COALESCE(flag_reason,'')
		FROM tool_calls WHERE risk_level IN ('high','critical') ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ToolCallRecord
	for rows.Next() {
		var rec ToolCallRecord
		var flagged int
		if err := rows.Scan(&rec.ID, &rec.Timestamp, &rec.TraceID, &rec.SenderID, &rec.AppID, &rec.ToolName, &rec.InputPreview, &rec.ResultPreview, &rec.DurationMs, &rec.RiskLevel, &flagged, &rec.FlagReason); err != nil {
			continue
		}
		rec.Flagged = flagged != 0
		records = append(records, rec)
	}
	return records, nil
}

// Timeline 按小时聚合工具调用
func (ta *ToolCallAuditor) Timeline(hours int) ([]map[string]interface{}, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	rows, err := ta.db.Query(`
		SELECT
			strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour_bucket,
			risk_level,
			COUNT(*) as cnt
		FROM tool_calls
		WHERE timestamp >= ?
		GROUP BY hour_bucket, risk_level
		ORDER BY hour_bucket ASC
	`, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hourMap := map[string]map[string]int{}
	for rows.Next() {
		var hour, risk string
		var count int
		if rows.Scan(&hour, &risk, &count) == nil {
			if hourMap[hour] == nil {
				hourMap[hour] = map[string]int{}
			}
			hourMap[hour][risk] = count
		}
	}

	var timeline []map[string]interface{}
	for i := hours - 1; i >= 0; i-- {
		t := time.Now().UTC().Add(-time.Duration(i) * time.Hour)
		hourKey := t.Format("2006-01-02T15") + ":00:00Z"
		entry := map[string]interface{}{
			"hour":     hourKey,
			"low":      0,
			"medium":   0,
			"high":     0,
			"critical": 0,
			"total":    0,
		}
		if m, ok := hourMap[hourKey]; ok {
			total := 0
			for risk, cnt := range m {
				entry[risk] = cnt
				total += cnt
			}
			entry["total"] = total
		}
		timeline = append(timeline, entry)
	}
	return timeline, nil
}
