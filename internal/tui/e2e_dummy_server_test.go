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
		Context: config.ContextConfig{
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
	return m
}

func runCommandLoop(m *SimpleModel, cmd tea.Cmd, maxSteps int) error {
	for step := 0; step < maxSteps && cmd != nil; step++ {
		msg := cmd()
		if msg == nil {
			return nil
		}
		_, next := m.Update(msg)
		cmd = next
	}
	if cmd != nil {
		return fmt.Errorf("command loop exceeded %d steps", maxSteps)
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
