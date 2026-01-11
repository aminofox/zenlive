package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/google/uuid"
)

// BaseRecorder provides a base implementation for recording livestreams
type BaseRecorder struct {
	config RecordingConfig
	info   RecordingInfo

	segments       []SegmentInfo
	currentFile    *os.File
	currentSegment *SegmentInfo

	mu     sync.RWMutex
	logger logger.Logger

	// Callbacks
	onSegmentComplete func(segment SegmentInfo)
	onThumbnail       func(thumbnail ThumbnailInfo)
	onError           func(error)
}

// NewBaseRecorder creates a new base recorder
func NewBaseRecorder(config RecordingConfig, log logger.Logger) *BaseRecorder {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &BaseRecorder{
		config:   config,
		logger:   log,
		segments: make([]SegmentInfo, 0),
		info: RecordingInfo{
			ID:       uuid.New().String(),
			StreamID: config.StreamID,
			State:    StateIdle,
			Format:   config.Format,
			Metadata: config.Metadata,
		},
	}
}

// Start begins recording
func (r *BaseRecorder) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.info.State == StateRecording {
		return ErrRecordingAlreadyStarted
	}

	r.info.State = StateRecording
	r.info.StartTime = time.Now()
	r.info.Error = nil

	r.logger.Info("Recording started",
		logger.Field{Key: "recording_id", Value: r.info.ID},
		logger.Field{Key: "stream_id", Value: r.info.StreamID},
		logger.Field{Key: "format", Value: r.info.Format},
	)

	// Create first segment
	if err := r.createNewSegment(); err != nil {
		r.info.State = StateError
		r.info.Error = err
		return err
	}

	return nil
}

// Stop ends recording and finalizes all segments
func (r *BaseRecorder) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.info.State != StateRecording && r.info.State != StatePaused {
		return ErrRecordingNotStarted
	}

	// Finalize current segment
	if err := r.finalizeCurrentSegment(); err != nil {
		r.logger.Error("Failed to finalize segment",
			logger.Field{Key: "error", Value: err},
		)
	}

	r.info.State = StateStopped
	r.info.EndTime = time.Now()
	r.info.Duration = r.info.EndTime.Sub(r.info.StartTime)

	r.logger.Info("Recording stopped",
		logger.Field{Key: "recording_id", Value: r.info.ID},
		logger.Field{Key: "duration", Value: r.info.Duration},
		logger.Field{Key: "segments", Value: len(r.segments)},
		logger.Field{Key: "size", Value: r.info.Size},
	)

	// Upload pending segments if auto-upload is enabled
	if r.config.AutoUpload && r.config.Storage != nil {
		go r.uploadPendingSegments(context.Background())
	}

	return nil
}

// Pause temporarily pauses recording
func (r *BaseRecorder) Pause(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.info.State != StateRecording {
		return ErrRecordingNotStarted
	}

	r.info.State = StatePaused

	r.logger.Info("Recording paused",
		logger.Field{Key: "recording_id", Value: r.info.ID},
	)

	return nil
}

// Resume resumes a paused recording
func (r *BaseRecorder) Resume(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.info.State != StatePaused {
		return ErrRecordingPaused
	}

	r.info.State = StateRecording

	r.logger.Info("Recording resumed",
		logger.Field{Key: "recording_id", Value: r.info.ID},
	)

	return nil
}

// GetInfo returns current recording information
func (r *BaseRecorder) GetInfo() RecordingInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := r.info
	if r.info.State == StateRecording {
		info.Duration = time.Since(r.info.StartTime)
	}

	return info
}

// GetSegments returns all recorded segments
func (r *BaseRecorder) GetSegments() []SegmentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	segments := make([]SegmentInfo, len(r.segments))
	copy(segments, r.segments)
	return segments
}

// Close closes the recorder and releases resources
func (r *BaseRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentFile != nil {
		if err := r.currentFile.Close(); err != nil {
			r.logger.Error("Failed to close current file",
				logger.Field{Key: "error", Value: err},
			)
		}
		r.currentFile = nil
	}

	r.logger.Info("Recorder closed",
		logger.Field{Key: "recording_id", Value: r.info.ID},
	)

	return nil
}

// SetOnSegmentComplete sets the callback for segment completion
func (r *BaseRecorder) SetOnSegmentComplete(callback func(SegmentInfo)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onSegmentComplete = callback
}

// SetOnThumbnail sets the callback for thumbnail generation
func (r *BaseRecorder) SetOnThumbnail(callback func(ThumbnailInfo)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onThumbnail = callback
}

// SetOnError sets the callback for errors
func (r *BaseRecorder) SetOnError(callback func(error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onError = callback
}

// createNewSegment creates a new recording segment
func (r *BaseRecorder) createNewSegment() error {
	// Close current segment if exists
	if r.currentFile != nil {
		if err := r.finalizeCurrentSegment(); err != nil {
			return err
		}
	}

	// Generate segment filename
	segmentIndex := len(r.segments)
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%s_segment_%d_%d.%s",
		r.info.StreamID, segmentIndex, timestamp, r.config.Format)

	segmentPath := filepath.Join(r.config.OutputPath, filename)

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(segmentPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Open new segment file
	file, err := os.Create(segmentPath)
	if err != nil {
		return fmt.Errorf("failed to create segment file: %w", err)
	}

	r.currentFile = file
	r.currentSegment = &SegmentInfo{
		Index:     segmentIndex,
		Path:      segmentPath,
		StartTime: time.Now(),
		Uploaded:  false,
	}

	r.logger.Info("New segment created",
		logger.Field{Key: "recording_id", Value: r.info.ID},
		logger.Field{Key: "segment_index", Value: segmentIndex},
		logger.Field{Key: "path", Value: segmentPath},
	)

	return nil
}

// finalizeCurrentSegment finalizes the current segment
func (r *BaseRecorder) finalizeCurrentSegment() error {
	if r.currentFile == nil || r.currentSegment == nil {
		return nil
	}

	// Sync and close file
	if err := r.currentFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment file: %w", err)
	}

	if err := r.currentFile.Close(); err != nil {
		return fmt.Errorf("failed to close segment file: %w", err)
	}

	r.currentFile = nil

	// Get file size
	stat, err := os.Stat(r.currentSegment.Path)
	if err != nil {
		return fmt.Errorf("failed to stat segment file: %w", err)
	}

	r.currentSegment.EndTime = time.Now()
	r.currentSegment.Duration = r.currentSegment.EndTime.Sub(r.currentSegment.StartTime)
	r.currentSegment.Size = stat.Size()

	// Add to segments list
	r.segments = append(r.segments, *r.currentSegment)
	r.info.SegmentCount = len(r.segments)
	r.info.Size += r.currentSegment.Size

	r.logger.Info("Segment finalized",
		logger.Field{Key: "recording_id", Value: r.info.ID},
		logger.Field{Key: "segment_index", Value: r.currentSegment.Index},
		logger.Field{Key: "size", Value: r.currentSegment.Size},
		logger.Field{Key: "duration", Value: r.currentSegment.Duration},
	)

	// Call segment complete callback
	if r.onSegmentComplete != nil {
		segment := *r.currentSegment
		go r.onSegmentComplete(segment)
	}

	// Upload segment if auto-upload is enabled
	if r.config.AutoUpload && r.config.Storage != nil {
		segment := *r.currentSegment
		go r.uploadSegment(context.Background(), &segment)
	}

	r.currentSegment = nil

	return nil
}

// shouldRotateSegment checks if segment should be rotated
func (r *BaseRecorder) shouldRotateSegment() bool {
	if r.currentSegment == nil {
		return false
	}

	// Check duration limit
	if r.config.SegmentDuration > 0 {
		duration := time.Since(r.currentSegment.StartTime)
		if duration >= r.config.SegmentDuration {
			return true
		}
	}

	// Check size limit
	if r.config.MaxSegmentSize > 0 && r.currentFile != nil {
		stat, err := r.currentFile.Stat()
		if err == nil && stat.Size() >= r.config.MaxSegmentSize {
			return true
		}
	}

	return false
}

// uploadSegment uploads a segment to storage
func (r *BaseRecorder) uploadSegment(ctx context.Context, segment *SegmentInfo) {
	if segment.Uploaded {
		return
	}

	file, err := os.Open(segment.Path)
	if err != nil {
		r.logger.Error("Failed to open segment for upload",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: segment.Path},
		)
		return
	}
	defer file.Close()

	// Generate remote path
	remotePath := fmt.Sprintf("recordings/%s/segments/%s",
		r.info.StreamID, filepath.Base(segment.Path))

	// Upload to storage
	err = r.config.Storage.Upload(ctx, remotePath, file, segment.Size, "video/mp4")
	if err != nil {
		r.logger.Error("Failed to upload segment",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: remotePath},
		)
		return
	}

	segment.RemotePath = remotePath
	segment.Uploaded = true

	r.logger.Info("Segment uploaded",
		logger.Field{Key: "recording_id", Value: r.info.ID},
		logger.Field{Key: "segment_index", Value: segment.Index},
		logger.Field{Key: "remote_path", Value: remotePath},
	)
}

// uploadPendingSegments uploads all pending segments
func (r *BaseRecorder) uploadPendingSegments(ctx context.Context) {
	r.mu.RLock()
	segments := make([]SegmentInfo, len(r.segments))
	copy(segments, r.segments)
	r.mu.RUnlock()

	for i := range segments {
		if !segments[i].Uploaded {
			r.uploadSegment(ctx, &segments[i])
		}
	}
}
