// Package version provides version information
package version

import (
	"fmt"
	"runtime"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info returns version information
func Info() string {
	return fmt.Sprintf("WireRift %s (commit: %s, built: %s, go: %s)",
		Version, Commit, BuildDate, runtime.Version())
}

// Short returns short version string
func Short() string {
	return Version
}
