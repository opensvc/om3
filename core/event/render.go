package event

import (
	"fmt"
	"time"

	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/util/jsondelta"
)

func unixFromFloat64(f float64) time.Time {
	d := time.Duration(f * 1000000000)
	t := time.Unix(0, 0).Add(d)
	return t
}

// Render formats a opensvc agent event
func Render(e Event) string {
	t := unixFromFloat64(e.Timestamp)
	s := fmt.Sprintf("%s %s\n", t, e.Kind)
	if e.Kind == "event" {
		s += output.SprintFlat(*e.Data)
	} else if e.Data != nil {
		patch := jsondelta.NewPatch(*e.Data)
		s += patch.Render()
	}
	return s
}
