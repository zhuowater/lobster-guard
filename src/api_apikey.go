package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (api *ManagementAPI) handleAPIKeyList(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	tenantID := r.URL.Query().Get("tenant")
	status := r.URL.Query().Get("status")
	list, err := api.apiKeyMgr.List(tenantID, status)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"keys": list, "total": len(list)})
}

// handleAPIKeyCreate POST /api/v1/apikeys — 创建 API Key
func (api *ManagementAPI) handleAPIKeyCreate(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	var entry APIKeyEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if entry.UserID == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id required"})
		return
	}
	created, rawKey, err := api.apiKeyMgr.Create(&entry)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"key":    created,
		"raw_key": rawKey,
		"warning": "请妥善保管此 Key，它将不再显示完整内容",
	})
}

// handleAPIKeyGet GET /api/v1/apikeys/:id — API Key 详情
func (api *ManagementAPI) handleAPIKeyGet(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	entry, err := api.apiKeyMgr.Get(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, entry)
}

// handleAPIKeyUpdate PUT /api/v1/apikeys/:id — 更新 API Key
func (api *ManagementAPI) handleAPIKeyUpdate(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	var entry APIKeyEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	entry.ID = id
	if err := api.apiKeyMgr.Update(&entry); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleAPIKeyDelete DELETE /api/v1/apikeys/:id — 删除 API Key
func (api *ManagementAPI) handleAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	if err := api.apiKeyMgr.Delete(id); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted"})
}

// handleAPIKeyRotate POST /api/v1/apikeys/:id/rotate — 轮换 API Key
func (api *ManagementAPI) handleAPIKeyRotate(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	// 从 /api/v1/apikeys/:id/rotate 中提取 id
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	id := strings.TrimSuffix(trimmed, "/rotate")
	entry, rawKey, err := api.apiKeyMgr.Rotate(id)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "rotated",
		"key":     entry,
		"raw_key": rawKey,
		"warning": "旧 Key 已失效，请使用新 Key",
	})
}

// handleAPIKeyBind POST /api/v1/apikeys/:id/bind — 绑定用户信息到 key
func (api *ManagementAPI) handleAPIKeyBind(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/apikeys/")
	id := strings.TrimSuffix(trimmed, "/bind")
	if id == "" {
		jsonResponse(w, 400, map[string]string{"error": "id required"})
		return
	}
	var req struct {
		UserID     string `json:"user_id"`
		UserName   string `json:"user_name"`
		Department string `json:"department"`
		TenantID   string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.UserID == "" {
		jsonResponse(w, 400, map[string]string{"error": "user_id required"})
		return
	}
	if err := api.apiKeyMgr.Bind(id, req.UserID, req.UserName, req.Department, req.TenantID); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "bound", "id": id, "user_id": req.UserID})
}

// handleAPIKeyPendingList GET /api/v1/apikeys/pending — 只列出待绑定的 key
func (api *ManagementAPI) handleAPIKeyPendingList(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	list, err := api.apiKeyMgr.List("", "pending")
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"keys": list, "total": len(list)})
}

// handleAPIKeyStats GET /api/v1/apikeys/stats — key 统计
func (api *ManagementAPI) handleAPIKeyStats(w http.ResponseWriter, r *http.Request) {
	if api.apiKeyMgr == nil {
		jsonResponse(w, 400, map[string]string{"error": "API Key manager not enabled"})
		return
	}
	stats, err := api.apiKeyMgr.Stats()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, stats)
}

// ============================================================
// v27.0: 租户策略模板绑定 API
// ============================================================

// handleTenantBindTemplate POST /api/v1/tenants/:id/bind-template — 绑定策略模板
