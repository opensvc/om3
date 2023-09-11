package object

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceselector"
	"github.com/opensvc/om3/core/resourceset"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/xsession"
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
	ErrDisabled    = errors.New("object instance is disabled")
)

func (t *actor) validateAction() error {
	node := rawconfig.NodeSection()
	if t.Env() != "PRD" && node.Env == "PRD" {
		return fmt.Errorf("%w: not allowed to run on this node (svc env=%s node env=%s)", ErrInvalidNode, t.Env(), node.Env)
	}
	if v, err := t.config.IsInNodes(hostname.Hostname()); err != nil {
		return err
	} else if v {
		return nil
	}
	if v, err := t.config.IsInDRPNodes(hostname.Hostname()); err != nil {
		return err
	} else if v {
		return nil
	}
	return fmt.Errorf("%w: the hostname '%s' is not a member of DEFAULT.nodes, DEFAULT.drpnode nor DEFAULT.drpnodes", ErrInvalidNode, hostname.Hostname())
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
	if err := t.mayFreeze(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) needRollback(ctx context.Context) bool {
	if actionrollback.Len(ctx) == 0 {
		t.Log().Debug().Msgf("Skip rollback: Empty stack")
		return false
	}
	action := actioncontext.Props(ctx)
	if !action.Rollback {
		t.Log().Debug().Msgf("Skip rollback: Not demanded by the %s action", action.Name)
		return false
	}
	if actioncontext.IsRollbackDisabled(ctx) {
		t.Log().Debug().Msg("Skip rollback: Disabled via the command flag")
		return false
	}
	k := key.Parse("rollback")
	if !t.Config().GetBool(k) {
		t.Log().Debug().Msg("Skip rollback: Disabled via configuration keyword")
		return false
	}
	return true
}

func (t *actor) rollback(ctx context.Context) error {
	t.Log().Info().Msg("Rollback")
	return actionrollback.Rollback(ctx)
}

func (t *actor) withTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	timeout, source := t.actionTimeout(props.TimeoutKeywords)
	t.log.Debug().Msgf("Action timeout set to %s from keyword %s", timeout, source)
	if timeout == 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (t *actor) actionTimeout(kwNames []string) (time.Duration, string) {
	for _, kwName := range kwNames {
		k := key.Parse(kwName)
		timeout := t.Config().GetDuration(k)
		if timeout != nil {
			return *timeout, kwName
		}
	}
	return 0, ""
}

func (t actor) abortWorker(ctx context.Context, r resource.Driver, q chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	a, ok := r.(resource.Aborter)
	if !ok {
		q <- false
		return
	}
	r.Progress(ctx, "▶ run abort tests")
	if a.Abort(ctx) {
		t.log.Error().Str("rid", r.RID()).Msg("deny start")
		r.Progress(ctx, rawconfig.Colorize.Error("deny start"))
		q <- true
		return
	}
	r.Progress(ctx, rawconfig.Colorize.Optimal("✓")+" allow start")
	q <- false
}

// announceProgress signals the daemon that an action is in progress, using the
// POST /object/progress. This handler manages local expect:
// * set to "started" via InstanceMonitorUpdated event handler
// * set to "" if progress is idle
func (t *actor) announceProgress(ctx context.Context, progress string) error {
	if env.HasDaemonOrigin() {
		// no need to announce if the daemon started this action
		return nil
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	isPartial := !resourceselector.FromContext(ctx, nil).IsZero()
	resp, err := c.PostObjectProgress(ctx, api.PostObjectProgress{
		Path:      t.path.String(),
		State:     progress,
		SessionId: xsession.ID,
		IsPartial: &isPartial,
	})
	switch {
	case errors.Is(err, os.ErrNotExist):
		t.log.Debug().Msg("Skip announce progress: The daemon is not running")
		return nil
	case err != nil:
		t.log.Error().Err(err).Msgf("Announce %s state", progress)
		return err
	case resp.StatusCode != http.StatusOK:
		err := fmt.Errorf("unexpected post object progress status %s", resp.Status)
		t.log.Error().Err(err).Msgf("Announce %s state", progress)
		return err
	}
	t.log.Info().Msgf("Announce %s state", progress)
	return nil
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
			return fmt.Errorf("hard affinity object %s parse path: %w", p, err)
		}
		obj, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return fmt.Errorf("hard affinity object %s init: %w", p, err)
		}
		instanceStatus, err := obj.Status(ctx)
		if err != nil {
			return fmt.Errorf("hard affinity object %s status: %w", p, err)
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
			return fmt.Errorf("hard anti affinity object %s parse path: %w", p, err)
		}
		obj, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return fmt.Errorf("hard anti affinity object %s init: %w", p, err)
		}
		instanceStatus, err := obj.Status(ctx)
		if err != nil {
			return fmt.Errorf("hard anti affinity object %s status: %w", p, err)
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
		return fmt.Errorf("abort start")
	}
	return nil
}

func (t *actor) action(ctx context.Context, fn resourceset.DoFunc) error {
	if t.IsDisabled() {
		return ErrDisabled
	}
	wd, _ := os.Getwd()
	action := actioncontext.Props(ctx)
	t.log.Info().
		Strs("argv", os.Args).
		Str("cwd", wd).
		Str("action", action.Name).
		Str("origin", env.Origin()).
		Msg("do")
	beginTime := time.Now()
	defer func() {
		t.log.Info().
			Strs("argv", os.Args).
			Str("cwd", wd).
			Str("action", action.Name).
			Str("origin", env.Origin()).
			Dur("duration", time.Now().Sub(beginTime)).
			Msg("done")
	}()

	// daemon instance monitor updates
	progress := actioncontext.Props(ctx).Progress
	t.announceProgress(ctx, progress)
	defer t.announceProgress(ctx, "idle") // TODO: failed cases ?

	if mgr := pg.FromContext(ctx); mgr != nil {
		mgr.Register(t.pg)
	}
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()
	if err := t.preAction(ctx); err != nil {
		return err
	}
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	l := resourceselector.FromContext(ctx, t)
	b := actioncontext.To(ctx)

	progressWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			l := t.log.With().Str("rid", r.RID()).Logger()
			ctx = l.WithContext(ctx)
			err := fn(ctx, r)
			switch {
			case errors.Is(err, resource.ErrDisabled):
				err = nil
			case errors.Is(err, resource.ErrActionNotSupported):
				err = nil
			case errors.Is(err, resource.ErrActionPostponedToLinker):
				err = nil
			case err == nil:
				r.Progress(ctx, rawconfig.Colorize.Optimal("✓"))
			case r.IsOptional():
				r.Progress(ctx, rawconfig.Colorize.Warning(err))
			default:
				r.Progress(ctx, rawconfig.Colorize.Error(err))
			}
			return err
		}
	}

	linkWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			if linkToer, ok := r.(resource.LinkToer); ok {
				if name := linkToer.LinkTo(); name != "" && l.Resources().HasRID(name) {
					// will be handled by the targeted LinkNameser resource
					return resource.ErrActionPostponedToLinker
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
					if err := t.ResourceSets().Do(ctx, l, b, "linked-"+action.Name, progressWrap(filter(fn))); err != nil {
						return err
					}
				}
				if err := fn(ctx, r); err != nil {
					return err
				}
				// On ascending action, do action on linkers last.
				if !l.IsDesc() {
					if err := t.ResourceSets().Do(ctx, l, b, "linked-"+action.Name, progressWrap(filter(fn))); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	t.ResourceSets().Do(ctx, l, b, "pre-"+action.Name+" status", func(ctx context.Context, r resource.Driver) error {
		sb := statusbus.FromContext(ctx)
		sb.Post(r.RID(), resource.EvalStatus(ctx, r), false)
		return nil
	})
	if err := t.abortStart(ctx, l); err != nil {
		_, _ = t.statusEval(ctx)
		return err
	}
	if err := t.ResourceSets().Do(ctx, l, b, action.Name, progressWrap(linkWrap(fn))); err != nil {
		if t.needRollback(ctx) {
			if errRollback := t.rollback(ctx); errRollback != nil {
				t.Log().Err(errRollback).Msg("Rollback")
			}
		}
		return err
	}
	if action.Order.IsDesc() {
		t.CleanPG(ctx)
	}
	return t.postStartStopStatusEval(ctx)
}

func (t *actor) postStartStopStatusEval(ctx context.Context) error {
	action := actioncontext.Props(ctx)
	instStatus, err := t.statusEval(ctx)
	if err != nil {
		return err
	}
	if !resourceselector.FromContext(ctx, nil).IsZero() {
		// don't verify instance avail if a resource selection was requested
		return nil
	}
	switch action.Name {
	case "stop":
		switch instStatus.Avail {
		case status.Down, status.StandbyUp, status.StandbyUpWithDown, status.NotApplicable, status.StandbyDown:
		default:
			return fmt.Errorf("the stop action returned no error but end avail status is %s", instStatus.Avail)
		}
	case "start":
		switch instStatus.Avail {
		case status.Up, status.NotApplicable, status.StandbyUpWithUp:
		default:
			return fmt.Errorf("the start action returned no error but end avail status is %s", instStatus.Avail)
		}
	}
	return nil
}

func (t *actor) mayFreeze(ctx context.Context) error {
	action := actioncontext.Props(ctx)
	if !action.Freeze {
		return nil
	}
	if !resourceselector.FromContext(ctx, nil).IsZero() {
		t.log.Debug().Msg("Skip freeze: Resource selection")
		return nil
	}
	if !t.orchestrateWantsFreeze() {
		t.log.Debug().Msg("Skip freeze: Orchestrate value")
		return nil
	}
	if env.HasDaemonOrigin() {
		t.log.Debug().Msg("Skip freeze: Action has daemon origin")
		return nil
	}
	return t.Freeze(ctx)
}

func (t *actor) orchestrateWantsFreeze() bool {
	switch t.Orchestrate() {
	case "ha", "start":
		return true
	default:
		return false
	}
}
