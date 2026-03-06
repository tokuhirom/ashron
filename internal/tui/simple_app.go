package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gen2brain/beeep"

	"github.com/tokuhirom/ashron/internal/agentsmd"
	"github.com/tokuhirom/ashron/internal/customcmd"
	"github.com/tokuhirom/ashron/internal/plan"
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
	showApprovalDetail bool

	// Current streaming message
	currentMessage string

	// Operation context for better error messages
	currentOperation   string
	operationStartedAt time.Time
	lastUserInput      string

	commandRegistry *CommandRegistry

	// Command completion state
	showCompletion  bool
	completionIndex int

	// Display content - stores all conversation output
	displayContent []string

	// Token usage tracking
	currentUsage *api.Usage

	availableSkills         []skills.Skill
	availableCustomCommands []customcmd.Command

	collaborationMode string

	// Session persistence
	sess     *session.Session
	isResume bool

	// Cancel function for the current API call
	cancelAPICall context.CancelFunc

	// Live subagent summaries for display, updated by a periodic tick
	subagentSummary []tools.SubagentSummary

	// scrolledToBottom is set to true once we have scrolled to the bottom after resume.
	scrolledToBottom bool

	// viewportDirty tracks whether displayContent has changed since the last SetContent call.
	viewportDirty bool
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
	ta.Prompt = "❯ "
	taStyles := ta.Styles()
	taStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(primaryColor)
	ta.SetStyles(taStyles)
	ta.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	// Create viewport for conversation history
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.MouseWheelEnabled = true

	isResume := sess != nil
	availableSkills := skills.Discover()
	availableCustomCommands := customcmd.Discover()

	// Initialize chat session
	chatSession := &api.ChatSession{Messages: []api.Message{}}

	var messages []api.Message
	if isResume {
		messages = sess.Messages
	} else {
		sess = session.New(provName, modelName)
		messages = initialMessagesForNewSession(availableSkills)
	}
	chatSession.Messages = messages

	initDisplay := buildHeaderLines(cfg.Tools.Yolo)

	commandRegistry := NewCommandRegistry()

	m := &SimpleModel{
		config:                  cfg,
		currentProviderName:     provName,
		currentModelName:        modelName,
		apiClient:               apiClient,
		textarea:                ta,
		spinner:                 sp,
		viewport:                vp,
		session:                 chatSession,
		messages:                messages,
		contextMgr:              ctxMgr,
		toolExec:                toolExec,
		statusMsg:               "Ready",
		ready:                   true,
		commandRegistry:         commandRegistry,
		availableSkills:         availableSkills,
		availableCustomCommands: availableCustomCommands,
		collaborationMode:       "default",
		displayContent:          initDisplay,
		sess:                    sess,
		isResume:                isResume,
	}

	if isResume {
		m.restoreSessionDisplay()
	}
	m.registerCustomCommands()

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
			displayInput := compactUserInputForDisplay(msg.Content)
			line := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Bold(true).
				Render("You: ") + displayInput
			m.displayContent = append(m.displayContent, line, "")
		case "assistant":
			if msg.Content == "" {
				continue
			}
			label := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Render("Ashron: ")
			rendered := renderMarkdown(msg.Content, m.width)
			for i, line := range strings.Split(rendered, "\n") {
				if i == 0 {
					m.displayContent = append(m.displayContent, label+line)
				} else if line != "" {
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
	m.viewportDirty = true
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

// subagentTickMsg is sent by the periodic subagent status ticker.
type subagentTickMsg time.Time

func subagentTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return subagentTickMsg(t)
	})
}

// Init initializes the model
func (m *SimpleModel) Init() tea.Cmd {
	m.ReadAgentsMD()
	m.viewportDirty = true
	// Initialize viewport content
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		subagentTick(),
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

func initialMessagesForNewSession(availableSkills []skills.Skill) []api.Message {
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
	return []api.Message{api.NewSystemMessage(systemPrompt)}
}

// StartNewSession saves current progress and switches to a brand new session.
func (m *SimpleModel) StartNewSession() tea.Cmd {
	m.saveSession()

	newSess := session.New(m.currentProviderName, m.currentModelName)
	m.sess = newSess
	m.isResume = false
	m.scrolledToBottom = false

	m.messages = initialMessagesForNewSession(m.availableSkills)
	m.session.Messages = m.messages
	m.pendingToolCalls = nil
	m.waitingForApproval = false
	m.currentMessage = ""
	m.currentUsage = nil
	m.loading = false
	m.err = nil
	m.cancelAPICall = nil
	m.currentOperation = ""
	m.operationStartedAt = time.Time{}

	m.resetDisplayHeader()

	agents := agentsmd.ReadAgentsMD()
	if agents != "" {
		m.session.Messages = append(m.session.Messages, api.NewSystemMessage(agents))
		m.messages = m.session.Messages
	}

	m.saveSession()
	m.AddDisplayContent(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Render("Started new session: "+newSess.ID), "")
	return nil
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
		// On resume, scroll to bottom once the viewport has a real size.
		if m.isResume && !m.scrolledToBottom {
			m.updateViewportContent()
			m.viewport.GotoBottom()
			m.scrolledToBottom = true
		}

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		if isShiftTab(msg) {
			m.toggleCollaborationMode()
			return m, nil
		}

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
					cancelledSubagents := tools.CancelAllRunningSubagents()
					if cancelledSubagents > 0 {
						cancelMsg := lipgloss.NewStyle().
							Foreground(lipgloss.Color("#FF7F50")).
							Render(fmt.Sprintf("✗ Cancelled %d running subagent(s)", cancelledSubagents))
						m.AddDisplayContent(cancelMsg, "")
					}
					m.statusMsg = "Cancelled all running operations"
				} else {
					m.saveSession()
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
			// Send a message (blocked while loading)
			if !m.loading {
				input := m.textarea.Value()
				if strings.TrimSpace(input) != "" {
					// Check for commands
					if strings.HasPrefix(input, "/") {
						return m, m.handleCommand(input)
					}
					// ! prefix: run shell command directly
					if strings.HasPrefix(input, "!") {
						return m, m.runShellCommand(strings.TrimPrefix(input, "!"))
					}
					return m, m.SendMessage(input)
				}
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
				case "d", "D":
					m.showApprovalDetail = !m.showApprovalDetail
					return m, nil
				}
			}
		}

	case StreamOutput:
		// Handle streaming output - add to display content
		m.loading = false
		m.statusMsg = "Ready"
		m.currentOperation = ""
		m.operationStartedAt = time.Time{}

		// Update token usage if provided
		if msg.Usage != nil {
			m.currentUsage = msg.Usage
		}

		// Render the assistant's markdown text and add to display.
		assistantLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Render("Ashron: ")
		if msg.AssistantText != "" {
			rendered := renderMarkdown(msg.AssistantText, m.width)
			lines := strings.Split(rendered, "\n")
			for i, line := range lines {
				if i == 0 {
					m.AddDisplayContent(assistantLabel + line)
				} else if line != "" {
					m.AddDisplayContent(line)
				}
			}
		} else {
			// Tool-only response: show label so the user knows the assistant acted.
			m.AddDisplayContent(assistantLabel)
		}
		for _, line := range msg.ToolLines {
			m.AddDisplayContent(line)
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
		m.savePlanIfNeeded(msg.AssistantText)

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
			m.operationStartedAt = time.Now()
			return m, m.continueConversation()
		} else {
			// All processing done - send notification
			m.currentOperation = ""
			m.operationStartedAt = time.Time{}
			m.sendCompletionNotification()
		}

		return m, nil

	case errorMsg:
		m.err = msg.error
		m.loading = false
		m.statusMsg = "Error: " + msg.error.Error()
		m.currentOperation = ""
		m.operationStartedAt = time.Time{}

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

	case subagentTickMsg:
		m.subagentSummary = tools.GetSubagentsSummary()
		return m, subagentTick()

	case shellCmdMsg:
		m.handleShellCmdMsg(msg)
		return m, nil
	}

	// Update textarea only when the user can freely type
	// (not while waiting for tool approval)
	if !m.waitingForApproval {
		m.textarea, tiCmd = m.textarea.Update(msg)
		cmds = append(cmds, tiCmd)
		m.updateInputMode()
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

// updateInputMode switches the textarea prompt and border to reflect whether the
// user is typing a shell command (! prefix) or a normal message.
func (m *SimpleModel) updateInputMode() {
	s := m.textarea.Styles()
	if strings.HasPrefix(m.textarea.Value(), "!") {
		m.textarea.Prompt = "$ "
		s.Focused.Prompt = lipgloss.NewStyle().Foreground(shellModeColor).Bold(true)
	} else {
		m.textarea.Prompt = "❯ "
		s.Focused.Prompt = lipgloss.NewStyle().Foreground(primaryColor)
	}
	m.textarea.SetStyles(s)
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

	// Status line above the textarea
	if m.loading {
		operation := m.currentOperation
		if operation == "" {
			operation = "Processing"
		}
		elapsed := ""
		if !m.operationStartedAt.IsZero() {
			elapsed = fmt.Sprintf(" [%s]", time.Since(m.operationStartedAt).Round(time.Second))
		}
		b.WriteString(m.spinner.View() + " " + operation + elapsed + " (Esc: cancel request, Ctrl+C: cancel all)\n")
		for _, ag := range m.subagentSummary {
			label := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Render(fmt.Sprintf("  🤖 %s: ", ag.ID))
			lastLine := ag.LastLine
			if lastLine == "" {
				lastLine = "(starting...)"
			}
			maxLen := m.width - lipgloss.Width(label) - 2
			if maxLen > 0 && len(lastLine) > maxLen {
				lastLine = lastLine[:maxLen-1] + "…"
			}
			b.WriteString(label + lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(lastLine) + "\n")
		}
	} else if m.waitingForApproval {
		b.WriteString(m.renderApprovalPanel())
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Render("⚠ Tool execution requires approval. Press [y] to approve, [n] to cancel, [d] to toggle details."))
		b.WriteString("\n")
	}

	// Textarea is always rendered, wrapped with a border applied externally
	// (not via Focused.Base) to avoid double-border.
	taView := m.textarea.View()
	if strings.HasPrefix(m.textarea.Value(), "!") {
		b.WriteString(shellTextareaBorder.Render(taView))
	} else {
		b.WriteString(normalTextareaBorder.Render(taView))
	}

	b.WriteString("\n")
	var modeStr string
	if m.collaborationMode == "plan" {
		modeStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true).
			Render("PLAN") +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				Italic(true).
				Render(" mode (Shift+Tab to toggle)")
	} else {
		modeStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Italic(true).
			Render("DEFAULT mode (Shift+Tab to toggle)")
	}
	b.WriteString(modeStr)

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

func (m *SimpleModel) renderApprovalPanel() string {
	calls := m.pendingApprovalCalls()
	if len(calls) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Render("Pending approvals detected, but no non-auto-approved tool calls were found.")
	}

	var sb strings.Builder
	sb.WriteString("Approve these tool calls:\n")
	maxItems := 6
	for i, tc := range calls {
		if i >= maxItems {
			fmt.Fprintf(&sb, "  ... and %d more", len(calls)-maxItems)
			break
		}
		summary := summarizeToolForApproval(tc)
		why := approvalWhy(tc)
		danger, reason := approvalDanger(tc)
		if danger {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true).Render(fmt.Sprintf("  %d. [DANGER] %s", i+1, summary)))
			sb.WriteString("\n")
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7F50")).Render("     reason: " + reason))
		} else {
			fmt.Fprintf(&sb, "  %d. %s", i+1, summary)
		}

		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0")).Render("     why: " + why))
		if m.showApprovalDetail {
			sb.WriteString("\n")
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")).Render("     args: " + truncateForApproval(tc.Function.Arguments)))
		}
		if i < len(calls)-1 && i < maxItems-1 {
			sb.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFA500")).
		Padding(0, 1).
		Render(sb.String())
}

func (m *SimpleModel) pendingApprovalCalls() []api.ToolCall {
	calls := make([]api.ToolCall, 0, len(m.pendingToolCalls))
	for _, tc := range m.pendingToolCalls {
		if !m.isAutoApproved(tc.Function.Name, tc.Function.Arguments) {
			calls = append(calls, tc)
		}
	}
	return calls
}

func summarizeToolForApproval(tc api.ToolCall) string {
	var oneLiner string
	switch tc.Function.Name {
	case "execute_command":
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.Command) != "" {
			oneLiner = "execute_command: " + truncateForApproval(args.Command)
		}
	case "read_file", "write_file", "list_directory":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.Path) != "" {
			oneLiner = tc.Function.Name + ": " + truncateForApproval(args.Path)
		}
	case "read_skill":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.Name) != "" {
			oneLiner = "read_skill: " + truncateForApproval(args.Name)
		}
	case "fetch_url":
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.URL) != "" {
			oneLiner = "fetch_url: " + truncateForApproval(args.URL)
		}
	}

	if oneLiner == "" {
		oneLiner = tc.Function.Name
	}
	return oneLiner
}

func approvalWhy(tc api.ToolCall) string {
	switch tc.Function.Name {
	case "execute_command":
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.Command) != "" {
			mode := strings.ToLower(tools.EffectiveSandboxMode(&config.ToolsConfig{}, args))
			if mode == "off" {
				return "Runs a shell command without sandbox isolation."
			}
			return "Runs a shell command in the workspace sandbox."
		}
		return "Runs a shell command."
	case "write_file":
		return "Writes file contents; existing files are backed up before overwrite."
	case "read_file":
		return "Reads file contents."
	case "list_directory":
		return "Lists files in a directory."
	case "fetch_url":
		return "Fetches content from a remote URL."
	case "read_skill":
		return "Reads installed skill instructions."
	default:
		return "Uses an internal tool."
	}
}

func approvalDanger(tc api.ToolCall) (bool, string) {
	switch tc.Function.Name {
	case "execute_command":
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return true, "Could not parse command arguments safely."
		}
		if strings.EqualFold(tools.EffectiveSandboxMode(&config.ToolsConfig{}, args), "off") {
			return true, "Command requests sandbox_mode: off."
		}
		return commandDanger(args.Command)
	case "write_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return true, "Could not parse target path."
		}
		path := filepath.Clean(args.Path)
		dangerRoots := []string{"/etc", "/usr", "/bin", "/sbin", "/lib", "/boot", "/System"}
		for _, root := range dangerRoots {
			if path == root || strings.HasPrefix(path, root+"/") {
				return true, "Writes to system-managed path: " + root
			}
		}
	}
	return false, ""
}

func commandDanger(command string) (bool, string) {
	lowered := strings.ToLower(strings.TrimSpace(command))
	dangerPatterns := []struct {
		pattern string
		reason  string
	}{
		{"rm -rf", "Recursive delete detected."},
		{"git reset --hard", "Hard reset discards local changes."},
		{"mkfs", "Filesystem formatting command detected."},
		{"dd if=", "Raw disk write command detected."},
		{"shutdown", "System shutdown command detected."},
		{"reboot", "System reboot command detected."},
		{"chmod -r", "Recursive permission change detected."},
		{"chown -r", "Recursive ownership change detected."},
		{"| sh", "Piped shell execution detected."},
	}
	for _, p := range dangerPatterns {
		if strings.Contains(lowered, p.pattern) {
			return true, p.reason
		}
	}
	return false, ""
}

func truncateForApproval(s string) string {
	s = strings.TrimSpace(s)
	const maxLen = 120
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func isShiftTab(msg tea.KeyPressMsg) bool {
	if msg.Code == tea.KeyTab && msg.Mod.Contains(tea.ModShift) {
		return true
	}
	return msg.String() == "shift+tab"
}

func (m *SimpleModel) toggleCollaborationMode() {
	if m.loading {
		return
	}

	if m.collaborationMode == "default" {
		m.collaborationMode = "plan"
		m.messages = append(m.messages, api.NewSystemMessage("Collaboration mode is Plan. First provide a concise plan and wait for explicit user approval before running tools or making code changes."))
		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Render("Mode switched: PLAN"))
		return
	}

	m.collaborationMode = "default"
	m.messages = append(m.messages, api.NewSystemMessage("Collaboration mode is Default. Execute the request directly when feasible, using tools and edits as needed."))
	m.AddDisplayContent(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Mode switched: DEFAULT"))
}

// updateViewportContent updates the viewport with current display content.
// It only calls SetContent when the content has changed (viewportDirty),
// to avoid resetting the scroll position on every render.
func (m *SimpleModel) updateViewportContent() {
	if !m.viewportDirty {
		return
	}
	content := strings.Join(m.displayContent, "\n")
	m.viewport.SetContent(content)
	m.viewportDirty = false
}

// handleCommand processes slash commands
func (m *SimpleModel) handleCommand(input string) tea.Cmd {
	parts, err := parseCommandLine(input)
	if err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render(fmt.Sprintf("Command parse error: %v", err))
		m.AddDisplayContent(errorMsg, "")
		return nil
	}
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

func (m *SimpleModel) RenderStatus() tea.Cmd {
	cwd, _ := os.Getwd()
	sessionID := "(none)"
	if m.sess != nil && m.sess.ID != "" {
		sessionID = m.sess.ID
	}

	status := fmt.Sprintf(`Current Status:
  Mode: %s
  Model: %s (provider: %s)
  Working Dir: %s
  Session ID: %s
  Sandbox Mode: %s
  YOLO Mode: %v
  Auto-Approve Tools: %d
  Auto-Approve Commands: %d`,
		strings.ToUpper(m.collaborationMode),
		m.currentModelName,
		m.currentProviderName,
		cwd,
		sessionID,
		m.config.Tools.SandboxMode,
		m.config.Tools.Yolo,
		len(m.config.Tools.AutoApproveTools),
		len(m.config.Tools.AutoApproveCommands),
	)

	msg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(status)
	for _, line := range strings.Split(msg, "\n") {
		m.AddDisplayContent(line)
	}
	m.AddDisplayContent("")
	return nil
}

func (m *SimpleModel) RenderSessions(args []string) tea.Cmd {
	action := "list"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}

	switch action {
	case "list":
		summaries, err := session.ListSummaries(30)
		if err != nil {
			errMsg := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333")).
				Render(fmt.Sprintf("Error listing sessions: %v", err))
			m.AddDisplayContent(errMsg, "")
			return nil
		}
		if len(summaries) == 0 {
			m.AddDisplayContent(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				Render("No sessions found."))
			m.AddDisplayContent("")
			return nil
		}

		currentID := ""
		if m.sess != nil {
			currentID = m.sess.ID
		}

		var sb strings.Builder
		sb.WriteString("Recent Sessions:\n")
		for i, s := range summaries {
			marker := " "
			if s.ID == currentID {
				marker = "*"
			}
			fmt.Fprintf(&sb, "  %s %2d. %s  %s  %s/%s\n", marker, i+1, s.ID, s.CreatedAt.Local().Format(time.DateTime), shortPath(s.WorkingDir), s.Model)
		}
		sb.WriteString("\nUsage: /sessions resume <id> | /sessions delete <id>")

		msg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Render(sb.String())
		for _, line := range strings.Split(msg, "\n") {
			m.AddDisplayContent(line)
		}
		m.AddDisplayContent("")
		return nil

	case "resume":
		if len(args) < 2 {
			m.AddDisplayContent(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Render("Usage: /sessions resume <id>"), "")
			return nil
		}
		sess, err := session.Load(args[1])
		if err != nil {
			m.AddDisplayContent(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333")).
				Render(fmt.Sprintf("Failed to load session: %v", err)), "")
			return nil
		}

		if err := m.switchModel(sess.Model); err != nil {
			m.AddDisplayContent(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFA500")).
				Render(fmt.Sprintf("Warning: could not switch model to %s: %v", sess.Model, err)))
		}

		m.sess = sess
		m.isResume = true
		m.messages = sess.Messages
		m.session.Messages = sess.Messages
		m.pendingToolCalls = nil
		m.waitingForApproval = false
		m.currentMessage = ""
		m.loading = false
		m.err = nil
		m.cancelAPICall = nil
		m.currentOperation = ""
		m.operationStartedAt = time.Time{}
		m.resetDisplayHeader()
		m.restoreSessionDisplay()
		m.viewport.GotoBottom()

		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Render("Resumed session: "+sess.ID), "")
		return nil

	case "delete":
		if len(args) < 2 {
			m.AddDisplayContent(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Render("Usage: /sessions delete <id>"), "")
			return nil
		}
		target := args[1]
		if m.sess != nil && target == m.sess.ID {
			m.AddDisplayContent(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333")).
				Render("Cannot delete the currently active session."), "")
			return nil
		}
		if err := session.Delete(target); err != nil {
			m.AddDisplayContent(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF3333")).
				Render(fmt.Sprintf("Failed to delete session: %v", err)), "")
			return nil
		}
		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Render("Deleted session: "+target), "")
		return nil

	default:
		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render("Usage: /sessions [list|resume <id>|delete <id>]"), "")
		return nil
	}
}

func (m *SimpleModel) RenderTools() tea.Cmd {
	all := tools.GetAllTools()
	var sb strings.Builder
	sb.WriteString("Available Tools and Approval Policy:\n")
	for _, t := range all {
		policy := "manual approval"
		if m.config.Tools.Yolo {
			policy = "auto-approved (YOLO)"
		} else if containsString(m.config.Tools.AutoApproveTools, t.Name) {
			policy = "auto-approved"
		} else if t.Name == "execute_command" && len(m.config.Tools.AutoApproveCommands) > 0 {
			policy = fmt.Sprintf("manual (command rules: %d, unsandboxed always manual)", len(m.config.Tools.AutoApproveCommands))
		}
		fmt.Fprintf(&sb, "  - %s: %s\n", t.Name, policy)
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

func (m *SimpleModel) RenderCustomCommands() tea.Cmd {
	if len(m.availableCustomCommands) == 0 {
		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Render("No custom commands found. Add *.md files under $XDG_CONFIG_HOME/ashron/commands."), "")
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Custom Slash Commands:\n")
	for _, cmd := range m.availableCustomCommands {
		fmt.Fprintf(&sb, "  /%s - %s\n    %s\n", cmd.Name, cmd.Description, cmd.Path)
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

func (m *SimpleModel) registerCustomCommands() {
	for _, cc := range m.availableCustomCommands {
		custom := cc
		m.commandRegistry.Register(Command{
			Name:        "/" + custom.Name,
			Description: "Custom: " + custom.Description,
			Body: func(_ *CommandRegistry, model *SimpleModel, args []string) tea.Cmd {
				return model.runCustomCommand(custom, args)
			},
		})
	}
}

func (m *SimpleModel) runCustomCommand(cmd customcmd.Command, args []string) tea.Cmd {
	expanded := customcmd.Expand(cmd.Template, args)
	if strings.TrimSpace(expanded) == "" {
		m.AddDisplayContent(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Render("Custom command produced empty prompt: /"+cmd.Name), "")
		return nil
	}

	label := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(fmt.Sprintf("↪ /%s (%s)", cmd.Name, cmd.Path))
	m.AddDisplayContent(label)
	return m.SendMessage(expanded)
}

func (m *SimpleModel) resetDisplayHeader() {
	m.displayContent = buildHeaderLines(m.config.Tools.Yolo)
	m.viewportDirty = true
}

func buildHeaderLines(yolo bool) []string {
	welcomeMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("🤖 Ashron - AI Coding Assistant")
	versionMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(buildInfoLine())
	helpMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Type /help for available commands")

	lines := []string{welcomeMsg, versionMsg, helpMsg}
	if yolo {
		yoloMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF3333")).
			Bold(true).
			Render("YOLO MODE ENABLED: sandbox disabled and tools auto-approved")
		lines = append(lines, yoloMsg)
	}
	lines = append(lines, "")
	return lines
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// Helper functions for managing messages
func (m *SimpleModel) addUserMessage(content string) {
	m.messages = append(m.messages, api.NewUserMessage(content))
	// Persist immediately so interrupted requests are resumable.
	m.saveSession()
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
	m.currentOperation = "Executing approved tools"
	m.operationStartedAt = time.Now()
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
	if m.currentOperation == "" {
		m.currentOperation = "Processing tool results"
	}
	if m.operationStartedAt.IsZero() {
		m.operationStartedAt = time.Now()
	}
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
	m.viewportDirty = true
	m.updateViewportContent()
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

func (m *SimpleModel) savePlanIfNeeded(content string) {
	if m.collaborationMode != "plan" {
		return
	}
	if strings.TrimSpace(content) == "" {
		return
	}

	sessionID := ""
	if m.sess != nil {
		sessionID = m.sess.ID
	}
	path, err := plan.Save(sessionID, content)
	if err != nil {
		slog.Warn("Failed to save plan", "error", err)
		return
	}

	m.AddDisplayContent(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Plan saved: " + path))
}

// AddDisplayContent appends content to displayContent and automatically scrolls to bottom
func (m *SimpleModel) AddDisplayContent(content ...string) {
	m.displayContent = append(m.displayContent, content...)
	m.viewportDirty = true
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func (m *SimpleModel) CompactContext() (int, int) {
	originalCount := len(m.messages)
	m.messages = m.contextMgr.Compact(m.messages)
	newCount := len(m.messages)
	return originalCount, newCount
}
