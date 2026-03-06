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
					m.resetDisplayHeader()
					m.viewport.GotoTop()
					return nil
				},
			},
			"/new": {
				Name:        "/new",
				Description: "Start a new chat session",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.StartNewSession()
				},
			},
			"/compact": {
				Name:        "/compact",
				Description: "Compact conversation context using LLM summarization",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					m.AddDisplayContent(lipgloss.NewStyle().
						Foreground(lipgloss.Color("#626262")).
						Render("Compacting context..."), "")
					return m.CompactContext()
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
					m.saveSession()
					return tea.Quit
				},
			},
			"/exit": {
				Name:        "/exit",
				Description: "Exit the application",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					m.saveSession()
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
			"/commands": {
				Name:        "/commands",
				Description: "List custom slash commands",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					return m.RenderCustomCommands()
				},
			},
			"/model": {
				Name:        "/model",
				Description: "Show or switch model. Usage: /model [name]",
				Body: func(cr *CommandRegistry, m *SimpleModel, args []string) tea.Cmd {
					if len(args) == 0 {
						m.textarea.SetValue("/model ")
						m.textarea.CursorEnd()
						m.updateCompletionState()
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

func (c *CommandRegistry) Register(cmd Command) bool {
	if _, exists := c.commands[cmd.Name]; exists {
		return false
	}
	c.commands[cmd.Name] = cmd
	return true
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
	var names []string
	for name := range cr.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cmd := cr.commands[name]
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
