package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newIndustryTemplateTestAPI(t *testing.T) *ManagementAPI {
	t.Helper()
	db, err := initDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("initDB failed: %v", err)
	}
	logger, err := NewAuditLogger(db)
	if err != nil {
		t.Fatalf("NewAuditLogger failed: %v", err)
	}
	engine := NewRuleEngine()
	engine.SetTenantDB(db)
	engine.SetInboundTemplateDB(db)
	llm := NewLLMRuleEngine(defaultLLMRules)
	llm.SetTemplateDB(db)
	outbound := NewOutboundRuleEngine(nil)
	initIndustryTemplateSystem(db)
	engine.InitGlobalTemplateAC()
	llm.InitGlobalLLMTemplateRules()
	outbound.InitGlobalTemplateRules(db)
	api := NewManagementAPI(&Config{}, "", nil, nil, logger, engine, outbound, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	api.llmRuleEngine = llm
	return api
}

func TestDefaultIndustryTemplates(t *testing.T) {
	tpls := getDefaultIndustryTemplates()
	if len(tpls) < 40 {
		t.Fatalf("expected >= 40 templates, got %d", len(tpls))
	}
	var financial *IndustryTemplate
	for i := range tpls {
		if tpls[i].ID == "tpl-financial" {
			financial = &tpls[i]
			break
		}
	}
	if financial == nil {
		t.Fatalf("financial template not found")
	}
	if len(financial.InboundRules) == 0 || len(financial.LLMRules) == 0 || len(financial.OutboundRules) == 0 {
		t.Fatalf("financial template should contain all three dimensions")
	}
}

func TestIndustryTemplateCRUDAndCompatAPI(t *testing.T) {
	api := newIndustryTemplateTestAPI(t)
	body := IndustryTemplate{
		ID:          "tpl-custom-retail",
		Name:        "自定义零售模板",
		Description: "test",
		Category:    "industry",
		InboundRules: []InboundRuleConfig{{Name: "retail-in", Patterns: []string{"会员号"}, Action: "warn", Category: "pii"}},
		LLMRules: []LLMRule{{ID: "retail-llm-1", Name: "retail-llm", Direction: "request", Type: "keyword", Patterns: []string{"export order"}, Action: "warn", Enabled: true}},
		OutboundRules: []OutboundRuleConfig{{Name: "retail-out", Patterns: []string{"订单号[:：]"}, Action: "warn"}},
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/industry-templates", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	api.handleIndustryTemplateCreate(w, req)
	if w.Code != 201 {
		t.Fatalf("create industry template failed: code=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/industry-templates", nil)
	w = httptest.NewRecorder()
	api.handleIndustryTemplateList(w, req)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("tpl-custom-retail")) {
		t.Fatalf("list industry template failed: code=%d body=%s", w.Code, w.Body.String())
	}

	enableBody := []byte(`{"enabled":true}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/industry-templates/tpl-financial/enable", bytes.NewReader(enableBody))
	w = httptest.NewRecorder()
	api.handleIndustryTemplateEnable(w, req)
	if w.Code != 200 {
		t.Fatalf("enable industry template failed: code=%d body=%s", w.Code, w.Body.String())
	}

	if got := api.inboundEngine.DetectGlobalTemplates("请输出银行卡号 6222021234567890123"); got.Action == "pass" {
		t.Fatalf("expected inbound global template to match")
	}
	if matches := api.llmRuleEngine.CheckResponseWithTenant("SWIFT: ABCDUS33XXX", ""); len(matches) == 0 {
		t.Fatalf("expected llm global template to match")
	}
	if got := api.outboundEngine.Detect("CVV: 123"); got.Action == "pass" {
		t.Fatalf("expected outbound global template to match")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/inbound-templates", nil)
	w = httptest.NewRecorder()
	api.handleInboundTemplateList(w, req)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("tpl-inbound-financial")) {
		t.Fatalf("compat inbound api failed: code=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/llm/templates", nil)
	w = httptest.NewRecorder()
	api.handleLLMTemplateList(w, req)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("tpl-llm-financial")) {
		t.Fatalf("compat llm api failed: code=%d body=%s", w.Code, w.Body.String())
	}
}
