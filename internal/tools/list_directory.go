package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type ListDirectoryArgs struct {
	Path string `json:"path"`
}

func ListDirectory(_ *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	// Parse arguments
	var args ListDirectoryArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	path := args.Path

	// Clean and validate the path
	path = filepath.Clean(path)

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading directory: %v", err)
		return result
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Contents of %s:\n", path))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Format entry
		typeStr := "file"
		if entry.IsDir() {
			typeStr = "dir "
		}

		output.WriteString(fmt.Sprintf("  [%s] %s (%d bytes) %s\n",
			typeStr,
			entry.Name(),
			info.Size(),
			info.ModTime().Format("2006-01-02 15:04:05"),
		))
	}

	result.Output = output.String()
	return result
}
