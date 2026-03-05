package customcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ashron", "commands")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "review.md"), []byte("---\ndescription: Review current changes\n---\nPlease review $ARGUMENTS"), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad name.md"), []byte("ignored"), 0644); err != nil {
		t.Fatalf("write bad: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	got := Discover()
	if len(got) != 1 {
		t.Fatalf("expected 1 command, got %d", len(got))
	}
	if got[0].Name != "review" {
		t.Fatalf("unexpected name: %s", got[0].Name)
	}
	if got[0].Description != "Review current changes" {
		t.Fatalf("unexpected description: %s", got[0].Description)
	}
}

func TestExpand(t *testing.T) {
	template := "do $1 then $2 all:$ARGUMENTS"
	got := Expand(template, []string{"alpha", "beta"})
	want := "do alpha then beta all:alpha beta"
	if got != want {
		t.Fatalf("Expand() = %q, want %q", got, want)
	}
}
