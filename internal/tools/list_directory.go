package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func ListDirectory(_ *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	// Clean and validate path
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
