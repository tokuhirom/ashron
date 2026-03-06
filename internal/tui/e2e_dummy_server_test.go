package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func TestE2E_DummyServerSimpleStreamingReply(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return []api.StreamResponse{
			{
				Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "hello from dummy"}}},
			},
			{
				Choices: []api.Choice{{Index: 0, Delta: api.Message{}, FinishReason: "stop"}},
				Usage:   &api.Usage{PromptTokens: 12, CompletionTokens: 3, TotalTokens: 15},
			},
		}
	})
	defer server.Close()

	m := newE2EModel(t, server.URL)
	cmd := m.SendMessage("say hi")
	if err := runCommandLoop(m, cmd, 10); err != nil {
		t.Fatalf("run command loop: %v", err)
	}

	if len(m.messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(m.messages))
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" {
		t.Fatalf("last role = %s, want assistant", last.Role)
	}
	if !strings.Contains(last.Content, "hello from dummy") {
		t.Fatalf("unexpected assistant content: %q", last.Content)
	}
	if m.currentUsage == nil || m.currentUsage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %#v", m.currentUsage)
	}
}

func TestE2E_DummyServerToolRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "note.txt")
	if err := os.WriteFile(filePath, []byte("dummy file content\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	server := newDummyChatServer(t, func(call int, req api.ChatCompletionRequest) []api.StreamResponse {
		switch call {
		case 0:
			args, _ := json.Marshal(map[string]string{"path": filePath})
			return []api.StreamResponse{
				{
					Choices: []api.Choice{{
						Index: 0,
						Delta: api.Message{ToolCalls: []api.ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: api.FunctionCall{
								Name:      "read_file",
								Arguments: string(args),
							},
						}}},
						FinishReason: "tool_calls",
					}},
				},
			}
		case 1:
			hasToolMsg := false
			for _, m := range req.Messages {
				if m.Role == "tool" && m.ToolCallID == "call_1" && strings.Contains(m.Content, "dummy file content") {
					hasToolMsg = true
				}
			}
			if !hasToolMsg {
				t.Fatalf("second request missing expected tool message")
			}
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "file read complete"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		default:
			t.Fatalf("unexpected call index: %d", call)
			return nil
		}
	})
	defer server.Close()

	m := newE2EModel(t, server.URL)
	cmd := m.SendMessage("please read the file")
	if err := runCommandLoop(m, cmd, 20); err != nil {
		t.Fatalf("run command loop: %v", err)
	}

	if len(m.messages) == 0 {
		t.Fatalf("expected messages to be populated")
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "file read complete") {
		t.Fatalf("unexpected final assistant message: %#v", last)
	}
}

// TestE2E_ParallelToolCalls verifies that two tool calls made in the same
// streaming response (parallel tool calls) are tracked independently using the
// Index field from the streaming delta. Previously, both mapped to idx=0 which
// caused their arguments to be concatenated, producing invalid JSON like
// {"path":"."} {"path":"internal"} → "Extra data: line 1 column 22" from GLM-4.7.
func TestE2E_ParallelToolCalls(t *testing.T) {
	tmp := t.TempDir()
	fileA := filepath.Join(tmp, "a.txt")
	fileB := filepath.Join(tmp, "b.txt")
	if err := os.WriteFile(fileA, []byte("content-a\n"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("content-b\n"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	argsA, _ := json.Marshal(map[string]string{"path": fileA})
	argsB, _ := json.Marshal(map[string]string{"path": fileB})

	server := newDummyChatServer(t, func(call int, req api.ChatCompletionRequest) []api.StreamResponse {
		switch call {
		case 0:
			// Return two parallel tool calls, each in a separate streaming delta
			// using the Index field to distinguish them (index 0 and index 1).
			return []api.StreamResponse{
				// Start call_a (index 0)
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 0, ID: "call_a", Type: "function",
					Function: api.FunctionCall{Name: "read_file"},
				}}}}}},
				// Stream args for call_a
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 0, Function: api.FunctionCall{Arguments: string(argsA)},
				}}}}}},
				// Start call_b (index 1)
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 1, ID: "call_b", Type: "function",
					Function: api.FunctionCall{Name: "read_file"},
				}}}}}},
				// Stream args for call_b
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 1, Function: api.FunctionCall{Arguments: string(argsB)},
				}}}}}},
				// Finish
				{Choices: []api.Choice{{Index: 0, FinishReason: "tool_calls"}}},
			}
		case 1:
			// Verify both tool results are present
			var gotA, gotB bool
			for _, msg := range req.Messages {
				if msg.Role == "tool" && msg.ToolCallID == "call_a" && strings.Contains(msg.Content, "content-a") {
					gotA = true
				}
				if msg.Role == "tool" && msg.ToolCallID == "call_b" && strings.Contains(msg.Content, "content-b") {
					gotB = true
				}
			}
			if !gotA {
				t.Fatalf("missing tool result for call_a")
			}
			if !gotB {
				t.Fatalf("missing tool result for call_b")
			}
			// Also verify the assistant message's tool_call arguments are valid JSON
			for _, msg := range req.Messages {
				if msg.Role == "assistant" {
					for _, tc := range msg.ToolCalls {
						var v map[string]string
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &v); err != nil {
							t.Fatalf("tool call %s has invalid JSON arguments %q: %v", tc.ID, tc.Function.Arguments, err)
						}
					}
				}
			}
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "both files read"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		default:
			t.Fatalf("unexpected call index: %d", call)
			return nil
		}
	})
	defer server.Close()

	m := newE2EModel(t, server.URL)
	cmd := m.SendMessage("read both files")
	if err := runCommandLoop(m, cmd, 30); err != nil {
		t.Fatalf("run command loop: %v", err)
	}

	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "both files read") {
		t.Fatalf("unexpected final message: %#v", last)
	}
}

// TestE2E_ToolArgsDuplicatedInStartDelta verifies that when a provider (e.g.
// GLM-4.7) sends complete arguments in the first (ID-bearing) delta AND ALSO
// repeats those same arguments in a subsequent continuation delta, the arguments
// are not concatenated.  Without the fix, the result would be:
//
//	{"path":"README.md"}{"path":"README.md"}  → invalid JSON → tool error
//	→ API 400 "Extra data: line 1 column 21 (char 20)" on the next turn.
func TestE2E_ToolArgsDuplicatedInStartDelta(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(filePath, []byte("readme content\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	args, _ := json.Marshal(map[string]string{"path": filePath})

	server := newDummyChatServer(t, func(call int, req api.ChatCompletionRequest) []api.StreamResponse {
		switch call {
		case 0:
			return []api.StreamResponse{
				// Start delta: ID present + complete arguments (GLM-4.7 style)
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 0, ID: "call_1", Type: "function",
					Function: api.FunctionCall{Name: "read_file", Arguments: string(args)},
				}}}}}},
				// Continuation delta: same complete arguments repeated (GLM-4.7 quirk)
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{ToolCalls: []api.ToolCall{{
					Index: 0, Function: api.FunctionCall{Arguments: string(args)},
				}}}}}},
				// Finish
				{Choices: []api.Choice{{Index: 0, FinishReason: "tool_calls"}}},
			}
		case 1:
			// Verify the tool result message has valid content (file was read correctly).
			for _, msg := range req.Messages {
				if msg.Role == "tool" && msg.ToolCallID == "call_1" {
					if !strings.Contains(msg.Content, "readme content") {
						t.Fatalf("tool result has unexpected content: %q", msg.Content)
					}
					// Also verify the assistant message's tool_call args are valid JSON.
					for _, m := range req.Messages {
						if m.Role == "assistant" {
							for _, tc := range m.ToolCalls {
								var v map[string]string
								if err := json.Unmarshal([]byte(tc.Function.Arguments), &v); err != nil {
									t.Fatalf("tool call %s has invalid JSON args %q: %v", tc.ID, tc.Function.Arguments, err)
								}
							}
						}
					}
				}
			}
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "file read ok"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		default:
			t.Fatalf("unexpected call index: %d", call)
			return nil
		}
	})
	defer server.Close()

	m := newE2EModel(t, server.URL)
	cmd := m.SendMessage("read the readme")
	if err := runCommandLoop(m, cmd, 20); err != nil {
		t.Fatalf("run command loop: %v", err)
	}

	last := m.messages[len(m.messages)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "file read ok") {
		t.Fatalf("unexpected final message: %#v", last)
	}
}

func newE2EModel(t *testing.T, baseURL string) *SimpleModel {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := &config.Config{
		Default: config.DefaultConfig{Provider: "dummy", Model: "dummy-model"},
		Providers: map[string]config.ProviderConfig{
			"dummy": {
				Type:    "openai-compat",
				BaseURL: baseURL,
				APIKey:  "dummy-key",
				Timeout: 5 * time.Second,
				Models: map[string]config.ModelConfig{
					"dummy-model": {Model: "dummy-model", Temperature: 0},
				},
			},
		},
		Tools: config.ToolsConfig{
			AutoApproveTools: []string{"read_file", "list_directory", "list_tools"},
			MaxOutputSize:    1024 * 1024,
			CommandTimeout:   time.Second,
			SandboxMode:      "auto",
		},
		DefaultContext: config.ContextConfig{
			MaxMessages:     50,
			MaxTokens:       4096,
			CompactionRatio: 0.9,
			AutoCompact:     true,
		},
	}

	m, err := NewSimpleModel(cfg, nil)
	if err != nil {
		t.Fatalf("NewSimpleModel: %v", err)
	}
	// Existing E2E scenarios use temp files outside repo cwd; keep them focused on
	// tool round-trip behavior rather than workspace permission checks.
	m.workspaceRoot = "/"
	return m
}

func runCommandLoop(m *SimpleModel, cmd tea.Cmd, maxSteps int) error {
	queue := []tea.Cmd{cmd}
	for step := 0; step < maxSteps && len(queue) > 0; step++ {
		// Pop the first command.
		cmd, queue = queue[0], queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		// Expand BatchMsg into the queue so all commands are processed.
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, []tea.Cmd(batch)...)
			step-- // don't count expansion as a step
			continue
		}
		_, next := m.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
	return nil
}

func newDummyChatServer(t *testing.T, replyFn func(call int, req api.ChatCompletionRequest) []api.StreamResponse) *httptest.Server {
	t.Helper()

	var (
		mu    sync.Mutex
		calls int
	)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req api.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		mu.Lock()
		call := calls
		calls++
		mu.Unlock()

		chunks := replyFn(call, req)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		for _, chunk := range chunks {
			b, err := json.Marshal(chunk)
			if err != nil {
				t.Fatalf("marshal chunk: %v", err)
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
}
