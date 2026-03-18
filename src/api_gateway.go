// api_gateway.go — API Gateway 引擎：JWT 认证 + 请求转换 + 灰度发布 + 路由匹配
// lobster-guard v20.4
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 类型定义
// ============================================================

// APIGateway API 网关引擎
type APIGateway struct {
	db        *sql.DB
	mu        sync.RWMutex
	config    APIGatewayConfig
	routes    []GatewayRoute
	jwtSecret []byte
	// 统计
	totalRequests   int64
	authFailures    int64
	canaryRequests  int64
	routeHits       sync.Map // map[string]*int64
	methodHits      sync.Map // map[string]*int64
	upstreamLatency sync.Map // map[string]*gwLatencyAccum
}

// gwLatencyAccum 延迟累加器
type gwLatencyAccum struct {
	sum   float64
	count int64
	mu    sync.Mutex
}

// APIGatewayConfig 网关配置
type APIGatewayConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	JWTSecret     string   `yaml:"jwt_secret" json:"jwt_secret"`
	JWTEnabled    bool     `yaml:"jwt_enabled" json:"jwt_enabled"`
	APIKeyEnabled bool     `yaml:"apikey_enabled" json:"apikey_enabled"`
	APIKeys       []string `yaml:"api_keys" json:"api_keys"`
}

// GatewayRoute 网关路由规则
type GatewayRoute struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	PathPattern    string            `json:"path_pattern"`
	UpstreamURL    string            `json:"upstream_url"`
	Methods        []string          `json:"methods"`
	Auth           string            `json:"auth"`
	AddHeaders     map[string]string `json:"add_headers"`
	RemoveHeaders  []string          `json:"remove_headers"`
	CanaryPercent  int               `json:"canary_percent"`
	CanaryUpstream string            `json:"canary_upstream"`
	Enabled        bool              `json:"enabled"`
	Priority       int               `json:"priority"`
	CreatedAt      time.Time         `json:"created_at"`
}

// GatewayStats 网关统计
type GatewayStats struct {
	TotalRequests   int64              `json:"total_requests"`
	AuthFailures    int64              `json:"auth_failures"`
	CanaryRequests  int64              `json:"canary_requests"`
	RouteHits       map[string]int64   `json:"route_hits"`
	MethodBreakdown map[string]int64   `json:"method_breakdown"`
	UpstreamLatency map[string]float64 `json:"upstream_latency_ms"`
}

// GWJWTClaims Gateway JWT 声明（与 auth.go JWTClaims 区别开）
type GWJWTClaims struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
	Sub      string `json:"sub"`
}

// GatewayLogEntry 网关日志
type GatewayLogEntry struct {
	ID         int64   `json:"id"`
	Timestamp  string  `json:"timestamp"`
	Path       string  `json:"path"`
	Method     string  `json:"method"`
	RouteID    string  `json:"route_id"`
	TenantID   string  `json:"tenant_id"`
	Upstream   string  `json:"upstream"`
	StatusCode int     `json:"status_code"`
	LatencyMs  float64 `json:"latency_ms"`
	AuthResult string  `json:"auth_result"`
}

// ============================================================
// 初始化
// ============================================================

// NewAPIGateway 创建 API Gateway 引擎
func NewAPIGateway(db *sql.DB, cfg APIGatewayConfig) *APIGateway {
	gw := &APIGateway{
		db:        db,
		config:    cfg,
		jwtSecret: []byte(cfg.JWTSecret),
	}
	gw.initGatewayDB()
	gw.loadGatewayRoutes()
	return gw
}

// initGatewayDB 初始化 SQLite 表
func (gw *APIGateway) initGatewayDB() {
	gw.db.Exec(`CREATE TABLE IF NOT EXISTS gateway_routes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		path_pattern TEXT NOT NULL,
		upstream_url TEXT NOT NULL,
		methods_json TEXT DEFAULT '["GET","POST"]',
		auth TEXT DEFAULT 'none',
		add_headers_json TEXT DEFAULT '{}',
		remove_headers_json TEXT DEFAULT '[]',
		canary_percent INTEGER DEFAULT 0,
		canary_upstream TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		priority INTEGER DEFAULT 10,
		created_at TEXT NOT NULL
	)`)

	gw.db.Exec(`CREATE TABLE IF NOT EXISTS gateway_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		path TEXT NOT NULL,
		method TEXT NOT NULL,
		route_id TEXT DEFAULT '',
		tenant_id TEXT DEFAULT '',
		upstream TEXT DEFAULT '',
		status_code INTEGER DEFAULT 0,
		latency_ms REAL DEFAULT 0,
		auth_result TEXT DEFAULT ''
	)`)

	gw.db.Exec(`CREATE INDEX IF NOT EXISTS idx_gw_log_ts ON gateway_log(timestamp)`)
}

// loadGatewayRoutes 从数据库加载路由
func (gw *APIGateway) loadGatewayRoutes() {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	rows, err := gw.db.Query(`SELECT id, name, path_pattern, upstream_url, methods_json, auth,
		add_headers_json, remove_headers_json, canary_percent, canary_upstream, enabled, priority, created_at
		FROM gateway_routes ORDER BY priority DESC`)
	if err != nil {
		return
	}
	defer rows.Close()

	var routes []GatewayRoute
	for rows.Next() {
		var r GatewayRoute
		var methodsJSON, addHeadersJSON, removeHeadersJSON, createdAt string
		var enabled int
		err := rows.Scan(&r.ID, &r.Name, &r.PathPattern, &r.UpstreamURL,
			&methodsJSON, &r.Auth, &addHeadersJSON, &removeHeadersJSON,
			&r.CanaryPercent, &r.CanaryUpstream, &enabled, &r.Priority, &createdAt)
		if err != nil {
			continue
		}
		r.Enabled = enabled == 1
		json.Unmarshal([]byte(methodsJSON), &r.Methods)
		r.AddHeaders = make(map[string]string)
		json.Unmarshal([]byte(addHeadersJSON), &r.AddHeaders)
		json.Unmarshal([]byte(removeHeadersJSON), &r.RemoveHeaders)
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		routes = append(routes, r)
	}
	gw.routes = routes
}

// ============================================================
// JWT 实现（纯 Go, HMAC-SHA256）
// 使用 auth.go 中已有的 base64URLEncode / base64URLDecode
// ============================================================

// GenerateGWJWT 生成 Gateway JWT token
func (gw *APIGateway) GenerateGWJWT(claims GWJWTClaims) (string, error) {
	if len(gw.jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not configured")
	}

	// Header
	header := `{"alg":"HS256","typ":"JWT"}`
	headerB64 := base64URLEncode([]byte(header))

	// Payload
	if claims.Iat == 0 {
		claims.Iat = time.Now().Unix()
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	payloadB64 := base64URLEncode(payloadBytes)

	// Signature
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, gw.jwtSecret)
	mac.Write([]byte(signingInput))
	signature := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

// ValidateGWJWT 验证 Gateway JWT token
func (gw *APIGateway) ValidateGWJWT(token string) (*GWJWTClaims, error) {
	if len(gw.jwtSecret) == 0 {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	headerB64, payloadB64, signatureB64 := parts[0], parts[1], parts[2]

	// 验证签名
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, gw.jwtSecret)
	mac.Write([]byte(signingInput))
	expectedSig := base64URLEncode(mac.Sum(nil))
	if !hmac.Equal([]byte(signatureB64), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// 解码 payload
	payloadBytes, err := base64URLDecode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims GWJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	// 检查过期
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// ============================================================
// 认证
// ============================================================

// AuthenticateRequest 认证请求
// 根据网关配置判断使用 JWT 还是 API Key 认证
func (gw *APIGateway) AuthenticateRequest(r *http.Request) (*GWJWTClaims, error) {
	gw.mu.RLock()
	cfg := gw.config
	gw.mu.RUnlock()

	// JWT 认证
	if cfg.JWTEnabled {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			claims, err := gw.ValidateGWJWT(token)
			if err != nil {
				atomic.AddInt64(&gw.authFailures, 1)
				return nil, fmt.Errorf("JWT auth failed: %w", err)
			}
			return claims, nil
		}
	}

	// API Key 认证
	if cfg.APIKeyEnabled {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}
		if apiKey != "" {
			for _, k := range cfg.APIKeys {
				if k == apiKey {
					return &GWJWTClaims{
						TenantID: "apikey",
						Role:     "user",
						Sub:      "apikey-user",
					}, nil
				}
			}
			atomic.AddInt64(&gw.authFailures, 1)
			return nil, fmt.Errorf("invalid API key")
		}
	}

	// 如果两种认证都启用但都没提供凭证
	if cfg.JWTEnabled || cfg.APIKeyEnabled {
		atomic.AddInt64(&gw.authFailures, 1)
		return nil, fmt.Errorf("authentication required")
	}

	// 无认证要求
	return &GWJWTClaims{
		TenantID: "anonymous",
		Role:     "user",
		Sub:      "anonymous",
	}, nil
}

// AuthenticateForRoute 根据路由级别认证设置进行认证
func (gw *APIGateway) AuthenticateForRoute(r *http.Request, route *GatewayRoute) (*GWJWTClaims, error) {
	switch route.Auth {
	case "none", "":
		return &GWJWTClaims{
			TenantID: "anonymous",
			Role:     "user",
			Sub:      "anonymous",
		}, nil
	case "jwt":
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			atomic.AddInt64(&gw.authFailures, 1)
			return nil, fmt.Errorf("JWT token required")
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := gw.ValidateGWJWT(token)
		if err != nil {
			atomic.AddInt64(&gw.authFailures, 1)
			return nil, err
		}
		return claims, nil
	case "apikey":
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}
		if apiKey == "" {
			atomic.AddInt64(&gw.authFailures, 1)
			return nil, fmt.Errorf("API key required")
		}
		gw.mu.RLock()
		keys := gw.config.APIKeys
		gw.mu.RUnlock()
		for _, k := range keys {
			if k == apiKey {
				return &GWJWTClaims{
					TenantID: "apikey",
					Role:     "user",
					Sub:      "apikey-user",
				}, nil
			}
		}
		atomic.AddInt64(&gw.authFailures, 1)
		return nil, fmt.Errorf("invalid API key")
	default:
		return nil, fmt.Errorf("unknown auth type: %s", route.Auth)
	}
}

// ============================================================
// 路由匹配
// ============================================================

// MatchRoute 按优先级匹配路由规则（前缀匹配）
func (gw *APIGateway) MatchRoute(path string, method string) *GatewayRoute {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	// 路由已按优先级降序排列
	for i := range gw.routes {
		r := &gw.routes[i]
		if !r.Enabled {
			continue
		}
		// 前缀匹配
		if !strings.HasPrefix(path, r.PathPattern) {
			continue
		}
		// 方法匹配
		if len(r.Methods) > 0 {
			methodAllowed := false
			for _, m := range r.Methods {
				if strings.EqualFold(m, method) {
					methodAllowed = true
					break
				}
			}
			if !methodAllowed {
				continue
			}
		}
		return r
	}
	return nil
}

// ============================================================
// 请求转换
// ============================================================

// TransformRequest 转换请求
func (gw *APIGateway) TransformRequest(r *http.Request, route *GatewayRoute, claims *GWJWTClaims) {
	// 注入 AddHeaders
	for k, v := range route.AddHeaders {
		r.Header.Set(k, v)
	}

	// 删除 RemoveHeaders
	for _, h := range route.RemoveHeaders {
		r.Header.Del(h)
	}

	// 注入 X-Tenant-ID
	if claims != nil && claims.TenantID != "" {
		r.Header.Set("X-Tenant-ID", claims.TenantID)
	}

	// 注入 X-Request-ID
	r.Header.Set("X-Request-ID", uuid.New().String())
}

// ============================================================
// 灰度发布
// ============================================================

// SelectUpstream 选择上游（支持灰度路由）
func (gw *APIGateway) SelectUpstream(route *GatewayRoute) string {
	if route.CanaryPercent > 0 && route.CanaryUpstream != "" {
		if rand.Intn(100) < route.CanaryPercent {
			atomic.AddInt64(&gw.canaryRequests, 1)
			return route.CanaryUpstream
		}
	}
	return route.UpstreamURL
}

// ============================================================
// 路由 CRUD
// ============================================================

// AddRoute 添加路由
func (gw *APIGateway) AddRoute(route GatewayRoute) error {
	if route.ID == "" {
		route.ID = uuid.New().String()
	}
	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}
	if route.Methods == nil {
		route.Methods = []string{"GET", "POST"}
	}
	if route.AddHeaders == nil {
		route.AddHeaders = make(map[string]string)
	}
	if route.RemoveHeaders == nil {
		route.RemoveHeaders = []string{}
	}
	if route.Auth == "" {
		route.Auth = "none"
	}

	methodsJSON, _ := json.Marshal(route.Methods)
	addHeadersJSON, _ := json.Marshal(route.AddHeaders)
	removeHeadersJSON, _ := json.Marshal(route.RemoveHeaders)

	enabled := 0
	if route.Enabled {
		enabled = 1
	}

	_, err := gw.db.Exec(`INSERT INTO gateway_routes (id, name, path_pattern, upstream_url, methods_json, auth,
		add_headers_json, remove_headers_json, canary_percent, canary_upstream, enabled, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		route.ID, route.Name, route.PathPattern, route.UpstreamURL,
		string(methodsJSON), route.Auth,
		string(addHeadersJSON), string(removeHeadersJSON),
		route.CanaryPercent, route.CanaryUpstream, enabled, route.Priority,
		route.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert route: %w", err)
	}

	gw.loadGatewayRoutes()
	return nil
}

// UpdateRoute 更新路由
func (gw *APIGateway) UpdateRoute(id string, route GatewayRoute) error {
	if route.Methods == nil {
		route.Methods = []string{"GET", "POST"}
	}
	if route.AddHeaders == nil {
		route.AddHeaders = make(map[string]string)
	}
	if route.RemoveHeaders == nil {
		route.RemoveHeaders = []string{}
	}

	methodsJSON, _ := json.Marshal(route.Methods)
	addHeadersJSON, _ := json.Marshal(route.AddHeaders)
	removeHeadersJSON, _ := json.Marshal(route.RemoveHeaders)

	enabled := 0
	if route.Enabled {
		enabled = 1
	}

	result, err := gw.db.Exec(`UPDATE gateway_routes SET name=?, path_pattern=?, upstream_url=?, methods_json=?, auth=?,
		add_headers_json=?, remove_headers_json=?, canary_percent=?, canary_upstream=?, enabled=?, priority=?
		WHERE id=?`,
		route.Name, route.PathPattern, route.UpstreamURL,
		string(methodsJSON), route.Auth,
		string(addHeadersJSON), string(removeHeadersJSON),
		route.CanaryPercent, route.CanaryUpstream, enabled, route.Priority, id)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("route not found: %s", id)
	}

	gw.loadGatewayRoutes()
	return nil
}

// RemoveRoute 删除路由
func (gw *APIGateway) RemoveRoute(id string) error {
	result, err := gw.db.Exec(`DELETE FROM gateway_routes WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("route not found: %s", id)
	}

	gw.loadGatewayRoutes()
	return nil
}

// ListRoutes 列出所有路由
func (gw *APIGateway) ListRoutes() []GatewayRoute {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	result := make([]GatewayRoute, len(gw.routes))
	copy(result, gw.routes)
	return result
}

// ============================================================
// 统计
// ============================================================

// RecordRequest 记录请求统计
func (gw *APIGateway) RecordRequest(routeID string, method string, upstream string, latencyMs float64) {
	atomic.AddInt64(&gw.totalRequests, 1)

	// 路由命中
	if routeID != "" {
		val, _ := gw.routeHits.LoadOrStore(routeID, new(int64))
		atomic.AddInt64(val.(*int64), 1)
	}

	// 方法统计
	if method != "" {
		val, _ := gw.methodHits.LoadOrStore(method, new(int64))
		atomic.AddInt64(val.(*int64), 1)
	}

	// 延迟统计
	if upstream != "" {
		val, _ := gw.upstreamLatency.LoadOrStore(upstream, &gwLatencyAccum{})
		accum := val.(*gwLatencyAccum)
		accum.mu.Lock()
		accum.sum += latencyMs
		accum.count++
		accum.mu.Unlock()
	}
}

// GetStats 获取网关统计
func (gw *APIGateway) GetStats() GatewayStats {
	stats := GatewayStats{
		TotalRequests:   atomic.LoadInt64(&gw.totalRequests),
		AuthFailures:    atomic.LoadInt64(&gw.authFailures),
		CanaryRequests:  atomic.LoadInt64(&gw.canaryRequests),
		RouteHits:       make(map[string]int64),
		MethodBreakdown: make(map[string]int64),
		UpstreamLatency: make(map[string]float64),
	}

	gw.routeHits.Range(func(key, value interface{}) bool {
		stats.RouteHits[key.(string)] = atomic.LoadInt64(value.(*int64))
		return true
	})

	gw.methodHits.Range(func(key, value interface{}) bool {
		stats.MethodBreakdown[key.(string)] = atomic.LoadInt64(value.(*int64))
		return true
	})

	gw.upstreamLatency.Range(func(key, value interface{}) bool {
		accum := value.(*gwLatencyAccum)
		accum.mu.Lock()
		if accum.count > 0 {
			stats.UpstreamLatency[key.(string)] = accum.sum / float64(accum.count)
		}
		accum.mu.Unlock()
		return true
	})

	return stats
}

// ============================================================
// 日志
// ============================================================

// LogRequest 记录请求到 gateway_log 表
func (gw *APIGateway) LogRequest(entry GatewayLogEntry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	gw.db.Exec(`INSERT INTO gateway_log (timestamp, path, method, route_id, tenant_id, upstream, status_code, latency_ms, auth_result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Path, entry.Method, entry.RouteID,
		entry.TenantID, entry.Upstream, entry.StatusCode, entry.LatencyMs, entry.AuthResult)
}

// QueryLog 查询网关日志
func (gw *APIGateway) QueryLog(limit int, routeID string, tenantID string) []GatewayLogEntry {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	query := `SELECT id, timestamp, path, method, route_id, tenant_id, upstream, status_code, latency_ms, auth_result FROM gateway_log WHERE 1=1`
	var args []interface{}

	if routeID != "" {
		query += ` AND route_id = ?`
		args = append(args, routeID)
	}
	if tenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}

	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := gw.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []GatewayLogEntry
	for rows.Next() {
		var e GatewayLogEntry
		err := rows.Scan(&e.ID, &e.Timestamp, &e.Path, &e.Method, &e.RouteID,
			&e.TenantID, &e.Upstream, &e.StatusCode, &e.LatencyMs, &e.AuthResult)
		if err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

// ============================================================
// 配置管理
// ============================================================

// GetConfig 获取当前网关配置
func (gw *APIGateway) GetConfig() APIGatewayConfig {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.config
}

// UpdateConfig 更新网关配置
func (gw *APIGateway) UpdateConfig(cfg APIGatewayConfig) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.config = cfg
	gw.jwtSecret = []byte(cfg.JWTSecret)
}

// ============================================================
// 辅助
// ============================================================

// sortRoutesByPriority 按优先级降序排序路由
func sortRoutesByPriority(routes []GatewayRoute) {
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Priority > routes[j].Priority
	})
}
