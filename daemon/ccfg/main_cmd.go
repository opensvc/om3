package ccfg

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/stringslice"
)

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routine
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (t *Manager) onConfigFileUpdated(c *msgbus.ConfigFileUpdated) {
	if n, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		t.log.Errorf("can't create volatile cluster object on config file updated: %s", err)
	} else {
		t.clusterConfig = n.Config()
	}
	t.pubClusterConfig()
}

func (t *Manager) pubClusterConfig() {
	previousNodes := t.state.Nodes
	state := t.getClusterConfig()
	t.state = *state.DeepCopy()
	labelLocalNode := pubsub.Label{"node", t.localhost}

	removed, added := stringslice.Diff(previousNodes, state.Nodes)
	if len(added) > 0 {
		t.log.Debugf("added nodes: %s", added)
	}
	if len(removed) > 0 {
		t.log.Debugf("removed nodes: %s", removed)
	}
	cluster.ConfigData.Set(&state)
	clusternode.Set(state.Nodes)

	t.bus.Pub(&msgbus.ClusterConfigUpdated{Node: t.localhost, Value: state, NodesAdded: added, NodesRemoved: removed}, labelLocalNode)

	for _, v := range added {
		t.bus.Pub(&msgbus.JoinSuccess{Node: v}, labelLocalNode, pubsub.Label{"added", v})
	}
	for _, v := range removed {
		t.bus.Pub(&msgbus.LeaveSuccess{Node: v}, labelLocalNode, pubsub.Label{"removed", v})
	}
}

func (t *Manager) getClusterConfig() cluster.Config {
	var (
		keyID         = key.New("cluster", "id")
		keySecret     = key.New("cluster", "secret")
		keyName       = key.New("cluster", "name")
		keyNodes      = key.New("cluster", "nodes")
		keyDNS        = key.New("cluster", "dns")
		keyCASecPaths = key.New("cluster", "ca")
		keyQuorum     = key.New("cluster", "quorum")

		keyListenerCRL             = key.New("listener", "crl")
		keyListenerAddr            = key.New("listener", "addr")
		keyListenerPort            = key.New("listener", "port")
		keyListenerOpenIDWellKnown = key.New("listener", "openid_well_known")
		keyListenerDNSSockUID      = key.New("listener", "dns_sock_uid")
		keyListenerDNSSockGID      = key.New("listener", "dns_sock_gid")
	)

	cfg := cluster.Config{}
	cfg.ID = t.clusterConfig.GetString(keyID)
	cfg.DNS = t.clusterConfig.GetStrings(keyDNS)
	cfg.Nodes = t.clusterConfig.GetStrings(keyNodes)
	cfg.Name = t.clusterConfig.GetString(keyName)
	cfg.CASecPaths = t.clusterConfig.GetStrings(keyCASecPaths)
	cfg.SetSecret(t.clusterConfig.GetString(keySecret))
	cfg.Quorum = t.clusterConfig.GetBool(keyQuorum)
	cfg.Vip = t.getVipFromConfig(cfg.Nodes)
	cfg.Listener.CRL = t.clusterConfig.GetString(keyListenerCRL)
	if v, err := t.clusterConfig.Eval(keyListenerAddr); err != nil {
		t.log.Errorf("eval listener addr: %s", err)
	} else {
		cfg.Listener.Addr = v.(string)
	}
	if v, err := t.clusterConfig.Eval(keyListenerPort); err != nil {
		t.log.Errorf("eval listener port: %s", err)
	} else {
		cfg.Listener.Port = v.(int)
	}
	cfg.Listener.OpenIDWellKnown = t.clusterConfig.GetString(keyListenerOpenIDWellKnown)
	cfg.Listener.DNSSockGID = t.clusterConfig.GetString(keyListenerDNSSockGID)
	cfg.Listener.DNSSockUID = t.clusterConfig.GetString(keyListenerDNSSockUID)

	var change bool

	for _, name := range t.clusterConfig.SectionStrings() {
		if strings.HasPrefix(name, "network#") {
			lastSig, _ := t.networkSigs[name]
			sig := t.clusterConfig.SectionSig(name)
			if sig != lastSig {
				change = true
				t.log.Infof("configuration section %s changed (sig %s is now %s)", name, lastSig, sig)
				t.networkSigs[name] = sig
			}
		}
	}
	if change {
		if n, err := object.NewNode(object.WithLogger(t.log)); err != nil {
			t.log.Errorf("allocate Node for network setup: %s", err)
		} else {
			t.log.Infof("reconfigure networks")
			if err := network.Setup(n); err != nil {
				t.log.Infof("reconfigure networks: %s", err.Error())
			}
		}
	}
	return cfg
}

// getVipFromConfig returns the Vip from cluster config
func (t *Manager) getVipFromConfig(nodes []string) (vip cluster.Vip) {
	keyVip := key.New("cluster", "vip")
	defaultVip := t.clusterConfig.Get(keyVip)
	if defaultVip == "" {
		return cluster.Vip{}
	}

	// pickup defaults from vip keyword
	ipname, netmask, dev, err := parseVip(defaultVip)
	if err != nil {
		t.log.Errorf("skip vip: %s", err)
		return cluster.Vip{}
	}

	devs := make(map[string]string)

	for _, n := range nodes {
		v, err := t.clusterConfig.EvalAs(keyVip, n)
		if err != nil {
			t.log.Warnf("eval vip from node %s: %s", n, err)
			continue
		}
		customVip := v.(string)
		if customVip == "" || customVip == defaultVip {
			continue
		}
		if _, _, customDev, err := parseVip(customVip); err != nil {
			t.log.Errorf("skip vip value from node %s: %s", n, err)
			continue
		} else if customDev != dev {
			devs[n] = customDev
		}
	}

	return cluster.Vip{
		Default: defaultVip,
		Addr:    ipname,
		Netmask: netmask,
		Dev:     dev,
		Devs:    devs,
	}
}

func parseVip(s string) (ipname, netmask, ipdev string, err error) {
	r := strings.Split(s, "@")
	if len(r) != 2 {
		err = fmt.Errorf("unexpected vip value: missing @ in %s", s)
		return
	}
	if len(r[1]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty addr in %s", s)
		return
	}
	ipdev = r[1]
	r = strings.Split(r[0], "/")
	if len(r) != 2 {
		err = fmt.Errorf("unexpected vip value: missing / in %s", s)
		return
	}
	if len(r[0]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty ipname in %s", s)
		return
	}
	ipname = r[0]
	if len(r[1]) == 0 {
		err = fmt.Errorf("unexpected vip value: empty netmask in %s", s)
		return
	}
	netmask = r[1]
	return
}
