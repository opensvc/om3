package naming

import "github.com/opensvc/om3/v3/util/plog"

// LogWithPath returns plog.Logger from existing logger with naming attrs sets
func LogWithPath(l *plog.Logger, p Path) *plog.Logger {
	return l.
		Attr("obj_path", p.String()).
		Attr("obj_kind", p.Kind.String()).
		Attr("obj_name", p.Name).
		Attr("obj_namespace", p.Namespace)
}
