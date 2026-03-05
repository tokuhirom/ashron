package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	tmp := t.TempDir()
	skillsRoot := filepath.Join(tmp, "ashron", "skills")
	if err := os.MkdirAll(filepath.Join(skillsRoot, "alpha"), 0755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillsRoot, "beta"), 0755); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsRoot, "alpha", "SKILL.md"), []byte("# Alpha\n\nAlpha description"), 0644); err != nil {
		t.Fatalf("write alpha skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsRoot, "beta", "SKILL.md"), []byte("# Beta\n\nBeta description"), 0644); err != nil {
		t.Fatalf("write beta skill: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	got := Discover()
	if len(got) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(got))
	}
	if got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Fatalf("unexpected skill names: %+v", got)
	}
	if got[0].Description != "Alpha description" {
		t.Fatalf("unexpected alpha description: %q", got[0].Description)
	}
}
