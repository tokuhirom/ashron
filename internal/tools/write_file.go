package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func WriteFile(_ *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	var args WriteFileArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.Any("arguments", args))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	path := args.Path
	content := args.Content

	// Clean and validate path
	path = filepath.Clean(path)

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error creating directory: %v", err)
		return result
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error writing file: %v", err)
		return result
	}

	result.Output = fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)
	return result
}
