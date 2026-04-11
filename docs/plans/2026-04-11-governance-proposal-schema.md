# Unified Governance Proposal / Outcome Schema Design

> **For Hermes:** 这份文档定义统一治理的“中间语义层”。它不讨论具体哪个引擎先跑，也不讨论 roadmap，只回答一个问题：在尽可能保留现有能力的情况下，治理引擎之间应该用什么统一数据结构交流。

**Goal:** 为 lobster-guard 设计一套统一的 proposal / outcome schema，使 ToolPolicy / Path / Plan / Capability / IFC / Deviation / Counterfactual / Taint / Rewrite 等不同语义的引擎可以共存、组合、解释，而不被粗暴压缩成 `allow/warn/block`。

**Architecture:** 统一治理不应该直接让每个引擎都返回一个 string decision，而应该让每个引擎输出结构化 proposal。proposal 按语义分为 decision / transform / route / signal 四类，最终再由 combiner 生成一个 `GovernanceOutcome`。`GovernanceOutcome` 是执行层唯一消费的结构，而不是直接依赖某个具体引擎的私有返回值。

**Tech Stack:** Go structs, JSON-serializable audit artifacts, SQLite evidence references, LLMProxy orchestration.

---

## 1. Problem Statement

当前各高级引擎的返回结构差异极大：

- ToolPolicy → `ToolCallEvent{Decision, RuleHit, RiskLevel}`
- PathPolicy → `PathDecision{Decision, RuleName, RiskScore}`
- PlanCompiler → `PlanEvaluation{Allowed, Decision, Violation}`
- Capability → `CapEvaluation{Decision, Labels, TrustScore}`
- IFC → `IFCDecision{Allowed, Decision, Violation}`
- Deviation → `DeviationResult{Decision, RepairedTool, RepairedArgs}`
- Counterfactual → `CFVerification{Decision, Verdict, AttributionScore}`
- TaintReversal → string transform + record
- LLM rule engine → block / warn / rewrite / expose

问题不是字段名不统一，而是**语义层级不同**：

- 有些是 final decision
- 有些是 transform
- 有些是 route
- 有些是 signal/evidence
- 有些自带 state mutation

所以 schema 设计的目标不是“字段统一”，而是“语义统一”。

---

## 2. Design Principles

### Principle 1 — Proposal First
每个引擎只能输出 proposal，不直接定义 final outcome。

### Principle 2 — Semantics Over Strings
不要再依赖：
- `Decision string`
- `Allowed bool`

而要显式表达：
- 这是 decision / transform / route / signal 的哪一种

### Principle 3 — Evidence Must Be Carried Through
proposal 不能只给一句 reason，必须能引用：
- trace object
- violation id
- variable id
- plan id
- attribution report id

### Principle 4 — Backward-Compatible Mapping
schema 必须能映射回当前各引擎旧语义，便于 shadow mode 对照。

### Principle 5 — Support Non-Blocking Features
必须原生支持这些现有能力：
- rewrite
- selective_hide
- sanitize_args
- replace_tool
- quarantine
- expose
- isolate
- async signal

---

## 3. Top-Level Schema

统一治理建议分成三层结构：

1. **Input Context**
2. **Proposals**
3. **Outcome**

---

## 4. Governance Input Context

```go
type GovernanceInput struct {
    TraceID         string                 `json:"trace_id"`
    TenantID        string                 `json:"tenant_id,omitempty"`
    SenderID        string                 `json:"sender_id,omitempty"`
    Stage           string                 `json:"stage"`           // llm_request / tool_call / llm_response / outbound
    Action          string                 `json:"action"`          // tool name / route / response action
    RawArgs         string                 `json:"raw_args,omitempty"`
    RequestBody     []byte                 `json:"-"`
    ResponseBody    []byte                 `json:"-"`
    InputObjectIDs  []string               `json:"input_object_ids,omitempty"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
```

### 用途
- 作为 proposal 生成时的统一上下文
- 让不同引擎不必再各自定义半套 ctx struct

---

## 5. Proposal Types

## 5.1 Decision Proposal

适用于：
- ToolPolicy
- PathPolicy
- Plan preview
- Capability evaluate
- IFC enforcement
- Counterfactual(sync)
- 部分 Deviation

```go
type GovernanceDecisionProposal struct {
    Engine          string                 `json:"engine"`          // tool_policy / path_policy / plan / capability / ifc / counterfactual / deviation
    Kind            string                 `json:"kind"`            // rule_violation / path_degradation / plan_mismatch / capability_deny / ifc_integrity / attribution_injection ...
    ProposedAction  string                 `json:"proposed_action"` // allow / warn / review / block / deny / isolate / quarantine
    Severity        string                 `json:"severity"`        // info / low / medium / high / critical
    Reason          string                 `json:"reason"`
    Confidence      float64                `json:"confidence,omitempty"`
    EvidenceRefs    []string               `json:"evidence_refs,omitempty"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
```

### 说明
- `deny` 可以保留作为 proposal 语义，combiner 再决定是否映射到 `block`
- `ProposedAction` 不等于 final decision

---

## 5.2 Transform Proposal

适用于：
- Deviation repair
- IFC Hide / SelectiveHide
- LLM rewrite
- Taint reversal
- PII redact

```go
type GovernanceTransformProposal struct {
    Engine         string                 `json:"engine"`
    Action         string                 `json:"action"`         // rewrite / redact / selective_hide / sanitize_args / replace_tool / taint_reverse_soft / taint_reverse_hard / taint_reverse_stealth
    Target         string                 `json:"target"`         // request_body / response_body / tool_args / tool_name / tool_message
    Reason         string                 `json:"reason"`
    SafeByDefault  bool                   `json:"safe_by_default"` // true 表示可默认执行；false 需要 combine 选择
    EvidenceRefs   []string               `json:"evidence_refs,omitempty"`
    Payload        map[string]interface{} `json:"payload,omitempty"`
}
```

### Payload 示例

#### replace_tool
```json
{
  "from": "send_email",
  "to": "email_compose"
}
```

#### sanitize_args
```json
{
  "repaired_args": "{...}",
  "policy_id": "rp-002"
}
```

#### selective_hide
```json
{
  "modified_content": "[IFC_VAR:ifc-123 ...]",
  "hidden_var_ids": ["ifc-123"]
}
```

---

## 5.3 Route Proposal

适用于：
- IFC quarantine
- Path isolate
- Singularity expose
- future MCP reroute

```go
type GovernanceRouteProposal struct {
    Engine        string                 `json:"engine"`
    Route         string                 `json:"route"`          // normal / quarantine / isolate / expose / alternate_upstream
    Reason        string                 `json:"reason"`
    Target        string                 `json:"target,omitempty"` // upstream id / session id / expose template id
    EvidenceRefs  []string               `json:"evidence_refs,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
```

### 说明
- `Route` 是 first-class，不要再拿 block/warn 代替
- `quarantine` / `isolate` / `expose` 都应该走 route channel

---

## 5.4 Signal Proposal

适用于：
- Path risk high
- DOE warning
- Counterfactual async result
- Taint present
- untrusted lineage
- hidden content generated

```go
type GovernanceSignalProposal struct {
    Engine        string                 `json:"engine"`
    Signal        string                 `json:"signal"`
    Severity      string                 `json:"severity"`
    Reason        string                 `json:"reason"`
    Value         interface{}            `json:"value,omitempty"`
    EvidenceRefs  []string               `json:"evidence_refs,omitempty"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
```

### 作用
- 不直接决定 final action
- 进入 explainability / scoring / audit / retrospective analysis

---

## 6. Proposal Bundle

统一组合结构：

```go
type GovernanceProposalBundle struct {
    Input       GovernanceInput               `json:"input"`
    Decisions   []GovernanceDecisionProposal  `json:"decisions,omitempty"`
    Transforms  []GovernanceTransformProposal `json:"transforms,omitempty"`
    Routes      []GovernanceRouteProposal     `json:"routes,omitempty"`
    Signals     []GovernanceSignalProposal    `json:"signals,omitempty"`
}
```

### Combiner 输入
combiner 不直接接某个 engine 的结果，而只接 `GovernanceProposalBundle`。

---

## 7. Governance Outcome

执行层唯一应该消费的结构：

```go
type GovernanceOutcome struct {
    FinalDecision   string                       `json:"final_decision"`   // allow / warn / review / block
    RouteAction     string                       `json:"route_action"`     // normal / quarantine / isolate / expose / alternate_upstream
    AppliedTransforms []GovernanceTransformProposal `json:"applied_transforms,omitempty"`
    WinningReasons  []GovernanceDecisionProposal `json:"winning_reasons,omitempty"`
    SupportingSignals []GovernanceSignalProposal `json:"supporting_signals,omitempty"`
    AuditRefs       []string                     `json:"audit_refs,omitempty"`
    Explain         GovernanceExplanation        `json:"explain"`
}
```

### Why separate `WinningReasons` and `SupportingSignals`
因为：
- 最终 block 可能是由 capability deny 决定
- 但 path risk、DOE、taint 也可能是 supporting evidence

不能把所有东西都塞进一个 reason list。

---

## 8. Explanation Schema

```go
type GovernanceExplanation struct {
    Summary         string   `json:"summary"`
    DecisionPath    []string `json:"decision_path,omitempty"`
    MissingCaps     []string `json:"missing_caps,omitempty"`
    ViolatedFlows   []string `json:"violated_flows,omitempty"`
    SelectedRoute   string   `json:"selected_route,omitempty"`
    AppliedTransformNames []string `json:"applied_transform_names,omitempty"`
}
```

### 示例

```json
{
  "summary": "Blocked because send_email was proposed in an untrusted context and parameter lineage lacked act.email.send.",
  "decision_path": [
    "ifc.integrity_violation -> quarantine candidate",
    "capability.deny -> block candidate",
    "combiner selected capability deny as stronger final decision"
  ],
  "missing_caps": ["act.email.send"],
  "violated_flows": ["SECRET -> PUBLIC"],
  "selected_route": "normal",
  "applied_transform_names": []
}
```

---

## 9. Combiner Rules (Recommended Semantics)

## 9.1 Decision precedence
建议顺序：

```text
block > deny > quarantine > isolate > review > warn > allow
```

注意：
- `deny` 是 proposal semantic，不一定直接对外暴露
- `quarantine` / `isolate` 是 route semantic，但优先级要能压过 warn/allow

## 9.2 Route precedence
建议：

```text
expose > quarantine > isolate > alternate_upstream > normal
```

说明：
- `expose` 是故意替代响应
- `quarantine` 是安全分流
- `isolate` 是会话级隔离

## 9.3 Transform ordering
建议执行顺序：

```text
sanitize_args / replace_tool
    -> hide / selective_hide / redact
    -> rewrite
    -> taint reversal
```

理由：
- 先修正动作语义
- 再缩减数据暴露
- 再做内容级 rewrite
- 最后做 response 级 reversal

---

## 10. Mapping Existing Engines to the New Schema

## 10.1 ToolPolicy → DecisionProposal

```go
GovernanceDecisionProposal{
    Engine: "tool_policy",
    Kind: "tool_rule_hit",
    ProposedAction: tpEvent.Decision,
    Severity: tpEvent.RiskLevel,
    Reason: tpEvent.RuleHit,
    EvidenceRefs: []string{tpEvent.ID},
}
```

## 10.2 PathPolicy → DecisionProposal + SignalProposal

```go
GovernanceDecisionProposal{
    Engine: "path_policy",
    Kind: "path_rule",
    ProposedAction: pp.Decision,
    Severity: pathDecisionSeverity(pp.Decision),
    Reason: pp.Reason,
}

GovernanceSignalProposal{
    Engine: "path_policy",
    Signal: "risk_score",
    Severity: riskBand(pp.RiskScore),
    Value: pp.RiskScore,
}
```

## 10.3 PlanCompiler → DecisionProposal

```go
GovernanceDecisionProposal{
    Engine: "plan",
    Kind: "plan_violation",
    ProposedAction: preview.Decision,
    Severity: previewViolationSeverity(preview),
    Reason: preview.Reason,
    EvidenceRefs: []string{preview.PlanID},
}
```

## 10.4 Deviation repair → TransformProposal

```go
GovernanceTransformProposal{
    Engine: "deviation",
    Action: "replace_tool",
    Target: "tool_name",
    Reason: repair.Reason,
    Payload: map[string]interface{}{"to": repair.Tool},
}
```

## 10.5 Capability → DecisionProposal + SignalProposal

```go
GovernanceDecisionProposal{
    Engine: "capability",
    Kind: "capability_deny",
    ProposedAction: mapCapDecision(capEval.Decision),
    Severity: capSeverity(capEval.Decision),
    Reason: capEval.Reason,
    EvidenceRefs: []string{capEval.ID},
}

GovernanceSignalProposal{
    Engine: "capability",
    Signal: "trust_score",
    Severity: trustBand(capEval.TrustScore),
    Value: capEval.TrustScore,
}
```

## 10.6 IFC → Decision / Transform / Route / Signal

### Enforcement
```go
GovernanceDecisionProposal{
    Engine: "ifc",
    Kind: "integrity_violation",
    ProposedAction: ifcDecision.Decision,
    Severity: "high",
    Reason: ifcDecision.Reason,
    EvidenceRefs: []string{ifcDecision.Violation.ID},
}
```

### SelectiveHide
```go
GovernanceTransformProposal{
    Engine: "ifc",
    Action: "selective_hide",
    Target: "tool_message",
    Reason: hideResult.Reason,
    Payload: map[string]interface{}{
        "modified_content": hideResult.Modified,
        "var_ids": hideResult.VarIDs,
    },
}
```

### Quarantine
```go
GovernanceRouteProposal{
    Engine: "ifc",
    Route: "quarantine",
    Reason: "tainted integrity requires quarantined LLM",
    Target: quarantineUpstream,
}
```

### DOE
```go
GovernanceSignalProposal{
    Engine: "ifc",
    Signal: "doe",
    Severity: doeResult.Severity,
    Reason: "data over-exposure detected",
    Value: doeResult.ExcessFields,
}
```

## 10.7 Counterfactual → Decision or Signal

### sync
```go
GovernanceDecisionProposal{...}
```

### async
```go
GovernanceSignalProposal{
    Engine: "counterfactual",
    Signal: "attribution_async_pending",
    Severity: "medium",
    Reason: "counterfactual verification scheduled asynchronously",
}
```

## 10.8 TaintReversal → TransformProposal

```go
GovernanceTransformProposal{
    Engine: "taint_reversal",
    Action: "taint_reverse_soft",
    Target: "response_body",
    Reason: "tainted output requires detox message",
}
```

---

## 11. Audit Serialization

为了让 unified governance 可审计，建议新增统一表或统一 envelope payload：

```go
type GovernanceAuditRecord struct {
    TraceID   string                  `json:"trace_id"`
    Stage     string                  `json:"stage"`
    Input     GovernanceInput         `json:"input"`
    Bundle    GovernanceProposalBundle `json:"bundle"`
    Outcome   GovernanceOutcome       `json:"outcome"`
    Timestamp string                  `json:"timestamp"`
}
```

这会让未来 Dashboard 能直接展示：
- proposals
- chosen decision
- discarded proposals
- applied transforms
- selected route

---

## 12. Backward Compatibility Strategy

### Stage 1
只新增 schema 和 adapter，不改旧返回值。

### Stage 2
每个引擎增加 adapter：
- `ToDecisionProposal()`
- `ToTransformProposal()`
- `ToRouteProposal()`
- `ToSignalProposal()`

### Stage 3
主链生成 bundle，但先不依赖 outcome，shadow compare 旧行为。

### Stage 4
执行层只消费 `GovernanceOutcome`。

---

## 13. What This Schema Explicitly Prevents

这套 schema 的价值，在于它显式阻止以下错误：

1. 把 `SelectiveHide` 当成 warn/block
2. 把 `Quarantine` 当成 block
3. 把 `Deviation repair` 当成“只是一个日志”
4. 把 async Counterfactual 强制变成同步 verdict
5. 把 TaintReversal 错塞进 final decision
6. 把所有 evidence 压成一条 reason 字符串

---

## 14. Suggested Next Step

在真正实现 unified governance 之前，建议下一份文档只做一件事：

- 给出 `src/governance_types.go` 的完整 struct 草案
- 给出 `src/governance_combiner.go` 的最小 API
- 给出每个旧引擎的 adapter skeleton

一句话总结：

> 统一治理不是让所有引擎都说同一句话，而是让它们用统一语法表达各自真正的能力。
