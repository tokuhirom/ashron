package customcmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Command struct {
	Name        string
	Description string
	Template    string
	Path        string
}

type frontmatter struct {
	Description string `yaml:"description"`
}

var nameRE = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

func Discover() []Command {
	roots := discoverRoots()
	seen := make(map[string]struct{})
	var out []Command

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) != ".md" {
				return nil
			}

			cmd, err := parseCommand(path)
			if err != nil {
				return nil
			}
			if _, ok := seen[cmd.Name]; ok {
				return nil
			}
			seen[cmd.Name] = struct{}{}
			out = append(out, cmd)
			return nil
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func discoverRoots() []string {
	var roots []string
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		roots = append(roots, filepath.Join(dir, "ashron", "commands"))
	}
	roots = append(roots, filepath.Join(xdgConfigDir(), "ashron", "commands"))

	seen := make(map[string]struct{})
	var existing []string
	for _, root := range roots {
		if root == "" {
			continue
		}
		abs := root
		if p, err := filepath.Abs(root); err == nil {
			abs = p
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			existing = append(existing, abs)
		}
	}
	return existing
}

func parseCommand(path string) (Command, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !nameRE.MatchString(name) {
		return Command{}, fmt.Errorf("invalid command name: %s", name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Command{}, err
	}
	content := string(data)
	desc := "Custom command"
	template := content

	if front, body, ok := extractFrontmatter(content); ok {
		var fm frontmatter
		if err := yaml.Unmarshal([]byte(front), &fm); err == nil {
			if strings.TrimSpace(fm.Description) != "" {
				desc = strings.TrimSpace(fm.Description)
			}
		}
		template = body
	}
	template = strings.TrimSpace(template)
	if template == "" {
		return Command{}, fmt.Errorf("empty template")
	}

	if desc == "Custom command" {
		desc = firstNonEmptyLine(template)
		if desc == "" {
			desc = "Custom command"
		}
	}

	absPath := path
	if p, err := filepath.Abs(path); err == nil {
		absPath = p
	}
	return Command{
		Name:        name,
		Description: truncate(desc, 80),
		Template:    template,
		Path:        absPath,
	}, nil
}

func extractFrontmatter(content string) (front string, body string, ok bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true
		}
	}
	return "", "", false
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Expand renders template with positional arguments.
// Supported placeholders: $ARGUMENTS, $1..$9.
func Expand(template string, args []string) string {
	out := strings.ReplaceAll(template, "$ARGUMENTS", strings.Join(args, " "))
	for i := 9; i >= 1; i-- {
		key := fmt.Sprintf("$%d", i)
		val := ""
		if len(args) >= i {
			val = args[i-1]
		}
		out = strings.ReplaceAll(out, key, val)
	}
	return out
}

func xdgConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config")
}
