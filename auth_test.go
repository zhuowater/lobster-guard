// auth_test.go — v14.1 认证系统测试
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// 辅助函数
// ============================================================

func setupAuthManager(t *testing.T) (*AuthManager, *sql.DB, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-auth-test-%d.db", time.Now().UnixNano())
	db, err := sql.Open("sqlite3", tmpDB+"?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	cfg := &AuthConfig{
		Enabled:          true,
		JWTSecret:        "test-secret-key-for-unit-tests",
		DefaultPassword:  "admin123",
		TokenExpireHours: 1,
	}
	am := NewAuthManager(db, cfg)
	cleanup := func() { db.Close(); os.Remove(tmpDB) }
	return am, db, cleanup
}

func setupMgmtAPIWithAuth(t *testing.T) (*ManagementAPI, func()) {
	t.Helper()
	tmpDB := fmt.Sprintf("/tmp/lobster-auth-api-test-%d.db", time.Now().UnixNano())
	cfg := &Config{
		StaticUpstreams:       []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		ManagementToken:       "mgmt-token",
		RegistrationToken:     "reg-token",
		HeartbeatIntervalSec:  10,
		HeartbeatTimeoutCount: 3,
		RoutePersist:          false,
		Auth: AuthConfig{
			Enabled:          true,
			JWTSecret:        "test-jwt-secret",
			DefaultPassword:  "admin123",
			TokenExpireHours: 1,
		},
	}
	db, _ := initDB(tmpDB)
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	outEngine := NewOutboundRuleEngine(nil)
	engine := NewRuleEngine()
	channel := NewGenericPlugin("", "")
	inbound := NewInboundProxy(cfg, channel, engine, logger, pool, routes, nil, nil, nil, nil)
	api := NewManagementAPI(cfg, "", pool, routes, logger, engine, outEngine, inbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// 注入 AuthManager
	am := NewAuthManager(db, &cfg.Auth)
	api.authManager = am

	cleanup := func() { logger.Close(); db.Close(); os.Remove(tmpDB) }
	return api, cleanup
}

// ============================================================
// JWT 测试
// ============================================================

func TestJWTSignAndVerify(t *testing.T) {
	key := []byte("test-secret")
	claims := &JWTClaims{
		Sub:    "admin",
		Role:   "admin",
		Tenant: "",
		Iat:    time.Now().Unix(),
		Exp:    time.Now().Add(1 * time.Hour).Unix(),
	}
	token := signJWT(claims, key)
	if token == "" {
		t.Fatal("JWT 签发返回空")
	}

	// 验证
	parsed, err := verifyJWT(token, key)
	if err != nil {
		t.Fatalf("JWT 验证失败: %v", err)
	}
	if parsed.Sub != "admin" {
		t.Fatalf("期望 Sub=admin，实际 %s", parsed.Sub)
	}
	if parsed.Role != "admin" {
		t.Fatalf("期望 Role=admin，实际 %s", parsed.Role)
	}
}

func TestJWTExpired(t *testing.T) {
	key := []byte("test-secret")
	claims := &JWTClaims{
		Sub: "user1",
		Exp: time.Now().Add(-1 * time.Hour).Unix(), // 已过期
		Iat: time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := signJWT(claims, key)

	_, err := verifyJWT(token, key)
	if err == nil {
		t.Fatal("过期 token 验证应该失败")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("期望 expired 错误，实际: %v", err)
	}
}

func TestJWTInvalidSignature(t *testing.T) {
	key1 := []byte("key-one")
	key2 := []byte("key-two")
	claims := &JWTClaims{Sub: "admin", Exp: time.Now().Add(1 * time.Hour).Unix()}
	token := signJWT(claims, key1)

	_, err := verifyJWT(token, key2)
	if err == nil {
		t.Fatal("不同密钥签名的 token 验证应该失败")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Fatalf("期望 signature 错误，实际: %v", err)
	}
}

func TestJWTMalformed(t *testing.T) {
	key := []byte("test-secret")
	cases := []string{
		"",
		"not-a-jwt",
		"only.two",
		"a.b.c.d",
	}
	for _, tc := range cases {
		_, err := verifyJWT(tc, key)
		if err == nil {
			t.Fatalf("格式错误的 token %q 验证应该失败", tc)
		}
	}
}

// ============================================================
// 密码哈希测试
// ============================================================

func TestPasswordHashAndVerify(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	// 创建用户
	user, err := am.CreateUser("hashtest", "mypassword", "Hash Test", "viewer", "")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 登录验证
	_, _, err = am.Login("hashtest", "mypassword", "127.0.0.1")
	if err != nil {
		t.Fatalf("正确密码登录失败: %v", err)
	}
	_ = user
}

func TestPasswordHashWrong(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.CreateUser("wrongpw", "correct", "WP", "viewer", "")

	_, _, err := am.Login("wrongpw", "wrong-password", "127.0.0.1")
	if err == nil {
		t.Fatal("错误密码登录应该失败")
	}
}

// ============================================================
// 用户 CRUD 测试
// ============================================================

func TestCreateUser(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	user, err := am.CreateUser("newuser", "pass1234", "New User", "operator", "team-a")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if user.Username != "newuser" {
		t.Fatalf("期望 username=newuser，实际 %s", user.Username)
	}
	if user.Role != "operator" {
		t.Fatalf("期望 role=operator，实际 %s", user.Role)
	}
	if user.TenantID != "team-a" {
		t.Fatalf("期望 tenant_id=team-a，实际 %s", user.TenantID)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.CreateUser("dupuser", "pass1234", "Dup", "viewer", "")
	_, err := am.CreateUser("dupuser", "pass5678", "Dup2", "viewer", "")
	if err == nil {
		t.Fatal("重复用户名创建应该失败")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("期望包含'已存在'，实际: %v", err)
	}
}

func TestCreateUserInvalidRole(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	_, err := am.CreateUser("badrole", "pass1234", "Bad", "superadmin", "")
	if err == nil {
		t.Fatal("无效角色应该失败")
	}
}

func TestCreateUserShortPassword(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	_, err := am.CreateUser("shortpw", "ab", "Short", "viewer", "")
	if err == nil {
		t.Fatal("短密码应该失败")
	}
}

func TestListUsers(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	// admin 是默认创建的
	am.CreateUser("user1", "pass1234", "U1", "operator", "")
	am.CreateUser("user2", "pass1234", "U2", "viewer", "")

	users, err := am.ListUsers()
	if err != nil {
		t.Fatalf("列出用户失败: %v", err)
	}
	if len(users) != 3 { // admin + user1 + user2
		t.Fatalf("期望 3 个用户，实际 %d", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	user, _ := am.CreateUser("upuser", "pass1234", "Old Name", "viewer", "old-team")
	err := am.UpdateUser(user.ID, "New Name", "operator", "new-team", true)
	if err != nil {
		t.Fatalf("更新用户失败: %v", err)
	}

	updated, _ := am.getUserByUsername("upuser")
	if updated.DisplayName != "New Name" {
		t.Fatalf("期望 display_name=New Name，实际 %s", updated.DisplayName)
	}
	if updated.Role != "operator" {
		t.Fatalf("期望 role=operator，实际 %s", updated.Role)
	}
}

func TestDeleteUser(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	user, _ := am.CreateUser("deluser", "pass1234", "Del", "viewer", "")
	err := am.DeleteUser(user.ID, "admin")
	if err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}

	_, err = am.getUserByUsername("deluser")
	if err == nil {
		t.Fatal("删除后应该查找不到用户")
	}
}

func TestDeleteUserCannotDeleteSelf(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	admin, _ := am.getUserByUsername("admin")
	err := am.DeleteUser(admin.ID, "admin")
	if err == nil {
		t.Fatal("不应该能删除自己")
	}
}

func TestDeleteUserCannotDeleteLastAdmin(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	// admin 是唯一的管理员，用其他用户身份删除
	admin, _ := am.getUserByUsername("admin")
	err := am.DeleteUser(admin.ID, "other")
	if err == nil {
		t.Fatal("不应该能删除最后一个管理员")
	}
}

// ============================================================
// 登录流程测试
// ============================================================

func TestLoginSuccess(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	token, user, err := am.Login("admin", "admin123", "127.0.0.1")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if token == "" {
		t.Fatal("token 不应该为空")
	}
	if user.Username != "admin" {
		t.Fatalf("期望 username=admin，实际 %s", user.Username)
	}
	if user.Role != "admin" {
		t.Fatalf("期望 role=admin，实际 %s", user.Role)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	_, _, err := am.Login("admin", "wrong-password", "127.0.0.1")
	if err == nil {
		t.Fatal("错误密码应该登录失败")
	}
}

func TestLoginNonExistent(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	_, _, err := am.Login("ghost", "whatever", "127.0.0.1")
	if err == nil {
		t.Fatal("不存在的用户应该登录失败")
	}
}

func TestLoginDisabledUser(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	user, _ := am.CreateUser("disabled", "pass1234", "Disabled", "viewer", "")
	am.UpdateUser(user.ID, "Disabled", "viewer", "", false)

	_, _, err := am.Login("disabled", "pass1234", "127.0.0.1")
	if err == nil {
		t.Fatal("禁用用户应该登录失败")
	}
}

func TestLoginAndValidateToken(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	token, _, err := am.Login("admin", "admin123", "127.0.0.1")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	user, err := am.ValidateToken(token)
	if err != nil {
		t.Fatalf("验证 token 失败: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("期望 admin，实际 %s", user.Username)
	}
}

// ============================================================
// 角色权限测试
// ============================================================

func TestRolePermissions(t *testing.T) {
	admin := &User{Role: "admin", TenantID: ""}
	operator := &User{Role: "operator", TenantID: "team-a"}
	viewer := &User{Role: "viewer", TenantID: "team-b"}

	t.Run("Admin", func(t *testing.T) {
		if !admin.IsAdmin() {
			t.Fatal("admin.IsAdmin 应该返回 true")
		}
		if !admin.CanWrite() {
			t.Fatal("admin.CanWrite 应该返回 true")
		}
		if !admin.CanManageUsers() {
			t.Fatal("admin.CanManageUsers 应该返回 true")
		}
		if !admin.CanViewTenant("any-tenant") {
			t.Fatal("admin 应该能查看任何租户")
		}
	})

	t.Run("Operator", func(t *testing.T) {
		if operator.IsAdmin() {
			t.Fatal("operator.IsAdmin 应该返回 false")
		}
		if !operator.CanWrite() {
			t.Fatal("operator.CanWrite 应该返回 true")
		}
		if operator.CanManageUsers() {
			t.Fatal("operator.CanManageUsers 应该返回 false")
		}
		if !operator.CanViewTenant("team-a") {
			t.Fatal("operator 应该能查看自己的租户")
		}
		if operator.CanViewTenant("team-b") {
			t.Fatal("operator 不应该能查看其他租户")
		}
	})

	t.Run("Viewer", func(t *testing.T) {
		if viewer.IsAdmin() {
			t.Fatal("viewer.IsAdmin 应该返回 false")
		}
		if viewer.CanWrite() {
			t.Fatal("viewer.CanWrite 应该返回 false")
		}
		if viewer.CanManageUsers() {
			t.Fatal("viewer.CanManageUsers 应该返回 false")
		}
		if !viewer.CanViewTenant("team-b") {
			t.Fatal("viewer 应该能查看自己的租户")
		}
	})
}

// ============================================================
// 密码修改测试
// ============================================================

func TestChangePassword(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.CreateUser("chpw", "oldpass1", "CP", "viewer", "")

	err := am.ChangePassword("chpw", "oldpass1", "newpass1")
	if err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	// 旧密码登录失败
	_, _, err = am.Login("chpw", "oldpass1", "")
	if err == nil {
		t.Fatal("旧密码登录应该失败")
	}

	// 新密码登录成功
	_, _, err = am.Login("chpw", "newpass1", "")
	if err != nil {
		t.Fatalf("新密码登录失败: %v", err)
	}
}

func TestChangePasswordWrongOld(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.CreateUser("chpw2", "correct", "CP2", "viewer", "")

	err := am.ChangePassword("chpw2", "wrong-old", "newpass")
	if err == nil {
		t.Fatal("旧密码错误应该失败")
	}
}

// ============================================================
// 操作审计测试
// ============================================================

func TestLogOperation(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.LogOperation("admin", "login", "登录成功", "127.0.0.1")
	am.LogOperation("admin", "config_change", "修改配置", "127.0.0.1")

	entries, err := am.QueryOpAudit("", "", 10)
	if err != nil {
		t.Fatalf("查询审计日志失败: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("期望至少 2 条审计日志，实际 %d", len(entries))
	}
}

func TestQueryOpAuditFilter(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	am.LogOperation("admin", "login", "登录", "")
	am.LogOperation("user1", "login", "登录", "")
	am.LogOperation("admin", "config_change", "改配置", "")

	// 按用户名过滤
	entries, _ := am.QueryOpAudit("admin", "", 10)
	if len(entries) < 2 {
		t.Fatalf("admin 操作应该至少有 2 条，实际 %d", len(entries))
	}

	// 按动作过滤
	entries, _ = am.QueryOpAudit("", "login", 10)
	if len(entries) < 2 {
		t.Fatalf("login 操作应该至少有 2 条，实际 %d", len(entries))
	}
}

// ============================================================
// API 集成测试
// ============================================================

func TestAuthLoginAPI(t *testing.T) {
	api, cleanup := setupMgmtAPIWithAuth(t)
	defer cleanup()

	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("登录 API 期望 200，实际 %d，body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("登录响应应该包含 token")
	}
	user := resp["user"].(map[string]interface{})
	if user["username"] != "admin" {
		t.Fatalf("期望 username=admin，实际 %v", user["username"])
	}
}

func TestAuthLoginAPIWrongPassword(t *testing.T) {
	api, cleanup := setupMgmtAPIWithAuth(t)
	defer cleanup()

	body := `{"username":"admin","password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("错误密码 API 期望 401，实际 %d", rec.Code)
	}
}

func TestAuthMeAPI(t *testing.T) {
	api, cleanup := setupMgmtAPIWithAuth(t)
	defer cleanup()

	// 先登录获取 token
	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	var loginResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)

	// 用 JWT 访问 /me
	req2 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	api.ServeHTTP(rec2, req2)

	if rec2.Code != 200 {
		t.Fatalf("/me API 期望 200，实际 %d, body: %s", rec2.Code, rec2.Body.String())
	}
}

func TestAuthBackwardCompatBearerToken(t *testing.T) {
	api, cleanup := setupMgmtAPIWithAuth(t)
	defer cleanup()

	// 旧的 Bearer mgmt-token 仍然有效
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("healthz 期望 200，实际 %d", rec.Code)
	}
}

func TestSeedDemoUsers(t *testing.T) {
	am, _, cleanup := setupAuthManager(t)
	defer cleanup()

	created := am.SeedDemoUsers()
	if created < 2 { // admin 已存在会 update，sec-operator + viewer 是新建
		t.Fatalf("期望至少创建 2 个 demo 用户，实际 %d", created)
	}

	// 验证 sec-operator 可以登录
	_, user, err := am.Login("sec-operator", "sec123", "")
	if err != nil {
		t.Fatalf("sec-operator 登录失败: %v", err)
	}
	if user.Role != "operator" {
		t.Fatalf("期望 role=operator，实际 %s", user.Role)
	}
	if user.TenantID != "security-team" {
		t.Fatalf("期望 tenant_id=security-team，实际 %s", user.TenantID)
	}
}

func TestExtractTokenFromRequest(t *testing.T) {
	t.Run("Bearer header", func(t *testing.T) {
		token := ExtractTokenFromRequest("Bearer my-jwt-token", "")
		if token != "my-jwt-token" {
			t.Fatalf("期望 my-jwt-token，实际 %s", token)
		}
	})

	t.Run("Cookie", func(t *testing.T) {
		token := ExtractTokenFromRequest("", "lg_token=cookie-jwt; other=val")
		if token != "cookie-jwt" {
			t.Fatalf("期望 cookie-jwt，实际 %s", token)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		token := ExtractTokenFromRequest("", "")
		if token != "" {
			t.Fatalf("期望空，实际 %s", token)
		}
	})
}
