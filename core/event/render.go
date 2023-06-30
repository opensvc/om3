package event

import (
	"encoding/json"
	"fmt"
)

// Render formats a opensvc agent event
func (e Event) Render() string {
	s := fmt.Sprintf("%s [%d] %s\n", e.At, e.ID, e.Kind)
	s += fmt.Sprintf("  %s\n", string(e.Data))
	return s
}

func (e Event) String() string {
	return fmt.Sprintf("event %s %d", e.Kind, e.ID)
}

func (e ConcreteEvent) Render() string {
	s := fmt.Sprintf("%s [%d] %s\n", e.At, e.ID, e.Kind)
	b, _ := json.Marshal(e.Data)
	s += fmt.Sprintf("  %s\n", string(b))
	return s
}

func (e ConcreteEvent) String() string {
	return fmt.Sprintf("event %s %d", e.Kind, e.ID)
}
