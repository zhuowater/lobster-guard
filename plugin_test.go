// plugin_test.go — 通道插件测试（蓝信/飞书/钉钉/企微/通用HTTP）
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================
// Channel Plugin 单元测试 (v3.2)
// ============================================================

// === 飞书加密辅助函数 ===
func feishuEncrypt(t *testing.T, encryptKey string, plaintext []byte) string {
	t.Helper()
	keyHash := sha256.Sum256([]byte(encryptKey))
	key := keyHash[:32]
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("AES cipher: %v", err)
	}
	blockSize := aes.BlockSize
	padding := blockSize - (len(plaintext) % blockSize)
	padtext := append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
	iv := []byte("1234567890abcdef")
	ciphertext := make([]byte, len(padtext))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padtext)
	result := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(result)
}

func TestFeishuPlugin_ParseInbound(t *testing.T) {
	encryptKey := "test_feishu_encrypt_key_123"
	fp := NewFeishuPlugin(encryptKey, "test_verification_token")

	t.Run("正常消息解析", func(t *testing.T) {
		plainJSON := `{"header":{"event_type":"im.message.receive_v1"},"event":{"sender":{"sender_id":{"open_id":"ou_test123"}},"message":{"content":"{\"text\":\"你好飞书\"}"}}}`
		encrypted := feishuEncrypt(t, encryptKey, []byte(plainJSON))
		body, _ := json.Marshal(map[string]string{"encrypt": encrypted})
		msg, err := fp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "你好飞书" {
			t.Errorf("文本期望 '你好飞书'，实际 %q", msg.Text)
		}
		if msg.SenderID != "ou_test123" {
			t.Errorf("发送者期望 'ou_test123'，实际 %q", msg.SenderID)
		}
		if msg.EventType != "im.message.receive_v1" {
			t.Errorf("事件类型期望 'im.message.receive_v1'，实际 %q", msg.EventType)
		}
		if msg.IsVerify {
			t.Error("IsVerify 应为 false")
		}
	})

	t.Run("URL Verification 加密", func(t *testing.T) {
		verifyJSON := `{"type":"url_verification","challenge":"test_challenge_abc","token":"test_token"}`
		encrypted := feishuEncrypt(t, encryptKey, []byte(verifyJSON))
		body, _ := json.Marshal(map[string]string{"encrypt": encrypted})
		msg, err := fp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.EventType != "url_verification" {
			t.Errorf("事件类型期望 'url_verification'，实际 %q", msg.EventType)
		}
		if !msg.IsVerify {
			t.Error("IsVerify 应为 true")
		}
		if msg.VerifyReply == nil {
			t.Fatal("VerifyReply 不应为 nil")
		}
		var resp map[string]string
		json.Unmarshal(msg.VerifyReply, &resp)
		if resp["challenge"] != "test_challenge_abc" {
			t.Errorf("challenge 期望 'test_challenge_abc'，实际 %q", resp["challenge"])
		}
	})

	t.Run("URL Verification 明文", func(t *testing.T) {
		body := []byte(`{"type":"url_verification","challenge":"plain_challenge","token":"xxx"}`)
		msg, err := fp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if !msg.IsVerify {
			t.Error("IsVerify 应为 true")
		}
		var resp map[string]string
		json.Unmarshal(msg.VerifyReply, &resp)
		if resp["challenge"] != "plain_challenge" {
			t.Errorf("challenge 期望 'plain_challenge'，实际 %q", resp["challenge"])
		}
	})

	t.Run("无效 encrypt 数据", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"encrypt": "not_valid_base64!!!"})
		_, err := fp.ParseInbound(body)
		if err == nil {
			t.Fatal("无效 encrypt 应返回错误")
		}
	})
}

func TestFeishuPlugin_ExtractOutbound(t *testing.T) {
	fp := NewFeishuPlugin("key", "token")

	t.Run("飞书出站消息文本提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"content": `{"text":"Hello from feishu"}`,
		})
		text, ok := fp.ExtractOutbound("/open-apis/im/v1/messages", body)
		if !ok {
			t.Fatal("应返回 ok=true")
		}
		if text != "Hello from feishu" {
			t.Errorf("文本期望 'Hello from feishu'，实际 %q", text)
		}
	})

	t.Run("ShouldAuditOutbound 路径匹配", func(t *testing.T) {
		if !fp.ShouldAuditOutbound("/open-apis/im/v1/messages") {
			t.Error("应审计 /open-apis/im/v1/messages")
		}
		if !fp.ShouldAuditOutbound("/open-apis/im/v1/messages/reply") {
			t.Error("应审计前缀匹配路径")
		}
		if fp.ShouldAuditOutbound("/open-apis/auth/v3/token") {
			t.Error("不应审计 auth 路径")
		}
	})
}

// === 钉钉加密辅助函数 ===
func dingtalkEncrypt(t *testing.T, aesKeyBase64, corpId string, plaintext []byte) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64 + "=")
	if err != nil {
		t.Fatalf("解码 aesKey: %v", err)
	}
	aesKey := decoded[:32]
	random := []byte("1234567890123456")
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(plaintext)))
	data := append(random, msgLen...)
	data = append(data, plaintext...)
	data = append(data, []byte(corpId)...)
	blockSize := aes.BlockSize
	paddingSize := blockSize - (len(data) % blockSize)
	data = append(data, bytes.Repeat([]byte{byte(paddingSize)}, paddingSize)...)
	block, _ := aes.NewCipher(aesKey)
	iv := aesKey[:16]
	ct := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, data)
	return base64.StdEncoding.EncodeToString(ct)
}

func TestDingtalkPlugin_ParseInbound(t *testing.T) {
	aesKeyBase64 := "ODk0RUVBMEEyRjhDOThGQjhFOTAwNjdGODFFN0IwQUY"
	token := "test_ding_token"
	corpId := "ding_corp_123"
	dp := NewDingtalkPlugin(token, aesKeyBase64, corpId)

	t.Run("正常消息解析（明文）", func(t *testing.T) {
		body := []byte(`{"msgtype":"text","text":{"content":"钉钉消息"},"senderStaffId":"staff_001"}`)
		msg, err := dp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "钉钉消息" {
			t.Errorf("文本期望 '钉钉消息'，实际 %q", msg.Text)
		}
		if msg.SenderID != "staff_001" {
			t.Errorf("发送者期望 'staff_001'，实际 %q", msg.SenderID)
		}
		if msg.EventType != "text" {
			t.Errorf("事件类型期望 'text'，实际 %q", msg.EventType)
		}
	})

	t.Run("加密消息解析", func(t *testing.T) {
		plainJSON := `{"msgtype":"text","text":{"content":"加密钉钉"},"senderStaffId":"staff_002"}`
		encrypted := dingtalkEncrypt(t, aesKeyBase64, corpId, []byte(plainJSON))
		body, _ := json.Marshal(map[string]string{"encrypt": encrypted})
		msg, err := dp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "加密钉钉" {
			t.Errorf("文本期望 '加密钉钉'，实际 %q", msg.Text)
		}
		if msg.SenderID != "staff_002" {
			t.Errorf("发送者期望 'staff_002'，实际 %q", msg.SenderID)
		}
	})

	t.Run("签名校验-正确签名", func(t *testing.T) {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := ts + "\n" + token
		mac := hmac.New(sha256.New, []byte(token))
		mac.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if !dp.dingtalkVerifySign(ts, sign) {
			t.Error("正确签名应通过")
		}
	})

	t.Run("签名校验-错误签名", func(t *testing.T) {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		if dp.dingtalkVerifySign(ts, "wrong_sign_value") {
			t.Error("错误签名不应通过")
		}
	})
}

func TestDingtalkPlugin_ExtractOutbound(t *testing.T) {
	dp := NewDingtalkPlugin("token", "", "corp")

	t.Run("text.content 提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": "钉钉出站消息"},
		})
		text, ok := dp.ExtractOutbound("/robot/send", body)
		if !ok {
			t.Fatal("应返回 ok=true")
		}
		if text != "钉钉出站消息" {
			t.Errorf("文本期望 '钉钉出站消息'，实际 %q", text)
		}
	})

	t.Run("markdown.text 提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"text": "# 标题\n内容"},
		})
		text, _ := dp.ExtractOutbound("/robot/send", body)
		if text != "# 标题\n内容" {
			t.Errorf("文本提取错误: %q", text)
		}
	})

	t.Run("ShouldAuditOutbound", func(t *testing.T) {
		if !dp.ShouldAuditOutbound("/robot/send") {
			t.Error("应审计 /robot/send")
		}
		if dp.ShouldAuditOutbound("/user/get") {
			t.Error("不应审计 /user/get")
		}
	})
}

// === 企微加密辅助函数 ===
func wecomEncrypt(t *testing.T, encodingAesKeyBase64, corpId string, plaintext []byte) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(encodingAesKeyBase64 + "=")
	if err != nil {
		t.Fatalf("解码 aesKey: %v", err)
	}
	aesKey := decoded[:32]
	random := []byte("abcdefghijklmnop")
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(plaintext)))
	data := append(random, msgLen...)
	data = append(data, plaintext...)
	data = append(data, []byte(corpId)...)
	blockSize := aes.BlockSize
	paddingSize := blockSize - (len(data) % blockSize)
	data = append(data, bytes.Repeat([]byte{byte(paddingSize)}, paddingSize)...)
	block, _ := aes.NewCipher(aesKey)
	iv := aesKey[:16]
	ct := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, data)
	return base64.StdEncoding.EncodeToString(ct)
}

func wecomSign(token string, timestamp, nonce, encryptMsg string) string {
	parts := []string{token, timestamp, nonce, encryptMsg}
	sort.Strings(parts)
	h := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", h)
}

func TestWecomPlugin_ParseInbound(t *testing.T) {
	token := "test_wecom_token"
	aesKeyBase64 := "ODk0RUVBMEEyRjhDOThGQjhFOTAwNjdGODFFN0IwQUY"
	corpId := "ww_corp_123"
	wp := NewWecomPlugin(token, aesKeyBase64, corpId)

	t.Run("XML 消息解析", func(t *testing.T) {
		innerXML := `<xml><ToUserName><![CDATA[ww_corp_123]]></ToUserName><FromUserName><![CDATA[user_001]]></FromUserName><CreateTime>1234567890</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[企微消息内容]]></Content><MsgId>1234</MsgId><AgentID>1000001</AgentID></xml>`
		encrypted := wecomEncrypt(t, aesKeyBase64, corpId, []byte(innerXML))
		envelope := fmt.Sprintf(`<xml><Encrypt><![CDATA[%s]]></Encrypt><ToUserName><![CDATA[ww_corp_123]]></ToUserName><AgentID><![CDATA[1000001]]></AgentID></xml>`, encrypted)
		msg, err := wp.ParseInbound([]byte(envelope))
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "企微消息内容" {
			t.Errorf("文本期望 '企微消息内容'，实际 %q", msg.Text)
		}
		if msg.SenderID != "user_001" {
			t.Errorf("发送者期望 'user_001'，实际 %q", msg.SenderID)
		}
		if msg.EventType != "text" {
			t.Errorf("事件类型期望 'text'，实际 %q", msg.EventType)
		}
	})

	t.Run("签名校验", func(t *testing.T) {
		timestamp := "1234567890"
		nonce := "test_nonce"
		encryptMsg := "test_encrypt_data"
		sig := wecomSign(token, timestamp, nonce, encryptMsg)
		if !wp.wecomVerifySignature(sig, timestamp, nonce, encryptMsg) {
			t.Error("正确签名应通过")
		}
		if wp.wecomVerifySignature("wrong_signature", timestamp, nonce, encryptMsg) {
			t.Error("错误签名不应通过")
		}
	})

	t.Run("空加密消息", func(t *testing.T) {
		envelope := `<xml><Encrypt></Encrypt></xml>`
		_, err := wp.ParseInbound([]byte(envelope))
		if err == nil {
			t.Fatal("空加密消息应返回错误")
		}
	})
}

func TestWecomPlugin_VerifyURL(t *testing.T) {
	token := "test_wecom_token"
	aesKeyBase64 := "ODk0RUVBMEEyRjhDOThGQjhFOTAwNjdGODFFN0IwQUY"
	corpId := "ww_corp_123"
	wp := NewWecomPlugin(token, aesKeyBase64, corpId)

	t.Run("GET 验证成功", func(t *testing.T) {
		echoStr := wecomEncrypt(t, aesKeyBase64, corpId, []byte("echo_test_12345"))
		timestamp := "1234567890"
		nonce := "test_nonce_verify"
		msgSignature := wecomSign(token, timestamp, nonce, echoStr)
		result, err := wp.VerifyURL(msgSignature, timestamp, nonce, echoStr)
		if err != nil {
			t.Fatalf("VerifyURL 失败: %v", err)
		}
		if result != "echo_test_12345" {
			t.Errorf("echostr 解密期望 'echo_test_12345'，实际 %q", result)
		}
	})

	t.Run("GET 验证签名错误", func(t *testing.T) {
		echoStr := wecomEncrypt(t, aesKeyBase64, corpId, []byte("test"))
		_, err := wp.VerifyURL("wrong_sig", "123", "nonce", echoStr)
		if err == nil {
			t.Fatal("签名错误应返回 error")
		}
	})

	t.Run("GET 验证 echostr 解密失败", func(t *testing.T) {
		badEchoStr := "not_valid_base64!!!"
		timestamp := "123"
		nonce := "n"
		sig := wecomSign(token, timestamp, nonce, badEchoStr)
		_, err := wp.VerifyURL(sig, timestamp, nonce, badEchoStr)
		if err == nil {
			t.Fatal("无效 echostr 应返回 error")
		}
	})
}

func TestWecomPlugin_ExtractOutbound(t *testing.T) {
	wp := NewWecomPlugin("token", "", "corp")

	t.Run("text.content 提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]string{"content": "企微出站消息"},
		})
		text, ok := wp.ExtractOutbound("/cgi-bin/message/send", body)
		if !ok {
			t.Fatal("应返回 ok=true")
		}
		if text != "企微出站消息" {
			t.Errorf("文本期望 '企微出站消息'，实际 %q", text)
		}
	})

	t.Run("markdown.content 提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": "## Markdown内容"},
		})
		text, _ := wp.ExtractOutbound("/cgi-bin/message/send", body)
		if text != "## Markdown内容" {
			t.Errorf("文本提取错误: %q", text)
		}
	})

	t.Run("ShouldAuditOutbound", func(t *testing.T) {
		if !wp.ShouldAuditOutbound("/cgi-bin/message/send") {
			t.Error("应审计 /cgi-bin/message/send")
		}
		if wp.ShouldAuditOutbound("/cgi-bin/user/get") {
			t.Error("不应审计 /cgi-bin/user/get")
		}
	})
}

func TestGenericPlugin_ParseInbound(t *testing.T) {
	t.Run("默认字段配置", func(t *testing.T) {
		gp := NewGenericPlugin("", "")
		body := []byte(`{"content":"通用消息","sender_id":"user123","event_type":"message"}`)
		msg, err := gp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "通用消息" {
			t.Errorf("文本期望 '通用消息'，实际 %q", msg.Text)
		}
		if msg.SenderID != "user123" {
			t.Errorf("发送者期望 'user123'，实际 %q", msg.SenderID)
		}
		if msg.EventType != "message" {
			t.Errorf("事件类型期望 'message'，实际 %q", msg.EventType)
		}
	})

	t.Run("自定义字段配置", func(t *testing.T) {
		gp := NewGenericPlugin("X-Custom-Sender", "text")
		body := []byte(`{"text":"自定义字段消息","sender":"custom_user"}`)
		msg, err := gp.ParseInbound(body)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if msg.Text != "自定义字段消息" {
			t.Errorf("文本期望 '自定义字段消息'，实际 %q", msg.Text)
		}
		if msg.SenderID != "custom_user" {
			t.Errorf("发送者期望 'custom_user'，实际 %q", msg.SenderID)
		}
	})

	t.Run("无效 JSON", func(t *testing.T) {
		gp := NewGenericPlugin("", "")
		_, err := gp.ParseInbound([]byte("not json"))
		if err == nil {
			t.Fatal("无效 JSON 应返回错误")
		}
	})
}

func TestGenericPlugin_ExtractOutbound(t *testing.T) {
	gp := NewGenericPlugin("", "")

	t.Run("所有路径都审计", func(t *testing.T) {
		if !gp.ShouldAuditOutbound("/any/path") {
			t.Error("通用插件应审计所有路径")
		}
		if !gp.ShouldAuditOutbound("/another/path") {
			t.Error("通用插件应审计所有路径")
		}
	})

	t.Run("出站消息提取", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"content": "通用出站消息",
		})
		text, ok := gp.ExtractOutbound("/api/send", body)
		if !ok {
			t.Fatal("应返回 ok=true")
		}
		if text != "通用出站消息" {
			t.Errorf("文本期望 '通用出站消息'，实际 %q", text)
		}
	})
}

func TestPluginBridgeSupport(t *testing.T) {
	tests := []struct {
		name    string
		plugin  ChannelPlugin
		support bool
	}{
		{"Lanxin", func() ChannelPlugin {
			crypto, _ := NewLanxinCrypto("ODk0RUVBMEEyRjhDOThGQjhFOTAwNjdGODFFN0IwQUY", "token")
			return NewLanxinPlugin(crypto)
		}(), false},
		{"Feishu", NewFeishuPlugin("key", "token"), true},
		{"Dingtalk", NewDingtalkPlugin("token", "", "corp"), true},
		{"Wecom", NewWecomPlugin("token", "", "corp"), false},
		{"Generic", NewGenericPlugin("", ""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.plugin.SupportsBridge() != tt.support {
				t.Errorf("%s SupportsBridge() 期望 %v，实际 %v", tt.name, tt.support, tt.plugin.SupportsBridge())
			}
		})
	}
}


// ============================================================
// 企微 GET 验证 HTTP 集成测试 (v3.2)
// ============================================================

func TestWecomGETVerification_HTTP(t *testing.T) {
	token := "test_wecom_token"
	aesKeyBase64 := "ODk0RUVBMEEyRjhDOThGQjhFOTAwNjdGODFFN0IwQUY"
	corpId := "ww_corp_123"

	tmpDB := fmt.Sprintf("/tmp/lobster-guard-test-wecom-verify-%d.db", time.Now().UnixNano())
	defer os.Remove(tmpDB)
	cfg := &Config{
		Channel:              "wecom",
		WecomToken:           token,
		WecomEncodingAesKey:  aesKeyBase64,
		WecomCorpId:          corpId,
		StaticUpstreams:      []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 3,
		InboundDetectEnabled: true,
		DetectTimeoutMs:      50,
	}
	db, _ := initDB(tmpDB)
	defer db.Close()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	wp := NewWecomPlugin(token, aesKeyBase64, corpId)
	inbound := NewInboundProxy(cfg, wp, engine, logger, pool, routes, nil, nil, nil, nil, nil)

	t.Run("企微 GET 验证成功", func(t *testing.T) {
		echoStr := wecomEncrypt(t, aesKeyBase64, corpId, []byte("verify_success"))
		timestamp := "1234567890"
		nonce := "test_nonce"
		msgSignature := wecomSign(token, timestamp, nonce, echoStr)

		reqURL := fmt.Sprintf("/?msg_signature=%s&timestamp=%s&nonce=%s&echostr=%s",
			url.QueryEscape(msgSignature), timestamp, nonce, url.QueryEscape(echoStr))
		req := httptest.NewRequest("GET", reqURL, nil)
		rec := httptest.NewRecorder()
		inbound.ServeHTTP(rec, req)

		if rec.Code != 200 {
			t.Fatalf("期望 200，实际 %d body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "verify_success" {
			t.Errorf("期望 'verify_success'，实际 %q", rec.Body.String())
		}
	})

	t.Run("企微 GET 缺少参数", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?msg_signature=xxx", nil)
		rec := httptest.NewRecorder()
		inbound.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("期望 400，实际 %d", rec.Code)
		}
	})

	t.Run("企微 GET 签名错误", func(t *testing.T) {
		echoStr := wecomEncrypt(t, aesKeyBase64, corpId, []byte("test"))
		reqURL := fmt.Sprintf("/?msg_signature=wrong&timestamp=123&nonce=n&echostr=%s",
			url.QueryEscape(echoStr))
		req := httptest.NewRequest("GET", reqURL, nil)
		rec := httptest.NewRecorder()
		inbound.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatalf("期望 403，实际 %d", rec.Code)
		}
	})
}

// ============================================================
// 飞书 URL Verification HTTP 集成测试 (v3.2)
// ============================================================

func TestFeishuURLVerification_HTTP(t *testing.T) {
	tmpDB := fmt.Sprintf("/tmp/lobster-guard-test-feishu-verify-%d.db", time.Now().UnixNano())
	defer os.Remove(tmpDB)
	cfg := &Config{
		Channel:              "feishu",
		StaticUpstreams:      []StaticUpstreamConfig{{ID: "up-1", Address: "127.0.0.1", Port: 18790}},
		HeartbeatIntervalSec: 10,
		HeartbeatTimeoutCount: 3,
		InboundDetectEnabled: true,
		DetectTimeoutMs:      50,
	}
	db, _ := initDB(tmpDB)
	defer db.Close()
	pool := NewUpstreamPool(cfg, db)
	routes := NewRouteTable(db, false)
	logger, _ := NewAuditLogger(db)
	defer logger.Close()
	engine := NewRuleEngine()
	fp := NewFeishuPlugin("key", "token")
	inbound := NewInboundProxy(cfg, fp, engine, logger, pool, routes, nil, nil, nil, nil, nil)

	body := []byte(`{"type":"url_verification","challenge":"http_test_challenge","token":"xxx"}`)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	inbound.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("期望 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["challenge"] != "http_test_challenge" {
		t.Errorf("challenge 期望 'http_test_challenge'，实际 %q", resp["challenge"])
	}
}

// ============================================================
// InboundMessage 结构测试 (v3.2)
// ============================================================

func TestInboundMessageVerifyFields(t *testing.T) {
	msg := InboundMessage{
		Text:        "test",
		SenderID:    "user1",
		EventType:   "url_verification",
		Raw:         []byte(`{"challenge":"abc"}`),
		IsVerify:    true,
		VerifyReply: []byte(`{"challenge":"abc"}`),
	}
	if !msg.IsVerify {
		t.Error("IsVerify 应为 true")
	}
	if msg.VerifyReply == nil {
		t.Error("VerifyReply 不应为 nil")
	}
}

// ============================================================
// 健壮性增强测试 (v3.2)
// ============================================================
