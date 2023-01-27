package ccfg

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/key"
)

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routine
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (o *ccfg) onConfigFileUpdated(c msgbus.ConfigFileUpdated) {
	if err := o.clusterConfig.Reload(); err != nil {
		o.log.Error().Err(err).Msg("reload merged config")
		return
	}
	o.pubClusterConfig()
}

func (o *ccfg) pubClusterConfig() {
	o.state = o.getClusterConfig()
	err := o.databus.SetClusterConfig(o.state)
	if err != nil {
		o.log.Error().Err(err).Msg("SetClusterConfig")
	}
}

func (o *ccfg) getClusterConfig() cluster.Config {
	var (
		keySecret = key.New("cluster", "secret")
		keyName   = key.New("cluster", "name")
		keyNodes  = key.New("cluster", "nodes")
		keyDNS    = key.New("cluster", "dns")
	)
	cfg := cluster.Config{}
	cfg.DNS = o.clusterConfig.GetStrings(keyDNS)
	cfg.Nodes = o.clusterConfig.GetStrings(keyNodes)
	cfg.Name = o.clusterConfig.GetString(keyName)
	cfg.SetSecret(o.clusterConfig.GetString(keySecret))
	return cfg
}

func (o *ccfg) onCmdGet(c cmdGet) {
	resp := o.state
	c.resp <- resp
}
