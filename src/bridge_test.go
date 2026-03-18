// bridge_test.go — Bridge Mode 测试（飞书/钉钉 WSS 桥接）
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================================
// Bridge Mode 单元测试 (v3.2)
// ============================================================

func TestBridgeStatus(t *testing.T) {
	bs := BridgeStatus{
		Connected:    true,
		ConnectedAt:  time.Now(),
		Reconnects:   3,
		LastError:    "test error",
		LastMessage:  time.Now(),
		MessageCount: 42,
	}
	data, err := json.Marshal(bs)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var bs2 BridgeStatus
	if err := json.Unmarshal(data, &bs2); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if bs2.Connected != true {
		t.Error("Connected 应为 true")
	}
	if bs2.Reconnects != 3 {
		t.Errorf("Reconnects 期望 3，实际 %d", bs2.Reconnects)
	}
	if bs2.MessageCount != 42 {
		t.Errorf("MessageCount 期望 42，实际 %d", bs2.MessageCount)
	}
	if bs2.LastError != "test error" {
		t.Errorf("LastError 期望 'test error'，实际 %q", bs2.LastError)
	}
}

func TestFeishuBridge_TokenRefresh(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != "POST" {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["app_id"] != "test_app_id" || req["app_secret"] != "test_app_secret" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 10003,
				"msg":  "invalid app_id or app_secret",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": fmt.Sprintf("test_token_%d", callCount),
			"expire":              7200,
		})
	}))
	defer server.Close()

	// 模拟获取 token（通过 httptest 服务器）
	client := server.Client()

	// 正常请求
	body, _ := json.Marshal(map[string]string{
		"app_id": "test_app_id", "app_secret": "test_app_secret",
	})
	resp, err := client.Post(server.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	var result struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 0 {
		t.Fatalf("code 期望 0，实际 %d", result.Code)
	}
	if result.TenantAccessToken != "test_token_1" {
		t.Errorf("token 期望 'test_token_1'，实际 %q", result.TenantAccessToken)
	}

	// 第二次请求验证缓存/递增
	body2, _ := json.Marshal(map[string]string{
		"app_id": "test_app_id", "app_secret": "test_app_secret",
	})
	resp2, _ := client.Post(server.URL, "application/json", bytes.NewReader(body2))
	defer resp2.Body.Close()
	var result2 struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	json.NewDecoder(resp2.Body).Decode(&result2)
	if result2.TenantAccessToken != "test_token_2" {
		t.Errorf("第二次 token 期望 'test_token_2'，实际 %q", result2.TenantAccessToken)
	}

	// 错误 credential
	bodyBad, _ := json.Marshal(map[string]string{
		"app_id": "wrong", "app_secret": "wrong",
	})
	resp3, _ := client.Post(server.URL, "application/json", bytes.NewReader(bodyBad))
	defer resp3.Body.Close()
	var result3 struct {
		Code int `json:"code"`
	}
	json.NewDecoder(resp3.Body).Decode(&result3)
	if result3.Code == 0 {
		t.Error("错误 credential 应返回非零 code")
	}
}

func TestDingtalkBridge_TicketAcquire(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if req["clientId"] != "test_client_id" || req["clientSecret"] != "test_client_secret" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"endpoint": "",
				"ticket":   "",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"endpoint": "wss://test.dingtalk.com/ws",
			"ticket":   "test_ticket_abc123",
		})
	}))
	defer server.Close()

	client := server.Client()
	reqBody, _ := json.Marshal(map[string]interface{}{
		"clientId":     "test_client_id",
		"clientSecret": "test_client_secret",
	})
	resp, err := client.Post(server.URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	var result struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Endpoint != "wss://test.dingtalk.com/ws" {
		t.Errorf("endpoint 期望 'wss://test.dingtalk.com/ws'，实际 %q", result.Endpoint)
	}
	if result.Ticket != "test_ticket_abc123" {
		t.Errorf("ticket 期望 'test_ticket_abc123'，实际 %q", result.Ticket)
	}

	// 错误 credential
	reqBad, _ := json.Marshal(map[string]interface{}{
		"clientId": "wrong", "clientSecret": "wrong",
	})
	resp2, _ := client.Post(server.URL, "application/json", bytes.NewReader(reqBad))
	defer resp2.Body.Close()
	var result2 struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	}
	json.NewDecoder(resp2.Body).Decode(&result2)
	if result2.Endpoint != "" || result2.Ticket != "" {
		t.Error("错误 credential 应返回空 endpoint/ticket")
	}
}

