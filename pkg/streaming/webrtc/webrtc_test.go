// Package webrtc provides tests for WebRTC streaming components.
package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// TestDefaultConfig tests the default WebRTC configuration
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if len(config.STUNServers) == 0 {
		t.Error("Expected at least one STUN server")
	}

	if config.MaxBitrate <= 0 {
		t.Error("Expected positive max bitrate")
	}

	if config.MinBitrate <= 0 {
		t.Error("Expected positive min bitrate")
	}

	if config.MinBitrate > config.MaxBitrate {
		t.Error("Min bitrate should not exceed max bitrate")
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "no ICE servers",
			config: Config{
				STUNServers:   []string{},
				TURNServers:   []TURNServer{},
				MaxBitrate:    5000000,
				MinBitrate:    500000,
				TargetBitrate: 2000000,
			},
			wantErr: true,
		},
		{
			name: "invalid bitrate range",
			config: Config{
				STUNServers:   []string{DefaultSTUNServer},
				MaxBitrate:    500000,
				MinBitrate:    5000000,
				TargetBitrate: 2000000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestBandwidthEstimator tests bandwidth estimation
func TestBandwidthEstimator(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	config := DefaultBWEConfig()
	bwe := NewBandwidthEstimator(config, log)

	// Initial bitrate should be start bitrate
	if bwe.GetCurrentBitrate() != config.StartBitrate {
		t.Errorf("Expected initial bitrate %d, got %d", config.StartBitrate, bwe.GetCurrentBitrate())
	}

	// Simulate good network conditions
	time.Sleep(100 * time.Millisecond)
	bitrate := bwe.Update(1000000, 0, 50*time.Millisecond)
	if bitrate == 0 {
		t.Error("Expected non-zero bitrate")
	}

	// Test manual bitrate setting
	testBitrate := 3000000
	bwe.SetBitrate(testBitrate)
	if bwe.GetCurrentBitrate() != testBitrate {
		t.Errorf("Expected bitrate %d after manual set, got %d", testBitrate, bwe.GetCurrentBitrate())
	}

	// Test reset
	bwe.Reset()
	if bwe.GetCurrentBitrate() != config.StartBitrate {
		t.Errorf("Expected bitrate %d after reset, got %d", config.StartBitrate, bwe.GetCurrentBitrate())
	}
}

// TestCongestionController tests congestion detection
func TestCongestionController(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	config := DefaultBWEConfig()
	cc := NewCongestionController(config, log)

	// Should not be congested initially
	if cc.IsCongested() {
		t.Error("Expected no congestion initially")
	}

	// Recommended bitrate should be positive
	bitrate := cc.GetRecommendedBitrate()
	if bitrate <= 0 {
		t.Error("Expected positive recommended bitrate")
	}
}

// TestICEGatherer tests ICE candidate gathering
func TestICEGatherer(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	config := DefaultConfig()
	gatherer := NewICEGatherer(config, log)

	peerID := "test-peer"

	// Initially should have no candidates
	candidates := gatherer.GetCandidates(peerID)
	if len(candidates) != 0 {
		t.Error("Expected no candidates initially")
	}

	// Should not be complete initially
	if gatherer.IsGatheringComplete(peerID) {
		t.Error("Expected gathering not complete initially")
	}

	// Clear should not error
	gatherer.ClearPeerCandidates(peerID)
}

// TestTrackManager tests track management
func TestTrackManager(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	tm := NewTrackManager(log)

	// Create video track
	videoTrack, err := CreateVideoTrack("video-1", "stream-1")
	if err != nil {
		t.Fatalf("Failed to create video track: %v", err)
	}

	if videoTrack == nil {
		t.Error("Expected non-nil video track")
	}

	// Create audio track
	audioTrack, err := CreateAudioTrack("audio-1", "stream-1")
	if err != nil {
		t.Fatalf("Failed to create audio track: %v", err)
	}

	if audioTrack == nil {
		t.Error("Expected non-nil audio track")
	}

	// Close all should not error
	tm.CloseAll()
}

// TestPeerManager tests peer connection management
func TestPeerManager(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	config := DefaultConfig()
	pm := NewPeerManager(config, log)

	ctx := context.Background()

	// Create peer
	peer, err := pm.CreatePeer(ctx, "peer-1", "stream-1", PeerRolePublisher)
	if err != nil {
		t.Fatalf("Failed to create peer: %v", err)
	}

	if peer == nil {
		t.Error("Expected non-nil peer")
	}

	if peer.ID != "peer-1" {
		t.Errorf("Expected peer ID 'peer-1', got '%s'", peer.ID)
	}

	if peer.Role != PeerRolePublisher {
		t.Errorf("Expected role %s, got %s", PeerRolePublisher, peer.Role)
	}

	// Get peer
	retrieved, err := pm.GetPeer("peer-1")
	if err != nil {
		t.Errorf("Failed to get peer: %v", err)
	}

	if retrieved.ID != peer.ID {
		t.Error("Retrieved peer ID mismatch")
	}

	// Get peer count
	count := pm.GetPeerCount()
	if count != 1 {
		t.Errorf("Expected peer count 1, got %d", count)
	}

	// Remove peer
	err = pm.RemovePeer("peer-1")
	if err != nil {
		t.Errorf("Failed to remove peer: %v", err)
	}

	// Should not exist after removal
	_, err = pm.GetPeer("peer-1")
	if err == nil {
		t.Error("Expected error when getting removed peer")
	}

	// Close all
	pm.CloseAll()
}

// TestSFU tests SFU functionality
func TestSFU(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "json")
	config := DefaultSFUConfig()
	sfu := NewSFU(config, log)
	defer sfu.Close()

	// Create stream
	err := sfu.CreateStream("stream-1", "Test Stream")
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Get stream
	stream, err := sfu.GetStream("stream-1")
	if err != nil {
		t.Errorf("Failed to get stream: %v", err)
	}

	if stream.ID != "stream-1" {
		t.Errorf("Expected stream ID 'stream-1', got '%s'", stream.ID)
	}

	// Stream count should be 1
	count := sfu.GetStreamCount()
	if count != 1 {
		t.Errorf("Expected stream count 1, got %d", count)
	}

	// Get streams
	streams := sfu.GetStreams()
	if len(streams) != 1 {
		t.Errorf("Expected 1 stream, got %d", len(streams))
	}

	// Delete stream
	err = sfu.DeleteStream("stream-1")
	if err != nil {
		t.Errorf("Failed to delete stream: %v", err)
	}

	// Should not exist after deletion
	_, err = sfu.GetStream("stream-1")
	if err == nil {
		t.Error("Expected error when getting deleted stream")
	}
}

// TestSignalingServerConfig tests signaling server configuration
func TestSignalingServerConfig(t *testing.T) {
	config := DefaultSignalingServerConfig()

	if err := config.Validate(); err != nil {
		t.Errorf("Default signaling config should be valid: %v", err)
	}

	// Test invalid config
	invalidConfig := SignalingServerConfig{
		ListenAddr: "",
		Path:       "",
	}

	if err := invalidConfig.Validate(); err == nil {
		t.Error("Expected error for invalid signaling config")
	}
}

// TestWebRTCErrors tests error types
func TestWebRTCErrors(t *testing.T) {
	err := ErrPeerNotFound
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}

	err2 := ErrStreamNotFound
	if err2.Error() == "" {
		t.Error("Expected non-empty error message")
	}

	customErr := &WebRTCError{Code: "TEST_ERROR", Message: "test message"}
	if customErr.Error() != "TEST_ERROR: test message" {
		t.Errorf("Unexpected error format: %s", customErr.Error())
	}
}
