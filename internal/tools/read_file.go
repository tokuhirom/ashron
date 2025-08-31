package tools

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func ReadFile(config *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// TODO: support read offset

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	// Clean and validate path
	path = filepath.Clean(path)

	// Read file
	file, err := os.Open(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file: %v", err)
		return result
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("failed to close file",
				slog.String("path", path),
				slog.Any("error", err))
		}
	}()

	// Limit file size
	limited := &io.LimitedReader{
		R: file,
		N: int64(config.MaxOutputSize),
	}

	content, err := io.ReadAll(limited)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file content: %v", err)
		return result
	}

	result.Output = string(content)
	if limited.N == 0 {
		result.Output += fmt.Sprintf("\n\n[File truncated at %d bytes]", config.MaxOutputSize)
	}

	// Log with truncated content for readability
	lines := strings.Split(string(content), "\n")
	if len(lines) > 5 {
		truncatedLog := strings.Join(lines[:5], "\n")
		slog.Info("File read completed",
			slog.String("path", path),
			slog.Int("totalLines", len(lines)),
			slog.Int("totalBytes", len(content)),
			slog.String("preview", truncatedLog+"\n[... truncated in log, full content returned]"))
	} else {
		slog.Info("File read completed",
			slog.String("path", path),
			slog.Int("totalBytes", len(content)),
			slog.String("content", string(content)))
	}

	return result
}
