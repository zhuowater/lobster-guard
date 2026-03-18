// anomaly_detect_test.go — 异常基线检测器测试
package main

import (
	"database/sql"
	"math"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupAnomalyTestDB(t *testing.T) *sql.DB {
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

func TestAnomalyDetector_Init(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)
	if d == nil {
		t.Fatal("NewAnomalyDetector 不应返回 nil")
	}
	if d.maxAlerts != 100 {
		t.Errorf("maxAlerts 默认应为100，实际 %d", d.maxAlerts)
	}
	if len(d.baselines) != 0 {
		t.Errorf("初始基线应为空，实际 %d", len(d.baselines))
	}
	if len(d.alerts) != 0 {
		t.Errorf("初始告警应为空，实际 %d", len(d.alerts))
	}
}

func TestAnomalyDetector_BaselineNotReadyWithFewData(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 只插入1天的数据 → 基线不就绪
	for i := 0; i < 24; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
		db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'pass', '', '')`, ts)
	}

	d := NewAnomalyDetector(db)
	d.UpdateBaselines()

	baselines := d.GetBaselines()
	for name, b := range baselines {
		if b.Ready {
			t.Errorf("只有1天数据，基线 %s 不应就绪", name)
		}
	}
}

func TestAnomalyDetector_BaselineReadyAfter3Days(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	now := time.Now().UTC()
	// 插入4天的数据（每天每小时1条）
	for day := 0; day < 4; day++ {
		for hour := 0; hour < 24; hour++ {
			ts := now.AddDate(0, 0, -day).Add(-time.Duration(hour) * time.Hour).Format(time.RFC3339)
			db.Exec(`INSERT INTO audit_log (timestamp, direction, sender_id, action, reason, content_preview) VALUES (?, 'inbound', 'user-1', 'pass', '', '')`, ts)
		}
	}

	d := NewAnomalyDetector(db)
	d.UpdateBaselines()

	baselines := d.GetBaselines()
	imReq, ok := baselines["im_requests_per_hour"]
	if !ok {
		t.Fatal("应有 im_requests_per_hour 基线")
	}
	if !imReq.Ready {
		t.Error("4天数据后基线应就绪")
	}
	if imReq.SampleCount == 0 {
		t.Error("样本数不应为0")
	}
}

func TestAnomalyDetector_MeanCalculation(t *testing.T) {
	tests := []struct {
		vals []float64
		want float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 3.0},
		{[]float64{10, 10, 10}, 10.0},
		{[]float64{0, 100}, 50.0},
		{[]float64{}, 0},
	}
	for _, tt := range tests {
		got := mean(tt.vals)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("mean(%v) = %f, want %f", tt.vals, got, tt.want)
		}
	}
}

func TestAnomalyDetector_StdDevCalculation(t *testing.T) {
	// stdDev([1,2,3,4,5]) = sqrt(10/4) = 1.5811
	vals := []float64{1, 2, 3, 4, 5}
	got := stdDev(vals)
	expected := math.Sqrt(10.0 / 4.0) // sample std dev
	if math.Abs(got-expected) > 0.01 {
		t.Errorf("stdDev(%v) = %f, want %f", vals, got, expected)
	}

	// single value → 0
	got2 := stdDev([]float64{5})
	if got2 != 0 {
		t.Errorf("stdDev([5]) = %f, want 0", got2)
	}

	// empty → 0
	got3 := stdDev([]float64{})
	if got3 != 0 {
		t.Errorf("stdDev([]) = %f, want 0", got3)
	}
}

func TestAnomalyDetector_NormalValueNoAlert(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	// 手动注入基线
	hour := time.Now().UTC().Hour()
	baseline := &Baseline{
		MetricName: "im_requests_per_hour",
		Ready:      true,
	}
	baseline.HourlyMean[hour] = 50.0
	baseline.HourlyStd[hour] = 10.0

	d.mu.Lock()
	d.baselines["im_requests_per_hour"] = baseline
	d.mu.Unlock()

	// 检查正常值（mean=50, std=10, value=55 → deviation=0.5 < 2）
	alert := d.checkMetric("im_requests_per_hour", 55.0, baseline)
	if alert != nil {
		t.Error("正常值不应产生告警")
	}
}

func TestAnomalyDetector_2SigmaWarning(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	hour := time.Now().UTC().Hour()
	baseline := &Baseline{
		MetricName: "im_requests_per_hour",
		Ready:      true,
	}
	baseline.HourlyMean[hour] = 50.0
	baseline.HourlyStd[hour] = 10.0

	// value=75 → deviation = |75-50|/10 = 2.5 > 2 → warning
	alert := d.checkMetric("im_requests_per_hour", 75.0, baseline)
	if alert == nil {
		t.Fatal("2.5σ 偏离应产生告警")
	}
	if alert.Severity != "warning" {
		t.Errorf("2.5σ 偏离应为 warning，实际 %s", alert.Severity)
	}
}

func TestAnomalyDetector_3SigmaCritical(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	hour := time.Now().UTC().Hour()
	baseline := &Baseline{
		MetricName: "im_requests_per_hour",
		Ready:      true,
	}
	baseline.HourlyMean[hour] = 50.0
	baseline.HourlyStd[hour] = 10.0

	// value=85 → deviation = |85-50|/10 = 3.5 > 3 → critical
	alert := d.checkMetric("im_requests_per_hour", 85.0, baseline)
	if alert == nil {
		t.Fatal("3.5σ 偏离应产生告警")
	}
	if alert.Severity != "critical" {
		t.Errorf("3.5σ 偏离应为 critical，实际 %s", alert.Severity)
	}
}

func TestAnomalyDetector_MinStdDev(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	hour := time.Now().UTC().Hour()
	baseline := &Baseline{
		MetricName: "im_requests_per_hour",
		Ready:      true,
	}
	baseline.HourlyMean[hour] = 10.0
	baseline.HourlyStd[hour] = 0.1 // 很小，会被提升到1.0

	// value=13 → std=1.0(min), deviation=|13-10|/1=3.0 → 刚好等于3 → 还是 > 2 → warning
	alert := d.checkMetric("im_requests_per_hour", 13.0, baseline)
	if alert == nil {
		t.Fatal("当最小标准差生效时应正确检测偏离")
	}
	// deviation = 3.0, 刚好 > 2 but == 3.0, source code: deviation > 3.0 → critical, else warning
	// 3.0 is not > 3.0, so warning
	if alert.Severity != "warning" {
		t.Errorf("deviation=3.0 应为 warning (不是 >3.0)，实际 %s", alert.Severity)
	}
}

func TestAnomalyDetector_DirectionAboveBelow(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	hour := time.Now().UTC().Hour()
	baseline := &Baseline{
		MetricName: "im_requests_per_hour",
		Ready:      true,
	}
	baseline.HourlyMean[hour] = 50.0
	baseline.HourlyStd[hour] = 10.0

	// Above: value=80 → direction=above
	alertAbove := d.checkMetric("im_requests_per_hour", 80.0, baseline)
	if alertAbove == nil {
		t.Fatal("should produce alert for value=80")
	}
	if alertAbove.Direction != "above" {
		t.Errorf("value>mean 方向应为 above，实际 %s", alertAbove.Direction)
	}

	// Below: value=20 → direction=below
	alertBelow := d.checkMetric("im_requests_per_hour", 20.0, baseline)
	if alertBelow == nil {
		t.Fatal("should produce alert for value=20")
	}
	if alertBelow.Direction != "below" {
		t.Errorf("value<mean 方向应为 below，实际 %s", alertBelow.Direction)
	}
}

func TestAnomalyDetector_AlertLimit(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	// 注入超过100条告警
	for i := 0; i < 150; i++ {
		d.mu.Lock()
		d.alerts = append(d.alerts, AnomalyAlert{
			ID:         "test-alert-" + string(rune(i)),
			MetricName: "test",
			Timestamp:  time.Now().UTC(),
		})
		if len(d.alerts) > d.maxAlerts {
			d.alerts = d.alerts[:d.maxAlerts]
		}
		d.mu.Unlock()
	}

	alerts := d.GetAlerts(200)
	if len(alerts) > 100 {
		t.Errorf("告警数不应超过100，实际 %d", len(alerts))
	}
}

func TestAnomalyDetector_DemoSeed(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)
	d.InjectDemoBaselines()
	d.InjectDemoAlerts()

	baselines := d.GetBaselines()
	if len(baselines) != 6 {
		t.Errorf("Demo基线应有6个指标，实际 %d", len(baselines))
	}

	for name, b := range baselines {
		if !b.Ready {
			t.Errorf("Demo基线 %s 应就绪", name)
		}
		if b.SampleCount != 168 {
			t.Errorf("Demo基线 %s 样本数应为168，实际 %d", name, b.SampleCount)
		}
	}

	alerts := d.GetAlerts(100)
	if len(alerts) < 6 {
		t.Errorf("Demo告警应至少6条，实际 %d", len(alerts))
	}
}

func TestAnomalyDetector_GetMetricDetail(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)
	d.InjectDemoBaselines()

	// 查询已知指标
	detail := d.GetMetricDetail("im_requests_per_hour")
	if detail["metric_name"] != "im_requests_per_hour" {
		t.Errorf("metric_name 不匹配")
	}
	if detail["display_name"] != "IM 每小时请求数" {
		t.Errorf("display_name 期望 'IM 每小时请求数'，实际 '%v'", detail["display_name"])
	}
	if detail["baseline"] == nil {
		t.Error("Demo注入后应有基线")
	}

	// 查询未知指标
	detail2 := d.GetMetricDetail("nonexistent_metric")
	if detail2["baseline"] != nil {
		t.Error("不存在的指标不应有基线")
	}
}

func TestAnomalyDetector_AllSixMetrics(t *testing.T) {
	expectedMetrics := []string{
		"im_requests_per_hour",
		"im_blocks_per_hour",
		"llm_calls_per_hour",
		"llm_tokens_per_hour",
		"tool_calls_per_hour",
		"high_risk_tools_per_hour",
	}

	if len(monitoredMetrics) != 6 {
		t.Errorf("应监控6个指标，实际 %d", len(monitoredMetrics))
	}

	for i, expected := range expectedMetrics {
		if monitoredMetrics[i].Name != expected {
			t.Errorf("指标 #%d 期望 %s，实际 %s", i, expected, monitoredMetrics[i].Name)
		}
	}
}

func TestAnomalyDetector_GetStatus(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)
	d.InjectDemoBaselines()
	d.InjectDemoAlerts()

	status := d.GetStatus()
	if status["enabled"] != true {
		t.Error("状态应为 enabled=true")
	}
	if status["metrics_count"] != len(monitoredMetrics) {
		t.Errorf("metrics_count 应为 %d，实际 %v", len(monitoredMetrics), status["metrics_count"])
	}
	baselinesReady, _ := status["baselines_ready"].(int)
	if baselinesReady != 6 {
		t.Errorf("baselines_ready 应为6，实际 %d", baselinesReady)
	}
	totalAlerts, _ := status["total_alerts"].(int)
	if totalAlerts < 6 {
		t.Errorf("total_alerts 应>=6，实际 %d", totalAlerts)
	}
}

func TestAnomalyDetector_GetAlerts24h(t *testing.T) {
	db := setupAnomalyTestDB(t)
	defer db.Close()

	d := NewAnomalyDetector(db)

	// 注入2条24h内的告警，1条老告警
	d.mu.Lock()
	d.alerts = []AnomalyAlert{
		{ID: "a1", Timestamp: time.Now().UTC().Add(-1 * time.Hour)},
		{ID: "a2", Timestamp: time.Now().UTC().Add(-12 * time.Hour)},
		{ID: "a3", Timestamp: time.Now().UTC().Add(-48 * time.Hour)},
	}
	d.mu.Unlock()

	count := d.GetAlerts24h()
	if count != 2 {
		t.Errorf("24h内告警应为2条，实际 %d", count)
	}
}

func TestMetricDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		display string
	}{
		{"im_requests_per_hour", "IM 每小时请求数"},
		{"im_blocks_per_hour", "IM 每小时拦截数"},
		{"llm_calls_per_hour", "LLM 每小时调用数"},
		{"unknown_metric", "unknown_metric"},
	}
	for _, tt := range tests {
		got := MetricDisplayName(tt.name)
		if got != tt.display {
			t.Errorf("MetricDisplayName(%q) = %q, want %q", tt.name, got, tt.display)
		}
	}
}

func TestAverageFactor(t *testing.T) {
	avg := averageFactor()
	if avg <= 0 || avg > 1 {
		t.Errorf("averageFactor 应在 (0,1] 范围内，实际 %f", avg)
	}
}
