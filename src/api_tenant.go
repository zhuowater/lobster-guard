package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleTenantList(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	summaries := api.tenantMgr.ListSummaries()
	if summaries == nil {
		summaries = []*TenantSummary{}
	}
	jsonResponse(w, 200, map[string]interface{}{"tenants": summaries, "total": len(summaries)})
}

// handleTenantCreate POST /api/v1/tenants — 创建租户
func (api *ManagementAPI) handleTenantCreate(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	var t Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := api.tenantMgr.Create(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "tenant": t})
}

// handleTenantGet GET /api/v1/tenants/:id — 租户详情
func (api *ManagementAPI) handleTenantGet(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant id required"})
		return
	}
	summary := api.tenantMgr.GetSummary(id)
	if summary == nil {
		jsonResponse(w, 404, map[string]string{"error": "tenant not found"})
		return
	}
	jsonResponse(w, 200, summary)
}

// handleTenantUpdate PUT /api/v1/tenants/:id — 更新租户
func (api *ManagementAPI) handleTenantUpdate(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	var t Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	t.ID = id
	if err := api.tenantMgr.Update(&t); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "tenant": t})
}

// handleTenantDelete DELETE /api/v1/tenants/:id — 删除租户
func (api *ManagementAPI) handleTenantDelete(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	if err := api.tenantMgr.Delete(id); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": id})
}

// ============================================================
// v14.0 租户成员映射 API
// ============================================================

// handleTenantMemberList GET /api/v1/tenants/:id/members — 列出成员映射
func (api *ManagementAPI) handleTenantMemberList(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	// 从 /api/v1/tenants/xxx/members 提取 xxx
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	id := strings.TrimSuffix(path, "/members")
	members, err := api.tenantMgr.ListMembers(id)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"members": members, "total": len(members)})
}

// handleTenantMemberAdd POST /api/v1/tenants/:id/members — 添加成员映射
func (api *ManagementAPI) handleTenantMemberAdd(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(path, "/members")

	var body struct {
		MatchType   string `json:"match_type"`
		MatchValue  string `json:"match_value"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if err := api.tenantMgr.AddMember(tenantID, body.MatchType, body.MatchValue, body.Description); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "added"})
}

// handleTenantMemberDelete DELETE /api/v1/tenants/:id/members/:mid — 删除成员映射
func (api *ManagementAPI) handleTenantMemberDelete(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	// /api/v1/tenants/xxx/members/123
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	parts := strings.SplitN(path, "/members/", 2)
	if len(parts) != 2 {
		jsonResponse(w, 400, map[string]string{"error": "invalid path"})
		return
	}
	mid, err := strconv.Atoi(parts[1])
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid member id"})
		return
	}
	if err := api.tenantMgr.RemoveMember(mid); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "deleted", "id": mid})
}

// handleTenantResolve GET /api/v1/tenants/resolve?sender_id=&app_id= — 测试租户解析
func (api *ManagementAPI) handleTenantResolve(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	senderID := r.URL.Query().Get("sender_id")
	appID := r.URL.Query().Get("app_id")
	tenantID := api.tenantMgr.ResolveTenant(senderID, appID)
	jsonResponse(w, 200, map[string]interface{}{
		"sender_id": senderID,
		"app_id":    appID,
		"tenant_id": tenantID,
	})
}

// ============================================================
// v14.0 租户安全配置 API
// ============================================================

// handleTenantConfigGet GET /api/v1/tenants/:id/config — 获取租户安全配置
func (api *ManagementAPI) handleTenantConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(path, "/config")
	if !api.tenantMgr.Exists(tenantID) {
		jsonResponse(w, 404, map[string]string{"error": "tenant not found"})
		return
	}
	cfg := api.tenantMgr.GetConfig(tenantID)
	// 附加全局入站规则列表（用于 UI 展示可禁用的规则）
	var globalRules []string
	if api.inboundEngine != nil {
		for _, rc := range api.inboundEngine.GetRuleConfigs() {
			globalRules = append(globalRules, rc.Name)
		}
	}
	jsonResponse(w, 200, map[string]interface{}{"config": cfg, "global_rules": globalRules})
}

// handleTenantConfigUpdate PUT /api/v1/tenants/:id/config — 更新租户安全配置
func (api *ManagementAPI) handleTenantConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.tenantMgr == nil {
		jsonResponse(w, 500, map[string]string{"error": "tenant manager not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(path, "/config")

	var cfg TenantConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	cfg.TenantID = tenantID
	if err := api.tenantMgr.UpdateConfig(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "config": cfg})
}

// ============================================================
// v14.1 认证 API handlers
// ============================================================

// handleAuthLogin POST /api/v1/auth/login — 用户登录
func (api *ManagementAPI) handleTenantBindTemplate(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	// 提取 tenant_id
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/bind-template")
	if api.tenantMgr != nil && !api.tenantMgr.Exists(tenantID) {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("租户 %q 不存在", tenantID)})
		return
	}
	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.TemplateID == "" {
		jsonResponse(w, 400, map[string]string{"error": "template_id required"})
		return
	}
	bound, err := api.pathPolicyEngine.BindTemplateToTenant(req.TemplateID, tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status":     "bound",
		"tenant_id":  tenantID,
		"template_id": req.TemplateID,
		"rules_bound": bound,
	})
}

// handleTenantPolicies GET /api/v1/tenants/:id/policies — 查看租户绑定的策略规则
func (api *ManagementAPI) handleTenantPolicies(w http.ResponseWriter, r *http.Request) {
	if api.pathPolicyEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "path policy engine not enabled"})
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/policies")
	rules := api.pathPolicyEngine.ListRulesForTenant(tenantID)
	if rules == nil {
		rules = []PathPolicyRule{}
	}
	jsonResponse(w, 200, map[string]interface{}{"tenant_id": tenantID, "rules": rules, "total": len(rules)})
}

// ============================================================
// v27.1 入站规则行业模板 API
// ============================================================

// handleInboundTemplateList GET /api/v1/inbound-templates — 列出所有入站规则模板
func (api *ManagementAPI) handleTenantBindInboundTemplate(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/bind-inbound-template")
	if tenantID == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant_id required"})
		return
	}
	if api.tenantMgr != nil && !api.tenantMgr.Exists(tenantID) {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("租户 %q 不存在", tenantID)})
		return
	}
	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.TemplateID == "" {
		jsonResponse(w, 400, map[string]string{"error": "template_id required"})
		return
	}
	// 查找模板
	templates := api.inboundEngine.ListInboundTemplates()
	var found *InboundRuleTemplate
	for i, tpl := range templates {
		if tpl.ID == req.TemplateID {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("inbound template %q not found", req.TemplateID)})
		return
	}
	// 复制规则并添加租户后缀
	tenantRules := make([]InboundRuleConfig, len(found.Rules))
	for i, rule := range found.Rules {
		tenantRules[i] = InboundRuleConfig{
			Name:     rule.Name + "-" + tenantID,
			Patterns: make([]string, len(rule.Patterns)),
			Action:   rule.Action,
			Category: rule.Category,
			Priority: rule.Priority,
			Message:  rule.Message,
			Type:     rule.Type,
			Group:    rule.Group,
		}
		copy(tenantRules[i].Patterns, rule.Patterns)
	}
	// 获取已有规则并追加（支持多次绑定不同模板）
	existing := api.inboundEngine.GetTenantRules(tenantID)
	merged := append(existing, tenantRules...)
	api.inboundEngine.SetTenantRules(tenantID, merged)
	jsonResponse(w, 200, map[string]interface{}{
		"status":      "bound",
		"tenant_id":   tenantID,
		"template_id": req.TemplateID,
		"rules_bound": len(tenantRules),
		"total_rules": len(merged),
	})
}

// handleTenantInboundRules GET /api/v1/tenants/:tid/inbound-rules — 获取租户的入站规则列表
func (api *ManagementAPI) handleTenantInboundRules(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/inbound-rules")
	rules := api.inboundEngine.GetTenantRules(tenantID)
	if rules == nil {
		rules = []InboundRuleConfig{}
	}
	jsonResponse(w, 200, map[string]interface{}{"tenant_id": tenantID, "rules": rules, "total": len(rules)})
}

// handleTenantDeleteInboundRules DELETE /api/v1/tenants/:tid/inbound-rules — 清除租户的入站规则
func (api *ManagementAPI) handleTenantDeleteInboundRules(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/inbound-rules")
	if tenantID == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant_id required"})
		return
	}
	api.inboundEngine.RemoveTenantRules(tenantID)
	jsonResponse(w, 200, map[string]interface{}{"status": "cleared", "tenant_id": tenantID})
}

// ============================================================
// v28.0 入站规则模板 CRUD API（POST/PUT/DELETE）
// ============================================================

// handleInboundTemplateCreate POST /api/v1/inbound-templates — 创建自定义入站规则模板
func (api *ManagementAPI) handleTenantBindLLMTemplate(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/bind-llm-template")
	if tenantID == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant_id required"})
		return
	}
	if api.tenantMgr != nil && !api.tenantMgr.Exists(tenantID) {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("租户 %q 不存在", tenantID)})
		return
	}
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM 规则引擎未启用"})
		return
	}
	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "无效的 JSON: " + err.Error()})
		return
	}
	if req.TemplateID == "" {
		jsonResponse(w, 400, map[string]string{"error": "template_id required"})
		return
	}
	// 查找模板
	tpl := api.llmRuleEngine.GetLLMTemplate(req.TemplateID)
	if tpl == nil {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("LLM 模板 %q 不存在", req.TemplateID)})
		return
	}
	// 复制规则并添加租户后缀
	tenantRules := make([]LLMRule, len(tpl.Rules))
	for i, rule := range tpl.Rules {
		tenantRules[i] = rule
		tenantRules[i].Name = rule.Name + "-" + tenantID
	}
	// 获取已有规则并追加（支持多次绑定不同模板）
	existing := api.llmRuleEngine.GetTenantLLMRules(tenantID)
	merged := append(existing, tenantRules...)
	api.llmRuleEngine.SetTenantLLMRules(tenantID, merged)
	jsonResponse(w, 200, map[string]interface{}{
		"status":      "bound",
		"tenant_id":   tenantID,
		"template_id": req.TemplateID,
		"rules_bound": len(tenantRules),
		"total_rules": len(merged),
	})
}

// handleTenantLLMRules GET /api/v1/tenants/:tid/llm-rules — 获取租户的 LLM 规则列表
func (api *ManagementAPI) handleTenantLLMRules(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/llm-rules")
	if api.llmRuleEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"tenant_id": tenantID, "rules": []LLMRule{}, "total": 0})
		return
	}
	rules := api.llmRuleEngine.GetTenantLLMRules(tenantID)
	if rules == nil {
		rules = []LLMRule{}
	}
	jsonResponse(w, 200, map[string]interface{}{"tenant_id": tenantID, "rules": rules, "total": len(rules)})
}

// handleTenantDeleteLLMRules DELETE /api/v1/tenants/:tid/llm-rules — 清除租户的 LLM 规则
func (api *ManagementAPI) handleTenantDeleteLLMRules(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
	tenantID := strings.TrimSuffix(trimmed, "/llm-rules")
	if tenantID == "" {
		jsonResponse(w, 400, map[string]string{"error": "tenant_id required"})
		return
	}
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM 规则引擎未启用"})
		return
	}
	api.llmRuleEngine.RemoveTenantLLMRules(tenantID)
	jsonResponse(w, 200, map[string]interface{}{"status": "cleared", "tenant_id": tenantID})
}
