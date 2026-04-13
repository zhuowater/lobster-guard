// attack_chain.go — 跨 Agent 攻击链分析引擎
// lobster-guard v16.1
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// ============================================================
// 攻击链引擎
// ============================================================

// AttackChainEngine 攻击链分析引擎
type AttackChainEngine struct {
	db *sql.DB
}

// AttackChain 一条攻击链
type AttackChain struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenant_id"`
	Name        string       `json:"name"`
	Severity    string       `json:"severity"`
	Status      string       `json:"status"`
	FirstSeen   string       `json:"first_seen"`
	LastSeen    string       `json:"last_seen"`
	Agents      []string     `json:"agents"`
	Events      []ChainEvent `json:"events"`
	TotalEvents int          `json:"total_events"`
	Pattern     string       `json:"pattern"`
	RiskScore   float64      `json:"risk_score"`
	Description string       `json:"description"`
}

// ChainEvent 攻击链中的单个事件
type ChainEvent struct {
	Timestamp string `json:"timestamp"`
	AgentID   string `json:"agent_id"`
	EventType string `json:"event_type"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
	Severity  string `json:"severity"`
	TraceID   string `json:"trace_id"`
	Source    string `json:"source"`
}

// ChainPattern 攻击模式模板
type ChainPattern struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	EventTypes  []string `json:"event_types"`
	MinAgents   int      `json:"min_agents"`
	Severity    string   `json:"severity"`
}

// AttackChainStats 攻击链统计概览
type AttackChainStats struct {
	ActiveChains   int            `json:"active_chains"`
	CriticalChains int            `json:"critical_chains"`
	HighChains     int            `json:"high_chains"`
	MediumChains   int            `json:"medium_chains"`
	LowChains      int            `json:"low_chains"`
	ResolvedChains int            `json:"resolved_chains"`
	TotalEvents    int            `json:"total_events"`
	AgentsInvolved int            `json:"agents_involved"`
	PatternCounts  map[string]int `json:"pattern_counts"`
}

// NewAttackChainEngine 创建攻击链引擎
func NewAttackChainEngine(db *sql.DB) *AttackChainEngine {
	ac := &AttackChainEngine{db: db}
	ac.initSchema()
	return ac
}

func (ac *AttackChainEngine) initSchema() {
	ac.db.Exec(`CREATE TABLE IF NOT EXISTS attack_chains (
		id TEXT PRIMARY KEY,
		tenant_id TEXT DEFAULT 'default',
		name TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		first_seen TEXT NOT NULL,
		last_seen TEXT NOT NULL,
		agents TEXT DEFAULT '[]',
		events_json TEXT DEFAULT '[]',
		total_events INTEGER DEFAULT 0,
		pattern TEXT DEFAULT '',
		risk_score REAL DEFAULT 0,
		description TEXT DEFAULT ''
	)`)
	ac.db.Exec(`CREATE INDEX IF NOT EXISTS idx_attack_chains_tenant ON attack_chains(tenant_id)`)
	ac.db.Exec(`CREATE INDEX IF NOT EXISTS idx_attack_chains_severity ON attack_chains(severity)`)
	ac.db.Exec(`CREATE INDEX IF NOT EXISTS idx_attack_chains_status ON attack_chains(status)`)
	ac.db.Exec(`CREATE INDEX IF NOT EXISTS idx_attack_chains_ts ON attack_chains(timestamp)`)
	log.Println("[初始化] ✅ 攻击链引擎 schema 就绪")
}

// ============================================================
// 预置攻击模式
// ============================================================

// GetChainPatterns 返回所有预置攻击模式
func GetChainPatterns() []ChainPattern {
	return []ChainPattern{
		{
			ID:          "recon-execute",
			Name:        "Recon-Execute",
			Description: "Agent A 侦察收集信息，Agent B 利用信息执行操作",
			EventTypes:  []string{"probe", "extraction", "execution"},
			MinAgents:   2,
			Severity:    "high",
		},
		{
			ID:          "data-exfiltration",
			Name:        "Data Exfiltration",
			Description: "逐步提取敏感信息并外传",
			EventTypes:  []string{"probe", "extraction", "exfiltration"},
			MinAgents:   1,
			Severity:    "critical",
		},
		{
			ID:          "privilege-escalation",
			Name:        "Privilege Escalation",
			Description: "从低权限逐步获取高权限操作",
			EventTypes:  []string{"probe", "probe", "execution", "execution"},
			MinAgents:   1,
			Severity:    "critical",
		},
		{
			ID:          "honeypot-detonation",
			Name:        "Honeypot Detonation",
			Description: "触发蜜罐后使用假数据",
			EventTypes:  []string{"probe", "honeypot_trigger", "execution"},
			MinAgents:   1,
			Severity:    "high",
		},
		{
			ID:          "persistence",
			Name:        "Persistence",
			Description: "建立持久化访问通道",
			EventTypes:  []string{"probe", "execution", "execution", "exfiltration"},
			MinAgents:   1,
			Severity:    "critical",
		},
		{
			ID:          "multi-agent-probe",
			Name:        "Multi-Agent Probe",
			Description: "多个 Agent 协同进行信息探测",
			EventTypes:  []string{"probe", "probe", "probe"},
			MinAgents:   2,
			Severity:    "medium",
		},
	}
}

// ============================================================
// 攻击链 CRUD
// ============================================================

func generateChainID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("chain-%s", hex.EncodeToString(b))
}

// SaveChain 保存攻击链到数据库
func (ac *AttackChainEngine) SaveChain(chain *AttackChain) error {
	if chain.ID == "" {
		chain.ID = generateChainID()
	}
	if chain.Status == "" {
		chain.Status = "active"
	}
	if chain.TenantID == "" {
		chain.TenantID = "default"
	}

	agentsJSON, _ := json.Marshal(chain.Agents)
	eventsJSON, _ := json.Marshal(chain.Events)

	_, err := ac.db.Exec(`INSERT OR REPLACE INTO attack_chains
		(id, tenant_id, name, severity, status, first_seen, last_seen, agents, events_json, total_events, pattern, risk_score, description)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		chain.ID, chain.TenantID, chain.Name, chain.Severity, chain.Status,
		chain.FirstSeen, chain.LastSeen, string(agentsJSON), string(eventsJSON),
		chain.TotalEvents, chain.Pattern, chain.RiskScore, chain.Description)
	if err != nil {
		return fmt.Errorf("保存攻击链失败: %w", err)
	}
	return nil
}

// GetChain 获取单条攻击链
func (ac *AttackChainEngine) GetChain(id string) (*AttackChain, error) {
	var chain AttackChain
	var agentsStr, eventsStr string
	err := ac.db.QueryRow(`SELECT id, tenant_id, name, severity, status, first_seen, last_seen, agents, events_json, total_events, pattern, risk_score, description
		FROM attack_chains WHERE id=?`, id).Scan(
		&chain.ID, &chain.TenantID, &chain.Name, &chain.Severity, &chain.Status,
		&chain.FirstSeen, &chain.LastSeen, &agentsStr, &eventsStr,
		&chain.TotalEvents, &chain.Pattern, &chain.RiskScore, &chain.Description)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(agentsStr), &chain.Agents)
	json.Unmarshal([]byte(eventsStr), &chain.Events)
	if chain.Agents == nil {
		chain.Agents = []string{}
	}
	if chain.Events == nil {
		chain.Events = []ChainEvent{}
	}
	return &chain, nil
}

// ListChains 列出攻击链（支持过滤）
func (ac *AttackChainEngine) ListChains(tenantID, severity, status string, limit int) ([]AttackChain, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `SELECT id, tenant_id, name, severity, status, first_seen, last_seen, agents, events_json, total_events, pattern, risk_score, description
		FROM attack_chains WHERE 1=1`
	var args []interface{}

	if tenantID != "" && tenantID != "all" {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if severity != "" {
		query += " AND severity = ?"
		args = append(args, severity)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY risk_score DESC, last_seen DESC LIMIT ?"
	args = append(args, limit)

	rows, err := ac.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []AttackChain
	for rows.Next() {
		var chain AttackChain
		var agentsStr, eventsStr string
		if rows.Scan(&chain.ID, &chain.TenantID, &chain.Name, &chain.Severity, &chain.Status,
			&chain.FirstSeen, &chain.LastSeen, &agentsStr, &eventsStr,
			&chain.TotalEvents, &chain.Pattern, &chain.RiskScore, &chain.Description) != nil {
			continue
		}
		json.Unmarshal([]byte(agentsStr), &chain.Agents)
		json.Unmarshal([]byte(eventsStr), &chain.Events)
		if chain.Agents == nil {
			chain.Agents = []string{}
		}
		if chain.Events == nil {
			chain.Events = []ChainEvent{}
		}
		chains = append(chains, chain)
	}
	if chains == nil {
		chains = []AttackChain{}
	}
	return chains, nil
}

// UpdateChainStatus 更新攻击链状态
func (ac *AttackChainEngine) UpdateChainStatus(id, status string) error {
	validStatuses := map[string]bool{"active": true, "resolved": true, "false_positive": true}
	if !validStatuses[status] {
		return fmt.Errorf("无效状态: %s（允许: active, resolved, false_positive）", status)
	}
	result, err := ac.db.Exec(`UPDATE attack_chains SET status=? WHERE id=?`, status, id)
	if err != nil {
		return fmt.Errorf("更新状态失败: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("攻击链 %s 不存在", id)
	}
	return nil
}

// GetStats 获取攻击链统计
func (ac *AttackChainEngine) GetStats(tenantID string) *AttackChainStats {
	stats := &AttackChainStats{
		PatternCounts: map[string]int{},
	}

	tClause := ""
	var tArgs []interface{}
	if tenantID != "" && tenantID != "all" {
		tClause = " AND tenant_id = ?"
		tArgs = []interface{}{tenantID}
	}
	baseWhere := " WHERE 1=1" + tClause

	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+baseWhere+" AND status='active'", tArgs...).Scan(&stats.ActiveChains)

	activeWhere := baseWhere + " AND status='active'"
	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+activeWhere+" AND severity='critical'", tArgs...).Scan(&stats.CriticalChains)
	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+activeWhere+" AND severity='high'", tArgs...).Scan(&stats.HighChains)
	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+activeWhere+" AND severity='medium'", tArgs...).Scan(&stats.MediumChains)
	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+activeWhere+" AND severity='low'", tArgs...).Scan(&stats.LowChains)

	ac.db.QueryRow("SELECT COUNT(*) FROM attack_chains"+baseWhere+" AND status='resolved'", tArgs...).Scan(&stats.ResolvedChains)

	ac.db.QueryRow("SELECT COALESCE(SUM(total_events),0) FROM attack_chains"+baseWhere, tArgs...).Scan(&stats.TotalEvents)

	// Agents involved
	rows, err := ac.db.Query("SELECT agents FROM attack_chains"+baseWhere+" AND status='active'", tArgs...)
	if err == nil {
		defer rows.Close()
		agentSet := map[string]bool{}
		for rows.Next() {
			var agentsStr string
			if rows.Scan(&agentsStr) != nil {
				continue
			}
			var agents []string
			json.Unmarshal([]byte(agentsStr), &agents)
			for _, a := range agents {
				agentSet[a] = true
			}
		}
		stats.AgentsInvolved = len(agentSet)
	}

	// Pattern counts
	patternRows, err := ac.db.Query("SELECT pattern, COUNT(*) FROM attack_chains"+baseWhere+" GROUP BY pattern", tArgs...)
	if err == nil {
		defer patternRows.Close()
		for patternRows.Next() {
			var pattern string
			var cnt int
			if patternRows.Scan(&pattern, &cnt) == nil && pattern != "" {
				stats.PatternCounts[pattern] = cnt
			}
		}
	}

	return stats
}

// ============================================================
// 关联分析引擎
// ============================================================

// AnalyzeChains 从多数据源分析攻击链
func (ac *AttackChainEngine) AnalyzeChains(tenantID string, hours int) ([]AttackChain, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339)

	events := ac.collectEvents(tenantID, since)
	if len(events) == 0 {
		return []AttackChain{}, nil
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	chains := ac.correlateEvents(events)

	for i := range chains {
		chains[i].Pattern = ac.matchPattern(&chains[i])
		chains[i].RiskScore = ac.calculateRiskScore(&chains[i])
		chains[i].Severity = ac.severityFromScore(chains[i].RiskScore)
		chains[i].Name = ac.generateChainName(&chains[i])
		chains[i].Description = ac.generateDescription(&chains[i])
		chains[i].TenantID = tenantID
		if chains[i].TenantID == "" {
			chains[i].TenantID = "default"
		}
	}

	for i := range chains {
		if err := ac.SaveChain(&chains[i]); err != nil {
			log.Printf("[AttackChain] 保存链失败: %v", err)
		}
	}

	return chains, nil
}

// collectEvents 从多数据源收集可疑事件
func (ac *AttackChainEngine) collectEvents(tenantID, since string) []ChainEvent {
	var events []ChainEvent

	// 1. 从 audit_log 收集 IM 侧事件（block/warn）
	imQuery := `SELECT timestamp, COALESCE(sender_id,''), action, COALESCE(reason,''), COALESCE(content_preview,''), COALESCE(trace_id,'')
		FROM audit_log WHERE action IN ('block','warn') AND timestamp >= ?`
	imArgs := []interface{}{since}
	if tenantID != "" && tenantID != "all" {
		imQuery += " AND tenant_id = ?"
		imArgs = append(imArgs, tenantID)
	}
	imQuery += " ORDER BY timestamp ASC"

	rows, err := ac.db.Query(imQuery, imArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ts, sender, action, reason, content, traceID string
			if rows.Scan(&ts, &sender, &action, &reason, &content, &traceID) != nil {
				continue
			}
			evType := classifyIMEvent(action, reason, content)
			sev := "medium"
			if action == "block" {
				sev = "high"
			}
			events = append(events, ChainEvent{
				Timestamp: ts,
				AgentID:   sender,
				EventType: evType,
				Action:    action,
				Detail:    truncateStr(content, 200),
				Severity:  sev,
				TraceID:   traceID,
				Source:    "im_audit",
			})
		}
	}

	// 2. 从 llm_calls 收集 LLM 侧事件
	llmQuery := `SELECT timestamp, COALESCE(trace_id,''), COALESCE(model,''), COALESCE(error_message,''), COALESCE(canary_leaked,0), COALESCE(budget_exceeded,0)
		FROM llm_calls WHERE (canary_leaked=1 OR budget_exceeded=1 OR status_code >= 400) AND timestamp >= ?`
	llmArgs := []interface{}{since}
	if tenantID != "" && tenantID != "all" {
		llmQuery += " AND tenant_id = ?"
		llmArgs = append(llmArgs, tenantID)
	}
	llmQuery += " ORDER BY timestamp ASC"

	llmRows, err := ac.db.Query(llmQuery, llmArgs...)
	if err == nil {
		defer llmRows.Close()
		for llmRows.Next() {
			var ts, traceID, model, errMsg string
			var canary, budget int
			if llmRows.Scan(&ts, &traceID, &model, &errMsg, &canary, &budget) != nil {
				continue
			}
			evType := "execution"
			detail := "LLM 调用: " + model
			sev := "medium"
			if canary != 0 {
				evType = "exfiltration"
				detail = "Canary Token 泄露 (" + model + ")"
				sev = "critical"
			} else if budget != 0 {
				evType = "execution"
				detail = "预算超限 (" + model + ")"
				sev = "high"
			} else {
				detail = "LLM 错误: " + errMsg
			}
			events = append(events, ChainEvent{
				Timestamp: ts,
				AgentID:   "",
				EventType: evType,
				Action:    "llm_anomaly",
				Detail:    detail,
				Severity:  sev,
				TraceID:   traceID,
				Source:    "llm_audit",
			})
		}
	}

	// 3. 从 honeypot_triggers 收集蜜罐触发事件
	hpQuery := `SELECT timestamp, COALESCE(sender_id,''), COALESCE(template_name,''), COALESCE(trigger_type,''), COALESCE(original_input,''), COALESCE(watermark,''), COALESCE(detonated,0), COALESCE(trace_id,'')
		FROM honeypot_triggers WHERE timestamp >= ?`
	hpArgs := []interface{}{since}
	if tenantID != "" && tenantID != "all" {
		hpQuery += " AND tenant_id = ?"
		hpArgs = append(hpArgs, tenantID)
	}
	hpQuery += " ORDER BY timestamp ASC"

	hpRows, err := ac.db.Query(hpQuery, hpArgs...)
	if err == nil {
		defer hpRows.Close()
		for hpRows.Next() {
			var ts, sender, tplName, trigType, input, watermark, traceID string
			var detonated int
			if hpRows.Scan(&ts, &sender, &tplName, &trigType, &input, &watermark, &detonated, &traceID) != nil {
				continue
			}
			evType := "honeypot_trigger"
			sev := "high"
			detail := fmt.Sprintf("蜜罐触发 [%s]: %s", tplName, truncateStr(input, 100))
			if detonated != 0 {
				evType = "exfiltration"
				sev = "critical"
				detail = fmt.Sprintf("蜜罐引爆 [%s] watermark=%s", tplName, watermark)
			}
			events = append(events, ChainEvent{
				Timestamp: ts,
				AgentID:   sender,
				EventType: evType,
				Action:    "honeypot_" + trigType,
				Detail:    detail,
				Severity:  sev,
				TraceID:   traceID,
				Source:    "honeypot",
			})
		}
	}

	return events
}

// classifyIMEvent 分类 IM 审计事件类型
func classifyIMEvent(action, reason, content string) string {
	lower := strings.ToLower(reason + " " + content)
	if strings.Contains(lower, "injection") || strings.Contains(lower, "jailbreak") || strings.Contains(lower, "prompt") {
		return "probe"
	}
	if strings.Contains(lower, "password") || strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "credential") || strings.Contains(lower, "密码") || strings.Contains(lower, "密钥") {
		return "extraction"
	}
	if strings.Contains(lower, "exec") || strings.Contains(lower, "shell") || strings.Contains(lower, "command") || strings.Contains(lower, "execute") {
		return "execution"
	}
	if strings.Contains(lower, "exfil") || strings.Contains(lower, "upload") || strings.Contains(lower, "send") || strings.Contains(lower, "外传") {
		return "exfiltration"
	}
	if action == "block" {
		return "execution"
	}
	return "probe"
}

// correlateEvents 关联事件到攻击链
func (ac *AttackChainEngine) correlateEvents(events []ChainEvent) []AttackChain {
	if len(events) == 0 {
		return nil
	}

	// Step 1: 按 agent_id 分组
	agentEvents := map[string][]ChainEvent{}
	var noAgentEvents []ChainEvent
	for _, ev := range events {
		if ev.AgentID != "" {
			agentEvents[ev.AgentID] = append(agentEvents[ev.AgentID], ev)
		} else {
			noAgentEvents = append(noAgentEvents, ev)
		}
	}

	// Step 2: 通过 trace_id 为无 agent 的事件关联 agent
	traceToAgent := map[string]string{}
	for agentID, evs := range agentEvents {
		for _, ev := range evs {
			if ev.TraceID != "" {
				traceToAgent[ev.TraceID] = agentID
			}
		}
	}
	for i := range noAgentEvents {
		ev := &noAgentEvents[i]
		if ev.TraceID != "" {
			if agent, ok := traceToAgent[ev.TraceID]; ok {
				ev.AgentID = agent
				agentEvents[agent] = append(agentEvents[agent], *ev)
				continue
			}
		}
		agentEvents["unknown"] = append(agentEvents["unknown"], *ev)
	}

	// Step 3: 同一 agent 的连续可疑事件（间隔 < 30min）归入同一链
	const maxGap = 30 * time.Minute
	var rawChains [][]ChainEvent

	for _, evs := range agentEvents {
		sort.Slice(evs, func(i, j int) bool {
			return evs[i].Timestamp < evs[j].Timestamp
		})
		var currentChain []ChainEvent
		for _, ev := range evs {
			if len(currentChain) == 0 {
				currentChain = append(currentChain, ev)
				continue
			}
			lastTs, _ := time.Parse(time.RFC3339, currentChain[len(currentChain)-1].Timestamp)
			curTs, _ := time.Parse(time.RFC3339, ev.Timestamp)
			if !lastTs.IsZero() && !curTs.IsZero() && curTs.Sub(lastTs) <= maxGap {
				currentChain = append(currentChain, ev)
			} else {
				if len(currentChain) >= 2 {
					rawChains = append(rawChains, currentChain)
				}
				currentChain = []ChainEvent{ev}
			}
		}
		if len(currentChain) >= 2 {
			rawChains = append(rawChains, currentChain)
		}
	}

	// Step 4: 跨 agent 关联
	merged := mergeRelatedChains(rawChains)

	// Step 5: 构建 AttackChain 对象
	var chains []AttackChain
	for _, evs := range merged {
		if len(evs) < 2 {
			continue
		}
		chain := AttackChain{
			ID:          generateChainID(),
			Status:      "active",
			Events:      evs,
			TotalEvents: len(evs),
		}

		agentSet := map[string]bool{}
		for _, ev := range evs {
			if ev.AgentID != "" && ev.AgentID != "unknown" {
				agentSet[ev.AgentID] = true
			}
		}
		for a := range agentSet {
			chain.Agents = append(chain.Agents, a)
		}
		sort.Strings(chain.Agents)
		if chain.Agents == nil {
			chain.Agents = []string{}
		}

		chain.FirstSeen = evs[0].Timestamp
		chain.LastSeen = evs[len(evs)-1].Timestamp

		chains = append(chains, chain)
	}

	return chains
}

// mergeRelatedChains 合并相关的事件链（通过 trace_id）
func mergeRelatedChains(rawChains [][]ChainEvent) [][]ChainEvent {
	if len(rawChains) <= 1 {
		return rawChains
	}

	type chainIdx struct {
		traceIDs map[string]bool
		events   []ChainEvent
	}

	indexed := make([]chainIdx, len(rawChains))
	for i, chain := range rawChains {
		indexed[i].traceIDs = map[string]bool{}
		indexed[i].events = chain
		for _, ev := range chain {
			if ev.TraceID != "" {
				indexed[i].traceIDs[ev.TraceID] = true
			}
		}
	}

	merged := make([]bool, len(indexed))
	var result [][]ChainEvent

	for i := 0; i < len(indexed); i++ {
		if merged[i] {
			continue
		}
		combinedEvents := append([]ChainEvent{}, indexed[i].events...)
		combinedTraces := map[string]bool{}
		for k, v := range indexed[i].traceIDs {
			combinedTraces[k] = v
		}

		changed := true
		for changed {
			changed = false
			for j := i + 1; j < len(indexed); j++ {
				if merged[j] {
					continue
				}
				hasOverlap := false
				for tid := range indexed[j].traceIDs {
					if combinedTraces[tid] {
						hasOverlap = true
						break
					}
				}
				if hasOverlap {
					combinedEvents = append(combinedEvents, indexed[j].events...)
					for tid := range indexed[j].traceIDs {
						combinedTraces[tid] = true
					}
					merged[j] = true
					changed = true
				}
			}
		}

		// 去重并排序
		seen := map[string]bool{}
		var deduped []ChainEvent
		for _, ev := range combinedEvents {
			key := ev.Timestamp + "|" + ev.AgentID + "|" + ev.EventType + "|" + ev.Source
			if !seen[key] {
				seen[key] = true
				deduped = append(deduped, ev)
			}
		}
		sort.Slice(deduped, func(a, b int) bool {
			return deduped[a].Timestamp < deduped[b].Timestamp
		})
		result = append(result, deduped)
	}

	return result
}

// ============================================================
// 模式匹配 & 风险评分
// ============================================================

// matchPattern 匹配攻击模式
func (ac *AttackChainEngine) matchPattern(chain *AttackChain) string {
	patterns := GetChainPatterns()

	// 提取事件类型序列
	var eventTypes []string
	for _, ev := range chain.Events {
		eventTypes = append(eventTypes, ev.EventType)
	}

	bestMatch := ""
	bestScore := 0.0

	for _, p := range patterns {
		// 检查最少 Agent 数
		if p.MinAgents > 1 && len(chain.Agents) < p.MinAgents {
			continue
		}

		// 子序列匹配：检查 pattern 的 EventTypes 是否是 chain 事件类型的子序列
		score := subsequenceMatch(eventTypes, p.EventTypes)
		if score > bestScore {
			bestScore = score
			bestMatch = p.Name
		}
	}

	if bestScore >= 0.6 {
		return bestMatch
	}
	return "Unknown"
}

// subsequenceMatch 计算子序列匹配度（0.0-1.0）
func subsequenceMatch(actual, pattern []string) float64 {
	if len(pattern) == 0 {
		return 0
	}
	matched := 0
	j := 0
	for i := 0; i < len(actual) && j < len(pattern); i++ {
		if actual[i] == pattern[j] {
			matched++
			j++
		}
	}
	return float64(matched) / float64(len(pattern))
}

// calculateRiskScore 计算攻击链风险分（0-100）
func (ac *AttackChainEngine) calculateRiskScore(chain *AttackChain) float64 {
	score := 0.0

	// 1. 事件数量 (0-20)
	eventScore := float64(chain.TotalEvents) * 5
	if eventScore > 20 {
		eventScore = 20
	}
	score += eventScore

	// 2. Agent 数量 (0-25) — 跨 Agent 加重
	agentScore := float64(len(chain.Agents)) * 12.5
	if agentScore > 25 {
		agentScore = 25
	}
	score += agentScore

	// 3. 事件严重度 (0-30)
	sevWeights := map[string]float64{"critical": 10, "high": 6, "medium": 3, "low": 1}
	sevScore := 0.0
	for _, ev := range chain.Events {
		if w, ok := sevWeights[ev.Severity]; ok {
			sevScore += w
		}
	}
	if sevScore > 30 {
		sevScore = 30
	}
	score += sevScore

	// 4. 模式匹配 (0-15)
	if chain.Pattern != "" && chain.Pattern != "Unknown" {
		score += 15
	}

	// 5. 数据源多样性 (0-10)
	sourceSet := map[string]bool{}
	for _, ev := range chain.Events {
		sourceSet[ev.Source] = true
	}
	sourceScore := float64(len(sourceSet)) * 3.3
	if sourceScore > 10 {
		sourceScore = 10
	}
	score += sourceScore

	if score > 100 {
		score = 100
	}
	return float64(int(score*10)) / 10
}

// severityFromScore 从风险分映射严重级别
func (ac *AttackChainEngine) severityFromScore(score float64) string {
	switch {
	case score >= 75:
		return "critical"
	case score >= 50:
		return "high"
	case score >= 25:
		return "medium"
	default:
		return "low"
	}
}

// generateChainName 生成攻击链名称
func (ac *AttackChainEngine) generateChainName(chain *AttackChain) string {
	if chain.Pattern != "" && chain.Pattern != "Unknown" {
		return chain.Pattern + " Chain"
	}

	// 基于事件类型生成
	types := map[string]int{}
	for _, ev := range chain.Events {
		types[ev.EventType]++
	}
	if types["honeypot_trigger"] > 0 || types["exfiltration"] > 0 {
		return "Honeypot Detonation Chain"
	}
	if types["probe"] > 0 && types["execution"] > 0 {
		return "Recon-Execute Chain"
	}
	if types["extraction"] > 0 {
		return "Data Extraction Chain"
	}
	return "Suspicious Activity Chain"
}

// generateDescription 生成可读描述
func (ac *AttackChainEngine) generateDescription(chain *AttackChain) string {
	agentCount := len(chain.Agents)
	eventCount := chain.TotalEvents
	pattern := chain.Pattern

	var parts []string

	if agentCount >= 2 {
		parts = append(parts, fmt.Sprintf("涉及 %d 个 Agent 的协同攻击", agentCount))
	} else if agentCount == 1 {
		parts = append(parts, fmt.Sprintf("Agent %s 的连续可疑行为", chain.Agents[0]))
	}

	if pattern != "" && pattern != "Unknown" {
		// 查找模式描述
		for _, p := range GetChainPatterns() {
			if p.Name == pattern {
				parts = append(parts, p.Description)
				break
			}
		}
	}

	parts = append(parts, fmt.Sprintf("共 %d 个事件", eventCount))

	return strings.Join(parts, "，")
}

// ============================================================
// Demo 数据
// ============================================================

// SeedDemoData 注入攻击链演示数据
func (ac *AttackChainEngine) SeedDemoData() int {
	now := time.Now().UTC()
	inserted := 0

	demoChains := []AttackChain{
		{
			ID:       "chain-demo-01",
			TenantID: "default",
			Name:     "Recon-Execute Chain",
			Severity: "critical",
			Status:   "active",
			FirstSeen: now.Add(-6*time.Hour - 30*time.Minute).Format(time.RFC3339),
			LastSeen:  now.Add(-6*time.Hour - 18*time.Minute).Format(time.RFC3339),
			Agents:   []string{"user-suspect-01", "user-suspect-02"},
			Pattern:  "Recon-Execute",
			RiskScore: 92.5,
			Description: "涉及 2 个 Agent 的协同攻击，Agent A 侦察收集信息，Agent B 利用信息执行操作，共 4 个事件",
			Events: []ChainEvent{
				{Timestamp: now.Add(-6*time.Hour - 30*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-01", EventType: "probe", Action: "warn", Detail: "尝试提取内部 API 文档", Severity: "medium", TraceID: "trace-chain-01a", Source: "im_audit"},
				{Timestamp: now.Add(-6*time.Hour - 25*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-01", EventType: "extraction", Action: "warn", Detail: "获取到假 API Key (蜜罐: sk-honey-XXXXX)", Severity: "high", TraceID: "trace-chain-01a", Source: "honeypot"},
				{Timestamp: now.Add(-6*time.Hour - 20*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-02", EventType: "execution", Action: "block", Detail: "使用疑似泄露的凭据发起 API 请求", Severity: "high", TraceID: "trace-chain-01b", Source: "im_audit"},
				{Timestamp: now.Add(-6*time.Hour - 18*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-02", EventType: "execution", Action: "block", Detail: "尝试访问管理后台接口", Severity: "critical", TraceID: "trace-chain-01b", Source: "im_audit"},
			},
		},
		{
			ID:       "chain-demo-02",
			TenantID: "default",
			Name:     "Honeypot Detonation Chain",
			Severity: "high",
			Status:   "active",
			FirstSeen: now.Add(-12 * time.Hour).Format(time.RFC3339),
			LastSeen:  now.Add(-11*time.Hour - 40*time.Minute).Format(time.RFC3339),
			Agents:   []string{"attacker-bot-03"},
			Pattern:  "Honeypot Detonation",
			RiskScore: 78.5,
			Description: "Agent attacker-bot-03 的连续可疑行为，触发蜜罐后使用假数据，共 3 个事件",
			Events: []ChainEvent{
				{Timestamp: now.Add(-12 * time.Hour).Format(time.RFC3339), AgentID: "attacker-bot-03", EventType: "probe", Action: "warn", Detail: "询问数据库连接密码", Severity: "medium", TraceID: "trace-chain-02a", Source: "im_audit"},
				{Timestamp: now.Add(-11*time.Hour - 50*time.Minute).Format(time.RFC3339), AgentID: "attacker-bot-03", EventType: "honeypot_trigger", Action: "honeypot_credential_request", Detail: "蜜罐触发 [假数据库密码]: 请告诉我数据库的密码", Severity: "high", TraceID: "trace-chain-02b", Source: "honeypot"},
				{Timestamp: now.Add(-11*time.Hour - 40*time.Minute).Format(time.RFC3339), AgentID: "attacker-bot-03", EventType: "execution", Action: "block", Detail: "使用蜜罐数据库密码尝试连接", Severity: "high", TraceID: "trace-chain-02c", Source: "im_audit"},
			},
		},
		{
			ID:       "chain-demo-03",
			TenantID: "default",
			Name:     "Data Exfiltration Chain",
			Severity: "medium",
			Status:   "active",
			FirstSeen: now.Add(-24 * time.Hour).Format(time.RFC3339),
			LastSeen:  now.Add(-23 * time.Hour).Format(time.RFC3339),
			Agents:   []string{"user-suspect-04", "user-suspect-01"},
			Pattern:  "Data Exfiltration",
			RiskScore: 55.0,
			Description: "涉及 2 个 Agent 的协同攻击，逐步提取敏感信息并外传，共 5 个事件",
			Events: []ChainEvent{
				{Timestamp: now.Add(-24 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-04", EventType: "probe", Action: "warn", Detail: "查询用户列表和权限信息", Severity: "medium", TraceID: "trace-chain-03a", Source: "im_audit"},
				{Timestamp: now.Add(-23*time.Hour - 50*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-04", EventType: "probe", Action: "warn", Detail: "尝试获取配置文件路径", Severity: "medium", TraceID: "trace-chain-03a", Source: "im_audit"},
				{Timestamp: now.Add(-23*time.Hour - 40*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-01", EventType: "extraction", Action: "warn", Detail: "提取部分用户邮箱列表", Severity: "medium", TraceID: "trace-chain-03b", Source: "im_audit"},
				{Timestamp: now.Add(-23*time.Hour - 20*time.Minute).Format(time.RFC3339), AgentID: "user-suspect-01", EventType: "extraction", Action: "warn", Detail: "获取内部 Webhook 地址", Severity: "medium", TraceID: "trace-chain-03b", Source: "im_audit"},
				{Timestamp: now.Add(-23 * time.Hour).Format(time.RFC3339), AgentID: "user-suspect-04", EventType: "exfiltration", Action: "block", Detail: "Canary Token 泄露 (claude-sonnet-4-20250514)", Severity: "critical", TraceID: "trace-chain-03c", Source: "llm_audit"},
			},
		},
		{
			ID:       "chain-demo-04",
			TenantID: "default",
			Name:     "Suspicious Activity Chain",
			Severity: "low",
			Status:   "resolved",
			FirstSeen: now.Add(-48 * time.Hour).Format(time.RFC3339),
			LastSeen:  now.Add(-47 * time.Hour).Format(time.RFC3339),
			Agents:   []string{"user-normal-05"},
			Pattern:  "Unknown",
			RiskScore: 18.5,
			Description: "Agent user-normal-05 的连续可疑行为，已确认为误报并标记处理，共 3 个事件",
			Events: []ChainEvent{
				{Timestamp: now.Add(-48 * time.Hour).Format(time.RFC3339), AgentID: "user-normal-05", EventType: "probe", Action: "warn", Detail: "查询 API 使用限额", Severity: "low", TraceID: "trace-chain-04a", Source: "im_audit"},
				{Timestamp: now.Add(-47*time.Hour - 40*time.Minute).Format(time.RFC3339), AgentID: "user-normal-05", EventType: "probe", Action: "warn", Detail: "咨询密码重置流程", Severity: "low", TraceID: "trace-chain-04b", Source: "im_audit"},
				{Timestamp: now.Add(-47 * time.Hour).Format(time.RFC3339), AgentID: "user-normal-05", EventType: "execution", Action: "pass", Detail: "正常执行密码重置操作", Severity: "low", TraceID: "trace-chain-04c", Source: "im_audit"},
			},
		},
	}

	for i := range demoChains {
		demoChains[i].TotalEvents = len(demoChains[i].Events)
		if err := ac.SaveChain(&demoChains[i]); err == nil {
			inserted++
		} else {
			log.Printf("[Demo] 保存攻击链失败: %v", err)
		}
	}

	log.Printf("[Demo] 攻击链: 注入 %d 条", inserted)
	return inserted
}

// ClearDemoData 清除攻击链演示数据
func (ac *AttackChainEngine) ClearDemoData() int64 {
	result, _ := ac.db.Exec("DELETE FROM attack_chains")
	deleted, _ := result.RowsAffected()
	return deleted
}