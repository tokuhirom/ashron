package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
)

// Session holds a persisted conversation.
type Session struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	WorkingDir string        `json:"working_dir"`
	Provider   string        `json:"provider"`
	Model      string        `json:"model"`
	Messages   []api.Message `json:"messages"`
}

// Summary is a lightweight representation of a persisted session.
type Summary struct {
	ID         string
	CreatedAt  time.Time
	WorkingDir string
	Provider   string
	Model      string
}

// New creates a new session with a timestamp-based ID.
func New(provider, model string) *Session {
	wd, _ := os.Getwd()
	return &Session{
		ID:         time.Now().Format("20060102-150405"),
		CreatedAt:  time.Now(),
		WorkingDir: wd,
		Provider:   provider,
		Model:      model,
	}
}

// DataDir returns the directory where sessions are stored.
// Follows XDG Base Directory Specification.
func DataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ashron", "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "share", "ashron", "sessions")
}

// Save persists the session to disk.
func (s *Session) Save() error {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, s.ID+".json"), data, 0644)
}

// Load reads a session from disk by ID.
func Load(sessionID string) (*Session, error) {
	path := filepath.Join(DataDir(), sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session %q not found: %w", sessionID, err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session %q: %w", sessionID, err)
	}
	return &s, nil
}

// ListSummaries returns persisted sessions sorted by created_at desc.
func ListSummaries(limit int) ([]Summary, error) {
	dir := DataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	summaries := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			continue
		}

		var s Session
		if unmarshalErr := json.Unmarshal(data, &s); unmarshalErr != nil {
			continue
		}

		summaries = append(summaries, Summary{
			ID:         s.ID,
			CreatedAt:  s.CreatedAt,
			WorkingDir: s.WorkingDir,
			Provider:   s.Provider,
			Model:      s.Model,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	if limit > 0 && len(summaries) > limit {
		return summaries[:limit], nil
	}
	return summaries, nil
}
