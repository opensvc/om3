package discover

import (
	"context"
	"os"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/icfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/remoteconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (d *discover) startSubscriptions() *pubsub.Subscription {
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("discover.cfg")
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.AddFilter(&msgbus.InstanceConfigDeleted{})
	sub.AddFilter(&msgbus.ConfigFileUpdated{})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.Start()
	return sub
}

func (d *discover) cfg(started chan<- bool) {
	d.log.Info().Msg("cfg started")
	defer func() {
		t := time.NewTicker(d.dropCmdDuration)
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
	defer func() {
		if err := sub.Stop(); err != nil {
			d.log.Error().Err(err).Msg("subscription stop")
		}
	}()
	if last := cluster.ConfigData.Get(); last != nil {
		msg := &msgbus.ClusterConfigUpdated{Value: *last}
		d.onClusterConfigUpdated(msg)
	}
	started <- true
	for {
		select {
		case <-d.ctx.Done():
			d.log.Info().Msg("cfg stopped")
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.InstanceConfigUpdated:
				d.onInstanceConfigUpdated(c)
			case *msgbus.InstanceConfigDeleted:
				d.onInstanceConfigDeleted(c)
			case *msgbus.ConfigFileUpdated:
				d.onConfigFileUpdated(c)
			case *msgbus.ClusterConfigUpdated:
				d.onClusterConfigUpdated(c)
			}
		case i := <-d.cfgCmdC:
			switch c := i.(type) {
			case *msgbus.RemoteFileConfig:
				d.onRemoteConfigFetched(c)
			case *msgbus.InstanceConfigManagerDone:
				d.onMonConfigDone(c)
			default:
				d.log.Error().Interface("cmd", i).Msg("unknown cmd")
			}
		}
	}
}

func (d *discover) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	d.clusterConfig = c.Value
}

func (d *discover) onConfigFileUpdated(c *msgbus.ConfigFileUpdated) {
	if c.Path.Kind == kind.Invalid {
		// may be node.conf
		return
	}
	s := c.Path.String()
	mtime := file.ModTime(c.Filename)
	if mtime.IsZero() {
		d.log.Info().Msgf("configFile no present(mtime) %s", c.Filename)
		return
	}
	if _, ok := d.cfgMTime[s]; !ok {
		if err := icfg.Start(d.ctx, c.Path, c.Filename, d.cfgCmdC); err != nil {
			return
		}
	}
	d.cfgMTime[s] = mtime
}

// cmdLocalConfigDeleted starts a new icfg when a local configuration file exists
func (d *discover) onMonConfigDone(c *msgbus.InstanceConfigManagerDone) {
	filename := c.Filename
	p := c.Path
	s := p.String()

	delete(d.cfgMTime, s)
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		return
	}
	if err := icfg.Start(d.ctx, p, filename, d.cfgCmdC); err != nil {
		return
	}
	d.cfgMTime[s] = mtime
}

func (d *discover) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Node == d.localhost {
		return
	}
	d.onRemoteConfigUpdated(c.Path, c.Node, c.Value)
}

func (d *discover) onRemoteConfigUpdated(p path.T, node string, remoteConfig instance.Config) {
	s := p.String()

	localUpdated := file.ModTime(p.ConfigFile())

	// Never drop local cluster config, ignore remote config older that local
	if !p.Equal(path.Cluster) && remoteConfig.UpdatedAt.After(localUpdated) && !d.inScope(&remoteConfig) {
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
		if !remoteConfig.UpdatedAt.After(mtime) {
			// our version is more recent than remote one
			return
		}
	} else if !remoteConfig.UpdatedAt.After(localUpdated) {
		// Not yet started icfg, but file exists
		return
	}
	if remoteFetcherUpdated, ok := d.fetcherUpdated[s]; ok {
		// fetcher in progress for s, verify if new fetcher is required
		if remoteConfig.UpdatedAt.After(remoteFetcherUpdated) {
			d.log.Warn().Msgf("cancel pending remote cfg fetcher, more recent config from %s on %s", s, node)
			d.cancelFetcher(s)
		} else {
			// let running fetcher does its job
			return
		}
	}
	d.log.Info().Msgf("fetch config %s from node %s", s, node)
	d.fetchConfigFromRemote(p, node, remoteConfig.UpdatedAt)
}

func (d *discover) onInstanceConfigDeleted(c *msgbus.InstanceConfigDeleted) {
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

func (d *discover) onRemoteConfigFetched(c *msgbus.RemoteFileConfig) {
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

func (d *discover) fetchConfigFromRemote(p path.T, node string, updated time.Time) {
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

	cli, err := d.newDaemonClient(node)
	if err != nil {
		d.log.Error().Msgf("can't create newDaemonClient to fetch %s from %s", p, node)
		return
	}
	go fetch(ctx, cli, p, node, d.cfgCmdC)
}

func (d *discover) newDaemonClient(node string) (*client.T, error) {
	// TODO add WithRootCa to avoid send password to wrong url ?
	return client.New(
		client.WithURL(daemonenv.UrlHttpNode(node)),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(d.clusterConfig.Secret()),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
}

func fetch(ctx context.Context, cli *client.T, p path.T, node string, cmdC chan<- any) {
	id := p.String() + "@" + node
	log := daemonlogctx.Logger(ctx).With().Str("_pkg", "cfg.fetch").Str("id", id).Logger()

	tmpFilename, updated, err := remoteconfig.FetchObjectFile(cli, p)
	if err != nil {
		log.Info().Err(err).Msgf("FetchObjectFile %s", id)
		return
	}
	defer func() {
		log.Debug().Msgf("done fetcher routine for %s@%s", p, node)
		_ = os.Remove(tmpFilename)
	}()
	configure, err := object.NewConfigurer(p, object.WithConfigFile(tmpFilename), object.WithVolatile(true))
	if err != nil {
		log.Error().Err(err).Msgf("configure error for %s", p)
		return
	}
	nodes := configure.Config().Referrer.Nodes()
	validScope := false
	for _, n := range nodes {
		if n == hostname.Hostname() {
			validScope = true
			break
		}
	}
	if !validScope {
		log.Info().Msgf("invalid scope %s", nodes)
		return
	}
	select {
	case <-ctx.Done():
		log.Info().Msgf("abort fetch config %s", id)
		return
	default:
		err := make(chan error)
		cmdC <- &msgbus.RemoteFileConfig{
			Path:     p,
			Node:     node,
			Filename: tmpFilename,
			Updated:  updated,
			Ctx:      ctx,
			Err:      err,
		}
		<-err
	}
}
