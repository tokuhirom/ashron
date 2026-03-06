package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders markdown-formatted text for terminal display using
// glamour.  width is the available terminal width for word-wrapping; pass 0
// to use glamour's default.  On any error the original text is returned
// unchanged so that the caller always gets displayable output.
//
// NOTE: glamour.WithAutoStyle() sends an OSC 11 terminal query to detect dark/light
// background. Inside bubbletea (which owns stdin), the terminal response leaks into
// input. We avoid this by using GLAMOUR_STYLE env var or defaulting to "dark".
func renderMarkdown(text string, width int) string {
	if text == "" {
		return ""
	}

	style := os.Getenv("GLAMOUR_STYLE")
	if style == "" {
		style = "dark"
	}
	opts := []glamour.TermRendererOption{
		glamour.WithStandardStyle(style),
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
