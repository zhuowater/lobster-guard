# 🦞 龙虾卫士 · 安全画像 — v33.0

Per-upstream 实例安全画像，5 维评分体系（入站防护/LLM安全/数据防泄漏/行为合规/工具管控），16 引擎 × 14 张 DB 表聚合。

## 通用信息

| 项目 | 值 |
|---|---|
| 基础 URL | `http://10.44.96.142:9090` |
| 认证 | `Authorization: Bearer <token>` |
| 响应格式 | JSON |

---

## API 参考

### 安全画像

| 方法 | 端点 | 说明 |
|---|---|---|
| GET | `/api/v1/upstreams/{id}/security-profile` | 单个上游安全画像 |
| GET | `/api/v1/upstream-profiles` | 全部上游安全画像列表 |

### 单个画像响应结构

```json
{
  "upstream_id": "openclaw-local",
  "updated_at": "2026-03-30T22:00:00Z",
  "security_score": 76.3,
  "risk_level": "medium",
  "user_count": 2,
  "dimensions": [
    { "name": "入站防护", "score": 20, "max": 20, "icon": "🛡️",
      "details": { "total_requests": 1234, "blocked": 5, "warned": 12 } },
    { "name": "LLM安全", "score": 3.5, "max": 20, "icon": "🤖",
      "details": { "llm_rule_hits": 13, "expose_events": 9 } },
    { "name": "数据防泄漏", "score": 20, "max": 20, "icon": "🔒",
      "details": { "ifc_violations": 0, "taint_entries": 0, "taint_reversals": 3, "ifc_hidden": 2 } },
    { "name": "行为合规", "score": 14, "max": 20, "icon": "📊",
      "details": { "behavior_anomalies": 249, "plan_deviations": 4, "plan_executions": 72 } },
    { "name": "工具管控", "score": 18.8, "max": 20, "icon": "🔧",
      "details": { "envelope_failures": 4, "envelope_total": 129, "cap_denials": 0 } }
  ],
  "traffic": { "total_24h": 1234, "blocked_24h": 5, "warn_24h": 12 },
  "engine_alerts": { ... },
  "top_risk_events": [ ... ],
  "trend": [ { "date": "2026-03-30", "score": 76.3 }, ... ]
}
```

### 列表响应（含分段统计）

```json
{
  "profiles": [ ... ],
  "total": 2,
  "total_users": 3,
  "avg_score": 76.3,
  "segments": {
    "gt80": 0,
    "61_80": 1,
    "41_60": 0,
    "20_40": 0,
    "lt20": 0
  }
}
```

## 评分体系

### 5 维度 × 20 分

| 维度 | 满分 | 评分依据 |
|------|------|---------|
| 入站防护 | 20 | 入站拦截数、审计日志异常 |
| LLM 安全 | 20 | LLM 规则命中数(COUNT)、暴露事件数、攻击链数 |
| 数据防泄漏 | 20 | IFC 违规、污染条目；**正面加分**: 污染逆转 + IFC 隐藏(+0.5/个, 上限+3) |
| 行为合规 | 20 | 行为异常数、计划偏离**率**(deviations/executions) |
| 工具管控 | 20 | 信封失败**率**(failures/total)、Capability 拒绝**率** |

### 比率 vs 绝对值

- **比率评分**: 偏离率、拒绝率、失败率 — 更有意义（10/1000 好于 2/5）
- **正面信号**: taint_reversals（污染逆转成功）和 ifc_hidden_content（信息流隐藏）是防御成功证据，加分而非减分

### 16 引擎告警聚合

| 类型 | 引擎 | 信号性质 |
|------|------|---------|
| 负面 | 入站规则、LLM规则、IFC违规、Capability拒绝、偏差检测、攻击链、行为异常、信封失败、反事实、蜜罐、奇点 | 🔴 风险 |
| 正面 | 污染逆转、IFC隐藏、规则进化 | 🟢 防御成功 |

## Dashboard 页面

路径: `#/behavior`（威胁中心 → 安全画像）

### 三级穿透

| 层级 | 内容 |
|------|------|
| L0 概览 | Canvas 粒子背景 + Treemap(面积=用户数,颜色=评分) + 甜甜圈(实例/用户总数) + 5档分段统计 + 5维度环 |
| L1 排名 | 表格（环形评分 + 5 维进度条 + 等级标签 + 用户数 + 告警数）；支持过滤/排序联动 |
| L2 详情 | 展开行内：评分环 + 雷达图 + 维度卡片 + 16 引擎告警网格 + Top 风险事件 + 7 天趋势 |

### Treemap

- **面积** = 上游绑定用户数（从 `user_routes` 表按 `upstream_id` 聚合）
- **颜色** = 安全评分等级（绿 >80 / 靛 61-80 / 黄 41-60 / 橙 20-40 / 红 <20）
- **点击** = 展开该实例详情

## 使用示例

```bash
# 获取单个上游安全画像
curl -s -H "Authorization: Bearer $TOKEN" \
  "$URL/api/v1/upstreams/openclaw-local/security-profile" | jq '.security_score, .risk_level'

# 获取全部上游安全画像 + 分段统计
curl -s -H "Authorization: Bearer $TOKEN" \
  "$URL/api/v1/upstream-profiles" | jq '.avg_score, .segments'
```
