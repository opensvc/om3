package event

import (
	"fmt"

	"github.com/opensvc/om3/core/output"
)

// Render formats a opensvc agent event
func Render(e Event) string {
	s := fmt.Sprintf("%s [%d] %s", e.Time, e.ID, e.Kind)
	if e.Kind == "event" {
		s += output.SprintFlat(e.Data)
	} else if len(e.Data) > 0 {
		s += "\n  " + string(e.Data)
	}
	return s
}

func (e Event) String() string {
	return fmt.Sprintf("event %s %d", e.Kind, e.ID)
}
