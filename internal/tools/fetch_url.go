package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type FetchURLArgs struct {
	URL     string `json:"url"`
	Raw     bool   `json:"raw"`
	Timeout int    `json:"timeout_seconds"`
}

func FetchURL(cfg *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args FetchURLArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}
	if args.URL == "" {
		result.Error = fmt.Errorf("url is required")
		result.Output = "Error: url is required"
		return result
	}

	timeout := 30 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, args.URL, nil)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: Failed to create request - %v", err)
		return result
	}
	req.Header.Set("User-Agent", "ashron/1.0")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: Failed to fetch URL - %v", err)
		return result
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	limited := &io.LimitedReader{R: resp.Body, N: int64(cfg.MaxOutputSize)}
	body, err := io.ReadAll(limited)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error: Failed to read response - %v", err)
		return result
	}

	contentType := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(contentType, "text/html")

	var output string
	if isHTML && !args.Raw {
		output = extractText(string(body))
	} else {
		output = string(body)
	}

	if limited.N == 0 {
		output += fmt.Sprintf("\n\n[Content truncated at %d bytes]", cfg.MaxOutputSize)
	}

	slog.Info("URL fetched",
		slog.String("url", args.URL),
		slog.Int("statusCode", resp.StatusCode),
		slog.Int("bytes", len(body)))

	result.Output = fmt.Sprintf("URL: %s\nStatus: %d\nContent-Type: %s\n\n%s",
		args.URL, resp.StatusCode, contentType, output)
	return result
}

// extractText strips HTML tags and returns readable text.
func extractText(src string) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		// Fall back to raw content on parse error.
		return src
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Skip script, style, and head nodes entirely.
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "head", "noscript":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Collapse multiple blank lines.
	lines := strings.Split(sb.String(), "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
		} else {
			blank = 0
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
