// k8s_discovery_test.go — K8s 服务发现测试
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockK8sServer 创建模拟 K8s API server
func mockK8sServer(t *testing.T, endpoints k8sEndpoints) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Logf("请求缺少 Authorization header")
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(endpoints)
	}))
}

// setupTestPool 创建测试用的 UpstreamPool（不依赖数据库）
func setupTestPool(t *testing.T) *UpstreamPool {
	return &UpstreamPool{
		upstreams:         make(map[string]*Upstream),
		heartbeatInterval: 10 * time.Second,
		heartbeatTimeout:  3,
	}
}

func TestEndpointsParseNormal(t *testing.T) {
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "gateway-pod-1"}},
					{IP: "10.0.0.2", TargetRef: &k8sObjRef{Name: "gateway-pod-2"}},
					{IP: "10.0.0.3", TargetRef: &k8sObjRef{Name: "gateway-pod-3"}},
				},
				Ports: []k8sPort{
					{Name: "gateway", Port: 8080},
					{Name: "metrics", Port: 9090},
				},
			},
		},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "my-gateway"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.SyncInterval = 15
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	// 验证注册了 3 个上游
	upstreams := pool.ListUpstreams()
	if len(upstreams) != 3 {
		t.Fatalf("期望 3 个上游，实际 %d", len(upstreams))
	}

	// 验证 ID 格式和 tags
	for _, up := range upstreams {
		if up.Tags["source"] != "k8s" {
			t.Errorf("上游 %s 的 source tag 应为 k8s, 实际 %q", up.ID, up.Tags["source"])
		}
		if up.Tags["namespace"] != "default" {
			t.Errorf("上游 %s 的 namespace tag 应为 default, 实际 %q", up.ID, up.Tags["namespace"])
		}
		if up.Port != 8080 {
			t.Errorf("上游 %s 的端口应为 8080, 实际 %d", up.ID, up.Port)
		}
	}

	// 验证发现状态
	status := d.Status()
	if !status.Connected {
		t.Error("应该处于连接状态")
	}
	if status.PodsCount != 3 {
		t.Errorf("期望 3 个 Pod, 实际 %d", status.PodsCount)
	}
}

func TestEndpointsParseEmpty(t *testing.T) {
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "my-gateway"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	upstreams := pool.ListUpstreams()
	if len(upstreams) != 0 {
		t.Fatalf("空 Endpoints 应该注册 0 个上游，实际 %d", len(upstreams))
	}

	status := d.Status()
	if !status.Connected {
		t.Error("空 Endpoints 仍应标记为已连接")
	}
	if status.PodsCount != 0 {
		t.Errorf("期望 0 个 Pod, 实际 %d", status.PodsCount)
	}
}

func TestEndpointsMultiSubset(t *testing.T) {
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "pod-a"}},
				},
				Ports: []k8sPort{
					{Name: "gateway", Port: 8080},
				},
			},
			{
				Addresses: []k8sAddress{
					{IP: "10.0.1.1", TargetRef: &k8sObjRef{Name: "pod-b"}},
					{IP: "10.0.1.2", TargetRef: &k8sObjRef{Name: "pod-c"}},
				},
				Ports: []k8sPort{
					{Name: "gateway", Port: 9090},
				},
			},
		},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "prod"
	cfg.Discovery.Kubernetes.Service = "svc"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	upstreams := pool.ListUpstreams()
	if len(upstreams) != 3 {
		t.Fatalf("多子集应该注册 3 个上游，实际 %d", len(upstreams))
	}
}

func TestPodAppearAndDisappear(t *testing.T) {
	// 第一次同步：2 个 Pod
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "pod-1"}},
					{IP: "10.0.0.2", TargetRef: &k8sObjRef{Name: "pod-2"}},
				},
				Ports: []k8sPort{{Name: "gateway", Port: 8080}},
			},
		},
	}

	currentEndpoints := &endpoints
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(*currentEndpoints)
	}))
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "my-svc"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	// 第一次同步
	d.sync()
	if len(pool.ListUpstreams()) != 2 {
		t.Fatalf("第一次同步应注册 2 个上游, 实际 %d", len(pool.ListUpstreams()))
	}

	// 第二次同步：Pod-2 消失，Pod-3 出现
	*currentEndpoints = k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "pod-1"}},
					{IP: "10.0.0.3", TargetRef: &k8sObjRef{Name: "pod-3"}},
				},
				Ports: []k8sPort{{Name: "gateway", Port: 8080}},
			},
		},
	}

	d.sync()
	upstreams := pool.ListUpstreams()
	if len(upstreams) != 2 {
		t.Fatalf("第二次同步后应有 2 个上游, 实际 %d", len(upstreams))
	}

	// 验证 pod-2 已移除，pod-3 已添加
	ids := map[string]bool{}
	for _, up := range upstreams {
		ids[up.ID] = true
	}
	if ids["k8s-default-pod-2"] {
		t.Error("pod-2 应该已被移除")
	}
	if !ids["k8s-default-pod-3"] {
		t.Error("pod-3 应该已被注册")
	}
	if !ids["k8s-default-pod-1"] {
		t.Error("pod-1 应该仍然存在")
	}
}

func TestPortNameFilter(t *testing.T) {
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "pod-1"}},
				},
				Ports: []k8sPort{
					{Name: "metrics", Port: 9090},
					{Name: "grpc", Port: 50051},
				},
			},
		},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "my-svc"
	cfg.Discovery.Kubernetes.PortName = "gateway" // 不匹配任何端口
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	// "gateway" 端口不存在，且有 2 个端口，所以不使用单端口回退
	upstreams := pool.ListUpstreams()
	if len(upstreams) != 0 {
		t.Fatalf("端口名不匹配时不应注册上游（多端口场景），实际 %d", len(upstreams))
	}
}

func TestSinglePortFallback(t *testing.T) {
	// 只有一个端口时，即使端口名不匹配也应该使用
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.1", TargetRef: &k8sObjRef{Name: "pod-1"}},
				},
				Ports: []k8sPort{
					{Name: "http", Port: 80},
				},
			},
		},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "my-svc"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	upstreams := pool.ListUpstreams()
	if len(upstreams) != 1 {
		t.Fatalf("单端口回退应注册 1 个上游, 实际 %d", len(upstreams))
	}
	if upstreams[0].Port != 80 {
		t.Errorf("端口应为 80, 实际 %d", upstreams[0].Port)
	}
}

func TestInClusterTokenMock(t *testing.T) {
	// 模拟 InCluster 文件
	tmpDir := t.TempDir()
	saDir := filepath.Join(tmpDir, "var", "run", "secrets", "kubernetes.io", "serviceaccount")
	os.MkdirAll(saDir, 0755)
	os.WriteFile(filepath.Join(saDir, "token"), []byte("mock-sa-token"), 0644)
	os.WriteFile(filepath.Join(saDir, "ca.crt"), []byte("-----BEGIN CERTIFICATE-----\nMIIC\n-----END CERTIFICATE-----"), 0644)

	// 验证文件可读取
	tokenBytes, err := os.ReadFile(filepath.Join(saDir, "token"))
	if err != nil {
		t.Fatalf("读取 mock token 失败: %v", err)
	}
	if string(tokenBytes) != "mock-sa-token" {
		t.Errorf("token 不匹配: %q", string(tokenBytes))
	}

	caBytes, err := os.ReadFile(filepath.Join(saDir, "ca.crt"))
	if err != nil {
		t.Fatalf("读取 mock ca.crt 失败: %v", err)
	}
	if len(caBytes) == 0 {
		t.Error("ca.crt 应不为空")
	}
}

func TestDiscoveryStatus(t *testing.T) {
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "prod"
	cfg.Discovery.Kubernetes.Service = "gateway-svc"
	cfg.Discovery.Kubernetes.PortName = "http"

	d := &K8sDiscovery{
		cfg:           cfg,
		discoveredIDs: make(map[string]bool),
	}

	status := d.Status()
	if !status.Enabled {
		t.Error("应该报告 enabled=true")
	}
	if status.Namespace != "prod" {
		t.Errorf("期望 namespace=prod, 实际 %q", status.Namespace)
	}
	if status.Service != "gateway-svc" {
		t.Errorf("期望 service=gateway-svc, 实际 %q", status.Service)
	}
	if status.PortName != "http" {
		t.Errorf("期望 port_name=http, 实际 %q", status.PortName)
	}
	if status.Mode != "incluster" {
		t.Errorf("期望 mode=incluster, 实际 %q", status.Mode)
	}
	if status.Connected {
		t.Error("初始时不应该已连接")
	}
}

func TestDiscoveryStatusKubeconfig(t *testing.T) {
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Kubeconfig = "/home/user/.kube/config"
	cfg.Discovery.Kubernetes.Namespace = "staging"
	cfg.Discovery.Kubernetes.Service = "api"

	d := &K8sDiscovery{
		cfg:           cfg,
		discoveredIDs: make(map[string]bool),
	}

	status := d.Status()
	if status.Mode != "kubeconfig" {
		t.Errorf("期望 mode=kubeconfig, 实际 %q", status.Mode)
	}
}

func TestNoPodNameFallbackID(t *testing.T) {
	// 当 TargetRef 为 nil 时，ID 应使用 IP:Port 格式
	endpoints := k8sEndpoints{
		Subsets: []k8sSubset{
			{
				Addresses: []k8sAddress{
					{IP: "10.0.0.5", TargetRef: nil},
				},
				Ports: []k8sPort{
					{Name: "gateway", Port: 8080},
				},
			},
		},
	}

	ts := mockK8sServer(t, endpoints)
	defer ts.Close()

	pool := setupTestPool(t)
	cfg := &Config{}
	cfg.Discovery.Kubernetes.Enabled = true
	cfg.Discovery.Kubernetes.Namespace = "default"
	cfg.Discovery.Kubernetes.Service = "svc"
	cfg.Discovery.Kubernetes.PortName = "gateway"
	cfg.Discovery.Kubernetes.InsecureSkipVerify = true

	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		baseURL:       ts.URL,
		token:         "test-token",
		httpClient:    ts.Client(),
		discoveredIDs: make(map[string]bool),
	}

	d.sync()

	upstreams := pool.ListUpstreams()
	if len(upstreams) != 1 {
		t.Fatalf("期望 1 个上游, 实际 %d", len(upstreams))
	}

	expected := "k8s-10.0.0.5-8080"
	if upstreams[0].ID != expected {
		t.Errorf("期望 ID=%q, 实际 %q", expected, upstreams[0].ID)
	}
}
