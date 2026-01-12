package room

import (
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

func TestNewRoom(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{
		Name:            "Test Room",
		MaxParticipants: 10,
		EmptyTimeout:    5 * time.Minute,
		Metadata:        map[string]interface{}{"test": "value"},
	}

	room := NewRoom(req, "user-123", log, eventBus)

	if room.ID == "" {
		t.Error("Room ID should not be empty")
	}

	if room.Name != "Test Room" {
		t.Errorf("Expected room name 'Test Room', got '%s'", room.Name)
	}

	if room.MaxParticipants != 10 {
		t.Errorf("Expected max participants 10, got %d", room.MaxParticipants)
	}

	if room.CreatedBy != "user-123" {
		t.Errorf("Expected created by 'user-123', got '%s'", room.CreatedBy)
	}

	if room.IsClosed() {
		t.Error("New room should not be closed")
	}

	if !room.IsEmpty() {
		t.Error("New room should be empty")
	}
}

func TestRoomAddParticipant(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{
		Name:            "Test Room",
		MaxParticipants: 2,
	}

	room := NewRoom(req, "user-123", log, eventBus)

	// Add first participant
	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	err := room.AddParticipant(p1)
	if err != nil {
		t.Fatalf("Failed to add participant: %v", err)
	}

	if room.GetParticipantCount() != 1 {
		t.Errorf("Expected 1 participant, got %d", room.GetParticipantCount())
	}

	if room.IsEmpty() {
		t.Error("Room should not be empty after adding participant")
	}

	// Add second participant
	p2 := NewParticipant("p2", "user-2", "Bob", RoleSpeaker)
	err = room.AddParticipant(p2)
	if err != nil {
		t.Fatalf("Failed to add second participant: %v", err)
	}

	if room.GetParticipantCount() != 2 {
		t.Errorf("Expected 2 participants, got %d", room.GetParticipantCount())
	}

	// Try to add third participant (should fail - room is full)
	p3 := NewParticipant("p3", "user-3", "Charlie", RoleAttendee)
	err = room.AddParticipant(p3)
	if err != ErrRoomFull {
		t.Errorf("Expected ErrRoomFull, got %v", err)
	}

	// Try to add duplicate participant
	err = room.AddParticipant(p1)
	if err != ErrParticipantExists {
		t.Errorf("Expected ErrParticipantExists, got %v", err)
	}
}

func TestRoomRemoveParticipant(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	// Remove participant
	err := room.RemoveParticipant("p1")
	if err != nil {
		t.Fatalf("Failed to remove participant: %v", err)
	}

	if room.GetParticipantCount() != 0 {
		t.Errorf("Expected 0 participants, got %d", room.GetParticipantCount())
	}

	if !room.IsEmpty() {
		t.Error("Room should be empty after removing all participants")
	}

	// Try to remove non-existent participant
	err = room.RemoveParticipant("p999")
	if err != ErrParticipantNotFound {
		t.Errorf("Expected ErrParticipantNotFound, got %v", err)
	}
}

func TestRoomGetParticipant(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	// Get existing participant
	participant, err := room.GetParticipant("p1")
	if err != nil {
		t.Fatalf("Failed to get participant: %v", err)
	}

	if participant.ID != "p1" {
		t.Errorf("Expected participant ID 'p1', got '%s'", participant.ID)
	}

	// Get non-existent participant
	_, err = room.GetParticipant("p999")
	if err != ErrParticipantNotFound {
		t.Errorf("Expected ErrParticipantNotFound, got %v", err)
	}
}

func TestRoomListParticipants(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	p2 := NewParticipant("p2", "user-2", "Bob", RoleSpeaker)

	room.AddParticipant(p1)
	room.AddParticipant(p2)

	participants := room.ListParticipants()

	if len(participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(participants))
	}
}

func TestRoomUpdateParticipantPermissions(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleAttendee)
	room.AddParticipant(p1)

	// Update permissions
	newPerms := ParticipantPermissions{
		CanPublish:   true,
		CanSubscribe: true,
	}

	err := room.UpdateParticipantPermissions("p1", newPerms)
	if err != nil {
		t.Fatalf("Failed to update permissions: %v", err)
	}

	participant, _ := room.GetParticipant("p1")
	perms := participant.GetPermissions()

	if !perms.CanPublish {
		t.Error("Expected CanPublish to be true")
	}

	// Try to update non-existent participant
	err = room.UpdateParticipantPermissions("p999", newPerms)
	if err != ErrParticipantNotFound {
		t.Errorf("Expected ErrParticipantNotFound, got %v", err)
	}
}

func TestRoomPublishTrack(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	track := &MediaTrack{
		ID:            "track-1",
		Kind:          "video",
		Source:        "camera",
		ParticipantID: "p1",
	}

	err := room.PublishTrack("p1", track)
	if err != nil {
		t.Fatalf("Failed to publish track: %v", err)
	}

	participant, _ := room.GetParticipant("p1")
	tracks := participant.GetTracks()

	if len(tracks) != 1 {
		t.Errorf("Expected 1 track, got %d", len(tracks))
	}

	// Try to publish track for participant without permission
	p2 := NewParticipant("p2", "user-2", "Bob", RoleAttendee)
	room.AddParticipant(p2)

	track2 := &MediaTrack{
		ID:            "track-2",
		Kind:          "audio",
		Source:        "microphone",
		ParticipantID: "p2",
	}

	err = room.PublishTrack("p2", track2)
	if err != ErrUnauthorized {
		t.Errorf("Expected ErrUnauthorized, got %v", err)
	}
}

func TestRoomUnpublishTrack(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	track := &MediaTrack{
		ID:            "track-1",
		Kind:          "video",
		Source:        "camera",
		ParticipantID: "p1",
	}

	room.PublishTrack("p1", track)

	// Unpublish track
	err := room.UnpublishTrack("p1", "track-1")
	if err != nil {
		t.Fatalf("Failed to unpublish track: %v", err)
	}

	participant, _ := room.GetParticipant("p1")
	tracks := participant.GetTracks()

	if len(tracks) != 0 {
		t.Errorf("Expected 0 tracks, got %d", len(tracks))
	}
}

func TestRoomUpdateMetadata(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{
		Name:     "Test Room",
		Metadata: map[string]interface{}{"key1": "value1"},
	}
	room := NewRoom(req, "user-123", log, eventBus)

	newMetadata := map[string]interface{}{
		"key2": "value2",
		"key3": 123,
	}

	room.UpdateMetadata(newMetadata)

	if room.Metadata["key1"] != "value1" {
		t.Error("Original metadata should be preserved")
	}

	if room.Metadata["key2"] != "value2" {
		t.Error("New metadata should be added")
	}

	if room.Metadata["key3"] != 123 {
		t.Error("New metadata should be added")
	}
}

func TestRoomClose(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	eventBus := NewEventBus()

	req := &CreateRoomRequest{Name: "Test Room"}
	room := NewRoom(req, "user-123", log, eventBus)

	p1 := NewParticipant("p1", "user-1", "Alice", RoleHost)
	room.AddParticipant(p1)

	room.Close()

	if !room.IsClosed() {
		t.Error("Room should be closed")
	}

	if !room.IsEmpty() {
		t.Error("Room should be empty after closing")
	}

	// Try to add participant to closed room
	p2 := NewParticipant("p2", "user-2", "Bob", RoleSpeaker)
	err := room.AddParticipant(p2)
	if err == nil {
		t.Error("Should not be able to add participant to closed room")
	}
}
