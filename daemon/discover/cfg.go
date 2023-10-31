package discover

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/icfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/remoteconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

var (
	// SubscriptionQueueSizeCfg is size of "discover.cfg" subscription
	SubscriptionQueueSizeCfg = 30000
)

func (d *discover) startSubscriptions() *pubsub.Subscription {
	bus := pubsub.BusFromContext(d.ctx)
	sub := bus.Sub("discover.cfg", pubsub.WithQueueSize(SubscriptionQueueSizeCfg))
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})
	sub.AddFilter(&msgbus.InstanceConfigDeleted{})
	sub.AddFilter(&msgbus.ConfigFileUpdated{})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDeleted{})
	sub.Start()
	return sub
}

func (d *discover) cfg(started chan<- bool) {
	d.log.Infof("cfg: started")
	defer d.log.Infof("cfg: stopped")
	defer func() {
		d.log.Debugf("cfg: flushing the command bus message queue")
		defer d.log.Debugf("cfg: flushed the command bus message queue")
		t := time.NewTicker(d.drainDuration)
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
			d.log.Errorf("cfg: subscription stop: %s", err)
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
			case *msgbus.ObjectStatusUpdated:
				d.onObjectStatusUpdated(c)
			case *msgbus.ObjectStatusDeleted:
				d.onObjectStatusDeleted(c)
			}
		case i := <-d.cfgCmdC:
			switch c := i.(type) {
			case *msgbus.RemoteFileConfig:
				d.onRemoteConfigFetched(c)
			case *msgbus.InstanceConfigManagerDone:
				d.onMonConfigDone(c)
			default:
				d.log.Errorf("cfg: unsupported command bus message type: %#v", i)
			}
		case nfo := <-d.objectList.InfoC:
			d.log.Infof("cfg: object list: " + nfo)
		case err := <-d.objectList.ErrC:
			d.log.Infof("cfg: object list error: %s", err)
		case nfo := <-d.nodeList.InfoC:
			d.log.Infof("cfg: node list: " + nfo)
		case err := <-d.nodeList.ErrC:
			d.log.Infof("cfg: node list: error: %s", err)
		}
	}
}

func (d *discover) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	d.clusterConfig = c.Value
	d.nodeList.Add(c.NodesAdded...)
	d.nodeList.Del(c.NodesRemoved...)
}

func (d *discover) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	d.objectList.Add(c.Path.String())
}

func (d *discover) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	d.objectList.Del(c.Path.String())
}

func (d *discover) onConfigFileUpdated(c *msgbus.ConfigFileUpdated) {
	if c.Path.Kind == naming.KindInvalid {
		// may be node.conf
		return
	}
	s := c.Path.String()
	mtime := file.ModTime(c.File)
	if mtime.IsZero() {
		d.log.Infof("cfg: config file %s mtime is zero", c.File)
		return
	}
	if _, ok := d.cfgMTime[s]; !ok {
		if err := icfg.Start(d.ctx, c.Path, c.File, d.cfgCmdC); err != nil {
			return
		}
	}
	d.cfgMTime[s] = mtime
}

// cmdLocalConfigDeleted starts a new icfg when a local configuration file exists
func (d *discover) onMonConfigDone(c *msgbus.InstanceConfigManagerDone) {
	filename := c.File
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

func (d *discover) onRemoteConfigUpdated(p naming.Path, node string, remoteInstanceConfig instance.Config) {
	s := p.String()

	localUpdated := file.ModTime(p.ConfigFile())

	// Never drop local cluster config, ignore remote config older that local
	if !p.Equal(naming.Cluster) && remoteInstanceConfig.UpdatedAt.After(localUpdated) && !d.inScope(&remoteInstanceConfig) {
		d.cancelFetcher(s)
		cfgFile := p.ConfigFile()
		if file.Exists(cfgFile) {
			d.log.Infof("cfg: remove local config %s (localnode not in node %s config scope)", s, node)
			if err := os.Remove(cfgFile); err != nil {
				d.log.Debugf("cfg: remove %s: %s", cfgFile, err)
			}
		}
		return
	}
	if mtime, ok := d.cfgMTime[s]; ok {
		if !remoteInstanceConfig.UpdatedAt.After(mtime) {
			// our version is more recent than remote one
			return
		}
	} else if !remoteInstanceConfig.UpdatedAt.After(localUpdated) {
		// Not yet started icfg, but file exists
		return
	}
	if remoteFetcherUpdated, ok := d.fetcherUpdated[s]; ok {
		// fetcher in progress for s, verify if new fetcher is required
		if remoteInstanceConfig.UpdatedAt.After(remoteFetcherUpdated) {
			d.log.Warnf("cfg: cancel pending remote cfg fetcher, a more recent %s config is available on node %s", s, node)
			d.cancelFetcher(s)
		} else {
			// let running fetcher does its job
			return
		}
	}
	d.log.Infof("cfg: fetch %s config from node %s", s, node)
	d.fetchConfigFromRemote(p, node, remoteInstanceConfig)
}

func (d *discover) onInstanceConfigDeleted(c *msgbus.InstanceConfigDeleted) {
	if c.Node == "" || c.Node == d.localhost {
		return
	}
	s := c.Path.String()
	if fetchFrom, ok := d.fetcherFrom[s]; ok {
		if fetchFrom == c.Node {
			d.log.Infof("cfg: cancel pending remote cfg fetcher, instance %s@%s is no longer present", s, c.Node)
			d.cancelFetcher(s)
		}
	}
}

func (d *discover) onRemoteConfigFetched(c *msgbus.RemoteFileConfig) {

	freezeIfOrchestrateHA := func(confFile string) error {
		if !c.Freeze {
			return nil
		}
		if err := freeze.Freeze(c.Path.FrozenFile()); err != nil {
			d.log.Errorf("cfg: can't freeze instance before installing %s config fetched from node %s: %s", c.Path, c.Node, err)
			return err
		}
		d.log.Infof("cfg: freeze instance before installing %s config fetched from node %s", c.Path, c.Node)
		return nil
	}

	defer d.cancelFetcher(c.Path.String())
	select {
	case <-c.Ctx.Done():
		c.Err <- nil
	default:
		confFile := c.Path.ConfigFile()
		if err := freezeIfOrchestrateHA(confFile); err != nil {
			c.Err <- err
			return
		}
		if err := os.Rename(c.File, confFile); err != nil {
			d.log.Errorf("cfg: can't install %s config fetched from node %s to %s: %s", c.Path, c.Node, confFile, err)
			c.Err <- err
		} else {
			d.log.Infof("cfg: install %s config fetched from node %s", c.Path, c.Node)
		}
		c.Err <- nil
	}
}

func (d *discover) inScope(cfg *instance.Config) bool {
	localhost := d.localhost
	for _, s := range cfg.Scope {
		if s == localhost {
			return true
		}
	}
	return false
}

func (d *discover) cancelFetcher(s string) {
	if cancel, ok := d.fetcherCancel[s]; ok {
		d.log.Debugf("cfg: cancelFetcher %s", s)
		cancel()
		peer := d.fetcherFrom[s]
		delete(d.fetcherCancel, s)
		delete(d.fetcherNodeCancel[peer], s)
		delete(d.fetcherUpdated, s)
		delete(d.fetcherFrom, s)
	}
}

func (d *discover) fetchConfigFromRemote(p naming.Path, peer string, remoteInstanceConfig instance.Config) {
	s := p.String()
	if n, ok := d.fetcherFrom[s]; ok {
		d.log.Errorf("cfg: fetcher already in progress for %s from node %s", s, n)
		return
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.fetcherCancel[s] = cancel
	d.fetcherFrom[s] = peer
	d.fetcherUpdated[s] = remoteInstanceConfig.UpdatedAt
	if _, ok := d.fetcherNodeCancel[peer]; ok {
		d.fetcherNodeCancel[peer][s] = cancel
	} else {
		d.fetcherNodeCancel[peer] = make(map[string]context.CancelFunc)
	}

	cli, err := newDaemonClient(peer)
	if err != nil {
		d.log.Errorf("cfg: can't create newDaemonClient to fetch %s from node %s: %s", p, peer, err)
		return
	}
	go fetch(ctx, cli, p, peer, d.cfgCmdC, remoteInstanceConfig)
}

func fetch(ctx context.Context, cli *client.T, p naming.Path, peer string, cmdC chan<- any, remoteInstanceConfig instance.Config) {
	id := p.String() + "@" + peer
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/discover:cfg.fetch").Attr("id", id).WithPrefix("daemon: discover: cfg: fetch: ")

	tmpFilename, updated, err := remoteconfig.FetchObjectFile(cli, p)
	if err != nil {
		log.Warnf("unable to retrieve %s from %s: %s", id, cli.URL(), err)
		time.Sleep(250 * time.Millisecond)
		url := peerUrl(peer)
		if url == cli.URL() {
			return
		} else {
			log.Infof("detected updated %s url: recreate client to fetch %s", peer, id)
			if cli, err = newDaemonClient(peer); err != nil {
				log.Errorf("unable to recreate client: %s", err)
				return
			}
			if tmpFilename, updated, err = remoteconfig.FetchObjectFile(cli, p); err != nil {
				log.Infof("unable to retrieve %s from outdated url %s: %s", id, cli.URL(), err)
				return
			}
		}
	}
	defer func() {
		log.Debugf("routine done for instance %s@%s", p, peer)
		_ = os.Remove(tmpFilename)
	}()
	configure, err := object.NewConfigurer(p, object.WithConfigFile(tmpFilename), object.WithVolatile(true))
	if err != nil {
		log.Errorf("can't configure %s: %s", p, err)
		return
	}
	nodes, err := configure.Config().Referrer.Nodes()
	if err != nil {
		log.Errorf("can't eval nodes for %s: %s", p, err)
		return
	}
	validScope := false
	for _, n := range nodes {
		if n == hostname.Hostname() {
			validScope = true
			break
		}
	}
	if !validScope {
		log.Infof("invalid scope %s", nodes)
		return
	}
	var freeze bool
	if remoteInstanceConfig.Orchestrate == "ha" && len(remoteInstanceConfig.Scope) > 1 {
		freeze = true
	}
	select {
	case <-ctx.Done():
		log.Infof("abort on done context for %s", id)
		return
	default:
		err := make(chan error)
		cmdC <- &msgbus.RemoteFileConfig{
			Path:      p,
			Node:      peer,
			File:      tmpFilename,
			Freeze:    freeze,
			UpdatedAt: updated,
			Ctx:       ctx,
			Err:       err,
		}
		<-err
	}
}

func newDaemonClient(n string) (*client.T, error) {
	// TODO add WithRootCa to avoid send password to wrong url ?
	return client.New(
		client.WithURL(peerUrl(n)),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(ccfg.Get().Secret()),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
}

func peerUrl(s string) string {
	addr := s
	port := fmt.Sprintf("%d", daemonenv.HttpPort)
	if lsnr := node.LsnrData.Get(s); lsnr != nil {
		if lsnr.Port != "" {
			port = lsnr.Port
		}
		if lsnr.Addr != "::" && lsnr.Addr != "" {
			addr = lsnr.Addr
		}
	}
	return daemonenv.UrlHttpNodeAndPort(addr, port)
}
