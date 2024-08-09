package version

import (
	"embed"
	"runtime/debug"
	"strings"
)

var (
	//go:embed text
	fs embed.FS
)

func BuildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}

func ExplicitVersion() string {
	if b, err := fs.ReadFile("text/VERSION"); err == nil {
		s := string(b)
		return strings.TrimSpace(s)
	}
	return ""
}

// Version returns a string containing either the content of
// util/version/text/VERSION (which is created or updated by the
// packaging automation), or the build info release (ie the git
// HEAD commit id at build time).
//
// Example VERSION generation:
//
//	git describe --tags > util/version/text/VERSION
func Version() string {
	if v := ExplicitVersion(); v != "" {
		return v
	} else {
		return BuildVersion()
	}
}
