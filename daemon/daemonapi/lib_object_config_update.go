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
		log.Debugf("PrepareUpdate %s: %s", p, err)
		return false, fmt.Errorf("prepare update %s: %w", p, err)

	}
	if alerts, err := oc.Config().Validate(); err != nil {
		log.Debugf("Validate %s: %s", p, err)
		return false, fmt.Errorf("validate %s: %w", p, err)
	} else if alerts.HasError() {
		log.Debugf("Validate has errors %s", p)
		return false, fmt.Errorf("validate %s has errors: %w", p, err)
	}
	changed := oc.Config().Changed()
	if err := oc.Config().CommitInvalid(); err != nil {
		log.Errorf("CommitInvalid %s: %s", p, err)
		return false, fmt.Errorf("commit %s is invalid: %w", p, err)
	}
	return changed, nil
}
