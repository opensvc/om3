package object

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/freeze"
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
	os.Setenv("OPENSVC_SID", xsession.ID.String())
	if leader {
		os.Setenv("OPENSVC_LEADER", "1")
	} else {
		os.Setenv("OPENSVC_LEADER", "0")
	}
	// each Setenv resource Driver will load its own env vars when actioned
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

func (t *actor) withTimeoutFromKeywords(ctx context.Context, kwNames []string) (context.Context, func()) {
	timeout, source := t.actionTimeout(kwNames)
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

func (t *actor) withRollbackTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	switch props.Name {
	case "provision":
		return t.withTimeoutFromKeywords(ctx, []string{"unprovision_timeout", "timeout"})
	case "start":
		return t.withTimeoutFromKeywords(ctx, []string{"stop_timeout", "timeout"})
	default:
		return ctx, func() {}
	}
}

func (t *actor) withStatusTimeout(ctx context.Context) (context.Context, func()) {
	return t.withTimeoutFromKeywords(ctx, []string{"status_timeout", "timeout"})
}

func (t *actor) withActionTimeout(ctx context.Context) (context.Context, func()) {
	props := actioncontext.Props(ctx)
	return t.withTimeoutFromKeywords(ctx, props.TimeoutKeywords)
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
	s := actioncontext.Props(ctx).Progress
	if s == "" {
		// we did not announce at the beginning of the action
		return nil
	}
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
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
				if sysErr.Err == syscall.ECONNREFUSED {
					t.log.Debugf("skip announce progress: the daemon connection is refused")
					return nil
				}
			}
		}
		t.log.Errorf("announcing %s state: %s", progress, err)
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

func (t *actor) abortStart(ctx context.Context, resources resource.Drivers) (err error) {
	if actioncontext.Props(ctx).Name != "start" {
		return nil
	}
	if err := t.abortStartAffinity(ctx); err != nil {
		return err
	}
	return t.abortStartDrivers(ctx, resources)
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

func (t *actor) abortStartDrivers(ctx context.Context, resources resource.Drivers) (err error) {
	sb := statusbus.FromContext(ctx)
	added := 0
	q := make(chan bool, len(resources))
	var wg sync.WaitGroup
	for _, r := range resources {
		if v, err := t.isEncapNodeMatchingResource(r); err != nil {
			return err
		} else if !v {
			return nil
		}

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
	var a, b string
	switch avail {
	case status.Undef:
		a = "?"
	case status.NotApplicable:
		a = "/"
	case status.Up:
		a = "O"
	case status.StandbyUp:
		a = "o"
	case status.Down:
		a = "X"
	case status.StandbyDown:
		a = "x"
	case status.Warn:
		a = "!"
	}
	switch overall {
	case status.Warn:
		b = "!"
	}
	return fmt.Sprintf("%s%s", a, b)
}

func (t *actor) action(ctx context.Context, fn resourceset.DoFunc) error {
	if t.IsDisabled() {
		return ErrDisabled
	}
	t.pg = t.pgConfig("")
	wd, _ := os.Getwd()
	action := actioncontext.Props(ctx)
	barrier := actioncontext.To(ctx)
	resourceSelector := resourceselector.FromContext(ctx, t)
	resources := resourceSelector.Resources()
	isDesc := resourceSelector.IsDesc()
	isActionForMaster := actioncontext.IsActionForMaster(ctx)
	hasEncapResourcesSelected := false
	encaperRIDsAddedForSelectedEncapResources := make([]string, 0)

	if len(resources) == 0 && !resourceSelector.IsZero() {
		return fmt.Errorf("resource does not exist")
	}

	for _, r := range t.Resources() {
		if !hasEncapResourcesSelected && r.IsEncap() {
			hasEncapResourcesSelected = true
		}
		if _, ok := r.(encaper); ok {
			if !resources.HasRID(r.RID()) {
				encaperRIDsAddedForSelectedEncapResources = append(encaperRIDsAddedForSelectedEncapResources, r.RID())
			}
		}
	}

	if hasEncapResourcesSelected {
		resourceSelector.SelectRIDs(encaperRIDsAddedForSelectedEncapResources)
	}

	logger := t.log.
		Attr("argv", os.Args).
		Attr("cwd", wd).
		Attr("action", action.Name).
		Attr("origin", env.Origin()).
		Attr("crm", "true")
	logger.Infof(">>> do %s %s (origin %s, sid %s)", action.Name, os.Args, env.Origin(), xsession.ID)
	beginTime := time.Now()
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer func() {
		sb := statusbus.FromContext(ctx)
		statusIcon := sb.Get("avail").String()
		if overallStatus := sb.Get("overall"); overallStatus == status.Warn {
			statusIcon += ", with warnings"
		}
		logger.Attr("duration", time.Now().Sub(beginTime)).Infof("<<< done %s %s in %s, instance status is now %s", action.Name, os.Args, time.Now().Sub(beginTime), statusIcon)
	}()

	// daemon instance monitor updates
	t.announceProgressing(ctx)

	if mgr := pg.FromContext(ctx); mgr != nil {
		mgr.Register(t.pg)
	}

	// TODO: clarify timeouts: does start_timeout includes the eventual rollback,
	//       statusEval, postStartStopStatusEval, announceProgress "idle" and "failed"
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctxWithTimeout, cancelCtxWithTimeout := t.withActionTimeout(ctx)
	defer cancelCtxWithTimeout()

	freeze := func() error {
		if !action.Freeze {
			return nil
		}
		if !resourceSelector.IsZero() {
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
		if err := freeze.Freeze(t.path.FrozenFile()); err != nil {
			return err
		}
		return nil
	}

	if err := freeze(); err != nil {
		_, _ = t.statusEval(ctxWithTimeout)
		t.announceFailure(ctxWithTimeout)
		return err
	}

	doEncap := func(ctx context.Context, r resource.Driver) error {
		encapContainer, ok := r.(encaper)
		if !ok {
			return nil
		}

		hostname := encapContainer.GetHostname()

		if !actioncontext.IsActionForSlave(ctx, hostname) && !hasEncapResourcesSelected {
			return nil
		}

		configFile := t.path.ConfigFile()
		rid := r.RID()

		if v, err := t.Config().IsInEncapNodes(hostname); err != nil {
			return err
		} else if !v {
			return nil
		}

		args := append([]string{encapContainer.GetOsvcRootPath(), t.path.String()}, "config", "mtime")
		envs := []string{
			"OSVC_SESSION_ID=" + xsession.ID.String(),
			env.OriginSetenvArg(env.Origin()),
		}
		if s := os.Getenv(env.ActionOrchestrationIDVar); s != "" {
			envs = append(envs, env.ActionOrchestrationIDVar+"="+s)
		}
		cmd := encapContainer.EncapCmd(ctx, args, envs)
		err := cmd.Run()
		if err != nil {
			switch cmd.ProcessState.ExitCode() {
			case 2:
				if err := encapContainer.EncapCp(ctx, configFile, configFile); err != nil {
					return err
				}
			case 128:
				return fmt.Errorf("opensvc is not installed in the container")
			default:
				return err
			}
		}

		options := make([]string, 0)
		if s := actioncontext.RID(ctx); s != "" {
			options = append(options, "--rid", s)
		}
		if s := actioncontext.Subset(ctx); s != "" {
			options = append(options, "--subset", s)
		}
		if s := actioncontext.Tag(ctx); s != "" {
			options = append(options, "--tag", s)
		}
		if s := actioncontext.To(ctx); s != "" {
			options = append(options, "--to", s)
		}
		if s := actioncontext.IsLeader(ctx); s {
			options = append(options, "--leader")
		}
		if s := actioncontext.IsRollbackDisabled(ctx); s {
			options = append(options, "--disable-rollback")
		}

		args = append([]string{encapContainer.GetOsvcRootPath(), t.path.String(), "instance", action.Name}, options...)
		cmd = encapContainer.EncapCmd(ctx, args, envs)
		t.log.Infof("%s", strings.Join(cmd.Args, " "))

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("error creating StderrPipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("Error starting command: %w", err)
		}

		var wg sync.WaitGroup

		// Goroutine to read stderr line by line
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				l := strings.Fields(line)
				if len(l) > 2 {
					switch {
					case strings.Contains(l[1], "DBG"):
						t.log.Debugf("%s: %s", rid, line)
					case strings.Contains(l[1], "INF"):
						t.log.Infof("%s: %s", rid, line)
					case strings.Contains(l[1], "WRN"):
						t.log.Warnf("%s: %s", rid, line)
					case strings.Contains(l[1], "ERR"):
						t.log.Errorf("%s: %s", rid, line)
					}
				} else {
					t.log.Errorf("%s: %s", rid, line)
				}
			}
			if err := scanner.Err(); err != nil && err != io.EOF {
				t.log.Errorf("error reading stderr: %v", err)
			}
		}()

		wg.Wait()
		return cmd.Wait()
	}

	encapWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			// do host action before encap if ascending
			if !isDesc && isActionForMaster && !slices.Contains(encaperRIDsAddedForSelectedEncapResources, r.RID()) {
				if err := fn(ctx, r); err != nil {
					return err
				}
			}

			if err := doEncap(ctx, r); err != nil {
				return nil
			}

			// do host action after encap if descending
			if isDesc && isActionForMaster && !slices.Contains(encaperRIDsAddedForSelectedEncapResources, r.RID()) {
				if err := fn(ctx, r); err != nil {
					return err
				}
			}
			return nil
		}
	}

	progressWrap := func(fn resourceset.DoFunc) resourceset.DoFunc {
		return func(ctx context.Context, r resource.Driver) error {
			if v, err := t.isEncapNodeMatchingResource(r); err != nil {
				return err
			} else if !v {
				return nil
			}
			logger := t.log.Attr("rid", r.RID())
			ctx = logger.WithContext(ctx)
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
				if name := linkToer.LinkTo(); name != "" && resources.HasRID(name) {
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
				rids := resources.LinkersRID(names)
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
				if isDesc {
					if err := t.ResourceSets().Do(ctx, resourceSelector, barrier, "linked-"+action.Name, progressWrap(filter(fn))); err != nil {
						return err
					}
				}
				if err := fn(ctx, r); err != nil {
					return err
				}
				// On ascending action, do action on linkers last.
				if !isDesc {
					if err := t.ResourceSets().Do(ctx, resourceSelector, barrier, "linked-"+action.Name, progressWrap(filter(fn))); err != nil {
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
	t.ResourceSets().Do(ctxWithTimeout, resourceSelector, barrier, "pre-"+action.Name+" status", func(ctx context.Context, r resource.Driver) error {
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

	if err := t.abortStart(ctxWithTimeout, resources); err != nil {
		_, _ = t.statusEval(ctxWithTimeout)
		t.announceIdle(ctxWithTimeout)
		return err
	}
	if err := t.ResourceSets().Do(ctxWithTimeout, resourceSelector, barrier, action.Name, progressWrap(linkWrap(encapWrap(fn)))); err != nil {
		if t.needRollback(ctxWithTimeout) {
			if rb := actionrollback.FromContext(ctxWithTimeout); rb != nil {
				t.Log().Infof("rollback")
				ctxWithTimeout, cancelCtxWithTimeout := t.withRollbackTimeout(ctx)
				defer cancelCtxWithTimeout()
				if errRollback := rb.Rollback(ctxWithTimeout); errRollback != nil {
					t.Log().Errorf("rollback: %s", err)
				}
			}
		}
		ctxWithTimeout, cancelCtxWithTimeout = t.withStatusTimeout(ctx)
		defer cancelCtxWithTimeout()
		_, _ = t.statusEval(ctxWithTimeout)
		t.announceFailure(ctxWithTimeout)
		return err
	}

	// the action is done without error ... start a new timeout for status eval
	ctxWithTimeout, cancelCtxWithTimeout = t.withStatusTimeout(ctx)
	defer cancelCtxWithTimeout()

	if action.Order.IsDesc() {
		t.CleanPG(ctxWithTimeout)
	}
	err := t.postStartStopStatusEval(ctxWithTimeout)
	if err == nil {
		t.announceIdle(ctxWithTimeout)
	} else {
		t.log.Errorf("%s", err)
		t.announceFailure(ctxWithTimeout)
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
	if len(actioncontext.Slaves(ctx)) > 0 || actioncontext.AllSlaves(ctx) || actioncontext.Master(ctx) {
		// don't verify instance avail if a encap selection was requested
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

func (t *actor) orchestrateWantsFreeze() bool {
	switch t.Orchestrate() {
	case "ha", "start":
		return true
	default:
		return false
	}
}
