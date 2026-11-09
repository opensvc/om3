package collector

import (
	"errors"
	"io/fs"
	"time"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

var (
	kindsConfigToPost = naming.NewKinds(naming.KindSvc, naming.KindVol)
)

func (t *T) onRefreshTicker() {
	if t.starting() {
		// The startup hold ends at its deadline, whatever the peers say.
		t.updateSpeaker()
		t.publishOnChange(t.getState())
	}
	t.checkCredentials(time.Now())
	if time.Since(t.actionAnnounceCheckAt) >= actionAnnounceInterval {
		t.actionAnnounceCheckAt = time.Now()
		t.announceActionPending(false)
		t.pruneActionSent()
	}
	if t.isSpeaker {
		// Each send runs off the loop, at most one per feed: a collector
		// slow to answer holds no event.
		t.sendDaemonData()
		t.sendObjectConfigChange()
		t.sendResInfoChange()
		t.sendActions()
	} else {
		t.previousUpdatedAt = time.Time{}
		t.dropChanges()
	}
	t.setPendingMetrics()
}

func (t *T) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	for _, nodename := range c.NodesAdded {
		t.clusterNode[nodename] = struct{}{}
	}
	for _, nodename := range c.NodesRemoved {
		delete(t.clusterNode, nodename)
	}
	// The speaker is chosen in the order of the cluster nodes.
	t.updateSpeaker()
}

func (t *T) onInstanceConfigDeleted(c *msgbus.InstanceConfigDeleted) {
	if instanceConfig, ok := t.objectConfigToSend[c.Path]; ok {
		if instanceConfig == nil {
			// nothing to drop
			return
		}
		if instanceConfig.Node != c.Node {
			// don't drop yet, wait for event InstanceConfigDeleted from the same
			// node that emit the InstanceConfigUpdated.
			return
		}
		delete(t.objectConfigToSend, c.Path)
	}
}

func (t *T) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if !kindsConfigToPost.Has(c.Path.Kind) {
		return
	}
	sent, ok := t.objectConfigSent[c.Path]
	if !ok {
		sent.path = c.Path
		if err := sent.read(); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				t.log.Tracef("onInstanceConfigUpdated from %s@%s with checksum %s sent config cache absent", c.Path, c.Node, c.Value.Checksum)
			} else {
				t.log.Warnf("can't read sent config: %s", err)
			}
		} else {
			t.log.Tracef("onInstanceConfigUpdated from %s@%s with checksum %s init sent config cache %s", c.Path, c.Node, c.Value.Checksum, sent.Checksum)
			t.objectConfigSent[c.Path] = sent
		}
	}
	if sent.Checksum == c.Value.Checksum {
		// skip already sent config
		t.log.Tracef("onInstanceConfigUpdated from %s@%s skipped on same checksum %s", c.Path, c.Node, sent.Checksum)
		return
	}

	// Prefer the localhost objectConfigToSend if checksum match to avoid from
	// fetching config from peer when already present on localhost.
	// Example on node1 speaker:
	//     create object on node1
	//     => event 1: InstanceConfigUpdated{Node: <node1>
	//     => event 2: InstanceConfigUpdated{Node: <node2> (after node2 fetch config from node1) we can drop
	//                 event if its checksum value is same as event1 checksum.
	if toSend, ok := t.objectConfigToSend[c.Path]; ok && toSend != nil {
		// already have objectConfigToSend
		if toSend.Value.Checksum == c.Value.Checksum {
			if c.Node != t.localhost {
				t.log.Tracef("onInstanceConfigUpdated from %s@%s skipped: found localhost InstanceConfigUpdated with same checksum %s", c.Path, c.Node, sent.Checksum)
				return
			}
		}
	}

	t.log.Tracef("onInstanceConfigUpdated from %s@%s checksum %s need send", c.Path, c.Node, c.Value.Checksum)
	t.objectConfigToSend[c.Path] = c
}

func (t *T) onInstanceResourceInfoUpdated(c *msgbus.InstanceResourceInfoUpdated) {
	i := instance.InstanceString(c.Path, c.Node)
	if sent, ok := t.resInfoSent[i]; ok && sent.Checksum == c.Checksum {
		t.log.Tracef("onInstanceResourceInfoUpdated %s skipped on same checksum %s", i, c.Checksum)
		return
	}
	t.log.Tracef("onInstanceResourceInfoUpdated %s checksum %s need send", i, c.Checksum)
	t.resInfoToSend[i] = c
}

func (t *T) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	i := instance.InstanceString(c.Path, c.Node)
	t.daemonStatusChange[i] = struct{}{}
	delete(t.resInfoToSend, i)
	delete(t.resInfoSent, i)
	sent := resInfoSent{path: c.Path, nodename: c.Node}
	if err := sent.drop(); err != nil {
		t.log.Warnf("drop sent resource info flag for %s: %s", i, err)
	}
}

func (t *T) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	t.daemonStatusChange[instance.InstanceString(c.Path, c.Node)] = struct{}{}
}

func (t *T) onNodeConfigUpdated(c *msgbus.NodeConfigUpdated) {
	t.applyConfig(c.Value.Collector)
}

// applyConfig sets the collector up from its configuration, and remembers
// it as the one applied.
func (t *T) applyConfig(cfg *collector.Config) {
	t.log.Tracef("reconfigure")
	if !cfg.Equal(t.appliedConfig) {
		// The sends in flight go to the collector of the configuration
		// they were cut with.
		t.newFeedEpoch()
	}
	if cfg == nil || t.appliedConfig == nil ||
		cfg.Password != t.appliedConfig.Password ||
		cfg.FeederUrl != t.appliedConfig.FeederUrl {
		// A new node.uuid, or another collector, is offered at once,
		// whatever the collector refused of the former credentials.
		t.resetCredsRefused()
	}
	t.appliedConfig = cfg.DeepCopy()
	t.setPostTimeout(cfg)
	t.setThrottle(cfg)
	t.setActionTunables(cfg)
	if err := t.setupRequester(cfg); err != nil {
		if !errors.Is(err, collector.ErrConfig) {
			t.log.Errorf("can't setup requester: %s", err)
		}
	}
	// The collector configuration enables or disables the speaker, as a
	// collector url set or unset on a node already elected.
	t.updateSpeaker()
	t.publishOnChange(t.getState())
}

// setPostTimeout sets the timeout of the posts of the daemon data from the
// collector configuration cfg, nil when the collector is not configured.
func (t *T) setPostTimeout(cfg *collector.Config) {
	n := collector.DefaultTimeout
	if cfg != nil {
		n = collector.BoundTimeout(cfg.Timeout)
	}
	if n != t.postTimeout {
		t.log.Infof("feeder post timeout: %s", n)
		t.postTimeout = n
	}
}

// onNodeMonitorUpdated chooses the speaker again when a node comes back: the
// monitor of a peer rejoining, which tells its liveness, comes after its node
// status and its collector state.
func (t *T) onNodeMonitorUpdated(c *msgbus.NodeMonitorUpdated) {
	if c.Node == t.localhost {
		return
	}
	t.updateSpeaker()
}

func (t *T) onNodeMonitorDeleted(c *msgbus.NodeMonitorDeleted) {
	delete(t.nodeFrozenAt, c.Node)
	t.daemonStatusChange[c.Node] = struct{}{}
	// A node gone speaks no more.
	t.updateSpeaker()
}

func (t *T) onNodeStatusUpdated(c *msgbus.NodeStatusUpdated) {
	if c.Value.FrozenAt != t.nodeFrozenAt[c.Node] {
		t.nodeFrozenAt[c.Node] = c.Value.FrozenAt
		t.daemonStatusChange[c.Node] = struct{}{}
	}
	t.nodeIsLeader[c.Node] = c.Value.IsLeader
	// The status of a peer comes with its liveness, as at its rejoin.
	t.updateSpeaker()
}

// onDaemonCollectorUpdated chooses the speaker again on a change of the
// collector state of a peer: one losing or getting its client, as on a
// change of its node.uuid, gives or takes the speech.
func (t *T) onDaemonCollectorUpdated(c *msgbus.DaemonCollectorUpdated) {
	if c.Node == t.localhost {
		return
	}
	t.updateSpeaker()
}

// speakerNode returns the node speaking to the collector: the first live
// cluster node whose collector can speak, as the node leader is the first
// live one. A node whose collector can not, as one that lost its node.uuid,
// leaves the speech to the next, and stays the leader, granting the cluster
// locks. With no node able to speak, the leader speaks, its state saying the
// warning.
//
// Every node chooses from the same data, the cluster nodes, their liveness
// and the collector states they publish, and so the same speaker.
func (t *T) speakerNode() string {
	for _, nodename := range clusternode.Get() {
		if nodename != t.localhost && node.MonitorData.GetByNode(nodename) == nil {
			// not alive
			continue
		}
		if t.canSpeak(nodename) {
			return nodename
		}
	}
	// The leader is taken among the live nodes, in their order: the leader
	// flag of a node gone stays set, beside the one of the leader elected
	// after it.
	for _, nodename := range clusternode.Get() {
		if nodename != t.localhost && node.MonitorData.GetByNode(nodename) == nil {
			continue
		}
		if nodename == t.localhost && t.starting() {
			// A leader starting does not speak during its startup hold
			// either: the node speaking in its absence may still be.
			continue
		}
		if t.nodeIsLeader[nodename] {
			return nodename
		}
	}
	return ""
}

// canSpeak says whether the collector of nodename can speak: configured,
// with a client whose credentials the collector does not refuse, and done
// starting. A peer whose collector state is not known yet, as one rejoining
// whose collector has not started, can not: the node speaking keeps the
// speech until the peer says it can take it.
func (t *T) canSpeak(nodename string) bool {
	if nodename == t.localhost {
		return !t.disable && t.client != nil && !t.credsRefused() && !t.starting()
	}
	c := daemonsubsystem.DataCollector.Get(nodename)
	if c == nil || c.State == "" {
		return false
	}
	switch c.State {
	case "speaker", "speaker-candidate":
		return true
	default:
		return false
	}
}

// updateSpeaker sets isSpeaker from the speaker chosen, and applies its
// transition. It must be called on a change of what the choice reads: the
// collector configuration and client of the node, the cluster nodes, their
// liveness, their leader, or the collector state of a peer.
func (t *T) updateSpeaker() {
	t.checkStarting(time.Now())
	speaker := t.speakerNode()
	if speaker == t.localhost && !t.isSpeaker {
		// Nodes starting together each see the others starting, unable to
		// speak yet: the first of them in the cluster nodes takes the
		// speech, the others leave it to it. A node speaking already keeps
		// it, as from a peer rejoining.
		if peer := t.startingPeerBefore(); peer != "" {
			speaker = peer
		}
	}
	if speaker != t.speaker {
		if t.speaker != "" && speaker != "" {
			// A new speaker has none of the local actions pending:
			// announce them all again.
			t.log.Infof("collector speaker %s -> %s", t.speaker, speaker)
			defer t.announceActionPending(true)
		}
		t.speaker = speaker
	}
	isSpeaker := !t.disable && speaker == t.localhost
	if isSpeaker != t.isSpeaker {
		t.isSpeaker = isSpeaker
		if isSpeaker {
			// A node not speaker drops the changes it sees: the new
			// speaker sends the full dataset, the changes the former
			// speaker failed to send included.
			t.initChanges()
			t.seedResInfoToSend()
		} else {
			t.newFeedEpoch()
			t.dropActionToSend()
			t.daemonStatusFailure.reset()
			t.resInfoFailure.reset()
			t.objectConfigFailure.reset()
		}
	}
	// The state changes without the speaker too, as at the end of the
	// startup hold of a node not speaking: starting to speaker-candidate.
	t.publishOnChange(t.getState())
}

func (t *T) onObjectStatusDeleted(c *msgbus.ObjectStatusDeleted) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
	delete(t.clusterObject, c.Path.String())
}

func (t *T) onObjectStatusUpdated(c *msgbus.ObjectStatusUpdated) {
	t.daemonStatusChange[c.Path.String()] = struct{}{}
	t.clusterObject[c.Path.String()] = struct{}{}
}
