package chat

import (
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// ModerationAction represents a moderation action type
type ModerationAction string

const (
	// ActionBan permanently bans a user
	ActionBan ModerationAction = "ban"
	// ActionMute temporarily prevents a user from sending messages
	ActionMute ModerationAction = "mute"
	// ActionUnban removes a ban
	ActionUnban ModerationAction = "unban"
	// ActionUnmute removes a mute
	ActionUnmute ModerationAction = "unmute"
)

// Moderator handles chat moderation
// Note: Only stores state in-memory for current session.
// Users should log moderation events to their own database if needed.
type Moderator struct {
	// bannedUsers stores banned user IDs by room
	bannedUsers map[string]map[string]bool
	// mutedUsers stores muted users with expiration times
	mutedUsers map[string]map[string]time.Time
	// mu protects concurrent access
	mu sync.RWMutex
	// logger for moderation events
	logger logger.Logger
}

// NewModerator creates a new moderator
// Note: State is kept in-memory only. Log moderation events to your database if needed.
func NewModerator(log logger.Logger) *Moderator {
	return &Moderator{
		bannedUsers: make(map[string]map[string]bool),
		mutedUsers:  make(map[string]map[string]time.Time),
		logger:      log,
	}
}

// BanUser permanently bans a user from a room
func (m *Moderator) BanUser(roomID, userID, moderatorID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bannedUsers[roomID] == nil {
		m.bannedUsers[roomID] = make(map[string]bool)
	}

	m.bannedUsers[roomID][userID] = true

	m.logger.Info("User banned", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "moderator_id", Value: moderatorID}, logger.Field{Key: "reason", Value: reason})

	return nil
}

// UnbanUser removes a ban from a user
func (m *Moderator) UnbanUser(roomID, userID, moderatorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bannedUsers[roomID] != nil {
		delete(m.bannedUsers[roomID], userID)
	}

	m.logger.Info("User unbanned", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "moderator_id", Value: moderatorID})

	return nil
}

// IsUserBanned checks if a user is banned from a room
func (m *Moderator) IsUserBanned(roomID, userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.bannedUsers[roomID] == nil {
		return false
	}

	return m.bannedUsers[roomID][userID]
}

// MuteUser mutes a user for a specified duration
func (m *Moderator) MuteUser(roomID, userID, moderatorID, reason string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mutedUsers[roomID] == nil {
		m.mutedUsers[roomID] = make(map[string]time.Time)
	}

	expiresAt := time.Now().Add(duration)
	m.mutedUsers[roomID][userID] = expiresAt

	m.logger.Info("User muted", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "moderator_id", Value: moderatorID}, logger.Field{Key: "duration", Value: duration}, logger.Field{Key: "reason", Value: reason})

	return nil
}

// UnmuteUser removes a mute from a user
func (m *Moderator) UnmuteUser(roomID, userID, moderatorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mutedUsers[roomID] != nil {
		delete(m.mutedUsers[roomID], userID)
	}

	m.logger.Info("User unmuted", logger.Field{Key: "room_id", Value: roomID}, logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "moderator_id", Value: moderatorID})

	return nil
}

// IsUserMuted checks if a user is currently muted
func (m *Moderator) IsUserMuted(roomID, userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.mutedUsers[roomID] == nil {
		return false
	}

	expiresAt, exists := m.mutedUsers[roomID][userID]
	if !exists {
		return false
	}

	// Check if mute has expired
	if time.Now().After(expiresAt) {
		// Cleanup expired mute (upgrade to write lock)
		m.mu.RUnlock()
		m.mu.Lock()
		delete(m.mutedUsers[roomID], userID)
		m.mu.Unlock()
		m.mu.RLock()
		return false
	}

	return true
}

// GetBannedUsers returns all banned users in a room
func (m *Moderator) GetBannedUsers(roomID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.bannedUsers[roomID] == nil {
		return []string{}
	}

	users := make([]string, 0, len(m.bannedUsers[roomID]))
	for userID := range m.bannedUsers[roomID] {
		users = append(users, userID)
	}

	return users
}

// GetMutedUsers returns all currently muted users in a room
func (m *Moderator) GetMutedUsers(roomID string) map[string]time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]time.Time)

	if m.mutedUsers[roomID] == nil {
		return result
	}

	now := time.Now()
	for userID, expiresAt := range m.mutedUsers[roomID] {
		// Only include non-expired mutes
		if now.Before(expiresAt) {
			result[userID] = expiresAt
		}
	}

	return result
}

// CleanupExpired removes expired mutes
func (m *Moderator) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Clean up expired mutes
	for _, users := range m.mutedUsers {
		for userID, expiresAt := range users {
			if now.After(expiresAt) {
				delete(users, userID)
			}
		}
	}
}

// CanUserPerformAction checks if a user has permission to perform a moderation action
func CanUserPerformAction(user *User, action ModerationAction) bool {
	switch user.Role {
	case RoleBroadcaster, RoleAdmin:
		// Broadcasters and admins can perform all actions
		return true
	case RoleModerator:
		// Moderators can perform most actions except banning
		return action != ActionBan
	default:
		// Viewers cannot perform moderation actions
		return false
	}
}
