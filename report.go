// report.go — 报告模板引擎：安全日报/周报/月报
// lobster-guard v12.0 — "给领导看的，不是给开发者看的"
package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================
// 类型定义
// ============================================================

// ReportType 报告类型
type ReportType string

const (
	ReportDaily   ReportType = "daily"
	ReportWeekly  ReportType = "weekly"
	ReportMonthly ReportType = "monthly"
)

// ReportMeta 报告元数据（存 SQLite）
type ReportMeta struct {
	ID        string     `json:"id"`
	Type      ReportType `json:"type"`
	Title     string     `json:"title"`
	TimeRange string     `json:"time_range"`
	FromTime  string     `json:"from_time"`
	ToTime    string     `json:"to_time"`
	CreatedAt string     `json:"created_at"`
	FilePath  string     `json:"-"`
	FileSize  int64      `json:"file_size"`
	Status    string     `json:"status"`
}

// ReportData 报告数据（聚合所有维度）
type ReportData struct {
	Meta ReportMeta

	// 安全健康分
	HealthScore      int
	HealthLevel      string
	HealthLevelLabel string
	HealthColor      string
	Deductions       []HealthDeduction

	// IM 安全域
	IMTotal     int64
	IMBlocked   int64
	IMWarned    int64
	IMBlockRate string
	IMTopRules  []reportRuleHit

	// LLM 安全域
	LLMTotalCalls   int
	LLMTotalTokens  int64
	LLMErrorRate    string
	LLMEstCostUSD   string
	OWASPItems      []reportOWASP
	LLMTopTools     []reportToolStat

	// 高风险用户
	TopRiskUsers []reportUser

	// 异常检测
	AnomalyAlerts []reportAnomaly
	AnomalyCount  int

	// 建议
	Recommendations []string

	// 系统
	Version   string
	Generated string
}

type reportRuleHit struct {
	Name  string
	Group string
	Hits  int64
}

type reportOWASP struct {
	ID        string
	Name      string
	NameZh    string
	Count     int64
	RiskLevel string
	RiskColor string
}

type reportUser struct {
	UserID    string
	RiskScore int
	RiskLevel string
	RiskColor string
	BlockRate string
	Requests  int64
}

type reportAnomaly struct {
	MetricName  string
	DisplayName string
	Expected    float64
	Actual      float64
	Deviation   float64
	Severity    string
	SevColor    string
	Timestamp   string
}

type reportToolStat struct {
	Name  string
	Count int
}

// ============================================================
// ReportEngine
// ============================================================

type ReportEngine struct {
	db             *sql.DB
	healthEngine   *HealthScoreEngine
	owaspEngine    *OWASPMatrixEngine
	llmAuditor     *LLMAuditor
	userProfileEng *UserProfileEngine
	anomalyDet     *AnomalyDetector
	auditLogger    *AuditLogger
	reportsDir     string
}

func NewReportEngine(db *sql.DB, reportsDir string) *ReportEngine {
	if reportsDir == "" {
		reportsDir = "/var/lib/lobster-guard/reports/"
	}
	os.MkdirAll(reportsDir, 0755)

	db.Exec(`CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		time_range TEXT NOT NULL,
		from_time TEXT NOT NULL,
		to_time TEXT NOT NULL,
		created_at TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_size INTEGER DEFAULT 0,
		status TEXT DEFAULT 'generating'
	)`)

	return &ReportEngine{db: db, reportsDir: reportsDir}
}

func (e *ReportEngine) SetEngines(health *HealthScoreEngine, owasp *OWASPMatrixEngine, llm *LLMAuditor, userProfile *UserProfileEngine, anomaly *AnomalyDetector, audit *AuditLogger) {
	e.healthEngine = health
	e.owaspEngine = owasp
	e.llmAuditor = llm
	e.userProfileEng = userProfile
	e.anomalyDet = anomaly
	e.auditLogger = audit
}

func (e *ReportEngine) Generate(reportType ReportType) (*ReportMeta, error) {
	now := time.Now().UTC()
	id := fmt.Sprintf("rpt-%s-%s", reportType, now.Format("20060102-150405"))

	var timeRange, title string
	var since time.Time
	switch reportType {
	case ReportDaily:
		timeRange = "24h"
		title = fmt.Sprintf("安全日报 %s", now.Format("2006-01-02"))
		since = now.Add(-24 * time.Hour)
	case ReportWeekly:
		timeRange = "7d"
		title = fmt.Sprintf("安全周报 %s", now.Format("2006-01-02"))
		since = now.AddDate(0, 0, -7)
	case ReportMonthly:
		timeRange = "30d"
		title = fmt.Sprintf("月度审计报告 %s", now.Format("2006-01"))
		since = now.AddDate(0, 0, -30)
	default:
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}

	sinceStr := since.Format(time.RFC3339)
	filePath := filepath.Join(e.reportsDir, id+".html")

	meta := ReportMeta{
		ID: id, Type: reportType, Title: title, TimeRange: timeRange,
		FromTime: sinceStr, ToTime: now.Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339), FilePath: filePath, Status: "generating",
	}

	_, err := e.db.Exec(`INSERT INTO reports (id, type, title, time_range, from_time, to_time, created_at, file_path, file_size, status) VALUES (?,?,?,?,?,?,?,?,0,'generating')`,
		meta.ID, meta.Type, meta.Title, meta.TimeRange, meta.FromTime, meta.ToTime, meta.CreatedAt, meta.FilePath)
	if err != nil {
		return nil, fmt.Errorf("insert report meta: %w", err)
	}

	data := e.aggregateData(sinceStr, meta)

	htmlContent, err := renderReportHTML(data)
	if err != nil {
		e.db.Exec(`UPDATE reports SET status='failed' WHERE id=?`, meta.ID)
		return nil, fmt.Errorf("render HTML: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		e.db.Exec(`UPDATE reports SET status='failed' WHERE id=?`, meta.ID)
		return nil, fmt.Errorf("write file: %w", err)
	}

	fi, _ := os.Stat(filePath)
	fileSize := int64(0)
	if fi != nil {
		fileSize = fi.Size()
	}
	meta.FileSize = fileSize
	meta.Status = "ready"
	e.db.Exec(`UPDATE reports SET status='ready', file_size=? WHERE id=?`, fileSize, meta.ID)

	log.Printf("[报告] ✅ 生成 %s: %s (%d bytes)", reportType, filePath, fileSize)
	return &meta, nil
}

func (e *ReportEngine) aggregateData(sinceRFC3339 string, meta ReportMeta) ReportData {
	data := ReportData{
		Meta: meta, Version: AppVersion,
		Generated: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}

	if e.healthEngine != nil {
		if hs, err := e.healthEngine.Calculate(); err == nil {
			data.HealthScore = hs.Score
			data.HealthLevel = hs.Level
			data.HealthLevelLabel = hs.LevelLabel
			data.HealthColor = healthScoreColor(hs.Level)
			data.Deductions = hs.Deductions
		}
	}

	if e.auditLogger != nil {
		st := e.auditLogger.StatsWithFilter(sinceRFC3339)
		total, _ := st["total"].(int)
		data.IMTotal = int64(total)
		if breakdown, ok := st["breakdown"].(map[string]interface{}); ok {
			for k, v := range breakdown {
				cnt, _ := v.(int)
				if strings.Contains(k, "block") {
					data.IMBlocked += int64(cnt)
				}
				if strings.Contains(k, "warn") {
					data.IMWarned += int64(cnt)
				}
			}
		}
		if data.IMTotal > 0 {
			data.IMBlockRate = fmt.Sprintf("%.1f%%", float64(data.IMBlocked)/float64(data.IMTotal)*100)
		} else {
			data.IMBlockRate = "0.0%"
		}
		data.IMTopRules = e.queryTopRules(sinceRFC3339, 5)
	}

	if e.llmAuditor != nil {
		if ov, err := e.llmAuditor.OverviewWithFilter(sinceRFC3339); err == nil {
			if v, ok := ov["total_calls"].(int); ok {
				data.LLMTotalCalls = v
			}
			if v, ok := ov["total_tokens"].(int64); ok {
				data.LLMTotalTokens = v
			}
			if v, ok := ov["error_rate"].(float64); ok {
				data.LLMErrorRate = fmt.Sprintf("%.1f%%", v*100)
			} else {
				data.LLMErrorRate = "0.0%"
			}
			if v, ok := ov["estimated_cost_usd"].(float64); ok {
				data.LLMEstCostUSD = fmt.Sprintf("$%.2f", v)
			} else {
				data.LLMEstCostUSD = "$0.00"
			}
		}
		data.LLMTopTools = e.queryTopTools(sinceRFC3339, 5)
	}

	if e.owaspEngine != nil {
		items := e.owaspEngine.CalculateWithFilter(sinceRFC3339)
		for _, item := range items {
			if item.Count > 0 {
				data.OWASPItems = append(data.OWASPItems, reportOWASP{
					ID: item.ID, Name: item.Name, NameZh: item.NameZh,
					Count: item.Count, RiskLevel: item.RiskLevel,
					RiskColor: riskColor(item.RiskLevel),
				})
			}
		}
	}

	if e.userProfileEng != nil {
		if users, err := e.userProfileEng.GetTopRiskUsers(5); err == nil {
			for _, u := range users {
				if u.RiskScore > 0 {
					data.TopRiskUsers = append(data.TopRiskUsers, reportUser{
						UserID: u.UserID, RiskScore: u.RiskScore,
						RiskLevel: u.RiskLevel, RiskColor: userRiskColor(u.RiskLevel),
						BlockRate: fmt.Sprintf("%.1f%%", u.BlockRate*100),
						Requests:  u.TotalRequests,
					})
				}
			}
		}
	}

	if e.anomalyDet != nil {
		alerts := e.anomalyDet.GetAlerts(10)
		data.AnomalyCount = len(alerts)
		for _, a := range alerts {
			data.AnomalyAlerts = append(data.AnomalyAlerts, reportAnomaly{
				MetricName: a.MetricName, DisplayName: MetricDisplayName(a.MetricName),
				Expected: a.Expected, Actual: a.Actual, Deviation: a.Deviation,
				Severity: a.Severity, SevColor: sevColor(a.Severity),
				Timestamp: a.Timestamp.Format("01-02 15:04"),
			})
		}
	}

	data.Recommendations = e.generateRecommendations(data)
	return data
}

func (e *ReportEngine) queryTopRules(sinceRFC3339 string, limit int) []reportRuleHit {
	rows, err := e.db.Query(`SELECT reason, COUNT(*) as cnt FROM audit_log WHERE action='block' AND timestamp >= ? AND reason != '' GROUP BY reason ORDER BY cnt DESC LIMIT ?`, sinceRFC3339, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var rules []reportRuleHit
	for rows.Next() {
		var reason string
		var cnt int64
		if rows.Scan(&reason, &cnt) == nil {
			name := reason
			if len(name) > 50 {
				name = name[:50] + "..."
			}
			rules = append(rules, reportRuleHit{Name: name, Hits: cnt})
		}
	}
	return rules
}

func (e *ReportEngine) queryTopTools(sinceRFC3339 string, limit int) []reportToolStat {
	rows, err := e.db.Query(`SELECT tool_name, COUNT(*) as cnt FROM llm_tool_calls WHERE timestamp >= ? GROUP BY tool_name ORDER BY cnt DESC LIMIT ?`, sinceRFC3339, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var tools []reportToolStat
	for rows.Next() {
		var name string
		var cnt int
		if rows.Scan(&name, &cnt) == nil {
			tools = append(tools, reportToolStat{Name: name, Count: cnt})
		}
	}
	return tools
}

func (e *ReportEngine) generateRecommendations(data ReportData) []string {
	var recs []string
	if data.IMTotal > 0 {
		blockRate := float64(data.IMBlocked) / float64(data.IMTotal) * 100
		if blockRate > 20 {
			recs = append(recs, fmt.Sprintf("建议审查入站规则，拦截率偏高 (%.1f%%)", blockRate))
		}
	}
	for _, d := range data.Deductions {
		if strings.Contains(d.Name, "Canary") {
			recs = append(recs, fmt.Sprintf("检测到 Canary Token 泄露（%s），建议轮换 Canary Token", d.Detail))
		}
	}
	critCount := 0
	for _, u := range data.TopRiskUsers {
		if u.RiskLevel == "critical" || u.RiskLevel == "high" {
			critCount++
		}
	}
	if critCount > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个高风险用户，建议重点关注其操作行为", critCount))
	}
	if data.AnomalyCount > 0 {
		recs = append(recs, fmt.Sprintf("检测到 %d 个异常行为偏离基线，建议排查原因", data.AnomalyCount))
	}
	if data.LLMTotalCalls > 0 && data.LLMErrorRate != "0.0%" {
		recs = append(recs, "LLM 调用存在异常，建议检查模型服务健康状况")
	}
	if data.HealthScore < 50 {
		recs = append(recs, "安全健康分偏低，建议立即审查安全策略")
	}
	if len(recs) == 0 {
		recs = append(recs, "当前安全状况良好，请继续保持")
	}
	return recs
}

// ============================================================
// 查询 API
// ============================================================

func (e *ReportEngine) ListReports(reportType string, limit int) ([]ReportMeta, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := "SELECT id, type, title, time_range, from_time, to_time, created_at, file_path, file_size, status FROM reports"
	var args []interface{}
	if reportType != "" {
		query += " WHERE type=?"
		args = append(args, reportType)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []ReportMeta
	for rows.Next() {
		var m ReportMeta
		if rows.Scan(&m.ID, &m.Type, &m.Title, &m.TimeRange, &m.FromTime, &m.ToTime, &m.CreatedAt, &m.FilePath, &m.FileSize, &m.Status) == nil {
			reports = append(reports, m)
		}
	}
	return reports, nil
}

func (e *ReportEngine) GetReport(id string) (*ReportMeta, error) {
	var m ReportMeta
	err := e.db.QueryRow("SELECT id, type, title, time_range, from_time, to_time, created_at, file_path, file_size, status FROM reports WHERE id=?", id).
		Scan(&m.ID, &m.Type, &m.Title, &m.TimeRange, &m.FromTime, &m.ToTime, &m.CreatedAt, &m.FilePath, &m.FileSize, &m.Status)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *ReportEngine) DeleteReport(id string) error {
	var filePath string
	e.db.QueryRow("SELECT file_path FROM reports WHERE id=?", id).Scan(&filePath)
	if filePath != "" {
		os.Remove(filePath)
	}
	_, err := e.db.Exec("DELETE FROM reports WHERE id=?", id)
	return err
}

// ============================================================
// 颜色辅助
// ============================================================

func healthScoreColor(level string) string {
	m := map[string]string{"excellent": "#10B981", "good": "#3B82F6", "warning": "#F59E0B", "danger": "#EF4444", "critical": "#DC2626"}
	if c, ok := m[level]; ok {
		return c
	}
	return "#6B7280"
}

func riskColor(level string) string {
	m := map[string]string{"high": "#EF4444", "medium": "#F59E0B", "low": "#10B981"}
	if c, ok := m[level]; ok {
		return c
	}
	return "#6B7280"
}

func userRiskColor(level string) string {
	m := map[string]string{"critical": "#DC2626", "high": "#EF4444", "medium": "#F59E0B", "low": "#10B981"}
	if c, ok := m[level]; ok {
		return c
	}
	return "#10B981"
}

func sevColor(severity string) string {
	m := map[string]string{"critical": "#DC2626", "warning": "#F59E0B"}
	if c, ok := m[severity]; ok {
		return c
	}
	return "#6B7280"
}

// ============================================================
// HTML 模板渲染
// ============================================================

func renderReportHTML(data ReportData) (string, error) {
	funcMap := template.FuncMap{
		"safe":  func(s string) template.HTML { return template.HTML(s) },
		"inc":   func(i int) int { return i + 1 },
		"fmtf1": func(f float64) string { return fmt.Sprintf("%.1f", f) },
	}
	tmpl, err := template.New("report").Funcs(funcMap).Parse(getReportHTMLTpl())
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func getReportHTMLTpl() string {
	return "<!DOCTYPE html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"UTF-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n<title>{{.Meta.Title}}</title>\n" +
		"<style>\n*{margin:0;padding:0;box-sizing:border-box}\nbody{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;max-width:720px;margin:0 auto;padding:24px;color:#1a1a2e;background:#fff;line-height:1.6}\n" +
		".hdr{text-align:center;margin-bottom:32px;padding-bottom:24px;border-bottom:2px solid #6366F1}\n.hdr h1{font-size:24px;color:#1a1a2e;margin-bottom:4px}\n.hdr .sub{color:#6b7280;font-size:14px}\n" +
		".sbox{text-align:center;background:linear-gradient(135deg,#f8f9fa 0%,#e9ecef 100%);border-radius:16px;padding:28px;margin-bottom:28px;border:1px solid #e5e7eb}\n.snum{font-size:64px;font-weight:800;line-height:1}\n.slbl{font-size:14px;color:#6b7280;margin-top:4px}\n" +
		".krow{display:flex;gap:12px;margin-bottom:28px;flex-wrap:wrap}\n.kcd{flex:1;min-width:140px;background:#f8f9fa;border-radius:10px;padding:16px;text-align:center;border:1px solid #e5e7eb}\n.kv{font-size:28px;font-weight:700;line-height:1.2}\n.kn{font-size:12px;color:#6b7280;margin-top:4px}\n" +
		"h2{font-size:16px;font-weight:700;color:#1a1a2e;margin:24px 0 12px;padding-bottom:8px;border-bottom:2px solid #6366F1}\n" +
		"table{width:100%;border-collapse:collapse;margin-bottom:20px;font-size:13px}\nth{text-align:left;padding:8px 12px;background:#f3f4f6;color:#6b7280;font-weight:600;font-size:11px;text-transform:uppercase;letter-spacing:.05em;border-bottom:2px solid #e5e7eb}\ntd{padding:8px 12px;border-bottom:1px solid #f3f4f6}\ntr:last-child td{border-bottom:none}\n" +
		".bdg{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:11px;font-weight:700;color:#fff}\n.rl{list-style:none;padding:0}\n.rl li{padding:8px 12px;background:#FFFBEB;border-left:3px solid #F59E0B;margin-bottom:8px;border-radius:0 8px 8px 0;font-size:13px}\n" +
		".ft{text-align:center;margin-top:36px;padding-top:20px;border-top:1px solid #e5e7eb;color:#9ca3af;font-size:12px}\n.em{color:#9ca3af;font-size:13px;text-align:center;padding:16px 0}\n" +
		"@media(max-width:600px){.krow{flex-direction:column}.kcd{min-width:auto}.snum{font-size:48px}body{padding:16px}}\n</style>\n</head>\n<body>\n" +
		"<div class=\"hdr\">\n  <h1>&#x1F99E; {{.Meta.Title}}</h1>\n  <div class=\"sub\">{{.Generated}} &middot; 最近 {{.Meta.TimeRange}}</div>\n</div>\n" +
		"<div class=\"sbox\">\n  <div class=\"snum\" style=\"color:{{.HealthColor}}\">{{.HealthScore}}</div>\n  <div class=\"slbl\">安全健康分 / 100 &middot; 等级：{{.HealthLevelLabel}}</div>\n</div>\n" +
		"<div class=\"krow\">\n  <div class=\"kcd\"><div class=\"kv\" style=\"color:#3B82F6\">{{.IMTotal}}</div><div class=\"kn\">IM 总请求</div></div>\n  <div class=\"kcd\"><div class=\"kv\" style=\"color:#EF4444\">{{.IMBlocked}}</div><div class=\"kn\">拦截数</div></div>\n  <div class=\"kcd\"><div class=\"kv\" style=\"color:#F59E0B\">{{.IMWarned}}</div><div class=\"kn\">告警数</div></div>\n  <div class=\"kcd\"><div class=\"kv\" style=\"color:#10B981\">{{.IMBlockRate}}</div><div class=\"kn\">拦截率</div></div>\n</div>\n" +
		"{{if .Deductions}}\n<h2>&#x1F4C9; 扣分明细</h2>\n<table>\n  <tr><th>扣分项</th><th style=\"text-align:right\">扣分</th><th>详情</th></tr>\n  {{range .Deductions}}<tr><td style=\"font-weight:600;white-space:nowrap\">{{.Name}}</td><td style=\"text-align:right;color:#EF4444;font-weight:700\">-{{.Points}}</td><td style=\"color:#6b7280;font-size:12px\">{{.Detail}}</td></tr>\n  {{end}}\n</table>\n{{end}}\n" +
		"<h2>&#x1F6E1;&#xFE0F; IM 安全域</h2>\n<table>\n  <tr><td>总请求</td><td style=\"text-align:right;font-weight:700\">{{.IMTotal}}</td></tr>\n  <tr><td>拦截数</td><td style=\"text-align:right;font-weight:700;color:#EF4444\">{{.IMBlocked}}</td></tr>\n  <tr><td>告警数</td><td style=\"text-align:right;font-weight:700;color:#F59E0B\">{{.IMWarned}}</td></tr>\n  <tr><td>拦截率</td><td style=\"text-align:right;font-weight:700\">{{.IMBlockRate}}</td></tr>\n</table>\n" +
		"{{if .IMTopRules}}\n<table>\n  <tr><th>#</th><th>拦截原因</th><th style=\"text-align:right\">次数</th></tr>\n  {{range $i, $r := .IMTopRules}}<tr><td style=\"width:30px;color:#6366F1;font-weight:700\">{{inc $i}}</td><td>{{$r.Name}}</td><td style=\"text-align:right;font-weight:700\">{{$r.Hits}}</td></tr>\n  {{end}}\n</table>\n{{end}}\n" +
		"{{if .LLMTotalCalls}}\n<h2>&#x1F916; LLM 安全域</h2>\n<table>\n  <tr><td>总调用</td><td style=\"text-align:right;font-weight:700\">{{.LLMTotalCalls}}</td></tr>\n  <tr><td>总 Token</td><td style=\"text-align:right;font-weight:700\">{{.LLMTotalTokens}}</td></tr>\n  <tr><td>异常率</td><td style=\"text-align:right;font-weight:700\">{{.LLMErrorRate}}</td></tr>\n  <tr><td>预估成本</td><td style=\"text-align:right;font-weight:700\">{{.LLMEstCostUSD}}</td></tr>\n</table>\n" +
		"{{if .LLMTopTools}}\n<table>\n  <tr><th>#</th><th>工具</th><th style=\"text-align:right\">调用数</th></tr>\n  {{range $i, $t := .LLMTopTools}}<tr><td style=\"width:30px;color:#6366F1;font-weight:700\">{{inc $i}}</td><td>{{$t.Name}}</td><td style=\"text-align:right;font-weight:700\">{{$t.Count}}</td></tr>\n  {{end}}\n</table>\n{{end}}\n" +
		"{{if .OWASPItems}}\n<table>\n  <tr><th>OWASP ID</th><th>威胁</th><th style=\"text-align:right\">检出数</th><th>等级</th></tr>\n  {{range .OWASPItems}}<tr><td style=\"font-weight:600\">{{.ID}}</td><td>{{.NameZh}}</td><td style=\"text-align:right;font-weight:700\">{{.Count}}</td><td><span class=\"bdg\" style=\"background:{{.RiskColor}}\">{{.RiskLevel}}</span></td></tr>\n  {{end}}\n</table>\n{{end}}\n{{end}}\n" +
		"{{if .TopRiskUsers}}\n<h2>&#x26A0;&#xFE0F; 高风险用户</h2>\n<table>\n  <tr><th>用户</th><th style=\"text-align:right\">风险分</th><th>等级</th><th style=\"text-align:right\">拦截率</th><th style=\"text-align:right\">请求数</th></tr>\n  {{range .TopRiskUsers}}<tr><td style=\"font-weight:600\">{{.UserID}}</td><td style=\"text-align:right;font-weight:700\">{{.RiskScore}}</td><td><span class=\"bdg\" style=\"background:{{.RiskColor}}\">{{.RiskLevel}}</span></td><td style=\"text-align:right\">{{.BlockRate}}</td><td style=\"text-align:right\">{{.Requests}}</td></tr>\n  {{end}}\n</table>\n{{end}}\n" +
		"{{if .AnomalyAlerts}}\n<h2>&#x1F4CA; 异常检测</h2>\n<table>\n  <tr><th>指标</th><th style=\"text-align:right\">期望</th><th style=\"text-align:right\">实际</th><th style=\"text-align:right\">偏离</th><th>等级</th><th>时间</th></tr>\n  {{range .AnomalyAlerts}}<tr><td>{{.DisplayName}}</td><td style=\"text-align:right\">{{fmtf1 .Expected}}</td><td style=\"text-align:right;font-weight:700\">{{fmtf1 .Actual}}</td><td style=\"text-align:right\">{{fmtf1 .Deviation}}&sigma;</td><td><span class=\"bdg\" style=\"background:{{.SevColor}}\">{{.Severity}}</span></td><td style=\"color:#6b7280;font-size:12px\">{{.Timestamp}}</td></tr>\n  {{end}}\n</table>\n{{end}}\n" +
		"{{if .Recommendations}}\n<h2>&#x1F4A1; 安全建议</h2>\n<ul class=\"rl\">\n  {{range .Recommendations}}<li>{{.}}</li>\n  {{end}}\n</ul>\n{{end}}\n" +
		"<div class=\"ft\">龙虾卫士 v{{.Version}} &middot; 自动生成 &middot; 勿回复</div>\n</body>\n</html>"
}