package main

import (
	"database/sql"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type sqliteTableStat struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

func (api *ManagementAPI) handleSQLiteStats(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.logger == nil || api.logger.DB() == nil {
		jsonResponse(w, 200, map[string]interface{}{"error": "database unavailable"})
		return
	}
	db := api.logger.DB()
	dbPath := ""
	if api.cfg != nil {
		dbPath = api.cfg.DBPath
	}
	result := map[string]interface{}{
		"database": map[string]interface{}{
			"path": dbPath,
		},
		"tables":    []sqliteTableStat{},
		"write_qps": 0.0,
	}
	if dbPath != "" {
		if fi, err := os.Stat(dbPath); err == nil {
			result["database"].(map[string]interface{})["size_bytes"] = fi.Size()
			result["database"].(map[string]interface{})["size_human"] = formatBytes(fi.Size())
		}
		if fi, err := os.Stat(dbPath + "-wal"); err == nil {
			result["database"].(map[string]interface{})["wal_size_bytes"] = fi.Size()
			result["database"].(map[string]interface{})["wal_size_human"] = formatBytes(fi.Size())
		} else {
			result["database"].(map[string]interface{})["wal_size_bytes"] = int64(0)
			result["database"].(map[string]interface{})["wal_size_human"] = formatBytes(0)
		}
	}
	pragmas := map[string]interface{}{}
	var pageCount, pageSize, walAutoCheckpoint int64
	_ = db.QueryRow(`PRAGMA page_count`).Scan(&pageCount)
	_ = db.QueryRow(`PRAGMA page_size`).Scan(&pageSize)
	_ = db.QueryRow(`PRAGMA wal_autocheckpoint`).Scan(&walAutoCheckpoint)
	pragmas["page_count"] = pageCount
	pragmas["page_size"] = pageSize
	pragmas["wal_autocheckpoint"] = walAutoCheckpoint
	result["pragmas"] = pragmas

	tables, err := listSQLiteTables(db)
	if err == nil {
		result["table_count"] = len(tables)
		stats := make([]sqliteTableStat, 0, len(tables))
		for _, name := range tables {
			count, qerr := countRows(db, name)
			if qerr != nil {
				continue
			}
			stats = append(stats, sqliteTableStat{Name: name, Rows: count})
		}
		sort.Slice(stats, func(i, j int) bool {
			if stats[i].Rows == stats[j].Rows {
				return stats[i].Name < stats[j].Name
			}
			return stats[i].Rows > stats[j].Rows
		})
		if len(stats) > 10 {
			stats = stats[:10]
		}
		result["tables"] = stats
	}
	var recentWrites int64
	oneMinuteAgo := time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339Nano)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE timestamp >= ?`, oneMinuteAgo).Scan(&recentWrites)
	result["recent_writes_1m"] = recentWrites
	result["write_qps"] = float64(recentWrites) / 60.0
	jsonResponse(w, 200, result)
}

func listSQLiteTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && name != "" {
			tables = append(tables, name)
		}
	}
	return tables, nil
}

func countRows(db *sql.DB, table string) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`
	err := db.QueryRow(query).Scan(&count)
	return count, err
}
