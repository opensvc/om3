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
	"strings"
	"time"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/resourceset"
	"github.com/opensvc/om3/core/schedule"
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
		path         naming.Path
		configure    object.Configurer
		filename     string
		log          *plog.Logger
		lastMtime    time.Time
		localhost    string
		forceRefresh bool

		// pubLabel is the list of labels for this icfg publications (path and node)
		pubLabel  []pubsub.Label
		published bool
		publisher pubsub.Publisher
		sub       *pubsub.Subscription

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

	// standbyDefaultRestart defines the default minimum restart threshold for
	// standby resources.
	standbyDefaultRestart = 2

	keyApp              = key.New("DEFAULT", "app")
	keyChildren         = key.New("DEFAULT", "children")
	keyDisable          = key.New("DEFAULT", "disable")
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
		publisher:      pubsub.PubFromContext(ctx),
		filename:       filename,

		ctx:    ctx,
		cancel: cancel,

		log: naming.LogWithPath(plog.NewDefaultLogger(), p).
			Attr("pkg", "daemon/icfg").
			WithPrefix("daemon: icfg: " + p.String() + ": "),

		pubLabel: []pubsub.Label{
			{"namespace", p.Namespace},
			{"path", p.String()},
			{"node", localhost},
		},
	}

	if err := t.setConfigure(); err != nil {
		return err
	}

	t.startSubscriptions()

	go func() {
		t.log.Debugf("starting")
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
	clusterPathString := clusterPath.String()

	labelPath := pubsub.Label{"path", t.path.String()}
	labelPathCluster := pubsub.Label{"path", clusterPathString}
	labelLocalhost := pubsub.Label{"node", t.localhost}

	t.sub = pubsub.SubFromContext(t.ctx, "daemon.icfg "+t.path.String())
	t.sub.AddFilter(&msgbus.ConfigFileRemoved{}, labelPath)
	if t.path.String() != clusterPathString {
		t.sub.AddFilter(&msgbus.ConfigFileUpdated{}, labelPath)

		// the scope value may depend on cluster nodes values: *, clusternodes ...
		// so we must also watch for cluster config updates to configFileCheckRefresh non cluster instance config scope
		t.sub.AddFilter(&msgbus.InstanceConfigUpdated{}, labelPathCluster, labelLocalhost)
		t.sub.AddFilter(&msgbus.NodeStatusLabelsCommited{})
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
		t.sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)
	}
	t.sub.Start()
}

// worker watch for local instConfig config file updates until file is removed
func (t *Manager) worker() {
	// do once what we do later on msgbus.ConfigFileUpdated
	if err := t.configFileCheck(); err != nil {
		t.log.Debugf("initial: %s", err)
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
			case *msgbus.NodeStatusLabelsCommited:
				t.onNodeStatusLabelsCommited()
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
		t.log.Debugf("refresh: %s", err)
		t.cancel()
	}
	return err
}

func (t *Manager) onClusterConfigUpdated() {
	t.log.Infof("cluster config updated => refresh")
	_ = t.configFileCheckRefresh(true)
}

func (t *Manager) onConfigFileUpdated() {
	t.log.Infof("config file updated => refresh if csum changed")
	_ = t.configFileCheckRefresh(false)
}

func (t *Manager) onLocalClusterInstanceConfigUpdated() {
	t.log.Infof("cluster instance config changed => refresh")
	_ = t.configFileCheckRefresh(true)
}

func (t *Manager) onNodeStatusLabelsCommited() {
	t.log.Infof("node labels changed => refresh")
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
		t.publisher.Pub(&msgbus.ObjectCreated{Path: t.path, Node: t.localhost}, t.pubLabel...)
	}
	t.instanceConfig = *newConfig
	instance.ConfigData.Set(t.path, t.localhost, newConfig.DeepCopy())
	t.publisher.Pub(&msgbus.InstanceConfigUpdated{Path: t.path, Node: t.localhost, Value: *newConfig.DeepCopy()}, t.pubLabel...)
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
		t.log.Infof("the config file mtime is zero")
		return errConfigFileCheck
	}
	if mtime.Equal(t.lastMtime) && !t.forceRefresh {
		t.log.Debugf("the config file mtime has not changed, keep the cached icfg")
		return nil
	}
	checksum, err := file.MD5(t.filename)
	if err != nil {
		t.log.Infof("the config file md5sum failed: %s", err)
		return errConfigFileCheck
	}
	if err := t.setConfigure(); err != nil {
		return errConfigFileCheck
	}
	t.forceRefresh = false
	cf := t.configure.Config()
	scope, err := t.getScope(cf)
	if err != nil {
		t.log.Errorf("the config file scope evaluation failed: %s", err)
		return errConfigFileCheck
	}
	if len(scope) == 0 {
		t.log.Infof("the config file scope is empty")
		return errConfigFileCheck
	}
	newMtime := file.ModTime(t.filename)
	if newMtime.IsZero() {
		t.log.Infof("the config file mtime has changed to zero")
		return errConfigFileCheck
	}
	if !newMtime.Equal(mtime) {
		t.log.Infof("the config file mtime changed, rescan after the storm")
		return nil
	}
	if !slices.Contains(scope, t.localhost) {
		t.log.Infof("the local node is not in the config file scope (%s)", strings.Join(scope, ","))
		cfg := t.instanceConfig
		cfg.Scope = scope
		cfg.UpdatedAt = mtime
		msg := msgbus.InstanceConfigFor{
			Path:      t.path,
			Node:      t.localhost,
			Scope:     append([]string{}, cfg.Scope...),
			UpdatedAt: cfg.UpdatedAt,
		}
		if cfg.ActorConfig != nil {
			msg.Orchestrate = cfg.Orchestrate
		}
		t.publisher.Pub(&msg, t.pubLabel...)
		if _, ok := any(t.configure).(object.Actor); ok {
			cfg.ActorConfig = &instance.ActorConfig{
				Orchestrate: t.getOrchestrate(cf),
			}
		}
		return errConfigFileCheck
	}

	cfg := t.instanceConfig
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Scope = scope
	cfg.UpdatedAt = mtime
	cfg.Priority = t.getPriority(cf)
	if t.path.Kind == naming.KindVol {
		cfg.VolConfig = &instance.VolConfig{
			Pool: cf.GetString(keyPool),
		}
		if szPtr := cf.GetSize(keySize); szPtr != nil {
			cfg.Size = *szPtr
		}
	}
	if actor, ok := any(t.configure).(object.Actor); ok {
		cfg.ActorConfig = &instance.ActorConfig{
			App:              cf.GetString(keyApp),
			Children:         t.getChildren(cf),
			Env:              cf.GetString(keyEnv),
			IsDisabled:       cf.GetBool(keyDisable),
			MonitorAction:    t.getMonitorAction(cf),
			Orchestrate:      t.getOrchestrate(cf),
			Parents:          t.getParents(cf),
			PreMonitorAction: cf.GetString(keyPreMonitorAction),
			PlacementPolicy:  t.getPlacementPolicy(cf),
			Resources:        t.getResources(cf),
			Schedules:        make([]schedule.Config, 0),
			Subsets:          t.getSubsets(cf),
			Topology:         t.getTopology(cf),
		}
		if cfg.Topology == topology.Flex {
			instanceCount := len(scope)
			flexMin := t.getFlexMin(cf, instanceCount)
			flexMax := t.getFlexMax(cf, flexMin, instanceCount)
			flexTarget := t.getFlexTarget(cf, flexMin, flexMax)
			cfg.FlexConfig = &instance.FlexConfig{
				FlexMin:    flexMin,
				FlexMax:    flexMax,
				FlexTarget: flexTarget,
			}
		}
		for _, e := range actor.Schedules() {
			cfg.ActorConfig.Schedules = append(cfg.ActorConfig.Schedules, e.Config)
		}
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

func (t *Manager) getMonitorAction(cf *xconfig.T) []instance.MonitorAction {
	l := make([]instance.MonitorAction, 0)
	for _, s := range cf.GetStrings(keyMonitorAction) {
		l = append(l, instance.MonitorAction(s))
	}
	return l
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
		restart := cf.GetInt(key.New(section, "restart"))
		isStandby := cf.GetBool(key.New(section, "standby"))
		if isStandby && restart < standbyDefaultRestart {
			restart = standbyDefaultRestart
		}
		m[section] = instance.ResourceConfig{
			RestartDelay: cf.GetDuration(key.New(section, "restart_delay")),
			Restart:      restart,
			IsDisabled:   cf.GetBool(key.New(section, "disable")),
			IsMonitored:  cf.GetBool(key.New(section, "monitor")),
			IsStandby:    isStandby,
		}
	}
	return m
}

func (t *Manager) getPriority(cf *xconfig.T) priority.T {
	s := cf.GetInt(keyPriority)
	return priority.T(s)
}

func (t *Manager) getFlexMin(cf *xconfig.T, maxInstanceCount int) int {
	var minInstanceCount int

	switch t.path.Kind {
	case naming.KindSvc:
		minInstanceCount = 1
	case naming.KindVol:
		minInstanceCount = 0
	default:
		return 0
	}

	i, err := cf.GetIntStrict(keyFlexMin)
	if err != nil {
		t.log.Warnf("get flex_min value: %s", err)
		return minInstanceCount
	}
	if i < minInstanceCount {
		t.log.Warnf("increase flex_min value %d to %d", i, minInstanceCount)
		return minInstanceCount
	}
	if i > maxInstanceCount {
		t.log.Warnf("decrease flex_min value %d to the instance count %d", i, maxInstanceCount)
		return maxInstanceCount
	}
	return i
}

func (t *Manager) getFlexMax(cf *xconfig.T, minInstanceCount, maxInstanceCount int) int {
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
		i, err := cf.GetIntStrict(keyFlexMax)
		if err != nil {
			t.log.Warnf("get flex_max value: %s", err)
			return maxInstanceCount
		}
		if i < minInstanceCount {
			t.log.Warnf("increase flex_max value %d to %d", i, minInstanceCount)
			return minInstanceCount
		}
		if i > maxInstanceCount {
			t.log.Warnf("decrease flex_max value %d to %d", i, maxInstanceCount)
			return maxInstanceCount
		}
		return i
	default:
		return 0
	}
}

func (t *Manager) getFlexTarget(cf *xconfig.T, minInstanceCount, maxInstanceCount int) (target int) {
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
		i, err := cf.GetIntStrict(keyFlexTarget)
		if err != nil {
			t.log.Debugf("can't get flex_target value: %s", err)
			return minInstanceCount
		}
		if i < minInstanceCount {
			t.log.Warnf("increase flex_target value %d to %d", i, minInstanceCount)
			return minInstanceCount
		}
		if i > maxInstanceCount {
			t.log.Warnf("decrease flex_target value %d to %d", i, maxInstanceCount)
			return maxInstanceCount
		}
		return i
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
	if t.published {
		instance.ConfigData.Unset(t.path, t.localhost)
		t.publisher.Pub(&msgbus.InstanceConfigDeleted{Path: t.path, Node: t.localhost}, t.pubLabel...)
	}
}

func (t *Manager) done(parent context.Context, doneChan chan<- any) {
	t.publisher.Pub(&msgbus.InstanceConfigManagerDone{Path: t.path, File: t.filename}, t.pubLabel...)
}
