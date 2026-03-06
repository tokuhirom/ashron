package logger

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultLogFilePathRespectsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/ashron-xdg")
	got := DefaultLogFilePath(time.Date(2026, 3, 6, 12, 34, 56, 0, time.UTC))
	want := "/tmp/ashron-xdg/ashron/logs/ashron-20260306-123456.log"
	if got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
}

func TestSetupCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "logs", "app.log")
	if err := Setup(path); err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	Close()
}
