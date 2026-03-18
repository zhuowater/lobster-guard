// tenant.go — 租户管理器（CRUD + 内存缓存）
// lobster-guard v14.0 — 安全域隔离
package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================
// 租户管理器
// ============================================================

// Tenant 租户（安全域）
type Tenant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	MaxAgents   int    `json:"max_agents"`
	MaxRules    int    `json:"max_rules"`
	Enabled     bool   `json:"enabled"`
	StrictMode  bool   `json:"strict_mode"`
}

// TenantSummary 租户概要（含统计）
type TenantSummary struct {
	Tenant
	HealthScore int   `json:"health_score"`
	IMCalls     int64 `json:"im_calls"`
	LLMCalls    int64 `json:"llm_calls"`
	BlockCount  int64 `json:"block_count"`
	UserCount   int   `json:"user_count"`
}

// TenantManager 租户管理器
type TenantManager struct {
	db      *sql.DB
	tenants map[string]*Tenant
	mu      sync.RWMutex
}

// NewTenantManager 创建租户管理器
func NewTenantManager(db *sql.DB) *TenantManager {
	tm := &TenantManager{
		db:      db,
		tenants: make(map[string]*Tenant),
	}
	tm.initSchema()
	tm.loadAll()
	return tm
}

// initSchema 初始化租户表 + 默认租户 + 已有表 tenant_id 列
func (tm *TenantManager) initSchema() {
	// 租户表
	tm.db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		max_agents INTEGER DEFAULT 0,
		max_rules INTEGER DEFAULT 0,
		enabled INTEGER DEFAULT 1,
		strict_mode INTEGER DEFAULT 0
	)`)

	// 默认租户
	tm.db.Exec(`INSERT OR IGNORE INTO tenants (id, name, description, created_at, enabled) 
		VALUES ('default', '默认租户', '系统默认安全域', ?, 1)`,
		time.Now().UTC().Format(time.RFC3339))

	// 已有表加 tenant_id 列（忽略 "duplicate column" 错误）
	tables := []string{"audit_log", "llm_calls", "llm_tool_calls", "session_tags", "prompt_versions", "reports"}
	for _, t := range tables {
		tm.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN tenant_id TEXT DEFAULT 'default'`, t))
	}

	// 索引
	tm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_log_tenant ON audit_log(tenant_id)`)
	tm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_calls_tenant ON llm_calls(tenant_id)`)
	tm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_tenant ON llm_tool_calls(tenant_id)`)

	log.Println("[初始化] ✅ 租户体系 schema 就绪")
}

// loadAll 从数据库加载所有租户到内存缓存
func (tm *TenantManager) loadAll() {
	rows, err := tm.db.Query(`SELECT id, name, description, created_at, max_agents, max_rules, enabled, strict_mode FROM tenants`)
	if err != nil {
		log.Printf("[租户] 加载失败: %v", err)
		return
	}
	defer rows.Close()

	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tenants = make(map[string]*Tenant)

	for rows.Next() {
		t := &Tenant{}
		var enabled, strict int
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt, &t.MaxAgents, &t.MaxRules, &enabled, &strict); err != nil {
			continue
		}
		t.Enabled = enabled != 0
		t.StrictMode = strict != 0
		tm.tenants[t.ID] = t
	}
	log.Printf("[租户] 加载了 %d 个租户", len(tm.tenants))
}

// List 返回所有租户列表
func (tm *TenantManager) List() []*Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	list := make([]*Tenant, 0, len(tm.tenants))
	for _, t := range tm.tenants {
		list = append(list, t)
	}
	return list
}

// Get 获取单个租户
func (tm *TenantManager) Get(id string) *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tenants[id]
}

// Exists 租户是否存在
func (tm *TenantManager) Exists(id string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, ok := tm.tenants[id]
	return ok
}

// Create 创建租户
func (tm *TenantManager) Create(t *Tenant) error {
	if t.ID == "" {
		return fmt.Errorf("租户 ID 不能为空")
	}
	if t.Name == "" {
		return fmt.Errorf("租户名称不能为空")
	}
	if tm.Exists(t.ID) {
		return fmt.Errorf("租户 %q 已存在", t.ID)
	}
	t.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if !t.Enabled {
		t.Enabled = true
	}

	_, err := tm.db.Exec(`INSERT INTO tenants (id, name, description, created_at, max_agents, max_rules, enabled, strict_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Description, t.CreatedAt, t.MaxAgents, t.MaxRules,
		boolToInt(t.Enabled), boolToInt(t.StrictMode))
	if err != nil {
		return fmt.Errorf("创建租户失败: %w", err)
	}

	// Copy to avoid aliasing with caller's pointer
	copy := *t
	tm.mu.Lock()
	tm.tenants[t.ID] = &copy
	tm.mu.Unlock()

	log.Printf("[租户] 创建: %s (%s)", t.ID, t.Name)
	return nil
}

// Update 更新租户
func (tm *TenantManager) Update(t *Tenant) error {
	if !tm.Exists(t.ID) {
		return fmt.Errorf("租户 %q 不存在", t.ID)
	}

	_, err := tm.db.Exec(`UPDATE tenants SET name=?, description=?, max_agents=?, max_rules=?, enabled=?, strict_mode=? WHERE id=?`,
		t.Name, t.Description, t.MaxAgents, t.MaxRules,
		boolToInt(t.Enabled), boolToInt(t.StrictMode), t.ID)
	if err != nil {
		return fmt.Errorf("更新租户失败: %w", err)
	}

	tm.mu.Lock()
	existing := tm.tenants[t.ID]
	if existing != nil {
		existing.Name = t.Name
		existing.Description = t.Description
		existing.MaxAgents = t.MaxAgents
		existing.MaxRules = t.MaxRules
		existing.Enabled = t.Enabled
		existing.StrictMode = t.StrictMode
	}
	tm.mu.Unlock()

	log.Printf("[租户] 更新: %s", t.ID)
	return nil
}

// Delete 删除租户（不允许删除 default）
func (tm *TenantManager) Delete(id string) error {
	if id == "default" {
		return fmt.Errorf("不能删除默认租户")
	}
	if !tm.Exists(id) {
		return fmt.Errorf("租户 %q 不存在", id)
	}

	_, err := tm.db.Exec(`DELETE FROM tenants WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("删除租户失败: %w", err)
	}

	tm.mu.Lock()
	delete(tm.tenants, id)
	tm.mu.Unlock()

	log.Printf("[租户] 删除: %s", id)
	return nil
}

// GetSummary 获取租户概要（含统计数据）
func (tm *TenantManager) GetSummary(id string) *TenantSummary {
	t := tm.Get(id)
	if t == nil {
		return nil
	}
	s := &TenantSummary{Tenant: *t}

	// IM 调用数
	tm.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE tenant_id=?`, id).Scan(&s.IMCalls)
	// IM 拦截数
	tm.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE tenant_id=? AND action='block'`, id).Scan(&s.BlockCount)
	// LLM 调用数
	tm.db.QueryRow(`SELECT COUNT(*) FROM llm_calls WHERE tenant_id=?`, id).Scan(&s.LLMCalls)
	// 用户数
	tm.db.QueryRow(`SELECT COUNT(DISTINCT sender_id) FROM audit_log WHERE tenant_id=?`, id).Scan(&s.UserCount)

	return s
}

// ListSummaries 返回所有租户概要
func (tm *TenantManager) ListSummaries() []*TenantSummary {
	tenants := tm.List()
	summaries := make([]*TenantSummary, 0, len(tenants))
	for _, t := range tenants {
		s := tm.GetSummary(t.ID)
		if s != nil {
			summaries = append(summaries, s)
		}
	}
	return summaries
}

// ============================================================
// 租户感知查询辅助
// ============================================================

// TenantFilter 构建 tenant_id WHERE 子句
// tenantID="" → default, "all" → 不加过滤, 其他 → 指定租户
func TenantFilter(tenantID string) (clause string, args []interface{}) {
	if tenantID == "" {
		tenantID = "default"
	}
	if tenantID == "all" {
		return "", nil
	}
	return " AND tenant_id = ?", []interface{}{tenantID}
}

// ParseTenantParam 从 URL Query 中提取 tenant 参数
func ParseTenantParam(tenantRaw string) string {
	if tenantRaw == "" {
		return "default"
	}
	return tenantRaw
}

// boolToInt bool → int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
