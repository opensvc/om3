// Package smon is responsible for of local instance state
//
//	It provides the cluster data:
//		["monitor", "nodes", <localhost>, "services", "status", <instance>, "monitor"]
//		["monitor", "nodes", <localhost>, "services", "smon", <instance>]
//
//	smon are created by the local instcfg, with parent context instcfg context.
//	instcfg done => smon done
//
//	worker watches on local instance status updates to clear reached status
//		=> unsetStatusWhenReached
//		=> orchestrate
//		=> pub new state if change
//
//	worker watches on remote instance smon updates converge global expects
//		=> convergeGlobalExpectFromRemote
//		=> orchestrate
//		=> pub new state if change
package smon

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemondata"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	smon struct {
		state         instance.Monitor
		previousState instance.Monitor

		path     path.T
		id       string
		ctx      context.Context
		cancel   context.CancelFunc
		cmdC     chan *moncmd.T
		dataCmdC chan<- interface{}
		log      zerolog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		// updated data from aggregated status update srcEvent
		instStatus map[string]instance.Status
		instSmon   map[string]instance.Monitor
		scopeNodes []string

		svcAgg      object.AggregatedStatus
		cancelReady context.CancelFunc
		localhost   string
		change      bool
	}

	// cmdOrchestrate can be used from post action go routines
	cmdOrchestrate struct {
		state    string
		newState string
	}
)

var (
	statusDeleted           = "deleted"
	statusDeleting          = "deleting"
	statusFreezeFailed      = "freeze failed"
	statusFreezing          = "freezing"
	statusIdle              = "idle"
	statusProvisioned       = "provisioned"
	statusProvisioning      = "provisioning"
	statusProvisionFailed   = "provision failed"
	statusPurgeFailed       = "purge failed"
	statusReady             = "ready"
	statusStarted           = "started"
	statusStartFailed       = "start failed"
	statusStarting          = "starting"
	statusStopFailed        = "stop failed"
	statusStopping          = "stopping"
	statusThawedFailed      = "unfreeze failed"
	statusThawing           = "thawing"
	statusUnProvisioned     = "unprovisioned"
	statusUnProvisionFailed = "unprovision failed"
	statusUnProvisioning    = "unprovisioning"
	statusWaitLeader        = "wait leader"

	localExpectStarted = "started"
	localExpectUnset   = ""

	globalExpectAbort         = "abort"
	globalExpectFrozen        = "frozen"
	globalExpectProvisioned   = "provisioned"
	globalExpectPurged        = "purged"
	globalExpectStarted       = "started"
	globalExpectStopped       = "stopped"
	globalExpectThawed        = "thawed"
	globalExpectUnProvisioned = "unprovisioned"
	globalExpectUnset         = ""
)

// Start launch goroutine smon worker for a local instance state
func Start(parent context.Context, p path.T, nodes []string) error {
	ctx, cancel := context.WithCancel(parent)
	id := p.String()

	previousState := instance.Monitor{
		GlobalExpect: globalExpectUnset,
		LocalExpect:  localExpectUnset,
		Status:       statusIdle,
		Placement:    "",
		Restart:      make(map[string]instance.MonitorRestart),
	}
	state := previousState

	o := &smon{
		state:         state,
		previousState: previousState,
		path:          p,
		id:            id,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan *moncmd.T),
		dataCmdC:      daemondata.BusFromContext(ctx),
		log:           log.Logger.With().Str("func", "smon").Stringer("object", p).Logger(),
		instStatus:    make(map[string]instance.Status),
		instSmon:      make(map[string]instance.Monitor),
		localhost:     hostname.Hostname(),
		scopeNodes:    nodes,
		change:        true,
	}

	go o.worker(nodes)
	return nil
}

// worker watch for local smon updates
func (o *smon) worker(initialNodes []string) {
	defer o.log.Debug().Msg("done")

	bus := pubsub.BusFromContext(o.ctx)
	defer ps.UnSub(bus, ps.SubSvcAgg(bus, pubsub.OpUpdate, "smon agg.update", o.id, o.onEv))
	defer ps.UnSub(bus, ps.SubSetSmon(bus, pubsub.OpUpdate, "smon setSmon.update", o.id, o.onEv))
	defer ps.UnSub(bus, ps.SubSmon(bus, pubsub.OpUpdate, "smon smon.update", o.id, o.onEv))

	for _, node := range initialNodes {
		o.instStatus[node] = daemondata.GetInstanceStatus(o.dataCmdC, o.path, node)
	}
	o.updateIfChange()
	defer o.delete()

	defer moncmd.DropPendingCmd(o.cmdC, time.Second)
	go o.crmStatus()
	o.log.Debug().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.cmdC:
			switch c := (*i).(type) {
			case moncmd.MonSvcAggUpdated:
				o.cmdSvcAggUpdated(c)
			case moncmd.SetSmon:
				o.cmdSetSmonClient(c.Monitor)
			case moncmd.SmonUpdated:
				o.cmdSmonUpdated(c)
			case cmdOrchestrate:
				o.needOrchestrate(c)
			}
		}
	}
}

func (o *smon) onEv(i interface{}) {
	o.cmdC <- moncmd.New(i)
}

func (o *smon) delete() {
	if err := daemondata.DelSmon(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelSmon")
	}
}

func (o *smon) update() {
	newValue := o.state
	if err := daemondata.SetSmon(o.dataCmdC, o.path, newValue); err != nil {
		o.log.Error().Err(err).Msg("SetSmon")
	}
}

// updateIfChange log updates and publish new state value when changed
func (o *smon) updateIfChange() {
	if !o.change {
		return
	}
	o.change = false
	o.state.StatusUpdated = time.Now()
	previousVal := o.previousState
	newVal := o.state
	if newVal.Status != previousVal.Status {
		o.log.Info().Msgf("change monitor state %s -> %s", previousVal.Status, newVal.Status)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		from, to := o.logFromTo(previousVal.LocalExpect, newVal.LocalExpect)
		o.log.Info().Msgf("change monitor local expect %s -> %s", from, to)
	}
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		from, to := o.logFromTo(previousVal.GlobalExpect, newVal.GlobalExpect)
		o.log.Info().Msgf("change monitor global expect %s -> %s", from, to)
	}
	o.previousState = o.state
	o.update()
}

func (o *smon) hasOtherNodeActing() bool {
	for remoteNode, remoteSmon := range o.instSmon {
		if remoteNode == o.localhost {
			continue
		}
		if strings.HasSuffix(remoteSmon.Status, "ing") {
			return true
		}
	}
	return false
}

func (o *smon) createPendingWithCancel() {
	o.pendingCtx, o.pendingCancel = context.WithCancel(o.ctx)
}

func (o *smon) createPendingWithDuration(duration time.Duration) {
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *smon) clearPending() {
	if o.pendingCancel != nil {
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *smon) logFromTo(from, to string) (string, string) {
	if from == "" {
		from = "unset"
	}
	if to == "" {
		to = "unset"
	}
	return from, to
}
