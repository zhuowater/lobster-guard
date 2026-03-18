// leaderboard.go — 安全排行榜 + SLA 基线 + 攻击热力图
// lobster-guard v14.3
package main

import (
	"database/sql"
	"log"
	"sort"
	"sync"
	"time"
)

// ============================================================
// 排行榜引擎
// ============================================================

// LeaderboardEngine 安全排行榜引擎
type LeaderboardEngine struct {
	db        *sql.DB
	tenantMgr *TenantManager
	healthEng *HealthScoreEngine

	mu        sync.RWMutex
	slaConfig SLAConfig
}

// TenantScore 租户安全评分
type TenantScore struct {
	TenantID      string  `json:"tenant_id"`
	TenantName    string  `json:"tenant_name"`
	HealthScore   int     `json:"health_score"`
	RedTeamScore  float64 `json:"redteam_score"`
	IncidentCount int     `json:"incident_count"`
	BlockRate     float64 `json:"block_rate"`
	TotalRequests int     `json:"total_requests"`
	BlockedCount  int     `json:"blocked_count"`
	SLAStatus     string  `json:"sla_status"`
	SLAScore      float64 `json:"sla_score"`
	Rank          int     `json:"rank"`
	Trend         string  `json:"trend"`
}

// SLAConfig SLA 配置
type SLAConfig struct {
	MinHealthScore   int     `json:"min_health_score"`
	MaxIncidentCount int     `json:"max_incident_count"`
	MinRedTeamScore  float64 `json:"min_redteam_score"`
	MinBlockRate     float64 `json:"min_block_rate"`
}

// AttackHeatmapCell 攻击热力图单元格
type AttackHeatmapCell struct {
	TenantID  string `json:"tenant_id"`
	Category  string `json:"category"`
	Count     int    `json:"count"`
	Intensity string `json:"intensity"`
}

// DefaultSLAConfig 默认 SLA 配置
func DefaultSLAConfig() SLAConfig {
	return SLAConfig{
		MinHealthScore:   70,
		MaxIncidentCount: 10,
		MinRedTeamScore:  80.0,
		MinBlockRate:     0.0,
	}
}

// NewLeaderboardEngine 创建排行榜引擎
func NewLeaderboardEngine(db *sql.DB, tenantMgr *TenantManager, healthEng *HealthScoreEngine) *LeaderboardEngine {
	return &LeaderboardEngine{
		db:        db,
		tenantMgr: tenantMgr,
		healthEng: healthEng,
		slaConfig: DefaultSLAConfig(),
	}
}

// GetSLAConfig 获取当前 SLA 配置
func (le *LeaderboardEngine) GetSLAConfig() SLAConfig {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.slaConfig
}

// SetSLAConfig 更新 SLA 配置
func (le *LeaderboardEngine) SetSLAConfig(cfg SLAConfig) {
	le.mu.Lock()
	defer le.mu.Unlock()
	if cfg.MinHealthScore <= 0 {
		cfg.MinHealthScore = 70
	}
	if cfg.MaxIncidentCount <= 0 {
		cfg.MaxIncidentCount = 10
	}
	if cfg.MinRedTeamScore <= 0 {
		cfg.MinRedTeamScore = 80.0
	}
	le.slaConfig = cfg
	log.Printf("[排行榜] SLA 配置已更新: 健康≥%d | 事件≤%d | 检测≥%.0f%% | 拦截≥%.0f%%",
		cfg.MinHealthScore, cfg.MaxIncidentCount, cfg.MinRedTeamScore, cfg.MinBlockRate*100)
}

// ============================================================
// 排行榜计算
// ============================================================

// GetLeaderboard 获取排行榜（按健康分降序排列）
func (le *LeaderboardEngine) GetLeaderboard() []TenantScore {
	if le.tenantMgr == nil {
		return []TenantScore{}
	}

	tenants := le.tenantMgr.List()
	if len(tenants) == 0 {
		return []TenantScore{}
	}

	slaCfg := le.GetSLAConfig()
	now := time.Now().UTC()
	since7d := now.AddDate(0, 0, -7).Format(time.RFC3339)

	var scores []TenantScore
	for _, t := range tenants {
		if !t.Enabled {
			continue
		}
		ts := le.computeTenantScore(t, since7d, slaCfg)
		scores = append(scores, ts)
	}

	// 按健康分降序排序
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].HealthScore != scores[j].HealthScore {
			return scores[i].HealthScore > scores[j].HealthScore
		}
		// 相同健康分时比较红队检测率
		return scores[i].RedTeamScore > scores[j].RedTeamScore
	})

	// 设置排名
	for i := range scores {
		scores[i].Rank = i + 1
	}

	return scores
}

// computeTenantScore 计算单个租户的安全评分
func (le *LeaderboardEngine) computeTenantScore(t *Tenant, since7d string, slaCfg SLAConfig) TenantScore {
	ts := TenantScore{
		TenantID:   t.ID,
		TenantName: t.Name,
	}

	// 1. 健康分
	if le.healthEng != nil {
		result, err := le.healthEng.CalculateForTenant(t.ID)
		if err == nil && result != nil {
			ts.HealthScore = result.Score
		}
	}

	// 2. 红队检测率（最近一次报告的 pass_rate）
	ts.RedTeamScore = le.getLatestRedTeamScore(t.ID)

	// 3. 安全事件数（近7天 block 数量）
	ts.IncidentCount, ts.TotalRequests, ts.BlockedCount = le.getIncidentStats(t.ID, since7d)

	// 4. 拦截率
	if ts.TotalRequests > 0 {
		ts.BlockRate = float64(ts.BlockedCount) / float64(ts.TotalRequests) * 100
	}

	// 5. SLA 判定
	ts.SLAStatus, ts.SLAScore = le.evaluateSLA(ts, slaCfg)

	// 6. 趋势（简化：与上周健康分比较）
	ts.Trend = le.computeTrend(t.ID)

	return ts
}

// getLatestRedTeamScore 获取租户最近一次红队检测率
func (le *LeaderboardEngine) getLatestRedTeamScore(tenantID string) float64 {
	var passRate float64
	err := le.db.QueryRow(
		`SELECT pass_rate FROM redteam_reports WHERE tenant_id = ? ORDER BY timestamp DESC LIMIT 1`,
		tenantID,
	).Scan(&passRate)
	if err != nil {
		return 0
	}
	return passRate
}

// getIncidentStats 获取租户近7天的安全事件统计
func (le *LeaderboardEngine) getIncidentStats(tenantID string, since7d string) (incidents int, total int, blocked int) {
	le.db.QueryRow(
		`SELECT COUNT(*) FROM audit_log WHERE tenant_id = ? AND action = 'block' AND timestamp >= ?`,
		tenantID, since7d,
	).Scan(&incidents)

	le.db.QueryRow(
		`SELECT COUNT(*) FROM audit_log WHERE tenant_id = ? AND timestamp >= ?`,
		tenantID, since7d,
	).Scan(&total)

	blocked = incidents
	return
}

// evaluateSLA 评估 SLA 达标情况
// 返回 status ("green"/"yellow"/"red") 和 score (0-100)
func (le *LeaderboardEngine) evaluateSLA(ts TenantScore, cfg SLAConfig) (string, float64) {
	met := 0
	total := 0

	// 健康分
	total++
	if ts.HealthScore >= cfg.MinHealthScore {
		met++
	}

	// 事件数
	total++
	if ts.IncidentCount <= cfg.MaxIncidentCount {
		met++
	}

	// 红队检测率（只在有数据时检查）
	if ts.RedTeamScore > 0 {
		total++
		if ts.RedTeamScore >= cfg.MinRedTeamScore {
			met++
		}
	}

	// 拦截率阈值（如果配置了）
	if cfg.MinBlockRate > 0 {
		total++
		if ts.BlockRate >= cfg.MinBlockRate*100 {
			met++
		}
	}

	if total == 0 {
		return "green", 100
	}

	score := float64(met) / float64(total) * 100

	var status string
	switch {
	case score >= 100:
		status = "green"
	case score >= 50:
		status = "yellow"
	default:
		status = "red"
	}

	return status, score
}

// computeTrend 计算租户的健康分趋势
func (le *LeaderboardEngine) computeTrend(tenantID string) string {
	now := time.Now().UTC()
	since7d := now.AddDate(0, 0, -7).Format(time.RFC3339)
	since14d := now.AddDate(0, 0, -14).Format(time.RFC3339)

	// 当前周期（最近7天）block rate
	var thisTotal, thisBlocked int64
	le.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0) FROM audit_log WHERE tenant_id = ? AND timestamp >= ?`,
		tenantID, since7d,
	).Scan(&thisTotal, &thisBlocked)

	// 上一周期（7-14天前）block rate
	var lastTotal, lastBlocked int64
	le.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0) FROM audit_log WHERE tenant_id = ? AND timestamp >= ? AND timestamp < ?`,
		tenantID, since14d, since7d,
	).Scan(&lastTotal, &lastBlocked)

	thisRate := float64(0)
	if thisTotal > 0 {
		thisRate = float64(thisBlocked) / float64(thisTotal)
	}
	lastRate := float64(0)
	if lastTotal > 0 {
		lastRate = float64(lastBlocked) / float64(lastTotal)
	}

	// block rate 下降 = 改善 = up, 上升 = 恶化 = down
	diff := lastRate - thisRate
	if diff > 0.05 {
		return "up"
	} else if diff < -0.05 {
		return "down"
	}
	return "stable"
}

// ============================================================
// 攻击热力图
// ============================================================

// owaspCategories OWASP 分类名称映射
var owaspCategories = map[string]string{
	"PI": "prompt_injection",
	"IO": "insecure_output",
	"SI": "sensitive_info",
	"IP": "insecure_plugin",
	"OR": "overreliance",
	"MD": "model_dos",
}

// owaspCategoryOrder 排序用
var owaspCategoryOrder = []string{"PI", "IO", "SI", "IP", "OR", "MD"}

// GetHeatmap 获取攻击热力图（租户×OWASP分类矩阵）
func (le *LeaderboardEngine) GetHeatmap() []AttackHeatmapCell {
	if le.tenantMgr == nil {
		return []AttackHeatmapCell{}
	}

	tenants := le.tenantMgr.List()
	now := time.Now().UTC()
	since7d := now.AddDate(0, 0, -7).Format(time.RFC3339)

	var cells []AttackHeatmapCell

	// 根据审计日志中 reason 字段判定 OWASP 分类
	categoryKeywords := map[string][]string{
		"PI": {"injection", "jailbreak", "ignore", "忽略", "DAN", "override", "system prompt", "提示注入"},
		"IO": {"xss", "sql injection", "malicious code", "reverse shell", "恶意代码", "output"},
		"SI": {"api_key", "password", "token", "secret", "pii", "sensitive", "ssn", "credit card", "身份证", "敏感"},
		"IP": {"curl", "bash", "chmod", "base64", "path traversal", "command injection", "命令注入"},
		"OR": {"system update", "debug mode", "developer mode", "开发者模式", "角色扮演"},
		"MD": {"repeat", "超长", "dos", "overload"},
	}

	for _, t := range tenants {
		if !t.Enabled {
			continue
		}
		// Query block reasons for this tenant
		rows, err := le.db.Query(
			`SELECT reason FROM audit_log WHERE tenant_id = ? AND action = 'block' AND timestamp >= ? AND reason != ''`,
			t.ID, since7d,
		)
		if err != nil {
			continue
		}

		// Count per category
		catCounts := make(map[string]int)
		for rows.Next() {
			var reason string
			rows.Scan(&reason)
			for cat, keywords := range categoryKeywords {
				for _, kw := range keywords {
					if containsCI(reason, kw) {
						catCounts[cat]++
						break
					}
				}
			}
		}
		rows.Close()

		// Generate cells
		for _, cat := range owaspCategoryOrder {
			count := catCounts[cat]
			cells = append(cells, AttackHeatmapCell{
				TenantID:  t.ID,
				Category:  cat,
				Count:     count,
				Intensity: countToIntensity(count),
			})
		}
	}

	return cells
}

// countToIntensity 将计数转换为强度等级
func countToIntensity(count int) string {
	switch {
	case count >= 20:
		return "critical"
	case count >= 10:
		return "high"
	case count >= 3:
		return "medium"
	case count > 0:
		return "low"
	default:
		return "none"
	}
}

// ============================================================
// SLA 达标情况查询
// ============================================================

// SLAOverview SLA 概览
type SLAOverview struct {
	Config  SLAConfig          `json:"config"`
	Tenants []TenantSLAStatus  `json:"tenants"`
	Summary SLASummary         `json:"summary"`
}

// TenantSLAStatus 单个租户的 SLA 状态
type TenantSLAStatus struct {
	TenantID     string  `json:"tenant_id"`
	TenantName   string  `json:"tenant_name"`
	HealthScore  int     `json:"health_score"`
	HealthMet    bool    `json:"health_met"`
	IncidentCount int    `json:"incident_count"`
	IncidentMet  bool    `json:"incident_met"`
	RedTeamScore float64 `json:"redteam_score"`
	RedTeamMet   bool    `json:"redteam_met"`
	SLAStatus    string  `json:"sla_status"`
	SLAScore     float64 `json:"sla_score"`
}

// SLASummary SLA 汇总
type SLASummary struct {
	Total   int `json:"total"`
	Green   int `json:"green"`
	Yellow  int `json:"yellow"`
	Red     int `json:"red"`
}

// GetSLAOverview 获取 SLA 达标概览
func (le *LeaderboardEngine) GetSLAOverview() *SLAOverview {
	cfg := le.GetSLAConfig()
	leaderboard := le.GetLeaderboard()

	overview := &SLAOverview{
		Config: cfg,
	}

	for _, ts := range leaderboard {
		status := TenantSLAStatus{
			TenantID:      ts.TenantID,
			TenantName:    ts.TenantName,
			HealthScore:   ts.HealthScore,
			HealthMet:     ts.HealthScore >= cfg.MinHealthScore,
			IncidentCount: ts.IncidentCount,
			IncidentMet:   ts.IncidentCount <= cfg.MaxIncidentCount,
			RedTeamScore:  ts.RedTeamScore,
			RedTeamMet:    ts.RedTeamScore >= cfg.MinRedTeamScore || ts.RedTeamScore == 0,
			SLAStatus:     ts.SLAStatus,
			SLAScore:      ts.SLAScore,
		}
		overview.Tenants = append(overview.Tenants, status)

		overview.Summary.Total++
		switch ts.SLAStatus {
		case "green":
			overview.Summary.Green++
		case "yellow":
			overview.Summary.Yellow++
		case "red":
			overview.Summary.Red++
		}
	}

	if overview.Tenants == nil {
		overview.Tenants = []TenantSLAStatus{}
	}

	return overview
}
