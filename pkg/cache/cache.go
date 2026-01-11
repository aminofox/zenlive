package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CacheEntry represents a cache entry
type CacheEntry struct {
	Key         string      // Cache key
	Value       interface{} // Cached value
	ExpiresAt   time.Time   // Expiration time
	CreatedAt   time.Time   // Creation time
	AccessCount int64       // Number of times accessed
	LastAccess  time.Time   // Last access time
}

// IsExpired checks if the entry is expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Now().After(ce.ExpiresAt)
}

// Cache interface defines caching operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a value in the cache
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)

	// Clear clears all cache entries
	Clear(ctx context.Context) error

	// Keys returns all keys
	Keys(ctx context.Context) ([]string, error)

	// Stats returns cache statistics
	Stats(ctx context.Context) (CacheStats, error)
}

// CacheStats represents cache statistics
type CacheStats struct {
	Hits      int64   // Cache hits
	Misses    int64   // Cache misses
	Evictions int64   // Number of evictions
	Size      int     // Current cache size
	HitRate   float64 // Hit rate (hits / (hits + misses))
}

// InMemoryCache implements an in-memory cache
type InMemoryCache struct {
	entries        map[string]*CacheEntry
	maxSize        int
	defaultTTL     time.Duration
	evictionPolicy EvictionPolicy

	// Stats
	hits      int64
	misses    int64
	evictions int64

	mu       sync.RWMutex
	stopChan chan struct{}
	running  bool
}

// EvictionPolicy defines how entries are evicted
type EvictionPolicy string

const (
	// EvictionPolicyLRU evicts least recently used entries
	EvictionPolicyLRU EvictionPolicy = "lru"
	// EvictionPolicyLFU evicts least frequently used entries
	EvictionPolicyLFU EvictionPolicy = "lfu"
	// EvictionPolicyFIFO evicts oldest entries first
	EvictionPolicyFIFO EvictionPolicy = "fifo"
)

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxSize int, defaultTTL time.Duration, evictionPolicy EvictionPolicy) *InMemoryCache {
	cache := &InMemoryCache{
		entries:        make(map[string]*CacheEntry),
		maxSize:        maxSize,
		defaultTTL:     defaultTTL,
		evictionPolicy: evictionPolicy,
	}

	return cache
}

// Start starts the cache cleanup routine
func (c *InMemoryCache) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return
	}

	c.running = true
	c.stopChan = make(chan struct{})

	go c.cleanupLoop()
}

// Stop stops the cache
func (c *InMemoryCache) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopChan)
}

// Get retrieves a value from the cache
func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.misses++
		return nil, errors.New("key not found")
	}

	if entry.IsExpired() {
		delete(c.entries, key)
		c.misses++
		return nil, errors.New("key expired")
	}

	// Update access stats
	entry.AccessCount++
	entry.LastAccess = time.Now()
	c.hits++

	return entry.Value, nil
}

// Set stores a value in the cache
func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Check if we need to evict
	if len(c.entries) >= c.maxSize {
		if _, exists := c.entries[key]; !exists {
			c.evict()
		}
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		LastAccess:  now,
		AccessCount: 0,
	}

	c.entries[key] = entry

	return nil
}

// Delete removes a value from the cache
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)

	return nil
}

// Exists checks if a key exists
func (c *InMemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return false, nil
	}

	if entry.IsExpired() {
		return false, nil
	}

	return true, nil
}

// Clear clears all cache entries
func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)

	return nil
}

// Keys returns all keys
func (c *InMemoryCache) Keys(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}

	return keys, nil
}

// Stats returns cache statistics
func (c *InMemoryCache) Stats(ctx context.Context) (CacheStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Size:      len(c.entries),
	}

	total := c.hits + c.misses
	if total > 0 {
		stats.HitRate = float64(c.hits) / float64(total)
	}

	return stats, nil
}

// evict evicts an entry based on the eviction policy
func (c *InMemoryCache) evict() {
	if len(c.entries) == 0 {
		return
	}

	var victimKey string

	switch c.evictionPolicy {
	case EvictionPolicyLRU:
		victimKey = c.evictLRU()
	case EvictionPolicyLFU:
		victimKey = c.evictLFU()
	case EvictionPolicyFIFO:
		victimKey = c.evictFIFO()
	default:
		victimKey = c.evictLRU()
	}

	if victimKey != "" {
		delete(c.entries, victimKey)
		c.evictions++
	}
}

// evictLRU evicts the least recently used entry
func (c *InMemoryCache) evictLRU() string {
	var victimKey string
	var oldestAccess time.Time

	for key, entry := range c.entries {
		if victimKey == "" || entry.LastAccess.Before(oldestAccess) {
			victimKey = key
			oldestAccess = entry.LastAccess
		}
	}

	return victimKey
}

// evictLFU evicts the least frequently used entry
func (c *InMemoryCache) evictLFU() string {
	var victimKey string
	var lowestCount int64 = -1

	for key, entry := range c.entries {
		if lowestCount == -1 || entry.AccessCount < lowestCount {
			victimKey = key
			lowestCount = entry.AccessCount
		}
	}

	return victimKey
}

// evictFIFO evicts the oldest entry
func (c *InMemoryCache) evictFIFO() string {
	var victimKey string
	var oldestCreation time.Time

	for key, entry := range c.entries {
		if victimKey == "" || entry.CreatedAt.Before(oldestCreation) {
			victimKey = key
			oldestCreation = entry.CreatedAt
		}
	}

	return victimKey
}

// cleanupLoop periodically removes expired entries
func (c *InMemoryCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

// cleanup removes expired entries
func (c *InMemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

// MultiLevelCache implements a multi-level cache
type MultiLevelCache struct {
	caches []Cache
	mu     sync.RWMutex
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(caches ...Cache) *MultiLevelCache {
	return &MultiLevelCache{
		caches: caches,
	}
}

// Get retrieves a value from the cache hierarchy
func (mc *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, error) {
	for i, cache := range mc.caches {
		value, err := cache.Get(ctx, key)
		if err == nil {
			// Promote to higher levels
			for j := 0; j < i; j++ {
				mc.caches[j].Set(ctx, key, value, 0)
			}
			return value, nil
		}
	}

	return nil, errors.New("key not found in any cache level")
}

// Set stores a value in all cache levels
func (mc *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	for _, cache := range mc.caches {
		cache.Set(ctx, key, value, ttl)
	}

	return nil
}

// Delete removes a value from all cache levels
func (mc *MultiLevelCache) Delete(ctx context.Context, key string) error {
	for _, cache := range mc.caches {
		cache.Delete(ctx, key)
	}

	return nil
}

// Exists checks if a key exists in any cache level
func (mc *MultiLevelCache) Exists(ctx context.Context, key string) (bool, error) {
	for _, cache := range mc.caches {
		exists, err := cache.Exists(ctx, key)
		if err == nil && exists {
			return true, nil
		}
	}

	return false, nil
}

// Clear clears all cache levels
func (mc *MultiLevelCache) Clear(ctx context.Context) error {
	for _, cache := range mc.caches {
		cache.Clear(ctx)
	}

	return nil
}

// Keys returns keys from the first cache level
func (mc *MultiLevelCache) Keys(ctx context.Context) ([]string, error) {
	if len(mc.caches) == 0 {
		return nil, errors.New("no cache levels")
	}

	return mc.caches[0].Keys(ctx)
}

// Stats returns aggregated statistics from all cache levels
func (mc *MultiLevelCache) Stats(ctx context.Context) (CacheStats, error) {
	aggregated := CacheStats{}

	for _, cache := range mc.caches {
		stats, err := cache.Stats(ctx)
		if err != nil {
			continue
		}

		aggregated.Hits += stats.Hits
		aggregated.Misses += stats.Misses
		aggregated.Evictions += stats.Evictions
		aggregated.Size += stats.Size
	}

	total := aggregated.Hits + aggregated.Misses
	if total > 0 {
		aggregated.HitRate = float64(aggregated.Hits) / float64(total)
	}

	return aggregated, nil
}
