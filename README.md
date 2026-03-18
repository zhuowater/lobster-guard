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
| 🔀 **多 Bot 亲和路由** | 按 (用户ID, BotID) 复合键绑定 · 邮箱/部门策略路由 |
| 🌐 **WebSocket 代理** | Agent 实时 streaming 安全扫描 · inspect/passthrough 模式 |
| 🔌 **多通道插件** | 蓝信/飞书/钉钉/企微/通用HTTP · Bridge Mode WSS 长连接 |
| 🤖 **LLM 安全域** | LLM 反向代理（:8445）· 工具调用审计 · Token 成本看板 · 11 条默认规则 |
| 🎯 **Canary Token** | Prompt 泄露检测 · Shadow Mode · 自动轮换 |
| 🔴 **Red Team Autopilot** | 33 攻击向量自动化测试 · 安全排行榜 · SLA 达成率 |
| 🍯 **Agent 蜜罐** | 8 模板诱捕 · 水印追踪 · 引爆检测 |
| 🧬 **行为画像** | 特征提取 · 模式学习 · 异常基线检测 · 攻击者画像 |
| 🔗 **攻击链检测** | 多阶段关联分析 · Kill Chain 映射 · 自动升级策略 |
| 📊 **态势感知大屏** | 实时攻防态势 · 可拖拽布局 · 4 预设模板 · 全屏驾驶舱 |
| 🖥️ **管理后台** | Vue 3 Dashboard · 29 页面 · 19 组件 · 5 组侧边栏 |

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

### 安全拦截一览

> Block 红色高亮 · Warn 黄色高亮 · 支持方向/动作/发送者多维筛选

![Dashboard Block Filter](docs/screenshots/dashboard-block-filter.jpg)

📖 更多截图：[管理后台详情](docs/dashboard.md)

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
CGO_ENABLED=1 go build -o lobster-guard .
```

### 2. 配置

```bash
cp config.yaml.example config.yaml
vim config.yaml
```

**最小配置（单机模式）：**

```yaml
callbackKey: "你的回调加密密钥"
callbackSignToken: "你的签名令牌"
inbound_listen: ":18443"
outbound_listen: ":18444"
openclaw_upstream: "http://127.0.0.1:18790"
lanxin_upstream: "https://apigw.lx.qianxin.com"
llm_proxy:
  enabled: true
  listen: ":8445"
management_listen: ":9090"
management_token: "your-secret-management-token"
auth:
  enabled: true
  jwt_secret: "your-jwt-secret-at-least-32-chars"
inbound_detect_enabled: true
outbound_audit_enabled: true
db_path: "./audit.db"
```

### 3. 运行

```bash
./lobster-guard -config config.yaml
```

### 4. 验证

```bash
curl http://localhost:9090/healthz          # 健康检查
open http://localhost:9090/                  # 管理后台
```

📖 详细配置：[配置参考](docs/configuration.md) · 📖 部署方式：[部署指南](docs/deployment.md)

---

## 📖 文档目录

| 文档 | 说明 |
|------|------|
| [🏛️ 架构说明](docs/architecture.md) | 架构图 · 4 端口 · 数据流 · 插件架构 |
| [🔌 多通道配置](docs/channels.md) | 5 通道配置示例 · Bridge Mode WSS 长连接 |
| [🛡️ 安全检测能力](docs/detection.md) | 规则体系 · 检测管线 · 规则模板库 |
| [📡 API 参考](docs/api-reference.md) | ~227 路由完整列表 · 调用示例 |
| [📦 部署指南](docs/deployment.md) | 直接运行 · Systemd · Docker · Make |
| [🧪 测试说明](docs/testing.md) | 754 用例 · 端到端模拟 · 性能指标 |
| [📋 配置参考](docs/configuration.md) | 完整配置项 · 出站规则合并机制 |
| [🖥️ 管理后台](docs/dashboard.md) | 29 页面详情 · 组件库 · 截图集合 |

### 其他文档

| 文档 | 说明 |
|------|------|
| [Bridge Mode 设计](docs/bridge-mode-design.md) | Bridge Mode 详细设计文档 |
| [Channel Plugin 设计](docs/channel-plugin-design.md) | 通道插件架构设计 |
| [Dashboard 路线图](docs/dashboard-roadmap.md) | Dashboard 开发路线图 |
| [数据流审查](docs/data-flow-review.md) | 数据流审查报告 |
| [部署测试报告](docs/deployment-test-report.md) | 部署测试结果 |
| [系统审查 v17](docs/system-review-v17.md) | v17 系统审查报告 |

---

## 🤖 OpenClaw Skill 集成

lobster-guard 提供了 OpenClaw Agent Skill，让 AI Agent 可以通过自然语言管理安全网关：

```
你：龙虾状态怎么样？
Agent：4 个上游全部健康，路由 5 条，已拦截 7 次攻击...
```

Skill 文件位于 `skills/lobster-guard/SKILL.md`。

---

## 📁 项目结构

```
lobster-guard/
├── src/                    # Go 源代码（42 个文件）
│   ├── main.go             #   入口 + CLI 参数 + 启动流程
│   ├── config.go           #   配置加载 + 验证器
│   ├── plugin.go           #   ChannelPlugin 接口 + 5 通道插件
│   ├── detect.go           #   RuleEngine (AC自动机 + 正则)
│   ├── proxy.go            #   入站/出站 HTTP 代理
│   ├── api.go              #   管理 API (~227 路由)
│   ├── llm_proxy.go        #   LLM 反向代理
│   └── ...                 #   其余 35 个源文件
├── dashboard/              # Vue 3 前端 (29 页面 + 19 组件)
│   ├── src/views/          #   29 个页面
│   └── src/components/     #   19 个组件
├── rules/                  # 规则模板库 (66 条, 4 场景)
│   ├── general.yaml        #   通用 (越狱/注入/社工)
│   ├── financial.yaml      #   金融
│   ├── medical.yaml        #   医疗
│   └── government.yaml     #   政务
├── docs/                   # 文档 + 截图
├── skills/                 # OpenClaw Agent Skill
├── config.yaml.example     # 配置模板
├── Makefile                # 编译/测试/部署
├── ROADMAP.md              # 版本路线图
└── go.mod / go.sum         # Go 依赖 (4 个)
```

---

## 🗺️ 版本历史

| 版本 | 里程碑 |
|------|--------|
| **v1-v2** | 核心代理（入站检测/出站拦截/亲和路由） |
| **v3.x** | 5 通道插件 + Bridge Mode + 限流 + Prometheus |
| **v4** | WebSocket 代理 + 高可用（优雅关闭/备份/Store 抽象） |
| **v5** | 实时监控 + 智能检测 Pipeline + 会话风险检测 |
| **v6-v7** | Vue 3 Dashboard（29 页面 · Indigo 配色） |
| **v8** | 运维工具箱 + 策略路由 CRUD |
| **v9** | 双安全域（LLM 反向代理 + 审计 + 成本看板） |
| **v10** | LLM 规则引擎 + Canary Token + Shadow Mode |
| **v11** | 攻击者画像 + 驾驶舱模式 + 异常基线检测 |
| **v12** | 报告引擎（日报/周报/合规/PDF 导出） |
| **v13** | 会话回放 + Prompt 版本追踪 |
| **v14** | 多租户 + JWT + Red Team Autopilot + 排行榜 |
| **v15** | Agent 蜜罐（8 模板）+ Prompt A/B 测试 |
| **v16** | 行为画像 + 攻击链检测 |
| **v17** | 态势感知大屏 + 可拖拽自定义布局 |
| **v17.1.0** | 全链路修复（蜜罐集成/用户管理/端到端模拟） |

详细版本历史参见 [ROADMAP.md](ROADMAP.md)。

---

## 📄 License

[MIT License](LICENSE)

---

<p align="center">
  <sub>🦞 Built with Go, secured with care. v17.1.0 · 42 files · 48K lines · 754 tests · 227 APIs</sub>
</p>
