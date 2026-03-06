package tui

import (
	"charm.land/lipgloss/v2"
)

var (
	// Color palette
	primaryColor   = lipgloss.Color("#7D56F4")
	shellModeColor = lipgloss.Color("#04B575") // green for shell mode

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// Textarea border styles
	normalTextareaBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)

	shellTextareaBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(shellModeColor)

	approvalBorderColor  = lipgloss.Color("#FFA500")
	approvalPromptBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(approvalBorderColor)
)
