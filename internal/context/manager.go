package context

import (
	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

const (
	// recentMessagesToKeep is the number of recent messages preserved verbatim
	// after compaction (kept alongside the LLM-generated summary).
	recentMessagesToKeep = 20

	// toolOutputTruncateLen is the maximum bytes kept for tool result content
	// in messages that fall outside the recent window during pruning.
	toolOutputTruncateLen = 200
)

// Manager handles context management and compaction
type Manager struct {
	config *config.ContextConfig
}

// NewManager creates a new context manager
func NewManager(cfg *config.ContextConfig) *Manager {
	return &Manager{
		config: cfg,
	}
}

// GetTokenUsage estimates the token usage of messages using ~4 chars per token.
func (m *Manager) GetTokenUsage(messages []api.Message) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			totalChars += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
	}
	return totalChars / 4
}

// CompactionStatus returns the current estimated token usage, the compaction
// threshold, and whether auto-compact is enabled.
func (m *Manager) CompactionStatus(messages []api.Message) (current, threshold int, autoCompact bool) {
	current = m.GetTokenUsage(messages)
	threshold = int(float32(m.config.MaxTokens) * m.config.CompactionRatio)
	autoCompact = m.config.AutoCompact
	return
}

// NeedsCompaction reports whether the context should be compacted.
func (m *Manager) NeedsCompaction(messages []api.Message) bool {
	if !m.config.AutoCompact {
		return false
	}
	usage := m.GetTokenUsage(messages)
	threshold := int(float32(m.config.MaxTokens) * m.config.CompactionRatio)
	return usage > threshold || len(messages) > m.config.MaxMessages
}

// Prune reduces token usage by truncating large tool outputs in messages
// outside the recent window without removing messages or breaking message
// pairing. This is a cheap pre-step before LLM-based summarization.
func (m *Manager) Prune(messages []api.Message) []api.Message {
	if len(messages) <= recentMessagesToKeep {
		return messages
	}
	cutoff := len(messages) - recentMessagesToKeep
	result := make([]api.Message, len(messages))
	copy(result, messages)
	for i := 0; i < cutoff; i++ {
		if result[i].Role == "tool" && len(result[i].Content) > toolOutputTruncateLen {
			result[i].Content = result[i].Content[:toolOutputTruncateLen] + "\n[truncated]"
		}
	}
	return result
}

// BuildCompacted assembles a new message list from an LLM-generated summary
// and the most recent messages. Initial system messages are always kept.
func (m *Manager) BuildCompacted(summary string, original []api.Message) []api.Message {
	var result []api.Message

	// Keep leading system messages (instructions, context, etc.)
	i := 0
	for i < len(original) && original[i].Role == "system" {
		result = append(result, original[i])
		i++
	}

	// Insert the LLM-generated summary as a system message.
	result = append(result, api.NewSystemMessage(
		"The following is a summary of the conversation so far:\n\n"+summary,
	))

	// Append the most recent messages verbatim, starting at a user-message
	// boundary so the API always receives well-formed turn pairs.
	recent := safeRecentMessages(original, recentMessagesToKeep)
	result = append(result, recent...)

	return result
}

// safeRecentMessages returns up to n recent messages, adjusted to start at
// a user-message boundary so tool_call / tool pairs are never split.
func safeRecentMessages(messages []api.Message, n int) []api.Message {
	if len(messages) <= n {
		return messages
	}
	start := len(messages) - n
	// Walk forward until we land on a user message.
	for start < len(messages) && messages[start].Role != "user" {
		start++
	}
	return messages[start:]
}
