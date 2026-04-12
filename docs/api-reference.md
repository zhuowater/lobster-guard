# 📡 API 参考（487 路由）

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
| GET | `/api/v1/source-classifier` | 读取全局来源分类规则 |
| PUT | `/api/v1/source-classifier` | 更新全局来源分类规则 |
| POST | `/api/v1/source-classifier/explain` | tenant-aware dry-run explain（返回 global/effective descriptor、PathDecision、CapabilityEvaluation） |

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
| POST | `/api/v1/users` | 创建用户 |
| PUT | `/api/v1/users/:id` | 更新用户 |
| DELETE | `/api/v1/users/:id` | 删除用户 |
| POST | `/api/v1/backup` | 创建数据库备份 |
| GET | `/api/v1/backups` | 列出备份 |
| GET | `/api/v1/metrics/realtime` | 实时统计 |
| GET | `/api/v1/config/settings` | 配置设置(含引擎开关) |
| PUT | `/api/v1/config/settings` | 更新配置设置 |
| GET | `/api/v1/config/validate` | 配置校验 |
| GET | `/api/v1/config/view` | 配置 YAML 查看 |

## 路径策略引擎 (v23)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/path-policies` | 路径策略列表 |
| POST | `/api/v1/path-policies` | 添加策略规则 |
| PUT | `/api/v1/path-policies/:id` | 更新策略 |
| DELETE | `/api/v1/path-policies/:id` | 删除策略 |
| GET | `/api/v1/path-policies/stats` | 路径统计 |
| GET | `/api/v1/path-policies/risk-gauge` | 实时风险仪表 |
| GET | `/api/v1/path-policies/templates` | 策略模板(含 AI Act) |
| POST | `/api/v1/path-policies/templates` | 创建策略模板 |

## 反事实验证 (v24)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/counterfactual/reports` | 验证报告 |
| GET | `/api/v1/counterfactual/verifications` | 验证记录 |
| GET | `/api/v1/counterfactual/cost` | 成本追踪 |
| GET | `/api/v1/counterfactual/effectiveness` | 效果评估 |
| GET | `/api/v1/counterfactual/adaptive-config` | 自适应配置 |
| PUT | `/api/v1/counterfactual/adaptive-config` | 更新自适应配置 |
| GET | `/api/v1/counterfactual/high-risk-tools` | 高风险工具列表 |
| POST | `/api/v1/counterfactual/high-risk-tools` | 添加高风险工具 |
| DELETE | `/api/v1/counterfactual/cache` | 清空验证缓存 |

## CaMeL 执行计划编译器 (v25)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/plans/compile` | 编译用户意图为执行计划 |
| POST | `/api/v1/plans/evaluate` | 评估 tool_call 是否符合计划 |
| GET | `/api/v1/plans/templates` | 模板列表(22个,6分类) |
| POST | `/api/v1/plans/templates` | 创建自定义模板 |
| PUT | `/api/v1/plans/templates/:id` | 更新模板 |
| DELETE | `/api/v1/plans/templates/:id` | 删除模板 |
| GET | `/api/v1/plans/active` | 活跃计划 |
| GET | `/api/v1/plans/violations` | 违规记录 |
| GET | `/api/v1/plans/stats` | 编译器统计 |

## Capability 权限系统 (v25)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/capabilities/mappings` | 工具→权限映射 |
| PUT | `/api/v1/capabilities/mappings/:tool` | 更新映射 |
| POST | `/api/v1/capabilities/contexts` | 初始化权限上下文 |
| GET | `/api/v1/capabilities/evaluations` | 权限评估记录 |
| GET | `/api/v1/capabilities/stats` | 权限统计 |

## 偏差检测 (v25)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/deviations/check` | 检查偏离计划 |
| GET | `/api/v1/deviations` | 偏差记录 |
| GET | `/api/v1/deviations/stats` | 偏差统计 |
| GET | `/api/v1/deviations/config` | 检测器配置 |
| PUT | `/api/v1/deviations/config` | 更新配置 |
| GET | `/api/v1/deviations/repair-policies` | 修复策略 |
| PUT | `/api/v1/deviations/repair-policies/:id` | 更新修复策略 |

## IFC 信息流控制 (v26)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/ifc/sources` | IFC 数据源规则 |
| POST | `/api/v1/ifc/sources` | 添加来源规则 |
| GET | `/api/v1/ifc/tool-requirements` | 工具要求 |
| POST | `/api/v1/ifc/tool-requirements` | 添加工具要求 |
| GET | `/api/v1/ifc/violations` | IFC 违规记录 |
| GET | `/api/v1/ifc/quarantine` | 隔离 LLM 路由 |
| GET | `/api/v1/ifc/variables` | IFC 变量 |
| GET | `/api/v1/ifc/config` | IFC 配置 |
| PUT | `/api/v1/ifc/config` | 更新 IFC 配置 |

## API Key 管理 (v27)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/apikeys` | API Key 列表 |
| POST | `/api/v1/apikeys` | 创建 API Key |
| PUT | `/api/v1/apikeys/:id` | 更新 API Key |
| DELETE | `/api/v1/apikeys/:id` | 删除 API Key |
| POST | `/api/v1/apikeys/:id/rotate` | 轮换 Key |
| POST | `/api/v1/apikeys/:id/bind` | 绑定租户 |

## Gateway 远程管理 (v29)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/gateway-monitor/upstreams` | 监控的上游 Gateway 列表 |
| PUT | `/api/v1/gateway-monitor/upstreams/:id` | 配置上游 Gateway Token/Origin |
| GET | `/api/v1/gateway-monitor/upstreams/:id/wss-status` | WSS 连接状态 |
| POST | `/api/v1/gateway-monitor/upstreams/:id/wss-connect` | 手动建立 WSS |
| POST | `/api/v1/gateway/rpc/:method` | WSS RPC 代理(88 methods) |
| POST | `/api/v1/tools/invoke` | tools/invoke fallback |

## 行业模板 (v31)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/industry-templates` | 行业模板列表(40×12) |
| POST | `/api/v1/industry-templates` | 创建模板 |
| PUT | `/api/v1/industry-templates/:id` | 更新模板 |
| DELETE | `/api/v1/industry-templates/:id` | 删除模板 |
| POST | `/api/v1/industry-templates/:id/toggle` | 启用/禁用模板 |

## 检测调试 (v32)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/debug/detect-all-layers` | 全链路四层检测 |
| GET | `/api/v1/sqlite/stats` | SQLite 统计 |
| GET | `/api/v1/sqlite/batch-stats` | 批量写入统计 |

## 安全画像 (v33)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/upstreams/:id/security-profile` | 单个上游安全画像(5维评分) |
| GET | `/api/v1/upstream-profiles` | 全部上游画像+分段统计 |

## Auto-Review (v31)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/auto-review/status` | 自动审查状态 |
| GET | `/api/v1/auto-review/stats` | 审查统计 |
| POST | `/api/v1/auto-review/rules/:id/review` | 触发 LLM 复核 |
| POST | `/api/v1/auto-review/rules/:id/restore` | 恢复规则 |

## 规则建议 + 进化 (v32)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/rule-suggestions` | 建议队列 |
| POST | `/api/v1/rule-suggestions/:id/accept` | 接受建议 |
| POST | `/api/v1/rule-suggestions/:id/reject` | 拒绝建议 |
| POST | `/api/v1/evolution/accept/:id` | 接受进化规则 |
| POST | `/api/v1/evolution/reject/:id` | 拒绝进化规则 |

## Merkle 审计 + 金丝雀 (v32)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/envelopes/merkle/batches` | Merkle 批次列表 |
| POST | `/api/v1/envelopes/merkle/verify-batch` | 批量验证 |
| GET | `/api/v1/canary/status` | 金丝雀状态 |
| POST | `/api/v1/canary/rotate` | 手动轮换 |
| GET | `/api/v1/canary/history` | 轮换历史 |

> 以上为 v23-v33 新增 API 摘要。完整 487 路由参见源码 `src/api.go`。

## API 调用示例

```bash
BASE="http://localhost:9090"

# JWT 登录
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"username":"admin","password": "***"}' \
  "$BASE/api/v1/auth/login" | jq .

# 健康检查
curl -s "$BASE/healthz" | jq .

# 下列管理接口请求需自行附带 Authorization: Bearer <MANAGEMENT_TOKEN>

# 查看全局 source classifier 配置
curl -s "$BASE/api/v1/source-classifier" | jq .

# 更新全局 source classifier 规则
curl -s -X PUT \
  -H "Content-Type: application/json" \
  "$BASE/api/v1/source-classifier" \
  -d '{
    "rules": [
      {
        "name": "corp-control-plane",
        "host_pattern": "^control\\.corp\\.example$",
        "category": "internal_control_plane",
        "confidentiality": 3,
        "integrity": 3,
        "trust_score": 0.91
      }
    ]
  }' | jq .

# 对单个 tool call 做 tenant-aware explain
curl -s -X POST \
  -H "Content-Type: application/json" \
  "$BASE/api/v1/source-classifier/explain" \
  -d '{
    "tenant_id": "tenant-a",
    "tool_name": "web_fetch",
    "tool_args": {
      "url": "https://docs.python.org/3/library/json.html"
    },
    "proposed_action": "shell_exec",
    "capability_action": "write"
  }' | jq .

# 端到端模拟测试
curl -s -X POST "$BASE/api/v1/simulate/traffic" | jq .

# 查询拦截日志
curl -s "$BASE/api/v1/audit/logs?action=block&limit=20" | jq .

# 触发 Red Team 测试
curl -s -X POST "$BASE/api/v1/redteam/run" | jq .
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

### Dashboard 企业级打磨 (v33.0)
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
