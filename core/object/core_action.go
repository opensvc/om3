package object

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
	"opensvc.com/opensvc/core/resourceset"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/stringslice"
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

func (t *actor) validateAction() error {
	node := rawconfig.NodeSection()
	if t.Env() != "PRD" && node.Env == "PRD" {
		return errors.Wrapf(ErrInvalidNode, "not allowed to run on this node (svc env=%s node env=%s)", t.Env(), node.Env)
	}
	if t.config.IsInNodes(hostname.Hostname()) {
		return nil
	}
	if t.config.IsInDRPNodes(hostname.Hostname()) {
		return nil
	}
	return errors.Wrapf(ErrInvalidNode, "hostname '%s' is not a member of DEFAULT.nodes, DEFAULT.drpnode nor DEFAULT.drpnodes", hostname.Hostname())
}

func (t *actor) setenv(action string, leader bool) {
	os.Setenv("OPENSVC_SVCPATH", t.path.String())
	os.Setenv("OPENSVC_SVCNAME", t.path.Name)
	os.Setenv("OPENSVC_NAMESPACE", t.path.Namespace)
	os.Setenv("OPENSVC_ACTION", action)
	if leader {
		os.Setenv("OPENSVC_LEADER", "1")
	} else {
		os.Setenv("OPENSVC_LEADER", "0")
	}
	// each Setenv resource Driver will load its own env vars when actioned
}

func (t *actor) preAction(ctx context.Context) error {
	t.log.Info().Strs("argv", os.Args).Str("origin", env.Origin()).Msg("do")
	if err := t.notifyAction(ctx); err != nil {
		action := actioncontext.Props(ctx)
		t.Log().Debug().Err(err).Msgf("unable to notify %v preAction", action.Name)
	}
	if err := t.mayFreeze(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) needRollback(ctx context.Context) bool {
	if actionrollback.Len(ctx) == 0 {
		t.Log().Debug().Msgf("skip rollback: empty stack")
		return false
	}
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

func (t *actor) rollback(ctx context.Context) error {
	t.Log().Info().Msg("rollback")
	return actionrollback.Rollback(ctx)
}

func (t *actor) withTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	timeout := t.actionTimeout(props.TimeoutKeywords)
	if timeout == 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (t *actor) actionTimeout(kwNames []string) time.Duration {
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

func (t actor) abortWorker(ctx context.Context, r resource.Driver, q chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	a, ok := r.(resource.Aborter)
	if !ok {
		q <- false
		return
	}
	if a.Abort(ctx) {
		t.log.Error().Str("rid", r.RID()).Msg("abort start")
		q <- true
		return
	}
	q <- false
}

func (t *actor) abortStart(ctx context.Context, l resourceLister) (err error) {
	if actioncontext.Props(ctx).Name != "start" {
		return nil
	}
	if err := t.abortStartAffinity(ctx); err != nil {
		return err
	}
	return t.abortStartDrivers(ctx, l)
}

func (t *actor) abortStartAffinity(ctx context.Context) (err error) {
	if env.HasDaemonOrigin() {
		return nil
	}
	if actioncontext.IsForce(ctx) {
		return nil
	}
	for _, pStr := range t.HardAffinity() {
		p, err := path.Parse(pStr)
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s parse path", p)
		}
		obj, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s init", p)
		}
		instanceStatus, err := obj.Status(ctx)
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s status", p)
		}
		switch instanceStatus.Avail {
		case status.Up:
		case status.NotApplicable:
		default:
			return fmt.Errorf("hard affinity with %s is not satisfied (currently %s). use --force if you really want to start", p, instanceStatus.Avail)
		}
	}
	for _, pStr := range t.HardAntiAffinity() {
		p, err := path.Parse(pStr)
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s parse path", p)
		}
		obj, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s init", p)
		}
		instanceStatus, err := obj.Status(ctx)
		if err != nil {
			return errors.Wrapf(err, "hard affinity object %s status", p)
		}
		switch instanceStatus.Avail {
		case status.Down:
		case status.StandbyUp:
		case status.StandbyDown:
		case status.NotApplicable:
		default:
			return fmt.Errorf("hard anti affinity with %s is not satisfied (currently %s). use --force if you really want to start", p, instanceStatus.Avail)
		}
	}
	return nil
}

func (t *actor) abortStartDrivers(ctx context.Context, l resourceLister) (err error) {
	t.log.Log().Msg("abort tests")
	t.log.Debug().Msg("call resource drivers abort start")
	sb := statusbus.FromContext(ctx)
	resources := l.Resources()
	added := 0
	q := make(chan bool, len(resources))
	var wg sync.WaitGroup
	for _, r := range resources {
		currentState := sb.Get(r.RID())
		if currentState.Is(status.Up, status.StandbyUp) {
			continue
		}
		if r.IsDisabled() {
			continue
		}
		wg.Add(1)
		added = added + 1
		go t.abortWorker(ctx, r, q, &wg)
	}
	wg.Wait()
	var ret bool
	for i := 0; i < added; i = i + 1 {
		ret = ret || <-q
	}
	if ret {
		return errors.New("abort start")
	}
	return nil
}

func (t *actor) action(ctx context.Context, fn resourceset.DoFunc) error {
	pg.FromContext(ctx).Register(t.pg)
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()
	if err := t.preAction(ctx); err != nil {
		return err
	}
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	l := resourceselector.FromContext(ctx, t)
	b := actioncontext.To(ctx)
	linkWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			if linkToer, ok := r.(resource.LinkToer); ok {
				if name := linkToer.LinkTo(); name != "" && l.Resources().HasRID(name) {
					// will be handled by the targeted LinkNameser resource
					return nil
				}
			}
			if linkNameser, ok := r.(resource.LinkNameser); !ok {
				// normal action for a non-linkable resource
				return fn(ctx, r)
			} else {
				// Here, we handle a resource other resources can link to.
				names := linkNameser.LinkNames()
				rids := l.Resources().LinkersRID(names)
				filter := func(fn resourceset.DoFunc) resourceset.DoFunc {
					// filter applies the action only on linkers
					return func(ctx context.Context, r resource.Driver) error {
						if !stringslice.Has(r.RID(), rids) {
							return nil
						}
						return fn(ctx, r)
					}
				}

				// On descending action, do action on linkers first.
				if l.IsDesc() {
					if err := t.ResourceSets().Do(ctx, l, b, filter(fn)); err != nil {
						return err
					}
				}
				if err := fn(ctx, r); err != nil {
					return err
				}
				// On ascending action, do action on linkers last.
				if !l.IsDesc() {
					if err := t.ResourceSets().Do(ctx, l, b, filter(fn)); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	t.ResourceSets().Do(ctx, l, b, func(ctx context.Context, r resource.Driver) error {
		sb := statusbus.FromContext(ctx)
		sb.Post(r.RID(), resource.Status(ctx, r), false)
		return nil
	})
	if err := t.abortStart(ctx, l); err != nil {
		return err
	}
	if err := t.ResourceSets().Do(ctx, l, b, linkWrap(fn)); err != nil {
		if t.needRollback(ctx) {
			if errRollback := t.rollback(ctx); errRollback != nil {
				t.Log().Err(errRollback).Msg("rollback")
			}
		}
		return err
	}
	t.CleanPG(ctx)
	return nil
}

//
// notifyAction signals the daemon that an action is in progress, setting the
// instance local expect via POST /object_monitor
//
func (t *actor) notifyAction(ctx context.Context) error {
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
	req.ObjectSelector = t.path.String()
	req.State = action.Progress
	if resourceselector.FromContext(ctx, nil).IsZero() {
		req.LocalExpect = action.LocalExpect
	}
	_, err = req.Do()
	return err
}

func (t *actor) mayFreeze(ctx context.Context) error {
	action := actioncontext.Props(ctx)
	if !action.Freeze {
		return nil
	}
	if actioncontext.IsDryRun(ctx) {
		t.log.Debug().Msg("skip freeze: dry run")
		return nil
	}
	if !resourceselector.FromContext(ctx, nil).IsZero() {
		t.log.Debug().Msg("skip freeze: resource selection")
		return nil
	}
	if !t.orchestrateWantsFreeze() {
		t.log.Debug().Msg("skip freeze: orchestrate value")
		return nil
	}
	return t.Freeze()
}

func (t *actor) orchestrateWantsFreeze() bool {
	switch t.Orchestrate() {
	case "ha", "start":
		return true
	default:
		return false
	}
}
