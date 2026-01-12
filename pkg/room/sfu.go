package room

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/streaming/webrtc"
)

// RoomSFU connects a Room with WebRTC SFU for real-time media streaming
type RoomSFU struct {
	// room is the associated room
	room *Room

	// sfu is the WebRTC SFU instance
	sfu *webrtc.SFU

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// publishers maps participant ID to their publisher
	publishers map[string]*webrtc.Publisher

	// subscribers maps participant ID to their subscriptions (subscriberID -> publisher ID -> subscriber)
	subscribers map[string]map[string]*webrtc.Subscriber

	// tracks maps track ID to track info
	tracks map[string]*MediaTrack

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRoomSFU creates a new RoomSFU instance
func NewRoomSFU(room *Room, sfu *webrtc.SFU, log logger.Logger) *RoomSFU {
	ctx, cancel := context.WithCancel(context.Background())

	rs := &RoomSFU{
		room:        room,
		sfu:         sfu,
		logger:      log,
		publishers:  make(map[string]*webrtc.Publisher),
		subscribers: make(map[string]map[string]*webrtc.Subscriber),
		tracks:      make(map[string]*MediaTrack),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start cleanup goroutine
	go rs.startCleanup()

	return rs
}

// startCleanup starts background cleanup tasks
func (rs *RoomSFU) startCleanup() {
	// Room cleanup or monitoring could go here
}

// PublishTrack publishes a media track for a participant
func (rs *RoomSFU) PublishTrack(participantID, trackID, kind, label string) (string, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Verify participant exists in room
	participant, err := rs.room.GetParticipant(participantID)
	if err != nil {
		return "", err
	}

	// Check if participant has permission to publish
	if !participant.Permissions.CanPublish {
		return "", errors.New(errors.ErrCodeUnauthorized, "participant does not have permission to publish")
	}

	// Create track info
	track := &MediaTrack{
		ID:            trackID,
		ParticipantID: participantID,
		Kind:          kind,
		Source:        label, // Use label as source
	}

	rs.tracks[trackID] = track

	rs.logger.Info("Track published",
		logger.String("room_id", rs.room.ID),
		logger.String("participant_id", participantID),
		logger.String("track_id", trackID),
		logger.String("kind", kind),
	)

	// Auto-subscribe other participants to this track
	rs.autoSubscribeToNewTrack(participantID, trackID)

	return trackID, nil
}

// UnpublishTrack unpublishes a media track
func (rs *RoomSFU) UnpublishTrack(participantID, trackID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	track, exists := rs.tracks[trackID]
	if !exists {
		return errors.NewNotFoundError(fmt.Sprintf("track %s", trackID))
	}

	if track.ParticipantID != participantID {
		return errors.New(errors.ErrCodeUnauthorized, "participant does not own this track")
	}

	delete(rs.tracks, trackID)

	rs.logger.Info("Track unpublished",
		logger.String("room_id", rs.room.ID),
		logger.String("participant_id", participantID),
		logger.String("track_id", trackID),
	)

	return nil
}

// GetParticipantTracks returns all tracks published by a participant
func (rs *RoomSFU) GetParticipantTracks(participantID string) []*MediaTrack {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var tracks []*MediaTrack
	for _, track := range rs.tracks {
		if track.ParticipantID == participantID {
			tracks = append(tracks, track)
		}
	}

	return tracks
}

// autoSubscribeToNewTrack automatically subscribes other participants to a new track
func (rs *RoomSFU) autoSubscribeToNewTrack(publisherID, trackID string) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// Get all participants except the publisher
	for participantID := range rs.publishers {
		if participantID == publisherID {
			continue
		}

		// Check if participant exists in room
		participant, err := rs.room.GetParticipant(participantID)
		if err != nil {
			continue
		}

		// Check if participant has permission to subscribe
		if !participant.Permissions.CanSubscribe {
			continue
		}

		rs.logger.Debug("Auto-subscribing participant to track",
			logger.String("subscriber_id", participant.ID),
			logger.String("publisher_id", publisherID),
			logger.String("track_id", trackID),
		)
	}
}

// autoSubscribeToExistingTracks automatically subscribes a new participant to existing tracks
func (rs *RoomSFU) autoSubscribeToExistingTracks(participantID string) {
	participant, err := rs.room.GetParticipant(participantID)
	if err != nil {
		return
	}

	// Check if participant has permission to subscribe
	if !participant.Permissions.CanSubscribe {
		return
	}

	// Subscribe to all existing tracks
	for trackID, track := range rs.tracks {
		if track.ParticipantID == participantID {
			// Don't subscribe to own tracks
			continue
		}

		rs.logger.Debug("Auto-subscribing new participant to existing track",
			logger.String("subscriber_id", participantID),
			logger.String("publisher_id", track.ParticipantID),
			logger.String("track_id", trackID),
		)
	}
}

// OnParticipantJoined should be called when a participant joins the room
func (rs *RoomSFU) OnParticipantJoined(participantID string) {
	rs.logger.Info("Participant joined, setting up WebRTC",
		logger.String("room_id", rs.room.ID),
		logger.String("participant_id", participantID),
	)

	// Auto-subscribe to existing tracks
	rs.autoSubscribeToExistingTracks(participantID)
}

// OnParticipantLeft should be called when a participant leaves the room
func (rs *RoomSFU) OnParticipantLeft(participantID string) {
	rs.logger.Info("Participant left, cleaning up WebRTC",
		logger.String("room_id", rs.room.ID),
		logger.String("participant_id", participantID),
	)

	rs.cleanupParticipant(participantID)
}

// cleanupParticipant cleans up WebRTC resources for a participant
func (rs *RoomSFU) cleanupParticipant(participantID string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Clean up publisher
	if publisher, exists := rs.publishers[participantID]; exists {
		publisher.Stop()
		delete(rs.publishers, participantID)
	}

	// Clean up subscribers
	if subs, exists := rs.subscribers[participantID]; exists {
		for _, sub := range subs {
			sub.Stop()
		}
		delete(rs.subscribers, participantID)
	}

	// Clean up tracks
	for trackID, track := range rs.tracks {
		if track.ParticipantID == participantID {
			delete(rs.tracks, trackID)
		}
	}

	rs.logger.Debug("Cleaned up participant WebRTC resources",
		logger.String("participant_id", participantID),
	)
}

// Close closes the RoomSFU and cleans up all resources
func (rs *RoomSFU) Close() error {
	rs.cancel()

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Clean up all publishers
	for _, publisher := range rs.publishers {
		publisher.Stop()
	}

	// Clean up all subscribers
	for _, subs := range rs.subscribers {
		for _, sub := range subs {
			sub.Stop()
		}
	}

	rs.logger.Info("RoomSFU closed",
		logger.String("room_id", rs.room.ID),
	)

	return nil
}
