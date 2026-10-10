package network

import (
	"fmt"
	"math/big"

	"github.com/opensvc/om3/v3/core/clusterip"
	"github.com/opensvc/om3/v3/core/ipam"
)

type (
	Usage struct {
		Free *big.Int `json:"free"`
		Used *big.Int `json:"used"`
		Size *big.Int `json:"size"`
	}

	Status struct {
		Name    string      `json:"name"`
		Type    string      `json:"type"`
		Network string      `json:"network"`
		IPs     clusterip.L `json:"ips"`
		Errors  []string    `json:"errors,omitempty"`
		Usage
	}
	StatusList []Status
)

func NewStatus() Status {
	t := Status{}
	t.IPs = make(clusterip.L, 0)
	t.Errors = make([]string, 0)
	return t
}

// GetStatus returns the usage of the network. The usage counts are always
// set, rather than leaving its readers a nil count to dereference: a network
// whose range does not parse has a zero size and says why in its errors, and
// a network whose driver allows no range, as the loopback one, has a zero
// size and no error, as IsValid accepts it.
func GetStatus(t Networker, ips clusterip.L) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Network = t.Network()
	data.Usage.Used = big.NewInt(0)
	data.Usage.Size = big.NewInt(0)
	data.Usage.Free = big.NewInt(0)
	if data.Network == "" && t.AllowEmptyNetwork() {
		return data
	}
	ipn, err := t.IPNet()
	if err != nil {
		data.Errors = append(data.Errors, fmt.Sprintf("invalid network %q: %s", data.Network, err))
		return data
	}
	pools := []ipam.Pool{ipam.PoolOf(ipn)}
	if i, ok := t.(Pooler); ok {
		if pools, err = i.Pools(); err != nil {
			data.Errors = append(data.Errors, err.Error())
			return data
		}
	}
	if ips != nil {
		data.IPs = inPools(t.FilterIPs(ips), pools)
		data.Usage.Used = big.NewInt(int64(usedCount(t, data.IPs)))
	}
	for _, p := range pools {
		data.Usage.Size.Add(data.Usage.Size, p.Size())
	}
	data.Usage.Free = new(big.Int).Sub(data.Usage.Size, data.Usage.Used)
	return data
}

// inPools returns the addresses of the pools: a network handing out some of
// the addresses of its segment only counts those, the others being the ones
// of the nodes and of the other hosts of the segment.
func inPools(ips clusterip.L, pools []ipam.Pool) clusterip.L {
	l := make(clusterip.L, 0, len(ips))
	for _, ip := range ips {
		for _, p := range pools {
			if p.Contains(ip.IP) {
				l = append(l, ip)
				break
			}
		}
	}
	return l
}

func NewStatusList() StatusList {
	return make(StatusList, 0)
}

func (t StatusList) Len() int {
	return len(t)
}

func (t StatusList) Less(i, j int) bool {
	return t[i].Name < t[j].Name
}

func (t StatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t StatusList) Add(p Networker, ips clusterip.L) StatusList {
	s := GetStatus(p, ips)
	l := []Status(t)
	l = append(l, s)
	return StatusList(l)
}

func ShowNetworksByName(noder Noder, name string, ips clusterip.L) StatusList {
	l := NewStatusList()
	for _, p := range Networks(noder) {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(p, ips)
	}
	return l
}

func ShowNetworks(noder Noder, ips clusterip.L) StatusList {
	return ShowNetworksByName(noder, "", ips)
}

// usedCount is how many addresses of the network are used. Every node of a
// cluster-wide network reports the address a failover object holds, which is
// one address: the addresses are counted. The addresses of a node local
// network are the node's own, the same one on two nodes being two.
func usedCount(t Networker, ips clusterip.L) int {
	if i, ok := t.(ClusterWider); !ok || !i.IsClusterWide() {
		return len(ips)
	}
	seen := make(map[string]bool, len(ips))
	for _, ip := range ips {
		seen[ip.IP.String()] = true
	}
	return len(seen)
}
