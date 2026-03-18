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
