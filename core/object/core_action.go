package object

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceselector"
	"github.com/opensvc/om3/core/resourceset"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pg"
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
	thisEnv := t.Env()
	nodeEnv := t.node.Env()
	if thisEnv != "PRD" && nodeEnv == "PRD" {
		return fmt.Errorf("%w: not allowed to run on this node (svc env=%s node env=%s)", ErrInvalidNode, thisEnv, nodeEnv)
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
		t.Log().Debugf("skip rollback: empty stack")
		return false
	}
	action := actioncontext.Props(ctx)
	if !action.Rollback {
		t.Log().Debugf("skip rollback: not demanded by the %s action", action.Name)
		return false
	}
	if actioncontext.IsRollbackDisabled(ctx) {
		t.Log().Debugf("skip rollback: disabled via the command flag")
		return false
	}
	k := key.Parse("rollback")
	if !t.Config().GetBool(k) {
		t.Log().Debugf("skip rollback: disabled via configuration keyword")
		return false
	}
	return true
}

func (t *actor) withTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	timeout, source := t.actionTimeout(props.TimeoutKeywords)
	t.log.Debugf("action timeout set to %s from keyword %s", timeout, source)
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

func (t *actor) abortWorker(ctx context.Context, r resource.Driver, q chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	a, ok := r.(resource.Aborter)
	if !ok {
		q <- false
		return
	}
	if a.Abort(ctx) {
		t.log.Attr("rid", r.RID()).Errorf("resource %s denied start", r.RID())
		q <- true
		return
	}
	q <- false
}

func (t *actor) announceIdle(ctx context.Context) error {
	return t.announceProgress(ctx, "idle")
}

func (t *actor) announceFailure(ctx context.Context) error {
	s := actioncontext.Props(ctx).Failure
	return t.announceProgress(ctx, s)
}

func (t *actor) announceProgressing(ctx context.Context) error {
	s := actioncontext.Props(ctx).Progress
	return t.announceProgress(ctx, s)
}

// announceProgress signals the daemon that an action is in progress, using the
// POST /object/progress. This handler manages local expect:
// * set to "started" via InstanceMonitorUpdated event handler
// * set to "" if progress is idle
func (t *actor) announceProgress(ctx context.Context, progress string) error {
	if env.HasDaemonMonitorOrigin() {
		// no need to announce if the daemon started this action
		return nil
	}
	if progress == "" {
		return nil
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	isPartial := !resourceselector.FromContext(ctx, nil).IsZero()
	p := t.Path()
	resp, err := c.PostInstanceProgressWithResponse(ctx, p.Namespace, p.Kind, p.Name, api.PostInstanceProgress{
		State:     progress,
		SessionID: xsession.ID,
		IsPartial: &isPartial,
	})
	switch {
	case errors.Is(err, os.ErrNotExist):
		t.log.Debugf("skip announce progress: the daemon is not running")
		return nil
	case err != nil:
		t.log.Errorf("announced %s state: %s", progress, err)
		return err
	case resp.StatusCode() == http.StatusBadRequest:
		err := fmt.Errorf("announcing state %s: post instance progress request status: %s: %s", progress, resp.JSON400.Title, resp.JSON400.Detail)
		t.log.Errorf("%s", err)
		return err
	case resp.StatusCode() == http.StatusInternalServerError:
		err := fmt.Errorf("announcing state %s: post instance progress request status: %s: %s", progress, resp.JSON500.Title, resp.JSON500.Detail)
		t.log.Errorf("%s", err)
		return err
	case resp.StatusCode() != http.StatusOK:
		err := fmt.Errorf("announcing state %s: unexpected post instance progress request status: %s", progress, resp.Status())
		t.log.Errorf("%s", err)
		return err
	}
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
	if env.HasDaemonMonitorOrigin() {
		return nil
	}
	if actioncontext.IsForce(ctx) {
		return nil
	}
	for _, pStr := range t.HardAffinity() {
		p, err := naming.ParsePath(pStr)
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
		p, err := naming.ParsePath(pStr)
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

func instanceStatusIcon(avail, overall status.T) string {
	switch avail {
	case status.Undef, status.NotApplicable:
		return "⚪"
	case status.Up, status.StandbyUp:
		switch overall {
		case status.Warn:
			return "🟢⚠️"
		default:
			return "🟢"
		}
	case status.Down, status.StandbyDown:
		return "🔴"
	case status.Warn:
		return "🟠"
	default:
		return ""
	}
}

func (t *actor) action(ctx context.Context, fn resourceset.DoFunc) error {
	if t.IsDisabled() {
		return ErrDisabled
	}
	wd, _ := os.Getwd()
	action := actioncontext.Props(ctx)
	logger := t.log.
		Attr("argv", os.Args).
		Attr("cwd", wd).
		Attr("action", action.Name).
		Attr("origin", env.Origin()).
		Attr("crm", "true")
	logger.Infof("do %s %s (origin %s, sid %s)", action.Name, os.Args, env.Origin(), xsession.ID)
	beginTime := time.Now()
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer func() {
		sb := statusbus.FromContext(ctx)
		statusIcon := instanceStatusIcon(sb.Get("avail"), sb.Get("overall"))
		logger.Attr("duration", time.Now().Sub(beginTime)).Infof("%s done %s %s in %s", statusIcon, action.Name, os.Args, time.Now().Sub(beginTime))
	}()

	// daemon instance monitor updates
	t.announceProgressing(ctx)

	if mgr := pg.FromContext(ctx); mgr != nil {
		mgr.Register(t.pg)
	}

	// Prepare alternate context without timeout, that can be used on situations
	// where initial context is DeadlineExceeded.
	// TODO: clarify timeouts: does start_timeout includes the eventual rollback,
	//       statusEval, postStartStopStatusEval, announceProgress "idle" and "failed"
	ctxWithoutTimeout, cancel1 := context.WithCancel(ctx)
	defer cancel1()

	ctx, cancel := t.withTimeout(ctx)
	defer cancel()
	if err := t.preAction(ctx); err != nil {
		_, _ = t.statusEval(ctx)
		t.announceFailure(ctx)
		return err
	}
	l := resourceselector.FromContext(ctx, t)
	b := actioncontext.To(ctx)

	progressWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			if v, err := t.isEncapNodeMatchingResource(r); err != nil {
				return err
			} else if !v {
				return nil
			}
			l := t.log.Attr("rid", r.RID())
			ctx = l.WithContext(ctx)
			err := fn(ctx, r)
			switch {
			case errors.Is(err, resource.ErrDisabled):
				err = nil
			case errors.Is(err, resource.ErrActionNotSupported):
				err = nil
			case errors.Is(err, resource.ErrActionPostponedToLinker):
				err = nil
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
						if !slices.Contains(rids, r.RID()) {
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

	// Pre action resource evaluation.
	// For action requirements like fs#1(up)
	var evaluated sync.Map
	t.ResourceSets().Do(ctx, l, b, "pre-"+action.Name+" status", func(ctx context.Context, r resource.Driver) error {
		if v, err := t.isEncapNodeMatchingResource(r); err != nil {
			return err
		} else if !v {
			return nil
		}

		for requiredRID := range r.Requires(action.Name).Requirements() {
			if _, ok := evaluated.Load(requiredRID); ok {
				continue
			}
			requiredResource := t.getConfiguredResourceByID(requiredRID)
			if requiredResource == nil {
				continue
			}
			resource.EvalStatus(ctx, requiredResource)
			evaluated.Store(requiredRID, true)
		}
		rid := r.RID()
		resource.EvalStatus(ctx, r)
		evaluated.Store(rid, true)
		return nil
	})

	if err := t.abortStart(ctx, l); err != nil {
		_, _ = t.statusEval(ctx)
		t.announceIdle(ctx)
		return err
	}
	if err := t.ResourceSets().Do(ctx, l, b, action.Name, progressWrap(linkWrap(fn))); err != nil {
		if t.needRollback(ctx) {
			if rb := actionrollback.FromContext(ctx); rb != nil {
				t.Log().Infof("rollback")
				ctx2, cancel2 := t.withTimeout(ctxWithoutTimeout)
				defer cancel2()
				if errRollback := rb.Rollback(ctx2); errRollback != nil {
					t.Log().Errorf("rollback: %s", err)
				}
			}
		}
		_, _ = t.statusEval(ctx)
		if err == nil {
			t.announceIdle(ctx)
		} else {
			t.announceFailure(ctx)
		}
		return err
	}
	if action.Order.IsDesc() {
		t.CleanPG(ctx)
	}
	err := t.postStartStopStatusEval(ctx)
	if err == nil {
		t.announceIdle(ctx)
	} else {
		t.log.Errorf("%s", err)
		t.announceFailure(ctx)
	}
	return err
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
		t.log.Debugf("skip freeze: resource selection")
		return nil
	}
	if !t.orchestrateWantsFreeze() {
		t.log.Debugf("skip freeze: orchestrate value")
		return nil
	}
	if env.HasDaemonMonitorOrigin() {
		t.log.Debugf("skip freeze: action has daemon origin")
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
