# 🦞 龙虾卫士 · 污染追踪域 — v36.6

数据流污染标签传播追踪与自动逆转引擎。覆盖污染标记注入/查询/扫描、泄露检测、逆转记录、自定义污点规则 CRUD、以及污染追踪与逆转引擎配置。
所有 API 调用需带 `Authorization: Bearer <token>` 头，基础 URL 为 `http://10.44.96.142:9090`。

---

## API 参考

### 污染统计与查询

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/taint/stats` | 统计概览（活跃数/累计/拦截数/最近泄露/标签分布） | — |
| GET | `/api/v1/taint/active` | 活跃污染标记列表（含传播链） | — |
| GET | `/api/v1/taint/trace/:id` | 查询指定 TraceID 的污染链 | :id = trace_id |

### 污染标记操作

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| POST | `/api/v1/taint/inject` | 手动注入污染标记 | body: `{labels, source, detail}` |
| DELETE | `/api/v1/taint/entry/:id` | 删除指定污染标记 | :id = trace_id |
| POST | `/api/v1/taint/cleanup` | 清理所有已过期标记 | — |
| POST | `/api/v1/taint/scan` | PII 扫描（检测文本是否含敏感信息） | body: `{text}` |

### 污染追踪配置

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/taint/config` | 获取污染追踪配置 | — |
| PUT | `/api/v1/taint/config` | 更新配置 | body: `{action, ttl_minutes}` |

### 自定义污点规则 CRUD（v35.0）

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/taint/rules` | 列出内置规则 + 自定义规则 | — |
| POST | `/api/v1/taint/rules` | 新增自定义污点规则 | body: `{name, pattern, label, enabled, desc_text}` |
| PUT | `/api/v1/taint/rules/:id` | 更新自定义规则 | :id = 规则 ID |
| DELETE | `/api/v1/taint/rules/:id` | 删除自定义规则 | :id = 规则 ID |

**自定义规则字段说明**：
- `name`：规则名称（唯一标识，如 `openai-key`）
- `pattern`：Go RE2 正则表达式（如 `(?i)(sk-[a-zA-Z0-9]{32,})`）
- `label`：污染标签，可选 `CREDENTIAL-TAINTED` / `PII-TAINTED` / `CONFIDENTIAL` / `INTERNAL-ONLY`
- `enabled`：是否启用（bool）
- `desc_text`：规则描述（可选）

**常见正则参考**：
```
# 奇安信内网 API URL
https?://[^/]*qianxin-inc\.cn[^\s"']*  → CONFIDENTIAL

# K8s 集群内部服务地址
https?://[^/]+\.svc\.cluster\.local[^\s"']*  → INTERNAL-ONLY

# 40 位 hex token（API Key）
\b[0-9a-f]{40}\b  → CREDENTIAL-TAINTED

# OpenAI API Key
(?i)(sk-[a-zA-Z0-9]{32,})  → CREDENTIAL-TAINTED
```

### 逆转引擎

| 方法 | 端点 | 说明 | 关键参数 |
|------|------|------|----------|
| GET | `/api/v1/reversal/config` | 获取逆转引擎配置 | — |
| PUT | `/api/v1/reversal/config` | 更新逆转配置 | body: `{request_mode, response_mode}` |
| GET | `/api/v1/reversal/records` | 逆转操作记录 | — |
| GET | `/api/v1/reversal/stats` | 逆转统计 | — |
| GET | `/api/v1/reversal/templates` | 逆转模板列表 | — |
| POST | `/api/v1/reversal/templates` | 添加逆转模板 | body: 模板内容 |

**逆转模式说明**：
- `request_mode`: `none`（不注入）/ `pre-inject`（LLM 请求前注入"数据不可信"提示）
- `response_mode`: `none`（不处理）/ `soft`（追加警告提示）/ `hard`（替换被污染响应）/ `stealth`（不可见水印标记）

### 标签说明

| 标签 | 含义 | 典型场景 |
|------|------|----------|
| `PII-TAINTED` | 个人身份信息 | 姓名、身份证、手机号 |
| `CREDENTIAL-TAINTED` | 凭据类敏感数据 | API Key、密码、Token |
| `CONFIDENTIAL` | 机密信息 | 内部 URL、商业机密 |
| `INTERNAL-ONLY` | 内部专用 | K8s 内网地址、内部系统路径 |

---

## 意图映射

| 用户说 | 调用 |
|--------|------|
| 污染统计 / taint 概览 | `GET /api/v1/taint/stats` |
| 活跃污染标记 / 当前有哪些污染 | `GET /api/v1/taint/active` |
| 查污染链 / 追踪某个 trace | `GET /api/v1/taint/trace/:id` |
| 扫描文本 / 检测 PII / taint scan | `POST /api/v1/taint/scan` |
| 注入污染标记 / 手动打污点 | `POST /api/v1/taint/inject` |
| 删除污染标记 | `DELETE /api/v1/taint/entry/:id` |
| 清理过期标记 | `POST /api/v1/taint/cleanup` |
| 污染追踪配置 / 检测动作 / TTL | `GET /api/v1/taint/config` |
| 更新污染配置 | `PUT /api/v1/taint/config` |
| 自定义污点规则 / 规则列表 | `GET /api/v1/taint/rules` |
| 新增自定义规则 / 添加污点规则 | `POST /api/v1/taint/rules` |
| 更新自定义规则 | `PUT /api/v1/taint/rules/:id` |
| 删除自定义规则 | `DELETE /api/v1/taint/rules/:id` |
| 逆转配置 / 逆转引擎模式 | `GET /api/v1/reversal/config` |
| 更新逆转配置 | `PUT /api/v1/reversal/config` |
| 逆转记录 / 有哪些数据被逆转了 | `GET /api/v1/reversal/records` |
| 逆转统计 | `GET /api/v1/reversal/stats` |
| 逆转模板 | `GET /api/v1/reversal/templates` |
