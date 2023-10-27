package object

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/resourceselector"
)

type enterer interface {
	Enter() error
}

// Enter returns a keyword value
func (t *actor) Enter(ctx context.Context, rid string) error {
	rs := resourceselector.New(t, resourceselector.WithRID(rid))
	for _, r := range rs.Resources() {
		i, ok := r.(enterer)
		if !ok {
			t.Log().Debugf("skip %s: not enterer", r.RID())
			continue
		}
		return i.Enter()
	}
	return fmt.Errorf("no resource supports enter")
}
