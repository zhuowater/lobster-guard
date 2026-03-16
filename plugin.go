// plugin.go — ChannelPlugin 接口、5 个通道插件实现（蓝信/飞书/钉钉/企微/通用HTTP）
// lobster-guard v4.0 代码拆分
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ============================================================
// Channel Plugin 接口（v3.0 消息通道抽象）
// ============================================================

type InboundMessage struct {
	Text         string
	SenderID     string
	EventType    string
	AppID        string // 应用 ID（蓝信: entryId / appId）
	Raw          []byte
	IsVerify     bool   // URL verification / echostr 验证请求
	VerifyReply  []byte // 验证请求的响应内容
}

// RequestAwareParser 可选接口：支持从 HTTP 请求中提取额外参数（如蓝信 URL query 中的 timestamp/nonce）
type RequestAwareParser interface {
	ParseInboundRequest(body []byte, r *http.Request) (InboundMessage, error)
}

type ChannelPlugin interface {
	Name() string
	ParseInbound(body []byte) (InboundMessage, error)
	ExtractOutbound(path string, body []byte) (string, bool)
	ShouldAuditOutbound(path string) bool
	BlockResponse() (int, []byte)
	BlockResponseWithMessage(customMsg string) (int, []byte) // v3.6: 自定义拦截消息
	OutboundBlockResponse(reason, ruleName string) (int, []byte)
	OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) // v3.6
	SupportsBridge() bool
	NewBridgeConnector(cfg *Config) (BridgeConnector, error)
}

// ============================================================
// Bridge Mode 接口（v3.1 长连接桥接）
// ============================================================

type BridgeStatus struct {
	Connected    bool      `json:"connected"`
	ConnectedAt  time.Time `json:"connected_at,omitempty"`
	Reconnects   int       `json:"reconnects"`
	LastError    string    `json:"last_error,omitempty"`
	LastMessage  time.Time `json:"last_message,omitempty"`
	MessageCount int64     `json:"message_count"`
}

type BridgeConnector interface {
	Name() string
	Start(ctx context.Context, onMessage func(msg InboundMessage)) error
	Stop() error
	Status() BridgeStatus
}


// ============================================================
// LanxinPlugin — 蓝信通道插件
// ============================================================

type LanxinPlugin struct {
	crypto *LanxinCrypto
}

func NewLanxinPlugin(crypto *LanxinCrypto) *LanxinPlugin {
	return &LanxinPlugin{crypto: crypto}
}

func (lp *LanxinPlugin) Name() string { return "lanxin" }

func (lp *LanxinPlugin) ParseInbound(body []byte) (InboundMessage, error) {
	return lp.parseInbound(body, "", "", "")
}

// ParseInboundRequest 实现 RequestAwareParser 接口，从 URL query 提取 timestamp/nonce/signature
func (lp *LanxinPlugin) ParseInboundRequest(body []byte, r *http.Request) (InboundMessage, error) {
	q := r.URL.Query()
	ts := q.Get("timestamp")
	nonce := q.Get("nonce")
	// 蓝信签名可能在 dev_data_signature 或 signature 参数中
	sig := q.Get("dev_data_signature")
	if sig == "" {
		sig = q.Get("signature")
	}
	return lp.parseInbound(body, ts, nonce, sig)
}

func (lp *LanxinPlugin) parseInbound(body []byte, urlTimestamp, urlNonce, urlSignature string) (InboundMessage, error) {
	var wb LanxinWebhookBody
	if err := json.Unmarshal(body, &wb); err != nil {
		return InboundMessage{}, fmt.Errorf("非蓝信 webhook 格式")
	}
	dataEncrypt := wb.DataEncryptValue()
	if dataEncrypt == "" {
		return InboundMessage{}, fmt.Errorf("非蓝信 webhook 格式")
	}
	// 蓝信通过 URL query 传 timestamp/nonce/signature（优先 URL，兜底 body）
	timestamp := urlTimestamp
	if timestamp == "" {
		timestamp = wb.Timestamp
	}
	nonce := urlNonce
	if nonce == "" {
		nonce = wb.Nonce
	}
	signature := urlSignature
	if signature == "" {
		signature = wb.Signature
	}
	// 用统一的值做签名验证
	verifyBody := &LanxinWebhookBody{
		DataEncrypt: dataEncrypt,
		Timestamp:   timestamp,
		Nonce:       nonce,
		Signature:   signature,
	}
	if !lp.crypto.VerifySignature(verifyBody) {
		return InboundMessage{}, fmt.Errorf("签名验证失败")
	}
	dec, err := lp.crypto.Decrypt(dataEncrypt)
	if err != nil {
		return InboundMessage{}, fmt.Errorf("解密失败: %w", err)
	}
	text, senderID, eventType, appID := extractMessageText(dec)
	return InboundMessage{Text: text, SenderID: senderID, EventType: eventType, AppID: appID, Raw: dec}, nil
}

var lanxinAuditPaths = map[string]bool{
	"/v1/bot/messages/create": true,
	"/v1/bot/sendGroupMsg":    true,
	"/v1/bot/sendPrivateMsg":  true,
}

func (lp *LanxinPlugin) ShouldAuditOutbound(path string) bool {
	return lanxinAuditPaths[path]
}

func (lp *LanxinPlugin) ExtractOutbound(path string, body []byte) (string, bool) {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil {
		return string(body), true
	}
	if md, ok := msg["msgData"].(map[string]interface{}); ok {
		if to, ok := md["text"].(map[string]interface{}); ok {
			if c, ok := to["content"].(string); ok {
				return c, true
			}
		}
	}
	if c, ok := msg["content"].(string); ok {
		return c, true
	}
	return string(body), true
}

// ExtractOutboundRecipient 从蓝信出站消息中提取接收者
func (lp *LanxinPlugin) ExtractOutboundRecipient(body []byte) string {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil { return "" }
	// 私聊: userIdList
	if uids, ok := msg["userIdList"].([]interface{}); ok && len(uids) > 0 {
		if s, ok := uids[0].(string); ok { return s }
	}
	// 群聊: groupId
	if gid, ok := msg["groupId"].(string); ok { return gid }
	return ""
}

func (lp *LanxinPlugin) BlockResponse() (int, []byte) {
	return 200, []byte(`{"errcode":0,"errmsg":"ok"}`)
}

func (lp *LanxinPlugin) BlockResponseWithMessage(customMsg string) (int, []byte) {
	if customMsg == "" {
		return lp.BlockResponse()
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 0, "errmsg": "ok", "message": customMsg,
	})
	return 200, resp
}

func (lp *LanxinPlugin) OutboundBlockResponse(reason, ruleName string) (int, []byte) {
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": "Message blocked by security policy",
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (lp *LanxinPlugin) OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) {
	if customMsg == "" {
		return lp.OutboundBlockResponse(reason, ruleName)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": customMsg,
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (lp *LanxinPlugin) SupportsBridge() bool { return false }

func (lp *LanxinPlugin) NewBridgeConnector(cfg *Config) (BridgeConnector, error) {
	return nil, fmt.Errorf("蓝信通道不支持桥接模式")
}


// ============================================================
// FeishuPlugin — 飞书通道插件
// ============================================================

type FeishuPlugin struct {
	encryptKey        []byte
	verificationToken string
}

func NewFeishuPlugin(encryptKey, verificationToken string) *FeishuPlugin {
	return &FeishuPlugin{
		encryptKey:        []byte(encryptKey),
		verificationToken: verificationToken,
	}
}

func (fp *FeishuPlugin) Name() string { return "feishu" }

func (fp *FeishuPlugin) feishuDecrypt(encrypted string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("密文过短")
	}
	keyHash := sha256.Sum256(fp.encryptKey)
	key := keyHash[:32]
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不合法")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES 失败: %w", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	// PKCS7 unpadding
	if n := len(plaintext); n > 0 {
		pad := int(plaintext[n-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= n {
			ok := true
			for i := n - pad; i < n; i++ {
				if plaintext[i] != byte(pad) { ok = false; break }
			}
			if ok { plaintext = plaintext[:n-pad] }
		}
	}
	return plaintext, nil
}

func (fp *FeishuPlugin) ParseInbound(body []byte) (InboundMessage, error) {
	// 尝试解密
	var encBody struct {
		Encrypt string `json:"encrypt"`
	}
	var plainBody []byte
	if json.Unmarshal(body, &encBody) == nil && encBody.Encrypt != "" {
		dec, err := fp.feishuDecrypt(encBody.Encrypt)
		if err != nil {
			return InboundMessage{}, fmt.Errorf("飞书解密失败: %w", err)
		}
		plainBody = dec
	} else {
		plainBody = body
	}

	// 解析 JSON
	var msg map[string]interface{}
	if err := json.Unmarshal(plainBody, &msg); err != nil {
		return InboundMessage{}, fmt.Errorf("JSON 解析失败: %w", err)
	}

	// URL Verification
	if tp, ok := msg["type"].(string); ok && tp == "url_verification" {
		challenge, _ := msg["challenge"].(string)
		resp, _ := json.Marshal(map[string]string{"challenge": challenge})
		return InboundMessage{EventType: "url_verification", Raw: resp, IsVerify: true, VerifyReply: resp}, nil
	}

	// 提取消息
	var text, senderID, eventType string
	if header, ok := msg["header"].(map[string]interface{}); ok {
		if et, ok := header["event_type"].(string); ok {
			eventType = et
		}
	}
	if event, ok := msg["event"].(map[string]interface{}); ok {
		// 提取发送者
		if sender, ok := event["sender"].(map[string]interface{}); ok {
			if senderIdMap, ok := sender["sender_id"].(map[string]interface{}); ok {
				if openID, ok := senderIdMap["open_id"].(string); ok {
					senderID = openID
				}
			}
		}
		// 提取消息文本
		if message, ok := event["message"].(map[string]interface{}); ok {
			if content, ok := message["content"].(string); ok {
				var contentObj map[string]interface{}
				if json.Unmarshal([]byte(content), &contentObj) == nil {
					if t, ok := contentObj["text"].(string); ok {
						text = t
					}
				}
			}
		}
	}

	return InboundMessage{Text: text, SenderID: senderID, EventType: eventType, Raw: plainBody}, nil
}

func (fp *FeishuPlugin) ShouldAuditOutbound(path string) bool {
	return strings.HasPrefix(path, "/open-apis/im/v1/messages")
}

func (fp *FeishuPlugin) ExtractOutbound(path string, body []byte) (string, bool) {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil {
		return string(body), true
	}
	if content, ok := msg["content"].(string); ok {
		var contentObj map[string]interface{}
		if json.Unmarshal([]byte(content), &contentObj) == nil {
			if t, ok := contentObj["text"].(string); ok {
				return t, true
			}
		}
		return content, true
	}
	return string(body), true
}

func (fp *FeishuPlugin) BlockResponse() (int, []byte) {
	return 200, []byte(`{"code":0,"msg":"ok"}`)
}

func (fp *FeishuPlugin) BlockResponseWithMessage(customMsg string) (int, []byte) {
	if customMsg == "" {
		return fp.BlockResponse()
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 0, "msg": "ok", "message": customMsg,
	})
	return 200, resp
}

func (fp *FeishuPlugin) OutboundBlockResponse(reason, ruleName string) (int, []byte) {
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 403, "msg": "blocked by security policy",
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (fp *FeishuPlugin) OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) {
	if customMsg == "" {
		return fp.OutboundBlockResponse(reason, ruleName)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 403, "msg": customMsg,
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (fp *FeishuPlugin) SupportsBridge() bool { return true }

func (fp *FeishuPlugin) NewBridgeConnector(cfg *Config) (BridgeConnector, error) {
	if cfg.FeishuAppID == "" || cfg.FeishuAppSecret == "" {
		return nil, fmt.Errorf("飞书桥接模式需要配置 feishu_app_id 和 feishu_app_secret")
	}
	return &FeishuBridge{
		appID:     cfg.FeishuAppID,
		appSecret: cfg.FeishuAppSecret,
		plugin:    fp,
	}, nil
}


// ============================================================
// GenericPlugin — 通用 HTTP 通道插件
// ============================================================

type GenericPlugin struct {
	senderHeader string
	textField    string
}

func NewGenericPlugin(senderHeader, textField string) *GenericPlugin {
	if senderHeader == "" {
		senderHeader = "X-Sender-Id"
	}
	if textField == "" {
		textField = "content"
	}
	return &GenericPlugin{senderHeader: senderHeader, textField: textField}
}

func (gp *GenericPlugin) Name() string { return "generic" }

func (gp *GenericPlugin) ParseInbound(body []byte) (InboundMessage, error) {
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		return InboundMessage{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	text, _ := msg[gp.textField].(string)
	senderID, _ := msg["sender_id"].(string)
	if senderID == "" {
		senderID, _ = msg["sender"].(string)
	}
	eventType, _ := msg["event_type"].(string)
	return InboundMessage{Text: text, SenderID: senderID, EventType: eventType, Raw: body}, nil
}

func (gp *GenericPlugin) ShouldAuditOutbound(path string) bool {
	return true // 通用插件审计所有路径
}

func (gp *GenericPlugin) ExtractOutbound(path string, body []byte) (string, bool) {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil {
		return string(body), true
	}
	if text, ok := msg[gp.textField].(string); ok {
		return text, true
	}
	return string(body), true
}

func (gp *GenericPlugin) BlockResponse() (int, []byte) {
	return 200, []byte(`{"code":0,"msg":"ok"}`)
}

func (gp *GenericPlugin) BlockResponseWithMessage(customMsg string) (int, []byte) {
	if customMsg == "" {
		return gp.BlockResponse()
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 0, "msg": "ok", "message": customMsg,
	})
	return 200, resp
}

func (gp *GenericPlugin) OutboundBlockResponse(reason, ruleName string) (int, []byte) {
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 403, "msg": "blocked by security policy",
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (gp *GenericPlugin) OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) {
	if customMsg == "" {
		return gp.OutboundBlockResponse(reason, ruleName)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"code": 403, "msg": customMsg,
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (gp *GenericPlugin) SupportsBridge() bool { return false }

func (gp *GenericPlugin) NewBridgeConnector(cfg *Config) (BridgeConnector, error) {
	return nil, fmt.Errorf("通用通道不支持桥接模式")
}


// ============================================================
// DingtalkPlugin — 钉钉通道插件
// ============================================================

type DingtalkPlugin struct {
	token  string
	aesKey []byte
	corpId string
}

func NewDingtalkPlugin(token, aesKeyBase64, corpId string) *DingtalkPlugin {
	var aesKey []byte
	if aesKeyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64 + "=")
		if err == nil && len(decoded) >= 32 {
			aesKey = decoded[:32]
		}
	}
	return &DingtalkPlugin{token: token, aesKey: aesKey, corpId: corpId}
}

func (dp *DingtalkPlugin) Name() string { return "dingtalk" }

func (dp *DingtalkPlugin) dingtalkVerifySign(timestamp, sign string) bool {
	if dp.token == "" || timestamp == "" {
		return true // 未配置 token 则跳过签名校验
	}
	stringToSign := timestamp + "\n" + dp.token
	mac := hmac.New(sha256.New, []byte(dp.token))
	mac.Write([]byte(stringToSign))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return sign == expected
}

func (dp *DingtalkPlugin) dingtalkDecrypt(encrypted string) ([]byte, error) {
	if dp.aesKey == nil {
		return nil, fmt.Errorf("钉钉 AES key 未配置")
	}
	ct, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	if len(ct) < aes.BlockSize || len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不合法")
	}
	block, err := aes.NewCipher(dp.aesKey)
	if err != nil {
		return nil, fmt.Errorf("AES 失败: %w", err)
	}
	iv := dp.aesKey[:16]
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
	// PKCS7 unpadding
	if n := len(pt); n > 0 {
		pad := int(pt[n-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= n {
			ok := true
			for i := n - pad; i < n; i++ {
				if pt[i] != byte(pad) { ok = false; break }
			}
			if ok { pt = pt[:n-pad] }
		}
	}
	// 明文格式: random(16) + msg_len(4) + msg + corpId
	if len(pt) < 20 {
		return nil, fmt.Errorf("数据过短: %d", len(pt))
	}
	msgLen := binary.BigEndian.Uint32(pt[16:20])
	if int(msgLen) > len(pt)-20 {
		return nil, fmt.Errorf("消息长度不合法")
	}
	return pt[20 : 20+msgLen], nil
}

func (dp *DingtalkPlugin) ParseInbound(body []byte) (InboundMessage, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return InboundMessage{}, fmt.Errorf("JSON 解析失败: %w", err)
	}

	// 尝试解密（如果有 encrypt 字段）
	var plainBody []byte
	if encrypted, ok := raw["encrypt"].(string); ok && encrypted != "" {
		dec, err := dp.dingtalkDecrypt(encrypted)
		if err != nil {
			return InboundMessage{}, fmt.Errorf("钉钉解密失败: %w", err)
		}
		plainBody = dec
		// 重新解析
		raw = nil
		if json.Unmarshal(plainBody, &raw) != nil {
			return InboundMessage{}, fmt.Errorf("解密后 JSON 解析失败")
		}
	} else {
		plainBody = body
	}

	// 提取消息
	var text, senderID, eventType string

	// msgtype 字段
	if mt, ok := raw["msgtype"].(string); ok {
		eventType = mt
	}

	// 发送者
	if sid, ok := raw["senderStaffId"].(string); ok {
		senderID = sid
	} else if sid, ok := raw["senderId"].(string); ok {
		senderID = sid
	}

	// 文本提取
	if textObj, ok := raw["text"].(map[string]interface{}); ok {
		if c, ok := textObj["content"].(string); ok {
			text = strings.TrimSpace(c)
		}
	}

	return InboundMessage{Text: text, SenderID: senderID, EventType: eventType, Raw: plainBody}, nil
}

var dingtalkAuditPaths = map[string]bool{
	"/robot/send": true,
	"/topapi/message/corpconversation/asyncsend_v2": true,
	"/v1.0/robot/oToMessages/batchSend":             true,
}

func (dp *DingtalkPlugin) ShouldAuditOutbound(path string) bool {
	return dingtalkAuditPaths[path]
}

func (dp *DingtalkPlugin) ExtractOutbound(path string, body []byte) (string, bool) {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil {
		return string(body), true
	}
	// text.content
	if textObj, ok := msg["text"].(map[string]interface{}); ok {
		if c, ok := textObj["content"].(string); ok {
			return c, true
		}
	}
	// markdown.text
	if mdObj, ok := msg["markdown"].(map[string]interface{}); ok {
		if t, ok := mdObj["text"].(string); ok {
			return t, true
		}
	}
	if c, ok := msg["content"].(string); ok {
		return c, true
	}
	return string(body), true
}

func (dp *DingtalkPlugin) BlockResponse() (int, []byte) {
	return 200, []byte(`{"errcode":0,"errmsg":"ok"}`)
}

func (dp *DingtalkPlugin) BlockResponseWithMessage(customMsg string) (int, []byte) {
	if customMsg == "" {
		return dp.BlockResponse()
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 0, "errmsg": "ok", "message": customMsg,
	})
	return 200, resp
}

func (dp *DingtalkPlugin) OutboundBlockResponse(reason, ruleName string) (int, []byte) {
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": "blocked by security policy",
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (dp *DingtalkPlugin) OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) {
	if customMsg == "" {
		return dp.OutboundBlockResponse(reason, ruleName)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": customMsg,
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (dp *DingtalkPlugin) SupportsBridge() bool { return true }

func (dp *DingtalkPlugin) NewBridgeConnector(cfg *Config) (BridgeConnector, error) {
	if cfg.DingtalkClientID == "" || cfg.DingtalkClientSecret == "" {
		return nil, fmt.Errorf("钉钉桥接模式需要配置 dingtalk_client_id 和 dingtalk_client_secret")
	}
	return &DingtalkBridge{
		clientID:     cfg.DingtalkClientID,
		clientSecret: cfg.DingtalkClientSecret,
		plugin:       dp,
	}, nil
}


// ============================================================
// WecomPlugin — 企业微信通道插件
// ============================================================

type WecomPlugin struct {
	token          string
	encodingAesKey []byte
	corpId         string
}

func NewWecomPlugin(token, encodingAesKeyBase64, corpId string) *WecomPlugin {
	var aesKey []byte
	if encodingAesKeyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(encodingAesKeyBase64 + "=")
		if err == nil && len(decoded) >= 32 {
			aesKey = decoded[:32]
		}
	}
	return &WecomPlugin{token: token, encodingAesKey: aesKey, corpId: corpId}
}

func (wp *WecomPlugin) Name() string { return "wecom" }

// wecomVerifySignature: SHA1(sort(token, timestamp, nonce, encrypt_msg))
func (wp *WecomPlugin) wecomVerifySignature(signature, timestamp, nonce, encryptMsg string) bool {
	if wp.token == "" {
		return true // 未配置 token 则跳过
	}
	parts := []string{wp.token, timestamp, nonce, encryptMsg}
	sort.Strings(parts)
	h := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", h) == signature
}

func (wp *WecomPlugin) wecomDecrypt(encrypted string) ([]byte, error) {
	if wp.encodingAesKey == nil {
		return nil, fmt.Errorf("企微 AES key 未配置")
	}
	ct, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	if len(ct) < aes.BlockSize || len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不合法")
	}
	block, err := aes.NewCipher(wp.encodingAesKey)
	if err != nil {
		return nil, fmt.Errorf("AES 失败: %w", err)
	}
	iv := wp.encodingAesKey[:16]
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
	// PKCS7 unpadding
	if n := len(pt); n > 0 {
		pad := int(pt[n-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= n {
			ok := true
			for i := n - pad; i < n; i++ {
				if pt[i] != byte(pad) { ok = false; break }
			}
			if ok { pt = pt[:n-pad] }
		}
	}
	// 明文格式: random(16) + msg_len(4) + msg + corp_id
	if len(pt) < 20 {
		return nil, fmt.Errorf("数据过短: %d", len(pt))
	}
	msgLen := binary.BigEndian.Uint32(pt[16:20])
	if int(msgLen) > len(pt)-20 {
		return nil, fmt.Errorf("消息长度不合法")
	}
	return pt[20 : 20+msgLen], nil
}

// wecomXMLEncrypt 用于解析企微入站的 XML 信封
type wecomXMLEncrypt struct {
	XMLName    xml.Name `xml:"xml"`
	Encrypt    string   `xml:"Encrypt"`
	ToUserName string   `xml:"ToUserName"`
	AgentID    string   `xml:"AgentID"`
}

// wecomXMLMessage 用于解析企微解密后的消息 XML
type wecomXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
	AgentID      int64    `xml:"AgentID"`
}

func (wp *WecomPlugin) ParseInbound(body []byte) (InboundMessage, error) {
	// 企微入站是 XML 格式
	var envelope wecomXMLEncrypt
	if err := xml.Unmarshal(body, &envelope); err != nil {
		// 回退：尝试 JSON 格式（某些测试场景）
		var jsonBody map[string]interface{}
		if json.Unmarshal(body, &jsonBody) == nil {
			text, _ := jsonBody["content"].(string)
			sender, _ := jsonBody["from_user"].(string)
			return InboundMessage{Text: text, SenderID: sender, EventType: "text", Raw: body}, nil
		}
		return InboundMessage{}, fmt.Errorf("XML 解析失败: %w", err)
	}

	if envelope.Encrypt == "" {
		return InboundMessage{}, fmt.Errorf("空加密消息")
	}

	// 解密
	dec, err := wp.wecomDecrypt(envelope.Encrypt)
	if err != nil {
		return InboundMessage{}, fmt.Errorf("企微解密失败: %w", err)
	}

	// 解析消息 XML
	var msg wecomXMLMessage
	if err := xml.Unmarshal(dec, &msg); err != nil {
		return InboundMessage{}, fmt.Errorf("消息 XML 解析失败: %w", err)
	}

	return InboundMessage{
		Text:      msg.Content,
		SenderID:  msg.FromUserName,
		EventType: msg.MsgType,
		Raw:       dec,
	}, nil
}

var wecomAuditPaths = map[string]bool{
	"/cgi-bin/message/send":          true,
	"/cgi-bin/appchat/send":          true,
	"/cgi-bin/message/send_markdown": true,
}

func (wp *WecomPlugin) ShouldAuditOutbound(path string) bool {
	return wecomAuditPaths[path]
}

func (wp *WecomPlugin) ExtractOutbound(path string, body []byte) (string, bool) {
	var msg map[string]interface{}
	if json.Unmarshal(body, &msg) != nil {
		return string(body), true
	}
	// text.content
	if textObj, ok := msg["text"].(map[string]interface{}); ok {
		if c, ok := textObj["content"].(string); ok {
			return c, true
		}
	}
	// markdown.content
	if mdObj, ok := msg["markdown"].(map[string]interface{}); ok {
		if c, ok := mdObj["content"].(string); ok {
			return c, true
		}
	}
	if c, ok := msg["content"].(string); ok {
		return c, true
	}
	return string(body), true
}

func (wp *WecomPlugin) BlockResponse() (int, []byte) {
	return 200, []byte("success")
}

func (wp *WecomPlugin) BlockResponseWithMessage(customMsg string) (int, []byte) {
	if customMsg == "" {
		return wp.BlockResponse()
	}
	return 200, []byte(customMsg)
}

func (wp *WecomPlugin) OutboundBlockResponse(reason, ruleName string) (int, []byte) {
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": "blocked by security policy",
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (wp *WecomPlugin) OutboundBlockResponseWithMessage(reason, ruleName, customMsg string) (int, []byte) {
	if customMsg == "" {
		return wp.OutboundBlockResponse(reason, ruleName)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"errcode": 403, "errmsg": customMsg,
		"detail": reason, "rule": ruleName,
	})
	return 403, resp
}

func (wp *WecomPlugin) SupportsBridge() bool { return false }

func (wp *WecomPlugin) NewBridgeConnector(cfg *Config) (BridgeConnector, error) {
	return nil, fmt.Errorf("企业微信通道不支持桥接模式")
}

// VerifyURL 处理企微 GET 验证回调
// 企微首次配置回调 URL 时会发 GET 请求: ?msg_signature=xxx&timestamp=xxx&nonce=xxx&echostr=xxx
// 1. 验签: SHA1(sort(token, timestamp, nonce, echostr)) == msg_signature
// 2. AES 解密 echostr → 返回解密后的明文
func (wp *WecomPlugin) VerifyURL(msgSignature, timestamp, nonce, echostr string) (string, error) {
	// 验证签名
	if !wp.wecomVerifySignature(msgSignature, timestamp, nonce, echostr) {
		return "", fmt.Errorf("签名验证失败")
	}
	// AES 解密 echostr
	dec, err := wp.wecomDecrypt(echostr)
	if err != nil {
		return "", fmt.Errorf("解密 echostr 失败: %w", err)
	}
	return string(dec), nil
}

