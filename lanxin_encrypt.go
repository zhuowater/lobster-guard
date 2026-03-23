// lanxin_encrypt.go — 工具：生成蓝信 webhook 测试消息
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <senderID> <appID> <messageText>\n", os.Args[0])
		os.Exit(1)
	}
	senderID := os.Args[1]
	appID := os.Args[2]
	msgText := os.Args[3]

	callbackKey := "Rjc1QzJGMTlBNjYxQjQ4Qzg1RDkzRTM1RTkwRTQzQTQ"
	signToken := "BE7F53A5265CB08063E6E649AD297432"

	// Decode key (same as NewLanxinCrypto)
	dec, err := base64.StdEncoding.DecodeString(callbackKey + "=")
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode key: %v\n", err)
		os.Exit(1)
	}
	aesKey := dec[:32]
	iv := aesKey[:16]

	// Build plaintext JSON (lanxin webhook format - matches extractMessageText expectations)
	payload := map[string]interface{}{
		"eventType": "message",
		"data": map[string]interface{}{
			"FromStaffId": senderID,
			"entryId":     appID,
			"msgData": map[string]interface{}{
				"text": map[string]interface{}{
					"content": msgText,
				},
			},
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	// Encrypt: 16 random bytes + 4 bytes content length + content + appID + PKCS7 padding
	rand.Seed(time.Now().UnixNano())
	randomPrefix := make([]byte, 16)
	for i := range randomPrefix {
		randomPrefix[i] = byte(rand.Intn(256))
	}

	contentBytes := []byte(string(payloadJSON))
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(contentBytes)))

	plain := append(randomPrefix, lenBuf...)
	plain = append(plain, contentBytes...)
	// appID suffix (蓝信格式)
	plain = append(plain, []byte(appID)...)

	// PKCS7 padding
	padLen := aes.BlockSize - (len(plain) % aes.BlockSize)
	for i := 0; i < padLen; i++ {
		plain = append(plain, byte(padLen))
	}

	// AES-CBC encrypt
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aes: %v\n", err)
		os.Exit(1)
	}
	ct := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, plain)
	dataEncrypt := base64.StdEncoding.EncodeToString(ct)

	// Generate signature
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%d", rand.Int63())

	parts := []string{signToken, timestamp, nonce, dataEncrypt}
	sort.Strings(parts)
	h := sha1.Sum([]byte(strings.Join(parts, "")))
	signature := fmt.Sprintf("%x", h)

	// Build webhook body
	webhookBody := map[string]string{
		"dataEncrypt": dataEncrypt,
		"timestamp":   timestamp,
		"nonce":       nonce,
		"signature":   signature,
	}
	out, _ := json.Marshal(webhookBody)
	fmt.Print(string(out))
}
