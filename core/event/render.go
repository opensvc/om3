package event

import (
	"fmt"

	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/util/jsondelta"
)

// Render formats a opensvc agent event
func Render(e Event) string {
	s := fmt.Sprintf("%s %s\n", e.Timestamp, e.Kind)
	if e.Kind == "event" {
		s += output.SprintFlat(*e.Data)
	} else if e.Data != nil {
		patch := jsondelta.NewPatch(*e.Data)
		s += patch.Render()
	}
	return s
}
