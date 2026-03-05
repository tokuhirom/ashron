package tui

import (
	"charm.land/lipgloss/v2"
)

var (
	// Color palette
	primaryColor = lipgloss.Color("#7D56F4")

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)
