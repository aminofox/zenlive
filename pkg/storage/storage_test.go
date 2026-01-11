package storage

import (
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

func TestDefaultRecordingConfig(t *testing.T) {
	config := DefaultRecordingConfig()

	if config.Format != FormatMP4 {
		t.Errorf("Expected format %s, got %s", FormatMP4, config.Format)
	}

	if config.SegmentDuration != 10*time.Minute {
		t.Errorf("Expected segment duration 10m, got %v", config.SegmentDuration)
	}

	if config.MaxSegmentSize != 500*1024*1024 {
		t.Errorf("Expected max segment size 500MB, got %d", config.MaxSegmentSize)
	}

	if config.AutoUpload != false {
		t.Error("Expected auto upload to be false")
	}

	if config.Metadata == nil {
		t.Error("Expected metadata map to be initialized")
	}
}

func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()

	if config.Type != StorageTypeLocal {
		t.Errorf("Expected storage type %s, got %s", StorageTypeLocal, config.Type)
	}

	if config.BasePath != "./recordings" {
		t.Errorf("Expected base path ./recordings, got %s", config.BasePath)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}

	if config.RetryDelay != 2*time.Second {
		t.Errorf("Expected retry delay 2s, got %v", config.RetryDelay)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.UseSSL != true {
		t.Error("Expected UseSSL to be true")
	}
}

func TestDefaultThumbnailConfig(t *testing.T) {
	config := DefaultThumbnailConfig()

	if !config.Enabled {
		t.Error("Expected thumbnails to be enabled")
	}

	if config.Interval != 60*time.Second {
		t.Errorf("Expected interval 60s, got %v", config.Interval)
	}

	if config.Format != "jpeg" {
		t.Errorf("Expected format jpeg, got %s", config.Format)
	}

	if len(config.Sizes) != 3 {
		t.Errorf("Expected 3 thumbnail sizes, got %d", len(config.Sizes))
	}

	// Check sizes
	expectedSizes := map[string]struct{ width, height, quality int }{
		"small":  {160, 90, 80},
		"medium": {320, 180, 85},
		"large":  {640, 360, 90},
	}

	for _, size := range config.Sizes {
		expected, ok := expectedSizes[size.Name]
		if !ok {
			t.Errorf("Unexpected size name: %s", size.Name)
			continue
		}

		if size.Width != expected.width {
			t.Errorf("Size %s: expected width %d, got %d", size.Name, expected.width, size.Width)
		}

		if size.Height != expected.height {
			t.Errorf("Size %s: expected height %d, got %d", size.Name, expected.height, size.Height)
		}

		if size.Quality != expected.quality {
			t.Errorf("Size %s: expected quality %d, got %d", size.Name, expected.quality, size.Quality)
		}
	}
}

func TestRecordingFormats(t *testing.T) {
	formats := []RecordingFormat{FormatMP4, FormatFLV, FormatHLS}

	if len(formats) != 3 {
		t.Errorf("Expected 3 recording formats, got %d", len(formats))
	}

	if FormatMP4 != "mp4" {
		t.Errorf("Expected FormatMP4 to be 'mp4', got %s", FormatMP4)
	}

	if FormatFLV != "flv" {
		t.Errorf("Expected FormatFLV to be 'flv', got %s", FormatFLV)
	}

	if FormatHLS != "hls" {
		t.Errorf("Expected FormatHLS to be 'hls', got %s", FormatHLS)
	}
}

func TestRecordingStates(t *testing.T) {
	states := []RecordingState{StateIdle, StateRecording, StatePaused, StateStopped, StateError}

	if len(states) != 5 {
		t.Errorf("Expected 5 recording states, got %d", len(states))
	}

	if StateIdle != "idle" {
		t.Errorf("Expected StateIdle to be 'idle', got %s", StateIdle)
	}

	if StateRecording != "recording" {
		t.Errorf("Expected StateRecording to be 'recording', got %s", StateRecording)
	}

	if StatePaused != "paused" {
		t.Errorf("Expected StatePaused to be 'paused', got %s", StatePaused)
	}

	if StateStopped != "stopped" {
		t.Errorf("Expected StateStopped to be 'stopped', got %s", StateStopped)
	}

	if StateError != "error" {
		t.Errorf("Expected StateError to be 'error', got %s", StateError)
	}
}

func TestStorageTypes(t *testing.T) {
	if StorageTypeLocal != "local" {
		t.Errorf("Expected StorageTypeLocal to be 'local', got %s", StorageTypeLocal)
	}

	if StorageTypeS3 != "s3" {
		t.Errorf("Expected StorageTypeS3 to be 's3', got %s", StorageTypeS3)
	}
}

func TestBaseRecorder(t *testing.T) {
	config := DefaultRecordingConfig()
	config.StreamID = "test-stream"
	config.OutputPath = "./test-recordings"

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	recorder := NewBaseRecorder(config, log)

	if recorder == nil {
		t.Fatal("Expected recorder to be created")
	}

	info := recorder.GetInfo()
	if info.StreamID != "test-stream" {
		t.Errorf("Expected stream ID 'test-stream', got %s", info.StreamID)
	}

	if info.State != StateIdle {
		t.Errorf("Expected initial state to be idle, got %s", info.State)
	}

	if info.Format != FormatMP4 {
		t.Errorf("Expected format mp4, got %s", info.Format)
	}

	// Clean up
	recorder.Close()
}

func TestInMemoryMetadataStore(t *testing.T) {
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	store := NewInMemoryMetadataStore(log)

	if store == nil {
		t.Fatal("Expected metadata store to be created")
	}

	// Clean up
	store.Close()
}

func TestLocalStorage(t *testing.T) {
	config := DefaultStorageConfig()
	config.BasePath = "./test-storage"

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	storage, err := NewLocalStorage(config, log)

	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	// Clean up
	storage.Close()
}

func TestThumbnailGenerator(t *testing.T) {
	config := DefaultThumbnailConfig()

	log := logger.NewDefaultLogger(logger.InfoLevel, "text")
	generator := NewThumbnailGenerator(config, log)

	if generator == nil {
		t.Fatal("Expected thumbnail generator to be created")
	}

	if !generator.ShouldCaptureThumbnail() {
		t.Error("Expected first thumbnail capture to be allowed")
	}

	// Clean up
	generator.Close()
}
