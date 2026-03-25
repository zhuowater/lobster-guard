// counterfactual.go — CounterfactualVerifier: 反事实验证引擎
// lobster-guard v24.0
// 基于 AttriGuard (arXiv:2603.10749): 对可疑 tool call 构造对照请求，比对行为差异判断是否注入驱动
package main

import (
	"bytes"
	"context"
	"crypto/rand"
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

// CFConfig 反事实验证配置
type CFConfig struct {
	Enabled       bool    `yaml:"enabled" json:"enabled"`
	Mode          string  `yaml:"mode" json:"mode"`
	MaxPerHour    int     `yaml:"max_per_hour" json:"max_per_hour"`
	RiskThreshold float64 `yaml:"risk_threshold" json:"risk_threshold"`
	CacheTTLSec   int     `yaml:"cache_ttl_sec" json:"cache_ttl_sec"`
	TimeoutSec    int     `yaml:"timeout_sec" json:"timeout_sec"`
	FuzzyMatch    bool    `yaml:"fuzzy_match" json:"fuzzy_match"`
}

// CFVerification 一次验证记录
type CFVerification struct {
	ID               string    `json:"id"`
	TraceID          string    `json:"trace_id"`
	ToolName         string    `json:"tool_name"`
	ToolArgs         string    `json:"tool_args"`
	OriginalMessages string    `json:"original_messages"`
	ControlMessages  string    `json:"control_messages"`
	OriginalResult   string    `json:"original_result"`
	ControlResult    string    `json:"control_result"`
	Survived         bool      `json:"survived"`
	AttributionScore float64   `json:"attribution_score"`
	CausalDriver     string    `json:"causal_driver"`
	Verdict          string    `json:"verdict"`
	Decision         string    `json:"decision"`
	LatencyMs        int64     `json:"latency_ms"`
	Cached           bool      `json:"cached"`
	TenantID         string    `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// CFStats 统计
type CFStats struct {
	TotalVerifications int64   `json:"total_verifications"`
	HourlyUsed         int    `json:"hourly_used"`
	HourlyBudget       int    `json:"hourly_budget"`
	BlockedCount       int64   `json:"blocked_count"`
	AllowedCount       int64   `json:"allowed_count"`
	InconclusiveCount  int64   `json:"inconclusive_count"`
	CacheHitRate       float64 `json:"cache_hit_rate"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	AvgAttribution     float64 `json:"avg_attribution_score"`
}

// CFCacheEntry 缓存条目
type CFCacheEntry struct {
	Survived         bool
	AttributionScore float64
	Verdict          string
	CachedAt         time.Time
}

// CFMessage 用于构造对照请求的 Message 结构
type CFMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolUseID  string      `json:"tool_use_id,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// CounterfactualVerifier 反事实验证引擎
type CounterfactualVerifier struct {
	db         *sql.DB
	mu         sync.RWMutex
	config     CFConfig
	client     *http.Client
	cache      map[string]*CFCacheEntry
	stats      CFStats
	pathPolicy *PathPolicyEngine

	totalVerifications int64
	blockedCount       int64
	allowedCount       int64
	inconclusiveCount  int64
	cacheHits          int64
	cacheMisses        int64
	totalLatencyMs     int64
	totalAttribution   int64 // x1000

	hourlyWindow []time.Time
}

var defaultCFConfig = CFConfig{
	Enabled:       false,
	Mode:          "async",
	MaxPerHour:    100,
	RiskThreshold: 50,
	CacheTTLSec:   300,
	TimeoutSec:    10,
	FuzzyMatch:    true,
}

var highRiskTools = map[string]bool{
	"shell_exec": true, "execute_command": true, "run_command": true,
	"send_email": true, "send_message": true,
	"file_write": true, "write_file": true, "create_file": true,
	"http_request": true, "fetch_url": true,
	"database_query": true, "sql_query": true,
	"delete_file": true, "remove_file": true,
}

// NewCounterfactualVerifier 初始化反事实验证引擎
func NewCounterfactualVerifier(db *sql.DB, config CFConfig, client *http.Client) *CounterfactualVerifier {
	if config.MaxPerHour <= 0 {
		config.MaxPerHour = defaultCFConfig.MaxPerHour
	}
	if config.RiskThreshold <= 0 {
		config.RiskThreshold = defaultCFConfig.RiskThreshold
	}
	if config.CacheTTLSec <= 0 {
		config.CacheTTLSec = defaultCFConfig.CacheTTLSec
	}
	if config.TimeoutSec <= 0 {
		config.TimeoutSec = defaultCFConfig.TimeoutSec
	}
	if config.Mode == "" {
		config.Mode = defaultCFConfig.Mode
	}
	if client == nil {
		client = &http.Client{Timeout: time.Duration(config.TimeoutSec) * time.Second}
	}
	v := &CounterfactualVerifier{
		db:           db,
		config:       config,
		client:       client,
		cache:        make(map[string]*CFCacheEntry),
		hourlyWindow: make([]time.Time, 0, config.MaxPerHour),
	}
	v.stats.HourlyBudget = config.MaxPerHour
	if db != nil {
		v.initDB()
	}
	log.Printf("[Counterfactual] 引擎初始化: enabled=%v mode=%s budget=%d/h threshold=%.0f fuzzy=%v",
		config.Enabled, config.Mode, config.MaxPerHour, config.RiskThreshold, config.FuzzyMatch)
	return v
}

func (v *CounterfactualVerifier) initDB() {
	if v.db == nil {
		return
	}
	v.db.Exec(`CREATE TABLE IF NOT EXISTS cf_verifications (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_args TEXT,
		original_messages TEXT,
		control_messages TEXT,
		original_result TEXT,
		control_result TEXT,
		survived INTEGER NOT NULL DEFAULT 0,
		attribution_score REAL NOT NULL DEFAULT 0,
		causal_driver TEXT,
		verdict TEXT NOT NULL,
		decision TEXT NOT NULL DEFAULT 'allow',
		latency_ms INTEGER,
		cached INTEGER NOT NULL DEFAULT 0,
		tenant_id TEXT NOT NULL DEFAULT 'default',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cf_trace ON cf_verifications(trace_id)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cf_verdict ON cf_verifications(verdict)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cf_created ON cf_verifications(created_at)`)
}

func (v *CounterfactualVerifier) SetPathPolicy(pp *PathPolicyEngine) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pathPolicy = pp
}

// ShouldVerify 判断是否需要对该 tool call 进行反事实验证
func (v *CounterfactualVerifier) ShouldVerify(toolName, toolArgs, traceID string, riskScore float64) bool {
	v.mu.RLock()
	cfg := v.config
	v.mu.RUnlock()

	if !cfg.Enabled {
		return false
	}
	isHighRisk := highRiskTools[toolName]
	aboveThreshold := riskScore >= cfg.RiskThreshold
	if !isHighRisk && !aboveThreshold {
		return false
	}
	cacheKey := v.buildCacheKey(toolName, toolArgs)
	v.mu.RLock()
	if entry, ok := v.cache[cacheKey]; ok {
		if time.Since(entry.CachedAt).Seconds() < float64(cfg.CacheTTLSec) {
			v.mu.RUnlock()
			return false
		}
	}
	v.mu.RUnlock()
	if !v.checkBudget() {
		return false
	}
	return true
}

func (v *CounterfactualVerifier) checkBudget() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)
	newWindow := make([]time.Time, 0, len(v.hourlyWindow))
	for _, t := range v.hourlyWindow {
		if t.After(cutoff) {
			newWindow = append(newWindow, t)
		}
	}
	v.hourlyWindow = newWindow
	return len(v.hourlyWindow) < v.config.MaxPerHour
}

func (v *CounterfactualVerifier) consumeBudget() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hourlyWindow = append(v.hourlyWindow, time.Now())
}

func (v *CounterfactualVerifier) buildCacheKey(toolName, toolArgs string) string {
	truncated := toolArgs
	if len(truncated) > 512 {
		truncated = truncated[:512]
	}
	h := sha256.Sum256([]byte(toolName + ":" + truncated))
	return hex.EncodeToString(h[:])
}

func generateCFID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "cf-" + hex.EncodeToString(b)
}

// Verify 执行反事实验证
func (v *CounterfactualVerifier) Verify(ctx context.Context, originalReqBody []byte, toolName, toolArgs, upstreamURL, authHeader string) *CFVerification {
	start := time.Now()
	v.consumeBudget()

	cacheKey := v.buildCacheKey(toolName, toolArgs)
	v.mu.RLock()
	if entry, ok := v.cache[cacheKey]; ok {
		cfg := v.config
		v.mu.RUnlock()
		if time.Since(entry.CachedAt).Seconds() < float64(cfg.CacheTTLSec) {
			atomic.AddInt64(&v.cacheHits, 1)
			vf := &CFVerification{
				ID: generateCFID(), ToolName: toolName, ToolArgs: toolArgs,
				Survived: entry.Survived, AttributionScore: entry.AttributionScore,
				Verdict: entry.Verdict, Decision: v.verdictToDecision(entry.Verdict),
				LatencyMs: time.Since(start).Milliseconds(), Cached: true,
				TenantID: "default", CreatedAt: time.Now(),
			}
			v.recordVerification(vf)
			return vf
		}
	} else {
		v.mu.RUnlock()
	}
	atomic.AddInt64(&v.cacheMisses, 1)

	messages := v.parseMessages(originalReqBody)
	controlMessages := v.BuildControlMessages(messages)
	controlReqBody := v.buildControlRequest(originalReqBody, controlMessages)
	origMsgJSON := cfTruncateStr(cfMarshalMessages(messages), 4096)
	ctrlMsgJSON := cfTruncateStr(cfMarshalMessages(controlMessages), 4096)

	controlRespBody, err := v.sendControlRequest(ctx, controlReqBody, upstreamURL, authHeader)

	vf := &CFVerification{
		ID: generateCFID(), ToolName: toolName, ToolArgs: toolArgs,
		OriginalMessages: origMsgJSON, ControlMessages: ctrlMsgJSON,
		OriginalResult: cfTruncateStr(fmt.Sprintf(`{"tool_name":%q,"tool_args":%s}`, toolName, toolArgs), 4096),
		TenantID: "default", CreatedAt: time.Now(),
	}

	if err != nil {
		log.Printf("[Counterfactual] 对照请求失败: %v", err)
		vf.Verdict = "INCONCLUSIVE"
		vf.CausalDriver = "control_request_failed"
		vf.Decision = "allow"
		vf.LatencyMs = time.Since(start).Milliseconds()
		v.recordVerification(vf)
		atomic.AddInt64(&v.inconclusiveCount, 1)
		return vf
	}

	vf.ControlResult = cfTruncateStr(string(controlRespBody), 4096)

	v.mu.RLock()
	fuzzy := v.config.FuzzyMatch
	v.mu.RUnlock()

	survived, attribution := v.CompareResults(toolName, toolArgs, controlRespBody, fuzzy)
	vf.Survived = survived
	vf.AttributionScore = attribution

	if attribution <= 0.3 {
		vf.Verdict = "USER_DRIVEN"
		vf.CausalDriver = "tool_call_survived_in_control"
	} else if attribution >= 0.7 {
		vf.Verdict = "INJECTION_DRIVEN"
		vf.CausalDriver = "tool_call_absent_in_control"
	} else {
		vf.Verdict = "INCONCLUSIVE"
		vf.CausalDriver = "partial_match"
	}
	vf.Decision = v.verdictToDecision(vf.Verdict)
	vf.LatencyMs = time.Since(start).Milliseconds()

	v.mu.Lock()
	v.cache[cacheKey] = &CFCacheEntry{
		Survived: survived, AttributionScore: attribution,
		Verdict: vf.Verdict, CachedAt: time.Now(),
	}
	v.mu.Unlock()

	v.recordVerification(vf)

	switch vf.Verdict {
	case "USER_DRIVEN":
		atomic.AddInt64(&v.allowedCount, 1)
	case "INJECTION_DRIVEN":
		atomic.AddInt64(&v.blockedCount, 1)
	default:
		atomic.AddInt64(&v.inconclusiveCount, 1)
	}
	atomic.AddInt64(&v.totalLatencyMs, vf.LatencyMs)
	atomic.AddInt64(&v.totalAttribution, int64(attribution*1000))

	log.Printf("[Counterfactual] 验证完成: tool=%s verdict=%s attribution=%.2f latency=%dms",
		toolName, vf.Verdict, attribution, vf.LatencyMs)

	return vf
}

func (v *CounterfactualVerifier) verdictToDecision(verdict string) string {
	switch verdict {
	case "INJECTION_DRIVEN":
		return "block"
	case "INCONCLUSIVE":
		return "warn"
	default:
		return "allow"
	}
}

// parseMessages 从请求体解析 messages 数组
func (v *CounterfactualVerifier) parseMessages(body []byte) []CFMessage {
	var req map[string]interface{}
	if json.Unmarshal(body, &req) != nil {
		return nil
	}
	var messages []CFMessage
	if sys, ok := req["system"]; ok {
		messages = append(messages, CFMessage{Role: "system", Content: sys})
	}
	if msgs, ok := req["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			if m, ok := msg.(map[string]interface{}); ok {
				cfm := CFMessage{Role: cfStringVal(m, "role"), Content: m["content"]}
				if tid, ok := m["tool_use_id"].(string); ok {
					cfm.ToolUseID = tid
				}
				if tcid, ok := m["tool_call_id"].(string); ok {
					cfm.ToolCallID = tcid
				}
				if name, ok := m["name"].(string); ok {
					cfm.Name = name
				}
				messages = append(messages, cfm)
			}
		}
	}
	return messages
}

// BuildControlMessages 构造对照 messages — 移除外部数据载荷
func (v *CounterfactualVerifier) BuildControlMessages(messages []CFMessage) []CFMessage {
	var control []CFMessage
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			control = append(control, msg)
		case "user":
			filtered := v.filterToolResultBlocks(msg)
			if filtered != nil {
				control = append(control, *filtered)
			}
		case "assistant":
			control = append(control, msg)
		case "tool", "function":
			continue // 移除 OpenAI 格式的 tool/function messages
		default:
			control = append(control, msg)
		}
	}
	return control
}

func (v *CounterfactualVerifier) filterToolResultBlocks(msg CFMessage) *CFMessage {
	if _, ok := msg.Content.(string); ok {
		return &msg
	}
	blocks, ok := msg.Content.([]interface{})
	if !ok {
		return &msg
	}
	var filtered []interface{}
	for _, block := range blocks {
		bm, ok := block.(map[string]interface{})
		if !ok {
			filtered = append(filtered, block)
			continue
		}
		blockType, _ := bm["type"].(string)
		if blockType == "tool_result" {
			continue
		}
		filtered = append(filtered, block)
	}
	if len(filtered) == 0 {
		return nil
	}
	return &CFMessage{Role: msg.Role, Content: filtered}
}

func (v *CounterfactualVerifier) buildControlRequest(originalBody []byte, controlMessages []CFMessage) []byte {
	var req map[string]interface{}
	if json.Unmarshal(originalBody, &req) != nil {
		return originalBody
	}
	_, hasTopSystem := req["system"]
	var newMessages []interface{}
	for _, msg := range controlMessages {
		if msg.Role == "system" && hasTopSystem {
			continue
		}
		m := map[string]interface{}{"role": msg.Role, "content": msg.Content}
		if msg.ToolUseID != "" {
			m["tool_use_id"] = msg.ToolUseID
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		if msg.Name != "" {
			m["name"] = msg.Name
		}
		newMessages = append(newMessages, m)
	}
	req["messages"] = newMessages
	result, err := json.Marshal(req)
	if err != nil {
		return originalBody
	}
	return result
}

func (v *CounterfactualVerifier) sendControlRequest(ctx context.Context, body []byte, upstreamURL, authHeader string) ([]byte, error) {
	v.mu.RLock()
	timeout := time.Duration(v.config.TimeoutSec) * time.Second
	v.mu.RUnlock()
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("X-Counterfactual-Control", "true")
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream error: status=%d", resp.StatusCode)
	}
	return respBody, nil
}

// CompareResults 比对原始 tool call 和对照组响应
func (v *CounterfactualVerifier) CompareResults(origToolName, origToolArgs string, controlResp []byte, fuzzyMatch bool) (survived bool, attribution float64) {
	info := ParseAnthropicResponse(controlResp)
	if info == nil || !info.HasToolUse || len(info.ToolNames) == 0 {
		return false, 1.0
	}
	for i, tcName := range info.ToolNames {
		if tcName == origToolName {
			tcArgs := ""
			if i < len(info.ToolInputs) {
				tcArgs = info.ToolInputs[i]
			}
			if v.argsEqual(origToolArgs, tcArgs) {
				return true, 0.0
			}
			if fuzzyMatch {
				return true, 0.3
			}
			return false, 0.7
		}
	}
	return false, 0.9
}

func (v *CounterfactualVerifier) argsEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return true
	}
	var ja, jb interface{}
	if json.Unmarshal([]byte(a), &ja) == nil && json.Unmarshal([]byte(b), &jb) == nil {
		na, _ := json.Marshal(ja)
		nb, _ := json.Marshal(jb)
		return string(na) == string(nb)
	}
	return false
}

func (v *CounterfactualVerifier) recordVerification(vf *CFVerification) {
	atomic.AddInt64(&v.totalVerifications, 1)
	if v.db == nil {
		return
	}
	survived := 0
	if vf.Survived {
		survived = 1
	}
	cached := 0
	if vf.Cached {
		cached = 1
	}
	_, err := v.db.Exec(`INSERT INTO cf_verifications
		(id, trace_id, tool_name, tool_args, original_messages, control_messages,
		 original_result, control_result, survived, attribution_score, causal_driver,
		 verdict, decision, latency_ms, cached, tenant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		vf.ID, vf.TraceID, vf.ToolName, vf.ToolArgs,
		vf.OriginalMessages, vf.ControlMessages,
		vf.OriginalResult, vf.ControlResult,
		survived, vf.AttributionScore, vf.CausalDriver,
		vf.Verdict, vf.Decision, vf.LatencyMs, cached,
		vf.TenantID, vf.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		log.Printf("[Counterfactual] DB 写入失败: %v", err)
	}
}

// GetVerification 查询单条验证记录
func (v *CounterfactualVerifier) GetVerification(id string) *CFVerification {
	if v.db == nil {
		return nil
	}
	row := v.db.QueryRow(`SELECT id, trace_id, tool_name, COALESCE(tool_args,''),
		COALESCE(original_messages,''), COALESCE(control_messages,''),
		COALESCE(original_result,''), COALESCE(control_result,''),
		survived, attribution_score, COALESCE(causal_driver,''), verdict, decision,
		COALESCE(latency_ms,0), cached, COALESCE(tenant_id,'default'), COALESCE(created_at,'')
		FROM cf_verifications WHERE id = ?`, id)
	return v.scanVerification(row)
}

func (v *CounterfactualVerifier) scanVerification(row *sql.Row) *CFVerification {
	var vf CFVerification
	var survived, cached int
	var createdAt string
	err := row.Scan(&vf.ID, &vf.TraceID, &vf.ToolName, &vf.ToolArgs,
		&vf.OriginalMessages, &vf.ControlMessages, &vf.OriginalResult, &vf.ControlResult,
		&survived, &vf.AttributionScore, &vf.CausalDriver, &vf.Verdict, &vf.Decision,
		&vf.LatencyMs, &cached, &vf.TenantID, &createdAt)
	if err != nil {
		return nil
	}
	vf.Survived = survived != 0
	vf.Cached = cached != 0
	vf.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &vf
}

// QueryVerifications 查询验证记录列表
func (v *CounterfactualVerifier) QueryVerifications(traceID, verdict, since string, limit int) []CFVerification {
	if v.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	q := `SELECT id, trace_id, tool_name, COALESCE(tool_args,''),
		COALESCE(original_messages,''), COALESCE(control_messages,''),
		COALESCE(original_result,''), COALESCE(control_result,''),
		survived, attribution_score, COALESCE(causal_driver,''), verdict, decision,
		COALESCE(latency_ms,0), cached, COALESCE(tenant_id,'default'), COALESCE(created_at,'')
		FROM cf_verifications WHERE 1=1`
	var args []interface{}
	if traceID != "" {
		q += " AND trace_id = ?"
		args = append(args, traceID)
	}
	if verdict != "" {
		q += " AND verdict = ?"
		args = append(args, verdict)
	}
	if since != "" {
		q += " AND created_at >= ?"
		args = append(args, since)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := v.db.Query(q, args...)
	if err != nil {
		log.Printf("[Counterfactual] 查询失败: %v", err)
		return nil
	}
	defer rows.Close()
	var results []CFVerification
	for rows.Next() {
		var vf CFVerification
		var survived, cached int
		var createdAt string
		err := rows.Scan(&vf.ID, &vf.TraceID, &vf.ToolName, &vf.ToolArgs,
			&vf.OriginalMessages, &vf.ControlMessages, &vf.OriginalResult, &vf.ControlResult,
			&survived, &vf.AttributionScore, &vf.CausalDriver, &vf.Verdict, &vf.Decision,
			&vf.LatencyMs, &cached, &vf.TenantID, &createdAt)
		if err != nil {
			continue
		}
		vf.Survived = survived != 0
		vf.Cached = cached != 0
		vf.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, vf)
	}
	return results
}

// GetStats 获取统计
func (v *CounterfactualVerifier) GetStats() CFStats {
	total := atomic.LoadInt64(&v.totalVerifications)
	blocked := atomic.LoadInt64(&v.blockedCount)
	allowed := atomic.LoadInt64(&v.allowedCount)
	inconclusive := atomic.LoadInt64(&v.inconclusiveCount)
	hits := atomic.LoadInt64(&v.cacheHits)
	misses := atomic.LoadInt64(&v.cacheMisses)
	totalLat := atomic.LoadInt64(&v.totalLatencyMs)
	totalAttr := atomic.LoadInt64(&v.totalAttribution)

	v.mu.RLock()
	now := time.Now()
	cutoff := now.Add(-1 * time.Hour)
	hourlyUsed := 0
	for _, t := range v.hourlyWindow {
		if t.After(cutoff) {
			hourlyUsed++
		}
	}
	budget := v.config.MaxPerHour
	v.mu.RUnlock()

	var cacheHitRate float64
	if hits+misses > 0 {
		cacheHitRate = float64(hits) / float64(hits+misses)
	}
	var avgLatency float64
	if total > 0 {
		avgLatency = float64(totalLat) / float64(total)
	}
	var avgAttribution float64
	if total > 0 {
		avgAttribution = float64(totalAttr) / (float64(total) * 1000)
	}

	return CFStats{
		TotalVerifications: total,
		HourlyUsed:         hourlyUsed,
		HourlyBudget:       budget,
		BlockedCount:       blocked,
		AllowedCount:       allowed,
		InconclusiveCount:  inconclusive,
		CacheHitRate:       cacheHitRate,
		AvgLatencyMs:       avgLatency,
		AvgAttribution:     avgAttribution,
	}
}

// UpdateConfig 运行时更新配置
func (v *CounterfactualVerifier) UpdateConfig(cfg CFConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if cfg.MaxPerHour > 0 {
		v.config.MaxPerHour = cfg.MaxPerHour
	}
	if cfg.RiskThreshold > 0 {
		v.config.RiskThreshold = cfg.RiskThreshold
	}
	if cfg.CacheTTLSec > 0 {
		v.config.CacheTTLSec = cfg.CacheTTLSec
	}
	if cfg.TimeoutSec > 0 {
		v.config.TimeoutSec = cfg.TimeoutSec
	}
	if cfg.Mode == "sync" || cfg.Mode == "async" {
		v.config.Mode = cfg.Mode
	}
	v.config.Enabled = cfg.Enabled
	v.config.FuzzyMatch = cfg.FuzzyMatch
	v.stats.HourlyBudget = v.config.MaxPerHour
	log.Printf("[Counterfactual] 配置更新: enabled=%v mode=%s budget=%d threshold=%.0f fuzzy=%v",
		v.config.Enabled, v.config.Mode, v.config.MaxPerHour, v.config.RiskThreshold, v.config.FuzzyMatch)
}

// GetConfig 获取当前配置
func (v *CounterfactualVerifier) GetConfig() CFConfig {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.config
}

// GetCacheStats 获取缓存状态
func (v *CounterfactualVerifier) GetCacheStats() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()
	valid := 0
	expired := 0
	now := time.Now()
	for _, entry := range v.cache {
		if now.Sub(entry.CachedAt).Seconds() < float64(v.config.CacheTTLSec) {
			valid++
		} else {
			expired++
		}
	}
	return map[string]interface{}{
		"total_entries":   len(v.cache),
		"valid_entries":   valid,
		"expired_entries": expired,
		"ttl_sec":         v.config.CacheTTLSec,
		"hit_count":       atomic.LoadInt64(&v.cacheHits),
		"miss_count":      atomic.LoadInt64(&v.cacheMisses),
	}
}

// ClearCache 清除缓存
func (v *CounterfactualVerifier) ClearCache() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := len(v.cache)
	v.cache = make(map[string]*CFCacheEntry)
	log.Printf("[Counterfactual] 缓存已清除: %d 条目", n)
	return n
}

// 辅助函数

func cfStringVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func cfMarshalMessages(msgs []CFMessage) string {
	b, err := json.Marshal(msgs)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func cfTruncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}