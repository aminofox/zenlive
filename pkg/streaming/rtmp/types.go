package rtmp

import (
	"time"

	"github.com/aminofox/zenlive/pkg/types"
)

// RTMP protocol constants
const (
	// Version is the RTMP protocol version
	Version = 3

	// Default chunk size
	DefaultChunkSize = 128

	// Default window acknowledgement size
	DefaultWindowAckSize = 2500000

	// Default peer bandwidth
	DefaultPeerBandwidth = 2500000

	// Default port
	DefaultPort = 1935
)

// Message type IDs
const (
	MessageTypeSetChunkSize     uint8 = 1
	MessageTypeAbortMessage     uint8 = 2
	MessageTypeAcknowledgement  uint8 = 3
	MessageTypeUserControl      uint8 = 4
	MessageTypeWindowAckSize    uint8 = 5
	MessageTypeSetPeerBandwidth uint8 = 6
	MessageTypeAudio            uint8 = 8
	MessageTypeVideo            uint8 = 9
	MessageTypeDataAMF3         uint8 = 15
	MessageTypeSharedObjectAMF3 uint8 = 16
	MessageTypeCommandAMF3      uint8 = 17
	MessageTypeDataAMF0         uint8 = 18
	MessageTypeSharedObjectAMF0 uint8 = 19
	MessageTypeCommandAMF0      uint8 = 20
	MessageTypeAggregate        uint8 = 22
)

// Chunk stream IDs
const (
	ChunkStreamIDProtocolControl = 2
	ChunkStreamIDCommand         = 3
	ChunkStreamIDAudio           = 4
	ChunkStreamIDVideo           = 5
)

// User control event types
const (
	UserControlEventStreamBegin      uint16 = 0
	UserControlEventStreamEOF        uint16 = 1
	UserControlEventStreamDry        uint16 = 2
	UserControlEventSetBufferLength  uint16 = 3
	UserControlEventStreamIsRecorded uint16 = 4
	UserControlEventPingRequest      uint16 = 6
	UserControlEventPingResponse     uint16 = 7
)

// Chunk header format
const (
	ChunkFormat0 byte = 0 // Full header (11 bytes)
	ChunkFormat1 byte = 1 // No message stream ID (7 bytes)
	ChunkFormat2 byte = 2 // Only timestamp delta (3 bytes)
	ChunkFormat3 byte = 3 // No header (0 bytes)
)

// ChunkHeader represents an RTMP chunk header
type ChunkHeader struct {
	// Format is the chunk header format (0-3)
	Format byte

	// ChunkStreamID is the chunk stream ID
	ChunkStreamID uint32

	// Timestamp is the message timestamp
	Timestamp uint32

	// MessageLength is the message body length
	MessageLength uint32

	// MessageTypeID is the message type
	MessageTypeID uint8

	// MessageStreamID is the message stream ID
	MessageStreamID uint32

	// ExtendedTimestamp indicates if extended timestamp is used
	ExtendedTimestamp bool
}

// Message represents an RTMP message
type Message struct {
	// ChunkStreamID is the chunk stream ID
	ChunkStreamID uint32

	// Timestamp is the message timestamp
	Timestamp uint32

	// MessageTypeID is the message type
	MessageTypeID uint8

	// MessageStreamID is the message stream ID
	MessageStreamID uint32

	// Payload is the message payload
	Payload []byte
}

// StreamInfo contains stream metadata
type StreamInfo struct {
	// StreamKey is the unique stream identifier
	StreamKey string

	// StreamID is the RTMP stream ID
	StreamID uint32

	// AppName is the application name
	AppName string

	// PublishType is the publish type (live, record, append)
	PublishType string

	// User is the authenticated user
	User *types.User

	// StartTime is when the stream started
	StartTime time.Time

	// Metadata contains stream metadata (width, height, fps, etc.)
	Metadata map[string]interface{}

	// IsPublishing indicates if the stream is currently publishing
	IsPublishing bool

	// ViewerCount is the number of viewers
	ViewerCount int
}

// ConnectionState represents the RTMP connection state
type ConnectionState int

const (
	// StateInit is the initial state
	StateInit ConnectionState = iota

	// StateHandshake0 is during handshake C0/S0
	StateHandshake0

	// StateHandshake1 is during handshake C1/S1
	StateHandshake1

	// StateHandshake2 is during handshake C2/S2
	StateHandshake2

	// StateConnected is after successful handshake
	StateConnected

	// StatePublishing is when publishing a stream
	StatePublishing

	// StatePlaying is when playing a stream
	StatePlaying

	// StateClosed is when connection is closed
	StateClosed
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateInit:
		return "Init"
	case StateHandshake0:
		return "Handshake0"
	case StateHandshake1:
		return "Handshake1"
	case StateHandshake2:
		return "Handshake2"
	case StateConnected:
		return "Connected"
	case StatePublishing:
		return "Publishing"
	case StatePlaying:
		return "Playing"
	case StateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

// PublishMode represents the publishing mode
type PublishMode string

const (
	// PublishModeLive is for live streaming
	PublishModeLive PublishMode = "live"

	// PublishModeRecord is for recording
	PublishModeRecord PublishMode = "record"

	// PublishModeAppend is for appending to recording
	PublishModeAppend PublishMode = "append"
)
