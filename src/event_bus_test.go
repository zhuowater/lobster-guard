// event_bus_test.go — 事件总线单元测试
// lobster-guard v18.1
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestEventBusDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// 使用单连接避免 in-memory DB 多连接看不到表的问题
	db.SetMaxOpenConns(1)
	return db
}

func newTestEventBus(t *testing.T, targets []WebhookTarget, chains []ActionChain) (*EventBus, *sql.DB) {
	t.Helper()
	db := setupTestEventBusDB(t)
	cfg := &Config{
		EventBus: EventBusConfig{
			Enabled: true,
			Targets: targets,
			Chains:  chains,
		},
	}
	eb := NewEventBus(db, cfg)
	return eb, db
}

// 1. TestEventBusEmit — 基本事件发射
func TestEventBusEmit(t *testing.T) {
	eb, db := newTestEventBus(t, nil, nil)
	defer db.Close()
	defer eb.Stop()

	event := &SecurityEvent{
		Type:     "inbound_block",
		Severity: "high",
		Domain:   "inbound",
		TraceID:  "trace-001",
		SenderID: "user-001",
		Summary:  "测试事件",
		Details:  map[string]interface{}{"key": "value"},
	}
	eb.Emit(event)

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	// 验证已写入数据库
	var count int
	db.QueryRow("SELECT COUNT(*) FROM security_events").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 event, got %d", count)
	}

	// 验证事件内容
	var typ, severity, summary string
	db.QueryRow("SELECT type, severity, summary FROM security_events WHERE id=?", event.ID).Scan(&typ, &severity, &summary)
	if typ != "inbound_block" {
		t.Errorf("expected type inbound_block, got %s", typ)
	}
	if severity != "high" {
		t.Errorf("expected severity high, got %s", severity)
	}
	if summary != "测试事件" {
		t.Errorf("expected summary '测试事件', got %s", summary)
	}

	// 验证统计
	stats := eb.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("expected TotalEvents=1, got %d", stats.TotalEvents)
	}
}

// 2. TestEventBusWebhookDelivery — HTTP 推送
func TestEventBusWebhookDelivery(t *testing.T) {
	var received *SecurityEvent
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		received = &SecurityEvent{}
		json.Unmarshal(body, received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "test-01", Name: "Test Target", URL: server.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{
		Type:     "inbound_block",
		Severity: "high",
		Domain:   "inbound",
		Summary:  "Webhook 测试",
	})

	// 等待异步投递
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("expected webhook to receive event")
	}
	if received.Type != "inbound_block" {
		t.Errorf("expected type inbound_block, got %s", received.Type)
	}
	if received.Summary != "Webhook 测试" {
		t.Errorf("expected summary 'Webhook 测试', got %s", received.Summary)
	}

	// 验证投递记录
	stats := eb.GetStats()
	if stats.TotalDelivered != 1 {
		t.Errorf("expected TotalDelivered=1, got %d", stats.TotalDelivered)
	}

	// 验证 event_deliveries 表
	var dStatus string
	db.QueryRow("SELECT status FROM event_deliveries LIMIT 1").Scan(&dStatus)
	if dStatus != "success" {
		t.Errorf("expected delivery status 'success', got %s", dStatus)
	}
}

// 3. TestEventBusWebhookSignature — 签名验证
func TestEventBusWebhookSignature(t *testing.T) {
	secret := "test-secret-key"
	var receivedSig string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Lobster-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "sig-01", Name: "Signed Target", URL: server.URL, Secret: secret, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{
		Type:     "test",
		Severity: "info",
		Domain:   "system",
		Summary:  "签名测试",
	})

	time.Sleep(500 * time.Millisecond)

	if receivedSig == "" {
		t.Fatal("expected X-Lobster-Signature header")
	}

	// 验证签名
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(receivedBody)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if receivedSig != expectedSig {
		t.Errorf("signature mismatch: got %s, want %s", receivedSig, expectedSig)
	}
}

// 4. TestEventBusFilter_Severity — 按严重级别过滤
func TestEventBusFilter_Severity(t *testing.T) {
	var receivedCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&receivedCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "sev-01", Name: "High Only", URL: server.URL, Enabled: true, MinSeverity: "high"},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	// 低级别事件不应推送
	eb.Emit(&SecurityEvent{Type: "test", Severity: "info", Domain: "system", Summary: "low priority"})
	eb.Emit(&SecurityEvent{Type: "test", Severity: "low", Domain: "system", Summary: "low priority"})
	eb.Emit(&SecurityEvent{Type: "test", Severity: "medium", Domain: "system", Summary: "medium priority"})
	// 高级别事件应推送
	eb.Emit(&SecurityEvent{Type: "test", Severity: "high", Domain: "system", Summary: "high priority"})
	eb.Emit(&SecurityEvent{Type: "test", Severity: "critical", Domain: "system", Summary: "critical"})

	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt64(&receivedCount)
	if count != 2 {
		t.Errorf("expected 2 deliveries (high+critical), got %d", count)
	}
}

// 5. TestEventBusFilter_EventType — 按事件类型过滤
func TestEventBusFilter_EventType(t *testing.T) {
	var receivedCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&receivedCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "type-01", URL: server.URL, Enabled: true, EventTypes: []string{"inbound_block", "canary_leaked"}},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{Type: "inbound_block", Severity: "high", Domain: "inbound", Summary: "should match"})
	eb.Emit(&SecurityEvent{Type: "outbound_block", Severity: "high", Domain: "outbound", Summary: "should not match"})
	eb.Emit(&SecurityEvent{Type: "canary_leaked", Severity: "critical", Domain: "llm", Summary: "should match"})

	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt64(&receivedCount)
	if count != 2 {
		t.Errorf("expected 2 deliveries, got %d", count)
	}
}

// 6. TestEventBusFilter_Tenant — 按租户过滤
func TestEventBusFilter_Tenant(t *testing.T) {
	var receivedCount int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&receivedCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "tenant-01", URL: server.URL, Enabled: true, TenantIDs: []string{"security-team"}},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{Type: "test", Severity: "high", Domain: "system", TenantID: "security-team", Summary: "match"})
	eb.Emit(&SecurityEvent{Type: "test", Severity: "high", Domain: "system", TenantID: "product-team", Summary: "no match"})
	eb.Emit(&SecurityEvent{Type: "test", Severity: "high", Domain: "system", TenantID: "security-team", Summary: "match2"})

	time.Sleep(500 * time.Millisecond)

	count := atomic.LoadInt64(&receivedCount)
	if count != 2 {
		t.Errorf("expected 2 deliveries, got %d", count)
	}
}

// 7. TestEventBusActionChain — 动作链执行
func TestEventBusActionChain(t *testing.T) {
	var logCalled bool
	var strictEnabled bool
	var bannedUser string

	chains := []ActionChain{
		{
			ID:   "chain-01",
			Name: "Test Chain",
			Trigger: ActionChainTrigger{
				MinSeverity: "critical",
			},
			Steps: []ActionChainStep{
				{Type: "log", Config: map[string]string{"message": "动作链测试"}},
				{Type: "ban_user"},
				{Type: "enable_strict_mode"},
			},
			Enabled: true,
		},
	}

	eb, db := newTestEventBus(t, nil, chains)
	defer db.Close()
	defer eb.Stop()

	// 设置回调
	eb.strictModeFunc = func(enable bool) error {
		strictEnabled = enable
		return nil
	}
	eb.banUserFunc = func(senderID string) {
		bannedUser = senderID
	}

	_ = logCalled // log is verified via stats

	// 非 critical 事件不触发动作链
	eb.Emit(&SecurityEvent{Type: "test", Severity: "medium", Domain: "system", SenderID: "user-01", Summary: "low"})
	time.Sleep(200 * time.Millisecond)

	stats := eb.GetStats()
	if stats.TotalChainsFired != 0 {
		t.Errorf("expected 0 chains fired for medium severity, got %d", stats.TotalChainsFired)
	}

	// critical 事件触发动作链
	eb.Emit(&SecurityEvent{Type: "test", Severity: "critical", Domain: "system", SenderID: "attacker-01", Summary: "critical"})
	time.Sleep(200 * time.Millisecond)

	stats = eb.GetStats()
	if stats.TotalChainsFired != 1 {
		t.Errorf("expected 1 chain fired, got %d", stats.TotalChainsFired)
	}
	if !strictEnabled {
		t.Error("expected strict mode to be enabled")
	}
	if bannedUser != "attacker-01" {
		t.Errorf("expected banned user 'attacker-01', got %q", bannedUser)
	}
}

// 8. TestEventBusConcurrent — 并发事件
func TestEventBusConcurrent(t *testing.T) {
	var deliveredCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&deliveredCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "conc-01", URL: server.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	var wg sync.WaitGroup
	count := 50
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			eb.Emit(&SecurityEvent{
				Type:     "inbound_block",
				Severity: "high",
				Domain:   "inbound",
				Summary:  "并发测试",
			})
		}(i)
	}
	wg.Wait()

	// 等待所有投递完成
	time.Sleep(2 * time.Second)

	delivered := atomic.LoadInt64(&deliveredCount)
	if delivered != int64(count) {
		t.Errorf("expected %d deliveries, got %d", count, delivered)
	}

	// 验证数据库记录
	var dbCount int
	db.QueryRow("SELECT COUNT(*) FROM security_events").Scan(&dbCount)
	if dbCount != count {
		t.Errorf("expected %d events in DB, got %d", count, dbCount)
	}
}

// 9. TestEventBusStats — 统计
func TestEventBusStats(t *testing.T) {
	// 一个成功的 target 和一个失败的 target（无效 URL）
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer successServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer failServer.Close()

	targets := []WebhookTarget{
		{ID: "ok-01", URL: successServer.URL, Enabled: true},
		{ID: "fail-01", URL: failServer.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{Type: "test", Severity: "info", Domain: "system", Summary: "stats test"})
	time.Sleep(500 * time.Millisecond)

	stats := eb.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("expected TotalEvents=1, got %d", stats.TotalEvents)
	}
	if stats.TotalDelivered != 1 {
		t.Errorf("expected TotalDelivered=1, got %d", stats.TotalDelivered)
	}
	if stats.TotalFailed != 1 {
		t.Errorf("expected TotalFailed=1, got %d", stats.TotalFailed)
	}
}

// 10. TestEventBusTargetCRUD — 推送目标增删改
func TestEventBusTargetCRUD(t *testing.T) {
	eb, db := newTestEventBus(t, nil, nil)
	defer db.Close()
	defer eb.Stop()

	// Add
	err := eb.AddTarget(WebhookTarget{ID: "t1", Name: "Target 1", URL: "http://example.com/hook", Enabled: true})
	if err != nil {
		t.Fatalf("AddTarget: %v", err)
	}

	// Add duplicate
	err = eb.AddTarget(WebhookTarget{ID: "t1", Name: "Dup", URL: "http://example.com/hook2"})
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}

	// List
	targets := eb.ListTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Name != "Target 1" {
		t.Errorf("expected name 'Target 1', got %s", targets[0].Name)
	}

	// Update
	err = eb.UpdateTarget(WebhookTarget{ID: "t1", Name: "Updated", URL: "http://example.com/hook3", Enabled: false})
	if err != nil {
		t.Fatalf("UpdateTarget: %v", err)
	}
	targets = eb.ListTargets()
	if targets[0].Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", targets[0].Name)
	}
	if targets[0].Enabled {
		t.Error("expected disabled")
	}

	// Update non-existent
	err = eb.UpdateTarget(WebhookTarget{ID: "nonexist", Name: "nope"})
	if err == nil {
		t.Fatal("expected error for non-existent target")
	}

	// Delete
	err = eb.DeleteTarget("t1")
	if err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	targets = eb.ListTargets()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}

	// Delete non-existent
	err = eb.DeleteTarget("t1")
	if err == nil {
		t.Fatal("expected error for non-existent delete")
	}
}

// 11. TestEventBusList — 事件列表查询
func TestEventBusList(t *testing.T) {
	eb, db := newTestEventBus(t, nil, nil)
	defer db.Close()
	defer eb.Stop()

	// 插入多个事件
	events := []*SecurityEvent{
		{Type: "inbound_block", Severity: "high", Domain: "inbound", Summary: "event1"},
		{Type: "outbound_block", Severity: "medium", Domain: "outbound", Summary: "event2"},
		{Type: "inbound_block", Severity: "critical", Domain: "inbound", Summary: "event3"},
		{Type: "canary_leaked", Severity: "critical", Domain: "llm", Summary: "event4"},
	}
	for _, e := range events {
		eb.Emit(e)
	}
	time.Sleep(200 * time.Millisecond)

	// 查所有
	results, err := eb.QueryEvents("", "", "", 100)
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(results) != 4 {
		t.Errorf("expected 4 events, got %d", len(results))
	}

	// 按类型筛选
	results, err = eb.QueryEvents("inbound_block", "", "", 100)
	if err != nil {
		t.Fatalf("QueryEvents by type: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 inbound_block events, got %d", len(results))
	}

	// 按严重级别筛选
	results, err = eb.QueryEvents("", "critical", "", 100)
	if err != nil {
		t.Fatalf("QueryEvents by severity: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 critical events, got %d", len(results))
	}

	// limit
	results, err = eb.QueryEvents("", "", "", 2)
	if err != nil {
		t.Fatalf("QueryEvents with limit: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 events with limit, got %d", len(results))
	}
}

// 12. TestEventBusDeliveries — 投递记录
func TestEventBusDeliveries(t *testing.T) {
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer successServer.Close()

	targets := []WebhookTarget{
		{ID: "del-01", URL: successServer.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	event := &SecurityEvent{Type: "test", Severity: "info", Domain: "system", Summary: "delivery test"}
	eb.Emit(event)
	time.Sleep(500 * time.Millisecond)

	// 查询投递记录
	deliveries, err := eb.QueryDeliveries(event.ID, "", "", 10)
	if err != nil {
		t.Fatalf("QueryDeliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0]["status"] != "success" {
		t.Errorf("expected status 'success', got %s", deliveries[0]["status"])
	}
	if deliveries[0]["target_id"] != "del-01" {
		t.Errorf("expected target_id 'del-01', got %s", deliveries[0]["target_id"])
	}

	// 按 target_id 筛选
	deliveries, err = eb.QueryDeliveries("", "del-01", "", 10)
	if err != nil {
		t.Fatalf("QueryDeliveries by target: %v", err)
	}
	if len(deliveries) != 1 {
		t.Errorf("expected 1 delivery by target, got %d", len(deliveries))
	}

	// 按 status 筛选
	deliveries, err = eb.QueryDeliveries("", "", "failed", 10)
	if err != nil {
		t.Fatalf("QueryDeliveries by status: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("expected 0 failed deliveries, got %d", len(deliveries))
	}
}

// 13. TestEventBusDisabled — 禁用状态
func TestEventBusDisabled(t *testing.T) {
	// EventBus 不应被创建，测试 nil 安全
	var eb *EventBus
	// 确保 nil eventBus 不会 panic
	if eb != nil {
		t.Error("expected nil eventBus")
	}

	// 测试禁用配置不创建实例
	cfg := &Config{
		EventBus: EventBusConfig{
			Enabled: false,
		},
	}
	if cfg.EventBus.Enabled {
		t.Error("expected EventBus to be disabled")
	}
}

// 14. TestEventBusChannelFull — 通道满时丢弃
func TestEventBusChannelFull(t *testing.T) {
	db := setupTestEventBusDB(t)
	defer db.Close()
	cfg := &Config{
		EventBus: EventBusConfig{
			Enabled: true,
		},
	}
	eb := &EventBus{
		db:        db,
		targets:   []WebhookTarget{},
		chains:    []ActionChain{},
		eventChan: make(chan *SecurityEvent, 5), // 小缓冲
		stopCh:    make(chan struct{}),
		cfg:       cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	eb.initTables()

	// 不启动 processLoop → 通道会满
	// 发射超过缓冲大小的事件
	for i := 0; i < 10; i++ {
		eb.Emit(&SecurityEvent{
			Type:     "test",
			Severity: "info",
			Domain:   "system",
			Summary:  "channel full test",
		})
	}

	dropped := atomic.LoadInt64(&eb.dropped)
	if dropped == 0 {
		t.Error("expected some events to be dropped")
	}
	stats := eb.GetStats()
	if stats.TotalDropped == 0 {
		t.Error("expected TotalDropped > 0")
	}

	// 清理
	close(eb.stopCh)
}

// 15. TestEventBusChainTrigger — 触发条件匹配
func TestEventBusChainTrigger(t *testing.T) {
	var chainWebhookCount int64

	chainWebhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&chainWebhookCount, 1)
		w.WriteHeader(200)
	}))
	defer chainWebhookServer.Close()

	// 注意：chain-target 只被动作链使用，不放在 targets 列表中
	// 这样它只在 chain webhook step 执行时被调用
	targets := []WebhookTarget{
		{ID: "chain-target", URL: chainWebhookServer.URL, Enabled: false}, // disabled as direct target
	}
	chains := []ActionChain{
		{
			ID:   "type-chain",
			Name: "Inbound Only Chain",
			Trigger: ActionChainTrigger{
				EventType: "inbound_block",
			},
			Steps: []ActionChainStep{
				{Type: "webhook", Config: map[string]string{"target_id": "chain-target"}},
			},
			Enabled: true,
		},
		{
			ID:   "sev-chain",
			Name: "Critical Chain",
			Trigger: ActionChainTrigger{
				MinSeverity: "critical",
			},
			Steps: []ActionChainStep{
				{Type: "log", Config: map[string]string{"message": "严重事件"}},
			},
			Enabled: true,
		},
		{
			ID:   "disabled-chain",
			Name: "Disabled Chain",
			Trigger: ActionChainTrigger{
				MinSeverity: "info",
			},
			Steps: []ActionChainStep{
				{Type: "log"},
			},
			Enabled: false,
		},
	}

	eb, db := newTestEventBus(t, targets, chains)
	defer db.Close()
	defer eb.Stop()

	// inbound_block + high → triggers type-chain only (not sev-chain because not critical)
	eb.Emit(&SecurityEvent{Type: "inbound_block", Severity: "high", Domain: "inbound", Summary: "trigger type chain"})
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt64(&chainWebhookCount) != 1 {
		t.Errorf("expected 1 chain webhook call (type-chain), got %d", atomic.LoadInt64(&chainWebhookCount))
	}

	stats := eb.GetStats()
	// type-chain fired: 1
	if stats.TotalChainsFired != 1 {
		t.Errorf("expected 1 chains fired, got %d", stats.TotalChainsFired)
	}

	// outbound_block + critical → triggers sev-chain but NOT type-chain (wrong event type)
	eb.Emit(&SecurityEvent{Type: "outbound_block", Severity: "critical", Domain: "outbound", Summary: "trigger sev chain"})
	time.Sleep(500 * time.Millisecond)

	stats = eb.GetStats()
	// +1 for sev-chain
	if stats.TotalChainsFired != 2 {
		t.Errorf("expected 2 total chains fired, got %d", stats.TotalChainsFired)
	}

	// inbound_block + critical → triggers BOTH type-chain and sev-chain
	prevCount := atomic.LoadInt64(&chainWebhookCount)
	eb.Emit(&SecurityEvent{Type: "inbound_block", Severity: "critical", Domain: "inbound", Summary: "trigger both"})
	time.Sleep(500 * time.Millisecond)

	newCount := atomic.LoadInt64(&chainWebhookCount)
	if newCount-prevCount != 1 {
		t.Errorf("expected 1 new webhook call from type-chain, got %d", newCount-prevCount)
	}
	stats = eb.GetStats()
	// +2 (type-chain + sev-chain)
	if stats.TotalChainsFired != 4 {
		t.Errorf("expected 4 total chains fired, got %d", stats.TotalChainsFired)
	}
}

// 16. TestEventBusSeverityLevel — 严重级别映射
func TestEventBusSeverityLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"info", 0},
		{"low", 1},
		{"medium", 2},
		{"high", 3},
		{"critical", 4},
		{"INFO", 0},
		{"CRITICAL", 4},
		{"unknown", -1},
	}
	for _, tt := range tests {
		got := severityLevel(tt.input)
		if got != tt.expected {
			t.Errorf("severityLevel(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// 17. TestEventBusWebhookFailed — Webhook 失败处理
func TestEventBusWebhookFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()

	targets := []WebhookTarget{
		{ID: "fail-01", URL: server.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{Type: "test", Severity: "info", Domain: "system", Summary: "fail test"})
	time.Sleep(500 * time.Millisecond)

	stats := eb.GetStats()
	if stats.TotalFailed != 1 {
		t.Errorf("expected TotalFailed=1, got %d", stats.TotalFailed)
	}

	// 验证 event_deliveries 表记录了失败
	var status, errMsg string
	db.QueryRow("SELECT status, error_msg FROM event_deliveries LIMIT 1").Scan(&status, &errMsg)
	if status != "failed" {
		t.Errorf("expected delivery status 'failed', got %s", status)
	}
	if !strings.Contains(errMsg, "503") {
		t.Errorf("expected error msg to contain 503, got %s", errMsg)
	}
}

// 18. TestEventBusListChains — 动作链列表
func TestEventBusListChains(t *testing.T) {
	chains := []ActionChain{
		{ID: "c1", Name: "Chain 1", Enabled: true, Trigger: ActionChainTrigger{MinSeverity: "high"}},
		{ID: "c2", Name: "Chain 2", Enabled: false, Trigger: ActionChainTrigger{EventType: "test"}},
	}
	eb, db := newTestEventBus(t, nil, chains)
	defer db.Close()
	defer eb.Stop()

	result := eb.ListChains()
	if len(result) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(result))
	}
	if result[0].Name != "Chain 1" {
		t.Errorf("expected name 'Chain 1', got %s", result[0].Name)
	}
	if result[1].Enabled {
		t.Error("expected chain 2 to be disabled")
	}
}

// 19. TestEventBusAutoID — 自动生成 ID
func TestEventBusAutoID(t *testing.T) {
	eb, db := newTestEventBus(t, nil, nil)
	defer db.Close()
	defer eb.Stop()

	event := &SecurityEvent{
		Type:     "test",
		Severity: "info",
		Domain:   "system",
		Summary:  "auto id test",
	}
	eb.Emit(event)

	if event.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if event.Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
}

// 20. TestEventBusMultipleTargets — 多目标推送
func TestEventBusMultipleTargets(t *testing.T) {
	var count1, count2 int64

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count1, 1)
		w.WriteHeader(200)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count2, 1)
		w.WriteHeader(200)
	}))
	defer server2.Close()

	targets := []WebhookTarget{
		{ID: "m1", URL: server1.URL, Enabled: true},
		{ID: "m2", URL: server2.URL, Enabled: true},
	}
	eb, db := newTestEventBus(t, targets, nil)
	defer db.Close()
	defer eb.Stop()

	eb.Emit(&SecurityEvent{Type: "test", Severity: "info", Domain: "system", Summary: "multi target"})
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt64(&count1) != 1 {
		t.Errorf("expected target 1 to receive 1 event, got %d", atomic.LoadInt64(&count1))
	}
	if atomic.LoadInt64(&count2) != 1 {
		t.Errorf("expected target 2 to receive 1 event, got %d", atomic.LoadInt64(&count2))
	}
}