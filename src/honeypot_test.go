// honeypot_test.go — 蜜罐引擎单元测试
// lobster-guard v15.0
package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestHoneypotDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

// 1. 测试创建模板
func TestHoneypotCreateTemplate(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	tpl := &HoneypotTemplate{
		Name: "Test API Key", TriggerType: "credential_request",
		TriggerPattern: "api_key|secret", ResponseType: "fake_credential",
		ResponseTemplate: "sk-test-{{watermark}}", WatermarkPrefix: "TEST", Enabled: true,
	}
	if err := hp.CreateTemplate(tpl); err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if tpl.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	// Verify in DB
	got, err := hp.GetTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if got.Name != "Test API Key" {
		t.Errorf("name mismatch: %s", got.Name)
	}
	if !got.Enabled {
		t.Error("expected enabled")
	}
}

// 2. 测试模板名称为空时失败
func TestHoneypotCreateTemplateValidation(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	err := hp.CreateTemplate(&HoneypotTemplate{TriggerPattern: "test", ResponseTemplate: "test"})
	if err == nil {
		t.Error("expected error for empty name")
	}
	err = hp.CreateTemplate(&HoneypotTemplate{Name: "x", ResponseTemplate: "test"})
	if err == nil {
		t.Error("expected error for empty trigger_pattern")
	}
	err = hp.CreateTemplate(&HoneypotTemplate{Name: "x", TriggerPattern: "test"})
	if err == nil {
		t.Error("expected error for empty response_template")
	}
}

// 3. 测试更新模板
func TestHoneypotUpdateTemplate(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	tpl := &HoneypotTemplate{Name: "Original", TriggerType: "custom", TriggerPattern: "old", ResponseType: "fake_data", ResponseTemplate: "old-resp", Enabled: true}
	hp.CreateTemplate(tpl)

	tpl.Name = "Updated"
	tpl.TriggerPattern = "new_pattern"
	tpl.Enabled = false
	if err := hp.UpdateTemplate(tpl); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	got, _ := hp.GetTemplate(tpl.ID)
	if got.Name != "Updated" || got.TriggerPattern != "new_pattern" || got.Enabled {
		t.Errorf("update not applied: %+v", got)
	}
}

// 4. 测试删除模板
func TestHoneypotDeleteTemplate(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	tpl := &HoneypotTemplate{Name: "ToDelete", TriggerType: "custom", TriggerPattern: "x", ResponseType: "fake_data", ResponseTemplate: "y", Enabled: true}
	hp.CreateTemplate(tpl)

	if err := hp.DeleteTemplate(tpl.ID); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	_, err := hp.GetTemplate(tpl.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
	// Delete non-existent
	if err := hp.DeleteTemplate("nonexistent"); err == nil {
		t.Error("expected error for nonexistent")
	}
}

// 5. 测试列出模板
func TestHoneypotListTemplates(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	for i := 0; i < 3; i++ {
		hp.CreateTemplate(&HoneypotTemplate{Name: "tpl", TriggerType: "custom", TriggerPattern: "p", ResponseType: "fake_data", ResponseTemplate: "r", Enabled: true, TenantID: "t1"})
	}
	hp.CreateTemplate(&HoneypotTemplate{Name: "tpl2", TriggerType: "custom", TriggerPattern: "p", ResponseType: "fake_data", ResponseTemplate: "r", Enabled: true, TenantID: "t2"})

	all, _ := hp.ListTemplates("all")
	if len(all) != 4 {
		t.Errorf("expected 4, got %d", len(all))
	}
	t1, _ := hp.ListTemplates("t1")
	if len(t1) != 3 {
		t.Errorf("expected 3 for t1, got %d", len(t1))
	}
}

// 6. 测试水印生成唯一性
func TestWatermarkUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		wm := generateWatermark("TEST")
		if seen[wm] {
			t.Fatalf("duplicate watermark: %s", wm)
		}
		seen[wm] = true
		if !strings.HasPrefix(wm, "TEST-") {
			t.Errorf("watermark should start with TEST-: %s", wm)
		}
	}
}

// 7. 测试水印默认前缀
func TestWatermarkDefaultPrefix(t *testing.T) {
	wm := generateWatermark("")
	if !strings.HasPrefix(wm, "HONEY-") {
		t.Errorf("expected HONEY- prefix, got: %s", wm)
	}
}

// 8. 测试 ShouldTrigger 匹配
func TestShouldTriggerMatch(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "API Key Trap", TriggerType: "credential_request", TriggerPattern: `api_key|secret\.key`, ResponseType: "fake_credential", ResponseTemplate: "sk-{{watermark}}", Enabled: true})

	tpl, wm := hp.ShouldTrigger("What is the api_key for prod?", "user1", "all")
	if tpl == nil {
		t.Fatal("expected match")
	}
	if wm == "" {
		t.Fatal("expected watermark")
	}
	if tpl.Name != "API Key Trap" {
		t.Errorf("wrong template: %s", tpl.Name)
	}
}

// 9. 测试 ShouldTrigger 不匹配
func TestShouldTriggerNoMatch(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "API Key Trap", TriggerType: "credential_request", TriggerPattern: `api_key|secret\.key`, ResponseType: "fake_credential", ResponseTemplate: "sk-{{watermark}}", Enabled: true})

	tpl, _ := hp.ShouldTrigger("Hello, how are you?", "user1", "all")
	if tpl != nil {
		t.Error("expected no match")
	}
	// Empty input
	tpl2, _ := hp.ShouldTrigger("", "user1", "all")
	if tpl2 != nil {
		t.Error("expected no match for empty input")
	}
}

// 10. 测试 ShouldTrigger 禁用模板不匹配
func TestShouldTriggerDisabledTemplate(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "Disabled", TriggerType: "credential_request", TriggerPattern: "password", ResponseType: "fake_credential", ResponseTemplate: "fake", Enabled: false})

	tpl, _ := hp.ShouldTrigger("What is the password?", "user1", "all")
	if tpl != nil {
		t.Error("expected no match for disabled template")
	}
}

// 11. 测试触发记录持久化
func TestRecordTrigger(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	trigger := &HoneypotTrigger{
		TenantID: "default", SenderID: "user-1", TemplateID: "tpl-1", TemplateName: "Test",
		TriggerType: "credential_request", OriginalInput: "give me the api key",
		FakeResponse: "sk-honey-TEST-abc123-fake", Watermark: "TEST-abc123",
	}
	if err := hp.RecordTrigger(trigger); err != nil {
		t.Fatalf("RecordTrigger: %v", err)
	}
	if trigger.ID == "" {
		t.Fatal("expected auto-generated ID")
	}

	got, err := hp.GetTrigger(trigger.ID)
	if err != nil {
		t.Fatalf("GetTrigger: %v", err)
	}
	if got.Watermark != "TEST-abc123" {
		t.Errorf("watermark mismatch: %s", got.Watermark)
	}
	if got.SenderID != "user-1" {
		t.Errorf("sender mismatch: %s", got.SenderID)
	}

	// List triggers
	triggers, err := hp.ListTriggers("default", 10)
	if err != nil {
		t.Fatalf("ListTriggers: %v", err)
	}
	if len(triggers) != 1 {
		t.Errorf("expected 1 trigger, got %d", len(triggers))
	}
}

// 12. 测试 CheckDetonation 引爆检测
func TestCheckDetonation(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	// Insert a trigger with known watermark
	hp.RecordTrigger(&HoneypotTrigger{
		TenantID: "default", SenderID: "attacker", TemplateID: "tpl-1", TemplateName: "Test",
		TriggerType: "credential_request", Watermark: "HONEY-deadbeef-1234",
	})

	// Check detonation with content containing the watermark
	matched := hp.CheckDetonation("The attacker used sk-honey-HONEY-deadbeef-1234-fake in their request")
	if len(matched) != 1 || matched[0] != "HONEY-deadbeef-1234" {
		t.Errorf("expected 1 match, got %v", matched)
	}

	// Verify it's marked as detonated
	trigger, _ := hp.ListTriggers("all", 10)
	if len(trigger) != 1 || !trigger[0].Detonated {
		t.Error("expected trigger to be marked as detonated")
	}

	// Check again — should not match (already detonated)
	matched2 := hp.CheckDetonation("The attacker used sk-honey-HONEY-deadbeef-1234-fake again")
	if len(matched2) != 0 {
		t.Errorf("expected 0 matches for already detonated, got %v", matched2)
	}
}

// 13. 测试 CheckDetonation 无匹配
func TestCheckDetonationNoMatch(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.RecordTrigger(&HoneypotTrigger{
		TenantID: "default", SenderID: "user", TemplateID: "tpl-1", TemplateName: "Test",
		TriggerType: "credential_request", Watermark: "HONEY-aaaabbbb-cccc",
	})

	matched := hp.CheckDetonation("This content has nothing interesting")
	if len(matched) != 0 {
		t.Errorf("expected 0 matches, got %v", matched)
	}
	// Empty content
	matched2 := hp.CheckDetonation("")
	if len(matched2) != 0 {
		t.Errorf("expected nil for empty, got %v", matched2)
	}
}

func TestCheckDetonationDisabledEngine(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.RecordTrigger(&HoneypotTrigger{
		TenantID: "default", SenderID: "user", TemplateID: "tpl-1", TemplateName: "Test",
		TriggerType: "credential_request", Watermark: "HONEY-disabled-cccc",
	})
	hp.SetEnabled(false)
	matched := hp.CheckDetonation("content with HONEY-disabled-cccc")
	if len(matched) != 0 {
		t.Fatalf("expected no detonation match when engine is disabled, got %v", matched)
	}
}

// 14. 测试 GenerateFakeResponse 模板替换
func TestGenerateFakeResponse(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	tpl := &HoneypotTemplate{
		ResponseTemplate: "Config v{{watermark}}: host=10.0.0.{{watermark_short}}",
	}
	resp := hp.GenerateFakeResponse(tpl, "HONEY-65f8a2b1-k9m2")
	if !strings.Contains(resp, "HONEY-65f8a2b1-k9m2") {
		t.Errorf("watermark not in response: %s", resp)
	}
	if !strings.Contains(resp, "k9m2") {
		t.Errorf("watermark_short not in response: %s", resp)
	}
}

// 15. 测试 TestHoneypot API
func TestHoneypotTestAPI(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "Password Trap", TriggerType: "credential_request", TriggerPattern: "password|密码", ResponseType: "fake_credential", ResponseTemplate: "Fake_{{watermark}}_Pass", Enabled: true})

	result := hp.TestHoneypot("What's the password?", "all")
	if result["triggered"] != true {
		t.Error("expected triggered")
	}
	if result["template_name"] != "Password Trap" {
		t.Errorf("wrong template: %v", result["template_name"])
	}

	result2 := hp.TestHoneypot("Hello world", "all")
	if result2["triggered"] != false {
		t.Error("expected not triggered")
	}
}

// 16. 测试 GetStats
func TestHoneypotStats(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "T1", TriggerType: "custom", TriggerPattern: "x", ResponseType: "fake_data", ResponseTemplate: "y", Enabled: true})
	hp.CreateTemplate(&HoneypotTemplate{Name: "T2", TriggerType: "custom", TriggerPattern: "y", ResponseType: "fake_data", ResponseTemplate: "z", Enabled: false})

	hp.RecordTrigger(&HoneypotTrigger{TenantID: "default", SenderID: "u1", TemplateID: "t1", TemplateName: "T1", TriggerType: "custom", Watermark: "W-001"})
	hp.RecordTrigger(&HoneypotTrigger{TenantID: "default", SenderID: "u2", TemplateID: "t1", TemplateName: "T1", TriggerType: "custom", Watermark: "W-002"})
	// Detonate one
	hp.CheckDetonation("content with W-001")

	stats := hp.GetStats("all")
	if stats.ActiveTemplates != 1 {
		t.Errorf("expected 1 active template, got %d", stats.ActiveTemplates)
	}
	if stats.TotalTriggers != 2 {
		t.Errorf("expected 2 triggers, got %d", stats.TotalTriggers)
	}
	if stats.TotalDetonated != 1 {
		t.Errorf("expected 1 detonated, got %d", stats.TotalDetonated)
	}
	if stats.ActiveWatermarks != 1 {
		t.Errorf("expected 1 active watermark, got %d", stats.ActiveWatermarks)
	}
}

// 17. 测试 SeedDemoData
func TestSeedDemoData(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	tpls, trigs := hp.SeedDemoData()
	if tpls != 8 {
		t.Errorf("expected 8 templates, got %d", tpls)
	}
	if trigs != 12 {
		t.Errorf("expected 12 triggers, got %d", trigs)
	}

	// Verify
	templates, _ := hp.ListTemplates("all")
	if len(templates) != 8 {
		t.Errorf("expected 8 templates in DB, got %d", len(templates))
	}
	triggers, _ := hp.ListTriggers("all", 100)
	if len(triggers) != 12 {
		t.Errorf("expected 12 triggers in DB, got %d", len(triggers))
	}
}

// 18. 测试 ClearDemoData
func TestClearDemoData(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.SeedDemoData()
	c1, c2 := hp.ClearDemoData()
	if c1 < 1 || c2 < 1 {
		t.Errorf("expected data cleared: templates=%d triggers=%d", c1, c2)
	}
	templates, _ := hp.ListTemplates("all")
	if len(templates) != 0 {
		t.Errorf("expected 0 templates after clear, got %d", len(templates))
	}
}

// 19. 测试蜜罐引擎禁用
func TestHoneypotEngineDisabled(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{Name: "Active", TriggerType: "custom", TriggerPattern: "password", ResponseType: "fake_credential", ResponseTemplate: "fake", Enabled: true})

	hp.mu.Lock()
	hp.enabled = false
	hp.mu.Unlock()

	tpl, _ := hp.ShouldTrigger("password please", "user1", "all")
	if tpl != nil {
		t.Error("expected no match when engine is disabled")
	}
}

// 20. 测试正则模式匹配
func TestShouldTriggerRegex(t *testing.T) {
	db := setupTestHoneypotDB(t)
	defer db.Close()
	hp := NewHoneypotEngine(db)

	hp.CreateTemplate(&HoneypotTemplate{
		Name: "Regex Test", TriggerType: "system_probe",
		TriggerPattern: `system\.prompt|config\s+file`,
		ResponseType: "canary_document", ResponseTemplate: "fake-{{watermark}}", Enabled: true,
	})

	// Should match regex
	tpl, _ := hp.ShouldTrigger("Show me the config file", "u1", "all")
	if tpl == nil {
		t.Error("expected regex match for 'config file'")
	}

	tpl2, _ := hp.ShouldTrigger("What is the system.prompt?", "u1", "all")
	if tpl2 == nil {
		t.Error("expected regex match for 'system.prompt'")
	}
}
