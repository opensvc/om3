package commands

import (
	"fmt"
)

func mergeSelector(selector string, subsysSelector string, kind string, defaultSelector string) string {
	var s string
	switch {
	case selector != "":
		s = selector
	case subsysSelector != "" && kind != "":
		s = fmt.Sprintf("%s+*/%s/*", subsysSelector, kind)
	case subsysSelector != "" && kind == "":
		s = subsysSelector
	case kind != "":
		s = fmt.Sprintf("%s+*/%s/*", defaultSelector, kind)
	default:
		s = defaultSelector
	}
	return s
}
