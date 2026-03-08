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
	"strconv"
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
		Model:             c.modelCfg.Model,
		Messages:          messages,
		Temperature:       c.modelCfg.Temperature,
		TopP:              c.modelCfg.TopP,
		MinP:              c.modelCfg.MinP,
		TopK:              c.modelCfg.TopK,
		FrequencyPenalty:  c.modelCfg.FrequencyPenalty,
		PresencePenalty:   c.modelCfg.PresencePenalty,
		Stop:              c.modelCfg.Stop,
		Seed:              c.modelCfg.Seed,
		ParallelToolCalls: c.modelCfg.ParallelToolCalls,
		ReasoningEffort:   c.modelCfg.ReasoningEffort,
		MaxTokens:         c.contextConfig.MaxTokens,
		Tools:             tools,
		Stream:            true,
		StreamOptions: &StreamOptions{
			IncludeUsage: true,
		},
	}
	if c.modelCfg.ResponseFormat != "" {
		req.ResponseFormat = &ResponseFormat{Type: c.modelCfg.ResponseFormat}
	}

	slog.Info("Sending streaming request",
		slog.String("model", req.Model),
		slog.Int("messages", len(req.Messages)),
		slog.Int("tools", len(req.Tools)))

	slog.Debug("Starting streaming chat completion", "model", req.Model, "messages", len(req.Messages))

	body, err := json.Marshal(req)
	if err != nil {
		slog.Error("Failed to marshal streaming request", "error", err)
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	// Communication log (debug): outgoing request payload.
	// Keep this bounded to avoid exploding log size on large conversations.
	slog.Debug("API request payload",
		slog.String("path", "/chat/completions"),
		slog.Int("bytes", len(body)),
		slog.String("json", truncateForLog(string(body), 4000)))

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
	slog.Debug("API response received",
		slog.Int("status", resp.StatusCode),
		slog.String("statusText", resp.Status),
		slog.String("contentType", resp.Header.Get("Content-Type")),
		slog.String("requestID", firstNonEmpty(
			resp.Header.Get("x-request-id"),
			resp.Header.Get("x-amzn-requestid"),
			resp.Header.Get("cf-ray"),
		)),
		slog.String("retryAfter", resp.Header.Get("Retry-After")))

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
				slog.Debug("API stream line", slog.String("line", truncateForLog(line, 2000)))
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
						slog.String("content", chunk.Choices[0].Delta.Content),
						slog.Int("toolCalls", len(chunk.Choices[0].Delta.ToolCalls)),
						slog.String("finishReason", chunk.Choices[0].FinishReason))
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

// Summarize sends a non-streaming chat completion to summarize the given
// messages. It appends a summarization instruction to the message list and
// returns the model's text response.
func (c *Client) Summarize(ctx context.Context, messages []Message) (string, error) {
	summarizeInstruction := Message{
		Role: "user",
		Content: `Create a concise but comprehensive summary of the conversation above.
You MUST preserve the following (in order of priority):
1. Files modified or created, with exact paths and the nature of changes
2. Unresolved bugs, errors, or failing tests — include exact error messages
3. Architectural decisions and their rationale
4. User's stated preferences, constraints, and instructions
5. Current state of the work and what still needs to be done
6. Key commands run and their outcomes (success/failure only for completed items)

You may aggressively compress or omit:
- Intermediate exploration steps that did not lead to changes
- Tool outputs that were only read for information gathering
- Redundant or superseded attempts

Write the summary as structured notes (not prose) to maximize information density.
Use file paths, function names, and concrete details rather than vague descriptions.`,
	}

	req := &ChatCompletionRequest{
		Model:            c.modelCfg.Model,
		Messages:         append(messages, summarizeInstruction),
		Temperature:      c.modelCfg.Temperature,
		TopP:             c.modelCfg.TopP,
		MinP:             c.modelCfg.MinP,
		TopK:             c.modelCfg.TopK,
		FrequencyPenalty: c.modelCfg.FrequencyPenalty,
		PresencePenalty:  c.modelCfg.PresencePenalty,
		Stop:             c.modelCfg.Stop,
		Seed:             c.modelCfg.Seed,
		ReasoningEffort:  c.modelCfg.ReasoningEffort,
	}
	if c.modelCfg.ResponseFormat != "" {
		req.ResponseFormat = &ResponseFormat{Type: c.modelCfg.ResponseFormat}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal summarize request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, "POST", "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("summarize request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Failed to close summarize response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", c.handleError(resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read summarize response: %w", err)
	}

	var completion ChatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return "", fmt.Errorf("parse summarize response: %w", err)
	}

	if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty summarize response")
	}

	return completion.Choices[0].Message.Content, nil
}

// handleError processes API error responses
func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	slog.Debug("API error response body", slog.String("body", truncateForLog(string(body), 4000)))

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("API error: %s (type: %s, code: %s)",
		errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated " + strconv.Itoa(len(s)-max) + " bytes)"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
