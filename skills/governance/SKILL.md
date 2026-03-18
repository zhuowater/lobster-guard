# 🦞 龙虾卫士 · 安全治理 — v17.1

AI Agent 安全平台的治理与运营子系统。覆盖多租户管理、报告引擎、安全排行榜、Agent 蜜罐、Prompt A/B 测试、会话回放与 Prompt 版本追踪七大能力域，为组织提供从租户隔离到合规审计的全链路安全治理能力。

## 通用信息

| 项目 | 值 |
|---|---|
| 基础 URL | `http://10.44.96.142:9090` |
| 认证 | `Authorization: Bearer <token>` |
| 响应格式 | JSON |

---

## API 参考

### 多租户管理

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/tenants` | 租户列表 |
| POST | `/api/v1/tenants` | 创建租户 |
| GET | `/api/v1/tenants/resolve` | 解析租户（按域名/标识） |
| GET | `/api/v1/tenants/:id` | 租户详情 |
| PUT | `/api/v1/tenants/:id` | 更新租户 |
| DELETE | `/api/v1/tenants/:id` | 删除租户 |
| GET | `/api/v1/tenants/:id/members` | 成员列表 |
| POST | `/api/v1/tenants/:id/members` | 添加成员 |
| DELETE | `/api/v1/tenants/:id/members/:uid` | 移除成员 |
| GET | `/api/v1/tenants/:id/config` | 租户安全配置 |
| PUT | `/api/v1/tenants/:id/config` | 更新安全配置 |

### 报告引擎

| 方法 | 端点 | 说明 |
|---|---|---|
| POST | `/api/v1/reports/generate` | 生成报告（body: `type=daily\|weekly\|monthly`） |
| GET | `/api/v1/reports` | 报告列表 |
| GET | `/api/v1/reports/:id` | 报告详情 |
| GET | `/api/v1/reports/:id/download` | 下载报告（HTML 内联 CSS） |
| DELETE | `/api/v1/reports/:id` | 删除报告 |

### 安全排行榜

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/leaderboard` | 排行榜（跨租户安全评分对比） |
| GET | `/api/v1/leaderboard/heatmap` | 攻击热力图 |
| GET | `/api/v1/leaderboard/sla` | SLA 达成率（三档判定） |
| PUT | `/api/v1/leaderboard/sla/config` | SLA 配置 |

### Agent 蜜罐

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/honeypot/templates` | 蜜罐模板列表（8 种预置） |
| POST | `/api/v1/honeypot/templates` | 创建蜜罐 |
| PUT | `/api/v1/honeypot/templates/:id` | 更新蜜罐 |
| DELETE | `/api/v1/honeypot/templates/:id` | 删除蜜罐 |
| GET | `/api/v1/honeypot/triggers` | 引爆记录列表 |
| GET | `/api/v1/honeypot/triggers/:id` | 引爆详情 |
| GET | `/api/v1/honeypot/stats` | 蜜罐统计 |
| POST | `/api/v1/honeypot/test` | 测试蜜罐 |

### Prompt A/B 测试

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/ab-tests` | 测试列表 |
| POST | `/api/v1/ab-tests` | 创建测试 |
| GET | `/api/v1/ab-tests/:id` | 测试详情 |
| PUT | `/api/v1/ab-tests/:id` | 更新测试 |
| DELETE | `/api/v1/ab-tests/:id` | 删除测试 |
| POST | `/api/v1/ab-tests/:id/start` | 启动测试 |
| POST | `/api/v1/ab-tests/:id/stop` | 停止测试 |

### 会话回放

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/sessions/replay` | 会话列表 |
| GET | `/api/v1/sessions/replay/:trace_id` | 会话详情（入站+出站双向） |
| POST | `/api/v1/sessions/replay/:trace_id/tags` | 添加标签 |
| DELETE | `/api/v1/sessions/replay/tags/:id` | 删除标签 |
| GET | `/api/v1/sessions/risks` | 高风险会话 |
| POST | `/api/v1/sessions/risks/reset` | 重置风险标记 |

### Prompt 版本追踪

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/prompts` | Prompt 版本列表 |
| GET | `/api/v1/prompts/current` | 当前生效 Prompt |
| GET | `/api/v1/prompts/:hash` | Prompt 详情 |
| GET | `/api/v1/prompts/:hash/diff` | Prompt Diff（LCS 行级对比） |

---

## 意图映射表

| 用户意图 | 调用端点 | 备注 |
|---|---|---|
| 列出租户 | `GET /api/v1/tenants` | |
| 创建租户 | `POST /api/v1/tenants` | body: 租户信息 |
| 查看租户详情 | `GET /api/v1/tenants/:id` | 需要租户 ID |
| 更新/删除租户 | `PUT/DELETE /api/v1/tenants/:id` | 需要租户 ID |
| 管理租户成员 | `GET/POST /api/v1/tenants/:id/members` | |
| 移除成员 | `DELETE /api/v1/tenants/:id/members/:uid` | 需要租户 ID + 用户 ID |
| 查看/修改租户安全配置 | `GET/PUT /api/v1/tenants/:id/config` | |
| 生成安全报告 | `POST /api/v1/reports/generate` | body: type=daily/weekly/monthly |
| 查看报告列表 | `GET /api/v1/reports` | |
| 下载报告 | `GET /api/v1/reports/:id/download` | 返回 HTML |
| 查看排行榜 | `GET /api/v1/leaderboard` | 跨租户对比 |
| 查看攻击热力图 | `GET /api/v1/leaderboard/heatmap` | |
| 查看 SLA 达成率 | `GET /api/v1/leaderboard/sla` | 三档判定 |
| 配置 SLA | `PUT /api/v1/leaderboard/sla/config` | |
| 查看蜜罐模板 | `GET /api/v1/honeypot/templates` | 8 种预置 |
| 创建/更新蜜罐 | `POST/PUT /api/v1/honeypot/templates` | |
| 查看引爆记录 | `GET /api/v1/honeypot/triggers` | |
| 蜜罐统计 | `GET /api/v1/honeypot/stats` | |
| 测试蜜罐 | `POST /api/v1/honeypot/test` | |
| 查看 A/B 测试 | `GET /api/v1/ab-tests` | |
| 创建 A/B 测试 | `POST /api/v1/ab-tests` | |
| 启动/停止 A/B 测试 | `POST /api/v1/ab-tests/:id/start\|stop` | 需要测试 ID |
| 查看会话列表 | `GET /api/v1/sessions/replay` | |
| 回放会话 | `GET /api/v1/sessions/replay/:trace_id` | 入站+出站双向 |
| 给会话打标签 | `POST /api/v1/sessions/replay/:trace_id/tags` | |
| 查看高风险会话 | `GET /api/v1/sessions/risks` | |
| 查看 Prompt 版本 | `GET /api/v1/prompts` | |
| 查看当前 Prompt | `GET /api/v1/prompts/current` | |
| 对比 Prompt 变更 | `GET /api/v1/prompts/:hash/diff` | LCS 行级对比 |
