// store.go — Store 接口、SQLiteStore 实现（v4.2 存储抽象层）
// lobster-guard v4.2 高可用
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Store 接口 — 存储抽象层（v4.2）
// 为未来 PostgresStore 等多后端做准备
// ============================================================

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID             int     `json:"id"`
	Timestamp      string  `json:"timestamp"`
	Direction      string  `json:"direction"`
	SenderID       string  `json:"sender_id"`
	Action         string  `json:"action"`
	Reason         string  `json:"reason"`
	ContentPreview string  `json:"content_preview"`
	FullRequestHash string `json:"full_request_hash"`
	LatencyMs      float64 `json:"latency_ms"`
	UpstreamID     string  `json:"upstream_id"`
	AppID          string  `json:"app_id"`
}

// AuditFilter 审计日志查询过滤条件
type AuditFilter struct {
	Direction string
	Action    string
	SenderID  string
	AppID     string
	Query     string // 全文搜索
	Limit     int
}

// AuditStatsResult 审计统计结果
type AuditStatsResult struct {
	Total     int              `json:"total"`
	Earliest  *string          `json:"earliest"`
	Latest    *string          `json:"latest"`
	DiskBytes int64            `json:"disk_bytes"`
	Breakdown map[string]int   `json:"breakdown,omitempty"`
}

// TimelineBucket 时间线聚合桶
type TimelineBucket struct {
	Hour  string         `json:"hour"`
	Counts map[string]int `json:"counts"` // action -> count
}

// Store 存储接口 — 抽象所有数据库操作
type Store interface {
	// 审计
	LogAudit(entry *AuditEntry) error
	QueryAuditLogs(filter AuditFilter) ([]AuditEntry, error)
	CleanupAuditLogs(retentionDays int) (int, error)
	AuditStats() (*AuditStatsResult, error)
	AuditTimeline(hours int) ([]TimelineBucket, error)

	// 路由
	SaveRoute(senderID, appID, upstreamID string) error
	SaveRouteWithMeta(senderID, appID, upstreamID, department, displayName, email string) error
	DeleteRoute(senderID, appID string) error
	LoadRoutes() ([]RouteEntry, error)
	ListRoutesByApp(appID string) ([]RouteEntry, error)
	ListRoutesByDepartment(department string) ([]RouteEntry, error)
	UpdateRouteUserInfo(senderID, displayName, email, department string) error
	MigrateRoute(senderID, appID, toUpstreamID string) error
	RouteStats() (map[string]int, error) // department -> count

	// 用户信息缓存
	GetUserInfo(senderID string) (*UserInfo, error)
	SaveUserInfo(info *UserInfo) error
	ListUserInfo(department, email string) ([]*UserInfo, error)

	// 上游管理
	SaveUpstream(up *Upstream) error
	LoadUpstreams() ([]*Upstream, error)
	DeleteUpstream(id string) error

	// 生命周期
	Close() error
	Ping() error

	// 原始 DB（向后兼容 — 过渡期用）
	RawDB() *sql.DB
}

// ============================================================
// SQLiteStore — SQLite 实现
// ============================================================

type SQLiteStore struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// NewSQLiteStore 创建 SQLiteStore（使用已初始化的 *sql.DB）
func NewSQLiteStore(db *sql.DB, dbPath string) *SQLiteStore {
	return &SQLiteStore{db: db, path: dbPath}
}

func (s *SQLiteStore) RawDB() *sql.DB {
	return s.db
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteStore) Ping() error {
	// 执行简单的读写验证
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS _health_check (id INTEGER PRIMARY KEY, ts TEXT)`)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`INSERT OR REPLACE INTO _health_check (id, ts) VALUES (1, ?)`, now)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM _health_check WHERE id = 1`)
	return err
}

// ============================================================
// 审计日志操作
// ============================================================

func (s *SQLiteStore) LogAudit(entry *AuditEntry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	preview := entry.ContentPreview
	if rs := []rune(preview); len(rs) > 200 {
		preview = string(rs[:200]) + "..."
	}
	_, err := s.db.Exec(`INSERT INTO audit_log
		(timestamp,direction,sender_id,action,reason,content_preview,full_request_hash,latency_ms,upstream_id,app_id)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		entry.Timestamp, entry.Direction, entry.SenderID, entry.Action,
		entry.Reason, preview, entry.FullRequestHash, entry.LatencyMs,
		entry.UpstreamID, entry.AppID)
	return err
}

func (s *SQLiteStore) QueryAuditLogs(filter AuditFilter) ([]AuditEntry, error) {
	query := `SELECT id, timestamp, direction, sender_id, action, reason, content_preview, latency_ms, upstream_id, app_id FROM audit_log WHERE 1=1`
	var args []interface{}
	if filter.Direction != "" {
		query += ` AND direction=?`
		args = append(args, filter.Direction)
	}
	if filter.Action != "" {
		query += ` AND action=?`
		args = append(args, filter.Action)
	}
	if filter.SenderID != "" {
		query += ` AND sender_id=?`
		args = append(args, filter.SenderID)
	}
	if filter.AppID != "" {
		query += ` AND app_id=?`
		args = append(args, filter.AppID)
	}
	if filter.Query != "" {
		query += ` AND content_preview LIKE ?`
		args = append(args, "%"+filter.Query+"%")
	}
	query += ` ORDER BY id DESC`
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 10000 {
		limit = 10000
	}
	query += ` LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if rows.Scan(&e.ID, &e.Timestamp, &e.Direction, &e.SenderID, &e.Action, &e.Reason, &e.ContentPreview, &e.LatencyMs, &e.UpstreamID, &e.AppID) != nil {
			continue
		}
		results = append(results, e)
	}
	return results, nil
}

func (s *SQLiteStore) CleanupAuditLogs(retentionDays int) (int, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	result, err := s.db.Exec(`DELETE FROM audit_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (s *SQLiteStore) AuditStats() (*AuditStatsResult, error) {
	stats := &AuditStatsResult{}
	s.db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&stats.Total)

	var earliest, latest sql.NullString
	s.db.QueryRow(`SELECT MIN(timestamp) FROM audit_log`).Scan(&earliest)
	s.db.QueryRow(`SELECT MAX(timestamp) FROM audit_log`).Scan(&latest)
	if earliest.Valid {
		stats.Earliest = &earliest.String
	}
	if latest.Valid {
		stats.Latest = &latest.String
	}

	var pageCount, pageSize int
	s.db.QueryRow(`PRAGMA page_count`).Scan(&pageCount)
	s.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize)
	stats.DiskBytes = int64(pageCount) * int64(pageSize)

	// Breakdown
	rows, err := s.db.Query(`SELECT direction, action, COUNT(*) FROM audit_log GROUP BY direction, action`)
	if err == nil {
		defer rows.Close()
		stats.Breakdown = make(map[string]int)
		for rows.Next() {
			var dir, action string
			var cnt int
			if rows.Scan(&dir, &action, &cnt) == nil {
				stats.Breakdown[dir+"_"+action] = cnt
			}
		}
	}

	return stats, nil
}

func (s *SQLiteStore) AuditTimeline(hours int) ([]TimelineBucket, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.Query(`
		SELECT
			strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour_bucket,
			action,
			COUNT(*) as cnt
		FROM audit_log
		WHERE timestamp >= ?
		GROUP BY hour_bucket, action
		ORDER BY hour_bucket ASC
	`, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hourMap := map[string]map[string]int{}
	for rows.Next() {
		var hour, action string
		var count int
		if rows.Scan(&hour, &action, &count) == nil {
			if hourMap[hour] == nil {
				hourMap[hour] = map[string]int{}
			}
			hourMap[hour][action] = count
		}
	}

	var timeline []TimelineBucket
	for i := hours - 1; i >= 0; i-- {
		t := time.Now().UTC().Add(-time.Duration(i) * time.Hour)
		hourKey := t.Format("2006-01-02T15") + ":00:00Z"
		bucket := TimelineBucket{
			Hour:   hourKey,
			Counts: map[string]int{"pass": 0, "block": 0, "warn": 0},
		}
		if m, ok := hourMap[hourKey]; ok {
			for action, cnt := range m {
				bucket.Counts[action] = cnt
			}
		}
		timeline = append(timeline, bucket)
	}
	return timeline, nil
}

// ============================================================
// 路由操作
// ============================================================

func (s *SQLiteStore) SaveRoute(senderID, appID, upstreamID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, email, created_at, updated_at) VALUES(?,?,?,'','','',?,?)`,
		senderID, appID, upstreamID, now, now)
	return err
}

func (s *SQLiteStore) SaveRouteWithMeta(senderID, appID, upstreamID, department, displayName, email string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT OR REPLACE INTO user_routes (sender_id, app_id, upstream_id, department, display_name, email, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		senderID, appID, upstreamID, department, displayName, email, now, now)
	return err
}

func (s *SQLiteStore) DeleteRoute(senderID, appID string) error {
	_, err := s.db.Exec(`DELETE FROM user_routes WHERE sender_id = ? AND app_id = ?`, senderID, appID)
	return err
}

func (s *SQLiteStore) LoadRoutes() ([]RouteEntry, error) {
	rows, err := s.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, COALESCE(email,''), created_at, updated_at FROM user_routes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []RouteEntry
	for rows.Next() {
		var e RouteEntry
		if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.Email, &e.CreatedAt, &e.UpdatedAt) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (s *SQLiteStore) ListRoutesByApp(appID string) ([]RouteEntry, error) {
	rows, err := s.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, COALESCE(email,''), created_at, updated_at FROM user_routes WHERE app_id = ? ORDER BY updated_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []RouteEntry
	for rows.Next() {
		var e RouteEntry
		if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.Email, &e.CreatedAt, &e.UpdatedAt) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (s *SQLiteStore) ListRoutesByDepartment(department string) ([]RouteEntry, error) {
	rows, err := s.db.Query(`SELECT sender_id, app_id, upstream_id, department, display_name, COALESCE(email,''), created_at, updated_at FROM user_routes WHERE department = ? ORDER BY updated_at DESC`, department)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []RouteEntry
	for rows.Next() {
		var e RouteEntry
		if rows.Scan(&e.SenderID, &e.AppID, &e.UpstreamID, &e.Department, &e.DisplayName, &e.Email, &e.CreatedAt, &e.UpdatedAt) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (s *SQLiteStore) UpdateRouteUserInfo(senderID, displayName, email, department string) error {
	now := time.Now().Format(time.RFC3339)
	s.db.Exec(`UPDATE user_routes SET display_name=?, department=?, updated_at=? WHERE sender_id=? AND (display_name='' OR display_name IS NULL OR display_name!=?)`,
		displayName, department, now, senderID, displayName)
	s.db.Exec(`UPDATE user_routes SET email=?, updated_at=? WHERE sender_id=? AND (email='' OR email IS NULL OR email!=?)`,
		email, now, senderID, email)
	return nil
}

func (s *SQLiteStore) MigrateRoute(senderID, appID, toUpstreamID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE user_routes SET upstream_id=?, updated_at=? WHERE sender_id=? AND app_id=?`,
		toUpstreamID, now, senderID, appID)
	return err
}

func (s *SQLiteStore) RouteStats() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT COALESCE(department,''), COUNT(*) FROM user_routes GROUP BY department`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var dept string
		var cnt int
		if rows.Scan(&dept, &cnt) == nil && dept != "" {
			result[dept] = cnt
		}
	}
	return result, nil
}

// ============================================================
// 用户信息缓存操作
// ============================================================

func (s *SQLiteStore) GetUserInfo(senderID string) (*UserInfo, error) {
	var info UserInfo
	var fetchedAt string
	err := s.db.QueryRow(`SELECT sender_id, name, email, department, avatar, mobile, fetched_at FROM user_info_cache WHERE sender_id = ?`, senderID).
		Scan(&info.SenderID, &info.Name, &info.Email, &info.Department, &info.Avatar, &info.Mobile, &fetchedAt)
	if err != nil {
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339, fetchedAt)
	info.FetchedAt = t
	return &info, nil
}

func (s *SQLiteStore) SaveUserInfo(info *UserInfo) error {
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT OR REPLACE INTO user_info_cache (sender_id, name, email, department, avatar, mobile, fetched_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		info.SenderID, info.Name, info.Email, info.Department, info.Avatar, info.Mobile, info.FetchedAt.Format(time.RFC3339), now)
	return err
}

func (s *SQLiteStore) ListUserInfo(department, email string) ([]*UserInfo, error) {
	query := `SELECT sender_id, name, email, department, avatar, mobile, fetched_at FROM user_info_cache WHERE 1=1`
	var args []interface{}
	if department != "" {
		query += ` AND department = ?`
		args = append(args, department)
	}
	if email != "" {
		query += ` AND email LIKE ?`
		args = append(args, "%"+email+"%")
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*UserInfo
	for rows.Next() {
		var info UserInfo
		var fetchedAt string
		if rows.Scan(&info.SenderID, &info.Name, &info.Email, &info.Department, &info.Avatar, &info.Mobile, &fetchedAt) == nil {
			t, _ := time.Parse(time.RFC3339, fetchedAt)
			info.FetchedAt = t
			results = append(results, &info)
		}
	}
	return results, nil
}

// ============================================================
// 上游管理操作
// ============================================================

func (s *SQLiteStore) SaveUpstream(up *Upstream) error {
	tagsJSON := "{}"
	loadJSON := "{}"
	if up.Tags != nil {
		if b, err := jsonMarshalSafe(up.Tags); err == nil {
			tagsJSON = string(b)
		}
	}
	if up.Load != nil {
		if b, err := jsonMarshalSafe(up.Load); err == nil {
			loadJSON = string(b)
		}
	}
	h := 0
	if up.Healthy {
		h = 1
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO upstreams (id,address,port,healthy,registered_at,last_heartbeat,tags,load) VALUES(?,?,?,?,?,?,?,?)`,
		up.ID, up.Address, up.Port, h, up.RegisteredAt.Format(time.RFC3339), up.LastHeartbeat.Format(time.RFC3339),
		tagsJSON, loadJSON)
	return err
}

func (s *SQLiteStore) LoadUpstreams() ([]*Upstream, error) {
	rows, err := s.db.Query(`SELECT id, address, port, healthy, registered_at, last_heartbeat, tags, load FROM upstreams`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*Upstream
	for rows.Next() {
		var id, address, regAt, hbAt, tagsJSON, loadJSON string
		var port, healthy int
		if rows.Scan(&id, &address, &port, &healthy, &regAt, &hbAt, &tagsJSON, &loadJSON) != nil {
			continue
		}
		up := &Upstream{
			ID: id, Address: address, Port: port, Healthy: healthy == 1,
			Tags: map[string]string{}, Load: map[string]interface{}{},
		}
		up.RegisteredAt, _ = time.Parse(time.RFC3339, regAt)
		up.LastHeartbeat, _ = time.Parse(time.RFC3339, hbAt)
		jsonUnmarshalSafe([]byte(tagsJSON), &up.Tags)
		jsonUnmarshalSafe([]byte(loadJSON), &up.Load)
		results = append(results, up)
	}
	return results, nil
}

func (s *SQLiteStore) DeleteUpstream(id string) error {
	_, err := s.db.Exec(`DELETE FROM upstreams WHERE id = ?`, id)
	return err
}

// ============================================================
// 备份操作（v4.2）
// ============================================================

// Backup 使用 VACUUM INTO 创建数据库备份
func (s *SQLiteStore) Backup(backupDir string) (string, int64, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建备份目录失败: %w", err)
	}
	filename := fmt.Sprintf("lobster-guard-%s.db", time.Now().Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, filename)

	// 先做 WAL checkpoint 确保数据持久化
	s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)

	// 使用 VACUUM INTO 创建备份（SQLite 3.27+ 支持）
	_, err := s.db.Exec(`VACUUM INTO ?`, backupPath)
	if err != nil {
		return "", 0, fmt.Errorf("VACUUM INTO 备份失败: %w", err)
	}

	// 获取备份文件大小
	info, err := os.Stat(backupPath)
	if err != nil {
		return backupPath, 0, nil
	}
	return backupPath, info.Size(), nil
}

// ListBackups 列出备份目录中的所有备份
func ListBackups(backupDir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}
	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "lobster-guard-") || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(backupDir, entry.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	// 按时间降序排列
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name > backups[j].Name
	})
	return backups, nil
}

// CleanupOldBackups 删除超过 maxCount 的旧备份
func CleanupOldBackups(backupDir string, maxCount int) (int, error) {
	if maxCount <= 0 {
		return 0, nil
	}
	backups, err := ListBackups(backupDir)
	if err != nil {
		return 0, err
	}
	if len(backups) <= maxCount {
		return 0, nil
	}
	// 删除最旧的
	deleted := 0
	for i := maxCount; i < len(backups); i++ {
		if err := os.Remove(backups[i].Path); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

// DeleteBackup 删除指定备份文件
func DeleteBackup(backupDir, name string) error {
	// 安全检查：防止路径遍历
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid backup name")
	}
	path := filepath.Join(backupDir, name)
	return os.Remove(path)
}

// RestoreFromBackup 从备份文件恢复
func RestoreFromBackup(backupPath, targetPath string) error {
	// 读取备份文件
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}
	// 确保目标目录存在
	if dir := filepath.Dir(targetPath); dir != "" {
		os.MkdirAll(dir, 0755)
	}
	// 写入目标（覆盖）
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("写入数据库文件失败: %w", err)
	}
	// 删除 WAL/SHM 文件（恢复后重建）
	os.Remove(targetPath + "-wal")
	os.Remove(targetPath + "-shm")
	return nil
}

// BackupInfo 备份文件信息
type BackupInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// jsonMarshalSafe wraps json.Marshal for store usage
func jsonMarshalSafe(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// jsonUnmarshalSafe wraps json.Unmarshal for store usage
func jsonUnmarshalSafe(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
