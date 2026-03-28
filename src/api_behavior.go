package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleUserRiskTop(w http.ResponseWriter, r *http.Request) {
	if api.userProfileEng == nil {
		jsonResponse(w, 200, map[string]interface{}{"users": []interface{}{}, "total": 0})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	users, err := api.userProfileEng.GetTopRiskUsersTenant(limit, tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if users == nil {
		users = []UserRiskProfile{}
	}
	jsonResponse(w, 200, map[string]interface{}{"users": users, "total": len(users), "tenant": tenantID})
}

// handleUserRiskProfile GET /api/v1/users/risk/:id — 单个用户风险画像
func (api *ManagementAPI) handleUserRiskProfile(w http.ResponseWriter, r *http.Request) {
	if api.userProfileEng == nil {
		jsonResponse(w, 404, map[string]string{"error": "user profile engine not available"})
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/api/v1/users/risk/")
	if userID == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id required"})
		return
	}
	profile, err := api.userProfileEng.GetUserProfile(userID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if profile.TotalRequests == 0 {
		jsonResponse(w, 404, map[string]string{"error": "user not found"})
		return
	}
	jsonResponse(w, 200, profile)
}

// handleUserTimeline GET /api/v1/users/timeline/:id — 用户行为时间线
func (api *ManagementAPI) handleUserTimeline(w http.ResponseWriter, r *http.Request) {
	if api.userProfileEng == nil {
		jsonResponse(w, 200, map[string]interface{}{"events": []interface{}{}, "total": 0})
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/api/v1/users/timeline/")
	if userID == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id required"})
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	events, err := api.userProfileEng.GetUserTimeline(userID, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if events == nil {
		events = []UserTimelineEvent{}
	}
	jsonResponse(w, 200, map[string]interface{}{"events": events, "total": len(events)})
}

// handleUserRiskStats GET /api/v1/users/risk-stats — 风险统计概览（固定 30 天窗口）
func (api *ManagementAPI) handleUserRiskStats(w http.ResponseWriter, r *http.Request) {
	if api.userProfileEng == nil {
		jsonResponse(w, 200, map[string]interface{}{"time_range": "30d"})
		return
	}
	stats, err := api.userProfileEng.GetRiskStats()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, stats)
}

// ============================================================
// v11.1 驾驶舱模式 API handlers
// ============================================================

// handleHealthScore GET /api/v1/health/score — 综合安全健康分（固定 7 天窗口，v11.4 增加 time_range 标注）
func (api *ManagementAPI) handleHealthScore(w http.ResponseWriter, r *http.Request) {
	if api.healthScoreEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "health score engine not available"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	result, err := api.healthScoreEng.CalculateForTenant(tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// v11.4: 包装为带 time_range 的响应
	resp := map[string]interface{}{
		"time_range":  "7d",
		"score":       result.Score,
		"level":       result.Level,
		"level_label": result.LevelLabel,
		"deductions":  result.Deductions,
		"trend":       result.Trend,
		"updated_at":  result.UpdatedAt,
		"tenant":      tenantID,
	}
	jsonResponse(w, 200, resp)
}

// handleOWASPMatrix GET /api/v1/llm/owasp-matrix — OWASP LLM Top 10 矩阵（v11.4: 支持 ?since 参数, v14.0: ?tenant）
func (api *ManagementAPI) handleAnomalyBaselines(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"baselines": map[string]interface{}{}, "total": 0})
		return
	}
	baselines := api.anomalyDetector.GetBaselines()
	jsonResponse(w, 200, map[string]interface{}{"baselines": baselines, "total": len(baselines)})
}

// handleAnomalyAlerts GET /api/v1/anomaly/alerts — 最近异常告警列表
func (api *ManagementAPI) handleAnomalyAlerts(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"alerts": []interface{}{}, "total": 0})
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	alerts := api.anomalyDetector.GetAlerts(limit)
	if alerts == nil {
		alerts = []AnomalyAlert{}
	}
	jsonResponse(w, 200, map[string]interface{}{"alerts": alerts, "total": len(alerts)})
}

// handleAnomalyStatus GET /api/v1/anomaly/status — 异常检测器状态
func (api *ManagementAPI) handleAnomalyStatus(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":         false,
			"metrics_count":   0,
			"baselines_ready": 0,
			"alerts_24h":      0,
			"total_alerts":    0,
		})
		return
	}
	status := api.anomalyDetector.GetStatus()
	jsonResponse(w, 200, status)
}

// handleAnomalyMetric GET /api/v1/anomaly/metric/:name — 单个指标基线详情
func (api *ManagementAPI) handleAnomalyMetric(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "anomaly detector not available"})
		return
	}
	metricName := strings.TrimPrefix(r.URL.Path, "/api/v1/anomaly/metric/")
	if metricName == "" {
		jsonResponse(w, 400, map[string]string{"error": "metric name required"})
		return
	}
	detail := api.anomalyDetector.GetMetricDetail(metricName)
	jsonResponse(w, 200, detail)
}

// handleAnomalyConfigGet GET /api/v1/anomaly/config — 获取异常检测配置
func (api *ManagementAPI) handleAnomalyConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "anomaly detector not available"})
		return
	}
	cfg := api.anomalyDetector.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleAnomalyConfigPut PUT /api/v1/anomaly/config — 更新异常检测配置
func (api *ManagementAPI) handleAnomalyConfigPut(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "anomaly detector not available"})
		return
	}
	var cfg AnomalyConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	api.anomalyDetector.UpdateConfig(cfg)
	newCfg := api.anomalyDetector.GetConfig()
	jsonResponse(w, 200, map[string]interface{}{
		"status": "ok",
		"config": newCfg,
		"note":   "Config updated. Baseline/check intervals take effect on next cycle.",
	})
}

// handleAnomalyMetricThresholdsGet GET /api/v1/anomaly/metric-thresholds — 获取所有指标的独立阈值
func (api *ManagementAPI) handleAnomalyMetricThresholdsGet(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"thresholds": map[string]interface{}{}})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"thresholds": api.anomalyDetector.GetMetricThresholds()})
}

// handleAnomalyMetricThresholdPut PUT /api/v1/anomaly/metric-thresholds/:name — 设置单个指标阈值
func (api *ManagementAPI) handleAnomalyMetricThresholdPut(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 404, map[string]string{"error": "anomaly detector not available"})
		return
	}
	metricName := strings.TrimPrefix(r.URL.Path, "/api/v1/anomaly/metric-thresholds/")
	if metricName == "" {
		jsonResponse(w, 400, map[string]string{"error": "metric name required"})
		return
	}
	var body struct {
		WarningThreshold  float64 `json:"warning_threshold"`
		CriticalThreshold float64 `json:"critical_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	api.anomalyDetector.SetMetricThreshold(metricName, body.WarningThreshold, body.CriticalThreshold)
	jsonResponse(w, 200, map[string]interface{}{"status": "ok", "metric": metricName, "warning_threshold": body.WarningThreshold, "critical_threshold": body.CriticalThreshold})
}

// handleAnomalyTrend GET /api/v1/anomaly/trend/:name — 指标的24h趋势数据（含基线+阈值带）
func (api *ManagementAPI) handleAnomalyTrend(w http.ResponseWriter, r *http.Request) {
	if api.anomalyDetector == nil {
		jsonResponse(w, 200, map[string]interface{}{"points": []interface{}{}})
		return
	}
	metricName := strings.TrimPrefix(r.URL.Path, "/api/v1/anomaly/trend/")
	if metricName == "" {
		jsonResponse(w, 400, map[string]string{"error": "metric name required"})
		return
	}
	trend := api.anomalyDetector.GetMetricTrend(metricName)
	jsonResponse(w, 200, trend)
}

// ============================================================
// v13.1 Prompt 版本追踪 API handlers
// ============================================================

// handlePromptsList GET /api/v1/prompts — Prompt 版本列表（按时间倒序, v14.0: ?tenant）
func (api *ManagementAPI) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if api.leaderboardEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "leaderboard engine not available"})
		return
	}
	scores := api.leaderboardEng.GetLeaderboard()
	if scores == nil {
		scores = []TenantScore{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"scores": scores,
		"total":  len(scores),
		"sla":    api.leaderboardEng.GetSLAConfig(),
	})
}

// handleLeaderboardHeatmap GET /api/v1/leaderboard/heatmap — 攻击热力图
func (api *ManagementAPI) handleLeaderboardHeatmap(w http.ResponseWriter, r *http.Request) {
	if api.leaderboardEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "leaderboard engine not available"})
		return
	}
	cells := api.leaderboardEng.GetHeatmap()
	if cells == nil {
		cells = []AttackHeatmapCell{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"cells":      cells,
		"categories": owaspCategoryOrder,
	})
}

// handleLeaderboardSLA GET /api/v1/leaderboard/sla — SLA 达标情况
func (api *ManagementAPI) handleLeaderboardSLA(w http.ResponseWriter, r *http.Request) {
	if api.leaderboardEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "leaderboard engine not available"})
		return
	}
	overview := api.leaderboardEng.GetSLAOverview()
	jsonResponse(w, 200, overview)
}

// handleLeaderboardSLAConfig PUT /api/v1/leaderboard/sla/config — 更新 SLA 阈值配置
func (api *ManagementAPI) handleLeaderboardSLAConfig(w http.ResponseWriter, r *http.Request) {
	if api.leaderboardEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "leaderboard engine not available"})
		return
	}
	var cfg SLAConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	api.leaderboardEng.SetSLAConfig(cfg)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "ok",
		"config": api.leaderboardEng.GetSLAConfig(),
	})
}

// ============================================================
// v15.0 蜜罐 API
// ============================================================

func (api *ManagementAPI) handleAttackChainList(w http.ResponseWriter, r *http.Request) {
	if api.attackChainEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "attack chain engine not available"})
		return
	}
	q := r.URL.Query()
	tenantID := q.Get("tenant")
	if tenantID == "" {
		tenantID = "all"
	}
	severity := q.Get("severity")
	status := q.Get("status")
	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	chains, err := api.attackChainEng.ListChains(tenantID, severity, status, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, chains)
}

// handleAttackChainGet GET /api/v1/attack-chains/:id
func (api *ManagementAPI) handleAttackChainGet(w http.ResponseWriter, r *http.Request) {
	if api.attackChainEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "attack chain engine not available"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/attack-chains/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	chain, err := api.attackChainEng.GetChain(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "attack chain not found"})
		return
	}
	jsonResponse(w, 200, chain)
}

// handleAttackChainAnalyze POST /api/v1/attack-chains/analyze
func (api *ManagementAPI) handleAttackChainAnalyze(w http.ResponseWriter, r *http.Request) {
	if api.attackChainEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "attack chain engine not available"})
		return
	}
	var req struct {
		TenantID string `json:"tenant_id"`
		Hours    int    `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.TenantID = "default"
		req.Hours = 24
	}
	if req.Hours <= 0 {
		req.Hours = 24
	}
	if req.TenantID == "" {
		req.TenantID = "default"
	}
	chains, err := api.attackChainEng.AnalyzeChains(req.TenantID, req.Hours)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status":    "completed",
		"chains":    chains,
		"count":     len(chains),
		"tenant_id": req.TenantID,
		"hours":     req.Hours,
	})
}

// handleAttackChainUpdateStatus PUT /api/v1/attack-chains/:id/status
func (api *ManagementAPI) handleAttackChainUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if api.attackChainEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "attack chain engine not available"})
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/attack-chains/")
	trimmed = strings.TrimSuffix(trimmed, "/status")
	id := trimmed
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if err := api.attackChainEng.UpdateChainStatus(id, req.Status); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "id": id, "new_status": req.Status})
}

// handleAttackChainPatterns GET /api/v1/attack-chains/patterns
func (api *ManagementAPI) handleAttackChainPatterns(w http.ResponseWriter, r *http.Request) {
	patterns := GetChainPatterns()
	jsonResponse(w, 200, patterns)
}

// handleAttackChainStats GET /api/v1/attack-chains/stats
func (api *ManagementAPI) handleAttackChainStats(w http.ResponseWriter, r *http.Request) {
	if api.attackChainEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "attack chain engine not available"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		tenantID = "all"
	}
	stats := api.attackChainEng.GetStats(tenantID)
	jsonResponse(w, 200, stats)
}

// ============================================================
// v16.0 Agent 行为画像 API handlers
// ============================================================

// handleBehaviorProfileList GET /api/v1/behavior/profiles — Agent 画像列表
func (api *ManagementAPI) handleBehaviorProfileList(w http.ResponseWriter, r *http.Request) {
	if api.behaviorProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "behavior profile engine not available"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	profiles, err := api.behaviorProfileEng.ListProfiles(tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if profiles == nil {
		profiles = []AgentProfile{}
	}

	totalAnomalies := 0
	highRiskCount := 0
	totalPatterns := 0
	for _, p := range profiles {
		totalAnomalies += len(p.Anomalies)
		if p.RiskLevel == "high" || p.RiskLevel == "critical" {
			highRiskCount++
		}
		totalPatterns += len(p.CommonPatterns)
	}

	jsonResponse(w, 200, map[string]interface{}{
		"profiles":        profiles,
		"total":           len(profiles),
		"total_anomalies": totalAnomalies,
		"high_risk_count": highRiskCount,
		"total_patterns":  totalPatterns,
		"tenant":          tenantID,
	})
}

// handleBehaviorProfileGet GET /api/v1/behavior/profiles/:id — 单个 Agent 画像
func (api *ManagementAPI) handleBehaviorProfileGet(w http.ResponseWriter, r *http.Request) {
	if api.behaviorProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "behavior profile engine not available"})
		return
	}
	agentID := strings.TrimPrefix(r.URL.Path, "/api/v1/behavior/profiles/")
	if agentID == "" {
		jsonResponse(w, 400, map[string]string{"error": "agent_id required"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	profile, err := api.behaviorProfileEng.BuildProfile(agentID, tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if profile.TotalRequests == 0 {
		jsonResponse(w, 404, map[string]string{"error": "agent not found"})
		return
	}
	jsonResponse(w, 200, profile)
}

// handleBehaviorAnomalyList GET /api/v1/behavior/anomalies — 行为突变列表
func (api *ManagementAPI) handleBehaviorAnomalyList(w http.ResponseWriter, r *http.Request) {
	if api.behaviorProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "behavior profile engine not available"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	severity := r.URL.Query().Get("severity")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	anomalies, err := api.behaviorProfileEng.ListAnomalies(tenantID, severity, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if anomalies == nil {
		anomalies = []BehaviorAnomaly{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"anomalies": anomalies,
		"total":     len(anomalies),
		"tenant":    tenantID,
	})
}

// handleBehaviorProfileScan POST /api/v1/behavior/profiles/:id/scan — 手动触发扫描
func (api *ManagementAPI) handleBehaviorProfileScan(w http.ResponseWriter, r *http.Request) {
	if api.behaviorProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "behavior profile engine not available"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/behavior/profiles/")
	agentID := strings.TrimSuffix(path, "/scan")
	if agentID == "" {
		jsonResponse(w, 400, map[string]string{"error": "agent_id required"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	profile, err := api.behaviorProfileEng.ScanAndPersist(agentID, tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status":    "scanned",
		"profile":   profile,
		"anomalies": len(profile.Anomalies),
	})
}

// handleBehaviorPatterns GET /api/v1/behavior/patterns — 全局行为序列模式
func (api *ManagementAPI) handleBehaviorPatterns(w http.ResponseWriter, r *http.Request) {
	if api.behaviorProfileEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "behavior profile engine not available"})
		return
	}
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	patterns, err := api.behaviorProfileEng.ListAllPatterns(tenantID)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if patterns == nil {
		patterns = []BehaviorPattern{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"patterns": patterns,
		"total":    len(patterns),
		"tenant":   tenantID,
	})
}

// handleOverviewSummary GET /api/v1/overview/summary — v18 概览页摘要聚合
