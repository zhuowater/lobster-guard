// ifc_quarantine.go — IFC Quarantine LLM 路由 (v26.1)
// 管理被污染数据的隔离处理：路由到隔离LLM，去污后返回
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// IFCQuarantine 管理隔离LLM路由
type IFCQuarantine struct {
	engine *IFCEngine
	pool   *UpstreamPool // 上游连接池（获取quarantine上游）
	mu     sync.RWMutex
	stats  IFCQuarantineStats
	sessions map[string]*QuarantineSession // sessionID → session
}

// IFCQuarantineStats 隔离统计
type IFCQuarantineStats struct {
	TotalRouted     int64 `json:"total_routed"`      // 被路由到隔离LLM的总次数
	TotalDepurified int64 `json:"total_depurified"`   // 去污成功次数
	TotalFailed     int64 `json:"total_failed"`       // 隔离处理失败次数
	ActiveSessions  int64 `json:"active_sessions"`    // 当前活跃隔离会话
}

// QuarantineSession 隔离会话
type QuarantineSession struct {
	TraceID   string    `json:"trace_id"`
	SessionID string    `json:"session_id"`
	InputVars []string  `json:"input_vars"`    // 被污染的输入变量ID
	Status    string    `json:"status"`         // pending / processing / completed / failed
	OutputVar string    `json:"output_var"`     // 去污后的输出变量ID
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Upstream  string    `json:"upstream"`       // 隔离上游 URL
}

// NewIFCQuarantine 创建隔离管理器
func NewIFCQuarantine(engine *IFCEngine, pool *UpstreamPool) *IFCQuarantine {
	return &IFCQuarantine{
		engine:   engine,
		pool:     pool,
		sessions: make(map[string]*QuarantineSession),
	}
}

// ShouldRoute 检查是否需要路由到隔离LLM
// 条件: quarantine_enabled && 任何输入变量 integ == TAINT
func (q *IFCQuarantine) ShouldRoute(traceID string, inputVarIDs []string) bool {
	if q.engine == nil || !q.engine.config.QuarantineEnabled {
		return false
	}

	q.engine.mu.RLock()
	defer q.engine.mu.RUnlock()

	traceVars := q.engine.variables[traceID]
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

// Route 执行隔离路由
// 1. 创建 QuarantineSession
// 2. 选择 quarantine 上游
// 3. 返回上游 URL (调用方负责实际代理)
func (q *IFCQuarantine) Route(traceID string, inputVarIDs []string) (upstreamURL string, sessionID string, err error) {
	if q.engine == nil {
		return "", "", fmt.Errorf("quarantine engine not initialized")
	}

	// 确定隔离上游 URL
	quarantineUpstream := q.engine.config.QuarantineUpstream
	if quarantineUpstream == "" {
		// 尝试从 pool 获取带 quarantine tag 的上游
		if q.pool != nil {
			quarantineUpstream = q.findQuarantineUpstream()
		}
		if quarantineUpstream == "" {
			return "", "", fmt.Errorf("no quarantine upstream configured")
		}
	}

	// 生成 sessionID
	sessionID = generateIFCID()

	// 创建 session
	session := &QuarantineSession{
		TraceID:   traceID,
		SessionID: sessionID,
		InputVars: inputVarIDs,
		Status:    "processing",
		StartTime: time.Now().UTC(),
		Upstream:  quarantineUpstream,
	}

	q.mu.Lock()
	q.sessions[sessionID] = session
	q.mu.Unlock()

	// 更新统计
	atomic.AddInt64(&q.stats.TotalRouted, 1)
	atomic.AddInt64(&q.stats.ActiveSessions, 1)

	return quarantineUpstream, sessionID, nil
}

// CompleteSession 标记隔离处理完成，注册去污后的变量
// outputContent 是隔离LLM的输出
// 去污规则: 输出变量 integ = MEDIUM (从 TAINT 提升)
func (q *IFCQuarantine) CompleteSession(traceID, sessionID, outputContent string) *IFCVariable {
	q.mu.Lock()
	session, ok := q.sessions[sessionID]
	if !ok {
		q.mu.Unlock()
		return nil
	}
	session.Status = "completed"
	session.EndTime = time.Now().UTC()
	q.mu.Unlock()

	atomic.AddInt64(&q.stats.TotalDepurified, 1)
	atomic.AddInt64(&q.stats.ActiveSessions, -1)

	// 注册去污后的输出变量，integ = MEDIUM (从 TAINT 提升)
	if q.engine != nil {
		// 先确定最高机密性（从输入变量中继承）
		var maxConf IFCLevel
		q.engine.mu.RLock()
		traceVars := q.engine.variables[traceID]
		for _, vid := range session.InputVars {
			if traceVars != nil {
				if v, ok := traceVars[vid]; ok {
					if v.Label.Confidentiality > maxConf {
						maxConf = v.Label.Confidentiality
					}
				}
			}
		}
		q.engine.mu.RUnlock()

		// 创建去污后变量
		v := &IFCVariable{
			ID:        generateIFCID(),
			TraceID:   traceID,
			Name:      "quarantine_output",
			Label:     IFCLabel{Confidentiality: maxConf, Integrity: IntegMedium},
			Source:    "quarantine",
			Parents:   session.InputVars,
			CreatedAt: time.Now().UTC(),
		}

		q.engine.mu.Lock()
		if q.engine.variables[traceID] == nil {
			q.engine.variables[traceID] = make(map[string]*IFCVariable)
		}
		q.engine.variables[traceID][v.ID] = v
		q.engine.mu.Unlock()

		atomic.AddInt64(&q.engine.totalVariables, 1)

		// 更新 session
		q.mu.Lock()
		session.OutputVar = v.ID
		q.mu.Unlock()

		return v
	}
	return nil
}

// FailSession 标记隔离处理失败
func (q *IFCQuarantine) FailSession(sessionID string) {
	q.mu.Lock()
	session, ok := q.sessions[sessionID]
	if ok {
		session.Status = "failed"
		session.EndTime = time.Now().UTC()
	}
	q.mu.Unlock()

	if ok {
		atomic.AddInt64(&q.stats.TotalFailed, 1)
		atomic.AddInt64(&q.stats.ActiveSessions, -1)
	}
}

// GetSessions 获取隔离会话列表
func (q *IFCQuarantine) GetSessions(limit int) []QuarantineSession {
	if limit <= 0 {
		limit = 20
	}

	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []QuarantineSession
	for _, s := range q.sessions {
		result = append(result, *s)
		if len(result) >= limit {
			break
		}
	}

	// 按 start_time 降序排列
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].StartTime.After(result[i].StartTime) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetStats 获取统计
func (q *IFCQuarantine) GetStats() IFCQuarantineStats {
	return IFCQuarantineStats{
		TotalRouted:     atomic.LoadInt64(&q.stats.TotalRouted),
		TotalDepurified: atomic.LoadInt64(&q.stats.TotalDepurified),
		TotalFailed:     atomic.LoadInt64(&q.stats.TotalFailed),
		ActiveSessions:  atomic.LoadInt64(&q.stats.ActiveSessions),
	}
}

// findQuarantineUpstream 从 pool 中查找 quarantine 上游
func (q *IFCQuarantine) findQuarantineUpstream() string {
	if q.pool == nil {
		return ""
	}
	// 查找配置中指定的 quarantine upstream ID
	upstreamID := q.engine.config.QuarantineUpstream
	if upstreamID == "" {
		return ""
	}

	up, ok := q.pool.GetUpstream(upstreamID)
	if ok && up != nil {
		addr := up.Address
		if up.Port > 0 {
			addr = fmt.Sprintf("%s:%d", addr, up.Port)
		}
		return addr
	}
	return upstreamID // 如果不在 pool 中，直接返回配置的 URL
}
