#!/usr/bin/env python3
"""Generate README.md for lobster-guard"""

parts = []

# Part 1: Header
parts.append("""<p align="center">
  <br>
  <span style="font-size:64px">🦞</span>
  <br>
</p>

<h1 align="center">lobster-guard（龙虾卫士）</h1>

<p align="center">
  <strong>AI Agent 安全网关 · 一个二进制搞定 IM + LLM 双安全域</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-17.1+v18-00d4ff?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/language-Go_1.21-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/frontend-Vue_3.5-4FC08D?style=flat-square&logo=vue.js" alt="Vue">
  <img src="https://img.shields.io/badge/database-SQLite_WAL-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/binary-single_file-00ff88?style=flat-square" alt="Single Binary">
  <img src="https://img.shields.io/badge/API-117_endpoints-ff6688?style=flat-square" alt="117 APIs">
  <img src="https://img.shields.io/badge/tests-648_cases-brightgreen?style=flat-square" alt="648 Tests">
  <img src="https://img.shields.io/badge/deps-3_only-yellow?style=flat-square" alt="3 Dependencies">
  <img src="https://img.shields.io/badge/license-MIT-yellow?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Protecting your upstream, one lobster at a time. 🦞</em>
</p>

---

## 🎯 这是什么

**lobster-guard** 是一个面向 AI Agent 的安全网关，专为 [OpenClaw](https://github.com/openclaw/openclaw) 等 AI Agent 平台设计。它同时守护 **IM 消息通道**和 **LLM API 通道**两个安全域——用户的消息进来之前先安检，Agent 的回复出去之前再安检，LLM 调用也全程审计拦截。

**一个二进制，42 个 Go 源文件编译，go:embed 内嵌 Vue 3 前端，扔上去就跑。**

支持蓝信、飞书、钉钉、企业微信、通用 HTTP 五种消息通道，通过插件机制一行配置切换。""")

# Part 2: Core features
parts.append("""---

## ✨ 核心特性

### 🛡️ IM 安全

| 能力 | 说明 |
|------|------|
| AC 自动机检测 | O(n) 多模式匹配，40+ 内置关键词规则 |
| 正则规则引擎 | 自定义正则 + 优先级权重 + 命中率统计 |
| DetectPipeline | Keyword → Regex → PII → Session → LLM 五阶段检测链 |
| 上下文感知 | 会话级风险积分 · 多轮攻击识别 · warn → block 自动升级 |
| 出站拦截 | 防 Agent 泄露身份证号/API Key/私钥等 · PII 正则可配 |
| 规则模板库 | 4 场景 66 条规则（通用/金融/医疗/政务）· go:embed 内置 |
| 规则分组绑定 | 按 app_id 绑定不同规则组 · 多租户规则隔离 |

### 🤖 LLM 安全

| 能力 | 说明 |
|------|------|
| LLM 反向代理 | 独立端口 :8445 · 多 target 负载 · SSE 流式拦截 |
| LLM 规则引擎 | 独立规则集 · keyword/regex · log/warn/block/rewrite · Shadow Mode |
| Canary Token | 自动注入追踪令牌 · Prompt 泄露实时检测 |
| Response Budget | Agent 行为预算 · Token/调用次数限额 · 超限告警 |
| 成本管控 | 模型定价 · 每日成本上限 · 超限 Webhook 告警 |
| OWASP LLM Top10 | 攻击矩阵视图 · 规则自动映射到 LLM01-LLM10 |
| Prompt 追踪 | SHA256 变化检测 · LCS 行级 diff · 安全指标关联分析 |

### 🔍 威胁分析

| 能力 | 说明 |
|------|------|
| 攻击链分析 | 跨 Agent 协同攻击模式发现 · 事件关联 · 时间线可视化 |
| 行为画像 | Agent 正常行为学习 · 5 类突变检测 · 行为序列风险评分 |
| 异常检测 | 6 指标 · 7 天滑动窗口 · 2σ/3σ 自动告警 · 零外部 ML 依赖 |
| Agent 蜜罐 | 8 预置模板 · 水印追踪 · 假数据引爆 · 攻击者画像关联 |
| Red Team Autopilot | 33 攻击向量 · 6 个 OWASP 分类 · 自动攻防测试+修复建议 |

### 📊 安全治理

| 能力 | 说明 |
|------|------|
| 安全排行榜 | 跨租户评分对比 · 热力图 · SLA 三档判定 |
| A/B 测试 | Prompt 双版本流量分配 · 指标对比 · 统计显著性检验 |
| 合规报告 | 日报/周报/月报 · HTML 邮件友好 · 智能建议 |
| 多租户 | 安全域隔离 · 6 表 tenant_id · 向后兼容 default 租户 |

### ⚙️ 系统管理

| 能力 | 说明 |
|------|------|
| 单二进制部署 | go:embed 前端 · 42 个源文件编译为一个 ~15MB 二进制 |
| 零外部依赖 | 仅 sqlite3 + yaml.v3 + gorilla/websocket 三个依赖 |
| 多通道插件 | 蓝信/飞书/钉钉/企微/通用 HTTP · Bridge WSS 长连接 |
| 多 Bot 亲和路由 | (用户ID, BotID) 复合键 · 邮箱/部门策略路由 · 批量绑定 |
| WebSocket 代理 | Agent streaming 实时安全扫描 · inspect/passthrough 模式 |
| 全量审计 | SQLite WAL · CSV/JSON 导出 · 自动归档 · 时间线图 · 全文搜索 |
| 会话回放 | trace_id 全链路追踪 · 决策过程可视化 · 运维标签 |
| 态势感知大屏 | 4K 适配 · 数字滚动 · 弹幕事件流 · 可拖拽布局 |
| 登录认证 | JWT HS256 · admin/operator/viewer 三角色 · 操作审计 |
| 高可用 | 优雅关闭 · 5 维健康检查 · 数据备份/恢复 · Store 抽象层 |""")

# Part 3: Architecture
parts.append("""---

## 🏛️ 架构

```
                           ┌──────────────────────────────────────────────┐
                           │              lobster-guard 🦞               │
                           │                                              │
                           │   ┌─────────── IM 安全域 ───────────┐       │
  消息平台                  │   │                                  │       │
  (蓝信/飞书/钉钉/企微) ──────►│ :8443 入站代理 ──► 检测引擎 ──────────────►│ Agent
                           │   │  Channel Plugin    AC+正则+PII   │       │  ×N
                           │   │                    +Session+LLM  │       │
                           │   └──────────────────────────────────┘       │
                           │                                              │
                           │   ┌─────────── IM 出站 ─────────────┐       │
  消息平台 API ◄───────────────│ :8444 出站代理 ◄── 出站规则引擎  │◄──────│
                           │   │  PII/凭据/命令     block/warn   │       │
                           │   └──────────────────────────────────┘       │
                           │                                              │
                           │   ┌─────────── LLM 安全域 ──────────┐       │
  LLM Provider             │   │                                  │       │
  (OpenAI/Claude/…) ◄─────────│ :8445 LLM 代理 ◄── LLM 规则引擎 │◄──────│
                           │   │  多target·SSE流式  Canary Token  │       │
                           │   │                    Budget控制    │       │
                           │   └──────────────────────────────────┘       │
                           │                                              │
                           │   ┌─────────── Dashboard ───────────┐       │
  浏览器 ─────────────────────►│ :9090 管理 API + Vue 3 前端      │       │
                           │   │  117 API · 29 页面 · 19 组件    │       │
                           │   │  SQLite WAL · Prometheus 指标    │       │
                           │   └──────────────────────────────────┘       │
                           └──────────────────────────────────────────────┘
```

### 四个端口，各司其职

| 端口 | 用途 | 方向 |
|------|------|------|
| `:8443` | IM 入站代理 | 消息平台 → Agent |
| `:8444` | IM 出站代理 | Agent → 消息平台 API |
| `:8445` | LLM 反向代理 | Agent → LLM Provider |
| `:9090` | Dashboard + 管理 API | 浏览器 / API 调用 |""")

# Part 4: Quick start
parts.append("""---

## 🚀 快速开始

### 1. 编译

```bash
git clone https://github.com/zhuowater/lobster-guard.git
cd lobster-guard

# 编译（需要 Go 1.21+ 和 GCC）
CGO_ENABLED=1 go build -o lobster-guard .
```

编译产物是一个约 15MB 的单二进制文件，前端已通过 go:embed 内嵌。

### 2. 配置

```bash
cp config.yaml.example config.yaml
vim config.yaml
```

**最小配置：**

```yaml
# 消息通道
channel: "lanxin"                    # lanxin | feishu | dingtalk | wecom | generic
callbackKey: "你的回调加密密钥"
callbackSignToken: "你的签名令牌"

# IM 代理
inbound_listen: ":8443"
outbound_listen: ":8444"
openclaw_upstream: "http://127.0.0.1:18790"
lanxin_upstream: "https://apigw.lx.qianxin.com"

# LLM 代理
llm_proxy:
  enabled: true
  listen: ":8445"
  targets:
    - name: "openai"
      url: "https://api.openai.com"

# 管理
management_listen: ":9090"
management_token: "your-secret-token"

# 安全检测
inbound_detect_enabled: true
outbound_audit_enabled: true
db_path: "./audit.db"
```

### 3. 运行

```bash
./lobster-guard -config config.yaml

# 验证
curl http://localhost:9090/healthz
open http://localhost:9090/
```

### CLI 工具

```bash
./lobster-guard -version              # 打印版本号
./lobster-guard -check-config         # 验证配置文件
./lobster-guard -dump-routes          # 打印路由表
./lobster-guard -dump-rules           # 打印规则
./lobster-guard -gen-rules rules.yaml # 导出默认规则
./lobster-guard -restore backup.db    # 从备份恢复
```""")

# Part 5: Channel plugins
parts.append("""---

## 🔌 Channel 插件

通过 Channel Plugin 机制，一行配置切换消息平台：

```yaml
channel: "feishu"    # lanxin | feishu | dingtalk | wecom | generic
```

| 通道 | 入站加密 | 消息格式 | Webhook | Bridge | 状态 |
|------|---------|---------|---------|--------|------|
| 🔵 **蓝信** | AES-256-CBC + SHA1 | JSON | ✅ | ❌ | ✅ 生产可用 |
| 🟢 **飞书** | AES-256-CBC + SHA256 | JSON | ✅ | ✅ WSS | ✅ 已实现 |
| 🔷 **钉钉** | AES-256-CBC + HMAC-SHA256 | JSON | ✅ | ✅ Stream | ✅ 已实现 |
| 🟠 **企微** | AES-256-CBC + SHA1 | XML入/JSON出 | ✅ | ❌ | ✅ 已实现 |
| ⚪ **通用 HTTP** | 无加密 | JSON（字段可配） | ✅ | ❌ | ✅ 已实现 |

### Bridge Mode（长连接桥接）

飞书和钉钉支持 WebSocket 长连接模式（**无需公网 IP**）。lobster-guard 主动连接平台拉取消息，安检后转为 Webhook 格式推给 Agent——对 Agent 完全透明。

```yaml
channel: "feishu"
mode: "bridge"       # 加这一行即可
feishu_app_id: "cli_xxx"
feishu_app_secret: "xxx"
```

```
Webhook 模式:   平台 ──POST──► :8443 ──► 安检 ──► Agent
Bridge 模式:    lobster-guard ══WSS══► 平台（拉消息）──► 安检 ──► POST Agent
```

Bridge 特性：🔄 自动重连（指数退避 1s→60s）· 🔑 Token 自动刷新 · 💓 心跳保活 · 📊 状态监控""")

# Part 6: Dashboard
parts.append("""---

## 🖥️ Dashboard

访问 `http://your-server:9090/` 打开管理后台。Vue 3 + Vite 构建，go:embed 嵌入二进制，零外部依赖。

### 29 个页面，5 组分类

| 分组 | 页面 | 说明 |
|------|------|------|
| **🛡️ IM 安全** | Overview | 综合概览 · 安全健康分 · 系统指标 · 通知中心 |
| | Audit | 审计日志 · 多维筛选 · 24h 时间线 · CSV/JSON 导出 |
| | Rules | 入站/出站规则 CRUD · 正则测试器 · 热更新 · YAML 导入导出 |
| | Routes | 亲和路由 · 策略路由 · 批量绑定 · 路由统计 |
| | Monitor | 实时监控 · QPS 柱状图 · 攻击实时流 |
| **🤖 LLM 安全** | LLMOverview | LLM 概览 · OWASP 矩阵 · 成本看板 · 工具调用 |
| | LLMRules | LLM 规则管理 · Shadow Mode · 命中统计 |
| | PromptTracker | Prompt 版本追踪 · SHA256 diff · 安全指标关联 |
| | AgentBehavior | Agent 行为画像 · 工具调用分布 · Token 趋势 |
| | Settings | LLM 配置 · Canary Token · Response Budget |
| | SessionDetail | 会话详情 · 请求/响应对比 · 风险标注 |
| **🔍 威胁分析** | AttackChain | 攻击链可视化 · 跨 Agent 关联 · 时间线 |
| | BehaviorProfile | Agent 行为画像 · 5 类突变检测 · 风险评分 |
| | AnomalyDetection | 异常检测 · 基线图 · ±2σ 带 · 自动告警 |
| | Honeypot | 蜜罐管理 · 模板编辑 · 触发记录 |
| | RedTeam | 红队测试 · 攻击成功率 · 漏洞列表 · 修复建议 |
| **📊 安全治理** | Leaderboard | 安全排行榜 · 热力图 · SLA 基线 |
| | ABTesting | A/B 测试 · 双版本对比 · 统计显著性 |
| | Reports | 安全报告 · 日/周/月报 · HTML 预览 |
| | Tenants | 租户管理 · 安全域隔离 · 成员 · 策略 |
| **⚙️ 系统** | Operations | 运维工具箱 · 配置查看 · 备份 · 诊断 · 告警 |
| | Users | 用户列表 · 风险统计 · 批量刷新 |
| | UserProfiles | 攻击者画像 · 风险 TOP10 · 5 维度评分 |
| | UserDetail | 用户详情 · 圆环评分 · 行为时间线 |
| | BigScreen | 态势感知全屏 · 8 面板 · 弹幕事件流 · 4K |
| | CustomDashboard | 可拖拽布局 · 4 预设模板 · 布局持久化 |
| | Login | JWT 登录 · 角色鉴权 |
| | SessionReplay | 会话回放 · trace_id 追踪 · 决策可视化 |
| | Upstream | 上游容器管理 · 健康状态 · 故障转移 |

### 19 个通用组件

`BindModal` · `ConfirmModal` · `DataTable` · `DraggableGrid` · `EmptyState` · `HeatMap` · `Icon` · `JsonHighlight` · `Loading` · `PieChart` · `RegexTester` · `RuleEditor` · `Sidebar` · `Skeleton` · `StatCard` · `Toast` · `Topbar` · `TrendChart` · `UpstreamSelect`""")

# Part 7: API Reference
parts.append("""---

## 📡 API 参考（117 个端点）

所有 `/api/v1/*` 管理接口需携带 `Authorization: Bearer <management_token>` 请求头。

### 🛡️ IM 安全

<details>
<summary>概览与统计（5 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/overview/summary` | 综合概览 |
| GET | `/api/v1/stats` | 基础统计 |
| GET | `/api/v1/health/score` | 安全健康分 |
| GET | `/api/v1/metrics/realtime` | 逐秒实时统计 |
| GET | `/api/v1/notifications` | 通知中心 |

</details>

<details>
<summary>规则管理（19 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/inbound-rules` | 入站规则列表 |
| POST | `/api/v1/inbound-rules/add` | 添加入站规则 |
| PUT | `/api/v1/inbound-rules/update` | 更新入站规则 |
| DELETE | `/api/v1/inbound-rules/delete` | 删除入站规则 |
| POST | `/api/v1/inbound-rules/reload` | 热更新入站规则 |
| GET | `/api/v1/outbound-rules` | 出站规则列表 |
| POST | `/api/v1/outbound-rules/add` | 添加出站规则 |
| PUT | `/api/v1/outbound-rules/update` | 更新出站规则 |
| DELETE | `/api/v1/outbound-rules/delete` | 删除出站规则 |
| POST | `/api/v1/outbound-rules/reload` | 热更新出站规则 |
| GET | `/api/v1/rules/hits` | 命中率排行 |
| POST | `/api/v1/rules/hits/reset` | 重置命中统计 |
| GET | `/api/v1/rules/export` | 导出规则 YAML |
| POST | `/api/v1/rules/import` | 导入规则 |
| POST | `/api/v1/rules/reload` | 热更新全部规则 |
| GET | `/api/v1/rule-templates` | 规则模板列表 |
| GET | `/api/v1/rule-templates/detail` | 模板详情 |
| GET | `/api/v1/rule-bindings` | 规则组绑定关系 |
| POST | `/api/v1/rule-bindings/test` | 测试绑定匹配 |

</details>

<details>
<summary>审计日志（7 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/audit/logs` | 日志查询（多维筛选 + 全文搜索）|
| GET | `/api/v1/audit/export` | 导出（CSV/JSON · from/to 时间范围）|
| GET | `/api/v1/audit/stats` | 审计统计 |
| GET | `/api/v1/audit/timeline` | 时间线（按小时聚合）|
| POST | `/api/v1/audit/archive` | 手动归档 |
| GET | `/api/v1/audit/archives` | 归档列表 |
| POST | `/api/v1/audit/cleanup` | 清理过期日志 |

</details>

<details>
<summary>路由与会话（12 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/routes` | 路由绑定列表 |
| POST | `/api/v1/routes/bind` | 绑定用户到上游 |
| POST | `/api/v1/routes/unbind` | 解绑路由 |
| POST | `/api/v1/routes/migrate` | 迁移用户 |
| POST | `/api/v1/routes/batch-bind` | 批量绑定 |
| GET | `/api/v1/routes/stats` | 路由统计 |
| GET/POST/PUT/DELETE | `/api/v1/route-policies` | 策略路由 CRUD |
| POST | `/api/v1/route-policies/test` | 测试策略匹配 |
| GET | `/api/v1/sessions/risks` | 高风险会话 |
| POST | `/api/v1/sessions/risks/reset` | 重置风险积分 |

</details>

<details>
<summary>限流与告警（4 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/rate-limit/stats` | 限流统计 |
| POST | `/api/v1/rate-limit/reset` | 重置限流计数器 |
| GET | `/api/v1/alerts/config` | 告警配置 |
| GET | `/api/v1/alerts/history` | 告警历史 |

</details>

### 🤖 LLM 安全

<details>
<summary>LLM 代理与审计（21 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/llm/overview` | LLM 概览 |
| GET | `/api/v1/llm/status` | 代理状态 |
| GET/PUT | `/api/v1/llm/config` | LLM 配置读写 |
| GET | `/api/v1/llm/calls` | 调用记录 |
| GET | `/api/v1/llm/export` | 导出 LLM 审计 |
| GET/PUT | `/api/v1/llm/rules` | LLM 规则读写 |
| GET | `/api/v1/llm/rules/hits` | LLM 规则命中 |
| GET | `/api/v1/llm/owasp-matrix` | OWASP Top10 矩阵 |
| GET | `/api/v1/llm/canary/status` | Canary 状态 |
| POST | `/api/v1/llm/canary/rotate` | 轮换 Canary Token |
| GET | `/api/v1/llm/canary/leaks` | 泄露记录 |
| GET | `/api/v1/llm/budget/status` | Budget 状态 |
| GET | `/api/v1/llm/budget/violations` | 预算违规 |
| GET | `/api/v1/llm/tools` | Agent 工具列表 |
| GET | `/api/v1/llm/tools/stats` | 工具统计 |
| GET | `/api/v1/llm/tools/timeline` | 工具时间线 |
| GET | `/api/v1/prompts` | Prompt 版本列表 |
| GET | `/api/v1/prompts/current` | 当前 Prompt |
| GET | `/api/v1/sessions/replay` | 会话回放列表 |
| GET | `/api/v1/sessions/replay/{trace_id}` | 单个会话回放 |

</details>

### 🔍 威胁分析

<details>
<summary>攻击链 + 行为 + 异常 + 蜜罐 + 红队（18 个）</summary>

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/attack-chains` | 攻击链列表 |
| POST | `/api/v1/attack-chains/analyze` | 触发分析 |
| GET | `/api/v1/attack-chains/patterns` | 攻击模式 |
| GET | `/api/v1/attack-chains/stats` | 攻击链统计 |
| GET | `/api/v1/behavior/profiles` | 行为画像 |
| GET | `/api/v1/behavior/patterns` | 行为模式 |
| GET | `/api/v1/behavior/anomalies` | 行为异常 |
| GET | `/api/v1/anomaly/status` | 异常检测状态 |
| GET/PUT | `/api/v1/anomaly/config` | 异常检测配置 |
| GET | `/api/v1/anomaly/baselines` | 基线数据 |
| GET | `/api/v1/anomaly/alerts` | 异常告警 |
| GET | `/api/v1/honeypot/stats` | 蜜罐统计 |
| GET/POST/PUT/DELETE | `/api/v1/honeypot/templates` | 蜜罐模板 CRUD |
| POST | `/api/v1/honeypot/test` | 测试蜜罐 |
| GET | `/