package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		jsonResponse(w, 400, map[string]string{"error": "username and password required"})
		return
	}

	ip := getRequestIP(r)
	token, user, err := api.authManager.Login(req.Username, req.Password, ip)
	if err != nil {
		jsonResponse(w, 401, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, 200, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"role":         user.Role,
			"tenant_id":    user.TenantID,
		},
	})
}

// handleAuthCheck GET /api/v1/auth/check — 检查认证状态（前端路由守卫用）
func (api *ManagementAPI) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	authEnabled := api.authManager != nil && api.authManager.enabled
	result := map[string]interface{}{
		"auth_enabled": authEnabled,
	}

	if !authEnabled {
		// auth 未启用，检查旧 token
		result["authenticated"] = api.checkManagementAuth(r)
		jsonResponse(w, 200, result)
		return
	}

	// 检查 JWT
	tokenStr := ExtractTokenFromRequest(r.Header.Get("Authorization"), r.Header.Get("Cookie"))
	if tokenStr == "" {
		// 也尝试旧 token
		if api.checkManagementAuth(r) {
			result["authenticated"] = true
			jsonResponse(w, 200, result)
			return
		}
		result["authenticated"] = false
		jsonResponse(w, 200, result)
		return
	}

	user, err := api.authManager.ValidateToken(tokenStr)
	if err != nil {
		result["authenticated"] = false
		jsonResponse(w, 200, result)
		return
	}

	result["authenticated"] = true
	result["user"] = map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"tenant_id":    user.TenantID,
	}
	jsonResponse(w, 200, result)
}

// handleAuthLogout POST /api/v1/auth/logout — 登出
func (api *ManagementAPI) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r)
	username := "unknown"
	if user != nil {
		username = user.Username
	}
	if api.authManager != nil {
		api.authManager.LogOperation(username, "logout", "用户登出", getRequestIP(r))
	}
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleAuthMe GET /api/v1/auth/me — 当前用户信息
func (api *ManagementAPI) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r)
	if user == nil {
		// auth 未启用或使用旧 token
		jsonResponse(w, 200, map[string]interface{}{
			"username":     "admin",
			"display_name": "管理员",
			"role":         "admin",
			"tenant_id":    "",
			"auth_enabled": false,
		})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"tenant_id":    user.TenantID,
		"enabled":      user.Enabled,
		"created_at":   user.CreatedAt,
		"last_login":   user.LastLogin,
		"auth_enabled": true,
	})
}

// handleAuthPassword POST /api/v1/auth/password — 修改密码
func (api *ManagementAPI) handleAuthPassword(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	user := getUserFromContext(r)
	if user == nil {
		jsonResponse(w, 401, map[string]string{"error": "login required"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	if err := api.authManager.ChangePassword(user.Username, req.OldPassword, req.NewPassword); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	api.authManager.LogOperation(user.Username, "password_change", "修改密码", getRequestIP(r))
	jsonResponse(w, 200, map[string]string{"status": "ok"})
}

// handleAuthUserList GET /api/v1/auth/users — 用户列表（admin only）
func (api *ManagementAPI) handleAuthUserList(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	user := getUserFromContext(r)
	if user != nil && !user.IsAdmin() {
		jsonResponse(w, 403, map[string]string{"error": "admin only"})
		return
	}
	users, err := api.authManager.ListUsers()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"users": users, "total": len(users)})
}

// handleAuthUserCreate POST /api/v1/auth/users — 创建用户（admin only）
func (api *ManagementAPI) handleAuthUserCreate(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	user := getUserFromContext(r)
	if user != nil && !user.IsAdmin() {
		jsonResponse(w, 403, map[string]string{"error": "admin only"})
		return
	}
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		TenantID    string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	newUser, err := api.authManager.CreateUser(req.Username, req.Password, req.DisplayName, req.Role, req.TenantID)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if api.authManager != nil && user != nil {
		api.authManager.LogOperation(user.Username, "user_create", "创建用户: "+req.Username, getRequestIP(r))
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "created", "user": newUser})
}

// handleAuthUserUpdate PUT /api/v1/auth/users/:id — 更新用户（admin only）
func (api *ManagementAPI) handleAuthUserUpdate(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	user := getUserFromContext(r)
	if user != nil && !user.IsAdmin() {
		jsonResponse(w, 403, map[string]string{"error": "admin only"})
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/auth/users/")
	id, err := parseUserID(idStr)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid user id"})
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		TenantID    string `json:"tenant_id"`
		Enabled     *bool  `json:"enabled"`
		Password    string `json:"password"` // 可选：重置密码
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request"})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := api.authManager.UpdateUser(id, req.DisplayName, req.Role, req.TenantID, enabled); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}

	// 如果提供了新密码，重置密码
	if req.Password != "" {
		if err := api.authManager.ResetPassword(id, req.Password); err != nil {
			jsonResponse(w, 400, map[string]string{"error": "更新成功但重置密码失败: " + err.Error()})
			return
		}
	}

	if api.authManager != nil && user != nil {
		api.authManager.LogOperation(user.Username, "user_update", fmt.Sprintf("更新用户 #%d", id), getRequestIP(r))
	}
	jsonResponse(w, 200, map[string]string{"status": "updated"})
}

// handleAuthUserDelete DELETE /api/v1/auth/users/:id — 删除用户（admin only）
func (api *ManagementAPI) handleAuthUserDelete(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 400, map[string]string{"error": "auth not initialized"})
		return
	}
	user := getUserFromContext(r)
	if user != nil && !user.IsAdmin() {
		jsonResponse(w, 403, map[string]string{"error": "admin only"})
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/auth/users/")
	id, err := parseUserID(idStr)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid user id"})
		return
	}

	currentUsername := ""
	if user != nil {
		currentUsername = user.Username
	}

	if err := api.authManager.DeleteUser(id, currentUsername); err != nil {
		jsonResponse(w, 400, map[string]string{"error": err.Error()})
		return
	}

	if api.authManager != nil && user != nil {
		api.authManager.LogOperation(user.Username, "user_delete", fmt.Sprintf("删除用户 #%d", id), getRequestIP(r))
	}
	jsonResponse(w, 200, map[string]string{"status": "deleted"})
}

// handleOpAudit GET /api/v1/op-audit — 操作审计日志（admin only）
func (api *ManagementAPI) handleOpAudit(w http.ResponseWriter, r *http.Request) {
	if api.authManager == nil {
		jsonResponse(w, 200, map[string]interface{}{"entries": []interface{}{}, "total": 0})
		return
	}
	user := getUserFromContext(r)
	if user != nil && !user.IsAdmin() {
		jsonResponse(w, 403, map[string]string{"error": "admin only"})
		return
	}

	username := r.URL.Query().Get("username")
	action := r.URL.Query().Get("action")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	entries, err := api.authManager.QueryOpAudit(username, action, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"entries": entries, "total": len(entries)})
}

// ============================================================
// v14.2 Red Team Autopilot API
// ============================================================

// handleRedTeamRun POST /api/v1/redteam/run — 执行红队测试
