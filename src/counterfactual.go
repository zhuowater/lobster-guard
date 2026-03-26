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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CFConfig 反事实验证配置
type CFConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Mode          string   `yaml:"mode" json:"mode"`
	MaxPerHour    int      `yaml:"max_per_hour" json:"max_per_hour"`
	RiskThreshold float64  `yaml:"risk_threshold" json:"risk_threshold"`
	CacheTTLSec   int      `yaml:"cache_ttl_sec" json:"cache_ttl_sec"`
	TimeoutSec    int      `yaml:"timeout_sec" json:"timeout_sec"`
	FuzzyMatch    bool     `yaml:"fuzzy_match" json:"fuzzy_match"`
	HighRiskTools []string `yaml:"high_risk_tools" json:"high_risk_tools"`
}

// CFVerification 一次验证记录
type CFVerification struct {
	ID               string    `json:"id"`
	TraceID          string    `json:"trace_id"`
	SenderID         string    `json:"sender_id"`
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

// AttributionReport 归因报告
type AttributionReport struct {
	ID                   string       `json:"id"`
	VerificationID       string       `json:"verification_id"`
	TraceID              string       `json:"trace_id"`
	OriginalToolCall     string       `json:"original_tool_call"`
	CounterfactualResult string       `json:"counterfactual_result"`
	CausalDriver         string       `json:"causal_driver"`
	CausalChain          []CausalStep `json:"causal_chain"`
	AttributionScore     float64      `json:"attribution_score"`
	Verdict              string       `json:"verdict"`
	EvidenceSummary      string       `json:"evidence_summary"`
	CreatedAt            time.Time    `json:"created_at"`
}

// CausalStep 因果链步骤
type CausalStep struct {
	StepIndex   int    `json:"step_index"`
	Role        string `json:"role"`
	ContentType string `json:"content_type"`
	WasRemoved  bool   `json:"was_removed"`
	Impact      string `json:"impact"`
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
	db               *sql.DB
	mu               sync.RWMutex
	config           CFConfig
	client           *http.Client
	cache            map[string]*CFCacheEntry
	stats            CFStats
	pathPolicy       *PathPolicyEngine
	adaptiveStrategy *AdaptiveStrategy // v24.2

	// 可配置的高风险工具列表（锁保护 via mu）
	customHighRiskTools map[string]bool

	// v24.1 归因报告联动
	envelopeMgr    *EnvelopeManager
	userProfileEng *UserProfileEngine

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

var defaultHighRiskTools = map[string]bool{
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
	// 初始化高风险工具列表
	customTools := make(map[string]bool)
	if len(config.HighRiskTools) > 0 {
		for _, t := range config.HighRiskTools {
			customTools[t] = true
		}
	} else {
		for k, v := range defaultHighRiskTools {
			customTools[k] = v
		}
	}

	v := &CounterfactualVerifier{
		db:                  db,
		config:              config,
		client:              client,
		cache:               make(map[string]*CFCacheEntry),
		customHighRiskTools: customTools,
		hourlyWindow:        make([]time.Time, 0, config.MaxPerHour),
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

	// v24.1: 归因报告表
	v.db.Exec(`CREATE TABLE IF NOT EXISTS attribution_reports (
		id TEXT PRIMARY KEY,
		verification_id TEXT NOT NULL,
		trace_id TEXT NOT NULL,
		original_tool_call TEXT,
		counterfactual_result TEXT,
		causal_driver TEXT,
		causal_chain TEXT,
		attribution_score REAL NOT NULL DEFAULT 0,
		verdict TEXT NOT NULL,
		evidence_summary TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ar_verification ON attribution_reports(verification_id)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ar_trace ON attribution_reports(trace_id)`)
	v.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ar_created ON attribution_reports(created_at)`)
}

// SetEnvelopeManager 设置执行信封管理器（v24.1 联动）
func (v *CounterfactualVerifier) SetEnvelopeManager(em *EnvelopeManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.envelopeMgr = em
}

// SetUserProfileEngine 设置用户画像引擎（v24.1 联动）
func (v *CounterfactualVerifier) SetUserProfileEngine(upe *UserProfileEngine) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.userProfileEng = upe
}

func (v *CounterfactualVerifier) SetPathPolicy(pp *PathPolicyEngine) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pathPolicy = pp
}

// SetAdaptiveStrategy v24.2: 关联自适应验证策略引擎
func (v *CounterfactualVerifier) SetAdaptiveStrategy(as *AdaptiveStrategy) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.adaptiveStrategy = as
}


// ShouldVerify 判断是否需要对该 tool call 进行反事实验证
func (v *CounterfactualVerifier) ShouldVerify(toolName, toolArgs, traceID string, riskScore float64) bool {
	v.mu.RLock()
	cfg := v.config
	v.mu.RUnlock()

	if !cfg.Enabled {
		return false
	}
	isHighRisk := v.isHighRiskTool(toolName)
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
// senderID 可选，用于 v24.1 攻击者画像联动
func (v *CounterfactualVerifier) Verify(ctx context.Context, originalReqBody []byte, toolName, toolArgs, upstreamURL, authHeader string, senderID ...string) *CFVerification {
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
			if len(senderID) > 0 {
				vf.SenderID = senderID[0]
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
	if len(senderID) > 0 {
		vf.SenderID = senderID[0]
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

	// v24.1: 生成归因报告
	v.generateAttributionReport(vf, messages, controlMessages)

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
	cfg := v.config
	// 同步 HighRiskTools 字段
	cfg.HighRiskTools = v.getHighRiskToolsLocked()
	return cfg
}

// isHighRiskTool 内部查询工具是否为高风险（调用方需持有锁或方法内部加锁）
func (v *CounterfactualVerifier) isHighRiskTool(name string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.customHighRiskTools[name]
}

// GetHighRiskTools 返回当前高风险工具列表
func (v *CounterfactualVerifier) GetHighRiskTools() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.getHighRiskToolsLocked()
}

// getHighRiskToolsLocked 内部方法，调用方需持有读锁
func (v *CounterfactualVerifier) getHighRiskToolsLocked() []string {
	tools := make([]string, 0, len(v.customHighRiskTools))
	for k := range v.customHighRiskTools {
		tools = append(tools, k)
	}
	// 排序以保证顺序稳定
	sort.Strings(tools)
	return tools
}

// AddHighRiskTool 添加一个高风险工具
func (v *CounterfactualVerifier) AddHighRiskTool(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.customHighRiskTools[name] = true
	log.Printf("[Counterfactual] 添加高风险工具: %s (total=%d)", name, len(v.customHighRiskTools))
}

// RemoveHighRiskTool 删除一个高风险工具
func (v *CounterfactualVerifier) RemoveHighRiskTool(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.customHighRiskTools, name)
	log.Printf("[Counterfactual] 移除高风险工具: %s (total=%d)", name, len(v.customHighRiskTools))
}

// SetHighRiskTools 批量设置高风险工具列表
func (v *CounterfactualVerifier) SetHighRiskTools(tools []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.customHighRiskTools = make(map[string]bool, len(tools))
	for _, t := range tools {
		v.customHighRiskTools[t] = true
	}
	log.Printf("[Counterfactual] 批量设置高风险工具: %d 个", len(tools))
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

// ============================================================
// v24.1 归因报告生成 + 联动
// ============================================================

func generateARID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ar-" + hex.EncodeToString(b)
}

// generateAttributionReport 在 Verify 末尾调用，生成归因报告并持久化
func (v *CounterfactualVerifier) generateAttributionReport(vf *CFVerification, origMsgs, ctrlMsgs []CFMessage) {
	if vf == nil {
		return
	}

	report := &AttributionReport{
		ID:                   generateARID(),
		VerificationID:       vf.ID,
		TraceID:              vf.TraceID,
		OriginalToolCall:     fmt.Sprintf("%s(%s)", vf.ToolName, cfTruncateStr(vf.ToolArgs, 200)),
		CounterfactualResult: cfTruncateStr(vf.ControlResult, 500),
		CausalDriver:         vf.CausalDriver,
		AttributionScore:     vf.AttributionScore,
		Verdict:              vf.Verdict,
		CreatedAt:            vf.CreatedAt,
	}

	// 构建因果链：分析 origMsgs 与 ctrlMsgs 的差异
	report.CausalChain = v.buildCausalChain(origMsgs, ctrlMsgs)

	// 生成人类可读的证据摘要（中文）
	report.EvidenceSummary = v.buildEvidenceSummary(vf, report.CausalChain)

	// 持久化到 DB
	v.saveAttributionReport(report)

	// v24.1: 与 v18.0 执行信封联动
	v.mu.RLock()
	em := v.envelopeMgr
	upe := v.userProfileEng
	v.mu.RUnlock()

	if em != nil {
		reportJSON, _ := json.Marshal(report)
		em.Seal(vf.TraceID, "attribution_report", string(reportJSON), vf.Verdict, nil, "")
	}

	// v24.1: 与 v11.0 攻击者画像联动
	if upe != nil && vf.Verdict == "INJECTION_DRIVEN" && vf.SenderID != "" {
		upe.RecordEvent(vf.SenderID, "injection_detected", vf.AttributionScore)
	}

	log.Printf("[Counterfactual] 归因报告生成: id=%s verdict=%s score=%.2f chain_len=%d",
		report.ID, report.Verdict, report.AttributionScore, len(report.CausalChain))
}

// buildCausalChain 分析原始与对照 messages 的差异，构建因果链
func (v *CounterfactualVerifier) buildCausalChain(origMsgs, ctrlMsgs []CFMessage) []CausalStep {
	var chain []CausalStep

	// 构建对照 messages 的快速查找集（按 role+index）
	ctrlSet := make(map[string]bool)
	for i, msg := range ctrlMsgs {
		key := fmt.Sprintf("%d:%s", i, msg.Role)
		ctrlSet[key] = true
	}

	ctrlIdx := 0
	for i, msg := range origMsgs {
		step := CausalStep{
			StepIndex: i,
			Role:      msg.Role,
		}

		// 判断 content type
		switch msg.Content.(type) {
		case string:
			step.ContentType = "text"
		case []interface{}:
			blocks, _ := msg.Content.([]interface{})
			hasToolResult := false
			for _, b := range blocks {
				if bm, ok := b.(map[string]interface{}); ok {
					if bm["type"] == "tool_result" {
						hasToolResult = true
						break
					}
					if bm["type"] == "tool_use" {
						step.ContentType = "tool_use"
					}
				}
			}
			if hasToolResult {
				step.ContentType = "tool_result"
			} else if step.ContentType == "" {
				step.ContentType = "text"
			}
		default:
			step.ContentType = "text"
		}

		// 判断是否被移除
		wasRemoved := false
		if msg.Role == "tool" || msg.Role == "function" {
			wasRemoved = true
		} else if step.ContentType == "tool_result" {
			wasRemoved = true
		} else {
			// 检查对照组中是否还存在匹配项
			if ctrlIdx < len(ctrlMsgs) && ctrlMsgs[ctrlIdx].Role == msg.Role {
				ctrlIdx++
			} else {
				wasRemoved = true
			}
		}
		step.WasRemoved = wasRemoved

		// 判断影响程度
		if wasRemoved {
			if step.ContentType == "tool_result" || msg.Role == "tool" || msg.Role == "function" {
				step.Impact = "high"
			} else {
				step.Impact = "medium"
			}
		} else {
			step.Impact = "low"
		}

		chain = append(chain, step)
	}

	return chain
}

// buildEvidenceSummary 生成人类可读的中文解释
func (v *CounterfactualVerifier) buildEvidenceSummary(vf *CFVerification, chain []CausalStep) string {
	removedCount := 0
	highImpactCount := 0
	var removedRoles []string
	for _, step := range chain {
		if step.WasRemoved {
			removedCount++
			removedRoles = append(removedRoles, fmt.Sprintf("step%d(%s/%s)", step.StepIndex, step.Role, step.ContentType))
			if step.Impact == "high" {
				highImpactCount++
			}
		}
	}

	var summary string
	switch vf.Verdict {
	case "INJECTION_DRIVEN":
		summary = fmt.Sprintf(
			"分析结论: 该工具调用 %s 被判定为注入驱动。对照实验中移除了 %d 个外部数据步骤（其中 %d 个高影响），"+
				"工具调用在对照组中消失，归因分数 %.2f（高于0.7阈值）。"+
				"因果来源: %s。移除的步骤: %s。",
			vf.ToolName, removedCount, highImpactCount,
			vf.AttributionScore, vf.CausalDriver,
			strings.Join(removedRoles, ", "),
		)
	case "USER_DRIVEN":
		summary = fmt.Sprintf(
			"分析结论: 该工具调用 %s 被判定为用户驱动。对照实验中移除了 %d 个外部数据步骤，"+
				"但工具调用在对照组中仍然存在，归因分数 %.2f（低于0.3阈值）。"+
				"这表明该调用源自用户的原始意图，而非外部数据注入。",
			vf.ToolName, removedCount, vf.AttributionScore,
		)
	default:
		summary = fmt.Sprintf(
			"分析结论: 该工具调用 %s 的因果归因不确定。对照实验中移除了 %d 个外部数据步骤，"+
				"归因分数 %.2f 处于灰色区间（0.3-0.7）。建议人工审查。因果来源: %s。",
			vf.ToolName, removedCount, vf.AttributionScore, vf.CausalDriver,
		)
	}

	return summary
}

// saveAttributionReport 持久化归因报告
func (v *CounterfactualVerifier) saveAttributionReport(report *AttributionReport) {
	if v.db == nil || report == nil {
		return
	}
	chainJSON, _ := json.Marshal(report.CausalChain)
	_, err := v.db.Exec(`INSERT INTO attribution_reports
		(id, verification_id, trace_id, original_tool_call, counterfactual_result,
		 causal_driver, causal_chain, attribution_score, verdict, evidence_summary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID, report.VerificationID, report.TraceID,
		report.OriginalToolCall, report.CounterfactualResult,
		report.CausalDriver, string(chainJSON),
		report.AttributionScore, report.Verdict,
		report.EvidenceSummary,
		report.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		log.Printf("[Counterfactual] 归因报告写入失败: %v", err)
	}
}

// GetAttributionReport 查询单条归因报告
func (v *CounterfactualVerifier) GetAttributionReport(id string) *AttributionReport {
	if v.db == nil {
		return nil
	}
	row := v.db.QueryRow(`SELECT id, verification_id, trace_id, COALESCE(original_tool_call,''),
		COALESCE(counterfactual_result,''), COALESCE(causal_driver,''),
		COALESCE(causal_chain,'[]'), attribution_score, verdict,
		COALESCE(evidence_summary,''), COALESCE(created_at,'')
		FROM attribution_reports WHERE id = ?`, id)
	return v.scanAttributionReport(row)
}

func (v *CounterfactualVerifier) scanAttributionReport(row *sql.Row) *AttributionReport {
	var ar AttributionReport
	var chainJSON, createdAt string
	err := row.Scan(&ar.ID, &ar.VerificationID, &ar.TraceID,
		&ar.OriginalToolCall, &ar.CounterfactualResult,
		&ar.CausalDriver, &chainJSON,
		&ar.AttributionScore, &ar.Verdict,
		&ar.EvidenceSummary, &createdAt)
	if err != nil {
		return nil
	}
	json.Unmarshal([]byte(chainJSON), &ar.CausalChain)
	if ar.CausalChain == nil {
		ar.CausalChain = []CausalStep{}
	}
	ar.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &ar
}

// QueryAttributionReports 查询归因报告列表
func (v *CounterfactualVerifier) QueryAttributionReports(traceID, verdict, since string, limit int) []AttributionReport {
	if v.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	q := `SELECT id, verification_id, trace_id, COALESCE(original_tool_call,''),
		COALESCE(counterfactual_result,''), COALESCE(causal_driver,''),
		COALESCE(causal_chain,'[]'), attribution_score, verdict,
		COALESCE(evidence_summary,''), COALESCE(created_at,'')
		FROM attribution_reports WHERE 1=1`
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
		log.Printf("[Counterfactual] 归因报告查询失败: %v", err)
		return nil
	}
	defer rows.Close()

	var results []AttributionReport
	for rows.Next() {
		var ar AttributionReport
		var chainJSON, createdAt string
		err := rows.Scan(&ar.ID, &ar.VerificationID, &ar.TraceID,
			&ar.OriginalToolCall, &ar.CounterfactualResult,
			&ar.CausalDriver, &chainJSON,
			&ar.AttributionScore, &ar.Verdict,
			&ar.EvidenceSummary, &createdAt)
		if err != nil {
			continue
		}
		json.Unmarshal([]byte(chainJSON), &ar.CausalChain)
		if ar.CausalChain == nil {
			ar.CausalChain = []CausalStep{}
		}
		ar.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, ar)
	}
	return results
}

// CFTimelineEvent 时间线事件（验证+归因合并）
type CFTimelineEvent struct {
	Timestamp        time.Time    `json:"timestamp"`
	EventType        string       `json:"event_type"` // "verification" or "attribution"
	ID               string       `json:"id"`
	TraceID          string       `json:"trace_id"`
	ToolName         string       `json:"tool_name"`
	Verdict          string       `json:"verdict"`
	AttributionScore float64      `json:"attribution_score"`
	Decision         string       `json:"decision,omitempty"`
	EvidenceSummary  string       `json:"evidence_summary,omitempty"`
	CausalChain      []CausalStep `json:"causal_chain,omitempty"`
}

// QueryTimeline 查询因果归因时间线
func (v *CounterfactualVerifier) QueryTimeline(since string, limit int) []CFTimelineEvent {
	if v.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var events []CFTimelineEvent

	// 查询验证记录
	vq := `SELECT id, trace_id, tool_name, verdict, attribution_score, decision, COALESCE(created_at,'')
		FROM cf_verifications WHERE 1=1`
	var vargs []interface{}
	if since != "" {
		vq += " AND created_at >= ?"
		vargs = append(vargs, since)
	}
	vq += " ORDER BY created_at DESC LIMIT ?"
	vargs = append(vargs, limit)

	rows, err := v.db.Query(vq, vargs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e CFTimelineEvent
			var createdAt string
			err := rows.Scan(&e.ID, &e.TraceID, &e.ToolName, &e.Verdict, &e.AttributionScore, &e.Decision, &createdAt)
			if err != nil {
				continue
			}
			e.EventType = "verification"
			e.Timestamp, _ = time.Parse(time.RFC3339, createdAt)
			events = append(events, e)
		}
	}

	// 查询归因报告
	aq := `SELECT id, trace_id, COALESCE(original_tool_call,''), verdict, attribution_score, COALESCE(evidence_summary,''), COALESCE(causal_chain,'[]'), COALESCE(created_at,'')
		FROM attribution_reports WHERE 1=1`
	var aargs []interface{}
	if since != "" {
		aq += " AND created_at >= ?"
		aargs = append(aargs, since)
	}
	aq += " ORDER BY created_at DESC LIMIT ?"
	aargs = append(aargs, limit)

	rows2, err := v.db.Query(aq, aargs...)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var e CFTimelineEvent
			var createdAt, chainJSON string
			err := rows2.Scan(&e.ID, &e.TraceID, &e.ToolName, &e.Verdict, &e.AttributionScore, &e.EvidenceSummary, &chainJSON, &createdAt)
			if err != nil {
				continue
			}
			e.EventType = "attribution"
			e.Timestamp, _ = time.Parse(time.RFC3339, createdAt)
			json.Unmarshal([]byte(chainJSON), &e.CausalChain)
			if e.CausalChain == nil {
				e.CausalChain = []CausalStep{}
			}
			events = append(events, e)
		}
	}

	// 按时间排序（最近优先）
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Timestamp.After(events[i].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	if len(events) > limit {
		events = events[:limit]
	}

	return events
}