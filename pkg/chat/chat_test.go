package chat

import (
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// mockConnection implements Connection interface for testing
type mockConnection struct {
	id       string
	messages []*Message
	closed   bool
}

func (m *mockConnection) SendMessage(msg *Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockConnection) Close() error {
	m.closed = true
	return nil
}

func (m *mockConnection) ID() string {
	return m.id
}

// TestMessageValidator tests message validation
func TestMessageValidator(t *testing.T) {
	rules := DefaultValidationRules()
	validator := NewMessageValidator(rules)

	// Test valid message
	msg := &Message{Type: MessageTypeText, Content: "Hello"}
	if err := validator.Validate(msg); err != nil {
		t.Errorf("Valid message failed validation: %v", err)
	}

	// Test message too long
	msg.Content = string(make([]byte, 600))
	if err := validator.Validate(msg); err == nil {
		t.Error("Long message should fail validation")
	}
}

// TestRoom tests room functionality
func TestRoom(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	room := NewRoom("room1", "stream1", "Test", DefaultRoomConfig(), log)

	// Add user
	user := &User{ID: "user1", Username: "Alice", Role: RoleViewer}
	conn := &mockConnection{id: "conn1"}
	if err := room.AddUser(user, conn); err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	if room.GetUserCount() != 1 {
		t.Error("User count should be 1")
	}

	// Broadcast message
	msg := &Message{
		ID:        "msg1",
		RoomID:    room.ID,
		UserID:    user.ID,
		Username:  user.Username,
		Type:      MessageTypeText,
		Content:   "Hello",
		Timestamp: time.Now(),
	}
	if err := room.BroadcastMessage(msg); err != nil {
		t.Fatalf("Failed to broadcast: %v", err)
	}

	if len(conn.messages) == 0 {
		t.Error("Message should have been sent")
	}
}

// TestModerator tests moderation
func TestModerator(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	mod := NewModerator(log)

	// Ban user
	if err := mod.BanUser("room1", "user1", "mod1", "spam"); err != nil {
		t.Fatalf("Failed to ban: %v", err)
	}

	if !mod.IsUserBanned("room1", "user1") {
		t.Error("User should be banned")
	}

	// Mute user
	if err := mod.MuteUser("room1", "user2", "mod1", "caps", 5*time.Minute); err != nil {
		t.Fatalf("Failed to mute: %v", err)
	}

	if !mod.IsUserMuted("room1", "user2") {
		t.Error("User should be muted")
	}
}

// TestEmoteManager tests custom emotes
func TestEmoteManager(t *testing.T) {
	manager := NewEmoteManager()

	emote := &CustomEmote{
		ID:   "emote1",
		Name: ":happy:",
		URL:  "https://example.com/happy.png",
	}

	if err := manager.AddGlobalEmote(emote); err != nil {
		t.Fatalf("Failed to add emote: %v", err)
	}

	retrieved, ok := manager.GetEmote(":happy:", "room1")
	if !ok {
		t.Error("Failed to get emote")
	}
	if !retrieved.IsGlobal {
		t.Error("Emote should be global")
	}
}

// TestRateLimiter tests rate limiting
func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)

	// Should allow 3 messages
	for i := 0; i < 3; i++ {
		if !limiter.Allow("user1") {
			t.Errorf("Message %d should be allowed", i+1)
		}
	}

	// 4th should be blocked
	if limiter.Allow("user1") {
		t.Error("4th message should be blocked")
	}

	// Reset should allow again
	limiter.Reset("user1")
	if !limiter.Allow("user1") {
		t.Error("Should allow after reset")
	}
}
