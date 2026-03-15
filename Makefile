# lobster-guard Makefile
# 龙虾卫士 - 高性能安全代理网关

APP_NAME := lobster-guard
VERSION := 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_FLAGS := -ldflags="-s -w -X main.AppVersion=$(VERSION)"

# 默认目标
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

# 测试规则引擎
.PHONY: test
test:
	go test -v -run TestRuleEngine ./...

# 安装到系统
.PHONY: install
install: build
	install -Dm755 $(APP_NAME) /usr/local/bin/$(APP_NAME)
	install -Dm644 config.yaml /etc/lobster-guard/config.yaml
	install -Dm644 lobster-guard.service /etc/systemd/system/lobster-guard.service
	mkdir -p /var/lib/lobster-guard
	mkdir -p /var/log/lobster-guard
	systemctl daemon-reload
	@echo "✅ 安装完成！使用以下命令管理服务："
	@echo "   systemctl start lobster-guard"
	@echo "   systemctl enable lobster-guard"
	@echo "   journalctl -u lobster-guard -f"

# 卸载
.PHONY: uninstall
uninstall:
	systemctl stop lobster-guard 2>/dev/null || true
	systemctl disable lobster-guard 2>/dev/null || true
	rm -f /usr/local/bin/$(APP_NAME)
	rm -f /etc/systemd/system/lobster-guard.service
	systemctl daemon-reload
	@echo "✅ 已卸载 lobster-guard"

# 查看审计日志
.PHONY: logs
logs:
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT id, timestamp, direction, action, reason, substr(content_preview, 1, 50) FROM audit_log ORDER BY id DESC LIMIT 20;"

# 统计
.PHONY: stats
stats:
	@echo "=== 审计日志统计 ==="
	@sqlite3 /var/lib/lobster-guard/audit.db \
		"SELECT direction, action, COUNT(*) as cnt FROM audit_log GROUP BY direction, action ORDER BY direction, cnt DESC;"

.PHONY: help
help:
	@echo "lobster-guard Makefile 命令:"
	@echo "  make build     - 编译"
	@echo "  make static    - 静态编译"
	@echo "  make run       - 编译并运行"
	@echo "  make install   - 安装到系统"
	@echo "  make uninstall - 从系统卸载"
	@echo "  make clean     - 清理"
	@echo "  make logs      - 查看最近审计日志"
	@echo "  make stats     - 查看统计"
