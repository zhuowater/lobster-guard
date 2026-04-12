# 📋 配置参考

> 返回 [README](../README.md)

v20.6 起采用**分层配置**：核心 `config.yaml`（~70 行必填项）+ 可选 `conf.d/` 模块目录。

```
config.yaml              ← 核心配置（端口/认证/存储），必须改
conf.d/                   ← 模块配置，按需启用（不存在时静默跳过）
├── channels.yaml         ← 通道加密凭据
├── rules-inbound.yaml    ← 入站检测规则
├── rules-outbound.yaml   ← 出站审计规则
├── llm-proxy.yaml        ← LLM 代理
├── detection.yaml        ← 高级检测引擎
├── routing.yaml          ← 路由策略 + 静态上游
├── api-gateway.yaml      ← API 网关
├── discovery.yaml        ← K8s 服务发现
├── llm-cache.yaml        ← LLM 响应缓存
├── source-classifier.yaml← 来源分类（URL / host / auth provenance）
└── advanced.yaml         ← 运维/WebSocket/备份/限流
```

加载顺序：主配置 → `conf.d/` 按文件名字母序合并。旧版单文件模式向后兼容。

> v36.5 起，关于 slice merge key、API 写回与重启冲突判定的正式规则见 [`docs/config-precedence.md`](./config-precedence.md)。

详细配置分组：

| 分组 | 主要配置项 |
|------|-----------|
| **通道** | `channel`, `mode`, 各平台加密凭据 |
| **代理** | `inbound_listen`(:18443), `outbound_listen`(:18444), `llm_proxy.listen`(:8445), `management_listen`(:9090) |
| **LLM 代理** | `llm_proxy.enabled`, `llm_proxy.listen`(:8445), `llm_proxy.targets` |
| **认证** | `auth.enabled`, `auth.jwt_secret`, `auth.token_expiry` |
| **多租户** | `tenant.enabled`, `tenant.default_tenant`, `tenant.isolation` |
| **检测** | `inbound_detect_enabled`, `outbound_audit_enabled`, `detect_timeout_ms` |
| **规则** | `inbound_rules`, `outbound_rules`(6 默认+合并), `llm_proxy.rules`(11 默认+合并) |
| **上游** | `static_upstreams`, `openclaw_upstream` |
| **K8s 发现** | `discovery.kubernetes.*` (enabled/kubeconfig/namespace/service/sync_interval) |
| **路由** | `route_default_policy`, `route_policies` |
| **Red Team** | `redteam.enabled`, `redteam.vectors`, `redteam.schedule` |
| **蜜罐** | `honeypot.enabled`, `honeypot.templates`, `honeypot.watermark` |
| **A/B 测试** | `ab_testing.enabled`, `ab_testing.max_concurrent` |
| **行为画像** | `behavior_profile.enabled`, `behavior_profile.features` |
| **攻击链** | `attack_chain.enabled`, `attack_chain.stages`, `attack_chain.auto_escalate` |
| **异常检测** | `anomaly.enabled`, `anomaly.baseline_days`, `anomaly.threshold` |
| **大屏** | `bigscreen.enabled`, `bigscreen.refresh_interval` |
| **自定义布局** | `custom_layout.enabled`, `custom_layout.presets` |
| **端到端模拟** | `simulate.enabled`, `simulate.auto_schedule` |
| **限流** | `rate_limit.global_rps`, `rate_limit.per_sender_rps` |
| **来源分类** | `source_classifier.rules[]`, `tenant_configs.source_classifier_yaml` |
| **可观测性** | `metrics_enabled`, `log_format`, `log_level` |
| **高可用** | `shutdown_timeout`, `backup_dir`, `backup_auto_interval` |

## 最小配置示例（单机模式）

```yaml
# 蓝信加密凭据
callbackKey: "你的回调加密密钥"
callbackSignToken: "你的签名令牌"

# IM 代理监听（4 端口架构）
inbound_listen: ":18443"
outbound_listen: ":18444"
openclaw_upstream: "http://127.0.0.1:18790"
lanxin_upstream: "https://apigw.lx.qianxin.com"

# LLM 代理（可选）
llm_proxy:
  enabled: true
  listen: ":8445"

# 管理
management_listen: ":9090"
management_token: "your-secret-management-token"

# JWT 认证
auth:
  enabled: true
  jwt_secret: "your-jwt-secret-at-least-32-chars"

# 安全检测
inbound_detect_enabled: true
outbound_audit_enabled: true
detect_timeout_ms: 50
db_path: "./audit.db"
```

### K8s 服务发现 (`discovery`)

> 详见 [K8s 服务发现](k8s-discovery.md)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `discovery.kubernetes.enabled` | bool | `false` | 启用 K8s 服务发现 |
| `discovery.kubernetes.kubeconfig` | string | `""` | kubeconfig 路径（空 = InCluster） |
| `discovery.kubernetes.namespace` | string | `"default"` | 目标 namespace |
| `discovery.kubernetes.service` | string | `""` | Service 名称 |
| `discovery.kubernetes.port_name` | string | `"gateway"` | Service 端口名 |
| `discovery.kubernetes.label_selector` | string | `""` | Pod 标签过滤 |
| `discovery.kubernetes.sync_interval` | int | `15` | 轮询间隔（秒） |

### 上游管理 (`static_upstreams`)

> 详见 [上游管理](upstream-management.md)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `static_upstreams[].id` | string | 必填 | 上游唯一标识 |
| `static_upstreams[].address` | string | 必填 | 上游地址 (IP 或域名) |
| `static_upstreams[].port` | int | 必填 | 上游端口 |
| `static_upstreams[].tags` | map | `{}` | 自定义标签 (key-value) |

```yaml
static_upstreams:
  - id: "openclaw-prod-1"
    address: "10.0.1.10"
    port: 18789
    tags:
      region: "cn-north"
      env: "production"
```

## 来源分类 (`source_classifier`)

> 详见 [来源分类说明](source-classifier.md)

`source_classifier` 用于把 tool call 中的 URL / API endpoint 从“普通参数”提升为一等来源对象。
系统会结合 `tool_name + tool_args` 提取 `url / host / path / method / auth`，产出统一 `SourceDescriptor`，再接到：

- IFC label
- Capability provenance
- PathPolicy 风险信号
- explain / 审计信息

### 全局配置结构

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
      suspicious: false
      tags:
        - corp_override
        - control_plane
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source_classifier.rules[].name` | string | 是 | 规则名，用于 explain 与审计展示 |
| `tool_pattern` | regex string | 否 | 匹配工具名，例如 `^web_fetch$` |
| `host_pattern` | regex string | 否 | 匹配 host |
| `path_pattern` | regex string | 否 | 匹配 URL path |
| `method_pattern` | regex string | 否 | 匹配 HTTP method |
| `auth_type_pattern` | regex string | 否 | 匹配认证类型：`none` / `bearer` / `api_key` / `basic` / `authorization` |
| `category` | string | 是 | 来源分类，如 `public_web` / `external_api` / `internal_api` / 自定义类别 |
| `confidentiality` | int | 是 | IFC 机密等级（0-3） |
| `integrity` | int | 是 | IFC 完整性等级（0-3） |
| `trust_score` | float | 是 | 信任分（0-1） |
| `private_network` | bool | 否 | 强制标注是否私网 |
| `suspicious` | bool | 否 | 强制标注可疑来源 |
| `tags` | string[] | 否 | 额外标签，进入 descriptor 与 explain 结果 |

### 默认启发式分类

当没有命中显式规则时，系统会按启发式分类：

- `metadata_service`：如 `169.254.169.254` / metadata path
- `public_web`：公开网页且无认证
- `internal_api`：私网 / localhost / RFC1918 地址
- `external_api`：API host、带认证请求或 `/api/` 路径
- `unknown_url`：提取出 URL 但无法进一步分类

### 租户级 override

租户可以在 Dashboard → **租户管理** → **安全策略** 中设置 `source_classifier_yaml`，覆盖全局规则。

覆盖优先级：

1. tenant `source_classifier_yaml`
2. 全局 `source_classifier.rules`
3. 启发式分类 fallback

`source_classifier_yaml` 示例：

```yaml
rules:
  - name: tenant-docs-override
    host_pattern: '^docs\\.python\\.org$'
    category: tenant_docs
    confidentiality: 2
    integrity: 2
    trust_score: 0.77
```

## 出站规则配置（v18 智能合并）

> v18 起，出站规则采用智能合并机制：6 条默认规则（PII 身份证/手机/银行卡 + 凭据密码/APIKey + 恶意命令）始终加载，用户配置的同名规则覆盖默认规则，新名称规则追加。

```yaml
outbound_rules:
  # 覆盖默认规则
  - name: "pii_id_card"
    pattern: '(?:^|\D)\d{17}[\dXx](?:\D|$)'
    action: "warn"    # 从 block 改为 warn
  # 追加自定义规则
  - name: "internal_ip_leak"
    pattern: '10\.\d{1,3}\.\d{1,3}\.\d{1,3}'
    action: "warn"
```

## Phase 1 新增配置 (v18-v20)

### 执行信封 (`envelope`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `envelope.enabled` | bool | `true` | 启用执行信封 |
| `envelope.hmac_key` | string | 随机生成 | HMAC-SHA256 签名密钥 |
| `envelope.batch_size` | int | `64` | Merkle Tree 批次叶子数 |
| `envelope.chain_enabled` | bool | `true` | 启用哈希链 |
| `envelope.auto_verify` | bool | `false` | 自动定期验证 |

### 事件总线 (`event_bus`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `event_bus.enabled` | bool | `true` | 启用事件总线 |
| `event_bus.buffer_size` | int | `1000` | 事件缓冲区大小 |
| `event_bus.retry_count` | int | `3` | Webhook 投递重试次数 |
| `event_bus.retry_interval` | duration | `5s` | 重试间隔 |
| `event_bus.targets` | array | `[]` | Webhook 目标列表 |
| `event_bus.action_chains` | array | `[]` | ActionChain 定义 |

### 自适应决策 (`adaptive`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `adaptive.enabled` | bool | `true` | 启用自适应决策 |
| `adaptive.fp_target` | float | `0.01` | 目标误伤率 (1%) |
| `adaptive.confidence_level` | float | `0.95` | 置信区间水平 |
| `adaptive.decay_hours` | int | `24` | 反馈衰减时间窗口 |
| `adaptive.min_samples` | int | `100` | 最小样本数 |

### 奇点蜜罐 (`singularity`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `singularity.enabled` | bool | `true` | 启用奇点蜜罐 |
| `singularity.budget_euler_chi` | float | `2.0` | 欧拉特征χ拓扑预算 |
| `singularity.exposure_level` | string | `medium` | 暴露等级 (low/medium/high) |
| `singularity.channels` | array | `["network","application","data"]` | 三通道 |
| `singularity.auto_recommend` | bool | `true` | 自动推荐放置点 |

### 对抗性自进化 (`evolution`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `evolution.enabled` | bool | `true` | 启用自进化引擎 |
| `evolution.interval` | duration | `6h` | 自动进化间隔 |
| `evolution.strategies` | array | 6策略 | 变异策略列表 |
| `evolution.max_rules_per_gen` | int | `10` | 每代最大生成规则数 |
| `evolution.fitness_threshold` | float | `0.7` | 适应度阈值 |
| `evolution.auto_apply` | bool | `false` | 自动应用生成规则 |

### 语义检测 (`semantic`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `semantic.enabled` | bool | `true` | 启用语义检测 |
| `semantic.tfidf_threshold` | float | `0.6` | TF-IDF 相似度阈值 |
| `semantic.syntax_weight` | float | `0.25` | 句法权重 |
| `semantic.anomaly_weight` | float | `0.25` | 异常权重 |
| `semantic.intent_weight` | float | `0.25` | 意图权重 |
| `semantic.tfidf_weight` | float | `0.25` | TF-IDF 权重 |
| `semantic.action` | string | `warn` | 触发动作 (block/warn/log) |

### 蜜罐深度交互 (`honeypot_deep`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `honeypot_deep.enabled` | bool | `true` | 启用深度交互 |
| `honeypot_deep.loyalty_decay` | float | `0.95` | 忠诚度衰减系数 |
| `honeypot_deep.feedback_to_evolution` | bool | `true` | 回馈自进化引擎 |
| `honeypot_deep.max_sessions` | int | `100` | 最大并发交互会话 |

### 工具策略 (`tool_policy`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `tool_policy.enabled` | bool | `true` | 启用工具策略引擎 |
| `tool_policy.rules` | array | 18条默认 | 工具调用规则 |
| `tool_policy.window_size` | duration | `60s` | 滑窗限流时间窗口 |
| `tool_policy.window_max` | int | `30` | 滑窗最大调用次数 |
| `tool_policy.action` | string | `warn` | 默认触发动作 |

### 污染追踪 (`taint`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `taint.enabled` | bool | `true` | 启用污染追踪 |
| `taint.pii_types` | array | 12类型 | 启用的 PII 检测类型 |
| `taint.propagation` | array | `["inbound","outbound","llm"]` | 三端传播跟踪 |
| `taint.lineage_block` | bool | `true` | 血缘链阻断 |
| `taint.auto_scan` | bool | `true` | 自动实时扫描 |

### 污染逆转 (`reversal`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `reversal.enabled` | bool | `true` | 启用污染逆转 |
| `reversal.mode` | string | `soft` | 逆转模式 (soft/hard/stealth) |
| `reversal.templates` | array | 12模板 | 逆转模板 |
| `reversal.auto_reverse` | bool | `false` | 自动逆转 |

### LLM 缓存 (`llm_cache`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `llm_cache.enabled` | bool | `true` | 启用 LLM 语义缓存 |
| `llm_cache.similarity_threshold` | float | `0.85` | TF-IDF 语义匹配阈值 |
| `llm_cache.ttl` | duration | `24h` | 缓存 TTL |
| `llm_cache.max_entries` | int | `10000` | 最大缓存条目数 |
| `llm_cache.tenant_isolation` | bool | `true` | 租户隔离 |

### LLM Proxy strip_prefix 路由 (v20.8.1+)

> 当 LLM Proxy 需要将不同路径前缀路由到不同目标时，`strip_prefix` 可在转发前去掉 `path_prefix`，使上游收到的是标准 API 路径。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `llm_proxy.targets[].strip_prefix` | bool | `false` | 转发前去掉 path_prefix |

**示例**：将 `/qax/v1/chat/completions` 路由到上游的 `/v1/chat/completions`：

```yaml
llm_proxy:
  targets:
    - name: "qax-llm"
      upstream: "https://aip.b.qianxin-inc.cn"
      path_prefix: "/qax/v1/"
      strip_prefix: true          # /qax/v1/... → /v1/...
      api_key_header: "Authorization"
    - name: "deepseek"
      upstream: "https://api.deepseek.com"
      path_prefix: "/v1/"
      strip_prefix: false          # /v1/... → /v1/...（保持不变）
      api_key_header: "Authorization"
```

**使用场景**：将 OpenClaw 的 LLM 流量透明引入龙虾卫士审计，无需修改上游 API 路径。

### Taint Reversal 自动注入 (v20.8.1+)

> v20.8.1 起，污染逆转不再仅限手动 API 触发。LLM Proxy 在响应路径自动调用 `reversalEngine.Reverse()`，覆盖非流式和 SSE 流式两种场景。

**工作原理**：

1. **IM↔LLM trace 关联** — LLM Proxy 优先使用 `SessionCorrelator` 关联的 IM `trace_id`（而非 LLM 生成的 trace_id），确保入站标记的 taint 在 LLM 响应路径中被正确关联
2. **非流式逆转** — 响应返回前检查关联 trace 的 taint labels，命中时根据 `reversal.mode`（soft/hard/stealth）追加或替换响应内容
3. **SSE 流式逆转** — 流结束后追加自定义 SSE 事件 `event: lobster_guard_taint_reversal`，客户端通过 event type 区分正常输出和安全缓解提示

**相关配置**（在 `conf.d/detection.yaml` 中）：

```yaml
taint_tracker:
  enabled: true
  action: warn
  ttl_minutes: 30

taint_reversal:
  enabled: true
  mode: soft           # soft: 追加提示 | hard: 替换响应 | stealth: 静默记录
```

> 注意：`taint_reversal.auto_reverse` 配置项在 v20.8.1 中已隐式生效——只要 `taint_reversal.enabled: true` 且 LLM Proxy 开启，逆转即自动执行。

### API 网关 (`api_gateway`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `api_gateway.enabled` | bool | `true` | 启用 API 网关 |
| `api_gateway.jwt_secret` | string | 必填 | JWT 签名密钥 |
| `api_gateway.jwt_expiry` | duration | `24h` | JWT 过期时间 |
| `api_gateway.api_key_enabled` | bool | `true` | 启用 APIKey 认证 |
| `api_gateway.canary_percent` | int | `0` | 灰度路由百分比 |
| `api_gateway.routes` | array | `[]` | 路由定义 |
