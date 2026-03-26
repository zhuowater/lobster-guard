# v26 Implementation Plan — Fides IFC (Information Flow Control)

## 教训总结 (从 v25 学到的)
1. **StatCard 字段名必须和 API 返回一致** — 写 Vue 时直接对照 Go struct JSON tag
2. **内存计数器重启归零** — 所有 Stats() 必须从 DB 恢复
3. **config.yaml 支持** — 新引擎必须有 config struct + main.go 读取
4. **proxy hooks 需要 gate 条件** — 确保 toolPolicy 等依赖注入到位
5. **测试要真的触发** — 注入所有依赖，断言要 fail 不要 skip
6. **四层验证一次做完** — L1 编译 + L2 测试 + L3 设计 + L4 E2E

## 版本拆分

### v26.0 — IFC 核心引擎 + 双标签系统
**文件**: ifc_engine.go (~500L) + ifc_test.go (~300L) + api_ifc.go (~200L)

核心类型:
```go
type IFCLevel int
const (
    LevelPublic IFCLevel = 0
    LevelInternal IFCLevel = 1
    LevelConfidential IFCLevel = 2
    LevelSecret IFCLevel = 3
)

type IntegLevel int
const (
    IntegTaint IntegLevel = 0
    IntegLow IntegLevel = 1
    IntegMedium IntegLevel = 2
    IntegHigh IntegLevel = 3
)

type IFCLabel struct {
    Confidentiality IFCLevel  `json:"confidentiality"`
    Integrity       IntegLevel `json:"integrity"`
}

type IFCSourceRule struct {
    Source string   `json:"source"` // system_prompt / user_input / tool:web_fetch / tool:db_query ...
    Label  IFCLabel `json:"label"`
}

type IFCToolRequirement struct {
    Tool          string     `json:"tool"`
    RequiredInteg IntegLevel `json:"required_integrity"`
    MaxConf       IFCLevel   `json:"max_confidentiality"` // 最大允许机密等级(出站)
}

type IFCVariable struct {
    ID       string   `json:"id"`
    TraceID  string   `json:"trace_id"`
    Name     string   `json:"name"`
    Label    IFCLabel `json:"label"`
    Source   string   `json:"source"`
    Parents  []string `json:"parents"` // 父变量 ID (传播链)
}

type IFCViolation struct {
    ID        string    `json:"id"`
    TraceID   string    `json:"trace_id"`
    Type      string    `json:"type"` // confidentiality / integrity
    Variable  string    `json:"variable"`
    VarLabel  IFCLabel  `json:"var_label"`
    Required  IFCLabel  `json:"required"`
    Tool      string    `json:"tool"`
    Action    string    `json:"action"` // block / warn / log
    Timestamp time.Time `json:"timestamp"`
}

type IFCConfig struct {
    Enabled            bool                 `yaml:"enabled" json:"enabled"`
    DefaultConf        IFCLevel             `yaml:"default_confidentiality" json:"default_confidentiality"`
    DefaultInteg       IntegLevel           `yaml:"default_integrity" json:"default_integrity"`
    ViolationAction    string               `yaml:"violation_action" json:"violation_action"` // block/warn/log
    SourceRules        []IFCSourceRule       `yaml:"source_rules" json:"source_rules"`
    ToolRequirements   []IFCToolRequirement  `yaml:"tool_requirements" json:"tool_requirements"`
    QuarantineEnabled  bool                 `yaml:"quarantine_enabled" json:"quarantine_enabled"`
    QuarantineUpstream string               `yaml:"quarantine_upstream" json:"quarantine_upstream"`
    HidingEnabled      bool                 `yaml:"hiding_enabled" json:"hiding_enabled"`
    HidingThreshold    IFCLevel             `yaml:"hiding_threshold" json:"hiding_threshold"`
}
```

核心方法:
```
IFCEngine.RegisterVariable(traceID, name, source, content) → IFCVariable
IFCEngine.Propagate(traceID, outputName, inputVarIDs) → IFCVariable  // conf=max, integ=min
IFCEngine.CheckToolCall(traceID, toolName, inputVarIDs) → IFCDecision
IFCEngine.GetVariables(traceID) → []IFCVariable
IFCEngine.GetViolations(traceID) → []IFCViolation
IFCEngine.GetStats() → IFCStats
```

API:
```
GET  /api/v1/ifc/config          -- 获取配置
PUT  /api/v1/ifc/config          -- 更新配置
GET  /api/v1/ifc/source-rules    -- 标签来源规则
POST /api/v1/ifc/source-rules    -- 添加规则
PUT  /api/v1/ifc/source-rules/:source -- 更新规则
DELETE /api/v1/ifc/source-rules/:source -- 删除规则
GET  /api/v1/ifc/tool-requirements -- 工具安全要求
POST /api/v1/ifc/tool-requirements -- 添加要求
GET  /api/v1/ifc/variables?trace_id= -- 变量列表
GET  /api/v1/ifc/violations       -- 违规列表
GET  /api/v1/ifc/stats            -- 统计
POST /api/v1/ifc/check            -- 手动检查
```

测试:
- TestIFCLabelPropagation — conf=max, integ=min
- TestIFCConfidentialityViolation — SECRET 数据不能流向 PUBLIC 通道
- TestIFCIntegrityViolation — TAINT 数据不能驱动 HIGH integ 工具
- TestIFCToolCallCheck — 完整 tool call 检查链路
- TestIFCSourceRuleCRUD — 规则增删改查
- TestIFCStatsFromDB — 重启恢复计数器

### v26.1 — 隔离 LLM (Quarantine)
**文件**: ifc_quarantine.go (~200L) + 修改 llm_proxy.go

- IFC 检测到 integ=TAINT 数据进入高 integ 操作
- 自动路由到 quarantine 上游（只读 tool 权限）
- quarantine 输出标记 integ=MEDIUM（去污）
- 与现有多上游路由联动

### v26.2 — 选择性隐藏 + DOE 检测
**文件**: ifc_hiding.go (~300L) + ifc_doe.go (~200L)

- 高机密字段替换为 [REDACTED:conf=SECRET]
- DOE: 比对 tool 间传输字段 vs 任务最小集 (v25.0 PlanTemplate.allowed_tools)
- 三级: info / warning / critical

### Dashboard + config + proxy hooks (贯穿 v26.0-v26.2)
- IFC.vue — 标签来源配置 + 传播规则可视化 + 违规日志 + 变量追踪
- config.yaml 的 ifc section
- InboundProxy: 入站消息自动打标
- LLMProxy: tool_call 前检查 + tool_result 打标
- 确保 StatCard 字段名 = API JSON tag

## 质量检查点 (四层)
L1: 编译通过
L2: 全量回归 (>1151 tests PASS + 新增 v26 tests)
L3: 设计闭环 (config.yaml + API + Dashboard + CRUD)
L4: E2E 全链路 (simulate + 142 部署 + 46+ tests PASS)

## 预估
- v26.0: ~1000 行新代码 + ~300 行测试
- v26.1: ~200 行
- v26.2: ~500 行
- Dashboard: ~400 行
- 总计: ~2400 行
