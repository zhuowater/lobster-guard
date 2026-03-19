# 📋 配置参考

> 返回 [README](../README.md)

详细配置说明参见 `config.yaml.example`，涵盖以下配置分组：

| 分组 | 主要配置项 |
|------|-----------|
| **通道** | `channel`, `mode`, 各平台加密凭据 |
| **代理** | `inbound_listen`(:18443), `outbound_listen`(:18444), `management_listen`(:9090) |
| **LLM 代理** | `llm_proxy.enabled`, `llm_proxy.listen`(:8445), `llm_proxy.targets` |
| **认证** | `auth.enabled`, `auth.jwt_secret`, `auth.token_expiry` |
| **多租户** | `tenant.enabled`, `tenant.default_tenant`, `tenant.isolation` |
| **检测** | `inbound_detect_enabled`, `outbound_audit_enabled`, `detect_timeout_ms` |
| **规则** | `inbound_rules`, `outbound_rules`(6 默认+合并), `llm_proxy.rules`(11 默认+合并) |
| **路由** | `route_default_policy`, `route_policies`, `static_upstreams` |
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

### API 网关 (`api_gateway`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `api_gateway.enabled` | bool | `true` | 启用 API 网关 |
| `api_gateway.jwt_secret` | string | 必填 | JWT 签名密钥 |
| `api_gateway.jwt_expiry` | duration | `24h` | JWT 过期时间 |
| `api_gateway.api_key_enabled` | bool | `true` | 启用 APIKey 认证 |
| `api_gateway.canary_percent` | int | `0` | 灰度路由百分比 |
| `api_gateway.routes` | array | `[]` | 路由定义 |
