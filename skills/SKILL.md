# 🦞 Lobster Guard Skill — 龙虾卫士智能管控

通过自然语言与 lobster-guard 安全网关交互，实现 AI Agent 对安全网关的智能管控。

## 概述

本 Skill 使 OpenClaw Agent 能够通过自然语言管理 lobster-guard 安全网关，包括：
- 查看系统状态和健康检查
- 管理上游容器（列出、注册、注销）
- 管理用户路由（查看、绑定、迁移、批量操作）
- 查询用户信息（姓名、邮箱、手机、部门 — 自动从 IM 获取）
- 管理路由策略（基于邮箱/部门/Bot 的策略匹配）
- 查询和分析审计日志
- 查看安全统计和规则命中率
- 热更新入站/出站规则
- 查看限流统计
- 查看 Prometheus 指标

## 配置

在使用前需要设置环境变量或在调用时指定：

```bash
# 龙虾卫士管理 API 地址（默认本机）
LOBSTER_GUARD_URL="http://127.0.0.1:9090"

# 管理 Token
LOBSTER_GUARD_TOKEN="your-management-token"

# 注册 Token（仅服务注册相关操作需要）
LOBSTER_GUARD_REG_TOKEN="your-registration-token"
```

## API 端点参考

### 公开端点（无需认证）

| 端点 | 说明 |
|------|------|
| `GET /healthz` | 健康检查 + 系统概览 |
| `GET /metrics` | Prometheus 格式指标导出 |

### 管理端点（需要 management_token）

| 端点 | 说明 |
|------|------|
| `GET /api/v1/upstreams` | 列出上游容器 |
| `GET /api/v1/routes` | 列出路由绑定（支持 ?app_id 筛选）|
| `POST /api/v1/routes/bind` | 绑定用户到上游（支持 app_id/display_name/department）|
| `POST /api/v1/routes/unbind` | 解绑用户路由 |
| `POST /api/v1/routes/migrate` | 迁移用户到新上游 |
| `POST /api/v1/routes/batch-bind` | 批量绑定（按部门/按列表）|
| `GET /api/v1/routes/stats` | 路由统计（按 Bot/上游/部门分布）|
| `GET /api/v1/users` | 用户信息列表（支持 ?department/?email 筛选）|
| `GET /api/v1/users/:id` | 单个用户详情（姓名/邮箱/手机/部门）|
| `POST /api/v1/users/:id/refresh` | 强制刷新用户信息（从 IM API 重新获取）|
| `POST /api/v1/users/refresh-all` | 全量刷新所有用户信息 |
| `GET /api/v1/route-policies` | 列出路由策略 |
| `POST /api/v1/route-policies/test` | 测试策略匹配（输入邮箱/部门/app_id）|
| `GET /api/v1/inbound-rules` | 列出入站规则 + 版本信息 |
| `POST /api/v1/inbound-rules/reload` | 热更新入站规则（重建 AC 自动机）|
| `GET /api/v1/outbound-rules` | 列出出站规则 |
| `POST /api/v1/rules/reload` | 热更新出站规则 |
| `GET /api/v1/rules/hits` | 规则命中率排行 |
| `POST /api/v1/rules/hits/reset` | 重置命中统计 |
| `GET /api/v1/audit/logs` | 审计日志（支持 direction/action/sender_id 筛选）|
| `GET /api/v1/stats` | 统计概览 |
| `GET /api/v1/rate-limit/stats` | 限流统计 |
| `POST /api/v1/rate-limit/reset` | 重置限流计数器 |

### 注册端点（需要 registration_token）

| 端点 | 说明 |
|------|------|
| `POST /api/v1/register` | 容器注册 |
| `POST /api/v1/heartbeat` | 心跳上报 |
| `POST /api/v1/deregister` | 容器注销 |

## 意图映射

| 用户说 | 调用 |
|--------|------|
| "龙虾状态" "网关怎么样" | `GET /healthz` |
| "上游节点" "有哪些容器" | `GET /api/v1/upstreams` |
| "路由表" "谁绑在哪" | `GET /api/v1/routes` |
| "把 user-123 绑到 node-1" | `POST /api/v1/routes/bind` |
| "批量绑定安全研究院的人" | `POST /api/v1/routes/batch-bind` |
| "路由统计" "各 Bot 多少人" | `GET /api/v1/routes/stats` |
| "用户列表" "都有谁" | `GET /api/v1/users` |
| "张三的信息" | `GET /api/v1/users/:id` |
| "刷新张三的信息" | `POST /api/v1/users/:id/refresh` |
| "路由策略" "怎么分配的" | `GET /api/v1/route-policies` |
| "测试策略" "zhangzhuo@qianxin.com 会走哪" | `POST /api/v1/route-policies/test` |
| "谁被拦截了" "最近的攻击" | `GET /api/v1/audit/logs?action=block` |
| "统计" "多少请求" | `GET /api/v1/stats` |
| "更新规则" "刷新规则" | `POST /api/v1/inbound-rules/reload` + `POST /api/v1/rules/reload` |
| "哪条规则命中最多" | `GET /api/v1/rules/hits` |
| "限流情况" | `GET /api/v1/rate-limit/stats` |
| "安全报告" "全面分析" | 依次调用 healthz + stats + audit/logs + rules/hits + rate-limit/stats，综合分析 |

## 使用 CLI 工具

本 Skill 附带 `lobster-cli.sh` 命令行工具：

```bash
export LOBSTER_GUARD_URL="http://127.0.0.1:9090"
export LOBSTER_GUARD_TOKEN="your-token"

./lobster-cli.sh help             # 查看帮助
./lobster-cli.sh status           # 健康检查
./lobster-cli.sh upstreams        # 上游容器
./lobster-cli.sh routes           # 路由表
./lobster-cli.sh logs             # 审计日志
./lobster-cli.sh blocks           # 拦截记录
./lobster-cli.sh stats            # 统计
./lobster-cli.sh inbound-rules    # 入站规则
./lobster-cli.sh outbound-rules   # 出站规则
./lobster-cli.sh rule-hits        # 规则命中率
./lobster-cli.sh rate-limit       # 限流统计
./lobster-cli.sh metrics          # Prometheus 指标
./lobster-cli.sh reload           # 热更新全部规则
./lobster-cli.sh report           # 综合安全报告
```
