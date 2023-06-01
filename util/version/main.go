package version

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

var (
	//go:embed VERSION
	ExplicitVersion string
)

// Version returns a string containing either the content of
// util/version/VERSION (which is created or updated by the
// packaging automation), or the build info release (ie the
// git HEAD commit id at build time).
//
// Example VERSION generation:
//
//	git describe --tags >util/version/VERSION
func Version() string {
	if ExplicitVersion != "" {
		return strings.TrimSpace(ExplicitVersion)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}
