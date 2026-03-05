package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type ApplyPatchArgs struct {
	Path  string `json:"path"`
	Patch string `json:"patch"`
}

type patchHunk struct {
	Header      string
	OldStart    int
	OldLen      int
	NewStart    int
	NewLen      int
	Lines       []string
	OldSequence []string
	NewSequence []string
}

var hunkHeaderRE = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func ApplyPatch(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args ApplyPatchArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	path := filepath.Clean(args.Path)
	hunks, err := parsePatch(args.Patch)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error parsing patch: %v", err)
		return result
	}
	if len(hunks) == 0 {
		result.Error = fmt.Errorf("no hunks found")
		result.Output = "Error: patch has no hunks (expected unified diff hunks starting with @@)"
		return result
	}

	oldContent, existed, err := readExistingFile(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading target file: %v", err)
		return result
	}
	lines := splitLines(oldContent)

	newLines, applyErr := applyHunks(lines, hunks)
	if applyErr != nil {
		result.Error = applyErr
		result.Output = applyErr.Error()
		return result
	}
	newContent := strings.Join(newLines, "\n")
	if oldContent != "" && strings.HasSuffix(oldContent, "\n") {
		if newContent != "" && !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error creating directory: %v", err)
		return result
	}

	var backupPath string
	if existed {
		backupPath, err = createBackup(path)
		if err != nil {
			result.Error = err
			result.Output = fmt.Sprintf("Error creating backup: %v", err)
			return result
		}
	}
	if err := atomicWrite(path, []byte(newContent)); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error applying patch: %v", err)
		return result
	}

	change, err := AnalyzeWriteFileChange(path, newContent)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Patch applied but change analysis failed: %v", err)
		return result
	}

	added, removed := patchStats(hunks)
	result.Output = fmt.Sprintf(
		"Successfully applied patch to %s\nHunks: %d, patch lines: +%d -%d\nChange summary: lines %d -> %d, +%d -%d",
		path, len(hunks), added, removed, change.OldLines, change.NewLines, change.Added, change.Removed,
	)
	if backupPath != "" {
		result.Output += "\nBackup: " + backupPath
	}
	return result
}

func parsePatch(patch string) ([]patchHunk, error) {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	var hunks []patchHunk
	var current *patchHunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			h.Header = line
			hunks = append(hunks, h)
			current = &hunks[len(hunks)-1]
			continue
		}
		if current == nil {
			continue
		}
		if line == "\\ No newline at end of file" {
			continue
		}
		if line == "" {
			continue
		}
		prefix := line[0]
		if prefix != ' ' && prefix != '+' && prefix != '-' {
			return nil, fmt.Errorf("invalid hunk line: %q", line)
		}
		current.Lines = append(current.Lines, line)
		switch prefix {
		case ' ', '-':
			current.OldSequence = append(current.OldSequence, line[1:])
		}
		switch prefix {
		case ' ', '+':
			current.NewSequence = append(current.NewSequence, line[1:])
		}
	}
	return hunks, nil
}

func parseHunkHeader(line string) (patchHunk, error) {
	m := hunkHeaderRE.FindStringSubmatch(line)
	if len(m) == 0 {
		return patchHunk{}, fmt.Errorf("invalid hunk header: %s", line)
	}
	oldStart := atoiDefault(m[1], 0)
	oldLen := atoiDefault(m[2], 1)
	newStart := atoiDefault(m[3], 0)
	newLen := atoiDefault(m[4], 1)
	return patchHunk{OldStart: oldStart, OldLen: oldLen, NewStart: newStart, NewLen: newLen}, nil
}

func applyHunks(lines []string, hunks []patchHunk) ([]string, error) {
	result := append([]string(nil), lines...)
	offset := 0
	for i, h := range hunks {
		preferred := h.OldStart - 1 + offset
		pos := locateSequence(result, h.OldSequence, preferred)
		if pos < 0 {
			return nil, formatHunkApplyError(i+1, h, result, preferred)
		}
		before := append([]string(nil), result[:pos]...)
		after := append([]string(nil), result[pos+len(h.OldSequence):]...)
		result = append(before, append(h.NewSequence, after...)...)
		offset += len(h.NewSequence) - len(h.OldSequence)
	}
	return result, nil
}

func locateSequence(lines []string, seq []string, preferred int) int {
	if len(seq) == 0 {
		if preferred < 0 {
			return 0
		}
		if preferred > len(lines) {
			return len(lines)
		}
		return preferred
	}

	if preferred >= 0 && preferred+len(seq) <= len(lines) && matchesAt(lines, seq, preferred) {
		return preferred
	}

	for delta := 1; delta <= 8; delta++ {
		up := preferred - delta
		if up >= 0 && up+len(seq) <= len(lines) && matchesAt(lines, seq, up) {
			return up
		}
		down := preferred + delta
		if down >= 0 && down+len(seq) <= len(lines) && matchesAt(lines, seq, down) {
			return down
		}
	}

	for i := 0; i+len(seq) <= len(lines); i++ {
		if matchesAt(lines, seq, i) {
			return i
		}
	}
	return -1
}

func matchesAt(lines, seq []string, at int) bool {
	for i := range seq {
		if lines[at+i] != seq[i] {
			return false
		}
	}
	return true
}

func formatHunkApplyError(index int, h patchHunk, lines []string, preferred int) error {
	ctxStart := preferred - 2
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := preferred + max(6, len(h.OldSequence)+2)
	if ctxEnd > len(lines) {
		ctxEnd = len(lines)
	}
	around := ""
	if ctxStart < ctxEnd {
		around = strings.Join(lines[ctxStart:ctxEnd], "\n")
	}
	expected := ""
	if len(h.OldSequence) > 0 {
		expected = strings.Join(h.OldSequence, "\n")
	}

	msg := fmt.Sprintf(
		"Patch failed at hunk %d (%s).\nExpected to find:\n%s\n\nAround target line %d:\n%s\n\nRetry hint: read the latest file content around line %d and regenerate a minimal hunk with stable context lines.",
		index,
		h.Header,
		truncateMultiline(expected, 800),
		h.OldStart,
		truncateMultiline(around, 800),
		h.OldStart,
	)
	return errors.New(msg)
}

func patchStats(hunks []patchHunk) (added, removed int) {
	for _, h := range hunks {
		for _, l := range h.Lines {
			if len(l) == 0 {
				continue
			}
			switch l[0] {
			case '+':
				added++
			case '-':
				removed++
			}
		}
	}
	return added, removed
}

func truncateMultiline(s string, maxLen int) string {
	if len(s) <= maxLen {
		if s == "" {
			return "(empty)"
		}
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	if n == 0 {
		return def
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
