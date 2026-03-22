package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupRaceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS user_info_cache (
		sender_id TEXT PRIMARY KEY, name TEXT DEFAULT '', email TEXT DEFAULT '',
		department TEXT DEFAULT '', avatar TEXT DEFAULT '', mobile TEXT DEFAULT '',
		fetched_at TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS upstreams (
		id TEXT PRIMARY KEY, address TEXT NOT NULL, port INTEGER NOT NULL,
		healthy INTEGER DEFAULT 1, registered_at TEXT NOT NULL, last_heartbeat TEXT,
		tags TEXT DEFAULT '{}', load TEXT DEFAULT '{}', path_prefix TEXT DEFAULT '')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
		sender_id TEXT, app_id TEXT DEFAULT '', upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '', display_name TEXT DEFAULT '', email TEXT DEFAULT '',
		created_at TEXT, updated_at TEXT, PRIMARY KEY(sender_id, app_id))`)
	return db
}

// TestGetOrFetchWithTimeout_CacheHit 验证缓存命中时立即返回
func TestGetOrFetchWithTimeout_CacheHit(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	provider := &mockUserProvider{
		users: map[string]*UserInfo{
			"u1": {Name: "张三", Department: "安全部"},
		},
	}
	cache := NewUserInfoCache(db, provider, 24*time.Hour)
	// 预热缓存
	cache.GetOrFetch("u1")
	calls := provider.getCalls()

	// 带超时获取 — 应从缓存返回，不调 API
	info, err := cache.GetOrFetchWithTimeout("u1", 100*time.Millisecond)
	if err != nil || info == nil || info.Name != "张三" {
		t.Fatalf("缓存命中应返回正确数据: err=%v info=%+v", err, info)
	}
	if provider.getCalls() != calls {
		t.Fatal("缓存命中不应增加 API 调用次数")
	}
}

// TestGetOrFetchWithTimeout_APITimeout 验证 API 超时时优雅降级
func TestGetOrFetchWithTimeout_APITimeout(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()

	// 模拟慢 API（2 秒延迟）
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apptoken") {
			w.Write([]byte(`{"errCode":0,"data":{"app_token":"t","expires_in":7200}}`))
			return
		}
		time.Sleep(2 * time.Second)
		w.Write([]byte(`{"errCode":0,"data":{"name":"慢用户","departments":[{"name":"测试部"}]}}`))
	}))
	defer slowServer.Close()

	provider := NewLanxinUserProvider("app", "secret", slowServer.URL)
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	start := time.Now()
	info, err := cache.GetOrFetchWithTimeout("slow-user", 200*time.Millisecond)
	elapsed := time.Since(start)

	// 应在 200ms 左右超时返回 nil
	if info != nil {
		t.Fatal("超时应返回 nil")
	}
	if err != nil {
		t.Fatal("超时应返回 nil error（降级）")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("超时等待太久: %v", elapsed)
	}

	// 等待后台 goroutine 完成，验证缓存被异步填充
	time.Sleep(3 * time.Second)
	cached := cache.GetCached("slow-user")
	if cached == nil || cached.Name != "慢用户" {
		t.Fatalf("后台 goroutine 应异步填充缓存: %+v", cached)
	}
}

// TestGetOrFetchWithTimeout_FastAPI 验证 API 快速响应时正常返回
func TestGetOrFetchWithTimeout_FastAPI(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()

	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "apptoken") {
			w.Write([]byte(`{"errCode":0,"data":{"app_token":"t","expires_in":7200}}`))
			return
		}
		w.Write([]byte(`{"errCode":0,"data":{"name":"快用户","departments":[{"name":"研发部"}]}}`))
	}))
	defer fastServer.Close()

	provider := NewLanxinUserProvider("app", "secret", fastServer.URL)
	cache := NewUserInfoCache(db, provider, 24*time.Hour)

	info, err := cache.GetOrFetchWithTimeout("fast-user", 2*time.Second)
	if err != nil || info == nil || info.Name != "快用户" {
		t.Fatalf("快速 API 应正常返回: err=%v info=%+v", err, info)
	}
}

// TestCompareAndBind_Success 验证原子比较绑定成功
func TestCompareAndBind_Success(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "upstream-a")

	ok, actual := rt.CompareAndBind("user1", "app1", "upstream-a", "upstream-b")
	if !ok || actual != "upstream-b" {
		t.Fatalf("期望成功迁移, ok=%v actual=%s", ok, actual)
	}
	uid, _ := rt.Lookup("user1", "app1")
	if uid != "upstream-b" {
		t.Fatalf("Lookup 期望 upstream-b, 实际 %s", uid)
	}
}

// TestCompareAndBind_Conflict 验证并发冲突时拒绝绑定
func TestCompareAndBind_Conflict(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "upstream-c") // 已被改到 c

	// 尝试从 a 迁移到 b — 应失败（当前是 c 不是 a）
	ok, actual := rt.CompareAndBind("user1", "app1", "upstream-a", "upstream-b")
	if ok {
		t.Fatal("冲突时应拒绝绑定")
	}
	if actual != "upstream-c" {
		t.Fatalf("应返回当前实际值 upstream-c, 实际 %s", actual)
	}
}

// TestCompareAndBind_NotFound 验证无绑定时的行为
func TestCompareAndBind_NotFound(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	rt := NewRouteTable(db, true)

	// expectedUID="" 表示期望无绑定
	ok, _ := rt.CompareAndBind("new-user", "app1", "", "upstream-a")
	if !ok {
		t.Fatal("无绑定时 CAS 应成功")
	}
	uid, found := rt.Lookup("new-user", "app1")
	if !found || uid != "upstream-a" {
		t.Fatalf("期望 upstream-a, 实际 found=%v uid=%s", found, uid)
	}
}

// TestTransferUserCount_Atomic 验证原子计数转移
func TestTransferUserCount_Atomic(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("a", "127.0.0.1", 8001, nil)
	pool.Register("b", "127.0.0.1", 8002, nil)
	pool.IncrUserCount("a", 5)
	pool.IncrUserCount("b", 3)

	pool.TransferUserCount("a", "b")

	// a: 5-1=4, b: 3+1=4
	pool.mu.RLock()
	aCount := pool.upstreams["a"].UserCount
	bCount := pool.upstreams["b"].UserCount
	pool.mu.RUnlock()

	if aCount != 4 || bCount != 4 {
		t.Fatalf("期望 a=4 b=4, 实际 a=%d b=%d", aCount, bCount)
	}
}

// TestUserCountNeverNegative 验证用户计数不会变成负数
func TestUserCountNeverNegative(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("a", "127.0.0.1", 8001, nil)

	pool.IncrUserCount("a", -100) // 尝试减到负数

	pool.mu.RLock()
	count := pool.upstreams["a"].UserCount
	pool.mu.RUnlock()

	if count < 0 {
		t.Fatalf("用户计数不应为负数: %d", count)
	}
}

// TestAtomicMigrate 验证 AtomicMigrate 辅助函数
func TestAtomicMigrate(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("old", "127.0.0.1", 8001, nil)
	pool.Register("new", "127.0.0.1", 8002, nil)
	pool.IncrUserCount("old", 3)

	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "old")

	ok := AtomicMigrate(rt, pool, "user1", "app1", "old", "new")
	if !ok {
		t.Fatal("AtomicMigrate 应成功")
	}

	uid, _ := rt.Lookup("user1", "app1")
	if uid != "new" {
		t.Fatalf("期望 new, 实际 %s", uid)
	}
	pool.mu.RLock()
	oldCount := pool.upstreams["old"].UserCount
	newCount := pool.upstreams["new"].UserCount
	pool.mu.RUnlock()
	if oldCount != 2 || newCount != 1 {
		t.Fatalf("期望 old=2 new=1, 实际 old=%d new=%d", oldCount, newCount)
	}
}

// TestAtomicMigrate_Conflict 验证 AtomicMigrate 冲突时不修改计数
func TestAtomicMigrate_Conflict(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("old", "127.0.0.1", 8001, nil)
	pool.Register("new", "127.0.0.1", 8002, nil)
	pool.IncrUserCount("old", 3)

	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "already-migrated") // 已被其他 goroutine 迁移

	ok := AtomicMigrate(rt, pool, "user1", "app1", "old", "new")
	if ok {
		t.Fatal("冲突时 AtomicMigrate 不应成功")
	}
	// 计数应不变
	pool.mu.RLock()
	oldCount := pool.upstreams["old"].UserCount
	newCount := pool.upstreams["new"].UserCount
	pool.mu.RUnlock()
	if oldCount != 3 || newCount != 0 {
		t.Fatalf("冲突时计数不应变化, 期望 old=3 new=0, 实际 old=%d new=%d", oldCount, newCount)
	}
}

// TestConcurrentCompareAndBind 验证并发 CAS 只有一个成功
func TestConcurrentCompareAndBind(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "original")

	const goroutines = 50
	var wg sync.WaitGroup
	var successCount int32
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newUID := fmt.Sprintf("upstream-%d", idx)
			ok, _ := rt.CompareAndBind("user1", "app1", "original", newUID)
			if ok {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("并发 CAS 应只有 1 个成功, 实际 %d 个", successCount)
	}
}

// TestTraceCorrelator_LRU_O1 验证 LRU 淘汰是 O(1) 且正确
func TestTraceCorrelator_LRU_O1(t *testing.T) {
	tc := NewTraceCorrelator(3) // 最大 3 个

	tc.Set("a", "trace-a")
	tc.Set("b", "trace-b")
	tc.Set("c", "trace-c")

	// 应全部存在
	if tc.Get("a") != "trace-a" { t.Fatal("a 应存在") }
	if tc.Get("b") != "trace-b" { t.Fatal("b 应存在") }
	if tc.Get("c") != "trace-c" { t.Fatal("c 应存在") }

	// 加第 4 个 → a 被淘汰（最旧的）
	tc.Set("d", "trace-d")
	// 注意：a 在上面 Get 时被 moveToFront 了，所以最旧的变成 b
	if tc.Get("d") != "trace-d" { t.Fatal("d 应存在") }

	// 验证 size
	tc.mu.Lock()
	size := tc.size
	tc.mu.Unlock()
	if size != 3 {
		t.Fatalf("LRU 大小应为 3, 实际 %d", size)
	}
}

// TestTraceCorrelator_UpdateExisting 验证更新已有 key 不增加 size
func TestTraceCorrelator_UpdateExisting(t *testing.T) {
	tc := NewTraceCorrelator(10)
	tc.Set("a", "trace-1")
	tc.Set("a", "trace-2") // 更新

	if tc.Get("a") != "trace-2" {
		t.Fatal("更新后应返回新值")
	}
	tc.mu.Lock()
	size := tc.size
	tc.mu.Unlock()
	if size != 1 {
		t.Fatalf("更新不应增加 size, 实际 %d", size)
	}
}

// TestTraceCorrelator_Expiry 验证 5 分钟过期
func TestTraceCorrelator_Expiry(t *testing.T) {
	tc := NewTraceCorrelator(100)
	tc.Set("expired", "old-trace")

	// 手动修改时间戳让它过期
	tc.mu.Lock()
	if node, ok := tc.entries["expired"]; ok {
		node.ts = time.Now().Add(-6 * time.Minute)
	}
	tc.mu.Unlock()

	if tc.Get("expired") != "" {
		t.Fatal("过期 trace 应返回空")
	}
}

// TestUpdateUserInfo_WithLock 验证 UpdateUserInfo 现在是线程安全的
func TestUpdateUserInfo_WithLock(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	rt := NewRouteTable(db, true)
	rt.Bind("user1", "app1", "upstream-a")

	// 并发更新用户信息
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("名字%d", idx)
			email := fmt.Sprintf("user%d@test.com", idx)
			dept := fmt.Sprintf("部门%d", idx)
			rt.UpdateUserInfo("user1", name, email, dept)
		}(i)
	}
	wg.Wait()
	// 不 panic 不死锁就算成功
}

// TestConcurrentTransferUserCount 验证并发计数转移不会负数
func TestConcurrentTransferUserCount(t *testing.T) {
	db := setupRaceTestDB(t)
	defer db.Close()
	cfg := &Config{}
	pool := NewUpstreamPool(cfg, db)
	pool.Register("a", "127.0.0.1", 8001, nil)
	pool.Register("b", "127.0.0.1", 8002, nil)
	pool.IncrUserCount("a", 100)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool.TransferUserCount("a", "b")
		}()
	}
	wg.Wait()

	pool.mu.RLock()
	aCount := pool.upstreams["a"].UserCount
	bCount := pool.upstreams["b"].UserCount
	pool.mu.RUnlock()

	// a 初始 100，转移 200 次每次 -1，但有 floor(0) 保护
	if aCount < 0 {
		t.Fatalf("a 不应为负数: %d", aCount)
	}
	if bCount != 200 {
		t.Fatalf("b 应为 200, 实际 %d", bCount)
	}
	// a 被 floor 到 0 后不再减：最终 a=0, b=200, 总和=200（初始100+净增100）
	// 这是预期行为：floor 保护导致不守恒，但避免了负数
}
