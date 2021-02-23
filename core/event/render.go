package event

import (
	"fmt"
	"time"

	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/patch"
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
		for k, v := range output.Flatten(e.Data.(map[string]interface{})) {
			s += fmt.Sprintln(" ", k, "=", v)
		}
	} else if e.Data != nil {
		ps := patch.NewSet(e.Data.([]interface{}))
		s += patch.RenderSet(ps)
	}
	return s
}
