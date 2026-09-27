package daemonapi

import (
	"errors"
	"fmt"

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

func configUpdate(ctx echo.Context, log *plog.Logger, p naming.Path, deletes []string, unsets []key.T, sets []keyop.T) (bool, error) {
	u, err := prepareConfigUpdate(ctx, log, p, deletes, unsets, sets)
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
func prepareConfigUpdate(ctx echo.Context, log *plog.Logger, p naming.Path, deletes []string, unsets []key.T, sets []keyop.T) (*pendingConfigUpdate, error) {
	base, err := readConfigBase(p)
	if err != nil {
		return nil, fmt.Errorf("read the configuration of %s: %w", p, err)
	}
	oc, err := object.NewConfigurer(p)
	if err != nil {
		return nil, fmt.Errorf("new configurer %s: %w", p, err)
	}
	if err := oc.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
		log.Tracef("prepare configuration update for object %s: %s", p, err)
		return nil, fmt.Errorf("prepare configuration update for object %s: %w", p, err)
	}
	if b, err := oc.Config().Dump(); err != nil {
		log.Tracef("configuration dump for object %s: %s", p, err)
		return nil, fmt.Errorf("configuration dump for object %s: %w", p, err)
	} else if err := configRbac(ctx, p, b); err != nil {
		return nil, err
	}
	if alerts, err := oc.Config().Validate(); err != nil {
		log.Tracef("configuration validation for object %s: %s", p, err)
		return nil, fmt.Errorf("configuration validation for object %s: %w", p, err)
	} else if alerts.HasError() {
		// Say which keywords are wrong. The caller cannot see the
		// configuration this validated, so naming the object alone leaves
		// them nothing to act on.
		errs := alerts.Errors().StringWithoutMeta()
		log.Tracef("configuration validation has errors for object %s:\n%s", p, errs)
		return nil, invalidConfigError{fmt.Errorf("configuration validation has errors for object %s:\n%s", p, errs)}
	}
	if err := refuseClaimOverrun(ctx.Request().Context(), p, oc, oc.Config()); err != nil {
		log.Tracef("claim check for object %s: %s", p, err)
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
	err := u.base.commit(u.oc.Config().CommitInvalid)
	if errors.Is(err, ErrConfigChanged) {
		return false, err
	} else if err != nil {
		u.log.Errorf("configuration commit is invalid for object %s: %s", u.p, err)
		return false, fmt.Errorf("configuration commit is invalid for object %s: %w", u.p, err)
	}
	warnSharedRootlessAccounts(u.ctx, u.p)
	return u.changed, nil
}
