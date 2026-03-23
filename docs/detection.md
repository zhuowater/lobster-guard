# 🛡️ 安全检测能力

> 返回 [README](../README.md)

## 规则体系

### 入站检测（多层防御）

**DetectPipeline 检测链**: Keyword → Regex → PII → Session → LLM（可配置顺序和启停）

| 层级 | 引擎 | 说明 |
|------|------|------|
| 关键词 | AC 自动机 | O(n) 多模式匹配，40+ 内置规则 |
| 正则 | regexp | 自定义正则模式，100ms 超时保护 |
| 规则模板 | go:embed | 4 场景 64 条规则（通用/金融/医疗/政务）|
| 上下文感知 | SessionDetector | 会话级风险积分 · 多轮攻击识别 · 自动升级 |
| 语义分析 | LLMDetector | 可选 · async/sync · 外部 LLM API · fail-open |
| 结果缓存 | DetectCache | SHA-256 LRU · pass/warn 缓存 · block 不缓存 |

### 入站检测规则

| 类别 | 检测内容 | 动作 |
|------|----------|------|
| Prompt Injection | `ignore previous instructions`, `忽略之前的指令` | 🔴 Block |
| 越狱攻击 | `you are now DAN`, `jailbreak`, `没有限制的AI` | 🔴 Block |
| 系统提示词窃取 | `show system prompt`, `输出提示词` | 🔴 Block |
| 命令注入 | `rm -rf /`, `curl\|bash`, `base64 -d\|bash` | 🔴 Block |
| 角色扮演诱导 | `假设你是`, `pretend you are` | 🟡 Warn |
| PII 检测 | 身份证号、手机号、银行卡号 | 🟡 Warn + 记录 |

### 出站检测（v18 智能合并）

6 条默认规则始终加载，用户配置同名覆盖、新名称追加：

| 默认规则 | 检测内容 | 动作 |
|----------|----------|------|
| `pii_id_card` | 身份证号 | 🔴 Block |
| `pii_phone` | 手机号 | 🟡 Warn |
| `pii_bank_card` | 银行卡号 | 🔴 Block |
| `credential_password` | 密码泄露 | 🔴 Block |
| `credential_apikey` | API Key（sk-/ghp_/AKIA） | 🔴 Block |
| `malicious_command` | 恶意命令（rm -rf/curl\|bash） | 🔴 Block |

### LLM 规则（v18 智能合并）

11 条默认规则始终加载，用户配置同名覆盖、新名称追加：

| 类别 | 规则数 | 检测内容 |
|------|--------|----------|
| PI 检测 | 3 | Prompt Injection 注入检测 |
| PII 检测 | 3 | 请求/响应中的个人信息 |
| 敏感话题 | 1 | 政治/暴力/违法内容 |
| Token 滥用 | 1 | Token 用量异常 |
| 响应方向 | 3 | 响应内容安全检测（v18 新增）|

## 检测管线

```
入站消息
  │
  ▼
┌──────────────┐
│ Keyword 层   │ ── AC 自动机 O(n) 多模式匹配
├──────────────┤
│ Regex 层     │ ── 自定义正则 + 100ms 超时
├──────────────┤
│ PII 层       │ ── 身份证/手机/银行卡
├──────────────┤
│ Session 层   │ ── 会话级风险积分 + 自动升级
├──────────────┤
│ LLM 层       │ ── 可选语义分析（async/sync）
└──────┬───────┘
       │
       ▼
  检测结果（pass / warn / block）
       │
       ├── block → 拦截 + 告警 + 审计
       ├── warn  → 放行 + 记录 + 积分
       └── pass  → 放行（缓存）
```

## 污染追踪（Taint Tracking）

### IM↔LLM Trace 关联 (v20.8.1+)

LLM Proxy 在处理请求时，优先使用 `SessionCorrelator` 关联的 IM `trace_id`（而非 LLM 自身生成的 trace_id），确保入站 IM 检测阶段标记的 taint 在 LLM 响应路径中被正确关联。

```
IM 入站（trace_id=T1）
  → PII 检测命中 → taint_entries 写入 T1
  → 转发 OpenClaw
  → Agent 调用 LLM Proxy
  → SessionCorrelator 查询: sender → T1
  → taintTraceID = T1（而非 LLM 自身 trace）
  → taint propagation 使用 T1
  → reversal 检查 T1 的 labels
```

这解决了之前 LLM trace_id 与入站 IM trace_id 不匹配导致 taint 链断裂的问题。

### SSE 流式逆转

当 LLM 响应以 SSE (Server-Sent Events) 流式传输时，数据已逐行推送给客户端。龙虾卫士在流结束后执行逆转：

1. 流正常传输完毕
2. `taint propagation` 使用关联的 IM trace_id 触发
3. `reversalEngine.Reverse()` 检查活跃的 taint labels
4. 命中时追加一个自定义 SSE 事件：

```
event: lobster_guard_taint_reversal
data: {"warning": "检测到敏感信息泄露风险...", "template": "pii-soft-1", "labels": ["pii_phone"]}
```

5. 客户端通过 `event` 类型区分正常 LLM 输出和安全缓解提示

**客户端集成示例**（EventSource）:

```javascript
const es = new EventSource('/v1/chat/completions');
es.addEventListener('lobster_guard_taint_reversal', (e) => {
  const warning = JSON.parse(e.data);
  showSecurityWarning(warning);
});
```

### 非流式逆转

普通 HTTP 响应模式下，`reversalEngine.Reverse()` 在响应返回客户端前自动调用：

- **soft 模式**：在响应体末尾追加安全提示
- **hard 模式**：替换整个响应内容为安全提示
- **stealth 模式**：静默记录到 `taint_reversals` 表，不修改响应

## 规则模板库

4 场景 64 条预置规则，通过 `go:embed` 编译进二进制：

| 模板 | 场景 | 规则数 |
|------|------|--------|
| `general.yaml` | 通用（越狱/注入/社工） | — |
| `financial.yaml` | 金融 | — |
| `medical.yaml` | 医疗 | — |
| `government.yaml` | 政务 | — |

规则文件位于 `rules/` 目录，可通过 API 热更新。
