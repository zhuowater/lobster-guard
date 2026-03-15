# Channel Plugin 架构设计

## 一、问题分析

当前 `main.go` 中蓝信强耦合的代码：

| 模块 | 蓝信耦合点 | 需要抽象 |
|------|-----------|---------|
| `LanxinCrypto` | AES-256-CBC 加解密 + SHA1 签名 | → `ChannelCrypto` 接口 |
| `LanxinWebhookBody` | `dataEncrypt/signature/timestamp/nonce` 结构 | → `WebhookParser` 接口 |
| `extractMessageText` | 蓝信 JSON 结构 `data.msgData.text.content` | → `MessageExtractor` 接口 |
| `OutboundProxy` | 蓝信消息 API 路径 `/v1/bot/messages/create` | → `OutboundAuditPaths` 配置 |
| `OutboundProxy.ServeHTTP` | 蓝信出站消息体解析 `msgData.text.content` | → `OutboundMessageExtractor` 接口 |
| `InboundProxy` | 入站解密+消息提取耦合在 ServeHTTP 中 | → `InboundProcessor` 接口 |

## 二、插件接口设计

```go
// ChannelPlugin 是消息通道插件的核心接口
// 每种消息平台（蓝信、飞书、钉钉、企微）实现此接口
type ChannelPlugin interface {
    // 名称标识
    Name() string
    
    // 入站处理：从 webhook 请求中提取明文消息
    // 返回：消息文本、发送者ID、事件类型、是否成功
    ParseInbound(body []byte) (InboundMessage, error)
    
    // 出站处理：从 Agent 出站 API 请求中提取要审计的文本
    // 返回：消息文本（用于安全检测）
    ExtractOutbound(path string, body []byte) (string, bool)
    
    // 判断出站路径是否需要审计
    ShouldAuditOutbound(path string) bool
    
    // 生成拦截响应（入站被拦截时返回的 "假装成功" 响应）
    BlockResponse() (int, []byte)
    
    // 生成出站拦截响应（出站被拦截时的 403 响应）
    OutboundBlockResponse(reason, ruleName string) (int, []byte)
}

// InboundMessage 入站消息解析结果
type InboundMessage struct {
    Text      string  // 消息文本
    SenderID  string  // 发送者ID
    EventType string  // 事件类型
    Raw       []byte  // 解密后的原始 JSON
}
```

## 三、各平台实现要点

### 蓝信 (LanXin)

```yaml
channel: lanxin
lanxin:
  callback_key: "..."
  callback_sign_token: "..."
```

- 入站：AES-256-CBC 解密 → JSON 解析 `data.senderId` + `data.msgData.text.content`
- 出站审计路径：`/v1/bot/messages/create`, `/v1/bot/sendGroupMsg`
- 出站消息提取：`msgData.text.content` 或 `content`
- 拦截响应：`200 {"errcode":0,"errmsg":"ok"}`

### 飞书 (Feishu/Lark)

```yaml
channel: feishu
feishu:
  encrypt_key: "..."
  verification_token: "..."
```

- 入站：AES-256-CBC (encrypt_key) → JSON 解析 `event.message.content`
- 出站审计路径：`/open-apis/im/v1/messages`
- 出站消息提取：`content` JSON 字符串 → `text` 字段
- 拦截响应：`200 {"code":0,"msg":"ok"}`
- 特殊：需处理 URL Verification 回调（`challenge` 字段）

### 钉钉 (DingTalk)

```yaml
channel: dingtalk
dingtalk:
  app_key: "..."
  app_secret: "..."
  aes_key: "..."
  token: "..."
```

- 入站：AES 解密 + 签名校验 → `text.content`
- 出站审计路径：`/robot/send`, `/topapi/message/corpconversation/asyncsend_v2`
- 出站消息提取：`msgtype=text` → `text.content`
- 拦截响应：`200 {"errcode":0,"errmsg":"ok"}`

### 企业微信 (WeCom)

```yaml
channel: wecom
wecom:
  token: "..."
  encoding_aes_key: "..."
  corp_id: "..."
```

- 入站：AES-256-CBC 解密 XML → 提取 `Content` + `FromUserName`
- 出站审计路径：`/cgi-bin/message/send`
- 出站消息提取：`msgtype=text` → `text.content`
- 拦截响应：`200 <xml><return_code>SUCCESS</return_code></xml>`
- 特殊：XML 格式，非 JSON

### 通用 HTTP（无加密）

```yaml
channel: generic
generic:
  sender_header: "X-Sender-Id"
  text_field: "content"
```

- 入站：直接 JSON 解析，从配置的字段路径提取
- 出站：直接 JSON 解析
- 适用于自定义 webhook 集成

## 四、配置结构变更

```yaml
# v3.0 配置结构
channel: lanxin    # 选择通道插件: lanxin | feishu | dingtalk | wecom | generic

# 通道特定配置
lanxin:
  callback_key: "..."
  callback_sign_token: "..."

feishu:
  encrypt_key: "..."
  verification_token: "..."

# 通用配置（不变）
inbound_listen: ":8443"
outbound_listen: ":8444"
management_listen: ":9090"
# ...
```

## 五、向后兼容

- `callbackKey` + `callbackSignToken` 出现在顶层 → 自动识别为 `channel: lanxin`
- 不配 `channel` 字段 → 默认蓝信
- 现有 config.yaml 无需修改即可升级

## 六、实现计划

### Phase 1：抽象接口 + 蓝信插件
- 定义 `ChannelPlugin` 接口
- 将现有蓝信逻辑封装为 `LanxinPlugin`
- InboundProxy / OutboundProxy 改为调用接口
- **零行为变更**，纯重构

### Phase 2：飞书插件
- 实现 `FeishuPlugin`
- 飞书加解密 + URL Verification
- 飞书消息格式解析

### Phase 3：钉钉 + 企微
- 实现 `DingtalkPlugin` + `WecomPlugin`

### Phase 4：通用 HTTP
- 实现 `GenericPlugin`，支持任意自定义 webhook
