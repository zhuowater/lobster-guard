// layout.go — v17.1 Dashboard 布局持久化 + 预设模板
// 面板可折叠/拖拽布局 — "用户自定义大屏"
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// 数据结构
// ============================================================

// PanelConfig 面板配置
type PanelConfig struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Order     int    `json:"order"`
	Collapsed bool   `json:"collapsed"`
	Visible   bool   `json:"visible"`
	Width     string `json:"width"` // "full" / "half" / "third"
	Locked    bool   `json:"locked"`
	Removable bool   `json:"removable"`
}

// DashboardLayout 布局配置
type DashboardLayout struct {
	ID          string        `json:"id"`
	UserID      string        `json:"user_id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Panels      []PanelConfig `json:"panels"`
	IsDefault   bool          `json:"is_default"`
	IsPreset    bool          `json:"is_preset,omitempty"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

// ============================================================
// 预设模板
// ============================================================

// allPanels 全部可用面板定义
var allPanels = []PanelConfig{
	{ID: "health-score", Title: "安全健康分", Order: 0, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "realtime-stats", Title: "实时统计", Order: 1, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "owasp-matrix", Title: "OWASP 矩阵", Order: 2, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
	{ID: "attack-trend", Title: "攻击趋势图", Order: 3, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
	{ID: "audit-log", Title: "审计日志", Order: 4, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
	{ID: "attack-chain", Title: "攻击链摘要", Order: 5, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "honeypot-stats", Title: "蜜罐统计", Order: 6, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "redteam-results", Title: "红队结果", Order: 7, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "leaderboard", Title: "排行榜", Order: 8, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "ab-testing", Title: "A/B 测试", Order: 9, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
	{ID: "behavior-anomaly", Title: "行为异常", Order: 10, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
}

// presetLayouts 预设布局模板
var presetLayouts = []DashboardLayout{
	{
		ID:          "preset-soc",
		Name:        "SOC 运营模式",
		Description: "安全运营中心标准视图：健康分+OWASP+审计+攻击链",
		IsPreset:    true,
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "realtime-stats", Title: "实时统计", Order: 1, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "owasp-matrix", Title: "OWASP 矩阵", Order: 2, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "audit-log", Title: "审计日志", Order: 3, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "attack-chain", Title: "攻击链摘要", Order: 4, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "honeypot-stats", Title: "蜜罐统计", Order: 5, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "leaderboard", Title: "排行榜", Order: 6, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
		},
	},
	{
		ID:          "preset-executive",
		Name:        "管理层视图",
		Description: "高层汇报用：健康分+排行榜+SLA+趋势",
		IsPreset:    true,
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Collapsed: false, Visible: true, Width: "full", Locked: true, Removable: false},
			{ID: "leaderboard", Title: "排行榜", Order: 1, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "attack-trend", Title: "攻击趋势图", Order: 2, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "realtime-stats", Title: "实时统计", Order: 3, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "redteam-results", Title: "红队结果", Order: 4, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
		},
	},
	{
		ID:          "preset-redteam",
		Name:        "Red Team 视图",
		Description: "红队测试聚焦：红队结果+漏洞+A/B测试+攻击链",
		IsPreset:    true,
		Panels: []PanelConfig{
			{ID: "redteam-results", Title: "红队结果", Order: 0, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "attack-chain", Title: "攻击链摘要", Order: 1, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "honeypot-stats", Title: "蜜罐统计", Order: 2, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "ab-testing", Title: "A/B 测试", Order: 3, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
			{ID: "behavior-anomaly", Title: "行为异常", Order: 4, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
		},
	},
	{
		ID:          "preset-minimal",
		Name:        "极简模式",
		Description: "只看核心指标",
		IsPreset:    true,
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Collapsed: false, Visible: true, Width: "half", Locked: true, Removable: false},
			{ID: "realtime-stats", Title: "实时统计", Order: 1, Collapsed: false, Visible: true, Width: "half", Locked: false, Removable: true},
			{ID: "audit-log", Title: "审计日志", Order: 2, Collapsed: false, Visible: true, Width: "full", Locked: false, Removable: true},
		},
	},
}

// ============================================================
// LayoutStore 布局存储层
// ============================================================

// LayoutStore 布局持久化存储
type LayoutStore struct {
	db *sql.DB
}

// NewLayoutStore 创建布局存储
func NewLayoutStore(db *sql.DB) *LayoutStore {
	ls := &LayoutStore{db: db}
	ls.initTable()
	return ls
}

func (ls *LayoutStore) initTable() {
	_, err := ls.db.Exec(`
		CREATE TABLE IF NOT EXISTS dashboard_layouts (
			id TEXT PRIMARY KEY,
			user_id TEXT DEFAULT '',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			layout_json TEXT NOT NULL,
			is_default INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Printf("[布局] 创建 dashboard_layouts 表失败: %v", err)
	}
}

// List 获取布局列表
func (ls *LayoutStore) List(userID string) ([]DashboardLayout, error) {
	query := `SELECT id, user_id, name, description, layout_json, is_default, created_at, updated_at FROM dashboard_layouts`
	args := []interface{}{}
	if userID != "" {
		query += ` WHERE user_id = ? OR user_id = ''`
		args = append(args, userID)
	}
	query += ` ORDER BY is_default DESC, updated_at DESC`

	rows, err := ls.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layouts []DashboardLayout
	for rows.Next() {
		var l DashboardLayout
		var layoutJSON string
		var isDefault int
		if err := rows.Scan(&l.ID, &l.UserID, &l.Name, &l.Description, &layoutJSON, &isDefault, &l.CreatedAt, &l.UpdatedAt); err != nil {
			continue
		}
		l.IsDefault = isDefault == 1
		if err := json.Unmarshal([]byte(layoutJSON), &l.Panels); err != nil {
			l.Panels = []PanelConfig{}
		}
		layouts = append(layouts, l)
	}
	return layouts, nil
}

// Get 获取单个布局
func (ls *LayoutStore) Get(id string) (*DashboardLayout, error) {
	var l DashboardLayout
	var layoutJSON string
	var isDefault int
	err := ls.db.QueryRow(
		`SELECT id, user_id, name, description, layout_json, is_default, created_at, updated_at FROM dashboard_layouts WHERE id = ?`, id,
	).Scan(&l.ID, &l.UserID, &l.Name, &l.Description, &layoutJSON, &isDefault, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	l.IsDefault = isDefault == 1
	if err := json.Unmarshal([]byte(layoutJSON), &l.Panels); err != nil {
		l.Panels = []PanelConfig{}
	}
	return &l, nil
}

// Create 创建布局
func (ls *LayoutStore) Create(l *DashboardLayout) error {
	panelsJSON, err := json.Marshal(l.Panels)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if l.CreatedAt == "" {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	isDefault := 0
	if l.IsDefault {
		isDefault = 1
	}
	_, err = ls.db.Exec(
		`INSERT INTO dashboard_layouts (id, user_id, name, description, layout_json, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.UserID, l.Name, l.Description, string(panelsJSON), isDefault, l.CreatedAt, l.UpdatedAt,
	)
	return err
}

// Update 更新布局
func (ls *LayoutStore) Update(l *DashboardLayout) error {
	panelsJSON, err := json.Marshal(l.Panels)
	if err != nil {
		return err
	}
	l.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	isDefault := 0
	if l.IsDefault {
		isDefault = 1
	}
	result, err := ls.db.Exec(
		`UPDATE dashboard_layouts SET user_id=?, name=?, description=?, layout_json=?, is_default=?, updated_at=? WHERE id=?`,
		l.UserID, l.Name, l.Description, string(panelsJSON), isDefault, l.UpdatedAt, l.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("layout not found: %s", l.ID)
	}
	return nil
}

// Delete 删除布局
func (ls *LayoutStore) Delete(id string) error {
	result, err := ls.db.Exec(`DELETE FROM dashboard_layouts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("layout not found: %s", id)
	}
	return nil
}

// SetActive 设置活跃布局
func (ls *LayoutStore) SetActive(userID, layoutID string) error {
	// 先清除该用户的所有默认
	_, err := ls.db.Exec(`UPDATE dashboard_layouts SET is_default=0 WHERE user_id=? OR (user_id='' AND ?='')`, userID, userID)
	if err != nil {
		return err
	}
	// 设置新的默认
	result, err := ls.db.Exec(`UPDATE dashboard_layouts SET is_default=1 WHERE id=?`, layoutID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("layout not found: %s", layoutID)
	}
	return nil
}

// GetPresets 返回预设布局模板
func GetPresets() []DashboardLayout {
	now := time.Now().UTC().Format(time.RFC3339)
	presets := make([]DashboardLayout, len(presetLayouts))
	for i, p := range presetLayouts {
		presets[i] = p
		presets[i].CreatedAt = now
		presets[i].UpdatedAt = now
	}
	return presets
}

// GetAllPanels 返回所有可用面板定义
func GetAllPanels() []PanelConfig {
	panels := make([]PanelConfig, len(allPanels))
	copy(panels, allPanels)
	return panels
}

// ============================================================
// API Handlers
// ============================================================

// handleLayoutList GET /api/v1/layouts — 布局列表
func (api *ManagementAPI) handleLayoutList(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	userID := r.URL.Query().Get("user_id")
	layouts, err := api.layoutStore.List(userID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if layouts == nil {
		layouts = []DashboardLayout{}
	}
	jsonResponse(w, 200, map[string]interface{}{"layouts": layouts, "total": len(layouts)})
}

// handleLayoutCreate POST /api/v1/layouts — 保存布局
func (api *ManagementAPI) handleLayoutCreate(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	var layout DashboardLayout
	if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if layout.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "name is required"})
		return
	}
	if layout.ID == "" {
		layout.ID = fmt.Sprintf("layout-%d", time.Now().UnixNano()%1000000)
	}
	if layout.Panels == nil {
		layout.Panels = []PanelConfig{}
	}
	if err := api.layoutStore.Create(&layout); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[布局] 创建布局: %s (%s)", layout.ID, layout.Name)
	jsonResponse(w, 200, layout)
}

// handleLayoutGet GET /api/v1/layouts/:id — 获取布局
func (api *ManagementAPI) handleLayoutGet(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/layouts/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "layout id required"})
		return
	}
	layout, err := api.layoutStore.Get(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "layout not found"})
		return
	}
	jsonResponse(w, 200, layout)
}

// handleLayoutUpdate PUT /api/v1/layouts/:id — 更新布局
func (api *ManagementAPI) handleLayoutUpdate(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/layouts/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "layout id required"})
		return
	}
	var layout DashboardLayout
	if err := json.NewDecoder(r.Body).Decode(&layout); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	layout.ID = id
	if err := api.layoutStore.Update(&layout); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[布局] 更新布局: %s (%s)", layout.ID, layout.Name)
	jsonResponse(w, 200, layout)
}

// handleLayoutDelete DELETE /api/v1/layouts/:id — 删除布局
func (api *ManagementAPI) handleLayoutDelete(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/layouts/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "layout id required"})
		return
	}
	if err := api.layoutStore.Delete(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[布局] 删除布局: %s", id)
	jsonResponse(w, 200, map[string]string{"status": "deleted", "id": id})
}

// handleLayoutPresets GET /api/v1/layouts/presets — 预设模板列表
func (api *ManagementAPI) handleLayoutPresets(w http.ResponseWriter, r *http.Request) {
	presets := GetPresets()
	jsonResponse(w, 200, map[string]interface{}{
		"presets": presets,
		"total":   len(presets),
		"panels":  GetAllPanels(),
	})
}

// handleLayoutSetActive POST /api/v1/layouts/active — 设置当前活跃布局
func (api *ManagementAPI) handleLayoutSetActive(w http.ResponseWriter, r *http.Request) {
	if api.layoutStore == nil {
		jsonResponse(w, 500, map[string]string{"error": "layout store not initialized"})
		return
	}
	var req struct {
		LayoutID string `json:"layout_id"`
		UserID   string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LayoutID == "" {
		jsonResponse(w, 400, map[string]string{"error": "layout_id is required"})
		return
	}
	if err := api.layoutStore.SetActive(req.UserID, req.LayoutID); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[布局] 设置活跃布局: user=%s layout=%s", req.UserID, req.LayoutID)
	jsonResponse(w, 200, map[string]string{"status": "ok", "layout_id": req.LayoutID})
}
