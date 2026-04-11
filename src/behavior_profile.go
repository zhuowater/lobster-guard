// behavior_profile.go — Agent 行为画像引擎：学习正常行为，检测突变
// lobster-guard v16.0 — 语义行为模式（Agent 做了什么序列的事，而不只是数字偏差）
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ============================================================
// 核心类型
// ============================================================

// BehaviorProfileEngine Agent 行为画像引擎
type BehaviorProfileEngine struct {
	db *sql.DB
}

// AgentProfile Agent 行为画像
type AgentProfile struct {
	AgentID         string            `json:"agent_id"`
	TenantID        string            `json:"tenant_id"`
	DisplayName     string            `json:"display_name"`
	Department      string            `json:"department"`
	TypicalTools    []ToolUsage       `json:"typical_tools"`
	AvgTokensPerReq float64           `json:"avg_tokens"`
	AvgToolsPerReq  float64           `json:"avg_tools_per_req"`
	PeakHours       []int             `json:"peak_hours"`
	CommonPatterns  []BehaviorPattern `json:"common_patterns"`
	Anomalies       []BehaviorAnomaly `json:"anomalies"`
	RiskLevel       string            `json:"risk_level"`
	LastSeen        string            `json:"last_seen"`
	TotalRequests   int               `json:"total_requests"`
	ProfiledSince   string            `json:"profiled_since"`
}

// ToolUsage 工具使用统计
type ToolUsage struct {
	ToolName   string  `json:"tool_name"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	AvgTokens  float64 `json:"avg_tokens"`
	LastUsed   string  `json:"last_used"`
}

// BehaviorPattern 行为模式（工具调用序列）
type BehaviorPattern struct {
	Sequence    []string `json:"sequence"`
	Count       int      `json:"count"`
	AvgDuration int64    `json:"avg_duration_ms"`
	RiskScore   float64  `json:"risk_score"`
}

// BehaviorAnomaly 行为突变
type BehaviorAnomaly struct {
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Details     string `json:"details"`
	TraceID     string `json:"trace_id"`
	AgentID     string `json:"agent_id"`
	TenantID    string `json:"tenant_id"`
}

// ============================================================
// 构造
// ============================================================

// NewBehaviorProfileEngine 创建行为画像引擎
func NewBehaviorProfileEngine(db *sql.DB) *BehaviorProfileEngine {
	db.Exec(`CREATE TABLE IF NOT EXISTS behavior_anomalies (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		tenant_id TEXT DEFAULT 'default',
		type TEXT NOT NULL,
		severity TEXT NOT NULL,
		description TEXT DEFAULT '',
		details TEXT DEFAULT '',
		trace_id TEXT DEFAULT '',
		acknowledged INTEGER DEFAULT 0
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_behavior_anomalies_ts ON behavior_anomalies(timestamp)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_behavior_anomalies_agent ON behavior_anomalies(agent_id)`)

	return &BehaviorProfileEngine{db: db}
}

// ============================================================
// Agent 发现
// ============================================================

func (bp *BehaviorProfileEngine) discoverAgents(tenantID string) []struct{ agentID, displayName string } {
	tClause, tArgs := TenantFilter(tenantID)
	query := `SELECT DISTINCT sender_id FROM audit_log WHERE sender_id != '' AND sender_id IS NOT NULL` + tClause + ` ORDER BY sender_id`
	rows, err := bp.db.Query(query, tArgs...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var agents []struct{ agentID, displayName string }
	for rows.Next() {
		var sid string
		if rows.Scan(&sid) == nil && sid != "" {
			agents = append(agents, struct{ agentID, displayName string }{sid, sid})
		}
	}
	return agents
}

// ============================================================
// 画像构建
// ============================================================

func (bp *BehaviorProfileEngine) BuildProfile(agentID, tenantID string) (*AgentProfile, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if tenantID == "" {
		tenantID = "default"
	}

	displayName, department := lookupSenderIdentity(bp.db, agentID)
	profile := &AgentProfile{
		AgentID:     agentID,
		TenantID:    tenantID,
		DisplayName: agentID,
		Department:  department,
		RiskLevel:   "normal",
	}
	if displayName != "" {
		profile.DisplayName = displayName
	}

	var totalReq int
	var firstSeen, lastSeen sql.NullString
	bp.db.QueryRow(`SELECT COUNT(*), MIN(timestamp), MAX(timestamp) FROM audit_log WHERE sender_id=?`, agentID).Scan(&totalReq, &firstSeen, &lastSeen)
	profile.TotalRequests = totalReq
	if firstSeen.Valid {
		profile.ProfiledSince = firstSeen.String
	}
	if lastSeen.Valid {
		profile.LastSeen = lastSeen.String
	}

	if totalReq == 0 {
		return profile, nil
	}

	// 平均 token/请求
	var avgTokens sql.NullFloat64
	bp.db.QueryRow(`SELECT AVG(total_tokens) FROM llm_calls WHERE trace_id IN (SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '')`, agentID).Scan(&avgTokens)
	if avgTokens.Valid {
		profile.AvgTokensPerReq = math.Round(avgTokens.Float64*100) / 100
	}

	// 工具使用统计
	profile.TypicalTools = bp.getToolUsage(agentID)

	// 平均工具调用/请求
	if totalReq > 0 {
		totalTools := 0
		for _, t := range profile.TypicalTools {
			totalTools += t.Count
		}
		profile.AvgToolsPerReq = math.Round(float64(totalTools)/float64(totalReq)*100) / 100
	}

	// 活跃时段
	profile.PeakHours = bp.getPeakHours(agentID)

	// 行为序列模式
	profile.CommonPatterns = bp.extractPatterns(agentID, tenantID)

	// 突变检测
	anomalies, _ := bp.DetectAnomalies(agentID, tenantID)
	profile.Anomalies = anomalies

	// 风险等级
	profile.RiskLevel = bp.calcProfileRisk(profile)

	return profile, nil
}

func (bp *BehaviorProfileEngine) getToolUsage(agentID string) []ToolUsage {
	since := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)

	rows, err := bp.db.Query(`
		SELECT tc.tool_name, COUNT(*) as cnt, MAX(tc.timestamp) as last_used, AVG(lc.total_tokens) as avg_tok
		FROM llm_tool_calls tc
		JOIN llm_calls lc ON tc.llm_call_id = lc.id
		WHERE lc.trace_id IN (
			SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ?
		)
		AND tc.timestamp >= ?
		GROUP BY tc.tool_name
		ORDER BY cnt DESC
	`, agentID, since, since)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var tools []ToolUsage
	totalCount := 0
	for rows.Next() {
		var tu ToolUsage
		var avgTok sql.NullFloat64
		var lastUsed sql.NullString
		if rows.Scan(&tu.ToolName, &tu.Count, &lastUsed, &avgTok) == nil {
			if lastUsed.Valid {
				tu.LastUsed = lastUsed.String
			}
			if avgTok.Valid {
				tu.AvgTokens = math.Round(avgTok.Float64*100) / 100
			}
			totalCount += tu.Count
			tools = append(tools, tu)
		}
	}

	for i := range tools {
		if totalCount > 0 {
			tools[i].Percentage = math.Round(float64(tools[i].Count)/float64(totalCount)*10000) / 100
		}
	}
	return tools
}

func (bp *BehaviorProfileEngine) getPeakHours(agentID string) []int {
	rows, err := bp.db.Query(`
		SELECT CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
		FROM audit_log WHERE sender_id=?
		GROUP BY h ORDER BY cnt DESC LIMIT 3
	`, agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var hours []int
	for rows.Next() {
		var h, cnt int
		if rows.Scan(&h, &cnt) == nil {
			_ = cnt
			hours = append(hours, h)
		}
	}
	sort.Ints(hours)
	return hours
}

// ============================================================
// 行为序列提取
// ============================================================

func (bp *BehaviorProfileEngine) extractPatterns(agentID, tenantID string) []BehaviorPattern {
	since := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)

	traceRows, err := bp.db.Query(`
		SELECT DISTINCT trace_id FROM audit_log
		WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ?
	`, agentID, since)
	if err != nil {
		return nil
	}
	defer traceRows.Close()

	var traceIDs []string
	for traceRows.Next() {
		var tid string
		if traceRows.Scan(&tid) == nil && tid != "" {
			traceIDs = append(traceIDs, tid)
		}
	}

	if len(traceIDs) == 0 {
		return nil
	}

	type seqInfo struct {
		seq        []string
		durationMs int64
	}
	var allSeqs []seqInfo

	for _, tid := range traceIDs {
		callRows, err := bp.db.Query(`SELECT id FROM llm_calls WHERE trace_id=? ORDER BY timestamp ASC`, tid)
		if err != nil {
			continue
		}
		var callIDs []interface{}
		var phs []string
		for callRows.Next() {
			var id int64
			if callRows.Scan(&id) == nil {
				callIDs = append(callIDs, id)
				phs = append(phs, "?")
			}
		}
		callRows.Close()

		if len(callIDs) == 0 {
			continue
		}

		toolQuery := fmt.Sprintf(`SELECT tool_name, timestamp FROM llm_tool_calls WHERE llm_call_id IN (%s) ORDER BY timestamp ASC`, strings.Join(phs, ","))
		toolRows, err := bp.db.Query(toolQuery, callIDs...)
		if err != nil {
			continue
		}

		var seq []string
		var firstTs, lastTs string
		for toolRows.Next() {
			var toolName, ts string
			if toolRows.Scan(&toolName, &ts) == nil {
				seq = append(seq, toolName)
				if firstTs == "" {
					firstTs = ts
				}
				lastTs = ts
			}
		}
		toolRows.Close()

		if len(seq) > 0 {
			dur := int64(0)
			if firstTs != "" && lastTs != "" {
				t1, e1 := time.Parse(time.RFC3339Nano, firstTs)
				t2, e2 := time.Parse(time.RFC3339Nano, lastTs)
				if e1 != nil {
					t1, _ = time.Parse(time.RFC3339, firstTs)
				}
				if e2 != nil {
					t2, _ = time.Parse(time.RFC3339, lastTs)
				}
				if !t1.IsZero() && !t2.IsZero() {
					dur = t2.Sub(t1).Milliseconds()
				}
			}
			allSeqs = append(allSeqs, seqInfo{seq: seq, durationMs: dur})
		}
	}

	type patternData struct {
		seq      []string
		count    int
		totalDur int64
	}
	patternCounts := make(map[string]*patternData)
	for _, si := range allSeqs {
		key := strings.Join(si.seq, "→")
		if p, ok := patternCounts[key]; ok {
			p.count++
			p.totalDur += si.durationMs
		} else {
			patternCounts[key] = &patternData{seq: si.seq, count: 1, totalDur: si.durationMs}
		}
	}

	var patterns []BehaviorPattern
	for _, p := range patternCounts {
		avgDur := int64(0)
		if p.count > 0 {
			avgDur = p.totalDur / int64(p.count)
		}
		patterns = append(patterns, BehaviorPattern{
			Sequence:    p.seq,
			Count:       p.count,
			AvgDuration: avgDur,
			RiskScore:   CalcSequenceRiskScore(p.seq),
		})
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})

	if len(patterns) > 10 {
		patterns = patterns[:10]
	}
	return patterns
}

// CalcSequenceRiskScore 对工具调用序列打风险分（0-100）
func CalcSequenceRiskScore(seq []string) float64 {
	if len(seq) == 0 {
		return 0
	}

	highRiskTools := map[string]float64{
		"exec": 40, "shell": 40, "bash": 40, "run_command": 40, "execute_command": 40,
		"write_file": 20, "edit_file": 15, "delete_file": 25, "write": 20, "edit": 15,
		"http_request": 15, "curl": 20, "web_fetch": 10,
		"send_email": 25, "send_message": 20, "message": 15,
		"rm": 35, "dd": 35,
	}
	readTools := map[string]bool{
		"read_file": true, "read": true, "search": true, "web_search": true,
		"list_directory": true, "summarize": true, "browser": true,
	}

	score := 0.0
	hasRead := false
	hasSend := false
	hasExec := false

	for _, tool := range seq {
		tl := strings.ToLower(tool)
		if rs, ok := highRiskTools[tl]; ok {
			score += rs
		}
		if readTools[tl] {
			hasRead = true
		}
		if tl == "send_email" || tl == "send_message" || tl == "message" || tl == "curl" || tl == "http_request" {
			hasSend = true
		}
		if tl == "exec" || tl == "shell" || tl == "bash" || tl == "run_command" || tl == "execute_command" {
			hasExec = true
		}
	}

	if hasRead && hasSend {
		score += 15
	}
	if hasRead && hasExec && hasSend {
		score += 20
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

// ============================================================
// 突变检测
// ============================================================

func (bp *BehaviorProfileEngine) DetectAnomalies(agentID, tenantID string) ([]BehaviorAnomaly, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if tenantID == "" {
		tenantID = "default"
	}

	now := time.Now().UTC()
	since24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	since7d := now.AddDate(0, 0, -7).Format(time.RFC3339)

	var anomalies []BehaviorAnomaly

	anomalies = append(anomalies, bp.detectNewTools(agentID, since24h, since7d, tenantID, now)...)

	if va := bp.detectVolumeSpike(agentID, since24h, since7d, tenantID, now); va != nil {
		anomalies = append(anomalies, *va)
	}

	if ta := bp.detectTimeAnomaly(agentID, since24h, tenantID, now); ta != nil {
		anomalies = append(anomalies, *ta)
	}

	anomalies = append(anomalies, bp.detectTokenSpike(agentID, since24h, since7d, tenantID, now)...)
	anomalies = append(anomalies, bp.detectUnusualSequences(agentID, since24h, since7d, tenantID, now)...)

	return anomalies, nil
}

func (bp *BehaviorProfileEngine) detectNewTools(agentID, since24h, since7d, tenantID string, now time.Time) []BehaviorAnomaly {
	baselineRows, err := bp.db.Query(`
		SELECT DISTINCT tc.tool_name FROM llm_tool_calls tc
		JOIN llm_calls lc ON tc.llm_call_id = lc.id
		WHERE lc.trace_id IN (
			SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ? AND timestamp < ?
		)
	`, agentID, since7d, since24h)
	if err != nil {
		return nil
	}
	defer baselineRows.Close()

	baselineTools := make(map[string]bool)
	for baselineRows.Next() {
		var tool string
		if baselineRows.Scan(&tool) == nil {
			baselineTools[tool] = true
		}
	}

	if len(baselineTools) == 0 {
		return nil
	}

	recentRows, err := bp.db.Query(`
		SELECT DISTINCT tc.tool_name, tc.timestamp FROM llm_tool_calls tc
		JOIN llm_calls lc ON tc.llm_call_id = lc.id
		WHERE lc.trace_id IN (
			SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ?
		)
		AND tc.timestamp >= ?
	`, agentID, since24h, since24h)
	if err != nil {
		return nil
	}
	defer recentRows.Close()

	var anomalies []BehaviorAnomaly
	seenNew := make(map[string]bool)
	for recentRows.Next() {
		var tool, ts string
		if recentRows.Scan(&tool, &ts) == nil {
			if !baselineTools[tool] && !seenNew[tool] {
				seenNew[tool] = true
				severity := "medium"
				if isHighRiskToolName(tool) {
					severity = "high"
				}
				anomalies = append(anomalies, BehaviorAnomaly{
					ID:          fmt.Sprintf("ba-%s-new-%s-%d", agentID, tool, now.UnixNano()%100000),
					Timestamp:   ts,
					Type:        "new_tool",
					Severity:    severity,
					Description: fmt.Sprintf("%s 首次调用 %s 工具", agentID, tool),
					Details:     fmt.Sprintf(`{"tool":"%s","baseline_tools":%d}`, tool, len(baselineTools)),
					AgentID:     agentID,
					TenantID:    tenantID,
				})
			}
		}
	}
	return anomalies
}

func (bp *BehaviorProfileEngine) detectVolumeSpike(agentID, since24h, since7d, tenantID string, now time.Time) *BehaviorAnomaly {
	var total7d int
	bp.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND timestamp >= ?`, agentID, since7d).Scan(&total7d)

	var today int
	bp.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND timestamp >= ?`, agentID, since24h).Scan(&today)

	if total7d == 0 || today == 0 {
		return nil
	}

	dailyAvg := float64(total7d) / 7.0
	if dailyAvg < 1 {
		dailyAvg = 1
	}
	ratio := float64(today) / dailyAvg

	if ratio > 2.0 {
		severity := "medium"
		if ratio > 5.0 {
			severity = "high"
		}
		if ratio > 10.0 {
			severity = "critical"
		}
		return &BehaviorAnomaly{
			ID:          fmt.Sprintf("ba-%s-volume-%d", agentID, now.UnixNano()%100000),
			Timestamp:   now.Format(time.RFC3339),
			Type:        "volume_spike",
			Severity:    severity,
			Description: fmt.Sprintf("%s 今日调用量 %d 次，是日均 %.0f 次的 %.1f 倍", agentID, today, dailyAvg, ratio),
			Details:     fmt.Sprintf(`{"today":%d,"daily_avg":%.1f,"ratio":%.1f}`, today, dailyAvg, ratio),
			AgentID:     agentID,
			TenantID:    tenantID,
		}
	}
	return nil
}

func (bp *BehaviorProfileEngine) detectTimeAnomaly(agentID, since24h, tenantID string, now time.Time) *BehaviorAnomaly {
	rows, err := bp.db.Query(`
		SELECT CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
		FROM audit_log WHERE sender_id=?
		GROUP BY h ORDER BY cnt DESC
	`, agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	hourCounts := make(map[int]int)
	totalHist := 0
	for rows.Next() {
		var h, cnt int
		if rows.Scan(&h, &cnt) == nil {
			hourCounts[h] = cnt
			totalHist += cnt
		}
	}

	if totalHist < 10 {
		return nil
	}

	recentRows, err := bp.db.Query(`
		SELECT CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
		FROM audit_log WHERE sender_id=? AND timestamp >= ?
		GROUP BY h
	`, agentID, since24h)
	if err != nil {
		return nil
	}
	defer recentRows.Close()

	for recentRows.Next() {
		var h, cnt int
		if recentRows.Scan(&h, &cnt) == nil {
			histPct := float64(hourCounts[h]) / float64(totalHist)
			if histPct < 0.02 && cnt >= 2 {
				peakHours := bp.getPeakHours(agentID)
				peakStr := "未知"
				if len(peakHours) > 0 {
					peakStr = fmt.Sprintf("%d:00-%d:00", peakHours[0], peakHours[len(peakHours)-1]+1)
				}
				return &BehaviorAnomaly{
					ID:          fmt.Sprintf("ba-%s-time-%d", agentID, now.UnixNano()%100000),
					Timestamp:   now.Format(time.RFC3339),
					Type:        "time_anomaly",
					Severity:    "medium",
					Description: fmt.Sprintf("%s 在 %d:00 活跃，通常只在 %s 活跃", agentID, h, peakStr),
					Details:     fmt.Sprintf(`{"anomaly_hour":%d,"count":%d,"usual_hours":"%s"}`, h, cnt, peakStr),
					AgentID:     agentID,
					TenantID:    tenantID,
				}
			}
		}
	}
	return nil
}

func (bp *BehaviorProfileEngine) detectTokenSpike(agentID, since24h, since7d, tenantID string, now time.Time) []BehaviorAnomaly {
	var avgTokens sql.NullFloat64
	bp.db.QueryRow(`SELECT AVG(total_tokens) FROM llm_calls WHERE trace_id IN (
		SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ? AND timestamp < ?
	)`, agentID, since7d, since24h).Scan(&avgTokens)

	if !avgTokens.Valid || avgTokens.Float64 < 100 {
		return nil
	}

	baseline := avgTokens.Float64
	threshold := baseline * 5

	rows, err := bp.db.Query(`SELECT lc.total_tokens, lc.timestamp, lc.trace_id FROM llm_calls lc WHERE lc.trace_id IN (
		SELECT DISTINCT trace_id FROM audit_log WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ?
	) AND lc.total_tokens > ? AND lc.timestamp >= ? ORDER BY lc.total_tokens DESC LIMIT 3`, agentID, since24h, int(threshold), since24h)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var anomalies []BehaviorAnomaly
	for rows.Next() {
		var tokens int
		var ts, traceID string
		if rows.Scan(&tokens, &ts, &traceID) == nil {
			anomalies = append(anomalies, BehaviorAnomaly{
				ID:          fmt.Sprintf("ba-%s-token-%d", agentID, now.UnixNano()%100000),
				Timestamp:   ts,
				Type:        "token_spike",
				Severity:    "high",
				Description: fmt.Sprintf("%s 单次请求消耗 %d token，基线平均 %.0f", agentID, tokens, baseline),
				Details:     fmt.Sprintf(`{"tokens":%d,"baseline_avg":%.0f,"ratio":%.1f}`, tokens, baseline, float64(tokens)/baseline),
				TraceID:     traceID,
				AgentID:     agentID,
				TenantID:    tenantID,
			})
		}
	}
	return anomalies
}

func (bp *BehaviorProfileEngine) detectUnusualSequences(agentID, since24h, since7d, tenantID string, now time.Time) []BehaviorAnomaly {
	// 获取基线序列
	baselineTraces, err := bp.db.Query(`
		SELECT DISTINCT trace_id FROM audit_log
		WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ? AND timestamp < ?
	`, agentID, since7d, since24h)
	if err != nil {
		return nil
	}
	defer baselineTraces.Close()

	baselineSeqs := make(map[string]bool)
	for baselineTraces.Next() {
		var tid string
		if baselineTraces.Scan(&tid) == nil {
			seq := bp.getTraceToolSequence(tid)
			if len(seq) > 0 {
				baselineSeqs[strings.Join(seq, "→")] = true
			}
		}
	}

	if len(baselineSeqs) == 0 {
		return nil
	}

	// 获取最近 24h 的序列
	recentTraces, err := bp.db.Query(`
		SELECT DISTINCT trace_id FROM audit_log
		WHERE sender_id=? AND trace_id IS NOT NULL AND trace_id != '' AND timestamp >= ?
	`, agentID, since24h)
	if err != nil {
		return nil
	}
	defer recentTraces.Close()

	var anomalies []BehaviorAnomaly
	seenSeqs := make(map[string]bool)
	for recentTraces.Next() {
		var tid string
		if recentTraces.Scan(&tid) == nil {
			seq := bp.getTraceToolSequence(tid)
			if len(seq) > 0 {
				key := strings.Join(seq, "→")
				if !baselineSeqs[key] && !seenSeqs[key] {
					seenSeqs[key] = true
					riskScore := CalcSequenceRiskScore(seq)
					severity := "low"
					if riskScore >= 50 {
						severity = "high"
					} else if riskScore >= 20 {
						severity = "medium"
					}
					anomalies = append(anomalies, BehaviorAnomaly{
						ID:          fmt.Sprintf("ba-%s-seq-%d", agentID, now.UnixNano()%100000),
						Timestamp:   now.Format(time.RFC3339),
						Type:        "unusual_sequence",
						Severity:    severity,
						Description: fmt.Sprintf("%s 出现新的调用序列: %s", agentID, strings.Join(seq, " → ")),
						Details:     fmt.Sprintf(`{"sequence":%s,"risk_score":%.1f}`, mustJSON(seq), riskScore),
						TraceID:     tid,
						AgentID:     agentID,
						TenantID:    tenantID,
					})
				}
			}
		}
	}
	return anomalies
}

// getTraceToolSequence 获取单个 trace 的工具调用序列
func (bp *BehaviorProfileEngine) getTraceToolSequence(traceID string) []string {
	callRows, err := bp.db.Query(`SELECT id FROM llm_calls WHERE trace_id=?`, traceID)
	if err != nil {
		return nil
	}
	var callIDs []interface{}
	var phs []string
	for callRows.Next() {
		var id int64
		if callRows.Scan(&id) == nil {
			callIDs = append(callIDs, id)
			phs = append(phs, "?")
		}
	}
	callRows.Close()

	if len(callIDs) == 0 {
		return nil
	}

	toolQuery := fmt.Sprintf(`SELECT tool_name FROM llm_tool_calls WHERE llm_call_id IN (%s) ORDER BY timestamp ASC`, strings.Join(phs, ","))
	toolRows, err := bp.db.Query(toolQuery, callIDs...)
	if err != nil {
		return nil
	}
	defer toolRows.Close()

	var seq []string
	for toolRows.Next() {
		var name string
		if toolRows.Scan(&name) == nil {
			seq = append(seq, name)
		}
	}
	return seq
}

// ============================================================
// 风险计算
// ============================================================

func (bp *BehaviorProfileEngine) calcProfileRisk(p *AgentProfile) string {
	if len(p.Anomalies) == 0 {
		return "normal"
	}

	maxSeverity := "normal"
	critCount := 0
	highCount := 0
	for _, a := range p.Anomalies {
		switch a.Severity {
		case "critical":
			critCount++
		case "high":
			highCount++
		}
	}

	if critCount > 0 {
		maxSeverity = "critical"
	} else if highCount > 0 {
		maxSeverity = "high"
	} else if len(p.Anomalies) > 2 {
		maxSeverity = "elevated"
	} else {
		maxSeverity = "elevated"
	}
	return maxSeverity
}

// isHighRiskToolName 判断工具是否高危
func isHighRiskToolName(name string) bool {
	hr := map[string]bool{
		"exec": true, "shell": true, "bash": true, "run_command": true,
		"execute_command": true, "rm": true, "curl": true, "dd": true,
	}
	return hr[strings.ToLower(name)]
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// ============================================================
// API 查询方法
// ============================================================

// ListProfiles 列出所有 Agent 画像
func (bp *BehaviorProfileEngine) ListProfiles(tenantID string) ([]AgentProfile, error) {
	agents := bp.discoverAgents(tenantID)
	var profiles []AgentProfile
	for _, a := range agents {
		p, err := bp.BuildProfile(a.agentID, tenantID)
		if err != nil {
			continue
		}
		profiles = append(profiles, *p)
	}

	// 按风险等级排序（critical > high > elevated > normal）
	riskOrder := map[string]int{"critical": 0, "high": 1, "elevated": 2, "normal": 3}
	sort.Slice(profiles, func(i, j int) bool {
		oi := riskOrder[profiles[i].RiskLevel]
		oj := riskOrder[profiles[j].RiskLevel]
		if oi != oj {
			return oi < oj
		}
		return profiles[i].TotalRequests > profiles[j].TotalRequests
	})

	return profiles, nil
}

// ListAnomalies 列出行为突变记录
func (bp *BehaviorProfileEngine) ListAnomalies(tenantID, severity string, limit int) ([]BehaviorAnomaly, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// 优先从数据库查
	where := "WHERE 1=1"
	var args []interface{}
	if tenantID != "" && tenantID != "all" {
		where += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if severity != "" && severity != "all" {
		where += " AND severity = ?"
		args = append(args, severity)
	}
	where += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := bp.db.Query(`SELECT id, timestamp, agent_id, tenant_id, type, severity, description, details, trace_id FROM behavior_anomalies `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []BehaviorAnomaly
	for rows.Next() {
		var a BehaviorAnomaly
		if rows.Scan(&a.ID, &a.Timestamp, &a.AgentID, &a.TenantID, &a.Type, &a.Severity, &a.Description, &a.Details, &a.TraceID) == nil {
			anomalies = append(anomalies, a)
		}
	}
	return anomalies, nil
}

// ListAllPatterns 全局行为序列模式统计
func (bp *BehaviorProfileEngine) ListAllPatterns(tenantID string) ([]BehaviorPattern, error) {
	agents := bp.discoverAgents(tenantID)
	allPatterns := make(map[string]*BehaviorPattern)

	for _, a := range agents {
		patterns := bp.extractPatterns(a.agentID, tenantID)
		for _, p := range patterns {
			key := strings.Join(p.Sequence, "→")
			if existing, ok := allPatterns[key]; ok {
				existing.Count += p.Count
			} else {
				cp := p
				allPatterns[key] = &cp
			}
		}
	}

	var result []BehaviorPattern
	for _, p := range allPatterns {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if len(result) > 50 {
		result = result[:50]
	}
	return result, nil
}

// ScanAndPersist 扫描并持久化突变记录
func (bp *BehaviorProfileEngine) ScanAndPersist(agentID, tenantID string) (*AgentProfile, error) {
	profile, err := bp.BuildProfile(agentID, tenantID)
	if err != nil {
		return nil, err
	}

	// 持久化突变到数据库
	for _, a := range profile.Anomalies {
		bp.db.Exec(`INSERT OR IGNORE INTO behavior_anomalies (id, timestamp, agent_id, tenant_id, type, severity, description, details, trace_id) VALUES (?,?,?,?,?,?,?,?,?)`,
			a.ID, a.Timestamp, a.AgentID, a.TenantID, a.Type, a.Severity, a.Description, a.Details, a.TraceID)
	}

	return profile, nil
}

// ============================================================
// 批量扫描
// ============================================================

// ScanAllActive 扫描所有活跃 Agent 的行为异常并持久化
func (bp *BehaviorProfileEngine) ScanAllActive(tenantID string) (scanned int, anomaliesFound int) {
	agents := bp.discoverAgents(tenantID)
	for _, a := range agents {
		profile, err := bp.ScanAndPersist(a.agentID, tenantID)
		if err != nil {
			continue
		}
		scanned++
		anomaliesFound += len(profile.Anomalies)
	}
	return scanned, anomaliesFound
}

// ============================================================
// Demo 数据注入
// ============================================================

// SeedBehaviorDemoData 注入行为画像演示数据
func (bp *BehaviorProfileEngine) SeedBehaviorDemoData(db *sql.DB) (profiles int, anomalies int, patterns int) {
	now := time.Now().UTC()

	// 创建 5 个 Agent 的演示数据：
	// 1. bot-security: 正常 — 安全扫描Bot
	// 2. bot-assistant: 正常 — 日常助手Bot
	// 3. bot-data: 轻微异常 — 数据分析Bot
	// 4. user-suspect-01: 高风险 — 可疑用户
	// 5. bot-new: 新Agent无基线

	agents := []struct {
		senderID string
		appID    string
		tools    []struct{ name, risk string }
		traces   int
		blocked  bool
	}{
		{
			senderID: "bot-security",
			appID:    "app-chat",
			tools: []struct{ name, risk string }{
				{"web_search", "low"}, {"read_file", "medium"}, {"summarize", "low"},
			},
			traces: 40,
		},
		{
			senderID: "bot-assistant",
			appID:    "app-assistant",
			tools: []struct{ name, risk string }{
				{"web_search", "low"}, {"read_file", "medium"}, {"write_file", "high"}, {"send_message", "high"},
			},
			traces: 30,
		},
		{
			senderID: "bot-data",
			appID:    "app-code",
			tools: []struct{ name, risk string }{
				{"read_file", "medium"}, {"web_search", "low"}, {"exec", "critical"}, {"web_fetch", "medium"},
			},
			traces: 20,
		},
		{
			senderID: "user-suspect-01",
			appID:    "app-chat",
			tools: []struct{ name, risk string }{
				{"exec", "critical"}, {"curl", "high"}, {"read_file", "medium"}, {"send_email", "high"}, {"write_file", "high"},
			},
			traces: 28,
			blocked: true,
		},
		{
			senderID: "bot-new",
			appID:    "app-translate",
			tools: []struct{ name, risk string }{
				{"web_search", "low"}, {"summarize", "low"},
			},
			traces: 5,
		},
	}

	for _, ag := range agents {
		for i := 0; i < ag.traces; i++ {
			offsetMin := i * 60 * 4 // 每 4 小时一个 trace
			baseTime := now.Add(-time.Duration(offsetMin) * time.Minute)
			traceID := fmt.Sprintf("bp-demo-%s-%04d", ag.senderID, i)

			// 工作时间分布 (大部分在 9-18 点)
			hour := 9 + (i % 10)
			if ag.senderID == "user-suspect-01" && i%7 == 0 {
				hour = 3 // 凌晨活动
			}
			tsStr := time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), hour, i%60, 0, 0, time.UTC).Format(time.RFC3339)

			// 插入 audit_log
			action := "pass"
			reason := ""
			if ag.blocked && i%4 == 0 {
				action = "block"
				reason = "Prompt injection detected"
			}
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, full_request_hash, latency_ms, upstream_id, app_id, trace_id) VALUES (?,?,?,?,?,?,'',?,?,?,?)`,
				tsStr, "inbound", ag.senderID, action, reason, "Agent 请求内容...", 20.0+float64(i%50), "upstream-1", ag.appID, traceID)

			// 插入 llm_calls
			reqTokens := 800 + (i%10)*200
			respTokens := 400 + (i%10)*100
			totalTokens := reqTokens + respTokens
			if ag.senderID == "user-suspect-01" && i%5 == 0 {
				totalTokens = 50000 // token spike
			}
			result, err := db.Exec(`INSERT INTO llm_calls (timestamp, trace_id, model, request_tokens, response_tokens, total_tokens, latency_ms, status_code, has_tool_use, tool_count, error_message) VALUES (?,?,?,?,?,?,?,200,1,?,?)`,
				tsStr, traceID, "claude-sonnet-4-20250514", reqTokens, respTokens, totalTokens, 500.0+float64(i%100)*20, len(ag.tools), "")

			if err == nil {
				callID, _ := result.LastInsertId()
				// 插入 llm_tool_calls (序列)
				for j, tool := range ag.tools {
					toolTime := baseTime.Add(time.Duration(100*(j+1)) * time.Millisecond)
					db.Exec(`INSERT INTO llm_tool_calls (llm_call_id, timestamp, tool_name, tool_input_preview, tool_result_preview, risk_level, flagged, flag_reason) VALUES (?,?,?,?,?,?,?,?)`,
						callID, toolTime.Format(time.RFC3339Nano), tool.name, `{"param":"value"}`, `{"result":"ok"}`, tool.risk,
						func() int {
							if tool.risk == "critical" || tool.risk == "high" {
								return 1
							}
							return 0
						}(),
						func() string {
							if tool.risk == "critical" {
								return "高危工具: " + tool.name
							}
							return ""
						}())
				}
				patterns++
			}
		}
		profiles++
	}

	// 注入行为突变记录
	demoAnomalies := []BehaviorAnomaly{
		{ID: "ba-demo-001", Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339), AgentID: "user-suspect-01", TenantID: "default", Type: "new_tool", Severity: "high", Description: "user-suspect-01 首次调用 execute_command 工具", Details: `{"tool":"execute_command","baseline_tools":3}`},
		{ID: "ba-demo-002", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-01", TenantID: "default", Type: "volume_spike", Severity: "high", Description: "user-suspect-01 今日调用量 28 次，是日均 3 次的 9.3 倍", Details: `{"today":28,"daily_avg":3,"ratio":9.3}`},
		{ID: "ba-demo-003", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-01", TenantID: "default", Type: "time_anomaly", Severity: "medium", Description: "user-suspect-01 凌晨 3:00 活跃，通常只在 10:00-17:00 活跃", Details: `{"anomaly_hour":3,"count":5,"usual_hours":"10:00-17:00"}`},
		{ID: "ba-demo-004", Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-01", TenantID: "default", Type: "token_spike", Severity: "high", Description: "user-suspect-01 单次请求消耗 50,000 token，基线平均 2,000", Details: `{"tokens":50000,"baseline_avg":2000,"ratio":25.0}`, TraceID: "bp-demo-user-suspect-01-0005"},
		{ID: "ba-demo-005", Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-01", TenantID: "default", Type: "unusual_sequence", Severity: "high", Description: "user-suspect-01 出现新的调用序列: read_file → exec → curl → send_email", Details: `{"sequence":["read_file","exec","curl","send_email"],"risk_score":95.0}`, TraceID: "bp-demo-user-suspect-01-0010"},
		{ID: "ba-demo-006", Timestamp: now.Add(-5 * time.Hour).Format(time.RFC3339), AgentID: "bot-data", TenantID: "default", Type: "new_tool", Severity: "medium", Description: "bot-data 首次调用 web_fetch 工具", Details: `{"tool":"web_fetch","baseline_tools":2}`},
		{ID: "ba-demo-007", Timestamp: now.Add(-6 * time.Hour).Format(time.RFC3339), AgentID: "bot-data", TenantID: "default", Type: "volume_spike", Severity: "medium", Description: "bot-data 今日调用量 45 次，是日均 15 次的 3.0 倍", Details: `{"today":45,"daily_avg":15,"ratio":3.0}`},
		{ID: "ba-demo-008", Timestamp: now.Add(-8 * time.Hour).Format(time.RFC3339), AgentID: "bot-assistant", TenantID: "default", Type: "unusual_sequence", Severity: "low", Description: "bot-assistant 出现新的调用序列: read_file → write_file → send_message", Details: `{"sequence":["read_file","write_file","send_message"],"risk_score":55.0}`},
	}

	for _, a := range demoAnomalies {
		db.Exec(`INSERT OR IGNORE INTO behavior_anomalies (id, timestamp, agent_id, tenant_id, type, severity, description, details, trace_id) VALUES (?,?,?,?,?,?,?,?,?)`,
			a.ID, a.Timestamp, a.AgentID, a.TenantID, a.Type, a.Severity, a.Description, a.Details, a.TraceID)
		anomalies++
	}

	return profiles, anomalies, patterns
}

// ClearBehaviorDemoData 清除行为画像演示数据
func (bp *BehaviorProfileEngine) ClearBehaviorDemoData() int64 {
	result, _ := bp.db.Exec(`DELETE FROM behavior_anomalies`)
	deleted, _ := result.RowsAffected()
	return deleted
}