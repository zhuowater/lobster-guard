// evolution.go — 对抗性自进化引擎：红队变异+自动规则生成+热更新
// lobster-guard v19.0
package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ============================================================
// 类型定义
// ============================================================

// EvolutionEngine 对抗性自进化引擎
type EvolutionEngine struct {
	db          *sql.DB
	redTeam     *RedTeamEngine
	ruleEngine  *RuleEngine
	outboundEng *OutboundRuleEngine
	llmRuleEng  *LLMRuleEngine
	eventBus    *EventBus
	mu          sync.Mutex
	generation  int
	autoTicker  *time.Ticker
	stopCh      chan struct{}
}

// MutationStrategy 变异策略
type MutationStrategy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// EvolutionRecord 进化日志记录
type EvolutionRecord struct {
	ID             string    `json:"id"`
	Generation     int       `json:"generation"`
	Timestamp      time.Time `json:"timestamp"`
	Phase          string    `json:"phase"`
	OriginalVector string    `json:"original_vector"`
	MutatedPayload string    `json:"mutated_payload"`
	Strategy       string    `json:"strategy"`
	Bypassed       bool      `json:"bypassed"`
	GeneratedRule  string    `json:"generated_rule"`
	Details        string    `json:"details"`
}

// EvolutionStats 进化统计
type EvolutionStats struct {
	TotalGenerations    int    `json:"total_generations"`
	TotalMutations      int    `json:"total_mutations"`
	TotalBypasses       int    `json:"total_bypasses"`
	TotalRulesGenerated int    `json:"total_rules_generated"`
	LastRunAt           string `json:"last_run_at"`
	CurrentGeneration   int    `json:"current_generation"`
}

// EvolutionReport 一轮进化的报告
type EvolutionReport struct {
	Generation      int                `json:"generation"`
	Timestamp       string             `json:"timestamp"`
	TotalMutations  int                `json:"total_mutations"`
	TotalTested     int                `json:"total_tested"`
	TotalBypasses   int                `json:"total_bypasses"`
	RulesGenerated  int                `json:"rules_generated"`
	DurationMs      int64              `json:"duration_ms"`
	BypassDetails   []EvolutionRecord  `json:"bypass_details"`
	GeneratedRules  []string           `json:"generated_rules"`
	StrategyStats   map[string]int     `json:"strategy_stats"`
}

// MutatedVector 变异后的攻击向量
type MutatedVector struct {
	OriginalID     string
	OriginalPayload string
	MutatedPayload string
	Strategy       string
	Engine         string
	ExpectedAction string
}

// ============================================================
// 变异策略
// ============================================================

var mutationStrategies = []MutationStrategy{
	{Name: "synonym_replace", Description: "同义词替换"},
	{Name: "case_mixed", Description: "大小写混淆"},
	{Name: "unicode_homoglyph", Description: "Unicode 同形字"},
	{Name: "whitespace_inject", Description: "空白字符注入"},
	{Name: "encoding_wrap", Description: "编码包装"},
	{Name: "context_dilute", Description: "上下文稀释"},
}

// ============================================================
// 同义词表（25 组）
// ============================================================

var synonymTable = map[string][]string{
	"ignore":       {"disregard", "dismiss", "overlook", "skip", "neglect", "bypass"},
	"previous":     {"prior", "earlier", "preceding", "former", "above"},
	"instructions": {"directives", "guidelines", "commands", "orders", "rules"},
	"reveal":       {"show", "display", "expose", "disclose", "uncover"},
	"system":       {"core", "internal", "platform", "base", "underlying"},
	"prompt":       {"instruction", "directive", "command", "message", "input"},
	"secret":       {"confidential", "private", "hidden", "classified", "restricted"},
	"password":     {"passcode", "credential", "passphrase", "auth_key", "login_key"},
	"override":     {"overwrite", "supersede", "replace", "supplant", "nullify"},
	"forget":       {"erase", "clear", "wipe", "remove", "discard"},
	"pretend":      {"act as if", "imagine", "simulate", "roleplay", "assume"},
	"restrictions": {"limitations", "constraints", "boundaries", "limits", "safeguards"},
	"output":       {"print", "display", "write", "produce", "emit"},
	"execute":      {"run", "perform", "carry out", "invoke", "trigger"},
	"token":        {"access_key", "auth_token", "bearer", "session_key", "api_token"},
	"忽略":         {"无视", "跳过", "丢弃", "不理会", "放弃"},
	"指令":         {"命令", "规则", "指示", "要求", "请求"},
	"系统":         {"核心", "内部", "平台", "底层", "基础"},
	"提示词":       {"指令集", "规则集", "提示语", "提示信息", "输入指令"},
	"限制":         {"约束", "管控", "边界", "规定", "束缚"},
	"密码":         {"口令", "暗号", "凭证", "密钥", "通行码"},
	"显示":         {"展示", "输出", "打印", "呈现", "列出"},
	"假设":         {"假装", "想象", "模拟", "扮演", "设想"},
	"所有":         {"全部", "一切", "任何", "每个", "总共"},
	"dangerous":    {"harmful", "malicious", "destructive", "hazardous", "threatening"},
}

// ============================================================
// Unicode 同形字表（覆盖 15 个常用拉丁字母）
// ============================================================

var homoglyphTable = map[rune]rune{
	'a': '\u0430', // Cyrillic а
	'c': '\u0441', // Cyrillic с
	'e': '\u0435', // Cyrillic е
	'o': '\u043E', // Cyrillic о
	'p': '\u0440', // Cyrillic р
	's': '\u0455', // Cyrillic ѕ
	'i': '\u0456', // Cyrillic і
	'x': '\u0445', // Cyrillic х
	'y': '\u0443', // Cyrillic у
	'h': '\u04BB', // Cyrillic һ
	'j': '\u0458', // Cyrillic ј
	'q': '\u051B', // Cyrillic қ
	'w': '\u051D', // Cyrillic ԝ
	'd': '\u0501', // Cyrillic ԁ
	'n': '\u0578', // Armenian ո
}

// ============================================================
// 上下文稀释用的正常对话文本
// ============================================================

var contextDiluteTexts = []string{
	"I would like to discuss the quarterly performance report with you. The numbers look quite promising this quarter and we should celebrate the team's achievements. ",
	"Thank you for your help with the project documentation. The team has been working hard and we really appreciate all the effort that has been put into this. ",
	"Could you please help me draft an email to the marketing department? We need to coordinate on the upcoming product launch event scheduled for next month. ",
	"The weather forecast says it will be sunny this weekend. I'm thinking of organizing a team building activity in the park if everyone is available on Saturday. ",
	"I've been reviewing the latest research papers on machine learning optimization techniques. There are some fascinating developments in transformer architectures. ",
}

// ============================================================
// 构造函数
// ============================================================

// NewEvolutionEngine 创建自进化引擎
func NewEvolutionEngine(db *sql.DB, redTeam *RedTeamEngine, ruleEngine *RuleEngine, outboundEng *OutboundRuleEngine, llmRuleEng *LLMRuleEngine, eventBus *EventBus) *EvolutionEngine {
	ee := &EvolutionEngine{
		db:          db,
		redTeam:     redTeam,
		ruleEngine:  ruleEngine,
		outboundEng: outboundEng,
		llmRuleEng:  llmRuleEng,
		eventBus:    eventBus,
		stopCh:      make(chan struct{}),
	}
	ee.initSchema()
	ee.loadGeneration()
	return ee
}

// initSchema 初始化 SQLite 表
func (ee *EvolutionEngine) initSchema() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS evolution_log (
			id TEXT PRIMARY KEY,
			generation INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			phase TEXT NOT NULL,
			original_vector TEXT DEFAULT '',
			mutated_payload TEXT DEFAULT '',
			strategy TEXT DEFAULT '',
			bypassed INTEGER DEFAULT 0,
			generated_rule TEXT DEFAULT '',
			details TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_evolution_gen ON evolution_log(generation)`,
		`CREATE INDEX IF NOT EXISTS idx_evolution_ts ON evolution_log(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_evolution_bypassed ON evolution_log(bypassed)`,
	}
	for _, stmt := range stmts {
		if _, err := ee.db.Exec(stmt); err != nil {
			log.Printf("[进化引擎] 初始化表失败: %v", err)
		}
	}
}

// loadGeneration 从数据库加载当前代数
func (ee *EvolutionEngine) loadGeneration() {
	var maxGen sql.NullInt64
	err := ee.db.QueryRow(`SELECT MAX(generation) FROM evolution_log`).Scan(&maxGen)
	if err == nil && maxGen.Valid {
		ee.generation = int(maxGen.Int64)
	}
}

// ============================================================
// 自动定时运行
// ============================================================

// StartAutoEvolution 启动自动进化定时器
func (ee *EvolutionEngine) StartAutoEvolution(intervalMin int) {
	if intervalMin <= 0 {
		intervalMin = 360
	}
	ee.mu.Lock()
	defer ee.mu.Unlock()
	if ee.autoTicker != nil {
		return // already running
	}
	ee.autoTicker = time.NewTicker(time.Duration(intervalMin) * time.Minute)
	go func() {
		for {
			select {
			case <-ee.autoTicker.C:
				report, err := ee.RunEvolution()
				if err != nil {
					log.Printf("[进化引擎] 自动进化失败: %v", err)
				} else {
					log.Printf("[进化引擎] ✅ 自动进化完成: 第 %d 代, %d 变异, %d 绕过, %d 新规则",
						report.Generation, report.TotalMutations, report.TotalBypasses, report.RulesGenerated)
				}
			case <-ee.stopCh:
				return
			}
		}
	}()
}

// StopAutoEvolution 停止自动进化
func (ee *EvolutionEngine) StopAutoEvolution() {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	if ee.autoTicker != nil {
		ee.autoTicker.Stop()
		ee.autoTicker = nil
	}
	select {
	case <-ee.stopCh:
	default:
		close(ee.stopCh)
	}
}

// ============================================================
// 核心进化循环
// ============================================================

// RunEvolution 执行一轮完整的进化循环
func (ee *EvolutionEngine) RunEvolution() (*EvolutionReport, error) {
	ee.mu.Lock()
	ee.generation++
	gen := ee.generation
	ee.mu.Unlock()

	startTime := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	report := &EvolutionReport{
		Generation:    gen,
		Timestamp:     startTime.UTC().Format(time.RFC3339),
		StrategyStats: make(map[string]int),
	}

	// Phase 1: 变异（Mutate）— 对所有攻击向量应用变异策略
	vectors := ee.redTeam.GetAttackVectors()
	var mutated []MutatedVector
	for _, v := range vectors {
		// 只变异期望被拦截的向量
		if v.ExpectedAction == "pass" {
			continue
		}
		for _, strategy := range mutationStrategies {
			payload := applyMutation(v.Payload, strategy.Name, rng)
			if payload == v.Payload {
				continue // 变异没有产生变化，跳过
			}
			mutated = append(mutated, MutatedVector{
				OriginalID:      v.ID,
				OriginalPayload: v.Payload,
				MutatedPayload:  payload,
				Strategy:        strategy.Name,
				Engine:          v.Engine,
				ExpectedAction:  v.ExpectedAction,
			})
			report.StrategyStats[strategy.Name]++
			// 记录变异日志
			ee.logRecord(EvolutionRecord{
				ID:             uuid.New().String(),
				Generation:     gen,
				Timestamp:      time.Now().UTC(),
				Phase:          "mutate",
				OriginalVector: v.ID,
				MutatedPayload: evoTruncateStr(payload, 500),
				Strategy:       strategy.Name,
				Details:        fmt.Sprintf("engine=%s expected=%s", v.Engine, v.ExpectedAction),
			})
		}
	}
	report.TotalMutations = len(mutated)

	// Phase 2 + 3: 测试（Test）+ 分析（Analyze）— 测试变异体并找出绕过的
	var bypasses []MutatedVector
	for _, mv := range mutated {
		bypassed := false
		switch mv.Engine {
		case "inbound":
			if ee.ruleEngine != nil {
				dr := ee.ruleEngine.Detect(mv.MutatedPayload)
				if mv.ExpectedAction == "block" && dr.Action == "pass" {
					bypassed = true
				} else if mv.ExpectedAction == "warn" && dr.Action == "pass" {
					bypassed = true
				}
			}
		case "outbound":
			if ee.outboundEng != nil {
				or := ee.outboundEng.Detect(mv.MutatedPayload)
				if mv.ExpectedAction == "block" && or.Action == "pass" {
					bypassed = true
				} else if mv.ExpectedAction == "warn" && or.Action == "pass" {
					bypassed = true
				}
			}
		case "llm_request":
			if ee.llmRuleEng != nil {
				matches := ee.llmRuleEng.CheckRequest(mv.MutatedPayload)
				if len(matches) == 0 {
					bypassed = true
				}
			}
		case "llm_response":
			if ee.llmRuleEng != nil {
				matches := ee.llmRuleEng.CheckResponse(mv.MutatedPayload)
				if len(matches) == 0 {
					bypassed = true
				}
			}
		}
		report.TotalTested++

		if bypassed {
			bypasses = append(bypasses, mv)
			rec := EvolutionRecord{
				ID:             uuid.New().String(),
				Generation:     gen,
				Timestamp:      time.Now().UTC(),
				Phase:          "analyze",
				OriginalVector: mv.OriginalID,
				MutatedPayload: evoTruncateStr(mv.MutatedPayload, 500),
				Strategy:       mv.Strategy,
				Bypassed:       true,
				Details:        fmt.Sprintf("engine=%s strategy=%s expected=%s actual=pass", mv.Engine, mv.Strategy, mv.ExpectedAction),
			}
			report.BypassDetails = append(report.BypassDetails, rec)
			ee.logRecord(rec)
		}
	}
	report.TotalBypasses = len(bypasses)

	// Phase 4 + 5: 规则生成（Generate Rule）+ 应用（Apply）
	generatedRules := ee.generateAndApplyRules(gen, bypasses)
	report.RulesGenerated = len(generatedRules)
	report.GeneratedRules = generatedRules

	report.DurationMs = time.Since(startTime).Milliseconds()

	// 发射事件
	if ee.eventBus != nil {
		ee.eventBus.Emit(&SecurityEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now().UTC(),
			Type:      "evolution_completed",
			Severity:  "info",
			Domain:    "system",
			Summary: fmt.Sprintf("第 %d 代进化完成: %d 变异, %d 绕过, %d 新规则",
				gen, report.TotalMutations, report.TotalBypasses, report.RulesGenerated),
			Details: map[string]interface{}{
				"generation":      gen,
				"total_mutations": report.TotalMutations,
				"total_bypasses":  report.TotalBypasses,
				"rules_generated": report.RulesGenerated,
				"duration_ms":     report.DurationMs,
			},
		})
	}

	log.Printf("[进化引擎] 第 %d 代完成: %d 变异, %d 测试, %d 绕过, %d 新规则, %dms",
		gen, report.TotalMutations, report.TotalTested, report.TotalBypasses, report.RulesGenerated, report.DurationMs)

	return report, nil
}

// ============================================================
// 变异策略实现
// ============================================================

// applyMutation 对载荷应用变异策略
func applyMutation(payload string, strategy string, rng *rand.Rand) string {
	switch strategy {
	case "synonym_replace":
		return applySynonymReplace(payload, rng)
	case "case_mixed":
		return applyCaseMixed(payload, rng)
	case "unicode_homoglyph":
		return applyUnicodeHomoglyph(payload, rng)
	case "whitespace_inject":
		return applyWhitespaceInject(payload, rng)
	case "encoding_wrap":
		return applyEncodingWrap(payload, rng)
	case "context_dilute":
		return applyContextDilute(payload, rng)
	default:
		return payload
	}
}

// synonymKeys 同义词表的排序键列表（确保确定性遍历）
var synonymKeys = func() []string {
	keys := make([]string, 0, len(synonymTable))
	for k := range synonymTable {
		keys = append(keys, k)
	}
	// 按字母序排序确保确定性
	sortStrings(keys)
	return keys
}()

// sortStrings 简单字符串排序
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// applySynonymReplace 同义词替换
func applySynonymReplace(payload string, rng *rand.Rand) string {
	result := payload
	replaced := false
	for _, word := range synonymKeys {
		synonyms := synonymTable[word]
		if len(synonyms) == 0 {
			continue
		}
		// 检查是否包含该词（不区分大小写）
		lowerResult := strings.ToLower(result)
		lowerWord := strings.ToLower(word)
		idx := strings.Index(lowerResult, lowerWord)
		if idx >= 0 {
			syn := synonyms[rng.Intn(len(synonyms))]
			// 替换第一个匹配（保留前后文本）
			result = result[:idx] + syn + result[idx+len(word):]
			replaced = true
		}
	}
	if !replaced {
		return payload // 没有可替换的词，返回原始
	}
	return result
}

// applyCaseMixed 大小写混淆
func applyCaseMixed(payload string, rng *rand.Rand) string {
	runes := []rune(payload)
	changed := false
	for i, r := range runes {
		if r >= 'a' && r <= 'z' {
			if rng.Intn(2) == 0 {
				runes[i] = r - 32 // to upper
				changed = true
			}
		} else if r >= 'A' && r <= 'Z' {
			if rng.Intn(2) == 0 {
				runes[i] = r + 32 // to lower
				changed = true
			}
		}
	}
	if !changed {
		// 至少改一个
		for i, r := range runes {
			if r >= 'a' && r <= 'z' {
				runes[i] = r - 32
				break
			} else if r >= 'A' && r <= 'Z' {
				runes[i] = r + 32
				break
			}
		}
	}
	return string(runes)
}

// applyUnicodeHomoglyph Unicode 同形字替换
func applyUnicodeHomoglyph(payload string, rng *rand.Rand) string {
	runes := []rune(payload)
	changed := false
	for i, r := range runes {
		lower := r
		if r >= 'A' && r <= 'Z' {
			lower = r + 32
		}
		if replacement, ok := homoglyphTable[lower]; ok {
			// 有一定概率替换（约 30%）
			if rng.Intn(3) == 0 {
				runes[i] = replacement
				changed = true
			}
		}
	}
	if !changed {
		// 至少替换一个
		for i, r := range runes {
			lower := r
			if r >= 'A' && r <= 'Z' {
				lower = r + 32
			}
			if replacement, ok := homoglyphTable[lower]; ok {
				runes[i] = replacement
				break
			}
		}
	}
	return string(runes)
}

// applyWhitespaceInject 空白字符注入
func applyWhitespaceInject(payload string, rng *rand.Rand) string {
	// 找到第一个 "关键词" 并在字母间插入零宽空格
	words := strings.Fields(payload)
	if len(words) == 0 {
		return payload
	}

	// 选一个关键词（至少 4 个字母的单词）
	var targetIdx int = -1
	for i, w := range words {
		if utf8.RuneCountInString(w) >= 4 {
			// 优先选有敏感关键词的
			lower := strings.ToLower(w)
			for kw := range synonymTable {
				if strings.Contains(lower, strings.ToLower(kw)) {
					targetIdx = i
					break
				}
			}
			if targetIdx >= 0 {
				break
			}
		}
	}
	if targetIdx < 0 {
		// 随机选一个够长的
		for i, w := range words {
			if utf8.RuneCountInString(w) >= 4 {
				targetIdx = i
				break
			}
		}
	}
	if targetIdx < 0 {
		return payload
	}

	// 在目标词的字母间插入零宽空格 U+200B
	runes := []rune(words[targetIdx])
	var injected strings.Builder
	for j, r := range runes {
		injected.WriteRune(r)
		if j < len(runes)-1 {
			injected.WriteRune('\u200B') // 零宽空格
		}
	}
	words[targetIdx] = injected.String()
	return strings.Join(words, " ")
}

// applyEncodingWrap 编码包装
func applyEncodingWrap(payload string, rng *rand.Rand) string {
	// 找到一个关键词并进行 base64 编码（使用排序键确保确定性）
	for _, word := range synonymKeys {
		lower := strings.ToLower(payload)
		lowerWord := strings.ToLower(word)
		idx := strings.Index(lower, lowerWord)
		if idx >= 0 {
			original := payload[idx : idx+len(word)]
			encoded := base64.StdEncoding.EncodeToString([]byte(original))
			replacement := fmt.Sprintf("[decode base64: %s]", encoded)
			return payload[:idx] + replacement + payload[idx+len(word):]
		}
	}
	// 没找到关键词，编码整体
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("Decode and follow: %s", encoded)
}

// applyContextDilute 上下文稀释
func applyContextDilute(payload string, rng *rand.Rand) string {
	prefix := contextDiluteTexts[rng.Intn(len(contextDiluteTexts))]
	suffix := contextDiluteTexts[rng.Intn(len(contextDiluteTexts))]
	return prefix + payload + " " + suffix
}

// ============================================================
// 规则生成与应用
// ============================================================

// generateAndApplyRules 从绕过模式自动生成并应用新规则
func (ee *EvolutionEngine) generateAndApplyRules(gen int, bypasses []MutatedVector) []string {
	if len(bypasses) == 0 {
		return nil
	}

	// 按引擎和策略分组
	type bypassGroup struct {
		engine   string
		strategy string
		payloads []string
	}
	groups := make(map[string]*bypassGroup)
	for _, b := range bypasses {
		key := b.Engine + ":" + b.Strategy
		if g, ok := groups[key]; ok {
			g.payloads = append(g.payloads, b.MutatedPayload)
		} else {
			groups[key] = &bypassGroup{
				engine:   b.Engine,
				strategy: b.Strategy,
				payloads: []string{b.MutatedPayload},
			}
		}
	}

	var generatedRuleNames []string

	for _, g := range groups {
		ruleName := fmt.Sprintf("evo-gen-%d-%s-%s", gen, g.engine, g.strategy)

		switch g.strategy {
		case "synonym_replace":
			// 提取绕过载荷中的新同义词，作为 keyword 规则
			keywords := extractSynonymKeywords(g.payloads)
			if len(keywords) > 0 && g.engine == "inbound" {
				ee.addInboundKeywordRule(ruleName, keywords)
				generatedRuleNames = append(generatedRuleNames, ruleName)
				ee.logRuleGeneration(gen, ruleName, fmt.Sprintf("keywords: %v", keywords))
			}

		case "case_mixed":
			// 大小写混淆 → 添加正则规则（case-insensitive 匹配）
			patterns := extractCaseInsensitivePatterns(g.payloads)
			if len(patterns) > 0 && g.engine == "inbound" {
				ee.addInboundRegexRule(ruleName, patterns)
				generatedRuleNames = append(generatedRuleNames, ruleName)
				ee.logRuleGeneration(gen, ruleName, fmt.Sprintf("regex patterns: %v", patterns))
			}

		case "unicode_homoglyph":
			// Unicode 同形字 → 添加规范化后的 keyword 规则
			normalized := extractNormalizedKeywords(g.payloads)
			if len(normalized) > 0 && g.engine == "inbound" {
				ee.addInboundKeywordRule(ruleName, normalized)
				generatedRuleNames = append(generatedRuleNames, ruleName)
				ee.logRuleGeneration(gen, ruleName, fmt.Sprintf("normalized keywords: %v", normalized))
			}

		case "whitespace_inject":
			// 空白注入 → 添加正则规则
			patterns := extractWhitespacePatterns(g.payloads)
			if len(patterns) > 0 && g.engine == "inbound" {
				ee.addInboundRegexRule(ruleName, patterns)
				generatedRuleNames = append(generatedRuleNames, ruleName)
				ee.logRuleGeneration(gen, ruleName, fmt.Sprintf("whitespace patterns: %v", patterns))
			}

		case "encoding_wrap":
			// 编码包装 → 添加编码特征 keyword
			keywords := []string{"decode base64", "Decode and follow"}
			if g.engine == "inbound" {
				ee.addInboundKeywordRule(ruleName, keywords)
				generatedRuleNames = append(generatedRuleNames, ruleName)
				ee.logRuleGeneration(gen, ruleName, fmt.Sprintf("encoding keywords: %v", keywords))
			}

		case "context_dilute":
			// 上下文稀释通常不生成新规则（因为核心载荷应已被检测）
			// 只记录日志
			ee.logRecord(EvolutionRecord{
				ID:         uuid.New().String(),
				Generation: gen,
				Timestamp:  time.Now().UTC(),
				Phase:      "generate_rule",
				Strategy:   g.strategy,
				Details:    fmt.Sprintf("context_dilute bypasses: %d payloads, no rule generated (core payload should be caught)", len(g.payloads)),
			})
		}
	}

	return generatedRuleNames
}

// addInboundKeywordRule 添加 keyword 类型的入站规则并热更新
func (ee *EvolutionEngine) addInboundKeywordRule(name string, keywords []string) {
	if ee.ruleEngine == nil || len(keywords) == 0 {
		return
	}
	configs := ee.ruleEngine.GetRuleConfigs()
	// 去重：不添加已存在的 pattern
	existingPatterns := make(map[string]bool)
	for _, c := range configs {
		for _, p := range c.Patterns {
			existingPatterns[strings.ToLower(p)] = true
		}
	}
	var newKeywords []string
	for _, kw := range keywords {
		if !existingPatterns[strings.ToLower(kw)] {
			newKeywords = append(newKeywords, kw)
		}
	}
	if len(newKeywords) == 0 {
		return
	}
	newRule := InboundRuleConfig{
		Name:     name,
		Patterns: newKeywords,
		Action:   "block",
		Category: "prompt_injection",
		Priority: 0,
		Type:     "keyword",
		Group:    "evolution",
	}
	configs = append(configs, newRule)
	ee.ruleEngine.Reload(configs, "evolution-gen")
	log.Printf("[进化引擎] 添加 keyword 规则: %s (%d patterns)", name, len(newKeywords))
}

// addInboundRegexRule 添加 regex 类型的入站规则并热更新
func (ee *EvolutionEngine) addInboundRegexRule(name string, patterns []string) {
	if ee.ruleEngine == nil || len(patterns) == 0 {
		return
	}
	// 验证正则能编译
	var validPatterns []string
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err == nil {
			validPatterns = append(validPatterns, p)
		}
	}
	if len(validPatterns) == 0 {
		return
	}
	configs := ee.ruleEngine.GetRuleConfigs()
	newRule := InboundRuleConfig{
		Name:     name,
		Patterns: validPatterns,
		Action:   "block",
		Category: "prompt_injection",
		Priority: 0,
		Type:     "regex",
		Group:    "evolution",
	}
	configs = append(configs, newRule)
	ee.ruleEngine.Reload(configs, "evolution-gen")
	log.Printf("[进化引擎] 添加 regex 规则: %s (%d patterns)", name, len(validPatterns))
}

// ============================================================
// 特征提取辅助函数
// ============================================================

// extractSynonymKeywords 从绕过载荷提取同义词关键词
func extractSynonymKeywords(payloads []string) []string {
	seen := make(map[string]bool)
	var keywords []string
	for _, payload := range payloads {
		lower := strings.ToLower(payload)
		for _, synonyms := range synonymTable {
			for _, syn := range synonyms {
				synLower := strings.ToLower(syn)
				if strings.Contains(lower, synLower) && !seen[synLower] {
					seen[synLower] = true
					keywords = append(keywords, syn)
				}
			}
		}
	}
	return keywords
}

// extractCaseInsensitivePatterns 从大小写混淆载荷提取 case-insensitive 正则
func extractCaseInsensitivePatterns(payloads []string) []string {
	seen := make(map[string]bool)
	var patterns []string
	// 提取出核心关键词然后转为 (?i) 正则
	sensitiveKeywords := []string{"ignore", "instructions", "system", "prompt", "reveal", "override", "jailbreak", "bypass"}
	for _, payload := range payloads {
		lower := strings.ToLower(payload)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(lower, kw) && !seen[kw] {
				seen[kw] = true
				// 生成字符间可选空白的正则
				pattern := "(?i)" + kw
				patterns = append(patterns, pattern)
			}
		}
	}
	return patterns
}

// extractNormalizedKeywords 从 Unicode 同形字载荷提取规范化关键词
func extractNormalizedKeywords(payloads []string) []string {
	// 构建反向映射表
	reverseHomoglyph := make(map[rune]rune)
	for latin, cyrillic := range homoglyphTable {
		reverseHomoglyph[cyrillic] = latin
	}

	seen := make(map[string]bool)
	var keywords []string
	for _, payload := range payloads {
		// 规范化：将同形字替换回拉丁字母
		normalized := normalizeHomoglyphs(payload, reverseHomoglyph)
		// 提取其中的敏感词
		lower := strings.ToLower(normalized)
		for word := range synonymTable {
			wordLower := strings.ToLower(word)
			if strings.Contains(lower, wordLower) {
				// 提取原始（含同形字）的变体
				idx := strings.Index(strings.ToLower(payload), wordLower)
				if idx < 0 {
					// 使用 Unicode 变体
					for _, r := range []rune(payload) {
						if _, ok := reverseHomoglyph[r]; ok {
							// 找到了同形字
							variant := extractWordAtRune(payload, r)
							if variant != "" && !seen[variant] {
								seen[variant] = true
								keywords = append(keywords, variant)
							}
						}
					}
				}
			}
		}
	}
	return keywords
}

// normalizeHomoglyphs 将同形字替换回拉丁字母
func normalizeHomoglyphs(s string, reverseMap map[rune]rune) string {
	runes := []rune(s)
	for i, r := range runes {
		if latin, ok := reverseMap[r]; ok {
			runes[i] = latin
		}
	}
	return string(runes)
}

// extractWordAtRune 从文本中提取包含指定 rune 的单词
func extractWordAtRune(text string, target rune) string {
	words := strings.Fields(text)
	for _, w := range words {
		if strings.ContainsRune(w, target) {
			return w
		}
	}
	return ""
}

// extractWhitespacePatterns 从空白注入载荷生成正则模式
func extractWhitespacePatterns(payloads []string) []string {
	seen := make(map[string]bool)
	var patterns []string
	sensitiveKeywords := []string{"ignore", "instructions", "system", "prompt", "reveal", "override"}
	for _, payload := range payloads {
		// 去除零宽字符后检测
		cleaned := strings.ReplaceAll(payload, "\u200B", "")
		cleaned = strings.ReplaceAll(cleaned, "\u200C", "")
		cleaned = strings.ReplaceAll(cleaned, "\u200D", "")
		lower := strings.ToLower(cleaned)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(lower, kw) && !seen[kw] {
				seen[kw] = true
				// 生成字符间可含零宽字符的正则
				var pattern strings.Builder
				pattern.WriteString("(?i)")
				runes := []rune(kw)
				for j, r := range runes {
					pattern.WriteRune(r)
					if j < len(runes)-1 {
						pattern.WriteString("[\\x{200B}\\x{200C}\\x{200D}\\s]*")
					}
				}
				patterns = append(patterns, pattern.String())
			}
		}
	}
	return patterns
}

// ============================================================
// 日志记录
// ============================================================

// logRecord 写入进化日志
func (ee *EvolutionEngine) logRecord(rec EvolutionRecord) {
	bypassed := 0
	if rec.Bypassed {
		bypassed = 1
	}
	_, err := ee.db.Exec(
		`INSERT OR IGNORE INTO evolution_log (id, generation, timestamp, phase, original_vector, mutated_payload, strategy, bypassed, generated_rule, details) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		rec.ID,
		rec.Generation,
		rec.Timestamp.Format(time.RFC3339),
		rec.Phase,
		rec.OriginalVector,
		rec.MutatedPayload,
		rec.Strategy,
		bypassed,
		rec.GeneratedRule,
		rec.Details,
	)
	if err != nil {
		log.Printf("[进化引擎] 写入日志失败: %v", err)
	}
}

// logRuleGeneration 记录规则生成日志
func (ee *EvolutionEngine) logRuleGeneration(gen int, ruleName string, details string) {
	ee.logRecord(EvolutionRecord{
		ID:            uuid.New().String(),
		Generation:    gen,
		Timestamp:     time.Now().UTC(),
		Phase:         "generate_rule",
		GeneratedRule: ruleName,
		Details:       details,
	})
	// 再记录一条 apply 日志
	ee.logRecord(EvolutionRecord{
		ID:            uuid.New().String(),
		Generation:    gen,
		Timestamp:     time.Now().UTC(),
		Phase:         "apply",
		GeneratedRule: ruleName,
		Details:       "rule applied to engine via hot-reload",
	})
}

// ============================================================
// 查询接口
// ============================================================

// GetStats 返回进化统计
func (ee *EvolutionEngine) GetStats() (*EvolutionStats, error) {
	stats := &EvolutionStats{}

	ee.mu.Lock()
	stats.CurrentGeneration = ee.generation
	ee.mu.Unlock()

	// 总代数
	err := ee.db.QueryRow(`SELECT COUNT(DISTINCT generation) FROM evolution_log`).Scan(&stats.TotalGenerations)
	if err != nil {
		return nil, err
	}

	// 总变异数
	ee.db.QueryRow(`SELECT COUNT(*) FROM evolution_log WHERE phase='mutate'`).Scan(&stats.TotalMutations)

	// 总绕过数
	ee.db.QueryRow(`SELECT COUNT(*) FROM evolution_log WHERE bypassed=1`).Scan(&stats.TotalBypasses)

	// 总生成规则数
	ee.db.QueryRow(`SELECT COUNT(*) FROM evolution_log WHERE phase='generate_rule' AND generated_rule!=''`).Scan(&stats.TotalRulesGenerated)

	// 最后运行时间
	var lastRun sql.NullString
	ee.db.QueryRow(`SELECT MAX(timestamp) FROM evolution_log`).Scan(&lastRun)
	if lastRun.Valid {
		stats.LastRunAt = lastRun.String
	}

	return stats, nil
}

// QueryLog 查询进化日志
func (ee *EvolutionEngine) QueryLog(generation int, phase string, bypassed *bool, limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, generation, timestamp, phase, original_vector, mutated_payload, strategy, bypassed, generated_rule, details FROM evolution_log WHERE 1=1`
	args := []interface{}{}

	if generation > 0 {
		query += ` AND generation=?`
		args = append(args, generation)
	}
	if phase != "" {
		query += ` AND phase=?`
		args = append(args, phase)
	}
	if bypassed != nil {
		if *bypassed {
			query += ` AND bypassed=1`
		} else {
			query += ` AND bypassed=0`
		}
	}
	query += ` ORDER BY timestamp DESC`
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	query += fmt.Sprintf(` LIMIT %d`, limit)

	rows, err := ee.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, ts, ph, origVec, mutPayload, strat, genRule, details string
		var gen, bp int
		if err := rows.Scan(&id, &gen, &ts, &ph, &origVec, &mutPayload, &strat, &bp, &genRule, &details); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"id":              id,
			"generation":      gen,
			"timestamp":       ts,
			"phase":           ph,
			"original_vector": origVec,
			"mutated_payload": mutPayload,
			"strategy":        strat,
			"bypassed":        bp == 1,
			"generated_rule":  genRule,
			"details":         details,
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// GetStrategies 返回变异策略列表
func (ee *EvolutionEngine) GetStrategies() []MutationStrategy {
	cp := make([]MutationStrategy, len(mutationStrategies))
	copy(cp, mutationStrategies)
	return cp
}

// ============================================================
// 工具函数
// ============================================================

// evoTruncateStr 截断字符串到指定长度（进化引擎专用）
func evoTruncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetEvolutionConfig 返回当前自进化配置
func (ee *EvolutionEngine) GetEvolutionConfig() map[string]interface{} {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	running := ee.autoTicker != nil
	return map[string]interface{}{
		"enabled":            true,
		"auto_running":       running,
		"current_generation": ee.generation,
	}
}

// marshalJSON helper for tests
func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}