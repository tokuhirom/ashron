package tui

import (
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
)

func TestStubOldToolResults_NoAssistantAfter(t *testing.T) {
	t.Parallel()

	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "full content"},
	}

	got := stubOldToolResults(msgs)
	// No assistant message after the tool message → content must be unchanged.
	if got[2].Content != "full content" {
		t.Fatalf("expected full content, got %q", got[2].Content)
	}
}

func TestStubOldToolResults_AssistantAfterToolIsStubbed(t *testing.T) {
	t.Parallel()

	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "full content"},
		{Role: "assistant", Content: "done"},
	}

	got := stubOldToolResults(msgs)
	if !strings.Contains(got[2].Content, "get_tool_result") {
		t.Fatalf("expected stub, got %q", got[2].Content)
	}
	if strings.Contains(got[2].Content, "full content") {
		t.Fatalf("stub must not contain original content: %q", got[2].Content)
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

func TestStubOldToolResults_MultiTurn(t *testing.T) {
	t.Parallel()

	// tool(A) has assistant after → stubbed
	// tool(B) is the latest (no assistant after) → kept full
	msgs := []api.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c1"}}},
		{Role: "tool", ToolCallID: "c1", Content: "result A"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{ID: "c2"}}},
		{Role: "tool", ToolCallID: "c2", Content: "result B"},
	}

	got := stubOldToolResults(msgs)
	if !strings.Contains(got[2].Content, "get_tool_result") {
		t.Fatalf("tool A should be stubbed, got %q", got[2].Content)
	}
	if got[4].Content != "result B" {
		t.Fatalf("tool B should be full, got %q", got[4].Content)
	}
}
