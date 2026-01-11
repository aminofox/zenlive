package streaming

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// VideoSource represents a video source in a multi-stream setup
type VideoSource struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	Type       SourceType             `json:"type"`
	StreamURL  string                 `json:"stream_url"`
	Resolution Resolution             `json:"resolution"`
	Bitrate    int                    `json:"bitrate"`
	FPS        int                    `json:"fps"`
	Active     bool                   `json:"active"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	JoinedAt   time.Time              `json:"joined_at"`
	LastUpdate time.Time              `json:"last_update"`
}

// AudioSource represents an audio source in a multi-stream setup
type AudioSource struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	StreamURL  string                 `json:"stream_url"`
	SampleRate int                    `json:"sample_rate"`
	Channels   int                    `json:"channels"`
	Bitrate    int                    `json:"bitrate"`
	Volume     float64                `json:"volume"` // 0.0 to 1.0
	Muted      bool                   `json:"muted"`
	Active     bool                   `json:"active"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	JoinedAt   time.Time              `json:"joined_at"`
}

// SourceType represents the type of video source
type SourceType string

const (
	// SourceTypeCamera represents camera video
	SourceTypeCamera SourceType = "camera"
	// SourceTypeScreen represents screen sharing
	SourceTypeScreen SourceType = "screen"
	// SourceTypeWindow represents window sharing
	SourceTypeWindow SourceType = "window"
	// SourceTypeFile represents file playback
	SourceTypeFile SourceType = "file"
	// SourceTypeExternal represents external RTMP/HLS source
	SourceTypeExternal SourceType = "external"
)

// Resolution represents video resolution
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LayoutType represents the layout type for multi-stream
type LayoutType string

const (
	// LayoutTypeSingle shows a single main stream
	LayoutTypeSingle LayoutType = "single"
	// LayoutTypePIP shows picture-in-picture
	LayoutTypePIP LayoutType = "pip"
	// LayoutTypeGrid shows grid layout
	LayoutTypeGrid LayoutType = "grid"
	// LayoutTypeSideBySide shows side-by-side layout
	LayoutTypeSideBySide LayoutType = "side_by_side"
	// LayoutTypeCustom shows custom layout
	LayoutTypeCustom LayoutType = "custom"
)

// Layout represents the layout configuration for multi-stream
type Layout struct {
	Type         LayoutType        `json:"type"`
	MainSourceID string            `json:"main_source_id,omitempty"` // For PIP and single layouts
	Positions    []*SourcePosition `json:"positions,omitempty"`      // For custom layouts
	GridRows     int               `json:"grid_rows,omitempty"`      // For grid layout
	GridCols     int               `json:"grid_cols,omitempty"`      // For grid layout
}

// SourcePosition represents the position of a source in the layout
type SourcePosition struct {
	SourceID string  `json:"source_id"`
	X        float64 `json:"x"`       // X position (0.0 to 1.0)
	Y        float64 `json:"y"`       // Y position (0.0 to 1.0)
	Width    float64 `json:"width"`   // Width (0.0 to 1.0)
	Height   float64 `json:"height"`  // Height (0.0 to 1.0)
	ZIndex   int     `json:"z_index"` // Z-index for layering
}

// MultiStreamSession represents a multi-stream session with multiple participants
type MultiStreamSession struct {
	ID           string                  `json:"id"`
	StreamID     string                  `json:"stream_id"`
	HostUserID   string                  `json:"host_user_id"`
	VideoSources map[string]*VideoSource `json:"video_sources"`
	AudioSources map[string]*AudioSource `json:"audio_sources"`
	Layout       *Layout                 `json:"layout"`
	MaxSources   int                     `json:"max_sources"`
	Status       SessionStatus           `json:"status"`
	CreatedAt    time.Time               `json:"created_at"`
	StartedAt    *time.Time              `json:"started_at,omitempty"`
	EndedAt      *time.Time              `json:"ended_at,omitempty"`
	mu           sync.RWMutex            `json:"-"`
}

// SessionStatus represents the status of a multi-stream session
type SessionStatus string

const (
	// SessionStatusPending indicates the session is pending
	SessionStatusPending SessionStatus = "pending"
	// SessionStatusActive indicates the session is active
	SessionStatusActive SessionStatus = "active"
	// SessionStatusEnded indicates the session has ended
	SessionStatusEnded SessionStatus = "ended"
)

// MultiStreamManager manages multi-stream sessions
type MultiStreamManager struct {
	sessions       map[string]*MultiStreamSession // sessionID -> MultiStreamSession
	streamSessions map[string]string              // streamID -> sessionID
	mu             sync.RWMutex
	callbacks      MultiStreamCallbacks
}

// MultiStreamCallbacks defines callback functions for multi-stream events
type MultiStreamCallbacks struct {
	OnSessionCreated func(session *MultiStreamSession)
	OnSessionStarted func(session *MultiStreamSession)
	OnSessionEnded   func(session *MultiStreamSession)
	OnSourceAdded    func(session *MultiStreamSession, source interface{})
	OnSourceRemoved  func(session *MultiStreamSession, sourceID string)
	OnLayoutChanged  func(session *MultiStreamSession, layout *Layout)
	OnAudioMixed     func(session *MultiStreamSession, mixedAudio []byte)
}

// NewMultiStreamManager creates a new multi-stream manager
func NewMultiStreamManager() *MultiStreamManager {
	return &MultiStreamManager{
		sessions:       make(map[string]*MultiStreamSession),
		streamSessions: make(map[string]string),
	}
}

// SetCallbacks sets the callback functions for multi-stream events
func (msm *MultiStreamManager) SetCallbacks(callbacks MultiStreamCallbacks) {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	msm.callbacks = callbacks
}

// CreateSession creates a new multi-stream session
func (msm *MultiStreamManager) CreateSession(streamID, hostUserID string, maxSources int) (*MultiStreamSession, error) {
	if streamID == "" {
		return nil, errors.New("stream ID cannot be empty")
	}
	if hostUserID == "" {
		return nil, errors.New("host user ID cannot be empty")
	}
	if maxSources <= 0 {
		maxSources = 4 // Default max sources
	}

	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Check if session already exists for this stream
	if _, exists := msm.streamSessions[streamID]; exists {
		return nil, fmt.Errorf("session already exists for stream: %s", streamID)
	}

	session := &MultiStreamSession{
		ID:           generateMultiStreamID(),
		StreamID:     streamID,
		HostUserID:   hostUserID,
		VideoSources: make(map[string]*VideoSource),
		AudioSources: make(map[string]*AudioSource),
		Layout: &Layout{
			Type: LayoutTypeSingle,
		},
		MaxSources: maxSources,
		Status:     SessionStatusPending,
		CreatedAt:  time.Now(),
	}

	msm.sessions[session.ID] = session
	msm.streamSessions[streamID] = session.ID

	// Trigger callback
	if msm.callbacks.OnSessionCreated != nil {
		msm.callbacks.OnSessionCreated(session)
	}

	return session, nil
}

// StartSession starts a multi-stream session
func (msm *MultiStreamManager) StartSession(sessionID string) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status == SessionStatusActive {
		return errors.New("session is already active")
	}

	if session.Status == SessionStatusEnded {
		return errors.New("cannot start an ended session")
	}

	now := time.Now()
	session.Status = SessionStatusActive
	session.StartedAt = &now

	// Trigger callback
	if msm.callbacks.OnSessionStarted != nil {
		msm.callbacks.OnSessionStarted(session)
	}

	return nil
}

// EndSession ends a multi-stream session
func (msm *MultiStreamManager) EndSession(sessionID string) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status == SessionStatusEnded {
		return errors.New("session is already ended")
	}

	now := time.Now()
	session.Status = SessionStatusEnded
	session.EndedAt = &now

	// Deactivate all sources
	for _, source := range session.VideoSources {
		source.Active = false
	}
	for _, source := range session.AudioSources {
		source.Active = false
	}

	// Trigger callback
	if msm.callbacks.OnSessionEnded != nil {
		msm.callbacks.OnSessionEnded(session)
	}

	return nil
}

// AddVideoSource adds a video source to a session
func (msm *MultiStreamManager) AddVideoSource(sessionID, userID string, sourceType SourceType, streamURL string, resolution Resolution) (*VideoSource, error) {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status == SessionStatusEnded {
		return nil, errors.New("cannot add source to ended session")
	}

	// Check max sources limit
	if len(session.VideoSources) >= session.MaxSources {
		return nil, fmt.Errorf("max sources limit reached: %d", session.MaxSources)
	}

	source := &VideoSource{
		ID:         generateSourceID(),
		UserID:     userID,
		Type:       sourceType,
		StreamURL:  streamURL,
		Resolution: resolution,
		Bitrate:    2000000, // Default 2 Mbps
		FPS:        30,      // Default 30 FPS
		Active:     true,
		JoinedAt:   time.Now(),
		LastUpdate: time.Now(),
	}

	session.VideoSources[source.ID] = source

	// Auto-adjust layout if needed
	msm.autoAdjustLayout(session)

	// Trigger callback
	if msm.callbacks.OnSourceAdded != nil {
		msm.callbacks.OnSourceAdded(session, source)
	}

	return source, nil
}

// AddAudioSource adds an audio source to a session
func (msm *MultiStreamManager) AddAudioSource(sessionID, userID, streamURL string, sampleRate, channels int) (*AudioSource, error) {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status == SessionStatusEnded {
		return nil, errors.New("cannot add source to ended session")
	}

	source := &AudioSource{
		ID:         generateSourceID(),
		UserID:     userID,
		StreamURL:  streamURL,
		SampleRate: sampleRate,
		Channels:   channels,
		Bitrate:    128000, // Default 128 kbps
		Volume:     1.0,    // Full volume
		Muted:      false,
		Active:     true,
		JoinedAt:   time.Now(),
	}

	session.AudioSources[source.ID] = source

	// Trigger callback
	if msm.callbacks.OnSourceAdded != nil {
		msm.callbacks.OnSourceAdded(session, source)
	}

	return source, nil
}

// RemoveVideoSource removes a video source from a session
func (msm *MultiStreamManager) RemoveVideoSource(sessionID, sourceID string) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if _, exists := session.VideoSources[sourceID]; !exists {
		return fmt.Errorf("video source not found: %s", sourceID)
	}

	delete(session.VideoSources, sourceID)

	// Auto-adjust layout after removal
	msm.autoAdjustLayout(session)

	// Trigger callback
	if msm.callbacks.OnSourceRemoved != nil {
		msm.callbacks.OnSourceRemoved(session, sourceID)
	}

	return nil
}

// RemoveAudioSource removes an audio source from a session
func (msm *MultiStreamManager) RemoveAudioSource(sessionID, sourceID string) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if _, exists := session.AudioSources[sourceID]; !exists {
		return fmt.Errorf("audio source not found: %s", sourceID)
	}

	delete(session.AudioSources, sourceID)

	// Trigger callback
	if msm.callbacks.OnSourceRemoved != nil {
		msm.callbacks.OnSourceRemoved(session, sourceID)
	}

	return nil
}

// SetLayout sets the layout for a session
func (msm *MultiStreamManager) SetLayout(sessionID string, layout *Layout) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Validate layout
	if err := msm.validateLayout(session, layout); err != nil {
		return err
	}

	session.Layout = layout

	// Trigger callback
	if msm.callbacks.OnLayoutChanged != nil {
		msm.callbacks.OnLayoutChanged(session, layout)
	}

	return nil
}

// validateLayout validates a layout configuration
func (msm *MultiStreamManager) validateLayout(session *MultiStreamSession, layout *Layout) error {
	switch layout.Type {
	case LayoutTypeSingle:
		if layout.MainSourceID == "" {
			return errors.New("main source ID required for single layout")
		}
		if _, exists := session.VideoSources[layout.MainSourceID]; !exists {
			return fmt.Errorf("main source not found: %s", layout.MainSourceID)
		}
	case LayoutTypePIP:
		if layout.MainSourceID == "" {
			return errors.New("main source ID required for PIP layout")
		}
		if _, exists := session.VideoSources[layout.MainSourceID]; !exists {
			return fmt.Errorf("main source not found: %s", layout.MainSourceID)
		}
	case LayoutTypeGrid:
		if layout.GridRows <= 0 || layout.GridCols <= 0 {
			return errors.New("grid rows and cols must be positive")
		}
	case LayoutTypeCustom:
		if len(layout.Positions) == 0 {
			return errors.New("positions required for custom layout")
		}
	}
	return nil
}

// autoAdjustLayout automatically adjusts layout based on number of sources
func (msm *MultiStreamManager) autoAdjustLayout(session *MultiStreamSession) {
	sourceCount := len(session.VideoSources)

	switch sourceCount {
	case 0:
		session.Layout = &Layout{Type: LayoutTypeSingle}
	case 1:
		// Get the only source
		for sourceID := range session.VideoSources {
			session.Layout = &Layout{
				Type:         LayoutTypeSingle,
				MainSourceID: sourceID,
			}
			break
		}
	case 2:
		session.Layout = &Layout{Type: LayoutTypeSideBySide}
	case 3, 4:
		session.Layout = &Layout{
			Type:     LayoutTypeGrid,
			GridRows: 2,
			GridCols: 2,
		}
	default:
		// For more sources, use grid layout
		cols := 3
		rows := (sourceCount + cols - 1) / cols
		session.Layout = &Layout{
			Type:     LayoutTypeGrid,
			GridRows: rows,
			GridCols: cols,
		}
	}
}

// SetAudioVolume sets the volume for an audio source
func (msm *MultiStreamManager) SetAudioVolume(sessionID, sourceID string, volume float64) error {
	if volume < 0.0 || volume > 1.0 {
		return errors.New("volume must be between 0.0 and 1.0")
	}

	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	source, exists := session.AudioSources[sourceID]
	if !exists {
		return fmt.Errorf("audio source not found: %s", sourceID)
	}

	source.Volume = volume
	return nil
}

// MuteAudioSource mutes or unmutes an audio source
func (msm *MultiStreamManager) MuteAudioSource(sessionID, sourceID string, muted bool) error {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	source, exists := session.AudioSources[sourceID]
	if !exists {
		return fmt.Errorf("audio source not found: %s", sourceID)
	}

	source.Muted = muted
	return nil
}

// GetSession returns a session by ID
func (msm *MultiStreamManager) GetSession(sessionID string) (*MultiStreamSession, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	session, exists := msm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// GetSessionByStream returns a session by stream ID
func (msm *MultiStreamManager) GetSessionByStream(streamID string) (*MultiStreamSession, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	sessionID, exists := msm.streamSessions[streamID]
	if !exists {
		return nil, fmt.Errorf("no session found for stream: %s", streamID)
	}

	session := msm.sessions[sessionID]
	return session, nil
}

// GetActiveSessions returns all active sessions
func (msm *MultiStreamManager) GetActiveSessions() []*MultiStreamSession {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	sessions := make([]*MultiStreamSession, 0)
	for _, session := range msm.sessions {
		if session.Status == SessionStatusActive {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// MixAudio mixes audio from multiple sources
func (msm *MultiStreamManager) MixAudio(sessionID string, audioBuffers map[string][]byte) ([]byte, error) {
	msm.mu.RLock()
	session, exists := msm.sessions[sessionID]
	msm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	// Simple audio mixing: sum samples with volume adjustment
	// In production, use a proper audio mixing library
	mixed := make([]byte, 0)

	for sourceID, buffer := range audioBuffers {
		source, exists := session.AudioSources[sourceID]
		if !exists || source.Muted || !source.Active {
			continue
		}

		// Apply volume (simplified - in production use proper audio processing)
		volumeAdjusted := make([]byte, len(buffer))
		for i, sample := range buffer {
			volumeAdjusted[i] = byte(float64(sample) * source.Volume)
		}

		// Mix with existing (simplified - in production use proper mixing)
		if len(mixed) == 0 {
			mixed = volumeAdjusted
		} else {
			for i := 0; i < len(mixed) && i < len(volumeAdjusted); i++ {
				// Prevent overflow
				sum := int(mixed[i]) + int(volumeAdjusted[i])
				if sum > 255 {
					sum = 255
				}
				mixed[i] = byte(sum)
			}
		}
	}

	// Trigger callback
	if msm.callbacks.OnAudioMixed != nil && len(mixed) > 0 {
		msm.callbacks.OnAudioMixed(session, mixed)
	}

	return mixed, nil
}

var (
	multiStreamIDCounter int64
	sourceIDCounter      int64
)

// generateMultiStreamID generates a unique multi-stream session ID
func generateMultiStreamID() string {
	id := time.Now().UnixNano() + multiStreamIDCounter
	multiStreamIDCounter++
	return fmt.Sprintf("multistream_%d", id)
}

// generateSourceID generates a unique source ID
func generateSourceID() string {
	id := time.Now().UnixNano() + sourceIDCounter
	sourceIDCounter++
	return fmt.Sprintf("source_%d", id)
}
