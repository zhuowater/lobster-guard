package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	appIDFilter := r.URL.Query().Get("app_id")
	senderIDFilter := r.URL.Query().Get("sender_id")
	var entries []RouteEntry
	if appIDFilter != "" {
		entries = api.routes.ListByApp(appIDFilter)
	} else {
		entries = api.routes.ListRoutes()
	}
	if entries == nil {
		entries = []RouteEntry{}
	}
	// sender_id 过滤（精确匹配）
	if senderIDFilter != "" {
		var filtered []RouteEntry
		for _, e := range entries {
			if e.SenderID == senderIDFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// 策略冲突检测：对每条路由，用用户信息匹配策略，看结果是否和当前绑定一致
	// 注意：默认(兜底)策略命中时不标记冲突 — 用户已有亲和路由说明已被精确分配，
	// 默认策略本来就是"没有其他匹配时才生效"，不应与已有亲和绑定冲突。
	enriched := make([]RouteEntryWithConflict, 0, len(entries))
	conflictCount := 0
	for _, e := range entries {
		ec := RouteEntryWithConflict{RouteEntry: e}
		if api.policyEng != nil {
			// 构造 UserInfo 用于策略匹配
			info := &UserInfo{
				SenderID:   e.SenderID,
				Name:       e.DisplayName,
				Email:      e.Email,
				Department: e.Department,
			}
			// 如果路由条目没有用户信息，尝试从缓存获取
			if info.Department == "" && info.Email == "" && api.userCache != nil {
				if cached := api.userCache.GetCached(e.SenderID); cached != nil {
					info = cached
				}
			}
			if _, policy, ok := api.policyEng.TestMatch(info, e.AppID); ok && policy != nil {
				ec.PolicyUpstream = policy.UpstreamID
				if policy.UpstreamID != e.UpstreamID {
					// 默认(兜底)策略不构成冲突 — 亲和路由优先级更高
					if !policy.Match.Default {
						ec.PolicyConflict = true
						ec.PolicyRule = api.describePolicyMatch(info, e.AppID)
						conflictCount++
					}
				}
			}
		}
		enriched = append(enriched, ec)
	}
	resp := map[string]interface{}{
		"routes": enriched,
		"total":  len(enriched),
	}
	if conflictCount > 0 {
		resp["conflict_count"] = conflictCount
		resp["conflict_warning"] = fmt.Sprintf("%d 条路由与策略规则冲突，下次请求时将自动迁移到策略指定的上游", conflictCount)
	}
	jsonResponse(w, 200, resp)
}

// describePolicyMatch 描述匹配到的策略规则（人类可读）
func (api *ManagementAPI) describePolicyMatch(info *UserInfo, appID string) string {
	if api.policyEng == nil || info == nil {
		return ""
	}
	api.policyEng.mu.RLock()
	defer api.policyEng.mu.RUnlock()
	for _, p := range api.policyEng.policies {
		if p.Match.Default {
			continue
		}
		if p.Match.Department != "" && containsDepartment(info.Department, p.Match.Department) {
			return fmt.Sprintf("department=%s → %s", p.Match.Department, p.UpstreamID)
		}
		if p.Match.EmailSuffix != "" && info.Email != "" && strings.HasSuffix(info.Email, p.Match.EmailSuffix) {
			return fmt.Sprintf("email_suffix=%s → %s", p.Match.EmailSuffix, p.UpstreamID)
		}
		if p.Match.Email != "" && info.Email == p.Match.Email {
			return fmt.Sprintf("email=%s → %s", p.Match.Email, p.UpstreamID)
		}
		if p.Match.AppID != "" && appID == p.Match.AppID {
			return fmt.Sprintf("app_id=%s → %s", p.Match.AppID, p.UpstreamID)
		}
	}
	// 命中了默认策略
	for _, p := range api.policyEng.policies {
		if p.Match.Default {
			return fmt.Sprintf("default → %s", p.UpstreamID)
		}
	}
	return ""
}

func (api *ManagementAPI) handleBindRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SenderID    string `json:"sender_id"`
		AppID       string `json:"app_id"`
		UpstreamID  string `json:"upstream_id"`
		Department  string `json:"department"`
		DisplayName string `json:"display_name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SenderID == "" || req.UpstreamID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id and upstream_id required"})
		return
	}
	// R2-004: 长度限制
	if len(req.SenderID) > 256 || len(req.AppID) > 256 || len(req.UpstreamID) > 256 {
		jsonResponse(w, 400, map[string]string{"error": "sender_id, app_id, upstream_id must be <= 256 characters"})
		return
	}
	// BUG-002 fix: validate upstream exists
	if _, ok := api.pool.GetUpstream(req.UpstreamID); !ok {
		jsonResponse(w, 400, map[string]string{"error": "upstream not found: " + req.UpstreamID})
		return
	}
	// BUG-004 fix: check old binding for user_count adjustment
	oldUpstream, hadOld := api.routes.Lookup(req.SenderID, req.AppID)
	if req.Department != "" || req.DisplayName != "" {
		api.routes.BindWithMeta(req.SenderID, req.AppID, req.UpstreamID, req.Department, req.DisplayName)
	} else {
		api.routes.Bind(req.SenderID, req.AppID, req.UpstreamID)
	}
	// BUG-004 fix: update user_count on old and new upstream
	if hadOld && oldUpstream != req.UpstreamID {
		api.pool.IncrUserCount(oldUpstream, -1)
	}
	if !hadOld || oldUpstream != req.UpstreamID {
		api.pool.IncrUserCount(req.UpstreamID, 1)
	}
	jsonResponse(w, 200, map[string]string{"status": "bound", "sender_id": req.SenderID, "app_id": req.AppID, "upstream_id": req.UpstreamID})
}

func (api *ManagementAPI) handleUnbindRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SenderID string `json:"sender_id"`
		AppID    string `json:"app_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SenderID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id required"})
		return
	}
	// BUG-004 fix: decrement user_count on old upstream before unbinding
	if oldUpstream, ok := api.routes.Lookup(req.SenderID, req.AppID); ok {
		api.pool.IncrUserCount(oldUpstream, -1)
	}
	api.routes.Unbind(req.SenderID, req.AppID)
	jsonResponse(w, 200, map[string]string{"status": "unbound", "sender_id": req.SenderID, "app_id": req.AppID})
}

func (api *ManagementAPI) handleMigrateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SenderID string `json:"sender_id"`
		AppID    string `json:"app_id"`
		From     string `json:"from"`
		To       string `json:"to"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SenderID == "" || req.To == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id and to required"})
		return
	}
	if api.routes.Migrate(req.SenderID, req.AppID, req.From, req.To) {
		api.pool.IncrUserCount(req.From, -1)
		api.pool.IncrUserCount(req.To, 1)
		jsonResponse(w, 200, map[string]interface{}{
			"status": "migrated", "sender_id": req.SenderID, "app_id": req.AppID, "from": req.From, "to": req.To,
		})
	} else {
		jsonResponse(w, 404, map[string]string{"error": "route not found or mismatch"})
	}
}

func (api *ManagementAPI) handleBatchBindRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID      string       `json:"app_id"`
		UpstreamID string       `json:"upstream_id"`
		Department string       `json:"department"`
		Entries    []RouteEntry `json:"entries"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.UpstreamID == "" {
		jsonResponse(w, 400, map[string]string{"error": "upstream_id required"})
		return
	}
	// R2-004: 长度限制
	if len(req.UpstreamID) > 256 || len(req.AppID) > 256 {
		jsonResponse(w, 400, map[string]string{"error": "upstream_id, app_id must be <= 256 characters"})
		return
	}
	// BUG-002 fix: validate upstream exists
	if _, ok := api.pool.GetUpstream(req.UpstreamID); !ok {
		jsonResponse(w, 400, map[string]string{"error": "upstream not found: " + req.UpstreamID})
		return
	}
	var bound int
	if len(req.Entries) > 0 {
		// 模式1: 按条目列表批量绑定
		entries := make([]RouteEntry, 0, len(req.Entries))
		// BUG-004 fix: track old upstreams for user_count adjustment
		oldUpstreamDeltas := make(map[string]int)
		newCount := 0
		for _, e := range req.Entries {
			if e.SenderID == "" { continue }
			appID := req.AppID
			if oldUID, ok := api.routes.Lookup(e.SenderID, appID); ok {
				if oldUID != req.UpstreamID {
					oldUpstreamDeltas[oldUID]--
					newCount++
				}
				// same upstream → no change
			} else {
				newCount++
			}
			entries = append(entries, RouteEntry{
				SenderID:    e.SenderID,
				AppID:       appID,
				UpstreamID:  req.UpstreamID,
				Department:  e.Department,
				DisplayName: e.DisplayName,
			})
		}
		api.routes.BindBatch(entries)
		// BUG-004 fix: adjust user_counts
		for uid, delta := range oldUpstreamDeltas {
			api.pool.IncrUserCount(uid, delta)
		}
		if newCount > 0 {
			api.pool.IncrUserCount(req.UpstreamID, newCount)
		}
		bound = len(entries)
	} else if req.Department != "" {
		// 模式2: 按部门批量分配
		existing := api.routes.ListByDepartment(req.Department)
		entries := make([]RouteEntry, 0, len(existing))
		oldUpstreamDeltas := make(map[string]int)
		newCount := 0
		for _, e := range existing {
			if req.AppID != "" && e.AppID != req.AppID { continue }
			appID := func() string { if req.AppID != "" { return req.AppID }; return e.AppID }()
			if oldUID, ok := api.routes.Lookup(e.SenderID, appID); ok {
				if oldUID != req.UpstreamID {
					oldUpstreamDeltas[oldUID]--
					newCount++
				}
			} else {
				newCount++
			}
			entries = append(entries, RouteEntry{
				SenderID:    e.SenderID,
				AppID:       appID,
				UpstreamID:  req.UpstreamID,
				Department:  e.Department,
				DisplayName: e.DisplayName,
			})
		}
		api.routes.BindBatch(entries)
		for uid, delta := range oldUpstreamDeltas {
			api.pool.IncrUserCount(uid, delta)
		}
		if newCount > 0 {
			api.pool.IncrUserCount(req.UpstreamID, newCount)
		}
		bound = len(entries)
	} else {
		jsonResponse(w, 400, map[string]string{"error": "entries or department required"})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "batch_bound", "count": bound})
}

// handleBatchUnbindRoute POST /api/v1/routes/batch-unbind — 批量解绑路由
func (api *ManagementAPI) handleBatchUnbindRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []struct {
			SenderID string `json:"sender_id"`
			AppID    string `json:"app_id"`
		} `json:"entries"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Entries) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "entries required"})
		return
	}
	// BUG-004 fix: decrement user_count for each unbound route
	upstreamDeltas := make(map[string]int)
	count := 0
	for _, e := range req.Entries {
		if e.SenderID == "" {
			continue
		}
		if oldUID, ok := api.routes.Lookup(e.SenderID, e.AppID); ok {
			upstreamDeltas[oldUID]--
		}
		api.routes.Unbind(e.SenderID, e.AppID)
		count++
	}
	for uid, delta := range upstreamDeltas {
		api.pool.IncrUserCount(uid, delta)
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "batch_unbound", "count": count})
}

// handleBatchMigrateRoute POST /api/v1/routes/batch-migrate — 批量迁移路由到新上游
func (api *ManagementAPI) handleBatchMigrateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []struct {
			SenderID string `json:"sender_id"`
			AppID    string `json:"app_id"`
		} `json:"entries"`
		To string `json:"to"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.To == "" || len(req.Entries) == 0 {
		jsonResponse(w, 400, map[string]string{"error": "entries and to required"})
		return
	}
	migrated := 0
	for _, e := range req.Entries {
		if e.SenderID == "" {
			continue
		}
		if api.routes.Migrate(e.SenderID, e.AppID, "", req.To) {
			migrated++
		}
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "batch_migrated", "count": migrated, "to": req.To})
}

func (api *ManagementAPI) handleRouteStats(w http.ResponseWriter, r *http.Request) {
	stats := api.routes.Stats()
	jsonResponse(w, 200, stats)
}

func (api *ManagementAPI) handleListRoutePolicies(w http.ResponseWriter, r *http.Request) {
	if api.policyEng == nil {
		jsonResponse(w, 200, map[string]interface{}{"policies": []interface{}{}, "total": 0})
		return
	}
	policies := api.policyEng.ListPolicies()
	jsonResponse(w, 200, map[string]interface{}{"policies": policies, "total": len(policies)})
}

// handleTestRoutePolicy POST /api/v1/route-policies/test — 测试策略匹配
func (api *ManagementAPI) handleTestRoutePolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SenderID string `json:"sender_id"`
		AppID    string `json:"app_id"`
		Email    string `json:"email"`
		Department string `json:"department"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}

	// 构建 UserInfo（优先用请求中的字段，其次查缓存）
	var info *UserInfo
	if req.Email != "" || req.Department != "" {
		info = &UserInfo{
			SenderID:   req.SenderID,
			Email:      req.Email,
			Department: req.Department,
		}
	} else if api.userCache != nil && req.SenderID != "" {
		info = api.userCache.GetCached(req.SenderID)
	}
	if info == nil {
		// info 为 nil 时仍然检查 default 策略
		if api.policyEng != nil {
			idx, policy, matched := api.policyEng.TestMatch(nil, req.AppID)
			if matched {
				jsonResponse(w, 200, map[string]interface{}{
					"matched":      true,
					"policy_index": idx,
					"policy":       policy,
					"upstream_id":  policy.UpstreamID,
					"user_info":    nil,
					"note":         "matched via default policy (no user info needed)",
				})
				return
			}
		}
		jsonResponse(w, 200, map[string]interface{}{
			"matched":  false,
			"message":  "no user info available and no default policy configured",
		})
		return
	}

	if api.policyEng == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"matched":  false,
			"message":  "no route policies configured",
		})
		return
	}

	idx, policy, matched := api.policyEng.TestMatch(info, req.AppID)
	if !matched {
		jsonResponse(w, 200, map[string]interface{}{
			"matched":   false,
			"user_info": info,
		})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"matched":      true,
		"policy_index": idx,
		"policy":       policy,
		"upstream_id":  policy.UpstreamID,
		"user_info":    info,
	})
}

// saveRoutePolicies 将策略列表写回 config.yaml（读取→修改 route_policies 字段→写回）
func (api *ManagementAPI) saveRoutePolicies(policies []RoutePolicyConfig) error {
	// 转换为 []interface{} 以保证 yaml marshal 正确
	policyList := make([]interface{}, len(policies))
	for i, p := range policies {
		m := map[string]interface{}{}
		match := map[string]interface{}{}
		if p.Match.Department != "" {
			match["department"] = p.Match.Department
		}
		if p.Match.EmailSuffix != "" {
			match["email_suffix"] = p.Match.EmailSuffix
		}
		if p.Match.Email != "" {
			match["email"] = p.Match.Email
		}
		if p.Match.AppID != "" {
			match["app_id"] = p.Match.AppID
		}
		if p.Match.Default {
			match["default"] = true
		}
		m["match"] = match
		m["upstream_id"] = p.UpstreamID
		if p.FixedResponse != nil {
			fr := map[string]interface{}{
				"enabled":      p.FixedResponse.Enabled,
				"status_code":  p.FixedResponse.StatusCode,
				"content_type": p.FixedResponse.ContentType,
				"body":         p.FixedResponse.Body,
			}
			if len(p.FixedResponse.Headers) > 0 {
				fr["headers"] = p.FixedResponse.Headers
			}
			m["fixed_response"] = fr
		}
		policyList[i] = m
	}
	if err := api.configPersistence().ReplaceSection("route_policies", policyList); err != nil {
		return fmt.Errorf("写入 route_policies 失败: %w", err)
	}
	log.Printf("[策略路由] 已保存 %d 条策略到 %s", len(policies), api.cfgPath)
	return nil
}

// respondPolicies 返回更新后的策略列表（CRUD 共用）
func (api *ManagementAPI) respondPolicies(w http.ResponseWriter, policies []RoutePolicyConfig) {
	jsonResponse(w, 200, map[string]interface{}{"policies": policies, "total": len(policies)})
}

// handleReorderRoutePolicies POST /api/v1/route-policies/reorder — 原子重排策略顺序
func (api *ManagementAPI) handleReorderRoutePolicies(w http.ResponseWriter, r *http.Request) {
	if api.policyEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "route policy engine not initialized"})
		return
	}

	var req struct {
		Policies []RoutePolicyConfig `json:"policies"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	for i, p := range req.Policies {
		hasFixedResponse := p.FixedResponse != nil && p.FixedResponse.Enabled
		// default 策略允许 upstream_id 为空（走全局默认上游）
		if p.UpstreamID == "" && !hasFixedResponse && !p.Match.Default {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("policy #%d upstream_id is required (unless fixed_response is enabled or match is default)", i)})
			return
		}
		if !p.Match.Default && p.Match.Department == "" && p.Match.EmailSuffix == "" && p.Match.Email == "" && p.Match.AppID == "" {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("policy #%d match conditions cannot be empty", i)})
			return
		}
	}

	api.policyEng.SetPolicies(req.Policies)
	if err := api.saveRoutePolicies(req.Policies); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[策略路由] 已原子重排 %d 条策略", len(req.Policies))
	migrated := api.reevaluateAllRoutes()
	resp := map[string]interface{}{"policies": req.Policies, "total": len(req.Policies)}
	if migrated > 0 {
		resp["routes_migrated"] = migrated
	}
	jsonResponse(w, 200, resp)
}

// D-004: reevaluateAllRoutes 策略变更后重评估所有路由绑定
func (api *ManagementAPI) reevaluateAllRoutes() int {
	if api.policyEng == nil || api.userCache == nil {
		return 0
	}
	routes := api.routes.ListRoutes()
	migrated := 0
	for _, r := range routes {
		info := api.userCache.GetCached(r.SenderID)
		if info == nil {
			continue
		}
		if pUID, ok := api.policyEng.Match(info, r.AppID); ok && pUID != "" && pUID != r.UpstreamID {
			if _, exists := api.pool.GetUpstream(pUID); exists {
				if AtomicMigrate(api.routes, api.pool, r.SenderID, r.AppID, r.UpstreamID, pUID) {
					migrated++
				}
			}
		}
	}
	if migrated > 0 {
		log.Printf("[策略路由] 策略变更触发重评估: %d 条路由迁移", migrated)
	}
	return migrated
}

// handleCreateRoutePolicy POST /api/v1/route-policies — 新增策略
func (api *ManagementAPI) handleCreateRoutePolicy(w http.ResponseWriter, r *http.Request) {
	var req RoutePolicyConfig
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	// D-007 fix: upstream_id 不能为空（除非配置了 fixed_response）
	hasFixedResponse := req.FixedResponse != nil && req.FixedResponse.Enabled
	if req.UpstreamID == "" && !hasFixedResponse {
		jsonResponse(w, 400, map[string]string{"error": "upstream_id is required (unless fixed_response is enabled)"})
		return
	}
	// R2-004: 长度限制
	if len(req.UpstreamID) > 256 {
		jsonResponse(w, 400, map[string]string{"error": "upstream_id must be <= 256 characters"})
		return
	}
	// R2-002 fix: match 条件不能全空
	if !req.Match.Default && req.Match.Department == "" && req.Match.EmailSuffix == "" && req.Match.Email == "" && req.Match.AppID == "" {
		jsonResponse(w, 400, map[string]string{"error": "match conditions cannot be empty, set at least one field or use default:true"})
		return
	}
	if api.policyEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "route policy engine not initialized"})
		return
	}
	// BUG-001 fix: check for duplicate match conditions
	policies := api.policyEng.ListPolicies()
	for _, p := range policies {
		if p.Match.Department == req.Match.Department &&
			p.Match.EmailSuffix == req.Match.EmailSuffix &&
			p.Match.Email == req.Match.Email &&
			p.Match.AppID == req.Match.AppID &&
			p.Match.Default == req.Match.Default {
			jsonResponse(w, 409, map[string]string{"error": "policy with same match conditions already exists"})
			return
		}
	}
	// R2-005: 策略可指向不存在上游 → warn 而非 block
	upstreamWarning := ""
	if _, exists := api.pool.GetUpstream(req.UpstreamID); !exists {
		upstreamWarning = "upstream not currently registered: " + req.UpstreamID
		log.Printf("[策略路由] ⚠️ 新增策略指向未注册上游: %s", req.UpstreamID)
	}
	// 追加
	policies = append(policies, req)
	// 更新内存
	api.policyEng.SetPolicies(policies)
	// 写回文件
	if err := api.saveRoutePolicies(policies); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[策略路由] 新增策略: upstream_id=%s", req.UpstreamID)
	// D-004: 策略变更触发全量路由重评估
	migrated := api.reevaluateAllRoutes()
	resp := map[string]interface{}{"policies": policies, "total": len(policies)}
	if migrated > 0 {
		resp["routes_migrated"] = migrated
	}
	if upstreamWarning != "" {
		resp["warning"] = upstreamWarning
	}
	jsonResponse(w, 200, resp)
	return
}

// handleUpdateRoutePolicy PUT /api/v1/route-policies/:index — 修改策略
func (api *ManagementAPI) handleUpdateRoutePolicy(w http.ResponseWriter, r *http.Request) {
	// 解析 index
	idxStr := strings.TrimPrefix(r.URL.Path, "/api/v1/route-policies/")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid index: " + idxStr})
		return
	}
	var req RoutePolicyConfig
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	// D-007 fix: upstream_id 不能为空（除非配置了 fixed_response）
	hasFixedResponse := req.FixedResponse != nil && req.FixedResponse.Enabled
	if req.UpstreamID == "" && !hasFixedResponse {
		jsonResponse(w, 400, map[string]string{"error": "upstream_id is required (unless fixed_response is enabled)"})
		return
	}
	if api.policyEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "route policy engine not initialized"})
		return
	}
	policies := api.policyEng.ListPolicies()
	if idx < 0 || idx >= len(policies) {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("policy index %d out of range (total %d)", idx, len(policies))})
		return
	}
	policies[idx] = req
	api.policyEng.SetPolicies(policies)
	if err := api.saveRoutePolicies(policies); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[策略路由] 修改策略 #%d: upstream_id=%s", idx, req.UpstreamID)
	// D-004: 策略变更触发全量路由重评估
	migrated := api.reevaluateAllRoutes()
	resp := map[string]interface{}{"policies": policies, "total": len(policies)}
	if migrated > 0 {
		resp["routes_migrated"] = migrated
	}
	jsonResponse(w, 200, resp)
}

// handleDeleteRoutePolicy DELETE /api/v1/route-policies/:index — 删除策略
func (api *ManagementAPI) handleDeleteRoutePolicy(w http.ResponseWriter, r *http.Request) {
	idxStr := strings.TrimPrefix(r.URL.Path, "/api/v1/route-policies/")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid index: " + idxStr})
		return
	}
	if api.policyEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "route policy engine not initialized"})
		return
	}
	policies := api.policyEng.ListPolicies()
	if idx < 0 || idx >= len(policies) {
		jsonResponse(w, 404, map[string]string{"error": fmt.Sprintf("policy index %d out of range (total %d)", idx, len(policies))})
		return
	}
	policies = append(policies[:idx], policies[idx+1:]...)
	api.policyEng.SetPolicies(policies)
	if err := api.saveRoutePolicies(policies); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[策略路由] 删除策略 #%d", idx)
	// D-004: 策略变更触发全量路由重评估
	migrated := api.reevaluateAllRoutes()
	resp := map[string]interface{}{"policies": policies, "total": len(policies)}
	if migrated > 0 {
		resp["routes_migrated"] = migrated
	}
	jsonResponse(w, 200, resp)
}

// ============================================================
// v3.11 规则绑定 API
// ============================================================

// handleListRuleBindings GET /api/v1/rule-bindings — 查看规则绑定关系
