package zenlive

import (
	"fmt"
	"sync"

	"github.com/aminofox/zenlive/pkg/config"
	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
	"github.com/aminofox/zenlive/pkg/types"
)

// SDK is the main ZenLive SDK instance
type SDK struct {
	config *config.Config
	logger logger.Logger

	// Streaming providers
	providers map[types.StreamProtocol]types.StreamProvider

	// Room manager for video conferencing
	roomManager *room.RoomManager

	// Internal state
	mu        sync.RWMutex
	isRunning bool
}

// New creates a new ZenLive SDK instance
func New(cfg *config.Config) (*SDK, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Create logger
	logLevel := logger.ParseLevel(cfg.Logging.Level)
	log := logger.NewDefaultLogger(logLevel, cfg.Logging.Format)

	// Create room manager
	roomMgr := room.NewRoomManager(log)

	sdk := &SDK{
		config:      cfg,
		logger:      log,
		providers:   make(map[types.StreamProtocol]types.StreamProvider),
		roomManager: roomMgr,
		isRunning:   false,
	}

	return sdk, nil
}

// Start starts the SDK and all enabled streaming providers
func (s *SDK) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return errors.New(errors.ErrCodeUnknown, "SDK is already running")
	}

	s.logger.Info("Starting ZenLive SDK",
		logger.String("version", "1.0.0"),
	)

	// Start all registered providers
	for protocol, provider := range s.providers {
		s.logger.Info("Starting streaming provider",
			logger.String("protocol", string(protocol)),
		)

		if err := provider.Start(); err != nil {
			return errors.Wrap(
				errors.ErrCodeProtocolError,
				fmt.Sprintf("failed to start %s provider", protocol),
				err,
			)
		}
	}

	s.isRunning = true
	s.logger.Info("ZenLive SDK started successfully")

	return nil
}

// Stop stops the SDK and all streaming providers
func (s *SDK) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return errors.New(errors.ErrCodeUnknown, "SDK is not running")
	}

	s.logger.Info("Stopping ZenLive SDK")

	// Stop all providers
	for protocol, provider := range s.providers {
		s.logger.Info("Stopping streaming provider",
			logger.String("protocol", string(protocol)),
		)

		if err := provider.Stop(); err != nil {
			s.logger.Error("Failed to stop provider",
				logger.String("protocol", string(protocol)),
				logger.Err(err),
			)
		}
	}

	// Shutdown room manager
	if s.roomManager != nil {
		s.roomManager.Shutdown()
	}

	s.isRunning = false
	s.logger.Info("ZenLive SDK stopped successfully")

	return nil
}

// IsRunning returns true if the SDK is currently running
func (s *SDK) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// RegisterProvider registers a streaming protocol provider
func (s *SDK) RegisterProvider(protocol types.StreamProtocol, provider types.StreamProvider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return errors.New(errors.ErrCodeUnknown, "cannot register provider while SDK is running")
	}

	if provider == nil {
		return errors.New(errors.ErrCodeInvalidInput, "provider cannot be nil")
	}

	s.providers[protocol] = provider
	s.logger.Info("Registered streaming provider",
		logger.String("protocol", string(protocol)),
	)

	return nil
}

// GetProvider returns a registered streaming provider
func (s *SDK) GetProvider(protocol types.StreamProtocol) (types.StreamProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider, exists := s.providers[protocol]
	if !exists {
		return nil, errors.New(
			errors.ErrCodeProtocolError,
			fmt.Sprintf("provider for protocol %s not found", protocol),
		)
	}

	return provider, nil
}

// Config returns the SDK configuration
func (s *SDK) Config() *config.Config {
	return s.config
}

// Logger returns the SDK logger
func (s *SDK) Logger() logger.Logger {
	return s.logger
}

// SetLogger sets a custom logger
func (s *SDK) SetLogger(log logger.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = log
}

// GetRoomManager returns the room manager instance
func (s *SDK) GetRoomManager() *room.RoomManager {
	return s.roomManager
}

// Room Management Methods

// CreateRoom creates a new room with the given name and options
func (s *SDK) CreateRoom(name string, opts *room.CreateRoomRequest) (*room.Room, error) {
	if opts == nil {
		opts = &room.CreateRoomRequest{
			Name: name,
		}
	} else {
		opts.Name = name
	}

	return s.roomManager.CreateRoom(opts, "system")
}

// DeleteRoom deletes a room by ID
func (s *SDK) DeleteRoom(roomID string) error {
	return s.roomManager.DeleteRoom(roomID)
}

// GetRoom retrieves a room by ID
func (s *SDK) GetRoom(roomID string) (*room.Room, error) {
	return s.roomManager.GetRoom(roomID)
}

// ListRooms returns all active rooms
func (s *SDK) ListRooms() []*room.Room {
	return s.roomManager.ListRooms()
}

// Event Callbacks

// OnRoomCreated registers a callback for room creation events
func (s *SDK) OnRoomCreated(callback func(*room.Room)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventRoomCreated, func(event *room.RoomEvent) {
			if rm, ok := event.Data.(*room.Room); ok {
				callback(rm)
			}
		})
	}
}

// OnRoomDeleted registers a callback for room deletion events
func (s *SDK) OnRoomDeleted(callback func(roomID string)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventRoomDeleted, func(event *room.RoomEvent) {
			callback(event.RoomID)
		})
	}
}

// OnParticipantJoined registers a callback for participant join events
func (s *SDK) OnParticipantJoined(callback func(roomID string, participant *room.Participant)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventParticipantJoined, func(event *room.RoomEvent) {
			if p, ok := event.Data.(*room.Participant); ok {
				callback(event.RoomID, p)
			}
		})
	}
}

// OnParticipantLeft registers a callback for participant leave events
func (s *SDK) OnParticipantLeft(callback func(roomID string, participantID string)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventParticipantLeft, func(event *room.RoomEvent) {
			if p, ok := event.Data.(*room.Participant); ok {
				callback(event.RoomID, p.ID)
			}
		})
	}
}

// OnTrackPublished registers a callback for track publication events
func (s *SDK) OnTrackPublished(callback func(roomID string, track *room.MediaTrack)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventTrackPublished, func(event *room.RoomEvent) {
			if data, ok := event.Data.(map[string]interface{}); ok {
				if track, ok := data["track"].(*room.MediaTrack); ok {
					callback(event.RoomID, track)
				}
			}
		})
	}
}

// OnTrackUnpublished registers a callback for track unpublication events
func (s *SDK) OnTrackUnpublished(callback func(roomID string, trackID string)) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventTrackUnpublished, func(event *room.RoomEvent) {
			if data, ok := event.Data.(map[string]interface{}); ok {
				if trackID, ok := data["track_id"].(string); ok {
					callback(event.RoomID, trackID)
				}
			}
		})
	}
}

// OnMetadataUpdated registers a callback for metadata update events
func (s *SDK) OnMetadataUpdated(callback func(roomID string, metadata map[string]interface{})) {
	if s.roomManager != nil && s.roomManager.GetEventBus() != nil {
		s.roomManager.GetEventBus().Subscribe(room.EventMetadataUpdated, func(event *room.RoomEvent) {
			if metadata, ok := event.Data.(map[string]interface{}); ok {
				callback(event.RoomID, metadata)
			}
		})
	}
}

// Version returns the SDK version
func (s *SDK) Version() string {
	return "1.0.0"
}
