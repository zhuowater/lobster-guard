// session_detect.go — 上下文感知检测 SessionDetector（v5.1 智能检测）
// 跟踪每个 sender_id 的对话历史，累计风险积分，支持时间衰减
package main

import (
	"sync"
	"time"
)

// ============================================================
// 会话消息摘要
// ============================================================

// SessionMessage 会话消息摘要（只保留前 100 字符 + 检测结果，不存储全文）
type SessionMessage struct {
	Preview   string    `json:"preview"`    // 前 100 字符
	Action    string    `json:"action"`     // pass/warn/block
	RuleName  string    `json:"rule_name"`  // 匹配的规则
	Timestamp time.Time `json:"timestamp"`
}

// SessionInfo 会话风险信息
type SessionInfo struct {
	SenderID    string            `json:"sender_id"`
	RiskScore   float64           `json:"risk_score"`      // 当前风险积分
	RawScore    float64           `json:"-"`               // 累计原始积分（衰减前）
	Messages    []SessionMessage  `json:"recent_messages"` // 最近 N 条消息摘要
	LastUpdate  time.Time         `json:"last_update"`
	TotalWarn   int64             `json:"total_warn"`
	TotalBlock  int64             `json:"total_block"`
}

// ============================================================
// SessionDetector
// ============================================================

// SessionDetectorConfig 会话检测配置
type SessionDetectorConfig struct {
	Enabled       bool    // 是否启用
	RiskThreshold float64 // 风险积分阈值（超过则 warn → block）
	Window        int     // 保留最近 N 条消息上下文
	DecayRate     float64 // 每小时衰减积分
}

// SessionDetector 上下文感知检测器
type SessionDetector struct {
	mu       sync.RWMutex
	sessions map[string]*SessionInfo
	cfg      SessionDetectorConfig
}

// NewSessionDetector 创建会话检测器
func NewSessionDetector(cfg SessionDetectorConfig) *SessionDetector {
	if cfg.Window <= 0 {
		cfg.Window = 20
	}
	if cfg.RiskThreshold <= 0 {
		cfg.RiskThreshold = 10
	}
	if cfg.DecayRate <= 0 {
		cfg.DecayRate = 1
	}
	return &SessionDetector{
		sessions: make(map[string]*SessionInfo),
		cfg:      cfg,
	}
}

// RecordAndEvaluate 记录消息检测结果，并返回是否需要升级 action
// 返回升级后的 action（如果需要升级）和当前风险积分
func (sd *SessionDetector) RecordAndEvaluate(senderID, text, action, ruleName string) (upgradedAction string, riskScore float64) {
	if senderID == "" {
		return action, 0
	}

	preview := text
	if rs := []rune(preview); len(rs) > 100 {
		preview = string(rs[:100])
	}

	sd.mu.Lock()
	defer sd.mu.Unlock()

	info, ok := sd.sessions[senderID]
	if !ok {
		info = &SessionInfo{
			SenderID:   senderID,
			Messages:   make([]SessionMessage, 0, sd.cfg.Window),
			LastUpdate: time.Now(),
		}
		sd.sessions[senderID] = info
	}

	// 时间衰减
	elapsed := time.Since(info.LastUpdate).Hours()
	if elapsed > 0 {
		decay := elapsed * sd.cfg.DecayRate
		info.RawScore -= decay
		if info.RawScore < 0 {
			info.RawScore = 0
		}
	}
	info.LastUpdate = time.Now()

	// 累加积分
	switch action {
	case "warn":
		info.RawScore += 1
		info.TotalWarn++
	case "block":
		info.RawScore += 5
		info.TotalBlock++
	}
	info.RiskScore = info.RawScore

	// 添加消息摘要
	msg := SessionMessage{
		Preview:   preview,
		Action:    action,
		RuleName:  ruleName,
		Timestamp: time.Now(),
	}
	info.Messages = append(info.Messages, msg)
	if len(info.Messages) > sd.cfg.Window {
		info.Messages = info.Messages[len(info.Messages)-sd.cfg.Window:]
	}

	// 判断是否需要升级 action
	upgradedAction = action
	if info.RiskScore >= sd.cfg.RiskThreshold && action == "warn" {
		upgradedAction = "block"
	}

	return upgradedAction, info.RiskScore
}

// GetRiskScore 获取指定 sender 的风险积分（带衰减计算）
func (sd *SessionDetector) GetRiskScore(senderID string) float64 {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	info, ok := sd.sessions[senderID]
	if !ok {
		return 0
	}
	// 计算衰减后的积分
	elapsed := time.Since(info.LastUpdate).Hours()
	score := info.RawScore - elapsed*sd.cfg.DecayRate
	if score < 0 {
		score = 0
	}
	return score
}

// ListHighRiskSessions 列出所有高风险会话（risk_score >= threshold）
func (sd *SessionDetector) ListHighRiskSessions() []SessionInfo {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	var result []SessionInfo
	for _, info := range sd.sessions {
		// 计算衰减后的积分
		elapsed := time.Since(info.LastUpdate).Hours()
		score := info.RawScore - elapsed*sd.cfg.DecayRate
		if score < 0 {
			score = 0
		}
		if score > 0 {
			snapshot := *info
			snapshot.RiskScore = score
			result = append(result, snapshot)
		}
	}
	return result
}

// ResetRisk 重置指定用户的风险积分
func (sd *SessionDetector) ResetRisk(senderID string) bool {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	info, ok := sd.sessions[senderID]
	if !ok {
		return false
	}
	info.RawScore = 0
	info.RiskScore = 0
	info.TotalWarn = 0
	info.TotalBlock = 0
	info.Messages = info.Messages[:0]
	return true
}

// SessionCount 返回跟踪的会话数
func (sd *SessionDetector) SessionCount() int {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return len(sd.sessions)
}

// Cleanup 清理过期会话（超过 maxAge 没有活动的会话）
func (sd *SessionDetector) Cleanup(maxAge time.Duration) int {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	deleted := 0
	for id, info := range sd.sessions {
		if info.LastUpdate.Before(cutoff) {
			delete(sd.sessions, id)
			deleted++
		}
	}
	return deleted
}

// ============================================================
// SessionStage — Pipeline 阶段实现
// ============================================================

// SessionStage 会话检测阶段（集成到 Pipeline）
type SessionStage struct {
	detector *SessionDetector
}

func NewSessionStage(detector *SessionDetector) *SessionStage {
	return &SessionStage{detector: detector}
}

func (s *SessionStage) Name() string { return "session" }

func (s *SessionStage) Detect(ctx *DetectContext) *StageResult {
	if s.detector == nil || !s.detector.cfg.Enabled {
		return &StageResult{Action: "pass"}
	}

	// 获取前置阶段的最高 action
	prevAction := "pass"
	prevRule := ""
	for _, prev := range ctx.PreviousResults {
		if actionWeight(prev.Action) > actionWeight(prevAction) {
			prevAction = prev.Action
			prevRule = prev.RuleName
		}
	}

	// 记录并评估
	upgradedAction, riskScore := s.detector.RecordAndEvaluate(
		ctx.SenderID, ctx.Text, prevAction, prevRule,
	)

	// 如果 action 被升级了，返回升级结果
	if upgradedAction != prevAction {
		return &StageResult{
			Action:   upgradedAction,
			RuleName: "session_risk_upgrade",
			Detail:   "风险积分超过阈值，动作已升级",
		}
	}

	// 如果风险分数很高但未升级（例如本次是 pass），仍标记
	if riskScore >= s.detector.cfg.RiskThreshold && prevAction == "pass" {
		return &StageResult{
			Action:   "pass",
			RuleName: "",
			Detail:   "会话风险积分较高，持续监控中",
		}
	}

	return &StageResult{Action: "pass"}
}
