package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/config"
)

func TestReadSkill(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	skillPath := filepath.Join(tmp, "ashron", "skills", "alpha", "SKILL.md")
	content := `---
name: alpha
description: test skill
---
body line
`
	if err := writeFile(t, skillPath, content); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	cfg := &config.ToolsConfig{MaxOutputSize: 1024}
	result := ReadSkill(cfg, "tc-1", `{"name":"alpha"}`)
	if result.Error != nil {
		t.Fatalf("ReadSkill() error = %v", result.Error)
	}
	if !strings.Contains(result.Output, "name: alpha") {
		t.Fatalf("unexpected output: %q", result.Output)
	}
	if !strings.Contains(result.Output, "body line") {
		t.Fatalf("unexpected output: %q", result.Output)
	}
}

func TestReadSkillNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &config.ToolsConfig{MaxOutputSize: 1024}
	result := ReadSkill(cfg, "tc-1", `{"name":"missing"}`)
	if result.Error == nil {
		t.Fatal("expected error for missing skill")
	}
	if !strings.Contains(result.Output, "skill not found") {
		t.Fatalf("unexpected output: %q", result.Output)
	}
}

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
