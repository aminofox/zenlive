package room

import (
	"sync"
	"time"
)

// Participant represents a participant in a room
type Participant struct {
	// ID is the unique participant identifier
	ID string `json:"id"`
	// UserID is the user identifier (from authentication)
	UserID string `json:"user_id"`
	// Username is the display name
	Username string `json:"username"`
	// JoinedAt is when the participant joined the room
	JoinedAt time.Time `json:"joined_at"`
	// Role is the participant's role in the room
	Role ParticipantRole `json:"role"`
	// Permissions define what the participant can do
	Permissions ParticipantPermissions `json:"permissions"`
	// State is the current connection state
	State ParticipantState `json:"state"`
	// Metadata contains custom participant data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Token-based permissions (from access token)
	CanPublish     bool `json:"can_publish"`
	CanSubscribe   bool `json:"can_subscribe"`
	CanPublishData bool `json:"can_publish_data"`
	IsAdmin        bool `json:"is_admin"`
	IsHidden       bool `json:"is_hidden"`
	IsRecorder     bool `json:"is_recorder"`

	// tracks stores published tracks by track ID
	tracks map[string]*MediaTrack
	// mu protects concurrent access
	mu sync.RWMutex
}

// NewParticipant creates a new participant
func NewParticipant(id, userID, username string, role ParticipantRole) *Participant {
	return &Participant{
		ID:          id,
		UserID:      userID,
		Username:    username,
		JoinedAt:    time.Now(),
		Role:        role,
		Permissions: DefaultPermissions(role),
		State:       StateJoining,
		Metadata:    make(map[string]interface{}),
		tracks:      make(map[string]*MediaTrack),
	}
}

// AddTrack adds a media track to the participant
func (p *Participant) AddTrack(track *MediaTrack) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tracks[track.ID] = track
}

// RemoveTrack removes a media track from the participant
func (p *Participant) RemoveTrack(trackID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.tracks[trackID]; exists {
		delete(p.tracks, trackID)
		return true
	}
	return false
}

// GetTrack returns a track by ID
func (p *Participant) GetTrack(trackID string) (*MediaTrack, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	track, exists := p.tracks[trackID]
	return track, exists
}

// GetTracks returns all tracks for this participant
func (p *Participant) GetTracks() []*MediaTrack {
	p.mu.RLock()
	defer p.mu.RUnlock()
	tracks := make([]*MediaTrack, 0, len(p.tracks))
	for _, track := range p.tracks {
		tracks = append(tracks, track)
	}
	return tracks
}

// UpdateState updates the participant's state
func (p *Participant) UpdateState(state ParticipantState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.State = state
}

// UpdatePermissions updates the participant's permissions
func (p *Participant) UpdatePermissions(perms ParticipantPermissions) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Permissions = perms
}

// UpdateMetadata updates participant metadata
func (p *Participant) UpdateMetadata(metadata map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, value := range metadata {
		p.Metadata[key] = value
	}
}

// GetMetadata returns a copy of participant metadata
func (p *Participant) GetMetadata() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	metadata := make(map[string]interface{})
	for key, value := range p.Metadata {
		metadata[key] = value
	}
	return metadata
}

// GetState returns the participant's current state
func (p *Participant) GetState() ParticipantState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// GetPermissions returns a copy of the participant's permissions
func (p *Participant) GetPermissions() ParticipantPermissions {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Permissions
}
