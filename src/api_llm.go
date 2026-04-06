package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func (api *ManagementAPI) handleLLMStatus(w http.ResponseWriter, r *http.Request) {
	cfg := api.cfg.LLMProxy
	targets := []string{}
	for _, t := range cfg.Targets {
		targets = append(targets, t.Name)
	}
	jsonResponse(w, 200, map[string]interface{}{
		"enabled":  cfg.Enabled,
		"listen":   cfg.Listen,
		"targets":  targets,
		"status":   "healthy",
	})
}

// handleLLMOverview GET /api/v1/llm/overview — LLM 概览统计（v11.4: 支持 ?since 参数, v14.0: 支持 ?tenant）
func (api *ManagementAPI) handleLLMOverview(w http.ResponseWriter, r *http.Request) {
	since := r.URL.Query().Get("since")
	sinceTime := parseSinceParam(since)
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	overview, err := api.llmAuditor.OverviewWithFilterTenant(sinceTime, tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if since != "" {
		overview["time_range"] = since
	} else {
		overview["time_range"] = "all"
	}
	overview["tenant"] = tenantID
	jsonResponse(w, 200, overview)
}

// handleLLMCalls GET /api/v1/llm/calls — LLM 调用列表
func (api *ManagementAPI) handleLLMCalls(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	hasToolUse := r.URL.Query().Get("has_tool_use")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil { limit = n }
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil { offset = n }
	}
	records, total, err := api.llmAuditor.QueryCalls(model, hasToolUse, from, to, limit, offset)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if records == nil { records = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"records": records, "total": total})
}

// handleLLMTools GET /api/v1/llm/tools — 工具调用列表
func (api *ManagementAPI) handleLLMTools(w http.ResponseWriter, r *http.Request) {
	tool := r.URL.Query().Get("tool_name")
	risk := r.URL.Query().Get("risk_level")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil { limit = n }
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil { offset = n }
	}
	records, total, err := api.llmAuditor.QueryToolCalls(tool, risk, from, to, limit, offset)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if records == nil { records = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"records": records, "total": total})
}

// handleLLMToolStats GET /api/v1/llm/tools/stats — 工具统计
func (api *ManagementAPI) handleLLMToolStats(w http.ResponseWriter, r *http.Request) {
	stats, err := api.llmAuditor.ToolStats()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, stats)
}

// handleLLMToolTimeline GET /api/v1/llm/tools/timeline — 工具调用时间线
func (api *ManagementAPI) handleLLMToolTimeline(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 { hours = n }
	}
	timeline, err := api.llmAuditor.ToolTimeline(hours)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if timeline == nil { timeline = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"timeline": timeline, "hours": hours})
}

// handleLLMConfigGet GET /api/v1/llm/config — 获取 LLM 代理完整配置（脱敏）
func (api *ManagementAPI) handleLLMConfigGet(w http.ResponseWriter, r *http.Request) {
	cfg := api.cfg.LLMProxy
	// 脱敏: targets 中不返回 API key 值（header 名可以返回）
	targets := make([]map[string]interface{}, len(cfg.Targets))
	for i, t := range cfg.Targets {
		targets[i] = map[string]interface{}{
			"name":           t.Name,
			"upstream":       t.Upstream,
			"path_prefix":    t.PathPrefix,
			"api_key_header": t.APIKeyHeader,
		}
	}
	result := map[string]interface{}{
		"enabled": cfg.Enabled,
		"listen":  cfg.Listen,
		"targets": targets,
		"audit": map[string]interface{}{
			"log_system_prompt": cfg.AuditConfig.LogSystemPrompt,
			"log_tool_input":    cfg.AuditConfig.LogToolInput,
			"log_tool_result":   cfg.AuditConfig.LogToolResult,
			"max_preview_len":   cfg.AuditConfig.MaxPreviewLen,
		},
		"timeout_sec": cfg.TimeoutSec,
		"cost_alert": map[string]interface{}{
			"daily_limit_usd": cfg.CostAlert.DailyLimitUSD,
			"webhook_url":     cfg.CostAlert.WebhookURL,
		},
		"security": map[string]interface{}{
			"scan_pii_in_response":  cfg.Security.ScanPIIInResponse,
			"block_high_risk_tools": cfg.Security.BlockHighRiskTools,
			"high_risk_tool_list":   cfg.Security.HighRiskToolList,
			"prompt_injection_scan": cfg.Security.PromptInjectionScan,
			"canary_token": map[string]interface{}{
				"enabled":      cfg.Security.CanaryToken.Enabled,
				"auto_rotate":  cfg.Security.CanaryToken.AutoRotate,
				"alert_action": cfg.Security.CanaryToken.AlertAction,
			},
			"response_budget": map[string]interface{}{
				"enabled":               cfg.Security.ResponseBudget.Enabled,
				"max_tool_calls_per_req":  cfg.Security.ResponseBudget.MaxToolCallsPerReq,
				"max_single_tool_per_req": cfg.Security.ResponseBudget.MaxSingleToolPerReq,
				"max_tokens_per_req":      cfg.Security.ResponseBudget.MaxTokensPerReq,
				"over_budget_action":      cfg.Security.ResponseBudget.OverBudgetAction,
				"tool_limits":            cfg.Security.ResponseBudget.ToolLimits,
			},
		},
	}
	jsonResponse(w, 200, result)
}

// handleLLMConfigPut PUT /api/v1/llm/config — 更新 LLM 代理配置（写回 config.yaml）
func (api *ManagementAPI) handleLLMConfigPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled    *bool `json:"enabled"`
		Listen     string `json:"listen"`
		Audit      *struct {
			LogSystemPrompt *bool `json:"log_system_prompt"`
			LogToolInput    *bool `json:"log_tool_input"`
			LogToolResult   *bool `json:"log_tool_result"`
			MaxPreviewLen   *int  `json:"max_preview_len"`
		} `json:"audit"`
		TimeoutSec *int `json:"timeout_sec"`
		CostAlert  *struct {
			DailyLimitUSD *float64 `json:"daily_limit_usd"`
			WebhookURL    *string  `json:"webhook_url"`
		} `json:"cost_alert"`
		Security *struct {
			ScanPIIInResponse   *bool    `json:"scan_pii_in_response"`
			BlockHighRiskTools  *bool    `json:"block_high_risk_tools"`
			HighRiskToolList    []string `json:"high_risk_tool_list"`
			PromptInjectionScan *bool    `json:"prompt_injection_scan"`
			CanaryToken *struct {
				Enabled     *bool   `json:"enabled"`
				AutoRotate  *bool   `json:"auto_rotate"`
				AlertAction *string `json:"alert_action"`
			} `json:"canary_token"`
			ResponseBudget *struct {
				Enabled             *bool          `json:"enabled"`
				MaxToolCallsPerReq  *int           `json:"max_tool_calls_per_req"`
				MaxSingleToolPerReq *int           `json:"max_single_tool_per_req"`
				MaxTokensPerReq     *int           `json:"max_tokens_per_req"`
				OverBudgetAction    *string        `json:"over_budget_action"`
				ToolLimits          map[string]int `json:"tool_limits"`
			} `json:"response_budget"`
		} `json:"security"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}

	// 更新内存配置
	cfg := &api.cfg.LLMProxy
	needRestart := false
	if req.Enabled != nil && *req.Enabled != cfg.Enabled {
		cfg.Enabled = *req.Enabled
		needRestart = true
	}
	if req.Listen != "" && req.Listen != cfg.Listen {
		cfg.Listen = req.Listen
		needRestart = true
	}
	if req.Audit != nil {
		if req.Audit.LogSystemPrompt != nil { cfg.AuditConfig.LogSystemPrompt = *req.Audit.LogSystemPrompt }
		if req.Audit.LogToolInput != nil { cfg.AuditConfig.LogToolInput = *req.Audit.LogToolInput }
		if req.Audit.LogToolResult != nil { cfg.AuditConfig.LogToolResult = *req.Audit.LogToolResult }
		if req.Audit.MaxPreviewLen != nil { cfg.AuditConfig.MaxPreviewLen = *req.Audit.MaxPreviewLen }
	}
	if req.TimeoutSec != nil { cfg.TimeoutSec = *req.TimeoutSec }
	if req.CostAlert != nil {
		if req.CostAlert.DailyLimitUSD != nil { cfg.CostAlert.DailyLimitUSD = *req.CostAlert.DailyLimitUSD }
		if req.CostAlert.WebhookURL != nil { cfg.CostAlert.WebhookURL = *req.CostAlert.WebhookURL }
	}
	if req.Security != nil {
		if req.Security.ScanPIIInResponse != nil { cfg.Security.ScanPIIInResponse = *req.Security.ScanPIIInResponse }
		if req.Security.BlockHighRiskTools != nil { cfg.Security.BlockHighRiskTools = *req.Security.BlockHighRiskTools }
		if req.Security.HighRiskToolList != nil { cfg.Security.HighRiskToolList = req.Security.HighRiskToolList }
		if req.Security.PromptInjectionScan != nil { cfg.Security.PromptInjectionScan = *req.Security.PromptInjectionScan }
		// v10.1: Canary Token
		if req.Security.CanaryToken != nil {
			if req.Security.CanaryToken.Enabled != nil { cfg.Security.CanaryToken.Enabled = *req.Security.CanaryToken.Enabled }
			if req.Security.CanaryToken.AutoRotate != nil { cfg.Security.CanaryToken.AutoRotate = *req.Security.CanaryToken.AutoRotate }
			if req.Security.CanaryToken.AlertAction != nil { cfg.Security.CanaryToken.AlertAction = *req.Security.CanaryToken.AlertAction }
		}
		// v10.1: Response Budget
		if req.Security.ResponseBudget != nil {
			if req.Security.ResponseBudget.Enabled != nil { cfg.Security.ResponseBudget.Enabled = *req.Security.ResponseBudget.Enabled }
			if req.Security.ResponseBudget.MaxToolCallsPerReq != nil { cfg.Security.ResponseBudget.MaxToolCallsPerReq = *req.Security.ResponseBudget.MaxToolCallsPerReq }
			if req.Security.ResponseBudget.MaxSingleToolPerReq != nil { cfg.Security.ResponseBudget.MaxSingleToolPerReq = *req.Security.ResponseBudget.MaxSingleToolPerReq }
			if req.Security.ResponseBudget.MaxTokensPerReq != nil { cfg.Security.ResponseBudget.MaxTokensPerReq = *req.Security.ResponseBudget.MaxTokensPerReq }
			if req.Security.ResponseBudget.OverBudgetAction != nil { cfg.Security.ResponseBudget.OverBudgetAction = *req.Security.ResponseBudget.OverBudgetAction }
			if req.Security.ResponseBudget.ToolLimits != nil { cfg.Security.ResponseBudget.ToolLimits = req.Security.ResponseBudget.ToolLimits }
		}
	}

	// 同步更新 llmAuditor 的审计配置
	if api.llmAuditor != nil && req.Audit != nil {
		api.llmAuditor.cfg = cfg.AuditConfig
	}

	// 写回 config.yaml
	if err := api.saveLLMConfig(); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "保存配置失败: " + err.Error()})
		return
	}

	log.Printf("[LLM配置] 配置已更新并写回 %s", api.cfgPath)
	result := map[string]interface{}{
		"status":       "ok",
		"need_restart": needRestart,
	}
	if needRestart {
		result["message"] = "部分配置变更（enabled/listen）需要重启服务生效"
	}
	jsonResponse(w, 200, result)
}

// saveLLMConfig 将 LLM 代理配置写回 config.yaml
func (api *ManagementAPI) saveLLMConfig() error {
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

	cfg := api.cfg.LLMProxy
	llmProxy := map[string]interface{}{
		"enabled":    cfg.Enabled,
		"listen":     cfg.Listen,
		"timeout_sec": cfg.TimeoutSec,
	}

	// targets
	targets := make([]interface{}, len(cfg.Targets))
	for i, t := range cfg.Targets {
		targets[i] = map[string]interface{}{
			"name":           t.Name,
			"upstream":       t.Upstream,
			"path_prefix":    t.PathPrefix,
			"strip_prefix":   t.StripPrefix,
			"api_key_header": t.APIKeyHeader,
		}
	}
	llmProxy["targets"] = targets

	// audit
	llmProxy["audit"] = map[string]interface{}{
		"log_system_prompt": cfg.AuditConfig.LogSystemPrompt,
		"log_tool_input":    cfg.AuditConfig.LogToolInput,
		"log_tool_result":   cfg.AuditConfig.LogToolResult,
		"max_preview_len":   cfg.AuditConfig.MaxPreviewLen,
	}

	// cost_alert
	llmProxy["cost_alert"] = map[string]interface{}{
		"daily_limit_usd": cfg.CostAlert.DailyLimitUSD,
		"webhook_url":     cfg.CostAlert.WebhookURL,
	}

	// security
	securityMap := map[string]interface{}{
		"scan_pii_in_response":  cfg.Security.ScanPIIInResponse,
		"block_high_risk_tools": cfg.Security.BlockHighRiskTools,
		"high_risk_tool_list":   cfg.Security.HighRiskToolList,
		"prompt_injection_scan": cfg.Security.PromptInjectionScan,
	}
	// v10.1: canary_token
	canaryMap := map[string]interface{}{
		"enabled":      cfg.Security.CanaryToken.Enabled,
		"token":        cfg.Security.CanaryToken.Token,
		"auto_rotate":  cfg.Security.CanaryToken.AutoRotate,
		"alert_action": cfg.Security.CanaryToken.AlertAction,
	}
	securityMap["canary_token"] = canaryMap
	// v10.1: response_budget
	budgetMap := map[string]interface{}{
		"enabled":               cfg.Security.ResponseBudget.Enabled,
		"max_tool_calls_per_req":  cfg.Security.ResponseBudget.MaxToolCallsPerReq,
		"max_single_tool_per_req": cfg.Security.ResponseBudget.MaxSingleToolPerReq,
		"max_tokens_per_req":      cfg.Security.ResponseBudget.MaxTokensPerReq,
		"over_budget_action":      cfg.Security.ResponseBudget.OverBudgetAction,
	}
	if cfg.Security.ResponseBudget.ToolLimits != nil {
		budgetMap["tool_limits"] = cfg.Security.ResponseBudget.ToolLimits
	}
	securityMap["response_budget"] = budgetMap
	llmProxy["security"] = securityMap

	// v10.0: 规则
	if len(cfg.Rules) > 0 {
		var rulesList []interface{}
		for _, rule := range cfg.Rules {
			rm := map[string]interface{}{
				"id":        rule.ID,
				"name":      rule.Name,
				"category":  rule.Category,
				"direction": rule.Direction,
				"type":      rule.Type,
				"patterns":  rule.Patterns,
				"action":    rule.Action,
				"enabled":   rule.Enabled,
				"priority":  rule.Priority,
			}
			if rule.Description != "" {
				rm["description"] = rule.Description
			}
			if rule.RewriteTo != "" {
				rm["rewrite_to"] = rule.RewriteTo
			}
			if rule.ShadowMode {
				rm["shadow_mode"] = true
			}
			rulesList = append(rulesList, rm)
		}
		llmProxy["rules"] = rulesList
	}

	raw["llm_proxy"] = llmProxy
	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(api.cfgPath, out, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// v35.1: 同步 conf.d/ 中含 llm_proxy 的文件，防止重启后 conf.d 覆盖
	// 找到所有可能含 llm_proxy 的 conf.d 文件并更新其中的 llm_proxy 节
	confDir := filepath.Join(filepath.Dir(api.cfgPath), "conf.d")
	if entries, err := os.ReadDir(confDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			confPath := filepath.Join(confDir, entry.Name())
			confData, err := os.ReadFile(confPath)
			if err != nil {
				continue
			}
			var confRaw map[string]interface{}
			if err := yaml.Unmarshal(confData, &confRaw); err != nil {
				continue
			}
			if _, hasLLM := confRaw["llm_proxy"]; !hasLLM {
				continue
			}
			// 该 conf.d 文件含 llm_proxy，覆盖其 llm_proxy 节
			confRaw["llm_proxy"] = llmProxy
			confOut, err := yaml.Marshal(confRaw)
			if err != nil {
				log.Printf("[LLM配置] 序列化 conf.d/%s 失败: %v", entry.Name(), err)
				continue
			}
			if err := os.WriteFile(confPath, confOut, 0644); err != nil {
				log.Printf("[LLM配置] 写入 conf.d/%s 失败: %v", entry.Name(), err)
			} else {
				log.Printf("[LLM配置] 同步 conf.d/%s", entry.Name())
			}
		}
	}
	return nil
}

// ============================================================
// v10.0 LLM 规则 CRUD API
// ============================================================

// ─── LLM Targets CRUD ────────────────────────────────────────────────────────

// handleLLMTargetsList GET /api/v1/llm/targets — LLM 上游目标列表
func (api *ManagementAPI) handleLLMTargetsList(w http.ResponseWriter, r *http.Request) {
	targets := api.cfg.LLMProxy.Targets
	if targets == nil {
		targets = []LLMTargetConfig{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"targets": targets,
		"total":   len(targets),
	})
}

// handleLLMTargetsCreate POST /api/v1/llm/targets — 新建上游目标
func (api *ManagementAPI) handleLLMTargetsCreate(w http.ResponseWriter, r *http.Request) {
	var req LLMTargetConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body: " + err.Error()})
		return
	}
	if req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "name is required"})
		return
	}
	if req.Upstream == "" {
		jsonResponse(w, 400, map[string]string{"error": "upstream is required"})
		return
	}
	// 检查名称重复
	for _, t := range api.cfg.LLMProxy.Targets {
		if t.Name == req.Name {
			jsonResponse(w, 409, map[string]string{"error": "target with name '" + req.Name + "' already exists"})
			return
		}
	}

	api.cfg.LLMProxy.Targets = append(api.cfg.LLMProxy.Targets, req)

	// 热更新 LLM 代理的目标列表
	if api.llmProxy != nil {
		api.llmProxy.cfg.Targets = api.cfg.LLMProxy.Targets
	}

	if err := api.saveLLMConfig(); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "保存配置失败: " + err.Error()})
		return
	}

	log.Printf("[LLM目标] 新建目标: %s → %s (prefix=%s)", req.Name, req.Upstream, req.PathPrefix)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"target": req,
		"total":  len(api.cfg.LLMProxy.Targets),
	})
}

// handleLLMTargetsUpdate PUT /api/v1/llm/targets/:name — 编辑上游目标（部分更新）
func (api *ManagementAPI) handleLLMTargetsUpdate(w http.ResponseWriter, r *http.Request) {
	targetName := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/targets/")
	if targetName == "" {
		jsonResponse(w, 400, map[string]string{"error": "target name required"})
		return
	}

	// 使用 raw JSON 解析以区分"未传"和"传了零值"
	bodyBytes, _ := io.ReadAll(r.Body)
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawFields); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	found := false
	var updated LLMTargetConfig
	for i, existing := range api.cfg.LLMProxy.Targets {
		if existing.Name == targetName {
			updated = existing
			if v, ok := rawFields["upstream"]; ok {
				json.Unmarshal(v, &updated.Upstream)
			}
			if v, ok := rawFields["path_prefix"]; ok {
				json.Unmarshal(v, &updated.PathPrefix)
			}
			if v, ok := rawFields["strip_prefix"]; ok {
				json.Unmarshal(v, &updated.StripPrefix)
			}
			if v, ok := rawFields["api_key_header"]; ok {
				json.Unmarshal(v, &updated.APIKeyHeader)
			}
			api.cfg.LLMProxy.Targets[i] = updated
			found = true
			break
		}
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "target not found: " + targetName})
		return
	}

	// 热更新 LLM 代理的目标列表
	if api.llmProxy != nil {
		api.llmProxy.cfg.Targets = api.cfg.LLMProxy.Targets
	}

	if err := api.saveLLMConfig(); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "保存配置失败: " + err.Error()})
		return
	}

	log.Printf("[LLM目标] 更新目标: %s", targetName)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"target": updated,
	})
}

// handleLLMTargetsDelete DELETE /api/v1/llm/targets/:name — 删除上游目标
func (api *ManagementAPI) handleLLMTargetsDelete(w http.ResponseWriter, r *http.Request) {
	targetName := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/targets/")
	if targetName == "" {
		jsonResponse(w, 400, map[string]string{"error": "target name required"})
		return
	}

	found := false
	var newTargets []LLMTargetConfig
	for _, existing := range api.cfg.LLMProxy.Targets {
		if existing.Name == targetName {
			found = true
			continue
		}
		newTargets = append(newTargets, existing)
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "target not found: " + targetName})
		return
	}

	if newTargets == nil {
		newTargets = []LLMTargetConfig{}
	}
	api.cfg.LLMProxy.Targets = newTargets

	// 热更新 LLM 代理的目标列表
	if api.llmProxy != nil {
		api.llmProxy.cfg.Targets = api.cfg.LLMProxy.Targets
	}

	if err := api.saveLLMConfig(); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "保存配置失败: " + err.Error()})
		return
	}

	log.Printf("[LLM目标] 删除目标: %s", targetName)
	jsonResponse(w, 200, map[string]interface{}{
		"status":      "deleted",
		"target_name": targetName,
		"total":       len(newTargets),
	})
}

// ─── LLM Rules CRUD ─────────────────────────────────────────────────────────

// handleLLMRulesList GET /api/v1/llm/rules — LLM 规则列表
func (api *ManagementAPI) handleLLMRulesList(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		// 未启用时返回默认规则列表
		jsonResponse(w, 200, map[string]interface{}{
			"rules": defaultLLMRules,
			"total": len(defaultLLMRules),
		})
		return
	}
	rules := api.llmRuleEngine.GetRules()
	jsonResponse(w, 200, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// handleLLMRulesCreate POST /api/v1/llm/rules — 新建规则
func (api *ManagementAPI) handleLLMRulesCreate(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM rule engine not initialized"})
		return
	}
	var req LLMRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request, name required"})
		return
	}
	if len(req.Patterns) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "patterns required"})
		return
	}
	if req.ID == "" {
		req.ID = fmt.Sprintf("llm-custom-%d", time.Now().UnixNano()%100000)
	}
	if req.Type == "" {
		req.Type = "keyword"
	}
	if req.Action == "" {
		req.Action = "log"
	}

	rules := api.llmRuleEngine.GetRules()
	// 检查 ID 重复
	for _, existing := range rules {
		if existing.ID == req.ID {
			jsonResponse(w, 409, map[string]string{"error": "rule with ID '" + req.ID + "' already exists"})
			return
		}
	}

	rules = append(rules, req)
	api.llmRuleEngine.UpdateRules(rules)
	api.persistLLMRules(rules)

	log.Printf("[LLM规则] 新建规则: %s (type=%s, action=%s, patterns=%d)", req.ID, req.Type, req.Action, len(req.Patterns))
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"rule":   req,
		"total":  len(rules),
	})
}

// handleLLMRulesUpdate PUT /api/v1/llm/rules/:id — 编辑规则（完整替换）
func (api *ManagementAPI) handleLLMRulesUpdate(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM rule engine not initialized"})
		return
	}
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/rules/")
	if ruleID == "" {
		jsonResponse(w, 400, map[string]string{"error": "rule id required"})
		return
	}

	// 使用 raw JSON 解析以区分"未传"和"传了零值"
	bodyBytes, _ := io.ReadAll(r.Body)
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawFields); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	rules := api.llmRuleEngine.GetRules()
	found := false
	var updated LLMRule
	for i, existing := range rules {
		if existing.ID == ruleID {
			// 以现有规则为基础
			updated = existing
			// 只覆盖请求中明确传入的字段
			if v, ok := rawFields["name"]; ok { json.Unmarshal(v, &updated.Name) }
			if v, ok := rawFields["description"]; ok { json.Unmarshal(v, &updated.Description) }
			if v, ok := rawFields["category"]; ok { json.Unmarshal(v, &updated.Category) }
			if v, ok := rawFields["direction"]; ok { json.Unmarshal(v, &updated.Direction) }
			if v, ok := rawFields["type"]; ok { json.Unmarshal(v, &updated.Type) }
			if v, ok := rawFields["patterns"]; ok { json.Unmarshal(v, &updated.Patterns) }
			if v, ok := rawFields["action"]; ok { json.Unmarshal(v, &updated.Action) }
			if v, ok := rawFields["rewrite_to"]; ok { json.Unmarshal(v, &updated.RewriteTo) }
			if v, ok := rawFields["enabled"]; ok { json.Unmarshal(v, &updated.Enabled) }
			if v, ok := rawFields["priority"]; ok { json.Unmarshal(v, &updated.Priority) }
			if v, ok := rawFields["shadow_mode"]; ok { json.Unmarshal(v, &updated.ShadowMode) }
			rules[i] = updated
			found = true
			break
		}
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule not found: " + ruleID})
		return
	}

	api.llmRuleEngine.UpdateRules(rules)
	api.persistLLMRules(rules)

	log.Printf("[LLM规则] 更新规则: %s", ruleID)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "updated",
		"rule":   updated,
	})
}

// handleLLMRulesDelete DELETE /api/v1/llm/rules/:id — 删除规则
func (api *ManagementAPI) handleLLMRulesDelete(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM rule engine not initialized"})
		return
	}
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/rules/")
	if ruleID == "" {
		jsonResponse(w, 400, map[string]string{"error": "rule id required"})
		return
	}

	rules := api.llmRuleEngine.GetRules()
	found := false
	var newRules []LLMRule
	for _, existing := range rules {
		if existing.ID == ruleID {
			found = true
			continue
		}
		newRules = append(newRules, existing)
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule not found: " + ruleID})
		return
	}

	api.llmRuleEngine.UpdateRules(newRules)
	api.persistLLMRules(newRules)

	log.Printf("[LLM规则] 删除规则: %s", ruleID)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "deleted",
		"rule_id": ruleID,
		"total":   len(newRules),
	})
}

// handleLLMRulesHits GET /api/v1/llm/rules/hits — 规则命中统计
func (api *ManagementAPI) handleLLMRulesHits(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{"hits": map[string]interface{}{}})
		return
	}
	hits := api.llmRuleEngine.GetHits()
	// 转换为 JSON 友好格式
	result := make(map[string]interface{}, len(hits))
	for id, h := range hits {
		result[id] = map[string]interface{}{
			"count":       h.Count,
			"last_hit":    h.LastHit.Format(time.RFC3339),
			"shadow_hits": h.ShadowHits,
		}
	}
	jsonResponse(w, 200, map[string]interface{}{"hits": result})
}

// handleLLMRulesToggleShadow POST /api/v1/llm/rules/:id/toggle-shadow — 切换影子模式
func (api *ManagementAPI) handleLLMRulesToggleShadow(w http.ResponseWriter, r *http.Request) {
	if api.llmRuleEngine == nil {
		jsonResponse(w, 400, map[string]string{"error": "LLM rule engine not initialized"})
		return
	}
	// 解析 rule ID: /api/v1/llm/rules/{id}/toggle-shadow
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/llm/rules/")
	ruleID := strings.TrimSuffix(path, "/toggle-shadow")
	if ruleID == "" {
		jsonResponse(w, 400, map[string]string{"error": "rule id required"})
		return
	}

	rules := api.llmRuleEngine.GetRules()
	found := false
	var newShadow bool
	for i, existing := range rules {
		if existing.ID == ruleID {
			rules[i].ShadowMode = !existing.ShadowMode
			newShadow = rules[i].ShadowMode
			found = true
			break
		}
	}
	if !found {
		jsonResponse(w, 404, map[string]string{"error": "rule not found: " + ruleID})
		return
	}

	api.llmRuleEngine.UpdateRules(rules)
	api.persistLLMRules(rules)

	mode := "active"
	if newShadow {
		mode = "shadow"
	}
	log.Printf("[LLM规则] 切换影子模式: %s → %s", ruleID, mode)
	jsonResponse(w, 200, map[string]interface{}{
		"status":      "toggled",
		"rule_id":     ruleID,
		"shadow_mode": newShadow,
	})
}

// persistLLMRules 将 LLM 规则持久化到 config.yaml
func (api *ManagementAPI) persistLLMRules(rules []LLMRule) {
	api.cfg.LLMProxy.Rules = rules
	if err := api.saveLLMConfig(); err != nil {
		log.Printf("[LLM规则] 持久化失败: %v", err)
	} else {
		log.Printf("[LLM规则] 已持久化 %d 条规则到 %s", len(rules), api.cfgPath)
	}
}

// ============================================================
// v10.1 Canary Token API
// ============================================================

// handleCanaryStatus GET /api/v1/llm/canary/status — Canary Token 状态
func (api *ManagementAPI) handleLLMExport(w http.ResponseWriter, r *http.Request) {
	if api.llmAuditor == nil {
		jsonResponse(w, 404, map[string]string{"error": "LLM proxy not enabled"})
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	dataType := r.URL.Query().Get("data") // "calls" or "tools"
	if dataType == "" {
		dataType = "calls"
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if dataType == "tools" {
		records, _, err := api.llmAuditor.QueryToolCalls("", "", from, to, 10000, 0)
		if err != nil {
			jsonResponse(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", "attachment; filename=\"llm-tools-export.csv\"")
			cw := csv.NewWriter(w)
			cw.Write([]string{"id", "llm_call_id", "timestamp", "tool_name", "tool_input_preview", "tool_result_preview", "risk_level", "flagged", "flag_reason"})
			for _, rec := range records {
				cw.Write([]string{
					fmt.Sprintf("%v", rec["id"]),
					fmt.Sprintf("%v", rec["llm_call_id"]),
					fmt.Sprintf("%v", rec["timestamp"]),
					fmt.Sprintf("%v", rec["tool_name"]),
					fmt.Sprintf("%v", rec["tool_input_preview"]),
					fmt.Sprintf("%v", rec["tool_result_preview"]),
					fmt.Sprintf("%v", rec["risk_level"]),
					fmt.Sprintf("%v", rec["flagged"]),
					fmt.Sprintf("%v", rec["flag_reason"]),
				})
			}
			cw.Flush()
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", "attachment; filename=\"llm-tools-export.json\"")
			json.NewEncoder(w).Encode(map[string]interface{}{"data": records, "total": len(records)})
		}
		return
	}

	// calls
	records, _, err := api.llmAuditor.QueryCalls("", "", from, to, 10000, 0)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"llm-calls-export.csv\"")
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "timestamp", "trace_id", "model", "request_tokens", "response_tokens", "total_tokens", "latency_ms", "status_code", "has_tool_use", "tool_count", "error_message", "canary_leaked", "budget_exceeded"})
		for _, rec := range records {
			cw.Write([]string{
				fmt.Sprintf("%v", rec["id"]),
				fmt.Sprintf("%v", rec["timestamp"]),
				fmt.Sprintf("%v", rec["trace_id"]),
				fmt.Sprintf("%v", rec["model"]),
				fmt.Sprintf("%v", rec["request_tokens"]),
				fmt.Sprintf("%v", rec["response_tokens"]),
				fmt.Sprintf("%v", rec["total_tokens"]),
				fmt.Sprintf("%v", rec["latency_ms"]),
				fmt.Sprintf("%v", rec["status_code"]),
				fmt.Sprintf("%v", rec["has_tool_use"]),
				fmt.Sprintf("%v", rec["tool_count"]),
				fmt.Sprintf("%v", rec["error_message"]),
				fmt.Sprintf("%v", rec["canary_leaked"]),
				fmt.Sprintf("%v", rec["budget_exceeded"]),
			})
		}
		cw.Flush()
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\"llm-calls-export.json\"")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": records, "total": len(records)})
	}
}

// ============================================================

// ============================================================
// v13.0 会话回放 API handlers
// ============================================================

// handleSessionReplayList GET /api/v1/sessions/replay — 会话列表（v14.0: 支持 ?tenant）
