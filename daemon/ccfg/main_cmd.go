package ccfg

import (
	"strings"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
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
		keyQuorum     = key.New("cluster", "quorum")

		keyListenerCRL             = key.New("listener", "crl")
		keyListenerAddr            = key.New("listener", "addr")
		keyListenerPort            = key.New("listener", "port")
		keyListenerTLSAddr         = key.New("listener", "tls_addr")
		keyListenerTLSPort         = key.New("listener", "tls_port")
		keyListenerOpenIdWellKnown = key.New("listener", "openid_well_known")
	)

	cfg := cluster.Config{}
	cfg.ID = o.clusterConfig.GetString(keyID)
	cfg.DNS = o.clusterConfig.GetStrings(keyDNS)
	cfg.Nodes = o.clusterConfig.GetStrings(keyNodes)
	cfg.Name = o.clusterConfig.GetString(keyName)
	cfg.CASecPaths = o.clusterConfig.GetStrings(keyCASecPaths)
	cfg.SetSecret(o.clusterConfig.GetString(keySecret))
	cfg.Quorum = o.clusterConfig.GetBool(keyQuorum)

	cfg.Listener.CRL = o.clusterConfig.GetString(keyListenerCRL)
	cfg.Listener.Addr = o.clusterConfig.GetString(keyListenerAddr)
	cfg.Listener.Port = o.clusterConfig.GetInt(keyListenerPort)
	cfg.Listener.TLSAddr = o.clusterConfig.GetString(keyListenerTLSAddr)
	cfg.Listener.TLSPort = o.clusterConfig.GetInt(keyListenerTLSPort)
	cfg.Listener.OpenIdWellKnown = o.clusterConfig.GetString(keyListenerOpenIdWellKnown)

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
	c.ErrC <- nil
	c.resp <- resp
}
