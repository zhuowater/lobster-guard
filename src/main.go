// main.go — 入口 main()、banner、CLI 参数解析、启动流程
// lobster-guard v4.0 代码拆分
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	AppName    = "lobster-guard"
	AppVersion = "29.0"
)

var startTime = time.Now()

func printBanner() {
	banner := `
  _         _         _                                         _
 | |   ___ | |__  ___| |_ ___ _ __       __ _ _   _  __ _ _ __| |
 | |  / _ \| '_ \/ __| __/ _ \ '__|____ / _' | | | |/ _' | '__| |
 | |_| (_) | |_) \__ \ ||  __/ | |_____| (_| | |_| | (_| | |  | |_
 |___|\___/|_.__/|___/\__\___|_|        \__, |\__,_|\__,_|_|  |___|
                                         |___/
        龙虾卫士 - AI Agent 安全网关 v%s
        入站检测 | 出站拦截 | 多Bot亲和路由 | 多通道支持 | 桥接模式 | 请求限流 | 规则热更新 | 规则引擎增强 | 用户信息自动获取
`
	fmt.Printf(banner, AppVersion)
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "配置文件路径")
	genRulesFile := flag.String("gen-rules", "", "生成默认入站规则文件到指定路径")
	restorePath := flag.String("restore", "", "从备份文件恢复数据库后启动")
	checkConfig := flag.Bool("check-config", false, "验证配置文件并退出")
	dumpRoutes := flag.Bool("dump-routes", false, "打印当前路由表并退出")
	dumpRules := flag.Bool("dump-rules", false, "打印当前入站+出站规则并退出")
	showVersion := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()

	// -version: 打印版本号并退出
	if *showVersion {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		return
	}

	// -gen-rules: 导出默认规则文件后退出
	if *genRulesFile != "" {
		rules := getDefaultInboundRules()
		rulesFile := InboundRulesFileConfig{Rules: rules}
		data, err := yaml.Marshal(&rulesFile)
		if err != nil { log.Fatalf("序列化规则失败: %v", err) }
		header := "# lobster-guard 默认入站规则文件\n# 由 lobster-guard -gen-rules 自动生成\n# 可自定义修改后通过 inbound_rules_file 配置项加载\n\n"
		if err := os.WriteFile(*genRulesFile, []byte(header+string(data)), 0644); err != nil {
			log.Fatalf("写入规则文件失败: %v", err)
		}
		fmt.Printf("✅ 默认入站规则已导出到: %s (%d 条规则, %d 个 pattern)\n",
			*genRulesFile, len(rules), countPatterns(rules))
		return
	}

	printBanner()

	cfg, err := loadConfig(*cfgPath)
	if err != nil { log.Fatalf("加载配置失败: %v", err) }

	// v4.0 配置验证
	if errs := validateConfig(cfg); len(errs) > 0 {
		for _, e := range errs { log.Printf("[配置错误] ❌ %s", e) }
		log.Fatalf("配置验证失败，共 %d 个错误", len(errs))
	}

	// -check-config: 验证配置文件并退出
	if *checkConfig {
		fmt.Println("✅ 配置文件验证通过")
		fmt.Printf("  通道: %s\n", func() string { if cfg.Channel == "" { return "lanxin" }; return cfg.Channel }())
		fmt.Printf("  模式: %s\n", func() string { if cfg.Mode == "" { return "webhook" }; return cfg.Mode }())
		fmt.Printf("  入站监听: %s\n", cfg.InboundListen)
		fmt.Printf("  出站监听: %s\n", cfg.OutboundListen)
		fmt.Printf("  管理监听: %s\n", cfg.ManagementListen)
		fmt.Printf("  数据库: %s\n", cfg.DBPath)
		fmt.Printf("  上游数: %d\n", len(cfg.StaticUpstreams))
		fmt.Printf("  日志格式: %s\n", func() string { if cfg.LogFormat == "" { return "text" }; return cfg.LogFormat }())
		return
	}

	// -dump-rules: 打印规则并退出
	if *dumpRules {
		rules, source, err := resolveInboundRules(cfg)
		if err != nil { log.Fatalf("加载入站规则失败: %v", err) }
		if rules == nil {
			rules = getDefaultInboundRules()
			source = "default"
		}
		fmt.Printf("=== 入站规则 (来源: %s, %d 条) ===\n", source, len(rules))
		for _, r := range rules {
			typeStr := r.Type
			if typeStr == "" { typeStr = "keyword" }
			fmt.Printf("  [%s] %s (%s) patterns=%d priority=%d group=%q\n", r.Action, r.Name, typeStr, len(r.Patterns), r.Priority, r.Group)
		}
		fmt.Printf("\n=== 出站规则 (%d 条) ===\n", len(cfg.OutboundRules))
		for _, r := range cfg.OutboundRules {
			pCount := 0
			if r.Pattern != "" { pCount = 1 }
			if len(r.Patterns) > 0 { pCount = len(r.Patterns) }
			fmt.Printf("  [%s] %s patterns=%d priority=%d\n", r.Action, r.Name, pCount, r.Priority)
		}
		return
	}

	// -dump-routes: 需要数据库，打印路由表并退出
	if *dumpRoutes {
		db, err := initDB(cfg.DBPath)
		if err != nil { log.Fatalf("初始化数据库失败: %v", err) }
		defer db.Close()
		routes := NewRouteTable(db, cfg.RoutePersist)
		entries := routes.ListRoutes()
		fmt.Printf("=== 路由表 (%d 条) ===\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  sender=%s app=%s -> upstream=%s (dept=%s name=%s)\n",
				e.SenderID, e.AppID, e.UpstreamID, e.Department, e.DisplayName)
		}
		return
	}

	// v5.0: 初始化结构化日志
	InitAppLogger(cfg.LogFormat)
	slog := GetAppLogger()

	// v4.2: 从备份恢复
	if *restorePath != "" {
		log.Printf("[恢复] 从备份文件恢复: %s -> %s", *restorePath, cfg.DBPath)
		if err := RestoreFromBackup(*restorePath, cfg.DBPath); err != nil {
			log.Fatalf("恢复备份失败: %v", err)
		}
		log.Printf("[恢复] ✅ 数据库已从备份恢复")
	}

	channelName := cfg.Channel
	if channelName == "" { channelName = "lanxin" }
	modeName := cfg.Mode
	if modeName == "" { modeName = "webhook" }

	// 初始化通道插件
	var channel ChannelPlugin
	switch cfg.Channel {
	case "feishu":
		channel = NewFeishuPlugin(cfg.FeishuEncryptKey, cfg.FeishuVerificationToken)
	case "dingtalk":
		channel = NewDingtalkPlugin(cfg.DingtalkToken, cfg.DingtalkAesKey, cfg.DingtalkCorpId)
	case "wecom":
		channel = NewWecomPlugin(cfg.WecomToken, cfg.WecomEncodingAesKey, cfg.WecomCorpId)
	case "generic":
		channel = NewGenericPlugin(cfg.GenericSenderHeader, cfg.GenericTextField)
	default:
		crypto, err := NewLanxinCrypto(cfg.CallbackKey, cfg.CallbackSignToken)
		if err != nil { log.Fatalf("初始化蓝信加解密失败: %v", err) }
		channel = NewLanxinPlugin(crypto)
	}
	fmt.Printf("[初始化] ✅ 通道插件: %s (%s 模式)\n", channelName, modeName)

	// 初始化入站规则引擎
	var engine *RuleEngine
	inboundRules, inboundSource, err := resolveInboundRules(cfg)
	if err != nil { log.Fatalf("加载入站规则失败: %v", err) }

	// v30.0: 行业模板不再通过 config.yaml rule_templates 加载
	// 改为 DB 全局开关机制，通过 Dashboard 启用/禁用
	if inboundRules != nil {
		engine = NewRuleEngineWithPII(inboundRules, inboundSource, cfg.OutboundPIIPatterns, cfg.RuleBindings)
	} else {
		engine = NewRuleEngineWithPII(getDefaultInboundRules(), "default", cfg.OutboundPIIPatterns, cfg.RuleBindings)
	}
	printInboundRuleSummary(engine)

	// 初始化出站规则引擎
	outboundEngine := NewOutboundRuleEngine(cfg.OutboundRules)
	printOutboundRuleSummary(cfg.OutboundRules)

	// PII 模式摘要
	printPIISummary(engine)

	// 初始化数据库
	db, err := initDB(cfg.DBPath)
	if err != nil { log.Fatalf("初始化数据库失败: %v", err) }
	defer db.Close()

	// v4.2: 创建 Store 抽象层
	store := NewSQLiteStore(db, cfg.DBPath)

	// v14.0: 初始化租户管理器
	tenantMgr := NewTenantManager(db)

	// v27.2: 入站规则持久化（重启后从 DB 恢复租户入站规则）
	engine.SetTenantDB(db)
	// v28.0: 入站规则模板 DB 初始化
	engine.SetInboundTemplateDB(db)
	// v31.0: 初始化统一行业模板系统（优先于旧模板）
	initIndustryTemplateSystem(db)
	// v30.0/v31.0: 初始化全局启用模板规则
	engine.InitGlobalTemplateAC()
	outboundEngine.InitGlobalTemplateRules(db)
	// (v31.0 auto-review init moved below pool creation)

	// v14.1: 初始化认证管理器
	authMgr := NewAuthManager(db, &cfg.Auth)
	if cfg.Auth.Enabled {
		fmt.Println("[初始化] ✅ 登录认证: 已启用")
	} else {
		fmt.Println("[初始化] ⚠️ 登录认证: 未启用（使用 Bearer token 模式）")
	}

	logger, err := NewAuditLogger(db)
	if err != nil { log.Fatalf("初始化审计日志失败: %v", err) }
	defer logger.Close()
	logger.SetTenantManager(tenantMgr) // v14.0: 审计日志自动解析租户

	// v9.0: LLM 代理（可选）
	var llmAuditor *LLMAuditor
	var llmProxy *LLMProxy
	var llmRuleEngine *LLMRuleEngine
	if cfg.LLMProxy.Enabled {
		// v10.0: 初始化 LLM 规则引擎（v18: 合并用户配置 + 默认规则）
		llmRules := mergeLLMRuleDefaults(cfg.LLMProxy.Rules)
		llmRuleEngine = NewLLMRuleEngine(llmRules)
		llmRuleEngine.SetDB(logger.DB()) // Issue #7 fix: 命中计数持久化
		llmRuleEngine.SetTenantDB(logger.DB())   // v28.0: LLM 规则租户绑定持久化
		llmRuleEngine.SetTemplateDB(logger.DB())  // v28.0: LLM 规则模板持久化
		initIndustryTemplateSystem(logger.DB())
		llmRuleEngine.InitGlobalLLMTemplateRules() // v30.0/v31.0: 初始化全局启用的 LLM 行业模板规则
		log.Printf("[初始化] ✅ LLM 规则引擎: %d 条规则 (用户%d+默认补充)", len(llmRules), len(cfg.LLMProxy.Rules))

		llmAuditor = NewLLMAuditor(logger.DB(), cfg.LLMProxy.AuditConfig, &cfg.LLMProxy)
		llmProxy = NewLLMProxy(cfg.LLMProxy, llmAuditor, llmRuleEngine)
		llmProxy.mainCfg = cfg
		go func() {
			if err := llmProxy.Start(); err != nil {
				log.Printf("[LLM代理] 启动失败: %v", err)
			}
		}()
		log.Printf("[初始化] ✅ LLM 代理已启动: %s (%d 个 target)", cfg.LLMProxy.Listen, len(cfg.LLMProxy.Targets))
	} else {
		fmt.Println("[初始化] ⚠️ LLM 代理: 未启用")
	}

	// v4.2: 创建关闭管理器
	shutdownMgr := NewShutdownManager(cfg)
	shutdownMgr.SetLogger(logger)
	shutdownMgr.SetStore(store)

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, cfg.RoutePersist)

	// v31.0: AC 智能分级（自动模式）
	autoReviewMgr := NewAutoReviewManager(cfg.AutoReview, pool)
	// 连接 LLM Proxy targets（用于 LLM 复核调用，而不是 UpstreamPool）
	if cfg.LLMProxy.Enabled && len(cfg.LLMProxy.Targets) > 0 {
		autoReviewMgr.llmTargets = cfg.LLMProxy.Targets
	}
	engine.autoReviewMgr = autoReviewMgr
	if cfg.AutoReview.Enabled {
		autoReviewMgr.Start()
		fmt.Printf("[初始化] ✅ AC 智能分级已启用 (窗口=%ds, 阈值=%d, TTL=%ds)\n",
			cfg.AutoReview.WindowSeconds, cfg.AutoReview.SpikeThreshold, cfg.AutoReview.AutoReviewTTL)
	} else {
		fmt.Println("[初始化] ⚠️ AC 智能分级: 未启用 (auto_review.enabled=false)")
	}
	// D-006: 从路由表聚合恢复 user_count（比逐个 CountByUpstream 更精确）
	pool.RestoreUserCounts(db)

	var metrics *MetricsCollector
	if cfg.IsMetricsEnabled() { metrics = NewMetricsCollector() }

	ruleHits := NewRuleHitStats()

	var userCache *UserInfoCache
	var policyEng *RoutePolicyEngine
	provider := createUserInfoProvider(cfg)
	if provider != nil { userCache = NewUserInfoCache(db, provider, 24*time.Hour) }
	if len(cfg.RoutePolicies) > 0 { policyEng = NewRoutePolicyEngine(cfg.RoutePolicies) }

	// 限流
	printRateLimitSummary(cfg)

	// 上游
	upTotal, _ := pool.Count()
	upIDs := make([]string, 0)
	for _, u := range pool.ListUpstreams() { upIDs = append(upIDs, u.ID) }
	fmt.Printf("[初始化] ✅ 上游: %d 个静态 (%s)\n", upTotal, strings.Join(upIDs, ", "))

	// 审计
	retentionDays := cfg.AuditRetentionDays
	if retentionDays <= 0 { retentionDays = 30 }
	alertDesc := "未配置"
	if cfg.AlertWebhook != "" { alertDesc = cfg.AlertWebhook }
	fmt.Printf("[初始化] ✅ 审计: 保留 %d 天, 告警 webhook: %s\n", retentionDays, alertDesc)

	// Metrics
	if cfg.IsMetricsEnabled() {
		fmt.Printf("[初始化] ✅ Prometheus: %s/metrics\n", cfg.ManagementListen)
	} else {
		fmt.Println("[初始化] ⚠️ Prometheus: 未启用")
	}

	// Bridge
	if cfg.Mode == "bridge" {
		fmt.Printf("[初始化] ✅ Bridge Mode: %s 长连接\n", channelName)
	} else {
		fmt.Println("[初始化] ⚠️ Bridge Mode: 未启用")
	}

	// v5.0: 实时监控指标
	realtime := NewRealtimeMetrics()

	// v5.1: 会话检测器
	var sessionDetector *SessionDetector
	if cfg.SessionDetectEnabled {
		sessionDetector = NewSessionDetector(SessionDetectorConfig{
			Enabled:       true,
			RiskThreshold: func() float64 { if cfg.SessionRiskThreshold > 0 { return cfg.SessionRiskThreshold }; return 10 }(),
			Window:        func() int { if cfg.SessionWindow > 0 { return cfg.SessionWindow }; return 20 }(),
			DecayRate:     func() float64 { if cfg.SessionDecayRate > 0 { return cfg.SessionDecayRate }; return 1 }(),
		})
		fmt.Printf("[初始化] ✅ 会话检测: threshold=%.0f, window=%d, decay_rate=%.1f/h\n",
			sessionDetector.cfg.RiskThreshold, sessionDetector.cfg.Window, sessionDetector.cfg.DecayRate)
	}

	// v5.1: LLM 检测器
	var llmDetector *LLMDetector
	if cfg.LLMDetectEnabled {
		llmDetector = NewLLMDetector(LLMDetectorConfig{
			Enabled:  true,
			Endpoint: cfg.LLMDetectEndpoint,
			APIKey:   cfg.LLMDetectAPIKey,
			Model:    cfg.LLMDetectModel,
			Timeout:  cfg.LLMDetectTimeout,
			Mode:     cfg.LLMDetectMode,
			Prompt:   cfg.LLMDetectPrompt,
		})
		fmt.Printf("[初始化] ✅ LLM 检测: endpoint=%s, model=%s, mode=%s\n",
			cfg.LLMDetectEndpoint, llmDetector.cfg.Model, llmDetector.cfg.Mode)
	}

	// v5.1: 检测缓存
	var detectCache *DetectCache
	cacheTTL := cfg.DetectCacheTTL
	if cacheTTL <= 0 { cacheTTL = 300 }
	cacheSize := cfg.DetectCacheSize
	if cacheSize <= 0 { cacheSize = 1000 }
	detectCache = NewDetectCache(cacheSize, time.Duration(cacheTTL)*time.Second)
	fmt.Printf("[初始化] ✅ 检测缓存: size=%d, ttl=%ds\n", cacheSize, cacheTTL)

	// 创建代理
	// v15.0: 蜜罐引擎（提前初始化，供入站/出站代理使用）
	honeypotEngine := NewHoneypotEngine(logger.DB())
	fmt.Println("[初始化] ✅ 蜜罐引擎已就绪 (Agent 蜜罐: 假数据+水印追踪+引爆检测)")

	// v18: Trace 关联缓存 — 入站↔出站 trace_id 自动关联
	traceCorrelator := NewTraceCorrelator(10000)
	// v17.3: IM↔LLM 会话关联
	sessionIdleMs := int64(60 * 60 * 1000) // 默认 1 小时
	if cfg.SessionIdleTimeoutMin > 0 {
		sessionIdleMs = int64(cfg.SessionIdleTimeoutMin) * 60 * 1000
	}
	sessionFPMs := int64(5 * 60 * 1000) // 默认 5 分钟
	if cfg.SessionFPWindowSec > 0 {
		sessionFPMs = int64(cfg.SessionFPWindowSec) * 1000
	}
	sessionCorrelator := NewSessionCorrelator(50000, sessionIdleMs)
	sessionCorrelator.fpWindowMs = sessionFPMs
	if llmProxy != nil {
		llmProxy.sessionCorrelator = sessionCorrelator
	}

	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, metrics, ruleHits, userCache, policyEng, honeypotEngine)
	inbound.realtime = realtime
	inbound.slog = slog
	inbound.traceCorrelator = traceCorrelator
	inbound.sessionCorrelator = sessionCorrelator
	// v5.1: 注入智能检测组件
	inbound.sessionDetector = sessionDetector
	inbound.llmDetector = llmDetector
	inbound.detectCache = detectCache
	// v5.1: 构建检测 Pipeline
	{
		additionalStages := map[string]DetectStage{}
		if sessionDetector != nil {
			additionalStages["session"] = NewSessionStage(sessionDetector)
		}
		if llmDetector != nil {
			additionalStages["llm"] = NewLLMStage(llmDetector)
		}
		if len(cfg.DetectPipeline) > 0 {
			inbound.pipeline = BuildPipelineFromConfig(cfg.DetectPipeline, engine, additionalStages)
			fmt.Printf("[初始化] ✅ 检测链: %v\n", cfg.DetectPipeline)
		} else {
			inbound.pipeline = BuildDefaultPipeline(engine)
			fmt.Printf("[初始化] ✅ 检测链: [keyword, regex, pii] (默认)\n")
		}
	}
	outbound, err := NewOutboundProxy(cfg, channel, engine, outboundEngine, logger, metrics, ruleHits, honeypotEngine)
	if err != nil { log.Fatalf("初始化出站代理失败: %v", err) }
	outbound.realtime = realtime
	outbound.traceCorrelator = traceCorrelator
	outbound.routes = routes // D-005: 出站代理路由感知
	fmt.Println("[初始化] ✅ Trace 关联缓存 (入站↔出站 trace_id 自动关联, 5min 窗口)")

	// v4.1 WebSocket 代理管理器
	wsProxy := NewWSProxyManager(cfg, engine, outboundEngine, logger, metrics, pool, routes, ruleHits)
	inbound.wsProxy = wsProxy
	wsMode := cfg.WSMode
	if wsMode == "" { wsMode = "inspect" }
	wsMaxConn := cfg.WSMaxConnections
	if wsMaxConn <= 0 { wsMaxConn = 100 }
	fmt.Printf("[初始化] ✅ WebSocket 代理: mode=%s, max_connections=%d, idle_timeout=%ds, max_duration=%ds\n",
		wsMode, wsMaxConn, func() int { if cfg.WSIdleTimeout <= 0 { return 300 }; return cfg.WSIdleTimeout }(),
		func() int { if cfg.WSMaxDuration <= 0 { return 3600 }; return cfg.WSMaxDuration }())

	var alertNotifier *AlertNotifier
	if cfg.AlertWebhook != "" {
		alertNotifier = NewAlertNotifier(cfg.AlertWebhook, cfg.AlertFormat, cfg.AlertMinInterval, metrics)
		inbound.alertNotifier = alertNotifier
		outbound.alertNotifier = alertNotifier
	}

	mgmtAPI := NewManagementAPI(cfg, *cfgPath, pool, routes, logger, engine, outboundEngine, inbound, channel, metrics, ruleHits, userCache, policyEng, alertNotifier, wsProxy, store, shutdownMgr, realtime)
	mgmtAPI.tenantMgr = tenantMgr // v14.0
	mgmtAPI.authManager = authMgr // v14.1
	// v5.1: 注入智能检测组件
	mgmtAPI.sessionDetector = sessionDetector
	mgmtAPI.llmDetector = llmDetector
	mgmtAPI.detectCache = detectCache
	mgmtAPI.llmAuditor = llmAuditor // v9.0
	mgmtAPI.llmRuleEngine = llmRuleEngine // v10.0
	mgmtAPI.llmProxy = llmProxy // v10.1
	mgmtAPI.userProfileEng = NewUserProfileEngine(logger.DB()) // v11.0
	// v11.1: 驾驶舱模式
	mgmtAPI.healthScoreEng = NewHealthScoreEngine(logger.DB())
	mgmtAPI.owaspMatrixEng = NewOWASPMatrixEngine(logger.DB(), llmRuleEngine)
	mgmtAPI.strictMode = NewStrictModeManager(engine, llmRuleEngine)
	mgmtAPI.notificationEng = NewNotificationEngine(logger.DB())
	// v11.2: 异常基线检测器
	anomalyDetector := NewAnomalyDetector(logger.DB())
	mgmtAPI.anomalyDetector = anomalyDetector
	anomalyDetector.StartBackground()
	fmt.Println("[初始化] ✅ 异常基线检测器已启动 (6 个指标, 7 天窗口, >2σ 告警)")

	// v13.0: 会话回放引擎
	sessionReplayEng := NewSessionReplayEngine(logger.DB())
	fmt.Println("[初始化] ✅ 会话回放引擎已就绪 (trace_id 串联 IM+LLM+Tools)")

	// v13.1: Prompt 版本追踪器
	promptTracker := NewPromptTracker(logger.DB())
	if llmAuditor != nil {
		llmAuditor.promptTracker = promptTracker
	}
	mgmtAPI.promptTracker = promptTracker
	fmt.Println("[初始化] ✅ Prompt 版本追踪器已就绪 (自动检测 System Prompt 变化)")

	// v14.2: Red Team Autopilot
	redTeamEngine := NewRedTeamEngine(logger.DB(), engine)
	redTeamEngine.outboundEngine = outboundEngine   // v17.3: 出站规则红队
	redTeamEngine.llmRuleEngine = llmRuleEngine      // v17.3: LLM 规则红队
	redTeamEngine.honeypotEngine = honeypotEngine     // v17.3: 蜜罐红队
	mgmtAPI.redTeamEngine = redTeamEngine
	fmt.Println("[初始化] ✅ Red Team Autopilot 引擎已就绪 (35 攻击向量, OWASP LLM Top10)")

	// v14.3: 安全排行榜 + SLA 基线
	leaderboardEng := NewLeaderboardEngine(logger.DB(), tenantMgr, mgmtAPI.healthScoreEng)
	mgmtAPI.leaderboardEng = leaderboardEng
	fmt.Println("[初始化] ✅ 安全排行榜 + SLA 基线引擎已就绪 (排行榜/热力图/SLA 达标)")

	// v12.0: 报告引擎
	reportEngine := NewReportEngine(logger.DB(), "/var/lib/lobster-guard/reports/")
	reportEngine.SetEngines(mgmtAPI.healthScoreEng, mgmtAPI.owaspMatrixEng, llmAuditor, mgmtAPI.userProfileEng, anomalyDetector, logger)
	mgmtAPI.reportEngine = reportEngine
	mgmtAPI.sessionReplayEng = sessionReplayEng
	mgmtAPI.honeypotEngine = honeypotEngine // v15.0
	mgmtAPI.traceCorrelator = traceCorrelator // v18

	// v15.1: A/B 测试引擎
	abTestEngine := NewABTestEngine(logger.DB())
	mgmtAPI.abTestEngine = abTestEngine
	fmt.Println("[初始化] ✅ A/B 测试引擎已就绪 (Prompt A/B 测试 + 流量分配 + 统计显著性)")

	// v16.0: Agent 行为画像引擎
	behaviorProfileEng := NewBehaviorProfileEngine(logger.DB())
	mgmtAPI.behaviorProfileEng = behaviorProfileEng
	fmt.Println("[初始化] ✅ Agent 行为画像引擎已就绪 (语义行为模式 + 突变检测 + 风险评估)")

	// v16.1: 攻击链引擎
	attackChainEng := NewAttackChainEngine(logger.DB())
	mgmtAPI.attackChainEng = attackChainEng
	fmt.Println("[初始化] ✅ 攻击链引擎已就绪 (跨 Agent 关联分析 + 模式匹配 + 风险评分)")

	// v17.1: 布局存储
	mgmtAPI.layoutStore = NewLayoutStore(logger.DB())
	mgmtAPI.sessionCorrelator = sessionCorrelator // v17.3
	fmt.Println("[初始化] ✅ 布局引擎已就绪 (面板拖拽 + 折叠 + 预设模板)")

	// v19.0: 对抗性自进化引擎（声明，在事件总线初始化后创建）
	var evolutionEngine *EvolutionEngine

	// v18.1: 事件总线
	var eventBus *EventBus
	if cfg.EventBus.Enabled {
		eventBus = NewEventBus(logger.DB(), cfg)
		fmt.Println("[初始化] ✅ 事件总线已启用")
	} else {
		fmt.Println("[初始化] ⚠️ 事件总线: 未启用")
	}

	// v18.0: 执行信封 — 密码学审计链
	var envelopeMgr *EnvelopeManager
	if cfg.EnvelopeEnabled && cfg.EnvelopeSecretKey != "" {
		batchSize := cfg.EnvelopeBatchSize
		if batchSize <= 0 {
			batchSize = 64
		}
		envelopeMgr = NewEnvelopeManagerWithBatchSize(logger.DB(), cfg.EnvelopeSecretKey, batchSize)
		envelopeMgr.startAutoFlush()
		fmt.Println("[初始化] ✅ 执行信封已启用（密码学审计链）")
	} else {
		fmt.Println("[初始化] ⚠️ 执行信封: 未启用")
	}
	// v18.0: 注入执行信封到各数据通道
	if envelopeMgr != nil {
		inbound.envelopeMgr = envelopeMgr
		outbound.envelopeMgr = envelopeMgr
		if llmProxy != nil {
			llmProxy.envelopeMgr = envelopeMgr
		}
		mgmtAPI.envelopeMgr = envelopeMgr
	}

	// v18.1: 注入事件总线到各组件
	if eventBus != nil {
		inbound.eventBus = eventBus
		outbound.eventBus = eventBus
		if llmProxy != nil {
			llmProxy.eventBus = eventBus
		}
		mgmtAPI.eventBus = eventBus
		// v31.0: 提示语追踪→告警
		promptTracker.eventBus = eventBus
		promptTracker.driftThreshold = 0.5 // 默认漂移阈值
		// 注入严格模式回调
		if mgmtAPI.strictMode != nil {
			eventBus.strictModeFunc = func(enable bool) error {
				mgmtAPI.strictMode.SetEnabled(enable)
				return nil
			}
		}
	}

	// v19.0: 对抗性自进化
	if cfg.EvolutionEnabled {
		evolutionEngine = NewEvolutionEngine(logger.DB(), redTeamEngine, engine, outboundEngine, llmRuleEngine, eventBus)
		intervalMin := cfg.EvolutionIntervalMin
		if intervalMin <= 0 {
			intervalMin = 360
		}
		evolutionEngine.StartAutoEvolution(intervalMin)
		fmt.Printf("[初始化] ✅ 对抗性自进化已启用 (每 %d 分钟)\n", intervalMin)
	} else {
		fmt.Println("[初始化] ⚠️ 对抗性自进化: 未启用")
	}
	mgmtAPI.evolutionEngine = evolutionEngine

	// v18.3: 自适应决策引擎
	var adaptiveEngine *AdaptiveDecisionEngine
	if cfg.AdaptiveDecision.Enabled {
		adaptiveEngine = NewAdaptiveDecisionEngine(logger.DB(), envelopeMgr, cfg.AdaptiveDecision)
		fmt.Println("[初始化] ✅ 自适应决策引擎已启用（贝叶斯误伤率优化）")
	} else {
		fmt.Println("[初始化] ⚠️ 自适应决策引擎: 未启用")
	}
	inbound.adaptiveEngine = adaptiveEngine
	mgmtAPI.adaptiveEngine = adaptiveEngine

	// v18.3: 奇点蜜罐引擎
	var singularityEngine *SingularityEngine
	if cfg.Singularity.Enabled {
		singularityEngine = NewSingularityEngine(logger.DB(), honeypotEngine, envelopeMgr, cfg.Singularity)
		fmt.Println("[初始化] ✅ 奇点蜜罐已启用")
	} else {
		fmt.Println("[初始化] ⚠️ 奇点蜜罐: 未启用")
	}
	mgmtAPI.singularityEngine = singularityEngine
	// v18.3: 注入奇点蜜罐到代理
	inbound.singularityEngine = singularityEngine
	if llmProxy != nil {
		llmProxy.singularityEngine = singularityEngine
	}

	// v19.2: 蜜罐深度交互引擎
	var honeypotDeep *HoneypotDeepEngine
	if cfg.HoneypotDeep.Enabled {
		honeypotDeep = NewHoneypotDeepEngine(logger.DB(), honeypotEngine, evolutionEngine, eventBus, cfg.HoneypotDeep)
		fmt.Println("[初始化] ✅ 蜜罐深度交互引擎已启用（忠诚度曲线 + 自进化回馈）")
	} else {
		fmt.Println("[初始化] ⚠️ 蜜罐深度交互引擎: 未启用")
	}

	// v19.1: 语义检测引擎
	var semanticDetector *SemanticDetector
	if cfg.SemanticDetector.Enabled {
		semanticDetector = NewSemanticDetector(logger.DB(), cfg.SemanticDetector)
		fmt.Printf("[初始化] ✅ 语义检测引擎已启用 (阈值=%.1f, 模式库=%d)\n", cfg.SemanticDetector.Threshold, len(semanticDetector.attackVectors))
	} else {
		fmt.Println("[初始化] ⚠️ 语义检测引擎: 未启用")
	}
	// v23.0: 路径级策略引擎
	pathPolicyEngine := NewPathPolicyEngine(logger.DB())
	pathPolicyEngine.SetUserProfileEngine(mgmtAPI.userProfileEng) // v23.1: 攻击者画像联动
	if cfg.PathPolicy.Enabled {
		fmt.Println("[初始化] ✅ 路径级策略引擎已启用 (路径追踪 + 序列/累计/降级规则 + 画像联动)")
	} else {
		fmt.Println("[初始化] ✅ 路径级策略引擎已就绪 (路径追踪 + 序列/累计/降级规则 + 画像联动)")
	}

	// v24.0: 反事实验证引擎
	cfConfig := defaultCFConfig
	if cfg.Counterfactual.Enabled {
		cfConfig = cfg.Counterfactual
	}
	cfVerifier := NewCounterfactualVerifier(logger.DB(), cfConfig, nil)
	cfVerifier.SetPathPolicy(pathPolicyEngine)
	mgmtAPI.cfVerifier = cfVerifier
	if llmProxy != nil {
		llmProxy.cfVerifier = cfVerifier
	}
	fmt.Println("[初始化] ✅ 反事实验证引擎已就绪 (AttriGuard 对照验证 + 归因分析)")

	// v24.2: 自适应验证策略引擎
	adaptiveStrategy := NewAdaptiveStrategy(logger.DB(), defaultAdaptiveConfig, pathPolicyEngine)
	cfVerifier.SetAdaptiveStrategy(adaptiveStrategy)
	mgmtAPI.adaptiveStrategy = adaptiveStrategy
	fmt.Println("[初始化] ✅ 自适应验证策略引擎已就绪 (成本控制 + 优先级调度 + 效果追踪)")

	// v25.0: 执行计划编译器
	planCfg := defaultPlanConfig
	if cfg.PlanCompiler.Enabled {
		planCfg = cfg.PlanCompiler
	}
	planCompiler := NewPlanCompiler(logger.DB(), planCfg)
	mgmtAPI.planCompiler = planCompiler
	if llmProxy != nil {
		llmProxy.planCompiler = planCompiler
	}
	fmt.Println("[初始化] ✅ 执行计划编译器已就绪 (CaMeL 网关级程序解释器, 20+ 内置模板)")

	// v25.1: Capability 权限系统
	capCfg := cfg.Capability
	if capCfg.DefaultPolicy == "" {
		capCfg.DefaultPolicy = "allow"
	}
	capEngine := NewCapabilityEngine(logger.DB(), capCfg)
	mgmtAPI.capabilityEngine = capEngine
	if llmProxy != nil {
		llmProxy.capabilityEngine = capEngine
	}
	fmt.Println("[初始化] ✅ Capability 权限系统已就绪")

	// v25.2: Plan 偏差检测器
	devCfg := cfg.Deviation
	if devCfg.MaxRepairs == 0 {
		devCfg.MaxRepairs = 5
	}
	devDetector := NewDeviationDetector(logger.DB(), devCfg, planCompiler, capEngine)
	mgmtAPI.deviationDetector = devDetector
	if llmProxy != nil {
		llmProxy.deviationDetector = devDetector
	}
	fmt.Println("[初始化] ✅ 偏差检测器已就绪")

	// v26.0: 信息流控制引擎
	ifcEngine := NewIFCEngine(logger.DB(), cfg.IFC)
	mgmtAPI.ifcEngine = ifcEngine
	if llmProxy != nil {
		llmProxy.ifcEngine = ifcEngine
	}
	fmt.Printf("[初始化] ✅ IFC 信息流控制已就绪 (来源规则=%d, 工具要求=%d, 隔离=%v, 隐藏=%v)\n",
		len(ifcEngine.ListSourceRules()), len(ifcEngine.ListToolRequirements()),
		cfg.IFC.QuarantineEnabled, cfg.IFC.HidingEnabled)

	// v26.1: 隔离LLM
	var ifcQuarantine *IFCQuarantine
	if cfg.IFC.QuarantineEnabled {
		ifcQuarantine = NewIFCQuarantine(ifcEngine, pool)
		if llmProxy != nil {
			llmProxy.ifcQuarantine = ifcQuarantine
		}
		mgmtAPI.ifcQuarantine = ifcQuarantine
		fmt.Printf("[初始化] ✅ IFC 隔离LLM已启用 (上游=%s)\n", cfg.IFC.QuarantineUpstream)
	} else {
		fmt.Printf("[初始化] ℹ️  IFC 隔离LLM未启用\n")
	}

	// v26.3 审计日志注入LLMProxy，治理引擎事件联动
	if llmProxy != nil {
		llmProxy.auditLogger = logger
	}

	// v20.0: 工具策略引擎
	var toolPolicyEngine *ToolPolicyEngine
	if cfg.ToolPolicy.Enabled {
		toolPolicyEngine = NewToolPolicyEngine(logger.DB(), cfg.ToolPolicy)
		fmt.Printf("[初始化] ✅ 工具策略引擎已启用 (规则=%d, 默认=%s)\n", len(toolPolicyEngine.ListRules()), cfg.ToolPolicy.DefaultAction)
	} else {
		fmt.Println("[初始化] ⚠️ 工具策略引擎: 未启用")
	}
	mgmtAPI.toolPolicy = toolPolicyEngine
	if llmProxy != nil {
		llmProxy.toolPolicy = toolPolicyEngine
		llmProxy.pathPolicyEngine = pathPolicyEngine // v23.0
	}

	// v20.1: 信息流污染追踪
	var taintTracker *TaintTracker
	if cfg.TaintTracker.Enabled {
		taintTracker = NewTaintTracker(db, cfg.TaintTracker)
		fmt.Printf("[初始化] ✅ 信息流污染追踪已启用 (action=%s, ttl=%d分钟, PII模式=%d)\n",
			taintTracker.config.Action, taintTracker.config.TTLMinutes, len(piiPatterns))
	} else {
		fmt.Println("[初始化] ⚠️ 信息流污染追踪: 未启用")
	}
	mgmtAPI.taintTracker = taintTracker
	// v31.0: 污点→IFC 统一
	if taintTracker != nil {
		taintTracker.SetIFCEngine(ifcEngine)
	}
	inbound.pathPolicyEngine = pathPolicyEngine // v23.0
	inbound.planCompiler = planCompiler          // v25.0
	inbound.capabilityEngine = capEngine         // v25.1
	inbound.deviationDetector = devDetector      // v25.2
	inbound.ifcEngine = ifcEngine                // v26.0
	// v27.0: 租户策略闭环 + API Key 身份管理
	apiKeyMgr := NewAPIKeyManager(db)
	inbound.SetTenantManager(tenantMgr)
	inbound.SetAPIKeyManager(apiKeyMgr)
	if llmProxy != nil {
		llmProxy.SetTenantManager(tenantMgr)
		llmProxy.SetAPIKeyManager(apiKeyMgr)
	}
	mgmtAPI.apiKeyMgr = apiKeyMgr
	mgmtAPI.autoReviewMgr = autoReviewMgr
	fmt.Printf("[初始化] ✅ 租户策略闭环 + API Key 管理器已就绪\n")
	inbound.taintTracker = taintTracker
	outbound.taintTracker = taintTracker
	if llmProxy != nil {
		llmProxy.taintTracker = taintTracker
	}

	// v20.2: 污染链逆转引擎
	var reversalEngine *TaintReversalEngine
	if cfg.TaintReversal.Enabled {
		reversalEngine = NewTaintReversalEngine(db, taintTracker, envelopeMgr, cfg.TaintReversal)
		fmt.Printf("[初始化] ✅ 污染链逆转已启用 (mode=%s, 模板=%d)\n",
			reversalEngine.GetConfig().Mode, len(reversalEngine.GetTemplates()))
	} else {
		fmt.Println("[初始化] ⚠️ 污染链逆转: 未启用")
	}
	mgmtAPI.reversalEngine = reversalEngine
	outbound.reversalEngine = reversalEngine
	if llmProxy != nil {
		llmProxy.reversalEngine = reversalEngine
	}

	// v20.3: LLM 响应缓存
	var llmCache *LLMCache
	if cfg.LLMCache.Enabled {
		llmCache = NewLLMCache(db, cfg.LLMCache)
		fmt.Printf("[初始化] ✅ LLM 响应缓存已启用 (max=%d, ttl=%d分钟, similarity=%.2f, 租户隔离=%v)\n",
			llmCache.config.MaxEntries, llmCache.config.TTLMinutes, llmCache.config.SimilarityMin, llmCache.config.TenantIsolation)
	} else {
		fmt.Println("[初始化] ⚠️ LLM 响应缓存: 未启用")
	}
	mgmtAPI.llmCache = llmCache
	if llmProxy != nil {
		llmProxy.llmCache = llmCache
	}

	// v20.4: API Gateway 引擎
	var apiGateway *APIGateway
	if cfg.APIGateway.Enabled {
		apiGateway = NewAPIGateway(db, cfg.APIGateway)
		fmt.Printf("[初始化] ✅ API Gateway 已启用 (JWT=%v, APIKey=%v, 路由=%d)\n",
			cfg.APIGateway.JWTEnabled, cfg.APIGateway.APIKeyEnabled, len(apiGateway.ListRoutes()))
	} else {
		fmt.Println("[初始化] ⚠️ API Gateway: 未启用")
	}
	mgmtAPI.apiGateway = apiGateway
	if llmProxy != nil {
		llmProxy.apiGateway = apiGateway
	}

	// v21.0: K8s 服务发现
	var k8sDiscovery *K8sDiscovery
	if cfg.Discovery.Kubernetes.Enabled {
		var err error
		k8sDiscovery, err = NewK8sDiscovery(cfg, pool)
		if err != nil {
			log.Printf("[K8s发现] ⚠️ 初始化失败（将继续运行但不启用发现）: %v", err)
		} else {
			fmt.Printf("[初始化] ✅ K8s 服务发现: namespace=%s, service=%s, interval=%ds\n",
				cfg.Discovery.Kubernetes.Namespace, cfg.Discovery.Kubernetes.Service,
				func() int { if cfg.Discovery.Kubernetes.SyncInterval > 0 { return cfg.Discovery.Kubernetes.SyncInterval }; return 15 }())
		}
	} else {
		fmt.Println("[初始化] ⚠️ K8s 服务发现: 未启用")
	}

	mgmtAPI.honeypotDeep = honeypotDeep
	mgmtAPI.pathPolicyEngine = pathPolicyEngine // v23.0
	mgmtAPI.k8sDiscovery = k8sDiscovery // v21.0
	inbound.honeypotDeep = honeypotDeep
	mgmtAPI.semanticDetector = semanticDetector
	inbound.semanticDetector = semanticDetector
	if llmProxy != nil {
		llmProxy.semanticDetector = semanticDetector
	}
	// v19.1: 将语义检测阶段追加到检测 Pipeline 末尾
	if semanticDetector != nil && inbound.pipeline != nil {
		inbound.pipeline.stages = append(inbound.pipeline.stages, NewSemanticStage(semanticDetector))
	}

	// v18.0: 后台调度器（攻击链自动分析 + 行为画像自动扫描）
	bgScheduler := NewBackgroundScheduler(cfg, attackChainEng, behaviorProfileEng)
	chainMin := cfg.ChainAnalysisIntervalMin
	if chainMin <= 0 { chainMin = 5 }
	behaviorMin := cfg.BehaviorScanIntervalMin
	if behaviorMin <= 0 { behaviorMin = 10 }
	fmt.Printf("[初始化] ✅ 后台调度器已就绪 (攻击链分析: %d 分钟, 行为画像扫描: %d 分钟)\n", chainMin, behaviorMin)

	fmt.Println("[初始化] ✅ 报告引擎已就绪 (日报/周报/月报)")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.HealthCheck(ctx)
	// v21.0: 启动 K8s 服务发现
	if k8sDiscovery != nil {
		go k8sDiscovery.Run(ctx)
	}
	if inbound.limiter != nil { go inbound.limiter.startCleanup(ctx) }

	// v18.0: 启动后台调度器
	bgScheduler.Start(ctx)

	// v4.2: 设置关闭管理器的 cancel
	shutdownMgr.SetCancel(cancel)
	shutdownMgr.SetWSProxy(wsProxy)

	// 日志轮转
	go func() { defer func() { recover() }(); logger.CleanupOldLogs(retentionDays) }()
	go func() {
		ticker := time.NewTicker(24 * time.Hour); defer ticker.Stop()
		for { select { case <-ctx.Done(): return; case <-ticker.C: logger.CleanupOldLogs(retentionDays) } }
	}()

	// v5.0: 审计日志归档
	if cfg.AuditArchiveEnabled {
		archiveDir := cfg.AuditArchiveDir
		if archiveDir == "" { archiveDir = "/var/lib/lobster-guard/archives/" }
		fmt.Printf("[初始化] ✅ 审计归档: 目录 %s, 保留 %d 天\n", archiveDir, retentionDays)
		// 启动时立即归档一次
		go func() {
			defer func() { recover() }()
			path, deleted, err := logger.ArchiveLogs(retentionDays, archiveDir)
			if err != nil {
				log.Printf("[归档] 启动归档失败: %v", err)
			} else if path != "" {
				log.Printf("[归档] ✅ 启动归档完成: %s，删除 %d 条", path, deleted)
			}
		}()
		// 每天归档
		go func() {
			ticker := time.NewTicker(24 * time.Hour); defer ticker.Stop()
			for {
				select {
				case <-ctx.Done(): return
				case <-ticker.C:
					path, deleted, err := logger.ArchiveLogs(retentionDays, archiveDir)
					if err != nil {
						log.Printf("[归档] 定时归档失败: %v", err)
					} else if path != "" {
						log.Printf("[归档] ✅ 定时归档完成: %s，删除 %d 条", path, deleted)
					}
				}
			}
		}()
	}

	// Bridge 模式
	if cfg.Mode == "bridge" {
		if !channel.SupportsBridge() { log.Fatalf("[错误] %s 通道不支持 bridge 模式", channel.Name()) }
		go func() {
			if err := inbound.startBridge(ctx); err != nil && err != context.Canceled { log.Fatalf("[错误] 启动桥接失败: %v", err) }
		}()
		shutdownMgr.SetBridge(inbound.bridge)
	}

	// v4.2: 自动备份
	if cfg.BackupAutoInterval > 0 {
		backupDir := cfg.BackupDir
		if backupDir == "" { backupDir = "/var/lib/lobster-guard/backups/" }
		maxCount := cfg.BackupMaxCount
		if maxCount <= 0 { maxCount = 10 }
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.BackupAutoInterval) * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					path, size, err := store.Backup(backupDir)
					if err != nil {
						log.Printf("[自动备份] 失败: %v", err)
					} else {
						log.Printf("[自动备份] ✅ 已创建: %s (%.2f MB)", path, float64(size)/1024/1024)
						CleanupOldBackups(backupDir, maxCount)
					}
				}
			}
		}()
		fmt.Printf("[初始化] ✅ 自动备份: 每 %d 小时, 最多保留 %d 份\n", cfg.BackupAutoInterval, maxCount)
	}

	// 启动 HTTP 服务
	inSrv := &http.Server{Addr: cfg.InboundListen, Handler: inbound, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	outSrv := &http.Server{Addr: cfg.OutboundListen, Handler: outbound, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	mgmtSrv := &http.Server{Addr: cfg.ManagementListen, Handler: mgmtAPI, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}

	// v4.2: 注册服务器到关闭管理器
	shutdownMgr.SetServers(inSrv, outSrv, mgmtSrv)

	go func() { if err := inSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("入站代理启动失败: %v", err) } }()
	go func() { if err := outSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("出站代理启动失败: %v", err) } }()
	go func() { if err := mgmtSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("管理API启动失败: %v", err) } }()

	log.Printf("[启动完成] 龙虾卫士 v%s 已就绪 (入站=%s 出站=%s 管理=%s log_format=%s)", AppVersion, cfg.InboundListen, cfg.OutboundListen, cfg.ManagementListen, slog.Format())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[关闭] 收到信号 %v，正在优雅关闭...", sig)

	// v31.0: 停止自动复核管理器
	if autoReviewMgr != nil {
		autoReviewMgr.Stop()
	}
	// v20.1: 停止污染追踪引擎
	if taintTracker != nil {
		taintTracker.Stop()
	}
	// v25.0: 停止计划编译器
	if planCompiler != nil {
		planCompiler.Stop()
	}
	// v19.0: 停止自进化引擎
	if evolutionEngine != nil {
		evolutionEngine.StopAutoEvolution()
	}
	// v18.1: 停止事件总线
	if eventBus != nil {
		eventBus.Stop()
	}
	// v18.0: 停止后台调度器
	bgScheduler.Stop()
	// v4.2: 使用 ShutdownManager 优雅关闭
	if llmProxy != nil {
		llmProxy.Stop()
	}
	shutdownMgr.Shutdown()
}

// printInboundRuleSummary 打印入站规则摘要
func printInboundRuleSummary(engine *RuleEngine) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	keywordCount := len(engine.rules)
	regexCount := len(engine.regexRules)
	groups := make(map[string]bool)
	for _, r := range engine.rules { if r.Group != "" { groups[r.Group] = true } }
	for _, r := range engine.regexRules { if r.Group != "" { groups[r.Group] = true } }
	fmt.Printf("[初始化] ✅ 入站规则: %d 条 (keyword: %d, regex: %d, 分组: %d)\n",
		keywordCount+regexCount, keywordCount, regexCount, len(groups))
}

// printOutboundRuleSummary 打印出站规则摘要
func printOutboundRuleSummary(rules []OutboundRuleConfig) {
	block, warn, logCount := 0, 0, 0
	for _, r := range rules {
		switch r.Action {
		case "block": block++
		case "warn": warn++
		case "log": logCount++
		}
	}
	fmt.Printf("[初始化] ✅ 出站规则: block %d / warn %d / log %d\n", block, warn, logCount)
}

// printPIISummary 打印 PII 模式摘要
func printPIISummary(engine *RuleEngine) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if len(engine.piiNames) > 0 {
		fmt.Printf("[初始化] ✅ PII 模式: %d 条 (%s)\n", len(engine.piiNames), strings.Join(engine.piiNames, "/"))
	}
}

// printRateLimitSummary 打印限流摘要
func printRateLimitSummary(cfg *Config) {
	if cfg.RateLimit.GlobalRPS > 0 || cfg.RateLimit.PerSenderRPS > 0 {
		fmt.Printf("[初始化] ✅ 限流: 全局 %.0f rps, 用户 %.0f rps\n", cfg.RateLimit.GlobalRPS, cfg.RateLimit.PerSenderRPS)
	} else {
		fmt.Println("[初始化] ⚠️ 限流: 未配置")
	}
}
