# 🖥️ Dashboard 文档

> Vue 3 + Vite · 深色科技主题 · Indigo (#6366F1) 配色 · 零 UI 库

## Vue 3 Dashboard（50 页面 · 75 组件）

单二进制嵌入（`go:embed dashboard/dist/*`），无需独立前端服务器。

### v33.0 企业级打磨标准

每个页面都满足：
- ✅ CRUD 闭环（能创建就能编辑、删除）
- ✅ 配置页面化 — 所有配置项都在页面操作，修改自动回写 config.yaml
- ✅ 操作反馈 — Toast 成功/失败 + Loading 状态 + Skeleton 骨架屏
- ✅ 确认对话框 — ConfirmModal 二次确认（危险操作必须）
- ✅ 空状态引导 — EmptyState + 操作按钮
- ✅ 搜索和过滤 — URL 同步参数
- ✅ 批量操作 — 多选 + 批量启用/禁用/删除
- ✅ 表单验证 — 必填 + 格式 + 实时错误提示
- ✅ 统一 Indigo (#6366F1) 配色
- ✅ 响应式布局（1024px+）

---

## 页面列表（50 页面）

### 总览与监控（5 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| Overview | `/overview` | 安全驾驶舱 · 健康分环形图 · 时间范围选择器 · 快捷操作 · 自动刷新 |
| CustomDashboard | `/custom` | 自定义大屏 · 布局保存/加载 · 预览模式 |
| AnomalyDetection | `/anomaly` | 异常检测 · 基线管理 + 迷你 SVG 图 · 独立阈值配置 · 趋势图弹窗 |
| Monitor | `/monitor` | 监控指标 · 4 指标图表 · 阈值设置 · 告警联动 · 自动刷新 |
| BigScreen | `/bigscreen` | 态势大屏 · 全屏(F11) · 数字滚动 · 告警闪烁 |

### IM 安全域（7 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| Audit | `/audit` | 审计日志 · 统计面板 · 高级过滤 + URL 同步 · 日志详情展开 · 归档管理 |
| SessionReplay | `/sessions` | 会话回放 · 统计卡片 · 高级筛选 · 风险标记 · 卡片展开预览 |
| SessionDetail | `/sessions/:traceId` | 会话详情 · 聊天气泡风格 · 安全事件标注 · 标签注释 · 导出 JSON/MD |
| AttackChain | `/attack-chains` | 攻击链分析 · 攻击时间线 · 处置操作(确认/误报/封禁) · 分析配置 |
| UserProfiles | `/user-profiles` | 用户画像 · 风险评分进度条 · 搜索过滤 + 排序 |
| UserDetail | `/user-profiles/:id` | 用户详情 · 行为时间线 · 封禁/解封 · 标签系统 · 关联数据 |
| SecurityOverview | `/behavior` | **安全画像(v33)** · Treemap(面积=用户数,颜色=评分) · 甜甜圈 · 5 档分段 · Canvas 粒子系统 · 三级穿透(概览→排名→详情) · 16 引擎告警网格 |

### IM 规则与路由（5 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| Rules | `/rules` | 入站/出站规则 · 出站 CRUD · 批量操作 · 正则校验 · 行业模板管理 · review 四级动作 |
| InboundTemplates | `/inbound-templates` | **行业模板(v31)** · 40 模板 × 12 分类 · Toggle 启用 · 规则详情 · 入站+LLM+出站三维度 |
| Routes | `/routes` | 路由策略 · 三 Tab · 策略优先级调整 · 批量解绑/迁移 · SVG 绑定关系图 |
| Upstream | `/upstream` | 上游管理 · 搜索过滤 · 批量健检 · 健康可视化 · K8s 发现面板 |
| APIKeys | `/apikeys` | **API Key 管理(v27)** · 创建/编辑/轮换/删除 · SHA-256 哈希存储 · 日配额 |

### LLM 安全域（9 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| LLMOverview | `/llm` | LLM 概览 · 快捷操作 · Token 消耗双饼图 |
| LLMRules | `/llm-rules` | LLM 规则 · 4 维过滤 · 规则测试器 · 批量操作 · 影子模式 |
| LLMTargets | `/llm-targets` | LLM 代理目标 · 上游配置 · 请求/响应规则绑定 |
| LLMTemplates | `/llm-templates` | LLM 行业模板 · 模板 CRUD · 分类筛选 |
| LLMCache | `/cache` | 响应缓存 · 缓存列表 + 搜索 · 策略配置(TTL/LRU/LFU) · 批量操作 |
| SessionReplay | `/sessions` | 会话回放(LLM 视角) · trace_id 串联 IM+LLM+tool_calls |
| AgentBehavior | `/agent` | Agent 行为 · 规则 CRUD · 异常标记 · 高危导出 |
| PromptTracker | `/prompts` | Prompt 追踪 · 版本 diff(split/unified) · 标签管理 · 回滚 |
| ABTesting | `/ab-testing` | A/B 测试 · 统计面板 · 显著性检验 · 应用胜出方案 |

### LLM 高级引擎（4 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| ToolPolicy | `/tools` | 工具策略 · 搜索过滤 · 批量操作 · 策略详情展开 |
| PlanCompiler | `/plan-compiler` | **执行计划编译器(v25)** · CaMeL 意图→模板 · 20+ 内置模板 · 违规记录 |
| Capability | `/capability` | **能力标签引擎(v25)** · 工具→权限映射 CRUD · 权限评估记录 · 信任分 |
| PlanDeviation | `/deviations` | **偏差检测器(v25)** · 计划 vs 实际对比 · 自动修复 · 策略配置 |

### 威胁分析（4 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| SemanticDetector | `/semantic` | 语义检测 · 四维雷达图 · 模式库搜索 + 快速测试 · 检测历史 |
| Singularity | `/singularity` | 奇点蜜罐 · 诱饵配置 · 捕获记录 + 历史 Tab |
| Honeypot | `/honeypot` | Agent 蜜罐 · 模板搜索 · 部署管理 Tab · 触发记录展开 |
| TaintTracker | `/taint` | 污染追踪 · 传播路径时间线 · 清理操作 · 配置 Tab |

### 安全治理（8 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| PathPolicy | `/path-policy` | **路径策略引擎(v23)** · 序列/累计/条件规则 · 风险仪表 · 策略模板(AI Act) |
| Counterfactual | `/counterfactual` | **反事实验证(v24)** · 验证报告 · 成本追踪 · 效果评估 · 自适应配置 |
| IFC | `/ifc` | **信息流控制(v26)** · Bell-LaPadula 双标签 · 来源规则 · 工具要求 · 违规记录 |
| Tenants | `/tenants` | 租户管理 · 双视图 · 成员批量 · 安全配置分组 |
| Reports | `/reports` | 报告中心 · 模板管理 · 定时任务 · 预览/下载 |
| Leaderboard | `/leaderboard` | 排行榜 · 时间范围 · 多维排行 · 导出 CSV/JSON |
| Envelopes | `/envelopes` | 执行信封 · 3 Tab(信封/Merkle/链验证) · 配置管理 |
| Evolution | `/evolution` | 自进化 · 3 Tab(日志/策略/学习曲线) · 配置 |

### 系统管理（6 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| GatewayMonitor | `/gateway-monitor` | **Gateway 远程管理(v29)** · WSS RPC 持久连接 · 10+ Tab: 仪表盘/Sessions/Cron/Agent/Skills/文件/心跳/记忆/配置/安全画像 |
| APIGateway | `/gateway` | API 网关 · 路由列表 · JWT 校验 · 路由测试 Tab |
| Operations | `/ops` | 运维工具 · YAML 高亮 · 备份策略 · 诊断 6 宫格 · 告警 CRUD |
| Settings | `/settings` | 系统设置 · 7+ Tab: 基础/入站/出站/LLM/检测引擎/AC 分级/Gateway |
| Users | `/users` | 用户管理 · 密码强度 · 角色权限 · 操作审计弹窗 |
| Login | `/login` | 登录 · 记住登录 · 密码显隐 · 粒子光影 |

### 独立子页面（2 页面）

| 页面 | 路由 | 功能 |
|------|------|------|
| RedTeam | `/redteam` | 红队测试 · 场景管理 + 自定义 · 批量执行 + 进度 · 漏洞报告 |
| EventBus | `/events` | 事件总线 · 4 Tab(事件流/目标/送达/ActionChain) · 目标 CRUD |

---

## 侧边栏导航分组

| 分组 | 页面 |
|------|------|
| 总览与监控 | Overview · CustomDashboard · AnomalyDetection · Monitor |
| IM 安全 | Audit · Sessions · AttackChain · UserProfiles · SecurityOverview(安全画像) |
| LLM 安全 | LLMOverview · LLMRules · LLMTargets · AgentBehavior · Sessions · Prompts · ABTesting · ToolPolicy · PlanCompiler · Cache |
| 威胁中心 | UserProfiles · Honeypot · AttackChain · Anomaly · Singularity · Semantic · Taint · SecurityOverview |
| 策略引擎 | Rules · InboundTemplates · LLMRules · LLMTemplates · PathPolicy · IFC · Capability · Counterfactual · PlanDeviation |
| 安全治理 | Reports · Tenants · APIKeys · RedTeam · Leaderboard · Envelopes · Events · Evolution |
| 运营管理 | GatewayMonitor · APIGateway · Operations · Settings · Users |

## 通用组件（25 个）

| 组件 | 功能 |
|------|------|
| DataTable | 排序 · 分页 · 展开行 · 多选 · 搜索 |
| StatCard | 统计卡片 · 动画数字滚动 · 趋势 · 可点击 |
| ConfirmModal | 危险操作二次确认 · 红色警告 |
| EmptyState | 空状态引导 + 操作按钮 |
| Toast | 全局消息提示 · 成功/失败/警告 |
| Skeleton | 加载骨架屏 |
| Icon | SVG 图标系统（语义化 icon name） |
| TrendChart | SVG 趋势折线图 |
| PieChart | SVG 饼图 |
| HeatMap | CSS 热力图 |
| RuleEditor | 规则编辑器 · 正则测试器 · 优先级 |
| RegexTester | 正则表达式实时测试 |
| JsonHighlight | JSON 语法高亮 |
| BindModal | 绑定关系弹窗 |
| UpstreamSelect | 上游选择器 |
| PolicyModal | 策略路由编辑弹窗 |
| ConfigWizard | 配置向导（新手引导） |

## 技术栈

| 项目 | 版本/说明 |
|------|-----------|
| Vue | 3.5 (Composition API + `<script setup>`) |
| Vue Router | 4.5 (Hash 模式) |
| Vite | 6.2 (构建工具) |
| UI 库 | 零依赖（全部手写） |
| 图表 | 纯 SVG + CSS（无 Chart.js/D3） |
| 粒子系统 | Canvas 2D（SecurityOverview, Login） |
| 嵌入方式 | `go:embed dashboard/dist/*` → 单二进制 |

## 版本变更日志

| 版本 | 变更 |
|------|------|
| v33.0 | 安全画像替换行为画像 · Treemap(面积=用户数) · 甜甜圈 · 粒子系统 · 三级穿透 |
| v32.0 | 43 页面全量巡检 · 骨架屏/Toast/fade · 配色统一 · 交互打磨 |
| v31.0 | 行业模板管理 UI · 分类筛选 · Toggle 启用 · 规则详情 |
| v30.0 | Settings 7-Tab · AC 分级设置 · 引擎联动修复 |
| v29.0 | GatewayMonitor 10+ Tab · WSS RPC · Sessions/Cron/Agent/Skills CRUD |
| v27.0 | API Key 管理页面 · 租户策略闭环 |
| v26.0 | IFC 信息流控制页面 · Bell-LaPadula |
| v25.0 | PlanCompiler · Capability · PlanDeviation 三页面 |
| v24.0 | Counterfactual 反事实验证页面 |
| v23.0 | PathPolicy 路径策略页面 + 风险仪表 |
| v22.0 | 侧边栏子分组 · SVG 图标系统 · 登录页粒子光影 |
| v20.7 | 38 页面 CRUD 闭环 · 配置页面化 · Indigo 配色 |
