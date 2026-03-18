// rule_templates.go — 规则模板库（v5.1 智能检测）
// 使用 go:embed 嵌入 rules/*.yaml 模板文件
package main

import (
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed rules/*.yaml
var embeddedRuleTemplates embed.FS

// RuleTemplateMeta 规则模板元数据
type RuleTemplateMeta struct {
	Name      string `json:"name"`
	RuleCount int    `json:"rule_count"`
	Groups    []string `json:"groups"`
}

// ListRuleTemplates 列出所有可用的规则模板及其规则数
func ListRuleTemplates() []RuleTemplateMeta {
	entries, err := embeddedRuleTemplates.ReadDir("rules")
	if err != nil {
		log.Printf("[规则模板] 读取嵌入目录失败: %v", err)
		return nil
	}
	var templates []RuleTemplateMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		data, err := embeddedRuleTemplates.ReadFile("rules/" + entry.Name())
		if err != nil {
			continue
		}
		var rulesFile InboundRulesFileConfig
		if err := yaml.Unmarshal(data, &rulesFile); err != nil {
			continue
		}
		groups := make(map[string]bool)
		for _, r := range rulesFile.Rules {
			if r.Group != "" {
				groups[r.Group] = true
			}
		}
		groupList := make([]string, 0, len(groups))
		for g := range groups {
			groupList = append(groupList, g)
		}
		sort.Strings(groupList)
		templates = append(templates, RuleTemplateMeta{
			Name:      name,
			RuleCount: len(rulesFile.Rules),
			Groups:    groupList,
		})
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates
}

// LoadRuleTemplate 加载指定名称的规则模板
func LoadRuleTemplate(name string) ([]InboundRuleConfig, error) {
	filename := "rules/" + name + ".yaml"
	data, err := embeddedRuleTemplates.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("规则模板 %q 不存在", name)
	}
	var rulesFile InboundRulesFileConfig
	if err := yaml.Unmarshal(data, &rulesFile); err != nil {
		return nil, fmt.Errorf("解析规则模板 %q 失败: %w", name, err)
	}
	// 验证规则
	for i, rule := range rulesFile.Rules {
		if rule.Name == "" {
			return nil, fmt.Errorf("模板 %q 规则 #%d 缺少 name 字段", name, i+1)
		}
		if len(rule.Patterns) == 0 {
			return nil, fmt.Errorf("模板 %q 规则 %q 缺少 patterns", name, rule.Name)
		}
		if rule.Action == "" {
			rulesFile.Rules[i].Action = "block"
		}
	}
	return rulesFile.Rules, nil
}

// LoadRuleTemplates 加载多个规则模板并合并
func LoadRuleTemplates(names []string) ([]InboundRuleConfig, error) {
	var allRules []InboundRuleConfig
	for _, name := range names {
		rules, err := LoadRuleTemplate(name)
		if err != nil {
			return nil, err
		}
		allRules = append(allRules, rules...)
	}
	return allRules, nil
}

// MergeRulesWithTemplates 合并模板规则和自定义规则
// 自定义规则优先级更高：相同 name 的规则以自定义为准
func MergeRulesWithTemplates(templateRules, customRules []InboundRuleConfig) []InboundRuleConfig {
	// 建立自定义规则名称索引
	customNames := make(map[string]bool)
	for _, r := range customRules {
		customNames[r.Name] = true
	}
	// 模板规则中排除已被自定义规则覆盖的
	var merged []InboundRuleConfig
	for _, r := range templateRules {
		if !customNames[r.Name] {
			merged = append(merged, r)
		}
	}
	// 自定义规则追加（优先级更高）
	merged = append(merged, customRules...)
	return merged
}
