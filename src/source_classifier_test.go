package main

import "testing"

func TestClassifyToolSource_PublicWebURL(t *testing.T) {
	classifier := NewToolSourceClassifier()
	desc := classifier.Classify("web_fetch", `{"url":"https://docs.python.org/3/library/json.html"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "public_web" {
		t.Fatalf("expected public_web, got %q", desc.Category)
	}
	if desc.Confidentiality != ConfPublic {
		t.Fatalf("expected PUBLIC confidentiality, got %v", desc.Confidentiality)
	}
	if desc.Integrity != IntegTaint {
		t.Fatalf("expected TAINT integrity, got %v", desc.Integrity)
	}
}

func TestClassifyToolSource_MetadataServiceEscalatesToSecret(t *testing.T) {
	classifier := NewToolSourceClassifier()
	desc := classifier.Classify("http_request", `{"url":"http://169.254.169.254/latest/meta-data/iam/security-credentials/"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "metadata_service" {
		t.Fatalf("expected metadata_service, got %q", desc.Category)
	}
	if desc.Confidentiality != ConfSecret {
		t.Fatalf("expected SECRET confidentiality, got %v", desc.Confidentiality)
	}
	if !desc.PrivateNetwork {
		t.Fatal("expected private network true")
	}
}

func TestClassifyToolSource_InternalAPIByPrivateIP(t *testing.T) {
	classifier := NewToolSourceClassifier()
	desc := classifier.Classify("http_request", `{"url":"http://10.20.30.40/api/customers"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "internal_api" {
		t.Fatalf("expected internal_api, got %q", desc.Category)
	}
	if desc.Confidentiality != ConfConfidential {
		t.Fatalf("expected CONFIDENTIAL confidentiality, got %v", desc.Confidentiality)
	}
}

func TestClassifyToolSource_AuthenticatedExternalAPIUpgradesFromPublic(t *testing.T) {
	classifier := NewToolSourceClassifier()
	desc := classifier.Classify("http_request", `{"url":"https://api.stripe.com/v1/customers","headers":{"Authorization":"Bearer sk_test_123"}}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "external_api" {
		t.Fatalf("expected external_api, got %q", desc.Category)
	}
	if desc.AuthType != "bearer" {
		t.Fatalf("expected bearer auth, got %q", desc.AuthType)
	}
	if desc.Confidentiality != ConfInternal {
		t.Fatalf("expected INTERNAL confidentiality, got %v", desc.Confidentiality)
	}
	if desc.Integrity != IntegLow {
		t.Fatalf("expected LOW integrity, got %v", desc.Integrity)
	}
}

func TestClassifyToolSource_UnknownWhenNoURL(t *testing.T) {
	classifier := NewToolSourceClassifier()
	desc := classifier.Classify("file_write", `{"path":"/tmp/out.txt","content":"hello"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "unknown" {
		t.Fatalf("expected unknown, got %q", desc.Category)
	}
	if desc.SourceKey != "tool:file_write" {
		t.Fatalf("expected fallback source key, got %q", desc.SourceKey)
	}
}

func TestClassifyToolSource_ConfigRuleMatchesHostAndPath(t *testing.T) {
	classifier := NewToolSourceClassifierWithConfig(ToolSourceClassifierConfig{
		Rules: []ToolSourceRule{
			{
				Name:            "corp-control-plane",
				HostPattern:     `^control\.corp\.example$`,
				PathPattern:     `/v[0-9]+/admin/`,
				Category:        "internal_control_plane",
				Confidentiality: ConfSecret,
				Integrity:       IntegHigh,
				TrustScore:      0.92,
				Tags:            []string{"control_plane", "corp_override"},
			},
		},
	})
	desc := classifier.Classify("http_request", `{"url":"https://control.corp.example/v1/admin/rotate-keys"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "internal_control_plane" {
		t.Fatalf("expected internal_control_plane, got %q", desc.Category)
	}
	if desc.Confidentiality != ConfSecret || desc.Integrity != IntegHigh {
		t.Fatalf("unexpected labels conf=%v integ=%v", desc.Confidentiality, desc.Integrity)
	}
	if desc.TrustScore != 0.92 {
		t.Fatalf("expected trust 0.92, got %v", desc.TrustScore)
	}
	if !containsTag(desc.Tags, "corp_override") {
		t.Fatalf("expected corp_override tag, got %#v", desc.Tags)
	}
}

func TestClassifyToolSource_ConfigRuleOverridesBuiltinHeuristic(t *testing.T) {
	classifier := NewToolSourceClassifierWithConfig(ToolSourceClassifierConfig{
		Rules: []ToolSourceRule{
			{
				Name:            "docs-portal-promoted-to-internal",
				HostPattern:     `^docs\.python\.org$`,
				Category:        "internal_knowledge_portal",
				Confidentiality: ConfInternal,
				Integrity:       IntegMedium,
				TrustScore:      0.8,
			},
		},
	})
	desc := classifier.Classify("web_fetch", `{"url":"https://docs.python.org/3/library/json.html"}`)
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.Category != "internal_knowledge_portal" {
		t.Fatalf("expected override category internal_knowledge_portal, got %q", desc.Category)
	}
	if desc.Confidentiality != ConfInternal || desc.Integrity != IntegMedium {
		t.Fatalf("unexpected override labels conf=%v integ=%v", desc.Confidentiality, desc.Integrity)
	}
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
