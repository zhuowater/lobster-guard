# lobster-guard Makefile
# 龙虾卫士 - AI Agent 安全网关 v20.7（Dashboard 企业级打磨 · 38 页面全部 CRUD 闭环）
# Go 源文件: 70 个 + 50 个测试 = 120 个 .go 文件，~71,900 行
# Vue 前端: 65 个文件（38 页面 + 21 组件），~23,700 行
# 测试用例: 950 个通过 | API 端点: ~275+ 个
# 外部依赖: sqlite3 + yaml.v3 + gorilla/websocket + x/crypto

APP_NAME := lobster-guard
VERSION := 20.7.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_FLAGS := -ldflags="-s -w"

.PHONY: all
all: build

# 准备 embed 资源（go:embed 不支持 symlink，复制到 src/ 下）
.PHONY: embed-prep
embed-prep:
	@rm -rf src/dashboard src/rules
	@cp -r dashboard src/dashboard
	@cp -r rules src/rules

# 清理 embed 副本
.PHONY: embed-clean
embed-clean:
	@rm -rf src/dashboard src/rules

# 编译（需要 CGO 支持 SQLite）
.PHONY: build
build: embed-prep
	cd src && CGO_ENABLED=1 go build $(GO_FLAGS) -o ../$(APP_NAME) .
	@$(MAKE) embed-clean

# 构建 Vue 前端（dashboard/dist/ 被 go:embed 嵌入）
.PHONY: dashboard
dashboard:
	cd dashboard && npm run build
	@echo "✅ Dashboard 构建完成（dashboard/dist/）"

# 完整构建：先构建前端，再编译 Go
.PHONY: build-all
build-all: dashboard build
	@echo "✅ 完整构建完成：前端 + Go 二进制"

# 静态编译（完全静态链接，适合 Docker/容器部署）
.PHONY: static
static: embed-prep
	cd src && CGO_ENABLED=1 go build $(GO_FLAGS) -tags 'netgo osusergo static_build' \
		-ldflags='-s -w -extldflags "-static"' -o ../$(APP_NAME) .
	@$(MAKE) embed-clean

# 清理
.PHONY: clean
clean:
	rm -f $(APP_NAME)
	rm -f audit.db
	rm -rf src/dashboard src/rules

# 运行
.PHONY: run
run: build
	./$(APP_NAME) -config config.yaml

# 测试（全部测试）
.PHONY: test
test: embed-prep
	cd src && CGO_ENABLED=1 go test -v -count=1 -timeout 60s ./...
	@$(MAKE) embed-clean

# 快速测试（不输出详细日志）
.PHONY: test-quick
test-quick: embed-prep
	cd src && CGO_ENABLED=1 go test -count=1 -timeout 60s ./...
	@$(MAKE) embed-clean

# 代码检查
.PHONY: lint
lint:
	@echo "=== Go vet ==="
	@$(MAKE) embed-prep
	cd src && CGO_ENABLED=1 go vet ./...
	@$(MAKE) embed-clean
	@echo "=== 检查完成 ==="

# 端到端模拟测试（通过 API 触发）
.PHONY: simulate
simulate:
	@echo "=== 端到端模拟测试 ==="
	@curl -s -X POST -H "Authorization: Bearer $${LOBSTER_GUARD_TOKEN}" \
		http://localhost:9090/api/v1/simulate/e2e | python3 -m json.tool 2>/dev/null || \
		echo "❌ 模拟测试失败（确保服务已启动）"

# 代码行数统计
.PHONY: count
count:
	@echo "=== Go 源文件（非测试） ==="
	@find src -name '*.go' ! -name '*_test.go' | wc -l | xargs -I{} echo "  文件数: {}"
	@find src -name '*.go' ! -name '*_test.go' | xargs wc -l | tail -1
	@echo ""
	@echo "=== Go 测试文件 ==="
	@find src -name '*_test.go' | wc -l | xargs -I{} echo "  文件数: {}"
	@find src -name '*_test.go' | xargs wc -l | tail -1
	@echo ""
	@echo "=== Vue 前端 ==="
	@find dashboard/src -name '*.vue' 2>/dev/null | wc -l | xargs -I{} echo "  文件数: {}"
	@find dashboard/src -name '*.vue' 2>/dev/null | xargs wc -l 2>/dev/null | tail -1
	@echo ""
	@echo "=== 总计 ==="
	@find src -name '*.go' -o -name '*.vue' | xargs wc -l 2>/dev/null | tail -1

# 安装到系统
.PHONY: install
install: build
	install -Dm755 $(APP_NAME) /usr/local/bin/$(APP_NAME)
	install -Dm644 config.yaml.example /etc/lobster-guard/config.yaml
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

# Docker 构建
.PHONY: docker
docker:
	docker build -t lobster-guard:latest .

# Docker Compose 启动
.PHONY: docker-compose
docker-compose:
	docker-compose up -d

# 本地 CI（vet + test）
.PHONY: ci-local
ci-local:
	cd src && CGO_ENABLED=1 go vet ./... && CGO_ENABLED=1 go test -count=1 -timeout 120s ./...

.PHONY: help
help:
	@echo "lobster-guard v20.7 Makefile 命令:"
	@echo ""
	@echo "  构建:"
	@echo "    make build         - 编译 Go 二进制"
	@echo "    make dashboard     - 构建 Vue 前端"
	@echo "    make build-all     - 完整构建（前端 + Go）"
	@echo "    make static        - 静态编译（Docker/容器用）"
	@echo "    make clean         - 清理"
	@echo ""
	@echo "  测试:"
	@echo "    make test          - 运行全部测试（950 用例）"
	@echo "    make test-quick    - 快速测试（无详细输出）"
	@echo "    make simulate      - 端到端模拟测试"
	@echo "    make lint          - 代码检查"
	@echo ""
	@echo "  运行:"
	@echo "    make run           - 编译并运行"
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
	@echo ""
	@echo "  Docker:"
	@echo "    make docker        - 构建 Docker 镜像"
	@echo "    make docker-compose - Docker Compose 启动"
	@echo "    make ci-local      - 本地 CI（vet + test）"
	@echo ""
	@echo "  统计:"
	@echo "    make count         - 代码行数统计"
