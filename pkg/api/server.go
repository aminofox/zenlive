// Package api provides REST API server for room management
package api

import (
	"fmt"
	"net/http"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

// Server represents the REST API server
type Server struct {
	roomHandler     *RoomHandler
	tokenHandler    *TokenHandler
	signalingServer *SignalingServer
	authMW          *AuthMiddleware
	rateLimiter     *RateLimiter
	corsMW          *CORSMiddleware
	logger          logger.Logger
	addr            string
}

// Config contains server configuration
type Config struct {
	Addr         string
	JWTSecret    string // JWT secret for token generation
	RateLimitRPM int    // Requests per minute
	CORSOrigins  []string
	CORSMethods  []string
	CORSHeaders  []string
}

// DefaultConfig returns default server configuration
func DefaultConfig() *Config {
	return &Config{
		Addr:         ":8080",
		JWTSecret:    "change-this-secret-in-production",
		RateLimitRPM: 60,
		CORSOrigins:  []string{"*"},
		CORSMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSHeaders:  []string{"Content-Type", "Authorization"},
	}
}

// NewServer creates a new API server
func NewServer(
	roomManager *room.RoomManager,
	jwtAuth *auth.JWTAuthenticator,
	config *Config,
	log logger.Logger,
) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	// Create handlers
	roomHandler := NewRoomHandler(roomManager, log)
	tokenHandler := NewTokenHandler(roomManager, jwtAuth, config.JWTSecret, log)
	signalingServer := NewSignalingServer(roomManager, log)

	// Create middleware
	authMW := NewAuthMiddleware(jwtAuth, log)
	rateLimiter := NewRateLimiter(config.RateLimitRPM, log)
	corsMW := NewCORSMiddleware(config.CORSOrigins, config.CORSMethods, config.CORSHeaders)

	return &Server{
		roomHandler:     roomHandler,
		tokenHandler:    tokenHandler,
		signalingServer: signalingServer,
		authMW:          authMW,
		rateLimiter:     rateLimiter,
		corsMW:          corsMW,
		logger:          log,
		addr:            config.Addr,
	}
}

// Start starts the API server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	s.registerRoutes(mux)

	s.logger.Info("Starting API server", logger.String("addr", s.addr))
	return http.ListenAndServe(s.addr, mux)
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Public routes (with rate limiting and CORS only)
	mux.HandleFunc("/api/health", s.chain(s.healthCheck, s.corsMW.Handle, s.rateLimiter.Limit))

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.chain(s.signalingServer.HandleWebSocket, s.corsMW.Handle))

	// Token generation (protected by auth)
	mux.HandleFunc("/api/rooms/", s.routeRoomRequests)

	// Admin routes (protected by auth and rate limiting)
	// In production, you should add role-based access control here
}

// routeRoomRequests routes room-related requests
func (s *Server) routeRoomRequests(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Apply CORS and rate limiting to all room requests
	handler := s.chain(func(w http.ResponseWriter, r *http.Request) {
		// Route based on path
		if path == "/api/rooms" || path == "/api/rooms/" {
			// List or create rooms
			if r.Method == http.MethodGet {
				s.roomHandler.ListRooms(w, r)
			} else if r.Method == http.MethodPost {
				// Create room requires authentication
				s.authMW.Authenticate(s.roomHandler.CreateRoom)(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Extract room ID from path
		// Format: /api/rooms/{roomId}/...
		roomID := s.extractRoomID(path)
		if roomID == "" {
			http.Error(w, "Invalid room ID", http.StatusBadRequest)
			return
		}

		// Check if it's a token request
		if len(path) > len("/api/rooms/"+roomID+"/tokens") &&
			path[:len("/api/rooms/"+roomID+"/tokens")] == "/api/rooms/"+roomID+"/tokens" {
			// Token generation requires authentication
			s.authMW.Authenticate(s.tokenHandler.GenerateAccessToken)(w, r)
			return
		}

		// Check if it's a participant request
		if len(path) > len("/api/rooms/"+roomID+"/participants") &&
			path[:len("/api/rooms/"+roomID+"/participants")] == "/api/rooms/"+roomID+"/participants" {
			// Participant operations
			if r.Method == http.MethodGet {
				s.roomHandler.ListParticipants(w, r)
			} else if r.Method == http.MethodPost {
				s.authMW.Authenticate(s.roomHandler.AddParticipant)(w, r)
			} else if r.Method == http.MethodDelete {
				s.authMW.Authenticate(s.roomHandler.RemoveParticipant)(w, r)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Single room operations
		if r.Method == http.MethodGet {
			s.roomHandler.GetRoom(w, r)
		} else if r.Method == http.MethodDelete {
			s.authMW.Authenticate(s.roomHandler.DeleteRoom)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}, s.corsMW.Handle, s.rateLimiter.Limit)

	handler(w, r)
}

// extractRoomID extracts room ID from path
func (s *Server) extractRoomID(path string) string {
	// Remove /api/rooms/ prefix
	if len(path) < len("/api/rooms/") {
		return ""
	}
	remaining := path[len("/api/rooms/"):]

	// Find next slash
	for i, c := range remaining {
		if c == '/' {
			return remaining[:i]
		}
	}

	return remaining
}

// chain chains multiple middleware together
func (s *Server) chain(handler http.HandlerFunc, middleware ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	// Apply middleware in reverse order so they execute in the correct order
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// healthCheck handles health check requests
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}
