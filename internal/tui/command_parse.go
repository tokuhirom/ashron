package tui

import (
	"fmt"
	"strings"
)

// parseCommandLine parses slash commands with support for quoting and escaping.
// Supported forms:
// - plain words: /cmd a b
// - quoted args: /cmd "a b" 'c d'
// - escaping: /cmd a\ b "x\"y"
func parseCommandLine(input string) ([]string, error) {
	var out []string
	var cur strings.Builder

	inSingle := false
	inDouble := false
	escape := false
	hadQuotedSegment := false

	flush := func() {
		if cur.Len() > 0 || hadQuotedSegment {
			out = append(out, cur.String())
			cur.Reset()
			hadQuotedSegment = false
		}
	}

	for _, r := range input {
		if escape {
			cur.WriteRune(r)
			escape = false
			continue
		}

		switch r {
		case '\\':
			escape = true
		case '\'':
			if inDouble {
				cur.WriteRune(r)
			} else {
				inSingle = !inSingle
				hadQuotedSegment = true
			}
		case '"':
			if inSingle {
				cur.WriteRune(r)
			} else {
				inDouble = !inDouble
				hadQuotedSegment = true
			}
		case ' ', '\t', '\n':
			if inSingle || inDouble {
				cur.WriteRune(r)
			} else {
				flush()
			}
		default:
			cur.WriteRune(r)
		}
	}

	if escape {
		return nil, fmt.Errorf("command ends with trailing escape")
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	flush()
	return out, nil
}
