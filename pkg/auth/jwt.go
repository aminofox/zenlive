package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/types"
)

// JWTAuthenticator implements the Authenticator interface using JWT tokens
type JWTAuthenticator struct {
	secret        []byte
	userStore     UserStore
	tokenStore    TokenStore
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(secret string, userStore UserStore, tokenStore TokenStore) *JWTAuthenticator {
	return &JWTAuthenticator{
		secret:        []byte(secret),
		userStore:     userStore,
		tokenStore:    tokenStore,
		accessExpiry:  15 * time.Minute,   // Default 15 minutes
		refreshExpiry: 7 * 24 * time.Hour, // Default 7 days
	}
}

// SetAccessExpiry sets the access token expiry duration
func (j *JWTAuthenticator) SetAccessExpiry(duration time.Duration) {
	j.accessExpiry = duration
}

// SetRefreshExpiry sets the refresh token expiry duration
func (j *JWTAuthenticator) SetRefreshExpiry(duration time.Duration) {
	j.refreshExpiry = duration
}

// Authenticate authenticates a user with credentials and returns an auth token
func (j *JWTAuthenticator) Authenticate(ctx context.Context, credentials *types.Credentials) (*types.AuthToken, error) {
	// Get user by username
	user, err := j.userStore.GetUserByUsername(ctx, credentials.Username)
	if err != nil {
		return nil, errors.NewAuthenticationError("invalid credentials")
	}

	// Validate password
	valid, err := j.userStore.ValidatePassword(ctx, user.ID, credentials.Password)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "failed to validate password", err)
	}
	if !valid {
		return nil, errors.NewAuthenticationError("invalid credentials")
	}

	// Generate access and refresh tokens
	now := time.Now()

	accessClaims := &TokenClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		IssuedAt:  now,
		ExpiresAt: now.Add(j.accessExpiry),
	}

	refreshClaims := &TokenClaims{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		IssuedAt:  now,
		ExpiresAt: now.Add(j.refreshExpiry),
	}

	accessToken, err := j.generateToken(accessClaims)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "failed to generate access token", err)
	}

	refreshToken, err := j.generateToken(refreshClaims)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "failed to generate refresh token", err)
	}

	// Store tokens for revocation tracking
	if err := j.tokenStore.StoreToken(ctx, accessToken, user.ID, accessClaims.ExpiresAt); err != nil {
		return nil, errors.Wrap(errors.ErrCodeStorageError, "failed to store access token", err)
	}
	if err := j.tokenStore.StoreToken(ctx, refreshToken, user.ID, refreshClaims.ExpiresAt); err != nil {
		return nil, errors.Wrap(errors.ErrCodeStorageError, "failed to store refresh token", err)
	}

	return &types.AuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(j.accessExpiry.Seconds()),
	}, nil
}

// ValidateToken validates an access token and returns the user claims
func (j *JWTAuthenticator) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	// Check if token is revoked
	revoked, err := j.tokenStore.IsTokenRevoked(ctx, token)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeStorageError, "failed to check token revocation", err)
	}
	if revoked {
		return nil, errors.NewAuthenticationError("token is revoked")
	}

	// Parse and validate token
	claims, err := j.parseToken(token)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "failed to parse token", err)
	}

	// Check if expired
	if claims.IsExpired() {
		return nil, errors.NewAuthenticationError("token is expired")
	}

	return claims, nil
}

// RefreshToken refreshes an access token using a refresh token
func (j *JWTAuthenticator) RefreshToken(ctx context.Context, refreshToken string) (*types.AuthToken, error) {
	// Validate refresh token
	claims, err := j.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "invalid refresh token", err)
	}

	// Generate new access token
	now := time.Now()
	newAccessClaims := &TokenClaims{
		UserID:    claims.UserID,
		Username:  claims.Username,
		Email:     claims.Email,
		Role:      claims.Role,
		IssuedAt:  now,
		ExpiresAt: now.Add(j.accessExpiry),
	}

	accessToken, err := j.generateToken(newAccessClaims)
	if err != nil {
		return nil, errors.Wrap(errors.ErrCodeAuthenticationFailed, "failed to generate new access token", err)
	}

	// Store new access token
	if err := j.tokenStore.StoreToken(ctx, accessToken, claims.UserID, newAccessClaims.ExpiresAt); err != nil {
		return nil, errors.Wrap(errors.ErrCodeStorageError, "failed to store new access token", err)
	}

	return &types.AuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(j.accessExpiry.Seconds()),
	}, nil
}

// RevokeToken revokes a token (logout)
func (j *JWTAuthenticator) RevokeToken(ctx context.Context, token string) error {
	return j.tokenStore.RevokeToken(ctx, token)
}

// generateToken generates a JWT token from claims
func (j *JWTAuthenticator) generateToken(claims *TokenClaims) (string, error) {
	// Create header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Create payload
	payload := map[string]interface{}{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"email":    claims.Email,
		"role":     claims.Role,
		"iat":      claims.IssuedAt.Unix(),
		"exp":      claims.ExpiresAt.Unix(),
	}
	if claims.Custom != nil {
		for k, v := range claims.Custom {
			payload[k] = v
		}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	message := headerEncoded + "." + payloadEncoded
	signature := j.sign(message)

	return message + "." + signature, nil
}

// parseToken parses and validates a JWT token
func (j *JWTAuthenticator) parseToken(token string) (*TokenClaims, error) {
	// Split token into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	message := parts[0] + "." + parts[1]
	expectedSignature := j.sign(message)
	if parts[2] != expectedSignature {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Extract claims
	claims := &TokenClaims{
		Custom: make(map[string]interface{}),
	}

	if userID, ok := payload["user_id"].(string); ok {
		claims.UserID = userID
	}
	if username, ok := payload["username"].(string); ok {
		claims.Username = username
	}
	if email, ok := payload["email"].(string); ok {
		claims.Email = email
	}
	if role, ok := payload["role"].(string); ok {
		claims.Role = types.UserRole(role)
	}
	if iat, ok := payload["iat"].(float64); ok {
		claims.IssuedAt = time.Unix(int64(iat), 0)
	}
	if exp, ok := payload["exp"].(float64); ok {
		claims.ExpiresAt = time.Unix(int64(exp), 0)
	}

	// Extract custom claims
	for k, v := range payload {
		switch k {
		case "user_id", "username", "email", "role", "iat", "exp":
			// Skip standard claims
		default:
			claims.Custom[k] = v
		}
	}

	return claims, nil
}

// sign creates a HMAC-SHA256 signature
func (j *JWTAuthenticator) sign(message string) string {
	h := hmac.New(sha256.New, j.secret)
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// generateJWT generates a JWT token from AccessTokenClaims using a secret key
func generateJWT(claims *AccessTokenClaims, secret string) (string, error) {
	// Create header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Create payload
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	message := headerEncoded + "." + payloadEncoded
	signature := signWithSecret(message, secret)

	return message + "." + signature, nil
}

// parseJWT parses and validates a JWT token
func parseJWT(token, secret string) (*AccessTokenClaims, error) {
	// Split token into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	message := parts[0] + "." + parts[1]
	expectedSignature := signWithSecret(message, secret)
	if parts[2] != expectedSignature {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims AccessTokenClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return &claims, nil
}

// signWithSecret creates a HMAC-SHA256 signature with a given secret
func signWithSecret(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
