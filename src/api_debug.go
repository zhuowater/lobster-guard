// api_debug.go — v32.0 全链路检测调试 + 规则重叠分析 API
package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LayerResult 单层检测结果
type LayerResult struct {
	Action       string        `json:"action"`
	MatchedRules []LayerMatch  `json:"matched_rules"`
	RuleCount    int           `json:"rule_count"`
	PatternCount int           `json:"pattern_count"`
	LatencyUs    int64         `json:"latency_us"`
}

// LayerMatch 单条命中
type LayerMatch struct {
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Category string `json:"category"`
	Action   string `json:"action"`
	Pattern  string `json:"pattern,omitempty"`
	Type     string `json:"type"` // keyword / regex
}

// OverlapEntry 重叠条目
type OverlapEntry struct {
	Pattern string   `json:"pattern"`
	Layers  []string `json:"layers"` // ["inbound", "llm_request", "llm_response", "outbound"]
	Rules   []string `json:"rules"`  // 对应的规则 ID
}

// handleDetectAllLayers POST /api/v1/debug/detect-all-layers — 全链路三层检测
func (api *ManagementAPI) handleDetectAllLayers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text     string `json:"text"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Text == "" {
		jsonResponse(w, 400, map[string]string{"error": "text is required"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}

	totalStart := time.Now()

	// === 1. 入站检测 ===
	inboundLayer := LayerResult{Action: "pass"}
	if api.inboundEngine != nil {
		t0 := time.Now()
		result := api.inboundEngine.DetectWithAppID(req.Text, "debug")
		globalResult := api.inboundEngine.DetectGlobalTemplates(req.Text)
		if globalResult.Action != "pass" {
			result = mergeDetectResults(result, globalResult)
		}
		if req.TenantID != "default" {
			tenantResult := api.inboundEngine.DetectTenantRules(req.TenantID, req.Text)
			if tenantResult.Action != "pass" {
				result = mergeDetectResults(result, tenantResult)
			}
		}
		inboundLayer.Action = result.Action
		inboundLayer.LatencyUs = time.Since(t0).Microseconds()
		for _, ruleName := range result.MatchedRules {
			inboundLayer.MatchedRules = append(inboundLayer.MatchedRules, LayerMatch{
				RuleID:   ruleName,
				RuleName: ruleName,
				Action:   result.Action,
				Type:     "keyword",
			})
		}
		// 统计规则数
		api.inboundEngine.mu.RLock()
		inboundLayer.RuleCount = len(api.inboundEngine.ruleConfigs)
		for _, cfg := range api.inboundEngine.ruleConfigs {
			inboundLayer.PatternCount += len(cfg.Patterns)
		}
		api.inboundEngine.mu.RUnlock()
	}

	// === 2. LLM 请求检测 ===
	llmReqLayer := LayerResult{Action: "pass"}
	if api.llmRuleEngine != nil {
		t0 := time.Now()
		matches := api.llmRuleEngine.CheckRequestWithTenant(req.Text, req.TenantID)
		llmReqLayer.LatencyUs = time.Since(t0).Microseconds()
		if len(matches) > 0 {
			action, _ := HighestPriorityAction(matches)
			llmReqLayer.Action = action
			for _, m := range matches {
				llmReqLayer.MatchedRules = append(llmReqLayer.MatchedRules, LayerMatch{
					RuleID:   m.RuleID,
					RuleName: m.RuleName,
					Category: m.Category,
					Action:   m.Action,
					Pattern:  m.Pattern,
					Type:     "keyword",
				})
			}
		}
		// 统计 request 方向规则
		api.llmRuleEngine.mu.RLock()
		for _, rule := range api.llmRuleEngine.rules {
			if rule.Direction == "request" || rule.Direction == "both" {
				llmReqLayer.RuleCount++
				llmReqLayer.PatternCount += len(rule.Patterns)
			}
		}
		api.llmRuleEngine.mu.RUnlock()
	}

	// === 3. LLM 响应检测 ===
	llmRespLayer := LayerResult{Action: "pass"}
	if api.llmRuleEngine != nil {
		t0 := time.Now()
		matches := api.llmRuleEngine.CheckResponseWithTenant(req.Text, req.TenantID)
		llmRespLayer.LatencyUs = time.Since(t0).Microseconds()
		if len(matches) > 0 {
			action, _ := HighestPriorityAction(matches)
			llmRespLayer.Action = action
			for _, m := range matches {
				llmRespLayer.MatchedRules = append(llmRespLayer.MatchedRules, LayerMatch{
					RuleID:   m.RuleID,
					RuleName: m.RuleName,
					Category: m.Category,
					Action:   m.Action,
					Pattern:  m.Pattern,
					Type:     "keyword",
				})
			}
		}
		api.llmRuleEngine.mu.RLock()
		for _, rule := range api.llmRuleEngine.rules {
			if rule.Direction == "response" || rule.Direction == "both" {
				llmRespLayer.RuleCount++
				llmRespLayer.PatternCount += len(rule.Patterns)
			}
		}
		api.llmRuleEngine.mu.RUnlock()
	}

	// === 4. 出站检测 ===
	outboundLayer := LayerResult{Action: "pass"}
	if api.outboundEngine != nil {
		t0 := time.Now()
		result := api.outboundEngine.Detect(req.Text)
		outboundLayer.Action = result.Action
		outboundLayer.LatencyUs = time.Since(t0).Microseconds()
		if result.Action != "pass" {
			outboundLayer.MatchedRules = append(outboundLayer.MatchedRules, LayerMatch{
				RuleName: result.RuleName,
				RuleID:   result.RuleName,
				Action:   result.Action,
				Type:     "regex",
			})
		}
		// 影子模式命中也列出
		for _, sr := range result.ShadowReasons {
			outboundLayer.MatchedRules = append(outboundLayer.MatchedRules, LayerMatch{
				RuleName: sr,
				RuleID:   sr,
				Action:   "shadow",
				Type:     "regex",
			})
		}
		obConfigs := api.outboundEngine.GetRuleConfigs()
		outboundLayer.RuleCount = len(obConfigs)
		for _, cfg := range obConfigs {
			outboundLayer.PatternCount += len(cfg.Patterns)
		}
	}

	// 判断总体 action
	overallAction := "pass"
	for _, a := range []string{inboundLayer.Action, llmReqLayer.Action, llmRespLayer.Action, outboundLayer.Action} {
		if a == "block" {
			overallAction = "block"
			break
		}
		if a == "warn" && overallAction == "pass" {
			overallAction = "warn"
		}
	}

	totalHits := len(inboundLayer.MatchedRules) + len(llmReqLayer.MatchedRules) + len(llmRespLayer.MatchedRules) + len(outboundLayer.MatchedRules)

	jsonResponse(w, 200, map[string]interface{}{
		"overall_action": overallAction,
		"total_hits":     totalHits,
		"total_latency_us": time.Since(totalStart).Microseconds(),
		"tenant_id":      req.TenantID,
		"layers": map[string]interface{}{
			"inbound":      inboundLayer,
			"llm_request":  llmReqLayer,
			"llm_response": llmRespLayer,
			"outbound":     outboundLayer,
		},
	})
}

// handleRuleOverlap GET /api/v1/debug/rule-overlap — 三层规则重叠分析
func (api *ManagementAPI) handleRuleOverlap(w http.ResponseWriter, r *http.Request) {
	// 收集各层 patterns
	type patternInfo struct {
		Pattern string
		RuleID  string
		Layer   string
	}
	var allPatterns []patternInfo

	// 入站 patterns（从原始配置获取）
	if api.inboundEngine != nil {
		api.inboundEngine.mu.RLock()
		for _, cfg := range api.inboundEngine.ruleConfigs {
			for _, p := range cfg.Patterns {
				allPatterns = append(allPatterns, patternInfo{
					Pattern: strings.ToLower(p),
					RuleID:  cfg.Name,
					Layer:   "inbound",
				})
			}
		}
		api.inboundEngine.mu.RUnlock()
	}

	// LLM patterns
	if api.llmRuleEngine != nil {
		api.llmRuleEngine.mu.RLock()
		for _, rule := range api.llmRuleEngine.rules {
			layer := "llm_" + rule.Direction
			if rule.Direction == "both" {
				layer = "llm_both"
			}
			for _, p := range rule.Patterns {
				allPatterns = append(allPatterns, patternInfo{
					Pattern: strings.ToLower(p),
					RuleID:  rule.ID,
					Layer:   layer,
				})
			}
		}
		api.llmRuleEngine.mu.RUnlock()
	}

	// 出站 patterns
	if api.outboundEngine != nil {
		obConfigs := api.outboundEngine.GetRuleConfigs()
		for _, cfg := range obConfigs {
			for _, p := range cfg.Patterns {
				allPatterns = append(allPatterns, patternInfo{
					Pattern: strings.ToLower(p),
					RuleID:  cfg.Name,
					Layer:   "outbound",
				})
			}
		}
	}

	// 构建 pattern → layers 映射
	type pKey struct {
		Pattern string
	}
	patternMap := make(map[string]*OverlapEntry)
	for _, pi := range allPatterns {
		key := pi.Pattern
		if entry, ok := patternMap[key]; ok {
			// 检查是否已有该层
			found := false
			for _, l := range entry.Layers {
				if l == pi.Layer {
					found = true
					break
				}
			}
			if !found {
				entry.Layers = append(entry.Layers, pi.Layer)
			}
			entry.Rules = append(entry.Rules, pi.RuleID+"@"+pi.Layer)
		} else {
			patternMap[key] = &OverlapEntry{
				Pattern: pi.Pattern,
				Layers:  []string{pi.Layer},
				Rules:   []string{pi.RuleID + "@" + pi.Layer},
			}
		}
	}

	// 筛选出跨层重叠的 patterns
	var overlaps []OverlapEntry
	for _, entry := range patternMap {
		if len(entry.Layers) >= 2 {
			sort.Strings(entry.Layers)
			sort.Strings(entry.Rules)
			overlaps = append(overlaps, *entry)
		}
	}
	sort.Slice(overlaps, func(i, j int) bool {
		if len(overlaps[i].Layers) != len(overlaps[j].Layers) {
			return len(overlaps[i].Layers) > len(overlaps[j].Layers)
		}
		return overlaps[i].Pattern < overlaps[j].Pattern
	})

	// 统计
	inboundCount := 0
	llmCount := 0
	outboundCount := 0
	for _, pi := range allPatterns {
		switch {
		case pi.Layer == "inbound":
			inboundCount++
		case strings.HasPrefix(pi.Layer, "llm"):
			llmCount++
		case pi.Layer == "outbound":
			outboundCount++
		}
	}

	// 唯一 pattern 数
	uniquePatterns := len(patternMap)

	jsonResponse(w, 200, map[string]interface{}{
		"summary": map[string]interface{}{
			"total_patterns":        len(allPatterns),
			"unique_patterns":       uniquePatterns,
			"overlap_count":         len(overlaps),
			"inbound_patterns":      inboundCount,
			"llm_patterns":          llmCount,
			"outbound_patterns":     outboundCount,
			"overlap_ratio_percent": float64(len(overlaps)) / float64(uniquePatterns) * 100,
		},
		"overlaps": overlaps,
		"recommendation": getOverlapRecommendation(len(overlaps), uniquePatterns),
	})
}

func getOverlapRecommendation(overlapCount, uniqueCount int) string {
	ratio := float64(overlapCount) / float64(uniqueCount) * 100
	switch {
	case ratio < 1:
		return "✅ 规则重叠率极低（<1%），三层检测互补良好，属于纵深防御设计，无需调整"
	case ratio < 5:
		return "🟡 存在少量重叠（<5%），可保持现状作为纵深防御，或考虑将重叠规则标记为 scope=both"
	case ratio < 15:
		return "🟠 重叠较多（5-15%），建议引入共享规则库，用 scope 字段管理适用层，减少维护成本"
	default:
		return "🔴 重叠严重（>15%），强烈建议合并为共享规则库，避免规则不一致和维护负担"
	}
}
