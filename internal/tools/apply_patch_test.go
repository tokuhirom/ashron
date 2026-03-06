package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPatchSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	patch := "@@ -1,3 +1,3 @@\n a\n-b\n+x\n c\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc1", string(args))
	if res.Error != nil {
		t.Fatalf("ApplyPatch error: %v\noutput=%s", res.Error, res.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "a\nx\nc\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
	if !strings.Contains(res.Output, "Hunks: 1") {
		t.Fatalf("missing hunk summary: %s", res.Output)
	}
}

func TestApplyPatchFuzzyMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("head\na\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Header points to old line 1 but actual sequence starts at line 2.
	patch := "@@ -1,3 +1,3 @@\n a\n-b\n+y\n c\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc2", string(args))
	if res.Error != nil {
		t.Fatalf("ApplyPatch error: %v\noutput=%s", res.Error, res.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "head\na\ny\nc\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestApplyPatchFailureIncludesRetryHint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	patch := "@@ -1,2 +1,2 @@\n z\n-b\n+x\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc3", string(args))
	if res.Error == nil {
		t.Fatalf("expected ApplyPatch failure")
	}
	if !strings.Contains(res.Output, "Patch failed at hunk 1") {
		t.Fatalf("expected hunk failure info: %s", res.Output)
	}
	if !strings.Contains(res.Output, "Retry hint") {
		t.Fatalf("expected retry hint: %s", res.Output)
	}
}

func TestApplyPatchRejectsAmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	// "a\nb\nc" appears twice, both 3 lines away from preferred line 4.
	if err := os.WriteFile(path, []byte("a\nb\nc\nx\ny\nz\na\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	patch := "@@ -4,3 +4,3 @@\n a\n-b\n+y\n c\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc4", string(args))
	if res.Error == nil {
		t.Fatalf("expected ApplyPatch ambiguity failure")
	}
	if !strings.Contains(res.Output, "ambiguous exact match") {
		t.Fatalf("expected ambiguity detail: %s", res.Output)
	}
}

func TestApplyPatchAcceptsTrailingWhitespaceDifference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb \nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	patch := "@@ -1,3 +1,3 @@\n a\n-b\n+x\n c\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc5", string(args))
	if res.Error != nil {
		t.Fatalf("ApplyPatch error: %v\noutput=%s", res.Error, res.Output)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "a\nx\nc\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestApplyPatchRejectsHeaderBodyMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	patch := "@@ -1,99 +1,1 @@\n a\n"
	args, _ := json.Marshal(ApplyPatchArgs{Path: path, Patch: patch})
	res := ApplyPatch(nil, "tc6", string(args))
	if res.Error == nil {
		t.Fatalf("expected parse failure")
	}
	if !strings.Contains(res.Output, "hunk header/body mismatch") {
		t.Fatalf("expected hunk mismatch detail: %s", res.Output)
	}
}
