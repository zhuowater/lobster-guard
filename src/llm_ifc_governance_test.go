package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectIFCVarIDs_Empty(t *testing.T) {
	if ids := collectIFCVarIDs(nil); len(ids) != 0 {
		t.Fatalf("expected empty ids, got %#v", ids)
	}
}

func TestEvaluateIFCForTool_NoEngineNoop(t *testing.T) {
	lp := &LLMProxy{}
	blocked := lp.evaluateIFCForTool(nil, llmIFCGovernanceContext{}, "tool-a", "{}")
	if blocked {
		t.Fatal("expected no block without ifc engine")
	}
}

func TestCollectIFCVarIDs_PreservesIDs(t *testing.T) {
	ids := collectIFCVarIDs([]IFCVariable{{ID: "a"}, {ID: "b"}})
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestEvaluateIFCForTool_BlockWritesResponse(t *testing.T) {
	db := openTestSQLite(t)
	eng := NewIFCEngine(db, IFCConfig{Enabled: true, DefaultConf: ConfPublic, DefaultInteg: IntegLow, ViolationAction: "block"})
	lp := &LLMProxy{ifcEngine: eng}
	rr := httptest.NewRecorder()
	blocked := lp.evaluateIFCForTool(rr, llmIFCGovernanceContext{TraceID: "trace-1"}, "send_email", `{"body":"hello"}`)
	if !blocked {
		t.Fatal("expected IFC block")
	}
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Tool call blocked by IFC") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestEvaluateIFCForTool_RegistersClassifiedSourceKey(t *testing.T) {
	db := openTestSQLite(t)
	eng := NewIFCEngine(db, IFCConfig{Enabled: true, DefaultConf: ConfPublic, DefaultInteg: IntegHigh, ViolationAction: "warn"})
	lp := &LLMProxy{ifcEngine: eng}
	traceID := "trace-classified-source"
	blocked := lp.evaluateIFCForTool(httptest.NewRecorder(), llmIFCGovernanceContext{TraceID: traceID}, "http_request", `{"url":"http://169.254.169.254/latest/meta-data/iam/security-credentials/"}`)
	if blocked {
		t.Fatal("expected no block while only registering classified source")
	}
	vars := eng.GetVariables(traceID)
	if len(vars) == 0 {
		t.Fatal("expected variables to be registered")
	}
	found := false
	for _, v := range vars {
		if strings.Contains(v.Source, "metadata_service") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected classified metadata_service source, got %#v", vars)
	}
}

func TestEvaluateIFCForTool_SourceClassificationSetsIFCLabel(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		wantConf  IFCLevel
		wantInteg IntegLevel
	}{
		{
			name:      "metadata service escalates to secret low integrity",
			args:      `{"url":"http://169.254.169.254/latest/meta-data/iam/security-credentials/"}`,
			wantConf:  ConfSecret,
			wantInteg: IntegLow,
		},
		{
			name:      "internal api escalates to confidential low integrity",
			args:      `{"url":"https://10.0.0.42/api/v1/customers"}`,
			wantConf:  ConfConfidential,
			wantInteg: IntegLow,
		},
		{
			name:      "public web downgrades to public taint",
			args:      `{"url":"https://docs.python.org/3/library/json.html"}`,
			wantConf:  ConfPublic,
			wantInteg: IntegTaint,
		},
		{
			name:      "authenticated external api is internal low integrity",
			args:      `{"url":"https://api.openai.com/v1/responses","headers":{"Authorization":"Bearer sk-test"}}`,
			wantConf:  ConfInternal,
			wantInteg: IntegLow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestSQLite(t)
			eng := NewIFCEngine(db, IFCConfig{Enabled: true, DefaultConf: ConfPublic, DefaultInteg: IntegHigh, ViolationAction: "warn"})
			lp := &LLMProxy{ifcEngine: eng}
			traceID := strings.ReplaceAll(tc.name, " ", "-")

			blocked := lp.evaluateIFCForTool(httptest.NewRecorder(), llmIFCGovernanceContext{TraceID: traceID}, "http_request", tc.args)
			if blocked {
				t.Fatal("expected no block while testing IFC label assignment")
			}

			vars := eng.GetVariables(traceID)
			if len(vars) == 0 {
				t.Fatal("expected a registered variable")
			}
			got := vars[len(vars)-1]
			if got.Label.Confidentiality != tc.wantConf || got.Label.Integrity != tc.wantInteg {
				t.Fatalf("unexpected IFC label: got=(%s,%s) want=(%s,%s) source=%s", got.Label.Confidentiality, got.Label.Integrity, tc.wantConf, tc.wantInteg, got.Source)
			}
		})
	}
}
