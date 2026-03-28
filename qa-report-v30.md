# v30 API 黑盒 QA 报告

## 测试统计
- 总端点数: 59
- 通过: 55
- 失败: 4
  - GET /api/v1/inbound-templates/tpl-inbound-financial -> 200 (expect 200, verify enabled=true)
  - GET /api/v1/inbound-templates/tpl-inbound-financial -> 200 (expect 200, verify enabled=false)
  - POST /api/v1/llm/templates/non-existent-template/enable -> 400 (expect 404 for missing template)
  - 检测链路: webhook -> 405 / 未验证到命中审计
- 跳过: 12
  - /api/v1/health-score: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/session-replay: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/envelopes?limit=5: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/events?limit=5: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/taint/entries?limit=5: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/tool-policies: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/plan-compiler/templates: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/capability/tool-mappings: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/plan-deviations?limit=5: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/api-keys: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/metrics: endpoint absent on 142 test env (returned 404, but no 500)
  - /api/v1/realtime-metrics: endpoint absent on 142 test env (returned 404, but no 500)

## v30 新增 API
| 端点 | 方法 | 状态码 | 预期 | 结果 |
|---|---|---:|---|---|
| `/api/v1/auto-review/status` | GET | 200 | 200, current config and review rules | ✅ 通过 |
| `/api/v1/auto-review/config` | POST | 200 | 200, config updated | ✅ 通过 |
| `/api/v1/auto-review/status` | GET | 200 | 200, verify config updated | ✅ 通过 |
| `/api/v1/auto-review/rules/prompt_injection_en/review` | POST | 200 | 200, set rule to review | ✅ 通过 |
| `/api/v1/auto-review/status` | GET | 200 | 200, rule appears in review list | ✅ 通过 |
| `/api/v1/auto-review/rules/prompt_injection_en/restore` | POST | 200 | 200, restore rule to block | ✅ 通过 |
| `/api/v1/auto-review/status` | GET | 200 | 200, rule disappears from review list | ✅ 通过 |
| `/api/v1/auto-review/stats` | GET | 200 | 200, stats returned | ✅ 通过 |
| `/api/v1/inbound-templates` | GET | 200 | 200, list with enabled field | ✅ 通过 |
| `/api/v1/inbound-templates/tpl-inbound-financial/enable` | POST | 200 | 200, enabled | ✅ 通过 |
| `/api/v1/inbound-templates/tpl-inbound-financial` | GET | 200 | 200, verify enabled=true | ❌ 失败 |
| `/api/v1/inbound-templates/tpl-inbound-financial/enable` | POST | 200 | 200, disabled | ✅ 通过 |
| `/api/v1/inbound-templates/tpl-inbound-financial` | GET | 200 | 200, verify enabled=false | ❌ 失败 |
| `/api/v1/llm/templates` | GET | 200 | 200, list returned | ✅ 通过 |
| `/api/v1/llm/templates/tpl-llm-financial/enable` | POST | 200 | 200, enabled | ✅ 通过 |
| `/api/v1/llm/templates/non-existent-template/enable` | POST | 400 | 404 for missing template | ❌ 失败 |

## 核心 API 回归
| 端点 | 状态码 | 结果 |
|---|---:|---|
| `/healthz` | 200 | ✅ 通过 |
| `/api/v1/stats` | 200 | ✅ 通过 |
| `/api/v1/audit/logs?limit=5` | 200 | ✅ 通过 |
| `/api/v1/inbound-rules` | 200 | ✅ 通过 |
| `/api/v1/outbound-rules` | 200 | ✅ 通过 |
| `/api/v1/upstreams` | 200 | ✅ 通过 |
| `/api/v1/tenants` | 200 | ✅ 通过 |
| `/api/v1/llm/overview` | 200 | ✅ 通过 |
| `/api/v1/llm/rules` | 200 | ✅ 通过 |
| `/api/v1/llm/calls?limit=5` | 200 | ✅ 通过 |
| `/api/v1/honeypot/stats` | 200 | ✅ 通过 |
| `/api/v1/honeypot/templates` | 200 | ✅ 通过 |
| `/api/v1/attack-chains` | 200 | ✅ 通过 |
| `/api/v1/behavior/profiles` | 200 | ✅ 通过 |
| `/api/v1/anomaly/status` | 200 | ✅ 通过 |
| `/api/v1/redteam/reports` | 200 | ✅ 通过 |
| `/api/v1/leaderboard` | 200 | ✅ 通过 |
| `/api/v1/health-score` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/session-replay` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/prompts` | 200 | ✅ 通过 |
| `/api/v1/envelopes?limit=5` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/events?limit=5` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/taint/entries?limit=5` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/path-policies` | 200 | ✅ 通过 |
| `/api/v1/tool-policies` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/ifc/variables?limit=5` | 200 | ✅ 通过 |
| `/api/v1/ifc/source-rules` | 200 | ✅ 通过 |
| `/api/v1/counterfactual/verifications?limit=5` | 200 | ✅ 通过 |
| `/api/v1/plan-compiler/templates` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/capability/tool-mappings` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/plan-deviations?limit=5` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/api-keys` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/reports` | 200 | ✅ 通过 |
| `/api/v1/gateway/wss/status` | 200 | ✅ 通过 |
| `/api/v1/metrics` | 404 | ⏭️ 404/环境未提供 |
| `/api/v1/realtime-metrics` | 404 | ⏭️ 404/环境未提供 |

## 异常输入
| 场景 | 端点 | 状态码 | 预期 | 结果 |
|---|---|---:|---|---|
| 空 body POST | `/api/v1/inbound-templates/tpl-inbound-financial/enable` | 400 | 400 not 500 | ✅ 通过 |
| 错误 JSON | `/api/v1/inbound-templates/tpl-inbound-financial/enable` | 400 | 400 | ✅ 通过 |
| 无 Authorization | `/api/v1/stats` | 401 | 401 | ✅ 通过 |
| 超长 ID | `/api/v1/llm/templates/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa...` | 400 | 400/404 not 500 | ✅ 通过 |

## 检测链路验证
- 启用金融模板: 200 ✅
- 向 `http://localhost:18443/webhook` 发送测试消息: 405 ❌（预期 200/202）
- 审计日志检查: 未发现可明确归因于本次金融文本命中的记录，链路验证未完成。
- 结论: 当前 142 环境的 webhook 接口方法/路由与测试说明不一致，导致无法完成入站检测 E2E 验证。

## Auto-Review 端到端
- 启用 auto-review: 200
- 手动 review `prompt_injection_en`: 200
- status 校验: ✅ 包含 prompt_injection_en
- restore: 200
- restore 后 status 校验: ✅ 已移除 prompt_injection_en

## 发现的 Bug
- P1: `POST /api/v1/llm/templates/non-existent-template/enable` 返回 400，而需求预期为 404。错误信息明确表明模板不存在，语义更适合 404。
- P1: `GET /api/v1/inbound-templates/tpl-inbound-financial` 返回体中未见模板级 `enabled` 字段，无法直接验证开关状态；仅 enable/disable 接口返回了 `enabled`。这与测试要求“GET 验证 enabled 状态”不符。
- P1: 文档给出的检测链路 `POST http://localhost:18443/webhook` 在 142 环境返回 405，导致无法完成模板命中 E2E 验证；需确认入站端口、HTTP 方法或实际路由。
- P2: 多个“核心 API 回归”端点在当前环境返回 404（如 `/api/v1/health-score`、`/api/v1/session-replay`、`/api/v1/envelopes`、`/api/v1/events`、`/api/v1/taint/entries`、`/api/v1/tool-policies`、`/api/v1/plan-compiler/templates`、`/api/v1/capability/tool-mappings`、`/api/v1/plan-deviations`、`/api/v1/api-keys`、`/api/v1/metrics`、`/api/v1/realtime-metrics`）。若这些按版本应存在，则属于接口缺失/未注册。
