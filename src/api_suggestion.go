package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSuggestionList GET /api/v1/suggestions — 规则建议列表
func (a *ManagementAPI) handleSuggestionList(w http.ResponseWriter, r *http.Request) {
	if a.suggestionQueue == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"suggestions": []interface{}{},
			"stats":       map[string]int{"pending": 0, "accepted": 0, "rejected": 0, "total": 0},
		})
		return
	}

	status := r.URL.Query().Get("status") // "pending" | "accepted" | "rejected" | ""
	suggestions, err := a.suggestionQueue.List(status, 200)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if suggestions == nil {
		suggestions = []RuleSuggestion{}
	}

	stats := a.suggestionQueue.Stats()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"stats":       stats,
	})
}

// handleSuggestionAccept POST /api/v1/suggestions/:id/accept — 接受建议
func (a *ManagementAPI) handleSuggestionAccept(w http.ResponseWriter, r *http.Request) {
	if a.suggestionQueue == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "suggestion queue not available"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/suggestions/")
	id = strings.TrimSuffix(id, "/accept")

	reviewer := "admin" // TODO: 从 JWT 获取
	if err := a.suggestionQueue.Accept(id, reviewer); err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "accepted", "id": id})
}

// handleSuggestionReject POST /api/v1/suggestions/:id/reject — 拒绝建议
func (a *ManagementAPI) handleSuggestionReject(w http.ResponseWriter, r *http.Request) {
	if a.suggestionQueue == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "suggestion queue not available"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/suggestions/")
	id = strings.TrimSuffix(id, "/reject")

	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	reviewer := "admin"
	if err := a.suggestionQueue.Reject(id, reviewer, body.Reason); err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "rejected", "id": id})
}

// handleSuggestionStats GET /api/v1/suggestions/stats — 建议统计
func (a *ManagementAPI) handleSuggestionStats(w http.ResponseWriter, r *http.Request) {
	if a.suggestionQueue == nil {
		jsonResponse(w, http.StatusOK, map[string]int{"pending": 0, "accepted": 0, "rejected": 0, "total": 0})
		return
	}
	jsonResponse(w, http.StatusOK, a.suggestionQueue.Stats())
}
