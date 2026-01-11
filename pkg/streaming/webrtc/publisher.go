// Package webrtc provides publisher implementation for WebRTC stream ingestion.
package webrtc

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// Publisher handles WebRTC stream publishing (ingestion)
type Publisher struct {
	// id is the publisher identifier
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

	// videoTrack is the received video track
	videoTrack *webrtc.TrackRemote

	// audioTrack is the received audio track
	audioTrack *webrtc.TrackRemote

	// onVideoPacket is called for each video RTP packet
	onVideoPacket func(*rtp.Packet)

	// onAudioPacket is called for each audio RTP packet
	onAudioPacket func(*rtp.Packet)

	// onPublishStart is called when publishing starts
	onPublishStart func()

	// onPublishStop is called when publishing stops
	onPublishStop func()

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// isPublishing indicates if currently publishing
	isPublishing bool
}

// NewPublisher creates a new WebRTC publisher
func NewPublisher(id, streamID string, pm *PeerManager, tm *TrackManager, log logger.Logger) *Publisher {
	ctx, cancel := context.WithCancel(context.Background())

	return &Publisher{
		id:           id,
		streamID:     streamID,
		peerManager:  pm,
		trackManager: tm,
		logger:       log,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// OnVideoPacket sets the callback for video RTP packets
func (p *Publisher) OnVideoPacket(callback func(*rtp.Packet)) {
	p.onVideoPacket = callback
}

// OnAudioPacket sets the callback for audio RTP packets
func (p *Publisher) OnAudioPacket(callback func(*rtp.Packet)) {
	p.onAudioPacket = callback
}

// OnPublishStart sets the callback for publish start event
func (p *Publisher) OnPublishStart(callback func()) {
	p.onPublishStart = callback
}

// OnPublishStop sets the callback for publish stop event
func (p *Publisher) OnPublishStop(callback func()) {
	p.onPublishStop = callback
}

// Start starts the publisher and creates a peer connection
func (p *Publisher) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.isPublishing {
		p.mu.Unlock()
		return &WebRTCError{Code: "ALREADY_PUBLISHING", Message: "publisher already started"}
	}
	p.isPublishing = true
	p.mu.Unlock()

	// Create peer connection
	_, err := p.peerManager.CreatePeer(ctx, p.id, p.streamID, PeerRolePublisher)
	if err != nil {
		return fmt.Errorf("failed to create publisher peer: %w", err)
	}

	// Set up track handler
	p.peerManager.OnTrack(func(peerID string, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if peerID != p.id {
			return
		}

		p.handleTrack(track, receiver)
	})

	p.logger.Info("Publisher started",
		logger.Field{Key: "publisher_id", Value: p.id},
		logger.Field{Key: "stream_id", Value: p.streamID},
	)

	if p.onPublishStart != nil {
		go p.onPublishStart()
	}

	return nil
}

// Stop stops the publisher
func (p *Publisher) Stop() error {
	p.mu.Lock()
	if !p.isPublishing {
		p.mu.Unlock()
		return &WebRTCError{Code: "NOT_PUBLISHING", Message: "publisher not started"}
	}
	p.isPublishing = false
	p.mu.Unlock()

	// Stop track readers
	if p.videoTrack != nil {
		p.trackManager.StopTrackReader(p.videoTrack.ID())
	}
	if p.audioTrack != nil {
		p.trackManager.StopTrackReader(p.audioTrack.ID())
	}

	// Remove peer connection
	if err := p.peerManager.RemovePeer(p.id); err != nil {
		p.logger.Error("Failed to remove publisher peer",
			logger.Field{Key: "publisher_id", Value: p.id},
			logger.Field{Key: "error", Value: err.Error()},
		)
	}

	p.cancel()

	p.logger.Info("Publisher stopped",
		logger.Field{Key: "publisher_id", Value: p.id},
	)

	if p.onPublishStop != nil {
		go p.onPublishStop()
	}

	return nil
}

// handleTrack handles received tracks from the publisher
func (p *Publisher) handleTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	p.logger.Info("Received track from publisher",
		logger.Field{Key: "publisher_id", Value: p.id},
		logger.Field{Key: "track_id", Value: track.ID()},
		logger.Field{Key: "kind", Value: track.Kind().String()},
		logger.Field{Key: "codec", Value: track.Codec().MimeType},
	)

	// Add track to manager
	p.trackManager.AddRemoteTrack(track)

	// Handle based on track kind
	if track.Kind() == webrtc.RTPCodecTypeVideo {
		p.mu.Lock()
		p.videoTrack = track
		p.mu.Unlock()

		// Start reading video packets
		p.trackManager.StartTrackReader(p.ctx, track, func(packet *rtp.Packet) {
			if p.onVideoPacket != nil {
				p.onVideoPacket(packet)
			}
		})

	} else if track.Kind() == webrtc.RTPCodecTypeAudio {
		p.mu.Lock()
		p.audioTrack = track
		p.mu.Unlock()

		// Start reading audio packets
		p.trackManager.StartTrackReader(p.ctx, track, func(packet *rtp.Packet) {
			if p.onAudioPacket != nil {
				p.onAudioPacket(packet)
			}
		})
	}

	// Handle RTCP feedback
	go p.handleRTCP(receiver)
}

// handleRTCP handles RTCP feedback for a track
func (p *Publisher) handleRTCP(receiver *webrtc.RTPReceiver) {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Read RTCP packets
		packets, _, err := receiver.ReadRTCP()
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			continue
		}

		// Process RTCP packets (for future extension)
		for _, pkt := range packets {
			p.logger.Debug("Received RTCP packet",
				logger.Field{Key: "publisher_id", Value: p.id},
				logger.Field{Key: "type", Value: fmt.Sprintf("%T", pkt)},
			)
		}
	}
}

// GetVideoTrack returns the video track
func (p *Publisher) GetVideoTrack() *webrtc.TrackRemote {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.videoTrack
}

// GetAudioTrack returns the audio track
func (p *Publisher) GetAudioTrack() *webrtc.TrackRemote {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.audioTrack
}

// IsPublishing returns whether the publisher is active
func (p *Publisher) IsPublishing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.isPublishing
}

// GetID returns the publisher ID
func (p *Publisher) GetID() string {
	return p.id
}

// GetStreamID returns the stream ID
func (p *Publisher) GetStreamID() string {
	return p.streamID
}

// GetStats returns publisher statistics
func (p *Publisher) GetStats() map[string]*TrackStats {
	stats := make(map[string]*TrackStats)

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.videoTrack != nil {
		if s := p.trackManager.GetTrackStats(p.videoTrack.ID()); s != nil {
			stats["video"] = s
		}
	}

	if p.audioTrack != nil {
		if s := p.trackManager.GetTrackStats(p.audioTrack.ID()); s != nil {
			stats["audio"] = s
		}
	}

	return stats
}

// HandleOffer handles an SDP offer from the publisher
func (p *Publisher) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	// Set remote description
	if err := p.peerManager.SetRemoteDescription(p.id, offer); err != nil {
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := p.peerManager.CreateAnswer(p.id)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	p.logger.Info("Created answer for publisher",
		logger.Field{Key: "publisher_id", Value: p.id},
	)

	return answer, nil
}

// HandleICECandidate handles an ICE candidate from the publisher
func (p *Publisher) HandleICECandidate(candidate webrtc.ICECandidateInit) error {
	if err := p.peerManager.AddICECandidate(p.id, candidate); err != nil {
		return fmt.Errorf("failed to add ICE candidate: %w", err)
	}

	return nil
}
