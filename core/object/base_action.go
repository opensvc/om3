package object

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
	"opensvc.com/opensvc/core/resourceset"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

// Resources implementing setters
type (
	confirmer interface {
		SetConfirm(v bool)
	}
	forcer interface {
		SetForce(v bool)
	}
)

var (
	ErrInvalidNode = errors.New("invalid node")
)

func (t *Base) validateAction() error {
	if t.Env() != "PRD" && rawconfig.Node.Node.Env == "PRD" {
		return errors.Wrapf(ErrInvalidNode, "not allowed to run on this node (svc env=%s node env=%s)", t.Env(), rawconfig.Node.Node.Env)
	}
	if t.config.IsInNodes(hostname.Hostname()) {
		return nil
	}
	if t.config.IsInDRPNodes(hostname.Hostname()) {
		return nil
	}
	return errors.Wrapf(ErrInvalidNode, "hostname '%s' is not a member of DEFAULT.nodes, DEFAULT.drpnode nor DEFAULT.drpnodes", hostname.Hostname())
}

func (t *Base) setenv(action string, leader bool) {
	os.Setenv("OPENSVC_SVCPATH", t.Path.String())
	os.Setenv("OPENSVC_SVCNAME", t.Path.Name)
	os.Setenv("OPENSVC_NAMESPACE", t.Path.Namespace)
	os.Setenv("OPENSVC_ACTION", action)
	if leader {
		os.Setenv("OPENSVC_LEADER", "1")
	} else {
		os.Setenv("OPENSVC_LEADER", "0")
	}
	// each Setenv resource Driver will load its own env vars when actioned
}

func (t *Base) preAction(ctx context.Context) error {
	if err := t.notifyAction(ctx); err != nil {
		action := actioncontext.Props(ctx)
		t.Log().Debug().Err(err).Msgf("unable to notify %v preAction", action.Name)
	}
	if err := t.mayFreeze(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) needRollback(ctx context.Context) bool {
	action := actioncontext.Props(ctx)
	if !action.Rollback {
		t.Log().Debug().Msgf("skip rollback: not demanded by the %s action", action.Name)
		return false
	}
	if actioncontext.IsRollbackDisabled(ctx) {
		t.Log().Debug().Msg("skip rollback: disabled via the command flag")
		return false
	}
	k := key.Parse("rollback")
	if !t.Config().GetBool(k) {
		t.Log().Debug().Msg("skip rollback: disabled via configuration keyword")
		return false
	}
	return true
}

func (t *Base) rollback(ctx context.Context) error {
	t.Log().Info().Msg("rollback")
	return actionrollback.Rollback(ctx)
}

func (t *Base) withTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	timeout := t.actionTimeout(props.TimeoutKeywords)
	if timeout == 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (t *Base) actionTimeout(kwNames []string) time.Duration {
	for _, kwName := range kwNames {
		k := key.Parse(kwName)
		timeout := t.Config().GetDuration(k)
		if timeout != nil {
			t.log.Debug().Msgf("action timeout set to %s from keyword %s", timeout, kwName)
			return *timeout
		}
	}
	return 0
}

func (t *Base) action(ctx context.Context, fn resourceset.DoFunc) error {
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()
	if err := t.preAction(ctx); err != nil {
		return err
	}
	ctx, stop := statusbus.WithContext(ctx, t.Path)
	defer stop()
	l := resourceselector.FromContext(ctx, t)
	b := actioncontext.To(ctx)
	t.ResourceSets().Do(ctx, l, b, func(ctx context.Context, r resource.Driver) error {
		sb := statusbus.FromContext(ctx)
		sb.Post(r.RID(), resource.Status(ctx, r), false)
		return nil
	})
	if err := t.ResourceSets().Do(ctx, l, b, fn); err != nil {
		t.Log().Err(err).Msg("")
		err = errors.Wrapf(err, "original error")
		if t.needRollback(ctx) {
			if errRollback := t.rollback(ctx); errRollback != nil {
				t.Log().Err(errRollback).Msg("rollback")
			}
		}
		return err
	}
	return nil
}

func (t *Base) notifyAction(ctx context.Context) error {
	if env.HasDaemonOrigin() {
		return nil
	}
	if actioncontext.IsDryRun(ctx) {
		return nil
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	action := actioncontext.Props(ctx)
	req := c.NewPostObjectMonitor()
	req.ObjectSelector = t.Path.String()
	req.State = action.Progress
	if resourceselector.OptionsFromContext(ctx).IsZero() {
		req.LocalExpect = action.LocalExpect
	}
	_, err = req.Do()
	return err
}

func (t *Base) mayFreeze(ctx context.Context) error {
	action := actioncontext.Props(ctx)
	if !action.Freeze {
		return nil
	}
	if actioncontext.IsDryRun(ctx) {
		t.log.Debug().Msg("skip freeze: dry run")
		return nil
	}
	if !resourceselector.OptionsFromContext(ctx).IsZero() {
		t.log.Debug().Msg("skip freeze: resource selection")
		return nil
	}
	if !t.orchestrateWantsFreeze() {
		t.log.Debug().Msg("skip freeze: orchestrate value")
		return nil
	}
	return t.Freeze()
}

func (t *Base) orchestrateWantsFreeze() bool {
	switch t.Orchestrate() {
	case "ha", "start":
		return true
	default:
		return false
	}
}
