package object

import (
	"fmt"

	"opensvc.com/opensvc/core/resourceselector"
)

// OptsEnter is the options of the Enter function of actor objects.
type OptsEnter struct {
	ObjectSelector string `flag:"object"`
	OptsResourceSelector
	OptsLock
}

type enterer interface {
	Enter() error
}

// Enter returns a keyword value
func (t *core) Enter(options OptsEnter) error {
	rs := resourceselector.New(t, resourceselector.WithRID(options.OptsResourceSelector.RID))
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
