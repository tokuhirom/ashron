package skills

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

type Skill struct {
	Name        string
	Description string
	Path        string // SKILL.md absolute path
	Dir         string // skill directory absolute path
}

func Discover() []Skill {
	roots := discoverRoots()
	seen := make(map[string]struct{})
	var out []Skill

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if d.Name() != "SKILL.md" {
				return nil
			}
			dir := filepath.Dir(path)
			if _, ok := seen[dir]; ok {
				return nil
			}

			skill, err := parseSkill(path)
			if err != nil {
				// Skip invalid skills silently; users can still inspect files directly.
				return nil
			}
			seen[dir] = struct{}{}
			out = append(out, skill)
			return nil
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Path < out[j].Path
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func discoverRoots() []string {
	var roots []string
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		roots = append(roots, filepath.Join(dir, "ashron", "skills"))
	}
	// Fallback path if UserConfigDir fails in restricted environments.
	roots = append(roots, filepath.Join(xdgConfigDir(), "ashron", "skills"))

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

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

var skillNameRE = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

func parseSkill(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	front, err := extractFrontmatter(string(data))
	if err != nil {
		return Skill{}, err
	}

	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if err := validateFrontmatter(meta); err != nil {
		return Skill{}, err
	}

	absPath := path
	if p, err := filepath.Abs(path); err == nil {
		absPath = p
	}
	dir := filepath.Dir(absPath)
	return Skill{
		Name:        meta.Name,
		Description: meta.Description,
		Path:        absPath,
		Dir:         dir,
	}, nil
}

func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("missing YAML frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), nil
		}
	}
	return "", fmt.Errorf("unterminated YAML frontmatter")
}

func validateFrontmatter(meta skillFrontmatter) error {
	name := strings.TrimSpace(meta.Name)
	desc := strings.TrimSpace(meta.Description)
	if !skillNameRE.MatchString(name) {
		return fmt.Errorf("invalid skill name: %q", meta.Name)
	}
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "anthropic") || strings.Contains(lowerName, "claude") {
		return fmt.Errorf("skill name contains reserved word")
	}
	if containsXMLLikeTag(name) {
		return fmt.Errorf("skill name contains XML-like tag")
	}
	if desc == "" {
		return fmt.Errorf("description is required")
	}
	if len([]rune(desc)) > 1024 {
		return fmt.Errorf("description exceeds 1024 characters")
	}
	if containsXMLLikeTag(desc) {
		return fmt.Errorf("description contains XML-like tag")
	}
	return nil
}

func containsXMLLikeTag(s string) bool {
	if strings.Contains(s, "<") && strings.Contains(s, ">") {
		return true
	}
	return false
}

func MetadataPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Installed skills metadata (load SKILL.md only when relevant):\n")
	for _, sk := range skills {
		fmt.Fprintf(&sb, "- %s: %s\n", sk.Name, sk.Description)
	}
	return strings.TrimSpace(sb.String())
}

// xdgConfigDir returns $XDG_CONFIG_HOME if set, else $HOME/.config per XDG spec.
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
