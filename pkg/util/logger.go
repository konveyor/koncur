package util

import (
	"log/slog"
	"os"

	"github.com/go-logr/logr"
)

var logger logr.Logger

// InitLogger initializes the global logger with the specified log level
func InitLogger(verbose bool) {
	var level slog.Level
	if verbose {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	slogger := slog.New(handler)
	logger = logr.FromSlogHandler(handler)
	slog.SetDefault(slogger)
}

// GetLogger returns the global logger instance
func GetLogger() logr.Logger {
	if logger.GetSink() == nil {
		// Initialize with default settings if not already initialized
		InitLogger(false)
	}
	return logger
}
