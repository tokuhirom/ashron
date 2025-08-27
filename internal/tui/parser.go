package tui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
)

// parseXMLToolCalls attempts to parse XML-style tool calls from content
func parseXMLToolCalls(content string) []api.ToolCall {
	var toolCalls []api.ToolCall

	// Pattern 1: <function=name>...</function>
	functionPattern := regexp.MustCompile(`<function=(\w+)>(.*?)</function>`)
	matches := functionPattern.FindAllStringSubmatch(content, -1)
	for i, match := range matches {
		if len(match) >= 3 {
			functionName := match[1]
			paramsContent := strings.TrimSpace(match[2])

			// Special handling for execute_command
			if functionName == "execute_command" {
				// Look for <parameter=command>...</parameter>
				cmdPattern := regexp.MustCompile(`<parameter=command>(.*?)</parameter>`)
				cmdMatch := cmdPattern.FindStringSubmatch(paramsContent)

				args := make(map[string]interface{})
				if len(cmdMatch) >= 2 {
					args["command"] = strings.TrimSpace(cmdMatch[1])
				} else {
					// Maybe the command is directly in the content
					args["command"] = paramsContent
				}

				jsonArgs, _ := json.Marshal(args)

				toolCall := api.ToolCall{
					ID:   fmt.Sprintf("call_%d", i),
					Type: "function",
					Function: api.FunctionCall{
						Name:      functionName,
						Arguments: string(jsonArgs),
					},
				}
				toolCalls = append(toolCalls, toolCall)
				slog.Debug("Parsed execute_command", "args", string(jsonArgs))
				continue
			}

			toolCall := api.ToolCall{
				ID:   fmt.Sprintf("call_%d", i),
				Type: "function",
				Function: api.FunctionCall{
					Name: functionName,
				},
			}

			// Extract parameters
			if paramsContent != "" {
				// Try to extract JSON-like parameters
				args := extractParameters(paramsContent)
				toolCall.Function.Arguments = args
			} else {
				toolCall.Function.Arguments = "{}"
			}

			toolCalls = append(toolCalls, toolCall)
			slog.Debug("Parsed XML function call", "name", toolCall.Function.Name, "args", toolCall.Function.Arguments)
		}
	}

	// Pattern 2: <tool_call><function>name</function><parameters>...</parameters></tool_call>
	toolCallPattern := regexp.MustCompile(`<tool_call>.*?<function>(\w+)</function>.*?</tool_call>`)
	toolMatches := toolCallPattern.FindAllStringSubmatch(content, -1)
	for i, match := range toolMatches {
		if len(match) >= 2 {
			toolCall := api.ToolCall{
				ID:   fmt.Sprintf("tool_%d", i),
				Type: "function",
				Function: api.FunctionCall{
					Name: match[1],
				},
			}

			// Extract parameters between tags
			paramPattern := regexp.MustCompile(`<parameter=(\w+)>(.*?)</parameter>`)
			paramMatches := paramPattern.FindAllStringSubmatch(content, -1)

			params := make(map[string]string)
			for _, pm := range paramMatches {
				if len(pm) >= 3 {
					params[pm[1]] = strings.TrimSpace(pm[2])
				}
			}

			if len(params) > 0 {
				jsonParams, _ := json.Marshal(params)
				toolCall.Function.Arguments = string(jsonParams)
			} else {
				toolCall.Function.Arguments = "{}"
			}

			toolCalls = append(toolCalls, toolCall)
			slog.Debug("Parsed XML tool call", "name", toolCall.Function.Name, "args", toolCall.Function.Arguments)
		}
	}

	return toolCalls
}

// extractParameters tries to extract parameters from various formats
func extractParameters(content string) string {
	content = strings.TrimSpace(content)

	// Check if it's already valid JSON
	if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
		return content
	}

	// Try to parse parameter tags
	params := make(map[string]interface{})

	// Pattern: <parameter=name>value</parameter>
	paramPattern := regexp.MustCompile(`<parameter=(\w+)>(.*?)</parameter>`)
	matches := paramPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			params[match[1]] = strings.TrimSpace(match[2])
		}
	}

	// If we found parameters, return as JSON
	if len(params) > 0 {
		jsonParams, err := json.Marshal(params)
		if err == nil {
			return string(jsonParams)
		}
	}

	// Try to parse as key-value pairs (e.g., "command: ls -la")
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" && value != "" {
				params[key] = value
			}
		}
	}

	if len(params) > 0 {
		jsonParams, err := json.Marshal(params)
		if err == nil {
			return string(jsonParams)
		}
	}

	// If all else fails, wrap the content as a single parameter
	if content != "" {
		params["value"] = content
		jsonParams, _ := json.Marshal(params)
		return string(jsonParams)
	}

	return "{}"
}
