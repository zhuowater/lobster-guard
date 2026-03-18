// merkle.go — Merkle Tree 密码学审计链（v18.0+）
// 替代单链结构，提供 O(log N) 验证、局部篡改定位、批量证明
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// MerkleNode Merkle 树节点
type MerkleNode struct {
	Hash  string `json:"hash"`
	Left  string `json:"left,omitempty"`  // 左子哈希
	Right string `json:"right,omitempty"` // 右子哈希
}

// MerkleBatch 一个 Merkle 批次
type MerkleBatch struct {
	ID          string    `json:"id"`
	Root        string    `json:"root"`           // Merkle Root Hash
	PrevRoot    string    `json:"prev_root"`      // 前一个批次的 Root（根链）
	Signature   string    `json:"signature"`      // HMAC-SHA256(Root + PrevRoot, secretKey)
	LeafCount   int       `json:"leaf_count"`
	LeafHashes  []string  `json:"leaf_hashes"`    // 叶子节点哈希列表（信封的 ContentHash）
	EnvelopeIDs []string  `json:"envelope_ids"`   // 信封 ID 列表
	CreatedAt   time.Time `json:"created_at"`
}

// MerkleProof 证明路径
type MerkleProof struct {
	EnvelopeID  string      `json:"envelope_id"`
	ContentHash string      `json:"content_hash"`
	BatchID     string      `json:"batch_id"`
	Root        string      `json:"root"`
	Verified    bool        `json:"verified"`
	Path        []ProofStep `json:"path"` // 从叶子到根的路径
}

// ProofStep 证明步骤
type ProofStep struct {
	Hash     string `json:"hash"`
	Position string `json:"position"` // "left" or "right"
}

// pendingLeaf 待处理的叶子（缓冲区中的信封）
type pendingLeaf struct {
	envelopeID  string
	contentHash string
}

// BuildMerkleTree 从叶子哈希列表构建 Merkle Tree，返回所有节点层级
// 返回值: layers[0] = 叶子层, layers[len-1] = 根层（单元素）
func BuildMerkleTree(leafHashes []string) [][]string {
	if len(leafHashes) == 0 {
		return nil
	}

	// 复制一份，避免修改原数组
	current := make([]string, len(leafHashes))
	copy(current, leafHashes)

	// 如果叶子数为奇数，复制最后一个
	if len(current)%2 != 0 {
		current = append(current, current[len(current)-1])
	}

	layers := [][]string{current}

	for len(current) > 1 {
		var next []string
		for i := 0; i < len(current); i += 2 {
			combined := current[i] + current[i+1]
			h := sha256.Sum256([]byte(combined))
			next = append(next, hex.EncodeToString(h[:]))
		}
		// 如果中间层为奇数且不是根，复制最后一个
		if len(next) > 1 && len(next)%2 != 0 {
			next = append(next, next[len(next)-1])
		}
		layers = append(layers, next)
		current = next
	}

	return layers
}

// ComputeMerkleRoot 计算 Merkle Root
func ComputeMerkleRoot(leafHashes []string) string {
	if len(leafHashes) == 0 {
		return ""
	}
	if len(leafHashes) == 1 {
		// 单叶退化：自己和自己合并
		combined := leafHashes[0] + leafHashes[0]
		h := sha256.Sum256([]byte(combined))
		return hex.EncodeToString(h[:])
	}
	layers := BuildMerkleTree(leafHashes)
	if len(layers) == 0 {
		return ""
	}
	topLayer := layers[len(layers)-1]
	return topLayer[0]
}

// GenerateMerkleProofFromLeaves 从叶子哈希列表生成指定叶子的 Merkle Proof
func GenerateMerkleProofFromLeaves(leafHashes []string, targetIndex int) []ProofStep {
	if len(leafHashes) == 0 || targetIndex < 0 || targetIndex >= len(leafHashes) {
		return nil
	}

	if len(leafHashes) == 1 {
		// 单叶退化：proof path 包含自身作为 sibling
		return []ProofStep{{Hash: leafHashes[0], Position: "right"}}
	}

	layers := BuildMerkleTree(leafHashes)
	if len(layers) == 0 {
		return nil
	}

	var proof []ProofStep
	idx := targetIndex
	// 如果原始叶子数为奇数，索引可能需要在补齐后的层中调整
	// layers[0] 已经是补齐后的叶子层

	for layerIdx := 0; layerIdx < len(layers)-1; layerIdx++ {
		layer := layers[layerIdx]
		if idx%2 == 0 {
			// 当前在左侧，sibling 在右侧
			siblingIdx := idx + 1
			if siblingIdx < len(layer) {
				proof = append(proof, ProofStep{
					Hash:     layer[siblingIdx],
					Position: "right",
				})
			}
		} else {
			// 当前在右侧，sibling 在左侧
			siblingIdx := idx - 1
			proof = append(proof, ProofStep{
				Hash:     layer[siblingIdx],
				Position: "left",
			})
		}
		idx = idx / 2
	}

	return proof
}

// VerifyMerkleProof 独立验证一个 Merkle Proof（不需要数据库）
func VerifyMerkleProof(proof *MerkleProof) bool {
	if proof == nil || len(proof.Path) == 0 {
		return false
	}

	current := proof.ContentHash
	for _, step := range proof.Path {
		var combined string
		if step.Position == "left" {
			combined = step.Hash + current
		} else {
			combined = current + step.Hash
		}
		h := sha256.Sum256([]byte(combined))
		current = hex.EncodeToString(h[:])
	}

	return current == proof.Root
}

// computeBatchSignature 计算批次签名 HMAC-SHA256(Root + PrevRoot, secretKey)
func computeBatchSignature(root, prevRoot, secretKey string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(root + prevRoot))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyBatchSignature 验证批次签名
func verifyBatchSignature(batch *MerkleBatch, secretKey string) bool {
	expected := computeBatchSignature(batch.Root, batch.PrevRoot, secretKey)
	return hmac.Equal([]byte(expected), []byte(batch.Signature))
}

// ============================================================
// EnvelopeManager Merkle 批次方法
// ============================================================

// initMerkleTables 初始化 Merkle 相关的 SQLite 表
func (em *EnvelopeManager) initMerkleTables() {
	em.db.Exec(`CREATE TABLE IF NOT EXISTS merkle_batches (
		id TEXT PRIMARY KEY,
		root TEXT NOT NULL,
		prev_root TEXT DEFAULT '',
		signature TEXT NOT NULL,
		leaf_count INTEGER NOT NULL,
		leaf_hashes_json TEXT NOT NULL,
		envelope_ids_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	em.db.Exec(`CREATE INDEX IF NOT EXISTS idx_merkle_batches_ts ON merkle_batches(created_at)`)

	// 信封表新增列（忽略 "duplicate column name" 错误）
	em.db.Exec(`ALTER TABLE execution_envelopes ADD COLUMN batch_id TEXT DEFAULT ''`)
}

// loadLastBatchRoot 从数据库加载最后一个批次的 Root
func (em *EnvelopeManager) loadLastBatchRoot() {
	row := em.db.QueryRow(`SELECT root FROM merkle_batches ORDER BY created_at DESC, rowid DESC LIMIT 1`)
	var root string
	if row.Scan(&root) == nil {
		em.prevBatchRoot = root
	}
}

// buildBatch 构建一个 Merkle 批次（调用者必须持有 mu 锁）
func (em *EnvelopeManager) buildBatch() error {
	if len(em.pendingLeaves) == 0 {
		return nil
	}

	// 提取叶子信息
	leaves := make([]pendingLeaf, len(em.pendingLeaves))
	copy(leaves, em.pendingLeaves)
	em.pendingLeaves = em.pendingLeaves[:0]

	leafHashes := make([]string, len(leaves))
	envelopeIDs := make([]string, len(leaves))
	for i, l := range leaves {
		leafHashes[i] = l.contentHash
		envelopeIDs[i] = l.envelopeID
	}

	// 构建 Merkle Tree
	root := ComputeMerkleRoot(leafHashes)

	// 创建批次
	batch := &MerkleBatch{
		ID:          generateEnvelopeID(),
		Root:        root,
		PrevRoot:    em.prevBatchRoot,
		LeafCount:   len(leafHashes),
		LeafHashes:  leafHashes,
		EnvelopeIDs: envelopeIDs,
		CreatedAt:   time.Now().UTC(),
	}
	batch.Signature = computeBatchSignature(batch.Root, batch.PrevRoot, em.secretKey)

	// 写入数据库
	leafHashesJSON, _ := json.Marshal(batch.LeafHashes)
	envelopeIDsJSON, _ := json.Marshal(batch.EnvelopeIDs)

	_, err := em.db.Exec(`INSERT INTO merkle_batches (id, root, prev_root, signature, leaf_count, leaf_hashes_json, envelope_ids_json, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		batch.ID,
		batch.Root,
		batch.PrevRoot,
		batch.Signature,
		batch.LeafCount,
		string(leafHashesJSON),
		string(envelopeIDsJSON),
		batch.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("写入 merkle_batches 失败: %w", err)
	}

	// 更新信封的 batch_id
	for _, eid := range envelopeIDs {
		em.db.Exec(`UPDATE execution_envelopes SET batch_id = ? WHERE id = ?`, batch.ID, eid)
	}

	// 更新根链
	em.prevBatchRoot = batch.Root

	return nil
}

// FlushBatch 强制构建当前不满的批次（用于关闭时或手动触发）
func (em *EnvelopeManager) FlushBatch() error {
	em.mu.Lock()
	defer em.mu.Unlock()
	return em.buildBatch()
}

// startAutoFlush 启动定时器，每 60 秒自动 flush pending leaves
func (em *EnvelopeManager) startAutoFlush() {
	em.flushTicker = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-em.flushTicker.C:
				em.mu.Lock()
				if len(em.pendingLeaves) > 0 {
					em.buildBatch()
				}
				em.mu.Unlock()
			case <-em.stopCh:
				return
			}
		}
	}()
}

// StopAutoFlush 停止自动 flush（flush 剩余 pending）
func (em *EnvelopeManager) StopAutoFlush() {
	if em.flushTicker != nil {
		em.flushTicker.Stop()
	}
	close(em.stopCh)
	// Flush remaining
	em.FlushBatch()
}

// GenerateProof 生成指定信封的 Merkle Proof
func (em *EnvelopeManager) GenerateProof(envelopeID string) (*MerkleProof, error) {
	// 加载信封
	env, err := em.loadEnvelope(envelopeID)
	if err != nil {
		return nil, fmt.Errorf("信封不存在: %s", envelopeID)
	}

	// 获取 batch_id
	var batchID string
	em.db.QueryRow(`SELECT batch_id FROM execution_envelopes WHERE id = ?`, envelopeID).Scan(&batchID)
	if batchID == "" {
		return nil, fmt.Errorf("信封 %s 尚未分配到批次（可能在 pending 缓冲区中）", envelopeID)
	}

	// 加载批次
	batch, err := em.loadBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("加载批次 %s 失败: %w", batchID, err)
	}

	// 找到信封在叶子列表中的位置
	targetIndex := -1
	for i, h := range batch.LeafHashes {
		if h == env.ContentHash {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return nil, fmt.Errorf("信封 %s 的 ContentHash 在批次 %s 中未找到", envelopeID, batchID)
	}

	// 生成 proof path
	path := GenerateMerkleProofFromLeaves(batch.LeafHashes, targetIndex)

	proof := &MerkleProof{
		EnvelopeID:  envelopeID,
		ContentHash: env.ContentHash,
		BatchID:     batchID,
		Root:        batch.Root,
		Path:        path,
	}

	// 验证
	proof.Verified = VerifyMerkleProof(proof)

	return proof, nil
}

// loadBatch 从数据库加载一个批次
func (em *EnvelopeManager) loadBatch(batchID string) (*MerkleBatch, error) {
	row := em.db.QueryRow(`SELECT id, root, prev_root, signature, leaf_count, leaf_hashes_json, envelope_ids_json, created_at FROM merkle_batches WHERE id = ?`, batchID)

	var batch MerkleBatch
	var leafHashesJSON, envelopeIDsJSON, createdAtStr string
	err := row.Scan(&batch.ID, &batch.Root, &batch.PrevRoot, &batch.Signature,
		&batch.LeafCount, &leafHashesJSON, &envelopeIDsJSON, &createdAtStr)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(leafHashesJSON), &batch.LeafHashes)
	json.Unmarshal([]byte(envelopeIDsJSON), &batch.EnvelopeIDs)
	batch.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)

	return &batch, nil
}

// VerifyBatch 验证整个批次的 Merkle Root 完整性
func (em *EnvelopeManager) VerifyBatch(batchID string) (*BatchVerifyResult, error) {
	batch, err := em.loadBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("批次 %s 不存在: %w", batchID, err)
	}

	result := &BatchVerifyResult{
		BatchID:   batchID,
		Valid:     true,
		LeafCount: batch.LeafCount,
	}

	// 1. 重建 Merkle Root
	recomputedRoot := ComputeMerkleRoot(batch.LeafHashes)
	if recomputedRoot != batch.Root {
		result.Valid = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Merkle Root 不匹配: recomputed=%s, stored=%s", recomputedRoot, batch.Root))
	}

	// 2. 验证批次签名
	if !verifyBatchSignature(batch, em.secretKey) {
		result.Valid = false
		result.FailureReasons = append(result.FailureReasons,
			"批次签名验证失败")
	}

	// 3. 验证每个叶子信封的完整性
	for i, eid := range batch.EnvelopeIDs {
		vr, err := em.Verify(eid)
		if err != nil || !vr.Valid {
			result.Valid = false
			reason := fmt.Sprintf("叶子 #%d 信封 %s 验证失败", i, eid)
			if err != nil {
				reason += ": " + err.Error()
			} else if len(vr.FailureReasons) > 0 {
				reason += ": " + vr.FailureReasons[0]
			}
			result.FailureReasons = append(result.FailureReasons, reason)
		}
	}

	return result, nil
}

// BatchVerifyResult 批次验证结果
type BatchVerifyResult struct {
	BatchID        string   `json:"batch_id"`
	Valid          bool     `json:"valid"`
	LeafCount      int      `json:"leaf_count"`
	FailureReasons []string `json:"failure_reasons,omitempty"`
}

// ListBatches 列出所有批次（按时间排序）
func (em *EnvelopeManager) ListBatches(limit int) ([]MerkleBatch, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := em.db.Query(`SELECT id, root, prev_root, signature, leaf_count, leaf_hashes_json, envelope_ids_json, created_at FROM merkle_batches ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batches []MerkleBatch
	for rows.Next() {
		var batch MerkleBatch
		var leafHashesJSON, envelopeIDsJSON, createdAtStr string
		err := rows.Scan(&batch.ID, &batch.Root, &batch.PrevRoot, &batch.Signature,
			&batch.LeafCount, &leafHashesJSON, &envelopeIDsJSON, &createdAtStr)
		if err != nil {
			continue
		}
		json.Unmarshal([]byte(leafHashesJSON), &batch.LeafHashes)
		json.Unmarshal([]byte(envelopeIDsJSON), &batch.EnvelopeIDs)
		batch.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		batches = append(batches, batch)
	}
	if batches == nil {
		batches = []MerkleBatch{}
	}
	return batches, nil
}

// VerifyRootChain 验证批次间的根链完整性
func (em *EnvelopeManager) VerifyRootChain() (*RootChainVerifyResult, error) {
	batches, err := em.db.Query(`SELECT id, root, prev_root, signature, leaf_count, leaf_hashes_json, envelope_ids_json, created_at FROM merkle_batches ORDER BY created_at ASC, rowid ASC`)
	if err != nil {
		return nil, err
	}
	defer batches.Close()

	result := &RootChainVerifyResult{
		Valid: true,
	}

	var prevRoot string
	var idx int
	for batches.Next() {
		var batch MerkleBatch
		var leafHashesJSON, envelopeIDsJSON, createdAtStr string
		err := batches.Scan(&batch.ID, &batch.Root, &batch.PrevRoot, &batch.Signature,
			&batch.LeafCount, &leafHashesJSON, &envelopeIDsJSON, &createdAtStr)
		if err != nil {
			continue
		}

		result.TotalBatches++

		// 验证 PrevRoot 链接
		if batch.PrevRoot != prevRoot {
			result.Valid = false
			if result.BrokenAt == 0 && idx > 0 {
				result.BrokenAt = idx
			}
			result.Details = append(result.Details,
				fmt.Sprintf("批次 #%d (%s) PrevRoot 不匹配: stored=%s, expected=%s", idx, batch.ID, batch.PrevRoot, prevRoot))
		}

		// 验证批次签名
		json.Unmarshal([]byte(leafHashesJSON), &batch.LeafHashes)
		if !verifyBatchSignature(&batch, em.secretKey) {
			result.Valid = false
			result.Details = append(result.Details,
				fmt.Sprintf("批次 #%d (%s) 签名验证失败", idx, batch.ID))
		}

		// 验证 Merkle Root
		recomputedRoot := ComputeMerkleRoot(batch.LeafHashes)
		if recomputedRoot != batch.Root {
			result.Valid = false
			result.Details = append(result.Details,
				fmt.Sprintf("批次 #%d (%s) Merkle Root 不匹配", idx, batch.ID))
		}

		prevRoot = batch.Root
		idx++
	}

	return result, nil
}

// RootChainVerifyResult 根链验证结果
type RootChainVerifyResult struct {
	Valid        bool     `json:"valid"`
	TotalBatches int     `json:"total_batches"`
	BrokenAt     int     `json:"broken_at,omitempty"`
	Details      []string `json:"details,omitempty"`
}
