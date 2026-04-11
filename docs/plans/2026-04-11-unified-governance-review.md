# Unified Governance Engine System Review & Fusion Plan

> **For Hermes:** 这不是立即实现计划，而是 v37 之前的系统性梳理文档。目标是先把现有治理引擎的语义边界、状态提交时机、互相耦合关系和融合约束讲清楚，再决定实现顺序。

**Goal:** 在尽可能保留现有特性的前提下，为 lobster-guard 设计一套可渐进落地的融合治理引擎（Unified Governance Engine），避免统一过程中丢失 IFC / Capability / Path / Plan / Deviation / Counterfactual / Taint / Rewrite 等已有能力。

**Architecture:** 现有系统不是“多个 block/warn 引擎并列”，而是混合了决策器、修复器、分流器、变换器、证据层和状态机。融合的关键不是做一个简单的 `combine(decisions)`，而是先把所有高级引擎拆成统一的语义层：**Evidence / Proposal / Combine / Execute / Commit**。只有这样，才能在不牺牲现有能力的情况下完成统一。

**Tech Stack:** Go, SQLite, LLMProxy pipeline, PathPolicyEngine, PlanCompiler, DeviationDetector, CapabilityEngine, IFCEngine, CounterfactualVerifier, TaintTracker, TaintReversalEngine.

---

## 1. Executive Summary

当前 lobster-guard 的高级治理能力已经非常强，但它们的运行语义并不一致。统一时最容易犯的错，是把所有引擎都压成同一类东西：

- allow / warn / block

如果这么做，会直接损坏多类现有能力：

- IFC 的 `SelectiveHide` / `HideContent` / `Quarantine`
- Deviation 的 `replace_tool` / `sanitize_args`
- LLM rule engine 的 `rewrite`
- TaintReversal 的 `soft/hard/stealth`
- PathPolicy 的 `isolate`
- Counterfactual 的 async 模式

所以统一治理引擎必须满足两个原则：

1. **现有能力不降级**：不是删除复杂性，而是给复杂性明确分层。
2. **状态提交延后**：先 proposal，再 combine，再 execute，最后 commit。不能在最终裁决前就写 execution state。

---

## 2. Current Engines and Their Real Semantics

下面不是按“模块名”分类，而是按“实际系统角色”分类。

### 2.1 ToolPolicyEngine

**代码位置**
- `src/tool_policy.go`
- `src/llm_tool_governance.go`

**当前能力**
- 工具名模式匹配（shell / execute / write_file / sudo 等）
- 参数正则（敏感路径、curl|bash、rm -rf、SQL 注入等）
- rate limit
- 事件审计

**实际语义**
- 主要是 **verdict engine**
- 但目前在 `llm_tool_governance.go` 中已经和 PathPolicy 混合裁决
- tenant blacklist 语义也不是显式 final block，而是“continue/skip”式短路

**风险**
- 统一后容易被视为“最标准引擎”，进而成为错误模板，把其他复杂引擎也强行压成 ToolPolicy 的样子。

---

### 2.2 PathPolicyEngine

**代码位置**
- `src/path_policy.go`
- `src/llm_tool_governance.go`

**当前能力**
- PathContext（steps / taint_labels / tool_history / risk_score）
- sequence / cumulative / degradation 三类规则
- risk score 衰减
- 输出 `allow / warn / block / isolate`

**实际语义**
- 不是纯 verdict engine
- 它同时是：
  - **state engine**（维护路径上下文）
  - **risk engine**（累积风险）
  - **verdict proposal engine**
  - **route-like engine**（`isolate`）

**当前实现问题**
- 在 `llm_tool_governance.go` 中，先 `RegisterStep`，后 `Evaluate`
- 这意味着 proposed tool_call 在 final verdict 之前就进入了历史
- `isolate` 已有语义，但当前执行层没有 first-class 支持

**融合要求**
- Path state 不能丢
- `isolate` 不能丢
- 需要把 `RegisterStep` 拆成 preview/commit 语义

---

### 2.3 CounterfactualVerifier

**代码位置**
- `src/counterfactual.go`
- `src/llm_tool_governance.go`

**当前能力**
- 高风险 tool 反事实验证
- 同步 / 异步两种模式
- budget 控制
- cache
- attribution report
- envelope / user profile 联动

**实际语义**
- sync 模式：**verdict engine**
- async 模式：**signal / evidence engine**

**融合风险**
- 如果统一治理要求“每个引擎每次都给 final decision”，会破坏 async 模式
- 反事实不是必须同步发生的，它本来就是“昂贵但高价值”的检查

**融合要求**
- sync 模式可进入 final combiner
- async 模式只能作为 evidence/signal，不能被强制同步化

---

### 2.4 PlanCompiler

**代码位置**
- `src/plan_compiler.go`
- `src/llm_deep_governance.go`

**当前能力**
- CompileIntent
- ActivePlan 状态机
- EvaluateToolCall
- ExecutedSteps / Violations / CurrentStep / CompletePlan

**实际语义**
- 不是纯评估器，而是 **state machine + verdict engine**

**当前实现问题**
- `EvaluateToolCall()` 内部会推进状态：
  - append ExecutedSteps
  - 推进 CurrentStep
  - persist plan / violation
- `llm_proxy.go` 中 PlanCompiler 被调用一次
- `DeviationDetector.Detect()` 中又会再次调用 PlanCompiler
- 这意味着同一个 tool_call 存在重复评估、重复推进状态的风险

**融合要求**
- 必须拆出：
  - `PreviewToolCall(...)`：不改状态，只产 proposal
  - `CommitToolCall(...)`：最终 allow 后才推进状态
- 否则统一治理后状态漂移会更严重

---

### 2.5 DeviationDetector

**代码位置**
- `src/plan_deviation.go`
- `src/llm_deviation_governance.go`

**当前能力**
- 检测 plan deviation
- 检测 capability violation（但当前判断枚举存在问题）
- auto-repair
- repair policy：`replace_tool` / `sanitize_args` / `block` / `skip` / `log`

**实际语义**
- 同时是：
  - **verdict engine**
  - **transform/repair engine**
  - **audit engine**

**当前实现问题**
- capability 检查使用 `capEval.Decision == "block"`
- 但 CapabilityEngine 返回的是 `allow / warn / deny`
- 这说明当前 capability→deviation 链已经存在语义错位

**融合要求**
- repair 不能丢
- 统一后应拆成：
  - `repair proposal`
  - `deviation verdict proposal`
- 不能把 DeviationDetector 简化成“偏差=warn/block”

---

### 2.6 CapabilityEngine

**代码位置**
- `src/capability.go`
- `src/llm_deep_governance.go`

**当前能力**
- CapContext
- DataItems
- RegisterToolResult
- PropagateData
- RegisterLLMSummary
- Evaluate / EvaluateWithProvenance
- tool mappings
- trust score / sources / parent_ids

**实际语义**
- 不是简单 verdict engine
- 它更像 **provenance-aware authorization substrate**
- 同时包含：
  - object layer
  - propagation layer
  - evaluation layer

**当前实现问题**
- 主链上更多是 log / event / audit，不是 hard enforcement
- 在 LLM 主链上先 RegisterToolResult，再 Evaluate
- 在 DeviationDetector 里又被用错枚举

**融合要求**
- Capability 不应该被压缩成单一 `allow/deny` 函数
- 它应该成为 unified evidence/object layer 的核心之一
- 评估结果则进入 decision proposal

---

### 2.7 IFCEngine

**代码位置**
- `src/ifc_engine.go`
- `src/ifc_quarantine.go`
- `src/llm_ifc_governance.go`
- `src/llm_proxy.go`

**当前能力**
- 双标签系统（conf + integ）
- `CheckToolCallFides`（P-F / P-T）
- `HideContent`
- `SelectiveHide`
- `ShouldQuarantine / Route / CompleteSession`
- `DetectDOE`
- `PropagateWithTool`
- variable graph / hidden-content persistence / violation audit

**实际语义**
- IFC 不是一个引擎，而是四类能力的集合：
  - **IFC-Enforcement**：P-F / P-T
  - **IFC-Transform**：Hide / SelectiveHide
  - **IFC-Route**：Quarantine
  - **IFC-Analysis**：DOE / propagation / hidden-content audit

**当前实现问题**
- `evaluateIFCForTool()` 先 `RegisterVariable()`，后 `CheckToolCall()`
- 不是检查“本次 action 的真实输入对象”，而是粗暴地取 trace 下所有 vars
- `CheckToolCallFides` 明明支持 `contextLabel`，代理层却未真正传入上下文标签
- Quarantine 目前是 ifc 内部副作用，不是 first-class final action

**融合要求**
- 不能整体并进统一 verdict
- 必须拆层保留所有现有能力

---

### 2.8 TaintTracker + TaintReversal

**代码位置**
- `src/taint_tracker.go`
- `src/taint_reversal.go`
- `src/llm_response_postprocess.go`
- `src/llm_proxy.go`

**当前能力**
- taint labels
- request pre-inject
- response reversal
- soft / hard / stealth 三模式

**实际语义**
- 主要是：
  - **signal engine**
  - **transform engine**

**融合要求**
- 不应进入 final decision combiner
- 应保留在 pre/post process layer

---

### 2.9 LLM Rule Engine (Request/Response)

**代码位置**
- `src/llm_request_policy.go`
- `src/llm_response_policy.go`

**当前能力**
- request/response rule matching
- block / warn / log
- rewrite
- auto-review
- singularity expose（直接返回诱饵内容）

**实际语义**
- 混合了：
  - verdict
  - transform
  - route/replace

**融合要求**
- 如果未来统一治理扩到内容侧，也不能把这些动作全部压成 block/warn

---

## 3. The Core Problem: Proposed vs Executed Are Not Separated

这是当前系统统一前必须先承认的核心问题。

### 现状
多个引擎在 final decision 之前就开始写 execution state：

- PathPolicy：先 `RegisterStep`，后 `Evaluate`
- PlanCompiler：`EvaluateToolCall` 内就 append executed steps / move current step
- Capability：先 `RegisterToolResult` / `PropagateData`，后 `Evaluate`
- IFC：先 `RegisterVariable`，后 `CheckToolCall`

### 结果
当前系统里已经存在这些潜在偏差：

1. **被 block 的 proposed action 可能已写进执行历史**
2. **同一 tool_call 可能被多个引擎重复落账**
3. **某些引擎的上下文会因为 preview 行为而污染后续判断**

### 结论
在融合引擎之前，必须先把治理链显式分成：

1. **Preview / Propose**
2. **Combine**
3. **Execute**
4. **Commit**

---

## 4. Recommended Fusion Architecture

### 4.1 Five-Layer Architecture

#### Layer 1: Evidence Layer
统一证据与对象层，不做决策。

包含：
- Capability data objects
- IFC variables and labels
- Taint labels
- Path contexts
- Counterfactual attribution reports
- Hidden-content records
- Quarantine sessions

职责：
- 存证
- lineage
- provenance
- explainability

#### Layer 2: Proposal Layer
每个引擎只产 proposal，不直接写 final state。

proposal 类型：
- **decision proposal**：allow/warn/block/deny/review/isolate/quarantine
- **transform proposal**：rewrite/sanitize/replace/hide/redact/reverse
- **route proposal**：quarantine/expose/isolate
- **signal proposal**：risk / attribution / doe / taint / evidence

#### Layer 3: Combine Layer
只做统一裁决，不做内容改写。

输出至少包括：
- `final_decision`
- `route_action`
- `ordered_transforms`
- `supporting_reasons`

注意：
- transform proposal 不等于 final decision
- route proposal 不等于 block

#### Layer 4: Execute Layer
真正执行：
- block response
- allow forward
- quarantine route
- apply rewrite / sanitize / selective hide / reversal / singularity expose
- apply deviation repair

#### Layer 5: Commit Layer
只在最终动作明确后提交状态：
- commit path step
- commit plan step
- commit capability object
- commit ifc variable
- persist final decision evidence

---

## 5. Recommended Semantic Taxonomy

为了尽可能保留现有特性，统一时不要强行用单一 verdict 词表。

### 5.1 Final decisions
- `allow`
- `warn`
- `review`
- `block`

### 5.2 Route actions
- `quarantine`
- `isolate`
- `expose`
- `normal`

### 5.3 Transform actions
- `rewrite`
- `redact`
- `selective_hide`
- `sanitize_args`
- `replace_tool`
- `taint_reverse_soft`
- `taint_reverse_hard`
- `taint_reverse_stealth`

### 5.4 Signals
- `path_risk_high`
- `attribution_injection_driven`
- `ifc_doe_warning`
- `capability_untrusted_lineage`
- `taint_present`

---

## 6. Engine-by-Engine Mapping into the Fusion Architecture

### ToolPolicy
- Proposal: decision
- Commit: event/audit only after final action
- Notes: tenant blacklist 应改为显式 proposal，不要 continue/skip 静默短路

### PathPolicy
- Evidence: path context + risk score
- Proposal: decision (`warn/block/isolate`)
- Commit: path step must happen after final action
- Notes: `isolate` 要升格为 route action

### Counterfactual
- Evidence: attribution report
- Proposal: sync mode → decision proposal；async mode → signal proposal
- Notes: budget/cache 逻辑保持现状，不强制同步化

### PlanCompiler
- Evidence: active plan state
- Proposal: preview evaluation
- Commit: only after allow/repair success
- Notes: 必须拆 PreviewToolCall / CommitToolCall

### DeviationDetector
- Proposal: decision + transform(repair)
- Execute: apply repaired tool/args
- Commit: only after final chosen action
- Notes: 不能丢 `replace_tool` / `sanitize_args`

### Capability
- Evidence: provenance objects / sources / trust / labels
- Proposal: decision
- Commit: data objects must be committed after execution, not before
- Notes: 修复 `deny` vs `block` 枚举不一致

### IFC
- Evidence: variables / labels / hidden-content / quarantine sessions
- Proposal:
  - Enforcement → decision
  - Transform → transform
  - Route → route
  - Analysis → signal
- Commit: output vars and hidden records after final route/transform
- Notes: 检查输入对象不能再用全 trace 聚合

### Taint / Reversal
- Evidence: taint labels / reversal records
- Proposal: transform only
- Execute: pre/post process layer
- Notes: 不进入 final decision combiner

### LLM Request/Response Rules
- Proposal: decision + transform + route/replace
- Execute: request/response layer
- Notes: singularity expose 不是 block，是替代响应/route-like action

---

## 7. Minimum Refactors Required Before Any Real Unification

### Refactor A: Preview / Commit split
必须优先做：
- `PathPolicyEngine.RegisterStepPreview / CommitStep`
- `PlanCompiler.PreviewToolCall / CommitToolCall`
- `CapabilityEngine.PreviewObject / CommitObject`
- `IFCEngine.PreviewCheck / CommitVariable`

### Refactor B: Unified proposal structures
新增统一结构：

```go
type GovernanceDecisionProposal struct {
    Engine      string
    Decision    string
    Severity    string
    Reason      string
    EvidenceRef []string
}

type GovernanceTransformProposal struct {
    Engine      string
    Action      string
    Target      string
    Reason      string
    Payload     map[string]any
}

type GovernanceRouteProposal struct {
    Engine      string
    Route       string
    Reason      string
    Target      string
}
```

### Refactor C: Unified final result

```go
type GovernanceOutcome struct {
    FinalDecision string
    RouteAction   string
    Transforms    []GovernanceTransformProposal
    Reasons       []GovernanceDecisionProposal
    Signals       []string
}
```

### Refactor D: Stop double evaluation
优先修复：
- PlanCompiler 在主链 + DeviationDetector 双重评估
- Capability 在多个路径上提前注册对象
- IFC 在 check 前先注册结果变量

---

## 8. Recommended Incremental Rollout

### Phase A — System review hardening (no semantic change)
目标：不改行为，只把 preview/commit 边界标出来。

具体：
- 给 Plan / Path / Capability / IFC 引入 preview 接口
- 保持旧接口兼容
- 增加测试证明 preview 不提交状态

### Phase B — Proposal bus
目标：让各引擎开始输出 proposal，而不是直接 HTTP 403 / 直接 mutate state。

具体：
- ToolPolicy / Path / Capability / IFC-Enforcement / Counterfactual(sync) 先接 decision proposal
- Deviation / IFC-Transform / TaintReversal 先接 transform proposal
- IFC-Route / Path isolate / Singularity expose 先接 route proposal

### Phase C — Combiner
目标：接统一 outcome，但不立刻删旧逻辑。

具体：
- 先 shadow mode：生成 unified outcome，与旧行为比对
- 审计差异
- 等差异足够小，再切主路

### Phase D — Commit semantics
目标：真正把执行状态写入推迟到 final action 后。

---

## 9. Non-Negotiable Compatibility Requirements

融合设计必须满足以下刚性要求：

1. **IFC 现有能力全部继承**
2. **Deviation repair 不丢**
3. **SelectiveHide 不退化成 warn/block**
4. **Quarantine 保持 first-class route action**
5. **Counterfactual async 模式继续存在**
6. **TaintReversal soft/hard/stealth 全保留**
7. **LLM rewrite / singularity expose 继续存在**
8. **PathPolicy 的 risk state 与 isolate 语义不能丢**
9. **PlanCompiler 保持状态机语义，但必须拆 preview/commit**
10. **Capability 的 provenance object 模型不能被压扁成单次函数调用**

---

## 10. Final Recommendation

**不要直接开做“统一裁决器”。**

正确顺序应是：

1. **先承认系统里存在 verdict / transform / route / signal / state 五种语义**
2. **先修 preview/commit 边界**
3. **先消灭重复评估和提前落账**
4. **再引入 proposal bus**
5. **最后才做 unified combiner**

一句话总结：

> lobster-guard 现在不是“多个规则引擎”，而是“多个不同语义层的治理能力堆叠”。
> 要融合，不能先做统一 block/warn，而要先做统一治理语义。

---

## 11. Suggested Next Documents

在真正排 v37 之前，建议再补两份文档：

1. `docs/plans/2026-04-11-unified-governance-preview-commit-plan.md`
   - 只讲 Preview / Commit 改造

2. `docs/plans/2026-04-11-governance-proposal-schema.md`
   - 只讲 proposal / outcome 数据结构

这两份完成后，再决定是否重新把统一治理挂回 ROADMAP。
