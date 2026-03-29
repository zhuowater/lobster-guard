package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// IndustryTemplate v31.0 统一行业模板：入站 + LLM + 出站
// 启用一个模板 = 全链路防护
// ID 统一为 tpl-<industry>
type IndustryTemplate struct {
	ID            string               `json:"id" yaml:"id"`
	Name          string               `json:"name" yaml:"name"`
	Description   string               `json:"description" yaml:"description"`
	Category      string               `json:"category" yaml:"category"`
	InboundRules  []InboundRuleConfig  `json:"inbound_rules" yaml:"inbound_rules"`
	LLMRules      []LLMRule            `json:"llm_rules" yaml:"llm_rules"`
	OutboundRules []OutboundRuleConfig `json:"outbound_rules" yaml:"outbound_rules"`
	Enabled       bool                 `json:"enabled" yaml:"enabled"`
	BuiltIn       bool                 `json:"built_in" yaml:"built_in"`
}

type industryTemplateStore struct {
	db *sql.DB
}

func newIndustryTemplateStore(db *sql.DB) *industryTemplateStore {
	if db == nil {
		return nil
	}
	return &industryTemplateStore{db: db}
}

func normalizeIndustryTemplateID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "tpl-inbound-")
	id = strings.TrimPrefix(id, "tpl-llm-")
	id = strings.TrimPrefix(id, "tpl-")
	if id == "" {
		return ""
	}
	return "tpl-" + id
}

func normalizeIndustryTemplateName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, " 入站规则")
	name = strings.TrimSuffix(name, " LLM 规则")
	return name
}

func getIndustryOutboundTemplateRules() map[string][]OutboundRuleConfig {
	return map[string][]OutboundRuleConfig{
		"financial": {
			{Name: "tpl_financial_bank_card", DisplayName: "银行卡号泄露", Patterns: []string{`\b(62|4[0-9]|5[1-5])\d{14,17}\b`}, Action: "warn", Priority: 15, Message: "检测到金融行业银行卡号泄露"},
			{Name: "tpl_financial_swift", DisplayName: "SWIFT代码泄露", Patterns: []string{`(?i)\bSWIFT\s*[:：]\s*[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?\b`}, Action: "warn", Priority: 16, Message: "检测到 SWIFT 代码泄露"},
			{Name: "tpl_financial_transfer_account", DisplayName: "交易金额+账号组合", Patterns: []string{`(?i)(transfer|转账|wire)\s*.{0,30}(account|账[户号]|卡号)`}, Action: "warn", Priority: 17, Message: "检测到转账与账号组合信息"},
			{Name: "tpl_financial_cvv", DisplayName: "信用卡CVV泄露", Patterns: []string{`(?i)(CVV|CVC|安全码)\s*[:：]\s*\d{3,4}`}, Action: "block", Priority: 20, Message: "检测到 CVV / 安全码泄露"},
			{Name: "tpl_financial_loan_result", DisplayName: "贷款审批结果泄露", Patterns: []string{`(?i)(loan\s*approv|贷款审批|授信结果)\s*[:：]`}, Action: "warn", Priority: 16, Message: "检测到贷款审批/授信结果泄露"},
		},
		"government": {
			{Name: "tpl_government_doc_no", DisplayName: "红头文件号泄露", Patterns: []string{`(?:[\p{Han}]+发\s*[\[〔]?\d{4}[\]〕]?\s*\d+\s*号)`}, Action: "warn", Priority: 16, Message: "检测到政务红头文件编号"},
			{Name: "tpl_government_id_card", DisplayName: "公民身份证泄露", Patterns: []string{`\b\d{17}[\dXx]\b`}, Action: "warn", Priority: 16, Message: "检测到身份证号泄露"},
			{Name: "tpl_government_secret_level", DisplayName: "涉密等级标记泄露", Patterns: []string{`(?i)(机密|秘密|绝密|confidential|secret|top secret)`}, Action: "block", Priority: 20, Message: "检测到涉密等级标记"},
			{Name: "tpl_government_official_info", DisplayName: "公务员信息泄露", Patterns: []string{`(?:[\p{Han}]{2,4}(局长|处长|科长|厅长|部长|书记|主任))`}, Action: "warn", Priority: 15, Message: "检测到公务员身份信息"},
		},
		"healthcare": {
			{Name: "tpl_healthcare_mrn", DisplayName: "病历号泄露", Patterns: []string{`(?i)(病历号|medical record|MRN)\s*[:：]\s*[A-Z0-9]+`}, Action: "warn", Priority: 16, Message: "检测到病历号 / MRN 泄露"},
			{Name: "tpl_healthcare_gene", DisplayName: "基因序列泄露", Patterns: []string{`\b[ATCG]{20,}\b`}, Action: "warn", Priority: 18, Message: "检测到疑似基因序列"},
			{Name: "tpl_healthcare_hiv", DisplayName: "HIV状态泄露", Patterns: []string{`(?i)(HIV\s*(positive|negative|阳性|阴性|status))`}, Action: "block", Priority: 20, Message: "检测到 HIV 状态信息"},
			{Name: "tpl_healthcare_prescription_dose", DisplayName: "处方药物+剂量泄露", Patterns: []string{`(?i)(prescribed|处方|给药)\s*.{0,20}\d+\s*(mg|ml|片|粒)`}, Action: "warn", Priority: 17, Message: "检测到处方与剂量组合信息"},
		},
		"compliance": {
			{Name: "tpl_compliance_cross_border", DisplayName: "数据跨境", Patterns: []string{`(?i)(cross.?border|跨境传输|数据出境|data\s*transfer.*overseas)`}, Action: "warn", Priority: 16, Message: "检测到数据跨境/出境内容"},
			{Name: "tpl_compliance_bulk_pii", DisplayName: "个人信息批量泄露", Patterns: []string{`(?is)(?=(?:.*(?:姓名|name)))(?=(?:.*(?:身份证|id\s*card|身份证号)))(?=(?:.*(?:手机号|phone|mobile|电话)))`}, Action: "block", Priority: 20, Message: "检测到批量个人信息组合泄露"},
			{Name: "tpl_compliance_nda", DisplayName: "NDA/保密协议内容泄露", Patterns: []string{`(?i)(保密协议|NDA|non.?disclosure)\s*.{0,50}(内容|条款|details)`}, Action: "warn", Priority: 16, Message: "检测到保密协议/NDA 内容"},
		},
	}
}

func getDefaultIndustryTemplates() []IndustryTemplate {
	inboundTemplates := getDefaultInboundTemplates()
	llmTemplates := getDefaultLLMTemplates()
	llmBySuffix := make(map[string]LLMRuleTemplate)
	for _, tpl := range llmTemplates {
		suffix := strings.TrimPrefix(tpl.ID, "tpl-llm-")
		llmBySuffix[suffix] = tpl
	}
	outboundBySuffix := getIndustryOutboundTemplateRules()

	result := make([]IndustryTemplate, 0, len(inboundTemplates))
	seen := make(map[string]bool)
	for _, inboundTpl := range inboundTemplates {
		suffix := strings.TrimPrefix(inboundTpl.ID, "tpl-inbound-")
		unifiedID := normalizeIndustryTemplateID(suffix)
		llmTpl, ok := llmBySuffix[suffix]
		name := normalizeIndustryTemplateName(inboundTpl.Name)
		desc := inboundTpl.Description
		category := inboundTpl.Category
		llmRules := []LLMRule{}
		if ok {
			llmRules = llmTpl.Rules
			if name == "" {
				name = normalizeIndustryTemplateName(llmTpl.Name)
			}
			if desc == "" {
				desc = llmTpl.Description
			}
			if category == "" {
				category = llmTpl.Category
			}
		}
		result = append(result, IndustryTemplate{
			ID:            unifiedID,
			Name:          name,
			Description:   desc,
			Category:      category,
			InboundRules:  inboundTpl.Rules,
			LLMRules:      llmRules,
			OutboundRules: outboundBySuffix[suffix],
			BuiltIn:       true,
			Enabled:       inboundTpl.Enabled || (ok && llmTpl.Enabled),
		})
		seen[suffix] = true
	}
	for _, llmTpl := range llmTemplates {
		suffix := strings.TrimPrefix(llmTpl.ID, "tpl-llm-")
		if seen[suffix] {
			continue
		}
		result = append(result, IndustryTemplate{
			ID:            normalizeIndustryTemplateID(suffix),
			Name:          normalizeIndustryTemplateName(llmTpl.Name),
			Description:   llmTpl.Description,
			Category:      llmTpl.Category,
			InboundRules:  nil,
			LLMRules:      llmTpl.Rules,
			OutboundRules: outboundBySuffix[suffix],
			BuiltIn:       true,
			Enabled:       llmTpl.Enabled,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *industryTemplateStore) ensureTable() {
	if s == nil || s.db == nil {
		return
	}
	s.db.Exec(`CREATE TABLE IF NOT EXISTS industry_templates (
		id TEXT PRIMARY KEY,
		name TEXT,
		description TEXT,
		category TEXT,
		inbound_rules_json TEXT,
		llm_rules_json TEXT,
		outbound_rules_json TEXT,
		enabled INTEGER DEFAULT 0,
		built_in INTEGER DEFAULT 1,
		created_at TEXT,
		updated_at TEXT
	)`)
}

func (s *industryTemplateStore) seedBuiltins() {
	if s == nil || s.db == nil {
		return
	}
	s.ensureTable()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, tpl := range getDefaultIndustryTemplates() {
		inboundJSON, _ := json.Marshal(tpl.InboundRules)
		llmJSON, _ := json.Marshal(tpl.LLMRules)
		outboundJSON, _ := json.Marshal(tpl.OutboundRules)
		enabled := 0
		if existing := s.get(tpl.ID); existing != nil && existing.Enabled {
			enabled = 1
		}
		s.db.Exec(`INSERT OR IGNORE INTO industry_templates (id, name, description, category, inbound_rules_json, llm_rules_json, outbound_rules_json, enabled, built_in, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(inboundJSON), string(llmJSON), string(outboundJSON), enabled, 1, now, now)
		s.db.Exec(`UPDATE industry_templates SET name=?, description=?, category=?, inbound_rules_json=?, llm_rules_json=?, outbound_rules_json=?, updated_at=? WHERE id=? AND built_in=1`,
			tpl.Name, tpl.Description, tpl.Category, string(inboundJSON), string(llmJSON), string(outboundJSON), now, tpl.ID)
	}
}

func (s *industryTemplateStore) migrateLegacyTemplates() {
	if s == nil || s.db == nil {
		return
	}
	s.ensureTable()
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM industry_templates`).Scan(&count); err == nil && count > 0 {
		return
	}
	defaults := getDefaultIndustryTemplates()
	byID := make(map[string]IndustryTemplate)
	for _, tpl := range defaults {
		byID[tpl.ID] = tpl
	}

	if rows, err := s.db.Query(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM inbound_rule_templates`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var oldID, name, desc, category, rulesJSON string
			var builtIn, enabled int
			if rows.Scan(&oldID, &name, &desc, &category, &rulesJSON, &builtIn, &enabled) != nil {
				continue
			}
			newID := normalizeIndustryTemplateID(oldID)
			tpl := byID[newID]
			tpl.ID = newID
			if name != "" { tpl.Name = normalizeIndustryTemplateName(name) }
			if desc != "" { tpl.Description = desc }
			if category != "" { tpl.Category = category }
			if json.Unmarshal([]byte(rulesJSON), &tpl.InboundRules) != nil {
				continue
			}
			tpl.BuiltIn = builtIn == 1
			tpl.Enabled = tpl.Enabled || enabled == 1
			byID[newID] = tpl
		}
	}
	if rows, err := s.db.Query(`SELECT id, name, description, category, rules_json, built_in, COALESCE(enabled,0) FROM llm_rule_templates`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var oldID, name, desc, category, rulesJSON string
			var builtIn, enabled int
			if rows.Scan(&oldID, &name, &desc, &category, &rulesJSON, &builtIn, &enabled) != nil {
				continue
			}
			newID := normalizeIndustryTemplateID(oldID)
			tpl := byID[newID]
			tpl.ID = newID
			if tpl.Name == "" && name != "" { tpl.Name = normalizeIndustryTemplateName(name) }
			if tpl.Description == "" && desc != "" { tpl.Description = desc }
			if tpl.Category == "" && category != "" { tpl.Category = category }
			if json.Unmarshal([]byte(rulesJSON), &tpl.LLMRules) != nil {
				continue
			}
			tpl.BuiltIn = tpl.BuiltIn || builtIn == 1
			tpl.Enabled = tpl.Enabled || enabled == 1
			if tpl.OutboundRules == nil {
				tpl.OutboundRules = getIndustryOutboundTemplateRules()[strings.TrimPrefix(newID, "tpl-")]
			}
			byID[newID] = tpl
		}
	}
	for _, tpl := range byID {
		inboundJSON, _ := json.Marshal(tpl.InboundRules)
		llmJSON, _ := json.Marshal(tpl.LLMRules)
		outboundJSON, _ := json.Marshal(tpl.OutboundRules)
		builtIn := 0
		if tpl.BuiltIn { builtIn = 1 }
		enabled := 0
		if tpl.Enabled { enabled = 1 }
		s.db.Exec(`INSERT OR IGNORE INTO industry_templates (id, name, description, category, inbound_rules_json, llm_rules_json, outbound_rules_json, enabled, built_in, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(inboundJSON), string(llmJSON), string(outboundJSON), enabled, builtIn, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	}
}

func (s *industryTemplateStore) list() []IndustryTemplate {
	if s == nil || s.db == nil {
		return getDefaultIndustryTemplates()
	}
	s.ensureTable()
	rows, err := s.db.Query(`SELECT id, name, description, category, inbound_rules_json, llm_rules_json, outbound_rules_json, COALESCE(enabled,0), COALESCE(built_in,1) FROM industry_templates ORDER BY id`)
	if err != nil {
		return getDefaultIndustryTemplates()
	}
	defer rows.Close()
	var result []IndustryTemplate
	for rows.Next() {
		var tpl IndustryTemplate
		var inboundJSON, llmJSON, outboundJSON string
		var enabled, builtIn int
		if rows.Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.Category, &inboundJSON, &llmJSON, &outboundJSON, &enabled, &builtIn) != nil {
			continue
		}
		tpl.Enabled = enabled == 1
		tpl.BuiltIn = builtIn == 1
		_ = json.Unmarshal([]byte(inboundJSON), &tpl.InboundRules)
		_ = json.Unmarshal([]byte(llmJSON), &tpl.LLMRules)
		_ = json.Unmarshal([]byte(outboundJSON), &tpl.OutboundRules)
		result = append(result, tpl)
	}
	if len(result) == 0 {
		return getDefaultIndustryTemplates()
	}
	return result
}

func (s *industryTemplateStore) get(id string) *IndustryTemplate {
	id = normalizeIndustryTemplateID(id)
	for _, tpl := range s.list() {
		if tpl.ID == id {
			cp := tpl
			return &cp
		}
	}
	return nil
}

func (s *industryTemplateStore) create(tpl IndustryTemplate) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("行业模板 DB 未初始化")
	}
	tpl.ID = normalizeIndustryTemplateID(tpl.ID)
	if tpl.ID == "" || tpl.Name == "" {
		return fmt.Errorf("id 和 name 不能为空")
	}
	if s.get(tpl.ID) != nil {
		return fmt.Errorf("模板 %q 已存在", tpl.ID)
	}
	inboundJSON, _ := json.Marshal(tpl.InboundRules)
	llmJSON, _ := json.Marshal(tpl.LLMRules)
	outboundJSON, _ := json.Marshal(tpl.OutboundRules)
	builtIn := 0
	if tpl.BuiltIn { builtIn = 1 }
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO industry_templates (id, name, description, category, inbound_rules_json, llm_rules_json, outbound_rules_json, enabled, built_in, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, tpl.ID, tpl.Name, tpl.Description, tpl.Category, string(inboundJSON), string(llmJSON), string(outboundJSON), 0, builtIn, now, now)
	return err
}

func (s *industryTemplateStore) update(id string, tpl IndustryTemplate) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("行业模板 DB 未初始化")
	}
	id = normalizeIndustryTemplateID(id)
	existing := s.get(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	if tpl.Name == "" {
		tpl.Name = existing.Name
	}
	if tpl.Description == "" {
		tpl.Description = existing.Description
	}
	if tpl.Category == "" {
		tpl.Category = existing.Category
	}
	if tpl.InboundRules == nil {
		tpl.InboundRules = existing.InboundRules
	}
	if tpl.LLMRules == nil {
		tpl.LLMRules = existing.LLMRules
	}
	if tpl.OutboundRules == nil {
		tpl.OutboundRules = existing.OutboundRules
	}
	inboundJSON, _ := json.Marshal(tpl.InboundRules)
	llmJSON, _ := json.Marshal(tpl.LLMRules)
	outboundJSON, _ := json.Marshal(tpl.OutboundRules)
	_, err := s.db.Exec(`UPDATE industry_templates SET name=?, description=?, category=?, inbound_rules_json=?, llm_rules_json=?, outbound_rules_json=?, updated_at=? WHERE id=?`,
		tpl.Name, tpl.Description, tpl.Category, string(inboundJSON), string(llmJSON), string(outboundJSON), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *industryTemplateStore) delete(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("行业模板 DB 未初始化")
	}
	id = normalizeIndustryTemplateID(id)
	existing := s.get(id)
	if existing == nil {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	if existing.BuiltIn {
		return fmt.Errorf("内置模板 %q 不可删除", id)
	}
	_, err := s.db.Exec(`DELETE FROM industry_templates WHERE id=? AND built_in=0`, id)
	return err
}

func (s *industryTemplateStore) setEnabled(id string, enabled bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("行业模板 DB 未初始化")
	}
	id = normalizeIndustryTemplateID(id)
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(`UPDATE industry_templates SET enabled=?, updated_at=? WHERE id=?`, val, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("模板 %q 不存在", id)
	}
	return nil
}

func (api *ManagementAPI) industryTemplateStore() *industryTemplateStore {
	if api == nil || api.logger == nil || api.logger.DB() == nil {
		return nil
	}
	store := newIndustryTemplateStore(api.logger.DB())
	store.ensureTable()
	store.seedBuiltins()
	store.migrateLegacyTemplates()
	return store
}

func (api *ManagementAPI) listIndustryTemplates() []IndustryTemplate {
	store := api.industryTemplateStore()
	if store == nil {
		return getDefaultIndustryTemplates()
	}
	return store.list()
}

func (api *ManagementAPI) getIndustryTemplate(id string) *IndustryTemplate {
	store := api.industryTemplateStore()
	if store == nil {
		for _, tpl := range getDefaultIndustryTemplates() {
			if tpl.ID == normalizeIndustryTemplateID(id) {
				cp := tpl
				return &cp
			}
		}
		return nil
	}
	return store.get(id)
}

func (api *ManagementAPI) syncIndustryTemplateEngines() {
	if api == nil {
		return
	}
	if api.inboundEngine != nil {
		api.inboundEngine.InitGlobalTemplateAC()
	}
	if api.llmRuleEngine != nil {
		api.llmRuleEngine.InitGlobalLLMTemplateRules()
	}
	if api.outboundEngine != nil && api.logger != nil {
		api.outboundEngine.InitGlobalTemplateRules(api.logger.DB())
	}
}

func initIndustryTemplateSystem(db *sql.DB) {
	store := newIndustryTemplateStore(db)
	if store == nil {
		return
	}
	store.ensureTable()
	store.seedBuiltins()
	store.migrateLegacyTemplates()
	log.Printf("[行业模板] 统一模板系统已初始化")
}
