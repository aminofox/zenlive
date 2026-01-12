// Package main demonstrates multi-participant video conferencing with WebRTC integration
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
	"github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

func main() {
	// Create logger
	roomLogger := logger.NewDefaultLogger(1, "text") // Info level

	fmt.Println("=== ZenLive Phase 2: Multi-Participant Video Conferencing Demo ===\n")

	// 1. Create room manager
	roomManager := room.NewRoomManager(roomLogger)

	// 2. Create WebRTC SFU
	sfuConfig := webrtc.DefaultSFUConfig()
	sfu := webrtc.NewSFU(sfuConfig, roomLogger)

	// 3. Create session manager for multi-room support
	sessionManager := room.NewSessionManager(room.DefaultSessionManagerConfig(), roomLogger)

	// 4. Create quality monitor
	qualityMonitor := room.NewNetworkQualityMonitor("conference-room", roomLogger)

	// 5. Create reconnection handler
	reconnectionHandler := room.NewReconnectionHandler(room.DefaultReconnectionConfig(), roomLogger)

	// Set reconnection callbacks
	reconnectionHandler.SetCallbacks(
		func(participantID string) {
			fmt.Printf("✓ Participant %s reconnected successfully\n", participantID)
		},
		func(participantID string, err error) {
			fmt.Printf("✗ Participant %s reconnection failed: %v\n", participantID, err)
		},
	)

	// 6. Create subscription manager for adaptive streaming
	config := room.DefaultSimulcastConfig()
	subscriptionManager := room.NewSubscriptionManager(config)

	fmt.Println("=== Creating Conference Room ===")

	// Create a conference room
	conferenceRoom, err := roomManager.CreateRoom(&room.CreateRoomRequest{
		Name:            "Tech Conference",
		MaxParticipants: 10,
		EmptyTimeout:    5 * time.Minute,
		Metadata: map[string]interface{}{
			"type":        "conference",
			"description": "Multi-participant video conferencing demo",
		},
	}, "admin-001")
	if err != nil {
		log.Fatal("Failed to create room:", err)
	}

	fmt.Printf("✓ Conference room created: %s (%s)\n\n", conferenceRoom.Name, conferenceRoom.ID)

	// 7. Create RoomSFU to connect room with WebRTC
	roomSFU := room.NewRoomSFU(conferenceRoom, sfu, roomLogger)

	fmt.Println("=== Simulating Participants Joining ===")

	// Simulate participants joining
	participants := []struct {
		ID       string
		Username string
		UserID   string
	}{
		{"p1", "Alice", "user-001"},
		{"p2", "Bob", "user-002"},
		{"p3", "Charlie", "user-003"},
	}

	for _, p := range participants {
		// Create session for user
		sessionManager.CreateSession(p.UserID)

		// Create participant object
		participant := room.NewParticipant(p.ID, p.UserID, p.Username, room.RoleSpeaker)
		participant.Metadata = map[string]interface{}{
			"device": "browser",
		}

		// Join room
		err := conferenceRoom.AddParticipant(participant)
		if err != nil {
			fmt.Printf("✗ Failed to add %s: %v\n", p.Username, err)
			continue
		}

		// Add room to session
		sessionManager.JoinRoom(p.UserID, conferenceRoom.ID, p.ID)

		// Notify RoomSFU about new participant
		roomSFU.OnParticipantJoined(p.ID)

		// Simulate each participant publishing video and audio tracks
		videoTrack := fmt.Sprintf("%s-video", p.ID)
		audioTrack := fmt.Sprintf("%s-audio", p.ID)

		// Publish tracks
		_, err = roomSFU.PublishTrack(p.ID, videoTrack, "video", "camera")
		if err != nil {
			fmt.Printf("✗ Failed to publish video for %s: %v\n", p.Username, err)
		}

		_, err = roomSFU.PublishTrack(p.ID, audioTrack, "audio", "microphone")
		if err != nil {
			fmt.Printf("✗ Failed to publish audio for %s: %v\n", p.Username, err)
		}

		// Add tracks to session
		sessionManager.AddTrack(p.UserID, conferenceRoom.ID, videoTrack)
		sessionManager.AddTrack(p.UserID, conferenceRoom.ID, audioTrack)

		// Initialize network quality monitoring (95/100 = excellent)
		qualityMonitor.UpdateQuality(p.ID, 0.5, 5*time.Millisecond, 50*time.Millisecond, 95)

		fmt.Printf("✓ %s joined and published video + audio tracks\n", p.Username)
	}

	fmt.Println("\n=== Demonstrating Multi-Stream Subscription ===")

	// Subscribe Alice to Bob's video with high quality
	subID1, err := subscriptionManager.Subscribe("p1", "p2", "p2-video", room.QualityHigh)
	if err != nil {
		fmt.Printf("✗ Failed to subscribe: %v\n", err)
	} else {
		fmt.Printf("✓ Alice subscribed to Bob's video (high quality) - %s\n", subID1)
	}

	// Subscribe Alice to Charlie's video with auto quality
	subID2, err := subscriptionManager.Subscribe("p1", "p3", "p3-video", room.QualityAuto)
	if err != nil {
		fmt.Printf("✗ Failed to subscribe: %v\n", err)
	} else {
		fmt.Printf("✓ Alice subscribed to Charlie's video (auto quality) - %s\n", subID2)
	}

	fmt.Println("\n=== Demonstrating Simulcast Quality Switching ===")

	// Select best layer for subscription based on available bandwidth
	// 1 Mbps = 1000000 bps
	selectedQuality := subscriptionManager.SelectLayer(1000000)
	fmt.Printf("✓ Selected quality level for 1 Mbps bandwidth: %s\n", selectedQuality)

	fmt.Println("\n=== Demonstrating Multi-Room Participation ===")

	// Create a second room
	breakoutRoom, err := roomManager.CreateRoom(&room.CreateRoomRequest{
		Name:            "Breakout Session",
		MaxParticipants: 5,
		EmptyTimeout:    5 * time.Minute,
		Metadata: map[string]interface{}{
			"type": "breakout",
		},
	}, "admin-001")
	if err != nil {
		log.Fatal("Failed to create breakout room:", err)
	}

	// Alice joins the breakout room (now in 2 rooms)
	breakoutParticipant := room.NewParticipant("p1-breakout", "user-001", "Alice", room.RoleSpeaker)
	breakoutParticipant.Metadata = map[string]interface{}{
		"device": "browser",
	}
	err = breakoutRoom.AddParticipant(breakoutParticipant)
	if err != nil {
		fmt.Printf("✗ Failed to add Alice to breakout room: %v\n", err)
	} else {
		sessionManager.JoinRoom("user-001", breakoutRoom.ID, "p1-breakout")
		fmt.Println("✓ Alice joined breakout room - now in 2 rooms simultaneously")
	}

	// Check session stats
	stats := sessionManager.GetSessionStats()
	fmt.Printf("✓ Session stats: %d sessions, %d total rooms, %.2f avg rooms/session\n",
		stats["total_sessions"], stats["total_rooms"], stats["avg_rooms_per_session"])

	fmt.Println("\n=== Demonstrating Network Quality Monitoring ===")

	// Simulate network quality degradation for Bob (25/100 = poor)
	qualityMonitor.UpdateQuality("p2", 12.0, 150*time.Millisecond, 400*time.Millisecond, 25)

	// Check if quality is good (returns quality object and exists flag)
	bobQuality, exists := qualityMonitor.GetQuality("p2")
	if !exists {
		fmt.Println("✗ Could not get Bob's quality")
	} else if bobQuality.Score < 50 {
		fmt.Printf("⚠ Bob's network quality is poor (score: %.1f/100)\n", bobQuality.Score)
	} else {
		fmt.Println("✓ Bob's network quality is good")
	}

	// Get average quality across room
	avgQuality := qualityMonitor.GetRoomAverageQuality()
	fmt.Printf("✓ Room average quality score: %.1f/100\n", avgQuality.Score)

	fmt.Println("\n=== Demonstrating Reconnection Handling ===")

	// Simulate Bob disconnecting
	fmt.Println("⚠ Simulating Bob disconnection...")
	reconnectionHandler.HandleDisconnect("p2")

	// Wait to see reconnection attempts
	time.Sleep(2 * time.Second)

	// Check reconnection state
	state, err := reconnectionHandler.GetReconnectionState("p2")
	if err == nil {
		fmt.Printf("✓ Reconnection state for Bob: %d attempts made\n", state.Attempts)
	}

	fmt.Println("\n=== Demo Summary ===")

	// Count rooms and participants
	fmt.Printf("Rooms: 2, Participants: 4, Total tracks: 6\n")

	// List participants and their tracks
	fmt.Println("\n=== Participants and Tracks ===")
	for _, p := range participants {
		tracks := roomSFU.GetParticipantTracks(p.ID)
		fmt.Printf("- %s: %d tracks\n", p.Username, len(tracks))
	}

	fmt.Println("\n✓ Phase 2 features demonstrated successfully!")
	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n=== Shutting down ===")

	// Cleanup
	// Close RoomSFU
	roomSFU.Close()

	// Close reconnection handler
	reconnectionHandler.Close()

	// Shutdown room manager
	roomManager.Shutdown()

	fmt.Println("✓ Demo completed successfully")
}
