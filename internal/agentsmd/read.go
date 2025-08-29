package agentsmd

import (
	"log/slog"
	"os"
	"strings"
)

func ReadAgentsMD() string {
	// Search for AGENTS.md from current directory upwards
	dir, err := os.Getwd()
	if err != nil {
		slog.Error("Failed to get current directory",
			slog.Any("error", err))
		return ""
	}

	for {
		candidate := dir + "/AGENTS.md"
		if _, err := os.Stat(candidate); err == nil {
			slog.Info("Found AGENTS.md",
				slog.String("path", candidate))

			content, err := os.ReadFile(candidate)
			if err != nil {
				slog.Error("Failed to read AGENTS.md",
					slog.String("path", candidate),
					slog.Any("error", err))
				return ""
			}
			return string(content)
		}

		parent := parentDir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return ""
}

// parentDir returns the parent directory of the given path
func parentDir(path string) string {
	if path == "/" {
		return "/"
	}
	parts := strings.Split(path, string(os.PathSeparator))
	if len(parts) <= 1 {
		return path
	}
	return strings.Join(parts[:len(parts)-1], string(os.PathSeparator))
}
