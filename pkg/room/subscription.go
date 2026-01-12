package room

import (
	"sync"
	"time"
)

// QualityLevel represents the quality level for adaptive bitrate
type QualityLevel string

const (
	// QualityHigh represents high quality (1080p)
	QualityHigh QualityLevel = "high"

	// QualityMedium represents medium quality (720p)
	QualityMedium QualityLevel = "medium"

	// QualityLow represents low quality (360p)
	QualityLow QualityLevel = "low"

	// QualityAuto represents automatic quality selection
	QualityAuto QualityLevel = "auto"
)

// SubscriptionState represents the state of a subscription
type SubscriptionState string

const (
	// SubscriptionStateSubscribing indicates subscription is being established
	SubscriptionStateSubscribing SubscriptionState = "subscribing"

	// SubscriptionStateSubscribed indicates subscription is active
	SubscriptionStateSubscribed SubscriptionState = "subscribed"

	// SubscriptionStateUnsubscribing indicates subscription is being removed
	SubscriptionStateUnsubscribing SubscriptionState = "unsubscribing"

	// SubscriptionStateUnsubscribed indicates subscription is removed
	SubscriptionStateUnsubscribed SubscriptionState = "unsubscribed"

	// SubscriptionStateFailed indicates subscription failed
	SubscriptionStateFailed SubscriptionState = "failed"
)

// Subscription represents a participant's subscription to another participant's track
type Subscription struct {
	// SubscriberID is the ID of the subscribing participant
	SubscriberID string `json:"subscriber_id"`

	// PublisherID is the ID of the publishing participant
	PublisherID string `json:"publisher_id"`

	// TrackID is the ID of the subscribed track
	TrackID string `json:"track_id"`

	// Quality is the requested quality level
	Quality QualityLevel `json:"quality"`

	// State is the current state of the subscription
	State SubscriptionState `json:"state"`

	// CreatedAt is when the subscription was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the subscription was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata contains custom subscription data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// mu protects concurrent access
	mu sync.RWMutex
}

// NewSubscription creates a new subscription
func NewSubscription(subscriberID, publisherID, trackID string, quality QualityLevel) *Subscription {
	now := time.Now()
	return &Subscription{
		SubscriberID: subscriberID,
		PublisherID:  publisherID,
		TrackID:      trackID,
		Quality:      quality,
		State:        SubscriptionStateSubscribing,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     make(map[string]interface{}),
	}
}

// UpdateState updates the subscription state
func (s *Subscription) UpdateState(state SubscriptionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State = state
	s.UpdatedAt = time.Now()
}

// UpdateQuality updates the requested quality level
func (s *Subscription) UpdateQuality(quality QualityLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Quality = quality
	s.UpdatedAt = time.Now()
}

// GetState returns the current state
func (s *Subscription) GetState() SubscriptionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// GetQuality returns the current quality level
func (s *Subscription) GetQuality() QualityLevel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Quality
}

// SimulcastLayer represents a simulcast quality layer
type SimulcastLayer struct {
	// Quality is the quality level identifier
	Quality QualityLevel `json:"quality"`

	// MaxWidth is the maximum width in pixels
	MaxWidth int `json:"max_width"`

	// MaxHeight is the maximum height in pixels
	MaxHeight int `json:"max_height"`

	// MaxBitrate is the maximum bitrate in bps
	MaxBitrate int `json:"max_bitrate"`

	// MaxFramerate is the maximum framerate
	MaxFramerate int `json:"max_framerate,omitempty"`
}

// SimulcastConfig contains simulcast configuration
type SimulcastConfig struct {
	// Enabled indicates if simulcast is enabled
	Enabled bool `json:"enabled"`

	// Layers defines the available simulcast layers
	Layers []SimulcastLayer `json:"layers"`
}

// DefaultSimulcastConfig returns the default simulcast configuration
func DefaultSimulcastConfig() SimulcastConfig {
	return SimulcastConfig{
		Enabled: true,
		Layers: []SimulcastLayer{
			{
				Quality:      QualityHigh,
				MaxWidth:     1920,
				MaxHeight:    1080,
				MaxBitrate:   3_000_000, // 3 Mbps
				MaxFramerate: 30,
			},
			{
				Quality:      QualityMedium,
				MaxWidth:     1280,
				MaxHeight:    720,
				MaxBitrate:   1_500_000, // 1.5 Mbps
				MaxFramerate: 30,
			},
			{
				Quality:      QualityLow,
				MaxWidth:     640,
				MaxHeight:    360,
				MaxBitrate:   500_000, // 500 Kbps
				MaxFramerate: 24,
			},
		},
	}
}

// SubscriptionManager manages all subscriptions in a room
type SubscriptionManager struct {
	// subscriptions maps subscriber ID to their subscriptions
	// subscriberID -> trackID -> Subscription
	subscriptions map[string]map[string]*Subscription

	// simulcastConfig is the simulcast configuration for the room
	simulcastConfig SimulcastConfig

	// mu protects concurrent access
	mu sync.RWMutex
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(config SimulcastConfig) *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions:   make(map[string]map[string]*Subscription),
		simulcastConfig: config,
	}
}

// Subscribe creates a new subscription
func (sm *SubscriptionManager) Subscribe(subscriberID, publisherID, trackID string, quality QualityLevel) (*Subscription, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Initialize subscriber's subscriptions map if needed
	if sm.subscriptions[subscriberID] == nil {
		sm.subscriptions[subscriberID] = make(map[string]*Subscription)
	}

	// Check if already subscribed to this track
	if sub, exists := sm.subscriptions[subscriberID][trackID]; exists {
		// Update existing subscription
		sub.UpdateQuality(quality)
		sub.UpdateState(SubscriptionStateSubscribed)
		return sub, nil
	}

	// Create new subscription
	sub := NewSubscription(subscriberID, publisherID, trackID, quality)
	sm.subscriptions[subscriberID][trackID] = sub

	return sub, nil
}

// Unsubscribe removes a subscription
func (sm *SubscriptionManager) Unsubscribe(subscriberID, trackID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs, exists := sm.subscriptions[subscriberID]
	if !exists {
		return nil // Already unsubscribed
	}

	if sub, exists := subs[trackID]; exists {
		sub.UpdateState(SubscriptionStateUnsubscribed)
		delete(subs, trackID)
	}

	// Clean up empty map
	if len(subs) == 0 {
		delete(sm.subscriptions, subscriberID)
	}

	return nil
}

// GetSubscription gets a specific subscription
func (sm *SubscriptionManager) GetSubscription(subscriberID, trackID string) (*Subscription, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs, exists := sm.subscriptions[subscriberID]
	if !exists {
		return nil, false
	}

	sub, exists := subs[trackID]
	return sub, exists
}

// GetSubscriberSubscriptions gets all subscriptions for a subscriber
func (sm *SubscriptionManager) GetSubscriberSubscriptions(subscriberID string) []*Subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs, exists := sm.subscriptions[subscriberID]
	if !exists {
		return nil
	}

	result := make([]*Subscription, 0, len(subs))
	for _, sub := range subs {
		result = append(result, sub)
	}

	return result
}

// UnsubscribeAll removes all subscriptions for a subscriber
func (sm *SubscriptionManager) UnsubscribeAll(subscriberID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if subs, exists := sm.subscriptions[subscriberID]; exists {
		for _, sub := range subs {
			sub.UpdateState(SubscriptionStateUnsubscribed)
		}
		delete(sm.subscriptions, subscriberID)
	}
}

// SelectLayer selects the appropriate simulcast layer based on available bandwidth
func (sm *SubscriptionManager) SelectLayer(availableBandwidthBps int) QualityLevel {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.simulcastConfig.Enabled {
		return QualityHigh
	}

	// Select highest layer that fits within bandwidth
	selectedQuality := QualityLow

	for _, layer := range sm.simulcastConfig.Layers {
		if layer.MaxBitrate <= availableBandwidthBps {
			selectedQuality = layer.Quality
			break
		}
	}

	return selectedQuality
}

// UpdateSubscriptionQuality updates the quality for a subscription based on bandwidth
func (sm *SubscriptionManager) UpdateSubscriptionQuality(subscriberID, trackID string, availableBandwidth int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs, exists := sm.subscriptions[subscriberID]
	if !exists {
		return nil
	}

	sub, exists := subs[trackID]
	if !exists {
		return nil
	}

	// Only update if quality is set to auto
	if sub.GetQuality() != QualityAuto {
		return nil
	}

	// Select appropriate layer
	newQuality := sm.SelectLayer(availableBandwidth)
	sub.UpdateQuality(newQuality)

	return nil
}

// GetSimulcastConfig returns the simulcast configuration
func (sm *SubscriptionManager) GetSimulcastConfig() SimulcastConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.simulcastConfig
}

// UpdateSimulcastConfig updates the simulcast configuration
func (sm *SubscriptionManager) UpdateSimulcastConfig(config SimulcastConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.simulcastConfig = config
}
