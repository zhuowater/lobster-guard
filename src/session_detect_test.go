// session_detect_test.go — SessionDetector 测试（v5.1）
package main

import (
	"testing"
	"time"
)

func TestSessionDetector_Basic(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        5,
		DecayRate:     1,
	})

	// First message: pass
	action, score := sd.RecordAndEvaluate("user1", "hello world", "pass", "")
	if action != "pass" {
		t.Errorf("expected pass, got %s", action)
	}
	if score != 0 {
		t.Errorf("expected score 0, got %f", score)
	}

	// Warn message
	action, score = sd.RecordAndEvaluate("user1", "suspicious content", "warn", "rule1")
	if action != "warn" {
		t.Errorf("expected warn, got %s", action)
	}
	if score != 1 {
		t.Errorf("expected score 1, got %f", score)
	}
}

func TestSessionDetector_RiskEscalation(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 5,
		Window:        20,
		DecayRate:     0, // no decay for test
	})

	// Accumulate warns
	for i := 0; i < 5; i++ {
		sd.RecordAndEvaluate("user1", "warn text", "warn", "rule1")
	}

	// Now a warn should be escalated to block
	action, score := sd.RecordAndEvaluate("user1", "another warn", "warn", "rule1")
	if action != "block" {
		t.Errorf("expected block (escalation), got %s", action)
	}
	if score < 5 {
		t.Errorf("expected score >= 5, got %f", score)
	}
}

func TestSessionDetector_BlockAddsMorePoints(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     0,
	})

	// Block adds 5 points
	action, score := sd.RecordAndEvaluate("user1", "bad", "block", "rule1")
	if action != "block" {
		t.Errorf("expected block, got %s", action)
	}
	if score != 5 {
		t.Errorf("expected score 5, got %f", score)
	}

	// Another block → 10 points
	action, score = sd.RecordAndEvaluate("user1", "bad again", "block", "rule1")
	if score < 9.9 || score > 10.1 {
		t.Errorf("expected score ~10, got %f", score)
	}
}

func TestSessionDetector_EmptySenderID(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     1,
	})

	action, score := sd.RecordAndEvaluate("", "hello", "warn", "rule1")
	if action != "warn" {
		t.Errorf("expected warn, got %s", action)
	}
	if score != 0 {
		t.Errorf("expected score 0 for empty sender, got %f", score)
	}
}

func TestSessionDetector_Window(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 100,
		Window:        3,
		DecayRate:     0,
	})

	// Add 5 messages, only last 3 should be kept
	for i := 0; i < 5; i++ {
		sd.RecordAndEvaluate("user1", "message", "pass", "")
	}

	sd.mu.RLock()
	info := sd.sessions["user1"]
	msgCount := len(info.Messages)
	sd.mu.RUnlock()

	if msgCount != 3 {
		t.Errorf("expected 3 messages (window), got %d", msgCount)
	}
}

func TestSessionDetector_GetRiskScore(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     0,
	})

	sd.RecordAndEvaluate("user1", "warn", "warn", "rule1")
	score := sd.GetRiskScore("user1")
	if score < 0.9 || score > 1.1 {
		t.Errorf("expected score ~1, got %f", score)
	}

	// Unknown user
	score = sd.GetRiskScore("unknown")
	if score != 0 {
		t.Errorf("expected score 0 for unknown, got %f", score)
	}
}

func TestSessionDetector_ListHighRiskSessions(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 5,
		Window:        20,
		DecayRate:     0,
	})

	// user1: high risk
	for i := 0; i < 3; i++ {
		sd.RecordAndEvaluate("user1", "bad", "warn", "rule1")
	}
	// user2: low risk
	sd.RecordAndEvaluate("user2", "ok", "pass", "")

	sessions := sd.ListHighRiskSessions()
	if len(sessions) != 1 {
		t.Errorf("expected 1 high risk session, got %d", len(sessions))
	}
	if len(sessions) > 0 && sessions[0].SenderID != "user1" {
		t.Errorf("expected user1, got %s", sessions[0].SenderID)
	}
}

func TestSessionDetector_ResetRisk(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     0,
	})

	sd.RecordAndEvaluate("user1", "bad", "block", "rule1")
	if sd.GetRiskScore("user1") == 0 {
		t.Error("expected non-zero score")
	}

	ok := sd.ResetRisk("user1")
	if !ok {
		t.Error("expected reset to succeed")
	}
	if sd.GetRiskScore("user1") != 0 {
		t.Error("expected zero score after reset")
	}

	// Reset unknown user
	ok = sd.ResetRisk("unknown")
	if ok {
		t.Error("expected reset to fail for unknown user")
	}
}

func TestSessionDetector_Cleanup(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     0,
	})

	sd.RecordAndEvaluate("user1", "hello", "pass", "")
	// Manually set old timestamp
	sd.mu.Lock()
	sd.sessions["user1"].LastUpdate = time.Now().Add(-2 * time.Hour)
	sd.mu.Unlock()

	deleted := sd.Cleanup(1 * time.Hour)
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
	if sd.SessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", sd.SessionCount())
	}
}

func TestSessionStage_Disabled(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled: false,
	})
	stage := NewSessionStage(sd)
	ctx := &DetectContext{Text: "test", SenderID: "user1"}
	result := stage.Detect(ctx)
	if result.Action != "pass" {
		t.Errorf("expected pass when disabled, got %s", result.Action)
	}
}

func TestSessionStage_Escalation(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 3,
		Window:        20,
		DecayRate:     0,
	})
	stage := NewSessionStage(sd)

	// Build up risk
	for i := 0; i < 3; i++ {
		sd.RecordAndEvaluate("user1", "bad", "warn", "rule1")
	}

	// Now simulate a pipeline where previous stage returned warn
	ctx := &DetectContext{
		Text:     "another suspicious message",
		SenderID: "user1",
		PreviousResults: []*StageResult{
			{Action: "warn", RuleName: "some_rule"},
		},
	}
	result := stage.Detect(ctx)
	if result.Action != "block" {
		t.Errorf("expected block (escalation), got %s", result.Action)
	}
	if result.RuleName != "session_risk_upgrade" {
		t.Errorf("expected session_risk_upgrade, got %s", result.RuleName)
	}
}

func TestSessionDetector_LongPreview(t *testing.T) {
	sd := NewSessionDetector(SessionDetectorConfig{
		Enabled:       true,
		RiskThreshold: 10,
		Window:        20,
		DecayRate:     0,
	})

	// Long text should be truncated to 100 runes
	longText := ""
	for i := 0; i < 200; i++ {
		longText += "字"
	}
	sd.RecordAndEvaluate("user1", longText, "pass", "")

	sd.mu.RLock()
	info := sd.sessions["user1"]
	preview := info.Messages[0].Preview
	sd.mu.RUnlock()

	if len([]rune(preview)) != 100 {
		t.Errorf("expected preview 100 runes, got %d", len([]rune(preview)))
	}
}
