package subagent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func TestManagerSpawnWaitAndSendInput(t *testing.T) {
	server := newSubagentDummyServer(t, func(call int, req api.ChatCompletionRequest) []api.StreamResponse {
		switch call {
		case 0:
			if len(req.Messages) == 0 || req.Messages[len(req.Messages)-1].Role != "user" {
				t.Fatalf("unexpected first request messages: %#v", req.Messages)
			}
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "first run"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		case 1:
			var hasPrevAssistant, hasSecondUser bool
			for _, m := range req.Messages {
				if m.Role == "assistant" && strings.Contains(m.Content, "first run") {
					hasPrevAssistant = true
				}
				if m.Role == "user" && strings.Contains(m.Content, "second prompt") {
					hasSecondUser = true
				}
			}
			if !hasPrevAssistant || !hasSecondUser {
				t.Fatalf("second request missing expected context: %#v", req.Messages)
			}
			return []api.StreamResponse{
				{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "second run"}}}},
				{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
			}
		default:
			t.Fatalf("unexpected call index: %d", call)
			return nil
		}
	}, 0)
	defer server.Close()

	mgr := newTestManager(t, server.URL)
	id, err := mgr.Spawn("first prompt")
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}

	snap, timedOut, err := mgr.Wait(id, time.Second)
	if err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if timedOut {
		t.Fatalf("Wait should not timeout")
	}
	if snap.Status != AgentStatusCompleted || snap.LastOutput != "first run" {
		t.Fatalf("unexpected snapshot after first run: %#v", snap)
	}

	if err := mgr.SendInput(id, "second prompt"); err != nil {
		t.Fatalf("SendInput error: %v", err)
	}
	snap2, timedOut2, err := mgr.Wait(id, time.Second)
	if err != nil {
		t.Fatalf("Wait(2) error: %v", err)
	}
	if timedOut2 {
		t.Fatalf("second Wait should not timeout")
	}
	if snap2.Status != AgentStatusCompleted || snap2.LastOutput != "second run" {
		t.Fatalf("unexpected snapshot after second run: %#v", snap2)
	}
}

func TestManagerWaitTimeoutAndRunningSummary(t *testing.T) {
	server := newSubagentDummyServer(t, func(call int, req api.ChatCompletionRequest) []api.StreamResponse {
		if call != 0 {
			t.Fatalf("unexpected call index: %d", call)
		}
		_ = req
		return []api.StreamResponse{
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "line-1\n"}}}},
			{Choices: []api.Choice{{Index: 0, Delta: api.Message{Content: "line-2\n"}}}},
			{Choices: []api.Choice{{Index: 0, FinishReason: "stop"}}},
		}
	}, 150*time.Millisecond)
	defer server.Close()

	mgr := newTestManager(t, server.URL)
	id, err := mgr.Spawn("slow prompt")
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}

	snap, timedOut, err := mgr.Wait(id, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if !timedOut {
		t.Fatalf("expected timeout snapshot")
	}
	if snap.Status != AgentStatusRunning {
		t.Fatalf("expected running status on timeout, got %s", snap.Status)
	}

	summary := mgr.GetRunningSummary()
	if len(summary) != 1 || summary[0].ID != id {
		t.Fatalf("unexpected running summary: %#v", summary)
	}

	finalSnap, finalTimedOut, err := mgr.Wait(id, time.Second)
	if err != nil {
		t.Fatalf("final Wait error: %v", err)
	}
	if finalTimedOut {
		t.Fatalf("final Wait should not timeout")
	}
	if finalSnap.Status != AgentStatusCompleted {
		t.Fatalf("expected completed status, got %s", finalSnap.Status)
	}
	if !strings.Contains(finalSnap.LastOutput, "line-1") {
		t.Fatalf("unexpected final output: %q", finalSnap.LastOutput)
	}
}

func TestManagerCloseCancelsAndRemovesAgent(t *testing.T) {
	server := newSubagentCancelableServer(t)
	defer server.Close()

	mgr := newTestManager(t, server.URL)
	id, err := mgr.Spawn("cancel me")
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}

	if err := mgr.Close(id); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if _, err := mgr.Snapshot(id); err == nil {
		t.Fatalf("expected Snapshot error after close")
	}
	if _, _, err := mgr.Wait(id, 10*time.Millisecond); err == nil {
		t.Fatalf("expected Wait error after close")
	}
}

func newTestManager(t *testing.T, baseURL string) *Manager {
	t.Helper()
	provider := &config.ProviderConfig{
		Type:    "openai-compat",
		BaseURL: baseURL,
		APIKey:  "dummy-key",
		Timeout: 5 * time.Second,
	}
	model := &config.ModelConfig{Model: "dummy-model", Temperature: 0}
	ctxCfg := &config.ContextConfig{MaxMessages: 50, MaxTokens: 4096, CompactionRatio: 0.9, AutoCompact: true}
	client := api.NewClient(provider, model, ctxCfg)
	return NewManager(client, ctxCfg)
}

func newSubagentDummyServer(t *testing.T, replyFn func(call int, req api.ChatCompletionRequest) []api.StreamResponse, chunkDelay time.Duration) *httptest.Server {
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
			if chunkDelay > 0 {
				time.Sleep(chunkDelay)
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
}

func newSubagentCancelableServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"running\"}}]}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
}
