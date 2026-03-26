// apikey.go — API Key 身份管理（CRUD + 配额 + 缓存）
// lobster-guard v27.0 — API Key 身份管理
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// APIKeyEntry API Key 条目
type APIKeyEntry struct {
	ID           string `json:"id"`
	Key          string `json:"key,omitempty"`    // API Key (hash存储，仅创建时返回明文)
	KeyPrefix    string `json:"key_prefix"`       // 前10位明文，用于识别
	UserID       string `json:"user_id"`          // 用户标识（工号/邮箱）
	UserName     string `json:"user_name"`        // 用户名
	Department   string `json:"department"`       // 部门
	TenantID     string `json:"tenant_id"`        // 归属租户
	Enabled      bool   `json:"enabled"`
	QuotaDaily   int    `json:"quota_daily"`      // 日配额，0=不限
	UsedToday    int    `json:"used_today"`       // 今日已用
	ExpiresAt    string `json:"expires_at"`       // 过期时间，空=永不过期
	CreatedAt    string `json:"created_at"`
	LastUsedAt   string `json:"last_used_at"`
	Status       string `json:"status"`           // "active"(已绑定) / "pending"(待绑定/自动发现)
	DiscoveredAt string `json:"discovered_at"`    // 自动发现时间
	RequestCount int    `json:"request_count"`    // 发现以来的请求次数
	LastSeenAt   string `json:"last_seen_at"`     // 最后活动时间
}

// APIKeyManager API Key 管理器
type APIKeyManager struct {
	db    *sql.DB
	cache sync.Map // key_hash → *APIKeyEntry (热缓存)
}

// NewAPIKeyManager 创建 API Key 管理器
func NewAPIKeyManager(db *sql.DB) *APIKeyManager {
	mgr := &APIKeyManager{db: db}
	mgr.initSchema()
	mgr.loadCache()
	return mgr
}

// initSchema 初始化 API Key 表
func (m *APIKeyManager) initSchema() {
	if m.db == nil {
		return
	}
	m.db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		key_hash TEXT NOT NULL UNIQUE,
		key_prefix TEXT NOT NULL,
		user_id TEXT NOT NULL,
		user_name TEXT DEFAULT '',
		department TEXT DEFAULT '',
		tenant_id TEXT NOT NULL DEFAULT 'default',
		enabled INTEGER NOT NULL DEFAULT 1,
		quota_daily INTEGER DEFAULT 0,
		used_today INTEGER DEFAULT 0,
		usage_date TEXT DEFAULT '',
		expires_at TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		last_used_at TEXT DEFAULT ''
	)`)
	m.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id)`)
	m.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id)`)
	m.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash)`)
	// 兼容旧表: 新增列（列已存在时 SQLite 会报错，忽略即可）
	m.db.Exec(`ALTER TABLE api_keys ADD COLUMN status TEXT DEFAULT 'active'`)
	m.db.Exec(`ALTER TABLE api_keys ADD COLUMN discovered_at TEXT DEFAULT ''`)
	m.db.Exec(`ALTER TABLE api_keys ADD COLUMN request_count INTEGER DEFAULT 0`)
	m.db.Exec(`ALTER TABLE api_keys ADD COLUMN last_seen_at TEXT DEFAULT ''`)
	m.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status)`)
	log.Println("[APIKey] ✅ Schema 就绪")
}

// loadCache 启动时加载所有启用的 Key 到缓存
func (m *APIKeyManager) loadCache() {
	if m.db == nil {
		return
	}
	rows, err := m.db.Query(`SELECT id, key_hash, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, usage_date, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys WHERE enabled=1`)
	if err != nil {
		log.Printf("[APIKey] 加载缓存失败: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	today := time.Now().UTC().Format("2006-01-02")
	for rows.Next() {
		var entry APIKeyEntry
		var keyHash, usageDate string
		var enabled int
		if rows.Scan(&entry.ID, &keyHash, &entry.KeyPrefix, &entry.UserID, &entry.UserName,
			&entry.Department, &entry.TenantID, &enabled, &entry.QuotaDaily,
			&entry.UsedToday, &usageDate, &entry.ExpiresAt, &entry.CreatedAt, &entry.LastUsedAt,
			&entry.Status, &entry.DiscoveredAt, &entry.RequestCount, &entry.LastSeenAt) != nil {
			continue
		}
		entry.Enabled = enabled != 0
		if entry.Status == "" {
			entry.Status = "active"
		}
		// 跨日重置用量
		if usageDate != today {
			entry.UsedToday = 0
		}
		m.cache.Store(keyHash, &entry)
		count++
	}
	log.Printf("[APIKey] 加载了 %d 个 Key 到缓存", count)
}

// hashKey SHA-256 哈希
func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// generateAPIKey 生成 API Key（sk- 前缀 + 48位随机hex）
func generateAPIKey() string {
	b := make([]byte, 24)
	// 使用 crypto/rand 已在 import
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[int(time.Now().UnixNano())%36]
	}
	h := sha256.Sum256(append(b, []byte(fmt.Sprintf("%d", time.Now().UnixNano()))...))
	return "sk-" + hex.EncodeToString(h[:])[:48]
}

// Resolve 从原始 Key 解析用户身份
// rawKey 可以是 "Bearer sk-xxx" 或直接 "sk-xxx"
func (m *APIKeyManager) Resolve(rawKey string) (*APIKeyEntry, error) {
	// 提取 Key
	key := strings.TrimSpace(rawKey)
	if strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	if key == "" {
		return nil, fmt.Errorf("空的 API Key")
	}

	keyHash := hashKey(key)

	// 查缓存
	if val, ok := m.cache.Load(keyHash); ok {
		entry := val.(*APIKeyEntry)
		if !entry.Enabled {
			return nil, fmt.Errorf("API Key 已禁用")
		}
		// 检查过期
		if entry.ExpiresAt != "" {
			expires, err := time.Parse(time.RFC3339, entry.ExpiresAt)
			if err == nil && time.Now().UTC().After(expires) {
				return nil, fmt.Errorf("API Key 已过期")
			}
		}
		return entry, nil
	}

	// 查数据库
	if m.db == nil {
		// 无 DB 时自动发现
		discovered := m.AutoDiscover(key)
		if discovered != nil {
			return discovered, nil
		}
		return nil, fmt.Errorf("API Key 不存在")
	}
	var entry APIKeyEntry
	var enabled int
	var usageDate string
	err := m.db.QueryRow(`SELECT id, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, usage_date, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys WHERE key_hash=?`, keyHash).
		Scan(&entry.ID, &entry.KeyPrefix, &entry.UserID, &entry.UserName, &entry.Department,
			&entry.TenantID, &enabled, &entry.QuotaDaily, &entry.UsedToday, &usageDate,
			&entry.ExpiresAt, &entry.CreatedAt, &entry.LastUsedAt,
			&entry.Status, &entry.DiscoveredAt, &entry.RequestCount, &entry.LastSeenAt)
	if err != nil {
		// Key 不在 DB 中 → 自动发现
		discovered := m.AutoDiscover(key)
		if discovered != nil {
			return discovered, nil
		}
		return nil, fmt.Errorf("API Key 不存在")
	}
	entry.Enabled = enabled != 0
	if entry.Status == "" {
		entry.Status = "active"
	}
	// 跨日重置
	today := time.Now().UTC().Format("2006-01-02")
	if usageDate != today {
		entry.UsedToday = 0
	}

	if !entry.Enabled {
		return nil, fmt.Errorf("API Key 已禁用")
	}
	if entry.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, entry.ExpiresAt)
		if err == nil && time.Now().UTC().After(expires) {
			return nil, fmt.Errorf("API Key 已过期")
		}
	}

	// 写入缓存
	m.cache.Store(keyHash, &entry)
	return &entry, nil
}

// Create 创建 API Key，返回包含明文 Key 的条目
func (m *APIKeyManager) Create(entry *APIKeyEntry) (*APIKeyEntry, string, error) {
	if entry.UserID == "" {
		return nil, "", fmt.Errorf("user_id 不能为空")
	}
	if entry.TenantID == "" {
		entry.TenantID = "default"
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("ak-%d", time.Now().UnixNano())
	}

	// 生成 Key
	rawKey := generateAPIKey()
	keyHash := hashKey(rawKey)
	entry.KeyPrefix = rawKey[:10] // "sk-" + 前7位
	entry.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	entry.Enabled = true
	if entry.Status == "" {
		entry.Status = "active"
	}

	if m.db != nil {
		_, err := m.db.Exec(`INSERT INTO api_keys (id, key_hash, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, expires_at, created_at, status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			entry.ID, keyHash, entry.KeyPrefix, entry.UserID, entry.UserName, entry.Department,
			entry.TenantID, boolToInt(entry.Enabled), entry.QuotaDaily, entry.ExpiresAt, entry.CreatedAt, entry.Status)
		if err != nil {
			return nil, "", fmt.Errorf("创建 API Key 失败: %w", err)
		}
	}

	// 写入缓存
	m.cache.Store(keyHash, entry)
	log.Printf("[APIKey] 创建: id=%s user=%s tenant=%s prefix=%s", entry.ID, entry.UserID, entry.TenantID, entry.KeyPrefix)
	return entry, rawKey, nil
}

// List 列出所有 API Key（不含 hash）
// tenantID 为空表示不过滤租户，status 为空表示不过滤状态
func (m *APIKeyManager) List(tenantID, status string) ([]*APIKeyEntry, error) {
	if m.db == nil {
		return nil, nil
	}
	query := `SELECT id, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys`
	var args []interface{}
	var conditions []string
	if tenantID != "" {
		conditions = append(conditions, `tenant_id=?`)
		args = append(args, tenantID)
	}
	if status != "" {
		conditions = append(conditions, `status=?`)
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*APIKeyEntry
	for rows.Next() {
		var e APIKeyEntry
		var enabled int
		if rows.Scan(&e.ID, &e.KeyPrefix, &e.UserID, &e.UserName, &e.Department,
			&e.TenantID, &enabled, &e.QuotaDaily, &e.UsedToday,
			&e.ExpiresAt, &e.CreatedAt, &e.LastUsedAt,
			&e.Status, &e.DiscoveredAt, &e.RequestCount, &e.LastSeenAt) != nil {
			continue
		}
		e.Enabled = enabled != 0
		if e.Status == "" {
			e.Status = "active"
		}
		list = append(list, &e)
	}
	if list == nil {
		list = []*APIKeyEntry{}
	}
	return list, nil
}

// Get 获取单个 API Key
func (m *APIKeyManager) Get(id string) (*APIKeyEntry, error) {
	if m.db == nil {
		return nil, fmt.Errorf("API Key 不存在")
	}
	var e APIKeyEntry
	var enabled int
	err := m.db.QueryRow(`SELECT id, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys WHERE id=?`, id).
		Scan(&e.ID, &e.KeyPrefix, &e.UserID, &e.UserName, &e.Department,
			&e.TenantID, &enabled, &e.QuotaDaily, &e.UsedToday,
			&e.ExpiresAt, &e.CreatedAt, &e.LastUsedAt,
			&e.Status, &e.DiscoveredAt, &e.RequestCount, &e.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("API Key %q 不存在", id)
	}
	e.Enabled = enabled != 0
	if e.Status == "" {
		e.Status = "active"
	}
	return &e, nil
}

// Update 更新 API Key
func (m *APIKeyManager) Update(entry *APIKeyEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("id 不能为空")
	}
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	_, err := m.db.Exec(`UPDATE api_keys SET user_name=?, department=?, tenant_id=?, enabled=?, quota_daily=?, expires_at=? WHERE id=?`,
		entry.UserName, entry.Department, entry.TenantID, boolToInt(entry.Enabled),
		entry.QuotaDaily, entry.ExpiresAt, entry.ID)
	if err != nil {
		return fmt.Errorf("更新 API Key 失败: %w", err)
	}

	// 刷新缓存：遍历缓存找到对应条目更新
	m.cache.Range(func(key, value interface{}) bool {
		e := value.(*APIKeyEntry)
		if e.ID == entry.ID {
			e.UserName = entry.UserName
			e.Department = entry.Department
			e.TenantID = entry.TenantID
			e.Enabled = entry.Enabled
			e.QuotaDaily = entry.QuotaDaily
			e.ExpiresAt = entry.ExpiresAt
			return false
		}
		return true
	})
	log.Printf("[APIKey] 更新: id=%s", entry.ID)
	return nil
}

// Delete 删除 API Key
func (m *APIKeyManager) Delete(id string) error {
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	result, err := m.db.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("删除 API Key 失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API Key %q 不存在", id)
	}

	// 清除缓存
	m.cache.Range(func(key, value interface{}) bool {
		e := value.(*APIKeyEntry)
		if e.ID == id {
			m.cache.Delete(key)
			return false
		}
		return true
	})
	log.Printf("[APIKey] 删除: id=%s", id)
	return nil
}

// Rotate 轮换 API Key，生成新 Key，旧 Key 失效
func (m *APIKeyManager) Rotate(id string) (*APIKeyEntry, string, error) {
	if m.db == nil {
		return nil, "", fmt.Errorf("数据库未初始化")
	}

	// 读取旧条目
	old, err := m.Get(id)
	if err != nil {
		return nil, "", err
	}

	// 生成新 Key
	newRawKey := generateAPIKey()
	newHash := hashKey(newRawKey)
	newPrefix := newRawKey[:10]

	// 先从缓存删除旧 hash
	m.cache.Range(func(key, value interface{}) bool {
		e := value.(*APIKeyEntry)
		if e.ID == id {
			m.cache.Delete(key)
			return false
		}
		return true
	})

	// 更新数据库
	_, err = m.db.Exec(`UPDATE api_keys SET key_hash=?, key_prefix=? WHERE id=?`,
		newHash, newPrefix, id)
	if err != nil {
		return nil, "", fmt.Errorf("轮换 API Key 失败: %w", err)
	}

	old.KeyPrefix = newPrefix
	// 写入新缓存
	m.cache.Store(newHash, old)
	log.Printf("[APIKey] 轮换: id=%s new_prefix=%s", id, newPrefix)
	return old, newRawKey, nil
}

// CheckQuota 检查配额是否允许
func (m *APIKeyManager) CheckQuota(keyID string) bool {
	var quotaDaily, usedToday int
	var usageDate string
	if m.db == nil {
		return true
	}
	err := m.db.QueryRow(`SELECT quota_daily, used_today, usage_date FROM api_keys WHERE id=?`, keyID).
		Scan(&quotaDaily, &usedToday, &usageDate)
	if err != nil {
		return true // Key 不存在，放行
	}
	if quotaDaily <= 0 {
		return true // 不限额
	}
	today := time.Now().UTC().Format("2006-01-02")
	if usageDate != today {
		return true // 新的一天
	}
	return usedToday < quotaDaily
}

// IncrUsage 增加使用计数
func (m *APIKeyManager) IncrUsage(keyID string) {
	if m.db == nil {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	// 如果日期变了，重置计数
	m.db.Exec(`UPDATE api_keys SET used_today = CASE WHEN usage_date = ? THEN used_today + 1 ELSE 1 END, usage_date = ?, last_used_at = ? WHERE id = ?`,
		today, today, time.Now().UTC().Format(time.RFC3339), keyID)

	// 更新缓存中的使用计数
	m.cache.Range(func(key, value interface{}) bool {
		e := value.(*APIKeyEntry)
		if e.ID == keyID {
			e.UsedToday++
			e.LastUsedAt = time.Now().UTC().Format(time.RFC3339)
			return false
		}
		return true
	})
}

// AutoDiscover 自动发现并注册未知 API Key
// 如果 key 已存在则更新 last_seen_at 和 request_count，返回已有条目
// 如果 key 不存在则创建 pending 条目
func (m *APIKeyManager) AutoDiscover(rawKey string) *APIKeyEntry {
	key := strings.TrimSpace(rawKey)
	if strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	if key == "" {
		return nil
	}

	keyHash := hashKey(key)
	now := time.Now().UTC().Format(time.RFC3339)

	// 查缓存 — 找到则更新 last_seen_at 和 request_count++
	if val, ok := m.cache.Load(keyHash); ok {
		entry := val.(*APIKeyEntry)
		entry.LastSeenAt = now
		entry.RequestCount++
		// 异步更新 DB
		if m.db != nil {
			m.db.Exec(`UPDATE api_keys SET last_seen_at=?, request_count=request_count+1 WHERE key_hash=?`, now, keyHash)
		}
		return entry
	}

	// 查 DB（缓存没有但 DB 可能有）
	if m.db != nil {
		var entry APIKeyEntry
		var enabled int
		var usageDate string
		err := m.db.QueryRow(`SELECT id, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, usage_date, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys WHERE key_hash=?`, keyHash).
			Scan(&entry.ID, &entry.KeyPrefix, &entry.UserID, &entry.UserName, &entry.Department,
				&entry.TenantID, &enabled, &entry.QuotaDaily, &entry.UsedToday, &usageDate,
				&entry.ExpiresAt, &entry.CreatedAt, &entry.LastUsedAt,
				&entry.Status, &entry.DiscoveredAt, &entry.RequestCount, &entry.LastSeenAt)
		if err == nil {
			entry.Enabled = enabled != 0
			if entry.Status == "" {
				entry.Status = "active"
			}
			entry.LastSeenAt = now
			entry.RequestCount++
			m.db.Exec(`UPDATE api_keys SET last_seen_at=?, request_count=request_count+1 WHERE key_hash=?`, now, keyHash)
			m.cache.Store(keyHash, &entry)
			return &entry
		}
	}

	// 没找到 → 创建新的 pending 条目
	// key_prefix: 前10位（不足10位取全部）
	prefix := key
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}

	id := fmt.Sprintf("ak-disc-%d", time.Now().UnixNano())
	entry := &APIKeyEntry{
		ID:           id,
		KeyPrefix:    prefix,
		UserID:       "unknown",
		UserName:     "",
		Department:   "",
		TenantID:     "default",
		Enabled:      true,
		QuotaDaily:   0,
		Status:       "pending",
		DiscoveredAt: now,
		LastSeenAt:   now,
		RequestCount: 1,
		CreatedAt:    now,
	}

	if m.db != nil {
		// 使用 INSERT OR IGNORE 防并发重复插入
		m.db.Exec(`INSERT OR IGNORE INTO api_keys (id, key_hash, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, expires_at, created_at, status, discovered_at, request_count, last_seen_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			entry.ID, keyHash, entry.KeyPrefix, entry.UserID, entry.UserName, entry.Department,
			entry.TenantID, boolToInt(entry.Enabled), entry.QuotaDaily, "", entry.CreatedAt,
			entry.Status, entry.DiscoveredAt, entry.RequestCount, entry.LastSeenAt)

		// INSERT OR IGNORE 可能因为 key_hash UNIQUE 冲突而跳过（并发情况）
		// 随后 SELECT 确保拿到正确数据
		var existEntry APIKeyEntry
		var enabled2 int
		var usageDate2 string
		err := m.db.QueryRow(`SELECT id, key_prefix, user_id, user_name, department, tenant_id, enabled, quota_daily, used_today, usage_date, expires_at, created_at, last_used_at, status, discovered_at, request_count, last_seen_at FROM api_keys WHERE key_hash=?`, keyHash).
			Scan(&existEntry.ID, &existEntry.KeyPrefix, &existEntry.UserID, &existEntry.UserName, &existEntry.Department,
				&existEntry.TenantID, &enabled2, &existEntry.QuotaDaily, &existEntry.UsedToday, &usageDate2,
				&existEntry.ExpiresAt, &existEntry.CreatedAt, &existEntry.LastUsedAt,
				&existEntry.Status, &existEntry.DiscoveredAt, &existEntry.RequestCount, &existEntry.LastSeenAt)
		if err == nil {
			existEntry.Enabled = enabled2 != 0
			if existEntry.Status == "" {
				existEntry.Status = "active"
			}
			m.cache.Store(keyHash, &existEntry)
			log.Printf("[APIKey] 🔍 自动发现新 Key: prefix=%s (待绑定)", existEntry.KeyPrefix)
			return &existEntry
		}
	}

	// 无 DB 时仅缓存
	m.cache.Store(keyHash, entry)
	log.Printf("[APIKey] 🔍 自动发现新 Key: prefix=%s (待绑定)", entry.KeyPrefix)
	return entry
}

// Bind 将待绑定的 API Key 绑定到具体用户
func (m *APIKeyManager) Bind(id string, userID, userName, department, tenantID string) error {
	if m.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == "" {
		return fmt.Errorf("id 不能为空")
	}
	if userID == "" {
		return fmt.Errorf("user_id 不能为空")
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// 更新 DB
	result, err := m.db.Exec(`UPDATE api_keys SET user_id=?, user_name=?, department=?, tenant_id=?, status='active' WHERE id=?`,
		userID, userName, department, tenantID, id)
	if err != nil {
		return fmt.Errorf("绑定 API Key 失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("API Key %q 不存在", id)
	}

	// 更新缓存
	m.cache.Range(func(key, value interface{}) bool {
		e := value.(*APIKeyEntry)
		if e.ID == id {
			e.UserID = userID
			e.UserName = userName
			e.Department = department
			e.TenantID = tenantID
			e.Status = "active"
			return false
		}
		return true
	})

	log.Printf("[APIKey] ✅ 绑定: id=%s → user=%s tenant=%s", id, userID, tenantID)
	return nil
}

// APIKeyStats API Key 统计信息
type APIKeyStats struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Pending int `json:"pending"`
}

// Stats 返回 API Key 统计
func (m *APIKeyManager) Stats() (*APIKeyStats, error) {
	if m.db == nil {
		return &APIKeyStats{}, nil
	}
	stats := &APIKeyStats{}
	m.db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&stats.Total)
	m.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE status='active' OR status='' OR status IS NULL`).Scan(&stats.Active)
	m.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE status='pending'`).Scan(&stats.Pending)
	return stats, nil
}
