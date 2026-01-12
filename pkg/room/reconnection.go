// Package room provides automatic reconnection handling for participants
package room

import (
	"context"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/logger"
)

// ReconnectionState represents the state of a reconnection attempt
type ReconnectionState int

const (
	// ReconnectionStateNone means no reconnection in progress
	ReconnectionStateNone ReconnectionState = iota

	// ReconnectionStateReconnecting means reconnection is in progress
	ReconnectionStateReconnecting

	// ReconnectionStateReconnected means reconnection succeeded
	ReconnectionStateReconnected

	// ReconnectionStateFailed means reconnection failed
	ReconnectionStateFailed
)

// ReconnectionAttempt represents a single reconnection attempt
type ReconnectionAttempt struct {
	// AttemptNumber is the attempt number (1-indexed)
	AttemptNumber int

	// Timestamp is when the attempt was made
	Timestamp time.Time

	// Success indicates if the attempt succeeded
	Success bool

	// Error is the error if the attempt failed
	Error error
}

// ParticipantReconnection tracks reconnection state for a participant
type ParticipantReconnection struct {
	// ParticipantID is the participant identifier
	ParticipantID string

	// State is the current reconnection state
	State ReconnectionState

	// Attempts is the number of reconnection attempts made
	Attempts int

	// StartTime is when reconnection started
	StartTime time.Time

	// LastAttempt is when the last attempt was made
	LastAttempt time.Time

	// History is the history of reconnection attempts
	History []ReconnectionAttempt

	// mu protects concurrent access
	mu sync.RWMutex
}

// ReconnectionHandler handles automatic reconnection for disconnected participants
type ReconnectionHandler struct {
	// config is the reconnection configuration
	config ReconnectionConfig

	// logger for logging
	logger logger.Logger

	// mu protects concurrent access
	mu sync.RWMutex

	// reconnections maps participant ID to their reconnection state
	reconnections map[string]*ParticipantReconnection

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// callbacks
	onReconnected        func(participantID string)
	onReconnectionFailed func(participantID string, err error)
}

// ReconnectionConfig contains configuration for reconnection handling
type ReconnectionConfig struct {
	// MaxAttempts is the maximum number of reconnection attempts
	MaxAttempts int

	// InitialDelay is the initial delay before the first reconnection attempt
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between attempts
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64

	// Timeout is the total timeout for all reconnection attempts
	Timeout time.Duration
}

// DefaultReconnectionConfig returns the default configuration
func DefaultReconnectionConfig() ReconnectionConfig {
	return ReconnectionConfig{
		MaxAttempts:       5,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Timeout:           2 * time.Minute,
	}
}

// NewReconnectionHandler creates a new reconnection handler
func NewReconnectionHandler(config ReconnectionConfig, log logger.Logger) *ReconnectionHandler {
	ctx, cancel := context.WithCancel(context.Background())

	return &ReconnectionHandler{
		config:        config,
		logger:        log,
		reconnections: make(map[string]*ParticipantReconnection),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetCallbacks sets the reconnection callbacks
func (rh *ReconnectionHandler) SetCallbacks(
	onReconnected func(participantID string),
	onReconnectionFailed func(participantID string, err error),
) {
	rh.onReconnected = onReconnected
	rh.onReconnectionFailed = onReconnectionFailed
}

// HandleDisconnect handles when a participant disconnects
func (rh *ReconnectionHandler) HandleDisconnect(participantID string) {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	_, exists := rh.reconnections[participantID]
	if !exists {
		rh.reconnections[participantID] = &ParticipantReconnection{
			ParticipantID: participantID,
			State:         ReconnectionStateReconnecting,
			Attempts:      0,
			StartTime:     time.Now(),
			History:       make([]ReconnectionAttempt, 0),
		}
	}

	rh.logger.Info("Participant disconnected, starting reconnection",
		logger.String("participant_id", participantID),
	)

	// Start reconnection process in background
	go rh.reconnectParticipant(participantID)
}

// reconnectParticipant attempts to reconnect a participant
func (rh *ReconnectionHandler) reconnectParticipant(participantID string) {
	rh.mu.RLock()
	reconnection, exists := rh.reconnections[participantID]
	if !exists {
		rh.mu.RUnlock()
		return
	}
	rh.mu.RUnlock()

	// Create timeout context
	ctx, cancel := context.WithTimeout(rh.ctx, rh.config.Timeout)
	defer cancel()

	delay := rh.config.InitialDelay

	for attempt := 1; attempt <= rh.config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			// Timeout or cancelled
			rh.handleReconnectionFailure(participantID, errors.New(errors.ErrCodeReconnectionTimeout, "reconnection timed out"))
			return

		case <-time.After(delay):
			// Attempt reconnection
			success, err := rh.attemptReconnection(participantID, attempt)

			if success {
				rh.handleReconnectionSuccess(participantID)
				return
			}

			// Record failed attempt
			reconnection.mu.Lock()
			reconnection.Attempts = attempt
			reconnection.LastAttempt = time.Now()
			reconnection.History = append(reconnection.History, ReconnectionAttempt{
				AttemptNumber: attempt,
				Timestamp:     time.Now(),
				Success:       false,
				Error:         err,
			})
			reconnection.mu.Unlock()

			rh.logger.Warn("Reconnection attempt failed",
				logger.String("participant_id", participantID),
				logger.Field{Key: "attempt", Value: attempt},
				logger.Field{Key: "max_attempts", Value: rh.config.MaxAttempts},
				logger.Err(err),
			)

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * rh.config.BackoffMultiplier)
			if delay > rh.config.MaxDelay {
				delay = rh.config.MaxDelay
			}
		}
	}

	// All attempts failed
	rh.handleReconnectionFailure(participantID, errors.New(errors.ErrCodeMaxAttemptsExceeded, "maximum reconnection attempts exceeded"))
}

// attemptReconnection attempts a single reconnection
func (rh *ReconnectionHandler) attemptReconnection(participantID string, attemptNumber int) (bool, error) {
	rh.mu.RLock()
	_, exists := rh.reconnections[participantID]
	rh.mu.RUnlock()

	if !exists {
		return false, errors.NewNotFoundError(participantID)
	}

	// In a real implementation, this would:
	// 1. Ping the participant
	// 2. Re-establish WebRTC connection
	// 3. Restore media tracks
	// 4. Sync room state

	// For now, just log the attempt
	rh.logger.Debug("Attempting reconnection",
		logger.String("participant_id", participantID),
		logger.Field{Key: "attempt", Value: attemptNumber},
	)

	// Placeholder - always fails for now
	return false, errors.New(errors.ErrCodeNetworkError, "connection failed")
}

// HandleReconnect handles when a participant successfully reconnects
func (rh *ReconnectionHandler) HandleReconnect(participantID string) error {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	reconnection, exists := rh.reconnections[participantID]
	if !exists {
		return errors.NewParticipantNotFoundError(participantID)
	}

	reconnection.mu.Lock()
	if reconnection.State != ReconnectionStateReconnecting {
		reconnection.mu.Unlock()
		return errors.New(errors.ErrCodeInvalidState, "participant is not in reconnecting state")
	}

	reconnection.State = ReconnectionStateReconnected
	reconnection.mu.Unlock()

	rh.logger.Info("Participant reconnected",
		logger.String("participant_id", participantID),
		logger.Field{Key: "attempts", Value: reconnection.Attempts},
	)

	if rh.onReconnected != nil {
		rh.onReconnected(participantID)
	}

	// Clean up after a delay
	go func() {
		time.Sleep(5 * time.Second)
		rh.mu.Lock()
		delete(rh.reconnections, participantID)
		rh.mu.Unlock()
	}()

	return nil
}

// handleReconnectionSuccess handles successful reconnection
func (rh *ReconnectionHandler) handleReconnectionSuccess(participantID string) {
	rh.mu.Lock()
	reconnection, exists := rh.reconnections[participantID]
	if exists {
		reconnection.mu.Lock()
		reconnection.State = ReconnectionStateReconnected
		reconnection.mu.Unlock()
	}
	rh.mu.Unlock()

	rh.logger.Info("Participant reconnected successfully",
		logger.String("participant_id", participantID),
	)

	if rh.onReconnected != nil {
		rh.onReconnected(participantID)
	}

	// Clean up after a delay
	go func() {
		time.Sleep(5 * time.Second)
		rh.mu.Lock()
		delete(rh.reconnections, participantID)
		rh.mu.Unlock()
	}()
}

// handleReconnectionFailure handles failed reconnection
func (rh *ReconnectionHandler) handleReconnectionFailure(participantID string, err error) {
	rh.mu.Lock()
	reconnection, exists := rh.reconnections[participantID]
	if exists {
		reconnection.mu.Lock()
		reconnection.State = ReconnectionStateFailed
		reconnection.mu.Unlock()
	}
	rh.mu.Unlock()

	rh.logger.Error("Participant reconnection failed",
		logger.String("participant_id", participantID),
		logger.Err(err),
	)

	if rh.onReconnectionFailed != nil {
		rh.onReconnectionFailed(participantID, err)
	}

	// Clean up after a delay
	go func() {
		time.Sleep(5 * time.Second)
		rh.mu.Lock()
		delete(rh.reconnections, participantID)
		rh.mu.Unlock()
	}()
}

// GetReconnectionState returns the reconnection state for a participant
func (rh *ReconnectionHandler) GetReconnectionState(participantID string) (*ParticipantReconnection, error) {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	reconnection, exists := rh.reconnections[participantID]
	if !exists {
		return nil, errors.NewParticipantNotFoundError(participantID)
	}

	return reconnection, nil
}

// GetActiveReconnections returns all active reconnections
func (rh *ReconnectionHandler) GetActiveReconnections() []*ParticipantReconnection {
	rh.mu.RLock()
	defer rh.mu.RUnlock()

	reconnections := make([]*ParticipantReconnection, 0, len(rh.reconnections))
	for _, r := range rh.reconnections {
		if r.State == ReconnectionStateReconnecting {
			reconnections = append(reconnections, r)
		}
	}

	return reconnections
}

// Close closes the reconnection handler
func (rh *ReconnectionHandler) Close() error {
	rh.cancel()

	rh.logger.Info("Reconnection handler closed")
	return nil
}
