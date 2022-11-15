package discover

import (
	"context"
	"os"
	"time"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/monitor/instcfg"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/daemon/remoteconfig"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/pubsub"
)

func (d *discover) startSubscriptions() *pubsub.Subscription {
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("discover.cfg")
	sub.AddFilter(msgbus.CfgUpdated{})
	sub.AddFilter(msgbus.CfgDeleted{})
	sub.AddFilter(msgbus.CfgFileUpdated{})
	sub.Start()
	return sub
}

func (d *discover) cfg(started chan<- bool) {
	d.log.Info().Msg("cfg started")
	defer func() {
		t := time.NewTicker(dropCmdTimeout)
		defer t.Stop()
		for {
			select {
			case <-d.ctx.Done():
				return
			case <-t.C:
				return
			case <-d.cfgCmdC:
			}
		}
	}()
	sub := d.startSubscriptions()
	defer sub.Stop()
	started <- true
	for {
		select {
		case <-d.ctx.Done():
			d.log.Info().Msg("cfg stopped")
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case msgbus.CfgUpdated:
				d.onCfgUpdated(c)
			case msgbus.CfgDeleted:
				d.onCfgDeleted(c)
			case msgbus.CfgFileUpdated:
				d.onCfgFileUpdated(c)
			}
		case i := <-d.cfgCmdC:
			switch c := i.(type) {
			case msgbus.RemoteFileConfig:
				d.onRemoteCfgFetched(c)
			case msgbus.MonCfgDone:
				d.onMonCfgDone(c)
			default:
				d.log.Error().Interface("cmd", i).Msg("unknown cmd")
			}
		}
	}
}

func (d *discover) onCfgFileUpdated(c msgbus.CfgFileUpdated) {
	if c.Path.Kind == kind.Invalid {
		if c.Filename == rawconfig.NodeConfigFile() {
			// node config file change
			d.setNodeLabels()
		}
		return
	}
	s := c.Path.String()
	mtime := file.ModTime(c.Filename)
	if mtime.IsZero() {
		d.log.Info().Msgf("configFile no present(mtime) %s", c.Filename)
		return
	}
	if _, ok := d.cfgMTime[s]; !ok {
		if err := instcfg.Start(d.ctx, c.Path, c.Filename, d.cfgCmdC); err != nil {
			return
		}
	}
	d.cfgMTime[s] = mtime
}

func (d *discover) setNodeLabels() {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		d.log.Error().Err(err).Msg("on node.conf change, error updating labels")
		return
	}
	labels := node.Labels()
	databus := daemondata.BusFromContext(d.ctx)
	daemondata.SetNodeStatusLabels(databus, labels)
}

// cmdLocalCfgDeleted starts a new instcfg when a local configuration file exists
func (d *discover) onMonCfgDone(c msgbus.MonCfgDone) {
	filename := c.Filename
	p := c.Path
	s := p.String()

	delete(d.cfgMTime, s)
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		return
	}
	if err := instcfg.Start(d.ctx, p, filename, d.cfgCmdC); err != nil {
		return
	}
	d.cfgMTime[s] = mtime
}

func (d *discover) onCfgUpdated(c msgbus.CfgUpdated) {
	if c.Node == d.localhost {
		return
	}
	d.onRemoteCfgUpdated(c.Path, c.Node, c.Config)
}

func (d *discover) onRemoteCfgUpdated(p path.T, node string, remoteCfg instance.Config) {
	s := p.String()
	if !d.inScope(&remoteCfg) {
		d.cancelFetcher(s)
		cfgFile := p.ConfigFile()
		if file.Exists(cfgFile) {
			d.log.Info().Msgf("remove local config %s (localnode not in node %s config scope)", s, node)
			if err := os.Remove(cfgFile); err != nil {
				d.log.Debug().Err(err).Msgf("remove %s", cfgFile)
			}
		}
		return
	}
	if mtime, ok := d.cfgMTime[s]; ok {
		if !remoteCfg.Updated.After(mtime) {
			// our version is more recent than remote one
			return
		}
	} else {
		// Not yet started instcfg, but file exists
		localUpdated := file.ModTime(p.ConfigFile())
		if !remoteCfg.Updated.After(localUpdated) {
			return
		}
	}
	if remoteFetcherUpdated, ok := d.fetcherUpdated[s]; ok {
		// fetcher in progress for s, verify if new fetcher is required
		if remoteCfg.Updated.After(remoteFetcherUpdated) {
			d.log.Warn().Msgf("cancel pending remote cfg fetcher, more recent config from %s on %s", s, node)
			d.cancelFetcher(s)
		} else {
			// let running fetcher does its job
			return
		}
	}
	d.log.Info().Msgf("fetch config %s from node %s", s, node)
	d.fetchCfgFromRemote(p, node, remoteCfg.Updated)
}

func (d *discover) onCfgDeleted(c msgbus.CfgDeleted) {
	if c.Node == "" || c.Node == d.localhost {
		return
	}
	s := c.Path.String()
	if fetchFrom, ok := d.fetcherFrom[s]; ok {
		if fetchFrom == c.Node {
			d.log.Info().Msgf("cancel pending remote cfg fetcher %s@%s not anymore present", s, c.Node)
			d.cancelFetcher(s)
		}
	}
}

func (d *discover) onRemoteCfgFetched(c msgbus.RemoteFileConfig) {
	defer d.cancelFetcher(c.Path.String())
	select {
	case <-c.Ctx.Done():
		c.Err <- nil
		return
	default:
		var prefix string
		if c.Path.Namespace != "root" {
			prefix = "namespaces/"
		}
		s := c.Path.String()
		confFile := rawconfig.Paths.Etc + "/" + prefix + s + ".conf"
		d.log.Info().Msgf("install fetched config %s from %s", s, c.Node)
		err := os.Rename(c.Filename, confFile)
		if err != nil {
			d.log.Error().Err(err).Msgf("can't install fetched config to %s", confFile)
		}
		c.Err <- err
	}
	return
}

func (d *discover) inScope(cfg *instance.Config) bool {
	localhost := d.localhost
	for _, node := range cfg.Scope {
		if node == localhost {
			return true
		}
	}
	return false
}

func (d *discover) cancelFetcher(s string) {
	if cancel, ok := d.fetcherCancel[s]; ok {
		d.log.Debug().Msgf("cancelFetcher %s", s)
		cancel()
		node := d.fetcherFrom[s]
		delete(d.fetcherCancel, s)
		delete(d.fetcherNodeCancel[node], s)
		delete(d.fetcherUpdated, s)
		delete(d.fetcherFrom, s)
	}
}

func (d *discover) fetchCfgFromRemote(p path.T, node string, updated time.Time) {
	s := p.String()
	if n, ok := d.fetcherFrom[s]; ok {
		d.log.Error().Msgf("fetcher already in progress for %s from %s", s, n)
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.fetcherCancel[s] = cancel
	d.fetcherFrom[s] = node
	d.fetcherUpdated[s] = updated
	if _, ok := d.fetcherNodeCancel[node]; ok {
		d.fetcherNodeCancel[node][s] = cancel
	} else {
		d.fetcherNodeCancel[node] = make(map[string]context.CancelFunc)
	}

	go remoteconfig.Fetch(ctx, p, node, d.cfgCmdC)
}
