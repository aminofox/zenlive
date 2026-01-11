package logger

import (
	"io"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DebugLevel is the most verbose level
	DebugLevel LogLevel = iota

	// InfoLevel is for informational messages
	InfoLevel

	// WarnLevel is for warnings
	WarnLevel

	// ErrorLevel is for errors
	ErrorLevel

	// FatalLevel is for fatal errors that cause the application to exit
	FatalLevel
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// Logger is the interface that all loggers must implement
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, fields ...Field)

	// Info logs an info message
	Info(msg string, fields ...Field)

	// Warn logs a warning message
	Warn(msg string, fields ...Field)

	// Error logs an error message
	Error(msg string, fields ...Field)

	// Fatal logs a fatal message and exits
	Fatal(msg string, fields ...Field)

	// With creates a child logger with additional fields
	With(fields ...Field) Logger

	// SetLevel sets the minimum log level
	SetLevel(level LogLevel)

	// SetOutput sets the output writer
	SetOutput(w io.Writer)
}

// NewField creates a new logging field
func NewField(key string, value interface{}) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Common field constructors for convenience

// String creates a string field
func String(key, value string) Field {
	return NewField(key, value)
}

// Int creates an int field
func Int(key string, value int) Field {
	return NewField(key, value)
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return NewField(key, value)
}

// Bool creates a bool field
func Bool(key string, value bool) Field {
	return NewField(key, value)
}

// Err creates an error field
func Err(err error) Field {
	return NewField("error", err)
}

// Duration creates a duration field
func Duration(key string, value interface{}) Field {
	return NewField(key, value)
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return NewField(key, value)
}

// ParseLevel parses a log level string
func ParseLevel(levelStr string) LogLevel {
	switch levelStr {
	case "debug", "DEBUG":
		return DebugLevel
	case "info", "INFO":
		return InfoLevel
	case "warn", "WARN", "warning", "WARNING":
		return WarnLevel
	case "error", "ERROR":
		return ErrorLevel
	case "fatal", "FATAL":
		return FatalLevel
	default:
		return InfoLevel
	}
}
