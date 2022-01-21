package networklo

import (
	"opensvc.com/opensvc/core/network"
)

type (
	T struct {
		network.T
	}
)

func init() {
	network.Register("lo", NewNetworker)
}

func NewNetworker() network.Networker {
	t := New()
	var i interface{} = t
	return i.(network.Networker)
}

func New() *T {
	t := T{}
	return &t
}

func (t *T) Network() string {
	if t.IsImplicit() {
		return ""
	}
	return t.GetString("network")
}

func (t T) Usage() (network.StatusUsage, error) {
	usage := network.StatusUsage{}
	return usage, nil
}

func (t T) AllowEmptyNetwork() bool {
	return true
}

// CNIConfigData returns a cni network configuration, like
// {
//    "cniVersion": "0.3.0",
//    "name": "lo",
//    "type": "loopback"
// }
func (t T) CNIConfigData() (interface{}, error) {
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
