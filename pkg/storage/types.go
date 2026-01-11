package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// Recording formats supported by the SDK
const (
	FormatMP4 RecordingFormat = "mp4"
	FormatFLV RecordingFormat = "flv"
	FormatHLS RecordingFormat = "hls"
)

// Recording states
const (
	StateIdle      RecordingState = "idle"
	StateRecording RecordingState = "recording"
	StatePaused    RecordingState = "paused"
	StateStopped   RecordingState = "stopped"
	StateError     RecordingState = "error"
)

// Common errors
var (
	ErrRecordingNotStarted     = errors.New("recording not started")
	ErrRecordingAlreadyStarted = errors.New("recording already started")
	ErrRecordingPaused         = errors.New("recording is paused")
	ErrInvalidFormat           = errors.New("invalid recording format")
	ErrInvalidSegment          = errors.New("invalid segment")
	ErrStorageNotConfigured    = errors.New("storage not configured")
	ErrObjectNotFound          = errors.New("object not found")
	ErrInvalidObjectKey        = errors.New("invalid object key")
	ErrUploadFailed            = errors.New("upload failed")
	ErrDownloadFailed          = errors.New("download failed")
)

// RecordingFormat represents the format of the recording
type RecordingFormat string

// RecordingState represents the current state of a recording
type RecordingState string

// RecordingConfig contains configuration for a recording session
type RecordingConfig struct {
	StreamID        string
	Format          RecordingFormat
	OutputPath      string
	SegmentDuration time.Duration
	MaxSegmentSize  int64
	Storage         Storage
	AutoUpload      bool
	Metadata        map[string]string
}

// DefaultRecordingConfig returns a default recording configuration
func DefaultRecordingConfig() RecordingConfig {
	return RecordingConfig{
		Format:          FormatMP4,
		SegmentDuration: 10 * time.Minute,
		MaxSegmentSize:  500 * 1024 * 1024,
		AutoUpload:      false,
		Metadata:        make(map[string]string),
	}
}

// RecordingInfo contains information about a recording session
type RecordingInfo struct {
	ID           string
	StreamID     string
	State        RecordingState
	Format       RecordingFormat
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Size         int64
	SegmentCount int
	CurrentFile  string
	Error        error
	Metadata     map[string]string
}

// SegmentInfo contains information about a recording segment
type SegmentInfo struct {
	Index      int
	Path       string
	RemotePath string
	Size       int64
	Duration   time.Duration
	StartTime  time.Time
	EndTime    time.Time
	Uploaded   bool
}

// Recorder defines the interface for recording livestreams
type Recorder interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	GetInfo() RecordingInfo
	GetSegments() []SegmentInfo
	Close() error
}

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
)

// StorageConfig contains configuration for storage backends
type StorageConfig struct {
	Type            StorageType
	BasePath        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	MaxRetries      int
	RetryDelay      time.Duration
	Timeout         time.Duration
}

// DefaultStorageConfig returns a default storage configuration
func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		Type:       StorageTypeLocal,
		BasePath:   "./recordings",
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		Timeout:    30 * time.Second,
		UseSSL:     true,
	}
}

// StorageObject represents an object in storage
type StorageObject struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	Metadata     map[string]string
}

// Storage defines the interface for storage backends
type Storage interface {
	Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string, maxKeys int) ([]StorageObject, error)
	GetMetadata(ctx context.Context, key string) (map[string]string, error)
	SetMetadata(ctx context.Context, key string, metadata map[string]string) error
	Copy(ctx context.Context, srcKey, dstKey string) error
	GetURL(ctx context.Context, key string, expires time.Duration) (string, error)
	Close() error
}

// ThumbnailSize represents a thumbnail size configuration
type ThumbnailSize struct {
	Name    string
	Width   int
	Height  int
	Quality int
}

// ThumbnailConfig contains configuration for thumbnail generation
type ThumbnailConfig struct {
	Enabled    bool
	Sizes      []ThumbnailSize
	Interval   time.Duration
	Format     string
	Storage    Storage
	AutoUpload bool
}

// DefaultThumbnailConfig returns a default thumbnail configuration
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		Enabled:  true,
		Interval: 60 * time.Second,
		Format:   "jpeg",
		Sizes: []ThumbnailSize{
			{Name: "small", Width: 160, Height: 90, Quality: 80},
			{Name: "medium", Width: 320, Height: 180, Quality: 85},
			{Name: "large", Width: 640, Height: 360, Quality: 90},
		},
		AutoUpload: false,
	}
}

// ThumbnailInfo contains information about a generated thumbnail
type ThumbnailInfo struct {
	RecordingID string
	Timestamp   time.Time
	Size        string
	Width       int
	Height      int
	Path        string
	RemotePath  string
	FileSize    int64
	Uploaded    bool
}

// RecordingMetadata contains metadata about a recording
type RecordingMetadata struct {
	RecordingID    string
	StreamID       string
	UserID         string
	Title          string
	Description    string
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	FileSize       int64
	Format         RecordingFormat
	SegmentCount   int
	ThumbnailCount int
	ViewCount      int64
	Tags           []string
	CustomMetadata map[string]string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MetadataQuery contains parameters for querying recording metadata
type MetadataQuery struct {
	StreamID    string
	UserID      string
	Tags        []string
	StartDate   time.Time
	EndDate     time.Time
	MinDuration time.Duration
	MaxDuration time.Duration
	SortBy      string
	SortOrder   string
	Offset      int
	Limit       int
}

// MetadataStore defines the interface for storing and querying recording metadata
type MetadataStore interface {
	Save(ctx context.Context, metadata *RecordingMetadata) error
	Get(ctx context.Context, recordingID string) (*RecordingMetadata, error)
	Update(ctx context.Context, metadata *RecordingMetadata) error
	Delete(ctx context.Context, recordingID string) error
	Query(ctx context.Context, query MetadataQuery) ([]*RecordingMetadata, error)
	IncrementViews(ctx context.Context, recordingID string) error
	Close() error
}
