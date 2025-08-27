package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	contextmgr "github.com/tokuhirom/ashron/internal/context"
	"github.com/tokuhirom/ashron/internal/tools"
)

// Model represents the application state
type Model struct {
	// Configuration
	config *config.Config

	// API client
	apiClient *api.Client

	// UI components
	viewport viewport.Model
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
	width     int
	height    int
	ready     bool
	loading   bool
	err       error
	statusMsg string

	// Conversation state
	waitingForApproval bool
	pendingToolCalls   []api.ToolCall
}

// NewModel creates a new application model
func NewModel(cfg *config.Config) (*Model, error) {
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

	vp := viewport.New(0, 0)

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

	return &Model{
		config:     cfg,
		apiClient:  apiClient,
		viewport:   vp,
		textarea:   ta,
		spinner:    sp,
		session:    session,
		messages:   session.Messages,
		contextMgr: ctxMgr,
		toolExec:   toolExec,
		statusMsg:  "Ready. Type /help for available commands.",
	}, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width-2, msg.Height-10)
			m.viewport.YPosition = 1
			m.textarea.SetWidth(msg.Width - 2)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 2
			m.viewport.Height = msg.Height - 10
			m.textarea.SetWidth(msg.Width - 2)
		}

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
			// Clear chat
			m.messages = []api.Message{m.messages[0]} // Keep system message
			m.viewport.SetContent("")
			m.statusMsg = "Chat cleared"

		case tea.KeyCtrlJ:
			// Use Ctrl+J to send
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
		// Handle streaming response
		m.handleStreamMessage(msg)
		return m, waitForStream(m.apiClient)

	case streamCompleteMsg:
		// Stream completed
		m.loading = false
		m.statusMsg = "Ready"
		m.updateViewport()

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
		m.updateViewport()

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

	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View renders the interface
func (m *Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// Header
	b.WriteString(RenderHeader("🤖 Ashron - AI Coding Assistant"))
	b.WriteString("\n")

	// Chat viewport
	chatContent := m.viewport.View()
	b.WriteString(chatViewStyle.Width(m.width - 2).Height(m.height - 10).Render(chatContent))
	b.WriteString("\n")

	// Input area
	if m.loading {
		b.WriteString(m.spinner.View() + " Processing...")
	} else if m.waitingForApproval {
		b.WriteString(warningStyle.Render("⚠ Tools require approval. Press TAB to approve, ESC to cancel."))
	} else {
		b.WriteString(m.textarea.View())
	}
	b.WriteString("\n")

	// Status bar
	status := m.statusMsg
	if m.contextMgr != nil {
		usage := m.contextMgr.GetTokenUsage(m.messages)
		status = fmt.Sprintf("%s | Tokens: %d/%d | Messages: %d",
			status, usage, m.config.Context.MaxTokens, len(m.messages))
	}
	b.WriteString(RenderStatusBar(status))

	return b.String()
}

// handleCommand processes slash commands
func (m *Model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "/help":
		m.showHelp()
	case "/clear":
		m.messages = []api.Message{m.messages[0]}
		m.viewport.SetContent("")
		m.statusMsg = "Chat cleared"
	case "/compact":
		m.compactContext()
	case "/exit", "/quit":
		return tea.Quit
	case "/config":
		m.showConfig()
	default:
		m.statusMsg = fmt.Sprintf("Unknown command: %s", parts[0])
	}

	m.textarea.SetValue("")
	return nil
}

// showHelp displays available commands
func (m *Model) showHelp() {
	help := `
Available Commands:
  /help     - Show this help message
  /clear    - Clear chat history
  /compact  - Compact conversation context
  /config   - Show current configuration
  /exit     - Exit application

Keyboard Shortcuts:
  Ctrl+J     - Send message
  Alt+Enter  - Send message (alternative)
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear chat
  Tab        - Approve pending tool calls
  Esc        - Cancel pending tool calls
  Enter      - New line in input
`
	m.addSystemMessage(help)
	m.updateViewport()
}

// showConfig displays current configuration
func (m *Model) showConfig() {
	config := fmt.Sprintf(`
Current Configuration:
  Provider: %s
  Model: %s
  Max Tokens: %d
  Temperature: %.2f
  Auto-Compact: %v
  Context Limit: %d tokens
`,
		m.config.Provider,
		m.config.API.Model,
		m.config.API.MaxTokens,
		m.config.API.Temperature,
		m.config.Context.AutoCompact,
		m.config.Context.MaxTokens,
	)
	m.addSystemMessage(config)
	m.updateViewport()
}

// updateViewport updates the chat viewport with current messages
func (m *Model) updateViewport() {
	var content strings.Builder

	for _, msg := range m.messages[1:] { // Skip system message
		switch msg.Role {
		case "user":
			content.WriteString(userMessageStyle.Render("You: " + msg.Content))
		case "assistant":
			content.WriteString(assistantMessageStyle.Render("Ashron: " + msg.Content))
		case "system":
			content.WriteString(systemMessageStyle.Render("System: " + msg.Content))
		case "tool":
			content.WriteString(toolResultStyle.Render("Tool Result: " + msg.Content))
		}
		content.WriteString("\n")

		// Show tool calls
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				// Format the tool call more clearly
				toolMsg := fmt.Sprintf("🔧 Calling %s", tc.Function.Name)
				if tc.Function.Arguments != "" && tc.Function.Arguments != "{}" {
					// Try to format JSON arguments nicely, but don't fail if invalid
					toolMsg += fmt.Sprintf("\n   Arguments: %s", tc.Function.Arguments)
				}
				content.WriteString(toolCallStyle.Render(toolMsg))
				content.WriteString("\n")
			}
		}
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// Helper functions for managing messages
func (m *Model) addUserMessage(content string) {
	m.messages = append(m.messages, api.NewUserMessage(content))
}

func (m *Model) addAssistantMessage(content string) {
	m.messages = append(m.messages, api.NewAssistantMessage(content))
}

func (m *Model) addSystemMessage(content string) {
	m.messages = append(m.messages, api.NewSystemMessage(content))
}

func (m *Model) addToolMessage(toolCallID, content string) {
	m.messages = append(m.messages, api.NewToolMessage(toolCallID, content))
}
