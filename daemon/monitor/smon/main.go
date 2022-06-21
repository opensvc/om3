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
//
package smon

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/timestamp"
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

		// updated data from aggregated status update srcEvent
		instStatus map[string]instance.Status
		instSmon   map[string]instance.Monitor
		scopeNodes []string

		svcAgg      object.AggregatedStatus
		cancelReady context.CancelFunc
		localhost   string
		change      bool
	}

	cmdReady struct {
		ctx context.Context
	}
)

var (
	statusIdle     = ""
	statusReady    = "ready"
	statusStarting = "starting"
	statusStopping = "stopping"

	localExpectStarted = "started"
	localExpectUnset   = ""

	globalExpectUnset   = ""
	globalExpectStarted = "started"
	globalExpectStopped = "stopped"

	readyDuration = 1 * time.Second
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
		dataCmdC:      daemonctx.DaemonDataCmd(ctx),
		log:           daemonctx.Logger(ctx).With().Str("_smon", p.String()).Logger(),
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
	defer o.log.Info().Msg("done")

	c := daemonctx.DaemonPubSubCmd(o.ctx)
	defer ps.UnSub(c, ps.SubSvcAgg(c, pubsub.OpUpdate, "smon agg.update", o.id, o.onEv))
	defer ps.UnSub(c, ps.SubSetSmon(c, pubsub.OpUpdate, "smon setSmon.update", o.id, o.onEv))
	defer ps.UnSub(c, ps.SubSmon(c, pubsub.OpUpdate, "smon smon.update", o.id, o.onEv))

	for _, node := range initialNodes {
		o.instStatus[node] = daemondata.GelInstanceStatus(o.dataCmdC, o.path, node)
	}
	o.updateIfChange()
	defer o.delete()

	defer moncmd.DropPendingCmd(o.cmdC, time.Second)
	go o.status()
	o.log.Info().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.cmdC:
			switch c := (*i).(type) {
			case moncmd.MonSvcAggUpdated:
				if c.SrcEv != nil {
					switch srcCmd := (*c.SrcEv).(type) {
					case moncmd.InstStatusUpdated:
						srcNode := srcCmd.Node
						if _, ok := o.instStatus[srcNode]; !ok {
							continue
						}
						instStatus := srcCmd.Status
						o.instStatus[srcNode] = instStatus
						if srcNode == o.localhost {
							o.unsetStatusWhenReached(instStatus)
						}
					case moncmd.CfgUpdated:
						if srcCmd.Node != o.localhost {
							continue
						}
						cfgNodes := make(map[string]struct{})
						for _, node := range srcCmd.Config.Scope {
							cfgNodes[node] = struct{}{}
							if _, ok := o.instStatus[node]; !ok {
								o.instStatus[node] = instance.Status{Avail: status.Undef}
							}
						}
						for node := range o.instStatus {
							if _, ok := cfgNodes[node]; !ok {
								o.log.Info().Msgf("drop not anymore in local config status from node %s", node)
								delete(o.instStatus, node)
							}
						}
						o.scopeNodes = append([]string{}, srcCmd.Config.Scope...)
					}
				}
				o.cmdSvcAggUpdated(c.SvcAgg)
			case moncmd.SetSmon:
				o.cmdSetSmonClient(c.Monitor)
			case moncmd.SmonUpdated:
				node := c.Node
				if node == o.localhost {
					continue
				}
				instSmon := c.Status
				o.log.Debug().Msgf("updated instance smon from node %s  -> %s", node, instSmon.GlobalExpect)
				o.instSmon[node] = instSmon
				o.convergeGlobalExpectFromRemote()
				o.updateIfChange()
				o.orchestrate()
				o.updateIfChange()
			case cmdReady:
				o.cmdTryLeaveReady(c.ctx)
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
	o.state.StatusUpdated = timestamp.Now()
	previousVal := o.previousState
	newVal := o.state
	if newVal.Status != previousVal.Status {
		o.log.Info().Msgf("change monitor status %s -> %s", previousVal.Status, newVal.Status)
	}
	if newVal.LocalExpect != previousVal.LocalExpect {
		o.log.Info().Msgf("change local expect %s -> %s", previousVal.LocalExpect, newVal.LocalExpect)
	}
	if newVal.GlobalExpect != previousVal.GlobalExpect {
		o.log.Info().Msgf("change global expect %s -> %s", previousVal.GlobalExpect, newVal.GlobalExpect)
	}
	o.previousState = o.state
	o.update()
}

// unsetStatusWhenReached
func (o *smon) unsetStatusWhenReached(localInstanceStatus instance.Status) {
	localStatus := o.state.Status
	switch {
	case localStatus == statusIdle:
		return
	case localStatus == statusStarting || localStatus == statusReady:
		if localInstanceStatus.Avail == status.Up {
			o.log.Info().Msgf("reached local instance status: %s smon.status: %s", localInstanceStatus.Avail, localStatus)
			o.change = true
			o.state.Status = statusIdle
			o.state.LocalExpect = localExpectStarted
			if o.cancelReady != nil {
				o.cancelReady()
				o.cancelReady = nil
			}
		}
	case localStatus == statusStopping:
		if localInstanceStatus.Avail == status.Down {
			o.log.Info().Msgf("reached local instance status: %s smon.status: %s", localInstanceStatus.Avail, localStatus)
			o.change = true
			o.state.Status = statusIdle
			o.state.LocalExpect = localExpectUnset
			if o.cancelReady != nil {
				o.cancelReady()
				o.cancelReady = nil
			}
		}
	}
}

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (o *smon) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, state := range o.instStatus {
		nodeTime := state.Monitor.GlobalExpectUpdated.Time()
		if nodeTime.After(mostRecentUpdated) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentNode != o.localhost {
		o.change = true
		o.state.GlobalExpect = o.instStatus[mostRecentNode].Monitor.GlobalExpect
		o.state.GlobalExpectUpdated = o.instStatus[mostRecentNode].Monitor.GlobalExpectUpdated
		o.log.Info().Msgf("remote node %s has most recent global expect value %s", mostRecentNode, o.state.GlobalExpect)
	}
}

func (o *smon) hasOtherNodeActing() bool {
	for node, instanceStatus := range o.instStatus {
		if node == o.localhost {
			continue
		}
		switch instanceStatus.Monitor.Status {
		case statusReady:
			return true
		case statusStarting:
			return true
		}
	}
	return false
}
