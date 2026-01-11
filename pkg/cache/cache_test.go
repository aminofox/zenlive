package cache

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache(100, 1*time.Minute, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "value1" {
		t.Errorf("Expected 'value1', got '%v'", value)
	}

	// Check existence
	exists, err := cache.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if !exists {
		t.Error("Expected key1 to exist")
	}

	// Delete
	err = cache.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, err = cache.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if exists {
		t.Error("Expected key1 to not exist after delete")
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := NewInMemoryCache(100, 100*time.Millisecond, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Set a value with short TTL
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	exists, err := cache.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}

	if !exists {
		t.Error("Expected key1 to exist")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error when getting expired key")
	}
}

func TestCacheEviction(t *testing.T) {
	cache := NewInMemoryCache(3, 1*time.Minute, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Fill cache to capacity
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Access key1 to make it recently used
	cache.Get(ctx, "key1")

	// Add another key - should evict least recently used (key2 or key3)
	cache.Set(ctx, "key4", "value4", 0)

	// Check stats
	stats, err := cache.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Size != 3 {
		t.Errorf("Expected size 3, got %d", stats.Size)
	}

	if stats.Evictions == 0 {
		t.Error("Expected at least 1 eviction")
	}
}

func TestCacheStats(t *testing.T) {
	cache := NewInMemoryCache(100, 1*time.Minute, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Add some data
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// Generate hits and misses
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key1") // hit
	cache.Get(ctx, "key3") // miss

	stats, err := cache.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	expectedHitRate := 2.0 / 3.0
	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("Expected hit rate %.2f, got %.2f", expectedHitRate, stats.HitRate)
	}
}

func TestMultiLevelCache(t *testing.T) {
	l1 := NewInMemoryCache(10, 1*time.Minute, EvictionPolicyLRU)
	l1.Start()
	defer l1.Stop()

	l2 := NewInMemoryCache(100, 5*time.Minute, EvictionPolicyLRU)
	l2.Start()
	defer l2.Stop()

	mlc := NewMultiLevelCache(l1, l2)
	ctx := context.Background()

	// Set in multi-level cache
	err := mlc.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be in both levels
	value1, err := l1.Get(ctx, "key1")
	if err != nil {
		t.Error("Expected key1 in L1 cache")
	}

	value2, err := l2.Get(ctx, "key1")
	if err != nil {
		t.Error("Expected key1 in L2 cache")
	}

	if value1 != value2 {
		t.Error("Values in L1 and L2 should be the same")
	}

	// Remove from L1
	l1.Delete(ctx, "key1")

	// Get from multi-level - should promote from L2 to L1
	value, err := mlc.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "value1" {
		t.Errorf("Expected 'value1', got '%v'", value)
	}

	// Should now be in L1 again (promoted)
	_, err = l1.Get(ctx, "key1")
	if err != nil {
		t.Error("Expected key1 to be promoted to L1")
	}
}

func TestCacheKeys(t *testing.T) {
	cache := NewInMemoryCache(100, 1*time.Minute, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Add multiple keys
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Get keys
	keys, err := cache.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Verify all keys are present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	if !keyMap["key1"] || !keyMap["key2"] || !keyMap["key3"] {
		t.Error("Not all keys found")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewInMemoryCache(100, 1*time.Minute, EvictionPolicyLRU)
	cache.Start()
	defer cache.Stop()

	ctx := context.Background()

	// Add data
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	// Clear
	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify empty
	keys, err := cache.Keys(ctx)
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after clear, got %d", len(keys))
	}
}
