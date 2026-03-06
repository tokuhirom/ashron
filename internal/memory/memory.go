// Package memory manages persistent memory files for Ashron.
//
// Two scopes are supported:
//   - Global:  ~/.local/share/ashron/memory/MEMORY.md
//   - Project: ~/.local/share/ashron/memory/<encoded-cwd>.MEMORY.md
//
// The project path is encoded by replacing each '/' with '-', so
// /home/user/myproject becomes -home-user-myproject.
package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// MemoryDir returns the base directory where memory files are stored.
func MemoryDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ashron", "memory")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".local", "share", "ashron", "memory")
}

// GlobalPath returns the path to the global memory file.
func GlobalPath() string {
	return filepath.Join(MemoryDir(), "MEMORY.md")
}

// ProjectPath returns the path to the project-scoped memory file for the
// given working directory. The cwd is encoded by replacing '/' with '-'.
func ProjectPath(cwd string) string {
	encoded := strings.ReplaceAll(cwd, string(os.PathSeparator), "-")
	return filepath.Join(MemoryDir(), encoded+".MEMORY.md")
}

// ReadGlobal returns the contents of the global memory file, or "" if absent.
func ReadGlobal() string {
	return readFile(GlobalPath())
}

// ReadProject returns the contents of the project memory file for cwd, or "" if absent.
func ReadProject(cwd string) string {
	return readFile(ProjectPath(cwd))
}

// WriteGlobal overwrites the global memory file with content.
func WriteGlobal(content string) error {
	return writeFile(GlobalPath(), content)
}

// WriteProject overwrites the project memory file for cwd with content.
func WriteProject(cwd, content string) error {
	return writeFile(ProjectPath(cwd), content)
}

// SystemPromptSection returns a formatted section to inject into the system
// prompt. Returns "" if both memory files are empty.
func SystemPromptSection(cwd string) string {
	global := ReadGlobal()
	project := ReadProject(cwd)

	if global == "" && project == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Ashron Memory\n\n")
	sb.WriteString("The following notes were saved in previous sessions. ")
	sb.WriteString("Use the `memory_write` tool to update them.\n\n")

	if global != "" {
		sb.WriteString("### Global Memory\n\n")
		sb.WriteString(global)
		sb.WriteString("\n\n")
	}
	if project != "" {
		sb.WriteString("### Project Memory\n\n")
		sb.WriteString(project)
		sb.WriteString("\n")
	}

	return sb.String()
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
