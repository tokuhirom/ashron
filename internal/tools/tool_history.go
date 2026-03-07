package tools

import (
	"fmt"
	"strings"
)

const (
	defaultToolHistoryLimit = 4000
	readFileHistoryLimit    = 8000
	commandHistoryLimit     = 6000
	searchHistoryLimit      = 3000
)

// CompactToolResultForHistory limits the amount of tool output saved into
// conversation history to reduce token usage in follow-up requests.
// Different tools use different compaction strategies optimized for their output.
func CompactToolResultForHistory(toolName, output string) string {
	switch toolName {
	case "search_files", "grep_files":
		return compactSearchResult(output, searchHistoryLimit)
	case "list_directory":
		return compactForHistory(output, defaultToolHistoryLimit)
	case "execute_command":
		return compactCommandResult(output, commandHistoryLimit)
	case "read_file":
		return compactForHistory(output, readFileHistoryLimit)
	case "fetch_url":
		return compactForHistory(output, commandHistoryLimit)
	default:
		return compactForHistory(output, defaultToolHistoryLimit)
	}
}

// compactSearchResult preserves match count and top results for search tools.
func compactSearchResult(output string, limit int) string {
	if len(output) <= limit {
		return output
	}
	lines := strings.Split(output, "\n")
	lineCount := len(lines)

	// Keep as many lines as fit within the limit.
	var kept []string
	total := 0
	for _, line := range lines {
		if total+len(line)+1 > limit-100 { // reserve space for summary
			break
		}
		kept = append(kept, line)
		total += len(line) + 1
	}
	summary := fmt.Sprintf("\n[truncated: showing %d of %d lines, %d bytes total]",
		len(kept), lineCount, len(output))
	return strings.Join(kept, "\n") + summary
}

// compactCommandResult uses a 50/50 head/tail split to preserve error messages
// that typically appear at the end of command output.
func compactCommandResult(output string, limit int) string {
	if len(output) <= limit {
		return output
	}
	// For command output, keep more from the tail (error messages tend to be at the end).
	headLen := limit / 2
	tailLen := limit - headLen
	head := output[:headLen]
	tail := output[len(output)-tailLen:]
	return fmt.Sprintf(
		"%s\n\n[truncated: kept first %d and last %d of %d bytes]\n\n%s",
		head, headLen, tailLen, len(output), tail,
	)
}

func compactForHistory(s string, limit int) string {
	if len(s) <= limit || limit <= 0 {
		return s
	}
	headLen := (limit * 3) / 4
	if headLen <= 0 {
		headLen = limit
	}
	tailLen := limit - headLen
	if tailLen < 0 {
		tailLen = 0
	}
	head := s[:headLen]
	if tailLen == 0 {
		return fmt.Sprintf("%s\n\n[truncated for history: kept first %d of %d bytes]", head, headLen, len(s))
	}
	tail := s[len(s)-tailLen:]
	return fmt.Sprintf(
		"%s\n\n[truncated for history: kept first %d and last %d of %d bytes]\n\n%s",
		head,
		headLen,
		tailLen,
		len(s),
		tail,
	)
}
