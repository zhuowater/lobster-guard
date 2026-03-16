// trace.go — 请求追踪 trace_id 生成（v5.0 可观测性）
// 使用 crypto/rand 生成 8 字节随机 hex 字符串（16 字符）
package main

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateTraceID 生成 16 字符的 hex trace_id（8 字节随机数）
func GenerateTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// fallback: 不应该发生，但安全起见返回固定值
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}
