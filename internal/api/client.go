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
	config     *config.APIConfig
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new API client
func NewClient(cfg *config.APIConfig) *Client {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second // fallback to 60 seconds if not configured
	}
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

// StreamChatCompletion sends a streaming chat completion request
func (c *Client) StreamChatCompletion(ctx context.Context, req *ChatCompletionRequest) (<-chan StreamEvent, error) {
	req.Stream = true

	slog.Debug("Starting streaming chat completion", "model", req.Model, "messages", len(req.Messages))

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

				var chunk StreamResponse
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					slog.Warn("Failed to parse streaming chunk", "error", err, "data", data)
					eventChan <- StreamEvent{Error: fmt.Errorf("parse chunk: %w", err)}
					continue
				}

				// Log the raw chunk for debugging
				if len(chunk.Choices) > 0 {
					slog.Debug("Raw stream chunk",
						"content", chunk.Choices[0].Delta.Content,
						"toolCalls", len(chunk.Choices[0].Delta.ToolCalls),
						"finishReason", chunk.Choices[0].FinishReason)
				}

				if len(chunk.Choices) > 0 {
					if chunk.Choices[0].Delta.Content != "" {
						slog.Debug("Received streaming content", "length", len(chunk.Choices[0].Delta.Content))
					}
					if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
						slog.Debug("Received streaming tool calls", "count", len(chunk.Choices[0].Delta.ToolCalls))
					}
				}
				eventChan <- StreamEvent{Data: &chunk}
			}
		}
	}()

	return eventChan, nil
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
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	return req, nil
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

// StreamChatCompletionWithTools sends a streaming request with tool support
func (c *Client) StreamChatCompletionWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamEvent, error) {
	req := &ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Tools:       tools,
		Stream:      true,
	}

	return c.StreamChatCompletion(ctx, req)
}
