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

	// Operation context for better error messages
	currentOperation string
	lastUserInput    string
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
					return m, tea.Printf("\n" + lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FF3333")).
						Render("✗ Tool execution cancelled\n"))
				}
			}
		}

	case StreamOutput:
		// Handle streaming output - print it using tea.Printf
		m.loading = false
		m.statusMsg = "Ready"

		// Check if we have pending tool calls
		if len(m.pendingToolCalls) > 0 {
			m.checkToolApproval()
			if m.waitingForApproval {
				return m, tea.Printf(msg.Content)
			}
			// Auto-approve and execute
			m.approvePendingTools()
			m.currentOperation = "Executing approved tools"
			return m, tea.Sequence(
				tea.Printf(msg.Content),
				m.executePendingTools(),
			)
		}

		// Agent finished processing - send notification
		m.sendCompletionNotification()
		return m, tea.Printf(msg.Content)

	case toolExecutionMsg:
		// Handle tool execution result
		m.handleToolResult(msg)

		// Print any output from tool execution
		cmds := []tea.Cmd{}
		if msg.output != "" {
			cmds = append(cmds, tea.Printf(msg.output))
		}

		if msg.hasMore {
			m.currentOperation = "Processing tool results"
			cmds = append(cmds, m.continueConversation())
		} else {
			// All processing done - send notification
			m.sendCompletionNotification()
		}

		return m, tea.Batch(cmds...)

	case errorMsg:
		m.err = msg.error
		m.loading = false
		m.statusMsg = "Error: " + msg.error.Error()

		// Provide more context for timeout errors
		errorMessage := msg.error.Error()
		if strings.Contains(errorMessage, "context deadline exceeded") ||
			strings.Contains(errorMessage, "Client.Timeout") {
			// Calculate timeout duration for display
			timeout := m.config.API.Timeout
			if timeout == 0 {
				timeout = 60 // default fallback
			}

			// Enhance timeout error message
			errorOutput := "\n" + lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333")).
				Bold(true).
				Render(fmt.Sprintf("✗ Request Timeout (after %v)", timeout)) + "\n\n"

			// Show what was being processed
			if m.currentOperation != "" {
				errorOutput += lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFA500")).
					Render("What was happening:") + "\n"

				errorOutput += lipgloss.NewStyle().
					Foreground(lipgloss.Color("#626262")).
					PaddingLeft(2).
					Render("• "+m.currentOperation) + "\n"

				if m.lastUserInput != "" && len(m.lastUserInput) < 100 {
					errorOutput += lipgloss.NewStyle().
						Foreground(lipgloss.Color("#626262")).
						PaddingLeft(2).
						Render("• Your message: \""+m.lastUserInput+"\"") + "\n"
				} else if m.lastUserInput != "" {
					errorOutput += lipgloss.NewStyle().
						Foreground(lipgloss.Color("#626262")).
						PaddingLeft(2).
						Render("• Your message: \""+m.lastUserInput[:97]+"...\"") + "\n"
				}
				errorOutput += "\n"
			}

			errorOutput += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500")).
				Render("Possible causes:") + "\n"

			errorOutput += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				PaddingLeft(2).
				Render("• The model is processing a complex request\n"+
					"  • Network connection is slow or unstable\n"+
					"  • API service is experiencing high load\n"+
					"  • Response size is very large\n") + "\n"

			errorOutput += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Render("Try:") + "\n"

			errorOutput += lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				PaddingLeft(2).
				Render("• Simplifying your request\n" +
					"  • Checking your internet connection\n" +
					"  • Waiting a moment and trying again\n")

			return m, tea.Printf(errorOutput)
		}

		// Print other errors normally
		errorOutput := "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true).
			Render("✗ Error: "+errorMessage) + "\n"
		return m, tea.Printf(errorOutput)

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
			Render("⚠ Tool execution requires approval. Press [y] to approve, [n] to cancel."))
		b.WriteString("\n")
	} else {
		// Show textarea with prompt
		b.WriteString("\n\n")
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
	case "/init":
		return m.initProject()
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
  /init     - Generate AGENTS.md for the project
  /exit     - Exit application

Keyboard Shortcuts:
  Ctrl+J     - Send message
  Alt+Enter  - Send message (alternative)
  Ctrl+C     - Cancel current operation or exit
  Ctrl+L     - Clear screen
  y/n        - Approve/Cancel tool execution (when prompted)
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

	return tea.Println("\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(configData))
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
	return m.sendContinuation()
}

// initProject generates AGENTS.md for the project
func (m *SimpleModel) initProject() tea.Cmd {
	// Get current directory as root path
	rootPath := "."

	// Generate AGENTS.md
	content, err := tools.GenerateAgentsMD(rootPath)
	if err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error generating AGENTS.md: %v", err))
		return tea.Printf("\n%s\n", errMsg)
	}

	// Write the file
	if err := os.WriteFile("AGENTS.md", []byte(content), 0644); err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error writing AGENTS.md: %v", err))
		return tea.Printf("\n%s\n", errMsg)
	}

	// Show success message and content preview
	output := "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("✓ Successfully generated AGENTS.md") + "\n\n"

	// Show first few lines of the content as preview
	lines := strings.Split(content, "\n")
	preview := strings.Join(lines[:min(20, len(lines))], "\n")
	if len(lines) > 20 {
		preview += "\n..."
	}

	output += lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(preview) + "\n"

	return tea.Printf(output)
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
