package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RuleSuggestion 规则建议
type RuleSuggestion struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Source       string    `json:"source"`        // "evolution" | "redteam" | "manual"
	SourceDetail string    `json:"source_detail"` // e.g. "gen-5-synonym_replace" or "redteam-run-xxx"
	RuleName     string    `json:"rule_name"`
	RuleType     string    `json:"rule_type"`  // "keyword" | "regex"
	Patterns     []string  `json:"patterns"`
	Action       string    `json:"action"`     // suggested action: "block" | "warn" | "review"
	Category     string    `json:"category"`
	Engine       string    `json:"engine"`     // "inbound" | "llm" | "outbound"
	Reason       string    `json:"reason"`     // why this rule is suggested
	Status       string    `json:"status"`     // "pending" | "accepted" | "rejected"
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy   string    `json:"reviewed_by,omitempty"`
	RejectReason string    `json:"reject_reason,omitempty"`
}

// SuggestionQueue 规则建议队列
type SuggestionQueue struct {
	db         *sql.DB
	ruleEngine *RuleEngine
	mu         sync.RWMutex
}

// NewSuggestionQueue 创建建议队列
func NewSuggestionQueue(db *sql.DB, ruleEngine *RuleEngine) *SuggestionQueue {
	sq := &SuggestionQueue{
		db:         db,
		ruleEngine: ruleEngine,
	}
	sq.initSchema()
	return sq
}

func (sq *SuggestionQueue) initSchema() {
	if sq.db == nil {
		return
	}
	sq.db.Exec(`CREATE TABLE IF NOT EXISTS rule_suggestions (
		id TEXT PRIMARY KEY,
		created_at TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'evolution',
		source_detail TEXT DEFAULT '',
		rule_name TEXT NOT NULL,
		rule_type TEXT NOT NULL DEFAULT 'keyword',
		patterns TEXT NOT NULL DEFAULT '[]',
		action TEXT NOT NULL DEFAULT 'block',
		category TEXT NOT NULL DEFAULT 'prompt_injection',
		engine TEXT NOT NULL DEFAULT 'inbound',
		reason TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		reviewed_at TEXT,
		reviewed_by TEXT DEFAULT '',
		reject_reason TEXT DEFAULT ''
	)`)
	// 索引
	sq.db.Exec(`CREATE INDEX IF NOT EXISTS idx_suggestion_status ON rule_suggestions(status)`)
	sq.db.Exec(`CREATE INDEX IF NOT EXISTS idx_suggestion_created ON rule_suggestions(created_at)`)
}

// Add 添加一条规则建议
func (sq *SuggestionQueue) Add(s RuleSuggestion) error {
	if sq.db == nil {
		return fmt.Errorf("database not available")
	}
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if s.Status == "" {
		s.Status = "pending"
	}

	patternsJSON, _ := json.Marshal(s.Patterns)
	_, err := sq.db.Exec(`INSERT INTO rule_suggestions 
		(id, created_at, source, source_detail, rule_name, rule_type, patterns, action, category, engine, reason, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.CreatedAt.Format(time.RFC3339), s.Source, s.SourceDetail,
		s.RuleName, s.RuleType, string(patternsJSON), s.Action, s.Category, s.Engine, s.Reason, s.Status)
	if err != nil {
		return fmt.Errorf("insert suggestion: %w", err)
	}
	log.Printf("[建议队列] 新增规则建议: %s (%s, %d patterns, source=%s)", s.RuleName, s.RuleType, len(s.Patterns), s.Source)
	return nil
}

// List 列出建议（支持状态过滤）
func (sq *SuggestionQueue) List(status string, limit int) ([]RuleSuggestion, error) {
	if sq.db == nil {
		return nil, nil
	}
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = sq.db.Query(`SELECT id, created_at, source, source_detail, rule_name, rule_type, patterns, action, category, engine, reason, status, reviewed_at, reviewed_by, reject_reason FROM rule_suggestions WHERE status = ? ORDER BY created_at DESC LIMIT ?`, status, limit)
	} else {
		rows, err = sq.db.Query(`SELECT id, created_at, source, source_detail, rule_name, rule_type, patterns, action, category, engine, reason, status, reviewed_at, reviewed_by, reject_reason FROM rule_suggestions ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RuleSuggestion
	for rows.Next() {
		var s RuleSuggestion
		var createdStr, patternsStr string
		var reviewedStr sql.NullString
		err := rows.Scan(&s.ID, &createdStr, &s.Source, &s.SourceDetail, &s.RuleName, &s.RuleType, &patternsStr, &s.Action, &s.Category, &s.Engine, &s.Reason, &s.Status, &reviewedStr, &s.ReviewedBy, &s.RejectReason)
		if err != nil {
			continue
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		json.Unmarshal([]byte(patternsStr), &s.Patterns)
		if reviewedStr.Valid && reviewedStr.String != "" {
			t, _ := time.Parse(time.RFC3339, reviewedStr.String)
			s.ReviewedAt = &t
		}
		results = append(results, s)
	}
	return results, nil
}

// Accept 接受建议 → 热更新到规则引擎
func (sq *SuggestionQueue) Accept(id string, reviewedBy string) error {
	if sq.db == nil {
		return fmt.Errorf("database not available")
	}
	sq.mu.Lock()
	defer sq.mu.Unlock()

	// 查询建议
	var s RuleSuggestion
	var patternsStr, statusStr string
	err := sq.db.QueryRow(`SELECT id, rule_name, rule_type, patterns, action, category, engine, status FROM rule_suggestions WHERE id = ?`, id).
		Scan(&s.ID, &s.RuleName, &s.RuleType, &patternsStr, &s.Action, &s.Category, &s.Engine, &statusStr)
	if err != nil {
		return fmt.Errorf("suggestion not found: %w", err)
	}
	if statusStr != "pending" {
		return fmt.Errorf("suggestion already %s", statusStr)
	}
	json.Unmarshal([]byte(patternsStr), &s.Patterns)

	// 应用到规则引擎
	if s.Engine == "inbound" && sq.ruleEngine != nil {
		configs := sq.ruleEngine.GetRuleConfigs()
		newRule := InboundRuleConfig{
			Name:     s.RuleName,
			Patterns: s.Patterns,
			Action:   s.Action,
			Category: s.Category,
			Priority: 0,
			Type:     s.RuleType,
			Group:    "suggestion-accepted",
		}
		configs = append(configs, newRule)
		sq.ruleEngine.Reload(configs, "suggestion-accept")
		log.Printf("[建议队列] 接受规则: %s → 热更新到入站引擎 (%d patterns)", s.RuleName, len(s.Patterns))
	}

	// 更新状态
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = sq.db.Exec(`UPDATE rule_suggestions SET status = 'accepted', reviewed_at = ?, reviewed_by = ? WHERE id = ?`, now, reviewedBy, id)
	return err
}

// Reject 拒绝建议
func (sq *SuggestionQueue) Reject(id string, reviewedBy string, reason string) error {
	if sq.db == nil {
		return fmt.Errorf("database not available")
	}
	sq.mu.Lock()
	defer sq.mu.Unlock()

	var statusStr string
	err := sq.db.QueryRow(`SELECT status FROM rule_suggestions WHERE id = ?`, id).Scan(&statusStr)
	if err != nil {
		return fmt.Errorf("suggestion not found: %w", err)
	}
	if statusStr != "pending" {
		return fmt.Errorf("suggestion already %s", statusStr)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = sq.db.Exec(`UPDATE rule_suggestions SET status = 'rejected', reviewed_at = ?, reviewed_by = ?, reject_reason = ? WHERE id = ?`, now, reviewedBy, reason, id)
	if err == nil {
		log.Printf("[建议队列] 拒绝规则: %s (reason: %s)", id, reason)
	}
	return err
}

// Stats 统计
func (sq *SuggestionQueue) Stats() map[string]int {
	stats := map[string]int{"pending": 0, "accepted": 0, "rejected": 0, "total": 0}
	if sq.db == nil {
		return stats
	}
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	rows, err := sq.db.Query(`SELECT status, COUNT(*) FROM rule_suggestions GROUP BY status`)
	if err != nil {
		return stats
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		stats[status] = count
		stats["total"] += count
	}
	return stats
}
