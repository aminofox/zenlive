package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// UserRole represents a user's role in the chat room
type UserRole string

const (
	// RoleViewer is a regular viewer
	RoleViewer UserRole = "viewer"
	// RoleModerator can moderate chat
	RoleModerator UserRole = "moderator"
	// RoleAdmin has full control
	RoleAdmin UserRole = "admin"
	// RoleBroadcaster is the stream owner
	RoleBroadcaster UserRole = "broadcaster"
)

// User represents a user in the chat room
type User struct {
	// ID is the user identifier
	ID string `json:"id"`
	// Username is the display name
	Username string `json:"username"`
	// Role is the user's role
	Role UserRole `json:"role"`
	// JoinedAt is when the user joined
	JoinedAt time.Time `json:"joined_at"`
	// LastActivity is the last activity timestamp
	LastActivity time.Time `json:"last_activity"`
	// IsTyping indicates if user is typing
	IsTyping bool `json:"is_typing"`
	// Metadata contains additional user data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Room represents a chat room
type Room struct {
	// ID is the room identifier (typically stream ID)
	ID string
	// StreamID is the associated stream
	StreamID string
	// Name is the room display name
	Name string
	// CreatedAt is when the room was created
	CreatedAt time.Time
	// users stores connected users by ID
	users map[string]*User
	// connections stores WebSocket connections by user ID
	connections map[string]Connection
	// mu protects concurrent access
	mu sync.RWMutex
	// logger for room events
	logger logger.Logger
	// broadcaster for message broadcasting
	broadcaster *MessageBroadcaster
	// isClosed indicates if the room is closed
	isClosed bool
}

// Connection represents a WebSocket connection interface
type Connection interface {
	// SendMessage sends a message to the connection
	SendMessage(msg *Message) error
	// Close closes the connection
	Close() error
	// ID returns the connection identifier
	ID() string
}

// RoomConfig contains room configuration
type RoomConfig struct {
	// EnableTypingIndicators enables typing indicators
	EnableTypingIndicators bool
	// EnableReadReceipts enables read receipts
	EnableReadReceipts bool
	// PresenceTimeout is how long before marking user as inactive
	PresenceTimeout time.Duration
}

// DefaultRoomConfig returns default room configuration
func DefaultRoomConfig() RoomConfig {
	return RoomConfig{
		EnableTypingIndicators: true,
		EnableReadReceipts:     true,
		PresenceTimeout:        5 * time.Minute,
	}
}

// NewRoom creates a new chat room
// Note: Room only handles real-time message delivery. No message history is stored.
// Users should implement their own message persistence if needed.
func NewRoom(id, streamID, name string, config RoomConfig, log logger.Logger) *Room {
	return &Room{
		ID:          id,
		StreamID:    streamID,
		Name:        name,
		CreatedAt:   time.Now(),
		users:       make(map[string]*User),
		connections: make(map[string]Connection),
		logger:      log,
		broadcaster: NewMessageBroadcaster(log),
		isClosed:    false,
	}
}

// AddUser adds a user to the room
func (r *Room) AddUser(user *User, conn Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return fmt.Errorf("room is closed")
	}

	// Check if user already exists
	if _, exists := r.users[user.ID]; exists {
		return fmt.Errorf("user already in room")
	}

	// Set join time
	user.JoinedAt = time.Now()
	user.LastActivity = time.Now()

	// Add user and connection
	r.users[user.ID] = user
	r.connections[user.ID] = conn

	r.logger.Info("User joined room", logger.Field{Key: "room_id", Value: r.ID}, logger.Field{Key: "user_id", Value: user.ID}, logger.Field{Key: "username", Value: user.Username})

	// Broadcast join message
	joinMsg := &Message{
		ID:        fmt.Sprintf("join_%s_%d", user.ID, time.Now().UnixNano()),
		RoomID:    r.ID,
		UserID:    user.ID,
		Username:  user.Username,
		Type:      MessageTypeJoin,
		Content:   fmt.Sprintf("%s joined the chat", user.Username),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"user_count": len(r.users),
		},
	}
	r.broadcaster.Broadcast(joinMsg, r.connections)

	return nil
}

// RemoveUser removes a user from the room
func (r *Room) RemoveUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user not in room")
	}

	// Close connection
	if conn, ok := r.connections[userID]; ok {
		conn.Close()
	}

	// Remove user
	delete(r.users, userID)
	delete(r.connections, userID)

	r.logger.Info("User left room", logger.Field{Key: "room_id", Value: r.ID}, logger.Field{Key: "user_id", Value: userID})

	// Broadcast leave message
	leaveMsg := &Message{
		ID:        fmt.Sprintf("leave_%s_%d", userID, time.Now().UnixNano()),
		RoomID:    r.ID,
		UserID:    userID,
		Username:  user.Username,
		Type:      MessageTypeLeave,
		Content:   fmt.Sprintf("%s left the chat", user.Username),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"user_count": len(r.users),
		},
	}
	r.broadcaster.Broadcast(leaveMsg, r.connections)

	return nil
}

// GetUser returns a user by ID
func (r *Room) GetUser(userID string) (*User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	return user, exists
}

// GetUsers returns all users in the room
func (r *Room) GetUsers() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users
}

// GetUserCount returns the number of users in the room
func (r *Room) GetUserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.users)
}

// BroadcastMessage broadcasts a message to all users
func (r *Room) BroadcastMessage(msg *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return fmt.Errorf("room is closed")
	}

	// Update user activity
	if user, exists := r.users[msg.UserID]; exists {
		user.LastActivity = time.Now()
		user.IsTyping = false
	}

	// Broadcast to all connections (real-time only)
	r.broadcaster.Broadcast(msg, r.connections)

	r.logger.Debug("Message broadcasted", logger.Field{Key: "room_id", Value: r.ID}, logger.Field{Key: "message_id", Value: msg.ID}, logger.Field{Key: "user_count", Value: len(r.connections)})

	return nil
}

// SendToUser sends a message to a specific user
func (r *Room) SendToUser(userID string, msg *Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, exists := r.connections[userID]
	if !exists {
		return fmt.Errorf("user not connected")
	}

	return conn.SendMessage(msg)
}

// SetUserTyping sets the typing status for a user
func (r *Room) SetUserTyping(userID string, isTyping bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user not in room")
	}

	user.IsTyping = isTyping
	user.LastActivity = time.Now()

	// Broadcast typing indicator
	typingMsg := &Message{
		ID:        fmt.Sprintf("typing_%s_%d", userID, time.Now().UnixNano()),
		RoomID:    r.ID,
		UserID:    userID,
		Username:  user.Username,
		Type:      MessageTypeTyping,
		Content:   "",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"is_typing": isTyping,
		},
	}

	r.broadcaster.Broadcast(typingMsg, r.connections)
	return nil
}

// GetTypingUsers returns all users currently typing
func (r *Room) GetTypingUsers() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	typing := make([]*User, 0)
	for _, user := range r.users {
		if user.IsTyping {
			typing = append(typing, user)
		}
	}
	return typing
}

// UpdateUserRole updates a user's role
func (r *Room) UpdateUserRole(userID string, role UserRole) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user not in room")
	}

	oldRole := user.Role
	user.Role = role

	r.logger.Info("User role updated", logger.Field{Key: "room_id", Value: r.ID}, logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "old_role", Value: oldRole}, logger.Field{Key: "new_role", Value: role})

	return nil
}

// Close closes the room and disconnects all users
func (r *Room) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return nil
	}

	r.isClosed = true

	// Close all connections
	for _, conn := range r.connections {
		conn.Close()
	}

	// Clear users and connections
	r.users = make(map[string]*User)
	r.connections = make(map[string]Connection)

	r.logger.Info("Room closed", logger.Field{Key: "room_id", Value: r.ID})

	return nil
}

// IsClosed returns whether the room is closed
func (r *Room) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isClosed
}

// MessageBroadcaster handles message broadcasting to multiple connections
type MessageBroadcaster struct {
	logger logger.Logger
}

// NewMessageBroadcaster creates a new message broadcaster
func NewMessageBroadcaster(log logger.Logger) *MessageBroadcaster {
	return &MessageBroadcaster{
		logger: log,
	}
}

// Broadcast sends a message to all connections
func (mb *MessageBroadcaster) Broadcast(msg *Message, connections map[string]Connection) {
	// Send to all connections concurrently
	var wg sync.WaitGroup
	for userID, conn := range connections {
		wg.Add(1)
		go func(uid string, c Connection) {
			defer wg.Done()
			if err := c.SendMessage(msg); err != nil {
				mb.logger.Error("Failed to send message", logger.Field{Key: "user_id", Value: uid}, logger.Field{Key: "error", Value: err})
			}
		}(userID, conn)
	}
	wg.Wait()
}

// BroadcastExcept sends a message to all connections except one
func (mb *MessageBroadcaster) BroadcastExcept(msg *Message, connections map[string]Connection, excludeUserID string) {
	var wg sync.WaitGroup
	for userID, conn := range connections {
		if userID == excludeUserID {
			continue
		}
		wg.Add(1)
		go func(uid string, c Connection) {
			defer wg.Done()
			if err := c.SendMessage(msg); err != nil {
				mb.logger.Error("Failed to send message", logger.Field{Key: "user_id", Value: uid}, logger.Field{Key: "error", Value: err})
			}
		}(userID, conn)
	}
	wg.Wait()
}
