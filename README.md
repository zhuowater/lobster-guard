<p align="center">
  <br>
  <span style="font-size:64px">🦞</span>
  <br>
</p>

<h1 align="center">lobster-guard（龙虾卫士）</h1>

<p align="center">
  <strong>AI Agent 安全网关 · 双安全域 · 入站检测 · 出站拦截 · LLM 审计 · 态势感知</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v17.1.0-00d4ff?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/database-SQLite-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/binary-single_file-00ff88?style=flat-square" alt="Single Binary">
  <img src="https://img.shields.io/badge/channels-5_platforms-ff6688?style=flat-square" alt="5 Channels">
  <img src="https://img.shields.io/badge/tests-754_passed-brightgreen?style=flat-square" alt="Tests">
  <img src="https://img.shields.io/badge/API-227_routes-purple?style=flat-square" alt="API Routes">
  <img src="https://img.shields.io/badge/dashboard-29_pages-orange?style=flat-square" alt="Dashboard">
  <img src="https://img.shields.io/badge/license-MIT-yellow?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Protecting your upstream, one lobster at a time.</em>
</p>

---

## 🎯 这是什么

**lobster-guard** 是一个全功能 AI Agent 安全网关，专为 AI Agent（如 [OpenClaw](https://github.com/openclaw/openclaw)）设计。它部署在消息平台/LLM 服务与 AI Agent 之间，提供 **IM 安全域 + LLM 安全域**双安全域防护，覆盖入站检测、出站拦截、LLM 审计、行为分析、攻击链检测、蜜罐诱捕和态势感知全链路。

**支持蓝信、飞书、钉钉、企业微信等 5 种消息通道，通过插件机制一行配置切换。**

**一句话：IM 消息进来之前先安检，Agent 回复出去之前再安检，LLM API 调用全程审计。**

### ✨ 核心能力

| 能力 | 说明 |
|------|------|
| 🛡️ **智能检测** | AC 自动机 + 正则 + 检测链 Pipeline + LLM 语义分析 · 4 场景规则模板库（66 条）|
| 🧠 **上下文感知** | 会话级风险积分 · 多轮对话攻击识别 · 自动升级（warn → block）|
| 🔒 **出站拦截** | 默认 6 条规则（PII/凭据/命令）· 用户自定义 · v18 智能合并 |
| 🔀 **多 Bot 亲和路由** | 按 (用户ID, BotID) 复合键绑定 · 邮箱/部门策略路由 · IM 用户信息自动获取 |
| 🌐 **WebSocket 代理** | Agent 实时 streaming 安全扫描 · inspect/passthrough 模式 · 帧级检测 |
| 🔌 **多通道插件** | 蓝信/飞书/钉钉/企微/通用HTTP · Bridge Mode WSS 长连接 |
| 🤖 **LLM 安全域** | LLM 反向代理（:8445）· 工具调用审计 · Token 成本看板 · 11 条默认规则 |
| 🎯 **Canary Token** | Prompt 泄露检测 · Shadow Mode · 自动轮换 |
| 🔴 **Red Team Autopilot** | 33 攻击向量自动化测试 · 安全排行榜 · SLA 达成率 |
| 🍯 **Agent 蜜罐** | 8 模板诱捕 · 水印追踪 · 引爆检测 · 集成代理数据流 |
| 🧬 **行为画像** | 特征提取 · 模式学习 · 异常基线检测 · 攻击者画像 |
| 🔗 **攻击链检测** | 多阶段关联分析 · Kill Chain 映射 · 自动升级策略 |
| 📊 **态势感知大屏** | 实时攻防态势 · 可拖拽布局 · 4 预设模板 · 全屏驾驶舱 |
| 📋 **报告引擎** | 安全日报/周报 · 合规审计报告 · PDF 导出 |
| 🎭 **Prompt A/B 测试** | 多版本对比 · 效果量化 · 自动推荐 |
| 📡 **会话回放** | 时间线 · 标签 · 搜索 · Prompt 版本追踪 |
| 👥 **多租户** | JWT 登录认证 · 租户隔离 · 用户管理 CRUD |
| 📈 **可观测性** | Prometheus 指标 · 结构化日志 · trace_id 全链路追踪 |
| 🖥️ **管理后台** | Vue 3 Dashboard · 29 页面 · 19 组件 · 5 组侧边栏 |
| 💾 **高可用** | 优雅关闭 · 5 维健康检查 · 数据备份/恢复 · Store 抽象层 |
| ⚡ **限流** | 令牌桶 · 全局/每用户 · 白名单 · 429 + Retry-After |

### 🏗️ 设计哲学

- **单二进制部署** — 42 个源文件编译出一个二进制（含 Dashboard + 规则模板），扔上去就跑
- **Fail-Open** — 检测异常不阻塞业务，宁可漏检不可误杀
- **零外部依赖** — 只依赖 SQLite + YAML 解析 + WebSocket + x/crypto（4 个依赖），不引入 Redis/MQ/K8s
- **向后兼容** — 不配多容器就自动退化为单上游模式，平滑升级

### 📊 项目统计

| 指标 | 数值 |
|------|------|
| Go 源文件 | 42 个 |
| Go 代码行数 | ~48,500 行（含测试 18,200 行）|
| Vue 前端 | 29 个页面 + 19 个组件，~14,800 行 |
| API 路由 | ~227 个 |
| 测试函数 | 754 个 |
| Git Commit | 106 个 |
| 外部依赖 | 4 个（sqlite3 + yaml.v3 + gorilla/websocket + x/crypto）|

---

## 📸 界面预览

### 管理后台全览

> Vue 3 深色科技主题 · Indigo 配色 · 29 页面 · 5 组侧边栏

![Dashboard Full](docs/screenshots/dashboard-full.jpg)

### Dashboard 侧边栏导航（5 组）

| 分组 | 页面 | 功能 |
|------|------|------|
| 🛡️ **IM 安全** | Overview / Rules / Audit / Monitor / Routes | IM 入站出站安全管控 |
| 🤖 **LLM 安全** | LLMOverview / LLMRules / PromptTracker / Honeypot / ABTesting / SessionReplay | LLM 代理审计与检测 |
| 🔍 **威胁分析** | UserProfiles / BehaviorProfile / AttackChain / AnomalyDetection / RedTeam | 高级威胁分析与画像 |
| 📋 **安全治理** | Reports / Leaderboard / Tenants / Settings | 报告、排行榜、租户管理 |
| ⚙️ **系统** | Operations / Upstream / Users / BigScreen | 运维工具箱、态势大屏 |

*附加页面：CustomDashboard（自定义布局）/ Login / UserDetail / SessionDetail*

### 安全拦截一览

> Block 红色高亮 · Warn 黄色高亮 · 支持方向/动作/发送者多维筛选

![Dashboard Block Filter](docs/screenshots/dashboard-block-filter.jpg)

### 启动画面

```
  _         _         _                                         _
 | |   ___ | |__  ___| |_ ___ _ __       __ _ _   _  __ _ _ __| |
 | |  / _ \| '_ \/ __| __/ _ \ '__|____ / _' | | | |/ _' | '__| |
 | |_| (_) | |_) \__ \ ||  __/ | |_____| (_| | |_| | (_| | |  | |_
 |___|\___/|_.__/|___/\__\___|_|        \__, |\__,_|\__,_|_|  |___|
                                         |___/
        龙虾卫士 - AI Agent 安全网关 v17.1.0
        双安全域 | IM检测 | LLM审计 | 态势感知 | 蜜罐 | Red Team

┌─────────────────────────────────────────────────┐
│              配置摘要 v17.1.0                    │
├─────────────────────────────────────────────────┤
│ 消息通道:    lanxin                             │
│ 接入模式:    webhook                            │
│ IM 入站:     :18443                             │
│ IM 出站:     :18444                             │
│ LLM 代理:    :8445                              │
│ Dashboard:   :9090                              │
│ 入站检测:    true                               │
│ 出站审计:    true                               │
│ 入站规则:    40 patterns (内置默认)              │
│ 出站规则:    6 (默认) + 用户自定义               │
│ LLM 规则:    11 (默认) + 用户自定义              │
│ 路由策略:    least-users                        │
│ 限流:        100 rps (全局) / 5 rps (每用户)    │
│ Metrics:     :9090/metrics (Prometheus)          │
│ 蜜罐:        8 模板 + 水印追踪                   │
│ Red Team:    33 攻击向量                         │
│ 多租户:      JWT 认证 + 租户隔离                 │
│ 大屏:        4 预设模板 + 自定义布局              │
└─────────────────────────────────────────────────┘
```

---

## 🔌 多通道支持（v3.0）

通过 Channel Plugin 机制，一行配置切换消息平台：

```yaml
channel: "feishu"    # lanxin | feishu | dingtalk | wecom | generic
```

| 通道 | 入站加密 | 消息格式 | 出站审计路径 | 状态 |
|------|---------|---------|------------|------|
| 🔵 **蓝信** (LanXin) | AES-256-CBC + SHA1 签名 | JSON | `/v1/bot/messages/create` | ✅ 生产可用 |
| 🟢 **飞书** (Feishu/Lark) | AES-256-CBC + SHA256 签名 | JSON + URL Verification | `/open-apis/im/v1/messages` | ✅ 已实现 |
| 🔷 **钉钉** (DingTalk) | AES-256-CBC + HMAC-SHA256 | JSON | `/robot/send` | ✅ 已实现 |
| 🟠 **企业微信** (WeCom) | AES-256-CBC + SHA1 签名 | **XML** 入站 / JSON 出站 | `/cgi-bin/message/send` | ✅ 已实现 |
| ⚪ **通用 HTTP** (Generic) | 无加密 | JSON（字段可配置） | 所有 POST | ✅ 已实现 |

### 配置示例

<details>
<summary>🔵 蓝信（默认，向后兼容）</summary>

```yaml
# channel: "lanxin"    # 可省略，默认就是蓝信
callbackKey: "YOUR_CALLBACK_KEY_BASE64"
callbackSignToken: "YOUR_SIGN_TOKEN"
lanxin_upstream: "https://apigw.lx.qianxin.com"
```

</details>

<details>
<summary>🟢 飞书</summary>

```yaml
channel: "feishu"
feishu_encrypt_key: "YOUR_ENCRYPT_KEY"
feishu_verification_token: "YOUR_VERIFICATION_TOKEN"
lanxin_upstream: "https://open.feishu.cn"
```

飞书 URL Verification 自动处理：收到 `{"type":"url_verification","challenge":"xxx"}` 时自动返回 challenge。

</details>

<details>
<summary>🔷 钉钉</summary>

```yaml
channel: "dingtalk"
dingtalk_token: "YOUR_TOKEN"
dingtalk_aes_key: "YOUR_AES_KEY_43CHARS"      # 43 字符 base64
dingtalk_corp_id: "YOUR_CORP_ID"
lanxin_upstream: "https://oapi.dingtalk.com"
```

</details>

<details>
<summary>🟠 企业微信</summary>

```yaml
channel: "wecom"
wecom_token: "YOUR_TOKEN"
wecom_encoding_aes_key: "YOUR_ENCODING_AES_KEY_43CHARS"
wecom_corp_id: "YOUR_CORP_ID"
lanxin_upstream: "https://qyapi.weixin.qq.com"
```

注意：企微入站是 XML 格式，lobster-guard 自动处理 XML↔JSON 转换。

</details>

<details>
<summary>⚪ 通用 HTTP（自定义 webhook）</summary>

```yaml
channel: "generic"
generic_sender_header: "X-Sender-Id"
generic_text_field: "content"
lanxin_upstream: "https://your-api.example.com"
```

适用于自建消息系统或其他未内置支持的平台。

</details>

### 插件架构

```
┌─────────────────────────────────────────────────────┐
│                  ChannelPlugin 接口                   │
├──────────┬──────────┬──────────┬──────────┬──────────┤
│ 🔵 蓝信  │ 🟢 飞书  │ 🔷 钉钉  │ 🟠 企微  │ ⚪ 通用  │
│ AES+SHA1 │ AES+SHA2 │ AES+HMAC │ AES+XML  │ 明文JSON │
├──────────┴──────────┴──────────┴──────────┴──────────┤
│              InboundProxy / OutboundProxy             │
│          （安全检测、路由、审计 — 通道无关）            │
└─────────────────────────────────────────────────────┘
```

### Bridge Mode（v3.1 长连接桥接）

飞书和钉钉支持 WebSocket 长连接模式（无需公网 IP）。

```yaml
channel: "feishu"
mode: "bridge"       # 加这一行，从 Webhook 切到长连接
```

```
Webhook 模式:   平台 ──POST──► :18443 ──► 安检 ──► Agent
Bridge 模式:    lobster-guard ══WSS══► 平台（拉消息）──► 安检 ──► POST Agent
                                       对 Agent 来说完全一样 ↑
```

| 通道 | Webhook | Bridge | Bridge 特性 |
|------|---------|--------|------------|
| 🔵 蓝信 | ✅ | ❌ | 蓝信仅支持 Webhook |
| 🟢 飞书 | ✅ | ✅ | WSS + token 自动刷新（2h）+ 事件确认 |
| 🔷 钉钉 | ✅ | ✅ | Stream + ticket 自动获取 + ping/pong |
| 🟠 企微 | ✅ | ❌ | 企微仅支持 Webhook |
| ⚪ 通用 | ✅ | ❌ | — |

Bridge 特性：
- 🔄 **自动重连** — 断线指数退避（1s → 2s → 4s → ... → 60s max）
- 🔑 **Token 自动刷新** — 飞书 token 每 100 分钟自动刷新
- 💓 **心跳保活** — 自动处理 ping/pong
- 📊 **状态监控** — `/healthz` 展示连接状态、重连次数、消息计数
- 🔀 **混合模式** — Bridge 模式下 `:18443` 仍然监听，可同时接收 Webhook

---

## 🏛️ 架构

```
                        ┌────────────────────────────────────────────────┐
                        │              lobster-guard 🦞 v17.1.0          │
                        │                                                │
                        │  ┌─────────────── IM 安全域 ────────────────┐  │
                        │  │                                          │  │
                        │  │  ┌──────────┐    ┌──────────────────┐    │  │
 消息平台 ─────────────────►│ :18443   │───►│ 入站规则引擎      │    │  │
 (蓝信/飞书/钉钉/企微)  │  │  │ 入站代理  │    │ AC自动机+正则     │    │  │
                        │  │  │ Channel  │    │ +Pipeline+Session │    │  │
                        │  │  │ Plugin   │    │ +LLM语义分析      │    │  │
                        │  │  └────┬─────┘    └──────────────────┘    │  │
                        │  │       │                                  │  │
                        │  │       ▼                                  │  │
                        │  │  ┌──────────┐    ┌──────────────┐        │  │
                        │  │  │ 路由表    │───►│ 上游容器池    │───────────► OpenClaw ×N
                        │  │  │ 亲和绑定  │    │ 健康检查      │        │  │
                        │  │  └──────────┘    └──────────────┘        │  │
                        │  │                                          │  │
                        │  │  ┌──────────┐    ┌──────────────────┐    │  │
 OpenClaw 出站 ────────────►│ :18444   │───►│ 出站规则引擎      │    │  │
   (API调用)            │  │  │ 出站代理  │    │ PII/凭据/命令     │───────► 消息平台 API
                        │  │  └──────────┘    │ (6默认+自定义)    │    │  │
                        │  │                  └──────────────────┘    │  │
                        │  └──────────────────────────────────────────┘  │
                        │                                                │
                        │  ┌─────────────── LLM 安全域 ───────────────┐  │
                        │  │                                          │  │
                        │  │  ┌──────────┐    ┌──────────────────┐    │  │
 Agent/应用 ───────────────►│ :8445    │───►│ LLM 规则引擎      │    │  │
   (LLM API)            │  │  │ LLM 代理  │    │ 11默认+自定义     │───────► LLM API
                        │  │  │ 蜜罐集成  │    │ +Canary Token     │    │  │
                        │  │  └──────────┘    └──────────────────┘    │  │
                        │  └──────────────────────────────────────────┘  │
                        │                                                │
                        │  ┌─────────── 安全分析 + 管理 ──────────────┐  │
                        │  │  ┌──────────┐  ┌────────┐  ┌─────────┐  │  │
 运维/安全团队 ────────────►│ :9090    │  │ 行为   │  │ 攻击链  │  │  │
                        │  │  │ Dashboard │  │ 画像   │  │ Red Team│  │  │
                        │  │  │ 227 API  │  │ 异常   │  │ 蜜罐    │  │  │
                        │  │  │ 态势大屏  │  │ 基线   │  │ A/B测试 │  │  │
                        │  │  └──────────┘  └────────┘  └─────────┘  │  │
                        │  │                                          │  │
                        │  │  ┌──────────────────────────────────┐    │  │
                        │  │  │ SQLite · WAL · 报告引擎 · JWT    │    │  │
                        │  │  │ 多租户 · 会话回放 · 排行榜        │    │  │
                        │  │  └──────────────────────────────────┘    │  │
                        │  └──────────────────────────────────────────┘  │
                        └────────────────────────────────────────────────┘
```

### 4 端口架构

| 端口 | 功能 | 安全域 |
|------|------|--------|
| `:18443` | IM 入站代理 | IM 安全域 |
| `:18444` | IM 出站代理 | IM 安全域 |
| `:8445` | LLM 反向代理 | LLM 安全域 |
| `:9090` | Dashboard + API + Metrics | 管理层 |

### 单机模式（v1.0 兼容）

不配 `static_upstreams` 时自动退化：

```
消息平台 ──► :18443 ──► OpenClaw(:18790)
OpenClaw ──► :18444 ──► 消息平台 API
Agent    ──► :8445  ──► LLM API (Anthropic/OpenAI)
```

---

## 🚀 快速开始

### 环境要求

- **Go 1.21+**（编译需要）
- **GCC**（CGO 编译 SQLite 需要）
- **Linux / macOS**（生产推荐 Linux）

### 1. 编译

```bash
git clone https://github.com/zhuowater/lobster-guard.git
cd lobster-guard

# 编译
CGO_ENABLED=1 go build -o lobster-guard .

# 或使用 Makefile
make build
```

编译产物是一个约 19MB 的单二进制文件。

### 2. 配置

```bash
cp config.yaml.example config.yaml
vim config.yaml
```

**最小配置（单机模式）：**

```yaml
# 蓝信加密凭据
callbackKey: "你的回调加密密钥"
callbackSignToken: "你的签名令牌"

# IM 代理监听（4 端口架构）
inbound_listen: ":18443"
outbound_listen: ":18444"
openclaw_upstream: "http://127.0.0.1:18790"
lanxin_upstream: "https://apigw.lx.qianxin.com"

# LLM 代理（可选）
llm_proxy:
  enabled: true
  listen: ":8445"

# 管理
management_listen: ":9090"
management_token: "your-secret-management-token"

# JWT 认证
auth:
  enabled: true
  jwt_secret: "your-jwt-secret-at-least-32-chars"

# 安全检测
inbound_detect_enabled: true
outbound_audit_enabled: true
detect_timeout_ms: 50
db_path: "./audit.db"
```

**出站规则（v18 智能合并）：**

> v18 起，出站规则采用智能合并机制：6 条默认规则（PII 身份证/手机/银行卡 + 凭据密码/APIKey + 恶意命令）始终加载，用户配置的同名规则覆盖默认规则，新名称规则追加。

```yaml
outbound_rules:
  # 覆盖默认规则
  - name: "pii_id_card"
    pattern: '(?:^|\D)\d{17}[\dXx](?:\D|$)'
    action: "warn"    # 从 block 改为 warn
  # 追加自定义规则
  - name: "internal_ip_leak"
    pattern: '10\.\d{1,3}\.\d{1,3}\.\d{1,3}'
    action: "warn"
```

### 3. 运行

```bash
# 前台运行
./lobster-guard -config config.yaml

# 或后台运行
nohup ./lobster-guard -config config.yaml > /var/log/lobster-guard.log 2>&1 &
```

### 4. 验证

```bash
# 健康检查
curl http://localhost:9090/healthz

# 打开管理后台
open http://localhost:9090/

# 端到端模拟测试（v18 新增）
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  http://localhost:9090/api/v1/simulate/traffic
```

---

## 🧪 端到端模拟测试（v18 新增）

lobster-guard 内置端到端流量模拟器，可一键验证入站检测→路由→出站拦截→蜜罐引爆→审计记录全链路：

```bash
# 触发端到端模拟
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/simulate/traffic

# 响应示例
{
  "status": "completed",
  "scenarios_run": 12,
  "scenarios_passed": 12,
  "scenarios_failed": 0,
  "duration_ms": 234,
  "results": [
    {"scenario": "inbound_injection_block", "passed": true},
    {"scenario": "outbound_pii_block", "passed": true},
    {"scenario": "llm_canary_detect", "passed": true},
    {"scenario": "honeypot_trigger", "passed": true},
    {"scenario": "trace_correlation", "passed": true},
    ...
  ]
}
```

模拟覆盖场景：
- ✅ 入站 Prompt Injection 检测与拦截
- ✅ 出站 PII/凭据泄露拦截
- ✅ LLM 规则引擎检测（11 条默认规则）
- ✅ Canary Token 泄露检测
- ✅ 蜜罐引爆检测（8 模板）
- ✅ 路由亲和与策略匹配
- ✅ trace_id 全链路关联
- ✅ 审计日志完整性

---

## 🖥️ 管理后台

访问 `http://your-server:9090/` 即可打开管理后台（需 JWT 登录）。

### Vue 3 Dashboard（29 页面 · 19 组件）

| 分组 | 页面 | 说明 |
|------|------|------|
| **IM 安全** | Overview | 总览：请求/拦截/告警大数字 + 趋势图 + 饼图 |
| | Rules | 入站/出站规则管理 + 热更新 + 命中率排行 |
| | Audit | 审计日志 + 时间线 + 全文搜索 + CSV/JSON 导出 |
| | Monitor | 实时监控 + QPS 柱状图 + 攻击实时流 |
| | Routes | 路由管理 + 策略匹配 + Bot/部门筛选 |
| **LLM 安全** | LLMOverview | LLM 代理概览 + Token 成本看板 |
| | LLMRules | LLM 规则管理（11 条默认 + 自定义）|
| | PromptTracker | Prompt 版本追踪 + Diff 对比 |
| | Honeypot | 蜜罐管理 + 8 模板 + 引爆记录 |
| | ABTesting | Prompt A/B 测试 + 效果量化 |
| | SessionReplay | 会话回放 + 时间线 + 标签 |
| **威胁分析** | UserProfiles | 攻击者画像 + 驾驶舱模式 |
| | BehaviorProfile | 行为画像 + 特征提取 + 模式学习 |
| | AttackChain | 攻击链检测 + Kill Chain 映射 |
| | AnomalyDetection | 异常基线检测 + 健康分 + OWASP 矩阵 |
| | RedTeam | Red Team Autopilot + 33 攻击向量 |
| **安全治理** | Reports | 安全日报/周报 + 合规审计 + PDF 导出 |
| | Leaderboard | 安全排行榜 + SLA 达成率 |
| | Tenants | 多租户管理 + 租户隔离 |
| | Settings | 系统设置 + 参数配置 |
| **系统** | Operations | 运维工具箱（配置/备份/诊断/告警）|
| | Upstream | 上游容器管理 + 健康检查 |
| | Users | 用户管理 CRUD |
| | BigScreen | 态势感知大屏 + 4 预设模板 |

*附加页面：CustomDashboard（可拖拽自定义布局）/ Login / UserDetail / SessionDetail*

组件库（19 个）：TrendChart / PieChart / HeatMap / RuleEditor / TimelineChart 等，统一 Indigo 配色主题。

---

## 🛡️ 安全检测能力

### 规则体系

#### 入站检测（多层防御）

**DetectPipeline 检测链**: Keyword → Regex → PII → Session → LLM（可配置顺序和启停）

| 层级 | 引擎 | 说明 |
|------|------|------|
| 关键词 | AC 自动机 | O(n) 多模式匹配，40+ 内置规则 |
| 正则 | regexp | 自定义正则模式，100ms 超时保护 |
| 规则模板 | go:embed | 4 场景 66 条规则（通用/金融/医疗/政务）|
| 上下文感知 | SessionDetector | 会话级风险积分 · 多轮攻击识别 · 自动升级 |
| 语义分析 | LLMDetector | 可选 · async/sync · 外部 LLM API · fail-open |
| 结果缓存 | DetectCache | SHA-256 LRU · pass/warn 缓存 · block 不缓存 |

| 类别 | 检测内容 | 动作 |
|------|----------|------|
| Prompt Injection | `ignore previous instructions`, `忽略之前的指令` | 🔴 Block |
| 越狱攻击 | `you are now DAN`, `jailbreak`, `没有限制的AI` | 🔴 Block |
| 系统提示词窃取 | `show system prompt`, `输出提示词` | 🔴 Block |
| 命令注入 | `rm -rf /`, `curl\|bash`, `base64 -d\|bash` | 🔴 Block |
| 角色扮演诱导 | `假设你是`, `pretend you are` | 🟡 Warn |
| PII 检测 | 身份证号、手机号、银行卡号 | 🟡 Warn + 记录 |

#### 出站检测（v18 智能合并）

6 条默认规则始终加载，用户配置同名覆盖、新名称追加：

| 默认规则 | 检测内容 | 动作 |
|----------|----------|------|
| `pii_id_card` | 身份证号 | 🔴 Block |
| `pii_phone` | 手机号 | 🟡 Warn |
| `pii_bank_card` | 银行卡号 | 🔴 Block |
| `credential_password` | 密码泄露 | 🔴 Block |
| `credential_apikey` | API Key（sk-/ghp_/AKIA） | 🔴 Block |
| `malicious_command` | 恶意命令（rm -rf/curl\|bash） | 🔴 Block |

#### LLM 规则（v18 智能合并）

11 条默认规则始终加载，用户配置同名覆盖、新名称追加：

| 类别 | 规则数 | 检测内容 |
|------|--------|----------|
| PI 检测 | 3 | Prompt Injection 注入检测 |
| PII 检测 | 3 | 请求/响应中的个人信息 |
| 敏感话题 | 1 | 政治/暴力/违法内容 |
| Token 滥用 | 1 | Token 用量异常 |
| 响应方向 | 3 | 响应内容安全检测（v18 新增）|

---

## 📡 API 参考（~227 路由）

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 管理后台 Dashboard |
| GET | `/healthz` | 健康检查 + 系统概览 |
| GET | `/metrics` | Prometheus 指标导出 |
| POST | `/api/v1/auth/login` | JWT 登录 |

### IM 安全域

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams` | 列出所有上游容器 |
| GET | `/api/v1/routes` | 列出用户路由绑定 |
| POST | `/api/v1/routes/bind` | 绑定用户到上游 |
| POST | `/api/v1/routes/unbind` | 解绑用户路由 |
| POST | `/api/v1/routes/migrate` | 迁移用户到新上游 |
| POST | `/api/v1/routes/batch-bind` | 批量绑定 |
| GET | `/api/v1/routes/stats` | 路由统计 |
| GET | `/api/v1/inbound-rules` | 入站规则列表 |
| POST | `/api/v1/inbound-rules/reload` | 热更新入站规则 |
| GET | `/api/v1/outbound-rules` | 出站规则列表 |
| POST | `/api/v1/outbound-rules` | 出站规则 CRUD（v18）|
| PUT | `/api/v1/outbound-rules/:name` | 更新出站规则（v18）|
| DELETE | `/api/v1/outbound-rules/:name` | 删除出站规则（v18）|
| GET | `/api/v1/audit/logs` | 查询审计日志 |
| GET | `/api/v1/audit/export` | 导出审计日志 |
| GET | `/api/v1/audit/timeline` | 时间线统计 |
| GET | `/api/v1/stats` | 统计概览 |

### LLM 安全域

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/llm/overview` | LLM 代理概览 |
| GET | `/api/v1/llm/rules` | LLM 规则列表（11 默认 + 自定义）|
| POST | `/api/v1/llm/rules` | LLM 规则 CRUD |
| GET | `/api/v1/llm/cost` | Token 成本看板 |
| GET | `/api/v1/llm/audit` | LLM 审计日志 |

### 威胁分析

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/user-profiles` | 攻击者画像列表 |
| GET | `/api/v1/user-profiles/:id` | 用户详情 |
| GET | `/api/v1/behavior-profiles` | 行为画像 |
| GET | `/api/v1/attack-chains` | 攻击链检测结果 |
| GET | `/api/v1/anomaly/alerts` | 异常检测告警 |
| GET | `/api/v1/health-score` | 健康分 + OWASP 矩阵 |
| POST | `/api/v1/redteam/run` | 触发 Red Team 测试 |
| GET | `/api/v1/redteam/results` | Red Team 结果 |

### 安全治理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/reports` | 安全报告列表 |
| POST | `/api/v1/reports/generate` | 生成报告（日报/周报/合规）|
| GET | `/api/v1/reports/:id/pdf` | 导出 PDF |
| GET | `/api/v1/leaderboard` | 安全排行榜 |
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants` | 创建租户 |

### 高级功能

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/honeypots` | 蜜罐列表 |
| POST | `/api/v1/honeypots` | 创建蜜罐 |
| GET | `/api/v1/honeypots/triggers` | 引爆记录 |
| GET | `/api/v1/ab-tests` | A/B 测试列表 |
| POST | `/api/v1/ab-tests` | 创建 A/B 测试 |
| GET | `/api/v1/sessions/replay/:id` | 会话回放 |
| GET | `/api/v1/prompt-tracker` | Prompt 版本追踪 |
| GET | `/api/v1/layouts` | 自定义布局列表 |
| POST | `/api/v1/layouts` | 保存布局 |
| GET | `/api/v1/bigscreen/data` | 态势大屏数据 |
| POST | `/api/v1/simulate/traffic` | 端到端模拟测试（v18）|

### 系统管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/users` | 用户列表 |
| POST | `/api/v1/users` | 创建用户（v18）|
| PUT | `/api/v1/users/:id` | 更新用户（v18）|
| DELETE | `/api/v1/users/:id` | 删除用户（v18）|
| POST | `/api/v1/backup` | 创建数据库备份 |
| GET | `/api/v1/backups` | 列出备份 |
| GET | `/api/v1/metrics/realtime` | 实时统计 |
| GET | `/api/v1/ws/connections` | WebSocket 连接列表 |

> 以上为主要 API 摘要，完整 ~227 路由参见源码 `api.go`。

<details>
<summary>📝 API 调用示例</summary>

```bash
TOKEN="your-management-token"

# JWT 登录
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"xxx"}' \
  http://localhost:9090/api/v1/auth/login | jq .

# 健康检查
curl -s http://localhost:9090/healthz | jq .

# 端到端模拟测试
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/simulate/traffic | jq .

# 查询拦截日志
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9090/api/v1/audit/logs?action=block&limit=20" | jq .

# 触发 Red Team 测试
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/redteam/run | jq .
```

</details>

---

## 📦 部署指南

### 方式一：直接运行（推荐快速试用）

```bash
CGO_ENABLED=1 go build -o lobster-guard .
cp config.yaml.example config.yaml
./lobster-guard -config config.yaml
```

### 方式二：Systemd 服务（推荐生产部署）

```bash
sudo cp lobster-guard /usr/local/bin/
sudo mkdir -p /etc/lobster-guard /var/lib/lobster-guard
sudo cp config.yaml /etc/lobster-guard/

sudo tee /etc/systemd/system/lobster-guard.service << 'EOF'
[Unit]
Description=Lobster Guard - AI Agent Security Gateway
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/lobster-guard -config /etc/lobster-guard/config.yaml
Restart=always
RestartSec=5
WorkingDirectory=/etc/lobster-guard
LimitNOFILE=65536
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/lobster-guard
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl start lobster-guard
sudo systemctl enable lobster-guard
```

### 方式三：Docker

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o lobster-guard .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/lobster-guard /usr/local/bin/
COPY config.yaml.example /etc/lobster-guard/config.yaml
EXPOSE 18443 18444 8445 9090
CMD ["lobster-guard", "-config", "/etc/lobster-guard/config.yaml"]
```

```bash
docker build -t lobster-guard .
docker run -d -p 18443:18443 -p 18444:18444 -p 8445:8445 -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/lobster-guard/config.yaml \
  lobster-guard
```

### 方式四：Make（推荐开发）

```bash
make build      # 编译
make test       # 运行测试（754 个用例）
make install    # 安装到系统
make healthz    # 检查健康状态
make stats      # 查看统计
make logs       # 查看审计日志
```

---

## 🧪 测试

```bash
# 运行全部测试（754 个用例，约 25 秒）
CGO_ENABLED=1 go test -v -count=1 ./...
```

### 测试覆盖（754 个用例，全部通过）

| 类别 | 用例数 | 内容 |
|------|--------|------|
| AC 自动机 | 6 | 基本匹配、大小写、中文、多模式、空输入 |
| 入站规则引擎 | 20+ | block/warn/log/PII/优先级权重/自定义消息 |
| 正则规则 | 9 | 匹配/优先级/编译失败/热更新/命中统计 |
| 规则分组/绑定 | 10+ | group 标签/app_id 绑定/通配符/无分组兼容 |
| 出站规则引擎 | 8+ | block/warn/热更新/优先级/v18 智能合并/CRUD |
| PII 可配置 | 8+ | 默认模式/自定义模式/编译失败回退/API |
| 蓝信加解密 | 4 | 初始化/签名/加密解密全链路 |
| 飞书插件 | 6 | 加解密/URL Verification/出站提取 |
| 钉钉插件 | 5 | 加解密/HMAC 签名/出站提取 |
| 企微插件 | 7 | XML 加解密/GET 验证/签名校验/出站提取 |
| 通用插件 | 4 | 默认/自定义字段/出站审计 |
| Bridge Mode | 5 | 状态序列化/Token 刷新/Ticket 获取/支持矩阵 |
| Rate Limiting | 14 | 令牌桶/全局/每用户/白名单/清理/统计/API |
| Prometheus | 11 | 计数器/直方图/格式输出/端点/开关 |
| 规则热更新 | 16 | 文件加载/验证/优先级/并发 reload+detect |
| 路由表 | 12+ | 复合键 CRUD/迁移/批量绑定/策略匹配 |
| 用户信息 | 15+ | 缓存 GetOrFetch/ListAll/刷新/Provider/API |
| 审计日志 | 18+ | 导出CSV/JSON/清理/时间线/归档/全文搜索 |
| 告警通知 | 6+ | webhook/蓝信格式/最小间隔/内容截断 |
| WebSocket 代理 | 24 | 连接/帧转发/检测拦截/超时/心跳/并发限制 |
| Store 抽象层 | 20+ | SQLiteStore CRUD/备份/恢复/Ping |
| 优雅关闭 | 10+ | 信号处理/健康检查/5维检查/关闭流程 |
| 配置验证 | 9 | 端口冲突/通道/模式/正则编译/PII/上游 |
| 检测链 Pipeline | 14 | 阶段串行/block 终止/自定义顺序 |
| 上下文感知 | 14 | 风险积分/时间衰减/自动升级/重置 |
| LLM 检测 | 18 | async/sync/超时/fail-open/mock |
| 检测缓存 | 14 | LRU/TTL/block 不缓存/命中统计 |
| 规则模板库 | 18 | 加载/合并/go:embed/API |
| LLM 代理 | 15+ | 反向代理/审计/成本/规则引擎/Canary Token |
| LLM 规则 | 20+ | 11 默认规则/合并/CRUD/v18 修复 |
| JWT 认证 | 15+ | 登录/Token 验证/过期/刷新/多租户 |
| Red Team | 12+ | 33 攻击向量/结果收集/排行榜/SLA |
| 蜜罐 | 15+ | 8 模板/水印/引爆检测/代理集成 |
| A/B 测试 | 12+ | 创建/分流/效果量化/推荐 |
| 行为画像 | 12+ | 特征提取/模式学习/基线/异常 |
| 攻击链 | 15+ | 多阶段关联/Kill Chain/升级策略 |
| 异常检测 | 12+ | 基线建立/偏差检测/告警/衰减 |
| 布局 | 12+ | 4 预设模板/自定义/拖拽/保存 |
| 报告引擎 | 10+ | 日报/周报/合规/PDF 导出 |
| 会话回放 | 10+ | 时间线/标签/搜索/Prompt 追踪 |
| 多租户 | 12+ | 租户 CRUD/隔离/JWT/权限 |
| 用户管理 | 10+ | CRUD/v18 修复/批量操作 |
| 端到端模拟 | 8+ | 全链路/trace 关联/蜜罐集成（v18）|
| **集成测试** | 10+ | Mock 上游 + 加密 webhook 全链路 |
| **并发测试** | 5+ | 多 goroutine 入站/出站/混合攻防 |

---

## ⚡ 性能

| 指标 | 数值 |
|------|------|
| 检测延迟（P99） | < 5ms |
| 入站吞吐（单核） | > 5,000 req/s |
| 审计写入 | 异步，不阻塞请求 |
| 内存占用 | < 80MB |
| 二进制大小 | ~19MB |
| Dashboard 加载 | < 10KB (gzip) |
| 测试用例 | 754 (~25s) |

- 规则引擎基于 **Aho-Corasick 算法**，O(n) 时间复杂度，文本长度无关
- SQLite **WAL 模式**，支持并发读写
- HTTP 连接池复用，减少 TCP 握手开销

---

## 📁 项目结构

```
lobster-guard/
├── main.go                 # 入口 + CLI 参数 + 启动流程
├── config.go               # 配置加载 + 验证器
├── plugin.go               # ChannelPlugin 接口 + 5 通道插件
├── bridge.go               # Bridge Mode (飞书/钉钉 WSS)
├── detect.go               # RuleEngine (AC自动机 + 正则 + 规则绑定)
├── pipeline.go             # DetectPipeline 检测链
├── session_detect.go       # 上下文感知检测 (风险积分)
├── llm_detect.go           # 可选 LLM 语义检测
├── detect_cache.go         # 检测结果 LRU 缓存
├── rule_templates.go       # 规则模板库 (go:embed)
├── route.go                # 路由表 + 上游池 + 用户信息 + 策略引擎
├── proxy.go                # 入站/出站 HTTP 代理 + 蜜罐集成
├── ws_proxy.go             # WebSocket 消息流代理
├── audit.go                # 审计日志 + 归档
├── alert.go                # Block 告警通知 (webhook)
├── api.go                  # 管理 API (~227 路由)
├── auth.go                 # JWT 认证 + 多租户
├── tenant.go               # 多租户管理
├── llm_proxy.go            # LLM 反向代理
├── llm_audit.go            # LLM 审计 + 成本看板
├── llm_rules.go            # LLM 规则引擎 (11 默认规则)
├── honeypot.go             # Agent 蜜罐 (8 模板)
├── ab_testing.go           # Prompt A/B 测试
├── redteam.go              # Red Team Autopilot (33 向量)
├── user_profile.go         # 攻击者画像
├── behavior_profile.go     # 行为画像 + 模式学习
├── attack_chain.go         # 攻击链检测 + Kill Chain
├── anomaly_detect.go       # 异常基线检测
├── health_score.go         # 健康分 + OWASP 矩阵 + 驾驶舱
├── leaderboard.go          # 安全排行榜 + SLA
├── report.go               # 报告引擎 (日报/周报/PDF)
├── session_replay.go       # 会话回放 + 时间线
├── prompt_tracker.go       # Prompt 版本追踪
├── layout.go               # 可拖拽自定义布局 (4 预设模板)
├── metrics.go              # Prometheus + 限流
├── realtime.go             # 实时监控 (环形缓冲区)
├── logger.go               # 结构化日志 (text/json)
├── trace.go                # 请求追踪 (trace_id)
├── store.go                # Store 抽象层 + SQLiteStore
├── shutdown.go             # 优雅关闭 + 健康检查增强
├── crypto.go               # AES 加解密 + AC 自动机
├── dashboard.go            # go:embed Vue 3 Dashboard
├── dashboard/              # Vue 3 前端 (29 页面 + 19 组件)
│   ├── src/views/          #   29 个页面
│   └── src/components/     #   19 个组件
├── config.yaml.example     # 配置模板
├── rules/                  # 规则模板库 (66 条, 4 场景)
│   ├── general.yaml        #   通用 (越狱/注入/社工)
│   ├── financial.yaml      #   金融
│   ├── medical.yaml        #   医疗
│   └── government.yaml     #   政务
├── skills/SKILL.md         # OpenClaw Agent Skill
├── docs/                   # 设计文档 + 截图
├── ROADMAP.md              # 版本路线图
└── go.mod / go.sum         # Go 依赖 (4 个)
```

### 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/mattn/go-sqlite3` | SQLite 驱动 |
| `gopkg.in/yaml.v3` | YAML 配置解析 |
| `github.com/gorilla/websocket` | WebSocket（Bridge Mode + 实时通信）|
| `golang.org/x/crypto` | bcrypt 密码哈希（用户认证）|

仅四个外部依赖，其余全部使用 Go 标准库。

---

## 🤖 OpenClaw Skill 集成

lobster-guard 提供了 OpenClaw Agent Skill，让 AI Agent 可以通过自然语言管理安全网关：

```
你：龙虾状态怎么样？
Agent：4 个上游全部健康，路由 5 条，已拦截 7 次攻击...

你：谁被拦截了？
Agent：最近拦截记录：attacker-001 (Prompt Injection)、attacker-002 (命令注入)...

你：跑一下 Red Team 测试
Agent：Red Team Autopilot 启动，33 个攻击向量测试中... 通过率 97%
```

Skill 文件位于 `skills/lobster-guard/SKILL.md`。

---

## 📋 配置参考

详细配置说明参见 `config.yaml.example`，涵盖以下配置分组：

| 分组 | 主要配置项 |
|------|-----------|
| **通道** | `channel`, `mode`, 各平台加密凭据 |
| **代理** | `inbound_listen`(:18443), `outbound_listen`(:18444), `management_listen`(:9090) |
| **LLM 代理** | `llm_proxy.enabled`, `llm_proxy.listen`(:8445), `llm_proxy.targets` |
| **认证** | `auth.enabled`, `auth.jwt_secret`, `auth.token_expiry` |
| **多租户** | `tenant.enabled`, `tenant.default_tenant`, `tenant.isolation` |
| **检测** | `inbound_detect_enabled`, `outbound_audit_enabled`, `detect_timeout_ms` |
| **规则** | `inbound_rules`, `outbound_rules`(6 默认+合并), `llm_proxy.rules`(11 默认+合并) |
| **路由** | `route_default_policy`, `route_policies`, `static_upstreams` |
| **Red Team** | `redteam.enabled`, `redteam.vectors`, `redteam.schedule` |
| **蜜罐** | `honeypot.enabled`, `honeypot.templates`, `honeypot.watermark` |
| **A/B 测试** | `ab_testing.enabled`, `ab_testing.max_concurrent` |
| **行为画像** | `behavior_profile.enabled`, `behavior_profile.features` |
| **攻击链** | `attack_chain.enabled`, `attack_chain.stages`, `attack_chain.auto_escalate` |
| **异常检测** | `anomaly.enabled`, `anomaly.baseline_days`, `anomaly.threshold` |
| **大屏** | `bigscreen.enabled`, `bigscreen.refresh_interval` |
| **自定义布局** | `custom_layout.enabled`, `custom_layout.presets` |
| **端到端模拟** | `simulate.enabled`, `simulate.auto_schedule` |
| **限流** | `rate_limit.global_rps`, `rate_limit.per_sender_rps` |
| **可观测性** | `metrics_enabled`, `log_format`, `log_level` |
| **高可用** | `shutdown_timeout`, `backup_dir`, `backup_auto_interval` |

---

## 🗺️ 版本历史

- [x] **v1-v2** — 核心代理（入站检测/出站拦截/亲和路由/注册心跳）
- [x] **v3.x** — 5 通道插件 + Bridge Mode + 限流 + Prometheus + 规则热更新 + 多 Bot 路由 + 告警通知
- [x] **v4** — 代码拆分 + WebSocket 代理 + 高可用（优雅关闭/备份/Store 抽象）
- [x] **v5** — 实时监控 + 智能检测 Pipeline + 会话风险检测 + LLM 语义分析
- [x] **v6-v7** — Vue 3 Dashboard（侧边栏/Hash 路由/TrendChart/PieChart/HeatMap/RuleEditor/Indigo 配色）
- [x] **v8** — 运维工具箱 + 策略路由 CRUD
- [x] **v9** — 双安全域（LLM 反向代理 + 审计 + 成本看板）
- [x] **v10** — LLM 规则引擎 + Canary Token + Shadow Mode
- [x] **v11** — 攻击者画像 + 驾驶舱模式（健康分/OWASP 矩阵/严格模式）+ 异常基线检测
- [x] **v12** — 报告引擎（安全日报/周报/合规审计/PDF 导出）
- [x] **v13** — 会话回放 + Prompt 版本追踪
- [x] **v14** — 多租户 + JWT 登录 + Red Team Autopilot（33 攻击向量）+ 排行榜 + SLA
- [x] **v15** — Agent 蜜罐（8 模板/水印追踪/引爆检测）+ Prompt A/B 测试
- [x] **v16** — 行为画像（特征提取/模式学习）+ 攻击链检测（多阶段关联）
- [x] **v17** — 态势感知大屏 + 可拖拽自定义布局（4 预设模板）
- [x] **v17.1.0 (v18 修复)** — 数据流全链路修复（蜜罐集成代理/用户管理 CRUD/出站规则 CRUD/自动调度/参数可配/trace 关联/端到端模拟测试）

---

## 📄 License

[MIT License](LICENSE)

---

<p align="center">
  <sub>🦞 Built with Go, secured with care. v17.1.0 · 42 files · 48K lines · 754 tests · 227 APIs</sub>
</p>
