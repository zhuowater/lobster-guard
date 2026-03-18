// adaptive_decision.go — 自适应决策引擎：基于贝叶斯统计的误伤率优化（v18.3）
// 通过历史行为画像数据预测管理员对特定用户/场景的决策偏好，自动调整 block→warn 阈值
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// ============================================================
// 自适应决策引擎
// ============================================================

// AdaptiveDecisionEngine 自适应决策引擎
type AdaptiveDecisionEngine struct {
	db          *sql.DB
	envelopeMgr *EnvelopeManager // 用于生成带数学证明的决策信封
	mu          sync.RWMutex
	// 用户决策历史缓存：userID → DecisionHistory
	histories map[string]*DecisionHistory
	cfg       AdaptiveDecisionConfig
}

// AdaptiveDecisionConfig 自适应决策配置
type AdaptiveDecisionConfig struct {
	Enabled         bool    `yaml:"enabled" json:"enabled"`
	LookbackDays    int     `yaml:"lookback_days" json:"lookback_days"`       // 历史数据回溯天数（默认30）
	MinSamples      int     `yaml:"min_samples" json:"min_samples"`           // 最少样本数才启用（默认10）
	FPThreshold     float64 `yaml:"fp_threshold" json:"fp_threshold"`         // 误伤率阈值（超过则降级block→warn，默认0.5）
	ConfidenceLevel float64 `yaml:"confidence_level" json:"confidence_level"` // 贝叶斯置信水平（默认0.95）
}

// DecisionHistory 用户决策历史
type DecisionHistory struct {
	UserID      string    `json:"user_id"`
	TotalBlocks int       `json:"total_blocks"`  // 历史 block 总数
	FalseBlocks int       `json:"false_blocks"`  // 其中误伤次数（管理员手动改为 pass 或 warn 的）
	TotalWarns  int       `json:"total_warns"`   // 历史 warn 总数
	LastUpdated time.Time `json:"last_updated"`
}

// BayesianProof 贝叶斯证明
type BayesianProof struct {
	PriorAlpha     float64 `json:"prior_alpha"`       // Beta 先验 α
	PriorBeta      float64 `json:"prior_beta"`        // Beta 先验 β
	ObservedFP     int     `json:"observed_fp"`        // 观测到的误伤次数
	ObservedTotal  int     `json:"observed_total"`     // 观测总次数
	PosteriorMean  float64 `json:"posterior_mean"`     // 后验均值 = (α+FP)/(α+β+Total)
	PosteriorLower float64 `json:"posterior_ci_lower"` // 置信区间下界
	PosteriorUpper float64 `json:"posterior_ci_upper"` // 置信区间上界
	Decision       string  `json:"decision"`           // "downgrade_to_warn" / "keep_block"
	Reason         string  `json:"reason"`
}

// AdaptiveStats 自适应决策统计
type AdaptiveStats struct {
	TotalDowngrades int     `json:"total_downgrades"` // 总降级数
	TotalUsers      int     `json:"total_users"`      // 有记录的用户数
	AvgFPRate       float64 `json:"avg_fp_rate"`      // 平均误伤率
	Config          AdaptiveDecisionConfig `json:"config"`
}

// NewAdaptiveDecisionEngine 创建自适应决策引擎
func NewAdaptiveDecisionEngine(db *sql.DB, envelopeMgr *EnvelopeManager, cfg AdaptiveDecisionConfig) *AdaptiveDecisionEngine {
	// 设置默认值
	if cfg.LookbackDays <= 0 {
		cfg.LookbackDays = 30
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = 10
	}
	if cfg.FPThreshold <= 0 {
		cfg.FPThreshold = 0.5
	}
	if cfg.ConfidenceLevel <= 0 {
		cfg.ConfidenceLevel = 0.95
	}

	ade := &AdaptiveDecisionEngine{
		db:          db,
		envelopeMgr: envelopeMgr,
		histories:   make(map[string]*DecisionHistory),
		cfg:         cfg,
	}
	ade.initSchema()
	ade.LoadHistories()
	return ade
}

// initSchema 初始化数据库表
func (ade *AdaptiveDecisionEngine) initSchema() {
	ade.db.Exec(`CREATE TABLE IF NOT EXISTS decision_outcomes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		trace_id TEXT DEFAULT '',
		original_action TEXT NOT NULL,
		final_action TEXT NOT NULL,
		was_false_positive INTEGER DEFAULT 0,
		proof_json TEXT DEFAULT '{}',
		timestamp TEXT NOT NULL
	)`)
	ade.db.Exec(`CREATE INDEX IF NOT EXISTS idx_decision_user ON decision_outcomes(user_id)`)
}

// ShouldDowngrade 判断是否应该降级 block→warn
// 返回新的 action 和贝叶斯证明
func (ade *AdaptiveDecisionEngine) ShouldDowngrade(userID string, originalAction string) (string, *BayesianProof) {
	// 只对 block 动作进行降级评估
	if originalAction != "block" {
		return originalAction, nil
	}

	ade.mu.RLock()
	history, exists := ade.histories[userID]
	cfg := ade.cfg
	ade.mu.RUnlock()

	// 没有历史数据或样本不足
	if !exists || history.TotalBlocks < cfg.MinSamples {
		return originalAction, &BayesianProof{
			PriorAlpha:    1.0,
			PriorBeta:     1.0,
			ObservedFP:    0,
			ObservedTotal: 0,
			PosteriorMean: 0.5,
			Decision:      "keep_block",
			Reason:        fmt.Sprintf("样本不足 (%d < %d)，保持原决策", func() int { if exists { return history.TotalBlocks }; return 0 }(), cfg.MinSamples),
		}
	}

	// 计算贝叶斯后验
	proof := ade.computeBayesianProof(history, cfg)

	if proof.Decision == "downgrade_to_warn" {
		// 记录降级决策
		proofJSON, _ := json.Marshal(proof)
		ade.db.Exec(`INSERT INTO decision_outcomes (user_id, trace_id, original_action, final_action, was_false_positive, proof_json, timestamp) VALUES (?,?,?,?,?,?,?)`,
			userID, "", originalAction, "warn", 0, string(proofJSON), time.Now().UTC().Format(time.RFC3339))

		// 生成执行信封
		if ade.envelopeMgr != nil {
			ade.envelopeMgr.Seal(
				"", // traceID 由调用方补充
				"adaptive_decision",
				fmt.Sprintf("downgrade user=%s P(FP)=%.3f", userID, proof.PosteriorMean),
				"warn",
				[]string{"adaptive_bayesian_downgrade"},
				userID,
			)
		}

		return "warn", proof
	}

	return originalAction, proof
}

// computeBayesianProof 计算贝叶斯后验证明
func (ade *AdaptiveDecisionEngine) computeBayesianProof(history *DecisionHistory, cfg AdaptiveDecisionConfig) *BayesianProof {
	// Beta 先验: α=1, β=1（均匀先验）
	priorAlpha := 1.0
	priorBeta := 1.0

	// 后验参数: α' = α + FP, β' = β + (Total - FP)
	posteriorAlpha := priorAlpha + float64(history.FalseBlocks)
	posteriorBeta := priorBeta + float64(history.TotalBlocks-history.FalseBlocks)

	// 后验均值: P(FP) = α' / (α' + β')
	posteriorMean := posteriorAlpha / (posteriorAlpha + posteriorBeta)

	// 置信区间（Beta 分布正态近似）
	lower, upper := betaConfidenceInterval(posteriorAlpha, posteriorBeta, cfg.ConfidenceLevel)

	decision := "keep_block"
	reason := fmt.Sprintf("P(FP)=%.3f < %.3f, 保持 block", posteriorMean, cfg.FPThreshold)

	if posteriorMean > cfg.FPThreshold {
		decision = "downgrade_to_warn"
		reason = fmt.Sprintf("P(FP)=%.3f > %.3f, 降级为 warn (CI=[%.3f, %.3f])", posteriorMean, cfg.FPThreshold, lower, upper)
	}

	return &BayesianProof{
		PriorAlpha:     priorAlpha,
		PriorBeta:      priorBeta,
		ObservedFP:     history.FalseBlocks,
		ObservedTotal:  history.TotalBlocks,
		PosteriorMean:  posteriorMean,
		PosteriorLower: lower,
		PosteriorUpper: upper,
		Decision:       decision,
		Reason:         reason,
	}
}

// betaConfidenceInterval Beta 分布置信区间（正态近似）
// 对于 Beta(α, β)：均值 = α/(α+β)，方差 = αβ/((α+β)²(α+β+1))
func betaConfidenceInterval(alpha, beta, confidence float64) (float64, float64) {
	mean := alpha / (alpha + beta)
	variance := (alpha * beta) / ((alpha + beta) * (alpha + beta) * (alpha + beta + 1))
	stddev := math.Sqrt(variance)

	// z-score 映射（常见置信水平）
	z := 1.96 // 默认 95%
	switch {
	case confidence >= 0.999:
		z = 3.291
	case confidence >= 0.99:
		z = 2.576
	case confidence >= 0.95:
		z = 1.96
	case confidence >= 0.90:
		z = 1.645
	case confidence >= 0.80:
		z = 1.282
	}

	lower := mean - z*stddev
	upper := mean + z*stddev

	// 截断到 [0, 1]
	if lower < 0 {
		lower = 0
	}
	if upper > 1 {
		upper = 1
	}

	return lower, upper
}

// RecordOutcome 记录决策结果（人工反馈）
func (ade *AdaptiveDecisionEngine) RecordOutcome(userID string, action string, wasFalsePositive bool) error {
	fp := 0
	if wasFalsePositive {
		fp = 1
	}

	_, err := ade.db.Exec(`INSERT INTO decision_outcomes (user_id, trace_id, original_action, final_action, was_false_positive, proof_json, timestamp) VALUES (?,?,?,?,?,?,?)`,
		userID, "", action, action, fp, "{}", time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("记录决策结果失败: %w", err)
	}

	// 更新内存缓存
	ade.mu.Lock()
	defer ade.mu.Unlock()

	h, exists := ade.histories[userID]
	if !exists {
		h = &DecisionHistory{UserID: userID}
		ade.histories[userID] = h
	}

	if action == "block" {
		h.TotalBlocks++
		if wasFalsePositive {
			h.FalseBlocks++
		}
	} else if action == "warn" {
		h.TotalWarns++
	}
	h.LastUpdated = time.Now()

	return nil
}

// LoadHistories 从 decision_outcomes 表加载历史数据
func (ade *AdaptiveDecisionEngine) LoadHistories() {
	ade.mu.Lock()
	defer ade.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -ade.cfg.LookbackDays).UTC().Format(time.RFC3339)

	rows, err := ade.db.Query(`
		SELECT user_id,
			COUNT(*) as total,
			SUM(CASE WHEN was_false_positive = 1 THEN 1 ELSE 0 END) as fp_count,
			SUM(CASE WHEN original_action = 'warn' THEN 1 ELSE 0 END) as warn_count,
			MAX(timestamp) as last_ts
		FROM decision_outcomes
		WHERE timestamp >= ? AND original_action IN ('block', 'warn')
		GROUP BY user_id`, cutoff)
	if err != nil {
		return
	}
	defer rows.Close()

	newHistories := make(map[string]*DecisionHistory)
	for rows.Next() {
		var userID, lastTs string
		var total, fpCount, warnCount int
		if err := rows.Scan(&userID, &total, &fpCount, &warnCount, &lastTs); err != nil {
			continue
		}
		lastUpdated, _ := time.Parse(time.RFC3339, lastTs)
		newHistories[userID] = &DecisionHistory{
			UserID:      userID,
			TotalBlocks: total - warnCount,
			FalseBlocks: fpCount,
			TotalWarns:  warnCount,
			LastUpdated: lastUpdated,
		}
	}

	ade.histories = newHistories
}

// GetProof 获取指定用户的当前贝叶斯证明
func (ade *AdaptiveDecisionEngine) GetProof(userID string) *BayesianProof {
	ade.mu.RLock()
	history, exists := ade.histories[userID]
	cfg := ade.cfg
	ade.mu.RUnlock()

	if !exists {
		return &BayesianProof{
			PriorAlpha:    1.0,
			PriorBeta:     1.0,
			ObservedFP:    0,
			ObservedTotal: 0,
			PosteriorMean: 0.5,
			Decision:      "no_data",
			Reason:        "用户无历史数据",
		}
	}

	return ade.computeBayesianProof(history, cfg)
}

// GetStats 获取统计数据
func (ade *AdaptiveDecisionEngine) GetStats() *AdaptiveStats {
	ade.mu.RLock()
	defer ade.mu.RUnlock()

	stats := &AdaptiveStats{
		Config: ade.cfg,
	}

	// 统计降级数
	var downgrades int
	ade.db.QueryRow(`SELECT COUNT(*) FROM decision_outcomes WHERE original_action = 'block' AND final_action = 'warn'`).Scan(&downgrades)
	stats.TotalDowngrades = downgrades

	// 统计用户数
	stats.TotalUsers = len(ade.histories)

	// 计算平均误伤率
	if len(ade.histories) > 0 {
		totalFP := 0.0
		count := 0
		for _, h := range ade.histories {
			if h.TotalBlocks > 0 {
				totalFP += float64(h.FalseBlocks) / float64(h.TotalBlocks)
				count++
			}
		}
		if count > 0 {
			stats.AvgFPRate = totalFP / float64(count)
		}
	}

	return stats
}

// GetConfig 获取配置
func (ade *AdaptiveDecisionEngine) GetConfig() AdaptiveDecisionConfig {
	ade.mu.RLock()
	defer ade.mu.RUnlock()
	return ade.cfg
}

// UpdateConfig 更新配置
func (ade *AdaptiveDecisionEngine) UpdateConfig(newCfg AdaptiveDecisionConfig) {
	ade.mu.Lock()
	defer ade.mu.Unlock()

	if newCfg.LookbackDays > 0 {
		ade.cfg.LookbackDays = newCfg.LookbackDays
	}
	if newCfg.MinSamples > 0 {
		ade.cfg.MinSamples = newCfg.MinSamples
	}
	if newCfg.FPThreshold > 0 {
		ade.cfg.FPThreshold = newCfg.FPThreshold
	}
	if newCfg.ConfidenceLevel > 0 {
		ade.cfg.ConfidenceLevel = newCfg.ConfidenceLevel
	}
}

// GetHistory 获取指定用户的决策历史
func (ade *AdaptiveDecisionEngine) GetHistory(userID string) *DecisionHistory {
	ade.mu.RLock()
	defer ade.mu.RUnlock()
	h, ok := ade.histories[userID]
	if !ok {
		return nil
	}
	// 返回副本
	copy := *h
	return &copy
}

// ListHistories 列出所有用户历史
func (ade *AdaptiveDecisionEngine) ListHistories() []*DecisionHistory {
	ade.mu.RLock()
	defer ade.mu.RUnlock()
	result := make([]*DecisionHistory, 0, len(ade.histories))
	for _, h := range ade.histories {
		copy := *h
		result = append(result, &copy)
	}
	return result
}
