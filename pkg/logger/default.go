package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// DefaultLogger is a simple implementation of the Logger interface
type DefaultLogger struct {
	mu     sync.Mutex
	level  LogLevel
	output io.Writer
	fields []Field
	format string // "json" or "text"
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(level LogLevel, format string) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		output: os.Stdout,
		fields: make([]Field, 0),
		format: format,
	}
}

// Debug logs a debug message
func (l *DefaultLogger) Debug(msg string, fields ...Field) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, fields...)
	}
}

// Info logs an info message
func (l *DefaultLogger) Info(msg string, fields ...Field) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, fields...)
	}
}

// Warn logs a warning message
func (l *DefaultLogger) Warn(msg string, fields ...Field) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, fields...)
	}
}

// Error logs an error message
func (l *DefaultLogger) Error(msg string, fields ...Field) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, fields...)
	}
}

// Fatal logs a fatal message and exits
func (l *DefaultLogger) Fatal(msg string, fields ...Field) {
	l.log(FatalLevel, msg, fields...)
	os.Exit(1)
}

// With creates a child logger with additional fields
func (l *DefaultLogger) With(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &DefaultLogger{
		level:  l.level,
		output: l.output,
		fields: newFields,
		format: l.format,
	}
}

// SetLevel sets the minimum log level
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer
func (l *DefaultLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// log is the internal logging function
func (l *DefaultLogger) log(level LogLevel, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Combine logger fields with message fields
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	if l.format == "json" {
		l.logJSON(level, msg, allFields)
	} else {
		l.logText(level, msg, allFields)
	}
}

// logJSON logs in JSON format
func (l *DefaultLogger) logJSON(level LogLevel, msg string, fields []Field) {
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level.String(),
		"message":   msg,
	}

	for _, field := range fields {
		entry[field.Key] = field.Value
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(l.output, "Error marshaling log entry: %v\n", err)
		return
	}

	fmt.Fprintf(l.output, "%s\n", data)
}

// logText logs in text format
func (l *DefaultLogger) logText(level LogLevel, msg string, fields []Field) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.output, "[%s] %s: %s", timestamp, level.String(), msg)

	if len(fields) > 0 {
		fmt.Fprint(l.output, " {")
		for i, field := range fields {
			if i > 0 {
				fmt.Fprint(l.output, ", ")
			}
			fmt.Fprintf(l.output, "%s=%v", field.Key, field.Value)
		}
		fmt.Fprint(l.output, "}")
	}

	fmt.Fprintln(l.output)
}
