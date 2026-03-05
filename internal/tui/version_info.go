package tui

import "fmt"

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

// SetBuildInfo sets build metadata shown in the startup header.
func SetBuildInfo(version, commit, date string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if date != "" {
		buildDate = date
	}
}

func buildInfoLine() string {
	return fmt.Sprintf("v%s (commit: %s, built: %s)", buildVersion, buildCommit, buildDate)
}
