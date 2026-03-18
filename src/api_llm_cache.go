// api_llm_cache.go — LLM 响应缓存管理 API
// lobster-guard v20.3
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleCacheStats GET /api/v1/cache/stats
func (api *ManagementAPI) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 200, map[string]interface{}{
			"enabled":        false,
			"total_entries":  0,
			"total_hits":     0,
			"total_misses":   0,
			"hit_rate":       0,
			"tokens_saved":   0,
			"cost_saved_usd": 0,
		})
		return
	}
	stats := api.llmCache.Stats()
	jsonResponse(w, 200, stats)
}

// handleCacheEntries GET /api/v1/cache/entries
func (api *ManagementAPI) handleCacheEntries(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 200, map[string]interface{}{"entries": []interface{}{}, "total": 0})
		return
	}
	tenant := r.URL.Query().Get("tenant")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	entries := api.llmCache.ListEntries(tenant, limit)
	if entries == nil {
		entries = []*CacheEntry{}
	}
	jsonResponse(w, 200, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleCacheEntriesDelete DELETE /api/v1/cache/entries
func (api *ManagementAPI) handleCacheEntriesDelete(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "cache not initialized"})
		return
	}
	count := api.llmCache.InvalidateAll()
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "cleared",
		"cleared": count,
	})
}

// handleCacheTenantDelete DELETE /api/v1/cache/tenant/:id
func (api *ManagementAPI) handleCacheTenantDelete(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "cache not initialized"})
		return
	}
	// 提取 tenant_id from path: /api/v1/cache/tenant/xxxx
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 6 {
		jsonResponse(w, 400, map[string]string{"error": "missing tenant_id"})
		return
	}
	tenantID := parts[5]
	count := api.llmCache.Invalidate(tenantID)
	jsonResponse(w, 200, map[string]interface{}{
		"status":    "cleared",
		"tenant_id": tenantID,
		"cleared":   count,
	})
}

// handleCacheConfigGet GET /api/v1/cache/config
func (api *ManagementAPI) handleCacheConfigGet(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 200, LLMCacheConfig{})
		return
	}
	cfg := api.llmCache.GetConfig()
	jsonResponse(w, 200, cfg)
}

// handleCacheConfigUpdate PUT /api/v1/cache/config
func (api *ManagementAPI) handleCacheConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "cache not initialized"})
		return
	}
	var req LLMCacheConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.llmCache.UpdateConfig(req)
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleCacheLookup POST /api/v1/cache/lookup
func (api *ManagementAPI) handleCacheLookup(w http.ResponseWriter, r *http.Request) {
	if api.llmCache == nil {
		jsonResponse(w, 400, map[string]string{"error": "cache not initialized"})
		return
	}
	var req struct {
		Query    string `json:"query"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Query == "" {
		jsonResponse(w, 400, map[string]string{"error": "query is required"})
		return
	}

	entry, similarity, hit := api.llmCache.TestLookup(req.Query, req.TenantID)
	result := map[string]interface{}{
		"query":      req.Query,
		"tenant_id":  req.TenantID,
		"hit":        hit,
		"similarity": similarity,
	}
	if entry != nil {
		result["entry"] = entry
	}
	jsonResponse(w, 200, result)
}
