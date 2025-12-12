package networklo

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/network"
)

type (
	T struct {
		network.T
	}
)

var (
	drvID = driver.NewID(driver.GroupNetwork, "lo")
)

func init() {
	driver.Register(drvID, NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	t.SetAllowEmptyNetwork(true)
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	t := T{}
	return &t
}

func (t *T) Network() string {
	if t.IsImplicit() {
		return t.T.Network()
	}
	return t.GetString("network")
}

// CNIConfigData returns a cni network configuration, like
//
//	{
//	   "cniVersion": "0.3.0",
//	   "name": "lo",
//	   "type": "loopback"
//	}
func (t *T) CNIConfigData() (interface{}, error) {
	m := map[string]interface{}{
		"cniVersion": network.CNIVersion,
		"name":       t.Name(),
		"type":       "loopback",
	}
	nwStr := t.Network()
	if nwStr != "" {
		m["ipam"] = map[string]interface{}{
			"subnet": t.Network(),
		}
	}
	return m, nil
}
