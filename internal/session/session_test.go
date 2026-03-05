package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListSummariesSortedByCreatedAtDesc(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	s1 := &Session{ID: "old", CreatedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), WorkingDir: "/tmp/a", Provider: "openai", Model: "gpt-4.1"}
	s2 := &Session{ID: "new", CreatedAt: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC), WorkingDir: "/tmp/b", Provider: "openai", Model: "gpt-4.1"}
	if err := s1.Save(); err != nil {
		t.Fatalf("save old session: %v", err)
	}
	if err := s2.Save(); err != nil {
		t.Fatalf("save new session: %v", err)
	}

	summaries, err := ListSummaries(0)
	if err != nil {
		t.Fatalf("ListSummaries error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].ID != "new" || summaries[1].ID != "old" {
		t.Fatalf("unexpected order: %+v", summaries)
	}
}

func TestListSummariesLimit(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	for i := 0; i < 3; i++ {
		s := &Session{
			ID:         time.Date(2026, 3, i+1, 10, 0, 0, 0, time.UTC).Format("20060102-150405"),
			CreatedAt:  time.Date(2026, 3, i+1, 10, 0, 0, 0, time.UTC),
			WorkingDir: "/tmp",
			Provider:   "openai",
			Model:      "gpt-4.1",
		}
		if err := s.Save(); err != nil {
			t.Fatalf("save session %d: %v", i, err)
		}
	}

	summaries, err := ListSummaries(2)
	if err != nil {
		t.Fatalf("ListSummaries error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestListSummariesMissingDirectory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	dir := DataDir()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("session dir should not exist yet: %s", dir)
	}

	summaries, err := ListSummaries(10)
	if err != nil {
		t.Fatalf("ListSummaries error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected no summaries, got %d", len(summaries))
	}
}

func TestLoadReadsFromDataDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s := &Session{ID: "abc", CreatedAt: time.Now(), WorkingDir: "/tmp", Provider: "openai", Model: "gpt-4.1"}
	if err := s.Save(); err != nil {
		t.Fatalf("save session: %v", err)
	}

	if _, err := os.Stat(filepath.Join(DataDir(), "abc.json")); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	loaded, err := Load("abc")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.ID != "abc" {
		t.Fatalf("unexpected loaded id: %s", loaded.ID)
	}
}
