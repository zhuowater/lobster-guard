# 🦞 龙虾卫士 · 系统运维 — v17.1

龙虾卫士管理平台的系统运维子技能，覆盖健康检查、安全诊断、备份恢复、态势感知、认证管理等核心运维操作。所有 API 基于 `http://10.44.96.142:9090`，需 `Authorization: Bearer <token>` 认证（健康检查与指标端点除外）。

---

## API 参考

### 系统状态

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/healthz` | 健康检查 | 公开 |
| GET | `/metrics` | Prometheus 指标 | 公开 |
| GET | `/api/v1/health/score` | 综合安全健康分（0-100 + 7 天趋势） | ✅ |
| GET | `/api/v1/overview/summary` | 首页概览聚合数据 | ✅ |
| GET | `/api/v1/system/diag` | 系统诊断（CPU/内存/磁盘/DB/连接） | ✅ |
| GET | `/api/v1/config/view` | 查看配置（脱敏显示） | ✅ |

### 严格模式

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/system/strict-mode` | 严格模式状态 | ✅ |
| POST | `/api/v1/system/strict-mode` | 切换严格模式（warn→block + 激活 Shadow 规则） | ✅ |

> POST body: `{"enabled": true}` 开启，`{"enabled": false}` 关闭

### 通知中心

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/notifications` | 通知列表（Canary 泄露/预算超限/规则命中等） | ✅ |
| GET | `/api/v1/alerts/history` | 告警历史 | ✅ |
| GET | `/api/v1/alerts/config` | 告警配置 | ✅ |

### 备份恢复

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/backup` | 创建备份 | ✅ |
| GET | `/api/v1/backups` | 备份列表 | ✅ |
| DELETE | `/api/v1/backups/:name` | 删除备份 | ✅ |
| POST | `/api/v1/backups/:name/restore` | 恢复备份 | ✅ |
| GET | `/api/v1/backups/:name/download` | 下载备份文件 | ✅ |

### 态势大屏

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/bigscreen/data` | 大屏数据（聚合全部安全指标） | ✅ |

### 自定义布局

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/layouts` | 布局列表 | ✅ |
| POST | `/api/v1/layouts` | 创建布局 | ✅ |
| GET | `/api/v1/layouts/presets` | 预设模板（运维/安全分析师/管理层/LLM 监控） | ✅ |
| POST | `/api/v1/layouts/active` | 设置活跃布局 | ✅ |
| GET | `/api/v1/layouts/:id` | 布局详情 | ✅ |
| PUT | `/api/v1/layouts/:id` | 更新布局 | ✅ |
| DELETE | `/api/v1/layouts/:id` | 删除布局 | ✅ |

### 端到端模拟

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/simulate/traffic` | 运行 9 场景全链路模拟测试 | ✅ |

### 演示数据

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/demo/seed` | 注入演示数据 | ✅ |
| DELETE | `/api/v1/demo/clear` | 清除演示数据 | ✅ |

### 认证管理

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 登录（返回 JWT） | — |
| GET | `/api/v1/auth/check` | Token 验证 | ✅ |
| POST | `/api/v1/auth/logout` | 登出 | ✅ |
| GET | `/api/v1/auth/me` | 当前用户信息 | ✅ |
| POST | `/api/v1/auth/password` | 修改密码 | ✅ |
| GET | `/api/v1/auth/users` | 用户列表 | ✅ |
| POST | `/api/v1/auth/users` | 创建用户 | ✅ |
| PUT | `/api/v1/auth/users/:id` | 更新用户 | ✅ |
| DELETE | `/api/v1/auth/users/:id` | 删除用户 | ✅ |

### 操作审计

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/op-audit` | 操作审计日志 | ✅ |

### 容器注册（需 registration_token）

| 方法 | 端点 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/register` | 注册容器 | reg_token |
| POST | `/api/v1/heartbeat` | 心跳上报 | reg_token |
| POST | `/api/v1/deregister` | 注销容器 | reg_token |

---

## 意图映射表

| 用户意图 | 命令 / API | 示例 |
|----------|-----------|------|
| 系统健康吗 / 服务正常吗 | `GET /healthz` | "龙虾卫士状态" |
| 安全评分多少 | `GET /api/v1/health/score` | "安全健康分" |
| 系统诊断 / CPU 内存怎么样 | `GET /api/v1/system/diag` | "跑个诊断" |
| 查看配置 | `GET /api/v1/config/view` | "看下配置" |
| 开启严格模式 | `POST /api/v1/system/strict-mode` `{"enabled":true}` | "开启严格模式" |
| 关闭严格模式 | `POST /api/v1/system/strict-mode` `{"enabled":false}` | "关闭严格模式" |
| 有什么通知 / 告警 | `GET /api/v1/notifications` | "查看通知" |
| 创建备份 | `POST /api/v1/backup` | "备份一下" |
| 备份列表 | `GET /api/v1/backups` | "有哪些备份" |
| 恢复备份 | `POST /api/v1/backups/:name/restore` | "恢复到 xxx 备份" |
| 大屏数据 | `GET /api/v1/bigscreen/data` | "态势大屏" |
| 布局管理 | `GET/POST /api/v1/layouts` | "切换布局" |
| 跑模拟测试 | `POST /api/v1/simulate/traffic` | "端到端模拟" |
| 注入演示数据 | `POST /api/v1/demo/seed` | "注入 demo 数据" |
| 清除演示数据 | `DELETE /api/v1/demo/clear` | "清掉演示数据" |
| 登录 | `POST /api/v1/auth/login` | "登录龙虾卫士" |
| 用户管理 | `GET/POST /api/v1/auth/users` | "创建用户" |
| 操作审计 | `GET /api/v1/op-audit` | "查看操作日志" |
| 综合概览 | `GET /api/v1/overview/summary` | "首页概览" |

---

## 使用示例

```bash
# 健康检查
curl -s http://10.44.96.142:9090/healthz | jq .

# 安全健康分
curl -s -H "Authorization: Bearer $TOKEN" http://10.44.96.142:9090/api/v1/health/score | jq .

# 开启严格模式
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled":true}' http://10.44.96.142:9090/api/v1/system/strict-mode | jq .

# 创建备份
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://10.44.96.142:9090/api/v1/backup | jq .

# 端到端模拟
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://10.44.96.142:9090/api/v1/simulate/traffic | jq .
```

---

## CLI 工具

完整命令行工具见 `../lobster-cli.sh`，支持所有运维命令的快捷调用。
