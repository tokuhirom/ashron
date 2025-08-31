package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	userMsg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true).
		Render("You: ") + input

	m.addUserMessage(input)
	m.textarea.SetValue("")
	m.loading = true
	m.statusMsg = "Thinking..."
	m.currentMessage = ""
	m.currentOperation = "Processing user message"
	m.lastUserInput = input

	// Add user message to display content
	m.AddDisplayContent(userMsg, "")

	// Return a command that processes the message
	return m.processMessage()
}

// processMessage handles the actual API call
func (m *SimpleModel) processMessage() tea.Cmd {
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
		ctx := context.Background()
		builtinTools := tools.GetBuiltinTools()
		slog.Debug("Requesting streaming completion",
			slog.Int("messages", len(m.messages)),
			slog.Int("tools", len(builtinTools)))
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, m.messages, builtinTools)
		if err != nil {
			slog.Error("Failed to start streaming", "error", err)
			return errorMsg{error: err}
		}

		// Start handling stream
		return m.processStreamNew(stream)
	}
}

// processStreamNew processes streaming responses with proper output collection
func (m *SimpleModel) processStreamNew(stream <-chan api.StreamEvent) tea.Msg {
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

						// Add tool call info to output with arguments
						output.WriteString(lipgloss.NewStyle().
							Background(lipgloss.Color("#2a2a2a")).
							Foreground(lipgloss.Color("#FF7F50")).
							Padding(0, 1).
							Render(fmt.Sprintf("🔧 Calling %s", tc.Function.Name)))
						output.WriteString("\n")

						// Display arguments in a readable format
						if tc.Function.Arguments != "" && tc.Function.Arguments != "{}" {
							var args map[string]interface{}
							if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
								output.WriteString(lipgloss.NewStyle().
									Foreground(lipgloss.Color("#626262")).
									PaddingLeft(3).
									Render("Arguments:"))
								output.WriteString("\n")
								for key, value := range args {
									output.WriteString(lipgloss.NewStyle().
										Foreground(lipgloss.Color("#626262")).
										PaddingLeft(5).
										Render(fmt.Sprintf("• %s: %v", key, value)))
									output.WriteString("\n")
								}
							}
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

			// Collect tool result output
			output.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("#2a2a2a")).
				Foreground(lipgloss.Color("#626262")).
				Padding(0, 1).
				Render("Tool Result:"))
			output.WriteString("\n")

			// Truncate long output for display
			lines := strings.Split(result.Output, "\n")
			maxLines := 20
			if len(lines) > maxLines {
				displayOutput := strings.Join(lines[:maxLines], "\n")
				output.WriteString(displayOutput)
				output.WriteString("\n")
				output.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#626262")).
					Italic(true).
					Render(fmt.Sprintf("[... %d more lines truncated for display]", len(lines)-maxLines)))
			} else {
				output.WriteString(result.Output)
			}
			output.WriteString("\n\n")

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
