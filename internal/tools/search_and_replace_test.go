package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAndReplaceSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello\nhello\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	args, _ := json.Marshal(SearchAndReplaceArgs{Path: path, Search: "hello", Replace: "world"})
	res := SearchAndReplace(nil, "tc1", string(args))
	if res.Error != nil {
		t.Fatalf("SearchAndReplace error: %v\noutput=%s", res.Error, res.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "world\nworld\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
	if !strings.Contains(res.Output, "Successfully replaced 2 occurrence") {
		t.Fatalf("missing replacement summary: %s", res.Output)
	}
}

func TestSearchAndReplaceSearchNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	args, _ := json.Marshal(SearchAndReplaceArgs{Path: path, Search: "missing", Replace: "x"})
	res := SearchAndReplace(nil, "tc2", string(args))
	if res.Error == nil {
		t.Fatalf("expected SearchAndReplace failure")
	}
	if !strings.Contains(res.Output, "search text not found") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
}
