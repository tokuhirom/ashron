package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDirUsesXDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	got := DataDir()
	want := filepath.Join(tmp, "ashron", "plans")
	if got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}

func TestSaveWritesPlanFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	path, err := Save("20260305-120000", "plan content")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !strings.HasPrefix(path, filepath.Join(tmp, "ashron", "plans")+string(filepath.Separator)) {
		t.Fatalf("unexpected path: %q", path)
	}
	if filepath.Ext(path) != ".md" {
		t.Fatalf("expected .md extension, got %q", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved plan: %v", err)
	}
	if string(data) != "plan content" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}
