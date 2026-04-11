# URL / API Source Classification Design

> **For Hermes:** 这份文档聚焦一个具体设计点：在 `llm_proxy` 的 tool-call 治理阶段，基于 tool args 中的 URL / API endpoint / 认证方式 / 域名特征，对数据来源进行更细粒度的机密等级和完整性分级。目标是增强 IFC / Capability / Path risk / Explainability，而不是立刻实现全部统一治理。

**Goal:** 让 lobster-guard 不再只按“工具类型”粗粒度地给数据打标签（例如 `tool:web_fetch = PUBLIC/TAINT`），而是能按实际访问的 URL / API endpoint / host / path / auth 语义，对外部数据进行更细粒度来源分类，进而映射到 `Confidentiality` / `Integrity` / `TrustScore` / `SourceCategory`。

**Architecture:** 在 `llm_proxy` 的 tool-call 治理阶段加入一个新的 `ToolSourceClassifier`。它从 tool name + tool args 中提取 URL/host/path/auth 特征，产出 `SourceDescriptor`。`SourceDescriptor` 再映射到：
- IFC source label
- Capability provenance source
- Path risk signal
- 审计/解释信息

**Tech Stack:** Go, `net/url`, JSON parsing, IFC labels, Capability provenance, LLMProxy tool governance pipeline.

---

## 1. Problem Statement

当前 lobster-guard 已经有初步的数据分级，但粒度偏粗。

### 当前 IFC 默认规则（现状）
见：`src/ifc_engine.go`

```go
{Source: "tool:web_fetch", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegTaint}},
{Source: "tool:database_query", Label: IFCLabel{Confidentiality: ConfConfidential, Integrity: IntegLow}},
{Source: "tool:mcp_tool", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegLow}},
{Source: "tool:file_read", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium}},
```

问题在于：

- `web_fetch(https://docs.python.org/...)`
- `web_fetch(https://crm.internal.company/customers)`
- `http_request(https://api.stripe.com/v1/customers)`
- `http_request(http://169.254.169.254/latest/meta-data)`

在当前模型里都容易被归为类似的工具源，无法区分它们在实际安全语义上的巨大差异。

这会导致：

1. **Confidentiality 分级过粗**
2. **Capability lineage 解释力不足**
3. **Path risk 无法利用 endpoint 语义**
4. **IFC 不能根据 API 类型做更合理的 P-F / P-T 检查**
5. **未来 MCP / Cross-Agent 接入时缺少统一“来源分级语言”**

---

## 2. Design Goal

这个设计不是做一个“URL 黑名单”。

真正目标是：

> **把 URL / API endpoint 从普通文本参数，提升为一等安全来源对象。**

也就是：

- 相同 tool，不同 URL → 不同 source category
- 相同 host，不同 path → 不同 confidentiality
- 有认证 / 无认证 → 不同 integrity / trust
- metadata/admin/internal endpoint → 自动升高机密等级

最终让系统能表达：

- 这是公开网页数据
- 这是带 token 的外部 SaaS API 数据
- 这是内部 CRM API 返回的客户数据
- 这是 metadata service 或 secrets endpoint 数据

这些应该在 IFC / Capability / 审计里成为不同对象，而不是都统称“tool:web_fetch”。

---

## 3. High-Level Architecture

建议新增一个组件：

## `ToolSourceClassifier`

职责：
- 从 tool name + args 中抽取 URL / endpoint 相关特征
- 做 host/path/auth/category 分类
- 输出统一 `SourceDescriptor`

### 调用位置
首选：
- `src/llm_ifc_governance.go`
- `src/llm_deep_governance.go`

更理想的长期位置：
- 一个统一的 governance preview layer

### 流程图

```text
tool_call(name, args)
  -> parse args
  -> extract candidate URLs / endpoint fields
  -> classify host/path/auth/features
  -> SourceDescriptor
  -> map to IFC / Capability / Path signals / Audit
```

---

## 4. Core Data Model

建议新增：

```go
type SourceDescriptor struct {
    SourceKey        string    `json:"source_key"`
    BaseTool         string    `json:"base_tool"`
    URL              string    `json:"url,omitempty"`
    Host             string    `json:"host,omitempty"`
    Path             string    `json:"path,omitempty"`
    Method           string    `json:"method,omitempty"`
    Category         string    `json:"category"` // public_web / external_api / internal_api / admin_api / metadata_service / unknown

    Confidentiality  IFCLevel   `json:"confidentiality"`
    Integrity        IntegLevel `json:"integrity"`
    TrustScore       float64    `json:"trust_score"`

    AuthType         string     `json:"auth_type,omitempty"` // none / bearer / api_key / basic / signed / cookie / unknown
    PrivateNetwork   bool       `json:"private_network"`
    Suspicious       bool       `json:"suspicious"`
    Tags             []string   `json:"tags,omitempty"`
    Evidence         []string   `json:"evidence,omitempty"`
}
```

### 字段解释

#### `SourceKey`
供 IFC / Capability 持久化和 lineage 使用。
例如：
- `tool:web_fetch:public_web`
- `tool:http_request:host=api.stripe.com:path=/v1/customers`
- `tool:http_request:metadata_service`
- `tool:web_fetch:internal_api:crm.internal.company`

#### `Category`
供规则、审计和 explainability 使用。

#### `Confidentiality / Integrity / TrustScore`
供治理引擎直接使用。

---

## 5. URL Extraction Strategy

URL 不一定在固定字段里，所以不能只查一个 key。

## 5.1 支持的常见字段
优先尝试这些字段：

- `url`
- `uri`
- `endpoint`
- `api_url`
- `base_url`
- `resource`
- `link`
- `href`
- `target`
- `webhook`
- `callback_url`

## 5.2 HTTP 请求类字段
如果 args 结构是 HTTP 风格，还要支持：

- `method`
- `headers`
- `query`
- `params`
- `body`

## 5.3 嵌套 JSON
要支持：
- 顶层 map
- 一层嵌套 map
- 数组里对象的 URL

## 5.4 多 URL 情况
如果一次参数里出现多个 URL：

- 先分别分类
- 取最敏感的 confidentiality
- 取最低的 integrity
- tags 合并
- source key 可采用 canonical summary 或主 URL + tags

---

## 6. Classification Dimensions

## 6.1 Host / Domain 分类

建议至少支持：

### A. Public Web
特征：
- 常见公开站点
- docs / wiki / blog / news / search
- 无认证

建议标签：
- `Confidentiality = PUBLIC`
- `Integrity = TAINT`
- `TrustScore ≈ 0.2-0.3`

### B. External API
特征：
- `api.*`
- 第三方 SaaS API
- 有认证但不一定高敏

建议标签：
- `Confidentiality = INTERNAL`
- `Integrity = LOW`
- `TrustScore ≈ 0.4-0.6`

### C. Internal API
特征：
- 私有域名
- 公司内网 host
- `*.internal`, `*.corp`, 私网 IP

建议标签：
- `Confidentiality = CONFIDENTIAL`
- `Integrity = LOW 或 MEDIUM`
- `TrustScore ≈ 0.5-0.7`

### D. Admin / Secrets / Metadata
特征：
- `/admin`, `/keys`, `/secrets`, `/credentials`, `/vault`
- metadata service
- kube / cloud instance metadata

建议标签：
- `Confidentiality = SECRET`
- `Integrity = LOW`（如果来源外部可控）或 `MEDIUM`
- `TrustScore` 依情况，但必须高敏标签

---

## 6.2 Path / Endpoint 分类

同一个 host 下 path 可能语义完全不同。

### 示例
- `/public/search` → PUBLIC
- `/v1/customers` → CONFIDENTIAL
- `/admin/keys` → SECRET
- `/metadata/iam/security-credentials` → SECRET

建议规则支持：
- exact path
- prefix
- regex

---

## 6.3 Auth Classification

认证方式应该影响分级。

### 识别目标
- `Authorization: Bearer ...`
- `X-API-Key`
- `api_key`
- `token`
- `Basic ...`
- cookie auth
- signed request

### 影响建议
- 无 auth：更可能是 public_web / low trust
- 有 bearer/api-key：至少不是 PUBLIC
- 有 admin token / credential path：可直接升高 confidentiality

---

## 6.4 Network Location Classification

### 要识别的高风险位置
- 私网 IP
- loopback
- link-local
- metadata service
- kubernetes service DNS

### 高风险例子
- `169.254.169.254`
- `127.0.0.1`
- `localhost`
- `10.x.x.x`
- `172.16.0.0/12`
- `192.168.x.x`
- `*.svc.cluster.local`

这些至少应该自动打：
- `private_network`
- `internal_api` 或 `metadata_service`
- 更高 confidentiality

---

## 7. Initial Classification Policy

先给一版实用默认策略：

| Category | Example | Conf | Integ | Trust |
|---|---|---:|---:|---:|
| public_web | docs/blog/wiki/search | PUBLIC | TAINT | 0.2-0.3 |
| external_api | stripe/github/slack API | INTERNAL | LOW | 0.4-0.6 |
| internal_api | corp/local/private service | CONFIDENTIAL | LOW/MEDIUM | 0.5-0.7 |
| customer_data_api | CRM / tickets / profiles | CONFIDENTIAL | LOW | 0.5 |
| admin_api | `/admin/*`, `/keys/*` | SECRET | LOW/MEDIUM | 0.4-0.6 |
| metadata_service | cloud metadata / kube secrets | SECRET | LOW | 0.3 |
| local_system_http | localhost / loopback service | CONFIDENTIAL 或 SECRET | LOW | 0.4 |
| unknown_url | parse 成功但无法判别 | INTERNAL | LOW | 0.3 |

---

## 8. Integration Points

## 8.1 IFC Integration

### 当前问题
现在 IFC 用的是：

```go
toolSource := "tool:" + tool
RegisterVariable(..., toolSource, ...)
```

### 建议改法
改成：

```go
src := lp.classifyToolSource(tool, args)
toolSource := src.SourceKey
```

并让 IFC 使用：
- `src.Confidentiality`
- `src.Integrity`
- `src.Tags`

### 两种落法
#### 方案 A：动态 source rule
直接把 `SourceDescriptor` 结果作为这次变量的实际 label

#### 方案 B：SourceKey + rule lookup
按 `SourceKey` 去查 rule，没命中时 fallback 到 descriptor 默认值

建议：
- 短期走 A（实现简单）
- 中期补 B（支持可配置 override）

---

## 8.2 Capability Integration

现在 capability 的 source 比较粗：
- `tool:web_fetch`
- `tool:http_request`

建议改成：
- `tool:web_fetch:public_web`
- `tool:http_request:internal_api`
- `tool:http_request:metadata_service`
- `tool:http_request:admin_api`

这样 lineage 和 `EvaluateWithProvenance()` 会更有解释力。

### 额外建议
把 `TrustScore` 也同步给 capability object。

---

## 8.3 PathPolicy / Risk Integration

URL/API 分类可以作为 path risk 的额外风险信号。

### 示例
- public_web → +5
- external_api with auth → +10
- internal_api → +15
- admin_api / metadata_service → +30

这样 PathPolicy 能更早识别：
- “刚从 metadata service 读了东西，后面又要 send_email”

---

## 8.4 Audit / Explainability Integration

审计里应记录：
- 原始 URL（必要时脱敏）
- host
- category
- conf/integ
- auth_type
- classification evidence

这样最后可以解释：

> 这次 tool_call 被拦，不只是因为 `http_request` 危险，
> 而是因为它访问的是 `metadata_service`，返回数据被标记为 `SECRET/LOW`。

---

# 9. Configuration Model

建议新增配置：

```yaml
source_classification:
  enabled: true
  default_unknown_category: unknown_url
  rules:
    - match:
        tool: http_request
        host: "169.254.169.254"
      category: metadata_service
      conf: SECRET
      integ: LOW
      trust: 0.3

    - match:
        tool: web_fetch
        host_suffix: ".internal.company"
      category: internal_api
      conf: CONFIDENTIAL
      integ: LOW
      trust: 0.6

    - match:
        tool: http_request
        host: "api.stripe.com"
        path_prefix: "/v1/customers"
      category: customer_data_api
      conf: CONFIDENTIAL
      integ: LOW
      trust: 0.5
```

### 配置优先级建议
1. exact host + path
2. host + path prefix
3. host suffix
4. private network / metadata built-in rule
5. auth-based heuristic
6. tool default fallback

---

## 10. Security Heuristics Worth Adding

## 10.1 Auto-escalate to SECRET if path contains
- `secret`
- `token`
- `key`
- `credential`
- `passwd`
- `metadata`
- `iam`
- `vault`

## 10.2 Auto-mark suspicious if URL targets
- loopback
- link-local
- private IP
- cluster-local service

## 10.3 Auto-upgrade from PUBLIC if authenticated
- bearer token
- api key
- signed headers

---

## 11. Suggested Minimal API / Code Shape

建议新建：

### `src/source_classifier.go`

```go
type ToolSourceClassifier struct {
    // config + compiled rules
}

func (c *ToolSourceClassifier) Classify(toolName, toolArgs string) *SourceDescriptor
```

### helper functions
- `extractCandidateURLs(args string) []string`
- `extractHTTPMetadata(args string) HTTPMeta`
- `classifyHost(host string) ...`
- `classifyPath(path string) ...`
- `classifyAuth(args string) ...`
- `isPrivateHost(host string) bool`
- `isMetadataService(host, path string) bool`

---

## 12. Rollout Strategy

## Stage 1 — Audit only
- 先只分类
- 写日志
- 不改 IFC / Capability 决策

目标：看真实流量分布是否合理

## Stage 2 — Feed IFC labels
- 用分类结果替代粗粒度 `tool:web_fetch`
- 但保留 fallback

## Stage 3 — Feed Capability provenance
- source 更细粒度
- trust 更真实

## Stage 4 — Feed Path risk / unified explainability
- 分类结果进入更高层治理

---

## 13. Risks and Pitfalls

### 风险 1：域名不够
必须支持 path 和 auth，不然太粗。

### 风险 2：参数结构不稳定
不同工具 JSON 结构不同，需要兼容多种字段。

### 风险 3：误分类
要支持：
- audit-only shadow mode
- config override
- fallback 到 tool default

### 风险 4：日志泄露 URL
审计时要考虑：
- query string 脱敏
- token/header 脱敏

---

## 14. Final Recommendation

一句话建议：

> **先把 URL/API source classification 做成一个独立组件，以 audit-only + label enrichment 模式上线；先增强 IFC 和 Capability 的来源语义，再考虑更深层的统一治理联动。**

这样收益很高，而且不会一下子把系统复杂度推爆。
