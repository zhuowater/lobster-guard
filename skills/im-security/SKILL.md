# 🦞 龙虾卫士 · IM 安全域 — v35.0

IM 消息安全网关的管理技能。覆盖入站/出站规则引擎、审计日志、路由亲和与策略、限流、用户信息及 WebSocket 代理的全部管理操作。
所有 API 调用需带 `Authorization: Bearer <token>` 头，基础 URL 为 `http://10.44.96.142:9090`。

---

## API 参考

### 规则管理

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/inbound-rules` | 入站规则列表（含版本信息） | — |
| POST | `/api/v1/inbound-rules/add` | 添加入站规则 | body: 规则定义 |
| PUT | `/api/v1/inbound-rules/update` | 更新入站规则 | body: 规则定义 |
| DELETE | `/api/v1/inbound-rules/delete` | 删除入站规则 | body: 规则 ID |
| POST | `/api/v1/inbound-rules/reload` | 热更新入站规则（重建 AC 自动机） | — |
| GET | `/api/v1/outbound-rules` | 出站规则列表 | — |
| POST | `/api/v1/outbound-rules/add` | 添加出站规则 | body: 规则定义 |
| PUT | `/api/v1/outbound-rules/update` | 更新出站规则 | body: 规则定义 |
| DELETE | `/api/v1/outbound-rules/delete` | 删除出站规则 | body: 规则 ID |
| POST | `/api/v1/outbound-rules/reload` | 热更新出站规则 | — |

> **v35.0 出站规则动作说明**：action 字段支持 `block`（拦截）/ `redact`（脱敏替换后放行，需 `replacement` 字段）/ `review`（人工审核）/ `warn`（告警放行）/ `log`（仅记录），优先级 block > redact > review > warn > log。
| POST | `/api/v1/rules/reload` | 热更新全部规则（入站+出站） | — |
| GET | `/api/v1/rules/hits` | 规则命中率统计 | — |
| POST | `/api/v1/rules/hits/reset` | 重置命中统计 | — |
| GET | `/api/v1/rules/export` | 导出规则（YAML 格式） | — |
| POST | `/api/v1/rules/import` | 导入规则 | body: YAML 规则文件 |
| GET | `/api/v1/rule-templates` | 规则模板列表 | — |
| GET | `/api/v1/rule-templates/detail` | 模板详情 | ?id= |
| GET | `/api/v1/rule-bindings` | 规则组绑定关系 | — |
| POST | `/api/v1/rule-bindings/test` | 测试规则组匹配 | body: 测试消息 |

### 审计日志

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/audit/logs` | 审计日志查询 | ?direction=&action=&sender_id=&app_id=&q=&trace_id=&limit=&offset=&since= |
| GET | `/api/v1/audit/export` | 导出审计日志 | ?format=csv\|json&since=&until= |
| POST | `/api/v1/audit/cleanup` | 清理过期日志 | — |
| GET | `/api/v1/audit/stats` | 审计统计（总数/磁盘/时间范围） | — |
| GET | `/api/v1/audit/timeline` | 时间线视图 | ?hours=24 |
| GET | `/api/v1/audit/archives` | 归档列表 | — |
| GET | `/api/v1/audit/archives/:name` | 下载归档文件 | :name = 归档名 |
| POST | `/api/v1/audit/archive` | 手动触发归档 | — |

### 路由管理

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/upstreams` | 上游容器列表 | — |
| GET | `/api/v1/routes` | 路由绑定列表 | ?app_id= |
| POST | `/api/v1/routes/bind` | 绑定路由 | body: sender_id, upstream_id, app_id? |
| POST | `/api/v1/routes/unbind` | 解绑路由 | body: sender_id |
| POST | `/api/v1/routes/migrate` | 迁移路由 | body: 迁移参数 |
| POST | `/api/v1/routes/batch-bind` | 批量绑定（按部门/列表） | body: 部门或用户列表 |
| GET | `/api/v1/routes/stats` | 路由统计 | — |
| GET | `/api/v1/route-policies` | 路由策略列表 | — |
| POST | `/api/v1/route-policies` | 创建路由策略 | body: 策略定义 |
| POST | `/api/v1/route-policies/test` | 测试策略匹配 | body: 测试条件 |
| PUT | `/api/v1/route-policies/:id` | 更新路由策略 | :id = 策略 ID |
| DELETE | `/api/v1/route-policies/:id` | 删除路由策略 | :id = 策略 ID |

### 用户信息

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/users` | 用户列表 | ?department=&email= |
| GET | `/api/v1/users/:id` | 用户详情 | :id = 用户 ID |
| POST | `/api/v1/users/:id/refresh` | 刷新单用户缓存 | :id = 用户 ID |
| POST | `/api/v1/users/refresh-all` | 全量刷新用户缓存 | — |

### 限流

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/rate-limit/stats` | 限流统计 | — |
| POST | `/api/v1/rate-limit/reset` | 重置限流计数器 | — |

### WebSocket

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/ws/connections` | 活跃 WebSocket 连接列表 | — |

### 执行审批 (v29.0)

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/upstreams/{id}/gateway/exec-approvals` | 待审批命令列表 | — |
| POST | `/api/v1/upstreams/{id}/gateway/exec-approvals/approve` | 批准执行 | body: approval id / decision |
| POST | `/api/v1/upstreams/{id}/gateway/exec-approvals/reject` | 拒绝执行 | body: approval id / reason |

### 统计概览

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/stats` | 全局统计概览 | — |
| GET | `/api/v1/metrics/realtime` | 实时指标（最近 60 秒） | — |

---

## 意图映射

| 用户说 | 调用 |
|--------|------|
| 查看入站规则 / 列出入站规则 | `GET /api/v1/inbound-rules` |
| 查看出站规则 / 列出出站规则 | `GET /api/v1/outbound-rules` |
| 添加入站规则 | `POST /api/v1/inbound-rules/add` |
| 添加出站规则 | `POST /api/v1/outbound-rules/add` |
| 添加 redact 脱敏规则 | `POST /api/v1/outbound-rules/add` body: `{action:"redact", replacement:"[REDACTED]", ...}` |
| 更新入站规则 | `PUT /api/v1/inbound-rules/update` |
| 更新出站规则 | `PUT /api/v1/outbound-rules/update` |
| 删除入站规则 | `DELETE /api/v1/inbound-rules/delete` |
| 删除出站规则 | `DELETE /api/v1/outbound-rules/delete` |
| 重载规则 / 刷新规则 / 热更新 | `POST /api/v1/rules/reload` |
| 重载入站规则 | `POST /api/v1/inbound-rules/reload` |
| 重载出站规则 | `POST /api/v1/outbound-rules/reload` |
| 规则命中率 / 哪些规则被触发了 | `GET /api/v1/rules/hits` |
| 重置命中统计 | `POST /api/v1/rules/hits/reset` |
| 导出规则 | `GET /api/v1/rules/export` |
| 导入规则 | `POST /api/v1/rules/import` |
| 规则模板 / 有哪些模板 | `GET /api/v1/rule-templates` |
| 规则绑定 / 绑定关系 | `GET /api/v1/rule-bindings` |
| 测试规则匹配 | `POST /api/v1/rule-bindings/test` |
| 查看审计日志 / 最近的日志 | `GET /api/v1/audit/logs` |
| 搜索日志 / 查某人的日志 | `GET /api/v1/audit/logs?sender_id=&q=` |
| 导出审计日志 | `GET /api/v1/audit/export?format=csv` |
| 清理日志 | `POST /api/v1/audit/cleanup` |
| 审计统计 / 日志量 | `GET /api/v1/audit/stats` |
| 日志时间线 | `GET /api/v1/audit/timeline` |
| 归档日志 / 手动归档 | `POST /api/v1/audit/archive` |
| 归档列表 | `GET /api/v1/audit/archives` |
| 下载归档 | `GET /api/v1/audit/archives/:name` |
| 查看上游 / 容器列表 | `GET /api/v1/upstreams` |
| 查看路由 / 路由绑定 | `GET /api/v1/routes` |
| 绑定路由 / 把用户绑到某容器 | `POST /api/v1/routes/bind` |
| 解绑路由 | `POST /api/v1/routes/unbind` |
| 迁移路由 | `POST /api/v1/routes/migrate` |
| 批量绑定 / 按部门绑定 | `POST /api/v1/routes/batch-bind` |
| 路由统计 | `GET /api/v1/routes/stats` |
| 路由策略 / 策略列表 | `GET /api/v1/route-policies` |
| 创建路由策略 | `POST /api/v1/route-policies` |
| 测试路由策略 | `POST /api/v1/route-policies/test` |
| 查看用户 / 用户列表 | `GET /api/v1/users` |
| 查看用户详情 | `GET /api/v1/users/:id` |
| 刷新用户信息 | `POST /api/v1/users/:id/refresh` |
| 全量刷新用户 | `POST /api/v1/users/refresh-all` |
| 限流统计 / 限流情况 | `GET /api/v1/rate-limit/stats` |
| 重置限流 | `POST /api/v1/rate-limit/reset` |
| WS 连接 / WebSocket 连接数 | `GET /api/v1/ws/connections` |
| 整体统计 / 概览 / 仪表盘 | `GET /api/v1/stats` |
| 实时数据 / 实时指标 | `GET /api/v1/metrics/realtime` |
| 查看待审批执行命令 | `GET /api/v1/upstreams/{id}/gateway/exec-approvals` |
| 批准执行命令 | `POST /api/v1/upstreams/{id}/gateway/exec-approvals/approve` |
| 拒绝执行命令 | `POST /api/v1/upstreams/{id}/gateway/exec-approvals/reject` |

> Phase 1 新功能请见对应子技能：执行信封 `../envelope/`、事件总线 `../event-bus/`、自进化 `../evolution/`、语义检测 `../semantic/`、奇点蜜罐 `../singularity/`、工具策略 `../tool-policy/`、污染追踪 `../taint/`、响应缓存 `../cache/`、API网关 `../gateway/`
> v29.0 新功能：执行审批（exec-approvals）+ Gateway WSS RPC 远程管理 — 详见 `../gateway/SKILL.md`
