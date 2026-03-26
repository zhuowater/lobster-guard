// plan_compiler_ext.go - PlanCompiler query/stats/config methods
// lobster-guard v25.0
package main

import (
	"encoding/json"
	"sync/atomic"
)

// defaultPlanConfig is the default plan config used in main.go
var defaultPlanConfig = PlanConfig{
	Enabled:         true,
	StrictMode:      false,
	MaxStepsPerPlan: 20,
	DefaultTimeout:  300,
	AutoComplete:    true,
	ViolationAction: "warn",
	MatchThreshold:  0.3,
	MaxActivePlans:  1000,
	RetentionDays:   30,
}

// Stop is a no-op cleanup for PlanCompiler
func (pc *PlanCompiler) Stop() {}

// GetTemplate returns a template by ID
func (pc *PlanCompiler) GetTemplate(id string) *PlanTemplate {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.templates[id]
}

// ListTemplates returns all templates
func (pc *PlanCompiler) ListTemplates() []PlanTemplate {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	out := make([]PlanTemplate, 0, len(pc.templates))
	for _, t := range pc.templates {
		out = append(out, *t)
	}
	return out
}

// GetPlan returns a plan by trace ID (memory or DB)
func (pc *PlanCompiler) GetPlan(traceID string) *ActivePlan {
	pc.mu.RLock()
	if p, ok := pc.activePlans[traceID]; ok {
		pc.mu.RUnlock()
		return p
	}
	pc.mu.RUnlock()
	// try DB
	row := pc.db.QueryRow("SELECT id,trace_id,template_id,template_name,user_query,status,current_step,total_steps,executed_steps,violations,score,started_at,completed_at FROM plan_executions WHERE trace_id=? LIMIT 1", traceID)
	var p ActivePlan
	var esJ, vJ string
	if err := row.Scan(&p.ID, &p.TraceID, &p.TemplateID, &p.TemplateName, &p.UserQuery, &p.Status,
		&p.CurrentStep, &p.TotalSteps, &esJ, &vJ, &p.Score, &p.StartedAt, &p.CompletedAt); err != nil {
		return nil
	}
	json.Unmarshal([]byte(esJ), &p.ExecutedSteps)
	json.Unmarshal([]byte(vJ), &p.Violations)
	if p.ExecutedSteps == nil {
		p.ExecutedSteps = []ExecutedStep{}
	}
	if p.Violations == nil {
		p.Violations = []PlanViolation{}
	}
	return &p
}

// QueryPlans retrieves plan executions from DB with pagination
func (pc *PlanCompiler) QueryPlans(status string, limit, offset int) ([]ActivePlan, int) {
	if limit <= 0 {
		limit = 50
	}

	// count total
	cq := "SELECT COUNT(*) FROM plan_executions"
	var cargs []interface{}
	if status != "" {
		cq += " WHERE status=?"
		cargs = append(cargs, status)
	}
	var total int
	pc.db.QueryRow(cq, cargs...).Scan(&total)

	q := "SELECT id,trace_id,template_id,template_name,user_query,status,current_step,total_steps,executed_steps,violations,score,started_at,completed_at FROM plan_executions"
	var args []interface{}
	if status != "" {
		q += " WHERE status=?"
		args = append(args, status)
	}
	q += " ORDER BY started_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := pc.db.Query(q, args...)
	if err != nil {
		return []ActivePlan{}, total
	}
	defer rows.Close()
	var plans []ActivePlan
	for rows.Next() {
		var p ActivePlan
		var esJ, vJ string
		if err := rows.Scan(&p.ID, &p.TraceID, &p.TemplateID, &p.TemplateName, &p.UserQuery, &p.Status,
			&p.CurrentStep, &p.TotalSteps, &esJ, &vJ, &p.Score, &p.StartedAt, &p.CompletedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(esJ), &p.ExecutedSteps)
		json.Unmarshal([]byte(vJ), &p.Violations)
		if p.ExecutedSteps == nil {
			p.ExecutedSteps = []ExecutedStep{}
		}
		if p.Violations == nil {
			p.Violations = []PlanViolation{}
		}
		plans = append(plans, p)
	}
	if plans == nil {
		plans = []ActivePlan{}
	}
	return plans, total
}

// QueryViolations retrieves violations from DB with pagination
func (pc *PlanCompiler) QueryViolations(traceID string, limit, offset int) ([]PlanViolation, int) {
	if limit <= 0 {
		limit = 50
	}
	cq := "SELECT COUNT(*) FROM plan_violations_log"
	var cargs []interface{}
	if traceID != "" {
		cq += " WHERE trace_id=?"
		cargs = append(cargs, traceID)
	}
	var total int
	pc.db.QueryRow(cq, cargs...).Scan(&total)

	q := "SELECT id,trace_id,plan_id,step_order,tool_name,expected,severity,description,action,timestamp FROM plan_violations_log"
	var args []interface{}
	if traceID != "" {
		q += " WHERE trace_id=?"
		args = append(args, traceID)
	}
	q += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := pc.db.Query(q, args...)
	if err != nil {
		return []PlanViolation{}, total
	}
	defer rows.Close()
	var viols []PlanViolation
	for rows.Next() {
		var v PlanViolation
		if rows.Scan(&v.ID, &v.TraceID, &v.PlanID, &v.StepOrder, &v.ToolName, &v.Expected, &v.Severity, &v.Description, &v.Action, &v.Timestamp) == nil {
			viols = append(viols, v)
		}
	}
	if viols == nil {
		viols = []PlanViolation{}
	}
	return viols, total
}

// GetStats returns aggregate statistics
func (pc *PlanCompiler) GetStats() PlanStats {
	var s PlanStats
	s.TopTemplates = []map[string]interface{}{}
	s.RecentViolations = []PlanViolation{}
	s.CategoryBreakdown = map[string]int64{}

	pc.db.QueryRow("SELECT COUNT(*) FROM plan_executions").Scan(&s.TotalPlans)
	pc.db.QueryRow("SELECT COUNT(*) FROM plan_executions WHERE status='active'").Scan(&s.ActivePlans)
	pc.db.QueryRow("SELECT COUNT(*) FROM plan_executions WHERE status='completed'").Scan(&s.CompletedPlans)
	pc.db.QueryRow("SELECT COUNT(DISTINCT plan_id) FROM plan_violations_log").Scan(&s.ViolatedPlans)
	pc.db.QueryRow("SELECT COUNT(*) FROM plan_violations_log").Scan(&s.TotalViolations)
	pc.db.QueryRow("SELECT COALESCE(AVG(score),0) FROM plan_executions").Scan(&s.AvgScore)

	s.TotalEvaluations = atomic.LoadInt64(&pc.totalEvaluations)
	pc.mu.RLock()
	s.TemplateCount = len(pc.templates)
	for _, t := range pc.templates {
		s.CategoryBreakdown[t.Category]++
	}
	pc.mu.RUnlock()

	trows, err := pc.db.Query("SELECT template_name, COUNT(*) as cnt FROM plan_executions GROUP BY template_name ORDER BY cnt DESC LIMIT 10")
	if err == nil {
		defer trows.Close()
		for trows.Next() {
			var name string
			var cnt int64
			if trows.Scan(&name, &cnt) == nil {
				s.TopTemplates = append(s.TopTemplates, map[string]interface{}{"name": name, "count": cnt})
			}
		}
	}

	vrows, err := pc.db.Query("SELECT id,trace_id,plan_id,step_order,tool_name,expected,severity,description,action,timestamp FROM plan_violations_log ORDER BY timestamp DESC LIMIT 10")
	if err == nil {
		defer vrows.Close()
		for vrows.Next() {
			var v PlanViolation
			if vrows.Scan(&v.ID, &v.TraceID, &v.PlanID, &v.StepOrder, &v.ToolName, &v.Expected, &v.Severity, &v.Description, &v.Action, &v.Timestamp) == nil {
				s.RecentViolations = append(s.RecentViolations, v)
			}
		}
	}
	return s
}

// GetConfig returns the current config
func (pc *PlanCompiler) GetConfig() PlanConfig {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.config
}

// UpdateConfig updates config at runtime
func (pc *PlanCompiler) UpdateConfig(cfg PlanConfig) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if cfg.ViolationAction != "" {
		pc.config.ViolationAction = cfg.ViolationAction
	}
	if cfg.MatchThreshold > 0 {
		pc.config.MatchThreshold = cfg.MatchThreshold
	}
	if cfg.MaxActivePlans > 0 {
		pc.config.MaxActivePlans = cfg.MaxActivePlans
	}
	if cfg.MaxStepsPerPlan > 0 {
		pc.config.MaxStepsPerPlan = cfg.MaxStepsPerPlan
	}
	pc.config.Enabled = cfg.Enabled
	pc.config.StrictMode = cfg.StrictMode
	pc.config.AutoComplete = cfg.AutoComplete
}
