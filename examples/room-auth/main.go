package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== ZenLive Complete Room Join Flow with Token Authentication ===\n")

	// Step 1: Setup API Key Management (one-time setup)
	fmt.Println("Step 1: Setting up API Key Management...")
	apiKeyStore := auth.NewMemoryAPIKeyStore()
	keyManager := auth.NewAPIKeyManager(apiKeyStore)

	// Generate API key pair
	expiresIn := 365 * 24 * time.Hour
	apiKeyPair, err := keyManager.GenerateAPIKey(ctx, "Production Key", &expiresIn, nil)
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	fmt.Printf("✓ API Key: %s\n", apiKeyPair.AccessKey)
	fmt.Printf("✓ Secret: %s\n", apiKeyPair.SecretKey)
	fmt.Println()

	// Step 2: Setup Room Manager with Authentication
	fmt.Println("Step 2: Setting up Room Manager with Authentication...")
	logr := logger.NewDefaultLogger(logger.InfoLevel, "text")
	authenticator := room.NewRoomAuthenticator(keyManager, logr)
	roomManager := room.NewAuthenticatedRoomManager(authenticator, apiKeyPair.SecretKey, logr)
	fmt.Println("✓ Room manager initialized\n")

	// Step 3: Backend creates access token for User 1 (Publisher)
	fmt.Println("Step 3: Backend creates access token for User 1 (Publisher)...")
	publisherToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_001").
		SetName("Alice (Publisher)").
		SetEmail("alice@example.com").
		SetRoomJoin("livestream-demo").
		SetCanPublish(true).     // Can stream video/audio
		SetCanSubscribe(false).  // Cannot watch others (pure publisher)
		SetCanPublishData(true). // Can send chat messages
		SetTTL(6 * time.Hour).
		SetMetadata(map[string]interface{}{
			"user_type": "streamer",
			"plan":      "premium",
		}).
		Build()
	if err != nil {
		log.Fatalf("Failed to create publisher token: %v", err)
	}
	fmt.Printf("✓ Publisher token: %s...\n\n", publisherToken[:40])

	// Step 4: Backend creates access token for User 2 (Viewer)
	fmt.Println("Step 4: Backend creates access token for User 2 (Viewer)...")
	viewerToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_002").
		SetName("Bob (Viewer)").
		SetEmail("bob@example.com").
		SetRoomJoin("livestream-demo").
		SetCanPublish(false).    // Cannot stream
		SetCanSubscribe(true).   // Can watch streams
		SetCanPublishData(true). // Can send chat messages
		SetTTL(6 * time.Hour).
		Build()
	if err != nil {
		log.Fatalf("Failed to create viewer token: %v", err)
	}
	fmt.Printf("✓ Viewer token: %s...\n\n", viewerToken[:40])

	// Step 5: Backend creates access token for Admin
	fmt.Println("Step 5: Backend creates access token for Admin...")
	adminToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("admin_001").
		SetName("Admin User").
		SetRoomJoin("livestream-demo").
		SetRoomAdmin(true).  // Admin privileges
		SetRoomCreate(true). // Can create rooms
		SetCanPublish(true).
		SetCanSubscribe(true).
		SetCanPublishData(true).
		SetTTL(24 * time.Hour).
		Build()
	if err != nil {
		log.Fatalf("Failed to create admin token: %v", err)
	}
	fmt.Printf("✓ Admin token: %s...\n\n", adminToken[:40])

	// Step 6: Publisher joins the room with token
	fmt.Println("Step 6: Publisher (Alice) joins the room...")
	publisherJoinReq := &room.JoinRoomRequest{
		RoomName:    "livestream-demo",
		AccessToken: publisherToken,
		Metadata: map[string]interface{}{
			"device":     "iPhone 14 Pro",
			"connection": "WiFi",
		},
	}

	publisher, publisherRoom, err := roomManager.JoinRoomWithToken(ctx, publisherJoinReq)
	if err != nil {
		log.Fatalf("Publisher failed to join: %v", err)
	}
	fmt.Printf("✓ Publisher joined room: %s\n", publisherRoom.Name)
	fmt.Printf("  - User ID: %s\n", publisher.ID)
	fmt.Printf("  - Username: %s\n", publisher.Username)
	fmt.Printf("  - Can Publish: %v\n", publisher.CanPublish)
	fmt.Printf("  - Can Subscribe: %v\n", publisher.CanSubscribe)
	fmt.Printf("  - Is Admin: %v\n", publisher.IsAdmin)
	fmt.Println()

	// Step 7: Viewer joins the room with token
	fmt.Println("Step 7: Viewer (Bob) joins the room...")
	viewerJoinReq := &room.JoinRoomRequest{
		RoomName:    "livestream-demo",
		AccessToken: viewerToken,
		Metadata: map[string]interface{}{
			"device":     "Chrome Browser",
			"connection": "4G",
		},
	}

	viewer, viewerRoom, err := roomManager.JoinRoomWithToken(ctx, viewerJoinReq)
	if err != nil {
		log.Fatalf("Viewer failed to join: %v", err)
	}
	fmt.Printf("✓ Viewer joined room: %s\n", viewerRoom.Name)
	fmt.Printf("  - User ID: %s\n", viewer.ID)
	fmt.Printf("  - Username: %s\n", viewer.Username)
	fmt.Printf("  - Can Publish: %v\n", viewer.CanPublish)
	fmt.Printf("  - Can Subscribe: %v\n", viewer.CanSubscribe)
	fmt.Println()

	// Step 8: Check room status
	fmt.Println("Step 8: Checking room status...")
	roomInfo, err := roomManager.GetRoomByName("livestream-demo")
	if err != nil {
		log.Fatalf("Failed to get room: %v", err)
	}
	participants := roomInfo.ListParticipants()
	fmt.Printf("✓ Room: %s\n", roomInfo.Name)
	fmt.Printf("  - Total participants: %d\n", len(participants))
	for i, p := range participants {
		fmt.Printf("  - [%d] %s (%s)\n", i+1, p.Username, p.ID)
	}
	fmt.Println()

	// Step 9: Validate permissions
	fmt.Println("Step 9: Validating permissions...")

	// Publisher should be able to publish
	if err := roomManager.ValidateParticipantAction("livestream-demo", publisher.ID, "publish"); err != nil {
		fmt.Printf("✗ Publisher cannot publish: %v\n", err)
	} else {
		fmt.Println("✓ Publisher can publish streams")
	}

	// Viewer should NOT be able to publish
	if err := roomManager.ValidateParticipantAction("livestream-demo", viewer.ID, "publish"); err != nil {
		fmt.Printf("✓ Viewer cannot publish (as expected): %v\n", err)
	} else {
		fmt.Println("✗ Viewer should not be able to publish!")
	}

	// Viewer should be able to subscribe
	if err := roomManager.ValidateParticipantAction("livestream-demo", viewer.ID, "subscribe"); err != nil {
		fmt.Printf("✗ Viewer cannot subscribe: %v\n", err)
	} else {
		fmt.Println("✓ Viewer can subscribe to streams")
	}
	fmt.Println()

	// Step 10: Test invalid token
	fmt.Println("Step 10: Testing invalid token...")
	invalidJoinReq := &room.JoinRoomRequest{
		RoomName:    "livestream-demo",
		AccessToken: "invalid.token.here",
	}
	_, _, err = roomManager.JoinRoomWithToken(ctx, invalidJoinReq)
	if err != nil {
		fmt.Printf("✓ Invalid token rejected (as expected): %v\n", err)
	} else {
		fmt.Println("✗ Invalid token should have been rejected!")
	}
	fmt.Println()

	// Step 11: Test wrong room token
	fmt.Println("Step 11: Testing token for wrong room...")
	wrongRoomToken, _ := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_003").
		SetName("Charlie").
		SetRoomJoin("other-room"). // Different room
		SetCanPublish(true).
		SetCanSubscribe(true).
		SetTTL(1 * time.Hour).
		Build()

	wrongRoomReq := &room.JoinRoomRequest{
		RoomName:    "livestream-demo", // Trying to join different room
		AccessToken: wrongRoomToken,
	}
	_, _, err = roomManager.JoinRoomWithToken(ctx, wrongRoomReq)
	if err != nil {
		fmt.Printf("✓ Wrong room token rejected (as expected): %v\n", err)
	} else {
		fmt.Println("✗ Wrong room token should have been rejected!")
	}
	fmt.Println()

	// Step 12: Summary
	fmt.Println("=== Summary ===")
	fmt.Println("✓ API Key/Secret pair generated")
	fmt.Println("✓ Tokens created with specific permissions")
	fmt.Println("✓ Publisher joined with publish permissions")
	fmt.Println("✓ Viewer joined with subscribe-only permissions")
	fmt.Println("✓ Permission validation working correctly")
	fmt.Println("✓ Invalid tokens properly rejected")
	fmt.Println("✓ Room-specific token validation working")
	fmt.Println()

	fmt.Println("=== Integration Flow ===")
	fmt.Println("1. Server generates API Key/Secret pair (one-time)")
	fmt.Println("2. Backend creates access tokens for users (per-session)")
	fmt.Println("3. Client receives token from backend API")
	fmt.Println("4. Client uses token to join room")
	fmt.Println("5. Server validates token and permissions")
	fmt.Println("6. User joins room with appropriate permissions")
	fmt.Println()
	fmt.Println("✅ Complete authentication flow demonstrated!")
}
