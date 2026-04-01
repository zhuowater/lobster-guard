package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func (api *ManagementAPI) handleReloadRules(w http.ResponseWriter, r *http.Request) {
	newCfg, err := loadConfig(api.cfgPath)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "reload failed: " + err.Error()})
		return
	}
	api.outboundEngine.Reload(newCfg.OutboundRules)
	jsonResponse(w, 200, map[string]string{"status": "reloaded"})
}

func (api *ManagementAPI) handleRuleHits(w http.ResponseWriter, r *http.Request) {
	if api.ruleHits == nil {
		jsonResponse(w, 200, []RuleHitDetail{})
		return
	}
	// v3.11: 支持 ?group=xxx 筛选
	group := r.URL.Query().Get("group")
	if group != "" {
		details := api.ruleHits.GetDetailsByGroup(group)
		jsonResponse(w, 200, details)
		return
	}
	details := api.ruleHits.GetDetails()
	jsonResponse(w, 200, details)
}

// handleRuleHitsReset POST /api/v1/rules/hits/reset — 重置命中统计
func (api *ManagementAPI) handleRuleHitsReset(w http.ResponseWriter, r *http.Request) {
	if api.ruleHits == nil {
		jsonResponse(w, 200, map[string]string{"status": "no stats"})
		return
	}
	api.ruleHits.Reset()
	jsonResponse(w, 200, map[string]string{"status": "reset"})
}

// ============================================================
// v3.9 Management API 新端点
// ============================================================

// handleListUsers GET /api/v1/users — 列出所有已知用户
func (api *ManagementAPI) handleListRuleBindings(w http.ResponseWriter, r *http.Request) {
	bindings := api.inboundEngine.GetRuleBindings()
	jsonResponse(w, 200, map[string]interface{}{
		"bindings": bindings,
		"total":    len(bindings),
	})
}

// handleTestRuleBindings POST /api/v1/rule-bindings/test — 测试某个 app_id 会应用哪些规则
func (api *ManagementAPI) handleTestRuleBindings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID string `json:"app_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.AppID == "" {
		jsonResponse(w, 400, map[string]string{"error": "app_id required"})
		return
	}
	groups := api.inboundEngine.GetApplicableGroups(req.AppID)
	rules := api.inboundEngine.GetRulesForAppID(req.AppID)
	jsonResponse(w, 200, map[string]interface{}{
		"app_id":           req.AppID,
		"applicable_groups": groups,
		"all_rules_apply":  groups == nil,
		"rules":            rules,
		"rules_count":      len(rules),
	})
}

// handleListInboundRules GET /api/v1/inbound-rules — 列出当前入站规则
func (api *ManagementAPI) handleListInboundRules(w http.ResponseWriter, r *http.Request) {
	// v6.3: ?detail=1 返回含 patterns 的完整规则信息
	if r.URL.Query().Get("detail") == "1" {
		configs := api.inboundEngine.GetRuleConfigs()
		version := api.inboundEngine.Version()
		groupCounts := make(map[string]int)
		for _, c := range configs {
			if c.Group != "" {
				groupCounts[c.Group]++
			}
		}
		jsonResponse(w, 200, map[string]interface{}{
			"rules":        configs,
			"version":      version,
			"group_counts": groupCounts,
		})
		return
	}
	rules := api.inboundEngine.ListRules()
	version := api.inboundEngine.Version()
	// v3.11: 统计各分组的规则数
	groupCounts := make(map[string]int)
	for _, rule := range rules {
		if rule.Group != "" {
			groupCounts[rule.Group]++
		}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"rules":        rules,
		"version":      version,
		"group_counts": groupCounts,
	})
}

// handleReloadInboundRules POST /api/v1/inbound-rules/reload — 重新加载入站规则
func (api *ManagementAPI) handleReloadInboundRules(w http.ResponseWriter, r *http.Request) {
	// 重新加载配置
	newCfg, err := loadConfig(api.cfgPath)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "reload config failed: " + err.Error()})
		return
	}

	rules, source, err := resolveInboundRules(newCfg)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "resolve rules failed: " + err.Error()})
		return
	}

	if rules == nil {
		// 使用默认规则
		rules = getDefaultInboundRules()
		source = "default"
	}

	// v3.11: 同时更新规则绑定配置
	api.inboundEngine.ReloadWithBindings(rules, source, newCfg.RuleBindings)

	rv := api.inboundEngine.Version()
	jsonResponse(w, 200, map[string]interface{}{
		"status":        "ok",
		"rules_count":   rv.RuleCount,
		"patterns_count": rv.PatternCount,
		"source":        rv.Source,
		"version":       rv.Version,
	})
}

// handleListOutboundRules GET /api/v1/outbound-rules — 列出当前出站规则
func (api *ManagementAPI) handleListOutboundRules(w http.ResponseWriter, r *http.Request) {
	// v6.4: ?detail=1 返回含 patterns 的完整规则信息
	if r.URL.Query().Get("detail") == "1" {
		configs := api.outboundEngine.GetRuleConfigs()
		detailRules := make([]map[string]interface{}, len(configs))
		for i, c := range configs {
			m := map[string]interface{}{
				"name":           c.Name,
				"patterns":       c.Patterns,
				"patterns_count": len(c.Patterns),
				"action":         c.Action,
				"priority":       c.Priority,
				"message":        c.Message,
				"shadow_mode":    c.ShadowMode,
				"enabled":        isEnabled(c.Enabled),
			}
			if c.DisplayName != "" {
				m["display_name"] = c.DisplayName
			}
			detailRules[i] = m
		}
		piiPatterns := api.inboundEngine.ListPIIPatterns()
		jsonResponse(w, 200, map[string]interface{}{
			"rules":        detailRules,
			"total":        len(detailRules),
			"pii_patterns": piiPatterns,
		})
		return
	}

	api.outboundEngine.mu.RLock()
	rules := make([]map[string]interface{}, len(api.outboundEngine.rules))
	for i, rule := range api.outboundEngine.rules {
		m := map[string]interface{}{
			"name":           rule.Name,
			"patterns_count": len(rule.Regexps),
			"action":         rule.Action,
			"shadow_mode":    rule.ShadowMode,
			"enabled":        rule.Enabled,
		}
		if rule.DisplayName != "" {
			m["display_name"] = rule.DisplayName
		}
		m["shadow_mode"] = rule.ShadowMode
		m["enabled"] = rule.Enabled
		rules[i] = m
	}
	api.outboundEngine.mu.RUnlock()
	// v3.11: 包含 PII 模式列表
	piiPatterns := api.inboundEngine.ListPIIPatterns()
	jsonResponse(w, 200, map[string]interface{}{
		"rules":        rules,
		"total":        len(rules),
		"pii_patterns": piiPatterns,
	})
}

// persistOutboundRules 将出站规则写回 config.yaml 的 outbound_rules 字段
func (api *ManagementAPI) persistOutboundRules(configs []OutboundRuleConfig) error {
	api.cfgMu.Lock()
	defer api.cfgMu.Unlock()
	data, err := os.ReadFile(api.cfgPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}
	ruleList := make([]interface{}, len(configs))
	for i, c := range configs {
		m := map[string]interface{}{
			"name":   c.Name,
			"action": c.Action,
		}
		if c.Pattern != "" {
			m["pattern"] = c.Pattern
		}
		if len(c.Patterns) > 0 {
			m["patterns"] = c.Patterns
		}
		if c.Priority != 0 {
			m["priority"] = c.Priority
		}
		if c.Message != "" {
			m["message"] = c.Message
		}
		ruleList[i] = m
	}
	raw["outbound_rules"] = ruleList
	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(api.cfgPath, out, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	// 同步更新内存中的 cfg.OutboundRules
	api.cfg.OutboundRules = configs
	log.Printf("[出站规则] 已持久化 %d 条规则到 %s", len(configs), api.cfgPath)
	return nil
}

// handleAddOutboundRule POST /api/v1/outbound-rules/add — 添加出站规则
func (api *ManagementAPI) handleAddOutboundRule(w http.ResponseWriter, r *http.Request) {
	var req OutboundRuleConfig
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	if len(req.Patterns) == 0 && req.Pattern == "" {
		jsonResponse(w, 400, map[string]string{"error": "pattern or patterns required"})
		return
	}
	if req.Action == "" {
		req.Action = "log"
	}
	if !validateInboundAction(req.Action) {
		jsonResponse(w, 400, map[string]string{"error": "invalid action, must be block/review/warn/log"})
		return
	}

	if err := api.outboundEngine.AddRule(req); err != nil {
		jsonResponse(w, 409, map[string]string{"error": err.Error()})
		return
	}

	// 持久化
	configs := api.outboundEngine.GetRuleConfigs()
	if err := api.persistOutboundRules(configs); err != nil {
		log.Printf("[出站规则] 持久化失败: %v", err)
	}

	log.Printf("[出站规则CRUD] 添加规则: %s (action=%s)", req.Name, req.Action)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "added",
		"rule":   req.Name,
		"total":  len(configs),
	})
}

// handleUpdateOutboundRule PUT /api/v1/outbound-rules/update — 更新出站规则
func (api *ManagementAPI) handleUpdateOutboundRule(w http.ResponseWriter, r *http.Request) {
	var req OutboundRuleConfig
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	if len(req.Patterns) == 0 && req.Pattern == "" {
		jsonResponse(w, 400, map[string]string{"error": "pattern or patterns required"})
		return
	}
	if req.Action != "" && !validateInboundAction(req.Action) {
		jsonResponse(w, 400, map[string]string{"error": "invalid action, must be block/review/warn/log"})
		return
	}

	if err := api.outboundEngine.UpdateRule(req); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	// 持久化
	configs := api.outboundEngine.GetRuleConfigs()
	if err := api.persistOutboundRules(configs); err != nil {
		log.Printf("[出站规则] 持久化失败: %v", err)
	}

	log.Printf("[出站规则CRUD] 更新规则: %s", req.Name)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"rule":   req.Name,
		"total":  len(configs),
	})
}

// handleDeleteOutboundRule DELETE /api/v1/outbound-rules/delete — 删除出站规则
func (api *ManagementAPI) handleDeleteOutboundRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}

	if err := api.outboundEngine.DeleteRule(req.Name); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	// 持久化
	configs := api.outboundEngine.GetRuleConfigs()
	if err := api.persistOutboundRules(configs); err != nil {
		log.Printf("[出站规则] 持久化失败: %v", err)
	}

	log.Printf("[出站规则CRUD] 删除规则: %s", req.Name)
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "deleted",
		"rule":    req.Name,
		"total":   len(configs),
	})
}

// handleReloadOutboundRules POST /api/v1/outbound-rules/reload — 重新加载出站规则
func (api *ManagementAPI) handleReloadOutboundRules(w http.ResponseWriter, r *http.Request) {
	newCfg, err := loadConfig(api.cfgPath)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "reload config failed: " + err.Error()})
		return
	}
	api.outboundEngine.Reload(newCfg.OutboundRules)
	api.cfg.OutboundRules = newCfg.OutboundRules

	api.outboundEngine.mu.RLock()
	ruleCount := len(api.outboundEngine.rules)
	api.outboundEngine.mu.RUnlock()

	log.Printf("[出站规则] 从配置文件重新加载了 %d 条规则", ruleCount)
	jsonResponse(w, 200, map[string]interface{}{
		"status":      "reloaded",
		"rules_count": ruleCount,
	})
}

func (api *ManagementAPI) handleAddInboundRule(w http.ResponseWriter, r *http.Request) {
	var req InboundRuleConfig
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	if len(req.Patterns) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "patterns required"})
		return
	}
	if req.Action == "" {
		req.Action = "block"
	}
	if !validateInboundAction(req.Action) {
		jsonResponse(w, 400, map[string]string{"error": "invalid action, must be block/review/warn/log"})
		return
	}
	if req.Type != "" && req.Type != "keyword" && req.Type != "regex" {
		jsonResponse(w, 400, map[string]string{"error": "invalid type, must be keyword or regex"})
		return
	}

	// 获取当前规则列表并检查重名
	configs := api.inboundEngine.GetRuleConfigs()
	for _, c := range configs {
		if c.Name == req.Name {
			jsonResponse(w, 409, map[string]string{"error": "rule with name '" + req.Name + "' already exists"})
			return
		}
	}

	// 追加新规则
	configs = append(configs, req)
	source := api.inboundEngine.Version().Source
	api.inboundEngine.Reload(configs, source)

	// 持久化
	api.persistInboundRules(configs)

	log.Printf("[规则CRUD] 添加规则: %s (type=%s, action=%s, patterns=%d)", req.Name, req.Type, req.Action, len(req.Patterns))
	rules := api.inboundEngine.ListRules()
	jsonResponse(w, 200, map[string]interface{}{
		"status": "added",
		"rule":   req.Name,
		"rules":  rules,
		"total":  len(rules),
	})
}

// handleUpdateInboundRule PUT /api/v1/inbound-rules/update — 更新入站规则
func (api *ManagementAPI) handleUpdateInboundRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Patterns []string `json:"patterns"`
		Action   string   `json:"action"`
		Category string   `json:"category"`
		Priority int      `json:"priority"`
		Message  string   `json:"message"`
		Type     string   `json:"type"`
		Group    string   `json:"group"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	if req.Action != "" && !validateInboundAction(req.Action) {
		jsonResponse(w, 400, map[string]string{"error": "invalid action, must be block/review/warn/log"})
		return
	}
	if req.Type != "" && req.Type != "keyword" && req.Type != "regex" {
		jsonResponse(w, 400, map[string]string{"error": "invalid type, must be keyword or regex"})
		return
	}

	configs := api.inboundEngine.GetRuleConfigs()
	found := false
	for i, c := range configs {
		if c.Name == req.Name {
			if len(req.Patterns) > 0 {
				configs[i].Patterns = req.Patterns
			}
			if req.Action != "" {
				configs[i].Action = req.Action
			}
			if req.Category != "" {
				configs[i].Category = req.Category
			}
			configs[i].Priority = req.Priority
			configs[i].Message = req.Message
			if req.Type != "" {
				configs[i].Type = req.Type
			}
			configs[i].Group = req.Group
			found = true
			break
		}
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule '" + req.Name + "' not found"})
		return
	}

	source := api.inboundEngine.Version().Source
	api.inboundEngine.Reload(configs, source)
	api.persistInboundRules(configs)

	log.Printf("[规则CRUD] 更新规则: %s", req.Name)
	rules := api.inboundEngine.ListRules()
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"rule":   req.Name,
		"rules":  rules,
		"total":  len(rules),
	})
}

// handleDeleteInboundRule DELETE /api/v1/inbound-rules/delete — 删除入站规则
func (api *ManagementAPI) handleDeleteInboundRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}

	configs := api.inboundEngine.GetRuleConfigs()
	newConfigs := make([]InboundRuleConfig, 0, len(configs))
	found := false
	for _, c := range configs {
		if c.Name == req.Name {
			found = true
			continue
		}
		newConfigs = append(newConfigs, c)
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule '" + req.Name + "' not found"})
		return
	}

	source := api.inboundEngine.Version().Source
	api.inboundEngine.Reload(newConfigs, source)
	api.persistInboundRules(newConfigs)

	log.Printf("[规则CRUD] 删除规则: %s", req.Name)
	rules := api.inboundEngine.ListRules()
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "deleted",
		"rule":    req.Name,
		"rules":   rules,
		"total":   len(rules),
	})
}

// handleInboundToggleShadow POST /api/v1/inbound-rules/toggle-shadow — 切换入站规则影子模式
func (api *ManagementAPI) handleInboundToggleShadow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	configs := api.inboundEngine.GetRuleConfigs()
	found := false
	var newShadow bool
	for i, c := range configs {
		if c.Name == req.Name {
			configs[i].ShadowMode = !c.ShadowMode
			newShadow = configs[i].ShadowMode
			found = true
			break
		}
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule '" + req.Name + "' not found"})
		return
	}
	source := api.inboundEngine.Version().Source
	api.inboundEngine.Reload(configs, source)
	api.persistInboundRules(configs)
	mode := "active"
	if newShadow { mode = "shadow" }
	log.Printf("[规则CRUD] 入站规则 %s 切换为 %s 模式", req.Name, mode)
	jsonResponse(w, 200, map[string]interface{}{"status": "toggled", "rule": req.Name, "shadow_mode": newShadow})
}

// handleOutboundToggleShadow POST /api/v1/outbound-rules/toggle-shadow — 切换出站规则影子模式
func (api *ManagementAPI) handleOutboundToggleShadow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	api.outboundEngine.mu.Lock()
	found := false
	var newShadow bool
	for i, rule := range api.outboundEngine.rules {
		if rule.Name == req.Name {
			api.outboundEngine.rules[i].ShadowMode = !rule.ShadowMode
			newShadow = api.outboundEngine.rules[i].ShadowMode
			found = true
			break
		}
	}
	api.outboundEngine.mu.Unlock()
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule '" + req.Name + "' not found"})
		return
	}
	mode := "active"
	if newShadow { mode = "shadow" }
	log.Printf("[规则CRUD] 出站规则 %s 切换为 %s 模式", req.Name, mode)
	jsonResponse(w, 200, map[string]interface{}{"status": "toggled", "rule": req.Name, "shadow_mode": newShadow})
}

// handleExportRules GET /api/v1/rules/export — 导出所有入站规则为 YAML
func (api *ManagementAPI) handleExportRules(w http.ResponseWriter, r *http.Request) {
	configs := api.inboundEngine.GetRuleConfigs()
	rulesFile := InboundRulesFileConfig{Rules: configs}
	data, err := yaml.Marshal(&rulesFile)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "marshal failed: " + err.Error()})
		return
	}
	header := "# lobster-guard 入站规则导出\n# 导出时间: " + time.Now().Format(time.RFC3339) + "\n\n"
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=lobster-guard-rules.yaml")
	w.WriteHeader(200)
	w.Write([]byte(header + string(data)))
}

// handleImportRules POST /api/v1/rules/import — 导入 YAML 规则
func (api *ManagementAPI) handleImportRules(w http.ResponseWriter, r *http.Request) {
	// 读取请求体（支持 raw YAML body 或 JSON body 包含 yaml 字段）
	body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024)) // max 2MB
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "read body failed: " + err.Error()})
		return
	}

	var importRules []InboundRuleConfig

	// 尝试解析为 JSON 包装格式 {"yaml": "..."}
	var jsonReq struct {
		YAML string `json:"yaml"`
		Mode string `json:"mode"` // "merge" 或 "replace"，默认 merge
	}
	if json.Unmarshal(body, &jsonReq) == nil && jsonReq.YAML != "" {
		body = []byte(jsonReq.YAML)
	}

	// 解析 YAML
	var rulesFile InboundRulesFileConfig
	if err := yaml.Unmarshal(body, &rulesFile); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid YAML: " + err.Error()})
		return
	}

	// 验证规则
	for i, rule := range rulesFile.Rules {
		if rule.Name == "" {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("规则 #%d 缺少 name 字段", i+1)})
			return
		}
		if len(rule.Patterns) == 0 {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("规则 %q 缺少 patterns", rule.Name)})
			return
		}
		if rule.Action == "" {
			rulesFile.Rules[i].Action = "block"
		} else if !validateInboundAction(rule.Action) {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("规则 %q 的 action %q 无效", rule.Name, rule.Action)})
			return
		}
		if rule.Type != "" && rule.Type != "keyword" && rule.Type != "regex" {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("规则 %q 的 type %q 无效", rule.Name, rule.Type)})
			return
		}
	}
	importRules = rulesFile.Rules

	// 计算预览信息
	currentConfigs := api.inboundEngine.GetRuleConfigs()
	currentNames := make(map[string]bool)
	for _, c := range currentConfigs {
		currentNames[c.Name] = true
	}

	var newRules, overrideRules []string
	for _, r := range importRules {
		if currentNames[r.Name] {
			overrideRules = append(overrideRules, r.Name)
		} else {
			newRules = append(newRules, r.Name)
		}
	}

	// 如果是 preview 模式（query param ?preview=1），只返回预览
	if r.URL.Query().Get("preview") == "1" {
		jsonResponse(w, 200, map[string]interface{}{
			"preview":       true,
			"total":         len(importRules),
			"new_rules":     newRules,
			"override_rules": overrideRules,
			"new_count":     len(newRules),
			"override_count": len(overrideRules),
		})
		return
	}

	// 合并规则（merge 模式：导入的覆盖同名的，新增的追加）
	merged := make([]InboundRuleConfig, 0, len(currentConfigs)+len(newRules))
	importMap := make(map[string]InboundRuleConfig)
	for _, r := range importRules {
		importMap[r.Name] = r
	}
	for _, c := range currentConfigs {
		if imp, ok := importMap[c.Name]; ok {
			merged = append(merged, imp)
			delete(importMap, c.Name)
		} else {
			merged = append(merged, c)
		}
	}
	// 追加新规则
	for _, r := range importRules {
		if _, ok := importMap[r.Name]; ok {
			merged = append(merged, r)
		}
	}

	source := api.inboundEngine.Version().Source
	api.inboundEngine.Reload(merged, source)
	api.persistInboundRules(merged)

	log.Printf("[规则导入] 导入 %d 条规则（新增 %d，覆盖 %d）", len(importRules), len(newRules), len(overrideRules))
	rules := api.inboundEngine.ListRules()
	jsonResponse(w, 200, map[string]interface{}{
		"status":         "imported",
		"imported":       len(importRules),
		"new_count":      len(newRules),
		"override_count": len(overrideRules),
		"rules":          rules,
		"total":          len(rules),
	})
}

// handleRuleTemplateDetail GET /api/v1/rule-templates/detail?name=xxx — 获取模板详情（v30.0: 转发到入站模板）
