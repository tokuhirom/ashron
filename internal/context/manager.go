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

	// pruneRatio is the fraction of MaxTokens at which lightweight pruning
	// (tool output truncation) kicks in. This is the first line of defense.
	pruneRatio = 0.80

	// summarizeRatio is the fraction of MaxTokens at which full LLM-based
	// summarization is triggered. This is the last resort.
	summarizeRatio = 0.90
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

// CompactionStatus returns the current estimated token usage, the summarization
// threshold, and whether auto-compact is enabled.
func (m *Manager) CompactionStatus(messages []api.Message) (current, threshold int, autoCompact bool) {
	current = m.GetTokenUsage(messages)
	// Report the summarization threshold (the point of full compaction).
	sumThreshold := float32(summarizeRatio)
	if m.config.CompactionRatio > 0 && m.config.CompactionRatio < sumThreshold {
		sumThreshold = m.config.CompactionRatio
	}
	threshold = int(sumThreshold * float32(m.config.MaxTokens))
	autoCompact = m.config.AutoCompact
	return
}

// CompactionLevel indicates which stage of context management is needed.
type CompactionLevel int

const (
	// CompactionNone means no action needed.
	CompactionNone CompactionLevel = iota
	// CompactionPrune means lightweight pruning (tool output truncation) is needed.
	CompactionPrune
	// CompactionSummarize means full LLM summarization is needed.
	CompactionSummarize
)

// NeedsCompaction reports whether the context should be compacted.
// Kept for backward compatibility — returns true when any compaction is needed.
func (m *Manager) NeedsCompaction(messages []api.Message) bool {
	return m.CompactionLevel(messages) >= CompactionSummarize
}

// NeedsPruning reports whether the context needs at least lightweight pruning.
func (m *Manager) NeedsPruning(messages []api.Message) bool {
	return m.CompactionLevel(messages) >= CompactionPrune
}

// CompactionLevel returns the recommended compaction stage for the current context.
func (m *Manager) CompactionLevel(messages []api.Message) CompactionLevel {
	if !m.config.AutoCompact {
		return CompactionNone
	}
	usage := m.GetTokenUsage(messages)
	maxTokens := m.config.MaxTokens

	// Message count overflow triggers full summarization.
	if len(messages) > m.config.MaxMessages {
		return CompactionSummarize
	}

	// Use the configured CompactionRatio as the summarization threshold if it
	// is set below our default summarizeRatio (for backward compatibility).
	sumThreshold := float32(summarizeRatio)
	if m.config.CompactionRatio > 0 && m.config.CompactionRatio < sumThreshold {
		sumThreshold = m.config.CompactionRatio
	}

	if usage > int(sumThreshold*float32(maxTokens)) {
		return CompactionSummarize
	}
	if usage > int(pruneRatio*float32(maxTokens)) {
		return CompactionPrune
	}
	return CompactionNone
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
