// gateway_monitor_test.go — OpenClaw 直接扫描功能测试
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestScanOpenClawConfig 测试从 openclaw.json 提取 agents 列表
func TestScanOpenClawConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")

	config := map[string]interface{}{
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": map[string]string{"primary": "claude-3"},
			},
			"list": []map[string]interface{}{
				{"id": "agent-alice", "workspace": "/home/alice/ws"},
				{"id": "agent-bob", "workspace": "/home/bob/ws"},
				{"id": "agent-charlie", "workspace": "/home/charlie/ws", "agentDir": "/etc/agents/charlie"},
			},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	agents, err := scanOpenClawConfig(configPath)
	if err != nil {
		t.Fatalf("scanOpenClawConfig 失败: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}
	if agents[0].ID != "agent-alice" {
		t.Errorf("expected agent-alice, got %s", agents[0].ID)
	}
	if agents[2].AgentDir != "/etc/agents/charlie" {
		t.Errorf("expected agentDir, got %s", agents[2].AgentDir)
	}
}

// TestScanOpenClawConfig_FileNotFound 测试文件不存在
func TestScanOpenClawConfig_FileNotFound(t *testing.T) {
	_, err := scanOpenClawConfig("/nonexistent/openclaw.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// TestScanOpenClawConfig_InvalidJSON 测试无效 JSON
func TestScanOpenClawConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")
	os.WriteFile(configPath, []byte("not json"), 0644)

	_, err := scanOpenClawConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestScanOpenClawConfig_EmptyAgents 测试没有 agents 的配置
func TestScanOpenClawConfig_EmptyAgents(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")
	os.WriteFile(configPath, []byte(`{"agents":{"list":[]}}`), 0644)

	agents, err := scanOpenClawConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(agents))
	}
}

// TestScanOpenClawSessions 测试扫描 sessions 目录
func TestScanOpenClawSessions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")
	os.WriteFile(configPath, []byte(`{}`), 0644)

	// 创建模拟 agents 目录结构
	agentsDir := filepath.Join(tmpDir, "agents")

	// agent-alice: 3 sessions
	aliceSess := filepath.Join(agentsDir, "agent-alice", "sessions")
	os.MkdirAll(aliceSess, 0755)
	os.WriteFile(filepath.Join(aliceSess, "sess-001.jsonl"), []byte(`{"role":"user"}`), 0644)
	os.WriteFile(filepath.Join(aliceSess, "sess-002.jsonl"), []byte(`{"role":"assistant"}`), 0644)
	os.WriteFile(filepath.Join(aliceSess, "sess-003.jsonl"), []byte(`{"role":"user"}\n{"role":"assistant"}`), 0644)

	// agent-bob: 1 session
	bobSess := filepath.Join(agentsDir, "agent-bob", "sessions")
	os.MkdirAll(bobSess, 0755)
	os.WriteFile(filepath.Join(bobSess, "sess-100.jsonl"), []byte(`{}`), 0644)

	// agent-charlie: no sessions dir
	os.MkdirAll(filepath.Join(agentsDir, "agent-charlie"), 0755)

	// agent-dave: sessions dir with non-jsonl files
	daveSess := filepath.Join(agentsDir, "agent-dave", "sessions")
	os.MkdirAll(daveSess, 0755)
	os.WriteFile(filepath.Join(daveSess, "readme.txt"), []byte("not a session"), 0644)

	sessions, err := scanOpenClawSessions(configPath)
	if err != nil {
		t.Fatalf("scanOpenClawSessions 失败: %v", err)
	}
	if len(sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sessions))
	}

	// 验证 agent 归属
	agentCounts := make(map[string]int)
	for _, s := range sessions {
		agentCounts[s.AgentID]++
		if s.SessionID == "" {
			t.Error("empty session ID")
		}
		if s.LastModifiedAt == 0 {
			t.Error("zero last modified time")
		}
	}
	if agentCounts["agent-alice"] != 3 {
		t.Errorf("expected 3 alice sessions, got %d", agentCounts["agent-alice"])
	}
	if agentCounts["agent-bob"] != 1 {
		t.Errorf("expected 1 bob session, got %d", agentCounts["agent-bob"])
	}
	if agentCounts["agent-charlie"] != 0 {
		t.Errorf("expected 0 charlie sessions, got %d", agentCounts["agent-charlie"])
	}
}

// TestScanOpenClawSessions_SortOrder 测试按最后修改时间降序排列
func TestScanOpenClawSessions_SortOrder(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")
	os.WriteFile(configPath, []byte(`{}`), 0644)

	sessDir := filepath.Join(tmpDir, "agents", "agent-x", "sessions")
	os.MkdirAll(sessDir, 0755)

	// 创建文件并设置不同的修改时间
	old := filepath.Join(sessDir, "old.jsonl")
	os.WriteFile(old, []byte(`{}`), 0644)
	os.Chtimes(old, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour))

	mid := filepath.Join(sessDir, "mid.jsonl")
	os.WriteFile(mid, []byte(`{}`), 0644)
	os.Chtimes(mid, time.Now().Add(-1*time.Hour), time.Now().Add(-1*time.Hour))

	recent := filepath.Join(sessDir, "recent.jsonl")
	os.WriteFile(recent, []byte(`{}`), 0644)
	// recent 保持当前时间

	sessions, err := scanOpenClawSessions(configPath)
	if err != nil {
		t.Fatalf("scanOpenClawSessions 失败: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// 验证降序排列
	if sessions[0].SessionID != "recent" {
		t.Errorf("expected newest first, got %s", sessions[0].SessionID)
	}
	if sessions[2].SessionID != "old" {
		t.Errorf("expected oldest last, got %s", sessions[2].SessionID)
	}
}

// TestDirectScanOpenClaw_Integration 集成测试
func TestDirectScanOpenClaw_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "openclaw.json")

	config := map[string]interface{}{
		"agents": map[string]interface{}{
			"list": []map[string]interface{}{
				{"id": "Agent-Alpha", "workspace": filepath.Join(tmpDir, "ws-alpha")},
				{"id": "Agent-Beta", "workspace": filepath.Join(tmpDir, "ws-beta")},
			},
		},
	}
	data, _ := json.Marshal(config)
	os.WriteFile(configPath, data, 0644)

	// 创建 sessions（注意 OpenClaw agents 目录用小写 agent ID）
	alphaSess := filepath.Join(tmpDir, "agents", "agent-alpha", "sessions")
	os.MkdirAll(alphaSess, 0755)
	os.WriteFile(filepath.Join(alphaSess, "s1.jsonl"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(alphaSess, "s2.jsonl"), []byte(`{}`), 0644)

	betaSess := filepath.Join(tmpDir, "agents", "agent-beta", "sessions")
	os.MkdirAll(betaSess, 0755)
	os.WriteFile(filepath.Join(betaSess, "s3.jsonl"), []byte(`{}`), 0644)

	result := directScanOpenClaw(configPath)
	if result.Error != "" {
		t.Fatalf("directScan error: %s", result.Error)
	}
	if result.Source != "direct_scan" {
		t.Errorf("expected source=direct_scan, got %s", result.Source)
	}
	if len(result.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(result.Agents))
	}
	if len(result.Sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(result.Sessions))
	}

	// 测试 directScanToOverviewData
	agents, sessions := directScanToOverviewData(result)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents in overview, got %d", len(agents))
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions in overview, got %d", len(sessions))
	}

	// 验证 session_count 回填到 agents
	for _, a := range agents {
		id := a["id"].(string)
		sc := a["session_count"].(int)
		switch id {
		case "Agent-Alpha":
			if sc != 2 {
				t.Errorf("Agent-Alpha session_count: expected 2, got %d", sc)
			}
		case "Agent-Beta":
			if sc != 1 {
				t.Errorf("Agent-Beta session_count: expected 1, got %d", sc)
			}
		}
	}
}

// TestDirectScanOpenClaw_RealSystem 如果运行在有 OpenClaw 的系统上，测试真实数据
func TestDirectScanOpenClaw_RealSystem(t *testing.T) {
	configPath := "/root/.openclaw/openclaw.json"
	if _, err := os.Stat(configPath); err != nil {
		t.Skip("no real OpenClaw config found, skipping")
	}

	result := directScanOpenClaw(configPath)
	if result.Error != "" {
		t.Fatalf("real system scan error: %s", result.Error)
	}

	t.Logf("Real system scan: %d agents, %d sessions", len(result.Agents), len(result.Sessions))
	if len(result.Agents) == 0 {
		t.Error("expected at least 1 agent on real system")
	}

	// 验证 agents 有 ID
	for _, a := range result.Agents {
		if a.ID == "" {
			t.Error("agent with empty ID")
		}
	}

	// 验证 sessions 有必要字段
	for _, s := range result.Sessions {
		if s.AgentID == "" {
			t.Error("session with empty AgentID")
		}
		if s.SessionID == "" {
			t.Error("session with empty SessionID")
		}
	}
}
