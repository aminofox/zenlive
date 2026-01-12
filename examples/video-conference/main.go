// Example: Complete Video Conference Application
//
// This example demonstrates a full video conferencing application
// using ZenLive SDK with REST API, WebSocket signaling, and room management.
//
// Features:
// - Complete HTTP REST API server
// - WebSocket signaling server
// - Room management (create, join, leave)
// - Participant tracking
// - Media track publishing/unpublishing
// - Data messaging
// - Event callbacks
// - Health monitoring
//
// Usage:
//   go run main.go
//
// Then connect clients to:
//   HTTP API: http://localhost:8080/api
//   WebSocket: ws://localhost:8080/ws

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive"
	"github.com/aminofox/zenlive/pkg/api"
	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/config"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

func main() {
	fmt.Println("=== ZenLive Video Conference Server ===\n")

	// Create configuration
	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	// Create ZenLive SDK
	sdk, err := zenlive.New(cfg)
	if err != nil {
		log.Fatal("Failed to create SDK:", err)
	}

	// Register event callbacks
	registerEventCallbacks(sdk)

	// Start SDK
	if err := sdk.Start(); err != nil {
		log.Fatal("Failed to start SDK:", err)
	}
	fmt.Println("✓ ZenLive SDK started")

	// Create some demo rooms
	createDemoRooms(sdk)

	// Start API server
	apiConfig := &api.Config{
		Addr:         ":8080",
		JWTSecret:    "demo-secret-change-in-production",
		RateLimitRPM: 100,
		CORSOrigins:  []string{"*"},
		CORSMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSHeaders:  []string{"Content-Type", "Authorization"},
	}

	// Create dummy JWT authenticator (in production, use real user/token stores)
	jwtAuth := createDummyJWTAuth(apiConfig.JWTSecret, sdk.Logger())

	apiServer := api.NewServer(sdk.GetRoomManager(), jwtAuth, apiConfig, sdk.Logger())

	// Start API server in background
	go func() {
		fmt.Printf("✓ API server starting on %s\n", apiConfig.Addr)
		fmt.Println("\nEndpoints:")
		fmt.Println("  HTTP REST API: http://localhost:8080/api")
		fmt.Println("  WebSocket:     ws://localhost:8080/ws")
		fmt.Println("  Health Check:  http://localhost:8080/api/health")
		fmt.Println("\nPress Ctrl+C to stop\n")

		if err := apiServer.Start(); err != nil {
			log.Fatal("Failed to start API server:", err)
		}
	}()

	// Wait for demo period
	time.Sleep(2 * time.Second)

	// Print server statistics
	printStatistics(sdk)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\nShutting down...")

	// Stop SDK
	if err := sdk.Stop(); err != nil {
		log.Printf("Error stopping SDK: %v", err)
	}

	fmt.Println("✓ Server stopped gracefully")
}

// registerEventCallbacks registers event callbacks
func registerEventCallbacks(sdk *zenlive.SDK) {
	// Room created callback
	sdk.OnRoomCreated(func(rm *room.Room) {
		fmt.Printf("📢 Room created: %s (ID: %s)\n", rm.Name, rm.ID)
	})

	// Room deleted callback
	sdk.OnRoomDeleted(func(roomID string) {
		fmt.Printf("📢 Room deleted: %s\n", roomID)
	})

	// Participant joined callback
	sdk.OnParticipantJoined(func(roomID string, participant *room.Participant) {
		fmt.Printf("📢 Participant joined room %s: %s (ID: %s)\n",
			roomID, participant.Username, participant.ID)
	})

	// Participant left callback
	sdk.OnParticipantLeft(func(roomID string, participantID string) {
		fmt.Printf("📢 Participant left room %s: %s\n", roomID, participantID)
	})

	// Track published callback
	sdk.OnTrackPublished(func(roomID string, track *room.MediaTrack) {
		fmt.Printf("📢 Track published in room %s: %s (%s)\n",
			roomID, track.ID, track.Kind)
	})

	// Track unpublished callback
	sdk.OnTrackUnpublished(func(roomID string, trackID string) {
		fmt.Printf("📢 Track unpublished in room %s: %s\n", roomID, trackID)
	})

	// Metadata updated callback
	sdk.OnMetadataUpdated(func(roomID string, metadata map[string]interface{}) {
		fmt.Printf("📢 Metadata updated in room %s\n", roomID)
	})

	fmt.Println("✓ Event callbacks registered")
}

// createDemoRooms creates some demo rooms
func createDemoRooms(sdk *zenlive.SDK) {
	// Create demo room 1
	room1, err := sdk.CreateRoom("Team Meeting", &room.CreateRoomRequest{
		MaxParticipants: 10,
		EmptyTimeout:    30 * time.Minute,
		Metadata: map[string]interface{}{
			"description": "Daily team standup",
			"scheduled":   true,
		},
	})
	if err != nil {
		log.Printf("Failed to create demo room 1: %v", err)
	} else {
		fmt.Printf("✓ Created demo room: %s (ID: %s)\n", room1.Name, room1.ID)
	}

	// Create demo room 2
	room2, err := sdk.CreateRoom("Product Demo", &room.CreateRoomRequest{
		MaxParticipants: 50,
		EmptyTimeout:    1 * time.Hour,
		Metadata: map[string]interface{}{
			"description": "Product demonstration for clients",
			"public":      true,
		},
	})
	if err != nil {
		log.Printf("Failed to create demo room 2: %v", err)
	} else {
		fmt.Printf("✓ Created demo room: %s (ID: %s)\n", room2.Name, room2.ID)
	}

	// Create demo room 3
	room3, err := sdk.CreateRoom("Webinar Hall", &room.CreateRoomRequest{
		MaxParticipants: 100,
		Metadata: map[string]interface{}{
			"description": "Large webinar room",
			"type":        "webinar",
		},
	})
	if err != nil {
		log.Printf("Failed to create demo room 3: %v", err)
	} else {
		fmt.Printf("✓ Created demo room: %s (ID: %s)\n", room3.Name, room3.ID)
	}
}

// printStatistics prints server statistics
func printStatistics(sdk *zenlive.SDK) {
	rooms := sdk.ListRooms()
	totalParticipants := 0

	fmt.Println("\n=== Server Statistics ===")
	fmt.Printf("Active rooms: %d\n", len(rooms))

	for _, rm := range rooms {
		participantCount := rm.GetParticipantCount()
		totalParticipants += participantCount
		fmt.Printf("  - %s: %d participants\n", rm.Name, participantCount)
	}

	fmt.Printf("Total participants: %d\n", totalParticipants)
	fmt.Println("=========================\n")
}

// createDummyJWTAuth creates a dummy JWT authenticator for demo
// In production, use real user/token stores
func createDummyJWTAuth(secret string, log logger.Logger) *auth.JWTAuthenticator {
	// Create in-memory stores
	userStore := auth.NewInMemoryUserStore()
	tokenStore := auth.NewInMemoryTokenStore()

	// Create JWT authenticator
	jwtAuth := auth.NewJWTAuthenticator(secret, userStore, tokenStore)

	return jwtAuth
}
