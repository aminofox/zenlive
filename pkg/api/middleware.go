// Package api provides middleware for authentication and rate limiting
package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
)

// ContextKey is a custom type for context keys
type ContextKey string

const (
	// ContextKeyClaims is the key for storing claims in context
	ContextKeyClaims ContextKey = "claims"
)

// AuthMiddleware provides JWT authentication middleware
type AuthMiddleware struct {
	jwtAuth *auth.JWTAuthenticator
	logger  logger.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtAuth *auth.JWTAuthenticator, log logger.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtAuth: jwtAuth,
		logger:  log,
	}
}

// Authenticate validates JWT tokens from Authorization header
func (m *AuthMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.sendError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Expected format: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			m.sendError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		token := parts[1]

		// Validate token
		claims, err := m.jwtAuth.ValidateToken(r.Context(), token)
		if err != nil {
			m.logger.Warn("Token validation failed", logger.Err(err))
			m.sendError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), ContextKeyClaims, claims)
		next(w, r.WithContext(ctx))
	}
}

// GetClaims extracts claims from request context
func GetClaims(r *http.Request) (*auth.TokenClaims, bool) {
	claims, ok := r.Context().Value(ContextKeyClaims).(*auth.TokenClaims)
	return claims, ok
}

// RateLimiter provides rate limiting middleware
type RateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*clientLimiter
	logger  logger.Logger

	// Configuration
	requestsPerMinute int
	cleanupInterval   time.Duration
}

type clientLimiter struct {
	tokens     int
	lastUpdate time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int, log logger.Logger) *RateLimiter {
	rl := &RateLimiter{
		clients:           make(map[string]*clientLimiter),
		logger:            log,
		requestsPerMinute: requestsPerMinute,
		cleanupInterval:   5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Limit applies rate limiting based on client IP
func (rl *RateLimiter) Limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := getClientIP(r)

		// Check rate limit
		if !rl.allow(clientIP) {
			rl.logger.Warn("Rate limit exceeded",
				logger.String("ip", clientIP),
				logger.String("path", r.URL.Path),
			)
			rl.sendError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next(w, r)
	}
}

// allow checks if a request from the client is allowed
func (rl *RateLimiter) allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create client limiter
	limiter, exists := rl.clients[clientIP]
	if !exists {
		limiter = &clientLimiter{
			tokens:     rl.requestsPerMinute,
			lastUpdate: now,
		}
		rl.clients[clientIP] = limiter
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(limiter.lastUpdate)
	tokensToAdd := int(elapsed.Minutes() * float64(rl.requestsPerMinute))
	limiter.tokens += tokensToAdd
	if limiter.tokens > rl.requestsPerMinute {
		limiter.tokens = rl.requestsPerMinute
	}
	limiter.lastUpdate = now

	// Check if request is allowed
	if limiter.tokens <= 0 {
		return false
	}

	limiter.tokens--
	return true
}

// cleanup removes old client limiters
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, limiter := range rl.clients {
			if now.Sub(limiter.lastUpdate) > rl.cleanupInterval {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the list
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Helper methods

func (m *AuthMiddleware) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

func (rl *RateLimiter) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

// CORS middleware for cross-origin requests
type CORSMiddleware struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(origins, methods, headers []string) *CORSMiddleware {
	return &CORSMiddleware{
		allowedOrigins: origins,
		allowedMethods: methods,
		allowedHeaders: headers,
	}
}

// Handle applies CORS headers
func (cm *CORSMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		origin := r.Header.Get("Origin")
		if origin != "" && cm.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cm.allowedMethods, ", "))
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cm.allowedHeaders, ", "))
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func (cm *CORSMiddleware) isOriginAllowed(origin string) bool {
	for _, allowed := range cm.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
