// shutdown.go — 优雅关闭逻辑（v4.2）
// lobster-guard v4.2 高可用
package main

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

// ============================================================
// 优雅关闭管理器（v4.2）
// ============================================================

// ShutdownManager 管理优雅关闭的全生命周期
type ShutdownManager struct {
	shuttingDown   int32 // atomic: 0=正常, 1=正在关闭
	shutdownTimeout time.Duration
	cfg            *Config

	// 需要关闭的组件引用
	inSrv   *http.Server
	outSrv  *http.Server
	mgmtSrv *http.Server
	bridge  BridgeConnector
	logger  *AuditLogger
	store   Store
	cancel  context.CancelFunc // 取消主 context
	wsProxy *WSProxyManager
}

// NewShutdownManager 创建关闭管理器
func NewShutdownManager(cfg *Config) *ShutdownManager {
	timeout := 30 * time.Second
	if cfg.ShutdownTimeout > 0 {
		timeout = time.Duration(cfg.ShutdownTimeout) * time.Second
	}
	return &ShutdownManager{
		cfg:             cfg,
		shutdownTimeout: timeout,
	}
}

// SetServers 设置需要关闭的 HTTP 服务器
func (sm *ShutdownManager) SetServers(inSrv, outSrv, mgmtSrv *http.Server) {
	sm.inSrv = inSrv
	sm.outSrv = outSrv
	sm.mgmtSrv = mgmtSrv
}

// SetBridge 设置 Bridge 连接器
func (sm *ShutdownManager) SetBridge(bridge BridgeConnector) {
	sm.bridge = bridge
}

// SetLogger 设置审计日志
func (sm *ShutdownManager) SetLogger(logger *AuditLogger) {
	sm.logger = logger
}

// SetStore 设置存储
func (sm *ShutdownManager) SetStore(store Store) {
	sm.store = store
}

// SetCancel 设置主 context cancel 函数
func (sm *ShutdownManager) SetCancel(cancel context.CancelFunc) {
	sm.cancel = cancel
}

// SetWSProxy 设置 WebSocket 代理管理器
func (sm *ShutdownManager) SetWSProxy(wsProxy *WSProxyManager) {
	sm.wsProxy = wsProxy
}

// IsShuttingDown 检查是否正在关闭
func (sm *ShutdownManager) IsShuttingDown() bool {
	return atomic.LoadInt32(&sm.shuttingDown) == 1
}

// Shutdown 执行优雅关闭流程
func (sm *ShutdownManager) Shutdown() {
	atomic.StoreInt32(&sm.shuttingDown, 1)

	log.Printf("[关闭] 开始优雅关闭（超时 %v）...", sm.shutdownTimeout)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), sm.shutdownTimeout)
	defer shutdownCancel()

	// 步骤 1/7: 标记健康检查为 unhealthy
	log.Printf("[关闭] 步骤 1/7: 标记健康检查为 unhealthy")
	// IsShuttingDown() 已经返回 true, /healthz 会返回 503

	// 步骤 2/7: 停止接受新连接 + 等待进行中的请求
	log.Printf("[关闭] 步骤 2/7: 停止接受新连接，等待进行中的请求完成...")
	if sm.cancel != nil {
		sm.cancel()
	}

	// 关闭入站服务器（会等待进行中的请求完成）
	if sm.inSrv != nil {
		if err := sm.inSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[关闭] 入站服务关闭出错: %v", err)
		}
	}

	// 步骤 3/7: 关闭出站服务
	log.Printf("[关闭] 步骤 3/7: 关闭出站服务...")
	if sm.outSrv != nil {
		if err := sm.outSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[关闭] 出站服务关闭出错: %v", err)
		}
	}

	// 步骤 4/7: 关闭 WebSocket 连接
	log.Printf("[关闭] 步骤 4/7: 关闭活跃 WebSocket 连接...")
	if sm.wsProxy != nil {
		sm.wsProxy.CloseAll()
	}

	// 步骤 5/7: 停止 Bridge Mode
	log.Printf("[关闭] 步骤 5/7: 停止 Bridge Mode 连接...")
	if sm.bridge != nil {
		sm.bridge.Stop()
	}

	// 步骤 6/7: 刷新审计日志
	log.Printf("[关闭] 步骤 6/7: 刷新审计日志到磁盘...")
	if sm.logger != nil {
		sm.logger.Close()
	}

	// 步骤 7/7: 关闭数据库 + 管理服务
	log.Printf("[关闭] 步骤 7/7: 关闭数据库和管理服务...")
	if sm.mgmtSrv != nil {
		sm.mgmtSrv.Shutdown(shutdownCtx)
	}
	if sm.store != nil {
		sm.store.Close()
	}

	log.Println("[关闭] 龙虾卫士已停止 ✅")
}

// ============================================================
// 健康检查增强（v4.2）
// ============================================================

// HealthCheckResult 多维度健康检查结果
type HealthCheckResult struct {
	Status  string                       `json:"status"` // healthy, degraded, unhealthy, shutting_down
	Checks  map[string]*HealthCheckItem  `json:"checks"`
	Uptime  string                       `json:"uptime"`
	Version string                       `json:"version"`
}

// HealthCheckItem 单个健康检查项
type HealthCheckItem struct {
	Status      string  `json:"status"`                 // ok, warning, error
	LatencyMs   float64 `json:"latency_ms,omitempty"`
	Healthy     int     `json:"healthy,omitempty"`
	Total       int     `json:"total,omitempty"`
	UsedPercent float64 `json:"used_percent,omitempty"`
	AllocMB     float64 `json:"alloc_mb,omitempty"`
	Count       int     `json:"count,omitempty"`
}

// PerformHealthChecks 执行所有健康检查
func PerformHealthChecks(store Store, pool *UpstreamPool, dbPath string) *HealthCheckResult {
	result := &HealthCheckResult{
		Checks:  make(map[string]*HealthCheckItem),
		Uptime:  time.Since(startTime).Round(time.Second).String(),
		Version: AppVersion,
	}

	overallStatus := "healthy"

	// 1. Database check
	dbCheck := checkDatabase(store)
	result.Checks["database"] = dbCheck
	if dbCheck.Status == "error" {
		overallStatus = "unhealthy"
	} else if dbCheck.Status == "warning" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// 2. Upstream check
	upCheck := checkUpstreams(pool)
	result.Checks["upstream"] = upCheck
	if upCheck.Status == "error" {
		overallStatus = "unhealthy"
	} else if upCheck.Status == "warning" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// 3. Disk check
	diskCheck := checkDisk(dbPath)
	result.Checks["disk"] = diskCheck
	if diskCheck.Status == "error" {
		overallStatus = "unhealthy"
	} else if diskCheck.Status == "warning" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// 4. Memory check
	memCheck := checkMemory()
	result.Checks["memory"] = memCheck
	if memCheck.Status == "error" {
		overallStatus = "unhealthy"
	} else if memCheck.Status == "warning" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	// 5. Goroutines check
	grCheck := checkGoroutines()
	result.Checks["goroutines"] = grCheck
	if grCheck.Status == "error" {
		overallStatus = "unhealthy"
	} else if grCheck.Status == "warning" && overallStatus == "healthy" {
		overallStatus = "degraded"
	}

	result.Status = overallStatus
	return result
}

func checkDatabase(store Store) *HealthCheckItem {
	item := &HealthCheckItem{}
	if store == nil {
		item.Status = "ok" // no store configured = skip check
		return item
	}
	start := time.Now()
	err := store.Ping()
	item.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		item.Status = "error"
	} else if item.LatencyMs > 100 {
		item.Status = "warning" // 慢响应
	} else {
		item.Status = "ok"
	}
	return item
}

func checkUpstreams(pool *UpstreamPool) *HealthCheckItem {
	item := &HealthCheckItem{}
	if pool == nil {
		item.Status = "ok"
		item.Total = 0
		item.Healthy = 0
		return item
	}
	total, healthy := pool.Count()
	item.Total = total
	item.Healthy = healthy
	if total == 0 {
		item.Status = "warning"
	} else if healthy == 0 {
		item.Status = "error"
	} else if healthy < total {
		item.Status = "warning"
	} else {
		item.Status = "ok"
	}
	return item
}

func checkDisk(dbPath string) *HealthCheckItem {
	item := &HealthCheckItem{}
	usedPct := getDiskUsagePercent(dbPath)
	item.UsedPercent = usedPct
	if usedPct >= 95 {
		item.Status = "error"
	} else if usedPct >= 90 {
		item.Status = "warning"
	} else {
		item.Status = "ok"
	}
	return item
}

func checkMemory() *HealthCheckItem {
	item := &HealthCheckItem{}
	allocMB := getMemoryAllocMB()
	item.AllocMB = allocMB
	if allocMB >= 1024 {
		item.Status = "error" // >1GB
	} else if allocMB >= 768 {
		item.Status = "warning" // >768MB
	} else {
		item.Status = "ok"
	}
	return item
}

func checkGoroutines() *HealthCheckItem {
	item := &HealthCheckItem{}
	count := getGoroutineCount()
	item.Count = count
	if count >= 10000 {
		item.Status = "error"
	} else if count >= 5000 {
		item.Status = "warning"
	} else {
		item.Status = "ok"
	}
	return item
}

// ============================================================
// 系统指标辅助函数
// ============================================================

// getDiskUsagePercent 获取指定路径所在文件系统的磁盘使用百分比
func getDiskUsagePercent(path string) float64 {
	if path == "" {
		path = "/"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0
	}
	used := total - free
	return float64(used) / float64(total) * 100.0
}

// getMemoryAllocMB 获取 Go runtime 当前堆内存分配（MB）
func getMemoryAllocMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024.0 / 1024.0
}

// getGoroutineCount 获取当前 goroutine 数量
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}
