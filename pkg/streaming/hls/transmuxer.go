// Package hls implements transmuxing from RTMP to HLS
package hls

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// Transmuxer converts RTMP streams to HLS format
type Transmuxer struct {
	config *TransmuxerConfig
	logger logger.Logger

	// streams maps stream key to stream info
	streams map[string]*StreamInfo

	// segmentBuffer buffers data for segment creation
	segmentBuffer map[string]*SegmentBuffer

	// callbacks
	onSegmentComplete func(streamKey string, segment *Segment)
	onStreamStart     func(streamKey string)
	onStreamEnd       func(streamKey string)

	mu sync.RWMutex
}

// SegmentBuffer buffers audio and video data for segment creation
type SegmentBuffer struct {
	streamKey string
	startTime time.Time
	videoData []byte
	audioData []byte
	duration  float64
	keyFrame  bool
}

// NewTransmuxer creates a new HLS transmuxer
func NewTransmuxer(config *TransmuxerConfig, log logger.Logger) (*Transmuxer, error) {
	if config == nil {
		config = DefaultTransmuxerConfig()
	}

	// Create output directory if it doesn't exist
	if config.OutputDir != "" {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	return &Transmuxer{
		config:        config,
		logger:        log,
		streams:       make(map[string]*StreamInfo),
		segmentBuffer: make(map[string]*SegmentBuffer),
	}, nil
}

// StartStream starts transmuxing for a new stream
func (t *Transmuxer) StartStream(streamKey string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.streams[streamKey]; exists {
		return fmt.Errorf("stream %s already exists", streamKey)
	}

	// Create stream info
	streamInfo := &StreamInfo{
		StreamKey:      streamKey,
		StartTime:      time.Now(),
		MediaPlaylists: make(map[string]*MediaPlaylist),
		SegmentCount:   0,
		Active:         true,
		DVREnabled:     t.config.EnableDVR,
	}

	// Create master playlist if ABR is enabled
	if t.config.EnableABR && len(t.config.Variants) > 0 {
		streamInfo.MasterPlaylist = NewMasterPlaylist()
		for _, variant := range t.config.Variants {
			streamInfo.MasterPlaylist.AddVariant(variant)
		}
		streamInfo.MasterPlaylist.SortVariantsByBandwidth()

		// Create media playlist for each variant
		for _, variant := range t.config.Variants {
			playlistType := PlaylistTypeLive
			if t.config.EnableDVR {
				playlistType = PlaylistTypeEvent
			}
			playlist := NewMediaPlaylist(t.config.SegmentDuration, playlistType)
			playlist.DVREnabled = t.config.EnableDVR
			playlist.DVRWindowSize = t.config.DVRWindowSize
			streamInfo.MediaPlaylists[variant.Name] = playlist
		}
	} else {
		// Create single media playlist
		playlistType := PlaylistTypeLive
		if t.config.EnableDVR {
			playlistType = PlaylistTypeEvent
		}
		playlist := NewMediaPlaylist(t.config.SegmentDuration, playlistType)
		playlist.DVREnabled = t.config.EnableDVR
		playlist.DVRWindowSize = t.config.DVRWindowSize
		streamInfo.MediaPlaylists["default"] = playlist
	}

	t.streams[streamKey] = streamInfo

	// Initialize segment buffer
	t.segmentBuffer[streamKey] = &SegmentBuffer{
		streamKey: streamKey,
		startTime: time.Now(),
		videoData: make([]byte, 0),
		audioData: make([]byte, 0),
		duration:  0,
		keyFrame:  false,
	}

	t.logger.Info("Started HLS transmuxing for stream", logger.Field{Key: "streamKey", Value: streamKey})

	// Call callback
	if t.onStreamStart != nil {
		go t.onStreamStart(streamKey)
	}

	return nil
}

// StopStream stops transmuxing for a stream
func (t *Transmuxer) StopStream(streamKey string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	streamInfo, exists := t.streams[streamKey]
	if !exists {
		return fmt.Errorf("stream %s not found", streamKey)
	}

	// Flush remaining data
	if buf, ok := t.segmentBuffer[streamKey]; ok && len(buf.videoData) > 0 {
		t.flushSegment(streamKey, buf)
	}

	// Mark stream as inactive
	streamInfo.Active = false

	// Set end list for all playlists
	for _, playlist := range streamInfo.MediaPlaylists {
		playlist.SetEndList()
	}

	// Save final playlists
	t.savePlaylists(streamKey)

	// Clean up
	delete(t.segmentBuffer, streamKey)

	t.logger.Info("Stopped HLS transmuxing for stream", logger.Field{Key: "streamKey", Value: streamKey})

	// Call callback
	if t.onStreamEnd != nil {
		go t.onStreamEnd(streamKey)
	}

	return nil
}

// WriteVideoFrame writes a video frame to the transmuxer
func (t *Transmuxer) WriteVideoFrame(streamKey string, data []byte, timestamp uint32, isKeyFrame bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	buf, ok := t.segmentBuffer[streamKey]
	if !ok {
		return fmt.Errorf("stream %s not found", streamKey)
	}

	// Check if we should start a new segment
	currentDuration := time.Since(buf.startTime).Seconds()
	shouldFlush := false

	// Flush on key frame if duration >= target
	if isKeyFrame && currentDuration >= float64(t.config.SegmentDuration) {
		shouldFlush = true
	}

	// Flush if segment is too long (1.5x target)
	if currentDuration >= float64(t.config.SegmentDuration)*1.5 {
		shouldFlush = true
	}

	if shouldFlush && len(buf.videoData) > 0 {
		t.flushSegment(streamKey, buf)
		buf = t.segmentBuffer[streamKey] // Get new buffer
	}

	// Append video data
	buf.videoData = append(buf.videoData, data...)
	if isKeyFrame {
		buf.keyFrame = true
	}

	return nil
}

// WriteAudioFrame writes an audio frame to the transmuxer
func (t *Transmuxer) WriteAudioFrame(streamKey string, data []byte, timestamp uint32) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	buf, ok := t.segmentBuffer[streamKey]
	if !ok {
		return fmt.Errorf("stream %s not found", streamKey)
	}

	// Append audio data
	buf.audioData = append(buf.audioData, data...)

	return nil
}

// flushSegment creates a segment from buffered data
func (t *Transmuxer) flushSegment(streamKey string, buf *SegmentBuffer) {
	streamInfo := t.streams[streamKey]
	if streamInfo == nil {
		return
	}

	duration := time.Since(buf.startTime).Seconds()
	if duration < 0.1 {
		duration = float64(t.config.SegmentDuration)
	}

	// Create segment
	segment, err := CreateSegment(streamInfo.SegmentCount, duration, buf.videoData, buf.audioData)
	if err != nil {
		t.logger.Error("Failed to create segment",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "streamKey", Value: streamKey})
		return
	}

	segment.KeyFrame = buf.keyFrame
	segment.Discontinuity = false

	// Save segment to disk if output directory is configured
	if t.config.OutputDir != "" {
		segmentPath := filepath.Join(t.config.OutputDir, streamKey, segment.Filename)
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0755); err != nil {
			t.logger.Error("Failed to create segment directory", logger.Field{Key: "error", Value: err})
		} else {
			if err := os.WriteFile(segmentPath, segment.Data, 0644); err != nil {
				t.logger.Error("Failed to write segment",
					logger.Field{Key: "error", Value: err},
					logger.Field{Key: "path", Value: segmentPath})
			}
		}
	}

	// Add segment to all playlists
	for _, playlist := range streamInfo.MediaPlaylists {
		playlist.AddSegment(segment)

		// Remove old segments if needed
		if t.config.DeleteOldSegments {
			if playlist.DVREnabled {
				// DVR mode: remove segments outside window
				windowStart := time.Now().Add(-time.Duration(playlist.DVRWindowSize) * time.Second)
				removed := playlist.RemoveSegmentsBefore(windowStart)
				if removed > 0 {
					t.logger.Debug("Removed old segments from DVR window",
						logger.Field{Key: "count", Value: removed},
						logger.Field{Key: "streamKey", Value: streamKey})
				}
			} else {
				// Live mode: keep only recent segments
				playlist.RemoveOldSegments(t.config.PlaylistSize)
			}
		}
	}

	// Save playlists
	t.savePlaylists(streamKey)

	// Increment segment count
	streamInfo.SegmentCount++

	t.logger.Debug("Created HLS segment",
		logger.Field{Key: "streamKey", Value: streamKey},
		logger.Field{Key: "index", Value: segment.Index},
		logger.Field{Key: "duration", Value: duration},
		logger.Field{Key: "size", Value: len(segment.Data)})

	// Call callback
	if t.onSegmentComplete != nil {
		go t.onSegmentComplete(streamKey, segment)
	}

	// Reset buffer
	t.segmentBuffer[streamKey] = &SegmentBuffer{
		streamKey: streamKey,
		startTime: time.Now(),
		videoData: make([]byte, 0),
		audioData: make([]byte, 0),
		duration:  0,
		keyFrame:  false,
	}
}

// savePlaylists saves all playlists for a stream
func (t *Transmuxer) savePlaylists(streamKey string) {
	if t.config.OutputDir == "" {
		return
	}

	streamInfo := t.streams[streamKey]
	if streamInfo == nil {
		return
	}

	streamDir := filepath.Join(t.config.OutputDir, streamKey)

	// Save master playlist if ABR is enabled
	if streamInfo.MasterPlaylist != nil {
		masterPath := filepath.Join(streamDir, "master.m3u8")
		content := streamInfo.MasterPlaylist.Render()
		if err := os.WriteFile(masterPath, []byte(content), 0644); err != nil {
			t.logger.Error("Failed to write master playlist",
				logger.Field{Key: "error", Value: err},
				logger.Field{Key: "path", Value: masterPath})
		}
	}

	// Save media playlists
	for name, playlist := range streamInfo.MediaPlaylists {
		filename := "playlist.m3u8"
		if name != "default" {
			filename = fmt.Sprintf("playlist_%s.m3u8", name)
		}
		playlistPath := filepath.Join(streamDir, filename)
		content := playlist.Render()
		if err := os.WriteFile(playlistPath, []byte(content), 0644); err != nil {
			t.logger.Error("Failed to write media playlist",
				logger.Field{Key: "error", Value: err},
				logger.Field{Key: "path", Value: playlistPath},
				logger.Field{Key: "variant", Value: name})
		}
	}
}

// GetStreamInfo returns information about a stream
func (t *Transmuxer) GetStreamInfo(streamKey string) (*StreamInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	streamInfo, ok := t.streams[streamKey]
	if !ok {
		return nil, fmt.Errorf("stream %s not found", streamKey)
	}

	return streamInfo, nil
}

// GetActiveStreams returns all active stream keys
func (t *Transmuxer) GetActiveStreams() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	keys := make([]string, 0, len(t.streams))
	for key, info := range t.streams {
		if info.Active {
			keys = append(keys, key)
		}
	}
	return keys
}

// SetOnSegmentComplete sets the callback for segment completion
func (t *Transmuxer) SetOnSegmentComplete(callback func(streamKey string, segment *Segment)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onSegmentComplete = callback
}

// SetOnStreamStart sets the callback for stream start
func (t *Transmuxer) SetOnStreamStart(callback func(streamKey string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onStreamStart = callback
}

// SetOnStreamEnd sets the callback for stream end
func (t *Transmuxer) SetOnStreamEnd(callback func(streamKey string)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onStreamEnd = callback
}

// Close closes the transmuxer and cleans up resources
func (t *Transmuxer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stop all active streams
	for streamKey, info := range t.streams {
		if info.Active {
			t.StopStream(streamKey)
		}
	}

	t.logger.Info("Transmuxer closed")
	return nil
}
