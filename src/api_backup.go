package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func (api *ManagementAPI) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	sqlStore, ok := api.store.(*SQLiteStore)
	if !ok {
		jsonResponse(w, 500, map[string]string{"error": "backup only supported for SQLite store"})
		return
	}

	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	path, size, err := sqlStore.Backup(backupDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// 自动清理旧备份
	maxCount := api.cfg.BackupMaxCount
	if maxCount <= 0 {
		maxCount = 10
	}
	CleanupOldBackups(backupDir, maxCount)

	log.Printf("[备份] ✅ 手动创建备份: %s (%.2f MB)", path, float64(size)/1024/1024)
	jsonResponse(w, 200, map[string]interface{}{
		"status": "created",
		"path":   path,
		"size":   size,
	})
}

// handleListBackups GET /api/v1/backups — 列出已有备份
func (api *ManagementAPI) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	backups, err := ListBackups(backupDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{
		"backups": backups,
		"total":   len(backups),
	})
}

// handleDeleteBackup DELETE /api/v1/backups/:name — 删除指定备份
func (api *ManagementAPI) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/backups/")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "backup name required"})
		return
	}

	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}

	if err := DeleteBackup(backupDir, name); err != nil {
		jsonResponse(w, 404, map[string]string{"error": err.Error()})
		return
	}

	log.Printf("[备份] 已删除备份: %s", name)
	jsonResponse(w, 200, map[string]string{"status": "deleted", "name": name})
}

// ============================================================
// v5.0 实时监控 API
// ============================================================

// handleRealtimeMetrics GET /api/v1/metrics/realtime — 返回最近 60 秒逐秒统计
func (api *ManagementAPI) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	// Extract name: /api/v1/backups/{name}/restore
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/backups/")
	name := strings.TrimSuffix(path, "/restore")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		jsonResponse(w, 400, map[string]string{"error": "invalid backup name"})
		return
	}
	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}
	backupPath := fmt.Sprintf("%s%s", backupDir, name)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		jsonResponse(w, 404, map[string]string{"error": "backup not found"})
		return
	}
	if err := RestoreFromBackup(backupPath, api.cfg.DBPath); err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[备份] ✅ 从备份恢复: %s", name)
	jsonResponse(w, 200, map[string]string{"status": "restored", "name": name})
}

// handleDownloadBackup GET /api/v1/backups/:name/download — 下载备份文件
func (api *ManagementAPI) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/backups/")
	name := strings.TrimSuffix(path, "/download")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		jsonResponse(w, 400, map[string]string{"error": "invalid backup name"})
		return
	}
	backupDir := api.cfg.BackupDir
	if backupDir == "" {
		backupDir = "/var/lib/lobster-guard/backups/"
	}
	filePath := fmt.Sprintf("%s%s", backupDir, name)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonResponse(w, 404, map[string]string{"error": "backup not found"})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	http.ServeFile(w, r, filePath)
}

// formatBytes 格式化字节数为可读字符串
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ============================================================
// v9.0 LLM 侧安全审计 API
// ============================================================

// handleLLMStatus GET /api/v1/llm/status — LLM 代理状态
