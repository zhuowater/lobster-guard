// capability_ext.go - CapabilityEngine: persistence, queries, stats
// lobster-guard v25.1
package main

import "encoding/json"

// capPersistCtx persists a capability context to DB
func (ce *CapabilityEngine) capPersistCtx(ctx *CapContext) {
	ucJ, _ := json.Marshal(ctx.UserCaps)
	trJ, _ := json.Marshal(ctx.ToolResults)
	intJ, _ := json.Marshal(ctx.Intersections)
	diJ, _ := json.Marshal(ctx.DataItems)
	ce.db.Exec(`INSERT INTO cap_contexts(id,trace_id,user_id,status,user_caps,tool_results,intersections,data_items,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=?,user_caps=?,tool_results=?,intersections=?,data_items=?,updated_at=?`,
		ctx.ID, ctx.TraceID, ctx.UserID, ctx.Status, string(ucJ), string(trJ), string(intJ), string(diJ), ctx.CreatedAt, ctx.UpdatedAt,
		ctx.Status, string(ucJ), string(trJ), string(intJ), string(diJ), ctx.UpdatedAt)
}

// capPersistEval persists a capability evaluation to DB
func (ce *CapabilityEngine) capPersistEval(eval *CapEvaluation) {
	labJ, _ := json.Marshal(eval.Labels)
	ce.db.Exec("INSERT OR IGNORE INTO cap_evaluations(id,trace_id,data_id,action,tool_name,decision,reason,labels,trust_score,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?)",
		eval.ID, eval.TraceID, eval.DataID, eval.Action, eval.ToolName, eval.Decision, eval.Reason, string(labJ), eval.TrustScore, eval.Timestamp)
}

// capLoadCtxDB loads a context from DB
func (ce *CapabilityEngine) capLoadCtxDB(traceID string) *CapContext {
	row := ce.db.QueryRow("SELECT id,trace_id,user_id,status,user_caps,tool_results,intersections,data_items,created_at,updated_at FROM cap_contexts WHERE trace_id=? LIMIT 1", traceID)
	var c CapContext
	var ucJ, trJ, intJ, diJ string
	if row.Scan(&c.ID, &c.TraceID, &c.UserID, &c.Status, &ucJ, &trJ, &intJ, &diJ, &c.CreatedAt, &c.UpdatedAt) != nil {
		return nil
	}
	json.Unmarshal([]byte(ucJ), &c.UserCaps)
	json.Unmarshal([]byte(trJ), &c.ToolResults)
	json.Unmarshal([]byte(intJ), &c.Intersections)
	json.Unmarshal([]byte(diJ), &c.DataItems)
	if c.UserCaps == nil {
		c.UserCaps = []CapLabel{}
	}
	if c.ToolResults == nil {
		c.ToolResults = []CapToolResult{}
	}
	if c.Intersections == nil {
		c.Intersections = []CapIntersection{}
	}
	if c.DataItems == nil {
		c.DataItems = map[string]*CapDataItem{}
	}
	c.Evaluations = []CapEvaluation{}
	return &c
}

// QueryEvaluations retrieves evaluations from DB
func (ce *CapabilityEngine) QueryEvaluations(traceID string, limit, offset int) ([]CapEvaluation, int) {
	if limit <= 0 {
		limit = 50
	}
	cq := "SELECT COUNT(*) FROM cap_evaluations"
	var cargs []interface{}
	if traceID != "" {
		cq += " WHERE trace_id=?"
		cargs = append(cargs, traceID)
	}
	var total int
	ce.db.QueryRow(cq, cargs...).Scan(&total)
	q := "SELECT id,trace_id,data_id,action,tool_name,decision,reason,labels,trust_score,timestamp FROM cap_evaluations"
	var args []interface{}
	if traceID != "" {
		q += " WHERE trace_id=?"
		args = append(args, traceID)
	}
	q += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := ce.db.Query(q, args...)
	if err != nil {
		return []CapEvaluation{}, total
	}
	defer rows.Close()
	var evals []CapEvaluation
	for rows.Next() {
		var e CapEvaluation
		var labJ string
		if rows.Scan(&e.ID, &e.TraceID, &e.DataID, &e.Action, &e.ToolName, &e.Decision, &e.Reason, &labJ, &e.TrustScore, &e.Timestamp) == nil {
			json.Unmarshal([]byte(labJ), &e.Labels)
			if e.Labels == nil {
				e.Labels = []CapLabel{}
			}
			evals = append(evals, e)
		}
	}
	if evals == nil {
		evals = []CapEvaluation{}
	}
	return evals, total
}

// GetStats returns aggregate statistics
func (ce *CapabilityEngine) GetStats() CapStats {
	s := CapStats{
		TopDeniedTools:    []map[string]interface{}{},
		RecentEvaluations: []CapEvaluation{},
		LabelBreakdown:    map[string]int64{},
	}
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_contexts").Scan(&s.TotalContexts)
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_contexts WHERE status='active'").Scan(&s.ActiveContexts)
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_evaluations").Scan(&s.TotalEvaluations)
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_evaluations WHERE decision='deny'").Scan(&s.DenyCount)
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_evaluations WHERE decision='warn'").Scan(&s.WarnCount)
	ce.db.QueryRow("SELECT COUNT(*) FROM cap_evaluations WHERE decision='allow'").Scan(&s.AllowCount)
	ce.db.QueryRow("SELECT COALESCE(AVG(trust_score),0) FROM cap_evaluations").Scan(&s.AvgTrustScore)
	ce.mu.RLock()
	s.ToolMappingCount = len(ce.toolMappings)
	ce.mu.RUnlock()

	// Top denied tools
	trows, err := ce.db.Query("SELECT tool_name, COUNT(*) as cnt FROM cap_evaluations WHERE decision='deny' GROUP BY tool_name ORDER BY cnt DESC LIMIT 10")
	if err == nil {
		defer trows.Close()
		for trows.Next() {
			var name string
			var cnt int64
			if trows.Scan(&name, &cnt) == nil {
				s.TopDeniedTools = append(s.TopDeniedTools, map[string]interface{}{"tool": name, "count": cnt})
			}
		}
	}

	// Recent evaluations
	vrows, err := ce.db.Query("SELECT id,trace_id,data_id,action,tool_name,decision,reason,labels,trust_score,timestamp FROM cap_evaluations ORDER BY timestamp DESC LIMIT 10")
	if err == nil {
		defer vrows.Close()
		for vrows.Next() {
			var e CapEvaluation
			var labJ string
			if vrows.Scan(&e.ID, &e.TraceID, &e.DataID, &e.Action, &e.ToolName, &e.Decision, &e.Reason, &labJ, &e.TrustScore, &e.Timestamp) == nil {
				json.Unmarshal([]byte(labJ), &e.Labels)
				if e.Labels == nil {
					e.Labels = []CapLabel{}
				}
				s.RecentEvaluations = append(s.RecentEvaluations, e)
			}
		}
	}

	// Label breakdown
	ce.mu.RLock()
	for _, m := range ce.toolMappings {
		s.LabelBreakdown[m.Category]++
	}
	ce.mu.RUnlock()

	return s
}

// QueryContexts retrieves contexts from DB with pagination
func (ce *CapabilityEngine) QueryContexts(status string, limit, offset int) ([]CapContext, int) {
	if limit <= 0 {
		limit = 50
	}
	cq := "SELECT COUNT(*) FROM cap_contexts"
	var cargs []interface{}
	if status != "" {
		cq += " WHERE status=?"
		cargs = append(cargs, status)
	}
	var total int
	ce.db.QueryRow(cq, cargs...).Scan(&total)
	q := "SELECT id,trace_id,user_id,status,user_caps,tool_results,intersections,data_items,created_at,updated_at FROM cap_contexts"
	var args []interface{}
	if status != "" {
		q += " WHERE status=?"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := ce.db.Query(q, args...)
	if err != nil {
		return []CapContext{}, total
	}
	defer rows.Close()
	var ctxs []CapContext
	for rows.Next() {
		var c CapContext
		var ucJ, trJ, intJ, diJ string
		if rows.Scan(&c.ID, &c.TraceID, &c.UserID, &c.Status, &ucJ, &trJ, &intJ, &diJ, &c.CreatedAt, &c.UpdatedAt) == nil {
			json.Unmarshal([]byte(ucJ), &c.UserCaps)
			json.Unmarshal([]byte(trJ), &c.ToolResults)
			json.Unmarshal([]byte(intJ), &c.Intersections)
			json.Unmarshal([]byte(diJ), &c.DataItems)
			if c.UserCaps == nil {
				c.UserCaps = []CapLabel{}
			}
			if c.ToolResults == nil {
				c.ToolResults = []CapToolResult{}
			}
			if c.Intersections == nil {
				c.Intersections = []CapIntersection{}
			}
			if c.DataItems == nil {
				c.DataItems = map[string]*CapDataItem{}
			}
			c.Evaluations = []CapEvaluation{}
			ctxs = append(ctxs, c)
		}
	}
	if ctxs == nil {
		ctxs = []CapContext{}
	}
	return ctxs, total
}

// GetConfig returns current config
func (ce *CapabilityEngine) GetConfig() CapConfig {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.config
}

// UpdateConfig updates config at runtime
func (ce *CapabilityEngine) UpdateConfig(cfg CapConfig) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	if cfg.DefaultPolicy != "" {
		ce.config.DefaultPolicy = cfg.DefaultPolicy
	}
	if cfg.TrustThreshold > 0 {
		ce.config.TrustThreshold = cfg.TrustThreshold
	}
	if cfg.MaxContextsPerUser > 0 {
		ce.config.MaxContextsPerUser = cfg.MaxContextsPerUser
	}
	ce.config.Enabled = cfg.Enabled
	ce.config.EnforceIntersect = cfg.EnforceIntersect
	ce.config.PropagateLabels = cfg.PropagateLabels
	ce.config.AuditAll = cfg.AuditAll
}

// Stop is a no-op cleanup
func (ce *CapabilityEngine) Stop() {}
