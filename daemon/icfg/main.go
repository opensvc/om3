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
	"slices"
	"time"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/resourceset"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Manager struct {
		path                     naming.Path
		configure                object.Configurer
		filename                 string
		log                      *plog.Logger
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
	clusterPath = naming.Path{Name: "cluster", Kind: naming.KindCcfg}

	errConfigFileCheck = errors.New("config file check")

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
	keyPool             = key.New("DEFAULT", "pool")
	keyPlacement        = key.New("DEFAULT", "placement")
	keyPreMonitorAction = key.New("DEFAULT", "pre_monitor_action")
	keyPriority         = key.New("DEFAULT", "priority")
	keySize             = key.New("DEFAULT", "size")
	keyTopology         = key.New("DEFAULT", "topology")
)

// Start launch goroutine instConfig worker for a local instance config
func Start(parent context.Context, p naming.Path, filename string, svcDiscoverCmd chan<- any) error {
	localhost := hostname.Hostname()
	ctx, cancel := context.WithCancel(parent)
	t := &Manager{
		instanceConfig: instance.Config{Path: p},
		path:           p,
		localhost:      localhost,
		forceRefresh:   false,
		bus:            pubsub.BusFromContext(ctx),
		filename:       filename,

		ctx:    ctx,
		cancel: cancel,

		log: naming.LogWithPath(plog.NewDefaultLogger(), p).
			Attr("pkg", "daemon/icfg").
			WithPrefix("daemon: icfg: " + p.String() + ": "),
	}

	if err := t.setConfigure(); err != nil {
		return err
	}

	t.startSubscriptions()

	go func() {
		defer t.log.Debugf("stopped")
		defer func() {
			cancel()
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Errorf("subscription stop: %s", err)
			}
			t.done(parent, svcDiscoverCmd)
		}()
		t.worker()
	}()

	return nil
}

func (t *Manager) startSubscriptions() {
	clusterID := clusterPath.String()
	label := pubsub.Label{"path", t.path.String()}
	t.sub = t.bus.Sub("daemon.icfg " + t.path.String())
	t.sub.AddFilter(&msgbus.ConfigFileRemoved{}, label)
	if t.path.String() != clusterID {
		t.sub.AddFilter(&msgbus.ConfigFileUpdated{}, label)

		// the scope value may depend on cluster nodes values: *, clusternodes ...
		// so we must also watch for cluster config updates to configFileCheckRefresh non cluster instance config scope
		localClusterLabels := []pubsub.Label{{"path", clusterID}, {"node", t.localhost}}
		t.sub.AddFilter(&msgbus.InstanceConfigUpdated{}, localClusterLabels...)
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
		t.sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", t.localhost})
	}
	t.sub.Start()
}

// worker watch for local instConfig config file updates until file is removed
func (t *Manager) worker() {
	defer t.log.Debugf("done")
	t.log.Debugf("starting")

	// do once what we do later on msgbus.ConfigFileUpdated
	if err := t.configFileCheck(); err != nil {
		t.log.Warnf("initial configFileCheck: %s", err)
		return
	}
	defer t.delete()

	t.log.Debugf("started")
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch i.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated()
			case *msgbus.ConfigFileRemoved:
				t.onConfigFileRemoved()
			case *msgbus.ConfigFileUpdated:
				t.onConfigFileUpdated()
			case *msgbus.InstanceConfigUpdated:
				t.onLocalClusterInstanceConfigUpdated()
			}
		}
	}
}

func (t *Manager) configFileCheckRefresh(force bool) error {
	if force {
		t.forceRefresh = true
	}
	err := t.configFileCheck()
	if err != nil {
		t.log.Errorf("configFileCheck: %s", err)
		t.cancel()
	}
	return err
}

func (t *Manager) onClusterConfigUpdated() {
	t.log.Infof("cluster config updated => refresh")
	_ = t.configFileCheckRefresh(true)
}

func (t *Manager) onConfigFileUpdated() {
	t.log.Infof("config file changed => refresh")
	_ = t.configFileCheckRefresh(false)
}

func (t *Manager) onLocalClusterInstanceConfigUpdated() {
	t.log.Infof("cluster instance config changed => refresh")
	_ = t.configFileCheckRefresh(true)
}

func (t *Manager) onConfigFileRemoved() {
	t.cancel()
}

// updateConfig update iConfig.cfg when newConfig differ from iConfig.cfg
func (t *Manager) updateConfig(newConfig *instance.Config) {
	if instance.ConfigEqual(&t.instanceConfig, newConfig) {
		t.log.Debugf("no update required")
		return
	}
	if !t.published {
		t.bus.Pub(&msgbus.ObjectCreated{Path: t.path, Node: t.localhost},
			pubsub.Label{"path", t.path.String()},
			pubsub.Label{"node", t.localhost},
		)
	}
	t.instanceConfig = *newConfig
	instance.ConfigData.Set(t.path, t.localhost, newConfig.DeepCopy())
	t.bus.Pub(&msgbus.InstanceConfigUpdated{Path: t.path, Node: t.localhost, Value: *newConfig.DeepCopy()},
		pubsub.Label{"path", t.path.String()},
		pubsub.Label{"node", t.localhost},
	)
	t.published = true
}

// configFileCheck verify if config file has been changed
//
//		if config file absent cancel worker
//		if updated time or checksum has changed:
//	       reload load config
//		   updateConfig
//
//		when localhost is not anymore in scope then ends worker
func (t *Manager) configFileCheck() error {
	mtime := file.ModTime(t.filename)
	if mtime.IsZero() {
		t.log.Infof("configFile no mtime %s", t.filename)
		return errConfigFileCheck
	}
	if mtime.Equal(t.lastMtime) && !t.forceRefresh {
		t.log.Debugf("same mtime, skip")
		return nil
	}
	checksum, err := file.MD5(t.filename)
	if err != nil {
		t.log.Infof("configFile no present(md5sum)")
		return errConfigFileCheck
	}
	if err := t.setConfigure(); err != nil {
		return errConfigFileCheck
	}
	t.forceRefresh = false
	cf := t.configure.Config()
	scope, err := t.getScope(cf)
	if err != nil {
		t.log.Errorf("can't get scope: %s", err)
		return errConfigFileCheck
	}
	if len(scope) == 0 {
		t.log.Infof("empty scope")
		return errConfigFileCheck
	}
	newMtime := file.ModTime(t.filename)
	if newMtime.IsZero() {
		t.log.Infof("configFile no more mtime %s", t.filename)
		return errConfigFileCheck
	}
	if !newMtime.Equal(mtime) {
		t.log.Infof("configFile changed(wait next evaluation)")
		return nil
	}
	if !slices.Contains(scope, t.localhost) {
		t.log.Infof("localhost not anymore an instance node")
		return errConfigFileCheck
	}

	cfg := t.instanceConfig
	cfg.App = cf.GetString(keyApp)
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Children = t.getChildren(cf)
	cfg.Env = cf.GetString(keyEnv)
	cfg.MonitorAction = t.getMonitorAction(cf)
	cfg.Nodename = t.localhost
	cfg.Orchestrate = t.getOrchestrate(cf)
	cfg.Parents = t.getParents(cf)
	cfg.PlacementPolicy = t.getPlacementPolicy(cf)
	cfg.PreMonitorAction = cf.GetString(keyPreMonitorAction)
	cfg.Priority = t.getPriority(cf)
	cfg.Resources = t.getResources(cf)
	cfg.Scope = scope
	cfg.Topology = t.getTopology(cf)
	cfg.UpdatedAt = mtime
	cfg.Size = cf.GetSize(keySize)
	cfg.Subsets = t.getSubsets(cf)

	if pool := cf.GetString(keyPool); pool != "" {
		cfg.Pool = &pool
	}
	if cfg.Topology == topology.Flex {
		cfg.FlexMin = t.getFlexMin(cf)
		cfg.FlexMax = t.getFlexMax(cf)
		cfg.FlexTarget = t.getFlexTarget(cf, cfg.FlexMin, cfg.FlexMax)
	}

	t.lastMtime = mtime
	t.updateConfig(&cfg)
	return nil
}

// getScope return sorted scopes for object
//
// depending on object kind
// Ccfg => cluster.nodes
// else => eval DEFAULT.nodes
func (t *Manager) getScope(cf *xconfig.T) (scope []string, err error) {
	switch t.path.Kind {
	case naming.KindCcfg:
		scope = clusternode.Get()
	default:
		var evalNodes interface{}
		evalNodes, err = cf.Eval(keyNodes)
		if err != nil {
			t.log.Errorf("eval DEFAULT.nodes: %s", err)
			return
		}
		scope = evalNodes.([]string)
	}
	return
}

func (t *Manager) getMonitorAction(cf *xconfig.T) instance.MonitorAction {
	s := cf.GetString(keyMonitorAction)
	return instance.MonitorAction(s)
}

func (t *Manager) getChildren(cf *xconfig.T) naming.Relations {
	l := cf.GetStrings(keyChildren)
	return naming.ParseRelations(l)
}

func (t *Manager) getParents(cf *xconfig.T) naming.Relations {
	l := cf.GetStrings(keyParents)
	return naming.ParseRelations(l)
}

func (t *Manager) getPlacementPolicy(cf *xconfig.T) placement.Policy {
	s := cf.GetString(keyPlacement)
	return placement.NewPolicy(s)
}

func (t *Manager) getTopology(cf *xconfig.T) topology.T {
	s := cf.GetString(keyTopology)
	return topology.New(s)
}

func (t *Manager) getOrchestrate(cf *xconfig.T) string {
	s := cf.GetString(keyOrchestrate)
	return s
}

func (t *Manager) getSubsets(cf *xconfig.T) map[string]instance.SubsetConfig {
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

func (t *Manager) getResources(cf *xconfig.T) instance.ResourceConfigs {
	m := make(instance.ResourceConfigs, 0)
	for _, section := range cf.SectionStrings() {
		switch section {
		case "env", "DEFAULT":
			continue
		}
		if resourceset.IsSubsetSection(section) {
			continue
		}
		m[section] = instance.ResourceConfig{
			RestartDelay: cf.GetDuration(key.New(section, "restart_delay")),
			Restart:      cf.GetInt(key.New(section, "restart")),
			IsDisabled:   cf.GetBool(key.New(section, "disable")),
			IsMonitored:  cf.GetBool(key.New(section, "monitor")),
			IsStandby:    cf.GetBool(key.New(section, "standby")),
		}
	}
	return m
}

func (t *Manager) getPriority(cf *xconfig.T) priority.T {
	s := cf.GetInt(keyPriority)
	return priority.T(s)
}

func (t *Manager) getFlexTarget(cf *xconfig.T, min, max int) (target int) {
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
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

func (t *Manager) getFlexMin(cf *xconfig.T) int {
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
		return cf.GetInt(keyFlexMin)
	}
	return 0
}

func (t *Manager) getFlexMax(cf *xconfig.T) int {
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
		if i, err := cf.GetIntStrict(keyFlexMax); err == nil {
			return i
		} else if scope, err := t.getScope(cf); err == nil {
			return len(scope)
		} else {
			return 0
		}
	default:
		return 0
	}
}

func (t *Manager) setConfigure() error {
	configure, err := object.NewConfigurer(t.path)
	if err != nil {
		t.log.Warnf("configure failed: %s", err)
		return err
	}
	t.configure = configure
	return nil
}

func (t *Manager) delete() {
	labels := []pubsub.Label{
		{"node", t.localhost},
		{"path", t.path.String()},
	}
	if t.published {
		instance.ConfigData.Unset(t.path, t.localhost)
		t.bus.Pub(&msgbus.InstanceConfigDeleted{Path: t.path, Node: t.localhost}, labels...)
	}
}

func (t *Manager) done(parent context.Context, doneChan chan<- any) {
	op := &msgbus.InstanceConfigManagerDone{
		Path: t.path,
		File: t.filename,
	}
	select {
	case <-parent.Done():
		return
	case doneChan <- op:
	}
}
