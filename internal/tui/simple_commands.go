package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tokuhirom/ashron/internal/api"
)

// Message types for tea.Cmd (duplicated from commands.go for SimpleModel)
type streamMsg struct {
	chunk *api.StreamResponse
}

type streamCompleteMsg struct{}

type toolExecutionMsg struct {
	results []api.ToolResult
	hasMore bool
}

type errorMsg struct {
	error error
}

// sendMessage sends a user message to the API (SimpleModel version)
func (m *SimpleModel) sendMessage(input string) tea.Cmd {
	slog.Info("User sending message", "length", len(input))
	
	// Print user message
	fmt.Println(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("You: ") + input)
	fmt.Println()
	
	m.addUserMessage(input)
	m.textarea.SetValue("")
	m.loading = true
	m.statusMsg = "Thinking..."
	m.currentMessage = ""

	return func() tea.Msg {
		// Check if context needs compaction
		if m.config.Context.AutoCompact {
			usage := m.contextMgr.GetTokenUsage(m.messages)
			threshold := int(float32(m.config.Context.MaxTokens) * m.config.Context.CompactionRatio)
			slog.Debug("Checking context compaction", "usage", usage, "threshold", threshold, "maxTokens", m.config.Context.MaxTokens)

			if usage > threshold {
				slog.Info("Compacting context", "beforeMessages", len(m.messages), "tokenUsage", usage)
				m.messages = m.contextMgr.Compact(m.messages)
				slog.Info("Context compacted", "afterMessages", len(m.messages))
			}
		}

		// Stream the response
		ctx := context.Background()
		slog.Debug("Requesting streaming completion", "messages", len(m.messages), "tools", len(api.BuiltinTools))
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, m.messages, api.BuiltinTools)
		if err != nil {
			slog.Error("Failed to start streaming", "error", err)
			return errorMsg{error: err}
		}

		// Start handling stream
		return m.processStream(stream)
	}
}

// processStream processes streaming responses (SimpleModel version)
func (m *SimpleModel) processStream(stream <-chan api.StreamEvent) tea.Msg {
	var fullContent strings.Builder
	var toolCalls []api.ToolCall
	var chunkCount int
	// Map to accumulate tool call arguments by index
	toolCallArgs := make(map[int]*strings.Builder)
	toolCallsByIndex := make(map[int]*api.ToolCall)
	
	// Print assistant label at start
	fmt.Print(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Render("Ashron: "))

	slog.Debug("Starting to process stream")

	for event := range stream {
		if event.Error != nil {
			slog.Error("Stream error received", "error", event.Error)
			return errorMsg{error: event.Error}
		}

		if event.Data != nil && len(event.Data.Choices) > 0 {
			choice := event.Data.Choices[0]
			chunkCount++

			// Handle content
			if choice.Delta.Content != "" {
				// Print content directly to stdout as it streams
				fmt.Print(choice.Delta.Content)
				fullContent.WriteString(choice.Delta.Content)
				slog.Debug("Received content chunk", "chunk", chunkCount, "contentLength", len(choice.Delta.Content), "totalLength", fullContent.Len())

				// Update message history
				if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
					m.messages[len(m.messages)-1].Content = fullContent.String()
				} else {
					m.messages = append(m.messages, api.Message{
						Role:    "assistant",
						Content: fullContent.String(),
					})
				}
			}

			// Handle tool calls - accumulate arguments properly
			for i, deltaToolCall := range choice.Delta.ToolCalls {
				// Determine the index for this tool call
				idx := choice.Index*100 + i

				if deltaToolCall.ID != "" {
					// New tool call starting
					tc := &api.ToolCall{
						ID:   deltaToolCall.ID,
						Type: deltaToolCall.Type,
						Function: api.FunctionCall{
							Name:      deltaToolCall.Function.Name,
							Arguments: deltaToolCall.Function.Arguments,
						},
					}
					toolCallsByIndex[idx] = tc
					if deltaToolCall.Function.Arguments != "" {
						if toolCallArgs[idx] == nil {
							toolCallArgs[idx] = &strings.Builder{}
						}
						toolCallArgs[idx].WriteString(deltaToolCall.Function.Arguments)
					}
				} else if deltaToolCall.Function.Arguments != "" {
					// Continuing existing tool call - append arguments
					if _, exists := toolCallsByIndex[idx]; exists {
						if toolCallArgs[idx] == nil {
							toolCallArgs[idx] = &strings.Builder{}
						}
						toolCallArgs[idx].WriteString(deltaToolCall.Function.Arguments)
					} else {
						// Try to find by function name if index doesn't work
						for tidx, tc := range toolCallsByIndex {
							if tc.Function.Name == deltaToolCall.Function.Name {
								if toolCallArgs[tidx] == nil {
									toolCallArgs[tidx] = &strings.Builder{}
								}
								toolCallArgs[tidx].WriteString(deltaToolCall.Function.Arguments)
								break
							}
						}
					}
				}

				slog.Debug("Tool call delta",
					"id", deltaToolCall.ID,
					"name", deltaToolCall.Function.Name,
					"args", deltaToolCall.Function.Arguments,
					"index", idx)
			}

			// Check if finished
			if choice.FinishReason == "stop" || choice.FinishReason == "tool_calls" {
				// Finalize tool calls with complete arguments
				for idx, tc := range toolCallsByIndex {
					if toolCallArgs[idx] != nil && toolCallArgs[idx].Len() > 0 {
						tc.Function.Arguments = toolCallArgs[idx].String()
					}
					// Ensure we have valid arguments - if empty, use empty JSON object
					if tc.Function.Arguments == "" {
						tc.Function.Arguments = "{}"
					}
					toolCalls = append(toolCalls, *tc)
					
					// Print tool call info
					fmt.Println()
					fmt.Println(lipgloss.NewStyle().
						Background(lipgloss.Color("#2a2a2a")).
						Foreground(lipgloss.Color("#FF7F50")).
						Padding(0, 1).
						Render(fmt.Sprintf("🔧 Calling %s", tc.Function.Name)))
					
					slog.Debug("Finalized tool call",
						"id", tc.ID,
						"name", tc.Function.Name,
						"args", tc.Function.Arguments)
				}

				slog.Info("Stream finished", "reason", choice.FinishReason, "chunks", chunkCount, "contentLength", fullContent.Len(), "toolCalls", len(toolCalls))

				// Update the complete message
				if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
					m.messages[len(m.messages)-1].Content = fullContent.String()
					m.messages[len(m.messages)-1].ToolCalls = toolCalls
				} else {
					msg := api.Message{
						Role:      "assistant",
						Content:   fullContent.String(),
						ToolCalls: toolCalls,
					}
					m.messages = append(m.messages, msg)
				}

				// Handle tool calls if any
				if len(toolCalls) > 0 {
					m.pendingToolCalls = toolCalls
					return toolExecutionMsg{
						results: nil,
						hasMore: false, // Will trigger tool approval check
					}
				}

				return streamCompleteMsg{}
			}
		}
	}

	// Stream ended without proper finish
	return streamCompleteMsg{}
}

// checkToolApproval checks if tools need approval (SimpleModel version)
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
		m.executePendingTools()
	}
}

// isAutoApproved checks if a tool is auto-approved (SimpleModel version)
func (m *SimpleModel) isAutoApproved(toolName string) bool {
	for _, approved := range m.config.Tools.AutoApprove {
		if approved == toolName {
			return true
		}
	}
	return false
}

// approvePendingTools approves pending tool calls (SimpleModel version)
func (m *SimpleModel) approvePendingTools() {
	m.waitingForApproval = false
	m.loading = true
	m.statusMsg = "Executing tools..."
}

// executePendingTools executes approved tool calls (SimpleModel version)
func (m *SimpleModel) executePendingTools() tea.Cmd {
	return func() tea.Msg {
		var results []api.ToolResult

		for _, tc := range m.pendingToolCalls {
			result := m.toolExec.Execute(tc)
			results = append(results, result)

			// Print tool result
			fmt.Println(lipgloss.NewStyle().
				Background(lipgloss.Color("#2a2a2a")).
				Foreground(lipgloss.Color("#626262")).
				Padding(0, 1).
				Render("Tool Result:"))
			fmt.Println(result.Output)
			
			// Add tool result message
			m.messages = append(m.messages, api.NewToolMessage(tc.ID, result.Output))
		}

		m.pendingToolCalls = nil

		// Continue conversation with tool results
		return toolExecutionMsg{
			results: results,
			hasMore: true,
		}
	}
}

// handleToolResult processes tool execution results (SimpleModel version)
func (m *SimpleModel) handleToolResult(msg toolExecutionMsg) {
	if msg.hasMore {
		m.statusMsg = "Processing tool results..."
	}
}

// continueConversation continues after tool execution (SimpleModel version)
func (m *SimpleModel) continueConversation() tea.Cmd {
	return m.sendContinuation()
}

// sendContinuation sends a continuation request (SimpleModel version)
func (m *SimpleModel) sendContinuation() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		slog.Debug("Sending continuation request after tool execution")
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, m.messages, api.BuiltinTools)
		if err != nil {
			slog.Error("Failed to send continuation request", "error", err)
			return errorMsg{error: err}
		}

		// Process the stream
		return m.processStream(stream)
	}
}

// compactContext compacts the conversation context (SimpleModel version)
func (m *SimpleModel) compactContext() {
	originalCount := len(m.messages)
	m.messages = m.contextMgr.Compact(m.messages)
	newCount := len(m.messages)

	fmt.Fprintln(os.Stderr, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render(fmt.Sprintf("Context compacted: %d → %d messages", originalCount, newCount)))
}

// handleStreamMessage processes a stream message (SimpleModel version)
func (m *SimpleModel) handleStreamMessage(msg streamMsg) {
	if msg.chunk != nil && len(msg.chunk.Choices) > 0 {
		content := msg.chunk.Choices[0].Delta.Content
		
		// Print content directly
		if content != "" {
			fmt.Print(content)
			m.currentMessage += content
			
			// Update the last assistant message or create new one
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
				m.messages[len(m.messages)-1].Content += content
			} else {
				m.addAssistantMessage(content)
			}
		}
	}
}

// waitForStream waits for stream updates (SimpleModel version)
func waitForStream(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		// This would be connected to the actual stream handler
		return nil
	}
}