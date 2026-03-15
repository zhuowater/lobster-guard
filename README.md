<p align="center">
  <br>
  <span style="font-size:64px">🦞</span>
  <br>
</p>

<h1 align="center">lobster-guard（龙虾卫士）</h1>

<p align="center">
  <strong>AI Agent 安全网关 · 入站检测 · 出站拦截 · 亲和路由 · 服务注册</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-2.0.0-00d4ff?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/database-SQLite-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/binary-single_file-00ff88?style=flat-square" alt="Single Binary">
  <img src="https://img.shields.io/badge/license-MIT-yellow?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Protecting your upstream, one lobster at a time.</em>
</p>

---

## 🎯 这是什么

**lobster-guard** 是一个轻量级安全代理网关，专为 AI Agent（如 [OpenClaw](https://github.com/openclaw/openclaw)）设计。它部署在蓝信等消息平台与 AI Agent 之间，提供双向安全检测、智能路由和全量审计。

**一句话：用户的消息进来之前先安检，Agent 的回复出去之前再安检。**

### ✨ 核心能力

| 能力 | 说明 |
|------|------|
| 🛡️ **入站检测** | Aho-Corasick 多模式匹配 + 正则，拦截 Prompt Injection / 命令注入 / 越狱攻击 |
| 🔒 **出站拦截** | 防止 Agent 泄露身份证号、API Key、私钥、系统提示词等敏感信息 |
| 🔀 **亲和路由** | 按用户 ID 绑定到固定容器，保持 Agent 会话上下文连续性 |
| 📦 **服务注册** | 容器启动自动注册，心跳保活，故障自动转移 |
| 📊 **全量审计** | SQLite 持久化每一条请求的检测结果、延迟和路由决策 |
| 🖥️ **管理后台** | 内置 Web Dashboard，深色科技主题，实时监控 |

### 🏗️ 设计哲学

- **单二进制部署** — `main.go` 一个文件，编译出一个二进制，扔上去就跑
- **Fail-Open** — 检测异常不阻塞业务，宁可漏检不可误杀
- **零外部依赖** — 只依赖 Go 标准库 + SQLite + YAML 解析，不引入 Redis/MQ/K8s
- **向后兼容** — 不配多容器就自动退化为单上游模式，平滑升级

---

## 📸 界面预览

### 管理后台全览

> 深色科技主题 · SVG 环形图 · CSS 柱状图 · 实时数据刷新

![Dashboard Full](docs/screenshots/dashboard-full.jpg)

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
        龙虾卫士 - AI Agent 安全网关 v2.0.0
        入站检测 | 出站拦截 | 亲和路由 | 服务注册

┌─────────────────────────────────────────────────┐
│                  配置摘要 v2.0                   │
├─────────────────────────────────────────────────┤
│ 入站监听:    :8443                              │
│ 出站监听:    :8444                              │
│ 管理API:     :9090                              │
│ 入站检测:    true                               │
│ 出站审计:    true                               │
│ 路由策略:    least-users                        │
│ 静态上游:    3                                  │
│ 出站规则:    6                                  │
│ 检测超时:    50ms                               │
└─────────────────────────────────────────────────┘
```

---

## 🏛️ 架构

```
                        ┌──────────────────────────────────────┐
                        │         lobster-guard 🦞             │
                        │                                      │
                        │  ┌──────────┐    ┌──────────────┐    │
 蓝信/消息平台 ────────────►│ :8443    │───►│ 入站规则引擎  │    │
   (Webhook)            │  │ 入站代理  │    │ AC自动机+正则 │    │
                        │  └────┬─────┘    └──────────────┘    │
                        │       │                              │
                        │       ▼                              │
                        │  ┌──────────┐    ┌──────────────┐    │
                        │  │ 路由表    │───►│ 上游容器池    │────────► OpenClaw 容器 1
                        │  │ 亲和绑定  │    │ 健康检查      │────────► OpenClaw 容器 2
                        │  └──────────┘    │ 故障转移      │────────► OpenClaw 容器 3
                        │                  └──────────────┘    │
                        │                                      │
                        │  ┌──────────┐    ┌──────────────┐    │
 OpenClaw 出站 ────────────►│ :8444    │───►│ 出站规则引擎  │    │
   (API调用)            │  │ 出站代理  │    │ PII/凭据/命令 │────────► 蓝信 API
                        │  └──────────┘    └──────────────┘    │
                        │                                      │
                        │  ┌──────────┐    ┌──────────────┐    │
 运维/Agent ───────────────►│ :9090    │    │ SQLite 审计   │    │
   (管理+注册)          │  │ 管理API   │    │ WAL模式       │    │
                        │  │ Dashboard │    └──────────────┘    │
                        │  └──────────┘                        │
                        └──────────────────────────────────────┘
```

### 单机模式（v1.0 兼容）

不配 `static_upstreams` 时自动退化：

```
蓝信 ──► :8443 ──► OpenClaw(:18790)
OpenClaw ──► :8444 ──► 蓝信API
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

编译产物是一个约 14MB 的单二进制文件。

### 2. 配置

```bash
cp config.yaml.example config.yaml
vim config.yaml
```

**最小配置（单机模式）：**

```yaml
# 蓝信加密凭据（从蓝信开放平台获取）
callbackKey: "你的回调加密密钥"
callbackSignToken: "你的签名令牌"

# 代理监听
inbound_listen: ":8443"       # 接收蓝信 webhook
outbound_listen: ":8444"      # 代理 OpenClaw 出站 API 调用
openclaw_upstream: "http://127.0.0.1:18790"  # OpenClaw 地址
lanxin_upstream: "https://apigw.lx.qianxin.com"

# 管理
management_listen: ":9090"
management_token: "your-secret-management-token"

# 安全检测
inbound_detect_enabled: true
outbound_audit_enabled: true
detect_timeout_ms: 50
db_path: "./audit.db"
```

**多容器负载均衡模式：**

```yaml
# 静态上游容器
static_upstreams:
  - id: "openclaw-node-1"
    address: "10.0.1.10"
    port: 18790
  - id: "openclaw-node-2"
    address: "10.0.1.11"
    port: 18790
  - id: "openclaw-node-3"
    address: "10.0.1.12"
    port: 18790

# 路由策略：least-users（最少用户）或 round-robin（轮询）
route_default_policy: "least-users"
route_persist: true  # 路由持久化到 SQLite

# 动态服务注册（容器启动时自动注册）
registration_enabled: true
registration_token: "your-registration-token"
heartbeat_interval_sec: 10
heartbeat_timeout_count: 3
```

**出站规则配置：**

```yaml
outbound_rules:
  # 身份证号 → 拦截
  - name: "pii_id_card"
    pattern: '\d{17}[\dXx]'
    action: "block"

  # 手机号 → 告警
  - name: "pii_phone"
    pattern: '1[3-9]\d{9}'
    action: "warn"

  # API Key 泄露 → 拦截
  - name: "credential_apikey"
    patterns:
      - 'sk-[a-zA-Z0-9]{20,}'
      - 'AKIA[0-9A-Z]{16}'
      - 'ghp_[a-zA-Z0-9]{36}'
    action: "block"

  # 私钥泄露 → 拦截
  - name: "private_key_leak"
    patterns:
      - '-----BEGIN .* PRIVATE KEY-----'
    action: "block"

  # 系统提示词 → 告警
  - name: "system_prompt_leak"
    patterns:
      - 'SOUL\.md'
      - 'AGENTS\.md'
      - 'MEMORY\.md'
    action: "warn"

  # 恶意命令 → 拦截
  - name: "malicious_command"
    pattern: 'rm\s+-rf\s+/'
    action: "block"
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
```

---

## 🖥️ 管理后台

访问 `http://your-server:9090/` 即可打开管理后台。

| 功能 | 说明 |
|------|------|
| 📊 系统状态 | 版本、运行时间、上游健康比例（SVG 环形图），每 10 秒自动刷新 |
| 🔗 上游管理 | 容器列表，绿色脉冲 = 健康，红色闪烁 = 异常 |
| 🗺️ 路由表 | 用户→容器绑定关系，支持手动绑定和迁移 |
| 📈 统计面板 | 总请求/拦截/告警大数字 + CSS 柱状图分类展示 |
| 📋 审计日志 | 支持方向/动作/发送者筛选，block 红色高亮，warn 黄色高亮 |
| ⚙️ 规则管理 | 查看出站规则 + 一键热更新 |

Dashboard 是一个 **27KB 的单 HTML 文件**（gzip 后 7.3KB），零外部依赖，无需 npm/webpack。

---

## 🛡️ 安全检测能力

### 入站检测（40+ 规则）

基于 **Aho-Corasick 多模式匹配算法**，O(n) 时间复杂度扫描全文。

| 类别 | 检测内容 | 动作 |
|------|----------|------|
| Prompt Injection | `ignore previous instructions`, `忽略之前的指令`, `无视规则` | 🔴 Block |
| 越狱攻击 | `you are now DAN`, `jailbreak`, `没有限制的AI` | 🔴 Block |
| 系统提示词窃取 | `show system prompt`, `输出提示词`, `reveal instructions` | 🔴 Block |
| 命令注入 | `rm -rf /`, `curl\|bash`, `base64 -d\|bash` | 🔴 Block |
| 角色扮演诱导 | `假设你是`, `pretend you are`, `假装你没有限制` | 🟡 Warn |
| PII 检测 | 身份证号、手机号、银行卡号 | 🟡 Warn + 记录 |

### 出站检测（可配置规则）

| 动作 | 行为 | 适用场景 |
|------|------|----------|
| `block` | 拦截消息，返回 403 | 高危：身份证号、私钥、API Key |
| `warn` | 放行 + 告警日志 | 中危：手机号、系统文件名 |
| `log` | 放行 + 审计日志 | 低危、合规留痕 |

---

## 📡 API 参考

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 管理后台 Dashboard |
| GET | `/healthz` | 健康检查 + 系统概览 |

### 管理接口（需要 `management_token`）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams` | 列出所有上游容器 |
| GET | `/api/v1/routes` | 列出用户路由绑定 |
| POST | `/api/v1/routes/bind` | 绑定用户到上游 |
| POST | `/api/v1/routes/migrate` | 迁移用户到新上游 |
| POST | `/api/v1/rules/reload` | 热更新出站规则 |
| GET | `/api/v1/audit/logs` | 查询审计日志（支持筛选） |
| GET | `/api/v1/stats` | 统计概览 |

### 注册接口（需要 `registration_token`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/register` | 容器注册 |
| POST | `/api/v1/heartbeat` | 心跳上报 |
| POST | `/api/v1/deregister` | 容器注销 |

<details>
<summary>📝 API 调用示例</summary>

```bash
TOKEN="your-management-token"

# 健康检查
curl -s http://localhost:9090/healthz | jq .

# 列出上游
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/upstreams | jq .

# 绑定路由
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sender_id":"user-001","upstream_id":"node-1"}' \
  http://localhost:9090/api/v1/routes/bind | jq .

# 查询拦截日志
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9090/api/v1/audit/logs?action=block&limit=20" | jq .

# 统计
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/stats | jq .

# 热更新规则
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/rules/reload | jq .

# 容器注册
curl -s -X POST -H "Authorization: Bearer $REG_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":"node-4","address":"10.0.1.14","port":18790}' \
  http://localhost:9090/api/v1/register | jq .
```

</details>

---

## 📦 部署指南

### 方式一：直接运行（推荐快速试用）

```bash
# 编译
CGO_ENABLED=1 go build -o lobster-guard .

# 配置
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入蓝信凭据和上游地址

# 运行
./lobster-guard -config config.yaml
```

### 方式二：Systemd 服务（推荐生产部署）

```bash
# 安装到系统目录
sudo cp lobster-guard /usr/local/bin/
sudo mkdir -p /etc/lobster-guard /var/lib/lobster-guard
sudo cp config.yaml /etc/lobster-guard/
sudo cp dashboard.html /etc/lobster-guard/

# 创建 systemd service
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

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/lobster-guard
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# 启动
sudo systemctl daemon-reload
sudo systemctl start lobster-guard
sudo systemctl enable lobster-guard

# 查看日志
sudo journalctl -u lobster-guard -f
```

### 方式三：Docker

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=1 go build -o lobster-guard .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/lobster-guard /usr/local/bin/
COPY dashboard.html /etc/lobster-guard/
COPY config.yaml.example /etc/lobster-guard/config.yaml
EXPOSE 8443 8444 9090
CMD ["lobster-guard", "-config", "/etc/lobster-guard/config.yaml"]
```

```bash
docker build -t lobster-guard .
docker run -d -p 8443:8443 -p 8444:8444 -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/lobster-guard/config.yaml \
  lobster-guard
```

### 方式四：Make（推荐开发）

```bash
make build      # 编译
make test       # 运行测试（53 个用例）
make install    # 安装到系统
make healthz    # 检查健康状态
make stats      # 查看统计
make logs       # 查看审计日志
```

---

## 🧪 测试

```bash
# 运行全部测试（53 个用例，约 3.5 秒）
CGO_ENABLED=1 go test -v -count=1 ./...
```

### 测试覆盖

| 类别 | 用例数 | 内容 |
|------|--------|------|
| AC 自动机 | 6 | 基本匹配、大小写、中文、多模式、空输入 |
| 入站规则引擎 | 17 | block/warn/pass/PII/优先级 |
| 出站规则引擎 | 4 | block/warn/热更新/空规则 |
| 蓝信加解密 | 4 | 初始化/签名/加密解密全链路 |
| 消息提取 | 5 | 多种 JSON 格式 |
| 路由表 | 2 | CRUD/迁移/解绑 |
| 上游池 | 3 | 选择/注册注销/健康检查 |
| 审计日志 | 1 | 异步写入/查询/统计 |
| 管理 API | 5 | 鉴权/注册流程/路由/统计 |
| 数据库 | 2 | 初始化/幂等性 |
| **集成测试** | 7 | Mock 上游 + 加密 webhook 全链路 |
| **并发测试** | 3 | 20 goroutine 入站/出站/混合攻防 |
| **异常测试** | 2 | 无上游 502/蓝信宕机 503 |

---

## ⚡ 性能

| 指标 | 数值 |
|------|------|
| 检测延迟（P99） | < 5ms |
| 入站吞吐（单核） | > 5,000 req/s |
| 审计写入 | 异步，不阻塞请求 |
| 内存占用 | < 50MB |
| 二进制大小 | ~14MB |
| Dashboard 加载 | 7.3KB (gzip) |

- 规则引擎基于 **Aho-Corasick 算法**，O(n) 时间复杂度，文本长度无关
- SQLite **WAL 模式**，支持并发读写
- HTTP 连接池复用，减少 TCP 握手开销

---

## 📁 项目结构

```
lobster-guard/
├── main.go                 # 全部源码（~1650 行）
├── main_test.go            # 单元测试（33 用例）
├── integration_test.go     # 集成测试（20 用例）
├── dashboard.html          # 管理后台（27KB 单文件）
├── config.yaml.example     # 配置模板
├── Makefile                # 构建和管理命令
├── lobster-guard.service   # Systemd 服务文件
├── go.mod / go.sum         # Go 依赖
├── docs/
│   ├── design-v2.md        # 设计文档（17 章，79KB）
│   └── screenshots/        # 截图
└── LICENSE                 # MIT License
```

### 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/mattn/go-sqlite3` | SQLite 驱动 |
| `gopkg.in/yaml.v3` | YAML 配置解析 |

仅两个外部依赖，其余全部使用 Go 标准库。

---

## 🤖 OpenClaw Skill 集成

lobster-guard 提供了 OpenClaw Agent Skill，让 AI Agent 可以通过自然语言管理安全网关：

```
你：龙虾状态怎么样？
Agent：4 个上游全部健康，路由 5 条，已拦截 7 次攻击...

你：谁被拦截了？
Agent：最近拦截记录：attacker-001 (Prompt Injection)、attacker-002 (命令注入)...

你：把 user-123 迁移到 node-2
Agent：迁移成功 ✅
```

Skill 文件位于 `skills/lobster-guard/SKILL.md`。

---

## 📋 配置参考

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `callbackKey` | string | - | 蓝信回调加密密钥（必填） |
| `callbackSignToken` | string | - | 蓝信回调签名令牌（必填） |
| `inbound_listen` | string | `:8443` | 入站代理监听 |
| `outbound_listen` | string | `:8444` | 出站代理监听 |
| `management_listen` | string | `:9090` | 管理 API + Dashboard |
| `openclaw_upstream` | string | - | OpenClaw 上游地址（单机模式） |
| `lanxin_upstream` | string | - | 蓝信 API 地址 |
| `management_token` | string | - | 管理 API Token |
| `registration_token` | string | - | 容器注册 Token |
| `db_path` | string | `./audit.db` | SQLite 数据库路径 |
| `detect_timeout_ms` | int | `50` | 检测超时（毫秒） |
| `inbound_detect_enabled` | bool | `true` | 启用入站检测 |
| `outbound_audit_enabled` | bool | `true` | 启用出站审计 |
| `route_default_policy` | string | `least-users` | 路由策略 |
| `route_persist` | bool | `true` | 路由持久化到 SQLite |
| `heartbeat_interval_sec` | int | `10` | 心跳间隔 |
| `heartbeat_timeout_count` | int | `3` | 心跳超时次数 |
| `static_upstreams` | list | `[]` | 静态上游列表 |
| `outbound_rules` | list | `[]` | 出站检测规则 |
| `whitelist` | list | `[]` | 入站白名单 |

---

## 🗺️ Roadmap

- [ ] Rate limiting（请求限流）
- [ ] Prometheus metrics 导出
- [ ] WebSocket 支持
- [ ] 规则引擎可视化编辑
- [ ] 多租户隔离
- [ ] 入站规则热更新

---

## 📄 License

[MIT License](LICENSE)

---

<p align="center">
  <sub>🦞 Built with Go, secured with care.</sub>
</p>
