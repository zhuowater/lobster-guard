# 📡 API 参考（~227 路由）

> 返回 [README](../README.md)

## 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 管理后台 Dashboard |
| GET | `/healthz` | 健康检查 + 系统概览 |
| GET | `/metrics` | Prometheus 指标导出 |
| POST | `/api/v1/auth/login` | JWT 登录 |

## IM 安全域

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams` | 列出所有上游容器 |
| GET | `/api/v1/routes` | 列出用户路由绑定 |
| POST | `/api/v1/routes/bind` | 绑定用户到上游 |
| POST | `/api/v1/routes/unbind` | 解绑用户路由 |
| POST | `/api/v1/routes/migrate` | 迁移用户到新上游 |
| POST | `/api/v1/routes/batch-bind` | 批量绑定 |
| GET | `/api/v1/routes/stats` | 路由统计 |
| GET | `/api/v1/inbound-rules` | 入站规则列表 |
| POST | `/api/v1/inbound-rules/reload` | 热更新入站规则 |
| GET | `/api/v1/outbound-rules` | 出站规则列表 |
| POST | `/api/v1/outbound-rules` | 出站规则 CRUD（v18）|
| PUT | `/api/v1/outbound-rules/:name` | 更新出站规则（v18）|
| DELETE | `/api/v1/outbound-rules/:name` | 删除出站规则（v18）|
| GET | `/api/v1/audit/logs` | 查询审计日志 |
| GET | `/api/v1/audit/export` | 导出审计日志 |
| GET | `/api/v1/audit/timeline` | 时间线统计 |
| GET | `/api/v1/stats` | 统计概览 |

## LLM 安全域

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/llm/overview` | LLM 代理概览 |
| GET | `/api/v1/llm/rules` | LLM 规则列表（11 默认 + 自定义）|
| POST | `/api/v1/llm/rules` | LLM 规则 CRUD |
| GET | `/api/v1/llm/cost` | Token 成本看板 |
| GET | `/api/v1/llm/audit` | LLM 审计日志 |

## 威胁分析

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/user-profiles` | 攻击者画像列表 |
| GET | `/api/v1/user-profiles/:id` | 用户详情 |
| GET | `/api/v1/behavior-profiles` | 行为画像 |
| GET | `/api/v1/attack-chains` | 攻击链检测结果 |
| GET | `/api/v1/anomaly/alerts` | 异常检测告警 |
| GET | `/api/v1/health-score` | 健康分 + OWASP 矩阵 |
| POST | `/api/v1/redteam/run` | 触发 Red Team 测试 |
| GET | `/api/v1/redteam/results` | Red Team 结果 |

## 安全治理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/reports` | 安全报告列表 |
| POST | `/api/v1/reports/generate` | 生成报告（日报/周报/合规）|
| GET | `/api/v1/reports/:id/pdf` | 导出 PDF |
| GET | `/api/v1/leaderboard` | 安全排行榜 |
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants` | 创建租户 |

## 高级功能

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/honeypots` | 蜜罐列表 |
| POST | `/api/v1/honeypots` | 创建蜜罐 |
| GET | `/api/v1/honeypots/triggers` | 引爆记录 |
| GET | `/api/v1/ab-tests` | A/B 测试列表 |
| POST | `/api/v1/ab-tests` | 创建 A/B 测试 |
| GET | `/api/v1/sessions/replay/:id` | 会话回放 |
| GET | `/api/v1/prompt-tracker` | Prompt 版本追踪 |
| GET | `/api/v1/layouts` | 自定义布局列表 |
| POST | `/api/v1/layouts` | 保存布局 |
| GET | `/api/v1/bigscreen/data` | 态势大屏数据 |
| POST | `/api/v1/simulate/traffic` | 端到端模拟测试（v18）|

## 系统管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/users` | 用户列表 |
| POST | `/api/v1/users` | 创建用户（v18）|
| PUT | `/api/v1/users/:id` | 更新用户（v18）|
| DELETE | `/api/v1/users/:id` | 删除用户（v18）|
| POST | `/api/v1/backup` | 创建数据库备份 |
| GET | `/api/v1/backups` | 列出备份 |
| GET | `/api/v1/metrics/realtime` | 实时统计 |
| GET | `/api/v1/ws/connections` | WebSocket 连接列表 |

> 以上为主要 API 摘要，完整 ~227 路由参见源码 `src/api.go`。

## API 调用示例

```bash
TOKEN="your-management-token"

# JWT 登录
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"xxx"}' \
  http://localhost:9090/api/v1/auth/login | jq .

# 健康检查
curl -s http://localhost:9090/healthz | jq .

# 端到端模拟测试
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/simulate/traffic | jq .

# 查询拦截日志
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:9090/api/v1/audit/logs?action=block&limit=20" | jq .

# 触发 Red Team 测试
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/redteam/run | jq .
```

## Phase 1 新增 API (v18-v20)

### 执行信封 (v18.0)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/envelopes/stats` | 信封统计 |
| GET | `/api/v1/envelopes/list` | 信封列表 |
| GET | `/api/v1/envelopes/verify/:id` | 验证单个信封 |
| GET | `/api/v1/envelopes/batches` | Merkle 批次列表 |
| GET | `/api/v1/envelopes/batch/:id` | 验证 Merkle 批次 |
| GET | `/api/v1/envelopes/chain/:id` | 信封链查询 |
| GET | `/api/v1/envelopes/proof/:id` | Merkle Proof |
| GET/PUT | `/api/v1/envelopes/config` | 信封配置 |

### 事件总线 (v18.1)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/events/stats` | 事件统计 |
| GET | `/api/v1/events/list` | 事件列表 |
| GET | `/api/v1/events/deliveries` | 投递记录 |
| GET/POST/PUT/DELETE | `/api/v1/events/targets` | Webhook 目标 CRUD |
| GET | `/api/v1/events/chains` | ActionChain 列表 |
| POST | `/api/v1/events/test` | 测试事件发送 |

### 自适应决策 (v18.3)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/adaptive/stats` | 决策统计 |
| GET/PUT | `/api/v1/adaptive/config` | 决策配置 |
| POST | `/api/v1/adaptive/feedback` | 反馈误伤/漏报 |
| GET | `/api/v1/adaptive/proof/:id` | 决策证明 |

### 奇点蜜罐 (v18.3)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/singularity/budget` | 拓扑预算 |
| GET | `/api/v1/singularity/recommend` | 推荐放置 |
| GET/PUT | `/api/v1/singularity/config` | 奇点配置 |
| GET | `/api/v1/singularity/history` | 历史记录 |
| GET | `/api/v1/singularity/templates` | 蜜罐模板 |

### 对抗性自进化 (v19.0)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/evolution/stats` | 进化统计 |
| GET | `/api/v1/evolution/log` | 进化日志 |
| POST | `/api/v1/evolution/run` | 手动运行一轮 |
| GET | `/api/v1/evolution/strategies` | 变异策略 |
| GET/PUT | `/api/v1/evolution/config` | 进化配置 |

### 语义检测 (v19.1)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/semantic/stats` | 语义统计 |
| POST | `/api/v1/semantic/analyze` | 实时语义分析 |
| GET | `/api/v1/semantic/patterns` | 攻击模式库 |
| GET/PUT | `/api/v1/semantic/config` | 语义配置 |

### 蜜罐深度交互 (v19.2)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/honeypot/deep/stats` | 深度交互统计 |
| GET | `/api/v1/honeypot/loyalty` | 忠诚度排行 |
| GET | `/api/v1/honeypot/loyalty/:id` | 单个攻击者详情 |
| POST | `/api/v1/honeypot/deep/record` | 记录交互 |

### 工具策略 (v20.0)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/tools/stats` | 策略统计 |
| POST | `/api/v1/tools/evaluate` | 实时工具评估 |
| GET/POST/PUT/DELETE | `/api/v1/tools/rules` | 规则 CRUD |
| GET | `/api/v1/tools/events` | 工具事件日志 |
| GET/PUT | `/api/v1/tools/config` | 策略配置 |

### 污染追踪 (v20.1)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/taint/stats` | 污染统计 |
| POST | `/api/v1/taint/scan` | 实时污染扫描 |
| GET | `/api/v1/taint/active` | 活跃污染列表 |
| GET/PUT | `/api/v1/taint/config` | 污染配置 |
| GET | `/api/v1/taint/trace/:id` | 污染链详情 |

### 污染逆转 (v20.2)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/reversal/stats` | 逆转统计 |
| GET | `/api/v1/reversal/records` | 逆转记录 |
| GET | `/api/v1/reversal/templates` | 逆转模板 |
| POST | `/api/v1/reversal/test` | 测试逆转 |
| GET/PUT | `/api/v1/reversal/config` | 逆转配置 |

### LLM 缓存 (v20.3)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/cache/stats` | 缓存统计 |
| GET | `/api/v1/cache/entries` | 缓存条目 |
| POST | `/api/v1/cache/lookup` | 缓存查询 |
| DELETE | `/api/v1/cache/entries` | 清除全部 |
| DELETE | `/api/v1/cache/tenant/:id` | 清除租户 |
| GET/PUT | `/api/v1/cache/config` | 缓存配置 |

### API 网关 (v20.4)
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway/stats` | 网关统计 |
| GET/POST/PUT/DELETE | `/api/v1/gateway/routes` | 路由 CRUD |
| POST | `/api/v1/gateway/token` | 生成 JWT |
| POST | `/api/v1/gateway/validate` | 验证 JWT |
| GET | `/api/v1/gateway/log` | 网关日志 |
| GET/PUT | `/api/v1/gateway/config` | 网关配置 |

### Dashboard 企业级打磨 (v20.7)
| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/v1/config/settings` | 批量更新配置（回写 config.yaml + 内存） |
| POST | `/api/v1/alerts/test` | 发送测试告警 |
| PUT | `/api/v1/alerts/config` | 更新告警配置 |
| POST | `/api/v1/routes/batch-unbind` | 批量解绑路由 |
| POST | `/api/v1/routes/batch-migrate` | 批量迁移路由 |
| GET | `/api/v1/anomaly/metric-thresholds` | 获取指标独立阈值 |
| PUT | `/api/v1/anomaly/metric-thresholds/:name` | 设置指标阈值 |
| GET | `/api/v1/anomaly/trend/:name` | 24h 趋势数据 |
| POST | `/api/v1/prompts/:hash/tag` | 设置 Prompt 版本标签 |
| POST | `/api/v1/prompts/:hash/rollback` | 回滚 Prompt 版本 |
| GET | `/api/v1/prompts/stats` | Prompt 统计概览 |
| POST | `/api/v1/taint/cleanup` | 批量清理过期标记 |
| DELETE | `/api/v1/taint/entry/:trace_id` | 删除污染标记 |
| POST | `/api/v1/taint/inject` | 手动注入标记 |
