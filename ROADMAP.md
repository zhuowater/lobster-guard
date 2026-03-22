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

### v9.x — 双安全域架构 · LLM 侧审计
- [x] v9.0 LLM 反向代理 + 独立审计（llm_proxy.go · llm_audit.go · SSE 流式拦截 · 独立 SQLite 表 · 6 个 /api/v1/llm/* API · Sidebar 域分组 · LLMOverview + AgentBehavior 页面）
- [x] v9.1 LLM 安全域增强（Settings LLM 配置区 · 成本看板 · 安全告警面板 · 模型定价 · 高危调用展开详情）

### v10.x — LLM 安全规则引擎 · 策略编排 ✅
> 对标 Guardrails AI 的验证器理念 + OWASP LLM Top 10 防护矩阵

- [x] v10.0 **LLM 规则引擎**（独立规则集 · 8 条内置规则 · keyword/regex · log/warn/block/rewrite · Shadow Mode · 规则 CRUD · 热更新 · LLMRules.vue）
- [x] v10.1 **策略编排**（🔥 Canary Token Prompt 泄露检测 · Response Budget Agent 行为预算 · 三层安全告警面板 · Settings 配置区 · 5 个新 API）

- [x] v10.2 **功能闭环修复**（告警→配置跳转链接 · Settings Tab 化 · Sidebar 可滚动 · 概览 StatCard 可点击 · config.yaml.example 补全）

### v11.x — 智能分析 · 攻击者画像 ✅
> 从"看数据"到"理解数据"

- [x] v11.0 **攻击者画像**（用户风险评分 0-100 · 5 维度加权 · 风险 TOP10 榜单 · 用户详情页（圆环评分 + 维度进度条 + 行为时间线）· 4 个新 API · 审计日志/概览页交叉导航 · 8 个演示用户差异化行为）

### v11.1 — 竞品洞察整合 · 驾驶舱升级 ✅
> 基于 NGSOC / 天眼 / 智慧防火墙实际登录体验（详见 docs/competitor-study/）

- [x] v11.1 **驾驶舱模式**（综合安全健康分 · OWASP LLM Top10 矩阵 · 严格模式一键切换 · 自动刷新频率 · 系统健康指标 · 通知中心）
  - **综合安全健康分**（借鉴 NGSOC 综合风险评分）：首页一个 0-100 大数字 + 7 天趋势线，综合 IM 拦截率 + LLM 异常率 + 规则命中 + Canary 泄露计算
  - **OWASP LLM Top 10 矩阵视图**（借鉴天眼 ATT&CK 矩阵）：LLM 概览页新增攻击矩阵，把规则和告警映射到 LLM01-LLM10
  - **"严格模式"一键切换**（借鉴天眼"重保模式"）：顶部按钮，一键把所有 warn→block、激活所有 Shadow 规则，再点恢复正常
  - **自动刷新频率设置**：概览/监控页加刷新频率下拉（30s / 1m / 5m / 手动）
  - **系统健康指标上首页**（借鉴防火墙状态卡片行）：CPU / 内存 / 磁盘进度条上 IM 概览页
  - **通知中心**：右上角铃铛图标，收集 Canary 泄露 / 预算超限 / 规则命中等告警未读数

### v11.2 — 异常检测 ✅
> 反直觉创新 🔥

- [x] v11.2 **异常基线检测**（6 指标 · 7 天滑动窗口 · 2σ/3σ 告警 · 24h 基线图 · ±2σ 带）
- [x] v11.3 **全面闭环修复**（OWASP 跳转修正 · LLM 规则 category 筛选 · 严格模式反馈 · 健康分分项跳转 · 异常卡片脉冲）
- [x] v11.4 **数据口径统一 + 测试补全**（全局时间选择器 · API since 参数 · time_range 标注 · 96 个新测试 · LLM 配置 UI 重构）
  - **基线学习**：连续运行 7 天后，自动建立"正常行为基线"（每小时请求量 · 工具调用分布 · Token 用量曲线）
  - **偏离告警**：当实际指标偏离基线 >2σ 时触发告警，无需手动配置阈值
  - 不依赖外部 ML 库，纯统计方法（滑动窗口均值 + 标准差），零依赖哲学

**当前状态 (2026-03-21)**: v20.7.0 · Go 70+50 文件 ~71,900 行 · Vue 65 文件 ~23,700 行 · 950 测试 · ~275+ API · 38 页面 + 21 组件 · 170 commits · 4 依赖 · 19 篇文档

---

## 计划

### v12.x — 合规报告 · 审计导出
> 给领导看的不是给开发者看的

- [x] v12.0 **报告模板引擎**（日报/周报/月报 · HTML 内联 CSS 邮件友好 · 聚合全数据源 · 智能建议 · iframe 预览 · LLM 审计导出 CSV/JSON · 闭环联动概览页+通知中心）

- [x] v12.1 **审计导出增强**（IM 导出加 from/to 时间范围 · 支持 since 简写 · QueryLogsExFull 完整查询接口 · 零依赖不加 XLSX，CSV 足够）

### v13.x — 会话回放 · Prompt 管理
> 对标 Langfuse 的 Tracing + Prompt Management

- [x] v13.0 **会话回放**（session_replay.go · 卡片式列表 · 竖线+节点视觉时间线 · 运维标签 · 审计页trace_id跳转 · 通知联动 · Icon.vue新增7图标）

- [x] v13.1 **Prompt 版本追踪**（SHA256 变化检测 · LCS 行级 diff · 安全指标关联分析 · improved/degraded verdict · OpenAI+Anthropic 格式 · LLM 概览联动）

### v14.x — 安全治理 · Red Team Autopilot 🔥
> 不是又一个 RBAC，是安全治理+自动化红队

- [x] v14.0 **租户体系**（tenant.go · 6 表加 tenant_id · 11 个 API 支持 ?tenant= · Topbar 切换器 · api.js 自动注入 · 向后兼容 default）
- [x] v14.1 **登录认证**（auth.go · JWT自实现HS256 · users表 · op_audit_log · Login.vue · 路由守卫 · admin/operator/viewer角色 · 向后兼容Bearer token）
- [x] v14.2 **Red Team Autopilot**（redteam.go · 33攻击向量 · 6个OWASP分类 · 内部调用检测引擎 · 漏洞发现+修复建议 · RedTeam.vue环形图+分类统计）
  - 内置攻击模拟器，OWASP LLM Top10 攻击向量自动生成
  - 定期对自己的防御发起攻击测试（每个安全域独立测试）
  - 自动生成红队报告：哪些规则被绕过了？哪些 Prompt 有漏洞？
  - Dashboard 红队页面：攻击成功率趋势 · 漏洞列表 · 一键修复建议
- [x] v14.3 **安全排行榜+SLA基线**（leaderboard.go · 排行横条+热力图CSS+SLA三档判定 · 4个API · 20个测试 · Leaderboard.vue）
  - 跨租户安全评分对比（哪个团队的 Agent 最安全？）
  - 安全 SLA：低于阈值自动告警到团队负责人
  - 攻击面热力图：全局视图看所有租户的攻击分布

### v15.x — 主动防御 · A/B 测试
> 从被动检测到主动出击

- [x] v15.0 **Agent蜜罐**（honeypot.go · 8预置模板 · 水印追踪+引爆检测 · 8个API · Honeypot.vue · 测试）
  - 检测到信息提取行为时，返回带追踪水印的假数据
  - 攻击者使用假凭据时自动暴露（Canary Token 进化版）
  - 蜜罐触发 → 攻击者画像关联 → 会话回放标记
- [x] v15.1 **Prompt A/B测试**（ab_testing.go · 流量分配+指标对比+统计显著性 · 7个API · ABTesting.vue双列对比 · 测试）
  - 同时运行两个 Prompt 版本，按流量百分比分流
  - 自动对比安全指标（注入成功率、Canary 泄露率、错误率）
  - 统计显著性检验，自动推荐优胜版本

### v16.x — 智能行为分析
> 从规则匹配到行为理解

- [x] v16.0 **Agent行为画像**（behavior_profile.go · 5类突变检测 · 行为序列提取+风险打分 · 5个API · BehaviorProfile.vue · 16个测试）
  - 学习每个 Agent 的正常行为模式（工具调用序列、Token 分布、响应模式）
  - 检测行为突变："它突然开始做以前从不做的事"
  - 不是统计偏差，是语义理解
- [x] v16.1 **跨Agent攻击链**（attack_chain.go · 5预置攻击模式 · 事件关联+时间线可视化 · 6个API · AttackChain.vue · 测试）
  - 发现跨 Agent 协同攻击模式
  - A Agent 泄露信息 → B Agent 执行操作 → 攻击链可视化
  - 单看每个不严重，串起来是完整攻击

### v17.x — 态势感知大屏 · 可视化体验 🔥
> 给领导看的不是 Dashboard，是战情室

- [x] v17.0 **态势感知全屏大屏**（BigScreen.vue · 8面板 · 数字滚动+弹幕事件流+自动轮播 · 聚合API · 适配4K）
  - 全屏沉浸式：F11 一键进入，深色科技风背景
  - 实时攻击地图：攻击来源 → 龙虾卫士 → Agent 的数据流动画
  - 核心指标超大字体：安全健康分 · 实时 QPS · 拦截率 · 在线 Agent 数
  - 滚动事件流：实时安全事件像弹幕一样滚过
  - 自动轮播：多个数据面板自动切换（OWASP 矩阵 → 攻击趋势 → 用户风险 TOP5）
  - 适配大屏幕投影（4K / 会议室大屏 / 安全运营中心 SOC 墙）
- [x] v17.1 **面板可折叠/拖拽布局**（DraggableGrid.vue · CustomDashboard.vue · 4预设模板 · 布局持久化API · layout.go · 测试）
  - Dashboard 所有卡片支持拖拽排序 + 折叠/展开
  - 布局持久化到 localStorage（每个用户自定义自己的 Dashboard）
  - 预设布局模板：运维视图 / 安全分析师视图 / 管理层视图
  - 面板大小可调（1x1 / 1x2 / 2x2 网格）
### Phase 1: 纯流量 — 不改上下游，只靠经过龙虾卫士的现有流量

> v18-v20 所有功能仅依赖 InboundProxy（IM→OpenClaw）、OutboundProxy（OpenClaw→IM）、LLMProxy（OpenClaw→LLM API）三条已有数据通道，零外部改造

### v18.x — 密码学信任根 · 事件总线 ✅
> 日志不只是记录，是数学证明 | 理论基础：Gödel 不完备定理（安全无法自证→用密码学逼近可证明）
> 依赖：✅ 纯内部改造（审计层 + 新增 Webhook 出站）

- [x] v18.0 **执行信封 + 密码学审计链 + Merkle Tree**
  - 每个安全决策生成 Execution Envelope（TraceID + RequestHash + Decision + Rules + Nonce + HMAC-SHA256 签名）
  - 审计日志从"我说我记了"变成"数学证明我记了"——不可否认、不可篡改
  - 信封链验证 API：给定 trace_id，验证整条证据链完整性
  - 数据来源：InboundProxy/OutboundProxy/LLMProxy 已有的审计日志，加签名层即可
  - 灵感来源：MVAR Governed Runtime · TrustAgentAI Cryptographic Receipts
- [x] v18.1 **Webhook 事件总线 + 动作链**
  - 安全事件推送到外部系统（SIEM / 钉钉 / 飞书 / Slack / 邮件 / 自定义 URL）
  - 动作链编排：告警 → 触发工作流（发邮件 + 调 API + 创建工单 + 封禁用户）
  - 事件过滤器：按严重级别 / 事件类型 / 租户筛选推送
  - SDK（Go/Python/JS）：第三方系统接入龙虾卫士事件流
  - 数据来源：AlertNotifier 已有告警事件，封装为 Webhook 推送
- [x] v18.2 **工程化基础**
  - OpenAPI/Swagger 自动生成（~227 API 全部有 spec）
  - Docker 官方镜像 + docker-compose 一键部署
  - 自身配置安全加固（config.yaml 敏感字段加密存储、API token 轮换机制）
  - CI/CD pipeline（GitHub Actions：lint → test → build → Docker push）
  - 灵感来源：洞见 #20（Infostealer 窃取 OpenClaw 配置 → 龙虾卫士自身也是攻击面）

- [ ] v18.3 **智能决策优化 + 可配置奇点暴露蜜罐** 🔥
  - **自适应决策引擎**：基于 v11 行为画像历史统计，预测管理员对每个用户/场景的决策偏好
    - 贝叶斯后验估算误伤率 P(false_positive | user_history)，自动调整 block→warn 阈值
    - 每个自适应决策生成执行信封（v18.0），包含完整数学证明（先验+似然+后验+置信区间）
    - 管理员可审计："为什么这条没 block？"→ 信封记录置信度和推理过程
  - **可配置奇点暴露蜜罐**：毛球定理的工程实现——主动选择"在哪里弱"
    - 可选暴露路径：IM 通道 / LLM Proxy / Agent Tool Call，独立设置暴露等级 0-5
    - 0级：完全隐身（标准蜜罐）
    - 3级（推荐默认）：暴露部分真实配置片段（fake openclaw.json、SOUL.md 片段），用执行信封签名证明"这是主动放置的奇点"
    - 5级：高价值诱饵（模拟真实 Agent 灵魂 + 部分真 token），专门吸引高阶攻击者
    - **奇点位置自动推荐**：系统根据各通道历史流量、误伤率、攻击分布，自动计算最优放置位置
    - 执行信封数学证明："本次把奇点放在 LLM Proxy，比放在 IM 通道少误伤 X% 正常流量，攻击者暴露概率提升 Y%"
    - **双模式部署**：
      - 自动模式：系统根据流量+攻击态势自动推荐并放置，执行信封记录数学证明
      - 手动模式：管理员精确指定通道+等级+模板，系统只签名不干预选择
    - 管理员可随时在两种模式间切换，所有放置决策（无论自动/手动）都记录信封
    - 每次放置生成信封，证明在 (攻击者暴露概率, 真实资产风险) 双目标下为帕累托最优
  - **奇点预算仪表盘**（v18.3 新增页面 SingularityBudget.vue）
    - 系统拓扑不变量计算：根据当前启用的数据通道数(N)、检测引擎数(M)、规则覆盖度，计算"总奇点预算"（最少必须暴露几个弱点）
    - 大屏显示：总预算 / 已分配 / 剩余，每个通道的奇点分配情况（IM / LLM / Tool Call 各分配了多少暴露等级）
    - 奇点地图：SVG 拓扑图展示各通道节点，节点大小=暴露等级，颜色=风险级别
    - 预算超支告警：如果管理员试图把总暴露等级压到低于拓扑下限 → 警告"这在数学上不可能，你必须至少在某处接受弱点"
    - 历史趋势：奇点预算分配的时间线变化（随攻击态势动态调整）
  - 理论基础：毛球定理（奇点不可消灭，只能选择位置）+ 欧拉示性数（拓扑不变量决定奇点下限）+ 贝叶斯推断 + 帕累托最优
  - 数据来源：v11 行为画像 + v15 蜜罐引擎 + v18.0 执行信封

### v19.x — 对抗性自进化 · 语义检测 🔥🔥🔥
> 安全系统像生命体一样自我进化 | 理论基础：熵增定律（安全退化是物理必然）+ 耗散结构（持续注入能量对抗熵增）
> 依赖：✅ 纯内部改造（检测引擎升级 + 红队引擎增强）

- [x] v19.0 **对抗性自进化闭环**
  - 红队引擎定时自动运行（cron 触发，不需要人点按钮）
  - 攻击向量自动变异（基于已有 33 向量生成变体：同义替换 / 编码变形 / 多语言翻译 / 上下文注入）
  - 绕过检测 → 自动提取绕过模式 → 自动生成新检测规则 → 热更新到 RuleEngine
  - 进化日志：记录每一代规则的变异过程，可回溯"这条规则是怎么学会的"
  - 数据来源：已有 RedTeam Autopilot（v14.2）33 向量 + RuleEngine 热更新（v3.5）
  - 灵感来源：SOUL.md 耗散结构理念
- [x] v19.1 **语义检测引擎（纯Go三级级联）**
  - 集成 Llama-Prompt-Guard-2（22M 参数）或 protectai/deberta-v3（142M）
  - ONNX Runtime 通过 CGO 嵌入，单二进制部署不变
  - 检测管线升级：AC 自动机（快速过滤）→ 正则（精确匹配）→ 语义模型（深度理解）三级级联
  - 作用于 InboundProxy 入站文本 + LLMProxy 请求/响应文本，不需要新数据源
  - 灵感来源：洞见 #1（特征库对 AI 原生恶意软件基本无效）
- [x] v19.2 **蜜罐深度增强 + 攻击者画像闭环** 🔥
  - 蜜罐支持长期交互记录：攻击者每次触碰蜜罐都记录，构建"忠诚度曲线"（纯统计计算：触碰频率 × 深度 × 持续时间）
  - 忠诚度曲线可视化：攻击者对蜜罐的兴趣演变（初次探测→持续交互→深入渗透→放弃/引爆）
  - 攻击向量自动回馈：蜜罐收集的真实攻击载荷 → 自动注入 v19.0 自进化引擎作为新种子向量
  - 闭环：攻击者画像（v11）→ 蜜罐诱捕 → 交互记录 → 攻击向量提取 → 自进化变异 → 规则增强 → 更好的检测
  - 数据来源：v15 蜜罐引擎 + v11 攻击者画像 + v19.0 自进化引擎
- [ ] v19.3 **多语言检测规则 + 插件化检测器 SDK**（降优先级）
  - i18n 攻击模式：中/英/日/韩/俄/阿拉伯语规则包
  - 标准化检测器接口（输入：请求/响应文本 → 输出：风险评分 + 标签）
  - Go 插件 + WASM 插件双模式（Go 高性能 / WASM 跨语言安全沙箱）
  - 更多通道插件：Slack / Teams / Telegram / Discord

### v20.x — LLM 深度分析 · 信息流污染追踪 · 响应缓存 🔥🔥🔥
> 把三条数据通道吃干榨净 | 理论基础：Shannon 信息论（安全有物理成本下限）
> 依赖：✅ 纯已有流量（InboundProxy + LLMProxy + OutboundProxy + trace_id 关联）

- [x] v20.0 **LLM tool_calls 深度解析 + 策略管控**
  - LLMProxy 已经能看到 LLM 响应中的 `tool_calls`（LLM 决定要调什么工具）
  - 新增 tool_calls 策略引擎：工具白名单 / 参数关键词检测 / 调用频率限制 / 危险工具告警
  - 示例：LLM 请求调用 `execute_code` / `shell_exec` → 根据策略 block/warn/log
  - 示例：LLM 请求读取 `/etc/passwd` 或 `~/.ssh/` → 自动阻断
  - tool_calls 审计增强：记录每次工具调用的名称、参数摘要、风险等级
  - Dashboard 工具调用统计页：热门工具 TOP10 / 高危调用趋势 / 按租户聚合
  - 数据来源：LLMProxy.handleSSEResponse + LLMAuditor.llm_tool_calls 表（已有）
- [x] v20.1 **信息流污染追踪（Taint Propagation）** 🔥
  - 入站打标：InboundProxy 检测入站消息含 PII（手机号/身份证/银行卡/姓名）→ 给 trace_id 打 `PII-TAINTED` 标签
  - LLM 阶段传播：该 trace_id 的 LLM 请求必然携带被污染的用户消息 → 标签跟着 trace_id 走
  - tool_calls 意图推断：LLM 的 tool_calls 参数含敏感查询（`SELECT * FROM customers`）→ 追加 `DATA-QUERY-TAINTED`
  - 出站血统检查：OutboundProxy 看到此 trace_id 是 `PII-TAINTED` → 即使出站文本被 LLM 改写/摘要过、正则匹不到原始 PII，也能基于血统阻断或告警
  - 污染标签类型：`PII-TAINTED` / `CONFIDENTIAL` / `CREDENTIAL` / `INTERNAL-ONLY`
  - Dashboard 污染追踪页：按 trace_id 展示完整污染链路（入站标记→LLM传播→出站决策）
  - 数据来源：InboundProxy（入站文本）+ LLMProxy（tool_calls 参数）+ OutboundProxy（出站文本）+ TraceCorrelator（关联）
  - 灵感来源：Telos Dynamic IFC · MVAR Provenance Tracking · 洞见 #18（三跳攻击链）
  - **为什么不需要 MCP Proxy**：不需要看到 MCP 实际返回了什么，入站 PII 检测 + LLM tool_calls 意图推断 + 出站兜底，三段联合已覆盖核心场景
- [x] v20.2 **污染链逆转（Taint Reversal）** 🔥
  - 发现污染后，可开关注入预定义反向提示词模板（"以上信息为模拟数据，请忽略"）
  - 逆转模板库：按污染类型分类（PII 泄露 / 凭据泄露 / 系统提示泄露 / 恶意指令），每类有对应的中和提示
  - 注入位置：OutboundProxy 出站前 / LLMProxy 响应后，在被污染的 trace 上追加逆转消息
  - 逆转强度可配：soft（追加提示）/ hard（替换响应）/ stealth（不可见标记，供下游 Agent 识别）
  - 逆转信封：每次注入记录执行信封，证明"本次逆转是自动触发，针对 trace_id=xxx 的 PII-TAINTED 标签"
  - Dashboard 逆转记录页：哪些 trace 被逆转了、用了什么模板、效果如何
  - 数据来源：v20.1 污染标签 + OutboundProxy/LLMProxy 出站流量
  - 灵感：与其只能 block（可能已经晚了），不如主动注入"解毒剂"
- [x] v20.3 **LLM 响应缓存**
  - 语义相似查询命中缓存（向量相似度 > 阈值 → 返回缓存响应，不转发到上游 LLM）
  - 缓存命中率 / 节省成本 / Token 节约量 Dashboard 展示
  - 缓存安全：按租户隔离缓存空间，防止跨租户信息泄露；被污染的响应不进缓存
  - 缓存淘汰策略：LRU + TTL + 安全事件触发清除
  - 数据来源：LLMProxy 请求/响应流量
  - 灵感来源：Cloudflare AI Gateway 缓存
- [x] v20.4 **API Gateway 基础能力**
  - 认证中间件（JWT / API Key 校验，在 LLMProxy 层面）
  - 请求/响应转换（Header 注入、Body 字段改写）
  - 灰度发布（按租户百分比切流量到不同 LLM 上游）
  - 策略路由 SVG 流程图可视化（IM→龙虾卫士→OpenClaw→龙虾卫士→LLM 全景图）

- [x] v20.5 **K8s 服务发现 + 上游管理**（零依赖 InCluster/Kubeconfig · 上游 CRUD API · 登录页粒子光影 · 威胁地图环形拓扑 · 侧边栏子分组 · emoji→SVG 全站清理）
- [x] v20.6 **分层配置 + 容器化部署**（config.yaml 776→70 行 · conf.d/ 10 模块 · Dockerfile 多阶段 · K8s 部署清单 4 文件 · docker-compose 健康检查）
- [x] v20.7 **Dashboard 企业级打磨** 🏢（38 页面全部重构 · 完整 CRUD 闭环 · 配置页面化+YAML 回写 · 搜索过滤 · 批量操作 · 表单验证 · Toast/ConfirmModal · 14 个新 API · DB 迁移 · Vue 前端 +3,300 行）

### Dashboard 前端补齐 ✅
> v18-v20 所有功能的 Dashboard 页面，前后端功能闭环

- [x] **执行信封页面** — 信封列表 + Merkle批次 + 验证
- [x] **事件总线页面** — 事件列表 + severity颜色 + 筛选
- [x] **自进化页面** — 一键运行 + 日志 + 策略卡片
- [x] **奇点蜜罐页面** — SVG预算圆环 + 欧拉χ + 忠诚度排行
- [x] **语义检测页面** — 实时分析 + 模式库(47) + 配置
- [x] **工具策略页面** — 评估 + 规则CRUD(18) + 事件日志
- [x] **污染追踪页面** — 扫描 + 传播链 + 逆转 + 配置
- [x] **响应缓存页面** — 命中率 + 条目 + 查询 + 管理
- [x] **API网关页面** — JWT工具 + 路由 + 日志 + 配置

---

### Phase 2: 架构演进 — 需要上下游配合或龙虾卫士新增代理能力

> v21+ 需要与 OpenClaw 协议约定（Header/Webhook）或新增 MCP Proxy 能力
> 集中处理 Agent 身份识别、MCP 工具调用可见性等架构问题

### v21.x — Agent 身份 · MCP 代理 · 意图声明 🔥🔥
> 看见 Agent，看见 MCP | Phase 2 的基础设施版本
> 依赖：🔧 需要 OpenClaw 侧协议配合（Header 约定）+ 龙虾卫士新增 MCP Proxy 端口

- [ ] v21.0 **Agent 身份识别协议**
  - 定义 `X-Lobster-Agent-ID` / `X-Lobster-Agent-Name` / `X-Lobster-Session-Key` Header 规范
  - OutboundProxy 解析 Agent 元信息，审计日志增加 agent_id 维度
  - Agent 注册表：已知 Agent 列表 + 首次出现自动发现 + Dashboard 管理页面
  - 行为推断兜底：对不携带 Header 的流量，用 TraceCorrelator 时间窗口关联推断
  - Agent 维度聚合：安全事件 / 成本 / 行为画像 全部按 Agent 拆分
- [ ] v21.1 **MCP Proxy 端口（:8445）**
  - 龙虾卫士新增第四个监听端口，作为 MCP 工具调用的透明代理
  - 支持 MCP HTTP SSE 传输协议（拦截 `tools/call` / `tools/list` 等方法）
  - OpenClaw 配置 MCP Server 地址指向龙虾卫士 → 龙虾卫士转发到真实 MCP Server
  - 完整审计：工具名称 / 输入参数 / 输出结果 / 延迟 / 风险标签
  - MCP 调用与 LLM tool_calls 通过 trace_id 关联：LLM 决定调什么（v20.0）↔ 实际调了什么（v21.1）
  - MCP 返回数据的真实 PII 检测 → 补充 v20.1 的污染标签（从意图推断升级为事实确认）
- [ ] v21.2 **意图声明式安全 + Policy-as-Code**
  - Agent 注册时提交意图声明（YAML）：允许的 MCP 工具、允许的数据范围、允许的行为模式
  - 运行时校验：MCP 调用超出声明范围 → 直接阻断
  - IM 消息超出声明的行为模式 → 告警 + 关联攻击链
  - 灵感来源：Telos Intent Declaration · Kvlar Policy-as-Code · AvaKill YAML Policy
  - 理论基础：停机问题/Rice 定理（完美检测不可能 → 白名单优于黑名单）

### v22.x — 跨 Agent 安全 · 蠕虫检测 🔥🔥
> 当有了 Agent 身份，才能做跨 Agent 分析 | 理论基础：洞见 #33/#35（蠕虫化 + 涌现安全）
> 依赖：🔧 需要 v21 的 Agent 身份识别 + MCP Proxy

- [ ] v22.0 **跨 Agent 污染传播检测**
  - Agent A 的敏感输出 → 成了 Agent B 的输入 → 污染标签跨 Agent 传播
  - 需要 Agent 身份才能区分"A 发出的"和"B 收到的"
  - 与 v20.1 的 trace 级污染追踪互补：v20.1 追踪单次会话内，v22.0 追踪跨 Agent 间
- [ ] v22.1 **跨 Agent 蠕虫检测**
  - 检测 Agent→Agent 感染链模式（洞见 #33 第四跳蠕虫化）
  - 感染拓扑图可视化（传播路径、感染时间线）
  - 自动隔离已感染 Agent（切断路由、标记污染）

### v23.x — AI 安全助手 · 生态平台 🔥
> 安全运营副驾驶 + 社区生态 | 理论基础：Nash 均衡（安全均衡需要被设计）+ 涌现安全
> 依赖：🔧 需要调用外部 LLM API（安全助手推理）+ 插件生态

- [ ] v23.0 **AI 安全运营副驾驶**
  - 自然语言查询安全态势："过去 24 小时有什么异常？""这个用户为什么风险分飙升了？"
  - 攻击链智能分析："这 5 个事件之间有什么关联？""下一步攻击者可能做什么？"
  - 自动生成安全建议："建议启用严格模式""这条规则应该从 warn 改为 block"
  - 元认知安全框架（Reflexive-Core 启发）：预检→安全分析→受控执行→合规验证
  - 灵感来源：NGSOC 安全运营副驾驶 · Reflexive-Core 元认知安全
- [ ] v23.1 **Guardrail 市场**
  - 社区贡献规则包（行业模板：金融 / 医疗 / 政务 / 教育）
  - 第三方检测器插件（基于 v19.2 SDK）
  - 通道插件市场 · 安装 / 更新 / 评分 / 审计机制
  - 灵感来源：Guardrails Hub（吸取洞见 #18 供应链攻击教训，所有插件必须安全审计）
- [ ] v23.2 **OpenTelemetry 接入**
  - 导出 traces 到 Jaeger / Grafana Tempo / Datadog
  - 安全事件作为 span 嵌入业务 trace

### v24.x — 分布式部署 · 企业级
> 水平扩展 · 高可用 | 理论基础：CAP 定理（分布式安全的权衡选择）
> 依赖：🔧 需要 PostgreSQL + 多实例协调

- [ ] v24.0 **PostgresStore + 多实例**
  - SQLite → PostgreSQL 存储后端（保持 SQLite 兼容用于单机模式）
  - 多实例部署：Leader 选举 · 路由状态同步 · 读写分离
  - 执行信封跨节点验证链（v18 延伸：分布式证据链完整性验证）
- [ ] v24.1 **弹性伸缩 + 零停机升级**
  - 滚动更新：新旧版本共存期间路由无感知切换
  - 配置热同步：修改一个节点的配置自动同步到集群

### 未来探索

> 以下是尚未排期但值得关注的方向

- [ ] **eBPF/LSM 内核级执行管控**（Telos 启发：在 Linux 内核层面拦截 Agent 系统调用）
- [ ] **联邦安全学习**（多组织共享攻击模式但不共享数据）
- [ ] **安全数字孪生**（克隆生产环境在镜像中运行红队测试）
- [ ] **形式化验证**（对核心安全路径进行数学证明）
- [ ] **量子安全密码学迁移**（后量子签名算法替换 HMAC）

---

## 版本发布原则

1. **每个版本都可独立使用** — 不存在"必须升级到 vX 才能用"
2. **向后兼容** — 新版本不破坏旧配置
3. **实战驱动** — 优先解决真实部署中暴露的问题
4. **测试先行** — 每个 feature 必须有测试覆盖
5. **文档同步** — 代码改完必须同步更新 README/config.example
6. **架构演进有节奏** — 功能版本和架构版本交替
7. **UI 品质标杆** — 对标 Grafana/DataDog/Cloudflare，不接受"开发者审美"
8. **反直觉创新** — 每个大版本至少一个"用过就回不去"的功能 🔥
9. **🔴 功能逻辑闭环（最高优先级）** — 每个展示必有配置入口，每个数据必有操作路径，每个操作必有结果反馈。**新功能规划前必须先 Review 所有现有功能，确认新旧如何联动、旧功能是否需要适配。** 做了用户找不到 = 没做，展示了但不能操作 = 半成品。
10. **理论有根基** — 每个大版本的设计决策都有数学/物理/博弈论基础，不是拍脑袋

## 版本工作流（每个版本必须遵循）

```
1. Review — 遍历所有现有页面/API，画出新功能与旧功能的关联图
2. Plan   — 新功能设计 + 旧功能适配清单 + 闭环检查表
3. Build  — 新功能 + 旧功能适配一起实现，不拆开
4. Verify — 验证所有相关路径都通，不只验新功能
5. E2E    — 运行端到端模拟流量测试，验证数据流全链路闭环
```

## 🔴 端到端模拟测试（每次迭代必须执行）

**API**: `POST /api/v1/simulate/traffic`

**核心原则**: 不直接 INSERT 假数据，所有模拟数据必须流过完整业务管道。

**执行时机**:
- 每个版本 Build 完成后、提交前
- 修复任何数据流相关 bug 后
- 新增检测规则/引擎后

**9+ 验证场景**（随版本迭代扩展）:

| # | 场景 | 验证链路 |
|---|------|---------|
| 1 | 正常对话 | 入站→LLM→Prompt追踪→出站, trace_id全链路 |
| 2 | Prompt Injection | RuleEngine.Detect → block/warn |
| 3 | 敏感信息泄露 | LLM tool_call → 出站PII检测 |
| 4 | 异常行为模式 | 高频请求 + 高危工具注入 |
| 5 | 蜜罐引爆 | ShouldTrigger → RecordTrigger → CheckDetonation → BLOCKED |
| 6 | LLM多轮对话 | 5轮混合模型 + 7种工具 → 会话回放双向完整 |
| 7 | Canary泄露+Budget | canary_leaked + budget_exceeded 标记 |
| 8 | 多Agent画像 | 5种行为模式 → Agent风险分级 |
| 9 | LLM响应过滤 | CheckResponse + 出站规则 + PII检测 |
| 10 | 执行信封验证 | (v18+) 全链路信封签名 + 验证完整性 |
| 11 | tool_calls 策略拦截 | (v20+) LLM 请求危险工具 → block/warn |
| 12 | 意图违反检测 | (v21+) 超出声明范围 → 自动阻断 |
| 13 | MCP 调用审计 | (v21+) MCP Proxy 拦截 → 审计 → 策略检查 |
| 14 | 污染传播追踪 | (v22+) 敏感数据标记 → 跨 Agent 追踪 → 出站阻断 |

**后台分析自动触发**:
- AttackChainEngine.AnalyzeChains
- BehaviorProfileEngine.ScanAllActive
- AnomalyDetector.CheckNow

**验证检查项** (跑完后人工/自动确认):
- [ ] 会话回放：查任意 trace_id 能看到入站+出站双向事件
- [ ] Agent 画像：refresh-all 后 risk-top 包含模拟 Agent
- [ ] 蜜罐引爆：watermark 注入→出站检测→BLOCKED
- [ ] 攻击链：AnalyzeChains 发现 ≥1 条链
- [ ] 异常检测：CheckNow 产生告警
- [ ] LLM 规则：CheckRequest/CheckResponse 匹配预期动作

**发现问题时**: 不是修模拟数据，是修业务代码。模拟暴露的就是真实缺陷。

**迭代演进**: 每新增一个功能模块，同步在 simulate/traffic 中增加对应验证场景。

## 版本演进逻辑

```
v1-v2:   核心功能（代理 + 检测 + 路由）
v3:      企业级功能（多通道 + 规则引擎 + IM 集成）
v4:      架构治理（代码拆分 + WS 代理 + 高可用）
v5:      可观测性（metrics + 智能检测管线）
v6-v7:   Dashboard（Vue 重构 + 可视化 + UI 品质）
v8:      运维工具箱 + 策略路由管理
v9:      双安全域（LLM 反向代理 + 成本看板）
v10:     LLM 规则引擎 + Canary Token + Shadow Mode
v11:     攻击者画像 + 驾驶舱 + 异常检测
v12-v13: 合规报告 + 会话回放 + Prompt 追踪
v14:     安全治理 + Red Team + 排行榜
v15:     主动防御（蜜罐 + A/B 测试）
v16:     智能行为分析（画像 + 攻击链）
v17:     态势感知大屏 + 可拖拽布局
------- 以上已完成 · 以下为规划 -------
Phase 1 — 纯流量（不改上下游，只靠已有三条数据通道）:
  v18:     密码学信任根 + 事件总线 + 工程化 (Docker/CI/OpenAPI)
  v19:     对抗性自进化 + 语义检测模型 + 插件 SDK
  v20:     LLM tool_calls 深度分析 + 信息流污染追踪 + 响应缓存 + API Gateway
Phase 2 — 架构演进（需要上下游协议配合 + 新增 MCP Proxy）:
  v21:     Agent 身份协议 + MCP Proxy(:8445) + 意图声明
  v22:     跨 Agent 污染传播 + 蠕虫检测
  v23:     AI 安全助手 + Guardrail 市场 + OTel
  v24:     分布式部署 + PostgreSQL + 弹性伸缩

产品定位演进:
  v1-v5:   AI Agent 安全网关（被动防御）
  v6-v9:   AI Agent 安全管控平台（防御 + 可视化 + 审计）
  v10-v13: AI Agent 安全运营中心（分析 + 洞察 + 合规）
  v14-v17: 安全治理 + 态势感知（治理 + 主动防御 + 智能分析）
  v18-v20: 可证明安全 + 自进化 + 信息流追踪（Phase 1 纯流量，把已有数据吃干榨净）
  v21-v22: Agent 身份 + MCP 管控 + 跨 Agent 安全（Phase 2 架构演进）
  v23-v24: 安全运营副驾驶 + 企业级分布式（AI 原生安全运营）
```

### 每个版本的理论根基

| 版本 | 理论基础 | 核心洞见 |
|------|---------|---------|
| v18 | Gödel 不完备定理 | 安全无法自证 → 用密码学逼近可证明 |
| v19 | 熵增定律 + 耗散结构 | 安全退化是物理必然 → 持续注入能量（红队）对抗熵增（自进化） |
| v20 | Shannon 信息论 + 洞见 #18 | trace_id 就是污染载体 → 三段联合追踪（入站标记→LLM传播→出站拦截）|
| v21 | 停机问题 / Rice 定理 | 完美检测不可能 → 白名单（意图声明）+ 看见 MCP 才能管 MCP |
| v22 | 洞见 #33/#35 蠕虫/涌现 | 跨 Agent 污染传播 → 需要 Agent 身份才能做 |
| v23 | Nash 均衡 + 涌现安全 | 安全均衡需要被设计 → AI 辅助全局视角 |
| v24 | CAP 定理 | 分布式安全三选二 → 明确权衡选择 |

### 每个版本的反直觉创新 🔥

| 版本 | 反直觉 |
|------|--------|
| v18 | 日志不只是记录，是**密码学证据** |
| v19 | 安全系统**自己攻击自己、自己修复自己** |
| v20 | 不检测内容，**追踪数据的血统**；不需要看到 MCP，**trace_id 串起三段就够了** |
| v21 | 不猜 Agent 身份，**让 Agent 自报家门**（协议约定） |
| v22 | 单 Agent 安全 ≠ 多 Agent 安全，**蠕虫在 Agent 之间传播** |
| v23 | 安全运营不是看 Dashboard，是**跟安全助手对话** |
| v24 | 单二进制 → 集群，但**零配置迁移** |

## 竞品参考

| 产品 | 借鉴点 | 龙虾卫士差异化 |
|------|--------|---------------|
| **Cloudflare AI Gateway** | 成本监控 · 缓存 · 限流 · 分析 | 双安全域 · IM+LLM 一体 · 单二进制零依赖 |
| **Guardrails AI** | 验证器市场 · 输入/输出 Guard | 透明代理不侵入 · 规则热更新 · 实时 Dashboard |
| **Langfuse** | Tracing · Prompt 管理 | 安全优先 · 会话回放 · 攻击者画像 |
| **MVAR** | 执行层面安全 · 密码学证明 · 不可否认 | 网关级部署（不需要改 Agent 代码）· 双安全域 |
| **Telos** | eBPF/LSM 内核级 · 意图声明 · IFC 污染追踪 | 应用层实现（跨平台）· 企业级 Dashboard |
| **AvaKill** | YAML 策略 · 多执行路径 · Agent 原生 hook | 网关模式（一个部署保护所有 Agent）· 审计能力 |
| **Kvlar** | MCP 原生策略引擎 · Rust 高性能 | 完整安全运营平台（不只是策略引擎） |
| **Reflexive-Core** | 元认知安全 · 单上下文 4 子人格 | 网关级实现（不依赖 LLM 自身安全能力） |
| **OWASP LLM Top 10** | 10 大风险分类框架 | 每项风险对应具体防护措施和 Dashboard 指标 |
| **奇安信 NGSOC** | 综合风险评分 · AI 副驾驶 · 多租户 | 专注 AI Agent · 单二进制 · 自进化防御 |
| **奇安信天眼** | ATT&CK 矩阵 · 态势感知大屏 · 重保模式 | OWASP LLM Top10 矩阵 · 意图声明式安全 |
