package version

import (
	"runtime/debug"
	"strings"
)

// Commit is injected at build time with -ldflags. The development fallback is
// explicit so update checks can fall back to config/repo state.
var Commit = "dev"

// Current returns the injected commit, or the version embedded by go install.
func Current() string {
	if Commit != "dev" {
		return Commit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	parts := strings.Split(info.Main.Version, "-")
	if len(parts) >= 3 {
		return strings.TrimSuffix(parts[len(parts)-1], "+dirty")
	}
	return info.Main.Version
}
