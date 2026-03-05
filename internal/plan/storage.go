package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DataDir returns the directory where plan files are stored.
// Follows XDG Base Directory Specification.
func DataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ashron", "plans")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "share", "ashron", "plans")
}

// Save writes plan content to a timestamped markdown file and returns the full path.
func Save(sessionID, content string) (string, error) {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create plan dir: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	base := ts
	if sessionID != "" {
		base = ts + "-" + sanitizeFileName(sessionID)
	}
	path := filepath.Join(dir, base+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write plan file: %w", err)
	}
	return path, nil
}

func sanitizeFileName(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, string(filepath.Separator), "-")
	s = strings.TrimSpace(s)
	if s == "" {
		return "session"
	}
	return s
}
