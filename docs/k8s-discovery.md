# ☸️ Kubernetes 服务发现

> 返回 [README](../README.md) | 相关: [部署指南](deployment.md) · [配置参考](configuration.md) · [上游管理](upstream-management.md)

## 概述

龙虾卫士支持从 Kubernetes 集群自动发现 OpenClaw 实例并注册为上游。零外部依赖——直接调用 K8s REST API，不引入 client-go。

支持两种模式：
- **InCluster** — Pod 内自动检测 ServiceAccount，零配置
- **Kubeconfig** — 集群外通过 kubeconfig 文件连接

## 配置

```yaml
discovery:
  kubernetes:
    enabled: true                    # 开关，默认 false
    kubeconfig: ""                   # 空 = InCluster 模式
                                     # 填路径 = 用 kubeconfig，如 "/root/.kube/config"
    namespace: "openclaw"            # OpenClaw 所在 namespace
    service: "openclaw-gateway"      # Service 名称
    port_name: "gateway"             # 端口名（匹配 Service spec.ports[].name）
    label_selector: ""               # 可选，Pod 标签过滤
    sync_interval: 15                # 轮询间隔（秒），默认 15
```

## 工作原理

### 发现流程

```
K8s Endpoints API ─→ 解析 Pod IP + Port ─→ 注册/更新上游 ─→ 心跳保活
                                      │
                                      └→ Pod 消失 ─→ 自动移除上游
```

1. **定时轮询** — 每 `sync_interval` 秒调用 `GET /api/v1/namespaces/{ns}/endpoints/{svc}`
2. **解析 Endpoints** — 提取所有 Ready 的 Pod 地址和端口
3. **比对注册** — 新 Pod 自动注册，消失的 Pod 自动移除
4. **上游 ID** — 格式 `k8s-{namespace}-{pod-name}`（如 `k8s-openclaw-gateway-7b4f9-xk2m3`）

### InCluster 模式

龙虾卫士部署为 K8s Pod 时，自动：
- 从 `/var/run/secrets/kubernetes.io/serviceaccount/token` 读取 Bearer Token
- 从环境变量 `KUBERNETES_SERVICE_HOST` / `KUBERNETES_SERVICE_PORT` 获取 API Server 地址
- 从 `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt` 加载 CA 证书

### Kubeconfig 模式

龙虾卫士部署在集群外时：
- 读取指定的 kubeconfig 文件
- 解析 `current-context` → `cluster.server` + `user.token` (或 `client-certificate`)
- 支持 Bearer Token 和 TLS 客户端证书两种认证

## RBAC 配置

最小权限——只需读取 Endpoints：

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: lobster-guard
  namespace: lobster-guard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: lobster-guard-discovery
  namespace: openclaw           # 目标 namespace
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: lobster-guard-discovery
  namespace: openclaw
subjects:
- kind: ServiceAccount
  name: lobster-guard
  namespace: lobster-guard
roleRef:
  kind: Role
  name: lobster-guard-discovery
  apiGroup: rbac.authorization.k8s.io
```

> 如果 OpenClaw 部署在多个 namespace，改用 `ClusterRole` + `ClusterRoleBinding`。

## K8s 部署示例

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: lobster-guard
  namespace: lobster-guard
spec:
  replicas: 1
  selector:
    matchLabels:
      app: lobster-guard
  template:
    metadata:
      labels:
        app: lobster-guard
    spec:
      serviceAccountName: lobster-guard
      containers:
      - name: lobster-guard
        image: lobster-guard:v20.5
        ports:
        - containerPort: 18443
          name: inbound
        - containerPort: 18444
          name: outbound
        - containerPort: 8445
          name: llm
        - containerPort: 9090
          name: management
        volumeMounts:
        - name: config
          mountPath: /etc/lobster-guard
        - name: data
          mountPath: /var/lib/lobster-guard
      volumes:
      - name: config
        configMap:
          name: lobster-guard-config
      - name: data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: lobster-guard
  namespace: lobster-guard
spec:
  selector:
    app: lobster-guard
  ports:
  - name: inbound
    port: 18443
  - name: outbound
    port: 18444
  - name: llm
    port: 8445
  - name: management
    port: 9090
```

## API

### 查询发现状态

```
GET /api/v1/discovery/status
Authorization: Bearer <token>
```

响应：
```json
{
  "enabled": true,
  "connected": true,
  "mode": "in-cluster",
  "namespace": "openclaw",
  "service": "openclaw-gateway",
  "last_sync": "2026-03-19T05:00:00Z",
  "pods_discovered": 3,
  "error": ""
}
```

| 字段 | 说明 |
|------|------|
| `enabled` | 是否启用 K8s 发现 |
| `connected` | 是否成功连接 K8s API |
| `mode` | `in-cluster` 或 `kubeconfig` |
| `namespace` | 监听的 namespace |
| `service` | 监听的 Service |
| `last_sync` | 上次同步时间 |
| `pods_discovered` | 发现的 Pod 数量 |
| `error` | 最近一次错误（空 = 正常） |

### Dashboard 展示

上游管理页面顶部显示 K8s 发现状态条：
- 连接状态（绿色已连接 / 红色断开）
- Namespace 和 Service 名称
- 发现的 Pod 数量
- 上次同步时间

K8s 发现的上游在列表中标记为 **k8s** 来源（indigo 标签），与静态上游（gray）和 API 注册上游（green）区分。

## 故障排查

| 症状 | 原因 | 解决 |
|------|------|------|
| `connected: false` | RBAC 权限不足 | 检查 Role/RoleBinding |
| `pods_discovered: 0` | Service 名或 namespace 不对 | `kubectl get endpoints -n <ns>` 验证 |
| 频繁注册/移除 | Pod 滚动更新 | 正常现象，sync_interval 可调大 |
| InCluster 检测失败 | 不在 K8s Pod 内 | 改用 kubeconfig 模式 |
| kubeconfig 解析失败 | 文件格式/路径错误 | `kubectl config view` 检查 |

## 实现细节

- **源文件**: `src/k8s_discovery.go` (513 行)
- **测试**: `src/k8s_discovery_test.go` (493 行, 10 个测试)
- **零依赖**: 直接 `net/http` + `encoding/json` 调用 K8s REST API
- **并发安全**: `sync.RWMutex` 保护上游注册表
- **优雅关闭**: `context.WithCancel` 控制轮询 goroutine 生命周期
