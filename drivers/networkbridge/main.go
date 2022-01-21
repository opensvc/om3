package networkbridge

import (
	"opensvc.com/opensvc/core/network"
)

type (
	T struct {
		network.T
	}
)

func init() {
	network.Register("bridge", NewNetworker)
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

func (t T) Usage() (network.StatusUsage, error) {
	usage := network.StatusUsage{}
	return usage, nil
}

// CNIConfigData returns a cni network configuration, like
// {
//   "bridge": "cni0",
//   "cniVersion": "0.3.0",
//   "ipMasq": true,
//   "name": "mynet",
//   "ipam": {
//     "routes": [
//       {"dst": "0.0.0.0/0"}
//     ],
//     "subnet": "10.22.0.0/16",
//     "type": "host-local"
//   },
//   "isGateway": true,
//   "type": "bridge"
// }
func (t T) CNIConfigData() (interface{}, error) {
	name := t.Name()
	m := map[string]interface{}{
		"cniVersion": network.CNIVersion,
		"name":       name,
		"type":       "bridge",
		"bridge":     "obr_" + name,
		"isGateway":  true,
		"ipMasq":     true,
		"ipam": map[string]interface{}{
			"type": "host-local",
			"routes": []map[string]interface{}{
				{"dst": "0.0.0.0/0"},
			},
			"subnet": t.Network(),
		},
	}
	return m, nil
}
