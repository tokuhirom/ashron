package tui

import (
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
)

// TestTextareaSingleBorder verifies that the textarea footer renders with exactly
// one rounded-border frame (╭). Previously, setting Focused.Base to a border style
// caused a double-border: one from the textarea's own Base style and another from
// the surrounding renderFooter wrapper, resulting in nested ╭ characters.
func TestTextareaSingleBorder(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	// Give the model a realistic width so borders are rendered
	m.width = 80
	m.height = 24

	footer := m.renderFooter()

	count := strings.Count(footer, "╭")
	if count != 1 {
		t.Errorf("renderFooter() contains %d '╭' characters, want exactly 1 (double border bug)\nfooter:\n%s", count, footer)
	}
}

// TestTextareaShellModeSingleBorder verifies the same constraint when the user
// has typed '!' to enter shell mode (which switches to shellTextareaBorder).
func TestTextareaShellModeSingleBorder(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	m.width = 80
	m.height = 24

	m.textarea.SetValue("!")
	m.updateInputMode()

	footer := m.renderFooter()

	count := strings.Count(footer, "╭")
	if count != 1 {
		t.Errorf("shell mode renderFooter() contains %d '╭' characters, want exactly 1 (double border bug)\nfooter:\n%s", count, footer)
	}
}
