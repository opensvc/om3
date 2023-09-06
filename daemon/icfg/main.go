// Package icfg is responsible for local instance.Config
//
// instConfig are created by daemon discover.
// It provides the cluster data at cluster.node.<localhost>.instance.<path>.config
// It reloads config updates:
//   - for not cluster config
//   - when on ConfigFileUpdated is fired
//   - when on InstanceConfigUpdated for local cluster is fired (scope may need refresh)
//   - for cluster config
//   - when on ClusterConfigUpdated for local node is fired
//
// The worker routine is terminated when ConfigFileUpdated is fired, or
// when daemon discover context is done.
package icfg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/resourceset"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/core/xconfig"
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
		configure                object.Configurer
		filename                 string
		log                      zerolog.Logger
		lastMtime                time.Time
		localhost                string
		forceRefresh             bool
		published                bool
		bus                      *pubsub.Bus
		sub                      *pubsub.Subscription
		instanceConfig           instance.Config
		instanceMonitorCtx       context.Context
		isInstanceMonitorStarted bool

		// ctx is a context created from parent context
		ctx context.Context
		// cancel is a cancel func for icfg, used to stop ifg if error occurs
		cancel context.CancelFunc
	}
)

var (
	clusterPath = path.T{Name: "cluster", Kind: kind.Ccfg}

	configFileCheckError = errors.New("config file check")

	keyApp              = key.New("DEFAULT", "app")
	keyChildren         = key.New("DEFAULT", "children")
	keyEnv              = key.New("DEFAULT", "env")
	keyFlexMax          = key.New("DEFAULT", "flex_max")
	keyFlexMin          = key.New("DEFAULT", "flex_min")
	keyFlexTarget       = key.New("DEFAULT", "flex_target")
	keyMonitorAction    = key.New("DEFAULT", "monitor_action")
	keyNodes            = key.New("DEFAULT", "nodes")
	keyOrchestrate      = key.New("DEFAULT", "orchestrate")
	keyParents          = key.New("DEFAULT", "parents")
	keyPlacement        = key.New("DEFAULT", "placement")
	keyPreMonitorAction = key.New("DEFAULT", "pre_monitor_action")
	keyPriority         = key.New("DEFAULT", "priority")
	keyTopology         = key.New("DEFAULT", "topology")
)

// Start launch goroutine instConfig worker for a local instance config
func Start(parent context.Context, p path.T, filename string, svcDiscoverCmd chan<- any) error {
	localhost := hostname.Hostname()
	ctx, cancel := context.WithCancel(parent)
	o := &T{
		instanceConfig: instance.Config{Path: p},
		path:           p,
		log:            log.Logger.With().Str("func", "icfg").Stringer("object", p).Logger(),
		localhost:      localhost,
		forceRefresh:   false,
		bus:            pubsub.BusFromContext(ctx),
		filename:       filename,

		ctx:    ctx,
		cancel: cancel,
	}

	if err := o.setConfigure(); err != nil {
		return err
	}

	o.startSubscriptions()

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

func (o *T) startSubscriptions() {
	clusterId := clusterPath.String()
	label := pubsub.Label{"path", o.path.String()}
	o.sub = o.bus.Sub(o.path.String() + " icfg")
	o.sub.AddFilter(&msgbus.ConfigFileRemoved{}, label)
	if o.path.String() != clusterId {
		o.sub.AddFilter(&msgbus.ConfigFileUpdated{}, label)

		// the scope value may depend on cluster nodes values: *, clusternodes ...
		// so we must also watch for cluster config updates to configFileCheckRefresh non cluster instance config scope
		localClusterLabels := []pubsub.Label{{"path", clusterId}, {"node", o.localhost}}
		o.sub.AddFilter(&msgbus.InstanceConfigUpdated{}, localClusterLabels...)
	} else {
		// Special note for cluster instance config: we don't subscribe for ConfigFileUpdated, instead we subscribe for
		// ClusterConfigUpdated.
		// The cluster instance config scope is computed from cached cluster nodes.
		// cached cluster nodes is updated by ccfg on ConfigFileUpdated event:
		//     on ConfigFileUpdated -> update cached cluster nodes -> publish ClusterConfigUpdated
		// So watch ConfigFileUpdated is replaced by ClusterConfigUpdated to ensure sequence:
		//     ccfg: ConfigFileUpdated =>  - update cached cluster nodes
		//                                 - ClusterConfigUpdated
		//     icfg:                         ClusterConfigUpdated         => cluster InstanceConfigUpdated
		o.sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", o.localhost})
	}
	o.sub.Start()
}

// worker watch for local instConfig config file updates until file is removed
func (o *T) worker() {
	defer o.log.Debug().Msg("done")
	o.log.Debug().Msg("starting")

	// do once what we do later on msgbus.ConfigFileUpdated
	if err := o.configFileCheck(); err != nil {
		o.log.Warn().Err(err).Msg("initial configFileCheck")
		return
	}
	defer o.delete()

	o.log.Debug().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch i.(type) {
			case *msgbus.ClusterConfigUpdated:
				o.onClusterConfigUpdated()
			case *msgbus.ConfigFileRemoved:
				o.onConfigFileRemoved()
			case *msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated()
			case *msgbus.InstanceConfigUpdated:
				o.onLocalClusterInstanceConfigUpdated()
			}
		}
	}
}

func (o *T) configFileCheckRefresh(force bool) error {
	if force {
		o.forceRefresh = true
	}
	err := o.configFileCheck()
	if err != nil {
		o.log.Error().Err(err).Msg("configFileCheck error")
		o.cancel()
	}
	return err
}

func (o *T) onClusterConfigUpdated() {
	o.log.Info().Msg("cluster config updated => refresh")
	_ = o.configFileCheckRefresh(true)
}

func (o *T) onConfigFileUpdated() {
	o.log.Info().Msgf("config file changed => refresh")
	_ = o.configFileCheckRefresh(false)
}

func (o *T) onLocalClusterInstanceConfigUpdated() {
	o.log.Info().Msg("cluster instance config changed => refresh")
	_ = o.configFileCheckRefresh(true)
}

func (o *T) onConfigFileRemoved() {
	o.cancel()
}

// updateConfig update iConfig.cfg when newConfig differ from iConfig.cfg
func (o *T) updateConfig(newConfig *instance.Config) {
	if instance.ConfigEqual(&o.instanceConfig, newConfig) {
		o.log.Debug().Msg("no update required")
		return
	}
	o.instanceConfig = *newConfig
	instance.ConfigData.Set(o.path, o.localhost, newConfig.DeepCopy())
	o.bus.Pub(&msgbus.InstanceConfigUpdated{Path: o.path, Node: o.localhost, Value: *newConfig.DeepCopy()},
		pubsub.Label{"path", o.path.String()},
		pubsub.Label{"node", o.localhost},
	)
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
	cfg.App = cf.GetString(keyApp)
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Children = o.getChildren(cf)
	cfg.Env = cf.GetString(keyEnv)
	cfg.MonitorAction = o.getMonitorAction(cf)
	cfg.Nodename = o.localhost
	cfg.Orchestrate = o.getOrchestrate(cf)
	cfg.Parents = o.getParents(cf)
	cfg.PlacementPolicy = o.getPlacementPolicy(cf)
	cfg.PreMonitorAction = cf.GetString(keyPreMonitorAction)
	cfg.Priority = o.getPriority(cf)
	cfg.Resources = o.getResources(cf)
	cfg.Scope = scope
	cfg.Topology = o.getTopology(cf)
	cfg.UpdatedAt = mtime
	cfg.Subsets = o.getSubsets(cf)

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
		scope = clusternode.Get()
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

func (o *T) getChildren(cf *xconfig.T) []path.Relation {
	l := cf.GetStrings(keyChildren)
	relations := make([]path.Relation, len(l))
	for i, s := range l {
		relations[i] = path.Relation(s)
	}
	return relations
}

func (o *T) getParents(cf *xconfig.T) []path.Relation {
	l := cf.GetStrings(keyParents)
	relations := make([]path.Relation, len(l))
	for i, s := range l {
		relations[i] = path.Relation(s)
	}
	return relations
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

func (o *T) getSubsets(cf *xconfig.T) map[string]instance.SubsetConfig {
	m := make(map[string]instance.SubsetConfig)
	for _, s := range cf.SectionStrings() {
		if name := resourceset.SubsetSectionToName(s); name == "" {
			continue
		}
		k := key.New(s, "parallel")
		m[s] = instance.SubsetConfig{
			Parallel: cf.GetBool(k),
		}
	}
	return m
}

func (o *T) getResources(cf *xconfig.T) instance.ResourceConfigs {
	m := make(instance.ResourceConfigs, 0)
	for _, section := range cf.SectionStrings() {
		switch section {
		case "env", "DEFAULT":
			continue
		}
		m = append(m, instance.ResourceConfig{
			Rid:          section,
			RestartDelay: cf.GetDuration(key.New(section, "restart_delay")),
			Restart:      cf.GetInt(key.New(section, "restart")),
			IsDisabled:   cf.GetBool(key.New(section, "disable")),
			IsMonitored:  cf.GetBool(key.New(section, "monitor")),
			IsStandby:    cf.GetBool(key.New(section, "standby")),
		})
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
	labels := []pubsub.Label{
		{"node", o.localhost},
		{"path", o.path.String()},
	}
	if o.published {
		instance.ConfigData.Unset(o.path, o.localhost)
		o.bus.Pub(&msgbus.InstanceConfigDeleted{Path: o.path, Node: o.localhost}, labels...)
	}
}

func (o *T) done(parent context.Context, doneChan chan<- any) {
	op := &msgbus.InstanceConfigManagerDone{
		Path: o.path,
		File: o.filename,
	}
	select {
	case <-parent.Done():
		return
	case doneChan <- op:
	}
}
