package daemonapi

import (
	"fmt"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
)

func configUpdate(log *plog.Logger, p naming.Path, deletes []string, unsets []key.T, sets []keyop.T) (bool, error) {
	oc, err := object.NewConfigurer(p)
	if err != nil {
		return false, fmt.Errorf("new configurer %s: %w", p, err)
	}
	if err := oc.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
		log.Tracef("prepare configuration update for object %s: %s", p, err)
		return false, fmt.Errorf("prepare configuration update for object %s: %w", p, err)
	}
	if alerts, err := oc.Config().Validate(); err != nil {
		log.Tracef("configuration validation for object %s: %s", p, err)
		return false, fmt.Errorf("configuration validation for object %s: %w", p, err)
	} else if alerts.HasError() {
		log.Tracef("configuration validation has errors for object %s", p)
		return false, fmt.Errorf("configuration validation has errors for object %s", p)
	}
	changed := oc.Config().Changed()
	if err := oc.Config().CommitInvalid(); err != nil {
		log.Errorf("configuration commit is invalid for object %s: %s", p, err)
		return false, fmt.Errorf("configuration commit is invalid for object %s: %w", p, err)
	}
	return changed, nil
}
