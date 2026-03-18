// crypto.go — AES 加解密、签名验证、AC 自动机（AhoCorasick）
// lobster-guard v4.0 代码拆分
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ============================================================
// Aho-Corasick 多模式匹配自动机
// ============================================================

type AhoCorasick struct {
	gotoFn   []map[rune]int
	fail     []int
	output   [][]int
	patterns []string
}

func NewAhoCorasick(patterns []string) *AhoCorasick {
	ac := &AhoCorasick{
		gotoFn: []map[rune]int{make(map[rune]int)}, fail: []int{0}, output: [][]int{nil}, patterns: patterns,
	}
	for i, p := range patterns {
		s := 0
		for _, ch := range strings.ToLower(p) {
			if ns, ok := ac.gotoFn[s][ch]; ok {
				s = ns
			} else {
				ns := len(ac.gotoFn)
				ac.gotoFn = append(ac.gotoFn, make(map[rune]int))
				ac.fail = append(ac.fail, 0)
				ac.output = append(ac.output, nil)
				ac.gotoFn[s][ch] = ns
				s = ns
			}
		}
		ac.output[s] = append(ac.output[s], i)
	}
	var q []int
	for _, s := range ac.gotoFn[0] { q = append(q, s) }
	for len(q) > 0 {
		r := q[0]; q = q[1:]
		for ch, s := range ac.gotoFn[r] {
			q = append(q, s)
			st := ac.fail[r]
			for st != 0 {
				if _, ok := ac.gotoFn[st][ch]; ok { break }
				st = ac.fail[st]
			}
			if nx, ok := ac.gotoFn[st][ch]; ok && nx != s { ac.fail[s] = nx }
			if ac.output[ac.fail[s]] != nil {
				cp := make([]int, len(ac.output[s]))
				copy(cp, ac.output[s])
				ac.output[s] = append(cp, ac.output[ac.fail[s]]...)
			}
		}
	}
	return ac
}

func (ac *AhoCorasick) Search(text string) []int {
	var matches []int
	s := 0
	for _, ch := range strings.ToLower(text) {
		for s != 0 {
			if _, ok := ac.gotoFn[s][ch]; ok { break }
			s = ac.fail[s]
		}
		if nx, ok := ac.gotoFn[s][ch]; ok { s = nx }
		matches = append(matches, ac.output[s]...)
	}
	return matches
}

// ============================================================
// 蓝信加解密
// ============================================================

type LanxinCrypto struct { aesKey, iv []byte; signToken string }

func NewLanxinCrypto(callbackKey, signToken string) (*LanxinCrypto, error) {
	dec, err := base64.StdEncoding.DecodeString(callbackKey + "=")
	if err != nil { return nil, fmt.Errorf("解码 callbackKey 失败: %w", err) }
	if len(dec) < 32 { return nil, fmt.Errorf("callbackKey 过短: %d", len(dec)) }
	k := dec[:32]
	return &LanxinCrypto{aesKey: k, iv: k[:16], signToken: signToken}, nil
}

type LanxinWebhookBody struct {
	DataEncrypt string `json:"dataEncrypt"`
	Encrypt     string `json:"encrypt"`    // 兼容字段
	Signature   string `json:"signature"`  // 可能在 URL query 中
	Timestamp   string `json:"timestamp"`  // 可能在 URL query 中
	Nonce       string `json:"nonce"`      // 可能在 URL query 中
}

// DataEncryptValue 返回密文（兼容 dataEncrypt 和 encrypt 两种字段名）
func (wb *LanxinWebhookBody) DataEncryptValue() string {
	if wb.DataEncrypt != "" {
		return wb.DataEncrypt
	}
	return wb.Encrypt
}

func (lc *LanxinCrypto) VerifySignature(b *LanxinWebhookBody) bool {
	parts := []string{lc.signToken, b.Timestamp, b.Nonce, b.DataEncrypt}
	sort.Strings(parts)
	h := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", h) == b.Signature
}

func (lc *LanxinCrypto) Decrypt(dataEncrypt string) ([]byte, error) {
	ct, err := base64.StdEncoding.DecodeString(dataEncrypt)
	if err != nil { return nil, fmt.Errorf("base64 解码失败: %w", err) }
	block, err := aes.NewCipher(lc.aesKey)
	if err != nil { return nil, fmt.Errorf("AES 失败: %w", err) }
	if len(ct)%aes.BlockSize != 0 { return nil, fmt.Errorf("密文长度不合法") }
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, lc.iv).CryptBlocks(pt, ct)
	if n := len(pt); n > 0 {
		pad := int(pt[n-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= n {
			ok := true
			for i := n - pad; i < n; i++ { if pt[i] != byte(pad) { ok = false; break } }
			if ok { pt = pt[:n-pad] }
		}
	}
	if len(pt) < 20 { return nil, fmt.Errorf("数据过短: %d", len(pt)) }
	cl := binary.BigEndian.Uint32(pt[16:20])
	var raw string
	if int(cl) <= len(pt)-20 {
		raw = string(pt[20 : 20+cl])
	} else {
		raw = string(pt[20:])
	}
	// 找第一个 { 开始的位置
	jsonStart := strings.Index(raw, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("未找到 JSON")
	}
	// 用括号匹配提取第一个完整 JSON 对象（与 OpenClaw 对齐，过滤掉尾缀 appId 等）
	extracted := extractFirstJSON(raw[jsonStart:])
	if extracted == "" {
		return nil, fmt.Errorf("JSON 结构不完整")
	}
	return []byte(extracted), nil
}

// extractFirstJSON 提取字符串中第一个完整的 JSON 对象（支持嵌套大括号）
func extractFirstJSON(s string) string {
	depth := 0
	inStr := false
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape { escape = false; continue }
		if inStr {
			if c == '\\' { escape = true } else if c == '"' { inStr = false }
			continue
		}
		if c == '"' { inStr = true; continue }
		if c == '{' { depth++; continue }
		if c == '}' {
			depth--
			if depth == 0 { return s[:i+1] }
		}
	}
	return ""
}

func extractMessageText(data []byte) (text, senderID, eventType, appID string) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 { return }

	var msg map[string]interface{}
	if json.Unmarshal(data, &msg) != nil { return }

	if et, ok := msg["eventType"].(string); ok { eventType = et }
	if d, ok := msg["data"].(map[string]interface{}); ok {
		for _, k := range []string{"FromStaffId", "from", "senderId", "sender_id"} {
			if s, ok := d[k].(string); ok && s != "" { senderID = s; break }
		}
		// appID: entryId 或 appId
		for _, k := range []string{"entryId", "appId", "app_id"} {
			if s, ok := d[k].(string); ok && s != "" { appID = s; break }
		}
		if md, ok := d["msgData"].(map[string]interface{}); ok {
			if to, ok := md["text"].(map[string]interface{}); ok {
				for _, k := range []string{"content", "Content"} {
					if c, ok := to[k].(string); ok { text = c; return }
				}
			}
			if s, ok := md["text"].(string); ok { text = s; return }
			if c, ok := md["content"].(string); ok { text = c; return }
		}
	}
	if c, ok := msg["content"].(string); ok { text = c }
	return
}

