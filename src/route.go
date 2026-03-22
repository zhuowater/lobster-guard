// route.go — RouteTable、UpstreamPool、UserInfoCache、UserInfoProvider、RoutePolicyEngine
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 上游容器管理
// ============================================================

type Upstream struct {
	ID            string                 `json:"id"`
	Address       string                 `json:"address"`
	Port          int                    `json:"port"`
	PathPrefix    string                 `json:"path_prefix,omitempty"`
	Healthy       bool                   `json:"healthy"`
	RegisteredAt  time.Time              `json:"registered_at"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	Tags          map[string]string      `json:"tags"`
	Load          map[string]interface{} `json:"load"`
	UserCount     int                    `json:"user_count"`
	Static        bool                   `json:"static"`
	proxy         *httputil.ReverseProxy
}

type UpstreamPool struct {
	mu                sync.RWMutex
	upstreams         map[string]*Upstream
	heartbeatInterval time.Duration
	heartbeatTimeout  int
	db                *sql.DB
	roundRobinIdx     uint64
}

func NewUpstreamPool(cfg *Config, db *sql.DB) *UpstreamPool {
	pool := &UpstreamPool{
		upstreams:         make(map[string]*Upstream),
		heartbeatInterval: time.Duration(cfg.HeartbeatIntervalSec) * time.Second,
		heartbeatTimeout:  cfg.HeartbeatTimeoutCount,
		db:                db,
	}
	if pool.heartbeatInterval <= 0 { pool.heartbeatInterval = 10 * time.Second }
	if pool.heartbeatTimeout <= 0 { pool.heartbeatTimeout = 30 } // BUG-012 fix: default 30 * interval = 5min @10s
	for _, su := range cfg.StaticUpstreams {
		up := &Upstream{
			ID: su.ID, Address: su.Address, Port: su.Port, PathPrefix: su.PathPrefix, Healthy: true,
			RegisteredAt: time.Now(), LastHeartbeat: time.Now(),
			Tags: map[string]string{"type": "static"}, Load: map[string]interface{}{}, Static: true,
		}
		up.proxy = createReverseProxy(up.Address, up.Port, up.PathPrefix)
		pool.upstreams[up.ID] = up
		log.Printf("[上游池] 加载静态上游: %s -> %s:%d prefix=%s", up.ID, up.Address, up.Port, up.PathPrefix)
	}
	if len(pool.upstreams) == 0 && cfg.OpenClawUpstream != "" {
		u, err := url.Parse(cfg.OpenClawUpstream)
		if err == nil {
			port := 18790
			if u.Port() != "" { fmt.Sscanf(u.Port(), "%d", &port) }
			host := u.Hostname()
			if host == "" { host = "127.0.0.1" }
			pathPrefix := strings.TrimRight(u.Path, "/")
			up := &Upstream{
				ID: "openclaw-default", Address: host, Port: port, PathPrefix: pathPrefix, Healthy: true,
				RegisteredAt: time.Now(), LastHeartbeat: time.Now(),
				Tags: map[string]string{"type": "legacy"}, Load: map[string]interface{}{}, Static: true,
			}
			up.proxy = createReverseProxy(host, port, pathPrefix)
			pool.upstreams[up.ID] = up
			log.Printf("[上游池] v1.0 兼容上游: %s -> %s:%d prefix=%s", up.ID, host, port, pathPrefix)
		}
	}
	pool.loadUpstreamsFromDB()
	return pool
}

func createReverseProxy(address string, port int, pathPrefix string) *httputil.ReverseProxy {
	target := fmt.Sprintf("http://%s:%d", address, port)
	u, _ := url.Parse(target)
	prefix := path.Clean("/" + strings.TrimRight(pathPrefix, "/"))
	if prefix == "/" { prefix = "" } // Clean("/") → "/" but we want empty
	p := httputil.NewSingleHostReverseProxy(u)
	p.Transport = &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns: 100, MaxIdleConnsPerHost: 50, IdleConnTimeout: 90 * time.Second,
	}
	od := p.Director
	p.Director = func(r *http.Request) {
		od(r)
		r.Host = u.Host
		if prefix != "" {
			r.URL.Path = prefix + r.URL.Path
			if r.URL.RawPath != "" {
				r.URL.RawPath = prefix + r.URL.RawPath
			}
		}
	}
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		log.Printf("[上游] 转发错误 -> %s: %v", target, e)
		w.WriteHeader(502)
		w.Write([]byte(`{"errcode":502,"errmsg":"upstream unavailable"}`))
	}
	return p
}

func (pool *UpstreamPool) loadUpstreamsFromDB() {
	if pool.db == nil { return }
	rows, err := pool.db.Query(`SELECT id, address, port, healthy, registered_at, last_heartbeat, tags, load, COALESCE(path_prefix,'') FROM upstreams`)
	if err != nil { return }
	defer rows.Close()
	for rows.Next() {
		var id, address, regAt, hbAt, tagsJSON, loadJSON, pathPrefix string
		var port, healthy int
		if rows.Scan(&id, &address, &port, &healthy, &regAt, &hbAt, &tagsJSON, &loadJSON, &pathPrefix) != nil { continue }
		if _, exists := pool.upstreams[id]; exists { continue }
		up := &Upstream{ID: id, Address: address, Port: port, PathPrefix: pathPrefix, Healthy: healthy == 1,
			Tags: map[string]string{}, Load: map[string]interface{}{}}
		up.RegisteredAt, _ = time.Parse(time.RFC3339, regAt)
		up.LastHeartbeat, _ = time.Parse(time.RFC3339, hbAt)
		json.Unmarshal([]byte(tagsJSON), &up.Tags)
		json.Unmarshal([]byte(loadJSON), &up.Load)
		up.proxy = createReverseProxy(address, port, pathPrefix)
		pool.upstreams[id] = up
		log.Printf("[上游池] 从数据库恢复上游: %s -> %s:%d prefix=%s healthy=%v", id, address, port, pathPrefix, up.Healthy)
	}
}

func (pool *UpstreamPool) saveUpstreamToDB(id string) {
	if pool.db == nil { return }
	up, ok := pool.upstreams[id]
	if !ok { return }
	tagsJSON, _ := json.Marshal(up.Tags)
	loadJSON, _ := json.Marshal(up.Load)
	h := 0; if up.Healthy { h = 1 }
	pool.db.Exec(`INSERT OR REPLACE INTO upstreams (id,address,port,healthy,registered_at,last_heartbeat,tags,load,path_prefix) VALUES(?,?,?,?,?,?,?,?,?)`,
		id, up.Address, up.Port, h, up.RegisteredAt.Format(time.RFC3339), up.LastHeartbeat.Format(time.RFC3339),
		string(tagsJSON), string(loadJSON), up.PathPrefix)
}

func (pool *UpstreamPool) Register(id, address string, port int, tags map[string]string) error {
	pool.mu.Lock(); defer pool.mu.Unlock()
	now := time.Now()
	if existing, ok := pool.upstreams[id]; ok {
		existing.Address = address; existing.Port = port
		existing.Healthy = true; existing.LastHeartbeat = now
		if tags != nil { existing.Tags = tags }
		existing.proxy = createReverseProxy(address, port, existing.PathPrefix)
	} else {
		up := &Upstream{ID: id, Address: address, Port: port, Healthy: true,
			RegisteredAt: now, LastHeartbeat: now,
			Tags: tags, Load: map[string]interface{}{}}
		if up.Tags == nil { up.Tags = map[string]string{} }
		up.proxy = createReverseProxy(address, port, "")
		pool.upstreams[id] = up
	}
	pool.saveUpstreamToDB(id)
	log.Printf("[上游池] 注册上游: %s -> %s:%d", id, address, port)
	return nil
}

func (pool *UpstreamPool) Heartbeat(id string, load map[string]interface{}) (int, error) {
	pool.mu.Lock(); defer pool.mu.Unlock()
	up, ok := pool.upstreams[id]
	if !ok { return 0, fmt.Errorf("上游 %s 未注册", id) }
	up.LastHeartbeat = time.Now()
	up.Healthy = true
	if load != nil { up.Load = load }
	pool.saveUpstreamToDB(id)
	return up.UserCount, nil
}

func (pool *UpstreamPool) Deregister(id string) {
	pool.mu.Lock(); defer pool.mu.Unlock()
	if up, ok := pool.upstreams[id]; ok && !up.Static {
		delete(pool.upstreams, id)
		if pool.db != nil { pool.db.Exec(`DELETE FROM upstreams WHERE id = ?`, id) }
		log.Printf("[上游池] 注销上游: %s", id)
	}
}

// Update 更新已有上游的地址、端口、tags（v21.0 上游 CRUD）
func (pool *UpstreamPool) Update(id, address string, port int, tags map[string]string) error {
	pool.mu.Lock(); defer pool.mu.Unlock()
	up, ok := pool.upstreams[id]
	if !ok { return fmt.Errorf("上游 %s 不存在", id) }
	if address != "" { up.Address = address }
	if port > 0 { up.Port = port }
	if tags != nil { up.Tags = tags }
	up.proxy = createReverseProxy(up.Address, up.Port, up.PathPrefix)
	pool.saveUpstreamToDB(id)
	log.Printf("[上游池] 更新上游: %s -> %s:%d prefix=%s", id, up.Address, up.Port, up.PathPrefix)
	return nil
}

// GetUpstream 获取单个上游详情（v21.0 上游 CRUD）
func (pool *UpstreamPool) GetUpstream(id string) (*Upstream, bool) {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	up, ok := pool.upstreams[id]
	if !ok { return nil, false }
	copy := *up
	return &copy, true
}

// ForceDeregister 强制注销上游（包括静态上游，K8s 发现的也可以手动删除）
func (pool *UpstreamPool) ForceDeregister(id string) bool {
	pool.mu.Lock(); defer pool.mu.Unlock()
	if _, ok := pool.upstreams[id]; !ok { return false }
	delete(pool.upstreams, id)
	if pool.db != nil { pool.db.Exec(`DELETE FROM upstreams WHERE id = ?`, id) }
	log.Printf("[上游池] 强制注销上游: %s", id)
	return true
}

// GetProxy 获取指定上游的反向代理
func (pool *UpstreamPool) GetProxy(id string) *httputil.ReverseProxy {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	if up, ok := pool.upstreams[id]; ok && up.proxy != nil { return up.proxy }
	return nil
}

// GetAnyHealthyProxy 返回任意一个健康上游的代理（failopen 兜底）
func (pool *UpstreamPool) GetAnyHealthyProxy() (*httputil.ReverseProxy, string) {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	for id, up := range pool.upstreams {
		if up.Healthy && up.proxy != nil { return up.proxy, id }
	}
	// 所有都不健康，返回第一个（failopen）
	for id, up := range pool.upstreams {
		if up.proxy != nil { return up.proxy, id }
	}
	return nil, ""
}

// SelectUpstream 按策略选择上游容器（用于新用户分配）
func (pool *UpstreamPool) SelectUpstream(policy string) string {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	var healthy []*Upstream
	for _, up := range pool.upstreams {
		if up.Healthy { healthy = append(healthy, up) }
	}
	if len(healthy) == 0 {
		// failopen: 返回任意一个
		for _, up := range pool.upstreams { return up.ID }
		return ""
	}
	switch policy {
	case "round-robin":
		idx := atomic.AddUint64(&pool.roundRobinIdx, 1)
		return healthy[int(idx)%len(healthy)].ID
	default: // least-users
		sort.Slice(healthy, func(i, j int) bool { return healthy[i].UserCount < healthy[j].UserCount })
		return healthy[0].ID
	}
}

// IsHealthy 检查指定上游是否健康
func (pool *UpstreamPool) IsHealthy(id string) bool {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	if up, ok := pool.upstreams[id]; ok { return up.Healthy }
	return false
}

// IncrUserCount 增加上游用户计数
func (pool *UpstreamPool) IncrUserCount(id string, delta int) {
	pool.mu.Lock(); defer pool.mu.Unlock()
	if up, ok := pool.upstreams[id]; ok {
		up.UserCount += delta
		if up.UserCount < 0 { up.UserCount = 0 } // 防止负数
	}
}

// TransferUserCount 原子转移用户计数：from -1, to +1，单次加锁
func (pool *UpstreamPool) TransferUserCount(fromID, toID string) {
	pool.mu.Lock(); defer pool.mu.Unlock()
	if fromID != "" {
		if up, ok := pool.upstreams[fromID]; ok {
			up.UserCount--
			if up.UserCount < 0 { up.UserCount = 0 }
		}
	}
	if toID != "" {
		if up, ok := pool.upstreams[toID]; ok {
			up.UserCount++
		}
	}
}

// Count returns total and healthy upstream counts
func (pool *UpstreamPool) Count() (total, healthy int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	for _, u := range pool.upstreams {
		total++
		if u.Healthy {
			healthy++
		}
	}
	return
}

// ListUpstreams 列出所有上游
func (pool *UpstreamPool) ListUpstreams() []Upstream {
	pool.mu.RLock(); defer pool.mu.RUnlock()
	var list []Upstream
	for _, up := range pool.upstreams {
		list = append(list, *up)
	}
	return list
}

// HealthCheck 健康检查循环（标记超时的上游为不健康，移除长期不健康的）
func (pool *UpstreamPool) HealthCheck(ctx context.Context) {
	ticker := time.NewTicker(pool.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pool.mu.Lock()
			now := time.Now()
			timeout := pool.heartbeatInterval * time.Duration(pool.heartbeatTimeout)
			var toRemove []string
			for id, up := range pool.upstreams {
				if up.Static { continue }
				if now.Sub(up.LastHeartbeat) > timeout {
					if up.Healthy {
						up.Healthy = false
						log.Printf("[健康检查] 上游 %s 心跳超时，标记为不健康", id)
					}
					// 5分钟持续不健康则移除
					if now.Sub(up.LastHeartbeat) > 5*time.Minute {
						toRemove = append(toRemove, id)
					}
				}
			}
			for _, id := range toRemove {
				delete(pool.upstreams, id)
				if pool.db != nil { pool.db.Exec(`DELETE FROM upstreams WHERE id = ?`, id) }
				log.Printf("[健康检查] 上游 %s 持续不健康，已自动移除", id)
			}
			pool.mu.Unlock()
		}
	}
}

// ============================================================
// 路由表 v3.8 — 复合键 (sender_id, app_id) → upstream_id
// ============================================================

// RouteEntry 路由条目（v3.8 结构化）
type RouteEntry struct {
	SenderID       string `json:"sender_id"`
	AppID          string `json:"app_id"`
	UpstreamID     string `json:"upstream_id"`
	Department     string `json:"department,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	Email          string `json:"email,omitempty"`    // v3.9
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

// routeKey 生成复合路由键
func routeKey(senderID, appID string) string {
	return senderID + "|" + appID
}

// ============================================================
// v3.9: 用户信息自动获取 — UserInfo + UserInfoProvider + UserInfoCache
// ============================================================

// UserInfo 用户信息（所有 IM 平台统一）
type UserInfo struct {
	SenderID   string `json:"sender_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Mobile     string `json:"mobile,omitempty"`
	Department string `json:"department"`
	Avatar     string `json:"avatar,omitempty"`
	FetchedAt  time.Time `json:"fetched_at,omitempty"`
}

// UserInfoProvider 可选接口 — 插件实现此接口则支持用户信息自动获取
type UserInfoProvider interface {
	FetchUserInfo(senderID string) (*UserInfo, error)
	NeedsCredentials() []string
}

// UserInfoCache 内存+DB 两级缓存
type UserInfoCache struct {
	mu       sync.RWMutex
	memory   map[string]*UserInfo
	memTime  map[string]time.Time // sender_id -> fetched_at in memory
	db       *sql.DB
	ttl      time.Duration
	provider UserInfoProvider
}

// NewUserInfoCache 创建用户信息缓存
func NewUserInfoCache(db *sql.DB, provider UserInfoProvider, ttl time.Duration) *UserInfoCache {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &UserInfoCache{
		memory:   make(map[string]*UserInfo),
		memTime:  make(map[string]time.Time),
		db:       db,
		ttl:      ttl,
		provider: provider,
	}
}

// GetOrFetch 获取用户信息：内存 → DB → API
func (c *UserInfoCache) GetOrFetch(senderID string) (*UserInfo, error) {
	if senderID == "" {
		return nil, nil
	}

	// 1. 内存缓存
	c.mu.RLock()
	if info, ok := c.memory[senderID]; ok {
		if ft, ok2 := c.memTime[senderID]; ok2 && time.Since(ft) < c.ttl {
			c.mu.RUnlock()
			return info, nil
		}
	}
	c.mu.RUnlock()

	// 2. DB 缓存
	if c.db != nil {
		info, err := c.loadFromDB(senderID)
		if err == nil && info != nil && time.Since(info.FetchedAt) < c.ttl {
			c.putMemory(info)
			return info, nil
		}
	}

	// 3. API 获取
	if c.provider == nil {
		return nil, nil
	}
	info, err := c.provider.FetchUserInfo(senderID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, nil
	}
	info.SenderID = senderID
	info.FetchedAt = time.Now()

	// 写入缓存
	c.putMemory(info)
	if c.db != nil {
		c.saveToDB(info)
	}
	return info, nil
}

// GetOrFetchWithTimeout 带超时的 GetOrFetch — 用于新用户首次请求时同步等待用户信息
// 内存/DB 缓存命中时立即返回；需要 API 调用时限制在 timeout 内完成
// 超时返回 nil（调用方降级到负载均衡）
func (c *UserInfoCache) GetOrFetchWithTimeout(senderID string, timeout time.Duration) (*UserInfo, error) {
	if senderID == "" || timeout <= 0 {
		return nil, nil
	}

	// 1. 内存缓存（无需等待）
	c.mu.RLock()
	if info, ok := c.memory[senderID]; ok {
		if ft, ok2 := c.memTime[senderID]; ok2 && time.Since(ft) < c.ttl {
			c.mu.RUnlock()
			return info, nil
		}
	}
	c.mu.RUnlock()

	// 2. DB 缓存（无需等待）
	if c.db != nil {
		info, err := c.loadFromDB(senderID)
		if err == nil && info != nil && time.Since(info.FetchedAt) < c.ttl {
			c.putMemory(info)
			return info, nil
		}
	}

	// 3. API 获取（有超时限制）
	if c.provider == nil {
		return nil, nil
	}
	type fetchResult struct {
		info *UserInfo
		err  error
	}
	ch := make(chan fetchResult, 1)
	go func() {
		defer func() {
			if rv := recover(); rv != nil {
				ch <- fetchResult{nil, fmt.Errorf("panic: %v", rv)}
			}
		}()
		info, err := c.provider.FetchUserInfo(senderID)
		ch <- fetchResult{info, err}
	}()

	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		if result.info == nil {
			return nil, nil
		}
		result.info.SenderID = senderID
		result.info.FetchedAt = time.Now()
		c.putMemory(result.info)
		if c.db != nil {
			c.saveToDB(result.info)
		}
		return result.info, nil
	case <-time.After(timeout):
		// 超时：API 调用仍在后台跑（goroutine 最终会完成并写入缓存），但不阻塞请求
		go func() {
			if result := <-ch; result.err == nil && result.info != nil {
				result.info.SenderID = senderID
				result.info.FetchedAt = time.Now()
				c.putMemory(result.info)
				if c.db != nil {
					c.saveToDB(result.info)
				}
			}
		}()
		return nil, nil
	}
}

// GetCached 仅从缓存获取（不调API）
func (c *UserInfoCache) GetCached(senderID string) *UserInfo {
	c.mu.RLock()
	if info, ok := c.memory[senderID]; ok {
		c.mu.RUnlock()
		return info
	}
	c.mu.RUnlock()

	if c.db != nil {
		info, err := c.loadFromDB(senderID)
		if err == nil && info != nil {
			c.putMemory(info)
			return info
		}
	}
	return nil
}

func (c *UserInfoCache) putMemory(info *UserInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memory[info.SenderID] = info
	c.memTime[info.SenderID] = info.FetchedAt
}

func (c *UserInfoCache) loadFromDB(senderID string) (*UserInfo, error) {
	var info UserInfo
	var fetchedAt string
	err := c.db.QueryRow(`SELECT sender_id, name, email, department, avatar, mobile, fetched_at FROM user_info_cache WHERE sender_id = ?`, senderID).
		Scan(&info.SenderID, &info.Name, &info.Email, &info.Department, &info.Avatar, &info.Mobile, &fetchedAt)
	if err != nil {
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339, fetchedAt)
	info.FetchedAt = t
	return &info, nil
}

func (c *UserInfoCache) saveToDB(info *UserInfo) {
	now := time.Now().Format(time.RFC3339)
	c.db.Exec(`INSERT OR REPLACE INTO user_info_cache (sender_id, name, email, department, avatar, mobile, fetched_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		info.SenderID, info.Name, info.Email, info.Department, info.Avatar, info.Mobile, info.FetchedAt.Format(time.RFC3339), now)
}

// ListAll 列出所有缓存用户
func (c *UserInfoCache) ListAll(department, email string) []*UserInfo {
	if c.db == nil {
		return nil
	}
	query := `SELECT sender_id, name, email, department, avatar, mobile, fetched_at FROM user_info_cache WHERE 1=1`
	var args []interface{}
	if department != "" {
		query += ` AND department = ?`
		args = append(args, department)
	}
	if email != "" {
		query += ` AND email LIKE ?`
		args = append(args, "%"+email+"%")
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var results []*UserInfo
	for rows.Next() {
		var info UserInfo
		var fetchedAt string
		if rows.Scan(&info.SenderID, &info.Name, &info.Email, &info.Department, &info.Avatar, &info.Mobile, &fetchedAt) == nil {
			t, _ := time.Parse(time.RFC3339, fetchedAt)
			info.FetchedAt = t
			results = append(results, &info)
		}
	}
	return results
}

// GetByID 获取单个用户信息
func (c *UserInfoCache) GetByID(senderID string) *UserInfo {
	return c.GetCached(senderID)
}

// Refresh 强制刷新单个用户
func (c *UserInfoCache) Refresh(senderID string) (*UserInfo, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("no provider configured")
	}
	info, err := c.provider.FetchUserInfo(senderID)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("user not found")
	}
	info.SenderID = senderID
	info.FetchedAt = time.Now()
	c.putMemory(info)
	if c.db != nil {
		c.saveToDB(info)
	}
	return info, nil
}

// RefreshAll 刷新所有已知用户
func (c *UserInfoCache) RefreshAll() (int, int) {
	if c.provider == nil || c.db == nil {
		return 0, 0
	}
	rows, err := c.db.Query(`SELECT sender_id FROM user_info_cache`)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	var senderIDs []string
	for rows.Next() {
		var sid string
		if rows.Scan(&sid) == nil {
			senderIDs = append(senderIDs, sid)
		}
	}
	success, failed := 0, 0
	for _, sid := range senderIDs {
		_, err := c.Refresh(sid)
		if err != nil {
			failed++
		} else {
			success++
		}
	}
	return success, failed
}

// ============================================================
// v3.9: LanxinUserProvider — 蓝信用户信息获取
// ============================================================

type LanxinUserProvider struct {
	appID     string
	appSecret string
	upstream  string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewLanxinUserProvider(appID, appSecret, upstream string) *LanxinUserProvider {
	return &LanxinUserProvider{
		appID:     appID,
		appSecret: appSecret,
		upstream:  strings.TrimRight(upstream, "/"),
	}
}

func (p *LanxinUserProvider) getToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}
	url := fmt.Sprintf("%s/v1/apptoken/create?grant_type=client_credential&appid=%s&secret=%s",
		p.upstream, url.QueryEscape(p.appID), url.QueryEscape(p.appSecret))
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("蓝信获取app_token失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
		Data struct {
			AppToken string `json:"app_token"`
			ExpiresIn int  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("蓝信app_token解析失败: %w", err)
	}
	if result.ErrCode != 0 || result.Data.AppToken == "" {
		return "", fmt.Errorf("蓝信app_token获取失败, errCode=%d, errMsg=%s", result.ErrCode, result.ErrMsg)
	}
	p.token = result.Data.AppToken
	expireIn := result.Data.ExpiresIn
	if expireIn <= 0 {
		expireIn = 7200
	}
	// 提前5分钟过期
	p.tokenExp = time.Now().Add(time.Duration(expireIn-300) * time.Second)
	return p.token, nil
}

func (p *LanxinUserProvider) FetchUserInfo(senderID string) (*UserInfo, error) {
	token, err := p.getToken()
	if err != nil {
		return nil, err
	}
	// 使用详细信息接口 /v1/staffs/:staffid/infor/fetch（返回 email、手机号等）
	reqURL := fmt.Sprintf("%s/v1/staffs/%s/infor/fetch?app_token=%s",
		p.upstream, url.PathEscape(senderID), url.QueryEscape(token))
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("蓝信查询用户失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
		Data    struct {
			Name        string `json:"name"`
			Email       string `json:"email"`
			OrgName     string `json:"orgName"`
			AvatarURL   string `json:"avatarUrl"`
			MobilePhone struct {
				CountryCode string `json:"countryCode"`
				Number      string `json:"number"`
			} `json:"mobilePhone"`
			Departments []struct {
				Name string `json:"name"`
			} `json:"departments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("蓝信用户信息解析失败: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("蓝信用户查询失败, errCode=%d, errMsg=%s", result.ErrCode, result.ErrMsg)
	}
	if result.Data.Name == "" {
		return nil, nil // 用户不存在
	}
	// 部门：拼接所有部门名（逗号分隔）
	dept := ""
	if len(result.Data.Departments) > 0 {
		var deptNames []string
		for _, d := range result.Data.Departments {
			if d.Name != "" {
				deptNames = append(deptNames, d.Name)
			}
		}
		dept = strings.Join(deptNames, ",")
	}
	if dept == "" {
		dept = result.Data.OrgName
	}
	// 手机号拼接
	mobile := ""
	if result.Data.MobilePhone.Number != "" {
		mobile = result.Data.MobilePhone.CountryCode + "-" + result.Data.MobilePhone.Number
	}
	info := &UserInfo{
		SenderID:   senderID,
		Name:       result.Data.Name,
		Email:      result.Data.Email,
		Mobile:     mobile,
		Department: dept,
		Avatar:     result.Data.AvatarURL,
	}
	return info, nil
}

func (p *LanxinUserProvider) NeedsCredentials() []string {
	return []string{"lanxin_app_id", "lanxin_app_secret"}
}

// ============================================================
// v3.9: FeishuUserProvider — 飞书用户信息获取
// ============================================================

type FeishuUserProvider struct {
	appID     string
	appSecret string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewFeishuUserProvider(appID, appSecret string) *FeishuUserProvider {
	return &FeishuUserProvider{appID: appID, appSecret: appSecret}
}

func (p *FeishuUserProvider) getTenantToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"app_id":     p.appID,
		"app_secret": p.appSecret,
	})
	resp, err := http.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("飞书获取tenant_token失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.TenantAccessToken == "" {
		return "", fmt.Errorf("飞书tenant_token为空: code=%d msg=%s", result.Code, result.Msg)
	}
	p.token = result.TenantAccessToken
	expire := result.Expire
	if expire <= 0 {
		expire = 7200
	}
	p.tokenExp = time.Now().Add(time.Duration(expire-300) * time.Second)
	return p.token, nil
}

func (p *FeishuUserProvider) FetchUserInfo(senderID string) (*UserInfo, error) {
	token, err := p.getTenantToken()
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("https://open.feishu.cn/open-apis/contact/v3/users/%s", url.PathEscape(senderID))
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("飞书查询用户失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				Name          string   `json:"name"`
				Email         string   `json:"email"`
				DepartmentIDs []string `json:"department_ids"`
				Avatar        struct {
					Avatar72 string `json:"avatar_72"`
				} `json:"avatar"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Data.User.Name == "" {
		return nil, nil
	}
	dept := ""
	if len(result.Data.User.DepartmentIDs) > 0 {
		dept = strings.Join(result.Data.User.DepartmentIDs, ",")
	}
	return &UserInfo{
		SenderID:   senderID,
		Name:       result.Data.User.Name,
		Email:      result.Data.User.Email,
		Department: dept,
		Avatar:     result.Data.User.Avatar.Avatar72,
	}, nil
}

func (p *FeishuUserProvider) NeedsCredentials() []string {
	return []string{"feishu_app_id", "feishu_app_secret"}
}

// ============================================================
// v3.9: DingTalkUserProvider — 钉钉用户信息获取
// ============================================================

type DingTalkUserProvider struct {
	clientID     string
	clientSecret string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewDingTalkUserProvider(clientID, clientSecret string) *DingTalkUserProvider {
	return &DingTalkUserProvider{clientID: clientID, clientSecret: clientSecret}
}

func (p *DingTalkUserProvider) getAccessToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}
	reqURL := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s",
		url.QueryEscape(p.clientID), url.QueryEscape(p.clientSecret))
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("钉钉获取access_token失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("钉钉access_token为空: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	p.token = result.AccessToken
	expire := result.ExpiresIn
	if expire <= 0 {
		expire = 7200
	}
	p.tokenExp = time.Now().Add(time.Duration(expire-300) * time.Second)
	return p.token, nil
}

func (p *DingTalkUserProvider) FetchUserInfo(senderID string) (*UserInfo, error) {
	token, err := p.getAccessToken()
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/v2/user/get?access_token=%s", url.QueryEscape(token))
	body, _ := json.Marshal(map[string]string{"userid": senderID})
	resp, err := http.Post(reqURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("钉钉查询用户失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode int `json:"errcode"`
		Result  struct {
			Name       string `json:"name"`
			Email      string `json:"email"`
			DeptIDList []int  `json:"dept_id_list"`
			Avatar     string `json:"avatar"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Result.Name == "" {
		return nil, nil
	}
	dept := ""
	if len(result.Result.DeptIDList) > 0 {
		deptStrs := make([]string, len(result.Result.DeptIDList))
		for i, d := range result.Result.DeptIDList {
			deptStrs[i] = strconv.Itoa(d)
		}
		dept = strings.Join(deptStrs, ",")
	}
	return &UserInfo{
		SenderID:   senderID,
		Name:       result.Result.Name,
		Email:      result.Result.Email,
		Department: dept,
		Avatar:     result.Result.Avatar,
	}, nil
}

func (p *DingTalkUserProvider) NeedsCredentials() []string {
	return []string{"dingtalk_client_id", "dingtalk_client_secret"}
}

// ============================================================
// v3.9: WeComUserProvider — 企业微信用户信息获取
// ============================================================

type WeComUserProvider struct {
	corpID     string
	corpSecret string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewWeComUserProvider(corpID, corpSecret string) *WeComUserProvider {
	return &WeComUserProvider{corpID: corpID, corpSecret: corpSecret}
}

func (p *WeComUserProvider) getAccessToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}
	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		url.QueryEscape(p.corpID), url.QueryEscape(p.corpSecret))
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("企微获取access_token失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("企微access_token为空: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	p.token = result.AccessToken
	expire := result.ExpiresIn
	if expire <= 0 {
		expire = 7200
	}
	p.tokenExp = time.Now().Add(time.Duration(expire-300) * time.Second)
	return p.token, nil
}

func (p *WeComUserProvider) FetchUserInfo(senderID string) (*UserInfo, error) {
	token, err := p.getAccessToken()
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=%s&userid=%s",
		url.QueryEscape(token), url.QueryEscape(senderID))
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("企微查询用户失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		ErrCode    int    `json:"errcode"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		Department []int  `json:"department"`
		Avatar     string `json:"avatar"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Name == "" {
		return nil, nil
	}
	dept := ""
	if len(result.Department) > 0 {
		deptStrs := make([]string, len(result.Department))
		for i, d := range result.Department {
			deptStrs[i] = strconv.Itoa(d)
		}
		dept = strings.Join(deptStrs, ",")
	}
	return &UserInfo{
		SenderID:   senderID,
		Name:       result.Name,
		Email:      result.Email,
		Department: dept,
		Avatar:     result.Avatar,
	}, nil
}

func (p *WeComUserProvider) NeedsCredentials() []string {
	return []string{"wecom_corp_id", "wecom_corp_secret"}
}

// ============================================================
// containsDepartment 检查用户部门列表（逗号分隔）是否包含目标部门
func containsDepartment(userDepts, target string) bool {
	for _, d := range strings.Split(userDepts, ",") {
		if strings.EqualFold(strings.TrimSpace(d), target) {
			return true
		}
	}
	return false
}

// v3.9: RoutePolicyEngine — 路由策略引擎
// ============================================================

type RoutePolicyEngine struct {
	mu       sync.RWMutex
	policies []RoutePolicyConfig
}

func NewRoutePolicyEngine(policies []RoutePolicyConfig) *RoutePolicyEngine {
	return &RoutePolicyEngine{policies: policies}
}

// Match 匹配策略，返回 upstream_id 和是否命中
func (rpe *RoutePolicyEngine) Match(info *UserInfo, appID string) (string, bool) {
	if info == nil {
		return "", false
	}
	rpe.mu.RLock()
	defer rpe.mu.RUnlock()

	// default 策略作为兜底，必须在所有精确匹配之后才生效
	var defaultUpstream string
	hasDefault := false

	for _, p := range rpe.policies {
		if p.Match.Default {
			defaultUpstream = p.UpstreamID
			hasDefault = true
			continue // 记录但不立即返回，继续匹配更精确的规则
		}
		matched := true
		hasCondition := false

		if p.Match.Email != "" {
			hasCondition = true
			if !strings.EqualFold(info.Email, p.Match.Email) {
				matched = false
			}
		}
		if matched && p.Match.EmailSuffix != "" {
			hasCondition = true
			if !strings.HasSuffix(strings.ToLower(info.Email), strings.ToLower(p.Match.EmailSuffix)) {
				matched = false
			}
		}
		if matched && p.Match.Department != "" {
			hasCondition = true
			if !containsDepartment(info.Department, p.Match.Department) {
				matched = false
			}
		}
		if matched && p.Match.AppID != "" {
			hasCondition = true
			if appID != p.Match.AppID {
				matched = false
			}
		}
		if hasCondition && matched {
			return p.UpstreamID, true
		}
	}
	// 没有精确匹配命中，使用 default 兜底
	if hasDefault {
		return defaultUpstream, true
	}
	return "", false
}

// ListPolicies 返回策略列表
func (rpe *RoutePolicyEngine) ListPolicies() []RoutePolicyConfig {
	rpe.mu.RLock()
	defer rpe.mu.RUnlock()
	result := make([]RoutePolicyConfig, len(rpe.policies))
	copy(result, rpe.policies)
	return result
}

// SetPolicies 替换策略列表（用于 CRUD 操作后更新内存）
func (rpe *RoutePolicyEngine) SetPolicies(policies []RoutePolicyConfig) {
	rpe.mu.Lock()
	defer rpe.mu.Unlock()
	rpe.policies = make([]RoutePolicyConfig, len(policies))
	copy(rpe.policies, policies)
}

// TestMatch 测试某个用户会命中哪条策略
func (rpe *RoutePolicyEngine) TestMatch(info *UserInfo, appID string) (int, *RoutePolicyConfig, bool) {
	if info == nil {
		return -1, nil, false
	}
	rpe.mu.RLock()
	defer rpe.mu.RUnlock()

	// default 策略作为兜底，必须在所有精确匹配之后才生效
	defaultIdx := -1

	for i, p := range rpe.policies {
		if p.Match.Default {
			defaultIdx = i
			continue // 记录但不立即返回
		}
		matched := true
		hasCondition := false

		if p.Match.Email != "" {
			hasCondition = true
			if !strings.EqualFold(info.Email, p.Match.Email) {
				matched = false
			}
		}
		if matched && p.Match.EmailSuffix != "" {
			hasCondition = true
			if !strings.HasSuffix(strings.ToLower(info.Email), strings.ToLower(p.Match.EmailSuffix)) {
				matched = false
			}
		}
		if matched && p.Match.Department != "" {
			hasCondition = true
			if !containsDepartment(info.Department, p.Match.Department) {
				matched = false
			}
		}
		if matched && p.Match.AppID != "" {
			hasCondition = true
			if appID != p.Match.AppID {
				matched = false
			}
		}
		if hasCondition && matched {
			return i, &rpe.policies[i], true
		}
	}
	// 没有精确匹配命中，使用 default 兜底
	if defaultIdx >= 0 {
		return defaultIdx, &rpe.policies[defaultIdx], true
	}
	return -1, nil, false
}

// createUserInfoProvider 根据配置创建对应平台的 UserInfoProvider
func createUserInfoProvider(cfg *Config) UserInfoProvider {
	channel := cfg.Channel
	if channel == "" {
		channel = "lanxin"
	}
	switch channel {
	case "lanxin":
		if cfg.LanxinAppID != "" && cfg.LanxinAppSecret != "" {
			return NewLanxinUserProvider(cfg.LanxinAppID, cfg.LanxinAppSecret, cfg.LanxinUpstream)
		}
	case "feishu":
		if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
			return NewFeishuUserProvider(cfg.FeishuAppID, cfg.FeishuAppSecret)
		}
	case "dingtalk":
		if cfg.DingtalkClientID != "" && cfg.DingtalkClientSecret != "" {
			return NewDingTalkUserProvider(cfg.DingtalkClientID, cfg.DingtalkClientSecret)
		}
	case "wecom":
		if cfg.WecomCorpId != "" && cfg.WecomCorpSecret != "" {
			return NewWeComUserProvider(cfg.WecomCorpId, cfg.WecomCorpSecret)
		}
	}
	return nil
}

type RouteTable struct {
	mu    sync.RWMutex
	exact map[string]string // "sender_id|app_id" -> upstream_id
	db    *sql.DB
}

func NewRouteTable(db *sql.DB, persist bool) *RouteTable {
	rt := &RouteTable{exact: make(map[string]string), db: db}
	if persist && db != nil {
		rt.loadFromDB()
	}
	return rt
}

func (rt *RouteTable) loadFromDB() {
	rows, err := rt.db.Query(`SELECT sender_id, app_id, upstream_id FROM user_routes`)
	if err != nil { return }
	defer rows.Close()
	for rows.Next() {
		var sid, appID, uid string
		if rows.Scan(&sid, &appID, &uid) == nil {
			rt.exact[routeKey(sid, appID)] = uid
		}
	}
	log.Printf("[路由表] 从数据库恢复 %d 条路由", len(rt.exact))
}

// Lookup 先精确匹配 (senderID, appID)，没找到再 fallback 到 (senderID, "")
func (rt *RouteTable) Lookup(senderID, appID string) (string, bool) {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	// 精确匹配
	if appID != "" {
		if uid, ok := rt.exact[routeKey(senderID, appID)]; ok {
			return uid, true
		}
	}
	// fallback 到 (senderID, "")
	uid, ok := rt.exact[routeKey(senderID, "")]
	return uid, ok
}

func (rt *RouteTable) Bind(senderID, appID, upstreamID string) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	rt.exact[routeKey(senderID, appID)] = upstreamID
	if rt.db != nil {
		now := time.Now().Format(time.RFC3339)
		rt.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, created_at, updated_at) VALUES(?,?,?,'','',?,?)`,
			senderID, appID, upstreamID, now, now)
	}
}

// BindWithMeta 带元数据的绑定（部门、显示名）
func (rt *RouteTable) BindWithMeta(senderID, appID, upstreamID, department, displayName string) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	rt.exact[routeKey(senderID, appID)] = upstreamID
	if rt.db != nil {
		now := time.Now().Format(time.RFC3339)
		rt.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
			senderID, appID, upstreamID, department, displayName, now, now)
	}
}

func (rt *RouteTable) Unbind(senderID, appID string) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	delete(rt.exact, routeKey(senderID, appID))
	if rt.db != nil {
		rt.db.Exec(`DELETE FROM user_routes WHERE sender_id = ? AND app_id = ?`, senderID, appID)
	}
}

func (rt *RouteTable) Migrate(senderID, appID, fromID, toID string) bool {
	rt.mu.Lock(); defer rt.mu.Unlock()
	key := routeKey(senderID, appID)
	current, ok := rt.exact[key]
	if !ok || (fromID != "" && current != fromID) { return false }
	rt.exact[key] = toID
	if rt.db != nil {
		now := time.Now().Format(time.RFC3339)
		rt.db.Exec(`UPDATE user_routes SET upstream_id=?, updated_at=? WHERE sender_id=? AND app_id=?`, toID, now, senderID, appID)
	}
	return true
}

// CompareAndBind 原子比较并绑定 — 仅当当前绑定等于 expectedUID 时才更新为 newUID
// 返回 (是否成功, 当前实际绑定的 upstreamID)
// expectedUID 为空字符串表示期望无绑定
func (rt *RouteTable) CompareAndBind(senderID, appID, expectedUID, newUID string) (bool, string) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	key := routeKey(senderID, appID)
	current, exists := rt.exact[key]
	if expectedUID == "" {
		// 期望无绑定
		if exists {
			return false, current
		}
	} else {
		// 期望绑定到 expectedUID
		if !exists || current != expectedUID {
			return false, current
		}
	}
	rt.exact[key] = newUID
	if rt.db != nil {
		now := time.Now().Format(time.RFC3339)
		rt.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, created_at, updated_at) VALUES(?,?,?,'','',?,?)`,
			senderID, appID, newUID, now, now)
	}
	return true, newUID
}

// AtomicMigrate 原子迁移：比较当前绑定并迁移，同时调整 UserCount
// 返回 true 表示成功迁移
func AtomicMigrate(rt *RouteTable, pool *UpstreamPool, senderID, appID, expectedUID, newUID string) bool {
	ok, _ := rt.CompareAndBind(senderID, appID, expectedUID, newUID)
	if !ok {
		return false
	}
	pool.TransferUserCount(expectedUID, newUID)
	return true
}

// ListRoutes 返回结构化路由列表（v3.8）
func (rt *RouteTable) ListRoutes() []RouteEntry {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	// 如果有 db，从 db 读取完整信息（包含 department/display_name）
	if rt.db != nil {
		rows, err := rt.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, created_at, updated_at FROM user_routes ORDER BY updated_at DESC`)
		if err == nil {
			defer rows.Close()
			var entries []RouteEntry
			for rows.Next() {
				var e RouteEntry
				if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.CreatedAt, &e.UpdatedAt) == nil {
					entries = append(entries, e)
				}
			}
			return entries
		}
	}
	// fallback: 从内存 map
	entries := make([]RouteEntry, 0, len(rt.exact))
	for k, uid := range rt.exact {
		parts := strings.SplitN(k, "|", 2)
		sid := parts[0]
		appID := ""
		if len(parts) > 1 { appID = parts[1] }
		entries = append(entries, RouteEntry{SenderID: sid, AppID: appID, UpstreamID: uid})
	}
	return entries
}

// BindBatch 批量绑定路由条目
func (rt *RouteTable) BindBatch(entries []RouteEntry) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	for _, e := range entries {
		rt.exact[routeKey(e.SenderID, e.AppID)] = e.UpstreamID
		if rt.db != nil {
			rt.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
				e.SenderID, e.AppID, e.UpstreamID, e.Department, e.DisplayName, now, now)
		}
	}
}

// UpdateUserInfo 更新路由表中用户的显示名、邮箱和部门（v3.9）
func (rt *RouteTable) UpdateUserInfo(senderID, displayName, email, department string) {
	rt.mu.Lock(); defer rt.mu.Unlock()
	if rt.db == nil {
		return
	}
	now := time.Now().Format(time.RFC3339)
	rt.db.Exec(`UPDATE user_routes SET display_name=?, department=?, email=?, updated_at=? WHERE sender_id=? AND (display_name='' OR display_name IS NULL OR display_name!=? OR email='' OR email IS NULL OR email!=?)`,
		displayName, department, email, now, senderID, displayName, email)
}

func (rt *RouteTable) Count() int {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	return len(rt.exact)
}

func (rt *RouteTable) CountByUpstream(upstreamID string) int {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	n := 0
	for _, uid := range rt.exact {
		if uid == upstreamID { n++ }
	}
	return n
}

// CountByApp 统计指定 appID 的路由数
func (rt *RouteTable) CountByApp(appID string) int {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	n := 0
	suffix := "|" + appID
	for k := range rt.exact {
		if strings.HasSuffix(k, suffix) { n++ }
	}
	return n
}

// ListByApp 按 Bot 筛选路由
func (rt *RouteTable) ListByApp(appID string) []RouteEntry {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	if rt.db != nil {
		rows, err := rt.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, created_at, updated_at FROM user_routes WHERE app_id = ? ORDER BY updated_at DESC`, appID)
		if err == nil {
			defer rows.Close()
			var entries []RouteEntry
			for rows.Next() {
				var e RouteEntry
				if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.CreatedAt, &e.UpdatedAt) == nil {
					entries = append(entries, e)
				}
			}
			return entries
		}
	}
	// fallback: memory
	suffix := "|" + appID
	var entries []RouteEntry
	for k, uid := range rt.exact {
		if strings.HasSuffix(k, suffix) {
			parts := strings.SplitN(k, "|", 2)
			entries = append(entries, RouteEntry{SenderID: parts[0], AppID: appID, UpstreamID: uid})
		}
	}
	return entries
}

// ListByDepartment 按部门筛选路由（需要 db）
func (rt *RouteTable) ListByDepartment(department string) []RouteEntry {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	if rt.db == nil { return nil }
	rows, err := rt.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, created_at, updated_at FROM user_routes WHERE department = ? ORDER BY updated_at DESC`, department)
	if err != nil { return nil }
	defer rows.Close()
	var entries []RouteEntry
	for rows.Next() {
		var e RouteEntry
		if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.CreatedAt, &e.UpdatedAt) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// RouteStats 路由统计信息
type RouteStats struct {
	TotalRoutes int                `json:"total_routes"`
	TotalUsers  int                `json:"total_users"`
	TotalApps   int                `json:"total_apps"`
	ByUpstream  map[string]int     `json:"by_upstream"`
	ByApp       map[string]int     `json:"by_app"`
	ByDepartment map[string]int    `json:"by_department"`
}

// Stats 统计路由信息
func (rt *RouteTable) Stats() RouteStats {
	rt.mu.RLock(); defer rt.mu.RUnlock()
	stats := RouteStats{
		TotalRoutes:  len(rt.exact),
		ByUpstream:   make(map[string]int),
		ByApp:        make(map[string]int),
		ByDepartment: make(map[string]int),
	}
	users := make(map[string]bool)
	apps := make(map[string]bool)
	for k, uid := range rt.exact {
		parts := strings.SplitN(k, "|", 2)
		sid := parts[0]
		appID := ""
		if len(parts) > 1 { appID = parts[1] }
		users[sid] = true
		if appID != "" { apps[appID] = true }
		stats.ByUpstream[uid]++
		stats.ByApp[appID]++
	}
	stats.TotalUsers = len(users)
	stats.TotalApps = len(apps)

	// 从 db 读取部门统计
	if rt.db != nil {
		rows, err := rt.db.Query(`SELECT COALESCE(department,''), COUNT(*) FROM user_routes GROUP BY department`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var dept string
				var cnt int
				if rows.Scan(&dept, &cnt) == nil && dept != "" {
					stats.ByDepartment[dept] = cnt
				}
			}
		}
	}
	return stats
}

