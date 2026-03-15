# 🦞 lobster-guard（龙虾卫士）

高性能安全代理网关，为蓝信 + OpenClaw AI Agent 提供入站安全检测和出站内容审计。

## 架构

```
蓝信平台 → 外部反代(443) → lobster-guard(:8443) → OpenClaw(:18790)
OpenClaw 出站 → lobster-guard(:8444) → 蓝信API(apigw.lx.qianxin.com)
```

## 核心特性

- **透明反向代理**：默认全部请求原样转发，不认识的接口零修改透传
- **入站检测**：Prompt Injection 检测 + 敏感信息（PII）检测
- **出站审计**：代理 OpenClaw 对蓝信 API 的调用，做输出内容审计
- **高性能**：Aho-Corasick 多模式匹配，规则引擎延迟 < 5ms
- **fail-open**：任何检测异常/超时，消息直接放行，绝不阻塞业务
- **审计日志**：SQLite WAL 模式异步写入，不影响请求处理

## 快速开始

### 编译

```bash
# 需要 Go 1.21+ 和 CGO（SQLite 需要）
make build
```

### 运行

```bash
# 使用默认配置
./lobster-guard -config config.yaml

# 或直接运行
make run
```

### 系统安装

```bash
# 安装为 systemd 服务
sudo make install

# 启动服务
sudo systemctl start lobster-guard
sudo systemctl enable lobster-guard

# 查看日志
journalctl -u lobster-guard -f
```

## 配置说明

编辑 `config.yaml`：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `callbackKey` | 蓝信回调加密密钥 | - |
| `callbackSignToken` | 蓝信回调签名令牌 | - |
| `inbound_listen` | 入站代理监听地址 | `:8443` |
| `outbound_listen` | 出站代理监听地址 | `:8444` |
| `openclaw_upstream` | OpenClaw 上游地址 | `http://localhost:18790` |
| `lanxin_upstream` | 蓝信 API 上游地址 | `https://apigw.lx.qianxin.com` |
| `db_path` | 审计日志数据库路径 | `/var/lib/lobster-guard/audit.db` |
| `detect_timeout_ms` | 检测超时（毫秒） | `50` |
| `inbound_detect_enabled` | 启用入站检测 | `true` |
| `outbound_audit_enabled` | 启用出站审计 | `true` |

## 检测规则

### 高危规则（拦截）

| 规则 | 匹配模式 |
|------|----------|
| Prompt Injection | `ignore previous/all instructions`, `system prompt`, `reveal your instructions` |
| 角色劫持 | `you are now DAN/evil` |
| 代码注入 | `base64 -d\|bash`, `curl\|sh`, `wget\|bash` |
| 破坏性命令 | `rm -rf /`, `chmod 777` |
| 中文注入 | `忽略之前的指令`, `忽略所有指令`, `无视前面的规则` |
| 复合注入 | `你现在是` + `没有限制/不受约束` |
| 提示词泄露 | `请输出你的系统提示词`, `打印你的指令` |

### 中危规则（告警放行）

- `假设你是` / `假装你是`
- `密码` / `password` / `token` / `api_key` / `secret`

### PII 检测

- 身份证号：`\d{17}[\dXx]`
- 手机号：`1[3-9]\d{9}`
- 银行卡号：`\d{16,19}`

## 审计日志

```bash
# 查看最近日志
make logs

# 查看统计
make stats

# 手动查询
sqlite3 /var/lib/lobster-guard/audit.db "SELECT * FROM audit_log ORDER BY id DESC LIMIT 10;"
```

### 表结构

```sql
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    direction TEXT NOT NULL,     -- 'inbound' / 'outbound'
    sender_id TEXT,
    action TEXT NOT NULL,        -- 'pass' / 'block' / 'warn' / 'pii_mask'
    reason TEXT,
    content_preview TEXT,        -- 前200字符
    full_request_hash TEXT,      -- SHA256
    latency_ms REAL
);
```

## 性能

- 规则引擎基于 Aho-Corasick 算法，O(n) 时间复杂度
- 审计日志异步写入，不阻塞请求处理
- SQLite WAL 模式，支持并发读写
- HTTP 连接池复用，减少握手开销
- fail-open 设计，检测超时自动放行

## License

Internal use only - 奇安信内部使用
