package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/chat"
	"github.com/aminofox/zenlive/pkg/logger"
)

func main() {
	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	log.Info("Starting ZenLive Chat Server Example")

	// Run examples
	runChatServerExample(log)
}

// runChatServerExample demonstrates a complete chat server setup
func runChatServerExample(log logger.Logger) {
	ctx := context.Background()

	// Create server (no storage - users handle their own persistence)
	config := chat.DefaultServerConfig()
	server := chat.NewServer(config, log)

	log.Info("Chat server created - real-time messaging only")

	// Create rooms for different streams
	room1, err := server.CreateRoom(ctx, "stream-123", "Gaming Stream Chat")
	if err != nil {
		log.Fatal("Failed to create room", logger.Field{Key: "error", Value: err})
	}
	log.Info("Room created", logger.Field{Key: "room_id", Value: room1.ID}, logger.Field{Key: "name", Value: room1.Name})

	room2, err := server.CreateRoom(ctx, "stream-456", "Music Stream Chat")
	if err != nil {
		log.Fatal("Failed to create room", logger.Field{Key: "error", Value: err})
	}
	log.Info("Room created", logger.Field{Key: "room_id", Value: room2.ID}, logger.Field{Key: "name", Value: room2.Name})

	// Demonstrate room management
	log.Info("=== Room Management Example ===")
	runRoomManagementExample(server, room1, log)

	// Demonstrate moderation
	log.Info("=== Moderation Example ===")
	runModerationExample(server, room1, log)

	// Demonstrate emotes
	log.Info("=== Custom Emotes Example ===")
	runEmoteExample(log)

	// Start HTTP server for WebSocket connections
	log.Info("=== Starting HTTP Server ===")

	// Setup routes
	http.HandleFunc("/ws", server.HandleWebSocket)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/rooms", func(w http.ResponseWriter, r *http.Request) {
		rooms := server.GetRooms()
		fmt.Fprintf(w, "Active Rooms: %d\n", len(rooms))
		for _, room := range rooms {
			stats, _ := server.GetRoomStats(room.ID)
			fmt.Fprintf(w, "- %s: %d users\n", room.Name, stats.UserCount)
		}
	})

	// Start server in goroutine
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", logger.Field{Key: "addr", Value: ":8080"})
		log.Info("WebSocket endpoint", logger.Field{Key: "path", Value: "/ws?room_id=<id>&user_id=<id>&username=<name>"})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", logger.Field{Key: "error", Value: err})
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", logger.Field{Key: "error", Value: err})
	}

	// Close rooms
	for _, room := range server.GetRooms() {
		room.Close()
	}

	log.Info("Server stopped")
}

// runRoomManagementExample demonstrates room management
func runRoomManagementExample(server *chat.Server, room *chat.Room, log logger.Logger) {
	ctx := context.Background()

	// Create mock connection
	conn := &MockConnection{
		id:       "conn-1",
		messages: make([]*chat.Message, 0),
	}

	// Add users to room
	user1 := &chat.User{
		ID:       "user-1",
		Username: "Alice",
		Role:     chat.RoleViewer,
		Metadata: map[string]interface{}{
			"avatar": "https://example.com/alice.png",
		},
	}

	err := room.AddUser(user1, conn)
	if err != nil {
		log.Error("Failed to add user", logger.Field{Key: "error", Value: err})
		return
	}

	// Add moderator
	user2 := &chat.User{
		ID:       "user-2",
		Username: "Bob",
		Role:     chat.RoleModerator,
	}

	conn2 := &MockConnection{id: "conn-2", messages: make([]*chat.Message, 0)}
	err = room.AddUser(user2, conn2)
	if err != nil {
		log.Error("Failed to add moderator", logger.Field{Key: "error", Value: err})
		return
	}

	log.Info("Users added to room", logger.Field{Key: "user_count", Value: room.GetUserCount()})

	// Broadcast message
	msg := &chat.Message{
		ID:        "msg-1",
		RoomID:    room.ID,
		UserID:    user1.ID,
		Username:  user1.Username,
		Type:      chat.MessageTypeText,
		Content:   "Hello everyone!",
		Timestamp: time.Now(),
	}

	err = room.BroadcastMessage(msg)
	if err != nil {
		log.Error("Failed to broadcast message", logger.Field{Key: "error", Value: err})
		return
	}

	log.Info("Message broadcasted", logger.Field{Key: "message_id", Value: msg.ID}, logger.Field{Key: "content", Value: msg.Content})

	// Set typing indicator
	room.SetUserTyping(user1.ID, true)
	typingUsers := room.GetTypingUsers()
	log.Info("Typing users", logger.Field{Key: "count", Value: len(typingUsers)})

	// Update user role
	room.UpdateUserRole(user1.ID, chat.RoleModerator)
	log.Info("User role updated", logger.Field{Key: "user_id", Value: user1.ID}, logger.Field{Key: "new_role", Value: chat.RoleModerator})

	// Get room stats
	stats, err := server.GetRoomStats(room.ID)
	if err == nil {
		log.Info("Room stats", logger.Field{Key: "user_count", Value: stats.UserCount}, logger.Field{Key: "typing_count", Value: stats.TypingUserCount})
	}

	// Send system message
	server.SendSystemMessage(ctx, room.ID, "Welcome to the chat! Be respectful to others.")
	log.Info("System message sent")

	// Note: No message history - SDK only delivers real-time messages
	// Users should save messages to their own database if they need history
}

// runModerationExample demonstrates moderation features
func runModerationExample(server *chat.Server, room *chat.Room, log logger.Logger) {
	// Get moderator from server
	// Note: In a real scenario, this would be injected or accessed differently
	moderator := chat.NewModerator(log)

	// Ban a user
	err := moderator.BanUser(room.ID, "user-toxic", "mod-1", "Spam and harassment")
	if err != nil {
		log.Error("Failed to ban user", logger.Field{Key: "error", Value: err})
	} else {
		log.Info("User banned", logger.Field{Key: "user_id", Value: "user-toxic"})
	}

	// Check if user is banned
	isBanned := moderator.IsUserBanned(room.ID, "user-toxic")
	log.Info("Ban status", logger.Field{Key: "user_id", Value: "user-toxic"}, logger.Field{Key: "is_banned", Value: isBanned})

	// Mute a user for 5 minutes
	err = moderator.MuteUser(room.ID, "user-loud", "mod-1", "Excessive caps", 5*time.Minute)
	if err != nil {
		log.Error("Failed to mute user", logger.Field{Key: "error", Value: err})
	} else {
		log.Info("User muted", logger.Field{Key: "user_id", Value: "user-loud"}, logger.Field{Key: "duration", Value: "5m"})
	}

	// Check if user is muted
	isMuted := moderator.IsUserMuted(room.ID, "user-loud")
	log.Info("Mute status", logger.Field{Key: "user_id", Value: "user-loud"}, logger.Field{Key: "is_muted", Value: isMuted})

	// Get banned users
	bannedUsers := moderator.GetBannedUsers(room.ID)
	log.Info("Banned users", logger.Field{Key: "count", Value: len(bannedUsers)})

	// Get muted users
	mutedUsers := moderator.GetMutedUsers(room.ID)
	log.Info("Muted users", logger.Field{Key: "count", Value: len(mutedUsers)})

	// Cleanup expired mutes
	moderator.CleanupExpired()
	log.Info("Expired moderation actions cleaned up")

	// Unban a user
	err = moderator.UnbanUser(room.ID, "user-toxic", "admin-1")
	if err != nil {
		log.Error("Failed to unban user", logger.Field{Key: "error", Value: err})
	} else {
		log.Info("User unbanned", logger.Field{Key: "user_id", Value: "user-toxic"})
	}

	// Note: Users should log moderation events to their own database if needed
	// This SDK only maintains current session state (who's banned/muted NOW)
}

// runEmoteExample demonstrates custom emotes
func runEmoteExample(log logger.Logger) {
	manager := chat.NewEmoteManager()

	// Add global emotes
	globalEmotes := []*chat.CustomEmote{
		{
			ID:        "emote-1",
			Name:      ":happyface:",
			URL:       "https://cdn.example.com/emotes/happy.png",
			CreatedBy: "admin",
			IsGlobal:  true,
		},
		{
			ID:        "emote-2",
			Name:      ":sadface:",
			URL:       "https://cdn.example.com/emotes/sad.png",
			CreatedBy: "admin",
			IsGlobal:  true,
		},
	}

	for _, emote := range globalEmotes {
		err := manager.AddGlobalEmote(emote)
		if err != nil {
			log.Error("Failed to add global emote", logger.Field{Key: "error", Value: err})
		}
	}

	log.Info("Global emotes added", logger.Field{Key: "count", Value: len(globalEmotes)})

	// Add room-specific emote
	roomEmote := &chat.CustomEmote{
		ID:        "emote-3",
		Name:      ":streamlogo:",
		URL:       "https://cdn.example.com/emotes/logo.png",
		CreatedBy: "broadcaster-1",
	}

	err := manager.AddRoomEmote("room_stream-123", roomEmote)
	if err != nil {
		log.Error("Failed to add room emote", logger.Field{Key: "error", Value: err})
	} else {
		log.Info("Room emote added", logger.Field{Key: "emote_name", Value: roomEmote.Name})
	}

	// Get room emotes
	roomEmotes := manager.GetRoomEmotes("room_stream-123")
	log.Info("Room emotes", logger.Field{Key: "room_id", Value: "room_stream-123"}, logger.Field{Key: "count", Value: len(roomEmotes)})

	// Replace emotes in message
	message := "I'm feeling :happyface: about this stream :streamlogo:"
	replaced := manager.ReplaceEmotesInMessage(message, "room_stream-123")
	log.Info("Emote replacement", logger.Field{Key: "original", Value: message}, logger.Field{Key: "replaced", Value: replaced})
}

// MockConnection is a mock connection for testing
type MockConnection struct {
	id       string
	messages []*chat.Message
	closed   bool
}

func (m *MockConnection) SendMessage(msg *chat.Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MockConnection) Close() error {
	m.closed = true
	return nil
}

func (m *MockConnection) ID() string {
	return m.id
}
