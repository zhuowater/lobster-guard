# lobster-guard Dockerfile — 多阶段构建
# v18.2 工程化基础

# Stage 1: Build Vue Dashboard
FROM node:22-alpine AS frontend
WORKDIR /app/dashboard
COPY dashboard/package*.json ./
RUN npm ci
COPY dashboard/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS backend
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY src/ ./src/
COPY go.mod go.sum ./
# Copy built dashboard into embed location
COPY --from=frontend /app/dashboard/dist ./src/dashboard/dist/
WORKDIR /app/src
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /lobster-guard .

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=backend /lobster-guard /usr/local/bin/lobster-guard
COPY config.yaml.example /etc/lobster-guard/config.yaml
VOLUME ["/var/lib/lobster-guard"]
EXPOSE 8443 8444 9090
ENTRYPOINT ["lobster-guard", "-config", "/etc/lobster-guard/config.yaml"]
