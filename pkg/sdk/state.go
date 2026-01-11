package sdk

import (
	"fmt"
	"sync"
)

// StreamState represents the current state of a stream
type StreamState string

const (
	// StateIdle indicates stream is created but not started
	StateIdle StreamState = "idle"

	// StateLive indicates stream is currently broadcasting
	StateLive StreamState = "live"

	// StatePaused indicates stream is temporarily paused
	StatePaused StreamState = "paused"

	// StateEnded indicates stream has finished
	StateEnded StreamState = "ended"

	// StateError indicates stream encountered an error
	StateError StreamState = "error"
)

// StreamStateTransition represents a valid state transition
type StreamStateTransition struct {
	From StreamState
	To   StreamState
}

// validTransitions defines all valid state transitions
var validTransitions = map[StreamStateTransition]bool{
	// From Idle
	{StateIdle, StateLive}:  true,
	{StateIdle, StateError}: true,
	{StateIdle, StateEnded}: true, // Can delete without starting

	// From Live
	{StateLive, StatePaused}: true,
	{StateLive, StateEnded}:  true,
	{StateLive, StateError}:  true,

	// From Paused
	{StatePaused, StateLive}:  true,
	{StatePaused, StateEnded}: true,
	{StatePaused, StateError}: true,

	// From Error
	{StateError, StateIdle}:  true, // Can reset
	{StateError, StateEnded}: true, // Can end gracefully

	// Self-transitions (idempotent operations)
	{StateIdle, StateIdle}:     true,
	{StateLive, StateLive}:     true,
	{StatePaused, StatePaused}: true,
	{StateEnded, StateEnded}:   true,
	{StateError, StateError}:   true,
}

// StreamStateMachine manages stream state transitions
type StreamStateMachine struct {
	mu            sync.RWMutex
	currentState  StreamState
	previousState StreamState
	errorMessage  string
}

// NewStreamStateMachine creates a new state machine starting in Idle state
func NewStreamStateMachine() *StreamStateMachine {
	return &StreamStateMachine{
		currentState:  StateIdle,
		previousState: StateIdle,
	}
}

// GetState returns the current state (thread-safe)
func (sm *StreamStateMachine) GetState() StreamState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// GetPreviousState returns the previous state (thread-safe)
func (sm *StreamStateMachine) GetPreviousState() StreamState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.previousState
}

// GetErrorMessage returns the error message if in error state
func (sm *StreamStateMachine) GetErrorMessage() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.errorMessage
}

// CanTransitionTo checks if transition to target state is valid
func (sm *StreamStateMachine) CanTransitionTo(to StreamState) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	transition := StreamStateTransition{
		From: sm.currentState,
		To:   to,
	}

	return validTransitions[transition]
}

// TransitionTo attempts to transition to the target state
func (sm *StreamStateMachine) TransitionTo(to StreamState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	transition := StreamStateTransition{
		From: sm.currentState,
		To:   to,
	}

	if !validTransitions[transition] {
		return fmt.Errorf("invalid state transition from %s to %s", sm.currentState, to)
	}

	sm.previousState = sm.currentState
	sm.currentState = to

	// Clear error message when leaving error state
	if to != StateError {
		sm.errorMessage = ""
	}

	return nil
}

// TransitionToError transitions to error state with a message
func (sm *StreamStateMachine) TransitionToError(errorMsg string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	transition := StreamStateTransition{
		From: sm.currentState,
		To:   StateError,
	}

	if !validTransitions[transition] {
		return fmt.Errorf("invalid state transition from %s to %s", sm.currentState, StateError)
	}

	sm.previousState = sm.currentState
	sm.currentState = StateError
	sm.errorMessage = errorMsg

	return nil
}

// Reset resets the state machine to Idle state
func (sm *StreamStateMachine) Reset() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.previousState = sm.currentState
	sm.currentState = StateIdle
	sm.errorMessage = ""

	return nil
}

// IsLive returns true if stream is currently live
func (sm *StreamStateMachine) IsLive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == StateLive
}

// IsPaused returns true if stream is paused
func (sm *StreamStateMachine) IsPaused() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == StatePaused
}

// IsEnded returns true if stream has ended
func (sm *StreamStateMachine) IsEnded() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == StateEnded
}

// IsError returns true if stream is in error state
func (sm *StreamStateMachine) IsError() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == StateError
}

// IsIdle returns true if stream is idle
func (sm *StreamStateMachine) IsIdle() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == StateIdle
}

// String returns string representation of current state
func (sm *StreamStateMachine) String() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentState == StateError && sm.errorMessage != "" {
		return fmt.Sprintf("%s (%s)", sm.currentState, sm.errorMessage)
	}

	return string(sm.currentState)
}
