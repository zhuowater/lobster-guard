package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type sourceClassifierExplainRequest struct {
	TenantID         string          `json:"tenant_id"`
	ToolName         string          `json:"tool_name"`
	ToolArgs         json.RawMessage `json:"tool_args"`
	ProposedAction   string          `json:"proposed_action"`
	CapabilityAction string          `json:"capability_action"`
}

type sourceClassifierRuleHit struct {
	Matched  bool            `json:"matched"`
	Name     string          `json:"name,omitempty"`
	Category string          `json:"category,omitempty"`
	Scope    string          `json:"scope,omitempty"`
	Rule     *ToolSourceRule `json:"rule,omitempty"`
}

type sourceClassifierExplainResponse struct {
	TenantID             string                  `json:"tenant_id,omitempty"`
	ToolName             string                  `json:"tool_name"`
	TenantOverrideActive bool                    `json:"tenant_override_active"`
	GlobalDescriptor     *SourceDescriptor       `json:"global_descriptor,omitempty"`
	EffectiveDescriptor  *SourceDescriptor       `json:"effective_descriptor,omitempty"`
	GlobalRule           sourceClassifierRuleHit `json:"global_rule"`
	EffectiveRule        sourceClassifierRuleHit `json:"effective_rule"`
	PathDecision         *PathDecision           `json:"path_decision,omitempty"`
	PathContext          *PathContext            `json:"path_context,omitempty"`
	CapabilityEvaluation *CapEvaluation          `json:"capability_evaluation,omitempty"`
}

func (api *ManagementAPI) persistConfig() error {
	if api.cfg == nil || api.cfgPath == "" {
		return nil
	}
	data, err := yaml.Marshal(api.cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(api.cfgPath, data, 0o600)
}

func (api *ManagementAPI) handleSourceClassifierGet(w http.ResponseWriter, r *http.Request) {
	if api.cfg == nil {
		jsonResponse(w, 500, map[string]string{"error": "config not available"})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"config": api.cfg.SourceClassifier})
}

func (api *ManagementAPI) handleSourceClassifierUpdate(w http.ResponseWriter, r *http.Request) {
	if api.cfg == nil {
		jsonResponse(w, 500, map[string]string{"error": "config not available"})
		return
	}
	var cfg ToolSourceClassifierConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	api.cfgMu.Lock()
	api.cfg.SourceClassifier = cfg
	api.cfgMu.Unlock()
	applySourceClassifierConfig(api.cfg)
	if err := api.persistConfig(); err != nil {
		jsonResponse(w, 500, map[string]string{"error": "persist config failed: " + err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]interface{}{"status": "updated", "config": api.cfg.SourceClassifier})
}

func (api *ManagementAPI) handleSourceClassifierExplain(w http.ResponseWriter, r *http.Request) {
	var req sourceClassifierExplainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ToolName == "" {
		jsonResponse(w, 400, map[string]string{"error": "tool_name required"})
		return
	}
	toolArgs := string(req.ToolArgs)
	if len(req.ToolArgs) == 0 {
		toolArgs = `{}`
	}

	globalDesc, globalRule := NewToolSourceClassifier().Explain(req.ToolName, toolArgs)
	effectiveDesc, effectiveRule := globalDesc, globalRule
	if req.TenantID != "" {
		effectiveDesc, effectiveRule = explainToolSourceForTenant(api.tenantMgr, req.TenantID, req.ToolName, toolArgs)
	}
	resp := sourceClassifierExplainResponse{
		TenantID:             req.TenantID,
		ToolName:             req.ToolName,
		TenantOverrideActive: !sameSourceDescriptor(globalDesc, effectiveDesc),
		GlobalDescriptor:     globalDesc,
		EffectiveDescriptor:  effectiveDesc,
		GlobalRule:           buildRuleHit(globalRule, "global"),
		EffectiveRule:        buildRuleHit(effectiveRule, effectiveRuleScope(req.TenantID, globalRule, effectiveRule)),
	}
	if pd, pc := api.explainPathPolicy(req.TenantID, req.ToolName, effectiveDesc, req.ProposedAction); pd != nil || pc != nil {
		resp.PathDecision = pd
		resp.PathContext = pc
	}
	if ce := api.explainCapability(req.TenantID, req.ToolName, toolArgs, effectiveDesc, req.CapabilityAction); ce != nil {
		resp.CapabilityEvaluation = ce
	}
	jsonResponse(w, 200, resp)
}

func buildRuleHit(rule *ToolSourceRule, scope string) sourceClassifierRuleHit {
	if rule == nil {
		return sourceClassifierRuleHit{Matched: false, Scope: scope}
	}
	return sourceClassifierRuleHit{Matched: true, Name: rule.Name, Category: rule.Category, Scope: scope, Rule: rule}
}

func effectiveRuleScope(tenantID string, globalRule, effectiveRule *ToolSourceRule) string {
	if effectiveRule == nil {
		if tenantID != "" {
			return "heuristic"
		}
		return "global"
	}
	if tenantID == "" || tenantID == "default" {
		return "global"
	}
	if globalRule != nil && effectiveRule.Name == globalRule.Name {
		return "global_fallback"
	}
	return "tenant_override"
}

func sameSourceDescriptor(a, b *SourceDescriptor) bool {
	if a == nil || b == nil {
		return a == b
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func (api *ManagementAPI) explainPathPolicy(tenantID, toolName string, sourceDesc *SourceDescriptor, proposedAction string) (*PathDecision, *PathContext) {
	if api.pathPolicyEngine == nil || sourceDesc == nil || proposedAction == "" {
		return nil, nil
	}
	engine := &PathPolicyEngine{
		contexts:    make(map[string]*PathContext),
		rules:       append([]PathPolicyRule{}, api.pathPolicyEngine.ListRules()...),
		riskWeights: map[string]float64{},
		halfLifeSec: 300,
	}
	for k, v := range defaultRiskWeights {
		engine.riskWeights[k] = v
	}
	traceID := fmt.Sprintf("source-classifier-explain-%d", time.Now().UnixNano())
	engine.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: toolName})
	if tenantID != "" {
		engine.SetTenantID(traceID, tenantID)
	}
	applySourceClassificationToPathPolicy(engine, traceID, sourceDesc)
	decision := engine.Evaluate(traceID, proposedAction)
	ctx := engine.GetContext(traceID)
	return &decision, ctx
}

func (api *ManagementAPI) explainCapability(tenantID, toolName, toolArgs string, sourceDesc *SourceDescriptor, action string) *CapEvaluation {
	if api.capabilityEngine == nil || action == "" {
		return nil
	}
	traceID := fmt.Sprintf("source-classifier-cap-%d", time.Now().UnixNano())
	api.capabilityEngine.InitContext(traceID, tenantID, nil)
	dataID := "source-classifier-preview"
	api.capabilityEngine.RegisterToolResultWithSource(traceID, toolName, dataID, sourceDesc)
	return api.capabilityEngine.EvaluateWithProvenance(traceID, dataID, action, toolName)
}
