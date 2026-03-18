// api_evolution.go — 对抗性自进化管理 API
// lobster-guard v19.0
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleEvolutionRun POST /api/v1/evolution/run — 手动触发一轮进化
func (api *ManagementAPI) handleEvolutionRun(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "evolution engine not available"})
		return
	}
	report, err := api.evolutionEngine.RunEvolution()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, report)
}

// handleEvolutionStats GET /api/v1/evolution/stats — 进化统计
func (api *ManagementAPI) handleEvolutionStats(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "evolution engine not available"})
		return
	}
	stats, err := api.evolutionEngine.GetStats()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, stats)
}

// handleEvolutionLog GET /api/v1/evolution/log — 进化日志
func (api *ManagementAPI) handleEvolutionLog(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "evolution engine not available"})
		return
	}

	q := r.URL.Query()
	generation := 0
	if g := q.Get("generation"); g != "" {
		generation, _ = strconv.Atoi(g)
	}
	phase := q.Get("phase")
	var bypassed *bool
	if b := q.Get("bypassed"); b != "" {
		bv := b == "true" || b == "1"
		bypassed = &bv
	}
	limit := 50
	if l := q.Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	logs, err := api.evolutionEngine.QueryLog(generation, phase, bypassed, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// handleEvolutionStrategies GET /api/v1/evolution/strategies — 变异策略列表
func (api *ManagementAPI) handleEvolutionStrategies(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "evolution engine not available"})
		return
	}
	strategies := api.evolutionEngine.GetStrategies()
	jsonResponse(w, 200, map[string]interface{}{
		"strategies": strategies,
		"count":      len(strategies),
	})
}

// handleEvolutionConfigGet GET /api/v1/evolution/config — 获取配置
func (api *ManagementAPI) handleEvolutionConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":      false,
			"interval_min": 360,
		})
		return
	}
	config := api.evolutionEngine.GetEvolutionConfig()
	jsonResponse(w, 200, config)
}

// handleEvolutionConfigPut PUT /api/v1/evolution/config — 更新配置
func (api *ManagementAPI) handleEvolutionConfigPut(w http.ResponseWriter, r *http.Request) {
	if api.evolutionEngine == nil {
		jsonResponse(w, 500, map[string]string{"error": "evolution engine not available"})
		return
	}

	var req struct {
		Enabled     *bool `json:"enabled"`
		IntervalMin *int  `json:"interval_min"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Enabled != nil {
		if *req.Enabled {
			intervalMin := 360
			if req.IntervalMin != nil && *req.IntervalMin > 0 {
				intervalMin = *req.IntervalMin
			}
			api.evolutionEngine.StartAutoEvolution(intervalMin)
		} else {
			api.evolutionEngine.StopAutoEvolution()
		}
	} else if req.IntervalMin != nil && *req.IntervalMin > 0 {
		// 只更新间隔：先停再启
		api.evolutionEngine.StopAutoEvolution()
		// 重新创建 stopCh
		api.evolutionEngine.mu.Lock()
		api.evolutionEngine.stopCh = make(chan struct{})
		api.evolutionEngine.mu.Unlock()
		api.evolutionEngine.StartAutoEvolution(*req.IntervalMin)
	}

	config := api.evolutionEngine.GetEvolutionConfig()
	jsonResponse(w, 200, config)
}
