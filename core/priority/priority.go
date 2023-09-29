package priority

import (
	"fmt"
)

// T is the scheduling priority of an object instance on a its node.
type T int

// Default is the default priority
const Default = 50

// StatusString returns a short string representation of the priority
// to embed in printed status.
func (t T) StatusString() string {
	if t != Default {
		return fmt.Sprintf("p%d", t)
	}
	return ""
}

// New allocates and set to default a priority.
func New() *T {
	var t T
	t = 50
	return &t
}
