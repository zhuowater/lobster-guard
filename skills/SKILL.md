# 🦞 Lobster Guard — 龙虾卫士 v33.0 智能管控


## 连接配置

```bash
LOBSTER_GUARD_URL="http://10.44.96.142:9090"   # 管理 API 地址
LOBSTER_GUARD_TOKEN="test-token-2026!"          # Bearer Token 或 JWT
```

所有 API 调用格式：`curl -s -H "Authorization: Bearer $TOKEN" $URL/<endpoint>`

## 快速状态检查

无需加载子技能：
- `GET /healthz` — 健康检查（公开）
- `GET /metrics` — Prometheus 指标（公开）
- `GET /api/v1/health/score` — 综合安全健康分
- `GET /api/v1/overview/summary` — 首页概览聚合

## 子技能路由

根据用户意图加载对应子技能的 SKILL.md：

| 用户意图关键词 | 子技能 | 路径 |
|---------------|--------|------|
| 规则、入站、出站、审计日志、路由、绑定、限流、策略路由、IM、检测 | **IM 安全域** | `im-security/SKILL.md` |
| LLM、模型、Token、成本、Canary、泄露、Budget、预算、OWASP、LLM规则 | **LLM 安全域** | `llm-security/SKILL.md` |
| 画像、行为、攻击链、异常、基线、红队、Red Team、风险评分、用户风险 | **威胁分析** | `threat-analysis/SKILL.md` |
| 安全画像、安全评分、安全总览、上游画像、维度评分、Treemap | **安全画像** | `security-profile/SKILL.md` |
| 租户、报告、排行榜、SLA、蜜罐、A/B测试、会话回放、Prompt追踪 | **安全治理** | `governance/SKILL.md` |
| 备份、恢复、诊断、配置、严格模式、大屏、布局、通知、模拟、演示 | **系统运维** | `ops/SKILL.md` |
| 信封、Merkle、密码学、验证链 | **执行信封** | `envelope/SKILL.md` |
| 事件总线、Webhook、订阅、推送 | **事件总线** | `event-bus/SKILL.md` |
| 自进化、进化、变异、自动规则 | **自进化引擎** | `evolution/SKILL.md` |
| 语义检测、语义分析、模式库、ONNX | **语义检测** | `semantic/SKILL.md` |
| 奇点、暴露预算、欧拉、忠诚度 | **奇点蜜罐** | `singularity/SKILL.md` |
| tool_calls、工具策略、工具白名单 | **工具策略** | `tool-policy/SKILL.md` |
| 污染、taint、PII追踪、逆转 | **污染追踪** | `taint/SKILL.md` |
| 缓存、响应缓存、命中率 | **响应缓存** | `cache/SKILL.md` |
| 网关、JWT、灰度、路由转换 | **API网关** | `gateway/SKILL.md` |
| Gateway Monitor、上游网关、AOC、Agent操作中心、技能目录 | **API网关** | `gateway/SKILL.md` |
| 路径策略、path policy、序列规则、累计规则、降级、风险仪表 | **路径策略** | `path-policy/SKILL.md` |
| 反事实、counterfactual、对照验证、归因、AttriGuard | **反事实验证** | `counterfactual/SKILL.md` |
| 执行计划、plan compiler、CaMeL、意图匹配、tool_call序列 | **执行计划** | `plan-compiler/SKILL.md` |
| capability、权限标签、数据级权限、信任分 | **Capability** | `capability/SKILL.md` |
| 偏差检测、deviation、auto-repair、计划偏离 | **偏差检测** | `deviation/SKILL.md` |

## v29.0 Gateway 远程管理速查

### Gateway Remote Management / WSS RPC (v29.0)
- 持久化 WSS RPC：`challenge → connect → hello → ready`
- WSS 优先，失败自动 fallback 到 `POST /api/v1/tools/invoke`
- 新增/确认 P2 能力：`exec-approvals`、`restart`、`update`、`memory/search`、`skills/uninstall`
- 远程管理覆盖：sessions / chat / cron / agents / approvals / config / skills / restart / update / heartbeat / devices / nodes / memory / system
- Dashboard / AOC 子视图共 8 个：仪表盘、卡片、协作、用户、Skills、文件、心跳、记忆

## v22.x 新功能速查

### Gateway Monitor · 上游网关监控 / 远程管理 (v29)
- 优先走持久化 WSS RPC（challenge → connect → hello → ready），失败时自动 fallback 到 `tools/invoke`
- `GET /api/v1/gateway-monitor/upstreams` — 监控的上游 Gateway 列表
- `PUT /api/v1/gateway-monitor/upstreams/:id` — 配置上游 Gateway Token / Origin
- `GET /api/v1/gateway-monitor/upstreams/:id/aoc` — 上游 Agent Operations Center (AOC)
- `POST /api/v1/tools/invoke` — fallback 通道：调用上游 OpenClaw Gateway 的 tools/invoke 协议
- P2 新增能力已列出：`/gateway/exec-approvals`、`/gateway/restart`、`/gateway/update`、`/gateway/memory/search`、`/gateway/skills/uninstall`

### Agent Operations Center · AOC (v29)
- `GET /api/v1/aoc/dashboard` — 仪表盘
- `GET /api/v1/aoc/cards` — 卡片视图
- `GET /api/v1/aoc/collab` — 协作视图
- `GET /api/v1/aoc/users` — 用户视图
- `GET /api/v1/aoc/skills` — Skills 视图
- `GET /api/v1/aoc/files` — 文件视图
- `GET /api/v1/aoc/heartbeat` — 心跳视图
- `GET /api/v1/aoc/memory` — 记忆视图

## Phase 1 新功能速查 (v18-v20)

### 执行信封 · 密码学审计 (v18)
- `GET /api/v1/envelopes` — 信封列表
- `GET /api/v1/envelopes/:id/verify` — 验证信封
- `GET /api/v1/envelopes/merkle/batches` — Merkle 批次

### 事件总线 (v18)
- `GET /api/v1/events` — 事件列表
- `GET /api/v1/events/subscriptions` — Webhook 订阅
- `POST /api/v1/events/subscriptions` — 添加订阅
- `POST /api/v1/events/test` — 测试推送

### 自进化引擎 (v19)
- `GET /api/v1/evolution/status` — 进化状态
- `POST /api/v1/evolution/run` — 手动触发进化
- `GET /api/v1/evolution/log` — 进化日志
- `GET /api/v1/evolution/generated-rules` — 自动生成的规则

### 语义检测 (v19)
- `GET /api/v1/semantic/status` — 引擎状态
- `POST /api/v1/semantic/analyze` — 分析文本
- `GET /api/v1/semantic/patterns` — 模式库（47 模式）

### 奇点蜜罐 (v18/v19)
- `GET /api/v1/singularity/budget` — 奇点预算
- `GET /api/v1/singularity/map` — 拓扑图
- `GET /api/v1/honeypot/loyalty` — 忠诚度排行

### 工具策略 · tool_calls (v20)
- `GET /api/v1/tool-policies` — 策略列表（18 条内置）
- `POST /api/v1/tool-policies` — 添加策略
- `POST /api/v1/tool-policies/evaluate` — 评估 tool_calls
- `GET /api/v1/tool-policies/events` — 事件日志

### 污染追踪 (v20)
- `POST /api/v1/taint/scan` — PII 扫描
- `GET /api/v1/taint/trace/:id` — 查看污染链
- `GET /api/v1/taint/propagation` — 传播记录
- `POST /api/v1/taint/reverse` — 触发逆转

### 响应缓存 (v20)
- `GET /api/v1/cache/status` — 命中率统计
- `GET /api/v1/cache/entries` — 缓存条目
- `POST /api/v1/cache/query` — 语义查询匹配
- `POST /api/v1/cache/clear` — 清空缓存

### API 网关 (v20)
- `GET /api/v1/gateway/status` — 网关状态
- `GET /api/v1/gateway/routes` — 路由列表
- `POST /api/v1/gateway/jwt/validate` — JWT 校验
- `GET /api/v1/gateway/config` — 网关配置

## Phase 2 新功能速查 (v23-v25)

### 路径级策略引擎 (v23)
- `GET /api/v1/path-policies` — 路径策略列表
- `POST /api/v1/path-policies` — 添加策略规则
- `GET /api/v1/path-policies/stats` — 路径统计
- `GET /api/v1/path-policies/risk-gauge` — 实时风险仪表
- `GET /api/v1/path-policies/templates` — 策略模板（含 AI Act 合规）
- `POST /api/v1/path-policies/templates` — 创建策略模板

### 反事实验证 (v24)
- `GET /api/v1/counterfactual/reports` — 验证报告
- `GET /api/v1/counterfactual/cost` — 成本追踪
- `GET /api/v1/counterfactual/effectiveness` — 效果评估
- `GET /api/v1/counterfactual/adaptive-config` — 自适应配置
- `PUT /api/v1/counterfactual/adaptive-config` — 更新自适应配置

### 执行计划编译器 · CaMeL (v25.0)
- `POST /api/v1/plans/compile` — 编译用户意图为执行计划
- `POST /api/v1/plans/evaluate` — 评估 tool_call 是否符合计划
- `GET /api/v1/plans/templates` — 模板列表（22 个，6 分类）
- `POST /api/v1/plans/templates` — 创建自定义模板
- `GET /api/v1/plans/active` — 活跃计划
- `GET /api/v1/plans/violations` — 违规记录
- `GET /api/v1/plans/stats` — 编译器统计

### Capability 权限系统 (v25.1)
- `GET /api/v1/capabilities/mappings` — 工具→权限映射（17 条）
- `PUT /api/v1/capabilities/mappings/:tool` — 更新映射
- `POST /api/v1/capabilities/contexts` — 初始化权限上下文
- `GET /api/v1/capabilities/evaluations` — 权限评估记录
- `GET /api/v1/capabilities/stats` — 权限统计

### 偏差检测 (v25.2)
- `POST /api/v1/deviations/check` — 检查 tool_call 是否偏离计划
- `GET /api/v1/deviations` — 偏差记录
- `GET /api/v1/deviations/stats` — 偏差统计
- `GET /api/v1/deviations/config` — 检测器配置
- `PUT /api/v1/deviations/config` — 更新配置

## 调用流程

1. 识别用户意图 → 匹配上表关键词
2. `read` 对应子技能 SKILL.md
3. 按子技能中的 API 参考执行操作
4. 如果涉及多个域（如"全面安全报告"），依次加载相关子技能

## CLI 工具

`lobster-cli.sh` 提供命令行快捷方式，覆盖常用操作（含 Phase 1 全部命令 + Gateway Monitor / Gateway WSS RPC 远程管理）：

```bash
./lobster-cli.sh help          # 查看所有命令
./lobster-cli.sh status        # 健康检查
./lobster-cli.sh report        # 综合安全报告
./lobster-cli.sh evolution-run # 手动触发自进化
./lobster-cli.sh taint-scan "test text"  # PII 扫描
./lobster-cli.sh cache-status  # 缓存命中率
```

## 认证方式

- **Bearer Token**: `Authorization: Bearer <management_token>`（简单模式）
- **JWT**: 先 `POST /api/v1/auth/login` 获取 token，后续请求带上（Dashboard 登录模式）

## 版本历史

- **v33.0** — Upstream 安全画像：per-upstream 5 维安全评分（入站/LLM/数据/行为/工具，各 20 分）；16 引擎 × 14 表聚合；比率评分（偏离率/拒绝率/失败率）+ 正面信号加分；Treemap（面积=用户数，颜色=评分）+ 甜甜圈 + 5 档分段 + Canvas 粒子系统；三级穿透（概览→排名→详情）
- **v32.0** — 架构优化 + Dashboard 演示级打磨：全链路检测调试 API；SQLite 批量写入 + 监控；三级配置简化 + 向导；43 页面全量巡检；骨架屏/Toast/fade 过渡；红队规则建议 + Merkle 批量验证 + 金丝雀轮换 + 报告定时 + 引擎开关
- **v31.0** — 统一行业模板：40 模板 × 12 分类；入站 + LLM + 出站三维度一键启用；Benchmark F1=1.0（500 样本）；auto-review LLM 二审
- **v30.0** — 质量硬化：模板系统统一；api.go 拆分（9744→1551 行 + 17 域文件）；AC 智能分级；检测基准测试
- **v29.0** — Gateway WSS RPC 远程管理：从 `POST /tools/invoke` 升级为持久化 WSS RPC，复刻 OpenClaw Control UI 协议（challenge → connect → hello → ready），WSS 优先 / `tools/invoke` 自动 fallback；55 个 Gateway 管理端点；GatewayMonitor 升级为 Sessions / Cron / Diag / Agent 全功能远程运维页面；新增 `default_gateway_origin` / `gateway_origin` Origin 配置
- **v25.2** — CaMeL 全链路：PlanCompiler（意图→模板，20+ 内置，中文 i18n）+ CapabilityEngine（数据级权限标签）+ DeviationDetector（偏差检测 + 自动修复）+ proxy hooks（入站+LLM双向拦截）+ 4 个管理 API + PathPolicy（路径级治理）+ Counterfactual（反事实验证）。85 Go 源文件 ~86K 行、72 Vue 文件 ~27K 行、1151 测试、~256 API、46 页面、242 commits、5 依赖
- **v22.4** — Gateway Monitor（上游 OpenClaw Gateway 监控 + tools/invoke 协议）、Agent Operations Center（5 视图）、Per-Upstream AOC、Skill Directory、SVG 图标系统
- **v20.6** — Phase 1 完成：执行信封 + 事件总线 + 自进化 + 语义检测 + 奇点蜜罐 + 工具策略 + 污染追踪 + 响应缓存 + API网关
- **v17.1** — 态势感知大屏 + 可拖拽布局 + 29 页面
