package resourcereqs

import (
	"regexp"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/status"
)

var (
	reElement *regexp.Regexp = regexp.MustCompile(`([\w#.-_]+)(\([\w\s,]+\)|)`)
)

type (
	T struct {
		definition string
		data       map[string]status.L
	}
)

func New(s string) *T {
	t := T{
		definition: s,
	}
	return &t
}

func parse(definition string) map[string]status.L {
	data := make(map[string]status.L)
	for _, match := range reElement.FindAllStringSubmatch(definition, -1) {
		rid := match[1]
		states := strings.Trim(match[2], "()")
		l := status.List()
		if len(states) == 0 {
			l = l.Add(status.Up, status.StandbyUp)
		} else {
			for _, s := range strings.Split(states, ",") {
				l = l.Add(status.Parse(s))
			}
		}
		data[rid] = l
	}
	return data
}

// RIDs returns the ids of the resources a requirement definition names, as
// "ip#0 fs#0(down,stdby down)" names ip#0 and fs#0, in the order it names
// them, each once.
func RIDs(definition string) []string {
	var l []string
	for _, match := range reElement.FindAllStringSubmatch(definition, -1) {
		if rid := match[1]; !slices.Contains(l, rid) {
			l = append(l, rid)
		}
	}
	return l
}

func (t *T) Requirements() map[string]status.L {
	if t.data == nil {
		t.data = parse(t.definition)
	}
	return t.data
}
