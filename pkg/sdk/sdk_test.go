package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// Test State Machine

func TestStreamStateMachine(t *testing.T) {
	sm := NewStreamStateMachine()

	// Check initial state
	if sm.GetState() != StateIdle {
		t.Errorf("expected initial state to be Idle, got %s", sm.GetState())
	}

	// Test valid transitions
	validTransitions := []struct {
		from StreamState
		to   StreamState
	}{
		{StateIdle, StateLive},
		{StateLive, StatePaused},
		{StatePaused, StateLive},
		{StateLive, StateEnded},
	}

	for _, tt := range validTransitions {
		sm.Reset()
		sm.TransitionTo(tt.from)

		if err := sm.TransitionTo(tt.to); err != nil {
			t.Errorf("expected transition from %s to %s to succeed, got error: %v", tt.from, tt.to, err)
		}
	}
}

func TestStreamStateMachineInvalidTransitions(t *testing.T) {
	sm := NewStreamStateMachine()

	// Test invalid transitions
	invalidTransitions := []struct {
		from StreamState
		to   StreamState
	}{
		{StateEnded, StateLive},
		{StateEnded, StatePaused},
	}

	for _, tt := range invalidTransitions {
		sm.Reset()
		sm.TransitionTo(tt.from)

		if err := sm.TransitionTo(tt.to); err == nil {
			t.Errorf("expected transition from %s to %s to fail", tt.from, tt.to)
		}
	}
}

func TestStreamStateMachineHelpers(t *testing.T) {
	sm := NewStreamStateMachine()

	// Test IsIdle
	if !sm.IsIdle() {
		t.Error("expected IsIdle to return true")
	}

	// Test IsLive
	sm.TransitionTo(StateLive)
	if !sm.IsLive() {
		t.Error("expected IsLive to return true")
	}

	// Test IsPaused
	sm.TransitionTo(StatePaused)
	if !sm.IsPaused() {
		t.Error("expected IsPaused to return true")
	}

	// Test IsEnded
	sm.TransitionTo(StateEnded)
	if !sm.IsEnded() {
		t.Error("expected IsEnded to return true")
	}
}

// Test Stream Manager

func TestStreamManagerCreate(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	req := &CreateStreamRequest{
		UserID:      "user-123",
		Title:       "Test Stream",
		Description: "Test Description",
		Protocol:    ProtocolRTMP,
	}

	stream, err := manager.CreateStream(ctx, req)
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}

	if stream.ID == "" {
		t.Error("expected stream ID to be set")
	}

	if stream.StreamKey == "" {
		t.Error("expected stream key to be set")
	}

	if stream.State != StateIdle {
		t.Errorf("expected initial state to be Idle, got %s", stream.State)
	}

	if stream.UserID != req.UserID {
		t.Errorf("expected user ID %s, got %s", req.UserID, stream.UserID)
	}
}

func TestStreamManagerGet(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create stream
	req := &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Test Stream",
		Protocol: ProtocolRTMP,
	}

	created, _ := manager.CreateStream(ctx, req)

	// Get stream
	retrieved, err := manager.GetStream(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected stream ID %s, got %s", created.ID, retrieved.ID)
	}
}

func TestStreamManagerUpdate(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create stream
	stream, _ := manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Original Title",
		Protocol: ProtocolRTMP,
	})

	// Update stream
	newTitle := "Updated Title"
	newDesc := "Updated Description"

	updated, err := manager.UpdateStream(ctx, stream.ID, &UpdateStreamRequest{
		Title:       &newTitle,
		Description: &newDesc,
	})

	if err != nil {
		t.Fatalf("failed to update stream: %v", err)
	}

	if updated.Title != newTitle {
		t.Errorf("expected title %s, got %s", newTitle, updated.Title)
	}

	if updated.Description != newDesc {
		t.Errorf("expected description %s, got %s", newDesc, updated.Description)
	}
}

func TestStreamManagerDelete(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create stream
	stream, _ := manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Test Stream",
		Protocol: ProtocolRTMP,
	})

	// Delete stream
	err := manager.DeleteStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("failed to delete stream: %v", err)
	}

	// Verify deletion
	_, err = manager.GetStream(ctx, stream.ID)
	if err == nil {
		t.Error("expected error when getting deleted stream")
	}
}

func TestStreamManagerList(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create multiple streams
	for i := 0; i < 3; i++ {
		manager.CreateStream(ctx, &CreateStreamRequest{
			UserID:   "user-123",
			Title:    "Test Stream",
			Protocol: ProtocolRTMP,
		})
	}

	// List streams
	streams, err := manager.ListStreams(ctx)
	if err != nil {
		t.Fatalf("failed to list streams: %v", err)
	}

	if len(streams) != 3 {
		t.Errorf("expected 3 streams, got %d", len(streams))
	}
}

// Test Stream Controller

func TestStreamControllerStart(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)
	events := NewEventBus(log)
	controller := NewStreamController(manager, events, log)

	ctx := context.Background()

	// Create stream
	stream, _ := manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Test Stream",
		Protocol: ProtocolRTMP,
	})

	// Start stream
	err := controller.StartStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("failed to start stream: %v", err)
	}

	// Verify state
	updated, _ := manager.GetStream(ctx, stream.ID)
	if updated.State != StateLive {
		t.Errorf("expected state to be Live, got %s", updated.State)
	}

	if updated.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}

func TestStreamControllerStop(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)
	events := NewEventBus(log)
	controller := NewStreamController(manager, events, log)

	ctx := context.Background()

	// Create and start stream
	stream, _ := manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Test Stream",
		Protocol: ProtocolRTMP,
	})

	controller.StartStream(ctx, stream.ID)

	// Stop stream
	time.Sleep(100 * time.Millisecond) // Ensure some duration
	err := controller.StopStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("failed to stop stream: %v", err)
	}

	// Verify state
	updated, _ := manager.GetStream(ctx, stream.ID)
	if updated.State != StateEnded {
		t.Errorf("expected state to be Ended, got %s", updated.State)
	}

	if updated.EndedAt == nil {
		t.Error("expected EndedAt to be set")
	}

	if updated.TotalDuration == 0 {
		t.Error("expected TotalDuration to be greater than 0")
	}
}

func TestStreamControllerPauseResume(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)
	events := NewEventBus(log)
	controller := NewStreamController(manager, events, log)

	ctx := context.Background()

	// Create and start stream
	stream, _ := manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-123",
		Title:    "Test Stream",
		Protocol: ProtocolRTMP,
	})

	controller.StartStream(ctx, stream.ID)

	// Pause stream
	err := controller.PauseStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("failed to pause stream: %v", err)
	}

	updated, _ := manager.GetStream(ctx, stream.ID)
	if updated.State != StatePaused {
		t.Errorf("expected state to be Paused, got %s", updated.State)
	}

	// Resume stream
	err = controller.ResumeStream(ctx, stream.ID)
	if err != nil {
		t.Fatalf("failed to resume stream: %v", err)
	}

	updated, _ = manager.GetStream(ctx, stream.ID)
	if updated.State != StateLive {
		t.Errorf("expected state to be Live, got %s", updated.State)
	}
}

// Test Query Builder

func TestStreamQueryBuilder(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create test streams
	manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-1",
		Title:    "Stream A",
		Protocol: ProtocolRTMP,
	})

	manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-2",
		Title:    "Stream B",
		Protocol: ProtocolHLS,
	})

	manager.CreateStream(ctx, &CreateStreamRequest{
		UserID:   "user-1",
		Title:    "Stream C",
		Protocol: ProtocolWebRTC,
	})

	// Query by user
	query := NewStreamQueryBuilder().
		WithUserID("user-1").
		Build()

	result, err := manager.QueryStreams(ctx, query)
	if err != nil {
		t.Fatalf("failed to query streams: %v", err)
	}

	if result.TotalCount != 2 {
		t.Errorf("expected 2 streams for user-1, got %d", result.TotalCount)
	}

	// Query by protocol
	query = NewStreamQueryBuilder().
		WithProtocol(ProtocolRTMP).
		Build()

	result, err = manager.QueryStreams(ctx, query)
	if err != nil {
		t.Fatalf("failed to query streams: %v", err)
	}

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RTMP stream, got %d", result.TotalCount)
	}
}

func TestStreamQueryPagination(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := NewStreamManager(log)

	ctx := context.Background()

	// Create 10 test streams
	for i := 0; i < 10; i++ {
		manager.CreateStream(ctx, &CreateStreamRequest{
			UserID:   "user-1",
			Title:    "Test Stream",
			Protocol: ProtocolRTMP,
		})
	}

	// Query with pagination
	query := NewStreamQueryBuilder().
		Limit(5).
		Offset(0).
		Build()

	result, err := manager.QueryStreams(ctx, query)
	if err != nil {
		t.Fatalf("failed to query streams: %v", err)
	}

	if len(result.Streams) != 5 {
		t.Errorf("expected 5 streams in first page, got %d", len(result.Streams))
	}

	if result.TotalCount != 10 {
		t.Errorf("expected total count of 10, got %d", result.TotalCount)
	}

	// Get second page
	query.Offset = 5
	result, err = manager.QueryStreams(ctx, query)
	if err != nil {
		t.Fatalf("failed to query streams: %v", err)
	}

	if len(result.Streams) != 5 {
		t.Errorf("expected 5 streams in second page, got %d", len(result.Streams))
	}
}

// Test Event System

func TestEventBus(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	bus := NewEventBus(log)

	// Track event received
	eventReceived := false

	// Subscribe to event
	bus.Subscribe(EventStreamStart, func(event *StreamEvent) {
		eventReceived = true
	})

	// Publish event
	bus.Publish(&StreamEvent{
		Type:      EventStreamStart,
		StreamID:  "stream-123",
		Timestamp: time.Now(),
	})

	// Wait for async handler
	time.Sleep(100 * time.Millisecond)

	if !eventReceived {
		t.Error("expected event to be received")
	}
}

func TestEventCallbacks(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	bus := NewEventBus(log)

	startCount := 0
	endCount := 0

	callbacks := &EventCallbacks{
		OnStreamStart: func(event *StreamEvent) {
			startCount++
		},
		OnStreamEnd: func(event *StreamEvent) {
			endCount++
		},
	}

	bus.RegisterCallbacks(callbacks)

	bus.Publish(&StreamEvent{
		Type:      EventStreamStart,
		StreamID:  "stream-123",
		Timestamp: time.Now(),
	})

	bus.Publish(&StreamEvent{
		Type:      EventStreamEnd,
		StreamID:  "stream-123",
		Timestamp: time.Now(),
	})

	// Wait for async handlers
	time.Sleep(100 * time.Millisecond)

	if startCount != 1 {
		t.Errorf("expected 1 start event, got %d", startCount)
	}

	if endCount != 1 {
		t.Errorf("expected 1 end event, got %d", endCount)
	}
}

// Test Webhook Manager

func TestWebhookManager(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	bus := NewEventBus(log)
	manager := NewWebhookManager(bus, 2, log)

	defer manager.Stop()

	// Add webhook
	config := DefaultWebhookConfig("http://example.com/webhook")
	config.EventTypes = []EventType{EventStreamStart}

	err := manager.AddWebhook("webhook-1", config)
	if err != nil {
		t.Fatalf("failed to add webhook: %v", err)
	}

	// Verify webhook added
	retrieved, err := manager.GetWebhook("webhook-1")
	if err != nil {
		t.Fatalf("failed to get webhook: %v", err)
	}

	if retrieved.URL != config.URL {
		t.Errorf("expected URL %s, got %s", config.URL, retrieved.URL)
	}

	// Remove webhook
	err = manager.RemoveWebhook("webhook-1")
	if err != nil {
		t.Fatalf("failed to remove webhook: %v", err)
	}

	// Verify removal
	_, err = manager.GetWebhook("webhook-1")
	if err == nil {
		t.Error("expected error when getting removed webhook")
	}
}
