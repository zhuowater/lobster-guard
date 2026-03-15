# 🦞 lobster-guard（龙虾卫士）v2.0

AI Agent 安全网关，为蓝信 + OpenClaw AI Agent 提供：

- **入站安全检测** — Prompt Injection 拦截 + PII 检测
- **出站内容拦截** — block/warn/log 三级策略，防止敏感信息泄露
- **用户ID亲和路由** — 多容器负载均衡，用户会话粘滞
- **服务自动注册** — 容器启动自动注册，心跳保活，故障转移
- **全量审计日志** — SQLite 持久化，支持查询和统计

## 架构

```
蓝信平台 → lobster-guard(:8443) → [路由表] → OpenClaw 容器池
                                                    │
OpenClaw 出站 → lobster-guard(:8444) → [出站检测] → 蓝信API
                                                    
管理/注册 → lobster-guard(:9090) → 容器注册/心跳/管理API
```

### 单机模式（v1.0 兼容）

如果没有配置 `static_upstreams` 和服务注册，自动退化为单上游模式：

```
蓝信 → :8443 → OpenClaw(:18790)
OpenClaw → :8444 → 蓝信API
```

## 快速开始

### 编译

```bash
# 需要 Go 1.21+ 和 CGO（SQLite 需要）
make build
```

### 运行

```bash
# 编辑配置文件
cp config.yaml.example config.yaml
vim config.yaml

# 运行
./lobster-guard -config config.yaml
```

### 系统安装

```bash
sudo make install
sudo systemctl start lobster-guard
sudo systemctl enable lobster-guard
journalctl -u lobster-guard -f
```

## 配置说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `callbackKey` | 蓝信回调加密密钥 | - |
| `callbackSignToken` | 蓝信回调签名令牌 | - |
| `inbound_listen` | 入站代理监听地址 | `:8443` |
| `outbound_listen` | 出站代理监听地址 | `:8444` |
| `management_listen` | 管理API监听地址 | `:9090` |
| `management_token` | 管理API认证Token | - |
| `registration_enabled` | 启用服务注册 | `true` |
| `registration_token` | 注册认证Token | - |
| `heartbeat_interval_sec` | 心跳间隔（秒） | `10` |
| `heartbeat_timeout_count` | 心跳超时次数 | `3` |
| `route_default_policy` | 路由策略 | `least-users` |
| `route_persist` | 路由持久化 | `true` |
| `db_path` | 审计数据库路径 | `/var/lib/lobster-guard/audit.db` |
| `detect_timeout_ms` | 检测超时（毫秒） | `50` |

## 出站规则

支持三种 action：

| Action | 行为 | 适用场景 |
|--------|------|----------|
| `block` | 拦截消息，返回 403 | 高危泄露（身份证、私钥、API Key） |
| `warn` | 放行消息 + 告警日志 | 中危（手机号、系统提示词提及） |
| `log` | 放行消息 + 审计日志 | 低危、默认策略 |

内置规则：
- **PII 泄露** — 身份证号(block)、银行卡号(block)
- **凭据泄露** — API Key `sk-xxx`(block)、GitHub Token `ghp_xxx`(block)
- **私钥泄露** — `-----BEGIN PRIVATE KEY-----`(block)
- **系统提示词** — SOUL.md / AGENTS.md / MEMORY.md(warn)
- **恶意命令** — `rm -rf /`、`curl|bash`(block)

## 管理 API

所有管理接口需要 Bearer Token 认证。

### 服务注册（容器调用）

```bash
# 注册
curl -X POST http://localhost:9090/api/v1/register \
  -H "Authorization: Bearer container-register-token" \
  -H "Content-Type: application/json" \
  -d '{"id":"openclaw-01","address":"172.20.0.2","port":18790}'

# 心跳
curl -X POST http://localhost:9090/api/v1/heartbeat \
  -H "Authorization: Bearer container-register-token" \
  -H "Content-Type: application/json" \
  -d '{"id":"openclaw-01","load":{"cpu_percent":35.2}}'

# 注销
curl -X POST http://localhost:9090/api/v1/deregister \
  -H "Authorization: Bearer container-register-token" \
  -d '{"id":"openclaw-01"}'
```

### 管理操作

```bash
# 健康检查（无需认证）
curl http://localhost:9090/healthz

# 列出上游容器
curl -H "Authorization: Bearer your-management-token" \
  http://localhost:9090/api/v1/upstreams

# 列出路由绑定
curl -H "Authorization: Bearer your-management-token" \
  http://localhost:9090/api/v1/routes

# 手动绑定用户到容器
curl -X POST -H "Authorization: Bearer your-management-token" \
  -H "Content-Type: application/json" \
  http://localhost:9090/api/v1/routes/bind \
  -d '{"sender_id":"user-123","upstream_id":"openclaw-01"}'

# 迁移用户
curl -X POST -H "Authorization: Bearer your-management-token" \
  -H "Content-Type: application/json" \
  http://localhost:9090/api/v1/routes/migrate \
  -d '{"sender_id":"user-123","from":"openclaw-01","to":"openclaw-02"}'

# 热更新规则
curl -X POST -H "Authorization: Bearer your-management-token" \
  http://localhost:9090/api/v1/rules/reload

# 查询审计日志
curl -H "Authorization: Bearer your-management-token" \
  "http://localhost:9090/api/v1/audit/logs?direction=outbound&action=block&limit=10"

# 统计概览
curl -H "Authorization: Bearer your-management-token" \
  http://localhost:9090/api/v1/stats
```

## 数据库

### 审计日志表

```sql
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    direction TEXT NOT NULL,     -- 'inbound' / 'outbound'
    sender_id TEXT,
    action TEXT NOT NULL,        -- 'pass' / 'block' / 'warn' / 'log'
    reason TEXT,
    content_preview TEXT,
    full_request_hash TEXT,
    latency_ms REAL,
    upstream_id TEXT             -- v2.0 新增：路由到的上游容器
);
```

### 上游容器表

```sql
CREATE TABLE upstreams (
    id TEXT PRIMARY KEY,
    address TEXT NOT NULL,
    port INTEGER NOT NULL,
    healthy INTEGER DEFAULT 1,
    registered_at TEXT NOT NULL,
    last_heartbeat TEXT,
    tags TEXT DEFAULT '{}',
    load TEXT DEFAULT '{}'
);
```

### 用户路由表

```sql
CREATE TABLE user_routes (
    sender_id TEXT PRIMARY KEY,
    upstream_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

## 查看日志

```bash
# 最近审计日志
make logs

# 统计
make stats

# 健康检查
make healthz

# 查看上游
make upstreams

# 查看路由
make routes
```

## 设计原则

1. **默认透传** — 不认识的接口零修改直传，兼容未来新接口
2. **failopen** — 检测异常不阻塞，宁可漏检不可误杀
3. **入站拦截** — 只拦截高危 Prompt Injection
4. **出站分级** — block/warn/log 按规则精确控制
5. **异步日志** — 审计不影响请求延迟
6. **用户亲和** — 按用户 ID 路由，保持会话上下文
7. **最小依赖** — Go 标准库 + SQLite，单二进制部署

## 性能

- 规则引擎基于 Aho-Corasick 算法，O(n) 时间复杂度
- 审计日志异步写入，不阻塞请求处理
- SQLite WAL 模式，支持并发读写
- HTTP 连接池复用，减少握手开销
- 代理附加延迟 < 5ms (P99)

## License

Internal use only - 奇安信内部使用
