package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// setupSubagentTestServer creates a dummy streaming server and configures
// the package-level subagent runtime so that SpawnSubagent / WaitSubagent /
// etc. work end-to-end through the tool functions.
func setupSubagentTestServer(t *testing.T, replyFn func(call int, req api.ChatCompletionRequest) []api.StreamResponse, chunkDelay time.Duration) *httptest.Server {
	t.Helper()

	var (
		mu    sync.Mutex
		calls int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			b, err := json.Marshal(chunk)
			if err != nil {
				t.Fatalf("marshal chunk: %v", err)
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			if flusher != nil {
				flusher.Flush()
			}
			if chunkDelay > 0 {
				time.Sleep(chunkDelay)
			}
		}
		io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))

	provider := &config.ProviderConfig{
		Type:    "openai-compat",
		BaseURL: server.URL,
		APIKey:  "dummy",
		Timeout: 5 * time.Second,
	}
	model := &config.ModelConfig{Model: "test-model", Temperature: 0}
	ctxCfg := &config.ContextConfig{MaxMessages: 50, MaxTokens: 4096, CompactionRatio: 0.9, AutoCompact: true}
	client := api.NewClient(provider, model, ctxCfg)

	ConfigureSubagentRuntime(client, ctxCfg)
	t.Cleanup(func() {
		subagentMu.Lock()
		subagentManager = nil
		subagentMu.Unlock()
		server.Close()
	})

	return server
}

// TestSubagentToolsIntegration_SpawnAndWait tests the full path:
// SpawnSubagent (JSON args) -> server streams response -> WaitSubagent (JSON args) -> JSON result
func TestSubagentToolsIntegration_SpawnAndWait(t *testing.T) {
	setupSubagentTestServer(t, func(call int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return []api.StreamResponse{
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "hello from subagent"}}}},
			{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
		}
	}, 0)

	// Spawn via tool function
	spawnResult := SpawnSubagent(nil, "tc-1", `{"prompt":"do something"}`)
	if spawnResult.Error != nil {
		t.Fatalf("SpawnSubagent error: %v", spawnResult.Error)
	}
	var spawnOut struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(spawnResult.Output), &spawnOut); err != nil {
		t.Fatalf("SpawnSubagent output parse error: %v (output: %s)", err, spawnResult.Output)
	}
	if spawnOut.ID == "" || spawnOut.Status != "running" {
		t.Fatalf("unexpected spawn output: %+v", spawnOut)
	}

	// Wait via tool function
	waitArgs := fmt.Sprintf(`{"id":"%s","timeout_seconds":5}`, spawnOut.ID)
	waitResult := WaitSubagent(nil, "tc-2", waitArgs)
	if waitResult.Error != nil {
		t.Fatalf("WaitSubagent error: %v", waitResult.Error)
	}
	var waitOut struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		TimedOut bool   `json:"timed_out"`
		Output   string `json:"output"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal([]byte(waitResult.Output), &waitOut); err != nil {
		t.Fatalf("WaitSubagent output parse error: %v (output: %s)", err, waitResult.Output)
	}
	if waitOut.Status != "completed" {
		t.Fatalf("expected completed, got %s", waitOut.Status)
	}
	if waitOut.TimedOut {
		t.Fatalf("expected no timeout")
	}
	if waitOut.Output != "hello from subagent" {
		t.Fatalf("unexpected output: %q", waitOut.Output)
	}
}

// TestSubagentToolsIntegration_WaitTimeout tests that WaitSubagent returns timed_out=true
// when the subagent takes longer than the specified timeout.
func TestSubagentToolsIntegration_WaitTimeout(t *testing.T) {
	setupSubagentTestServer(t, func(call int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return []api.StreamResponse{
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "slow response"}}}},
			{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
		}
	}, 200*time.Millisecond) // slow chunks

	// Setup with long delay so timeout triggers
	subagentMu.Lock()
	subagentManager = nil
	subagentMu.Unlock()

	setupSubagentTestServer(t, func(call int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return []api.StreamResponse{
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "chunk1"}}}},
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "chunk2"}}}},
			{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
		}
	}, 2*time.Second) // each chunk takes 2s → total ~4s

	spawnResult2 := SpawnSubagent(nil, "tc-3", `{"prompt":"very slow"}`)
	if spawnResult2.Error != nil {
		t.Fatalf("SpawnSubagent error: %v", spawnResult2.Error)
	}
	var spawnOut2 struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(spawnResult2.Output), &spawnOut2)

	waitArgs2 := fmt.Sprintf(`{"id":"%s","timeout_seconds":1}`, spawnOut2.ID)
	waitResult := WaitSubagent(nil, "tc-4", waitArgs2)
	if waitResult.Error != nil {
		t.Fatalf("WaitSubagent error: %v", waitResult.Error)
	}
	var waitOut struct {
		TimedOut bool   `json:"timed_out"`
		Status  string `json:"status"`
	}
	json.Unmarshal([]byte(waitResult.Output), &waitOut)
	if !waitOut.TimedOut {
		t.Fatalf("expected timeout, got timed_out=false")
	}
	if waitOut.Status != "running" {
		t.Fatalf("expected running during timeout, got %s", waitOut.Status)
	}
}

// TestSubagentToolsIntegration_WaitNotFound tests WaitSubagent with invalid ID.
func TestSubagentToolsIntegration_WaitNotFound(t *testing.T) {
	setupSubagentTestServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return nil
	}, 0)

	result := WaitSubagent(nil, "tc-1", `{"id":"nonexistent"}`)
	if result.Error == nil {
		t.Fatalf("expected error for nonexistent subagent")
	}
}

// TestSubagentToolsIntegration_WaitInvalidArgs tests WaitSubagent with malformed JSON.
func TestSubagentToolsIntegration_WaitInvalidArgs(t *testing.T) {
	setupSubagentTestServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse {
		return nil
	}, 0)

	result := WaitSubagent(nil, "tc-1", `{invalid json}`)
	if result.Error == nil {
		t.Fatalf("expected error for invalid JSON args")
	}
}

// TestSubagentToolsIntegration_NoRuntime tests all tool functions when runtime is not configured.
func TestSubagentToolsIntegration_NoRuntime(t *testing.T) {
	subagentMu.Lock()
	orig := subagentManager
	subagentManager = nil
	subagentMu.Unlock()
	defer func() {
		subagentMu.Lock()
		subagentManager = orig
		subagentMu.Unlock()
	}()

	result := WaitSubagent(nil, "tc-1", `{"id":"any"}`)
	if result.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}

	spawnResult := SpawnSubagent(nil, "tc-2", `{"prompt":"test"}`)
	if spawnResult.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}

	listResult := ListSubagents(nil, "tc-3", `{}`)
	if listResult.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}

	closeResult := CloseSubagent(nil, "tc-4", `{"id":"any"}`)
	if closeResult.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}

	logResult := GetSubagentLogTool(nil, "tc-5", `{"id":"any"}`)
	if logResult.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}

	inputResult := SendSubagentInput(nil, "tc-6", `{"id":"any","input":"test"}`)
	if inputResult.Error == nil {
		t.Fatalf("expected error when runtime not configured")
	}
}

// TestSubagentToolsIntegration_FullLifecycle tests spawn -> wait -> send_input -> wait -> close.
func TestSubagentToolsIntegration_FullLifecycle(t *testing.T) {
	setupSubagentTestServer(t, func(call int, _ api.ChatCompletionRequest) []api.StreamResponse {
		switch call {
		case 0:
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "first reply"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		case 1:
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "second reply"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		default:
			t.Fatalf("unexpected call: %d", call)
			return nil
		}
	}, 0)

	// 1. Spawn
	spawnResult := SpawnSubagent(nil, "tc-1", `{"prompt":"hello"}`)
	if spawnResult.Error != nil {
		t.Fatalf("Spawn error: %v", spawnResult.Error)
	}
	var spawnOut struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(spawnResult.Output), &spawnOut)

	// 2. Wait for completion
	waitResult := WaitSubagent(nil, "tc-2", fmt.Sprintf(`{"id":"%s","timeout_seconds":5}`, spawnOut.ID))
	if waitResult.Error != nil {
		t.Fatalf("Wait error: %v", waitResult.Error)
	}
	var waitOut struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	json.Unmarshal([]byte(waitResult.Output), &waitOut)
	if waitOut.Status != "completed" || waitOut.Output != "first reply" {
		t.Fatalf("unexpected wait output: %+v", waitOut)
	}

	// 3. List — should show the agent
	listResult := ListSubagents(nil, "tc-3", `{}`)
	if listResult.Error != nil {
		t.Fatalf("List error: %v", listResult.Error)
	}
	var listOut []map[string]interface{}
	json.Unmarshal([]byte(listResult.Output), &listOut)
	if len(listOut) != 1 {
		t.Fatalf("expected 1 agent in list, got %d", len(listOut))
	}

	// 4. Get log
	logResult := GetSubagentLogTool(nil, "tc-4", fmt.Sprintf(`{"id":"%s"}`, spawnOut.ID))
	if logResult.Error != nil {
		t.Fatalf("GetLog error: %v", logResult.Error)
	}
	if logResult.Output != "first reply" {
		t.Fatalf("unexpected log: %q", logResult.Output)
	}

	// 5. Send more input
	inputResult := SendSubagentInput(nil, "tc-5", fmt.Sprintf(`{"id":"%s","input":"followup"}`, spawnOut.ID))
	if inputResult.Error != nil {
		t.Fatalf("SendInput error: %v", inputResult.Error)
	}

	// 6. Wait again
	waitResult2 := WaitSubagent(nil, "tc-6", fmt.Sprintf(`{"id":"%s","timeout_seconds":5}`, spawnOut.ID))
	if waitResult2.Error != nil {
		t.Fatalf("Wait(2) error: %v", waitResult2.Error)
	}
	var waitOut2 struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	json.Unmarshal([]byte(waitResult2.Output), &waitOut2)
	if waitOut2.Status != "completed" || waitOut2.Output != "second reply" {
		t.Fatalf("unexpected wait(2) output: %+v", waitOut2)
	}

	// 7. Close
	closeResult := CloseSubagent(nil, "tc-7", fmt.Sprintf(`{"id":"%s"}`, spawnOut.ID))
	if closeResult.Error != nil {
		t.Fatalf("Close error: %v", closeResult.Error)
	}

	// 8. Wait after close should fail
	waitResult3 := WaitSubagent(nil, "tc-8", fmt.Sprintf(`{"id":"%s","timeout_seconds":1}`, spawnOut.ID))
	if waitResult3.Error == nil {
		t.Fatalf("expected error after close")
	}
}
