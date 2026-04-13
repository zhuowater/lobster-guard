# lobster-guard Dockerfile — 多阶段构建
# v34.0 AI Agent 安全网关
#
# 构建: docker build -t lobster-guard:v34.0 .
# 运行: docker run -d -p 18443:18443 -p 18444:18444 -p 8445:8445 -p 9090:9090 \
#          -v ./config.yaml:/etc/lobster-guard/config.yaml:ro \
#          lobster-guard:v34.0

# ── Stage 1: Build Vue Dashboard ──
FROM node:22-alpine AS frontend
WORKDIR /app/dashboard
COPY dashboard/package*.json ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm ci --ignore-scripts
RUN npm ci --ignore-scripts
COPY dashboard/ .
# Vite outDir = '../src/dashboard/dist'，输出到 /app/src/dashboard/dist
RUN npm run build

# ── Stage 2: Build Go binary ──
FROM golang:1.23-alpine AS backend
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app/src
COPY src/go.mod src/go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY src/ ./
# go:embed dashboard/dist/* — 从 frontend 阶段拿构建产物（WORKDIR /app/dashboard，outDir=dist）
COPY --from=frontend /app/dashboard/dist ./dashboard/dist/
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /lobster-guard .

# ── Stage 3: Runtime ──
FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
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
