// user_profile.go — 用户风险画像引擎
// lobster-guard v11.0
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// UserRiskProfile 用户风险画像
type UserRiskProfile struct {
	UserID            string    `json:"user_id"`
	DisplayName       string    `json:"display_name"`
	Department        string    `json:"department"`
	RiskScore         int       `json:"risk_score"`
	RiskLevel         string    `json:"risk_level"`
	TotalRequests     int64     `json:"total_requests"`
	BlockedRequests   int64     `json:"blocked_requests"`
	BlockRate         float64   `json:"block_rate"`
	InjectionAttempts int64     `json:"injection_attempts"`
	HighRiskTools     int64     `json:"high_risk_tools"`
	CanaryLeaks       int64     `json:"canary_leaks"`
	BudgetViolations  int64     `json:"budget_violations"`
	LastSeen          time.Time `json:"last_seen"`
	FirstSeen         time.Time `json:"first_seen"`
	ActiveDays        int       `json:"active_days"`
	PeakHour          int       `json:"peak_hour"`
	OffHoursRate      float64   `json:"off_hours_rate"`
	RiskTrend         string    `json:"risk_trend"`
	Last7dScore       int       `json:"last_7d_score"`
	Last30dScore      int       `json:"last_30d_score"`
}

// UserTimelineEvent 用户行为时间线事件
type UserTimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	RiskLevel string    `json:"risk_level"`
	Summary   string    `json:"summary"`
	Details   string    `json:"details"`
}

// UserRiskStats 风险统计概览
type UserRiskStats struct {
	TotalUsers    int     `json:"total_users"`
	CriticalCount int     `json:"critical_count"`
	HighCount     int     `json:"high_count"`
	MediumCount   int     `json:"medium_count"`
	LowCount      int     `json:"low_count"`
	AvgScore      float64 `json:"avg_score"`
	Alerts24h     int     `json:"alerts_24h"`
}

// UserProfileEngine 用户画像引擎
type UserProfileEngine struct {
	db *sql.DB
}

func lookupSenderIdentity(db *sql.DB, senderID string) (string, string) {
	if db == nil || senderID == "" {
		return "", ""
	}

	var name, dept string
	if err := db.QueryRow(`SELECT name, department FROM user_info_cache WHERE sender_id = ? ORDER BY updated_at DESC LIMIT 1`, senderID).Scan(&name, &dept); err == nil {
		return strings.TrimSpace(name), strings.TrimSpace(dept)
	}
	if err := db.QueryRow(`SELECT display_name, department FROM user_routes WHERE sender_id = ? ORDER BY updated_at DESC LIMIT 1`, senderID).Scan(&name, &dept); err == nil {
		return strings.TrimSpace(name), strings.TrimSpace(dept)
	}
	return "", ""
}

// NewUserProfileEngine 创建用户画像引擎
func NewUserProfileEngine(db *sql.DB) *UserProfileEngine {
	return &UserProfileEngine{db: db}
}

// GetTopRiskUsers 获取风险最高的 N 个用户
func (e *UserProfileEngine) GetTopRiskUsers(limit int) ([]UserRiskProfile, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// 获取所有唯一 sender_id
	rows, err := e.db.Query(`SELECT DISTINCT sender_id FROM audit_log WHERE sender_id != '' ORDER BY sender_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if rows.Scan(&uid) == nil && uid != "" {
			userIDs = append(userIDs, uid)
		}
	}

	// 为每个用户计算画像
	var profiles []UserRiskProfile
	for _, uid := range userIDs {
		p, err := e.GetUserProfile(uid)
		if err != nil {
			continue
		}
		profiles = append(profiles, *p)
	}

	// 按风险分排序
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			if profiles[j].RiskScore > profiles[i].RiskScore {
				profiles[i], profiles[j] = profiles[j], profiles[i]
			}
		}
	}

	if len(profiles) > limit {
		profiles = profiles[:limit]
	}
	return profiles, nil
}

// GetTopRiskUsersTenant v14.0: 租户感知的风险用户 TOP N
func (e *UserProfileEngine) GetTopRiskUsersTenant(limit int, tenantID string) ([]UserRiskProfile, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	tClause, tArgs := TenantFilter(tenantID)
	query := `SELECT DISTINCT sender_id FROM audit_log WHERE sender_id != ''` + tClause + ` ORDER BY sender_id`
	rows, err := e.db.Query(query, tArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if rows.Scan(&uid) == nil && uid != "" {
			userIDs = append(userIDs, uid)
		}
	}

	var profiles []UserRiskProfile
	for _, uid := range userIDs {
		p, err := e.GetUserProfile(uid)
		if err != nil {
			continue
		}
		profiles = append(profiles, *p)
	}

	// Sort by risk score descending
	for i := 0; i < len(profiles); i++ {
		for j := i + 1; j < len(profiles); j++ {
			if profiles[j].RiskScore > profiles[i].RiskScore {
				profiles[i], profiles[j] = profiles[j], profiles[i]
			}
		}
	}

	if len(profiles) > limit {
		profiles = profiles[:limit]
	}
	return profiles, nil
}

// GetUserProfile 获取单个用户的风险画像
func (e *UserProfileEngine) GetUserProfile(userID string) (*UserRiskProfile, error) {
	displayName, department := lookupSenderIdentity(e.db, userID)
	p := &UserRiskProfile{
		UserID:      userID,
		DisplayName: userID,
		Department:  department,
	}
	if displayName != "" {
		p.DisplayName = displayName
	}

	// 总请求数和拦截数
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=?`, userID).Scan(&p.TotalRequests)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND action='block'`, userID).Scan(&p.BlockedRequests)

	if p.TotalRequests > 0 {
		p.BlockRate = float64(p.BlockedRequests) / float64(p.TotalRequests)
	}

	// 注入尝试（block 事件中 reason 包含 injection/jailbreak/prompt 的）
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND action='block' AND (
		LOWER(reason) LIKE '%injection%' OR LOWER(reason) LIKE '%jailbreak%' OR 
		LOWER(reason) LIKE '%prompt%' OR LOWER(reason) LIKE '%xss%' OR
		LOWER(reason) LIKE '%command%'
	)`, userID).Scan(&p.InjectionAttempts)

	// 高危工具调用（LLM 侧 — 通过 llm_tool_calls 的 tool_input_preview 或 flag_reason 关联）
	// 由于 LLM 调用没有直接的 user_id，我们用时间关联
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_tool_calls WHERE risk_level IN ('high','critical')`).Scan(&p.HighRiskTools)
	// 按用户比例分配（简化处理）
	var totalUsers int64
	e.db.QueryRow(`SELECT COUNT(DISTINCT sender_id) FROM audit_log WHERE sender_id != ''`).Scan(&totalUsers)
	if totalUsers > 1 {
		// 用拦截率加权分配高危工具
		p.HighRiskTools = int64(float64(p.HighRiskTools) * p.BlockRate * 2)
		if p.HighRiskTools < 0 {
			p.HighRiskTools = 0
		}
	}

	// Canary 泄露
	var totalCanary int64
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE canary_leaked=1`).Scan(&totalCanary)
	if totalUsers > 0 && p.BlockRate > 0.1 {
		p.CanaryLeaks = int64(math.Ceil(float64(totalCanary) * p.BlockRate))
	}

	// 预算超限
	var totalBudget int64
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE budget_exceeded=1`).Scan(&totalBudget)
	if totalUsers > 0 && p.BlockRate > 0.1 {
		p.BudgetViolations = int64(math.Ceil(float64(totalBudget) * p.BlockRate))
	}

	// 时间相关
	var firstSeen, lastSeen sql.NullString
	e.db.QueryRow(`SELECT MIN(timestamp), MAX(timestamp) FROM audit_log WHERE sender_id=?`, userID).Scan(&firstSeen, &lastSeen)
	if firstSeen.Valid {
		if t, err := time.Parse(time.RFC3339, firstSeen.String); err == nil {
			p.FirstSeen = t
		}
	}
	if lastSeen.Valid {
		if t, err := time.Parse(time.RFC3339, lastSeen.String); err == nil {
			p.LastSeen = t
		}
	}

	// 活跃天数
	var activeDays int
	e.db.QueryRow(`SELECT COUNT(DISTINCT date(timestamp)) FROM audit_log WHERE sender_id=?`, userID).Scan(&activeDays)
	p.ActiveDays = activeDays

	// 高峰时段
	var peakHour sql.NullInt64
	e.db.QueryRow(`SELECT CAST(strftime('%H', timestamp) AS INTEGER) as h FROM audit_log WHERE sender_id=? GROUP BY h ORDER BY COUNT(*) DESC LIMIT 1`, userID).Scan(&peakHour)
	if peakHour.Valid {
		p.PeakHour = int(peakHour.Int64)
	}

	// 非工作时间活跃率 (22:00-06:00)
	var offHoursCount int64
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND (
		CAST(strftime('%H', timestamp) AS INTEGER) >= 22 OR CAST(strftime('%H', timestamp) AS INTEGER) < 6
	)`, userID).Scan(&offHoursCount)
	if p.TotalRequests > 0 {
		p.OffHoursRate = float64(offHoursCount) / float64(p.TotalRequests)
	}

	// 计算风险分
	p.RiskScore = e.calculateRiskScore(p)
	p.RiskLevel = riskLevelFromScore(p.RiskScore)

	// 计算趋势
	p.Last7dScore = e.calculatePeriodScore(userID, 7)
	p.Last30dScore = e.calculatePeriodScore(userID, 30)
	if p.Last7dScore > p.Last30dScore+5 {
		p.RiskTrend = "rising"
	} else if p.Last7dScore < p.Last30dScore-5 {
		p.RiskTrend = "falling"
	} else {
		p.RiskTrend = "stable"
	}

	return p, nil
}

// calculateRiskScore 计算用户风险评分（0-100）
func (e *UserProfileEngine) calculateRiskScore(p *UserRiskProfile) int {
	score := 0.0

	// 1. 拦截率 (0-30 分)
	if p.BlockRate > 0.50 {
		score += 30
	} else if p.BlockRate > 0.20 {
		score += 20
	} else if p.BlockRate > 0.05 {
		score += 10
	}

	// 2. 注入尝试次数 (0-25 分)
	if p.InjectionAttempts > 20 {
		score += 25
	} else if p.InjectionAttempts > 10 {
		score += 20
	} else if p.InjectionAttempts > 5 {
		score += 15
	} else if p.InjectionAttempts > 0 {
		score += 5
	}

	// 3. 高危工具使用 (0-20 分)
	if p.HighRiskTools > 10 {
		score += 20
	} else if p.HighRiskTools > 5 {
		score += 15
	} else if p.HighRiskTools > 0 {
		score += 5
	}

	// 4. Canary 泄露 (0-15 分)
	canaryScore := float64(p.CanaryLeaks) * 5
	if canaryScore > 15 {
		canaryScore = 15
	}
	score += canaryScore

	// 5. 非工作时间活跃 (0-10 分)
	if p.OffHoursRate > 0.60 {
		score += 10
	} else if p.OffHoursRate > 0.30 {
		score += 5
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(score)
}

// calculatePeriodScore 计算指定天数内的风险分
func (e *UserProfileEngine) calculatePeriodScore(userID string, days int) int {
	since := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)

	var totalReq, blockedReq, injectionAttempts int64
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND timestamp>=?`, userID, since).Scan(&totalReq)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND action='block' AND timestamp>=?`, userID, since).Scan(&blockedReq)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE sender_id=? AND action='block' AND timestamp>=? AND (
		LOWER(reason) LIKE '%injection%' OR LOWER(reason) LIKE '%jailbreak%' OR 
		LOWER(reason) LIKE '%prompt%'
	)`, userID, since).Scan(&injectionAttempts)

	blockRate := 0.0
	if totalReq > 0 {
		blockRate = float64(blockedReq) / float64(totalReq)
	}

	score := 0.0
	if blockRate > 0.50 {
		score += 30
	} else if blockRate > 0.20 {
		score += 20
	} else if blockRate > 0.05 {
		score += 10
	}
	if injectionAttempts > 20 {
		score += 25
	} else if injectionAttempts > 10 {
		score += 20
	} else if injectionAttempts > 5 {
		score += 15
	} else if injectionAttempts > 0 {
		score += 5
	}

	if score > 100 {
		score = 100
	}
	return int(score)
}

// GetUserTimeline 获取用户行为时间线
func (e *UserProfileEngine) GetUserTimeline(userID string, limit int) ([]UserTimelineEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var events []UserTimelineEvent

	// IM 侧审计日志事件
	rows, err := e.db.Query(`SELECT timestamp, action, reason, content_preview FROM audit_log 
		WHERE sender_id=? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ts, action, reason, content string
		if rows.Scan(&ts, &action, &reason, &content) != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, ts)

		evt := UserTimelineEvent{
			Timestamp: t,
		}

		switch action {
		case "block":
			evt.EventType = "im_blocked"
			if reason != "" && (containsAny(reason, "injection", "jailbreak", "prompt", "xss", "command")) {
				evt.RiskLevel = "critical"
			} else {
				evt.RiskLevel = "high"
			}
			evt.Summary = "IM 请求被拦截"
			if reason != "" {
				evt.Summary += " (" + truncateStr(reason, 50) + ")"
			}
		case "warn":
			evt.EventType = "im_request"
			evt.RiskLevel = "medium"
			evt.Summary = "IM 请求告警"
			if reason != "" {
				evt.Summary += " (" + truncateStr(reason, 50) + ")"
			}
		default:
			evt.EventType = "im_request"
			evt.RiskLevel = "low"
			evt.Summary = "IM 请求通过"
		}

		// 构建详情 JSON
		details := map[string]string{
			"action":  action,
			"reason":  reason,
			"content": truncateStr(content, 200),
		}
		if dj, err := json.Marshal(details); err == nil {
			evt.Details = string(dj)
		}

		events = append(events, evt)
	}

	// 按时间倒序排列
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Timestamp.After(events[i].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

// GetRiskStats 获取风险统计概览
func (e *UserProfileEngine) GetRiskStats() (*UserRiskStats, error) {
	stats := &UserRiskStats{}

	profiles, err := e.GetTopRiskUsers(100)
	if err != nil {
		return stats, err
	}

	stats.TotalUsers = len(profiles)
	totalScore := 0
	for _, p := range profiles {
		totalScore += p.RiskScore
		switch p.RiskLevel {
		case "critical":
			stats.CriticalCount++
		case "high":
			stats.HighCount++
		case "medium":
			stats.MediumCount++
		case "low":
			stats.LowCount++
		}
	}
	if stats.TotalUsers > 0 {
		stats.AvgScore = math.Round(float64(totalScore)/float64(stats.TotalUsers)*10) / 10
	}

	// 24h 新增告警
	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='block' AND timestamp>=?`, since24h).Scan(&stats.Alerts24h)

	return stats, nil
}

// riskLevelFromScore 从分数映射到风险等级
func riskLevelFromScore(score int) string {
	switch {
	case score >= 76:
		return "critical"
	case score >= 51:
		return "high"
	case score >= 26:
		return "medium"
	default:
		return "low"
	}
}

// containsAny 检查字符串是否包含任一子串
func containsAny(s string, subs ...string) bool {
	lower := fmt.Sprintf("%s", s)
	for _, sub := range subs {
		if len(lower) >= len(sub) {
			for i := 0; i <= len(lower)-len(sub); i++ {
				match := true
				for j := 0; j < len(sub); j++ {
					c := lower[i+j]
					if c >= 'A' && c <= 'Z' {
						c += 32
					}
					sc := sub[j]
					if sc >= 'A' && sc <= 'Z' {
						sc += 32
					}
					if c != sc {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RecordEvent 记录安全事件到用户画像（v24.1 反事实验证联动）
func (e *UserProfileEngine) RecordEvent(userID, eventType string, score float64) {
	if e == nil || e.db == nil || userID == "" {
		return
	}
	// 将事件记录为 audit_log 中的 block 条目（与现有风险分计算兼容）
	ts := time.Now().UTC().Format(time.RFC3339)
	reason := fmt.Sprintf("cf_attribution: %s (score=%.2f)", eventType, score)
	_, err := e.db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview, full_request_hash, latency_ms, upstream_id, app_id, trace_id)
		VALUES (?, 'inbound', ?, 'block', ?, ?, '', 0, '', '', '')`,
		ts, userID, reason, fmt.Sprintf("counterfactual_%s", eventType))
	if err != nil {
		// best-effort, log and continue
		fmt.Printf("[UserProfile] RecordEvent 写入失败: %v\n", err)
	}
}
