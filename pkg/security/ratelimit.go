package security

import (
	"errors"
	"sync"
	"time"
)

// RateLimitLevel represents different levels of rate limiting
type RateLimitLevel string

const (
	// RateLimitLevelGlobal applies to entire system
	RateLimitLevelGlobal RateLimitLevel = "global"
	// RateLimitLevelIP applies per IP address
	RateLimitLevelIP RateLimitLevel = "ip"
	// RateLimitLevelUser applies per user
	RateLimitLevelUser RateLimitLevel = "user"
	// RateLimitLevelEndpoint applies per API endpoint
	RateLimitLevelEndpoint RateLimitLevel = "endpoint"
	// RateLimitLevelStream applies per stream
	RateLimitLevelStream RateLimitLevel = "stream"
)

// RateLimitConfig defines rate limit configuration
type RateLimitConfig struct {
	// Requests is the number of requests allowed
	Requests int
	// Window is the time window for the rate limit
	Window time.Duration
	// Burst allows burst traffic up to this limit
	Burst int
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu              sync.RWMutex
	config          *RateLimitConfig
	buckets         map[string]*bucket
	onExceeded      func(key string, level RateLimitLevel)
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// MultiLevelRateLimiter manages rate limiting at multiple levels
type MultiLevelRateLimiter struct {
	mu       sync.RWMutex
	limiters map[RateLimitLevel]*RateLimiter
	enabled  map[RateLimitLevel]bool
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool
	Level      RateLimitLevel
	Limit      int
	Remaining  int
	RetryAfter time.Duration
	ResetTime  time.Time
}

var (
	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config.Burst == 0 {
		config.Burst = config.Requests
	}

	rl := &RateLimiter{
		config:          config,
		buckets:         make(map[string]*bucket),
		cleanupInterval: 10 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     rl.config.Burst,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed / rl.config.Window * time.Duration(rl.config.Requests))

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > rl.config.Burst {
			b.tokens = rl.config.Burst
		}
		b.lastRefill = now
	}

	// Check if we have tokens
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	if rl.onExceeded != nil {
		rl.onExceeded(key, "")
	}

	return false
}

// GetStatus returns the current status for a key
func (rl *RateLimiter) GetStatus(key string) *RateLimitResult {
	rl.mu.RLock()
	b, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		return &RateLimitResult{
			Allowed:   true,
			Limit:     rl.config.Requests,
			Remaining: rl.config.Burst,
			ResetTime: time.Now().Add(rl.config.Window),
		}
	}

	b.mu.Lock()
	tokens := b.tokens
	lastRefill := b.lastRefill
	b.mu.Unlock()

	resetTime := lastRefill.Add(rl.config.Window)
	retryAfter := time.Duration(0)
	if tokens == 0 {
		retryAfter = time.Until(resetTime)
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return &RateLimitResult{
		Allowed:    tokens > 0,
		Limit:      rl.config.Requests,
		Remaining:  tokens,
		RetryAfter: retryAfter,
		ResetTime:  resetTime,
	}
}

// Reset resets the rate limit for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, key)
}

// SetCallback sets the callback for when rate limit is exceeded
func (rl *RateLimiter) SetCallback(callback func(key string, level RateLimitLevel)) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.onExceeded = callback
}

// cleanup periodically removes expired buckets
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, b := range rl.buckets {
				b.mu.Lock()
				if now.Sub(b.lastRefill) > rl.config.Window*2 {
					delete(rl.buckets, key)
				}
				b.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopCleanup:
			return
		}
	}
}

// Stop stops the rate limiter and cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// NewMultiLevelRateLimiter creates a new multi-level rate limiter
func NewMultiLevelRateLimiter() *MultiLevelRateLimiter {
	return &MultiLevelRateLimiter{
		limiters: make(map[RateLimitLevel]*RateLimiter),
		enabled:  make(map[RateLimitLevel]bool),
	}
}

// AddLevel adds a rate limiting level with configuration
func (ml *MultiLevelRateLimiter) AddLevel(level RateLimitLevel, config *RateLimitConfig) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	limiter := NewRateLimiter(config)
	ml.limiters[level] = limiter
	ml.enabled[level] = true
}

// Check checks rate limits at all enabled levels
func (ml *MultiLevelRateLimiter) Check(keys map[RateLimitLevel]string) (*RateLimitResult, error) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	for level, limiter := range ml.limiters {
		if !ml.enabled[level] {
			continue
		}

		key, exists := keys[level]
		if !exists {
			continue
		}

		if !limiter.Allow(key) {
			status := limiter.GetStatus(key)
			status.Level = level
			return status, ErrRateLimitExceeded
		}
	}

	return &RateLimitResult{
		Allowed: true,
	}, nil
}

// GetStatus returns status for all levels
func (ml *MultiLevelRateLimiter) GetStatus(keys map[RateLimitLevel]string) map[RateLimitLevel]*RateLimitResult {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	results := make(map[RateLimitLevel]*RateLimitResult)

	for level, limiter := range ml.limiters {
		key, exists := keys[level]
		if !exists {
			continue
		}

		status := limiter.GetStatus(key)
		status.Level = level
		results[level] = status
	}

	return results
}

// EnableLevel enables a specific rate limiting level
func (ml *MultiLevelRateLimiter) EnableLevel(level RateLimitLevel) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.enabled[level] = true
}

// DisableLevel disables a specific rate limiting level
func (ml *MultiLevelRateLimiter) DisableLevel(level RateLimitLevel) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.enabled[level] = false
}

// Reset resets rate limits for specific keys
func (ml *MultiLevelRateLimiter) Reset(keys map[RateLimitLevel]string) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	for level, key := range keys {
		if limiter, exists := ml.limiters[level]; exists {
			limiter.Reset(key)
		}
	}
}

// Stop stops all rate limiters
func (ml *MultiLevelRateLimiter) Stop() {
	ml.mu.RLock()
	defer ml.mu.RUnlock()

	for _, limiter := range ml.limiters {
		limiter.Stop()
	}
}

// DefaultRateLimitConfigs returns recommended rate limit configurations
func DefaultRateLimitConfigs() map[RateLimitLevel]*RateLimitConfig {
	return map[RateLimitLevel]*RateLimitConfig{
		RateLimitLevelGlobal: {
			Requests: 10000,
			Window:   time.Minute,
			Burst:    15000,
		},
		RateLimitLevelIP: {
			Requests: 100,
			Window:   time.Minute,
			Burst:    150,
		},
		RateLimitLevelUser: {
			Requests: 200,
			Window:   time.Minute,
			Burst:    250,
		},
		RateLimitLevelEndpoint: {
			Requests: 50,
			Window:   time.Minute,
			Burst:    75,
		},
		RateLimitLevelStream: {
			Requests: 30,
			Window:   time.Minute,
			Burst:    40,
		},
	}
}
