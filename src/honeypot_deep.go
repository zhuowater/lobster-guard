// honeypot_deep.go — 蜜罐深度交互引擎：忠诚度曲线 + 攻击者画像 + 自进化回馈
// lobster-guard v19.2
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 类型定义
// ============================================================

// HoneypotDeepEngine 蜜罐深度交互引擎
type HoneypotDeepEngine struct {
	db              *sql.DB
	honeypot        *HoneypotEngine  // 已有蜜罐引擎
	evolutionEngine *EvolutionEngine // v19.0 自进化引擎（回馈攻击向量）
	eventBus        *EventBus
	cfg             HoneypotDeepConfig
	mu              sync.RWMutex
}

// HoneypotDeepConfig 蜜罐深度交互配置
type HoneypotDeepConfig struct {
	Enabled              bool    `yaml:"enabled" json:"enabled"`
	AutoFeedbackEnabled  bool    `yaml:"auto_feedback_enabled" json:"auto_feedback_enabled"`
	AutoFeedbackMinScore float64 `yaml:"auto_feedback_min_score" json:"auto_feedback_min_score"`
	SessionTimeoutMin    int     `yaml:"session_timeout_min" json:"session_timeout_min"`
}

// HoneypotInteraction 蜜罐交互记录
type HoneypotInteraction struct {
	ID           string    `json:"id"`
	AttackerID   string    `json:"attacker_id"`
	HoneypotType string    `json:"honeypot_type"`
	Channel      string    `json:"channel"`
	Payload      string    `json:"payload"`
	Depth        int       `json:"depth"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
}

// LoyaltyCurve 忠诚度曲线
type LoyaltyCurve struct {
	AttackerID        string            `json:"attacker_id"`
	TotalInteractions int               `json:"total_interactions"`
	FirstSeen         time.Time         `json:"first_seen"`
	LastSeen          time.Time         `json:"last_seen"`
	DurationHours     float64           `json:"duration_hours"`
	MaxDepth          int               `json:"max_depth"`
	AvgDepth          float64           `json:"avg_depth"`
	Frequency         float64           `json:"frequency"`
	LoyaltyScore      float64           `json:"loyalty_score"`
	Phase             string            `json:"phase"`
	PhaseHistory      []PhaseTransition `json:"phase_history"`
}

// PhaseTransition 阶段变迁
type PhaseTransition struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Timestamp time.Time `json:"timestamp"`
	Trigger   string    `json:"trigger"`
}

// HoneypotDeepStats 蜜罐深度统计
type HoneypotDeepStats struct {
	TotalInteractions int            `json:"total_interactions"`
	TotalAttackers    int            `json:"total_attackers"`
	TotalSessions     int            `json:"total_sessions"`
	AvgLoyaltyScore   float64        `json:"avg_loyalty_score"`
	PhaseDistribution map[string]int `json:"phase_distribution"`
	ChannelBreakdown  map[string]int `json:"channel_breakdown"`
	DepthBreakdown    map[int]int    `json:"depth_breakdown"`
	TopAttackers      []LoyaltyCurve `json:"top_attackers"`
}

// ============================================================
// 常量
// ============================================================

const (
	PhaseProbe    = "probe"
	PhaseInterest = "interest"
	PhaseEngage   = "engage"
	PhaseDeepDive = "deep_dive"
	PhaseAbandon  = "abandon"
)

// ============================================================
// 构造 + Schema
// ============================================================

// NewHoneypotDeepEngine 创建蜜罐深度交互引擎
func NewHoneypotDeepEngine(db *sql.DB, honeypot *HoneypotEngine, evolution *EvolutionEngine, eventBus *EventBus, cfg HoneypotDeepConfig) *HoneypotDeepEngine {
	if cfg.SessionTimeoutMin <= 0 {
		cfg.SessionTimeoutMin = 30
	}
	if cfg.AutoFeedbackMinScore <= 0 {
		cfg.AutoFeedbackMinScore = 50
	}
	hd := &HoneypotDeepEngine{
		db:              db,
		honeypot:        honeypot,
		evolutionEngine: evolution,
		eventBus:        eventBus,
		cfg:             cfg,
	}
	hd.initSchema()
	return hd
}

func (hd *HoneypotDeepEngine) initSchema() {
	hd.db.Exec(`CREATE TABLE IF NOT EXISTS honeypot_interactions (
		id TEXT PRIMARY KEY,
		attacker_id TEXT NOT NULL,
		honeypot_type TEXT DEFAULT '',
		channel TEXT DEFAULT '',
		payload TEXT DEFAULT '',
		depth INTEGER DEFAULT 1,
		session_id TEXT DEFAULT '',
		timestamp TEXT NOT NULL
	)`)
	hd.db.Exec(`CREATE INDEX IF NOT EXISTS idx_hp_attacker ON honeypot_interactions(attacker_id)`)
	hd.db.Exec(`CREATE INDEX IF NOT EXISTS idx_hp_ts ON honeypot_interactions(timestamp)`)
	hd.db.Exec(`CREATE INDEX IF NOT EXISTS idx_hp_session ON honeypot_interactions(session_id)`)
	log.Println("[初始化] ✅ 蜜罐深度交互引擎 schema 就绪")
}

// ============================================================
// RecordInteraction — 核心记录方法
// ============================================================

// RecordInteraction 记录蜜罐交互
func (hd *HoneypotDeepEngine) RecordInteraction(attackerID, honeypotType, channel, payload string) *HoneypotInteraction {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	now := time.Now().UTC()
	id := uuid.New().String()

	// 自动判断 depth
	depth := hd.computeDepth(attackerID, honeypotType)

	// 自动判断 session_id
	sessionID := hd.resolveSessionID(attackerID, now)

	interaction := &HoneypotInteraction{
		ID:           id,
		AttackerID:   attackerID,
		HoneypotType: honeypotType,
		Channel:      channel,
		Payload:      payload,
		Depth:        depth,
		Timestamp:    now,
		SessionID:    sessionID,
	}

	_, err := hd.db.Exec(`INSERT INTO honeypot_interactions (id, attacker_id, honeypot_type, channel, payload, depth, session_id, timestamp) VALUES (?,?,?,?,?,?,?,?)`,
		id, attackerID, honeypotType, channel, payload, depth, sessionID, now.Format(time.RFC3339Nano))
	if err != nil {
		log.Printf("[蜜罐深度] 记录交互失败: %v", err)
		return nil
	}

	// 发射事件
	if hd.eventBus != nil {
		hd.eventBus.Emit(&SecurityEvent{
			Type:     "honeypot_deep_interaction",
			Severity: "info",
			Domain:   "honeypot",
			SenderID: attackerID,
			Summary:  fmt.Sprintf("蜜罐深度交互: depth=%d type=%s channel=%s", depth, honeypotType, channel),
			Details: map[string]interface{}{
				"attacker_id":   attackerID,
				"honeypot_type": honeypotType,
				"depth":         depth,
				"session_id":    sessionID,
			},
		})
	}

	return interaction
}

// RecordInteractionAt 记录蜜罐交互（指定时间，用于测试）
func (hd *HoneypotDeepEngine) RecordInteractionAt(attackerID, honeypotType, channel, payload string, at time.Time) *HoneypotInteraction {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	id := uuid.New().String()
	depth := hd.computeDepth(attackerID, honeypotType)
	sessionID := hd.resolveSessionID(attackerID, at)

	interaction := &HoneypotInteraction{
		ID:           id,
		AttackerID:   attackerID,
		HoneypotType: honeypotType,
		Channel:      channel,
		Payload:      payload,
		Depth:        depth,
		Timestamp:    at,
		SessionID:    sessionID,
	}

	_, err := hd.db.Exec(`INSERT INTO honeypot_interactions (id, attacker_id, honeypot_type, channel, payload, depth, session_id, timestamp) VALUES (?,?,?,?,?,?,?,?)`,
		id, attackerID, honeypotType, channel, payload, depth, sessionID, at.Format(time.RFC3339Nano))
	if err != nil {
		log.Printf("[蜜罐深度] 记录交互失败: %v", err)
		return nil
	}

	return interaction
}

// computeDepth 计算交互深度
// depth=1: 首次探测（只触碰过 1 种蜜罐类型）
// depth=2: 持续尝试（触碰过 2 种不同蜜罐类型，或同类型 >= 3 次）
// depth=3: 深入渗透（触碰过 3+ 种不同蜜罐类型，或任意类型 >= 6 次）
func (hd *HoneypotDeepEngine) computeDepth(attackerID, currentType string) int {
	var distinctTypes int
	var totalCount int
	hd.db.QueryRow(`SELECT COUNT(DISTINCT honeypot_type), COUNT(*) FROM honeypot_interactions WHERE attacker_id = ?`, attackerID).Scan(&distinctTypes, &totalCount)

	// 加上当前这次（还没插入）
	var typeExists int
	hd.db.QueryRow(`SELECT COUNT(*) FROM honeypot_interactions WHERE attacker_id = ? AND honeypot_type = ?`, attackerID, currentType).Scan(&typeExists)
	if typeExists == 0 {
		distinctTypes++
	}
	totalCount++

	if distinctTypes >= 3 || totalCount >= 6 {
		return 3
	}
	if distinctTypes >= 2 || totalCount >= 3 {
		return 2
	}
	return 1
}

// resolveSessionID 解析 session ID
// 同一攻击者在 SessionTimeoutMin 分钟内的交互归为同一 session
func (hd *HoneypotDeepEngine) resolveSessionID(attackerID string, now time.Time) string {
	timeoutDuration := time.Duration(hd.cfg.SessionTimeoutMin) * time.Minute
	cutoff := now.Add(-timeoutDuration).Format(time.RFC3339Nano)

	var lastSessionID string
	var lastTimestamp string
	err := hd.db.QueryRow(
		`SELECT session_id, timestamp FROM honeypot_interactions WHERE attacker_id = ? AND timestamp > ? ORDER BY timestamp DESC LIMIT 1`,
		attackerID, cutoff).Scan(&lastSessionID, &lastTimestamp)

	if err == nil && lastSessionID != "" {
		return lastSessionID
	}

	// 创建新 session
	return fmt.Sprintf("sess-%s", uuid.New().String()[:8])
}

// ============================================================
// LoyaltyCurve — 忠诚度曲线计算
// ============================================================

// GetLoyaltyCurve 计算指定攻击者的忠诚度曲线
func (hd *HoneypotDeepEngine) GetLoyaltyCurve(attackerID string) *LoyaltyCurve {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	return hd.getLoyaltyCurveInternal(attackerID)
}

func (hd *HoneypotDeepEngine) getLoyaltyCurveInternal(attackerID string) *LoyaltyCurve {
	curve := &LoyaltyCurve{
		AttackerID:   attackerID,
		PhaseHistory: []PhaseTransition{},
	}

	var totalInteractions int
	var firstSeen, lastSeen string
	var maxDepth int
	var sumDepth int

	err := hd.db.QueryRow(`SELECT COUNT(*), MIN(timestamp), MAX(timestamp), MAX(depth), SUM(depth) FROM honeypot_interactions WHERE attacker_id = ?`,
		attackerID).Scan(&totalInteractions, &firstSeen, &lastSeen, &maxDepth, &sumDepth)
	if err != nil || totalInteractions == 0 {
		curve.Phase = PhaseProbe
		return curve
	}

	curve.TotalInteractions = totalInteractions
	curve.MaxDepth = maxDepth

	first := parseTimestamp(firstSeen)
	last := parseTimestamp(lastSeen)
	curve.FirstSeen = first
	curve.LastSeen = last

	durationHours := last.Sub(first).Hours()
	if durationHours < 0 {
		durationHours = 0
	}
	curve.DurationHours = durationHours

	if totalInteractions > 0 {
		curve.AvgDepth = float64(sumDepth) / float64(totalInteractions)
	}

	if durationHours > 0 {
		curve.Frequency = float64(totalInteractions) / durationHours
	} else if totalInteractions > 0 {
		curve.Frequency = float64(totalInteractions)
	}

	curve.LoyaltyScore = calculateLoyaltyScore(curve.Frequency, float64(maxDepth), durationHours)
	curve.Phase = determinePhase(curve)
	curve.PhaseHistory = hd.buildPhaseHistory(attackerID)

	return curve
}

// calculateLoyaltyScore 纯统计忠诚度评分
func calculateLoyaltyScore(frequency, maxDepth, durationHours float64) float64 {
	freqScore := math.Min(100, frequency*20)
	depthScore := math.Min(100, maxDepth*33.3)
	durationScore := math.Min(100, durationHours*10)

	loyalty := freqScore*0.3 + depthScore*0.4 + durationScore*0.3
	return math.Min(100, loyalty)
}

// determinePhase 阶段判定
func determinePhase(curve *LoyaltyCurve) string {
	if !curve.LastSeen.IsZero() && time.Since(curve.LastSeen) > 4*time.Hour {
		return PhaseAbandon
	}
	return determinePhaseNoAbandon(curve)
}

// determinePhaseNoAbandon 阶段判定（不考虑 abandon，用于历史推断）
func determinePhaseNoAbandon(curve *LoyaltyCurve) string {
	if curve.TotalInteractions > 15 || curve.MaxDepth >= 3 {
		return PhaseDeepDive
	}
	if curve.TotalInteractions >= 6 && curve.MaxDepth >= 2 {
		return PhaseEngage
	}
	if curve.TotalInteractions >= 3 && curve.Frequency > 0.5 {
		return PhaseInterest
	}
	return PhaseProbe
}

// buildPhaseHistory 从交互记录推断阶段变迁历史
func (hd *HoneypotDeepEngine) buildPhaseHistory(attackerID string) []PhaseTransition {
	rows, err := hd.db.Query(`SELECT timestamp, depth FROM honeypot_interactions WHERE attacker_id = ? ORDER BY timestamp ASC`, attackerID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var transitions []PhaseTransition
	currentPhase := ""
	count := 0
	var maxDepth int
	var firstSeen, lastSeen time.Time
	var frequency float64

	for rows.Next() {
		var ts string
		var depth int
		if rows.Scan(&ts, &depth) != nil {
			continue
		}
		t := parseTimestamp(ts)

		count++
		if depth > maxDepth {
			maxDepth = depth
		}
		if firstSeen.IsZero() {
			firstSeen = t
		}
		lastSeen = t

		durationHours := lastSeen.Sub(firstSeen).Hours()
		if durationHours > 0 {
			frequency = float64(count) / durationHours
		} else if count > 0 {
			frequency = float64(count)
		}

		tmpCurve := &LoyaltyCurve{
			TotalInteractions: count,
			MaxDepth:          maxDepth,
			Frequency:         frequency,
			LastSeen:          t,
		}
		phase := determinePhaseNoAbandon(tmpCurve)

		if phase != currentPhase {
			if currentPhase != "" {
				transitions = append(transitions, PhaseTransition{
					From:      currentPhase,
					To:        phase,
					Timestamp: t,
					Trigger:   fmt.Sprintf("interactions=%d depth=%d freq=%.2f", count, maxDepth, frequency),
				})
			}
			currentPhase = phase
		}
	}

	return transitions
}

// parseTimestamp 解析时间字符串
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, ts)
	}
	return t
}

// ============================================================
// ListLoyaltyCurves — 排行列表
// ============================================================

// ListLoyaltyCurves 返回所有攻击者的忠诚度曲线排行
func (hd *HoneypotDeepEngine) ListLoyaltyCurves(limit int) []LoyaltyCurve {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	return hd.listCurvesInternal(limit)
}

func (hd *HoneypotDeepEngine) listCurvesInternal(limit int) []LoyaltyCurve {
	if limit <= 0 {
		limit = 50
	}

	rows, err := hd.db.Query(`SELECT DISTINCT attacker_id FROM honeypot_interactions`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var attackerIDs []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			attackerIDs = append(attackerIDs, id)
		}
	}

	var curves []LoyaltyCurve
	for _, id := range attackerIDs {
		curve := hd.getLoyaltyCurveInternal(id)
		if curve != nil {
			curves = append(curves, *curve)
		}
	}

	// 按忠诚度评分降序排序
	for i := 0; i < len(curves); i++ {
		for j := i + 1; j < len(curves); j++ {
			if curves[j].LoyaltyScore > curves[i].LoyaltyScore {
				curves[i], curves[j] = curves[j], curves[i]
			}
		}
	}

	if len(curves) > limit {
		curves = curves[:limit]
	}

	return curves
}

// ============================================================
// Feedback — 回馈自进化引擎
// ============================================================

// FeedbackToEvolution 将攻击者的载荷回馈到自进化引擎
func (hd *HoneypotDeepEngine) FeedbackToEvolution(attackerID string) (int, error) {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	return hd.feedbackInternal(attackerID)
}

// AutoFeedback 自动回馈：遍历所有 loyalty_score > minScore 的攻击者，批量回馈
func (hd *HoneypotDeepEngine) AutoFeedback() (int, error) {
	minScore := hd.cfg.AutoFeedbackMinScore
	if minScore <= 0 {
		minScore = 50
	}

	// 获取所有攻击者
	rows, err := hd.db.Query(`SELECT DISTINCT attacker_id FROM honeypot_interactions`)
	if err != nil {
		return 0, fmt.Errorf("query attackers: %w", err)
	}
	defer rows.Close()

	var attackerIDs []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			attackerIDs = append(attackerIDs, id)
		}
	}

	totalInjected := 0
	for _, id := range attackerIDs {
		hd.mu.RLock()
		curve := hd.getLoyaltyCurveInternal(id)
		hd.mu.RUnlock()

		if curve != nil && curve.LoyaltyScore >= minScore {
			hd.mu.Lock()
			injected, err := hd.feedbackInternal(id)
			hd.mu.Unlock()
			if err == nil {
				totalInjected += injected
			}
		}
	}

	log.Printf("[蜜罐深度] 自动回馈: %d 个向量注入自进化引擎 (min_score=%.0f)", totalInjected, minScore)
	return totalInjected, nil
}

// feedbackInternal 内部回馈方法（调用者已持有 mu 锁）
func (hd *HoneypotDeepEngine) feedbackInternal(attackerID string) (int, error) {
	if hd.evolutionEngine == nil {
		return 0, fmt.Errorf("evolution engine not available")
	}

	rows, err := hd.db.Query(`SELECT payload FROM honeypot_interactions WHERE attacker_id = ? AND payload != ''`, attackerID)
	if err != nil {
		return 0, fmt.Errorf("query payloads: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var uniquePayloads []string
	for rows.Next() {
		var payload string
		if rows.Scan(&payload) == nil && payload != "" {
			if !seen[payload] {
				seen[payload] = true
				uniquePayloads = append(uniquePayloads, payload)
			}
		}
	}

	if len(uniquePayloads) == 0 {
		return 0, nil
	}

	injected := 0
	for _, payload := range uniquePayloads {
		if hd.evolutionEngine.redTeam != nil {
			vectorID := fmt.Sprintf("hp-deep-%s-%s", attackerID[:minInt(8, len(attackerID))], uuid.New().String()[:8])
			hd.evolutionEngine.redTeam.InjectVector(AttackVector{
				ID:             vectorID,
				Category:       "honeypot_feedback",
				Name:           fmt.Sprintf("蜜罐回馈: %s", attackerID),
				Description:    fmt.Sprintf("从攻击者 %s 的蜜罐交互中回馈的攻击载荷", attackerID),
				Payload:        payload,
				Severity:       "high",
				ExpectedAction: "block",
				Engine:         "inbound",
			})
			injected++
		}
	}

	log.Printf("[蜜罐深度] 回馈自进化引擎: attacker=%s injected=%d payloads", attackerID, injected)
	return injected, nil
}

// ============================================================
// ListInteractions — 查询交互记录
// ============================================================

// ListInteractions 查询交互记录列表
func (hd *HoneypotDeepEngine) ListInteractions(attackerID, channel string, limit int) []HoneypotInteraction {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, attacker_id, honeypot_type, channel, payload, depth, session_id, timestamp FROM honeypot_interactions WHERE 1=1`
	var args []interface{}

	if attackerID != "" {
		query += ` AND attacker_id = ?`
		args = append(args, attackerID)
	}
	if channel != "" {
		query += ` AND channel = ?`
		args = append(args, channel)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := hd.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var interactions []HoneypotInteraction
	for rows.Next() {
		var i HoneypotInteraction
		var ts string
		if rows.Scan(&i.ID, &i.AttackerID, &i.HoneypotType, &i.Channel, &i.Payload, &i.Depth, &i.SessionID, &ts) == nil {
			i.Timestamp = parseTimestamp(ts)
			interactions = append(interactions, i)
		}
	}

	if interactions == nil {
		interactions = []HoneypotInteraction{}
	}
	return interactions
}

// ============================================================
// GetStats — 深度蜜罐统计
// ============================================================

// GetStats 返回深度蜜罐统计
func (hd *HoneypotDeepEngine) GetStats() *HoneypotDeepStats {
	hd.mu.RLock()
	defer hd.mu.RUnlock()

	stats := &HoneypotDeepStats{
		PhaseDistribution: make(map[string]int),
		ChannelBreakdown:  make(map[string]int),
		DepthBreakdown:    make(map[int]int),
	}

	hd.db.QueryRow(`SELECT COUNT(*) FROM honeypot_interactions`).Scan(&stats.TotalInteractions)
	hd.db.QueryRow(`SELECT COUNT(DISTINCT attacker_id) FROM honeypot_interactions`).Scan(&stats.TotalAttackers)
	hd.db.QueryRow(`SELECT COUNT(DISTINCT session_id) FROM honeypot_interactions`).Scan(&stats.TotalSessions)

	// Channel breakdown
	chRows, err := hd.db.Query(`SELECT channel, COUNT(*) FROM honeypot_interactions GROUP BY channel`)
	if err == nil {
		defer chRows.Close()
		for chRows.Next() {
			var ch string
			var cnt int
			if chRows.Scan(&ch, &cnt) == nil {
				stats.ChannelBreakdown[ch] = cnt
			}
		}
	}

	// Depth breakdown
	depthRows, err := hd.db.Query(`SELECT depth, COUNT(*) FROM honeypot_interactions GROUP BY depth`)
	if err == nil {
		defer depthRows.Close()
		for depthRows.Next() {
			var d, cnt int
			if depthRows.Scan(&d, &cnt) == nil {
				stats.DepthBreakdown[d] = cnt
			}
		}
	}

	// Phase distribution + avg loyalty
	curves := hd.listCurvesInternal(50)
	totalScore := 0.0
	for _, c := range curves {
		stats.PhaseDistribution[c.Phase]++
		totalScore += c.LoyaltyScore
	}
	if len(curves) > 0 {
		stats.AvgLoyaltyScore = totalScore / float64(len(curves))
	}

	if len(curves) > 5 {
		stats.TopAttackers = curves[:5]
	} else {
		stats.TopAttackers = curves
	}

	return stats
}

// ============================================================
// 辅助函数
// ============================================================

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
