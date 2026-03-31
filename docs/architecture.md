# 🏛️ 架构说明

> lobster-guard v33.0 · Go ~67,500 行 + 测试 ~37,500 行 · 183 源文件 · 67 张 DB 表 · 5 外部依赖

## 架构总览

左边：**业务流**（IM ↔ OpenClaw ↔ LLM 双向链路）
右边：**龙虾卫士**在每个环节的安全能力

```
  ┌─ 业务流(双向) ──────────────────┐     ┌─ 龙虾卫士 🦞 安全能力 ────────────────┐
  │                                 │     │                                        │
  │  ┌──────────┐                   │     │  :18443 入站代理                       │
  │  │ 消息平台  │  ①用户发消息      │     │  ┌────────────────────────────────┐   │
  │  │ 蓝信/飞书 │ ────────────────────►  │  │ AC自动机(μs) + 正则 + 语义      │   │
  │  │ 钉钉/企微 │                  │     │  │ 行业模板(40×12) + Auto-Review   │   │
  │  └──────────┘                   │     │  │ 奇点蜜罐 + PII标记 + 信封签名  │   │
  │                                 │     │  │ 四级动作: block/review/warn/log  │   │
  │                                 │     │  └──────────────┬─────────────────┘   │
  │                                 │     │        ②通过    │ 拦截→拒绝           │
  │                                 │     │                 ▼                      │
  │                                 │     │  路由表(亲和绑定) → 上游容器池          │
  │                                 │     │                 │                      │
  │  ┌──────────┐  ②消息到达Agent   │     │                 │                      │
  │  │          │ ◄────────────────────── │                 │                      │
  │  │ OpenClaw │                   │     │                                        │
  │  │ AI Agent │                   │     │                                        │
  │  │          │  ③Agent↔LLM(双向) │     │  :8445 LLM 反向代理                   │
  │  │          │ ────────────────────►  │  ┌────────────────────────────────┐   │
  │  └──────────┘ ◄────────────────────── │  │ ┌─ 请求方向 ──┐ ┌─ 响应方向 ──┐│   │
  │       │                         │     │  │ │ LLM规则     │ │ LLM规则     ││   │
  │       │                         │     │  │ │ 工具策略    │ │ IFC Hide    ││   │
  │       │                         │     │  │ │ 执行计划    │ │ 偏差检测    ││   │
  │  ┌──────────┐                   │     │  │ │ Capability  │ │ 反事实验证  ││   │
  │  │ LLM API  │ ◄─── :8445 ──────────► │  │ │ IFC检查     │ │ 污染逆转    ││   │
  │  │ DeepSeek │  (龙虾卫士代理)   │     │  │ │ 蜜罐注入    │ │ Canary检测  ││   │
  │  │ GLM/Qwen │                   │     │  │ └─────────────┘ └─────────────┘│   │
  │  └──────────┘                   │     │  │ SSE streaming 安全扫描          │   │
  │                                 │     │  └────────────────────────────────┘   │
  │                                 │     │                                        │
  │                                 │     │  :18444 出站代理                       │
  │  ┌──────────┐  ⑤Agent回复       │     │  ┌────────────────────────────────┐   │
  │  │ OpenClaw │ ────────────────────►  │  │ 出站规则(PII/凭据/命令)        │   │
  │  │ AI Agent │                   │     │  │ 污染追踪(12 PII) + 逆转引擎   │   │
  │  └──────────┘                   │     │  │ block/warn/log 三级策略        │   │
  │                                 │     │  └──────────────┬─────────────────┘   │
  │                                 │     │        ⑥通过    │ 拦截→脱敏/阻断      │
  │  ┌──────────┐  ⑥消息送达用户    │     │                 ▼                      │
  │  │ 消息平台  │ ◄────────────────────── │  调用消息平台API发回                   │
  │  │          │                   │     │                                        │
  │  └──────────┘                   │     │                                        │
  │                                 │     │  :9090 管理层                          │
  └─────────────────────────────────┘     │  ┌────────────────────────────────┐   │
                                          │  │ Dashboard(50页面) + API(487路由)│   │
  完整业务流:                             │  │ SQLite(67表) + 安全画像(5维)   │   │
  ① 用户发消息 → 消息平台回调             │  │ WSS RPC → Gateway远程管理      │   │
  ② → 龙虾卫士入站检测 → OpenClaw         │  │ 多租户 · 会话回放 · 报告 · 红队 │   │
  ③ Agent ↔ 龙虾卫士LLM代理 ↔ LLM(双向)  │  └────────────────────────────────┘   │
  ④ (Agent思考、多轮tool_call)            │                                        │
  ⑤ Agent回复 → 龙虾卫士出站检测          │  横切面引擎(贯穿全链路):                │
  ⑥ → 消息平台API → 用户收到回复          │  · 行为画像 + 攻击链 + 路径策略         │
                                          │  · 信封签名 + Merkle验证 + 事件总线     │
  龙虾卫士 = 安全侧车                     │  · 自进化 + 语义检测 + 安全画像         │
  不改业务流，在3个接触点注入安全检测:     │  · 审计日志 + trace_id 全链路关联       │
  入站(:18443) · LLM(:8445) · 出站(:18444)│                                        │
                                          └────────────────────────────────────────┘
```

## 4 端口架构

| 端口 | 功能 | 安全域 | 协议 |
|------|------|--------|------|
| `:18443` | IM 入站代理 | IM 安全域 | HTTP(S) |
| `:18444` | IM 出站代理 | IM 安全域 | HTTP(S) |
| `:8445` | LLM 反向代理 | LLM 安全域 | HTTP(S) + SSE |
| `:9090` | Dashboard + API + WebSocket | 管理层 | HTTP + WS |

---

## 检测引擎全景 (21 引擎)

### IM 域引擎

| 引擎 | 源文件 | 功能 |
|------|--------|------|
| RuleEngine | `detect.go` | AC 自动机 O(n) + 正则 + 行业模板(40×12) + 四级动作(block/review/warn/log) |
| AutoReviewEngine | `auto_review.go` | AC 快筛 + LLM 语义复核(降级→warn) |
| SemanticDetector | `semantic.go` | TF-IDF + 句法 + 异常 + 意图四维(47 模式) |
| SingularityHoneypot | `singularity.go` | 拓扑预算(欧拉χ) + 暴露等级 + 帕累托推荐 |
| EvolutionEngine | `evolution.go` | 6 策略变异 + 规则建议队列 + 人工审批 |
| EnvelopeEngine | `envelope.go` | HMAC-SHA256 签名 + Merkle Tree 批量验证 + 金丝雀轮换 |
| EventBus | `event_bus.go` | SecurityEvent → Webhook + ActionChain 编排 |

### LLM 域引擎

| 引擎 | 源文件 | 功能 |
|------|--------|------|
| LLMRuleEngine | `llm_rules.go` | request/response/both 三方向规则 + AC 自动机 |
| ToolPolicyEngine | `tool_policy.go` | tool_calls 规则 + 通配符 + 滑窗限流 + 租户黑名单 |
| PlanCompiler | `plan_compiler.go` | CaMeL 意图→执行计划编译(20+ 模板, 6 分类) |
| CapabilityEngine | `capability.go` | 数据级权限标签(Sources 并集 + Labels 交集 + TrustScore) |
| DeviationDetector | `deviation.go` | 计划 vs 实际偏差检测 + 策略化自动修复(block/warn/rewrite) |
| IFCEngine | `ifc_engine.go` | Bell-LaPadula 双标签(机密性+完整性) + 格积运算 |
| SelectiveHide | `ifc_engine.go` | Fides HIDE 函数 — tool result 标签驱动隐藏 |
| CounterfactualVerifier | `counterfactual.go` | 反事实对照验证 + 归因报告 + 自适应策略 + 成本追踪 |
| TaintTracker | `taint.go` | 12 PII 模式 + IM↔LLM trace 关联 + 三端传播 |
| TaintReversal | `taint.go` | soft/hard/stealth 三强度 + SSE 流式逆转 |
| LLMCache | `llm_cache.go` | TF-IDF 语义匹配 + 租户隔离 + 污染跳过 |
| HoneypotDeep | `honeypot.go` | 忠诚度曲线 + 蜜罐 token 注入 + 引爆检测 |

### 分析引擎

| 引擎 | 源文件 | 功能 |
|------|--------|------|
| BehaviorProfiler | `behavior.go` | 特征提取 + 模式学习 + 异常基线 |
| AttackChainDetector | `attack_chain.go` | 多阶段关联 + Kill Chain 映射 + 自动升级 |
| PathPolicyEngine | `path_policy.go` | 序列/累计/条件规则 + 风险仪表 + AI Act 模板 |
| UpstreamProfileEngine | `upstream_profile.go` | 5 维评分(入站/LLM/数据/行为/工具) × 16 引擎聚合 |

---

## 三层数据安全模型

```
┌─────────────────────────────────────────────────────────────┐
│  IFC (v26, Fides)                                           │
│  数据"什么密级" → 流向控制                                    │
│  Bell-LaPadula 双标签(Confidentiality, Integrity)           │
│  格积(lattice) 运算 → 隔离路由 + Selective Hide              │
├─────────────────────────────────────────────────────────────┤
│  Capability (v25, CaMeL)                                    │
│  数据"从哪来" → 来源追溯                                     │
│  Sources 并集 + Labels 交集 + TrustScore                    │
│  ACL-like 集合运算                                           │
├─────────────────────────────────────────────────────────────┤
│  Deviation (v25, CaMeL)                                     │
│  LLM"想干什么" → 行为合规                                    │
│  PlanCompiler(意图→计划) → DeviationDetector(计划 vs 实际)   │
│  策略化修复: block / warn / rewrite                          │
└─────────────────────────────────────────────────────────────┘
互补不冲突: CaMeL = ACL-like(集合运算), Fides = MLS-like(格运算)
```

---

## 入站检测管线

消息从 IM 平台到达 `:18443` 后的完整处理链：

```
消息到达 → ChannelPlugin 解密验签
    │
    ▼
AC 自动机 (μs) ─── 命中block → 拦截 + 审计
    │ 通过
    ▼
正则引擎 (ms, 100ms超时保护) ─── 命中block → 拦截
    │ 通过
    ▼
行业模板检测 (40模板×12分类) ─── 命中 → 按规则动作处理
    │ 通过/review
    ▼
Auto-Review (仅对review规则) ─── LLM语义复核 → block/warn/通过
    │                                  ↓ LLM失败
    │                           降级为 warn
    ▼
语义检测 (TF-IDF四维) ─── 可疑 → warn
    │ 通过
    ▼
奇点蜜罐 ─── 陷阱触发 → 审计
    │
    ▼
PII标记 (12模式) ─── 命中 → taint_entries写入 + trace_id关联
    │
    ▼
信封签名 (HMAC-SHA256) → Merkle 批量入队
    │
    ▼
事件发射 (SecurityEvent → Webhook/ActionChain)
    │
    ▼
路由表(亲和绑定) → 上游容器池 → OpenClaw
```

---

## LLM 代理检测管线

Agent 发起 LLM 请求到达 `:8445` 后的完整处理链：

```
┌─────────────── 请求方向 ──────────────────┐
│                                           │
│  LLM 请求到达                              │
│      │                                    │
│      ▼                                    │
│  API Key 身份识别 → 租户解析               │
│      │                                    │
│      ▼                                    │
│  LLM 规则检测 (request方向)                │
│      │                                    │
│      ▼                                    │
│  工具策略 (tool_calls白/黑名单+限流)        │
│      │                                    │
│      ▼                                    │
│  执行计划编译 (CaMeL: 意图→计划)            │
│      │                                    │
│      ▼                                    │
│  Capability 权限评估 (TrustScore)          │
│      │                                    │
│      ▼                                    │
│  IFC 信息流检查 (Bell-LaPadula)            │
│      │                                    │
│      ▼                                    │
│  蜜罐 token 注入                           │
│      │                                    │
│      ▼                                    │
│  缓存查询 (TF-IDF 语义匹配)               │
│      │ 未命中                             │
│      ▼                                    │
│  转发 → 上游 LLM API                      │
│                                           │
└───────────────────────────────────────────┘

┌─────────────── 响应方向 ──────────────────┐
│                                           │
│  上游 LLM 响应 (非流式 / SSE 流式)         │
│      │                                    │
│      ▼                                    │
│  LLM 规则检测 (response方向)               │
│      │                                    │
│      ▼                                    │
│  IFC Selective Hide (标签驱动隐藏)          │
│      │                                    │
│      ▼                                    │
│  偏差检测 (计划 vs 实际 tool_calls)         │
│      │                                    │
│      ▼                                    │
│  反事实验证 (高风险 tool_call 对照)         │
│      │                                    │
│      ▼                                    │
│  污染逆转 (soft/hard/stealth)              │
│      │                                    │
│      ▼                                    │
│  Canary 泄露检测                           │
│      │                                    │
│      ▼                                    │
│  信封签名 → 审计日志 → 事件发射            │
│      │                                    │
│      ▼                                    │
│  返回 Agent                               │
│                                           │
└───────────────────────────────────────────┘
```

---

## Taint 全链路闭环

IM 入站标记 → LLM 传播 → SSE/非流式逆转 的完整链路：

```
IM 入站 (:18443)
  │ PII 检测(12模式) → taint_entries 写入
  │ SessionCorrelator: sender_id ↔ trace_id 关联
  ▼
转发到 OpenClaw
  │
LLM Proxy (:8445)
  │ 查询 SessionCorrelator → taintTraceID
  │ taint propagation → taint_entries += "llm_request"
  ▼
上游 LLM 响应
  ├── 非流式: Reverse(traceID) → 检查 labels → soft/hard/stealth
  └── SSE 流式: stream 完成后 → Reverse → 追加 SSE event
  │
  ▼
taint_reversals 表记录
propagation chain: inbound → llm_request → llm_response
```

---

## 通道插件架构

```
┌──────────────────────────────────────────────────────────────┐
│                     ChannelPlugin 接口                        │
│                                                              │
│  Parse(req) → (text, senderID, metadata, error)              │
│  Verify(req) → bool                                          │
│  Decrypt(body) → plaintext                                   │
│                                                              │
├──────────┬──────────┬──────────┬──────────┬──────────────────┤
│ 🔵 蓝信  │ 🟢 飞书  │ 🔷 钉钉  │ 🟠 企微  │ ⚪ 通用 HTTP     │
│ AES+SHA1 │ AES+SHA2 │ AES+HMAC │ AES+XML  │ 明文 JSON       │
│ URL签名  │ Header   │ Header   │ Body签名 │                  │
├──────────┴──────────┴──────────┴──────────┴──────────────────┤
│            Bridge Mode: WSS 长连接 (飞书/钉钉)                │
│            消息平台 ←WSS→ lobster-guard ←HTTP→ OpenClaw       │
├──────────────────────────────────────────────────────────────┤
│                InboundProxy / OutboundProxy                   │
│            (安全检测、路由、审计 — 通道无关)                    │
└──────────────────────────────────────────────────────────────┘
```

### 蓝信特殊处理

- `timestamp`/`nonce`/`signature` 在 URL query（不在 body）
- 解密后含多事件拼接 + appId 尾缀 → `extractFirstJSON` 括号匹配
- 两个用户信息接口: `/v1/staffs/{id}/fetch`(基本) vs `/v1/staffs/{id}/infor/fetch`(含邮箱手机)

---

## 多租户架构

```
┌─────────────────────────────────────────────────────┐
│                     请求到达                         │
│                        │                            │
│                        ▼                            │
│              API Key 身份识别                        │
│              Bearer sk-xxx → apikeys 表             │
│              SHA-256 哈希匹配 + 热缓存              │
│                        │                            │
│                        ▼                            │
│              resolveTenantID()                       │
│              API Key → 租户映射                     │
│                        │                            │
│                        ▼                            │
│   ┌────────────────────┼────────────────────┐       │
│   │                    │                    │       │
│   ▼                    ▼                    ▼       │
│ 默认规则         全局模板(启用)        租户绑定模板   │
│ (code defaults)  (industry_templates)  (tenant→tpl) │
│                                                     │
│   三者 UNION → 并行检测 → 最高权重动作胜出          │
│                                                     │
│ 租户级控制:                                         │
│   - DisabledRules: 跳过指定规则                     │
│   - ToolBlacklist: 拦截指定工具                     │
│   - 模板绑定: BindTemplateToTenant()                │
│   - 日配额: API Key 级别                           │
└─────────────────────────────────────────────────────┘
```

---

## Gateway 远程管理 (v29)

```
┌──────────────────────────────────────────────┐
│  lobster-guard Dashboard (:9090)             │
│                                              │
│  GatewayWSManager (连接池)                   │
│    │                                         │
│    ├── upstream-1 → GatewayWSClient ─WSS──→ Gateway-1 (:19444)
│    ├── upstream-2 → GatewayWSClient ─WSS──→ Gateway-2
│    └── upstream-N → GatewayWSClient ─WSS──→ Gateway-N
│                                              │
│  WSS RPC 协议:                               │
│    challenge → connect(token) → hello → ready│
│    client.id = "openclaw-control-ui"         │
│    role = "operator"                         │
│    scopes = [admin, approvals, pairing]      │
│                                              │
│  88 RPC methods:                             │
│    sessions.* / chat.* / cron.* / agents.*   │
│    config.* / skills.* / exec-approvals.*    │
│    heartbeat.* / devices.* / nodes.*         │
│                                              │
│  双路径: WSS RPC 优先 → tools/invoke 降级    │
│  延迟: 11-44ms (WSS) vs 78-184ms (HTTP)     │
└──────────────────────────────────────────────┘
```

---

## 会话回放架构

使用 `trace_id` 串联三张表，还原完整对话时间线：

```
trace_id 串联:
  audit_log        → IM 层每条消息的检测记录
  llm_calls        → LLM Agent 层的调用记录
  llm_tool_calls   → LLM 返回的 tool_calls

时间线:
  T1: IM 消息到达 → audit_log (入站检测结果)
  T2: OpenClaw 调 LLM → llm_calls (请求/响应/token)
  T3: LLM 返回 tool_calls → llm_tool_calls (工具名/参数)
  T4: 出站响应 → audit_log (出站检测结果)

展示:
  聊天气泡风格 + 安全事件标注 + 标签注释 + 导出 JSON/MD
```

---

## 安全画像架构 (v33)

```
upstream_profile.go: 5 维评分 × 16 引擎 × 14 张 DB 表

维度        满分   数据源
─────────  ────  ──────────────────────────────
入站防护    20    audit_log(入站block/warn率)
LLM安全     20    llm_rule_hits + llm_calls(规则命中/暴露数据)
数据防泄漏  20    taint_entries + ifc_violations + taint_reversals(+bonus) + ifc_hidden_content(+bonus)
行为合规    20    plan_deviations/plan_executions(偏离率) + behavior_anomalies + attack_chains
工具管控    20    execution_envelopes(信封失败率) + cap_evaluations(权限拒绝率) + tool_call_events

评分方式: 比率评分(偏离率/拒绝率/失败率), 非绝对数
正面信号: taint_reversals + ifc_hidden_content = 防御成功, 加分(+3 bonus)
5 档: 优秀(>80) / 良好(61-80) / 一般(41-60) / 较差(20-40) / 危险(<20)
```

---

## 安全层级 (8 层)

| 层级 | 名称 | 引擎 | 延迟 |
|------|------|------|------|
| L1 | 模式匹配 | AC 自动机 + 正则 + 行业模板(40×12) | μs |
| L2 | 语义分析 | TF-IDF 四维 + LLM auto-review | ms-s |
| L3 | 行为分析 | 画像 + 基线 + 攻击链 + 路径策略 | s |
| L4 | 密码学保证 | HMAC 信封 + Merkle Tree + 金丝雀轮换 | ms |
| L5 | 自进化 | 对抗变异 + 规则生成 + 建议审批 | min |
| L6 | 污染追踪 | IM↔LLM trace + SSE 逆转 + IFC | 全链路 |
| L7 | 意图验证 | CaMeL 计划 + 偏差检测 + Capability | 决策级 |
| L8 | 安全画像 | 5 维评分 + 16 引擎聚合 + 趋势 | 运营级 |

---

## 存储架构

```
SQLite WAL 模式
├── 67 张表
├── BatchAuditWriter (channel 缓冲 + 批量写入)
├── 审计日志保留策略 (归档 + 导出)
└── 配置分层:
    ├── config.yaml        (核心配置)
    ├── conf.d/*.yaml      (模块化覆盖, 同名字段后文件覆盖前文件)
    └── DB (industry_templates, tenant_configs, ...)
        (运行时 CRUD, API 写回)

⚠️ conf.d slice 字段: yaml.Unmarshal 是替换不是追加!
   多文件定义同一 slice → 必须 loadConfDir 手动合并
```

---

## 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/mattn/go-sqlite3` | SQLite 驱动 (CGO) |
| `gopkg.in/yaml.v3` | YAML 配置解析 |
| `github.com/google/uuid` | UUID 生成 |
| `github.com/gorilla/websocket` | WebSocket (Bridge Mode + WSS RPC) |
| `golang.org/x/crypto` | bcrypt 密码哈希 |

5 个外部依赖，其余全部 Go 标准库。单二进制 ~36MB（含 Dashboard 前端）。

---

## 构建与嵌入

```
dashboard/             ← Vue 3 源码
  └── dist/            ← vite build 产物

src/dashboard.go       ← go:embed dashboard/dist/*
src/main.go            ← 编译入口

⚠️ go:embed 缓存: go build 不会因 dist 文件变化重编译 dashboard.go
   必须 touch src/dashboard.go 后再 go build

⚠️ vite outDir: dashboard/vite.config.js 中 outDir 设为 ../src/dashboard/dist
   保证 go:embed 路径一致
```

---

## LLM Proxy strip_prefix 路由

```
客户端请求                     strip_prefix=true            上游收到
─────────────                ─────────────────           ──────────
/qax/v1/chat/completions  →  去掉 /qax/v1/ 前缀  →  /v1/chat/completions
                               target: aip.b.qianxin-inc.cn

/v1/chat/completions      →  strip_prefix=false     →  /v1/chat/completions
                               target: api.deepseek.com
```

用于将 OpenClaw 的 LLM 流量透明引入龙虾卫士审计，无需修改上游 API 路径。
