package interactive

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ReactionType represents the type of reaction
type ReactionType string

const (
	// ReactionTypeLike represents a like/thumbs up
	ReactionTypeLike ReactionType = "like"
	// ReactionTypeLove represents a love/heart
	ReactionTypeLove ReactionType = "love"
	// ReactionTypeFire represents fire/hot
	ReactionTypeFire ReactionType = "fire"
	// ReactionTypeClap represents applause/clapping
	ReactionTypeClap ReactionType = "clap"
	// ReactionTypeLaugh represents laughter/funny
	ReactionTypeLaugh ReactionType = "laugh"
	// ReactionTypeSad represents sadness
	ReactionTypeSad ReactionType = "sad"
	// ReactionTypeWow represents amazement
	ReactionTypeWow ReactionType = "wow"
	// ReactionTypeAngry represents anger
	ReactionTypeAngry ReactionType = "angry"
	// ReactionTypeCustom represents a custom reaction
	ReactionTypeCustom ReactionType = "custom"
)

// Reaction represents a single reaction from a user
type Reaction struct {
	ID         string        `json:"id"`
	StreamID   string        `json:"stream_id"`
	UserID     string        `json:"user_id"`
	Type       ReactionType  `json:"type"`
	CustomData string        `json:"custom_data,omitempty"` // For custom reactions
	Timestamp  time.Time     `json:"timestamp"`
	Duration   time.Duration `json:"duration,omitempty"` // How long to show the reaction
}

// ReactionAggregate represents aggregated reactions for a stream
type ReactionAggregate struct {
	StreamID    string               `json:"stream_id"`
	TotalCount  int                  `json:"total_count"`
	CountByType map[ReactionType]int `json:"count_by_type"`
	RecentRate  float64              `json:"recent_rate"` // Reactions per second in last window
	LastUpdated time.Time            `json:"last_updated"`
}

// ReactionBurst represents a burst of reactions in a short time
type ReactionBurst struct {
	Type      ReactionType `json:"type"`
	Count     int          `json:"count"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Intensity float64      `json:"intensity"` // Reactions per second
}

// CustomReactionConfig represents configuration for a custom reaction
type CustomReactionConfig struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	ImageURL     string                 `json:"image_url"`
	AnimationURL string                 `json:"animation_url,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ReactionManager manages reactions for livestreams
type ReactionManager struct {
	reactions       map[string]*Reaction             // reactionID -> Reaction
	streamReactions map[string][]*Reaction           // streamID -> []*Reaction
	userReactions   map[string][]*Reaction           // userID -> []*Reaction
	aggregates      map[string]*ReactionAggregate    // streamID -> ReactionAggregate
	customReactions map[string]*CustomReactionConfig // customID -> CustomReactionConfig
	mu              sync.RWMutex
	callbacks       ReactionCallbacks
	burstThreshold  int           // Minimum reactions in window to trigger burst
	burstWindow     time.Duration // Time window for burst detection
}

// ReactionCallbacks defines callback functions for reaction events
type ReactionCallbacks struct {
	OnReaction        func(reaction *Reaction)
	OnReactionBurst   func(streamID string, burst *ReactionBurst)
	OnAggregateUpdate func(aggregate *ReactionAggregate)
}

// NewReactionManager creates a new reaction manager
func NewReactionManager() *ReactionManager {
	return &ReactionManager{
		reactions:       make(map[string]*Reaction),
		streamReactions: make(map[string][]*Reaction),
		userReactions:   make(map[string][]*Reaction),
		aggregates:      make(map[string]*ReactionAggregate),
		customReactions: make(map[string]*CustomReactionConfig),
		burstThreshold:  10,              // Default: 10 reactions in window
		burstWindow:     3 * time.Second, // Default: 3 second window
	}
}

// SetCallbacks sets the callback functions for reaction events
func (rm *ReactionManager) SetCallbacks(callbacks ReactionCallbacks) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.callbacks = callbacks
}

// SetBurstThreshold sets the threshold for burst detection
func (rm *ReactionManager) SetBurstThreshold(threshold int, window time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.burstThreshold = threshold
	rm.burstWindow = window
}

// AddReaction adds a reaction from a user
func (rm *ReactionManager) AddReaction(streamID, userID string, reactionType ReactionType, customData string) (*Reaction, error) {
	// Validate reaction type
	if reactionType == ReactionTypeCustom && customData == "" {
		return nil, errors.New("custom data required for custom reactions")
	}

	reaction := &Reaction{
		ID:         generateReactionID(),
		StreamID:   streamID,
		UserID:     userID,
		Type:       reactionType,
		CustomData: customData,
		Timestamp:  time.Now(),
		Duration:   3 * time.Second, // Default duration
	}

	rm.mu.Lock()
	rm.reactions[reaction.ID] = reaction
	rm.streamReactions[streamID] = append(rm.streamReactions[streamID], reaction)
	rm.userReactions[userID] = append(rm.userReactions[userID], reaction)

	// Update aggregate
	aggregate := rm.getOrCreateAggregate(streamID)
	aggregate.TotalCount++
	aggregate.CountByType[reactionType]++
	aggregate.LastUpdated = time.Now()

	// Calculate recent rate (reactions in last 10 seconds)
	aggregate.RecentRate = rm.calculateRecentRate(streamID, 10*time.Second)

	rm.mu.Unlock()

	// Trigger callbacks
	if rm.callbacks.OnReaction != nil {
		rm.callbacks.OnReaction(reaction)
	}

	// Check for burst
	rm.checkBurst(streamID, reactionType)

	// Trigger aggregate update callback
	if rm.callbacks.OnAggregateUpdate != nil {
		rm.callbacks.OnAggregateUpdate(aggregate)
	}

	return reaction, nil
}

// getOrCreateAggregate gets or creates an aggregate for a stream (must be called with lock held)
func (rm *ReactionManager) getOrCreateAggregate(streamID string) *ReactionAggregate {
	aggregate, exists := rm.aggregates[streamID]
	if !exists {
		aggregate = &ReactionAggregate{
			StreamID:    streamID,
			TotalCount:  0,
			CountByType: make(map[ReactionType]int),
			RecentRate:  0,
			LastUpdated: time.Now(),
		}
		rm.aggregates[streamID] = aggregate
	}
	return aggregate
}

// calculateRecentRate calculates reactions per second in the recent window
func (rm *ReactionManager) calculateRecentRate(streamID string, window time.Duration) float64 {
	reactions := rm.streamReactions[streamID]
	if len(reactions) == 0 {
		return 0
	}

	now := time.Now()
	count := 0
	for i := len(reactions) - 1; i >= 0; i-- {
		if now.Sub(reactions[i].Timestamp) > window {
			break
		}
		count++
	}

	return float64(count) / window.Seconds()
}

// checkBurst checks if there's a reaction burst
func (rm *ReactionManager) checkBurst(streamID string, reactionType ReactionType) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	reactions := rm.streamReactions[streamID]
	if len(reactions) < rm.burstThreshold {
		return
	}

	now := time.Now()
	count := 0
	var startTime time.Time

	// Count reactions of this type in the burst window
	for i := len(reactions) - 1; i >= 0; i-- {
		reaction := reactions[i]
		if now.Sub(reaction.Timestamp) > rm.burstWindow {
			break
		}
		if reaction.Type == reactionType {
			count++
			startTime = reaction.Timestamp
		}
	}

	// Check if burst threshold is met
	if count >= rm.burstThreshold {
		burst := &ReactionBurst{
			Type:      reactionType,
			Count:     count,
			StartTime: startTime,
			EndTime:   now,
			Intensity: float64(count) / rm.burstWindow.Seconds(),
		}

		// Trigger burst callback
		if rm.callbacks.OnReactionBurst != nil {
			rm.callbacks.OnReactionBurst(streamID, burst)
		}
	}
}

// GetStreamAggregate returns the aggregate reactions for a stream
func (rm *ReactionManager) GetStreamAggregate(streamID string) (*ReactionAggregate, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	aggregate, exists := rm.aggregates[streamID]
	if !exists {
		return &ReactionAggregate{
			StreamID:    streamID,
			TotalCount:  0,
			CountByType: make(map[ReactionType]int),
			RecentRate:  0,
			LastUpdated: time.Now(),
		}, nil
	}

	// Create a copy to avoid race conditions
	result := &ReactionAggregate{
		StreamID:    aggregate.StreamID,
		TotalCount:  aggregate.TotalCount,
		CountByType: make(map[ReactionType]int),
		RecentRate:  aggregate.RecentRate,
		LastUpdated: aggregate.LastUpdated,
	}
	for k, v := range aggregate.CountByType {
		result.CountByType[k] = v
	}

	return result, nil
}

// GetRecentReactions returns recent reactions for a stream
func (rm *ReactionManager) GetRecentReactions(streamID string, duration time.Duration) []*Reaction {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	reactions := rm.streamReactions[streamID]
	if len(reactions) == 0 {
		return []*Reaction{}
	}

	now := time.Now()
	result := make([]*Reaction, 0)

	for i := len(reactions) - 1; i >= 0; i-- {
		if now.Sub(reactions[i].Timestamp) > duration {
			break
		}
		result = append(result, reactions[i])
	}

	return result
}

// GetUserReactions returns reactions sent by a user
func (rm *ReactionManager) GetUserReactions(userID string, limit int) []*Reaction {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	reactions := rm.userReactions[userID]
	if len(reactions) == 0 {
		return []*Reaction{}
	}

	start := len(reactions) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Reaction, 0, limit)
	for i := len(reactions) - 1; i >= start; i-- {
		result = append(result, reactions[i])
	}

	return result
}

// GetReactionStats returns statistics for reactions in a stream
func (rm *ReactionManager) GetReactionStats(streamID string, duration time.Duration) *ReactionStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	reactions := rm.streamReactions[streamID]
	stats := &ReactionStats{
		StreamID:       streamID,
		TotalReactions: 0,
		ByType:         make(map[ReactionType]int),
		ByUser:         make(map[string]int),
		Timeline:       make([]*ReactionTimeSlot, 0),
	}

	if len(reactions) == 0 {
		return stats
	}

	now := time.Now()

	// Count reactions in the duration
	for i := len(reactions) - 1; i >= 0; i-- {
		reaction := reactions[i]
		if now.Sub(reaction.Timestamp) > duration {
			break
		}
		stats.TotalReactions++
		stats.ByType[reaction.Type]++
		stats.ByUser[reaction.UserID]++
	}

	// Create timeline (1-second intervals)
	if stats.TotalReactions > 0 {
		stats.Timeline = rm.createTimeline(streamID, duration)
	}

	return stats
}

// createTimeline creates a timeline of reactions (must be called with lock held)
func (rm *ReactionManager) createTimeline(streamID string, duration time.Duration) []*ReactionTimeSlot {
	reactions := rm.streamReactions[streamID]
	now := time.Now()

	// Create slots for each second
	slots := int(duration.Seconds())
	timeline := make([]*ReactionTimeSlot, slots)

	for i := 0; i < slots; i++ {
		slotTime := now.Add(-duration + time.Duration(i)*time.Second)
		timeline[i] = &ReactionTimeSlot{
			Timestamp: slotTime,
			Count:     0,
			ByType:    make(map[ReactionType]int),
		}
	}

	// Fill slots with reaction data
	for i := len(reactions) - 1; i >= 0; i-- {
		reaction := reactions[i]
		timeDiff := now.Sub(reaction.Timestamp)
		if timeDiff > duration {
			break
		}

		slotIndex := slots - 1 - int(timeDiff.Seconds())
		if slotIndex >= 0 && slotIndex < slots {
			timeline[slotIndex].Count++
			timeline[slotIndex].ByType[reaction.Type]++
		}
	}

	return timeline
}

// ReactionStats represents statistics for reactions
type ReactionStats struct {
	StreamID       string               `json:"stream_id"`
	TotalReactions int                  `json:"total_reactions"`
	ByType         map[ReactionType]int `json:"by_type"`
	ByUser         map[string]int       `json:"by_user"`
	Timeline       []*ReactionTimeSlot  `json:"timeline"`
}

// ReactionTimeSlot represents reactions in a time slot
type ReactionTimeSlot struct {
	Timestamp time.Time            `json:"timestamp"`
	Count     int                  `json:"count"`
	ByType    map[ReactionType]int `json:"by_type"`
}

// AddCustomReaction adds a custom reaction configuration
func (rm *ReactionManager) AddCustomReaction(config *CustomReactionConfig) error {
	if config.ID == "" {
		return errors.New("custom reaction ID cannot be empty")
	}
	if config.Name == "" {
		return errors.New("custom reaction name cannot be empty")
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.customReactions[config.ID]; exists {
		return fmt.Errorf("custom reaction already exists: %s", config.ID)
	}

	rm.customReactions[config.ID] = config
	return nil
}

// GetCustomReaction returns a custom reaction configuration
func (rm *ReactionManager) GetCustomReaction(customID string) (*CustomReactionConfig, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	config, exists := rm.customReactions[customID]
	if !exists {
		return nil, fmt.Errorf("custom reaction not found: %s", customID)
	}

	return config, nil
}

// GetAllCustomReactions returns all custom reaction configurations
func (rm *ReactionManager) GetAllCustomReactions() []*CustomReactionConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	configs := make([]*CustomReactionConfig, 0, len(rm.customReactions))
	for _, config := range rm.customReactions {
		configs = append(configs, config)
	}

	return configs
}

// DeleteCustomReaction removes a custom reaction configuration
func (rm *ReactionManager) DeleteCustomReaction(customID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.customReactions[customID]; !exists {
		return fmt.Errorf("custom reaction not found: %s", customID)
	}

	delete(rm.customReactions, customID)
	return nil
}

// CleanupOldReactions removes reactions older than the specified duration
func (rm *ReactionManager) CleanupOldReactions(maxAge time.Duration) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	cleanedCount := 0

	// Clean up from streamReactions
	for streamID, reactions := range rm.streamReactions {
		newReactions := make([]*Reaction, 0)
		for _, reaction := range reactions {
			if now.Sub(reaction.Timestamp) <= maxAge {
				newReactions = append(newReactions, reaction)
			} else {
				delete(rm.reactions, reaction.ID)
				cleanedCount++
			}
		}
		rm.streamReactions[streamID] = newReactions
	}

	// Clean up from userReactions
	for userID, reactions := range rm.userReactions {
		newReactions := make([]*Reaction, 0)
		for _, reaction := range reactions {
			if now.Sub(reaction.Timestamp) <= maxAge {
				newReactions = append(newReactions, reaction)
			}
		}
		rm.userReactions[userID] = newReactions
	}

	return cleanedCount
}

// GetTopReactors returns the top reactors for a stream
func (rm *ReactionManager) GetTopReactors(streamID string, duration time.Duration, limit int) []*ReactorEntry {
	stats := rm.GetReactionStats(streamID, duration)

	entries := make([]*ReactorEntry, 0, len(stats.ByUser))
	for userID, count := range stats.ByUser {
		entries = append(entries, &ReactorEntry{
			UserID: userID,
			Count:  count,
			Rank:   0, // Will be set after sorting
		})
	}

	// Sort by count (descending)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Count > entries[i].Count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Assign ranks and limit results
	if len(entries) > limit {
		entries = entries[:limit]
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}

	return entries
}

// ReactorEntry represents an entry in the top reactors list
type ReactorEntry struct {
	UserID string `json:"user_id"`
	Count  int    `json:"count"`
	Rank   int    `json:"rank"`
}

// generateReactionID generates a unique reaction ID
func generateReactionID() string {
	return fmt.Sprintf("reaction_%d", time.Now().UnixNano())
}
