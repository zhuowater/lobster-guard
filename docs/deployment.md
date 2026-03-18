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
make test       # 运行测试（754 个用例）
make install    # 安装到系统
make healthz    # 检查健康状态
make stats      # 查看统计
make logs       # 查看审计日志
```
