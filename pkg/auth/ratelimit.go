package auth

import (
	"context"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
)

// RateLimiter is the interface for rate limiting
type RateLimiter interface {
	// Allow checks if an action is allowed for a key
	Allow(ctx context.Context, key string) (bool, error)

	// Reset resets the rate limit for a key
	Reset(ctx context.Context, key string) error
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	tokens       int
	lastRefill   time.Time
	capacity     int
	refillRate   int // tokens per second
	refillPeriod time.Duration
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	buckets      map[string]*bucket
	mu           sync.RWMutex
	capacity     int
	refillRate   int
	refillPeriod time.Duration
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
// capacity: maximum number of tokens in the bucket
// refillRate: number of tokens to add per refill period
// refillPeriod: how often to refill tokens (e.g., 1 second)
func NewTokenBucketLimiter(capacity int, refillRate int, refillPeriod time.Duration) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		buckets:      make(map[string]*bucket),
		capacity:     capacity,
		refillRate:   refillRate,
		refillPeriod: refillPeriod,
	}
}

// Allow checks if an action is allowed for a key
func (rl *TokenBucketLimiter) Allow(ctx context.Context, key string) (bool, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Get or create bucket
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:       rl.capacity,
			lastRefill:   time.Now(),
			capacity:     rl.capacity,
			refillRate:   rl.refillRate,
			refillPeriod: rl.refillPeriod,
		}
		rl.buckets[key] = b
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	if elapsed >= b.refillPeriod {
		periods := int(elapsed / b.refillPeriod)
		tokensToAdd := periods * b.refillRate
		b.tokens = min(b.capacity, b.tokens+tokensToAdd)
		b.lastRefill = now
	}

	// Check if tokens are available
	if b.tokens > 0 {
		b.tokens--
		return true, nil
	}

	return false, nil
}

// Reset resets the rate limit for a key
func (rl *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.buckets, key)
	return nil
}

// CleanupOldBuckets removes buckets that haven't been used recently
func (rl *TokenBucketLimiter) CleanupOldBuckets(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, b := range rl.buckets {
		if now.Sub(b.lastRefill) > maxAge {
			delete(rl.buckets, key)
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AuthRateLimiter wraps a rate limiter with authentication-specific logic
type AuthRateLimiter struct {
	loginLimiter   RateLimiter
	tokenLimiter   RateLimiter
	generalLimiter RateLimiter
}

// NewAuthRateLimiter creates a new authentication rate limiter with default settings
func NewAuthRateLimiter() *AuthRateLimiter {
	return &AuthRateLimiter{
		// Login: 5 attempts per minute per IP/username
		loginLimiter: NewTokenBucketLimiter(5, 5, time.Minute),

		// Token refresh: 10 requests per minute per user
		tokenLimiter: NewTokenBucketLimiter(10, 10, time.Minute),

		// General auth operations: 100 requests per minute per user
		generalLimiter: NewTokenBucketLimiter(100, 100, time.Minute),
	}
}

// AllowLogin checks if a login attempt is allowed
func (arl *AuthRateLimiter) AllowLogin(ctx context.Context, key string) error {
	allowed, err := arl.loginLimiter.Allow(ctx, key)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.ErrCodeRateLimitExceeded, "too many login attempts")
	}
	return nil
}

// AllowTokenRefresh checks if a token refresh is allowed
func (arl *AuthRateLimiter) AllowTokenRefresh(ctx context.Context, userID string) error {
	allowed, err := arl.tokenLimiter.Allow(ctx, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.ErrCodeRateLimitExceeded, "too many token refresh requests")
	}
	return nil
}

// AllowGeneralAuth checks if a general auth operation is allowed
func (arl *AuthRateLimiter) AllowGeneralAuth(ctx context.Context, userID string) error {
	allowed, err := arl.generalLimiter.Allow(ctx, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.ErrCodeRateLimitExceeded, "too many authentication requests")
	}
	return nil
}

// ResetLogin resets the login rate limit for a key
func (arl *AuthRateLimiter) ResetLogin(ctx context.Context, key string) error {
	return arl.loginLimiter.Reset(ctx, key)
}

// ResetTokenRefresh resets the token refresh rate limit for a user
func (arl *AuthRateLimiter) ResetTokenRefresh(ctx context.Context, userID string) error {
	return arl.tokenLimiter.Reset(ctx, userID)
}

// ResetGeneralAuth resets the general auth rate limit for a user
func (arl *AuthRateLimiter) ResetGeneralAuth(ctx context.Context, userID string) error {
	return arl.generalLimiter.Reset(ctx, userID)
}
