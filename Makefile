# lobster-guard Makefile
# 龙虾卫士 - AI Agent 安全网关 v3.6

APP_NAME := lobster-guard
VERSION := 3.6.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_FLAGS := -ldflags="-s -w"

.PHONY: all
all: build

# 编译（需要 CGO 支持 SQLite）
.PHONY: build
build:
	CGO_ENABLED=1 go build $(GO_FLAGS) -o $(APP_NAME) .

# 静态编译（完全静态链接，适合 Docker/容器部署）
.PHONY: static
static:
	CGO_ENABLED=1 go build $(GO_FLAGS) -tags 'netgo osusergo static_build' \
		-ldflags='-s -w -extldflags "-static"' -o $(APP_NAME) .

# 清理
.PHONY: clean
clean:
	rm -f $(APP_NAME)
	rm -f audit.db

# 运行
.PHONY: run
run: build
	./$(APP_NAME) -config config.yaml

# 测试（全部测试）
.PHONY: test
test:
	CGO_ENABLED=1 go test -v -count=1 -timeout 60s ./...

# 快速测试（不输出详细日志）
.PHONY: test-quick
test-quick:
	CGO_ENABLED=1 go test -count=1 -timeout 60s ./...

# 安装到系统
.PHONY: install
install: build
	install -Dm755 $(APP_NAME) /usr/local/bin/$(APP_NAME)
	install -Dm644 config.yaml.example /etc/lobster-guard/config.yaml
	install -Dm644 dashboard.html /etc/lobster-guard/dashboard.html
	install -Dm644 lobster-guard.service /etc/systemd/system/lobster-guard.service
	mkdir -p /var/lib/lobster-guard
	mkdir -p /var/log/lobster-guard
	systemctl daemon-reload
	@echo "✅ 安装完成！"
	@echo "   1. 编辑配置: vim /etc/lobster-guard/config.yaml"
	@echo "   2. 启动服务: systemctl start lobster-guard"
	@echo "   3. 开机自启: systemctl enable lobster-guard"
	@echo "   4. 查看日志: journalctl -u lobster-guard -f"
	@echo "   5. 管理后台: http://localhost:9090/"

# 卸载
.PHONY: uninstall
uninstall:
	systemctl stop lobster-guard 2>/dev/null || true
	systemctl disable lobster-guard 2>/dev/null || true
	rm -f /usr/local/bin/$(APP_NAME)
	rm -f /etc/systemd/system/lobster-guard.service
	systemctl daemon-reload
	@echo "✅ 已卸载 lobster-guard"

# 导出默认入站规则
.PHONY: gen-rules
gen-rules:
	./$(APP_NAME) -gen-rules inbound-rules.yaml
	@echo "✅ 默认规则已导出到 inbound-rules.yaml"

# 查看审计日志
.PHONY: logs
logs:
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT id, timestamp, direction, action, reason, upstream_id, substr(content_preview, 1, 50) FROM audit_log ORDER BY id DESC LIMIT 20;"

# 统计
.PHONY: stats
stats:
	@echo "=== 审计日志统计 ==="
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT direction, action, COUNT(*) as cnt FROM audit_log GROUP BY direction, action ORDER BY direction, cnt DESC;"
	@echo ""
	@echo "=== 上游容器 ==="
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT id, address, port, healthy, last_heartbeat FROM upstreams;" 2>/dev/null || echo "(无动态上游)"
	@echo ""
	@echo "=== 路由绑定 ==="
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT sender_id, upstream_id, updated_at FROM user_routes ORDER BY updated_at DESC LIMIT 20;" 2>/dev/null || echo "(无路由)"

# 健康检查
.PHONY: healthz
healthz:
	@curl -s http://localhost:9090/healthz | python3 -m json.tool 2>/dev/null || curl -s http://localhost:9090/healthz

# Prometheus 指标
.PHONY: metrics
metrics:
	@curl -s http://localhost:9090/metrics

# 查看上游
.PHONY: upstreams
upstreams:
	@curl -s -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" http://localhost:9090/api/v1/upstreams | python3 -m json.tool 2>/dev/null

# 查看路由
.PHONY: routes
routes:
	@curl -s -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" http://localhost:9090/api/v1/routes | python3 -m json.tool 2>/dev/null

# 限流统计
.PHONY: rate-limit
rate-limit:
	@curl -s -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" http://localhost:9090/api/v1/rate-limit/stats | python3 -m json.tool 2>/dev/null

# 规则命中率
.PHONY: rule-hits
rule-hits:
	@curl -s -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" http://localhost:9090/api/v1/rules/hits | python3 -m json.tool 2>/dev/null

# 入站规则
.PHONY: inbound-rules
inbound-rules:
	@curl -s -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" http://localhost:9090/api/v1/inbound-rules | python3 -m json.tool 2>/dev/null

.PHONY: help
help:
	@echo "lobster-guard v3.6 Makefile 命令:"
	@echo ""
	@echo "  构建:"
	@echo "    make build         - 编译"
	@echo "    make static        - 静态编译（Docker/容器用）"
	@echo "    make test          - 运行全部测试"
	@echo "    make test-quick    - 快速测试（无详细输出）"
	@echo "    make run           - 编译并运行"
	@echo "    make clean         - 清理"
	@echo ""
	@echo "  部署:"
	@echo "    make install       - 安装到系统（systemd）"
	@echo "    make uninstall     - 从系统卸载"
	@echo "    make gen-rules     - 导出默认入站规则到 YAML"
	@echo ""
	@echo "  监控:"
	@echo "    make healthz       - 健康检查"
	@echo "    make metrics       - Prometheus 指标"
	@echo "    make stats         - 审计统计"
	@echo "    make logs          - 最近审计日志"
	@echo "    make upstreams     - 查看上游容器"
	@echo "    make routes        - 查看路由绑定"
	@echo "    make rate-limit    - 限流统计"
	@echo "    make rule-hits     - 规则命中率"
	@echo "    make inbound-rules - 入站规则列表"
