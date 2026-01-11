package chat

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Server represents a WebSocket chat server
type Server struct {
	// rooms stores chat rooms by ID
	rooms map[string]*Room
	// roomsMu protects concurrent access to rooms
	roomsMu sync.RWMutex
	// upgrader upgrades HTTP connections to WebSocket
	upgrader websocket.Upgrader
	// logger for server events
	logger logger.Logger
	// config contains server configuration
	config ServerConfig
	// validator validates messages
	validator *MessageValidator
	// emoteManager manages custom emotes
	emoteManager *EmoteManager
	// moderator handles moderation
	moderator *Moderator
	// rateLimiter limits message rates
	rateLimiter *RateLimiter
}

// ServerConfig contains server configuration
type ServerConfig struct {
	// ReadBufferSize is the WebSocket read buffer size
	ReadBufferSize int
	// WriteBufferSize is the WebSocket write buffer size
	WriteBufferSize int
	// CheckOrigin checks the origin of WebSocket requests
	CheckOrigin func(r *http.Request) bool
	// PingInterval is how often to send ping messages
	PingInterval time.Duration
	// PongTimeout is how long to wait for pong responses
	PongTimeout time.Duration
	// WriteTimeout is the write deadline for messages
	WriteTimeout time.Duration
	// MaxMessageSize is the maximum message size in bytes
	MaxMessageSize int64
	// RoomConfig is the default room configuration
	RoomConfig RoomConfig
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
		PingInterval:   30 * time.Second,
		PongTimeout:    60 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxMessageSize: 512 * 1024, // 512 KB
		RoomConfig:     DefaultRoomConfig(),
	}
}

// NewServer creates a new chat server
// Note: This server only handles real-time message delivery via WebSocket.
// Users are responsible for persisting chat messages to their own database.
func NewServer(config ServerConfig, log logger.Logger) *Server {
	return &Server{
		rooms: make(map[string]*Room),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin:     config.CheckOrigin,
		},
		logger:       log,
		config:       config,
		validator:    NewMessageValidator(DefaultValidationRules()),
		emoteManager: NewEmoteManager(),
		moderator:    NewModerator(log),
		rateLimiter:  NewRateLimiter(10, time.Minute), // 10 messages per minute default
	}
}

// CreateRoom creates a new chat room
func (s *Server) CreateRoom(ctx context.Context, streamID, name string) (*Room, error) {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	roomID := fmt.Sprintf("room_%s", streamID)

	// Check if room already exists
	if _, exists := s.rooms[roomID]; exists {
		return nil, fmt.Errorf("room already exists")
	}

	room := NewRoom(roomID, streamID, name, s.config.RoomConfig, s.logger)
	s.rooms[roomID] = room

	s.logger.Info("Room created", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "stream_id", Value: streamID})

	return room, nil
}

// GetRoom returns a room by ID
func (s *Server) GetRoom(roomID string) (*Room, bool) {
	s.roomsMu.RLock()
	defer s.roomsMu.RUnlock()

	room, exists := s.rooms[roomID]
	return room, exists
}

// DeleteRoom deletes a room
func (s *Server) DeleteRoom(ctx context.Context, roomID string) error {
	s.roomsMu.Lock()
	defer s.roomsMu.Unlock()

	room, exists := s.rooms[roomID]
	if !exists {
		return fmt.Errorf("room not found")
	}

	// Close the room
	room.Close()

	// Remove from map
	delete(s.rooms, roomID)

	s.logger.Info("Room deleted", logger.Field{Key: "room_id", Value: roomID})

	return nil
}

// GetRooms returns all active rooms
func (s *Server) GetRooms() []*Room {
	s.roomsMu.RLock()
	defer s.roomsMu.RUnlock()

	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

// HandleWebSocket handles WebSocket connections
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection", logger.Field{Key: "error", Value: err})
		return
	}

	// Get room ID and user info from query params
	roomID := r.URL.Query().Get("room_id")
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")

	if roomID == "" || userID == "" || username == "" {
		s.logger.Error("Missing required parameters", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "user_id", Value: userID})
		conn.Close()
		return
	}

	// Get room
	room, exists := s.GetRoom(roomID)
	if !exists {
		s.logger.Error("Room not found", logger.Field{Key: "room_id", Value: roomID})
		conn.Close()
		return
	}

	// Create user
	user := &User{
		ID:       userID,
		Username: username,
		Role:     RoleViewer, // Default role
		Metadata: make(map[string]interface{}),
	}

	// Create WebSocket connection wrapper
	wsConn := NewWebSocketConnection(conn, s.config, s.logger)

	// Add user to room
	if err := room.AddUser(user, wsConn); err != nil {
		s.logger.Error("Failed to add user to room", logger.Field{Key: "error", Value: err})
		conn.Close()
		return
	}

	// Handle connection
	s.handleConnection(wsConn, room, user)
}

// handleConnection handles a WebSocket connection
func (s *Server) handleConnection(conn *WebSocketConnection, room *Room, user *User) {
	defer func() {
		room.RemoveUser(user.ID)
		conn.Close()
	}()

	// Start ping/pong handler
	go conn.StartPingPong()

	// Read messages
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket error", logger.Field{Key: "error", Value: err})
			}
			break
		}

		// Set message metadata
		msg.ID = uuid.New().String()
		msg.RoomID = room.ID
		msg.UserID = user.ID
		msg.Username = user.Username
		msg.Timestamp = time.Now()

		// Handle special message types
		if msg.Type == MessageTypeTyping {
			isTyping := false
			if val, ok := msg.Metadata["is_typing"].(bool); ok {
				isTyping = val
			}
			room.SetUserTyping(user.ID, isTyping)
			continue
		}

		// Check rate limit
		if !s.rateLimiter.Allow(user.ID) {
			errMsg := &Message{
				ID:        uuid.New().String(),
				RoomID:    room.ID,
				Type:      MessageTypeSystem,
				Content:   "You are sending messages too quickly. Please slow down.",
				Timestamp: time.Now(),
			}
			conn.SendMessage(errMsg)
			continue
		}

		// Check if user is muted
		if s.moderator.IsUserMuted(room.ID, user.ID) {
			errMsg := &Message{
				ID:        uuid.New().String(),
				RoomID:    room.ID,
				Type:      MessageTypeSystem,
				Content:   "You are muted and cannot send messages.",
				Timestamp: time.Now(),
			}
			conn.SendMessage(errMsg)
			continue
		}

		// Validate message
		if err := s.validator.Validate(&msg); err != nil {
			errMsg := &Message{
				ID:        uuid.New().String(),
				RoomID:    room.ID,
				Type:      MessageTypeSystem,
				Content:   fmt.Sprintf("Message validation failed: %s", err.Error()),
				Timestamp: time.Now(),
			}
			conn.SendMessage(errMsg)
			continue
		}

		// Broadcast message (real-time delivery only)
		// Note: Users should save messages to their own database if needed
		if err := room.BroadcastMessage(&msg); err != nil {
			s.logger.Error("Failed to broadcast message", logger.Field{Key: "error", Value: err})
		}
	}
}

// WebSocketConnection wraps a WebSocket connection
type WebSocketConnection struct {
	conn   *websocket.Conn
	config ServerConfig
	logger logger.Logger
	sendMu sync.Mutex
	stopCh chan struct{}
}

// NewWebSocketConnection creates a new WebSocket connection wrapper
func NewWebSocketConnection(conn *websocket.Conn, config ServerConfig, log logger.Logger) *WebSocketConnection {
	return &WebSocketConnection{
		conn:   conn,
		config: config,
		logger: log,
		stopCh: make(chan struct{}),
	}
}

// SendMessage sends a message to the connection
func (c *WebSocketConnection) SendMessage(msg *Message) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	return c.conn.WriteJSON(msg)
}

// ReadJSON reads a JSON message from the connection
func (c *WebSocketConnection) ReadJSON(v interface{}) error {
	c.conn.SetReadLimit(c.config.MaxMessageSize)
	return c.conn.ReadJSON(v)
}

// Close closes the connection
func (c *WebSocketConnection) Close() error {
	close(c.stopCh)
	return c.conn.Close()
}

// ID returns the connection identifier
func (c *WebSocketConnection) ID() string {
	return c.conn.RemoteAddr().String()
}

// StartPingPong starts the ping/pong handler
func (c *WebSocketConnection) StartPingPong() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	// Set pong handler
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
		return nil
	})

	// Set initial read deadline
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))

	for {
		select {
		case <-ticker.C:
			c.sendMu.Lock()
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.sendMu.Unlock()
				return
			}
			c.sendMu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// BroadcastToRoom broadcasts a message to all users in a room
func (s *Server) BroadcastToRoom(ctx context.Context, roomID string, msg *Message) error {
	room, exists := s.GetRoom(roomID)
	if !exists {
		return fmt.Errorf("room not found")
	}

	return room.BroadcastMessage(msg)
}

// SendSystemMessage sends a system message to a room
func (s *Server) SendSystemMessage(ctx context.Context, roomID, content string) error {
	msg := &Message{
		ID:        uuid.New().String(),
		RoomID:    roomID,
		Type:      MessageTypeSystem,
		Content:   content,
		Timestamp: time.Now(),
	}

	return s.BroadcastToRoom(ctx, roomID, msg)
}

// GetRoomStats returns statistics for a room
type RoomStats struct {
	RoomID          string    `json:"room_id"`
	UserCount       int       `json:"user_count"`
	CreatedAt       time.Time `json:"created_at"`
	TypingUserCount int       `json:"typing_user_count"`
}

// GetRoomStats returns statistics for a room
func (s *Server) GetRoomStats(roomID string) (*RoomStats, error) {
	room, exists := s.GetRoom(roomID)
	if !exists {
		return nil, fmt.Errorf("room not found")
	}

	return &RoomStats{
		RoomID:          room.ID,
		UserCount:       room.GetUserCount(),
		CreatedAt:       room.CreatedAt,
		TypingUserCount: len(room.GetTypingUsers()),
	}, nil
}

// RateLimiter implements per-user rate limiting
type RateLimiter struct {
	limits   map[string]*userLimit
	mu       sync.RWMutex
	maxCount int
	window   time.Duration
}

type userLimit struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxCount int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits:   make(map[string]*userLimit),
		maxCount: maxCount,
		window:   window,
	}
}

// Allow checks if a user is allowed to send a message
func (rl *RateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	limit, exists := rl.limits[userID]
	if !exists {
		rl.limits[userID] = &userLimit{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	// Reset if window has passed
	if now.After(limit.resetTime) {
		limit.count = 1
		limit.resetTime = now.Add(rl.window)
		return true
	}

	// Check if under limit
	if limit.count < rl.maxCount {
		limit.count++
		return true
	}

	return false
}

// Reset resets the rate limit for a user
func (rl *RateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.limits, userID)
}

// Cleanup removes expired limits
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, limit := range rl.limits {
		if now.After(limit.resetTime) {
			delete(rl.limits, userID)
		}
	}
}
