// capability.go - CapabilityEngine: data-level capability tagging system
// lobster-guard v25.1
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type CapConfig struct {
	Enabled            bool    `json:"enabled" yaml:"enabled"`
	DefaultPolicy      string  `json:"default_policy" yaml:"default_policy"`
	EnforceIntersect   bool    `json:"enforce_intersect" yaml:"enforce_intersect"`
	PropagateLabels    bool    `json:"propagate_labels" yaml:"propagate_labels"`
	MaxContextsPerUser int     `json:"max_contexts_per_user" yaml:"max_contexts_per_user"`
	RetentionDays      int     `json:"retention_days" yaml:"retention_days"`
	TrustThreshold     float64 `json:"trust_threshold" yaml:"trust_threshold"`
	AuditAll           bool    `json:"audit_all" yaml:"audit_all"`
}

type CapLabel struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Level     string `json:"level"`
	Granted   bool   `json:"granted"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type CapDataItem struct {
	DataID     string     `json:"data_id"`
	Content    string     `json:"content"`
	Source     string     `json:"source"`
	Labels     []CapLabel `json:"labels"`
	TrustScore float64   `json:"trust_score"`
	CreatedAt  string     `json:"created_at"`
}

type CapContext struct {
	ID            string                  `json:"id"`
	TraceID       string                  `json:"trace_id"`
	UserID        string                  `json:"user_id"`
	Status        string                  `json:"status"`
	UserCaps      []CapLabel              `json:"user_caps"`
	ToolResults   []CapToolResult         `json:"tool_results"`
	Intersections []CapIntersection       `json:"intersections"`
	Evaluations   []CapEvaluation         `json:"evaluations"`
	DataItems     map[string]*CapDataItem `json:"data_items"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
}

type CapToolResult struct {
	ToolName   string     `json:"tool_name"`
	DataID     string     `json:"data_id"`
	RawCaps    []CapLabel `json:"raw_caps"`
	MappedCaps []CapLabel `json:"mapped_caps"`
	Timestamp  string     `json:"timestamp"`
}

type CapIntersection struct {
	SourceDataIDs []string   `json:"source_data_ids"`
	ResultLabels  []CapLabel `json:"result_labels"`
	Context       string     `json:"context"`
	Timestamp     string     `json:"timestamp"`
}

type CapEvaluation struct {
	ID         string     `json:"id"`
	TraceID    string     `json:"trace_id"`
	DataID     string     `json:"data_id"`
	Action     string     `json:"action"`
	ToolName   string     `json:"tool_name"`
	Decision   string     `json:"decision"`
	Reason     string     `json:"reason"`
	Labels     []CapLabel `json:"labels"`
	TrustScore float64    `json:"trust_score"`
	Timestamp  string     `json:"timestamp"`
}

type CapToolMapping struct {
	ToolName     string   `json:"tool_name"`
	Category     string   `json:"category"`
	DefaultLevel string   `json:"default_level"`
	AllowedCaps  []string `json:"allowed_caps"`
	DeniedCaps   []string `json:"denied_caps"`
	TrustFactor  float64  `json:"trust_factor"`
}

type CapStats struct {
	TotalContexts     int64                    `json:"total_contexts"`
	ActiveContexts    int64                    `json:"active_contexts"`
	TotalEvaluations  int64                    `json:"total_evaluations"`
	DenyCount         int64                    `json:"deny_count"`
	WarnCount         int64                    `json:"warn_count"`
	AllowCount        int64                    `json:"allow_count"`
	AvgTrustScore     float64                  `json:"avg_trust_score"`
	ToolMappingCount  int                      `json:"tool_mapping_count"`
	TopDeniedTools    []map[string]interface{} `json:"top_denied_tools"`
	RecentEvaluations []CapEvaluation          `json:"recent_evaluations"`
	LabelBreakdown    map[string]int64         `json:"label_breakdown"`
}

type CapabilityEngine struct {
	db           *sql.DB
	config       CapConfig
	mu           sync.RWMutex
	contexts     map[string]*CapContext
	toolMappings map[string]*CapToolMapping
}

func capGenID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var defaultCapConfig = CapConfig{
	Enabled: true, DefaultPolicy: "warn", EnforceIntersect: true,
	PropagateLabels: true, MaxContextsPerUser: 100, RetentionDays: 30,
	TrustThreshold: 0.5, AuditAll: false,
}

func NewCapabilityEngine(db *sql.DB, config CapConfig) *CapabilityEngine {
	if config.DefaultPolicy == "" { config.DefaultPolicy = "warn" }
	if config.MaxContextsPerUser <= 0 { config.MaxContextsPerUser = 100 }
	if config.RetentionDays <= 0 { config.RetentionDays = 30 }
	if config.TrustThreshold <= 0 { config.TrustThreshold = 0.5 }
	ce := &CapabilityEngine{db: db, config: config, contexts: make(map[string]*CapContext), toolMappings: make(map[string]*CapToolMapping)}
	ce.initCapDB()
	ce.loadCapToolMappings()
	ce.seedCapDefaults()
	return ce
}

func (ce *CapabilityEngine) initCapDB() {
	stmts := []string{
		"CREATE TABLE IF NOT EXISTS cap_contexts (id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, user_id TEXT DEFAULT '', status TEXT DEFAULT 'active', user_caps TEXT DEFAULT '[]', tool_results TEXT DEFAULT '[]', intersections TEXT DEFAULT '[]', data_items TEXT DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)",
		"CREATE INDEX IF NOT EXISTS idx_cap_ctx_trace ON cap_contexts(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_cap_ctx_status ON cap_contexts(status)",
		"CREATE TABLE IF NOT EXISTS cap_evaluations (id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, data_id TEXT DEFAULT '', action TEXT NOT NULL, tool_name TEXT DEFAULT '', decision TEXT NOT NULL, reason TEXT DEFAULT '', labels TEXT DEFAULT '[]', trust_score REAL DEFAULT 0, timestamp TEXT NOT NULL)",
		"CREATE INDEX IF NOT EXISTS idx_cap_eval_trace ON cap_evaluations(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_cap_eval_decision ON cap_evaluations(decision)",
		"CREATE TABLE IF NOT EXISTS cap_tool_mappings (tool_name TEXT PRIMARY KEY, category TEXT DEFAULT '', default_level TEXT DEFAULT 'none', allowed_caps TEXT DEFAULT '[]', denied_caps TEXT DEFAULT '[]', trust_factor REAL DEFAULT 0.0, updated_at TEXT NOT NULL)",
	}
	for _, s := range stmts {
		if _, err := ce.db.Exec(s); err != nil { log.Printf("[CapabilityEngine] initDB: %v", err) }
	}
}

func (ce *CapabilityEngine) loadCapToolMappings() {
	rows, err := ce.db.Query("SELECT tool_name,category,default_level,allowed_caps,denied_caps,trust_factor FROM cap_tool_mappings")
	if err != nil { return }
	defer rows.Close()
	for rows.Next() {
		var m CapToolMapping; var acJ, dcJ string
		if rows.Scan(&m.ToolName, &m.Category, &m.DefaultLevel, &acJ, &dcJ, &m.TrustFactor) == nil {
			json.Unmarshal([]byte(acJ), &m.AllowedCaps); json.Unmarshal([]byte(dcJ), &m.DeniedCaps)
			if m.AllowedCaps == nil { m.AllowedCaps = []string{} }
			if m.DeniedCaps == nil { m.DeniedCaps = []string{} }
			ce.toolMappings[m.ToolName] = &m
		}
	}
}

func (ce *CapabilityEngine) seedCapDefaults() {
	defs := []CapToolMapping{
		{ToolName: "web_search", Category: "web", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.3},
		{ToolName: "web_fetch", Category: "web", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.2},
		{ToolName: "http_get", Category: "web", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.3},
		{ToolName: "read_file", Category: "file", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"execute", "admin"}, TrustFactor: 0.4},
		{ToolName: "write_file", Category: "file", DefaultLevel: "write", AllowedCaps: []string{"read", "write"}, DeniedCaps: []string{"admin"}, TrustFactor: 0.5},
		{ToolName: "list_files", Category: "file", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.4},
		{ToolName: "send_email", Category: "email", DefaultLevel: "write", AllowedCaps: []string{"read", "write"}, DeniedCaps: []string{"admin"}, TrustFactor: 0.6},
		{ToolName: "read_email", Category: "email", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.5},
		{ToolName: "search_email", Category: "email", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.4},
		{ToolName: "run_code", Category: "code", DefaultLevel: "none", AllowedCaps: []string{}, DeniedCaps: []string{"write", "execute", "admin"}, TrustFactor: 0.1},
		{ToolName: "exec_command", Category: "code", DefaultLevel: "none", AllowedCaps: []string{}, DeniedCaps: []string{"write", "execute", "admin"}, TrustFactor: 0.1},
		{ToolName: "query_db", Category: "query", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"write", "admin"}, TrustFactor: 0.5},
		{ToolName: "query_api", Category: "query", DefaultLevel: "read", AllowedCaps: []string{"read"}, DeniedCaps: []string{"admin"}, TrustFactor: 0.4},
		{ToolName: "admin_action", Category: "admin", DefaultLevel: "none", AllowedCaps: []string{}, DeniedCaps: []string{"read", "write", "execute", "admin"}, TrustFactor: 0.0},
		{ToolName: "manage_users", Category: "admin", DefaultLevel: "none", AllowedCaps: []string{}, DeniedCaps: []string{"read", "write", "execute", "admin"}, TrustFactor: 0.0},
		{ToolName: "deploy", Category: "admin", DefaultLevel: "none", AllowedCaps: []string{}, DeniedCaps: []string{"read", "write", "execute", "admin"}, TrustFactor: 0.0},
	}
	ce.mu.Lock(); defer ce.mu.Unlock()
	for _, d := range defs {
		if _, ok := ce.toolMappings[d.ToolName]; !ok {
			dd := d; ce.toolMappings[d.ToolName] = &dd
			acJ, _ := json.Marshal(d.AllowedCaps); dcJ, _ := json.Marshal(d.DeniedCaps)
			ce.db.Exec("INSERT OR IGNORE INTO cap_tool_mappings(tool_name,category,default_level,allowed_caps,denied_caps,trust_factor,updated_at) VALUES(?,?,?,?,?,?,?)",
				d.ToolName, d.Category, d.DefaultLevel, string(acJ), string(dcJ), d.TrustFactor, time.Now().UTC().Format(time.RFC3339))
		}
	}
}

func capDefaultUserCaps() []CapLabel {
	return []CapLabel{
		{Name: "read", Source: "user_input", Level: "read", Granted: true},
		{Name: "write", Source: "user_input", Level: "write", Granted: true},
		{Name: "execute", Source: "user_input", Level: "execute", Granted: true},
	}
}

func (ce *CapabilityEngine) InitContext(traceID, userID string, userCaps []CapLabel) *CapContext {
	now := time.Now().UTC().Format(time.RFC3339)
	if userCaps == nil { userCaps = capDefaultUserCaps() }
	ctx := &CapContext{ID: capGenID(), TraceID: traceID, UserID: userID, Status: "active",
		UserCaps: userCaps, ToolResults: []CapToolResult{}, Intersections: []CapIntersection{},
		Evaluations: []CapEvaluation{}, DataItems: make(map[string]*CapDataItem), CreatedAt: now, UpdatedAt: now}
	uid := "user_input_" + capGenID()
	ctx.DataItems[uid] = &CapDataItem{DataID: uid, Source: "user_input", Labels: userCaps, TrustScore: 1.0, CreatedAt: now}
	ce.mu.Lock(); ce.contexts[traceID] = ctx; ce.mu.Unlock()
	ce.capPersistCtx(ctx)
	return ctx
}

func (ce *CapabilityEngine) RegisterToolResult(traceID, toolName, dataID string) *CapToolResult {
	ce.mu.Lock(); ctx := ce.contexts[traceID]; ce.mu.Unlock()
	if ctx == nil { return nil }
	now := time.Now().UTC().Format(time.RFC3339)
	rawCaps := []CapLabel{{Name: "none", Source: "tool_result", Level: "none", Granted: false}}
	var mappedCaps []CapLabel
	ce.mu.RLock(); mp := ce.toolMappings[toolName]; ce.mu.RUnlock()
	if mp != nil {
		for _, ac := range mp.AllowedCaps { mappedCaps = append(mappedCaps, CapLabel{Name: ac, Source: "tool_mapping", Level: ac, Granted: true}) }
	}
	if mappedCaps == nil { mappedCaps = []CapLabel{} }
	tr := CapToolResult{ToolName: toolName, DataID: dataID, RawCaps: rawCaps, MappedCaps: mappedCaps, Timestamp: now}
	tf := 0.0; if mp != nil { tf = mp.TrustFactor }
	ce.mu.Lock()
	ctx.ToolResults = append(ctx.ToolResults, tr)
	ctx.DataItems[dataID] = &CapDataItem{DataID: dataID, Source: "tool_result:" + toolName, Labels: rawCaps, TrustScore: tf, CreatedAt: now}
	ctx.UpdatedAt = now; ce.mu.Unlock()
	ce.capPersistCtx(ctx)
	return &tr
}

func (ce *CapabilityEngine) Evaluate(traceID, dataID, action, toolName string) *CapEvaluation {
	ce.mu.RLock(); ctx := ce.contexts[traceID]; ce.mu.RUnlock()
	eval := &CapEvaluation{ID: capGenID(), TraceID: traceID, DataID: dataID, Action: action, ToolName: toolName, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	if ctx == nil {
		eval.Decision = ce.config.DefaultPolicy; eval.Reason = "no context"; eval.Labels = []CapLabel{}
		ce.capPersistEval(eval); return eval
	}
	ce.mu.RLock(); di := ctx.DataItems[dataID]; ce.mu.RUnlock()
	if di == nil {
		eval.Decision = "allow"; eval.Reason = "data not tracked"; eval.TrustScore = 1.0; eval.Labels = []CapLabel{}
		ce.capPersistEval(eval); return eval
	}
	eval.Labels = di.Labels; eval.TrustScore = di.TrustScore
	userHas := false
	for _, uc := range ctx.UserCaps {
		if uc.Granted && (uc.Level == action || uc.Level == "admin") { userHas = true; break }
	}
	if di.Source == "user_input" {
		eval.Decision = "allow"; eval.Reason = "user input, full cap"; eval.TrustScore = 1.0
	} else if strings.HasPrefix(di.Source, "tool_result:") {
		if !userHas {
			eval.Decision = "deny"; eval.Reason = fmt.Sprintf("user lacks '%s' cap", action)
		} else {
			ce.mu.RLock(); m := ce.toolMappings[toolName]; ce.mu.RUnlock()
			if m != nil {
				denied := false
				for _, dc := range m.DeniedCaps {
					if dc == action { eval.Decision = "deny"; eval.Reason = fmt.Sprintf("tool '%s' denies '%s'", toolName, action); denied = true; break }
				}
				if !denied {
					if di.TrustScore < ce.config.TrustThreshold { eval.Decision = "warn"; eval.Reason = fmt.Sprintf("trust %.2f < threshold %.2f", di.TrustScore, ce.config.TrustThreshold)
					} else { eval.Decision = "allow"; eval.Reason = "tool mapping allows" }
				}
			} else { eval.Decision = ce.config.DefaultPolicy; eval.Reason = fmt.Sprintf("no mapping for '%s'", toolName) }
		}
	} else if di.Source == "llm_summary" {
		if ce.config.EnforceIntersect { eval.Decision = ce.capEvalIntersect(di, action); eval.Reason = "LLM summary intersection"
		} else { eval.Decision = "allow"; eval.Reason = "intersection disabled" }
	} else { eval.Decision = ce.config.DefaultPolicy; eval.Reason = "unknown source" }
	if eval.Decision == "" { eval.Decision = ce.config.DefaultPolicy }
	ce.mu.Lock()
	ctx.Evaluations = append(ctx.Evaluations, *eval)
	if eval.Decision == "deny" { ctx.Status = "violated" }
	ctx.UpdatedAt = time.Now().UTC().Format(time.RFC3339); ce.mu.Unlock()
	ce.capPersistEval(eval); ce.capPersistCtx(ctx)
	return eval
}

func (ce *CapabilityEngine) capEvalIntersect(item *CapDataItem, action string) string {
	for _, l := range item.Labels { if l.Granted && (l.Level == action || l.Level == "admin") { return "allow" } }
	return "deny"
}

func (ce *CapabilityEngine) RegisterLLMSummary(traceID, dataID string, sourceDataIDs []string) {
	ce.mu.Lock(); ctx := ce.contexts[traceID]; ce.mu.Unlock()
	if ctx == nil { return }
	now := time.Now().UTC().Format(time.RFC3339)
	intersected := ce.capComputeIntersect(ctx, sourceDataIDs)
	minTrust := 1.0
	ce.mu.RLock()
	for _, sid := range sourceDataIDs { if item, ok := ctx.DataItems[sid]; ok && item.TrustScore < minTrust { minTrust = item.TrustScore } }
	ce.mu.RUnlock()
	inter := CapIntersection{SourceDataIDs: sourceDataIDs, ResultLabels: intersected, Context: "llm_summary", Timestamp: now}
	ce.mu.Lock()
	ctx.Intersections = append(ctx.Intersections, inter)
	ctx.DataItems[dataID] = &CapDataItem{DataID: dataID, Source: "llm_summary", Labels: intersected, TrustScore: minTrust, CreatedAt: now}
	ctx.UpdatedAt = now; ce.mu.Unlock()
	ce.capPersistCtx(ctx)
}

func (ce *CapabilityEngine) capComputeIntersect(ctx *CapContext, dataIDs []string) []CapLabel {
	if len(dataIDs) == 0 { return []CapLabel{} }
	ce.mu.RLock()
	var sets [][]CapLabel
	for _, did := range dataIDs { if item, ok := ctx.DataItems[did]; ok { sets = append(sets, item.Labels) } }
	ce.mu.RUnlock()
	if len(sets) == 0 { return []CapLabel{} }
	counts := map[string]int{}
	for _, labels := range sets { seen := map[string]bool{}; for _, l := range labels { if l.Granted && !seen[l.Level] { counts[l.Level]++; seen[l.Level] = true } } }
	total := len(sets); var result []CapLabel
	for lv, c := range counts { if c == total { result = append(result, CapLabel{Name: lv, Source: "intersection", Level: lv, Granted: true}) } }
	if result == nil { result = []CapLabel{} }; return result
}

func (ce *CapabilityEngine) GetContext(traceID string) *CapContext {
	ce.mu.RLock(); ctx := ce.contexts[traceID]; ce.mu.RUnlock()
	if ctx != nil { return ctx }
	return ce.capLoadCtxDB(traceID)
}

func (ce *CapabilityEngine) CompleteContext(traceID string) {
	ce.mu.Lock(); ctx := ce.contexts[traceID]
	if ctx != nil { ctx.Status = "completed"; ctx.UpdatedAt = time.Now().UTC().Format(time.RFC3339) }
	ce.mu.Unlock(); if ctx != nil { ce.capPersistCtx(ctx) }
}

func (ce *CapabilityEngine) GetToolMapping(toolName string) *CapToolMapping { ce.mu.RLock(); defer ce.mu.RUnlock(); return ce.toolMappings[toolName] }

func (ce *CapabilityEngine) ListToolMappings() []CapToolMapping {
	ce.mu.RLock(); defer ce.mu.RUnlock()
	out := make([]CapToolMapping, 0, len(ce.toolMappings)); for _, m := range ce.toolMappings { out = append(out, *m) }; return out
}

func (ce *CapabilityEngine) UpdateToolMapping(m CapToolMapping) error {
	if m.ToolName == "" { return fmt.Errorf("tool_name required") }
	if m.Category == "" { return fmt.Errorf("category required") }
	if m.DefaultLevel == "" { m.DefaultLevel = "medium" }
	if m.AllowedCaps == nil { m.AllowedCaps = []string{} }; if m.DeniedCaps == nil { m.DeniedCaps = []string{} }
	if m.TrustFactor < 0 || m.TrustFactor > 1 { m.TrustFactor = 0.5 }
	now := time.Now().UTC().Format(time.RFC3339)
	acJ, _ := json.Marshal(m.AllowedCaps); dcJ, _ := json.Marshal(m.DeniedCaps)
	_, err := ce.db.Exec("INSERT INTO cap_tool_mappings(tool_name,category,default_level,allowed_caps,denied_caps,trust_factor,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tool_name) DO UPDATE SET category=?,default_level=?,allowed_caps=?,denied_caps=?,trust_factor=?,updated_at=?",
		m.ToolName, m.Category, m.DefaultLevel, string(acJ), string(dcJ), m.TrustFactor, now, m.Category, m.DefaultLevel, string(acJ), string(dcJ), m.TrustFactor, now)
	if err != nil { return err }
	ce.mu.Lock(); mm := m; ce.toolMappings[m.ToolName] = &mm; ce.mu.Unlock(); return nil
}

func (ce *CapabilityEngine) DeleteToolMapping(toolName string) error {
	ce.mu.Lock(); if _, ok := ce.toolMappings[toolName]; !ok { ce.mu.Unlock(); return fmt.Errorf("not found: %s", toolName) }
	delete(ce.toolMappings, toolName); ce.mu.Unlock()
	_, err := ce.db.Exec("DELETE FROM cap_tool_mappings WHERE tool_name=?", toolName); return err
}


