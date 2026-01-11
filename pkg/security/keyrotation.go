package security

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// StreamKey represents a stream key with metadata
type StreamKey struct {
	ID            string
	StreamID      string
	Key           string
	UserID        string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	IsActive      bool
	RotationCount int
	LastRotation  time.Time
}

// KeyRotationPolicy defines the key rotation policy
type KeyRotationPolicy struct {
	// RotateEvery defines how often to rotate keys
	RotateEvery time.Duration
	// MaxKeyAge is the maximum age before forcing rotation
	MaxKeyAge time.Duration
	// AutoRotate enables automatic key rotation
	AutoRotate bool
	// KeepHistory is how many old keys to keep for grace period
	KeepHistory int
	// GracePeriod allows old keys to work for this duration after rotation
	GracePeriod time.Duration
}

// KeyRotationManager manages stream key rotation
type KeyRotationManager struct {
	mu           sync.RWMutex
	keys         map[string]*StreamKey   // streamID -> current key
	keyHistory   map[string][]*StreamKey // streamID -> historical keys
	policy       *KeyRotationPolicy
	onRotate     func(oldKey, newKey *StreamKey)
	onExpire     func(key *StreamKey)
	stopRotation chan struct{}
}

// RotationStats provides statistics about key rotations
type RotationStats struct {
	TotalKeys        int
	ActiveKeys       int
	ExpiredKeys      int
	AvgRotationCount float64
	NextRotation     time.Time
}

var (
	// ErrKeyNotFound is returned when a key is not found
	ErrKeyNotFound = errors.New("stream key not found")
	// ErrKeyExpired is returned when a key has expired
	ErrKeyExpired = errors.New("stream key has expired")
	// ErrKeyInactive is returned when a key is inactive
	ErrKeyInactive = errors.New("stream key is inactive")
)

// DefaultRotationPolicy returns a secure default rotation policy
func DefaultRotationPolicy() *KeyRotationPolicy {
	return &KeyRotationPolicy{
		RotateEvery: 7 * 24 * time.Hour,  // Weekly rotation
		MaxKeyAge:   30 * 24 * time.Hour, // 30 days max
		AutoRotate:  true,
		KeepHistory: 3,             // Keep last 3 keys
		GracePeriod: 1 * time.Hour, // 1 hour grace period
	}
}

// NewKeyRotationManager creates a new key rotation manager
func NewKeyRotationManager(policy *KeyRotationPolicy) *KeyRotationManager {
	if policy == nil {
		policy = DefaultRotationPolicy()
	}

	krm := &KeyRotationManager{
		keys:         make(map[string]*StreamKey),
		keyHistory:   make(map[string][]*StreamKey),
		policy:       policy,
		stopRotation: make(chan struct{}),
	}

	if policy.AutoRotate {
		go krm.autoRotationLoop()
	}

	return krm
}

// GenerateKey generates a new stream key for a stream
func (krm *KeyRotationManager) GenerateKey(streamID, userID string) (*StreamKey, error) {
	// Generate cryptographically secure random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	keyString := base64.URLEncoding.EncodeToString(keyBytes)

	key := &StreamKey{
		ID:            generateKeyID(),
		StreamID:      streamID,
		Key:           keyString,
		UserID:        userID,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(krm.policy.MaxKeyAge),
		IsActive:      true,
		RotationCount: 0,
		LastRotation:  time.Now(),
	}

	krm.mu.Lock()
	defer krm.mu.Unlock()

	// Store old key in history if exists
	if oldKey, exists := krm.keys[streamID]; exists {
		krm.addToHistory(streamID, oldKey)
	}

	krm.keys[streamID] = key

	return key, nil
}

// RotateKey rotates the key for a stream
func (krm *KeyRotationManager) RotateKey(streamID string) (*StreamKey, error) {
	krm.mu.Lock()
	oldKey, exists := krm.keys[streamID]
	krm.mu.Unlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	// Generate new key
	newKey, err := krm.GenerateKey(streamID, oldKey.UserID)
	if err != nil {
		return nil, err
	}

	newKey.RotationCount = oldKey.RotationCount + 1

	// Deactivate old key but keep it in grace period
	krm.mu.Lock()
	oldKey.IsActive = false
	oldKey.ExpiresAt = time.Now().Add(krm.policy.GracePeriod)
	krm.mu.Unlock()

	if krm.onRotate != nil {
		krm.onRotate(oldKey, newKey)
	}

	return newKey, nil
}

// ValidateKey validates a stream key
func (krm *KeyRotationManager) ValidateKey(streamID, keyString string) error {
	krm.mu.RLock()
	defer krm.mu.RUnlock()

	// Check current key
	currentKey, exists := krm.keys[streamID]
	if exists {
		if currentKey.Key == keyString {
			return krm.checkKeyValidity(currentKey)
		}
	}

	// Check historical keys (grace period)
	history, exists := krm.keyHistory[streamID]
	if exists {
		for _, key := range history {
			if key.Key == keyString {
				return krm.checkKeyValidity(key)
			}
		}
	}

	return ErrKeyNotFound
}

// GetKey returns the current key for a stream
func (krm *KeyRotationManager) GetKey(streamID string) (*StreamKey, error) {
	krm.mu.RLock()
	defer krm.mu.RUnlock()

	key, exists := krm.keys[streamID]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return key, nil
}

// RevokeKey revokes a stream key immediately
func (krm *KeyRotationManager) RevokeKey(streamID string) error {
	krm.mu.Lock()
	defer krm.mu.Unlock()

	key, exists := krm.keys[streamID]
	if !exists {
		return ErrKeyNotFound
	}

	key.IsActive = false
	key.ExpiresAt = time.Now()

	if krm.onExpire != nil {
		krm.onExpire(key)
	}

	return nil
}

// GetRotationStats returns statistics about key rotations
func (krm *KeyRotationManager) GetRotationStats() *RotationStats {
	krm.mu.RLock()
	defer krm.mu.RUnlock()

	stats := &RotationStats{
		TotalKeys:   len(krm.keys),
		ActiveKeys:  0,
		ExpiredKeys: 0,
	}

	totalRotations := 0
	var nextRotation time.Time

	for _, key := range krm.keys {
		if key.IsActive && time.Now().Before(key.ExpiresAt) {
			stats.ActiveKeys++

			// Calculate next rotation time
			rotationTime := key.LastRotation.Add(krm.policy.RotateEvery)
			if nextRotation.IsZero() || rotationTime.Before(nextRotation) {
				nextRotation = rotationTime
			}
		} else {
			stats.ExpiredKeys++
		}

		totalRotations += key.RotationCount
	}

	if stats.TotalKeys > 0 {
		stats.AvgRotationCount = float64(totalRotations) / float64(stats.TotalKeys)
	}

	stats.NextRotation = nextRotation

	return stats
}

// SetRotationCallback sets the callback for key rotation events
func (krm *KeyRotationManager) SetRotationCallback(callback func(oldKey, newKey *StreamKey)) {
	krm.mu.Lock()
	defer krm.mu.Unlock()
	krm.onRotate = callback
}

// SetExpirationCallback sets the callback for key expiration events
func (krm *KeyRotationManager) SetExpirationCallback(callback func(key *StreamKey)) {
	krm.mu.Lock()
	defer krm.mu.Unlock()
	krm.onExpire = callback
}

// checkKeyValidity checks if a key is valid
func (krm *KeyRotationManager) checkKeyValidity(key *StreamKey) error {
	if !key.IsActive {
		return ErrKeyInactive
	}

	if time.Now().After(key.ExpiresAt) {
		return ErrKeyExpired
	}

	return nil
}

// addToHistory adds a key to the history
func (krm *KeyRotationManager) addToHistory(streamID string, key *StreamKey) {
	history, exists := krm.keyHistory[streamID]
	if !exists {
		history = make([]*StreamKey, 0, krm.policy.KeepHistory)
	}

	// Add to front
	history = append([]*StreamKey{key}, history...)

	// Trim to keep only KeepHistory items
	if len(history) > krm.policy.KeepHistory {
		history = history[:krm.policy.KeepHistory]
	}

	krm.keyHistory[streamID] = history
}

// autoRotationLoop automatically rotates keys based on policy
func (krm *KeyRotationManager) autoRotationLoop() {
	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			krm.checkAndRotateKeys()
		case <-krm.stopRotation:
			return
		}
	}
}

// checkAndRotateKeys checks all keys and rotates if needed
func (krm *KeyRotationManager) checkAndRotateKeys() {
	krm.mu.RLock()
	keysToRotate := make([]string, 0)

	for streamID, key := range krm.keys {
		if !key.IsActive {
			continue
		}

		// Check if rotation is needed
		if time.Since(key.LastRotation) >= krm.policy.RotateEvery {
			keysToRotate = append(keysToRotate, streamID)
		}
	}
	krm.mu.RUnlock()

	// Rotate keys
	for _, streamID := range keysToRotate {
		if _, err := krm.RotateKey(streamID); err != nil {
			// Log error in production
			continue
		}
	}

	// Cleanup expired keys from history
	krm.cleanupExpiredKeys()
}

// cleanupExpiredKeys removes expired keys from history
func (krm *KeyRotationManager) cleanupExpiredKeys() {
	krm.mu.Lock()
	defer krm.mu.Unlock()

	now := time.Now()

	for streamID, history := range krm.keyHistory {
		filtered := make([]*StreamKey, 0, len(history))

		for _, key := range history {
			// Keep key if still in grace period
			if now.Before(key.ExpiresAt) {
				filtered = append(filtered, key)
			}
		}

		if len(filtered) == 0 {
			delete(krm.keyHistory, streamID)
		} else {
			krm.keyHistory[streamID] = filtered
		}
	}
}

// Stop stops the auto-rotation loop
func (krm *KeyRotationManager) Stop() {
	close(krm.stopRotation)
}

// generateKeyID generates a unique key ID
func generateKeyID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("key_%s", base64.URLEncoding.EncodeToString(b)[:16])
}
