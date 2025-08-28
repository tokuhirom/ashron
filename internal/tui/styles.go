package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	primaryColor = lipgloss.Color("#7D56F4")

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)
