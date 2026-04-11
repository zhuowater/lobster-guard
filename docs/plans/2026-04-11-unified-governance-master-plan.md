# Unified Governance Engine Master Plan

> 这是一份整合版总文档，合并了：系统评审、Preview/Commit 改造计划、Proposal/Outcome Schema 设计。目标不是立刻开做统一裁决器，而是在**尽可能保留现有特性**的前提下，把 lobster-guard 现有高级治理能力系统性梳理清楚，并给出一条稳妥的融合路径。

---

# 0. 核心结论

当前 lobster-guard 已经有一套非常强的高级治理能力栈，但它们不是“多个同质 block/warn 引擎”，而是混合了多种不同语义：

- **Verdict**：真正提出 allow / warn / block / deny 的引擎
- **Transform**：rewrite / redact / selective hide / sanitize / reversal / replace 等内容或动作修复器
- **Route**：quarantine / isolate / expose 这类执行路径分流器
- **Signal / Evidence**：DOE、path risk、taint、attribution、lineage、hidden content、trust score 等证据和风险信号
- **State Machine / Object Layer**：Plan、Path、Capability、IFC 这些会维护上下文、对象和图谱的底层系统

所以：

> **统一治理不能先做“统一 block/warn/allow”，而必须先做“统一治理语义”。**

否则一定会伤到现有能力，尤其是：

- IFC 的 `SelectiveHide` / `HideContent` / `Quarantine`
- Deviation 的 `replace_tool` / `sanitize_args`
- LLM rule engine 的 `rewrite` / `singularity expose`
- TaintReversal 的 `soft/hard/stealth`
- PathPolicy 的 `isolate`
- Counterfactual 的 async 模式

---

# 1. 融合目标

统一治理引擎的目标不是“把所有引擎并到一个文件里”，而是：

1. **保留现有高级能力，不做功能回退**
2. **让引擎间语义边界明确**
3. **消除 preview 和 commit 混在一起的问题**
4. **让最终执行与审计、解释、状态提交一致**
5. **为未来 MCP / cross-agent / 更细粒度 capability vocabulary 铺路**

一句话版本：

> 在不削弱现有 IFC / Capability / Path / Plan / Deviation / Counterfactual / Taint / Rewrite 能力的前提下，做一套可渐进落地的融合治理引擎。

---

# 2. 现有引擎系统评审

## 2.1 ToolPolicyEngine

**代码位置**
- `src/tool_policy.go`
- `src/llm_tool_governance.go`

**主要能力**
- 工具名/参数规则
- 敏感路径检测
- rate limit
- 审计事件

**系统角色**
- 主要是 **verdict engine**

**当前问题**
- 已与 PathPolicy 混裁
- tenant blacklist 现在更像“静默 continue”而非显式 final block

**融合建议**
- 保持为 decision proposal source
- 先把 blacklist / path merge 拆干净

---

## 2.2 PathPolicyEngine

**代码位置**
- `src/path_policy.go`
- `src/llm_tool_governance.go`

**主要能力**
- path context
- risk score
- sequence / cumulative / degradation rules
- `allow / warn / block / isolate`

**系统角色**
- state engine
- risk engine
- verdict engine
- route-like engine（`isolate`）

**当前问题**
- 先 `RegisterStep` 再 `Evaluate`
- proposed action 会污染 history
- `isolate` 有语义，但执行层没接住

**融合建议**
- path state 保留
- `isolate` 升格为 first-class route action
- 必须拆 Preview / Commit

---

## 2.3 CounterfactualVerifier

**代码位置**
- `src/counterfactual.go`
- `src/llm_tool_governance.go`

**主要能力**
- sync/async 反事实验证
- attribution score
- budget / cache
- attribution report

**系统角色**
- sync 时：verdict engine
- async 时：signal / evidence engine

**当前问题**
- 如果强行统一成“每次都给 final verdict”，会破坏 async 模式与成本控制

**融合建议**
- sync → decision proposal
- async → signal proposal

---

## 2.4 PlanCompiler

**代码位置**
- `src/plan_compiler.go`
- `src/llm_deep_governance.go`

**主要能力**
- active plan 状态机
- tool call vs plan 模板匹配
- step progression
- violation log

**系统角色**
- state machine
- verdict engine

**当前问题**
- `EvaluateToolCall()` 内部会推进 plan 状态
- 主链 + DeviationDetector 可能重复评估

**融合建议**
- 必须拆：
  - `PreviewToolCall()`
  - `CommitToolCall()`
- 状态推进只能在 final allow/repaired-allow 之后发生

---

## 2.5 DeviationDetector

**代码位置**
- `src/plan_deviation.go`
- `src/llm_deviation_governance.go`

**主要能力**
- deviation detection
- repair policies
- `replace_tool`
- `sanitize_args`
- `block/skip/log`

**系统角色**
- verdict engine
- transform / repair engine
- audit engine

**当前问题**
- capability violation 分支用的是 `block`，但 CapabilityEngine 实际是 `deny`
- detection / repair / persistence 混在一起

**融合建议**
- repair proposal 和 decision proposal 分开
- repair 能力绝不能因为统一而丢失

---

## 2.6 CapabilityEngine

**代码位置**
- `src/capability.go`
- `src/llm_deep_governance.go`

**主要能力**
- CapContext
- DataItems
- sources / parent_ids / trust_score
- propagation
- provenance-aware evaluation

**系统角色**
- object layer
- propagation layer
- verdict engine

**当前问题**
- 先 RegisterToolResult，再 Evaluate
- 主链仍偏 log/audit，不是硬执行门
- 与 Deviation 存在 decision enum 错位

**融合建议**
- object/provenance 保留为 shared evidence layer
- evaluation 则进入 decision proposal

---

## 2.7 IFCEngine

**代码位置**
- `src/ifc_engine.go`
- `src/ifc_quarantine.go`
- `src/llm_ifc_governance.go`
- `src/llm_proxy.go`

**主要能力**
- 双标签（Conf + Integ）
- `CheckToolCallFides` (P-F / P-T)
- `HideContent`
- `SelectiveHide`
- `Quarantine`
- `DOE`
- `PropagateWithTool`
- hidden content / variable graph / violations

**系统角色**
- **IFC-Enforcement**：P-F / P-T
- **IFC-Transform**：Hide / SelectiveHide
- **IFC-Route**：Quarantine
- **IFC-Analysis**：DOE / propagation / hidden-content audit

**当前问题**
- 先 register 再 check
- 检查全 trace vars，不是本次 action input objects
- `contextLabel` 能力没被主链真正用起来
- Quarantine 还是 ifc 内部副作用，不是 first-class action

**融合建议**
- IFC 必须分层保留，不能整体并进统一 verdict

### 2.7a URL / API Source Classification（建议新增能力）

**问题背景**
当前 IFC / Capability 对外部数据来源的分级仍偏粗：
- `tool:web_fetch`
- `tool:http_request`
- `tool:mcp_tool`

这意味着：
- 公开文档站
- 带 token 的外部 SaaS API
- 公司内网 API
- metadata service / admin endpoint

在当前模型里仍然容易被归到过于接近的工具来源上。

**建议新增组件**
- `ToolSourceClassifier`
- 从 `toolName + toolArgs` 中抽取：
  - URL
  - host
  - path
  - method
  - auth type
  - private network / metadata 特征

**输出统一 `SourceDescriptor`**
- `SourceKey`
- `Category`（public_web / external_api / internal_api / admin_api / metadata_service / unknown）
- `Confidentiality`
- `Integrity`
- `TrustScore`
- `Tags`

**可直接增强的现有能力**
1. **IFC**：不再只按 `tool:web_fetch` 粗分，而能按 URL/API 语义给标签
2. **Capability**：provenance source 更细，lineage 解释力更强
3. **PathPolicy**：不同 endpoint 类型可以贡献不同 risk delta
4. **审计/解释**：能回答“为什么这个 http_request 的结果被视为 CONFIDENTIAL/SECRET”

**落地原则**
- 先 `audit-only`，只记录分类结果，不立刻改变拦截行为
- 然后喂给 IFC label enrichment
- 再喂给 Capability provenance
- 最后再联动更高层治理

---

## 2.8 TaintTracker + TaintReversal

**代码位置**
- `src/taint_tracker.go`
- `src/taint_reversal.go`
- `src/llm_response_postprocess.go`
- `src/llm_proxy.go`

**主要能力**
- taint labels
- request pre-inject
- response reversal
- soft / hard / stealth

**系统角色**
- signal engine
- transform engine

**融合建议**
- 保留在 pre/post process layer
- 不进入 final decision combiner

---

## 2.9 LLM Rule Engine (Request/Response)

**代码位置**
- `src/llm_request_policy.go`
- `src/llm_response_policy.go`

**主要能力**
- block / warn / log
- rewrite
- auto-review
- singularity expose

**系统角色**
- verdict
- transform
- route/replace

**融合建议**
- 如果未来统一扩到内容治理层，也必须按多语义分层，而不是压成字符串 decision

---

# 3. 当前最大的结构性问题：Preview 和 Commit 没分开

这比“多个引擎怎么合并”更严重。

## 3.1 现状
多个模块在 final decision 之前就已经改状态：

- PathPolicy：`RegisterStep` 在前
- PlanCompiler：`EvaluateToolCall` 内推进 step
- Capability：先注册对象后评估
- IFC：先注册变量后检查

## 3.2 后果
会导致：

1. 被 block 的动作像是执行过
2. preview 影响后续评估
3. 重复评估会重复推进状态
4. 审计和真实执行可能不一致

## 3.3 必须引入的链路
未来统一治理的主链应该明确分成：

```text
Preview / Propose
    -> Combine
    -> Execute
    -> Commit
```

这是融合前的必要前置条件。

---

# 4. 推荐的融合治理五层架构

## Layer 1: Evidence Layer
统一证据与对象层：
- capability objects
- ifc variables
- taint labels
- path contexts
- attribution reports
- hidden-content records
- quarantine sessions

职责：
- lineage
- provenance
- explainability
- audit

---

## Layer 2: Proposal Layer
每个引擎只产 proposal：
- decision proposal
- transform proposal
- route proposal
- signal proposal

---

## Layer 3: Combine Layer
只做统一 outcome：
- final decision
- route action
- applied transforms
- reasons
- supporting signals

---

## Layer 4: Execute Layer
真正执行：
- allow / block
- quarantine / isolate / expose
- rewrite / hide / redact / reversal
- replace_tool / sanitize_args

---

## Layer 5: Commit Layer
只在最终动作明确后提交状态：
- path step
- plan step
- capability object
- ifc variable
- deviation log
- unified audit record

---

# 5. Preview / Commit 改造计划（关键底座）

## 5.1 PathPolicyEngine

### 目标
从：
- `RegisterStep()`
- `Evaluate()`

改成：
- `Preview(traceID, step)`
- `CommitStep(traceID, step)`

### 要求
- preview 不能写 committed path
- final allow/repaired-allow 后才 commit
- `isolate` 保留为 route-like proposal

---

## 5.2 PlanCompiler

### 目标
新增：
- `PreviewToolCall(traceID, toolName, toolArgs)`
- `CommitToolCall(traceID, toolName, toolArgs, result)`
- `CommitViolation(...)`

### 要求
- preview 不推进 `CurrentStep`
- preview 不 append `ExecutedSteps`
- 只有 final action 确认后才推进状态
- 解决主链与 Deviation 重复评估问题

---

## 5.3 CapabilityEngine

### 目标
新增：
- `PreviewToolResult(...)`
- `PreviewEvaluate(...)`
- `CommitObject(...)`

### 要求
- preview object 不能污染 `ctx.DataItems`
- object 的 `sources / parents / trust_score / labels` 全量保留
- committed object 只能在最终执行后落账

---

## 5.4 IFCEngine

### 目标
新增：
- `PreviewCheckToolCall(...)`
- `PreviewToolOutput(...)`
- `CommitVariable(...)`

### 要求
- `CheckToolCallFides` 先 preview，不落变量
- 输入必须来自“本次 action 的 input object IDs”
- Quarantine 保留为 route action
- `HideContent` / `SelectiveHide` 保留为 transform
- DOE 保留为 signal

---

## 5.5 DeviationDetector

### 目标
新增：
- `Preview(...)`
- `ApplyRepair(...)`
- `CommitDeviation(...)`

### 要求
- repair proposal 和 decision proposal 分离
- `replace_tool` / `sanitize_args` 必须保留
- preview 不能直接写 DB

---

# 6. 统一 Proposal / Outcome Schema 设计

统一治理中，所有引擎应使用统一 schema 交流，而不是直接互相看私有结构体。

## 6.1 GovernanceInput

```go
type GovernanceInput struct {
    TraceID        string
    TenantID       string
    SenderID       string
    Stage          string
    Action         string
    RawArgs        string
    InputObjectIDs []string
    Metadata       map[string]interface{}
}
```

### URL/API Source Descriptor（建议新增并挂到 GovernanceInput.Metadata）

对于带 URL / endpoint 语义的 tool call，建议统一产出：

```go
type SourceDescriptor struct {
    SourceKey       string
    BaseTool        string
    URL             string
    Host            string
    Path            string
    Method          string
    Category        string // public_web / external_api / internal_api / admin_api / metadata_service / unknown
    Confidentiality IFCLevel
    Integrity       IntegLevel
    TrustScore      float64
    AuthType        string
    PrivateNetwork  bool
    Suspicious      bool
    Tags            []string
    Evidence        []string
}
```

它的作用不是替代 IFC/Capability，而是作为更细粒度来源分类结果，统一供：
- IFC label enrichment
- Capability provenance
- Path risk weighting
- 审计 / explainability

---

## 6.2 DecisionProposal

```go
type GovernanceDecisionProposal struct {
    Engine         string
    Kind           string
    ProposedAction string
    Severity       string
    Reason         string
    Confidence     float64
    EvidenceRefs   []string
    Metadata       map[string]interface{}
}
```

用于：
- ToolPolicy
- PathPolicy
- Plan
- Capability
- IFC-Enforcement
- Counterfactual(sync)
- 部分 Deviation

---

## 6.3 TransformProposal

```go
type GovernanceTransformProposal struct {
    Engine        string
    Action        string
    Target        string
    Reason        string
    SafeByDefault bool
    EvidenceRefs  []string
    Payload       map[string]interface{}
}
```

用于：
- rewrite
- redact
- selective_hide
- sanitize_args
- replace_tool
- taint reversal

---

## 6.4 RouteProposal

```go
type GovernanceRouteProposal struct {
    Engine       string
    Route        string
    Reason       string
    Target       string
    EvidenceRefs []string
    Metadata     map[string]interface{}
}
```

用于：
- quarantine
- isolate
- expose
- alternate_upstream

---

## 6.5 SignalProposal

```go
type GovernanceSignalProposal struct {
    Engine       string
    Signal       string
    Severity     string
    Reason       string
    Value        interface{}
    EvidenceRefs []string
    Metadata     map[string]interface{}
}
```

用于：
- path risk
- DOE
- taint present
- hidden content generated
- async attribution
- trust score

---

## 6.6 Proposal Bundle

```go
type GovernanceProposalBundle struct {
    Input      GovernanceInput
    Decisions  []GovernanceDecisionProposal
    Transforms []GovernanceTransformProposal
    Routes     []GovernanceRouteProposal
    Signals    []GovernanceSignalProposal
}
```

---

## 6.7 GovernanceOutcome

```go
type GovernanceOutcome struct {
    FinalDecision    string
    RouteAction      string
    AppliedTransforms []GovernanceTransformProposal
    WinningReasons   []GovernanceDecisionProposal
    SupportingSignals []GovernanceSignalProposal
    AuditRefs        []string
    Explain          GovernanceExplanation
}
```

---

## 6.8 GovernanceExplanation

```go
type GovernanceExplanation struct {
    Summary               string
    DecisionPath          []string
    MissingCaps           []string
    ViolatedFlows         []string
    SelectedRoute         string
    AppliedTransformNames []string
}
```

---

# 7. 各引擎映射到统一 Schema 的方式

## ToolPolicy
- 输出 `DecisionProposal`

## PathPolicy
- 输出 `DecisionProposal`
- 同时输出 `SignalProposal(risk_score)`
- `isolate` 最终应成为 route candidate

## PlanCompiler
- 输出 `DecisionProposal(plan_violation)`
- state commit 后移

## Deviation
- 输出 `TransformProposal(replace_tool / sanitize_args)`
- 未修复时再输出 `DecisionProposal`

## Capability
- 输出 `DecisionProposal`
- 同时输出 `SignalProposal(trust_score / lineage)`
- object/provenance 进入 evidence layer
- 若存在 `SourceDescriptor`，则 source 不再只记录 `tool:web_fetch` 这类粗粒度工具源，而是记录细粒度来源类别（如 `tool:http_request:internal_api` / `tool:web_fetch:public_web`）

## IFC
- Enforcement → `DecisionProposal`
- Hide / SelectiveHide → `TransformProposal`
- Quarantine → `RouteProposal`
- DOE / propagation → `SignalProposal`
- 若存在 `SourceDescriptor`，则优先使用 URL/API 分类结果给 output variable 赋更精细的 conf/integ，而不是只按工具名默认标签

## Counterfactual
- sync → `DecisionProposal`
- async → `SignalProposal`

## TaintReversal
- 只输出 `TransformProposal`

## LLM Rule Engine
- block/warn → `DecisionProposal`
- rewrite → `TransformProposal`
- singularity expose → `RouteProposal` 或 route-like replace action

---

# 8. 推荐的统一语义优先级

## 8.1 Decision precedence

```text
block > deny > quarantine > isolate > review > warn > allow
```

说明：
- `deny` 是 proposal 语义，可在 combiner 中映射到 block
- `quarantine/isolate` 虽然更像 route，但优先级必须高于 warn/allow

## 8.2 Route precedence

```text
expose > quarantine > isolate > alternate_upstream > normal
```

## 8.3 Transform ordering

```text
sanitize_args / replace_tool
    -> hide / selective_hide / redact
    -> rewrite
    -> taint reversal
```

---

# 9. 最小可实施落地顺序

## Phase A0 — URL/API Source Classification（audit-only）
先新增：
- `ToolSourceClassifier`
- `SourceDescriptor`
- 在 `llm_proxy` tool-call 治理阶段记录 host/path/auth/category/conf/integ/trust

目标：
- 不改变现有 block/warn 行为
- 先验证真实流量中的 URL/API 分类是否合理
- 为 IFC / Capability / Path risk 提供更细粒度来源语义

---

## Phase A — 不改业务行为，只补 preview/commit 接口
先补：
- Path preview/commit
- Plan preview/commit
- Capability preview/commit
- IFC preview/commit
- Deviation preview/commit

目标：
- 解决“先写状态，后裁决”

---

## Phase B — 加 proposal adapter
为各引擎加 adapter：
- `ToDecisionProposal()`
- `ToTransformProposal()`
- `ToRouteProposal()`
- `ToSignalProposal()`

目标：
- 让主链先能收 bundle，但暂不改 final behavior

---

## Phase C — shadow combiner
引入 unified combiner，但先 shadow mode：
- 生成 outcome
- 与旧行为对比
- 记录差异

目标：
- 不立刻切主路
- 先验证语义是否一致

---

## Phase D — commit after outcome
最终切换到：
- 先 proposal
- 再 combine
- 再 execute
- 再 commit

目标：
- 统一治理真正落地

---

# 10. 绝不能丢的兼容性要求

统一过程中必须保证这些现有能力完整保留：

1. IFC 全量能力
2. Deviation auto-repair
3. SelectiveHide
4. Quarantine
5. Counterfactual async
6. TaintReversal soft/hard/stealth
7. LLM rewrite
8. Singularity expose
9. PathPolicy isolate
10. PlanCompiler 状态机语义
11. Capability provenance object 模型
12. URL/API source classification 在 audit-only→label enrichment→provenance enrichment 的渐进演进路径，不得一次性硬切导致误判

---

# 11. 当前最合理的下一步

在真正重新把这件事挂回 roadmap 之前，最合理的工程顺序是：

### 第一步
先做 **Preview / Commit 改造**

### 第二步
再把 schema 落成代码：
- `src/governance_types.go`
- `src/governance_combiner.go`

### 第三步
shadow mode 对照现有行为

### 第四步
再决定是否恢复到正式 roadmap

---

# 12. 最终判断

> lobster-guard 现在不是“多个规则引擎”，而是“多个不同语义层的治理能力堆叠”。
> 要做融合，不能先做统一 block/warn，而要先做统一治理语义、统一状态边界、统一 proposal schema。

这就是当前最稳、也最不伤现有能力的演进路径。
