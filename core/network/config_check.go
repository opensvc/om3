package network

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

func init() {
	object.RegisterClusterConfigCheck(checkClusterConfig)
}

// configNoder is the noder of a cluster configuration being validated, as
// written and not committed yet: its networks are the ones of that
// configuration.
type configNoder struct {
	cfg *xconfig.T
}

func (t configNoder) MergedConfig() *xconfig.T {
	return t.cfg
}

func (t configNoder) Log() *plog.Logger {
	return plog.NewDefaultLogger().Attr("pkg", "core/network")
}

func (t configNoder) Nodes() ([]string, error) {
	return t.cfg.GetStrings(key.New("cluster", "nodes")), nil
}

// checkClusterConfig says the networks of a cluster configuration that
// cannot be used, as Check finds them, so a write making one is refused, and
// a configuration holding one says so when validated: the network setup
// would refuse it.
func checkClusterConfig(cfg *xconfig.T) xconfig.Alerts {
	var alerts xconfig.Alerts
	nws := Networks(configNoder{cfg: cfg})
	invalid := Check(nws)
	for _, nw := range nws {
		err := invalid[nw.Name()]
		if err == nil {
			continue
		}
		option := "network"
		if _, ok := nw.(Pooler); ok {
			option = "ranges"
		}
		k := key.New("network#"+nw.Name(), option)
		alerts = append(alerts, cfg.NewAlertCandidates(k, driver.NewID(driver.GroupNetwork, nw.Type()), err.Error()))
	}
	return alerts
}
