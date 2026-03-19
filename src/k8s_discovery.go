// k8s_discovery.go — K8s 服务发现模块（零外部依赖，直接调 K8s REST API）
// lobster-guard v21.0
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ============================================================
// K8s Endpoints JSON 结构（只解析需要的字段）
// ============================================================

type k8sEndpoints struct {
	Subsets []k8sSubset `json:"subsets"`
}

type k8sSubset struct {
	Addresses []k8sAddress `json:"addresses"`
	Ports     []k8sPort    `json:"ports"`
}

type k8sAddress struct {
	IP        string     `json:"ip"`
	TargetRef *k8sObjRef `json:"targetRef"`
}

type k8sObjRef struct {
	Name string `json:"name"` // Pod name
}

type k8sPort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// ============================================================
// Kubeconfig 结构（简化，只支持 current-context）
// ============================================================

type kubeConfig struct {
	CurrentContext string              `yaml:"current-context"`
	Clusters       []kubeClusterEntry  `yaml:"clusters"`
	Users          []kubeUserEntry     `yaml:"users"`
	Contexts       []kubeContextEntry  `yaml:"contexts"`
}

type kubeClusterEntry struct {
	Name    string      `yaml:"name"`
	Cluster kubeCluster `yaml:"cluster"`
}

type kubeCluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthority     string `yaml:"certificate-authority"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
}

type kubeUserEntry struct {
	Name string   `yaml:"name"`
	User kubeUser `yaml:"user"`
}

type kubeUser struct {
	Token                string `yaml:"token"`
	ClientCertificate     string `yaml:"client-certificate"`
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKey             string `yaml:"client-key"`
	ClientKeyData         string `yaml:"client-key-data"`
}

type kubeContextEntry struct {
	Name    string      `yaml:"name"`
	Context kubeContext `yaml:"context"`
}

type kubeContext struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace"`
}

// ============================================================
// K8sDiscovery 主结构
// ============================================================

// K8sDiscovery 通过 K8s API 自动发现上游服务
type K8sDiscovery struct {
	cfg        *Config
	pool       *UpstreamPool
	httpClient *http.Client
	baseURL    string // K8s API server URL
	token      string // ServiceAccount token
	caCert     []byte // CA 证书

	mu            sync.RWMutex
	lastSync      time.Time
	lastError     string
	connected     bool
	discoveredIDs map[string]bool // 当前发现的上游 ID 集合
	podsCount     int
}

// K8sDiscoveryStatus 发现状态（API 返回用）
type K8sDiscoveryStatus struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	LastSync  string `json:"last_sync"`
	LastError string `json:"last_error,omitempty"`
	PodsCount int    `json:"pods_count"`
	Namespace string `json:"namespace"`
	Service   string `json:"service"`
	PortName  string `json:"port_name"`
	Mode      string `json:"mode"` // "incluster" or "kubeconfig"
}

// NewK8sDiscovery 创建 K8s 发现实例
func NewK8sDiscovery(cfg *Config, pool *UpstreamPool) (*K8sDiscovery, error) {
	d := &K8sDiscovery{
		cfg:           cfg,
		pool:          pool,
		discoveredIDs: make(map[string]bool),
	}

	// 初始化连接参数
	if err := d.initConnection(); err != nil {
		return nil, fmt.Errorf("K8s 发现初始化失败: %w", err)
	}

	return d, nil
}

// initConnection 初始化 K8s API 连接参数
func (d *K8sDiscovery) initConnection() error {
	k8sCfg := d.cfg.Discovery.Kubernetes

	if k8sCfg.Kubeconfig != "" {
		// Kubeconfig 模式
		return d.initFromKubeconfig(k8sCfg.Kubeconfig)
	}

	// InCluster 模式
	return d.initInCluster()
}

// initInCluster 从 InCluster 环境初始化
func (d *K8sDiscovery) initInCluster() error {
	// 读取 ServiceAccount token
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return fmt.Errorf("读取 SA token 失败: %w", err)
	}
	d.token = strings.TrimSpace(string(tokenBytes))

	// 读取 CA 证书
	caBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}
	d.caCert = caBytes

	// API server 地址
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return fmt.Errorf("KUBERNETES_SERVICE_HOST/PORT 环境变量未设置")
	}
	d.baseURL = fmt.Sprintf("https://%s:%s", host, port)

	// 创建 HTTP 客户端
	d.httpClient = d.createHTTPClient()

	log.Printf("[K8s发现] InCluster 模式初始化: API=%s", d.baseURL)
	return nil
}

// initFromKubeconfig 从 kubeconfig 文件初始化
func (d *K8sDiscovery) initFromKubeconfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 kubeconfig 失败: %w", err)
	}

	var kc kubeConfig
	if err := yaml.Unmarshal(data, &kc); err != nil {
		return fmt.Errorf("解析 kubeconfig 失败: %w", err)
	}

	if kc.CurrentContext == "" {
		return fmt.Errorf("kubeconfig 缺少 current-context")
	}

	// 查找 current-context
	var ctxEntry *kubeContextEntry
	for i := range kc.Contexts {
		if kc.Contexts[i].Name == kc.CurrentContext {
			ctxEntry = &kc.Contexts[i]
			break
		}
	}
	if ctxEntry == nil {
		return fmt.Errorf("kubeconfig 找不到 context: %s", kc.CurrentContext)
	}

	// 查找 cluster
	var clusterEntry *kubeClusterEntry
	for i := range kc.Clusters {
		if kc.Clusters[i].Name == ctxEntry.Context.Cluster {
			clusterEntry = &kc.Clusters[i]
			break
		}
	}
	if clusterEntry == nil {
		return fmt.Errorf("kubeconfig 找不到 cluster: %s", ctxEntry.Context.Cluster)
	}

	// 查找 user
	var userEntry *kubeUserEntry
	for i := range kc.Users {
		if kc.Users[i].Name == ctxEntry.Context.User {
			userEntry = &kc.Users[i]
			break
		}
	}
	if userEntry == nil {
		return fmt.Errorf("kubeconfig 找不到 user: %s", ctxEntry.Context.User)
	}

	d.baseURL = clusterEntry.Cluster.Server
	d.token = userEntry.User.Token

	// 解析 CA
	if clusterEntry.Cluster.CertificateAuthorityData != "" {
		caBytes, err := base64.StdEncoding.DecodeString(clusterEntry.Cluster.CertificateAuthorityData)
		if err != nil {
			return fmt.Errorf("解码 CA 数据失败: %w", err)
		}
		d.caCert = caBytes
	} else if clusterEntry.Cluster.CertificateAuthority != "" {
		caBytes, err := os.ReadFile(clusterEntry.Cluster.CertificateAuthority)
		if err != nil {
			return fmt.Errorf("读取 CA 文件失败: %w", err)
		}
		d.caCert = caBytes
	}

	d.httpClient = d.createHTTPClient()

	log.Printf("[K8s发现] Kubeconfig 模式初始化: API=%s, context=%s", d.baseURL, kc.CurrentContext)
	return nil
}

// createHTTPClient 创建带 CA 验证的 HTTP 客户端
func (d *K8sDiscovery) createHTTPClient() *http.Client {
	tlsCfg := &tls.Config{}

	if d.cfg.Discovery.Kubernetes.InsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	} else if len(d.caCert) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(d.caCert)
		tlsCfg.RootCAs = pool
	}

	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
}

// Run 启动同步循环
func (d *K8sDiscovery) Run(ctx context.Context) {
	k8sCfg := d.cfg.Discovery.Kubernetes
	interval := k8sCfg.SyncInterval
	if interval <= 0 {
		interval = 15
	}

	log.Printf("[K8s发现] 启动同步循环: namespace=%s, service=%s, interval=%ds",
		k8sCfg.Namespace, k8sCfg.Service, interval)

	// 立即执行一次同步
	d.sync()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[K8s发现] 同步循环已停止")
			return
		case <-ticker.C:
			d.sync()
		}
	}
}

// sync 执行一次同步
func (d *K8sDiscovery) sync() {
	k8sCfg := d.cfg.Discovery.Kubernetes
	ns := k8sCfg.Namespace
	svc := k8sCfg.Service

	if ns == "" || svc == "" {
		d.mu.Lock()
		d.lastError = "namespace 或 service 未配置"
		d.connected = false
		d.mu.Unlock()
		return
	}

	// 调用 K8s API 获取 Endpoints
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/endpoints/%s", d.baseURL, ns, svc)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		d.setError(fmt.Sprintf("创建请求失败: %v", err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.setError(fmt.Sprintf("请求 K8s API 失败: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // max 1MB
	if err != nil {
		d.setError(fmt.Sprintf("读取响应失败: %v", err))
		return
	}

	if resp.StatusCode != 200 {
		d.setError(fmt.Sprintf("K8s API 返回 %d: %s", resp.StatusCode, string(body)))
		return
	}

	// 解析 Endpoints
	var endpoints k8sEndpoints
	if err := json.Unmarshal(body, &endpoints); err != nil {
		d.setError(fmt.Sprintf("解析 Endpoints JSON 失败: %v", err))
		return
	}

	// 提取 Pod IP:Port
	portName := k8sCfg.PortName
	if portName == "" {
		portName = "gateway"
	}

	type podInfo struct {
		ID      string
		IP      string
		Port    int
		PodName string
	}

	var discovered []podInfo
	for _, subset := range endpoints.Subsets {
		// 找到匹配的端口
		targetPort := 0
		for _, p := range subset.Ports {
			if p.Name == portName {
				targetPort = p.Port
				break
			}
		}
		// 如果只有一个端口，不管名字是否匹配都用它
		if targetPort == 0 && len(subset.Ports) == 1 {
			targetPort = subset.Ports[0].Port
		}
		if targetPort == 0 {
			continue
		}

		for _, addr := range subset.Addresses {
			podName := ""
			if addr.TargetRef != nil {
				podName = addr.TargetRef.Name
			}

			id := fmt.Sprintf("k8s-%s-%s", ns, podName)
			if podName == "" {
				id = fmt.Sprintf("k8s-%s-%d", addr.IP, targetPort)
			}

			discovered = append(discovered, podInfo{
				ID:      id,
				IP:      addr.IP,
				Port:    targetPort,
				PodName: podName,
			})
		}
	}

	// 对比当前 pool，更新
	d.mu.Lock()
	currentIDs := make(map[string]bool)
	for id := range d.discoveredIDs {
		currentIDs[id] = true
	}
	newDiscoveredIDs := make(map[string]bool)
	d.mu.Unlock()

	// 注册新发现的 / 保活已存在的
	for _, pod := range discovered {
		newDiscoveredIDs[pod.ID] = true

		tags := map[string]string{
			"source":    "k8s",
			"pod":       pod.PodName,
			"namespace": ns,
			"service":   svc,
		}

		if !currentIDs[pod.ID] {
			// 新发现
			if err := d.pool.Register(pod.ID, pod.IP, pod.Port, tags); err != nil {
				log.Printf("[K8s发现] 注册上游失败: %s %v", pod.ID, err)
			} else {
				log.Printf("[K8s发现] ✅ 发现新 Pod: %s -> %s:%d (pod=%s)", pod.ID, pod.IP, pod.Port, pod.PodName)
			}
		} else {
			// 已存在，保活 + 更新地址（Pod 可能重启了 IP 变了）
			d.pool.Heartbeat(pod.ID, nil)
			// 更新地址和 tags
			d.pool.Update(pod.ID, pod.IP, pod.Port, tags)
		}
	}

	// 移除消失的
	for id := range currentIDs {
		if !newDiscoveredIDs[id] {
			d.pool.ForceDeregister(id)
			log.Printf("[K8s发现] ❌ Pod 消失，移除上游: %s", id)
		}
	}

	// 更新状态
	d.mu.Lock()
	d.discoveredIDs = newDiscoveredIDs
	d.lastSync = time.Now()
	d.lastError = ""
	d.connected = true
	d.podsCount = len(discovered)
	d.mu.Unlock()

	if len(discovered) > 0 {
		log.Printf("[K8s发现] 同步完成: %d 个 Pod (ns=%s, svc=%s)", len(discovered), ns, svc)
	}
}

// setError 设置错误状态
func (d *K8sDiscovery) setError(msg string) {
	d.mu.Lock()
	d.lastError = msg
	d.connected = false
	d.mu.Unlock()
	log.Printf("[K8s发现] ⚠️ %s", msg)
}

// Status 返回发现状态
func (d *K8sDiscovery) Status() K8sDiscoveryStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	k8sCfg := d.cfg.Discovery.Kubernetes
	mode := "incluster"
	if k8sCfg.Kubeconfig != "" {
		mode = "kubeconfig"
	}

	portName := k8sCfg.PortName
	if portName == "" {
		portName = "gateway"
	}

	lastSync := ""
	if !d.lastSync.IsZero() {
		lastSync = d.lastSync.Format(time.RFC3339)
	}

	return K8sDiscoveryStatus{
		Enabled:   k8sCfg.Enabled,
		Connected: d.connected,
		LastSync:  lastSync,
		LastError: d.lastError,
		PodsCount: d.podsCount,
		Namespace: k8sCfg.Namespace,
		Service:   k8sCfg.Service,
		PortName:  portName,
		Mode:      mode,
	}
}
