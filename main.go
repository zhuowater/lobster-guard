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
	AppVersion = "4.1.0"
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
	flag.Parse()

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

	logger, err := NewAuditLogger(db)
	if err != nil { log.Fatalf("初始化审计日志失败: %v", err) }
	defer logger.Close()

	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, cfg.RoutePersist)
	for _, up := range pool.ListUpstreams() {
		pool.IncrUserCount(up.ID, routes.CountByUpstream(up.ID))
	}

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

	// 创建代理
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, metrics, ruleHits, userCache, policyEng)
	outbound, err := NewOutboundProxy(cfg, channel, engine, outboundEngine, logger, metrics, ruleHits)
	if err != nil { log.Fatalf("初始化出站代理失败: %v", err) }

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

	mgmtAPI := NewManagementAPI(cfg, *cfgPath, pool, routes, logger, engine, outboundEngine, inbound, channel, metrics, ruleHits, userCache, policyEng, alertNotifier, wsProxy)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.HealthCheck(ctx)
	if inbound.limiter != nil { go inbound.limiter.startCleanup(ctx) }

	// 日志轮转
	go func() { defer func() { recover() }(); logger.CleanupOldLogs(retentionDays) }()
	go func() {
		ticker := time.NewTicker(24 * time.Hour); defer ticker.Stop()
		for { select { case <-ctx.Done(): return; case <-ticker.C: logger.CleanupOldLogs(retentionDays) } }
	}()

	// Bridge 模式
	if cfg.Mode == "bridge" {
		if !channel.SupportsBridge() { log.Fatalf("[错误] %s 通道不支持 bridge 模式", channel.Name()) }
		go func() {
			if err := inbound.startBridge(ctx); err != nil && err != context.Canceled { log.Fatalf("[错误] 启动桥接失败: %v", err) }
		}()
	}

	// 启动 HTTP 服务
	inSrv := &http.Server{Addr: cfg.InboundListen, Handler: inbound, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	outSrv := &http.Server{Addr: cfg.OutboundListen, Handler: outbound, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	mgmtSrv := &http.Server{Addr: cfg.ManagementListen, Handler: mgmtAPI, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}

	go func() { if err := inSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("入站代理启动失败: %v", err) } }()
	go func() { if err := outSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("出站代理启动失败: %v", err) } }()
	go func() { if err := mgmtSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("管理API启动失败: %v", err) } }()

	log.Printf("[启动完成] 龙虾卫士 v%s 已就绪 (入站=%s 出站=%s 管理=%s)", AppVersion, cfg.InboundListen, cfg.OutboundListen, cfg.ManagementListen)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[关闭] 收到信号 %v，正在优雅关闭...", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	cancel()
	if inbound.bridge != nil { inbound.bridge.Stop() }
	inSrv.Shutdown(shutdownCtx)
	outSrv.Shutdown(shutdownCtx)
	mgmtSrv.Shutdown(shutdownCtx)
	log.Println("[关闭] 龙虾卫士已停止")
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
