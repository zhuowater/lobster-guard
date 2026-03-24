# 🦞 龙虾卫士 · 威胁分析 — v22.4

AI Agent 安全平台的威胁分析子系统。覆盖攻击者画像、用户风险评分、Agent 行为画像、攻击链检测、异常基线检测与 Red Team Autopilot 六大能力域，为安全运营提供从被动检测到主动验证的完整威胁分析闭环。

## 通用信息

| 项目 | 值 |
|---|---|
| 基础 URL | `http://10.44.96.142:9090` |
| 认证 | `Authorization: Bearer <token>` |
| 响应格式 | JSON |

---

## API 参考

### 攻击者画像 / 用户风险

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/users/risk-top` | 风险 TOP 用户排行 |
| GET | `/api/v1/users/risk-stats` | 全局风险统计概览 |
| GET | `/api/v1/users/risk/:id` | 用户风险详情（5 维度评分 0-100） |
| GET | `/api/v1/users/timeline/:id` | 用户行为时间线 |

> **5 维度评分**：每个维度 0-100，综合反映用户在不同安全维度的风险水位。

### 行为画像

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/behavior/profiles` | Agent 画像列表 |
| GET | `/api/v1/behavior/anomalies` | 行为异常列表 |
| GET | `/api/v1/behavior/patterns` | 行为模式汇总 |
| GET | `/api/v1/behavior/profiles/:agent_id` | Agent 详细画像 |
| POST | `/api/v1/behavior/profiles/:agent_id/scan` | 触发 Agent 行为扫描 |

### 攻击链检测

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/attack-chains` | 攻击链列表 |
| POST | `/api/v1/attack-chains/analyze` | 分析攻击链 |
| GET | `/api/v1/attack-chains/patterns` | 5 种预置攻击模式 |
| GET | `/api/v1/attack-chains/stats` | 攻击链统计 |
| GET | `/api/v1/attack-chains/:id` | 攻击链详情 |
| PUT | `/api/v1/attack-chains/:id/status` | 更新链状态（confirmed / ignored / processing） |

### 异常基线检测

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/anomaly/baselines` | 基线数据（7 天滑动窗口） |
| GET | `/api/v1/anomaly/alerts` | 异常告警（2σ / 3σ 阈值） |
| GET | `/api/v1/anomaly/status` | 检测器运行状态 |
| GET | `/api/v1/anomaly/metric/:name` | 指标详情（含 ±2σ 置信带） |
| GET | `/api/v1/anomaly/config` | 异常检测配置 |
| PUT | `/api/v1/anomaly/config` | 更新异常检测配置 |

### Red Team Autopilot

| 方法 | 端点 | 说明 |
|---|---|---|
| POST | `/api/v1/redteam/run` | 执行红队测试（33 攻击向量 × 6 OWASP 分类） |
| GET | `/api/v1/redteam/reports` | 红队报告列表 |
| GET | `/api/v1/redteam/vectors` | 攻击向量清单 |
| GET | `/api/v1/redteam/reports/:id` | 报告详情 |
| DELETE | `/api/v1/redteam/reports/:id` | 删除报告 |

---

## 意图映射表

| 用户意图 | 调用端点 | 备注 |
|---|---|---|
| 查看高风险用户 | `GET /api/v1/users/risk-top` | |
| 查看某用户风险 | `GET /api/v1/users/risk/:id` | 需要用户 ID |
| 查看用户行为时间线 | `GET /api/v1/users/timeline/:id` | 需要用户 ID |
| 风险整体统计 | `GET /api/v1/users/risk-stats` | |
| 列出 Agent 画像 | `GET /api/v1/behavior/profiles` | |
| 查看 Agent 行为详情 | `GET /api/v1/behavior/profiles/:agent_id` | 需要 agent_id |
| 扫描 Agent 行为 | `POST /api/v1/behavior/profiles/:agent_id/scan` | 需要 agent_id |
| 查看行为异常 | `GET /api/v1/behavior/anomalies` | |
| 查看行为模式 | `GET /api/v1/behavior/patterns` | |
| 查看攻击链 | `GET /api/v1/attack-chains` | |
| 分析攻击链 | `POST /api/v1/attack-chains/analyze` | |
| 查看攻击链详情 | `GET /api/v1/attack-chains/:id` | 需要链 ID |
| 确认/忽略攻击链 | `PUT /api/v1/attack-chains/:id/status` | body: status |
| 查看攻击模式 | `GET /api/v1/attack-chains/patterns` | 5 种预置模式 |
| 攻击链统计 | `GET /api/v1/attack-chains/stats` | |
| 查看异常基线 | `GET /api/v1/anomaly/baselines` | 7 天滑动窗口 |
| 查看异常告警 | `GET /api/v1/anomaly/alerts` | 2σ/3σ 阈值 |
| 检测器状态 | `GET /api/v1/anomaly/status` | |
| 查看某指标详情 | `GET /api/v1/anomaly/metric/:name` | 含 ±2σ 带 |
| 查看/修改异常配置 | `GET/PUT /api/v1/anomaly/config` | |
| 执行红队测试 | `POST /api/v1/redteam/run` | 33 向量 × 6 OWASP |
| 查看红队报告 | `GET /api/v1/redteam/reports` | |
| 查看攻击向量 | `GET /api/v1/redteam/vectors` | |
| 报告详情 | `GET /api/v1/redteam/reports/:id` | 需要报告 ID |
| 删除红队报告 | `DELETE /api/v1/redteam/reports/:id` | 需要报告 ID |
