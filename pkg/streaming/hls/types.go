// Package hls implements HTTP Live Streaming (HLS) protocol support.
// HLS is an adaptive bitrate streaming protocol developed by Apple that
// breaks streams into small HTTP-based file segments for delivery.
package hls

import (
	"sync"
	"time"
)

// Constants for HLS protocol
const (
	// DefaultSegmentDuration is the default duration for each TS segment (in seconds)
	DefaultSegmentDuration = 6

	// DefaultPlaylistSize is the default number of segments to keep in playlist
	DefaultPlaylistSize = 5

	// DefaultDVRWindowSize is the default DVR window size in seconds
	DefaultDVRWindowSize = 60

	// TSPacketSize is the MPEG-TS packet size (188 bytes)
	TSPacketSize = 188

	// TSExtendedPacketSize is the extended TS packet size with timestamp (192 bytes)
	TSExtendedPacketSize = 192

	// MaxSegmentSize is the maximum size of a TS segment in bytes (10MB)
	MaxSegmentSize = 10 * 1024 * 1024

	// PlaylistTypeVOD represents Video On Demand playlist
	PlaylistTypeVOD = "VOD"

	// PlaylistTypeEvent represents Event playlist
	PlaylistTypeEvent = "EVENT"

	// PlaylistTypeLive represents Live playlist
	PlaylistTypeLive = "LIVE"
)

// SegmentType represents the type of HLS segment
type SegmentType int

const (
	// SegmentTypeVideo represents a video segment
	SegmentTypeVideo SegmentType = iota
	// SegmentTypeAudio represents an audio segment
	SegmentTypeAudio
	// SegmentTypeMuxed represents a muxed audio+video segment
	SegmentTypeMuxed
)

// String returns the string representation of SegmentType
func (s SegmentType) String() string {
	switch s {
	case SegmentTypeVideo:
		return "video"
	case SegmentTypeAudio:
		return "audio"
	case SegmentTypeMuxed:
		return "muxed"
	default:
		return "unknown"
	}
}

// Segment represents a single HLS TS segment
type Segment struct {
	// Index is the sequential index of this segment
	Index uint64

	// Duration is the duration of this segment in seconds
	Duration float64

	// Filename is the name of the segment file
	Filename string

	// Data is the TS segment data
	Data []byte

	// ProgramDateTime is the absolute date/time of the first sample in segment
	ProgramDateTime time.Time

	// Type indicates the segment type (video, audio, or muxed)
	Type SegmentType

	// Discontinuity indicates if this segment has a format discontinuity
	Discontinuity bool

	// KeyFrame indicates if this segment starts with a key frame
	KeyFrame bool

	// CreatedAt is when this segment was created
	CreatedAt time.Time
}

// MediaPlaylist represents an HLS media playlist (variant stream)
type MediaPlaylist struct {
	// Version is the HLS protocol version (default 3)
	Version int

	// TargetDuration is the maximum segment duration
	TargetDuration int

	// MediaSequence is the sequence number of the first segment
	MediaSequence uint64

	// Segments contains the list of segments in this playlist
	Segments []*Segment

	// PlaylistType is the type of playlist (VOD, EVENT, LIVE)
	PlaylistType string

	// EndList indicates if this is the final playlist (VOD)
	EndList bool

	// DVREnabled indicates if DVR is enabled for this playlist
	DVREnabled bool

	// DVRWindowSize is the DVR window size in seconds
	DVRWindowSize int

	mu sync.RWMutex
}

// Variant represents a quality variant in ABR streaming
type Variant struct {
	// Bandwidth is the peak bandwidth in bits per second
	Bandwidth int

	// AverageBandwidth is the average bandwidth in bits per second
	AverageBandwidth int

	// Codecs is the codec string (e.g., "avc1.42E01E,mp4a.40.2")
	Codecs string

	// Resolution is the video resolution (e.g., "1920x1080")
	Resolution string

	// FrameRate is the maximum frame rate
	FrameRate float64

	// URI is the URI of the media playlist for this variant
	URI string

	// Name is a human-readable name for this variant
	Name string

	// Width is the video width in pixels
	Width int

	// Height is the video height in pixels
	Height int

	// VideoBitrate is the video bitrate in bits per second
	VideoBitrate int

	// AudioBitrate is the audio bitrate in bits per second
	AudioBitrate int
}

// MasterPlaylist represents an HLS master playlist
type MasterPlaylist struct {
	// Version is the HLS protocol version
	Version int

	// Variants contains all quality variants
	Variants []*Variant

	// CreatedAt is when this master playlist was created
	CreatedAt time.Time

	mu sync.RWMutex
}

// StreamInfo contains information about an HLS stream
type StreamInfo struct {
	// StreamKey is the unique identifier for this stream
	StreamKey string

	// StartTime is when the stream started
	StartTime time.Time

	// MasterPlaylist is the master playlist for ABR
	MasterPlaylist *MasterPlaylist

	// MediaPlaylists maps variant name to media playlist
	MediaPlaylists map[string]*MediaPlaylist

	// SegmentCount is the total number of segments created
	SegmentCount uint64

	// Active indicates if the stream is currently active
	Active bool

	// DVREnabled indicates if DVR is enabled
	DVREnabled bool

	mu sync.RWMutex
}

// TransmuxerConfig contains configuration for HLS transmuxer
type TransmuxerConfig struct {
	// SegmentDuration is the target duration for each segment in seconds
	SegmentDuration int

	// PlaylistSize is the number of segments to keep in playlist
	PlaylistSize int

	// EnableDVR enables DVR functionality
	EnableDVR bool

	// DVRWindowSize is the DVR window size in seconds
	DVRWindowSize int

	// EnableABR enables adaptive bitrate streaming
	EnableABR bool

	// Variants contains the quality variants for ABR
	Variants []*Variant

	// OutputDir is the directory to store segments
	OutputDir string

	// DeleteOldSegments indicates whether to delete old segments
	DeleteOldSegments bool
}

// DefaultTransmuxerConfig returns a default transmuxer configuration
func DefaultTransmuxerConfig() *TransmuxerConfig {
	return &TransmuxerConfig{
		SegmentDuration:   DefaultSegmentDuration,
		PlaylistSize:      DefaultPlaylistSize,
		EnableDVR:         false,
		DVRWindowSize:     DefaultDVRWindowSize,
		EnableABR:         false,
		DeleteOldSegments: true,
	}
}

// ServerConfig contains configuration for HLS HTTP server
type ServerConfig struct {
	// Address is the server address (e.g., ":8080")
	Address string

	// BaseURL is the base URL for playlist and segment URLs
	BaseURL string

	// EnableCORS enables CORS headers
	EnableCORS bool

	// AllowedOrigins contains allowed origins for CORS
	AllowedOrigins []string

	// CacheControl is the Cache-Control header value
	CacheControl string

	// SegmentCacheControl is the Cache-Control for segments
	SegmentCacheControl string

	// PlaylistCacheControl is the Cache-Control for playlists
	PlaylistCacheControl string

	// EnableCompression enables gzip compression
	EnableCompression bool

	// ReadTimeout is the HTTP read timeout
	ReadTimeout time.Duration

	// WriteTimeout is the HTTP write timeout
	WriteTimeout time.Duration
}

// DefaultServerConfig returns a default HLS server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Address:              ":8080",
		EnableCORS:           true,
		AllowedOrigins:       []string{"*"},
		CacheControl:         "no-cache, no-store, must-revalidate",
		SegmentCacheControl:  "public, max-age=86400",
		PlaylistCacheControl: "no-cache",
		EnableCompression:    true,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         10 * time.Second,
	}
}

// CodecInfo contains codec information for transcoding
type CodecInfo struct {
	// VideoCodec is the video codec (e.g., "h264", "h265")
	VideoCodec string

	// AudioCodec is the audio codec (e.g., "aac", "mp3")
	AudioCodec string

	// VideoProfile is the video profile (e.g., "baseline", "main", "high")
	VideoProfile string

	// AudioProfile is the audio profile
	AudioProfile string

	// Level is the codec level
	Level string
}

// TSWriter represents a MPEG-TS writer for creating HLS segments
type TSWriter struct {
	// packetCount is the number of TS packets written
	packetCount uint64

	// continuityCounter tracks continuity for each PID
	continuityCounter map[uint16]byte

	// pcrBase is the PCR base value
	pcrBase uint64

	// pts is the presentation timestamp
	pts uint64

	// dts is the decode timestamp
	dts uint64

	mu sync.Mutex
}
