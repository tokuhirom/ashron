package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown-formatted text for terminal display using
// glamour.  width is the available terminal width for word-wrapping; pass 0
// to use glamour's default.  On any error the original text is returned
// unchanged so that the caller always gets displayable output.
func renderMarkdown(text string, width int) string {
	if text == "" {
		return ""
	}

	opts := []glamour.TermRendererOption{
		glamour.WithAutoStyle(),
	}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}

	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return text
	}

	out, err := r.Render(text)
	if err != nil {
		return text
	}

	// glamour wraps output in leading/trailing newlines; strip them so callers
	// control spacing.
	return strings.Trim(out, "\n")
}
