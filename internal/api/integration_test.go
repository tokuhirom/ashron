package api

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/config"
)

const defaultOllamaURL = "http://localhost:11434/v1"

func ollamaModel() string {
	if v := os.Getenv("ASHRON_TEST_LLM_MODEL"); v != "" {
		return v
	}
	return "qwen3:1.7b"
}

func ollamaURL(t *testing.T) string {
	t.Helper()
	url := defaultOllamaURL
	if v := os.Getenv("ASHRON_TEST_LLM_URL"); v != "" {
		url = v
	}
	// Check that Ollama is reachable.
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url + "/models")
	if err != nil {
		t.Skipf("Ollama not reachable at %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("Ollama returned status %d at %s", resp.StatusCode, url)
	}
	return url
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	url := ollamaURL(t)
	return NewClient(
		&config.ProviderConfig{
			BaseURL: url,
			APIKey:  "ollama", // Ollama doesn't require a real key
			Timeout: 5 * time.Minute,
		},
		&config.ModelConfig{
			Model:       ollamaModel(),
			Temperature: 0.0,
		},
		&config.ContextConfig{
			MaxTokens: 4096, // qwen3 uses reasoning tokens; need headroom
		},
	)
}

func TestIntegration_SimpleChat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{
		NewSystemMessage("You are a helpful assistant. Reply concisely."),
		NewUserMessage("What is 2+3? Reply with just the number."),
	}

	eventCh, err := client.StreamChatCompletionWithTools(ctx, messages, nil)
	if err != nil {
		t.Fatalf("StreamChatCompletionWithTools failed: %v", err)
	}

	var content string
	for ev := range eventCh {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Data != nil && len(ev.Data.Choices) > 0 {
			content += ev.Data.Choices[0].Delta.Content
		}
	}

	if content == "" {
		t.Fatal("expected non-empty response")
	}
	t.Logf("Response: %s", content)

	if !strings.Contains(content, "5") {
		t.Errorf("expected response to contain '5', got: %s", content)
	}
}

func TestIntegration_ToolCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tools := []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_weather",
				Description: "Get the current weather for a city",
				Parameters: FunctionParameters{
					Type: "object",
					Properties: map[string]FunctionProperty{
						"city": {
							Type:        "string",
							Description: "City name",
						},
					},
					Required: []string{"city"},
				},
			},
		},
	}

	messages := []Message{
		NewSystemMessage("You are a helpful assistant. Always use available tools to answer questions. Do not answer directly without using tools first."),
		NewUserMessage("What's the weather in Tokyo?"),
	}

	eventCh, err := client.StreamChatCompletionWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("StreamChatCompletionWithTools failed: %v", err)
	}

	var content string
	var toolCalls []ToolCall
	for ev := range eventCh {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Data != nil && len(ev.Data.Choices) > 0 {
			delta := ev.Data.Choices[0].Delta
			content += delta.Content
			for _, tc := range delta.ToolCalls {
				// Merge streaming tool call deltas
				for tc.Index >= len(toolCalls) {
					toolCalls = append(toolCalls, ToolCall{})
				}
				if tc.ID != "" {
					toolCalls[tc.Index].ID = tc.ID
				}
				if tc.Type != "" {
					toolCalls[tc.Index].Type = tc.Type
				}
				if tc.Function.Name != "" {
					toolCalls[tc.Index].Function.Name = tc.Function.Name
				}
				toolCalls[tc.Index].Function.Arguments += tc.Function.Arguments
			}
		}
	}

	// Model should have called get_weather
	if len(toolCalls) == 0 {
		t.Logf("Response content: %s", content)
		t.Skip("model did not use tool call (some small models may not support tool calling reliably)")
	}

	t.Logf("Tool calls: %+v", toolCalls)
	if toolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected tool call to get_weather, got: %s", toolCalls[0].Function.Name)
	}
}

func TestIntegration_Summarize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	messages := []Message{
		NewSystemMessage("You are a coding assistant."),
		NewUserMessage("I created a file called main.go with a Hello World program."),
		{Role: "assistant", Content: "I see you created main.go with a Hello World program. That's a great start!"},
		NewUserMessage("Then I added a function called add(a, b int) int that returns a+b."),
		{Role: "assistant", Content: "Nice, you added an add function to main.go."},
	}

	summary, err := client.Summarize(ctx, messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	t.Logf("Summary: %s", summary)
}
