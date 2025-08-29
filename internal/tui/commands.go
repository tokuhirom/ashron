package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Command struct {
	Name        string
	Description string
	Body        func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd
}

type CommandRegistry struct {
	commands map[string]Command
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: map[string]Command{
			"/help": {
				Name:        "/help",
				Description: "Show help information",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return cmdHelp(cr, m)
				},
			},
			"/clear": {
				Name:        "/clear",
				Description: "Clear the screen",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					welcomeMsg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#7D56F4")).
						Bold(true).
						Render("🤖 Ashron - AI Coding Assistant")
					helpMsg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#626262")).
						Render("Type /help for available commands")
					m.displayContent = []string{welcomeMsg, helpMsg, ""}
					m.viewport.GotoTop()
					return nil
				},
			},
			"/compact": {
				Name:        "/compact",
				Description: "Compact conversation context",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					originalCount, newCount := m.CompactContext()

					msg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#626262")).
						Render(fmt.Sprintf("Context compacted: %d → %d messages", originalCount, newCount))

					m.AddDisplayContent(msg, "")
					return nil
				},
			},
			"/commit": {
				Name:        "/commit",
				Description: "Commit changes to git with a message",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.SendMessage("Run git status/git diff/git diff --cached to check the current changes, then generate commit message. and commit it.")
				},
			},
			"/init": {
				Name:        "/init",
				Description: "Generate AGENTS.md from current directory",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.InitProject()
				},
			},
			"/quit": {
				Name:        "/quit",
				Description: "Exit the application",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return tea.Quit
				},
			},
			"/exit": {
				Name:        "/exit",
				Description: "Exit the application",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return tea.Quit
				},
			},
			"/config": {
				Name:        "/config",
				Description: "Show current configuration",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderConfig()
				},
			},
		},
	}
}

func (c *CommandRegistry) GetCommand(name string) (*Command, bool) {
	cmd, exists := c.commands[name]
	return &cmd, exists
}

// cmdHelp displays available commands
func cmdHelp(cr *CommandRegistry, m *SimpleModel) tea.Cmd {
	var sb strings.Builder
	sb.WriteString("Available Commands:\n")
	for _, cmd := range cr.commands {
		sb.WriteString("  " + cmd.Name + " - " + cmd.Description + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(`

Keyboard Shortcuts:
  Ctrl+J     - Insert new line
  Enter      - Send message
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear screen
  y/n        - Approve/Cancel tool execution (when prompted)
  Enter      - New line in input`)

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(sb.String())

	// Add help text to display content
	lines := strings.Split(helpText, "\n")
	for _, line := range lines {
		m.AddDisplayContent(line)
	}
	m.AddDisplayContent("")

	return nil
}
