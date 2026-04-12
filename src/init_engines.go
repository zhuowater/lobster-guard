// init_engines.go — 安全引擎集中初始化（从 main.go 拆分）
// P2 重构：保持行为不变，消除 main() 中 800 行扁平初始化
package main

import (
	"fmt"
	"log"
	"time"
)

// EngineSet 所有安全引擎的集合，供 main() 一次性初始化后注入各组件
type EngineSet struct {
	// 核心基础设施
	Metrics         *MetricsCollector
	RuleHits        *RuleHitStats
	Realtime        *RealtimeMetrics
	UserCache       *UserInfoCache
	PolicyEng       *RoutePolicyEngine
	TraceCorrelator *TraceCorrelator
	SessionCorr     *SessionCorrelator
	AlertNotifier   *AlertNotifier
	AutoReviewMgr   *AutoReviewManager

	// 检测 & 审计
	SessionDetector *SessionDetector
	LLMDetector     *LLMDetector
	DetectCache     *DetectCache
	HoneypotEngine  *HoneypotEngine
	HoneypotDeep    *HoneypotDeepEngine
	SemanticDetect  *SemanticDetector
	AnomalyDetect   *AnomalyDetector

	// 安全治理
	EventBus          *EventBus
	EnvelopeMgr       *EnvelopeManager
	EvolutionEngine   *EvolutionEngine
	AdaptiveEngine    *AdaptiveDecisionEngine
	SingularityEngine *SingularityEngine
	PathPolicyEngine  *PathPolicyEngine
	CFVerifier        *CounterfactualVerifier
	AdaptiveStrategy  *AdaptiveStrategy

	// CaMeL 框架
	PlanCompiler      *PlanCompiler
	CapabilityEngine  *CapabilityEngine
	DeviationDetector *DeviationDetector

	// IFC
	IFCEngine     *IFCEngine
	IFCQuarantine *IFCQuarantine

	// 数据安全
	TaintTracker   *TaintTracker
	ReversalEngine *TaintReversalEngine
	LLMCache       *LLMCache

	// 分析 & 报告
	UserProfileEng     *UserProfileEngine
	HealthScoreEng     *HealthScoreEngine
	OwaspMatrixEng     *OWASPMatrixEngine
	SessionReplayEng   *SessionReplayEngine
	PromptTracker      *PromptTracker
	RedTeamEngine      *RedTeamEngine
	LeaderboardEng     *LeaderboardEngine
	ReportEngine       *ReportEngine
	BehaviorProfile    *BehaviorProfileEngine
	AttackChainEng     *AttackChainEngine
	ABTestEngine       *ABTestEngine
	UpstreamProfileEng *UpstreamProfileEngine

	// 工具 & 网关
	ToolPolicy   *ToolPolicyEngine
	APIGateway   *APIGateway
	APIKeyMgr    *APIKeyManager
	K8sDiscovery *K8sDiscovery

	// 运维
	SuggestionQueue *SuggestionQueue
	CanaryRotator   *CanaryRotator
	ReportScheduler *ReportScheduler
	LayoutStore     *LayoutStore
	NotificationEng *NotificationEngine
	StrictMode      *StrictModeManager
	BgScheduler     *BackgroundScheduler
}

func applySourceClassifierConfig(cfg *Config) {
	if cfg == nil {
		SetDefaultToolSourceClassifierConfig(ToolSourceClassifierConfig{})
		return
	}
	SetDefaultToolSourceClassifierConfig(cfg.SourceClassifier)
}

// initAllEngines 集中初始化所有安全引擎，返回 EngineSet
// 保持原始初始化顺序和依赖关系
func initAllEngines(cfg *Config, store *SQLiteStore, logger *AuditLogger, pool *UpstreamPool, routes *RouteTable, engine *RuleEngine, outboundEngine *OutboundRuleEngine, llmRuleEngine *LLMRuleEngine, llmAuditor *LLMAuditor, llmProxy *LLMProxy, tenantMgr *TenantManager, honeypotEngine *HoneypotEngine) *EngineSet {
	applySourceClassifierConfig(cfg)
	if llmAuditor != nil {
		llmAuditor.tenantMgr = tenantMgr
	}
	e := &EngineSet{}
	db := logger.DB()

	// --- Metrics & 实时监控 ---
	if cfg.IsMetricsEnabled() {
		e.Metrics = NewMetricsCollector()
	}
	e.RuleHits = NewRuleHitStats()
	e.Realtime = NewRealtimeMetrics()

	// --- 用户信息缓存 + 路由策略 ---
	provider := createUserInfoProvider(cfg)
	if provider != nil {
		e.UserCache = NewUserInfoCache(db, provider, 24*time.Hour)
	}
	if len(cfg.RoutePolicies) > 0 {
		e.PolicyEng = NewRoutePolicyEngine(cfg.RoutePolicies)
	}

	// --- Trace & Session 关联 ---
	e.TraceCorrelator = NewTraceCorrelator(10000)
	sessionIdleMs := int64(60 * 60 * 1000)
	if cfg.SessionIdleTimeoutMin > 0 {
		sessionIdleMs = int64(cfg.SessionIdleTimeoutMin) * 60 * 1000
	}
	sessionFPMs := int64(5 * 60 * 1000)
	if cfg.SessionFPWindowSec > 0 {
		sessionFPMs = int64(cfg.SessionFPWindowSec) * 1000
	}
	e.SessionCorr = NewSessionCorrelator(50000, sessionIdleMs)
	e.SessionCorr.fpWindowMs = sessionFPMs
	if llmProxy != nil {
		llmProxy.sessionCorrelator = e.SessionCorr
	}

	// --- 蜜罐引擎 ---
	e.HoneypotEngine = honeypotEngine
	fmt.Println("[初始化] ✅ 蜜罐引擎已就绪 (Agent 蜜罐: 假数据+水印追踪+引爆检测)")

	// --- 告警通知 ---
	if cfg.AlertWebhook != "" {
		e.AlertNotifier = NewAlertNotifier(cfg.AlertWebhook, cfg.AlertFormat, cfg.AlertMinInterval, e.Metrics)
	}

	// --- AC 智能分级 ---
	e.AutoReviewMgr = NewAutoReviewManager(cfg.AutoReview, pool)
	if cfg.LLMProxy.Enabled && len(cfg.LLMProxy.Targets) > 0 {
		e.AutoReviewMgr.llmTargets = cfg.LLMProxy.Targets
	}
	engine.autoReviewMgr = e.AutoReviewMgr
	if llmRuleEngine != nil {
		llmRuleEngine.autoReviewMgr = e.AutoReviewMgr
	}
	if cfg.AutoReview.Enabled {
		e.AutoReviewMgr.Start()
		fmt.Printf("[初始化] ✅ AC 智能分级已启用 (窗口=%ds, 阈值=%d, TTL=%ds)\n",
			cfg.AutoReview.WindowSeconds, cfg.AutoReview.SpikeThreshold, cfg.AutoReview.AutoReviewTTL)
	} else {
		fmt.Println("[初始化] ⚠️ AC 智能分级: 未启用 (auto_review.enabled=false)")
	}

	// --- 会话检测器 ---
	// 总是初始化，运行时靠 cfg.SessionDetectEnabled 控制
	{
		e.SessionDetector = NewSessionDetector(SessionDetectorConfig{
			Enabled: cfg.SessionDetectEnabled,
			RiskThreshold: func() float64 {
				if cfg.SessionRiskThreshold > 0 {
					return cfg.SessionRiskThreshold
				}
				return 10
			}(),
			Window: func() int {
				if cfg.SessionWindow > 0 {
					return cfg.SessionWindow
				}
				return 20
			}(),
			DecayRate: func() float64 {
				if cfg.SessionDecayRate > 0 {
					return cfg.SessionDecayRate
				}
				return 1
			}(),
		})
		fmt.Printf("[初始化] ✅ 会话检测: threshold=%.0f, window=%d, decay_rate=%.1f/h\n",
			e.SessionDetector.cfg.RiskThreshold, e.SessionDetector.cfg.Window, e.SessionDetector.cfg.DecayRate)
	}

	// --- LLM 检测器 ---
	// 总是初始化，运行时靠 cfg.LLMDetectEnabled 控制
	{
		e.LLMDetector = NewLLMDetector(LLMDetectorConfig{
			Enabled: cfg.LLMDetectEnabled, Endpoint: cfg.LLMDetectEndpoint, APIKey: cfg.LLMDetectAPIKey,
			Model: cfg.LLMDetectModel, Timeout: cfg.LLMDetectTimeout, Mode: cfg.LLMDetectMode, Prompt: cfg.LLMDetectPrompt,
		})
		fmt.Printf("[初始化] ✅ LLM 检测: endpoint=%s, model=%s, mode=%s\n",
			cfg.LLMDetectEndpoint, e.LLMDetector.cfg.Model, e.LLMDetector.cfg.Mode)
	}

	// --- 检测缓存 ---
	cacheTTL := cfg.DetectCacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 300
	}
	cacheSize := cfg.DetectCacheSize
	if cacheSize <= 0 {
		cacheSize = 1000
	}
	e.DetectCache = NewDetectCache(cacheSize, time.Duration(cacheTTL)*time.Second)
	fmt.Printf("[初始化] ✅ 检测缓存: size=%d, ttl=%ds\n", cacheSize, cacheTTL)

	// --- 分析引擎群 ---
	e.UserProfileEng = NewUserProfileEngine(db)
	e.HealthScoreEng = NewHealthScoreEngine(db)
	e.OwaspMatrixEng = NewOWASPMatrixEngine(db, llmRuleEngine)
	e.StrictMode = NewStrictModeManager(engine, llmRuleEngine)
	e.NotificationEng = NewNotificationEngine(db)

	e.AnomalyDetect = NewAnomalyDetector(db)
	e.AnomalyDetect.StartBackground()
	fmt.Println("[初始化] ✅ 异常基线检测器已启动 (6 个指标, 7 天窗口, >2σ 告警)")

	e.SessionReplayEng = NewSessionReplayEngine(db)
	fmt.Println("[初始化] ✅ 会话回放引擎已就绪 (trace_id 串联 IM+LLM+Tools)")

	e.PromptTracker = NewPromptTracker(db)
	if llmAuditor != nil {
		llmAuditor.promptTracker = e.PromptTracker
	}
	fmt.Println("[初始化] ✅ Prompt 版本追踪器已就绪 (自动检测 System Prompt 变化)")

	// --- Red Team ---
	e.RedTeamEngine = NewRedTeamEngine(db, engine)
	e.RedTeamEngine.outboundEngine = outboundEngine
	e.RedTeamEngine.llmRuleEngine = llmRuleEngine
	e.RedTeamEngine.honeypotEngine = honeypotEngine
	fmt.Println("[初始化] ✅ Red Team Autopilot 引擎已就绪 (35 攻击向量, OWASP LLM Top10)")

	e.LeaderboardEng = NewLeaderboardEngine(db, tenantMgr, e.HealthScoreEng)
	fmt.Println("[初始化] ✅ 安全排行榜 + SLA 基线引擎已就绪 (排行榜/热力图/SLA 达标)")

	e.ReportEngine = NewReportEngine(db, "/var/lib/lobster-guard/reports/")
	e.ReportEngine.SetEngines(e.HealthScoreEng, e.OwaspMatrixEng, llmAuditor, e.UserProfileEng, e.AnomalyDetect, logger)

	e.ABTestEngine = NewABTestEngine(db)
	fmt.Println("[初始化] ✅ A/B 测试引擎已就绪 (Prompt A/B 测试 + 流量分配 + 统计显著性)")

	e.BehaviorProfile = NewBehaviorProfileEngine(db)
	e.UpstreamProfileEng = NewUpstreamProfileEngine(db)
	fmt.Println("[初始化] ✅ 上游安全画像引擎已就绪 (5维评分 + 14表聚合 + 7天趋势)")
	fmt.Println("[初始化] ✅ Agent 行为画像引擎已就绪 (语义行为模式 + 突变检测 + 风险评估)")

	e.AttackChainEng = NewAttackChainEngine(db)
	fmt.Println("[初始化] ✅ 攻击链引擎已就绪 (跨 Agent 关联分析 + 模式匹配 + 风险评分)")

	e.LayoutStore = NewLayoutStore(db)
	fmt.Println("[初始化] ✅ 布局引擎已就绪 (面板拖拽 + 折叠 + 预设模板)")

	// --- 事件总线 ---
	// 总是初始化，运行时靠 cfg.EventBus.Enabled 控制
	e.EventBus = NewEventBus(db, cfg)
	fmt.Printf("[初始化] ✅ 事件总线已就绪 (enabled=%v)\n", cfg.EventBus.Enabled)

	// --- 执行信封 ---
	// 总是初始化（需要 SecretKey），运行时靠 cfg.EnvelopeEnabled 控制
	{
		batchSize := cfg.EnvelopeBatchSize
		if batchSize <= 0 {
			batchSize = 64
		}
		secretKey := cfg.EnvelopeSecretKey
		if secretKey == "" {
			secretKey = "lobster-guard-default-key"
		}
		e.EnvelopeMgr = NewEnvelopeManagerWithBatchSize(db, secretKey, batchSize)
		e.EnvelopeMgr.startAutoFlush()
		fmt.Printf("[初始化] ✅ 执行信封已就绪 (enabled=%v)\n", cfg.EnvelopeEnabled)
	}

	// --- 事件总线回调 ---
	if e.EventBus != nil {
		e.PromptTracker.eventBus = e.EventBus
		e.PromptTracker.driftThreshold = 0.5
		if e.StrictMode != nil {
			e.EventBus.strictModeFunc = func(enable bool) error {
				e.StrictMode.SetEnabled(enable)
				return nil
			}
		}
	}

	// --- 对抗性自进化 ---
	// 总是初始化，运行时靠 cfg.EvolutionEnabled 控制
	e.EvolutionEngine = NewEvolutionEngine(db, e.RedTeamEngine, engine, outboundEngine, llmRuleEngine, e.EventBus)
	if cfg.EvolutionEnabled {
		intervalMin := cfg.EvolutionIntervalMin
		if intervalMin <= 0 {
			intervalMin = 360
		}
		e.EvolutionEngine.StartAutoEvolution(intervalMin)
		fmt.Printf("[初始化] ✅ 对抗性自进化已启用 (每 %d 分钟)\n", intervalMin)
	} else {
		fmt.Printf("[初始化] ✅ 对抗性自进化已就绪 (enabled=%v)\n", cfg.EvolutionEnabled)
	}

	// --- 规则建议队列 ---
	e.SuggestionQueue = NewSuggestionQueue(db, engine)
	if e.EvolutionEngine != nil {
		e.EvolutionEngine.SetSuggestionQueue(e.SuggestionQueue)
	}
	fmt.Println("[初始化] ✅ 规则建议队列已启用")

	// --- 自适应决策 ---
	// 总是初始化，运行时靠 cfg.AdaptiveDecision.Enabled 控制
	e.AdaptiveEngine = NewAdaptiveDecisionEngine(db, e.EnvelopeMgr, cfg.AdaptiveDecision)
	fmt.Printf("[初始化] ✅ 自适应决策引擎已就绪 (enabled=%v)\n", cfg.AdaptiveDecision.Enabled)

	// --- 奇点蜜罐 ---
	// 总是初始化，运行时靠 cfg.Singularity.Enabled 控制
	e.SingularityEngine = NewSingularityEngine(db, honeypotEngine, e.EnvelopeMgr, cfg.Singularity)
	fmt.Printf("[初始化] ✅ 奇点蜜罐已就绪 (enabled=%v)\n", cfg.Singularity.Enabled)

	// --- 蜜罐深度交互 ---
	// 总是初始化，运行时靠 cfg.HoneypotDeep.Enabled 控制
	e.HoneypotDeep = NewHoneypotDeepEngine(db, honeypotEngine, e.EvolutionEngine, e.EventBus, cfg.HoneypotDeep)
	fmt.Printf("[初始化] ✅ 蜜罐深度交互引擎已就绪 (enabled=%v)\n", cfg.HoneypotDeep.Enabled)

	// --- 语义检测 ---
	// 总是初始化，运行时靠 cfg.SemanticDetector.Enabled 控制
	e.SemanticDetect = NewSemanticDetector(db, cfg.SemanticDetector)
	fmt.Printf("[初始化] ✅ 语义检测引擎已就绪 (enabled=%v, 模式库=%d)\n", cfg.SemanticDetector.Enabled, len(e.SemanticDetect.attackVectors))

	// --- 路径级策略 ---
	e.PathPolicyEngine = NewPathPolicyEngine(db)
	e.PathPolicyEngine.SetUserProfileEngine(e.UserProfileEng)
	if cfg.PathPolicy.Enabled {
		fmt.Println("[初始化] ✅ 路径级策略引擎已启用 (路径追踪 + 序列/累计/降级规则 + 画像联动)")
	} else {
		fmt.Println("[初始化] ✅ 路径级策略引擎已就绪 (路径追踪 + 序列/累计/降级规则 + 画像联动)")
	}

	// --- 反事实验证 ---
	cfConfig := defaultCFConfig
	if cfg.Counterfactual.Enabled {
		cfConfig = cfg.Counterfactual
	}
	e.CFVerifier = NewCounterfactualVerifier(db, cfConfig, nil)
	e.CFVerifier.SetPathPolicy(e.PathPolicyEngine)
	if llmProxy != nil {
		llmProxy.cfVerifier = e.CFVerifier
	}
	fmt.Println("[初始化] ✅ 反事实验证引擎已就绪 (AttriGuard 对照验证 + 归因分析)")

	e.AdaptiveStrategy = NewAdaptiveStrategy(db, defaultAdaptiveConfig, e.PathPolicyEngine)
	e.CFVerifier.SetAdaptiveStrategy(e.AdaptiveStrategy)
	fmt.Println("[初始化] ✅ 自适应验证策略引擎已就绪 (成本控制 + 优先级调度 + 效果追踪)")

	// --- CaMeL 框架 ---
	planCfg := defaultPlanConfig
	if cfg.PlanCompiler.Enabled {
		planCfg = cfg.PlanCompiler
	}
	e.PlanCompiler = NewPlanCompiler(db, planCfg)
	if llmProxy != nil {
		llmProxy.planCompiler = e.PlanCompiler
	}
	fmt.Println("[初始化] ✅ 执行计划编译器已就绪 (CaMeL 网关级程序解释器, 20+ 内置模板)")

	capCfg := cfg.Capability
	if capCfg.DefaultPolicy == "" {
		capCfg.DefaultPolicy = "allow"
	}
	e.CapabilityEngine = NewCapabilityEngine(db, capCfg)
	if llmProxy != nil {
		llmProxy.capabilityEngine = e.CapabilityEngine
	}
	fmt.Println("[初始化] ✅ Capability 权限系统已就绪")

	devCfg := cfg.Deviation
	if devCfg.MaxRepairs == 0 {
		devCfg.MaxRepairs = 5
	}
	e.DeviationDetector = NewDeviationDetector(db, devCfg, e.PlanCompiler, e.CapabilityEngine)
	if llmProxy != nil {
		llmProxy.deviationDetector = e.DeviationDetector
	}
	fmt.Println("[初始化] ✅ 偏差检测器已就绪")

	// --- IFC ---
	e.IFCEngine = NewIFCEngine(db, cfg.IFC)
	if llmProxy != nil {
		llmProxy.ifcEngine = e.IFCEngine
	}
	fmt.Printf("[初始化] ✅ IFC 信息流控制已就绪 (来源规则=%d, 工具要求=%d, 隔离=%v, 隐藏=%v)\n",
		len(e.IFCEngine.ListSourceRules()), len(e.IFCEngine.ListToolRequirements()),
		cfg.IFC.QuarantineEnabled, cfg.IFC.HidingEnabled)

	// 总是初始化，运行时靠 cfg.IFC.QuarantineEnabled 控制
	e.IFCQuarantine = NewIFCQuarantine(e.IFCEngine, pool)
	if llmProxy != nil {
		llmProxy.ifcQuarantine = e.IFCQuarantine
	}
	fmt.Printf("[初始化] ✅ IFC 隔离LLM已就绪 (enabled=%v)\n", cfg.IFC.QuarantineEnabled)

	if llmProxy != nil {
		llmProxy.auditLogger = logger
	}

	// --- 工具策略 ---
	// 总是初始化，运行时靠 cfg.ToolPolicy.Enabled 控制
	e.ToolPolicy = NewToolPolicyEngine(db, cfg.ToolPolicy)
	fmt.Printf("[初始化] ✅ 工具策略引擎已就绪 (enabled=%v, 规则=%d)\n", cfg.ToolPolicy.Enabled, len(e.ToolPolicy.ListRules()))
	if llmProxy != nil {
		llmProxy.toolPolicy = e.ToolPolicy
		llmProxy.pathPolicyEngine = e.PathPolicyEngine
	}

	// --- 污染追踪 ---
	// 总是初始化，运行时靠 cfg.TaintTracker.Enabled 控制
	e.TaintTracker = NewTaintTracker(db, cfg.TaintTracker)
	e.TaintTracker.SetIFCEngine(e.IFCEngine)
	fmt.Printf("[初始化] ✅ 信息流污染追踪已就绪 (enabled=%v)\n", cfg.TaintTracker.Enabled)
	if llmProxy != nil {
		llmProxy.taintTracker = e.TaintTracker
	}

	// --- 污染链逆转 ---
	// 总是初始化，运行时靠 cfg.TaintReversal.Enabled 控制
	e.ReversalEngine = NewTaintReversalEngine(db, e.TaintTracker, e.EnvelopeMgr, cfg.TaintReversal)
	fmt.Printf("[初始化] ✅ 污染链逆转已就绪 (enabled=%v)\n", cfg.TaintReversal.Enabled)
	if llmProxy != nil {
		llmProxy.reversalEngine = e.ReversalEngine
	}

	// --- LLM 缓存 ---
	// 总是初始化，运行时靠 cfg.LLMCache.Enabled 控制
	e.LLMCache = NewLLMCache(db, cfg.LLMCache)
	fmt.Printf("[初始化] ✅ LLM 响应缓存已就绪 (enabled=%v)\n", cfg.LLMCache.Enabled)
	if llmProxy != nil {
		llmProxy.llmCache = e.LLMCache
	}

	// --- API Gateway ---
	// 总是初始化，运行时靠 cfg.APIGateway.Enabled 控制
	e.APIGateway = NewAPIGateway(db, cfg.APIGateway)
	fmt.Printf("[初始化] ✅ API Gateway 已就绪 (enabled=%v)\n", cfg.APIGateway.Enabled)
	if llmProxy != nil {
		llmProxy.apiGateway = e.APIGateway
	}

	// --- K8s ---
	if cfg.Discovery.Kubernetes.Enabled {
		var err error
		e.K8sDiscovery, err = NewK8sDiscovery(cfg, pool)
		if err != nil {
			log.Printf("[K8s发现] ⚠️ 初始化失败（将继续运行但不启用发现）: %v", err)
		} else {
			fmt.Printf("[初始化] ✅ K8s 服务发现: namespace=%s, service=%s, interval=%ds\n",
				cfg.Discovery.Kubernetes.Namespace, cfg.Discovery.Kubernetes.Service,
				func() int {
					if cfg.Discovery.Kubernetes.SyncInterval > 0 {
						return cfg.Discovery.Kubernetes.SyncInterval
					}
					return 15
				}())
		}
	} else {
		fmt.Println("[初始化] ⚠️ K8s 服务发现: 未启用")
	}

	// --- API Key ---
	e.APIKeyMgr = NewAPIKeyManager(db)
	fmt.Printf("[初始化] ✅ 租户策略闭环 + API Key 管理器已就绪\n")

	// --- 金丝雀 + 报告定时 ---
	// 这两个需要 mgmtAPI，延迟到 wireEngines 中初始化

	// --- 后台调度器 ---
	e.BgScheduler = NewBackgroundScheduler(cfg, e.AttackChainEng, e.BehaviorProfile)
	chainMin := cfg.ChainAnalysisIntervalMin
	if chainMin <= 0 {
		chainMin = 5
	}
	behaviorMin := cfg.BehaviorScanIntervalMin
	if behaviorMin <= 0 {
		behaviorMin = 10
	}
	fmt.Printf("[初始化] ✅ 后台调度器已就绪 (攻击链分析: %d 分钟, 行为画像扫描: %d 分钟)\n", chainMin, behaviorMin)
	fmt.Println("[初始化] ✅ 报告引擎已就绪 (日报/周报/月报)")

	return e
}

// wireInbound 将引擎注入 InboundProxy
func (e *EngineSet) wireInbound(ip *InboundProxy, cfg *Config, engine *RuleEngine, tenantMgr *TenantManager) {
	ip.realtime = e.Realtime
	ip.slog = GetAppLogger()
	ip.traceCorrelator = e.TraceCorrelator
	ip.sessionCorrelator = e.SessionCorr
	ip.sessionDetector = e.SessionDetector
	ip.llmDetector = e.LLMDetector
	ip.detectCache = e.DetectCache
	ip.alertNotifier = e.AlertNotifier
	ip.adaptiveEngine = e.AdaptiveEngine
	ip.singularityEngine = e.SingularityEngine
	ip.honeypotDeep = e.HoneypotDeep
	ip.semanticDetector = e.SemanticDetect
	ip.pathPolicyEngine = e.PathPolicyEngine
	ip.planCompiler = e.PlanCompiler
	ip.capabilityEngine = e.CapabilityEngine
	ip.deviationDetector = e.DeviationDetector
	ip.ifcEngine = e.IFCEngine
	ip.taintTracker = e.TaintTracker
	ip.SetTenantManager(tenantMgr)
	ip.SetAPIKeyManager(e.APIKeyMgr)

	// 人工确认引擎（v37.0）
	if cfg.HumanConfirm.Enabled {
		cs := NewConfirmStore()
		cs.proxy = ip
		ip.confirmStore = cs
	}

	// 执行信封 + 事件总线
	if e.EnvelopeMgr != nil {
		ip.envelopeMgr = e.EnvelopeMgr
	}
	if e.EventBus != nil {
		ip.eventBus = e.EventBus
	}

	// 检测 Pipeline
	additionalStages := map[string]DetectStage{}
	if e.SessionDetector != nil {
		additionalStages["session"] = NewSessionStage(e.SessionDetector)
	}
	if e.LLMDetector != nil {
		additionalStages["llm"] = NewLLMStage(e.LLMDetector)
	}
	if len(cfg.DetectPipeline) > 0 {
		ip.pipeline = BuildPipelineFromConfig(cfg.DetectPipeline, engine, additionalStages)
		fmt.Printf("[初始化] ✅ 检测链: %v\n", cfg.DetectPipeline)
	} else {
		ip.pipeline = BuildDefaultPipeline(engine)
		fmt.Printf("[初始化] ✅ 检测链: [keyword, regex, pii] (默认)\n")
	}
	// 语义检测追加到 Pipeline 末尾
	if e.SemanticDetect != nil && ip.pipeline != nil {
		ip.pipeline.stages = append(ip.pipeline.stages, NewSemanticStage(e.SemanticDetect))
	}
}

// wireOutbound 将引擎注入 OutboundProxy
func (e *EngineSet) wireOutbound(op *OutboundProxy, routes *RouteTable) {
	op.realtime = e.Realtime
	op.traceCorrelator = e.TraceCorrelator
	op.routes = routes
	op.alertNotifier = e.AlertNotifier
	op.taintTracker = e.TaintTracker
	op.reversalEngine = e.ReversalEngine
	if e.EnvelopeMgr != nil {
		op.envelopeMgr = e.EnvelopeMgr
	}
	if e.EventBus != nil {
		op.eventBus = e.EventBus
	}
	fmt.Println("[初始化] ✅ Trace 关联缓存 (入站↔出站 trace_id 自动关联, 5min 窗口)")
}

// wireMgmtAPI 将引擎注入 ManagementAPI
func (e *EngineSet) wireMgmtAPI(m *ManagementAPI, tenantMgr *TenantManager, authMgr *AuthManager, llmAuditor *LLMAuditor, llmRuleEngine *LLMRuleEngine, llmProxy *LLMProxy) {
	m.tenantMgr = tenantMgr
	m.authManager = authMgr
	m.sessionDetector = e.SessionDetector
	m.llmDetector = e.LLMDetector
	m.detectCache = e.DetectCache
	m.llmAuditor = llmAuditor
	m.llmRuleEngine = llmRuleEngine
	m.llmProxy = llmProxy
	m.userProfileEng = e.UserProfileEng
	m.healthScoreEng = e.HealthScoreEng
	m.owaspMatrixEng = e.OwaspMatrixEng
	m.strictMode = e.StrictMode
	m.notificationEng = e.NotificationEng
	m.anomalyDetector = e.AnomalyDetect
	m.promptTracker = e.PromptTracker
	m.redTeamEngine = e.RedTeamEngine
	m.leaderboardEng = e.LeaderboardEng
	m.reportEngine = e.ReportEngine
	m.sessionReplayEng = e.SessionReplayEng
	m.honeypotEngine = e.HoneypotEngine
	m.traceCorrelator = e.TraceCorrelator
	m.abTestEngine = e.ABTestEngine
	m.behaviorProfileEng = e.BehaviorProfile
	m.upstreamProfileEng = e.UpstreamProfileEng
	m.attackChainEng = e.AttackChainEng
	m.layoutStore = e.LayoutStore
	m.sessionCorrelator = e.SessionCorr
	m.honeypotDeep = e.HoneypotDeep
	m.pathPolicyEngine = e.PathPolicyEngine
	m.k8sDiscovery = e.K8sDiscovery
	m.semanticDetector = e.SemanticDetect
	m.evolutionEngine = e.EvolutionEngine
	m.suggestionQueue = e.SuggestionQueue
	m.adaptiveEngine = e.AdaptiveEngine
	m.singularityEngine = e.SingularityEngine
	m.cfVerifier = e.CFVerifier
	m.adaptiveStrategy = e.AdaptiveStrategy
	m.planCompiler = e.PlanCompiler
	m.capabilityEngine = e.CapabilityEngine
	m.deviationDetector = e.DeviationDetector
	m.ifcEngine = e.IFCEngine
	m.ifcQuarantine = e.IFCQuarantine
	m.toolPolicy = e.ToolPolicy
	m.taintTracker = e.TaintTracker
	m.reversalEngine = e.ReversalEngine
	m.llmCache = e.LLMCache
	m.apiGateway = e.APIGateway
	m.apiKeyMgr = e.APIKeyMgr
	m.autoReviewMgr = e.AutoReviewMgr
	if e.EnvelopeMgr != nil {
		m.envelopeMgr = e.EnvelopeMgr
	}
	if e.EventBus != nil {
		m.eventBus = e.EventBus
	}

	// 需要 mgmtAPI 引用的组件
	e.CanaryRotator = NewCanaryRotator(m)
	m.canaryRotator = e.CanaryRotator
	fmt.Println("[初始化] ✅ 金丝雀轮换已启用")

	e.ReportScheduler = NewReportScheduler(m)
	m.reportScheduler = e.ReportScheduler
	fmt.Println("[初始化] ✅ 报告定时器已启用")
}

// wireLLMProxy 将引擎注入 LLMProxy（仅当 LLMProxy 启用时调用）
func (e *EngineSet) wireLLMProxy(lp *LLMProxy) {
	if lp == nil {
		return
	}
	if e.EnvelopeMgr != nil {
		lp.envelopeMgr = e.EnvelopeMgr
	}
	if e.EventBus != nil {
		lp.eventBus = e.EventBus
	}
	lp.singularityEngine = e.SingularityEngine
	lp.SetTenantManager(nil) // already set via initAllEngines tenantMgr injection
	lp.SetAPIKeyManager(e.APIKeyMgr)
}

// stopAll 优雅停止所有引擎
func (e *EngineSet) stopAll() {
	if e.AutoReviewMgr != nil {
		e.AutoReviewMgr.Stop()
	}
	if e.TaintTracker != nil {
		e.TaintTracker.Stop()
	}
	if e.PlanCompiler != nil {
		e.PlanCompiler.Stop()
	}
	if e.EvolutionEngine != nil {
		e.EvolutionEngine.StopAutoEvolution()
	}
	if e.EventBus != nil {
		e.EventBus.Stop()
	}
	if e.BgScheduler != nil {
		e.BgScheduler.Stop()
	}
}
