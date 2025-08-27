package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tokuhirom/ashron/internal/api"
)

// Message types for tea.Cmd
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

// sendMessage sends a user message to the API
func (m *Model) sendMessage(input string) tea.Cmd {
	slog.Info("User sending message", "length", len(input))
	m.addUserMessage(input)
	m.textarea.SetValue("")
	m.loading = true
	m.statusMsg = "Thinking..."
	m.updateViewport()

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

		// Start handling stream in a blocking way to ensure proper message flow
		return m.processStream(stream)
	}
}

// processStream processes streaming responses synchronously
func (m *Model) processStream(stream <-chan api.StreamEvent) tea.Msg {
	var fullContent strings.Builder
	var toolCalls []api.ToolCall
	var chunkCount int
	// Map to accumulate tool call arguments by index
	toolCallArgs := make(map[int]*strings.Builder)
	toolCallsByIndex := make(map[int]*api.ToolCall)

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
				fullContent.WriteString(choice.Delta.Content)
				slog.Debug("Received content chunk", "chunk", chunkCount, "contentLength", len(choice.Delta.Content), "totalLength", fullContent.Len())

				// Check if content contains XML-style function calls (some models return this format)
				currentContent := fullContent.String()
				if strings.Contains(currentContent, "<function=") || strings.Contains(currentContent, "<tool_call>") {
					// This is likely a function call in XML format
					// We'll parse it when the stream completes
					slog.Debug("Detected XML-style function call in content")
				}

				// Update the display with partial content
				if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
					m.messages[len(m.messages)-1].Content = currentContent
				} else {
					m.messages = append(m.messages, api.Message{
						Role:    "assistant",
						Content: currentContent,
					})
				}
			}

			// Handle tool calls - accumulate arguments properly
			for i, deltaToolCall := range choice.Delta.ToolCalls {
				// Determine the index for this tool call
				idx := choice.Index*100 + i // Use choice index and tool call position

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
					// Find the tool call this belongs to
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
					slog.Debug("Finalized tool call",
						"id", tc.ID,
						"name", tc.Function.Name,
						"args", tc.Function.Arguments)
				}

				// Check if content contains XML-style tool calls
				finalContent := fullContent.String()
				if len(toolCalls) == 0 && (strings.Contains(finalContent, "<function=") || strings.Contains(finalContent, "<tool_call>")) {
					slog.Info("Parsing XML-style tool calls from content")
					xmlToolCalls := parseXMLToolCalls(finalContent)
					if len(xmlToolCalls) > 0 {
						toolCalls = xmlToolCalls
						// Clear the content since it was actually tool calls
						fullContent.Reset()
					}
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

// checkToolApproval checks if tools need approval
func (m *Model) checkToolApproval() {
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
		m.updateViewport()
	} else {
		m.approvePendingTools()
		m.executePendingTools()
	}
}

// isAutoApproved checks if a tool is auto-approved
func (m *Model) isAutoApproved(toolName string) bool {
	for _, approved := range m.config.Tools.AutoApprove {
		if approved == toolName {
			return true
		}
	}
	return false
}

// approvePendingTools approves pending tool calls
func (m *Model) approvePendingTools() {
	m.waitingForApproval = false
	m.loading = true
	m.statusMsg = "Executing tools..."
}

// executePendingTools executes approved tool calls
func (m *Model) executePendingTools() tea.Cmd {
	return func() tea.Msg {
		var results []api.ToolResult

		for _, tc := range m.pendingToolCalls {
			result := m.toolExec.Execute(tc)
			results = append(results, result)

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

// handleToolResult processes tool execution results
func (m *Model) handleToolResult(msg toolExecutionMsg) {
	m.updateViewport()

	if msg.hasMore {
		m.statusMsg = "Processing tool results..."
	}
}

// continueConversation continues after tool execution
func (m *Model) continueConversation() tea.Cmd {
	return m.sendContinuation()
}

// sendContinuation sends a continuation request
func (m *Model) sendContinuation() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		slog.Debug("Sending continuation request after tool execution")
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, m.messages, api.BuiltinTools)
		if err != nil {
			slog.Error("Failed to send continuation request", "error", err)
			return errorMsg{error: err}
		}

		// Process the stream synchronously
		return m.processStream(stream)
	}
}

// compactContext compacts the conversation context
func (m *Model) compactContext() {
	originalCount := len(m.messages)
	m.messages = m.contextMgr.Compact(m.messages)
	newCount := len(m.messages)

	m.statusMsg = fmt.Sprintf("Context compacted: %d → %d messages", originalCount, newCount)
	m.updateViewport()
}

// waitForStream waits for stream updates
func waitForStream(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		// This would be connected to the actual stream handler
		return nil
	}
}

// handleStreamMessage processes a stream message
func (m *Model) handleStreamMessage(msg streamMsg) {
	if msg.chunk != nil && len(msg.chunk.Choices) > 0 {
		// Update the last assistant message or create new one
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			m.messages[len(m.messages)-1].Content += msg.chunk.Choices[0].Delta.Content
		} else {
			m.addAssistantMessage(msg.chunk.Choices[0].Delta.Content)
		}
		m.updateViewport()
	}
}

// completeStream completes the streaming response
func (m *Model) completeStream() {
	m.loading = false
	m.statusMsg = "Ready"
	m.updateViewport()
}

// sendError sends an error message
func (m *Model) sendError(err error) {
	m.err = err
	m.loading = false
	m.statusMsg = "Error: " + err.Error()
	m.addSystemMessage("Error: " + err.Error())
	m.updateViewport()
}
