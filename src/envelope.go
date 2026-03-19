// envelope.go — 执行信封核心 — 密码学审计链（v18.0）
// 每个安全决策生成 HMAC-SHA256 签名的不可篡改证据
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ExecutionEnvelope 执行信封 — 每个安全决策的密码学证据
type ExecutionEnvelope struct {
	ID          string    `json:"id"`           // 信封唯一 ID (16 字节 hex)
	TraceID     string    `json:"trace_id"`     // 关联的 trace_id
	Timestamp   time.Time `json:"timestamp"`    // 决策时间
	Domain      string    `json:"domain"`       // "inbound" / "outbound" / "llm_request" / "llm_response"
	RequestHash string    `json:"request_hash"` // SHA256(原始请求内容)
	Decision    string    `json:"decision"`     // "pass" / "warn" / "block" / "rewrite"
	Rules       []string  `json:"rules"`        // 触发的规则名列表
	SenderID    string    `json:"sender_id"`    // 关联的发送者
	Nonce       string    `json:"nonce"`        // 随机数防重放 (16 字节 hex)
	PrevHash    string    `json:"prev_hash"`    // 前一个信封的 ContentHash（全局链式结构）
	ContentHash string    `json:"content_hash"` // SHA256(所有字段拼接)
	Signature   string    `json:"signature"`    // HMAC-SHA256(ContentHash, secret_key)
}

// VerifyResult 单个信封验证结果
type VerifyResult struct {
	Valid          bool     `json:"valid"`
	EnvelopeID     string   `json:"envelope_id"`
	FailureReasons []string `json:"failure_reasons,omitempty"`
}

// ChainVerifyResult 信封链验证结果
type ChainVerifyResult struct {
	ChainValid   bool     `json:"chain_valid"`
	TraceID      string   `json:"trace_id"`
	TotalCount   int      `json:"total_count"`
	ValidCount   int      `json:"valid_count"`
	InvalidCount int      `json:"invalid_count"`
	BrokenAt     int      `json:"broken_at,omitempty"` // 链断裂的位置索引（0-based），-1 表示无断裂
	Details      []string `json:"details,omitempty"`    // 详细信息
}

// EnvelopeManager 信封管理器
type EnvelopeManager struct {
	db        *sql.DB
	secretKey string
	mu        sync.Mutex // 保护 prevHash 和 pendingLeaves 的写入
	prevHash  string     // 内存中缓存的最后一个信封的 ContentHash

	// v18.0+ Merkle Tree 批次
	pendingLeaves []pendingLeaf  // 待构建批次的叶子缓冲区
	batchSize     int            // 批次大小（默认 64）
	prevBatchRoot string         // 前一个批次的 Root（根链）
	flushTicker   *time.Ticker   // 定时 flush 定时器
	stopCh        chan struct{}   // 停止信号
}

// NewEnvelopeManager 创建信封管理器
func NewEnvelopeManager(db *sql.DB, secretKey string) *EnvelopeManager {
	return NewEnvelopeManagerWithBatchSize(db, secretKey, 64)
}

// NewEnvelopeManagerWithBatchSize 创建信封管理器（指定批次大小）
func NewEnvelopeManagerWithBatchSize(db *sql.DB, secretKey string, batchSize int) *EnvelopeManager {
	if batchSize <= 0 {
		batchSize = 64
	}
	em := &EnvelopeManager{
		db:        db,
		secretKey: secretKey,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
	}
	em.initTable()
	em.initMerkleTables()
	em.loadLastHash()
	em.loadLastBatchRoot()
	return em
}

// initTable 初始化 SQLite 表
func (em *EnvelopeManager) initTable() {
	em.db.Exec(`CREATE TABLE IF NOT EXISTS execution_envelopes (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		domain TEXT NOT NULL,
		request_hash TEXT NOT NULL,
		decision TEXT NOT NULL,
		rules_json TEXT NOT NULL,
		sender_id TEXT DEFAULT '',
		nonce TEXT NOT NULL,
		prev_hash TEXT DEFAULT '',
		content_hash TEXT NOT NULL,
		signature TEXT NOT NULL
	)`)
	em.db.Exec(`CREATE INDEX IF NOT EXISTS idx_envelopes_trace ON execution_envelopes(trace_id)`)
	em.db.Exec(`CREATE INDEX IF NOT EXISTS idx_envelopes_ts ON execution_envelopes(timestamp)`)
}

// loadLastHash 从数据库加载最后一个信封的 ContentHash
func (em *EnvelopeManager) loadLastHash() {
	row := em.db.QueryRow(`SELECT content_hash FROM execution_envelopes ORDER BY timestamp DESC, rowid DESC LIMIT 1`)
	var hash string
	if row.Scan(&hash) == nil {
		em.prevHash = hash
	}
}

// generateID 生成 16 字节 hex 编码的随机 ID
func generateEnvelopeID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// generateNonce 生成 16 字节 hex 编码的随机 nonce
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// computeRequestHash 计算请求内容的 SHA256
func computeRequestHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// computeContentHash 计算信封所有字段拼接的 SHA256
func computeContentHash(env *ExecutionEnvelope) string {
	rulesStr := strings.Join(env.Rules, ",")
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		env.ID,
		env.TraceID,
		env.Timestamp.UTC().Format(time.RFC3339Nano),
		env.Domain,
		env.RequestHash,
		env.Decision,
		rulesStr,
		env.SenderID,
		env.Nonce,
		env.PrevHash,
	)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// computeSignature 计算 HMAC-SHA256 签名
func computeSignature(contentHash, secretKey string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(contentHash))
	return hex.EncodeToString(mac.Sum(nil))
}

// Seal 生成并存储一个执行信封
func (em *EnvelopeManager) Seal(traceID, domain, requestContent, decision string, rules []string, senderID string) *ExecutionEnvelope {
	if rules == nil {
		rules = []string{}
	}

	now := time.Now().UTC()

	env := &ExecutionEnvelope{
		ID:          generateEnvelopeID(),
		TraceID:     traceID,
		Timestamp:   now,
		Domain:      domain,
		RequestHash: computeRequestHash(requestContent),
		Decision:    decision,
		Rules:       rules,
		SenderID:    senderID,
		Nonce:       generateNonce(),
	}

	// 获取全局链的前一个哈希 + 写入数据库（加锁保护，确保链式和写入的原子性）
	em.mu.Lock()
	env.PrevHash = em.prevHash

	// 计算 ContentHash 和 Signature
	env.ContentHash = computeContentHash(env)
	env.Signature = computeSignature(env.ContentHash, em.secretKey)

	// 写入数据库
	rulesJSON, _ := json.Marshal(env.Rules)
	em.db.Exec(`INSERT INTO execution_envelopes (id, trace_id, timestamp, domain, request_hash, decision, rules_json, sender_id, nonce, prev_hash, content_hash, signature) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		env.ID,
		env.TraceID,
		env.Timestamp.UTC().Format(time.RFC3339Nano),
		env.Domain,
		env.RequestHash,
		env.Decision,
		string(rulesJSON),
		env.SenderID,
		env.Nonce,
		env.PrevHash,
		env.ContentHash,
		env.Signature,
	)

	// 更新全局链
	em.prevHash = env.ContentHash

	// v18.0+ 加入 Merkle 批次缓冲区
	em.pendingLeaves = append(em.pendingLeaves, pendingLeaf{
		envelopeID:  env.ID,
		contentHash: env.ContentHash,
	})

	// 当缓冲区满时自动构建批次
	if len(em.pendingLeaves) >= em.batchSize {
		em.buildBatch()
	}

	em.mu.Unlock()

	return env
}

// loadEnvelope 从数据库加载一个信封
func (em *EnvelopeManager) loadEnvelope(envelopeID string) (*ExecutionEnvelope, error) {
	row := em.db.QueryRow(`SELECT id, trace_id, timestamp, domain, request_hash, decision, rules_json, sender_id, nonce, prev_hash, content_hash, signature FROM execution_envelopes WHERE id = ?`, envelopeID)
	return scanEnvelope(row)
}

// scanEnvelope 从 sql.Row 扫描一个信封
func scanEnvelope(row *sql.Row) (*ExecutionEnvelope, error) {
	var env ExecutionEnvelope
	var tsStr, rulesJSON string
	err := row.Scan(&env.ID, &env.TraceID, &tsStr, &env.Domain, &env.RequestHash, &env.Decision, &rulesJSON, &env.SenderID, &env.Nonce, &env.PrevHash, &env.ContentHash, &env.Signature)
	if err != nil {
		return nil, err
	}
	env.Timestamp, _ = time.Parse(time.RFC3339Nano, tsStr)
	json.Unmarshal([]byte(rulesJSON), &env.Rules)
	if env.Rules == nil {
		env.Rules = []string{}
	}
	return &env, nil
}

// scanEnvelopeRows 从 sql.Rows 扫描一个信封
func scanEnvelopeFromRows(rows *sql.Rows) (*ExecutionEnvelope, error) {
	var env ExecutionEnvelope
	var tsStr, rulesJSON string
	err := rows.Scan(&env.ID, &env.TraceID, &tsStr, &env.Domain, &env.RequestHash, &env.Decision, &rulesJSON, &env.SenderID, &env.Nonce, &env.PrevHash, &env.ContentHash, &env.Signature)
	if err != nil {
		return nil, err
	}
	env.Timestamp, _ = time.Parse(time.RFC3339Nano, tsStr)
	json.Unmarshal([]byte(rulesJSON), &env.Rules)
	if env.Rules == nil {
		env.Rules = []string{}
	}
	return &env, nil
}

// Verify 验证单个信封的完整性
func (em *EnvelopeManager) Verify(envelopeID string) (*VerifyResult, error) {
	env, err := em.loadEnvelope(envelopeID)
	if err != nil {
		return nil, fmt.Errorf("信封不存在: %s", envelopeID)
	}

	result := &VerifyResult{
		Valid:      true,
		EnvelopeID: envelopeID,
	}

	// 重新计算 ContentHash
	expectedHash := computeContentHash(env)
	if expectedHash != env.ContentHash {
		result.Valid = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("content_hash 不匹配: expected=%s, stored=%s", expectedHash, env.ContentHash))
	}

	// 重新计算 Signature
	expectedSig := computeSignature(env.ContentHash, em.secretKey)
	if expectedSig != env.Signature {
		result.Valid = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("signature 不匹配: expected=%s, stored=%s", expectedSig, env.Signature))
	}

	return result, nil
}

// VerifyChain 验证某个 trace_id 的信封链完整性
// v18.0+ 同时验证信封所在批次的 Merkle Root 完整性
func (em *EnvelopeManager) VerifyChain(traceID string) (*ChainVerifyResult, error) {
	envelopes, err := em.ListByTrace(traceID)
	if err != nil {
		return nil, err
	}

	result := &ChainVerifyResult{
		ChainValid: true,
		TraceID:    traceID,
		TotalCount: len(envelopes),
		BrokenAt:   -1,
	}

	if len(envelopes) == 0 {
		return result, nil
	}

	// 收集需要验证的批次 ID（去重）
	verifiedBatches := map[string]bool{}

	for i, env := range envelopes {
		// 验证每个信封的签名
		expectedHash := computeContentHash(&env)
		expectedSig := computeSignature(env.ContentHash, em.secretKey)

		if expectedHash != env.ContentHash || expectedSig != env.Signature {
			result.InvalidCount++
			result.ChainValid = false
			if result.BrokenAt == -1 {
				result.BrokenAt = i
			}
			result.Details = append(result.Details,
				fmt.Sprintf("信封 #%d (%s) 签名验证失败", i, env.ID))
			continue
		}
		result.ValidCount++

		// 检查链式结构：时序一致性
		if i > 0 {
			prevEnv := envelopes[i-1]
			if env.Timestamp.Before(prevEnv.Timestamp) {
				result.ChainValid = false
				if result.BrokenAt == -1 {
					result.BrokenAt = i
				}
				result.Details = append(result.Details,
					fmt.Sprintf("信封 #%d (%s) 时序异常: %s < %s", i, env.ID,
						env.Timestamp.Format(time.RFC3339Nano), prevEnv.Timestamp.Format(time.RFC3339Nano)))
			}
		}

		// v18.0+ 检查信封所在批次的 Merkle Root
		var batchID string
		em.db.QueryRow(`SELECT batch_id FROM execution_envelopes WHERE id = ?`, env.ID).Scan(&batchID)
		if batchID != "" && !verifiedBatches[batchID] {
			verifiedBatches[batchID] = true
			batchResult, err := em.VerifyBatch(batchID)
			if err != nil || !batchResult.Valid {
				result.ChainValid = false
				if result.BrokenAt == -1 {
					result.BrokenAt = i
				}
				reason := fmt.Sprintf("信封 #%d (%s) 所在批次 %s Merkle 验证失败", i, env.ID, batchID)
				if err != nil {
					reason += ": " + err.Error()
				}
				result.Details = append(result.Details, reason)
			}
		}
	}

	return result, nil
}

// ListByTrace 按 trace_id 查询信封列表（按时间排序）
// ListRecent returns the most recent envelopes (no filter)
func (em *EnvelopeManager) ListRecent(limit int) ([]ExecutionEnvelope, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := em.db.Query(
		`SELECT id, trace_id, timestamp, domain, request_hash, decision, rules_json, sender_id, nonce, prev_hash, content_hash, signature FROM execution_envelopes ORDER BY timestamp DESC, rowid DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envelopes []ExecutionEnvelope
	for rows.Next() {
		env, err := scanEnvelopeFromRows(rows)
		if err != nil {
			continue
		}
		envelopes = append(envelopes, *env)
	}
	return envelopes, nil
}

func (em *EnvelopeManager) ListByTrace(traceID string) ([]ExecutionEnvelope, error) {
	rows, err := em.db.Query(
		`SELECT id, trace_id, timestamp, domain, request_hash, decision, rules_json, sender_id, nonce, prev_hash, content_hash, signature FROM execution_envelopes WHERE trace_id = ? ORDER BY timestamp ASC, rowid ASC`,
		traceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envelopes []ExecutionEnvelope
	for rows.Next() {
		env, err := scanEnvelopeFromRows(rows)
		if err != nil {
			continue
		}
		envelopes = append(envelopes, *env)
	}
	if envelopes == nil {
		envelopes = []ExecutionEnvelope{}
	}
	return envelopes, nil
}

// Stats 返回信封统计信息
func (em *EnvelopeManager) Stats() map[string]interface{} {
	stats := map[string]interface{}{}

	// 总数
	var total int64
	em.db.QueryRow(`SELECT COUNT(*) FROM execution_envelopes`).Scan(&total)
	stats["total"] = total

	// 按 domain 统计
	domainStats := map[string]int64{}
	rows, err := em.db.Query(`SELECT domain, COUNT(*) FROM execution_envelopes GROUP BY domain`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var domain string
			var count int64
			if rows.Scan(&domain, &count) == nil {
				domainStats[domain] = count
			}
		}
	}
	stats["by_domain"] = domainStats

	// 按 decision 统计
	decisionStats := map[string]int64{}
	rows2, err := em.db.Query(`SELECT decision, COUNT(*) FROM execution_envelopes GROUP BY decision`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var decision string
			var count int64
			if rows2.Scan(&decision, &count) == nil {
				decisionStats[decision] = count
			}
		}
	}
	stats["by_decision"] = decisionStats

	// 最新信封时间
	var latestTS string
	if em.db.QueryRow(`SELECT timestamp FROM execution_envelopes ORDER BY timestamp DESC LIMIT 1`).Scan(&latestTS) == nil {
		stats["latest_timestamp"] = latestTS
	}

	// 唯一 trace 数
	var traceCount int64
	em.db.QueryRow(`SELECT COUNT(DISTINCT trace_id) FROM execution_envelopes`).Scan(&traceCount)
	stats["unique_traces"] = traceCount

	// v18.0+ Merkle 批次统计
	var batchCount int64
	em.db.QueryRow(`SELECT COUNT(*) FROM merkle_batches`).Scan(&batchCount)
	stats["merkle_batches"] = batchCount

	em.mu.Lock()
	stats["pending_leaves"] = int64(len(em.pendingLeaves))
	stats["batch_size"] = int64(em.batchSize)
	em.mu.Unlock()

	return stats
}

// UpdateSecretKey 热更新签名密钥（不影响已签名的信封）
func (em *EnvelopeManager) UpdateSecretKey(newKey string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.secretKey = newKey
}
