package event

import (
	"fmt"

	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/util/jsondelta"
)

// Render formats a opensvc agent event
func Render(e Event) string {
	s := fmt.Sprintf("%s [%d] %s", e.Time, e.ID, e.Kind)
	if e.Kind == "event" {
		s += output.SprintFlat(e.Data)
	} else if e.Kind == "DataUpdated" {
		if patch, err := jsondelta.NewPatch(e.Data); err != nil {
			s += "render error " + err.Error()
		} else {
			s += "\n" + patch.Render()
		}
	} else if len(e.Data) > 0 {
		s += "\n  " + string(e.Data)
	}
	return s
}

func (e Event) String() string {
	return fmt.Sprintf("event %s %d", e.Kind, e.ID)
}
