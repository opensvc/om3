package object

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// collectorAction is the instance action reported to the collector, held
	// in the action context so the actions a command chains report once.
	collectorAction struct {
		a   collector.Action
		dir collector.ActionPendingDir
	}

	collectorActionKey struct{}
)

var (
	// collectorActionSignalTimeout bounds the signal to the local daemon,
	// which must not hold the action.
	collectorActionSignalTimeout = 2 * time.Second
)

// beginCollectorAction records the begin of the action named name, to be
// reported to the collector, and returns the context carrying the record
// and the function recording its end.
//
// The end function is given the action error and what recover() returned,
// from the action deferred function:
//
//	ctx, done := t.beginCollectorAction(ctx, name)
//	defer func() { done(err, recover()) }()
//
// so an action panicking ends err, rather than ok as its error is still nil,
// and panics again with the same value once recorded.
//
// The context of an action already recorded is returned as is, with a
// no-op end: a restart, or a provision and the stop ending it, report once.
// So does the context of an action not to report, so the actions it chains
// do not report either.
func (t *actor) beginCollectorAction(ctx context.Context, name string) (context.Context, func(err error, panicked any)) {
	// The end of an action chained by another one records nothing, but a
	// panic it recovered goes on to the chaining action.
	noop := func(_ error, panicked any) {
		if panicked != nil {
			panic(panicked)
		}
	}
	if _, ok := ctx.Value(collectorActionKey{}).(*collectorAction); ok {
		return ctx, noop
	}
	if !t.isCollectorAction() {
		return context.WithValue(ctx, collectorActionKey{}, (*collectorAction)(nil)), noop
	}
	rec := &collectorAction{
		dir: collector.ActionPendingDir(rawconfig.CollectorActionPendingDir()),
		a: collector.Action{
			Path:      t.path,
			Action:    name,
			Argv:      maskArgv(os.Args[1:]),
			RIDs:      t.collectorActionRIDs(ctx),
			Origin:    string(env.Origin()),
			SessionID: xsession.SessionID().UUID(),
			ExecID:    xsession.ExecID().UUID(),
			PID:       os.Getpid(),
			Begin:     time.Now(),
		},
	}
	t.saveCollectorAction(rec)
	ctx = context.WithValue(ctx, collectorActionKey{}, rec)
	return ctx, func(err error, panicked any) {
		rec.a.End = time.Now()
		if err == nil && panicked == nil {
			rec.a.Status = "ok"
		} else {
			rec.a.Status = "err"
		}
		t.saveCollectorAction(rec)
		if panicked != nil {
			panic(panicked)
		}
	}
}

// withCollectorActionOf returns ctx carrying the collector action record of
// from, for an action using contexts derived from different parents to
// report once.
func withCollectorActionOf(ctx, from context.Context) context.Context {
	if rec, ok := from.Value(collectorActionKey{}).(*collectorAction); ok {
		return context.WithValue(ctx, collectorActionKey{}, rec)
	}
	return ctx
}

// isCollectorAction tells whether the action is to report to the
// collector.
//
// Actions started by the daemon scheduler are not, nor the actions of an
// object not to keep a trace of, nor any action when the node does not
// feed a collector.
func (t *actor) isCollectorAction() bool {
	if env.HasDaemonSchedulerOrigin() {
		return false
	}
	if t.IsVolatile() || t.IsDisabled() {
		return false
	}
	n, err := t.Node()
	if err != nil {
		t.log.Tracef("skip collector action report: %s", err)
		return false
	}
	if !n.MergedConfig().GetBool(key.Parse("node.dblog")) {
		return false
	}
	cfg := n.CollectorRawConfig().AsConfig()
	return cfg.FeederUrl != "" && cfg.Password != ""
}

// collectorActionRIDs returns the rids of the resources the action is
// restricted to, comma-separated, and an empty string when it acts on all.
func (t *actor) collectorActionRIDs(ctx context.Context) string {
	var sel *resourceselector.T
	if actioncontext.HasProps(ctx) {
		sel = resourceselector.FromContext(ctx, t)
	} else {
		// An action chaining others, as a restart, carries no action
		// properties: its selection is resolved from its --rid, --tag and
		// --subset alone.
		sel = resourceselector.New(t,
			resourceselector.WithRID(actioncontext.RID(ctx)),
			resourceselector.WithTag(actioncontext.Tag(ctx)),
			resourceselector.WithSubset(actioncontext.Subset(ctx)),
		)
	}
	if sel.IsZero() {
		return ""
	}
	rids := make([]string, 0)
	for _, r := range sel.Resources() {
		rids = append(rids, r.RID())
	}
	return strings.Join(rids, ",")
}

// saveCollectorAction writes the pending file of the current phase of the
// action and signals the local daemon, which announces it to the collector
// speaker.
//
// Neither may fail the action. A daemon not running, or refusing the
// signal, only delays the report: the daemon announces the pending files
// when it starts, and every minute those not acknowledged.
func (t *actor) saveCollectorAction(rec *collectorAction) {
	phase := rec.a.Phase()
	if err := rec.dir.Write(rec.a); err != nil {
		t.log.Warnf("collector action %s: write the %s pending file: %s", rec.a.Action, phase, err)
		return
	}
	c, err := client.New()
	if err != nil {
		t.log.Tracef("collector action %s: signal the %s: %s", rec.a.Action, phase, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), collectorActionSignalTimeout)
	defer cancel()
	p := t.path
	resp, err := c.PostInstanceCollectorActionWithResponse(ctx, hostname.Hostname(), p.Namespace, p.Kind, p.Name, api.PostInstanceCollectorAction{
		ExecID: rec.a.ExecID,
		Phase:  string(phase),
	})
	switch {
	case errors.Is(err, os.ErrNotExist), isConnRefused(err):
		t.log.Tracef("collector action %s: skip the %s signal: the daemon is not running", rec.a.Action, phase)
	case err != nil:
		t.log.Tracef("collector action %s: signal the %s: %s", rec.a.Action, phase, err)
	case resp.StatusCode() != 200:
		t.log.Tracef("collector action %s: signal the %s: unexpected status %s", rec.a.Action, phase, resp.Status())
	}
}

func isConnRefused(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var sysErr *os.SyscallError
	if !errors.As(opErr.Err, &sysErr) {
		return false
	}
	return sysErr.Err == syscall.ECONNREFUSED
}

// secretFlags are the command flags whose values may hold secrets: a
// keyword value, or an environment variable exported to the action.
var secretFlags = []string{"--value", "--env"}

// maskArgv returns a copy of argv with the values of the secretFlags masked,
// as given in the "--flag value" or the "--flag=value" form.
//
// An --env value keeps the name of the variable it sets, as NAME=xxx: which
// variable an action got is worth reading, its value is not.
func maskArgv(argv []string) []string {
	masked := make([]string, len(argv))
	copy(masked, argv)
	for i := 0; i < len(masked); i++ {
		for _, flag := range secretFlags {
			switch {
			case masked[i] == flag:
				if i+1 < len(masked) {
					masked[i+1] = maskFlagValue(flag, masked[i+1])
					i++
				}
			case strings.HasPrefix(masked[i], flag+"="):
				masked[i] = flag + "=" + maskFlagValue(flag, strings.TrimPrefix(masked[i], flag+"="))
			default:
				continue
			}
			break
		}
	}
	return masked
}

func maskFlagValue(flag, value string) string {
	if flag == "--env" {
		if name, _, ok := strings.Cut(value, "="); ok {
			return name + "=xxx"
		}
	}
	return "xxx"
}
