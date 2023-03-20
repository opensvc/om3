// Package icfg is responsible for local instance.Config
//
// New instConfig are created by daemon discover.
// It provides the cluster data at ["cluster", "node", localhost, "services",
// "config, <instance>]
// It watches local config file to load updates.
// It watches for local cluster config update to refresh scopes.
//
// The icfg also starts imon object (with icfg context)
// => this will end imon object
//
// The worker routine is terminated when config file is not any more present, or
// when daemon discover context is done.
package icfg

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

type (
	T struct {
		path                     path.T
		id                       string
		configure                object.Configurer
		filename                 string
		log                      zerolog.Logger
		lastMtime                time.Time
		localhost                string
		forceRefresh             bool
		published                bool
		databus                  *daemondata.T
		sub                      *pubsub.Subscription
		instanceConfig           instance.Config
		clusterConfig            cluster.Config
		instanceMonitorCtx       context.Context
		isInstanceMonitorStarted bool
		iMonStarter              IMonStarter
		// ctx is a context created from parent context
		ctx context.Context
		// cancel is a cancel func for icfg, used to stop ifg if error occurs
		cancel context.CancelFunc
	}

	IMonStarter interface {
		Start(parent context.Context, p path.T, nodes []string) error
	}
)

var (
	clusterPath = path.T{Name: "cluster", Kind: kind.Ccfg}

	configFileCheckError = errors.New("config file check")

	keyFlexMax          = key.New("DEFAULT", "flex_max")
	keyFlexMin          = key.New("DEFAULT", "flex_min")
	keyFlexTarget       = key.New("DEFAULT", "flex_target")
	keyMonitorAction    = key.New("DEFAULT", "monitor_action")
	keyNodes            = key.New("DEFAULT", "nodes")
	keyPlacement        = key.New("DEFAULT", "placement")
	keyPreMonitorAction = key.New("DEFAULT", "pre_monitor_action")
	keyPriority         = key.New("DEFAULT", "priority")
	keyTopology         = key.New("DEFAULT", "topology")
	keyOrchestrate      = key.New("DEFAULT", "orchestrate")
)

// Start launch goroutine instConfig worker for a local instance config
func Start(parent context.Context, p path.T, filename string, svcDiscoverCmd chan<- any, iMonStarter IMonStarter) error {
	localhost := hostname.Hostname()
	id := daemondata.InstanceId(p, localhost)
	ctx, cancel := context.WithCancel(parent)
	o := &T{
		instanceConfig: instance.Config{Path: p},
		path:           p,
		id:             id,
		log:            log.Logger.With().Str("func", "icfg").Stringer("object", p).Logger(),
		localhost:      localhost,
		forceRefresh:   false,
		databus:        daemondata.FromContext(ctx),
		filename:       filename,

		iMonStarter: iMonStarter,

		ctx:    ctx,
		cancel: cancel,
	}

	if err := o.setConfigure(); err != nil {
		return err
	}

	o.startSubscriptions(ctx)

	go func() {
		defer o.log.Debug().Msg("stopped")
		defer func() {
			cancel()
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Error().Err(err).Msg("subscription stop")
			}
			o.done(parent, svcDiscoverCmd)
		}()
		o.worker()
	}()

	return nil
}

func (o *T) startSubscriptions(ctx context.Context) {
	clusterId := clusterPath.String()
	bus := pubsub.BusFromContext(ctx)
	label := pubsub.Label{"path", o.path.String()}
	o.sub = bus.Sub(o.path.String() + " icfg")
	o.sub.AddFilter(msgbus.ConfigFileRemoved{}, label)
	o.sub.AddFilter(msgbus.ConfigFileUpdated{}, label)
	if last := o.sub.AddFilterGetLast(msgbus.ClusterConfigUpdated{}); last != nil {
		o.onClusterConfigUpdated(last.(msgbus.ClusterConfigUpdated))
	}
	if o.path.String() != clusterId {
		o.sub.AddFilter(msgbus.ConfigUpdated{}, pubsub.Label{"path", clusterId})
	}
	o.sub.Start()
}

func (o *T) startInstanceMonitor() (bool, error) {
	if len(o.instanceConfig.Scope) == 0 {
		o.log.Info().Msgf("wait scopes to create associated imon")
		return false, nil
	}
	o.log.Info().Msgf("starting imon worker...")
	// TODO refactor Start to use icfg context, and remove o.instanceMonitorCtx
	if err := o.iMonStarter.Start(o.instanceMonitorCtx, o.path, o.instanceConfig.Scope); err != nil {
		o.log.Error().Err(err).Msg("failure during start imon worker")
		return false, err
	}
	return true, nil
}

// worker watch for local instConfig config file updates until file is removed
func (o *T) worker() {
	var (
		err error
	)
	defer o.log.Debug().Msg("done")
	o.log.Debug().Msg("starting")

	// do once what we do later on msgbus.ConfigFileUpdated
	if err := o.configFileCheck(); err != nil {
		o.log.Warn().Err(err).Msg("initial configFileCheck")
		return
	}
	defer o.delete()

	instanceMonitorCtx, cancelInstanceMonitor := context.WithCancel(o.ctx)
	o.instanceMonitorCtx = instanceMonitorCtx
	defer cancelInstanceMonitor()
	if o.isInstanceMonitorStarted, err = o.startInstanceMonitor(); err != nil {
		o.log.Error().Err(err).Msg("fail to start imon worker")
		return
	}
	o.log.Debug().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated(c)
			case msgbus.ConfigFileRemoved:
				o.onConfigFileRemoved(c)
			case msgbus.ConfigUpdated:
				o.onConfigUpdated(c)
			case msgbus.ClusterConfigUpdated:
				o.onClusterConfigUpdated(c)
			}
		}
	}
}

func (o *T) onClusterConfigUpdated(c msgbus.ClusterConfigUpdated) {
	o.clusterConfig = c.Value
}

func (o *T) onConfigFileUpdated(c msgbus.ConfigFileUpdated) {
	var err error
	o.log.Debug().Msgf("recv %#v", c)
	if err = o.configFileCheck(); err != nil {
		o.log.Error().Err(err).Msg("configFileCheck error")
		o.cancel()
		return
	}
	if !o.isInstanceMonitorStarted {
		o.log.Info().Msgf("imon not yet started, try start")
		if o.isInstanceMonitorStarted, err = o.startInstanceMonitor(); err != nil {
			o.log.Error().Err(err).Msgf("imon start error")
			o.cancel()
			return
		}
	}

}

func (o *T) onConfigFileRemoved(c msgbus.ConfigFileRemoved) {
	o.cancel()
}

func (o *T) onConfigUpdated(c msgbus.ConfigUpdated) {
	o.log.Info().Msg("local cluster config changed => refresh cfg")
	o.forceRefresh = true
	if err := o.configFileCheck(); err != nil {
		o.cancel()
	}
}

// updateConfig update iConfig.cfg when newConfig differ from iConfig.cfg
func (o *T) updateConfig(newConfig *instance.Config) {
	if instance.ConfigEqual(&o.instanceConfig, newConfig) {
		o.log.Debug().Msg("no update required")
		return
	}
	o.instanceConfig = *newConfig
	if err := o.databus.SetInstanceConfig(o.path, *newConfig.DeepCopy()); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceConfig")
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
	cfg := o.instanceConfig
	cfg.Nodename = o.localhost
	cfg.Topology = o.getTopology(cf)
	cfg.Orchestrate = o.getOrchestrate(cf)
	cfg.Priority = o.getPriority(cf)
	cfg.Resources = o.getResources(cf)
	cfg.MonitorAction = o.getMonitorAction(cf)
	cfg.PlacementPolicy = o.getPlacementPolicy(cf)
	cfg.PreMonitorAction = cf.GetString(keyPreMonitorAction)
	cfg.Scope = scope
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Updated = mtime

	if cfg.Topology == topology.Flex {
		cfg.FlexMin = o.getFlexMin(cf)
		cfg.FlexMax = o.getFlexMax(cf)
		cfg.FlexTarget = o.getFlexTarget(cf, cfg.FlexMin, cfg.FlexMax)
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
		scope = o.clusterConfig.Nodes
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

func (o *T) getFlexTarget(cf *xconfig.T, min, max int) (target int) {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		target = cf.GetInt(keyFlexTarget)
	}
	switch {
	case target < min:
		target = min
	case target > max:
		target = max
	}
	return
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
