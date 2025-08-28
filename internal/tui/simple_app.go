package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	contextmgr "github.com/tokuhirom/ashron/internal/context"
	"github.com/tokuhirom/ashron/internal/tools"
)

// SimpleModel represents the simplified streaming application state
type SimpleModel struct {
	// Configuration
	config *config.Config

	// API client
	apiClient *api.Client

	// UI components
	textarea textarea.Model
	spinner  spinner.Model

	// Chat state
	session  *api.ChatSession
	messages []api.Message

	// Context manager
	contextMgr *contextmgr.Manager

	// Tool executor
	toolExec *tools.Executor

	// UI state
	ready     bool
	loading   bool
	err       error
	statusMsg string

	// Conversation state
	waitingForApproval bool
	pendingToolCalls   []api.ToolCall

	// Current streaming message
	currentMessage string
}

// NewSimpleModel creates a new simplified application model
func NewSimpleModel(cfg *config.Config) (*SimpleModel, error) {
	// Create API client
	apiClient := api.NewClient(&cfg.API)

	// Create context manager
	ctxMgr := contextmgr.NewManager(&cfg.Context)

	// Create tool executor
	toolExec := tools.NewExecutor(&cfg.Tools)

	// Create UI components
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Press Ctrl+J to send, /help for commands)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 10000
	ta.SetHeight(3)
	ta.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	// Initialize session
	session := &api.ChatSession{
		Messages: []api.Message{},
	}

	// Add system prompt
	systemPrompt := `You are Ashron, an AI coding assistant. You help users with programming tasks by:
- Writing and editing code
- Running commands
- Explaining concepts
- Debugging issues
- Suggesting improvements

You have access to tools for file operations and command execution. Always ask for approval before making changes unless the operation is pre-approved.`

	session.Messages = append(session.Messages, api.NewSystemMessage(systemPrompt))

	// Print welcome message
	fmt.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("🤖 Ashron - AI Coding Assistant"))
	fmt.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Type /help for available commands"))
	fmt.Println()

	return &SimpleModel{
		config:     cfg,
		apiClient:  apiClient,
		textarea:   ta,
		spinner:    sp,
		session:    session,
		messages:   session.Messages,
		contextMgr: ctxMgr,
		toolExec:   toolExec,
		statusMsg:  "Ready",
		ready:      true,
	}, nil
}

// Init initializes the model
func (m *SimpleModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// Update handles messages
func (m *SimpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Adjust textarea width
		m.textarea.SetWidth(msg.Width - 2)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.loading {
				m.loading = false
				m.statusMsg = "Request cancelled"
			} else {
				return m, tea.Quit
			}

		case tea.KeyCtrlL:
			// Clear chat (just clear screen in streaming mode)
			var b strings.Builder
			b.WriteString("\033[2J\033[H") // Clear screen and move cursor to top
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true).
				Render("🤖 Ashron - AI Coding Assistant") + "\n\n")
			m.statusMsg = "Screen cleared"
			return m, tea.Printf(b.String())

		case tea.KeyCtrlJ:
			// Send message
			input := m.textarea.Value()
			if strings.TrimSpace(input) != "" {
				// Check for commands
				if strings.HasPrefix(input, "/") {
					return m, m.handleCommand(input)
				}

				// Send chat message
				return m, m.sendMessage(input)
			}

		case tea.KeyEnter:
			// Check if Alt is pressed for sending (Alt+Enter)
			if msg.Alt {
				// Send message
				input := m.textarea.Value()
				if strings.TrimSpace(input) != "" {
					// Check for commands
					if strings.HasPrefix(input, "/") {
						return m, m.handleCommand(input)
					}

					// Send chat message
					return m, m.sendMessage(input)
				}
			} else {
				// Regular enter - let textarea handle it for newline
				m.textarea, tiCmd = m.textarea.Update(msg)
				return m, tiCmd
			}

		case tea.KeyTab:
			// Handle tool approval
			if m.waitingForApproval {
				m.approvePendingTools()
				return m, m.executePendingTools()
			}
		}

	case streamMsg:
		// Handle streaming response - print directly to stdout
		m.handleStreamMessage(msg)
		return m, waitForStream(m.apiClient)

	case streamCompleteMsg:
		// Stream completed
		m.loading = false
		m.statusMsg = "Ready"
		// Newlines are already added in processStream

	case toolExecutionMsg:
		// Handle tool execution result
		m.handleToolResult(msg)
		if !msg.hasMore {
			// Check for tool approval
			if len(m.pendingToolCalls) > 0 {
				m.checkToolApproval()
				if m.waitingForApproval {
					return m, nil
				}
				return m, m.executePendingTools()
			}
		} else if msg.hasMore {
			return m, m.continueConversation()
		}

	case errorMsg:
		m.err = msg.error
		m.loading = false
		m.statusMsg = "Error: " + msg.error.Error()
		// Print error directly
		fmt.Fprintln(os.Stderr, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render("✗ Error: "+msg.error.Error()))

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// Update components
	if !m.loading {
		m.textarea, tiCmd = m.textarea.Update(msg)
		cmds = append(cmds, tiCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders only the input area
func (m *SimpleModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// Only show input area and status
	if m.loading {
		// Show spinner during loading
		b.WriteString("\n" + m.spinner.View() + " Processing...\n")
	} else if m.waitingForApproval {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Render("⚠ Tools require approval. Press TAB to approve, ESC to cancel."))
		b.WriteString("\n")
	} else {
		// Show textarea with prompt
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true).
			Render("> "))
		b.WriteString(m.textarea.View())
	}

	return b.String()
}

// handleCommand processes slash commands
func (m *SimpleModel) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	m.textarea.SetValue("")
	switch parts[0] {
	case "/help":
		return m.showHelp()
	case "/clear":
		var b strings.Builder
		b.WriteString("\033[2J\033[H") // Clear screen and move cursor to top
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			Render("🤖 Ashron - AI Coding Assistant\n\n"))
		m.statusMsg = "Screen cleared"
		return tea.Printf(b.String())
	case "/compact":
		m.compactContext()
	case "/exit", "/quit":
		return tea.Quit
	case "/config":
		return m.showConfig()
	default:
		return tea.Println(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Unknown command: %s", parts[0])))
	}

	return nil
}

// showHelp displays available commands
func (m *SimpleModel) showHelp() tea.Cmd {
	help := `Available Commands:
  /help     - Show this help message
  /clear    - Clear screen
  /compact  - Compact conversation context
  /config   - Show current configuration
  /exit     - Exit application

Keyboard Shortcuts:
  Ctrl+J     - Send message
  Alt+Enter  - Send message (alternative)
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear screen
  Tab        - Approve pending tool calls
  Enter      - New line in input`

	return tea.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(help))
}

// showConfig displays current configuration
func (m *SimpleModel) showConfig() tea.Cmd {
	configData := fmt.Sprintf(`Current Configuration:
  Provider: %s
  Model: %s
  Max Tokens: %d
  Temperature: %.2f
  Auto-Compact: %v
  Context Limit: %d tokens`,
		m.config.Provider,
		m.config.API.Model,
		m.config.API.MaxTokens,
		m.config.API.Temperature,
		m.config.Context.AutoCompact,
		m.config.Context.MaxTokens,
	)

	return tea.Println("\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(configData))
}

// Helper functions for managing messages
func (m *SimpleModel) addUserMessage(content string) {
	m.messages = append(m.messages, api.NewUserMessage(content))
}

func (m *SimpleModel) addAssistantMessage(content string) {
	m.messages = append(m.messages, api.NewAssistantMessage(content))
}

func (m *SimpleModel) addSystemMessage(content string) {
	m.messages = append(m.messages, api.NewSystemMessage(content))
}

func (m *SimpleModel) addToolMessage(toolCallID, content string) {
	m.messages = append(m.messages, api.NewToolMessage(toolCallID, content))
}
