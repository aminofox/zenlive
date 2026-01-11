package security

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"time"
)

// WatermarkType defines the type of watermark
type WatermarkType string

const (
	// WatermarkTypeText is a text-based watermark
	WatermarkTypeText WatermarkType = "text"
	// WatermarkTypeImage is an image-based watermark
	WatermarkTypeImage WatermarkType = "image"
	// WatermarkTypeTimestamp adds timestamp watermark
	WatermarkTypeTimestamp WatermarkType = "timestamp"
	// WatermarkTypeUserID adds user ID watermark
	WatermarkTypeUserID WatermarkType = "userid"
)

// WatermarkPosition defines the position of watermark
type WatermarkPosition string

const (
	// WatermarkPositionTopLeft places watermark at top-left
	WatermarkPositionTopLeft WatermarkPosition = "top-left"
	// WatermarkPositionTopRight places watermark at top-right
	WatermarkPositionTopRight WatermarkPosition = "top-right"
	// WatermarkPositionBottomLeft places watermark at bottom-left
	WatermarkPositionBottomLeft WatermarkPosition = "bottom-left"
	// WatermarkPositionBottomRight places watermark at bottom-right
	WatermarkPositionBottomRight WatermarkPosition = "bottom-right"
	// WatermarkPositionCenter places watermark at center
	WatermarkPositionCenter WatermarkPosition = "center"
)

// WatermarkConfig defines watermark configuration
type WatermarkConfig struct {
	// Type is the type of watermark
	Type WatermarkType
	// Position is where to place the watermark
	Position WatermarkPosition
	// Text is the text for text-based watermarks
	Text string
	// Opacity is the opacity of the watermark (0.0 - 1.0)
	Opacity float64
	// Scale is the scale factor for the watermark (0.0 - 1.0)
	Scale float64
	// OffsetX is the horizontal offset in pixels
	OffsetX int
	// OffsetY is the vertical offset in pixels
	OffsetY int
	// ImageData is the watermark image data
	ImageData []byte
	// EnableForensic enables forensic watermarking (invisible)
	EnableForensic bool
	// ForensicID is the unique ID embedded in forensic watermark
	ForensicID string
}

// Watermark represents a watermark instance
type Watermark struct {
	ID        string
	Config    *WatermarkConfig
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WatermarkManager manages watermarks for streams
type WatermarkManager struct {
	mu         sync.RWMutex
	watermarks map[string]*Watermark // streamID -> watermark
	templates  map[string]*WatermarkConfig
	onApply    func(streamID string, watermark *Watermark)
}

// ForensicWatermark represents an invisible watermark for tracking
type ForensicWatermark struct {
	ID        string
	UserID    string
	StreamID  string
	SessionID string
	Timestamp time.Time
	Metadata  map[string]string
}

var (
	// ErrInvalidOpacity is returned for invalid opacity values
	ErrInvalidOpacity = errors.New("opacity must be between 0.0 and 1.0")
	// ErrInvalidScale is returned for invalid scale values
	ErrInvalidScale = errors.New("scale must be between 0.0 and 1.0")
	// ErrWatermarkNotFound is returned when watermark is not found
	ErrWatermarkNotFound = errors.New("watermark not found")
)

// DefaultWatermarkConfig returns a default watermark configuration
func DefaultWatermarkConfig() *WatermarkConfig {
	return &WatermarkConfig{
		Type:     WatermarkTypeText,
		Position: WatermarkPositionBottomRight,
		Opacity:  0.5,
		Scale:    0.1,
		OffsetX:  10,
		OffsetY:  10,
	}
}

// NewWatermarkManager creates a new watermark manager
func NewWatermarkManager() *WatermarkManager {
	return &WatermarkManager{
		watermarks: make(map[string]*Watermark),
		templates:  make(map[string]*WatermarkConfig),
	}
}

// AddTemplate adds a watermark template
func (wm *WatermarkManager) AddTemplate(name string, config *WatermarkConfig) error {
	if err := wm.validateConfig(config); err != nil {
		return err
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.templates[name] = config
	return nil
}

// ApplyWatermark applies a watermark to a stream
func (wm *WatermarkManager) ApplyWatermark(streamID string, config *WatermarkConfig) (*Watermark, error) {
	if err := wm.validateConfig(config); err != nil {
		return nil, err
	}

	watermark := &Watermark{
		ID:        generateWatermarkID(),
		Config:    config,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	wm.mu.Lock()
	wm.watermarks[streamID] = watermark
	wm.mu.Unlock()

	if wm.onApply != nil {
		wm.onApply(streamID, watermark)
	}

	return watermark, nil
}

// ApplyTemplate applies a watermark template to a stream
func (wm *WatermarkManager) ApplyTemplate(streamID, templateName string) (*Watermark, error) {
	wm.mu.RLock()
	config, exists := wm.templates[templateName]
	wm.mu.RUnlock()

	if !exists {
		return nil, errors.New("template not found")
	}

	return wm.ApplyWatermark(streamID, config)
}

// RemoveWatermark removes watermark from a stream
func (wm *WatermarkManager) RemoveWatermark(streamID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.watermarks[streamID]; !exists {
		return ErrWatermarkNotFound
	}

	delete(wm.watermarks, streamID)
	return nil
}

// GetWatermark returns the watermark for a stream
func (wm *WatermarkManager) GetWatermark(streamID string) (*Watermark, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	watermark, exists := wm.watermarks[streamID]
	if !exists {
		return nil, ErrWatermarkNotFound
	}

	return watermark, nil
}

// UpdateWatermark updates an existing watermark
func (wm *WatermarkManager) UpdateWatermark(streamID string, config *WatermarkConfig) error {
	if err := wm.validateConfig(config); err != nil {
		return err
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	watermark, exists := wm.watermarks[streamID]
	if !exists {
		return ErrWatermarkNotFound
	}

	watermark.Config = config
	watermark.UpdatedAt = time.Now()

	return nil
}

// ApplyToFrame applies watermark to a video frame (simple implementation)
func (wm *WatermarkManager) ApplyToFrame(streamID string, frame []byte) ([]byte, error) {
	wm.mu.RLock()
	watermark, exists := wm.watermarks[streamID]
	wm.mu.RUnlock()

	if !exists {
		return frame, nil // No watermark, return original
	}

	// In production, this would use a proper video processing library
	// For now, we'll provide a simple image watermarking example

	config := watermark.Config

	// Decode frame as image
	img, _, err := image.Decode(bytes.NewReader(frame))
	if err != nil {
		return nil, fmt.Errorf("failed to decode frame: %w", err)
	}

	// Create output image
	bounds := img.Bounds()
	output := image.NewRGBA(bounds)
	draw.Draw(output, bounds, img, bounds.Min, draw.Src)

	// Apply watermark based on type
	switch config.Type {
	case WatermarkTypeText, WatermarkTypeTimestamp, WatermarkTypeUserID:
		wm.applyTextWatermark(output, config)
	case WatermarkTypeImage:
		wm.applyImageWatermark(output, config)
	}

	// Encode back to PNG (in production, use original format)
	var buf bytes.Buffer
	if err := png.Encode(&buf, output); err != nil {
		return nil, fmt.Errorf("failed to encode frame: %w", err)
	}

	return buf.Bytes(), nil
}

// applyTextWatermark applies a text watermark to an image
func (wm *WatermarkManager) applyTextWatermark(img *image.RGBA, config *WatermarkConfig) {
	// Simple text watermark implementation
	// In production, use a proper font rendering library

	text := config.Text
	if config.Type == WatermarkTypeTimestamp {
		text = time.Now().Format("2006-01-02 15:04:05")
	}

	// Calculate position
	x, y := wm.calculatePosition(img.Bounds(), 100, 20, config.Position, config.OffsetX, config.OffsetY)

	// Draw simple text (simplified - in production use freetype or similar)
	textColor := color.RGBA{255, 255, 255, uint8(config.Opacity * 255)}
	wm.drawSimpleText(img, x, y, text, textColor)
}

// applyImageWatermark applies an image watermark
func (wm *WatermarkManager) applyImageWatermark(img *image.RGBA, config *WatermarkConfig) {
	if len(config.ImageData) == 0 {
		return
	}

	// Decode watermark image
	wmImg, _, err := image.Decode(bytes.NewReader(config.ImageData))
	if err != nil {
		return
	}

	// Calculate position and size
	wmBounds := wmImg.Bounds()
	scaledWidth := int(float64(wmBounds.Dx()) * config.Scale)
	scaledHeight := int(float64(wmBounds.Dy()) * config.Scale)

	x, y := wm.calculatePosition(img.Bounds(), scaledWidth, scaledHeight, config.Position, config.OffsetX, config.OffsetY)

	// Draw watermark with opacity
	// In production, use proper alpha blending
	for i := 0; i < scaledWidth; i++ {
		for j := 0; j < scaledHeight; j++ {
			srcX := wmBounds.Min.X + i*wmBounds.Dx()/scaledWidth
			srcY := wmBounds.Min.Y + j*wmBounds.Dy()/scaledHeight
			c := wmImg.At(srcX, srcY)
			r, g, b, a := c.RGBA()

			// Apply opacity
			a = uint32(float64(a) * config.Opacity)

			img.Set(x+i, y+j, color.RGBA{
				uint8(r >> 8),
				uint8(g >> 8),
				uint8(b >> 8),
				uint8(a >> 8),
			})
		}
	}
}

// calculatePosition calculates watermark position
func (wm *WatermarkManager) calculatePosition(bounds image.Rectangle, width, height int, position WatermarkPosition, offsetX, offsetY int) (int, int) {
	var x, y int

	switch position {
	case WatermarkPositionTopLeft:
		x = bounds.Min.X + offsetX
		y = bounds.Min.Y + offsetY
	case WatermarkPositionTopRight:
		x = bounds.Max.X - width - offsetX
		y = bounds.Min.Y + offsetY
	case WatermarkPositionBottomLeft:
		x = bounds.Min.X + offsetX
		y = bounds.Max.Y - height - offsetY
	case WatermarkPositionBottomRight:
		x = bounds.Max.X - width - offsetX
		y = bounds.Max.Y - height - offsetY
	case WatermarkPositionCenter:
		x = bounds.Min.X + (bounds.Dx()-width)/2
		y = bounds.Min.Y + (bounds.Dy()-height)/2
	}

	return x, y
}

// drawSimpleText draws simple text on image (placeholder implementation)
func (wm *WatermarkManager) drawSimpleText(img *image.RGBA, x, y int, text string, color color.RGBA) {
	// Simplified text drawing - in production use freetype or similar
	// This just draws a colored rectangle as placeholder
	for i := 0; i < len(text)*8; i++ {
		for j := 0; j < 16; j++ {
			img.Set(x+i, y+j, color)
		}
	}
}

// validateConfig validates watermark configuration
func (wm *WatermarkManager) validateConfig(config *WatermarkConfig) error {
	if config.Opacity < 0.0 || config.Opacity > 1.0 {
		return ErrInvalidOpacity
	}

	if config.Scale < 0.0 || config.Scale > 1.0 {
		return ErrInvalidScale
	}

	return nil
}

// SetApplyCallback sets the callback for when watermark is applied
func (wm *WatermarkManager) SetApplyCallback(callback func(streamID string, watermark *Watermark)) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.onApply = callback
}

// CreateForensicWatermark creates a forensic watermark for tracking
func CreateForensicWatermark(userID, streamID, sessionID string, metadata map[string]string) *ForensicWatermark {
	return &ForensicWatermark{
		ID:        generateForensicID(),
		UserID:    userID,
		StreamID:  streamID,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}
}

// EmbedForensicWatermark embeds forensic watermark data (invisible)
// This is a placeholder - in production, use steganography techniques
func EmbedForensicWatermark(frame []byte, fw *ForensicWatermark) ([]byte, error) {
	// In production, implement LSB steganography or other techniques
	// to embed forensic ID invisibly in the frame
	return frame, nil
}

// ExtractForensicWatermark extracts forensic watermark from frame
// This is a placeholder - in production, implement extraction
func ExtractForensicWatermark(frame []byte) (*ForensicWatermark, error) {
	// In production, implement extraction of embedded forensic data
	return nil, errors.New("forensic watermark extraction not implemented")
}

// generateWatermarkID generates a unique watermark ID
func generateWatermarkID() string {
	return fmt.Sprintf("wm_%d", time.Now().UnixNano())
}

// generateForensicID generates a unique forensic watermark ID
func generateForensicID() string {
	return fmt.Sprintf("fw_%d", time.Now().UnixNano())
}
