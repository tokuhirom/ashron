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
	bgColor        = lipgloss.Color("#1a1a1a")
	fgColor        = lipgloss.Color("#FAFAFA")

	// Base styles
	baseStyle = lipgloss.NewStyle().
		Background(bgColor).
		Foreground(fgColor)

	// Header styles
	headerStyle = lipgloss.NewStyle().
		Background(primaryColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Bold(true)

	// Chat area styles
	chatViewStyle = lipgloss.NewStyle().
		Padding(1)

	// Message styles
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

	// Status bar styles
	statusBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#2a2a2a")).
		Foreground(fgColor).
		Padding(0, 1)

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

// GetWidth returns the terminal width with padding
func GetWidth() int {
	return lipgloss.Width(baseStyle.String()) - 4
}

// RenderHeader renders the application header
func RenderHeader(title string) string {
	return headerStyle.Width(GetWidth()).Render(title)
}

// RenderStatusBar renders the status bar with given text
func RenderStatusBar(text string) string {
	return statusBarStyle.Width(GetWidth()).Render(text)
}
