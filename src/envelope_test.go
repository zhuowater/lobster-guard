// envelope_test.go — 执行信封测试（v18.0）
package main

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// newTestEnvelopeManager 创建测试用信封管理器（内存数据库）
func newTestEnvelopeManager(t *testing.T) *EnvelopeManager {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写，避免并发写锁
	t.Cleanup(func() { db.Close() })
	return NewEnvelopeManager(db, "test-secret-key-at-least-32-chars!!")
}

// TestEnvelopeSeal 基本签名生成
func TestEnvelopeSeal(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-001", "inbound", "hello world", "pass", []string{"rule1"}, "sender-001")
	if env == nil {
		t.Fatal("Seal returned nil")
	}
	if env.ID == "" {
		t.Error("ID should not be empty")
	}
	if len(env.ID) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("ID should be 32 hex chars, got %d: %s", len(env.ID), env.ID)
	}
	if env.TraceID != "trace-001" {
		t.Errorf("TraceID = %s, want trace-001", env.TraceID)
	}
	if env.Domain != "inbound" {
		t.Errorf("Domain = %s, want inbound", env.Domain)
	}
	if env.Decision != "pass" {
		t.Errorf("Decision = %s, want pass", env.Decision)
	}
	if len(env.Rules) != 1 || env.Rules[0] != "rule1" {
		t.Errorf("Rules = %v, want [rule1]", env.Rules)
	}
	if env.SenderID != "sender-001" {
		t.Errorf("SenderID = %s, want sender-001", env.SenderID)
	}
	if env.RequestHash == "" {
		t.Error("RequestHash should not be empty")
	}
	if len(env.Nonce) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("Nonce should be 32 hex chars, got %d: %s", len(env.Nonce), env.Nonce)
	}
	if env.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	if env.Signature == "" {
		t.Error("Signature should not be empty")
	}
	// First envelope has empty PrevHash
	if env.PrevHash != "" {
		t.Errorf("First envelope PrevHash should be empty, got %s", env.PrevHash)
	}
}

// TestEnvelopeSealNilRules 传 nil rules 应该不 panic
func TestEnvelopeSealNilRules(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-nil", "inbound", "test", "pass", nil, "")
	if env == nil {
		t.Fatal("Seal returned nil")
	}
	if env.Rules == nil {
		t.Error("Rules should not be nil (should be empty slice)")
	}
}

// TestEnvelopeVerify 签名验证成功
func TestEnvelopeVerify(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-002", "outbound", "response data", "warn", []string{"pii_rule"}, "")

	result, err := em.Verify(env.ID)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !result.Valid {
		t.Errorf("Expected valid, got invalid: %v", result.FailureReasons)
	}
	if result.EnvelopeID != env.ID {
		t.Errorf("EnvelopeID = %s, want %s", result.EnvelopeID, env.ID)
	}
}

// TestEnvelopeVerifyTampered 篡改后验证失败
func TestEnvelopeVerifyTampered(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-003", "inbound", "sensitive data", "block", []string{"injection_rule"}, "attacker-001")

	// 篡改 decision 字段
	em.db.Exec(`UPDATE execution_envelopes SET decision = 'pass' WHERE id = ?`, env.ID)

	result, err := em.Verify(env.ID)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Valid {
		t.Error("Expected invalid after tampering, got valid")
	}
	if len(result.FailureReasons) == 0 {
		t.Error("Expected failure reasons after tampering")
	}
}

// TestEnvelopeVerifyTamperedSignature 篡改签名验证失败
func TestEnvelopeVerifyTamperedSignature(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-sig", "inbound", "test", "pass", nil, "")

	// 篡改 signature
	em.db.Exec(`UPDATE execution_envelopes SET signature = 'deadbeef' WHERE id = ?`, env.ID)

	result, err := em.Verify(env.ID)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if result.Valid {
		t.Error("Expected invalid after signature tampering, got valid")
	}
}

// TestEnvelopeVerifyNotFound 验证不存在的信封
func TestEnvelopeVerifyNotFound(t *testing.T) {
	em := newTestEnvelopeManager(t)
	_, err := em.Verify("nonexistent-id")
	if err == nil {
		t.Error("Expected error for nonexistent envelope")
	}
}

// TestEnvelopeChain 链式结构验证
func TestEnvelopeChain(t *testing.T) {
	em := newTestEnvelopeManager(t)

	traceID := "trace-chain-001"
	// 创建 5 个信封
	for i := 0; i < 5; i++ {
		em.Seal(traceID, "inbound", fmt.Sprintf("message-%d", i), "pass", []string{"rule1"}, "sender")
		time.Sleep(time.Millisecond) // ensure ordering
	}

	result, err := em.VerifyChain(traceID)
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if !result.ChainValid {
		t.Errorf("Expected chain valid, got invalid: %v", result.Details)
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", result.TotalCount)
	}
	if result.ValidCount != 5 {
		t.Errorf("ValidCount = %d, want 5", result.ValidCount)
	}
	if result.InvalidCount != 0 {
		t.Errorf("InvalidCount = %d, want 0", result.InvalidCount)
	}
	if result.BrokenAt != -1 {
		t.Errorf("BrokenAt = %d, want -1", result.BrokenAt)
	}
}

// TestEnvelopeChainBroken 链断裂检测
func TestEnvelopeChainBroken(t *testing.T) {
	em := newTestEnvelopeManager(t)

	traceID := "trace-chain-broken"
	var envs []*ExecutionEnvelope
	for i := 0; i < 3; i++ {
		env := em.Seal(traceID, "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		envs = append(envs, env)
		time.Sleep(time.Millisecond)
	}

	// 篡改第2个信封的 content_hash
	em.db.Exec(`UPDATE execution_envelopes SET content_hash = 'tampered_hash' WHERE id = ?`, envs[1].ID)

	result, err := em.VerifyChain(traceID)
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if result.ChainValid {
		t.Error("Expected chain invalid after tampering")
	}
	if result.InvalidCount == 0 {
		t.Error("Expected at least 1 invalid envelope")
	}
}

// TestEnvelopeChainEmpty 空链验证
func TestEnvelopeChainEmpty(t *testing.T) {
	em := newTestEnvelopeManager(t)
	result, err := em.VerifyChain("nonexistent-trace")
	if err != nil {
		t.Fatalf("VerifyChain error: %v", err)
	}
	if !result.ChainValid {
		t.Error("Empty chain should be valid")
	}
	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
}

// TestEnvelopeListByTrace 按 trace 查询
func TestEnvelopeListByTrace(t *testing.T) {
	em := newTestEnvelopeManager(t)

	// 创建不同 trace 的信封
	em.Seal("trace-a", "inbound", "msg1", "pass", nil, "")
	em.Seal("trace-a", "outbound", "msg2", "warn", []string{"rule1"}, "")
	em.Seal("trace-b", "inbound", "msg3", "block", nil, "")
	em.Seal("trace-a", "llm_request", "msg4", "pass", nil, "")

	list, err := em.ListByTrace("trace-a")
	if err != nil {
		t.Fatalf("ListByTrace error: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("Expected 3 envelopes for trace-a, got %d", len(list))
	}

	// 验证排序（按时间）
	for i := 1; i < len(list); i++ {
		if list[i].Timestamp.Before(list[i-1].Timestamp) {
			t.Error("Envelopes should be sorted by timestamp ASC")
		}
	}

	listB, _ := em.ListByTrace("trace-b")
	if len(listB) != 1 {
		t.Errorf("Expected 1 envelope for trace-b, got %d", len(listB))
	}
}

// TestEnvelopeStats 统计
func TestEnvelopeStats(t *testing.T) {
	em := newTestEnvelopeManager(t)

	em.Seal("t1", "inbound", "m1", "pass", nil, "")
	em.Seal("t1", "outbound", "m2", "block", []string{"r1"}, "")
	em.Seal("t2", "inbound", "m3", "warn", nil, "")
	em.Seal("t2", "llm_request", "m4", "pass", nil, "")
	em.Seal("t3", "llm_response", "m5", "rewrite", []string{"r2"}, "")

	stats := em.Stats()
	if stats["total"].(int64) != 5 {
		t.Errorf("total = %v, want 5", stats["total"])
	}

	byDomain := stats["by_domain"].(map[string]int64)
	if byDomain["inbound"] != 2 {
		t.Errorf("inbound count = %d, want 2", byDomain["inbound"])
	}
	if byDomain["outbound"] != 1 {
		t.Errorf("outbound count = %d, want 1", byDomain["outbound"])
	}

	byDecision := stats["by_decision"].(map[string]int64)
	if byDecision["pass"] != 2 {
		t.Errorf("pass count = %d, want 2", byDecision["pass"])
	}

	if stats["unique_traces"].(int64) != 3 {
		t.Errorf("unique_traces = %v, want 3", stats["unique_traces"])
	}
}

// TestEnvelopeConcurrent 并发安全
func TestEnvelopeConcurrent(t *testing.T) {
	em := newTestEnvelopeManager(t)

	const goroutines = 10
	const perGoroutine = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				traceID := fmt.Sprintf("concurrent-trace-%d", gid)
				env := em.Seal(traceID, "inbound", fmt.Sprintf("msg-%d-%d", gid, i), "pass", nil, "")
				if env == nil {
					t.Errorf("Seal returned nil for g=%d i=%d", gid, i)
				}
			}
		}(g)
	}
	wg.Wait()

	// 验证总数
	stats := em.Stats()
	total := stats["total"].(int64)
	expected := int64(goroutines * perGoroutine)
	if total != expected {
		t.Errorf("total = %d, want %d", total, expected)
	}
}

// TestEnvelopeNonceUnique Nonce 唯一性
func TestEnvelopeNonceUnique(t *testing.T) {
	em := newTestEnvelopeManager(t)
	nonces := make(map[string]bool)

	for i := 0; i < 100; i++ {
		env := em.Seal("trace-nonce", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		if nonces[env.Nonce] {
			t.Errorf("Duplicate nonce detected: %s (iteration %d)", env.Nonce, i)
		}
		nonces[env.Nonce] = true
	}
}

// TestEnvelopeIDUnique ID 唯一性
func TestEnvelopeIDUnique(t *testing.T) {
	em := newTestEnvelopeManager(t)
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		env := em.Seal("trace-id-unique", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		if ids[env.ID] {
			t.Errorf("Duplicate ID detected: %s (iteration %d)", env.ID, i)
		}
		ids[env.ID] = true
	}
}

// TestEnvelopePrevHashChain 全局链式 PrevHash
func TestEnvelopePrevHashChain(t *testing.T) {
	em := newTestEnvelopeManager(t)

	env1 := em.Seal("trace-x", "inbound", "msg1", "pass", nil, "")
	env2 := em.Seal("trace-y", "outbound", "msg2", "block", nil, "")
	env3 := em.Seal("trace-x", "llm_request", "msg3", "pass", nil, "")

	// env1 是第一个，PrevHash 为空
	if env1.PrevHash != "" {
		t.Errorf("env1.PrevHash should be empty, got %s", env1.PrevHash)
	}
	// env2 的 PrevHash 应该是 env1 的 ContentHash
	if env2.PrevHash != env1.ContentHash {
		t.Errorf("env2.PrevHash = %s, want %s (env1.ContentHash)", env2.PrevHash, env1.ContentHash)
	}
	// env3 的 PrevHash 应该是 env2 的 ContentHash（全局链，跨 domain）
	if env3.PrevHash != env2.ContentHash {
		t.Errorf("env3.PrevHash = %s, want %s (env2.ContentHash)", env3.PrevHash, env2.ContentHash)
	}
}

// TestEnvelopeRequestHash 相同内容产生相同哈希
func TestEnvelopeRequestHash(t *testing.T) {
	h1 := computeRequestHash("hello world")
	h2 := computeRequestHash("hello world")
	h3 := computeRequestHash("hello world!")

	if h1 != h2 {
		t.Error("Same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("Different content should produce different hash")
	}
}

// TestEnvelopeSignatureConsistency 相同密钥相同内容产生相同签名
func TestEnvelopeSignatureConsistency(t *testing.T) {
	s1 := computeSignature("content-hash-1", "secret-key")
	s2 := computeSignature("content-hash-1", "secret-key")
	s3 := computeSignature("content-hash-1", "different-key")

	if s1 != s2 {
		t.Error("Same inputs should produce same signature")
	}
	if s1 == s3 {
		t.Error("Different keys should produce different signatures")
	}
}

// TestEnvelopeMultipleRules 多规则信封
func TestEnvelopeMultipleRules(t *testing.T) {
	em := newTestEnvelopeManager(t)
	rules := []string{"rule_injection", "rule_jailbreak", "rule_pii"}
	env := em.Seal("trace-multi", "inbound", "attack content", "block", rules, "attacker")

	if len(env.Rules) != 3 {
		t.Errorf("Rules count = %d, want 3", len(env.Rules))
	}

	// 验证存储后加载也正确
	result, err := em.Verify(env.ID)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !result.Valid {
		t.Error("Expected valid")
	}

	list, _ := em.ListByTrace("trace-multi")
	if len(list) != 1 {
		t.Fatalf("Expected 1 envelope, got %d", len(list))
	}
	if len(list[0].Rules) != 3 {
		t.Errorf("Loaded rules count = %d, want 3", len(list[0].Rules))
	}
}

// TestEnvelopeUpdateSecretKey 热更新密钥
func TestEnvelopeUpdateSecretKey(t *testing.T) {
	em := newTestEnvelopeManager(t)

	// 用旧密钥签名
	env1 := em.Seal("trace-key", "inbound", "msg", "pass", nil, "")
	result1, _ := em.Verify(env1.ID)
	if !result1.Valid {
		t.Error("Should be valid with original key")
	}

	// 更新密钥
	em.UpdateSecretKey("new-secret-key-completely-different!!")

	// 旧信封用新密钥验证应失败
	result2, _ := em.Verify(env1.ID)
	if result2.Valid {
		t.Error("Old envelope should fail with new key")
	}

	// 新信封用新密钥验证应成功
	env2 := em.Seal("trace-key2", "inbound", "msg2", "pass", nil, "")
	result3, _ := em.Verify(env2.ID)
	if !result3.Valid {
		t.Errorf("New envelope should be valid with new key: %v", result3.FailureReasons)
	}
}

// TestEnvelopeDomainVariety 不同 domain 类型
func TestEnvelopeDomainVariety(t *testing.T) {
	em := newTestEnvelopeManager(t)
	domains := []string{"inbound", "outbound", "llm_request", "llm_response"}

	for _, d := range domains {
		env := em.Seal("trace-domain", d, "content", "pass", nil, "")
		if env.Domain != d {
			t.Errorf("Domain = %s, want %s", env.Domain, d)
		}
		result, _ := em.Verify(env.ID)
		if !result.Valid {
			t.Errorf("Envelope for domain %s should be valid", d)
		}
	}

	stats := em.Stats()
	byDomain := stats["by_domain"].(map[string]int64)
	for _, d := range domains {
		if byDomain[d] != 1 {
			t.Errorf("by_domain[%s] = %d, want 1", d, byDomain[d])
		}
	}
}

// TestEnvelopeEmptyContent 空内容信封
func TestEnvelopeEmptyContent(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-empty", "inbound", "", "pass", []string{}, "")
	if env == nil {
		t.Fatal("Seal should not return nil for empty content")
	}
	result, _ := em.Verify(env.ID)
	if !result.Valid {
		t.Error("Empty content envelope should be valid")
	}
}

// TestEnvelopeTamperedContentHashOnly 只篡改 content_hash
func TestEnvelopeTamperedContentHashOnly(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-tamper-ch", "inbound", "secret", "block", nil, "")

	// 篡改 content_hash
	em.db.Exec(`UPDATE execution_envelopes SET content_hash = 'aaaa' WHERE id = ?`, env.ID)

	result, _ := em.Verify(env.ID)
	if result.Valid {
		t.Error("Should be invalid after content_hash tampering")
	}
	// Should have both content_hash and signature mismatch
	if len(result.FailureReasons) < 1 {
		t.Error("Expected at least 1 failure reason")
	}
}

// TestEnvelopeTamperedRules 篡改 rules 后验证失败
func TestEnvelopeTamperedRules(t *testing.T) {
	em := newTestEnvelopeManager(t)
	env := em.Seal("trace-tamper-rules", "inbound", "msg", "block", []string{"rule_a"}, "")

	// 篡改 rules_json
	em.db.Exec(`UPDATE execution_envelopes SET rules_json = '["rule_a","rule_b"]' WHERE id = ?`, env.ID)

	// 注意：rules_json 不参与 content_hash 计算（content_hash 是基于 rules 字段拼接的）
	// 所以需要验证 rules 是否影响 content_hash
	// 实际上 rules 通过 join 参与了 content_hash 计算
	// 但是从 DB 加载时 rules 来自 rules_json，所以篡改 rules_json 会改变重算的 content_hash

	// Reload: loadEnvelope 会从 rules_json 解析 rules
	// 然后 computeContentHash 会用这些 rules
	// 所以篡改 rules_json → 重算的 content_hash 会不同 → 验证失败
	result, _ := em.Verify(env.ID)
	if result.Valid {
		t.Error("Should be invalid after rules tampering")
	}
}

// TestEnvelopeStatsEmpty 空数据库统计
func TestEnvelopeStatsEmpty(t *testing.T) {
	em := newTestEnvelopeManager(t)
	stats := em.Stats()
	if stats["total"].(int64) != 0 {
		t.Errorf("total = %v, want 0", stats["total"])
	}
}

// ============================================================
// v18.0+ Merkle Tree 测试
// ============================================================

// newTestEnvelopeManagerWithBatch 创建测试用信封管理器（指定批次大小）
func newTestEnvelopeManagerWithBatch(t *testing.T, batchSize int) *EnvelopeManager {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewEnvelopeManagerWithBatchSize(db, "test-secret-key-at-least-32-chars!!", batchSize)
}

// TestMerkleBuildTree 基本树构建
func TestMerkleBuildTree(t *testing.T) {
	// 4 叶子 → 3 层（叶子层 + 中间层 + 根层）
	leaves := []string{"aaa", "bbb", "ccc", "ddd"}
	layers := BuildMerkleTree(leaves)

	if len(layers) != 3 {
		t.Fatalf("Expected 3 layers, got %d", len(layers))
	}
	if len(layers[0]) != 4 {
		t.Errorf("Leaf layer should have 4 elements, got %d", len(layers[0]))
	}
	if len(layers[1]) != 2 {
		t.Errorf("Middle layer should have 2 elements, got %d", len(layers[1]))
	}
	if len(layers[2]) != 1 {
		t.Errorf("Root layer should have 1 element, got %d", len(layers[2]))
	}

	// 根不为空
	root := ComputeMerkleRoot(leaves)
	if root == "" {
		t.Error("Root should not be empty")
	}

	// 相同输入 = 相同根
	root2 := ComputeMerkleRoot(leaves)
	if root != root2 {
		t.Error("Same leaves should produce same root")
	}

	// 不同输入 = 不同根
	leaves2 := []string{"aaa", "bbb", "ccc", "eee"}
	root3 := ComputeMerkleRoot(leaves2)
	if root == root3 {
		t.Error("Different leaves should produce different root")
	}

	// 空叶子
	rootEmpty := ComputeMerkleRoot([]string{})
	if rootEmpty != "" {
		t.Error("Empty leaves should produce empty root")
	}
}

// TestMerkleProof 证明生成和验证
func TestMerkleProof(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 4)

	// 创建 4 个信封触发自动批次构建
	var envIDs []string
	for i := 0; i < 4; i++ {
		env := em.Seal("trace-merkle-proof", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		envIDs = append(envIDs, env.ID)
	}

	// 验证每个信封都能生成有效的 proof
	for _, eid := range envIDs {
		proof, err := em.GenerateProof(eid)
		if err != nil {
			t.Fatalf("GenerateProof(%s) error: %v", eid, err)
		}
		if !proof.Verified {
			t.Errorf("Proof for %s should be verified", eid)
		}
		if proof.BatchID == "" {
			t.Errorf("BatchID should not be empty for %s", eid)
		}
		if proof.Root == "" {
			t.Errorf("Root should not be empty for %s", eid)
		}
		if len(proof.Path) == 0 {
			t.Errorf("Path should not be empty for %s", eid)
		}

		// 独立验证（不需要数据库）
		if !VerifyMerkleProof(proof) {
			t.Errorf("VerifyMerkleProof should return true for %s", eid)
		}
	}
}

// TestMerkleProofTampered 篡改后证明失败
func TestMerkleProofTampered(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 4)

	// 创建 4 个信封
	var envIDs []string
	for i := 0; i < 4; i++ {
		env := em.Seal("trace-merkle-tamper", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		envIDs = append(envIDs, env.ID)
	}

	// 获取第一个信封的 proof
	proof, err := em.GenerateProof(envIDs[0])
	if err != nil {
		t.Fatalf("GenerateProof error: %v", err)
	}
	if !proof.Verified {
		t.Fatal("Original proof should be verified")
	}

	// 篡改 ContentHash
	tamperedProof := *proof
	tamperedProof.ContentHash = "tampered_hash_value"
	if VerifyMerkleProof(&tamperedProof) {
		t.Error("Tampered proof (content hash) should fail verification")
	}

	// 篡改 Root
	tamperedProof2 := *proof
	tamperedProof2.Root = "tampered_root_value"
	if VerifyMerkleProof(&tamperedProof2) {
		t.Error("Tampered proof (root) should fail verification")
	}

	// 篡改 Path
	tamperedProof3 := *proof
	tamperedProof3.Path = []ProofStep{{Hash: "fake_hash", Position: "right"}}
	if VerifyMerkleProof(&tamperedProof3) {
		t.Error("Tampered proof (path) should fail verification")
	}
}

// TestMerkleBatchAutoFlush 自动构建触发
func TestMerkleBatchAutoFlush(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 4)

	// 创建 3 个信封（不足批次大小）
	for i := 0; i < 3; i++ {
		em.Seal("trace-auto", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	// 应该还没有批次
	batches, _ := em.ListBatches(10)
	if len(batches) != 0 {
		t.Errorf("Expected 0 batches before reaching batch size, got %d", len(batches))
	}

	// 第 4 个信封触发自动构建
	em.Seal("trace-auto", "inbound", "msg-3", "pass", nil, "")

	batches, _ = em.ListBatches(10)
	if len(batches) != 1 {
		t.Errorf("Expected 1 batch after reaching batch size, got %d", len(batches))
	}
	if batches[0].LeafCount != 4 {
		t.Errorf("Batch leaf count = %d, want 4", batches[0].LeafCount)
	}
}

// TestMerkleBatchManualFlush 手动 Flush
func TestMerkleBatchManualFlush(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 10)

	// 创建 3 个信封（不足批次大小）
	for i := 0; i < 3; i++ {
		em.Seal("trace-flush", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	// 手动 flush
	err := em.FlushBatch()
	if err != nil {
		t.Fatalf("FlushBatch error: %v", err)
	}

	batches, _ := em.ListBatches(10)
	if len(batches) != 1 {
		t.Fatalf("Expected 1 batch after manual flush, got %d", len(batches))
	}
	if batches[0].LeafCount != 3 {
		t.Errorf("Batch leaf count = %d, want 3", batches[0].LeafCount)
	}

	// 再次 flush（没有 pending）不应创建新批次
	em.FlushBatch()
	batches, _ = em.ListBatches(10)
	if len(batches) != 1 {
		t.Errorf("Expected still 1 batch after empty flush, got %d", len(batches))
	}
}

// TestMerkleRootChain 批次间根链验证
func TestMerkleRootChain(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 2)

	// 创建 6 个信封 → 3 个批次
	for i := 0; i < 6; i++ {
		em.Seal("trace-root-chain", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	batches, _ := em.ListBatches(10)
	if len(batches) != 3 {
		t.Fatalf("Expected 3 batches, got %d", len(batches))
	}

	// 验证根链
	result, err := em.VerifyRootChain()
	if err != nil {
		t.Fatalf("VerifyRootChain error: %v", err)
	}
	if !result.Valid {
		t.Errorf("Root chain should be valid: %v", result.Details)
	}
	if result.TotalBatches != 3 {
		t.Errorf("TotalBatches = %d, want 3", result.TotalBatches)
	}
}

// TestMerkleBatchVerify 整批验证
func TestMerkleBatchVerify(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 4)

	// 创建 4 个信封
	for i := 0; i < 4; i++ {
		em.Seal("trace-batch-verify", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	batches, _ := em.ListBatches(10)
	if len(batches) == 0 {
		t.Fatal("Expected at least 1 batch")
	}

	result, err := em.VerifyBatch(batches[0].ID)
	if err != nil {
		t.Fatalf("VerifyBatch error: %v", err)
	}
	if !result.Valid {
		t.Errorf("Batch should be valid: %v", result.FailureReasons)
	}
	if result.LeafCount != 4 {
		t.Errorf("LeafCount = %d, want 4", result.LeafCount)
	}

	// 篡改批次中某个信封后验证应失败
	em.db.Exec(`UPDATE execution_envelopes SET decision = 'block' WHERE id = ?`, batches[0].EnvelopeIDs[0])
	result2, _ := em.VerifyBatch(batches[0].ID)
	if result2.Valid {
		t.Error("Batch should be invalid after tampering an envelope")
	}
}

// TestMerkleOddLeaves 奇数叶子处理
func TestMerkleOddLeaves(t *testing.T) {
	// 3 叶子（奇数）
	leaves := []string{"aaa", "bbb", "ccc"}
	root := ComputeMerkleRoot(leaves)
	if root == "" {
		t.Error("Root should not be empty for odd leaves")
	}

	// 5 叶子
	leaves5 := []string{"a", "b", "c", "d", "e"}
	root5 := ComputeMerkleRoot(leaves5)
	if root5 == "" {
		t.Error("Root should not be empty for 5 leaves")
	}

	// 验证可以生成 proof
	for i := 0; i < len(leaves); i++ {
		path := GenerateMerkleProofFromLeaves(leaves, i)
		if len(path) == 0 {
			t.Errorf("Proof path should not be empty for leaf %d", i)
		}
		// 验证 proof
		proof := &MerkleProof{
			ContentHash: leaves[i],
			Root:        root,
			Path:        path,
		}
		if !VerifyMerkleProof(proof) {
			t.Errorf("Proof verification failed for leaf %d", i)
		}
	}

	// 同样验证 5 叶子
	for i := 0; i < len(leaves5); i++ {
		path := GenerateMerkleProofFromLeaves(leaves5, i)
		if len(path) == 0 {
			t.Errorf("Proof path should not be empty for leaf5 %d", i)
		}
		proof := &MerkleProof{
			ContentHash: leaves5[i],
			Root:        root5,
			Path:        path,
		}
		if !VerifyMerkleProof(proof) {
			t.Errorf("Proof verification failed for leaf5 %d", i)
		}
	}
}

// TestMerkleSingleLeaf 单叶退化
func TestMerkleSingleLeaf(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 10)

	// 创建 1 个信封 + flush
	env := em.Seal("trace-single", "inbound", "single-msg", "pass", nil, "")
	em.FlushBatch()

	batches, _ := em.ListBatches(10)
	if len(batches) != 1 {
		t.Fatalf("Expected 1 batch, got %d", len(batches))
	}
	if batches[0].LeafCount != 1 {
		t.Errorf("LeafCount = %d, want 1", batches[0].LeafCount)
	}

	// 生成 proof
	proof, err := em.GenerateProof(env.ID)
	if err != nil {
		t.Fatalf("GenerateProof error: %v", err)
	}
	if !proof.Verified {
		t.Error("Single leaf proof should be verified")
	}
	if !VerifyMerkleProof(proof) {
		t.Error("VerifyMerkleProof should return true for single leaf")
	}

	// 单叶 root 验证
	root := ComputeMerkleRoot([]string{env.ContentHash})
	if root == "" {
		t.Error("Single leaf root should not be empty")
	}
	if root != batches[0].Root {
		t.Errorf("Single leaf root mismatch: computed=%s, batch=%s", root, batches[0].Root)
	}
}

// TestMerkleStatsIncludeBatches 统计包含批次信息
func TestMerkleStatsIncludeBatches(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 2)

	// 创建 4 个信封 → 2 个批次
	for i := 0; i < 4; i++ {
		em.Seal("trace-stats", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	stats := em.Stats()

	// 总信封数
	if stats["total"].(int64) != 4 {
		t.Errorf("total = %v, want 4", stats["total"])
	}

	// Merkle 批次数
	batchCount, ok := stats["merkle_batches"].(int64)
	if !ok {
		t.Fatal("merkle_batches stat should exist")
	}
	if batchCount != 2 {
		t.Errorf("merkle_batches = %d, want 2", batchCount)
	}

	// pending leaves
	pendingCount, ok := stats["pending_leaves"].(int64)
	if !ok {
		t.Fatal("pending_leaves stat should exist")
	}
	if pendingCount != 0 {
		t.Errorf("pending_leaves = %d, want 0 (all flushed)", pendingCount)
	}

	// batch size
	batchSizeStat, ok := stats["batch_size"].(int64)
	if !ok {
		t.Fatal("batch_size stat should exist")
	}
	if batchSizeStat != 2 {
		t.Errorf("batch_size = %d, want 2", batchSizeStat)
	}
}

// TestMerkleVerifyProofStandalone 独立验证 Proof（不需要数据库）
func TestMerkleVerifyProofStandalone(t *testing.T) {
	// 手工构建一个 proof 并验证
	leaves := []string{"hash_a", "hash_b", "hash_c", "hash_d"}
	root := ComputeMerkleRoot(leaves)

	for i, leaf := range leaves {
		path := GenerateMerkleProofFromLeaves(leaves, i)
		proof := &MerkleProof{
			EnvelopeID:  fmt.Sprintf("env-%d", i),
			ContentHash: leaf,
			BatchID:     "batch-001",
			Root:        root,
			Path:        path,
		}
		if !VerifyMerkleProof(proof) {
			t.Errorf("Standalone proof verification failed for leaf %d", i)
		}
	}

	// nil proof
	if VerifyMerkleProof(nil) {
		t.Error("nil proof should return false")
	}

	// empty path proof
	emptyProof := &MerkleProof{
		ContentHash: "hash_a",
		Root:        root,
		Path:        []ProofStep{},
	}
	if VerifyMerkleProof(emptyProof) {
		t.Error("empty path proof should return false")
	}
}

// TestMerkleMultipleBatchesAndProofs 跨多个批次的 proof 验证
func TestMerkleMultipleBatchesAndProofs(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 2)

	// 创建 6 个信封 → 3 个批次
	var envIDs []string
	for i := 0; i < 6; i++ {
		env := em.Seal("trace-multi-batch", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
		envIDs = append(envIDs, env.ID)
	}

	// 每个信封都应该在某个批次中
	for _, eid := range envIDs {
		proof, err := em.GenerateProof(eid)
		if err != nil {
			t.Fatalf("GenerateProof(%s) error: %v", eid, err)
		}
		if !proof.Verified {
			t.Errorf("Proof for %s should be verified", eid)
		}
	}

	// 验证整条根链
	chainResult, err := em.VerifyRootChain()
	if err != nil {
		t.Fatalf("VerifyRootChain error: %v", err)
	}
	if !chainResult.Valid {
		t.Errorf("Root chain should be valid: %v", chainResult.Details)
	}
}

// TestMerkleBatchRootChainBroken 篡改批次根链后验证失败
func TestMerkleBatchRootChainBroken(t *testing.T) {
	em := newTestEnvelopeManagerWithBatch(t, 2)

	// 创建 4 个信封 → 2 个批次
	for i := 0; i < 4; i++ {
		em.Seal("trace-broken-chain", "inbound", fmt.Sprintf("msg-%d", i), "pass", nil, "")
	}

	batches, _ := em.ListBatches(10)
	if len(batches) != 2 {
		t.Fatalf("Expected 2 batches, got %d", len(batches))
	}

	// 篡改第一个批次的 root（batches 按时间倒序，所以索引 1 是较早的）
	em.db.Exec(`UPDATE merkle_batches SET root = 'tampered_root' WHERE id = ?`, batches[1].ID)

	result, err := em.VerifyRootChain()
	if err != nil {
		t.Fatalf("VerifyRootChain error: %v", err)
	}
	if result.Valid {
		t.Error("Root chain should be invalid after tampering")
	}
	if len(result.Details) == 0 {
		t.Error("Expected details about the broken chain")
	}
}
