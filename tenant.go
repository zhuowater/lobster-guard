// tenant.go — 租户管理器（CRUD + 内存缓存）
// lobster-guard v14.0 — 安全域隔离
package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
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
	MemberCount int   `json:"member_count"`
}

// TenantMember 租户成员映射
type TenantMember struct {
	ID          int    `json:"id"`
	TenantID    string `json:"tenant_id"`
	MatchType   string `json:"match_type"`   // "sender_id" | "app_id" | "pattern"
	MatchValue  string `json:"match_value"`  // 具体值或通配符模式
	Description string `json:"description"`  // 备注
	CreatedAt   string `json:"created_at"`
}

// TenantManager 租户管理器
type TenantManager struct {
	db      *sql.DB
	tenants map[string]*Tenant
	mu      sync.RWMutex

	// 成员映射缓存（v14.0 闭环）
	memberMu      sync.RWMutex
	senderMap     map[string]string // sender_id → tenant_id (精确匹配)
	appMap        map[string]string // app_id → tenant_id (精确匹配)
	patternRules  []patternEntry    // pattern 规则列表
}

// patternEntry 通配符匹配规则
type patternEntry struct {
	Pattern  string
	TenantID string
}

// NewTenantManager 创建租户管理器
func NewTenantManager(db *sql.DB) *TenantManager {
	tm := &TenantManager{
		db:        db,
		tenants:   make(map[string]*Tenant),
		senderMap: make(map[string]string),
		appMap:    make(map[string]string),
	}
	tm.initSchema()
	tm.loadAll()
	tm.loadMemberCache()
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

	// 租户成员映射表（v14.0 闭环）
	tm.db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tenant_id TEXT NOT NULL,
		match_type TEXT NOT NULL,
		match_value TEXT NOT NULL,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		UNIQUE(tenant_id, match_type, match_value)
	)`)
	tm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant ON tenant_members(tenant_id)`)

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

	// 租户安全配置表（v14.0 策略）
	tm.initConfigSchema()

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
	// 成员数
	tm.db.QueryRow(`SELECT COUNT(*) FROM tenant_members WHERE tenant_id=?`, id).Scan(&s.MemberCount)

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
// 成员映射管理（v14.0 闭环）
// ============================================================

// loadMemberCache 从数据库加载成员映射到内存缓存
func (tm *TenantManager) loadMemberCache() {
	rows, err := tm.db.Query(`SELECT tenant_id, match_type, match_value FROM tenant_members`)
	if err != nil {
		log.Printf("[租户] 加载成员映射失败: %v", err)
		return
	}
	defer rows.Close()

	tm.memberMu.Lock()
	defer tm.memberMu.Unlock()
	tm.senderMap = make(map[string]string)
	tm.appMap = make(map[string]string)
	tm.patternRules = nil

	count := 0
	for rows.Next() {
		var tenantID, matchType, matchValue string
		if rows.Scan(&tenantID, &matchType, &matchValue) != nil {
			continue
		}
		switch matchType {
		case "sender_id":
			tm.senderMap[matchValue] = tenantID
		case "app_id":
			tm.appMap[matchValue] = tenantID
		case "pattern":
			tm.patternRules = append(tm.patternRules, patternEntry{Pattern: matchValue, TenantID: tenantID})
		}
		count++
	}
	log.Printf("[租户] 加载了 %d 条成员映射 (sender=%d, app=%d, pattern=%d)",
		count, len(tm.senderMap), len(tm.appMap), len(tm.patternRules))
}

// ResolveTenant 根据 sender_id 和 app_id 解析租户
// 优先级：精确 sender_id > 精确 app_id > 通配符 pattern > "default"
func (tm *TenantManager) ResolveTenant(senderID, appID string) string {
	tm.memberMu.RLock()
	defer tm.memberMu.RUnlock()

	// 1. 精确匹配 sender_id
	if senderID != "" {
		if tid, ok := tm.senderMap[senderID]; ok {
			return tid
		}
	}

	// 2. 精确匹配 app_id
	if appID != "" {
		if tid, ok := tm.appMap[appID]; ok {
			return tid
		}
	}

	// 3. 通配符 pattern 匹配（用 filepath.Match 支持 * 和 ? ）
	for _, p := range tm.patternRules {
		if senderID != "" {
			if matched, _ := filepath.Match(p.Pattern, senderID); matched {
				return p.TenantID
			}
		}
		if appID != "" {
			if matched, _ := filepath.Match(p.Pattern, appID); matched {
				return p.TenantID
			}
		}
	}

	// 4. 兜底
	return "default"
}

// AddMember 添加成员映射
func (tm *TenantManager) AddMember(tenantID, matchType, matchValue, description string) error {
	if tenantID == "" || matchType == "" || matchValue == "" {
		return fmt.Errorf("tenant_id, match_type, match_value 不能为空")
	}
	if matchType != "sender_id" && matchType != "app_id" && matchType != "pattern" {
		return fmt.Errorf("match_type 必须是 sender_id/app_id/pattern")
	}
	if !tm.Exists(tenantID) {
		return fmt.Errorf("租户 %q 不存在", tenantID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := tm.db.Exec(`INSERT INTO tenant_members (tenant_id, match_type, match_value, description, created_at)
		VALUES (?, ?, ?, ?, ?)`, tenantID, matchType, matchValue, description, now)
	if err != nil {
		return fmt.Errorf("添加成员映射失败: %w", err)
	}

	// 刷新内存缓存
	tm.loadMemberCache()
	log.Printf("[租户] 添加成员映射: %s/%s=%s (%s)", tenantID, matchType, matchValue, description)
	return nil
}

// RemoveMember 删除成员映射
func (tm *TenantManager) RemoveMember(id int) error {
	result, err := tm.db.Exec(`DELETE FROM tenant_members WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("删除成员映射失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("成员映射 #%d 不存在", id)
	}
	// 刷新内存缓存
	tm.loadMemberCache()
	log.Printf("[租户] 删除成员映射 #%d", id)
	return nil
}

// ListMembers 列出租户的所有成员映射
func (tm *TenantManager) ListMembers(tenantID string) ([]TenantMember, error) {
	rows, err := tm.db.Query(`SELECT id, tenant_id, match_type, match_value, description, created_at FROM tenant_members WHERE tenant_id=? ORDER BY id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []TenantMember
	for rows.Next() {
		var m TenantMember
		if rows.Scan(&m.ID, &m.TenantID, &m.MatchType, &m.MatchValue, &m.Description, &m.CreatedAt) != nil {
			continue
		}
		members = append(members, m)
	}
	if members == nil {
		members = []TenantMember{}
	}
	return members, nil
}

// ============================================================
// 租户安全配置（v14.0 策略）
// ============================================================

// TenantConfig 租户安全策略配置
type TenantConfig struct {
	TenantID       string `json:"tenant_id"`
	DisabledRules  string `json:"disabled_rules"`    // 逗号分隔的禁用规则名
	ExtraRulesYAML string `json:"extra_rules_yaml"`  // 租户额外规则（YAML格式）
	StrictMode     bool   `json:"strict_mode"`
	CanaryEnabled  bool   `json:"canary_enabled"`
	BudgetEnabled  bool   `json:"budget_enabled"`
	BudgetMaxTokens int   `json:"budget_max_tokens"` // 0=使用全局
	BudgetMaxTools  int   `json:"budget_max_tools"`  // 0=使用全局
	ToolBlacklist  string `json:"tool_blacklist"`     // 逗号分隔
	AlertLevel     string `json:"alert_level"`        // low/medium/high/critical
	AlertWebhook   string `json:"alert_webhook"`      // 租户专属 webhook
	UpdatedAt      string `json:"updated_at"`
}

// initConfigSchema 初始化租户安全配置表
func (tm *TenantManager) initConfigSchema() {
	tm.db.Exec(`CREATE TABLE IF NOT EXISTS tenant_configs (
		tenant_id TEXT PRIMARY KEY,
		disabled_rules TEXT DEFAULT '',
		extra_rules_yaml TEXT DEFAULT '',
		strict_mode INTEGER DEFAULT 0,
		canary_enabled INTEGER DEFAULT 1,
		budget_enabled INTEGER DEFAULT 1,
		budget_max_tokens INTEGER DEFAULT 0,
		budget_max_tools INTEGER DEFAULT 0,
		tool_blacklist TEXT DEFAULT '',
		alert_level TEXT DEFAULT 'high',
		alert_webhook TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`)
}

// GetConfig 获取租户安全配置（不存在则返回默认值）
func (tm *TenantManager) GetConfig(tenantID string) *TenantConfig {
	cfg := &TenantConfig{
		TenantID:      tenantID,
		CanaryEnabled: true,
		BudgetEnabled: true,
		AlertLevel:    "high",
	}

	var strict, canary, budget int
	err := tm.db.QueryRow(`SELECT disabled_rules, extra_rules_yaml, strict_mode, canary_enabled, budget_enabled, budget_max_tokens, budget_max_tools, tool_blacklist, alert_level, alert_webhook, updated_at FROM tenant_configs WHERE tenant_id=?`, tenantID).Scan(
		&cfg.DisabledRules, &cfg.ExtraRulesYAML, &strict, &canary, &budget,
		&cfg.BudgetMaxTokens, &cfg.BudgetMaxTools, &cfg.ToolBlacklist,
		&cfg.AlertLevel, &cfg.AlertWebhook, &cfg.UpdatedAt)
	if err != nil {
		// 不存在，返回默认值
		return cfg
	}
	cfg.StrictMode = strict != 0
	cfg.CanaryEnabled = canary != 0
	cfg.BudgetEnabled = budget != 0
	return cfg
}

// UpdateConfig 更新租户安全配置（upsert）
func (tm *TenantManager) UpdateConfig(cfg *TenantConfig) error {
	if cfg.TenantID == "" {
		return fmt.Errorf("tenant_id 不能为空")
	}
	if !tm.Exists(cfg.TenantID) {
		return fmt.Errorf("租户 %q 不存在", cfg.TenantID)
	}
	if cfg.AlertLevel == "" {
		cfg.AlertLevel = "high"
	}
	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err := tm.db.Exec(`INSERT INTO tenant_configs (tenant_id, disabled_rules, extra_rules_yaml, strict_mode, canary_enabled, budget_enabled, budget_max_tokens, budget_max_tools, tool_blacklist, alert_level, alert_webhook, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET
			disabled_rules=excluded.disabled_rules,
			extra_rules_yaml=excluded.extra_rules_yaml,
			strict_mode=excluded.strict_mode,
			canary_enabled=excluded.canary_enabled,
			budget_enabled=excluded.budget_enabled,
			budget_max_tokens=excluded.budget_max_tokens,
			budget_max_tools=excluded.budget_max_tools,
			tool_blacklist=excluded.tool_blacklist,
			alert_level=excluded.alert_level,
			alert_webhook=excluded.alert_webhook,
			updated_at=excluded.updated_at`,
		cfg.TenantID, cfg.DisabledRules, cfg.ExtraRulesYAML,
		boolToInt(cfg.StrictMode), boolToInt(cfg.CanaryEnabled), boolToInt(cfg.BudgetEnabled),
		cfg.BudgetMaxTokens, cfg.BudgetMaxTools, cfg.ToolBlacklist,
		cfg.AlertLevel, cfg.AlertWebhook, cfg.UpdatedAt)
	if err != nil {
		return fmt.Errorf("更新租户配置失败: %w", err)
	}
	log.Printf("[租户] 更新安全配置: %s", cfg.TenantID)
	return nil
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
