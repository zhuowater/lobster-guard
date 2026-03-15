#!/bin/bash
# lobster-guard CLI — 龙虾卫士命令行工具 v3.6
# 用法: ./lobster-cli.sh <command> [args...]
#
# 环境变量:
#   LOBSTER_GUARD_URL   管理API地址 (默认 http://127.0.0.1:9090)
#   LOBSTER_GUARD_TOKEN 管理Token
#   LOBSTER_GUARD_REG_TOKEN 注册Token

set -euo pipefail

URL="${LOBSTER_GUARD_URL:-http://127.0.0.1:9090}"
TOKEN="${LOBSTER_GUARD_TOKEN:-}"
REG_TOKEN="${LOBSTER_GUARD_REG_TOKEN:-}"

auth_header() {
  if [ -n "$TOKEN" ]; then
    echo "-H" "Authorization: Bearer $TOKEN"
  fi
}

reg_header() {
  if [ -n "$REG_TOKEN" ]; then
    echo "-H" "Authorization: Bearer $REG_TOKEN"
  fi
}

jq_or_cat() {
  if command -v jq &>/dev/null; then
    jq .
  elif command -v python3 &>/dev/null; then
    python3 -m json.tool
  else
    cat
  fi
}

case "${1:-help}" in
  status|healthz|health)
    curl -s "$URL/healthz" | jq_or_cat
    ;;
  upstreams|upstream|up)
    curl -s $(auth_header) "$URL/api/v1/upstreams" | jq_or_cat
    ;;
  routes|route)
    curl -s $(auth_header) "$URL/api/v1/routes" | jq_or_cat
    ;;
  bind)
    [ -z "${2:-}" ] || [ -z "${3:-}" ] && { echo "用法: $0 bind <sender_id> <upstream_id>"; exit 1; }
    curl -s -X POST $(auth_header) -H "Content-Type: application/json" \
      -d "{\"sender_id\":\"$2\",\"upstream_id\":\"$3\"}" \
      "$URL/api/v1/routes/bind" | jq_or_cat
    ;;
  migrate)
    [ -z "${2:-}" ] || [ -z "${3:-}" ] && { echo "用法: $0 migrate <sender_id> <target_upstream_id>"; exit 1; }
    curl -s -X POST $(auth_header) -H "Content-Type: application/json" \
      -d "{\"sender_id\":\"$2\",\"target_upstream_id\":\"$3\"}" \
      "$URL/api/v1/routes/migrate" | jq_or_cat
    ;;
  logs|audit)
    PARAMS=""
    [ -n "${2:-}" ] && PARAMS="?direction=$2"
    [ -n "${3:-}" ] && PARAMS="${PARAMS:+${PARAMS}&}action=$3"
    curl -s $(auth_header) "$URL/api/v1/audit/logs${PARAMS:+?${PARAMS}}" | jq_or_cat
    ;;
  blocks|blocked)
    curl -s $(auth_header) "$URL/api/v1/audit/logs?action=block" | jq_or_cat
    ;;
  warns|warned)
    curl -s $(auth_header) "$URL/api/v1/audit/logs?action=warn" | jq_or_cat
    ;;
  stats|stat)
    curl -s $(auth_header) "$URL/api/v1/stats" | jq_or_cat
    ;;
  inbound-rules|inbound)
    curl -s $(auth_header) "$URL/api/v1/inbound-rules" | jq_or_cat
    ;;
  outbound-rules|outbound)
    curl -s $(auth_header) "$URL/api/v1/outbound-rules" | jq_or_cat
    ;;
  reload)
    echo "=== 热更新入站规则 ==="
    curl -s -X POST $(auth_header) "$URL/api/v1/inbound-rules/reload" | jq_or_cat
    echo ""
    echo "=== 热更新出站规则 ==="
    curl -s -X POST $(auth_header) "$URL/api/v1/rules/reload" | jq_or_cat
    ;;
  reload-inbound)
    curl -s -X POST $(auth_header) "$URL/api/v1/inbound-rules/reload" | jq_or_cat
    ;;
  reload-outbound)
    curl -s -X POST $(auth_header) "$URL/api/v1/rules/reload" | jq_or_cat
    ;;
  rule-hits|hits)
    curl -s $(auth_header) "$URL/api/v1/rules/hits" | jq_or_cat
    ;;
  reset-hits)
    curl -s -X POST $(auth_header) "$URL/api/v1/rules/hits/reset" | jq_or_cat
    ;;
  rate-limit|ratelimit|rl)
    curl -s $(auth_header) "$URL/api/v1/rate-limit/stats" | jq_or_cat
    ;;
  reset-rate-limit|reset-rl)
    curl -s -X POST $(auth_header) "$URL/api/v1/rate-limit/reset" | jq_or_cat
    ;;
  metrics)
    curl -s "$URL/metrics"
    ;;
  register)
    [ -z "${2:-}" ] || [ -z "${3:-}" ] || [ -z "${4:-}" ] && { echo "用法: $0 register <id> <address> <port>"; exit 1; }
    curl -s -X POST $(reg_header) -H "Content-Type: application/json" \
      -d "{\"id\":\"$2\",\"address\":\"$3\",\"port\":$4}" \
      "$URL/api/v1/register" | jq_or_cat
    ;;
  deregister)
    [ -z "${2:-}" ] && { echo "用法: $0 deregister <id>"; exit 1; }
    curl -s -X POST $(reg_header) -H "Content-Type: application/json" \
      -d "{\"id\":\"$2\"}" \
      "$URL/api/v1/deregister" | jq_or_cat
    ;;
  heartbeat)
    [ -z "${2:-}" ] && { echo "用法: $0 heartbeat <id>"; exit 1; }
    curl -s -X POST $(reg_header) -H "Content-Type: application/json" \
      -d "{\"id\":\"$2\"}" \
      "$URL/api/v1/heartbeat" | jq_or_cat
    ;;
  report)
    echo "========================================"
    echo "🦞 龙虾卫士安全报告"
    echo "========================================"
    echo ""
    echo "--- 系统状态 ---"
    curl -s "$URL/healthz" | jq_or_cat
    echo ""
    echo "--- 统计概览 ---"
    curl -s $(auth_header) "$URL/api/v1/stats" | jq_or_cat
    echo ""
    echo "--- 规则命中率 ---"
    curl -s $(auth_header) "$URL/api/v1/rules/hits" | jq_or_cat
    echo ""
    echo "--- 限流统计 ---"
    curl -s $(auth_header) "$URL/api/v1/rate-limit/stats" | jq_or_cat
    echo ""
    echo "--- 最近拦截记录 ---"
    curl -s $(auth_header) "$URL/api/v1/audit/logs?action=block&limit=10" | jq_or_cat
    echo ""
    echo "--- 最近告警记录 ---"
    curl -s $(auth_header) "$URL/api/v1/audit/logs?action=warn&limit=10" | jq_or_cat
    echo ""
    echo "========================================"
    ;;
  help|--help|-h)
    cat <<EOF
🦞 lobster-guard CLI v3.6 — 龙虾卫士命令行工具

用法: $0 <command> [args...]

状态:
  status              健康检查 + 系统概览
  metrics             Prometheus 指标

上游管理:
  upstreams           列出上游容器
  register <id> <addr> <port>  注册容器
  deregister <id>     注销容器
  heartbeat <id>      心跳上报

路由管理:
  routes              列出路由绑定
  bind <sender> <upstream>     绑定用户到上游
  migrate <sender> <upstream>  迁移用户

规则管理:
  inbound-rules       列出入站规则
  outbound-rules      列出出站规则
  rule-hits           规则命中率排行
  reload              热更新全部规则（入站+出站）
  reload-inbound      热更新入站规则
  reload-outbound     热更新出站规则
  reset-hits          重置命中统计

限流:
  rate-limit          限流统计
  reset-rate-limit    重置限流计数器

审计:
  logs [dir] [action]  审计日志（可选：方向/动作筛选）
  blocks              拦截记录
  warns               告警记录
  stats               统计概览

报告:
  report              综合安全报告（全面分析）

环境变量:
  LOBSTER_GUARD_URL     管理API地址 (默认 http://127.0.0.1:9090)
  LOBSTER_GUARD_TOKEN   管理Token
  LOBSTER_GUARD_REG_TOKEN 注册Token
EOF
    ;;
  *)
    echo "未知命令: $1"
    echo "运行 '$0 help' 查看帮助"
    exit 1
    ;;
esac
