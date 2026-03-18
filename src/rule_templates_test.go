// rule_templates_test.go — 规则模板库测试（v5.1）
package main

import (
	"testing"
)

func TestListRuleTemplates(t *testing.T) {
	templates := ListRuleTemplates()
	if len(templates) != 4 {
		t.Errorf("expected 4 templates (general/financial/medical/government), got %d", len(templates))
	}
	// Check names
	names := map[string]bool{}
	for _, tmpl := range templates {
		names[tmpl.Name] = true
		if tmpl.RuleCount < 15 {
			t.Errorf("template %s has %d rules, expected at least 15", tmpl.Name, tmpl.RuleCount)
		}
		if len(tmpl.Groups) == 0 {
			t.Errorf("template %s has no groups", tmpl.Name)
		}
	}
	for _, expected := range []string{"general", "financial", "medical", "government"} {
		if !names[expected] {
			t.Errorf("expected template %q not found", expected)
		}
	}
}

func TestLoadRuleTemplate_General(t *testing.T) {
	rules, err := LoadRuleTemplate("general")
	if err != nil {
		t.Fatalf("failed to load general template: %v", err)
	}
	if len(rules) < 15 {
		t.Errorf("general template should have at least 15 rules, got %d", len(rules))
	}
	// All rules should have name and patterns
	for _, r := range rules {
		if r.Name == "" {
			t.Error("found rule without name")
		}
		if len(r.Patterns) == 0 {
			t.Errorf("rule %q has no patterns", r.Name)
		}
	}
}

func TestLoadRuleTemplate_Financial(t *testing.T) {
	rules, err := LoadRuleTemplate("financial")
	if err != nil {
		t.Fatalf("failed to load financial template: %v", err)
	}
	if len(rules) < 15 {
		t.Errorf("financial template should have at least 15 rules, got %d", len(rules))
	}
}

func TestLoadRuleTemplate_Medical(t *testing.T) {
	rules, err := LoadRuleTemplate("medical")
	if err != nil {
		t.Fatalf("failed to load medical template: %v", err)
	}
	if len(rules) < 15 {
		t.Errorf("medical template should have at least 15 rules, got %d", len(rules))
	}
}

func TestLoadRuleTemplate_Government(t *testing.T) {
	rules, err := LoadRuleTemplate("government")
	if err != nil {
		t.Fatalf("failed to load government template: %v", err)
	}
	if len(rules) < 15 {
		t.Errorf("government template should have at least 15 rules, got %d", len(rules))
	}
}

func TestLoadRuleTemplate_NotFound(t *testing.T) {
	_, err := LoadRuleTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestLoadRuleTemplates_Multiple(t *testing.T) {
	rules, err := LoadRuleTemplates([]string{"general", "financial"})
	if err != nil {
		t.Fatalf("failed to load multiple templates: %v", err)
	}
	if len(rules) < 30 {
		t.Errorf("expected at least 30 combined rules, got %d", len(rules))
	}
}

func TestMergeRulesWithTemplates(t *testing.T) {
	templateRules := []InboundRuleConfig{
		{Name: "tmpl_rule1", Patterns: []string{"pattern1"}, Action: "block", Group: "test"},
		{Name: "tmpl_rule2", Patterns: []string{"pattern2"}, Action: "warn", Group: "test"},
	}
	customRules := []InboundRuleConfig{
		{Name: "tmpl_rule1", Patterns: []string{"custom_pattern"}, Action: "warn", Group: "custom"}, // override
		{Name: "custom_rule3", Patterns: []string{"pattern3"}, Action: "block", Group: "custom"},
	}

	merged := MergeRulesWithTemplates(templateRules, customRules)

	if len(merged) != 3 {
		t.Errorf("expected 3 rules (1 template + 2 custom), got %d", len(merged))
	}

	// Verify tmpl_rule1 is overridden by custom
	foundOverride := false
	for _, r := range merged {
		if r.Name == "tmpl_rule1" {
			foundOverride = true
			if r.Action != "warn" {
				t.Errorf("tmpl_rule1 should be overridden to warn, got %s", r.Action)
			}
			if r.Patterns[0] != "custom_pattern" {
				t.Errorf("tmpl_rule1 should have custom_pattern, got %s", r.Patterns[0])
			}
		}
	}
	if !foundOverride {
		t.Error("tmpl_rule1 override not found")
	}
}

func TestMergeRulesWithTemplates_EmptyCustom(t *testing.T) {
	templateRules := []InboundRuleConfig{
		{Name: "tmpl_rule1", Patterns: []string{"pattern1"}, Action: "block"},
	}
	merged := MergeRulesWithTemplates(templateRules, nil)
	if len(merged) != 1 {
		t.Errorf("expected 1 rule, got %d", len(merged))
	}
}

func TestMergeRulesWithTemplates_EmptyTemplate(t *testing.T) {
	customRules := []InboundRuleConfig{
		{Name: "custom_rule1", Patterns: []string{"pattern1"}, Action: "block"},
	}
	merged := MergeRulesWithTemplates(nil, customRules)
	if len(merged) != 1 {
		t.Errorf("expected 1 rule, got %d", len(merged))
	}
}

func TestRuleTemplatesHaveRequiredFields(t *testing.T) {
	names := []string{"general", "financial", "medical", "government"}
	for _, name := range names {
		rules, err := LoadRuleTemplate(name)
		if err != nil {
			t.Errorf("failed to load %s: %v", name, err)
			continue
		}
		for _, r := range rules {
			if r.Name == "" {
				t.Errorf("[%s] found rule without name", name)
			}
			if len(r.Patterns) == 0 {
				t.Errorf("[%s] rule %q has no patterns", name, r.Name)
			}
			if r.Action == "" {
				t.Errorf("[%s] rule %q has no action", name, r.Name)
			}
			if r.Group == "" {
				t.Errorf("[%s] rule %q has no group", name, r.Name)
			}
			if r.Message == "" {
				t.Errorf("[%s] rule %q has no message", name, r.Name)
			}
			if r.Priority == 0 {
				// Priority 0 is the default, which is acceptable but we want explicit values
				// Just a warning-level check, not a failure
			}
		}
	}
}
