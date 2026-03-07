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
	appcontext "github.com/tokuhirom/ashron/internal/context"
)

type toolExecutionMsg struct {
	results   []api.ToolResult
	hasMore   bool
	moreTools bool // true when more pending tool calls remain after this one
	output    string
}

type errorMsg struct {
	error error
}

// StreamOutput represents the complete assistant response ready for display.
type StreamOutput struct {
	// AssistantText is the AI response text with <think> blocks stripped by
	// thinkingFilter.  It is rendered through glamour for Markdown display.
	AssistantText string
	// ToolLines contains pre-styled tool call summary lines (one entry per line).
	ToolLines []string
	Usage     *api.Usage
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

	// Estimate input size from current messages for real-time token display.
	var promptChars int
	for _, msg := range m.messages {
		promptChars += len(msg.Content)
	}
	m.streamingPromptChars = promptChars

	// Add user message to display content
	m.AddDisplayContent(userMsg, "")

	// Return a command that processes the message, with a heartbeat tick to
	// keep the display refreshed during the long-running streaming phase.
	return tea.Batch(m.processMessage(), loadingTick())
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
		// Staged context management: prune at 80%, summarize at 90%.
		level := m.contextMgr.CompactionLevel(m.messages)
		if level == appcontext.CompactionPrune {
			slog.Info("Pruning context (lightweight)", slog.Int("messages", len(m.messages)))
			m.messages = m.contextMgr.Prune(m.messages)
			// Re-check: if pruning didn't bring us below threshold, escalate.
			if m.contextMgr.CompactionLevel(m.messages) >= appcontext.CompactionPrune {
				slog.Info("Pruning insufficient, escalating to summarization")
				level = appcontext.CompactionSummarize
			}
		}
		if level == appcontext.CompactionSummarize {
			slog.Info("Summarizing context", slog.Int("beforeMessages", len(m.messages)))
			pruned := m.contextMgr.Prune(m.messages)
			summary, err := m.apiClient.Summarize(ctx, pruned)
			if err != nil {
				slog.Warn("Context summarization failed, keeping pruned messages", slog.Any("error", err))
				m.messages = pruned
			} else {
				m.messages = m.contextMgr.BuildCompacted(summary, m.messages)
			}
			slog.Info("Context compacted", slog.Int("afterMessages", len(m.messages)))
		}

		// Stream the response
		builtinTools := tools.SelectBuiltinTools(m.lastUserInput)
		msgsToSend := stubOldToolResults(m.messages)
		slog.Debug("Requesting streaming completion",
			slog.Int("messages", len(msgsToSend)),
			slog.Int("tools", len(builtinTools)))
		stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, msgsToSend, builtinTools)
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
	var toolCalls []api.ToolCall
	var toolLines []string // pre-styled tool call summary lines for display
	var chunkCount int
	var usage *api.Usage
	// thinkingFilter strips <think>...</think> blocks from the history while
	// keeping them in the display output.  See thinking_filter.go for details.
	var tf thinkingFilter

	// Map to accumulate tool call arguments by index
	toolCallArgs := make(map[int]*strings.Builder)
	toolCallsByIndex := make(map[int]*api.ToolCall)

	slog.Debug("Starting to process stream")
	m.currentOperation = "Receiving AI response"
	m.streamingChars = 0

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
					// Run the chunk through the thinking filter.
					//   _            - display output is no longer used; glamour
					//                  renders the full text at stream end.
					//   historyChunk - written to message history (<think> blocks stripped)
					_, historyChunk := tf.Feed(choice.Delta.Content)

					fullContent.WriteString(historyChunk)
					m.streamingChars = fullContent.Len()
					slog.Debug("Received content chunk", "chunk", chunkCount, "contentLength", len(choice.Delta.Content), "totalLength", fullContent.Len())

					// Update message history incrementally so that partial content is
					// visible for context compaction even before the stream finishes.
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
						// New tool call starting.
						//
						// Some providers (e.g. GLM-4.7) include the complete arguments
						// in this first delta AND repeat them in subsequent continuation
						// deltas.  To avoid concatenating duplicates we do NOT write
						// start-delta arguments into toolCallArgs here.
						//
						// tc.Function.Arguments holds the start-delta args as a fallback:
						// if no continuation deltas arrive (provider sent everything
						// up-front), finalization will use this value directly.
						// If continuation deltas do arrive, their accumulated content
						// replaces tc.Function.Arguments at finalization time.
						tc := &api.ToolCall{
							ID:   deltaToolCall.ID,
							Type: deltaToolCall.Type,
							Function: api.FunctionCall{
								Name:      deltaToolCall.Function.Name,
								Arguments: deltaToolCall.Function.Arguments,
							},
						}
						toolCallsByIndex[idx] = tc
						toolCallArgs[idx] = &strings.Builder{} // always start fresh
					} else if deltaToolCall.Function.Arguments != "" {
						// Continuation delta: append arguments to the accumulator.
						// These are the chunks that build up the final arguments string.
						// For providers like OpenAI the start delta has empty args and
						// all content arrives here.  For providers like GLM-4.7 that
						// repeat the full args in the continuation, only these chunks
						// end up in toolCallArgs (start-delta args are kept separate in
						// tc.Function.Arguments as a fallback).
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

						// Collect compact tool summary lines for display.
						for _, line := range toolCallSummaryLines(*tc) {
							toolLines = append(toolLines, lipgloss.NewStyle().
								Foreground(lipgloss.Color("#626262")).
								Render(line))
						}

						slog.Debug("Finalized tool call",
							"id", tc.ID,
							"name", tc.Function.Name,
							"args", tc.Function.Arguments)
					}

					slog.Info("Stream finished", "reason", choice.FinishReason, "chunks", chunkCount, "contentLength", fullContent.Len(), "toolCalls", len(toolCalls))

					// Flush any bytes still held in the thinking filter carry buffer.
					// This handles the (unusual) case where the stream ends with a
					// partial tag like "<thi" that never resolved into a real tag.
					if _, flushHistory := tf.Flush(); flushHistory != "" {
						fullContent.WriteString(flushHistory)
					}

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
					return StreamOutput{AssistantText: fullContent.String(), ToolLines: toolLines, Usage: usage}
				}
			}
		}
	}

	// Stream ended without a proper finish (no stop/tool_calls reason received).
	// Flush any remaining carry from the thinking filter.
	if _, flushHistory := tf.Flush(); flushHistory != "" {
		fullContent.WriteString(flushHistory)
	}
	return StreamOutput{AssistantText: fullContent.String(), ToolLines: toolLines, Usage: usage}
}

// executeNextTool executes the first pending tool call and returns a
// toolExecutionMsg.  Tools are executed one at a time so that the TUI can
// update currentOperation with the name of each tool as it runs.
func (m *SimpleModel) executeNextTool() tea.Cmd {
	if len(m.pendingToolCalls) == 0 {
		return nil
	}

	// Pop the first pending tool call so the next iteration picks the next one.
	tc := m.pendingToolCalls[0]
	m.pendingToolCalls = m.pendingToolCalls[1:]
	moreTools := len(m.pendingToolCalls) > 0

	return func() tea.Msg {
		var output strings.Builder

		result := m.toolExec.Execute(tc)

		// Keep tool result display compact; detailed output remains in tool messages.
		if result.Error != nil {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7F50"))
			output.WriteString(errStyle.Render("tool error: " + tc.Function.Name))
			if tc.Function.Arguments != "" {
				output.WriteString(errStyle.Render(" " + tc.Function.Arguments))
			}
			output.WriteString("\n")
			output.WriteString(errStyle.Render("  → " + result.Error.Error()))
			output.WriteString("\n")
		}

		// Store the full output in the result store so get_tool_result can retrieve it.
		if m.toolResultStore != nil {
			m.toolResultStore.Store(tc.ID, result.Output)
		}
		// Keep tool outputs compact in message history to reduce prompt tokens.
		historyOutput := tools.CompactToolResultForHistory(tc.Function.Name, result.Output)
		m.messages = append(m.messages, api.NewToolMessage(tc.ID, historyOutput))

		return toolExecutionMsg{
			results:   []api.ToolResult{result},
			hasMore:   true, // always continue the conversation after tool execution
			moreTools: moreTools,
			output:    output.String(),
		}
	}
}

// recentToolResultWindow is the number of most recent tool results to keep
// in full when stubbing old tool results. Tool results within this window
// are preserved verbatim for better context quality and prefix cache hits.
const recentToolResultWindow = 10

// stubOldToolResults replaces tool message content with a lightweight reference
// for tool messages outside the recent window. The AI can retrieve the full
// content on demand via the get_tool_result tool.
//
// This implements observation masking: tool call metadata (name, arguments) is
// always preserved so the agent remembers what it did, while verbose outputs
// are replaced with stubs to reduce context noise.
func stubOldToolResults(messages []api.Message) []api.Message {
	// Collect indices of all tool messages in reverse order.
	var toolIndices []int
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			toolIndices = append(toolIndices, i)
		}
	}
	if len(toolIndices) == 0 {
		return messages
	}

	// Determine which tool messages are within the recent window.
	recentSet := make(map[int]bool, recentToolResultWindow)
	for i := 0; i < len(toolIndices) && i < recentToolResultWindow; i++ {
		recentSet[toolIndices[i]] = true
	}

	result := make([]api.Message, len(messages))
	copy(result, messages)
	for i, msg := range result {
		if msg.Role == "tool" && !recentSet[i] {
			result[i].Content = fmt.Sprintf("[stored: use get_tool_result with id=%q to retrieve full output]", msg.ToolCallID)
		}
	}
	return result
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
	if tc.Function.Name == "execute_command" {
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil && strings.TrimSpace(args.Command) != "" {
			return append(lines, "  └ $ "+strings.TrimSpace(args.Command))
		}
		return append(lines, "  └ execute_command")
	}
	if tc.Function.Name == "search_and_replace" {
		var args struct {
			Path    string `json:"path"`
			Search  string `json:"search"`
			Replace string `json:"replace"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || args.Path == "" {
			return append(lines, "  └ Search and replace")
		}
		return append(lines,
			"  └ Search and replace: "+args.Path,
			"    search: "+truncateForApproval(args.Search),
			"    replace: "+truncateForApproval(args.Replace),
			"    backup will be created before apply",
		)
	}
	if tc.Function.Name == "replace_range" {
		var args struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || args.Path == "" {
			return append(lines, "  └ Replace range")
		}
		return append(lines,
			"  └ Replace range: "+args.Path,
			fmt.Sprintf("    lines: %d-%d", args.StartLine, args.EndLine),
			"    backup will be created before apply",
		)
	}

	return append(lines, "  └ Used tool: "+tc.Function.Name)
}
