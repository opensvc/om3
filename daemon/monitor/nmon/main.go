// Package nmon is responsible for of local instance state
//
//	It provides the cluster data:
//		["cluster", "node", <localhost>, "services", "status", <instance>, "monitor"]
//		["cluster", "node", <localhost>, "services", "nmon", <instance>]
//
//	nmon are created by the local instcfg, with parent context instcfg context.
//	instcfg done => nmon done
//
//	worker watches on local instance status updates to clear reached status
//		=> unsetStatusWhenReached
//		=> orchestrate
//		=> pub new state if change
//
//	worker watches on remote instance nmon updates converge global expects
//		=> convergeGlobalExpectFromRemote
//		=> orchestrate
//		=> pub new state if change
package nmon

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	nmon struct {
		state         cluster.NodeMonitor
		previousState cluster.NodeMonitor

		ctx      context.Context
		cancel   context.CancelFunc
		cmdC     chan *msgbus.Msg
		dataCmdC chan<- any
		log      zerolog.Logger

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes []string
		nmons      map[string]cluster.NodeMonitor

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
	statusIdle         = "idle"
	statusThawedFailed = "unfreeze failed"
	statusFreezeFailed = "freeze failed"
	statusFreezing     = "freezing"
	statusThawing      = "thawing"
	statusShutting     = "shutting"
	statusMaintenance  = "maintenance"
	statusInit         = "init"
	statusUpgrade      = "upgrade"
	statusRejoin       = "rejoin"

	localExpectUnset = ""

	globalExpectAbort  = "abort"
	globalExpectFrozen = "frozen"
	globalExpectThawed = "thawed"
	globalExpectUnset  = ""

	// the node monitor states evicting a node from ranking algorithms
	statusUnrankable = map[string]bool{
		statusMaintenance: true,
		statusUpgrade:     true,
		statusInit:        true,
		statusShutting:    true,
		statusRejoin:      true,
	}
)

// Start launches the nmon worker goroutine
func Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)

	previousState := cluster.NodeMonitor{
		GlobalExpect: globalExpectUnset,
		Status:       statusIdle,
	}
	state := previousState

	o := &nmon{
		state:         state,
		previousState: previousState,
		ctx:           ctx,
		cancel:        cancel,
		cmdC:          make(chan *msgbus.Msg),
		dataCmdC:      daemondata.BusFromContext(ctx),
		log:           log.Logger.With().Str("func", "nmon").Logger(),
		localhost:     hostname.Hostname(),
		change:        true,
		nmons:         make(map[string]cluster.NodeMonitor),
	}

	go o.worker()
	return nil
}

// worker watch for local nmon updates
func (o *nmon) worker() {
	defer o.log.Debug().Msg("done")

	bus := pubsub.BusFromContext(o.ctx)
	defer msgbus.UnSub(bus, msgbus.SubNmon(bus, pubsub.OpUpdate, "nmon nmon.update", o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubNmon(bus, pubsub.OpDelete, "nmon nmon.delete", o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubFrozenFile(bus, pubsub.OpUpdate, "nmon frozenFile.update", "", o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubFrozenFile(bus, pubsub.OpDelete, "nmon frozenFile.delete", "", o.onEv))
	defer msgbus.UnSub(bus, msgbus.SubSetNmon(bus, "nmon setnmon", o.onEv))

	initialNodes := strings.Fields(rawconfig.ClusterSection().Nodes)
	for _, node := range initialNodes {
		o.nmons[node] = daemondata.GetNmon(o.dataCmdC, node)
	}
	o.updateIfChange()
	defer o.delete()
	defer msgbus.DropPendingMsg(o.cmdC, time.Second)
	o.log.Debug().Msg("started")

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.cmdC:
			switch c := (*i).(type) {
			case msgbus.SetNmon:
				o.onSetNmonCmd(c)
			case msgbus.NmonUpdated:
				o.onNmonUpdated(c)
			case msgbus.NmonDeleted:
				o.onNmonDeleted(c)
			case msgbus.FrozenFileRemoved:
				o.onFrozenFileRemoved(c)
			case msgbus.FrozenFileUpdated:
				o.onFrozenFileUpdated(c)
			case cmdOrchestrate:
				o.onOrchestrate(c)
			}
		}
	}
}

func (o *nmon) onEv(i interface{}) {
	o.cmdC <- msgbus.NewMsg(i)
}

func (o *nmon) delete() {
	if err := daemondata.DelNmon(o.dataCmdC); err != nil {
		o.log.Error().Err(err).Msg("DelNmon")
	}
}

func (o *nmon) update() {
	newValue := o.state
	if err := daemondata.SetNmon(o.dataCmdC, newValue); err != nil {
		o.log.Error().Err(err).Msg("SetNmon")
	}
}

// updateIfChange log updates and publish new state value when changed
func (o *nmon) updateIfChange() {
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
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		from, to := o.logFromTo(previousVal.GlobalExpect, newVal.GlobalExpect)
		o.log.Info().Msgf("change monitor global expect %s -> %s", from, to)
	}
	o.previousState = o.state
	o.update()
}

func (o *nmon) hasOtherNodeActing() bool {
	for remoteNode, remoteNmon := range o.nmons {
		if remoteNode == o.localhost {
			continue
		}
		if strings.HasSuffix(remoteNmon.Status, "ing") {
			return true
		}
	}
	return false
}

func (o *nmon) createPendingWithCancel() {
	o.pendingCtx, o.pendingCancel = context.WithCancel(o.ctx)
}

func (o *nmon) createPendingWithDuration(duration time.Duration) {
	o.pendingCtx, o.pendingCancel = context.WithTimeout(o.ctx, duration)
}

func (o *nmon) clearPending() {
	if o.pendingCancel != nil {
		o.pendingCancel()
		o.pendingCancel = nil
		o.pendingCtx = nil
	}
}

func (o *nmon) logFromTo(from, to string) (string, string) {
	if from == "" {
		from = "unset"
	}
	if to == "" {
		to = "unset"
	}
	return from, to
}
