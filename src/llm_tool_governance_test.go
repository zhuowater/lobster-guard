package main

import (
	"strings"
	"testing"
)

func TestApplySourceClassificationToPathPolicy(t *testing.T) {
	tests := []struct {
		name         string
		desc         *SourceDescriptor
		wantTaints   []string
		wantMinRisk  float64
		wantActionIn string
	}{
		{
			name: "metadata service raises secret taint and high risk",
			desc: &SourceDescriptor{Category: "metadata_service", Host: "169.254.169.254", Confidentiality: ConfSecret, Integrity: IntegLow, AuthType: "none"},
			wantTaints: []string{"SOURCE:METADATA_SERVICE", "CONF:SECRET", "INTEG:LOW", "PRIVATE_NETWORK"},
			wantMinRisk: 40,
			wantActionIn: "source:metadata_service",
		},
		{
			name: "internal api raises confidential taint and medium risk",
			desc: &SourceDescriptor{Category: "internal_api", Host: "10.0.0.42", Confidentiality: ConfConfidential, Integrity: IntegLow, PrivateNetwork: true},
			wantTaints: []string{"SOURCE:INTERNAL_API", "CONF:CONFIDENTIAL", "INTEG:LOW", "PRIVATE_NETWORK"},
			wantMinRisk: 20,
			wantActionIn: "source:internal_api",
		},
		{
			name: "public web marks taint but keeps low risk",
			desc: &SourceDescriptor{Category: "public_web", Host: "docs.python.org", Confidentiality: ConfPublic, Integrity: IntegTaint},
			wantTaints: []string{"SOURCE:PUBLIC_WEB", "CONF:PUBLIC", "INTEG:TAINT"},
			wantMinRisk: 5,
			wantActionIn: "source:public_web",
		},
		{
			name: "external api with auth marks egress sensitive path",
			desc: &SourceDescriptor{Category: "external_api", Host: "api.openai.com", Confidentiality: ConfInternal, Integrity: IntegLow, AuthType: "bearer"},
			wantTaints: []string{"SOURCE:EXTERNAL_API", "CONF:INTERNAL", "INTEG:LOW", "AUTH:BEARER"},
			wantMinRisk: 15,
			wantActionIn: "source:external_api",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := NewPathPolicyEngine(nil)
			traceID := strings.ReplaceAll(tc.name, " ", "-")
			e.RegisterStep(traceID, PathStep{Stage: "tool_call", Action: "http_request"})

			applySourceClassificationToPathPolicy(e, traceID, tc.desc)

			ctx := e.GetContext(traceID)
			if ctx == nil {
				t.Fatal("expected path context")
			}
			for _, want := range tc.wantTaints {
				if !containsString(ctx.TaintLabels, want) {
					t.Fatalf("missing taint %q in %#v", want, ctx.TaintLabels)
				}
			}
			if ctx.RiskScore < tc.wantMinRisk {
				t.Fatalf("expected risk >= %.1f, got %.1f", tc.wantMinRisk, ctx.RiskScore)
			}
			found := false
			for _, step := range ctx.Steps {
				if strings.Contains(step.Action, tc.wantActionIn) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected source classification step containing %q, got %#v", tc.wantActionIn, ctx.Steps)
			}
		})
	}
}

func TestEvaluateToolPolicyForResponseTool_SourceClassificationRaisesPathDecision(t *testing.T) {
	db := openTestSQLite(t)
	lp := &LLMProxy{
		toolPolicy:       NewToolPolicyEngine(db, ToolPolicyConfig{Enabled: true, DefaultAction: "allow"}),
		pathPolicyEngine: NewPathPolicyEngine(db),
	}

	tpEvent, ok := lp.evaluateToolPolicyForResponseTool(
		llmToolGovernanceContext{TraceID: "trace-metadata-path", TenantID: "default"},
		"http_request",
		`{"url":"http://169.254.169.254/latest/meta-data/iam/security-credentials/"}`,
	)
	if !ok {
		t.Fatal("expected governance evaluation to proceed")
	}
	if tpEvent == nil {
		t.Fatal("expected tool policy event")
	}
	if tpEvent.Decision != "warn" && tpEvent.Decision != "block" && tpEvent.Decision != "isolate" {
		t.Fatalf("expected path policy to elevate metadata-service call, got decision=%s rule=%s", tpEvent.Decision, tpEvent.RuleHit)
	}
	ctx := lp.pathPolicyEngine.GetContext("trace-metadata-path")
	if ctx == nil || !containsString(ctx.TaintLabels, "SOURCE:METADATA_SERVICE") {
		t.Fatalf("expected metadata source taint in path context, got %#v", ctx)
	}
}

func TestSourceAwarePathPolicyRules(t *testing.T) {
	tests := []struct {
		name         string
		source       *SourceDescriptor
		proposed     string
		wantDecision string
		wantRuleID   string
	}{
		{
			name:         "metadata service then send email blocks",
			source:       &SourceDescriptor{Category: "metadata_service", SourceKey: "tool:http_request:metadata_service", Confidentiality: ConfSecret, Integrity: IntegLow},
			proposed:     "send_email",
			wantDecision: "block",
			wantRuleID:   "pp-019",
		},
		{
			name:         "metadata service then shell blocks",
			source:       &SourceDescriptor{Category: "metadata_service", SourceKey: "tool:http_request:metadata_service", Confidentiality: ConfSecret, Integrity: IntegLow},
			proposed:     "shell_exec",
			wantDecision: "block",
			wantRuleID:   "pp-020",
		},
		{
			name:         "internal api then send email warns",
			source:       &SourceDescriptor{Category: "internal_api", SourceKey: "tool:http_request:internal_api:10.0.0.42", Confidentiality: ConfConfidential, Integrity: IntegLow, PrivateNetwork: true},
			proposed:     "send_email",
			wantDecision: "warn",
			wantRuleID:   "pp-021",
		},
		{
			name:         "external api with auth then file write warns",
			source:       &SourceDescriptor{Category: "external_api", SourceKey: "tool:http_request:external_api:api.openai.com", Confidentiality: ConfInternal, Integrity: IntegLow, AuthType: "bearer"},
			proposed:     "file_write",
			wantDecision: "warn",
			wantRuleID:   "pp-022",
		},
		{
			name:         "public web then shell blocks",
			source:       &SourceDescriptor{Category: "public_web", SourceKey: "tool:web_fetch:public_web", Confidentiality: ConfPublic, Integrity: IntegTaint},
			proposed:     "shell_exec",
			wantDecision: "block",
			wantRuleID:   "pp-023",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := NewPathPolicyEngine(nil)
			traceID := strings.ReplaceAll(tc.name, " ", "-")
			e.RegisterStep(traceID, PathStep{Stage: "inbound", Action: "inbound_message"})
			applySourceClassificationToPathPolicy(e, traceID, tc.source)
			d := e.Evaluate(traceID, tc.proposed)
			if d.Decision != tc.wantDecision || d.RuleID != tc.wantRuleID {
				t.Fatalf("unexpected decision: got=(%s,%s) want=(%s,%s)", d.Decision, d.RuleID, tc.wantDecision, tc.wantRuleID)
			}
		})
	}
}

func TestRegisterCapabilityToolResultWithSource(t *testing.T) {
	db := openTestSQLite(t)
	ce := NewCapabilityEngine(db, defaultCapConfig)
	ce.InitContext("trace-cap-src", "user-1", nil)

	tr := registerCapabilityToolResultWithSource(
		ce,
		nil,
		"default",
		"trace-cap-src",
		"http_request",
		"data-cap-src-1",
		`{"url":"http://169.254.169.254/latest/meta-data/iam/security-credentials/"}`,
	)
	if tr == nil {
		t.Fatal("expected capability tool result")
	}
	ctx := ce.GetContext("trace-cap-src")
	if ctx == nil {
		t.Fatal("expected capability context")
	}
	item := ctx.DataItems["data-cap-src-1"]
	if item == nil || item.SourceDescriptor == nil {
		t.Fatalf("expected source descriptor in capability item, got %#v", item)
	}
	if item.SourceDescriptor.Category != "metadata_service" {
		t.Fatalf("expected metadata_service, got %#v", item.SourceDescriptor)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
