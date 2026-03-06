package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type SearchAndReplaceArgs struct {
	Path    string `json:"path"`
	Search  string `json:"search"`
	Replace string `json:"replace"`
}

func SearchAndReplace(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args SearchAndReplaceArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		slog.Error("Failed to parse tool arguments", slog.Any("error", err), slog.String("tool", "search_and_replace"))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}
	if strings.TrimSpace(args.Path) == "" {
		result.Error = fmt.Errorf("path is required")
		result.Output = "Error: path is required"
		return result
	}
	if args.Search == "" {
		result.Error = fmt.Errorf("search must not be empty")
		result.Output = "Error: search must not be empty"
		return result
	}

	path := filepath.Clean(args.Path)
	oldContent, err := os.ReadFile(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file: %v", err)
		return result
	}

	occurrences := strings.Count(string(oldContent), args.Search)
	if occurrences == 0 {
		result.Error = fmt.Errorf("search text not found")
		result.Output = "Error: search text not found"
		return result
	}

	newContent := strings.ReplaceAll(string(oldContent), args.Search, args.Replace)
	change, err := AnalyzeWriteFileChange(path, newContent)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error analyzing file changes: %v", err)
		return result
	}

	backupPath, err := createBackup(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error creating backup: %v", err)
		return result
	}

	if err := atomicWrite(path, []byte(newContent)); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error writing file: %v", err)
		return result
	}

	result.Output = fmt.Sprintf(
		"Successfully replaced %d occurrence(s) in %s\nChange summary: lines %d -> %d, +%d -%d\nBackup: %s",
		occurrences, path, change.OldLines, change.NewLines, change.Added, change.Removed, backupPath,
	)
	if change.Unchanged {
		result.Output += "\nNote: content is unchanged"
	}
	return result
}
