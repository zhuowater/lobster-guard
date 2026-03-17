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

### v6.x — Dashboard 企业级演进
- [x] v6.0 导航与布局重构（侧边栏 220px 可折叠 · Hash 路由 7 页面 · 全局搜索 Ctrl+K · 概览首页重设计）
- [x] v6.1 Vue 3 + Vite 重构（单 HTML → 20 文件 Vue 项目 · DataTable 排序分页 · ConfirmModal 确认框）
- [x] v6.2 可视化升级（TrendChart SVG 折线 · PieChart 环形饼图 · HeatMap 7×24 热力图 · 24h/7d 切换）
- [x] v6.3 规则管理可视化（RuleEditor · RegexTester · 规则 CRUD API · YAML 导入导出 · 策略路由可视化 · 4 场景模板）

### v7.x — Dashboard UI 品质打磨
- [x] v7.0 视觉基础（SVG 图标系统 · 语义色彩 · 排版层级 · StatCard 重设计 · 运行时间格式化 · 空状态引导 · 按钮规范 · 图表网格线 · 页面过渡动画）
- [x] v7.1 配色升级 + 交互改造（Indigo 主色 · 路由页全面重做：策略路由增删改按钮 · 亲和路由模态框交互 · UpstreamSelect 上游选择器 · BindModal 通用表单 · 设置页 Token 密码框 · 规则页行级操作按钮）
- [x] v7.2 数据与体验（演示数据注入 API · StatCard 数字滚动动画 · JsonHighlight 审计日志语法高亮 · Skeleton 骨架屏 · 零数据兜底 · 列表过渡动画 · Settings 运行时间格式化）

### v8.x — 运维工具箱 + 策略路由管理
- [x] v8.0 运维工具箱（/ops 页面 4 Tab：配置 YAML 高亮脱敏 · 备份全生命周期 · 系统诊断 · 告警设置 · 6 个新 API）
- [x] v8.1 策略路由完整 CRUD（POST/PUT/DELETE route-policies · 自动写回 config.yaml · PolicyModal 前端 · 操作按钮启用）

**当前状态**: v8.1.0 · Go 24 文件 ~11,500 行 · Vue 27+ 文件 ~4,500 行 · 测试全过 · 3 个依赖 · Dashboard 8 页面

---

## 计划

### v9.0 — Agent 行为审计
> 安全网关的核心价值：不只拦截攻击，还要审计 Agent 的 tool_call 调用链

- [ ] tool_call 调用链解析（从 OpenClaw 响应中提取 tool_use/tool_result）
- [ ] 调用链可视化（时间线 + 工具名 + 参数摘要 + 结果状态）
- [ ] 高危工具告警（exec/shell/file_write/http_request 等标记为高危）
- [ ] 审计日志扩展（新增 tool_calls 字段，SQLite 存储）
- [ ] Dashboard 新增"Agent 行为"页面

### v10.0 — 合规报告
> 自动生成安全审计报告，给领导看的不是给开发者看的

- [ ] 报告模板系统（日报/周报/月报）
- [ ] 数据聚合（拦截趋势 · 攻击类型分布 · 高风险用户 TOP10 · 规则命中排行）
- [ ] 报告导出（HTML + PDF）
- [ ] 定时生成 + webhook 推送

### v11.0 — 多租户与权限
- [ ] 角色系统（管理员/运维/观察者/Bot 管理员）
- [ ] 登录认证（用户名密码 / SSO）
- [ ] 会话管理 + 操作审计

### v12.0 — 暗/亮主题 + 移动端
- [ ] CSS 变量主题系统（暗色/亮色一键切换）
- [ ] 移动端响应式优化（侧边栏抽屉 · 卡片堆叠布局）
- [ ] PWA 支持（离线缓存 · 添加到主屏幕）

### 未来探索
- [ ] PostgresStore + 多实例部署（路由同步 · Leader 选举）
- [ ] API Gateway 能力（认证中间件 · 灰度发布 · 请求转换）
- [ ] Slack / Teams / Telegram 通道插件
- [ ] OpenAPI/Swagger 文档自动生成
- [ ] 多语言检测规则（i18n 攻击模式）
- [ ] 攻击者画像（高风险用户列表 · 行为分析 · IP 地理分布）
- [ ] 策略路由 SVG 流程图可视化
- [ ] 拖拽调整面板顺序

---

## 版本发布原则

1. **每个版本都可独立使用** — 不存在"必须升级到 vX 才能用"
2. **向后兼容** — 新版本不破坏旧配置
3. **实战驱动** — 优先解决真实部署中暴露的问题
4. **测试先行** — 每个 feature 必须有测试覆盖
5. **文档同步** — 代码改完必须同步更新 README/config.example
6. **架构演进有节奏** — 功能版本和架构版本交替
7. **UI 品质标杆** — 对标 Grafana/DataDog/Cloudflare，不接受"开发者审美"

## 版本演进逻辑

```
v1-v2: 核心功能（代理 + 检测 + 路由）
v3:    企业级功能（多通道 + 规则引擎 + IM集成）
v4:    架构治理（代码拆分 + WS代理 + 高可用）
v5:    可观测性（metrics + 智能检测管线）
v6:    Dashboard 功能（导航 + Vue重构 + 可视化 + 规则管理）
v7:    Dashboard 品质（UI打磨 + 配色升级 + 交互改造 + 数据体验）✅
v8:    运维工具箱 + 策略路由管理 ✅
v9:    Agent 行为审计（tool_call 调用链 + 高危工具告警）← 下一个
v10:   合规报告（自动生成审计报告 + 导出）
v11:   企业级权限（RBAC + SSO + 审计）
v12:   主题 + 移动端（暗/亮切换 + PWA）
```
