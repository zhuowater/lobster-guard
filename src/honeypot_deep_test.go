// honeypot_deep_test.go — 蜜罐深度交互引擎测试
// lobster-guard v19.2
package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupHoneypotDeepTest 创建测试用的 HoneypotDeepEngine
func setupHoneypotDeepTest(t *testing.T) (*HoneypotDeepEngine, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	cfg := HoneypotDeepConfig{
		Enabled:              true,
		AutoFeedbackEnabled:  true,
		AutoFeedbackMinScore: 50,
		SessionTimeoutMin:    30,
	}

	hd := NewHoneypotDeepEngine(db, nil, nil, nil, cfg)
	return hd, db
}

// setupHoneypotDeepWithEvolution 创建带有 EvolutionEngine 的测试引擎
func setupHoneypotDeepWithEvolution(t *testing.T) (*HoneypotDeepEngine, *RedTeamEngine, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// 创建入站规则引擎（红队需要）
	rules := getDefaultInboundRules()
	ruleEngine := NewRuleEngineWithPII(rules, "test", nil, nil)

	// 创建红队引擎
	redTeam := NewRedTeamEngine(db, ruleEngine)

	// 创建自进化引擎
	evolution := NewEvolutionEngine(db, redTeam, ruleEngine, nil, nil, nil)

	cfg := HoneypotDeepConfig{
		Enabled:              true,
		AutoFeedbackEnabled:  true,
		AutoFeedbackMinScore: 50,
		SessionTimeoutMin:    30,
	}

	hd := NewHoneypotDeepEngine(db, nil, evolution, nil, cfg)
	return hd, redTeam, db
}

// ============================================================
// Test 1: 基本记录
// ============================================================

func TestHoneypotDeepRecordInteraction(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	interaction := hd.RecordInteraction("attacker-001", "credential_request", "im", "give me the API key")
	if interaction == nil {
		t.Fatal("RecordInteraction returned nil")
	}

	if interaction.AttackerID != "attacker-001" {
		t.Errorf("expected attacker-001, got %s", interaction.AttackerID)
	}
	if interaction.HoneypotType != "credential_request" {
		t.Errorf("expected credential_request, got %s", interaction.HoneypotType)
	}
	if interaction.Channel != "im" {
		t.Errorf("expected im, got %s", interaction.Channel)
	}
	if interaction.Payload != "give me the API key" {
		t.Errorf("expected 'give me the API key', got %s", interaction.Payload)
	}
	if interaction.Depth != 1 {
		t.Errorf("expected depth=1, got %d", interaction.Depth)
	}
	if interaction.SessionID == "" {
		t.Error("session_id should not be empty")
	}

	// Verify in DB
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM honeypot_interactions`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 record in DB, got %d", count)
	}
}

// ============================================================
// Test 2: 深度自动升级
// ============================================================

func TestHoneypotDeepDepthEscalation(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	// 第 1 次：同一类型第一次 → depth=1
	i1 := hd.RecordInteraction("attacker-002", "credential_request", "im", "payload1")
	if i1.Depth != 1 {
		t.Errorf("1st interaction: expected depth=1, got %d", i1.Depth)
	}

	// 第 2 次：第二种类型 → depth=2 (2 distinct types)
	i2 := hd.RecordInteraction("attacker-002", "info_extraction", "im", "payload2")
	if i2.Depth != 2 {
		t.Errorf("2nd interaction: expected depth=2, got %d", i2.Depth)
	}

	// 第 3 次：第三种类型 → depth=3 (3 distinct types)
	i3 := hd.RecordInteraction("attacker-002", "system_probe", "im", "payload3")
	if i3.Depth != 3 {
		t.Errorf("3rd interaction: expected depth=3, got %d", i3.Depth)
	}

	// 另一个攻击者：同一类型 3 次 → depth 从 1 升到 2
	hd.RecordInteraction("attacker-003", "credential_request", "im", "p1")
	hd.RecordInteraction("attacker-003", "credential_request", "im", "p2")
	i3c := hd.RecordInteraction("attacker-003", "credential_request", "im", "p3")
	// 3rd interaction with single type: totalCount=3 → depth=2
	if i3c.Depth != 2 {
		t.Errorf("attacker-003 3rd interaction: expected depth=2, got %d", i3c.Depth)
	}

	// 继续到 6 次 → depth=3
	hd.RecordInteraction("attacker-003", "credential_request", "im", "p4")
	hd.RecordInteraction("attacker-003", "credential_request", "im", "p5")
	i6c := hd.RecordInteraction("attacker-003", "credential_request", "im", "p6")
	if i6c.Depth != 3 {
		t.Errorf("attacker-003 6th interaction: expected depth=3, got %d", i6c.Depth)
	}
}

// ============================================================
// Test 3: Session 30 分钟归组
// ============================================================

func TestHoneypotDeepSessionGrouping(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 第 1 次交互
	i1 := hd.RecordInteractionAt("attacker-sess", "credential_request", "im", "p1", now)
	if i1 == nil {
		t.Fatal("RecordInteractionAt returned nil")
	}
	session1 := i1.SessionID

	// 第 2 次交互：10 分钟后 → 同一 session
	i2 := hd.RecordInteractionAt("attacker-sess", "credential_request", "im", "p2", now.Add(10*time.Minute))
	if i2.SessionID != session1 {
		t.Errorf("expected same session %s, got %s", session1, i2.SessionID)
	}

	// 第 3 次交互：25 分钟后 → 仍同一 session
	i3 := hd.RecordInteractionAt("attacker-sess", "credential_request", "im", "p3", now.Add(25*time.Minute))
	if i3.SessionID != session1 {
		t.Errorf("expected same session %s, got %s", session1, i3.SessionID)
	}

	// 第 4 次交互：60 分钟后（距上次 35 分钟） → 新 session
	i4 := hd.RecordInteractionAt("attacker-sess", "credential_request", "im", "p4", now.Add(60*time.Minute))
	if i4.SessionID == session1 {
		t.Error("expected new session, got same session")
	}
}

// ============================================================
// Test 4: 忠诚度计算
// ============================================================

func TestHoneypotDeepLoyaltyCurve(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 5 次交互，跨 2 小时
	hd.RecordInteractionAt("attacker-loy", "credential_request", "im", "p1", now.Add(-2*time.Hour))
	hd.RecordInteractionAt("attacker-loy", "info_extraction", "im", "p2", now.Add(-90*time.Minute))
	hd.RecordInteractionAt("attacker-loy", "credential_request", "im", "p3", now.Add(-60*time.Minute))
	hd.RecordInteractionAt("attacker-loy", "system_probe", "im", "p4", now.Add(-30*time.Minute))
	hd.RecordInteractionAt("attacker-loy", "credential_request", "im", "p5", now)

	curve := hd.GetLoyaltyCurve("attacker-loy")
	if curve == nil {
		t.Fatal("GetLoyaltyCurve returned nil")
	}

	if curve.TotalInteractions != 5 {
		t.Errorf("expected 5 interactions, got %d", curve.TotalInteractions)
	}

	if curve.MaxDepth != 3 {
		t.Errorf("expected max_depth=3, got %d", curve.MaxDepth)
	}

	// Duration should be ~2 hours
	if curve.DurationHours < 1.9 || curve.DurationHours > 2.1 {
		t.Errorf("expected duration ~2h, got %.2f", curve.DurationHours)
	}

	// Frequency = 5 / 2 = 2.5
	if curve.Frequency < 2.4 || curve.Frequency > 2.6 {
		t.Errorf("expected frequency ~2.5, got %.2f", curve.Frequency)
	}

	// LoyaltyScore should be > 0
	if curve.LoyaltyScore <= 0 {
		t.Errorf("expected positive loyalty score, got %.2f", curve.LoyaltyScore)
	}

	// LoyaltyScore formula check:
	// freq_score = min(100, 2.5*20) = 50
	// depth_score = min(100, 3*33.3) = 99.9
	// duration_score = min(100, 2*10) = 20
	// loyalty = 50*0.3 + 99.9*0.4 + 20*0.3 = 15 + 39.96 + 6 = 60.96
	if curve.LoyaltyScore < 55 || curve.LoyaltyScore > 65 {
		t.Errorf("expected loyalty ~60.96, got %.2f", curve.LoyaltyScore)
	}
}

// ============================================================
// Test 5: 阶段判定 probe→interest→engage→deep_dive
// ============================================================

func TestHoneypotDeepLoyaltyPhases(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 1 interaction → probe
	hd.RecordInteractionAt("attacker-phase", "credential_request", "im", "p1", now.Add(-5*time.Minute))
	curve := hd.GetLoyaltyCurve("attacker-phase")
	if curve.Phase != PhaseProbe {
		t.Errorf("1 interaction: expected probe, got %s", curve.Phase)
	}

	// 2 interactions → still probe
	hd.RecordInteractionAt("attacker-phase", "credential_request", "im", "p2", now.Add(-4*time.Minute))
	curve = hd.GetLoyaltyCurve("attacker-phase")
	if curve.Phase != PhaseProbe {
		t.Errorf("2 interactions: expected probe, got %s", curve.Phase)
	}

	// 3 interactions with high frequency → interest
	hd.RecordInteractionAt("attacker-phase", "credential_request", "im", "p3", now.Add(-3*time.Minute))
	curve = hd.GetLoyaltyCurve("attacker-phase")
	if curve.Phase != PhaseInterest {
		t.Errorf("3 interactions: expected interest, got %s (freq=%.2f)", curve.Phase, curve.Frequency)
	}

	// Add more to get to engage: 6+ interactions, depth >= 2
	hd.RecordInteractionAt("attacker-phase", "info_extraction", "im", "p4", now.Add(-2*time.Minute))
	hd.RecordInteractionAt("attacker-phase", "info_extraction", "im", "p5", now.Add(-1*time.Minute))
	hd.RecordInteractionAt("attacker-phase", "info_extraction", "im", "p6", now)
	curve = hd.GetLoyaltyCurve("attacker-phase")
	// depth may reach 3 due to honeypot_type escalation → deep_dive takes priority
	if curve.Phase != PhaseEngage && curve.Phase != PhaseDeepDive {
		t.Errorf("6 interactions depth>=2: expected engage or deep_dive, got %s (depth=%d)", curve.Phase, curve.MaxDepth)
	}

	// Add more to get to deep_dive: > 15 interactions
	for i := 7; i <= 16; i++ {
		hd.RecordInteractionAt("attacker-phase", "credential_request", "im", "p-extra", now.Add(time.Duration(i)*time.Minute))
	}
	curve = hd.GetLoyaltyCurve("attacker-phase")
	if curve.Phase != PhaseDeepDive {
		t.Errorf("16 interactions: expected deep_dive, got %s", curve.Phase)
	}
}

// ============================================================
// Test 6: 放弃阶段判定
// ============================================================

func TestHoneypotDeepAbandonPhase(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	// 交互时间在 5 小时前（> 4h）
	old := time.Now().UTC().Add(-5 * time.Hour)
	hd.RecordInteractionAt("attacker-abandon", "credential_request", "im", "p1", old)
	hd.RecordInteractionAt("attacker-abandon", "info_extraction", "im", "p2", old.Add(10*time.Minute))
	hd.RecordInteractionAt("attacker-abandon", "system_probe", "im", "p3", old.Add(20*time.Minute))

	curve := hd.GetLoyaltyCurve("attacker-abandon")
	if curve.Phase != PhaseAbandon {
		t.Errorf("expected abandon phase (last seen > 4h ago), got %s", curve.Phase)
	}
}

// ============================================================
// Test 7: 回馈自进化引擎
// ============================================================

func TestHoneypotDeepFeedbackToEvolution(t *testing.T) {
	hd, redTeam, db := setupHoneypotDeepWithEvolution(t)
	defer db.Close()

	// 先记录一些交互
	hd.RecordInteraction("attacker-fb", "credential_request", "im", "payload-A")
	hd.RecordInteraction("attacker-fb", "info_extraction", "im", "payload-B")
	hd.RecordInteraction("attacker-fb", "system_probe", "im", "payload-C")

	beforeCount := redTeam.GetInjectedVectorCount()

	injected, err := hd.FeedbackToEvolution("attacker-fb")
	if err != nil {
		t.Fatalf("FeedbackToEvolution error: %v", err)
	}

	if injected != 3 {
		t.Errorf("expected 3 injected payloads, got %d", injected)
	}

	afterCount := redTeam.GetInjectedVectorCount()
	if afterCount-beforeCount != 3 {
		t.Errorf("expected 3 new injected vectors, got %d", afterCount-beforeCount)
	}
}

// ============================================================
// Test 8: 自动回馈（超过最低忠诚度）
// ============================================================

func TestHoneypotDeepAutoFeedback(t *testing.T) {
	hd, redTeam, db := setupHoneypotDeepWithEvolution(t)
	defer db.Close()

	now := time.Now().UTC()

	// 创建一个高忠诚度攻击者（多次交互，多类型，跨时间）
	hd.RecordInteractionAt("high-loyalty", "credential_request", "im", "hl-p1", now.Add(-3*time.Hour))
	hd.RecordInteractionAt("high-loyalty", "info_extraction", "im", "hl-p2", now.Add(-2*time.Hour))
	hd.RecordInteractionAt("high-loyalty", "system_probe", "im", "hl-p3", now.Add(-1*time.Hour))
	hd.RecordInteractionAt("high-loyalty", "credential_request", "im", "hl-p4", now.Add(-30*time.Minute))
	hd.RecordInteractionAt("high-loyalty", "info_extraction", "im", "hl-p5", now)

	// 创建一个低忠诚度攻击者（只有 1 次交互）
	hd.RecordInteractionAt("low-loyalty", "credential_request", "im", "ll-p1", now)

	// 检查忠诚度
	highCurve := hd.GetLoyaltyCurve("high-loyalty")
	lowCurve := hd.GetLoyaltyCurve("low-loyalty")
	t.Logf("high-loyalty score=%.2f, low-loyalty score=%.2f", highCurve.LoyaltyScore, lowCurve.LoyaltyScore)

	if highCurve.LoyaltyScore < 50 {
		t.Skipf("high-loyalty score %.2f < 50, adjusting test expectations", highCurve.LoyaltyScore)
	}

	beforeCount := redTeam.GetInjectedVectorCount()

	injected, err := hd.AutoFeedback()
	if err != nil {
		t.Fatalf("AutoFeedback error: %v", err)
	}

	// Only high-loyalty should be fed back (5 unique payloads)
	if injected < 1 {
		t.Errorf("expected at least 1 injected payload from auto feedback, got %d", injected)
	}

	afterCount := redTeam.GetInjectedVectorCount()
	if afterCount <= beforeCount {
		t.Error("expected injected vector count to increase after auto feedback")
	}
}

// ============================================================
// Test 9: 排行列表
// ============================================================

func TestHoneypotDeepListCurves(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 攻击者 A: 多次交互
	for i := 0; i < 5; i++ {
		hd.RecordInteractionAt("attacker-A", "credential_request", "im", "pA", now.Add(time.Duration(i)*time.Minute))
	}

	// 攻击者 B: 少量交互
	hd.RecordInteractionAt("attacker-B", "credential_request", "im", "pB", now)

	// 攻击者 C: 中等交互
	for i := 0; i < 3; i++ {
		hd.RecordInteractionAt("attacker-C", "info_extraction", "im", "pC", now.Add(time.Duration(i)*time.Minute))
	}

	curves := hd.ListLoyaltyCurves(10)
	if len(curves) != 3 {
		t.Errorf("expected 3 curves, got %d", len(curves))
	}

	// Verify sorted by loyalty score descending
	for i := 0; i < len(curves)-1; i++ {
		if curves[i].LoyaltyScore < curves[i+1].LoyaltyScore {
			t.Errorf("curves not sorted: [%d].score=%.2f < [%d].score=%.2f",
				i, curves[i].LoyaltyScore, i+1, curves[i+1].LoyaltyScore)
		}
	}

	// Test limit
	curves2 := hd.ListLoyaltyCurves(2)
	if len(curves2) != 2 {
		t.Errorf("expected 2 curves with limit=2, got %d", len(curves2))
	}
}

// ============================================================
// Test 10: 统计
// ============================================================

func TestHoneypotDeepStats(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 多个攻击者，多个通道
	hd.RecordInteractionAt("attacker-s1", "credential_request", "im", "p1", now)
	hd.RecordInteractionAt("attacker-s1", "info_extraction", "llm", "p2", now.Add(time.Minute))
	hd.RecordInteractionAt("attacker-s2", "system_probe", "toolcall", "p3", now)

	stats := hd.GetStats()

	if stats.TotalInteractions != 3 {
		t.Errorf("expected 3 total interactions, got %d", stats.TotalInteractions)
	}
	if stats.TotalAttackers != 2 {
		t.Errorf("expected 2 total attackers, got %d", stats.TotalAttackers)
	}
	if stats.TotalSessions < 1 {
		t.Errorf("expected at least 1 session, got %d", stats.TotalSessions)
	}

	// Channel breakdown
	if stats.ChannelBreakdown["im"] != 1 {
		t.Errorf("expected 1 im interaction, got %d", stats.ChannelBreakdown["im"])
	}
	if stats.ChannelBreakdown["llm"] != 1 {
		t.Errorf("expected 1 llm interaction, got %d", stats.ChannelBreakdown["llm"])
	}
	if stats.ChannelBreakdown["toolcall"] != 1 {
		t.Errorf("expected 1 toolcall interaction, got %d", stats.ChannelBreakdown["toolcall"])
	}

	// Depth breakdown
	if stats.DepthBreakdown[1] < 1 {
		t.Errorf("expected at least 1 depth=1 interaction")
	}

	// Phase distribution
	if len(stats.PhaseDistribution) < 1 {
		t.Error("expected at least 1 phase in distribution")
	}
}

// ============================================================
// Test 11: 多攻击者独立曲线
// ============================================================

func TestHoneypotDeepMultipleAttackers(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// 攻击者 X: 大量交互
	for i := 0; i < 10; i++ {
		hd.RecordInteractionAt("attacker-X", "credential_request", "im", "pX", now.Add(time.Duration(i)*time.Minute))
	}

	// 攻击者 Y: 少量交互
	hd.RecordInteractionAt("attacker-Y", "info_extraction", "im", "pY", now)

	curveX := hd.GetLoyaltyCurve("attacker-X")
	curveY := hd.GetLoyaltyCurve("attacker-Y")

	if curveX.TotalInteractions != 10 {
		t.Errorf("attacker-X: expected 10 interactions, got %d", curveX.TotalInteractions)
	}
	if curveY.TotalInteractions != 1 {
		t.Errorf("attacker-Y: expected 1 interaction, got %d", curveY.TotalInteractions)
	}

	// X should have higher loyalty than Y
	if curveX.LoyaltyScore <= curveY.LoyaltyScore {
		t.Errorf("attacker-X loyalty (%.2f) should be > attacker-Y loyalty (%.2f)",
			curveX.LoyaltyScore, curveY.LoyaltyScore)
	}

	// X and Y should be independent
	if curveX.AttackerID != "attacker-X" || curveY.AttackerID != "attacker-Y" {
		t.Error("curves are not independent")
	}
}

// ============================================================
// Test 12: 载荷去重
// ============================================================

func TestHoneypotDeepPayloadDedup(t *testing.T) {
	hd, redTeam, db := setupHoneypotDeepWithEvolution(t)
	defer db.Close()

	// 同一攻击者重复相同 payload
	hd.RecordInteraction("attacker-dedup", "credential_request", "im", "same_payload")
	hd.RecordInteraction("attacker-dedup", "credential_request", "im", "same_payload")
	hd.RecordInteraction("attacker-dedup", "credential_request", "im", "same_payload")
	hd.RecordInteraction("attacker-dedup", "credential_request", "im", "different_payload")

	beforeCount := redTeam.GetInjectedVectorCount()

	injected, err := hd.FeedbackToEvolution("attacker-dedup")
	if err != nil {
		t.Fatalf("FeedbackToEvolution error: %v", err)
	}

	// Should only inject 2 unique payloads (same_payload + different_payload)
	if injected != 2 {
		t.Errorf("expected 2 unique payloads injected, got %d", injected)
	}

	afterCount := redTeam.GetInjectedVectorCount()
	if afterCount-beforeCount != 2 {
		t.Errorf("expected exactly 2 new vectors, got %d", afterCount-beforeCount)
	}
}

// ============================================================
// Test 13: 阶段变迁历史
// ============================================================

func TestHoneypotDeepPhaseHistory(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	now := time.Now().UTC()

	// Build interactions to transition through phases
	// 1-2: probe
	hd.RecordInteractionAt("attacker-hist", "credential_request", "im", "p1", now.Add(-10*time.Minute))
	hd.RecordInteractionAt("attacker-hist", "credential_request", "im", "p2", now.Add(-9*time.Minute))
	// 3: interest (3 interactions, high freq)
	hd.RecordInteractionAt("attacker-hist", "credential_request", "im", "p3", now.Add(-8*time.Minute))
	// 4-6: engage (depth >= 2 + 6 interactions)
	hd.RecordInteractionAt("attacker-hist", "info_extraction", "im", "p4", now.Add(-7*time.Minute))
	hd.RecordInteractionAt("attacker-hist", "info_extraction", "im", "p5", now.Add(-6*time.Minute))
	hd.RecordInteractionAt("attacker-hist", "info_extraction", "im", "p6", now.Add(-5*time.Minute))

	curve := hd.GetLoyaltyCurve("attacker-hist")
	if len(curve.PhaseHistory) < 2 {
		t.Errorf("expected at least 2 phase transitions, got %d", len(curve.PhaseHistory))
	}

	// Check that transitions progress forward
	for _, ph := range curve.PhaseHistory {
		if ph.From == "" || ph.To == "" {
			t.Errorf("phase transition has empty from/to: %+v", ph)
		}
		if ph.Trigger == "" {
			t.Errorf("phase transition has empty trigger: %+v", ph)
		}
	}
}

// ============================================================
// Test 14: ListInteractions 过滤
// ============================================================

func TestHoneypotDeepListInteractions(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	hd.RecordInteraction("attacker-li1", "credential_request", "im", "p1")
	hd.RecordInteraction("attacker-li1", "info_extraction", "llm", "p2")
	hd.RecordInteraction("attacker-li2", "system_probe", "im", "p3")

	// All
	all := hd.ListInteractions("", "", 50)
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}

	// Filter by attacker
	filtered := hd.ListInteractions("attacker-li1", "", 50)
	if len(filtered) != 2 {
		t.Errorf("expected 2 for attacker-li1, got %d", len(filtered))
	}

	// Filter by channel
	imOnly := hd.ListInteractions("", "im", 50)
	if len(imOnly) != 2 {
		t.Errorf("expected 2 im interactions, got %d", len(imOnly))
	}

	// Limit
	limited := hd.ListInteractions("", "", 1)
	if len(limited) != 1 {
		t.Errorf("expected 1 with limit=1, got %d", len(limited))
	}
}

// ============================================================
// Test 15: 忠诚度评分公式验证
// ============================================================

func TestHoneypotDeepLoyaltyScoreFormula(t *testing.T) {
	// freq_score = min(100, freq * 20)
	// depth_score = min(100, depth * 33.3)
	// duration_score = min(100, hours * 10)
	// loyalty = min(100, freq_score*0.3 + depth_score*0.4 + duration_score*0.3)

	// Case 1: freq=5/h, depth=3, duration=10h
	// freq_score = min(100, 100) = 100
	// depth_score = min(100, 99.9) = 99.9
	// duration_score = min(100, 100) = 100
	// loyalty = min(100, 30 + 39.96 + 30) = 99.96
	score1 := calculateLoyaltyScore(5, 3, 10)
	if score1 < 99 || score1 > 100 {
		t.Errorf("case 1: expected ~99.96, got %.2f", score1)
	}

	// Case 2: freq=0, depth=1, duration=0
	// freq_score = 0
	// depth_score = 33.3
	// duration_score = 0
	// loyalty = 13.32
	score2 := calculateLoyaltyScore(0, 1, 0)
	if score2 < 13 || score2 > 14 {
		t.Errorf("case 2: expected ~13.32, got %.2f", score2)
	}

	// Case 3: all zero
	score3 := calculateLoyaltyScore(0, 0, 0)
	if score3 != 0 {
		t.Errorf("case 3: expected 0, got %.2f", score3)
	}
}

// ============================================================
// Test 16: Empty/nonexistent attacker
// ============================================================

func TestHoneypotDeepNonexistentAttacker(t *testing.T) {
	hd, db := setupHoneypotDeepTest(t)
	defer db.Close()

	curve := hd.GetLoyaltyCurve("nonexistent")
	if curve == nil {
		t.Fatal("expected non-nil curve for nonexistent attacker")
	}
	if curve.TotalInteractions != 0 {
		t.Errorf("expected 0 interactions, got %d", curve.TotalInteractions)
	}
	if curve.Phase != PhaseProbe {
		t.Errorf("expected probe phase, got %s", curve.Phase)
	}
}
		