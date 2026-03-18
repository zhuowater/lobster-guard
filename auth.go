// auth.go — 用户认证管理器（JWT + bcrypt + 用户CRUD + 操作审计）
// lobster-guard v14.1 — 登录认证系统
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// 用户模型
// ============================================================

// User 系统用户
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // 不暴露到 JSON
	DisplayName  string `json:"display_name"`
	Role         string `json:"role"`      // admin / operator / viewer
	TenantID     string `json:"tenant_id"` // 空=全局管理员
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	LastLogin    string `json:"last_login"`
}

// OpAuditEntry 操作审计日志
type OpAuditEntry struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
	IP        string `json:"ip"`
}

// ============================================================
// JWT 实现（HS256，零第三方依赖）
// ============================================================

// jwtHeader 固定 HS256 header（base64url 编码后的值固定不变）
var jwtHeaderB64 = base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

// JWTClaims JWT 载荷
type JWTClaims struct {
	Sub    string `json:"sub"`    // 用户名
	Role   string `json:"role"`   // 角色
	Tenant string `json:"tenant"` // 绑定租户
	Exp    int64  `json:"exp"`    // 过期时间（Unix 秒）
	Iat    int64  `json:"iat"`    // 签发时间
}

// base64URLEncode base64url 无 padding 编码
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// base64URLDecode base64url 解码（自动补 padding）
func base64URLDecode(s string) ([]byte, error) {
	// 补回 padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// signJWT 签发 JWT token
func signJWT(claims *JWTClaims, key []byte) string {
	payloadJSON, _ := json.Marshal(claims)
	payloadB64 := base64URLEncode(payloadJSON)

	signingInput := jwtHeaderB64 + "." + payloadB64
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	sig := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + sig
}

// verifyJWT 验证并解析 JWT token
func verifyJWT(tokenStr string, key []byte) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// 验证签名
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	expectedSig := base64URLEncode(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// 解析 payload
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	// 检查过期
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// ============================================================
// AuthManager 认证管理器
// ============================================================

// AuthManager 管理用户认证、JWT 签发与验证、操作审计
type AuthManager struct {
	db               *sql.DB
	enabled          bool
	jwtKey           []byte
	tokenExpireHours int
	mu               sync.RWMutex
}

// NewAuthManager 创建认证管理器
func NewAuthManager(db *sql.DB, cfg *AuthConfig) *AuthManager {
	am := &AuthManager{
		db:               db,
		enabled:          cfg.Enabled,
		tokenExpireHours: cfg.TokenExpireHours,
	}

	if am.tokenExpireHours <= 0 {
		am.tokenExpireHours = 24
	}

	// JWT 签名密钥
	if cfg.JWTSecret != "" {
		am.jwtKey = []byte(cfg.JWTSecret)
	} else {
		// 自动生成随机密钥（每次重启会变化，token 失效）
		am.jwtKey = make([]byte, 32)
		rand.Read(am.jwtKey)
		log.Println("[认证] JWT secret 未配置，已自动生成随机密钥（重启后 token 将失效）")
	}

	am.initSchema()
	am.ensureDefaultAdmin(cfg.DefaultPassword)

	return am
}

// initSchema 初始化 users + op_audit_log 表
func (am *AuthManager) initSchema() {
	am.db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT DEFAULT '',
		role TEXT DEFAULT 'viewer',
		tenant_id TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		created_at TEXT NOT NULL,
		last_login TEXT DEFAULT ''
	)`)

	am.db.Exec(`CREATE TABLE IF NOT EXISTS op_audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		username TEXT NOT NULL,
		action TEXT NOT NULL,
		detail TEXT DEFAULT '',
		ip TEXT DEFAULT ''
	)`)

	am.db.Exec(`CREATE INDEX IF NOT EXISTS idx_op_audit_timestamp ON op_audit_log(timestamp)`)
	am.db.Exec(`CREATE INDEX IF NOT EXISTS idx_op_audit_username ON op_audit_log(username)`)

	log.Println("[认证] ✅ users + op_audit_log schema 就绪")
}

// ensureDefaultAdmin 确保至少存在一个管理员
func (am *AuthManager) ensureDefaultAdmin(defaultPassword string) {
	var count int
	am.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count > 0 {
		return
	}

	if defaultPassword == "" {
		defaultPassword = "lobster-guard"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[认证] 生成默认管理员密码失败: %v", err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = am.db.Exec(`INSERT INTO users (username, password_hash, display_name, role, tenant_id, enabled, created_at) VALUES (?, ?, ?, ?, ?, 1, ?)`,
		"admin", string(hash), "管理员", "admin", "", now)
	if err != nil {
		log.Printf("[认证] 创建默认管理员失败: %v", err)
		return
	}

	log.Printf("[认证] ✅ 已创建默认管理员 admin（密码: %s）", defaultPassword)
}

// ============================================================
// 登录 / Token 验证
// ============================================================

// Login 验证用户名密码，返回 JWT token 和用户信息
func (am *AuthManager) Login(username, password, ip string) (string, *User, error) {
	user, err := am.getUserByUsername(username)
	if err != nil {
		am.LogOperation(username, "login_failed", "用户不存在", ip)
		return "", nil, fmt.Errorf("用户名或密码错误")
	}

	if !user.Enabled {
		am.LogOperation(username, "login_failed", "用户已禁用", ip)
		return "", nil, fmt.Errorf("用户已禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		am.LogOperation(username, "login_failed", "密码错误", ip)
		return "", nil, fmt.Errorf("用户名或密码错误")
	}

	// 签发 JWT
	now := time.Now()
	claims := &JWTClaims{
		Sub:    user.Username,
		Role:   user.Role,
		Tenant: user.TenantID,
		Iat:    now.Unix(),
		Exp:    now.Add(time.Duration(am.tokenExpireHours) * time.Hour).Unix(),
	}
	token := signJWT(claims, am.jwtKey)

	// 更新 last_login
	am.db.Exec(`UPDATE users SET last_login=? WHERE id=?`, now.UTC().Format(time.RFC3339), user.ID)
	user.LastLogin = now.UTC().Format(time.RFC3339)

	am.LogOperation(username, "login", "登录成功", ip)
	return token, user, nil
}

// ValidateToken 验证 JWT，返回用户信息
func (am *AuthManager) ValidateToken(tokenStr string) (*User, error) {
	claims, err := verifyJWT(tokenStr, am.jwtKey)
	if err != nil {
		return nil, err
	}

	user, err := am.getUserByUsername(claims.Sub)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.Enabled {
		return nil, fmt.Errorf("user disabled")
	}

	return user, nil
}

// ============================================================
// 用户 CRUD
// ============================================================

// getUserByUsername 按用户名查询
func (am *AuthManager) getUserByUsername(username string) (*User, error) {
	u := &User{}
	var enabled int
	err := am.db.QueryRow(
		`SELECT id, username, password_hash, display_name, role, tenant_id, enabled, created_at, last_login FROM users WHERE username=?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.TenantID, &enabled, &u.CreatedAt, &u.LastLogin)
	if err != nil {
		return nil, err
	}
	u.Enabled = enabled != 0
	return u, nil
}

// ListUsers 列出所有用户
func (am *AuthManager) ListUsers() ([]User, error) {
	rows, err := am.db.Query(`SELECT id, username, display_name, role, tenant_id, enabled, created_at, last_login FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u := User{}
		var enabled int
		if rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.TenantID, &enabled, &u.CreatedAt, &u.LastLogin) != nil {
			continue
		}
		u.Enabled = enabled != 0
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, nil
}

// CreateUser 创建用户
func (am *AuthManager) CreateUser(username, password, displayName, role, tenantID string) (*User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}
	if role == "" {
		role = "viewer"
	}
	if role != "admin" && role != "operator" && role != "viewer" {
		return nil, fmt.Errorf("角色必须是 admin/operator/viewer")
	}
	if len(password) < 4 {
		return nil, fmt.Errorf("密码长度至少 4 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := am.db.Exec(`INSERT INTO users (username, password_hash, display_name, role, tenant_id, enabled, created_at) VALUES (?, ?, ?, ?, ?, 1, ?)`,
		username, string(hash), displayName, role, tenantID, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("用户名 %q 已存在", username)
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	id, _ := result.LastInsertId()
	return &User{
		ID: int(id), Username: username, DisplayName: displayName,
		Role: role, TenantID: tenantID, Enabled: true, CreatedAt: now,
	}, nil
}

// UpdateUser 更新用户信息（不改密码）
func (am *AuthManager) UpdateUser(id int, displayName, role, tenantID string, enabled bool) error {
	if role != "" && role != "admin" && role != "operator" && role != "viewer" {
		return fmt.Errorf("角色必须是 admin/operator/viewer")
	}

	_, err := am.db.Exec(`UPDATE users SET display_name=?, role=?, tenant_id=?, enabled=? WHERE id=?`,
		displayName, role, tenantID, boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}
	return nil
}

// DeleteUser 删除用户（不允许删除自己或最后一个 admin）
func (am *AuthManager) DeleteUser(id int, currentUsername string) error {
	// 检查目标用户
	var username, role string
	err := am.db.QueryRow(`SELECT username, role FROM users WHERE id=?`, id).Scan(&username, &role)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if username == currentUsername {
		return fmt.Errorf("不能删除自己")
	}

	// 如果删除的是 admin，确保还有其他 admin
	if role == "admin" {
		var adminCount int
		am.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND enabled=1`).Scan(&adminCount)
		if adminCount <= 1 {
			return fmt.Errorf("不能删除最后一个管理员")
		}
	}

	_, err = am.db.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	return nil
}

// ChangePassword 修改密码
func (am *AuthManager) ChangePassword(username, oldPassword, newPassword string) error {
	user, err := am.getUserByUsername(username)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("旧密码错误")
	}

	if len(newPassword) < 4 {
		return fmt.Errorf("新密码长度至少 4 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	_, err = am.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, string(hash), user.ID)
	if err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}
	return nil
}

// ResetPassword 管理员重置密码（不需要旧密码）
func (am *AuthManager) ResetPassword(userID int, newPassword string) error {
	if len(newPassword) < 4 {
		return fmt.Errorf("新密码长度至少 4 位")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	result, err := am.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, string(hash), userID)
	if err != nil {
		return fmt.Errorf("重置密码失败: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// ============================================================
// 操作审计
// ============================================================

// LogOperation 记录操作审计日志
func (am *AuthManager) LogOperation(username, action, detail, ip string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := am.db.Exec(`INSERT INTO op_audit_log (timestamp, username, action, detail, ip) VALUES (?, ?, ?, ?, ?)`,
		now, username, action, detail, ip)
	if err != nil {
		log.Printf("[审计] 记录操作失败: %v", err)
	}
}

// QueryOpAudit 查询操作审计日志
func (am *AuthManager) QueryOpAudit(username, action string, limit int) ([]OpAuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, timestamp, username, action, detail, ip FROM op_audit_log WHERE 1=1`
	args := []interface{}{}

	if username != "" {
		query += ` AND username=?`
		args = append(args, username)
	}
	if action != "" {
		query += ` AND action=?`
		args = append(args, action)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []OpAuditEntry
	for rows.Next() {
		e := OpAuditEntry{}
		if rows.Scan(&e.ID, &e.Timestamp, &e.Username, &e.Action, &e.Detail, &e.IP) != nil {
			continue
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []OpAuditEntry{}
	}
	return entries, nil
}

// SeedDemoUsers 注入演示用户（demo seed 使用）
func (am *AuthManager) SeedDemoUsers() int {
	demoUsers := []struct {
		Username    string
		Password    string
		DisplayName string
		Role        string
		TenantID    string
	}{
		{"admin", "admin123", "管理员", "admin", ""},
		{"sec-operator", "sec123", "安全运维", "operator", "security-team"},
		{"viewer", "view123", "只读用户", "viewer", "product-team"},
	}

	created := 0
	for _, u := range demoUsers {
		// 如果用户已存在则跳过
		var exists int
		am.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=?`, u.Username).Scan(&exists)
		if exists > 0 {
			// 更新密码（demo 重置）
			hash, _ := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
			am.db.Exec(`UPDATE users SET password_hash=?, display_name=?, role=?, tenant_id=?, enabled=1 WHERE username=?`,
				string(hash), u.DisplayName, u.Role, u.TenantID, u.Username)
			created++
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = am.db.Exec(`INSERT INTO users (username, password_hash, display_name, role, tenant_id, enabled, created_at) VALUES (?, ?, ?, ?, ?, 1, ?)`,
			u.Username, string(hash), u.DisplayName, u.Role, u.TenantID, now)
		if err == nil {
			created++
		}
	}
	return created
}

// ============================================================
// 角色权限检查
// ============================================================

// IsAdmin 是否管理员
func (u *User) IsAdmin() bool {
	return u != nil && u.Role == "admin"
}

// CanManageUsers 是否可以管理用户
func (u *User) CanManageUsers() bool {
	return u != nil && u.Role == "admin"
}

// CanWrite 是否可以执行写操作
func (u *User) CanWrite() bool {
	return u != nil && (u.Role == "admin" || u.Role == "operator")
}

// CanViewTenant 是否可以查看指定租户数据
func (u *User) CanViewTenant(tenantID string) bool {
	if u == nil {
		return false
	}
	if u.Role == "admin" || u.TenantID == "" {
		return true // admin 或全局用户可以看所有
	}
	return u.TenantID == tenantID || tenantID == "all"
}

// getClientIP 提取客户端 IP
func getClientIP(r interface{ Header() interface{ Get(string) string } }) string {
	// 通过 X-Forwarded-For 或 X-Real-IP 或 RemoteAddr
	return "" // 占位，在 handler 中实现
}

// ExtractTokenFromRequest 从请求中提取 token
// 优先 Authorization: Bearer <token>，其次 Cookie: lg_token=<token>
func ExtractTokenFromRequest(authHeader, cookieHeader string) string {
	// 1. Authorization header
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. Cookie
	if cookieHeader != "" {
		for _, part := range strings.Split(cookieHeader, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "lg_token=") {
				return strings.TrimPrefix(part, "lg_token=")
			}
		}
	}

	return ""
}

// GetUserIDStr 将整数 ID 转换为字符串（用于 URL path 参数解析）
func parseUserID(idStr string) (int, error) {
	return strconv.Atoi(idStr)
}
