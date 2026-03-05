package tui

import "fmt"

const (
	maxDisplayInputChars = 1600
	maxDisplayInputLines = 28
	headPreviewLines     = 16
	tailPreviewLines     = 8
)

func compactUserInputForDisplay(input string) string {
	if input == "" {
		return input
	}

	lines := splitLinesPreserveTrailing(input)
	if len(input) <= maxDisplayInputChars && len(lines) <= maxDisplayInputLines {
		return input
	}

	if len(lines) > maxDisplayInputLines {
		headN := min(headPreviewLines, len(lines))
		tailN := min(tailPreviewLines, len(lines)-headN)
		omittedLines := len(lines) - headN - tailN

		head := joinLines(lines[:headN])
		tail := ""
		if tailN > 0 {
			tail = joinLines(lines[len(lines)-tailN:])
		}

		omittedChars := len(input) - len(head) - len(tail)
		if omittedChars < 0 {
			omittedChars = 0
		}

		marker := fmt.Sprintf("\n...[omitted %d lines, %d chars]...\n", omittedLines, omittedChars)
		if tail == "" {
			return head + marker
		}
		return head + marker + tail
	}

	// Single/short-line very long text: trim by character budget.
	head := input
	tail := ""
	if len(input) > maxDisplayInputChars {
		headLen := maxDisplayInputChars * 2 / 3
		tailLen := maxDisplayInputChars - headLen
		if headLen > len(input) {
			headLen = len(input)
		}
		if tailLen > len(input)-headLen {
			tailLen = len(input) - headLen
		}
		head = input[:headLen]
		tail = input[len(input)-tailLen:]
	}

	omittedChars := len(input) - len(head) - len(tail)
	if omittedChars < 0 {
		omittedChars = 0
	}
	return fmt.Sprintf("%s\n...[omitted %d chars]...\n%s", head, omittedChars, tail)
}

func splitLinesPreserveTrailing(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}
