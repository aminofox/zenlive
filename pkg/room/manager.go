package room

import (
	"errors"
	"sync"

	"github.com/aminofox/zenlive/pkg/logger"
)

var (
	// ErrRoomNotFound is returned when a room doesn't exist
	ErrRoomNotFound = errors.New("room not found")
	// ErrRoomExists is returned when trying to create a duplicate room
	ErrRoomExists = errors.New("room already exists")
)

// RoomManager manages all rooms in the system
type RoomManager struct {
	// rooms stores all rooms by room ID
	rooms map[string]*Room
	// mu protects concurrent access
	mu sync.RWMutex
	// eventBus for publishing room events
	eventBus *EventBus
	// logger for room manager events
	logger logger.Logger
}

// NewRoomManager creates a new room manager
func NewRoomManager(log logger.Logger) *RoomManager {
	return &RoomManager{
		rooms:    make(map[string]*Room),
		eventBus: NewEventBus(),
		logger:   log,
	}
}

// CreateRoom creates a new room
func (rm *RoomManager) CreateRoom(req *CreateRoomRequest, createdBy string) (*Room, error) {
	if req == nil {
		return nil, errors.New("create room request is required")
	}

	if req.Name == "" {
		return nil, errors.New("room name is required")
	}

	room := NewRoom(req, createdBy, rm.logger, rm.eventBus)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if room ID already exists (extremely unlikely with UUID)
	if _, exists := rm.rooms[room.ID]; exists {
		return nil, ErrRoomExists
	}

	rm.rooms[room.ID] = room

	rm.logger.Info("Room created",
		logger.Field{Key: "room_id", Value: room.ID},
		logger.Field{Key: "name", Value: room.Name},
		logger.Field{Key: "created_by", Value: createdBy},
	)

	// Publish event
	rm.eventBus.Publish(createEvent(EventRoomCreated, room.ID, room))

	return room, nil
}

// DeleteRoom deletes a room
func (rm *RoomManager) DeleteRoom(roomID string) error {
	rm.mu.Lock()
	room, exists := rm.rooms[roomID]
	if !exists {
		rm.mu.Unlock()
		return ErrRoomNotFound
	}

	delete(rm.rooms, roomID)
	rm.mu.Unlock()

	// Close the room
	room.Close()

	rm.logger.Info("Room deleted",
		logger.Field{Key: "room_id", Value: roomID},
	)

	// Publish event
	rm.eventBus.Publish(createEvent(EventRoomDeleted, roomID, room))

	return nil
}

// GetRoom returns a room by ID
func (rm *RoomManager) GetRoom(roomID string) (*Room, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, exists := rm.rooms[roomID]
	if !exists {
		return nil, ErrRoomNotFound
	}

	return room, nil
}

// ListRooms returns all active rooms
func (rm *RoomManager) ListRooms() []*Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rooms := make([]*Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		rooms = append(rooms, room)
	}

	return rooms
}

// GetRoomCount returns the number of active rooms
func (rm *RoomManager) GetRoomCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.rooms)
}

// GetEventBus returns the event bus for subscribing to events
func (rm *RoomManager) GetEventBus() *EventBus {
	return rm.eventBus
}

// OnRoomCreated registers a callback for room created events
func (rm *RoomManager) OnRoomCreated(callback EventCallback) {
	rm.eventBus.Subscribe(EventRoomCreated, callback)
}

// OnRoomDeleted registers a callback for room deleted events
func (rm *RoomManager) OnRoomDeleted(callback EventCallback) {
	rm.eventBus.Subscribe(EventRoomDeleted, callback)
}

// OnParticipantJoined registers a callback for participant joined events
func (rm *RoomManager) OnParticipantJoined(callback EventCallback) {
	rm.eventBus.Subscribe(EventParticipantJoined, callback)
}

// OnParticipantLeft registers a callback for participant left events
func (rm *RoomManager) OnParticipantLeft(callback EventCallback) {
	rm.eventBus.Subscribe(EventParticipantLeft, callback)
}

// OnTrackPublished registers a callback for track published events
func (rm *RoomManager) OnTrackPublished(callback EventCallback) {
	rm.eventBus.Subscribe(EventTrackPublished, callback)
}

// OnTrackUnpublished registers a callback for track unpublished events
func (rm *RoomManager) OnTrackUnpublished(callback EventCallback) {
	rm.eventBus.Subscribe(EventTrackUnpublished, callback)
}

// CleanupEmptyRooms removes all empty rooms with expired timeouts
func (rm *RoomManager) CleanupEmptyRooms() {
	rm.mu.RLock()
	roomsToDelete := make([]string, 0)

	for roomID, room := range rm.rooms {
		if room.IsEmpty() && room.EmptyTimeout > 0 {
			roomsToDelete = append(roomsToDelete, roomID)
		}
	}
	rm.mu.RUnlock()

	// Delete rooms outside of read lock
	for _, roomID := range roomsToDelete {
		rm.DeleteRoom(roomID)
	}
}

// Shutdown closes all rooms and cleans up resources
func (rm *RoomManager) Shutdown() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.logger.Info("Shutting down room manager",
		logger.Field{Key: "room_count", Value: len(rm.rooms)},
	)

	// Close all rooms
	for _, room := range rm.rooms {
		room.Close()
	}

	// Clear rooms map
	rm.rooms = make(map[string]*Room)

	// Clear event bus
	rm.eventBus.Clear()
}
