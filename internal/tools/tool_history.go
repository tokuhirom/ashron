package tools

import "fmt"

const (
	defaultToolHistoryLimit = 4000
	readFileHistoryLimit    = 8000
	commandHistoryLimit     = 6000
)

// CompactToolResultForHistory limits the amount of tool output saved into
// conversation history to reduce token usage in follow-up requests.
func CompactToolResultForHistory(toolName, output string) string {
	limit := defaultToolHistoryLimit
	switch toolName {
	case "read_file":
		limit = readFileHistoryLimit
	case "execute_command", "fetch_url":
		limit = commandHistoryLimit
	}
	return compactForHistory(output, limit)
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
