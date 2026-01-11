package types

import (
	"time"
)

// StreamStatus represents the current status of a stream
type StreamStatus string

const (
	// StreamStatusIdle represents an idle stream (not started)
	StreamStatusIdle StreamStatus = "idle"

	// StreamStatusLive represents a live stream (currently broadcasting)
	StreamStatusLive StreamStatus = "live"

	// StreamStatusEnded represents an ended stream
	StreamStatusEnded StreamStatus = "ended"

	// StreamStatusPaused represents a paused stream
	StreamStatusPaused StreamStatus = "paused"

	// StreamStatusError represents a stream in error state
	StreamStatusError StreamStatus = "error"
)

// StreamProtocol represents the streaming protocol
type StreamProtocol string

const (
	// ProtocolRTMP represents the RTMP protocol
	ProtocolRTMP StreamProtocol = "rtmp"

	// ProtocolHLS represents the HLS protocol
	ProtocolHLS StreamProtocol = "hls"

	// ProtocolWebRTC represents the WebRTC protocol
	ProtocolWebRTC StreamProtocol = "webrtc"
)

// StreamQuality represents stream quality settings
type StreamQuality string

const (
	// QualityAuto represents automatic quality selection
	QualityAuto StreamQuality = "auto"

	// QualityLow represents low quality (360p)
	QualityLow StreamQuality = "low"

	// QualityMedium represents medium quality (480p)
	QualityMedium StreamQuality = "medium"

	// QualityHigh represents high quality (720p)
	QualityHigh StreamQuality = "high"

	// QualityUltra represents ultra quality (1080p)
	QualityUltra StreamQuality = "ultra"
)

// Stream represents a livestream session
type Stream struct {
	// ID is the unique identifier for the stream
	ID string `json:"id"`

	// UserID is the ID of the user who created the stream
	UserID string `json:"user_id"`

	// Title is the stream title
	Title string `json:"title"`

	// Description is the stream description
	Description string `json:"description"`

	// StreamKey is the unique key for publishing to this stream
	StreamKey string `json:"stream_key"`

	// Status is the current status of the stream
	Status StreamStatus `json:"status"`

	// Protocol is the streaming protocol being used
	Protocol StreamProtocol `json:"protocol"`

	// Quality is the stream quality setting
	Quality StreamQuality `json:"quality"`

	// StartTime is when the stream started
	StartTime *time.Time `json:"start_time,omitempty"`

	// EndTime is when the stream ended
	EndTime *time.Time `json:"end_time,omitempty"`

	// CreatedAt is when the stream was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the stream was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata contains additional stream metadata
	Metadata StreamMetadata `json:"metadata"`

	// Config contains stream-specific configuration
	Config StreamConfig `json:"config"`
}

// StreamMetadata contains metadata about a stream
type StreamMetadata struct {
	// ViewerCount is the current number of viewers
	ViewerCount int64 `json:"viewer_count"`

	// PeakViewerCount is the peak number of concurrent viewers
	PeakViewerCount int64 `json:"peak_viewer_count"`

	// Duration is the total duration of the stream
	Duration time.Duration `json:"duration"`

	// Bitrate is the current bitrate in bits per second
	Bitrate int64 `json:"bitrate"`

	// FPS is the current frames per second
	FPS int `json:"fps"`

	// Resolution is the stream resolution (e.g., "1920x1080")
	Resolution string `json:"resolution"`

	// AudioCodec is the audio codec being used
	AudioCodec string `json:"audio_codec"`

	// VideoCodec is the video codec being used
	VideoCodec string `json:"video_codec"`

	// DroppedFrames is the number of dropped frames
	DroppedFrames int64 `json:"dropped_frames"`

	// Tags are custom tags for the stream
	Tags []string `json:"tags,omitempty"`
}

// StreamConfig contains configuration for a stream
type StreamConfig struct {
	// EnableRecording enables recording for this stream
	EnableRecording bool `json:"enable_recording"`

	// EnableChat enables chat for this stream
	EnableChat bool `json:"enable_chat"`

	// EnableDVR enables DVR (rewind) functionality
	EnableDVR bool `json:"enable_dvr"`

	// DVRWindowSize is the size of the DVR window in seconds
	DVRWindowSize int `json:"dvr_window_size"`

	// MaxViewers is the maximum number of concurrent viewers (0 = unlimited)
	MaxViewers int `json:"max_viewers"`

	// IsPrivate indicates if the stream is private
	IsPrivate bool `json:"is_private"`

	// AllowedViewers is a list of user IDs allowed to view (if private)
	AllowedViewers []string `json:"allowed_viewers,omitempty"`

	// RecordingPath is the path where recordings will be stored
	RecordingPath string `json:"recording_path,omitempty"`

	// ThumbnailInterval is the interval for generating thumbnails in seconds
	ThumbnailInterval int `json:"thumbnail_interval"`
}

// StreamProvider is the interface that all streaming protocol providers must implement
type StreamProvider interface {
	// Start starts the stream provider
	Start() error

	// Stop stops the stream provider
	Stop() error

	// IsRunning returns true if the provider is running
	IsRunning() bool

	// Protocol returns the protocol name
	Protocol() StreamProtocol

	// Publish publishes a stream
	Publish(stream *Stream) error

	// Unpublish stops publishing a stream
	Unpublish(streamID string) error

	// Subscribe subscribes to a stream
	Subscribe(streamID string, opts SubscribeOptions) (StreamSession, error)

	// Unsubscribe unsubscribes from a stream
	Unsubscribe(sessionID string) error
}

// StreamSession represents an active stream session (viewer or publisher)
type StreamSession interface {
	// ID returns the session ID
	ID() string

	// StreamID returns the stream ID
	StreamID() string

	// UserID returns the user ID
	UserID() string

	// IsPublisher returns true if this is a publisher session
	IsPublisher() bool

	// StartTime returns when the session started
	StartTime() time.Time

	// Close closes the session
	Close() error

	// GetMetadata returns session metadata
	GetMetadata() SessionMetadata
}

// SessionMetadata contains metadata about a session
type SessionMetadata struct {
	// BytesSent is the total bytes sent
	BytesSent int64 `json:"bytes_sent"`

	// BytesReceived is the total bytes received
	BytesReceived int64 `json:"bytes_received"`

	// PacketsLost is the number of packets lost
	PacketsLost int64 `json:"packets_lost"`

	// Latency is the current latency in milliseconds
	Latency int64 `json:"latency"`

	// Bitrate is the current bitrate
	Bitrate int64 `json:"bitrate"`
}

// SubscribeOptions contains options for subscribing to a stream
type SubscribeOptions struct {
	// Quality is the preferred quality
	Quality StreamQuality `json:"quality"`

	// UserID is the ID of the subscribing user
	UserID string `json:"user_id"`

	// EnableAudio enables audio playback
	EnableAudio bool `json:"enable_audio"`

	// EnableVideo enables video playback
	EnableVideo bool `json:"enable_video"`
}

// StreamEvent represents an event that occurred on a stream
type StreamEvent struct {
	// Type is the event type
	Type StreamEventType `json:"type"`

	// StreamID is the ID of the stream
	StreamID string `json:"stream_id"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Data contains event-specific data
	Data map[string]interface{} `json:"data,omitempty"`
}

// StreamEventType represents the type of stream event
type StreamEventType string

const (
	// EventStreamStarted indicates a stream has started
	EventStreamStarted StreamEventType = "stream.started"

	// EventStreamEnded indicates a stream has ended
	EventStreamEnded StreamEventType = "stream.ended"

	// EventStreamPaused indicates a stream has been paused
	EventStreamPaused StreamEventType = "stream.paused"

	// EventStreamResumed indicates a stream has resumed
	EventStreamResumed StreamEventType = "stream.resumed"

	// EventViewerJoined indicates a viewer joined
	EventViewerJoined StreamEventType = "viewer.joined"

	// EventViewerLeft indicates a viewer left
	EventViewerLeft StreamEventType = "viewer.left"

	// EventStreamError indicates an error occurred
	EventStreamError StreamEventType = "stream.error"
)
