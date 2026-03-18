// pipeline.go — 检测链 DetectPipeline + DetectStage 接口（v5.1 智能检测）
// 支持多个检测阶段串行，遇到 block 立即终止
package main

import (
	"log"
	"strings"
	"time"
)

// ============================================================
// 检测上下文与结果
// ============================================================

// DetectContext 检测上下文（贯穿整个 Pipeline）
type DetectContext struct {
	Text          string            // 原文
	SenderID      string            // 发送者 ID
	AppID         string            // 应用 ID
	TraceID       string            // 追踪 ID
	PreviousResults []*StageResult  // 前置阶段结果
}

// StageResult 单阶段检测结果
type StageResult struct {
	StageName string `json:"stage_name"`
	Action    string `json:"action"`     // pass / warn / block
	RuleName  string `json:"rule_name"`
	Detail    string `json:"detail"`
	Duration  time.Duration `json:"-"`
}

// ============================================================
// DetectStage 接口
// ============================================================

// DetectStage 检测阶段接口
type DetectStage interface {
	Name() string
	Detect(ctx *DetectContext) *StageResult
}

// ============================================================
// DetectPipeline 检测链
// ============================================================

// DetectPipeline 多阶段检测链
type DetectPipeline struct {
	stages []DetectStage
}

// NewDetectPipeline 创建检测链
func NewDetectPipeline(stages []DetectStage) *DetectPipeline {
	return &DetectPipeline{stages: stages}
}

// PipelineResult 完整检测结果
type PipelineResult struct {
	FinalAction  string         `json:"final_action"`   // 最终动作: pass/warn/block
	FinalRule    string         `json:"final_rule"`      // 最终匹配规则名
	FinalMessage string         `json:"final_message"`   // 最终拦截消息
	StageResults []*StageResult `json:"stage_results"`   // 各阶段结果
	MatchedRules []string       `json:"matched_rules"`   // 所有匹配的规则
	PIIs         []string       `json:"piis,omitempty"`  // 检测到的 PII
	TotalDuration time.Duration `json:"-"`
}

// Execute 执行检测链
// 遇到 block 立即终止并返回
func (p *DetectPipeline) Execute(ctx *DetectContext) *PipelineResult {
	start := time.Now()
	result := &PipelineResult{
		FinalAction:  "pass",
		StageResults: make([]*StageResult, 0, len(p.stages)),
	}

	for _, stage := range p.stages {
		stageStart := time.Now()
		stageResult := stage.Detect(ctx)
		if stageResult == nil {
			stageResult = &StageResult{StageName: stage.Name(), Action: "pass"}
		}
		stageResult.StageName = stage.Name()
		stageResult.Duration = time.Since(stageStart)

		result.StageResults = append(result.StageResults, stageResult)
		ctx.PreviousResults = append(ctx.PreviousResults, stageResult)

		// 收集匹配规则
		if stageResult.RuleName != "" {
			result.MatchedRules = append(result.MatchedRules, stageResult.RuleName)
		}

		// 决策升级：只向上升级，不向下降级
		if actionWeight(stageResult.Action) > actionWeight(result.FinalAction) {
			result.FinalAction = stageResult.Action
			result.FinalRule = stageResult.RuleName
			result.FinalMessage = stageResult.Detail
		}

		// block 立即终止
		if stageResult.Action == "block" {
			break
		}
	}

	result.TotalDuration = time.Since(start)
	return result
}

// ============================================================
// 内置检测阶段实现
// ============================================================

// KeywordStage AC 自动机关键词检测阶段
type KeywordStage struct {
	engine *RuleEngine
}

func NewKeywordStage(engine *RuleEngine) *KeywordStage {
	return &KeywordStage{engine: engine}
}

func (s *KeywordStage) Name() string { return "keyword" }

func (s *KeywordStage) Detect(ctx *DetectContext) *StageResult {
	s.engine.mu.RLock()
	ac := s.engine.ac
	rules := s.engine.rules
	applicableGroups := s.engine.GetApplicableGroups(ctx.AppID)
	s.engine.mu.RUnlock()

	result := &StageResult{Action: "pass"}

	type matchedRule struct {
		Name     string
		Priority int
		Action   string
		Message  string
	}
	var bestMatch *matchedRule

	for _, idx := range ac.Search(ctx.Text) {
		if idx < 0 || idx >= len(rules) {
			continue
		}
		rule := rules[idx]
		if !isRuleApplicable(rule.Group, applicableGroups) {
			continue
		}
		action := levelToAction(rule.Level)
		if bestMatch == nil || rule.Priority > bestMatch.Priority ||
			(rule.Priority == bestMatch.Priority && actionWeight(action) > actionWeight(bestMatch.Action)) {
			bestMatch = &matchedRule{Name: rule.Name, Priority: rule.Priority, Action: action, Message: rule.Message}
		}
	}

	if bestMatch != nil {
		result.Action = bestMatch.Action
		result.RuleName = bestMatch.Name
		result.Detail = bestMatch.Message
	}
	return result
}

// RegexStage 正则检测阶段
type RegexStage struct {
	engine *RuleEngine
}

func NewRegexStage(engine *RuleEngine) *RegexStage {
	return &RegexStage{engine: engine}
}

func (s *RegexStage) Name() string { return "regex" }

func (s *RegexStage) Detect(ctx *DetectContext) *StageResult {
	s.engine.mu.RLock()
	regexRules := s.engine.regexRules
	applicableGroups := s.engine.GetApplicableGroups(ctx.AppID)
	s.engine.mu.RUnlock()

	result := &StageResult{Action: "pass"}

	type matchedRule struct {
		Name     string
		Priority int
		Action   string
		Message  string
	}
	var bestMatch *matchedRule

	for _, rr := range regexRules {
		if !isRuleApplicable(rr.Group, applicableGroups) {
			continue
		}
		done := make(chan bool, 1)
		go func() {
			defer func() { if rv := recover(); rv != nil { done <- false } }()
			done <- rr.Pattern.MatchString(ctx.Text)
		}()
		var matched bool
		select {
		case matched = <-done:
		case <-time.After(100 * time.Millisecond):
			log.Printf("[Pipeline:regex] 正则匹配超时 rule=%s", rr.Name)
			continue
		}
		if !matched {
			continue
		}
		action := levelToAction(rr.Level)
		if bestMatch == nil || rr.Priority > bestMatch.Priority ||
			(rr.Priority == bestMatch.Priority && actionWeight(action) > actionWeight(bestMatch.Action)) {
			bestMatch = &matchedRule{Name: rr.Name, Priority: rr.Priority, Action: action, Message: rr.Message}
		}
	}

	if bestMatch != nil {
		result.Action = bestMatch.Action
		result.RuleName = bestMatch.Name
		result.Detail = bestMatch.Message
	}
	return result
}

// PIIStage PII 检测阶段
type PIIStage struct {
	engine *RuleEngine
}

func NewPIIStage(engine *RuleEngine) *PIIStage {
	return &PIIStage{engine: engine}
}

func (s *PIIStage) Name() string { return "pii" }

func (s *PIIStage) Detect(ctx *DetectContext) *StageResult {
	piis := s.engine.DetectPII(ctx.Text)
	if len(piis) > 0 {
		return &StageResult{
			Action:   "warn",
			RuleName: "pii_detected",
			Detail:   "pii:" + strings.Join(piis, "+"),
		}
	}
	return &StageResult{Action: "pass"}
}

// ============================================================
// Pipeline 构建辅助
// ============================================================

// BuildDefaultPipeline 构建默认检测链（keyword → regex → pii）
func BuildDefaultPipeline(engine *RuleEngine) *DetectPipeline {
	stages := []DetectStage{
		NewKeywordStage(engine),
		NewRegexStage(engine),
		NewPIIStage(engine),
	}
	return NewDetectPipeline(stages)
}

// BuildPipelineFromConfig 根据配置构建检测链
// 支持的阶段名: keyword, regex, pii
// additionalStages 可注入额外的自定义阶段（如 session, llm）
func BuildPipelineFromConfig(stageNames []string, engine *RuleEngine, additionalStages map[string]DetectStage) *DetectPipeline {
	stageMap := map[string]DetectStage{
		"keyword": NewKeywordStage(engine),
		"regex":   NewRegexStage(engine),
		"pii":     NewPIIStage(engine),
	}
	// 注入额外阶段
	for name, stage := range additionalStages {
		stageMap[name] = stage
	}

	var stages []DetectStage
	for _, name := range stageNames {
		if stage, ok := stageMap[name]; ok {
			stages = append(stages, stage)
		} else {
			log.Printf("[Pipeline] 未知的检测阶段: %s，跳过", name)
		}
	}

	if len(stages) == 0 {
		// 回退到默认
		return BuildDefaultPipeline(engine)
	}
	return NewDetectPipeline(stages)
}
