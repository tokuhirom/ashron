package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
					m.displayContent = []string{welcomeMsg, helpMsg}
					if m.config.Tools.Yolo {
						yoloMsg := lipgloss.NewStyle().
							Foreground(lipgloss.Color("#FF3333")).
							Bold(true).
							Render("YOLO MODE ENABLED: sandbox disabled and tools auto-approved")
						m.displayContent = append(m.displayContent, yoloMsg)
					}
					m.displayContent = append(m.displayContent, "")
					m.viewportDirty = true
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
			"/status": {
				Name:        "/status",
				Description: "Show runtime status (model, approvals, sandbox, cwd)",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderStatus()
				},
			},
			"/sessions": {
				Name:        "/sessions",
				Description: "Manage sessions. Usage: /sessions [list|resume <id>|delete <id>]",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderSessions(args)
				},
			},
			"/tools": {
				Name:        "/tools",
				Description: "List tools and approval policy",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderTools()
				},
			},
			"/skills": {
				Name:        "/skills",
				Description: "List locally available skills",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderSkills()
				},
			},
			"/model": {
				Name:        "/model",
				Description: "Show or switch model. Usage: /model [name]",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					if len(args) == 0 {
						var sb strings.Builder
						fmt.Fprintf(&sb, "Current model: %s (provider: %s)\n\nAvailable models:\n", m.currentModelName, m.currentProviderName)
						for _, entry := range m.config.AllModelNames() {
							marker := "  "
							if entry.Model == m.currentModelName {
								marker = "* "
							}
							fmt.Fprintf(&sb, "%s%s (provider: %s)\n", marker, entry.Model, entry.Provider)
						}
						msg := lipgloss.NewStyle().
							Foreground(lipgloss.Color("#626262")).
							Render(sb.String())
						m.AddDisplayContent(msg, "")
						return nil
					}

					modelName := args[0]
					if err := m.switchModel(modelName); err != nil {
						errMsg := lipgloss.NewStyle().
							Foreground(lipgloss.Color("#FF3333")).
							Render(fmt.Sprintf("Error switching model: %v", err))
						m.AddDisplayContent(errMsg, "")
						return nil
					}
					successMsg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#04B575")).
						Render(fmt.Sprintf("Switched to model: %s (provider: %s)", m.currentModelName, m.currentProviderName))
					m.AddDisplayContent(successMsg, "")
					return nil
				},
			},
		},
	}
}

func (c *CommandRegistry) GetCommand(name string) (*Command, bool) {
	cmd, exists := c.commands[name]
	return &cmd, exists
}

// FilteredNames returns sorted command names that have the given prefix.
func (c *CommandRegistry) FilteredNames(prefix string) []string {
	var names []string
	for name := range c.commands {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
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
  Shift+Tab  - Toggle Default/Plan mode
  Enter      - Send message
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear screen
  y/n        - Approve/Cancel tool execution (when prompted)

Shell:
  !<command> - Run a shell command directly (e.g. !ls, !git status)`)

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(sb.String())

	lines := strings.Split(helpText, "\n")
	for _, line := range lines {
		m.AddDisplayContent(line)
	}
	m.AddDisplayContent("")

	return nil
}
