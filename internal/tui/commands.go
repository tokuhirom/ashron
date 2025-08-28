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
					return cmdHelp(cr)
				},
			},
			"/clear": {
				Name:        "/clear",
				Description: "Clear the screen",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					var b strings.Builder
					b.WriteString("\033[2J\033[H") // Clear screen and move cursor to top
					b.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("#7D56F4")).
						Bold(true).
						Render("🤖 Ashron - AI Coding Assistant\n\n"))
					return tea.Printf(b.String())
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

					return tea.Printf("\n%s\n", msg)
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
func cmdHelp(cr *CommandRegistry) tea.Cmd {
	var sb strings.Builder
	sb.WriteString("Available Commands:\n")
	for _, cmd := range cr.commands {
		sb.WriteString("  " + cmd.Name + " - " + cmd.Description + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(`

Keyboard Shortcuts:
  Ctrl+J     - Send message
  Alt+Enter  - Send message (alternative)
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear screen
  y/n        - Approve/Cancel tool execution (when prompted)
  Enter      - New line in input`)

	return tea.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(sb.String()))
}
