# Changelog

## [v2026.306.0](https://github.com/tokuhirom/ashron/compare/v2026.305.3...v2026.306.0) - 2026-03-06
- feat: apply_patch相当の安全編集ツールを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/86
- feat(tui): add @ file path completion in input by @tokuhirom in https://github.com/tokuhirom/ashron/pull/88
- feat: render markdown in assistant responses by @tokuhirom in https://github.com/tokuhirom/ashron/pull/90
- Rename context to default_context and support model context overrides by @tokuhirom in https://github.com/tokuhirom/ashron/pull/91
- chore(deps): update dependency go to v1.26.1 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/89
- fix: start new session by default instead of showing session picker by @tokuhirom in https://github.com/tokuhirom/ashron/pull/92
- fix: make @ path completion case-insensitive by @tokuhirom in https://github.com/tokuhirom/ashron/pull/93

## [v2026.305.3](https://github.com/tokuhirom/ashron/compare/v2026.305.2...v2026.305.3) - 2026-03-05
- fix: send content:null for assistant tool-call messages (GLM/OpenAI compat) by @tokuhirom in https://github.com/tokuhirom/ashron/pull/61
- feat: highlight PLAN mode label in gold by @tokuhirom in https://github.com/tokuhirom/ashron/pull/63
- feat: change textarea prompt and border color in shell (!) mode by @tokuhirom in https://github.com/tokuhirom/ashron/pull/64
- feat: 起動時セッション選択UIを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/65
- feat: write_fileを差分サマリ付き安全書き込みに改善 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/66
- feat: ツール承認UXを改善（危険操作警告・詳細トグル） by @tokuhirom in https://github.com/tokuhirom/ashron/pull/67
- feat: 長時間処理の可視化と中断制御を改善 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/68
- feat: /status /sessions /tools コマンドを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/69
- chore(deps): update actions/setup-go action to v6 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/9
- feat: カスタムスラッシュコマンドを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/70
- feat: スラッシュコマンドのクォート引数対応 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/71
- feat: 長文ペーストをTUI上で省略表示 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/72
- test: ダミーAIサーバーでE2Eテストを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/73
- test: subagent managerの挙動テストを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/74
- fix: Ctrl+C/キャンセル時のセッション保存漏れを修正 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/75
- feat: /new コマンドを追加 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/76
- fix: block textarea input while waiting for tool approval by @tokuhirom in https://github.com/tokuhirom/ashron/pull/78
- feat: 起動時ヘッダーにバージョン情報を表示 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/77
- fix: remove double border from textarea input by @tokuhirom in https://github.com/tokuhirom/ashron/pull/79
- fix: correct parallel tool call index tracking in streaming by @tokuhirom in https://github.com/tokuhirom/ashron/pull/80
- feat: strip <think> blocks from message history by @tokuhirom in https://github.com/tokuhirom/ashron/pull/81
- fix: don't write start-delta args into toolCallArgs accumulator by @tokuhirom in https://github.com/tokuhirom/ashron/pull/82
- fix: correct local-prefixes in .golangci.yml by @tokuhirom in https://github.com/tokuhirom/ashron/pull/84
- feat: show executing tool name in status bar by @tokuhirom in https://github.com/tokuhirom/ashron/pull/85

## [v2026.305.2](https://github.com/tokuhirom/ashron/compare/v2026.305.1...v2026.305.2) - 2026-03-05
- feat: session persistence and resume by @tokuhirom in https://github.com/tokuhirom/ashron/pull/45
- fix: set goreleaser main to cmd/ashron by @tokuhirom in https://github.com/tokuhirom/ashron/pull/47
- feat: add fetch_url tool by @tokuhirom in https://github.com/tokuhirom/ashron/pull/49
- feat: parse SKILL frontmatter and inject skill metadata by @tokuhirom in https://github.com/tokuhirom/ashron/pull/50
- tui: compact tool execution display by @tokuhirom in https://github.com/tokuhirom/ashron/pull/51
- feat: cancel in-flight API request on Escape key by @tokuhirom in https://github.com/tokuhirom/ashron/pull/52
- tools: add read_skill for SKILL.md content by @tokuhirom in https://github.com/tokuhirom/ashron/pull/53
- feat: show subagent progress in TUI and add get_subagent_log tool by @tokuhirom in https://github.com/tokuhirom/ashron/pull/54
- tui: fix last assistant lines not visible when viewport is full by @tokuhirom in https://github.com/tokuhirom/ashron/pull/55
- feat: run shell commands directly with ! prefix by @tokuhirom in https://github.com/tokuhirom/ashron/pull/57
- tui: add Plan mode toggle via Shift+Tab by @tokuhirom in https://github.com/tokuhirom/ashron/pull/56
- tui: show pending tool details near approval prompt by @tokuhirom in https://github.com/tokuhirom/ashron/pull/58
- fix: restore mouse scroll and fix resume scroll-to-bottom by @tokuhirom in https://github.com/tokuhirom/ashron/pull/59
- fix: allow typing in textarea while AI is processing by @tokuhirom in https://github.com/tokuhirom/ashron/pull/60

## [v2026.305.2](https://github.com/tokuhirom/ashron/compare/v2026.305.1...v2026.305.2) - 2026-03-05
- feat: session persistence and resume by @tokuhirom in https://github.com/tokuhirom/ashron/pull/45
- fix: set goreleaser main to cmd/ashron by @tokuhirom in https://github.com/tokuhirom/ashron/pull/47

## [v2026.305.1](https://github.com/tokuhirom/ashron/compare/v2026.305.0...v2026.305.1) - 2026-03-05
- Refactor config: provider/model hierarchy with /model switch command by @tokuhirom in https://github.com/tokuhirom/ashron/pull/27
- Add command completion popup on / input by @tokuhirom in https://github.com/tokuhirom/ashron/pull/31
- Update dependency go to v1.26.0 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/29
- Update actions/checkout action to v6 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/30
- Add argument completion for /model command by @tokuhirom in https://github.com/tokuhirom/ashron/pull/32
- feat: add OS sandboxing controls and yolo mode by @tokuhirom in https://github.com/tokuhirom/ashron/pull/33
- chore(deps): update actions/create-github-app-token action to v2 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/34
- feat: add subagent tools and runtime manager by @tokuhirom in https://github.com/tokuhirom/ashron/pull/35
- Migrate to bubbletea/lipgloss/bubbles v2 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/37
- refactor(config): replace viper with gopkg.in/yaml.v3 by @tokuhirom in https://github.com/tokuhirom/ashron/pull/38
- refactor: replace kingpin with kong for CLI argument parsing by @tokuhirom in https://github.com/tokuhirom/ashron/pull/39
- fix(config): use os.UserConfigDir() for XDG-compliant config path by @tokuhirom in https://github.com/tokuhirom/ashron/pull/40
- feat: add local skills discovery support by @tokuhirom in https://github.com/tokuhirom/ashron/pull/36
- fix: error display truncation and XDG_CONFIG_HOME compliance by @tokuhirom in https://github.com/tokuhirom/ashron/pull/41
- fix: use XDG_CONFIG_HOME or ~/.config on all platforms including macOS by @tokuhirom in https://github.com/tokuhirom/ashron/pull/42
- fix: spinner not animating during loading by @tokuhirom in https://github.com/tokuhirom/ashron/pull/43
- feat: enable mouse scroll for viewport by @tokuhirom in https://github.com/tokuhirom/ashron/pull/44

## [v2026.305.0](https://github.com/tokuhirom/ashron/commits/v2026.305.0) - 2026-03-05
- Configure Renovate by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/1
- Update actions/checkout action to v5 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/2
- Update dependency go to 1.26 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/17
- Update golangci/golangci-lint-action action to v9 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/12
- Update module github.com/charmbracelet/bubbletea to v1.3.10 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/10
- Update module github.com/gen2brain/beeep to v0.11.2 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/14
- Update actions/checkout action to v6 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/13
- Add tagpr for automated release management by @tokuhirom in https://github.com/tokuhirom/ashron/pull/22
- Add mise.toml to pin Go and golangci-lint versions by @tokuhirom in https://github.com/tokuhirom/ashron/pull/23
- Update goreleaser/goreleaser-action action to v7 by @renovate[bot] in https://github.com/tokuhirom/ashron/pull/18
- Fix staticcheck QF1012: replace WriteString(Sprintf) with fmt.Fprintf by @tokuhirom in https://github.com/tokuhirom/ashron/pull/24
- Simplify CI: use single Go 1.26.0 instead of matrix by @tokuhirom in https://github.com/tokuhirom/ashron/pull/26
