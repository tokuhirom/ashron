package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tokuhirom/ashron/internal/config"
)

// Client handles communication with the OpenAI-compatible API
type Client struct {
	providerCfg   *config.ProviderConfig
	modelCfg      *config.ModelConfig
	contextConfig *config.ContextConfig

	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new API client
func NewClient(providerCfg *config.ProviderConfig, modelCfg *config.ModelConfig, contextConfig *config.ContextConfig) *Client {
	timeout := providerCfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	slog.Info("Creating API client",
		slog.Duration("timeout", timeout))

	return &Client{
		providerCfg:   providerCfg,
		modelCfg:      modelCfg,
		contextConfig: contextConfig,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: strings.TrimSuffix(providerCfg.BaseURL, "/"),
	}
}

// StreamEvent represents a streaming response event
type StreamEvent struct {
	Data  *StreamResponse
	Error error
}

// newRequest creates a new HTTP request with authentication
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.providerCfg.APIKey)

	return req, nil
}

// StreamChatCompletionWithTools sends a streaming request with tool support
func (c *Client) StreamChatCompletionWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error) {
	req := &ChatCompletionRequest{
		Model:       c.modelCfg.Model,
		Messages:    messages,
		Temperature: c.modelCfg.Temperature,
		MaxTokens:   c.contextConfig.MaxTokens,
		Tools:       tools,
		Stream:      true,
		StreamOptions: &StreamOptions{
			IncludeUsage: true,
		},
	}

	slog.Info("Sending streaming request",
		slog.String("model", req.Model),
		slog.Int("messages", len(req.Messages)),
		slog.Int("tools", len(req.Tools)))

	slog.Debug("Starting streaming chat completion", "model", req.Model, "messages", len(req.Messages))

	// Log the outgoing messages at DEBUG so we can inspect tool_calls / content
	// in the conversation history when debugging provider-specific issues.
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		for i, msg := range req.Messages {
			if len(msg.ToolCalls) > 0 {
				tcJSON, _ := json.Marshal(msg.ToolCalls)
				slog.Debug("Outgoing message",
					"index", i,
					"role", msg.Role,
					"contentLen", len(msg.Content),
					"toolCalls", string(tcJSON))
			} else {
				slog.Debug("Outgoing message",
					"index", i,
					"role", msg.Role,
					"contentLen", len(msg.Content),
					"toolCallID", msg.ToolCallID)
			}
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		slog.Error("Failed to marshal streaming request", "error", err)
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, "POST", "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	slog.Debug("Sending streaming API request", "url", httpReq.URL.String())
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.Error("Failed to send streaming request",
			slog.Any("error", err))
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Warn("Failed to close response body",
					slog.Any("error", err))
			}
		}()
		slog.Error("Streaming API returned error", "status", resp.StatusCode)
		return nil, c.handleError(resp)
	}

	eventChan := make(chan StreamEvent)

	go func() {
		defer close(eventChan)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Warn("Failed to close response body",
					slog.Any("error", err))
			}
		}()

		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					eventChan <- StreamEvent{Error: err}
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					slog.Debug("Stream completed")
					return
				}

				// Log the raw SSE data line at DEBUG.
				// This lets us see exactly what the provider sends, which is
				// invaluable when debugging provider-specific quirks (e.g.
				// whether GLM-4.7 includes `index` in tool call deltas, or
				// whether it sends complete arguments in the start delta).
				slog.Debug("Stream raw SSE data", "data", data)

				var chunk StreamResponse
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					slog.Warn("Failed to parse streaming chunk", "error", err, "data", data)
					eventChan <- StreamEvent{Error: fmt.Errorf("parse chunk: %w", err)}
					continue
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]

					// Log a concise parsed summary.
					slog.Debug("Stream chunk parsed",
						"finishReason", choice.FinishReason,
						"contentLen", len(choice.Delta.Content),
						"toolCallCount", len(choice.Delta.ToolCalls))

					// For tool call deltas, log each field explicitly so we can
					// verify index, id, name, and arguments in one log line.
					for i, tc := range choice.Delta.ToolCalls {
						slog.Debug("Stream tool call delta",
							"deltaPos", i, // position within this delta's array
							"tcIndex", tc.Index, // Index field from the provider JSON
							"id", tc.ID,
							"name", tc.Function.Name,
							"argsLen", len(tc.Function.Arguments),
							"args", tc.Function.Arguments)
					}
				}
				eventChan <- StreamEvent{Data: &chunk}
			}
		}
	}()

	return eventChan, nil
}

// handleError processes API error responses
func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("API error: %s (type: %s, code: %s)",
		errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
}
