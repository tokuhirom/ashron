package tools

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/config"
)

func testToolsConfig() *config.ToolsConfig {
	return &config.ToolsConfig{
		MaxOutputSize:  50000,
		CommandTimeout: 30 * time.Second,
		SandboxMode:    "off",
	}
}

func TestExecuteBackgroundCommand_Basic(t *testing.T) {
	args, _ := json.Marshal(ExecuteCommandArgs{Command: "echo hello"})
	result := ExecuteBackgroundCommand(testToolsConfig(), "tc1", string(args))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "bg_") {
		t.Fatalf("expected task ID in output, got: %s", result.Output)
	}

	// Extract ID
	var id string
	for _, line := range strings.Split(result.Output, "\n") {
		if strings.HasPrefix(line, "ID: ") {
			id = strings.TrimPrefix(line, "ID: ")
			break
		}
	}
	if id == "" {
		t.Fatal("could not find ID in output")
	}

	// Wait for it to finish
	bgTasksMu.Lock()
	task := bgTasks[id]
	bgTasksMu.Unlock()
	<-task.done

	// Get output
	getArgs, _ := json.Marshal(GetBackgroundOutputArgs{ID: id})
	getResult := GetBackgroundOutput(nil, "tc2", string(getArgs))
	if getResult.Error != nil {
		t.Fatalf("unexpected error: %v", getResult.Error)
	}
	if !strings.Contains(getResult.Output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", getResult.Output)
	}
	if !strings.Contains(getResult.Output, "finished") {
		t.Errorf("expected 'finished' in output, got: %s", getResult.Output)
	}
}

func TestGetBackgroundOutput_NotFound(t *testing.T) {
	args, _ := json.Marshal(GetBackgroundOutputArgs{ID: "nonexistent"})
	result := GetBackgroundOutput(nil, "tc1", string(args))
	if result.Error == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestListBackgroundCommands_Empty(t *testing.T) {
	// Save and restore state
	bgTasksMu.Lock()
	saved := bgTasks
	bgTasks = make(map[string]*bgTask)
	bgTasksMu.Unlock()
	defer func() {
		bgTasksMu.Lock()
		bgTasks = saved
		bgTasksMu.Unlock()
	}()

	result := ListBackgroundCommands(nil, "tc1", "{}")
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output != "No background commands." {
		t.Errorf("expected empty message, got: %s", result.Output)
	}
}

func TestListBackgroundCommands_WithTasks(t *testing.T) {
	args, _ := json.Marshal(ExecuteCommandArgs{Command: "echo test"})
	result := ExecuteBackgroundCommand(testToolsConfig(), "tc1", string(args))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	listResult := ListBackgroundCommands(nil, "tc2", "{}")
	if listResult.Error != nil {
		t.Fatalf("unexpected error: %v", listResult.Error)
	}
	if !strings.Contains(listResult.Output, "echo test") {
		t.Errorf("expected command in list, got: %s", listResult.Output)
	}
}

func TestExecuteBackgroundCommand_RunningStatus(t *testing.T) {
	args, _ := json.Marshal(ExecuteCommandArgs{Command: "sleep 10"})
	result := ExecuteBackgroundCommand(testToolsConfig(), "tc1", string(args))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	var id string
	for _, line := range strings.Split(result.Output, "\n") {
		if strings.HasPrefix(line, "ID: ") {
			id = strings.TrimPrefix(line, "ID: ")
			break
		}
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	getArgs, _ := json.Marshal(GetBackgroundOutputArgs{ID: id})
	getResult := GetBackgroundOutput(nil, "tc2", string(getArgs))
	if getResult.Error != nil {
		t.Fatalf("unexpected error: %v", getResult.Error)
	}
	if !strings.Contains(getResult.Output, "running") {
		t.Errorf("expected 'running' status, got: %s", getResult.Output)
	}
}
