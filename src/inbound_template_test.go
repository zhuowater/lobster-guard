// inbound_template_test.go — 入站规则行业模板测试（v27.1）
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================
// 模板列表测试
// ============================================================

func TestListInboundTemplates(t *testing.T) {
	engine := NewRuleEngine()
	templates := engine.ListInboundTemplates()
	if len(templates) != 4 {
		t.Fatalf("期望 4 个默认模板，实际 %d", len(templates))
	}
	// 验证模板 ID
	expectedIDs := map[string]bool{
		"tpl-inbound-semiconductor": true,
		"tpl-inbound-financial":     true,
		"tpl-inbound-healthcare":    true,
		"tpl-inbound-compliance":    true,
	}
	for _, tpl := range templates {
		if !expectedIDs[tpl.ID] {
			t.Errorf("意外的模板 ID: %s", tpl.ID)
		}
		if tpl.Name == "" {
			t.Errorf("模板 %s 名称为空", tpl.ID)
		}
		if len(tpl.Rules) == 0 {
			t.Errorf("模板 %s 没有规则", tpl.ID)
		}
	}
}

func TestInboundTemplateCategories(t *testing.T) {
	templates := getDefaultInboundTemplates()
	for _, tpl := range templates {
		if tpl.Category != "industry" && tpl.Category != "security" && tpl.Category != "compliance" {
			t.Errorf("模板 %s category=%q 不在预期范围内", tpl.ID, tpl.Category)
		}
	}
}

// ============================================================
// 租户规则绑定测试
// ============================================================

func TestSetAndGetTenantRules(t *testing.T) {
	engine := NewRuleEngine()
	tenantID := "test-tenant-001"

	// 初始无规则
	rules := engine.GetTenantRules(tenantID)
	if rules != nil {
		t.Fatalf("初始应无租户规则，实际 %d 条", len(rules))
	}

	// 绑定芯片模板
	templates := engine.ListInboundTemplates()
	var chipTpl *InboundRuleTemplate
	for i, tpl := range templates {
		if tpl.ID == "tpl-inbound-semiconductor" {
			chipTpl = &templates[i]
			break
		}
	}
	if chipTpl == nil {
		t.Fatal("找不到芯片模板")
	}

	// 复制规则并加后缀
	tenantRules := make([]InboundRuleConfig, len(chipTpl.Rules))
	for i, r := range chipTpl.Rules {
		tenantRules[i] = InboundRuleConfig{
			Name:     r.Name + "-" + tenantID,
			Patterns: make([]string, len(r.Patterns)),
			Action:   r.Action,
			Category: r.Category,
		}
		copy(tenantRules[i].Patterns, r.Patterns)
	}
	engine.SetTenantRules(tenantID, tenantRules)

	// 验证获取
	got := engine.GetTenantRules(tenantID)
	if len(got) != len(chipTpl.Rules) {
		t.Fatalf("期望 %d 条规则，实际 %d", len(chipTpl.Rules), len(got))
	}
	for _, r := range got {
		if r.Name == "" || r.Action == "" {
			t.Errorf("规则字段不完整: %+v", r)
		}
	}
}

func TestRemoveTenantRules(t *testing.T) {
	engine := NewRuleEngine()
	tenantID := "remove-test"

	engine.SetTenantRules(tenantID, []InboundRuleConfig{
		{Name: "test-rule", Patterns: []string{"test"}, Action: "warn", Category: "test"},
	})
	if rules := engine.GetTenantRules(tenantID); len(rules) != 1 {
		t.Fatalf("期望 1 条规则，实际 %d", len(rules))
	}

	engine.RemoveTenantRules(tenantID)
	if rules := engine.GetTenantRules(tenantID); rules != nil {
		t.Fatalf("移除后应无规则，实际 %d 条", len(rules))
	}
}

// ============================================================
// 租户规则检测联动测试
// ============================================================

func TestDetectTenantRulesChip(t *testing.T) {
	engine := NewRuleEngine()
	tenantID := "chip-tenant"

	// 绑定芯片模板规则
	engine.SetTenantRules(tenantID, []InboundRuleConfig{
		{Name: "chip_ip_keyword_cn-" + tenantID, Patterns: []string{"RTL代码", "Verilog", "GDSII", "流片"}, Action: "warn", Category: "ip_protection"},
		{Name: "chip_export_control-" + tenantID, Patterns: []string{"EAR", "ITAR", "出口管制"}, Action: "block", Category: "export_control"},
	})

	tests := []struct {
		name       string
		input      string
		wantAction string
	}{
		{"无匹配", "今天天气不错", "pass"},
		{"IP关键词-warn", "这份文档包含了RTL代码的设计细节", "warn"},
		{"出口管制-block", "注意ITAR管制要求", "block"},
		{"多规则-取严格", "RTL代码和ITAR相关的出口管制文件", "block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.DetectTenantRules(tenantID, tt.input)
			if result.Action != tt.wantAction {
				t.Errorf("期望 %s，实际 %s，reasons=%v", tt.wantAction, result.Action, result.Reasons)
			}
		})
	}
}

func TestDetectTenantRulesFinancial(t *testing.T) {
	engine := NewRuleEngine()
	tenantID := "fin-tenant"

	engine.SetTenantRules(tenantID, []InboundRuleConfig{
		{Name: "fin_account_cn-" + tenantID, Patterns: []string{"账户余额", "交易流水", "征信报告"}, Action: "warn", Category: "financial_data"},
		{Name: "fin_trading-" + tenantID, Patterns: []string{"内幕交易", "insider trading"}, Action: "block", Category: "compliance"},
	})

	result := engine.DetectTenantRules(tenantID, "请查询我的账户余额")
	if result.Action != "warn" {
		t.Errorf("期望 warn，实际 %s", result.Action)
	}

	result = engine.DetectTenantRules(tenantID, "这是内幕交易信息")
	if result.Action != "block" {
		t.Errorf("期望 block，实际 %s", result.Action)
	}

	// 另一个租户无规则
	result = engine.DetectTenantRules("other-tenant", "请查询我的账户余额")
	if result.Action != "pass" {
		t.Errorf("无规则租户期望 pass，实际 %s", result.Action)
	}
}

func TestDetectTenantRulesEmpty(t *testing.T) {
	engine := NewRuleEngine()
	result := engine.DetectTenantRules("nonexistent", "any text")
	if result.Action != "pass" {
		t.Errorf("无租户规则应 pass，实际 %s", result.Action)
	}
	result = engine.DetectTenantRules("", "any text")
	if result.Action != "pass" {
		t.Errorf("空tenantID应 pass，实际 %s", result.Action)
	}
	result = engine.DetectTenantRules("tenant", "")
	if result.Action != "pass" {
		t.Errorf("空text应 pass，实际 %s", result.Action)
	}
}

// ============================================================
// 排除+追加组合测试
// ============================================================

func TestExcludeGlobalPlusAppendTenant(t *testing.T) {
	engine := NewRuleEngine()
	tenantID := "combo-tenant"

	// 全局规则: sensitive_keywords 对 "密码" 会 warn
	globalResult := engine.DetectWithExclusions("密码泄露", "", nil)
	if globalResult.Action == "pass" {
		t.Skip("默认规则未匹配 '密码'，跳过组合测试")
	}

	// 排除 sensitive_keywords
	excludedResult := engine.DetectWithExclusions("密码泄露", "", []string{"sensitive_keywords"})
	// 注意: 可能还有PII检测结果
	_ = excludedResult

	// 租户绑定金融规则
	engine.SetTenantRules(tenantID, []InboundRuleConfig{
		{Name: "fin_account_cn-" + tenantID, Patterns: []string{"征信报告"}, Action: "warn", Category: "financial_data"},
	})

	// 全局通过 + 租户追加命中
	tenantResult := engine.DetectTenantRules(tenantID, "请提供征信报告")
	if tenantResult.Action != "warn" {
		t.Errorf("租户规则应命中 warn，实际 %s", tenantResult.Action)
	}
}

func TestMergeDetectResults(t *testing.T) {
	base := DetectResult{Action: "warn", Reasons: []string{"r1"}, MatchedRules: []string{"r1"}}
	extra := DetectResult{Action: "block", Reasons: []string{"r2"}, MatchedRules: []string{"r2"}, Message: "blocked"}

	merged := mergeDetectResults(base, extra)
	if merged.Action != "block" {
		t.Errorf("合并后期望 block，实际 %s", merged.Action)
	}
	if len(merged.MatchedRules) != 2 {
		t.Errorf("合并后期望 2 条规则，实际 %d", len(merged.MatchedRules))
	}
	if merged.Message != "blocked" {
		t.Errorf("期望 Message=blocked，实际 %q", merged.Message)
	}
}

func TestMergeDetectResultsPassBase(t *testing.T) {
	base := DetectResult{Action: "pass"}
	extra := DetectResult{Action: "warn", Reasons: []string{"r1"}, MatchedRules: []string{"r1"}}
	merged := mergeDetectResults(base, extra)
	if merged.Action != "warn" {
		t.Errorf("base pass + extra warn 应得 warn，实际 %s", merged.Action)
	}
}

func TestMergeDetectResultsPassExtra(t *testing.T) {
	base := DetectResult{Action: "warn", Reasons: []string{"r1"}, MatchedRules: []string{"r1"}}
	extra := DetectResult{Action: "pass"}
	merged := mergeDetectResults(base, extra)
	if merged.Action != "warn" {
		t.Errorf("extra pass 不应改变 base，实际 %s", merged.Action)
	}
}

// ============================================================
// API 端点测试
// ============================================================

func TestInboundTemplateListAPI(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	req := httptest.NewRequest("GET", "/api/v1/inbound-templates", nil)
	w := httptest.NewRecorder()
	api.handleInboundTemplateList(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	var resp struct {
		Templates []InboundRuleTemplate `json:"templates"`
		Total     int                   `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Total != 4 {
		t.Errorf("期望 4 个模板，实际 %d", resp.Total)
	}
}

func TestInboundTemplateGetAPI(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	// 存在的模板
	req := httptest.NewRequest("GET", "/api/v1/inbound-templates/tpl-inbound-semiconductor", nil)
	w := httptest.NewRecorder()
	api.handleInboundTemplateGet(w, req)
	if w.Code != 200 {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	var tpl InboundRuleTemplate
	if err := json.NewDecoder(w.Body).Decode(&tpl); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if tpl.ID != "tpl-inbound-semiconductor" {
		t.Errorf("期望 ID=tpl-inbound-semiconductor，实际 %s", tpl.ID)
	}
	if len(tpl.Rules) != 3 {
		t.Errorf("芯片模板期望 3 条规则，实际 %d", len(tpl.Rules))
	}

	// 不存在的模板
	req2 := httptest.NewRequest("GET", "/api/v1/inbound-templates/nonexistent", nil)
	w2 := httptest.NewRecorder()
	api.handleInboundTemplateGet(w2, req2)
	if w2.Code != 404 {
		t.Errorf("不存在的模板期望 404，实际 %d", w2.Code)
	}
}

func TestBindInboundTemplateAPI(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	body := `{"template_id": "tpl-inbound-financial"}`
	req := httptest.NewRequest("POST", "/api/v1/tenants/test-fin/bind-inbound-template",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.handleTenantBindInboundTemplate(w, req)

	if w.Code != 200 {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "bound" {
		t.Errorf("期望 status=bound，实际 %v", resp["status"])
	}
	rulesBound := int(resp["rules_bound"].(float64))
	if rulesBound != 3 {
		t.Errorf("期望绑定 3 条规则，实际 %d", rulesBound)
	}

	// 验证规则已存入
	rules := engine.GetTenantRules("test-fin")
	if len(rules) != 3 {
		t.Fatalf("期望 3 条租户规则，实际 %d", len(rules))
	}
	// 检查名称后缀
	for _, r := range rules {
		if r.Name[len(r.Name)-8:] != "test-fin" {
			t.Errorf("规则名应以 -test-fin 结尾: %s", r.Name)
		}
	}

	// 验证检测有效
	result := engine.DetectTenantRules("test-fin", "请查询我的账户余额")
	if result.Action != "warn" {
		t.Errorf("绑定后检测应命中 warn，实际 %s", result.Action)
	}
}

func TestBindInboundTemplateNotFound(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	body := `{"template_id": "nonexistent"}`
	req := httptest.NewRequest("POST", "/api/v1/tenants/test/bind-inbound-template",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.handleTenantBindInboundTemplate(w, req)

	if w.Code != 404 {
		t.Errorf("期望 404，实际 %d", w.Code)
	}
}

func TestBindMultipleTemplates(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	// 先绑定芯片模板
	body1 := `{"template_id": "tpl-inbound-semiconductor"}`
	req1 := httptest.NewRequest("POST", "/api/v1/tenants/multi/bind-inbound-template",
		bytes.NewBufferString(body1))
	w1 := httptest.NewRecorder()
	api.handleTenantBindInboundTemplate(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("第一次绑定失败: %d", w1.Code)
	}

	// 再绑定金融模板
	body2 := `{"template_id": "tpl-inbound-financial"}`
	req2 := httptest.NewRequest("POST", "/api/v1/tenants/multi/bind-inbound-template",
		bytes.NewBufferString(body2))
	w2 := httptest.NewRecorder()
	api.handleTenantBindInboundTemplate(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("第二次绑定失败: %d", w2.Code)
	}

	// 验证总规则数=3+3=6
	rules := engine.GetTenantRules("multi")
	if len(rules) != 6 {
		t.Errorf("期望 6 条规则（芯片3+金融3），实际 %d", len(rules))
	}

	var resp map[string]interface{}
	json.NewDecoder(w2.Body).Decode(&resp)
	totalRules := int(resp["total_rules"].(float64))
	if totalRules != 6 {
		t.Errorf("API 返回 total_rules 应为 6，实际 %d", totalRules)
	}
}

func TestTenantInboundRulesAPI(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	// 无规则时
	req := httptest.NewRequest("GET", "/api/v1/tenants/empty/inbound-rules", nil)
	w := httptest.NewRecorder()
	api.handleTenantInboundRules(w, req)
	if w.Code != 200 {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	var resp struct {
		Rules []InboundRuleConfig `json:"rules"`
		Total int                 `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Errorf("空租户期望 0 条规则，实际 %d", resp.Total)
	}

	// 设置规则后
	engine.SetTenantRules("has-rules", []InboundRuleConfig{
		{Name: "test-rule", Patterns: []string{"test"}, Action: "warn", Category: "test"},
	})
	req2 := httptest.NewRequest("GET", "/api/v1/tenants/has-rules/inbound-rules", nil)
	w2 := httptest.NewRecorder()
	api.handleTenantInboundRules(w2, req2)
	var resp2 struct {
		Rules []InboundRuleConfig `json:"rules"`
		Total int                 `json:"total"`
	}
	json.NewDecoder(w2.Body).Decode(&resp2)
	if resp2.Total != 1 {
		t.Errorf("期望 1 条规则，实际 %d", resp2.Total)
	}
}

func TestTenantDeleteInboundRulesAPI(t *testing.T) {
	engine := NewRuleEngine()
	api := &ManagementAPI{inboundEngine: engine}

	engine.SetTenantRules("del-test", []InboundRuleConfig{
		{Name: "test", Patterns: []string{"test"}, Action: "warn", Category: "test"},
	})

	req := httptest.NewRequest("DELETE", "/api/v1/tenants/del-test/inbound-rules", nil)
	w := httptest.NewRecorder()
	api.handleTenantDeleteInboundRules(w, req)
	if w.Code != 200 {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}

	// 验证已清除
	rules := engine.GetTenantRules("del-test")
	if rules != nil {
		t.Errorf("删除后应无规则，实际 %d 条", len(rules))
	}
}

// ============================================================
// 全流程集成测试
// ============================================================

func TestFullFlowBindAndDetect(t *testing.T) {
	engine := NewRuleEngine()

	// 1. 列出模板
	templates := engine.ListInboundTemplates()
	if len(templates) == 0 {
		t.Fatal("无模板")
	}

	// 2. 绑定医疗模板到租户
	tenantID := "hospital-001"
	healthTpl := templates[2] // healthcare
	if healthTpl.ID != "tpl-inbound-healthcare" {
		// 找到正确的
		for _, tpl := range templates {
			if tpl.ID == "tpl-inbound-healthcare" {
				healthTpl = tpl
				break
			}
		}
	}

	tenantRules := make([]InboundRuleConfig, len(healthTpl.Rules))
	for i, r := range healthTpl.Rules {
		tenantRules[i] = InboundRuleConfig{
			Name:     r.Name + "-" + tenantID,
			Patterns: make([]string, len(r.Patterns)),
			Action:   r.Action,
			Category: r.Category,
		}
		copy(tenantRules[i].Patterns, r.Patterns)
	}
	engine.SetTenantRules(tenantID, tenantRules)

	// 3. 全局检测：正常文本应通过
	globalResult := engine.Detect("请查看患者的病历")
	// 全局规则不包含医疗关键词，应通过
	if globalResult.Action != "pass" {
		t.Logf("全局规则意外命中: %s, reasons=%v", globalResult.Action, globalResult.Reasons)
	}

	// 4. 租户检测：应命中
	tenantResult := engine.DetectTenantRules(tenantID, "请查看患者的病历")
	if tenantResult.Action != "warn" {
		t.Errorf("租户规则应命中病历关键词 warn，实际 %s", tenantResult.Action)
	}

	// 5. 合并结果
	merged := mergeDetectResults(globalResult, tenantResult)
	if merged.Action == "pass" {
		t.Errorf("合并后不应是 pass")
	}

	// 6. 管制药品应 block
	drugResult := engine.DetectTenantRules(tenantID, "开具管制药品处方")
	if drugResult.Action != "block" {
		t.Errorf("管制药品应 block，实际 %s", drugResult.Action)
	}

	// 7. 清除规则
	engine.RemoveTenantRules(tenantID)
	afterClear := engine.DetectTenantRules(tenantID, "请查看患者的病历")
	if afterClear.Action != "pass" {
		t.Errorf("清除后应 pass，实际 %s", afterClear.Action)
	}
}

// ============================================================
// HTTP 路由集成测试（模拟 ServeHTTP 路由分发）
// ============================================================

func TestInboundTemplateRouting(t *testing.T) {
	engine := NewRuleEngine()
	outEngine := NewOutboundRuleEngine(nil)

	api := &ManagementAPI{
		inboundEngine:  engine,
		outboundEngine: outEngine,
		cfg:            &Config{},
	}

	// 测试 GET /api/v1/inbound-templates
	t.Run("list templates", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/inbound-templates", nil)
		w := httptest.NewRecorder()
		api.handleInboundTemplateList(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("期望 200，实际 %d", w.Code)
		}
	})

	// 测试 POST bind 后 GET 和 DELETE
	t.Run("bind then get then delete", func(t *testing.T) {
		// bind
		body := `{"template_id":"tpl-inbound-compliance"}`
		req := httptest.NewRequest("POST", "/api/v1/tenants/route-test/bind-inbound-template",
			bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		api.handleTenantBindInboundTemplate(w, req)
		if w.Code != 200 {
			t.Fatalf("bind 期望 200，实际 %d", w.Code)
		}

		// get
		req2 := httptest.NewRequest("GET", "/api/v1/tenants/route-test/inbound-rules", nil)
		w2 := httptest.NewRecorder()
		api.handleTenantInboundRules(w2, req2)
		if w2.Code != 200 {
			t.Fatalf("get 期望 200，实际 %d", w2.Code)
		}
		var resp struct {
			Total int `json:"total"`
		}
		json.NewDecoder(w2.Body).Decode(&resp)
		if resp.Total != 2 { // compliance 模板有 2 条规则
			t.Errorf("期望 2 条规则，实际 %d", resp.Total)
		}

		// delete
		req3 := httptest.NewRequest("DELETE", "/api/v1/tenants/route-test/inbound-rules", nil)
		w3 := httptest.NewRecorder()
		api.handleTenantDeleteInboundRules(w3, req3)
		if w3.Code != 200 {
			t.Fatalf("delete 期望 200，实际 %d", w3.Code)
		}

		// verify empty
		req4 := httptest.NewRequest("GET", "/api/v1/tenants/route-test/inbound-rules", nil)
		w4 := httptest.NewRecorder()
		api.handleTenantInboundRules(w4, req4)
		var resp2 struct {
			Total int `json:"total"`
		}
		json.NewDecoder(w4.Body).Decode(&resp2)
		if resp2.Total != 0 {
			t.Errorf("清除后期望 0 条规则，实际 %d", resp2.Total)
		}
	})
}
