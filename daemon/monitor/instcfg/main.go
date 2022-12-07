// Package instcfg is responsible for local instance.Config
//
// New instCfg are created by daemon discover.
// It provides the cluster data at ["cluster", "node", localhost, "services",
// "config, <instance>]
// It watches local config file to load updates.
// It watches for local cluster config update to refresh scopes.
//
// The instcfg also starts smon object (with instcfg context)
// => this will end smon object
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
	"opensvc.com/opensvc/daemon/monitor/smon"
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
		dataCmdC     chan<- any
		sub          *pubsub.Subscription
	}
)

var (
	clusterPath = path.T{Name: "cluster", Kind: kind.Ccfg}

	dropMsgTimeout = 100 * time.Millisecond

	configFileCheckError = errors.New("config file check")
)

// Start launch goroutine instCfg worker for a local instance config
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
		dataCmdC:     daemondata.BusFromContext(parent),
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
	clusterId := clusterPath.String()
	bus := pubsub.BusFromContext(ctx)
	label := pubsub.Label{"path", o.path.String()}
	o.sub = bus.Sub(o.path.String() + " instcfg")
	o.sub.AddFilter(msgbus.CfgFileUpdated{}, label)
	o.sub.AddFilter(msgbus.CfgFileRemoved{}, label)
	if o.path.String() != clusterId {
		o.sub.AddFilter(msgbus.CfgUpdated{}, pubsub.Label{"path", clusterId})
	}
	o.sub.Start()
}

func (o *T) startSmon(ctx context.Context) (bool, error) {
	if len(o.cfg.Scope) == 0 {
		o.log.Info().Msgf("wait scopes to create associated smon")
		return false, nil
	}
	o.log.Info().Msgf("starting smon worker...")
	if err := smon.Start(ctx, o.path, o.cfg.Scope); err != nil {
		o.log.Error().Err(err).Msg("failure during start smon worker")
		return false, err
	}
	return true, nil
}

// worker watch for local instCfg config file updates until file is removed
func (o *T) worker(parent context.Context) {
	var (
		hasSmon bool
		err     error
	)
	defer o.log.Debug().Msg("done")
	defer o.log.Debug().Msg("starting")

	// do once what we do later on msgbus.CfgFileUpdated
	if err := o.configFileCheck(); err != nil {
		o.log.Warn().Err(err).Msg("initial configFileCheck")
		return
	}
	defer o.delete()

	smonCtx, cancelSmon := context.WithCancel(parent)
	defer cancelSmon()
	if hasSmon, err = o.startSmon(smonCtx); err != nil {
		o.log.Error().Err(err).Msg("fail to start smon worker")
		return
	}
	o.log.Debug().Msg("started")
	for {
		select {
		case <-parent.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case msgbus.CfgFileUpdated:
				o.log.Debug().Msgf("recv %#v", c)
				if err = o.configFileCheck(); err != nil {
					o.log.Error().Err(err).Msg("configFileCheck error")
					return
				}
				if !hasSmon {
					o.log.Info().Msgf("smon not yet started, try start")
					if hasSmon, err = o.startSmon(smonCtx); err != nil {
						o.log.Error().Err(err).Msgf("smon start error")
						return
					}
				}

			case msgbus.CfgFileRemoved:
				o.log.Debug().Msgf("recv %#v", c)
				return
			case msgbus.CfgUpdated:
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

// updateCfg update iCfg.cfg when newCfg differ from iCfg.cfg
func (o *T) updateCfg(newCfg *instance.Config) {
	if instance.ConfigEqual(&o.cfg, newCfg) {
		o.log.Debug().Msg("no update required")
		return
	}
	o.cfg = *newCfg
	if err := daemondata.SetInstanceConfig(o.dataCmdC, o.path, *newCfg.DeepCopy()); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceConfig")
	}
	o.published = true
}

// configFileCheck verify if config file has been changed
//
//		if config file absent cancel worker
//		if updated time or checksum has changed:
//	       reload load config
//		   updateCfg
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
	if o.path.String() == clusterPath.String() {
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
	o.updateCfg(&cfg)
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
		evalNodes, err = cf.Eval(key.Parse("DEFAULT.nodes"))
		if err != nil {
			o.log.Error().Err(err).Msg("eval DEFAULT.nodes")
			return
		}
		scope = evalNodes.([]string)
	}
	return
}

func (o *T) getPlacementPolicy(cf *xconfig.T) placement.Policy {
	s := cf.GetString(key.Parse("DEFAULT.placement"))
	return placement.NewPolicy(s)
}

func (o *T) getTopology(cf *xconfig.T) topology.T {
	s := cf.GetString(key.Parse("DEFAULT.topology"))
	return topology.New(s)
}

func (o *T) getOrchestrate(cf *xconfig.T) string {
	s := cf.GetString(key.Parse("DEFAULT.orchestrate"))
	return s
}

func (o *T) getPriority(cf *xconfig.T) priority.T {
	s := cf.GetInt(key.Parse("DEFAULT.priority"))
	return priority.T(s)
}

func (o *T) getFlexTarget(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		return cf.GetInt(key.Parse("DEFAULT.flex_target"))
	}
	return 0
}

func (o *T) getFlexMin(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		return cf.GetInt(key.Parse("DEFAULT.flex_min"))
	}
	return 0
}

func (o *T) getFlexMax(cf *xconfig.T) int {
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
		return cf.GetInt(key.Parse("DEFAULT.flex_max"))
	}
	return 0
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
		if err := daemondata.DelInstanceConfig(o.dataCmdC, o.path); err != nil {
			o.log.Error().Err(err).Msg("DelInstanceConfig")
		}
	}
	if err := daemondata.DelInstanceStatus(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceStatus")
	}
}

func (o *T) done(parent context.Context, doneChan chan<- any) {
	op := msgbus.MonCfgDone{
		Path:     o.path,
		Filename: o.filename,
	}
	select {
	case <-parent.Done():
		return
	case doneChan <- op:
	}
}
