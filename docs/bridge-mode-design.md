# Bridge Mode 设计 — 长连接桥接架构

## 一、问题

飞书和钉钉推荐长连接模式（WebSocket），数据流方向反转，传统反向代理架构拦不到入站消息。

## 二、解决方案：Bridge Mode（桥接模式）

lobster-guard 主动建立长连接拉取消息 → 安检 → 转为 Webhook POST 推给 Agent。
对 Agent 完全透明，收到的请求格式与 Webhook 模式一致。

```
┌─────────────────────────────────────────────────────────┐
│                   lobster-guard 🦞                       │
│                                                         │
│  ┌─────────────────────┐                                │
│  │   BridgeConnector    │◄══WSS══ 飞书/钉钉平台         │
│  │   (长连接客户端)      │         (主动拉取消息)         │
│  └──────────┬──────────┘                                │
│             │ InboundMessage                            │
│             ▼                                           │
│  ┌──────────────────────┐    ┌──────────────────┐       │
│  │   InboundProcessor    │───►│ 入站规则引擎      │       │
│  │   (统一安检)           │    │ AC自动机+正则     │       │
│  └──────────┬───────────┘    └──────────────────┘       │
│             │                                           │
│             ▼                                           │
│  ┌──────────────────────┐                               │
│  │   WebhookForwarder    │────► Agent (HTTP POST)       │
│  │   (转为Webhook推送)    │     格式与原生Webhook一致     │
│  └──────────────────────┘                               │
│                                                         │
│  出站方向不变：                                          │
│  Agent ──► :8444 ──► 安检 ──► 平台 API                  │
└─────────────────────────────────────────────────────────┘
```

## 三、接口设计

### BridgeConnector 接口

```go
// BridgeConnector 长连接桥接器接口
// 每种长连接协议（飞书WSS、钉钉Stream）实现此接口
type BridgeConnector interface {
    // 名称
    Name() string

    // 启动连接（阻塞，内部处理重连）
    // onMessage 回调：收到消息时调用，传入原始消息字节
    Start(ctx context.Context, onMessage func(raw []byte)) error

    // 优雅关闭
    Stop() error

    // 当前连接状态
    Status() BridgeStatus
}

type BridgeStatus struct {
    Connected    bool      `json:"connected"`
    ConnectedAt  time.Time `json:"connected_at,omitempty"`
    Reconnects   int       `json:"reconnects"`
    LastError    string    `json:"last_error,omitempty"`
    LastMessage  time.Time `json:"last_message,omitempty"`
    MessageCount int64     `json:"message_count"`
}
```

### ChannelPlugin 接口扩展

```go
type ChannelPlugin interface {
    // ...原有方法不变...

    // 新增：是否支持桥接模式
    SupportsBridge() bool

    // 新增：创建桥接连接器（不支持的返回 nil, error）
    NewBridgeConnector(cfg *Config) (BridgeConnector, error)

    // 新增：将桥接收到的原始消息转为标准 InboundMessage
    // 与 ParseInbound 的区别：ParseInbound 处理 HTTP body（含加密信封）
    // ParseBridgeMessage 处理长连接收到的原始消息（可能已解密）
    ParseBridgeMessage(raw []byte) (InboundMessage, error)

    // 新增：构建转发给 Agent 的 HTTP 请求 body
    // 将 InboundMessage 包装为该平台 Webhook 格式的 JSON
    BuildWebhookBody(msg InboundMessage) ([]byte, error)
}
```

## 四、飞书桥接实现

### 连接流程

```
1. POST /open-apis/auth/v3/tenant_access_token/internal
   → 获取 tenant_access_token

2. WSS wss://open.feishu.cn/callback/ws/endpoint
   Headers: Authorization: Bearer <token>
   → 建立 WebSocket 连接

3. 收到消息帧：
   {"schema":"2.0","header":{"event_id":"...","event_type":"im.message.receive_v1",...},"event":{...}}

4. 解析 event.message.content → InboundMessage

5. 安检 → 构建 Webhook body → POST 到 Agent
```

### 飞书 WSS 协议要点

- **心跳**：服务端发 ping，客户端回 pong（标准 WebSocket ping/pong）
- **重连**：断线后指数退避重连（1s → 2s → 4s → ... → 60s max）
- **Token 刷新**：tenant_access_token 有效期 2 小时，需要定期刷新
- **事件确认**：收到事件后需要在 WebSocket 中回复 `{"headers":{"X-Request-Id":"..."}}`
- **消息格式**：与 Webhook 格式几乎一致（schema 2.0），不需要解密

### 配置

```yaml
channel: "feishu"
mode: "bridge"        # "webhook" (默认) | "bridge" (长连接)

feishu_app_id: "cli_xxx"
feishu_app_secret: "xxx"
# bridge 模式不需要 encrypt_key（消息不加密）
# bridge 模式不需要公网 IP
```

## 五、钉钉桥接实现

### 连接流程

```
1. POST /v1.0/gateway/connections/open
   Headers: clientId + clientSecret
   → 获取 WebSocket 连接票据（endpoint + ticket）

2. WSS <endpoint>?ticket=<ticket>
   → 建立连接

3. 收到消息帧：
   {"specVersion":"1.0","type":"CALLBACK","headers":{...},"data":"..."}

4. data 字段就是原始事件 JSON → 解析 → InboundMessage

5. 回复确认：{"response":{"statusCode":200,"headers":{},"body":"..."}}

6. 安检 → 构建 Webhook body → POST 到 Agent
```

### 钉钉 Stream 协议要点

- **心跳**：服务端发 `{"type":"SYSTEM","headers":{"topic":"/ping"}}`，回复相同格式的 pong
- **消息确认**：必须通过 WebSocket 回复确认，否则会重发
- **重连**：需要重新获取 ticket
- **Subscription**：连接时通过 header 指定订阅的事件类型

### 配置

```yaml
channel: "dingtalk"
mode: "bridge"

dingtalk_client_id: "xxx"       # 原 app_key
dingtalk_client_secret: "xxx"   # 原 app_secret
# bridge 模式不需要 aes_key/token（Stream 模式不加密）
# bridge 模式不需要公网 IP
```

## 六、InboundProxy 改造

```go
type InboundProxy struct {
    channel  ChannelPlugin
    engine   *RuleEngine
    logger   *AuditLogger
    pool     *UpstreamPool
    routes   *RouteTable
    mode     string          // "webhook" | "bridge"
    bridge   BridgeConnector // bridge 模式下非 nil
}
```

### Bridge 模式启动流程

```go
func (ip *InboundProxy) startBridge(ctx context.Context) {
    bridge, _ := ip.channel.NewBridgeConnector(cfg)
    ip.bridge = bridge

    go bridge.Start(ctx, func(raw []byte) {
        // 1. 解析消息
        msg, err := ip.channel.ParseBridgeMessage(raw)

        // 2. 安检（复用现有规则引擎）
        result := ip.engine.Detect(msg.Text)

        // 3. 审计日志
        ip.logger.Log(...)

        // 4. 如果拦截，直接丢弃（长连接模式下无需返回错误响应）
        if result.Action == "block" { return }

        // 5. 构建 Webhook body
        body, _ := ip.channel.BuildWebhookBody(msg)

        // 6. 选择上游 + 转发
        upstream := ip.selectUpstream(msg.SenderID)
        http.Post(upstream.URL, "application/json", bytes.NewReader(body))
    })
}
```

### Webhook 模式不变

`mode: webhook` 时走原有的 `ServeHTTP` 路径，完全不受影响。

## 七、管理 API 扩展

### GET /healthz 新增字段

```json
{
    "mode": "bridge",
    "bridge": {
        "connected": true,
        "connected_at": "2026-03-15T20:00:00Z",
        "reconnects": 0,
        "message_count": 1234,
        "last_message": "2026-03-15T20:10:00Z"
    }
}
```

### Dashboard 更新

系统状态面板新增：
- 接入模式：Webhook / Bridge
- 连接状态：🟢 已连接 / 🔴 断线重连中
- 消息计数 / 最后消息时间 / 重连次数

## 八、Config 变更

```yaml
# 接入模式（所有通道通用）
mode: "webhook"    # "webhook" (默认) | "bridge" (长连接桥接)

# 飞书 bridge 模式需要 app 凭据
feishu_app_id: "cli_xxx"
feishu_app_secret: "xxx"

# 钉钉 bridge 模式需要 client 凭据
dingtalk_client_id: "xxx"
dingtalk_client_secret: "xxx"
```

| channel + mode | 行为 |
|---|---|
| lanxin + webhook | ✅ 默认模式 |
| lanxin + bridge | ❌ 不支持（蓝信只有 Webhook） |
| feishu + webhook | ✅ 传统 Webhook 模式 |
| feishu + bridge | ✅ WSS 长连接，无需公网 IP |
| dingtalk + webhook | ✅ 传统 Webhook 模式 |
| dingtalk + bridge | ✅ Stream 长连接，无需公网 IP |
| wecom + webhook | ✅ 传统 Webhook 模式 |
| wecom + bridge | ❌ 不支持（企微只有 Webhook） |
| generic + webhook | ✅ 默认 |
| generic + bridge | ❌ 不支持 |

## 九、向后兼容

- 不配 `mode` → 默认 `webhook`，行为和 v3.0 一致
- bridge 模式下 `:8443` 入站监听仍然启动（可以同时接收 Webhook，但通常不用）
- 出站 `:8444` 不受影响

## 十、实现优先级

1. **飞书 bridge** — 飞书官方强推 WSS，用户需求最大
2. **钉钉 bridge** — Stream 模式是钉钉新标准
3. Dashboard 桥接状态展示
4. 管理 API 桥接端点

## 十一、依赖

需要引入 WebSocket 库：
- `golang.org/x/net/websocket`（标准扩展库，轻量）
- 或 `github.com/gorilla/websocket`（更成熟，但多一个依赖）

建议用 `golang.org/x/net/websocket`，保持最小依赖原则。
但如果需要细粒度控制（ping/pong/压缩），`gorilla/websocket` 更合适。

**决策点**：用哪个 WebSocket 库？`gorilla/websocket` 功能更全但多一个第三方依赖。
