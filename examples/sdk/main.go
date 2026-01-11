package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/sdk"
)

func main() {
	fmt.Println("=== ZenLive SDK Phase 7 Demo ===\n")

	// Run examples
	runStreamLifecycleExample()
	fmt.Println()

	runStreamControlExample()
	fmt.Println()

	runQueryExample()
	fmt.Println()

	runEventSystemExample()
	fmt.Println()

	runWebhookExample()
}

// Example 1: Stream Lifecycle Management
func runStreamLifecycleExample() {
	fmt.Println("--- Example 1: Stream Lifecycle ---")

	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Create stream manager
	manager := sdk.NewStreamManager(log)

	ctx := context.Background()

	// Create a stream
	createReq := &sdk.CreateStreamRequest{
		UserID:      "user-123",
		Title:       "My Gaming Stream",
		Description: "Playing my favorite game!",
		Protocol:    sdk.ProtocolRTMP,
		Metadata: map[string]string{
			"game":     "Minecraft",
			"language": "English",
		},
	}

	stream, err := manager.CreateStream(ctx, createReq)
	if err != nil {
		log.Fatal("Failed to create stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream created: %s\n", stream.ID)
	fmt.Printf("  Title: %s\n", stream.Title)
	fmt.Printf("  Stream Key: %s\n", stream.StreamKey)
	fmt.Printf("  Protocol: %s\n", stream.Protocol)
	fmt.Printf("  State: %s\n", stream.State)

	// Update stream
	newTitle := "Epic Gaming Session"
	updateReq := &sdk.UpdateStreamRequest{
		Title: &newTitle,
	}

	updated, err := manager.UpdateStream(ctx, stream.ID, updateReq)
	if err != nil {
		log.Fatal("Failed to update stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream updated: %s\n", updated.Title)

	// Get stream info
	retrieved, err := manager.GetStream(ctx, stream.ID)
	if err != nil {
		log.Fatal("Failed to get stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream retrieved: %s\n", retrieved.ID)

	// List all streams
	streams, err := manager.ListStreams(ctx)
	if err != nil {
		log.Fatal("Failed to list streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Total streams: %d\n", len(streams))
}

// Example 2: Stream Control Operations
func runStreamControlExample() {
	fmt.Println("--- Example 2: Stream Control ---")

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Setup
	manager := sdk.NewStreamManager(log)
	events := sdk.NewEventBus(log)
	controller := sdk.NewStreamController(manager, events, log)

	ctx := context.Background()

	// Create stream
	stream, _ := manager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user-456",
		Title:    "Live Concert",
		Protocol: sdk.ProtocolWebRTC,
	})

	fmt.Printf("✓ Stream created: %s\n", stream.ID)

	// Start stream
	err := controller.StartStream(ctx, stream.ID)
	if err != nil {
		log.Fatal("Failed to start stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream started at: %s\n", stream.StartedAt.Format(time.RFC3339))

	// Get stream status
	status, _ := controller.GetStreamStatus(ctx, stream.ID)
	fmt.Printf("✓ Stream status:\n")
	fmt.Printf("  State: %s\n", status.State)
	fmt.Printf("  Is Live: %v\n", status.IsLive)
	fmt.Printf("  Viewers: %d\n", status.ViewerCount)

	// Simulate viewers joining
	stream.IncrementViewerCount()
	stream.IncrementViewerCount()
	stream.IncrementViewerCount()
	fmt.Printf("✓ Viewers joined: %d\n", stream.GetViewerCount())

	// Pause stream
	time.Sleep(500 * time.Millisecond)
	err = controller.PauseStream(ctx, stream.ID)
	if err != nil {
		log.Fatal("Failed to pause stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream paused\n")

	// Resume stream
	time.Sleep(200 * time.Millisecond)
	err = controller.ResumeStream(ctx, stream.ID)
	if err != nil {
		log.Fatal("Failed to resume stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream resumed\n")

	// Stop stream
	time.Sleep(500 * time.Millisecond)
	err = controller.StopStream(ctx, stream.ID)
	if err != nil {
		log.Fatal("Failed to stop stream",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Stream stopped\n")
	fmt.Printf("  Duration: %s\n", stream.GetDuration())
	fmt.Printf("  Final viewers: %d\n", stream.ViewerCount)
}

// Example 3: Query and Filtering
func runQueryExample() {
	fmt.Println("--- Example 3: Stream Queries ---")

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := sdk.NewStreamManager(log)
	events := sdk.NewEventBus(log)
	controller := sdk.NewStreamController(manager, events, log)

	ctx := context.Background()

	// Create multiple test streams
	users := []string{"user-1", "user-2", "user-1"}
	protocols := []sdk.StreamProtocol{sdk.ProtocolRTMP, sdk.ProtocolHLS, sdk.ProtocolWebRTC}
	titles := []string{"Gaming Stream", "Music Concert", "Coding Session"}

	for i := 0; i < 3; i++ {
		stream, _ := manager.CreateStream(ctx, &sdk.CreateStreamRequest{
			UserID:   users[i],
			Title:    titles[i],
			Protocol: protocols[i],
		})

		// Start some streams
		if i < 2 {
			controller.StartStream(ctx, stream.ID)

			// Simulate viewers
			for j := 0; j < (i+1)*5; j++ {
				stream.IncrementViewerCount()
			}
		}
	}

	// Query by user
	query := sdk.NewStreamQueryBuilder().
		WithUserID("user-1").
		Build()

	result, err := manager.QueryStreams(ctx, query)
	if err != nil {
		log.Fatal("Failed to query streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Streams for user-1: %d\n", result.TotalCount)

	// Query live streams
	query = sdk.NewStreamQueryBuilder().
		WithState(sdk.StateLive).
		SortBy("viewer_count").
		SortOrder("desc").
		Build()

	result, err = manager.QueryStreams(ctx, query)
	if err != nil {
		log.Fatal("Failed to query live streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Live streams: %d\n", result.TotalCount)
	for _, s := range result.Streams {
		fmt.Printf("  - %s (%d viewers)\n", s.Title, s.ViewerCount)
	}

	// Search streams
	streams, err := manager.SearchStreams(ctx, "Stream", 10)
	if err != nil {
		log.Fatal("Failed to search streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Search results for 'Stream': %d\n", len(streams))

	// Get popular streams
	popular, err := manager.GetPopularStreams(ctx, 5)
	if err != nil {
		log.Fatal("Failed to get popular streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Popular streams: %d\n", len(popular))

	// Pagination example
	query = sdk.NewStreamQueryBuilder().
		Limit(2).
		Offset(0).
		SortBy("created_at").
		SortOrder("desc").
		Build()

	result, err = manager.QueryStreams(ctx, query)
	if err != nil {
		log.Fatal("Failed to paginate streams",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Page 1 (limit 2): %d streams, total: %d\n", len(result.Streams), result.TotalCount)
}

// Example 4: Event System
func runEventSystemExample() {
	fmt.Println("--- Example 4: Event System ---")

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := sdk.NewStreamManager(log)
	events := sdk.NewEventBus(log)
	controller := sdk.NewStreamController(manager, events, log)

	ctx := context.Background()

	// Track events
	eventCount := 0

	// Subscribe to events
	callbacks := &sdk.EventCallbacks{
		OnStreamStart: func(event *sdk.StreamEvent) {
			fmt.Printf("✓ Event: Stream Started - %s\n", event.StreamID)
			eventCount++
		},
		OnStreamEnd: func(event *sdk.StreamEvent) {
			fmt.Printf("✓ Event: Stream Ended - %s (duration: %.0fs)\n",
				event.StreamID, event.Data["duration"])
			eventCount++
		},
		OnStreamPause: func(event *sdk.StreamEvent) {
			fmt.Printf("✓ Event: Stream Paused - %s\n", event.StreamID)
			eventCount++
		},
		OnStreamResume: func(event *sdk.StreamEvent) {
			fmt.Printf("✓ Event: Stream Resumed - %s\n", event.StreamID)
			eventCount++
		},
		OnStreamError: func(event *sdk.StreamEvent) {
			fmt.Printf("✗ Event: Stream Error - %s: %s\n", event.StreamID, event.Error)
			eventCount++
		},
	}

	subscriptions := events.RegisterCallbacks(callbacks)
	defer events.UnsubscribeAll(subscriptions)

	// Create and control stream
	stream, _ := manager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user-789",
		Title:    "Event Demo Stream",
		Protocol: sdk.ProtocolRTMP,
	})

	// Perform operations that trigger events
	controller.StartStream(ctx, stream.ID)
	time.Sleep(100 * time.Millisecond)

	controller.PauseStream(ctx, stream.ID)
	time.Sleep(100 * time.Millisecond)

	controller.ResumeStream(ctx, stream.ID)
	time.Sleep(100 * time.Millisecond)

	controller.StopStream(ctx, stream.ID)
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("✓ Total events received: %d\n", eventCount)
	fmt.Printf("✓ Event subscribers: %d\n", events.GetTotalSubscriberCount())
}

// Example 5: Webhook System
func runWebhookExample() {
	fmt.Println("--- Example 5: Webhook System ---")

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	manager := sdk.NewStreamManager(log)
	events := sdk.NewEventBus(log)
	webhookManager := sdk.NewWebhookManager(events, 2, log)

	defer webhookManager.Stop()

	// Add webhook (note: this won't actually send to the URL in this example)
	config := sdk.DefaultWebhookConfig("https://example.com/webhook")
	config.EventTypes = []sdk.EventType{
		sdk.EventStreamStart,
		sdk.EventStreamEnd,
	}
	config.MaxRetries = 3
	config.RetryDelay = 1 * time.Second
	config.Headers["X-Custom-Header"] = "ZenLive"

	err := webhookManager.AddWebhook("webhook-1", config)
	if err != nil {
		log.Fatal("Failed to add webhook",
			logger.Field{Key: "error", Value: err},
		)
	}

	fmt.Printf("✓ Webhook added: webhook-1\n")
	fmt.Printf("  URL: %s\n", config.URL)
	fmt.Printf("  Event types: %v\n", config.EventTypes)
	fmt.Printf("  Max retries: %d\n", config.MaxRetries)

	// List webhooks
	webhooks := webhookManager.ListWebhooks()
	fmt.Printf("✓ Total webhooks: %d\n", len(webhooks))

	// Trigger events (webhooks will be queued but won't actually send)
	controller := sdk.NewStreamController(manager, events, log)
	ctx := context.Background()

	stream, _ := manager.CreateStream(ctx, &sdk.CreateStreamRequest{
		UserID:   "user-999",
		Title:    "Webhook Demo",
		Protocol: sdk.ProtocolRTMP,
	})

	// This will trigger webhook delivery attempt
	controller.StartStream(ctx, stream.ID)
	time.Sleep(200 * time.Millisecond)

	controller.StopStream(ctx, stream.ID)
	time.Sleep(200 * time.Millisecond)

	fmt.Printf("✓ Webhooks triggered for stream lifecycle events\n")
	fmt.Printf("✓ Check webhook delivery queue for processing\n")

	// Remove webhook
	webhookManager.RemoveWebhook("webhook-1")
	fmt.Printf("✓ Webhook removed\n")
}
