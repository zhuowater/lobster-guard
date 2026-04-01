package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type CanaryRotationRecord struct {
	RotatedAt    time.Time `json:"rotated_at"`
	OldTokenHash string    `json:"old_token_hash"`
	NewTokenHash string    `json:"new_token_hash"`
}

type CanaryRotator struct {
	mu             sync.RWMutex
	api            *ManagementAPI
	interval       time.Duration
	enabled        bool
	createdAt      time.Time
	lastRotated    time.Time
	history        []CanaryRotationRecord
	graceTokens    map[string]time.Time // 旧 token → 过期时间（轮换后 grace period）
	ticker         *time.Ticker
	stopCh         chan struct{}
}

func NewCanaryRotator(api *ManagementAPI) *CanaryRotator {
	hours := api.cfg.CanaryRotation.IntervalHours
	if hours <= 0 {
		hours = 24
	}
	r := &CanaryRotator{api: api, interval: time.Duration(hours) * time.Hour, enabled: api.cfg.CanaryRotation.Enabled, stopCh: make(chan struct{}), createdAt: time.Now().UTC(), graceTokens: make(map[string]time.Time)}
	if api.cfg.LLMProxy.Security.CanaryToken.Token != "" {
		r.lastRotated = time.Now().UTC()
	}
	if r.enabled {
		r.start()
	}
	return r
}
func (r *CanaryRotator) start() {
	if r.ticker != nil {
		return
	}
	r.ticker = time.NewTicker(r.interval)
	go func() {
		for {
			select {
			case <-r.ticker.C:
				_, _ = r.Rotate()
			case <-r.stopCh:
				return
			}
		}
	}()
}
func (r *CanaryRotator) Stop() {
	if r.ticker != nil {
		r.ticker.Stop()
	}
	close(r.stopCh)
}
func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}
func genToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func (api *ManagementAPI) persistRawSection(section string, value map[string]interface{}) error {
	api.cfgMu.Lock()
	defer api.cfgMu.Unlock()
	data, err := os.ReadFile(api.cfgPath)
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw[section] = value
	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(api.cfgPath, out, 0644)
}
func (r *CanaryRotator) Rotate() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	newToken, err := genToken()
	if err != nil {
		return "", err
	}
	oldToken := r.api.cfg.LLMProxy.Security.CanaryToken.Token
	// Grace period: 旧 token 保留 5 分钟，避免轮换瞬间漏检泄露
	if oldToken != "" {
		r.graceTokens[oldToken] = time.Now().UTC().Add(5 * time.Minute)
	}
	// 清理过期的 grace tokens
	for tok, exp := range r.graceTokens {
		if time.Now().UTC().After(exp) {
			delete(r.graceTokens, tok)
		}
	}
	r.api.cfg.LLMProxy.Security.CanaryToken.Token = newToken
	r.api.cfg.LLMProxy.Security.CanaryToken.Enabled = true
	now := time.Now().UTC()
	if r.createdAt.IsZero() {
		r.createdAt = now
	}
	r.lastRotated = now
	r.history = append([]CanaryRotationRecord{{RotatedAt: now, OldTokenHash: hashToken(oldToken), NewTokenHash: hashToken(newToken)}}, r.history...)
	if len(r.history) > 20 {
		r.history = r.history[:20]
	}
	if err := r.api.saveLLMConfig(); err != nil {
		return "", err
	}
	return newToken, nil
}
func (r *CanaryRotator) Status() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	next := ""
	if r.enabled && !r.lastRotated.IsZero() {
		next = r.lastRotated.Add(r.interval).Format(time.RFC3339)
	}
	return map[string]interface{}{
		"enabled":            r.enabled,
		"current_token_hash": hashToken(r.api.cfg.LLMProxy.Security.CanaryToken.Token),
		"token_created_at":   r.createdAt.Format(time.RFC3339),
		"last_rotated_at": func() string {
			if r.lastRotated.IsZero() {
				return ""
			}
			return r.lastRotated.Format(time.RFC3339)
		}(),
		"next_rotation_at": next,
		"interval_hours":   int(r.interval.Hours()),
	}
}
// IsCanaryLeaked 检查文本中是否包含当前或 grace period 内的旧 canary token
func (r *CanaryRotator) IsCanaryLeaked(text string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	currentToken := r.api.cfg.LLMProxy.Security.CanaryToken.Token
	if currentToken != "" && strings.Contains(text, currentToken) {
		return true
	}
	now := time.Now().UTC()
	for tok, exp := range r.graceTokens {
		if now.Before(exp) && strings.Contains(text, tok) {
			return true
		}
	}
	return false
}

func (r *CanaryRotator) GetRotationHistory(limit int) []CanaryRotationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	out := make([]CanaryRotationRecord, limit)
	copy(out, r.history[:limit])
	return out
}

func (api *ManagementAPI) handleCanaryRotationStatus(w http.ResponseWriter, r *http.Request) {
	if api.canaryRotator == nil {
		jsonResponse(w, 200, map[string]interface{}{"enabled": false})
		return
	}
	jsonResponse(w, 200, api.canaryRotator.Status())
}
func (api *ManagementAPI) handleCanaryRotateNow(w http.ResponseWriter, r *http.Request) {
	if api.canaryRotator == nil {
		jsonResponse(w, 500, map[string]string{"error": "canary rotator unavailable"})
		return
	}
	token, err := api.canaryRotator.Rotate()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "token_hash": hashToken(token)})
}
func (api *ManagementAPI) handleCanaryHistory(w http.ResponseWriter, r *http.Request) {
	if api.canaryRotator == nil {
		jsonResponse(w, 200, map[string]interface{}{"history": []CanaryRotationRecord{}})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"history": api.canaryRotator.GetRotationHistory(20)})
}

type ReportRun struct {
	ID          string    `json:"id"`
	GeneratedAt time.Time `json:"generated_at"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
}

type ReportScheduler struct {
	mu         sync.RWMutex
	api        *ManagementAPI
	enabled    bool
	cronExpr   string
	webhookURL string
	nextRun    time.Time
	ticker     *time.Ticker
	stopCh     chan struct{}
}

func NewReportScheduler(api *ManagementAPI) *ReportScheduler {
	s := &ReportScheduler{api: api, enabled: api.cfg.ReportSchedule.Enabled, cronExpr: api.cfg.ReportSchedule.Cron, webhookURL: api.cfg.ReportSchedule.WebhookURL, stopCh: make(chan struct{})}
	api.initReportRunsTable()
	s.scheduleNext()
	if s.enabled {
		s.start()
	}
	return s
}
func (api *ManagementAPI) initReportRunsTable() {
	if api.logger != nil {
		_, _ = api.logger.DB().Exec(`CREATE TABLE IF NOT EXISTS report_runs (id TEXT PRIMARY KEY, generated_at TEXT NOT NULL, type TEXT NOT NULL, status TEXT NOT NULL, error TEXT DEFAULT '')`)
	}
}
func (s *ReportScheduler) parseInterval() time.Duration {
	// simple cron support: minute hour * * weekday => next matching time, tick every minute
	return time.Minute
}
func (s *ReportScheduler) scheduleNext() { s.nextRun = time.Now().UTC().Add(time.Minute) }
func (s *ReportScheduler) start() {
	s.ticker = time.NewTicker(s.parseInterval())
	go func() {
		for {
			select {
			case <-s.ticker.C:
				if s.enabled {
					_, _ = s.GenerateAndPush("scheduled")
					s.scheduleNext()
				}
			case <-s.stopCh:
				return
			}
		}
	}()
}
func (s *ReportScheduler) recordRun(run ReportRun) {
	if s.api.logger == nil {
		return
	}
	_, _ = s.api.logger.DB().Exec(`INSERT INTO report_runs (id, generated_at, type, status, error) VALUES (?,?,?,?,?)`, run.ID, run.GeneratedAt.Format(time.RFC3339), run.Type, run.Status, run.Error)
}
func (s *ReportScheduler) GenerateAndPush(runType string) (*ReportRun, error) {
	if s.api.reportEngine == nil {
		return nil, fmt.Errorf("report engine not initialized")
	}
	meta, err := s.api.reportEngine.Generate(ReportWeekly)
	run := &ReportRun{ID: fmt.Sprintf("run-%d", time.Now().UnixNano()), GeneratedAt: time.Now().UTC(), Type: runType, Status: "success"}
	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		s.recordRun(*run)
		return run, err
	}
	if s.webhookURL != "" {
		data, readErr := os.ReadFile(meta.FilePath)
		if readErr != nil {
			run.Status = "failed"
			run.Error = readErr.Error()
			s.recordRun(*run)
			return run, readErr
		}
		resp, postErr := http.Post(s.webhookURL, "text/html; charset=utf-8", bytes.NewReader(data))
		if postErr != nil {
			run.Status = "failed"
			run.Error = postErr.Error()
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 300 {
				run.Status = "failed"
				run.Error = resp.Status
			}
		}
	}
	s.recordRun(*run)
	return run, nil
}
func (s *ReportScheduler) ListRuns(limit int) []ReportRun {
	if s.api.logger == nil {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.api.logger.DB().Query(`SELECT id, generated_at, type, status, error FROM report_runs ORDER BY generated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []ReportRun{}
	for rows.Next() {
		var rr ReportRun
		var ts string
		_ = rows.Scan(&rr.ID, &ts, &rr.Type, &rr.Status, &rr.Error)
		rr.GeneratedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, rr)
	}
	return out
}
func (api *ManagementAPI) handleReportScheduleGet(w http.ResponseWriter, r *http.Request) {
	if api.reportScheduler == nil {
		jsonResponse(w, 200, map[string]interface{}{"enabled": false, "cron": "", "webhook_url": ""})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"enabled": api.reportScheduler.enabled, "cron": api.reportScheduler.cronExpr, "webhook_url": api.reportScheduler.webhookURL, "next_run": api.reportScheduler.nextRun.Format(time.RFC3339)})
}
func (api *ManagementAPI) handleReportScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled    bool   `json:"enabled"`
		Cron       string `json:"cron"`
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	// 校验 cron 表达式（简易：5 或 6 段空格分隔）
	if req.Cron != "" {
		fields := strings.Fields(req.Cron)
		if len(fields) < 5 || len(fields) > 6 {
			jsonResponse(w, 400, map[string]string{"error": fmt.Sprintf("invalid cron expression: expected 5-6 fields, got %d", len(fields))})
			return
		}
	}
	api.cfg.ReportSchedule.Enabled = req.Enabled
	api.cfg.ReportSchedule.Cron = req.Cron
	api.cfg.ReportSchedule.WebhookURL = req.WebhookURL
	if api.reportScheduler != nil {
		api.reportScheduler.enabled = req.Enabled
		api.reportScheduler.cronExpr = req.Cron
		api.reportScheduler.webhookURL = req.WebhookURL
		api.reportScheduler.scheduleNext()
	}
	_ = api.persistRawSection("report_schedule", map[string]interface{}{"enabled": req.Enabled, "cron": req.Cron, "webhook_url": req.WebhookURL})
	jsonResponse(w, 200, map[string]interface{}{"ok": true})
}
func (api *ManagementAPI) handleReportGenerateNow(w http.ResponseWriter, r *http.Request) {
	if api.reportScheduler == nil {
		jsonResponse(w, 500, map[string]string{"error": "report scheduler unavailable"})
		return
	}
	run, err := api.reportScheduler.GenerateAndPush("manual")
	if err != nil {
		jsonResponse(w, 500, map[string]interface{}{"error": err.Error(), "run": run})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "run": run})
}
func (api *ManagementAPI) handleReportRuns(w http.ResponseWriter, r *http.Request) {
	if api.reportScheduler == nil {
		jsonResponse(w, 200, map[string]interface{}{"runs": []ReportRun{}})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"runs": api.reportScheduler.ListRuns(50)})
}

func (api *ManagementAPI) handleEngineToggle(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/engines/")
	name = strings.TrimSuffix(name, "/toggle")
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Enabled == nil {
		jsonResponse(w, 400, map[string]string{"error": "missing required field: enabled"})
		return
	}
	mapping := map[string]struct {
		ptr *bool
		key string
	}{
		"plan-compiler":      {&api.cfg.PlanCompiler.Enabled, "plan_compiler"},
		"capability-engine":  {&api.cfg.Capability.Enabled, "capability"},
		"deviation-detector": {&api.cfg.Deviation.Enabled, "deviation"},
	}
	m, ok := mapping[name]
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "unknown engine"})
		return
	}
	*m.ptr = *req.Enabled
	_ = api.persistRawSection(m.key, map[string]interface{}{"enabled": *req.Enabled})
	jsonResponse(w, 200, map[string]interface{}{"ok": true, "name": name, "enabled": *req.Enabled})
}

// handleEngineToggleGet GET /api/v1/engines/:name/toggle — 获取引擎当前状态
func (api *ManagementAPI) handleEngineToggleGet(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/engines/")
	name = strings.TrimSuffix(name, "/toggle")
	mapping := map[string]*bool{
		"plan-compiler":      &api.cfg.PlanCompiler.Enabled,
		"capability-engine":  &api.cfg.Capability.Enabled,
		"deviation-detector": &api.cfg.Deviation.Enabled,
	}
	ptr, ok := mapping[name]
	if !ok {
		jsonResponse(w, 404, map[string]string{"error": "unknown engine"})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"name": name, "enabled": *ptr})
}
