// main.go — 入口 main()、banner、CLI 参数解析、启动流程
// lobster-guard v4.0 代码拆分
// P2 重构：引擎初始化移至 init_engines.go
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
	AppVersion = "36.5"
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

	// --- CLI 快捷命令 ---
	if *showVersion {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		return
	}
	if *genRulesFile != "" {
		handleGenRules(*genRulesFile)
		return
	}

	printBanner()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if errs := validateConfig(cfg); len(errs) > 0 {
		for _, e := range errs {
			log.Printf("[配置错误] ❌ %s", e)
		}
		log.Fatalf("配置验证失败，共 %d 个错误", len(errs))
	}
	if *checkConfig {
		handleCheckConfig(cfg)
		return
	}
	if *dumpRules {
		handleDumpRules(cfg)
		return
	}
	if *dumpRoutes {
		handleDumpRoutes(cfg)
		return
	}

	// --- 结构化日志 ---
	InitAppLogger(cfg.LogFormat)
	slog := GetAppLogger()

	// --- 备份恢复 ---
	if *restorePath != "" {
		log.Printf("[恢复] 从备份文件恢复: %s -> %s", *restorePath, cfg.DBPath)
		if err := RestoreFromBackup(*restorePath, cfg.DBPath); err != nil {
			log.Fatalf("恢复备份失败: %v", err)
		}
		log.Printf("[恢复] ✅ 数据库已从备份恢复")
	}

	// --- 通道插件 ---
	channelName := cfg.Channel
	if channelName == "" { channelName = "lanxin" }
	modeName := cfg.Mode
	if modeName == "" { modeName = "webhook" }
	channel := initChannel(cfg)
	fmt.Printf("[初始化] ✅ 通道插件: %s (%s 模式)\n", channelName, modeName)

	// --- 规则引擎 ---
	engine, outboundEngine := initRuleEngines(cfg)

	// --- 数据库 & Store ---
	db, err := initDB(cfg.DBPath)
	if err != nil { log.Fatalf("初始化数据库失败: %v", err) }
	defer db.Close()
	store := NewSQLiteStore(db, cfg.DBPath)

	// --- 租户 & 认证 ---
	tenantMgr := NewTenantManager(db)
	engine.SetTenantDB(db)
	engine.SetInboundTemplateDB(db)
	initIndustryTemplateSystem(db)
	engine.InitGlobalTemplateAC()
	outboundEngine.InitGlobalTemplateRules(db)

	authMgr := NewAuthManager(db, &cfg.Auth)
	if cfg.Auth.Enabled {
		fmt.Println("[初始化] ✅ 登录认证: 已启用")
	} else {
		fmt.Println("[初始化] ⚠️ 登录认证: 未启用（使用 Bearer token 模式）")
	}

	// --- 审计日志 ---
	logger, err := NewAuditLogger(db)
	if err != nil { log.Fatalf("初始化审计日志失败: %v", err) }
	defer logger.Close()
	logger.SetTenantManager(tenantMgr)

	// --- LLM 代理（可选）---
	var llmAuditor *LLMAuditor
	var llmProxy *LLMProxy
	var llmRuleEngine *LLMRuleEngine
	if cfg.LLMProxy.Enabled {
		llmRuleEngine, llmAuditor, llmProxy = initLLMProxy(cfg, logger)
	} else {
		fmt.Println("[初始化] ⚠️ LLM 代理: 未启用")
	}

	// --- 关闭管理器 ---
	shutdownMgr := NewShutdownManager(cfg)
	shutdownMgr.SetLogger(logger)
	shutdownMgr.SetStore(store)

	// --- 上游池 & 路由表 ---
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, cfg.RoutePersist)
	pool.RestoreUserCounts(db)

	// --- 蜜罐引擎（需要在 initAllEngines 之前）---
	honeypotEngine := NewHoneypotEngine(logger.DB())
	honeypotEngine.SetEnabled(cfg.Honeypot.Enabled)

	// === 核心：集中初始化所有安全引擎 ===
	engines := initAllEngines(cfg, store, logger, pool, routes, engine, outboundEngine, llmRuleEngine, llmAuditor, llmProxy, tenantMgr, honeypotEngine)

	// --- 创建代理 ---
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, engines.Metrics, engines.RuleHits, engines.UserCache, engines.PolicyEng, honeypotEngine)
	engines.wireInbound(inbound, cfg, engine, tenantMgr)

	outbound, err := NewOutboundProxy(cfg, channel, engine, outboundEngine, logger, engines.Metrics, engines.RuleHits, honeypotEngine)
	if err != nil { log.Fatalf("初始化出站代理失败: %v", err) }
	engines.wireOutbound(outbound, routes)

	// --- WebSocket 代理 ---
	wsProxy := NewWSProxyManager(cfg, engine, outboundEngine, logger, engines.Metrics, pool, routes, engines.RuleHits)
	inbound.wsProxy = wsProxy
	printWSConfig(cfg)

	// --- 管理 API ---
	mgmtAPI := NewManagementAPI(cfg, *cfgPath, pool, routes, logger, engine, outboundEngine, inbound, channel, engines.Metrics, engines.RuleHits, engines.UserCache, engines.PolicyEng, engines.AlertNotifier, wsProxy, store, shutdownMgr, engines.Realtime)
	engines.wireMgmtAPI(mgmtAPI, tenantMgr, authMgr, llmAuditor, llmRuleEngine, llmProxy)

	// --- LLM Proxy 额外注入 ---
	engines.wireLLMProxy(llmProxy)

	// --- 打印摘要 ---
	printRateLimitSummary(cfg)
	printUpstreamSummary(pool)
	printAuditSummary(cfg)
	printMetricsSummary(cfg)
	printBridgeSummary(cfg, channelName)

	// --- 启动后台 goroutines ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.HealthCheck(ctx)
	if engines.K8sDiscovery != nil { go engines.K8sDiscovery.Run(ctx) }
	if inbound.limiter != nil { go inbound.limiter.startCleanup(ctx) }
	engines.BgScheduler.Start(ctx)
	shutdownMgr.SetCancel(cancel)
	shutdownMgr.SetWSProxy(wsProxy)

	// --- 日志轮转 & 归档 ---
	retentionDays := cfg.AuditRetentionDays
	if retentionDays <= 0 { retentionDays = 30 }
	startLogRotation(ctx, logger, retentionDays)
	if cfg.AuditArchiveEnabled {
		startAuditArchive(ctx, cfg, logger, retentionDays)
	}

	// --- Bridge 模式 ---
	if cfg.Mode == "bridge" {
		if !channel.SupportsBridge() {
			log.Fatalf("[错误] %s 通道不支持 bridge 模式", channel.Name())
		}
		go func() {
			if err := inbound.startBridge(ctx); err != nil && err != context.Canceled {
				log.Fatalf("[错误] 启动桥接失败: %v", err)
			}
		}()
		shutdownMgr.SetBridge(inbound.bridge)
	}

	// --- 自动备份 ---
	if cfg.BackupAutoInterval > 0 {
		startAutoBackup(ctx, cfg, store)
	}

	// --- HTTP 服务器 ---
	inSrv := &http.Server{Addr: cfg.InboundListen, Handler: inbound, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	outSrv := &http.Server{Addr: cfg.OutboundListen, Handler: outbound, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	mgmtSrv := &http.Server{Addr: cfg.ManagementListen, Handler: mgmtAPI, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	shutdownMgr.SetServers(inSrv, outSrv, mgmtSrv)

	go func() { if err := inSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("入站代理启动失败: %v", err) } }()
	go func() { if err := outSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("出站代理启动失败: %v", err) } }()
	go func() { if err := mgmtSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("管理API启动失败: %v", err) } }()

	log.Printf("[启动完成] 龙虾卫士 v%s 已就绪 (入站=%s 出站=%s 管理=%s log_format=%s)", AppVersion, cfg.InboundListen, cfg.OutboundListen, cfg.ManagementListen, slog.Format())

	// --- 等待信号 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[关闭] 收到信号 %v，正在优雅关闭...", sig)

	engines.stopAll()
	if llmProxy != nil { llmProxy.Stop() }
	shutdownMgr.Shutdown()
}

// ============================================================
// CLI 子命令处理
// ============================================================

func handleGenRules(path string) {
	rules := getDefaultInboundRules()
	rulesFile := InboundRulesFileConfig{Rules: rules}
	data, err := yaml.Marshal(&rulesFile)
	if err != nil { log.Fatalf("序列化规则失败: %v", err) }
	header := "# lobster-guard 默认入站规则文件\n# 由 lobster-guard -gen-rules 自动生成\n# 可自定义修改后通过 inbound_rules_file 配置项加载\n\n"
	if err := os.WriteFile(path, []byte(header+string(data)), 0644); err != nil {
		log.Fatalf("写入规则文件失败: %v", err)
	}
	fmt.Printf("✅ 默认入站规则已导出到: %s (%d 条规则, %d 个 pattern)\n", path, len(rules), countPatterns(rules))
}

func handleCheckConfig(cfg *Config) {
	fmt.Println("✅ 配置文件验证通过")
	fmt.Printf("  通道: %s\n", func() string { if cfg.Channel == "" { return "lanxin" }; return cfg.Channel }())
	fmt.Printf("  模式: %s\n", func() string { if cfg.Mode == "" { return "webhook" }; return cfg.Mode }())
	fmt.Printf("  入站监听: %s\n", cfg.InboundListen)
	fmt.Printf("  出站监听: %s\n", cfg.OutboundListen)
	fmt.Printf("  管理监听: %s\n", cfg.ManagementListen)
	fmt.Printf("  数据库: %s\n", cfg.DBPath)
	fmt.Printf("  上游数: %d\n", len(cfg.StaticUpstreams))
	fmt.Printf("  日志格式: %s\n", func() string { if cfg.LogFormat == "" { return "text" }; return cfg.LogFormat }())
}

func handleDumpRules(cfg *Config) {
	rules, source, err := resolveInboundRules(cfg)
	if err != nil { log.Fatalf("加载入站规则失败: %v", err) }
	if rules == nil { rules = getDefaultInboundRules(); source = "default" }
	fmt.Printf("=== 入站规则 (来源: %s, %d 条) ===\n", source, len(rules))
	for _, r := range rules {
		typeStr := r.Type; if typeStr == "" { typeStr = "keyword" }
		fmt.Printf("  [%s] %s (%s) patterns=%d priority=%d group=%q\n", r.Action, r.Name, typeStr, len(r.Patterns), r.Priority, r.Group)
	}
	fmt.Printf("\n=== 出站规则 (%d 条) ===\n", len(cfg.OutboundRules))
	for _, r := range cfg.OutboundRules {
		pCount := 0; if r.Pattern != "" { pCount = 1 }; if len(r.Patterns) > 0 { pCount = len(r.Patterns) }
		fmt.Printf("  [%s] %s patterns=%d priority=%d\n", r.Action, r.Name, pCount, r.Priority)
	}
}

func handleDumpRoutes(cfg *Config) {
	db, err := initDB(cfg.DBPath)
	if err != nil { log.Fatalf("初始化数据库失败: %v", err) }
	defer db.Close()
	routes := NewRouteTable(db, cfg.RoutePersist)
	entries := routes.ListRoutes()
	fmt.Printf("=== 路由表 (%d 条) ===\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  sender=%s app=%s -> upstream=%s (dept=%s name=%s)\n", e.SenderID, e.AppID, e.UpstreamID, e.Department, e.DisplayName)
	}
}

// ============================================================
// 初始化辅助函数
// ============================================================

func initChannel(cfg *Config) ChannelPlugin {
	switch cfg.Channel {
	case "feishu":
		return NewFeishuPlugin(cfg.FeishuEncryptKey, cfg.FeishuVerificationToken)
	case "dingtalk":
		return NewDingtalkPlugin(cfg.DingtalkToken, cfg.DingtalkAesKey, cfg.DingtalkCorpId)
	case "wecom":
		return NewWecomPlugin(cfg.WecomToken, cfg.WecomEncodingAesKey, cfg.WecomCorpId)
	case "generic":
		return NewGenericPlugin(cfg.GenericSenderHeader, cfg.GenericTextField)
	default:
		crypto, err := NewLanxinCrypto(cfg.CallbackKey, cfg.CallbackSignToken)
		if err != nil { log.Fatalf("初始化蓝信加解密失败: %v", err) }
		return NewLanxinPlugin(crypto)
	}
}

func initRuleEngines(cfg *Config) (*RuleEngine, *OutboundRuleEngine) {
	inboundRules, inboundSource, err := resolveInboundRules(cfg)
	if err != nil { log.Fatalf("加载入站规则失败: %v", err) }
	var engine *RuleEngine
	if inboundRules != nil {
		engine = NewRuleEngineWithPII(inboundRules, inboundSource, cfg.OutboundPIIPatterns, cfg.RuleBindings)
	} else {
		engine = NewRuleEngineWithPII(getDefaultInboundRules(), "default", cfg.OutboundPIIPatterns, cfg.RuleBindings)
	}
	printInboundRuleSummary(engine)

	outboundEngine := NewOutboundRuleEngine(cfg.OutboundRules)
	printOutboundRuleSummary(cfg.OutboundRules)
	printPIISummary(engine)

	return engine, outboundEngine
}

func initLLMProxy(cfg *Config, logger *AuditLogger) (*LLMRuleEngine, *LLMAuditor, *LLMProxy) {
	llmRules := mergeLLMRuleDefaults(cfg.LLMProxy.Rules)
	llmRuleEngine := NewLLMRuleEngine(llmRules)
	llmRuleEngine.SetDB(logger.DB())
	llmRuleEngine.SetTenantDB(logger.DB())
	llmRuleEngine.SetTemplateDB(logger.DB())
	initIndustryTemplateSystem(logger.DB())
	llmRuleEngine.InitGlobalLLMTemplateRules()
	log.Printf("[初始化] ✅ LLM 规则引擎: %d 条规则 (用户%d+默认补充)", len(llmRules), len(cfg.LLMProxy.Rules))

	llmAuditor := NewLLMAuditor(logger.DB(), cfg.LLMProxy.AuditConfig, &cfg.LLMProxy)
	llmProxy := NewLLMProxy(cfg.LLMProxy, llmAuditor, llmRuleEngine)
	llmProxy.mainCfg = cfg
	go func() {
		if err := llmProxy.Start(); err != nil {
			log.Printf("[LLM代理] 启动失败: %v", err)
		}
	}()
	log.Printf("[初始化] ✅ LLM 代理已启动: %s (%d 个 target)", cfg.LLMProxy.Listen, len(cfg.LLMProxy.Targets))
	return llmRuleEngine, llmAuditor, llmProxy
}

// ============================================================
// 后台调度辅助
// ============================================================

func startLogRotation(ctx context.Context, logger *AuditLogger, retentionDays int) {
	go func() { defer func() { recover() }(); logger.CleanupOldLogs(retentionDays) }()
	go func() {
		ticker := time.NewTicker(24 * time.Hour); defer ticker.Stop()
		for { select { case <-ctx.Done(): return; case <-ticker.C: logger.CleanupOldLogs(retentionDays) } }
	}()
}

func startAuditArchive(ctx context.Context, cfg *Config, logger *AuditLogger, retentionDays int) {
	archiveDir := cfg.AuditArchiveDir
	if archiveDir == "" { archiveDir = "/var/lib/lobster-guard/archives/" }
	fmt.Printf("[初始化] ✅ 审计归档: 目录 %s, 保留 %d 天\n", archiveDir, retentionDays)
	go func() {
		defer func() { recover() }()
		path, deleted, err := logger.ArchiveLogs(retentionDays, archiveDir)
		if err != nil { log.Printf("[归档] 启动归档失败: %v", err) } else if path != "" { log.Printf("[归档] ✅ 启动归档完成: %s，删除 %d 条", path, deleted) }
	}()
	go func() {
		ticker := time.NewTicker(24 * time.Hour); defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				path, deleted, err := logger.ArchiveLogs(retentionDays, archiveDir)
				if err != nil { log.Printf("[归档] 定时归档失败: %v", err) } else if path != "" { log.Printf("[归档] ✅ 定时归档完成: %s，删除 %d 条", path, deleted) }
			}
		}
	}()
}

func startAutoBackup(ctx context.Context, cfg *Config, store *SQLiteStore) {
	backupDir := cfg.BackupDir
	if backupDir == "" { backupDir = "/var/lib/lobster-guard/backups/" }
	maxCount := cfg.BackupMaxCount
	if maxCount <= 0 { maxCount = 10 }
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.BackupAutoInterval) * time.Hour); defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				path, size, err := store.Backup(backupDir)
				if err != nil { log.Printf("[自动备份] 失败: %v", err) } else {
					log.Printf("[自动备份] ✅ 已创建: %s (%.2f MB)", path, float64(size)/1024/1024)
					CleanupOldBackups(backupDir, maxCount)
				}
			}
		}
	}()
	fmt.Printf("[初始化] ✅ 自动备份: 每 %d 小时, 最多保留 %d 份\n", cfg.BackupAutoInterval, maxCount)
}

// ============================================================
// 打印摘要辅助
// ============================================================

func printWSConfig(cfg *Config) {
	wsMode := cfg.WSMode; if wsMode == "" { wsMode = "inspect" }
	wsMaxConn := cfg.WSMaxConnections; if wsMaxConn <= 0 { wsMaxConn = 100 }
	fmt.Printf("[初始化] ✅ WebSocket 代理: mode=%s, max_connections=%d, idle_timeout=%ds, max_duration=%ds\n",
		wsMode, wsMaxConn,
		func() int { if cfg.WSIdleTimeout <= 0 { return 300 }; return cfg.WSIdleTimeout }(),
		func() int { if cfg.WSMaxDuration <= 0 { return 3600 }; return cfg.WSMaxDuration }())
}

func printUpstreamSummary(pool *UpstreamPool) {
	upTotal, _ := pool.Count()
	upIDs := make([]string, 0)
	for _, u := range pool.ListUpstreams() { upIDs = append(upIDs, u.ID) }
	fmt.Printf("[初始化] ✅ 上游: %d 个静态 (%s)\n", upTotal, strings.Join(upIDs, ", "))
}

func printAuditSummary(cfg *Config) {
	retentionDays := cfg.AuditRetentionDays; if retentionDays <= 0 { retentionDays = 30 }
	alertDesc := "未配置"; if cfg.AlertWebhook != "" { alertDesc = cfg.AlertWebhook }
	fmt.Printf("[初始化] ✅ 审计: 保留 %d 天, 告警 webhook: %s\n", retentionDays, alertDesc)
}

func printMetricsSummary(cfg *Config) {
	if cfg.IsMetricsEnabled() {
		fmt.Printf("[初始化] ✅ Prometheus: %s/metrics\n", cfg.ManagementListen)
	} else {
		fmt.Println("[初始化] ⚠️ Prometheus: 未启用")
	}
}

func printBridgeSummary(cfg *Config, channelName string) {
	if cfg.Mode == "bridge" {
		fmt.Printf("[初始化] ✅ Bridge Mode: %s 长连接\n", channelName)
	} else {
		fmt.Println("[初始化] ⚠️ Bridge Mode: 未启用")
	}
}

func printInboundRuleSummary(engine *RuleEngine) {
	engine.mu.RLock(); defer engine.mu.RUnlock()
	keywordCount := len(engine.rules); regexCount := len(engine.regexRules)
	groups := make(map[string]bool)
	for _, r := range engine.rules { if r.Group != "" { groups[r.Group] = true } }
	for _, r := range engine.regexRules { if r.Group != "" { groups[r.Group] = true } }
	fmt.Printf("[初始化] ✅ 入站规则: %d 条 (keyword: %d, regex: %d, 分组: %d)\n", keywordCount+regexCount, keywordCount, regexCount, len(groups))
}

func printOutboundRuleSummary(rules []OutboundRuleConfig) {
	block, warn, logCount := 0, 0, 0
	for _, r := range rules {
		switch r.Action { case "block": block++; case "warn": warn++; case "log": logCount++ }
	}
	fmt.Printf("[初始化] ✅ 出站规则: block %d / warn %d / log %d\n", block, warn, logCount)
}

func printPIISummary(engine *RuleEngine) {
	engine.mu.RLock(); defer engine.mu.RUnlock()
	if len(engine.piiNames) > 0 {
		fmt.Printf("[初始化] ✅ PII 模式: %d 条 (%s)\n", len(engine.piiNames), strings.Join(engine.piiNames, "/"))
	}
}

func printRateLimitSummary(cfg *Config) {
	if cfg.RateLimit.GlobalRPS > 0 || cfg.RateLimit.PerSenderRPS > 0 {
		fmt.Printf("[初始化] ✅ 限流: 全局 %.0f rps, 用户 %.0f rps\n", cfg.RateLimit.GlobalRPS, cfg.RateLimit.PerSenderRPS)
	} else {
		fmt.Println("[初始化] ⚠️ 限流: 未配置")
	}
}
