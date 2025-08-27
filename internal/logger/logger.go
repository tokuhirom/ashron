package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

var (
	logFile io.Closer
)

// Setup initializes the logger
func Setup(logFilePath string) error {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time more nicely
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
				}
			}
			return a
		},
	}

	if logFilePath != "" {
		// Open log file
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		logFile = file

		// Create JSON handler for file logging
		handler = slog.NewJSONHandler(file, opts)

		// Log to both file and stderr in development
		if os.Getenv("DEBUG") != "" {
			// Create multi-writer for both file and stderr
			mw := io.MultiWriter(file, os.Stderr)
			handler = slog.NewTextHandler(mw, opts)
		}
	} else if os.Getenv("DEBUG") != "" {
		// Log to stderr in debug mode
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		// Discard logs if no log file and not in debug mode
		handler = slog.NewTextHandler(io.Discard, opts)
	}

	// Set default logger
	slog.SetDefault(slog.New(handler))

	return nil
}

// Close closes the log file if open
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// Debug logs a debug message with context
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs an info message with context
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs a warning message with context
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs an error message with context
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
