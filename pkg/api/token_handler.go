// Package api provides JWT token generation for room access
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

// TokenHandler handles token generation for room access
type TokenHandler struct {
	roomManager *room.RoomManager
	jwtAuth     *auth.JWTAuthenticator
	jwtSecret   string
	logger      logger.Logger
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(roomManager *room.RoomManager, jwtAuth *auth.JWTAuthenticator, jwtSecret string, log logger.Logger) *TokenHandler {
	return &TokenHandler{
		roomManager: roomManager,
		jwtAuth:     jwtAuth,
		jwtSecret:   jwtSecret,
		logger:      log,
	}
}

// GenerateTokenRequest represents a request to generate an access token
type GenerateTokenRequest struct {
	RoomID      string                      `json:"room_id"`
	UserID      string                      `json:"user_id"`
	Username    string                      `json:"username"`
	Permissions room.ParticipantPermissions `json:"permissions,omitempty"`
	TTL         int                         `json:"ttl,omitempty"` // seconds, default 24h
	Metadata    map[string]interface{}      `json:"metadata,omitempty"`
}

// TokenResponse represents a token response
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
}

// GenerateAccessToken handles POST /api/rooms/:roomId/tokens
func (h *TokenHandler) GenerateAccessToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req GenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.RoomID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id is required")
		return
	}
	if req.UserID == "" {
		h.sendError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Username == "" {
		h.sendError(w, http.StatusBadRequest, "username is required")
		return
	}

	// Verify room exists
	_, err := h.roomManager.GetRoom(req.RoomID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "room not found")
		return
	}

	// Default TTL: 24 hours
	ttl := 24 * time.Hour
	if req.TTL > 0 {
		ttl = time.Duration(req.TTL) * time.Second
	}

	// Create custom claims for room access
	customClaims := map[string]interface{}{
		"room_id":     req.RoomID,
		"permissions": req.Permissions,
	}
	if req.Metadata != nil {
		customClaims["metadata"] = req.Metadata
	}

	// Generate JWT token
	expiresAt := time.Now().Add(ttl)
	claims := &auth.TokenClaims{
		UserID:    req.UserID,
		Username:  req.Username,
		IssuedAt:  time.Now(),
		ExpiresAt: expiresAt,
		Custom:    customClaims,
	}

	token, err := h.generateSimpleJWT(claims)
	if err != nil {
		h.logger.Error("Failed to generate token",
			logger.String("room_id", req.RoomID),
			logger.String("user_id", req.UserID),
			logger.Err(err),
		)
		h.sendError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	h.logger.Info("Access token generated",
		logger.String("room_id", req.RoomID),
		logger.String("user_id", req.UserID),
	)

	// Send response
	h.sendJSON(w, http.StatusOK, TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		RoomID:    req.RoomID,
		UserID:    req.UserID,
	})
}

// Helper methods

func (h *TokenHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TokenHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}

// generateSimpleJWT generates a simple JWT token for room access
// This is a simplified version since we can't access the private generateToken method
func (h *TokenHandler) generateSimpleJWT(claims *auth.TokenClaims) (string, error) {
	secret := []byte(h.jwtSecret)

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
	h256 := hmac.New(sha256.New, secret)
	h256.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h256.Sum(nil))

	return message + "." + signature, nil
}
