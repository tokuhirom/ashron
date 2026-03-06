package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceRangeSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	args, _ := json.Marshal(ReplaceRangeArgs{Path: path, StartLine: 2, EndLine: 3, Content: "x\ny"})
	res := ReplaceRange(nil, "tc1", string(args))
	if res.Error != nil {
		t.Fatalf("ReplaceRange error: %v\noutput=%s", res.Error, res.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "a\nx\ny\nd\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
	if !strings.Contains(res.Output, "Successfully replaced lines 2-3") {
		t.Fatalf("missing range summary: %s", res.Output)
	}
}

func TestReplaceRangeOutOfBounds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	args, _ := json.Marshal(ReplaceRangeArgs{Path: path, StartLine: 2, EndLine: 5, Content: "x"})
	res := ReplaceRange(nil, "tc2", string(args))
	if res.Error == nil {
		t.Fatalf("expected ReplaceRange failure")
	}
	if !strings.Contains(res.Output, "line range out of bounds") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
}
