package selection

import (
	"fmt"

	"opensvc.com/opensvc/core/objects/path"
)

type (
	// Type is the selection structure
	Type struct {
		SelectorExpression string
	}
)

// New allocates a new object selection
func New(selector string) Type {
	t := Type{
		SelectorExpression: selector,
	}
	return t
}

// Expand resolves a selector expression into a list of object paths
func (t Type) Expand() []path.Type {
	var l []path.Type
	return l
}

// Status executes Status on all selected objects
func (t Type) Status() error {
	for o := range t.Expand() {
		fmt.Println(o)
	}
	return nil
}
