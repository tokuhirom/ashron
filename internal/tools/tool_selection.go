package tools

import (
	"strings"

	"github.com/tokuhirom/ashron/internal/api"
)

var readOnlyToolNames = map[string]struct{}{
	"fetch_url":        {},
	"get_diagnostics":  {},
	"get_tool_result":  {},
	"list_directory":   {},
	"list_subagents":   {},
	"list_tools":       {},
	"memory_list":      {},
	"read_file":        {},
	"read_skill":       {},
	"wait_subagent":    {},
	"get_subagent_log": {},
}

var extendedToolsetKeywords = []string{
	"build",
	"change",
	"command",
	"commit",
	"create",
	"delete",
	"edit",
	"execute",
	"fix",
	"implement",
	"install",
	"modify",
	"patch",
	"refactor",
	"rename",
	"run",
	"test",
	"update",
	"write",
	"インストール",
	"コミット",
	"テスト",
	"ビルド",
	"リネーム",
	"修正",
	"作成",
	"変更",
	"実装",
	"実行",
	"削除",
	"書",
	"編集",
}

// SelectBuiltinTools picks a smaller read-only toolset by default to reduce
// per-request token overhead. It switches to the full toolset when the prompt
// likely needs edits or command execution.
func SelectBuiltinTools(prompt string) []api.Tool {
	if LikelyNeedsExtendedToolset(prompt) {
		return GetBuiltinTools()
	}
	srcTools := GetAllTools()
	out := make([]api.Tool, 0, len(srcTools))
	for _, t := range srcTools {
		if _, ok := readOnlyToolNames[t.Name]; !ok {
			continue
		}
		out = append(out, api.Tool{
			Type: "function",
			Function: api.FunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// LikelyNeedsExtendedToolset returns whether the input indicates write/command intent.
func LikelyNeedsExtendedToolset(input string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return false
	}
	for _, kw := range extendedToolsetKeywords {
		if strings.Contains(trimmed, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
