package object

import (
	"fmt"
)

type (
	// Selection is the selection structure
	Selection struct {
		SelectorExpression string
	}
)

// NewSelection allocates a new object selection
func NewSelection(selector string) Selection {
	t := Selection{
		SelectorExpression: selector,
	}
	return t
}

// Expand resolves a selector expression into a list of object paths
func (t Selection) Expand() []Path {
	var l []Path
	return l
}

// Status executes Status on all selected objects
func (t Selection) Status() error {
	for o := range t.Expand() {
		fmt.Println(o)
	}
	return nil
}
