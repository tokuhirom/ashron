package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func WriteFile(_ *config.ToolsConfig, toolCallID string, args map[string]interface{}) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	path, ok := args["path"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'path' argument")
		result.Output = "Error: Missing or invalid 'path' argument"
		return result
	}

	content, ok := args["content"].(string)
	if !ok {
		result.Error = fmt.Errorf("missing or invalid 'content' argument")
		result.Output = "Error: Missing or invalid 'content' argument"
		return result
	}

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
