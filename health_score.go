// health_score.go — 综合安全健康分引擎 + OWASP LLM Top 10 矩阵 + 严格模式 + 通知中心
// lobster-guard v11.1 — 驾驶舱模式
package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================
// 功能一：综合安全健康分
// ============================================================

// HealthScoreResult 安全健康分结果
type HealthScoreResult struct {
	Score      int                  `json:"score"`
	Level      string               `json:"level"`
	LevelLabel string               `json:"level_label"`
	Deductions []HealthDeduction    `json:"deductions"`
	Trend      []HealthScoreTrend   `json:"trend"`
	UpdatedAt  string               `json:"updated_at"`
}

// HealthDeduction 单项扣分
type HealthDeduction struct {
	Name     string `json:"name"`
	Points   int    `json:"points"`
	MaxPoints int   `json:"max_points"`
	Detail   string `json:"detail"`
}

// HealthScoreTrend 每天的分数趋势
type HealthScoreTrend struct {
	Date  string `json:"date"`
	Score int    `json:"score"`
}

// HealthScoreEngine 安全健康分引擎
type HealthScoreEngine struct {
	db *sql.DB
}

// NewHealthScoreEngine 创建引擎
func NewHealthScoreEngine(db *sql.DB) *HealthScoreEngine {
	return &HealthScoreEngine{db: db}
}

// Calculate 计算当前安全健康分
func (e *HealthScoreEngine) Calculate() (*HealthScoreResult, error) {
	score := 100
	var deductions []HealthDeduction
	now := time.Now().UTC()
	since7d := now.AddDate(0, 0, -7).Format(time.RFC3339)

	// 1. IM 拦截率 > 30% 扣30分
	var imTotal, imBlocked int64
	row := e.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0) FROM audit_log WHERE timestamp >= ?`, since7d)
	row.Scan(&imTotal, &imBlocked)
	imBlockRate := float64(0)
	if imTotal > 0 {
		imBlockRate = float64(imBlocked) / float64(imTotal) * 100
	}
	imDeduct := 0
	if imBlockRate > 30 {
		imDeduct = 30
	} else if imBlockRate > 20 {
		imDeduct = 20
	} else if imBlockRate > 10 {
		imDeduct = 10
	}
	if imDeduct > 0 {
		score -= imDeduct
		deductions = append(deductions, HealthDeduction{
			Name: "IM 拦截率", Points: imDeduct, MaxPoints: 30,
			Detail: fmt.Sprintf("拦截率 %.1f%% (%d/%d)", imBlockRate, imBlocked, imTotal),
		})
	}

	// 2. LLM 异常率扣20分
	var llmTotal, llmErrors int64
	row = e.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END),0) FROM llm_calls WHERE timestamp >= ?`, since7d)
	row.Scan(&llmTotal, &llmErrors)
	llmErrorRate := float64(0)
	if llmTotal > 0 {
		llmErrorRate = float64(llmErrors) / float64(llmTotal) * 100
	}
	llmDeduct := 0
	if llmErrorRate > 20 {
		llmDeduct = 20
	} else if llmErrorRate > 10 {
		llmDeduct = 15
	} else if llmErrorRate > 5 {
		llmDeduct = 10
	}
	if llmDeduct > 0 {
		score -= llmDeduct
		deductions = append(deductions, HealthDeduction{
			Name: "LLM 异常率", Points: llmDeduct, MaxPoints: 20,
			Detail: fmt.Sprintf("异常率 %.1f%% (%d/%d)", llmErrorRate, llmErrors, llmTotal),
		})
	}

	// 3. Canary 泄露每次扣10分最多20
	var canaryLeaks int64
	row = e.db.QueryRow(`SELECT COUNT(*) FROM llm_tool_calls WHERE flagged=1 AND flag_reason LIKE '%canary%' AND timestamp >= ?`, since7d)
	row.Scan(&canaryLeaks)
	canaryDeduct := int(canaryLeaks) * 10
	if canaryDeduct > 20 {
		canaryDeduct = 20
	}
	if canaryDeduct > 0 {
		score -= canaryDeduct
		deductions = append(deductions, HealthDeduction{
			Name: "Canary 泄露", Points: canaryDeduct, MaxPoints: 20,
			Detail: fmt.Sprintf("发现 %d 次 Canary Token 泄露", canaryLeaks),
		})
	}

	// 4. 高危用户每个扣5分最多15
	var highRiskUsers int64
	// Count distinct users with block rate > 30%
	rows, err := e.db.Query(`SELECT sender_id, COUNT(*) as total, SUM(CASE WHEN action='block' THEN 1 ELSE 0 END) as blocked FROM audit_log WHERE timestamp >= ? AND sender_id != '' GROUP BY sender_id`, since7d)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sid string
			var total, blocked int64
			rows.Scan(&sid, &total, &blocked)
			if total >= 5 && float64(blocked)/float64(total) > 0.3 {
				highRiskUsers++
			}
		}
	}
	hrDeduct := int(highRiskUsers) * 5
	if hrDeduct > 15 {
		hrDeduct = 15
	}
	if hrDeduct > 0 {
		score -= hrDeduct
		deductions = append(deductions, HealthDeduction{
			Name: "高危用户", Points: hrDeduct, MaxPoints: 15,
			Detail: fmt.Sprintf("发现 %d 个高危用户", highRiskUsers),
		})
	}

	// 5. 规则频繁命中扣15分（block 事件过多）
	ruleHitDeduct := 0
	if imBlocked > 100 {
		ruleHitDeduct = 15
	} else if imBlocked > 50 {
		ruleHitDeduct = 10
	} else if imBlocked > 20 {
		ruleHitDeduct = 5
	}
	if ruleHitDeduct > 0 {
		score -= ruleHitDeduct
		deductions = append(deductions, HealthDeduction{
			Name: "规则频繁命中", Points: ruleHitDeduct, MaxPoints: 15,
			Detail: fmt.Sprintf("7天内 %d 次拦截命中", imBlocked),
		})
	}

	if score < 0 {
		score = 0
	}

	// 计算等级
	level, levelLabel := scoreToLevel(score)

	// 7天趋势
	trend := e.calculateTrend(now)

	return &HealthScoreResult{
		Score:      score,
		Level:      level,
		LevelLabel: levelLabel,
		Deductions: deductions,
		Trend:      trend,
		UpdatedAt:  now.Format(time.RFC3339),
	}, nil
}

// calculateTrend 计算最近7天每天的分数
func (e *HealthScoreEngine) calculateTrend(now time.Time) []HealthScoreTrend {
	var trend []HealthScoreTrend
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		dayEnd := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339)

		dayScore := 100
		var total, blocked int64
		row := e.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN action='block' THEN 1 ELSE 0 END),0) FROM audit_log WHERE timestamp >= ? AND timestamp <= ?`, dayStart, dayEnd)
		row.Scan(&total, &blocked)

		if total > 0 {
			blockRate := float64(blocked) / float64(total) * 100
			if blockRate > 30 {
				dayScore -= 30
			} else if blockRate > 20 {
				dayScore -= 20
			} else if blockRate > 10 {
				dayScore -= 10
			}
			if blocked > 20 {
				dayScore -= 15
			} else if blocked > 10 {
				dayScore -= 10
			} else if blocked > 5 {
				dayScore -= 5
			}
		}

		if dayScore < 0 {
			dayScore = 0
		}

		trend = append(trend, HealthScoreTrend{
			Date:  day.Format("2006-01-02"),
			Score: dayScore,
		})
	}
	return trend
}

func scoreToLevel(score int) (string, string) {
	switch {
	case score >= 90:
		return "excellent", "优秀"
	case score >= 70:
		return "good", "良好"
	case score >= 50:
		return "warning", "警告"
	case score >= 30:
		return "danger", "危险"
	default:
		return "critical", "严重"
	}
}

// ============================================================
// 功能二：OWASP LLM Top 10 矩阵
// ============================================================

// OWASPMatrixItem OWASP LLM Top 10 单项
type OWASPMatrixItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NameZh    string `json:"name_zh"`
	Count     int64  `json:"count"`
	RiskLevel string `json:"risk_level"` // none / low / medium / high
}

// OWASPMatrixEngine OWASP 矩阵引擎
type OWASPMatrixEngine struct {
	db            *sql.DB
	llmRuleEngine *LLMRuleEngine
}

// NewOWASPMatrixEngine 创建
func NewOWASPMatrixEngine(db *sql.DB, llmRuleEngine *LLMRuleEngine) *OWASPMatrixEngine {
	return &OWASPMatrixEngine{db: db, llmRuleEngine: llmRuleEngine}
}

// Calculate 计算 OWASP Top 10 矩阵（默认 24h）
func (e *OWASPMatrixEngine) Calculate() []OWASPMatrixItem {
	return e.CalculateWithFilter("")
}

// CalculateWithFilter 计算 OWASP Top 10 矩阵（v11.4: 支持时间过滤）
// sinceRFC3339 为空则使用默认 24h 窗口
func (e *OWASPMatrixEngine) CalculateWithFilter(sinceRFC3339 string) []OWASPMatrixItem {
	since24h := sinceRFC3339
	if since24h == "" {
		since24h = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	}
	items := make([]OWASPMatrixItem, 10)

	// 定义 OWASP LLM Top 10
	owaspDefs := []struct {
		id, name, nameZh string
		categories       []string // 匹配的 LLM 规则 category
		flagReasons      []string // 匹配的 flag_reason
	}{
		{"LLM01", "Prompt Injection", "提示注入", []string{"prompt_injection"}, nil},
		{"LLM02", "Insecure Output", "不安全输出", []string{"pii_leak"}, nil},
		{"LLM03", "Training Data", "训练数据中毒", nil, nil},
		{"LLM04", "Model DoS", "模型拒绝服务", []string{"token_abuse"}, []string{"budget_exceeded"}},
		{"LLM05", "Supply Chain", "供应链漏洞", nil, nil},
		{"LLM06", "Sensitive Info", "敏感信息泄露", []string{"pii_leak"}, []string{"canary_leaked"}},
		{"LLM07", "Insecure Plugin", "不安全插件", nil, []string{"high_risk_tool"}},
		{"LLM08", "Excessive Agency", "过度代理权限", nil, []string{"budget_exceeded", "high_risk_tool"}},
		{"LLM09", "Overreliance", "过度依赖", nil, nil},
		{"LLM10", "Model Theft", "模型盗取", nil, []string{"canary_leaked"}},
	}

	// 从 LLM 规则命中统计中获取 category 级别计数
	categoryHits := make(map[string]int64)
	if e.llmRuleEngine != nil {
		hits := e.llmRuleEngine.GetHits()
		rules := e.llmRuleEngine.GetRules()
		ruleCategory := make(map[string]string)
		for _, r := range rules {
			ruleCategory[r.ID] = r.Category
		}
		for ruleID, hit := range hits {
			cat := ruleCategory[ruleID]
			if cat != "" {
				categoryHits[cat] += hit.Count + hit.ShadowHits
			}
		}
	}

	// 从 llm_tool_calls 获取 flagged 事件
	flagCounts := make(map[string]int64)
	flagRows, err := e.db.Query(`SELECT flag_reason, COUNT(*) FROM llm_tool_calls WHERE flagged=1 AND timestamp >= ? GROUP BY flag_reason`, since24h)
	if err == nil {
		defer flagRows.Close()
		for flagRows.Next() {
			var reason string
			var cnt int64
			flagRows.Scan(&reason, &cnt)
			flagCounts[reason] += cnt
		}
	}

	// 从 llm_calls 获取 budget 违规
	var budgetViolations int64
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_tool_calls WHERE flagged=1 AND flag_reason LIKE '%budget%' AND timestamp >= ?`, since24h).Scan(&budgetViolations)
	flagCounts["budget_exceeded"] += budgetViolations

	// 获取高风险工具调用
	var highRiskTools int64
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp >= ?`, since24h).Scan(&highRiskTools)
	flagCounts["high_risk_tool"] += highRiskTools

	for i, def := range owaspDefs {
		var count int64

		// 累加 category 命中
		for _, cat := range def.categories {
			count += categoryHits[cat]
		}
		// 累加 flag 命中
		for _, fr := range def.flagReasons {
			count += flagCounts[fr]
		}

		riskLevel := "none"
		if count > 5 {
			riskLevel = "high"
		} else if count > 0 {
			riskLevel = "low"
		}

		items[i] = OWASPMatrixItem{
			ID:        def.id,
			Name:      def.name,
			NameZh:    def.nameZh,
			Count:     count,
			RiskLevel: riskLevel,
		}
	}
	return items
}

// ============================================================
// 功能三：严格模式
// ============================================================

// StrictModeManager 严格模式管理器
type StrictModeManager struct {
	mu           sync.RWMutex
	enabled      bool
	origInbound  []InboundRuleConfig  // 保存原始 IM 规则
	origLLMRules []LLMRule            // 保存原始 LLM 规则
	inboundEngine  *RuleEngine
	llmRuleEngine  *LLMRuleEngine
}

// NewStrictModeManager 创建
func NewStrictModeManager(inboundEngine *RuleEngine, llmRuleEngine *LLMRuleEngine) *StrictModeManager {
	return &StrictModeManager{
		inboundEngine: inboundEngine,
		llmRuleEngine: llmRuleEngine,
	}
}

// IsEnabled 是否启用
func (m *StrictModeManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled 设置严格模式
func (m *StrictModeManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if enabled == m.enabled {
		return
	}

	if enabled {
		// 进入严格模式 — 保存原始状态，切所有为 block
		log.Printf("[严格模式] ⚠️ 启用严格模式")

		// 保存 IM 规则原始状态
		if m.inboundEngine != nil {
			orig := m.inboundEngine.GetRuleConfigs()
			m.origInbound = make([]InboundRuleConfig, len(orig))
			copy(m.origInbound, orig)

			// 切换所有规则为 block
			strictRules := make([]InboundRuleConfig, len(orig))
			copy(strictRules, orig)
			for i := range strictRules {
				strictRules[i].Action = "block"
			}
			source := m.inboundEngine.Version().Source
			m.inboundEngine.Reload(strictRules, source)
		}

		// 保存 LLM 规则原始状态
		if m.llmRuleEngine != nil {
			orig := m.llmRuleEngine.GetRules()
			m.origLLMRules = make([]LLMRule, len(orig))
			copy(m.origLLMRules, orig)

			// 切换所有 LLM 规则为 block + 关闭 shadow
			strictRules := make([]LLMRule, len(orig))
			copy(strictRules, orig)
			for i := range strictRules {
				strictRules[i].Action = "block"
				strictRules[i].ShadowMode = false
			}
			m.llmRuleEngine.UpdateRules(strictRules)
		}
	} else {
		// 退出严格模式 — 恢复原始状态
		log.Printf("[严格模式] ✅ 恢复正常模式")

		if m.inboundEngine != nil && m.origInbound != nil {
			source := m.inboundEngine.Version().Source
			m.inboundEngine.Reload(m.origInbound, source)
			m.origInbound = nil
		}

		if m.llmRuleEngine != nil && m.origLLMRules != nil {
			m.llmRuleEngine.UpdateRules(m.origLLMRules)
			m.origLLMRules = nil
		}
	}

	m.enabled = enabled
}

// ============================================================
// 功能六：通知中心
// ============================================================

// NotificationItem 通知项
type NotificationItem struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`    // canary_leak / budget_exceeded / blocked / high_risk_tool
	TypeLabel string `json:"type_label"`
	Severity  string `json:"severity"` // critical / high / medium / low
	Summary   string `json:"summary"`
	Detail    string `json:"detail"`
}

// NotificationEngine 通知引擎
type NotificationEngine struct {
	db *sql.DB
}

// NewNotificationEngine 创建
func NewNotificationEngine(db *sql.DB) *NotificationEngine {
	return &NotificationEngine{db: db}
}

// GetRecentNotifications 获取最近 24h 的通知
func (e *NotificationEngine) GetRecentNotifications() []NotificationItem {
	since24h := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	var items []NotificationItem
	idCounter := 0

	// 1. Canary Token 泄露事件
	canaryRows, err := e.db.Query(`SELECT timestamp, tool_name, flag_reason FROM llm_tool_calls WHERE flagged=1 AND flag_reason LIKE '%canary%' AND timestamp >= ? ORDER BY timestamp DESC LIMIT 20`, since24h)
	if err == nil {
		defer canaryRows.Close()
		for canaryRows.Next() {
			var ts, toolName, reason string
			canaryRows.Scan(&ts, &toolName, &reason)
			idCounter++
			items = append(items, NotificationItem{
				ID:        fmt.Sprintf("n-%d", idCounter),
				Timestamp: ts, Type: "canary_leak", TypeLabel: "Canary 泄露",
				Severity: "critical",
				Summary:  fmt.Sprintf("Canary Token 泄露: %s", toolName),
				Detail:   reason,
			})
		}
	}

	// 2. 预算超限事件
	budgetRows, err := e.db.Query(`SELECT timestamp, tool_name, flag_reason FROM llm_tool_calls WHERE flagged=1 AND flag_reason LIKE '%budget%' AND timestamp >= ? ORDER BY timestamp DESC LIMIT 20`, since24h)
	if err == nil {
		defer budgetRows.Close()
		for budgetRows.Next() {
			var ts, toolName, reason string
			budgetRows.Scan(&ts, &toolName, &reason)
			idCounter++
			items = append(items, NotificationItem{
				ID:        fmt.Sprintf("n-%d", idCounter),
				Timestamp: ts, Type: "budget_exceeded", TypeLabel: "预算超限",
				Severity: "high",
				Summary:  fmt.Sprintf("Agent 预算超限: %s", toolName),
				Detail:   reason,
			})
		}
	}

	// 3. IM 拦截事件（只取最近的重要的）
	blockRows, err := e.db.Query(`SELECT timestamp, sender_id, reason FROM audit_log WHERE action='block' AND timestamp >= ? ORDER BY id DESC LIMIT 20`, since24h)
	if err == nil {
		defer blockRows.Close()
		for blockRows.Next() {
			var ts, sender, reason string
			blockRows.Scan(&ts, &sender, &reason)
			idCounter++
			severity := "medium"
			if len(reason) > 0 {
				for _, kw := range []string{"injection", "jailbreak", "xss", "SQL"} {
					if containsCI(reason, kw) {
						severity = "high"
						break
					}
				}
			}
			summary := fmt.Sprintf("IM 拦截: %s", sender)
			if len(reason) > 60 {
				reason = reason[:60] + "..."
			}
			items = append(items, NotificationItem{
				ID:        fmt.Sprintf("n-%d", idCounter),
				Timestamp: ts, Type: "blocked", TypeLabel: "IM 拦截",
				Severity: severity,
				Summary:  summary,
				Detail:   reason,
			})
		}
	}

	// 4. 报告生成完成 (v12.0)
	rptRows, err := e.db.Query(`SELECT id, title, created_at FROM reports WHERE status='ready' AND created_at >= ? ORDER BY created_at DESC LIMIT 5`, since24h)
	if err == nil {
		defer rptRows.Close()
		for rptRows.Next() {
			var rptID, rptTitle, rptCreated string
			rptRows.Scan(&rptID, &rptTitle, &rptCreated)
			idCounter++
			items = append(items, NotificationItem{
				ID:        fmt.Sprintf("n-%d", idCounter),
				Timestamp: rptCreated, Type: "report_ready", TypeLabel: "报告就绪",
				Severity: "low",
				Summary:  fmt.Sprintf("报告已生成: %s", rptTitle),
				Detail:   rptID,
			})
		}
	}

	// 5. 高风险工具调用
	hrRows, err := e.db.Query(`SELECT timestamp, tool_name FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp >= ? ORDER BY timestamp DESC LIMIT 10`, since24h)
	if err == nil {
		defer hrRows.Close()
		for hrRows.Next() {
			var ts, toolName string
			hrRows.Scan(&ts, &toolName)
			idCounter++
			items = append(items, NotificationItem{
				ID:        fmt.Sprintf("n-%d", idCounter),
				Timestamp: ts, Type: "high_risk_tool", TypeLabel: "高危工具",
				Severity: "high",
				Summary:  fmt.Sprintf("高风险工具调用: %s", toolName),
			})
		}
	}

	return items
}

// containsCI case-insensitive contains
func containsCI(s, substr string) bool {
	sl := len(s)
	sl2 := len(substr)
	if sl2 > sl {
		return false
	}
	for i := 0; i <= sl-sl2; i++ {
		match := true
		for j := 0; j < sl2; j++ {
			c1, c2 := s[i+j], substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ============================================================
// 功能五: 系统健康指标增强
// ============================================================

// SystemHealthInfo 系统健康信息（CPU/内存/磁盘/协程）
type SystemHealthInfo struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB float64 `json:"memory_used_mb"`
	MemoryTotalMB float64 `json:"memory_total_mb"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	Goroutines    int     `json:"goroutines"`
}

// GetSystemHealth 获取系统健康指标
func GetSystemHealth(dbPath string) *SystemHealthInfo {
	info := &SystemHealthInfo{}

	// Memory
	allocMB := getMemoryAllocMB()
	info.MemoryUsedMB = allocMB
	// 估算总内存（从 /proc/meminfo 或简单固定）
	info.MemoryTotalMB = 1024 // 默认值
	info.MemoryPercent = allocMB / info.MemoryTotalMB * 100
	if info.MemoryPercent > 100 {
		info.MemoryPercent = 100
	}

	// Disk
	info.DiskUsedPercent = getDiskUsagePercent(dbPath)

	// Goroutines
	info.Goroutines = getGoroutineCount()

	// CPU: 简单返回基于 goroutine 数量的估算
	// 真正的 CPU 采样需要等待间隔，这里简化
	info.CPUPercent = float64(info.Goroutines) / 100.0 * 10
	if info.CPUPercent > 100 {
		info.CPUPercent = 100
	}

	return info
}
