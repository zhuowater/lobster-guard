# lobster-guard Roadmap

## 已完成

### v1.0 — 基础安全代理
- [x] 入站 Prompt Injection 检测（AC 自动机 40+ 规则）
- [x] 出站敏感信息审计（PII/凭据/私钥）
- [x] 单上游代理模式 · SQLite 审计日志 · AES-256-CBC 加解密

### v2.0 — 多容器与管理
- [x] 出站 block/warn/log 三级策略
- [x] 用户 ID 亲和路由（多容器负载均衡）
- [x] 容器自动注册/心跳/故障转移 · 管理 API · Web Dashboard

### v3.x — 多通道 + 规则引擎 + 企业级路由
- [x] v3.0 多通道插件架构（蓝信/飞书/钉钉/企微/通用HTTP）
- [x] v3.1 Bridge Mode（飞书/钉钉 WSS 长连接桥接）
- [x] v3.2 企微 GET 验证 + 健壮性
- [x] v3.3 Rate Limiting（令牌桶 · 全局/每用户 · 白名单）
- [x] v3.4 Prometheus Metrics（13 指标族 · 零依赖）
- [x] v3.5 入站规则热更新（外部 YAML · AC 自动机在线重建）
- [x] v3.6 规则引擎增强（优先级 · 自定义拦截消息 · 命中率统计）
- [x] v3.7 蓝信实战集成验证（双向全链路实测）
- [x] v3.8 多 Bot 亲和路由（复合键路由 · 批量绑定 · 路由统计）
- [x] v3.9 IM 用户信息自动获取（4 平台 UserInfoProvider · 邮箱/部门策略路由）
- [x] v3.10 审计日志增强 + 告警通知（导出/轮转/webhook 推送/时间线图/全文搜索）
- [x] v3.11 正则规则 + 规则分组（按 app_id 绑定规则组 · PII 可配置化）

### v4.x — 架构演进 + 高可用
- [x] v4.0 代码拆分 + 插件化（13 文件 · go:embed Dashboard · 配置验证器）
- [x] v4.1 WebSocket 消息流代理（inspect/passthrough · 帧级检测 · 连接生命周期）
- [x] v4.2 高可用基础设施（优雅关闭 · 5 维健康检查 · 数据备份/恢复 · Store 抽象层）

### v5.x — 可观测性 + 智能检测
- [x] v5.0 可观测性 + 运维增强（结构化日志 · trace_id 追踪 · 审计归档 · 实时监控大屏 · CLI 工具模式）
- [x] v5.1 智能检测（规则模板库 66 条 · DetectPipeline 检测链 · 上下文感知 · 可选 LLM · 检测缓存）

**当前状态**: v5.1.0 · 24 个源文件 ~11,400 行 · 472 个测试 · 45 commit · 3 个依赖

---

## 计划

### v5.2 — 多实例部署
> 真正的分布式高可用。前提：v4.2 的 Store 抽象层已就绪。

- [ ] PostgresStore 实现（替代 SQLite，支持多实例共享）
- [ ] 路由表跨实例同步（基于 PostgreSQL LISTEN/NOTIFY）
- [ ] Leader 选举（Bridge 模式防重复连接 · 基于 advisory lock）
- [ ] 配置中心集成（从 etcd/consul 读取配置 · 热更新）

### v5.3 — API Gateway 能力
> 从安全网关扩展为轻量 API Gateway。

- [ ] API 限流增强（按路径/方法 · 滑动窗口 · 令牌桶可选）
- [ ] 请求/响应转换（header 注入 · body 改写 · 路径重写）
- [ ] 认证中间件（API Key · JWT · OAuth2 · 蓝信 SSO）
- [ ] 灰度发布（按比例/按用户/按部门路由到不同上游版本）

### 未来探索
- [ ] Slack / Teams / Telegram 通道插件
- [ ] 可视化规则编辑器（拖拽式 · 规则沙箱测试）
- [ ] OpenAPI/Swagger 文档自动生成
- [ ] 多语言检测规则（i18n 攻击模式）
- [ ] Agent 行为审计（tool_call 调用链审计）
- [ ] 合规报告生成（自动生成安全审计报告 · PDF/HTML）

---

## 版本发布原则

1. **每个版本都可独立使用** — 不存在"必须升级到 vX 才能用"
2. **向后兼容** — 新版本不破坏旧配置
3. **实战驱动** — 优先解决真实部署中暴露的问题
4. **测试先行** — 每个 feature 必须有测试覆盖
5. **文档同步** — 代码改完必须同步更新 README/config.example
6. **架构演进有节奏** — 功能版本和架构版本交替
