package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name        string
	Description string
	Path        string
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
			seen[dir] = struct{}{}

			absPath := path
			if p, err := filepath.Abs(path); err == nil {
				absPath = p
			}
			out = append(out, Skill{
				Name:        filepath.Base(dir),
				Description: summarizeSkill(path),
				Path:        absPath,
			})
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
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		roots = append(roots, filepath.Join(xdgConfigHome, "ashron", "skills"))
	}
	if home := os.Getenv("HOME"); home != "" {
		roots = append(roots, filepath.Join(home, ".config", "ashron", "skills"))
	}

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

func summarizeSkill(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		// Keep descriptions short in AGENTS.md.
		runes := []rune(s)
		if len(runes) > 160 {
			return string(runes[:160]) + "..."
		}
		return s
	}
	return ""
}
