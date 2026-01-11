package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
)

// StreamController manages stream control operations
type StreamController struct {
	manager *StreamManager
	events  *EventBus
	logger  logger.Logger
}

// NewStreamController creates a new stream controller
func NewStreamController(manager *StreamManager, events *EventBus, log logger.Logger) *StreamController {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &StreamController{
		manager: manager,
		events:  events,
		logger:  log,
	}
}

// StartStream starts a stream
func (sc *StreamController) StartStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Check if transition is valid
	if !stream.stateMachine.CanTransitionTo(StateLive) {
		return fmt.Errorf("cannot start stream in current state: %s", stream.State)
	}

	// Transition to live state
	if err := stream.stateMachine.TransitionTo(StateLive); err != nil {
		return err
	}

	// Update stream
	stream.mu.Lock()
	stream.State = StateLive
	now := time.Now()
	stream.StartedAt = &now
	stream.UpdatedAt = now
	stream.mu.Unlock()

	sc.logger.Info("Stream started",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "user_id", Value: stream.UserID},
	)

	// Emit event
	if sc.events != nil {
		sc.events.Publish(&StreamEvent{
			Type:      EventStreamStart,
			StreamID:  streamID,
			UserID:    stream.UserID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"stream_key": stream.StreamKey,
				"protocol":   stream.Protocol,
			},
		})
	}

	return nil
}

// StopStream stops a stream gracefully
func (sc *StreamController) StopStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Check if transition is valid
	if !stream.stateMachine.CanTransitionTo(StateEnded) {
		return fmt.Errorf("cannot stop stream in current state: %s", stream.State)
	}

	// Calculate duration before stopping
	var duration time.Duration
	if stream.StartedAt != nil {
		duration = time.Since(*stream.StartedAt)
	}

	// Transition to ended state
	if err := stream.stateMachine.TransitionTo(StateEnded); err != nil {
		return err
	}

	// Update stream
	stream.mu.Lock()
	stream.State = StateEnded
	now := time.Now()
	stream.EndedAt = &now
	stream.UpdatedAt = now
	stream.TotalDuration = duration
	stream.mu.Unlock()

	sc.logger.Info("Stream stopped",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "duration", Value: duration},
	)

	// Emit event
	if sc.events != nil {
		sc.events.Publish(&StreamEvent{
			Type:      EventStreamEnd,
			StreamID:  streamID,
			UserID:    stream.UserID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"duration":     duration.Seconds(),
				"viewer_count": stream.ViewerCount,
			},
		})
	}

	return nil
}

// EmergencyStop stops a stream immediately without graceful shutdown
func (sc *StreamController) EmergencyStop(ctx context.Context, streamID string, reason string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	sc.logger.Warn("Emergency stop initiated",
		logger.Field{Key: "stream_id", Value: streamID},
		logger.Field{Key: "reason", Value: reason},
	)

	// Calculate duration
	var duration time.Duration
	if stream.StartedAt != nil {
		duration = time.Since(*stream.StartedAt)
	}

	// Force transition to ended state
	stream.mu.Lock()
	stream.State = StateEnded
	now := time.Now()
	stream.EndedAt = &now
	stream.UpdatedAt = now
	stream.TotalDuration = duration
	stream.mu.Unlock()

	// Update state machine
	stream.stateMachine.TransitionTo(StateEnded)

	// Emit event
	if sc.events != nil {
		sc.events.Publish(&StreamEvent{
			Type:      EventStreamError,
			StreamID:  streamID,
			UserID:    stream.UserID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"reason":       reason,
				"emergency":    true,
				"duration":     duration.Seconds(),
				"viewer_count": stream.ViewerCount,
			},
		})
	}

	return nil
}

// PauseStream pauses a live stream
func (sc *StreamController) PauseStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Check if transition is valid
	if !stream.stateMachine.CanTransitionTo(StatePaused) {
		return fmt.Errorf("cannot pause stream in current state: %s", stream.State)
	}

	// Transition to paused state
	if err := stream.stateMachine.TransitionTo(StatePaused); err != nil {
		return err
	}

	// Update stream
	stream.mu.Lock()
	stream.State = StatePaused
	stream.UpdatedAt = time.Now()
	stream.mu.Unlock()

	sc.logger.Info("Stream paused",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	// Emit event
	if sc.events != nil {
		sc.events.Publish(&StreamEvent{
			Type:      EventStreamPause,
			StreamID:  streamID,
			UserID:    stream.UserID,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// ResumeStream resumes a paused stream
func (sc *StreamController) ResumeStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Check if stream is paused
	if !stream.stateMachine.IsPaused() {
		return fmt.Errorf("stream is not paused, current state: %s", stream.State)
	}

	// Transition back to live state
	if err := stream.stateMachine.TransitionTo(StateLive); err != nil {
		return err
	}

	// Update stream
	stream.mu.Lock()
	stream.State = StateLive
	stream.UpdatedAt = time.Now()
	stream.mu.Unlock()

	sc.logger.Info("Stream resumed",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	// Emit event
	if sc.events != nil {
		sc.events.Publish(&StreamEvent{
			Type:      EventStreamResume,
			StreamID:  streamID,
			UserID:    stream.UserID,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// RestartStream restarts a stream (stop and start)
func (sc *StreamController) RestartStream(ctx context.Context, streamID string) error {
	if streamID == "" {
		return fmt.Errorf("stream ID is required")
	}

	sc.logger.Info("Restarting stream",
		logger.Field{Key: "stream_id", Value: streamID},
	)

	// Stop the stream
	if err := sc.StopStream(ctx, streamID); err != nil {
		// If already stopped, that's okay
		if err.Error() != fmt.Sprintf("cannot stop stream in current state: %s", StateEnded) {
			return fmt.Errorf("failed to stop stream: %w", err)
		}
	}

	// Get the stream
	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return err
	}

	// Reset state machine to idle
	if err := stream.stateMachine.Reset(); err != nil {
		return err
	}

	// Update stream state
	stream.mu.Lock()
	stream.State = StateIdle
	stream.StartedAt = nil
	stream.EndedAt = nil
	stream.ViewerCount = 0
	stream.UpdatedAt = time.Now()
	stream.mu.Unlock()

	// Start the stream
	if err := sc.StartStream(ctx, streamID); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	return nil
}

// GetStreamStatus returns the current status of a stream
func (sc *StreamController) GetStreamStatus(ctx context.Context, streamID string) (*StreamStatus, error) {
	if streamID == "" {
		return nil, fmt.Errorf("stream ID is required")
	}

	stream, err := sc.manager.GetStream(ctx, streamID)
	if err != nil {
		return nil, err
	}

	stream.mu.RLock()
	defer stream.mu.RUnlock()

	status := &StreamStatus{
		StreamID:    stream.ID,
		State:       stream.State,
		ViewerCount: stream.ViewerCount,
		Duration:    stream.GetDuration(),
		StartedAt:   stream.StartedAt,
		IsLive:      stream.stateMachine.IsLive(),
		IsPaused:    stream.stateMachine.IsPaused(),
		IsEnded:     stream.stateMachine.IsEnded(),
	}

	return status, nil
}

// StreamStatus represents the current status of a stream
type StreamStatus struct {
	StreamID    string        `json:"stream_id"`
	State       StreamState   `json:"state"`
	ViewerCount int64         `json:"viewer_count"`
	Duration    time.Duration `json:"duration"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	IsLive      bool          `json:"is_live"`
	IsPaused    bool          `json:"is_paused"`
	IsEnded     bool          `json:"is_ended"`
}

// StopAllStreams stops all active streams gracefully
func (sc *StreamController) StopAllStreams(ctx context.Context) error {
	streams, err := sc.manager.ListStreams(ctx)
	if err != nil {
		return err
	}

	count := 0
	for _, stream := range streams {
		if stream.stateMachine.IsLive() || stream.stateMachine.IsPaused() {
			if err := sc.StopStream(ctx, stream.ID); err != nil {
				sc.logger.Error("Failed to stop stream",
					logger.Field{Key: "stream_id", Value: stream.ID},
					logger.Field{Key: "error", Value: err},
				)
			} else {
				count++
			}
		}
	}

	sc.logger.Info("Stopped all streams",
		logger.Field{Key: "count", Value: count},
	)

	return nil
}

// GetActiveStre returns all active (live or paused) streams
func (sc *StreamController) GetActiveStreams(ctx context.Context) ([]*Stream, error) {
	streams, err := sc.manager.ListStreams(ctx)
	if err != nil {
		return nil, err
	}

	activeStreams := make([]*Stream, 0)
	for _, stream := range streams {
		if stream.stateMachine.IsLive() || stream.stateMachine.IsPaused() {
			activeStreams = append(activeStreams, stream)
		}
	}

	return activeStreams, nil
}
