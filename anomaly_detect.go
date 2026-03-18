// anomaly_detect.go — 异常基线检测器（纯统计方法、零依赖）
// lobster-guard v11.2 — 连续运行 7 天后自动建立正常行为基线，偏离 >2σ 自动告警
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ============================================================
// 核心类型
// ============================================================

// AnomalyDetector 异常基线检测器
type AnomalyDetector struct {
	db        *sql.DB
	mu        sync.RWMutex
	baselines map[string]*Baseline // key: metric_name
	alerts    []AnomalyAlert
	maxAlerts int
}

// Baseline 一个指标的基线
type Baseline struct {
	MetricName string     `json:"metric_name"`
	WindowDays int        `json:"window_days"`
	HourlyMean [24]float64 `json:"hourly_mean"`
	HourlyStd  [24]float64 `json:"hourly_std"`
	DailyMean  float64    `json:"daily_mean"`
	DailyStd   float64    `json:"daily_std"`
	SampleCount int       `json:"sample_count"`
	LastUpdate time.Time  `json:"last_update"`
	Ready      bool       `json:"ready"`
}

// AnomalyAlert 异常告警
type AnomalyAlert struct {
	ID         string    `json:"id"`
	MetricName string    `json:"metric_name"`
	Timestamp  time.Time `json:"timestamp"`
	Expected   float64   `json:"expected"`
	Actual     float64   `json:"actual"`
	StdDev     float64   `json:"std_dev"`
	Deviation  float64   `json:"deviation"`
	Direction  string    `json:"direction"`
	Severity   string    `json:"severity"`
}

// 监控的指标列表及其显示名称
var monitoredMetrics = []struct {
	Name    string
	Display string
}{
	{"im_requests_per_hour", "IM 每小时请求数"},
	{"im_blocks_per_hour", "IM 每小时拦截数"},
	{"llm_calls_per_hour", "LLM 每小时调用数"},
	{"llm_tokens_per_hour", "LLM 每小时 Token 消耗"},
	{"tool_calls_per_hour", "每小时工具调用数"},
	{"high_risk_tools_per_hour", "每小时高危工具调用数"},
}

// MetricDisplayName 获取指标显示名
func MetricDisplayName(metricName string) string {
	for _, m := range monitoredMetrics {
		if m.Name == metricName {
			return m.Display
		}
	}
	return metricName
}

// ============================================================
// 构造 + 生命周期
// ============================================================

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector(db *sql.DB) *AnomalyDetector {
	d := &AnomalyDetector{
		db:        db,
		baselines: make(map[string]*Baseline),
		alerts:    []AnomalyAlert{},
		maxAlerts: 100,
	}
	return d
}

// StartBackground 启动后台基线更新和异常检查
func (d *AnomalyDetector) StartBackground() {
	// 启动时立即计算一次基线
	d.UpdateBaselines()

	go func() {
		baselineTicker := time.NewTicker(1 * time.Hour)
		checkTicker := time.NewTicker(5 * time.Minute)
		defer baselineTicker.Stop()
		defer checkTicker.Stop()

		for {
			select {
			case <-baselineTicker.C:
				d.UpdateBaselines()
			case <-checkTicker.C:
				newAlerts := d.CheckNow()
				if len(newAlerts) > 0 {
					d.mu.Lock()
					d.alerts = append(newAlerts, d.alerts...)
					if len(d.alerts) > d.maxAlerts {
						d.alerts = d.alerts[:d.maxAlerts]
					}
					d.mu.Unlock()
					for _, a := range newAlerts {
						log.Printf("[异常检测] ⚠️ %s 异常: 期望=%.1f 实际=%.1f 偏离=%.1fσ (%s/%s)",
							a.MetricName, a.Expected, a.Actual, a.Deviation, a.Direction, a.Severity)
					}
				}
			}
		}
	}()
}

// ============================================================
// 基线计算
// ============================================================

// UpdateBaselines 更新所有指标的基线
func (d *AnomalyDetector) UpdateBaselines() {
	for _, m := range monitoredMetrics {
		baseline := d.computeBaseline(m.Name)
		if baseline != nil {
			d.mu.Lock()
			d.baselines[m.Name] = baseline
			d.mu.Unlock()
		}
	}
}

// computeBaseline 计算单个指标的基线
func (d *AnomalyDetector) computeBaseline(metricName string) *Baseline {
	b := &Baseline{
		MetricName: metricName,
		WindowDays: 7,
		LastUpdate: time.Now().UTC(),
	}

	// 查询过去 7 天每小时的指标值
	hourlyValues := d.queryHourlyMetric(metricName, 7)
	if len(hourlyValues) == 0 {
		return b
	}

	b.SampleCount = len(hourlyValues)

	// 按 24 个小时分组
	hourBuckets := make(map[int][]float64) // hour(0-23) -> values
	dayTotals := make(map[string]float64)  // date -> sum

	for _, hv := range hourlyValues {
		hourBuckets[hv.hour] = append(hourBuckets[hv.hour], hv.value)
		dayTotals[hv.date] += hv.value
	}

	// 计算每个小时的均值和标准差
	for h := 0; h < 24; h++ {
		vals := hourBuckets[h]
		if len(vals) == 0 {
			continue
		}
		b.HourlyMean[h] = mean(vals)
		b.HourlyStd[h] = stdDev(vals)
	}

	// 计算每日总量的均值和标准差
	if len(dayTotals) > 0 {
		var dailyVals []float64
		for _, v := range dayTotals {
			dailyVals = append(dailyVals, v)
		}
		b.DailyMean = mean(dailyVals)
		b.DailyStd = stdDev(dailyVals)
	}

	// 至少 3 天数据才认为基线就绪
	b.Ready = len(dayTotals) >= 3

	return b
}

// hourlyValue 每小时的指标值
type hourlyValue struct {
	date  string  // "2025-03-15"
	hour  int     // 0-23
	value float64 // count
}

// queryHourlyMetric 查询指标的小时级数据
func (d *AnomalyDetector) queryHourlyMetric(metricName string, windowDays int) []hourlyValue {
	since := time.Now().UTC().AddDate(0, 0, -windowDays).Format(time.RFC3339)
	var query string

	switch metricName {
	case "im_requests_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
			FROM audit_log WHERE timestamp >= ? GROUP BY d, h ORDER BY d, h`
	case "im_blocks_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
			FROM audit_log WHERE action='block' AND timestamp >= ? GROUP BY d, h ORDER BY d, h`
	case "llm_calls_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
			FROM llm_calls WHERE timestamp >= ? GROUP BY d, h ORDER BY d, h`
	case "llm_tokens_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COALESCE(SUM(total_tokens),0) as cnt
			FROM llm_calls WHERE timestamp >= ? GROUP BY d, h ORDER BY d, h`
	case "tool_calls_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
			FROM llm_tool_calls WHERE timestamp >= ? GROUP BY d, h ORDER BY d, h`
	case "high_risk_tools_per_hour":
		query = `SELECT date(timestamp) as d, CAST(strftime('%H', timestamp) AS INTEGER) as h, COUNT(*) as cnt
			FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp >= ? GROUP BY d, h ORDER BY d, h`
	default:
		return nil
	}

	rows, err := d.db.Query(query, since)
	if err != nil {
		log.Printf("[异常检测] 查询指标 %s 失败: %v", metricName, err)
		return nil
	}
	defer rows.Close()

	var results []hourlyValue
	for rows.Next() {
		var date string
		var hour int
		var count float64
		if err := rows.Scan(&date, &hour, &count); err != nil {
			continue
		}
		results = append(results, hourlyValue{date: date, hour: hour, value: count})
	}
	return results
}

// queryCurrentMetric 查询当前小时的指标值
func (d *AnomalyDetector) queryCurrentMetric(metricName string) float64 {
	now := time.Now().UTC()
	hourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC).Format(time.RFC3339)
	var query string

	switch metricName {
	case "im_requests_per_hour":
		query = `SELECT COUNT(*) FROM audit_log WHERE timestamp >= ?`
	case "im_blocks_per_hour":
		query = `SELECT COUNT(*) FROM audit_log WHERE action='block' AND timestamp >= ?`
	case "llm_calls_per_hour":
		query = `SELECT COUNT(*) FROM llm_calls WHERE timestamp >= ?`
	case "llm_tokens_per_hour":
		query = `SELECT COALESCE(SUM(total_tokens),0) FROM llm_calls WHERE timestamp >= ?`
	case "tool_calls_per_hour":
		query = `SELECT COUNT(*) FROM llm_tool_calls WHERE timestamp >= ?`
	case "high_risk_tools_per_hour":
		query = `SELECT COUNT(*) FROM llm_tool_calls WHERE risk_level IN ('high','critical') AND timestamp >= ?`
	default:
		return 0
	}

	var count float64
	d.db.QueryRow(query, hourStart).Scan(&count)
	return count
}

// ============================================================
// 异常检测
// ============================================================

// CheckNow 检查所有指标的当前值是否异常
func (d *AnomalyDetector) CheckNow() []AnomalyAlert {
	d.mu.RLock()
	baselines := d.baselines
	d.mu.RUnlock()

	var alerts []AnomalyAlert
	for _, m := range monitoredMetrics {
		baseline, ok := baselines[m.Name]
		if !ok || !baseline.Ready {
			continue
		}

		currentValue := d.queryCurrentMetric(m.Name)
		alert := d.checkMetric(m.Name, currentValue, baseline)
		if alert != nil {
			alerts = append(alerts, *alert)
		}
	}
	return alerts
}

// checkMetric 检查单个指标是否异常
func (d *AnomalyDetector) checkMetric(metricName string, currentValue float64, baseline *Baseline) *AnomalyAlert {
	if !baseline.Ready {
		return nil
	}

	hour := time.Now().UTC().Hour()
	mean := baseline.HourlyMean[hour]
	std := baseline.HourlyStd[hour]

	if std < 1.0 {
		std = 1.0 // 避免除零，最小标准差为 1
	}

	deviation := math.Abs(currentValue-mean) / std

	if deviation > 2.0 {
		severity := "warning"
		if deviation > 3.0 {
			severity = "critical"
		}
		direction := "above"
		if currentValue < mean {
			direction = "below"
		}
		return &AnomalyAlert{
			ID:         fmt.Sprintf("anomaly-%s-%d", metricName, time.Now().UnixNano()%1000000),
			MetricName: metricName,
			Timestamp:  time.Now().UTC(),
			Expected:   math.Round(mean*100) / 100,
			Actual:     math.Round(currentValue*100) / 100,
			StdDev:     math.Round(std*100) / 100,
			Deviation:  math.Round(deviation*100) / 100,
			Direction:  direction,
			Severity:   severity,
		}
	}
	return nil
}

// ============================================================
// 查询 API
// ============================================================

// GetBaselines 获取所有基线状态
func (d *AnomalyDetector) GetBaselines() map[string]*Baseline {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cp := make(map[string]*Baseline, len(d.baselines))
	for k, v := range d.baselines {
		clone := *v
		cp[k] = &clone
	}
	return cp
}

// GetAlerts 获取最近告警
func (d *AnomalyDetector) GetAlerts(limit int) []AnomalyAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	if limit > len(d.alerts) {
		limit = len(d.alerts)
	}
	cp := make([]AnomalyAlert, limit)
	copy(cp, d.alerts[:limit])
	return cp
}

// GetAlerts24h 获取 24 小时内的告警数量
func (d *AnomalyDetector) GetAlerts24h() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	since := time.Now().UTC().Add(-24 * time.Hour)
	count := 0
	for _, a := range d.alerts {
		if a.Timestamp.After(since) {
			count++
		}
	}
	return count
}

// GetStatus 获取检测器状态概览
func (d *AnomalyDetector) GetStatus() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	metricsCount := len(monitoredMetrics)
	baselinesReady := 0
	for _, b := range d.baselines {
		if b.Ready {
			baselinesReady++
		}
	}

	return map[string]interface{}{
		"enabled":         true,
		"metrics_count":   metricsCount,
		"baselines_ready": baselinesReady,
		"alerts_24h":      d.GetAlerts24hInternal(),
		"total_alerts":    len(d.alerts),
	}
}

// GetAlerts24hInternal 内部使用（已持有锁或不持有锁皆可）
func (d *AnomalyDetector) GetAlerts24hInternal() int {
	since := time.Now().UTC().Add(-24 * time.Hour)
	count := 0
	for _, a := range d.alerts {
		if a.Timestamp.After(since) {
			count++
		}
	}
	return count
}

// GetMetricDetail 获取单个指标的详情（基线 + 当前值 + 是否异常）
func (d *AnomalyDetector) GetMetricDetail(metricName string) map[string]interface{} {
	d.mu.RLock()
	baseline, ok := d.baselines[metricName]
	d.mu.RUnlock()

	result := map[string]interface{}{
		"metric_name":  metricName,
		"display_name": MetricDisplayName(metricName),
	}

	if !ok {
		result["baseline"] = nil
		result["current_value"] = 0
		result["anomaly"] = false
		return result
	}

	currentValue := d.queryCurrentMetric(metricName)
	result["baseline"] = baseline
	result["current_value"] = currentValue

	alert := d.checkMetric(metricName, currentValue, baseline)
	if alert != nil {
		result["anomaly"] = true
		result["alert"] = alert
	} else {
		result["anomaly"] = false
	}

	return result
}

// InjectDemoAlerts 注入演示告警数据
func (d *AnomalyDetector) InjectDemoAlerts() {
	now := time.Now().UTC()
	demoAlerts := []AnomalyAlert{
		{
			ID: "demo-anomaly-1", MetricName: "im_requests_per_hour",
			Timestamp: now.Add(-15 * time.Minute), Expected: 42.5, Actual: 127,
			StdDev: 12.8, Deviation: 6.6, Direction: "above", Severity: "critical",
		},
		{
			ID: "demo-anomaly-2", MetricName: "tool_calls_per_hour",
			Timestamp: now.Add(-20 * time.Minute), Expected: 12.1, Actual: 31,
			StdDev: 6.5, Deviation: 2.91, Direction: "above", Severity: "warning",
		},
		{
			ID: "demo-anomaly-3", MetricName: "high_risk_tools_per_hour",
			Timestamp: now.Add(-35 * time.Minute), Expected: 3.2, Actual: 15,
			StdDev: 2.1, Deviation: 5.62, Direction: "above", Severity: "critical",
		},
		{
			ID: "demo-anomaly-4", MetricName: "llm_tokens_per_hour",
			Timestamp: now.Add(-50 * time.Minute), Expected: 8500, Actual: 23000,
			StdDev: 4200, Deviation: 3.45, Direction: "above", Severity: "critical",
		},
		{
			ID: "demo-anomaly-5", MetricName: "im_blocks_per_hour",
			Timestamp: now.Add(-1 * time.Hour), Expected: 5.3, Actual: 18,
			StdDev: 3.8, Deviation: 3.34, Direction: "above", Severity: "critical",
		},
		{
			ID: "demo-anomaly-6", MetricName: "llm_calls_per_hour",
			Timestamp: now.Add(-2 * time.Hour), Expected: 25.0, Actual: 8,
			StdDev: 7.2, Deviation: 2.36, Direction: "below", Severity: "warning",
		},
	}

	d.mu.Lock()
	d.alerts = append(demoAlerts, d.alerts...)
	if len(d.alerts) > d.maxAlerts {
		d.alerts = d.alerts[:d.maxAlerts]
	}
	d.mu.Unlock()
}

// InjectDemoBaselines 注入演示基线数据（模拟 7 天学习完成）
func (d *AnomalyDetector) InjectDemoBaselines() {
	now := time.Now().UTC()

	// 生成每个指标的模拟基线
	demoBaselines := map[string]*Baseline{
		"im_requests_per_hour": {
			MetricName: "im_requests_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 850, DailyStd: 120,
		},
		"im_blocks_per_hour": {
			MetricName: "im_blocks_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 85, DailyStd: 25,
		},
		"llm_calls_per_hour": {
			MetricName: "llm_calls_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 500, DailyStd: 80,
		},
		"llm_tokens_per_hour": {
			MetricName: "llm_tokens_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 180000, DailyStd: 35000,
		},
		"tool_calls_per_hour": {
			MetricName: "tool_calls_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 250, DailyStd: 45,
		},
		"high_risk_tools_per_hour": {
			MetricName: "high_risk_tools_per_hour", WindowDays: 7, SampleCount: 168,
			LastUpdate: now, Ready: true, DailyMean: 60, DailyStd: 15,
		},
	}

	// 填充每小时的均值和标准差（模拟日周期性）
	for _, b := range demoBaselines {
		for h := 0; h < 24; h++ {
			// 工作时间（9-18h）流量更大
			factor := 0.3
			if h >= 9 && h <= 18 {
				factor = 1.0
			} else if h >= 7 && h < 9 || h > 18 && h <= 21 {
				factor = 0.6
			}
			b.HourlyMean[h] = b.DailyMean / 24 * factor * (24.0 / averageFactor())
			b.HourlyStd[h] = b.DailyStd / 24 * factor * (24.0 / averageFactor()) * 0.8
			if b.HourlyStd[h] < 1 {
				b.HourlyStd[h] = 1
			}
		}
	}

	d.mu.Lock()
	for k, v := range demoBaselines {
		d.baselines[k] = v
	}
	d.mu.Unlock()
}

// averageFactor 计算日平均系数
func averageFactor() float64 {
	// 模拟 24 小时中的平均流量因子
	sum := 0.0
	for h := 0; h < 24; h++ {
		if h >= 9 && h <= 18 {
			sum += 1.0
		} else if h >= 7 && h < 9 || h > 18 && h <= 21 {
			sum += 0.6
		} else {
			sum += 0.3
		}
	}
	return sum / 24
}

// ============================================================
// 统计工具函数
// ============================================================

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stdDev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sumSq := 0.0
	for _, v := range vals {
		diff := v - m
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(vals)-1))
}
