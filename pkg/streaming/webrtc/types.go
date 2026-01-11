// Package webrtc provides WebRTC streaming capabilities for ultra-low latency streaming.
// It implements WebRTC protocol using Pion WebRTC library with support for
// publishing, subscribing, and SFU (Selective Forwarding Unit) architecture.
package webrtc

import (
	"time"

	"github.com/pion/webrtc/v3"
)

// Constants for WebRTC configuration
const (
	// DefaultSTUNServer is the default STUN server for NAT traversal
	DefaultSTUNServer = "stun:stun.l.google.com:19302"

	// DefaultICEGatheringTimeout is the timeout for ICE gathering
	DefaultICEGatheringTimeout = 5 * time.Second

	// DefaultPeerConnectionTimeout is the timeout for peer connection establishment
	DefaultPeerConnectionTimeout = 30 * time.Second

	// DefaultMaxBitrate is the default maximum bitrate in bps
	DefaultMaxBitrate = 5_000_000 // 5 Mbps

	// DefaultMinBitrate is the default minimum bitrate in bps
	DefaultMinBitrate = 500_000 // 500 Kbps

	// DefaultTargetBitrate is the default target bitrate in bps
	DefaultTargetBitrate = 2_000_000 // 2 Mbps

	// MaxReconnectAttempts is the maximum number of reconnection attempts
	MaxReconnectAttempts = 5

	// ReconnectBackoff is the backoff duration between reconnect attempts
	ReconnectBackoff = 2 * time.Second
)

// SignalType represents the type of signaling message
type SignalType string

const (
	// SignalTypeOffer represents an SDP offer
	SignalTypeOffer SignalType = "offer"

	// SignalTypeAnswer represents an SDP answer
	SignalTypeAnswer SignalType = "answer"

	// SignalTypeCandidate represents an ICE candidate
	SignalTypeCandidate SignalType = "candidate"

	// SignalTypeSubscribe represents a subscribe request
	SignalTypeSubscribe SignalType = "subscribe"

	// SignalTypeUnsubscribe represents an unsubscribe request
	SignalTypeUnsubscribe SignalType = "unsubscribe"

	// SignalTypeError represents an error message
	SignalTypeError SignalType = "error"
)

// PeerRole defines the role of a peer connection
type PeerRole string

const (
	// PeerRolePublisher represents a peer that publishes media
	PeerRolePublisher PeerRole = "publisher"

	// PeerRoleSubscriber represents a peer that subscribes to media
	PeerRoleSubscriber PeerRole = "subscriber"
)

// PeerState represents the state of a peer connection
type PeerState string

const (
	// PeerStateNew indicates a newly created peer
	PeerStateNew PeerState = "new"

	// PeerStateConnecting indicates the peer is establishing connection
	PeerStateConnecting PeerState = "connecting"

	// PeerStateConnected indicates the peer is connected
	PeerStateConnected PeerState = "connected"

	// PeerStateDisconnected indicates the peer is disconnected
	PeerStateDisconnected PeerState = "disconnected"

	// PeerStateFailed indicates the peer connection failed
	PeerStateFailed PeerState = "failed"

	// PeerStateClosed indicates the peer connection is closed
	PeerStateClosed PeerState = "closed"
)

// Config represents WebRTC configuration
type Config struct {
	// STUNServers is the list of STUN servers for NAT traversal
	STUNServers []string

	// TURNServers is the list of TURN servers for relayed connections
	TURNServers []TURNServer

	// ICEGatheringTimeout is the timeout for ICE candidate gathering
	ICEGatheringTimeout time.Duration

	// PeerConnectionTimeout is the timeout for peer connection establishment
	PeerConnectionTimeout time.Duration

	// MaxBitrate is the maximum bitrate in bps
	MaxBitrate int

	// MinBitrate is the minimum bitrate in bps
	MinBitrate int

	// TargetBitrate is the target bitrate in bps
	TargetBitrate int

	// EnableBWE enables bandwidth estimation
	EnableBWE bool

	// EnableNACK enables NACK-based packet loss recovery
	EnableNACK bool

	// EnablePLI enables Picture Loss Indication
	EnablePLI bool

	// EnableFIR enables Full Intra Request
	EnableFIR bool
}

// TURNServer represents a TURN server configuration
type TURNServer struct {
	// URLs is the list of TURN server URLs
	URLs []string

	// Username for TURN authentication
	Username string

	// Credential for TURN authentication
	Credential string
}

// SignalMessage represents a signaling message
type SignalMessage struct {
	// Type is the type of signaling message
	Type SignalType `json:"type"`

	// StreamID is the stream identifier
	StreamID string `json:"stream_id,omitempty"`

	// PeerID is the peer identifier
	PeerID string `json:"peer_id,omitempty"`

	// SDP is the Session Description Protocol data
	SDP *webrtc.SessionDescription `json:"sdp,omitempty"`

	// Candidate is the ICE candidate
	Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`

	// Error message if Type is SignalTypeError
	Error string `json:"error,omitempty"`

	// Metadata contains additional signaling metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PeerConnection represents a WebRTC peer connection
type PeerConnection struct {
	// ID is the unique peer connection identifier
	ID string

	// StreamID is the associated stream identifier
	StreamID string

	// Role is the peer role (publisher or subscriber)
	Role PeerRole

	// State is the current connection state
	State PeerState

	// PC is the underlying Pion WebRTC peer connection
	PC *webrtc.PeerConnection

	// Tracks contains the media tracks
	Tracks []*Track

	// ICECandidates contains the gathered ICE candidates
	ICECandidates []webrtc.ICECandidateInit

	// CreatedAt is the creation timestamp
	CreatedAt time.Time

	// ConnectedAt is the connection established timestamp
	ConnectedAt *time.Time

	// DisconnectedAt is the disconnection timestamp
	DisconnectedAt *time.Time

	// Stats contains connection statistics
	Stats *PeerStats
}

// Track represents a media track (audio or video)
type Track struct {
	// ID is the track identifier
	ID string

	// Kind is the track kind (audio or video)
	Kind string

	// Codec is the codec name
	Codec string

	// SSRC is the synchronization source identifier
	SSRC uint32

	// LocalTrack is the local track for publishers
	LocalTrack *webrtc.TrackLocalStaticRTP

	// RemoteTrack is the remote track for subscribers
	RemoteTrack *webrtc.TrackRemote

	// RTPSender is the RTP sender
	RTPSender *webrtc.RTPSender

	// RTPReceiver is the RTP receiver
	RTPReceiver *webrtc.RTPReceiver
}

// PeerStats represents peer connection statistics
type PeerStats struct {
	// PacketsSent is the number of packets sent
	PacketsSent uint64

	// PacketsReceived is the number of packets received
	PacketsReceived uint64

	// BytesSent is the number of bytes sent
	BytesSent uint64

	// BytesReceived is the number of bytes received
	BytesReceived uint64

	// PacketsLost is the number of packets lost
	PacketsLost uint64

	// Jitter is the packet jitter in seconds
	Jitter float64

	// RTT is the round-trip time in seconds
	RTT float64

	// Bitrate is the current bitrate in bps
	Bitrate int

	// LastUpdated is the last update timestamp
	LastUpdated time.Time
}

// StreamInfo represents WebRTC stream information
type StreamInfo struct {
	// ID is the stream identifier
	ID string

	// Name is the stream name
	Name string

	// Publisher is the publishing peer connection
	Publisher *PeerConnection

	// Subscribers is the list of subscribing peer connections
	Subscribers []*PeerConnection

	// CreatedAt is the creation timestamp
	CreatedAt time.Time

	// StartedAt is the stream start timestamp
	StartedAt *time.Time

	// EndedAt is the stream end timestamp
	EndedAt *time.Time

	// Metadata contains additional stream metadata
	Metadata map[string]interface{}
}

// ICEServer represents an ICE server configuration
type ICEServer struct {
	// URLs is the list of server URLs
	URLs []string

	// Username for authentication (optional)
	Username string

	// Credential for authentication (optional)
	Credential string
}

// SignalingServerConfig represents signaling server configuration
type SignalingServerConfig struct {
	// ListenAddr is the WebSocket server listen address
	ListenAddr string

	// Path is the WebSocket endpoint path
	Path string

	// EnableCORS enables CORS support
	EnableCORS bool

	// AllowedOrigins is the list of allowed origins for CORS
	AllowedOrigins []string

	// ReadTimeout is the WebSocket read timeout
	ReadTimeout time.Duration

	// WriteTimeout is the WebSocket write timeout
	WriteTimeout time.Duration

	// PingInterval is the WebSocket ping interval
	PingInterval time.Duration

	// MaxMessageSize is the maximum message size in bytes
	MaxMessageSize int64
}

// SFUConfig represents SFU configuration
type SFUConfig struct {
	// WebRTCConfig is the WebRTC configuration
	WebRTCConfig Config

	// MaxPublishersPerStream is the maximum publishers per stream
	MaxPublishersPerStream int

	// MaxSubscribersPerStream is the maximum subscribers per stream
	MaxSubscribersPerStream int

	// MaxStreams is the maximum number of concurrent streams
	MaxStreams int

	// EnableSimulcast enables simulcast support
	EnableSimulcast bool

	// EnableSVC enables Scalable Video Coding
	EnableSVC bool
}

// BWEConfig represents bandwidth estimation configuration
type BWEConfig struct {
	// MinBitrate is the minimum bitrate in bps
	MinBitrate int

	// MaxBitrate is the maximum bitrate in bps
	MaxBitrate int

	// StartBitrate is the initial bitrate in bps
	StartBitrate int

	// ProbeInterval is the interval for bandwidth probing
	ProbeInterval time.Duration

	// RampUpFactor is the factor for bitrate increase
	RampUpFactor float64

	// RampDownFactor is the factor for bitrate decrease
	RampDownFactor float64

	// LossThreshold is the packet loss threshold for bitrate reduction
	LossThreshold float64

	// RTTThreshold is the RTT threshold for bitrate reduction in milliseconds
	RTTThreshold time.Duration
}

// DefaultConfig returns the default WebRTC configuration
func DefaultConfig() Config {
	return Config{
		STUNServers:           []string{DefaultSTUNServer},
		TURNServers:           []TURNServer{},
		ICEGatheringTimeout:   DefaultICEGatheringTimeout,
		PeerConnectionTimeout: DefaultPeerConnectionTimeout,
		MaxBitrate:            DefaultMaxBitrate,
		MinBitrate:            DefaultMinBitrate,
		TargetBitrate:         DefaultTargetBitrate,
		EnableBWE:             true,
		EnableNACK:            true,
		EnablePLI:             true,
		EnableFIR:             true,
	}
}

// DefaultSignalingServerConfig returns the default signaling server configuration
func DefaultSignalingServerConfig() SignalingServerConfig {
	return SignalingServerConfig{
		ListenAddr:     ":8081",
		Path:           "/ws",
		EnableCORS:     true,
		AllowedOrigins: []string{"*"},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		PingInterval:   30 * time.Second,
		MaxMessageSize: 1024 * 1024, // 1 MB
	}
}

// DefaultSFUConfig returns the default SFU configuration
func DefaultSFUConfig() SFUConfig {
	return SFUConfig{
		WebRTCConfig:            DefaultConfig(),
		MaxPublishersPerStream:  1,
		MaxSubscribersPerStream: 1000,
		MaxStreams:              100,
		EnableSimulcast:         false,
		EnableSVC:               false,
	}
}

// DefaultBWEConfig returns the default bandwidth estimation configuration
func DefaultBWEConfig() BWEConfig {
	return BWEConfig{
		MinBitrate:     DefaultMinBitrate,
		MaxBitrate:     DefaultMaxBitrate,
		StartBitrate:   DefaultTargetBitrate,
		ProbeInterval:  2 * time.Second,
		RampUpFactor:   1.08,
		RampDownFactor: 0.85,
		LossThreshold:  0.05, // 5% packet loss
		RTTThreshold:   200 * time.Millisecond,
	}
}

// Validate validates the WebRTC configuration
func (c *Config) Validate() error {
	if len(c.STUNServers) == 0 && len(c.TURNServers) == 0 {
		return ErrNoICEServers
	}

	if c.MaxBitrate <= 0 {
		return ErrInvalidBitrate
	}

	if c.MinBitrate <= 0 {
		return ErrInvalidBitrate
	}

	if c.MinBitrate > c.MaxBitrate {
		return ErrInvalidBitrateRange
	}

	if c.TargetBitrate < c.MinBitrate || c.TargetBitrate > c.MaxBitrate {
		return ErrInvalidBitrateRange
	}

	return nil
}

// Validate validates the signaling server configuration
func (c *SignalingServerConfig) Validate() error {
	if c.ListenAddr == "" {
		return ErrInvalidListenAddr
	}

	if c.Path == "" {
		return ErrInvalidPath
	}

	if c.MaxMessageSize <= 0 {
		return ErrInvalidMessageSize
	}

	return nil
}

// Error types for WebRTC
var (
	// ErrNoICEServers indicates no ICE servers configured
	ErrNoICEServers = &WebRTCError{Code: "NO_ICE_SERVERS", Message: "no ICE servers configured"}

	// ErrInvalidBitrate indicates invalid bitrate configuration
	ErrInvalidBitrate = &WebRTCError{Code: "INVALID_BITRATE", Message: "invalid bitrate configuration"}

	// ErrInvalidBitrateRange indicates invalid bitrate range
	ErrInvalidBitrateRange = &WebRTCError{Code: "INVALID_BITRATE_RANGE", Message: "min bitrate must be less than max bitrate"}

	// ErrInvalidListenAddr indicates invalid listen address
	ErrInvalidListenAddr = &WebRTCError{Code: "INVALID_LISTEN_ADDR", Message: "invalid listen address"}

	// ErrInvalidPath indicates invalid WebSocket path
	ErrInvalidPath = &WebRTCError{Code: "INVALID_PATH", Message: "invalid WebSocket path"}

	// ErrInvalidMessageSize indicates invalid message size
	ErrInvalidMessageSize = &WebRTCError{Code: "INVALID_MESSAGE_SIZE", Message: "invalid message size"}

	// ErrPeerNotFound indicates peer not found
	ErrPeerNotFound = &WebRTCError{Code: "PEER_NOT_FOUND", Message: "peer not found"}

	// ErrStreamNotFound indicates stream not found
	ErrStreamNotFound = &WebRTCError{Code: "STREAM_NOT_FOUND", Message: "stream not found"}

	// ErrPublisherExists indicates publisher already exists
	ErrPublisherExists = &WebRTCError{Code: "PUBLISHER_EXISTS", Message: "publisher already exists for stream"}

	// ErrMaxSubscribersReached indicates maximum subscribers reached
	ErrMaxSubscribersReached = &WebRTCError{Code: "MAX_SUBSCRIBERS", Message: "maximum subscribers reached"}

	// ErrInvalidSDP indicates invalid SDP
	ErrInvalidSDP = &WebRTCError{Code: "INVALID_SDP", Message: "invalid SDP"}

	// ErrConnectionFailed indicates connection failed
	ErrConnectionFailed = &WebRTCError{Code: "CONNECTION_FAILED", Message: "peer connection failed"}
)

// WebRTCError represents a WebRTC-specific error
type WebRTCError struct {
	Code    string
	Message string
}

// Error implements the error interface
func (e *WebRTCError) Error() string {
	return e.Code + ": " + e.Message
}
