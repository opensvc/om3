package object

import (
	"encoding/json"
	"fmt"

	"opensvc.com/opensvc/core/client"
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
	var (
		l   []Path
		err error
	)
	l, err = t.daemonExpand()
	if err != nil {
		l = make([]Path, 0)
	}
	return l
}

func (t Selection) daemonExpand() ([]Path, error) {
	api, err := client.New()
	if err != nil {
		return nil, err
	}
	handle := api.NewGetObjectSelector()
	handle.ObjectSelector = t.SelectorExpression
	b, err := handle.Do()
	if err != nil {
		return nil, err
	}
	l := make([]Path, 0)
	json.Unmarshal(b, &l)
	return l, nil
}

// Status executes Status on all selected objects
func (t Selection) Status() error {
	for _, o := range t.Expand() {
		fmt.Println(o)
	}
	return nil
}

// List prints all selected objects
func (t Selection) List() error {
	for _, o := range t.Expand() {
		fmt.Println(o)
	}
	return nil
}
