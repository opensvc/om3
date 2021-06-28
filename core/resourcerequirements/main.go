package resourcerequirement

import (
	"regexp"
	"strings"

	"opensvc.com/opensvc/core/status"
)

var (
	reElement *regexp.Regexp = regexp.MustCompile(`([\w#.-_]+)(\([\w\s,]+\)|)`)
)

type (
	T struct {
		definition string
		data       map[string][]status.T
	}
)

func New(s string) *T {
	t := T{
		definition: s,
	}
	return &t
}

func parse(definition string) map[string][]status.T {
	data := make(map[string][]status.T)
	for _, match := range reElement.FindAllStringSubmatch(definition, -1) {
		rid := match[1]
		states := strings.Trim(match[2], "()")
		if len(states) == 0 {
			data[rid] = []status.T{status.Up, status.StandbyUp}
		} else {
			l := make([]status.T, 0)
			for _, s := range strings.Split(states, ",") {
				l = append(l, status.Parse(s))
			}
			data[rid] = l
		}
	}
	return data
}

func (t *T) Requirements() map[string][]status.T {
	if t.data == nil {
		t.data = parse(t.definition)
	}
	return t.data
}
