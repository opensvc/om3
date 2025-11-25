package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resourceid"
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
	sub := pubsub.SubFromContext(t.ctx, "daemon.discover.cfg", t.subQS)

	sub.AddFilter(&msgbus.ClusterConfigUpdated{})
	sub.AddFilter(&msgbus.ConfigFileUpdated{})

	sub.AddFilter(&msgbus.HeartbeatMessageTypeUpdated{}, t.labelLocalhost)

	sub.AddFilter(&msgbus.InstanceConfigDeleting{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigFor{})
	sub.AddFilter(&msgbus.InstanceConfigManagerDone{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigUpdated{})

	sub.AddFilter(&msgbus.ObjectStatusUpdated{})
	sub.AddFilter(&msgbus.ObjectStatusDeleted{})

	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, t.labelLocalhost)

	sub.Start()
	return sub
}

func (t *Manager) cfg(started chan<- bool) {
	t.log.Infof("cfg: started")
	defer t.log.Infof("cfg: stopped")
	defer func() {
		t.log.Tracef("cfg: flushing the command bus message queue")
		defer t.log.Tracef("cfg: flushed the command bus message queue")
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

			case *msgbus.HeartbeatMessageTypeUpdated:
				t.onHeartbeatMessageTypeUpdated(c)

			case *msgbus.InstanceConfigDeleting:
				t.onInstanceConfigDeleting(c)
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

			case *msgbus.InstanceStatusDeleted:
				t.onInstanceStatusDeleted(c)
			case *msgbus.InstanceStatusUpdated:
				t.onInstanceStatusUpdated(c)
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

func (t *Manager) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	prevWatched, ok := t.watched[c.Path]
	if !ok {
		prevWatched = make(map[string]any)
	}
	watched := make(map[string]any)
	mustWatch := func(rid string) bool {
		if resourceId, err := resourceid.Parse(rid); err != nil {
			return false
		} else if resourceId == nil {
			return false
		} else {
			switch resourceId.DriverGroup() {
			case driver.GroupTask:
			case driver.GroupSync:
			default:
				return false
			}
		}
		return true
	}
	watch := func(runDir string) {
		if _, ok := prevWatched[runDir]; ok {
			watched[runDir] = nil
		} else if err := t.fsWatcher.Add(runDir); errors.Is(err, os.ErrNotExist) {
			t.log.Tracef("fs: skip dir watch %s: does not exist yet", runDir)
		} else if err != nil {
			t.log.Warnf("fs: failed to add dir watch %s: %s", runDir, err)
		} else {
			t.log.Infof("fs: add dir watch %s", runDir)
			watched[runDir] = nil
		}
	}
	publishInitialRunFileUpdatedEvents := func(path naming.Path, node, rid, runDir string) {
		if _, ok := prevWatched[runDir]; ok {
			return
		}
		entries, err := os.ReadDir(runDir)
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		if err != nil {
			t.log.Warnf("fs: failed to list run files in %s: %s", runDir, err)
			return
		}
		for _, entry := range entries {
			filename := filepath.Join(runDir, entry.Name())
			t.PubDebounce(filename, &msgbus.RunFileUpdated{File: filename, Path: path, RID: rid, At: file.ModTime(filename)}, t.labelLocalhost, pubsub.Label{"namespace", path.Namespace}, pubsub.Label{"path", path.String()})
		}
	}
	for rid, _ := range c.Value.Resources {
		if !mustWatch(rid) {
			continue
		}
		runDir := filepath.Join(c.Path.VarDir(), rid, "run")
		publishInitialRunFileUpdatedEvents(c.Path, c.Node, rid, runDir)
		watch(runDir)
	}
	for runDir, _ := range prevWatched {
		if _, ok := watched[runDir]; !ok {
			if err := t.fsWatcher.Remove(runDir); err != nil {
				t.log.Warnf("fs: failed to remove dir watch %s (resource deleted): %s", runDir, err)
			} else {
				t.log.Infof("fs: remove dir watch %s (resource deleted)", runDir)
			}
		}
	}
	t.watched[c.Path] = watched
}

func (t *Manager) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if watched, ok := t.watched[c.Path]; ok {
		for runDir, _ := range watched {
			if err := t.fsWatcher.Remove(runDir); err != nil {
				t.log.Warnf("fs: failed to remove dir watch %s (instance deleted): %s", runDir, err)
			} else {
				t.log.Infof("fs: remove dir watch %s (instance deleted)", runDir)
			}
		}
		delete(t.watched, c.Path)
	}
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
	log := t.objectLogger(c.Path)
	s := c.Path.String()
	mtime := file.ModTime(c.File)
	if mtime.IsZero() {
		log.Infof("cfg: config file %s mtime is zero", c.File)
		return
	}
	if _, ok := t.cfgMTime[s]; !ok {
		log.Infof("cfg: config file updated for %s: start icfg", c.Path)
		if err := icfg.Start(t.ctx, c.Path, c.File, t.cfgCmdC); err != nil {
			log.Infof("cfg: can't start icfg for %s: %s", c.Path, err)
			return
		} else {
			t.cfgMTime[s] = mtime
		}
	} else {
		log.Tracef("cfg: config file updated already have icfg: %s", c.Path)
	}
}

// onInstanceConfigManagerDone handles situation where icfg is done,
// but have to be started again:
//
//   - when config file is absent,
//     and it has not been deleted during a purge/delete imon orchestration (
//     t.cfgDeleting[p].
//     and it has not been deleted because of out of foreign instance config (
//     t.disableRecover[p]
//     and there is a peer with config for us that is the most recent config
//     where updated is >= t.cfgMTime[s]
//     => recovering config file from this peer:
//     calls t.onInstanceConfigUpdated with a recreated InstanceConfigUpdated
//     event from peer.
//
//   - file exists and is more recent than last t.cfgMTime[s] and t.disableRecover[p]
//     is not set.
//     => start new icfg and set the new t.cfgMTime[s]
//
// Example on delete/create config file:
//
//	1- delete config file
//	2- icfg will end because of deleted file
//	   => it will publish InstanceConfigManagerDone
//	3- config file created => publish ConfigFileUpdated
//	4- discover.cfg onConfigFileUpdated, ignored because icfg not yet done
//	5- discover.cfg onInstanceConfigManagerDone must start new icfg if config
//	   file still present
//
// Example on delete config file:
//
//	1- rm config file
//	2- discover.cfg onInstanceConfigManagerDone, config file exists on peers
//	   it is recovered using t.onInstanceConfigUpdated called with the most
//	   recent InstanceConfigUpdated from peer
func (t *Manager) onInstanceConfigManagerDone(c *msgbus.InstanceConfigManagerDone) {
	filename := c.File
	p := c.Path
	s := p.String()
	log := t.objectLogger(p)

	cleanup := func() {
		delete(t.cfgMTime, s)
		delete(t.cfgDeleting, p)
		delete(t.disableRecover, p)
	}

	if mtime := file.ModTime(filename); mtime.IsZero() {
		if _, ok := t.cfgDeleting[p]; ok {
			log.Infof("cfg: icfg is done, after imon crm deleting config file for %s", p)
		} else if _, ok := t.disableRecover[p]; ok {
			log.Infof("cfg: icfg is done, after removal foreign config file for %s", p)
		} else if waiters, ok := t.retainForeignConfigFor[p]; ok {
			if len(waiters) > 0 {
				log.Infof("cfg: on icfg done, no more local config file, drop peer config waiters %s", waiters)
			} else {
				log.Infof("cfg: on icfg done, no more local config file, no peer config waiters")
			}
		} else {
			if peer, instanceConfig := t.mostRecentPeerConfig(p); instanceConfig != nil {
				log.Infof("cfg: icfg is done, recovering config file for %s from %s", p, peer)
				cleanup()
				t.onInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{
					Path:  p,
					Node:  peer,
					Value: *instanceConfig,
				})
				return
			}
			log.Infof("cfg: icfg is done for %s", p)
		}
		cleanup()
	} else if mtime.After(t.cfgMTime[s]) {
		if _, ok := t.disableRecover[p]; ok {
			log.Infof("cfg: icfg is done, foreign config file exists for %s", p)
			cleanup()
		} else {
			log.Infof("cfg: icfg is done, but recent config file exists for %s: start new icfg", p)
			if err := icfg.Start(t.ctx, p, filename, t.cfgCmdC); err != nil {
				log.Warnf("cfg: start new icfg for %s failed: %s", p, err)
				cleanup()
				return
			} else {
				t.cfgMTime[s] = mtime
			}
		}
	} else {
		log.Infof("cfg: icfg is done for %s but unchanged config file", p)
		cleanup()
	}
}

func (t *Manager) mostRecentPeerConfig(p naming.Path) (peer string, cfg *instance.Config) {
	pathS := p.String()
	for n, peerConfig := range instance.ConfigData.GetByPath(p) {
		if n == t.localhost {
			continue
		}
		if t.inScope(peerConfig) {
			if (cfg == nil || cfg.UpdatedAt.Before(peerConfig.UpdatedAt)) &&
				!t.cfgMTime[pathS].After(peerConfig.UpdatedAt) {
				cfg = peerConfig
				peer = n
			}
		}
	}
	return
}

func (t *Manager) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Node == t.localhost {
		return
	}
	t.onRemoteConfigUpdated(c.Path, c.Node, c.Value)
}

func (t *Manager) onRemoteConfigUpdated(p naming.Path, node string, remoteInstanceConfig instance.Config) {
	pathS := p.String()
	log := t.objectLogger(p)
	localUpdated := file.ModTime(p.ConfigFile())

	icfgFor, hasIcfgFor := t.instanceConfigFor[p]

	if waitPeerConfig, ok := t.retainForeignConfigFor[p]; ok {
		if t.inScope(&remoteInstanceConfig) {
			if !hasIcfgFor || remoteInstanceConfig.UpdatedAt.After(icfgFor.UpdatedAt) {
				log.Infof("cfg: config is not anymore foreign peer node %s has %s with scope (%s)", node, p, strings.Join(remoteInstanceConfig.Scope, ","))
				delete(t.retainForeignConfigFor, p)
				delete(t.instanceConfigFor, p)
			}
		} else {
			l := make([]string, 0)
			updates := false
			for _, waiting := range waitPeerConfig {
				if waiting == node {
					updates = true
					continue
				}
				l = append(l, waiting)
			}
			if updates {
				t.retainForeignConfigFor[p] = l
				if len(l) > 0 {
					log.Infof("cfg: retain the foreign config file for %s until remaining peers have retrieved it (%s)",
						p, strings.Join(t.retainForeignConfigFor[p], ","))
				} else {
					log.Infof("cfg: can release the foreign config file for %s: all remaining peers have retrieved it", p)
				}
			}
		}
	}
	// Never drop local cluster config, ignore remote config older that local
	if !p.Equal(naming.Cluster) &&
		(remoteInstanceConfig.UpdatedAt.After(localUpdated) || remoteInstanceConfig.UpdatedAt.Equal(localUpdated)) &&
		!t.inScope(&remoteInstanceConfig) {
		t.cancelFetcher(pathS)
		cfgFile := p.ConfigFile()
		if file.Exists(cfgFile) && len(t.retainForeignConfigFor[p]) == 0 {
			log.Infof("cfg: removing foreign config file for %s", pathS)
			t.removeConfigFileAndDisableRecover(p, remoteInstanceConfig.UpdatedAt)
			delete(t.cfgMTime, pathS)
			delete(t.retainForeignConfigFor, p)
			delete(t.instanceConfigFor, p)
		}
		return
	}
	if mtime, ok := t.cfgMTime[pathS]; ok {
		if !remoteInstanceConfig.UpdatedAt.After(mtime) {
			// our version is more recent than remote one
			return
		}
	}
	if !remoteInstanceConfig.UpdatedAt.After(localUpdated) {
		// Not yet started icfg, but file exists
		return
	}
	if remoteFetcherUpdated, ok := t.fetcherUpdated[pathS]; ok {
		// fetcher in progress for s, verify if new fetcher is required
		if remoteInstanceConfig.UpdatedAt.After(remoteFetcherUpdated) {
			log.Infof("cfg: cancel pending remote cfg fetcher, a more recent %s config is available on node %s", pathS, node)
			t.cancelFetcher(pathS)
		} else {
			// let running fetcher does its job
			return
		}
	}
	var needFreeze bool
	if remoteInstanceConfig.ActorConfig != nil && remoteInstanceConfig.ActorConfig.Orchestrate == "ha" && len(remoteInstanceConfig.Scope) > 1 {
		needFreeze = true
	}
	log.Infof("cfg: fetch config from %s@%s", pathS, node)
	t.fetchConfigFromRemote(p, node, remoteInstanceConfig.UpdatedAt, needFreeze, remoteInstanceConfig.Scope)
}

// removeConfigFileAndDisableRecover is called to remove config file on localhost
//
// When t.cfgMTime[pathS] exists, it sets t.cfgMTime[pathS] to updatedAt. This
// prevents onInstanceConfigManagerDone -> recover (icfg is running but will
// die because of config file removal => icfg will publish InstanceConfigManagerDone).
func (t *Manager) removeConfigFileAndDisableRecover(p naming.Path, updatedAt time.Time) {
	cfgFile := p.ConfigFile()
	pathS := p.String()
	log := t.objectLogger(p)
	bckName := fmt.Sprintf("%s.%s.%s_%s.bck", p.Namespace, p.Kind, p.Name, time.Now().Format(time.RFC3339))
	bckCfgFile := path.Join(rawconfig.Paths.Backup, bckName)
	log.Infof("cfg: archive removed file %s to %s", cfgFile, bckCfgFile)
	if err := os.Rename(cfgFile, bckCfgFile); err != nil {
		log.Tracef("cfg: archive removed file %s: %s", cfgFile, err)
	}
	if _, ok := t.cfgMTime[pathS]; ok {
		// the running icfg will die soon, onInstanceConfigManagerDone will be called
		// and must not try to recover the config.
		t.disableRecover[p] = updatedAt
		log.Tracef("cfg: disable the next onInstanceConfigManagerDone recovering of %s configuration", p)
	}
}

// onHeartbeatMessageTypeUpdated must re-emit pending instanceConfigFor event when the
// HeartbeatMessageTypeUpdated.To is "patch".
// instanceConfigFor are not applied during apply full.
func (t *Manager) onHeartbeatMessageTypeUpdated(c *msgbus.HeartbeatMessageTypeUpdated) {
	if c.To != "patch" {
		return
	}
	t.log.Tracef("cfg: hb message type is now patch, verify if foreign config file event must be re-emmited")
	for p, ev := range t.instanceConfigFor {
		mtime := file.ModTime(p.ConfigFile())
		if !mtime.IsZero() && len(ev.Scope) > 0 {
			t.objectLogger(p).Infof("cfg: re-publish remaining foreign config file %s for peers", p)
			t.publisher.Pub(&msgbus.InstanceConfigFor{
				Path:        p,
				Node:        t.localhost,
				Orchestrate: ev.Orchestrate,
				Scope:       append([]string{}, ev.Scope...),
				UpdatedAt:   ev.UpdatedAt,
			},
				pubsub.Label{"namespace", p.Namespace},
				pubsub.Label{"path", p.String()},
				t.labelLocalhost,
			)
		} else {
			t.objectLogger(p).Tracef("cfg: drop obsolete foreign config file %s event, local config file is absent", ev.Path)
			delete(t.instanceConfigFor, p)
		}
	}
}

func (t *Manager) onInstanceConfigDeleting(c *msgbus.InstanceConfigDeleting) {
	if c.Node != t.localhost {
		return
	}
	t.cfgDeleting[c.Path] = true
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
			log.Infof("cfg: cancel current fetcher because of more recent config file from %s@%s%s (was fetching from %s)",
				c.Path, c.Node, c.Scope, t.fetcherFrom[pathS])
			t.cancelFetcher(pathS)
		}
	}

	if _, ok := t.cfgMTime[pathS]; ok {
		t.disableRecover[c.Path] = c.UpdatedAt
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
		log.Infof("cfg: foreign config file for %s has disappeared", c.Path)
		t.abortRetainForeignConfig(c.Path)
	} else {
		// we have local config file
		if cfgFileUpdatedAt.After(c.UpdatedAt) {
			log.Infof("cfg: ignore obsolete foreign config file %s with scopes %s", c.Path, c.Scope)
			t.abortRetainForeignConfig(c.Path)
		} else if len(c.Scope) == 0 {
			log.Infof("cfg: removing foreign config file %s that has no scopes", c.Path)
			t.removeConfigFileAndDisableRecover(c.Path, c.UpdatedAt)
			t.abortRetainForeignConfig(c.Path)
		} else {
			log.Infof("cfg: retain the foreign config file for %s until peers have retrieved it (%s)", c.Path, strings.Join(c.Scope, ","))
			t.retainForeignConfigFor[c.Path] = c.Scope
			t.instanceConfigFor[c.Path] = c
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
		log.Infof("cfg: ignore obsolete foreign config file for %s from %s", c.Path, c.Node)
		t.abortRetainForeignConfig(c.Path)
		return
	}

	if !inList(t.localhost, c.Scope) {
		// peer node has an extra config file that is not for us
		if hasLocalConfigFile {
			log.Infof("cfg: removing foreign config file for %s from %s", c.Path, c.Node)
			t.removeConfigFileAndDisableRecover(c.Path, c.UpdatedAt)
			t.abortRetainForeignConfig(c.Path)
		}
		return
	}

	// peer node has an extra config file for us
	if mtime, ok := t.cfgMTime[pathS]; ok {
		if !peerConfigUpdatedAt.After(mtime) {
			log.Infof("cfg: more recent icfg has been started, ignore foreign config file for %s from %s", c.Path, c.Node)
			return
		}
	}
	if !peerConfigUpdatedAt.After(localFileUpdated) {
		log.Infof("cfg: more recent config file exists, ignore foreign config file for %s from %s", c.Path, c.Node)
		return
	}
	if t.fetcherUpdated[pathS].Equal(peerConfigUpdatedAt) {
		// let running fetcher does its job
		return
	}
	var needFreeze bool
	if c.Orchestrate == "ha" && len(c.Scope) > 1 {
		needFreeze = true
	}
	log.Infof("cfg: fetch config %s from foreign config file on %s", c.Path, c.Node)
	t.fetchConfigFromRemote(c.Path, c.Node, c.UpdatedAt, needFreeze, c.Scope)
}

func (t *Manager) abortRetainForeignConfig(p naming.Path) {
	if waitingPeers, ok := t.retainForeignConfigFor[p]; ok {
		t.objectLogger(p).Infof("cfg: abort retain foreign config file for %s peers (%s)", p, strings.Join(waitingPeers, ","))
	}
	delete(t.retainForeignConfigFor, p)
	delete(t.instanceConfigFor, p)
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
		if instance.ConfigData.GetByPathAndNode(c.Path, t.localhost) == nil {
			if err := freezeIfOrchestrateHA(confFile); err != nil {
				c.Err <- err
				return
			}
		}
		if err := os.Rename(c.File, confFile); err != nil {
			log.Errorf("cfg: can't install %s config fetched from node %s to %s: %s", c.Path, c.Node, confFile, err)
			c.Err <- err
		} else {
			// Prevents from absent or empty config on reboot before the config file is
			// synched to stable storage.
			if err := file.Sync(confFile); err != nil {
				log.Errorf("cfg: can't install %s config fetched from node %s to %s sync: %s", c.Path, c.Node, confFile, err)
				c.Err <- err
				return
			}
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
		t.log.Tracef("cfg: cancelFetcher %s@%s", s, peer)
		cancel()
		delete(t.fetcherCancel, s)
		delete(t.fetcherNodeCancel[peer], s)
		delete(t.fetcherUpdated, s)
		delete(t.fetcherFrom, s)
	}
}

func (t *Manager) fetchConfigFromRemote(p naming.Path, peer string, updatedAt time.Time, needFreeze bool, scope []string) {
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
	go fetch(ctx, cli, p, peer, t.cfgCmdC, needFreeze, scope)
}

func fetch(ctx context.Context, cli *client.T, p naming.Path, peer string, cmdC chan<- any, needFreeze bool, scope []string) {
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
		url := daemonsubsystem.PeerURL(peer)
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
		log.Tracef("routine done for instance %s@%s", p, peer)
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
			Freeze:    needFreeze,
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
		client.WithURL(daemonsubsystem.PeerURL(n)),
		client.WithUsername(hostname.Hostname()),
		client.WithPassword(cluster.ConfigData.Get().Secret()),
		client.WithCertificate(daemonenv.CertChainFile()),
	)
}

func inList(s string, l []string) bool {
	for _, v := range l {
		if s == v {
			return true
		}
	}
	return false
}
