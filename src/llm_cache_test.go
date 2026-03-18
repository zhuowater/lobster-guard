// llm_cache_test.go — LLM 响应缓存引擎测试
// lobster-guard v20.3
package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestCache 创建测试用缓存实例
func setupTestCache(t *testing.T, cfg LLMCacheConfig) (*LLMCache, *sql.DB) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		cfg.Enabled = true
	}
	if cfg.MaxEntries == 0 {
		cfg.MaxEntries = 100
	}
	if cfg.TTLMinutes == 0 {
		cfg.TTLMinutes = 60
	}
	if cfg.SimilarityMin == 0 {
		cfg.SimilarityMin = 0.85
	}
	cache := NewLLMCache(db, cfg)
	return cache, db
}

// ============================================================
// 1. TestCacheStoreAndLookup — 存储+精确命中
// ============================================================
func TestCacheStoreAndLookup(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储
	err := cache.Store("What is the capital of France?", `{"answer":"Paris"}`, "gpt-4", "tenant-a", false)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// 等待内存写入完成
	time.Sleep(10 * time.Millisecond)

	// 精确查找
	entry, hit := cache.Lookup("What is the capital of France?", "gpt-4", "tenant-a")
	if !hit {
		t.Fatal("Expected cache hit but got miss")
	}
	if entry.Response != `{"answer":"Paris"}` {
		t.Fatalf("Unexpected response: %s", entry.Response)
	}
	if entry.TenantID != "tenant-a" {
		t.Fatalf("Unexpected tenant_id: %s", entry.TenantID)
	}
}

// ============================================================
// 2. TestCacheSemanticMatch — 语义相似命中
// ============================================================
func TestCacheSemanticMatch(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
		SimilarityMin:   0.70, // 较低阈值以便测试
	})
	defer db.Close()

	// 存储一个查询
	cache.Store("What is the capital of France?", `{"answer":"Paris"}`, "gpt-4", "tenant-a", false)

	// 用近似文本查找（略微不同的措辞但含义相似）
	entry, hit := cache.Lookup("what is the capital of france", "gpt-4", "tenant-a")
	if !hit {
		t.Fatal("Expected semantic cache hit but got miss")
	}
	if entry.Response != `{"answer":"Paris"}` {
		t.Fatalf("Unexpected response: %s", entry.Response)
	}
}

// ============================================================
// 3. TestCacheMiss — 未命中
// ============================================================
func TestCacheMiss(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储一个查询
	cache.Store("What is the capital of France?", `{"answer":"Paris"}`, "gpt-4", "tenant-a", false)

	// 查找完全不同的内容
	entry, hit := cache.Lookup("How does quantum computing work?", "gpt-4", "tenant-a")
	if hit {
		t.Fatalf("Expected cache miss but got hit: %+v", entry)
	}
}

// ============================================================
// 4. TestCacheTenantIsolation — 租户隔离
// ============================================================
func TestCacheTenantIsolation(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 租户 A 存储
	cache.Store("What is the capital of France?", `{"answer":"Paris"}`, "gpt-4", "tenant-a", false)

	// 租户 B 查找同一查询 — 不应命中
	entry, hit := cache.Lookup("What is the capital of France?", "gpt-4", "tenant-b")
	if hit {
		t.Fatalf("Tenant isolation violated! tenant-b got tenant-a's cache: %+v", entry)
	}

	// 租户 A 查找 — 应该命中
	entry, hit = cache.Lookup("What is the capital of France?", "gpt-4", "tenant-a")
	if !hit {
		t.Fatal("Expected cache hit for tenant-a but got miss")
	}

	// 关闭租户隔离后，租户 B 应能命中
	cache.UpdateConfig(LLMCacheConfig{
		Enabled:         true,
		TenantIsolation: false,
		SkipTainted:     true,
	})
	entry, hit = cache.Lookup("What is the capital of France?", "gpt-4", "tenant-b")
	if !hit {
		t.Fatal("Expected cache hit with tenant isolation disabled but got miss")
	}
}

// ============================================================
// 5. TestCacheSkipTainted — 污染跳过
// ============================================================
func TestCacheSkipTainted(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储被污染的响应 — 应该被跳过
	cache.Store("Show me the secret key", `{"key":"sk-xxx"}`, "gpt-4", "tenant-a", true)

	// 查找 — 应该未命中
	_, hit := cache.Lookup("Show me the secret key", "gpt-4", "tenant-a")
	if hit {
		t.Fatal("Tainted response should not be cached")
	}

	// 验证 skippedTainted 计数
	stats := cache.Stats()
	if stats.SkippedTainted != 1 {
		t.Fatalf("Expected skippedTainted=1, got %d", stats.SkippedTainted)
	}

	// 关闭 SkipTainted 后可以存储
	cache.UpdateConfig(LLMCacheConfig{
		Enabled:         true,
		TenantIsolation: true,
		SkipTainted:     false,
	})
	cache.Store("Show me the secret key v2", `{"key":"sk-yyy"}`, "gpt-4", "tenant-a", true)
	_, hit = cache.Lookup("Show me the secret key v2", "gpt-4", "tenant-a")
	if !hit {
		t.Fatal("Expected cache hit when SkipTainted is disabled")
	}
}

// ============================================================
// 6. TestCacheLRUEviction — LRU 淘汰
// ============================================================
func TestCacheLRUEviction(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		MaxEntries:      3, // 只保留 3 个
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储 4 个条目（超过容量）
	cache.Store("query 1", "response 1", "gpt-4", "t1", false)
	cache.Store("query 2", "response 2", "gpt-4", "t1", false)
	cache.Store("query 3", "response 3", "gpt-4", "t1", false)
	cache.Store("query 4", "response 4", "gpt-4", "t1", false)

	// 第一个应该被淘汰
	_, hit := cache.Lookup("query 1", "gpt-4", "t1")
	if hit {
		t.Fatal("query 1 should have been evicted")
	}

	// 最后三个应该在
	for _, q := range []string{"query 2", "query 3", "query 4"} {
		_, hit := cache.Lookup(q, "gpt-4", "t1")
		if !hit {
			t.Fatalf("%s should be in cache", q)
		}
	}

	// 验证条目数
	stats := cache.Stats()
	if stats.TotalEntries != 3 {
		t.Fatalf("Expected 3 entries, got %d", stats.TotalEntries)
	}
}

// ============================================================
// 7. TestCacheTTLExpiry — TTL 过期
// ============================================================
func TestCacheTTLExpiry(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TTLMinutes:      1, // 1 分钟 TTL
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储
	cache.Store("test query", "test response", "gpt-4", "t1", false)

	// 立即查找 — 应命中
	_, hit := cache.Lookup("test query", "gpt-4", "t1")
	if !hit {
		t.Fatal("Expected cache hit before TTL expiry")
	}

	// 手动修改 CreatedAt 为过去
	cache.mu.Lock()
	for _, e := range cache.entries {
		e.CreatedAt = time.Now().Add(-2 * time.Minute)
	}
	cache.mu.Unlock()

	// 查找 — 应未命中（TTL 过期）
	_, hit = cache.Lookup("test query", "gpt-4", "t1")
	if hit {
		t.Fatal("Expected cache miss after TTL expiry")
	}

	// Cleanup 应该清除过期条目
	cleaned := cache.Cleanup()
	if cleaned != 1 {
		t.Fatalf("Expected 1 cleaned, got %d", cleaned)
	}

	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Fatalf("Expected 0 entries after cleanup, got %d", stats.TotalEntries)
	}
}

// ============================================================
// 8. TestCacheInvalidate — 清除指定租户
// ============================================================
func TestCacheInvalidate(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储多租户数据
	cache.Store("query a1", "response a1", "gpt-4", "tenant-a", false)
	cache.Store("query a2", "response a2", "gpt-4", "tenant-a", false)
	cache.Store("query b1", "response b1", "gpt-4", "tenant-b", false)

	// 清除 tenant-a
	cleared := cache.Invalidate("tenant-a")
	if cleared != 2 {
		t.Fatalf("Expected 2 cleared, got %d", cleared)
	}

	// tenant-a 的缓存应该消失
	_, hit := cache.Lookup("query a1", "gpt-4", "tenant-a")
	if hit {
		t.Fatal("tenant-a cache should be invalidated")
	}

	// tenant-b 的缓存应该还在
	_, hit = cache.Lookup("query b1", "gpt-4", "tenant-b")
	if !hit {
		t.Fatal("tenant-b cache should still exist")
	}

	// InvalidateAll
	cache.Store("query c1", "response c1", "gpt-4", "tenant-c", false)
	allCleared := cache.InvalidateAll()
	if allCleared < 1 {
		t.Fatalf("Expected at least 1 cleared, got %d", allCleared)
	}

	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Fatalf("Expected 0 entries after InvalidateAll, got %d", stats.TotalEntries)
	}
}

// ============================================================
// 9. TestCacheStats — 统计（命中率+节省）
// ============================================================
func TestCacheStats(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 初始统计
	stats := cache.Stats()
	if stats.TotalHits != 0 || stats.TotalMisses != 0 {
		t.Fatal("Initial stats should be zero")
	}
	if stats.HitRate != 0 {
		t.Fatal("Initial hit rate should be 0")
	}

	// 存储
	cache.Store("test query", "long response text for testing", "gpt-4", "tenant-a", false)

	// 命中
	cache.Lookup("test query", "gpt-4", "tenant-a")
	// 未命中
	cache.Lookup("different query", "gpt-4", "tenant-a")

	stats = cache.Stats()
	if stats.TotalHits != 1 {
		t.Fatalf("Expected 1 hit, got %d", stats.TotalHits)
	}
	if stats.TotalMisses != 1 {
		t.Fatalf("Expected 1 miss, got %d", stats.TotalMisses)
	}
	if stats.HitRate != 0.5 {
		t.Fatalf("Expected 50%% hit rate, got %.2f", stats.HitRate)
	}
	if stats.ByTenant["tenant-a"] != 1 {
		t.Fatalf("Expected 1 entry for tenant-a, got %d", stats.ByTenant["tenant-a"])
	}
}

// ============================================================
// 10. TestCacheHitCount — 命中计数
// ============================================================
func TestCacheHitCount(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	cache.Store("test query", "test response", "gpt-4", "t1", false)

	// 多次命中
	for i := 0; i < 5; i++ {
		entry, hit := cache.Lookup("test query", "gpt-4", "t1")
		if !hit {
			t.Fatalf("Expected hit on iteration %d", i)
		}
		// 返回的是克隆，hit_count 反映当前命中时的计数
		if entry.HitCount != i+1 {
			t.Fatalf("Expected HitCount=%d, got %d", i+1, entry.HitCount)
		}
	}

	// 验证内部计数
	cache.mu.RLock()
	for _, e := range cache.entries {
		if e.HitCount != 5 {
			t.Fatalf("Internal HitCount should be 5, got %d", e.HitCount)
		}
	}
	cache.mu.RUnlock()
}

// ============================================================
// 11. TestCacheTokensSaved — token 节省估算
// ============================================================
func TestCacheTokensSaved(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 存储一个有明确 token 估算的响应
	longResponse := "This is a test response that should be around twenty tokens or more in estimation."
	cache.Store("test query", longResponse, "gpt-4", "t1", false)

	// 命中 3 次
	for i := 0; i < 3; i++ {
		cache.Lookup("test query", "gpt-4", "t1")
	}

	stats := cache.Stats()
	if stats.TokensSaved <= 0 {
		t.Fatalf("Expected positive tokens saved, got %d", stats.TokensSaved)
	}

	// 验证估算函数
	tokens := estimateTokens(longResponse)
	if tokens <= 0 {
		t.Fatal("estimateTokens should return positive value for non-empty text")
	}

	// 节省成本应该大于 0
	if stats.CostSaved < 0 {
		t.Fatalf("Expected non-negative cost saved, got %f", stats.CostSaved)
	}
}

// ============================================================
// 12. TestCacheConfig — 配置更新
// ============================================================
func TestCacheConfig(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		MaxEntries:      100,
		TTLMinutes:      60,
		SimilarityMin:   0.85,
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	// 获取初始配置
	cfg := cache.GetConfig()
	if cfg.MaxEntries != 100 {
		t.Fatalf("Expected MaxEntries=100, got %d", cfg.MaxEntries)
	}
	if cfg.TTLMinutes != 60 {
		t.Fatalf("Expected TTLMinutes=60, got %d", cfg.TTLMinutes)
	}
	if cfg.SimilarityMin != 0.85 {
		t.Fatalf("Expected SimilarityMin=0.85, got %f", cfg.SimilarityMin)
	}

	// 更新配置
	cache.UpdateConfig(LLMCacheConfig{
		Enabled:         true,
		MaxEntries:      200,
		TTLMinutes:      120,
		SimilarityMin:   0.90,
		TenantIsolation: false,
		SkipTainted:     false,
	})

	cfg = cache.GetConfig()
	if cfg.MaxEntries != 200 {
		t.Fatalf("Expected MaxEntries=200, got %d", cfg.MaxEntries)
	}
	if cfg.TTLMinutes != 120 {
		t.Fatalf("Expected TTLMinutes=120, got %d", cfg.TTLMinutes)
	}
	if cfg.SimilarityMin != 0.90 {
		t.Fatalf("Expected SimilarityMin=0.90, got %f", cfg.SimilarityMin)
	}
	if cfg.TenantIsolation {
		t.Fatal("Expected TenantIsolation=false")
	}
	if cfg.SkipTainted {
		t.Fatal("Expected SkipTainted=false")
	}
}

// ============================================================
// 13. TestCacheDisabled — 缓存禁用时的行为
// ============================================================
func TestCacheDisabled(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cache := NewLLMCache(db, LLMCacheConfig{
		Enabled:    false,
		MaxEntries: 100,
		TTLMinutes: 60,
	})

	// 存储应该无操作
	err = cache.Store("test", "response", "gpt-4", "t1", false)
	if err != nil {
		t.Fatalf("Store should not error when disabled: %v", err)
	}

	// 查找应该返回 miss
	_, hit := cache.Lookup("test", "gpt-4", "t1")
	if hit {
		t.Fatal("Lookup should miss when cache is disabled")
	}

	stats := cache.Stats()
	if stats.Enabled {
		t.Fatal("Stats should show disabled")
	}
}

// ============================================================
// 14. TestCacheExtractUserQuery — 提取用户查询
// ============================================================
func TestCacheExtractUserQuery(t *testing.T) {
	// Anthropic 格式
	body1 := `{"model":"claude-3","messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"Hello world"}]}`
	q1 := extractUserQuery([]byte(body1))
	if q1 != "Hello world" {
		t.Fatalf("Expected 'Hello world', got '%s'", q1)
	}

	// OpenAI 格式
	body2 := `{"model":"gpt-4","messages":[{"role":"user","content":"What is AI?"},{"role":"assistant","content":"AI is..."},{"role":"user","content":"Tell me more"}]}`
	q2 := extractUserQuery([]byte(body2))
	if q2 != "Tell me more" {
		t.Fatalf("Expected 'Tell me more', got '%s'", q2)
	}

	// 空消息
	body3 := `{"model":"gpt-4"}`
	q3 := extractUserQuery([]byte(body3))
	if q3 != "" {
		t.Fatalf("Expected empty, got '%s'", q3)
	}

	// 无效 JSON
	q4 := extractUserQuery([]byte("not json"))
	if q4 != "" {
		t.Fatalf("Expected empty for invalid JSON, got '%s'", q4)
	}
}

// ============================================================
// 15. TestCacheListEntries — 列表和过滤
// ============================================================
func TestCacheListEntries(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	cache.Store("q1", "r1", "gpt-4", "tenant-a", false)
	cache.Store("q2", "r2", "gpt-4", "tenant-a", false)
	cache.Store("q3", "r3", "gpt-4", "tenant-b", false)

	// 列出全部
	all := cache.ListEntries("", 100)
	if len(all) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(all))
	}

	// 过滤 tenant-a
	filtered := cache.ListEntries("tenant-a", 100)
	if len(filtered) != 2 {
		t.Fatalf("Expected 2 entries for tenant-a, got %d", len(filtered))
	}

	// 限制数量
	limited := cache.ListEntries("", 1)
	if len(limited) != 1 {
		t.Fatalf("Expected 1 entry with limit=1, got %d", len(limited))
	}
}

// ============================================================
// 16. TestCacheTestLookup — 测试查询（不更新统计）
// ============================================================
func TestCacheTestLookup(t *testing.T) {
	cache, db := setupTestCache(t, LLMCacheConfig{
		TenantIsolation: true,
		SkipTainted:     true,
	})
	defer db.Close()

	cache.Store("test query", "test response", "gpt-4", "t1", false)

	// TestLookup 不应更新 hit 统计
	entry, similarity, hit := cache.TestLookup("test query", "t1")
	if !hit {
		t.Fatal("Expected test lookup hit")
	}
	if similarity != 1.0 {
		t.Fatalf("Expected similarity 1.0 for exact match, got %f", similarity)
	}
	if entry.Response != "test response" {
		t.Fatalf("Unexpected response: %s", entry.Response)
	}

	// 验证统计没有被更新（TestLookup 不更新 totalHits/totalMisses）
	stats := cache.Stats()
	if stats.TotalHits != 0 {
		t.Fatalf("TestLookup should not update hit stats, got %d", stats.TotalHits)
	}
}

// ============================================================
// 辅助函数测试
// ============================================================

func TestCacheHash(t *testing.T) {
	h1 := cacheHash("hello world")
	h2 := cacheHash("Hello World")  // 大小写标准化
	h3 := cacheHash("hello world ") // 去掉尾部空格

	// 大小写应该不敏感
	if h1 != h2 {
		t.Fatal("cacheHash should be case-insensitive")
	}
	// 尾部空格应该被 trim
	if h1 != h3 {
		t.Fatal("cacheHash should trim whitespace")
	}
}

func TestCacheCosineSim(t *testing.T) {
	a := map[string]float64{"hello": 0.5, "world": 0.5}
	b := map[string]float64{"hello": 0.5, "world": 0.5}
	c := map[string]float64{"foo": 0.5, "bar": 0.5}

	sim1 := cacheCosineSim(a, b)
	if sim1 < 0.99 {
		t.Fatalf("Identical vectors should have sim ~1.0, got %f", sim1)
	}

	sim2 := cacheCosineSim(a, c)
	if sim2 != 0 {
		t.Fatalf("Orthogonal vectors should have sim 0, got %f", sim2)
	}

	sim3 := cacheCosineSim(map[string]float64{}, b)
	if sim3 != 0 {
		t.Fatal("Empty vector should have sim 0")
	}
}

func TestEstimateTokens(t *testing.T) {
	// 空文本
	if estimateTokens("") != 0 {
		t.Fatal("Empty text should estimate 0 tokens")
	}

	// 英文
	english := "Hello world this is a test"
	tokens := estimateTokens(english)
	if tokens <= 0 {
		t.Fatal("English text should estimate positive tokens")
	}

	// 中文
	chinese := "你好世界这是一个测试"
	tokens2 := estimateTokens(chinese)
	if tokens2 <= 0 {
		t.Fatal("Chinese text should estimate positive tokens")
	}
}
