package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tokuhirom/ashron/internal/tools"

	"github.com/tokuhirom/ashron/internal/api"
)

type toolExecutionMsg struct {
	results []api.ToolResult
	hasMore bool
	output  string
}

type errorMsg struct {
	error error
}

// StreamOutput represents output to be printed
type StreamOutput struct {
	Content string
	Usage   *api.Usage
}

// StreamingMsg represents a message chunk during streaming
type StreamingMsg struct {
	Content string
}

// SendMessage sends a user message to the API (SimpleModel version)
func (m *SimpleModel) SendMessage(input string) tea.Cmd {
	slog.Info("User sending message",
		slog.Int("length", len(input)))

	// Store the message for display
	displayInput := compactUserInputForDisplay(input)
	userMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("You: ") + displayInput

	m.addUserMessage(input)
	m.textarea.SetValue("")
	m.loading = true
	m.statusMsg = "Thinking..."
	m.currentMessage = ""
	m.currentOperation = "Processing user message"
	m.operationStartedAt = time.Now()
	m.lastUserInput = input

	// Add user message to display content
	m.AddDisplayContent(userMsg, "")

	// Return a command that processes the message
	return m.processMessage()
}

// cancelCurrentRequest cancels the in-flight API request and resets loading state.
func (m *SimpleModel) cancelCurrentRequest() {
	if m.cancelAPICall != nil {
		m.cancelAPICall()
		m.cancelAPICall = nil
		cancelMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF7F50")).
			Render("✗ Request cancelled")
		m.AddDisplayContent(cancelMsg, "")
	} else {
		cancelMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF7F50")).
			Render("✗ No cancellable API request")
		m.AddDisplayContent(cancelMsg, "")
	}
	m.loading = false
	m.statusMsg = "Cancelled"
	m.currentOperation = ""
	m.operationStartedAt = time.Time{}
	m.saveSession()
}

// processMessage handles the actual API call
func (m *SimpleModel) processMessage() tea.Cmd {
	// Create a cancellable context now (in Update goroutine) so Escape can cancel it.
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelAPICall = cancel

	return func() tea.Msg {
		// Check if context needs compaction
		if m.config.Context.AutoCompact {
			usage := m.contextMgr.GetTokenUsage(m.messages)
			threshold := int(float32(m.config.Context.MaxTokens) * m.config.Context.CompactionRatio)
			slog.Debug("Checking context compaction",
				slog.Int("usage", usage),
				slog.Int("threshold", threshold),
				slog.Int("maxTokens", m.config.Context.MaxTokens))

			if usage > threshold {
				slog.Info("Compacting context",
					slog.Int("beforeMessages", len(m.messages)),
					slog.Int("tokenUsage", usage))
				m.messages = m.contextMgr.Compact(m.messages)
				slog.Info("Context compacted",
					slog.Int("afterMessages", len(m.messages)))
			}
		}

		// Stream the response
		builtinTools := tools.GetBuiltinTools()
		slog.Debug("Requesting streaming completion",
			slog.Int("messages", len(m.messages)),
			slog.Int("tools", len(builtinTools)))
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, m.messages, builtinTools)
		if err != nil {
			if ctx.Err() != nil {
				// Cancelled by user - not an error worth reporting
				return nil
			}
			// Include the last message role to help diagnose which phase failed.
			lastRole := ""
			if n := len(m.messages); n > 0 {
				lastRole = m.messages[n-1].Role
			}
			slog.Error("Failed to start streaming", "error", err, "lastMessageRole", lastRole)
			return errorMsg{error: fmt.Errorf("%w (after %s message)", err, lastRole)}
		}

		// Start handling stream
		return m.processStreamNew(ctx, stream)
	}
}

// processStreamNew processes streaming responses with proper output collection
func (m *SimpleModel) processStreamNew(ctx context.Context, stream <-chan api.StreamEvent) tea.Msg {
	var fullContent strings.Builder
	var output strings.Builder // Collects all output to print
	var toolCalls []api.ToolCall
	var chunkCount int
	var usage *api.Usage
	// Map to accumulate tool call arguments by index
	toolCallArgs := make(map[int]*strings.Builder)
	toolCallsByIndex := make(map[int]*api.ToolCall)

	// Add assistant label
	assistantLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Render("Ashron: ")
	output.WriteString(assistantLabel)

	slog.Debug("Starting to process stream")
	m.currentOperation = "Receiving AI response"

	for event := range stream {
		if event.Error != nil {
			if ctx.Err() != nil {
				// Stream closed due to user cancellation - not an error
				slog.Info("Stream cancelled by user")
				return nil
			}
			slog.Error("Stream error received", "error", event.Error)
			return errorMsg{error: event.Error}
		}

		if event.Data != nil {
			// Capture usage data if present
			if event.Data.Usage != nil {
				usage = event.Data.Usage
				slog.Debug("Received usage data",
					"promptTokens", usage.PromptTokens,
					"completionTokens", usage.CompletionTokens,
					"totalTokens", usage.TotalTokens)
			}

			if len(event.Data.Choices) > 0 {
				choice := event.Data.Choices[0]
				chunkCount++

				// Handle content
				if choice.Delta.Content != "" {
					// Collect content for output
					output.WriteString(choice.Delta.Content)
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
				for _, deltaToolCall := range choice.Delta.ToolCalls {
					// Use the Index field from the streaming delta directly.
					// Previously this used choice.Index*100+i which mapped ALL
					// parallel tool calls to idx=0, causing argument concatenation
					// like {"path":"."} {"path":"internal"} -> invalid JSON.
					idx := deltaToolCall.Index

					if deltaToolCall.ID != "" {
						// New tool call starting - reset accumulator for this index
						tc := &api.ToolCall{
							ID:   deltaToolCall.ID,
							Type: deltaToolCall.Type,
							Function: api.FunctionCall{
								Name:      deltaToolCall.Function.Name,
								Arguments: deltaToolCall.Function.Arguments,
							},
						}
						toolCallsByIndex[idx] = tc
						toolCallArgs[idx] = &strings.Builder{}
						if deltaToolCall.Function.Arguments != "" {
							toolCallArgs[idx].WriteString(deltaToolCall.Function.Arguments)
						}
					} else if deltaToolCall.Function.Arguments != "" {
						// Continuing existing tool call - append arguments
						if _, exists := toolCallsByIndex[idx]; exists {
							if toolCallArgs[idx] == nil {
								toolCallArgs[idx] = &strings.Builder{}
							}
							toolCallArgs[idx].WriteString(deltaToolCall.Function.Arguments)
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
					m.currentOperation = "Finalizing response"
					// Add newlines after content if there was content
					if fullContent.Len() > 0 {
						output.WriteString("\n\n")
					}

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

						// Keep tool execution display compact (single-line summary).
						for _, line := range toolCallSummaryLines(*tc) {
							output.WriteString(lipgloss.NewStyle().
								Foreground(lipgloss.Color("#626262")).
								Render(line))
							output.WriteString("\n")
						}

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

					// Store tool calls for processing
					if len(toolCalls) > 0 {
						m.pendingToolCalls = toolCalls
					}

					// Store the final content
					m.currentMessage = fullContent.String()

					// Return the complete output as a StreamOutput message
					return StreamOutput{Content: output.String(), Usage: usage}
				}
			}
		}
	}

	// Stream ended without a proper finish
	return StreamOutput{Content: output.String(), Usage: usage}
}

// executePendingTools executes approved tool calls (SimpleModel version)
func (m *SimpleModel) executePendingTools() tea.Cmd {
	return func() tea.Msg {
		var output strings.Builder
		var results []api.ToolResult

		for _, tc := range m.pendingToolCalls {
			result := m.toolExec.Execute(tc)
			results = append(results, result)

			// Keep tool result display compact; detailed output remains in tool messages.
			if result.Error != nil {
				output.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FF7F50")).
					Render("tool error: " + tc.Function.Name))
				output.WriteString("\n")
			}

			// Add tool result message
			m.messages = append(m.messages, api.NewToolMessage(tc.ID, result.Output))
		}

		m.pendingToolCalls = nil

		// Return the output and indicate we need to continue
		return toolExecutionMsg{
			results: results,
			hasMore: true,
			output:  output.String(),
		}
	}
}

func toolCallSummaryLines(tc api.ToolCall) []string {
	lines := []string{"• Explored"}

	if tc.Function.Name == "read_file" {
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && args.Path != "" {
			return append(lines, "  └ Read file: "+args.Path)
		}
		return append(lines, "  └ Read file")
	}
	if tc.Function.Name == "write_file" {
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || args.Path == "" {
			return append(lines, "  └ Write file")
		}

		change, err := tools.AnalyzeWriteFileChange(args.Path, args.Content)
		if err != nil {
			return append(lines,
				"  └ Write file: "+args.Path,
				"    preview unavailable: "+err.Error(),
			)
		}

		changeLine := fmt.Sprintf("    lines %d -> %d, +%d -%d", change.OldLines, change.NewLines, change.Added, change.Removed)
		if change.Unchanged {
			changeLine += " (unchanged)"
		}
		if change.Existed {
			return append(lines,
				"  └ Write file: "+args.Path,
				changeLine,
				"    backup will be created before apply",
			)
		}
		return append(lines,
			"  └ Write file: "+args.Path,
			changeLine,
			"    new file",
		)
	}

	return append(lines, "  └ Used tool: "+tc.Function.Name)
}
