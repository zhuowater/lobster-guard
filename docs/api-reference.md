# 📡 API 参考（~227 路由）

> 返回 [README](../README.md)

## 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 管理后台 Dashboard |
| GET | `/healthz` | 健康检查 + 系统概览 |
| GET | `/metrics` | Prometheus 指标导出 |
| POST | `/api/v1/auth/login` | JWT 登录 |

## IM 安全域

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

## LLM 安全域

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/llm/overview` | LLM 代理概览 |
| GET | `/api/v1/llm/rules` | LLM 规则列表（11 默认 + 自定义）|
| POST | `/api/v1/llm/rules` | LLM 规则 CRUD |
| GET | `/api/v1/llm/cost` | Token 成本看板 |
| GET | `/api/v1/llm/audit` | LLM 审计日志 |

## 威胁分析

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

## 安全治理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/reports` | 安全报告列表 |
| POST | `/api/v1/reports/generate` | 生成报告（日报/周报/合规）|
| GET | `/api/v1/reports/:id/pdf` | 导出 PDF |
| GET | `/api/v1/leaderboard` | 安全排行榜 |
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants` | 创建租户 |

## 高级功能

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

## 系统管理

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

> 以上为主要 API 摘要，完整 ~227 路由参见源码 `src/api.go`。

## API 调用示例

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
