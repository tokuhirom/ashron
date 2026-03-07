package tui

import (
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
)

func TestStubOldToolResults_SingleToolKeptFull(t *testing.T) {
	t.Parallel()

	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "full content"},
	}

	got := stubOldToolResults(msgs)
	// Only 1 tool message, within the recent window → content must be unchanged.
	if got[2].Content != "full content" {
		t.Fatalf("expected full content, got %q", got[2].Content)
	}
}

func TestStubOldToolResults_SingleToolWithAssistantKeptFull(t *testing.T) {
	t.Parallel()

	// Even with an assistant after, a single tool message is within the
	// recent window (10) so it should be kept full.
	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "full content"},
		{Role: "assistant", Content: "done"},
	}

	got := stubOldToolResults(msgs)
	if got[2].Content != "full content" {
		t.Fatalf("expected full content within recent window, got %q", got[2].Content)
	}
}

func TestStubOldToolResults_OriginalUnmodified(t *testing.T) {
	t.Parallel()

	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "full content"},
		{Role: "assistant", Content: "done"},
	}

	_ = stubOldToolResults(msgs)
	// Original slice must not be modified.
	if msgs[2].Content != "full content" {
		t.Fatalf("original slice was mutated: %q", msgs[2].Content)
	}
}

func TestStubOldToolResults_MultiTurnWithinWindow(t *testing.T) {
	t.Parallel()

	// Both tool results are within the recent window (10) → both kept full
	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "result A"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c2"}}},
		{Role: "tool", ToolCallID: "c2", Content: "result B"},
	}

	got := stubOldToolResults(msgs)
	if got[2].Content != "result A" {
		t.Fatalf("tool A should be kept full (within window), got %q", got[2].Content)
	}
	if got[4].Content != "result B" {
		t.Fatalf("tool B should be kept full, got %q", got[4].Content)
	}
}

func TestStubOldToolResults_OldToolsOutsideWindow(t *testing.T) {
	t.Parallel()

	// Create more than recentToolResultWindow (10) tool messages.
	// The oldest ones should be stubbed.
	var msgs []api.Message
	msgs = append(msgs, api.Message{Role: "user", Content: "go"})
	for i := 0; i < 12; i++ {
		id := "c" + strings.Repeat("x", i+1) // unique IDs
		msgs = append(msgs,
			api.Message{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: id}}},
			api.Message{Role: "tool", ToolCallID: id, Content: "result " + id},
		)
	}

	got := stubOldToolResults(msgs)

	// First 2 tool results (indices 2, 4) should be stubbed (outside window of 10).
	for _, idx := range []int{2, 4} {
		if !strings.Contains(got[idx].Content, "get_tool_result") {
			t.Fatalf("tool at index %d should be stubbed, got %q", idx, got[idx].Content)
		}
	}

	// Last tool result should be kept full.
	lastToolIdx := len(got) - 1
	if strings.Contains(got[lastToolIdx].Content, "get_tool_result") {
		t.Fatalf("last tool should be kept full, got %q", got[lastToolIdx].Content)
	}
}
