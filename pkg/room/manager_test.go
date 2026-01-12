package room

import (
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

func TestNewRoomManager(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	if rm == nil {
		t.Fatal("RoomManager should not be nil")
	}

	if rm.GetRoomCount() != 0 {
		t.Errorf("Expected 0 rooms, got %d", rm.GetRoomCount())
	}
}

func TestRoomManagerCreateRoom(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	req := &CreateRoomRequest{
		Name:            "Test Room",
		MaxParticipants: 10,
	}

	room, err := rm.CreateRoom(req, "user-123")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	if room.Name != "Test Room" {
		t.Errorf("Expected room name 'Test Room', got '%s'", room.Name)
	}

	if rm.GetRoomCount() != 1 {
		t.Errorf("Expected 1 room, got %d", rm.GetRoomCount())
	}

	// Test creating room without name
	invalidReq := &CreateRoomRequest{}
	_, err = rm.CreateRoom(invalidReq, "user-123")
	if err == nil {
		t.Error("Should not be able to create room without name")
	}

	// Test creating room with nil request
	_, err = rm.CreateRoom(nil, "user-123")
	if err == nil {
		t.Error("Should not be able to create room with nil request")
	}
}

func TestRoomManagerDeleteRoom(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	req := &CreateRoomRequest{Name: "Test Room"}
	room, _ := rm.CreateRoom(req, "user-123")

	err := rm.DeleteRoom(room.ID)
	if err != nil {
		t.Fatalf("Failed to delete room: %v", err)
	}

	if rm.GetRoomCount() != 0 {
		t.Errorf("Expected 0 rooms, got %d", rm.GetRoomCount())
	}

	// Try to delete non-existent room
	err = rm.DeleteRoom("non-existent-room")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

func TestRoomManagerGetRoom(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	req := &CreateRoomRequest{Name: "Test Room"}
	createdRoom, _ := rm.CreateRoom(req, "user-123")

	room, err := rm.GetRoom(createdRoom.ID)
	if err != nil {
		t.Fatalf("Failed to get room: %v", err)
	}

	if room.ID != createdRoom.ID {
		t.Errorf("Expected room ID '%s', got '%s'", createdRoom.ID, room.ID)
	}

	// Try to get non-existent room
	_, err = rm.GetRoom("non-existent-room")
	if err != ErrRoomNotFound {
		t.Errorf("Expected ErrRoomNotFound, got %v", err)
	}
}

func TestRoomManagerListRooms(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	// Create multiple rooms
	for i := 0; i < 3; i++ {
		req := &CreateRoomRequest{Name: "Test Room"}
		rm.CreateRoom(req, "user-123")
	}

	rooms := rm.ListRooms()

	if len(rooms) != 3 {
		t.Errorf("Expected 3 rooms, got %d", len(rooms))
	}
}

func TestRoomManagerEventCallbacks(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	roomCreatedCalled := false
	participantJoinedCalled := false
	trackPublishedCalled := false

	// Register callbacks
	rm.OnRoomCreated(func(event *RoomEvent) {
		roomCreatedCalled = true
	})

	rm.OnParticipantJoined(func(event *RoomEvent) {
		participantJoinedCalled = true
	})

	rm.OnTrackPublished(func(event *RoomEvent) {
		trackPublishedCalled = true
	})

	// Create room
	req := &CreateRoomRequest{Name: "Test Room"}
	room, _ := rm.CreateRoom(req, "user-123")

	// Give events time to propagate (asynchronous)
	time.Sleep(10 * time.Millisecond)

	if !roomCreatedCalled {
		t.Error("OnRoomCreated callback should have been called")
	}

	// Add participant
	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	time.Sleep(10 * time.Millisecond)

	if !participantJoinedCalled {
		t.Error("OnParticipantJoined callback should have been called")
	}

	// Publish track
	track := &MediaTrack{
		ID:            "track-1",
		Kind:          "video",
		Source:        "camera",
		ParticipantID: "p1",
	}
	room.PublishTrack("p1", track)

	time.Sleep(10 * time.Millisecond)

	if !trackPublishedCalled {
		t.Error("OnTrackPublished callback should have been called")
	}
}

func TestRoomManagerShutdown(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	// Create rooms
	for i := 0; i < 3; i++ {
		req := &CreateRoomRequest{Name: "Test Room"}
		rm.CreateRoom(req, "user-123")
	}

	if rm.GetRoomCount() != 3 {
		t.Errorf("Expected 3 rooms before shutdown, got %d", rm.GetRoomCount())
	}

	rm.Shutdown()

	if rm.GetRoomCount() != 0 {
		t.Errorf("Expected 0 rooms after shutdown, got %d", rm.GetRoomCount())
	}
}

func TestRoomManagerCleanupEmptyRooms(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	rm := NewRoomManager(log)

	// Create room with timeout
	req := &CreateRoomRequest{
		Name:         "Test Room",
		EmptyTimeout: 100 * time.Millisecond,
	}
	room, _ := rm.CreateRoom(req, "user-123")

	// Add and remove participant to trigger empty state
	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)
	room.RemoveParticipant("p1")

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Cleanup should remove the room
	rm.CleanupEmptyRooms()

	_, err := rm.GetRoom(room.ID)
	if err != ErrRoomNotFound {
		t.Error("Empty room with timeout should have been cleaned up")
	}
}
