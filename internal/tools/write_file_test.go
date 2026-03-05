package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeWriteFileChangeNewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	change, err := AnalyzeWriteFileChange(path, "a\nb\n")
	if err != nil {
		t.Fatalf("AnalyzeWriteFileChange error: %v", err)
	}

	if change.Existed {
		t.Fatalf("expected non-existing file")
	}
	if change.OldLines != 0 || change.NewLines != 2 {
		t.Fatalf("unexpected line counts: %#v", change)
	}
	if change.Added != 2 || change.Removed != 0 {
		t.Fatalf("unexpected change stats: %#v", change)
	}
}

func TestAnalyzeWriteFileChangeExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	change, err := AnalyzeWriteFileChange(path, "a\nx\nc\nz\n")
	if err != nil {
		t.Fatalf("AnalyzeWriteFileChange error: %v", err)
	}

	if !change.Existed {
		t.Fatalf("expected existing file")
	}
	if change.OldLines != 3 || change.NewLines != 4 {
		t.Fatalf("unexpected line counts: %#v", change)
	}
	if change.Added != 2 || change.Removed != 1 {
		t.Fatalf("unexpected change stats: %#v", change)
	}
}

func TestWriteFileCreatesBackupWhenOverwriting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	path := filepath.Join(dir, "test.txt")
	old := "old content\n"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	args, _ := json.Marshal(WriteFileArgs{Path: path, Content: "new content\n"})
	result := WriteFile(nil, "tc1", string(args))
	if result.Error != nil {
		t.Fatalf("WriteFile error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "Backup: ") {
		t.Fatalf("expected backup line, got: %s", result.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "new content\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}

	backupPath := ""
	for _, line := range strings.Split(result.Output, "\n") {
		if strings.HasPrefix(line, "Backup: ") {
			backupPath = strings.TrimPrefix(line, "Backup: ")
		}
	}
	if backupPath == "" {
		t.Fatalf("backup path not found in output")
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != old {
		t.Fatalf("unexpected backup content: %q", string(backup))
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %o", st.Mode().Perm())
	}
}

func TestWriteFileNewFileDoesNotCreateBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	args, _ := json.Marshal(WriteFileArgs{Path: path, Content: "hello\n"})
	result := WriteFile(nil, "tc2", string(args))
	if result.Error != nil {
		t.Fatalf("WriteFile error: %v", result.Error)
	}
	if strings.Contains(result.Output, "Backup: ") {
		t.Fatalf("did not expect backup for new file: %s", result.Output)
	}
}
