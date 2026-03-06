package memory

import (
	"os"
	"strings"
	"testing"
)

func TestProjectPath(t *testing.T) {
	t.Parallel()
	got := ProjectPath("/home/user/myproject")
	if !strings.HasSuffix(got, "-home-user-myproject.MEMORY.md") {
		t.Fatalf("ProjectPath = %q, want suffix -home-user-myproject.MEMORY.md", got)
	}
}

func TestWriteAndReadGlobal(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	if err := WriteGlobal("hello global"); err != nil {
		t.Fatalf("WriteGlobal: %v", err)
	}
	got := ReadGlobal()
	if got != "hello global" {
		t.Fatalf("ReadGlobal = %q, want %q", got, "hello global")
	}
}

func TestWriteAndReadProject(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cwd, _ := os.Getwd()

	if err := WriteProject(cwd, "hello project"); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	got := ReadProject(cwd)
	if got != "hello project" {
		t.Fatalf("ReadProject = %q, want %q", got, "hello project")
	}
}

func TestSystemPromptSectionEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	got := SystemPromptSection("/some/path")
	if got != "" {
		t.Fatalf("expected empty section when no memory files exist, got %q", got)
	}
}

func TestSystemPromptSectionWithContent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	if err := WriteGlobal("# User prefs\n- prefers Go"); err != nil {
		t.Fatal(err)
	}

	got := SystemPromptSection("/some/path")
	if !strings.Contains(got, "# User prefs") {
		t.Fatalf("expected memory content in section, got %q", got)
	}
	if !strings.Contains(got, "Global Memory") {
		t.Fatalf("expected Global Memory header, got %q", got)
	}
}
