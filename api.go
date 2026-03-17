// api.go — ManagementAPI、所有 HTTP handler
// lobster-guard v4.0 代码拆分
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ============================================================
// 管理 API v2.0
// ============================================================

type ManagementAPI struct {
	pool           *UpstreamPool
	routes         *RouteTable
	logger         *AuditLogger
	inboundEngine  *RuleEngine         // v3.5 入站规则引擎引用
	outboundEngine *OutboundRuleEngine
	cfg            *Config
	cfgPath        string
	managementToken string
	registrationToken string
	inbound        *InboundProxy
	channel        ChannelPlugin       // v3.4 通道引用
	metrics        *MetricsCollector   // v3.4 指标采集器
	ruleHits       *RuleHitStats       // v3.6 规则命中统计
	userCache      *UserInfoCache      // v3.9 用户信息缓存
	policyEng      *RoutePolicyEngine  // v3.9 路由策略引擎
	alertNotifier  *AlertNotifier      // v3.10 告警通知器
	wsProxy        *WSProxyManager     // v4.1 WebSocket 代理管理器
	store          Store               // v4.2 存储抽象层
	shutdownMgr    *ShutdownManager    // v4.2 关闭管理器
	realtime       *RealtimeMetrics    // v5.0 实时监控
	// v5.1 智能检测
	sessionDetector *SessionDetector   // v5.1 会话检测器
	llmDetector     *LLMDetector       // v5.1 LLM 检测器
	detectCache     *DetectCache       // v5.1 检测缓存
}

func NewManagementAPI(cfg *Config, cfgPath string, pool *UpstreamPool, routes *RouteTable, logger *AuditLogger, inboundEngine *RuleEngine, outboundEngine *OutboundRuleEngine, inbound *InboundProxy, channel ChannelPlugin, metrics *MetricsCollector, ruleHits *RuleHitStats, userCache *UserInfoCache, policyEng *RoutePolicyEngine, alertNotifier *AlertNotifier, wsProxy *WSProxyManager, store Store, shutdownMgr *ShutdownManager, realtime *RealtimeMetrics) *ManagementAPI {
	return &ManagementAPI{
		pool: pool, routes: routes, logger: logger,
		inboundEngine: inboundEngine, outboundEngine: outboundEngine,
		cfg: cfg, cfgPath: cfgPath,
		managementToken: cfg.ManagementToken, registrationToken: cfg.RegistrationToken,
		inbound: inbound, channel: channel, metrics: metrics, ruleHits: ruleHits,
		userCache: userCache, policyEng: policyEng, alertNotifier: alertNotifier,
		wsProxy: wsProxy, store: store, shutdownMgr: shutdownMgr, realtime: realtime,
	}
}

func (api *ManagementAPI) checkManagementAuth(r *http.Request) bool {
	if api.managementToken == "" { return true }
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+api.managementToken
}

func (api *ManagementAPI) checkRegistrationAuth(r *http.Request) bool {
	if api.registrationToken == "" { return true }
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+api.registrationToken
}

func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func (api *ManagementAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	// Dashboard 静态文件（无需鉴权，页面内输入 Token）
	if path == "/" || path == "/dashboard" || strings.HasPrefix(path, "/assets/") {
		getDashboardHandler().ServeHTTP(w, r)
		return
	}

	// 健康检查（无需鉴权）
	if path == "/healthz" {
		api.handleHealthz(w, r)
		return
	}

	// Prometheus 指标（默认无需鉴权）
	if path == "/metrics" {
		if api.metrics != nil {
			api.handleMetrics(w, r)
		} else {
			w.WriteHeader(404)
			w.Write([]byte("metrics disabled"))
		}
		return
	}

	// 服务注册相关（使用 registration token）
	if strings.HasPrefix(path, "/api/v1/register") || strings.HasPrefix(path, "/api/v1/heartbeat") || strings.HasPrefix(path, "/api/v1/deregister") {
		if !api.checkRegistrationAuth(r) {
			jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		switch {
		case path == "/api/v1/register" && method == "POST":
			api.handleRegister(w, r)
		case path == "/api/v1/heartbeat" && method == "POST":
			api.handleHeartbeat(w, r)
		case path == "/api/v1/deregister" && method == "POST":
			api.handleDeregister(w, r)
		default:
			w.WriteHeader(404)
		}
		return
	}

	// 管理接口（使用 management token）
	if !api.checkManagementAuth(r) {
		jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
		return
	}

	switch {
	case path == "/api/v1/upstreams" && method == "GET":
		api.handleListUpstreams(w, r)
	case path == "/api/v1/routes" && method == "GET":
		api.handleListRoutes(w, r)
	case path == "/api/v1/routes/bind" && method == "POST":
		api.handleBindRoute(w, r)
	case path == "/api/v1/routes/unbind" && method == "POST":
		api.handleUnbindRoute(w, r)
	case path == "/api/v1/routes/migrate" && method == "POST":
		api.handleMigrateRoute(w, r)
	case path == "/api/v1/routes/batch-bind" && method == "POST":
		api.handleBatchBindRoute(w, r)
	case path == "/api/v1/routes/stats" && method == "GET":
		api.handleRouteStats(w, r)
	case path == "/api/v1/rules/reload" && method == "POST":
		api.handleReloadRules(w, r)
	case path == "/api/v1/inbound-rules" && method == "GET":
		api.handleListInboundRules(w, r)
	case path == "/api/v1/inbound-rules/reload" && method == "POST":
		api.handleReloadInboundRules(w, r)
	case path == "/api/v1/outbound-rules" && method == "GET":
		api.handleListOutboundRules(w, r)
	case path == "/api/v1/audit/logs" && method == "GET":
		api.handleAuditLogs(w, r)
	case path == "/api/v1/audit/export" && method == "GET":
		api.handleAuditExport(w, r)
	case path == "/api/v1/audit/cleanup" && method == "POST":
		api.handleAuditCleanup(w, r)
	case path == "/api/v1/audit/stats" && method == "GET":
		api.handleAuditStats(w, r)
	case path == "/api/v1/audit/timeline" && method == "GET":
		api.handleAuditTimeline(w, r)
	case path == "/api/v1/audit/archives" && method == "GET":
		api.handleAuditArchives(w, r)
	case strings.HasPrefix(path, "/api/v1/audit/archives/") && method == "GET":
		api.handleAuditArchiveDownload(w, r)
	case path == "/api/v1/audit/archive" && method == "POST":
		api.handleAuditArchiveNow(w, r)
	case path == "/api/v1/stats" && method == "GET":
		api.handleStats(w, r)
	case path == "/api/v1/rate-limit/stats" && method == "GET":
		api.handleRateLimitStats(w, r)
	case path == "/api/v1/rate-limit/reset" && method == "POST":
		api.handleRateLimitReset(w, r)
	case path == "/api/v1/metrics/realtime" && method == "GET":
		api.handleRealtimeMetrics(w, r)
	case path == "/api/v1/rules/hits" && method == "GET":
		api.handleRuleHits(w, r)
	case path == "/api/v1/rules/hits/reset" && method == "POST":
		api.handleRuleHitsReset(w, r)
	// v3.9 用户信息 API
	case path == "/api/v1/users" && method == "GET":
		api.handleListUsers(w, r)
	case path == "/api/v1/users/refresh-all" && method == "POST":
		api.handleRefreshAllUsers(w, r)
	case strings.HasPrefix(path, "/api/v1/users/") && strings.HasSuffix(path, "/refresh") && method == "POST":
		api.handleRefreshUser(w, r)
	case strings.HasPrefix(path, "/api/v1/users/") && method == "GET":
		api.handleGetUser(w, r)
	case path == "/api/v1/route-policies" && method == "GET":
		api.handleListRoutePolicies(w, r)
	case path == "/api/v1/route-policies/test" && method == "POST":
		api.handleTestRoutePolicy(w, r)
	// v3.11 规则绑定 API
	case path == "/api/v1/rule-bindings" && method == "GET":
		api.handleListRuleBindings(w, r)
	case path == "/api/v1/rule-bindings/test" && method == "POST":
		api.handleTestRuleBindings(w, r)
	// v4.1 WebSocket 连接状态 API
	case path == "/api/v1/ws/connections" && method == "GET":
		if api.wsProxy != nil {
			api.wsProxy.HandleWSConnectionsAPI(w, r)
		} else {
			jsonResponse(w, 200, map[string]interface{}{"connections": []interface{}{}, "active": 0, "total": 0})
		}
	// v4.2 备份管理 API
	case path == "/api/v1/backup" && method == "POST":
		api.handleCreateBackup(w, r)
	case path == "/api/v1/backups" && method == "GET":
		api.handleListBackups(w, r)
	case strings.HasPrefix(path, "/api/v1/backups/") && method == "DELETE":
		api.handleDeleteBackup(w, r)
	// v5.1 智能检测 API
	case path == "/api/v1/rule-templates" && method == "GET":
		api.handleListRuleTemplates(w, r)
	case path == "/api/v1/sessions/risks" && method == "GET":
		api.handleSessionRisks(w, r)
	case path == "/api/v1/sessions/risks/reset" && method == "POST":
		api.handleSessionRisksReset(w, r)
	default:
		w.WriteHeader(404)
	}
}

func (api *ManagementAPI) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// v4.2: 关闭过程中返回 503
	if api.shutdownMgr != nil && api.shutdownMgr.IsShuttingDown() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "shutting_down",
		})
		return
	}

	// v4.2: 增强型健康检查
	healthResult := PerformHealthChecks(api.store, api.pool, api.cfg.DBPath)

	// 同时保留原有的详细信息
	upstreams := api.pool.ListUpstreams()
	healthyCount := 0
	upstreamList := []map[string]interface{}{}
	for _, up := range upstreams {
		if up.Healthy { healthyCount++ }
		upstreamList = append(upstreamList, map[string]interface{}{
			"id": up.ID, "address": up.Address, "port": up.Port,
			"healthy": up.Healthy, "user_count": up.UserCount, "static": up.Static,
			"last_heartbeat": up.LastHeartbeat.Format(time.RFC3339),
		})
	}
	result := map[string]interface{}{
		"status":  healthResult.Status,
		"version": AppVersion,
		"uptime":  time.Since(startTime).String(),
		"mode":    api.inbound.mode,
		"checks":  healthResult.Checks,
		"upstreams": map[string]interface{}{
			"total": len(upstreams), "healthy": healthyCount, "list": upstreamList,
		},
		"routes": map[string]interface{}{"total": api.routes.Count()},
		"audit":  api.logger.Stats(),
	}
	// v3.5 入站规则信息
	if api.inboundEngine != nil {
		rv := api.inboundEngine.Version()
		inboundRulesInfo := map[string]interface{}{
			"version":       rv.Version,
			"source":        rv.Source,
			"rule_count":    rv.RuleCount,
			"pattern_count": rv.PatternCount,
			"loaded_at":     rv.LoadedAt.Format(time.RFC3339),
		}
		// v3.6 添加 total_hits
		if api.ruleHits != nil {
			inboundRulesInfo["total_hits"] = api.ruleHits.TotalHits()
		}
		result["inbound_rules"] = inboundRulesInfo
	}
	// v3.5 出站规则信息
	if api.outboundEngine != nil {
		api.outboundEngine.mu.RLock()
		outRuleCount := len(api.outboundEngine.rules)
		api.outboundEngine.mu.RUnlock()
		outboundRulesInfo := map[string]interface{}{
			"rule_count": outRuleCount,
		}
		// v3.6 出站命中数 — 从 ruleHits 中统计出站规则的命中总数
		// 注意：ruleHits 是入站和出站共享的，这里简单返回总数
		// 如果需要区分，可以在未来使用前缀区分
		if api.ruleHits != nil {
			// 统计出站规则的命中总数
			api.outboundEngine.mu.RLock()
			var outboundHits int64
			hits := api.ruleHits.Get()
			for _, rule := range api.outboundEngine.rules {
				if h, ok := hits[rule.Name]; ok {
					outboundHits += h
				}
			}
			api.outboundEngine.mu.RUnlock()
			outboundRulesInfo["total_hits"] = outboundHits
		}
		result["outbound_rules"] = outboundRulesInfo
	}
	if api.inbound.mode == "bridge" && api.inbound.bridge != nil {
		bs := api.inbound.bridge.Status()
		bridgeInfo := map[string]interface{}{
			"connected":     bs.Connected,
			"reconnects":    bs.Reconnects,
			"message_count": bs.MessageCount,
		}
		if !bs.ConnectedAt.IsZero() {
			bridgeInfo["connected_at"] = bs.ConnectedAt.Format(time.RFC3339)
		}
		if !bs.LastMessage.IsZero() {
			bridgeInfo["last_message"] = bs.LastMessage.Format(time.RFC3339)
		}
		if bs.LastError != "" {
			bridgeInfo["last_error"] = bs.LastError
		}
		result["bridge"] = bridgeInfo
	}
	// Rate limiter info
	if api.inbound.limiter != nil {
		stats := api.inbound.limiter.Stats()
		result["rate_limiter"] = map[string]interface{}{
			"enabled":            true,
			"global_rps":         api.cfg.RateLimit.GlobalRPS,
			"per_sender_rps":     api.cfg.RateLimit.PerSenderRPS,
			"total_allowed":      stats.TotalAllowed,
			"total_limited":      stats.TotalLimited,
			"limit_rate_percent": stats.LimitRate,
		}
	} else {
		result["rate_limiter"] = map[string]interface{}{"enabled": false}
	}
	jsonResponse(w, 200, result)
}

func (api *ManagementAPI) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      string            `json:"id"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Tags    map[string]string `json:"tags"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := api.pool.Register(req.ID, req.Address, req.Port, req.Tags); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "registered",
		"heartbeat_interval": fmt.Sprintf("%ds", api.cfg.HeartbeatIntervalSec),
		"heartbeat_path": "/api/v1/heartbeat",
	})
}

func (api *ManagementAPI) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string                 `json:"id"`
		Load map[string]interface{} `json:"load"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	userCount, err := api.pool.Heartbeat(req.ID, req.Load)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "ok", "user_count": userCount})
}

func (api *ManagementAPI) handleDeregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == "" {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	api.pool.Deregister(req.ID)
	jsonResponse(w, 200, map[string]string{"status": "deregistered"})
}

func (api *ManagementAPI) handleListUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams := api.pool.ListUpstreams()
	totalUsers := 0
	healthyCount := 0
	list := []map[string]interface{}{}
	for _, up := range upstreams {
		totalUsers += up.UserCount
		if up.Healthy { healthyCount++ }
		list = append(list, map[string]interface{}{
			"id": up.ID, "address": up.Address, "port": up.Port,
			"healthy": up.Healthy, "user_count": up.UserCount, "static": up.Static,
			"last_heartbeat": up.LastHeartbeat.Format(time.RFC3339),
			"tags": up.Tags, "load": up.Load,
		})
	}
	jsonResponse(w, 200, map[string]interface{}{
		"upstreams": list, "total": len(upstreams),
		"healthy": healthyCount, "total_users": totalUsers,
	})
}

func (api *ManagementAPI) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	appIDFilter := r.URL.Query().Get("app_id")
	var entries []RouteEntry
	if appIDFilter != "" {
		entries = api.routes.ListByApp(appIDFilter)
	} else {
		entries = api.routes.ListRoutes()
	}
	if entries == nil {
		entries = []RouteEntry{}
	}
	jsonResponse(w, 200, map[string]interface{}{"routes": entries, "total": len(entries)})
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
	if req.Department != "" || req.DisplayName != "" {
		api.routes.BindWithMeta(req.SenderID, req.AppID, req.UpstreamID, req.Department, req.DisplayName)
	} else {
		api.routes.Bind(req.SenderID, req.AppID, req.UpstreamID)
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
	var bound int
	if len(req.Entries) > 0 {
		// 模式1: 按条目列表批量绑定
		entries := make([]RouteEntry, 0, len(req.Entries))
		for _, e := range req.Entries {
			if e.SenderID == "" { continue }
			entries = append(entries, RouteEntry{
				SenderID:    e.SenderID,
				AppID:       req.AppID,
				UpstreamID:  req.UpstreamID,
				Department:  e.Department,
				DisplayName: e.DisplayName,
			})
		}
		api.routes.BindBatch(entries)
		bound = len(entries)
	} else if req.Department != "" {
		// 模式2: 按部门批量分配
		existing := api.routes.ListByDepartment(req.Department)
		entries := make([]RouteEntry, 0, len(existing))
		for _, e := range existing {
			if req.AppID != "" && e.AppID != req.AppID { continue }
			entries = append(entries, RouteEntry{
				SenderID:    e.SenderID,
				AppID:       func() string { if req.AppID != "" { return req.AppID }; return e.AppID }(),
				UpstreamID:  req.UpstreamID,
				Department:  e.Department,
				DisplayName: e.DisplayName,
			})
		}
		api.routes.BindBatch(entries)
		bound = len(entries)
	} else {
		jsonResponse(w, 400, map[string]string{"error": "entries or department required"})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "batch_bound", "count": bound})
}

func (api *ManagementAPI) handleRouteStats(w http.ResponseWriter, r *http.Request) {
	stats := api.routes.Stats()
	jsonResponse(w, 200, stats)
}

func (api *ManagementAPI) handleReloadRules(w http.ResponseWriter, r *http.Request) {
	newCfg, err := loadConfig(api.cfgPath)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": "reload failed: " + err.Error()})
		return
	}
	api.outboundEngine.Reload(newCfg.OutboundRules)
	jsonResponse(w, 200, map[string]string{"status": "reloaded"})
}

func (api *ManagementAPI) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	direction := r.URL.Query().Get("direction")
	action := r.URL.Query().Get("action")
	senderID := r.URL.Query().Get("sender_id")
	appID := r.URL.Query().Get("app_id")
	q := r.URL.Query().Get("q")
	traceID := r.URL.Query().Get("trace_id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil { limit = n }
	}
	logs, err := api.logger.QueryLogsExTrace(direction, action, senderID, appID, q, traceID, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"logs": logs, "total": len(logs)})
}

// handleAuditExport GET /api/v1/audit/export — 导出审计日志为 CSV 或 JSON（v3.10）
func (api *ManagementAPI) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		jsonResponse(w, 400, map[string]string{"error": "format must be 'csv' or 'json'"})
		return
	}
	direction := r.URL.Query().Get("direction")
	action := r.URL.Query().Get("action")
	senderID := r.URL.Query().Get("sender_id")
	appID := r.URL.Query().Get("app_id")
	q := r.URL.Query().Get("q")
	limit := 1000
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil { limit = n }
	}
	if limit > 10000 { limit = 10000 }

	logs, err := api.logger.QueryLogsEx(direction, action, senderID, appID, q, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")
		w.WriteHeader(200)
		cw := csv.NewWriter(w)
		// 写表头
		cw.Write([]string{"id", "timestamp", "direction", "sender_id", "action", "reason", "content_preview", "latency_ms", "upstream_id", "app_id"})
		for _, log := range logs {
			cw.Write([]string{
				fmt.Sprintf("%v", log["id"]),
				fmt.Sprintf("%v", log["timestamp"]),
				fmt.Sprintf("%v", log["direction"]),
				fmt.Sprintf("%v", log["sender_id"]),
				fmt.Sprintf("%v", log["action"]),
				fmt.Sprintf("%v", log["reason"]),
				fmt.Sprintf("%v", log["content_preview"]),
				fmt.Sprintf("%v", log["latency_ms"]),
				fmt.Sprintf("%v", log["upstream_id"]),
				fmt.Sprintf("%v", log["app_id"]),
			})
		}
		cw.Flush()
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.json")
		w.WriteHeader(200)
		if logs == nil { logs = []map[string]interface{}{} }
		json.NewEncoder(w).Encode(logs)
	}
}

// handleAuditCleanup POST /api/v1/audit/cleanup — 手动触发日志清理（v3.10）
func (api *ManagementAPI) handleAuditCleanup(w http.ResponseWriter, r *http.Request) {
	retentionDays := api.cfg.AuditRetentionDays
	if retentionDays <= 0 { retentionDays = 30 }
	deleted, err := api.logger.CleanupOldLogs(retentionDays)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[审计] 手动清理了 %d 条过期日志（超过 %d 天）", deleted, retentionDays)
	jsonResponse(w, 200, map[string]interface{}{
		"status":         "cleaned",
		"deleted":        deleted,
		"retention_days": retentionDays,
	})
}

// handleAuditStats GET /api/v1/audit/stats — 日志统计信息（v3.10）
func (api *ManagementAPI) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	stats := api.logger.AuditStats()
	jsonResponse(w, 200, stats)
}

// handleAuditTimeline GET /api/v1/audit/timeline — 时间线统计（v3.10）
func (api *ManagementAPI) handleAuditTimeline(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 { hours = n }
	}
	timeline := api.logger.Timeline(hours)
	if timeline == nil { timeline = []map[string]interface{}{} }
	jsonResponse(w, 200, map[string]interface{}{"timeline": timeline, "hours": hours})
}

func (api *ManagementAPI) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := api.logger.Stats()
	upstreams := api.pool.ListUpstreams()
	healthyCount := 0
	for _, up := range upstreams {
		if up.Healthy { healthyCount++ }
	}
	stats["upstreams_total"] = len(upstreams)
	stats["upstreams_healthy"] = healthyCount
	stats["routes_total"] = api.routes.Count()
	stats["version"] = AppVersion
	stats["uptime"] = time.Since(startTime).String()
	jsonResponse(w, 200, stats)
}

func (api *ManagementAPI) handleRateLimitStats(w http.ResponseWriter, r *http.Request) {
	if api.inbound.limiter == nil {
		jsonResponse(w, 200, map[string]interface{}{"enabled": false})
		return
	}
	stats := api.inbound.limiter.Stats()
	jsonResponse(w, 200, stats)
}

func (api *ManagementAPI) handleRateLimitReset(w http.ResponseWriter, r *http.Request) {
	if api.inbound.limiter == nil {
		jsonResponse(w, 200, map[string]interface{}{"status": "rate limiter not enabled"})
		return
	}
	api.inbound.limiter.Reset()
	jsonResponse(w, 200, map[string]string{"status": "reset"})
}

// handleRuleHits GET /api/v1/rules/hits — 查看规则命中率排行
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
func (api *ManagementAPI) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if api.userCache == nil {
		jsonResponse(w, 200, map[string]interface{}{"users": []interface{}{}, "total": 0, "message": "user info provider not configured"})
		return
	}
	department := r.URL.Query().Get("department")
	email := r.URL.Query().Get("email")
	users := api.userCache.ListAll(department, email)
	if users == nil {
		users = []*UserInfo{}
	}
	jsonResponse(w, 200, map[string]interface{}{"users": users, "total": len(users)})
}

// handleGetUser GET /api/v1/users/:sender_id — 查单个用户
func (api *ManagementAPI) handleGetUser(w http.ResponseWriter, r *http.Request) {
	if api.userCache == nil {
		jsonResponse(w, 404, map[string]string{"error": "user info provider not configured"})
		return
	}
	senderID := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	senderID = strings.TrimSuffix(senderID, "/refresh")
	if senderID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id required"})
		return
	}
	info := api.userCache.GetByID(senderID)
	if info == nil {
		jsonResponse(w, 404, map[string]string{"error": "user not found"})
		return
	}
	jsonResponse(w, 200, info)
}

// handleRefreshUser POST /api/v1/users/:sender_id/refresh — 强制刷新
func (api *ManagementAPI) handleRefreshUser(w http.ResponseWriter, r *http.Request) {
	if api.userCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "user info provider not configured"})
		return
	}
	// Extract sender_id: /api/v1/users/{sender_id}/refresh
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	senderID := strings.TrimSuffix(path, "/refresh")
	if senderID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id required"})
		return
	}
	info, err := api.userCache.Refresh(senderID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// 更新路由表
	api.routes.UpdateUserInfo(senderID, info.Name, info.Email, info.Department)
	jsonResponse(w, 200, info)
}

// handleRefreshAllUsers POST /api/v1/users/refresh-all — 刷新所有
func (api *ManagementAPI) handleRefreshAllUsers(w http.ResponseWriter, r *http.Request) {
	if api.userCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "user info provider not configured"})
		return
	}
	success, failed := api.userCache.RefreshAll()
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "completed",
		"success": success,
		"failed":  failed,
	})
}

// handleListRoutePolicies GET /api/v1/route-policies — 列出路由策略
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
		jsonResponse(w, 200, map[string]interface{}{
			"matched":  false,
			"message":  "no user info available for matching",
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

// ============================================================
// v3.11 规则绑定 API
// ============================================================

// handleListRuleBindings GET /api/v1/rule-bindings — 查看规则绑定关系
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
	api.outboundEngine.mu.RLock()
	rules := make([]map[string]interface{}, len(api.outboundEngine.rules))
	for i, rule := range api.outboundEngine.rules {
		rules[i] = map[string]interface{}{
			"name":           rule.Name,
			"patterns_count": len(rule.Regexps),
			"action":         rule.Action,
		}
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

func (api *ManagementAPI) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// 动态获取 gauge 数据
	upstreamsTotal, upstreamsHealthy := api.pool.Count()
	routesTotal := api.routes.Count()

	// 从 bridge 获取状态（如果有）
	var bridgeStatus *BridgeStatus
	if api.inbound != nil && api.inbound.bridge != nil {
		s := api.inbound.bridge.Status()
		bridgeStatus = &s
	}

	channelName := ""
	if api.channel != nil {
		channelName = api.channel.Name()
	}
	mode := api.cfg.Mode
	if mode == "" {
		mode = "webhook"
	}

	// 生成 Prometheus text format
	// v5.1: 额外指标写入器
	var extraWriters []func(io.Writer)
	extraWriters = append(extraWriters, api.writeV51Metrics)

	api.metrics.WritePrometheus(w, upstreamsTotal, upstreamsHealthy, routesTotal, bridgeStatus, channelName, mode, api.ruleHits, api.inboundEngine, api.outboundEngine, extraWriters...)
}

func (api *ManagementAPI) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// v6.1: 使用 Vue 3 + Vite 构建的 SPA，通过 getDashboardHandler() 提供静态文件
	getDashboardHandler().ServeHTTP(w, r)
}

// ============================================================
// v4.2 备份管理 API
// ============================================================

// handleCreateBackup POST /api/v1/backup — 创建数据库备份
func (api *ManagementAPI) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	sqlStore, ok := api.store.(*SQLiteStore)
	if !ok {
		jsonResponse(w, 500, map[string]string{"error": "backup only supported for SQLite store"})
		return
	}

	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	path, size, err := sqlStore.Backup(backupDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// 自动清理旧备份
	maxCount := api.cfg.BackupMaxCount
	if maxCount <= 0 {
		maxCount = 10
	}
	CleanupOldBackups(backupDir, maxCount)

	log.Printf("[备份] ✅ 手动创建备份: %s (%.2f MB)", path, float64(size)/1024/1024)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"path":   path,
		"size":   size,
	})
}

// handleListBackups GET /api/v1/backups — 列出已有备份
func (api *ManagementAPI) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	backups, err := ListBackups(backupDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"backups": backups,
		"total":   len(backups),
	})
}

// handleDeleteBackup DELETE /api/v1/backups/:name — 删除指定备份
func (api *ManagementAPI) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/backups/")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "backup name required"})
		return
	}

	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	if err := DeleteBackup(backupDir, name); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[备份] 已删除备份: %s", name)
	jsonResponse(w, 200, map[string]string{"status": "deleted", "name": name})
}

// ============================================================
// v5.0 实时监控 API
// ============================================================

// handleRealtimeMetrics GET /api/v1/metrics/realtime — 返回最近 60 秒逐秒统计
func (api *ManagementAPI) handleRealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	if api.realtime == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"slots":  []interface{}{},
			"events": []interface{}{},
		})
		return
	}
	snapshot := api.realtime.Snapshot()
	jsonResponse(w, 200, snapshot)
}

// ============================================================
// v5.0 审计日志归档 API
// ============================================================

// handleAuditArchives GET /api/v1/audit/archives — 列出归档文件
func (api *ManagementAPI) handleAuditArchives(w http.ResponseWriter, r *http.Request) {
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	archives, err := ListArchives(archiveDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"archives": archives, "total": len(archives)})
}

// handleAuditArchiveDownload GET /api/v1/audit/archives/:name — 下载归档文件
func (api *ManagementAPI) handleAuditArchiveDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/audit/archives/")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "archive name required"})
		return
	}
	// 安全检查：不允许路径穿越
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		jsonResponse(w, 400, map[string]string{"error": "invalid archive name"})
		return
	}
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	filePath := archiveDir + name
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonResponse(w, 404, map[string]string{"error": "archive not found"})
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	http.ServeFile(w, r, filePath)
}

// handleAuditArchiveNow POST /api/v1/audit/archive — 手动触发归档
func (api *ManagementAPI) handleAuditArchiveNow(w http.ResponseWriter, r *http.Request) {
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	retentionDays := api.cfg.AuditRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	path, deleted, err := api.logger.ArchiveLogs(retentionDays, archiveDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if path == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"status":  "no_data",
			"message": "没有需要归档的日志",
		})
		return
	}
	log.Printf("[归档] 手动归档完成: %s，删除 %d 条", path, deleted)
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "archived",
		"path":    path,
		"deleted": deleted,
	})
}

// ============================================================
// v5.1 智能检测 API
// ============================================================

// handleListRuleTemplates GET /api/v1/rule-templates — 列出可用规则模板及其规则数
func (api *ManagementAPI) handleListRuleTemplates(w http.ResponseWriter, r *http.Request) {
	templates := ListRuleTemplates()
	if templates == nil {
		templates = []RuleTemplateMeta{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// handleSessionRisks GET /api/v1/sessions/risks — 列出高风险会话
func (api *ManagementAPI) handleSessionRisks(w http.ResponseWriter, r *http.Request) {
	if api.sessionDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"sessions": []interface{}{},
			"total":    0,
			"enabled":  false,
		})
		return
	}
	sessions := api.sessionDetector.ListHighRiskSessions()
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"sessions": sessions,
		"total":    len(sessions),
		"enabled":  true,
	})
}

// handleSessionRisksReset POST /api/v1/sessions/risks/reset — 重置某用户的风险积分
func (api *ManagementAPI) handleSessionRisksReset(w http.ResponseWriter, r *http.Request) {
	if api.sessionDetector == nil {
		jsonResponse(w, 400, map[string]string{"error": "session detector not enabled"})
		return
	}
	var req struct {
		SenderID string `json:"sender_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.SenderID == "" {
		jsonResponse(w, 400, map[string]string{"error": "sender_id required"})
		return
	}
	if api.sessionDetector.ResetRisk(req.SenderID) {
		jsonResponse(w, 200, map[string]string{"status": "reset", "sender_id": req.SenderID})
	} else {
		jsonResponse(w, 404, map[string]string{"error": "session not found"})
	}
}

// writeV51Metrics 写入 v5.1 Prometheus 指标
func (api *ManagementAPI) writeV51Metrics(w io.Writer) {
	// Session risk score gauge
	if api.sessionDetector != nil {
		sessions := api.sessionDetector.ListHighRiskSessions()
		if len(sessions) > 0 {
			fmt.Fprintln(w, "# HELP lobster_guard_session_risk_score Current session risk score")
			fmt.Fprintln(w, "# TYPE lobster_guard_session_risk_score gauge")
			for _, s := range sessions {
				fmt.Fprintf(w, "lobster_guard_session_risk_score{sender_id=%q} %.1f\n", s.SenderID, s.RiskScore)
			}
		}
	}

	// LLM detect counters
	if api.llmDetector != nil && api.llmDetector.cfg.Enabled {
		stats := api.llmDetector.Stats()
		fmt.Fprintln(w, "# HELP lobster_guard_llm_detect_total LLM detection results")
		fmt.Fprintln(w, "# TYPE lobster_guard_llm_detect_total counter")
		fmt.Fprintf(w, "lobster_guard_llm_detect_total{result=\"attack\"} %d\n", stats["attack"])
		fmt.Fprintf(w, "lobster_guard_llm_detect_total{result=\"safe\"} %d\n", stats["safe"])
		fmt.Fprintf(w, "lobster_guard_llm_detect_total{result=\"error\"} %d\n", stats["error"])
		fmt.Fprintf(w, "lobster_guard_llm_detect_total{result=\"timeout\"} %d\n", stats["timeout"])
	}

	// Detect cache counters
	if api.detectCache != nil {
		hits, misses, _ := api.detectCache.Stats()
		fmt.Fprintln(w, "# HELP lobster_guard_detect_cache_hits_total Detect cache hit count")
		fmt.Fprintln(w, "# TYPE lobster_guard_detect_cache_hits_total counter")
		fmt.Fprintf(w, "lobster_guard_detect_cache_hits_total %d\n", hits)
		fmt.Fprintln(w, "# HELP lobster_guard_detect_cache_misses_total Detect cache miss count")
		fmt.Fprintln(w, "# TYPE lobster_guard_detect_cache_misses_total counter")
		fmt.Fprintf(w, "lobster_guard_detect_cache_misses_total %d\n", misses)
	}
}

