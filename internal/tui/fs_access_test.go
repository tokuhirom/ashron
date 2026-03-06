package tui

import (
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func TestRequiredFSAccessWorkspaceVsExternal(t *testing.T) {
	workspace := "/home/user/project"

	inside := api.ToolCall{
		Function: api.FunctionCall{Name: "read_file", Arguments: `{"path":"/home/user/project/README.md"}`},
	}
	if _, needed, err := requiredFSAccess(inside, workspace); err != nil || needed {
		t.Fatalf("workspace path should not require extra permission, needed=%v err=%v", needed, err)
	}

	outside := api.ToolCall{
		Function: api.FunctionCall{Name: "read_file", Arguments: `{"path":"/tmp/demo.txt"}`},
	}
	req, needed, err := requiredFSAccess(outside, workspace)
	if err != nil {
		t.Fatalf("requiredFSAccess returned error: %v", err)
	}
	if !needed {
		t.Fatalf("external path should require extra permission")
	}
	if req.Kind != fsRead {
		t.Fatalf("unexpected kind: %s", req.Kind)
	}
	if req.Scope != "/tmp" {
		t.Fatalf("unexpected scope: %s", req.Scope)
	}
}

func TestIsAutoApprovedRequiresSessionGrantForExternalRead(t *testing.T) {
	m := &SimpleModel{
		config: &config.Config{
			Tools: config.ToolsConfig{
				AutoApproveTools: []string{"read_file"},
			},
		},
		workspaceRoot:   "/home/user/project",
		fsSessionGrants: make(map[string]map[string]bool),
	}

	if m.isAutoApproved("read_file", `{"path":"/tmp/demo.txt"}`) {
		t.Fatalf("external read should not be auto-approved without grant")
	}

	m.addFSGrant(fsRead, "/tmp")
	if !m.isAutoApproved("read_file", `{"path":"/tmp/demo.txt"}`) {
		t.Fatalf("external read should be auto-approved after session grant")
	}
}

func TestIsAutoApprovedRequiresSessionGrantForExternalWrite(t *testing.T) {
	m := &SimpleModel{
		config: &config.Config{
			Tools: config.ToolsConfig{
				AutoApproveTools: []string{"write_file"},
			},
		},
		workspaceRoot:   "/home/user/project",
		fsSessionGrants: make(map[string]map[string]bool),
	}

	if m.isAutoApproved("write_file", `{"path":"/tmp/out.txt","content":"x"}`) {
		t.Fatalf("external write should not be auto-approved without grant")
	}

	m.addFSGrant(fsWrite, "/tmp")
	if !m.isAutoApproved("write_file", `{"path":"/tmp/out.txt","content":"x"}`) {
		t.Fatalf("external write should be auto-approved after session grant")
	}
}
