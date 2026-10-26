package backup

import (
	"path"
	"strings"
)

// Excluded tells whether the path rel, relative to a head, slash separated,
// is one an exclude pattern matches: a pattern starting with or holding a
// slash matches the path from the head, one without matches a name at any
// depth. Store drivers and the framework share it, so what a run leaves out
// is what a restore in place leaves alone.
func Excluded(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		pattern, anchored := SplitPattern(pattern)
		if pattern == "" {
			continue
		}
		subject := rel
		if !anchored {
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

// SplitPattern returns the pattern without its leading and trailing
// slashes, and whether it matches the path from the head, rather than a name
// at any depth: a pattern starting with a slash, as "/tmp", or holding one.
func SplitPattern(pattern string) (string, bool) {
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.Trim(pattern, "/")
	return pattern, anchored || strings.Contains(pattern, "/")
}

// LiteralPattern returns the pattern matching the path rel from the head,
// and nothing else: anchored, its wildcards escaped.
func LiteralPattern(rel string) string {
	var b strings.Builder
	b.WriteString("/")
	for _, r := range rel {
		switch r {
		case '\\', '*', '?', '[':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
