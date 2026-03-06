package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		// Open the log file
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		logFile = file

		// Create JSON handler for file logging
		handler = slog.NewJSONHandler(file, opts)
	} else {
		// Discard logs if no log file
		handler = slog.NewTextHandler(io.Discard, opts)
	}

	// Set default logger
	slog.SetDefault(slog.New(handler))

	return nil
}

// DefaultLogFilePath returns a timestamped log file path under the XDG data dir.
func DefaultLogFilePath(now time.Time) string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.Getenv("HOME")
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	filename := fmt.Sprintf("ashron-%s.log", now.Format("20060102-150405"))
	return filepath.Join(dataHome, "ashron", "logs", filename)
}

// Close closes the log file if open
func Close() {
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			slog.Error("failed to close log file",
				slog.Any("error", err))
		}
	}
}
