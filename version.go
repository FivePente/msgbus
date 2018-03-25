package msgbus

import (
	"fmt"
)

var (
	//Package package name
	Package = "msgbus"

	// Version release version
	Version = "0.1.0"

	// Build will be overwritten automatically by the build system
	Build = "dev"

	// GitCommit will be overwritten automatically by the build system
	GitCommit = "HEAD"
)

// FullVersion display the full version, build and commit hash
func FullVersion() string {
	return fmt.Sprintf("%s-%s-%s@%s", Package, Version, Build, GitCommit)
}
