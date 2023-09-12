package network

import (
	"math"
	"net"

	"github.com/opensvc/om3/core/clusterip"
)

type (
	Usage struct {
		Free int     `json:"free" yaml:"free"`
		Used int     `json:"used" yaml:"used"`
		Size int     `json:"size" yaml:"size"`
		Pct  float64 `json:"pct" yaml:"pct"`
	}

	Status struct {
		Name    string      `json:"name" yaml:"name"`
		Type    string      `json:"type" yaml:"type"`
		Network string      `json:"network" yaml:"network"`
		IPs     clusterip.L `json:"ips" yaml:"ips"`
		Errors  []string    `json:"errors,omitempty" yaml:"errors,omitempty"`
		Usage   `yaml:",inline"`
	}
	StatusList []Status
)

func NewStatus() Status {
	t := Status{}
	t.IPs = make(clusterip.L, 0)
	t.Errors = make([]string, 0)
	return t
}

func GetStatus(t Networker, withUsage bool, ips clusterip.L) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Network = t.Network()
	if withUsage {
		usage, err := t.Usage()
		if err != nil {
			data.Errors = append(data.Errors, err.Error())
		}
		if _, n, err := net.ParseCIDR(data.Network); err == nil {
			ones, _ := n.Mask.Size()
			data.Size = int(math.Pow(2.0, float64(ones)))
		}
		data.Free = usage.Free
		data.Used = usage.Used
		if usage.Size == 0 {
			data.Pct = 100.0
		} else {
			data.Pct = float64(usage.Used) / float64(usage.Size) * 100.0
		}
	}
	if ips != nil {
		data.IPs = t.FilterIPs(ips)
	}
	return data
}

func NewStatusList() StatusList {
	l := make(StatusList, 0)
	return StatusList(l)
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

func (t StatusList) Add(p Networker, withUsage bool, ips clusterip.L) StatusList {
	s := GetStatus(p, withUsage, ips)
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
		l = l.Add(p, true, ips)
	}
	return l
}

func ShowNetworks(noder Noder, ips clusterip.L) StatusList {
	return ShowNetworksByName(noder, "", ips)
}
