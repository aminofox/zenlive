// Package room provides session management for multi-room participation
package room

import (
	"fmt"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

// UserSession represents a user's session across multiple rooms
type UserSession struct {
	// UserID is the user identifier
	UserID string

	// ActiveRooms maps room ID to room participation
	ActiveRooms map[string]*RoomParticipation

	// TrackCount is the total number of tracks across all rooms
	TrackCount int

	// TotalBandwidth is the total allocated bandwidth in bits/sec
	TotalBandwidth uint64

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// LastActivity is the last activity timestamp
	LastActivity time.Time

	// mu protects concurrent access
	mu sync.RWMutex
}

// RoomParticipation represents a user's participation in a single room
type RoomParticipation struct {
	// RoomID is the room identifier
	RoomID string

	// ParticipantID is the participant ID in the room
	ParticipantID string

	// Publisher for publishing tracks
	Publisher *webrtc.Publisher

	// Subscribers for subscribing to tracks
	Subscribers map[string]*webrtc.Subscriber

	// AllocatedBandwidth in bits/sec
	AllocatedBandwidth uint64

	// JoinedAt is when the user joined this room
	JoinedAt time.Time
}

// SessionManager manages user sessions across multiple rooms
type SessionManager struct {
	// config is the session manager configuration
	config SessionManagerConfig

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// sessions maps user ID to their session
	sessions map[string]*UserSession
}

// SessionManagerConfig contains configuration for the session manager
type SessionManagerConfig struct {
	// MaxRoomsPerUser is the maximum number of rooms a user can join
	MaxRoomsPerUser int

	// MaxTracksPerUser is the maximum number of tracks a user can have
	MaxTracksPerUser int

	// MaxBandwidthPerUser is the maximum bandwidth per user in bits/sec
	MaxBandwidthPerUser uint64

	// SessionTimeout is how long to keep inactive sessions
	SessionTimeout time.Duration

	// BandwidthAllocationStrategy determines how to allocate bandwidth across rooms
	// Options: "equal", "proportional", "priority"
	BandwidthAllocationStrategy string
}

// DefaultSessionManagerConfig returns the default configuration
func DefaultSessionManagerConfig() SessionManagerConfig {
	return SessionManagerConfig{
		MaxRoomsPerUser:             5,
		MaxTracksPerUser:            10,
		MaxBandwidthPerUser:         10 * 1024 * 1024, // 10 Mbps
		SessionTimeout:              30 * time.Minute,
		BandwidthAllocationStrategy: "equal",
	}
}

// NewSessionManager creates a new session manager
func NewSessionManager(config SessionManagerConfig, log logger.Logger) *SessionManager {
	return &SessionManager{
		config:   config,
		logger:   log,
		sessions: make(map[string]*UserSession),
	}
}

// CreateSession creates a new user session
func (sm *SessionManager) CreateSession(userID string) *UserSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &UserSession{
		UserID:       userID,
		ActiveRooms:  make(map[string]*RoomParticipation),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	sm.sessions[userID] = session

	sm.logger.Info("Created user session",
		logger.String("user_id", userID),
	)

	return session
}

// GetSession returns a user's session
func (sm *SessionManager) GetSession(userID string) (*UserSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return nil, errors.NewSessionNotFoundError(userID)
	}

	return session, nil
}

// JoinRoom adds a room to a user's session
func (sm *SessionManager) JoinRoom(userID, roomID, participantID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		session = &UserSession{
			UserID:       userID,
			ActiveRooms:  make(map[string]*RoomParticipation),
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}
		sm.sessions[userID] = session
	}

	// Check room limit
	if len(session.ActiveRooms) >= sm.config.MaxRoomsPerUser {
		return errors.NewRoomLimitExceededError(sm.config.MaxRoomsPerUser)
	}

	// Create room participation
	participation := &RoomParticipation{
		RoomID:        roomID,
		ParticipantID: participantID,
		Subscribers:   make(map[string]*webrtc.Subscriber),
		JoinedAt:      time.Now(),
	}

	session.ActiveRooms[roomID] = participation
	session.LastActivity = time.Now()

	// Reallocate bandwidth across all rooms
	sm.reallocateBandwidth(session)

	sm.logger.Info("User joined room",
		logger.String("user_id", userID),
		logger.String("room_id", roomID),
		logger.Field{Key: "active_rooms", Value: len(session.ActiveRooms)},
	)

	return nil
}

// LeaveRoom removes a room from a user's session
func (sm *SessionManager) LeaveRoom(userID, roomID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return errors.NewSessionNotFoundError(userID)
	}

	participation, exists := session.ActiveRooms[roomID]
	if !exists {
		return errors.NewNotFoundError(fmt.Sprintf("room participation for room %s", roomID))
	}

	// Clean up WebRTC resources
	if participation.Publisher != nil {
		participation.Publisher.Stop()
	}

	for _, sub := range participation.Subscribers {
		sub.Stop()
	}

	delete(session.ActiveRooms, roomID)
	session.LastActivity = time.Now()

	// Reallocate bandwidth
	sm.reallocateBandwidth(session)

	sm.logger.Info("User left room",
		logger.String("user_id", userID),
		logger.String("room_id", roomID),
		logger.Field{Key: "active_rooms", Value: len(session.ActiveRooms)},
	)

	return nil
}

// AddTrack adds a track to a user's session
func (sm *SessionManager) AddTrack(userID, roomID, trackID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return errors.NewSessionNotFoundError(userID)
	}

	// Check track limit
	if session.TrackCount >= sm.config.MaxTracksPerUser {
		return errors.NewTrackLimitExceededError(sm.config.MaxTracksPerUser)
	}

	session.TrackCount++
	session.LastActivity = time.Now()

	sm.logger.Debug("Added track to session",
		logger.String("user_id", userID),
		logger.String("room_id", roomID),
		logger.String("track_id", trackID),
		logger.Field{Key: "total_tracks", Value: session.TrackCount},
	)

	return nil
}

// RemoveTrack removes a track from a user's session
func (sm *SessionManager) RemoveTrack(userID, trackID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return errors.NewSessionNotFoundError(userID)
	}

	if session.TrackCount > 0 {
		session.TrackCount--
	}
	session.LastActivity = time.Now()

	sm.logger.Debug("Removed track from session",
		logger.String("user_id", userID),
		logger.String("track_id", trackID),
		logger.Field{Key: "total_tracks", Value: session.TrackCount},
	)

	return nil
}

// reallocateBandwidth reallocates bandwidth across all rooms in a session
func (sm *SessionManager) reallocateBandwidth(session *UserSession) {
	numRooms := len(session.ActiveRooms)
	if numRooms == 0 {
		session.TotalBandwidth = 0
		return
	}

	switch sm.config.BandwidthAllocationStrategy {
	case "equal":
		// Divide equally
		perRoom := sm.config.MaxBandwidthPerUser / uint64(numRooms)
		for _, participation := range session.ActiveRooms {
			participation.AllocatedBandwidth = perRoom
		}

	case "proportional":
		// Allocate proportionally based on number of subscribers
		// Implementation would track subscriber counts
		// For now, fall back to equal
		perRoom := sm.config.MaxBandwidthPerUser / uint64(numRooms)
		for _, participation := range session.ActiveRooms {
			participation.AllocatedBandwidth = perRoom
		}

	case "priority":
		// Allocate based on room priority
		// Implementation would use room priority metadata
		// For now, fall back to equal
		perRoom := sm.config.MaxBandwidthPerUser / uint64(numRooms)
		for _, participation := range session.ActiveRooms {
			participation.AllocatedBandwidth = perRoom
		}

	default:
		// Default to equal
		perRoom := sm.config.MaxBandwidthPerUser / uint64(numRooms)
		for _, participation := range session.ActiveRooms {
			participation.AllocatedBandwidth = perRoom
		}
	}

	session.TotalBandwidth = sm.config.MaxBandwidthPerUser
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*UserSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*UserSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// CleanupInactiveSessions removes sessions that haven't been active
func (sm *SessionManager) CleanupInactiveSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for userID, session := range sm.sessions {
		session.mu.RLock()
		inactive := now.Sub(session.LastActivity) > sm.config.SessionTimeout
		numRooms := len(session.ActiveRooms)
		session.mu.RUnlock()

		if inactive && numRooms == 0 {
			delete(sm.sessions, userID)
			cleaned++

			sm.logger.Info("Cleaned up inactive session",
				logger.String("user_id", userID),
			)
		}
	}

	return cleaned
}

// GetSessionStats returns statistics about sessions
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	totalSessions := len(sm.sessions)
	totalRooms := 0
	totalTracks := 0

	for _, session := range sm.sessions {
		session.mu.RLock()
		totalRooms += len(session.ActiveRooms)
		totalTracks += session.TrackCount
		session.mu.RUnlock()
	}

	return map[string]interface{}{
		"total_sessions": totalSessions,
		"total_rooms":    totalRooms,
		"total_tracks":   totalTracks,
		"avg_rooms_per_session": func() float64 {
			if totalSessions == 0 {
				return 0
			}
			return float64(totalRooms) / float64(totalSessions)
		}(),
	}
}
