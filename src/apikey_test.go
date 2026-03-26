// apikey_test.go — API Key 管理器测试
// lobster-guard v27.0 — API Key 身份管理
package main

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupAPIKeyTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAPIKeyCreate(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	defer db.Close()
	mgr := NewAPIKeyManager(db)

	entry := &APIKeyEntry{
		UserID:     "zhangsan@qianxin.com",
		UserName:   "张三",
		Department: "安全部",
		TenantID:   "tenant-a",
		QuotaDaily: 100,
	}

	created, rawKey, err := mgr.Create(entry)
	if err != nil {
		t.Fatalf("创建 API Key 失败: %v", err)
	}
	if rawKey == "" {
		t.Fatal("原始 Key 不应为空")
	}
	if created.ID == "" {
		t.Fatal("ID 不应为空")
	}
	if created.KeyPrefix == "" {
		t.Fatal("KeyPrefix 不应为空")
	}
	if len(created.KeyPrefix) < 5 {
		t.Fatalf("KeyPrefix 太短: %s", created.KeyPrefix)
	}
	if created.UserID != "zhangsan@qianxin.com" {
		t.Fatalf("UserID 不匹配: got %s", created.UserID)
	}
	if created.TenantID != "tenant-a" {
		t.Fatalf("TenantID 不匹配: got %s", created.TenantID)
	}

	t.Logf("✅ 创建成功: id=%s prefix=%s rawKey=%s...%s", created.ID, created.KeyPrefix, rawKey[:10], rawKey[len(rawKey)-4:])
}

func TestAPIKeyResolve(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	defer db.Close()
	mgr := NewAPIKeyManager(db)

	entry := &APIKeyEntry{
		UserID:   "lisi@qianxin.com",
		UserName: "李四",
		TenantID: "tenant-b",
	}
	_, rawKey, err := mgr.Create(entry)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 直接用 rawKey 解析
	resolved, err := mgr.Resolve(rawKey)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if resolved.UserID != "lisi@qianxin.com" {
		t.Fatalf("UserID 不匹配: got %s", resolved.UserID)
	}
	if resolved.TenantID != "tenant-b" {
		t.Fatalf("TenantID 不匹配: got %s", resolved.TenantID)
	}

	// 用 Bearer 前缀解析
	resolved2, err := mgr.Resolve("Bearer " + rawKey)
	if err != nil {
		t.Fatalf("Resolve with Bearer 失败: %v", err)
	}
	if resolved2.UserID != "lisi@qianxin.com" {
		t.Fatalf("Bearer 模式 UserID 不匹配: got %s", resolved2.UserID)
	}

	// 不存在的 Key
	_, err = mgr.Resolve("sk-nonexistent1234567890")
	if err == nil {
		t.Fatal("不存在的 Key 应返回错误")
	}

	// 空 Key
	_, err = mgr.Resolve("")
	if err == nil {
		t.Fatal("空 Key 应返回错误")
	}

	t.Logf("✅ Resolve 成功")
}

func TestAPIKeyQuota(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	defer db.Close()
	mgr := NewAPIKeyManager(db)

	entry := &APIKeyEntry{
		UserID:     "quota-user@test.com",
		TenantID:   "default",
		QuotaDaily: 3,
	}
	created, _, err := mgr.Create(entry)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 未使用前应允许
	if !mgr.CheckQuota(created.ID) {
		t.Fatal("未使用时配额检查应通过")
	}

	// 使用 3 次
	mgr.IncrUsage(created.ID)
	mgr.IncrUsage(created.ID)
	mgr.IncrUsage(created.ID)

	// 第 4 次应拒绝
	if mgr.CheckQuota(created.ID) {
		t.Fatal("配额用完后应拒绝")
	}

	t.Logf("✅ 配额检查成功")
}

func TestAPIKeyExpiry(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	defer db.Close()
	mgr := NewAPIKeyManager(db)

	// 创建一个已过期的 Key
	entry := &APIKeyEntry{
		UserID:    "expired-user@test.com",
		TenantID:  "default",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
	}
	_, rawKey, err := mgr.Create(entry)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 解析应失败（过期）
	_, err = mgr.Resolve(rawKey)
	if err == nil {
		t.Fatal("过期 Key 应返回错误")
	}

	// 创建一个未过期的 Key
	entry2 := &APIKeyEntry{
		UserID:    "valid-user@test.com",
		TenantID:  "default",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	}
	_, rawKey2, err := mgr.Create(entry2)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 解析应成功
	resolved, err := mgr.Resolve(rawKey2)
	if err != nil {
		t.Fatalf("有效 Key 解析失败: %v", err)
	}
	if resolved.UserID != "valid-user@test.com" {
		t.Fatalf("UserID 不匹配: got %s", resolved.UserID)
	}

	t.Logf("✅ 过期检查成功")
}

func TestAPIKeyRotate(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	defer db.Close()
	mgr := NewAPIKeyManager(db)

	entry := &APIKeyEntry{
		UserID:   "rotate-user@test.com",
		TenantID: "default",
	}
	created, oldKey, err := mgr.Create(entry)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 轮换
	rotated, newKey, err := mgr.Rotate(created.ID)
	if err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	if newKey == oldKey {
		t.Fatal("新旧 Key 不应相同")
	}
	if rotated.KeyPrefix == "" {
		t.Fatal("轮换后 KeyPrefix 不应为空")
	}

	// 旧 Key 应失效
	_, err = mgr.Resolve(oldKey)
	if err == nil {
		t.Fatal("旧 Key 应失效")
	}

	// 新 Key 应可用
	resolved, err := mgr.Resolve(newKey)
	if err != nil {
		t.Fatalf("新 Key 解析失败: %v", err)
	}
	if resolved.UserID != "rotate-user@test.com" {
		t.Fatalf("UserID 不匹配: got %s", resolved.UserID)
	}

	t.Logf("✅ 轮换成功: old_prefix=%s new_prefix=%s", oldKey[:10], newKey[:10])
}
