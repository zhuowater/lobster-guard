// api_honeypot_deep.go — 蜜罐深度交互 API
// lobster-guard v19.2
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ============================================================
// GET /api/v1/honeypot/interactions — 交互记录列表
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepInteractions(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"interactions": []interface{}{},
			"total":        0,
			"enabled":      false,
		})
		return
	}

	attackerID := r.URL.Query().Get("attacker_id")
	channel := r.URL.Query().Get("channel")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	interactions := api.honeypotDeep.ListInteractions(attackerID, channel, limit)
	jsonResponse(w, 200, map[string]interface{}{
		"interactions": interactions,
		"total":        len(interactions),
		"enabled":      true,
	})
}

// ============================================================
// GET /api/v1/honeypot/loyalty — 忠诚度曲线排行
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepLoyaltyList(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"curves":  []interface{}{},
			"total":   0,
			"enabled": false,
		})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	curves := api.honeypotDeep.ListLoyaltyCurves(limit)
	if curves == nil {
		curves = []LoyaltyCurve{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"curves":  curves,
		"total":   len(curves),
		"enabled": true,
	})
}

// ============================================================
// GET /api/v1/honeypot/loyalty/:id — 指定攻击者的忠诚度曲线
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepLoyaltyGet(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 404, map[string]string{"error": "honeypot deep engine not enabled"})
		return
	}

	// 从路径提取 attacker_id: /api/v1/honeypot/loyalty/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/honeypot/loyalty/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "attacker_id required"})
		return
	}

	curve := api.honeypotDeep.GetLoyaltyCurve(id)
	if curve == nil || curve.TotalInteractions == 0 {
		jsonResponse(w, 404, map[string]string{"error": "attacker not found"})
		return
	}

	jsonResponse(w, 200, curve)
}

// ============================================================
// POST /api/v1/honeypot/feedback — 手动触发自动回馈
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepFeedback(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 404, map[string]string{"error": "honeypot deep engine not enabled"})
		return
	}

	injected, err := api.honeypotDeep.AutoFeedback()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"status":   "ok",
		"injected": injected,
	})
}

// ============================================================
// POST /api/v1/honeypot/feedback/:id — 指定攻击者回馈
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepFeedbackByID(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 404, map[string]string{"error": "honeypot deep engine not enabled"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/honeypot/feedback/")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "attacker_id required"})
		return
	}

	injected, err := api.honeypotDeep.FeedbackToEvolution(id)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"status":      "ok",
		"attacker_id": id,
		"injected":    injected,
	})
}

// ============================================================
// GET /api/v1/honeypot/deep/stats — 深度蜜罐统计
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepStats(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled": false,
		})
		return
	}

	stats := api.honeypotDeep.GetStats()
	jsonResponse(w, 200, map[string]interface{}{
		"enabled": true,
		"stats":   stats,
	})
}

// ============================================================
// POST /api/v1/honeypot/deep/record — 手动记录交互（调试用）
// ============================================================

func (api *ManagementAPI) handleHoneypotDeepRecord(w http.ResponseWriter, r *http.Request) {
	if api.honeypotDeep == nil {
		jsonResponse(w, 404, map[string]string{"error": "honeypot deep engine not enabled"})
		return
	}

	var req struct {
		AttackerID   string `json:"attacker_id"`
		HoneypotType string `json:"honeypot_type"`
		Channel      string `json:"channel"`
		Payload      string `json:"payload"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.AttackerID == "" {
		jsonResponse(w, 400, map[string]string{"error": "attacker_id required"})
		return
	}

	interaction := api.honeypotDeep.RecordInteraction(req.AttackerID, req.HoneypotType, req.Channel, req.Payload)
	if interaction == nil {
		jsonResponse(w, 500, map[string]string{"error": "failed to record interaction"})
		return
	}

	jsonResponse(w, 200, interaction)
}
