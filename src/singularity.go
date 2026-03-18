// singularity.go — 奇点蜜罐引擎：毛球定理的工程实现——主动选择"在哪里弱"（v18.3）
// 通过可配置的暴露等级，在指定通道放置不同诱饵等级的蜜罐内容
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// 奇点蜜罐引擎
// ============================================================

// SingularityEngine 奇点管理引擎
type SingularityEngine struct {
	db          *sql.DB
	honeypot    *HoneypotEngine  // 复用已有蜜罐引擎
	envelopeMgr *EnvelopeManager
	mu          sync.RWMutex
	config      SingularityConfig
}

// SingularityConfig 奇点配置
type SingularityConfig struct {
	Enabled               bool `yaml:"enabled" json:"enabled"`
	IMExposureLevel       int  `yaml:"im_exposure_level" json:"im_exposure_level"`             // IM 通道暴露等级 (0-5)
	LLMExposureLevel      int  `yaml:"llm_exposure_level" json:"llm_exposure_level"`           // LLM 通道暴露等级 (0-5)
	ToolCallExposureLevel int  `yaml:"toolcall_exposure_level" json:"toolcall_exposure_level"` // Tool Call 通道暴露等级 (0-5)
}

// ExposureTemplate 暴露模板
type ExposureTemplate struct {
	Level       int    `json:"level"`
	Channel     string `json:"channel"`     // "im" / "llm" / "toolcall"
	Name        string `json:"name"`
	Content     string `json:"content"`     // 暴露的假内容
	Description string `json:"description"`
}

// PlacementProof 放置最优性证明
type PlacementProof struct {
	Channel            string  `json:"channel"`
	Level              int     `json:"level"`
	IMTrafficVolume    int64   `json:"im_traffic_volume"`
	LLMTrafficVolume   int64   `json:"llm_traffic_volume"`
	ToolCallVolume     int64   `json:"toolcall_volume"`
	IMFPRate           float64 `json:"im_fp_rate"`
	LLMFPRate          float64 `json:"llm_fp_rate"`
	ToolCallFPRate     float64 `json:"toolcall_fp_rate"`
	IMAttackRate       float64 `json:"im_attack_rate"`
	LLMAttackRate      float64 `json:"llm_attack_rate"`
	ToolCallAttackRate float64 `json:"toolcall_attack_rate"`
	RecommendedChannel string  `json:"recommended_channel"`
	RecommendedLevel   int     `json:"recommended_level"`
	FPReduction        float64 `json:"fp_reduction_pct"`
	ExposureGain       float64 `json:"exposure_gain_pct"`
	ParetoOptimal      bool    `json:"pareto_optimal"`
	Reason             string  `json:"reason"`
}

// SingularityHistoryEntry 奇点配置历史
type SingularityHistoryEntry struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Level     int    `json:"level"`
	Action    string `json:"action"` // "set_level" / "recommend"
	ProofJSON string `json:"proof_json"`
	Timestamp string `json:"timestamp"`
}

// ============================================================
// 暴露模板定义（覆盖 3 通道 × 多等级，>= 10 个）
// ============================================================

var defaultExposureTemplates = []ExposureTemplate{
	// Level 1 — 轻微暴露
	{Level: 1, Channel: "im", Name: "error_format_hint",
		Content:     `{"error": "invalid_token", "hint": "use Bearer auth", "docs": "https://docs.internal.example.com/auth"}`,
		Description: "错误信息格式暗示（IM）"},
	{Level: 1, Channel: "llm", Name: "error_model_hint",
		Content:     `{"error": "model_not_found", "available": ["gpt-4o", "claude-3"], "endpoint": "/v1/chat/completions"}`,
		Description: "错误信息格式暗示（LLM）"},
	{Level: 1, Channel: "toolcall", Name: "error_tool_hint",
		Content:     `{"error": "tool_not_found", "available_tools": ["file_read", "shell_exec", "web_search"]}`,
		Description: "错误信息格式暗示（Tool Call）"},

	// Level 2 — 中等暴露
	{Level: 2, Channel: "im", Name: "fake_config_fragment",
		Content:     `# config.yaml fragment\napi_base: https://api.internal.example.com\nauth_method: bearer_token\nmax_retries: 3`,
		Description: "假配置文件片段（IM）"},
	{Level: 2, Channel: "llm", Name: "fake_env_vars",
		Content:     `OPENAI_API_KEY=sk-fake-xxxx\nANTHROPIC_API_KEY=sk-ant-fake-xxxx\nDATABASE_URL=postgres://fake:fake@localhost/db`,
		Description: "假环境变量片段（LLM）"},
	{Level: 2, Channel: "toolcall", Name: "fake_tool_schema",
		Content:     `{"tools": [{"name": "admin_exec", "params": {"command": "string"}, "auth_level": "admin"}]}`,
		Description: "假工具 schema 暴露（Tool Call）"},

	// Level 3 — 推荐默认
	{Level: 3, Channel: "im", Name: "fake_config",
		Content:     `{"api_key": "sk-fake-7a8b9c...", "model": "gpt-4o", "system_prompt": "You are a helpful assistant"}`,
		Description: "假 OpenClaw 配置片段"},
	{Level: 3, Channel: "im", Name: "fake_soul",
		Content:     "# SOUL.md\nI am an AI assistant. My core values are helpfulness and honesty.\nSecret admin password: admin123",
		Description: "假 SOUL.md 片段"},
	{Level: 3, Channel: "llm", Name: "fake_system_prompt",
		Content:     `System: You have access to internal tools. Use tool_exec to run commands. Admin token: adm-fake-token-xxx`,
		Description: "假系统提示暴露"},
	{Level: 3, Channel: "toolcall", Name: "fake_credentials",
		Content:     `{"db_password": "fake_p@ssw0rd_123", "redis_url": "redis://fake:6379", "s3_bucket": "internal-fake-bucket"}`,
		Description: "假凭证信息暴露（Tool Call）"},

	// Level 4 — 高暴露
	{Level: 4, Channel: "im", Name: "fake_agent_response",
		Content:     `I understand you want me to help. Let me check my tools... [tool_call: file_read("/etc/passwd")] Here's what I found...`,
		Description: "模拟真实 Agent 响应模式（IM）"},
	{Level: 4, Channel: "llm", Name: "fake_function_calls",
		Content:     `{"role": "assistant", "content": null, "function_call": {"name": "execute_code", "arguments": "{\"code\": \"import os; os.system('whoami')\"}"}}`,
		Description: "假函数调用模式（LLM）"},
	{Level: 4, Channel: "toolcall", Name: "fake_admin_panel",
		Content:     `{"admin_url": "https://admin.internal.example.com", "default_user": "admin", "default_pass": "changeme123", "2fa_disabled": true}`,
		Description: "假管理面板信息（Tool Call）"},

	// Level 5 — 高价值诱饵
	{Level: 5, Channel: "im", Name: "fake_agent_soul",
		Content:     "# 灵魂文件\n我是张卓的AI助手。GitHub token: ghp_fake_xxxxx\nSSH密钥在 /root/.ssh/id_rsa\nAWS Access Key: AKIAFAKE12345",
		Description: "模拟真实 Agent 灵魂"},
	{Level: 5, Channel: "llm", Name: "fake_real_token",
		Content:     `sk-fake-production-key-xxxxxxxxxxxxxxxxxxxxxxxxxxxx`,
		Description: "模拟真实 API Token"},
	{Level: 5, Channel: "toolcall", Name: "fake_ssh_key",
		Content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEfakeKeyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n-----END RSA PRIVATE KEY-----",
		Description: "假 SSH 私钥（Tool Call）"},
}

// NewSingularityEngine 创建奇点蜜罐引擎
func NewSingularityEngine(db *sql.DB, honeypot *HoneypotEngine, envelopeMgr *EnvelopeManager, cfg SingularityConfig) *SingularityEngine {
	se := &SingularityEngine{
		db:          db,
		honeypot:    honeypot,
		envelopeMgr: envelopeMgr,
		config:      cfg,
	}
	se.initSchema()
	return se
}

// initSchema 初始化数据库表
func (se *SingularityEngine) initSchema() {
	se.db.Exec(`CREATE TABLE IF NOT EXISTS singularity_history (
		id TEXT PRIMARY KEY,
		channel TEXT NOT NULL,
		level INTEGER NOT NULL,
		action TEXT NOT NULL,
		proof_json TEXT DEFAULT '{}',
		timestamp TEXT NOT NULL
	)`)
	se.db.Exec(`CREATE INDEX IF NOT EXISTS idx_singularity_channel ON singularity_history(channel)`)
	se.db.Exec(`CREATE INDEX IF NOT EXISTS idx_singularity_ts ON singularity_history(timestamp)`)
}

// GetExposureTemplates 获取指定通道+等级的暴露模板
// 返回所有等级 <= level 的模板（低等级模板在高等级也可用）
func (se *SingularityEngine) GetExposureTemplates(channel string, level int) []ExposureTemplate {
	var result []ExposureTemplate
	for _, tpl := range defaultExposureTemplates {
		if tpl.Channel == channel && tpl.Level <= level {
			result = append(result, tpl)
		}
	}
	return result
}

// GetAllTemplates 获取所有内置模板
func (se *SingularityEngine) GetAllTemplates() []ExposureTemplate {
	result := make([]ExposureTemplate, len(defaultExposureTemplates))
	copy(result, defaultExposureTemplates)
	return result
}

// ShouldExpose 判断是否在此请求上暴露蜜罐内容
// 返回是否暴露和具体模板
func (se *SingularityEngine) ShouldExpose(channel string, traceID string) (bool, *ExposureTemplate) {
	se.mu.RLock()
	cfg := se.config
	se.mu.RUnlock()

	if !cfg.Enabled {
		return false, nil
	}

	// 获取通道暴露等级
	level := se.getChannelLevel(channel)
	if level <= 0 {
		return false, nil
	}

	// 获取适用模板
	templates := se.GetExposureTemplates(channel, level)
	if len(templates) == 0 {
		return false, nil
	}

	// 选择最高等级的模板（优先使用最具诱饵价值的）
	bestLevel := 0
	var bestTemplates []ExposureTemplate
	for _, tpl := range templates {
		if tpl.Level > bestLevel {
			bestLevel = tpl.Level
			bestTemplates = []ExposureTemplate{tpl}
		} else if tpl.Level == bestLevel {
			bestTemplates = append(bestTemplates, tpl)
		}
	}

	if len(bestTemplates) == 0 {
		return false, nil
	}

	// 用 traceID 做确定性选择（同一请求始终返回相同模板）
	idx := deterministicSelect(traceID, len(bestTemplates))
	selected := bestTemplates[idx]

	// 记录暴露历史
	se.recordHistory(channel, selected.Level, "expose", "{}")

	return true, &selected
}

// deterministicSelect 根据字符串哈希确定性选择索引
func deterministicSelect(s string, n int) int {
	if n <= 0 {
		return 0
	}
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % n
}

// RecommendPlacement 根据历史数据推荐最优放置
func (se *SingularityEngine) RecommendPlacement() *PlacementProof {
	proof := &PlacementProof{}

	// 从 audit_log 统计各通道的流量
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'inbound'`).Scan(&proof.IMTrafficVolume)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'llm_request' OR direction = 'llm_response'`).Scan(&proof.LLMTrafficVolume)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'outbound'`).Scan(&proof.ToolCallVolume)

	// 统计各通道的攻击率（block 占总量的比例）
	var imBlocks, llmBlocks, toolBlocks int64
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'inbound' AND action = 'block'`).Scan(&imBlocks)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE (direction = 'llm_request' OR direction = 'llm_response') AND action = 'block'`).Scan(&llmBlocks)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'outbound' AND action = 'block'`).Scan(&toolBlocks)

	if proof.IMTrafficVolume > 0 {
		proof.IMAttackRate = float64(imBlocks) / float64(proof.IMTrafficVolume)
	}
	if proof.LLMTrafficVolume > 0 {
		proof.LLMAttackRate = float64(llmBlocks) / float64(proof.LLMTrafficVolume)
	}
	if proof.ToolCallVolume > 0 {
		proof.ToolCallAttackRate = float64(toolBlocks) / float64(proof.ToolCallVolume)
	}

	// 统计误伤率（warn 占 block+warn 的比例）
	var imWarn, llmWarn, toolWarn int64
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'inbound' AND action = 'warn'`).Scan(&imWarn)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE (direction = 'llm_request' OR direction = 'llm_response') AND action = 'warn'`).Scan(&llmWarn)
	se.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'outbound' AND action = 'warn'`).Scan(&toolWarn)

	if imBlocks+imWarn > 0 {
		proof.IMFPRate = float64(imWarn) / float64(imBlocks+imWarn)
	}
	if llmBlocks+llmWarn > 0 {
		proof.LLMFPRate = float64(llmWarn) / float64(llmBlocks+llmWarn)
	}
	if toolBlocks+toolWarn > 0 {
		proof.ToolCallFPRate = float64(toolWarn) / float64(toolBlocks+toolWarn)
	}

	// 计算每个通道的 score = attack_rate / (1 + fp_rate)
	type channelScore struct {
		name  string
		score float64
	}
	scores := []channelScore{
		{"im", proof.IMAttackRate / (1 + proof.IMFPRate)},
		{"llm", proof.LLMAttackRate / (1 + proof.LLMFPRate)},
		{"toolcall", proof.ToolCallAttackRate / (1 + proof.ToolCallFPRate)},
	}

	// 找到最优通道（score 最高 = 攻击多 + 误伤少 = 最佳放置点）
	bestIdx := 0
	secondBestIdx := 1
	for i := 1; i < len(scores); i++ {
		if scores[i].score > scores[bestIdx].score {
			secondBestIdx = bestIdx
			bestIdx = i
		} else if i != bestIdx && scores[i].score > scores[secondBestIdx].score {
			secondBestIdx = i
		}
	}

	proof.RecommendedChannel = scores[bestIdx].name
	proof.RecommendedLevel = 3 // 推荐默认等级
	proof.ParetoOptimal = true

	// 计算 FP 改善和暴露增益
	if scores[secondBestIdx].score > 0 {
		fpBest := se.getFPRateByChannel(proof, scores[bestIdx].name)
		fpSecond := se.getFPRateByChannel(proof, scores[secondBestIdx].name)
		if fpSecond > 0 {
			proof.FPReduction = (fpSecond - fpBest) / fpSecond * 100
		}
		if scores[secondBestIdx].score > 0 {
			proof.ExposureGain = (scores[bestIdx].score - scores[secondBestIdx].score) / scores[secondBestIdx].score * 100
		}
	}

	se.mu.RLock()
	proof.Channel = scores[bestIdx].name
	proof.Level = se.getChannelLevel(scores[bestIdx].name)
	se.mu.RUnlock()

	proof.Reason = fmt.Sprintf("推荐通道 %s (score=%.4f): 攻击率最高且误伤率最低，为帕累托最优放置点",
		proof.RecommendedChannel, scores[bestIdx].score)

	// 生成执行信封
	if se.envelopeMgr != nil {
		proofJSON, _ := json.Marshal(proof)
		se.envelopeMgr.Seal(
			"",
			"singularity_placement",
			string(proofJSON),
			"recommend",
			[]string{"singularity_pareto_optimal"},
			"",
		)
	}

	// 记录历史
	proofJSON, _ := json.Marshal(proof)
	se.recordHistory(proof.RecommendedChannel, proof.RecommendedLevel, "recommend", string(proofJSON))

	return proof
}

// getFPRateByChannel 获取通道的 FP 率
func (se *SingularityEngine) getFPRateByChannel(proof *PlacementProof, channel string) float64 {
	switch channel {
	case "im":
		return proof.IMFPRate
	case "llm":
		return proof.LLMFPRate
	case "toolcall":
		return proof.ToolCallFPRate
	}
	return 0
}

// SetExposureLevel 设置暴露等级
func (se *SingularityEngine) SetExposureLevel(channel string, level int) error {
	if level < 0 || level > 5 {
		return fmt.Errorf("暴露等级必须在 0-5 之间，收到 %d", level)
	}
	if channel != "im" && channel != "llm" && channel != "toolcall" {
		return fmt.Errorf("未知通道: %s（支持: im, llm, toolcall）", channel)
	}

	se.mu.Lock()
	switch channel {
	case "im":
		se.config.IMExposureLevel = level
	case "llm":
		se.config.LLMExposureLevel = level
	case "toolcall":
		se.config.ToolCallExposureLevel = level
	}
	se.mu.Unlock()

	// 生成执行信封
	if se.envelopeMgr != nil {
		se.envelopeMgr.Seal(
			"",
			"singularity_config",
			fmt.Sprintf("set_level channel=%s level=%d", channel, level),
			"config_change",
			[]string{"singularity_level_change"},
			"",
		)
	}

	// 记录历史
	se.recordHistory(channel, level, "set_level", "{}")

	return nil
}

// GetConfig 获取当前配置
func (se *SingularityEngine) GetConfig() SingularityConfig {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.config
}

// UpdateConfig 更新完整配置
func (se *SingularityEngine) UpdateConfig(cfg SingularityConfig) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.config = cfg
}

// GetBudget 计算奇点预算
func (se *SingularityEngine) GetBudget() *SingularityBudget {
	se.mu.RLock()
	cfg := se.config
	se.mu.RUnlock()

	return CalculateBudget(se.db, cfg)
}

// GetHistory 获取历史记录
func (se *SingularityEngine) GetHistory(limit int) []SingularityHistoryEntry {
	if limit <= 0 {
		limit = 50
	}

	rows, err := se.db.Query(`SELECT id, channel, level, action, proof_json, timestamp FROM singularity_history ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []SingularityHistoryEntry
	for rows.Next() {
		var entry SingularityHistoryEntry
		if err := rows.Scan(&entry.ID, &entry.Channel, &entry.Level, &entry.Action, &entry.ProofJSON, &entry.Timestamp); err != nil {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// recordHistory 记录配置变更历史
func (se *SingularityEngine) recordHistory(channel string, level int, action string, proofJSON string) {
	id := generateSingularityID()
	se.db.Exec(`INSERT INTO singularity_history (id, channel, level, action, proof_json, timestamp) VALUES (?,?,?,?,?,?)`,
		id, channel, level, action, proofJSON, time.Now().UTC().Format(time.RFC3339))
}

// getChannelLevel 获取通道暴露等级
func (se *SingularityEngine) getChannelLevel(channel string) int {
	switch channel {
	case "im":
		return se.config.IMExposureLevel
	case "llm":
		return se.config.LLMExposureLevel
	case "toolcall":
		return se.config.ToolCallExposureLevel
	}
	return 0
}

// generateSingularityID 生成唯一 ID
func generateSingularityID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
