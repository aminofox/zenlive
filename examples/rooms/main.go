package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aminofox/zenlive"
	"github.com/aminofox/zenlive/pkg/config"
	"github.com/aminofox/zenlive/pkg/room"
)

func main() {
	fmt.Println("ZenLive Room System Example")
	fmt.Println("============================")

	// Create SDK instance
	sdk, err := zenlive.New(&config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create SDK: %v", err)
	}

	// Start the SDK
	if err := sdk.Start(); err != nil {
		log.Fatalf("Failed to start SDK: %v", err)
	}
	defer sdk.Stop()

	// Get the room manager
	roomManager := sdk.GetRoomManager()

	// Example 1: Create a room
	fmt.Println("\n1. Creating a room...")
	testRoom, err := roomManager.CreateRoom(&room.CreateRoomRequest{
		Name:            "Conference Room",
		MaxParticipants: 50,
		EmptyTimeout:    5 * time.Minute,
		Metadata: map[string]interface{}{
			"topic":       "Product Launch Discussion",
			"description": "Q1 2024 Product Launch Planning",
			"scheduled":   time.Now().Format(time.RFC3339),
		},
	}, "admin-user-123")
	if err != nil {
		log.Fatalf("Failed to create room: %v", err)
	}
	fmt.Printf("✓ Room created: %s (ID: %s)\n", testRoom.Name, testRoom.ID)

	// Example 2: Add participants to the room
	fmt.Println("\n2. Adding participants...")

	// Add host
	host := room.NewParticipant("p-host-1", "user-001", "John (Host)", room.RoleHost)
	if err := testRoom.AddParticipant(host); err != nil {
		log.Fatalf("Failed to add host: %v", err)
	}
	fmt.Printf("✓ Added host: %s\n", host.Username)

	// Add speaker
	speaker := room.NewParticipant("p-speaker-1", "user-002", "Alice (Speaker)", room.RoleSpeaker)
	if err := testRoom.AddParticipant(speaker); err != nil {
		log.Fatalf("Failed to add speaker: %v", err)
	}
	fmt.Printf("✓ Added speaker: %s\n", speaker.Username)

	// Add attendee
	attendee := room.NewParticipant("p-attendee-1", "user-003", "Bob (Attendee)", room.RoleAttendee)
	if err := testRoom.AddParticipant(attendee); err != nil {
		log.Fatalf("Failed to add attendee: %v", err)
	}
	fmt.Printf("✓ Added attendee: %s\n", attendee.Username)

	// Example 3: Update participant state
	fmt.Println("\n3. Updating participant states...")
	host.UpdateState(room.StateJoined)
	speaker.UpdateState(room.StateJoined)
	attendee.UpdateState(room.StateJoined)
	fmt.Println("✓ All participants joined successfully")

	// Example 4: Publish tracks
	fmt.Println("\n4. Publishing media tracks...")

	// Host publishes video and audio
	videoTrack := &room.MediaTrack{
		ID:            "track-video-host",
		Kind:          "video",
		Source:        "camera",
		ParticipantID: host.ID,
	}
	if err := testRoom.PublishTrack(host.ID, videoTrack); err != nil {
		log.Fatalf("Failed to publish video: %v", err)
	}
	fmt.Printf("✓ Host published video track: %s\n", videoTrack.ID)

	audioTrack := &room.MediaTrack{
		ID:            "track-audio-host",
		Kind:          "audio",
		Source:        "microphone",
		ParticipantID: host.ID,
	}
	if err := testRoom.PublishTrack(host.ID, audioTrack); err != nil {
		log.Fatalf("Failed to publish audio: %v", err)
	}
	fmt.Printf("✓ Host published audio track: %s\n", audioTrack.ID)

	// Speaker publishes audio only
	speakerAudio := &room.MediaTrack{
		ID:            "track-audio-speaker",
		Kind:          "audio",
		Source:        "microphone",
		ParticipantID: speaker.ID,
	}
	if err := testRoom.PublishTrack(speaker.ID, speakerAudio); err != nil {
		log.Fatalf("Failed to publish speaker audio: %v", err)
	}
	fmt.Printf("✓ Speaker published audio track: %s\n", speakerAudio.ID)

	// Example 5: List all participants
	fmt.Println("\n5. Listing all participants...")
	participants := testRoom.ListParticipants()
	fmt.Printf("Total participants: %d\n", len(participants))
	for _, p := range participants {
		tracks := p.GetTracks()
		fmt.Printf("  - %s (%s) - %d tracks\n", p.Username, p.Role, len(tracks))
	}

	// Example 6: Update room metadata
	fmt.Println("\n6. Updating room metadata...")
	testRoom.UpdateMetadata(map[string]interface{}{
		"status":      "in-progress",
		"activeUsers": len(participants),
		"startTime":   time.Now().Format(time.RFC3339),
	})
	fmt.Println("✓ Room metadata updated")

	// Example 7: Update participant permissions
	fmt.Println("\n7. Promoting attendee to speaker...")
	newPermissions := room.ParticipantPermissions{
		CanPublish:        true,
		CanSubscribe:      true,
		CanPublishData:    true,
		CanUpdateMetadata: true,
	}
	if err := testRoom.UpdateParticipantPermissions(attendee.ID, newPermissions); err != nil {
		log.Fatalf("Failed to update permissions: %v", err)
	}
	fmt.Printf("✓ %s now has publishing permissions\n", attendee.Username)

	// Example 8: Event handling
	fmt.Println("\n8. Setting up event callbacks...")
	roomManager.OnParticipantJoined(func(event *room.RoomEvent) {
		fmt.Printf("📢 Event: Participant joined (Room: %s, Time: %s)\n",
			event.RoomID, event.Timestamp.Format(time.RFC3339))
	})

	roomManager.OnParticipantLeft(func(event *room.RoomEvent) {
		fmt.Printf("📢 Event: Participant left (Room: %s, Time: %s)\n",
			event.RoomID, event.Timestamp.Format(time.RFC3339))
	})

	roomManager.OnTrackPublished(func(event *room.RoomEvent) {
		fmt.Printf("📢 Event: Track published (Room: %s, Time: %s)\n",
			event.RoomID, event.Timestamp.Format(time.RFC3339))
	})

	// Example 9: Create another room to demonstrate listing
	fmt.Println("\n9. Creating additional room...")
	room2, err := roomManager.CreateRoom(&room.CreateRoomRequest{
		Name:            "Webinar Room",
		MaxParticipants: 100,
		EmptyTimeout:    10 * time.Minute,
		Metadata: map[string]interface{}{
			"type": "webinar",
		},
	}, "admin-user-123")
	if err != nil {
		log.Fatalf("Failed to create second room: %v", err)
	}
	fmt.Printf("✓ Room created: %s (ID: %s)\n", room2.Name, room2.ID)

	// Example 10: List all rooms
	fmt.Println("\n10. Listing all rooms...")
	allRooms := roomManager.ListRooms()
	fmt.Printf("Total active rooms: %d\n", len(allRooms))
	for _, r := range allRooms {
		fmt.Printf("  - %s (ID: %s, Participants: %d)\n",
			r.Name, r.ID, len(r.ListParticipants()))
	}

	// Example 11: Remove a participant
	fmt.Println("\n11. Removing a participant...")
	if err := testRoom.RemoveParticipant(attendee.ID); err != nil {
		log.Fatalf("Failed to remove participant: %v", err)
	}
	fmt.Printf("✓ Removed participant: %s\n", attendee.Username)

	// Example 12: Get room information
	fmt.Println("\n12. Getting room information...")
	retrievedRoom, err := roomManager.GetRoom(testRoom.ID)
	if err != nil {
		log.Fatalf("Failed to get room: %v", err)
	}
	fmt.Printf("✓ Retrieved room: %s\n", retrievedRoom.Name)
	fmt.Printf("  Created: %s\n", retrievedRoom.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Max Participants: %d\n", retrievedRoom.MaxParticipants)
	fmt.Printf("  Current Participants: %d\n", len(retrievedRoom.ListParticipants()))

	// Clean up
	fmt.Println("\n13. Cleaning up...")
	if err := roomManager.DeleteRoom(testRoom.ID); err != nil {
		log.Fatalf("Failed to delete room: %v", err)
	}
	fmt.Printf("✓ Deleted room: %s\n", testRoom.Name)

	if err := roomManager.DeleteRoom(room2.ID); err != nil {
		log.Fatalf("Failed to delete room: %v", err)
	}
	fmt.Printf("✓ Deleted room: %s\n", room2.Name)

	fmt.Println("\n✅ Room system example completed successfully!")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("  • Room creation and management")
	fmt.Println("  • Participant lifecycle (add, update, remove)")
	fmt.Println("  • Role-based permissions (Host, Speaker, Attendee)")
	fmt.Println("  • Media track publishing")
	fmt.Println("  • Event-driven architecture")
	fmt.Println("  • Room metadata management")
	fmt.Println("  • Multi-room support")
}
