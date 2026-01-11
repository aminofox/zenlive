package chat

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MessageType represents the type of chat message
type MessageType string

const (
	// MessageTypeText represents a regular text message
	MessageTypeText MessageType = "text"
	// MessageTypeEmoji represents an emoji-only message
	MessageTypeEmoji MessageType = "emoji"
	// MessageTypeGift represents a virtual gift message
	MessageTypeGift MessageType = "gift"
	// MessageTypeSystem represents a system message
	MessageTypeSystem MessageType = "system"
	// MessageTypeJoin represents a user join event
	MessageTypeJoin MessageType = "join"
	// MessageTypeLeave represents a user leave event
	MessageTypeLeave MessageType = "leave"
	// MessageTypeTyping represents typing indicator
	MessageTypeTyping MessageType = "typing"
	// MessageTypeReadReceipt represents read receipt
	MessageTypeReadReceipt MessageType = "read_receipt"
)

// Message represents a chat message
type Message struct {
	// ID is the unique message identifier
	ID string `json:"id"`
	// RoomID is the chat room identifier
	RoomID string `json:"room_id"`
	// UserID is the sender's identifier
	UserID string `json:"user_id"`
	// Username is the sender's display name
	Username string `json:"username"`
	// Type is the message type
	Type MessageType `json:"type"`
	// Content is the message content
	Content string `json:"content"`
	// Metadata contains additional message data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`
	// EditedAt is when the message was last edited
	EditedAt *time.Time `json:"edited_at,omitempty"`
	// DeletedAt is when the message was deleted
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	// IsDeleted indicates if the message was deleted
	IsDeleted bool `json:"is_deleted"`
}

// MessageValidationRules defines validation rules for messages
type MessageValidationRules struct {
	// MaxLength is the maximum message length
	MaxLength int
	// MinLength is the minimum message length
	MinLength int
	// AllowEmojis indicates if emojis are allowed
	AllowEmojis bool
	// AllowURLs indicates if URLs are allowed
	AllowURLs bool
	// BlockedWords is a list of blocked words
	BlockedWords []string
}

// DefaultValidationRules returns default validation rules
func DefaultValidationRules() MessageValidationRules {
	return MessageValidationRules{
		MaxLength:    500,
		MinLength:    1,
		AllowEmojis:  true,
		AllowURLs:    true,
		BlockedWords: []string{},
	}
}

// MessageValidator validates chat messages
type MessageValidator struct {
	rules MessageValidationRules
}

// NewMessageValidator creates a new message validator
func NewMessageValidator(rules MessageValidationRules) *MessageValidator {
	return &MessageValidator{
		rules: rules,
	}
}

// Validate validates a message against the rules
func (v *MessageValidator) Validate(msg *Message) error {
	// Skip validation for system messages
	if msg.Type == MessageTypeSystem || msg.Type == MessageTypeJoin || msg.Type == MessageTypeLeave {
		return nil
	}

	// Check content length
	length := utf8.RuneCountInString(msg.Content)
	if length < v.rules.MinLength {
		return fmt.Errorf("message too short: minimum %d characters", v.rules.MinLength)
	}
	if length > v.rules.MaxLength {
		return fmt.Errorf("message too long: maximum %d characters", v.rules.MaxLength)
	}

	// Check blocked words
	lowerContent := strings.ToLower(msg.Content)
	for _, word := range v.rules.BlockedWords {
		if strings.Contains(lowerContent, strings.ToLower(word)) {
			return fmt.Errorf("message contains blocked word: %s", word)
		}
	}

	// Check URLs if not allowed
	if !v.rules.AllowURLs {
		if strings.Contains(lowerContent, "http://") || strings.Contains(lowerContent, "https://") {
			return fmt.Errorf("URLs not allowed in messages")
		}
	}

	return nil
}

// EmojiReaction represents a reaction to a message
type EmojiReaction struct {
	// Emoji is the reaction emoji
	Emoji string `json:"emoji"`
	// UserID is the user who reacted
	UserID string `json:"user_id"`
	// Timestamp is when the reaction was added
	Timestamp time.Time `json:"timestamp"`
}

// MessageReactions stores reactions for a message
type MessageReactions struct {
	// MessageID is the message identifier
	MessageID string `json:"message_id"`
	// Reactions is a map of emoji to list of reactions
	Reactions map[string][]EmojiReaction `json:"reactions"`
}

// AddReaction adds a reaction to the message
func (mr *MessageReactions) AddReaction(emoji, userID string) {
	if mr.Reactions == nil {
		mr.Reactions = make(map[string][]EmojiReaction)
	}

	// Check if user already reacted with this emoji
	for _, reaction := range mr.Reactions[emoji] {
		if reaction.UserID == userID {
			return // Already reacted
		}
	}

	mr.Reactions[emoji] = append(mr.Reactions[emoji], EmojiReaction{
		Emoji:     emoji,
		UserID:    userID,
		Timestamp: time.Now(),
	})
}

// RemoveReaction removes a reaction from the message
func (mr *MessageReactions) RemoveReaction(emoji, userID string) {
	if mr.Reactions == nil {
		return
	}

	reactions := mr.Reactions[emoji]
	filtered := make([]EmojiReaction, 0, len(reactions))
	for _, reaction := range reactions {
		if reaction.UserID != userID {
			filtered = append(filtered, reaction)
		}
	}

	if len(filtered) == 0 {
		delete(mr.Reactions, emoji)
	} else {
		mr.Reactions[emoji] = filtered
	}
}

// GetReactionCount returns the total count of all reactions
func (mr *MessageReactions) GetReactionCount() int {
	count := 0
	for _, reactions := range mr.Reactions {
		count += len(reactions)
	}
	return count
}

// GetEmojiCount returns the count for a specific emoji
func (mr *MessageReactions) GetEmojiCount(emoji string) int {
	return len(mr.Reactions[emoji])
}

// CustomEmote represents a custom emote
type CustomEmote struct {
	// ID is the emote identifier
	ID string `json:"id"`
	// Name is the emote name (e.g., ":happyface:")
	Name string `json:"name"`
	// URL is the image URL for the emote
	URL string `json:"url"`
	// CreatedBy is the user who created the emote
	CreatedBy string `json:"created_by"`
	// IsGlobal indicates if the emote is available globally
	IsGlobal bool `json:"is_global"`
	// RoomID is the room ID if the emote is room-specific
	RoomID string `json:"room_id,omitempty"`
}

// EmoteManager manages custom emotes
type EmoteManager struct {
	// globalEmotes stores global emotes by name
	globalEmotes map[string]*CustomEmote
	// roomEmotes stores room-specific emotes by room ID
	roomEmotes map[string]map[string]*CustomEmote
}

// NewEmoteManager creates a new emote manager
func NewEmoteManager() *EmoteManager {
	return &EmoteManager{
		globalEmotes: make(map[string]*CustomEmote),
		roomEmotes:   make(map[string]map[string]*CustomEmote),
	}
}

// AddGlobalEmote adds a global emote
func (em *EmoteManager) AddGlobalEmote(emote *CustomEmote) error {
	if emote.Name == "" {
		return fmt.Errorf("emote name cannot be empty")
	}
	if emote.URL == "" {
		return fmt.Errorf("emote URL cannot be empty")
	}

	emote.IsGlobal = true
	em.globalEmotes[emote.Name] = emote
	return nil
}

// AddRoomEmote adds a room-specific emote
func (em *EmoteManager) AddRoomEmote(roomID string, emote *CustomEmote) error {
	if roomID == "" {
		return fmt.Errorf("room ID cannot be empty")
	}
	if emote.Name == "" {
		return fmt.Errorf("emote name cannot be empty")
	}
	if emote.URL == "" {
		return fmt.Errorf("emote URL cannot be empty")
	}

	emote.IsGlobal = false
	emote.RoomID = roomID

	if em.roomEmotes[roomID] == nil {
		em.roomEmotes[roomID] = make(map[string]*CustomEmote)
	}

	em.roomEmotes[roomID][emote.Name] = emote
	return nil
}

// GetEmote retrieves an emote by name and room ID
func (em *EmoteManager) GetEmote(name, roomID string) (*CustomEmote, bool) {
	// Check global emotes first
	if emote, ok := em.globalEmotes[name]; ok {
		return emote, true
	}

	// Check room-specific emotes
	if roomEmotes, ok := em.roomEmotes[roomID]; ok {
		if emote, ok := roomEmotes[name]; ok {
			return emote, true
		}
	}

	return nil, false
}

// GetRoomEmotes returns all emotes available in a room (global + room-specific)
func (em *EmoteManager) GetRoomEmotes(roomID string) []*CustomEmote {
	emotes := make([]*CustomEmote, 0)

	// Add global emotes
	for _, emote := range em.globalEmotes {
		emotes = append(emotes, emote)
	}

	// Add room-specific emotes
	if roomEmotes, ok := em.roomEmotes[roomID]; ok {
		for _, emote := range roomEmotes {
			emotes = append(emotes, emote)
		}
	}

	return emotes
}

// RemoveEmote removes an emote
func (em *EmoteManager) RemoveEmote(name, roomID string) {
	// Try to remove from global emotes
	if _, ok := em.globalEmotes[name]; ok {
		delete(em.globalEmotes, name)
		return
	}

	// Try to remove from room emotes
	if roomEmotes, ok := em.roomEmotes[roomID]; ok {
		delete(roomEmotes, name)
	}
}

// ReplaceEmotesInMessage replaces emote codes with URLs in a message
func (em *EmoteManager) ReplaceEmotesInMessage(content, roomID string) string {
	result := content

	// Replace global emotes
	for name, emote := range em.globalEmotes {
		result = strings.ReplaceAll(result, name, emote.URL)
	}

	// Replace room-specific emotes
	if roomEmotes, ok := em.roomEmotes[roomID]; ok {
		for name, emote := range roomEmotes {
			result = strings.ReplaceAll(result, name, emote.URL)
		}
	}

	return result
}
