# 🌍 Source Classifier（来源分类）

> 返回 [README](../README.md)

Source Classifier 把 tool args 里的 URL / host / path / method / auth，从“普通字符串参数”提升为一等安全来源对象。

它的目标不是简单做 URL 黑名单，而是让 lobster-guard 能回答：

- 这是公开网页数据，还是带认证的外部 API 数据？
- 这是私网内部接口，还是 metadata service？
- 某个租户是否需要把同一个 host 提升为更高信任、更高机密等级？

分类结果会统一输出为 `SourceDescriptor`，并接入：

- IFC label
- Capability provenance
- PathPolicy 风险信号
- Explain / 审计信息

---

## 1. 能力概览

当前实现支持：

- 全局 `source_classifier.rules` 规则管理
- 租户级 `source_classifier_yaml` override
- Dashboard 全局规则编辑器
- Dashboard 租户差异预览（global vs effective）
- Explain API：返回 global/effective descriptor + governance 决策

相关页面：

- Dashboard → **来源分类**
- Dashboard → **租户管理** → **安全策略** → `source_classifier_yaml`

相关 API：

- `GET /api/v1/source-classifier`
- `PUT /api/v1/source-classifier`
- `POST /api/v1/source-classifier/explain`

---

## 2. SourceDescriptor 结构

Source Classifier 的核心输出是 `SourceDescriptor`：

```json
{
  "source_key": "tool:web_fetch:public_web:docs.python.org",
  "base_tool": "web_fetch",
  "url": "https://docs.python.org/3/library/json.html",
  "host": "docs.python.org",
  "path": "/3/library/json.html",
  "method": "GET",
  "category": "public_web",
  "confidentiality": 0,
  "integrity": 0,
  "trust_score": 0.25,
  "auth_type": "none",
  "private_network": false,
  "suspicious": false,
  "tags": ["auth:none"],
  "evidence": ["public_web_host"]
}
```

常见字段含义：

| 字段 | 说明 |
|------|------|
| `source_key` | 统一来源标识，供 IFC / Capability lineage 使用 |
| `base_tool` | 原始工具名 |
| `category` | 来源类别，如 `public_web` / `external_api` / `internal_api` / `metadata_service` |
| `confidentiality` | IFC 机密等级 |
| `integrity` | IFC 完整性等级 |
| `trust_score` | 0-1 的信任分 |
| `auth_type` | `none` / `bearer` / `basic` / `api_key` / `authorization` |
| `private_network` | 是否识别为私网来源 |
| `suspicious` | 是否被标记为高风险来源 |
| `evidence` | 本次分类命中的证据 |

---

## 3. URL 提取与默认分类

### 3.1 URL 提取字段

系统会从 tool args 里递归查找这些字段：

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

同时还会尝试提取：

- `method` / `http_method`
- `headers.Authorization`
- `headers.x-api-key`

### 3.2 默认启发式类别

如果没有命中显式配置规则，则按启发式分类：

| 类别 | 条件 | 默认语义 |
|------|------|----------|
| `metadata_service` | metadata host/path | 高机密、低完整性、可疑 |
| `public_web` | 公开站点 + 无认证 | `ConfPublic` + `IntegTaint` |
| `internal_api` | 私网/localhost/RFC1918 | `ConfConfidential` + `IntegLow` |
| `external_api` | API host / `/api/` / 有认证 | `ConfInternal` + `IntegLow` |
| `unknown_url` | 提取出 URL 但无法进一步归类 | 保守内部低完整性 |

---

## 4. 全局规则配置

全局规则保存在：

- `config.yaml` 的 `source_classifier.rules`
- 或通过 Dashboard / API 写回

示例：

```yaml
source_classifier:
  rules:
    - name: corp-control-plane
      tool_pattern: '^http_request$'
      host_pattern: '^control\\.corp\\.example$'
      path_pattern: '^/v1/admin/'
      method_pattern: '^(GET|POST)$'
      auth_type_pattern: '^(bearer|api_key)$'
      category: internal_control_plane
      confidentiality: 3
      integrity: 3
      trust_score: 0.91
      private_network: true
      tags:
        - corp_override
        - control_plane
```

规则字段：

| 字段 | 说明 |
|------|------|
| `name` | 规则名 |
| `tool_pattern` | 工具名 regex |
| `host_pattern` | host regex |
| `path_pattern` | path regex |
| `method_pattern` | method regex |
| `auth_type_pattern` | 认证方式 regex |
| `category` | 命中后写入的来源类别 |
| `confidentiality` | 命中后写入的 IFC 机密等级 |
| `integrity` | 命中后写入的 IFC 完整性等级 |
| `trust_score` | 命中后的信任分 |
| `private_network` | 可选强制标记 |
| `suspicious` | 可选强制标记 |
| `tags` | 额外标签 |

---

## 5. 租户 override

租户可以通过 `source_classifier_yaml` 覆盖全局规则。

入口：

- Dashboard → **租户管理** → **安全策略**
- 字段：`source_classifier_yaml`

示例：

```yaml
rules:
  - name: tenant-docs-override
    host_pattern: '^docs\\.python\\.org$'
    category: tenant_docs
    confidentiality: 2
    integrity: 2
    trust_score: 0.77
```

优先级：

1. tenant override
2. 全局规则
3. 启发式 fallback

这意味着：

- 同一个 `docs.python.org`，全局可能被识别成 `public_web`
- 某个租户可以把它提升成 `tenant_docs`

Dashboard 里可以直接做 **global vs effective** 差异对比。

---

## 6. Explain API

### 6.1 请求

`POST /api/v1/source-classifier/explain`

```json
{
  "tenant_id": "tenant-a",
  "tool_name": "web_fetch",
  "tool_args": {
    "url": "https://docs.python.org/3/library/json.html"
  },
  "proposed_action": "shell_exec",
  "capability_action": "write"
}
```

### 6.2 响应

```json
{
  "tenant_id": "tenant-a",
  "tool_name": "web_fetch",
  "tenant_override_active": true,
  "global_descriptor": {
    "category": "public_web"
  },
  "effective_descriptor": {
    "category": "tenant_docs"
  },
  "global_rule": {
    "matched": false,
    "scope": "global"
  },
  "effective_rule": {
    "matched": true,
    "name": "tenant-docs-override",
    "scope": "tenant_override"
  },
  "path_decision": {
    "decision": "block"
  },
  "capability_evaluation": {
    "decision": "deny"
  }
}
```

### 6.3 响应字段说明

| 字段 | 说明 |
|------|------|
| `tenant_override_active` | effective 是否相对 global 发生变化 |
| `global_descriptor` | 不带租户 override 的分类结果 |
| `effective_descriptor` | 带租户 override 后的最终分类结果 |
| `global_rule` | 全局规则命中情况 |
| `effective_rule` | 最终命中的规则（可能是 tenant override / global fallback / heuristic） |
| `path_decision` | 结合来源分类后的 PathPolicy 决策 |
| `capability_evaluation` | 结合来源分类后的 Capability 决策 |

---

## 7. Dashboard 使用方式

### 7.1 全局规则编辑

页面：**来源分类**

支持：

- 新增 / 删除全局规则
- 即时查看 JSON 预览
- 查看当前覆盖的 category 列表
- 输入单条 tool call 做 dry-run explain

### 7.2 租户差异预览

页面：**租户管理 → 安全策略**

支持：

- 编辑 `source_classifier_yaml`
- 输入 tool + URL
- 查看 global 与 effective 的分类差异
- 直接确认 override 是否生效

---

## 8. 与其他治理引擎的关系

Source Classifier 不是孤立功能，它是治理链的一层基础语义。

当前已接入：

- **PathPolicy**：把来源类别转换成风险与 taint 信号
- **Capability**：把来源 descriptor 写入 provenance lineage
- **IFC**：根据来源类别分配更合理的 confidentiality / integrity
- **LLM Tool Governance**：在 tool 调用治理时使用来源分类

因此它的价值不只是“多了个配置页”，而是让整条治理链从“按工具名猜来源”升级为“按实际访问对象理解来源”。

---

## 9. 推荐实践

推荐先做两层：

1. **全局规则**：覆盖明显的控制面/API/内部域名
2. **租户 override**：只处理个别租户的特殊信任边界

建议优先配置这几类目标：

- 公司内部控制面 / 管理 API
- 私网 CRM / ERP / 用户数据接口
- metadata service / secret endpoint
- 经常被 agent 访问的外部 SaaS API

如果只是普通公开网页，不一定要写规则，让启发式保持 `public_web` 即可。

---

## 10. 验证清单

上线前建议至少验证：

- 全局规则 GET/PUT 能 round-trip
- 某个租户 override 能覆盖全局结果
- explain API 能返回 global/effective descriptor
- PathPolicy / Capability 决策能随分类变化
- Dashboard 页面能正常保存与 dry-run

相关测试可参考：

- `src/source_classifier_test.go`
- `src/source_classifier_api_test.go`
- `src/source_classifier_config_test.go`
- `src/llm_tool_governance_test.go`
- `src/llm_ifc_governance_test.go`
- `src/capability_test.go`
