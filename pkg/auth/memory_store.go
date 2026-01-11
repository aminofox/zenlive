package auth

import (
	"context"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/types"
	"golang.org/x/crypto/bcrypt"
)

// InMemoryUserStore is an in-memory implementation of UserStore for testing
type InMemoryUserStore struct {
	users     map[string]*types.User // userID -> User
	usernames map[string]string      // username -> userID
	passwords map[string]string      // userID -> hashed password
	mu        sync.RWMutex
}

// NewInMemoryUserStore creates a new in-memory user store
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{
		users:     make(map[string]*types.User),
		usernames: make(map[string]string),
		passwords: make(map[string]string),
	}
}

// GetUserByUsername gets a user by username or email
func (s *InMemoryUserStore) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.usernames[username]
	if !exists {
		return nil, errors.NewNotFoundError("user not found")
	}

	user, exists := s.users[userID]
	if !exists {
		return nil, errors.NewNotFoundError("user not found")
	}

	// Return a copy to prevent external modifications
	userCopy := *user
	return &userCopy, nil
}

// GetUserByID gets a user by ID
func (s *InMemoryUserStore) GetUserByID(ctx context.Context, userID string) (*types.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, errors.NewNotFoundError("user not found")
	}

	// Return a copy to prevent external modifications
	userCopy := *user
	return &userCopy, nil
}

// CreateUser creates a new user
func (s *InMemoryUserStore) CreateUser(ctx context.Context, user *types.User, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if username already exists
	if _, exists := s.usernames[user.Username]; exists {
		return errors.NewValidationError("username already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(errors.ErrCodeStorageError, "failed to hash password", err)
	}

	// Store user
	userCopy := *user
	s.users[user.ID] = &userCopy
	s.usernames[user.Username] = user.ID
	s.passwords[user.ID] = string(hashedPassword)

	return nil
}

// UpdateUser updates a user
func (s *InMemoryUserStore) UpdateUser(ctx context.Context, user *types.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user exists
	existingUser, exists := s.users[user.ID]
	if !exists {
		return errors.NewNotFoundError("user not found")
	}

	// If username changed, update username index
	if existingUser.Username != user.Username {
		// Check if new username is taken
		if _, exists := s.usernames[user.Username]; exists {
			return errors.NewValidationError("username already exists")
		}
		delete(s.usernames, existingUser.Username)
		s.usernames[user.Username] = user.ID
	}

	// Update user
	userCopy := *user
	s.users[user.ID] = &userCopy

	return nil
}

// DeleteUser deletes a user
func (s *InMemoryUserStore) DeleteUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[userID]
	if !exists {
		return errors.NewNotFoundError("user not found")
	}

	delete(s.users, userID)
	delete(s.usernames, user.Username)
	delete(s.passwords, userID)

	return nil
}

// ValidatePassword validates a user's password
func (s *InMemoryUserStore) ValidatePassword(ctx context.Context, userID string, password string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashedPassword, exists := s.passwords[userID]
	if !exists {
		return false, errors.NewNotFoundError("user not found")
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil, nil
}

// UpdatePassword updates a user's password
func (s *InMemoryUserStore) UpdatePassword(ctx context.Context, userID string, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[userID]; !exists {
		return errors.NewNotFoundError("user not found")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(errors.ErrCodeStorageError, "failed to hash password", err)
	}

	s.passwords[userID] = string(hashedPassword)
	return nil
}

// tokenEntry represents a token entry in the store
type tokenEntry struct {
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
}

// InMemoryTokenStore is an in-memory implementation of TokenStore for testing
type InMemoryTokenStore struct {
	tokens map[string]*tokenEntry
	mu     sync.RWMutex
}

// NewInMemoryTokenStore creates a new in-memory token store
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{
		tokens: make(map[string]*tokenEntry),
	}
}

// StoreToken stores a token
func (s *InMemoryTokenStore) StoreToken(ctx context.Context, token string, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[token] = &tokenEntry{
		UserID:    userID,
		ExpiresAt: expiresAt,
		Revoked:   false,
	}

	return nil
}

// IsTokenRevoked checks if a token is revoked
func (s *InMemoryTokenStore) IsTokenRevoked(ctx context.Context, token string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.tokens[token]
	if !exists {
		// Token not found means it was never issued or was cleaned up
		return false, nil
	}

	return entry.Revoked, nil
}

// RevokeToken revokes a token
func (s *InMemoryTokenStore) RevokeToken(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.tokens[token]
	if !exists {
		return errors.NewNotFoundError("token not found")
	}

	entry.Revoked = true
	return nil
}

// CleanExpiredTokens removes expired tokens
func (s *InMemoryTokenStore) CleanExpiredTokens(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for token, entry := range s.tokens {
		if now.After(entry.ExpiresAt) {
			delete(s.tokens, token)
		}
	}

	return nil
}
