package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/google/uuid"
)

// StreamProtocol represents the streaming protocol
type StreamProtocol string

const (
	// ProtocolRTMP represents RTMP protocol
	ProtocolRTMP StreamProtocol = "rtmp"

	// ProtocolHLS represents HLS protocol
	ProtocolHLS StreamProtocol = "hls"

	// ProtocolWebRTC represents WebRTC protocol
	ProtocolWebRTC StreamProtocol = "webrtc"
)

// Stream represents a livestream session
type Stream struct {
	// Unique stream identifier
	ID string `json:"id"`

	// Stream key for publishing
	StreamKey string `json:"stream_key"`

	// User ID who owns this stream
	UserID string `json:"user_id"`

	// Stream title
	Title string `json:"title"`

	// Stream description
	Description string `json:"description"`

	// Streaming protocol
	Protocol StreamProtocol `json:"protocol"`

	// Current state of the stream
	State StreamState `json:"state"`

	// Stream configuration
	Config *StreamConfig `json:"config,omitempty"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`

	// Metrics
	ViewerCount   int64         `json:"viewer_count"`
	TotalDuration time.Duration `json:"total_duration"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Internal state machine
	stateMachine *StreamStateMachine `json:"-"`

	// Thread safety
	mu sync.RWMutex `json:"-"`
}

// StreamConfig holds stream configuration
type StreamConfig struct {
	// Enable recording
	EnableRecording bool `json:"enable_recording"`

	// Enable chat
	EnableChat bool `json:"enable_chat"`

	// Enable DVR
	EnableDVR bool `json:"enable_dvr"`

	// Maximum duration (0 = unlimited)
	MaxDuration time.Duration `json:"max_duration"`

	// Maximum viewers (0 = unlimited)
	MaxViewers int `json:"max_viewers"`

	// Video quality settings
	VideoQuality *VideoQuality `json:"video_quality,omitempty"`

	// Custom settings
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// VideoQuality represents video quality settings
type VideoQuality struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bitrate   int    `json:"bitrate"`   // kbps
	Framerate int    `json:"framerate"` // fps
	Codec     string `json:"codec"`
	Profile   string `json:"profile"`
}

// DefaultStreamConfig returns default stream configuration
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		EnableRecording: true,
		EnableChat:      true,
		EnableDVR:       false,
		MaxDuration:     0,
		MaxViewers:      0,
		VideoQuality: &VideoQuality{
			Width:     1920,
			Height:    1080,
			Bitrate:   4000,
			Framerate: 30,
			Codec:     "h264",
			Profile:   "high",
		},
		Custom: make(map[string]interface{}),
	}
}

// CreateStreamRequest represents a request to create a stream
type CreateStreamRequest struct {
	UserID      string            `json:"user_id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Protocol    StreamProtocol    `json:"protocol"`
	Config      *StreamConfig     `json:"config,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateStreamRequest represents a request to update a stream
type UpdateStreamRequest struct {
	Title       *string           `json:"title,omitempty"`
	Description *string           `json:"description,omitempty"`
	Config      *StreamConfig     `json:"config,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StreamManager manages stream lifecycle
type StreamManager struct {
	streams map[string]*Stream
	mu      sync.RWMutex
	logger  logger.Logger
}

// NewStreamManager creates a new stream manager
func NewStreamManager(log logger.Logger) *StreamManager {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &StreamManager{
		streams: make(map[string]*Stream),
		logger:  log,
	}
}

// CreateStream creates a new stream
func (sm *StreamManager) CreateStream(ctx context.Context, req *CreateStreamRequest) (*Stream, error) {
	if req == nil {
		return nil, fmt.Errorf("create stream request is required")
	}

	if req.UserID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	if req.Title == "" {
		return nil, fmt.Errorf("stream title is required")
	}

	if req.Protocol == "" {
		req.Protocol = ProtocolRTMP // Default to RTMP
	}

	// Validate protocol
	if req.Protocol != ProtocolRTMP && req.Protocol != ProtocolHLS && req.Protocol != ProtocolWebRTC {
		return nil, fmt.Errorf("invalid protocol: %s", req.Protocol)
	}

	// Use default config if not provided
	if req.Config == nil {
		req.Config = DefaultStreamConfig()
	}

	// Generate unique IDs
	streamID := uuid.New().String()
	streamKey := generateStreamKey()

	now := time.Now()

	stream := &Stream{
		ID:           streamID,
		StreamKey:    streamKey,
		UserID:       req.UserID,
		Title:        req.Title,
		Description:  req.Description,
		Protocol:     req.Protocol,
		State:        StateIdle,
		Config:       req.Config,
		CreatedAt:    now,
		UpdatedAt:    now,
		ViewerCount:  0,
		Metadata:     req.Metadata,
		stateMachine: NewStreamStateMachine(),
	}

	if stream.Metadata == nil {
		stream.Metadata = make(map[string]string)
	}

	// Store stream
	sm.mu.Lock()
	sm.streams[streamID] = stream
	sm.mu.Unlock()

	sm.logger.Info("Stream created",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "user_id", Value: req.UserID},
		logger.Field{Key: "protocol", Value: req.Protocol},
	)

	return stream, nil
}

// GetStream retrieves a stream by ID
func (sm *StreamManager) GetStream(ctx context.Context, streamID string) (*Stream, error) {
	if streamID == "" {
		return nil, fmt.Errorf("stream ID is required")
	}

	sm.mu.RLock()
	stream, exists := sm.streams[streamID]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stream not found: %s", streamID)
	}

	return stream, nil
}

// UpdateStream updates an existing stream
func (sm *StreamManager) UpdateStream(ctx context.Context, streamID string, req *UpdateStreamRequest) (*Stream, error) {
	if streamID == "" {
		return nil, fmt.Errorf("stream ID is required")
	}

	if req == nil {
		return nil, fmt.Errorf("update request is required")
	}

	stream, err := sm.GetStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()

	// Update fields if provided
	if req.Title != nil {
		stream.Title = *req.Title
	}

	if req.Description != nil {
		stream.Description = *req.Description
	}

	if req.Config != nil {
		stream.Config = req.Config
	}

	if req.Metadata != nil {
		if stream.Metadata == nil {
			stream.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			stream.Metadata[k] = v
		}
	}

	stream.UpdatedAt = time.Now()

	sm.logger.Info("Stream updated",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	return stream, nil
}

// DeleteStream deletes a stream
func (sm *StreamManager) DeleteStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sm.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Check if stream can be deleted
	if stream.stateMachine.IsLive() {
		return fmt.Errorf("cannot delete stream while live, stop stream first")
	}

	sm.mu.Lock()
	delete(sm.streams, streamID)
	sm.mu.Unlock()

	sm.logger.Info("Stream deleted",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	return nil
}

// ListStreams returns all streams (use QueryStreams for filtering)
func (sm *StreamManager) ListStreams(ctx context.Context) ([]*Stream, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	streams := make([]*Stream, 0, len(sm.streams))
	for _, stream := range sm.streams {
		streams = append(streams, stream)
	}

	return streams, nil
}

// GetStreamCount returns the total number of streams
func (sm *StreamManager) GetStreamCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.streams)
}

// GetStreamsByUser returns all streams for a specific user
func (sm *StreamManager) GetStreamsByUser(ctx context.Context, userID string) ([]*Stream, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	streams := make([]*Stream, 0)
	for _, stream := range sm.streams {
		if stream.UserID == userID {
			streams = append(streams, stream)
		}
	}

	return streams, nil
}

// GetStreamsByState returns all streams in a specific state
func (sm *StreamManager) GetStreamsByState(ctx context.Context, state StreamState) ([]*Stream, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	streams := make([]*Stream, 0)
	for _, stream := range sm.streams {
		if stream.State == state {
			streams = append(streams, stream)
		}
	}

	return streams, nil
}

// GetLiveStreamCount returns the number of live streams
func (sm *StreamManager) GetLiveStreamCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, stream := range sm.streams {
		if stream.stateMachine.IsLive() {
			count++
		}
	}

	return count
}

// Helper functions

// generateStreamKey generates a random stream key
func generateStreamKey() string {
	return uuid.New().String()
}

// GetDuration returns the stream duration
func (s *Stream) GetDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.StartedAt == nil {
		return 0
	}

	if s.EndedAt != nil {
		return s.EndedAt.Sub(*s.StartedAt)
	}

	if s.stateMachine.IsLive() || s.stateMachine.IsPaused() {
		return time.Since(*s.StartedAt)
	}

	return s.TotalDuration
}

// IncrementViewerCount increments the viewer count
func (s *Stream) IncrementViewerCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ViewerCount++
}

// DecrementViewerCount decrements the viewer count
func (s *Stream) DecrementViewerCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ViewerCount > 0 {
		s.ViewerCount--
	}
}

// GetViewerCount returns the current viewer count (thread-safe)
func (s *Stream) GetViewerCount() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ViewerCount
}
