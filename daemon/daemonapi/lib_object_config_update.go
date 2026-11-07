package daemonapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

// ErrInvalidConfig is what a configuration update the validation refuses is,
// so a caller answers with the bad request it is rather than an internal
// error. The message is the validation's, naming the keywords refused.
var ErrInvalidConfig = errors.New("invalid configuration")

type invalidConfigError struct {
	error
}

func (invalidConfigError) Is(target error) bool {
	return target == ErrInvalidConfig
}

func configUpdate(ctx echo.Context, log *plog.Logger, p naming.Path, deletes []string, unsets []key.T, sets []keyop.T, force bool) (bool, error) {
	u, err := prepareConfigUpdate(ctx, log, p, deletes, unsets, sets, force)
	if err != nil {
		return false, err
	}
	return u.commit()
}

// pendingConfigUpdate is a configuration update every check has passed, and
// that is not written yet.
type pendingConfigUpdate struct {
	ctx     echo.Context
	log     *plog.Logger
	p       naming.Path
	oc      object.Configurer
	base    configBase
	changed bool
}

// prepareConfigUpdate makes the update and runs the checks a write passes,
// the rbac policy, the validation and the claims, without writing it, so a
// caller can do what the write depends on between the checks and the write.
// With force, root writes a change a resource up would leak by.
//
// The error is the first refusal of the checks, or the failure of one.
func prepareConfigUpdate(ctx echo.Context, log *plog.Logger, p naming.Path, deletes []string, unsets []key.T, sets []keyop.T, force bool) (*pendingConfigUpdate, error) {
	check := &configCheck{ctx: ctx, p: p}
	u, err := planConfigUpdate(check, log, deletes, unsets, sets, force)
	if err != nil {
		return nil, err
	} else if check.refused() {
		return nil, check.refusal.err
	}
	return u, nil
}

// planConfigUpdate makes the update and runs the checks of check over it.
// Collecting, as a dry run does, every check runs, and the update is
// returned whatever they found: the alerts of check say it. Else the checks
// stop at the first refusal, and no update is returned.
//
// The error is a check that could not run, or an update that could not be
// made.
func planConfigUpdate(check *configCheck, log *plog.Logger, deletes []string, unsets []key.T, sets []keyop.T, force bool) (*pendingConfigUpdate, error) {
	ctx, p := check.ctx, check.p
	base, err := readConfigBase(p)
	if err != nil {
		return nil, fmt.Errorf("read the configuration of %s: %w", p, err)
	}
	oc, err := object.NewConfigurer(p, object.WithVolatile(check.collect))
	if err != nil {
		return nil, fmt.Errorf("new configurer %s: %w", p, err)
	}
	if err := oc.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
		log.Tracef("prepare configuration update for object %s: %s", p, err)
		return nil, fmt.Errorf("prepare configuration update for object %s: %w", p, err)
	}
	b, err := oc.Config().Dump()
	if err != nil {
		log.Tracef("configuration dump for object %s: %s", p, err)
		return nil, fmt.Errorf("configuration dump for object %s: %w", p, err)
	}
	if !check.rbac(b) || !check.stopToChange(b, force) {
		return nil, nil
	}
	if ok, err := check.holds(b, configScope(p, oc), force); err != nil || !ok {
		return nil, err
	}
	alerts, err := oc.Config().Validate()
	if err != nil {
		log.Tracef("configuration validation for object %s: %s", p, err)
		return nil, fmt.Errorf("configuration validation for object %s: %w", p, err)
	}
	if !check.validate(alerts, func() configRefusal {
		// Say which keywords are wrong. The caller cannot see the
		// configuration this validated, so naming the object alone leaves
		// them nothing to act on.
		errs := alerts.Errors().StringWithoutMeta()
		log.Tracef("configuration validation has errors for object %s:\n%s", p, errs)
		err := invalidConfigError{fmt.Errorf("configuration validation has errors for object %s:\n%s", p, errs)}
		return configRefusal{status: http.StatusBadRequest, title: "Update config", detail: err.Error(), err: err}
	}) {
		return nil, nil
	}
	if ok, err := check.claims(oc, oc.Config()); err != nil || !ok {
		if err != nil {
			log.Tracef("claim check for object %s: %s", p, err)
		}
		return nil, err
	}
	return &pendingConfigUpdate{
		ctx:     ctx,
		log:     log,
		p:       p,
		oc:      oc,
		base:    base,
		changed: oc.Config().Changed(),
	}, nil
}

// commit writes the update, and says whether it changed the configuration.
//
// It writes only over the configuration it was checked against, and refuses
// with ErrConfigChanged if another write landed since. An update changing
// nothing writes nothing, and replaces no other write.
func (u *pendingConfigUpdate) commit() (bool, error) {
	if !u.changed {
		return false, nil
	}
	err := u.base.commit(u.oc.Config(), u.oc.Config().CommitInvalid)
	if errors.Is(err, ErrConfigChanged) {
		return false, err
	} else if err != nil {
		u.log.Errorf("configuration commit is invalid for object %s: %s", u.p, err)
		return false, fmt.Errorf("configuration commit is invalid for object %s: %w", u.p, err)
	}
	warnSharedRootlessAccounts(u.ctx, u.p)
	return u.changed, nil
}
