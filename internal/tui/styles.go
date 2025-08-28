package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#04B575")
	accentColor    = lipgloss.Color("#FF7F50")
	errorColor     = lipgloss.Color("#FF3333")
	warningColor   = lipgloss.Color("#FFA500")
	mutedColor     = lipgloss.Color("#626262")
	fgColor        = lipgloss.Color("#FAFAFA")

	// Message styles (used in streaming output)
	userMessageStyle = lipgloss.NewStyle().
		Foreground(secondaryColor).
		Bold(true).
		MarginBottom(1)

	assistantMessageStyle = lipgloss.NewStyle().
		Foreground(fgColor).
		MarginBottom(1)

	systemMessageStyle = lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true).
		MarginBottom(1)

	// Tool styles
	toolCallStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#2a2a2a")).
		Foreground(accentColor).
		Padding(0, 1).
		MarginBottom(1)

	toolResultStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#2a2a2a")).
		Foreground(mutedColor).
		Padding(0, 1).
		MarginBottom(1)

	// Error and warning styles
	errorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	warningStyle = lipgloss.NewStyle().
		Foreground(warningColor).
		Bold(true)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
		Foreground(primaryColor)
)
