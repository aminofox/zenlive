// Package main demonstrates a complete video call scenario with media publishing and subscribing
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
	"github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

func main() {
	ctx := context.Background()

	printHeader("ZenLive Video Call with Media Publishing & Subscribing")
	printSection("This example demonstrates a complete video call scenario with:")
	fmt.Println("  • API key authentication")
	fmt.Println("  • WebRTC SFU (Selective Forwarding Unit)")
	fmt.Println("  • Multiple participants with different permissions")
	fmt.Println("  • Audio/video track publishing and subscribing")
	fmt.Println("  • Permission enforcement")
	fmt.Println("  • Real-time subscription matrix")
	fmt.Println()

	// ==================== STEP 1: Setup API Key Authentication ====================
	printStep(1, "Setting up API Key Authentication")

	apiKeyStore := auth.NewMemoryAPIKeyStore()
	keyManager := auth.NewAPIKeyManager(apiKeyStore)

	expiresIn := 365 * 24 * time.Hour
	apiKeyPair, err := keyManager.GenerateAPIKey(ctx, "Video Call Example", &expiresIn, nil)
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	printSuccess("API Key generated")
	fmt.Printf("    Access Key: %s\n", apiKeyPair.AccessKey)
	fmt.Printf("    Secret Key: %s\n\n", apiKeyPair.SecretKey[:20]+"...")

	// ==================== STEP 2: Setup WebRTC SFU ====================
	printStep(2, "Setting up WebRTC SFU (Selective Forwarding Unit)")

	logr := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Create WebRTC SFU with custom configuration
	sfuConfig := webrtc.DefaultSFUConfig()
	sfuConfig.MaxSubscribersPerStream = 100
	sfu := webrtc.NewSFU(sfuConfig, logr)
	defer sfu.Close()

	printSuccess("WebRTC SFU initialized")
	fmt.Printf("    Max subscribers per stream: %d\n", sfuConfig.MaxSubscribersPerStream)
	fmt.Println()

	// ==================== STEP 3: Setup Room Manager ====================
	printStep(3, "Setting up Room Manager with Authentication")

	authenticator := room.NewRoomAuthenticator(keyManager, logr)
	roomManager := room.NewAuthenticatedRoomManager(authenticator, apiKeyPair.SecretKey, logr)

	printSuccess("Room manager initialized with authentication\n")

	// ==================== STEP 4: Create Access Tokens ====================
	printStep(4, "Creating Access Tokens for Participants")

	// Alice - Can publish AND subscribe (typical video call participant)
	aliceToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_alice").
		SetName("Alice").
		SetEmail("alice@videocall.com").
		SetRoomJoin("team-meeting").
		SetCanPublish(true).     // Can publish video/audio
		SetCanSubscribe(true).   // Can subscribe to others
		SetCanPublishData(true). // Can send chat messages
		SetTTL(2 * time.Hour).
		SetMetadata(map[string]interface{}{
			"role":   "presenter",
			"device": "MacBook Pro",
		}).
		Build()
	if err != nil {
		log.Fatalf("Failed to create Alice's token: %v", err)
	}
	printSuccess("Alice's token created (can publish + subscribe)")

	// Bob - Can publish AND subscribe (typical video call participant)
	bobToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_bob").
		SetName("Bob").
		SetEmail("bob@videocall.com").
		SetRoomJoin("team-meeting").
		SetCanPublish(true).     // Can publish video/audio
		SetCanSubscribe(true).   // Can subscribe to others
		SetCanPublishData(true). // Can send chat messages
		SetTTL(2 * time.Hour).
		SetMetadata(map[string]interface{}{
			"role":   "participant",
			"device": "iPhone 15",
		}).
		Build()
	if err != nil {
		log.Fatalf("Failed to create Bob's token: %v", err)
	}
	printSuccess("Bob's token created (can publish + subscribe)")

	// Charlie - Can ONLY subscribe (viewer-only, cannot publish)
	charlieToken, err := auth.NewAccessTokenBuilder(apiKeyPair.AccessKey, apiKeyPair.SecretKey).
		SetIdentity("user_charlie").
		SetName("Charlie").
		SetEmail("charlie@videocall.com").
		SetRoomJoin("team-meeting").
		SetCanPublish(false).     // CANNOT publish video/audio
		SetCanSubscribe(true).    // Can subscribe to others
		SetCanPublishData(false). // Cannot send chat messages
		SetTTL(2 * time.Hour).
		SetMetadata(map[string]interface{}{
			"role":   "viewer",
			"device": "Android Tablet",
		}).
		Build()
	if err != nil {
		log.Fatalf("Failed to create Charlie's token: %v", err)
	}
	printSuccess("Charlie's token created (can ONLY subscribe - viewer mode)\n")

	// ==================== STEP 5: Participants Join the Room ====================
	printStep(5, "Participants Joining the Video Call Room")

	// Alice joins
	aliceJoinReq := &room.JoinRoomRequest{
		RoomName:    "team-meeting",
		AccessToken: aliceToken,
		Metadata: map[string]interface{}{
			"connection": "WiFi",
			"bandwidth":  "50 Mbps",
		},
	}

	alice, aliceRoom, err := roomManager.JoinRoomWithToken(ctx, aliceJoinReq)
	if err != nil {
		log.Fatalf("Alice failed to join: %v", err)
	}
	printSuccess(fmt.Sprintf("Alice joined room '%s'", aliceRoom.Name))
	printParticipantInfo(alice)

	// Bob joins
	bobJoinReq := &room.JoinRoomRequest{
		RoomName:    "team-meeting",
		AccessToken: bobToken,
		Metadata: map[string]interface{}{
			"connection": "5G",
			"bandwidth":  "30 Mbps",
		},
	}

	bob, bobRoom, err := roomManager.JoinRoomWithToken(ctx, bobJoinReq)
	if err != nil {
		log.Fatalf("Bob failed to join: %v", err)
	}
	printSuccess(fmt.Sprintf("Bob joined room '%s'", bobRoom.Name))
	printParticipantInfo(bob)

	// Charlie joins
	charlieJoinReq := &room.JoinRoomRequest{
		RoomName:    "team-meeting",
		AccessToken: charlieToken,
		Metadata: map[string]interface{}{
			"connection": "4G",
			"bandwidth":  "10 Mbps",
		},
	}

	charlie, charlieRoom, err := roomManager.JoinRoomWithToken(ctx, charlieJoinReq)
	if err != nil {
		log.Fatalf("Charlie failed to join: %v", err)
	}
	printSuccess(fmt.Sprintf("Charlie joined room '%s'", charlieRoom.Name))
	printParticipantInfo(charlie)
	fmt.Println()

	// ==================== STEP 6: Setup WebRTC SFU for Room ====================
	printStep(6, "Setting up WebRTC SFU for the Room")

	roomSFU := room.NewRoomSFU(aliceRoom, sfu, logr)

	// Notify SFU about participants
	roomSFU.OnParticipantJoined(alice.ID)
	roomSFU.OnParticipantJoined(bob.ID)
	roomSFU.OnParticipantJoined(charlie.ID)

	printSuccess("WebRTC SFU connected to room")
	fmt.Println("    All participants are ready for media streaming\n")

	// ==================== STEP 7: Alice Publishes Video and Audio ====================
	printStep(7, "Alice Publishing Video and Audio Tracks")

	// Alice publishes video track
	aliceVideoTrackID, err := roomSFU.PublishTrack(alice.ID, "alice_video_1", "video", "camera")
	if err != nil {
		log.Fatalf("Alice failed to publish video: %v", err)
	}
	printSuccess("Alice published video track")
	fmt.Printf("    Track ID: %s\n", aliceVideoTrackID)
	fmt.Printf("    Kind: video\n")
	fmt.Printf("    Source: camera\n")

	// Alice publishes audio track
	aliceAudioTrackID, err := roomSFU.PublishTrack(alice.ID, "alice_audio_1", "audio", "microphone")
	if err != nil {
		log.Fatalf("Alice failed to publish audio: %v", err)
	}
	printSuccess("Alice published audio track")
	fmt.Printf("    Track ID: %s\n", aliceAudioTrackID)
	fmt.Printf("    Kind: audio\n")
	fmt.Printf("    Source: microphone\n\n")

	printMediaFlow("Alice is now streaming:")
	fmt.Println("    📹 Video (camera) → WebRTC SFU → Bob, Charlie")
	fmt.Println("    🎤 Audio (microphone) → WebRTC SFU → Bob, Charlie\n")

	// ==================== STEP 8: Bob Publishes Video and Audio ====================
	printStep(8, "Bob Publishing Video and Audio Tracks")

	// Bob publishes video track
	bobVideoTrackID, err := roomSFU.PublishTrack(bob.ID, "bob_video_1", "video", "camera")
	if err != nil {
		log.Fatalf("Bob failed to publish video: %v", err)
	}
	printSuccess("Bob published video track")
	fmt.Printf("    Track ID: %s\n", bobVideoTrackID)
	fmt.Printf("    Kind: video\n")
	fmt.Printf("    Source: camera\n")

	// Bob publishes audio track
	bobAudioTrackID, err := roomSFU.PublishTrack(bob.ID, "bob_audio_1", "audio", "microphone")
	if err != nil {
		log.Fatalf("Bob failed to publish audio: %v", err)
	}
	printSuccess("Bob published audio track")
	fmt.Printf("    Track ID: %s\n", bobAudioTrackID)
	fmt.Printf("    Kind: audio\n")
	fmt.Printf("    Source: microphone\n\n")

	printMediaFlow("Bob is now streaming:")
	fmt.Println("    📹 Video (camera) → WebRTC SFU → Alice, Charlie")
	fmt.Println("    🎤 Audio (microphone) → WebRTC SFU → Alice, Charlie\n")

	// ==================== STEP 9: Charlie Tries to Publish (Should Fail) ====================
	printStep(9, "Charlie Attempting to Publish (Permission Test)")

	printWarning("Charlie attempting to publish video...")
	_, err = roomSFU.PublishTrack(charlie.ID, "charlie_video_1", "video", "camera")
	if err != nil {
		printError("Charlie's publish attempt was BLOCKED (as expected)")
		fmt.Printf("    Reason: %v\n", err)
		fmt.Println("    ✓ Permission system working correctly!")
	} else {
		log.Fatal("ERROR: Charlie should NOT have been able to publish!")
	}
	fmt.Println()

	// ==================== STEP 10: Display Current Room State ====================
	printStep(10, "Current Room State")

	participants := aliceRoom.ListParticipants()
	fmt.Printf("Room: %s (ID: %s)\n", aliceRoom.Name, aliceRoom.ID)
	fmt.Printf("Participants: %d\n\n", len(participants))

	for i, p := range participants {
		fmt.Printf("[%d] %s (ID: %s)\n", i+1, p.Username, p.ID)
		fmt.Printf("    Can Publish: %v\n", p.CanPublish)
		fmt.Printf("    Can Subscribe: %v\n", p.CanSubscribe)
		fmt.Printf("    State: %s\n", p.State)

		// Show published tracks
		tracks := roomSFU.GetParticipantTracks(p.ID)
		if len(tracks) > 0 {
			fmt.Printf("    Published Tracks:\n")
			for _, track := range tracks {
				fmt.Printf("      - %s: %s (%s)\n", track.Kind, track.ID, track.Source)
			}
		} else {
			fmt.Printf("    Published Tracks: none\n")
		}
		fmt.Println()
	}

	// ==================== STEP 11: Subscription Matrix ====================
	printStep(11, "Real-time Subscription Matrix")

	printInfo("Who is subscribing to whom:")
	fmt.Println()

	// Create subscription matrix
	matrix := make(map[string]map[string][]string) // subscriber -> publisher -> [tracks]

	// Alice subscribes to Bob's tracks (can subscribe)
	if alice.CanSubscribe {
		if matrix[alice.Username] == nil {
			matrix[alice.Username] = make(map[string][]string)
		}
		bobTracks := roomSFU.GetParticipantTracks(bob.ID)
		for _, track := range bobTracks {
			matrix[alice.Username][bob.Username] = append(
				matrix[alice.Username][bob.Username],
				fmt.Sprintf("%s (%s)", track.Kind, track.ID),
			)
		}
	}

	// Bob subscribes to Alice's tracks (can subscribe)
	if bob.CanSubscribe {
		if matrix[bob.Username] == nil {
			matrix[bob.Username] = make(map[string][]string)
		}
		aliceTracks := roomSFU.GetParticipantTracks(alice.ID)
		for _, track := range aliceTracks {
			matrix[bob.Username][alice.Username] = append(
				matrix[bob.Username][alice.Username],
				fmt.Sprintf("%s (%s)", track.Kind, track.ID),
			)
		}
	}

	// Charlie subscribes to both Alice and Bob (can subscribe, cannot publish)
	if charlie.CanSubscribe {
		if matrix[charlie.Username] == nil {
			matrix[charlie.Username] = make(map[string][]string)
		}

		aliceTracks := roomSFU.GetParticipantTracks(alice.ID)
		for _, track := range aliceTracks {
			matrix[charlie.Username][alice.Username] = append(
				matrix[charlie.Username][alice.Username],
				fmt.Sprintf("%s (%s)", track.Kind, track.ID),
			)
		}

		bobTracks := roomSFU.GetParticipantTracks(bob.ID)
		for _, track := range bobTracks {
			matrix[charlie.Username][bob.Username] = append(
				matrix[charlie.Username][bob.Username],
				fmt.Sprintf("%s (%s)", track.Kind, track.ID),
			)
		}
	}

	// Display matrix
	for subscriber, publishers := range matrix {
		fmt.Printf("📺 %s is watching:\n", subscriber)
		for publisher, tracks := range publishers {
			fmt.Printf("   └─ %s's stream:\n", publisher)
			for _, track := range tracks {
				fmt.Printf("      • %s\n", track)
			}
		}
		fmt.Println()
	}

	// ==================== STEP 12: Media Flow Explanation ====================
	printStep(12, "Complete Media Flow Explanation")

	fmt.Println("┌─────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                     MEDIA FLOW DIAGRAM                          │")
	fmt.Println("└─────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("PUBLISHERS (Alice & Bob):")
	fmt.Println("  Alice (camera + mic)")
	fmt.Println("    ↓ WebRTC Publisher")
	fmt.Println("    ↓ Encode & Packetize")
	fmt.Println("    └──→ WebRTC SFU")
	fmt.Println()
	fmt.Println("  Bob (camera + mic)")
	fmt.Println("    ↓ WebRTC Publisher")
	fmt.Println("    ↓ Encode & Packetize")
	fmt.Println("    └──→ WebRTC SFU")
	fmt.Println()
	fmt.Println("SELECTIVE FORWARDING UNIT (SFU):")
	fmt.Println("  • Receives media streams from publishers")
	fmt.Println("  • Does NOT transcode (low latency, efficient)")
	fmt.Println("  • Selectively forwards to subscribers")
	fmt.Println("  • Handles bandwidth estimation")
	fmt.Println("  • Manages quality adaptation")
	fmt.Println()
	fmt.Println("SUBSCRIBERS (Alice, Bob, Charlie):")
	fmt.Println("  WebRTC SFU ──→ Alice receives:")
	fmt.Println("                   • Bob's video + audio")
	fmt.Println()
	fmt.Println("  WebRTC SFU ──→ Bob receives:")
	fmt.Println("                   • Alice's video + audio")
	fmt.Println()
	fmt.Println("  WebRTC SFU ──→ Charlie receives:")
	fmt.Println("                   • Alice's video + audio")
	fmt.Println("                   • Bob's video + audio")
	fmt.Println("                   (Charlie is view-only, cannot publish)")
	fmt.Println()

	// ==================== STEP 13: Architecture Benefits ====================
	printStep(13, "WebRTC SFU Architecture Benefits")

	fmt.Println("✓ Low Latency:")
	fmt.Println("    Sub-second latency (~100-300ms) for real-time communication")
	fmt.Println()
	fmt.Println("✓ Efficient Bandwidth:")
	fmt.Println("    Each publisher uploads once to SFU")
	fmt.Println("    SFU forwards to multiple subscribers")
	fmt.Println("    No peer-to-peer mesh complexity")
	fmt.Println()
	fmt.Println("✓ Scalability:")
	fmt.Println("    SFU can handle 100+ participants")
	fmt.Println("    No transcoding = lower server CPU usage")
	fmt.Println("    Subscribers receive optimal quality for their bandwidth")
	fmt.Println()
	fmt.Println("✓ Quality Adaptation:")
	fmt.Println("    Automatic bitrate adjustment")
	fmt.Println("    Simulcast support for multi-quality streams")
	fmt.Println("    Forward Error Correction (FEC)")
	fmt.Println()
	fmt.Println("✓ Security:")
	fmt.Println("    End-to-end encryption (DTLS-SRTP)")
	fmt.Println("    Token-based authentication")
	fmt.Println("    Permission-based access control")
	fmt.Println()

	// ==================== STEP 14: Use Cases ====================
	printStep(14, "Typical Use Cases")

	fmt.Println("📞 Video Conferencing:")
	fmt.Println("    All participants can publish and subscribe")
	fmt.Println("    Example: Team meetings, remote collaboration")
	fmt.Println()
	fmt.Println("🎓 Virtual Classroom:")
	fmt.Println("    Teacher can publish (presenter)")
	fmt.Println("    Students can only subscribe (viewers)")
	fmt.Println("    Example: Online courses, webinars")
	fmt.Println()
	fmt.Println("🎮 Live Streaming:")
	fmt.Println("    Streamer publishes")
	fmt.Println("    Viewers subscribe only")
	fmt.Println("    Example: Gaming streams, live events")
	fmt.Println()
	fmt.Println("🏥 Telemedicine:")
	fmt.Println("    Doctor and patient can both publish/subscribe")
	fmt.Println("    Observers can subscribe only")
	fmt.Println("    Example: Remote consultations, medical training")
	fmt.Println()

	printSuccess("Example completed successfully!")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("Summary:")
	fmt.Println("  • 3 participants joined with different permissions")
	fmt.Println("  • 2 participants (Alice & Bob) published 4 total tracks")
	fmt.Println("  • 1 participant (Charlie) is viewer-only")
	fmt.Println("  • Permission system blocked Charlie from publishing")
	fmt.Println("  • All participants can see each other's streams")
	fmt.Println("  • WebRTC SFU efficiently forwards media")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

// ==================== Helper Functions ====================

func printHeader(text string) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  %s\n", text)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

func printSection(text string) {
	fmt.Println(text)
}

func printStep(num int, title string) {
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("STEP %d: %s\n", num, title)
	fmt.Println("───────────────────────────────────────────────────────────────")
}

func printSuccess(text string) {
	fmt.Printf("✓ %s\n", text)
}

func printWarning(text string) {
	fmt.Printf("⚠ %s\n", text)
}

func printError(text string) {
	fmt.Printf("✗ %s\n", text)
}

func printInfo(text string) {
	fmt.Printf("ℹ %s\n", text)
}

func printMediaFlow(text string) {
	fmt.Printf("🔄 %s\n", text)
}

func printParticipantInfo(p *room.Participant) {
	fmt.Printf("    User ID: %s\n", p.ID)
	fmt.Printf("    Username: %s\n", p.Username)
	fmt.Printf("    Can Publish: %v\n", p.CanPublish)
	fmt.Printf("    Can Subscribe: %v\n", p.CanSubscribe)
	fmt.Printf("    Can Publish Data: %v\n", p.CanPublishData)
}
