# Changelog

## v20.8.1 (2026-03-23) — Taint 全链路闭环 + 设计级修复 + E2E 验证

> 125 Go 源文件 · 55 测试文件 · ~75,000 行 · 1112 测试全部通过 · 197 commits

### 🔄 Taint 全链路（IM↔LLM trace 关联 + SSE 逆转）

- **IM↔LLM trace 关联** — `llm_proxy.go` 新增 `taintTraceID` 变量，优先使用 SessionCorrelator 关联的 IM trace_id，解决 LLM trace_id 与入站 IM trace_id 不匹配导致 taint 链断裂 (c2e23fa)
- **非流式自动逆转** — `reversalEngine.Reverse()` 在非流式 LLM 响应路径自动调用，不再依赖手动 API 触发；taint propagation 统一使用 `taintTraceID` (c2e23fa)
- **SSE 流式逆转** — `handleSSEResponse()` 接收 `taintTraceID` 参数，流结束后自动检查 taint 并追加自定义 SSE 事件 `event: lobster_guard_taint_reversal`，客户端通过 event type 区分正常输出和安全缓解提示 (a377690)

### 🏗️ 设计级修复（9 项）

- **D-001** 策略上游不健康时记录 `policy_degraded` 审计事件 + Dashboard 告警，不再静默降级
- **D-002** 转发降级时更新路由表绑定和 user_count，审计日志记录实际上游 ID
- **D-003** 桥接模式转发支持 `path_prefix`，降级使用 `SelectUpstream` 而非 map 随机遍历
- **D-004** 策略 CRUD 变更后触发全量路由重评估（`reevaluateAllRoutes`）
- **D-005** 出站代理注入 `X-Lobster-Upstream` header 标记来源上游
- **D-006** 启动时从路由表聚合恢复 `user_count`，重启后 least-users 策略立即生效
- **D-007** 默认策略 `upstream_id=""` 在 Dashboard 显示为"不路由(LB 托管)"，创建时校验
- **D-008** 检测超时默认值调整为 200ms，语义/LLM 阶段支持独立超时配置
- **D-009** 桥接模式 block 时主动推送拦截提示消息给发送者

### 🔬 E2E 验证修复（7 项 + QA R2 5 项）

- **E2E #1** Token 计数支持 OpenAI 格式 (`prompt_tokens`/`completion_tokens`) — `llm_audit.go` 添加格式 fallback
- **E2E #2** 新增中国 PII 检测规则 `llm-pii-004`（身份证）、`llm-pii-005`（手机号）
- **E2E #3** `LLMRule` 结构体添加 `Severity` 字段
- **E2E #4** 中国 PII 正则改进：添加 word boundary anchors 防止部分匹配；`buildIndex()` 添加规则编译统计日志
- **E2E #5** 静态上游启用 TCP 健康检查（3s dial timeout），不再跳过
- **E2E #6** `ParseSSEEvents()` 支持 OpenAI SSE 流式 token 格式
- **E2E #7** LLM 规则命中计数持久化到 SQLite（`llm_rule_hits` 表），重启后恢复
- **QA R2** sender_id 长度限制（≤256）、空 match 策略拒绝创建、不存在用户查询返回 404、策略上游存在性校验、路由绑定原子性保证

### 🔀 LLM Proxy 增强

- **strip_prefix 路由** — `LLMTargetConfig` 新增 `StripPrefix bool` 字段 (`strip_prefix` in YAML)；当 `strip_prefix: true` 时，转发前去掉 `path_prefix`（如 `/qax/v1/...` → `/v1/...`），支持将 OpenClaw LLM 流量透明引入龙虾卫士审计 (db80b8e)

### 📊 质量方法论

- **DESIGN-REVIEW.md** — 9 项深层设计问题系统审查（数据流追踪、边界行为分析）
- **QA-REPORT-R2.md** — 46 场景第二轮测试，33 PASS / 4 FAIL / 9 INFO，旧 7 Bug 全部回归通过
- **E2E-TEST-REPORT.md** — 全链路实战测试：LLM 安全域 15+ 请求、IM 侧 53+ 消息（含 50 并发）、17 页面 Dashboard 验证
- **全链路闭环验证** — 蓝信→入站检测→OpenClaw→LLM Proxy→taint 传播→SSE 逆转→出站审计，3 条消息（正常/PII/PI）端到端通过

---

## v20.8.0 (2026-03-22) — 安全加固 + 规则模板 + path_prefix

### 🔒 安全加固 (Issue #1)
- **移除硬编码默认密码** — 未配置时自动生成随机 16 字符密码
- **移除密码明文日志** — 仅输出密码长度
- **移除 URL query token 认证** — 改用 Cookie (`lg_token`) 方式
- **WebSocket Origin 白名单** — 新增 `ws_allowed_origins` 配置项
- **密码最小长度提升** — 从 4 位提高到 8 位（创建+修改共 3 处）
- **SQLite 文件权限** — 自动设置 0600
- **JWT 未配置警告加强** — 明确提示生产环境必须配置

### 🛡️ 规则模板库 (64 条规则)
- **通用模板** (general.yaml) — 16 条：Prompt Injection / Jailbreak / 命令注入 / 社工 / 数据泄露
- **金融模板** (financial.yaml) — 16 条：银行卡 PII / 反洗钱 / 制裁 / BEC / 交易合规
- **医疗模板** (medical.yaml) — 16 条：HIPAA 患者隐私 / 用药安全 / 过敏检查 / 医保欺诈
- **政务模板** (government.yaml) — 16 条：涉密信息 / 公民 PII / 数据跨境 / 政策泄露

### 🔀 上游路径前缀 (path_prefix)
- 上游配置新增 `path_prefix` 字段，支持非根路径上游服务
- DB 持久化：schema 迁移 + 读写完整链路
- `path.Clean()` 防路径穿越
- **Dashboard 上游管理页面**：列表显示前缀标签、展开详情、编辑表单

### 🔧 工程改进
- `.gitignore` 不再排除 `src/rules/`，规则模板纳入版本控制
- `config.yaml.example` 新增 `path_prefix`、`ws_allowed_origins`、安全配置注释
- **main 分支保护**：必须 PR + 1 approve review 才能合并

## v20.7.0 (2026-03-21) — Dashboard 企业级打磨

### 🏢 Dashboard 全面重构

**38 个页面全部打磨到企业产品级**，覆盖 5 个 Phase：

#### Phase 1: 核心安全（5 页面）
- **Rules**: 出站规则完整 CRUD、批量操作（启用/禁用/删除）、搜索过滤（名称+pattern+action+group）、正则语法实时校验、优先级可视化
- **LLMRules**: 4 维过滤（类别/动作/方向/状态）、内嵌规则测试器、批量操作、影子模式视觉区分
- **Audit**: 统计面板（StatCard）、高级过滤+URL 参数同步、日志详情展开+关联跳转（用户画像/会话回放/规则页）、归档管理
- **Overview**: 时间范围选择器（1h/6h/24h/7d/30d）、快捷操作区、自动刷新（30s/1m/5m）、健康分 SVG 环形图、数据变化闪烁
- **Sessions**: 聊天气泡风格（入站蓝/出站绿）、安全事件标注（拦截红/告警橙）、标签注释系统、导出 JSON/Markdown、键盘导航

#### Phase 2: 基础设施（6 页面）
- **Upstream**: 搜索过滤、批量健康检查、健康状态可视化（绿/红/灰发光圆点）、K8s 发现面板、表单验证（地址格式+重复检查）
- **Routes**: 三 Tab（路由/策略/可视化）、策略优先级调整（↑↓按钮）、批量解绑/迁移、SVG 绑定关系图+负载饼图
- **Settings**: ⭐ **6 组配置页面化**（基础/安全/限流/会话/告警/高级）、变更预览面板（旧值→新值）、**YAML 回写**、需重启标记
- **Operations**: YAML 语法高亮+行号、备份策略配置、系统诊断 6 宫格、告警规则 CRUD+静默期+测试
- **Users**: 密码强度指示器、角色权限说明面板、操作审计弹窗、重置密码（手动/随机）
- **Tenants**: 卡片/列表双视图、成员批量管理、安全配置分组+继承说明

#### Phase 3: 高级安全（7 页面）
- **AttackChain**: 攻击时间线（彩色 dot+连接线）、处置操作（确认/误报/封禁）、分析配置面板
- **AnomalyDetection**: 基线管理+24h 迷你 SVG 图、独立阈值配置、趋势图弹窗、异常脉冲动画
- **SemanticDetector**: 5 统计卡、四维雷达图（TF-IDF/句法/异常/意图）、攻击模式库搜索+快速测试、检测历史、配置面板
- **RedTeam**: 场景管理+自定义场景创建、批量执行+进度动画、测试历史+漏洞报告
- **TaintTracker**: 传播路径垂直时间线、清理操作、配置 Tab（检测/逆转）
- **Honeypot**: 模板搜索过滤+编辑删除、部署管理 Tab、触发记录展开详情
- **Singularity**: 诱饵配置面板、捕获记录搜索+历史 Tab

#### Phase 4: 数据洞察（5 页面）
- **Reports**: 报告模板管理（3 预置+自定义）、定时任务 CRUD、预览/下载、生成进度动画
- **Monitor**: 4 指标图表+时间范围联动、阈值设置模态框、告警联动（超阈值红色脉冲）、自动刷新
- **PromptTracker**: 版本 diff 对比（split/unified）、标签管理（production/staging/deprecated）、一键回滚
- **LLMOverview**: 快捷操作栏、Token 消耗双饼图（按模型/按用户）
- **LLMCache**: 缓存列表+搜索、策略配置（TTL/大小/LRU-LFU）、批量操作

#### Phase 5: 辅助功能（15 页面）
- **Envelopes**: 3 Tab（信封列表/Merkle 批次/链验证）、配置模态框
- **EventBus**: 4 Tab（事件流/推送目标/送达记录/Action Chain）、目标 CRUD
- **Evolution**: 3 Tab（进化日志/变异策略/学习曲线）、配置模态框
- **Leaderboard**: 时间范围+多维排行（健康分/事件/拦截率）、导出 CSV/JSON、金银铜样式
- **ABTesting**: 统计面板、显著性检验结果、搜索过滤、应用胜出方案
- **ToolPolicy**: 搜索过滤+批量启用/禁用/删除+策略详情展开
- **BehaviorProfile**: 策略配置面板+行为模式可视化+24h 活跃热力格
- **AgentBehavior**: 行为规则 CRUD+异常标记+高危导出
- **UserProfiles**: 风险评分进度条+搜索过滤+排序
- **UserDetail**: 行为时间线+封禁/解封+标签系统+关联数据
- **APIGateway**: 搜索+方法过滤、JWT 配置增强、路由测试 Tab、批量操作
- **BigScreen**: 全屏模式（F11）、数字滚动动画、告警闪烁
- **CustomDashboard**: 布局保存/加载+预览模式切换
- **Login**: 记住登录+密码显隐+表单验证增强

### 🔧 后端新增

#### 新增 API 端点
- `PUT /api/v1/config/settings` — 批量更新配置（回写 config.yaml + 更新内存）
- `POST /api/v1/alerts/test` — 发送测试告警
- `PUT /api/v1/alerts/config` — 更新告警配置
- `POST /api/v1/routes/batch-unbind` — 批量解绑路由
- `POST /api/v1/routes/batch-migrate` — 批量迁移路由
- `GET /api/v1/anomaly/metric-thresholds` — 获取指标独立阈值
- `PUT /api/v1/anomaly/metric-thresholds/:name` — 设置指标阈值
- `GET /api/v1/anomaly/trend/:name` — 24h 趋势数据
- `POST /api/v1/prompts/:hash/tag` — 设置 Prompt 版本标签
- `POST /api/v1/prompts/:hash/rollback` — 回滚 Prompt 版本
- `GET /api/v1/prompts/stats` — Prompt 统计概览
- `POST /api/v1/taint/cleanup` — 批量清理过期标记
- `DELETE /api/v1/taint/entry/:trace_id` — 删除污染标记
- `POST /api/v1/taint/inject` — 手动注入标记

#### 数据库迁移
- `prompt_versions` 表新增 `tag` 列

#### 代码统计
- Go 源代码: ~44,100 行（+1,700）
- Vue 前端: ~23,700 行（+3,300）
- 测试: 950 个全部通过

### 📐 打磨标准（每个页面满足）
- ✅ CRUD 闭环（能创建就能编辑、删除）
- ✅ 配置页面化（不需要改 YAML）
- ✅ 操作反馈（Toast 成功/失败 + Loading 状态）
- ✅ 确认对话框（ConfirmModal 二次确认）
- ✅ 空状态引导（EmptyState + 操作按钮）
- ✅ 搜索和过滤
- ✅ 批量操作（多选+批量启用/禁用/删除）
- ✅ 表单验证（必填+格式+实时错误提示）
- ✅ 统一 Indigo (#6366F1) 配色
- ✅ 响应式布局（1024px+）
- ✅ Skeleton 加载骨架屏

---

## v20.6.0 (2026-03-20) — 分层配置 + K8s 部署

- 分层配置: config.yaml 从 776 行精简到 ~70 行，模块配置拆分到 conf.d/
- Dockerfile 多阶段重构
- K8s 部署清单（4 文件）
- docker-compose 健康检查

## v20.5.0 (2026-03-19) — K8s 服务发现 + 上游管理

- K8s 服务发现（零依赖 InCluster/Kubeconfig）
- 上游 CRUD API
- 登录页粒子光影
- 威胁地图环形拓扑
- 侧边栏子分组
- emoji → SVG 全站清理
