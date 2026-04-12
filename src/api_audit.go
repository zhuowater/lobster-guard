package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (api *ManagementAPI) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	direction := r.URL.Query().Get("direction")
	action := r.URL.Query().Get("action")
	senderID := r.URL.Query().Get("sender_id")
	appID := r.URL.Query().Get("app_id")
	q := r.URL.Query().Get("q")
	traceID := r.URL.Query().Get("trace_id")
	sourceCategory := r.URL.Query().Get("source_category")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	// 支持 since 简写: from=24h → 转为 RFC3339
	if from != "" && !strings.Contains(from, "T") {
		from = parseSinceParam(from)
	}
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if limit > 10000 {
		limit = 10000
	}
	var logs []map[string]interface{}
	var err error
	if from != "" || to != "" {
		logs, err = api.logger.QueryLogsExFullTenant(direction, action, senderID, appID, q, traceID, from, to, tenantID, sourceCategory, limit)
	} else {
		logs, err = api.logger.QueryLogsExTenant(direction, action, senderID, appID, q, traceID, tenantID, sourceCategory, limit)
	}
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"logs": logs, "total": len(logs), "tenant": tenantID, "source_category": sourceCategory})
}

// handleAuditExport GET /api/v1/audit/export — 导出审计日志为 CSV 或 JSON（v3.10）
func (api *ManagementAPI) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "csv" && format != "json" {
		jsonResponse(w, 400, map[string]string{"error": "format must be 'csv' or 'json'"})
		return
	}
	direction := r.URL.Query().Get("direction")
	action := r.URL.Query().Get("action")
	senderID := r.URL.Query().Get("sender_id")
	appID := r.URL.Query().Get("app_id")
	q := r.URL.Query().Get("q")
	sourceCategory := r.URL.Query().Get("source_category")
	from := r.URL.Query().Get("from") // v12.1: 时间范围起始 (RFC3339 或 since 格式)
	to := r.URL.Query().Get("to")     // v12.1: 时间范围结束
	tenantID := ParseTenantParam(r.URL.Query().Get("tenant"))
	// 支持 since 简写: from=24h → 转为 RFC3339
	if from != "" && !strings.Contains(from, "T") {
		from = parseSinceParam(from)
	}
	limit := 1000
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	if limit > 10000 {
		limit = 10000
	}

	logs, err := api.logger.QueryLogsExFullTenant(direction, action, senderID, appID, q, "", from, to, tenantID, sourceCategory, limit)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")
		w.WriteHeader(200)
		cw := csv.NewWriter(w)
		// 写表头
		cw.Write([]string{"id", "timestamp", "direction", "sender_id", "action", "reason", "content_preview", "latency_ms", "upstream_id", "app_id", "source_categories", "source_keys", "source_tool_call_count"})
		for _, log := range logs {
			cw.Write([]string{
				fmt.Sprintf("%v", log["id"]),
				fmt.Sprintf("%v", log["timestamp"]),
				fmt.Sprintf("%v", log["direction"]),
				fmt.Sprintf("%v", log["sender_id"]),
				fmt.Sprintf("%v", log["action"]),
				fmt.Sprintf("%v", log["reason"]),
				fmt.Sprintf("%v", log["content_preview"]),
				fmt.Sprintf("%v", log["latency_ms"]),
				fmt.Sprintf("%v", log["upstream_id"]),
				fmt.Sprintf("%v", log["app_id"]),
				fmt.Sprintf("%v", log["source_categories"]),
				fmt.Sprintf("%v", log["source_keys"]),
				fmt.Sprintf("%v", log["source_tool_call_count"]),
			})
		}
		cw.Flush()
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.json")
		w.WriteHeader(200)
		if logs == nil {
			logs = []map[string]interface{}{}
		}
		json.NewEncoder(w).Encode(logs)
	}
}

// handleAuditCleanup POST /api/v1/audit/cleanup — 手动触发日志清理（v3.10）
func (api *ManagementAPI) handleAuditCleanup(w http.ResponseWriter, r *http.Request) {
	retentionDays := api.cfg.AuditRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	deleted, err := api.logger.CleanupOldLogs(retentionDays)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[审计] 手动清理了 %d 条过期日志（超过 %d 天）", deleted, retentionDays)
	jsonResponse(w, 200, map[string]interface{}{
		"status":         "cleaned",
		"deleted":        deleted,
		"retention_days": retentionDays,
	})
}

// handleAuditStats GET /api/v1/audit/stats — 日志统计信息（v3.10）
func (api *ManagementAPI) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	stats := api.logger.AuditStats()
	jsonResponse(w, 200, stats)
}

// handleAuditTimeline GET /api/v1/audit/timeline — 时间线统计（v3.10）
func (api *ManagementAPI) handleAuditTimeline(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			hours = n
		}
	}
	timeline := api.logger.Timeline(hours)
	if timeline == nil {
		timeline = []map[string]interface{}{}
	}
	jsonResponse(w, 200, map[string]interface{}{"timeline": timeline, "hours": hours})
}

func (api *ManagementAPI) handleAuditArchives(w http.ResponseWriter, r *http.Request) {
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	archives, err := ListArchives(archiveDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"archives": archives, "total": len(archives)})
}

// handleAuditArchiveDownload GET /api/v1/audit/archives/:name — 下载归档文件
func (api *ManagementAPI) handleAuditArchiveDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/audit/archives/")
	if name == "" {
		jsonResponse(w, 400, map[string]string{"error": "archive name required"})
		return
	}
	// 安全检查：不允许路径穿越
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		jsonResponse(w, 400, map[string]string{"error": "invalid archive name"})
		return
	}
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	filePath := archiveDir + name
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		jsonResponse(w, 404, map[string]string{"error": "archive not found"})
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	http.ServeFile(w, r, filePath)
}

// handleAuditArchiveNow POST /api/v1/audit/archive — 手动触发归档
func (api *ManagementAPI) handleAuditArchiveNow(w http.ResponseWriter, r *http.Request) {
	archiveDir := api.cfg.AuditArchiveDir
	if archiveDir == "" {
		archiveDir = "/var/lib/lobster-guard/archives/"
	}
	retentionDays := api.cfg.AuditRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	path, deleted, err := api.logger.ArchiveLogs(retentionDays, archiveDir)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if path == "" {
		jsonResponse(w, 200, map[string]interface{}{
			"status":  "no_data",
			"message": "没有需要归档的日志",
		})
		return
	}
	log.Printf("[归档] 手动归档完成: %s，删除 %d 条", path, deleted)
	jsonResponse(w, 200, map[string]interface{}{
		"status":  "archived",
		"path":    path,
		"deleted": deleted,
	})
}

// ============================================================
// v5.1 智能检测 API
// ============================================================

// handleListRuleTemplates GET /api/v1/rule-templates — 列出可用规则模板（v30.0: 转发到入站模板列表）
