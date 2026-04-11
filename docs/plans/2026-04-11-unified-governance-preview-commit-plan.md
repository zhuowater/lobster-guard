# Unified Governance Preview/Commit Refactor Plan

> **For Hermes:** 这份文档只解决一个问题：把“预览评估”和“执行提交”拆开。它不是统一治理的全量方案，而是实现融合前必须先完成的底座改造。

**Goal:** 为 lobster-guard 的高级治理链引入清晰的 Preview / Execute / Commit 边界，消除当前多个引擎“先写状态、后裁决”的语义混乱，在尽可能保留现有特性的前提下，为后续统一治理铺平道路。

**Architecture:** 以“不改现有业务能力、先加兼容层”为原则，为 PathPolicy / PlanCompiler / Capability / IFC 先补 Preview 接口和 Commit 接口；旧接口暂时保留，主链逐步切换到新的 preview-first 流程。短期目标不是改变 block/warn 行为，而是保证**没有最终放行的动作不再污染执行状态**。

**Tech Stack:** Go, LLMProxy pipeline, PathPolicyEngine, PlanCompiler, CapabilityEngine, IFCEngine, SQLite.

---

## 1. Why This Refactor Must Happen First

当前高级治理链最大的结构性问题不是缺一个 combiner，而是：

- PathPolicy 在 final verdict 前就 `RegisterStep`
- PlanCompiler 在 `EvaluateToolCall` 内就推进 `CurrentStep` 并写 `ExecutedSteps`
- Capability 在 `Evaluate` 前就 `RegisterToolResult`
- IFC 在 `CheckToolCall` 前就 `RegisterVariable`

这会导致：

1. 被 block 的 proposed action 进入执行历史
2. 不同引擎互相读取到“未来状态”
3. 统一治理后更难解释“这个结果到底是真执行了还是只是 proposed”
4. 审计层可能记录“看起来执行过”的动作

因此，统一治理之前必须先把状态分为两类：

- **Preview State**：只为评估服务，不算真实执行
- **Committed State**：最终动作确认后才写入

---

## 2. Global Target Model

未来的 tool-call 治理链应该长这样：

```text
Parse tool_call
  -> Build preview context
  -> Path preview
  -> Plan preview
  -> Capability preview
  -> IFC preview
  -> Deviation preview / repair preview
  -> Combine outcome
  -> Execute (allow/block/quarantine/rewrite/repair)
  -> Commit final state
```

### 核心原则

1. **Preview 绝不写 committed state**
2. **Commit 只发生一次**
3. **Commit 只能在 final action 明确后触发**
4. **Preview 返回的数据必须足够支持 explainability**
5. **旧接口短期保留，但主链逐步迁移到新接口**

---

## 3. Module-by-Module Refactor Scope

## 3.1 PathPolicyEngine

### 当前问题

当前调用链：
- `llm_tool_governance.go`
  - `RegisterStep(traceID, PathStep{Stage:"tool_call", Action: tcName, Details: tcArgs})`
  - `Evaluate(traceID, tcName)`

这意味着 proposed action 已经写入 path history。

### 目标拆分

新增两个层次：

#### A. PreviewStep
只构造 `PathStep`，不落 context。

```go
type PathStepPreview struct {
    Step PathStep
}
```

#### B. PreviewEvaluate
基于当前 committed path context + preview step 评估，不写状态。

```go
func (e *PathPolicyEngine) Preview(traceID string, step PathStep) PathDecision
```

#### C. CommitStep
只有最终动作为 allow / quarantine-forward / repaired-allow 等可提交状态时才调用。

```go
func (e *PathPolicyEngine) CommitStep(traceID string, step PathStep)
```

### 兼容策略

保留旧接口：
- `RegisterStep()`
- `Evaluate()`

但在文档中标记为 legacy。

### 主链迁移目标

旧：
```go
RegisterStep(...)
Evaluate(...)
```

新：
```go
preview := PathStep{...}
ppDec := Preview(traceID, preview)
// combine
if shouldCommit(finalOutcome) {
    CommitStep(traceID, preview)
}
```

---

## 3.2 PlanCompiler

### 当前问题

`EvaluateToolCall()` 同时做：
- tool step 匹配
- append `ExecutedSteps`
- move `CurrentStep`
- persist violations
- auto complete

而且 DeviationDetector 还会再次调用它。

### 必须拆分的接口

#### A. PreviewToolCall
不改 active plan，只返回“如果执行会发生什么”。

```go
type PlanPreview struct {
    Allowed      bool
    Decision     string
    ToolName     string
    StepMatch    string
    Reason       string
    Violation    *PlanViolation
    PlanID       string
    PlanStatus   string
    NextStep     int
    WouldComplete bool
}

func (pc *PlanCompiler) PreviewToolCall(traceID, toolName, toolArgs string) *PlanPreview
```

#### B. CommitToolCall
只有在 final allow / repaired-allow 后才真正推进状态。

```go
func (pc *PlanCompiler) CommitToolCall(traceID, toolName, toolArgs, result string) error
```

#### C. CommitViolation
如果 final outcome 是 block/review/warn，按实际 final result 写 violation log，而不是 preview 期间立刻写。

```go
func (pc *PlanCompiler) CommitViolation(traceID string, preview *PlanPreview, finalDecision string) error
```

### 特别注意

`DeviationDetector` 不能再直接调老的 `EvaluateToolCall()`。否则会出现：
- 主链 preview 一次
- deviation 再 preview/commit 一次

### 迁移策略

第一阶段：
- 保留 `EvaluateToolCall()`
- 内部改成：调用 `PreviewToolCall()` + `CommitToolCall()`

这样可以保证旧调用方不立刻坏掉。

第二阶段：
- 主链和 Deviation 全部改成显式 Preview / Commit

---

## 3.3 CapabilityEngine

### 当前问题

主链中现在是：
- `RegisterToolResult(traceID, toolName, dataID)`
- `PropagateData(...)`
- `EvaluateWithProvenance(...)`

问题在于：
- tool 还没最终放行，data object 已经进入上下文

### 目标拆分

#### A. PreviewToolResult
构造一个未提交的 capability object，不写入 `ctx.DataItems`。

```go
type CapabilityPreviewObject struct {
    DataID      string
    Source      string
    Sources     []string
    ParentIDs   []string
    Labels      []CapLabel
    TrustScore  float64
}

func (ce *CapabilityEngine) PreviewToolResult(traceID, toolName, dataID string, parentIDs []string) *CapabilityPreviewObject
```

#### B. PreviewEvaluate
基于 preview object 做 capability evaluation。

```go
func (ce *CapabilityEngine) PreviewEvaluate(traceID string, obj *CapabilityPreviewObject, action, toolName string) *CapEvaluation
```

#### C. CommitObject
只有最终执行后，才真正写入 context 和 DB。

```go
func (ce *CapabilityEngine) CommitObject(traceID string, obj *CapabilityPreviewObject) error
```

### lineage 要求

preview object 必须完整保留：
- `sources`
- `parent_ids`
- `trust_score`
- `labels`

因为后续 unified explainability 会依赖这些数据。

---

## 3.4 IFCEngine

### 当前问题

主链中现在是：
- `RegisterVariable(traceID, "tool_result_"+tool, toolSource, args)`
- `collectIFCVarIDs(GetVariables(traceID))`
- `CheckToolCall(...)`

问题有两个：
1. check 前先 register output variable
2. 检查用的是全 trace vars，不是本次 action input objects

### 目标拆分

#### A. PreviewCheckToolCall
纯检查，不写变量。

```go
type IFCPreview struct {
    Decision     *IFCDecision
    ContextLabel IFCLabel
    ArgsLabel    IFCLabel
    InputVarIDs  []string
}

func (e *IFCEngine) PreviewCheckToolCall(traceID, toolName string, inputVarIDs []string, contextLabel *IFCLabel) *IFCPreview
```

#### B. PreviewToolOutput
为 tool output 构造 preview variable，但不落库。

```go
type IFCPreviewVariable struct {
    Variable IFCVariable
    Content  string
}

func (e *IFCEngine) PreviewToolOutput(traceID, name, source, content string, parentIDs []string) *IFCPreviewVariable
```

#### C. CommitVariable
当最终动作允许且 output 实际产生后，再写 committed IFCVariable。

```go
func (e *IFCEngine) CommitVariable(traceID string, pv *IFCPreviewVariable) error
```

### 现有能力保留要求

以下能力全部保留，但调用时机变化：

- `CheckToolCallFides` → 作为 preview enforcement
- `HideContent` → 仍在 request preprocess
- `SelectiveHide` → 仍在 request transform
- `Quarantine` → 仍是 route action，但不再是 ifc 内部副作用
- `DetectDOE` → 作为 signal preview
- `PropagateWithTool` → 用于 preview/commit 两阶段

### 特别注意

未来 `CheckToolCallFides` 的输入必须来自：
- 当前 action 的真实 input object IDs
而不是：
- `collectIFCVarIDs(GetVariables(traceID))`

---

## 3.5 DeviationDetector

### 当前问题

它既做 detection，也做 repair，还会影响 block/allow。

### 目标拆分

#### A. PreviewDeviation
不立刻改 tool / args，不立刻持久化 deviation。

```go
type DeviationPreview struct {
    Result       *DeviationResult
    Repair       *RepairResult
}

func (dd *DeviationDetector) Preview(traceID, toolName, toolArgs string) *DeviationPreview
```

#### B. ApplyRepair
在 combine 选择“采用 repair”之后，再真正替换 tool / args。

```go
func (dd *DeviationDetector) ApplyRepair(tool, args string, repair *RepairResult) (string, string)
```

#### C. CommitDeviation
最终动作明确后再记录 deviation。

```go
func (dd *DeviationDetector) CommitDeviation(traceID string, preview *DeviationPreview, finalDecision string) error
```

### 兼容要求

- `replace_tool` 不丢
- `sanitize_args` 不丢
- `block` 策略不丢
- 但 preview 阶段不能再直接写 DB

---

## 4. Preview/Commit Decision Table

| Engine | Preview 时允许做什么 | Commit 时允许做什么 | 绝对不能在 Preview 做什么 |
|---|---|---|---|
| PathPolicy | 读 committed context，算 risk/proposal | 写 step / path event | 写真实 step |
| PlanCompiler | 匹配 step / 生成 violation preview | 推进 current_step / append executed_steps / persist violation | 修改 active plan |
| Capability | 计算 preview object / evaluation | 写 DataItems / Evaluations | 注册真实 tool result |
| IFC | 检查 P-F/P-T / 计算 labels / DOE / quarantine proposal | 写 IFCVariable / violation / hidden records | 注册 output var 后再检查 |
| Deviation | 生成 deviation/repair proposal | 持久化 deviation | preview 时直接改 tool/args 并落库 |

---

## 5. Shared Compatibility Helpers

为了少改主链，建议先加一个共享 helper 层：

### 5.1 GovernancePreviewContext

```go
type GovernancePreviewContext struct {
    TraceID      string
    ToolName     string
    ToolArgs     string
    SenderID     string
    TenantID     string
    InputObjectIDs []string
    ContextLabel *IFCLabel
}
```

### 5.2 GovernanceCommitPolicy

```go
func shouldCommit(finalDecision string) bool {
    switch finalDecision {
    case "allow", "warn", "review", "quarantine_forwarded", "repaired_allow":
        return true
    default:
        return false
    }
}
```

这个函数后续可以细化，但先有一个统一入口，避免每个引擎自己猜。

---

## 6. Main LLM Proxy Refactor Sequence

### 当前顺序（简化）

```text
ToolPolicy+Path
Plan
Capability
Deviation
IFC
```

### 建议的新顺序（保持现有能力，先做边界改造）

```text
1. Build preview context
2. Path preview
3. Plan preview
4. Capability preview
5. Deviation preview
6. IFC preview
7. Combine
8. Apply transforms/repairs/routes
9. Commit path/plan/capability/ifc/deviation state
```

注意：
- request-side `HideContent / SelectiveHide / Reversal.PreInject` 仍在更前面的 preprocess stage
- 这里只改 tool-call governance 链

---

## 7. Proposed Implementation Order

### Stage 1 — Add preview APIs without changing behavior
先新增：
- `PathPolicyEngine.Preview`
- `PlanCompiler.PreviewToolCall`
- `CapabilityEngine.PreviewToolResult / PreviewEvaluate`
- `IFCEngine.PreviewCheckToolCall / PreviewToolOutput`
- `DeviationDetector.Preview`

旧接口暂时保留。

### Stage 2 — Make legacy methods call preview+commit internally
让：
- `EvaluateToolCall`
- `RegisterStep+Evaluate`
- `RegisterToolResult+Evaluate`
- `RegisterVariable+CheckToolCall`

这些旧路径内部变成：
- preview
- immediate commit（维持旧行为）

这样不会破坏现有外部调用，但底层已经具备新结构。

### Stage 3 — Migrate main LLM proxy to preview-first flow
把 `llm_proxy.go` 主链切换到：
- collect previews
- combine
- commit selected state only

### Stage 4 — Add regression tests for “blocked action does not commit state”
这是整个改造的验收关键。

---

## 8. Required Regression Tests

以下测试是本轮改造的硬要求。

### PathPolicy
- blocked tool call 不应写入 committed step
- isolate proposal 不应自动写 committed step

### PlanCompiler
- preview 不推进 `CurrentStep`
- block 后 `ExecutedSteps` 不增加
- repair adopted 后才 commit step

### Capability
- preview evaluate 不应写 `DataItems`
- final block 后无新 capability object
- final allow 后 object sources/parents/trust 正确落账

### IFC
- preview check 不创建 committed variable
- final block 后不生成 output variable
- quarantine route 后 session 正确创建，但 normal path 不提交 output variable

### Deviation
- preview repair 不持久化 deviation
- final adopt repair 后才记录 repaired deviation

---

## 9. Files Likely to Change

### Core refactor files
- `src/path_policy.go`
- `src/plan_compiler.go`
- `src/plan_deviation.go`
- `src/capability.go`
- `src/ifc_engine.go`
- `src/llm_tool_governance.go`
- `src/llm_deep_governance.go`
- `src/llm_ifc_governance.go`
- `src/llm_deviation_governance.go`
- `src/llm_proxy.go`

### New helper files (recommended)
- `src/governance_preview.go`
- `src/governance_commit.go`

### Tests
- `src/path_policy_test.go`
- `src/plan_compiler_test.go`
- `src/plan_deviation_test.go`
- `src/capability_test.go`
- `src/ifc_test.go`
- `src/llm_deep_governance_test.go`
- `src/llm_ifc_governance_test.go`
- `src/llm_deviation_governance_test.go`
- new: `src/governance_preview_commit_test.go`

---

## 10. Success Criteria

本轮改造完成后，必须满足：

1. 被 block 的 tool_call 不再污染 Path / Plan / Capability / IFC committed state
2. preview evaluation 可重复调用，不产生副作用
3. Deviation repair 仍然可用
4. IFC 全部现有能力保留
5. 现有行为在 legacy 路径上不回归
6. 为 proposal/outcome schema 铺好接口边界

---

## 11. Final Recommendation

如果只能先做一件事，就做这件事：

> **把 Plan / Path / Capability / IFC / Deviation 从“评估即提交”改成“先 preview，后 commit”。**

这一步做完，Unified Governance 才有可能在不损伤现有特性的情况下落地。
