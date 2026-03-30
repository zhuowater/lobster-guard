// upstream_profile.go — 上游实例安全画像引擎
// lobster-guard v33.0 — 聚合全部引擎表数据，生成 per-upstream 安全态势
package main

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// ============================================================
// 类型定义
// ============================================================

type UpstreamSecurityProfile struct {
	UpstreamID    string              `json:"upstream_id"`
	UpdatedAt     string              `json:"updated_at"`
	SecurityScore float64             `json:"security_score"`
	RiskLevel     string              `json:"risk_level"`
	UserCount     int                 `json:"user_count"`
	Dimensions    []SecurityDimension `json:"dimensions"`
	Traffic       TrafficOverview     `json:"traffic"`
	EngineAlerts  EngineAlertSummary  `json:"engine_alerts"`
	TopRiskEvents []RiskEvent         `json:"top_risk_events"`
	Trend         []DailyTrend        `json:"trend"`
}

type SecurityDimension struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Level   string  `json:"level"`
	Details string  `json:"details"`
	Alerts  int     `json:"alerts"`
	Icon    string  `json:"icon"`
}

type TrafficOverview struct {
	TotalIMRequests  int `json:"total_im_requests"`
	TotalLLMCalls    int `json:"total_llm_calls"`
	TotalToolCalls   int `json:"total_tool_calls"`
	BlockedRequests  int `json:"blocked_requests"`
	WarnedRequests   int `json:"warned_requests"`
	ReviewedRequests int `json:"reviewed_requests"`
}

type EngineAlertSummary struct {
	// 入站
	InboundDetections int `json:"inbound_detections"`
	AttackChains      int `json:"attack_chains"`
	// LLM
	LLMRuleHits        int `json:"llm_rule_hits"`
	SingularityExposes int `json:"singularity_exposes"`
	HoneypotDeep       int `json:"honeypot_deep"`
	// 数据
	IFCViolations    int `json:"ifc_violations"`
	IFCHidden        int `json:"ifc_hidden"`
	TaintEvents      int `json:"taint_events"`
	TaintReversals   int `json:"taint_reversals"`
	OutboundBlocks   int `json:"outbound_blocks"`
	// 行为
	PlanDeviations    int `json:"plan_deviations"`
	PlanExecutions    int `json:"plan_executions"`
	CapabilityDenials int `json:"capability_denials"`
	CapabilityTotal   int `json:"capability_total"`
	BehaviorAnomalies int `json:"behavior_anomalies"`
	// 工具
	EnvelopeFailures    int `json:"envelope_failures"`
	EnvelopeTotal       int `json:"envelope_total"`
	CounterfactualFlags int `json:"counterfactual_flags"`
	EvolutionRules      int `json:"evolution_rules"`
}

type RiskEvent struct {
	Timestamp string `json:"timestamp"`
	Engine    string `json:"engine"`
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
	TraceID   string `json:"trace_id"`
}

type DailyTrend struct {
	Date          string  `json:"date"`
	SecurityScore float64 `json:"security_score"`
	Alerts        int     `json:"alerts"`
	Blocks        int     `json:"blocks"`
}

// ============================================================
// 引擎
// ============================================================

type UpstreamProfileEngine struct {
	db *sql.DB
}

func NewUpstreamProfileEngine(db *sql.DB) *UpstreamProfileEngine {
	return &UpstreamProfileEngine{db: db}
}

// BuildProfile 构建单个上游的安全画像（24h 窗口）
func (e *UpstreamProfileEngine) BuildProfile(upstreamID string) (*UpstreamSecurityProfile, error) {
	now := time.Now().UTC()
	since24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	since7d := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)

	p := &UpstreamSecurityProfile{
		UpstreamID: upstreamID,
		UpdatedAt:  now.Format(time.RFC3339),
	}

	// 0. 绑定用户数
	e.db.QueryRow(`SELECT COUNT(*) FROM user_routes WHERE upstream_id=?`, upstreamID).Scan(&p.UserCount)

	// 1. 流量概览 (24h)
	p.Traffic = e.queryTraffic(upstreamID, since24h)

	// 2. 引擎告警汇总 (24h) — 全量引擎
	p.EngineAlerts = e.queryEngineAlerts(since24h)

	// 3. 计算 5 维度评分
	p.Dimensions = e.calcDimensions(p.Traffic, p.EngineAlerts)

	// 4. 综合评分
	total := 0.0
	for _, d := range p.Dimensions {
		total += d.Score
	}
	p.SecurityScore = math.Round(total*10) / 10
	p.RiskLevel = upstreamScoreToLevel(p.SecurityScore)

	// 5. Top 风险事件 (24h)
	p.TopRiskEvents = e.queryTopRiskEvents(since24h)

	// 6. 7 天趋势
	p.Trend = e.queryTrend(upstreamID, since7d)

	return p, nil
}

// ListProfiles 所有上游的安全画像概览
func (e *UpstreamProfileEngine) ListProfiles(upstreamIDs []string) []UpstreamSecurityProfile {
	var profiles []UpstreamSecurityProfile
	for _, id := range upstreamIDs {
		p, err := e.BuildProfile(id)
		if err != nil {
			continue
		}
		profiles = append(profiles, *p)
	}
	return profiles
}

// ============================================================
// 数据查询 — 全量引擎聚合
// ============================================================

func (e *UpstreamProfileEngine) queryTraffic(upstreamID, since string) TrafficOverview {
	var t TrafficOverview
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction='inbound' AND timestamp>=?`, since).Scan(&t.TotalIMRequests)
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE timestamp>=?`, since).Scan(&t.TotalLLMCalls)
	e.db.QueryRow(`SELECT COUNT(*) FROM tool_call_events WHERE timestamp>=?`, since).Scan(&t.TotalToolCalls)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='block' AND timestamp>=?`, since).Scan(&t.BlockedRequests)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='warn' AND timestamp>=?`, since).Scan(&t.WarnedRequests)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='review' AND timestamp>=?`, since).Scan(&t.ReviewedRequests)
	return t
}

func (e *UpstreamProfileEngine) queryEngineAlerts(since string) EngineAlertSummary {
	var a EngineAlertSummary

	// ——— 入站层 ———
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction='inbound' AND action IN ('block','warn','review') AND timestamp>=?`, since).Scan(&a.InboundDetections)
	e.db.QueryRow(`SELECT COUNT(*) FROM attack_chains WHERE timestamp>=?`, since).Scan(&a.AttackChains)

	// ——— LLM 层 ———
	e.db.QueryRow(`SELECT COUNT(*) FROM llm_rule_hits WHERE last_hit>=?`, since).Scan(&a.LLMRuleHits)
	e.db.QueryRow(`SELECT COUNT(*) FROM singularity_history WHERE timestamp>=?`, since).Scan(&a.SingularityExposes)
	e.db.QueryRow(`SELECT COUNT(*) FROM honeypot_interactions WHERE timestamp>=?`, since).Scan(&a.HoneypotDeep)

	// ——— 数据层 ———
	e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE timestamp>=?`, since).Scan(&a.IFCViolations)
	e.db.QueryRow(`SELECT COUNT(*) FROM ifc_hidden_content WHERE timestamp>=?`, since).Scan(&a.IFCHidden)
	e.db.QueryRow(`SELECT COUNT(*) FROM taint_entries WHERE timestamp>=?`, since).Scan(&a.TaintEvents)
	e.db.QueryRow(`SELECT COUNT(*) FROM taint_reversals WHERE timestamp>=?`, since).Scan(&a.TaintReversals)
	e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction='outbound' AND action='block' AND timestamp>=?`, since).Scan(&a.OutboundBlocks)

	// ——— 行为层 ———
	e.db.QueryRow(`SELECT COUNT(*) FROM plan_deviations WHERE timestamp>=?`, since).Scan(&a.PlanDeviations)
	e.db.QueryRow(`SELECT COUNT(*) FROM plan_executions WHERE timestamp>=?`, since).Scan(&a.PlanExecutions)
	e.db.QueryRow(`SELECT COUNT(*) FROM cap_evaluations WHERE decision='deny' AND timestamp>=?`, since).Scan(&a.CapabilityDenials)
	e.db.QueryRow(`SELECT COUNT(*) FROM cap_evaluations WHERE timestamp>=?`, since).Scan(&a.CapabilityTotal)
	e.db.QueryRow(`SELECT COUNT(*) FROM behavior_anomalies WHERE timestamp>=?`, since).Scan(&a.BehaviorAnomalies)

	// ——— 工具层 ———
	e.db.QueryRow(`SELECT COUNT(*) FROM execution_envelopes WHERE decision NOT IN ('pass','allow','expose') AND timestamp>=?`, since).Scan(&a.EnvelopeFailures)
	e.db.QueryRow(`SELECT COUNT(*) FROM execution_envelopes WHERE timestamp>=?`, since).Scan(&a.EnvelopeTotal)
	e.db.QueryRow(`SELECT COUNT(*) FROM cf_verifications WHERE timestamp>=?`, since).Scan(&a.CounterfactualFlags)
	e.db.QueryRow(`SELECT COUNT(*) FROM evolution_log WHERE phase='apply' AND timestamp>=?`, since).Scan(&a.EvolutionRules)

	return a
}

// ============================================================
// 评分计算 — 全引擎纳入
// ============================================================

func (e *UpstreamProfileEngine) calcDimensions(traffic TrafficOverview, a EngineAlertSummary) []SecurityDimension {
	dims := make([]SecurityDimension, 5)

	// ===== D1: 入站防护 (20分) =====
	// 负面: 入站检测命中 + 攻击链
	// 攻击链每条扣 1 分(上限 10), 入站 block 每条扣 2(上限 15)
	d1Score := 20.0
	d1Score -= math.Min(float64(a.AttackChains)*1.0, 10.0)
	d1Score -= math.Min(float64(traffic.BlockedRequests)*2.0, 15.0)
	warnOnly := a.InboundDetections - traffic.BlockedRequests
	if warnOnly > 0 {
		d1Score -= math.Min(float64(warnOnly)*0.5, 5.0)
	}
	d1Score = math.Max(d1Score, 0)
	d1Alerts := a.InboundDetections + a.AttackChains
	dims[0] = SecurityDimension{
		Name: "入站防护", Icon: "🛡️", Score: math.Round(d1Score*10) / 10, Alerts: d1Alerts,
		Level:   dimLevel(d1Score),
		Details: fmt.Sprintf("检测命中 %d, 拦截 %d, 攻击链 %d", a.InboundDetections, traffic.BlockedRequests, a.AttackChains),
	}

	// ===== D2: LLM 安全 (20分) =====
	// 负面: LLM规则命中 + 蜜罐暴露 + 蜜罐深度交互
	d2Score := 20.0
	d2Score -= math.Min(float64(a.LLMRuleHits)*0.5, 8.0)
	d2Score -= math.Min(float64(a.SingularityExposes)*2.0, 10.0)
	d2Score -= math.Min(float64(a.HoneypotDeep)*3.0, 6.0)
	d2Score = math.Max(d2Score, 0)
	d2Alerts := a.LLMRuleHits + a.SingularityExposes + a.HoneypotDeep
	dims[1] = SecurityDimension{
		Name: "LLM 安全", Icon: "🤖", Score: math.Round(d2Score*10) / 10, Alerts: d2Alerts,
		Level:   dimLevel(d2Score),
		Details: fmt.Sprintf("LLM规则 %d, 蜜罐暴露 %d, 深度交互 %d", a.LLMRuleHits, a.SingularityExposes, a.HoneypotDeep),
	}

	// ===== D3: 数据防泄漏 (20分) =====
	// 负面: IFC违规 + 出站拦截
	// 正面: 污染逆转 + IFC隐藏（每条加回 0.5 分，上限 +3）
	d3Score := 20.0
	d3Score -= math.Min(float64(a.IFCViolations)*2.0, 12.0)
	d3Score -= math.Min(float64(a.OutboundBlocks)*1.5, 8.0)
	d3Score -= math.Min(float64(a.TaintEvents)*0.5, 5.0)
	// 正面信号: 逆转和隐藏说明防御在工作
	positiveBonus := math.Min(float64(a.TaintReversals+a.IFCHidden)*0.5, 3.0)
	d3Score += positiveBonus
	d3Score = math.Max(math.Min(d3Score, 20), 0)
	d3Alerts := a.IFCViolations + a.TaintEvents + a.OutboundBlocks
	d3Positive := a.TaintReversals + a.IFCHidden
	dims[2] = SecurityDimension{
		Name: "数据防泄漏", Icon: "🔒", Score: math.Round(d3Score*10) / 10, Alerts: d3Alerts,
		Level:   dimLevel(d3Score),
		Details: fmt.Sprintf("IFC违规 %d, 污染 %d, 出站拦截 %d, 逆转+隐藏 %d", a.IFCViolations, a.TaintEvents, a.OutboundBlocks, d3Positive),
	}

	// ===== D4: 行为合规 (20分) =====
	// 负面: 计划偏离率 + 能力拒绝率 + 行为异常
	d4Score := 20.0
	// 计划偏离率: deviations/executions
	if a.PlanExecutions > 0 {
		deviationRate := float64(a.PlanDeviations) / float64(a.PlanExecutions)
		d4Score -= math.Min(deviationRate*40, 8.0) // 20% 偏离率扣 8 分
	}
	// 能力拒绝率: denials/total
	if a.CapabilityTotal > 0 {
		denyRate := float64(a.CapabilityDenials) / float64(a.CapabilityTotal)
		d4Score -= math.Min(denyRate*20, 6.0) // 30% 拒绝率扣 6 分
	}
	// 行为异常: 每条 -0.1 (上限 6), 大量异常才扣分
	d4Score -= math.Min(float64(a.BehaviorAnomalies)*0.1, 6.0)
	d4Score = math.Max(d4Score, 0)
	d4Alerts := a.PlanDeviations + a.CapabilityDenials + a.BehaviorAnomalies
	dims[3] = SecurityDimension{
		Name: "行为合规", Icon: "📋", Score: math.Round(d4Score*10) / 10, Alerts: d4Alerts,
		Level:   dimLevel(d4Score),
		Details: fmt.Sprintf("偏离 %d/%d, 拒绝 %d/%d, 异常 %d", a.PlanDeviations, a.PlanExecutions, a.CapabilityDenials, a.CapabilityTotal, a.BehaviorAnomalies),
	}

	// ===== D5: 工具管控 (20分) =====
	// 负面: 信封失败率 + 反事实标记
	// 中性: 进化规则生成（有进化说明系统在自我改进，不扣分）
	d5Score := 20.0
	if a.EnvelopeTotal > 0 {
		failRate := float64(a.EnvelopeFailures) / float64(a.EnvelopeTotal)
		d5Score -= math.Min(failRate*40, 12.0)
	}
	d5Score -= math.Min(float64(a.CounterfactualFlags)*3.0, 9.0)
	d5Score = math.Max(d5Score, 0)
	d5Alerts := a.EnvelopeFailures + a.CounterfactualFlags
	dims[4] = SecurityDimension{
		Name: "工具管控", Icon: "🔧", Score: math.Round(d5Score*10) / 10, Alerts: d5Alerts,
		Level:   dimLevel(d5Score),
		Details: fmt.Sprintf("信封 %d/%d 失败, 反事实 %d, 进化规则 %d", a.EnvelopeFailures, a.EnvelopeTotal, a.CounterfactualFlags, a.EvolutionRules),
	}

	return dims
}

func dimLevel(score float64) string {
	switch {
	case score >= 18:
		return "safe"
	case score >= 14:
		return "low"
	case score >= 10:
		return "medium"
	case score >= 6:
		return "high"
	default:
		return "critical"
	}
}

func upstreamScoreToLevel(score float64) string {
	switch {
	case score >= 90:
		return "safe"
	case score >= 70:
		return "low"
	case score >= 50:
		return "medium"
	case score >= 30:
		return "high"
	default:
		return "critical"
	}
}

// ============================================================
// Top 风险事件 — 全引擎来源
// ============================================================

func (e *UpstreamProfileEngine) queryTopRiskEvents(since string) []RiskEvent {
	var events []RiskEvent

	rows, err := e.db.Query(`
		SELECT timestamp, engine, severity, summary, trace_id FROM (
			SELECT timestamp, 'inbound' as engine,
				CASE WHEN action='block' THEN 'high' ELSE 'medium' END as severity,
				COALESCE(reason, content_preview, '') as summary,
				COALESCE(trace_id,'') as trace_id
			FROM audit_log
			WHERE action IN ('block','review') AND timestamp>=?
			UNION ALL
			SELECT timestamp, 'ifc' as engine, 'high' as severity,
				'IFC ' || type || ' 违规: ' || tool as summary,
				trace_id
			FROM ifc_violations WHERE timestamp>=?
			UNION ALL
			SELECT timestamp, 'singularity' as engine, 'high' as severity,
				'蜜罐暴露 channel=' || channel || ' level=' || level as summary,
				CAST(id AS TEXT) as trace_id
			FROM singularity_history WHERE timestamp>=?
			UNION ALL
			SELECT timestamp, 'taint' as engine, 'medium' as severity,
				'污染追踪: ' || tool || ' label=' || label as summary,
				trace_id
			FROM taint_entries WHERE timestamp>=?
			UNION ALL
			SELECT timestamp, 'plan' as engine, 'high' as severity,
				'计划偏离: ' || deviation_type || ' — ' || COALESCE(description,'') as summary,
				trace_id
			FROM plan_deviations WHERE timestamp>=?
			UNION ALL
			SELECT timestamp, 'attack_chain' as engine, 'high' as severity,
				'攻击链: stage=' || COALESCE(stage,'') || ' pattern=' || COALESCE(pattern,'') as summary,
				COALESCE(chain_id,'') as trace_id
			FROM attack_chains WHERE timestamp>=?
			UNION ALL
			SELECT timestamp, 'behavior' as engine, 'medium' as severity,
				'行为异常: ' || anomaly_type || ' — ' || COALESCE(description,'') as summary,
				COALESCE(agent_id,'') as trace_id
			FROM behavior_anomalies WHERE severity IN ('high','critical') AND timestamp>=?
		) combined
		ORDER BY timestamp DESC LIMIT 10
	`, since, since, since, since, since, since, since)
	if err != nil {
		return events
	}
	defer rows.Close()
	for rows.Next() {
		var ev RiskEvent
		if rows.Scan(&ev.Timestamp, &ev.Engine, &ev.Severity, &ev.Summary, &ev.TraceID) == nil {
			events = append(events, ev)
		}
	}
	return events
}

// ============================================================
// 7 天趋势 — 综合多表
// ============================================================

func (e *UpstreamProfileEngine) queryTrend(upstreamID, since string) []DailyTrend {
	var trend []DailyTrend
	now := time.Now().UTC()
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStr := day.Format("2006-01-02")
		dayStart := dayStr + "T00:00:00Z"
		dayEnd := dayStr + "T23:59:59Z"

		var imAlerts, blocks int
		e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('block','warn','review') AND timestamp BETWEEN ? AND ?`,
			dayStart, dayEnd).Scan(&imAlerts)
		e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='block' AND timestamp BETWEEN ? AND ?`,
			dayStart, dayEnd).Scan(&blocks)

		// 跨引擎告警数
		var ifcV, planD, anomalies int
		e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE timestamp BETWEEN ? AND ?`, dayStart, dayEnd).Scan(&ifcV)
		e.db.QueryRow(`SELECT COUNT(*) FROM plan_deviations WHERE timestamp BETWEEN ? AND ?`, dayStart, dayEnd).Scan(&planD)
		e.db.QueryRow(`SELECT COUNT(*) FROM behavior_anomalies WHERE timestamp BETWEEN ? AND ?`, dayStart, dayEnd).Scan(&anomalies)

		totalAlerts := imAlerts + ifcV + planD + anomalies

		// 日评分: 基础 100
		score := 100.0
		score -= float64(blocks) * 5.0                      // block 重扣
		score -= float64(imAlerts-blocks) * 1.0              // warn/review 轻扣
		score -= float64(ifcV) * 3.0                         // IFC 违规
		score -= float64(planD) * 2.0                        // 计划偏离
		score -= math.Min(float64(anomalies)*0.1, 10.0)     // 行为异常（封顶）
		score = math.Max(score, 0)

		trend = append(trend, DailyTrend{
			Date:          dayStr,
			SecurityScore: math.Round(score*10) / 10,
			Alerts:        totalAlerts,
			Blocks:        blocks,
		})
	}
	return trend
}
