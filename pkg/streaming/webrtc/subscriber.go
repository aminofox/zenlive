// Package webrtc provides subscriber implementation for WebRTC stream playback.
package webrtc

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// Subscriber handles WebRTC stream subscription (playback)
type Subscriber struct {
	// id is the subscriber identifier
	id string

	// streamID is the stream identifier
	streamID string

	// peerManager manages peer connections
	peerManager *PeerManager

	// trackManager manages media tracks
	trackManager *TrackManager

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// videoTrack is the local video track
	videoTrack *webrtc.TrackLocalStaticRTP

	// audioTrack is the local audio track
	audioTrack *webrtc.TrackLocalStaticRTP

	// onSubscribeStart is called when subscription starts
	onSubscribeStart func()

	// onSubscribeStop is called when subscription stops
	onSubscribeStop func()

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// isSubscribed indicates if currently subscribed
	isSubscribed bool
}

// NewSubscriber creates a new WebRTC subscriber
func NewSubscriber(id, streamID string, pm *PeerManager, tm *TrackManager, log logger.Logger) *Subscriber {
	ctx, cancel := context.WithCancel(context.Background())

	return &Subscriber{
		id:           id,
		streamID:     streamID,
		peerManager:  pm,
		trackManager: tm,
		logger:       log,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// OnSubscribeStart sets the callback for subscribe start event
func (s *Subscriber) OnSubscribeStart(callback func()) {
	s.onSubscribeStart = callback
}

// OnSubscribeStop sets the callback for subscribe stop event
func (s *Subscriber) OnSubscribeStop(callback func()) {
	s.onSubscribeStop = callback
}

// Start starts the subscriber and creates tracks
func (s *Subscriber) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isSubscribed {
		s.mu.Unlock()
		return &WebRTCError{Code: "ALREADY_SUBSCRIBED", Message: "subscriber already started"}
	}
	s.isSubscribed = true
	s.mu.Unlock()

	// Create peer connection
	_, err := s.peerManager.CreatePeer(ctx, s.id, s.streamID, PeerRoleSubscriber)
	if err != nil {
		return fmt.Errorf("failed to create subscriber peer: %w", err)
	}

	// Create local tracks
	if err := s.createTracks(); err != nil {
		s.peerManager.RemovePeer(s.id)
		return fmt.Errorf("failed to create tracks: %w", err)
	}

	// Add tracks to peer connection
	if s.videoTrack != nil {
		if _, err := s.peerManager.AddTrack(s.id, s.videoTrack); err != nil {
			s.peerManager.RemovePeer(s.id)
			return fmt.Errorf("failed to add video track: %w", err)
		}
	}

	if s.audioTrack != nil {
		if _, err := s.peerManager.AddTrack(s.id, s.audioTrack); err != nil {
			s.peerManager.RemovePeer(s.id)
			return fmt.Errorf("failed to add audio track: %w", err)
		}
	}

	s.logger.Info("Subscriber started",
		logger.Field{Key: "subscriber_id", Value: s.id},
		logger.Field{Key: "stream_id", Value: s.streamID},
	)

	if s.onSubscribeStart != nil {
		go s.onSubscribeStart()
	}

	return nil
}

// Stop stops the subscriber
func (s *Subscriber) Stop() error {
	s.mu.Lock()
	if !s.isSubscribed {
		s.mu.Unlock()
		return &WebRTCError{Code: "NOT_SUBSCRIBED", Message: "subscriber not started"}
	}
	s.isSubscribed = false
	s.mu.Unlock()

	// Remove tracks
	if s.videoTrack != nil {
		s.trackManager.RemoveLocalTrack(s.videoTrack.ID())
	}
	if s.audioTrack != nil {
		s.trackManager.RemoveLocalTrack(s.audioTrack.ID())
	}

	// Remove peer connection
	if err := s.peerManager.RemovePeer(s.id); err != nil {
		s.logger.Error("Failed to remove subscriber peer",
			logger.Field{Key: "subscriber_id", Value: s.id},
			logger.Field{Key: "error", Value: err.Error()},
		)
	}

	s.cancel()

	s.logger.Info("Subscriber stopped",
		logger.Field{Key: "subscriber_id", Value: s.id},
	)

	if s.onSubscribeStop != nil {
		go s.onSubscribeStop()
	}

	return nil
}

// createTracks creates local tracks for the subscriber
func (s *Subscriber) createTracks() error {
	// Create video track
	videoTrack, err := s.trackManager.CreateLocalTrack(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeH264,
			ClockRate: 90000,
		},
		"video",
		s.streamID,
	)
	if err != nil {
		return fmt.Errorf("failed to create video track: %w", err)
	}

	s.mu.Lock()
	s.videoTrack = videoTrack
	s.mu.Unlock()

	// Create audio track
	audioTrack, err := s.trackManager.CreateLocalTrack(
		webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeOpus,
			ClockRate:   48000,
			Channels:    2,
			SDPFmtpLine: "minptime=10;useinbandfec=1",
		},
		"audio",
		s.streamID,
	)
	if err != nil {
		return fmt.Errorf("failed to create audio track: %w", err)
	}

	s.mu.Lock()
	s.audioTrack = audioTrack
	s.mu.Unlock()

	return nil
}

// WriteVideoPacket writes a video RTP packet to the subscriber
func (s *Subscriber) WriteVideoPacket(packet *rtp.Packet) error {
	s.mu.RLock()
	track := s.videoTrack
	s.mu.RUnlock()

	if track == nil {
		return &WebRTCError{Code: "NO_VIDEO_TRACK", Message: "no video track available"}
	}

	return WriteRTPToTrack(track, packet)
}

// WriteAudioPacket writes an audio RTP packet to the subscriber
func (s *Subscriber) WriteAudioPacket(packet *rtp.Packet) error {
	s.mu.RLock()
	track := s.audioTrack
	s.mu.RUnlock()

	if track == nil {
		return &WebRTCError{Code: "NO_AUDIO_TRACK", Message: "no audio track available"}
	}

	return WriteRTPToTrack(track, packet)
}

// GetVideoTrack returns the video track
func (s *Subscriber) GetVideoTrack() *webrtc.TrackLocalStaticRTP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.videoTrack
}

// GetAudioTrack returns the audio track
func (s *Subscriber) GetAudioTrack() *webrtc.TrackLocalStaticRTP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.audioTrack
}

// IsSubscribed returns whether the subscriber is active
func (s *Subscriber) IsSubscribed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isSubscribed
}

// GetID returns the subscriber ID
func (s *Subscriber) GetID() string {
	return s.id
}

// GetStreamID returns the stream ID
func (s *Subscriber) GetStreamID() string {
	return s.streamID
}

// HandleOffer creates an offer for the subscriber
func (s *Subscriber) HandleOffer() (*webrtc.SessionDescription, error) {
	// Create offer
	offer, err := s.peerManager.CreateOffer(s.id)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	s.logger.Info("Created offer for subscriber",
		logger.Field{Key: "subscriber_id", Value: s.id},
	)

	return offer, nil
}

// HandleAnswer handles an SDP answer from the subscriber
func (s *Subscriber) HandleAnswer(answer webrtc.SessionDescription) error {
	// Set remote description
	if err := s.peerManager.SetRemoteDescription(s.id, answer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	s.logger.Info("Set answer for subscriber",
		logger.Field{Key: "subscriber_id", Value: s.id},
	)

	return nil
}

// HandleICECandidate handles an ICE candidate from the subscriber
func (s *Subscriber) HandleICECandidate(candidate webrtc.ICECandidateInit) error {
	if err := s.peerManager.AddICECandidate(s.id, candidate); err != nil {
		return fmt.Errorf("failed to add ICE candidate: %w", err)
	}

	return nil
}

// GetPeerConnection returns the peer connection
func (s *Subscriber) GetPeerConnection() (*PeerConnection, error) {
	return s.peerManager.GetPeer(s.id)
}
