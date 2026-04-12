// ifc_engine.go — IFC (Information Flow Control) 引擎 (v26.0)
// Bell-LaPadula 模型：机密性上行、完整性下行
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// IFC Level (Confidentiality)
// ============================================================

type IFCLevel int

const (
	ConfPublic       IFCLevel = 0 // 公开
	ConfInternal     IFCLevel = 1 // 内部
	ConfConfidential IFCLevel = 2 // 机密
	ConfSecret       IFCLevel = 3 // 绝密
)

func (l IFCLevel) String() string {
	switch l {
	case ConfPublic:
		return "PUBLIC"
	case ConfInternal:
		return "INTERNAL"
	case ConfConfidential:
		return "CONFIDENTIAL"
	case ConfSecret:
		return "SECRET"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(l))
	}
}

func ParseIFCLevel(s string) IFCLevel {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PUBLIC", "0":
		return ConfPublic
	case "INTERNAL", "1":
		return ConfInternal
	case "CONFIDENTIAL", "2":
		return ConfConfidential
	case "SECRET", "3":
		return ConfSecret
	default:
		return ConfPublic
	}
}

// ============================================================
// Integrity Level
// ============================================================

type IntegLevel int

const (
	IntegTaint  IntegLevel = 0 // 被污染
	IntegLow    IntegLevel = 1 // 低
	IntegMedium IntegLevel = 2 // 中
	IntegHigh   IntegLevel = 3 // 高
)

func (l IntegLevel) String() string {
	switch l {
	case IntegTaint:
		return "TAINT"
	case IntegLow:
		return "LOW"
	case IntegMedium:
		return "MEDIUM"
	case IntegHigh:
		return "HIGH"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(l))
	}
}

func ParseIntegLevel(s string) IntegLevel {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TAINT", "0":
		return IntegTaint
	case "LOW", "1":
		return IntegLow
	case "MEDIUM", "2":
		return IntegMedium
	case "HIGH", "3":
		return IntegHigh
	default:
		return IntegLow
	}
}

// ============================================================
// Core types
// ============================================================

type IFCLabel struct {
	Confidentiality IFCLevel   `json:"confidentiality"`
	Integrity       IntegLevel `json:"integrity"`
}

type IFCSourceRule struct {
	Source string   `json:"source" yaml:"source"`
	Label  IFCLabel `json:"label" yaml:"label"`
}

type IFCToolRequirement struct {
	Tool          string     `json:"tool" yaml:"tool"`
	RequiredInteg IntegLevel `json:"required_integrity" yaml:"required_integrity"`
	MaxConf       IFCLevel   `json:"max_confidentiality" yaml:"max_confidentiality"`
}

type IFCVariable struct {
	ID        string    `json:"id"`
	TraceID   string    `json:"trace_id"`
	Name      string    `json:"name"`
	Label     IFCLabel  `json:"label"`
	Source    string    `json:"source"`
	Parents   []string  `json:"parents"`
	CreatedAt time.Time `json:"created_at"`
}

type IFCViolation struct {
	ID        string    `json:"id"`
	TraceID   string    `json:"trace_id"`
	Type      string    `json:"type"`
	Variable  string    `json:"variable"`
	VarLabel  IFCLabel  `json:"var_label"`
	Required  IFCLabel  `json:"required"`
	Tool      string    `json:"tool"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
}

type IFCDecision struct {
	Allowed   bool          `json:"allowed"`
	Decision  string        `json:"decision"`
	Violation *IFCViolation `json:"violation,omitempty"`
	Reason    string        `json:"reason"`
}

type IFCHideResult struct {
	Original     string   `json:"original"`
	Redacted     string   `json:"redacted"`
	HiddenCount  int      `json:"hidden_count"`
	HiddenFields []string `json:"hidden_fields"`
}

type IFCDOEResult struct {
	Tool           string   `json:"tool"`
	ExposedFields  []string `json:"exposed_fields"`
	RequiredFields []string `json:"required_fields"`
	ExcessFields   []string `json:"excess_fields"`
	Severity       string   `json:"severity"`
}

type IFCConfig struct {
	Enabled            bool                 `yaml:"enabled" json:"enabled"`
	DefaultConf        IFCLevel             `yaml:"default_confidentiality" json:"default_confidentiality"`
	DefaultInteg       IntegLevel           `yaml:"default_integrity" json:"default_integrity"`
	ViolationAction    string               `yaml:"violation_action" json:"violation_action"`
	SourceRules        []IFCSourceRule       `yaml:"source_rules" json:"source_rules"`
	ToolRequirements   []IFCToolRequirement  `yaml:"tool_requirements" json:"tool_requirements"`
	QuarantineEnabled  bool                 `yaml:"quarantine_enabled" json:"quarantine_enabled"`
	QuarantineUpstream string               `yaml:"quarantine_upstream" json:"quarantine_upstream"`
	HidingEnabled      bool                 `yaml:"hiding_enabled" json:"hiding_enabled"`
	HidingThreshold    IFCLevel             `yaml:"hiding_threshold" json:"hiding_threshold"`
}

type IFCStats struct {
	TotalVariables    int64            `json:"total_variables"`
	ActiveTraces      int64            `json:"active_traces"`
	TotalViolations   int64            `json:"total_violations"`
	ConfViolations    int64            `json:"conf_violations"`
	IntegViolations   int64            `json:"integ_violations"`
	TotalBlocked      int64            `json:"total_blocked"`
	TotalWarned       int64            `json:"total_warned"`
	TotalHidden       int64            `json:"total_hidden"`
	TotalDOE          int64            `json:"total_doe"`
	SourceRuleCount   int              `json:"source_rule_count"`
	ToolReqCount      int              `json:"tool_req_count"`
	LabelDistribution map[string]int64 `json:"label_distribution"`
}

// ============================================================
// Engine
// ============================================================

type IFCEngine struct {
	db               *sql.DB
	config           IFCConfig
	mu               sync.RWMutex
	sourceRules      map[string]IFCLabel
	toolRequirements map[string]IFCToolRequirement
	variables        map[string]map[string]*IFCVariable // trace_id → var_id → var
	totalVariables   int64
	totalViolations  int64
	confViolations   int64
	integViolations  int64
	totalBlocked     int64
	totalWarned      int64
	totalHidden      int64
	totalDOE         int64
}

func generateIFCID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ifc-" + hex.EncodeToString(b)
}

// ============================================================
// Default rules
// ============================================================

func defaultIFCSourceRules() []IFCSourceRule {
	return []IFCSourceRule{
		{Source: "system_prompt", Label: IFCLabel{Confidentiality: ConfSecret, Integrity: IntegHigh}},
		{Source: "user_input", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium}},
		{Source: "tool:web_fetch", Label: IFCLabel{Confidentiality: ConfPublic, Integrity: IntegTaint}},
		{Source: "tool:database_query", Label: IFCLabel{Confidentiality: ConfConfidential, Integrity: IntegLow}},
		{Source: "tool:mcp_tool", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegLow}},
		{Source: "tool:file_read", Label: IFCLabel{Confidentiality: ConfInternal, Integrity: IntegMedium}},
	}
}

func defaultIFCToolRequirements() []IFCToolRequirement {
	return []IFCToolRequirement{
		{Tool: "shell_exec", RequiredInteg: IntegHigh, MaxConf: ConfInternal},
		{Tool: "send_email", RequiredInteg: IntegMedium, MaxConf: ConfInternal},
		{Tool: "file_write", RequiredInteg: IntegMedium, MaxConf: ConfConfidential},
		{Tool: "database_write", RequiredInteg: IntegHigh, MaxConf: ConfConfidential},
		{Tool: "web_fetch", RequiredInteg: IntegLow, MaxConf: ConfPublic},
	}
}

// ============================================================
// Constructor
// ============================================================

func NewIFCEngine(db *sql.DB, config IFCConfig) *IFCEngine {
	e := &IFCEngine{
		db:               db,
		config:           config,
		sourceRules:      make(map[string]IFCLabel),
		toolRequirements: make(map[string]IFCToolRequirement),
		variables:        make(map[string]map[string]*IFCVariable),
	}

	if config.ViolationAction == "" {
		e.config.ViolationAction = "warn"
	}

	e.initDB()

	// Load source rules: config > defaults
	if len(config.SourceRules) > 0 {
		for _, r := range config.SourceRules {
			e.sourceRules[r.Source] = r.Label
		}
	} else {
		for _, r := range defaultIFCSourceRules() {
			e.sourceRules[r.Source] = r.Label
		}
	}

	// Load tool requirements: config > defaults
	if len(config.ToolRequirements) > 0 {
		for _, r := range config.ToolRequirements {
			e.toolRequirements[r.Tool] = r
		}
	} else {
		for _, r := range defaultIFCToolRequirements() {
			e.toolRequirements[r.Tool] = r
		}
	}

	// Persist source rules to DB
	for src, label := range e.sourceRules {
		e.db.Exec(`INSERT OR IGNORE INTO ifc_source_rules (source, conf, integ) VALUES (?, ?, ?)`,
			src, int(label.Confidentiality), int(label.Integrity))
	}

	// Persist tool requirements to DB
	for tool, req := range e.toolRequirements {
		e.db.Exec(`INSERT OR IGNORE INTO ifc_tool_requirements (tool, required_integ, max_conf) VALUES (?, ?, ?)`,
			tool, int(req.RequiredInteg), int(req.MaxConf))
	}

	// Restore counters from DB
	e.restoreCounters()

	return e
}

func (e *IFCEngine) initDB() {
	if e.db == nil {
		return
	}

	e.db.Exec(`CREATE TABLE IF NOT EXISTS ifc_variables (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		conf INTEGER NOT NULL DEFAULT 0,
		integ INTEGER NOT NULL DEFAULT 0,
		source TEXT NOT NULL DEFAULT '',
		parents TEXT NOT NULL DEFAULT '[]',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ifc_vars_trace ON ifc_variables(trace_id)`)

	e.db.Exec(`CREATE TABLE IF NOT EXISTS ifc_violations (
		id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		type TEXT NOT NULL,
		variable TEXT NOT NULL DEFAULT '',
		var_conf INTEGER NOT NULL DEFAULT 0,
		var_integ INTEGER NOT NULL DEFAULT 0,
		req_conf INTEGER NOT NULL DEFAULT 0,
		req_integ INTEGER NOT NULL DEFAULT 0,
		tool TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT 'warn',
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ifc_viol_trace ON ifc_violations(trace_id)`)

	e.db.Exec(`CREATE TABLE IF NOT EXISTS ifc_source_rules (
		source TEXT PRIMARY KEY,
		conf INTEGER NOT NULL DEFAULT 0,
		integ INTEGER NOT NULL DEFAULT 0
	)`)

	e.db.Exec(`CREATE TABLE IF NOT EXISTS ifc_tool_requirements (
		tool TEXT PRIMARY KEY,
		required_integ INTEGER NOT NULL DEFAULT 0,
		max_conf INTEGER NOT NULL DEFAULT 0
	)`)

	// Fides SelectiveHide: store original content of hidden variables
	e.db.Exec(`CREATE TABLE IF NOT EXISTS ifc_hidden_content (
		var_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		tool_name TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		conf INTEGER NOT NULL DEFAULT 0,
		integ INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_ifc_hidden_trace ON ifc_hidden_content(trace_id)`)
}

func (e *IFCEngine) restoreCounters() {
	if e.db == nil {
		return
	}

	var count int64
	row := e.db.QueryRow(`SELECT COUNT(*) FROM ifc_variables`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.totalVariables, count)
	}

	row = e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.totalViolations, count)
	}

	row = e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE type='confidentiality'`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.confViolations, count)
	}

	row = e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE type='integrity'`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.integViolations, count)
	}

	row = e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE action='block'`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.totalBlocked, count)
	}

	row = e.db.QueryRow(`SELECT COUNT(*) FROM ifc_violations WHERE action='warn'`)
	if row.Scan(&count) == nil {
		atomic.StoreInt64(&e.totalWarned, count)
	}
}

func (e *IFCEngine) RegisterSourceRule(source string, label IFCLabel) {
	if source == "" {
		return
	}

	e.mu.Lock()
	e.sourceRules[source] = label
	e.mu.Unlock()

	if e.db != nil {
		e.db.Exec(`INSERT OR REPLACE INTO ifc_source_rules (source, conf, integ) VALUES (?, ?, ?)`,
			source, int(label.Confidentiality), int(label.Integrity))
	}
}

// ============================================================
// RegisterVariable
// ============================================================

// ============================================================
// RegisterVariable
// ============================================================

func (e *IFCEngine) RegisterVariable(traceID, name, source, content string) *IFCVariable {
	label := IFCLabel{
		Confidentiality: e.config.DefaultConf,
		Integrity:       e.config.DefaultInteg,
	}

	e.mu.RLock()
	if l, ok := e.sourceRules[source]; ok {
		label = l
	}
	e.mu.RUnlock()

	v := &IFCVariable{
		ID:        generateIFCID(),
		TraceID:   traceID,
		Name:      name,
		Label:     label,
		Source:    source,
		Parents:   []string{},
		CreatedAt: time.Now().UTC(),
	}

	e.mu.Lock()
	if e.variables[traceID] == nil {
		e.variables[traceID] = make(map[string]*IFCVariable)
	}
	e.variables[traceID][v.ID] = v
	e.mu.Unlock()

	atomic.AddInt64(&e.totalVariables, 1)

	// Persist
	if e.db != nil {
		parentsJSON, _ := json.Marshal(v.Parents)
		e.db.Exec(`INSERT INTO ifc_variables (id, trace_id, name, conf, integ, source, parents, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.ID, v.TraceID, v.Name, int(v.Label.Confidentiality), int(v.Label.Integrity),
			v.Source, string(parentsJSON), v.CreatedAt.Format(time.RFC3339))
	}

	return v
}

// ============================================================
// Propagate — Bell-LaPadula: conf=max, integ=min
// ============================================================

func (e *IFCEngine) Propagate(traceID, outputName string, inputVarIDs []string) *IFCVariable {
	return e.PropagateWithTool(traceID, outputName, "", inputVarIDs)
}

// PropagateWithTool — Fides-aligned: joins input var labels + tool's own source rule label (ℓf).
// Algorithm 5 line 9: ℓ'' = ⊔(τ(x) for x in R(f)) ⊔ ℓf ⊔ ⊔(ℓa for a in args)
func (e *IFCEngine) PropagateWithTool(traceID, outputName, toolName string, inputVarIDs []string) *IFCVariable {
	var maxConf IFCLevel
	var minInteg IntegLevel = IntegHigh
	var found bool

	e.mu.RLock()

	// Fides: include tool's own source rule label (ℓf)
	if toolName != "" {
		if toolLabel, ok := e.sourceRules["tool:"+toolName]; ok {
			maxConf = toolLabel.Confidentiality
			minInteg = toolLabel.Integrity
			found = true
		} else if toolLabel, ok := e.sourceRules[toolName]; ok {
			maxConf = toolLabel.Confidentiality
			minInteg = toolLabel.Integrity
			found = true
		}
	}

	traceVars := e.variables[traceID]
	for _, vid := range inputVarIDs {
		if traceVars != nil {
			if v, ok := traceVars[vid]; ok {
				if v.Label.Confidentiality > maxConf {
					maxConf = v.Label.Confidentiality
				}
				if v.Label.Integrity < minInteg {
					minInteg = v.Label.Integrity
				}
				found = true
			}
		}
	}
	e.mu.RUnlock()

	if !found {
		maxConf = e.config.DefaultConf
		minInteg = e.config.DefaultInteg
	}

	v := &IFCVariable{
		ID:        generateIFCID(),
		TraceID:   traceID,
		Name:      outputName,
		Label:     IFCLabel{Confidentiality: maxConf, Integrity: minInteg},
		Source:    "propagated",
		Parents:   inputVarIDs,
		CreatedAt: time.Now().UTC(),
	}

	e.mu.Lock()
	if e.variables[traceID] == nil {
		e.variables[traceID] = make(map[string]*IFCVariable)
	}
	e.variables[traceID][v.ID] = v
	e.mu.Unlock()

	atomic.AddInt64(&e.totalVariables, 1)

	// Persist
	if e.db != nil {
		parentsJSON, _ := json.Marshal(v.Parents)
		e.db.Exec(`INSERT INTO ifc_variables (id, trace_id, name, conf, integ, source, parents, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.ID, v.TraceID, v.Name, int(v.Label.Confidentiality), int(v.Label.Integrity),
			v.Source, string(parentsJSON), v.CreatedAt.Format(time.RFC3339))
	}

	return v
}

// ============================================================
// CheckToolCall
// ============================================================

func (e *IFCEngine) CheckToolCall(traceID, toolName string, inputVarIDs []string) *IFCDecision {
	return e.CheckToolCallFides(traceID, toolName, inputVarIDs, nil)
}

// CheckToolCallFides — Fides-aligned policy enforcement.
// Distinguishes between:
//   - Context label (ℓf): accumulated integrity of the context where the tool call was generated
//     → used for P-T (Trusted Action) check: was the decision to call this tool made in trusted context?
//   - Argument labels (ℓa): aggregated from inputVarIDs
//     → used for P-F (Permitted Flow) check: is the data allowed to flow to this tool?
//
// If contextLabel is nil, falls back to computing it from inputVarIDs (backward compatible).
func (e *IFCEngine) CheckToolCallFides(traceID, toolName string, inputVarIDs []string, contextLabel *IFCLabel) *IFCDecision {
	e.mu.RLock()
	req, hasReq := e.toolRequirements[toolName]
	e.mu.RUnlock()

	if !hasReq {
		return &IFCDecision{
			Allowed:  true,
			Decision: "allow",
			Reason:   fmt.Sprintf("no requirement for tool %s", toolName),
		}
	}

	// Aggregate argument labels from input vars
	var argsMaxConf IFCLevel
	var argsMinInteg IntegLevel = IntegHigh
	var found bool

	e.mu.RLock()
	traceVars := e.variables[traceID]
	for _, vid := range inputVarIDs {
		if traceVars != nil {
			if v, ok := traceVars[vid]; ok {
				if v.Label.Confidentiality > argsMaxConf {
					argsMaxConf = v.Label.Confidentiality
				}
				if v.Label.Integrity < argsMinInteg {
					argsMinInteg = v.Label.Integrity
				}
				found = true
			}
		}
	}
	e.mu.RUnlock()

	if !found {
		argsMaxConf = e.config.DefaultConf
		argsMinInteg = e.config.DefaultInteg
	}

	argsLabel := IFCLabel{Confidentiality: argsMaxConf, Integrity: argsMinInteg}
	reqLabel := IFCLabel{Confidentiality: req.MaxConf, Integrity: req.RequiredInteg}

	// Determine context label for P-T check
	ctxLabel := argsLabel // backward compatible default
	if contextLabel != nil {
		ctxLabel = *contextLabel
	}

	// P-F (Permitted Flow): argument confidentiality ⊑ max allowed
	// "Data flowing into this tool must not exceed the tool's confidentiality ceiling"
	if argsLabel.Confidentiality > req.MaxConf {
		return e.createViolation(traceID, toolName, "confidentiality", argsLabel, reqLabel, inputVarIDs)
	}

	// P-T (Trusted Action): context integrity ≥ required integrity
	// "The decision to call this tool must have been made in a sufficiently trusted context"
	if ctxLabel.Integrity < req.RequiredInteg {
		return e.createViolation(traceID, toolName, "integrity", ctxLabel, reqLabel, inputVarIDs)
	}

	return &IFCDecision{
		Allowed:  true,
		Decision: "allow",
		Reason:   fmt.Sprintf("tool %s: args{conf=%s} ctx{integ=%s} meets P-F{maxConf=%s} P-T{minInteg=%s}",
			toolName, argsLabel.Confidentiality, ctxLabel.Integrity, reqLabel.Confidentiality, reqLabel.Integrity),
	}
}

func (e *IFCEngine) createViolation(traceID, toolName, violationType string, aggLabel, reqLabel IFCLabel, inputVarIDs []string) *IFCDecision {
	action := e.config.ViolationAction
	if action == "" {
		action = "warn"
	}

	varName := ""
	if len(inputVarIDs) > 0 {
		varName = inputVarIDs[0]
	}

	viol := &IFCViolation{
		ID:        generateIFCID(),
		TraceID:   traceID,
		Type:      violationType,
		Variable:  varName,
		VarLabel:  aggLabel,
		Required:  reqLabel,
		Tool:      toolName,
		Action:    action,
		Timestamp: time.Now().UTC(),
	}

	atomic.AddInt64(&e.totalViolations, 1)
	if violationType == "confidentiality" {
		atomic.AddInt64(&e.confViolations, 1)
	} else {
		atomic.AddInt64(&e.integViolations, 1)
	}
	if action == "block" {
		atomic.AddInt64(&e.totalBlocked, 1)
	} else if action == "warn" {
		atomic.AddInt64(&e.totalWarned, 1)
	}

	// Persist
	if e.db != nil {
		e.db.Exec(`INSERT INTO ifc_violations (id, trace_id, type, variable, var_conf, var_integ, req_conf, req_integ, tool, action, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			viol.ID, viol.TraceID, viol.Type, viol.Variable,
			int(viol.VarLabel.Confidentiality), int(viol.VarLabel.Integrity),
			int(viol.Required.Confidentiality), int(viol.Required.Integrity),
			viol.Tool, viol.Action, viol.Timestamp.Format(time.RFC3339))
	}

	allowed := action == "log"

	return &IFCDecision{
		Allowed:   allowed,
		Decision:  action,
		Violation: viol,
		Reason: fmt.Sprintf("%s violation: tool=%s, data label={conf=%s, integ=%s}, required={conf<=%s, integ>=%s}",
			violationType, toolName, aggLabel.Confidentiality, aggLabel.Integrity,
			reqLabel.Confidentiality, reqLabel.Integrity),
	}
}

// ============================================================
// SelectiveHide — Fides-style: hide tool results that would raise context label
// ============================================================
// When a tool result's label would upgrade the current conversation context
// (higher conf or lower integ), replace the content with a variable reference
// and store the original in IFC variable memory. The LLM sees only the
// reference, preserving context label so subsequent tool calls remain allowed.
//
// Returns: modified content (with placeholders), list of created variable IDs,
// and whether any hiding was performed.

type IFCSelectiveHideResult struct {
	Modified    string   `json:"modified"`
	Hidden      bool     `json:"hidden"`
	VarIDs      []string `json:"var_ids"`
	Reason      string   `json:"reason"`
	OrigLabel   IFCLabel `json:"orig_label"`
	ContextLabel IFCLabel `json:"context_label"`
}

func (e *IFCEngine) SelectiveHide(traceID, toolName, content string, contextLabel IFCLabel) *IFCSelectiveHideResult {
	result := &IFCSelectiveHideResult{
		Modified:     content,
		Hidden:       false,
		VarIDs:       []string{},
		ContextLabel: contextLabel,
	}

	// Determine the tool result's label from source rules
	toolLabel := IFCLabel{
		Confidentiality: e.config.DefaultConf,
		Integrity:       e.config.DefaultInteg,
	}
	e.mu.RLock()
	if l, ok := e.sourceRules["tool:"+toolName]; ok {
		toolLabel = l
	} else if l, ok := e.sourceRules[toolName]; ok {
		toolLabel = l
	}
	e.mu.RUnlock()
	result.OrigLabel = toolLabel

	// Fides HIDE decision: would this data raise the context label?
	// Context label rises if: tool_conf > ctx_conf OR tool_integ < ctx_integ
	wouldRaiseConf := toolLabel.Confidentiality > contextLabel.Confidentiality
	wouldLowerInteg := toolLabel.Integrity < contextLabel.Integrity

	if !wouldRaiseConf && !wouldLowerInteg {
		result.Reason = "label within context bounds, no hiding needed"
		return result
	}

	// Store content in IFC variable
	v := e.RegisterVariable(traceID, fmt.Sprintf("hidden:%s", toolName), "tool:"+toolName, content)

	// Persist original content for later expansion
	if e.db != nil {
		e.db.Exec(`INSERT OR REPLACE INTO ifc_hidden_content (var_id, trace_id, tool_name, content, conf, integ, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.ID, traceID, toolName, content, int(toolLabel.Confidentiality), int(toolLabel.Integrity), time.Now().UTC().Format(time.RFC3339))
	}

	// Replace content with variable reference
	placeholder := fmt.Sprintf("[IFC_VAR:%s | tool=%s, conf=%s, integ=%s — content hidden to preserve context label]",
		v.ID, toolName, toolLabel.Confidentiality, toolLabel.Integrity)
	result.Modified = placeholder
	result.Hidden = true
	result.VarIDs = append(result.VarIDs, v.ID)

	reasons := []string{}
	if wouldRaiseConf {
		reasons = append(reasons, fmt.Sprintf("conf %s→%s would raise context", contextLabel.Confidentiality, toolLabel.Confidentiality))
	}
	if wouldLowerInteg {
		reasons = append(reasons, fmt.Sprintf("integ %s→%s would taint context", contextLabel.Integrity, toolLabel.Integrity))
	}
	result.Reason = strings.Join(reasons, "; ")

	atomic.AddInt64(&e.totalHidden, 1)

	log.Printf("[IFC:SelectiveHide] trace=%s tool=%s hidden=true vars=[%s] reason=%s",
		traceID, toolName, v.ID, result.Reason)

	return result
}

// ExpandVariable — retrieve hidden variable content for tool_call argument expansion.
// Called when proxy needs to restore the original data before forwarding to the tool.
func (e *IFCEngine) ExpandVariable(traceID, varID string) (string, *IFCLabel, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	traceVars := e.variables[traceID]
	if traceVars == nil {
		return "", nil, false
	}
	v, ok := traceVars[varID]
	if !ok {
		return "", nil, false
	}
	// Content was stored in RegisterVariable but we need a way to retrieve it.
	// Look up from DB since in-memory IFCVariable doesn't store content.
	if e.db != nil {
		var content string
		err := e.db.QueryRow("SELECT content FROM ifc_hidden_content WHERE var_id = ?", varID).Scan(&content)
		if err == nil {
			return content, &v.Label, true
		}
	}
	return "", &v.Label, false
}

// ============================================================
// HideContent — redact PII fields above threshold
// ============================================================

func (e *IFCEngine) HideContent(traceID, content string, threshold IFCLevel) *IFCHideResult {
	result := &IFCHideResult{
		Original:     content,
		Redacted:     content,
		HiddenCount:  0,
		HiddenFields: []string{},
	}

	redacted := content
	for _, entry := range piiPatterns {
		locs := entry.Pattern.FindAllString(content, -1)
		if len(locs) > 0 {
			replacement := fmt.Sprintf("[REDACTED:conf=%s]", threshold)
			for _, loc := range locs {
				redacted = strings.ReplaceAll(redacted, loc, replacement)
				result.HiddenCount++
			}
			result.HiddenFields = append(result.HiddenFields, entry.Name)
		}
	}

	result.Redacted = redacted
	if result.HiddenCount > 0 {
		atomic.AddInt64(&e.totalHidden, int64(result.HiddenCount))
	}
	return result
}

// ============================================================
// DetectDOE — Data Over-Exposure detection
// ============================================================

func (e *IFCEngine) DetectDOE(traceID, toolName string, fields []string, planTemplate *PlanTemplate) *IFCDOEResult {
	result := &IFCDOEResult{
		Tool:           toolName,
		ExposedFields:  fields,
		RequiredFields: []string{},
		ExcessFields:   []string{},
		Severity:       "info",
	}

	// Determine required fields from plan template
	if planTemplate != nil {
		for _, step := range planTemplate.Steps {
			if step.ToolName == toolName {
				result.RequiredFields = step.AllowedArgs
				break
			}
		}
	}

	// If no required fields defined, all fields are considered required (no excess)
	if len(result.RequiredFields) == 0 {
		result.RequiredFields = fields
		return result
	}

	// Find excess fields
	requiredSet := make(map[string]bool)
	for _, f := range result.RequiredFields {
		requiredSet[f] = true
	}

	for _, f := range fields {
		if !requiredSet[f] {
			result.ExcessFields = append(result.ExcessFields, f)
		}
	}

	// Determine severity based on excess count
	excessCount := len(result.ExcessFields)
	if excessCount == 0 {
		result.Severity = "info"
	} else if excessCount <= 3 {
		result.Severity = "warning"
	} else {
		result.Severity = "critical"
	}

	if excessCount > 0 {
		atomic.AddInt64(&e.totalDOE, 1)
	}

	return result
}

// ============================================================
// ShouldQuarantine
// ============================================================

func (e *IFCEngine) ShouldQuarantine(traceID string, inputVarIDs []string) bool {
	if !e.config.QuarantineEnabled {
		return false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	traceVars := e.variables[traceID]
	if traceVars == nil {
		return false
	}

	for _, vid := range inputVarIDs {
		if v, ok := traceVars[vid]; ok {
			if v.Label.Integrity == IntegTaint {
				return true
			}
		}
	}
	return false
}

// ============================================================
// Query methods
// ============================================================

func (e *IFCEngine) GetVariables(traceID string) []IFCVariable {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []IFCVariable
	if traceVars, ok := e.variables[traceID]; ok {
		for _, v := range traceVars {
			result = append(result, *v)
		}
	}

	// Also query DB for completeness (in case of restart, memory may be empty)
	if e.db != nil && len(result) == 0 {
		rows, err := e.db.Query(`SELECT id, trace_id, name, conf, integ, source, parents, created_at
			FROM ifc_variables WHERE trace_id=? ORDER BY created_at`, traceID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var v IFCVariable
				var conf, integ int
				var parentsJSON string
				var createdAt string
				if rows.Scan(&v.ID, &v.TraceID, &v.Name, &conf, &integ, &v.Source, &parentsJSON, &createdAt) == nil {
					v.Label.Confidentiality = IFCLevel(conf)
					v.Label.Integrity = IntegLevel(integ)
					json.Unmarshal([]byte(parentsJSON), &v.Parents)
					if v.Parents == nil {
						v.Parents = []string{}
					}
					v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
					result = append(result, v)
				}
			}
		}
	}

	return result
}

// GetAllVariables returns variables across all traces (from DB), ordered by created_at desc
func (e *IFCEngine) GetAllVariables(limit int) []IFCVariable {
	if e.db == nil {
		return []IFCVariable{}
	}
	rows, err := e.db.Query(`SELECT id, trace_id, name, conf, integ, source, parents, created_at
		FROM ifc_variables ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return []IFCVariable{}
	}
	defer rows.Close()
	var result []IFCVariable
	for rows.Next() {
		var v IFCVariable
		var conf, integ int
		var parentsStr, ts string
		if rows.Scan(&v.ID, &v.TraceID, &v.Name, &conf, &integ, &v.Source, &parentsStr, &ts) == nil {
			v.Label.Confidentiality = IFCLevel(conf)
			v.Label.Integrity = IntegLevel(integ)
			v.CreatedAt, _ = time.Parse(time.RFC3339, ts)
			if parentsStr != "" {
				json.Unmarshal([]byte(parentsStr), &v.Parents)
			}
			if v.Parents == nil {
				v.Parents = []string{}
			}
			result = append(result, v)
		}
	}
	if result == nil {
		return []IFCVariable{}
	}
	return result
}

func (e *IFCEngine) GetViolations(limit int) []IFCViolation {
	if e.db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := e.db.Query(`SELECT id, trace_id, type, variable, var_conf, var_integ, req_conf, req_integ, tool, action, timestamp
		FROM ifc_violations ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []IFCViolation
	for rows.Next() {
		var v IFCViolation
		var varConf, varInteg, reqConf, reqInteg int
		var ts string
		if rows.Scan(&v.ID, &v.TraceID, &v.Type, &v.Variable, &varConf, &varInteg, &reqConf, &reqInteg, &v.Tool, &v.Action, &ts) == nil {
			v.VarLabel = IFCLabel{Confidentiality: IFCLevel(varConf), Integrity: IntegLevel(varInteg)}
			v.Required = IFCLabel{Confidentiality: IFCLevel(reqConf), Integrity: IntegLevel(reqInteg)}
			v.Timestamp, _ = time.Parse(time.RFC3339, ts)
			result = append(result, v)
		}
	}
	return result
}

func (e *IFCEngine) GetStats() IFCStats {
	e.mu.RLock()
	srcCount := len(e.sourceRules)
	toolCount := len(e.toolRequirements)

	// Count active traces
	activeTraces := int64(len(e.variables))

	// Label distribution
	labelDist := make(map[string]int64)
	for _, traceVars := range e.variables {
		for _, v := range traceVars {
			key := fmt.Sprintf("%s/%s", v.Label.Confidentiality, v.Label.Integrity)
			labelDist[key]++
		}
	}
	e.mu.RUnlock()

	// Also get active traces from DB if memory is empty
	if activeTraces == 0 && e.db != nil {
		row := e.db.QueryRow(`SELECT COUNT(DISTINCT trace_id) FROM ifc_variables`)
		row.Scan(&activeTraces)
	}

	return IFCStats{
		TotalVariables:    atomic.LoadInt64(&e.totalVariables),
		ActiveTraces:      activeTraces,
		TotalViolations:   atomic.LoadInt64(&e.totalViolations),
		ConfViolations:    atomic.LoadInt64(&e.confViolations),
		IntegViolations:   atomic.LoadInt64(&e.integViolations),
		TotalBlocked:      atomic.LoadInt64(&e.totalBlocked),
		TotalWarned:       atomic.LoadInt64(&e.totalWarned),
		TotalHidden:       atomic.LoadInt64(&e.totalHidden),
		TotalDOE:          atomic.LoadInt64(&e.totalDOE),
		SourceRuleCount:   srcCount,
		ToolReqCount:      toolCount,
		LabelDistribution: labelDist,
	}
}

func (e *IFCEngine) GetConfig() IFCConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

func (e *IFCEngine) UpdateConfig(cfg IFCConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = cfg
}

// ============================================================
// CRUD for source rules
// ============================================================

func (e *IFCEngine) ListSourceRules() []IFCSourceRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var rules []IFCSourceRule
	for src, label := range e.sourceRules {
		rules = append(rules, IFCSourceRule{Source: src, Label: label})
	}
	return rules
}

func (e *IFCEngine) AddSourceRule(rule IFCSourceRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sourceRules[rule.Source]; exists {
		return fmt.Errorf("source rule %q already exists", rule.Source)
	}

	e.sourceRules[rule.Source] = rule.Label

	if e.db != nil {
		e.db.Exec(`INSERT OR REPLACE INTO ifc_source_rules (source, conf, integ) VALUES (?, ?, ?)`,
			rule.Source, int(rule.Label.Confidentiality), int(rule.Label.Integrity))
	}
	return nil
}

func (e *IFCEngine) UpdateSourceRule(source string, label IFCLabel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sourceRules[source]; !exists {
		return fmt.Errorf("source rule %q not found", source)
	}

	e.sourceRules[source] = label

	if e.db != nil {
		e.db.Exec(`UPDATE ifc_source_rules SET conf=?, integ=? WHERE source=?`,
			int(label.Confidentiality), int(label.Integrity), source)
	}
	return nil
}

func (e *IFCEngine) DeleteSourceRule(source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sourceRules[source]; !exists {
		return fmt.Errorf("source rule %q not found", source)
	}

	delete(e.sourceRules, source)

	if e.db != nil {
		e.db.Exec(`DELETE FROM ifc_source_rules WHERE source=?`, source)
	}
	return nil
}

// ============================================================
// CRUD for tool requirements
// ============================================================

func (e *IFCEngine) ListToolRequirements() []IFCToolRequirement {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var reqs []IFCToolRequirement
	for _, req := range e.toolRequirements {
		reqs = append(reqs, req)
	}
	return reqs
}

func (e *IFCEngine) AddToolRequirement(req IFCToolRequirement) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.toolRequirements[req.Tool]; exists {
		return fmt.Errorf("tool requirement %q already exists", req.Tool)
	}

	e.toolRequirements[req.Tool] = req

	if e.db != nil {
		e.db.Exec(`INSERT OR REPLACE INTO ifc_tool_requirements (tool, required_integ, max_conf) VALUES (?, ?, ?)`,
			req.Tool, int(req.RequiredInteg), int(req.MaxConf))
	}
	return nil
}

func (e *IFCEngine) UpdateToolRequirement(tool string, req IFCToolRequirement) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.toolRequirements[tool]; !exists {
		return fmt.Errorf("tool requirement %q not found", tool)
	}

	req.Tool = tool
	e.toolRequirements[tool] = req

	if e.db != nil {
		e.db.Exec(`UPDATE ifc_tool_requirements SET required_integ=?, max_conf=? WHERE tool=?`,
			int(req.RequiredInteg), int(req.MaxConf), tool)
	}
	return nil
}

func (e *IFCEngine) DeleteToolRequirement(tool string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.toolRequirements[tool]; !exists {
		return fmt.Errorf("tool requirement %q not found", tool)
	}

	delete(e.toolRequirements, tool)

	if e.db != nil {
		e.db.Exec(`DELETE FROM ifc_tool_requirements WHERE tool=?`, tool)
	}
	return nil
}

// ============================================================
// v26.2: extractFieldNames — 从 JSON 字符串提取顶层 key
// ============================================================

func extractFieldNames(jsonStr string) []string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(jsonStr), &m) != nil {
		return nil
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys) // 确保顺序稳定
	return keys
}