// config_confdir_test.go — conf.d/ 分层配置加载测试
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: 创建临时主配置文件 + 可选 conf.d/ 目录
func setupTestConfig(t *testing.T, mainYAML string) (mainPath string, confDir string) {
	t.Helper()
	dir := t.TempDir()
	mainPath = filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}
	confDir = filepath.Join(dir, "conf.d")
	return
}

// 测试1: conf.d 不存在 = 静默跳过
func TestLoadConfDir_NotExist(t *testing.T) {
	mainPath, _ := setupTestConfig(t, `
channel: "lanxin"
inbound_listen: ":8443"
`)
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Channel != "lanxin" {
		t.Errorf("expected channel=lanxin, got %q", cfg.Channel)
	}
}

// 测试2: conf.d 存在但为空 = 正常
func TestLoadConfDir_EmptyDir(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
channel: "lanxin"
log_level: "debug"
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level=debug, got %q", cfg.LogLevel)
	}
}

// 测试3: conf.d 中有一个 yaml = 字段覆盖
func TestLoadConfDir_SingleFile(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
channel: "lanxin"
log_level: "info"
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "override.yaml"), []byte(`log_level: "warn"`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log_level=warn after conf.d override, got %q", cfg.LogLevel)
	}
	// channel should remain from main config
	if cfg.Channel != "lanxin" {
		t.Errorf("expected channel=lanxin (unchanged), got %q", cfg.Channel)
	}
}

// 测试4: conf.d 中多个 yaml = 按字母序覆盖
func TestLoadConfDir_MultipleFiles_AlphaOrder(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
log_level: "info"
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	// a.yaml 设置 log_level = "debug"
	if err := os.WriteFile(filepath.Join(confDir, "a.yaml"), []byte(`log_level: "debug"`), 0644); err != nil {
		t.Fatal(err)
	}
	// z.yaml 设置 log_level = "error" — 最后加载，应胜出
	if err := os.WriteFile(filepath.Join(confDir, "z.yaml"), []byte(`log_level: "error"`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("expected log_level=error (z.yaml wins), got %q", cfg.LogLevel)
	}
}

// 测试5: conf.d 中有无效 yaml = 返回错误
func TestLoadConfDir_InvalidYAML(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `channel: "lanxin"`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "bad.yaml"), []byte(`{{{invalid yaml`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadConfig(mainPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML in conf.d/")
	}
	if !strings.Contains(err.Error(), "解析模块配置") {
		t.Errorf("expected '解析模块配置' in error, got: %v", err)
	}
}

// 测试6: slice 字段（outbound_rules）被完整替换
func TestLoadConfDir_SliceReplacement(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
outbound_rules:
  - name: "rule_a"
    pattern: "aaa"
    action: "block"
  - name: "rule_b"
    pattern: "bbb"
    action: "warn"
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	// conf.d 定义一条新规则 — 应追加到主配置的 2 条之后
	if err := os.WriteFile(filepath.Join(confDir, "rules.yaml"), []byte(`
outbound_rules:
  - name: "rule_x"
    pattern: "xxx"
    action: "block"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.OutboundRules) != 3 {
		t.Fatalf("expected 3 outbound rules (merged), got %d", len(cfg.OutboundRules))
	}
	names := make(map[string]bool)
	for _, r := range cfg.OutboundRules {
		names[r.Name] = true
	}
	if !names["rule_a"] || !names["rule_b"] || !names["rule_x"] {
		t.Errorf("expected rule_a, rule_b, rule_x; got %v", names)
	}
}

// 测试6b: conf.d 多文件同名规则后者覆盖前者（追加合并）
func TestLoadConfDir_SliceMerge_SameNameOverride(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, ``)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	// file1 定义 rule_a=block
	if err := os.WriteFile(filepath.Join(confDir, "01-rules.yaml"), []byte(`
inbound_rules:
  - name: "rule_a"
    patterns: ["aaa"]
    action: "block"
  - name: "rule_b"
    patterns: ["bbb"]
    action: "warn"
`), 0644); err != nil {
		t.Fatal(err)
	}
	// file2 定义 rule_a=review（同名，应覆盖）+ rule_c（新增）
	if err := os.WriteFile(filepath.Join(confDir, "02-override.yaml"), []byte(`
inbound_rules:
  - name: "rule_a"
    patterns: ["aaa"]
    action: "review"
  - name: "rule_c"
    patterns: ["ccc"]
    action: "log"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.InboundRules) != 3 {
		t.Fatalf("expected 3 inbound rules, got %d", len(cfg.InboundRules))
	}
	for _, r := range cfg.InboundRules {
		if r.Name == "rule_a" && r.Action != "review" {
			t.Errorf("rule_a should be overridden to review, got %q", r.Action)
		}
		if r.Name == "rule_b" && r.Action != "warn" {
			t.Errorf("rule_b should remain warn, got %q", r.Action)
		}
		if r.Name == "rule_c" && r.Action != "log" {
			t.Errorf("rule_c should be log, got %q", r.Action)
		}
	}
}

// 测试7: 嵌套结构体（llm_proxy）被合并
func TestLoadConfDir_NestedStructMerge(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
llm_proxy:
  enabled: false
  listen: ":8445"
  timeout_sec: 300
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	// 只覆盖 enabled，其他字段保留主配置值
	if err := os.WriteFile(filepath.Join(confDir, "llm-proxy.yaml"), []byte(`
llm_proxy:
  enabled: true
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.LLMProxy.Enabled {
		t.Error("expected llm_proxy.enabled=true after conf.d override")
	}
	if cfg.LLMProxy.Listen != ":8445" {
		t.Errorf("expected llm_proxy.listen=:8445 (unchanged), got %q", cfg.LLMProxy.Listen)
	}
	if cfg.LLMProxy.TimeoutSec != 300 {
		t.Errorf("expected llm_proxy.timeout_sec=300 (unchanged), got %d", cfg.LLMProxy.TimeoutSec)
	}
}

// 测试8: conf_dir 自定义路径
func TestLoadConfDir_CustomPath(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "my-modules")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "test.yaml"), []byte(`log_level: "error"`), 0644); err != nil {
		t.Fatal(err)
	}
	mainYAML := `
conf_dir: "my-modules"
log_level: "info"
`
	mainPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("expected log_level=error from custom conf_dir, got %q", cfg.LogLevel)
	}
}

// 测试9: conf_dir 绝对路径
func TestLoadConfDir_AbsolutePath(t *testing.T) {
	absDir := t.TempDir()
	confDir := filepath.Join(absDir, "abs-conf")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "test.yaml"), []byte(`channel: "feishu"`), 0644); err != nil {
		t.Fatal(err)
	}
	// main config in a different directory
	mainDir := t.TempDir()
	mainYAML := "conf_dir: " + `"` + confDir + `"` + "\nchannel: \"lanxin\"\n"
	mainPath := filepath.Join(mainDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Channel != "feishu" {
		t.Errorf("expected channel=feishu from absolute conf_dir, got %q", cfg.Channel)
	}
}

// 测试10: .yml 扩展名也能加载
func TestLoadConfDir_YmlExtension(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `log_level: "info"`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "test.yml"), []byte(`log_level: "debug"`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level=debug from .yml file, got %q", cfg.LogLevel)
	}
}

func TestLoadConfDir_StaticUpstreamsMergeByID(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
static_upstreams:
  - id: "up-a"
    address: "127.0.0.1"
    port: 8001
  - id: "up-b"
    address: "127.0.0.2"
    port: 8002
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "upstreams.yaml"), []byte(`
static_upstreams:
  - id: "up-b"
    address: "10.0.0.2"
    port: 9002
  - id: "up-c"
    address: "10.0.0.3"
    port: 9003
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.StaticUpstreams) != 3 {
		t.Fatalf("expected 3 static upstreams, got %d", len(cfg.StaticUpstreams))
	}
	ports := map[string]int{}
	for _, up := range cfg.StaticUpstreams {
		ports[up.ID] = up.Port
	}
	if ports["up-a"] != 8001 || ports["up-b"] != 9002 || ports["up-c"] != 9003 {
		t.Fatalf("unexpected upstream merge result: %#v", ports)
	}
}

func TestLoadConfDir_LLMTargetsMergeByName(t *testing.T) {
	mainPath, confDir := setupTestConfig(t, `
llm_proxy:
  enabled: true
  targets:
    - name: "anthropic"
      upstream: "https://a.example.com"
    - name: "openai"
      upstream: "https://o.example.com"
`)
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "llm-targets.yaml"), []byte(`
llm_proxy:
  targets:
    - name: "openai"
      upstream: "https://override.example.com"
    - name: "gemini"
      upstream: "https://g.example.com"
`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.LLMProxy.Targets) != 3 {
		t.Fatalf("expected 3 llm targets, got %d", len(cfg.LLMProxy.Targets))
	}
	ups := map[string]string{}
	for _, target := range cfg.LLMProxy.Targets {
		ups[target.Name] = target.Upstream
	}
	if ups["anthropic"] != "https://a.example.com" || ups["openai"] != "https://override.example.com" || ups["gemini"] != "https://g.example.com" {
		t.Fatalf("unexpected llm target merge result: %#v", ups)
	}
}
