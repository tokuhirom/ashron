package tui

import (
	"encoding/json"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
)

func TestCommandDanger(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command string
		want    bool
	}{
		{command: "rm -rf tmp", want: true},
		{command: "git reset --hard HEAD", want: true},
		{command: "echo hi", want: false},
	}

	for _, tc := range cases {
		got, _ := commandDanger(tc.command)
		if got != tc.want {
			t.Fatalf("commandDanger(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}

func TestApprovalDangerForWriteFileSystemPath(t *testing.T) {
	t.Parallel()

	args, err := json.Marshal(map[string]string{"path": "/etc/hosts"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	tc := api.ToolCall{Function: api.FunctionCall{Name: "write_file", Arguments: string(args)}}
	got, reason := approvalDanger(tc)
	if !got {
		t.Fatalf("approvalDanger should detect system path risk")
	}
	if reason == "" {
		t.Fatalf("approvalDanger should return reason")
	}
}
