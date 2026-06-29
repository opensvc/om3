package resourceparser

import (
	"strings"
)

type T struct {
	Schema string
	Target string
}

const (
	sep = "://"
)

func Parse(raw string) T {
	idx := strings.Index(raw, sep)
	if idx == -1 {
		return T{
			Schema: raw,
		}
	}
	return T{
		Schema: raw[:idx],
		Target: raw[idx+len(sep):],
	}
}

func (t *T) String() string {
	return t.Schema + sep + t.Target
}
