# 🚪 Gateway 远程管理 · v33.0

JWT + APIKey 双认证，路由优先级匹配，请求头变换，灰度发布。v33.0 将原 Gateway Monitor 升级为 **WSS RPC 远程管理**：通过持久化 WSS 连接优先直连上游 OpenClaw Gateway，失败时自动 fallback 到 `tools/invoke`。

> 连接配置见 `../SKILL.md`

## API — 网关核心

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway/stats` | 网关统计 |
| GET | `/api/v1/gateway/routes` | 路由列表 |
| POST | `/api/v1/gateway/routes` | 添加路由 |
| PUT | `/api/v1/gateway/routes/:id` | 更新路由 |
| DELETE | `/api/v1/gateway/routes/:id` | 删除路由 |
| POST | `/api/v1/gateway/token` | 生成 JWT |
| POST | `/api/v1/gateway/validate` | 验证 JWT |
| GET | `/api/v1/gateway/log?limit=50` | 网关日志 |
| GET/PUT | `/api/v1/gateway/config` | 网关配置 |

## API — WSS RPC 远程管理 (v29.0)

通过持久化 WSS RPC 连接管理上游 OpenClaw Gateway，协议流程复刻 Control UI：`challenge → connect → hello → ready`。默认优先走 WSS，若目标未建立 WSS 或握手失败，则自动回退到 `POST /api/v1/tools/invoke`。

### 核心机制

- 持久化 WSS RPC 连接，复刻 OpenClaw Control UI 协议：`challenge → connect → hello → ready`
- WSS 优先，`tools/invoke` 自动 fallback
- 典型响应延迟降低到 **11–44ms**（原 `tools/invoke` 为 **78–184ms**）
- 支持全局 `default_gateway_origin`，并允许每个上游通过 `gateway_origin` 单独覆盖

### Gateway 管理端点总览（59 个）

#### Session 管理（7）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/sessions` | 会话列表 |
| GET | `/api/v1/upstreams/{id}/gateway/session-history?key=xxx` | 聊天历史 |
| PATCH | `/api/v1/upstreams/{id}/gateway/session` | 修改会话 |
| DELETE | `/api/v1/upstreams/{id}/gateway/session?key=xxx` | 删除会话 |
| POST | `/api/v1/upstreams/{id}/gateway/session/reset` | 重置会话 |
| POST | `/api/v1/upstreams/{id}/gateway/session/compact` | 压缩上下文 |
| GET | `/api/v1/upstreams/{id}/gateway/status` | Gateway 状态 |

#### Chat 操作（2）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/upstreams/{id}/gateway/chat/send` | 发消息触发 Agent |
| POST | `/api/v1/upstreams/{id}/gateway/chat/abort` | 中止生成 |

#### Cron CRUD（6）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/cron` | 定时任务列表 |
| POST | `/api/v1/upstreams/{id}/gateway/cron/add` | 创建 |
| PUT | `/api/v1/upstreams/{id}/gateway/cron/update` | 更新 |
| DELETE | `/api/v1/upstreams/{id}/gateway/cron/remove` | 删除 |
| POST | `/api/v1/upstreams/{id}/gateway/cron/run` | 立即运行 |
| GET | `/api/v1/upstreams/{id}/gateway/cron/runs?id=xxx` | 运行历史 |

#### Agent 管理（7）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/agents` | Agent 列表 |
| POST | `/api/v1/upstreams/{id}/gateway/agents/create` | 创建 |
| PUT | `/api/v1/upstreams/{id}/gateway/agents/update` | 更新 |
| DELETE | `/api/v1/upstreams/{id}/gateway/agents/delete?id=xxx` | 删除 |
| GET | `/api/v1/upstreams/{id}/gateway/agents/files?agentId=xxx` | 文件列表 |
| GET | `/api/v1/upstreams/{id}/gateway/agents/file?agentId=xxx&name=xxx` | 获取文件 |
| PUT | `/api/v1/upstreams/{id}/gateway/agents/file` | 保存文件 |

#### 执行审批（3）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/exec-approvals` | 待审批列表 |
| POST | `/api/v1/upstreams/{id}/gateway/exec-approvals/approve` | 批准 |
| POST | `/api/v1/upstreams/{id}/gateway/exec-approvals/reject` | 拒绝 |

#### Config（3）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/config` | 获取配置 |
| PATCH | `/api/v1/upstreams/{id}/gateway/config` | 部分修改（需 `baseHash`） |
| GET | `/api/v1/upstreams/{id}/gateway/config/schema` | 配置 Schema |

#### Skills（5）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/skills` | Skill 列表 |
| GET | `/api/v1/upstreams/{id}/gateway/skills/bins` | 可用 Skill 仓库 |
| POST | `/api/v1/upstreams/{id}/gateway/skills/install` | 安装 |
| POST | `/api/v1/upstreams/{id}/gateway/skills/update` | 更新 |
| POST | `/api/v1/upstreams/{id}/gateway/skills/uninstall` | 卸载 |

#### Gateway 控制（2）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/upstreams/{id}/gateway/restart` | 重启 Gateway |
| POST | `/api/v1/upstreams/{id}/gateway/update` | 自更新 |

#### 心跳 / 设备 / 节点（12）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/heartbeat` | 心跳状态 |
| PUT | `/api/v1/upstreams/{id}/gateway/heartbeat` | 设置心跳 |
| POST | `/api/v1/upstreams/{id}/gateway/wake` | 唤醒 Agent |
| GET | `/api/v1/upstreams/{id}/gateway/devices` | 设备列表 |
| POST | `/api/v1/upstreams/{id}/gateway/devices/approve` | 批准配对 |
| POST | `/api/v1/upstreams/{id}/gateway/devices/reject` | 拒绝配对 |
| GET | `/api/v1/upstreams/{id}/gateway/node-pairs` | 节点配对 |
| POST | `/api/v1/upstreams/{id}/gateway/node-pairs/approve` | 批准 |
| POST | `/api/v1/upstreams/{id}/gateway/node-pairs/reject` | 拒绝 |
| GET | `/api/v1/upstreams/{id}/gateway/nodes` | 节点列表 |
| POST | `/api/v1/upstreams/{id}/gateway/nodes/describe` | 节点详情 |
| POST | `/api/v1/upstreams/{id}/gateway/nodes/rename` | 重命名 |

#### 记忆（1）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/upstreams/{id}/gateway/memory/search` | 记忆搜索 |

#### 系统（6）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/{id}/gateway/ping` | Ping |
| GET | `/api/v1/upstreams/{id}/gateway/models` | 模型列表 |
| GET | `/api/v1/upstreams/{id}/gateway/channels` | 渠道状态 |
| GET | `/api/v1/upstreams/{id}/gateway/logs` | 日志 |
| GET | `/api/v1/upstreams/{id}/gateway/usage` | 用量 |
| POST | `/api/v1/upstreams/{id}/gateway/system-event` | 系统事件 |

#### WSS 状态（1）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway/wss/status` | WSS 连接状态（全局） |


### 上游配置 / 注册接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway-monitor/upstreams` | 已注册的上游 Gateway 列表 |
| POST | `/api/v1/gateway-monitor/upstreams` | 添加上游 Gateway |
| PUT | `/api/v1/gateway-monitor/upstreams/:id` | 更新上游配置（含 Gateway Token / Origin） |
| DELETE | `/api/v1/gateway-monitor/upstreams/:id` | 删除上游 Gateway |
| POST | `/api/v1/tools/invoke` | fallback 通道：调用上游 Gateway 的 `tools/invoke` |

## API — Agent Operations Center · AOC (v29.0)

AOC 从原来的 5 视图扩展为 **8 个子视图**：仪表盘、卡片、协作、用户、Skills、文件、心跳、记忆。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/aoc/dashboard` | AOC 仪表盘总览 |
| GET | `/api/v1/aoc/cards` | Agent 卡片视图 |
| GET | `/api/v1/aoc/collab` | 协作视图 |
| GET | `/api/v1/aoc/users` | 用户视图 |
| GET | `/api/v1/aoc/skills` | Skills 视图 |
| GET | `/api/v1/aoc/files` | 文件视图 |
| GET | `/api/v1/aoc/heartbeat` | 心跳视图 |
| GET | `/api/v1/aoc/memory` | 记忆视图 |

### Per-Upstream AOC

每个上游 Gateway 在展开行中有独立的 Agent 标签页，提供该上游的 AOC 数据：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc` | 上游专属 AOC |
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc/cards` | 上游 Agent 卡片 |
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc/skills` | 上游技能目录 |
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc/files` | 上游文件视图 |
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc/heartbeat` | 上游心跳视图 |
| GET | `/api/v1/gateway-monitor/upstreams/:id/aoc/memory` | 上游记忆视图 |

## API — Skill Directory (v29.0)

从 OpenClaw 文件系统扫描并展示已安装的技能：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/skills/directory` | 技能目录列表 |
| GET | `/api/v1/skills/directory/:slug` | 技能详情 |
| POST | `/api/v1/skills/directory/scan` | 手动触发技能扫描 |

## JWT 生成示例

```bash
curl -s -H "Authorization: Bearer $TOKEN"   -X POST $URL/api/v1/gateway/token   -H "Content-Type: application/json"   -d '{"tenant_id": "team-a", "role": "admin", "expires_hours": 24}'
# → token: "eyJ..."
```

## tools/invoke 协议示例

```bash
# fallback: 调用上游 Gateway 获取 Agent 状态
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json"   -d '{"tool": "agent_status", "params": {"upstream_id": 1}}'   $URL/api/v1/tools/invoke
# → { "agents": [...], "status": "healthy" }
```

## 配置

```yaml
api_gateway:
  enabled: true
  jwt_enabled: true
  jwt_secret: "your-jwt-secret"
  apikey_enabled: true
  api_keys:
    - "your-api-key"

# 全局默认 Origin（同机部署用 http://localhost）
default_gateway_origin: "http://localhost"

static_upstreams:
  - id: "openclaw-local"
    address: "127.0.0.1"
    port: 19444
    gateway_token: "your-token"
    # gateway_origin: "https://domain.com"  # 可选覆盖

# OpenClaw 端需要配置：
# gateway.controlUi.allowInsecureAuth: true
# gateway.controlUi.allowedOrigins: ["http://localhost"]
```
