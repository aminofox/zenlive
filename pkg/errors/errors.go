package errors

import (
	"fmt"
)

// ErrorCode represents a unique error code
type ErrorCode int

const (
	// ErrCodeUnknown represents an unknown error
	ErrCodeUnknown ErrorCode = 1000

	// Authentication errors (2000-2999)
	ErrCodeAuthenticationFailed ErrorCode = 2000
	ErrCodeInvalidToken         ErrorCode = 2001
	ErrCodeTokenExpired         ErrorCode = 2002
	ErrCodeUnauthorized         ErrorCode = 2003
	ErrCodeInvalidCredentials   ErrorCode = 2004

	// Stream errors (3000-3999)
	ErrCodeStreamNotFound    ErrorCode = 3000
	ErrCodeStreamAlreadyLive ErrorCode = 3001
	ErrCodeStreamNotLive     ErrorCode = 3002
	ErrCodeInvalidStreamKey  ErrorCode = 3003
	ErrCodeStreamCreateError ErrorCode = 3004
	ErrCodeStreamDeleteError ErrorCode = 3005
	ErrCodeNotFound          ErrorCode = 3006

	// Storage errors (4000-4999)
	ErrCodeStorageError      ErrorCode = 4000
	ErrCodeFileNotFound      ErrorCode = 4001
	ErrCodeUploadFailed      ErrorCode = 4002
	ErrCodeDownloadFailed    ErrorCode = 4003
	ErrCodeInsufficientSpace ErrorCode = 4004

	// Protocol errors (5000-5999)
	ErrCodeRTMPError       ErrorCode = 5000
	ErrCodeHLSError        ErrorCode = 5001
	ErrCodeWebRTCError     ErrorCode = 5002
	ErrCodeProtocolError   ErrorCode = 5003
	ErrCodeHandshakeFailed ErrorCode = 5004

	// Chat errors (6000-6999)
	ErrCodeChatRoomNotFound  ErrorCode = 6000
	ErrCodeMessageTooLong    ErrorCode = 6001
	ErrCodeRateLimitExceeded ErrorCode = 6002
	ErrCodeUserBanned        ErrorCode = 6003
	ErrCodeUserMuted         ErrorCode = 6004

	// Configuration errors (7000-7999)
	ErrCodeInvalidConfig ErrorCode = 7000
	ErrCodeMissingConfig ErrorCode = 7001

	// Network errors (8000-8999)
	ErrCodeNetworkError     ErrorCode = 8000
	ErrCodeConnectionFailed ErrorCode = 8001
	ErrCodeTimeout          ErrorCode = 8002
	ErrCodeDisconnected     ErrorCode = 8003

	// Validation errors (9000-9999)
	ErrCodeValidationFailed ErrorCode = 9000
	ErrCodeInvalidInput     ErrorCode = 9001
	ErrCodeMissingParameter ErrorCode = 9002

	// Room errors (10000-10999)
	ErrCodeRoomNotFound        ErrorCode = 10000
	ErrCodeParticipantNotFound ErrorCode = 10001
	ErrCodeRoomLimitExceeded   ErrorCode = 10002
	ErrCodeTrackLimitExceeded  ErrorCode = 10003
	ErrCodeSessionNotFound     ErrorCode = 10004
	ErrCodeInvalidState        ErrorCode = 10005
	ErrCodeReconnectionTimeout ErrorCode = 10006
	ErrCodeMaxAttemptsExceeded ErrorCode = 10007
)

// Error represents a custom error with code and message
type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause of the error
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given code and message
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with a code and message
func Wrap(code ErrorCode, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// IsErrorCode checks if the error has the given error code
func IsErrorCode(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}

	if e, ok := err.(*Error); ok {
		return e.Code == code
	}

	return false
}

// GetErrorCode returns the error code from an error, or ErrCodeUnknown if not found
func GetErrorCode(err error) ErrorCode {
	if err == nil {
		return ErrCodeUnknown
	}

	if e, ok := err.(*Error); ok {
		return e.Code
	}

	return ErrCodeUnknown
}

// Common error constructors for convenience

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(message string) *Error {
	return New(ErrCodeAuthenticationFailed, message)
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string) *Error {
	return New(ErrCodeNotFound, message)
}

// NewStreamNotFoundError creates a new stream not found error
func NewStreamNotFoundError(streamID string) *Error {
	return New(ErrCodeStreamNotFound, fmt.Sprintf("stream not found: %s", streamID))
}

// NewInvalidTokenError creates a new invalid token error
func NewInvalidTokenError() *Error {
	return New(ErrCodeInvalidToken, "invalid or malformed token")
}

// NewTokenExpiredError creates a new token expired error
func NewTokenExpiredError() *Error {
	return New(ErrCodeTokenExpired, "token has expired")
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string) *Error {
	return New(ErrCodeUnauthorized, message)
}

// NewValidationError creates a new validation error
func NewValidationError(message string) *Error {
	return New(ErrCodeValidationFailed, message)
}

// NewNetworkError creates a new network error
func NewNetworkError(message string, cause error) *Error {
	return Wrap(ErrCodeNetworkError, message, cause)
}

// NewRoomNotFoundError creates a new room not found error
func NewRoomNotFoundError(roomID string) *Error {
	return New(ErrCodeRoomNotFound, fmt.Sprintf("room not found: %s", roomID))
}

// NewParticipantNotFoundError creates a new participant not found error
func NewParticipantNotFoundError(participantID string) *Error {
	return New(ErrCodeParticipantNotFound, fmt.Sprintf("participant not found: %s", participantID))
}

// NewSessionNotFoundError creates a new session not found error
func NewSessionNotFoundError(userID string) *Error {
	return New(ErrCodeSessionNotFound, fmt.Sprintf("session not found: %s", userID))
}

// NewRoomLimitExceededError creates a room limit exceeded error
func NewRoomLimitExceededError(limit int) *Error {
	return New(ErrCodeRoomLimitExceeded, fmt.Sprintf("room limit exceeded: max %d rooms", limit))
}

// NewTrackLimitExceededError creates a track limit exceeded error
func NewTrackLimitExceededError(limit int) *Error {
	return New(ErrCodeTrackLimitExceeded, fmt.Sprintf("track limit exceeded: max %d tracks", limit))
}
