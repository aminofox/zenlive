// Package webrtc provides peer connection management for WebRTC streaming.
package webrtc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/webrtc/v3"
)

// PeerManager manages WebRTC peer connections
type PeerManager struct {
	// config is the WebRTC configuration
	config Config

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// peers stores peer connections by peer ID
	peers map[string]*PeerConnection

	// iceGatherer for ICE candidate gathering
	iceGatherer *ICEGatherer

	// iceStateHandler for ICE state changes
	iceStateHandler *ICEConnectionStateHandler

	// onTrack is called when a new track is received
	onTrack func(peerID string, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)

	// onConnectionStateChange is called when connection state changes
	onConnectionStateChange func(peerID string, state webrtc.PeerConnectionState)

	// onICEConnectionStateChange is called when ICE connection state changes
	onICEConnectionStateChange func(peerID string, state webrtc.ICEConnectionState)
}

// NewPeerManager creates a new peer manager
func NewPeerManager(config Config, log logger.Logger) *PeerManager {
	return &PeerManager{
		config:          config,
		logger:          log,
		peers:           make(map[string]*PeerConnection),
		iceGatherer:     NewICEGatherer(config, log),
		iceStateHandler: NewICEConnectionStateHandler(log),
	}
}

// OnTrack sets the callback for new tracks
func (pm *PeerManager) OnTrack(callback func(peerID string, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)) {
	pm.onTrack = callback
}

// OnConnectionStateChange sets the callback for connection state changes
func (pm *PeerManager) OnConnectionStateChange(callback func(peerID string, state webrtc.PeerConnectionState)) {
	pm.onConnectionStateChange = callback
}

// OnICEConnectionStateChange sets the callback for ICE connection state changes
func (pm *PeerManager) OnICEConnectionStateChange(callback func(peerID string, state webrtc.ICEConnectionState)) {
	pm.onICEConnectionStateChange = callback
}

// CreatePeer creates a new peer connection
func (pm *PeerManager) CreatePeer(ctx context.Context, peerID, streamID string, role PeerRole) (*PeerConnection, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if peer already exists
	if _, exists := pm.peers[peerID]; exists {
		return nil, &WebRTCError{Code: "PEER_EXISTS", Message: "peer already exists"}
	}

	// Create WebRTC configuration
	webrtcConfig := webrtc.Configuration{
		ICEServers: CreateICEServers(pm.config),
	}

	// Create peer connection
	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		pm.logger.Error("Failed to create peer connection",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	peer := &PeerConnection{
		ID:            peerID,
		StreamID:      streamID,
		Role:          role,
		State:         PeerStateNew,
		PC:            pc,
		Tracks:        []*Track{},
		ICECandidates: []webrtc.ICECandidateInit{},
		CreatedAt:     time.Now(),
		Stats:         &PeerStats{},
	}

	// Set up event handlers
	pm.setupPeerHandlers(peer)

	// Store peer
	pm.peers[peerID] = peer

	pm.logger.Info("Created peer connection",
		logger.Field{Key: "peer_id", Value: peerID},
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "role", Value: role},
	)

	return peer, nil
}

// setupPeerHandlers sets up event handlers for a peer connection
func (pm *PeerManager) setupPeerHandlers(peer *PeerConnection) {
	// Track handler
	peer.PC.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		pm.logger.Info("Received track",
			logger.Field{Key: "peer_id", Value: peer.ID},
			logger.Field{Key: "kind", Value: track.Kind().String()},
			logger.Field{Key: "codec", Value: track.Codec().MimeType},
		)

		// Create track info
		trackInfo := &Track{
			ID:          track.ID(),
			Kind:        track.Kind().String(),
			Codec:       track.Codec().MimeType,
			SSRC:        uint32(track.SSRC()),
			RemoteTrack: track,
			RTPReceiver: receiver,
		}

		// Add to peer tracks
		pm.mu.Lock()
		peer.Tracks = append(peer.Tracks, trackInfo)
		pm.mu.Unlock()

		if pm.onTrack != nil {
			go pm.onTrack(peer.ID, track, receiver)
		}
	})

	// Connection state handler
	peer.PC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		pm.logger.Info("Connection state changed",
			logger.Field{Key: "peer_id", Value: peer.ID},
			logger.Field{Key: "state", Value: state.String()},
		)

		pm.mu.Lock()
		switch state {
		case webrtc.PeerConnectionStateConnected:
			peer.State = PeerStateConnected
			now := time.Now()
			peer.ConnectedAt = &now
		case webrtc.PeerConnectionStateDisconnected:
			peer.State = PeerStateDisconnected
			now := time.Now()
			peer.DisconnectedAt = &now
		case webrtc.PeerConnectionStateFailed:
			peer.State = PeerStateFailed
		case webrtc.PeerConnectionStateClosed:
			peer.State = PeerStateClosed
		}
		pm.mu.Unlock()

		if pm.onConnectionStateChange != nil {
			go pm.onConnectionStateChange(peer.ID, state)
		}
	})

	// ICE connection state handler
	peer.PC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		pm.iceStateHandler.HandleStateChange(peer.ID, state)

		if pm.onICEConnectionStateChange != nil {
			go pm.onICEConnectionStateChange(peer.ID, state)
		}
	})

	// ICE gathering state handler
	peer.PC.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		pm.logger.Debug("ICE gathering state changed",
			logger.Field{Key: "peer_id", Value: peer.ID},
			logger.Field{Key: "state", Value: state.String()},
		)
	})
}

// GetPeer returns a peer connection by ID
func (pm *PeerManager) GetPeer(peerID string) (*PeerConnection, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peer, exists := pm.peers[peerID]
	if !exists {
		return nil, ErrPeerNotFound
	}

	return peer, nil
}

// RemovePeer removes and closes a peer connection
func (pm *PeerManager) RemovePeer(peerID string) error {
	pm.mu.Lock()
	peer, exists := pm.peers[peerID]
	if !exists {
		pm.mu.Unlock()
		return ErrPeerNotFound
	}

	delete(pm.peers, peerID)
	pm.mu.Unlock()

	// Close peer connection
	if err := peer.PC.Close(); err != nil {
		pm.logger.Error("Failed to close peer connection",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return err
	}

	// Clear ICE candidates
	pm.iceGatherer.ClearPeerCandidates(peerID)

	pm.logger.Info("Removed peer connection",
		logger.Field{Key: "peer_id", Value: peerID},
	)

	return nil
}

// CreateOffer creates an SDP offer for a peer
func (pm *PeerManager) CreateOffer(peerID string) (*webrtc.SessionDescription, error) {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return nil, err
	}

	offer, err := peer.PC.CreateOffer(nil)
	if err != nil {
		pm.logger.Error("Failed to create offer",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	if err := peer.PC.SetLocalDescription(offer); err != nil {
		pm.logger.Error("Failed to set local description",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	pm.logger.Info("Created offer",
		logger.Field{Key: "peer_id", Value: peerID},
	)

	return &offer, nil
}

// CreateAnswer creates an SDP answer for a peer
func (pm *PeerManager) CreateAnswer(peerID string) (*webrtc.SessionDescription, error) {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return nil, err
	}

	answer, err := peer.PC.CreateAnswer(nil)
	if err != nil {
		pm.logger.Error("Failed to create answer",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	if err := peer.PC.SetLocalDescription(answer); err != nil {
		pm.logger.Error("Failed to set local description",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	pm.logger.Info("Created answer",
		logger.Field{Key: "peer_id", Value: peerID},
	)

	return &answer, nil
}

// SetRemoteDescription sets the remote SDP for a peer
func (pm *PeerManager) SetRemoteDescription(peerID string, sdp webrtc.SessionDescription) error {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return err
	}

	if err := peer.PC.SetRemoteDescription(sdp); err != nil {
		pm.logger.Error("Failed to set remote description",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return err
	}

	pm.logger.Info("Set remote description",
		logger.Field{Key: "peer_id", Value: peerID},
		logger.Field{Key: "type", Value: sdp.Type.String()},
	)

	return nil
}

// AddICECandidate adds an ICE candidate to a peer connection
func (pm *PeerManager) AddICECandidate(peerID string, candidate webrtc.ICECandidateInit) error {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return err
	}

	if err := pm.iceGatherer.AddCandidate(peer.PC, candidate); err != nil {
		return err
	}

	pm.mu.Lock()
	peer.ICECandidates = append(peer.ICECandidates, candidate)
	pm.mu.Unlock()

	return nil
}

// AddTrack adds a track to a peer connection
func (pm *PeerManager) AddTrack(peerID string, track *webrtc.TrackLocalStaticRTP) (*webrtc.RTPSender, error) {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return nil, err
	}

	sender, err := peer.PC.AddTrack(track)
	if err != nil {
		pm.logger.Error("Failed to add track",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, err
	}

	// Store track info
	trackInfo := &Track{
		ID:         track.ID(),
		Kind:       track.Kind().String(),
		Codec:      track.Codec().MimeType,
		LocalTrack: track,
		RTPSender:  sender,
	}

	pm.mu.Lock()
	peer.Tracks = append(peer.Tracks, trackInfo)
	pm.mu.Unlock()

	pm.logger.Info("Added track to peer",
		logger.Field{Key: "peer_id", Value: peerID},
		logger.Field{Key: "kind", Value: track.Kind().String()},
	)

	return sender, nil
}

// GetStats retrieves statistics for a peer connection
func (pm *PeerManager) GetStats(peerID string) (*PeerStats, error) {
	peer, err := pm.GetPeer(peerID)
	if err != nil {
		return nil, err
	}

	stats := peer.PC.GetStats()

	// Update peer stats
	pm.mu.Lock()
	peer.Stats.LastUpdated = time.Now()
	pm.mu.Unlock()

	// Parse stats
	for _, stat := range stats {
		switch s := stat.(type) {
		case *webrtc.InboundRTPStreamStats:
			pm.mu.Lock()
			peer.Stats.PacketsReceived = uint64(s.PacketsReceived)
			peer.Stats.BytesReceived = uint64(s.BytesReceived)
			peer.Stats.PacketsLost = uint64(s.PacketsLost)
			peer.Stats.Jitter = s.Jitter
			pm.mu.Unlock()

		case *webrtc.OutboundRTPStreamStats:
			pm.mu.Lock()
			peer.Stats.PacketsSent = uint64(s.PacketsSent)
			peer.Stats.BytesSent = uint64(s.BytesSent)
			pm.mu.Unlock()
		}
	}

	return peer.Stats, nil
}

// GetPeerCount returns the number of active peers
func (pm *PeerManager) GetPeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.peers)
}

// GetPeersByStream returns all peers for a stream
func (pm *PeerManager) GetPeersByStream(streamID string) []*PeerConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var peers []*PeerConnection
	for _, peer := range pm.peers {
		if peer.StreamID == streamID {
			peers = append(peers, peer)
		}
	}

	return peers
}

// GetPeersByRole returns all peers with a specific role
func (pm *PeerManager) GetPeersByRole(role PeerRole) []*PeerConnection {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var peers []*PeerConnection
	for _, peer := range pm.peers {
		if peer.Role == role {
			peers = append(peers, peer)
		}
	}

	return peers
}

// CloseAll closes all peer connections
func (pm *PeerManager) CloseAll() error {
	pm.mu.Lock()
	peerIDs := make([]string, 0, len(pm.peers))
	for id := range pm.peers {
		peerIDs = append(peerIDs, id)
	}
	pm.mu.Unlock()

	var lastErr error
	for _, peerID := range peerIDs {
		if err := pm.RemovePeer(peerID); err != nil {
			lastErr = fmt.Errorf("failed to remove peer %s: %w", peerID, err)
		}
	}

	pm.logger.Info("Closed all peer connections",
		logger.Field{Key: "count", Value: len(peerIDs)},
	)

	return lastErr
}
