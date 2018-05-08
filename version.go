package msgbus

import (
	"fmt"
)

var (
	// Version release version
	Version = "0.1.1"

	// Build will be overwritten automatically by the build system
	Build = "dev"

	// GitCommit will be overwritten automatically by the build system
	GitCommit = "HEAD"
)

// FullVersion returns the full version, build and commit hash
func FullVersion() string {
	return fmt.Sprintf("%s-%s@%s", Version, Build, GitCommit)
}
