// user_profile_test.go — 用户风险画像引擎测试
package main

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupUserProfileTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_log (
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
		app_id TEXT DEFAULT '',
		trace_id TEXT DEFAULT ''
	)`)
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
		error_message TEXT,
		canary_leaked INTEGER DEFAULT 0,
		budget_exceeded INTEGER DEFAULT 0,
		budget_violations TEXT
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS llm_tool_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		llm_call_id INTEGER,
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		tool_input_preview TEXT,
		tool_result_preview TEXT,
		risk_level TEXT DEFAULT 'low',
		flagged INTEGER DEFAULT 0,
		flag_reason TEXT
	)`)
	return db
}

func TestUserProfile_EmptyDB(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	eng := NewUserProfileEngine(db)
	users, err := eng.GetTopRiskUsers(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Errorf("空库应返回0个用户，实际 %d", len(users))
	}
}

func TestUserProfile_RiskScoreCalculation(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 创建一个高风险用户：高拦截率 + 注入攻击
	for i := 0; i < 20; i++ {
		action := "pass"
		reason := ""
		if i < 12 { // 60% block rate > 50% → 30分
			action = "block"
			if i < 8 {
				reason = "SQL injection detected"
			} else {
				reason = "keyword match"
			}
		}
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-high', ?, ?, '')`, ts, action, reason)
	}

	eng := NewUserProfileEngine(db)
	profile, err := eng.GetUserProfile("user-high")
	if err != nil {
		t.Fatal(err)
	}

	if profile.TotalRequests != 20 {
		t.Errorf("总请求数应为20，实际 %d", profile.TotalRequests)
	}
	if profile.BlockedRequests != 12 {
		t.Errorf("拦截数应为12，实际 %d", profile.BlockedRequests)
	}
	if profile.BlockRate < 0.50 {
		t.Errorf("拦截率应>=0.50，实际 %.2f", profile.BlockRate)
	}
	// 注入尝试: reason中含 injection 的 block
	if profile.InjectionAttempts < 8 {
		t.Errorf("注入尝试应>=8，实际 %d", profile.InjectionAttempts)
	}
	// 拦截率>50% → 30分, 注入>5 → 15分 → 至少45分
	if profile.RiskScore < 45 {
		t.Errorf("高风险用户分数应>=45，实际 %d", profile.RiskScore)
	}
}

func TestUserProfile_RiskLevel(t *testing.T) {
	tests := []struct {
		score int
		level string
	}{
		{0, "low"},
		{25, "low"},
		{26, "medium"},
		{50, "medium"},
		{51, "high"},
		{75, "high"},
		{76, "critical"},
		{100, "critical"},
	}
	for _, tt := range tests {
		got := riskLevelFromScore(tt.score)
		if got != tt.level {
			t.Errorf("riskLevelFromScore(%d) = %s, want %s", tt.score, got, tt.level)
		}
	}
}

func TestUserProfile_Top10(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 创建15个用户，各有不同的拦截率
	for uid := 0; uid < 15; uid++ {
		for i := 0; i < 10; i++ {
			action := "pass"
			reason := ""
			if i < uid { // uid越大block越多
				action = "block"
				reason = "test rule"
			}
			ts := now.Add(-time.Duration(uid*10+i) * time.Minute).Format(time.RFC3339)
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', ?, ?, ?, '')`,
				ts, fmt.Sprintf("user-%02d", uid), action, reason)
		}
	}

	eng := NewUserProfileEngine(db)
	users, err := eng.GetTopRiskUsers(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) > 10 {
		t.Errorf("Top10 应最多返回10个，实际 %d", len(users))
	}
	// 验证降序排列
	for i := 1; i < len(users); i++ {
		if users[i].RiskScore > users[i-1].RiskScore {
			t.Errorf("Top10 应按风险分降序，#%d(%d) > #%d(%d)", i, users[i].RiskScore, i-1, users[i-1].RiskScore)
		}
	}
}

func TestUserProfile_Timeline(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 插入不同action的事件
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'pass', '', 'hello')`,
		now.Add(-1*time.Hour).Format(time.RFC3339))
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'block', 'injection detected', 'bad input')`,
		now.Add(-2*time.Hour).Format(time.RFC3339))
	db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'warn', 'suspicious', 'maybe bad')`,
		now.Add(-3*time.Hour).Format(time.RFC3339))

	eng := NewUserProfileEngine(db)
	events, err := eng.GetUserTimeline("user-1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Errorf("应有3条时间线事件，实际 %d", len(events))
	}

	// 验证事件类型
	hasBlocked := false
	hasRequest := false
	for _, evt := range events {
		if evt.EventType == "im_blocked" {
			hasBlocked = true
			if evt.RiskLevel != "critical" {
				// injection 关键词 → critical
				t.Errorf("injection 拦截应为 critical 风险，实际 %s", evt.RiskLevel)
			}
		}
		if evt.EventType == "im_request" {
			hasRequest = true
		}
	}
	if !hasBlocked {
		t.Error("应有 im_blocked 事件")
	}
	if !hasRequest {
		t.Error("应有 im_request 事件")
	}
}

func TestUserProfile_DemoUsers(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	senders := []string{"user-001", "user-002", "user-003", "user-004", "user-005", "user-006", "user-007", "user-008"}
	for _, sender := range senders {
		for i := 0; i < 10; i++ {
			ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			action := "pass"
			if i < 3 {
				action = "block"
			}
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', ?, ?, '', '')`, ts, sender, action)
		}
	}

	eng := NewUserProfileEngine(db)
	users, err := eng.GetTopRiskUsers(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 8 {
		t.Errorf("应有8个用户，实际 %d", len(users))
	}
}

func TestUserProfile_BlockRateWeight(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 60% block rate → 30分
	for i := 0; i < 10; i++ {
		action := "pass"
		if i < 6 {
			action = "block"
		}
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-block', ?, '', '')`, ts, action)
	}

	eng := NewUserProfileEngine(db)
	profile, err := eng.GetUserProfile("user-block")
	if err != nil {
		t.Fatal(err)
	}

	// 仅拦截率维度: block rate 60% > 50% → 30分
	if profile.RiskScore < 30 {
		t.Errorf("60%%拦截率应贡献至少30分，实际总分 %d", profile.RiskScore)
	}
}

func TestUserProfile_InjectionWeight(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 注入攻击15次 (>10 → 20分)，低拦截率
	for i := 0; i < 30; i++ {
		action := "pass"
		reason := ""
		if i < 15 {
			action = "block"
			reason = "SQL injection detected"
		}
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-inj', ?, ?, '')`, ts, action, reason)
	}

	eng := NewUserProfileEngine(db)
	profile, err := eng.GetUserProfile("user-inj")
	if err != nil {
		t.Fatal(err)
	}

	if profile.InjectionAttempts < 15 {
		t.Errorf("注入尝试应>=15，实际 %d", profile.InjectionAttempts)
	}
	// injection > 10 → 20分
	// block rate 50% → 30分
	if profile.RiskScore < 20 {
		t.Errorf("注入攻击应贡献至少20分，实际总分 %d", profile.RiskScore)
	}
}

func TestUserProfile_OffHoursWeight(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 创建一个主要在凌晨活跃的用户
	for i := 0; i < 10; i++ {
		// 全部在凌晨2点
		ts := time.Date(now.Year(), now.Month(), now.Day()-i, 2, 0, 0, 0, time.UTC).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-night', 'pass', '', '')`, ts)
	}

	eng := NewUserProfileEngine(db)
	profile, err := eng.GetUserProfile("user-night")
	if err != nil {
		t.Fatal(err)
	}

	if profile.OffHoursRate < 0.90 {
		t.Errorf("全部凌晨活动的 OffHoursRate 应接近1.0，实际 %.2f", profile.OffHoursRate)
	}
}

func TestUserProfile_ScoreRange(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 创建多种用户
	users := map[string]string{
		"user-good":   "pass",
		"user-bad":    "block",
	}
	for uid, action := range users {
		for i := 0; i < 10; i++ {
			ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			reason := ""
			if action == "block" {
				reason = "SQL injection"
			}
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', ?, ?, ?, '')`, ts, uid, action, reason)
		}
	}

	eng := NewUserProfileEngine(db)
	for uid := range users {
		profile, err := eng.GetUserProfile(uid)
		if err != nil {
			t.Fatal(err)
		}
		if profile.RiskScore < 0 || profile.RiskScore > 100 {
			t.Errorf("用户 %s 分数 %d 超出 [0,100] 范围", uid, profile.RiskScore)
		}
	}
}

func TestUserProfile_TrendDirection(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 近7天有很多攻击，但30天内较少 → rising
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-trend', 'block', 'injection', '')`, ts)
	}
	// 30天前一些正常请求
	for i := 0; i < 20; i++ {
		ts := now.AddDate(0, 0, -20).Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-trend', 'pass', '', '')`, ts)
	}

	eng := NewUserProfileEngine(db)
	profile, err := eng.GetUserProfile("user-trend")
	if err != nil {
		t.Fatal(err)
	}

	// 应为 rising, falling, 或 stable
	validTrends := map[string]bool{"rising": true, "falling": true, "stable": true}
	if !validTrends[profile.RiskTrend] {
		t.Errorf("趋势方向 %s 不是有效值", profile.RiskTrend)
	}
}

func TestUserProfile_RiskStats(t *testing.T) {
	db := setupUserProfileTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 创建几个用户
	for _, uid := range []string{"u1", "u2", "u3"} {
		for i := 0; i < 10; i++ {
			action := "pass"
			if uid == "u1" && i < 8 {
				action = "block"
			}
			ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', ?, ?, '', '')`, ts, uid, action)
		}
	}

	eng := NewUserProfileEngine(db)
	stats, err := eng.GetRiskStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalUsers != 3 {
		t.Errorf("总用户数应为3，实际 %d", stats.TotalUsers)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s    string
		subs []string
		want bool
	}{
		{"SQL Injection found", []string{"injection", "xss"}, true},
		{"normal text here", []string{"injection", "xss"}, false},
		{"XSS attack", []string{"injection", "xss"}, true},
		{"", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := containsAny(tt.s, tt.subs...)
		if got != tt.want {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.subs, got, tt.want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello world", 20, "hello world"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncateStr(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}
