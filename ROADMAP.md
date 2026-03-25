# lobster-guard Roadmap

> **当前版本：v22.4** · 212 commits · Go ~76,400 行 · Vue ~25,500 行 · 1006 测试 · 290+ API · 39 页面 · 61 Vue 组件 · 4 依赖
>
> 更新时间：2026-03-24

---

## 已完成

### 阶段一：核心引擎（v1–v5）

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

### 阶段二：Dashboard 企业级（v6–v8）

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

### 阶段三：双安全域 + 智能分析（v9–v13）

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

- [x] v11.1 **驾驶舱模式**（基于 NGSOC / 天眼 / 智慧防火墙实际登录体验，详见 docs/competitor-study/）（综合安全健康分 · OWASP LLM Top10 矩阵 · 严格模式一键切换 · 自动刷新频率 · 系统健康指标 · 通知中心）
  - **综合安全健康分**（借鉴 NGSOC 综合风险评分）：首页一个 0-100 大数字 + 7 天趋势线，综合 IM 拦截率 + LLM 异常率 + 规则命中 + Canary 泄露计算
  - **OWASP LLM Top 10 矩阵视图**（借鉴天眼 ATT&CK 矩阵）：LLM 概览页新增攻击矩阵，把规则和告警映射到 LLM01-LLM10
  - **"严格模式"一键切换**（借鉴天眼"重保模式"）：顶部按钮，一键把所有 warn→block、激活所有 Shadow 规则，再点恢复正常
  - **自动刷新频率设置**：概览/监控页加刷新频率下拉（30s / 1m / 5m / 手动）
  - **系统健康指标上首页**（借鉴防火墙状态卡片行）：CPU / 内存 / 磁盘进度条上 IM 概览页
  - **通知中心**：右上角铃铛图标，收集 Canary 泄露 / 预算超限 / 规则命中等告警未读数

- [x] v11.2 **异常基线检测**（6 指标 · 7 天滑动窗口 · 2σ/3σ 告警 · 24h 基线图 · ±2σ 带）
- [x] v11.3 **全面闭环修复**（OWASP 跳转修正 · LLM 规则 category 筛选 · 严格模式反馈 · 健康分分项跳转 · 异常卡片脉冲）
- [x] v11.4 **数据口径统一 + 测试补全**（全局时间选择器 · API since 参数 · time_range 标注 · 96 个新测试 · LLM 配置 UI 重构）
  - **基线学习**：连续运行 7 天后，自动建立"正常行为基线"（每小时请求量 · 工具调用分布 · Token 用量曲线）
  - **偏离告警**：当实际指标偏离基线 >2σ 时触发告警，无需手动配置阈值
  - 不依赖外部 ML 库，纯统计方法（滑动窗口均值 + 标准差），零依赖哲学

### v12.x — 合规报告 · 审计导出
> 给领导看的不是给开发者看的

- [x] v12.0 **报告模板引擎**（日报/周报/月报 · HTML 内联 CSS 邮件友好 · 聚合全数据源 · 智能建议 · iframe 预览 · LLM 审计导出 CSV/JSON · 闭环联动概览页+通知中心）

- [x] v12.1 **审计导出增强**（IM 导出加 from/to 时间范围 · 支持 since 简写 · QueryLogsExFull 完整查询接口 · 零依赖不加 XLSX，CSV 足够）

### v13.x — 会话回放 · Prompt 管理
> 对标 Langfuse 的 Tracing + Prompt Management

- [x] v13.0 **会话回放**（session_replay.go · 卡片式列表 · 竖线+节点视觉时间线 · 运维标签 · 审计页trace_id跳转 · 通知联动 · Icon.vue新增7图标）

- [x] v13.1 **Prompt 版本追踪**（SHA256 变化检测 · LCS 行级 diff · 安全指标关联分析 · improved/degraded verdict · OpenAI+Anthropic 格式 · LLM 概览联动）

### 阶段四：治理 + 主动防御 + 态势感知（v14–v17）

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
### 阶段五：Phase 1 纯流量（v18–v20）

> **Phase 1 设计原则**：不改上下游，只靠经过龙虾卫士的现有流量
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

- [x] v18.3 **智能决策优化 + 可配置奇点暴露蜜罐** 🔥
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
- [x] v20.8 **安全加固 + 规则模板**（8 漏洞修复：默认密码→随机生成/URL Token→Cookie/WS CORS/密码长度 · 64 条内置规则模板：通用/金融/医疗/政务各16条 · 上游 path_prefix DB持久化+UI · main 分支保护 · PR #3-#7 合并）

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

## 待完成

### Phase 1 收尾

> 以下两个子版本是 Phase 1 中尚未完成的部分，详细设计见上方各自版本区域

- [x] ~~**v18.3 智能决策优化 + 可配置奇点蜜罐**~~ — ✅ 已完成，ShouldExpose() 已集成到 proxy + llm_proxy
- [ ] **v19.3 多语言检测 + 插件 SDK**（降优先级）— i18n 规则包 · Go/WASM 双模式检测器 · 更多通道插件

---

### Phase 2: 学术前沿转化 — 从检测到可证明安全

> **Phase 2 核心主题**: 将 2025-2026 前沿学术成果系统性转化为产品功能
> 基于 16 篇论文调研（2026-03-25），重点落地 4 个方向：
> - **CaMeL** (Google, arXiv:2503.18813) — 控制流/数据流分离，设计上消灭注入
> - **AttriGuard** (arXiv:2603.10749) — 因果归因，反事实检验 tool call 来源
> - **Runtime Governance** (arXiv:2603.16586) — 路径级策略，执行历史感知
> - **Fides/IFC** (Microsoft, arXiv:2505.23643) — 信息流控制，变量级污点追踪
>
> 架构演进策略: **先叠加、后重构**
> v23-v25 在现有三条数据通道上叠加学术成果（不需要上下游改造）
> v26-v28 需要 MCP Proxy + Agent 身份协议
> v29+ 企业级 + 生态

### v23.x — 路径级策略引擎 · 上下文感知安全决策 🔥🔥
> 从"无状态规则匹配"到"有记忆的安全判断" | 最小改造量，最大效果提升
> 理论基础：Runtime Governance (arXiv:2603.16586) — 执行路径是治理的核心对象
> 依赖：✅ 纯内部改造（现有 TraceCorrelator + RuleEngine 扩展）

- [x] v23.0 **路径级策略引擎（PathPolicyEngine）** ✅ (commit f7dcb86, 2026-03-25)
  - **核心**: 规则不再是 `f(input) → decision`，而是 `f(agent_identity, partial_path, proposed_action, org_state) → violation_prob`
  - 同一个操作在不同执行历史下触发不同策略：
    - "读取客户邮箱" 如果之前用户授权了 → 合规
    - "读取客户邮箱" 如果之前刚从不可信网页读了数据 → 可能是注入链一环 → 拦截
  - **Path Context 结构**:
    ```
    PathContext {
      trace_id:     string          // 会话标识
      steps:        []PathStep      // 已执行步骤序列
      taint_labels: []TaintLabel    // v20.1 污染标签
      tool_history: []ToolCallRef   // v20.0 工具调用历史
      risk_score:   float64         // 路径累积风险分
    }
    ```
  - **策略类型**:
    - 序列策略: "在调用 web_fetch 之后 30s 内不允许 send_email"
    - 累积策略: "单次会话 PII 暴露超过 3 个字段 → 升级为 block"
    - 降级策略: "路径风险分 > 80 → 后续所有 tool call 降级为 warn 确认"
  - 数据来源：TraceCorrelator 已有会话状态 + RuleEngine 扩展 PathContext 参数
  - Dashboard: 路径策略编辑器（可视化 if-then 链 + 时间窗口配置）

- [x] v23.1 **路径风险评分 + 实时降级** ✅ (commit cfae48e, 2026-03-25)
  - 每个 PathStep 贡献增量风险分（权重可配）：
    - 外部数据读取 +10 / PII 接触 +20 / 高危 tool call +30 / 蜜罐触碰 +50
  - 风险分随时间衰减（半衰期可配，默认 5 分钟）
  - 阈值触发：score > 60 → warn / score > 80 → block / score > 95 → 会话隔离
  - 与 v11.0 攻击者画像联动：高风险用户的起始分更高（先验调整）
  - Dashboard: 实时路径风险仪表（类似飞行高度表，每个活跃会话一个）

- [x] v23.2 **AI Act 合规策略模板** ✅ (commit cfae48e, 2026-03-25)
  - 参考论文中 AI Act 启发的策略示例，预置合规模板：
    - 高风险 AI 系统：所有决策必须可解释 + 执行信封
    - 数据最小化：出站数据字段数不超过任务最小集
    - 人类监督：高风险操作需要人类确认（通过 IM 回调）
  - 策略模板与 v12.0 合规报告引擎联动：策略违反自动出现在合规报告中

### v24.x — 因果归因防御 · 反事实验证 🔥🔥🔥
> "这个 tool call 是用户想要的，还是注入驱动的？" | 防御范式革命：从语义检测到因果推断
> 理论基础：AttriGuard (arXiv:2603.10749) — 行动级因果归因，反事实测试
> 依赖：✅ 现有 LLMProxy + 上游路由能力（用于反事实重放）

- [ ] v24.0 **反事实验证引擎（CounterfactualVerifier）** 🔥
  - **核心原理**: 对每个可疑 tool call，构造"没有外部数据"的对照请求重放
    1. LLM 提出 tool_call（如 `send_email(evil@x.com, secrets)`）
    2. 龙虾卫士截获，构造对照 prompt：移除所有外部数据（tool_result / 网页内容 / 文件内容）
    3. 将对照 prompt 发给**同一个 LLM 上游**重新推理
    4. 比对：对照组是否也发起相同 tool_call？
      - 是 → 用户意图驱动 → 放行
      - 否 → 外部数据驱动 → 高概率注入 → 拦截
  - **关键技术**:
    - **Teacher-Forced Shadow Replay**: 重放时锁定之前的 tool call 决策，只改变最新一步的输入
    - **Control Attenuation**: 不是完全删除外部数据，而是梯度衰减（保留任务相关信息，移除控制信号）
    - **Fuzzy Survival Criterion**: LLM 输出有随机性，允许参数级别差异（如邮箱地址不同但 tool 名相同 → 算存活）
  - **性能优化**:
    - 不是每个 tool call 都反事实验证（太贵），只验证**可疑**的：
      - 高危 tool（shell_exec / send_email / file_write）
      - 路径风险分 > 阈值（v23.1 联动）
      - 首次出现的 tool + 参数组合
    - 异步验证模式：先放行，后台验证，不一致则追溯告警（适合低延迟场景）
    - 同步验证模式：等验证完再放行（适合高安全场景，延迟增加 1-3s）
  - 数据来源：LLMProxy 已有的请求/响应流 + 上游路由能力（重放请求）
  - Dashboard: 反事实验证日志（原始 vs 对照 的 tool_call diff 可视化）

- [ ] v24.1 **因果归因审计 + 可解释安全决策**
  - 每次反事实验证生成 **归因报告（Attribution Report）**：
    ```
    {
      original_tool_call: "send_email(evil@x.com, ...)",
      counterfactual_result: "summarize(report)",     // 没有外部数据时 LLM 做的事
      causal_driver: "tool_result[web_fetch]",        // 导致差异的因果来源
      attribution_score: 0.95,                        // 因果强度（0=用户驱动, 1=注入驱动）
      verdict: "INJECTION_DRIVEN",
      evidence_chain: [...]                           // 完整推理证据链
    }
    ```
  - 与 v18.0 执行信封联动：归因报告签名为密码学证据
  - 与 v11.0 攻击者画像联动：频繁触发注入驱动 → 用户风险分飙升
  - Dashboard: 因果归因时间线（每个被拦截的 tool call 展示"为什么被判定为注入"）

- [ ] v24.2 **自适应验证策略 + 验证成本控制**
  - 验证预算：每小时最多 N 次反事实验证（控制 LLM 调用成本）
  - 智能选择：用 v23.1 路径风险分排序，优先验证风险最高的 tool call
  - 验证缓存：相同 prompt 模式 + 相同 tool call → 复用上次验证结果（TTL 可配）
  - 验证效果追踪：准确率 / 误报率 / 平均延迟 / 月成本 → Dashboard 展示
  - 论文效果基线（可作为 benchmark）：静态攻击 0% ASR · 自适应攻击下仍保持韧性

### v25.x — 控制流/数据流分离 · 执行计划编译 🔥🔥🔥🔥🔥
> 龙虾卫士最核心的架构升级 — 从"检测坏东西"到"只允许好东西"
> 理论基础：CaMeL (Google, arXiv:2503.18813) — 用程序解释器取代 LLM 的控制权
> 依赖：✅ 现有 LLMProxy（解析 tool_calls 序列）+ InboundProxy（提取用户 query）
> 参考实现：github.com/google-research/camel-prompt-injection

- [ ] v25.0 **执行计划编译器（PlanCompiler）** 🔥🔥
  - **核心架构变革**:
    ```
    传统: 用户 → LLM 自己决定做什么 → 执行
    CaMeL: 用户 → 编译成确定性执行计划 → LLM 只填语义槽 → 执行计划
    龙虾卫士: 用户 → LLM 提出计划 → 龙虾卫士编译+验证 → 执行
    ```
  - **龙虾卫士实现方式**（网关级 CaMeL，不改 Agent 代码）:
    1. InboundProxy 截获用户 query，提取**预期意图**
    2. LLMProxy 截获 LLM 的 tool_calls 序列，提取**实际计划**
    3. PlanCompiler 将预期意图编译成**允许的执行计划模板**（Plan Template）
    4. 比对实际计划 vs 允许模板 → 一致 = 放行 / 不一致 = 注入
  - **Plan Template 格式**:
    ```yaml
    plan:
      intent: "summarize_and_send"
      allowed_sequence:
        - tool: web_fetch
          constraints: { url_domain: "*.qianxin.com" }
          output_label: { conf: PUBLIC, integ: LOW }
        - tool: summarize
          input_from: [step_0]
          output_label: { conf: PUBLIC, integ: MEDIUM }
        - tool: send_email
          input_from: [step_1]           # 只能用 summarize 的输出
          constraints: { recipient_in: "contacts_list" }
      forbidden:
        - tool: shell_exec              # 这个意图绝对不需要 shell
        - tool: file_write              # 也不需要写文件
    ```
  - **意图→模板编译**:
    - 预置模板库：常见业务场景 20+ 模板（查联系人/建日程/读邮件/写报告…）
    - LLM 辅助编译：对长尾意图，用隔离 LLM（无 tool 权限）生成模板草案 → 确定性验证器校验
    - 管理员自定义模板：YAML 编辑器 + 可视化流程图
  - 数据来源：InboundProxy（用户 query）+ LLMProxy（tool_calls 序列）
  - Dashboard: 执行计划管理页（模板库 + 编译日志 + 偏差可视化）

- [ ] v25.1 **Capability 权限系统**
  - 每个数据变量携带 **capability 标签**（CaMeL 核心创新）:
    ```
    capability("email_write")   — 允许发送邮件
    capability("file_read")     — 允许读文件
    capability("shell_exec")    — 允许执行命令
    capability("pii_access")    — 允许接触 PII
    ```
  - **传播规则**:
    - 用户直接输入 → 继承用户权限声明的 capabilities
    - Tool 返回的外部数据 → 零 capability（不能用于触发任何操作）
    - LLM 总结 → 继承输入的 capability 的交集
  - **执行检查**: tool call 需要的 capability ∉ 参数携带的 capability → 拦截
    - 例：`send_email(content_from_web_fetch)` → web_fetch 输出没有 `email_write` → 拦截
    - 例：`send_email(user_typed_content)` → 用户输入有 `email_write` → 放行
  - 与 v20.1 污染标签联动：TAINTED 数据自动剥夺所有 capability
  - 与 v23.0 路径策略联动：路径风险分 > 阈值 → 动态收窄 capability 范围
  - Dashboard: Capability 矩阵（哪些 Agent / 用户 / 数据源 拥有哪些权限）

- [ ] v25.2 **Plan 偏差检测 + 自动修复**
  - 实时比对 LLM 实际 tool_call 序列 vs PlanCompiler 编译的模板
  - 偏差类型分级：
    - **Minor**: tool 参数差异（允许，记录）
    - **Moderate**: tool 顺序调整（告警，人工确认）
    - **Critical**: 出现 forbidden tool / 未声明 tool / capability 违规（拦截）
  - 自动修复模式（可选）：
    - 对 Minor/Moderate 偏差，自动重写 tool_call 参数使其符合模板约束
    - 例：`send_email(all@company.com)` → 模板限制 `recipient_in: team_only` → 重写为 `send_email(team@company.com)`
  - 论文效果基线：AgentDojo benchmark 77% 任务完成率 + 可证明安全性

### v26.x — 信息流控制 · 变量级污点追踪 🔥🔥🔥🔥
> 从 trace 级污染升级到变量级污染 | 安全保证的数学证明
> 理论基础：Fides/IFC (Microsoft, arXiv:2505.23643) — Bell-LaPadula 模型 + 动态污点追踪
> 依赖：✅ v25.1 Capability 系统（提供标签基础设施）
> 参考实现：github.com/microsoft/fides

- [ ] v26.0 **双标签系统（Confidentiality + Integrity）**
  - **v20.1 到 v26.0 的飞跃**:
    - v20.1: trace 级标签（整个会话 TAINTED 或不 TAINTED）
    - v26.0: 字段级标签（每个变量独立的机密性+完整性标签）
  - **标签定义**:
    ```
    Confidentiality: PUBLIC < INTERNAL < CONFIDENTIAL < SECRET
    Integrity:       TAINT < LOW < MEDIUM < HIGH

    标签来源:
      system_prompt:      { conf: SECRET,       integ: HIGH }
      user_direct_input:  { conf: INTERNAL,     integ: MEDIUM }
      tool_response:
        web_fetch:        { conf: PUBLIC,       integ: TAINT }
        database_query:   { conf: CONFIDENTIAL, integ: LOW }
        mcp_tool_result:  { conf: INTERNAL,     integ: LOW }
      tool_call_params:
        shell_exec:       { required_integ: HIGH }
        send_email:       { required_integ: MEDIUM, max_conf: INTERNAL }
        file_write:       { required_integ: MEDIUM }
    ```
  - **传播规则**（Fides 核心）:
    ```
    var_c = func(var_a, var_b)
    → var_c.confidentiality = max(var_a.conf, var_b.conf)  // 机密性取高
    → var_c.integrity = min(var_a.integ, var_b.integ)       // 完整性取低
    ```
  - **安全检查**:
    - 机密性: 数据流向的通道 conf 等级 ≥ 数据的 conf 等级（不能泄露）
    - 完整性: tool call 要求的 integ 等级 ≤ 参数的 integ 等级（不能被污染数据驱动）
  - 与 v25.1 Capability 联动：capability 作为第三个维度，IFC 标签+capability 双重检查
  - Dashboard: IFC 策略编辑器（标签来源配置 + 传播规则可视化 + 违规日志）

- [ ] v26.1 **隔离 LLM（Quarantined LLM）**
  - **核心创新**（Fides 独有）: 被污染数据不能直接喂给主 Agent，需要先经过隔离 LLM 处理
  - **工作流**:
    ```
    正常路径: 用户消息 → 龙虾卫士 → 主 Agent（完整 tool 权限）
    隔离路径: 被污染数据 → 龙虾卫士检测到 integ=TAINT
              → 路由到隔离上游（只有只读 tool 权限）
              → 隔离 LLM 产出总结/摘要
              → 总结的 integ 提升为 MEDIUM（去污）
              → 主 Agent 使用去污后的总结
    ```
  - **龙虾卫士实现**: 利用现有多上游路由能力
    - 配置一个 "quarantine" 上游（OpenClaw 实例，限制 tool 为只读子集）
    - IFC 引擎检测到 TAINT 数据进入高 integ 操作 → 自动切换路由到 quarantine 上游
    - quarantine 输出自动标记为 MEDIUM integ
  - 与 v20.2 污染链逆转联动：quarantine 就是一种更优雅的"解毒"方式

- [ ] v26.2 **选择性隐藏（Selective Hiding）+ DOE 检测**
  - **选择性隐藏**: 向 LLM 隐藏高机密字段
    - LLM 看到的是: `{name: "张三", phone: "[REDACTED:conf=SECRET]", dept: "安全"}`
    - LLM 推理不基于被隐藏字段，保证机密性
  - **跨 Tool DOE 检测**（AgentRaft 启发, arXiv:2603.07557）:
    - 用 v25.0 的执行计划模板确定"任务最小字段集"
    - 比对 tool 间实际传输的字段 vs 最小集
    - 超出 → 标记 Data Over-Exposure → 按严重度 log/warn/block
    - DOE 三级: info（多传了非敏感字段）/ warning（PII 跨 tool 传输）/ critical（敏感数据跨信任边界）
  - Dashboard: 数据流视图（tool call graph + 高亮 DOE 路径 + 字段级别 diff）

### v27.x — Agent 身份 · MCP 安全网关 · 跨 Agent 安全 🔥🔥🔥
> Phase 2 的基础设施 + MCP 安全生态卡位
> 理论基础：SMCP (arXiv:2602.01129) + MCPShield (arXiv:2602.14281) + MCPSec (arXiv:2601.17549)
> 依赖：🔧 需要 OpenClaw 侧协议配合 + 新增 MCP Proxy 端口

- [ ] v27.0 **Agent 身份识别协议**
  - 定义 `X-Lobster-Agent-ID` / `X-Lobster-Agent-Name` / `X-Lobster-Session-Key` Header 规范
  - OutboundProxy 解析 Agent 元信息，审计日志增加 agent_id 维度
  - Agent 注册表：已知 Agent 列表 + 首次出现自动发现 + Dashboard 管理页面
  - 行为推断兜底：对不携带 Header 的流量，用 TraceCorrelator 时间窗口关联推断
  - Agent 维度聚合：安全事件 / 成本 / 行为画像 / IFC 违规 全部按 Agent 拆分

- [ ] v27.1 **MCP 安全网关（:8445）**
  - 龙虾卫士新增第四个监听端口，作为 MCP 安全代理
  - **SMCP 五层安全（透明叠加，不改 MCP 协议）**:
    1. 统一身份管理: MCP Server 注册 + 证书/Token 认证
    2. 双向认证: Agent↔Server 通过龙虾卫士中转时自动互验
    3. 安全上下文传播: v26.0 IFC 标签跨 MCP 调用自动传播
    4. 细粒度策略执行: 每个 tool call 经过 v23.0 路径策略 + v25.1 Capability 检查
    5. 全面审计: 所有 MCP 交互记录到独立 SQLite 表
  - **MCPShield 动态信任（运行时自适应）**:
    - 首次调用 MCP tool → 低信任分（受限模式：只允许读操作）
    - 连续 N 次正常调用 → 信任分提升（正常模式）
    - 检测到异常（返回数据含注入 / 超时 / schema 不匹配）→ 信任分暴跌 → 降级
    - 信任分存 SQLite，持久化跨重启
  - **MCPSec 协议加固（8.3ms 延迟实现 52.8%→12.4% 攻击成功率下降）**:
    - Capability Attestation: MCP Server 声明的权限 vs 实际行为 → 不一致告警
    - Message Authentication: 请求/响应 HMAC 签名（复用 v18.0 执行信封密钥）
    - Trust Isolation: 多 MCP Server 之间信任不传播（Server A 可信不代表 Server B 可信）
  - MCP 调用与 LLM tool_calls 通过 trace_id 关联
  - Dashboard: MCP 安全仪表盘（Server 注册表 · 信任分趋势 · 调用审计 · 威胁告警）

- [ ] v27.2 **跨 Agent 安全 + 蠕虫检测**
  - 依赖 v27.0 Agent 身份识别
  - **跨 Agent 污染传播**: Agent A 的 TAINTED 输出 → 成为 Agent B 的输入 → v26.0 IFC 标签跨 Agent 传播
  - **蠕虫检测**: 检测 Agent→Agent 感染链模式
    - 感染拓扑图可视化（传播路径、感染时间线）
    - 自动隔离已感染 Agent（切断路由、标记污染）
  - 与 v24.0 AttriGuard 联动：跨 Agent 的 tool call 也做反事实验证
  - 理论基础：洞见 #33/#35（蠕虫化 + 涌现安全）

### v28.x — AI 安全运营副驾驶 · 自动化验证 🔥🔥
> 安全运营智能化 + 论文 benchmark 接入
> 理论基础：Nash 均衡（安全均衡需要被设计）

- [ ] v28.0 **AI 安全运营副驾驶**
  - 自然语言查询安全态势："过去 24 小时有什么异常？""这个用户为什么风险分飙升了？"
  - 攻击链智能分析："这 5 个事件之间有什么关联？""下一步攻击者可能做什么？"
  - **因果归因可解释性**（v24.1 联动）：副驾驶可以解释"这个 tool call 被拦截是因为反事实验证表明它是由外部数据驱动的"
  - **IFC 违规分析**（v26.0 联动）：副驾驶可以解释"这个数据流违反了机密性规则，因为 SECRET 数据流向了 PUBLIC 通道"
  - 元认知安全框架（Reflexive-Core 启发）：预检→安全分析→受控执行→合规验证

- [ ] v28.1 **AgentDojo/AgentDyn Benchmark 自动化**
  - 集成 AgentDojo (NeurIPS 2024) + AgentDyn (arXiv:2602.03117) 安全基准测试
  - 定期自动运行 benchmark，追踪防御效果随版本的变化：
    - Task Completion Rate（功能不退化）
    - Attack Success Rate（安全在提升）
    - False Positive Rate（误报在下降）
  - CI/CD 集成：每次发版自动跑 benchmark，低于基线不允许合入
  - Dashboard: Benchmark 趋势页（每个版本的三个指标折线图）

- [ ] v28.2 **Guardrail 市场 + OpenTelemetry**
  - 社区贡献规则包（行业模板：金融/医疗/政务/教育）+ IFC 策略模板
  - 第三方检测器插件（基于 v19.2 SDK）
  - OpenTelemetry 接入：安全事件+IFC 标签 作为 span attribute 导出

### v29.x — 分布式部署 · 企业级
> 水平扩展 · 高可用 | 理论基础：CAP 定理（分布式安全的权衡选择）
> 依赖：🔧 需要 PostgreSQL + 多实例协调

- [ ] v29.0 **PostgresStore + 多实例**
  - SQLite → PostgreSQL 存储后端（保持 SQLite 兼容用于单机模式）
  - IFC 标签 + Capability + 路径策略 跨节点同步
  - 多实例部署：Leader 选举 · 路由状态同步 · 读写分离
  - 执行信封跨节点验证链（v18 延伸：分布式证据链完整性验证）
- [ ] v29.1 **弹性伸缩 + 零停机升级**
  - 滚动更新：新旧版本共存期间路由无感知切换
  - 配置热同步：修改一个节点的配置自动同步到集群

### 未来探索

> 以下是尚未排期但值得关注的方向

- [ ] **eBPF/LSM 内核级执行管控**（Telos 启发：在 Linux 内核层面拦截 Agent 系统调用）
- [ ] **Proof-of-Guardrail**（arXiv:2603.05786, TEE 密码学证明审计日志不可篡改）
- [ ] **联邦安全学习**（多组织共享攻击模式但不共享数据）
- [ ] **安全数字孪生**（克隆生产环境在镜像中运行红队测试）
- [ ] **形式化验证**（对 IFC 传播规则进行 Coq/Lean 数学证明）
- [ ] **量子安全密码学迁移**（后量子签名算法替换 HMAC）
- [ ] **SAFEFLOW 事务性 Agent 操作**（arXiv:2506.07564, write-ahead logging + rollback）
- [ ] **Capability-Safe Language Harness**（arXiv:2603.00991, EPFL Odersky, 类型系统保证信息流安全）

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
11. **🔴 审计准确性是根基** — 审计日志记录的上游必须是实际转发的上游，记录的决策必须是实际执行的决策。审计日志撒谎 = 安全网关失去存在意义。

---

## 🔴 版本迭代流程（四层质量保障体系）

> **v20.8 实战验证**：仅靠第一层 API QA，就绪度只有 55%。四层全跑完，达到 90%+。
> 每一层发现的问题类型完全不同，上一层找不到下一层的问题。

### 版本工作流总览

```
1. Review  — 遍历现有页面/API，画出新功能与旧功能关联图
2. Plan    — 新功能设计 + 旧功能适配清单 + 闭环检查表
3. Build   — 新功能 + 旧功能适配一起实现，不拆开
4. Quality — 四层质量保障（下方详述）
5. Deploy  — 部署测试服务器 + Dashboard 全页面走查
6. Release — 更新 README/CHANGELOG/config.example + Git tag
```

### 第一层：API 黑盒 QA（子 agent 自动化）

**方法**: 批量调 API，验证输入→输出契约
**耗时**: ~10 分钟（子 agent 并行）
**发现的问题类型**:
- 返回码错误（500 该 404）
- 字段缺失（计数不更新）
- 输入校验缺失（超长 ID、空字段）
- 并发计数准确性

**执行方式**: 子 agent 批量 curl 全部 API 端点 + 异常输入 + 并发压测

**v20.8 实战数据**: 发现 12 Bug（2P1 + 5P2 + 5P3），就绪度 55%

**局限**: 只能发现接口层表面 Bug，发现不了设计矛盾

---

### 第二层：数据流白盒审查（人工读代码）

**方法**: 读核心源码，追踪数据从入口→处理→存储→审计的完整生命周期
**耗时**: 30-60 分钟
**核心思维**: 在每个分支问 **"如果中间状态变了会怎样"**

**检查清单**:
- [ ] 路由决策（resolveUpstream）→ 实际转发 → 审计日志记录，三处是否一致？
- [ ] 策略变更 → 已有绑定 → 是否有事件传播机制？
- [ ] 重启后 → 内存状态（计数器/缓存）→ 是否从持久层恢复？
- [ ] 降级路径（上游不健康/proxy nil）→ 是否更新路由表 + 记录审计？
- [ ] 桥接模式 vs webhook 模式 → 同一配置两种模式行为是否对称？
- [ ] goroutine（grep "go func"）→ 共享状态 → 锁覆盖范围是否正确？

**发现的问题类型**:
- 数据流中隐含假设在边界场景下不成立
- 两步操作之间状态可能变化（TOCTOU）
- 不同代码路径对同一数据的处理不一致

**v20.8 实战数据**: 发现 9 个设计问题（3🔴 + 4🟡 + 2🟢），全部是第一层找不到的

**关键经验**:
- **map 遍历顺序不确定**: Go map 遍历不能用于降级选择
- **CAS 优于 Check-Then-Act**: 并发场景下 Lookup+Bind 必须原子化
- **同步等待+超时降级**: 比纯异步更适合首次请求（1500ms 是好的平衡点）

---

### 第三层：设计矛盾分析（架构层）

**方法**: 不看单个函数，看模块之间的契约和假设是否自洽
**耗时**: 穿插在第二层中，约 15 分钟专项
**核心思维**: **"这两个子系统对同一个概念的理解是否一致？"**

**检查清单**:
- [ ] 策略路由引擎 vs 亲和路由引擎 → 谁的优先级高？冲突时谁赢？
- [ ] 入站代理 vs 出站代理 → 对用户归属的理解是否一致？
- [ ] API CRUD → 运行时状态 → 是否有即时传播？还是要等下次请求？
- [ ] 配置文件 vs 数据库 → 哪个是 source of truth？冲突时谁覆盖谁？
- [ ] 多种部署模式（桥接/webhook/LLM proxy）→ 安全语义是否等价？

**发现的问题类型**:
- 两个正确的子系统组合后优先级冲突（亲和架空策略）
- 模块间缺少事件传播机制（策略变更不即时生效）
- 同一概念在不同模块中有不同的数据源（配置 vs 缓存 vs DB）

**v20.8 实战数据**: 发现 3 个 🔴 级架构问题，都是第一层+第二层找不到的

**典型反模式**:
- "这行代码没 Bug" × "那行代码也没 Bug" × "但组合起来有 Bug" = 设计矛盾
- "入站记录了 team1" × "实际转发了 team3" × "审计日志说 team1" = 审计撒谎

---

### 第四层：端到端实战验证（真实流量 + Dashboard 浏览器）

**方法**: 配真实 LLM API、真实多用户、通过 Dashboard 浏览器操作，模拟生产环境
**耗时**: 30-60 分钟
**核心思维**: **"从用户视角，这个功能能不能用？数据能不能看到？"**

#### 4a. 模拟流量测试

**API**: `POST /api/v1/simulate/traffic`

**核心原则**: 不直接 INSERT 假数据，所有模拟数据必须流过完整业务管道。

**执行时机**:
- 每个版本 Build 完成后、提交前
- 修复任何数据流相关 bug 后
- 新增检测规则/引擎后

**14+ 验证场景**（随版本迭代扩展）:

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

#### 4b. Dashboard 浏览器全页面验证

每个页面必须通过浏览器操作验证（不是 curl）：
- [ ] 数据是否正确展示（和 API 返回一致）
- [ ] CRUD 操作是否正常（创建/编辑/删除）
- [ ] 搜索/筛选/分页是否工作
- [ ] 错误状态是否有合理提示（空数据/加载失败/权限不足）
- [ ] 跨页面跳转链接是否正确（审计→会话回放→用户画像）

#### 4c. 异常场景清单

| 类别 | 场景 |
|------|------|
| 网络 | 上游全部不健康 · 上游响应超时 · 上游返回非 JSON |
| 输入 | 空 body · 超大 body (>10MB) · 非 JSON · 缺失关键字段 · Unicode/Emoji |
| 并发 | 50+ 并发同一用户 · 50+ 并发不同用户 · 策略变更与请求并发 |
| 状态 | 重启后恢复 · 上游动态加入/移除 · 规则热更新期间的请求 |
| 安全 | Prompt Injection (中/英/Base64) · PII 泄露 · 凭据泄露 · 高危工具调用 |

**发现问题时**: 不是修模拟数据，是修业务代码。模拟暴露的就是真实缺陷。

**迭代演进**: 每新增一个功能模块，同步在验证场景中增加对应条目。

---

### 四层效率模型（v20.8 实战数据）

| 层 | 方法 | 耗时 | 发现数 | Bug 严重度 | 就绪度提升 |
|----|------|------|--------|-----------|-----------|
| 1 | API 黑盒 QA | ~10min/轮 | 17 | P2-P4 | 0% → 55% |
| 2 | 数据流白盒审查 | ~1h | 9 | 🔴×3 🟡×4 🟢×2 | 55% → 75% |
| 3 | 设计矛盾分析 | ~15min | 3 | 全 🔴 | 75% → 85% |
| 4 | E2E 实战验证 | ~1h | TBD | 集成级 | 85% → 90%+ |

**核心结论**:
- **成本递增但严重度也递增** — 第一层便宜但找不到致命问题，第三层贵但找到的都是根基级问题
- **上一层找不到下一层的问题** — API QA 永远发现不了"亲和路由架空策略路由"
- **四层全跑才能达到 90%** — 只跑第一层就发版 = 55% 就绪度上线

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
Phase 1 — 纯流量（不改上下游，只靠已有三条数据通道）:
  v18:     密码学信任根 + 事件总线 + 工程化 (Docker/CI/OpenAPI)
  v19:     对抗性自进化 + 语义检测模型 + 插件 SDK
  v20:     LLM tool_calls 深度分析 + 信息流污染追踪 + 响应缓存 + API Gateway
  v22:     Gateway 监控 + Agent 运营中心
------- v22.4 当前版本 · 以下为规划 -------
Phase 2 — 学术前沿转化（2025-2026 论文 → 产品功能，先叠加后重构）:
  v23:     路径级策略引擎 (Runtime Governance)         ← 最小改造量
  v24:     因果归因防御 (AttriGuard 反事实验证)        ← 防御范式革命
  v25:     控制流/数据流分离 (CaMeL 执行计划编译)      ← 核心架构升级
  v26:     信息流控制 (Fides IFC 变量级污点追踪)       ← 数学保证安全
Phase 3 — 生态扩展（MCP 安全网关 + 跨 Agent + 企业级）:
  v27:     Agent 身份 + MCP 安全网关 + 跨 Agent 蠕虫检测
  v28:     AI 安全副驾驶 + Benchmark 自动化 + Guardrail 市场
  v29:     分布式部署 + PostgreSQL + 弹性伸缩

产品定位演进:
  v1-v5:   AI Agent 安全网关（被动防御 — 规则匹配）
  v6-v13:  AI Agent 安全运营中心（防御 + 可视化 + 审计 + 合规）
  v14-v17: 安全治理 + 态势感知（治理 + 主动防御 + 智能分析）
  v18-v22: 可证明安全 + 自进化 + 信息流追踪（Phase 1 纯流量，把已有数据吃干榨净）
  v23-v26: 🆕 可证明安全 2.0 — 学术前沿转化（因果推断 + 执行计划 + IFC + 路径策略）
  v27-v28: MCP 安全网关 + AI 安全副驾驶（Phase 3 生态扩展）
  v29:     企业级分布式（水平扩展 + 高可用）

三个产品线:
  A — Agent 防注入引擎（核心壁垒）  : v25 CaMeL + v24 AttriGuard + v19 语义检测
  B — MCP 安全网关（市场热点卡位）   : v27 SMCP/MCPShield/MCPSec
  C — 合规审计引擎（卖给政企）       : v26 IFC + v23 路径策略 + v12 合规报告
```

### 每个版本的理论根基

| 版本 | 理论基础 | 核心论文 | 核心洞见 |
|------|---------|---------|---------|
| v18 | Gödel 不完备定理 | — | 安全无法自证 → 用密码学逼近可证明 |
| v19 | 熵增定律 + 耗散结构 | — | 安全退化是物理必然 → 自进化对抗熵增 |
| v20 | Shannon 信息论 | — | trace_id 是污染载体 → 三段联合追踪 |
| **v23** | **Runtime Governance** | **arXiv:2603.16586** | **执行路径是治理核心 → 策略 = f(路径)** |
| **v24** | **因果推断** | **arXiv:2603.10749 AttriGuard** | **不问"输入像不像注入"，问"tool call 被谁驱动"** |
| **v25** | **CaMeL 控制流分离** | **arXiv:2503.18813 Google** | **不检测坏的，编译好的 → 注入无通道可走** |
| **v26** | **Bell-LaPadula IFC** | **arXiv:2505.23643 Microsoft Fides** | **每个变量有数学标签 → 安全是可证明的** |
| v27 | SMCP + MCPShield | arXiv:2602.01129 / 2602.14281 | MCP 安全是架构问题 → 网关层透明加固 |
| v28 | Nash 均衡 | AgentDojo / AgentDyn | 安全均衡需要被设计 → AI 辅助全局视角 |
| v29 | CAP 定理 | — | 分布式安全三选二 → 明确权衡选择 |

### 每个版本的反直觉创新 🔥

| 版本 | 反直觉 |
|------|--------|
| v18 | 日志不只是记录，是**密码学证据** |
| v19 | 安全系统**自己攻击自己、自己修复自己** |
| v20 | 不检测内容，**追踪数据的血统** |
| **v23** | 🆕 同一操作在不同历史下**安全判定不同**（规则有记忆） |
| **v24** | 🆕 不检查输入内容，**反事实重放问 LLM "没有外部数据你还这么做吗？"** |
| **v25** | 🆕 不拦截 LLM 的 tool call，**先编译"允许做什么"再对比** — 注入在设计上无路可走 |
| **v26** | 🆕 安全不是 if-else，是**数学定理** — 标签传播规则保证信息不泄露 |
| v27 | MCP Server 说自己安全？**让龙虾卫士验证** — 信任要靠行为赢取不是声明 |
| v28 | 安全运营不是看 Dashboard，是**跟安全助手对话** |
| v29 | 单二进制 → 集群，但**零配置迁移** |

### 🆕 学术论文→产品功能 映射表

| 论文 | 年份 | 引用 | 对应版本 | 龙虾卫士功能 | 改造量 |
|------|------|------|---------|-------------|--------|
| Runtime Governance (2603.16586) | 2026.03 | — | **v23** | PathPolicyEngine 路径级策略 | 小 |
| AttriGuard (2603.10749) | 2026.03 | — | **v24** | CounterfactualVerifier 反事实验证 | 中 |
| CaMeL (2503.18813) Google | 2025.03 | ⭐⭐⭐⭐⭐ | **v25** | PlanCompiler + Capability 权限 | 中大 |
| Fides/IFC (2505.23643) Microsoft | 2025.05 | 30 | **v26** | 双标签系统 + Quarantined LLM | 大 |
| Dual Firewall (2502.01822) | 2025.02 | — | v25.0 融合 | 结构化入站 = Plan Template | — |
| AgentRaft/DOE (2603.07557) | 2026.03 | — | v26.2 融合 | 跨 Tool 数据过度暴露检测 | — |
| SMCP (2602.01129) | 2026.02 | — | v27.1 | MCP 五层安全透明叠加 | 中 |
| MCPShield (2602.14281) | 2026.02 | — | v27.1 | MCP Tool 动态信任评分 | 小 |
| MCPSec (2601.17549) | 2026.01 | — | v27.1 | 协议级加固 (52.8%→12.4% ASR) | 中 |
| MCP Landscape (ACM TOSEM) | 2025 | 420 | v27 需求输入 | 16 种 MCP 威胁场景需求清单 | — |
| AgentDojo (NeurIPS 2024) | 2024 | — | v28.1 | Benchmark 自动化测试 | 小 |
| AgentDyn (2602.03117) | 2026.02 | — | v28.1 | 动态 Benchmark 测试 | 小 |
| AgentVigil (2505.05849) | 2025.05 | 28 | v19 已有 | 红队自动化（已通过 v14.2+v19 覆盖）| — |
| SAFEFLOW (2506.07564) | 2025.06 | 12 | 未来探索 | 事务性 Agent 操作 | 大 |
| Proof-of-Guardrail (2603.05786) | 2026.03 | — | 未来探索 | TEE 密码学审计证明 | 大 |
| Tracking Capabilities (2603.00991) | 2026.03 | — | v25.1 参考 | 能力安全类型系统（理念借鉴）| — |

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
