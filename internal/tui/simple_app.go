package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gen2brain/beeep"
	"github.com/tokuhirom/ashron/internal/agentsmd"
	"github.com/tokuhirom/ashron/internal/skills"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	contextmgr "github.com/tokuhirom/ashron/internal/context"
	"github.com/tokuhirom/ashron/internal/session"
	"github.com/tokuhirom/ashron/internal/tools"
)

// SimpleModel represents the simplified streaming application state
type SimpleModel struct {
	// Configuration
	config              *config.Config
	currentProviderName string
	currentModelName    string

	// API client
	apiClient *api.Client

	// UI components
	textarea textarea.Model
	spinner  spinner.Model
	viewport viewport.Model

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

	// Command completion state
	showCompletion  bool
	completionIndex int

	// Display content - stores all conversation output
	displayContent []string

	// Token usage tracking
	currentUsage *api.Usage

	availableSkills []skills.Skill

	// Session persistence
	sess     *session.Session
	isResume bool

	// Cancel function for the current API call
	cancelAPICall context.CancelFunc
}

// NewSimpleModel creates a new simplified application model.
// Pass a non-nil sess to resume an existing session.
func NewSimpleModel(cfg *config.Config, sess *session.Session) (*SimpleModel, error) {
	provName, provCfg, err := cfg.ActiveProvider()
	if err != nil {
		return nil, err
	}
	modelName, modelCfg, err := cfg.ActiveModel()
	if err != nil {
		return nil, err
	}

	// Create API client
	apiClient := api.NewClient(provCfg, modelCfg, &cfg.Context)

	// Create a context manager
	ctxMgr := contextmgr.NewManager(&cfg.Context)

	// Create tool executor
	toolExec := tools.NewExecutor(&cfg.Tools)
	tools.ConfigureSubagentRuntime(apiClient, &cfg.Context)

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

	// Create viewport for conversation history
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.MouseWheelEnabled = true

	isResume := sess != nil
	availableSkills := skills.Discover()

	// Initialize chat session
	chatSession := &api.ChatSession{Messages: []api.Message{}}

	var messages []api.Message
	if isResume {
		messages = sess.Messages
	} else {
		sess = session.New(provName, modelName)

		// Add system prompt for new sessions
		systemPrompt := `You are Ashron, an AI coding assistant. You help users with programming tasks by:
- Writing and editing code
- Running commands
- Explaining concepts
- Debugging issues
- Suggesting improvements

You have access to tools for file operations and command execution. Always ask for approval before making changes unless the operation is pre-approved.`
		if skillsPrompt := skills.MetadataPrompt(availableSkills); skillsPrompt != "" {
			systemPrompt += "\n\n" + skillsPrompt
		}
		messages = append(messages, api.NewSystemMessage(systemPrompt))
	}
	chatSession.Messages = messages

	// Build initial display content
	welcomeMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("🤖 Ashron - AI Coding Assistant")
	helpMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Type /help for available commands")

	initDisplay := []string{welcomeMsg, helpMsg}
	if cfg.Tools.Yolo {
		initDisplay = append(initDisplay, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true).
			Render("YOLO MODE ENABLED: sandbox disabled and tools auto-approved"))
	}
	initDisplay = append(initDisplay, "")

	commandRegistry := NewCommandRegistry()

	m := &SimpleModel{
		config:              cfg,
		currentProviderName: provName,
		currentModelName:    modelName,
		apiClient:           apiClient,
		textarea:            ta,
		spinner:             sp,
		viewport:            vp,
		session:             chatSession,
		messages:            messages,
		contextMgr:          ctxMgr,
		toolExec:            toolExec,
		statusMsg:           "Ready",
		ready:               true,
		commandRegistry:     commandRegistry,
		availableSkills:     availableSkills,
		displayContent:      initDisplay,
		sess:                sess,
		isResume:            isResume,
	}

	if isResume {
		m.restoreSessionDisplay()
	}

	return m, nil
}

// SessionID returns the current session ID.
func (m *SimpleModel) SessionID() string {
	if m.sess == nil {
		return ""
	}
	return m.sess.ID
}

// saveSession updates the session messages and writes to disk.
func (m *SimpleModel) saveSession() {
	if m.sess == nil {
		return
	}
	m.sess.Messages = m.messages
	if err := m.sess.Save(); err != nil {
		slog.Warn("Failed to save session", "error", err)
	}
}

// restoreSessionDisplay renders past messages into displayContent.
func (m *SimpleModel) restoreSessionDisplay() {
	resumeHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Italic(true).
		Render("── Resuming session " + m.sess.ID + " ──")
	m.displayContent = append(m.displayContent, resumeHeader, "")

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			if msg.Content == "" {
				continue
			}
			line := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Bold(true).
				Render("You: ") + msg.Content
			m.displayContent = append(m.displayContent, line, "")
		case "assistant":
			if msg.Content == "" {
				continue
			}
			label := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Render("Ashron: ")
			for i, line := range strings.Split(msg.Content, "\n") {
				if i == 0 {
					m.displayContent = append(m.displayContent, label+line)
				} else {
					m.displayContent = append(m.displayContent, line)
				}
			}
			m.displayContent = append(m.displayContent, "")
		}
	}

	resumeFooter := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Italic(true).
		Render("── End of resumed session ──")
	m.displayContent = append(m.displayContent, resumeFooter, "")
}

// switchModel switches to a named model (searching all providers).
func (m *SimpleModel) switchModel(modelName string) error {
	provName, provCfg, modelCfg, err := m.config.FindModel(modelName)
	if err != nil {
		return err
	}
	m.config.Default.Provider = provName
	m.config.Default.Model = modelName
	m.currentProviderName = provName
	m.currentModelName = modelName
	m.apiClient = api.NewClient(provCfg, modelCfg, &m.config.Context)
	tools.ConfigureSubagentRuntime(m.apiClient, &m.config.Context)
	return nil
}

// Init initializes the model
func (m *SimpleModel) Init() tea.Cmd {
	m.ReadAgentsMD()
	// Initialize viewport content
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

func (m *SimpleModel) ReadAgentsMD() {
	// On resume the AGENTS.md system message is already in the conversation history.
	if m.isResume {
		return
	}

	content := agentsmd.ReadAgentsMD()
	if content == "" {
		slog.Info("No AGENTS.md found in current or parent directories")
		warningMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			Render("No AGENTS.md found. I suggest to use '/init' command to generate AGENTS.md.")
		m.AddDisplayContent(warningMsg)
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
		m.viewport.SetWidth(msg.Width - 2)
		m.viewport.SetHeight(msg.Height - 8)

	case tea.MouseMsg:
		slog.Info("Mouse event",
			slog.String("event", msg.String()))
		_, cmd := m.viewport.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		// Ctrl+key shortcuts
		if msg.Mod.Contains(tea.ModCtrl) {
			switch msg.Code {
			case 'p':
				m.viewport.ScrollUp(1)
				return m, nil
			case 'n':
				m.viewport.ScrollDown(1)
				return m, nil
			case 'c':
				if m.loading {
					m.cancelCurrentRequest()
				} else {
					return m, tea.Quit
				}
			case 'l':
				m.viewport.GotoBottom()
				m.statusMsg = "Screen cleared"
				return m, nil
			case 'j':
				m.textarea.InsertString("\n")
				return m, nil
			}
		}

		// Escape cancels the current API request when loading
		if msg.Code == tea.KeyEsc && m.loading {
			m.cancelCurrentRequest()
			return m, nil
		}

		// Intercept navigation keys when completion popup is visible
		if m.showCompletion && !m.loading {
			items := m.completionItems()
			switch msg.Code {
			case tea.KeyUp:
				if m.completionIndex > 0 {
					m.completionIndex--
				}
				return m, nil
			case tea.KeyDown:
				if m.completionIndex < len(items)-1 {
					m.completionIndex++
				}
				return m, nil
			case tea.KeyTab:
				if len(items) > 0 {
					prefix := m.completionArgPrefix()
					m.textarea.SetValue(prefix + items[m.completionIndex])
					m.textarea.CursorEnd()
					m.showCompletion = false
				}
				return m, nil
			case tea.KeyEsc:
				m.showCompletion = false
				return m, nil
			case tea.KeyEnter:
				if len(items) > 0 {
					prefix := m.completionArgPrefix()
					selected := prefix + items[m.completionIndex]
					m.showCompletion = false
					return m, m.handleCommand(selected)
				}
			}
		}

		switch msg.Code {
		case tea.KeyEnter:
			// Send a message
			input := m.textarea.Value()
			if strings.TrimSpace(input) != "" {
				// Check for commands
				if strings.HasPrefix(input, "/") {
					return m, m.handleCommand(input)
				}
				return m, m.SendMessage(input)
			}
		default:
			// Handle tool approval with y/n
			if m.waitingForApproval && msg.Text != "" {
				switch msg.Text {
				case "y", "Y":
					m.approvePendingTools()
					m.currentOperation = "Executing approved tools"
					return m, m.executePendingTools()
				case "n", "N":
					m.cancelPendingTools()
					cancelMsg := lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FF3333")).
						Render("✗ Tool execution cancelled")
					m.AddDisplayContent(cancelMsg, "")
					return m, nil
				}
			}
		}

	case StreamOutput:
		// Handle streaming output - add to display content
		m.loading = false
		m.statusMsg = "Ready"

		// Update token usage if provided
		if msg.Usage != nil {
			m.currentUsage = msg.Usage
		}

		// Add output to display content
		if msg.Content != "" {
			// Split content by lines and add to display
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				if line != "" {
					m.AddDisplayContent(line)
				}
			}
		}

		// Check if we have pending tool calls
		if len(m.pendingToolCalls) > 0 {
			m.checkToolApproval()
			if m.waitingForApproval {
				m.viewport.GotoBottom() // tweaks
				return m, nil
			}
			// Auto-approve and execute
			m.approvePendingTools()
			m.currentOperation = "Executing approved tools"
			return m, m.executePendingTools()
		}

		// Auto-save session after assistant response
		m.saveSession()

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
					m.AddDisplayContent(line)
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
		width := m.width - 4
		if width < 40 {
			width = 40
		}
		errorDisplay := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true).
			Width(width).
			Render("✗ Error: " + errorMessage)
		lines := strings.Split(errorDisplay, "\n")
		for _, line := range lines {
			m.AddDisplayContent(line)
		}
		m.AddDisplayContent("")
		return m, nil

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		return m, spinCmd
	}

	// Update components
	if !m.loading {
		m.textarea, tiCmd = m.textarea.Update(msg)
		cmds = append(cmds, tiCmd)
		m.updateCompletionState()
	}

	return m, tea.Batch(cmds...)
}

// View renders the entire UI with conversation history and input area
func (m *SimpleModel) View() tea.View {
	if !m.ready {
		v := tea.NewView("\n  Initializing...")
		v.AltScreen = true
		return v
	}

	footer := m.renderFooter()
	completion := m.renderCompletion()

	m.viewport.SetHeight(m.height - lipgloss.Height(footer) - lipgloss.Height(completion) - 3)
	m.updateViewportContent()
	viewportContent := m.viewport.View() + "\n\n"

	slog.Info("Viewport content",
		slog.Int("YOffset", m.viewport.YOffset()))

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		viewportContent,
		completion,
		footer)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// completionItems returns the current list of completion candidates based on textarea input.
func (m *SimpleModel) completionItems() []string {
	input := m.textarea.Value()
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	spaceIdx := strings.Index(input, " ")
	if spaceIdx == -1 {
		// No space yet: complete command names
		return m.commandRegistry.FilteredNames(input)
	}
	// After a space: complete arguments for the command
	cmd := input[:spaceIdx]
	argPrefix := input[spaceIdx+1:]
	switch cmd {
	case "/model":
		return m.filteredModelNames(argPrefix)
	}
	return nil
}

// completionArgPrefix returns the prefix to prepend when inserting a completion item.
// For command completion it's empty (the item is the full value).
// For argument completion it's the command + space (e.g. "/model ").
func (m *SimpleModel) completionArgPrefix() string {
	input := m.textarea.Value()
	spaceIdx := strings.Index(input, " ")
	if spaceIdx == -1 {
		return ""
	}
	return input[:spaceIdx+1]
}

// filteredModelNames returns model names (across all providers) that match the given prefix.
func (m *SimpleModel) filteredModelNames(prefix string) []string {
	var names []string
	for _, entry := range m.config.AllModelNames() {
		if strings.HasPrefix(entry.Model, prefix) {
			names = append(names, entry.Model)
		}
	}
	sort.Strings(names)
	return names
}

// updateCompletionState updates the completion popup visibility based on textarea content.
func (m *SimpleModel) updateCompletionState() {
	items := m.completionItems()
	m.showCompletion = len(items) > 0
	if m.completionIndex >= len(items) {
		m.completionIndex = max(0, len(items)-1)
	}
}

// renderCompletion renders the command completion popup.
func (m *SimpleModel) renderCompletion() string {
	if !m.showCompletion {
		return ""
	}
	items := m.completionItems()
	if len(items) == 0 {
		return ""
	}

	isArgMode := strings.Contains(m.textarea.Value(), " ")

	var sb strings.Builder
	for i, item := range items {
		var line string
		if isArgMode {
			line = item
		} else {
			cmd, _ := m.commandRegistry.GetCommand(item)
			line = fmt.Sprintf("%-12s %s", item, cmd.Description)
		}
		if i == m.completionIndex {
			sb.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Render(line))
		} else {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CCCCCC")).
				Padding(0, 1).
				Render(line))
		}
		sb.WriteString("\n")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Render(strings.TrimRight(sb.String(), "\n"))
}

func (m *SimpleModel) renderFooter() string {
	var b strings.Builder
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
		b.WriteString(m.textarea.View())
	}

	// Display token usage if available
	if m.currentUsage != nil {
		b.WriteString("\n")
		usageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Italic(true)

		usageText := fmt.Sprintf("📊 Tokens: Prompt: %d | Completion: %d | Total: %d",
			m.currentUsage.PromptTokens,
			m.currentUsage.CompletionTokens,
			m.currentUsage.TotalTokens)

		b.WriteString(usageStyle.Render(usageText))
	}

	return b.String()
}

// updateViewportContent updates the viewport with current display content
func (m *SimpleModel) updateViewportContent() {
	// Build content with current streaming message if any
	displayWithCurrent := make([]string, len(m.displayContent))
	copy(displayWithCurrent, m.displayContent)

	// Join all content and set it in viewport
	content := strings.Join(displayWithCurrent, "\n")
	m.viewport.SetContent(content)
}

// handleCommand processes slash commands
func (m *SimpleModel) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	m.textarea.SetValue("")

	cmd, ok := m.commandRegistry.GetCommand(parts[0])
	if !ok {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Unknown command: %s", parts[0]))
		m.AddDisplayContent(errorMsg, "")
		return nil
	}

	return cmd.Body(m.commandRegistry, m, parts[1:])
}

// RenderConfig showConfig displays current configuration
func (m *SimpleModel) RenderConfig() tea.Cmd {
	_, provCfg, _ := m.config.ActiveProvider()
	_, modelCfg, _ := m.config.ActiveModel()

	var temperature float32
	var modelStr, timeout string
	if modelCfg != nil {
		temperature = modelCfg.Temperature
		modelStr = modelCfg.Model
	}
	if provCfg != nil {
		timeout = provCfg.Timeout.String()
	}

	configData := fmt.Sprintf(`Current Configuration:
  Provider: %s
  Model alias: %s (%s)
  Temperature: %.2f
  API Timeout: %s
  Max Tokens: %d
  Auto-Compact: %v
  Sandbox Mode: %s
  YOLO Mode: %v`,
		m.currentProviderName,
		m.currentModelName,
		modelStr,
		temperature,
		timeout,
		m.config.Context.MaxTokens,
		m.config.Context.AutoCompact,
		m.config.Tools.SandboxMode,
		m.config.Tools.Yolo,
	)

	configDisplay := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(configData)

	// Add config to display content
	lines := strings.Split(configDisplay, "\n")
	for _, line := range lines {
		m.AddDisplayContent(line)
	}
	m.AddDisplayContent("")

	return nil
}

func (m *SimpleModel) RenderSkills() tea.Cmd {
	if len(m.availableSkills) == 0 {
		msg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Render("No local skills found. Looked under $XDG_CONFIG_HOME/ashron/skills and ~/.config/ashron/skills.")
		m.AddDisplayContent(msg, "")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Available Skills:\n")
	for _, skill := range m.availableSkills {
		if skill.Description != "" {
			fmt.Fprintf(&sb, "  - %s: %s\n    %s\n", skill.Name, skill.Description, skill.Path)
		} else {
			fmt.Fprintf(&sb, "  - %s\n    %s\n", skill.Name, skill.Path)
		}
	}

	msg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(sb.String())
	for _, line := range strings.Split(msg, "\n") {
		m.AddDisplayContent(line)
	}
	m.AddDisplayContent("")
	return nil
}

// Helper functions for managing messages
func (m *SimpleModel) addUserMessage(content string) {
	m.messages = append(m.messages, api.NewUserMessage(content))
}

// checkToolApproval checks if tools need approval
func (m *SimpleModel) checkToolApproval() {
	if m.config.Tools.Yolo {
		m.approvePendingTools()
		return
	}

	needsApproval := false
	for _, tc := range m.pendingToolCalls {
		if !m.isAutoApproved(tc.Function.Name, tc.Function.Arguments) {
			needsApproval = true
			break
		}
	}

	if needsApproval {
		m.waitingForApproval = true
		m.loading = false
		m.statusMsg = "Tools require approval. Press [y] to approve."
	} else {
		m.approvePendingTools()
	}
}

// isAutoApproved checks if a tool is auto-approved
func (m *SimpleModel) isAutoApproved(toolName string, arguments string) bool {
	if m.config.Tools.Yolo {
		return true
	}

	if toolName == "execute_command" {
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			slog.Error("Failed to parse tool arguments",
				slog.Any("error", err),
				slog.Any("arguments", arguments))
			return false
		}

		// Never auto-approve unsandboxed command execution.
		if strings.EqualFold(tools.EffectiveSandboxMode(&m.config.Tools, args), "off") {
			return false
		}
	}

	for _, approved := range m.config.Tools.AutoApproveTools {
		if approved == toolName {
			return true
		}
	}
	if toolName == "execute_command" {
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			slog.Error("Failed to parse tool arguments",
				slog.Any("error", err),
				slog.Any("arguments", arguments))
			return false
		}

		for _, cmd := range m.config.Tools.AutoApproveCommands {
			if strings.HasPrefix(cmd, "/") && strings.HasSuffix(cmd, "/") {
				// match '/git add .*/' style. regexp match is required.
				pattern := strings.TrimPrefix(strings.TrimSuffix(cmd, "/"), "/")
				matched, err := regexp.MatchString(pattern, args.Command)
				if err != nil {
					slog.Error("Invalid regex in auto-approve command",
						slog.Any("error", err),
						slog.String("pattern", pattern))
					continue
				}
				if matched {
					slog.Debug("Command matched auto-approve command(regexp)",
						slog.String("pattern", pattern),
						slog.String("command", args.Command))
					return true
				}
			} else {
				if cmd == args.Command {
					slog.Debug("Command matched auto-approve command(strict)",
						slog.String("pattern", cmd),
						slog.String("command", args.Command))
					return true
				}
			}
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
	content, err := agentsmd.GenerateAgentsMD(rootPath)
	if err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error generating AGENTS.md: %v", err))
		m.AddDisplayContent(errMsg, "")
		return nil
	}

	// Write the file
	if err := os.WriteFile("AGENTS.md", []byte(content), 0644); err != nil {
		errMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Error writing AGENTS.md: %v", err))
		m.AddDisplayContent(errMsg, "")
		return nil
	}

	// Show success message
	successMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("✓ Successfully generated AGENTS.md")
	m.AddDisplayContent(successMsg, "")

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
	m.displayContent = append(m.displayContent, previewLines...)
	m.displayContent = append(m.displayContent, "")
	m.viewport.GotoBottom()

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

// AddDisplayContent appends content to displayContent and automatically scrolls to bottom
func (m *SimpleModel) AddDisplayContent(content ...string) {
	m.displayContent = append(m.displayContent, content...)
	m.viewport.GotoBottom()
}

func (m *SimpleModel) CompactContext() (int, int) {
	originalCount := len(m.messages)
	m.messages = m.contextMgr.Compact(m.messages)
	newCount := len(m.messages)
	return originalCount, newCount
}
