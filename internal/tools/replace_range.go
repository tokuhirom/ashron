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

type ReplaceRangeArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

func ReplaceRange(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args ReplaceRangeArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		slog.Error("Failed to parse tool arguments", slog.Any("error", err), slog.String("tool", "replace_range"))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}
	if strings.TrimSpace(args.Path) == "" {
		result.Error = fmt.Errorf("path is required")
		result.Output = "Error: path is required"
		return result
	}
	if args.StartLine <= 0 {
		result.Error = fmt.Errorf("start_line must be >= 1")
		result.Output = "Error: start_line must be >= 1"
		return result
	}
	if args.EndLine < args.StartLine {
		result.Error = fmt.Errorf("end_line must be >= start_line")
		result.Output = "Error: end_line must be >= start_line"
		return result
	}

	path := filepath.Clean(args.Path)
	oldContentBytes, err := os.ReadFile(path)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading file: %v", err)
		return result
	}
	oldContent := string(oldContentBytes)
	oldLines := splitLines(oldContent)
	if args.EndLine > len(oldLines) {
		result.Error = fmt.Errorf("line range out of bounds: file has %d lines", len(oldLines))
		result.Output = fmt.Sprintf("Error: line range out of bounds: file has %d lines", len(oldLines))
		return result
	}

	replacementLines := splitLines(args.Content)
	prefix := append([]string(nil), oldLines[:args.StartLine-1]...)
	suffix := append([]string(nil), oldLines[args.EndLine:]...)
	newLines := append(prefix, replacementLines...)
	newLines = append(newLines, suffix...)

	newContent := strings.Join(newLines, "\n")
	if strings.HasSuffix(oldContent, "\n") {
		newContent += "\n"
	}

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

	replacedLines := args.EndLine - args.StartLine + 1
	result.Output = fmt.Sprintf(
		"Successfully replaced lines %d-%d (%d line(s)) in %s\nChange summary: lines %d -> %d, +%d -%d\nBackup: %s",
		args.StartLine, args.EndLine, replacedLines, path, change.OldLines, change.NewLines, change.Added, change.Removed, backupPath,
	)
	if change.Unchanged {
		result.Output += "\nNote: content is unchanged"
	}
	return result
}
