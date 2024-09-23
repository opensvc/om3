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
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/icfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/remoteconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func (t *Manager) startSubscriptions() *pubsub.Subscription {
	bus := pubsub.BusFromContext(t.ctx)
	sub := bus.Sub("daemon.discover.cfg", t.subQS)

	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.ConfigFileUpdated{})

	sub.AddFilter(&msgbus.InstanceConfigDeleted{})
	sub.AddFilter(&msgbus.InstanceConfigFor{})
	sub.AddFilter(&msgbus.InstanceConfigManagerDone{}, pubsub.Label{"node", t.localhost})
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})

	sub.AddFilter(&msgbus.ObjectStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDeleted{})

	sub.Start()
	return sub
}

func (t *Manager) cfg(started chan<- bool) {
	t.log.Infof("cfg: started")
	defer t.log.Infof("cfg: stopped")
	defer func() {
		t.log.Debugf("cfg: flushing the command bus message queue")
		defer t.log.Debugf("cfg: flushed the command bus message queue")
		ticker := time.NewTicker(t.drainDuration)
		defer ticker.Stop()
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
				return
			case <-t.cfgCmdC:
			}
		}
	}()
	sub := t.startSubscriptions()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("cfg: subscription stop: %s", err)
		}
	}()
	if last := cluster.ConfigData.Get(); last != nil {
		msg := &msgbus.ClusterConfigUpdated{Value: *last}
		t.onClusterConfigUpdated(msg)
	}
	started <- true
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(c)
			case *msgbus.ConfigFileUpdated:
				t.onConfigFileUpdated(c)

			case *msgbus.InstanceConfigDeleted:
				t.onInstanceConfigDeleted(c)
			case *msgbus.InstanceConfigFor:
				t.onInstanceConfigFor(c)
			case *msgbus.InstanceConfigManagerDone:
				t.onInstanceConfigManagerDone(c)
			case *msgbus.InstanceConfigUpdated:
				t.onInstanceConfigUpdated(c)

			case *msgbus.ObjectStatusDeleted:
				t.onObjectStatusDeleted(c)
			case *msgbus.ObjectStatusUpdated:
				t.onObjectStatusUpdated(c)
			}
		case i := <-t.cfgCmdC:
			switch c := i.(type) {
			case *msgbus.RemoteFileConfig:
				t.onRemoteConfigFetched(c)
			default:
				t.log.Errorf("cfg: unsupported command bus message type: %#v", i)
			}
		case nfo := <-t.objectList.InfoC:
			t.log.Infof("cfg: object list: " + nfo)
		case err := <-t.objectList.ErrC:
			t.log.Infof("cfg: object list error: %s", err)
		case nfo := <-t.nodeList.InfoC:
			t.log.Infof("cfg: node list: " + nfo)
		case err := <-t.nodeList.ErrC:
			t.log.Infof("cfg: node list: error: %s", err)
		}
	}
}

func (t *Manager) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	t.clusterConfig = c.Value
	t.nodeList.Add(c.NodesAdded...)
	t.nodeList.Del(c.NodesRemoved...)
}

func (t *Manager) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	t.objectList.Add(c.Path.String())
}

func (t *Manager) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.objectList.Del(c.Path.String())
}

func (t *Manager) onConfigFileUpdated(c *msgbus.ConfigFileUpdated) {
	if c.Path.Kind == naming.KindInvalid {
		// may be node.conf
		return
	}
	s := c.Path.String()
	mtime := file.ModTime(c.File)
	if mtime.IsZero() {
		t.objectLogger(c.Path).Infof("cfg: config file %s mtime is zero", c.File)
		return
	}
	if _, ok := t.cfgMTime[s]; !ok {
		if err := icfg.Start(t.ctx, c.Path, c.File, t.cfgCmdC); err != nil {
			return
		}
	}
	t.cfgMTime[s] = mtime
}

// onInstanceConfigManagerDone starts a new icfg when a local configuration file exists
func (t *Manager) onInstanceConfigManagerDone(c *msgbus.InstanceConfigManagerDone) {
	filename := c.File
	p := c.Path
	s := p.String()

	delete(t.cfgMTime, s)
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		return
	}
	if err := icfg.Start(t.ctx, p, filename, t.cfgCmdC); err != nil {
		return
	}
	t.cfgMTime[s] = mtime
}

func (t *Manager) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Node == t.localhost {
		return
	}
	t.onRemoteConfigUpdated(c.Path, c.Node, c.Value)
}

func (t *Manager) onRemoteConfigUpdated(p naming.Path, node string, remoteInstanceConfig instance.Config) {
	s := p.String()
	log := t.objectLogger(p)
	localUpdated := file.ModTime(p.ConfigFile())

	// Never drop local cluster config, ignore remote config older that local
	if !p.Equal(naming.Cluster) && remoteInstanceConfig.UpdatedAt.After(localUpdated) && !t.inScope(&remoteInstanceConfig) {
		t.cancelFetcher(s)
		cfgFile := p.ConfigFile()
		if file.Exists(cfgFile) {
			log.Infof("cfg: remove local config %s (localnode not in node %s config scope)", s, node)
			if err := os.Remove(cfgFile); err != nil {
				log.Debugf("cfg: remove %s: %s", cfgFile, err)
			}
		}
		return
	}
	if mtime, ok := t.cfgMTime[s]; ok {
		if !remoteInstanceConfig.UpdatedAt.After(mtime) {
			// our version is more recent than remote one
			return
		}
	} else if !remoteInstanceConfig.UpdatedAt.After(localUpdated) {
		// Not yet started icfg, but file exists
		return
	}
	if remoteFetcherUpdated, ok := t.fetcherUpdated[s]; ok {
		// fetcher in progress for s, verify if new fetcher is required
		if remoteInstanceConfig.UpdatedAt.After(remoteFetcherUpdated) {
			log.Infof("cfg: cancel pending remote cfg fetcher, a more recent %s config is available on node %s", s, node)
			t.cancelFetcher(s)
		} else {
			// let running fetcher does its job
			return
		}
	}
	log.Infof("cfg: fetch %s config from node %s", s, node)
	t.fetchConfigFromRemote(p, node, remoteInstanceConfig.UpdatedAt, remoteInstanceConfig.Orchestrate, remoteInstanceConfig.Scope)
}

func (t *Manager) onInstanceConfigDeleted(c *msgbus.InstanceConfigDeleted) {
	if c.Node == "" || c.Node == t.localhost {
		return
	}
	s := c.Path.String()
	if fetchFrom, ok := t.fetcherFrom[s]; ok {
		if fetchFrom == c.Node {
			t.objectLogger(c.Path).Infof("cfg: cancel pending remote cfg fetcher, instance %s@%s is no longer present", s, c.Node)
			t.cancelFetcher(s)
		}
	}
}

// onInstanceConfigFor is called on InstanceConfigFor event.
//
// It cancels obsolete fetcher if any.
// If more recent local exists it returns
// else it calls onInstanceConfigForFromLocalhost or onInstanceConfigForFromPeer
func (t *Manager) onInstanceConfigFor(c *msgbus.InstanceConfigFor) {
	if c.Path.Equal(naming.Cluster) {
		t.log.Warnf("humm InstanceConfigFor for cluster!")
		return
	}

	log := t.objectLogger(c.Path)
	pathS := c.Path.String()

	if fetchingUpdatingAt, ok := t.fetcherUpdated[pathS]; ok {
		if c.UpdatedAt.After(fetchingUpdatingAt) {
			log.Infof("cfg: cancel current fetcher because of more recent config file from %s@%s[%s] (was fetching from %s)",
				c.Path, c.Node, c.Scope, t.fetcherFrom[pathS])
			t.cancelFetcher(pathS)
		}
	}

	if c.Node == t.localhost {
		t.onInstanceConfigForFromLocalhost(c)
	} else {
		t.onInstanceConfigForFromPeer(c)
	}
}

// onInstanceConfigForFromLocalhost is called on InstanceConfigFor event from localhost.
func (t *Manager) onInstanceConfigForFromLocalhost(c *msgbus.InstanceConfigFor) {
	log := t.objectLogger(c.Path)
	cfgFile := c.Path.ConfigFile()
	cfgFileUpdatedAt := file.ModTime(cfgFile)

	if cfgFileUpdatedAt.IsZero() {
		log.Infof("cfg: local config already deleted on local extra config for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
		t.abortKeepLocalFileForPeer(c.Path)
	} else {
		// we have local config file
		if cfgFileUpdatedAt.After(c.UpdatedAt) {
			log.Infof("cfg: ignore obsolete local extra config for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
			t.abortKeepLocalFileForPeer(c.Path)
		} else if len(c.Scope) == 0 {
			log.Infof("cfg: remove config file on local node scope extra config for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
			if err := os.Remove(cfgFile); err != nil {
				log.Warnf("cfg: remove %s: %s", cfgFile, err)
			}
			t.abortKeepLocalFileForPeer(c.Path)
		} else {
			log.Infof("cfg: keep extra local config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
			t.waitPeerInstanceConfigUpdated[c.Path] = c.Scope
		}
	}
}

func (t *Manager) onInstanceConfigForFromPeer(c *msgbus.InstanceConfigFor) {
	log := t.objectLogger(c.Path)

	pathS := c.Path.String()

	cfgFile := c.Path.ConfigFile()
	hasLocalConfigFile := file.Exists(cfgFile)
	peerConfigUpdatedAt := c.UpdatedAt
	localFileUpdated := file.ModTime(cfgFile)

	if localFileUpdated.After(peerConfigUpdatedAt) {
		log.Infof("cfg: ignore obsolete extra config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
		t.abortKeepLocalFileForPeer(c.Path)
		return
	}

	if !inList(t.localhost, c.Scope) {
		// peer node has an extra config file that is not for us
		if hasLocalConfigFile {
			log.Infof("cfg: remove local config file on extra config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
			if err := os.Remove(cfgFile); err != nil {
				log.Debugf("cfg: remove %s: %s", cfgFile, err)
			}
			t.abortKeepLocalFileForPeer(c.Path)
		}
		return
	}

	// peer node has an extra config file for us
	if mtime, ok := t.cfgMTime[pathS]; ok {
		if !peerConfigUpdatedAt.After(mtime) {
			log.Infof("cfg: more recent icfg has been started, ignore extra config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
			return
		}
	}
	if !peerConfigUpdatedAt.After(localFileUpdated) {
		log.Infof("cfg: more recent config file exists, ignore extra config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
		return
	}
	if t.fetcherUpdated[pathS].Equal(peerConfigUpdatedAt) {
		// let running fetcher does its job
		return
	}
	log.Infof("cfg: fetch config from extra config file for peers %s@%s[%s]", c.Path, c.Node, c.Scope)
	t.fetchConfigFromRemote(c.Path, c.Node, c.UpdatedAt, c.Orchestrate, c.Scope)
}

func (t *Manager) abortKeepLocalFileForPeer(p naming.Path) {
	if waitingPeers, ok := t.waitPeerInstanceConfigUpdated[p]; ok {
		t.objectLogger(p).Infof("cfg: abort keep extra local config file for peers: %s@[%s]", p, waitingPeers)
	}
	delete(t.waitPeerInstanceConfigUpdated, p)
}

func (t *Manager) onRemoteConfigFetched(c *msgbus.RemoteFileConfig) {
	log := t.objectLogger(c.Path)

	freezeIfOrchestrateHA := func(confFile string) error {
		if !c.Freeze {
			return nil
		}
		if err := freeze.Freeze(c.Path.FrozenFile()); err != nil {
			t.log.Errorf("cfg: can't freeze instance before installing %s config fetched from node %s: %s", c.Path, c.Node, err)
			return err
		}
		log.Infof("cfg: freeze instance before installing %s config fetched from node %s", c.Path, c.Node)
		return nil
	}

	defer t.cancelFetcher(c.Path.String())
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
			log.Errorf("cfg: can't install %s config fetched from node %s to %s: %s", c.Path, c.Node, confFile, err)
			c.Err <- err
		} else {
			log.Infof("cfg: install %s config fetched from node %s", c.Path, c.Node)
		}
		c.Err <- nil
	}
}

func (t *Manager) inScope(cfg *instance.Config) bool {
	return inList(t.localhost, cfg.Scope)
}

func (t *Manager) cancelFetcher(s string) {
	if cancel, ok := t.fetcherCancel[s]; ok {
		peer := t.fetcherFrom[s]
		t.log.Debugf("cfg: cancelFetcher %s@%s", s, peer)
		cancel()
		delete(t.fetcherCancel, s)
		delete(t.fetcherNodeCancel[peer], s)
		delete(t.fetcherUpdated, s)
		delete(t.fetcherFrom, s)
	}
}

func (t *Manager) fetchConfigFromRemote(p naming.Path, peer string, updatedAt time.Time, orchestrate string, scope []string) {
	if peer == "" {
		t.objectLogger(p).Errorf("cfg: fetch config %s from node ''", p)
		return
	}
	s := p.String()
	if n, ok := t.fetcherFrom[s]; ok {
		t.objectLogger(p).Errorf("cfg: fetcher already in progress for %s from node %s", s, n)
		return
	}
	ctx, cancel := context.WithCancel(t.ctx)
	t.fetcherCancel[s] = cancel
	t.fetcherFrom[s] = peer
	t.fetcherUpdated[s] = updatedAt
	if _, ok := t.fetcherNodeCancel[peer]; ok {
		t.fetcherNodeCancel[peer][s] = cancel
	} else {
		t.fetcherNodeCancel[peer] = make(map[string]context.CancelFunc)
	}

	cli, err := newDaemonClient(peer)
	if err != nil {
		t.objectLogger(p).Errorf("cfg: can't create newDaemonClient to fetch %s from node %s: %s", p, peer, err)
		return
	}
	go fetch(ctx, cli, p, peer, t.cfgCmdC, orchestrate, scope)
}

func fetch(ctx context.Context, cli *client.T, p naming.Path, peer string, cmdC chan<- any, orchestrate string, scope []string) {
	id := p.String() + "@" + peer
	log := naming.LogWithPath(plog.NewDefaultLogger(), p).
		Attr("pkg", "daemon/discover").
		Attr("id", id).WithPrefix("daemon: discover: cfg: fetch: ")
	if peer == "" {
		log.Errorf("fetch called without peer for %s", p)
		panic("daemon/discover.cfg call fetch without peer")
	}
	tmpFilename, updated, err := remoteconfig.FetchObjectConfigFile(cli, p)
	if err != nil {
		log.Warnf("unable to retrieve %s from %s: %s", id, cli.URL(), err)
		time.Sleep(250 * time.Millisecond)
		url := peerURL(peer)
		if url == cli.URL() {
			return
		} else {
			log.Infof("detected updated %s url: recreate client to fetch %s", peer, id)
			if cli, err = newDaemonClient(peer); err != nil {
				log.Errorf("unable to recreate client: %s", err)
				return
			}
			if tmpFilename, updated, err = remoteconfig.FetchObjectConfigFile(cli, p); err != nil {
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
	var freezeV bool
	if orchestrate == "ha" && len(scope) > 1 {
		freezeV = true
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
			Freeze:    freezeV,
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
		client.WithURL(peerURL(n)),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(cluster.ConfigData.Get().Secret()),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
}

func peerURL(s string) string {
	addr := s
	port := fmt.Sprintf("%d", daemonenv.HTTPPort)
	if lsnr := daemonsubsystem.DataListener.Get(s); lsnr != nil {
		if lsnr.Port != "" {
			port = lsnr.Port
		}
		if lsnr.Addr != "::" && lsnr.Addr != "" {
			addr = lsnr.Addr
		}
	}
	return daemonenv.HTTPNodeAndPortURL(addr, port)
}

func inList(s string, l []string) bool {
	for _, v := range l {
		if s == v {
			return true
		}
	}
	return false
}
