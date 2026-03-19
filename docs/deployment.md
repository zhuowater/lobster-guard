# 📦 部署指南

> 返回 [README](../README.md)

## 方式一：直接运行（推荐快速试用）

```bash
CGO_ENABLED=1 go build -o lobster-guard .
cp config.yaml.example config.yaml
./lobster-guard -config config.yaml
```

## 方式二：Systemd 服务（推荐生产部署）

```bash
sudo cp lobster-guard /usr/local/bin/
sudo mkdir -p /etc/lobster-guard /var/lib/lobster-guard
sudo cp config.yaml /etc/lobster-guard/

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
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/lobster-guard
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl start lobster-guard
sudo systemctl enable lobster-guard
```

## 方式三：Docker

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o lobster-guard .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/lobster-guard /usr/local/bin/
COPY config.yaml.example /etc/lobster-guard/config.yaml
EXPOSE 18443 18444 8445 9090
CMD ["lobster-guard", "-config", "/etc/lobster-guard/config.yaml"]
```

```bash
docker build -t lobster-guard .
docker run -d -p 18443:18443 -p 18444:18444 -p 8445:8445 -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/lobster-guard/config.yaml \
  lobster-guard
```

## 方式四：Make（推荐开发）

```bash
make build      # 编译
make test       # 运行测试（930 个用例）
make install    # 安装到系统
make healthz    # 检查健康状态
make stats      # 查看统计
make logs       # 查看审计日志
```

## 方式五：Kubernetes

> 详见 [K8s 服务发现](k8s-discovery.md)

```bash
# 创建 namespace
kubectl create namespace lobster-guard

# 创建配置
kubectl create configmap lobster-guard-config \
  --from-file=config.yaml=config.yaml \
  -n lobster-guard

# 部署 RBAC + Deployment + Service
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# 验证
kubectl get pods -n lobster-guard
kubectl logs -f deploy/lobster-guard -n lobster-guard
```

启用 K8s 服务发现后，龙虾卫士自动发现同集群的 OpenClaw Pod 并注册为上游。详见 [K8s 服务发现文档](k8s-discovery.md)。

## Phase 1 部署注意事项 (v18-v20)

### 新增配置要求

v18-v20 引入了多个新引擎，部署时需注意以下配置：

1. **执行信封** — `envelope.hmac_key` 建议显式配置（否则随机生成，重启后变更）
2. **事件总线** — 如需 Webhook 推送，配置 `event_bus.targets`
3. **自适应决策** — 默认启用，`adaptive.fp_target` 控制误伤率目标
4. **对抗性自进化** — `evolution.auto_apply: false`（默认），生产环境建议手动审核
5. **LLM 缓存** — `llm_cache.tenant_isolation: true` 确保多租户数据隔离
6. **API 网关** — `api_gateway.jwt_secret` 必须配置（≥32字符）
7. **污染逆转** — `reversal.auto_reverse: false`（默认），建议先在 warn 模式下观察

### 数据库迁移

v18-v20 自动迁移 SQLite 表结构，无需手动操作。首次启动新版本时会创建以下新表：

- `envelopes` / `merkle_batches` — 执行信封
- `security_events` / `event_targets` — 事件总线
- `adaptive_decisions` / `adaptive_feedback` — 自适应决策
- `evolution_log` / `evolution_rules` — 自进化
- `semantic_patterns` — 语义检测
- `honeypot_interactions` / `loyalty_scores` — 深度交互
- `tool_policy_rules` / `tool_events` — 工具策略
- `taint_records` / `taint_lineage` — 污染追踪
- `reversal_records` — 污染逆转
- `llm_cache` — LLM 缓存
- `gateway_routes` / `gateway_log` — API 网关

### 资源建议

| 规模 | CPU | 内存 | 磁盘 |
|------|-----|------|------|
| 小型 (<100 用户) | 1 核 | 256MB | 1GB |
| 中型 (100-1000 用户) | 2 核 | 512MB | 5GB |
| 大型 (>1000 用户) | 4 核 | 1GB | 20GB |
