package ccfg

import (
	"strings"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
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
		keyID         = key.New("cluster", "id")
		keySecret     = key.New("cluster", "secret")
		keyName       = key.New("cluster", "name")
		keyNodes      = key.New("cluster", "nodes")
		keyDNS        = key.New("cluster", "dns")
		keyCASecPaths = key.New("cluster", "ca")
	)
	cfg := cluster.Config{}
	cfg.ID = o.clusterConfig.GetString(keyID)
	cfg.DNS = o.clusterConfig.GetStrings(keyDNS)
	cfg.Nodes = o.clusterConfig.GetStrings(keyNodes)
	cfg.Name = o.clusterConfig.GetString(keyName)
	cfg.CASecPaths = o.clusterConfig.GetStrings(keyCASecPaths)
	cfg.SetSecret(o.clusterConfig.GetString(keySecret))

	var change bool

	for _, name := range o.clusterConfig.SectionStrings() {
		if strings.HasPrefix(name, "network#") {
			lastSig, _ := o.networkSigs[name]
			sig := o.clusterConfig.SectionSig(name)
			if sig != lastSig {
				change = true
				o.log.Info().Msgf("%s configuration changed (sig %s => %s)", name, lastSig, sig)
				o.networkSigs[name] = sig
			}
		}
	}
	if change {
		if n, err := object.NewNode(); err != nil {
			o.log.Error().Err(err).Msg("allocate Node for network setup")
		} else {
			o.log.Info().Msgf("reconfigure networks")
			network.Setup(n)
		}
	}
	return cfg
}

func (o *ccfg) onCmdGet(c cmdGet) {
	resp := o.state
	c.resp <- resp
}
