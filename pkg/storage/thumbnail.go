package storage

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// ThumbnailGenerator generates thumbnails from video frames
type ThumbnailGenerator struct {
	config      ThumbnailConfig
	logger      logger.Logger
	thumbnails  map[string][]ThumbnailInfo
	mu          sync.RWMutex
	lastCapture time.Time
}

// NewThumbnailGenerator creates a new thumbnail generator
func NewThumbnailGenerator(config ThumbnailConfig, log logger.Logger) *ThumbnailGenerator {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &ThumbnailGenerator{
		config:     config,
		logger:     log,
		thumbnails: make(map[string][]ThumbnailInfo),
	}
}

// GenerateThumbnail generates thumbnails from a video frame
func (g *ThumbnailGenerator) GenerateThumbnail(ctx context.Context, recordingID string, frame image.Image, timestamp time.Time) ([]ThumbnailInfo, error) {
	if !g.config.Enabled {
		return nil, nil
	}

	// Check if we should capture thumbnail based on interval
	if !g.ShouldCaptureThumbnail() {
		return nil, nil
	}

	g.lastCapture = timestamp

	thumbnails := make([]ThumbnailInfo, 0, len(g.config.Sizes))

	// Generate thumbnail for each size
	for _, size := range g.config.Sizes {
		thumbnail, err := g.generateSingleThumbnail(ctx, recordingID, frame, timestamp, size)
		if err != nil {
			g.logger.Error("Failed to generate thumbnail",
				logger.Field{Key: "recording_id", Value: recordingID},
				logger.Field{Key: "size", Value: size.Name},
				logger.Field{Key: "error", Value: err},
			)
			continue
		}

		thumbnails = append(thumbnails, *thumbnail)
	}

	// Store thumbnails
	g.mu.Lock()
	g.thumbnails[recordingID] = append(g.thumbnails[recordingID], thumbnails...)
	g.mu.Unlock()

	g.logger.Info("Thumbnails generated",
		logger.Field{Key: "recording_id", Value: recordingID},
		logger.Field{Key: "count", Value: len(thumbnails)},
	)

	return thumbnails, nil
}

// generateSingleThumbnail generates a single thumbnail
func (g *ThumbnailGenerator) generateSingleThumbnail(ctx context.Context, recordingID string, frame image.Image, timestamp time.Time, size ThumbnailSize) (*ThumbnailInfo, error) {
	// Resize image
	resized := g.resizeImage(frame, size.Width, size.Height)

	// Generate filename
	filename := fmt.Sprintf("%s_%s_%d.%s",
		recordingID,
		size.Name,
		timestamp.Unix(),
		g.config.Format,
	)

	// Create thumbnails directory
	thumbnailDir := "./thumbnails/" + recordingID
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	filePath := filepath.Join(thumbnailDir, filename)

	// Save thumbnail
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer file.Close()

	// Encode image
	switch g.config.Format {
	case "jpeg", "jpg":
		opts := &jpeg.Options{Quality: size.Quality}
		if err := jpeg.Encode(file, resized, opts); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "png":
		if err := png.Encode(file, resized); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", g.config.Format)
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat thumbnail: %w", err)
	}

	thumbnail := &ThumbnailInfo{
		RecordingID: recordingID,
		Timestamp:   timestamp,
		Size:        size.Name,
		Width:       size.Width,
		Height:      size.Height,
		Path:        filePath,
		FileSize:    stat.Size(),
		Uploaded:    false,
	}

	// Upload to storage if configured
	if g.config.AutoUpload && g.config.Storage != nil {
		go g.uploadThumbnail(context.Background(), thumbnail)
	}

	return thumbnail, nil
}

// resizeImage resizes an image to the specified dimensions
func (g *ThumbnailGenerator) resizeImage(src image.Image, width, height int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Simple nearest-neighbor resize
	// For production, use a proper image library like github.com/nfnt/resize
	xRatio := float64(bounds.Dx()) / float64(width)
	yRatio := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * xRatio)
			srcY := int(float64(y) * yRatio)
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return dst
}

// uploadThumbnail uploads a thumbnail to storage
func (g *ThumbnailGenerator) uploadThumbnail(ctx context.Context, thumbnail *ThumbnailInfo) {
	file, err := os.Open(thumbnail.Path)
	if err != nil {
		g.logger.Error("Failed to open thumbnail for upload",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: thumbnail.Path},
		)
		return
	}
	defer file.Close()

	// Generate remote path
	remotePath := fmt.Sprintf("thumbnails/%s/%s",
		thumbnail.RecordingID,
		filepath.Base(thumbnail.Path),
	)

	// Upload to storage
	contentType := "image/jpeg"
	if g.config.Format == "png" {
		contentType = "image/png"
	}

	err = g.config.Storage.Upload(ctx, remotePath, file, thumbnail.FileSize, contentType)
	if err != nil {
		g.logger.Error("Failed to upload thumbnail",
			logger.Field{Key: "error", Value: err},
			logger.Field{Key: "path", Value: remotePath},
		)
		return
	}

	thumbnail.RemotePath = remotePath
	thumbnail.Uploaded = true

	g.logger.Info("Thumbnail uploaded",
		logger.Field{Key: "recording_id", Value: thumbnail.RecordingID},
		logger.Field{Key: "size", Value: thumbnail.Size},
		logger.Field{Key: "remote_path", Value: remotePath},
	)
}

// ShouldCaptureThumbnail checks if a thumbnail should be captured based on interval
func (g *ThumbnailGenerator) ShouldCaptureThumbnail() bool {
	if g.lastCapture.IsZero() {
		return true
	}

	return time.Since(g.lastCapture) >= g.config.Interval
}

// GetThumbnails returns all thumbnails for a recording
func (g *ThumbnailGenerator) GetThumbnails(recordingID string) []ThumbnailInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	thumbnails, ok := g.thumbnails[recordingID]
	if !ok {
		return []ThumbnailInfo{}
	}

	result := make([]ThumbnailInfo, len(thumbnails))
	copy(result, thumbnails)
	return result
}

// GetThumbnailsBySize returns thumbnails for a specific size
func (g *ThumbnailGenerator) GetThumbnailsBySize(recordingID, sizeName string) []ThumbnailInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	thumbnails, ok := g.thumbnails[recordingID]
	if !ok {
		return []ThumbnailInfo{}
	}

	result := make([]ThumbnailInfo, 0)
	for _, thumb := range thumbnails {
		if thumb.Size == sizeName {
			result = append(result, thumb)
		}
	}

	return result
}

// Close closes the thumbnail generator
func (g *ThumbnailGenerator) Close() error {
	g.logger.Info("Thumbnail generator closed")
	return nil
}

// GenerateThumbnailFromFile generates thumbnails from a video file
// This is a placeholder - actual implementation requires video decoding
func GenerateThumbnailFromFile(videoPath string, timestamp time.Duration, config ThumbnailConfig) ([]ThumbnailInfo, error) {
	// TODO: Implement using ffmpeg or a video processing library
	// Example: extract frame at timestamp using ffmpeg
	// ffmpeg -ss <timestamp> -i <videoPath> -vframes 1 -f image2 -
	return nil, fmt.Errorf("not implemented: requires video decoding library")
}

// BatchGenerateThumbnails generates thumbnails at regular intervals from a video file
func BatchGenerateThumbnails(ctx context.Context, recordingID, videoPath string, interval time.Duration, config ThumbnailConfig) ([]ThumbnailInfo, error) {
	// TODO: Implement using ffmpeg or video processing library
	// Example: ffmpeg -i <videoPath> -vf "fps=1/<interval>" -f image2 thumbnail_%03d.jpg
	return nil, fmt.Errorf("not implemented: requires video decoding library")
}

// ExtractFrameFromVideo extracts a single frame from a video at the specified timestamp
func ExtractFrameFromVideo(videoPath string, timestamp time.Duration) (image.Image, error) {
	// TODO: Implement using ffmpeg or video processing library
	// This would decode the video and extract a frame at the specified timestamp
	return nil, fmt.Errorf("not implemented: requires video decoding library")
}
