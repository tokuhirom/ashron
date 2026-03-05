package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileChange contains a lightweight line-based change summary.
type WriteFileChange struct {
	Path      string
	Existed   bool
	OldLines  int
	NewLines  int
	Added     int
	Removed   int
	Unchanged bool
}

// AnalyzeWriteFileChange compares current file content and new content.
func AnalyzeWriteFileChange(path string, newContent string) (WriteFileChange, error) {
	cleanPath := filepath.Clean(path)
	oldContent, existed, err := readExistingFile(cleanPath)
	if err != nil {
		return WriteFileChange{}, err
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	added, removed := lineDiffStats(oldLines, newLines)

	return WriteFileChange{
		Path:      cleanPath,
		Existed:   existed,
		OldLines:  len(oldLines),
		NewLines:  len(newLines),
		Added:     added,
		Removed:   removed,
		Unchanged: oldContent == newContent,
	}, nil
}

func lineDiffStats(oldLines, newLines []string) (added int, removed int) {
	n := len(oldLines)
	m := len(newLines)
	if n == 0 {
		return m, 0
	}
	if m == 0 {
		return 0, n
	}

	// Keep memory/time bounded for very large files.
	const maxCells = 2_000_000
	if n*m > maxCells {
		prefix := 0
		for prefix < n && prefix < m && oldLines[prefix] == newLines[prefix] {
			prefix++
		}
		return m - prefix, n - prefix
	}

	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldLines[i-1] == newLines[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
		clear(curr)
	}

	lcs := prev[m]
	return m - lcs, n - lcs
}

func readExistingFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read existing file %q: %w", path, err)
	}
	return string(data), true, nil
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}
