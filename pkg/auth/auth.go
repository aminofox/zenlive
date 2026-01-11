package auth

import (
	"context"
	"time"

	"github.com/aminofox/zenlive/pkg/types"
)

// Authenticator is the interface for authentication providers
type Authenticator interface {
	// Authenticate authenticates a user with credentials and returns an auth token
	Authenticate(ctx context.Context, credentials *types.Credentials) (*types.AuthToken, error)

	// ValidateToken validates an access token and returns the user claims
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)

	// RefreshToken refreshes an access token using a refresh token
	RefreshToken(ctx context.Context, refreshToken string) (*types.AuthToken, error)

	// RevokeToken revokes a token (logout)
	RevokeToken(ctx context.Context, token string) error
}

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	// UserID is the unique identifier of the user
	UserID string

	// Username is the username
	Username string

	// Email is the user's email
	Email string

	// Role is the user's role
	Role types.UserRole

	// IssuedAt is when the token was issued
	IssuedAt time.Time

	// ExpiresAt is when the token expires
	ExpiresAt time.Time

	// Custom claims
	Custom map[string]interface{}
}

// IsExpired checks if the token is expired
func (c *TokenClaims) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// TimeUntilExpiry returns the duration until the token expires
func (c *TokenClaims) TimeUntilExpiry() time.Duration {
	return time.Until(c.ExpiresAt)
}

// UserStore is the interface for user storage
type UserStore interface {
	// GetUserByUsername gets a user by username or email
	GetUserByUsername(ctx context.Context, username string) (*types.User, error)

	// GetUserByID gets a user by ID
	GetUserByID(ctx context.Context, userID string) (*types.User, error)

	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *types.User, password string) error

	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *types.User) error

	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, userID string) error

	// ValidatePassword validates a user's password
	ValidatePassword(ctx context.Context, userID string, password string) (bool, error)

	// UpdatePassword updates a user's password
	UpdatePassword(ctx context.Context, userID string, newPassword string) error
}

// TokenStore is the interface for token storage (for revocation)
type TokenStore interface {
	// StoreToken stores a token
	StoreToken(ctx context.Context, token string, userID string, expiresAt time.Time) error

	// IsTokenRevoked checks if a token is revoked
	IsTokenRevoked(ctx context.Context, token string) (bool, error)

	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token string) error

	// CleanExpiredTokens removes expired tokens
	CleanExpiredTokens(ctx context.Context) error
}
