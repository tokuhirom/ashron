package skills

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(
		filepath.Join(skillsRoot, "alpha", "SKILL.md"),
		[]byte("---\nname: alpha\ndescription: Alpha description\n---\n# Alpha"),
		0644,
	); err != nil {
		t.Fatalf("write alpha skill: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(skillsRoot, "beta", "SKILL.md"),
		[]byte("---\nname: beta\ndescription: Beta description\n---\n# Beta"),
		0644,
	); err != nil {
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

func TestDiscoverSkipsInvalidFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillsRoot := filepath.Join(tmp, "ashron", "skills")
	if err := os.MkdirAll(filepath.Join(skillsRoot, "bad"), 0755); err != nil {
		t.Fatalf("mkdir bad: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(skillsRoot, "bad", "SKILL.md"),
		[]byte("---\nname: BadName\ndescription: has uppercase\n---"),
		0644,
	); err != nil {
		t.Fatalf("write bad skill: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	got := Discover()
	if len(got) != 0 {
		t.Fatalf("expected invalid skill to be skipped, got %+v", got)
	}
}

func TestMetadataPrompt(t *testing.T) {
	got := MetadataPrompt([]Skill{
		{Name: "alpha", Description: "Alpha description"},
		{Name: "beta", Description: "Beta description"},
	})
	if !strings.Contains(got, "alpha: Alpha description") {
		t.Fatalf("missing alpha metadata: %q", got)
	}
	if !strings.Contains(got, "beta: Beta description") {
		t.Fatalf("missing beta metadata: %q", got)
	}
}
