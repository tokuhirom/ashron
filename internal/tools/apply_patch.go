package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

type matchMode string

const (
	matchExact          matchMode = "exact"
	matchTrimTrailingWS matchMode = "trim-trailing-whitespace"
)

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

	change, err := AnalyzeWriteFileChange(path, newContent)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error analyzing patch changes: %v", err)
		return result
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
	// Validate each hunk's declared old/new lengths against actual body lines.
	// This catches malformed patches early and avoids partial misapplication.
	for i := range hunks {
		oldCount, newCount := hunkLineCounts(hunks[i].Lines)
		if oldCount != hunks[i].OldLen || newCount != hunks[i].NewLen {
			return nil, fmt.Errorf(
				"hunk header/body mismatch (%s): header old/new=%d/%d, body old/new=%d/%d",
				hunks[i].Header, hunks[i].OldLen, hunks[i].NewLen, oldCount, newCount,
			)
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
		// Match using the original-side sequence (context + removed lines).
		// We fail on ambiguous matches to avoid silently patching the wrong block.
		pos, mode, err := locateSequence(result, h.OldSequence, preferred)
		if err != nil {
			return nil, formatHunkApplyError(i+1, h, result, preferred, err)
		}
		before := append([]string(nil), result[:pos]...)
		after := append([]string(nil), result[pos+len(h.OldSequence):]...)
		result = append(before, append(h.NewSequence, after...)...)
		offset += len(h.NewSequence) - len(h.OldSequence)
		_ = mode
	}
	return result, nil
}

func locateSequence(lines []string, seq []string, preferred int) (int, matchMode, error) {
	if len(seq) == 0 {
		if preferred < 0 {
			return 0, matchExact, nil
		}
		if preferred > len(lines) {
			return len(lines), matchExact, nil
		}
		return preferred, matchExact, nil
	}

	if preferred >= 0 && preferred+len(seq) <= len(lines) && matchesAt(lines, seq, preferred, false) {
		return preferred, matchExact, nil
	}

	exactMatches := findMatches(lines, seq, false)
	if len(exactMatches) > 0 {
		pos, err := chooseNearestMatch(exactMatches, preferred)
		if err != nil {
			return 0, "", fmt.Errorf("ambiguous exact match at lines %v", toOneBased(exactMatches))
		}
		return pos, matchExact, nil
	}

	trimMatches := findMatches(lines, seq, true)
	if len(trimMatches) > 0 {
		// Fallback for minor formatter drift. We still reject ambiguity.
		pos, err := chooseNearestMatch(trimMatches, preferred)
		if err != nil {
			return 0, "", fmt.Errorf("ambiguous whitespace-tolerant match at lines %v", toOneBased(trimMatches))
		}
		return pos, matchTrimTrailingWS, nil
	}

	return 0, "", errors.New("no matching location found")
}

func findMatches(lines, seq []string, trimTrailingWS bool) []int {
	if len(seq) == 0 {
		return nil
	}
	matches := make([]int, 0, 4)
	for i := 0; i+len(seq) <= len(lines); i++ {
		if matchesAt(lines, seq, i, trimTrailingWS) {
			matches = append(matches, i)
		}
	}
	return matches
}

func matchesAt(lines, seq []string, at int, trimTrailingWS bool) bool {
	for i := range seq {
		lhs := lines[at+i]
		rhs := seq[i]
		if trimTrailingWS {
			lhs = strings.TrimRight(lhs, " \t")
			rhs = strings.TrimRight(rhs, " \t")
		}
		if lhs != rhs {
			return false
		}
	}
	return true
}

func chooseNearestMatch(matches []int, preferred int) (int, error) {
	if len(matches) == 0 {
		return 0, errors.New("no matches")
	}
	type scored struct {
		pos  int
		dist int
	}
	scoredMatches := make([]scored, 0, len(matches))
	for _, pos := range matches {
		dist := pos - preferred
		if dist < 0 {
			dist = -dist
		}
		scoredMatches = append(scoredMatches, scored{pos: pos, dist: dist})
	}
	slices.SortFunc(scoredMatches, func(a, b scored) int {
		if a.dist != b.dist {
			return a.dist - b.dist
		}
		return a.pos - b.pos
	})
	// If two candidates are equally close, there is no deterministic "best" match.
	// Rejecting here avoids corrupting unrelated duplicated blocks.
	if len(scoredMatches) > 1 && scoredMatches[0].dist == scoredMatches[1].dist {
		return 0, errors.New("ambiguous nearest match")
	}
	return scoredMatches[0].pos, nil
}

func toOneBased(pos []int) []int {
	result := make([]int, 0, len(pos))
	for _, p := range pos {
		result = append(result, p+1)
	}
	return result
}

func formatHunkApplyError(index int, h patchHunk, lines []string, preferred int, cause error) error {
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
		"Patch failed at hunk %d (%s): %v.\nExpected to find:\n%s\n\nAround target line %d:\n%s\n\nRetry hint: read the latest file content around line %d and regenerate a minimal hunk with stable context lines.",
		index,
		h.Header,
		cause,
		truncateMultiline(expected, 800),
		h.OldStart,
		truncateMultiline(around, 800),
		h.OldStart,
	)
	return errors.New(msg)
}

func hunkLineCounts(lines []string) (oldCount int, newCount int) {
	for _, line := range lines {
		if line == "" {
			continue
		}
		switch line[0] {
		case ' ':
			oldCount++
			newCount++
		case '-':
			oldCount++
		case '+':
			newCount++
		}
	}
	return oldCount, newCount
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
