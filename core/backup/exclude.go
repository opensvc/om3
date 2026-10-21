package backup

import (
	"path"
	"strings"
)

// Excluded tells whether the path rel, relative to a head, slash separated,
// is one an exclude pattern matches: a pattern holding a slash matches the
// path from the head, one without matches a name at any depth. Store drivers
// and the framework share it, so what a run leaves out is what a restore in
// place leaves alone.
func Excluded(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		pattern = strings.Trim(pattern, "/")
		if pattern == "" {
			continue
		}
		subject := rel
		if !strings.Contains(pattern, "/") {
			subject = path.Base(rel)
		}
		if ok, _ := path.Match(pattern, subject); ok {
			return true
		}
	}
	return false
}

// HeadExcludes returns the patterns of excludes applying to the head rid:
// the ones prefixed with "<rid>:", without the prefix, and the ones with no
// prefix, which apply to every head.
func HeadExcludes(rid string, excludes []string) []string {
	l := make([]string, 0, len(excludes))
	for _, s := range excludes {
		prefix, pattern, found := strings.Cut(s, ":")
		switch {
		case !found:
			l = append(l, s)
		case prefix == rid:
			l = append(l, pattern)
		case !strings.Contains(prefix, "#"):
			// A colon in a pattern with no resource id before it.
			l = append(l, s)
		}
	}
	return l
}
