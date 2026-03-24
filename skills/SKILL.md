# 🦞 Lobster Guard — 龙虾卫士 v22.4 智能管控

AI Agent 安全网关的自然语言管控入口。通过子技能分域管理 ~290+ 个 API 端点。

## 连接配置

```bash
LOBSTER_GUARD_URL="http://10.44.96.142:9090"   # 管理 API 地址
LOBSTER_GUARD_TOKEN="test-token-2026"           # Bearer Token 或 JWT
```

所有 API 调用格式：`curl -s -H "Authorization: Bearer $TOKEN" $URL/<endpoint>`

## 快速状态检查

无需加载子技能：
- `GET /healthz` — 健康检查（公开）
- `GET /metrics` — Prometheus 指标（公开）
- `GET /api/v1/health/score` — 综合安全健康分
- `GET /api/v1/overview/summary` — 首页概览聚合

## 子技能路由

根据用户意图加载对应子技能的 SKILL.md：

| 用户意图关键词 | 子技能 | 路径 |
|---------------|--------|------|
| 规则、入站、出站、审计日志、路由、绑定、限流、策略路由、IM、检测 | **IM 安全域** | `im-security/SKILL.md` |
| LLM、模型、Token、成本、Canary、泄露、Budget、预算、OWASP、LLM规则 | **LLM 安全域** | `llm-security/SKILL.md` |
| 画像、行为、攻击链、异常、基线、红队、Red Team、风险评分、用户风险 | **威胁分析** | `threat-analysis/SKILL.md` |
| 租户、报告、排行榜、SLA、蜜罐、A/B测试、会话回放、Prompt追踪 | **安全治理** | `governance/SKILL.md` |
| 备份、恢复、诊断、配置、严格模式、大屏、布局、通知、模拟、演示 | **系统运维** | `ops/SKILL.md` |

## v22.x 新功能

- **Gateway Monitor** — 通过 `POST /tools/invoke` 协议监控上游 OpenClaw Gateway 实例
- **Agent Operations Center (AOC)** — 5 视图：Dashboard/Cards/Collab/Users/Skills
- **Per-Upstream AOC** — 每个上游 Gateway 在展开行中有独立的 Agent 标签页
- **Skill Directory** — 从 OpenClaw 文件系统扫描并展示已安装技能
- **SVG Icon System** — 全部 emoji 替换为 feather-style SVG 图标
- **Threat Map Fix** — AI 响应路径修正为经 LLM Detection 再到 OpenClaw
- **Gateway Token Config** — 在上游管理页面配置 Token

## 统计数据 (v22.4)

| 指标 | 值 |
|------|-----|
| Go 源文件 | 71 个，~76,400 行 |
| Vue 文件 | 61 个，~25,500 行 |
| 测试函数 | 1006 个 |
| API 端点 | ~290+ |
| Dashboard 页面 | 39 页，3 模式（Classic/Narrative/Threat Map） |
| 依赖 | 4（sqlite3 + yaml.v3 + gorilla/websocket + x/crypto） |
| Commits | 212 |

## 调用流程

1. 识别用户意图 → 匹配上表关键词
2. `read` 对应子技能 SKILL.md
3. 按子技能中的 API 参考执行操作
4. 如果涉及多个域（如"全面安全报告"），依次加载相关子技能

## CLI 工具

`lobster-cli.sh` 提供命令行快捷方式，覆盖常用操作（v22.4）：

```bash
./lobster-cli.sh help          # 查看所有命令
./lobster-cli.sh status        # 健康检查
./lobster-cli.sh report        # 综合安全报告
```

## 认证方式

- **Bearer Token**: `Authorization: Bearer <management_token>`（简单模式）
- **JWT**: 先 `POST /api/v1/auth/login` 获取 token，后续请求带上（Dashboard 登录模式）
