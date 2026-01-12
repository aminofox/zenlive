package room

import (
	"sync"
	"time"
)

// EventCallback is a function that handles room events
type EventCallback func(event *RoomEvent)

// EventBus manages event subscriptions and publishing
type EventBus struct {
	// subscribers stores callbacks by event type
	subscribers map[RoomEventType][]EventCallback
	// mu protects concurrent access
	mu sync.RWMutex
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[RoomEventType][]EventCallback),
	}
}

// Subscribe registers a callback for an event type
func (eb *EventBus) Subscribe(eventType RoomEventType, callback EventCallback) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], callback)
}

// SubscribeAll registers a callback for all event types
func (eb *EventBus) SubscribeAll(callback EventCallback) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Subscribe to all known event types
	eventTypes := []RoomEventType{
		EventRoomCreated,
		EventRoomDeleted,
		EventParticipantJoined,
		EventParticipantLeft,
		EventParticipantUpdated,
		EventTrackPublished,
		EventTrackUnpublished,
		EventMetadataUpdated,
	}

	for _, eventType := range eventTypes {
		eb.subscribers[eventType] = append(eb.subscribers[eventType], callback)
	}
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(event *RoomEvent) {
	eb.mu.RLock()
	callbacks := make([]EventCallback, len(eb.subscribers[event.Type]))
	copy(callbacks, eb.subscribers[event.Type])
	eb.mu.RUnlock()

	// Call callbacks asynchronously to avoid blocking
	for _, callback := range callbacks {
		go callback(event)
	}
}

// PublishSync publishes an event synchronously (blocking)
func (eb *EventBus) PublishSync(event *RoomEvent) {
	eb.mu.RLock()
	callbacks := make([]EventCallback, len(eb.subscribers[event.Type]))
	copy(callbacks, eb.subscribers[event.Type])
	eb.mu.RUnlock()

	for _, callback := range callbacks {
		callback(event)
	}
}

// Clear removes all subscribers
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers = make(map[RoomEventType][]EventCallback)
}

// createEvent is a helper to create a room event
func createEvent(eventType RoomEventType, roomID string, data interface{}) *RoomEvent {
	return &RoomEvent{
		Type:      eventType,
		RoomID:    roomID,
		Timestamp: time.Now(),
		Data:      data,
	}
}
