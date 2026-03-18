// event_bus.go — 事件总线核心：安全事件推送 + Webhook + 动作链
// lobster-guard v18.1
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 安全事件格式
// ============================================================

// SecurityEvent 统一安全事件格式
type SecurityEvent struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`      // "inbound_block" / "outbound_block" / "llm_block" / "canary_leaked" / "honeypot_triggered" / "budget_exceeded" / "anomaly_detected" / "redteam_completed"
	Severity  string                 `json:"severity"`  // "info" / "low" / "medium" / "high" / "critical"
	Domain    string                 `json:"domain"`    // "inbound" / "outbound" / "llm" / "system"
	TraceID   string                 `json:"trace_id"`
	TenantID  string                 `json:"tenant_id"`
	SenderID  string                 `json:"sender_id"`
	Summary   string                 `json:"summary"`   // 人类可读摘要
	Details   map[string]interface{} `json:"details"`   // 详细数据
}

// ============================================================
// 推送目标
// ============================================================

// WebhookTarget 推送目标
type WebhookTarget struct {
	ID      string            `json:"id" yaml:"id"`
	Name    string            `json:"name" yaml:"name"`
	URL     string            `json:"url" yaml:"url"`
	Method  string            `json:"method" yaml:"method"`   // POST (default)
	Headers map[string]string `json:"headers" yaml:"headers"` // 自定义 Header
	Secret  string            `json:"secret" yaml:"secret"`   // Webhook 签名密钥
	Enabled bool              `json:"enabled" yaml:"enabled"`
	// 过滤器
	MinSeverity string   `json:"min_severity" yaml:"min_severity"` // 最低严重级别
	EventTypes  []string `json:"event_types" yaml:"event_types"`   // 只推送特定类型事件（空=全部）
	TenantIDs   []string `json:"tenant_ids" yaml:"tenant_ids"`     // 只推送特定租户（空=全部）
}

// ============================================================
// 动作链
// ============================================================

// ActionChainStep 动作链步骤
type ActionChainStep struct {
	Type   string            `json:"type" yaml:"type"`     // "webhook" / "log" / "ban_user" / "enable_strict_mode"
	Config map[string]string `json:"config" yaml:"config"` // 步骤参数
}

// ActionChain 动作链
type ActionChain struct {
	ID      string            `json:"id" yaml:"id"`
	Name    string            `json:"name" yaml:"name"`
	Trigger ActionChainTrigger `json:"trigger" yaml:"trigger"` // 触发条件
	Steps   []ActionChainStep `json:"steps" yaml:"steps"`
	Enabled bool              `json:"enabled" yaml:"enabled"`
}

// ActionChainTrigger 触发条件
type ActionChainTrigger struct {
	EventType   string `json:"event_type" yaml:"event_type"`     // 事件类型（空=全部）
	MinSeverity string `json:"min_severity" yaml:"min_severity"` // 最低级别
}

// ============================================================
// 事件总线
// ============================================================

// EventBus 事件总线
type EventBus struct {
	db         *sql.DB
	targets    []WebhookTarget
	chains     []ActionChain
	mu         sync.RWMutex
	httpClient *http.Client
	eventChan  chan *SecurityEvent // 异步事件队列
	stats      EventBusStats
	dropped    int64 // 丢弃事件计数（通道满时）
	stopCh     chan struct{}
	wg         sync.WaitGroup
	// 内部引用（用于执行 ban_user / enable_strict_mode）
	cfg            *Config
	strictModeFunc func(bool) error // 可选的严格模式回调
	banUserFunc    func(string)     // 可选的封禁用户回调
}

// EventBusStats 事件总线统计
type EventBusStats struct {
	TotalEvents      int64 `json:"total_events"`
	TotalDelivered   int64 `json:"total_delivered"`
	TotalFailed      int64 `json:"total_failed"`
	TotalChainsFired int64 `json:"total_chains_fired"`
	TotalDropped     int64 `json:"total_dropped"`
}

// ============================================================
// 严重级别工具
// ============================================================

// severityLevel 严重级别映射到数字（用于比较）
func severityLevel(s string) int {
	switch strings.ToLower(s) {
	case "info":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return -1
	}
}

// ============================================================
// 构造 & 生命周期
// ============================================================

// NewEventBus 创建事件总线
func NewEventBus(db *sql.DB, cfg *Config) *EventBus {
	eb := &EventBus{
		db:        db,
		targets:   cfg.EventBus.Targets,
		chains:    cfg.EventBus.Chains,
		eventChan: make(chan *SecurityEvent, 1000),
		stopCh:    make(chan struct{}),
		cfg:       cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	// 初始化 SQLite 表
	eb.initTables()
	// 加载推送目标和动作链
	if eb.targets == nil {
		eb.targets = []WebhookTarget{}
	}
	if eb.chains == nil {
		eb.chains = []ActionChain{}
	}
	// 启动异步投递 goroutine
	eb.wg.Add(1)
	go eb.processLoop()
	return eb
}

// initTables 初始化数据库表
func (eb *EventBus) initTables() {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS security_events (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL,
			type TEXT NOT NULL,
			severity TEXT NOT NULL,
			domain TEXT NOT NULL,
			trace_id TEXT DEFAULT '',
			tenant_id TEXT DEFAULT '',
			sender_id TEXT DEFAULT '',
			summary TEXT NOT NULL,
			details_json TEXT DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts ON security_events(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON security_events(type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_severity ON security_events(severity)`,
		`CREATE TABLE IF NOT EXISTS event_deliveries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			status TEXT NOT NULL,
			response_code INTEGER DEFAULT 0,
			error_msg TEXT DEFAULT '',
			delivered_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := eb.db.Exec(stmt); err != nil {
			log.Printf("[事件总线] 初始化表失败: %v (SQL: %s)", err, stmt[:60])
		}
	}
}

// Stop 停止事件总线
func (eb *EventBus) Stop() {
	close(eb.stopCh)
	eb.wg.Wait()
}

// ============================================================
// 事件发射
// ============================================================

// Emit 发射事件（非阻塞）
func (eb *EventBus) Emit(event *SecurityEvent) {
	if event == nil {
		return
	}
	// 设置默认值
	if event.ID == "" {
		event.ID = GenerateTraceID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Details == nil {
		event.Details = map[string]interface{}{}
	}
	// 写入数据库
	eb.persistEvent(event)
	// 发到异步通道（非阻塞）
	select {
	case eb.eventChan <- event:
		atomic.AddInt64(&eb.stats.TotalEvents, 1)
	default:
		// 通道满，丢弃
		atomic.AddInt64(&eb.dropped, 1)
		atomic.AddInt64(&eb.stats.TotalDropped, 1)
		log.Printf("[事件总线] ⚠️ 事件通道已满，丢弃事件: type=%s id=%s", event.Type, event.ID)
	}
}

// persistEvent 写入数据库
func (eb *EventBus) persistEvent(event *SecurityEvent) {
	detailsJSON, _ := json.Marshal(event.Details)
	_, err := eb.db.Exec(
		`INSERT OR IGNORE INTO security_events (id, timestamp, type, severity, domain, trace_id, tenant_id, sender_id, summary, details_json) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		event.ID,
		event.Timestamp.Format(time.RFC3339),
		event.Type,
		event.Severity,
		event.Domain,
		event.TraceID,
		event.TenantID,
		event.SenderID,
		event.Summary,
		string(detailsJSON),
	)
	if err != nil {
		log.Printf("[事件总线] 写入事件失败: %v", err)
	}
}

// ============================================================
// 异步处理
// ============================================================

// processLoop 异步处理事件
func (eb *EventBus) processLoop() {
	defer eb.wg.Done()
	for {
		select {
		case event := <-eb.eventChan:
			eb.processEvent(event)
		case <-eb.stopCh:
			// 处理剩余事件
			for {
				select {
				case event := <-eb.eventChan:
					eb.processEvent(event)
				default:
					return
				}
			}
		}
	}
}

// processEvent 处理单个事件
func (eb *EventBus) processEvent(event *SecurityEvent) {
	eb.mu.RLock()
	targets := make([]WebhookTarget, len(eb.targets))
	copy(targets, eb.targets)
	chains := make([]ActionChain, len(eb.chains))
	copy(chains, eb.chains)
	eb.mu.RUnlock()

	// 遍历所有 targets，匹配过滤器后推送
	for i := range targets {
		if !targets[i].Enabled {
			continue
		}
		if !eb.matchesTarget(&targets[i], event) {
			continue
		}
		if err := eb.deliverWebhook(&targets[i], event); err != nil {
			log.Printf("[事件总线] 推送失败: target=%s event=%s err=%v", targets[i].ID, event.ID, err)
		}
	}

	// 遍历所有 chains，匹配触发条件后执行
	for i := range chains {
		if !chains[i].Enabled {
			continue
		}
		if !eb.matchesTrigger(&chains[i].Trigger, event) {
			continue
		}
		eb.executeChain(&chains[i], event)
	}
}

// ============================================================
// 过滤器匹配
// ============================================================

// matchesTarget 检查事件是否匹配推送目标的过滤器
func (eb *EventBus) matchesTarget(target *WebhookTarget, event *SecurityEvent) bool {
	// 严重级别过滤
	if target.MinSeverity != "" {
		targetLevel := severityLevel(target.MinSeverity)
		eventLevel := severityLevel(event.Severity)
		if eventLevel < targetLevel {
			return false
		}
	}
	// 事件类型过滤
	if len(target.EventTypes) > 0 {
		found := false
		for _, t := range target.EventTypes {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// 租户过滤
	if len(target.TenantIDs) > 0 {
		found := false
		for _, t := range target.TenantIDs {
			if t == event.TenantID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// matchesTrigger 检查事件是否匹配动作链触发条件
func (eb *EventBus) matchesTrigger(trigger *ActionChainTrigger, event *SecurityEvent) bool {
	// 事件类型匹配
	if trigger.EventType != "" && trigger.EventType != event.Type {
		return false
	}
	// 严重级别匹配
	if trigger.MinSeverity != "" {
		triggerLevel := severityLevel(trigger.MinSeverity)
		eventLevel := severityLevel(event.Severity)
		if eventLevel < triggerLevel {
			return false
		}
	}
	return true
}

// ============================================================
// Webhook 推送
// ============================================================

// deliverWebhook HTTP POST 推送
func (eb *EventBus) deliverWebhook(target *WebhookTarget, event *SecurityEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	method := target.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, target.URL, strings.NewReader(string(body)))
	if err != nil {
		eb.recordDelivery(event.ID, target.ID, "failed", 0, err.Error())
		atomic.AddInt64(&eb.stats.TotalFailed, 1)
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "lobster-guard/"+AppVersion)

	// 自定义 Header
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	// HMAC 签名
	if target.Secret != "" {
		mac := hmac.New(sha256.New, []byte(target.Secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Lobster-Signature", sig)
	}

	resp, err := eb.httpClient.Do(req)
	if err != nil {
		eb.recordDelivery(event.ID, target.ID, "failed", 0, err.Error())
		atomic.AddInt64(&eb.stats.TotalFailed, 1)
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(io.LimitReader(resp.Body, 4096)) // drain body

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		eb.recordDelivery(event.ID, target.ID, "success", resp.StatusCode, "")
		atomic.AddInt64(&eb.stats.TotalDelivered, 1)
		return nil
	}
	errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
	eb.recordDelivery(event.ID, target.ID, "failed", resp.StatusCode, errMsg)
	atomic.AddInt64(&eb.stats.TotalFailed, 1)
	return fmt.Errorf("HTTP 响应 %d", resp.StatusCode)
}

// recordDelivery 记录投递结果到 event_deliveries 表
func (eb *EventBus) recordDelivery(eventID, targetID, status string, responseCode int, errMsg string) {
	_, err := eb.db.Exec(
		`INSERT INTO event_deliveries (event_id, target_id, status, response_code, error_msg, delivered_at) VALUES (?,?,?,?,?,?)`,
		eventID, targetID, status, responseCode, errMsg, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		log.Printf("[事件总线] 记录投递失败: %v", err)
	}
}

// ============================================================
// 动作链执行
// ============================================================

// executeChain 执行动作链
func (eb *EventBus) executeChain(chain *ActionChain, event *SecurityEvent) {
	atomic.AddInt64(&eb.stats.TotalChainsFired, 1)
	log.Printf("[事件总线] 执行动作链: chain=%s event=%s", chain.ID, event.ID)

	for _, step := range chain.Steps {
		switch step.Type {
		case "webhook":
			// 调用指定的 webhook target
			targetID := step.Config["target_id"]
			if targetID == "" {
				log.Printf("[事件总线] 动作链步骤 webhook 缺少 target_id")
				continue
			}
			target := eb.findTarget(targetID)
			if target == nil {
				log.Printf("[事件总线] 动作链步骤 webhook target 未找到: %s", targetID)
				continue
			}
			if err := eb.deliverWebhook(target, event); err != nil {
				log.Printf("[事件总线] 动作链 webhook 失败: %v", err)
			}

		case "log":
			msg := step.Config["message"]
			if msg == "" {
				msg = fmt.Sprintf("动作链 %s 触发: event=%s type=%s severity=%s", chain.Name, event.ID, event.Type, event.Severity)
			}
			log.Printf("[事件总线-动作链] %s", msg)

		case "ban_user":
			if event.SenderID != "" && eb.banUserFunc != nil {
				eb.banUserFunc(event.SenderID)
				log.Printf("[事件总线-动作链] 封禁用户: %s", event.SenderID)
			}

		case "enable_strict_mode":
			if eb.strictModeFunc != nil {
				if err := eb.strictModeFunc(true); err != nil {
					log.Printf("[事件总线-动作链] 启用严格模式失败: %v", err)
				} else {
					log.Printf("[事件总线-动作链] 已启用严格模式")
				}
			}

		default:
			log.Printf("[事件总线] 未知动作链步骤类型: %s", step.Type)
		}
	}
}

// findTarget 查找推送目标
func (eb *EventBus) findTarget(id string) *WebhookTarget {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for i := range eb.targets {
		if eb.targets[i].ID == id {
			return &eb.targets[i]
		}
	}
	return nil
}

// ============================================================
// 推送目标 CRUD
// ============================================================

// AddTarget 添加推送目标
func (eb *EventBus) AddTarget(target WebhookTarget) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	// 检查 ID 重复
	for _, t := range eb.targets {
		if t.ID == target.ID {
			return fmt.Errorf("推送目标 ID %q 已存在", target.ID)
		}
	}
	if target.ID == "" {
		target.ID = GenerateTraceID()[:12]
	}
	eb.targets = append(eb.targets, target)
	return nil
}

// UpdateTarget 更新推送目标
func (eb *EventBus) UpdateTarget(target WebhookTarget) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for i, t := range eb.targets {
		if t.ID == target.ID {
			eb.targets[i] = target
			return nil
		}
	}
	return fmt.Errorf("推送目标 %q 未找到", target.ID)
}

// DeleteTarget 删除推送目标
func (eb *EventBus) DeleteTarget(id string) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for i, t := range eb.targets {
		if t.ID == id {
			eb.targets = append(eb.targets[:i], eb.targets[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("推送目标 %q 未找到", id)
}

// ListTargets 列出推送目标
func (eb *EventBus) ListTargets() []WebhookTarget {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	result := make([]WebhookTarget, len(eb.targets))
	copy(result, eb.targets)
	return result
}

// ============================================================
// 查询
// ============================================================

// QueryEvents 查询事件列表
func (eb *EventBus) QueryEvents(eventType, severity, since string, limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, timestamp, type, severity, domain, trace_id, tenant_id, sender_id, summary, details_json FROM security_events WHERE 1=1`
	args := []interface{}{}

	if eventType != "" {
		query += ` AND type=?`
		args = append(args, eventType)
	}
	if severity != "" {
		query += ` AND severity=?`
		args = append(args, severity)
	}
	if since != "" {
		query += ` AND timestamp>=?`
		args = append(args, since)
	}
	query += ` ORDER BY timestamp DESC`
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(` LIMIT %d`, limit)

	rows, err := eb.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, ts, typ, sev, domain, traceID, tenantID, senderID, summary, detailsJSON string
		if err := rows.Scan(&id, &ts, &typ, &sev, &domain, &traceID, &tenantID, &senderID, &summary, &detailsJSON); err != nil {
			continue
		}
		var details map[string]interface{}
		json.Unmarshal([]byte(detailsJSON), &details)
		results = append(results, map[string]interface{}{
			"id": id, "timestamp": ts, "type": typ, "severity": sev, "domain": domain,
			"trace_id": traceID, "tenant_id": tenantID, "sender_id": senderID,
			"summary": summary, "details": details,
		})
	}
	return results, nil
}

// QueryDeliveries 查询投递记录
func (eb *EventBus) QueryDeliveries(eventID, targetID, status string, limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, event_id, target_id, status, response_code, error_msg, delivered_at FROM event_deliveries WHERE 1=1`
	args := []interface{}{}

	if eventID != "" {
		query += ` AND event_id=?`
		args = append(args, eventID)
	}
	if targetID != "" {
		query += ` AND target_id=?`
		args = append(args, targetID)
	}
	if status != "" {
		query += ` AND status=?`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC`
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(` LIMIT %d`, limit)

	rows, err := eb.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, responseCode int
		var eid, tid, st, errMsg, deliveredAt string
		if err := rows.Scan(&id, &eid, &tid, &st, &responseCode, &errMsg, &deliveredAt); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"id": id, "event_id": eid, "target_id": tid, "status": st,
			"response_code": responseCode, "error_msg": errMsg, "delivered_at": deliveredAt,
		})
	}
	return results, nil
}

// GetStats 返回统计
func (eb *EventBus) GetStats() EventBusStats {
	return EventBusStats{
		TotalEvents:      atomic.LoadInt64(&eb.stats.TotalEvents),
		TotalDelivered:   atomic.LoadInt64(&eb.stats.TotalDelivered),
		TotalFailed:      atomic.LoadInt64(&eb.stats.TotalFailed),
		TotalChainsFired: atomic.LoadInt64(&eb.stats.TotalChainsFired),
		TotalDropped:     atomic.LoadInt64(&eb.dropped),
	}
}

// ListChains 列出动作链
func (eb *EventBus) ListChains() []ActionChain {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	result := make([]ActionChain, len(eb.chains))
	copy(result, eb.chains)
	return result
}
