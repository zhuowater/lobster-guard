// taint_tracker.go — 信息流污染追踪引擎（Taint Propagation）
// lobster-guard v20.1
// 数据从 IM 入站 → LLM 处理 → 出站响应，
// 如果入站消息含敏感信息（PII/凭据/机密），通过 trace_id 追踪污染标签的传播，实现血统级阻断。
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 污染标签类型
// ============================================================

const (
	TaintPII          = "PII-TAINTED"        // 个人身份信息
	TaintCredential   = "CREDENTIAL-TAINTED" // 凭据/密码/密钥
	TaintConfidential = "CONFIDENTIAL"       // 机密/内部文件
	TaintInternalOnly = "INTERNAL-ONLY"      // 仅限内部
	TaintDataQuery    = "DATA-QUERY-TAINTED" // 数据查询（tool_calls 推断）
)

// ============================================================
// PII 检测模式（中国 + 国际，>=10 种）
// ============================================================

type piiPatternEntry struct {
	Name    string
	Pattern *regexp.Regexp
	Label   string // 命中后映射到的 TaintLabel
}

// piiPatterns 全局 PII 模式表（编译一次，复用）
var piiPatterns = []piiPatternEntry{
	// --- PII-TAINTED ---
	{Name: "phone_cn", Pattern: regexp.MustCompile(`1[3-9]\d{9}`), Label: TaintPII},
	{Name: "id_card_cn", Pattern: regexp.MustCompile(`[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`), Label: TaintPII},
	{Name: "bank_card", Pattern: regexp.MustCompile(`[3-6]\d{15,18}`), Label: TaintPII},
	{Name: "email", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), Label: TaintPII},
	{Name: "ssn_us", Pattern: regexp.MustCompile(`\d{3}-\d{2}-\d{4}`), Label: TaintPII},
	{Name: "credit_card", Pattern: regexp.MustCompile(`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`), Label: TaintPII},
	{Name: "ip_address", Pattern: regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`), Label: TaintPII},
	// --- CREDENTIAL-TAINTED ---
	{Name: "private_key", Pattern: regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`), Label: TaintCredential},
	{Name: "api_key", Pattern: regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}|AKIA[A-Z0-9]{16})`), Label: TaintCredential},
	{Name: "password_leak", Pattern: regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`), Label: TaintCredential},
	// --- 额外模式（>10） ---
	{Name: "passport_cn", Pattern: regexp.MustCompile(`E\d{8}`), Label: TaintPII},
	{Name: "jwt_token", Pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), Label: TaintCredential},
}

// ============================================================
// TaintConfig 配置
// ============================================================

// TaintConfig 污染追踪配置
type TaintConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Action     string `yaml:"action" json:"action"`           // 出站遇到污染标签的动作 (block/warn/log)
	TTLMinutes int    `yaml:"ttl_minutes" json:"ttl_minutes"` // 污染标签存活时间（默认30）
}

// ============================================================
// TaintEntry / TaintPropagation / TaintDecision
// ============================================================

// TaintEntry 污染条目
type TaintEntry struct {
	TraceID      string             `json:"trace_id"`
	Labels       []string           `json:"labels"`        // 污染标签列表
	Source       string             `json:"source"`        // 污染源（inbound/llm/toolcall）
	SourceDetail string             `json:"source_detail"` // 详情（匹配到的 PII 类型）
	Timestamp    time.Time          `json:"timestamp"`
	Propagations []TaintPropagation `json:"propagations"` // 传播历史
}

// TaintPropagation 传播记录
type TaintPropagation struct {
	Stage     string    `json:"stage"`     // inbound → llm_request → llm_response → outbound
	Label     string    `json:"label"`
	Action    string    `json:"action"`    // 此阶段的处理动作
	Timestamp time.Time `json:"timestamp"`
	Detail    string    `json:"detail"`
}

// TaintDecision 出站决策
type TaintDecision struct {
	Tainted bool     `json:"tainted"`
	Labels  []string `json:"labels"`
	Action  string   `json:"action"` // block / warn / pass
	Reason  string   `json:"reason"`
	TraceID string   `json:"trace_id"`
}

// ============================================================
// TaintTracker 污染追踪引擎
// ============================================================

// TaintTracker 污染追踪引擎
type TaintTracker struct {
	db     *sql.DB
	mu     sync.RWMutex
	config TaintConfig
	// 内存中活跃污染标签缓存：trace_id → TaintEntry
	active map[string]*TaintEntry
	// 统计
	totalMarked  int64
	totalBlocked int64
	totalWarned  int64
	// 关闭
	stopCh chan struct{}
}

// NewTaintTracker 创建污染追踪引擎
func NewTaintTracker(db *sql.DB, cfg TaintConfig) *TaintTracker {
	if cfg.TTLMinutes <= 0 {
		cfg.TTLMinutes = 30
	}
	if cfg.Action == "" {
		cfg.Action = "block"
	}

	tt := &TaintTracker{
		db:     db,
		config: cfg,
		active: make(map[string]*TaintEntry),
		stopCh: make(chan struct{}),
	}

	// 初始化数据库表
	tt.initDB()

	// 加载活跃条目到内存
	tt.loadActive()

	// 启动 TTL 清理 goroutine
	go tt.cleanupLoop()

	return tt
}

// initDB 创建数据库表和索引
func (tt *TaintTracker) initDB() {
	if tt.db == nil {
		return
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS taint_entries (
			trace_id TEXT PRIMARY KEY,
			labels_json TEXT NOT NULL,
			source TEXT NOT NULL,
			source_detail TEXT DEFAULT '',
			timestamp TEXT NOT NULL,
			propagations_json TEXT DEFAULT '[]',
			expired INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_taint_ts ON taint_entries(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_taint_expired ON taint_entries(expired)`,
	}
	for _, s := range stmts {
		if _, err := tt.db.Exec(s); err != nil {
			log.Printf("[TaintTracker] 初始化表失败: %v (sql: %s)", err, s)
		}
	}
}

// loadActive 从 SQLite 加载未过期的条目到内存缓存
func (tt *TaintTracker) loadActive() {
	if tt.db == nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(tt.config.TTLMinutes) * time.Minute).Format(time.RFC3339)
	rows, err := tt.db.Query(
		`SELECT trace_id, labels_json, source, source_detail, timestamp, propagations_json
		 FROM taint_entries WHERE expired = 0 AND timestamp > ?`, cutoff)
	if err != nil {
		log.Printf("[TaintTracker] 加载活跃条目失败: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var traceID, labelsJSON, source, sourceDetail, tsStr, propJSON string
		if err := rows.Scan(&traceID, &labelsJSON, &source, &sourceDetail, &tsStr, &propJSON); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, tsStr)
		var labels []string
		json.Unmarshal([]byte(labelsJSON), &labels)
		var props []TaintPropagation
		json.Unmarshal([]byte(propJSON), &props)

		tt.active[traceID] = &TaintEntry{
			TraceID:      traceID,
			Labels:       labels,
			Source:       source,
			SourceDetail: sourceDetail,
			Timestamp:    ts,
			Propagations: props,
		}
		count++
	}
	if count > 0 {
		log.Printf("[TaintTracker] 已加载 %d 条活跃污染条目", count)
	}

	// Restore totalMarked from DB (all entries, not just active)
	var totalHistoric int64
	if tt.db.QueryRow("SELECT COUNT(*) FROM taint_entries").Scan(&totalHistoric) == nil && totalHistoric > 0 {
		tt.totalMarked = totalHistoric
	}
}

// ============================================================
// 核心方法
// ============================================================

// ScanPII 扫描文本，返回匹配到的 PII 类型名称和标签
func ScanPII(text string) (matchedNames []string, labels []string) {
	labelSet := make(map[string]bool)
	for _, entry := range piiPatterns {
		if entry.Pattern.MatchString(text) {
			matchedNames = append(matchedNames, entry.Name)
			labelSet[entry.Label] = true
		}
	}
	for label := range labelSet {
		labels = append(labels, label)
	}
	return
}

// MarkTainted 扫描 text 匹配 PII 模式，标记污染
// 返回 TaintEntry（没有匹配到任何 PII 则返回 nil）
func (tt *TaintTracker) MarkTainted(traceID string, text string, source string) *TaintEntry {
	if !tt.config.Enabled || traceID == "" || text == "" {
		return nil
	}

	matchedNames, labels := ScanPII(text)
	if len(labels) == 0 {
		return nil
	}

	now := time.Now()
	entry := &TaintEntry{
		TraceID:      traceID,
		Labels:       labels,
		Source:       source,
		SourceDetail: strings.Join(matchedNames, ","),
		Timestamp:    now,
		Propagations: []TaintPropagation{
			{
				Stage:     "inbound",
				Label:     strings.Join(labels, ","),
				Action:    "marked",
				Timestamp: now,
				Detail:    fmt.Sprintf("PII detected: %s", strings.Join(matchedNames, ", ")),
			},
		},
	}

	tt.mu.Lock()
	tt.active[traceID] = entry
	tt.totalMarked++
	tt.mu.Unlock()

	// 异步持久化到 SQLite
	go tt.persistEntry(entry)

	return entry
}

// Propagate 给已有 trace 添加传播记录
func (tt *TaintTracker) Propagate(traceID string, stage string, detail string) {
	if !tt.config.Enabled || traceID == "" {
		return
	}

	tt.mu.Lock()
	defer tt.mu.Unlock()

	entry, exists := tt.active[traceID]
	if !exists {
		return
	}

	prop := TaintPropagation{
		Stage:     stage,
		Label:     strings.Join(entry.Labels, ","),
		Action:    "propagated",
		Timestamp: time.Now(),
		Detail:    detail,
	}
	entry.Propagations = append(entry.Propagations, prop)

	// 异步更新 SQLite
	go tt.updatePropagations(traceID, entry.Propagations)
}

// CheckOutbound 检查 trace_id 是否有活跃污染标签
func (tt *TaintTracker) CheckOutbound(traceID string) *TaintDecision {
	if !tt.config.Enabled || traceID == "" {
		return &TaintDecision{
			Tainted: false,
			Action:  "pass",
			TraceID: traceID,
		}
	}

	tt.mu.RLock()
	entry, exists := tt.active[traceID]
	tt.mu.RUnlock()

	if !exists {
		return &TaintDecision{
			Tainted: false,
			Action:  "pass",
			TraceID: traceID,
		}
	}

	// 检查 TTL
	if time.Since(entry.Timestamp) > time.Duration(tt.config.TTLMinutes)*time.Minute {
		// 已过期
		return &TaintDecision{
			Tainted: false,
			Action:  "pass",
			TraceID: traceID,
			Reason:  "taint expired",
		}
	}

	// 有活跃污染标签
	action := tt.config.Action
	if action == "" {
		action = "block"
	}

	// 记录传播
	tt.Propagate(traceID, "outbound", fmt.Sprintf("outbound check: action=%s", action))

	// 更新统计
	tt.mu.Lock()
	switch action {
	case "block":
		tt.totalBlocked++
	case "warn":
		tt.totalWarned++
	}
	tt.mu.Unlock()

	return &TaintDecision{
		Tainted: true,
		Labels:  entry.Labels,
		Action:  action,
		Reason:  fmt.Sprintf("tainted by %s: %s", entry.Source, entry.SourceDetail),
		TraceID: traceID,
	}
}

// ListTainted 返回活跃污染列表
func (tt *TaintTracker) ListTainted(limit int) []TaintEntry {
	if limit <= 0 {
		limit = 100
	}

	tt.mu.RLock()
	defer tt.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(tt.config.TTLMinutes) * time.Minute)
	var result []TaintEntry
	for _, entry := range tt.active {
		if entry.Timestamp.After(cutoff) {
			result = append(result, *entry)
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetTaint 查看特定 trace 的污染信息
func (tt *TaintTracker) GetTaint(traceID string) *TaintEntry {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	if entry, exists := tt.active[traceID]; exists {
		return entry
	}

	// 尝试从 SQLite 查找
	if tt.db == nil {
		return nil
	}
	var labelsJSON, source, sourceDetail, tsStr, propJSON string
	err := tt.db.QueryRow(
		`SELECT labels_json, source, source_detail, timestamp, propagations_json
		 FROM taint_entries WHERE trace_id = ?`, traceID).
		Scan(&labelsJSON, &source, &sourceDetail, &tsStr, &propJSON)
	if err != nil {
		return nil
	}
	ts, _ := time.Parse(time.RFC3339, tsStr)
	var labels []string
	json.Unmarshal([]byte(labelsJSON), &labels)
	var props []TaintPropagation
	json.Unmarshal([]byte(propJSON), &props)

	return &TaintEntry{
		TraceID:      traceID,
		Labels:       labels,
		Source:       source,
		SourceDetail: sourceDetail,
		Timestamp:    ts,
		Propagations: props,
	}
}

// Stats 返回污染追踪统计
func (tt *TaintTracker) Stats() map[string]interface{} {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(tt.config.TTLMinutes) * time.Minute)

	// 标签分布
	labelDist := make(map[string]int)
	activeCount := 0
	for _, entry := range tt.active {
		if entry.Timestamp.After(cutoff) {
			activeCount++
			for _, label := range entry.Labels {
				labelDist[label]++
			}
		}
	}

	return map[string]interface{}{
		"enabled":        tt.config.Enabled,
		"action":         tt.config.Action,
		"ttl_minutes":    tt.config.TTLMinutes,
		"active_count":   activeCount,
		"total_marked":   tt.totalMarked,
		"total_blocked":  tt.totalBlocked,
		"total_warned":   tt.totalWarned,
		"label_distribution": labelDist,
	}
}

// GetConfig 返回当前配置
func (tt *TaintTracker) GetConfig() TaintConfig {
	tt.mu.RLock()
	defer tt.mu.RUnlock()
	return tt.config
}

// UpdateConfig 更新配置
func (tt *TaintTracker) UpdateConfig(cfg TaintConfig) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if cfg.TTLMinutes > 0 {
		tt.config.TTLMinutes = cfg.TTLMinutes
	}
	if cfg.Action != "" {
		tt.config.Action = cfg.Action
	}
	tt.config.Enabled = cfg.Enabled
}

// Stop 停止清理 goroutine
func (tt *TaintTracker) Stop() {
	close(tt.stopCh)
}

// ============================================================
// SQLite 持久化
// ============================================================

func (tt *TaintTracker) persistEntry(entry *TaintEntry) {
	if tt.db == nil {
		return
	}
	labelsJSON, _ := json.Marshal(entry.Labels)
	propJSON, _ := json.Marshal(entry.Propagations)
	_, err := tt.db.Exec(
		`INSERT OR REPLACE INTO taint_entries (trace_id, labels_json, source, source_detail, timestamp, propagations_json, expired)
		 VALUES (?, ?, ?, ?, ?, ?, 0)`,
		entry.TraceID,
		string(labelsJSON),
		entry.Source,
		entry.SourceDetail,
		entry.Timestamp.Format(time.RFC3339),
		string(propJSON),
	)
	if err != nil {
		log.Printf("[TaintTracker] 持久化失败: trace_id=%s error=%v", entry.TraceID, err)
	}
}

func (tt *TaintTracker) updatePropagations(traceID string, props []TaintPropagation) {
	if tt.db == nil {
		return
	}
	propJSON, _ := json.Marshal(props)
	_, err := tt.db.Exec(
		`UPDATE taint_entries SET propagations_json = ? WHERE trace_id = ?`,
		string(propJSON), traceID)
	if err != nil {
		log.Printf("[TaintTracker] 更新传播记录失败: trace_id=%s error=%v", traceID, err)
	}
}

// ============================================================
// TTL 清理
// ============================================================

// cleanupLoop 定时清理过期标签
func (tt *TaintTracker) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-tt.stopCh:
			return
		case <-ticker.C:
			tt.cleanup()
		}
	}
}

// cleanup 清理过期标签
func (tt *TaintTracker) cleanup() {
	tt.mu.Lock()
	cutoff := time.Now().Add(-time.Duration(tt.config.TTLMinutes) * time.Minute)
	var expired []string
	for traceID, entry := range tt.active {
		if entry.Timestamp.Before(cutoff) {
			expired = append(expired, traceID)
		}
	}
	for _, traceID := range expired {
		delete(tt.active, traceID)
	}
	tt.mu.Unlock()

	// 标记数据库中的过期条目
	if tt.db != nil && len(expired) > 0 {
		for _, traceID := range expired {
			tt.db.Exec(`UPDATE taint_entries SET expired = 1 WHERE trace_id = ?`, traceID)
		}
	}
}

// CleanupNow 立即执行清理（测试用）
func (tt *TaintTracker) CleanupNow() {
	tt.cleanup()
}

// DeleteEntry 删除单条污染标记
func (tt *TaintTracker) DeleteEntry(traceID string) {
	tt.mu.Lock()
	delete(tt.active, traceID)
	tt.mu.Unlock()
	if tt.db != nil {
		tt.db.Exec(`DELETE FROM taint_entries WHERE trace_id = ?`, traceID)
	}
}

// InjectManual 手动注入污染标记（管理 API / 测试）
func (tt *TaintTracker) InjectManual(traceID string, labels []string, source, detail string) {
	entry := &TaintEntry{
		TraceID:      traceID,
		Labels:       labels,
		Source:       source,
		SourceDetail: detail,
		Timestamp:    time.Now(),
		Propagations: []TaintPropagation{
			{Stage: "manual_inject", Label: strings.Join(labels, ","), Action: "inject", Timestamp: time.Now(), Detail: detail},
		},
	}
	tt.mu.Lock()
	tt.active[traceID] = entry
	tt.totalMarked++
	tt.mu.Unlock()
	tt.persistEntry(entry)
}
