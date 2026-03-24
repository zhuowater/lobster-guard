# lobster-guard Dockerfile — 多阶段构建
# v22.4 全功能版本（分层配置 + K8s 服务发现 + Dashboard 企业级打磨）
#
# 构建: docker build -t lobster-guard:v22.4 .
# 运行: docker run -d -p 18443:18443 -p 18444:18444 -p 8445:8445 -p 9090:9090 \
#          -v ./config.yaml:/etc/lobster-guard/config.yaml:ro \
#          lobster-guard:v22.4

# ── Stage 1: Build Vue Dashboard ──
FROM node:22-alpine AS frontend
WORKDIR /app/dashboard
COPY dashboard/package*.json ./
RUN npm ci --ignore-scripts
COPY dashboard/ .
RUN npm run build

# ── Stage 2: Build Go binary ──
FROM golang:1.23-alpine AS backend
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app/src
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ ./
COPY rules/ ./rules/
# Embed built dashboard
COPY --from=frontend /app/dashboard/dist ./dashboard/dist/
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /lobster-guard .

# ── Stage 3: Runtime ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates sqlite-libs tzdata \
    && addgroup -S lobster && adduser -S lobster -G lobster
COPY --from=backend /lobster-guard /usr/local/bin/lobster-guard
COPY config.yaml.example /etc/lobster-guard/config.yaml
COPY conf.d/ /etc/lobster-guard/conf.d/

RUN mkdir -p /var/lib/lobster-guard && chown lobster:lobster /var/lib/lobster-guard

# 4 端口架构
EXPOSE 18443 18444 8445 9090

VOLUME ["/var/lib/lobster-guard"]

USER lobster
ENTRYPOINT ["lobster-guard", "-config", "/etc/lobster-guard/config.yaml"]
