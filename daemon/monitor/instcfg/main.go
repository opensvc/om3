// Package instcfg is responsible for local instance.Config
//
// New instConfig are created by daemon discover.
// It provides the cluster data at ["cluster", "node", localhost, "services",
// "config, <instance>]
// It watches local config file to load updates.
// It watches for local cluster config update to refresh scopes.
//
// The instcfg also starts imon object (with instcfg context)
// => this will end imon object
//
// The worker routine is terminated when config file is not any more present, or
// when daemon discover context is done.
package instcfg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/monitor/imon"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/stringslice"
)

type (
	T struct {
		cfg instance.Config

		path         path.T
		id           string
		configure    object.Configurer
		filename     string
		log          zerolog.Logger
		lastMtime    time.Time
		localhost    string
		forceRefresh bool
		published    bool
		cmdC         chan any
		databus      *daemondata.T
		bus          *pubsub.Bus
		sub          *pubsub.Subscription
	}
)

var (
	dropMsgTimeout = 100 * time.Millisecond

	configFileCheckError = errors.New("config file check")

	keyFlexMax       = key.New("DEFAULT", "flex_max")
	keyFlexMin       = key.New("DEFAULT", "flex_min")
	keyFlexTarget    = key.New("DEFAULT", "flex_target")
	keyMonitorAction = key.New("DEFAULT", "monitor_action")
	keyNodes         = key.New("DEFAULT", "nodes")
	keyPlacement     = key.New("DEFAULT", "placement")
	keyPriority      = key.New("DEFAULT", "priority")
	keyTopology      = key.New("DEFAULT", "topology")
	keyOrchestrate   = key.New("DEFAULT", "orchestrate")
)

// Start launch goroutine instConfig worker for a local instance config
func Start(parent context.Context, p path.T, filename string, svcDiscoverCmd chan<- any) error {
	localhost := hostname.Hostname()
	id := daemondata.InstanceId(p, localhost)

	o := &T{
		cfg:          instance.Config{Path: p},
		path:         p,
		id:           id,
		log:          log.Logger.With().Str("func", "instcfg").Stringer("object", p).Logger(),
		localhost:    localhost,
		forceRefresh: false,
		cmdC:         make(chan any),
		databus:      daemondata.FromContext(parent),
		filename:     filename,
	}

	if err := o.setConfigure(); err != nil {
		return err
	}

	o.startSubscriptions(parent)

	go func() {
		defer o.log.Debug().Msg("stopped")
		defer func() {
			msgbus.DropPendingMsg(o.cmdC, dropMsgTimeout)
			o.sub.Stop()
			o.done(parent, svcDiscoverCmd)
		}()
		o.worker(parent)
	}()

	return nil
}

func (o *T) startSubscriptions(ctx context.Context) {
	o.bus = pubsub.BusFromContext(ctx)
	label := pubsub.Label{"path", o.path.String()}
	o.sub = o.bus.Sub(o.path.String() + " instcfg")
	o.sub.AddFilter(msgbus.ConfigFileUpdated{}, label)
	o.sub.AddFilter(msgbus.ConfigFileRemoved{}, label)
	if !o.path.Equal(path.Cluster) {
		o.sub.AddFilter(msgbus.ConfigUpdated{}, pubsub.Label{"path", path.Cluster.String()})
	}
	o.sub.Start()
}

func (o *T) startSmon(ctx context.Context) (bool, error) {
	if len(o.cfg.Scope) == 0 {
		o.log.Info().Msgf("wait scopes to create associated imon")
		return false, nil
	}
	o.log.Info().Msgf("starting imon worker...")
	if err := imon.Start(ctx, o.path, o.cfg.Scope); err != nil {
		o.log.Error().Err(err).Msg("failure during start imon worker")
		return false, err
	}
	return true, nil
}

// worker watch for local instConfig config file updates until file is removed
func (o *T) worker(parent context.Context) {
	var (
		hasSmon bool
		err     error
	)
	defer o.log.Debug().Msg("done")
	defer o.log.Debug().Msg("starting")

	// do once what we do later on msgbus.ConfigFileUpdated
	if err := o.configFileCheck(); err != nil {
		o.log.Warn().Err(err).Msg("initial configFileCheck")
		return
	}
	defer o.delete()

	imonCtx, cancelSmon := context.WithCancel(parent)
	defer cancelSmon()
	if hasSmon, err = o.startSmon(imonCtx); err != nil {
		o.log.Error().Err(err).Msg("fail to start imon worker")
		return
	}
	o.log.Debug().Msg("started")
	for {
		select {
		case <-parent.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.ConfigFileUpdated:
				o.log.Debug().Msgf("recv %#v", c)
				if err = o.configFileCheck(); err != nil {
					o.log.Error().Err(err).Msg("configFileCheck error")
					return
				}
				if !hasSmon {
					o.log.Info().Msgf("imon not yet started, try start")
					if hasSmon, err = o.startSmon(imonCtx); err != nil {
						o.log.Error().Err(err).Msgf("imon start error")
						return
					}
				}

			case msgbus.ConfigFileRemoved:
				o.log.Debug().Msgf("recv %#v", c)
				return
			case msgbus.ConfigUpdated:
				o.log.Debug().Msgf("recv %#v", c)
				if c.Node != o.localhost {
					// only watch local cluster config updates
					continue
				}
				o.log.Info().Msg("local cluster config changed => refresh cfg")
				o.forceRefresh = true
				if err = o.configFileCheck(); err != nil {
					return
				}
			}
		case i := <-o.cmdC:
			switch i.(type) {
			case msgbus.Exit:
				log.Debug().Msg("eat poison pill")
				return
			default:
				o.log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}

// checkClusterChanges informs daemondata about updated cluster scopes
// and publish msgbus.JoinSuccess,added=<newnode> for added nodes
func (o *T) checkClusterChanges(previous, new instance.Config) {
	labelNode := pubsub.Label{"node", hostname.Hostname()}
	o.log.Debug().Msgf("comparing scope: %s vs %s", previous.Scope, new.Scope)
	removed, added := stringslice.Diff(previous.Scope, new.Scope)
	if len(added) > 0 {
		o.log.Debug().Msgf("added nodes: %s", added)
	}
	if len(removed) > 0 {
		o.log.Debug().Msgf("removed nodes: %s", removed)
	}
	for _, v := range added {
		o.bus.Pub(
			msgbus.JoinSuccess{Node: v},
			labelNode,
			pubsub.Label{"added", v})
	}
	if (len(added) + len(removed)) > 0 {
		if err := o.databus.SetClusterConfig(cluster.ClusterConfig{Nodes: append([]string{}, new.Scope...)}); err != nil {
			o.log.Error().Err(err).Msg("SetClusterConfig")
		}
	}
}

// updateConfig update iConfig.cfg when newConfig differ from iConfig.cfg
func (o *T) updateConfig(newConfig *instance.Config) {
	if instance.ConfigEqual(&o.cfg, newConfig) {
		o.log.Debug().Msg("no update required")
		return
	}

    // previous value used checkClusterChanges when instance is cluster config
	previousConfig := o.cfg

	o.cfg = *newConfig
	if err := o.databus.SetInstanceConfig(o.path, *newConfig.DeepCopy()); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceConfig")
	}
	if o.path.Equal(path.Cluster) {
		o.checkClusterChanges(previousConfig, *newConfig)
	}
	o.published = true
}

// configFileCheck verify if config file has been changed
//
//		if config file absent cancel worker
//		if updated time or checksum has changed:
//	       reload load config
//		   updateConfig
//
//		when localhost is not anymore in scope then ends worker
func (o *T) configFileCheck() error {
	mtime := file.ModTime(o.filename)
	if mtime.IsZero() {
		o.log.Info().Msgf("configFile no mtime %s", o.filename)
		return configFileCheckError
	}
	if mtime.Equal(o.lastMtime) && !o.forceRefresh {
		o.log.Debug().Msg("same mtime, skip")
		return nil
	}
	checksum, err := file.MD5(o.filename)
	if err != nil {
		o.log.Info().Msgf("configFile no present(md5sum)")
		return configFileCheckError
	}
	if o.path.Equal(path.Cluster) {
		rawconfig.LoadSections()
	}
	if err := o.setConfigure(); err != nil {
		return configFileCheckError
	}
	o.forceRefresh = false
	cf := o.configure.Config()
	scope, err := o.getScope(cf)
	if err != nil {
		o.log.Error().Err(err).Msgf("can't get scope")
		return configFileCheckError
	}
	if len(scope) == 0 {
		o.log.Info().Msg("empty scope")
		return configFileCheckError
	}
	newMtime := file.ModTime(o.filename)
	if newMtime.IsZero() {
		o.log.Info().Msgf("configFile no more mtime %s", o.filename)
		return configFileCheckError
	}
	if !newMtime.Equal(mtime) {
		o.log.Info().Msg("configFile changed(wait next evaluation)")
		return nil
	}
	if !stringslice.Has(o.localhost, scope) {
		o.log.Info().Msg("localhost not anymore an instance node")
		return configFileCheckError
	}
	cfg := o.cfg
	cfg.Nodename = o.localhost
	cfg.Topology = o.getTopology(cf)
	cfg.Orchestrate = o.getOrchestrate(cf)
	cfg.Priority = o.getPriority(cf)
	cfg.Resources = o.getResources(cf)
	cfg.MonitorAction = o.getMonitorAction(cf)
	cfg.PlacementPolicy = o.getPlacementPolicy(cf)
	cfg.Scope = scope
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Updated = mtime

	if cfg.Topology == topology.Flex {
		cfg.FlexTarget = o.getFlexTarget(cf)
		cfg.FlexMin = o.getFlexMin(cf)
		cfg.FlexMax = o.getFlexMax(cf)
	}

	o.lastMtime = mtime
	o.updateConfig(&cfg)
	return nil
}

// getScope return sorted scopes for object
//
// depending on object kind
// Ccfg => cluster.nodes
// else => eval DEFAULT.nodes
func (o *T) getScope(cf *xconfig.T) (scope []string, err error) {
	switch o.path.Kind {
	case kind.Ccfg:
		scope = strings.Split(rawconfig.ClusterSection().Nodes, " ")
	default:
		var evalNodes interface{}
		evalNodes, err = cf.Eval(keyNodes)
		if err != nil {
			o.log.Error().Err(err).Msg("eval DEFAULT.nodes")
			return
		}
		scope = evalNodes.([]string)
	}
	return
}

func (o *T) getMonitorAction(cf *xconfig.T) instance.MonitorAction {
	s := cf.GetString(keyMonitorAction)
	return instance.MonitorAction(s)
}

func (o *T) getPlacementPolicy(cf *xconfig.T) placement.Policy {
	s := cf.GetString(keyPlacement)
	return placement.NewPolicy(s)
}

func (o *T) getTopology(cf *xconfig.T) topology.T {
	s := cf.GetString(keyTopology)
	return topology.New(s)
}

func (o *T) getOrchestrate(cf *xconfig.T) string {
	s := cf.GetString(keyOrchestrate)
	return s
}

func (o *T) getResources(cf *xconfig.T) map[string]instance.ResourceConfig {
	m := make(map[string]instance.ResourceConfig)
	for _, section := range cf.SectionStrings() {
		switch section {
		case "env", "DEFAULT":
			continue
		}
		m[section] = instance.ResourceConfig{
			RestartDelay: cf.GetDuration(key.New(section, "restart_delay")),
			Restart:      cf.GetInt(key.New(section, "restart")),
			IsDisabled:   cf.GetBool(key.New(section, "disable")),
			IsMonitored:  cf.GetBool(key.New(section, "monitor")),
		}
	}
	return m
}

func (o *T) getPriority(cf *xconfig.T) priority.T {
	s := cf.GetInt(keyPriority)
	return priority.T(s)
}

func (o *T) getFlexTarget(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		return cf.GetInt(keyFlexTarget)
	}
	return 0
}

func (o *T) getFlexMin(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		return cf.GetInt(keyFlexMin)
	}
	return 0
}

func (o *T) getFlexMax(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		if i, err := cf.GetIntStrict(keyFlexMax); err == nil {
			return i
		} else if scope, err := o.getScope(cf); err == nil {
			return len(scope)
		} else {
			return 0
		}
	default:
		return 0
	}
}

func (o *T) setConfigure() error {
	configure, err := object.NewConfigurer(o.path)
	if err != nil {
		o.log.Warn().Err(err).Msg("NewConfigurer failure")
		return err
	}
	o.configure = configure
	return nil
}

func (o *T) delete() {
	if o.published {
		if err := o.databus.DelInstanceConfig(o.path); err != nil {
			o.log.Error().Err(err).Msg("DelInstanceConfig")
		}
	}
	if err := o.databus.DelInstanceStatus(o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceStatus")
	}
}

func (o *T) done(parent context.Context, doneChan chan<- any) {
	op := msgbus.InstanceConfigManagerDone{
		Path:     o.path,
		Filename: o.filename,
	}
	select {
	case <-parent.Done():
		return
	case doneChan <- op:
	}
}
