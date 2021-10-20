package object

import (
	"fmt"

	"opensvc.com/opensvc/core/resourceselector"
)

// OptsEnter is the options of the Enter function of actor objects.
type OptsEnter struct {
	ObjectSelector string `flag:"object"`
	OptsResourceSelector
	Lock OptsLocking
}

type enterer interface {
	Enter() error
}

// Enter returns a keyword value
func (t *Base) Enter(options OptsEnter) error {
	rs := resourceselector.New(t, resourceselector.WithOptions(options.OptsResourceSelector.Options))
	for _, r := range rs.Resources() {
		i, ok := r.(enterer)
		if !ok {
			t.Log().Debug().Msgf("skip %s: not enterer", r.RID())
			continue
		}
		return i.Enter()
	}
	return fmt.Errorf("no resource supports enter")
}
