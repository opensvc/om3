// Package instcfg is responsible for local instance.Config
//
// New instCfg are created by daemon discover.
// It provides the cluster data at ["monitor", "nodes", localhost, "services",
// "config, <instance>]
// It watches local config file to load updates.
// It watches more recent remote config to refresh local config file.
// It watches for local cluster config update to refresh scopes.
//
// The worker routine is terminated when config file is not any more present, or
// when context is done.
//
// The worker also listen for cluster config updates to refresh its config to
// reflect scopes changes.
//
package instcfg

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/monitor/smon"
	"opensvc.com/opensvc/daemon/remoteconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/stringslice"
)

type (
	T struct {
		cfg    instance.Config
		ctx    context.Context
		Cancel context.CancelFunc

		path         path.T
		id           string
		configure    object.Configurer
		filename     string
		log          zerolog.Logger
		lastMtime    time.Time
		localhost    string
		forceRefresh bool

		fetchCtx     context.Context
		fetchUpdated time.Time
		fetchCancel  context.CancelFunc

		CmdC         chan *moncmd.T
		dataCmdC     chan<- interface{}
		discoverCmdC chan<- *moncmd.T
	}
)

var (
	clusterPath = path.T{Name: "cluster", Kind: kind.Ccfg}

	dropCmdTimeout = 100 * time.Millisecond

	delayInitialConfigure = 100 * time.Millisecond
)

// Start launch goroutine instCfg worker for a local instance config
func Start(ctx context.Context, p path.T, filename string, svcDiscoverCmd chan<- *moncmd.T) *T {
	localhost := hostname.Hostname()
	id := daemondata.InstanceId(p, localhost)

	o := &T{
		cfg:          instance.Config{Path: p},
		path:         p,
		id:           id,
		log:          log.Logger.With().Str("func", "instcfg").Stringer("object", p).Logger(),
		localhost:    localhost,
		forceRefresh: false,
		CmdC:         make(chan *moncmd.T),
		dataCmdC:     daemondata.BusFromContext(ctx),
		discoverCmdC: svcDiscoverCmd,
		filename:     filename,
	}
	go func() {
		defer o.log.Debug().Msg("stopped")
		o.worker(ctx)
	}()
	return o
}

// worker watch for local instCfg config file updates until file is removed
func (o *T) worker(ctx context.Context) {
	defer o.log.Debug().Msg("done")
	o.ctx, o.Cancel = context.WithCancel(ctx)
	defer o.Cancel()
	defer moncmd.DropPendingCmd(o.CmdC, dropCmdTimeout)
	defer o.done()
	clusterId := clusterPath.String()
	bus := pubsub.BusFromContext(ctx)
	defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpUpdate, "instance-config self cfg update", o.path.String(), o.onEv))
	if o.path.String() != clusterId {
		defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpUpdate, "instance-config cluster cfg update", clusterId, o.onEv))
	}

	// delay initial configure, seen on storm file creation
	time.Sleep(delayInitialConfigure)
	if err := o.setConfigure(); err != nil {
		o.log.Error().Err(err).Msg("setConfigure")
		return
	}
	o.configFileCheck()
	defer o.delete()
	if err := smon.Start(o.ctx, o.path, o.cfg.Scope); err != nil {
		o.log.Error().Err(err).Msg("fail to start smon worker")
		return
	}
	o.log.Debug().Msg("started")
	for {
		if o.fetchCtx != nil {
			select {
			case <-o.fetchCtx.Done():
				o.fetchCancel()
				o.fetchCtx = nil
			default:
			}
		}
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.CmdC:
			switch c := (*i).(type) {
			case moncmd.Exit:
				log.Debug().Msg("eat poison pill")
				return
			case moncmd.RemoteFileConfig:
				o.log.Debug().Msgf("recv %#v", c)
				o.cmdRemoteCfgFetched(c)
			case moncmd.CfgUpdated:
				o.log.Debug().Msgf("recv %#v", c)
				o.cmdCfgUpdated(c)
			case moncmd.CfgFileUpdated:
				o.log.Debug().Msgf("recv %#v", c)
				o.configFileCheck()
			case moncmd.CfgFileRemoved:
				o.log.Debug().Msgf("recv %#v", c)
				return
			default:
				o.log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}

func (o *T) cmdRemoteCfgFetched(c moncmd.RemoteFileConfig) {
	select {
	case <-c.Ctx.Done():
		o.fetchCtx = nil
		c.Err <- nil
		return
	default:
		defer o.fetchCancel()
		var prefix string
		if c.Path.Namespace != "root" {
			prefix = "namespaces/"
		}
		s := c.Path.String()
		confFile := rawconfig.Paths.Etc + "/" + prefix + s + ".conf"
		o.log.Info().Msgf("install fetched config %s from %s", s, c.Node)
		err := os.Rename(c.Filename, confFile)
		if err != nil {
			o.log.Error().Err(err).Msgf("can't install fetched config to %s", confFile)
		}
		o.fetchCtx = nil
		c.Err <- err
	}
	return
}

func (o *T) cmdCfgUpdated(c moncmd.CfgUpdated) {
	var clusterUpdate bool
	if c.Path.String() == clusterPath.String() {
		clusterUpdate = true
	}
	if c.Node != o.localhost {
		if clusterUpdate {
			o.cmdCfgUpdatedRemote(c)
		} else if o.path.Kind != kind.Sec && !stringslice.Has(o.localhost, c.Config.Scope) {
			o.log.Error().Msgf("not in scope: %s", c.Config.Scope)
			return
		} else {
			o.cmdCfgUpdatedRemote(c)
		}
	} else if clusterUpdate && o.path.String() != clusterPath.String() {
		o.log.Info().Msg("local cluster config changed => refresh cfg")
		o.forceRefresh = true
		o.configFileCheck()
	}
}

// cmdCfgUpdatedRemote retrieve config file from remote node
//
// it returns without any actions when current updated is newer, or if a fetcher
// from updated is already running.
//
// pending obsolete fetcher is canceled.
//
func (o *T) cmdCfgUpdatedRemote(c moncmd.CfgUpdated) {
	remoteCfgUpdated := c.Config.Updated
	if o.cfg.Updated.Unix() >= remoteCfgUpdated.Unix() {
		return
	}
	// need fetch
	if o.fetchCtx != nil {
		// fetcher is running
		if o.fetchUpdated.Unix() >= remoteCfgUpdated.Unix() {
			return
		} else {
			o.log.Info().Msgf("cancel current fetcher a more recent config file exists on %s", c.Node)
			o.fetchCancel()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	o.fetchCtx = ctx
	o.fetchCancel = cancel
	o.fetchUpdated = c.Config.Updated
	o.log.Info().Msgf("fetching more recent config from node %s", c.Node)
	go remoteconfig.Fetch(daemonlogctx.WithLogger(ctx, o.log), o.path, c.Node, o.CmdC)
}

func (o *T) onEv(i interface{}) {
	select {
	case o.CmdC <- moncmd.New(i):
	}
}

// updateCfg update iCfg.cfg when newCfg differ from iCfg.cfg
func (o *T) updateCfg(newCfg *instance.Config) {
	if instance.ConfigEqual(&o.cfg, newCfg) {
		o.log.Debug().Msg("no update required")
		return
	}
	o.cfg = *newCfg
	if err := daemondata.SetInstanceConfig(o.ctx, o.dataCmdC, o.path, *newCfg.DeepCopy()); err != nil {
		o.log.Error().Err(err).Msg("SetInstanceConfig")
	}
}

// configFileCheck verify if config file has been changed
//
//		if config file absent cancel worker
//		if updated time or checksum has changed:
//	       reload load config
// 		   updateCfg
//
//		when localhost is not anymore in scope then ends worker
func (o *T) configFileCheck() {
	mtime := file.ModTime(o.filename)
	if mtime.IsZero() {
		o.log.Info().Msgf("configFile no present(mtime) %s", o.filename)
		o.Cancel()
		return
	}
	if mtime.Equal(o.lastMtime) && !o.forceRefresh {
		o.log.Debug().Msg("same mtime, skip")
		return
	}
	checksum, err := file.MD5(o.filename)
	if err != nil {
		o.log.Info().Msgf("configFile no present(md5sum)")
		o.Cancel()
		return
	}
	if o.path.String() == clusterPath.String() {
		rawconfig.LoadSections()
	}
	if err := o.setConfigure(); err != nil {
		o.log.Error().Err(err).Msg("setConfigure")
		return
	}
	o.forceRefresh = false
	nodes := o.configure.Config().Referrer.Nodes()
	if len(nodes) == 0 {
		o.log.Info().Msg("configFile empty nodes")
		o.Cancel()
		return
	}
	newMtime := file.ModTime(o.filename)
	if newMtime.IsZero() {
		o.log.Info().Msg("configFile no present(mtime)")
		o.Cancel()
		return
	}
	if !newMtime.Equal(mtime) {
		o.log.Info().Msg("configFile changed(wait next evaluation)")
		return
	}
	if !stringslice.Has(o.localhost, nodes) {
		o.log.Info().Msg("localhost not anymore an instance node")
		o.Cancel()
		return
	}
	cfg := o.cfg
	cfg.Nodename = o.localhost
	sort.Strings(nodes)
	cfg.Scope = nodes
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Updated = mtime
	o.lastMtime = mtime
	o.updateCfg(&cfg)
}

func (o *T) setConfigure() error {
	configure, err := object.NewConfigurer(o.path)
	if err != nil {
		o.log.Warn().Err(err).Msg("worker NewConfigurerFromPath failure")
		o.Cancel()
		return err
	}
	o.configure = configure
	return nil
}

func (o *T) delete() {
	if err := daemondata.DelInstanceConfig(o.ctx, o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceConfig")
	}
	if err := daemondata.DelInstanceStatus(o.ctx, o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceStatus")
	}
}

func (o *T) done() {
	op := moncmd.New(moncmd.MonCfgDone{
		Path:     o.path,
		Filename: o.filename,
	})
	select {
	case <-o.ctx.Done():
		return
	case o.discoverCmdC <- op:
	}
}
