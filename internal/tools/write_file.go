package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

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
	change, err := AnalyzeWriteFileChange(path, content)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error analyzing file changes: %v", err)
		return result
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error creating directory: %v", err)
		return result
	}

	var backupPath string
	if change.Existed {
		backupPath, err = createBackup(path)
		if err != nil {
			result.Error = err
			result.Output = fmt.Sprintf("Error creating backup: %v", err)
			return result
		}
	}

	// Write file atomically.
	if err := atomicWrite(path, []byte(content)); err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error writing file: %v", err)
		return result
	}

	result.Output = fmt.Sprintf(
		"Successfully wrote %d bytes to %s\nChange summary: lines %d -> %d, +%d -%d",
		len(content), path, change.OldLines, change.NewLines, change.Added, change.Removed,
	)
	if change.Unchanged {
		result.Output += "\nNote: content is unchanged"
	}
	if backupPath != "" {
		result.Output += "\nBackup: " + backupPath
	}
	return result
}

func createBackup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	backupDir := filepath.Join(os.TempDir(), "ashron-backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	backupName := fmt.Sprintf(
		"%s.%s.bak",
		filepath.Base(path),
		time.Now().Format("20060102-150405.000000000"),
	)
	backupPath := filepath.Join(backupDir, backupName)
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}
	return backupPath, nil
}

func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ashron-write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	mode := os.FileMode(0644)
	if st, statErr := os.Stat(path); statErr == nil {
		mode = st.Mode().Perm()
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
