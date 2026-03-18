// singularity_budget.go — 奇点预算仪表盘：拓扑不变量映射的安全暴露预算管理（v18.3）
package main

import (
	"database/sql"
	"fmt"
)

// ============================================================
// 奇点预算
// ============================================================

// SingularityBudget 奇点预算
type SingularityBudget struct {
	// 拓扑不变量
	TotalChannels int     `json:"total_channels"` // 启用的数据通道数
	TotalEngines  int     `json:"total_engines"`  // 启用的检测引擎数
	RuleCoverage  float64 `json:"rule_coverage"`  // 规则覆盖度 (0-1)

	// 欧拉示性数映射
	EulerCharacteristic int `json:"euler_characteristic"` // χ = channels + coverage_gaps - engines
	MinSingularities    int `json:"min_singularities"`    // 拓扑下限（最少几个奇点）

	// 预算分配
	TotalBudget       int    `json:"total_budget"`       // 总预算 = max(MinSingularities, 配置值)
	AllocatedIM       int    `json:"allocated_im"`       // IM 通道分配
	AllocatedLLM      int    `json:"allocated_llm"`      // LLM 通道分配
	AllocatedToolCall int    `json:"allocated_toolcall"` // Tool Call 分配
	Remaining         int    `json:"remaining"`          // 剩余预算
	OverBudget        bool   `json:"over_budget"`        // 是否超支（总暴露 < 拓扑下限）
	OverBudgetWarning string `json:"over_budget_warning"` // 超支警告

	// 各通道详情
	CoverageGaps int `json:"coverage_gaps"` // 未覆盖的 OWASP 分类数
}

// OWASP LLM Top10 分类数
const owaspLLMTop10Count = 10

// CalculateBudget 计算奇点预算
func CalculateBudget(db *sql.DB, cfg SingularityConfig) *SingularityBudget {
	budget := &SingularityBudget{}

	// 1. 计算启用的通道数
	budget.TotalChannels = countActiveChannels(db)

	// 2. 计算活跃检测引擎数
	budget.TotalEngines = countActiveEngines(db)

	// 3. 计算规则覆盖度 + coverage_gaps
	budget.RuleCoverage, budget.CoverageGaps = computeRuleCoverage(db)

	// 4. 欧拉示性数映射: χ = channels + coverage_gaps - engines
	budget.EulerCharacteristic = budget.TotalChannels + budget.CoverageGaps - budget.TotalEngines

	// 5. 拓扑下限: MinSingularities = max(2, EulerCharacteristic)
	//    球面至少 2 个奇点（Poincaré–Hopf 定理）
	budget.MinSingularities = budget.EulerCharacteristic
	if budget.MinSingularities < 2 {
		budget.MinSingularities = 2
	}

	// 6. 预算分配
	budget.AllocatedIM = cfg.IMExposureLevel
	budget.AllocatedLLM = cfg.LLMExposureLevel
	budget.AllocatedToolCall = cfg.ToolCallExposureLevel
	allocated := budget.AllocatedIM + budget.AllocatedLLM + budget.AllocatedToolCall

	// 7. 总预算 = max(MinSingularities, 已分配值)
	budget.TotalBudget = budget.MinSingularities
	if allocated > budget.TotalBudget {
		budget.TotalBudget = allocated
	}

	// 8. 剩余预算
	budget.Remaining = budget.TotalBudget - allocated

	// 9. 超支检查: 如果已分配 < 拓扑下限，说明暴露不够
	if allocated < budget.MinSingularities {
		budget.OverBudget = true
		budget.OverBudgetWarning = fmt.Sprintf(
			"⚠️ 奇点预算不足: 已分配 %d 点（IM=%d + LLM=%d + ToolCall=%d），但拓扑下限要求至少 %d 点。"+
				"请增加暴露等级以满足安全覆盖需求。(χ=%d, channels=%d, engines=%d, gaps=%d)",
			allocated, cfg.IMExposureLevel, cfg.LLMExposureLevel, cfg.ToolCallExposureLevel,
			budget.MinSingularities,
			budget.EulerCharacteristic, budget.TotalChannels, budget.TotalEngines, budget.CoverageGaps)
	}

	return budget
}

// countActiveChannels 计算启用的通道数
func countActiveChannels(db *sql.DB) int {
	channels := 0

	// 检查入站通道
	var inboundCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'inbound' LIMIT 1`).Scan(&inboundCount); err == nil && inboundCount > 0 {
		channels++
	}

	// 检查出站通道
	var outboundCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction = 'outbound' LIMIT 1`).Scan(&outboundCount); err == nil && outboundCount > 0 {
		channels++
	}

	// 检查 LLM 通道
	var llmCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE direction IN ('llm_request', 'llm_response') LIMIT 1`).Scan(&llmCount); err == nil && llmCount > 0 {
		channels++
	}

	// 至少有 1 个通道（入站是标配）
	if channels == 0 {
		channels = 1
	}

	return channels
}

// countActiveEngines 计算活跃检测引擎数
func countActiveEngines(db *sql.DB) int {
	engines := 1 // 基础规则引擎始终存在

	// 检查 LLM 检测引擎
	var llmDetects int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE reason LIKE '%llm_detect%' LIMIT 1`).Scan(&llmDetects); err == nil && llmDetects > 0 {
		engines++
	}

	// 检查会话检测引擎
	var sessionDetects int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE reason LIKE '%session%' LIMIT 1`).Scan(&sessionDetects); err == nil && sessionDetects > 0 {
		engines++
	}

	// 检查蜜罐引擎
	var honeypotCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'honeypot' LIMIT 1`).Scan(&honeypotCount); err == nil && honeypotCount > 0 {
		engines++
	}

	return engines
}

// computeRuleCoverage 计算规则覆盖度和覆盖缺口
func computeRuleCoverage(db *sql.DB) (float64, int) {
	// OWASP LLM Top10 分类
	owaspCategories := []string{
		"prompt_injection",
		"insecure_output_handling",
		"training_data_poisoning",
		"model_denial_of_service",
		"supply_chain_vulnerabilities",
		"sensitive_information_disclosure",
		"insecure_plugin_design",
		"excessive_agency",
		"overreliance",
		"model_theft",
	}

	coveredCount := 0

	// 检查红队测试覆盖了哪些分类
	for _, cat := range owaspCategories {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE reason LIKE ?`, "%"+cat+"%").Scan(&count)
		if err == nil && count > 0 {
			coveredCount++
		}
	}

	total := len(owaspCategories)
	gaps := total - coveredCount
	coverage := 0.0
	if total > 0 {
		coverage = float64(coveredCount) / float64(total)
	}

	return coverage, gaps
}
