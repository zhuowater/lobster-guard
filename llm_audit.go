// llm_audit.go — LLMAuditor: LLM 侧独立审计记录器
// lobster-guard v9.0
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

// LLMAuditor — LLM 侧审计记录器（独立于 IM 侧）
type LLMAuditor struct {
	db       *sql.DB
	cfg      LLMAuditConfig
	highRisk map[string]string // tool_name → risk_level
}

// LLMAuditContext 代理请求的审计上下文
type LLMAuditContext struct {
	TraceID   string
	StartTime time.Time
	Model     string
	ReqBody   []byte
}

// NewLLMAuditor 创建 LLM 审计器
func NewLLMAuditor(db *sql.DB, cfg LLMAuditConfig) *LLMAuditor {
	if cfg.MaxPreviewLen <= 0 {
		cfg.MaxPreviewLen = 500
	}
	// 默认启用工具输入/输出记录
	if !cfg.LogToolInput {
		cfg.LogToolInput = true
	}
	if !cfg.LogToolResult {
		cfg.LogToolResult = true
	}

	// 确保表存在
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		trace_id TEXT,
		model TEXT,
		request_tokens INTEGER,
		response_tokens INTEGER,
		total_tokens INTEGER,
		latency_ms REAL,
		status_code INTEGER,
		has_tool_use INTEGER DEFAULT 0,
		tool_count INTEGER DEFAULT 0,
		error_message TEXT
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_calls_ts ON llm_calls(timestamp)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER REFERENCES llm_calls(id),
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_input_preview TEXT,
		tool_result_preview TEXT,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0,
		flag_reason TEXT
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_ts ON llm_tool_calls(timestamp)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_risk ON llm_tool_calls(risk_level)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_tool ON llm_tool_calls(tool_name)`)

	return &LLMAuditor{
		db:  db,
		cfg: cfg,
		highRisk: map[string]string{
			// critical
			"exec": "critical", "shell": "critical", "bash": "critical",
			"run_command": "critical", "execute_command": "critical",
			// high
			"write_file": "high", "edit_file": "high", "delete_file": "high",
			"http_request": "high", "curl": "high", "web_fetch": "high",
			"send_email": "high", "send_message": "high",
			// medium
			"read_file": "medium", "read": "medium", "list_directory": "medium",
			"web_search": "medium", "browser": "medium",
		},
	}
}

// ClassifyToolRisk 根据工具名返回风险等级
func (la *LLMAuditor) ClassifyToolRisk(toolName string) string {
	name := strings.ToLower(toolName)
	if level, ok := la.highRisk[name]; ok {
		return level
	}
	return "low"
}

// RecordCall 写入一条 LLM 调用记录，返回插入的 ID
func (la *LLMAuditor) RecordCall(ts string, traceID, model string, reqTokens, respTokens, totalTokens int, latencyMs float64, statusCode int, hasToolUse bool, toolCount int, errMsg string) (int64, error) {
	toolUse := 0
	if hasToolUse {
		toolUse = 1
	}
	result, err := la.db.Exec(`INSERT INTO llm_calls
		(timestamp, trace_id, model, request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, error_message)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		ts, traceID, model, reqTokens, respTokens, totalTokens, latencyMs, statusCode, toolUse, toolCount, errMsg)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// RecordToolCall 写入一条工具调用记录
func (la *LLMAuditor) RecordToolCall(llmCallID int64, ts, toolName, inputPreview, resultPreview string) error {
	riskLevel := la.ClassifyToolRisk(toolName)
	flagged := 0
	flagReason := ""
	if riskLevel == "critical" {
		flagged = 1
		flagReason = "高危工具: " + toolName
	}
	if !la.cfg.LogToolInput {
		inputPreview = ""
	}
	if !la.cfg.LogToolResult {
		resultPreview = ""
	}
	if la.cfg.MaxPreviewLen > 0 {
		inputPreview = truncateRunes(inputPreview, la.cfg.MaxPreviewLen)
		resultPreview = truncateRunes(resultPreview, la.cfg.MaxPreviewLen)
	}
	_, err := la.db.Exec(`INSERT INTO llm_tool_calls
		(llm_call_id, timestamp, tool_name, tool_input_preview, tool_result_preview, risk_level, flagged, flag_reason)
		VALUES (?,?,?,?,?,?,?,?)`,
		llmCallID, ts, toolName, inputPreview, resultPreview, riskLevel, flagged, flagReason)
	return err
}

// truncateRunes 截断字符串到指定字符数
func truncateRunes(s string, maxChars int) string {
	rs := []rune(s)
	if len(rs) > maxChars {
		return string(rs[:maxChars]) + "..."
	}
	return s
}

// ============================================================
// 解析器
// ============================================================

// ParseAnthropicRequest 提取 Anthropic 请求中的 model
func ParseAnthropicRequest(body []byte) (model string) {
	var req map[string]interface{}
	if json.Unmarshal(body, &req) != nil {
		return ""
	}
	if m, ok := req["model"].(string); ok {
		model = m
	}
	return
}

// AnthropicResponseInfo 从响应中提取的信息
type AnthropicResponseInfo struct {
	Model          string
	InputTokens    int
	OutputTokens   int
	TotalTokens    int
	HasToolUse     bool
	ToolCount      int
	ToolNames      []string
	ToolInputs     []string
	ErrorMessage   string
}

// ParseAnthropicResponse 提取 Anthropic 响应中的 content、usage
func ParseAnthropicResponse(body []byte) *AnthropicResponseInfo {
	var resp map[string]interface{}
	if json.Unmarshal(body, &resp) != nil {
		return nil
	}

	info := &AnthropicResponseInfo{}

	// model
	if m, ok := resp["model"].(string); ok {
		info.Model = m
	}

	// error
	if errObj, ok := resp["error"]; ok {
		if errMap, ok := errObj.(map[string]interface{}); ok {
			if msg, ok := errMap["message"].(string); ok {
				info.ErrorMessage = msg
			}
		}
	}

	// usage
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if v, ok := usage["input_tokens"].(float64); ok {
			info.InputTokens = int(v)
		}
		if v, ok := usage["output_tokens"].(float64); ok {
			info.OutputTokens = int(v)
		}
		info.TotalTokens = info.InputTokens + info.OutputTokens
	}

	// content — scan for tool_use
	if content, ok := resp["content"].([]interface{}); ok {
		for _, item := range content {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["type"].(string); ok && t == "tool_use" {
					info.HasToolUse = true
					info.ToolCount++
					if name, ok := m["name"].(string); ok {
						info.ToolNames = append(info.ToolNames, name)
					}
					if input, ok := m["input"]; ok {
						b, _ := json.Marshal(input)
						info.ToolInputs = append(info.ToolInputs, string(b))
					} else {
						info.ToolInputs = append(info.ToolInputs, "")
					}
				}
			}
		}
	}

	// OpenAI format: choices[].message.tool_calls
	if choices, ok := resp["choices"].([]interface{}); ok {
		for _, choice := range choices {
			if cm, ok := choice.(map[string]interface{}); ok {
				if msg, ok := cm["message"].(map[string]interface{}); ok {
					if tcs, ok := msg["tool_calls"].([]interface{}); ok {
						for _, tc := range tcs {
							if tcm, ok := tc.(map[string]interface{}); ok {
								if fn, ok := tcm["function"].(map[string]interface{}); ok {
									info.HasToolUse = true
									info.ToolCount++
									if name, ok := fn["name"].(string); ok {
										info.ToolNames = append(info.ToolNames, name)
									}
									if args, ok := fn["arguments"].(string); ok {
										info.ToolInputs = append(info.ToolInputs, args)
									} else {
										info.ToolInputs = append(info.ToolInputs, "")
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return info
}

// ParseSSEEvents 从 SSE 事件缓冲中拼装完整响应再解析
func ParseSSEEvents(events []byte) *AnthropicResponseInfo {
	info := &AnthropicResponseInfo{}
	lines := strings.Split(string(events), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}

		var evt map[string]interface{}
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}

		evtType, _ := evt["type"].(string)

		switch evtType {
		case "message_start":
			if msg, ok := evt["message"].(map[string]interface{}); ok {
				if m, ok := msg["model"].(string); ok {
					info.Model = m
				}
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					if v, ok := usage["input_tokens"].(float64); ok {
						info.InputTokens = int(v)
					}
				}
			}

		case "content_block_start":
			if cb, ok := evt["content_block"].(map[string]interface{}); ok {
				if t, ok := cb["type"].(string); ok && t == "tool_use" {
					info.HasToolUse = true
					info.ToolCount++
					if name, ok := cb["name"].(string); ok {
						info.ToolNames = append(info.ToolNames, name)
						info.ToolInputs = append(info.ToolInputs, "")
					}
				}
			}

		case "content_block_delta":
			if delta, ok := evt["delta"].(map[string]interface{}); ok {
				if t, ok := delta["type"].(string); ok && t == "input_json_delta" {
					if partialJSON, ok := delta["partial_json"].(string); ok {
						if len(info.ToolInputs) > 0 {
							info.ToolInputs[len(info.ToolInputs)-1] += partialJSON
						}
					}
				}
			}

		case "message_delta":
			if usage, ok := evt["usage"].(map[string]interface{}); ok {
				if v, ok := usage["output_tokens"].(float64); ok {
					info.OutputTokens = int(v)
				}
			}
		}
	}

	info.TotalTokens = info.InputTokens + info.OutputTokens
	return info
}

// ProcessResponse 处理完整的非流式响应
func (la *LLMAuditor) ProcessResponse(ctx *LLMAuditContext, statusCode int, respBody []byte) {
	defer func() { recover() }()

	latencyMs := float64(time.Since(ctx.StartTime).Microseconds()) / 1000.0
	ts := time.Now().UTC().Format(time.RFC3339)

	// 解析请求获取 model
	model := ctx.Model
	if model == "" {
		model = ParseAnthropicRequest(ctx.ReqBody)
	}

	// 解析响应
	info := ParseAnthropicResponse(respBody)
	if info == nil {
		info = &AnthropicResponseInfo{}
	}
	if info.Model != "" {
		model = info.Model
	}

	errMsg := ""
	if statusCode >= 400 {
		errMsg = info.ErrorMessage
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", statusCode)
		}
	}

	callID, err := la.RecordCall(ts, ctx.TraceID, model, info.InputTokens, info.OutputTokens, info.TotalTokens, latencyMs, statusCode, info.HasToolUse, info.ToolCount, errMsg)
	if err != nil {
		log.Printf("[LLMAudit] 写入 llm_call 失败: %v", err)
		return
	}

	// 记录工具调用
	for i, toolName := range info.ToolNames {
		inputPreview := ""
		if i < len(info.ToolInputs) {
			inputPreview = info.ToolInputs[i]
		}
		if err := la.RecordToolCall(callID, ts, toolName, inputPreview, ""); err != nil {
			log.Printf("[LLMAudit] 写入 llm_tool_call 失败: %v", err)
		}
	}
}

// ProcessSSEBuffer 处理 SSE 流式响应缓冲
func (la *LLMAuditor) ProcessSSEBuffer(ctx *LLMAuditContext, events []byte) {
	defer func() { recover() }()

	latencyMs := float64(time.Since(ctx.StartTime).Microseconds()) / 1000.0
	ts := time.Now().UTC().Format(time.RFC3339)

	model := ctx.Model
	if model == "" {
		model = ParseAnthropicRequest(ctx.ReqBody)
	}

	info := ParseSSEEvents(events)
	if info == nil {
		info = &AnthropicResponseInfo{}
	}
	if info.Model != "" {
		model = info.Model
	}

	callID, err := la.RecordCall(ts, ctx.TraceID, model, info.InputTokens, info.OutputTokens, info.TotalTokens, latencyMs, 200, info.HasToolUse, info.ToolCount, "")
	if err != nil {
		log.Printf("[LLMAudit] 写入 llm_call(SSE) 失败: %v", err)
		return
	}

	for i, toolName := range info.ToolNames {
		inputPreview := ""
		if i < len(info.ToolInputs) {
			inputPreview = info.ToolInputs[i]
		}
		if err := la.RecordToolCall(callID, ts, toolName, inputPreview, ""); err != nil {
			log.Printf("[LLMAudit] 写入 llm_tool_call(SSE) 失败: %v", err)
		}
	}
}

// ============================================================
// 查询 API
// ============================================================

// Overview 返回 LLM 概览统计
func (la *LLMAuditor) Overview() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"total_calls":       0,
		"total_tokens":      0,
		"input_tokens":      0,
		"output_tokens":     0,
		"estimated_cost_usd": 0.0,
		"avg_latency_ms":    0.0,
		"error_rate":        0.0,
		"tool_calls_total":  0,
		"high_risk_24h":     0,
		"models":            []map[string]interface{}{},
	}

	var totalCalls int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_calls").Scan(&totalCalls)
	result["total_calls"] = totalCalls

	var totalTokens, inputTokens, outputTokens sql.NullInt64
	la.db.QueryRow("SELECT COALESCE(SUM(total_tokens),0), COALESCE(SUM(request_tokens),0), COALESCE(SUM(response_tokens),0) FROM llm_calls").Scan(&totalTokens, &inputTokens, &outputTokens)
	result["total_tokens"] = totalTokens.Int64
	result["input_tokens"] = inputTokens.Int64
	result["output_tokens"] = outputTokens.Int64

	// 成本估算（粗略：input $3/MTok, output $15/MTok for Claude Sonnet）
	cost := float64(inputTokens.Int64)*3.0/1000000.0 + float64(outputTokens.Int64)*15.0/1000000.0
	result["estimated_cost_usd"] = float64(int(cost*100)) / 100

	var avgLatency sql.NullFloat64
	la.db.QueryRow("SELECT AVG(latency_ms) FROM llm_calls").Scan(&avgLatency)
	if avgLatency.Valid {
		result["avg_latency_ms"] = float64(int(avgLatency.Float64*10)) / 10
	}

	var errorCount int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_calls WHERE status_code >= 400 OR error_message != ''").Scan(&errorCount)
	if totalCalls > 0 {
		result["error_rate"] = float64(int(float64(errorCount)/float64(totalCalls)*10000)) / 10000
	}

	var toolCallsTotal int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls").Scan(&toolCallsTotal)
	result["tool_calls_total"] = toolCallsTotal

	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	var highRisk24h int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp>=?", since24h).Scan(&highRisk24h)
	result["high_risk_24h"] = highRisk24h

	// 模型分布
	rows, err := la.db.Query("SELECT model, COUNT(*) as cnt FROM llm_calls WHERE model != '' GROUP BY model ORDER BY cnt DESC LIMIT 10")
	if err == nil {
		defer rows.Close()
		var models []map[string]interface{}
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				models = append(models, map[string]interface{}{"name": name, "count": count})
			}
		}
		if models != nil {
			result["models"] = models
		}
	}

	return result, nil
}

// QueryCalls 查询 LLM 调用列表
func (la *LLMAuditor) QueryCalls(model, hasToolUse, from, to string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	var args []interface{}

	if model != "" {
		where += " AND model=?"
		args = append(args, model)
	}
	if hasToolUse == "1" || hasToolUse == "true" {
		where += " AND has_tool_use=1"
	} else if hasToolUse == "0" || hasToolUse == "false" {
		where += " AND has_tool_use=0"
	}
	if from != "" {
		where += " AND timestamp>=?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND timestamp<=?"
		args = append(args, to)
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	la.db.QueryRow("SELECT COUNT(*) FROM llm_calls "+where, countArgs...).Scan(&total)

	if limit <= 0 { limit = 50 }
	if limit > 1000 { limit = 1000 }
	query := "SELECT id, timestamp, COALESCE(trace_id,''), COALESCE(model,''), request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, COALESCE(error_message,'') FROM llm_calls " + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := la.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id, reqTok, respTok, totalTok, statusCode, hasToolUseV, toolCount int
		var ts, traceID, modelV, errMsg string
		var latencyMs float64
		if rows.Scan(&id, &ts, &traceID, &modelV, &reqTok, &respTok, &totalTok, &latencyMs, &statusCode, &hasToolUseV, &toolCount, &errMsg) != nil {
			continue
		}
		records = append(records, map[string]interface{}{
			"id": id, "timestamp": ts, "trace_id": traceID, "model": modelV,
			"request_tokens": reqTok, "response_tokens": respTok, "total_tokens": totalTok,
			"latency_ms": latencyMs, "status_code": statusCode,
			"has_tool_use": hasToolUseV != 0, "tool_count": toolCount,
			"error_message": errMsg,
		})
	}
	return records, total, nil
}

// QueryToolCalls 查询工具调用列表
func (la *LLMAuditor) QueryToolCalls(toolName, riskLevel, from, to string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	var args []interface{}

	if toolName != "" {
		where += " AND tool_name=?"
		args = append(args, toolName)
	}
	if riskLevel != "" {
		where += " AND risk_level=?"
		args = append(args, riskLevel)
	}
	if from != "" {
		where += " AND timestamp>=?"
		args = append(args, from)
	}
	if to != "" {
		where += " AND timestamp<=?"
		args = append(args, to)
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls "+where, countArgs...).Scan(&total)

	if limit <= 0 { limit = 50 }
	if limit > 1000 { limit = 1000 }
	query := "SELECT id, COALESCE(llm_call_id,0), timestamp, tool_name, COALESCE(tool_input_preview,''), COALESCE(tool_result_preview,''), risk_level, flagged, COALESCE(flag_reason,'') FROM llm_tool_calls " + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := la.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id, llmCallID, flagged int
		var ts, toolNameV, inputPreview, resultPreview, riskLevelV, flagReason string
		if rows.Scan(&id, &llmCallID, &ts, &toolNameV, &inputPreview, &resultPreview, &riskLevelV, &flagged, &flagReason) != nil {
			continue
		}
		records = append(records, map[string]interface{}{
			"id": id, "llm_call_id": llmCallID, "timestamp": ts,
			"tool_name": toolNameV, "tool_input_preview": inputPreview,
			"tool_result_preview": resultPreview, "risk_level": riskLevelV,
			"flagged": flagged != 0, "flag_reason": flagReason,
		})
	}
	return records, total, nil
}

// ToolStats 返回工具统计
func (la *LLMAuditor) ToolStats() (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"total":        0,
		"by_tool":      []map[string]interface{}{},
		"by_risk":      map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
		"high_risk_24h": 0,
		"flagged_count": 0,
	}

	var total int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls").Scan(&total)
	stats["total"] = total

	// by_tool
	rows, err := la.db.Query("SELECT tool_name, COUNT(*) as cnt FROM llm_tool_calls GROUP BY tool_name ORDER BY cnt DESC LIMIT 20")
	if err == nil {
		defer rows.Close()
		var byTool []map[string]interface{}
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				byTool = append(byTool, map[string]interface{}{"name": name, "count": count})
			}
		}
		if byTool != nil {
			stats["by_tool"] = byTool
		}
	}

	// by_risk
	rows2, err := la.db.Query("SELECT risk_level, COUNT(*) FROM llm_tool_calls GROUP BY risk_level")
	if err == nil {
		defer rows2.Close()
		byRisk := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
		for rows2.Next() {
			var level string
			var count int
			if rows2.Scan(&level, &count) == nil {
				byRisk[level] = count
			}
		}
		stats["by_risk"] = byRisk
	}

	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	var highRisk24h int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp>=?", since24h).Scan(&highRisk24h)
	stats["high_risk_24h"] = highRisk24h

	var flaggedCount int
	la.db.QueryRow("SELECT COUNT(*) FROM llm_tool_calls WHERE flagged=1").Scan(&flaggedCount)
	stats["flagged_count"] = flaggedCount

	return stats, nil
}

// ToolTimeline 按小时聚合工具调用
func (la *LLMAuditor) ToolTimeline(hours int) ([]map[string]interface{}, error) {
	if hours <= 0 { hours = 24 }
	if hours > 168 { hours = 168 }
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	rows, err := la.db.Query(`
		SELECT
			strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour_bucket,
			risk_level,
			COUNT(*) as cnt
		FROM llm_tool_calls
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

// ============================================================
// 演示数据
// ============================================================

// SeedDemoData 注入 LLM 演示数据，返回插入的 llm_calls 数和 llm_tool_calls 数
func (la *LLMAuditor) SeedDemoData(db *sql.DB) (int, int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	now := time.Now()

	models := []struct {
		name   string
		weight int
	}{
		{"claude-sonnet-4-20250514", 60},
		{"claude-opus-4-20250514", 20},
		{"gpt-4", 20},
	}
	totalModelWeight := 0
	for _, m := range models {
		totalModelWeight += m.weight
	}

	toolNames := []struct {
		name   string
		weight int
	}{
		{"exec", 15}, {"read_file", 20}, {"write_file", 10}, {"web_search", 15},
		{"web_fetch", 10}, {"browser", 10}, {"send_message", 5},
		{"read", 5}, {"edit", 3}, {"image", 2}, {"tts", 2}, {"canvas", 3},
	}
	totalToolWeight := 0
	for _, t := range toolNames {
		totalToolWeight += t.weight
	}

	inputSamples := map[string][]string{
		"exec":         {`{"command":"ls -la /tmp"}`, `{"command":"cat /etc/passwd"}`, `{"command":"ps aux"}`},
		"read_file":    {`{"path":"/tmp/data.txt"}`, `{"path":"config.yaml"}`, `{"path":"main.go"}`},
		"write_file":   {`{"path":"/tmp/out.txt","content":"..."}`, `{"path":"result.json"}`},
		"web_search":   {`{"query":"AI安全最新论文"}`, `{"query":"prompt injection"}`},
		"web_fetch":    {`{"url":"https://example.com"}`, `{"url":"https://api.github.com"}`},
		"browser":      {`{"action":"navigate","url":"https://example.com"}`, `{"action":"screenshot"}`},
		"send_message": {`{"target":"user-1","message":"Hello"}`},
		"read":         {`{"path":"api.go","offset":1}`},
		"edit":         {`{"path":"config.yaml","old":"...","new":"..."}`},
		"image":        {`{"image":"/tmp/photo.jpg","prompt":"描述"}`},
		"tts":          {`{"text":"你好世界"}`},
		"canvas":       {`{"action":"present","url":"http://localhost:3000"}`},
	}

	callCount := 80 + rng.Intn(41) // 80-120
	callsInserted := 0
	toolsInserted := 0

	tx, err := db.Begin()
	if err != nil {
		return 0, 0
	}

	callStmt, err := tx.Prepare(`INSERT INTO llm_calls
		(timestamp, trace_id, model, request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, error_message)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return 0, 0
	}

	toolStmt, err := tx.Prepare(`INSERT INTO llm_tool_calls
		(llm_call_id, timestamp, tool_name, tool_input_preview, tool_result_preview, risk_level, flagged, flag_reason)
		VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		callStmt.Close()
		tx.Rollback()
		return 0, 0
	}

	for i := 0; i < callCount; i++ {
		offsetSec := rng.Int63n(7 * 24 * 3600)
		ts := now.Add(-time.Duration(offsetSec) * time.Second).UTC().Format(time.RFC3339)
		traceID := fmt.Sprintf("llm-%08x%08x", rng.Uint32(), rng.Uint32())

		// 选模型
		roll := rng.Intn(totalModelWeight)
		var model string
		cum := 0
		for _, m := range models {
			cum += m.weight
			if roll < cum {
				model = m.name
				break
			}
		}

		reqTokens := 500 + rng.Intn(4501)   // 500-5000
		respTokens := 200 + rng.Intn(1801)   // 200-2000
		totalTokens := reqTokens + respTokens
		latencyMs := 500.0 + rng.Float64()*7500.0 // 500-8000
		statusCode := 200
		errMsg := ""
		if rng.Float64() < 0.05 { // 5% 错误率
			statusCode = 500
			errMsg = "Internal server error"
		}

		// 工具调用数 0-5
		toolCount := 0
		if rng.Float64() < 0.6 { // 60% 有工具调用
			toolCount = 1 + rng.Intn(5)
		}
		hasToolUse := 0
		if toolCount > 0 {
			hasToolUse = 1
		}

		result, err := callStmt.Exec(ts, traceID, model, reqTokens, respTokens, totalTokens, latencyMs, statusCode, hasToolUse, toolCount, errMsg)
		if err != nil {
			continue
		}
		callID, _ := result.LastInsertId()
		callsInserted++

		// 插入工具调用
		for j := 0; j < toolCount; j++ {
			toolRoll := rng.Intn(totalToolWeight)
			var toolName string
			toolCum := 0
			for _, t := range toolNames {
				toolCum += t.weight
				if toolRoll < toolCum {
					toolName = t.name
					break
				}
			}

			riskLevel := la.ClassifyToolRisk(toolName)
			var inputPreview string
			if samples, ok := inputSamples[toolName]; ok && len(samples) > 0 {
				inputPreview = samples[rng.Intn(len(samples))]
			}

			flagged := 0
			flagReason := ""
			if (riskLevel == "high" || riskLevel == "critical") && rng.Float64() < 0.15 {
				flagged = 1
				flagReason = "高危工具: " + toolName
			}

			_, err := toolStmt.Exec(callID, ts, toolName, inputPreview, "", riskLevel, flagged, flagReason)
			if err == nil {
				toolsInserted++
			}
		}
	}

	callStmt.Close()
	toolStmt.Close()
	tx.Commit()

	return callsInserted, toolsInserted
}

// ClearDemoData 清除 LLM 审计数据
func (la *LLMAuditor) ClearDemoData(db *sql.DB) int64 {
	var total int64
	if r, err := db.Exec("DELETE FROM llm_tool_calls"); err == nil {
		n, _ := r.RowsAffected()
		total += n
	}
	if r, err := db.Exec("DELETE FROM llm_calls"); err == nil {
		n, _ := r.RowsAffected()
		total += n
	}
	return total
}