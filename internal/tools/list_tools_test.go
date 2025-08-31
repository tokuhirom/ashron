package tools

import (
	"strings"
	"testing"
)

func TestListTools(t *testing.T) {
	// Test text format
	output := FormatAsText(GetAllTools())

	// Check that output contains expected tools
	expectedTools := []string{
		"read_file",
		"write_file",
		"execute_command",
		"list_directory",
		"list_tools",
		"git_grep",
		"git_ls_files",
	}

	for _, tool := range expectedTools {
		if !strings.Contains(output, tool) {
			t.Errorf("Expected tool '%s' not found in output", tool)
		}
	}

	// Check for headers
	if !strings.Contains(output, "Available Tools:") {
		t.Error("Missing header in output")
	}
}

func TestListToolsJSON(t *testing.T) {
	// Test JSON format
	output, err := FormatAsJSON(GetAllTools())
	if err != nil {
		t.Fatalf("FormatAsJSON() returned error: %v", err)
	}

	// Basic JSON structure check
	if !strings.HasPrefix(output, "[") || !strings.HasSuffix(strings.TrimSpace(output), "]") {
		t.Error("Output is not valid JSON array")
	}

	// Check that output contains expected tools
	expectedTools := []string{
		`"read_file"`,
		`"write_file"`,
		`"execute_command"`,
		`"list_directory"`,
		`"list_tools"`,
		`"git_grep"`,
		`"git_ls_files"`,
	}

	for _, tool := range expectedTools {
		if !strings.Contains(output, tool) {
			t.Errorf("Expected tool '%s' not found in JSON output", tool)
		}
	}
}
