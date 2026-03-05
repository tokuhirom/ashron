# Changelog

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
