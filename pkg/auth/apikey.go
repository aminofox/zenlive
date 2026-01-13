package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
)

// APIKey represents an API key pair used for authentication
type APIKey struct {
	// AccessKey is the public identifier (like API Key ID)
	AccessKey string `json:"access_key"`

	// SecretKey is the private key used for signing (never expose to clients)
	SecretKey string `json:"secret_key,omitempty"`

	// Name is a friendly name for this API key
	Name string `json:"name"`

	// CreatedAt is when the key was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the key expires (optional)
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// IsActive indicates if the key is active
	IsActive bool `json:"is_active"`

	// Metadata for additional information
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IsExpired checks if the API key is expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// APIKeyStore is the interface for storing API keys
type APIKeyStore interface {
	// StoreAPIKey stores an API key
	StoreAPIKey(ctx context.Context, key *APIKey) error

	// GetAPIKey retrieves an API key by access key
	GetAPIKey(ctx context.Context, accessKey string) (*APIKey, error)

	// ListAPIKeys lists all API keys
	ListAPIKeys(ctx context.Context) ([]*APIKey, error)

	// UpdateAPIKey updates an API key
	UpdateAPIKey(ctx context.Context, key *APIKey) error

	// DeleteAPIKey deletes an API key
	DeleteAPIKey(ctx context.Context, accessKey string) error
}

// APIKeyManager manages API key generation and validation
type APIKeyManager struct {
	store APIKeyStore
	mu    sync.RWMutex
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager(store APIKeyStore) *APIKeyManager {
	return &APIKeyManager{
		store: store,
	}
}

// GenerateAPIKey generates a new API key pair
// The access key is like: API_xxxxxxxxxxxxxxxx
// The secret key is like: SEC_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
func (m *APIKeyManager) GenerateAPIKey(ctx context.Context, name string, expiresIn *time.Duration, metadata map[string]string) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate access key (public identifier)
	accessKey, err := generateRandomKey("API", 16)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeUnknown, "failed to generate access key", err)
	}

	// Generate secret key (private key for signing)
	secretKey, err := generateRandomKey("SEC", 32)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeUnknown, "failed to generate secret key", err)
	}

	now := time.Now()
	apiKey := &APIKey{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Name:      name,
		CreatedAt: now,
		IsActive:  true,
		Metadata:  metadata,
	}

	if expiresIn != nil {
		expiresAt := now.Add(*expiresIn)
		apiKey.ExpiresAt = &expiresAt
	}

	// Store the API key
	if err := m.store.StoreAPIKey(ctx, apiKey); err != nil {
		return nil, errors.Wrap(errors.ErrCodeStorageError, "failed to store API key", err)
	}

	return apiKey, nil
}

// ValidateAPIKey validates an API key pair
func (m *APIKeyManager) ValidateAPIKey(ctx context.Context, accessKey, secretKey string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get stored API key
	storedKey, err := m.store.GetAPIKey(ctx, accessKey)
	if err != nil {
		return nil, errors.NewAuthenticationError("invalid API key")
	}

	// Check if active
	if !storedKey.IsActive {
		return nil, errors.NewAuthenticationError("API key is inactive")
	}

	// Check if expired
	if storedKey.IsExpired() {
		return nil, errors.NewAuthenticationError("API key is expired")
	}

	// Validate secret key
	if storedKey.SecretKey != secretKey {
		return nil, errors.NewAuthenticationError("invalid secret key")
	}

	return storedKey, nil
}

// GetAPIKey retrieves an API key (without exposing secret)
func (m *APIKeyManager) GetAPIKey(ctx context.Context, accessKey string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, err := m.store.GetAPIKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	// Don't expose secret key
	keyCopy := *key
	keyCopy.SecretKey = ""
	return &keyCopy, nil
}

// ListAPIKeys lists all API keys (without exposing secrets)
func (m *APIKeyManager) ListAPIKeys(ctx context.Context) ([]*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys, err := m.store.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}

	// Remove secret keys from response
	result := make([]*APIKey, len(keys))
	for i, key := range keys {
		keyCopy := *key
		keyCopy.SecretKey = ""
		result[i] = &keyCopy
	}

	return result, nil
}

// RevokeAPIKey revokes an API key
func (m *APIKeyManager) RevokeAPIKey(ctx context.Context, accessKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, err := m.store.GetAPIKey(ctx, accessKey)
	if err != nil {
		return err
	}

	key.IsActive = false
	return m.store.UpdateAPIKey(ctx, key)
}

// DeleteAPIKey deletes an API key permanently
func (m *APIKeyManager) DeleteAPIKey(ctx context.Context, accessKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.store.DeleteAPIKey(ctx, accessKey)
}

// generateRandomKey generates a random key with a prefix
func generateRandomKey(prefix string, length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// HashSecretKey creates a hash of the secret key for secure storage
// Use this if you want to store hashed secrets instead of plain text
func HashSecretKey(secretKey string) string {
	hash := sha256.Sum256([]byte(secretKey))
	return hex.EncodeToString(hash[:])
}

// MemoryAPIKeyStore is an in-memory implementation of APIKeyStore
type MemoryAPIKeyStore struct {
	keys map[string]*APIKey
	mu   sync.RWMutex
}

// NewMemoryAPIKeyStore creates a new in-memory API key store
func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	return &MemoryAPIKeyStore{
		keys: make(map[string]*APIKey),
	}
}

// StoreAPIKey stores an API key
func (s *MemoryAPIKeyStore) StoreAPIKey(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys[key.AccessKey] = key
	return nil
}

// GetAPIKey retrieves an API key by access key
func (s *MemoryAPIKeyStore) GetAPIKey(ctx context.Context, accessKey string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[accessKey]
	if !ok {
		return nil, fmt.Errorf("API key not found")
	}
	return key, nil
}

// ListAPIKeys lists all API keys
func (s *MemoryAPIKeyStore) ListAPIKeys(ctx context.Context) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, key)
	}
	return keys, nil
}

// UpdateAPIKey updates an API key
func (s *MemoryAPIKeyStore) UpdateAPIKey(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[key.AccessKey]; !ok {
		return fmt.Errorf("API key not found")
	}
	s.keys[key.AccessKey] = key
	return nil
}

// DeleteAPIKey deletes an API key
func (s *MemoryAPIKeyStore) DeleteAPIKey(ctx context.Context, accessKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.keys, accessKey)
	return nil
}

// ParseAPIKeyFromAuth parses an API key from an Authorization header
// Format: "Bearer API_xxx:SEC_xxx"
func ParseAPIKeyFromAuth(authHeader string) (accessKey, secretKey string, err error) {
	// Remove "Bearer " prefix
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	// Split by colon
	parts := strings.Split(authHeader, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid API key format, expected ACCESS_KEY:SECRET_KEY")
	}

	return parts[0], parts[1], nil
}
