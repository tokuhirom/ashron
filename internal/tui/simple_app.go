package tui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gen2brain/beeep"
	"github.com/tokuhirom/ashron/internal/agentsmd"

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
	height    int
	width     int

	// Conversation state
	waitingForApproval bool
	pendingToolCalls   []api.ToolCall

	// Current streaming message
	currentMessage string

	// Operation context for better error messages
	currentOperation string
	lastUserInput    string

	commandRegistry *CommandRegistry

	// Display content - stores all conversation output
	displayContent []string
	// Scroll offset for viewing conversation history
	scrollOffset int
}

// NewSimpleModel creates a new simplified application model
func NewSimpleModel(cfg *config.Config) (*SimpleModel, error) {
	// Create API client
	apiClient := api.NewClient(&cfg.API)

	// Create a context manager
	ctxMgr := contextmgr.NewManager(&cfg.Context)

	// Create tool executor
	toolExec := tools.NewExecutor(&cfg.Tools)

	// Create UI components
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Press Enter to send, /help for commands)"
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

	// Add welcome message to display content
	welcomeMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("🤖 Ashron - AI Coding Assistant")
	helpMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Type /help for available commands")

	commandRegistry := NewCommandRegistry()

	return &SimpleModel{
		config:          cfg,
		apiClient:       apiClient,
		textarea:        ta,
		spinner:         sp,
		session:         session,
		messages:        session.Messages,
		contextMgr:      ctxMgr,
		toolExec:        toolExec,
		statusMsg:       "Ready",
		ready:           true,
		commandRegistry: commandRegistry,
		displayContent:  []string{welcomeMsg, helpMsg, ""},
		scrollOffset:    0,
	}, nil
}

// Init initializes the model
func (m *SimpleModel) Init() tea.Cmd {
	m.ReadAgentsMD()
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

func (m *SimpleModel) ReadAgentsMD() {
	content := agentsmd.ReadAgentsMD()
	if content == "" {
		slog.Info("No AGENTS.md found in current or parent directories")
		warningMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			Render("No AGENTS.md found. I suggest to use '/init' command to generate AGENTS.md.")
		m.displayContent = append(m.displayContent, warningMsg)
		return
	}

	slog.Info("Add AGENTS.md content to session messages")
	m.session.Messages = append(m.session.Messages, api.NewSystemMessage(string(content)))
}

// Update handles messages
func (m *SimpleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Store window dimensions and adjust textarea width
		m.width = msg.Width
		m.height = msg.Height
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
			// Clear chat conversation
			welcomeMsg := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true).
				Render("🤖 Ashron - AI Coding Assistant")
			helpMsg := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				Render("Type /help for available commands")
			m.displayContent = []string{welcomeMsg, helpMsg, ""}
			m.scrollOffset = 0
			m.statusMsg = "Screen cleared"
			return m, nil

		case tea.KeyCtrlJ:
			m.textarea.InsertString("\n")
			return m, nil

		case tea.KeyEnter:
			// Send a message
			input := m.textarea.Value()
			if strings.TrimSpace(input) != "" {
				// Check for commands
				if strings.HasPrefix(input, "/") {
					return m, m.handleCommand(input)
				}

				// Send a chat message
				return m, m.SendMessage(input)
			}

		case tea.KeyRunes:
			// Handle tool approval with y/n
			if m.waitingForApproval {
				switch string(msg.Runes) {
				case "y", "Y":
					m.approvePendingTools()
					m.currentOperation = "Executing approved tools"
					return m, m.executePendingTools()
				case "n", "N":
					m.cancelPendingTools()
					cancelMsg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FF3333")).
						Render("✗ Tool execution cancelled")
					m.displayContent = append(m.displayContent, cancelMsg, "")
					return m, nil
				}
			}
		}

	case StreamOutput:
		// Handle streaming output - add to display content
		m.loading = false
		m.statusMsg = "Ready"

		// Add output to display content
		if msg.Content != "" {
			// Split content by lines and add to display
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				if line != "" {
					m.displayContent = append(m.displayContent, line)
				}
			}
		}

		// Check if we have pending tool calls
		if len(m.pendingToolCalls) > 0 {
			m.checkToolApproval()
			if m.waitingForApproval {
				return m, nil
			}
			// Auto-approve and execute
			m.approvePendingTools()
			m.currentOperation = "Executing approved tools"
			return m, m.executePendingTools()
		}

		// Agent finished processing - send notification
		m.sendCompletionNotification()
		return m, nil

	case toolExecutionMsg:
		// Handle tool execution result
		m.handleToolResult(msg)

		// Add any output from tool execution to display
		if msg.output != "" {
			lines := strings.Split(msg.output, "\n")
			for _, line := range lines {
				if line != "" {
					m.displayContent = append(m.displayContent, line)
				}
			}
		}

		if msg.hasMore {
			m.currentOperation = "Processing tool results"
			return m, m.continueConversation()
		} else {
			// All processing done - send notification
			m.sendCompletionNotification()
		}

		return m, nil

	case errorMsg:
		m.err = msg.error
		m.loading = false
		m.statusMsg = "Error: " + msg.error.Error()

		// Add error to display content
		errorMessage := msg.error.Error()
		errorDisplay := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true).
			Render("✗ Error: " + errorMessage)
		m.displayContent = append(m.displayContent, errorDisplay, "")
		return m, nil

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

// View renders the entire UI with conversation history and input area
func (m *SimpleModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// Calculate available height for content
	textareaHeight := 5 // Input area height including borders
	statusHeight := 2   // Status line height
	contentHeight := m.height - textareaHeight - statusHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render conversation history
	displayStart := m.scrollOffset
	displayEnd := displayStart + contentHeight
	if displayEnd > len(m.displayContent) {
		displayEnd = len(m.displayContent)
		displayStart = displayEnd - contentHeight
		if displayStart < 0 {
			displayStart = 0
		}
	}

	// Add visible content
	for i := displayStart; i < displayEnd && i < len(m.displayContent); i++ {
		b.WriteString(m.displayContent[i])
		b.WriteString("\n")
	}

	// Fill remaining space
	renderedLines := displayEnd - displayStart
	for i := renderedLines; i < contentHeight; i++ {
		b.WriteString("\n")
	}

	// Render status/input area
	if m.loading {
		// Show spinner during loading
		b.WriteString(m.spinner.View() + " Processing...\n")
	} else if m.waitingForApproval {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Render("⚠ Tool execution requires approval. Press [y] to approve, [n] to cancel."))
		b.WriteString("\n")
	} else {
		// Show textarea
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

	cmd, ok := m.commandRegistry.GetCommand(input)
	if !ok {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Unknown command: %s", parts[0]))
		m.displayContent = append(m.displayContent, errorMsg, "")
		return nil
	}

	return cmd.Body(m.commandRegistry, m, parts[1:])
}

// RenderConfig showConfig displays current configuration
func (m *SimpleModel) RenderConfig() tea.Cmd {
	configData := fmt.Sprintf(`Current Configuration:
  Provider: %s
  Model: %s
  Max Tokens: %d
  Temperature: %.2f
  API Timeout: %v
  Auto-Compact: %v
  Context Limit: %d tokens`,
		m.config.Provider,
		m.config.API.Model,
		m.config.API.MaxTokens,
		m.config.API.Temperature,
		m.config.API.Timeout,
		m.config.Context.AutoCompact,
		m.config.Context.MaxTokens,
	)

	configDisplay := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(configData)

	// Add config to display content
	lines := strings.Split(configDisplay, "\n")
	for _, line := range lines {
		m.displayContent = append(m.displayContent, line)
	}
	m.displayContent = append(m.displayContent, "")

	return nil
}

// Helper functions for managing messages
func (m *SimpleModel) addUserMessage(content string) {
	m.messages = append(m.messages, api.NewUserMessage(content))
}

// checkToolApproval checks if tools need approval
func (m *SimpleModel) checkToolApproval() {
	needsApproval := false
	for _, tc := range m.pendingToolCalls {
		if !m.isAutoApproved(tc.Function.Name) {
			needsApproval = true
			break
		}
	}

	if needsApproval {
		m.waitingForApproval = true
		m.loading = false
		m.statusMsg = "Tools require approval. Press TAB to approve."
	} else {
		m.approvePendingTools()
	}
}

// isAutoApproved checks if a tool is auto-approved
func (m *SimpleModel) isAutoApproved(toolName string) bool {
	for _, approved := range m.config.Tools.AutoApprove {
		if approved == toolName {
			return true
		}
	}
	return false
}

// approvePendingTools approves pending tool calls
func (m *SimpleModel) approvePendingTools() {
	m.waitingForApproval = false
	m.loading = true
	m.statusMsg = "Executing tools..."
}

// cancelPendingTools cancels pending tool calls
func (m *SimpleModel) cancelPendingTools() {
	m.waitingForApproval = false
	m.pendingToolCalls = nil
	m.statusMsg = "Tool execution cancelled"
}

// handleToolResult processes tool execution results
func (m *SimpleModel) handleToolResult(msg toolExecutionMsg) {
	if msg.hasMore {
		m.statusMsg = "Processing tool results..."
	}
}

// continueConversation continues after tool execution
func (m *SimpleModel) continueConversation() tea.Cmd {
	return m.processMessage()
}

// InitProject generates AGENTS.md for the project
func (m *SimpleModel) InitProject() tea.Cmd {
	// Get current directory as root path
	rootPath := "."

	// Generate AGENTS.md
	content, err := tools.GenerateAgentsMD(rootPath)
	if err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error generating AGENTS.md: %v", err))
		m.displayContent = append(m.displayContent, errMsg, "")
		return nil
	}

	// Write the file
	if err := os.WriteFile("AGENTS.md", []byte(content), 0644); err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error writing AGENTS.md: %v", err))
		m.displayContent = append(m.displayContent, errMsg, "")
		return nil
	}

	// Show success message
	successMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("✓ Successfully generated AGENTS.md")
	m.displayContent = append(m.displayContent, successMsg, "")

	// Show first few lines of the content as preview
	lines := strings.Split(content, "\n")
	preview := strings.Join(lines[:min(20, len(lines))], "\n")
	if len(lines) > 20 {
		preview += "\n..."
	}

	previewDisplay := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(preview)

	previewLines := strings.Split(previewDisplay, "\n")
	for _, line := range previewLines {
		m.displayContent = append(m.displayContent, line)
	}
	m.displayContent = append(m.displayContent, "")

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sendCompletionNotification sends a notification when the agent finishes processing
func (m *SimpleModel) sendCompletionNotification() {
	// Get the last assistant message for context
	var lastMessage string
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" && m.messages[i].Content != "" {
			lastMessage = m.messages[i].Content
			break
		}
	}

	// Create notification
	title := "Ashron Ready"
	msg := "Your assistant has finished processing"

	// Add a preview of the response if available
	if lastMessage != "" {
		// Truncate the message for notification
		preview := lastMessage
		if len(preview) > 100 {
			preview = preview[:97] + "..."
		}
		// Remove newlines for cleaner notification
		preview = strings.ReplaceAll(preview, "\n", " ")
		msg = preview
	}

	// Send the notification
	if err := beeep.Notify(title, msg, ""); err != nil {
		slog.Debug("Failed to send completion notification", "error", err)
	}
}

func (m *SimpleModel) CompactContext() (int, int) {
	originalCount := len(m.messages)
	m.messages = m.contextMgr.Compact(m.messages)
	newCount := len(m.messages)
	return originalCount, newCount
}
