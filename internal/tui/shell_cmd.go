package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type shellCmdMsg struct {
	command string
	output  string
	err     error
}

// runShellCommand executes a shell command and returns its combined output as a tea.Cmd.
func (m *SimpleModel) runShellCommand(command string) tea.Cmd {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	m.textarea.SetValue("")

	cmdLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("$ " + command)
	m.AddDisplayContent(cmdLabel)

	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", command)
		out, err := cmd.CombinedOutput()
		return shellCmdMsg{
			command: command,
			output:  string(out),
			err:     err,
		}
	}
}

// handleShellCmdMsg renders the result of a shell command to the display.
func (m *SimpleModel) handleShellCmdMsg(msg shellCmdMsg) {
	if msg.output != "" {
		lines := strings.Split(strings.TrimRight(msg.output, "\n"), "\n")
		for _, line := range lines {
			m.AddDisplayContent(line)
		}
	}
	if msg.err != nil {
		errLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("exit error: %v", msg.err))
		m.AddDisplayContent(errLine)
	}
	m.AddDisplayContent("")
}
