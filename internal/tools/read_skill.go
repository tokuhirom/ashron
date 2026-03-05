package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/skills"
)

type ReadSkillArgs struct {
	Name string `json:"name"`
}

func ReadSkill(config *config.ToolsConfig, toolCallID string, argsJson string) api.ToolResult {
	result := api.ToolResult{
		ToolCallID: toolCallID,
	}

	var args ReadSkillArgs
	if err := json.Unmarshal([]byte(argsJson), &args); err != nil {
		slog.Error("Failed to parse tool arguments",
			slog.Any("error", err),
			slog.String("arguments", argsJson))
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		result.Output = fmt.Sprintf("Error: Failed to parse arguments - %v", err)
		return result
	}

	name := strings.TrimSpace(args.Name)
	if name == "" {
		result.Error = fmt.Errorf("name is required")
		result.Output = "Error: name is required"
		return result
	}

	var skillPath string
	for _, skill := range skills.Discover() {
		if skill.Name == name {
			skillPath = skill.Path
			break
		}
	}
	if skillPath == "" {
		result.Error = fmt.Errorf("skill not found: %s", name)
		result.Output = fmt.Sprintf("Error: skill not found: %s", name)
		return result
	}

	file, err := os.Open(skillPath)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading skill file: %v", err)
		return result
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("failed to close skill file",
				slog.String("path", skillPath),
				slog.Any("error", err))
		}
	}()

	limited := &io.LimitedReader{
		R: file,
		N: int64(config.MaxOutputSize),
	}
	content, err := io.ReadAll(limited)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Error reading skill content: %v", err)
		return result
	}

	result.Output = string(content)
	if limited.N == 0 {
		result.Output += fmt.Sprintf("\n\n[Skill file truncated at %d bytes]", config.MaxOutputSize)
	}

	slog.Info("Skill read completed",
		slog.String("name", name),
		slog.String("path", skillPath),
		slog.Int("totalBytes", len(content)))

	return result
}
