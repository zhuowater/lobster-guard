// llm_cache.go — LLM 响应缓存引擎（语义相似缓存 + 租户隔离 + 污染安全）
// lobster-guard v20.3
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// ============================================================
// 配置
// ============================================================

// LLMCacheConfig LLM 响应缓存配置
type LLMCacheConfig struct {
	Enabled         bool    `yaml:"enabled" json:"enabled"`
	MaxEntries      int     `yaml:"max_entries" json:"max_entries"`           // 最大缓存数（默认1000）
	TTLMinutes      int     `yaml:"ttl_minutes" json:"ttl_minutes"`           // 缓存 TTL（默认60）
	SimilarityMin   float64 `yaml:"similarity_min" json:"similarity_min"`     // 语义相似度阈值（默认0.85）
	TenantIsolation bool    `yaml:"tenant_isolation" json:"tenant_isolation"` // 租户隔离（默认true）
	SkipTainted     bool    `yaml:"skip_tainted" json:"skip_tainted"`         // 跳过被污染的响应（默认true）
}

// ============================================================
// 缓存条目
// ============================================================

// CacheEntry 缓存条目
type CacheEntry struct {
	Key         string             `json:"key"`
	Query       string             `json:"query"`
	QueryHash   string             `json:"query_hash"`
	QueryTokens map[string]float64 `json:"-"`
	Response    string             `json:"response"`
	Model       string             `json:"model"`
	TenantID    string             `json:"tenant_id"`
	CreatedAt   time.Time          `json:"created_at"`
	LastHit     time.Time          `json:"last_hit"`
	HitCount    int                `json:"hit_count"`
	TokensSaved int                `json:"tokens_saved"`
}

// ============================================================
// 缓存统计
// ============================================================

// CacheStats 缓存统计
type CacheStats struct {
	Enabled        bool           `json:"enabled"`
	TotalEntries   int            `json:"total_entries"`
	MaxEntries     int            `json:"max_entries"`
	TotalHits      int64          `json:"total_hits"`
	TotalMisses    int64          `json:"total_misses"`
	HitRate        float64        `json:"hit_rate"`
	TokensSaved    int64          `json:"tokens_saved"`
	CostSaved      float64        `json:"cost_saved_usd"`
	ByTenant       map[string]int `json:"by_tenant"`
	SkippedTainted int64          `json:"skipped_tainted"`
}

// ============================================================
// LLMCache 主结构
// ============================================================

// LLMCache LLM 响应缓存引擎
type LLMCache struct {
	db     *sql.DB
	mu     sync.RWMutex
	config LLMCacheConfig

	// 内存 LRU 缓存：key → CacheEntry
	entries map[string]*CacheEntry
	lruList []string // LRU 顺序（尾部=最近使用）
	maxSize int

	// 统计计数器（原子操作）
	totalHits      int64
	totalMisses    int64
	skippedTainted int64
}

// ============================================================
// 构造 & 初始化
// ============================================================

// NewLLMCache 创建 LLM 缓存引擎
func NewLLMCache(db *sql.DB, cfg LLMCacheConfig) *LLMCache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 1000
	}
	if cfg.TTLMinutes <= 0 {
		cfg.TTLMinutes = 60
	}
	if cfg.SimilarityMin <= 0 {
		cfg.SimilarityMin = 0.85
	}
	// 默认启用租户隔离和跳过污染
	if !cfg.Enabled {
		cfg.TenantIsolation = true
		cfg.SkipTainted = true
	}

	c := &LLMCache{
		db:      db,
		config:  cfg,
		entries: make(map[string]*CacheEntry),
		lruList: make([]string, 0),
		maxSize: cfg.MaxEntries,
	}

	c.initDB()
	c.loadFromDB()
	return c
}

// initDB 创建 SQLite 表
func (c *LLMCache) initDB() {
	if c.db == nil {
		return
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS llm_cache (
			key TEXT PRIMARY KEY,
			query TEXT NOT NULL,
			query_hash TEXT NOT NULL,
			response TEXT NOT NULL,
			model TEXT DEFAULT '',
			tenant_id TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			last_hit TEXT NOT NULL,
			hit_count INTEGER DEFAULT 0,
			tokens_saved INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_hash ON llm_cache(query_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_tenant ON llm_cache(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_created ON llm_cache(created_at)`,
	}
	for _, s := range stmts {
		if _, err := c.db.Exec(s); err != nil {
			log.Printf("[LLMCache] 初始化表失败: %v", err)
		}
	}
}

// loadFromDB 从 SQLite 加载缓存到内存
func (c *LLMCache) loadFromDB() {
	if c.db == nil {
		return
	}
	rows, err := c.db.Query(`SELECT key, query, query_hash, response, model, tenant_id, created_at, last_hit, hit_count, tokens_saved FROM llm_cache ORDER BY last_hit ASC`)
	if err != nil {
		log.Printf("[LLMCache] 加载缓存失败: %v", err)
		return
	}
	defer rows.Close()

	now := time.Now()
	ttl := time.Duration(c.config.TTLMinutes) * time.Minute
	count := 0

	for rows.Next() {
		var e CacheEntry
		var createdStr, lastHitStr string
		if err := rows.Scan(&e.Key, &e.Query, &e.QueryHash, &e.Response, &e.Model, &e.TenantID, &createdStr, &lastHitStr, &e.HitCount, &e.TokensSaved); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		e.LastHit, _ = time.Parse(time.RFC3339, lastHitStr)

		// 跳过过期条目
		if now.Sub(e.CreatedAt) > ttl {
			continue
		}

		e.QueryTokens = cacheTokenize(e.Query)
		c.entries[e.Key] = &e
		c.lruList = append(c.lruList, e.Key)
		count++
	}

	// 如果超过上限，只保留最近的
	for len(c.lruList) > c.maxSize {
		oldest := c.lruList[0]
		c.lruList = c.lruList[1:]
		delete(c.entries, oldest)
		count--
	}

	if count > 0 {
		log.Printf("[LLMCache] 从数据库加载 %d 条缓存", count)
	}
}

// ============================================================
// 核心方法
// ============================================================

// Lookup 查找缓存（先精确 → 再语义）
func (c *LLMCache) Lookup(query string, model string, tenantID string) (*CacheEntry, bool) {
	if !c.config.Enabled {
		atomic.AddInt64(&c.totalMisses, 1)
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	hash := cacheHash(query)

	// 1. 精确匹配：同 hash + 同 model + 同 tenant
	for _, e := range c.entries {
		if e.QueryHash != hash {
			continue
		}
		if e.Model != model {
			continue
		}
		if c.config.TenantIsolation && e.TenantID != tenantID {
			continue
		}
		if c.isExpired(e) {
			continue
		}
		c.touchLocked(e)
		atomic.AddInt64(&c.totalHits, 1)
		return c.cloneEntry(e), true
	}

	// 多轮对话上下文使用精确缓存，避免语义匹配把不同历史误判为同一请求。
	if isConversationCacheKey(query) {
		atomic.AddInt64(&c.totalMisses, 1)
		return nil, false
	}

	// 2. 语义相似匹配（同 model 空间内）
	queryTokens := cacheTokenize(query)
	var bestEntry *CacheEntry
	bestSim := 0.0

	for _, e := range c.entries {
		if e.Model != model {
			continue
		}
		if c.config.TenantIsolation && e.TenantID != tenantID {
			continue
		}
		if c.isExpired(e) {
			continue
		}
		sim := cacheCosineSim(queryTokens, e.QueryTokens)
		if sim >= c.config.SimilarityMin && sim > bestSim {
			bestSim = sim
			bestEntry = e
		}
	}

	if bestEntry != nil {
		c.touchLocked(bestEntry)
		atomic.AddInt64(&c.totalHits, 1)
		return c.cloneEntry(bestEntry), true
	}

	atomic.AddInt64(&c.totalMisses, 1)
	return nil, false
}

// Store 存储缓存条目
func (c *LLMCache) Store(query string, response string, model string, tenantID string, tainted bool) error {
	if !c.config.Enabled {
		return nil
	}

	// 跳过被污染的响应
	if c.config.SkipTainted && tainted {
		atomic.AddInt64(&c.skippedTainted, 1)
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	hash := cacheHash(query)
	key := fmt.Sprintf("%s:%s:%s", tenantID, model, hash)
	now := time.Now()

	// 估算 token 数（简单按字符估算）
	estimatedTokens := estimateTokens(response)

	entry := &CacheEntry{
		Key:         key,
		Query:       query,
		QueryHash:   hash,
		QueryTokens: cacheTokenize(query),
		Response:    response,
		Model:       model,
		TenantID:    tenantID,
		CreatedAt:   now,
		LastHit:     now,
		HitCount:    0,
		TokensSaved: estimatedTokens,
	}

	// LRU 淘汰：如果超过上限，移除最久没用的
	for len(c.entries) >= c.maxSize {
		c.evictOldestLocked()
	}

	// 如果已存在相同 key，先移除旧的
	if _, exists := c.entries[key]; exists {
		c.removeLRULocked(key)
	}

	// 写入内存
	c.entries[key] = entry
	c.lruList = append(c.lruList, key)

	// 写入 SQLite
	c.persistEntry(entry)

	return nil
}

// Invalidate 清除指定租户的缓存，返回清除数量
func (c *LLMCache) Invalidate(tenantID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	keysToRemove := make([]string, 0)

	for key, e := range c.entries {
		if e.TenantID == tenantID {
			keysToRemove = append(keysToRemove, key)
			count++
		}
	}

	for _, key := range keysToRemove {
		delete(c.entries, key)
		c.removeLRULocked(key)
	}

	// 从 SQLite 删除
	if c.db != nil {
		c.db.Exec(`DELETE FROM llm_cache WHERE tenant_id = ?`, tenantID)
	}

	return count
}

// InvalidateAll 清除全部缓存，返回清除数量
func (c *LLMCache) InvalidateAll() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.entries)
	c.entries = make(map[string]*CacheEntry)
	c.lruList = make([]string, 0)

	// 从 SQLite 删除
	if c.db != nil {
		c.db.Exec(`DELETE FROM llm_cache`)
	}

	return count
}

// Cleanup 清理过期条目（TTL）
func (c *LLMCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	keysToRemove := make([]string, 0)

	for key, e := range c.entries {
		if c.isExpired(e) {
			keysToRemove = append(keysToRemove, key)
			count++
		}
	}

	for _, key := range keysToRemove {
		delete(c.entries, key)
		c.removeLRULocked(key)
	}

	// 从 SQLite 删除过期的
	if c.db != nil {
		cutoff := time.Now().Add(-time.Duration(c.config.TTLMinutes) * time.Minute).Format(time.RFC3339)
		c.db.Exec(`DELETE FROM llm_cache WHERE created_at < ?`, cutoff)
	}

	return count
}

// Stats 返回缓存统计
func (c *LLMCache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits := atomic.LoadInt64(&c.totalHits)
	misses := atomic.LoadInt64(&c.totalMisses)
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	var tokensSaved int64
	byTenant := make(map[string]int)
	for _, e := range c.entries {
		tokensSaved += int64(e.TokensSaved) * int64(e.HitCount)
		byTenant[e.TenantID]++
	}

	// 估算节省成本：$3/M input tokens (Sonnet) 近似
	costSaved := float64(tokensSaved) / 1000000.0 * 3.0

	return &CacheStats{
		Enabled:        c.config.Enabled,
		TotalEntries:   len(c.entries),
		MaxEntries:     c.maxSize,
		TotalHits:      hits,
		TotalMisses:    misses,
		HitRate:        hitRate,
		TokensSaved:    tokensSaved,
		CostSaved:      costSaved,
		ByTenant:       byTenant,
		SkippedTainted: atomic.LoadInt64(&c.skippedTainted),
	}
}

// GetConfig 获取当前配置
func (c *LLMCache) GetConfig() LLMCacheConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// UpdateConfig 更新配置
func (c *LLMCache) UpdateConfig(cfg LLMCacheConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cfg.MaxEntries > 0 {
		c.config.MaxEntries = cfg.MaxEntries
		c.maxSize = cfg.MaxEntries
	}
	if cfg.TTLMinutes > 0 {
		c.config.TTLMinutes = cfg.TTLMinutes
	}
	if cfg.SimilarityMin > 0 {
		c.config.SimilarityMin = cfg.SimilarityMin
	}
	// 布尔值直接更新
	c.config.TenantIsolation = cfg.TenantIsolation
	c.config.SkipTainted = cfg.SkipTainted
	c.config.Enabled = cfg.Enabled
}

// ListEntries 列出缓存条目（支持租户过滤和分页）
func (c *LLMCache) ListEntries(tenantID string, limit int) []*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	result := make([]*CacheEntry, 0)
	for _, e := range c.entries {
		if tenantID != "" && e.TenantID != tenantID {
			continue
		}
		if c.isExpired(e) {
			continue
		}
		result = append(result, c.cloneEntry(e))
		if len(result) >= limit {
			break
		}
	}
	return result
}

// TestLookup 测试查询（不更新统计）
func (c *LLMCache) TestLookup(query string, tenantID string) (*CacheEntry, float64, bool) {
	if !c.config.Enabled {
		return nil, 0, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	hash := cacheHash(query)

	// 精确匹配
	for _, e := range c.entries {
		if e.QueryHash == hash {
			if c.config.TenantIsolation && e.TenantID != tenantID {
				continue
			}
			if c.isExpired(e) {
				continue
			}
			return c.cloneEntry(e), 1.0, true
		}
	}

	// 语义匹配
	queryTokens := cacheTokenize(query)
	var bestEntry *CacheEntry
	bestSim := 0.0

	for _, e := range c.entries {
		if c.config.TenantIsolation && e.TenantID != tenantID {
			continue
		}
		if c.isExpired(e) {
			continue
		}
		sim := cacheCosineSim(queryTokens, e.QueryTokens)
		if sim >= c.config.SimilarityMin && sim > bestSim {
			bestSim = sim
			bestEntry = e
		}
	}

	if bestEntry != nil {
		return c.cloneEntry(bestEntry), bestSim, true
	}

	return nil, 0, false
}

// ============================================================
// 内部方法
// ============================================================

// isExpired 检查条目是否过期
func (c *LLMCache) isExpired(e *CacheEntry) bool {
	ttl := time.Duration(c.config.TTLMinutes) * time.Minute
	return time.Since(e.CreatedAt) > ttl
}

// touchLocked 更新命中统计（调用者须持有写锁）
func (c *LLMCache) touchLocked(e *CacheEntry) {
	e.HitCount++
	e.LastHit = time.Now()

	// 更新 LRU 位置：移到尾部
	c.removeLRULocked(e.Key)
	c.lruList = append(c.lruList, e.Key)

	// 更新 SQLite
	if c.db != nil {
		go func(key string, hitCount int, lastHit string) {
			c.db.Exec(`UPDATE llm_cache SET hit_count = ?, last_hit = ? WHERE key = ?`,
				hitCount, lastHit, key)
		}(e.Key, e.HitCount, e.LastHit.Format(time.RFC3339))
	}
}

// evictOldestLocked 淘汰 LRU 头部（调用者须持有写锁）
func (c *LLMCache) evictOldestLocked() {
	if len(c.lruList) == 0 {
		return
	}
	oldest := c.lruList[0]
	c.lruList = c.lruList[1:]
	delete(c.entries, oldest)

	// 从 SQLite 删除
	if c.db != nil {
		go func(key string) {
			c.db.Exec(`DELETE FROM llm_cache WHERE key = ?`, key)
		}(oldest)
	}
}

// removeLRULocked 从 LRU 列表中移除指定 key（调用者须持有写锁）
func (c *LLMCache) removeLRULocked(key string) {
	for i, k := range c.lruList {
		if k == key {
			c.lruList = append(c.lruList[:i], c.lruList[i+1:]...)
			return
		}
	}
}

// cloneEntry 克隆缓存条目（避免外部修改内部状态）
func (c *LLMCache) cloneEntry(e *CacheEntry) *CacheEntry {
	return &CacheEntry{
		Key:         e.Key,
		Query:       e.Query,
		QueryHash:   e.QueryHash,
		Response:    e.Response,
		Model:       e.Model,
		TenantID:    e.TenantID,
		CreatedAt:   e.CreatedAt,
		LastHit:     e.LastHit,
		HitCount:    e.HitCount,
		TokensSaved: e.TokensSaved,
	}
}

// persistEntry 写入 SQLite
func (c *LLMCache) persistEntry(e *CacheEntry) {
	if c.db == nil {
		return
	}
	go func() {
		_, err := c.db.Exec(
			`INSERT OR REPLACE INTO llm_cache (key, query, query_hash, response, model, tenant_id, created_at, last_hit, hit_count, tokens_saved) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Key, e.Query, e.QueryHash, e.Response, e.Model, e.TenantID,
			e.CreatedAt.Format(time.RFC3339), e.LastHit.Format(time.RFC3339),
			e.HitCount, e.TokensSaved,
		)
		if err != nil {
			log.Printf("[LLMCache] 持久化失败: %v", err)
		}
	}()
}

// ============================================================
// 工具函数（缓存专用，不依赖 semantic_detector）
// ============================================================

// cacheHash 计算查询的 SHA256 哈希
func cacheHash(query string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(query))))
	return hex.EncodeToString(h[:])
}

// cacheTokenize Unicode 感知分词（中文逐字 + 英文按词）
func cacheTokenize(text string) map[string]float64 {
	text = strings.ToLower(text)
	tokens := make([]string, 0)
	var cur []rune

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			if len(cur) > 0 {
				tokens = append(tokens, string(cur))
				cur = cur[:0]
			}
			tokens = append(tokens, string(r))
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur = append(cur, r)
		} else {
			if len(cur) > 0 {
				tokens = append(tokens, string(cur))
				cur = cur[:0]
			}
		}
	}
	if len(cur) > 0 {
		tokens = append(tokens, string(cur))
	}

	if len(tokens) == 0 {
		return map[string]float64{}
	}

	// 简化版 TF 向量
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}

	total := float64(len(tokens))
	vec := make(map[string]float64, len(tf))
	for w, c := range tf {
		// TF-IDF 简化：TF * log(常数 IDF)
		// 因为缓存中查询通常不多，使用简单的 TF 归一化即可达到语义匹配效果
		vec[w] = float64(c) / total
	}
	return vec
}

// cacheCosineSim 余弦相似度
func cacheCosineSim(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, nA, nB float64
	for k, va := range a {
		nA += va * va
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	for _, vb := range b {
		nB += vb * vb
	}
	d := math.Sqrt(nA) * math.Sqrt(nB)
	if d == 0 {
		return 0
	}
	return dot / d
}

func isConversationCacheKey(query string) bool {
	return strings.Contains(query, "\n") ||
		strings.Contains(query, "system:") ||
		strings.Contains(query, "assistant:") ||
		strings.Contains(query, "tool:")
}

// extractUserQuery 从 LLM 请求体中提取用户查询文本
// 支持 Anthropic 格式和 OpenAI 格式
func extractUserQuery(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	msgs, ok := req["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return ""
	}

	parts := make([]string, 0, len(msgs))
	for _, rawMsg := range msgs {
		msg, ok := rawMsg.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		content := extractCacheMessageContent(msg["content"])
		toolCallID, _ := msg["tool_call_id"].(string)
		if toolCallID != "" {
			content = content + " tool_call_id=" + toolCallID
		}
		if role == "" && content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", role, content))
	}
	return strings.Join(parts, "\n")
}

func extractCacheMessageContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			typ, _ := itemMap["type"].(string)
			switch typ {
			case "text", "input_text", "output_text", "text_delta":
				if text, ok := itemMap["text"].(string); ok && text != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			case "tool_result", "tool_use", "tool_call":
				if name, ok := itemMap["name"].(string); ok && name != "" {
					parts = append(parts, typ+":"+name)
				} else {
					parts = append(parts, typ)
				}
				if input, ok := itemMap["input"]; ok {
					if b, err := json.Marshal(input); err == nil {
						parts = append(parts, string(b))
					}
				}
				if text, ok := itemMap["text"].(string); ok && text != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			default:
				if text, ok := itemMap["text"].(string); ok && text != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
				if len(parts) == 0 {
					if b, err := json.Marshal(itemMap); err == nil {
						parts = append(parts, string(b))
					}
				}
			}
		}
		return strings.Join(parts, " ")
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}

// estimateTokens 估算响应的 token 数
// 英文约 4 字符 = 1 token，中文约 1.5 字符 = 1 token
func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	hanCount := 0
	otherCount := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			hanCount++
		} else {
			otherCount++
		}
	}
	return hanCount*2/3 + otherCount/4 + 1
}
