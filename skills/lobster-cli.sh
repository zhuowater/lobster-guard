#!/usr/bin/env bash
# 🦞 龙虾卫士 CLI — v33.0
# 覆盖：系统状态 / IM 安全 / LLM 安全 / 威胁分析 / 安全画像 / 安全治理 / 运维
set -euo pipefail

VERSION="33.0"
BASE_URL="${LOBSTER_GUARD_URL:-http://10.44.96.142:9090}"
TOKEN="${LOBSTER_GUARD_TOKEN:-}"
REG_TOKEN="${LOBSTER_GUARD_REG_TOKEN:-}"

# ── 工具函数 ─────────────────────────────────────────────
pretty() {
  if command -v jq &>/dev/null; then
    jq .
  elif command -v python3 &>/dev/null; then
    python3 -m json.tool
  else
    cat
  fi
}

auth_header() {
  [[ -n "$TOKEN" ]] && echo "-H \"Authorization: Bearer ${TOKEN}\"" || echo ""
}

api_get() {
  eval curl -sf -H "'Authorization: Bearer ${TOKEN}'" "'${BASE_URL}${1}'" | pretty
}

api_post() {
  local endpoint="$1"; shift
  if [[ $# -gt 0 ]]; then
    eval curl -sf -X POST -H "'Authorization: Bearer ${TOKEN}'" \
      -H "'Content-Type: application/json'" -d "'${1}'" "'${BASE_URL}${endpoint}'" | pretty
  else
    eval curl -sf -X POST -H "'Authorization: Bearer ${TOKEN}'" "'${BASE_URL}${endpoint}'" | pretty
  fi
}

api_delete() {
  eval curl -sf -X DELETE -H "'Authorization: Bearer ${TOKEN}'" "'${BASE_URL}${1}'" | pretty
}

api_put() {
  eval curl -sf -X PUT -H "'Authorization: Bearer ${TOKEN}'" \
    -H "'Content-Type: application/json'" -d "'${2}'" "'${BASE_URL}${1}'" | pretty
}

api_public() {
  curl -sf "${BASE_URL}${1}" | pretty
}

# ── 系统状态 ──────────────────────────────────────────────
cmd_status()          { api_public "/healthz"; }
cmd_healthz()         { api_public "/healthz"; }
cmd_health_score()    { api_get "/api/v1/health/score"; }
cmd_diag()            { api_get "/api/v1/system/diag"; }
cmd_strict_mode()     { api_get "/api/v1/system/strict-mode"; }
cmd_strict_mode_on()  { api_post "/api/v1/system/strict-mode" '{"enabled":true}'; }
cmd_strict_mode_off() { api_post "/api/v1/system/strict-mode" '{"enabled":false}'; }

# ── IM 安全 ───────────────────────────────────────────────
cmd_upstreams()     { api_get "/api/v1/upstreams"; }
cmd_routes()        { api_get "/api/v1/routes"; }
cmd_bind()          { api_post "/api/v1/routes/bind" "{\"sender\":\"${1}\",\"upstream\":\"${2}\"}"; }
cmd_migrate()       { api_post "/api/v1/routes/migrate" "{\"sender\":\"${1}\",\"upstream\":\"${2}\"}"; }
cmd_inbound_rules() { api_get "/api/v1/rules/inbound"; }
cmd_outbound_rules(){ api_get "/api/v1/rules/outbound"; }
cmd_rule_hits()     { api_get "/api/v1/rules/hits"; }
cmd_reload()        { api_post "/api/v1/rules/reload"; }
cmd_rate_limit()    { api_get "/api/v1/rate-limit/stats"; }
cmd_logs()          {
  local dir="${1:-all}" action="${2:-}"
  local qs="?direction=${dir}"
  [[ -n "$action" ]] && qs="${qs}&action=${action}"
  api_get "/api/v1/audit/logs${qs}"
}
cmd_blocks()        { api_get "/api/v1/audit/blocks"; }
cmd_warns()         { api_get "/api/v1/audit/warns"; }
cmd_stats()         { api_get "/api/v1/stats"; }
cmd_export_audit()  {
  local fmt="${1:-json}"
  api_get "/api/v1/audit/export?format=${fmt}"
}
cmd_timeline()      {
  local hours="${1:-24}"
  api_get "/api/v1/audit/timeline?hours=${hours}"
}

# ── LLM 安全 ──────────────────────────────────────────────
cmd_llm_status()    { api_get "/api/v1/llm/status"; }
cmd_llm_overview()  { api_get "/api/v1/llm/overview"; }
cmd_llm_rules()     { api_get "/api/v1/llm/rules"; }
cmd_canary()        { api_get "/api/v1/canary/status"; }
cmd_canary_rotate() { api_post "/api/v1/canary/rotate"; }
cmd_budget()        { api_get "/api/v1/budget/status"; }
cmd_owasp()         { api_get "/api/v1/llm/owasp"; }

# ── 威胁分析 ──────────────────────────────────────────────
cmd_risk_top()       { api_get "/api/v1/threat/risk-top"; }
cmd_anomaly()        { api_get "/api/v1/threat/anomaly"; }
cmd_redteam()        { api_post "/api/v1/threat/redteam"; }
cmd_redteam_reports(){ api_get "/api/v1/threat/redteam/reports"; }
cmd_behavior()       { api_get "/api/v1/threat/behavior"; }
cmd_attack_chains()  { api_get "/api/v1/threat/attack-chains"; }

# ── 安全治理 ──────────────────────────────────────────────
cmd_tenants()          { api_get "/api/v1/governance/tenants"; }
cmd_leaderboard()      { api_get "/api/v1/governance/leaderboard"; }
cmd_honeypot()         { api_get "/api/v1/governance/honeypot"; }
cmd_sessions()         { api_get "/api/v1/governance/sessions"; }
cmd_generate_report()  { api_post "/api/v1/governance/reports/generate" "{\"type\":\"${1}\"}"; }
cmd_reports()          { api_get "/api/v1/governance/reports"; }

# ── Gateway 远程管理 (v29.0) ───────────────────────────────
cmd_gw_restart()    { api_post "/api/v1/upstreams/${1}/gateway/restart"; }
cmd_gw_update()     { api_post "/api/v1/upstreams/${1}/gateway/update"; }
cmd_gw_config()     { api_get "/api/v1/upstreams/${1}/gateway/config"; }
cmd_gw_sessions()   { api_get "/api/v1/upstreams/${1}/gateway/sessions"; }
cmd_gw_approvals()  { api_get "/api/v1/upstreams/${1}/gateway/exec-approvals"; }
cmd_gw_memory()     {
  local upstream_id="$1"
  local query="${2:-}"
  api_post "/api/v1/upstreams/${upstream_id}/gateway/memory/search" "{\"query\":\"${query}\"}"
}

# ── 运维 ──────────────────────────────────────────────────
cmd_backup()        { api_post "/api/v1/backup"; }
cmd_backups()       { api_get "/api/v1/backups"; }
cmd_config()        { api_get "/api/v1/config/view"; }
cmd_notifications() { api_get "/api/v1/notifications"; }
cmd_simulate()      { api_post "/api/v1/simulate/traffic"; }
cmd_bigscreen()     { api_get "/api/v1/bigscreen/data"; }

cmd_report() {
  echo "═══════════════════════════════════════════"
  echo "  🦞 龙虾卫士 · 综合安全报告"
  echo "  $(date '+%Y-%m-%d %H:%M:%S')"
  echo "═══════════════════════════════════════════"
  echo ""
  echo "▸ 健康检查"
  api_public "/healthz"
  echo ""
  echo "▸ 统计概览"
  api_get "/api/v1/stats"
  echo ""
  echo "▸ 规则命中率"
  api_get "/api/v1/rules/hits"
  echo ""
  echo "▸ 限流统计"
  api_get "/api/v1/rate-limit/stats"
  echo ""
  echo "▸ 拦截记录"
  api_get "/api/v1/audit/blocks"
  echo ""
  echo "═══════════════════════════════════════════"
  echo "  报告生成完毕"
  echo "═══════════════════════════════════════════"
}

# ── 帮助 ──────────────────────────────────────────────────
# === 安全画像 (v33.0) ===
cmd_security_profile() {
  local uid="${1:?用法: security-profile <upstream_id>}"
  api_get "/api/v1/upstreams/${uid}/security-profile" | jq '{score: .security_score, level: .risk_level, users: .user_count, dimensions: [.dimensions[] | {(.name): .score}]}'
}
cmd_security_profiles() {
  api_get "/api/v1/upstream-profiles" | jq '{total: .total, users: .total_users, avg: .avg_score, segments: .segments, profiles: [.profiles[] | {id: .upstream_id, score: .security_score, level: .risk_level, users: .user_count}]}'
}

cmd_help() {
cat <<'EOF'
🦞 龙虾卫士 CLI v29.0

系统状态:
  status / healthz        健康检查
  health-score            安全健康分
  diag                    系统诊断
  strict-mode             严格模式状态
  strict-mode-on          开启严格模式
  strict-mode-off         关闭严格模式

IM 安全:
  upstreams               上游容器
  routes                  路由表
  bind <sender> <up>      绑定路由
  migrate <sender> <up>   迁移路由
  inbound-rules           入站规则
  outbound-rules          出站规则
  rule-hits               规则命中率
  reload                  热更新全部规则
  rate-limit              限流统计
  logs [dir] [action]     审计日志
  blocks                  拦截记录
  warns                   告警记录
  stats                   统计概览
  export-audit [format]   导出审计（csv/json）
  timeline [hours]        时间线（默认 24h）

LLM 安全:
  llm-status              LLM 状态
  llm-overview            LLM 概览
  llm-rules               LLM 规则
  canary                  Canary Token 状态
  canary-rotate           轮换 Canary Token
  budget                  预算状态
  owasp                   OWASP 矩阵

威胁分析:
  risk-top                风险 TOP 用户
  anomaly                 异常告警
  redteam                 运行红队测试
  redteam-reports         红队报告
  behavior                Agent 行为画像
  attack-chains           攻击链

安全治理:
  tenants                 租户列表
  leaderboard             排行榜
  honeypot                蜜罐统计
  sessions                会话回放列表
  generate-report <type>  生成报告
  reports                 报告列表

安全画像 (v33.0):
  security-profile <id>       单个上游安全画像（5维评分）
  security-profiles           全部上游安全画像列表 + 分段统计

Gateway 远程管理:
  gw-restart <upstream_id>    远程重启 Gateway
  gw-update <upstream_id>     触发 Gateway 自更新
  gw-config <upstream_id>     查看 Gateway 配置
  gw-sessions <upstream_id>   列出 Gateway 会话
  gw-approvals <upstream_id>  查看待审批命令
  gw-memory <upstream_id> <q> 搜索 Agent 记忆

运维:
  backup                  创建备份
  backups                 备份列表
  config                  查看配置
  notifications           通知中心
  simulate                端到端模拟
  bigscreen               大屏数据
  report                  综合安全报告

环境变量:
  LOBSTER_GUARD_URL       管理 API 地址（默认 http://10.44.96.142:9090）
  LOBSTER_GUARD_TOKEN     认证 Token
  LOBSTER_GUARD_REG_TOKEN 容器注册 Token
EOF
}

# ── 路由 ──────────────────────────────────────────────────
main() {
  local cmd="${1:-help}"; shift 2>/dev/null || true

  case "$cmd" in
    # 系统状态
    status|healthz)     cmd_status ;;
    health-score)       cmd_health_score ;;
    diag)               cmd_diag ;;
    strict-mode)        cmd_strict_mode ;;
    strict-mode-on)     cmd_strict_mode_on ;;
    strict-mode-off)    cmd_strict_mode_off ;;

    # IM 安全
    upstreams)          cmd_upstreams ;;
    routes)             cmd_routes ;;
    bind)               cmd_bind "$@" ;;
    migrate)            cmd_migrate "$@" ;;
    inbound-rules)      cmd_inbound_rules ;;
    outbound-rules)     cmd_outbound_rules ;;
    rule-hits)          cmd_rule_hits ;;
    reload)             cmd_reload ;;
    rate-limit)         cmd_rate_limit ;;
    logs)               cmd_logs "$@" ;;
    blocks)             cmd_blocks ;;
    warns)              cmd_warns ;;
    stats)              cmd_stats ;;
    export-audit)       cmd_export_audit "$@" ;;
    timeline)           cmd_timeline "$@" ;;

    # LLM 安全
    llm-status)         cmd_llm_status ;;
    llm-overview)       cmd_llm_overview ;;
    llm-rules)          cmd_llm_rules ;;
    canary)             cmd_canary ;;
    canary-rotate)      cmd_canary_rotate ;;
    budget)             cmd_budget ;;
    owasp)              cmd_owasp ;;

    # 威胁分析
    risk-top)           cmd_risk_top ;;
    anomaly)            cmd_anomaly ;;
    redteam)            cmd_redteam ;;
    redteam-reports)    cmd_redteam_reports ;;
    behavior)           cmd_behavior ;;
    attack-chains)      cmd_attack_chains ;;

    # 安全治理
    tenants)            cmd_tenants ;;
    leaderboard)        cmd_leaderboard ;;
    honeypot)           cmd_honeypot ;;
    sessions)           cmd_sessions ;;
    generate-report)    cmd_generate_report "$@" ;;
    reports)            cmd_reports ;;

    # 安全画像 (v33.0)
    security-profile)   cmd_security_profile "$@" ;;
    security-profiles)  cmd_security_profiles ;;

    # Gateway 远程管理
    gw-restart)         cmd_gw_restart "$@" ;;
    gw-update)          cmd_gw_update "$@" ;;
    gw-config)          cmd_gw_config "$@" ;;
    gw-sessions)        cmd_gw_sessions "$@" ;;
    gw-approvals)       cmd_gw_approvals "$@" ;;
    gw-memory)          cmd_gw_memory "$@" ;;

    # 运维
    backup)             cmd_backup ;;
    backups)            cmd_backups ;;
    config)             cmd_config ;;
    notifications)      cmd_notifications ;;
    simulate)           cmd_simulate ;;
    bigscreen)          cmd_bigscreen ;;
    report)             cmd_report ;;

    # 帮助 & 版本
    help|-h|--help)     cmd_help ;;
    version|-v|--version) echo "🦞 龙虾卫士 CLI v${VERSION}" ;;

    *) echo "❌ 未知命令: $cmd"; echo "运行 '$0 help' 查看可用命令"; exit 1 ;;
  esac
}

main "$@"
