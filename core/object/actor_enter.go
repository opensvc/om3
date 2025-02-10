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
	var container enterer
	for _, r := range rs.Resources() {
		if i, ok := r.(enterer); !ok {
			continue
		} else if container != nil {
			return fmt.Errorf("multiple resources support enter. use the --rid option")
		} else {
			container = i
			rid = r.RID()
		}
	}
	if container == nil {
		return fmt.Errorf("no resource supports enter")
	}
	if err := container.Enter(); err != nil {
		return fmt.Errorf("%s: %w", rid, err)
	}
	return nil
}
