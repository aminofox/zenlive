package room

import (
	"errors"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/google/uuid"
)

var (
	// ErrRoomFull is returned when trying to join a full room
	ErrRoomFull = errors.New("room is full")
	// ErrParticipantNotFound is returned when participant doesn't exist
	ErrParticipantNotFound = errors.New("participant not found")
	// ErrParticipantExists is returned when participant already exists
	ErrParticipantExists = errors.New("participant already exists in room")
	// ErrUnauthorized is returned when participant lacks permission
	ErrUnauthorized = errors.New("participant lacks required permissions")
)

// Room represents a video conferencing room
type Room struct {
	// ID is the unique room identifier
	ID string `json:"id"`
	// Name is the room display name
	Name string `json:"name"`
	// CreatedAt is when the room was created
	CreatedAt time.Time `json:"created_at"`
	// CreatedBy is the user ID who created the room
	CreatedBy string `json:"created_by"`
	// MaxParticipants is the maximum number of participants (0 = unlimited)
	MaxParticipants int `json:"max_participants"`
	// EmptyTimeout is how long to keep room alive when empty (0 = never delete)
	EmptyTimeout time.Duration `json:"empty_timeout"`
	// Metadata contains custom room data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// participants stores participants by participant ID
	participants map[string]*Participant
	// logger for room events
	logger logger.Logger
	// eventBus for publishing events
	eventBus *EventBus
	// mu protects concurrent access
	mu sync.RWMutex
	// emptyTimer tracks when room became empty
	emptyTimer *time.Timer
	// isClosed indicates if the room is closed
	isClosed bool
}

// NewRoom creates a new room
func NewRoom(req *CreateRoomRequest, createdBy string, log logger.Logger, eventBus *EventBus) *Room {
	roomID := uuid.New().String()

	room := &Room{
		ID:              roomID,
		Name:            req.Name,
		CreatedAt:       time.Now(),
		CreatedBy:       createdBy,
		MaxParticipants: req.MaxParticipants,
		EmptyTimeout:    req.EmptyTimeout,
		Metadata:        req.Metadata,
		participants:    make(map[string]*Participant),
		logger:          log,
		eventBus:        eventBus,
		isClosed:        false,
	}

	if room.Metadata == nil {
		room.Metadata = make(map[string]interface{})
	}

	return room
}

// AddParticipant adds a participant to the room
func (r *Room) AddParticipant(p *Participant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return errors.New("room is closed")
	}

	// Check if participant already exists
	if _, exists := r.participants[p.ID]; exists {
		return ErrParticipantExists
	}

	// Check if room is full
	if r.MaxParticipants > 0 && len(r.participants) >= r.MaxParticipants {
		return ErrRoomFull
	}

	// Stop empty timer if it's running
	if r.emptyTimer != nil {
		r.emptyTimer.Stop()
		r.emptyTimer = nil
	}

	// Add participant
	r.participants[p.ID] = p
	p.UpdateState(StateJoined)

	r.logger.Info("Participant joined room",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "participant_id", Value: p.ID},
		logger.Field{Key: "username", Value: p.Username},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventParticipantJoined, r.ID, p))
	}

	return nil
}

// RemoveParticipant removes a participant from the room
func (r *Room) RemoveParticipant(participantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	participant, exists := r.participants[participantID]
	if !exists {
		return ErrParticipantNotFound
	}

	// Remove participant
	delete(r.participants, participantID)
	participant.UpdateState(StateDisconnected)

	r.logger.Info("Participant left room",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "participant_id", Value: participantID},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventParticipantLeft, r.ID, participant))
	}

	// Start empty timer if room is now empty and has timeout configured
	if len(r.participants) == 0 && r.EmptyTimeout > 0 && !r.isClosed {
		r.startEmptyTimer()
	}

	return nil
}

// GetParticipant returns a participant by ID
func (r *Room) GetParticipant(participantID string) (*Participant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	participant, exists := r.participants[participantID]
	if !exists {
		return nil, ErrParticipantNotFound
	}

	return participant, nil
}

// ListParticipants returns all participants in the room
func (r *Room) ListParticipants() []*Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	participants := make([]*Participant, 0, len(r.participants))
	for _, p := range r.participants {
		participants = append(participants, p)
	}

	return participants
}

// GetParticipantCount returns the number of participants in the room
func (r *Room) GetParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.participants)
}

// UpdateParticipantPermissions updates a participant's permissions
func (r *Room) UpdateParticipantPermissions(participantID string, perms ParticipantPermissions) error {
	r.mu.RLock()
	participant, exists := r.participants[participantID]
	r.mu.RUnlock()

	if !exists {
		return ErrParticipantNotFound
	}

	participant.UpdatePermissions(perms)

	r.logger.Info("Participant permissions updated",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "participant_id", Value: participantID},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventParticipantUpdated, r.ID, participant))
	}

	return nil
}

// UpdateMetadata updates room metadata
func (r *Room) UpdateMetadata(metadata map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, value := range metadata {
		r.Metadata[key] = value
	}

	r.logger.Info("Room metadata updated",
		logger.Field{Key: "room_id", Value: r.ID},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventMetadataUpdated, r.ID, metadata))
	}
}

// PublishTrack publishes a media track for a participant
func (r *Room) PublishTrack(participantID string, track *MediaTrack) error {
	r.mu.RLock()
	participant, exists := r.participants[participantID]
	r.mu.RUnlock()

	if !exists {
		return ErrParticipantNotFound
	}

	// Check permissions
	if !participant.GetPermissions().CanPublish {
		return ErrUnauthorized
	}

	participant.AddTrack(track)

	r.logger.Info("Track published",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "participant_id", Value: participantID},
		logger.Field{Key: "track_id", Value: track.ID},
		logger.Field{Key: "track_kind", Value: track.Kind},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventTrackPublished, r.ID, track))
	}

	return nil
}

// UnpublishTrack unpublishes a media track
func (r *Room) UnpublishTrack(participantID, trackID string) error {
	r.mu.RLock()
	participant, exists := r.participants[participantID]
	r.mu.RUnlock()

	if !exists {
		return ErrParticipantNotFound
	}

	track, trackExists := participant.GetTrack(trackID)
	if !trackExists {
		return errors.New("track not found")
	}

	participant.RemoveTrack(trackID)

	r.logger.Info("Track unpublished",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "participant_id", Value: participantID},
		logger.Field{Key: "track_id", Value: trackID},
	)

	// Publish event
	if r.eventBus != nil {
		r.eventBus.Publish(createEvent(EventTrackUnpublished, r.ID, track))
	}

	return nil
}

// Close closes the room and disconnects all participants
func (r *Room) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return
	}

	r.isClosed = true

	// Stop empty timer if running
	if r.emptyTimer != nil {
		r.emptyTimer.Stop()
		r.emptyTimer = nil
	}

	// Clear all participants
	r.participants = make(map[string]*Participant)

	r.logger.Info("Room closed",
		logger.Field{Key: "room_id", Value: r.ID},
	)
}

// IsClosed returns whether the room is closed
func (r *Room) IsClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.isClosed
}

// IsEmpty returns whether the room has no participants
func (r *Room) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.participants) == 0
}

// startEmptyTimer starts the empty room timer
func (r *Room) startEmptyTimer() {
	if r.emptyTimer != nil {
		r.emptyTimer.Stop()
	}

	r.logger.Info("Starting empty room timer",
		logger.Field{Key: "room_id", Value: r.ID},
		logger.Field{Key: "timeout", Value: r.EmptyTimeout.String()},
	)

	r.emptyTimer = time.AfterFunc(r.EmptyTimeout, func() {
		r.logger.Info("Room empty timeout triggered",
			logger.Field{Key: "room_id", Value: r.ID},
		)
		// Note: Room will be deleted by RoomManager
	})
}
