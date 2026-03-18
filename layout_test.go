package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupLayoutTestDB 创建内存 SQLite 用于测试
func setupLayoutTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// setupLayoutTestAPI 创建一个 ManagementAPI 供 handler 测试
func setupLayoutTestAPI(t *testing.T) (*ManagementAPI, *sql.DB) {
	db := setupLayoutTestDB(t)
	ls := NewLayoutStore(db)
	api := &ManagementAPI{
		layoutStore: ls,
	}
	return api, db
}

// TestLayoutStoreCreate 测试创建布局
func TestLayoutStoreCreate(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	layout := &DashboardLayout{
		ID:   "test-1",
		Name: "测试布局",
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Visible: true, Width: "half"},
		},
	}
	if err := ls.Create(layout); err != nil {
		t.Fatalf("create: %v", err)
	}
	if layout.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
	if layout.UpdatedAt == "" {
		t.Error("expected UpdatedAt to be set")
	}
}

// TestLayoutStoreGet 测试获取布局
func TestLayoutStoreGet(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	ls.Create(&DashboardLayout{
		ID:   "test-get",
		Name: "获取测试",
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Visible: true, Width: "full"},
			{ID: "audit-log", Title: "审计日志", Order: 1, Visible: true, Width: "half"},
		},
	})

	got, err := ls.Get("test-get")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "获取测试" {
		t.Errorf("name = %q, want %q", got.Name, "获取测试")
	}
	if len(got.Panels) != 2 {
		t.Errorf("panels len = %d, want 2", len(got.Panels))
	}
	if got.Panels[0].ID != "health-score" {
		t.Errorf("panel[0].ID = %q, want health-score", got.Panels[0].ID)
	}
}

// TestLayoutStoreUpdate 测试更新布局
func TestLayoutStoreUpdate(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	ls.Create(&DashboardLayout{
		ID:   "test-update",
		Name: "原始名称",
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Visible: true, Width: "half"},
		},
	})

	err := ls.Update(&DashboardLayout{
		ID:   "test-update",
		Name: "更新后名称",
		Panels: []PanelConfig{
			{ID: "health-score", Title: "安全健康分", Order: 0, Visible: true, Width: "full"},
			{ID: "audit-log", Title: "审计日志", Order: 1, Visible: true, Width: "full"},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := ls.Get("test-update")
	if got.Name != "更新后名称" {
		t.Errorf("name = %q, want 更新后名称", got.Name)
	}
	if len(got.Panels) != 2 {
		t.Errorf("panels len = %d, want 2", len(got.Panels))
	}
}

// TestLayoutStoreDelete 测试删除布局
func TestLayoutStoreDelete(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	ls.Create(&DashboardLayout{ID: "test-delete", Name: "删除测试", Panels: []PanelConfig{}})

	if err := ls.Delete("test-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := ls.Get("test-delete")
	if err == nil {
		t.Error("expected error getting deleted layout")
	}
}

// TestLayoutStoreDeleteNotFound 测试删除不存在的布局
func TestLayoutStoreDeleteNotFound(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	err := ls.Delete("non-existent")
	if err == nil {
		t.Error("expected error deleting non-existent layout")
	}
}

// TestLayoutStoreList 测试列表查询
func TestLayoutStoreList(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	ls.Create(&DashboardLayout{ID: "list-1", Name: "布局1", UserID: "user-a", Panels: []PanelConfig{}})
	ls.Create(&DashboardLayout{ID: "list-2", Name: "布局2", UserID: "user-b", Panels: []PanelConfig{}})
	ls.Create(&DashboardLayout{ID: "list-3", Name: "全局布局", UserID: "", Panels: []PanelConfig{}})

	// 查全部
	all, err := ls.List("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("list all = %d, want 3", len(all))
	}

	// 按用户查（应返回该用户的 + 全局的）
	userA, err := ls.List("user-a")
	if err != nil {
		t.Fatalf("list user-a: %v", err)
	}
	if len(userA) != 2 {
		t.Errorf("list user-a = %d, want 2", len(userA))
	}
}

// TestLayoutStoreSetActive 测试设置活跃布局
func TestLayoutStoreSetActive(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	ls.Create(&DashboardLayout{ID: "active-1", Name: "布局1", Panels: []PanelConfig{}, IsDefault: true})
	ls.Create(&DashboardLayout{ID: "active-2", Name: "布局2", Panels: []PanelConfig{}})

	err := ls.SetActive("", "active-2")
	if err != nil {
		t.Fatalf("set active: %v", err)
	}

	got1, _ := ls.Get("active-1")
	got2, _ := ls.Get("active-2")
	if got1.IsDefault {
		t.Error("expected active-1 to not be default")
	}
	if !got2.IsDefault {
		t.Error("expected active-2 to be default")
	}
}

// TestPresetLayouts 测试预设模板
func TestPresetLayouts(t *testing.T) {
	presets := GetPresets()
	if len(presets) < 4 {
		t.Errorf("expected at least 4 presets, got %d", len(presets))
	}
	names := map[string]bool{}
	for _, p := range presets {
		if p.Name == "" {
			t.Error("preset has empty name")
		}
		if p.ID == "" {
			t.Error("preset has empty ID")
		}
		if len(p.Panels) == 0 {
			t.Errorf("preset %q has no panels", p.Name)
		}
		if !p.IsPreset {
			t.Errorf("preset %q should have IsPreset=true", p.Name)
		}
		names[p.Name] = true
	}
	expectedNames := []string{"SOC 运营模式", "管理层视图", "Red Team 视图", "极简模式"}
	for _, n := range expectedNames {
		if !names[n] {
			t.Errorf("missing preset: %s", n)
		}
	}
}

// TestAllPanels 测试面板列表完整性
func TestAllPanels(t *testing.T) {
	panels := GetAllPanels()
	if len(panels) < 10 {
		t.Errorf("expected at least 10 panels, got %d", len(panels))
	}
	ids := map[string]bool{}
	for _, p := range panels {
		if ids[p.ID] {
			t.Errorf("duplicate panel ID: %s", p.ID)
		}
		ids[p.ID] = true
	}
}

// TestLayoutAPIPresets 测试 presets API 端点
func TestLayoutAPIPresets(t *testing.T) {
	api, db := setupLayoutTestAPI(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/api/v1/layouts/presets", nil)
	w := httptest.NewRecorder()
	api.handleLayoutPresets(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	presets, ok := resp["presets"].([]interface{})
	if !ok {
		t.Fatal("expected presets array in response")
	}
	if len(presets) < 4 {
		t.Errorf("expected at least 4 presets, got %d", len(presets))
	}
	panels, ok := resp["panels"].([]interface{})
	if !ok {
		t.Fatal("expected panels array in response")
	}
	if len(panels) < 10 {
		t.Errorf("expected at least 10 panels, got %d", len(panels))
	}
}

// TestLayoutAPICRUD 测试完整 CRUD 流程
func TestLayoutAPICRUD(t *testing.T) {
	api, db := setupLayoutTestAPI(t)
	defer db.Close()

	// Create
	body := `{"id":"api-test","name":"API测试","panels":[{"id":"health-score","title":"安全健康分","order":0,"visible":true,"width":"half"}]}`
	req := httptest.NewRequest("POST", "/api/v1/layouts", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	api.handleLayoutCreate(w, req)
	if w.Code != 200 {
		t.Fatalf("create status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// Get
	req = httptest.NewRequest("GET", "/api/v1/layouts/api-test", nil)
	w = httptest.NewRecorder()
	api.handleLayoutGet(w, req)
	if w.Code != 200 {
		t.Fatalf("get status = %d", w.Code)
	}
	var layout DashboardLayout
	json.Unmarshal(w.Body.Bytes(), &layout)
	if layout.Name != "API测试" {
		t.Errorf("name = %q, want API测试", layout.Name)
	}

	// Update
	body = `{"name":"更新后","panels":[{"id":"health-score","title":"安全健康分","order":0,"visible":true,"width":"full"},{"id":"audit-log","title":"审计日志","order":1,"visible":true,"width":"full"}]}`
	req = httptest.NewRequest("PUT", "/api/v1/layouts/api-test", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	api.handleLayoutUpdate(w, req)
	if w.Code != 200 {
		t.Fatalf("update status = %d", w.Code)
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/layouts", nil)
	w = httptest.NewRecorder()
	api.handleLayoutList(w, req)
	if w.Code != 200 {
		t.Fatalf("list status = %d", w.Code)
	}
	var listResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	total := int(listResp["total"].(float64))
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/layouts/api-test", nil)
	w = httptest.NewRecorder()
	api.handleLayoutDelete(w, req)
	if w.Code != 200 {
		t.Fatalf("delete status = %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/api/v1/layouts/api-test", nil)
	w = httptest.NewRecorder()
	api.handleLayoutGet(w, req)
	if w.Code != 404 {
		t.Errorf("get deleted layout status = %d, want 404", w.Code)
	}
}

// TestLayoutAPISetActive 测试设置活跃布局 API
func TestLayoutAPISetActive(t *testing.T) {
	api, db := setupLayoutTestAPI(t)
	defer db.Close()

	// Create two layouts
	api.layoutStore.Create(&DashboardLayout{ID: "active-api-1", Name: "布局1", Panels: []PanelConfig{}})
	api.layoutStore.Create(&DashboardLayout{ID: "active-api-2", Name: "布局2", Panels: []PanelConfig{}})

	// Set active
	body := `{"layout_id":"active-api-2","user_id":""}`
	req := httptest.NewRequest("POST", "/api/v1/layouts/active", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	api.handleLayoutSetActive(w, req)
	if w.Code != 200 {
		t.Fatalf("set active status = %d, body: %s", w.Code, w.Body.String())
	}

	// Verify
	got, _ := api.layoutStore.Get("active-api-2")
	if !got.IsDefault {
		t.Error("expected active-api-2 to be default")
	}
}

// TestLayoutAPIIntegration 集成测试 — 通过 ServeHTTP 路由
func TestLayoutAPIIntegration(t *testing.T) {
	db := setupLayoutTestDB(t)
	defer db.Close()
	ls := NewLayoutStore(db)

	cfg := &Config{ManagementToken: "test-token"}
	// 创建 audit_log 表供 AuditLogger 使用
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT, direction TEXT, sender_id TEXT, action TEXT,
		reason TEXT, content_preview TEXT, full_request_hash TEXT,
		latency_ms REAL, upstream_id TEXT, app_id TEXT, trace_id TEXT,
		tenant_id TEXT DEFAULT ''
	)`)
	logger, err := NewAuditLogger(db)
	if err != nil {
		t.Fatalf("audit logger: %v", err)
	}
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	inboundEngine := NewRuleEngine()
	outboundEngine := &OutboundRuleEngine{}
	inbound := &InboundProxy{mode: "webhook"}
	shutdownMgr := NewShutdownManager(cfg)

	mapi := NewManagementAPI(cfg, "", pool, routes, logger, inboundEngine, outboundEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, shutdownMgr, nil)
	mapi.layoutStore = ls

	// Test: GET /api/v1/layouts/presets (with auth)
	req := httptest.NewRequest("GET", "/api/v1/layouts/presets", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mapi.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("integration presets status = %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	presets := resp["presets"].([]interface{})
	if len(presets) < 4 {
		t.Errorf("integration presets = %d, want >= 4", len(presets))
	}

	// Test: POST /api/v1/layouts (with auth)
	body := `{"id":"int-test","name":"集成测试","panels":[{"id":"health-score","title":"安全健康分","order":0,"visible":true,"width":"half"}]}`
	req = httptest.NewRequest("POST", "/api/v1/layouts", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mapi.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("integration create status = %d, body: %s", w.Code, w.Body.String())
	}

	// Test: GET /api/v1/layouts/int-test (with auth)
	req = httptest.NewRequest("GET", "/api/v1/layouts/int-test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mapi.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("integration get status = %d", w.Code)
	}

	// Test: DELETE /api/v1/layouts/int-test (with auth)
	req = httptest.NewRequest("DELETE", "/api/v1/layouts/int-test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mapi.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("integration delete status = %d", w.Code)
	}

	// Without auth should fail
	req = httptest.NewRequest("GET", "/api/v1/layouts/presets", nil)
	w = httptest.NewRecorder()
	mapi.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}

// TestLayoutCreateValidation 测试创建布局验证
func TestLayoutCreateValidation(t *testing.T) {
	api, db := setupLayoutTestAPI(t)
	defer db.Close()

	// Missing name
	body := `{"id":"bad","panels":[]}`
	req := httptest.NewRequest("POST", "/api/v1/layouts", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	api.handleLayoutCreate(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	// Invalid JSON
	req = httptest.NewRequest("POST", "/api/v1/layouts", bytes.NewBufferString("{invalid}"))
	w = httptest.NewRecorder()
	api.handleLayoutCreate(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// TestLayoutUpdateNotFound 测试更新不存在的布局
func TestLayoutUpdateNotFound(t *testing.T) {
	api, db := setupLayoutTestAPI(t)
	defer db.Close()

	body := `{"name":"不存在","panels":[]}`
	req := httptest.NewRequest("PUT", "/api/v1/layouts/non-existent", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	api.handleLayoutUpdate(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404 for non-existent layout, got %d", w.Code)
	}
}
