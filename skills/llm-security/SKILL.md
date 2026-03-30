# 🦞 龙虾卫士 · LLM 安全域 — v33.0

LLM 反向代理的安全管理技能。覆盖 LLM 代理状态监控、安全规则引擎、Canary Token 泄露检测、预算管控、OWASP LLM Top10 合规矩阵及审计导出。
所有 API 调用需带 `Authorization: Bearer <token>` 头，基础 URL 为 `http://10.44.96.142:9090`。

---

## API 参考

### LLM 状态与概览

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/status` | LLM 代理运行状态 | — |
| GET | `/api/v1/llm/overview` | LLM 概览（调用数/错误率/成本/模型分布） | — |

### LLM 调用与工具

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/calls` | LLM 调用列表 | ?limit=&offset=&model=&since= |
| GET | `/api/v1/llm/tools` | 工具调用列表 | — |
| GET | `/api/v1/llm/tools/stats` | 工具调用统计 | — |
| GET | `/api/v1/llm/tools/timeline` | 工具调用时间线 | — |

### LLM 配置

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/config` | 获取 LLM 配置 | — |
| PUT | `/api/v1/llm/config` | 更新 LLM 配置 | body: 配置对象 |

### LLM 规则引擎

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/rules` | LLM 规则列表 | — |
| POST | `/api/v1/llm/rules` | 创建 LLM 规则 | body: 规则定义 |
| GET | `/api/v1/llm/rules/hits` | LLM 规则命中率 | — |
| PUT | `/api/v1/llm/rules/:id` | 更新 LLM 规则 | :id = 规则 ID |
| DELETE | `/api/v1/llm/rules/:id` | 删除 LLM 规则 | :id = 规则 ID |
| POST | `/api/v1/llm/rules/:id/toggle-shadow` | 切换 Shadow Mode（观察/执行） | :id = 规则 ID |

### Canary Token

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/canary/status` | Canary Token 当前状态 | — |
| POST | `/api/v1/llm/canary/rotate` | 轮换 Canary Token | — |
| GET | `/api/v1/llm/canary/leaks` | Canary 泄露记录 | — |

### 预算管控

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/budget/status` | 预算使用状态 | — |
| GET | `/api/v1/llm/budget/violations` | 预算违规记录 | — |

### 审计与合规

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/llm/export` | LLM 审计导出 | ?format=csv\|json&since=&until= |
| GET | `/api/v1/llm/owasp-matrix` | OWASP LLM Top10 合规矩阵 | — |

---

## 意图映射

| 用户说 | 调用 |
|--------|------|
| LLM 状态 / LLM 代理状态 | `GET /api/v1/llm/status` |
| LLM 概览 / LLM 仪表盘 | `GET /api/v1/llm/overview` |
| LLM 调用记录 / 最近的 LLM 调用 | `GET /api/v1/llm/calls` |
| 查看某模型的调用 | `GET /api/v1/llm/calls?model=xxx` |
| 工具调用 / tool calls | `GET /api/v1/llm/tools` |
| 工具统计 | `GET /api/v1/llm/tools/stats` |
| 工具时间线 | `GET /api/v1/llm/tools/timeline` |
| LLM 配置 / 查看配置 | `GET /api/v1/llm/config` |
| 修改 LLM 配置 / 更新配置 | `PUT /api/v1/llm/config` |
| LLM 规则 / 查看 LLM 规则 | `GET /api/v1/llm/rules` |
| 添加 LLM 规则 / 创建 LLM 规则 | `POST /api/v1/llm/rules` |
| LLM 规则命中率 | `GET /api/v1/llm/rules/hits` |
| 更新 LLM 规则 | `PUT /api/v1/llm/rules/:id` |
| 删除 LLM 规则 | `DELETE /api/v1/llm/rules/:id` |
| 切换影子模式 / shadow mode | `POST /api/v1/llm/rules/:id/toggle-shadow` |
| Canary 状态 / 金丝雀状态 | `GET /api/v1/llm/canary/status` |
| 轮换 Canary / 换 Token | `POST /api/v1/llm/canary/rotate` |
| Canary 泄露 / 有没有泄露 | `GET /api/v1/llm/canary/leaks` |
| 预算 / 预算状态 / 花了多少钱 | `GET /api/v1/llm/budget/status` |
| 预算违规 / 超支记录 | `GET /api/v1/llm/budget/violations` |
| 导出 LLM 日志 / LLM 审计导出 | `GET /api/v1/llm/export?format=csv` |
| OWASP 矩阵 / LLM Top10 / 合规检查 | `GET /api/v1/llm/owasp-matrix` |

> Phase 1 新功能请见对应子技能：执行信封 `../envelope/`、事件总线 `../event-bus/`、自进化 `../evolution/`、语义检测 `../semantic/`、奇点蜜罐 `../singularity/`、工具策略 `../tool-policy/`、污染追踪 `../taint/`、响应缓存 `../cache/`、API网关 `../gateway/`
> v22.x 新功能：Gateway Monitor、AOC、Skill Directory — 详见 `../gateway/SKILL.md`
> v22.x Threat Map 修复：AI 响应路径现经 LLM Detection 节点再到 OpenClaw，确保出站 LLM 响应同样经过安全检测。
