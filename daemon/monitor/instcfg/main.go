//	Package instcfg is responsible for local instance.Config
//
//	It provides the cluster data ["monitor", "nodes", localhost, "services", "config, <instance>]
//	instance config are first created by daemon discover, then it watch local
//	config file to detect updates.
//
//	worker routine is terminated when config file is not any more present, or
//  when context is done.
//
//	worker also watch on cluster config updates to refresh its config because
//	config scopes needs refresh when cluster nodes are updated.
//
package instcfg

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	ps "opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/monitor/mondata"
	"opensvc.com/opensvc/daemon/monitor/smon"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	instCfg struct {
		cfg instance.Config

		path         path.T
		id           string
		configure    object.Configurer
		filename     string
		ctx          context.Context
		cancel       context.CancelFunc
		log          zerolog.Logger
		lastMtime    time.Time
		localhost    string
		forceRefresh bool

		cmdC         chan *moncmd.T
		dataCmdC     chan<- interface{}
		discoverCmdC chan<- *moncmd.T
	}
)

var (
	clusterPath = path.T{Name: "cluster", Kind: kind.Ccfg}

	dropCmdTimeout = 100 * time.Millisecond
)

// Start launch goroutine instCfg worker for a local instance config
func Start(parent context.Context, p path.T, filename string, svcDiscoverCmd chan<- *moncmd.T) {
	localhost := hostname.Hostname()
	id := mondata.InstanceId(p, localhost)

	o := &instCfg{
		cfg:          instance.Config{Path: p},
		path:         p,
		id:           id,
		log:          daemonctx.Logger(parent).With().Str("_instanceCfg", p.String()).Logger(),
		localhost:    localhost,
		forceRefresh: false,
		cmdC:         make(chan *moncmd.T),
		dataCmdC:     daemonctx.DaemonDataCmd(parent),
		discoverCmdC: svcDiscoverCmd,
		filename:     filename,
	}
	go o.worker(parent)
	return
}

// worker watch for local instCfg config file updates until file is removed
func (o *instCfg) worker(parent context.Context) {
	defer o.log.Info().Msg("done")
	o.ctx, o.cancel = context.WithCancel(parent)
	defer o.cancel()
	defer moncmd.DropPendingCmd(o.cmdC, dropCmdTimeout)
	defer o.done()
	clusterId := clusterPath.String()
	if o.path.String() != clusterId {
		c := daemonctx.DaemonPubSubCmd(o.ctx)
		defer ps.UnSub(c, ps.SubCfg(c, pubsub.OpUpdate, "instance-config cluster cfg update", clusterId, o.onEv))
	}

	if err := o.watchFile(); err != nil {
		o.log.Error().Err(err).Msg("watch file")
		return
	}
	if err := o.setConfigure(); err != nil {
		o.log.Error().Err(err).Msg("setConfigure")
		return
	}
	o.configFileCheck()
	defer o.delete()
	select {
	case <-o.ctx.Done():
		return
	default:
	}
	if err := smon.Start(o.ctx, o.path, o.cfg.Scope); err != nil {
		o.log.Error().Err(err).Msg("fail to start smon worker")
		return
	}
	o.log.Info().Msg("started")
	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.cmdC:
			switch c := (*i).(type) {
			case moncmd.CfgUpdated:
				if c.Node != o.localhost {
					// only watch local cluster cfg
					continue
				}
				o.log.Info().Msg("cluster changed => refresh cfg")
				o.forceRefresh = true
				o.configFileCheck()
			case moncmd.CfgFileUpdated:
				o.log.Info().Msg("config file updated")
				o.configFileCheck()
			case moncmd.CfgFileRemoved:
				return
			default:
				o.log.Error().Interface("cmd", i).Msg("unexpected cmd")
			}
		}
	}
}

func (o *instCfg) onEv(i interface{}) {
	o.cmdC <- moncmd.New(i)
}

// updateCfg update iCfg.cfg when newCfg differ from iCfg.cfg
func (o *instCfg) updateCfg(newCfg *instance.Config) {
	if o.cfg.Updated == newCfg.Updated {
		o.log.Info().Msg("same updated, skip")
		return
	}
	if err := daemondata.SetInstanceConfig(o.dataCmdC, o.path, *newCfg.DeepCopy()); err != nil {
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
func (o *instCfg) configFileCheck() {
	mtime := file.ModTime(o.filename)
	if mtime.IsZero() {
		o.log.Info().Msgf("configFile no present(mtime) %s", o.filename)
		o.cancel()
		return
	}
	if mtime.Equal(o.lastMtime) && !o.forceRefresh {
		o.log.Info().Msg("same mtime, skip")
		return
	}
	checksum, err := file.MD5(o.filename)
	if err != nil {
		o.log.Info().Msgf("configFile no present(md5sum)")
		o.cancel()
		return
	}
	if err := o.setConfigure(); err != nil {
		o.log.Error().Err(err).Msg("setConfigure")
		return
	}
	o.forceRefresh = false
	nodes := o.configure.Config().Referrer.Nodes()
	if len(nodes) == 0 {
		o.log.Info().Msg("configFile empty nodes")
		o.cancel()
		return
	}
	newMtime := file.ModTime(o.filename)
	if newMtime.IsZero() {
		o.log.Info().Msg("configFile no present(mtime)")
		o.cancel()
		return
	}
	if !newMtime.Equal(mtime) {
		o.log.Info().Msg("configFile changed(wait next evaluation)")
		return
	}
	hasLocalNode := false
	for _, node := range nodes {
		if node == o.localhost {
			hasLocalNode = true
			break
		}
	}
	if !hasLocalNode {
		o.log.Info().Msg("localhost not anymore an instance node")
		o.cancel()
		return
	}
	cfg := o.cfg
	cfg.Nodename = o.localhost
	cfg.Scope = nodes
	cfg.Checksum = fmt.Sprintf("%x", checksum)
	cfg.Updated = timestamp.New(mtime)
	o.lastMtime = mtime
	o.updateCfg(&cfg)
}

func (o *instCfg) setConfigure() error {
	configure, err := object.NewConfigurerFromPath(o.path)
	if err != nil {
		o.log.Debug().Err(err).Msg("worker NewConfigurerFromPath failure")
		o.cancel()
		return err
	}
	o.configure = configure
	return nil
}

func (o *instCfg) delete() {
	if err := daemondata.DelInstanceConfig(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceConfig")
	}
	if err := daemondata.DelInstanceStatus(o.dataCmdC, o.path); err != nil {
		o.log.Error().Err(err).Msg("DelInstanceStatus")
	}
}

func (o *instCfg) done() {
	o.discoverCmdC <- moncmd.New(moncmd.MonCfgDone{
		Path:     o.path,
		Filename: o.filename,
	})
}
