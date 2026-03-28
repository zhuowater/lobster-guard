// api.go — ManagementAPI、所有 HTTP handler
// lobster-guard v4.0 代码拆分
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// contextKey 用于 context 传递用户信息
type contextKey string

const userContextKey contextKey = "auth_user"

// getUserFromContext 从 context 中获取当前认证用户
func getUserFromContext(r *http.Request) *User {
	if u, ok := r.Context().Value(userContextKey).(*User); ok {
		return u
	}
	return nil
}

// getRequestIP 提取客户端 IP
func getRequestIP(r *http.Request) string {
	// X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

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
	// v9.0 LLM 侧审计
	llmAuditor      *LLMAuditor        // v9.0 LLM 审计器
	llmRuleEngine   *LLMRuleEngine     // v10.0 LLM 规则引擎
	llmProxy        *LLMProxy          // v10.1 LLM 代理引用（用于 canary token 操作）
	gwManager       *GatewayWSManager  // v29.0 Gateway WSS 连接管理器
	userProfileEng  *UserProfileEngine // v11.0 用户画像引擎
	// v11.1 驾驶舱模式
	healthScoreEng  *HealthScoreEngine
	owaspMatrixEng  *OWASPMatrixEngine
	strictMode      *StrictModeManager
	notificationEng *NotificationEngine
	// v11.2 异常基线检测
	anomalyDetector *AnomalyDetector
	// v12.0 报告引擎
	reportEngine *ReportEngine
	// v13.0 会话回放引擎
	sessionReplayEng *SessionReplayEngine
	// v13.1 Prompt 版本追踪器
	promptTracker *PromptTracker
	// v14.0 租户管理器
	tenantMgr *TenantManager
	// v14.2 红队引擎
	redTeamEngine *RedTeamEngine
	// v14.3 排行榜引擎
	leaderboardEng *LeaderboardEngine
	// v14.1 认证管理器
	authManager *AuthManager
	// v18 trace 关联
	traceCorrelator *TraceCorrelator
	// v15.0 蜜罐引擎
	honeypotEngine *HoneypotEngine
	// v15.1 A/B 测试引擎
	abTestEngine *ABTestEngine
	// v16.0 Agent 行为画像引擎
	behaviorProfileEng *BehaviorProfileEngine
	// v16.1 攻击链引擎
	attackChainEng *AttackChainEngine
	// v17.1 布局存储
	layoutStore *LayoutStore
	// v17.3 会话关联
	sessionCorrelator *SessionCorrelator
	// v18.0 执行信封
	envelopeMgr *EnvelopeManager
	// v18.1 事件总线
	eventBus *EventBus
	// v19.0 对抗性自进化引擎
	evolutionEngine *EvolutionEngine
	// v18.3 自适应决策 + 奇点蜜罐
	adaptiveEngine    *AdaptiveDecisionEngine
	singularityEngine *SingularityEngine
	// v19.1 语义检测引擎
	semanticDetector *SemanticDetector
	// v19.2 蜜罐深度交互引擎
	honeypotDeep *HoneypotDeepEngine
	// v20.0 工具策略引擎
	toolPolicy *ToolPolicyEngine
	// v20.1 污染追踪引擎
	taintTracker *TaintTracker
	// v20.2 污染链逆转引擎
	reversalEngine *TaintReversalEngine
	// v20.3 LLM 响应缓存
	llmCache *LLMCache
	// v20.4 API Gateway
	apiGateway *APIGateway
	// v21.0 K8s 服务发现
	k8sDiscovery *K8sDiscovery
	// v23.0 路径级策略引擎
	pathPolicyEngine *PathPolicyEngine
	// v24.0 反事实验证引擎
	cfVerifier *CounterfactualVerifier
	// v24.2 自适应验证策略
	adaptiveStrategy *AdaptiveStrategy
	// v25.0 执行计划编译器
	planCompiler      *PlanCompiler
	capabilityEngine  *CapabilityEngine
	deviationDetector *DeviationDetector
	// v26.0 信息流控制
	ifcEngine *IFCEngine
	// v26.1 隔离LLM
	ifcQuarantine *IFCQuarantine
	// v27.0 API Key 管理
	apiKeyMgr *APIKeyManager
	// v31.0 AC 智能分级（自动模式）
	autoReviewMgr *AutoReviewManager
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
		gwManager: NewGatewayWSManager(nil, cfg.DefaultGatewayOrigin), // v29.0 Gateway WSS 连接管理器
	}
}

func (api *ManagementAPI) checkManagementAuth(r *http.Request) bool {
	if api.managementToken == "" {
		return true // 未配置 token 时允许访问（启动时已有 🔴 警告，建议生产环境必须配置）
	}
	auth := r.Header.Get("Authorization")
	if auth == "Bearer "+api.managementToken { return true }
	// Cookie 方式（用于 Dashboard iframe/下载等无法设 header 的场景）
	if cookie, err := r.Cookie("lg_token"); err == nil && cookie.Value == api.managementToken { return true }
	return false
}

func (api *ManagementAPI) checkRegistrationAuth(r *http.Request) bool {
	if api.registrationToken == "" { return true }
	auth := r.Header.Get("Authorization")
	return auth == "Bearer "+api.registrationToken
}

// parseSinceParam 解析 since 参数: "1h", "24h", "7d", "30d" → RFC3339 时间字符串（v11.4）
func parseSinceParam(since string) string {
	now := time.Now().UTC()
	switch since {
	case "1h":
		return now.Add(-1 * time.Hour).Format(time.RFC3339)
	case "24h":
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	case "7d":
		return now.AddDate(0, 0, -7).Format(time.RFC3339)
	case "30d":
		return now.AddDate(0, 0, -30).Format(time.RFC3339)
	default:
		return "" // 全量
	}
}

func jsonResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// checkAuth 统一认证检查（v14.1）
// 返回 user（可能为 nil）和 authenticated 状态
func (api *ManagementAPI) checkAuth(r *http.Request) (*User, bool) {
	// 尝试 JWT 验证（无论 auth 启用与否，只要 authManager 存在就尝试）
	if api.authManager != nil {
		tokenStr := ExtractTokenFromRequest(r.Header.Get("Authorization"), r.Header.Get("Cookie"))
		if tokenStr != "" {
			user, err := api.authManager.ValidateToken(tokenStr)
			if err == nil {
				return user, true
			}
		}
	}

	// JWT 验证失败或没有 JWT → 回退到旧的 Bearer token
	if api.checkManagementAuth(r) {
		return nil, true
	}

	// auth 未启用且旧 token 也无效
	if api.authManager == nil || !api.authManager.enabled {
		return nil, false
	}

	return nil, false
}

func (api *ManagementAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	// Dashboard 静态文件（无需鉴权，页面内输入 Token）
	if path == "/" || path == "/dashboard" || strings.HasPrefix(path, "/assets/") ||
		path == "/favicon.svg" || path == "/favicon.ico" || path == "/favicon-32.png" || path == "/apple-touch-icon.png" {
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

	// v14.1: 登录接口（无需认证）
	if path == "/api/v1/auth/login" && method == "POST" {
		api.handleAuthLogin(w, r)
		return
	}

	// v14.1: 检查认证状态（前端用来判断是否需要登录）
	if path == "/api/v1/auth/check" && method == "GET" {
		api.handleAuthCheck(w, r)
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

	// 统一认证检查（v14.1 兼容旧模式）
	user, authenticated := api.checkAuth(r)
	if !authenticated {
		jsonResponse(w, 401, map[string]string{"error": "unauthorized"})
		return
	}

	// 将用户信息放入 context
	if user != nil {
		r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
	}

	// v14.1: 认证相关路由（需登录）
	switch {
	case path == "/api/v1/auth/logout" && method == "POST":
		api.handleAuthLogout(w, r)
		return
	case path == "/api/v1/auth/me" && method == "GET":
		api.handleAuthMe(w, r)
		return
	case path == "/api/v1/auth/password" && method == "POST":
		api.handleAuthPassword(w, r)
		return
	// v14.1: 用户管理（admin only）— 使用独立路径避免和现有 /api/v1/users 冲突
	case path == "/api/v1/auth/users" && method == "GET":
		api.handleAuthUserList(w, r)
		return
	case path == "/api/v1/auth/users" && method == "POST":
		api.handleAuthUserCreate(w, r)
		return
	case strings.HasPrefix(path, "/api/v1/auth/users/") && method == "PUT":
		api.handleAuthUserUpdate(w, r)
		return
	case strings.HasPrefix(path, "/api/v1/auth/users/") && method == "DELETE":
		api.handleAuthUserDelete(w, r)
		return
	case path == "/api/v1/op-audit" && method == "GET":
		api.handleOpAudit(w, r)
		return
	}

	switch {
	case path == "/api/v1/upstreams" && method == "GET":
		api.handleListUpstreams(w, r)
	case path == "/api/v1/upstreams" && method == "POST":
		api.handleCreateUpstream(w, r)
	// v22.0: Gateway 监控中心 — 聚合概览（必须在 /upstreams/{id} 前匹配）
	case path == "/api/v1/upstreams/gateway/overview" && method == "GET":
		api.handleGatewayOverview(w, r)
	// v22.0: Gateway Token 管理
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway-token/status") && method == "GET":
		api.handleGatewayTokenStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway-token") && method == "PUT":
		api.handleGatewayTokenPut(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway-token") && method == "DELETE":
		api.handleGatewayTokenDelete(w, r)
	// v22.0: Gateway 代理 API
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/ping") && method == "GET":
		api.handleGatewayPing(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/sessions") && method == "GET":
		api.handleGatewaySessions(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron") && method == "GET":
		api.handleGatewayCron(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/status") && method == "GET":
		api.handleGatewayStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents") && method == "GET":
		api.handleGatewayAgents(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/session-history") && method == "GET":
		api.handleGatewaySessionHistory(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/skills") && method == "GET":
		api.handleGatewaySkills(w, r)
	// v29.0: WSS 连接状态 + 新 RPC 代理端点
	case path == "/api/v1/gateway/wss/status" && method == "GET":
		api.handleGatewayWSSStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/models") && method == "GET":
		api.handleGatewayModels(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/channels") && method == "GET":
		api.handleGatewayChannels(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/nodes") && method == "GET":
		api.handleGatewayNodes(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/logs") && method == "GET":
		api.handleGatewayLogs(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/config") && method == "GET":
		api.handleGatewayConfig(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/usage") && method == "GET":
		api.handleGatewayUsage(w, r)

	// ===== v29.0 P0: Session 管理 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/session/reset") && method == "POST":
		api.handleGatewaySessionReset(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/session/compact") && method == "POST":
		api.handleGatewaySessionCompact(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/session") && method == "PATCH":
		api.handleGatewaySessionPatch(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/session") && method == "DELETE":
		api.handleGatewaySessionDelete(w, r)

	// ===== v29.0 P0: Chat 操作 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/chat/send") && method == "POST":
		api.handleGatewayChatSend(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/chat/abort") && method == "POST":
		api.handleGatewayChatAbort(w, r)

	// ===== v29.0 P0: Cron CRUD =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron/add") && method == "POST":
		api.handleGatewayCronAdd(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron/update") && method == "PUT":
		api.handleGatewayCronUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron/remove") && (method == "DELETE" || method == "POST"):
		api.handleGatewayCronRemove(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron/run") && method == "POST":
		api.handleGatewayCronRun(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/cron/runs") && method == "GET":
		api.handleGatewayCronRuns(w, r)

	// ===== v29.0 P1: Agent 生命周期 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/create") && method == "POST":
		api.handleGatewayAgentCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/update") && method == "PUT":
		api.handleGatewayAgentUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/delete") && (method == "DELETE" || method == "POST"):
		api.handleGatewayAgentDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/files") && method == "GET":
		api.handleGatewayAgentFiles(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/file") && method == "GET":
		api.handleGatewayAgentFileGet(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/agents/file") && method == "PUT":
		api.handleGatewayAgentFileSet(w, r)

	// ===== v29.0 P1: Config 修改 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/config/schema") && method == "GET":
		api.handleGatewayConfigSchema(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/config") && method == "PATCH":
		api.handleGatewayConfigPatch(w, r)

	// ===== v29.0 P1: Skills 管理 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/skills/bins") && method == "GET":
		api.handleGatewaySkillsBins(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/skills/install") && method == "POST":
		api.handleGatewaySkillsInstall(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/skills/update") && method == "POST":
		api.handleGatewaySkillsUpdate(w, r)

	// ===== v29.0 P1: 心跳管理 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/heartbeat") && method == "GET":
		api.handleGatewayHeartbeat(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/heartbeat") && method == "PUT":
		api.handleGatewaySetHeartbeats(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/wake") && method == "POST":
		api.handleGatewayWake(w, r)

	// ===== v29.0 P1: 设备配对 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/devices") && method == "GET":
		api.handleGatewayDevicePairs(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/devices/approve") && method == "POST":
		api.handleGatewayDevicePairAction(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/devices/reject") && method == "POST":
		api.handleGatewayDevicePairAction(w, r)

	// ===== v29.0 P1: 节点管理 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/node-pairs") && method == "GET":
		api.handleGatewayNodePairs(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/node-pairs/approve") && method == "POST":
		api.handleGatewayNodePairAction(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/node-pairs/reject") && method == "POST":
		api.handleGatewayNodePairAction(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/nodes/describe") && method == "GET":
		api.handleGatewayNodeDescribe(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/nodes/rename") && method == "POST":
		api.handleGatewayNodeRename(w, r)

	// ===== v29.0 P1: 系统事件 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/system-event") && method == "POST":
		api.handleGatewaySystemEvent(w, r)

	// ===== v29.0 P2: 执行审批 / Gateway 控制 / 记忆 / Skill 卸载 =====
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/exec-approvals") && method == "GET":
		api.handleGatewayExecApprovals(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/exec-approvals/approve") && method == "POST":
		api.handleGatewayExecApprovalAction(w, r, true)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/exec-approvals/reject") && method == "POST":
		api.handleGatewayExecApprovalAction(w, r, false)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/restart") && method == "POST":
		api.handleGatewayRestart(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/update") && method == "POST":
		api.handleGatewayUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/memory/search") && method == "POST":
		api.handleGatewayMemorySearch(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/gateway/skills/uninstall") && method == "POST":
		api.handleGatewaySkillUninstall(w, r)

	// v21.0: 上游 CRUD（带 ID 的路由必须在 health-check 之后匹配）
	case strings.HasPrefix(path, "/api/v1/upstreams/") && strings.HasSuffix(path, "/health-check") && method == "POST":
		api.handleUpstreamHealthCheck(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && method == "PUT":
		api.handleUpdateUpstream(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && method == "DELETE":
		api.handleDeleteUpstream(w, r)
	case strings.HasPrefix(path, "/api/v1/upstreams/") && method == "GET":
		api.handleGetUpstream(w, r)
	// v21.0: K8s 发现状态 API
	case path == "/api/v1/discovery/status" && method == "GET":
		api.handleDiscoveryStatus(w, r)
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
	case path == "/api/v1/routes/batch-unbind" && method == "POST":
		api.handleBatchUnbindRoute(w, r)
	case path == "/api/v1/routes/batch-migrate" && method == "POST":
		api.handleBatchMigrateRoute(w, r)
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
	case path == "/api/v1/outbound-rules/add" && method == "POST":
		api.handleAddOutboundRule(w, r)
	case path == "/api/v1/outbound-rules/update" && method == "PUT":
		api.handleUpdateOutboundRule(w, r)
	case path == "/api/v1/outbound-rules/delete" && method == "DELETE":
		api.handleDeleteOutboundRule(w, r)
	case path == "/api/v1/outbound-rules/reload" && method == "POST":
		api.handleReloadOutboundRules(w, r)
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
	// v11.0 用户画像 API（必须在通配 /api/v1/users/ GET 之前）
	case path == "/api/v1/users/risk-top" && method == "GET":
		api.handleUserRiskTop(w, r)
	case path == "/api/v1/users/risk-stats" && method == "GET":
		api.handleUserRiskStats(w, r)
	case strings.HasPrefix(path, "/api/v1/users/timeline/") && method == "GET":
		api.handleUserTimeline(w, r)
	case strings.HasPrefix(path, "/api/v1/users/risk/") && method == "GET":
		api.handleUserRiskProfile(w, r)
	case strings.HasPrefix(path, "/api/v1/users/") && strings.HasSuffix(path, "/refresh") && method == "POST":
		api.handleRefreshUser(w, r)
	case strings.HasPrefix(path, "/api/v1/users/") && method == "GET":
		api.handleGetUser(w, r)
	case path == "/api/v1/route-policies" && method == "GET":
		api.handleListRoutePolicies(w, r)
	case path == "/api/v1/route-policies" && method == "POST":
		api.handleCreateRoutePolicy(w, r)
	case path == "/api/v1/route-policies/test" && method == "POST":
		api.handleTestRoutePolicy(w, r)
	case strings.HasPrefix(path, "/api/v1/route-policies/") && method == "PUT":
		api.handleUpdateRoutePolicy(w, r)
	case strings.HasPrefix(path, "/api/v1/route-policies/") && method == "DELETE":
		api.handleDeleteRoutePolicy(w, r)
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
	// v6.3 规则 CRUD API
	case path == "/api/v1/inbound-rules/add" && method == "POST":
		api.handleAddInboundRule(w, r)
	case path == "/api/v1/inbound-rules/update" && method == "PUT":
		api.handleUpdateInboundRule(w, r)
	case path == "/api/v1/inbound-rules/delete" && method == "DELETE":
		api.handleDeleteInboundRule(w, r)
	case path == "/api/v1/inbound-rules/toggle-shadow" && method == "POST":
		api.handleInboundToggleShadow(w, r)
	case path == "/api/v1/outbound-rules/toggle-shadow" && method == "POST":
		api.handleOutboundToggleShadow(w, r)
	case path == "/api/v1/rules/export" && method == "GET":
		api.handleExportRules(w, r)
	case path == "/api/v1/rules/import" && method == "POST":
		api.handleImportRules(w, r)
	case path == "/api/v1/rule-templates/detail" && method == "GET":
		api.handleRuleTemplateDetail(w, r)
	// Demo data seed/clear API
	case path == "/api/v1/demo/seed" && method == "POST":
		api.handleDemoSeed(w, r)
	case path == "/api/v1/demo/clear" && method == "DELETE":
		api.handleDemoClear(w, r)
	case path == "/api/v1/simulate/traffic" && method == "POST":
		api.handleSimulateTraffic(w, r)
	// v27.1 入站规则行业模板 API
	case path == "/api/v1/inbound-templates" && method == "GET":
		api.handleInboundTemplateList(w, r)
	case path == "/api/v1/inbound-templates" && method == "POST":
		api.handleInboundTemplateCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/inbound-templates/") && method == "GET":
		api.handleInboundTemplateGet(w, r)
	case strings.HasPrefix(path, "/api/v1/inbound-templates/") && method == "PUT":
		api.handleInboundTemplateUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/inbound-templates/") && strings.HasSuffix(path, "/enable") && method == "POST":
		api.handleInboundTemplateEnable(w, r)
	case strings.HasPrefix(path, "/api/v1/inbound-templates/") && method == "DELETE":
		api.handleInboundTemplateDelete(w, r)

	// v28.0 LLM 规则模板 API
	case path == "/api/v1/llm/templates" && method == "GET":
		api.handleLLMTemplateList(w, r)
	case path == "/api/v1/llm/templates" && method == "POST":
		api.handleLLMTemplateCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/llm/templates/") && method == "GET":
		api.handleLLMTemplateGet(w, r)
	case strings.HasPrefix(path, "/api/v1/llm/templates/") && strings.HasSuffix(path, "/enable") && method == "POST":
		api.handleLLMTemplateEnable(w, r)
	case strings.HasPrefix(path, "/api/v1/llm/templates/") && method == "PUT":
		api.handleLLMTemplateUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/llm/templates/") && method == "DELETE":
		api.handleLLMTemplateDelete(w, r)

	// v14.0 租户管理 API
	case path == "/api/v1/tenants" && method == "GET":
		api.handleTenantList(w, r)
	case path == "/api/v1/tenants" && method == "POST":
		api.handleTenantCreate(w, r)
	case path == "/api/v1/tenants/resolve" && method == "GET":
		api.handleTenantResolve(w, r)
	// v14.0 租户成员 API（必须在通配 tenants/:id GET 之前）
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/members") && method == "GET":
		api.handleTenantMemberList(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/members") && method == "POST":
		api.handleTenantMemberAdd(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.Contains(path, "/members/") && method == "DELETE":
		api.handleTenantMemberDelete(w, r)
	// v14.0 租户安全配置 API
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/config") && method == "GET":
		api.handleTenantConfigGet(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/config") && method == "PUT":
		api.handleTenantConfigUpdate(w, r)
	// v27.0: 租户策略模板绑定 API
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/bind-template") && method == "POST":
		api.handleTenantBindTemplate(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/bind-inbound-template") && method == "POST":
		api.handleTenantBindInboundTemplate(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/inbound-rules") && method == "GET":
		api.handleTenantInboundRules(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/inbound-rules") && method == "DELETE":
		api.handleTenantDeleteInboundRules(w, r)
	// v28.0: 租户 LLM 规则模板绑定 API
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/bind-llm-template") && method == "POST":
		api.handleTenantBindLLMTemplate(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/llm-rules") && method == "GET":
		api.handleTenantLLMRules(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/llm-rules") && method == "DELETE":
		api.handleTenantDeleteLLMRules(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && strings.HasSuffix(path, "/policies") && method == "GET":
		api.handleTenantPolicies(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && method == "GET":
		api.handleTenantGet(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && method == "PUT":
		api.handleTenantUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/tenants/") && method == "DELETE":
		api.handleTenantDelete(w, r)
	// v27.0 API Key 管理 API
	case path == "/api/v1/apikeys" && method == "GET":
		api.handleAPIKeyList(w, r)
	case path == "/api/v1/apikeys" && method == "POST":
		api.handleAPIKeyCreate(w, r)
	case path == "/api/v1/apikeys/pending" && method == "GET":
		api.handleAPIKeyPendingList(w, r)
	case path == "/api/v1/apikeys/stats" && method == "GET":
		api.handleAPIKeyStats(w, r)
	case strings.HasPrefix(path, "/api/v1/apikeys/") && strings.HasSuffix(path, "/bind") && method == "POST":
		api.handleAPIKeyBind(w, r)
	case strings.HasPrefix(path, "/api/v1/apikeys/") && strings.HasSuffix(path, "/rotate") && method == "POST":
		api.handleAPIKeyRotate(w, r)
	case strings.HasPrefix(path, "/api/v1/apikeys/") && method == "GET":
		api.handleAPIKeyGet(w, r)
	case strings.HasPrefix(path, "/api/v1/apikeys/") && method == "PUT":
		api.handleAPIKeyUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/apikeys/") && method == "DELETE":
		api.handleAPIKeyDelete(w, r)
	// v13.1 Prompt 版本追踪 API
	case path == "/api/v1/prompts" && method == "GET":
		api.handlePromptsList(w, r)
	case path == "/api/v1/prompts/current" && method == "GET":
		api.handlePromptsCurrent(w, r)
	case strings.HasPrefix(path, "/api/v1/prompts/") && strings.HasSuffix(path, "/diff") && method == "GET":
		api.handlePromptsDiff(w, r)
	case strings.HasPrefix(path, "/api/v1/prompts/") && strings.HasSuffix(path, "/tag") && method == "POST":
		api.handlePromptsTag(w, r)
	case strings.HasPrefix(path, "/api/v1/prompts/") && strings.HasSuffix(path, "/rollback") && method == "POST":
		api.handlePromptsRollback(w, r)
	case path == "/api/v1/prompts/stats" && method == "GET":
		api.handlePromptsStats(w, r)
	case strings.HasPrefix(path, "/api/v1/prompts/") && method == "GET":
		api.handlePromptsGet(w, r)
	// v13.0 会话回放 API
	case path == "/api/v1/sessions/replay" && method == "GET":
		api.handleSessionReplayList(w, r)
	case strings.HasPrefix(path, "/api/v1/sessions/replay/tags/") && method == "DELETE":
		api.handleSessionReplayDeleteTag(w, r)
	case strings.HasPrefix(path, "/api/v1/sessions/replay/") && strings.HasSuffix(path, "/tags") && method == "POST":
		api.handleSessionReplayAddTag(w, r)
	case strings.HasPrefix(path, "/api/v1/sessions/replay/") && method == "GET":
		api.handleSessionReplayDetail(w, r)
	// v5.1 智能检测 API
	case path == "/api/v1/rule-templates" && method == "GET":
		api.handleListRuleTemplates(w, r)
	case path == "/api/v1/sessions/risks" && method == "GET":
		api.handleSessionRisks(w, r)
	case path == "/api/v1/sessions/risks/reset" && method == "POST":
		api.handleSessionRisksReset(w, r)
	// v17.3 会话关联
	case path == "/api/v1/session-correlator/stats" && method == "GET":
		api.handleSessionCorrelatorStats(w, r)
	case path == "/api/v1/session-correlator/config" && method == "GET":
		api.handleSessionCorrelatorConfig(w, r)
	case path == "/api/v1/session-correlator/config" && method == "PUT":
		api.handleSessionCorrelatorConfigUpdate(w, r)
	// v8.0 运维工具箱 API
	case path == "/api/v1/config/view" && method == "GET":
		api.handleConfigView(w, r)
	case path == "/api/v1/config" && method == "GET":
		api.handleConfigGet(w, r)
	case path == "/api/v1/config/validate" && method == "GET":
		api.handleConfigValidate(w, r)
	case path == "/api/v1/config/settings" && method == "PUT":
		api.handleConfigSettingsUpdate(w, r)
	case path == "/api/v1/alerts/test" && method == "POST":
		api.handleAlertTest(w, r)
	case path == "/api/v1/alerts/config" && method == "PUT":
		api.handleAlertsConfigUpdate(w, r)
	case path == "/api/v1/system/diag" && method == "GET":
		api.handleSystemDiag(w, r)
	case path == "/api/v1/alerts/history" && method == "GET":
		api.handleAlertsHistory(w, r)
	case path == "/api/v1/alerts/config" && method == "GET":
		api.handleAlertsConfig(w, r)
	// v8.0 备份恢复 + 下载
	case strings.HasPrefix(path, "/api/v1/backups/") && strings.HasSuffix(path, "/restore") && method == "POST":
		api.handleRestoreBackup(w, r)
	case strings.HasPrefix(path, "/api/v1/backups/") && strings.HasSuffix(path, "/download") && method == "GET":
		api.handleDownloadBackup(w, r)
	// v11.1 驾驶舱模式 API
	case path == "/api/v1/health/score" && method == "GET":
		api.handleHealthScore(w, r)
	case path == "/api/v1/llm/owasp-matrix" && method == "GET":
		api.handleOWASPMatrix(w, r)
	case path == "/api/v1/system/strict-mode" && method == "GET":
		api.handleStrictModeGet(w, r)
	case path == "/api/v1/system/strict-mode" && method == "POST":
		api.handleStrictModeSet(w, r)
	case path == "/api/v1/notifications" && method == "GET":
		api.handleNotifications(w, r)
	// v12.0 报告引擎 API
	case path == "/api/v1/reports/generate" && method == "POST":
		api.handleReportGenerate(w, r)
	case path == "/api/v1/reports" && method == "GET":
		api.handleReportList(w, r)
	case strings.HasPrefix(path, "/api/v1/reports/") && strings.HasSuffix(path, "/download") && method == "GET":
		api.handleReportDownload(w, r)
	case strings.HasPrefix(path, "/api/v1/reports/") && method == "DELETE":
		api.handleReportDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/reports/") && method == "GET":
		api.handleReportGet(w, r)
	// v12.0 LLM 审计导出 API
	case path == "/api/v1/llm/export" && method == "GET":
		api.handleLLMExport(w, r)
	// v11.2 异常基线检测 API
	case path == "/api/v1/anomaly/baselines" && method == "GET":
		api.handleAnomalyBaselines(w, r)
	case path == "/api/v1/anomaly/alerts" && method == "GET":
		api.handleAnomalyAlerts(w, r)
	case path == "/api/v1/anomaly/status" && method == "GET":
		api.handleAnomalyStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/anomaly/metric/") && method == "GET":
		api.handleAnomalyMetric(w, r)
	case path == "/api/v1/anomaly/config" && method == "GET":
		api.handleAnomalyConfigGet(w, r)
	case path == "/api/v1/anomaly/config" && method == "PUT":
		api.handleAnomalyConfigPut(w, r)
	case strings.HasPrefix(path, "/api/v1/anomaly/metric-thresholds/") && method == "PUT":
		api.handleAnomalyMetricThresholdPut(w, r)
	case path == "/api/v1/anomaly/metric-thresholds" && method == "GET":
		api.handleAnomalyMetricThresholdsGet(w, r)
	case strings.HasPrefix(path, "/api/v1/anomaly/trend/") && method == "GET":
		api.handleAnomalyTrend(w, r)
	// v14.3 排行榜 + SLA API
	case path == "/api/v1/leaderboard" && method == "GET":
		api.handleLeaderboard(w, r)
	case path == "/api/v1/leaderboard/heatmap" && method == "GET":
		api.handleLeaderboardHeatmap(w, r)
	case path == "/api/v1/leaderboard/sla" && method == "GET":
		api.handleLeaderboardSLA(w, r)
	case path == "/api/v1/leaderboard/sla/config" && method == "PUT":
		api.handleLeaderboardSLAConfig(w, r)
	// v14.2 Red Team Autopilot API
	case path == "/api/v1/redteam/run" && method == "POST":
		api.handleRedTeamRun(w, r)
	case path == "/api/v1/redteam/reports" && method == "GET":
		api.handleRedTeamReportList(w, r)
	case path == "/api/v1/redteam/vectors" && method == "GET":
		api.handleRedTeamVectors(w, r)
	case strings.HasPrefix(path, "/api/v1/redteam/reports/") && method == "DELETE":
		api.handleRedTeamReportDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/redteam/reports/") && method == "GET":
		api.handleRedTeamReportGet(w, r)
	// v9.0 LLM 侧安全审计 API
	case strings.HasPrefix(path, "/api/v1/llm/"):
		// LLM config 端点不需要 llmAuditor（即使 LLM 未启用也要能读写配置）
		switch {
		case path == "/api/v1/llm/config" && method == "GET":
			api.handleLLMConfigGet(w, r)
		case path == "/api/v1/llm/config" && method == "PUT":
			api.handleLLMConfigPut(w, r)
		// v10.2: LLM Targets CRUD API（不需要 llmAuditor）
		case path == "/api/v1/llm/targets" && method == "GET":
			api.handleLLMTargetsList(w, r)
		case path == "/api/v1/llm/targets" && method == "POST":
			api.handleLLMTargetsCreate(w, r)
		case strings.HasPrefix(path, "/api/v1/llm/targets/") && method == "PUT":
			api.handleLLMTargetsUpdate(w, r)
		case strings.HasPrefix(path, "/api/v1/llm/targets/") && method == "DELETE":
			api.handleLLMTargetsDelete(w, r)
		// v10.0: LLM 规则 API（不需要 llmAuditor）
		case path == "/api/v1/llm/rules" && method == "GET":
			api.handleLLMRulesList(w, r)
		case path == "/api/v1/llm/rules" && method == "POST":
			api.handleLLMRulesCreate(w, r)
		case path == "/api/v1/llm/rules/hits" && method == "GET":
			api.handleLLMRulesHits(w, r)
		case strings.HasPrefix(path, "/api/v1/llm/rules/") && strings.HasSuffix(path, "/toggle-shadow") && method == "POST":
			api.handleLLMRulesToggleShadow(w, r)
		case strings.HasPrefix(path, "/api/v1/llm/rules/") && method == "PUT":
			api.handleLLMRulesUpdate(w, r)
		case strings.HasPrefix(path, "/api/v1/llm/rules/") && method == "DELETE":
			api.handleLLMRulesDelete(w, r)
		// v10.1: Canary Token API（不需要 llmAuditor 来获取状态，但需要用于查询泄露）
		case path == "/api/v1/llm/canary/status" && method == "GET":
			api.handleCanaryStatus(w, r)
		case path == "/api/v1/llm/canary/rotate" && method == "POST":
			api.handleCanaryRotate(w, r)
		case path == "/api/v1/llm/canary/leaks" && method == "GET":
			api.handleCanaryLeaks(w, r)
		// v10.1: Response Budget API
		case path == "/api/v1/llm/budget/status" && method == "GET":
			api.handleBudgetStatus(w, r)
		case path == "/api/v1/llm/budget/violations" && method == "GET":
			api.handleBudgetViolations(w, r)
		default:
			if api.llmAuditor == nil {
				jsonResponse(w, 404, map[string]string{"error": "LLM proxy not enabled"})
				return
			}
			switch {
			case path == "/api/v1/llm/status" && method == "GET":
				api.handleLLMStatus(w, r)
			case path == "/api/v1/llm/overview" && method == "GET":
				api.handleLLMOverview(w, r)
			case path == "/api/v1/llm/calls" && method == "GET":
				api.handleLLMCalls(w, r)
			case path == "/api/v1/llm/tools" && method == "GET":
				api.handleLLMTools(w, r)
			case path == "/api/v1/llm/tools/stats" && method == "GET":
				api.handleLLMToolStats(w, r)
			case path == "/api/v1/llm/tools/timeline" && method == "GET":
				api.handleLLMToolTimeline(w, r)
			default:
				w.WriteHeader(404)
			}
		}
	// v15.0 蜜罐 API
	case path == "/api/v1/honeypot/templates" && method == "GET":
		api.handleHoneypotTemplateList(w, r)
	case path == "/api/v1/honeypot/templates" && method == "POST":
		api.handleHoneypotTemplateCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/honeypot/templates/") && method == "PUT":
		api.handleHoneypotTemplateUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/honeypot/templates/") && method == "DELETE":
		api.handleHoneypotTemplateDelete(w, r)
	case path == "/api/v1/honeypot/triggers" && method == "GET":
		api.handleHoneypotTriggerList(w, r)
	case strings.HasPrefix(path, "/api/v1/honeypot/triggers/") && method == "GET":
		api.handleHoneypotTriggerGet(w, r)
	case path == "/api/v1/honeypot/stats" && method == "GET":
		api.handleHoneypotStats(w, r)
	case path == "/api/v1/honeypot/test" && method == "POST":
		api.handleHoneypotTest(w, r)
	// v19.2 蜜罐深度交互 API
	case path == "/api/v1/honeypot/interactions" && method == "GET":
		api.handleHoneypotDeepInteractions(w, r)
	case path == "/api/v1/honeypot/loyalty" && method == "GET":
		api.handleHoneypotDeepLoyaltyList(w, r)
	case strings.HasPrefix(path, "/api/v1/honeypot/loyalty/") && method == "GET":
		api.handleHoneypotDeepLoyaltyGet(w, r)
	case path == "/api/v1/honeypot/feedback" && method == "POST":
		api.handleHoneypotDeepFeedback(w, r)
	case strings.HasPrefix(path, "/api/v1/honeypot/feedback/") && method == "POST":
		api.handleHoneypotDeepFeedbackByID(w, r)
	case path == "/api/v1/honeypot/deep/stats" && method == "GET":
		api.handleHoneypotDeepStats(w, r)
	case path == "/api/v1/honeypot/deep/record" && method == "POST":
		api.handleHoneypotDeepRecord(w, r)
	// v15.1 A/B 测试 API
	case path == "/api/v1/ab-tests" && method == "GET":
		api.handleABTestList(w, r)
	case path == "/api/v1/ab-tests" && method == "POST":
		api.handleABTestCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/ab-tests/") && strings.HasSuffix(path, "/start") && method == "POST":
		api.handleABTestStart(w, r)
	case strings.HasPrefix(path, "/api/v1/ab-tests/") && strings.HasSuffix(path, "/stop") && method == "POST":
		api.handleABTestStop(w, r)
	case strings.HasPrefix(path, "/api/v1/ab-tests/") && method == "PUT":
		api.handleABTestUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/ab-tests/") && method == "DELETE":
		api.handleABTestDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/ab-tests/") && method == "GET":
		api.handleABTestGet(w, r)
	// v16.0 Agent 行为画像 API
	case path == "/api/v1/behavior/profiles" && method == "GET":
		api.handleBehaviorProfileList(w, r)
	case path == "/api/v1/behavior/anomalies" && method == "GET":
		api.handleBehaviorAnomalyList(w, r)
	case path == "/api/v1/behavior/patterns" && method == "GET":
		api.handleBehaviorPatterns(w, r)
	case strings.HasPrefix(path, "/api/v1/behavior/profiles/") && strings.HasSuffix(path, "/scan") && method == "POST":
		api.handleBehaviorProfileScan(w, r)
	case strings.HasPrefix(path, "/api/v1/behavior/profiles/") && method == "GET":
		api.handleBehaviorProfileGet(w, r)

	// v16.1 攻击链分析 API
	case path == "/api/v1/attack-chains" && method == "GET":
		api.handleAttackChainList(w, r)
	case path == "/api/v1/attack-chains/analyze" && method == "POST":
		api.handleAttackChainAnalyze(w, r)
	case path == "/api/v1/attack-chains/patterns" && method == "GET":
		api.handleAttackChainPatterns(w, r)
	case path == "/api/v1/attack-chains/stats" && method == "GET":
		api.handleAttackChainStats(w, r)
	case strings.HasPrefix(path, "/api/v1/attack-chains/") && strings.HasSuffix(path, "/status") && method == "PUT":
		api.handleAttackChainUpdateStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/attack-chains/") && method == "GET":
		api.handleAttackChainGet(w, r)

	// v17.1 布局持久化 API
	case path == "/api/v1/layouts" && method == "GET":
		api.handleLayoutList(w, r)
	case path == "/api/v1/layouts" && method == "POST":
		api.handleLayoutCreate(w, r)
	case path == "/api/v1/layouts/presets" && method == "GET":
		api.handleLayoutPresets(w, r)
	case path == "/api/v1/layouts/active" && method == "POST":
		api.handleLayoutSetActive(w, r)
	case strings.HasPrefix(path, "/api/v1/layouts/") && method == "GET":
		api.handleLayoutGet(w, r)
	case strings.HasPrefix(path, "/api/v1/layouts/") && method == "PUT":
		api.handleLayoutUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/layouts/") && method == "DELETE":
		api.handleLayoutDelete(w, r)

	// v18.0 执行信封 API
	case strings.HasPrefix(path, "/api/v1/envelopes/verify/") && method == "GET":
		api.handleEnvelopeVerify(w, r)
	case strings.HasPrefix(path, "/api/v1/envelopes/chain/") && method == "GET":
		api.handleEnvelopeChainVerify(w, r)
	case path == "/api/v1/envelopes/list" && method == "GET":
		api.handleEnvelopeList(w, r)
	case path == "/api/v1/envelopes/stats" && method == "GET":
		api.handleEnvelopeStats(w, r)
	case path == "/api/v1/envelopes/config" && method == "PUT":
		api.handleEnvelopeConfigUpdate(w, r)

	// v18.0+ Merkle Tree API
	case strings.HasPrefix(path, "/api/v1/envelopes/proof/") && method == "GET":
		api.handleEnvelopeProof(w, r)
	case path == "/api/v1/envelopes/batches" && method == "GET":
		api.handleEnvelopeBatchList(w, r)
	case strings.HasPrefix(path, "/api/v1/envelopes/batch/") && method == "GET":
		api.handleEnvelopeBatchDetail(w, r)

	// v18.1: 事件总线 API
	case path == "/api/v1/events/list" && method == "GET":
		api.handleEventsList(w, r)
	case path == "/api/v1/events/stats" && method == "GET":
		api.handleEventsStats(w, r)
	case path == "/api/v1/events/targets" && method == "GET":
		api.handleEventsTargetList(w, r)
	case path == "/api/v1/events/targets" && method == "POST":
		api.handleEventsTargetCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/events/targets/") && method == "PUT":
		api.handleEventsTargetUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/events/targets/") && method == "DELETE":
		api.handleEventsTargetDelete(w, r)
	case path == "/api/v1/events/test" && method == "POST":
		api.handleEventsTest(w, r)
	case path == "/api/v1/events/deliveries" && method == "GET":
		api.handleEventsDeliveries(w, r)
	case path == "/api/v1/events/chains" && method == "GET":
		api.handleEventsChains(w, r)

	// v18: 概览摘要聚合 API
	case path == "/api/v1/overview/summary" && method == "GET":
		api.handleOverviewSummary(w, r)

	// v17.0: 态势大屏聚合 API
	case path == "/api/v1/bigscreen/data" && method == "GET":
		api.handleBigScreenData(w, r)

	// v18.3: 自适应决策 API
	case path == "/api/v1/adaptive/stats" && method == "GET":
		api.handleAdaptiveStats(w, r)
	case strings.HasPrefix(path, "/api/v1/adaptive/proof/") && method == "GET":
		api.handleAdaptiveProof(w, r)
	case path == "/api/v1/adaptive/feedback" && method == "POST":
		api.handleAdaptiveFeedback(w, r)
	case path == "/api/v1/adaptive/config" && method == "GET":
		api.handleAdaptiveConfigGet(w, r)
	case path == "/api/v1/adaptive/config" && method == "PUT":
		api.handleAdaptiveConfigPut(w, r)

	// v18.3: 奇点蜜罐 API
	case path == "/api/v1/singularity/config" && method == "GET":
		api.handleSingularityConfigGet(w, r)
	case path == "/api/v1/singularity/config" && method == "PUT":
		api.handleSingularityConfigPut(w, r)
	case path == "/api/v1/singularity/templates" && method == "GET":
		api.handleSingularityTemplates(w, r)
	case path == "/api/v1/singularity/recommend" && method == "GET":
		api.handleSingularityRecommend(w, r)
	case path == "/api/v1/singularity/budget" && method == "GET":
		api.handleSingularityBudget(w, r)
	case path == "/api/v1/singularity/history" && method == "GET":
		api.handleSingularityHistory(w, r)

	// v19.0: 对抗性自进化 API
	case path == "/api/v1/evolution/run" && method == "POST":
		api.handleEvolutionRun(w, r)
	case path == "/api/v1/evolution/stats" && method == "GET":
		api.handleEvolutionStats(w, r)
	case path == "/api/v1/evolution/log" && method == "GET":
		api.handleEvolutionLog(w, r)
	case path == "/api/v1/evolution/strategies" && method == "GET":
		api.handleEvolutionStrategies(w, r)
	case path == "/api/v1/evolution/config" && method == "GET":
		api.handleEvolutionConfigGet(w, r)
	case path == "/api/v1/evolution/config" && method == "PUT":
		api.handleEvolutionConfigPut(w, r)

	// v19.1: 语义检测引擎 API
	case path == "/api/v1/semantic/stats" && method == "GET":
		api.handleSemanticStats(w, r)
	case path == "/api/v1/semantic/config" && method == "GET":
		api.handleSemanticConfigGet(w, r)
	case path == "/api/v1/semantic/config" && method == "PUT":
		api.handleSemanticConfigPut(w, r)
	case path == "/api/v1/semantic/analyze" && method == "POST":
		api.handleSemanticAnalyze(w, r)
	case path == "/api/v1/semantic/patterns" && method == "GET":
		api.handleSemanticPatterns(w, r)

	// v20.0: 工具策略引擎 API
	case path == "/api/v1/tools/stats" && method == "GET":
		api.handleToolPolicyStats(w, r)
	case path == "/api/v1/tools/events" && method == "GET":
		api.handleToolPolicyEvents(w, r)
	case path == "/api/v1/tools/rules" && method == "GET":
		api.handleToolPolicyRulesList(w, r)
	case path == "/api/v1/tools/rules" && method == "POST":
		api.handleToolPolicyRulesCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/tools/rules/") && method == "PUT":
		api.handleToolPolicyRulesUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/tools/rules/") && method == "DELETE":
		api.handleToolPolicyRulesDelete(w, r)
	case path == "/api/v1/tools/config" && method == "GET":
		api.handleToolPolicyConfigGet(w, r)
	case path == "/api/v1/tools/config" && method == "PUT":
		api.handleToolPolicyConfigUpdate(w, r)
	case path == "/api/v1/tools/evaluate" && method == "POST":
		api.handleToolPolicyEvaluate(w, r)

	// v20.1: 污染追踪 API
	case path == "/api/v1/taint/stats" && method == "GET":
		api.handleTaintStats(w, r)
	case path == "/api/v1/taint/active" && method == "GET":
		api.handleTaintActive(w, r)
	case strings.HasPrefix(path, "/api/v1/taint/trace/") && method == "GET":
		api.handleTaintTrace(w, r)
	case path == "/api/v1/taint/config" && method == "GET":
		api.handleTaintConfigGet(w, r)
	case path == "/api/v1/taint/config" && method == "PUT":
		api.handleTaintConfigUpdate(w, r)
	case path == "/api/v1/taint/scan" && method == "POST":
		api.handleTaintScan(w, r)
	case path == "/api/v1/taint/cleanup" && method == "POST":
		api.handleTaintCleanup(w, r)
	case strings.HasPrefix(path, "/api/v1/taint/entry/") && method == "DELETE":
		api.handleTaintEntryDelete(w, r)
	case path == "/api/v1/taint/inject" && method == "POST":
		api.handleTaintInject(w, r)

	// v20.2: 污染链逆转 API
	case path == "/api/v1/reversal/stats" && method == "GET":
		api.handleReversalStats(w, r)
	case path == "/api/v1/reversal/records" && method == "GET":
		api.handleReversalRecords(w, r)
	case path == "/api/v1/reversal/templates" && method == "GET":
		api.handleReversalTemplates(w, r)
	case path == "/api/v1/reversal/templates" && method == "POST":
		api.handleReversalTemplatesAdd(w, r)
	case path == "/api/v1/reversal/config" && method == "GET":
		api.handleReversalConfigGet(w, r)
	case path == "/api/v1/reversal/config" && method == "PUT":
		api.handleReversalConfigUpdate(w, r)
	case path == "/api/v1/reversal/test" && method == "POST":
		api.handleReversalTest(w, r)

	// v20.3 LLM 响应缓存
	case path == "/api/v1/cache/stats" && method == "GET":
		api.handleCacheStats(w, r)
	case path == "/api/v1/cache/entries" && method == "GET":
		api.handleCacheEntries(w, r)
	case path == "/api/v1/cache/entries" && method == "DELETE":
		api.handleCacheEntriesDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/cache/tenant/") && method == "DELETE":
		api.handleCacheTenantDelete(w, r)
	case path == "/api/v1/cache/config" && method == "GET":
		api.handleCacheConfigGet(w, r)
	case path == "/api/v1/cache/config" && method == "PUT":
		api.handleCacheConfigUpdate(w, r)
	case path == "/api/v1/cache/lookup" && method == "POST":
		api.handleCacheLookup(w, r)

	// v20.4 API Gateway
	case path == "/api/v1/gateway/stats" && method == "GET":
		api.handleGatewayStats(w, r)
	case path == "/api/v1/gateway/routes" && method == "GET":
		api.handleGatewayRouteList(w, r)
	case path == "/api/v1/gateway/routes" && method == "POST":
		api.handleGatewayRouteAdd(w, r)
	case strings.HasPrefix(path, "/api/v1/gateway/routes/") && method == "PUT":
		api.handleGatewayRouteUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/gateway/routes/") && method == "DELETE":
		api.handleGatewayRouteDelete(w, r)
	case path == "/api/v1/gateway/config" && method == "GET":
		api.handleGatewayConfigGet(w, r)
	case path == "/api/v1/gateway/config" && method == "PUT":
		api.handleGatewayConfigUpdate(w, r)
	case path == "/api/v1/gateway/token" && method == "POST":
		api.handleGatewayTokenGenerate(w, r)
	case path == "/api/v1/gateway/validate" && method == "POST":
		api.handleGatewayTokenValidate(w, r)
	case path == "/api/v1/gateway/log" && method == "GET":
		api.handleGatewayLog(w, r)

	// v23.0: 路径级策略引擎 API
	case path == "/api/v1/path-policies" && method == "GET":
		api.handlePathPolicyList(w, r)
	case path == "/api/v1/path-policies" && method == "POST":
		api.handlePathPolicyCreate(w, r)
	case path == "/api/v1/path-policies/events" && method == "GET":
		api.handlePathPolicyEvents(w, r)
	case path == "/api/v1/path-policies/contexts" && method == "GET":
		api.handlePathPolicyContexts(w, r)
	case path == "/api/v1/path-policies/stats" && method == "GET":
		api.handlePathPolicyStats(w, r)
	case path == "/api/v1/path-policies/risk-gauge" && method == "GET":
		api.handlePathPolicyRiskGauge(w, r)
	case path == "/api/v1/path-policies/templates" && method == "GET":
		api.handlePathPolicyTemplates(w, r)
	case path == "/api/v1/path-policies/templates" && method == "POST":
		api.handlePathPolicyTemplateCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/templates/") && strings.HasSuffix(path, "/activate") && method == "POST":
		api.handlePathPolicyTemplateActivate(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/templates/") && strings.HasSuffix(path, "/deactivate") && method == "POST":
		api.handlePathPolicyTemplateDeactivate(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/templates/") && method == "PUT":
		api.handlePathPolicyTemplateUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/templates/") && method == "DELETE":
		api.handlePathPolicyTemplateDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/contexts/") && method == "GET":
		api.handlePathPolicyContextDetail(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/") && method == "PUT":
		api.handlePathPolicyUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/path-policies/") && method == "DELETE":
		api.handlePathPolicyDelete(w, r)

	// v24.0: 反事实验证引擎 API
	case path == "/api/v1/counterfactual/stats" && method == "GET":
		api.handleCFStats(w, r)
	case path == "/api/v1/counterfactual/verifications" && method == "GET":
		api.handleCFVerifications(w, r)
	case path == "/api/v1/counterfactual/config" && method == "GET":
		api.handleCFConfigGet(w, r)
	case path == "/api/v1/counterfactual/config" && method == "PUT":
		api.handleCFConfigUpdate(w, r)
	case path == "/api/v1/counterfactual/cache" && method == "GET":
		api.handleCFCacheGet(w, r)
	case path == "/api/v1/counterfactual/cache" && method == "DELETE":
		api.handleCFCacheClear(w, r)
	// 高风险工具 CRUD API
	case path == "/api/v1/counterfactual/high-risk-tools" && method == "GET":
		api.handleCFHighRiskToolsList(w, r)
	case path == "/api/v1/counterfactual/high-risk-tools" && method == "POST":
		api.handleCFHighRiskToolsAdd(w, r)
	case strings.HasPrefix(path, "/api/v1/counterfactual/high-risk-tools/") && method == "DELETE":
		api.handleCFHighRiskToolsDelete(w, r)
	// v24.1: 归因报告 API
	case path == "/api/v1/counterfactual/reports" && method == "GET":
		api.handleCFReports(w, r)
	case path == "/api/v1/counterfactual/timeline" && method == "GET":
		api.handleCFTimeline(w, r)
	case strings.HasPrefix(path, "/api/v1/counterfactual/reports/") && method == "GET":
		api.handleCFReportGet(w, r)
	case strings.HasPrefix(path, "/api/v1/counterfactual/verifications/") && method == "GET":
		api.handleCFVerificationGet(w, r)

	// v24.2: 自适应验证策略 API
	case path == "/api/v1/counterfactual/cost" && method == "GET":
		api.handleCFAdaptiveCost(w, r)
	case path == "/api/v1/counterfactual/effectiveness" && method == "GET":
		api.handleCFAdaptiveEffectiveness(w, r)
	case path == "/api/v1/counterfactual/feedback" && method == "POST":
		api.handleCFAdaptiveFeedback(w, r)
	case path == "/api/v1/counterfactual/adaptive-config" && method == "GET":
		api.handleCFAdaptiveConfigGet(w, r)
	case path == "/api/v1/counterfactual/adaptive-config" && method == "PUT":
		api.handleCFAdaptiveConfigUpdate(w, r)

	// v25.0: 执行计划编译器 API
	case path == "/api/v1/plans/templates" && method == "GET":
		api.handlePlanTemplatesList(w, r)
	case path == "/api/v1/plans/templates" && method == "POST":
		api.handlePlanTemplatesCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/plans/templates/") && method == "PUT":
		api.handlePlanTemplatesUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/plans/templates/") && method == "DELETE":
		api.handlePlanTemplatesDelete(w, r)
	case path == "/api/v1/plans/active" && method == "GET":
		api.handlePlanActive(w, r)
	case path == "/api/v1/plans/history" && method == "GET":
		api.handlePlanHistory(w, r)
	case path == "/api/v1/plans/violations" && method == "GET":
		api.handlePlanViolations(w, r)
	case path == "/api/v1/plans/stats" && method == "GET":
		api.handlePlanStats(w, r)
	case path == "/api/v1/plans/config" && method == "PUT":
		api.handlePlanConfigUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/plans/") && method == "GET":
		api.handlePlanGet(w, r)

	// v25.0: PlanCompiler 手动操作
	case path == "/api/v1/plans/compile" && method == "POST":
		api.handlePlanCompile(w, r)
	case path == "/api/v1/plans/evaluate" && method == "POST":
		api.handlePlanEvaluate(w, r)

	// v25.1: Capability 权限系统
	case path == "/api/v1/capabilities/contexts" && method == "POST":
		api.handleCapInitContext(w, r)
	case path == "/api/v1/capabilities/mappings" && method == "GET":
		api.handleCapMappingsList(w, r)
	case strings.HasPrefix(path, "/api/v1/capabilities/mappings/") && method == "PUT":
		api.handleCapMappingsUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/capabilities/mappings/") && method == "DELETE":
		api.handleCapMappingsDelete(w, r)
	case path == "/api/v1/capabilities/contexts" && method == "GET":
		api.handleCapContexts(w, r)
	case strings.HasPrefix(path, "/api/v1/capabilities/contexts/") && method == "GET":
		api.handleCapContextGet(w, r)
	case strings.HasPrefix(path, "/api/v1/capabilities/contexts/") && method == "PUT":
		api.handleCapContextUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/capabilities/contexts/") && method == "DELETE":
		api.handleCapContextDelete(w, r)
	case path == "/api/v1/capabilities/evaluations" && method == "GET":
		api.handleCapEvaluations(w, r)
	case path == "/api/v1/capabilities/stats" && method == "GET":
		api.handleCapStats(w, r)

	// v25.2: Plan 偏差检测
	case path == "/api/v1/deviations/check" && method == "POST":
		api.handleDeviationsCheck(w, r)
	case path == "/api/v1/deviations" && method == "GET":
		api.handleDeviationsList(w, r)
	case path == "/api/v1/deviations/stats" && method == "GET":
		api.handleDeviationsStats(w, r)
	case path == "/api/v1/deviations/config" && method == "GET":
		api.handleDeviationsConfigGet(w, r)
	case path == "/api/v1/deviations/config" && method == "PUT":
		api.handleDeviationsConfigUpdate(w, r)
	case path == "/api/v1/deviations/repair-policies" && method == "GET":
		api.handleRepairPoliciesList(w, r)
	case path == "/api/v1/deviations/repair-policies" && method == "POST":
		api.handleRepairPoliciesCreate(w, r)
	case strings.HasPrefix(path, "/api/v1/deviations/repair-policies/") && method == "PUT":
		api.handleRepairPoliciesUpdate(w, r)
	case strings.HasPrefix(path, "/api/v1/deviations/repair-policies/") && method == "DELETE":
		api.handleRepairPoliciesDelete(w, r)
	case strings.HasPrefix(path, "/api/v1/deviations/") && method == "GET":
		api.handleDeviationsDetail(w, r)

	// v31.0: AC 智能分级（自动复核）API
	case path == "/api/v1/auto-review/status" && method == "GET":
		api.handleAutoReviewStatus(w, r)
	case path == "/api/v1/auto-review/config" && method == "POST":
		api.handleAutoReviewConfig(w, r)
	case path == "/api/v1/auto-review/stats" && method == "GET":
		api.handleAutoReviewStats(w, r)
	case strings.HasPrefix(path, "/api/v1/auto-review/rules/") && strings.HasSuffix(path, "/review") && method == "POST":
		api.handleAutoReviewSetReview(w, r)
	case strings.HasPrefix(path, "/api/v1/auto-review/rules/") && strings.HasSuffix(path, "/restore") && method == "POST":
		api.handleAutoReviewRestore(w, r)

	default:
		// v26.0: IFC 信息流控制
		if api.ifcEngine != nil && api.handleIFC(w, r, path, method) {
			return
		}
		w.WriteHeader(404)
	}
}

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
		// R2-001: 用户不存在返回 404 而非 500
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
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
func (api *ManagementAPI) persistInboundRules(configs []InboundRuleConfig) {
	if api.cfg.InboundRulesFile == "" {
		return
	}
	rulesFile := InboundRulesFileConfig{Rules: configs}
	data, err := yaml.Marshal(&rulesFile)
	if err != nil {
		log.Printf("[规则CRUD] 序列化规则失败: %v", err)
		return
	}
	header := "# lobster-guard 入站规则文件\n# 由 Dashboard 自动保存\n\n"
	if err := os.WriteFile(api.cfg.InboundRulesFile, []byte(header+string(data)), 0644); err != nil {
		log.Printf("[规则CRUD] 写入规则文件失败: %v", err)
	} else {
		log.Printf("[规则CRUD] 规则已持久化到 %s", api.cfg.InboundRulesFile)
	}
}

// handleAddInboundRule POST /api/v1/inbound-rules/add — 添加入站规则
func (api *ManagementAPI) handleOWASPMatrix(w http.ResponseWriter, r *http.Request) {
	if api.owaspMatrixEng == nil {
		jsonResponse(w, 500, map[string]string{"error": "OWASP matrix engine not available"})
		return
	}
	since := r.URL.Query().Get("since")
	sinceTime := parseSinceParam(since)
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	items := api.owaspMatrixEng.CalculateWithFilterTenant(sinceTime, tenantID)
	timeRange := since
	if timeRange == "" {
		timeRange = "24h"
	}
	jsonResponse(w, 200, map[string]interface{}{"items": items, "total": len(items), "time_range": timeRange, "tenant": tenantID})
}

// handleStrictModeGet GET /api/v1/system/strict-mode — 获取严格模式状态
func NewBackgroundScheduler(cfg *Config, chainEng *AttackChainEngine, behaviorEng *BehaviorProfileEngine) *BackgroundScheduler {
	chainMin := cfg.ChainAnalysisIntervalMin
	if chainMin <= 0 {
		chainMin = 5
	}
	behaviorMin := cfg.BehaviorScanIntervalMin
	if behaviorMin <= 0 {
		behaviorMin = 10
	}
	return &BackgroundScheduler{
		attackChainEng:     chainEng,
		behaviorProfileEng: behaviorEng,
		chainInterval:      time.Duration(chainMin) * time.Minute,
		behaviorInterval:   time.Duration(behaviorMin) * time.Minute,
	}
}

// Start 启动后台调度 goroutine，需要传入一个可取消的 context
func (s *BackgroundScheduler) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// 攻击链自动分析
	if s.attackChainEng != nil {
		go func() {
			ticker := time.NewTicker(s.chainInterval)
			defer ticker.Stop()
			for {
				select {
				case <-childCtx.Done():
					return
				case <-ticker.C:
					chains, err := s.attackChainEng.AnalyzeChains("default", 1)
					if err != nil {
						log.Printf("[调度器] 攻击链分析失败: %v", err)
					} else if len(chains) > 0 {
						log.Printf("[调度器] 攻击链分析完成: 发现 %d 条新链", len(chains))
					}
				}
			}
		}()
	}

	// 行为画像自动扫描
	if s.behaviorProfileEng != nil {
		go func() {
			ticker := time.NewTicker(s.behaviorInterval)
			defer ticker.Stop()
			for {
				select {
				case <-childCtx.Done():
					return
				case <-ticker.C:
					scanned, anomalies := s.behaviorProfileEng.ScanAllActive("default")
					if anomalies > 0 {
						log.Printf("[调度器] 行为画像扫描完成: 扫描 %d 个 Agent, 发现 %d 个异常", scanned, anomalies)
					}
				}
			}
		}()
	}
}

// Stop 停止后台调度器
func (s *BackgroundScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// ============================================================
// v17.3: 会话关联配置与状态 API
// ============================================================

// handleSessionCorrelatorStats GET /api/v1/session-correlator/stats — 返回会话关联器运行状态
