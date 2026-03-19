# 🔀 上游管理

> 返回 [README](../README.md) | 相关: [K8s 服务发现](k8s-discovery.md) · [配置参考](configuration.md) · [部署指南](deployment.md)

## 概述

上游（Upstream）是龙虾卫士转发请求的目标——通常是 OpenClaw 实例。支持四种注册方式：

| 来源 | 说明 | 删除 |
|------|------|------|
| **静态** (static) | config.yaml 中 `static_upstreams` 定义 | 不可删除 |
| **API** (api) | 通过 REST API 手动注册 | 可删除 |
| **K8s** (k8s) | K8s 服务发现自动注册 | 可删除（下次 sync 会恢复） |
| **兼容** (legacy) | 旧版 `openclaw_upstream` 字段 | 不可删除 |

## 配置

### 静态上游

```yaml
static_upstreams:
  - id: "openclaw-1"
    address: "10.0.1.10"
    port: 18789
    tags:
      region: "cn-north"
      env: "production"
  - id: "openclaw-2"
    address: "10.0.1.11"
    port: 18789
    tags:
      region: "cn-south"
      env: "production"
```

### 兼容模式

如果不配 `static_upstreams`，从 `openclaw_upstream` 自动创建一个默认上游：

```yaml
openclaw_upstream: "http://127.0.0.1:18789"
# → 自动注册为 id="openclaw-default", source="legacy"
```

## REST API

### 列表上游

```
GET /api/v1/upstreams
Authorization: Bearer <token>
```

响应：
```json
{
  "upstreams": [
    {
      "id": "openclaw-default",
      "address": "127.0.0.1",
      "port": 18789,
      "status": "healthy",
      "source": "legacy",
      "bound_users": 2,
      "last_heartbeat": "2026-03-19T05:00:00Z",
      "tags": {}
    }
  ]
}
```

### 添加上游

```
POST /api/v1/upstreams
Authorization: Bearer <token>
Content-Type: application/json

{
  "id": "openclaw-staging",
  "address": "10.0.2.20",
  "port": 18789,
  "tags": {
    "env": "staging",
    "region": "cn-east"
  }
}
```

响应 `201 Created`：
```json
{
  "id": "openclaw-staging",
  "address": "10.0.2.20",
  "port": 18789,
  "status": "unknown",
  "source": "api"
}
```

### 更新上游

```
PUT /api/v1/upstreams/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "address": "10.0.2.21",
  "port": 18790,
  "tags": {
    "env": "staging",
    "version": "v2"
  }
}
```

响应 `200 OK`。

> ⚠️ 静态和兼容上游不可修改。

### 删除上游

```
DELETE /api/v1/upstreams/{id}
Authorization: Bearer <token>
```

响应 `204 No Content`。

> ⚠️ 静态和兼容上游不可删除。K8s 上游可删除但下次 sync 会恢复。

### 手动健康检查

```
POST /api/v1/upstreams/{id}/health-check
Authorization: Bearer <token>
```

响应：
```json
{
  "id": "openclaw-staging",
  "status": "healthy",
  "latency_ms": 12,
  "checked_at": "2026-03-19T05:01:00Z"
}
```

### 旧版兼容 API

v3.x 时代的注册/心跳/注销 API 仍然可用：

```
POST /register       — 注册上游容器
POST /heartbeat      — 心跳保活
POST /deregister     — 注销上游容器
GET  /upstreams      — 列表（旧格式）
POST /routes/bind    — 绑定用户路由
GET  /routes         — 查看路由表
```

## 路由策略

上游注册后，通过路由策略决定请求转发到哪个上游：

### 用户亲和路由

```yaml
route_policies:
  - sender_id: "user-alice"
    upstream_id: "openclaw-1"
  - sender_pattern: "admin-*"
    upstream_id: "openclaw-2"
```

### 默认路由

未匹配任何规则的请求转发到默认上游（`openclaw_upstream` 或第一个静态上游）。

```yaml
route_default_policy: "openclaw-default"
```

## Dashboard

上游管理页面（策略引擎 → IM 策略 → 上游管理）提供完整的可视化管理：

### 页面结构

1. **统计卡片** — 总上游数、健康数（绿）、异常数（红）、总用户数（紫）
2. **K8s 状态条** — 连接状态、Namespace、Service、Pod 数、上次同步时间（仅 K8s 启用时显示）
3. **上游列表** — DataTable，7 列：ID、地址:端口、状态 badge、来源标签、用户数、最后心跳、操作
4. **来源标签** — K8s(indigo) / 静态(gray) / API(green) / 兼容(amber)
5. **操作按钮** — 编辑 / 删除 / 健康检查（静态上游删除按钮禁用）

### 添加/编辑上游

点击"添加上游"或行内编辑按钮，弹出 Modal：
- ID（添加时可编辑，编辑时只读）
- 地址 + 端口
- Tags（key-value 动态添加/删除）

### 健康检查

点击行内健康检查按钮，调用 API 并通过 Toast 反馈结果。

## 实现细节

- **后端**: `src/api.go` 中 `handleUpstreamCRUD` 路由
- **前端**: `dashboard/src/views/Upstream.vue` (829 行)
- **K8s 集成**: `src/k8s_discovery.go` 发现的 Pod 自动注册为 source=k8s 的上游
