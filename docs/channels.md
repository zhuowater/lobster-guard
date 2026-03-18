# 🔌 多通道配置

> 返回 [README](../README.md)

## 通道支持（v3.0）

通过 Channel Plugin 机制，一行配置切换消息平台：

```yaml
channel: "feishu"    # lanxin | feishu | dingtalk | wecom | generic
```

| 通道 | 入站加密 | 消息格式 | 出站审计路径 | 状态 |
|------|---------|---------|------------|------|
| 🔵 **蓝信** (LanXin) | AES-256-CBC + SHA1 签名 | JSON | `/v1/bot/messages/create` | ✅ 生产可用 |
| 🟢 **飞书** (Feishu/Lark) | AES-256-CBC + SHA256 签名 | JSON + URL Verification | `/open-apis/im/v1/messages` | ✅ 已实现 |
| 🔷 **钉钉** (DingTalk) | AES-256-CBC + HMAC-SHA256 | JSON | `/robot/send` | ✅ 已实现 |
| 🟠 **企业微信** (WeCom) | AES-256-CBC + SHA1 签名 | **XML** 入站 / JSON 出站 | `/cgi-bin/message/send` | ✅ 已实现 |
| ⚪ **通用 HTTP** (Generic) | 无加密 | JSON（字段可配置） | 所有 POST | ✅ 已实现 |

## 配置示例

### 🔵 蓝信（默认，向后兼容）

```yaml
# channel: "lanxin"    # 可省略，默认就是蓝信
callbackKey: "YOUR_CALLBACK_KEY_BASE64"
callbackSignToken: "YOUR_SIGN_TOKEN"
lanxin_upstream: "https://apigw.lx.qianxin.com"
```

### 🟢 飞书

```yaml
channel: "feishu"
feishu_encrypt_key: "YOUR_ENCRYPT_KEY"
feishu_verification_token: "YOUR_VERIFICATION_TOKEN"
lanxin_upstream: "https://open.feishu.cn"
```

飞书 URL Verification 自动处理：收到 `{"type":"url_verification","challenge":"xxx"}` 时自动返回 challenge。

### 🔷 钉钉

```yaml
channel: "dingtalk"
dingtalk_token: "YOUR_TOKEN"
dingtalk_aes_key: "YOUR_AES_KEY_43CHARS"      # 43 字符 base64
dingtalk_corp_id: "YOUR_CORP_ID"
lanxin_upstream: "https://oapi.dingtalk.com"
```

### 🟠 企业微信

```yaml
channel: "wecom"
wecom_token: "YOUR_TOKEN"
wecom_encoding_aes_key: "YOUR_ENCODING_AES_KEY_43CHARS"
wecom_corp_id: "YOUR_CORP_ID"
lanxin_upstream: "https://qyapi.weixin.qq.com"
```

注意：企微入站是 XML 格式，lobster-guard 自动处理 XML↔JSON 转换。

### ⚪ 通用 HTTP（自定义 webhook）

```yaml
channel: "generic"
generic_sender_header: "X-Sender-Id"
generic_text_field: "content"
lanxin_upstream: "https://your-api.example.com"
```

适用于自建消息系统或其他未内置支持的平台。

## Bridge Mode（v3.1 长连接桥接）

飞书和钉钉支持 WebSocket 长连接模式（无需公网 IP）。

```yaml
channel: "feishu"
mode: "bridge"       # 加这一行，从 Webhook 切到长连接
```

```
Webhook 模式:   平台 ──POST──► :18443 ──► 安检 ──► Agent
Bridge 模式:    lobster-guard ══WSS══► 平台（拉消息）──► 安检 ──► POST Agent
                                       对 Agent 来说完全一样 ↑
```

### 通道支持矩阵

| 通道 | Webhook | Bridge | Bridge 特性 |
|------|---------|--------|------------|
| 🔵 蓝信 | ✅ | ❌ | 蓝信仅支持 Webhook |
| 🟢 飞书 | ✅ | ✅ | WSS + token 自动刷新（2h）+ 事件确认 |
| 🔷 钉钉 | ✅ | ✅ | Stream + ticket 自动获取 + ping/pong |
| 🟠 企微 | ✅ | ❌ | 企微仅支持 Webhook |
| ⚪ 通用 | ✅ | ❌ | — |

### Bridge 特性

- 🔄 **自动重连** — 断线指数退避（1s → 2s → 4s → ... → 60s max）
- 🔑 **Token 自动刷新** — 飞书 token 每 100 分钟自动刷新
- 💓 **心跳保活** — 自动处理 ping/pong
- 📊 **状态监控** — `/healthz` 展示连接状态、重连次数、消息计数
- 🔀 **混合模式** — Bridge 模式下 `:18443` 仍然监听，可同时接收 Webhook
