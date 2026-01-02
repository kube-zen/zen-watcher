package version

import "fmt"

var (
	// Version is the version of zenctl
	Version = "dev"
	// GitCommit is the git commit SHA
	GitCommit = "unknown"
	// BuildTime is the build timestamp
	BuildTime = "unknown"
)

// Info returns version information
func Info() string {
	return fmt.Sprintf("zenctl version %s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}

// VersionCommand returns the version string for the version command
func VersionCommand() string {
	return Info()
}

