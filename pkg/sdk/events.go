package sdk

import (
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// EventType represents the type of stream event
type EventType string

const (
	// EventStreamStart is emitted when a stream starts
	EventStreamStart EventType = "stream.start"

	// EventStreamEnd is emitted when a stream ends
	EventStreamEnd EventType = "stream.end"

	// EventStreamPause is emitted when a stream is paused
	EventStreamPause EventType = "stream.pause"

	// EventStreamResume is emitted when a stream is resumed
	EventStreamResume EventType = "stream.resume"

	// EventStreamError is emitted when a stream encounters an error
	EventStreamError EventType = "stream.error"

	// EventStreamUpdate is emitted when a stream is updated
	EventStreamUpdate EventType = "stream.update"

	// EventStreamCreate is emitted when a stream is created
	EventStreamCreate EventType = "stream.create"

	// EventStreamDelete is emitted when a stream is deleted
	EventStreamDelete EventType = "stream.delete"

	// EventViewerJoin is emitted when a viewer joins
	EventViewerJoin EventType = "viewer.join"

	// EventViewerLeave is emitted when a viewer leaves
	EventViewerLeave EventType = "viewer.leave"
)

// StreamEvent represents an event that occurred on a stream
type StreamEvent struct {
	// Event type
	Type EventType `json:"type"`

	// Stream ID
	StreamID string `json:"stream_id"`

	// User ID who triggered the event
	UserID string `json:"user_id,omitempty"`

	// Event timestamp
	Timestamp time.Time `json:"timestamp"`

	// Event-specific data
	Data map[string]interface{} `json:"data,omitempty"`

	// Error message (if applicable)
	Error string `json:"error,omitempty"`
}

// EventHandler is a function that handles stream events
type EventHandler func(event *StreamEvent)

// EventSubscription represents a subscription to events
type EventSubscription struct {
	ID      string
	Type    EventType
	Handler EventHandler
}

// EventBus manages event subscriptions and publishing
type EventBus struct {
	subscriptions map[EventType][]*EventSubscription
	mu            sync.RWMutex
	logger        logger.Logger
}

// NewEventBus creates a new event bus
func NewEventBus(log logger.Logger) *EventBus {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &EventBus{
		subscriptions: make(map[EventType][]*EventSubscription),
		logger:        log,
	}
}

// Subscribe subscribes to events of a specific type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) *EventSubscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	subscription := &EventSubscription{
		ID:      generateSubscriptionID(),
		Type:    eventType,
		Handler: handler,
	}

	eb.subscriptions[eventType] = append(eb.subscriptions[eventType], subscription)

	eb.logger.Debug("Event subscription added",
		logger.Field{Key: "type", Value: eventType},
		logger.Field{Key: "subscription_id", Value: subscription.ID},
	)

	return subscription
}

// SubscribeAll subscribes to all event types
func (eb *EventBus) SubscribeAll(handler EventHandler) []*EventSubscription {
	eventTypes := []EventType{
		EventStreamStart,
		EventStreamEnd,
		EventStreamPause,
		EventStreamResume,
		EventStreamError,
		EventStreamUpdate,
		EventStreamCreate,
		EventStreamDelete,
		EventViewerJoin,
		EventViewerLeave,
	}

	subscriptions := make([]*EventSubscription, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		sub := eb.Subscribe(eventType, handler)
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions
}

// Unsubscribe removes a subscription
func (eb *EventBus) Unsubscribe(subscription *EventSubscription) {
	if subscription == nil {
		return
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	subs, exists := eb.subscriptions[subscription.Type]
	if !exists {
		return
	}

	// Find and remove subscription
	for i, sub := range subs {
		if sub.ID == subscription.ID {
			eb.subscriptions[subscription.Type] = append(subs[:i], subs[i+1:]...)
			eb.logger.Debug("Event subscription removed",
				logger.Field{Key: "type", Value: subscription.Type},
				logger.Field{Key: "subscription_id", Value: subscription.ID},
			)
			break
		}
	}
}

// UnsubscribeAll removes all subscriptions
func (eb *EventBus) UnsubscribeAll(subscriptions []*EventSubscription) {
	for _, sub := range subscriptions {
		eb.Unsubscribe(sub)
	}
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(event *StreamEvent) {
	if event == nil {
		return
	}

	eb.mu.RLock()
	subs, exists := eb.subscriptions[event.Type]
	if !exists || len(subs) == 0 {
		eb.mu.RUnlock()
		return
	}

	// Create a copy of subscriptions to avoid holding lock during handler execution
	handlers := make([]EventHandler, len(subs))
	for i, sub := range subs {
		handlers[i] = sub.Handler
	}
	eb.mu.RUnlock()

	// Execute handlers asynchronously
	for _, handler := range handlers {
		go func(h EventHandler, e *StreamEvent) {
			defer func() {
				if r := recover(); r != nil {
					eb.logger.Error("Event handler panic",
						logger.Field{Key: "type", Value: e.Type},
						logger.Field{Key: "error", Value: r},
					)
				}
			}()

			h(e)
		}(handler, event)
	}

	eb.logger.Debug("Event published",
		logger.Field{Key: "type", Value: event.Type},
		logger.Field{Key: "stream_id", Value: event.StreamID},
		logger.Field{Key: "subscribers", Value: len(handlers)},
	)
}

// GetSubscriberCount returns the number of subscribers for an event type
func (eb *EventBus) GetSubscriberCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.subscriptions[eventType])
}

// GetTotalSubscriberCount returns the total number of subscriptions
func (eb *EventBus) GetTotalSubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	total := 0
	for _, subs := range eb.subscriptions {
		total += len(subs)
	}

	return total
}

// Clear removes all subscriptions
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscriptions = make(map[EventType][]*EventSubscription)

	eb.logger.Info("All event subscriptions cleared")
}

// EventCallbacks provides a convenient way to register callbacks
type EventCallbacks struct {
	OnStreamStart  EventHandler
	OnStreamEnd    EventHandler
	OnStreamPause  EventHandler
	OnStreamResume EventHandler
	OnStreamError  EventHandler
	OnStreamUpdate EventHandler
	OnStreamCreate EventHandler
	OnStreamDelete EventHandler
	OnViewerJoin   EventHandler
	OnViewerLeave  EventHandler
}

// RegisterCallbacks registers all non-nil callbacks with the event bus
func (eb *EventBus) RegisterCallbacks(callbacks *EventCallbacks) []*EventSubscription {
	if callbacks == nil {
		return nil
	}

	subscriptions := make([]*EventSubscription, 0)

	if callbacks.OnStreamStart != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamStart, callbacks.OnStreamStart))
	}

	if callbacks.OnStreamEnd != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamEnd, callbacks.OnStreamEnd))
	}

	if callbacks.OnStreamPause != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamPause, callbacks.OnStreamPause))
	}

	if callbacks.OnStreamResume != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamResume, callbacks.OnStreamResume))
	}

	if callbacks.OnStreamError != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamError, callbacks.OnStreamError))
	}

	if callbacks.OnStreamUpdate != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamUpdate, callbacks.OnStreamUpdate))
	}

	if callbacks.OnStreamCreate != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamCreate, callbacks.OnStreamCreate))
	}

	if callbacks.OnStreamDelete != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventStreamDelete, callbacks.OnStreamDelete))
	}

	if callbacks.OnViewerJoin != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventViewerJoin, callbacks.OnViewerJoin))
	}

	if callbacks.OnViewerLeave != nil {
		subscriptions = append(subscriptions, eb.Subscribe(EventViewerLeave, callbacks.OnViewerLeave))
	}

	return subscriptions
}

// Helper function to generate subscription IDs
var subscriptionCounter int64
var subscriptionCounterMu sync.Mutex

func generateSubscriptionID() string {
	subscriptionCounterMu.Lock()
	defer subscriptionCounterMu.Unlock()

	subscriptionCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune(subscriptionCounter))
}
