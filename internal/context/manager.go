package context

import (
	"encoding/json"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
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

// GetTokenUsage estimates the token usage of messages
func (m *Manager) GetTokenUsage(messages []api.Message) int {
	// Simple estimation: ~4 characters per token
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)

		// Add tool calls
		if len(msg.ToolCalls) > 0 {
			data, _ := json.Marshal(msg.ToolCalls)
			totalChars += len(data)
		}
	}

	return totalChars / 4
}

// NeedsCompaction checks if context needs compaction
func (m *Manager) NeedsCompaction(messages []api.Message) bool {
	if !m.config.AutoCompact {
		return false
	}

	usage := m.GetTokenUsage(messages)
	threshold := int(float32(m.config.MaxTokens) * m.config.CompactionRatio)

	return usage > threshold || len(messages) > m.config.MaxMessages
}

// Compact reduces the context size while preserving important information
func (m *Manager) Compact(messages []api.Message) []api.Message {
	if len(messages) <= 3 {
		// Keep at least system message, one user message, and one assistant message
		return messages
	}

	// Always keep the system message
	compacted := []api.Message{messages[0]}

	// Strategy 1: Summarize old messages
	summarized := m.summarizeMessages(messages[1 : len(messages)/2])
	if summarized != nil {
		compacted = append(compacted, *summarized)
	}

	// Strategy 2: Keep recent messages
	recentStart := len(messages) / 2
	if recentStart < len(messages)-m.config.MaxMessages/2 {
		recentStart = len(messages) - m.config.MaxMessages/2
	}

	for i := recentStart; i < len(messages); i++ {
		// Skip tool messages in compacted history
		if messages[i].Role != "tool" {
			compacted = append(compacted, messages[i])
		}
	}

	return compacted
}

// summarizeMessages creates a summary of multiple messages
func (m *Manager) summarizeMessages(messages []api.Message) *api.Message {
	if len(messages) == 0 {
		return nil
	}

	var summary strings.Builder
	summary.WriteString("Previous conversation summary:\n")

	userMsgCount := 0
	assistantMsgCount := 0
	toolCallCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userMsgCount++
			if userMsgCount <= 3 {
				// Include first few user messages
				summary.WriteString("- User: ")
				if len(msg.Content) > 100 {
					summary.WriteString(msg.Content[:100] + "...")
				} else {
					summary.WriteString(msg.Content)
				}
				summary.WriteString("\n")
			}

		case "assistant":
			assistantMsgCount++
			if len(msg.ToolCalls) > 0 {
				toolCallCount += len(msg.ToolCalls)
			}

		case "tool":
			// Skip tool results in summary
		}
	}

	// Add statistics
	summary.WriteString("\n")
	summary.WriteString("Summary statistics:\n")
	summary.WriteString("- ")
	summary.WriteString(strings.Join([]string{
		intToString(userMsgCount) + " user messages",
		intToString(assistantMsgCount) + " assistant responses",
		intToString(toolCallCount) + " tool calls executed",
	}, ", "))

	return &api.Message{
		Role:    "system",
		Content: summary.String(),
	}
}

// CompactWithStrategy applies a specific compaction strategy
func (m *Manager) CompactWithStrategy(messages []api.Message, strategy string) []api.Message {
	switch strategy {
	case "aggressive":
		// Keep only system message and last few exchanges
		if len(messages) <= 5 {
			return messages
		}
		compacted := []api.Message{messages[0]}
		compacted = append(compacted, messages[len(messages)-4:]...)
		return compacted

	case "smart":
		// Keep important messages (with tool calls, errors, etc.)
		compacted := []api.Message{messages[0]}
		for i := 1; i < len(messages); i++ {
			msg := messages[i]

			// Keep messages with tool calls
			if len(msg.ToolCalls) > 0 {
				compacted = append(compacted, msg)
				// Also keep the tool result
				if i+1 < len(messages) && messages[i+1].Role == "tool" {
					compacted = append(compacted, messages[i+1])
					i++
				}
			} else if strings.Contains(strings.ToLower(msg.Content), "error") ||
				strings.Contains(strings.ToLower(msg.Content), "important") {
				// Keep error messages and important content
				compacted = append(compacted, msg)
			} else if i >= len(messages)-10 {
				// Keep recent messages
				compacted = append(compacted, msg)
			}
		}
		return compacted

	default:
		// Default strategy
		return m.Compact(messages)
	}
}

// GetContextWindow returns the current context window size
func (m *Manager) GetContextWindow() int {
	return m.config.MaxTokens
}

// GetMessageLimit returns the maximum number of messages
func (m *Manager) GetMessageLimit() int {
	return m.config.MaxMessages
}

// ShouldAutoCompact returns whether auto-compaction is enabled
func (m *Manager) ShouldAutoCompact() bool {
	return m.config.AutoCompact
}

// Helper function to convert int to string
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	result := ""
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digit := n % 10
		result = string(rune('0'+digit)) + result
		n /= 10
	}

	if negative {
		result = "-" + result
	}

	return result
}
