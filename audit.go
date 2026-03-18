// audit.go — AuditLogger、审计日志查询/导出/清理/时间线/归档
// lobster-guard v4.0 代码拆分 + v5.0 trace_id + 归档
package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 审计日志
// ============================================================

type AuditLogger struct {
	db   *sql.DB
	mu   sync.Mutex
	stmt *sql.Stmt
}

func NewAuditLogger(db *sql.DB) (*AuditLogger, error) {
	stmt, err := db.Prepare(`INSERT INTO audit_log
		(timestamp,direction,sender_id,action,reason,content_preview,full_request_hash,latency_ms,upstream_id,app_id,trace_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil { return nil, err }
	return &AuditLogger{db: db, stmt: stmt}, nil
}

func (al *AuditLogger) Log(dir, sender, action, reason, preview, hash string, latMs float64, upstreamID, appID string) {
	al.LogWithTrace(dir, sender, action, reason, preview, hash, latMs, upstreamID, appID, "")
}

func (al *AuditLogger) LogWithTrace(dir, sender, action, reason, preview, hash string, latMs float64, upstreamID, appID, traceID string) {
	go func() {
		defer func() { recover() }()
		al.mu.Lock(); defer al.mu.Unlock()
		if rs := []rune(preview); len(rs) > 200 { preview = string(rs[:200]) + "..." }
		al.stmt.Exec(time.Now().UTC().Format(time.RFC3339Nano), dir, sender, action, reason, preview, hash, latMs, upstreamID, appID, traceID)
	}()
}

func (al *AuditLogger) Close() {
	if al == nil { return }
	if al.stmt != nil { al.stmt.Close() }
}

// DB returns the underlying database handle
func (al *AuditLogger) DB() *sql.DB {
	return al.db
}

func (al *AuditLogger) QueryLogs(direction, action, senderID string, limit int) ([]map[string]interface{}, error) {
	return al.QueryLogsEx(direction, action, senderID, "", "", limit)
}

// QueryLogsEx 扩展查询：支持 app_id、全文搜索 q 和 trace_id 参数（v3.10 + v5.0）
func (al *AuditLogger) QueryLogsEx(direction, action, senderID, appID, q string, limit int) ([]map[string]interface{}, error) {
	return al.QueryLogsExTrace(direction, action, senderID, appID, q, "", limit)
}

// QueryLogsExTrace 支持 trace_id 筛选
func (al *AuditLogger) QueryLogsExTrace(direction, action, senderID, appID, q, traceID string, limit int) ([]map[string]interface{}, error) {
	return al.QueryLogsExFull(direction, action, senderID, appID, q, traceID, "", "", limit)
}

// QueryLogsExFull 完整查询（支持 from/to 时间范围）
func (al *AuditLogger) QueryLogsExFull(direction, action, senderID, appID, q, traceID, from, to string, limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, timestamp, direction, sender_id, action, reason, content_preview, latency_ms, upstream_id, app_id, COALESCE(trace_id,'') FROM audit_log WHERE 1=1`
	var args []interface{}
	if direction != "" { query += ` AND direction=?`; args = append(args, direction) }
	if action != "" { query += ` AND action=?`; args = append(args, action) }
	if senderID != "" { query += ` AND sender_id=?`; args = append(args, senderID) }
	if appID != "" { query += ` AND app_id=?`; args = append(args, appID) }
	if q != "" { query += ` AND content_preview LIKE ?`; args = append(args, "%"+q+"%") }
	if traceID != "" { query += ` AND trace_id=?`; args = append(args, traceID) }
	if from != "" { query += ` AND timestamp >= ?`; args = append(args, from) }
	if to != "" { query += ` AND timestamp <= ?`; args = append(args, to) }
	query += ` ORDER BY id DESC`
	if limit <= 0 { limit = 50 }
	if limit > 10000 { limit = 10000 }
	query += ` LIMIT ?`; args = append(args, limit)

	rows, err := al.db.Query(query, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id int; var ts, dir, sid, act, reason, preview, uid, aid, tid string; var latMs float64
		if rows.Scan(&id, &ts, &dir, &sid, &act, &reason, &preview, &latMs, &uid, &aid, &tid) != nil { continue }
		results = append(results, map[string]interface{}{
			"id": id, "timestamp": ts, "direction": dir, "sender_id": sid,
			"action": act, "reason": reason, "content_preview": preview,
			"latency_ms": latMs, "upstream_id": uid, "app_id": aid, "trace_id": tid,
		})
	}
	return results, nil
}

// CleanupOldLogs 清理超过指定天数的日志（v3.10）
func (al *AuditLogger) CleanupOldLogs(retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	result, err := al.db.Exec(`DELETE FROM audit_log WHERE timestamp < ?`, cutoff)
	if err != nil { return 0, err }
	return result.RowsAffected()
}

// AuditStats 返回审计日志统计信息（v3.10）
func (al *AuditLogger) AuditStats() map[string]interface{} {
	stats := map[string]interface{}{}
	var total int
	al.db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&total)
	stats["total"] = total

	var earliest, latest sql.NullString
	al.db.QueryRow(`SELECT MIN(timestamp) FROM audit_log`).Scan(&earliest)
	al.db.QueryRow(`SELECT MAX(timestamp) FROM audit_log`).Scan(&latest)
	if earliest.Valid { stats["earliest"] = earliest.String } else { stats["earliest"] = nil }
	if latest.Valid { stats["latest"] = latest.String } else { stats["latest"] = nil }

	// 估算磁盘占用（SQLite page_count * page_size）
	var pageCount, pageSize int
	al.db.QueryRow(`PRAGMA page_count`).Scan(&pageCount)
	al.db.QueryRow(`PRAGMA page_size`).Scan(&pageSize)
	stats["disk_bytes"] = int64(pageCount) * int64(pageSize)

	return stats
}

// Timeline 按小时聚合审计日志（v3.10）
func (al *AuditLogger) Timeline(hours int) []map[string]interface{} {
	if hours <= 0 { hours = 24 }
	if hours > 168 { hours = 168 } // 最多 7 天
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	rows, err := al.db.Query(`
		SELECT
			strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour_bucket,
			action,
			COUNT(*) as cnt
		FROM audit_log
		WHERE timestamp >= ?
		GROUP BY hour_bucket, action
		ORDER BY hour_bucket ASC
	`, since.Format(time.RFC3339))
	if err != nil { return nil }
	defer rows.Close()

	// 收集数据
	type hourAction struct {
		hour   string
		action string
		count  int
	}
	var data []hourAction
	for rows.Next() {
		var ha hourAction
		if rows.Scan(&ha.hour, &ha.action, &ha.count) == nil {
			data = append(data, ha)
		}
	}

	// 生成完整的每小时时间线
	hourMap := map[string]map[string]int{}
	for _, d := range data {
		if hourMap[d.hour] == nil { hourMap[d.hour] = map[string]int{} }
		hourMap[d.hour][d.action] = d.count
	}

	// 填充所有小时槽
	var timeline []map[string]interface{}
	for i := hours - 1; i >= 0; i-- {
		t := time.Now().UTC().Add(-time.Duration(i) * time.Hour)
		hourKey := t.Format("2006-01-02T15") + ":00:00Z"
		entry := map[string]interface{}{
			"hour":  hourKey,
			"pass":  0,
			"block": 0,
			"warn":  0,
		}
		if m, ok := hourMap[hourKey]; ok {
			for action, cnt := range m {
				entry[action] = cnt
			}
		}
		timeline = append(timeline, entry)
	}
	return timeline
}

func (al *AuditLogger) Stats() map[string]interface{} {
	return al.StatsWithFilter("")
}

// StatsWithFilter 带时间过滤的统计（v11.4）
// sinceRFC3339 为空则全量，否则 WHERE timestamp >= sinceRFC3339
func (al *AuditLogger) StatsWithFilter(sinceRFC3339 string) map[string]interface{} {
	stats := map[string]interface{}{}
	var total int
	if sinceRFC3339 != "" {
		al.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE timestamp >= ?`, sinceRFC3339).Scan(&total)
	} else {
		al.db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&total)
	}
	stats["total"] = total
	var query string
	var args []interface{}
	if sinceRFC3339 != "" {
		query = `SELECT direction, action, COUNT(*) FROM audit_log WHERE timestamp >= ? GROUP BY direction, action`
		args = append(args, sinceRFC3339)
	} else {
		query = `SELECT direction, action, COUNT(*) FROM audit_log GROUP BY direction, action`
	}
	rows, err := al.db.Query(query, args...)
	if err != nil { return stats }
	defer rows.Close()
	breakdown := map[string]interface{}{}
	for rows.Next() {
		var dir, action string; var cnt int
		if rows.Scan(&dir, &action, &cnt) == nil {
			breakdown[dir+"_"+action] = cnt
		}
	}
	stats["breakdown"] = breakdown
	return stats
}

// ============================================================
// v5.0 审计日志归档
// ============================================================

// ArchiveLogs 将超过 retentionDays 天的日志导出为压缩 JSON 文件，然后从 SQLite 删除
func (al *AuditLogger) ArchiveLogs(retentionDays int, archiveDir string) (string, int64, error) {
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", 0, fmt.Errorf("创建归档目录失败: %w", err)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)

	// 查询待归档的日志
	rows, err := al.db.Query(`SELECT id, timestamp, direction, sender_id, action, reason, content_preview, latency_ms, upstream_id, app_id, COALESCE(trace_id,'') FROM audit_log WHERE timestamp < ? ORDER BY id ASC`, cutoff)
	if err != nil {
		return "", 0, fmt.Errorf("查询待归档日志失败: %w", err)
	}
	defer rows.Close()

	var logs []map[string]interface{}
	var maxID int
	for rows.Next() {
		var id int
		var ts, dir, sid, act, reason, preview, uid, aid, tid string
		var latMs float64
		if rows.Scan(&id, &ts, &dir, &sid, &act, &reason, &preview, &latMs, &uid, &aid, &tid) != nil {
			continue
		}
		logs = append(logs, map[string]interface{}{
			"id": id, "timestamp": ts, "direction": dir, "sender_id": sid,
			"action": act, "reason": reason, "content_preview": preview,
			"latency_ms": latMs, "upstream_id": uid, "app_id": aid, "trace_id": tid,
		})
		if id > maxID {
			maxID = id
		}
	}

	if len(logs) == 0 {
		return "", 0, nil // 无需归档
	}

	// 写入压缩 JSON 文件
	dateStr := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("audit-%s.json.gz", dateStr)
	filePath := filepath.Join(archiveDir, filename)

	// 如果文件已存在，追加日期序号
	if _, err := os.Stat(filePath); err == nil {
		for i := 1; i < 100; i++ {
			filename = fmt.Sprintf("audit-%s-%d.json.gz", dateStr, i)
			filePath = filepath.Join(archiveDir, filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				break
			}
		}
	}

	f, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("创建归档文件失败: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return "", 0, fmt.Errorf("创建 gzip writer 失败: %w", err)
	}

	encoder := json.NewEncoder(gz)
	for _, l := range logs {
		if err := encoder.Encode(l); err != nil {
			gz.Close()
			return "", 0, fmt.Errorf("写入归档日志失败: %w", err)
		}
	}
	if err := gz.Close(); err != nil {
		return "", 0, fmt.Errorf("关闭 gzip writer 失败: %w", err)
	}

	// 从 SQLite 删除已归档的日志
	result, err := al.db.Exec(`DELETE FROM audit_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return filePath, int64(len(logs)), fmt.Errorf("归档文件已创建但删除旧日志失败: %w", err)
	}
	deleted, _ := result.RowsAffected()

	return filePath, deleted, nil
}

// ListArchives 列出归档目录中的所有归档文件
func ListArchives(archiveDir string) ([]map[string]interface{}, error) {
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}

	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}

	var archives []map[string]interface{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		archives = append(archives, map[string]interface{}{
			"name":     name,
			"size":     info.Size(),
			"mod_time": info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	// 按修改时间倒序
	sort.Slice(archives, func(i, j int) bool {
		return archives[i]["mod_time"].(string) > archives[j]["mod_time"].(string)
	})

	if archives == nil {
		archives = []map[string]interface{}{}
	}
	return archives, nil
}


// ============================================================
// 数据库初始化
// ============================================================

func initDB(dbPath string) (*sql.DB, error) {
	if idx := strings.LastIndex(dbPath, "/"); idx > 0 {
		os.MkdirAll(dbPath[:idx], 0755)
	}
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil { return nil, err }

	// v2.0 schema
	schema := `
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		direction TEXT NOT NULL,
		sender_id TEXT,
		action TEXT NOT NULL,
		reason TEXT,
		content_preview TEXT,
		full_request_hash TEXT,
		latency_ms REAL,
		upstream_id TEXT DEFAULT '',
		app_id TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_ts ON audit_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_dir ON audit_log(direction);
	CREATE INDEX IF NOT EXISTS idx_act ON audit_log(action);
	CREATE INDEX IF NOT EXISTS idx_sender ON audit_log(sender_id);

	CREATE TABLE IF NOT EXISTS upstreams (
		id TEXT PRIMARY KEY,
		address TEXT NOT NULL,
		port INTEGER NOT NULL,
		healthy INTEGER DEFAULT 1,
		registered_at TEXT NOT NULL,
		last_heartbeat TEXT,
		tags TEXT DEFAULT '{}',
		load TEXT DEFAULT '{}'
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库 schema 失败: %w", err)
	}

	// 为旧表增加 upstream_id 列（v1.0 升级兼容）
	db.Exec(`ALTER TABLE audit_log ADD COLUMN upstream_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE audit_log ADD COLUMN app_id TEXT DEFAULT ''`)
	// v5.0: trace_id 列
	db.Exec(`ALTER TABLE audit_log ADD COLUMN trace_id TEXT DEFAULT ''`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_trace ON audit_log(trace_id)`)

	// v9.0: LLM 审计表（LLMAuditor 会初始化，但确保表结构存在）
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		trace_id TEXT,
		model TEXT,
		request_tokens INTEGER,
		response_tokens INTEGER,
		total_tokens INTEGER,
		latency_ms REAL,
		status_code INTEGER,
		has_tool_use INTEGER DEFAULT 0,
		tool_count INTEGER DEFAULT 0,
		error_message TEXT
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_calls_ts ON llm_calls(timestamp)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER REFERENCES llm_calls(id),
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_input_preview TEXT,
		tool_result_preview TEXT,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0,
		flag_reason TEXT
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_ts ON llm_tool_calls(timestamp)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_risk ON llm_tool_calls(risk_level)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_tool_calls_tool ON llm_tool_calls(tool_name)`)

	// v3.8 user_routes schema migration
	migrateUserRoutes(db)

	// v3.9 user_info_cache table
	db.Exec(`CREATE TABLE IF NOT EXISTS user_info_cache (
		sender_id TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		email TEXT DEFAULT '',
		department TEXT DEFAULT '',
		avatar TEXT DEFAULT '',
		mobile TEXT DEFAULT '',
		fetched_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_email ON user_info_cache(email)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_dept ON user_info_cache(department)`)

	return db, nil
}

// migrateUserRoutes 处理 user_routes 表的 schema 迁移
// 检测旧表（只有 sender_id 主键），如果存在则迁移数据到新 schema
func migrateUserRoutes(db *sql.DB) {
	// 检查 user_routes 表是否存在
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='user_routes'`).Scan(&tableName)
	if err != nil {
		// 表不存在，直接创建新 schema
		db.Exec(`CREATE TABLE IF NOT EXISTS user_routes (
			sender_id TEXT NOT NULL,
			app_id TEXT NOT NULL DEFAULT '',
			upstream_id TEXT NOT NULL,
			department TEXT DEFAULT '',
			display_name TEXT DEFAULT '',
			email TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (sender_id, app_id)
		)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_upstream ON user_routes(upstream_id)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_app ON user_routes(app_id)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_dept ON user_routes(department)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_email ON user_routes(email)`)
		return
	}

	// 表存在，检查是否有 app_id 列
	rows, err := db.Query(`PRAGMA table_info(user_routes)`)
	if err != nil { return }
	defer rows.Close()
	hasAppID := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk) == nil {
			if name == "app_id" { hasAppID = true }
		}
	}

	if hasAppID {
		// 已经是新 schema，只需确保索引存在 + v3.9 email 列
		db.Exec(`ALTER TABLE user_routes ADD COLUMN email TEXT DEFAULT ''`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_upstream ON user_routes(upstream_id)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_app ON user_routes(app_id)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_dept ON user_routes(department)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_email ON user_routes(email)`)
		return
	}

	// 旧 schema，需要迁移
	log.Println("[数据库迁移] 检测到旧版 user_routes 表，开始迁移到 v3.8 schema...")

	// 1. 读取旧数据
	oldRows, err := db.Query(`SELECT sender_id, upstream_id, created_at, updated_at FROM user_routes`)
	if err != nil {
		log.Printf("[数据库迁移] 读取旧数据失败: %v", err)
		return
	}
	type oldRoute struct {
		senderID, upstreamID, createdAt, updatedAt string
	}
	var oldData []oldRoute
	for oldRows.Next() {
		var r oldRoute
		if oldRows.Scan(&r.senderID, &r.upstreamID, &r.createdAt, &r.updatedAt) == nil {
			oldData = append(oldData, r)
		}
	}
	oldRows.Close()

	// 2. 重建表
	db.Exec(`ALTER TABLE user_routes RENAME TO user_routes_old`)
	db.Exec(`CREATE TABLE user_routes (
		sender_id TEXT NOT NULL,
		app_id TEXT NOT NULL DEFAULT '',
		upstream_id TEXT NOT NULL,
		department TEXT DEFAULT '',
		display_name TEXT DEFAULT '',
		email TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (sender_id, app_id)
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_upstream ON user_routes(upstream_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_app ON user_routes(app_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_dept ON user_routes(department)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_routes_email ON user_routes(email)`)

	// 3. 迁移数据（旧数据 app_id 设为空字符串）
	for _, r := range oldData {
		db.Exec(`INSERT INTO user_routes (sender_id, app_id, upstream_id, department, display_name, email, created_at, updated_at) VALUES(?,?,?,'','','',?,?)`,
			r.senderID, "", r.upstreamID, r.createdAt, r.updatedAt)
	}

	// 4. 删除旧表
	db.Exec(`DROP TABLE IF EXISTS user_routes_old`)

	log.Printf("[数据库迁移] 迁移完成，%d 条路由已升级到 v3.8 schema", len(oldData))
}

