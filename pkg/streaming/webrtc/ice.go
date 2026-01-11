// Package webrtc provides ICE (Interactive Connectivity Establishment) handling
// for WebRTC peer connections.
package webrtc

import (
	"context"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/pion/webrtc/v3"
)

// ICEGatherer handles ICE candidate gathering for peer connections
type ICEGatherer struct {
	// config is the WebRTC configuration
	config Config

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// candidates stores gathered ICE candidates by peer ID
	candidates map[string][]webrtc.ICECandidateInit

	// gatheringComplete tracks completion status by peer ID
	gatheringComplete map[string]bool

	// onCandidateCallbacks stores callbacks for new candidates
	onCandidateCallbacks map[string][]func(*webrtc.ICECandidate)
}

// NewICEGatherer creates a new ICE gatherer
func NewICEGatherer(config Config, log logger.Logger) *ICEGatherer {
	return &ICEGatherer{
		config:               config,
		logger:               log,
		candidates:           make(map[string][]webrtc.ICECandidateInit),
		gatheringComplete:    make(map[string]bool),
		onCandidateCallbacks: make(map[string][]func(*webrtc.ICECandidate)),
	}
}

// GatherCandidates starts ICE candidate gathering for a peer connection
func (g *ICEGatherer) GatherCandidates(ctx context.Context, peerID string, pc *webrtc.PeerConnection) error {
	g.mu.Lock()
	g.candidates[peerID] = []webrtc.ICECandidateInit{}
	g.gatheringComplete[peerID] = false
	g.mu.Unlock()

	// Set up ICE candidate handler
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			// Gathering complete
			g.mu.Lock()
			g.gatheringComplete[peerID] = true
			g.mu.Unlock()

			g.logger.Info("ICE gathering complete",
				logger.Field{Key: "peer_id", Value: peerID},
			)
			return
		}

		// Store candidate
		candidateInit := candidate.ToJSON()
		g.mu.Lock()
		g.candidates[peerID] = append(g.candidates[peerID], candidateInit)

		// Call registered callbacks
		callbacks := g.onCandidateCallbacks[peerID]
		g.mu.Unlock()

		g.logger.Debug("ICE candidate gathered",
			logger.Field{Key: "peer_id", Value: peerID},
			logger.Field{Key: "candidate", Value: candidateInit.Candidate},
		)

		// Execute callbacks
		for _, callback := range callbacks {
			go callback(candidate)
		}
	})

	// Wait for gathering with timeout
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(g.config.ICEGatheringTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			g.logger.Warn("ICE gathering timeout",
				logger.Field{Key: "peer_id", Value: peerID},
			)
			return nil // Return success even on timeout
		case <-ticker.C:
			g.mu.RLock()
			complete := g.gatheringComplete[peerID]
			g.mu.RUnlock()

			if complete {
				return nil
			}
		}
	}
}

// GetCandidates returns all gathered ICE candidates for a peer
func (g *ICEGatherer) GetCandidates(peerID string) []webrtc.ICECandidateInit {
	g.mu.RLock()
	defer g.mu.RUnlock()

	candidates := g.candidates[peerID]
	result := make([]webrtc.ICECandidateInit, len(candidates))
	copy(result, candidates)

	return result
}

// AddCandidate adds a remote ICE candidate to a peer connection
func (g *ICEGatherer) AddCandidate(pc *webrtc.PeerConnection, candidate webrtc.ICECandidateInit) error {
	if err := pc.AddICECandidate(candidate); err != nil {
		g.logger.Error("Failed to add ICE candidate",
			logger.Field{Key: "error", Value: err.Error()},
			logger.Field{Key: "candidate", Value: candidate.Candidate},
		)
		return err
	}

	g.logger.Debug("ICE candidate added",
		logger.Field{Key: "candidate", Value: candidate.Candidate},
	)

	return nil
}

// OnCandidate registers a callback for new ICE candidates
func (g *ICEGatherer) OnCandidate(peerID string, callback func(*webrtc.ICECandidate)) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.onCandidateCallbacks[peerID] = append(g.onCandidateCallbacks[peerID], callback)
}

// ClearPeerCandidates clears all candidates and callbacks for a peer
func (g *ICEGatherer) ClearPeerCandidates(peerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.candidates, peerID)
	delete(g.gatheringComplete, peerID)
	delete(g.onCandidateCallbacks, peerID)

	g.logger.Debug("Cleared ICE candidates",
		logger.Field{Key: "peer_id", Value: peerID},
	)
}

// IsGatheringComplete checks if ICE gathering is complete for a peer
func (g *ICEGatherer) IsGatheringComplete(peerID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.gatheringComplete[peerID]
}

// ICEConnectionStateHandler handles ICE connection state changes
type ICEConnectionStateHandler struct {
	// logger for logging
	logger logger.Logger

	// onConnected is called when connection is established
	onConnected func(peerID string)

	// onDisconnected is called when connection is lost
	onDisconnected func(peerID string)

	// onFailed is called when connection fails
	onFailed func(peerID string)

	// onClosed is called when connection is closed
	onClosed func(peerID string)
}

// NewICEConnectionStateHandler creates a new ICE connection state handler
func NewICEConnectionStateHandler(log logger.Logger) *ICEConnectionStateHandler {
	return &ICEConnectionStateHandler{
		logger: log,
	}
}

// OnConnected sets the callback for connected state
func (h *ICEConnectionStateHandler) OnConnected(callback func(peerID string)) {
	h.onConnected = callback
}

// OnDisconnected sets the callback for disconnected state
func (h *ICEConnectionStateHandler) OnDisconnected(callback func(peerID string)) {
	h.onDisconnected = callback
}

// OnFailed sets the callback for failed state
func (h *ICEConnectionStateHandler) OnFailed(callback func(peerID string)) {
	h.onFailed = callback
}

// OnClosed sets the callback for closed state
func (h *ICEConnectionStateHandler) OnClosed(callback func(peerID string)) {
	h.onClosed = callback
}

// HandleStateChange handles ICE connection state changes
func (h *ICEConnectionStateHandler) HandleStateChange(peerID string, state webrtc.ICEConnectionState) {
	h.logger.Info("ICE connection state changed",
		logger.Field{Key: "peer_id", Value: peerID},
		logger.Field{Key: "state", Value: state.String()},
	)

	switch state {
	case webrtc.ICEConnectionStateConnected:
		if h.onConnected != nil {
			go h.onConnected(peerID)
		}

	case webrtc.ICEConnectionStateDisconnected:
		if h.onDisconnected != nil {
			go h.onDisconnected(peerID)
		}

	case webrtc.ICEConnectionStateFailed:
		if h.onFailed != nil {
			go h.onFailed(peerID)
		}

	case webrtc.ICEConnectionStateClosed:
		if h.onClosed != nil {
			go h.onClosed(peerID)
		}
	}
}

// CreateICEServers creates ICE server configuration from Config
func CreateICEServers(config Config) []webrtc.ICEServer {
	var iceServers []webrtc.ICEServer

	// Add STUN servers
	if len(config.STUNServers) > 0 {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: config.STUNServers,
		})
	}

	// Add TURN servers
	for _, turn := range config.TURNServers {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       turn.URLs,
			Username:   turn.Username,
			Credential: turn.Credential,
		})
	}

	return iceServers
}

// TrickleICE manages trickle ICE for gradual candidate exchange
type TrickleICE struct {
	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// pendingCandidates stores candidates waiting to be sent
	pendingCandidates map[string][]webrtc.ICECandidateInit

	// sendCandidate is the callback to send a candidate
	sendCandidate func(peerID string, candidate webrtc.ICECandidateInit) error
}

// NewTrickleICE creates a new trickle ICE manager
func NewTrickleICE(log logger.Logger, sendFunc func(string, webrtc.ICECandidateInit) error) *TrickleICE {
	return &TrickleICE{
		logger:            log,
		pendingCandidates: make(map[string][]webrtc.ICECandidateInit),
		sendCandidate:     sendFunc,
	}
}

// AddPendingCandidate adds a candidate to the pending queue
func (t *TrickleICE) AddPendingCandidate(peerID string, candidate webrtc.ICECandidateInit) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pendingCandidates[peerID] = append(t.pendingCandidates[peerID], candidate)
}

// SendPendingCandidates sends all pending candidates for a peer
func (t *TrickleICE) SendPendingCandidates(peerID string) error {
	t.mu.Lock()
	candidates := t.pendingCandidates[peerID]
	delete(t.pendingCandidates, peerID)
	t.mu.Unlock()

	for _, candidate := range candidates {
		if err := t.sendCandidate(peerID, candidate); err != nil {
			t.logger.Error("Failed to send trickle ICE candidate",
				logger.Field{Key: "peer_id", Value: peerID},
				logger.Field{Key: "error", Value: err.Error()},
			)
			return err
		}
	}

	t.logger.Debug("Sent pending ICE candidates",
		logger.Field{Key: "peer_id", Value: peerID},
		logger.Field{Key: "count", Value: len(candidates)},
	)

	return nil
}

// ClearPendingCandidates clears pending candidates for a peer
func (t *TrickleICE) ClearPendingCandidates(peerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.pendingCandidates, peerID)
}

// GetPendingCandidatesCount returns the number of pending candidates
func (t *TrickleICE) GetPendingCandidatesCount(peerID string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.pendingCandidates[peerID])
}
