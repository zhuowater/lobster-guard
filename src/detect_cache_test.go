// detect_cache_test.go — DetectCache 测试（v5.1）
package main

import (
	"fmt"
	"testing"
	"time"
)

func TestDetectCache_BasicPutGet(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	result := DetectResult{Action: "pass", Reasons: []string{"clean"}}
	cache.Put("hello world", result)

	got, ok := cache.Get("hello world")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Action != "pass" {
		t.Errorf("expected pass, got %s", got.Action)
	}
}

func TestDetectCache_Miss(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	_, ok := cache.Get("not in cache")
	if ok {
		t.Error("expected cache miss")
	}

	hits, misses, _ := cache.Stats()
	if hits != 0 || misses != 1 {
		t.Errorf("expected 0 hits 1 miss, got %d hits %d misses", hits, misses)
	}
}

func TestDetectCache_BlockNotCached(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	result := DetectResult{Action: "block", Reasons: []string{"attack"}}
	cache.Put("attack text", result)

	_, ok := cache.Get("attack text")
	if ok {
		t.Error("block results should not be cached")
	}
}

func TestDetectCache_WarnCached(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	result := DetectResult{Action: "warn", Reasons: []string{"suspicious"}}
	cache.Put("warn text", result)

	got, ok := cache.Get("warn text")
	if !ok {
		t.Fatal("warn results should be cached")
	}
	if got.Action != "warn" {
		t.Errorf("expected warn, got %s", got.Action)
	}
}

func TestDetectCache_TTLExpiry(t *testing.T) {
	cache := NewDetectCache(100, 50*time.Millisecond) // very short TTL

	result := DetectResult{Action: "pass"}
	cache.Put("test", result)

	// Should hit immediately
	_, ok := cache.Get("test")
	if !ok {
		t.Fatal("expected cache hit")
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	_, ok = cache.Get("test")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestDetectCache_LRUEviction(t *testing.T) {
	cache := NewDetectCache(3, 10*time.Second) // capacity of 3

	cache.Put("a", DetectResult{Action: "pass"})
	cache.Put("b", DetectResult{Action: "pass"})
	cache.Put("c", DetectResult{Action: "pass"})
	cache.Put("d", DetectResult{Action: "pass"}) // should evict "a"

	_, ok := cache.Get("a")
	if ok {
		t.Error("expected 'a' to be evicted")
	}

	_, ok = cache.Get("b")
	if !ok {
		t.Error("expected 'b' to still be cached")
	}
}

func TestDetectCache_LRUAccess(t *testing.T) {
	cache := NewDetectCache(3, 10*time.Second)

	cache.Put("a", DetectResult{Action: "pass"})
	cache.Put("b", DetectResult{Action: "pass"})
	cache.Put("c", DetectResult{Action: "pass"})

	// Access "a" to make it recently used
	cache.Get("a")

	// Add "d" — should evict "b" (least recently used)
	cache.Put("d", DetectResult{Action: "pass"})

	_, ok := cache.Get("a")
	if !ok {
		t.Error("expected 'a' to still be cached (recently accessed)")
	}

	_, ok = cache.Get("b")
	if ok {
		t.Error("expected 'b' to be evicted (LRU)")
	}
}

func TestDetectCache_SameKeySHA256(t *testing.T) {
	// Same content should produce same key
	key1 := cacheKey("hello world")
	key2 := cacheKey("hello world")
	if key1 != key2 {
		t.Errorf("same content should produce same key: %s vs %s", key1, key2)
	}

	// Different content should produce different key
	key3 := cacheKey("different text")
	if key1 == key3 {
		t.Error("different content should produce different key")
	}

	// Key should be 32 chars (16 bytes hex)
	if len(key1) != 32 {
		t.Errorf("expected key length 32, got %d", len(key1))
	}
}

func TestDetectCache_Update(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	cache.Put("test", DetectResult{Action: "pass"})
	cache.Put("test", DetectResult{Action: "warn"})

	got, ok := cache.Get("test")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Action != "warn" {
		t.Errorf("expected warn (updated), got %s", got.Action)
	}
}

func TestDetectCache_Stats(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	cache.Put("a", DetectResult{Action: "pass"})
	cache.Get("a")   // hit
	cache.Get("b")   // miss
	cache.Get("a")   // hit

	hits, misses, size := cache.Stats()
	if hits != 2 {
		t.Errorf("expected 2 hits, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
	if size != 1 {
		t.Errorf("expected size 1, got %d", size)
	}
}

func TestDetectCache_Clear(t *testing.T) {
	cache := NewDetectCache(100, 10*time.Second)

	cache.Put("a", DetectResult{Action: "pass"})
	cache.Put("b", DetectResult{Action: "pass"})
	cache.Clear()

	_, ok := cache.Get("a")
	if ok {
		t.Error("expected miss after clear")
	}

	_, _, size := cache.Stats()
	if size != 0 {
		t.Errorf("expected size 0 after clear, got %d", size)
	}
}

func TestDetectCache_ConcurrentAccess(t *testing.T) {
	cache := NewDetectCache(1000, 10*time.Second)
	done := make(chan bool)

	// Writer
	go func() {
		for i := 0; i < 100; i++ {
			cache.Put(fmt.Sprintf("key%d", i), DetectResult{Action: "pass"})
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get(fmt.Sprintf("key%d", i))
		}
		done <- true
	}()

	<-done
	<-done
}
