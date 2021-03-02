package object

import (
	"encoding/json"
	"reflect"

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

// Action executes in parallel the action on all selected objects supporting
// the action.
func (t Selection) Action(action string, args ...interface{}) []ActionResult {
	paths := t.Expand()
	q := make(chan ActionResult, len(paths))
	results := make([]ActionResult, 0)

	for _, path := range paths {
		obj := path.NewObject()
		fn := reflect.ValueOf(obj).MethodByName(action)
		fa := make([]reflect.Value, len(args))
		for k, arg := range args {
			fa[k] = reflect.ValueOf(arg)
		}
		go func(path Path) {
			defer func() {
				if r := recover(); r != nil {
					q <- ActionResult{
						Path:  path,
						Panic: r,
					}
				}
			}()
			q <- fn.Call(fa)[0].Interface().(ActionResult)
		}(path)
	}

	for range paths {
		r := <-q
		results = append(results, r)
	}
	return results
}
