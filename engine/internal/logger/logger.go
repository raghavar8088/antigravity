// Package logger provides a shared structured logger for the RAIG engine.
// It wraps the stdlib log/slog package for structured JSON output without
// introducing external dependencies.
package logger

import (
	"log/slog"
	"os"
)

// Log is the shared structured logger for the RAIG engine.
// All engine subsystems should prefer logger.Log over the stdlib log package
// so log output is consistently structured JSON.
var Log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// Info logs at INFO level with optional key-value pairs.
func Info(msg string, args ...any) {
	Log.Info(msg, args...)
}

// Warn logs at WARN level with optional key-value pairs.
func Warn(msg string, args ...any) {
	Log.Warn(msg, args...)
}

// Error logs at ERROR level with optional key-value pairs.
func Error(msg string, args ...any) {
	Log.Error(msg, args...)
}

// Debug logs at DEBUG level with optional key-value pairs.
func Debug(msg string, args ...any) {
	Log.Debug(msg, args...)
}
