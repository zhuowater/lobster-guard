# lobster-guard Roadmap

## 已完成

### v1.0 — 基础安全代理
- [x] 入站 Prompt Injection 检测（AC 自动机 40+ 规则）
- [x] 出站敏感信息审计（PII/凭据/私钥）
- [x] 单上游代理模式
- [x] SQLite 审计日志
- [x] AES-256-CBC 蓝信加解密

### v2.0 — 多容器与管理
- [x] 出站 block/warn/log 三级策略
- [x] 用户 ID 亲和路由（多容器负载均衡）
- [x] 容器自动注册/心跳/故障转移
- [x] 管理 API（11 端点）
- [x] Web Dashboard（深色科技主题，单 HTML）
- [x] 53 个测试用例（33 单元 + 20 集成）

### v3.0 — 多通道插件
- [x] ChannelPlugin 接口抽象
- [x] 5 个通道插件：蓝信/飞书/钉钉/企微/通用HTTP
- [x] 一行配置切换通道
- [x] 向后兼容

### v3.1 — Bridge Mode（长连接桥接）
- [x] BridgeConnector 接口
- [x] 飞书 WSS 长连接桥接（token 刷新、断线重连、事件确认）
- [x] 钉钉 Stream 长连接桥接（ticket 获取、心跳、消息确认）
- [x] bridge/webhook 混合模式
- [x] /healthz 桥接状态展示

### v3.2 — 企微 GET 验证 + 健壮性
- [x] 企微入站 GET 请求验证（echostr 校验）
- [x] 飞书 URL Verification 回调优化
- [x] 各通道插件的单元测试补充
- [x] 入站超时保护 (30s)、出站 body 大小限制 (10MB)
- [x] Panic recovery、审计日志截断 (500 chars)

### v3.3 — Rate Limiting
- [x] 全局 QPS 限流（令牌桶算法）
- [x] 按 sender_id 限流（防单用户滥用）
- [x] 白名单豁免
- [x] 限流统计 API + reset
- [x] 429 响应 + Retry-After header

### v3.4 — Prometheus Metrics
- [x] /metrics 端点（Prometheus 格式，手工生成无依赖）
- [x] 13 个指标族：请求量/延迟直方图/上游健康/路由/桥接/限流/运行时间
- [x] 按通道/方向/动作分类

### v3.5 — 入站规则热更新
- [x] 入站规则配置文件分离（inbound_rules_file YAML）
- [x] /api/v1/inbound-rules/reload 端点
- [x] AC 自动机在线重建（RWMutex 保护，不停服务）
- [x] 规则版本管理（version/source/loaded_at）
- [x] -gen-rules 导出默认规则为 YAML

### v3.6 — 规则引擎增强
- [x] 规则优先级权重（多规则匹配取最高优先级）
- [x] 自定义响应模板（每条规则可配不同的拦截提示语）
- [x] 规则命中率统计（API + Prometheus 指标）

### v3.7 — 蓝信实战集成验证（2026-03-16）
- [x] 修复签名参数位置（URL query 提取 timestamp/nonce/signature）
- [x] 新增 RequestAwareParser 接口
- [x] 修复 SenderID 字段名兼容（FromStaffId/from）
- [x] 修复解密格式（extractFirstJSON 括号匹配，对齐 OpenClaw）
- [x] 审计日志新增 app_id 字段 + 出站接收者识别
- [x] Dashboard 审计日志表增加 App ID 和消息内容列
- [x] 双向全链路实测通过

### v3.8 — 多 Bot 亲和路由（2026-03-16）
- [x] 路由键从 `sender_id` 升级为 `(sender_id, app_id)` 复合键
- [x] 同一用户访问不同 Bot 可路由到不同龙虾容器
- [x] 批量绑定 API（按部门、按列表）+ 路由统计 API
- [x] Dashboard 企业级路由管理面板（Bot/部门筛选、批量操作）
- [x] 数据库 schema 自动迁移（旧单键表 → 新复合键表）

### v3.9 — IM 用户信息自动获取 + 邮箱策略路由（2026-03-16）
- [x] 4 平台 UserInfoProvider：蓝信/飞书/钉钉/企微自动获取用户信息
- [x] 蓝信详细信息接口：姓名、邮箱、手机号、部门一次获取
- [x] UserInfoCache：内存+SQLite 两级缓存，24h TTL，异步不阻塞
- [x] RoutePolicyEngine：基于 email/email_suffix/department/app_id 策略匹配
- [x] 路由决策：精确路由 → 策略匹配 → 动态分配
- [x] Dashboard 路由表自动显示姓名/邮箱/手机号
- [x] 6 个新 API 端点（用户管理 + 策略管理）

**当前状态**: main.go 6207 行 · 243 个测试 · 31 个 commit · 3 个依赖

## 计划

### v4.0 — 多租户
- [ ] 多租户隔离（不同 app 走不同规则集）
- [ ] 租户级路由策略
- [ ] 租户级统计和审计
- [ ] 租户管理 API + Dashboard 视图

### v4.1 — WebSocket 代理
- [ ] Agent WebSocket 连接代理（实时对话场景）
- [ ] WebSocket 消息帧检测
- [ ] WebSocket 连接生命周期管理

### v4.2 — 高可用
- [ ] 多实例部署（active-active）
- [ ] 共享存储（PostgreSQL 替代 SQLite）
- [ ] 路由表跨实例同步
- [ ] Leader 选举（Bridge 模式防重复连接）

### 未来可能
- [ ] Slack / Teams / Telegram 通道插件
- [ ] 基于 LLM 的智能检测（语义级攻击识别）
- [ ] 可视化规则编辑器（拖拽式）
- [ ] 审计日志导出（CSV/JSON/S3）
- [ ] 告警通知（邮件/webhook/企微机器人）

## 版本发布原则

1. **每个版本都可独立使用** — 不存在"必须升级到 vX 才能用"的情况
2. **向后兼容** — 新版本不破坏旧配置
3. **单文件优先** — 尽量保持 main.go 单文件
4. **测试先行** — 每个 feature 必须有测试覆盖
5. **文档同步** — 代码改完必须同步更新 README/config.example
